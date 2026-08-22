package debughandler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/debugrepo"
	"api-mock-system/internal/debugservice"
	"api-mock-system/internal/middleware"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/mockservice"
	"api-mock-system/internal/models"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type contractDebugLogRepo struct {
	saved chan error
}

func (r *contractDebugLogRepo) Save(ctx context.Context, _ *models.DebugLog) error {
	err := ctx.Err()
	r.saved <- err
	return err
}

func (*contractDebugLogRepo) ListByUser(context.Context, string, int) ([]models.DebugLog, error) {
	return nil, nil
}

func (*contractDebugLogRepo) ListByAPI(context.Context, string, int) ([]models.DebugLog, error) {
	return nil, nil
}

var _ debugrepo.Repository = (*contractDebugLogRepo)(nil)

func TestFailedDebugRequestRetainsHistoryRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:debug-contract?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.ProjectMember{}, &models.API{}, &models.MockData{}); err != nil {
		t.Fatal(err)
	}

	const userID = "debug-user"
	const projectID = "debug-project"
	const apiID = "debug-api"
	if err := db.Create(&models.Project{Base: models.Base{ID: projectID}, OwnerID: userID, Name: "debug"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ProjectMember{Base: models.Base{ID: "debug-member"}, ProjectID: projectID, UserID: userID, Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.API{Base: models.Base{ID: apiID}, ProjectID: projectID, Name: "target", Method: http.MethodGet, Path: "/target", Status: "published"}).Error; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apiQueries := 0
	if err := db.Callback().Query().Before("gorm:query").Register("debug-contract:cancel-mock-query", func(tx *gorm.DB) {
		if tx.Statement.Table == "apis" {
			apiQueries++
			if apiQueries == 2 {
				cancel()
			}
		}
	}); err != nil {
		t.Fatal(err)
	}

	projects := projectservice.New(projectrepo.New(db))
	apis := apiservice.New(apirepo.New(db), projects)
	mock := mockservice.New(apis, mockdatarepo.New(db), cache.New(), zap.NewNop())
	logs := &contractDebugLogRepo{saved: make(chan error, 1)}
	h := New(debugservice.New(apis, mock, logs, zap.NewNop()))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.UserIDKey), userID)
		c.Next()
	})
	router.POST("/apis/:id/debug", h.Debug)
	req := httptest.NewRequest(http.MethodPost, "/apis/"+apiID+"/debug", strings.NewReader(`{"method":"GET","path":"/target"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("debug request status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	select {
	case saveErr := <-logs.saved:
		if saveErr != nil {
			t.Fatalf("failed debug invocation was not retained: %v", saveErr)
		}
	default:
		t.Fatal("failed debug invocation did not reach the history repository")
	}
}
