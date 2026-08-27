package verification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-mock-system/internal/auth"
	"api-mock-system/internal/id"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/models"
	"api-mock-system/internal/storage"
	"api-mock-system/internal/userhandler"
	"api-mock-system/internal/userrepo"
	"api-mock-system/internal/userservice"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBug021AuthenticatedSubjectUsesStableSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:bug021?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	authn, err := auth.New("secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repo := userrepo.New(db)
	users := userservice.New(repo, authn)
	user := &models.User{Base: models.Base{ID: id.NewUUID()}, Email: "alice@example.com", Name: "Alice"}
	user.PasswordHash, err = auth.HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	token, err := authn.Issue(user.ID, user.Email)
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(middleware.RequireAuth(authn))
	r.GET("/me", userhandler.New(users).Me)
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated current-user lookup failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

var _ = storage.Store{}
var _ = zap.NewNop
