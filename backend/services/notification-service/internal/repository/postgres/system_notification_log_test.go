//go:build integration

// 需要本機 Docker daemon（testcontainers 起 postgres 重放 db/migrations）。
// getTestPool 與 CI 接線見 dbtest_test.go 檔頭。
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

func TestSystemNotificationLogRepository_InsertAndFindByContentHash(t *testing.T) {
	pool := getTestPool(t)
	repo := NewSystemNotificationLogRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	log := &domain.SystemNotificationLog{
		NotificationType:     "SYSTEM_MAINTENANCE",
		Title:                "test title",
		Body:                 "test body",
		ContentHash:          "abc123def456",
		TriggeredByAccountID: 999,
	}
	if err := repo.Insert(ctx, log); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if log.ID == 0 {
		t.Fatal("expected ID assigned after insert")
	}

	found, err := repo.FindRecentByContentHash(ctx, "abc123def456", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if found == nil || found.ID != log.ID {
		t.Fatalf("expected to find log id=%d, got %v", log.ID, found)
	}
}

func TestSystemNotificationLogRepository_UpdateStatus(t *testing.T) {
	pool := getTestPool(t)
	repo := NewSystemNotificationLogRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	log := &domain.SystemNotificationLog{
		NotificationType: "TERMS_UPDATE",
		Title:            "x", Body: "y", ContentHash: "h2",
		TriggeredByAccountID: 1,
	}
	if err := repo.Insert(ctx, log); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	fcmID := "fcm-message-123"
	if err := repo.UpdateStatus(ctx, log.ID, "sent", &fcmID, nil); err != nil {
		t.Fatalf("update status: %v", err)
	}

	logs, _ := repo.List(ctx, "TERMS_UPDATE", 10, 0)
	var got *domain.SystemNotificationLog
	for _, l := range logs {
		if l.ID == log.ID {
			got = l
			break
		}
	}
	if got == nil {
		t.Fatalf("expected to find log id=%d after update, got nil", log.ID)
	}
	if got.Status != "sent" {
		t.Errorf("expected status=sent, got %q", got.Status)
	}
	if got.FCMMessageID == nil || *got.FCMMessageID != fcmID {
		t.Errorf("expected FCMMessageID=%q, got %v", fcmID, got.FCMMessageID)
	}
	if got.CompletedAt == nil {
		t.Errorf("expected CompletedAt non-nil after status update")
	}
}
