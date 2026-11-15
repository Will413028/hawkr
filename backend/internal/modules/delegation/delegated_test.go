package delegation

import (
	"context"
	"errors"
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

// 讀取：被拒絕（能力、店家）都寫 FAILED 且不執行讀取；成功的讀取不寫。
func TestDelegatedReadRecordsDenialsAndSkipsTheRead(t *testing.T) {
	for name, tc := range map[string]struct {
		c      DelegatedCaller
		seller Seller
		denial DelegationDenial
	}{
		"缺能力":   {caller(DelegationActive, readMethod, CapabilityPostsWrite), fixedSeller{store: openStore()}, DenialCapabilityDenied},
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

// 寫入：進 transaction 之前的拒絕（session、能力、店家）寫 FAILED，不執行寫入；請求的 request_id 與操作原因記在 FAILED 列上。
func TestDelegatedWriteRecordsPreTransactionDenialsAndSkipsTheWrite(t *testing.T) {
	op := DelegatedOperation{RequestID: uuid.New(), Reason: "店家要求"}
	w := DelegatedWrite{Op: op, PayloadHash: "h"}
	for name, tc := range map[string]struct {
		c      DelegatedCaller
		seller Seller
		denial DelegationDenial
	}{
		"缺能力":   {caller(DelegationActive, writeMethod, CapabilityPostsRead), fixedSeller{store: openStore()}, DenialCapabilityDenied},
		"已撤銷":   {caller(DelegationRevoked, writeMethod, CapabilityPostsWrite), fixedSeller{store: openStore()}, DenialSessionRevoked},
		"店家已關閉": {caller(DelegationActive, writeMethod, CapabilityPostsWrite), fixedSeller{store: closedStore()}, DenialStoreClosed},
	} {
		rec := &recorder{}
		did := false
		_, err := newTestModule(rec, tc.seller).DelegatedWrite(t.Context(), tc.c, w, func(context.Context, seller.Store) (DelegatedOutcome, error) {
			did = true
			return DelegatedOutcome{}, nil
		})
		if denialOf(t, err) != tc.denial || did {
			t.Errorf("%s：%v（寫入執行 %v），預期 %v 且不執行", name, err, did, tc.denial)
		}
		if len(rec.got) != 1 {
			t.Fatalf("%s：FAILED 紀錄 %d 筆", name, len(rec.got))
		}
		if in := rec.got[0]; in.RequestID != op.RequestID.String() || in.OperationReason != "店家要求" || in.Action != AuditPostCreate {
			t.Errorf("%s：FAILED 紀錄 = %+v", name, in)
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
