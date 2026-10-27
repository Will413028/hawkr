package sellerrelay

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type recordingJobs struct {
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

type fakeStores struct {
	state seller.Store
	err   error
	asked []int64
}

func (s *fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	s.asked = append(s.asked, id)
	return s.state, s.err
}

func riskStore(ground seller.RiskGround) seller.Store {
	return seller.Store{Sanctions: seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk, RiskGround: &ground})}
}

var (
	eventID    = uuid.MustParse("01990000-0000-7000-8000-000000000001")
	occurredAt = time.Date(2026, 10, 5, 9, 30, 15, 123456789, time.FixedZone("TST", 8*3600))
	// timestamp 是 Go time.Time 的 JSON：UTC、RFC3339Nano。
	wireTimestamp = "2026-10-05T01:30:15.123456789Z"
)

// run 把模組事件交給 OnX，取出排入的 job，再交給 worker，回傳送出的 job 與解成 map 的 body。
func run[E any](t *testing.T, stores Stores, on func(jobs.Enqueuer) func(context.Context, E) error, event E) (rabbitmq.PublishArgs, map[string]any) {
	t.Helper()
	relayed := &recordingJobs{}
	if err := on(relayed)(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(relayed.args) != 1 {
		t.Fatalf("relay jobs=%d", len(relayed.args))
	}
	published := &recordingJobs{}
	w := NewWorker(stores, published, nil)
	if err := work(w, relayed.args[0]); err != nil {
		t.Fatal(err)
	}
	if len(published.args) != 1 {
		t.Fatalf("published jobs=%d", len(published.args))
	}
	a, ok := published.args[0].(rabbitmq.PublishArgs)
	if !ok {
		t.Fatalf("job type=%T", published.args[0])
	}
	if published.opts[0] == nil || published.opts[0].MaxAttempts != rabbitmq.InsertOpts().MaxAttempts {
		t.Fatalf("publish opts=%+v", published.opts[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(a.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if a.EventID != eventID.String() || a.EventID != payload["event_id"] {
		t.Fatalf("PublishArgs.EventID=%q body event_id=%v, want the module event id %s", a.EventID, payload["event_id"], eventID)
	}
	return a, payload
}

// work 以 relay job 的 args 呼叫對應的 worker 方法。
func work(w *Worker, args river.JobArgs) error {
	ctx := context.Background()
	switch a := args.(type) {
	case StoreRetiredArgs:
		return w.storeRetired(ctx, a)
	case StoreSuspendedArgs:
		return w.storeSuspended(ctx, a)
	case StoreUnsuspendedArgs:
		return w.storeUnsuspended(ctx, a)
	case StoreProfileChangedArgs:
		return w.storeProfileChanged(ctx, a)
	case StoreExposureChangedArgs:
		return w.storeExposureChanged(ctx, a)
	case PayUniForwardedArgs:
		return w.payUniForwarded(ctx, a)
	case PayUniApprovedArgs:
		return w.payUniApproved(ctx, a)
	case PayUniPendingInfoArgs:
		return w.payUniPendingInfo(ctx, a)
	case PayUniUnderReviewArgs:
		return w.payUniUnderReview(ctx, a)
	case PayUniRejectedArgs:
		return w.payUniRejected(ctx, a)
	case KYCRejectedArgs:
		return w.kycRejected(ctx, a)
	}
	panic("unknown relay job")
}

func check(t *testing.T, name string, a rabbitmq.PublishArgs, payload map[string]any, exchange, key string, want map[string]any) {
	t.Helper()
	if a.Exchange != exchange || a.RoutingKey != key {
		t.Errorf("%s: published to %s/%s, want %s/%s", name, a.Exchange, a.RoutingKey, exchange, key)
	}
	if !reflect.DeepEqual(payload, want) {
		t.Errorf("%s: payload\n got  %#v\n want %#v", name, payload, want)
	}
}

// 每個 routing key 一個測試，斷言完整的 JSON（解成 map[string]any，所以 string 與 number 分得清楚），對照舊 publisher 的 struct
// （pkg/mq/events.go、seller-service 的 merchant_verification_outbox；wire_test.go 另外逐欄位比對 struct 的 tag）。
func TestSellerDeletedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnStoreRetired, seller.StoreRetired{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, StoreCode: 26100000091, AccountID: 7})
	check(t, "seller.deleted", a, got, "hawkr.seller", "seller.deleted", map[string]any{
		"event_id": eventID.String(), "event_type": "SELLER_DELETED", "version": "1.0", "timestamp": wireTimestamp, "source": "seller-service",
		"accountId": float64(7), "store_id": float64(91),
	})
}

func TestSellerSuspendedWireForARiskSuspension(t *testing.T) {
	stores := &fakeStores{state: riskStore(seller.RiskFulfillment)}
	a, got := run(t, stores, OnStoreSuspended, seller.StoreSuspended{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, PeriodID: 55})
	check(t, "seller.suspended", a, got, "hawkr.seller", "seller.suspended", map[string]any{
		"event_id": eventID.String(), "event_type": "SELLER_SUSPENDED", "version": "1.0", "timestamp": wireTimestamp, "source": "seller-service",
		"suspensionId": "55", "accountId": "7", "store_id": "91", "reasonCode": "9603", "reasonLabel": "履約",
	})
	if len(stores.asked) != 1 || stores.asked[0] != 91 {
		t.Fatalf("the reason must be read from the store at relay time: %v", stores.asked)
	}
}

// 只依據認定的停權（或 relay 時已沒有生效中的措施）：reasonCode 是空字串（不省略）、不帶 reasonLabel，事件照樣送出。
func TestSellerSuspendedWireWithoutARiskSuspension(t *testing.T) {
	category := policy.Fraud
	governance := seller.Store{Sanctions: seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisViolation, LeadCategory: &category})}
	for name, state := range map[string]seller.Store{"只有認定的停權": governance, "relay 時已沒有生效中的措施": {Sanctions: seller.NoSanctions()}} {
		a, got := run(t, &fakeStores{state: state}, OnStoreSuspended, seller.StoreSuspended{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, PeriodID: 55})
		check(t, name, a, got, "hawkr.seller", "seller.suspended", map[string]any{
			"event_id": eventID.String(), "event_type": "SELLER_SUSPENDED", "version": "1.0", "timestamp": wireTimestamp, "source": "seller-service",
			"suspensionId": "55", "accountId": "7", "store_id": "91", "reasonCode": "",
		})
	}
}

// 讀到的店家沒有推導過停權狀態（Sanctions 是 nil）時不送出：重試，不當成沒有原因的停權。
func TestSellerSuspendedWithAnUnderivedStoreIsRetriedNotPublished(t *testing.T) {
	relayed := &recordingJobs{}
	if err := OnStoreSuspended(relayed)(context.Background(), seller.StoreSuspended{EventID: eventID, StoreID: 91, AccountID: 7, PeriodID: 55}); err != nil {
		t.Fatal(err)
	}
	published := &recordingJobs{}
	w := NewWorker(&fakeStores{}, published, nil)
	if err := work(w, relayed.args[0]); err == nil || len(published.args) != 0 {
		t.Fatalf("err=%v published=%d", err, len(published.args))
	}
}

func TestSellerSuspendedReadFailureIsRetriedNotPublishedWithoutAReason(t *testing.T) {
	relayed := &recordingJobs{}
	if err := OnStoreSuspended(relayed)(context.Background(), seller.StoreSuspended{EventID: eventID, StoreID: 91, AccountID: 7, PeriodID: 55}); err != nil {
		t.Fatal(err)
	}
	published := &recordingJobs{}
	w := NewWorker(&fakeStores{err: context.DeadlineExceeded}, published, nil)
	if err := work(w, relayed.args[0]); err == nil || len(published.args) != 0 {
		t.Fatalf("err=%v published=%d", err, len(published.args))
	}
}

func TestSellerUnsuspendedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnStoreUnsuspended, seller.StoreUnsuspended{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, PeriodID: 55})
	check(t, "seller.unsuspended", a, got, "hawkr.seller", "seller.unsuspended", map[string]any{
		"event_id": eventID.String(), "event_type": "SELLER_UNSUSPENDED", "version": "1.0", "timestamp": wireTimestamp, "source": "seller-service",
		"suspensionId": "55", "accountId": "7", "store_id": "91",
	})
}

// 沒有 BaseEvent；store_id 是字串，occurred_at 是秒精度的 UTC。
func TestSellerProfileChangedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnStoreProfileChanged, seller.StoreProfileChanged{OccurredAt: occurredAt, EventID: eventID, StoreID: 91})
	check(t, "seller.profile.changed", a, got, "hawkr.seller", "seller.profile.changed", map[string]any{
		"event_id": eventID.String(), "event_type": "SELLER_PROFILE_CHANGED", "store_id": "91", "occurred_at": "2026-10-05T01:30:15Z",
	})
}

func TestSellerExposureChangedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnStoreExposureChanged, seller.StoreExposureChanged{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, Exposable: true, Version: 4})
	check(t, "seller.exposure.changed", a, got, "hawkr.seller", "seller.exposure.changed", map[string]any{
		"event_id": eventID.String(), "event_type": "SELLER_EXPOSURE_CHANGED", "version": "1.0", "timestamp": wireTimestamp, "source": "seller-service",
		"store_id": float64(91), "exposable": true, "exposure_version": float64(4),
	})
	_, got = run(t, &fakeStores{}, OnStoreExposureChanged, seller.StoreExposureChanged{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, Exposable: false, Version: 5})
	if got["exposable"] != false || got["exposure_version"] != float64(5) {
		t.Fatalf("a false announcement must still carry exposable=false: %#v", got)
	}
}

func TestMerchantVerificationForwardedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnPayUniForwarded, seller.PayUniForwarded{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7})
	check(t, "forwarded", a, got, "hawkr.merchant", "merchant.verification.forwarded", map[string]any{
		"event_id": eventID.String(), "status": "forwarded_to_payuni", "merchant_id": float64(7), "store_id": float64(91),
	})
}

func TestMerchantVerificationApprovedWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnPayUniApproved, seller.PayUniApproved{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, Reason: "審核通過"})
	check(t, "approved", a, got, "hawkr.merchant", "merchant.verification.approved", map[string]any{
		"event_id": eventID.String(), "status": "approved", "reason": "審核通過", "merchant_id": float64(7), "store_id": float64(91),
	})
	_, got = run(t, &fakeStores{}, OnPayUniApproved, seller.PayUniApproved{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7})
	if _, has := got["reason"]; has {
		t.Fatalf("a blank reason is omitted: %#v", got)
	}
}

func TestMerchantVerificationPendingInfoWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnPayUniPendingInfo, seller.PayUniPendingInfo{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, Reason: "請補身分證背面"})
	check(t, "pending_info", a, got, "hawkr.merchant", "merchant.verification.pending_info", map[string]any{
		"event_id": eventID.String(), "status": "pending_info", "reason": "請補身分證背面", "merchant_id": float64(7), "store_id": float64(91),
	})
	for _, blank := range []string{"", "  \n"} {
		a, got = run(t, &fakeStores{}, OnPayUniPendingInfo, seller.PayUniPendingInfo{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, Reason: blank})
		check(t, "pending_info default", a, got, "hawkr.merchant", "merchant.verification.pending_info", map[string]any{
			"event_id": eventID.String(), "status": "pending_info", "reason": "請查收 PayUni 寄送的通知信，並依指示至 PayUni 會員後台填寫會員資料",
			"merchant_id": float64(7), "store_id": float64(91),
		})
	}
}

func TestMerchantVerificationUnderReviewWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnPayUniUnderReview, seller.PayUniUnderReview{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7})
	check(t, "under_review", a, got, "hawkr.merchant", "merchant.verification.under_review", map[string]any{
		"event_id": eventID.String(), "status": "under_review", "merchant_id": float64(7), "store_id": float64(91),
	})
}

// PayUni 拒絕：非空白的審核意見（去掉前後空白）優先；沒有時依狀態碼取目錄的文案，目錄沒有的碼用通用文案，沒有狀態碼沒有文案。
func TestMerchantVerificationRejectedByPayUniWire(t *testing.T) {
	for name, c := range map[string]struct {
		code, reason string
		want         any
	}{
		"審核意見優先":    {"MER_8", "  資料不符  ", "資料不符"},
		"MER_8":     {"MER_8", "", "金流申請未通過審核，請確認資料後重新提交"},
		"MER_2":     {"MER_2", " ", "這家商店的金流服務已關閉，請聯繫客服協助確認"},
		"MEMBER_92": {"MEMBER_92", "", "金流會員審核未通過，請確認個人或公司資料後重新提交"},
		"MEMBER_99": {"MEMBER_99", "", "金流會員帳號已停權，請聯繫客服協助確認"},
		"目錄沒有的碼":    {"MER_7", "", "申請未通過，請聯繫客服協助確認"},
		"目錄沒有的碼加意見": {"MER_7", "停用", "停用"},
		"沒有狀態碼":     {"", "停用", nil},
	} {
		a, got := run(t, &fakeStores{}, OnPayUniRejected, seller.PayUniRejected{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, Reason: c.reason, StatusCode: c.code})
		want := map[string]any{"event_id": eventID.String(), "status": "rejected", "merchant_id": float64(7), "store_id": float64(91)}
		if c.want != nil {
			want["reason"] = c.want
		}
		check(t, name, a, got, "hawkr.merchant", "merchant.verification.rejected", want)
	}
}

func TestMerchantVerificationRejectedByAnAdminWire(t *testing.T) {
	a, got := run(t, &fakeStores{}, OnKYCRejected, seller.KYCRejected{OccurredAt: occurredAt, EventID: eventID, StoreID: 91, AccountID: 7, Reason: "證件模糊"})
	check(t, "kyc rejected", a, got, "hawkr.merchant", "merchant.verification.rejected", map[string]any{
		"event_id": eventID.String(), "status": "rejected", "reason": "證件模糊", "merchant_id": float64(7), "store_id": float64(91),
	})
}

// OnX 把事件交給 jobs.Enqueuer（呼叫方在發布的 transaction 內），失敗時回錯誤讓整筆回滾。
func TestOnXReturnsTheEnqueueError(t *testing.T) {
	q := failingJobs{}
	if err := OnStoreExposureChanged(q)(context.Background(), seller.StoreExposureChanged{EventID: eventID}); err == nil {
		t.Fatal("an enqueue failure must fail the publishing transaction")
	}
}

type failingJobs struct{}

func (failingJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error {
	return context.Canceled
}

func TestKindsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range []river.JobArgs{
		StoreRetiredArgs{}, StoreSuspendedArgs{}, StoreUnsuspendedArgs{}, StoreProfileChangedArgs{}, StoreExposureChangedArgs{},
		PayUniForwardedArgs{}, PayUniApprovedArgs{}, PayUniPendingInfoArgs{}, PayUniUnderReviewArgs{}, PayUniRejectedArgs{}, KYCRejectedArgs{},
	} {
		if seen[a.Kind()] {
			t.Errorf("duplicate kind %s", a.Kind())
		}
		seen[a.Kind()] = true
	}
	if len(seen) != 11 {
		t.Fatalf("kinds=%d", len(seen))
	}
}
