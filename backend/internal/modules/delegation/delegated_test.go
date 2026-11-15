package delegation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// 拒絕路徑的單元測試：session 與能力、店家狀態的拒絕發生在任何資料庫存取之前，所以用替身的 seller 與 FAILED 紀錄器就能驗證「被拒絕就寫一列 FAILED、不執行讀寫」。
// 成功路徑與 transaction 內的行為（SUCCEEDED、冪等、rollback）在 integration 測試。

// recorder 是稽核子領域的替身（Module.audit 的唯一欄位）：嵌入 auditor，只實作 RecordFailure 與 LookupOperation。
type recorder struct {
	auditor
	got []model.FailureAuditInput
	// LookupOperation：記下收到的輸入；replayed 為真時第一次查就回第一次的列。
	opsGot     []model.OperationAuditInput
	replayed   bool
	ran        bool
	noAuditRow bool
	recorded   []model.OperationAuditInput
	outcomes   []model.OperationOutcome
	lookupErr  error
}

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

// failureReasonOf 與 FailureReason 是失敗列的 failure_reason：拒絕對應的代碼，與 FAILED 列實際寫的值（model.FailureReasonOf）要一致。
func failureReasonOf(d DelegationDenial) string { return DenialCodes()[d] }

func FailureReason(t *testing.T, cause error) string {
	t.Helper()
	c := model.FailureReasonOf(cause)
	if c == "" || c == model.FailureReasonInternal {
		t.Fatalf("%v 的失敗原因是 %q，預期拒絕的代碼", cause, c)
	}
	return c
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

func newTestModule(rec *recorder, s Seller) *Module { return &Module{seller: s, audit: rec} }

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
			in.CorrelationID != "trace-1" || in.RequestID != "trace-1" || in.UserAgent != "ua" || FailureReason(t, in.Cause) != failureReasonOf(tc.denial) {
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
		if len(rec.got) != 1 || rec.got[0].Action != AuditPostRead || FailureReason(t, rec.got[0].Cause) != failureReasonOf(tc.denial) {
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

func (r *recorder) RecordOperation(_ context.Context, in model.OperationAuditInput, out model.OperationOutcome) error {
	r.recorded = append(r.recorded, in)
	r.outcomes = append(r.outcomes, out)
	return nil
}

func (r *recorder) LookupOperation(_ context.Context, in model.OperationAuditInput) (model.AuditLog, bool, error) {
	r.opsGot = append(r.opsGot, in)
	if r.lookupErr != nil {
		return model.AuditLog{}, false, r.lookupErr
	}
	// 重送：第一次查就有列；一般寫入：do 跑過之後（訂閱者寫了 SUCCEEDED）才有列，除非 noAuditRow。
	if r.replayed || r.ran && !r.noAuditRow {
		id := "res-1"
		return model.AuditLog{ResourceID: &id}, true, nil
	}
	return model.AuditLog{}, false, nil
}

// 店家此刻不可代行在呼叫資源模組之前判斷，被拒絕時 do 不執行並另寫一列 FAILED（訂閱者在 social 的 transaction 內還會再檢查一次）；
// FAILED 列帶著請求已知的資源名稱、ID、request_id 與操作原因。
func TestDelegatedWriteChecksTheStoreBeforeTheWriteAndRecordsTheFailure(t *testing.T) {
	op := DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}
	w := DelegatedWrite{Op: op, PayloadHash: "h", ResourceName: "預覽", ResourceID: "r-1"}
	rec := &recorder{}
	m := &Module{seller: fixedSeller{store: closedStore()}, audit: rec}
	did := false
	_, err := m.DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w, func(context.Context, seller.Store, *delegatedactor.Actor) error {
		did = true
		return nil
	})
	if denialOf(t, err) != DenialStoreClosed || did {
		t.Fatalf("err=%v do=%v，預期 StoreClosed、do 不執行", err, did)
	}
	if len(rec.got) != 1 {
		t.Fatalf("FAILED 紀錄 %d 筆", len(rec.got))
	}
	if in := rec.got[0]; in.RequestID != op.RequestID.String() || in.OperationReason != "店家要求" || in.Action != AuditPostCreate || in.ResourceName != "預覽" || in.ResourceID != "r-1" {
		t.Errorf("FAILED 紀錄 = %+v", in)
	}
}

// 沒有能力的 session 在呼叫資源模組之前就被拒絕：do 不執行（資源模組的業務驗證不得先於授權），並寫一列 CAPABILITY_DENIED 的 FAILED。
// 拿掉呼叫 do 之前的 CheckSession，do 會被執行，這個測試失敗。
func TestDelegatedWriteChecksSessionAndCapabilityBeforeTheWrite(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	rec := &recorder{}
	m := &Module{seller: fixedSeller{store: openStore()}, audit: rec}
	did := false
	_, err := m.DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsRead), w, func(context.Context, seller.Store, *delegatedactor.Actor) error {
		did = true
		return nil
	})
	if denialOf(t, err) != DenialCapabilityDenied || did {
		t.Fatalf("err=%v do=%v，預期 CapabilityDenied、do 不執行", err, did)
	}
	if len(rec.got) != 1 || model.FailureReasonOf(rec.got[0].Cause) != failureReasonOf(DenialCapabilityDenied) {
		t.Fatalf("FAILED 紀錄 = %+v", rec.got)
	}
}

// 重送回第一次的結果，與店家此刻的狀態無關（AIP-155）：店家已關閉也成功，不呼叫 do、不寫 FAILED。
// 拿掉呼叫 do 之前的 LookupOperation，重送會呼叫 do（或在已關閉的店家被拒絕），這個測試失敗。
func TestDelegatedWriteReplayReturnsTheFirstResultWhateverTheStoreState(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	c := caller(DelegationActive, writeMethod, CapabilityPostsWrite)
	for name, st := range map[string]seller.Store{"可代行": openStore(), "已關閉": closedStore(), "等待刪除": storeIn(seller.PendingDeletion)} {
		rec := &recorder{replayed: true}
		did := false
		res, err := (&Module{seller: fixedSeller{store: st}, audit: rec}).DelegatedWrite(t.Context(), c, w, func(context.Context, seller.Store, *delegatedactor.Actor) error {
			did = true
			return nil
		})
		if err != nil || !res.Replayed || res.ResourceID != "res-1" || res.Store.StoreID != 5 || did {
			t.Fatalf("%s 的重送：%+v %v do=%v", name, res, err, did)
		}
		if len(rec.got) != 0 {
			t.Fatalf("%s：重送不該寫 FAILED：%+v", name, rec.got)
		}
	}
	// 讀不到店家的重送失敗也不為這把 request_id 寫 FAILED（它已經有 SUCCEEDED 的列）。
	rec := &recorder{replayed: true}
	if _, err := (&Module{seller: fixedSeller{err: notFound()}, audit: rec}).DelegatedWrite(t.Context(), c, w, nil); err == nil || len(rec.got) != 0 {
		t.Fatalf("店家不存在的重送：err=%v FAILED=%d", err, len(rec.got))
	}
}

// delegation 不開 transaction 包住資源模組的寫入（A7-D26，單體規則 3、5）：do 拿到的 ctx 沒有 transaction，代行者是明確的參數（A7-D27，不經 ctx）；
// Module 沒有資料庫欄位，重新加上外層 InTx 會在這裡 panic 或讓 do 看到 transaction。
func TestDelegatedWriteOpensNoTransactionAroundTheResourceWrite(t *testing.T) {
	op := DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求", NotifySeller: true}
	v := int64(3)
	w := DelegatedWrite{Op: op, PayloadHash: "h", ResourceID: "r-1", Version: &v}
	rec := &recorder{}
	called := false
	res, err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w,
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
			called = true
			rec.ran = true
			if _, ok := db.TxFrom(ctx); ok {
				t.Error("do 的 ctx 帶著 transaction：delegation 不得開外層 transaction")
			}
			a := actor
			if a == nil || a.SessionID != 7 || a.StoreID != 5 || a.RequestID != op.RequestID.String() || a.Reason != "店家要求" || !a.NotifySeller ||
				a.PayloadHash != auditPayloadHash(w) || a.ResourceVersionBefore == nil || *a.ResourceVersionBefore != 3 {
				t.Errorf("明確傳給 do 的代行者 = %+v", a)
			}
			if st.StoreID != 5 {
				t.Errorf("交給 do 的店家 = %d", st.StoreID)
			}
			return nil
		})
	if err != nil || !called || res.Replayed || res.ResourceID != "res-1" {
		t.Fatalf("res=%+v err=%v called=%v", res, err, called)
	}
}

// 並行的重送：訂閱者撞唯一鍵、回 ErrOperationKeyTaken 讓 social 的 transaction rollback，元件重讀稽核列並收斂成重送；不寫 FAILED。
func TestDelegatedWriteConvergesOnAConcurrentReplay(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	rec := &recorder{}
	res, err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w,
		func(context.Context, seller.Store, *delegatedactor.Actor) error {
			rec.ran = true // 對方的 transaction commit 了
			return fmt.Errorf("events：處理：%w", model.ErrOperationKeyTaken)
		})
	if err != nil || !res.Replayed || res.ResourceID != "res-1" || len(rec.got) != 0 {
		t.Fatalf("res=%+v err=%v FAILED=%d", res, err, len(rec.got))
	}
}

// 資源模組的寫入成功卻沒有稽核列（沒有訂閱者或事件沒發）是內部錯誤，並寫 FAILED。
func TestDelegatedWriteWithoutAnAuditRowIsAnInternalError(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	rec := &recorder{noAuditRow: true}
	_, err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w,
		func(context.Context, seller.Store, *delegatedactor.Actor) error { rec.ran = true; return nil })
	if err == nil || len(rec.got) != 1 {
		t.Fatalf("err=%v FAILED=%d", err, len(rec.got))
	}
}

// do 的失敗（業務失敗）原樣回傳並另寫一列 FAILED。
func TestDelegatedWriteBusinessFailureRecordsAFailedRow(t *testing.T) {
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}, PayloadHash: "h"}
	rec := &recorder{}
	boom := apperr.VersionConflict("版本衝突")
	_, err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w,
		func(context.Context, seller.Store, *delegatedactor.Actor) error { return boom })
	if !errors.Is(err, boom) || len(rec.got) != 1 {
		t.Fatalf("err=%v FAILED=%d", err, len(rec.got))
	}
}

// 請求內容雜湊空白：INVALID_COMMAND，不進稽核的寫入（也不被 reason／notify_seller 的包裝蓋掉）。
func TestDelegatedWriteRejectsAnEmptyPayloadHash(t *testing.T) {
	rec := &recorder{}
	w := DelegatedWrite{Op: DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求", NotifySeller: true}}
	_, err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), w, nil)
	if denialOf(t, err) != DenialInvalidCommand || len(rec.opsGot) != 0 {
		t.Fatalf("err=%v 稽核寫入=%d，預期 INVALID_COMMAND 且沒有寫入", err, len(rec.opsGot))
	}
}

// DelegatedRead 自己檢查 session 與能力（公開 API 不信任呼叫方已授權）：被拒絕不執行讀取並寫 FAILED。
func TestDelegatedReadChecksTheSessionAndCapabilityItself(t *testing.T) {
	for name, tc := range map[string]struct {
		c      DelegatedCaller
		denial DelegationDenial
	}{
		"已撤銷": {caller(DelegationRevoked, readMethod, CapabilityPostsRead), DenialSessionRevoked},
		"缺能力": {caller(DelegationActive, readMethod, CapabilityPostsWrite), DenialCapabilityDenied},
	} {
		rec := &recorder{}
		read := false
		err := (&Module{seller: fixedSeller{store: openStore()}, audit: rec}).DelegatedRead(t.Context(), tc.c, func(context.Context, seller.Store) error { read = true; return nil })
		if denialOf(t, err) != tc.denial || read || len(rec.got) != 1 {
			t.Errorf("%s：err=%v 讀取=%v FAILED=%d", name, err, read, len(rec.got))
		}
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
	ops := &recorder{}
	m := &Module{seller: fixedSeller{store: openStore()}, audit: ops}
	if _, err := m.DelegatedWrite(t.Context(), caller(DelegationActive, writeMethod, CapabilityPostsWrite), base, func(context.Context, seller.Store, *delegatedactor.Actor) error { ops.ran = true; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, in := range ops.opsGot {
		if in.PayloadHash != h {
			t.Fatalf("交給稽核的雜湊 = %+v，預期 %s", in, h)
		}
	}
	if len(ops.opsGot) == 0 {
		t.Fatal("沒有交給稽核")
	}
}

// 請求的 ctx 已取消仍要留下 FAILED 列：寫入時的 ctx 不帶取消，但有期限。
func TestFailureIsRecordedEvenWhenTheRequestContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var seen error
	var hasDeadline bool
	rec := &ctxRecorder{fn: func(c context.Context) { seen = c.Err(); _, hasDeadline = c.Deadline() }}
	m := &Module{seller: fixedSeller{store: openStore()}, audit: rec}
	m.recordFailure(ctx, caller(DelegationActive, writeMethod, CapabilityPostsWrite), DelegatedWrite{}, context.Canceled)
	if !rec.called || seen != nil || !hasDeadline {
		t.Fatalf("called=%v ctx.Err=%v deadline=%v，預期呼叫、未取消、有期限", rec.called, seen, hasDeadline)
	}
}

type ctxRecorder struct {
	auditor
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

func TestEveryDenialHasAFailureCode(t *testing.T) {
	codes := DenialCodes()
	for d := range DenialBizCodes() {
		c := codes[d]
		if c == "" {
			t.Errorf("%v 沒有 failure_reason 的代碼", d)
		}
		if back, ok := DenialOfCode(c); !ok || back != d {
			t.Errorf("代碼 %q 對不回 %v", c, d)
		}
	}
	if codes[DenialCapabilityDenied] != "CAPABILITY_DENIED" {
		t.Errorf("能力不足的代碼 = %q", codes[DenialCapabilityDenied])
	}
}

func TestFailureReasonOfClassifiesTheCause(t *testing.T) {
	wrapped := fmt.Errorf("wrap: %w", apperr.VersionConflict("版本過時"))
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"denial":              {model.Deny(model.DenialSessionRevoked, "x"), "SESSION_REVOKED"},
		"business reason":     {wrapped, "VERSION_CONFLICT"},
		"not found":           {apperr.NotFound("x"), "NOT_FOUND"},
		"invalid input":       {apperr.InvalidInput("x"), "INVALID_INPUT"},
		"failed precondition": {apperr.FailedPrecondition("x"), "FAILED_PRECONDITION"},
		"unavailable":         {apperr.Unavailable("x"), "UNAVAILABLE"},
		"plain error":         {errors.New("boom"), FailureReasonInternal},
		"nil":                 {nil, FailureReasonInternal},
	} {
		if got := model.FailureReasonOf(tc.err); got != tc.want || !model.LegalFailureReason(got) {
			t.Errorf("%s：%q，預期 %q（且符合 CHECK 格式）", name, got, tc.want)
		}
	}
}

// 訂閱者依事件種類決定稽核的動作與能力（事件不帶 delegation 的宣告字串，A7-D27）；店家一致、可代行才寫 SUCCEEDED；
// 不一致或店家不可代行回錯誤（social 的 transaction rollback），不寫稽核；沒有代行者（店主自己的寫入）略過、不寫稽核。
func TestPostWriteSubscribersMapEachEventKindToItsAuditActionAndCapability(t *testing.T) {
	post := uuid.New()
	actor := &delegatedactor.Actor{
		SessionID: 7, StoreID: 5, RequestID: uuid.New().String(), Reason: "店家要求", NotifySeller: true, PayloadHash: "h",
	}
	w := social.PostWritten{PostID: post, StoreID: 5, Preview: "  預覽  ", Version: 4, Actor: actor}
	m := func(rec *recorder, st seller.Store) *Module {
		return &Module{seller: fixedSeller{store: st}, audit: rec}
	}
	type sub func(*Module, context.Context, social.PostWritten) error
	cases := []struct {
		name   string
		call   sub
		action AuditAction
		cap    Capability
	}{
		{"PostCreated", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostCreated(c, social.PostCreated{PostWritten: w})
		}, AuditPostCreate, CapabilityPostsWrite},
		{"PostUpdated", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostUpdated(c, social.PostUpdated{PostWritten: w})
		}, AuditPostUpdate, CapabilityPostsWrite},
		{"PostUnpublished", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostUnpublished(c, social.PostUnpublished{PostWritten: w})
		}, AuditPostUnpublish, CapabilityPostsPublish},
		{"PostScheduled", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostScheduled(c, social.PostScheduled{PostWritten: w})
		}, AuditPostSchedule, CapabilityPostsPublish},
		{"PostPublishRepeated", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostPublishRepeated(c, social.PostPublishRepeated{PostWritten: w})
		}, AuditPostPublish, CapabilityPostsPublish},
		{"PostUnpublishRepeated", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostUnpublishRepeated(c, social.PostUnpublishRepeated{PostWritten: w})
		}, AuditPostUnpublish, CapabilityPostsPublish},
		{"PostPublished", func(m *Module, c context.Context, w social.PostWritten) error {
			return m.OnPostPublished(c, social.PostPublished{PostID: w.PostID, StoreID: w.StoreID, Preview: w.Preview, Version: w.Version, Origin: social.OriginDirect, Actor: w.Actor})
		}, AuditPostPublish, CapabilityPostsPublish},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			if err := tc.call(m(rec, openStore()), t.Context(), w); err != nil {
				t.Fatal(err)
			}
			if len(rec.recorded) != 1 || rec.recorded[0].ResourceID != post.String() || rec.recorded[0].ResourceName != "預覽" || rec.recorded[0].RequestID != actor.RequestID ||
				rec.recorded[0].Action != tc.action || rec.recorded[0].Capability != tc.cap || rec.recorded[0].ResourceType != ResourcePost || rec.recorded[0].SessionID != 7 ||
				rec.outcomes[0].VersionAfter != 4 {
				t.Fatalf("寫入的稽核 = %+v %+v，預期動作 %s 能力 %s", rec.recorded, rec.outcomes, tc.action, tc.cap)
			}

			owner := w
			owner.Actor = nil
			rec = &recorder{}
			if err := tc.call(m(rec, openStore()), t.Context(), owner); err != nil || len(rec.recorded) != 0 {
				t.Errorf("店主的寫入（沒有代行者）：err=%v 稽核=%d，預期略過", err, len(rec.recorded))
			}

			wrongStore := w
			wrongStore.StoreID = 6
			rec = &recorder{}
			if err := tc.call(m(rec, openStore()), t.Context(), wrongStore); err == nil || len(rec.recorded) != 0 {
				t.Errorf("店家不一致：err=%v 稽核=%d", err, len(rec.recorded))
			}
			rec = &recorder{}
			err := tc.call(m(rec, closedStore()), t.Context(), w)
			if denialOf(t, err) != DenialStoreClosed || len(rec.recorded) != 0 {
				t.Errorf("店家已關閉：err=%v 稽核=%d", err, len(rec.recorded))
			}
		})
	}
}
