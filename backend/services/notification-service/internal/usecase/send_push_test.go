package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

// neverDeactivate is the default token policy for tests that don't exercise
// deactivation (most). Deactivation is tested explicitly with an injected policy.
func neverDeactivate(error) bool { return false }

func newSendPushUC(tokenRepo *mockTokenRepo, pushRepo *mockPushRepo, notifier *mockNotifier) *SendPushUseCase {
	return NewSendPushUseCase(tokenRepo, pushRepo, &mockPrefRepo{}, notifier, neverDeactivate)
}

func baseReq() *dto.SendPushRequest {
	return &dto.SendPushRequest{
		UserID:      1,
		UserType:    "buyer",
		EventType:   "order_paid",
		ReferenceID: "order-123",
		Title:       "訂單已付款",
		Body:        "您的訂單已成功付款",
		Data:        map[string]string{"order_id": "123"},
	}
}

func TestSendPushUseCase_Idempotency(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{existsProcessed: true}
	tokenRepo := &mockTokenRepo{}
	notifier := &mockNotifier{}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	// tokenRepo.GetActiveByUser should NOT have been called
	if pushRepo.createdRecord != nil {
		t.Fatal("should not have created a push record for duplicate event")
	}
	// 去重必須以「身分 (event_type, reference_id, user)」為 key，含 recipient 維度
	if pushRepo.gotUserID != 1 || pushRepo.gotUserType != "buyer" ||
		pushRepo.gotEventType != "order_paid" || pushRepo.gotReferenceID != "order-123" {
		t.Errorf("ExistsProcessed called with wrong identity: type=%q ref=%q uid=%d utype=%q",
			pushRepo.gotEventType, pushRepo.gotReferenceID, pushRepo.gotUserID, pushRepo.gotUserType)
	}
}

func TestSendPushUseCase_IdempotencyCheckError_Continues(t *testing.T) {
	ctx := context.Background()

	// ExistsProcessed 回傳 error → 應繼續嘗試送出（不應 return error）
	pushRepo := &mockPushRepo{
		existsProcessed: false,
		existsErr:       errors.New("redis timeout"),
	}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{{Token: "tok1"}},
	}
	notifier := &mockNotifier{failures: map[string]error{}}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestSendPushUseCase_NoActiveTokens(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokens: []*domain.DeviceToken{}}
	notifier := &mockNotifier{}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord == nil {
		t.Fatal("expected a push record to be created")
	}
	if pushRepo.createdRecord.Status != domain.StatusSkipped {
		t.Errorf("expected status %q, got %q", domain.StatusSkipped, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_GetActiveTokensError(t *testing.T) {
	ctx := context.Background()

	dbErr := errors.New("db error")
	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokensErr: dbErr}
	notifier := &mockNotifier{}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	err := uc.Execute(ctx, baseReq())
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected %v, got %v", dbErr, err)
	}
}

func TestSendPushUseCase_AllTokensSucceed(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{
			{Token: "tok1"},
			{Token: "tok2"},
		},
	}
	notifier := &mockNotifier{failures: map[string]error{}} // 無失敗

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord.Status != domain.StatusSent {
		t.Errorf("expected status %q, got %q", domain.StatusSent, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_PartialFailure(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{
			{Token: "tok1"},
			{Token: "tok2"},
		},
	}
	notifier := &mockNotifier{
		failures: map[string]error{"tok2": errors.New("some error")},
	}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord.Status != domain.StatusPartial {
		t.Errorf("expected status %q, got %q", domain.StatusPartial, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_PartialFailure_RecordsErrorLog(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{{Token: "tok1"}, {Token: "tok2"}},
	}
	// tok2 失敗（典型殭屍 token：SenderId mismatch）
	notifier := &mockNotifier{failures: map[string]error{"tok2": errors.New("SenderId mismatch")}}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	rec := pushRepo.createdRecord
	if rec == nil || rec.Status != domain.StatusPartial {
		t.Fatalf("expected a partial record, got %+v", rec)
	}
	// partial 必須留下失敗原因，否則 DB 裡上千筆 partial 全無 error_log、無法診斷死 token
	if rec.ErrorLog == "" {
		t.Fatal("expected partial push to record failure reason in ErrorLog, got empty")
	}
	if !strings.Contains(rec.ErrorLog, "SenderId mismatch") {
		t.Errorf("expected ErrorLog to contain the FCM failure reason, got %q", rec.ErrorLog)
	}
}

func TestSendPushUseCase_AllTokensFail(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{
			{Token: "tok1"},
		},
	}
	notifier := &mockNotifier{
		failures: map[string]error{"tok1": errors.New("delivery failed")},
	}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord.Status != domain.StatusFailed {
		t.Errorf("expected status %q, got %q", domain.StatusFailed, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_FCMSendError(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{{Token: "tok1"}},
	}
	notifier := &mockNotifier{err: errors.New("FCM connection refused")}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	// FCM 整體失敗不應向上傳播（已記錄），回傳 nil
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord.Status != domain.StatusFailed {
		t.Errorf("expected status %q, got %q", domain.StatusFailed, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_InvalidTokenDeactivated(t *testing.T) {
	ctx := context.Background()

	var deactivatedToken string
	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{
			{Token: "valid-tok"},
			{Token: "invalid-tok"},
		},
		deactivateByTokenFn: func(token string) error {
			deactivatedToken = token
			return nil
		},
	}
	permanentErr := errors.New("permanent: token gone")
	notifier := &mockNotifier{
		failures: map[string]error{"invalid-tok": permanentErr},
	}

	// 注入的 policy 判定該 token error 為「永久失效」→ usecase 應停用它。
	// 證明停用決策走注入的 TokenDeactivationPolicy，而非 hardcode 的 infra 呼叫。
	deactivate := func(e error) bool { return errors.Is(e, permanentErr) }
	uc := NewSendPushUseCase(tokenRepo, pushRepo, &mockPrefRepo{}, notifier, deactivate)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if deactivatedToken != "invalid-tok" {
		t.Errorf("expected invalid-tok to be deactivated, got %q", deactivatedToken)
	}
	if pushRepo.createdRecord.Status != domain.StatusPartial {
		t.Errorf("expected status %q, got %q", domain.StatusPartial, pushRepo.createdRecord.Status)
	}
}

// policy 回 false（transient / APNs config / 不確定）時，token 不可被停用——
// 避免一個壞 payload 或 APNs 憑證窗口把健康 token 大量誤殺。
func TestSendPushUseCase_NonDeactivatingError_KeepsToken(t *testing.T) {
	var deactivated bool
	tokenRepo := &mockTokenRepo{
		activeTokens:        []*domain.DeviceToken{{Token: "tok1"}, {Token: "tok2"}},
		deactivateByTokenFn: func(string) error { deactivated = true; return nil },
	}
	notifier := &mockNotifier{failures: map[string]error{"tok2": errors.New("UNAVAILABLE")}}

	uc := NewSendPushUseCase(tokenRepo, &mockPushRepo{}, &mockPrefRepo{}, notifier, neverDeactivate)
	if err := uc.Execute(context.Background(), baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if deactivated {
		t.Error("token must NOT be deactivated when policy returns false (transient/config error)")
	}
}

func TestSendPushUseCase_PreferenceDisabled_Skips(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "tok1"}}}
	notifier := &mockNotifier{failures: map[string]error{}}
	prefRepo := &mockPrefRepo{
		prefByCat: &domain.NotificationPreference{Category: "follower", Enabled: false},
	}
	uc := NewSendPushUseCase(tokenRepo, pushRepo, prefRepo, notifier, neverDeactivate)

	req := &dto.SendPushRequest{
		UserID: 1, UserType: "seller", EventType: "follow.added",
		Category: "follower", // 由 worker 從 template.Category 帶入
		Title:    "新粉絲", Body: "有人追蹤你了",
	}
	if err := uc.Execute(ctx, req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pushRepo.createdRecord != nil {
		t.Errorf("expected no push record, got status=%q", pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_PreferenceEnabled_Sends(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "tok1"}}}
	notifier := &mockNotifier{failures: map[string]error{}}
	prefRepo := &mockPrefRepo{
		prefByCat: &domain.NotificationPreference{Category: "follower", Enabled: true},
	}
	uc := NewSendPushUseCase(tokenRepo, pushRepo, prefRepo, notifier, neverDeactivate)

	req := &dto.SendPushRequest{
		UserID: 1, UserType: "seller", EventType: "follow.added",
		Category: "follower", // 由 worker 從 template.Category 帶入
		Title:    "新粉絲", Body: "有人追蹤你了",
	}
	if err := uc.Execute(ctx, req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pushRepo.createdRecord == nil {
		t.Fatal("expected push record to be created")
	}
	if pushRepo.createdRecord.Status != domain.StatusSent {
		t.Errorf("expected status %q, got %q", domain.StatusSent, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_EmptyCategory_AlwaysSends(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "tok1"}}}
	notifier := &mockNotifier{failures: map[string]error{}}
	// prefRepo 不會被查詢，但 GetByCategory 若被誤呼叫會回傳 disabled
	prefRepo := &mockPrefRepo{
		prefByCat: &domain.NotificationPreference{Category: "follower", Enabled: false},
	}
	uc := NewSendPushUseCase(tokenRepo, pushRepo, prefRepo, notifier, neverDeactivate)

	// Category 為空 = fail-open，不查 preference，直接送
	req := &dto.SendPushRequest{
		UserID: 1, UserType: "seller", EventType: "unknown.event.type",
		Category: "",
		Title:    "測試", Body: "測試內容",
	}
	if err := uc.Execute(ctx, req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pushRepo.createdRecord == nil {
		t.Fatal("expected push record to be created when category is empty")
	}
	if pushRepo.createdRecord.Status != domain.StatusSent {
		t.Errorf("expected status %q, got %q", domain.StatusSent, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_NoReferenceID_SkipsIdempotencyCheck(t *testing.T) {
	ctx := context.Background()

	// ExistsProcessed 不應被呼叫（referenceID 為空）
	pushRepo := &mockPushRepo{existsProcessed: true} // 即使回傳 true 也應繼續
	tokenRepo := &mockTokenRepo{
		activeTokens: []*domain.DeviceToken{{Token: "tok1"}},
	}
	notifier := &mockNotifier{failures: map[string]error{}}

	req := baseReq()
	req.ReferenceID = ""

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if pushRepo.createdRecord.Status != domain.StatusSent {
		t.Errorf("expected status %q, got %q", domain.StatusSent, pushRepo.createdRecord.Status)
	}
}

// capturingPrefRepo 實作 domain.NotificationPreferenceRepository，
// 記錄 GetByCategory 是否被呼叫，以及模擬指定 category 為停用或回傳錯誤。
type capturingPrefRepo struct {
	disabled    map[string]bool
	queried     bool
	returnError error
}

func (r *capturingPrefRepo) Upsert(_ context.Context, _ int64, _, _ string, _ bool) error {
	return nil
}
func (r *capturingPrefRepo) GetByUser(_ context.Context, _ int64, _ string) ([]*domain.NotificationPreference, error) {
	return nil, nil
}
func (r *capturingPrefRepo) GetByCategory(_ context.Context, _ int64, _, category string) (*domain.NotificationPreference, error) {
	r.queried = true
	if r.returnError != nil {
		return nil, r.returnError
	}
	if r.disabled[category] {
		return &domain.NotificationPreference{Enabled: false}, nil
	}
	return nil, nil
}

func TestSendPushUseCase_UsesRequestCategoryNotMap(t *testing.T) {
	tokens := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "t1"}}}
	pushRepo := &mockPushRepo{}
	prefRepo := &capturingPrefRepo{disabled: map[string]bool{"order_update": true}}
	notifier := &mockNotifier{}
	uc := NewSendPushUseCase(tokens, pushRepo, prefRepo, notifier, neverDeactivate)

	// req.Category=order_update，使用者停用此 category → 應略過推播
	err := uc.Execute(context.Background(), &dto.SendPushRequest{
		UserID:    1,
		UserType:  "buyer",
		EventType: "order.paid",
		Category:  "order_update",
		Title:     "t", Body: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prefRepo.queried {
		t.Fatal("expected pref check using req.Category")
	}
	if pushRepo.createdRecord != nil {
		t.Fatal("expected push skipped due to disabled category")
	}
}

func TestSendPushUseCase_EmptyCategoryFailOpen(t *testing.T) {
	tokens := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "t1"}}}
	pushRepo := &mockPushRepo{}
	prefRepo := &capturingPrefRepo{}
	notifier := &mockNotifier{failures: map[string]error{}}
	uc := NewSendPushUseCase(tokens, pushRepo, prefRepo, notifier, neverDeactivate)

	err := uc.Execute(context.Background(), &dto.SendPushRequest{
		UserID: 1, UserType: "buyer", EventType: "x.y",
		Category: "", // 空字串 = fail-open
		Title:    "t", Body: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prefRepo.queried {
		t.Fatal("expected NO pref check when Category empty")
	}
	if pushRepo.createdRecord == nil || pushRepo.createdRecord.Status != domain.StatusSent {
		t.Fatalf("expected push sent with status=%q", domain.StatusSent)
	}
}

func TestSendPushUseCase_PrefRepoError_FailsOpen(t *testing.T) {
	// I4: prefRepo.GetByCategory 回傳錯誤時，應 fail-open（繼續送出推播）
	tokens := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "t1"}}}
	pushRepo := &mockPushRepo{}
	prefRepo := &capturingPrefRepo{returnError: errors.New("db down")}
	notifier := &mockNotifier{failures: map[string]error{}}
	uc := NewSendPushUseCase(tokens, pushRepo, prefRepo, notifier, neverDeactivate)

	err := uc.Execute(context.Background(), &dto.SendPushRequest{
		UserID: 1, UserType: "buyer", EventType: "x.y",
		Category: "order_update",
		Title:    "t", Body: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prefRepo.queried {
		t.Fatal("expected pref check attempted")
	}
	if pushRepo.createdRecord == nil {
		t.Fatal("expected push sent (fail-open on pref repo error)")
	}
	if pushRepo.createdRecord.Status != domain.StatusSent {
		t.Fatalf("expected status %q (fail-open), got %q", domain.StatusSent, pushRepo.createdRecord.Status)
	}
}

func TestSendPushUseCase_PushDisabled_RecordedAsSkipped(t *testing.T) {
	ctx := context.Background()

	pushRepo := &mockPushRepo{}
	tokenRepo := &mockTokenRepo{activeTokens: []*domain.DeviceToken{{Token: "tok-1"}}}
	notifier := &mockNotifier{err: fmt.Errorf("wrapped: %w", domain.ErrPushDisabled)}

	uc := newSendPushUC(tokenRepo, pushRepo, notifier)
	if err := uc.Execute(ctx, baseReq()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pushRepo.createdRecord == nil {
		t.Fatal("expected a push record to be created")
	}
	if pushRepo.createdRecord.Status != domain.StatusSkipped {
		t.Errorf("disabled push: status = %q, want %q", pushRepo.createdRecord.Status, domain.StatusSkipped)
	}
	if !strings.Contains(pushRepo.createdRecord.ErrorLog, "push disabled") {
		t.Errorf("disabled push: error log = %q, want the disabled reason", pushRepo.createdRecord.ErrorLog)
	}
}
