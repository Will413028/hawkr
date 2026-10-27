package mq

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// releaseRecorderMediaClient records ReleaseReferences calls; the other
// MediaClient methods are unused by the account.deleted handler.
type releaseRecorderMediaClient struct {
	usecase.MediaClient
	calls  []releaseCall
	failOn string // refType to fail on ("" = never)
}

type releaseCall struct {
	refType string
	refIDs  []string
}

func (m *releaseRecorderMediaClient) ReleaseReferences(_ context.Context, refType string, refIDs []string, _ []string) error {
	m.calls = append(m.calls, releaseCall{refType: refType, refIDs: refIDs})
	if m.failOn == refType {
		return context.DeadlineExceeded
	}
	return nil
}

func TestAccountDeletedHandler_ReleasesLogoAndCover(t *testing.T) {
	mc := &releaseRecorderMediaClient{}
	h := accountDeletedHandler(mc)

	err := h(context.Background(), "account.deleted", []byte(`{"event_type":"ACCOUNT_DELETED","accountId":2685914682}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mc.calls) != 2 {
		t.Fatalf("expected 2 release calls, got %d: %+v", len(mc.calls), mc.calls)
	}
	if mc.calls[0].refType != pkgmedia.RefTypeSellerLogo {
		t.Errorf("first call refType = %q, want SELLER_LOGO (%q)", mc.calls[0].refType, pkgmedia.RefTypeSellerLogo)
	}
	if mc.calls[1].refType != pkgmedia.RefTypeSellerCover {
		t.Errorf("second call refType = %q, want SELLER_COVER (%q)", mc.calls[1].refType, pkgmedia.RefTypeSellerCover)
	}
	for _, c := range mc.calls {
		if len(c.refIDs) != 1 || c.refIDs[0] != "2685914682" {
			t.Errorf("refIDs = %v, want [2685914682]", c.refIDs)
		}
	}
}

func TestAccountDeletedHandler_MissingAccountID(t *testing.T) {
	mc := &releaseRecorderMediaClient{}
	h := accountDeletedHandler(mc)
	if err := h(context.Background(), "account.deleted", []byte(`{"event_type":"ACCOUNT_DELETED"}`)); err == nil {
		t.Fatal("expected error for missing accountId, got nil")
	}
	if len(mc.calls) != 0 {
		t.Fatalf("expected no release calls on invalid payload, got %d", len(mc.calls))
	}
}

// A release failure must propagate so the ResilientConsumer retries / DLQs
// rather than silently leaking references.
func TestAccountDeletedHandler_ReleaseFailurePropagates(t *testing.T) {
	mc := &releaseRecorderMediaClient{failOn: pkgmedia.RefTypeSellerLogo}
	h := accountDeletedHandler(mc)
	if err := h(context.Background(), "account.deleted", []byte(`{"accountId":123}`)); err == nil {
		t.Fatal("expected error when ReleaseReferences fails, got nil")
	}
}

// TestAccountDeletedPayload_EventTypeTagMatchesRealWire guards against the
// wire-tag bug this task fixes: accountDeletedPayload tagged EventType as
// json:"eventType", but the real publisher (account-service) emits
// event_type on the wire (pkg/mq/events.go's BaseEvent, line 84).
func TestAccountDeletedPayload_EventTypeTagMatchesRealWire(t *testing.T) {
	wire, err := json.Marshal(pkgmq.AccountDeletedEvent{
		BaseEvent: pkgmq.NewBaseEvent(pkgmq.EventTypeAccountDeleted, "account-service"),
		AccountID: 777,
	})
	if err != nil {
		t.Fatalf("marshal real wire event: %v", err)
	}

	var payload pkgmq.AccountDeletedEvent
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.EventType != pkgmq.EventTypeAccountDeleted {
		t.Errorf("EventType = %q, want %q (payload's tag must match the real wire key event_type, not eventType)", payload.EventType, pkgmq.EventTypeAccountDeleted)
	}
	if payload.AccountID != 777 {
		t.Errorf("AccountID = %d, want 777", payload.AccountID)
	}
}

func TestAccountDeletedHandler_MalformedJSON_UsesBindTypedHandlerErrorFormat(t *testing.T) {
	mc := &releaseRecorderMediaClient{}
	h := accountDeletedHandler(mc)

	err := h(context.Background(), "account.deleted", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from pkgmq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
	if len(mc.calls) != 0 {
		t.Fatalf("expected no release calls on malformed JSON, got %d", len(mc.calls))
	}
}
