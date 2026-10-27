package usecase

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// Locks the SSE payload contract: camelCase keys + TSID id/userId as quoted
// strings. A regression here double-fires web toasts (see TSID id incident).
func TestInboxItemToMap_CamelAndStringIDs(t *testing.T) {
	m := inboxItemToMap(&domain.InboxItem{
		ID:     853917025239746700, // TSID > 2^53
		UserID: 853917025239746701,
	})
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	for _, key := range []string{`"id":"853917025239746700"`, `"userId":"853917025239746701"`,
		`"userType":`, `"eventType":`, `"referenceId":`, `"isRead":`, `"createdAt":`, `"readAt":`} {
		if !strings.Contains(got, key) {
			t.Errorf("missing camel key %s; got %s", key, got)
		}
	}
	for _, snake := range []string{`"user_id"`, `"user_type"`, `"event_type"`, `"reference_id"`, `"is_read"`, `"read_at"`, `"created_at"`} {
		if strings.Contains(got, snake) {
			t.Errorf("snake key %s leaked; got %s", snake, got)
		}
	}
}
