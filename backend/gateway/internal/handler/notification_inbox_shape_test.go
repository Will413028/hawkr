package handler

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
)

// Locks the REST inbox wire contract: camelCase keys + TSID id as a quoted
// string. Fails if anyone regresses inboxItemDTO to snake_case or number id.
func TestProtoToInboxDTO_CamelAndStringID(t *testing.T) {
	dto := protoToInboxDTO(&pb.NotificationItem{
		Id:        "853917025239746700", // TSID > 2^53
		EventType: "order.paid",
		Title:     "t",
		Body:      "b",
		IsRead:    false,
		CreatedAt: "2026-06-25T00:00:00Z",
		ReadAt:    "2026-06-25T01:00:00Z",
		Data:      map[string]string{"order_id": "1"},
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	for _, key := range []string{`"id":"853917025239746700"`, `"type":"ORDER_PAID"`,
		`"refType":`, `"refId":`, `"isRead":`, `"readAt":`, `"createdAt":`} {
		if !strings.Contains(got, key) {
			t.Errorf("missing camel key %s; got %s", key, got)
		}
	}
	for _, snake := range []string{`"ref_type"`, `"ref_id"`, `"is_read"`, `"read_at"`, `"created_at"`} {
		if strings.Contains(got, snake) {
			t.Errorf("snake key %s leaked; got %s", snake, got)
		}
	}
}
