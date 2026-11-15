package delegation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 拒絕路徑的單元測試：session 與能力、店家狀態的拒絕發生在任何資料庫存取之前，所以用替身的 seller 與 FAILED 紀錄器就能驗證「被拒絕就寫一列 FAILED、不執行讀寫」。
// 成功路徑與 transaction 內的行為（SUCCEEDED、冪等、rollback）在 integration 測試。

type recorder struct{ got []model.FailureAuditInput }

func (r *recorder) RecordFailure(_ context.Context, in model.FailureAuditInput) error {
	r.got = append(r.got, in)
	return nil
}

type fixedSeller struct {
	store seller.Store
	err   error
}

func (s fixedSeller) Store(context.Context, int64) (seller.Store, error) { return s.store, s.err }

func storeIn(status seller.Status) seller.Store {
	var st seller.Store
	st.StoreID, st.AccountID, st.StoreName, st.Status, st.Sanctions = 5, 12, "梨山盛花園", status, seller.NoSanctions()
	st.Conditions = seller.Conditions{
		Derived: true, Active: status != seller.Closed, NotPendingDeletion: status != seller.PendingDeletion, KYCApproved: true, TransfersActive: true,
		StoreName: true, Logo: true, TownCode: true,
	}
	return st
}

func openStore() seller.Store   { return storeIn(seller.Active) }
func closedStore() seller.Store { return storeIn(seller.Closed) }
func notFound() error           { return apperr.NotFound("店家不存在") }

// bizCodeOf 與 BizCode 是失敗列的業務碼：拒絕對應的碼，與 FAILED 列實際寫的碼（model.BizCodeOf）要一致。
func bizCodeOf(d DelegationDenial) int32 { return DenialBizCodes()[d] }

func BizCode(t *testing.T, cause error) int32 {
	t.Helper()
	c := model.BizCodeOf(cause)
	if c == nil {
		t.Fatalf("%v 沒有業務碼", cause)
	}
	return *c
}

func caller(status DelegationStatus, m DelegatedMethod, caps ...Capability) DelegatedCaller {
	return DelegatedCaller{
		Session: DelegationSession{ID: 7, Status: status, Capabilities: caps, Target: DelegationTarget{StoreID: 5}},
		Method:  m, CorrelationID: "trace-1", UserAgent: "ua",
	}
}

var (
	readMethod  = DelegatedMethod{Capability: CapabilityPostsRead, Action: AuditPostRead, Resource: ResourcePost}
	writeMethod = DelegatedMethod{Capability: CapabilityPostsWrite, Action: AuditPostCreate, Resource: ResourcePost}
)

func denialOf(t *testing.T, err error) DelegationDenial {
	t.Helper()
	d, ok := DelegationDenialOf(err)
	if !ok {
		t.Fatalf("err = %v，預期代行的拒絕", err)
	}
	return d.Denial
}

func newTestModule(rec *recorder, s Seller) *Module { return &Module{seller: s, failures: rec} }

func TestAuthorizeDelegatedRecordsAFailedRowForEveryDenial(t *testing.T) {
	for name, tc := range map[string]struct {
		c      DelegatedCaller
		denial DelegationDenial
	}{
		"缺能力":    {caller(DelegationActive, writeMethod, CapabilityPostsRead), DenialCapabilityDenied},
		"讀取缺能力":  {caller(DelegationActive, readMethod, CapabilityPostsWrite), DenialCapabilityDenied},
		"已撤銷":    {caller(DelegationRevoked, writeMethod, CapabilityPostsWrite), DenialSessionRevoked},
		"已到期或結束": {caller(DelegationExpired, readMethod, CapabilityPostsRead), DenialSessionExpired},
	} {
		rec := &recorder{}
		err := newTestModule(rec, fixedSeller{}).AuthorizeDelegated(t.Context(), tc.c)
		if denialOf(t, err) != tc.denial {
			t.Errorf("%s：%v，預期 %v", name, err, tc.denial)
		}
		if len(rec.got) != 1 {
			t.Fatalf("%s：FAILED 紀錄 %d 筆，預期 1", name, len(rec.got))
		}
		in := rec.got[0]
		if in.Session.ID != 7 || in.Capability != tc.c.Method.Capability || in.Action != tc.c.Method.Action || in.ResourceType != ResourcePost ||
			in.CorrelationID != "trace-1" || in.RequestID != "trace-1" || in.UserAgent != "ua" || BizCode(t, in.Cause) != bizCodeOf(tc.denial) {
			t.Errorf("%s：FAILED 紀錄 = %+v", name, in)
		}
	}
}

func TestAuthorizeDelegatedLeavesAnAllowedRequestUnrecorded(t *testing.T) {
	rec := &recorder{}
	if err := newTestModule(rec, fixedSeller{}).AuthorizeDelegated(t.Context(), caller(DelegationActive, readMethod, CapabilityPostsRead)); err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 0 {
		t.Fatalf("放行的請求不該有 FAILED 紀錄：%+v", rec.got)
	}
}

// 讀取：店家不可代行寫 FAILED（session 與能力由 interceptor 的 AuthorizeDelegated 檢查） 且不執行讀取；成功的讀取不寫。
func TestDelegatedReadRecordsDenialsAndSkipsTheRead(t *testing.T) {
	for name, tc := range map[string]struct {
		c      DelegatedCaller
		seller Seller
		denial DelegationDenial
	}{
		"店家已關閉": {caller(DelegationActive, readMethod, CapabilityPostsRead), fixedSeller{store: closedStore()}, DenialStoreClosed},
		"店家不存在": {caller(DelegationActive, readMethod, CapabilityPostsRead), fixedSeller{err: notFound()}, DenialStoreNotDelegable},
	} {
		rec := &recorder{}
		read := false
		err := newTestModule(rec, tc.seller).DelegatedRead(t.Context(), tc.c, func(context.Context, seller.Store) error { read = true; return nil })
		if denialOf(t, err) != tc.denial || read {
			t.Errorf("%s：%v（讀取執行 %v），預期 %v 且不執行", name, err, read, tc.denial)
		}
		if len(rec.got) != 1 || rec.got[0].Action != AuditPostRead || BizCode(t, rec.got[0].Cause) != bizCodeOf(tc.denial) {
			t.Errorf("%s：FAILED 紀錄 = %+v", name, rec.got)
		}
	}
	rec := &recorder{}
	read := false
	if err := newTestModule(rec, fixedSeller{store: openStore()}).DelegatedRead(t.Context(), caller(DelegationActive, readMethod, CapabilityPostsRead),
		func(_ context.Context, st seller.Store) error { read = st.StoreID == 5; return nil }); err != nil || !read || len(rec.got) != 0 {
		t.Fatalf("成功的讀取：err=%v 讀到店家=%v FAILED 紀錄=%d，預期成功、拿到 session 的店家、不稽核", err, read, len(rec.got))
	}
	// 讀取自己的失敗（找不到等）原樣回傳，不寫稽核。
	rec = &recorder{}
	boom := errors.New("not found")
	if err := newTestModule(rec, fixedSeller{store: openStore()}).DelegatedRead(t.Context(), caller(DelegationActive, readMethod, CapabilityPostsRead),
		func(context.Context, seller.Store) error { return boom }); !errors.Is(err, boom) || len(rec.got) != 0 {
		t.Fatalf("讀取的業務失敗：err=%v FAILED 紀錄=%d", err, len(rec.got))
	}
}

// operations 是單段稽核的替身：記下收到的輸入，像 RunOperation 一樣在「transaction 內」呼叫 do（do 失敗就整段失敗，不留 SUCCEEDED）。
type operations struct {
	got      []model.OperationAuditInput
	replayed bool
	ran      bool
}

func (o *operations) RunOperation(ctx context.Context, in model.OperationAuditInput, do func(context.Context) (model.OperationOutcome, error)) (model.AuditLog, bool, error) {
	o.got = append(o.got, in)
	if o.replayed {
		id := "res-1"
		return model.AuditLog{ResourceID: &id}, true, nil
	}
	o.ran = true
	if _, err := do(ctx); err != nil {
		return model.AuditLog{}, false, err
	}
	return model.AuditLog{}, false, nil
}

// 店家此刻不可代行在寫入的 transaction 內判斷（do 之前、同一個 RunOperation），被拒絕時 do 不執行並另寫一列 FAILED；
// FAILED 列帶著請求已知的資源名稱、ID、request_id 與操作原因。
func TestDelegatedWriteChecksTheStoreInsideTheOperationAndRecordsTheFailure(t *testing.T) {
	op := DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}
	w := DelegatedWrite{Op: op, PayloadHash: "h", ResourceName: "預覽", ResourceID: "r-1"}
	rec, ops := &recorder{}, &operations{}
	m := &Module{seller: fixedSeller{store: closedStore()}, failures: rec, operations: ops}
	did := false
	_, err := m.DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w, func(context.Context, seller.Store) (DelegatedOutcome, error) {
		did = true
		return DelegatedOutcome{}, nil
	})
	if denialOf(t, err) != DenialStoreClosed || did || !ops.ran {
		t.Fatalf("err=%v do=%v 稽核=%v，預期 StoreClosed、do 不執行、店家檢查在 RunOperation 之內", err, did, ops.ran)
	}
	if len(rec.got) != 1 {
		t.Fatalf("FAILED 紀錄 %d 筆", len(rec.got))
	}
	if in := rec.got[0]; in.RequestID != op.RequestID.String() || in.OperationReason != "店家要求" || in.Action != AuditPostCreate || in.ResourceName != "預覽" || in.ResourceID != "r-1" {
		t.Errorf("FAILED 紀錄 = %+v", in)
	}
}

// 重送：do 沒有被呼叫，店家仍要此刻可代行（拒絕就回拒絕）。
func TestDelegatedWriteReplayStillRequiresAnOperableStore(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	c := caller(DelegationActive, writeMethod, CapabilityPostsWrite)
	do := func(context.Context, seller.Store) (DelegatedOutcome, error) { return DelegatedOutcome{}, nil }
	res, err := (&Module{seller: fixedSeller{store: openStore()}, failures: &recorder{}, operations: &operations{replayed: true}}).DelegatedWrite(t.Context(), c, w, do)
	if err != nil || !res.Replayed || res.ResourceID != "res-1" || res.Store.StoreID != 5 {
		t.Fatalf("重送：%+v %v", res, err)
	}
	if _, err := (&Module{seller: fixedSeller{store: closedStore()}, failures: &recorder{}, operations: &operations{replayed: true}}).DelegatedWrite(t.Context(), c, w, do); denialOf(t, err) != DenialStoreClosed {
		t.Fatalf("店家已關閉的重送：%v", err)
	}
}

// 內容雜湊綁定操作原因與 notify_seller：同一把 request_id 帶不同的原因或通知設定不是重送（AIP-155）。
func TestAuditPayloadHashCoversTheReasonAndNotifySeller(t *testing.T) {
	base := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "原因", NotifySeller: true}, PayloadHash: "h"}
	h := auditPayloadHash(base)
	for name, w := range map[string]DelegatedWrite{
		"原因": {Op: DelegatedOperation{RequestID: base.Op.RequestID, Reason: "別的原因", NotifySeller: true}, PayloadHash: "h"},
		"通知": {Op: DelegatedOperation{RequestID: base.Op.RequestID, Reason: "原因", NotifySeller: false}, PayloadHash: "h"},
		"內容": {Op: base.Op, PayloadHash: "h2"},
	} {
		if auditPayloadHash(w) == h {
			t.Errorf("%s不同，雜湊卻相同", name)
		}
	}
	if auditPayloadHash(DelegatedWrite{Op: DelegatedOperation{RequestID: base.Op.RequestID, Reason: "  原因 ", NotifySeller: true}, PayloadHash: "h"}) != h {
		t.Error("首尾空白不算不同的原因")
	}
	ops := &operations{}
	m := &Module{seller: fixedSeller{store: openStore()}, failures: &recorder{}, operations: ops}
	if _, err := m.DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), base, func(context.Context, seller.Store) (DelegatedOutcome, error) { return DelegatedOutcome{}, nil }); err != nil {
		t.Fatal(err)
	}
	if len(ops.got) != 1 || ops.got[0].PayloadHash != h {
		t.Fatalf("交給稽核的雜湊 = %+v，預期 %s", ops.got, h)
	}
}

// 請求的 ctx 已取消仍要留下 FAILED 列：寫入時的 ctx 不帶取消，但有期限。
func TestFailureIsRecordedEvenWhenTheRequestContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var seen error
	var hasDeadline bool
	rec := &ctxRecorder{fn: func(c context.Context) { seen = c.Err(); _, hasDeadline = c.Deadline() }}
	m := &Module{seller: fixedSeller{store: openStore()}, failures: rec}
	m.recordFailure(ctx, caller(DelegationActive, writeMethod, CapabilityPostsWrite), DelegatedWrite{}, context.Canceled)
	if !rec.called || seen != nil || !hasDeadline {
		t.Fatalf("called=%v ctx.Err=%v deadline=%v，預期呼叫、未取消、有期限", rec.called, seen, hasDeadline)
	}
}

type ctxRecorder struct {
	fn     func(context.Context)
	called bool
}

func (r *ctxRecorder) RecordFailure(ctx context.Context, _ model.FailureAuditInput) error {
	r.called = true
	r.fn(ctx)
	return nil
}

// 店家不可代行的三個觸發點（停權、關店、等待刪除）先取消該店的排程，再動 delegation 的列：取消失敗時 session 不被碰（這裡的 session 是 nil，碰了就 panic）。
func TestStoreTriggersCancelSchedulesBeforeTouchingSessions(t *testing.T) {
	boom := errors.New("cancel failed")
	for name, trigger := range map[string]func(*Module) error{
		"StoreSuspended": func(m *Module) error { return m.OnStoreSuspended(t.Context(), seller.StoreSuspended{StoreID: 5}) },
		"StoreClosed":    func(m *Module) error { return m.OnStoreClosed(t.Context(), seller.StoreClosed{StoreID: 5}) },
		"StorePendingDeletion": func(m *Module) error {
			return m.OnStorePendingDeletion(t.Context(), seller.StorePendingDeletion{StoreID: 5})
		},
	} {
		var got int64
		m := &Module{cancelStoreSchedules: func(_ context.Context, id int64) error { got = id; return boom }}
		if err := trigger(m); !errors.Is(err, boom) || got != 5 {
			t.Errorf("%s：err=%v 取消的店家=%d，預期取消先執行並回傳它的錯誤", name, err, got)
		}
	}
}

// 合法的操作原因：去除首尾空白後 1 到 100 字、不含控制字元；不合法的值 FAILED 列不寫。
func TestLegalOperationReason(t *testing.T) {
	for in, ok := range map[string]bool{"店家要求": true, "  店家要求 ": true, "": false, "   ": false, "a\x07b": false, "a\nb": false, strings.Repeat("字", 100): true, strings.Repeat("字", 101): false} {
		if _, got := model.LegalOperationReason(in); got != ok {
			t.Errorf("%q：%v，預期 %v", in, got, ok)
		}
	}
}

func TestDelegatedPayloadHashBindsEveryPart(t *testing.T) {
	a := DelegatedPayloadHash(AuditPostCreate, "x", 1)
	if a != DelegatedPayloadHash(AuditPostCreate, "x", 1) {
		t.Fatal("同樣的內容雜湊要相同")
	}
	for _, other := range [][]any{{AuditPostUpdate, "x", 1}, {AuditPostCreate, "y", 1}, {AuditPostCreate, "x", 2}} {
		if DelegatedPayloadHash(other...) == a {
			t.Errorf("%v 與原內容的雜湊相同", other)
		}
	}
}

func TestEveryDenialHasAnAuditBizCode(t *testing.T) {
	codes := DenialBizCodes()
	for _, d := range []DelegationDenial{DenialCapabilityDenied, DenialSessionRevoked, DenialSessionExpired, DenialStoreClosed, DenialStoreNotDelegable, DenialStoreNotOperable, DenialIdempotencyKeyReused, DenialInvalidCommand} {
		if codes[d] == 0 {
			t.Errorf("%v 沒有業務碼", d)
		}
	}
	if codes[DenialCapabilityDenied] != 9262 {
		t.Errorf("能力不足的業務碼 = %d，預期 9262", codes[DenialCapabilityDenied])
	}
}
