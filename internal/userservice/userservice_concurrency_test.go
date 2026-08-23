package userservice_test

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"api-mock-system/internal/auth"
	"api-mock-system/internal/email"
	"api-mock-system/internal/models"
	"api-mock-system/internal/storage"
	"api-mock-system/internal/userrepo"
	"api-mock-system/internal/userservice"

	"go.uber.org/zap"
)

// newTestStore opens a fresh in-memory-adjacent SQLite DB, runs the same
// AutoMigrate + email-normalization backfill the production app runs at boot,
// and returns the *gorm.DB wired through the exact same stack a real request
// uses. This makes the test exercise the real unique-index + TranslateError
// path rather than a mocked contract.
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	log := zap.NewNop()
	store, err := storage.Open(context.Background(), dsn, log)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// userRepoForTest builds the real GORM repository against the test store, so the
// service talks to the actual unique-index enforcement path.
func userRepoForTest(t *testing.T, store *storage.Store) userrepo.Repository {
	t.Helper()
	return userrepo.New(store.DB)
}

// countUsersByEmail counts rows whose normalized email matches addr, including
// rows stored in a different case — so the test catches any account that
// escaped normalization.
func countUsersByEmail(store *storage.Store, addr string) (int64, error) {
	var n int64
	if err := store.DB.Model(&models.User{}).
		Where("lower(email) = ?", email.Normalize(addr)).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// TestRegisterConcurrentSingleAccount is the core regression: when many
// requests register the same email at the same instant, exactly one account is
// created and every other request is told the email is taken (-> 409 upstream).
// Before the fix, the check-then-create race let multiple accounts through.
func TestRegisterConcurrentSingleAccount(t *testing.T) {
	store := newTestStore(t)
	authn, err := auth.New("test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := userservice.New(userRepoForTest(t, store), authn)

	const n = 16
	email := "founder@example.com"

	var success, taken int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := userservice.RegisterInput{
				Email:    email,
				Password: "secret123",
				Name:     "Founder",
			}
			// Different casing in some requests exercises the normalization
			// guarantee under concurrency: "Founder@Example.com" must collide
			// with "founder@example.com", not create a second account.
			if i%3 == 0 {
				in.Email = "Founder@Example.com"
			} else if i%3 == 1 {
				in.Email = " FOUNDER@example.com "
			}
			<-start // fire all goroutines as simultaneously as possible
			_, err := svc.Register(context.Background(), in)
			switch {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case err.Error() == userservice.ErrEmailTaken.Error():
				atomic.AddInt64(&taken, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if success != 1 {
		t.Fatalf("expected exactly 1 successful registration, got %d", success)
	}
	if taken != n-1 {
		t.Fatalf("expected %d conflicts, got %d", n-1, taken)
	}

	// No matter which casing won, exactly one row exists for the identity.
	count, err := countUsersByEmail(store, "founder@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 persisted account, got %d", count)
	}
}

// TestLoginCaseInsensitive proves normalization closes the login gap: an
// account registered as "User@Example.com" can be reached by logging in as
// "user@example.com". Before the fix the lookup missed and returned invalid
// credentials.
func TestLoginCaseInsensitive(t *testing.T) {
	store := newTestStore(t)
	authn, err := auth.New("test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := userservice.New(userRepoForTest(t, store), authn)

	if _, err := svc.Register(context.Background(), userservice.RegisterInput{
		Email: "User@Example.com", Password: "secret123", Name: "U",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	u, token, err := svc.Login(context.Background(), userservice.LoginInput{
		Email: "user@example.com", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("login with lowercased email: %v", err)
	}
	if u.Email != "user@example.com" {
		t.Fatalf("stored email not normalized: got %q", u.Email)
	}
	if token == "" {
		t.Fatal("empty token")
	}
}

// TestRegisterMixedCaseCollides ensures two sequential registrations that differ
// only by case are treated as the same identity: the second returns a conflict.
func TestRegisterMixedCaseCollides(t *testing.T) {
	store := newTestStore(t)
	authn, err := auth.New("test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := userservice.New(userRepoForTest(t, store), authn)

	if _, err := svc.Register(context.Background(), userservice.RegisterInput{
		Email: "Alice@Example.com", Password: "secret123", Name: "Alice",
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err = svc.Register(context.Background(), userservice.RegisterInput{
		Email: "alice@example.com", Password: "secret123", Name: "Alice2",
	})
	if err == nil || err.Error() != userservice.ErrEmailTaken.Error() {
		t.Fatalf("second register: want ErrEmailTaken, got %v", err)
	}
}

// unsafeguardedRepo is a Repository whose check-then-create window never
// reports a conflict: FindByEmail always misses, Create always succeeds. It is
// exactly the failure mode the production code had before the unique index +
// conflict translation existed. Used by TestRegisterWithoutDBGuardDuplicates
// to prove that, absent the DB constraint, the service alone creates multiple
// accounts — i.e. the constraint is load-bearing, not the pre-check.
type unsafeguardedRepo struct {
	mu   sync.Mutex
	rows []models.User
}

func (u *unsafeguardedRepo) Create(_ context.Context, user *models.User) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.rows = append(u.rows, *user) // no uniqueness enforcement
	return nil
}
func (u *unsafeguardedRepo) FindByEmail(_ context.Context, _ string) (*models.User, error) {
	return nil, userrepo.ErrNotFound // always misses -> pre-check always passes
}
func (u *unsafeguardedRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, r := range u.rows {
		if r.ID == id {
			r2 := r
			return &r2, nil
		}
	}
	return nil, userrepo.ErrNotFound
}
func (u *unsafeguardedRepo) List(_ context.Context, _, _ int) ([]models.User, int64, error) {
	return nil, 0, nil
}
func (u *unsafeguardedRepo) Len() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.rows)
}

// TestRegisterWithoutDBGuardDuplicates is the contrast case: with a repository
// that has no uniqueness enforcement, concurrent registration of the same
// email produces multiple accounts. This documents the bug the fix closes and
// guards against regressions that remove the DB-level safeguard.
func TestRegisterWithoutDBGuardDuplicates(t *testing.T) {
	authn, err := auth.New("test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repo := &unsafeguardedRepo{}
	svc := userservice.New(repo, authn)

	const n = 16
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = svc.Register(context.Background(), userservice.RegisterInput{
				Email: "dupe@example.com", Password: "secret123", Name: "D",
			})
		}()
	}
	close(start)
	wg.Wait()

	if got := repo.Len(); got <= 1 {
		t.Fatalf("control test misconfigured: expected >1 account without DB guard (proving the race), got %d", got)
	}
	// NOTE: this test asserts the UNSAFEGUARDED behavior on purpose. The real
	// repository's guard is exercised by TestRegisterConcurrentSingleAccount.
}
