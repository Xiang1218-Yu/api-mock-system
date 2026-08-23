package verification_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"api-mock-system/internal/auth"
	"api-mock-system/internal/models"
	"api-mock-system/internal/userhandler"
	"api-mock-system/internal/userrepo"
	"api-mock-system/internal/userservice"

	"github.com/gin-gonic/gin"
)

// source marker: userservice.Register -> userrepo.Create -> userhandler.Register
type concurrentUserRepo struct {
	mu          sync.Mutex
	users       map[string]*models.User
	findMu      sync.Mutex
	findCalls   int
	findRelease chan struct{}
}

func newConcurrentUserRepo() *concurrentUserRepo {
	return &concurrentUserRepo{
		users:       make(map[string]*models.User),
		findRelease: make(chan struct{}),
	}
}

func (r *concurrentUserRepo) Create(_ context.Context, u *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[u.Email]; exists {
		return errors.New("duplicate email")
	}
	copy := *u
	r.users[u.Email] = &copy
	return nil
}

func (r *concurrentUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	r.findMu.Lock()
	r.findCalls++
	if r.findCalls == 2 {
		close(r.findRelease)
	}
	r.findMu.Unlock()
	<-r.findRelease

	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[strings.TrimSpace(email)]
	if !ok {
		return nil, userrepo.ErrNotFound
	}
	copy := *u
	return &copy, nil
}

func (r *concurrentUserRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.ID == id {
			copy := *u
			return &copy, nil
		}
	}
	return nil, userrepo.ErrNotFound
}

func (r *concurrentUserRepo) List(_ context.Context, _, _ int) ([]models.User, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, *u)
	}
	return out, int64(len(out)), nil
}

func TestRegisterConcurrentDuplicateEmailReturnsConflict(t *testing.T) {
	repo := newConcurrentUserRepo()
	jwtAuth, err := auth.New("verification-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service := userservice.New(repo, jwtAuth)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, registerErr := service.Register(context.Background(), userservice.RegisterInput{
				Email:    "duplicate@example.com",
				Password: "password",
				Name:     "tester",
			})
			errs <- registerErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for registerErr := range errs {
		if registerErr == nil {
			successes++
		}
	}
	if successes != 1 || len(repo.users) != 1 {
		t.Fatalf("concurrent registration created an unexpected account set: successes=%d users=%d", successes, len(repo.users))
	}

	gin.SetMode(gin.TestMode)
	handler := userhandler.New(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"email":"DUPLICATE@EXAMPLE.COM","password":"password","name":"again"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	handler.Register(ctx)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if len(repo.users) != 1 {
		t.Fatalf("case-insensitive duplicate created another account: users=%d", len(repo.users))
	}
}
