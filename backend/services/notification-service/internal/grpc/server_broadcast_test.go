package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"go.uber.org/zap"
)

type broadcastTokenRepo struct {
	tokens      []*domain.DeviceToken
	deactivated []string
}

func (m *broadcastTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error  { return nil }
func (m *broadcastTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error { return nil }
func (m *broadcastTokenRepo) DeactivateByToken(_ context.Context, token string) error {
	m.deactivated = append(m.deactivated, token)
	return nil
}

func (m *broadcastTokenRepo) DeactivateByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (m *broadcastTokenRepo) GetActiveByUser(_ context.Context, userID int64, userType string) ([]*domain.DeviceToken, error) {
	var out []*domain.DeviceToken
	for _, t := range m.tokens {
		if t.UserID == userID && t.UserType == userType {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *broadcastTokenRepo) GetActiveByUserType(_ context.Context, userType string, _, offset int64) ([]*domain.DeviceToken, error) {
	if offset > 0 {
		return nil, nil
	}
	var out []*domain.DeviceToken
	for _, t := range m.tokens {
		if t.UserType == userType {
			out = append(out, t)
		}
	}
	return out, nil
}

// broadcastNotifier fails exactly the tokens listed in failTokens, mirroring
// FCM's per-token error map (the transport call itself still succeeds).
type broadcastNotifier struct {
	failTokens map[string]error
	sent       [][]string
}

func (m *broadcastNotifier) Send(_ context.Context, tokens []string, _, _, _ string, _ map[string]string) (map[string]error, error) {
	m.sent = append(m.sent, tokens)
	if len(m.failTokens) == 0 {
		return nil, nil
	}
	failures := map[string]error{}
	for _, t := range tokens {
		if err, ok := m.failTokens[t]; ok {
			failures[t] = err
		}
	}
	return failures, nil
}

func (m *broadcastNotifier) SendDataOnly(_ context.Context, _ []string, _ map[string]string) (map[string]error, error) {
	return nil, nil
}

func (m *broadcastNotifier) SendToTopic(_ context.Context, _, _, _, _ string, _ map[string]string) (string, error) {
	return "test-msg-id", nil
}

// broadcastPushRepo emulates push_notifications, including the idempotency
// lookup keyed on (event_type, reference_id, user) — the same key the real
// repository uses. Without that emulation a reference_id collision would look
// like two happy writes here while production silently loses one.
type broadcastPushRepo struct {
	created []*domain.PushNotificationRecord
}

func pushKey(eventType, referenceID string, userID int64, userType string) string {
	return fmt.Sprintf("%s|%s|%d|%s", eventType, referenceID, userID, userType)
}

func (m *broadcastPushRepo) Create(_ context.Context, record *domain.PushNotificationRecord) error {
	m.created = append(m.created, record)
	return nil
}

func (m *broadcastPushRepo) ExistsProcessed(_ context.Context, eventType, referenceID string, userID int64, userType string) (bool, error) {
	if referenceID == "" {
		return false, nil
	}
	want := pushKey(eventType, referenceID, userID, userType)
	for _, r := range m.created {
		if pushKey(r.EventType, r.ReferenceID, r.UserID, r.UserType) == want {
			return true, nil
		}
	}
	return false, nil
}

// broadcastInboxRepo emulates notifications_inbox_dedup_idx: Create is
// ON CONFLICT DO NOTHING on (user_id, user_type, event_type, reference_id) and
// reports 0 rows affected when it dedupes. This is the assertion surface for the
// reference_id collision bug — a per-operator reference_id makes the second
// announcement to the same user vanish here.
type broadcastInboxRepo struct {
	created []*domain.InboxItem
	seen    map[string]bool
}

func inboxKey(item *domain.InboxItem) string {
	return fmt.Sprintf("%d|%s|%s|%s", item.UserID, item.UserType, item.EventType, item.ReferenceID)
}

func (m *broadcastInboxRepo) Create(_ context.Context, item *domain.InboxItem) (int64, error) {
	if m.seen == nil {
		m.seen = map[string]bool{}
	}
	if item.ReferenceID != "" && m.seen[inboxKey(item)] {
		return 0, nil
	}
	m.seen[inboxKey(item)] = true
	m.created = append(m.created, item)
	return 1, nil
}

func (m *broadcastInboxRepo) ListByUser(_ context.Context, _ int64, _ string, _ int64, _ int32, _ string) ([]*domain.InboxItem, error) {
	return nil, nil
}
func (m *broadcastInboxRepo) CountByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *broadcastInboxRepo) CountUnread(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *broadcastInboxRepo) MarkAsRead(_ context.Context, _, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *broadcastInboxRepo) MarkAllAsRead(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *broadcastInboxRepo) SoftDeleteByIDs(_ context.Context, _ []int64, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *broadcastInboxRepo) RedactChatBySender(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (m *broadcastInboxRepo) SoftDeleteByHistoricalBuyerIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (m *broadcastInboxRepo) SoftDeleteByHistoricalStoreIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

// broadcastPrefRepo answers the category preference lookup SendPushUseCase makes.
// disabled lists (userID, category) pairs the user has switched off.
type broadcastPrefRepo struct {
	disabled map[string]bool
}

func (m *broadcastPrefRepo) Upsert(_ context.Context, _ int64, _, _ string, _ bool) error {
	return nil
}
func (m *broadcastPrefRepo) GetByUser(_ context.Context, _ int64, _ string) ([]*domain.NotificationPreference, error) {
	return nil, nil
}
func (m *broadcastPrefRepo) GetByCategory(_ context.Context, userID int64, _, category string) (*domain.NotificationPreference, error) {
	key := fmt.Sprintf("%d|%s", userID, category)
	return &domain.NotificationPreference{Category: category, Enabled: !m.disabled[key]}, nil
}

type broadcastHarness struct {
	server    *Server
	tokenRepo *broadcastTokenRepo
	pushRepo  *broadcastPushRepo
	inboxRepo *broadcastInboxRepo
	prefRepo  *broadcastPrefRepo
	notifier  *broadcastNotifier
	nextTSID  int64
}

// newBroadcastHarness wires BroadcastAnnouncement against the real
// SendPushUseCase — not a stub — because everything this RPC is supposed to
// inherit (idempotency, preference gating, status/error_log, dead-token
// deactivation) lives inside that usecase. Stubbing it would test the wiring
// and nothing else.
func newBroadcastHarness(tokens []*domain.DeviceToken) *broadcastHarness {
	h := &broadcastHarness{
		tokenRepo: &broadcastTokenRepo{tokens: tokens},
		pushRepo:  &broadcastPushRepo{},
		inboxRepo: &broadcastInboxRepo{},
		prefRepo:  &broadcastPrefRepo{disabled: map[string]bool{}},
		notifier:  &broadcastNotifier{},
	}
	sendPush := usecase.NewSendPushUseCase(
		h.tokenRepo, h.pushRepo, h.prefRepo, h.notifier,
		func(err error) bool { return err != nil },
	)
	h.server = &Server{
		tokenRepo:  h.tokenRepo,
		notifier:   h.notifier,
		sendPushUC: sendPush,
		inboxUC:    usecase.NewNotificationInboxUseCase(h.inboxRepo, nil, nil, nil, nil),
		tsid:       func() int64 { h.nextTSID++; return h.nextTSID },
		logger:     zap.NewNop(),
	}
	return h
}

func (h *broadcastHarness) broadcast(t *testing.T, title, body string) *pb.BroadcastAnnouncementResponse {
	t.Helper()
	resp, err := h.server.BroadcastAnnouncement(context.Background(), &pb.BroadcastAnnouncementRequest{
		Title:          title,
		Body:           body,
		TargetUserType: "seller",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
	return resp
}

// TestBroadcastAnnouncement_WritesInboxItem asserts that a broadcast announcement
// lands in the recipient's in-app inbox, not just as a raw FCM push. A push
// banner alone is lost the moment it is dismissed or the app was closed — every
// other visible-mode notification writes both.
func TestBroadcastAnnouncement_WritesInboxItem(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
	})

	h.broadcast(t, "測試公告", "這是一則測試公告")

	if len(h.pushRepo.created) != 1 {
		t.Fatalf("expected 1 push_notifications row, got %d", len(h.pushRepo.created))
	}
	if len(h.inboxRepo.created) != 1 {
		t.Fatalf("expected 1 inbox row, got %d — BroadcastAnnouncement must write both push_notifications and the inbox", len(h.inboxRepo.created))
	}

	item := h.inboxRepo.created[0]
	if item.UserID != 2670447993 {
		t.Errorf("inbox item UserID = %d, want 2670447993", item.UserID)
	}
	if item.UserType != domain.UserTypeSeller {
		t.Errorf("inbox item UserType = %q, want %q", item.UserType, domain.UserTypeSeller)
	}
	if item.EventType != announcementEventType {
		t.Errorf("inbox item EventType = %q, want %q", item.EventType, announcementEventType)
	}
	if item.Title != "測試公告" || item.Body != "這是一則測試公告" {
		t.Errorf("inbox item title/body mismatch: got %q/%q", item.Title, item.Body)
	}
	if item.ReferenceID != h.pushRepo.created[0].ReferenceID {
		t.Errorf("inbox ReferenceID %q must match push_notifications ReferenceID %q (same idempotency key)",
			item.ReferenceID, h.pushRepo.created[0].ReferenceID)
	}
}

// TestBroadcastAnnouncement_DistinctAnnouncementsBothReachInbox pins the
// reference_id scoping bug.
//
// The old key was `broadcast_{operatorId}_{userId}` — it carried no announcement
// identity, so the same admin's second announcement to the same seller produced
// an identical reference_id. The inbox INSERT was then swallowed by
// ON CONFLICT DO NOTHING and the push row was rejected by the dedup index, with
// the repository error discarded by `_ =`. FCM had already delivered the banner,
// so the visible result was an announcement that flashed once and existed
// nowhere afterwards: no inbox row, no DB record, no error anywhere.
//
// Mutation check: make the broadcast id a constant (tsid: func() int64 { return 1 })
// and this test goes red on the second announcement.
func TestBroadcastAnnouncement_DistinctAnnouncementsBothReachInbox(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
	})

	h.broadcast(t, "第一則公告", "內容一")
	h.broadcast(t, "第二則公告", "內容二")

	if len(h.inboxRepo.created) != 2 {
		t.Fatalf("expected 2 inbox rows for 2 distinct announcements, got %d — "+
			"reference_id must be scoped per announcement, not per operator", len(h.inboxRepo.created))
	}
	if a, b := h.inboxRepo.created[0].ReferenceID, h.inboxRepo.created[1].ReferenceID; a == b {
		t.Fatalf("two announcements shared reference_id %q — the second one would be deduped away", a)
	}
	if got := h.inboxRepo.created[1].Title; got != "第二則公告" {
		t.Errorf("second inbox row Title = %q, want 第二則公告", got)
	}
	if len(h.pushRepo.created) != 2 {
		t.Fatalf("expected 2 push_notifications rows, got %d", len(h.pushRepo.created))
	}
}

// TestBroadcastAnnouncement_RedeliveryOfSameAnnouncementIsDeduped is the other
// half of the previous test: scoping the reference_id per announcement must not
// cost idempotency. Two deliveries of the SAME announcement to the same user
// still collapse to one row.
func TestBroadcastAnnouncement_RedeliveryOfSameAnnouncementIsDeduped(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-2", IsActive: true},
	})

	h.broadcast(t, "公告", "內容")

	if len(h.inboxRepo.created) != 1 {
		t.Fatalf("a user with 2 devices must still get exactly 1 inbox row, got %d", len(h.inboxRepo.created))
	}
	if len(h.pushRepo.created) != 1 {
		t.Fatalf("expected 1 push_notifications row, got %d", len(h.pushRepo.created))
	}
	if n := len(h.notifier.sent); n != 1 {
		t.Fatalf("expected 1 FCM call carrying both tokens, got %d calls", n)
	}
	if n := len(h.notifier.sent[0]); n != 2 {
		t.Errorf("FCM call carried %d tokens, want 2 (both of the user's devices)", n)
	}
}

// TestBroadcastAnnouncement_SkipsUserWhoDisabledSystemCategory asserts the
// announcement respects the recipient's 「系統通知」 switch (13 號設計書 §9 lists
// system announcements under the `system` category and both Apps expose the
// toggle). The old hand-rolled loop called notifier.Send directly and never
// consulted prefRepo, so muting the category did nothing.
//
// The inbox row is still written on purpose — the preference gates the push, not
// the durable copy, exactly as handleTemplate does for every other template.
//
// Mutation check: drop `Category: domain.CategorySystem` from the
// SendPushRequest and this test goes red (Category=="" is fail-open).
func TestBroadcastAnnouncement_SkipsUserWhoDisabledSystemCategory(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
	})
	h.prefRepo.disabled[fmt.Sprintf("%d|%s", int64(2670447993), domain.CategorySystem)] = true

	h.broadcast(t, "公告", "內容")

	if len(h.notifier.sent) != 0 {
		t.Fatalf("expected no FCM send for a user who muted the system category, got %d calls", len(h.notifier.sent))
	}
	if len(h.pushRepo.created) != 0 {
		t.Fatalf("expected no push_notifications row for a muted user, got %d", len(h.pushRepo.created))
	}
	if len(h.inboxRepo.created) != 1 {
		t.Fatalf("expected the inbox row to still be written (preference gates the push, not the inbox), got %d", len(h.inboxRepo.created))
	}
}

// TestBroadcastAnnouncement_PartialFCMFailureIsRecorded asserts the push row
// carries the real delivery outcome. The old loop hard-coded StatusSent and left
// error_log empty forever, so a device that had been failing for weeks looked
// identical in the DB to one that received everything — and the dead token was
// never deactivated.
//
// Mutation check: revert to writing the record via pushRepo.Create with
// Status: domain.StatusSent and this test goes red on both the status and the
// deactivation assertion.
func TestBroadcastAnnouncement_PartialFCMFailureIsRecorded(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "dead-token", IsActive: true},
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "live-token", IsActive: true},
	})
	h.notifier.failTokens = map[string]error{"dead-token": errors.New("registration-token-not-registered")}

	h.broadcast(t, "公告", "內容")

	if len(h.pushRepo.created) != 1 {
		t.Fatalf("expected 1 push_notifications row, got %d", len(h.pushRepo.created))
	}
	rec := h.pushRepo.created[0]
	if rec.Status == domain.StatusSent {
		t.Errorf("status = %q for a partially failed send — must not be hard-coded to sent", rec.Status)
	}
	if rec.Status != domain.StatusPartial {
		t.Errorf("status = %q, want %q", rec.Status, domain.StatusPartial)
	}
	if rec.ErrorLog == "" {
		t.Error("error_log is empty for a failed token — the failure reason must be recorded")
	}
	if len(h.tokenRepo.deactivated) != 1 || h.tokenRepo.deactivated[0] != "dead-token" {
		t.Errorf("deactivated tokens = %v, want [dead-token]", h.tokenRepo.deactivated)
	}
}

// TestBroadcastAnnouncement_DataCarriesCategoryAndType asserts the FCM/inbox data
// map carries `category`. The inbox list API filters on data->>'category', so an
// announcement without it is invisible to every category-filtered query — it is
// stored, it just cannot be found.
//
// Mutation check: remove the `"category"` entry from the data map and this test
// goes red.
func TestBroadcastAnnouncement_DataCarriesCategoryAndType(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
	})

	h.broadcast(t, "公告", "內容")

	if len(h.inboxRepo.created) != 1 {
		t.Fatalf("expected 1 inbox row, got %d", len(h.inboxRepo.created))
	}
	data := h.inboxRepo.created[0].Data
	if got := data["category"]; got != domain.CategorySystem {
		t.Errorf("data[category] = %q, want %q — the inbox list API filters on data->>'category'",
			got, domain.CategorySystem)
	}
	if got := data["type"]; got != "SYSTEM_ANNOUNCEMENT" {
		t.Errorf("data[type] = %q, want SYSTEM_ANNOUNCEMENT", got)
	}
	if data["announcementId"] == "" {
		t.Error("data[announcementId] is empty — the broadcast id must be exposed to the clients")
	}
	if len(h.pushRepo.created) != 1 {
		t.Fatalf("expected 1 push_notifications row, got %d", len(h.pushRepo.created))
	}
	if h.pushRepo.created[0].Data["category"] != domain.CategorySystem {
		t.Errorf("push record data[category] = %q, want %q",
			h.pushRepo.created[0].Data["category"], domain.CategorySystem)
	}
}

// TestBroadcastAnnouncement_ResponseCarriesIdAndCount asserts the response hands
// the caller the announcement's identity and reach as structured fields.
//
// Both values already existed inside the RPC — the id as the reference_id / FCM
// data.announcementId, the count only inside the Chinese sentence "已推播給 N 位
// 使用者". A caller wanting either had to parse that string, and the id was not
// recoverable at all, so an admin could not look up the announcement they had
// just sent.
//
// The id is asserted against data.announcementId rather than a literal: those two
// must be the same announcement, and pinning a literal would pass even if the
// response reported a different broadcast than the one that was delivered.
//
// Mutation check: drop BroadcastId (or RecipientCount) from the returned
// BroadcastAnnouncementResponse and this test goes red.
func TestBroadcastAnnouncement_ResponseCarriesIdAndCount(t *testing.T) {
	h := newBroadcastHarness([]*domain.DeviceToken{
		{UserID: 2670447993, UserType: domain.UserTypeSeller, Token: "fcm-token-1", IsActive: true},
		{UserID: 2670447994, UserType: domain.UserTypeSeller, Token: "fcm-token-2", IsActive: true},
	})

	resp := h.broadcast(t, "公告", "內容")

	if resp.RecipientCount != "2" {
		t.Errorf("RecipientCount = %q, want \"2\"", resp.RecipientCount)
	}
	if resp.BroadcastId == "" {
		t.Fatal("BroadcastId is empty — the caller cannot look up the announcement it just sent")
	}
	if len(h.inboxRepo.created) == 0 {
		t.Fatal("expected inbox rows")
	}
	if got := h.inboxRepo.created[0].Data["announcementId"]; got != resp.BroadcastId {
		t.Errorf("BroadcastId = %q but the delivered announcementId = %q — the response must "+
			"identify the announcement that was actually delivered", resp.BroadcastId, got)
	}
}
