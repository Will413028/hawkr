//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/jackc/pgx/v5"
)

// 主管機關來函（step10e-3cb-design §7.3、§7.4）。管理員 31 登記、32 完成。

const (
	authorityRegistrar = 31
	authorityCompleter = 32
)

// orderIn 是以 f.now 收文的來函輸入，對象是 targets。
func (f *fixture) orderIn(kind AuthorityOrderKind, targets ...AuthorityTargetInput) CreateAuthorityOrderInput {
	return CreateAuthorityOrderInput{
		AdminID: authorityRegistrar, Agency: "臺北市政府衛生局", DocumentNo: "北市衛食字第1150000001號", ReceivedAt: f.now, Kind: kind,
		Summary: "請提供該店家的登記資料", Targets: targets,
	}
}

func (f *fixture) createOrder(t *testing.T, in CreateAuthorityOrderInput) AdminAuthorityOrder {
	t.Helper()
	r, err := f.module.CreateAuthorityOrder(context.Background(), in)
	must(t, err)
	return r.Order
}

// holdOrder 登記一份調取資料的來函，指名帳號全部的店家。
func (f *fixture) holdOrder(t *testing.T, accountID int64) AdminAuthorityOrder {
	t.Helper()
	return f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID}))
}

func (f *fixture) orderOf(t *testing.T, id uuid.UUID) AdminAuthorityOrder {
	t.Helper()
	o, err := f.module.AuthorityOrder(context.Background(), id)
	must(t, err)
	return o
}

// notifyAll 是完成來函時每個對象都已告知的輸入。
func (f *fixture) notifyAll(o AdminAuthorityOrder) []TargetNoticeInput {
	at := f.now
	out := make([]TargetNoticeInput, len(o.Targets))
	for i := range o.Targets {
		out[i] = TargetNoticeInput{Position: i, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at, Note: "電話告知"}}
	}
	return out
}

func (f *fixture) completeOrder(o AdminAuthorityOrder, provide *time.Time, notices []TargetNoticeInput) (AdminAuthorityOrder, error) {
	return f.module.CompleteAuthorityOrder(context.Background(), CompleteAuthorityOrderInput{
		ID: o.ID, AdminID: authorityCompleter, ExpectedVersion: o.Version, ResolutionSummary: "已於期限內回覆機關", ProvideTime: provide, TargetNotices: notices,
	})
}

// closedStore 開店並關店，回傳帳號與店家；關店之後 f.now 不動。
func (f *fixture) closedStore(t *testing.T) (int64, Store) {
	t.Helper()
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(context.Background(), accountID)
	must(t, err)
	return accountID, s
}

func wantInvalid(t *testing.T, err error, field, sentence string) {
	t.Helper()
	wantKind(t, err, apperr.KindInvalidInput)
	if got := invalidFields(t, err); !slices.Contains(got, field) {
		t.Fatalf("要指到欄位 %s：%v", field, got)
	}
	if !strings.Contains(err.Error(), sentence) {
		t.Fatalf("訊息要含 %q：%v", sentence, err)
	}
}

// 登記：預設期限依種類、保存期限是收文後 184 天、對象逐家列出涵蓋狀態；沒有店家的帳號照樣登記（no_seller_data）。
func TestCreateAuthorityOrderDefaultsAndCoverage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	buyer := f.account(t)
	for kind, want := range map[AuthorityOrderKind]time.Duration{
		AuthorityRestrictProduct: 48 * time.Hour, AuthoritySuspendAccount: 48 * time.Hour, AuthorityDataRequest: 72 * time.Hour, AuthorityOther: 72 * time.Hour,
	} {
		o := f.createOrder(t, f.orderIn(kind, AuthorityTargetInput{AccountID: accountID}))
		if !o.DueAt.Equal(f.now.Add(want)) || !o.HoldUntil.Equal(f.now.Add(4416*time.Hour)) || o.Version != 1 || !o.Open() || !o.HoldActive {
			t.Errorf("%s：due=%v hold=%v version=%d", kind, o.DueAt, o.HoldUntil, o.Version)
		}
		if len(o.Availability.Complete) != 0 {
			t.Errorf("%s：新來函要能完成", kind)
		}
	}
	o := f.createOrder(t, f.orderIn(AuthorityRestrictProduct,
		AuthorityTargetInput{AccountID: accountID, StoreID: &s.StoreID, ProductIDs: []string{" p1 ", "p1", "", "p2"}},
		AuthorityTargetInput{AccountID: buyer}))
	if len(o.Targets) != 2 {
		t.Fatalf("targets=%+v", o.Targets)
	}
	first, second := o.Targets[0], o.Targets[1]
	if !slices.Equal(first.ProductIDs, []string{"p1", "p2"}) || len(first.Stores) != 1 || first.Stores[0].Coverage != CoverageHeld || first.NoSellerData {
		t.Fatalf("第一個對象：%+v", first)
	}
	if !second.NoSellerData || len(second.Stores) != 0 {
		t.Fatalf("沒有店家的帳號要標 no_seller_data：%+v", second)
	}
	// 往後的新 stint 也在不指定店家的對象裡；指定店家的不算。
	if got := f.orderOf(t, o.ID); len(got.Targets) != 2 || got.HoldExtensions != nil {
		t.Fatalf("讀回：%+v", got)
	}
}

// 登記的輸入檢查：每一種拒絕都指到欄位並用核准的句子。
func TestCreateAuthorityOrderRejectsBadInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	otherAccount, other := f.open(t)
	_ = otherAccount
	in := func(mutate func(*CreateAuthorityOrderInput)) error {
		i := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID})
		mutate(&i)
		_, err := f.module.CreateAuthorityOrder(context.Background(), i)
		return err
	}
	long := strings.Repeat("字", 101)
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Agency = " " }), "Agency", "機關名稱必須是 1 到 100 字")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.DocumentNo = long }), "DocumentNo", "文號必須是 1 到 100 字")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Summary = "" }), "Summary", "內容摘要必須是 1 到 4000 字")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Kind = "" }), "Kind", "請選擇來函的種類")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.ReceivedAt = f.now.Add(time.Second) }), "ReceivedAt", "收文時間不能晚於現在，也不能早於 1 年前")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.ReceivedAt = f.now.Add(-366 * 24 * time.Hour) }), "ReceivedAt", "收文時間不能晚於現在")
	tooSoon, tooLate := f.now.Add(30*time.Minute), f.now.Add(31*24*time.Hour)
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.DueAt = &tooSoon }), "DueAt", "期限要在收文後 1 小時到 30 天之間")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.DueAt = &tooLate }), "DueAt", "期限要在收文後 1 小時到 30 天之間")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets = nil }), "Targets", "指名的對象要有 1 到 20 個")
	many := make([]AuthorityTargetInput, 21)
	for i := range many {
		many[i] = AuthorityTargetInput{AccountID: int64(i + 1)}
	}
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets = many }), "Targets", "指名的對象要有 1 到 20 個")
	products := make([]string, 51)
	for i := range products {
		products[i] = uuid.NewV7().String()
	}
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets[0].ProductIDs = products }), "Targets[0].ProductIDs", "每個對象的商品最多 50 個")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets[0].AccountID = 999_999 }), "Targets[0].AccountID", "找不到這個帳號")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets[0].StoreID = &other.StoreID }), "Targets[0].StoreID", "這個店家不屬於這個帳號")
	missing := int64(987_654)
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets[0].StoreID = &missing }), "Targets[0].StoreID", "這個店家不屬於這個帳號")
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) { i.Targets = append(i.Targets, i.Targets[0]) }), "Targets[1].AccountID", "同一個帳號或店家重複了")
	// 重疊：整個帳號加上它的店家（兩個順序都拒絕），核准的句子。
	overlap := "這個帳號已經整個列為對象，不用再列它的店家。"
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) {
		i.Targets = append(i.Targets, AuthorityTargetInput{AccountID: accountID, StoreID: &s.StoreID})
	}), "Targets[1].StoreID", overlap)
	wantInvalid(t, in(func(i *CreateAuthorityOrderInput) {
		i.Targets = []AuthorityTargetInput{{AccountID: accountID, StoreID: &s.StoreID}, {AccountID: accountID}}
	}), "Targets[0].StoreID", overlap)
	// 同一個帳號列不同店家不是重疊。
	if f.count(t, "SELECT count(*) FROM seller.authority_orders") != 0 {
		t.Fatal("被拒絕的登記什麼都沒有寫")
	}
}

// 冪等鍵的範圍是（登記的管理員, request_id）：同一位管理員同樣的內容回原本那一份，內容不同回 invalid_argument，另一位管理員不會讀到別人的。
func TestCreateAuthorityOrderRequestIDIsScopedToTheAdmin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	id := uuid.NewV4()
	in := f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: accountID})
	in.RequestID = &id
	first, err := f.module.CreateAuthorityOrder(ctx, in)
	must(t, err)
	again, err := f.module.CreateAuthorityOrder(ctx, in)
	must(t, err)
	if !again.Replayed || again.Order.ID != first.Order.ID || f.count(t, "SELECT count(*) FROM seller.authority_orders") != 1 {
		t.Fatalf("重送要回原本那一份：%+v", again)
	}
	f.now = f.now.Add(time.Minute) // 重送時「現在」已經不同，不影響內容的比對
	again, err = f.module.CreateAuthorityOrder(ctx, in)
	must(t, err)
	if !again.Replayed {
		t.Fatal("時間前進之後重送仍是同一份")
	}
	changed := in
	changed.Summary = "不同的內容"
	_, err = f.module.CreateAuthorityOrder(ctx, changed)
	wantInvalid(t, err, "RequestID", "這次送出的內容和同一個請求先前送出的不同")
	otherAdmin := in
	otherAdmin.AdminID = authorityRegistrar + 1
	second, err := f.module.CreateAuthorityOrder(ctx, otherAdmin)
	must(t, err)
	if second.Replayed || second.Order.ID == first.Order.ID {
		t.Fatal("另一位管理員帶同一個值要建立自己的一份")
	}
}

// 保存只擋被指名的帳號（或店家）的清理：指名帳號——全部的 stint 都不清；指名店家——同帳號的另一個 stint 照清；別的帳號照清；hold_until 到期那一刻照清。
// mutation：StoreHolds 的保存條件拿掉 `pt.store_id IS NULL OR` → 指名帳號的測試失敗；改成只看帳號 → 指名店家的測試失敗。
func TestAuthorityHoldStopsThePurgeOnlyForTheNamedStoreOrAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	wholeAccount, wholeStore := f.closedStore(t)
	storeAccount, firstStint := f.closedStore(t)
	secondStint, err := f.module.Apply(ctx, validApply(storeAccount))
	must(t, err)
	f.setContact(t, secondStint.StoreID)
	_, err = f.module.CloseCurrent(ctx, storeAccount)
	must(t, err)
	_, free := f.closedStore(t)
	expiredAccount, expiredStore := f.closedStore(t)
	// 來函在關店一天之後收文：保存期限（收文後 184 天）才晚於保留期（關店後 184 天，同值）；與關店同時收文的來函保存不會比保留期更久。
	closedAt := f.now
	f.now = closedAt.Add(24 * time.Hour)

	named := f.createOrder(t, f.orderIn(AuthorityDataRequest,
		AuthorityTargetInput{AccountID: wholeAccount}, AuthorityTargetInput{AccountID: storeAccount, StoreID: &firstStint.StoreID}))
	old := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: expiredAccount})
	old.ReceivedAt = f.now.Add(-200 * 24 * time.Hour) // 保存期限（收文後 184 天）已在 16 天前到期
	f.createOrder(t, old)

	f.now = closedAt.Add(StoreRetention + time.Hour)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 3 || f.purged(t, wholeStore.StoreID) || f.purged(t, firstStint.StoreID) ||
		!f.purged(t, secondStint.StoreID) || !f.purged(t, free.StoreID) || !f.purged(t, expiredStore.StoreID) {
		t.Fatalf("只有被指名的不清：n=%d whole=%v first=%v second=%v free=%v expired=%v", n,
			f.purged(t, wholeStore.StoreID), f.purged(t, firstStint.StoreID), f.purged(t, secondStint.StoreID), f.purged(t, free.StoreID), f.purged(t, expiredStore.StoreID))
	}
	// 逐家的涵蓋狀態照實顯示。
	o := f.orderOf(t, named.ID)
	if got := o.Targets[1].Stores; len(got) != 1 || got[0].StoreID != firstStint.StoreID || got[0].Coverage != CoverageHeld {
		t.Fatalf("指名店家的涵蓋：%+v", got)
	}
	// 邊界：hold_until 之前一微秒不清，到了那一刻清。
	f.now = named.HoldUntil.Add(-time.Microsecond)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 {
		t.Fatalf("保存期限之前不清：n=%d err=%v", n, err)
	}
	f.now = named.HoldUntil
	if n, err := f.module.PurgePII(ctx); err != nil || n != 2 || !f.purged(t, wholeStore.StoreID) || !f.purged(t, firstStint.StoreID) {
		t.Fatalf("保存期限到了就清：n=%d err=%v", n, err)
	}
	o = f.orderOf(t, named.ID)
	for _, tgt := range o.Targets {
		for _, st := range tgt.Stores {
			if st.Coverage != CoveragePurgedAfterHoldEnded {
				t.Fatalf("保存期限已過、之後被清理：%+v", st)
			}
		}
	}
}

// 第一段清理（退場）鎖住候選之後再問保存：寫入保存的一方（這裡是登記來函，最壞的情形：不鎖店家列）在第一個語句開始之後 commit，
// 鎖住之後的 StoreHolds（新的快照）看得到它，這家店不退場（§2.4 情形 2）。
// mutation：purge.retireBatch 拿掉鎖住之後的 StoreHolds（或改成在選候選之前問）→ 店家退場，斷言失敗。
func TestPurgeRechecksTheAuthorityHoldAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.closedStore(t)
	f.now = f.now.Add(StoreRetireAfter + time.Hour)

	writer, err := f.db.Pool().Begin(ctx) // 連線 B：插入來函與對象，不鎖店家列、不 commit
	must(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	f.insertRawOrder(t, writer, accountID, f.now.Add(24*time.Hour))

	h := f.holdPurgeAfterCandidates(t) // 連線 A：第一個語句看不到 B 的插入，鎖住店家
	must(t, writer.Commit(ctx))        // B 在 A 的第一個語句之後 commit
	h.release(t)
	if f.retired(t, s.StoreID) || f.purged(t, s.StoreID) {
		t.Fatal("鎖住之後看到保存，店家不能退場")
	}
}

// 第二段清理（個資）的同一種交錯：店家已退場，B 在第二段的第一個語句之後 commit 保存，鎖住之後的 StoreHolds 看得到它，個資不清。
// mutation：purge.purgeBatch 拿掉 StoreHolds → 個資被清，斷言失敗。
func TestPurgeStageTwoRechecksTheAuthorityHoldAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.closedStore(t)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err := f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("要先退場")
	}
	f.now = closedAt.Add(StoreRetention + time.Hour)

	writer, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	f.insertRawOrder(t, writer, accountID, f.now.Add(24*time.Hour))

	h := f.holdPurgeStageTwoAfterCandidates(t)
	must(t, writer.Commit(ctx))
	h.release(t)
	if h.n != 0 || f.purged(t, s.StoreID) {
		t.Fatalf("鎖住之後看到保存，個資不能被清（n=%d）", h.n)
	}
}

// 第二段清理也重新判斷申覆的 legal hold：已退場的店家，帳號上之後才建立的終止合作（21.3，建在已關閉的最近 stint）有待處理的申覆，
// B 在第二段的第一個語句之後 commit 申覆（以原始 SQL，不鎖店家列：最壞的寫入方），鎖住之後的 StoreHolds 看得到它，個資不清。
// mutation：holds 對第二段傳 appeal=false → 個資被清，斷言失敗。
func TestPurgeStageTwoRechecksTheAppealHoldAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.closedStore(t)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err := f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("要先退場")
	}
	m := f.terminate(t, s.StoreID, GroundOtherSerious)
	f.now = closedAt.Add(StoreRetention + time.Hour)

	writer, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	_, err = writer.Exec(ctx, `INSERT INTO seller.measure_requests (id, measure_id, kind, basis, state, reason, revision, submitted_at)
		SELECT $1, id, 'APPEAL', basis, 'PENDING', '異議', current_revision, $2 FROM seller.measures WHERE id = $3`, uuid.NewV7(), f.now, m.ID)
	must(t, err)

	h := f.holdPurgeStageTwoAfterCandidates(t)
	must(t, writer.Commit(ctx))
	h.release(t)
	if h.n != 0 || f.purged(t, s.StoreID) {
		t.Fatalf("鎖住之後看到申覆的 legal hold，個資不能被清（n=%d）", h.n)
	}
}

// insertRawOrder 在 tx 裡直接寫一份指名 accountID 的來函（不經模組、不鎖店家列）：模擬最壞的寫入方。
func (f *fixture) insertRawOrder(t *testing.T, tx pgx.Tx, accountID int64, holdUntil time.Time) {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	_, err := tx.Exec(ctx, `INSERT INTO seller.authority_orders (id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id)
		VALUES ($1, 'x', 'y', $2, 'DATA_REQUEST', 's', $3, $4, $2, 31)`, id, f.now, f.now.Add(time.Hour), holdUntil)
	must(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO seller.authority_order_targets (order_id, position, account_id) VALUES ($1, 0, $2)`, id, accountID)
	must(t, err)
}

// 登記在清理之後才拿到店家列：清理先鎖住候選、登記等鎖；清理 commit 之後登記成功，那家店的涵蓋狀態是「登記之前已依保留期限清除」（§2.4 情形 3），
// 不是競態而是登記太晚。涵蓋狀態不比較時鐘：清理的實例時鐘比登記的實例快 1 小時（清理寫的 pii_purged_at 晚於登記的 registered_at），狀態仍是「登記之前已清除」。
// mutation：StoreCoverageOf 改回 `purgedAt <= RegisteredAt` 判斷（或拿掉 InsertAuthorityOrderPrepurgedStores）→ 狀態變成 PURGED_AFTER_HOLD_ENDED，斷言失敗。
func TestAuthorityOrderRegisteredAfterThePurgeShowsAlreadyPurged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.closedStore(t)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err := f.module.PurgePII(context.Background())
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("關店 30 天後要已退場")
	}
	f.now = closedAt.Add(StoreRetention + time.Hour)
	keys, mac := sealtest.Keys(t)
	registrar := f.newModuleClock(t, f.payuni, f.jobs, keys, mac, func() time.Time { return f.now.Add(-time.Hour) })
	h := f.holdPurgeStageTwoAfterCandidates(t)
	var created CreateAuthorityOrderResult
	var createErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		in := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID})
		in.ReceivedAt = f.now.Add(-2 * time.Hour)
		created, createErr = registrar.CreateAuthorityOrder(context.Background(), in)
	}()
	if !f.waitBlocked(t, done) {
		t.Fatalf("登記要被清理鎖住的店家列擋住：%v", createErr)
	}
	h.release(t)
	<-done
	must(t, createErr)
	if h.n != 1 || !f.purged(t, s.StoreID) {
		t.Fatalf("清理照常完成：n=%d", h.n)
	}
	stores := created.Order.Targets[0].Stores
	if len(stores) != 1 || stores[0].Coverage != CoveragePurgedBeforeRegistration || stores[0].StoreName != nil {
		t.Fatalf("涵蓋狀態：%+v", stores)
	}
	if !stores[0].PurgedAt.After(created.Order.RegisteredAt) {
		t.Fatalf("測試前提：清理的時間要晚於登記的時間（時鐘差）：purged=%v registered=%v", stores[0].PurgedAt, created.Order.RegisteredAt)
	}
	// 之後讀取、延長保存都不改變這個事實。
	later := f.orderOf(t, created.Order.ID)
	if got := later.Targets[0].Stores[0].Coverage; got != CoveragePurgedBeforeRegistration {
		t.Fatalf("重讀的涵蓋狀態：%s", got)
	}
}

// 保存期限已過、店家被清理之後才延長（設計允許已到期也能延長）：已清理的店家是「保存期限已過後清除」，不因目前的 hold_active 為 true 而變成保存中或保存期限已過；
// 還沒清理的另一家（同一份來函之外的對照）不受影響。
// mutation：StoreCoverageOf 拿掉 purgedAt != nil 的分支 → 已清理的店家變成 HELD，斷言失敗。
func TestExtendingTheHoldAfterThePurgeKeepsTheStoreShownPurged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.closedStore(t)
	o := f.holdOrder(t, accountID)
	f.now = o.HoldUntil.Add(time.Hour) // 保存期限已過，保留期也滿了
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 || !f.purged(t, s.StoreID) {
		t.Fatalf("保存期限過後清理：n=%d err=%v", n, err)
	}
	got := f.orderOf(t, o.ID)
	if c := got.Targets[0].Stores[0].Coverage; c != CoveragePurgedAfterHoldEnded || got.HoldActive {
		t.Fatalf("清理之後：%s hold_active=%v", c, got.HoldActive)
	}
	extended, err := f.module.ExtendAuthorityOrderHold(ctx, ExtendAuthorityOrderHoldInput{
		ID: o.ID, AdminID: authorityRegistrar, ExpectedVersion: got.Version, HoldUntil: f.now.Add(365 * 24 * time.Hour), Reason: "進入訴訟",
	})
	must(t, err)
	if !extended.HoldActive {
		t.Fatal("延長之後保存有效")
	}
	if c := f.orderOf(t, o.ID).Targets[0].Stores[0].Coverage; c != CoveragePurgedAfterHoldEnded {
		t.Fatalf("延長保存之後，已清理的店家仍是保存期限已過後清除：%s", c)
	}
}

// 恢復營業不看來函的保存（保存只擋刪除、不改變業務結果）：保留期內撤銷終止，被保存的店家照常恢復；保留期已滿而被保存的店家，撤銷終止時和沒有來函的店家一樣
// 只撤銷紀錄、不恢復營業，只是還沒被清理。
// mutation：closing.ReopenPlan 也看 StoreHolds 的 preservation_hold → 被保存而保留期已滿的店家與對照組的結果不同（保留期內的 reopen 不受影響），斷言失敗。
func TestReopenIgnoresTheAuthorityHold(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	heldAccount, held, heldMeasure := f.terminated(t, GroundCeasedBusiness)
	_, control, controlMeasure := f.terminated(t, GroundCeasedBusiness)
	o := f.holdOrder(t, heldAccount)
	// 延長到一年之後：保存期限遠比終止的保留期（申覆期限）長。
	_, err := f.module.ExtendAuthorityOrderHold(ctx, ExtendAuthorityOrderHoldInput{
		ID: o.ID, AdminID: authorityRegistrar, ExpectedVersion: o.Version, HoldUntil: f.now.Add(365 * 24 * time.Hour), Reason: "進入訴訟",
	})
	must(t, err)

	// 保留期已滿、清理還沒跑：兩家都只撤銷紀錄、不恢復；被保存的這家不被清理，對照組被清。
	f.now = heldMeasure.StartsAt.Add(4416*time.Hour + time.Hour)
	f.revoke(t, heldMeasure)
	f.revoke(t, controlMeasure)
	for _, c := range []struct {
		name  string
		store Store
	}{{"被保存的", held}, {"對照組", control}} {
		if got := f.storeRow(t, c.store.StoreID); got.Status != Closed || f.storeClosures(t, c.store.StoreID) != 0 {
			t.Fatalf("%s：保留期已滿，撤銷終止只撤銷紀錄、不恢復：%+v", c.name, got)
		}
	}
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 || f.purged(t, held.StoreID) || !f.purged(t, control.StoreID) {
		t.Fatalf("保存的店家還沒被清，對照組被清：n=%d", n)
	}

	// 保留期內撤銷：被保存的店家照常恢復營業。
	again, againStore, againMeasure := f.terminated(t, GroundCeasedBusiness)
	f.holdOrder(t, again)
	f.now = againMeasure.StartsAt.Add(40 * 24 * time.Hour)
	f.revoke(t, againMeasure)
	if got := f.storeRow(t, againStore.StoreID); got.Status != Active || !f.isSeller(t, again) {
		t.Fatalf("保留期內撤銷要恢復，不論有沒有保存：%+v", got)
	}
}

// 食品登錄審核紀錄（關店 2 年期滿）的刪除也遵守保存：期限內不刪，到期那一刻刪。
func TestFoodReviewDeletionRespectsTheHold(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.kycApprovedStoreWithAccount(t, idA, bankAcct)
	f.approveFood(t, s, "HELD-1")
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now
	f.now = f.now.Add(StoreRetention + time.Hour)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	expiry := closedAt.In(twtime.Taipei).AddDate(2, 0, 0)
	f.now = expiry.Add(-10 * 24 * time.Hour)
	o := f.holdOrder(t, accountID) // 保存到 f.now + 184 天，超過 2 年期滿
	rows := func() int64 {
		return f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id=$1", s.StoreID)
	}
	f.now = expiry.Add(time.Second)
	if _, err := f.module.PurgePII(ctx); err != nil || rows() != 1 {
		t.Fatalf("2 年期滿但還在保存：%d err=%v", rows(), err)
	}
	f.now = o.HoldUntil.Add(-time.Microsecond)
	if _, err := f.module.PurgePII(ctx); err != nil || rows() != 1 {
		t.Fatalf("保存期限之前不刪：%d err=%v", rows(), err)
	}
	f.now = o.HoldUntil
	if _, err := f.module.PurgePII(ctx); err != nil || rows() != 0 {
		t.Fatalf("保存期限到了就刪：%d err=%v", rows(), err)
	}
}

// 審核紀錄的刪除鎖住之後再判斷一次：登記來函的一方不鎖店家列、在第一個語句之後 commit，審核紀錄不刪。
// mutation：purge.deleteExpiredFoodReviews 拿掉鎖住之後的 StoreHolds → 審核紀錄被刪，斷言失敗。
func TestFoodReviewDeletionRechecksTheHoldAfterLocking(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.kycApprovedStoreWithAccount(t, idA, bankAcct)
	f.approveFood(t, s, "RACE-1")
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now
	f.now = f.now.Add(StoreRetention + time.Hour)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	f.now = closedAt.In(twtime.Taipei).AddDate(2, 0, 0).Add(time.Second)

	writer, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	f.insertRawOrder(t, writer, accountID, f.now.Add(24*time.Hour))

	locked, proceed, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.module.SetPurgeAfterFoodCandidates(func() {
		once.Do(func() { close(locked) })
		<-proceed
	})
	t.Cleanup(func() { f.module.SetPurgeAfterFoodCandidates(nil) })
	var purgeErr error
	go func() {
		defer close(done)
		_, purgeErr = f.module.PurgePII(ctx)
	}()
	select {
	case <-locked:
	case <-done:
		t.Fatalf("清理沒有審核紀錄的候選：%v", purgeErr)
	case <-time.After(20 * time.Second):
		t.Fatal("清理沒有到達 hook")
	}
	must(t, writer.Commit(ctx))
	close(proceed)
	<-done
	must(t, purgeErr)
	if n := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id=$1", s.StoreID); n != 1 {
		t.Fatalf("再判斷看到保存，審核紀錄不能刪：%d", n)
	}
}

// 調取資料完成時，保存期限取收文與提供兩者較晚的（設計 Q5 (b)）：提供時間晚於收文時延到提供時間＋184 天並記一列 DATA_PROVIDED；
// 提供時間等於收文時沒有變（不記）；已經被延得更遠的不縮短。提供時間必須在 [收文, 現在]，而且只有調取資料的來函可以帶。
// mutation：HoldAfterProvision 改成取較早的或把延長條件反過來 → 這裡失敗。
func TestCompletingADataRequestExtendsTheHoldToSixMonthsAfterProvision(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID})
	in.ReceivedAt = f.now.Add(-5 * 24 * time.Hour)
	o := f.createOrder(t, in)
	provide := f.now.Add(-2 * 24 * time.Hour)
	done, err := f.completeOrder(o, &provide, f.notifyAll(o))
	must(t, err)
	want := provide.Add(4416 * time.Hour)
	if !done.HoldUntil.Equal(want) || done.State() != AuthorityStateCompleted {
		t.Fatalf("hold=%v 預期 %v", done.HoldUntil, want)
	}
	if len(done.HoldExtensions) != 1 || done.HoldExtensions[0].Cause != HoldExtensionDataProvided || !done.HoldExtensions[0].From.Equal(o.HoldUntil) ||
		!done.HoldExtensions[0].To.Equal(want) || done.HoldExtensions[0].AdminID != authorityCompleter {
		t.Fatalf("延長歷史：%+v", done.HoldExtensions)
	}

	// 提供時間就是收文時間：沒有比目前晚，不延長、不記。
	equal := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID})
	equal.ReceivedAt = f.now.Add(-3 * 24 * time.Hour)
	o2 := f.createOrder(t, equal)
	atReceipt := o2.ReceivedAt
	d2, err := f.completeOrder(o2, &atReceipt, f.notifyAll(o2))
	must(t, err)
	if !d2.HoldUntil.Equal(o2.HoldUntil) || len(d2.HoldExtensions) != 0 {
		t.Fatalf("提供時間等於收文不延長：%+v", d2.HoldExtensions)
	}
	// 管理員已延得更遠：完成不縮短。
	o3 := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID}))
	far := f.now.Add(800 * 24 * time.Hour)
	o3b, err := f.module.ExtendAuthorityOrderHold(ctx, ExtendAuthorityOrderHoldInput{ID: o3.ID, AdminID: authorityRegistrar, ExpectedVersion: o3.Version, HoldUntil: far, Reason: "訴訟"})
	must(t, err)
	d3, err := f.completeOrder(o3b, nil, f.notifyAll(o3b))
	must(t, err)
	if !d3.HoldUntil.Equal(far) || len(d3.HoldExtensions) != 1 {
		t.Fatalf("完成不縮短保存：%v %+v", d3.HoldUntil, d3.HoldExtensions)
	}

	// 範圍與種類。
	o4 := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID}))
	before, after := o4.ReceivedAt.Add(-time.Second), f.now.Add(time.Second)
	_, err = f.completeOrder(o4, &before, f.notifyAll(o4))
	wantInvalid(t, err, "ProvideTime", "提供資料的時間不能早於收文，也不能晚於現在")
	_, err = f.completeOrder(o4, &after, f.notifyAll(o4))
	wantInvalid(t, err, "ProvideTime", "提供資料的時間不能早於收文，也不能晚於現在")
	other := f.createOrder(t, f.orderIn(AuthoritySuspendAccount, AuthorityTargetInput{AccountID: accountID}))
	now := f.now
	_, err = f.completeOrder(other, &now, f.notifyAll(other))
	wantInvalid(t, err, "ProvideTime", "只有調取資料的來函可以填寫提供資料的時間")
	if got := f.orderOf(t, o4.ID); !got.Open() || got.Version != 1 {
		t.Fatal("被拒絕的完成什麼都沒有寫")
	}
}

// 完成時每個對象都要有告知；暫不告知必填確認日期；由措施告知只限暫停帳號、限制商品而且已有導出的措施；已完成不能再完成；etag 過時回版本衝突。
func TestCompletingAnOrderRequiresAndValidatesEveryTargetNotice(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA := f.open(t)
	accountB := f.account(t)
	o := f.createOrder(t, f.orderIn(AuthoritySuspendAccount, AuthorityTargetInput{AccountID: accountA}, AuthorityTargetInput{AccountID: accountB}))
	notified := f.notifyAll(o)

	_, err := f.completeOrder(o, nil, notified[:1])
	wantInvalid(t, err, "TargetNotices", "請為每個指名的對象選擇告知的方式")
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], notified[0]})
	wantInvalid(t, err, "TargetNotices[1]", "請為每個指名的對象選擇告知的方式")
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], {Position: 5, Notice: notified[1].Notice}})
	wantInvalid(t, err, "TargetNotices[1]", "請為每個指名的對象選擇告知的方式")
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], {Position: 1}})
	wantInvalid(t, err, "TargetNotices[1].Notice.Kind", "請為每個指名的對象選擇告知的方式")
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], {Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld}}})
	wantInvalid(t, err, "TargetNotices[1].Notice.RecheckOn", "暫不告知時，請填寫向機關確認的日期")
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], {Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified}}})
	wantInvalid(t, err, "TargetNotices[1].Notice.NotifiedAt", "告知的時間不能早於收文，也不能晚於現在")
	early := o.ReceivedAt.Add(-time.Second)
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{notified[0], {Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &early}}})
	wantInvalid(t, err, "TargetNotices[1].Notice.NotifiedAt", "告知的時間不能早於收文")
	// 由措施告知：還沒有導出的措施 → 拒絕；建立之後（對象 A 有、對象 B 沒有）A 可以、B 不行。
	byMeasure := TargetNoticeInput{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeMeasure}}
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{byMeasure, notified[1]})
	wantInvalid(t, err, "TargetNotices[0].Notice.Kind", "只有暫停帳號或限制商品的來函、而且已建立措施時")
	review := f.now.Add(7 * 24 * time.Hour)
	_, err = f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: sA.StoreID, Kind: MeasureSuspension, StatementToSeller: "依主管機關來函暫停", Note: "來函導出", ReviewBy: &review,
		Risk: &RiskBasis{Ground: RiskFraud}, SourceAuthorityOrderID: &o.ID,
	})
	must(t, err)
	bToo := TargetNoticeInput{Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeMeasure}}
	_, err = f.completeOrder(o, nil, []TargetNoticeInput{byMeasure, bToo})
	wantInvalid(t, err, "TargetNotices[1].Notice.Kind", "只有暫停帳號或限制商品的來函、而且已建立措施時")
	// 調取資料不能選。
	data := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountA}))
	_, err = f.completeOrder(data, nil, []TargetNoticeInput{byMeasure})
	wantInvalid(t, err, "TargetNotices[0].Notice.Kind", "只有暫停帳號或限制商品的來函、而且已建立措施時")

	if got := f.orderOf(t, o.ID); !got.Open() || got.Version != 1 || f.count(t, "SELECT count(*) FROM seller.authority_order_targets WHERE notice IS NOT NULL") != 0 {
		t.Fatal("被拒絕的完成什麼都沒有寫")
	}
	// 版本過時。
	stale := o
	stale.Version = 99
	_, err = f.completeOrder(stale, nil, []TargetNoticeInput{byMeasure, notified[1]})
	wantKind(t, err, apperr.KindVersionConflict)
	// 成功：A 由措施告知、B 暫不告知。
	recheck := day(2026, 10, 20)
	withheld := TargetNoticeInput{Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck, Note: "偵查不公開"}}
	done, err := f.completeOrder(o, nil, []TargetNoticeInput{byMeasure, withheld})
	must(t, err)
	if done.State() != AuthorityStateCompleted || done.Version != 2 || done.Targets[0].Notice.Kind != AuthorityNoticeMeasure ||
		done.Targets[1].Notice.Kind != AuthorityNoticeWithheld || !done.Targets[1].Notice.RecheckOn.Equal(recheck) || done.Targets[1].Notice.Note != "偵查不公開" {
		t.Fatalf("完成：%+v", done.Targets)
	}
	if len(done.Availability.Complete) == 0 {
		t.Fatal("已完成不能再完成")
	}
	again := done
	_, err = f.completeOrder(again, nil, f.notifyAll(done))
	wantBlocked(t, err, "這筆來函已完成。")
}

// 暫不告知之後：改成已告知或寫新的確認日期，每次改變留歷史（被取代的值、誰、何時）、讓版本前進；只有暫不告知的對象能改；不能改成由措施告知。
func TestWithheldNoticeCanBeReplacedAndKeepsHistory(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	o := f.createOrder(t, f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: accountID}))
	recheck := day(2026, 10, 5)
	done, err := f.completeOrder(o, nil, []TargetNoticeInput{{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck}}})
	must(t, err)
	record := func(order AdminAuthorityOrder, position int, n AuthorityNoticeInput) (AdminAuthorityOrder, error) {
		return f.module.RecordAuthorityTargetNotice(ctx, RecordAuthorityTargetNoticeInput{ID: order.ID, AdminID: 33, ExpectedVersion: order.Version, Position: position, Notice: n})
	}
	next := day(2026, 10, 30)
	moved, err := record(done, 0, AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &next, Note: "機關再延"})
	must(t, err)
	if moved.Version != done.Version+1 || !moved.Targets[0].Notice.RecheckOn.Equal(next) || moved.Targets[0].Notice.RecordedByAdminID != 33 {
		t.Fatalf("新的確認日期：%+v", moved.Targets[0].Notice)
	}
	at := f.now
	notified, err := record(moved, 0, AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at, Note: "已 email 告知"})
	must(t, err)
	if notified.Targets[0].Notice.Kind != AuthorityNoticeNotified {
		t.Fatalf("已告知：%+v", notified.Targets[0].Notice)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.authority_target_notice_history WHERE order_id=$1 AND position=0 AND notice='WITHHELD'", o.ID); n != 2 {
		t.Fatalf("兩次取代各留一列歷史：%d", n)
	}
	if f.count(t, "SELECT count(*) FROM seller.authority_target_notice_history WHERE order_id=$1 AND seq=2 AND recheck_on=$2 AND replaced_by_admin_id=33", o.ID, next) != 1 {
		t.Fatal("第二列歷史是被取代的新確認日期")
	}
	// 已告知的不能再改；不是暫不告知；版本過時；位置不存在；方式不合法。
	_, err = record(notified, 0, AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &next})
	wantBlocked(t, err, "這個對象不是暫不告知的狀態，不能再記下告知")
	_, err = record(done, 0, AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &next})
	wantKind(t, err, apperr.KindVersionConflict)
	_, err = record(notified, 3, AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &next})
	wantInvalid(t, err, "TargetPosition", "找不到這個對象")
	fresh := f.createOrder(t, f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: accountID}))
	_, err = record(fresh, 0, AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at})
	wantBlocked(t, err, "這個對象不是暫不告知的狀態")
	w2, err := f.completeOrder(fresh, nil, []TargetNoticeInput{{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck}}})
	must(t, err)
	_, err = record(w2, 0, AuthorityNoticeInput{Kind: AuthorityNoticeMeasure})
	wantInvalid(t, err, "Notice.Kind", "請為每個指名的對象選擇告知的方式")
	_, err = record(w2, 0, AuthorityNoticeInput{Kind: AuthorityNoticeWithheld})
	wantInvalid(t, err, "Notice.RecheckOn", "暫不告知時，請填寫向機關確認的日期")
}

// 延長保存只能延長、已完成也能延、留下前後值與原因；hold_until 等於最後一列的 to_until（J5）。
func TestExtendingTheHoldIsOnlyForwardAndKeepsHistory(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	o := f.holdOrder(t, accountID)
	extend := func(order AdminAuthorityOrder, until time.Time, reason string) (AdminAuthorityOrder, error) {
		return f.module.ExtendAuthorityOrderHold(ctx, ExtendAuthorityOrderHoldInput{ID: order.ID, AdminID: 34, ExpectedVersion: order.Version, HoldUntil: until, Reason: reason})
	}
	_, err := extend(o, o.HoldUntil, "沒有延長")
	wantBlocked(t, err, "新的保存期限必須晚於目前的保存期限")
	_, err = extend(o, o.HoldUntil.Add(-time.Hour), "縮短")
	wantBlocked(t, err, "新的保存期限必須晚於目前的保存期限")
	_, err = extend(o, o.HoldUntil.Add(time.Hour), " ")
	wantInvalid(t, err, "Reason", "延長的原因必須是 1 到 2000 字")
	stale := o
	stale.Version = 7
	_, err = extend(stale, o.HoldUntil.Add(time.Hour), "原因")
	wantKind(t, err, apperr.KindVersionConflict)
	first := o.HoldUntil.Add(90 * 24 * time.Hour)
	e1, err := extend(o, first, "進入訴訟")
	must(t, err)
	done, err := f.completeOrder(e1, nil, f.notifyAll(e1))
	must(t, err)
	second := first.Add(100 * 24 * time.Hour)
	e2, err := extend(done, second, "裁判確定後 3 個月")
	must(t, err)
	if e2.State() != AuthorityStateCompleted || !e2.HoldUntil.Equal(second) || e2.Version != 4 {
		t.Fatalf("已完成也能延長：%+v", e2)
	}
	if len(e2.HoldExtensions) < 2 {
		t.Fatalf("歷史：%+v", e2.HoldExtensions)
	}
	last := e2.HoldExtensions[len(e2.HoldExtensions)-1]
	prev := e2.HoldExtensions[len(e2.HoldExtensions)-2]
	if last.Cause != HoldExtensionAdmin || last.Reason != "裁判確定後 3 個月" || !last.From.Equal(first) || !last.To.Equal(second) || !prev.To.Equal(first) || !prev.From.Equal(o.HoldUntil) {
		t.Fatalf("前後值：prev=%+v last=%+v", prev, last)
	}
	f.assertHoldInvariant(t)
}

// J5：hold_until 等於最後一列延長的 to_until；沒有延長時等於收文加預設保存期。
func (f *fixture) assertHoldInvariant(t *testing.T) {
	t.Helper()
	if n := f.count(t, `SELECT count(*) FROM seller.authority_orders o WHERE o.hold_until <> COALESCE(
		(SELECT e.to_until FROM seller.authority_order_hold_extensions e WHERE e.order_id = o.id ORDER BY e.seq DESC LIMIT 1), o.received_at + interval '4416 hours')`); n != 0 {
		t.Fatalf("%d 份來函的 hold_until 不等於最後一列延長（J5）", n)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.authority_orders o WHERE o.completed_at IS NOT NULL AND EXISTS (
		SELECT 1 FROM seller.authority_order_targets t WHERE t.order_id = o.id AND t.notice IS NULL)`); n != 0 {
		t.Fatalf("%d 份已完成的來函有對象沒有告知", n)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.authority_orders o WHERE o.completed_at IS NULL AND EXISTS (
		SELECT 1 FROM seller.authority_order_targets t WHERE t.order_id = o.id AND t.notice IS NOT NULL)`); n != 0 {
		t.Fatalf("%d 份未完成的來函有對象已有告知", n)
	}
}

// 清單：OPEN 依期限舊到新，其他依收文時間新到舊；翻頁不漏不重；狀態與帳號篩選等於 Open() 與指名。
func TestListAuthorityOrdersOrderingAndFilters(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a1, _ := f.open(t)
	a2, _ := f.open(t)
	var open, all []AdminAuthorityOrder
	for i := range 5 {
		f.now = f.now.Add(time.Hour)
		in := f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: a1})
		if i%2 == 1 {
			in.Targets[0].AccountID = a2
		}
		due := f.now.Add(time.Duration(20-3*i) * time.Hour) // 後登記的期限比較早
		in.DueAt = &due
		o := f.createOrder(t, in)
		all = append(all, o)
		if i == 3 {
			_, err := f.completeOrder(o, nil, f.notifyAll(o))
			must(t, err)
		} else {
			open = append(open, o)
		}
	}
	ids := func(state *AuthorityOrderState, account int64) []uuid.UUID {
		var out []uuid.UUID
		var after *AuthorityCursor
		for {
			page, err := f.module.AuthorityOrders(ctx, AuthorityOrderListInput{State: state, AccountID: account, PageSize: 2, After: after})
			must(t, err)
			for _, it := range page.Items {
				out = append(out, it.ID)
			}
			if page.Next == nil {
				return out
			}
			after = page.Next
		}
	}
	openState, doneState := AuthorityStateOpen, AuthorityStateCompleted
	wantOpen := []uuid.UUID{open[3].ID, open[2].ID, open[1].ID, open[0].ID} // 期限舊到新
	if got := ids(&openState, 0); !slices.Equal(got, wantOpen) {
		t.Fatalf("處理中依期限舊到新：%v 預期 %v", got, wantOpen)
	}
	if got := ids(&doneState, 0); !slices.Equal(got, []uuid.UUID{all[3].ID}) {
		t.Fatalf("已完成：%v", got)
	}
	wantAll := []uuid.UUID{all[4].ID, all[3].ID, all[2].ID, all[1].ID, all[0].ID} // 收文時間新到舊
	if got := ids(nil, 0); !slices.Equal(got, wantAll) {
		t.Fatalf("全部依收文時間新到舊：%v", got)
	}
	if got := ids(nil, a2); !slices.Equal(got, []uuid.UUID{all[3].ID, all[1].ID}) {
		t.Fatalf("指名帳號 a2：%v", got)
	}
	// ListAuthorityOrders(state=OPEN) 的集合等於 Open()。
	for _, id := range ids(&openState, 0) {
		if !f.orderOf(t, id).Open() {
			t.Fatalf("%s 不是處理中", id)
		}
	}
	f.assertHoldInvariant(t)
}

// 措施的來源來函：來函必須指名措施的帳號（指名店家時只算那一家，同帳號的另一個 stint 的措施被拒）；認定可以引用來函；
// ListMeasures(source_authority_order_id) 是 明確連結 ∪ 認定引用（含引用既有認定的措施），與「由措施告知」用的 AuthorityOrderMeasureTargets 同一個定義。
// mutation：少掉清單其中一條（明確連結或認定引用）的條件 → 這裡的集合少一筆而失敗。
func TestMeasureSourceAuthorityOrderNamingAndListFilter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s1 := f.open(t)
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	s2, err := f.module.Apply(ctx, validApply(account))
	must(t, err)
	f.setContact(t, s2.StoreID)
	strangerAccount, stranger := f.open(t)
	_ = strangerAccount

	wholeAccount := f.createOrder(t, f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: account}))
	firstStintOnly := f.createOrder(t, f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: account, StoreID: &s1.StoreID}))
	review := f.now.Add(7 * 24 * time.Hour)
	risk := func(storeID int64, order *uuid.UUID) (CreateMeasureResult, error) {
		return f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, StoreID: storeID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "來函導出", ReviewBy: &review,
			Risk: &RiskBasis{Ground: RiskFraud}, SourceAuthorityOrderID: order,
		})
	}
	// 沒有指名這個帳號 / 指名店家時另一個 stint / 來函不存在 → invalid_argument。
	_, err = risk(stranger.StoreID, &wholeAccount.ID)
	wantInvalid(t, err, "SourceAuthorityOrderID", "這筆來函沒有指名這個帳號")
	_, err = risk(s2.StoreID, &firstStintOnly.ID)
	wantInvalid(t, err, "SourceAuthorityOrderID", "這筆來函沒有指名這個帳號")
	missing := uuid.NewV7()
	_, err = risk(s2.StoreID, &missing)
	wantInvalid(t, err, "SourceAuthorityOrderID", "這筆來函沒有指名這個帳號")
	// request hash 含來函：同一個 request_id 帶不同的來函是不同的請求。
	reqID := uuid.NewV4()
	withReq := func(order uuid.UUID) (CreateMeasureResult, error) {
		return f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, RequestID: &reqID, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
			Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}}, SourceAuthorityOrderID: &order,
		})
	}
	replayable, err := withReq(wholeAccount.ID)
	must(t, err)
	other := f.createOrder(t, f.orderIn(AuthorityOther, AuthorityTargetInput{AccountID: account}))
	_, err = withReq(other.ID)
	wantInvalid(t, err, "RequestID", "這個 request_id 已經用在內容不同的請求")

	// 資料集：明確連結（RISK 停權）、認定引用、引用既有認定的措施、別的來函、沒有來源。
	linked, err := risk(s2.StoreID, &wholeAccount.ID)
	must(t, err)
	viaFinding, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceAuthorityOrderIDs: []uuid.UUID{wholeAccount.ID}}},
	})
	must(t, err)
	cited, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "再警告", Note: "n",
		Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{viaFinding.Measure.Findings[0].ID}},
	})
	must(t, err)
	viaOther, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceAuthorityOrderIDs: []uuid.UUID{other.ID}}},
	})
	must(t, err)
	onFirst, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: stranger.StoreID, Kind: MeasureWarning, StatementToSeller: "無關", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}},
	})
	must(t, err)
	_ = onFirst
	list := func(id uuid.UUID) []uuid.UUID {
		var out []uuid.UUID
		var after *MeasureCursor
		for {
			page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{SourceAuthorityOrderID: &id, PageSize: 2, After: after})
			must(t, err)
			for _, m := range page.Items {
				out = append(out, m.ID)
			}
			if page.Next == nil {
				slices.SortFunc(out, uuid.UUID.Compare)
				return out
			}
			after = page.Next
		}
	}
	want := []uuid.UUID{replayable.Measure.ID, linked.Measure.ID, viaFinding.Measure.ID, cited.Measure.ID}
	slices.SortFunc(want, uuid.UUID.Compare)
	if got := list(wholeAccount.ID); !slices.Equal(got, want) {
		t.Fatalf("來函導出的措施：%v 預期 %v", got, want)
	}
	if got := list(other.ID); !slices.Equal(got, []uuid.UUID{viaOther.Measure.ID}) {
		t.Fatalf("別的來函：%v", got)
	}
	// 認定引用的形狀：重複、空 ID、太多。
	dup := []uuid.UUID{wholeAccount.ID, wholeAccount.ID}
	_, err = f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceAuthorityOrderIDs: dup}},
	})
	wantInvalid(t, err, "Violation.Finding.SourceAuthorityOrderIDs[1]", "引用的來函不能是空的，也不能重複")
	six := make([]uuid.UUID, 6)
	for i := range six {
		six[i] = uuid.NewV7()
	}
	_, err = f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s2.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceAuthorityOrderIDs: six}},
	})
	wantInvalid(t, err, "Violation.Finding.SourceAuthorityOrderIDs", "一個認定最多引用 5 份來函")
	// 引用的來函沒有指名這個帳號。
	_, err = f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: stranger.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceAuthorityOrderIDs: []uuid.UUID{wholeAccount.ID}}},
	})
	wantInvalid(t, err, "Violation.Finding.SourceAuthorityOrderIDs[0]", "這筆來函沒有指名這個帳號")
	// 「由措施告知」的定義與清單一致：導出的措施所在的帳號與店家。
	suspend := f.createOrder(t, f.orderIn(AuthoritySuspendAccount, AuthorityTargetInput{AccountID: account}))
	_, err = risk(s2.StoreID, &suspend.ID)
	must(t, err)
	done, err := f.completeOrder(suspend, nil, []TargetNoticeInput{{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeMeasure}}})
	must(t, err)
	if done.Targets[0].Notice.Kind != AuthorityNoticeMeasure {
		t.Fatalf("%+v", done.Targets[0].Notice)
	}
	f.assertHoldInvariant(t)
}

// 來函保存的述詞只有一份（StoreHolds），店家個資的清理與審核紀錄的刪除（對象不同）都在鎖住候選之後問它；這個資料集鎖住兩條刪除路徑對「哪些來函算有效」一致：
// 每個情境各有兩個帳號，同樣的來函各指名一份——一個帳號的店家早就清理過、審核紀錄剛好 2 年期滿（驗審核紀錄的刪除），另一個帳號的店家剛好保留期滿（驗店家的清理）；
// 同一個時間點跑一次清理。被保存的（指名帳號、指名店家、有效期限內）兩邊都留下，沒有來函與保存已到期的兩邊都清掉。
// mutation：deleteExpiredFoodReviews 不問 StoreHolds（或只用帳號判斷）→ 指名店家的情境審核紀錄被刪而失敗。
func TestBothPreservationPredicatesAgreeOnWhichOrdersCount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	type scenario struct {
		name             string
		keep             bool
		receivedOffset   time.Duration
		byStore          bool
		noOrder          bool
		foodAccount      int64
		foodStore, plain Store
		plainAccount     int64
	}
	scenarios := []*scenario{
		{name: "沒有來函", noOrder: true},
		{name: "指名帳號、保存中", keep: true},
		{name: "指名店家、保存中", keep: true, byStore: true},
		{name: "指名帳號、保存已到期", receivedOffset: -300 * 24 * time.Hour},
	}
	ids := []struct{ id, bank string }{{idA, bankAcct}, {idB, "012345678902"}, {idC, "012345678903"}, {idD, "012345678904"}}
	start := f.now
	for i, sc := range scenarios {
		sc.foodAccount, sc.foodStore = f.kycApprovedStoreWithAccount(t, ids[i].id, ids[i].bank)
		f.approveFood(t, sc.foodStore, "AGREE-"+string(rune('A'+i)))
		_, err := f.module.CloseCurrent(ctx, sc.foodAccount)
		must(t, err)
		sc.plainAccount, sc.plain = f.open(t)
	}
	f.now = start.Add(StoreRetention + time.Hour)
	if n, err := f.module.PurgePII(ctx); err != nil || n != len(scenarios) { // 審核紀錄那一邊的店家先清掉（另一邊的店還開著）
		t.Fatalf("n=%d err=%v", n, err)
	}
	f.now = start.In(twtime.Taipei).AddDate(2, 0, 0)
	for _, sc := range scenarios {
		_, err := f.module.CloseCurrent(ctx, sc.plainAccount)
		must(t, err)
	}
	// 來函在關店十天之後收文：保存期限（收文後 184 天）才晚於保留期（關店後 184 天，同值）。
	plainClosedAt := f.now
	f.now = plainClosedAt.Add(10 * 24 * time.Hour)
	for _, sc := range scenarios {
		if sc.noOrder {
			continue
		}
		for _, side := range []struct {
			account int64
			store   Store
		}{{sc.foodAccount, sc.foodStore}, {sc.plainAccount, sc.plain}} {
			var storeID *int64
			if sc.byStore {
				storeID = &side.store.StoreID
			}
			f.holdOrderAt(t, side.account, storeID, sc.receivedOffset)
		}
	}
	f.now = plainClosedAt.Add(StoreRetention + time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scenarios {
		foodRows := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id=$1", sc.foodStore.StoreID)
		if (foodRows == 1) != sc.keep {
			t.Errorf("%s：保存=%v，審核紀錄 %d 列", sc.name, sc.keep, foodRows)
		}
		if purged := f.purged(t, sc.plain.StoreID); purged == sc.keep {
			t.Errorf("%s：保存=%v，店家已清=%v", sc.name, sc.keep, purged)
		}
	}
}

// holdOrderAt 登記一份調取資料的來函：指名帳號（storeID 為 nil）或店家，收文時間是現在加 receivedOffset。
func (f *fixture) holdOrderAt(t *testing.T, accountID int64, storeID *int64, receivedOffset time.Duration) AdminAuthorityOrder {
	t.Helper()
	in := f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountID, StoreID: storeID})
	in.ReceivedAt = f.now.Add(receivedOffset)
	return f.createOrder(t, in)
}

// 來函資料表的約束：每個案例只違反一條，而且指名是哪一條（wantViolates）。
func TestAuthorityOrderSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	_, other := f.open(t)
	o := f.holdOrder(t, accountID)
	order := func(set string) string {
		cols := map[string]string{
			"agency": "'a'", "document_no": "'b'", "received_at": "now() - interval '1 hour'", "kind": "'OTHER'", "summary": "'s'",
			"due_at": "now() + interval '1 day'", "hold_until": "now() + interval '184 days'", "registered_at": "now()", "registered_by_admin_id": "1",
		}
		extra := ""
		for _, kv := range strings.Split(set, ";") {
			if kv == "" {
				continue
			}
			k, v, _ := strings.Cut(kv, "=")
			if _, ok := cols[k]; ok {
				cols[k] = v
			} else {
				extra += ", " + k + ", " + v
			}
		}
		names, values := "id", "uuidv7()"
		for _, k := range []string{"agency", "document_no", "received_at", "kind", "summary", "due_at", "hold_until", "registered_at", "registered_by_admin_id"} {
			names += ", " + k
			values += ", " + cols[k]
		}
		if extra != "" {
			// extra 是 ", 欄位, 值" 的重複；拆成兩串。
			parts := strings.Split(strings.TrimPrefix(extra, ", "), ", ")
			for i := 0; i+1 < len(parts); i += 2 {
				names += ", " + parts[i]
				values += ", " + parts[i+1]
			}
		}
		return "INSERT INTO seller.authority_orders (" + names + ") VALUES (" + values + ")"
	}
	for _, v := range []struct{ name, constraint, sql string }{
		{"種類不認得", "authority_orders_kind_check", order("kind='OTHERS'")},
		{"機關是空的", "authority_orders_text_check", order("agency=''")},
		{"機關超過 100 字", "authority_orders_text_check", order("agency=repeat('字', 101)")},
		{"文號超過 100 字", "authority_orders_text_check", order("document_no=repeat('字', 101)")},
		{"摘要超過 4000 字", "authority_orders_text_check", order("summary=repeat('字', 4001)")},
		{"保存期限不晚於收文", "authority_orders_times_check", order("hold_until=now() - interval '2 hours'")},
		{"期限不晚於收文", "authority_orders_times_check", order("due_at=now() - interval '2 hours'")},
		{"登記早於收文", "authority_orders_times_check", order("registered_at=now() - interval '2 hours'")},
		{"完成只有時間", "authority_orders_complete_check", order("completed_at=now()")},
		{"完成的處理摘要是空的", "authority_orders_complete_check", order("completed_at=now();completed_by_admin_id=1;resolution_summary=''")},
		{"完成早於登記", "authority_orders_complete_check", order("completed_at=now() - interval '1 day';completed_by_admin_id=1;resolution_summary='x'")},
		{"只有 request_id", "authority_orders_request_check", order("request_id=uuidv7()")},
		{"只有 request_hash", "authority_orders_request_check", order("request_hash='\\x01'")},
		{"管理員是 0", "authority_orders_admin_check", order("registered_by_admin_id=0")},
		{"版本是 0", "authority_orders_version_check", order("version=0")},
	} {
		f.wantViolates(t, v.name, v.constraint, v.sql)
	}
	target := func(position int, account int64, store string, extraColumns, extraValues string) string {
		return fmt.Sprintf("INSERT INTO seller.authority_order_targets (order_id, position, account_id, store_id%s) VALUES ('%s', %d, %d, %s%s)", extraColumns, o.ID, position, account, store, extraValues)
	}
	noticeColumns := ", notice, notified_at, notice_note, recheck_on, notice_recorded_at, notice_recorded_by_admin_id"
	for _, v := range []struct{ name, constraint, sql string }{
		{"店家不屬於帳號", "authority_order_targets_store_fkey", target(1, accountID, fmt.Sprint(other.StoreID), "", "")},
		{"位置超過 19", "authority_order_targets_position_check", fmt.Sprintf("INSERT INTO seller.authority_order_targets (order_id, position, account_id) SELECT '%s', p, %d + p FROM generate_series(1, 20) p", o.ID, accountID+100)},
		{"商品超過 50 個", "authority_order_targets_products_check", target(1, accountID+1, "NULL", ", product_ids", ", array_fill('p'::text, ARRAY[51])")},
		{"帳號是 0", "authority_order_targets_account_check", target(1, 0, "NULL", "", "")},
		{"同一個帳號整個列兩次", "authority_order_targets_unique", target(1, accountID, "NULL", "", "")},
		{"告知只有說明", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", ", notice_note", ", 'x'")},
		{"已告知沒有時間", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", noticeColumns, ", 'NOTIFIED', NULL, NULL, NULL, now(), 1")},
		{"暫不告知沒有確認日期", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", noticeColumns, ", 'WITHHELD', NULL, NULL, NULL, now(), 1")},
		{"由措施告知帶了確認日期", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", noticeColumns, ", 'MEASURE_IS_NOTICE', NULL, NULL, current_date, now(), 1")},
		{"告知方式不認得", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", noticeColumns, ", 'OTHER', NULL, NULL, NULL, now(), 1")},
		{"告知的管理員是 0", "authority_order_targets_notice_check", target(1, accountID+1, "NULL", noticeColumns, ", 'MEASURE_IS_NOTICE', NULL, NULL, NULL, now(), 0")},
		{"告知說明超過 2000 字", "authority_order_targets_note_check", target(1, accountID+1, "NULL", noticeColumns, ", 'NOTIFIED', now(), repeat('字', 2001), NULL, now(), 1")},
	} {
		f.wantViolates(t, v.name, v.constraint, v.sql)
	}
	// 指名店家與整個帳號是不同的鍵（NULLS NOT DISTINCT 只擋兩個都不指定店家）；同一個店家列兩次被擋。
	_, err := f.db.Conn(context.Background()).Exec(context.Background(), target(1, accountID, fmt.Sprint(s.StoreID), "", ""))
	must(t, err)
	f.wantViolates(t, "同一個店家列兩次", "authority_order_targets_unique", target(2, accountID, fmt.Sprint(s.StoreID), "", ""))
	history := func(extra string) string {
		return fmt.Sprintf("INSERT INTO seller.authority_target_notice_history (order_id, position, seq, notice, recheck_on, recorded_at, recorded_by_admin_id, replaced_at, replaced_by_admin_id) VALUES ('%s', 0, 1, %s)", o.ID, extra)
	}
	for _, v := range []struct{ name, constraint, sql string }{
		{"被取代的不是暫不告知", "authority_target_notice_history_notice_check", history("'NOTIFIED', current_date, now() - interval '1 hour', 1, now(), 1")},
		{"取代早於記下", "authority_target_notice_history_notice_check", history("'WITHHELD', current_date, now(), 1, now() - interval '1 hour', 1")},
		{"管理員是 0", "authority_target_notice_history_admin_check", history("'WITHHELD', current_date, now() - interval '1 hour', 0, now(), 1")},
		{"對象不存在", "authority_target_notice_history_target_fkey", strings.Replace(history("'WITHHELD', current_date, now() - interval '1 hour', 1, now(), 1"), "0, 1,", "9, 1,", 1)},
	} {
		f.wantViolates(t, v.name, v.constraint, v.sql)
	}
	extension := func(extra string) string {
		return fmt.Sprintf("INSERT INTO seller.authority_order_hold_extensions (order_id, seq, from_until, to_until, cause, reason, admin_id, created_at) VALUES ('%s', 1, %s)", o.ID, extra)
	}
	for _, v := range []struct{ name, constraint, sql string }{
		{"新值不晚於前值", "authority_order_hold_extensions_order_check", extension("now() + interval '2 days', now() + interval '1 day', 'ADMIN', 'x', 1, now()")},
		{"管理員延長沒有原因", "authority_order_hold_extensions_cause_check", extension("now(), now() + interval '1 day', 'ADMIN', NULL, 1, now()")},
		{"提供資料之後帶了原因", "authority_order_hold_extensions_cause_check", extension("now(), now() + interval '1 day', 'DATA_PROVIDED', 'x', 1, now()")},
		{"原因超過 2000 字", "authority_order_hold_extensions_cause_check", extension("now(), now() + interval '1 day', 'ADMIN', repeat('字', 2001), 1, now()")},
		{"延長的管理員是 0", "authority_order_hold_extensions_admin_check", extension("now(), now() + interval '1 day', 'ADMIN', 'x', 0, now()")},
	} {
		f.wantViolates(t, v.name, v.constraint, v.sql)
	}
}

// 撤銷登記（登記錯誤）：只在完成之前、必附原因、etag 相符；撤銷之後不在處理中、保存不再有效、不能完成、延長或記下告知，也不能再當作措施的來源；
// 紀錄保留（時間、管理員、原因）。同一位管理員重送回 AlreadyRevoked（不比 etag 與原因），別的管理員回 failed_precondition；已完成的來函不能撤銷。
// 可用性（Availability().Revoke 與 API 的 can_revoke）與指令接受什麼一致。
// mutation：RevokeAuthorityOrder 把 etag 比較放回可用性檢查之前 → 拿撤銷之前的 etag 的第二位管理員得到版本衝突，斷言失敗；
// StoreCoverageOf 拿掉 Revoked 的分支 → 撤銷之後的店家變成 HOLD_ENDED，斷言失敗；
// Availability 的 Revoke 不看 Completed → 已完成的來函 can_revoke 為真而指令被 SQL 擋成版本衝突，斷言失敗；
// checkNamingOrders 不看 revoked → 以撤銷的來函建立措施成功，斷言失敗。
func TestRevokeAuthorityOrderReleasesTheHoldAndKeepsTheRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	o := f.holdOrder(t, accountID)
	if len(o.Availability.Revoke) != 0 || !o.HoldActive {
		t.Fatalf("新來函可以撤銷：%+v", o.Availability)
	}
	revoke := func(adminID int64, version int64, reason string) (RevokeAuthorityOrderResult, error) {
		return f.module.RevokeAuthorityOrder(ctx, RevokeAuthorityOrderInput{ID: o.ID, AdminID: adminID, ExpectedVersion: version, Reason: reason})
	}
	_, err := revoke(authorityRegistrar, o.Version, "  ")
	wantInvalid(t, err, "Reason", "撤銷的原因必須是 1 到 2000 字。")
	_, err = revoke(authorityRegistrar, o.Version+1, "帳號登記錯了")
	wantKind(t, err, apperr.KindVersionConflict)

	f.now = f.now.Add(time.Hour)
	r, err := revoke(authorityRegistrar, o.Version, "  帳號登記錯了  ")
	must(t, err)
	got := r.Order
	if r.AlreadyRevoked || got.State() != AuthorityStateRevoked || got.Open() || got.HoldActive || got.Revoked == nil ||
		got.Revoked.Reason != "帳號登記錯了" || got.Revoked.AdminID != authorityRegistrar || !got.Revoked.At.Equal(f.now) || got.Version != o.Version+1 {
		t.Fatalf("撤銷之後：%+v revoked=%+v", got.AuthorityOrder, got.Revoked)
	}
	for name, blocks := range map[string][]AuthorityOrderBlock{
		"完成": got.Availability.Complete, "延長": got.Availability.ExtendHold, "撤銷": got.Availability.Revoke, "記下告知": got.Availability.RecordNotice[0],
	} {
		if len(blocks) == 0 || blocks[0] != AuthorityBlockRevoked {
			t.Errorf("%s 要被撤銷擋下：%v", name, blocks)
		}
	}
	if st := got.Targets[0].Stores; len(st) != 1 || st[0].CanCreateMeasure || st[0].CanCreateTermination {
		t.Fatalf("撤銷的來函不能建立措施：%+v", st)
	}
	again, err := revoke(authorityRegistrar, o.Version, "另一個原因")
	must(t, err)
	if !again.AlreadyRevoked || again.Order.Version != got.Version || again.Order.Revoked.Reason != "帳號登記錯了" {
		t.Fatalf("同一位管理員重送回原本的紀錄：%+v", again)
	}
	_, err = revoke(authorityCompleter, got.Version, "x")
	wantBlocked(t, err, "這筆來函已撤銷登記。")
	// 第二位管理員拿的是撤銷之前讀到的 etag（實際的並行情形）：回 failed_precondition，不是版本衝突（先問能不能撤銷、再比 etag）。
	_, err = revoke(authorityCompleter, o.Version, "x")
	wantBlocked(t, err, "這筆來函已撤銷登記。")
	// 撤銷之後逐家的涵蓋狀態：保存期限還沒到，不能寫「保存期限已過」。
	for _, st := range got.Targets[0].Stores {
		if st.Coverage != CoverageHoldReleasedByRevocation || !got.HoldUntil.After(f.now) {
			t.Fatalf("撤銷之後的涵蓋狀態：%+v hold_until=%s now=%s", st, got.HoldUntil, f.now)
		}
	}
	_, err = f.completeOrder(got, nil, f.notifyAll(got))
	wantBlocked(t, err, "這筆來函已撤銷登記。")
	_, err = f.module.ExtendAuthorityOrderHold(ctx, ExtendAuthorityOrderHoldInput{ID: o.ID, AdminID: authorityRegistrar, ExpectedVersion: got.Version, HoldUntil: got.HoldUntil.Add(time.Hour), Reason: "訴訟"})
	wantBlocked(t, err, "這筆來函已撤銷登記。")

	// 不能再當作措施的來源。
	_, err = f.module.CreateMeasure(ctx, f.riskFromOrder(s.StoreID, o.ID))
	wantInvalid(t, err, "SourceAuthorityOrderID", "這筆來函已撤銷登記，不能當作措施的來源。")

	// 清單：處理中不含它，REVOKED 只有它。
	open, revokedState := AuthorityStateOpen, AuthorityStateRevoked
	page, err := f.module.AuthorityOrders(ctx, AuthorityOrderListInput{State: &open})
	must(t, err)
	if len(page.Items) != 0 {
		t.Fatalf("處理中不含撤銷的來函：%d", len(page.Items))
	}
	page, err = f.module.AuthorityOrders(ctx, AuthorityOrderListInput{State: &revokedState})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != o.ID {
		t.Fatalf("REVOKED 清單：%+v", page.Items)
	}

	// 已完成的來函不能撤銷（完成之後的錯誤寫在處理摘要）。
	other := f.holdOrder(t, accountID)
	done, err := f.completeOrder(other, nil, f.notifyAll(other))
	must(t, err)
	if len(done.Availability.Revoke) == 0 || done.Availability.Revoke[0] != AuthorityBlockCompleted {
		t.Fatalf("已完成的來函不能撤銷：%v", done.Availability.Revoke)
	}
	_, err = f.module.RevokeAuthorityOrder(ctx, RevokeAuthorityOrderInput{ID: other.ID, AdminID: authorityRegistrar, ExpectedVersion: done.Version, Reason: "x"})
	wantBlocked(t, err, "這筆來函已完成。")
}

// 記下告知與建立措施入口的可用性由伺服器推導：完成之前沒有告知（不是暫不告知）、暫不告知的對象可以記、已告知的不行；
// 對象範圍內每家店的建立措施：開著的店兩者都可以，已關閉的店不能建立一般措施，已關閉而且是帳號最近的 stint 仍可建立終止合作（21.3），
// 已關閉而帳號有較新店家的都不行。與 RecordAuthorityTargetNotice、CreateMeasure 接受什麼一致。
// mutation：RecordNotice 的原因不看 notice.Kind → 已告知的對象 can_record_notice 為真，斷言失敗；CanCreateMeasureOn 不看 Closed → 關閉的店可以建立，斷言失敗。
func TestTargetNoticeAndMeasureEntryAvailabilityAreServerDerived(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	openAccount, openStore := f.open(t)
	closedAccount, closedStore := f.closedStore(t)
	supersededAccount, superseded := f.closedStore(t)
	newer, err := f.module.Apply(ctx, validApply(supersededAccount))
	must(t, err)
	o := f.createOrder(t, f.orderIn(AuthoritySuspendAccount,
		AuthorityTargetInput{AccountID: openAccount}, AuthorityTargetInput{AccountID: closedAccount}, AuthorityTargetInput{AccountID: supersededAccount}))
	for i, blocks := range o.Availability.RecordNotice {
		if len(blocks) == 0 || blocks[0] != AuthorityBlockNoticeNotWithheld {
			t.Fatalf("完成之前對象 %d 不能記下告知：%v", i, blocks)
		}
	}
	type flags struct{ measure, termination bool }
	want := map[int64]flags{openStore.StoreID: {true, true}, closedStore.StoreID: {false, true}, superseded.StoreID: {false, false}, newer.StoreID: {true, true}}
	for _, tgt := range o.Targets {
		for _, st := range tgt.Stores {
			if w := want[st.StoreID]; st.CanCreateMeasure != w.measure || st.CanCreateTermination != w.termination {
				t.Errorf("店家 %d：measure=%v termination=%v，期望 %+v", st.StoreID, st.CanCreateMeasure, st.CanCreateTermination, w)
			}
		}
	}
	// 與 CreateMeasure 一致：關閉的店建立停權被拒。
	_, err = f.module.CreateMeasure(ctx, f.riskFromOrder(closedStore.StoreID, o.ID))
	if err == nil {
		t.Fatal("關閉的店不能建立停權")
	}
	recheck := f.now.Add(72 * time.Hour)
	at := f.now
	done, err := f.completeOrder(o, nil, []TargetNoticeInput{
		{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck}},
		{Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at}},
		{Position: 2, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at}},
	})
	must(t, err)
	if len(done.Availability.RecordNotice[0]) != 0 || len(done.Availability.RecordNotice[1]) == 0 {
		t.Fatalf("暫不告知的可以記、已告知的不行：%v", done.Availability.RecordNotice)
	}
	_, err = f.module.RecordAuthorityTargetNotice(ctx, RecordAuthorityTargetNoticeInput{
		ID: o.ID, AdminID: authorityCompleter, ExpectedVersion: done.Version, Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at},
	})
	wantBlocked(t, err, "這個對象不是暫不告知的狀態，不能再記下告知。")
	_, err = f.module.RecordAuthorityTargetNotice(ctx, RecordAuthorityTargetNoticeInput{
		ID: o.ID, AdminID: authorityCompleter, ExpectedVersion: done.Version, Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &at},
	})
	must(t, err)
}

// riskFromOrder 是從來函建立的風險停權（RISK）的輸入。
func (f *fixture) riskFromOrder(storeID int64, order uuid.UUID) CreateMeasureInput {
	review := f.now.Add(7 * 24 * time.Hour)
	return CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureSuspension, StatementToSeller: "依主管機關來函暫停", Note: "來函導出", ReviewBy: &review,
		Risk: &RiskBasis{Ground: RiskFraud}, SourceAuthorityOrderID: &order,
	}
}

// 兩位管理員同時用撤銷之前的 etag 撤銷同一份來函：來函列鎖把兩者串行，恰好一個成功，另一個看到已撤銷而回 failed_precondition（不是版本衝突）。
// mutation：RevokeAuthorityOrder 先比 etag 再問可用性 → 後到的得到版本衝突，斷言失敗。
func TestTwoAdminsRevokingConcurrentlyGetOneSuccessAndOneFailedPrecondition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	o := f.holdOrder(t, accountID)
	admins := []int64{authorityRegistrar, authorityCompleter}
	errs := make([]error, len(admins))
	var wg sync.WaitGroup
	for i, admin := range admins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.module.RevokeAuthorityOrder(ctx, RevokeAuthorityOrderInput{ID: o.ID, AdminID: admin, ExpectedVersion: o.Version, Reason: "登記錯了"})
		}()
	}
	wg.Wait()
	var ok, blocked int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.KindOf(err) == apperr.KindFailedPrecondition:
			blocked++
		default:
			t.Errorf("非預期的錯誤：%v", err)
		}
	}
	if ok != 1 || blocked != 1 {
		t.Fatalf("成功 %d、failed_precondition %d：%v", ok, blocked, errs)
	}
}
