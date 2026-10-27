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

	"github.com/jackc/pgx/v5/pgconn"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// kycApprovedStore 開店並讓 KYC 核准（食品業者登錄核准的前提）。
func (f *fixture) kycApprovedStore(t *testing.T, idCard, bank string) Store {
	t.Helper()
	_, s := f.approvedWith(t, idCard, bank)
	return s
}

func (f *fixture) kycApprovedStoreWithAccount(t *testing.T, idCard, bank string) (int64, Store) {
	t.Helper()
	return f.approvedWith(t, idCard, bank)
}

func (f *fixture) submitFood(t *testing.T, s Store, number string) Store {
	t.Helper()
	out, err := f.module.SubmitFoodRegistration(context.Background(), SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: number})
	must(t, err)
	return out
}

// pendingID 是店家目前待審那一筆的 ID。
func (f *fixture) pendingID(t *testing.T, s Store) uuid.UUID {
	t.Helper()
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if cur.OpenFoodRegistration == nil || cur.OpenFoodRegistration.State != FoodRegPending {
		t.Fatalf("沒有待審的登錄：%+v", cur.OpenFoodRegistration)
	}
	return cur.OpenFoodRegistration.ID
}

func (f *fixture) decideFood(id uuid.UUID, outcome FoodRegState, reason, note string) (FoodRegistrationReview, error) {
	return f.module.DecideFoodRegistration(context.Background(), DecideFoodRegistrationInput{ID: id, AdminID: 9, Outcome: outcome, Reason: reason, InternalNote: note})
}

func (f *fixture) approveFood(t *testing.T, s Store, number string) {
	t.Helper()
	f.approveFoodWithNote(t, s, number, "")
}

func (f *fixture) approveFoodWithNote(t *testing.T, s Store, number, note string) {
	t.Helper()
	f.submitFood(t, s, number)
	_, err := f.decideFood(f.pendingID(t, s), FoodRegApproved, "", note)
	must(t, err)
}

// foodStates 是店家全部登錄列的 (字號, 狀態, 結束原因)，依送出順序；沒有結束原因是空字串。
func (f *fixture) foodStates(t *testing.T, s Store) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT coalesce(registration_number, ''), state, coalesce(end_cause, '') FROM seller.food_registrations WHERE store_id = $1 ORDER BY submitted_at, id", s.StoreID)
	must(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var number, state, cause string
		must(t, rows.Scan(&number, &state, &cause))
		out = append(out, strings.TrimSuffix(fmt.Sprintf("%s/%s/%s", number, state, cause), "/"))
	}
	must(t, rows.Err())
	return out
}

func (f *fixture) approvedNumber(t *testing.T, s Store) string {
	t.Helper()
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if cur.ApprovedFoodRegistration == nil {
		return ""
	}
	return cur.ApprovedFoodRegistration.RegistrationNumber
}

func TestFoodRegistrationIsPendingUntilApprovedThenBecomesTheStoresNumber(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	profile, decided := record[StoreProfileChanged](f), record[FoodRegistrationDecided](f)

	submitted := f.submitFood(t, s, "  A-111  ")
	if submitted.ApprovedFoodRegistration != nil || submitted.OpenFoodRegistration == nil || submitted.OpenFoodRegistration.State != FoodRegPending ||
		submitted.OpenFoodRegistration.RegistrationNumber != "A-111" || submitted.Version != s.Version {
		t.Fatalf("submitted=%+v request=%+v", submitted, submitted.OpenFoodRegistration)
	}
	if len(profile.all()) != 0 || len(decided.all()) != 0 {
		t.Fatalf("送出不該發事件：profile=%d decided=%d", len(profile.all()), len(decided.all()))
	}

	before := f.revalidations(s)
	f.now = f.now.Add(time.Hour)
	review, err := f.decideFood(submitted.OpenFoodRegistration.ID, FoodRegApproved, "", "僅核對有效")
	must(t, err)
	if review.State != FoodRegApproved || review.DecidedByAdminID == nil || *review.DecidedByAdminID != 9 || review.InternalNote != "僅核對有效" ||
		review.Approved == nil || review.Approved.RegistrationNumber != "A-111" || review.CanApprove() || review.CanReject() {
		t.Fatalf("review=%+v", review)
	}
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if cur.ApprovedFoodRegistration == nil || cur.ApprovedFoodRegistration.RegistrationNumber != "A-111" || cur.OpenFoodRegistration != nil ||
		cur.Version != s.Version || !cur.UpdatedAt.Equal(s.UpdatedAt) {
		t.Fatalf("store=%+v", cur)
	}
	if len(profile.all()) != 1 || profile.all()[0].StoreID != s.StoreID || f.revalidations(s) != before+1 {
		t.Fatalf("核准要在同一個 transaction 發 StoreProfileChanged 並排入快取失效：profile=%d revalidations %d→%d", len(profile.all()), before, f.revalidations(s))
	}
	if got := decided.all(); len(got) != 1 || got[0].Outcome != FoodRegApproved || got[0].RegistrationID != review.ID || got[0].StoreID != s.StoreID || got[0].AccountID != s.AccountID ||
		got[0].RegistrationNumber != "A-111" || got[0].RejectReason != "" {
		t.Fatalf("decided=%+v", got)
	}
}

func TestApprovingANewNumberEndsTheOldApprovalAsSuperseded(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-111")

	pending := f.submitFood(t, s, "A-222")
	if pending.ApprovedFoodRegistration == nil || pending.ApprovedFoodRegistration.RegistrationNumber != "A-111" || pending.OpenFoodRegistration.RegistrationNumber != "A-222" {
		t.Fatalf("審核期間舊的核准照常有效：%+v", pending)
	}
	f.mustDecide(t, pending.OpenFoodRegistration.ID, FoodRegApproved)
	if want := []string{"A-111/ENDED/SUPERSEDED", "A-222/APPROVED"}; !slices.Equal(f.foodStates(t, s), want) || f.approvedNumber(t, s) != "A-222" {
		t.Fatalf("states=%v approved=%q", f.foodStates(t, s), f.approvedNumber(t, s))
	}
}

func (f *fixture) mustDecide(t *testing.T, id uuid.UUID, outcome FoodRegState) {
	t.Helper()
	_, err := f.decideFood(id, outcome, map[FoodRegState]string{FoodRegRejected: "不符"}[outcome], "")
	must(t, err)
}

func TestSubmittingWhilePendingReplacesItAndTheSameNumberIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	first := f.submitFood(t, s, "A-1")
	again := f.submitFood(t, s, "A-1")
	if again.OpenFoodRegistration.ID != first.OpenFoodRegistration.ID || again.OpenFoodRegistration.SubmittedAt != first.OpenFoodRegistration.SubmittedAt {
		t.Fatalf("同一個字號重送要沿用待審的那一筆：%+v → %+v", first.OpenFoodRegistration, again.OpenFoodRegistration)
	}
	f.now = f.now.Add(1)
	replaced := f.submitFood(t, s, "A-2")
	if replaced.OpenFoodRegistration.ID == first.OpenFoodRegistration.ID || replaced.OpenFoodRegistration.RegistrationNumber != "A-2" {
		t.Fatalf("replaced=%+v", replaced.OpenFoodRegistration)
	}
	if want := []string{"A-1/WITHDRAWN", "A-2/PENDING"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
	if _, err := f.decideFood(first.OpenFoodRegistration.ID, FoodRegApproved, "", ""); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被取代的那一筆不能裁定：%v", err)
	}
}

// 送出與目前核准相同的字號不建立新登錄：有待審的就撤回它（賣家要的就是目前核准的），沒有就什麼都不做，也不發事件。
func TestSubmittingTheApprovedNumberCreatesNoRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-111")
	profile := record[StoreProfileChanged](f)
	before := f.revalidations(s)

	same := f.submitFood(t, s, " A-111 ")
	if same.ApprovedFoodRegistration == nil || same.ApprovedFoodRegistration.RegistrationNumber != "A-111" || same.OpenFoodRegistration != nil {
		t.Fatalf("same=%+v request=%+v", same.ApprovedFoodRegistration, same.OpenFoodRegistration)
	}
	if want := []string{"A-111/APPROVED"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}

	f.submitFood(t, s, "A-222")
	f.now = f.now.Add(1)
	back := f.submitFood(t, s, "A-111")
	if back.OpenFoodRegistration != nil || back.ApprovedFoodRegistration == nil || back.ApprovedFoodRegistration.RegistrationNumber != "A-111" {
		t.Fatalf("back=%+v request=%+v", back.ApprovedFoodRegistration, back.OpenFoodRegistration)
	}
	if want := []string{"A-111/APPROVED", "A-222/WITHDRAWN"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
	if len(profile.all()) != 0 || f.revalidations(s) != before {
		t.Fatalf("公開內容沒變，不該發事件：profile=%d revalidations %d→%d", len(profile.all()), before, f.revalidations(s))
	}
}

// 賣家的三支寫入在鎖住店家之後以 CapStorefrontWrite 重新授權：停權回 PermissionDenied，等待刪除回 FailedPrecondition，已關閉回 NotFound，
// 而且什麼都不寫。
func TestSellerFoodRegistrationWritesReauthorizeUnderTheStoreLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	writes := map[string]func(Store) error{
		"submit": func(s Store) error {
			_, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: "B-1"})
			return err
		},
		"cancel": func(s Store) error { _, err := f.module.WithdrawFoodRegistration(ctx, s.StoreID); return err },
		"clear":  func(s Store) error { _, err := f.module.ClearFoodRegistration(ctx, s.StoreID); return err },
	}
	setupStore := func(idCard, bank string) Store {
		s := f.kycApprovedStore(t, idCard, bank)
		f.approveFood(t, s, "A-1")
		f.submitFood(t, s, "A-2")
		return s
	}
	suspended := setupStore(idA, bankAcct)
	f.suspend(t, suspended.StoreID)
	deleting := setupStore(idB, "012345678902")
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.identity.LockAccount(ctx, deleting.AccountID); err != nil {
			return err
		}
		return f.module.BeginDeletion(ctx, deleting.StoreID)
	}))
	closed := setupStore(idC, "012345678903")
	_, err := f.module.CloseCurrent(ctx, closed.AccountID)
	must(t, err)
	for _, c := range []struct {
		name  string
		store Store
		kind  apperr.Kind
	}{{"停權", suspended, apperr.KindPermissionDenied}, {"等待刪除", deleting, apperr.KindFailedPrecondition}, {"已關閉", closed, apperr.KindNotFound}} {
		before := f.foodStates(t, c.store)
		for op, write := range writes {
			if err := write(c.store); apperr.KindOf(err) != c.kind {
				t.Errorf("%s %s：err=%v，want %v", c.name, op, err, c.kind)
			}
		}
		if after := f.foodStates(t, c.store); !slices.Equal(before, after) {
			t.Errorf("%s：被拒絕的寫入改了登錄 %v → %v", c.name, before, after)
		}
	}
}

func TestConcurrentSubmitsLeaveExactlyOnePending(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.module.SubmitFoodRegistration(context.Background(), SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: fmt.Sprintf("A-%d", i)})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	if pending := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id = $1 AND state = 'PENDING'", s.StoreID); pending != 1 {
		t.Fatalf("待審 %d 筆：%v", pending, f.foodStates(t, s))
	}
	if withdrawn := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id = $1 AND state = 'WITHDRAWN'", s.StoreID); withdrawn < 1 {
		t.Fatalf("被取代的要撤回：%v", f.foodStates(t, s))
	}
}

// 每家店最多一筆待審、最多一筆核准由兩個 partial UNIQUE 保證（拿掉它們，這個測試會失敗）；不同狀態與不同店家不受限。
func TestPartialUniqueIndexesAllowOnePendingAndOneApprovedPerStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	other := f.kycApprovedStore(t, idB, "012345678902")
	insert := func(store Store, state, number string) error {
		decided := "NULL, NULL"
		if state == "APPROVED" {
			decided = "9, now()"
		}
		_, err := f.db.Conn(ctx).Exec(ctx, fmt.Sprintf(`INSERT INTO seller.food_registrations
			(id, store_id, account_id, registration_number, state, submitted_at, decided_by_admin_id, decided_at)
			VALUES ($1, $2, $3, $4, $5, now() - interval '1 minute', %s)`, decided), uuid.NewV7(), store.StoreID, store.AccountID, number, state)
		return err
	}
	must(t, insert(s, "PENDING", "P-1"))
	must(t, insert(s, "APPROVED", "G-1"))
	must(t, insert(other, "PENDING", "P-2"))
	for name, err := range map[string]error{"second pending": insert(s, "PENDING", "P-3"), "second approved": insert(s, "APPROVED", "G-2")} {
		var pgErr *pgconn.PgError
		if !asPgError(err, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("%s 應該違反 partial UNIQUE：%v", name, err)
		}
	}
}

func asPgError(err error, target **pgconn.PgError) bool {
	for err != nil {
		if e, ok := err.(*pgconn.PgError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestRejectingNeedsAReasonKeepsTheApprovedNumberAndShowsToTheSellerUntilResubmitted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-111")
	pending := f.submitFood(t, s, "A-222")
	profile, decided := record[StoreProfileChanged](f), record[FoodRegistrationDecided](f)
	before := f.revalidations(s)

	if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegRejected, "  ", ""); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("退回沒有理由：%v", err)
	}
	if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegApproved, "不該帶理由", ""); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("核准帶理由：%v", err)
	}
	if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegWithdrawn, "x", ""); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不是核准或退回的裁定：%v", err)
	}
	review, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegRejected, "字號與登錄資料不符", "查無此字號")
	must(t, err)
	if review.State != FoodRegRejected || review.RejectReason != "字號與登錄資料不符" || review.InternalNote != "查無此字號" {
		t.Fatalf("review=%+v", review)
	}
	if got := decided.all(); len(got) != 1 || got[0].Outcome != FoodRegRejected || got[0].RegistrationNumber != "A-222" || got[0].RejectReason != "字號與登錄資料不符" {
		t.Fatalf("退回的事件要帶字號與理由：%+v", got)
	}
	if len(profile.all()) != 0 || f.revalidations(s) != before || f.approvedNumber(t, s) != "A-111" {
		t.Fatalf("退回不改公開的字號：profile=%d revalidations %d→%d approved=%q", len(profile.all()), before, f.revalidations(s), f.approvedNumber(t, s))
	}
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if r := cur.OpenFoodRegistration; r == nil || r.State != FoodRegRejected || r.RejectReason != "字號與登錄資料不符" || r.DecidedAt == nil {
		t.Fatalf("賣家看得到被退回的請求：%+v", r)
	}
	if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegApproved, "", ""); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已裁定的不能再裁定：%v", err)
	}

	f.now = f.now.Add(1)
	resubmitted := f.submitFood(t, s, "A-333")
	if r := resubmitted.OpenFoodRegistration; r == nil || r.State != FoodRegPending || r.RegistrationNumber != "A-333" {
		t.Fatalf("重新送出之後請求是新的待審：%+v", r)
	}
	if want := []string{"A-111/APPROVED", "A-222/REJECTED", "A-333/PENDING"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
}

func TestApprovingRequiresTheAccountsKYCToBeApproved(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, noKYC := f.open(t)
	_, submittedStore, _ := f.submitted(t, idC, "012345678903")
	for name, s := range map[string]Store{"no kyc": noKYC, "kyc submitted": submittedStore} {
		pending := f.submitFood(t, s, "A-"+name)
		review, err := f.module.FoodRegistrationReview(context.Background(), pending.OpenFoodRegistration.ID)
		must(t, err)
		if review.CanApprove() || !review.CanReject() {
			t.Fatalf("%s：can_approve=%v can_reject=%v", name, review.CanApprove(), review.CanReject())
		}
		if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegApproved, "", ""); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s：核准要求 KYC 已核准：%v", name, err)
		}
		if f.approvedNumber(t, s) != "" {
			t.Fatalf("%s：被擋下的核准不能留下核准的登錄", name)
		}
		if _, err := f.decideFood(pending.OpenFoodRegistration.ID, FoodRegRejected, "實名認證未核准前先退回", ""); err != nil {
			t.Fatalf("%s：退回不要求 KYC：%v", name, err)
		}
	}
}

func TestClearEndsTheApprovalAndWithdrawsThePendingImmediately(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-111")
	f.submitFood(t, s, "A-222")
	profile := record[StoreProfileChanged](f)
	before := f.revalidations(s)

	cleared, err := f.module.ClearFoodRegistration(context.Background(), s.StoreID)
	must(t, err)
	if cleared.ApprovedFoodRegistration != nil || cleared.OpenFoodRegistration != nil {
		t.Fatalf("cleared=%+v %+v", cleared.ApprovedFoodRegistration, cleared.OpenFoodRegistration)
	}
	if want := []string{"A-111/ENDED/CLEARED", "A-222/WITHDRAWN"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
	if len(profile.all()) != 1 || f.revalidations(s) != before+1 {
		t.Fatalf("清除公開的字號要發 StoreProfileChanged 並排入快取失效：profile=%d revalidations %d→%d", len(profile.all()), before, f.revalidations(s))
	}
	if _, err := f.module.ClearFoodRegistration(context.Background(), s.StoreID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有可清除的登錄：%v", err)
	}
}

func TestClearingOnlyAPendingRequestDoesNotTouchTheStorePage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.submitFood(t, s, "A-1")
	profile := record[StoreProfileChanged](f)
	before := f.revalidations(s)
	cleared, err := f.module.ClearFoodRegistration(context.Background(), s.StoreID)
	must(t, err)
	if cleared.OpenFoodRegistration != nil || len(profile.all()) != 0 || f.revalidations(s) != before {
		t.Fatalf("request=%+v profile=%d revalidations %d→%d", cleared.OpenFoodRegistration, len(profile.all()), before, f.revalidations(s))
	}
	if want := []string{"A-1/WITHDRAWN"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
}

func TestCancelWithdrawsOnlyThePendingRequestAndKeepsTheApproval(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-111")
	f.submitFood(t, s, "A-222")
	cancelled, err := f.module.WithdrawFoodRegistration(context.Background(), s.StoreID)
	must(t, err)
	if cancelled.ApprovedFoodRegistration == nil || cancelled.ApprovedFoodRegistration.RegistrationNumber != "A-111" || cancelled.OpenFoodRegistration != nil {
		t.Fatalf("cancelled=%+v", cancelled)
	}
	if want := []string{"A-111/APPROVED", "A-222/WITHDRAWN"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
	if _, err := f.module.WithdrawFoodRegistration(context.Background(), s.StoreID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有待審的：%v", err)
	}
}

func TestSubmitValidatesTheNumberAndNeedsAnActiveStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	for name, number := range map[string]string{"empty": "", "blank": "   ", "too long": strings.Repeat("字", 51)} {
		if _, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: number}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	if _, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: strings.Repeat("字", 50)}); err != nil {
		t.Fatalf("50 字可以：%v", err)
	}
	if _, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID + 1000, RegistrationNumber: "A-1"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有這家店：%v", err)
	}
}

// 關店不撤回待審的件；收件匣只列營運中的店，裁定已關閉的店回 FailedPrecondition，賣家寫入回 NotFound（店家已關閉）。
func TestClosedStoreLeavesTheInboxAndCannotBeDecided(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	pending := f.submitFood(t, s, "A-1")
	if got := f.inbox(t, ""); len(got) != 1 {
		t.Fatalf("inbox=%v", got)
	}
	_, err := f.module.CloseCurrent(ctx, s.AccountID)
	must(t, err)
	if got := f.inbox(t, ""); len(got) != 0 {
		t.Fatalf("已關閉的店不在收件匣：%v", got)
	}
	if want := []string{"A-1/PENDING"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("關店不撤回待審：%v", f.foodStates(t, s))
	}
	for _, outcome := range []FoodRegState{FoodRegApproved, FoodRegRejected} {
		if _, err := f.decideFood(pending.OpenFoodRegistration.ID, outcome, map[FoodRegState]string{FoodRegRejected: "x"}[outcome], ""); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s 已關閉的店：%v", outcome, err)
		}
	}
	if _, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{AcceptedAgreements: CurrentFoodRegistrationAcceptances(), StoreID: s.StoreID, RegistrationNumber: "A-2"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("已關閉的店送審：%v", err)
	}
	review, err := f.module.FoodRegistrationReview(ctx, pending.OpenFoodRegistration.ID)
	must(t, err)
	if review.StoreStatus != model.Closed || review.CanReject() || review.CanApprove() {
		t.Fatalf("review=%+v", review)
	}
}

func (f *fixture) inbox(t *testing.T, query string) []string {
	t.Helper()
	return f.inboxOf(t, nil, query)
}

// inboxOf 一次一筆翻完指定隊列（nil 是預設）。
func (f *fixture) inboxOf(t *testing.T, inbox *FoodRegInbox, query string) []string {
	t.Helper()
	var out []string
	var after *FoodRegistrationCursor
	for {
		page, err := f.module.FoodRegistrations(context.Background(), FoodRegistrationListInput{Inbox: inbox, Query: query, PageSize: 1, After: after})
		must(t, err)
		for _, item := range page.Items {
			out = append(out, item.RegistrationNumber)
		}
		if page.Next == nil {
			return out
		}
		after = page.Next
	}
}

func inboxQueue(q FoodRegInbox) *FoodRegInbox { return &q }

// 食品登錄測試另外需要的身分證號（格式與檢查碼正確，不是真實的人）。
const idD = "W123456789"

// 收件匣預設只列 KYC 已核准、可以裁定的件，等 KYC 的件在另一個隊列，兩者都是送出時間舊到新；歷史依狀態篩選、新到舊；字號與店名可以搜尋；
// KYC 只有已核准的才帶主體，草稿與審核中的看不到。
func TestInboxIsOldestFirstAndHistoryNewestFirstAndOnlyApprovedKYCShowsASubject(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, waiting := f.open(t)
	f.submitFood(t, waiting, "W-1")
	f.now = f.now.Add(1)
	approvedKYC := f.kycApprovedStore(t, idA, bankAcct)
	f.submitFood(t, approvedKYC, "K-1")
	f.now = f.now.Add(1)
	_, reviewing, _ := f.submitted(t, idC, "012345678903")
	f.submitFood(t, reviewing, "R-1")
	f.now = f.now.Add(1)
	later := f.kycApprovedStore(t, idB, "012345678902")
	f.submitFood(t, later, "K-2")

	if got := f.inbox(t, ""); !slices.Equal(got, []string{"K-1", "K-2"}) {
		t.Fatalf("預設只列可以裁定的：%v", got)
	}
	if got := f.inboxOf(t, inboxQueue(FoodRegInboxDecidable), ""); !slices.Equal(got, []string{"K-1", "K-2"}) {
		t.Fatalf("DECIDABLE=%v", got)
	}
	if got := f.inboxOf(t, inboxQueue(FoodRegInboxAwaitingKYC), ""); !slices.Equal(got, []string{"W-1", "R-1"}) {
		t.Fatalf("AWAITING_KYC=%v", got)
	}
	if got := f.inbox(t, "w-"); len(got) != 0 {
		t.Fatalf("搜尋只在隊列內：%v", got)
	}
	if got := f.inboxOf(t, inboxQueue(FoodRegInboxAwaitingKYC), "w-"); !slices.Equal(got, []string{"W-1"}) {
		t.Fatalf("search=%v", got)
	}
	pending := FoodRegPending
	for _, in := range []FoodRegistrationListInput{{State: &pending, PageSize: 10}, {PageSize: 10}} {
		decidable, err := f.module.FoodRegistrations(ctx, in)
		must(t, err)
		if len(decidable.Items) != 2 || decidable.Items[0].KYCStatus == nil || *decidable.Items[0].KYCStatus != KYCStatusApproved || decidable.Items[0].Subject == nil || decidable.Items[0].Subject.RealName == "" ||
			decidable.Items[1].Subject == nil || !decidable.Items[0].CanApprove() {
			t.Fatalf("items=%+v", decidable.Items)
		}
	}
	first, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: inboxQueue(FoodRegInboxAwaitingKYC), PageSize: 10})
	must(t, err)
	if len(first.Items) != 2 || first.Items[0].KYCStatus != nil || first.Items[0].Subject != nil || first.Items[0].CanApprove() || !first.Items[0].CanReject() ||
		first.Items[1].KYCStatus == nil || *first.Items[1].KYCStatus == KYCStatusApproved || first.Items[1].Subject != nil {
		t.Fatalf("items=%+v", first.Items)
	}
	decidable, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{PageSize: 10})
	must(t, err)
	first.Items = []FoodRegistrationReview{first.Items[0], decidable.Items[0], first.Items[1], decidable.Items[1]}

	f.now = f.now.Add(1)
	f.mustDecide(t, first.Items[1].ID, FoodRegRejected)
	f.mustDecide(t, first.Items[3].ID, FoodRegApproved)
	for state, want := range map[FoodRegState][]string{FoodRegRejected: {"K-1"}, FoodRegApproved: {"K-2"}} {
		page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{State: &state, PageSize: 10})
		must(t, err)
		var got []string
		for _, item := range page.Items {
			got = append(got, item.RegistrationNumber)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s：%v，want %v", state, got, want)
		}
	}
	if got := f.inbox(t, ""); len(got) != 0 {
		t.Fatalf("裁定之後離開收件匣：%v", got)
	}
	if got := f.inboxOf(t, inboxQueue(FoodRegInboxAwaitingKYC), ""); !slices.Equal(got, []string{"W-1", "R-1"}) {
		t.Fatalf("等 KYC 的件不受影響：%v", got)
	}
	if _, err := f.module.FoodRegistrationReview(ctx, uuid.NewV7()); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的登錄：%v", err)
	}
}

// 後台看不到賣家的 KYC 草稿與審核中的內容：草稿、審核中、被退回之後重新編輯都只回狀態、不回主體（real_name、統編），核准之後才有。
func TestAdminNeverSeesAnUnapprovedKYCSubjectOnAFoodRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	subjectOf := func(s Store) (*KYCStatus, *FoodRegistrationSubject) {
		t.Helper()
		pending, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		review, err := f.module.FoodRegistrationReview(ctx, pending.OpenFoodRegistration.ID)
		must(t, err)
		return review.KYCStatus, review.Subject
	}
	expect := func(step string, s Store, status KYCStatus, hasSubject bool) {
		t.Helper()
		got, subject := subjectOf(s)
		if got == nil || *got != status || (subject != nil) != hasSubject {
			t.Fatalf("%s：status=%v subject=%+v", step, got, subject)
		}
		for _, inbox := range FoodRegInboxValues() {
			page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: &inbox, PageSize: 100})
			must(t, err)
			for _, item := range page.Items {
				if item.StoreID == s.StoreID && (item.Subject != nil) != hasSubject {
					t.Fatalf("%s：收件匣 %s subject=%+v", step, inbox, item.Subject)
				}
			}
		}
	}
	account, s := f.open(t)
	draft := f.savedDraft(t, kycDraft(account, idA, bankAcct))
	f.submitFood(t, s, "A-1")
	expect("草稿", s, KYCStatusDraft, false)

	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: account, ExpectedVersion: draft.Version})
	must(t, err)
	expect("審核中", s, KYCStatusSubmitted, false)

	rejected, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "請補件"})
	must(t, err)
	expect("被退回", s, KYCStatusRejected, false)

	edited := f.savedDraft(t, KYCDraftInput{AccountID: account, ExpectedVersion: rejected.Version, RealName: ptr("改過的草稿姓名")})
	expect("退回後重新編輯", s, KYCStatusDraft, false)

	resubmitted, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: account, ExpectedVersion: edited.Version})
	must(t, err)
	must(t, approve(f, resubmitted))
	expect("核准", s, KYCStatusApproved, true)
	if _, subject := subjectOf(s); subject.RealName != "改過的草稿姓名" {
		t.Fatalf("主體是核准那一份：%+v", subject)
	}
}

// 等 KYC 的隊列翻頁途中，店家的 KYC 核准了：游標只用 (送出時間, id)，已經翻過的與還沒翻到的核准都不會讓留在隊列的件重複或漏掉；
// 核准的件離開這個隊列、進入可裁定的隊列。
func TestAwaitingKYCPaginationIsStableWhenKYCIsApprovedBetweenPages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var apps []KYCApplication
	for i, c := range []struct{ id, bank string }{{idA, bankAcct}, {idB, "012345678902"}, {idC, "012345678903"}, {idRep, "123123123123"}, {idD, "012345678904"}} {
		_, s, sub := f.submitted(t, c.id, c.bank)
		f.now = f.now.Add(1)
		f.submitFood(t, s, fmt.Sprintf("P-%d", i+1))
		apps = append(apps, sub)
	}
	awaiting := inboxQueue(FoodRegInboxAwaitingKYC)
	page1, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: awaiting, PageSize: 2})
	must(t, err)
	if page1.Next == nil || len(page1.Items) != 2 || page1.Items[0].RegistrationNumber != "P-1" || page1.Items[1].RegistrationNumber != "P-2" {
		t.Fatalf("page1=%+v", page1)
	}
	must(t, approve(f, apps[1]))
	must(t, approve(f, apps[3]))
	page2, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: awaiting, PageSize: 2, After: page1.Next})
	must(t, err)
	var got []string
	for _, item := range page2.Items {
		got = append(got, item.RegistrationNumber)
	}
	if !slices.Equal(got, []string{"P-3", "P-5"}) || page2.Next != nil {
		t.Fatalf("留在隊列的件不重複不漏：%v next=%v", got, page2.Next)
	}
	if got := f.inbox(t, ""); !slices.Equal(got, []string{"P-2", "P-4"}) {
		t.Fatalf("核准的件進入可裁定的隊列：%v", got)
	}
}

// 可裁定的隊列翻頁途中，別家店的 KYC 核准了：已經翻過的位置之後加入的件會在後面翻到，之前加入的這一輪不會再出現，沒有重複，原本的件不漏。
func TestDecidablePaginationIsStableWhenKYCIsApprovedBetweenPages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, early, earlySub := f.submitted(t, idA, bankAcct)
	f.now = f.now.Add(1)
	f.submitFood(t, early, "E-0")
	for i, c := range []struct{ id, bank string }{{idB, "012345678902"}, {idC, "012345678903"}, {idRep, "123123123123"}} {
		f.now = f.now.Add(1)
		f.submitFood(t, f.kycApprovedStore(t, c.id, c.bank), fmt.Sprintf("D-%d", i+1))
	}
	f.now = f.now.Add(1)
	_, late, lateSub := f.submitted(t, idD, "012345678904")
	f.submitFood(t, late, "L-1")

	page1, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{PageSize: 2})
	must(t, err)
	if page1.Next == nil || len(page1.Items) != 2 || page1.Items[0].RegistrationNumber != "D-1" || page1.Items[1].RegistrationNumber != "D-2" {
		t.Fatalf("page1=%+v", page1)
	}
	must(t, approve(f, earlySub))
	must(t, approve(f, lateSub))
	page2, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{PageSize: 10, After: page1.Next})
	must(t, err)
	var got []string
	for _, item := range page2.Items {
		got = append(got, item.RegistrationNumber)
	}
	if !slices.Equal(got, []string{"D-3", "L-1"}) || page2.Next != nil {
		t.Fatalf("page2=%v next=%v", got, page2.Next)
	}
	if got := f.inbox(t, ""); !slices.Equal(got, []string{"E-0", "D-1", "D-2", "D-3", "L-1"}) {
		t.Fatalf("重新載入：%v", got)
	}
}

// 歷史清單不可以指定收件匣隊列；隊列不合法回 InvalidInput。
func TestFoodRegistrationListRejectsInboxOnHistoryAndUnknownInbox(t *testing.T) {
	t.Parallel()
	f := setup(t)
	approved, bad := FoodRegApproved, FoodRegInbox("NOPE")
	for name, in := range map[string]FoodRegistrationListInput{
		"歷史":  {State: &approved, Inbox: inboxQueue(FoodRegInboxDecidable)},
		"不合法": {Inbox: &bad},
	} {
		if _, err := f.module.FoodRegistrations(context.Background(), in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：%v", name, err)
		}
	}
	pending := FoodRegPending
	if _, err := f.module.FoodRegistrations(context.Background(), FoodRegistrationListInput{State: &pending, Inbox: inboxQueue(FoodRegInboxAwaitingKYC)}); err != nil {
		t.Fatalf("PENDING 就是收件匣：%v", err)
	}
}

// 公開的讀取只讀核准的那一筆：待審與被退回的不公開，清除之後消失；店家清單與批次讀取帶同一個核准的字號。
func TestPublicStoreReadsOnlyTheApprovedRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.exposable(t)
	publicNumber := func() string {
		got, err := f.module.PublicStoreByCode(ctx, s.StoreCode)
		must(t, err)
		if got.ApprovedFoodRegistration == nil {
			return ""
		}
		return got.ApprovedFoodRegistration.RegistrationNumber
	}
	listedNumber := func() string {
		for _, item := range f.listPublic(t, 1000, nil).Items {
			if item.StoreID == s.StoreID && item.ApprovedFoodRegistration != nil {
				return item.ApprovedFoodRegistration.RegistrationNumber
			}
		}
		return ""
	}
	if publicNumber() != "" || listedNumber() != "" {
		t.Fatal("沒有登錄的店不該有字號")
	}
	pending := f.submitFood(t, s, "A-123")
	if publicNumber() != "" || listedNumber() != "" {
		t.Fatalf("待審的不公開：%q %q", publicNumber(), listedNumber())
	}
	f.now = f.now.Add(time.Hour)
	f.mustDecide(t, pending.OpenFoodRegistration.ID, FoodRegApproved)
	if publicNumber() != "A-123" || listedNumber() != "A-123" {
		t.Fatalf("核准之後公開：%q %q", publicNumber(), listedNumber())
	}
	public, err := f.module.PublicStoreByCode(ctx, s.StoreCode)
	must(t, err)
	if !public.UpdateTime.After(s.UpdatedAt) {
		t.Fatalf("核准要推進公開的 update_time：%v → %v", s.UpdatedAt, public.UpdateTime)
	}
	batch, err := f.module.BatchStores(ctx, []int64{s.StoreID})
	must(t, err)
	admin, err := f.module.AdminStores(ctx, StoreListInput{})
	must(t, err)
	var adminNumber string
	for _, item := range admin.Items {
		if item.Store.StoreID == s.StoreID && item.Store.ApprovedFoodRegistration != nil {
			adminNumber = item.Store.ApprovedFoodRegistration.RegistrationNumber
		}
	}
	if len(batch) != 1 || batch[0].ApprovedFoodRegistration == nil || batch[0].ApprovedFoodRegistration.RegistrationNumber != "A-123" || adminNumber != "A-123" {
		t.Fatalf("batch=%+v admin=%q", batch, adminNumber)
	}
	f.now = f.now.Add(time.Hour)
	_, err = f.module.ClearFoodRegistration(ctx, s.StoreID)
	must(t, err)
	if publicNumber() != "" || listedNumber() != "" {
		t.Fatalf("清除之後不公開：%q %q", publicNumber(), listedNumber())
	}
	cleared, err := f.module.PublicStoreByCode(ctx, s.StoreCode)
	must(t, err)
	if !cleared.UpdateTime.Equal(f.now) {
		t.Fatalf("清除也要推進公開的 update_time：%v want %v", cleared.UpdateTime, f.now)
	}
	own, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if !own.UpdatedAt.Equal(s.UpdatedAt) || own.Version != s.Version {
		t.Fatalf("登錄不改店家的 updated_at 與版本：%v/%d → %v/%d", s.UpdatedAt, s.Version, own.UpdatedAt, own.Version)
	}
	var listedAt time.Time
	for _, item := range f.listPublic(t, 1000, nil).Items {
		if item.StoreID == s.StoreID {
			listedAt = item.UpdateTime
		}
	}
	if !listedAt.Equal(cleared.UpdateTime) {
		t.Fatalf("清單與單筆的 update_time 要一致：%v vs %v", listedAt, cleared.UpdateTime)
	}
}

// 店家與 KYC 申請不再有食品欄位：資料庫沒有那些欄位，KYC 的欄位名單與退回問題都沒有 food_business。
func TestStoresAndKYCApplicationsCarryNoFoodFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if n := f.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name IN ('stores', 'kyc_applications')
		AND column_name IN ('is_processed_food', 'food_business_reg_no')`); n != 0 {
		t.Fatalf("還有 %d 個食品欄位", n)
	}
	if slices.Contains(model.KYCFieldNames(), "food_business") {
		t.Fatal("KYCFieldNames 不該有 food_business")
	}
	_, _, sub := f.submitted(t, idA, bankAcct)
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{
		ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "請補件",
		Issues: []KYCIssue{{Field: "food_business", Code: IssueOther, Message: "請確認"}},
	})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("退回問題指向已移除的欄位：%v", err)
	}
}

func (f *fixture) revokeFood(id uuid.UUID, reason string) (FoodRegistrationReview, error) {
	return f.module.RevokeFoodRegistration(context.Background(), RevokeFoodRegistrationInput{ID: id, AdminID: 11, Reason: reason})
}

func (f *fixture) approvedID(t *testing.T, s Store) uuid.UUID {
	t.Helper()
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if cur.ApprovedFoodRegistration == nil {
		t.Fatal("沒有核准的登錄")
	}
	return cur.ApprovedFoodRegistration.ID
}

// 撤銷核准中的登錄：結束成 ENDED／REVOKED，帶給賣家看的理由與撤銷的管理員，核准時的內部註記留著；店家公開的字號立即消失（StoreProfileChanged、
// 快取失效、公開的 update_time 前進）；發 FoodRegistrationRevoked；賣家看到撤銷的那一筆。
func TestRevokeEndsTheApprovalWithAReasonAndTheSellerSeesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.exposable(t)
	f.approveFoodWithNote(t, s, "A-111", "核准時：已對照食藥署")
	id := f.approvedID(t, s)
	profile, revoked := record[StoreProfileChanged](f), record[FoodRegistrationRevoked](f)
	before := f.revalidations(s)
	f.now = f.now.Add(time.Hour)

	review, err := f.revokeFood(id, "  字號與食藥署資料不符  ")
	must(t, err)
	if review.State != FoodRegEnded || review.EndCause != FoodRegRevoked || review.EndReason != "字號與食藥署資料不符" || review.EndedByAdminID == nil || *review.EndedByAdminID != 11 ||
		review.EndedAt == nil || !review.EndedAt.Equal(f.now.UTC().Truncate(time.Microsecond)) || review.InternalNote != "核准時：已對照食藥署" || review.CanRevoke() || review.Approved != nil {
		t.Fatalf("review=%+v", review)
	}
	if want := []string{"A-111/ENDED/REVOKED"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
	if got := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE id=$1 AND internal_note='核准時：已對照食藥署'", id); got != 1 {
		t.Fatalf("撤銷不能清掉核准時的內部註記（稽核紀錄只增不改）：%d", got)
	}
	if len(profile.all()) != 1 || f.revalidations(s) != before+1 {
		t.Fatalf("撤銷要發 StoreProfileChanged 並排入快取失效：profile=%d revalidations %d→%d", len(profile.all()), before, f.revalidations(s))
	}
	if got := revoked.all(); len(got) != 1 || got[0].RegistrationID != id || got[0].StoreID != s.StoreID || got[0].AccountID != s.AccountID ||
		got[0].RegistrationNumber != "A-111" || got[0].Reason != "字號與食藥署資料不符" {
		t.Fatalf("revoked=%+v", got)
	}
	public, err := f.module.PublicStoreByCode(ctx, s.StoreCode)
	must(t, err)
	if public.ApprovedFoodRegistration != nil || !public.UpdateTime.Equal(f.now) {
		t.Fatalf("撤銷之後不公開、公開的 update_time 前進：%+v %v want %v", public.ApprovedFoodRegistration, public.UpdateTime, f.now)
	}
	own, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if own.ApprovedFoodRegistration != nil || own.RevokedFoodRegistration == nil || own.RevokedFoodRegistration.RegistrationNumber != "A-111" ||
		own.RevokedFoodRegistration.EndReason != "字號與食藥署資料不符" || own.RevokedFoodRegistration.EndedAt == nil || own.OpenFoodRegistration != nil {
		t.Fatalf("store=%+v", own)
	}
}

func TestRevokeValidatesAndOnlyAppliesToTheApprovedRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.kycApprovedStore(t, idA, bankAcct)
	pending := f.submitFood(t, s, "A-1")
	if _, err := f.revokeFood(pending.OpenFoodRegistration.ID, "待審"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("待審不能撤銷：%v", err)
	}
	f.mustDecide(t, pending.OpenFoodRegistration.ID, FoodRegApproved)
	id := f.approvedID(t, s)
	for _, reason := range []string{"", "   ", strings.Repeat("理", 1001)} {
		if _, err := f.revokeFood(id, reason); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("理由 %q：%v", reason, err)
		}
	}
	if _, err := f.module.RevokeFoodRegistration(context.Background(), RevokeFoodRegistrationInput{ID: id, AdminID: 0, Reason: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("管理員 ID：%v", err)
	}
	if _, err := f.revokeFood(uuid.NewV7(), "x"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
	if got := f.approvedNumber(t, s); got != "A-1" {
		t.Fatalf("失敗的撤銷不能動核准的登錄：%q", got)
	}
	if _, err := f.revokeFood(id, "不符"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.revokeFood(id, "再撤銷一次"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已結束：%v", err)
	}
	// 撤銷、清除與取代各自留下自己的結束原因；撤銷的那一筆不會再被當成「清除」。
	f.approveFood(t, s, "A-2")
	if _, err := f.module.ClearFoodRegistration(context.Background(), s.StoreID); err != nil {
		t.Fatal(err)
	}
	if want := []string{"A-1/ENDED/REVOKED", "A-2/ENDED/CLEARED"}; !slices.Equal(f.foodStates(t, s), want) {
		t.Fatalf("states=%v", f.foodStates(t, s))
	}
}

// 賣家看到撤銷的那一筆，條件是目前沒有核准、撤銷之後也沒有更新的待審或核准：重新送審後它消失，撤回那筆待審後又回來；新的核准之後消失；
// 撤銷時就已存在的待審件（送出早於撤銷）不影響。
func TestRevokedRegistrationShowsOnlyWithoutANewerPendingOrApproval(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-1")
	f.now = f.now.Add(time.Hour)
	f.submitFood(t, s, "A-2")
	_, err := f.revokeFood(f.approvedID(t, s), "不符")
	must(t, err)
	own := func() Store {
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		return cur
	}
	if cur := own(); cur.RevokedFoodRegistration == nil || cur.RevokedFoodRegistration.RegistrationNumber != "A-1" || cur.OpenFoodRegistration == nil || cur.OpenFoodRegistration.RegistrationNumber != "A-2" {
		t.Fatalf("撤銷時已存在的待審件不影響：%+v %+v", cur.RevokedFoodRegistration, cur.OpenFoodRegistration)
	}
	_, err = f.module.WithdrawFoodRegistration(ctx, s.StoreID)
	must(t, err)
	f.now = f.now.Add(time.Hour)
	f.submitFood(t, s, "A-3")
	if cur := own(); cur.RevokedFoodRegistration != nil || cur.OpenFoodRegistration == nil || cur.OpenFoodRegistration.RegistrationNumber != "A-3" {
		t.Fatalf("撤銷之後送出待審：撤銷的說明消失：%+v", cur.RevokedFoodRegistration)
	}
	_, err = f.module.WithdrawFoodRegistration(ctx, s.StoreID)
	must(t, err)
	if cur := own(); cur.RevokedFoodRegistration == nil || cur.RevokedFoodRegistration.RegistrationNumber != "A-1" {
		t.Fatalf("待審撤回之後撤銷的說明回來：%+v", cur.RevokedFoodRegistration)
	}
	f.now = f.now.Add(time.Hour)
	f.approveFood(t, s, "A-4")
	if cur := own(); cur.RevokedFoodRegistration != nil || cur.ApprovedFoodRegistration == nil {
		t.Fatalf("新的核准之後撤銷的說明消失：%+v", cur.RevokedFoodRegistration)
	}
	f.now = f.now.Add(time.Hour)
	_, err = f.revokeFood(f.approvedID(t, s), "又不符")
	must(t, err)
	if cur := own(); cur.RevokedFoodRegistration == nil || cur.RevokedFoodRegistration.RegistrationNumber != "A-4" || cur.RevokedFoodRegistration.EndReason != "又不符" {
		t.Fatalf("顯示最近一次撤銷：%+v", cur.RevokedFoodRegistration)
	}
}

// 撤銷之後有更新的核准，即使那一筆之後又結束（清除），撤銷的說明也不會回來：賣家已經重新取得過核准，舊的撤銷不再是現況。
func TestRevokedRegistrationStaysHiddenAfterANewerApprovalEnds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-1")
	f.now = f.now.Add(time.Hour)
	_, err := f.revokeFood(f.approvedID(t, s), "不符")
	must(t, err)
	f.now = f.now.Add(time.Hour)
	f.approveFood(t, s, "A-2")
	f.now = f.now.Add(time.Hour)
	_, err = f.module.ClearFoodRegistration(ctx, s.StoreID)
	must(t, err)
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if cur.ApprovedFoodRegistration != nil || cur.RevokedFoodRegistration != nil || cur.OpenFoodRegistration != nil {
		t.Fatalf("A 撤銷、B 核准、B 清除：不顯示撤銷：%+v", cur.RevokedFoodRegistration)
	}
	if got := f.foodStates(t, s); !slices.Equal(got, []string{"A-1/ENDED/REVOKED", "A-2/ENDED/CLEARED"}) {
		t.Fatalf("states=%v", got)
	}
}

// 核准的列一定有字號：資料庫拒絕把核准中的登錄清成沒有字號（個資清理要先結束它）。
func TestApprovedRegistrationCannotLoseItsNumber(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	f.approveFood(t, s, "A-1")
	_, err := f.db.Conn(ctx).Exec(ctx, `UPDATE seller.food_registrations SET registration_number=NULL, pii_purged_at=now() WHERE store_id=$1`, s.StoreID)
	var pgErr *pgconn.PgError
	if !asPgError(err, &pgErr) || pgErr.ConstraintName != "food_registrations_approved_number_check" {
		t.Fatalf("要違反 approved_number_check：%v", err)
	}
}

// 送出時確認食品業者登錄的個資告知：先於其他檢查驗證（種類、版本），建立新的登錄時紀錄綁定那一筆；沿用待審或與核准相同而沒有建立新登錄時不另記。
func TestSubmitRecordsTheNoticeAgainstTheNewRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	good := CurrentFoodRegistrationAcceptances()
	submit := func(number string, accepted []AgreementAcceptance) error {
		_, err := f.module.SubmitFoodRegistration(ctx, SubmitFoodRegistrationInput{StoreID: s.StoreID, RegistrationNumber: number, AcceptedAgreements: accepted, ClientIP: "203.0.113.9", UserAgent: "test-agent"})
		return err
	}
	outdated := []AgreementAcceptance{{Kind: good[0].Kind, Version: "2026-01-01"}}
	if ae := apperrOf(t, submit("A-1", outdated)); ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonAgreementOutdated {
		t.Fatalf("過期的版本：%v", ae)
	}
	for name, accepted := range map[string][]AgreementAcceptance{
		"缺":      nil,
		"種類不對":   CurrentKYCAcceptances(),
		"多一份":    append(slices.Clone(good), CurrentKYCAcceptances()...),
		"重複":     {good[0], good[0]},
		"空版本":    {{Kind: good[0].Kind}},
		"不認得的種類": {{Kind: "NOPE", Version: "x"}},
	} {
		if err := submit("A-1", accepted); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.food_registrations WHERE store_id = $1", s.StoreID); n != 0 {
		t.Fatalf("被拒的送出不能建立登錄：%d", n)
	}
	must(t, submit("A-1", good))
	var kind, version, ip, ua string
	var regID, boundTo uuid.UUID
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT a.kind, a.version, host(a.client_ip), a.user_agent, a.food_registration_id, f.id
		FROM seller.agreement_acceptances a JOIN seller.food_registrations f ON f.id = a.food_registration_id WHERE a.store_id = $1`, s.StoreID)
	must(t, err)
	n := 0
	for rows.Next() {
		n++
		must(t, rows.Scan(&kind, &version, &ip, &ua, &regID, &boundTo))
	}
	must(t, rows.Err())
	rows.Close()
	if n != 1 || kind != string(good[0].Kind) || version != good[0].Version || ip != "203.0.113.9" || ua != "test-agent" || regID != boundTo {
		t.Fatalf("n=%d kind=%s version=%s ip=%s ua=%s", n, kind, version, ip, ua)
	}
	// 同一個待審字號重送沒有建立新登錄，不另記；換字號建立新登錄，各記一列。
	must(t, submit("A-1", good))
	if got := f.count(t, "SELECT count(*) FROM seller.agreement_acceptances WHERE store_id = $1 AND kind = 'FOOD_REGISTRATION_NOTICE'", s.StoreID); got != 1 {
		t.Fatalf("沿用待審不另記：%d", got)
	}
	must(t, submit("A-2", good))
	if got := f.count(t, "SELECT count(*) FROM seller.agreement_acceptances WHERE store_id = $1 AND kind = 'FOOD_REGISTRATION_NOTICE'", s.StoreID); got != 2 {
		t.Fatalf("新的登錄各記一列：%d", got)
	}
}

// 同意紀錄依種類恰好綁一個來源（資料庫的 CHECK）。
func TestAgreementAcceptancesBindExactlyOneSourceByKind(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s := f.kycApprovedStore(t, idA, bankAcct)
	food := f.submitFood(t, s, "A-1")
	var reviewID uuid.UUID
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT submission_review_id FROM seller.agreement_acceptances WHERE store_id = $1 AND submission_review_id IS NOT NULL LIMIT 1", s.StoreID).Scan(&reviewID))
	insert := func(kind string, review, reg any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.agreement_acceptances (store_id, kind, version, account_id, accepted_at, submission_review_id, food_registration_id)
			VALUES ($1, $2, 'v', $3, now(), $4, $5)`, s.StoreID, kind, s.AccountID, review, reg)
		return err
	}
	regID := food.OpenFoodRegistration.ID
	for name, err := range map[string]error{
		"食品告知沒有登錄":   insert("FOOD_REGISTRATION_NOTICE", nil, nil),
		"食品告知綁了審核紀錄": insert("FOOD_REGISTRATION_NOTICE", reviewID, nil),
		"食品告知兩個來源":   insert("FOOD_REGISTRATION_NOTICE", reviewID, regID),
		"KYC 告知綁了登錄": insert("KYC_PRIVACY_NOTICE", reviewID, regID),
		"KYC 告知沒有來源": insert("KYC_PRIVACY_NOTICE", nil, nil),
		"賣家契約綁了登錄":   insert("SELLER_TERMS", nil, regID),
	} {
		if err == nil {
			t.Errorf("%s 寫進去了", name)
		}
	}
	if err := insert("FOOD_REGISTRATION_NOTICE", nil, regID); err != nil {
		t.Fatalf("食品告知綁登錄：%v", err)
	}
}
