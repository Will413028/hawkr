//go:build integration

package seller_test

import (
	"context"
	"slices"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 只為 legacy SellerService 存在的公開讀取（遷移計畫 A3b 步驟 9a）。

// legacyWorld 是各種狀態的店家：可曝光、剛開店、停權、等待刪除、已關閉（帳號之後又開了新的 stint）。
type legacyWorld struct {
	exposed, pending, suspended, deleting, closed, reopened Store
	closedAccount                                           int64
	suspension                                              Measure
}

func (f *fixture) legacyWorld(t *testing.T) legacyWorld {
	t.Helper()
	ctx := context.Background()
	var w legacyWorld
	_, w.exposed = f.exposable(t)
	_, w.pending = f.open(t)
	_, w.suspended = f.exposable(t)
	_, w.deleting = f.exposable(t)
	w.closedAccount, w.closed = f.exposable(t)
	w.suspension = f.suspend(t, w.suspended.StoreID)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, w.deleting.StoreID) }))
	_, err := f.module.CloseCurrent(ctx, w.closedAccount)
	must(t, err)
	w.reopened, err = f.module.Apply(ctx, validApply(w.closedAccount))
	must(t, err)
	return w
}

func TestBatchStoresIncludesClosedStintsInInputOrderWithTheDerivedSuspension(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.legacyWorld(t)
	got, err := f.module.BatchStores(context.Background(), []int64{w.closed.StoreID, w.suspended.StoreID, w.closed.StoreID, 999999, w.exposed.StoreID})
	must(t, err)
	if len(got) != 3 || got[0].StoreID != w.closed.StoreID || got[1].StoreID != w.suspended.StoreID || got[2].StoreID != w.exposed.StoreID {
		t.Fatalf("got=%+v", got)
	}
	if got[0].Status != Closed || !got[1].Suspended() || got[2].Suspended() || got[2].Status != Active {
		t.Fatalf("closed=%v suspended=%v exposed=%v/%v", got[0].Status, got[1].Suspended(), got[2].Status, got[2].Suspended())
	}
	if empty, err := f.module.BatchStores(context.Background(), nil); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%v %v", empty, err)
	}
}

func TestLegacyBatchesRejectOversizedAndNonPositiveIDs(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	tooMany := make([]int64, 101)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	for name, call := range map[string]func(ids []int64) error{
		"BatchStores":        func(ids []int64) error { _, err := f.module.BatchStores(ctx, ids); return err },
		"StoresWithExposure": func(ids []int64) error { _, err := f.module.StoresWithExposure(ctx, ids); return err },
		"ResolvePrincipals":  func(ids []int64) error { _, err := f.module.ResolvePrincipals(ctx, ids); return err },
	} {
		for label, ids := range map[string][]int64{"too many": tooMany, "zero": {0}, "negative": {-1}} {
			if err := call(ids); apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Errorf("%s %s: %v", name, label, err)
			}
		}
	}
}

// 批次的可曝光與 ExposableStores 是同一個規則：兩邊對每種狀態的店家答案必須一致。
func TestStoresWithExposureAgreesWithExposableStores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.legacyWorld(t)
	ids := []int64{w.exposed.StoreID, w.pending.StoreID, w.suspended.StoreID, w.deleting.StoreID, w.closed.StoreID, w.reopened.StoreID}
	got, err := f.module.StoresWithExposure(context.Background(), ids)
	must(t, err)
	exposed := f.exposed(t, ids...)
	if !slices.Equal(exposed, []int64{w.exposed.StoreID}) || len(got) != len(ids) {
		t.Fatalf("exposed=%v got=%d", exposed, len(got))
	}
	for i, e := range got {
		if e.Store.StoreID != ids[i] || e.Exposable != slices.Contains(exposed, ids[i]) {
			t.Errorf("%d: %+v", ids[i], e)
		}
	}
	// 能力：停權、等待刪除、已關閉都不能管理商品；沒有核准的新店可以。
	for id, want := range map[int64]bool{w.exposed.StoreID: true, w.pending.StoreID: true, w.suspended.StoreID: false, w.deleting.StoreID: false, w.closed.StoreID: false} {
		for _, e := range got {
			if e.Store.StoreID == id && e.Store.Can(CapProductManage) != want {
				t.Errorf("store %d Can(CapProductManage)=%v, want %v", id, !want, want)
			}
		}
	}
}

// 已關閉的 stint 不解析（舊契約的 unresolved：沒有存活的店家列）；存活的照解析，不存在的不在結果裡。
func TestResolvePrincipalsExcludesClosedStints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.legacyWorld(t)
	got, err := f.module.ResolvePrincipals(context.Background(), []int64{w.closed.StoreID, w.reopened.StoreID, w.exposed.StoreID, 999999})
	must(t, err)
	if len(got) != 2 || got[w.reopened.StoreID] != w.closedAccount || got[w.exposed.StoreID] != w.exposed.AccountID {
		t.Fatalf("got=%v", got)
	}
	if _, ok := got[w.closed.StoreID]; ok {
		t.Fatalf("已關閉的 stint 被解析：%v", got)
	}
}

func TestStoreIDsByAccountListsEveryStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.legacyWorld(t)
	got, err := f.module.StoreIDsByAccount(context.Background(), w.closedAccount)
	must(t, err)
	if !slices.Equal(got, []int64{w.closed.StoreID, w.reopened.StoreID}) {
		t.Fatalf("got=%v", got)
	}
	none, err := f.module.StoreIDsByAccount(context.Background(), 999999)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("none=%v %v", none, err)
	}
	if _, err := f.module.StoreIDsByAccount(context.Background(), 0); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
}

func TestExposableStoreIDsByCountyAppliesTheExposureRule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	w := f.legacyWorld(t)
	got, err := f.module.ExposableStoreIDsByCounty(ctx, []string{"63000", "64000"})
	must(t, err)
	// 63000（臺北市）只有可曝光的那一家：停權、等待刪除、已關閉、沒有核准的都不在；64000 沒有店家但有鍵。
	if len(got) != 2 || !slices.Equal(got["63000"], []int64{w.exposed.StoreID}) || got["64000"] == nil || len(got["64000"]) != 0 {
		t.Fatalf("got=%v", got)
	}
	// 與 ExposableStores 同一個規則：停權解除之後回來，依 store_id 排序。
	_, err = f.lift(ctx, w.suspension)
	must(t, err)
	got, err = f.module.ExposableStoreIDsByCounty(ctx, []string{"63000"})
	must(t, err)
	if !slices.Equal(got["63000"], []int64{w.exposed.StoreID, w.suspended.StoreID}) {
		t.Fatalf("got=%v", got)
	}
	for _, bad := range [][]string{{"63000050"}, {""}, {"99999"}} {
		if _, err := f.module.ExposableStoreIDsByCounty(ctx, bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%v: %v", bad, err)
		}
	}
	if none, err := f.module.ExposableStoreIDsByCounty(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("none=%v %v", none, err)
	}
}

func TestMatchCertificateHolder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	w := f.legacyWorld(t)
	match := func(storeID int64, holder string) CertHolderMatch {
		t.Helper()
		got, err := f.module.MatchCertificateHolder(ctx, storeID, holder)
		must(t, err)
		return got
	}
	if got := match(w.exposed.StoreID, "王小明"); !got.Matched || got.Field != "REAL_NAME" || !got.HasReference {
		t.Fatalf("approved holder: %+v", got)
	}
	if got := match(w.exposed.StoreID, "陳大明"); got.Matched || got.Field != "NONE" || !got.HasReference {
		t.Fatalf("other holder: %+v", got)
	}
	// 沒有基準：沒有申請、已關閉的 stint、不存在的店。（停權的店仍有基準：核准的身分沒有變。）
	for name, id := range map[string]int64{"no application": w.pending.StoreID, "closed": w.closed.StoreID, "reopened": w.reopened.StoreID, "unknown": 999999} {
		if got := match(id, "王小明"); got.Matched || got.HasReference || got.Field != "NONE" {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := match(w.suspended.StoreID, "王小明"); !got.Matched {
		t.Errorf("suspended: %+v", got)
	}
	if _, err := f.module.MatchCertificateHolder(ctx, 0, "王小明"); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
}

// 比對基準是人工核准的 KYC（設計書 19 §8.5.1「已通過實名認證」）：PayUni 還沒核准、甚至被拒絕，基準都還在；申請還沒核准則沒有基準。
func TestMatchCertificateHolderDoesNotNeedThePayUniApproval(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.boundStore(t) // KYC 核准、已綁定，PayUni 還沒有審核結果。
	got, err := f.module.MatchCertificateHolder(ctx, s.StoreID, "王小明")
	must(t, err)
	if !got.Matched || got.Field != "REAL_NAME" || !got.HasReference {
		t.Fatalf("PayUni 未核准：%+v", got)
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	f.now = f.now.Add(24 * time.Hour)
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	if got, err = f.module.MatchCertificateHolder(ctx, s.StoreID, "王小明"); err != nil || !got.Matched || !got.HasReference {
		t.Fatalf("PayUni 拒絕後：%+v %v", got, err)
	}
	// 申請送出但還沒人工核准：賣家自己填的名稱不能當基準。
	_, submittedStore, _ := f.submitted(t, idB, bankAcct)
	if got, err = f.module.MatchCertificateHolder(ctx, submittedStore.StoreID, "王小明"); err != nil || got.Matched || got.HasReference {
		t.Fatalf("未核准的申請：%+v %v", got, err)
	}
}

// 店家與停權狀態同一次讀取：停權帶生效中的措施（依據與風險類型），沒有停權什麼都沒有；Current 與 by-id 兩種讀法一致；
// 找不到店家或帳號回 NotFound。
func TestStoreCarriesTheEffectiveMeasuresOfTheSameRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, gov := f.exposable(t) // 措施建立之後同一個身分不能再通過認證，所以先建好
	w := f.legacyWorld(t)
	got, err := f.module.Store(ctx, w.suspended.StoreID)
	must(t, err)
	if !got.Suspended() || len(got.EffectiveMeasures()) != 1 || got.EffectiveMeasures()[0].MeasureID != w.suspension.ID || got.EffectiveMeasures()[0].Basis != BasisRisk ||
		got.EffectiveMeasures()[0].RiskGround == nil || *got.EffectiveMeasures()[0].RiskGround != RiskFraud {
		t.Fatalf("got=%+v", got)
	}
	got, err = f.module.Store(ctx, w.exposed.StoreID)
	must(t, err)
	if got.Suspended() || len(got.EffectiveMeasures()) != 0 {
		t.Fatalf("got=%+v", got)
	}
	// 依據認定的停權：帶主要依據的類別，沒有風險類型。
	ends := f.now.Add(30 * 24 * time.Hour)
	f.violate(t, gov.StoreID, SeverityHigh, MeasureSuspension, &ends)
	got, err = f.module.Store(ctx, gov.StoreID)
	must(t, err)
	if !got.Suspended() || len(got.EffectiveMeasures()) != 1 || got.EffectiveMeasures()[0].Basis != BasisViolation || got.EffectiveMeasures()[0].RiskGround != nil ||
		got.EffectiveMeasures()[0].LeadCategory == nil || *got.EffectiveMeasures()[0].LeadCategory != policy.MisleadingInfo {
		t.Fatalf("got=%+v", got)
	}
	// Current 與 by-id 一致。
	cur, err := f.module.CurrentStore(ctx, w.suspended.AccountID)
	must(t, err)
	if cur.StoreID != w.suspended.StoreID || !cur.Suspended() || len(cur.EffectiveMeasures()) != 1 || cur.EffectiveMeasures()[0].MeasureID != w.suspension.ID {
		t.Fatalf("cur=%+v", cur)
	}
	if _, err := f.module.CurrentStore(ctx, 999999); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
	if _, err := f.module.Store(ctx, 999999); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
	// 批次讀取帶同一份推導。
	batch, err := f.module.BatchStores(ctx, []int64{w.suspended.StoreID, gov.StoreID})
	must(t, err)
	if len(batch) != 2 || len(batch[0].EffectiveMeasures()) != 1 || batch[0].EffectiveMeasures()[0].Basis != BasisRisk || batch[1].EffectiveMeasures()[0].Basis != BasisViolation {
		t.Fatalf("batch=%+v", batch)
	}
}

// 措施記在舊 stint、帳號目前的 stint 繼承停權時，帶出的措施是帳號範圍的（本 stint 沒有措施紀錄）。
func TestStoreFindsAMeasureRecordedOnAnEarlierStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, old := f.exposable(t)
	record := f.suspend(t, old.StoreID)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	// Apply 在停權中會拒絕，所以直接寫一個新的 stint 重現「新 stint 繼承停權」。
	var newID int64
	must(t, f.db.Conn(ctx).QueryRow(ctx,
		`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, '新店', $3, $3) RETURNING store_id`,
		old.StoreCode+1, accountID, f.now).Scan(&newID))
	for name, read := range map[string]func() (Store, error){
		"current": func() (Store, error) { return f.module.CurrentStore(ctx, accountID) },
		"by id":   func() (Store, error) { return f.module.Store(ctx, newID) },
	} {
		got, err := read()
		must(t, err)
		if got.StoreID != newID || !got.Suspended() || len(got.EffectiveMeasures()) != 1 || got.EffectiveMeasures()[0].MeasureID != record.ID {
			t.Fatalf("%s: 繼承的停權要帶舊 stint 的措施：%+v", name, got)
		}
	}
	state, err := f.module.StoreWithActiveMeasures(ctx, newID)
	if err != nil || !state.Store.Suspended() || len(state.Active) != 1 || state.Active[0].ID != record.ID || state.Active[0].StoreID != old.StoreID || state.Active[0].State != StateActive {
		t.Fatalf("繼承的停權要在 StoreWithActiveMeasures 裡：err=%v %+v", err, state)
	}
}
