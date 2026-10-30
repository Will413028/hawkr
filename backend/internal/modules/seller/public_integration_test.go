//go:build integration

package seller_test

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
)

// reconcile 跑一次可曝光的對帳，回傳掃描的店家數。
func (f *fixture) reconcile(t *testing.T) int {
	t.Helper()
	n, err := f.module.ReconcileExposure(context.Background())
	must(t, err)
	return n
}

func (f *fixture) firstExposedAt(t *testing.T, storeID int64) *time.Time {
	t.Helper()
	var at *time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT first_exposed_at FROM seller.stores WHERE store_id=$1", storeID).Scan(&at))
	return at
}

func (f *fixture) cardIDs(t *testing.T, ids ...int64) []int64 {
	t.Helper()
	cards, err := f.module.StoreCards(context.Background(), ids)
	must(t, err)
	out := make([]int64, len(cards))
	for i, c := range cards {
		out[i] = c.StoreID
	}
	return out
}

func (f *fixture) card(t *testing.T, id int64) StoreCard {
	t.Helper()
	cards, err := f.module.StoreCards(context.Background(), []int64{id})
	must(t, err)
	if len(cards) != 1 {
		t.Fatalf("store %d: cards=%+v", id, cards)
	}
	return cards[0]
}

func TestPublicStoreByCodeReturnsAnExposableStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.exposable(t)
	got, err := f.module.PublicStoreByCode(context.Background(), s.StoreCode)
	must(t, err)
	if got.StoreID != s.StoreID || got.StoreName != s.StoreName || got.LogoKey == nil {
		t.Fatalf("got=%+v", got)
	}
}

// 每一個不可曝光的條件都讓店面頁讀不到；不存在與不可曝光是同一個錯誤，不洩漏哪些店存在。
func TestPublicStoreByCodeHidesEveryStoreThatIsNotExposable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := map[string]func(t *testing.T, f *fixture) Store{
		"closed": func(t *testing.T, f *fixture) Store {
			accountID, s := f.exposable(t)
			_, err := f.module.CloseCurrent(ctx, accountID)
			must(t, err)
			return s
		},
		"pending deletion": func(t *testing.T, f *fixture) Store {
			_, s := f.exposable(t)
			must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
			return s
		},
		"admin suspension": func(t *testing.T, f *fixture) Store {
			_, s := f.exposable(t)
			f.suspend(t, s.StoreID)
			return s
		},
		"governance suspension": func(t *testing.T, f *fixture) Store {
			_, s := f.exposable(t)
			ends := f.now.Add(30 * 24 * time.Hour)
			f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
			return s
		},
		"kyc not approved": func(t *testing.T, f *fixture) Store {
			accountID, s, _ := f.submitted(t, idA, bankAcct)
			f.logoed(t, accountID, s)
			return s
		},
		"no kyc application": func(t *testing.T, f *fixture) Store {
			accountID, s := f.open(t)
			f.logoed(t, accountID, s)
			return s
		},
		"payuni not approved": func(t *testing.T, f *fixture) Store {
			accountID, s := f.boundStore(t)
			f.logoed(t, accountID, s)
			return s
		},
		"missing logo": func(t *testing.T, f *fixture) Store {
			_, s := f.boundStore(t)
			f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
			must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
			return s
		},
	}
	for name, setupCase := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			hidden := setupCase(t, f)
			_, err := f.module.PublicStoreByCode(ctx, hidden.StoreCode)
			if apperr.KindOf(err) != apperr.KindNotFound {
				t.Fatalf("PublicStoreByCode(%d) err=%v, want NotFound", hidden.StoreCode, err)
			}
			_, unknown := f.module.PublicStoreByCode(ctx, 999999999)
			if err.Error() != unknown.Error() {
				t.Fatalf("不公開（%q）與不存在（%q）的錯誤不同", err, unknown)
			}
		})
	}
}

// store_id 是連號、可以逐號列舉，不是店面頁的鍵：拿一家公開店的 store_id 當 store_code 讀不到它。
func TestPublicStoreByCodeDoesNotAcceptTheStoreID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.exposable(t)
	if s.StoreID == s.StoreCode {
		t.Fatal("測試需要 store_id 與 store_code 不同值")
	}
	if _, err := f.module.PublicStoreByCode(context.Background(), s.StoreID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("以 store_id 讀取 err=%v, want NotFound", err)
	}
	for _, bad := range []int64{0, -1} {
		if _, err := f.module.PublicStoreByCode(context.Background(), bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("PublicStoreByCode(%d) err=%v, want InvalidInput", bad, err)
		}
	}
}

// 追蹤等需要完整店家資料的地方與探索共用曝光規則：PublicStores 與 ExposableStores 在每種情況下挑出同一批店家。
func TestPublicStoresSelectTheSameStoresAsExposableStores(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, a := f.exposable(t)
	accountID, closed := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	_, b := f.exposable(t)
	_, unfinished := f.open(t)
	ids := []int64{b.StoreID, closed.StoreID, 999999, unfinished.StoreID, a.StoreID, b.StoreID}
	public, err := f.module.PublicStores(ctx, ids)
	must(t, err)
	got := make([]int64, len(public))
	for i, s := range public {
		got[i] = s.StoreID
	}
	if want := f.exposed(t, ids...); !slices.Equal(got, want) || !slices.Equal(got, []int64{b.StoreID, a.StoreID}) {
		t.Fatalf("PublicStores=%v ExposableStores=%v", got, want)
	}
	if public[0].AccountID == 0 {
		t.Fatalf("PublicStores 要帶擁有者帳號（追蹤檢查自己的店用）：%+v", public[0])
	}
}

func TestStoreCardsReturnEveryEverExposedStoreWithItsStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, open := f.exposable(t)
	_, suspended := f.exposable(t)
	_, deleting := f.exposable(t)
	closer, closed := f.exposable(t)
	_, rejected := f.exposable(t)
	f.suspend(t, suspended.StoreID)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, deleting.StoreID) }))
	_, err := f.module.CloseCurrent(ctx, closer)
	must(t, err)
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, rejected.StoreID, PayUniSourceRefresh))
	for name, c := range map[string]struct {
		store  Store
		status CardStatus
	}{
		"open": {open, CardOpen}, "suspended": {suspended, CardPaused}, "pending deletion": {deleting, CardPaused},
		"closed": {closed, CardClosed}, "payuni rejected later": {rejected, CardPaused},
	} {
		got := f.card(t, c.store.StoreID)
		if got.TownCode == nil || c.store.TownCode == nil || *got.TownCode != *c.store.TownCode {
			t.Errorf("%s: card.TownCode=%v, want the store's %v", name, got.TownCode, c.store.TownCode)
		}
		if got.Status != c.status || got.StoreCode != c.store.StoreCode || got.Name == nil || *got.Name != c.store.StoreName {
			t.Errorf("%s: card=%+v, want status %s with the store's name and code", name, got, c.status)
		}
	}
	if got := f.card(t, open.StoreID); got.LogoKey == nil {
		t.Errorf("營業中的店有 logo：%+v", got)
	}
	if got := f.card(t, closed.StoreID); got.LogoKey != nil || got.CoverKey != nil {
		t.Errorf("關閉的店沒有目前的 logo 與封面：%+v", got)
	}
}

// 個資清除之後（店家關閉滿保留期）卡片仍然存在，但所在地與店名都是 nil（DB 是 NULL），不是殘留的舊值。
func TestStoreCardsHaveNoTownOrNameAfterThePersonalDataIsPurged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	closer, s := f.exposable(t)
	if before := f.card(t, s.StoreID); before.TownCode == nil || before.Name == nil {
		t.Fatalf("清除之前卡片有所在地與店名：%+v", before)
	}
	_, err := f.module.CloseCurrent(ctx, closer)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	got := f.card(t, s.StoreID)
	if got.TownCode != nil || got.Name != nil || got.Status != CardClosed {
		t.Fatalf("清除之後的卡片：%+v", got)
	}
}

// 空窗：對帳還沒追上時（時間造成的變化），此刻可曝光的店照樣有卡片（OPEN）；曾經公開但時間還沒被記下就被停權的店沒有。
// 時間欄位有值的店，之後不可曝光也有卡片。寫入路徑一律當場記下時間，所以這裡直接清掉時間來模擬那個空窗。
func TestStoreCardsCoverTheStampLag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, exposable := f.exposable(t)
	_, stamped := f.exposable(t)
	_, lagging := f.exposable(t)
	_, err := f.db.Conn(context.Background()).Exec(context.Background(), "UPDATE seller.stores SET first_exposed_at = NULL WHERE store_id = $1", lagging.StoreID)
	must(t, err)
	if f.firstExposedAt(t, lagging.StoreID) != nil {
		t.Fatal("測試需要一間還沒被記下的店")
	}
	if got := f.card(t, lagging.StoreID); got.Status != CardOpen {
		t.Fatalf("job 還沒追上的可曝光店 card=%+v, want OPEN", got)
	}
	f.suspend(t, lagging.StoreID)
	if got := f.cardIDs(t, lagging.StoreID); len(got) != 0 {
		t.Fatalf("沒被記下的店被停權之後不該有卡片：%v", got)
	}
	f.suspend(t, stamped.StoreID)
	if got := f.card(t, stamped.StoreID); got.Status != CardPaused {
		t.Fatalf("已記下時間的店被停權 card=%+v, want PAUSED", got)
	}
	// 時間欄位是唯一的依據：直接寫入也一樣（不經過 job）。
	_, direct := f.open(t)
	_, err = f.db.Conn(context.Background()).Exec(context.Background(), "UPDATE seller.stores SET first_exposed_at = $2 WHERE store_id = $1", direct.StoreID, f.now)
	must(t, err)
	if got := f.card(t, direct.StoreID); got.Status != CardPaused {
		t.Fatalf("有時間但不可曝光的店 card=%+v, want PAUSED", got)
	}
	_ = exposable
}

// first_exposed_at 只在公告為可曝光時記下（當場，不等對帳），已有的時間不覆寫；不可曝光的店沒有時間。
func TestFirstExposedAtIsStampedWhenTheStoreBecomesExposableAndNeverOverwritten(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, first := f.exposable(t)
	_, unfinished := f.open(t)
	accountID, pending := f.boundStore(t)
	firstAt := f.now
	f.reconcile(t)
	if at := f.firstExposedAt(t, first.StoreID); at == nil || !at.Equal(firstAt) {
		t.Fatalf("first=%v, want %v", at, firstAt)
	}
	for name, id := range map[string]int64{"unfinished": unfinished.StoreID, "payuni pending": pending.StoreID} {
		if f.firstExposedAt(t, id) != nil {
			t.Errorf("%s 不可曝光，不該有時間", name)
		}
	}
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, pending.StoreID, PayUniSourcePoll))
	if at := f.firstExposedAt(t, pending.StoreID); at != nil {
		t.Fatalf("PayUni 核准但還沒有 logo，不該有時間：%v", at)
	}
	f.logoed(t, accountID, pending)
	secondAt := f.now
	if at := f.firstExposedAt(t, pending.StoreID); at == nil || !at.Equal(secondAt) {
		t.Fatalf("pending=%v, want %v", at, secondAt)
	}
	f.now = f.now.Add(time.Hour)
	f.reconcile(t)
	if at := f.firstExposedAt(t, first.StoreID); at == nil || !at.Equal(firstAt) {
		t.Fatalf("已有的時間被覆寫：%v, want %v", at, firstAt)
	}
}

// store_id 連號可列舉：從沒公開過的店（還沒申請、審核中、被退回、管理員核准但 PayUni 還沒核准）不回卡片；不存在的略過，重複的只回一次，依輸入順序。
func TestStoreCardsOmitStoresThatWereNeverExposed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, approved := f.exposable(t)
	_, noApplication := f.open(t)
	_, inReview, _ := f.submitted(t, idA, bankAcct)
	_, rejected, sub := f.submitted(t, idA, bankAcct)
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "證件模糊"})
	must(t, err)
	_, approvedPending := f.boundStore(t)
	f.reconcile(t)
	ids := []int64{noApplication.StoreID, inReview.StoreID, rejected.StoreID, approvedPending.StoreID, approved.StoreID}
	if got := f.cardIDs(t, ids...); !slices.Equal(got, []int64{approved.StoreID}) {
		t.Fatalf("cards=%v, want only the exposed store %d", got, approved.StoreID)
	}
	_, other := f.exposable(t)
	if got := f.cardIDs(t, other.StoreID, 999999, approved.StoreID, other.StoreID); !slices.Equal(got, []int64{other.StoreID, approved.StoreID}) {
		t.Fatalf("cards=%v, want [%d %d]", got, other.StoreID, approved.StoreID)
	}
}

// 個資清除之後卡片沒有店名（不是空字串，也不是墓碑文字：「已關閉的店家」由 client 顯示），狀態仍是已關閉。
func TestStoreCardHasNoNameAfterThePIIPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.exposable(t)
	f.reconcile(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if got := f.card(t, s.StoreID); got.Name == nil {
		t.Fatalf("清除之前應該有店名：%+v", got)
	}
	f.now = f.now.Add(StoreRetention + time.Second)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.card(t, s.StoreID); got.Name != nil || got.Status != CardClosed {
		t.Fatalf("card=%+v, want closed without a name", got)
	}
}

func TestStoreCardsValidateTheBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if got, err := f.module.StoreCards(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty batch: %v %v", got, err)
	}
	for name, ids := range map[string][]int64{"non-positive": {1, 0}, "too many": make([]int64, MaxBatchSize+1)} {
		for i := range ids {
			if name == "too many" {
				ids[i] = int64(i + 1)
			}
		}
		if _, err := f.module.StoreCards(ctx, ids); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s: err=%v, want InvalidInput", name, err)
		}
	}
}

// OPEN 就是此刻可以公開曝光（同一個規則）：曾經公開、之後 PayUni 被拒絕的店是 PAUSED，不是 OPEN；OPEN 的卡片一定讀得到店面、
// 也可以被追蹤（PublicStores），PAUSED 的都不行。
func TestAnOpenCardIsExactlyAnExposableStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, open := f.exposable(t)
	_, rejected := f.exposable(t)
	f.reconcile(t)
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, rejected.StoreID, PayUniSourceRefresh))
	if got := f.card(t, rejected.StoreID); got.Status != CardPaused || got.Name == nil {
		t.Fatalf("PayUni 被拒絕的店 card=%+v, want PAUSED", got)
	}
	_, unfinished := f.boundStore(t)
	for _, c := range []struct {
		store  Store
		status CardStatus
	}{{open, CardOpen}, {rejected, CardPaused}} {
		card := f.card(t, c.store.StoreID)
		_, byCodeErr := f.module.PublicStoreByCode(ctx, c.store.StoreCode)
		public, err := f.module.PublicStores(ctx, []int64{c.store.StoreID})
		must(t, err)
		if card.Status != c.status || (card.Status == CardOpen) != (byCodeErr == nil) || (card.Status == CardOpen) != (len(public) == 1) {
			t.Errorf("store %d: card=%s byCodeErr=%v followable=%d, want status %s", c.store.StoreID, card.Status, byCodeErr, len(public), c.status)
		}
	}
	if got := f.cardIDs(t, unfinished.StoreID); len(got) != 0 {
		t.Errorf("從沒公開過的店沒有卡片：%v", got)
	}
}

func (f *fixture) listPublic(t *testing.T, size int32, after *PublicStoreCursor) PublicStorePage {
	t.Helper()
	page, err := f.module.ListPublicStores(context.Background(), ListPublicStoresInput{PageSize: size, After: after})
	must(t, err)
	return page
}

func (f *fixture) listAllPublic(t *testing.T, size int32) []int64 {
	t.Helper()
	var ids []int64
	var after *PublicStoreCursor
	for range 100 {
		page := f.listPublic(t, size, after)
		if limit := cmp.Or(size, 100); int32(len(page.Items)) > limit {
			t.Fatalf("page of %d items exceeds page size %d", len(page.Items), limit)
		}
		for _, s := range page.Items {
			ids = append(ids, s.StoreID)
		}
		if page.Next == nil {
			return ids
		}
		if len(page.Items) > 0 && page.Next.StoreID < page.Items[len(page.Items)-1].StoreID {
			t.Fatalf("next cursor %+v is before the last item of %+v", page.Next, page.Items)
		}
		if after != nil && page.Next.StoreID <= after.StoreID {
			t.Fatalf("next cursor %+v does not advance past %+v", page.Next, after)
		}
		after = page.Next
	}
	t.Fatal("paging did not terminate")
	return nil
}

// hiddenWorld 建立三家可曝光的店，夾著每一種不可曝光的店（id 交錯），回傳可曝光的與不可曝光的 store_id。
func (f *fixture) hiddenWorld(t *testing.T) (visible, hidden []int64) {
	t.Helper()
	ctx := context.Background()
	_, a := f.exposable(t)
	closer, closed := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, closer)
	must(t, err)
	_, deleting := f.exposable(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, deleting.StoreID) }))
	_, b := f.exposable(t)
	_, suspended := f.exposable(t)
	_, c := f.exposable(t)
	f.suspend(t, suspended.StoreID)
	accountID, notApproved, _ := f.submitted(t, idB, "012345678902")
	f.logoed(t, accountID, notApproved)
	_, unfinished := f.open(t)
	return []int64{a.StoreID, b.StoreID, c.StoreID}, []int64{closed.StoreID, deleting.StoreID, suspended.StoreID, notApproved.StoreID, unfinished.StoreID}
}

func TestListPublicStoresReturnsOnlyExposableStoresInIDOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	visible, hidden := f.hiddenWorld(t)
	for _, size := range []int32{0, 1, 2, 3, 100} {
		got := f.listAllPublic(t, size)
		if !slices.Equal(got, visible) {
			t.Fatalf("page_size %d：ListPublicStores=%v, want %v（不可曝光的 %v 不得出現）", size, got, visible, hidden)
		}
	}
	all := slices.Concat(visible, hidden)
	if want := f.exposed(t, all...); !slices.Equal(want, visible) {
		t.Fatalf("ExposableStores=%v, want %v", want, visible)
	}
}

func TestListPublicStoresPagesAreFullWhenHiddenStoresSitBetweenVisibleOnes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	visible, _ := f.hiddenWorld(t)
	first := f.listPublic(t, 2, nil)
	if len(first.Items) != 2 || first.Next == nil || first.Items[0].StoreID != visible[0] || first.Items[1].StoreID != visible[1] {
		t.Fatalf("first page=%+v, want %v", first, visible[:2])
	}
	second := f.listPublic(t, 2, first.Next)
	if len(second.Items) != 1 || second.Items[0].StoreID != visible[2] || second.Next != nil {
		t.Fatalf("second page=%+v", second)
	}
	exact := f.listPublic(t, 3, nil)
	if len(exact.Items) != 3 || exact.Next != nil {
		t.Fatalf("page_size 等於可曝光的店數時沒有下一頁：%+v", exact)
	}
}

func TestListPublicStoresReadsOnlyStoresWithAnAnnouncedExposure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, a := f.exposable(t)
	must(t, func() error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO identity.accounts (email) SELECT 'filler-' || g || '@example.test' FROM generate_series(1, 250) g`)
		return err
	}())
	_, err := f.db.Conn(ctx).Exec(ctx, `
		INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at)
		SELECT 3000000000 + row_number() OVER (ORDER BY id), id, 'filler', $1, $1 FROM identity.accounts WHERE email LIKE 'filler-%'`, f.now)
	must(t, err)
	_, b := f.exposable(t)
	first := f.listPublic(t, 1, nil)
	if len(first.Items) != 1 || first.Items[0].StoreID != a.StoreID || first.Next == nil {
		t.Fatalf("first=%+v", first)
	}
	second := f.listPublic(t, 1, first.Next)
	if len(second.Items) != 1 || second.Items[0].StoreID != b.StoreID || second.Next != nil {
		t.Fatalf("沒有公告列的店不是候選，不影響頁數：%+v", second)
	}
}

func TestListPublicStoresDropsAStoreThatStoppedBeingExposableSinceTheLastAnnouncement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, a := f.exposable(t)
	_, b := f.exposable(t)
	_, c := f.exposable(t)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET status='PENDING_DELETION' WHERE store_id=$1", b.StoreID)
	must(t, err)
	if exposable, _ := f.announced(t, b.StoreID); !exposable {
		t.Fatal("測試前提：b 最後公告的值仍是可曝光")
	}
	first := f.listPublic(t, 2, nil)
	if len(first.Items) != 1 || first.Items[0].StoreID != a.StoreID || first.Next == nil || first.Next.StoreID != b.StoreID {
		t.Fatalf("一頁可以少於 page_size，cursor 是最後一個候選（含被濾掉的）：%+v", first)
	}
	second := f.listPublic(t, 2, first.Next)
	if len(second.Items) != 1 || second.Items[0].StoreID != c.StoreID || second.Next != nil {
		t.Fatalf("second=%+v", second)
	}
	alone := f.listPublic(t, 1, &PublicStoreCursor{StoreID: a.StoreID})
	if len(alone.Items) != 0 || alone.Next == nil || alone.Next.StoreID != b.StoreID {
		t.Fatalf("整頁都被濾掉時仍要給 next：%+v", alone)
	}
	if got := f.listAllPublic(t, 1); !slices.Equal(got, []int64{a.StoreID, c.StoreID}) {
		t.Fatalf("listAll=%v", got)
	}
}

func TestListPublicStoresShowsAStoreAgainOnceItsExposureIsAnnounced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, a := f.exposable(t)
	_, b := f.exposable(t)
	m := f.suspend(t, b.StoreID)
	f.reconcile(t)
	if got := f.listAllPublic(t, 10); !slices.Equal(got, []int64{a.StoreID}) {
		t.Fatalf("停權的店不列出：%v", got)
	}
	before, err := f.module.PublicStoreByCode(ctx, a.StoreCode)
	must(t, err)
	f.now = f.now.Add(time.Hour)
	_, err = f.lift(ctx, m)
	must(t, err)
	f.reconcile(t)
	var listed *PublicStore
	for _, s := range f.listPublic(t, 10, nil).Items {
		if s.StoreID == b.StoreID {
			listed = &s
		}
	}
	if listed == nil {
		t.Fatal("恢復並公告之後要重新列出")
	}
	byCode, err := f.module.PublicStoreByCode(ctx, b.StoreCode)
	must(t, err)
	if !listed.UpdateTime.Equal(byCode.UpdateTime) || !listed.UpdateTime.Equal(f.now) {
		t.Fatalf("重新曝光時 update_time 要前進到公告時間：list=%v byCode=%v want=%v", listed.UpdateTime, byCode.UpdateTime, f.now)
	}
	if !before.UpdateTime.Before(f.now) {
		t.Fatalf("其他店的 update_time 不動：%v", before.UpdateTime)
	}
}

func TestReannounceDoesNotMoveUpdateTimeButAFlipDoes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, a := f.exposable(t)
	before, err := f.module.PublicStoreByCode(ctx, a.StoreCode)
	must(t, err)
	f.now = f.now.Add(time.Hour)
	n, err := f.module.ReannounceExposures(ctx)
	must(t, err)
	if n == 0 {
		t.Fatal("全量重新公告沒有公告任何店")
	}
	after, err := f.module.PublicStoreByCode(ctx, a.StoreCode)
	must(t, err)
	if !after.UpdateTime.Equal(before.UpdateTime) {
		t.Fatalf("重發同一個公告值不該動 update_time：%v -> %v", before.UpdateTime, after.UpdateTime)
	}
	m := f.suspend(t, a.StoreID)
	f.reconcile(t)
	f.now = f.now.Add(time.Hour)
	_, err = f.lift(ctx, m)
	must(t, err)
	f.reconcile(t)
	flipped, err := f.module.PublicStoreByCode(ctx, a.StoreCode)
	must(t, err)
	if !flipped.UpdateTime.Equal(f.now) {
		t.Fatalf("公告值翻轉要把 update_time 前進到 %v，got %v", f.now, flipped.UpdateTime)
	}
}

func TestListPublicStoresFollowsExposureChangesBetweenPagesWithoutRepeating(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, a := f.exposable(t)
	_, b := f.exposable(t)
	_, c := f.exposable(t)
	first := f.listPublic(t, 1, nil)
	if len(first.Items) != 1 || first.Items[0].StoreID != a.StoreID {
		t.Fatalf("first=%+v", first)
	}
	_, d := f.exposable(t)
	f.suspend(t, b.StoreID)
	var got []int64
	for after := first.Next; ; {
		page := f.listPublic(t, 1, after)
		for _, s := range page.Items {
			got = append(got, s.StoreID)
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if want := []int64{c.StoreID, d.StoreID}; !slices.Equal(got, want) {
		t.Fatalf("got=%v, want %v", got, want)
	}
}

func TestListPublicStoresWithNoExposableStoreIsEmpty(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.open(t)
	page := f.listPublic(t, 10, nil)
	if len(page.Items) != 0 || page.Next != nil {
		t.Fatalf("page=%+v", page)
	}
}

func TestListPublicStoresValidatesPaging(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.exposable(t)
	for name, in := range map[string]ListPublicStoresInput{
		"negative page size": {PageSize: -1},
		"zero cursor":        {After: &PublicStoreCursor{}},
		"negative cursor":    {After: &PublicStoreCursor{StoreID: -3}},
	} {
		if _, err := f.module.ListPublicStores(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v, want InvalidInput", name, err)
		}
	}
	page, err := f.module.ListPublicStores(ctx, ListPublicStoresInput{PageSize: 5000})
	must(t, err)
	if len(page.Items) != 1 {
		t.Fatalf("超過上限以上限計：%+v", page)
	}
}

func TestPublicStoreCarriesTheLatestUpdateTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, plain := f.exposable(t)
	var updatedAt time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT updated_at FROM seller.stores WHERE store_id=$1", plain.StoreID).Scan(&updatedAt))
	for name, c := range map[string]struct {
		firstExposedAt, announcedAt *time.Time
		want                        time.Time
	}{
		"first exposure later than the last edit": {ptr(updatedAt.Add(time.Hour)), ptr(updatedAt), updatedAt.Add(time.Hour)},
		"first exposure earlier":                  {ptr(updatedAt.Add(-time.Hour)), ptr(updatedAt.Add(-time.Hour)), updatedAt},
		"not recorded yet":                        {nil, ptr(updatedAt.Add(-time.Hour)), updatedAt},
		"announced after everything else":         {ptr(updatedAt.Add(time.Hour)), ptr(updatedAt.Add(2 * time.Hour)), updatedAt.Add(2 * time.Hour)},
	} {
		_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET first_exposed_at=$2 WHERE store_id=$1", plain.StoreID, c.firstExposedAt)
		must(t, err)
		_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable_changed_at=$2 WHERE store_id=$1", plain.StoreID, c.announcedAt)
		must(t, err)
		byCode, err := f.module.PublicStoreByCode(ctx, plain.StoreCode)
		must(t, err)
		page := f.listPublic(t, 1000, nil)
		var listed *PublicStore
		for i := range page.Items {
			if page.Items[i].StoreID == plain.StoreID {
				listed = &page.Items[i]
			}
		}
		if !byCode.UpdateTime.Equal(c.want) || listed == nil || !listed.UpdateTime.Equal(c.want) {
			t.Errorf("%s：GetStore %v、ListStores %v，want %v", name, byCode.UpdateTime, listed, c.want)
		}
	}
}

// StoreCards 與 StoresWithExposure（legacy 的 has_public_card）用同一條可見性規則：同一批店，有卡片就是 HasPublicCard，OPEN 就是 Exposable。
func TestStoreCardsAndStoresWithExposureAgreeOnTheSameStores(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, rejected := f.exposable(t)
	visible, hidden := f.hiddenWorld(t)
	f.reconcile(t)
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, rejected.StoreID, PayUniSourceRefresh))
	ids := slices.Concat(visible, hidden, []int64{rejected.StoreID})
	exposures, err := f.module.StoresWithExposure(ctx, ids)
	must(t, err)
	if len(exposures) != len(ids) {
		t.Fatalf("exposures=%d, want %d", len(exposures), len(ids))
	}
	cards, err := f.module.StoreCards(ctx, ids)
	must(t, err)
	byID := make(map[int64]StoreCard, len(cards))
	for _, c := range cards {
		byID[c.StoreID] = c
	}
	var withCard, withoutCard int
	for _, e := range exposures {
		card, hasCard := byID[e.Store.StoreID]
		if e.HasPublicCard != hasCard || e.Exposable != (hasCard && card.Status == CardOpen) {
			t.Errorf("store %d：HasPublicCard=%v Exposable=%v，card=%+v（有卡片 %v）", e.Store.StoreID, e.HasPublicCard, e.Exposable, card, hasCard)
		}
		if hasCard {
			withCard++
		} else {
			withoutCard++
		}
	}
	if withCard == 0 || withoutCard == 0 {
		t.Fatalf("測試要同時涵蓋有卡片（%d）與沒有卡片（%d）的店", withCard, withoutCard)
	}
}
