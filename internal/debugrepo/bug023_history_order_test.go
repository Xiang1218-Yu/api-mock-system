package debugrepo

import (
	"context"
	"testing"
	"time"

	"api-mock-system/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBug023HistoryOrderReturnsNewestFirst(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:bug023?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.DebugLog{}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	now := time.Now()
	for i, at := range []time.Time{now.Add(-time.Minute), now} {
		if err := repo.Save(context.Background(), &models.DebugLog{
			Base:  models.Base{ID: string(rune('a' + i)), CreatedAt: at},
			APIID: func() *string { v := "api-1"; return &v }(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	logs, err := repo.ListByAPI(context.Background(), "api-1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || !logs[0].CreatedAt.Equal(now) {
		t.Fatalf("history order=%v, want newest entry first", logs)
	}
}
