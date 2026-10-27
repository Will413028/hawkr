//go:build integration

package seller_test

import (
	"context"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type closureRow struct {
	by        *string
	measureID *string
}

func (f *fixture) closureOf(t *testing.T, storeID int64) closureRow {
	t.Helper()
	var r closureRow
	ctx := context.Background()
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT closed_by, closed_by_measure_id::text FROM seller.store_closures WHERE store_id = $1 AND reopened_at IS NULL", storeID).Scan(&r.by, &r.measureID))
	return r
}

// 每一條關店的路徑都記下為什麼關的：賣家自己、隨帳號刪除（終止合作見 termination_integration_test.go）。
func TestEveryClosePathRecordsWhoClosedTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	ownerAccount, owner := f.open(t)
	closed, err := f.module.CloseCurrent(ctx, ownerAccount)
	must(t, err)
	if closed.Closure == nil || closed.Closure.By != ClosedByOwner || closed.Closure.MeasureID != nil {
		t.Fatalf("owner closure=%+v", closed.Closure)
	}
	if row := f.closureOf(t, owner.StoreID); row.by == nil || *row.by != "ACCOUNT_OWNER" || row.measureID != nil {
		t.Fatalf("owner row=%+v", row)
	}

	_, deleted := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.CloseForDeletion(ctx, deleted.StoreID) }))
	if row := f.closureOf(t, deleted.StoreID); row.by == nil || *row.by != "ACCOUNT_DELETION" || row.measureID != nil {
		t.Fatalf("deletion row=%+v", row)
	}
	read, err := f.module.Store(ctx, deleted.StoreID)
	must(t, err)
	if read.Closure == nil || read.Closure.By != ClosedByDeletion {
		t.Fatalf("closure=%+v", read.Closure)
	}
}

// 個人資料清理（關店滿保留期）之後，關店的紀錄還在：它是稽核，不是申請人的欄位。
func TestTheClosureSurvivesThePIIPurge(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	read, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if read.StoreName != "" || read.Closure == nil || read.Closure.By != ClosedByOwner {
		t.Fatalf("store=%+v closure=%+v", read, read.Closure)
	}
}

// DB 保底：關店紀錄的欄位組合只有三種合法的形狀（終止合作一定指向造成關店的措施，其他沒有），一個 stint 同時最多一筆目前的紀錄。
func TestClosureSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	measureID := uuid.NewV7().String()
	const current = " WHERE store_id = $1 AND reopened_at IS NULL"
	for name, sql := range map[string]string{
		"an unknown closer":                    "UPDATE seller.store_closures SET closed_by = 'SOMEONE'" + current,
		"the removed admin closer":             "UPDATE seller.store_closures SET closed_by = 'ADMIN'" + current,
		"termination without a measure":        "UPDATE seller.store_closures SET closed_by = 'TERMINATION'" + current,
		"termination with a missing measure":   "UPDATE seller.store_closures SET closed_by = 'TERMINATION', closed_by_measure_id = '" + measureID + "'" + current,
		"owner with a measure":                 "UPDATE seller.store_closures SET closed_by_measure_id = '" + measureID + "'" + current,
		"a closure without any closer":         "UPDATE seller.store_closures SET closed_by = NULL" + current,
		"a deletion record with a measure":     "UPDATE seller.store_closures SET closed_by = 'ACCOUNT_DELETION', closed_by_measure_id = '" + measureID + "'" + current,
		"a reopening before the closing":       "UPDATE seller.store_closures SET reopened_at = closed_at - interval '1 second'" + current,
		"a second current closure":             "INSERT INTO seller.store_closures (store_id, closed_at, closed_by) SELECT store_id, closed_at + interval '1 second', 'ACCOUNT_OWNER' FROM seller.store_closures" + current,
		"a purged store that still has a logo": "UPDATE seller.stores SET pii_purged_at = now(), store_name = NULL, logo_key = 'x', logo_asset_id = 'x' WHERE store_id = $1",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, s.StoreID); err == nil {
			t.Fatalf("DB accepted %s: %s", name, sql)
		}
	}
}

// migration 對既有的已關閉 stint 回填 closed_by，再設「已關閉一定有紀錄」的 CHECK；沒有回填的話這支 migration 套不上去。
func TestClosureMigrationBackfillsStoresClosedBeforeTheRecordExisted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_store_closure.sql") })
	if at < 0 {
		t.Fatalf("找不到 store_closure 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, status, created_at, updated_at) VALUES (2600000001, 11, 'open', 'ACTIVE', now(), now())")
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, status, deleted_at, created_at, updated_at) VALUES (2600000002, 12, 'closed', 'CLOSED', now(), now(), now())")
	exec(body(files[at]))
	var closedBy *string
	must(t, d.Pool().QueryRow(ctx, "SELECT closed_by FROM seller.stores WHERE store_code = 2600000002").Scan(&closedBy))
	if closedBy == nil || *closedBy != "ACCOUNT_OWNER" {
		t.Fatalf("closed store closed_by=%v", closedBy)
	}
	must(t, d.Pool().QueryRow(ctx, "SELECT closed_by FROM seller.stores WHERE store_code = 2600000001").Scan(&closedBy))
	if closedBy != nil {
		t.Fatalf("open store closed_by=%v", closedBy)
	}
	if _, err := d.Pool().Exec(ctx, "UPDATE seller.stores SET closed_by = NULL WHERE store_code = 2600000002"); err == nil {
		t.Fatal("DB accepted a closed store without a closer")
	}
}

// openAt 在 at 這個時間開一家名為 name 的店。
func (f *fixture) openAt(t *testing.T, at time.Time, name string) Store {
	t.Helper()
	f.now = at
	in := validApply(f.account(t))
	in.StoreName = name
	s, err := f.module.Apply(context.Background(), in)
	must(t, err)
	return s
}

func storeIDs(items []StoreListItem) []int64 {
	out := make([]int64, len(items))
	for i, item := range items {
		out[i] = item.Store.StoreID
	}
	return out
}

// 翻頁不重不漏：同一個建立時間的店家以 store_id 決定順序，翻頁途中新開的店不會讓已經列過的店再出現或漏掉。
func TestAdminStoresKeysetPagesHaveNoGapsOrDuplicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	var created []Store
	for i, offset := range []int{0, 0, 1, 2, 2, 2, 3} {
		created = append(created, f.openAt(t, base.Add(time.Duration(offset)*time.Hour), "店"+string(rune('甲'+i))))
	}
	// 期望的順序：建立時間新到舊，同時間的 store_id 大到小。
	want := slices.Clone(created)
	slices.SortStableFunc(want, func(a, b Store) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return int(b.StoreID - a.StoreID)
	})
	wantIDs := make([]int64, len(want))
	for i, s := range want {
		wantIDs[i] = s.StoreID
	}

	for _, size := range []int32{1, 2, 3, 7, 100} {
		var got []int64
		var after *StoreCursor
		pages := 0
		for {
			page, err := f.module.AdminStores(ctx, StoreListInput{PageSize: size, After: after})
			must(t, err)
			pages++
			got = append(got, storeIDs(page.Items)...)
			if page.Next == nil {
				break
			}
			if len(page.Items) != int(size) {
				t.Fatalf("size %d: a page with a next cursor has %d items", size, len(page.Items))
			}
			after = page.Next
			if pages > 20 {
				t.Fatal("pagination does not terminate")
			}
		}
		if !slices.Equal(got, wantIDs) {
			t.Fatalf("size %d: got %v want %v", size, got, wantIDs)
		}
	}

	// 翻頁途中有新店：它比游標新，不在後面的頁；其餘的店一個不漏、一個不重。
	first, err := f.module.AdminStores(ctx, StoreListInput{PageSize: 3})
	must(t, err)
	late := f.openAt(t, base.Add(10*time.Hour), "店晚")
	got := storeIDs(first.Items)
	for after := first.Next; after != nil; {
		page, err := f.module.AdminStores(ctx, StoreListInput{PageSize: 3, After: after})
		must(t, err)
		got = append(got, storeIDs(page.Items)...)
		after = page.Next
	}
	if !slices.Equal(got, wantIDs) || slices.Contains(got, late.StoreID) {
		t.Fatalf("got %v want %v (late=%d)", got, wantIDs, late.StoreID)
	}
}

func TestAdminStoresFiltersAndDerivedFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.codes = []int64{2600000111, 2600000222, 2600000333, 2600000444}
	at := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	farm := f.openAt(t, at, "範例小農")
	other := f.openAt(t, at.Add(time.Hour), "海邊魚鋪")
	_, closing := f.open(t)
	phoneStore := f.openAt(t, at.Add(3*time.Hour), "電話測試店")
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET contact_phone = '0222334455' WHERE store_id = $1", phoneStore.StoreID); err != nil {
		t.Fatal(err)
	}
	closer, err := f.module.Store(ctx, closing.StoreID)
	must(t, err)
	_, err = f.module.CloseCurrent(ctx, closer.AccountID)
	must(t, err)
	f.suspend(t, farm.StoreID)

	list := func(in StoreListInput) []StoreListItem {
		t.Helper()
		page, err := f.module.AdminStores(ctx, in)
		must(t, err)
		return page.Items
	}
	idsOf := func(in StoreListInput) []int64 { return storeIDs(list(in)) }

	// 預設只列目前的店家，已關閉的要明列。
	if got := idsOf(StoreListInput{}); slices.Contains(got, closing.StoreID) || len(got) != 3 {
		t.Fatalf("default=%v", got)
	}
	if got := idsOf(StoreListInput{Statuses: []Status{Closed}}); !slices.Equal(got, []int64{closing.StoreID}) {
		t.Fatalf("closed=%v", got)
	}
	if got := idsOf(StoreListInput{Statuses: []Status{Closed, Active}}); len(got) != 4 {
		t.Fatalf("closed+active=%v", got)
	}
	if got := idsOf(StoreListInput{Statuses: []Status{PendingDeletion}}); len(got) != 0 {
		t.Fatalf("pending=%v", got)
	}
	// 店名、電話、store_code；% 與 _ 不是萬用字元；電話號碼不會被當成 store_code。
	if got := idsOf(StoreListInput{Query: "小農"}); !slices.Equal(got, []int64{farm.StoreID}) {
		t.Fatalf("name=%v", got)
	}
	if got := idsOf(StoreListInput{Query: "0222334"}); !slices.Equal(got, []int64{phoneStore.StoreID}) {
		t.Fatalf("phone=%v", got)
	}
	if got := idsOf(StoreListInput{Query: "2600000222"}); !slices.Equal(got, []int64{other.StoreID}) {
		t.Fatalf("store code=%v", got)
	}
	for _, q := range []string{"%", "_", "小%農", "26000002"} {
		if got := idsOf(StoreListInput{Query: q}); len(got) != 0 {
			t.Fatalf("query %q matched %v", q, got)
		}
	}
	// 停權與 KYC 流程是推導出來的，不是儲存的篩選。
	byID := map[int64]StoreListItem{}
	for _, item := range list(StoreListInput{}) {
		byID[item.Store.StoreID] = item
	}
	if !byID[farm.StoreID].Store.Suspended() || byID[other.StoreID].Store.Suspended() {
		t.Fatalf("suspension: %+v", byID)
	}
	for id, item := range byID {
		if item.KYCFlow != FlowNotStarted {
			t.Fatalf("store %d flow=%s", id, item.KYCFlow)
		}
	}
	// 不合法的篩選值被拒絕，不被忽略。
	if _, err := f.module.AdminStores(ctx, StoreListInput{Statuses: []Status{"WEIRD"}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("status: %v", err)
	}
	// 「待審核」不是儲存的事實，是待辦的成員（KYCAwaitingReview）：SUBMITTED 與草稿、不認得的值一樣被拒絕。
	for _, bad := range []KYCStatus{KYCStatusDraft, KYCStatusSubmitted, "WEIRD"} {
		if _, err := f.module.AdminStores(ctx, StoreListInput{KYCStatus: &bad}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("kyc %s: %v", bad, err)
		}
	}
	if _, err := f.module.AdminStores(ctx, StoreListInput{PageSize: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("page size: %v", err)
	}
}

func TestAdminStoresFilterByKYCStatusAndShowTheFlow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, noApplication := f.open(t)
	accountDraft, draftOnly := f.open(t)
	f.savedDraft(t, kycDraft(accountDraft, idA, bankAcct))
	_, submittedStore, _ := f.submitted(t, idB, "012345678902")
	_, approvedStore, toApprove := f.submitted(t, idC, "012345678903")
	must(t, approve(f, toApprove))

	list := func(status KYCStatus) map[int64]KYCFlowStatus {
		t.Helper()
		page, err := f.module.AdminStores(ctx, StoreListInput{KYCStatus: &status})
		must(t, err)
		out := map[int64]KYCFlowStatus{}
		for _, item := range page.Items {
			out[item.Store.StoreID] = item.KYCFlow
		}
		return out
	}
	awaiting, err := f.module.AdminStores(ctx, StoreListInput{KYCAwaitingReview: true})
	must(t, err)
	if len(awaiting.Items) != 1 || awaiting.Items[0].Store.StoreID != submittedStore.StoreID || awaiting.Items[0].KYCFlow != FlowSubmitted {
		t.Fatalf("awaiting review=%+v", awaiting.Items)
	}
	if got := list(KYCStatusApproved); len(got) != 1 || got[approvedStore.StoreID] == FlowNotStarted || got[approvedStore.StoreID] == FlowSubmitted {
		t.Fatalf("approved=%v", got)
	}
	if got := list(KYCStatusRejected); len(got) != 0 {
		t.Fatalf("rejected=%v", got)
	}
	// 沒有篩選時，草稿的店家只顯示它的流程狀態，沒有申請的是 NOT_STARTED。
	page, err := f.module.AdminStores(ctx, StoreListInput{})
	must(t, err)
	flows := map[int64]KYCFlowStatus{}
	for _, item := range page.Items {
		flows[item.Store.StoreID] = item.KYCFlow
	}
	if flows[noApplication.StoreID] != FlowNotStarted || flows[draftOnly.StoreID] != FlowDraft || flows[submittedStore.StoreID] != FlowSubmitted {
		t.Fatalf("flows=%v", flows)
	}
}

// 統計：目前的店家總數、依 KYC 申請狀態的家數、台北本月新開的家數（邊界是台北的月初，不是 UTC 的）。
func TestAdminStoreStats(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	taipeiSep30 := time.Date(2026, 9, 30, 15, 59, 59, 0, time.UTC) // 台北 9/30 23:59:59
	taipeiOct1 := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)    // 台北 10/1 00:00:00
	f.openAt(t, taipeiSep30, "九月的店")
	f.openAt(t, taipeiOct1, "十月的店一")
	f.openAt(t, taipeiOct1.Add(10*24*time.Hour), "十月的店二")
	closedAccount, _ := f.open(t)
	f.now = taipeiOct1.Add(time.Hour)
	_, err := f.module.CloseCurrent(ctx, closedAccount)
	must(t, err)
	_, _, _ = f.submitted(t, idB, "012345678902")
	_, _, toApprove := f.submitted(t, idC, "012345678903")
	must(t, approve(f, toApprove))
	_, _, toReject := f.submitted(t, idRep, "012345678904")
	_, err = f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: toReject.ID, ExpectedVersion: toReject.Version, AdminID: 9, Issues: anIssue(), Reason: "照片不清楚"})
	must(t, err)
	accountDraft, _ := f.open(t)
	f.savedDraft(t, kycDraft(accountDraft, idA, bankAcct))

	// 目前的店家：測月界的三家、三家有申請的、一家草稿，共 7 家（已關閉的不算）；其中九月那家以外的 6 家是十月開的。
	f.now = time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	stats, err := f.module.AdminStoreStats(ctx)
	must(t, err)
	if want := (StoreStats{Total: 7, KYCAwaitingReview: 1, KYCApproved: 1, KYCRejected: 1, NewThisMonth: 6}); stats != want {
		t.Fatalf("stats=%+v want=%+v", stats, want)
	}
	// 月界是台北的：時鐘退回 9/30 台北晚上，十月的店不算本月，九月的店算。
	f.now = taipeiSep30
	stats, err = f.module.AdminStoreStats(ctx)
	must(t, err)
	if stats.NewThisMonth != 1 {
		t.Fatalf("September: %+v", stats)
	}
}

func TestStoreKYC(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, none := f.open(t)
	draftAccount, draft := f.open(t)
	f.savedDraft(t, kycDraft(draftAccount, idA, bankAcct))
	submittedAccount, submittedStore, submitted := f.submitted(t, idB, "012345678902")

	if _, ok, err := f.module.StoreKYC(ctx, none.StoreID); err != nil || ok {
		t.Fatalf("no application: ok=%v err=%v", ok, err)
	}
	// 草稿是賣家自己的，後台讀不到。
	if _, ok, err := f.module.StoreKYC(ctx, draft.StoreID); err != nil || ok {
		t.Fatalf("draft: ok=%v err=%v", ok, err)
	}
	got, ok, err := f.module.StoreKYC(ctx, submittedStore.StoreID)
	must(t, err)
	if !ok || got.ID != submitted.ID || got.Status != KYCStatusSubmitted || got.Flow == nil || got.Flow.Status != FlowSubmitted ||
		got.SubmittedAt == nil || got.StoreID != submittedStore.StoreID || got.AccountID != submittedAccount {
		t.Fatalf("got=%+v ok=%v", got, ok)
	}
	if got.IDCardNoMasked == nil || strings.Contains(*got.IDCardNoMasked, idB) || strings.Contains(*got.BankAccountMasked, "012345678902") {
		t.Fatalf("the view must be masked: %+v", got)
	}
	// 取消之後也讀不到。
	if _, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: submittedAccount, ExpectedVersion: submitted.Version}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.module.StoreKYC(ctx, submittedStore.StoreID); err != nil || ok {
		t.Fatalf("cancelled: ok=%v err=%v", ok, err)
	}
	if _, _, err := f.module.StoreKYC(ctx, 987654); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing store: %v", err)
	}
	if _, _, err := f.module.StoreKYC(ctx, 0); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("zero store: %v", err)
	}
}

// approvedWith 開店、送出並核准一份申請（PayUni 註冊是 PENDING，job 還沒跑）。
func (f *fixture) approvedWith(t *testing.T, idCard, bank string) (int64, Store) {
	t.Helper()
	accountID, s, submitted := f.submitted(t, idCard, bank)
	must(t, approve(f, submitted))
	return accountID, s
}

func listIDs(t *testing.T, f *fixture, in StoreListInput) []int64 {
	t.Helper()
	var out []int64
	for {
		page, err := f.module.AdminStores(context.Background(), in)
		must(t, err)
		out = append(out, storeIDs(page.Items)...)
		if page.Next == nil {
			slices.Sort(out)
			return out
		}
		in.After = page.Next
	}
}

// 篩選的是目前那一筆還沒取消的申請：取消之後重新送出的店家，用 CANCELLED 篩選是 InvalidInput（不是比對到歷史上那一筆），
// 取消後沒有重新送出的店家不在任何 KYC 狀態的結果裡。
func TestAdminStoresKYCFilterOnlyMatchesTheCurrentApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	resubmittedAccount, resubmitted, first := f.submitted(t, idB, "012345678902")
	_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: resubmittedAccount, ExpectedVersion: first.Version})
	must(t, err)
	again := f.savedDraft(t, kycDraft(resubmittedAccount, idB, "012345678902"))
	if again.ID == first.ID {
		t.Fatal("重建的申請應該是新的一列")
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: resubmittedAccount, ExpectedVersion: again.Version})
	must(t, err)
	cancelledAccount, cancelledOnly, toCancel := f.submitted(t, idC, "012345678903")
	_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: cancelledAccount, ExpectedVersion: toCancel.Version})
	must(t, err)

	for _, status := range []KYCStatus{KYCStatusCancelled, KYCStatusDraft, KYCStatusSubmitted, "WEIRD"} {
		if _, err := f.module.AdminStores(ctx, StoreListInput{KYCStatus: &status}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("filter %s: %v", status, err)
		}
	}
	// 「待審核」是待辦的成員（KYCAwaitingReview），不是儲存的狀態：只有重送的那家（營運中、審核中）。
	if got := listIDs(t, f, StoreListInput{KYCAwaitingReview: true}); !slices.Equal(got, []int64{resubmitted.StoreID}) {
		t.Fatalf("submitted=%v want=%v", got, resubmitted.StoreID)
	}
	approved, rejected := KYCStatusApproved, KYCStatusRejected
	for _, status := range []*KYCStatus{&approved, &rejected} {
		if got := listIDs(t, f, StoreListInput{KYCStatus: status}); len(got) != 0 {
			t.Fatalf("%s=%v", *status, got)
		}
	}
	page, err := f.module.AdminStores(ctx, StoreListInput{})
	must(t, err)
	for _, item := range page.Items {
		if item.Store.StoreID == cancelledOnly.StoreID && item.KYCFlow != FlowNotStarted {
			t.Fatalf("a store with only a cancelled application is NOT_STARTED, got %s", item.KYCFlow)
		}
	}
}

// 帳號、PayUni 綁定狀態與停權的篩選。停權和能力檢查是同一個定義：結束、到期之後篩選與 CapabilitiesOf 一起變。
func TestAdminStoresFilterByAccountBindStatusAndSuspension(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	boundAccount, bound := f.approvedWith(t, idA, bankAcct)
	f.sync(t, bound.StoreID)
	_, failed := f.approvedWith(t, idB, "012345678902")
	f.payuni.bindErr = []error{apiErr("AGENT03003")}
	f.sync(t, failed.StoreID)
	_, pending := f.approvedWith(t, idC, "012345678903")
	_, none := f.open(t)
	all := []int64{bound.StoreID, failed.StoreID, pending.StoreID, none.StoreID}
	slices.Sort(all)

	for status, want := range map[PayUniBindStatus][]int64{
		BindStatusBound: {bound.StoreID}, BindStatusFailed: {failed.StoreID}, BindStatusPending: {pending.StoreID},
	} {
		if got := listIDs(t, f, StoreListInput{PayUniBindStatus: &status}); !slices.Equal(got, want) {
			t.Fatalf("bind %s=%v want=%v", status, got, want)
		}
	}
	weird := PayUniBindStatus("WEIRD")
	if _, err := f.module.AdminStores(ctx, StoreListInput{PayUniBindStatus: &weird}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("weird bind: %v", err)
	}

	// 帳號：完全相符，換過店的帳號有兩個 stint。
	if got := listIDs(t, f, StoreListInput{AccountID: &boundAccount}); !slices.Equal(got, []int64{bound.StoreID}) {
		t.Fatalf("account=%v", got)
	}
	_, err := f.module.CloseCurrent(ctx, boundAccount)
	must(t, err)
	reopened, err := f.module.Apply(ctx, validApply(boundAccount))
	must(t, err)
	if got := listIDs(t, f, StoreListInput{AccountID: &boundAccount}); !slices.Equal(got, []int64{reopened.StoreID}) {
		t.Fatalf("current only=%v", got)
	}
	if got := listIDs(t, f, StoreListInput{AccountID: &boundAccount, Statuses: []Status{Active, Closed}}); len(got) != 2 {
		t.Fatalf("with closed=%v", got)
	}
	for _, bad := range []int64{0, -1} {
		if _, err := f.module.AdminStores(ctx, StoreListInput{AccountID: &bad}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("account %d: %v", bad, err)
		}
	}
	other := int64(987654)
	if got := listIDs(t, f, StoreListInput{AccountID: &other}); len(got) != 0 {
		t.Fatalf("other account=%v", got)
	}

	// 停權：風險停權、結束；認定的停權到期。篩選與停權狀態（stint.Derive，能力檢查的來源）永遠一致。
	yes, no := true, false
	check := func(label string, want []int64) {
		t.Helper()
		got := listIDs(t, f, StoreListInput{Suspended: &yes})
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("%s: suspended=%v want=%v", label, got, want)
		}
		rest := listIDs(t, f, StoreListInput{Suspended: &no})
		if len(got)+len(rest) != len(listIDs(t, f, StoreListInput{})) {
			t.Fatalf("%s: suspended %v + not suspended %v 不等於全部", label, got, rest)
		}
		for _, id := range append(slices.Clone(got), rest...) {
			s, err := f.module.Store(ctx, id)
			must(t, err)
			if s.Suspended() != slices.Contains(got, id) {
				t.Fatalf("%s: store %d suspended=%v 與篩選不一致", label, id, s.Suspended())
			}
		}
	}
	check("nobody", nil)
	rec := f.suspend(t, failed.StoreID)
	check("risk suspension", []int64{failed.StoreID})
	_, err = f.lift(ctx, rec)
	must(t, err)
	check("ended", nil)
	ends := f.now.Add(48 * time.Hour)
	f.violate(t, pending.StoreID, SeverityHigh, MeasureSuspension, &ends)
	check("violation suspension", []int64{pending.StoreID})
	f.now = ends.Add(time.Second)
	check("violation suspension expired", nil)
	f.suspend(t, none.StoreID)
	check("risk suspension again", []int64{none.StoreID})
}
