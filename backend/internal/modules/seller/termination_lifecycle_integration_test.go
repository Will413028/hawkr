//go:build integration

package seller_test

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// terminated 開店、建立 21.3 終止並關店（沒有未結訂單）；回傳帳號、店家與措施。
func (f *fixture) terminated(t *testing.T, ground TerminationGround) (int64, Store, Measure) {
	t.Helper()
	accountID, s := f.open(t)
	m := f.terminate(t, s.StoreID, ground)
	f.effect(t, m)
	if got := f.storeRow(t, s.StoreID); got.Status != Closed || got.Closure == nil || got.Closure.By != ClosedByTermination {
		t.Fatalf("終止之後要關店：%+v", got)
	}
	return accountID, s, f.measureOf(t, m.ID)
}

func (f *fixture) revoke(t *testing.T, m Measure) Measure {
	t.Helper()
	cur := f.measureOf(t, m.ID)
	out, err := f.module.RevokeMeasure(context.Background(), RevokeMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: cur.Version, Reason: RevokeAdminCorrection})
	must(t, err)
	return out
}

func (f *fixture) storeClosures(t *testing.T, storeID int64) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.store_closures WHERE store_id=$1 AND reopened_at IS NOT NULL", storeID)
}

// 申覆期限：終止合作生效後 4416 小時（21.2 從生效起算，不是從建立）；deadline − 1µs 還可申覆，deadline 時 can_appeal 是 false、原因含
// APPEAL_WINDOW_ENDED、SubmitAppeal 回 FailedPrecondition；停權沒有期限、不受影響。原因的順序：已撤銷且期滿是 [REVOKED, APPEAL_WINDOW_ENDED]；
// can_appeal == 原因為空 == SubmitAppeal 成功。
// mutation：起算改成 created_at → 21.2 的邊界失敗；`>=` 改 `>` → 邊界失敗；CanAppeal 自己再寫一次不呼叫原因函式 → 一致性失敗。
func TestAppealWindowOfATermination(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	deadline := notice.StartsAt.Add(4416 * time.Hour)
	if got := notice.AppealDeadline(); got == nil || !got.Equal(deadline) || !deadline.Equal(notice.CreatedAt.Add(720*time.Hour+4416*time.Hour)) {
		t.Fatalf("deadline=%v", got)
	}
	f.now = deadline.Add(-time.Microsecond)
	before := f.measureOf(t, notice.ID)
	if !before.CanAppeal() || len(before.AppealUnavailableReasons()) != 0 {
		t.Fatalf("期限前一微秒可以申訴：%v", before.AppealUnavailableReasons())
	}
	f.now = deadline
	after := f.measureOf(t, notice.ID)
	if after.CanAppeal() || !slices.Equal(after.AppealUnavailableReasons(), []AppealUnavailableReason{AppealReasonWindowEnded}) {
		t.Fatalf("期限那一刻不可申訴：%v", after.AppealUnavailableReasons())
	}
	_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: notice.ID, ExpectedVersion: after.SellerVersion(), Reason: "太晚"})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("期滿後送出要 failed_precondition：%v", err)
	}
	// 停權沒有期限。
	other, s2 := f.open(t)
	ends := f.now.Add(1000 * 24 * time.Hour)
	susp := f.violate(t, s2.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.now = f.now.Add(4417 * time.Hour)
	if got := f.measureOf(t, susp.ID); !got.CanAppeal() || got.AppealDeadline() != nil {
		t.Fatalf("停權沒有申覆期限：%v", got.AppealUnavailableReasons())
	}
	_ = other

	// 已撤銷且期滿：[REVOKED, APPEAL_WINDOW_ENDED]。
	f.now = deadline.Add(-time.Hour)
	revoked := f.revoke(t, notice)
	f.now = deadline
	got := f.measureOf(t, revoked.ID)
	if !slices.Equal(got.AppealUnavailableReasons(), []AppealUnavailableReason{AppealReasonRevoked, AppealReasonWindowEnded}) || got.CanAppeal() {
		t.Fatalf("reasons=%v", got.AppealUnavailableReasons())
	}
}

// 期滿前提出的申覆，期滿後裁定 ACCEPTED：恢復同一個店（legal hold 擋住期間內的清理，裁定之前就算已超過保留期也沒被清）。
// mutation：拿掉 hold 條件，裁定前清理 job 已把店清掉，這裡不能恢復。
func TestAnAppealFiledInTimeAndDecidedAfterTheDeadlineStillReopensTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	retired := record[StoreRetired](f)
	reopened := record[StoreReopened](f)
	accountID, s, m := f.terminated(t, GroundCeasedBusiness)
	f.now = f.now.Add(24 * time.Hour)
	appealed := f.appeal(t, accountID, f.measureOf(t, m.ID), "我沒有停業")
	_ = appealed
	f.now = m.StartsAt.Add(4416*time.Hour + 10*24*time.Hour)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 0 || len(retired.all()) != 0 || f.purged(t, s.StoreID) {
		t.Fatalf("有待處理的申覆時清理暫停：purged=%d", n)
	}
	cur := f.measureOf(t, m.ID)
	_, err = f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestAccepted, Reason: "證明營業中"})
	must(t, err)
	back := f.storeRow(t, s.StoreID)
	if back.Status != Active || back.Closure != nil || back.StoreCode != s.StoreCode || !f.isSeller(t, accountID) {
		t.Fatalf("back=%+v", back)
	}
	if got := reopened.all(); len(got) != 1 || got[0].StoreID != s.StoreID || len(retired.all()) != 0 {
		t.Fatalf("reopened=%+v retired=%+v", got, retired.all())
	}
	if f.measureOf(t, m.ID).State != StateRevoked || f.storeClosures(t, s.StoreID) != 1 {
		t.Fatal("措施撤銷、關店紀錄移到歷史")
	}
}

// 保留期依關閉原因：賣家自己關店，關店後 StoreRetention 前一秒、剛好都不清，晚 1µs 清；因終止而關閉的，店家在終止生效時關閉，關店後的保留期與終止的申覆期限
// （生效後 184 天）同一刻到期，過了才清；訂單拖過申覆期限才關店的，關閉滿保留期才清；關店之後才生效的另一筆終止把保留期延到它自己的申覆期限
// （greatest 的第二項），已撤銷的不延長。
// mutation：TERMINATION 也只用關店後的保留期 → 最後一條失敗；拿掉 greatest 的第二項 → 最後一條失敗；保留期用 30 天 → 各條的「剛好」與「前一秒」失敗。
func TestRetentionDependsOnWhoClosedTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	purge := func() int {
		n, err := f.module.PurgePII(ctx)
		must(t, err)
		return n
	}
	// 賣家自己關店。
	ownerAccount, owner := f.open(t)
	closedAt := f.now
	_, err := f.module.CloseCurrent(ctx, ownerAccount)
	must(t, err)
	f.now = closedAt.Add(StoreRetention - time.Second)
	if purge() != 0 || f.purged(t, owner.StoreID) {
		t.Fatal("保留期前一秒不清")
	}
	f.now = closedAt.Add(StoreRetention)
	if purge() != 0 || f.purged(t, owner.StoreID) {
		t.Fatal("剛好保留期不清")
	}
	f.now = closedAt.Add(StoreRetention + time.Microsecond)
	if purge() != 1 || !f.purged(t, owner.StoreID) {
		t.Fatal("保留期＋1µs 要清")
	}

	// 各段接著上一段的時間往後走（時鐘只前進），每段用自己新開的店家；前一段的店家已經清理，不影響後面的清理筆數。
	// 因終止而關閉：生效後立刻關店，申覆期限之前都不清。
	_, s, m := f.terminated(t, GroundBankruptcy)
	deadline := m.StartsAt.Add(4416 * time.Hour)
	f.now = deadline.Add(-time.Second)
	if purge() != 0 || f.purged(t, s.StoreID) {
		t.Fatal("申覆期限前一秒不清")
	}
	f.now = deadline
	if purge() != 0 {
		t.Fatal("剛好是申覆期限不清（要嚴格超過）")
	}
	f.now = deadline.Add(time.Microsecond)
	if purge() != 1 || !f.purged(t, s.StoreID) {
		t.Fatal("超過申覆期限要清")
	}

	// 訂單拖過申覆期限才關店：關閉滿保留期才清。
	account3, s3 := f.open(t)
	late := f.terminate(t, s3.StoreID, GroundBankruptcy)
	f.orders.open = true
	f.effect(t, late)
	lateDeadline := late.StartsAt.Add(4416 * time.Hour)
	f.now = lateDeadline.Add(24 * time.Hour)
	f.orders.open = false
	f.effect(t, late)
	if f.storeRow(t, s3.StoreID).Status != Closed {
		t.Fatal("訂單結束後關店")
	}
	_ = account3
	closedLate := f.now
	f.now = closedLate.Add(StoreRetention)
	if purge() != 0 {
		t.Fatal("申覆期限之後才關店：關閉剛好保留期不清")
	}
	f.now = closedLate.Add(StoreRetention + time.Microsecond)
	if purge() != 1 || !f.purged(t, s3.StoreID) {
		t.Fatal("關閉滿保留期要清")
	}

	// 關店之後才生效的另一筆終止：保留期延到它自己的申覆期限；在那之前撤銷，就不再延長。
	_, s4, m4 := f.terminated(t, GroundBankruptcy)
	f.now = m4.StartsAt.Add(10 * 24 * time.Hour)
	extra := f.terminate(t, s4.StoreID, GroundFraud)
	extraDeadline := extra.StartsAt.Add(4416 * time.Hour)
	f.now = m4.StartsAt.Add(StoreRetention + time.Second)
	if !f.now.Before(extraDeadline) {
		t.Fatalf("前提：關店後的保留期先到期（%v），另一筆終止的申覆期限（%v）還沒到", f.now, extraDeadline)
	}
	if purge() != 0 || f.purged(t, s4.StoreID) {
		t.Fatal("關店後的保留期已過，但另一筆未撤銷終止的申覆期限未過：不清")
	}
	f.revoke(t, extra)
	f.now = f.now.Add(time.Second)
	if purge() != 1 || !f.purged(t, s4.StoreID) {
		t.Fatal("另一筆終止已撤銷：只剩關店後的保留期，已過，要清")
	}
}

// legal hold 看帳號：帳號上的終止有待處理的申覆時，該帳號「所有」已關閉的 stint（含賣家自己關閉的舊 stint）過期也不清、不發 StoreRetired；
// 裁定之後下一輪清理。關聯帳號的來源措施有待處理的申覆時，關聯帳號的 stint 也不清。
// mutation：hold 只看關店的那筆措施（不看帳號）→ 舊 stint 那條失敗；hold 不看來源措施 → 關聯那條失敗。
func TestLegalHoldCoversTheWholeAccountAndLinkedAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	retired := record[StoreRetired](f)
	// 帳號先有一個賣家自己關閉的舊 stint，之後開新店、被終止（NOTICE 結束後可重新申請，這裡用 21.3 直接建在新店上）。
	accountID, old := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(10 * 24 * time.Hour) // 終止晚於舊 stint 關店：終止關閉的新店保留期才晚於舊 stint 的
	second, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	m := f.terminate(t, second.StoreID, GroundOtherSerious)
	f.effect(t, m)
	f.now = f.now.Add(24 * time.Hour)
	f.appeal(t, accountID, f.measureOf(t, m.ID), "異議")
	f.now = closedAt.Add(StoreRetention + time.Hour)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 0 || f.purged(t, old.StoreID) || len(retired.all()) != 0 {
		t.Fatalf("申覆待處理：舊 stint 過期也不清（purged=%d）", n)
	}
	cur := f.measureOf(t, m.ID)
	_, err = f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestRejected, Reason: "不成立"})
	must(t, err)
	n, err = f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 || !f.purged(t, old.StoreID) || f.purged(t, second.StoreID) {
		t.Fatalf("裁定之後下一輪清理舊 stint（終止關閉的那個還在申覆期間內）：purged=%d", n)
	}
}

// Reopen 與清理用同一個判斷式：保留期已滿但清理 job 還沒跑時撤銷——不重開，只撤銷紀錄；之後清理照常。
// mutation：Reopen 改成只看「未清理」→ 這裡會重開。
func TestRevokingAfterTheRetentionButBeforeThePurgeJobDoesNotReopen(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, m := f.terminated(t, GroundCeasedBusiness)
	// 賣家自己關閉的對照：終止關店的保留期是申覆期限，所以在申覆期限之前撤銷仍重開（mutation「保留期用 30 天」會在這裡失敗）。
	f.now = m.StartsAt.Add(60 * 24 * time.Hour)
	other, s2, m2 := f.terminated(t, GroundCeasedBusiness)
	f.now = m2.StartsAt.Add(40 * 24 * time.Hour)
	f.revoke(t, m2)
	if back := f.storeRow(t, s2.StoreID); back.Status != Active || !f.isSeller(t, other) {
		t.Fatalf("申覆期限之前撤銷要重開：%+v", back)
	}
	// 保留期已滿、清理還沒跑。
	f.now = m.StartsAt.Add(4416*time.Hour + time.Hour)
	revoked := f.revoke(t, m)
	if revoked.Status != MeasureStatusRevoked {
		t.Fatalf("revoked=%+v", revoked)
	}
	if got := f.storeRow(t, s.StoreID); got.Status != Closed || f.isSeller(t, accountID) || f.storeClosures(t, s.StoreID) != 0 {
		t.Fatalf("不重開、只撤銷紀錄：%+v", got)
	}
	// 撤銷紀錄與禁止入駐：快照刪除、帳號可開新店（新 store_id）。
	if f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID) != 0 {
		t.Fatal("快照刪除")
	}
	n, err := f.module.PurgePII(context.Background())
	must(t, err)
	if n != 1 || !f.purged(t, s.StoreID) {
		t.Fatalf("之後清理照常：n=%d", n)
	}
	fresh, err := f.module.Apply(context.Background(), validApply(accountID))
	must(t, err)
	if fresh.StoreID == s.StoreID {
		t.Fatal("新店有新的 store_id")
	}
}

// 帳號被 identity 停權時撤銷終止措施只撤銷紀錄、不重開店（否則會有店且被停權）；解除停權後店家仍是關閉的。
// mutation：ReopenPlan 拿掉 account.Suspended 的判斷 → 這裡會重開。
func TestRevokingATerminationOfASuspendedAccountDoesNotReopen(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, m := f.terminated(t, GroundCeasedBusiness)
	_, _, err := f.identity.SuspendAccount(context.Background(), identity.SuspendInput{AccountID: accountID, Category: policy.Other, Reason: "停權"})
	must(t, err)
	revoked := f.revoke(t, m)
	if revoked.Status != MeasureStatusRevoked {
		t.Fatalf("revoked=%+v", revoked)
	}
	if got := f.storeRow(t, s.StoreID); got.Status != Closed || f.isSeller(t, accountID) || f.storeClosures(t, s.StoreID) != 0 {
		t.Fatalf("被停權的帳號不重開：%+v", got)
	}
}

// 申覆期間內撤銷（或接受申覆）：同一個 store_id、店號、商品與 KYC 欄位仍在、SELLER 重新授予、store_closures 有一列、可曝光恢復、沒有
// seller.deleted（StoreRetired）、PayUni 的綁定補排回去。
// mutation：Reopen 不重新授予 SELLER。
func TestRevokingATerminationInsideTheAppealWindowReopensTheSameStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	reopened, retired, announcements := record[StoreReopened](f), record[StoreRetired](f), record[StoreExposureChanged](f)
	accountID, s := f.exposable(t)
	before := f.storeRow(t, s.StoreID)
	f.jobs.args = nil
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Status != Closed {
		t.Fatal("應該已關店")
	}
	if exposable, _ := f.announced(t, s.StoreID); exposable {
		t.Fatal("關店之後不可曝光")
	}
	kycBefore := f.row(t, "kyc_applications", "store_id", s.StoreID)
	syncBefore := countKind(f.jobs.kinds(), "seller.payuni_sync")
	f.now = f.now.Add(40 * 24 * time.Hour)
	f.revoke(t, m)
	back := f.storeRow(t, s.StoreID)
	if back.Status != Active || back.Closure != nil || back.StoreCode != s.StoreCode || back.StoreName != before.StoreName ||
		back.LogoKey == nil || !f.isSeller(t, accountID) {
		t.Fatalf("back=%+v", back)
	}
	if kycAfter := f.row(t, "kyc_applications", "store_id", s.StoreID); kycAfter["real_name"] != kycBefore["real_name"] || kycAfter["id_card_hash"] == nil {
		t.Fatal("KYC 欄位仍在")
	}
	if exposable, _ := f.announced(t, s.StoreID); !exposable {
		t.Fatal("可曝光恢復（沒有其他停權）")
	}
	if len(reopened.all()) != 1 || len(retired.all()) != 0 || f.storeClosures(t, s.StoreID) != 1 {
		t.Fatalf("reopened=%d retired=%d closures=%d", len(reopened.all()), len(retired.all()), f.storeClosures(t, s.StoreID))
	}
	var closedBy string
	var closedMeasure uuid.UUID
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT closed_by, closed_by_measure_id FROM seller.store_closures WHERE store_id=$1 AND reopened_at IS NOT NULL", s.StoreID).Scan(&closedBy, &closedMeasure))
	if closedBy != "TERMINATION" || closedMeasure != m.ID {
		t.Fatalf("歷史：%s %s", closedBy, closedMeasure)
	}
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("撤銷之後帳號恢復")
	}
	if got := countKind(f.jobs.kinds(), "seller.payuni_sync"); got < syncBefore {
		t.Fatalf("sync=%d", got)
	}
	_ = announcements
}

func countKind(kinds []string, kind string) int {
	n := 0
	for _, k := range kinds {
		if k == kind {
			n++
		}
	}
	return n
}

// 恢復營業時補排 PayUni 的綁定：關店期間 sync job 對已關閉的店不做事、綁定意圖留在 PENDING，恢復後要重新排入。
func TestReopenRequeuesThePayUniBindThatWasDroppedWhileClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedWith(t, idA, bankAcct)
	if reg := f.payuniStatus(t, s.StoreID); reg != "PENDING" {
		t.Fatalf("bind_status=%s，應該是等綁定", reg)
	}
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.effect(t, m)
	f.sync(t, s.StoreID)
	if reg := f.payuniStatus(t, s.StoreID); reg != "PENDING" {
		t.Fatalf("已關閉的店不做 sync，綁定仍在等：%s", reg)
	}
	f.jobs.args = nil
	f.revoke(t, m)
	if countKind(f.jobs.kinds(), "seller.payuni_sync") != 1 {
		t.Fatalf("恢復營業後要補排一次 sync：%v", f.jobs.kinds())
	}
	f.sync(t, s.StoreID)
	if reg := f.payuniStatus(t, s.StoreID); reg != "BOUND" {
		t.Fatalf("恢復之後的 sync 要真的綁定：%s", reg)
	}
}

func (f *fixture) payuniStatus(t *testing.T, storeID int64) string {
	t.Helper()
	var status string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT bind_status FROM seller.payuni_registrations WHERE store_id=$1", storeID).Scan(&status))
	return status
}

// 帳號已開新店（21.2 允許）時撤銷：只撤銷、不重開；帳號在等待刪除時也不重開。
func TestRevokeDoesNotReopenWhenTheAccountHasANewStoreOrIsLeaving(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	f.now = notice.StartsAt
	f.effect(t, notice)
	if f.storeRow(t, s.StoreID).Status != Closed {
		t.Fatal("預告終止生效、沒有未結訂單：關店")
	}
	fresh, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	f.revoke(t, notice)
	if f.storeRow(t, s.StoreID).Status != Closed || f.storeRow(t, fresh.StoreID).Status != Active || f.storeClosures(t, s.StoreID) != 0 {
		t.Fatal("已開新店：只撤銷、不重開")
	}
	if got := f.measureOf(t, notice.ID); got.Status != MeasureStatusRevoked {
		t.Fatalf("got=%+v", got)
	}
}

// 申覆期間後撤銷 21.3：個資清理之後身分快照仍在（禁止入駐的依據，不受清理影響），撤銷之後快照刪除、帳號可以開新店（新的 store_id）。
// mutation：清理順手刪快照。
func TestRevokingTheTerminationAfterThePurgeLetsTheAccountOpenANewStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	m := f.terminate(t, s.StoreID, GroundFraud)
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Status != Closed || f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID) == 0 {
		t.Fatal("終止關店，並取了身分快照")
	}
	f.now = m.StartsAt.Add(4416*time.Hour + 24*time.Hour)
	n, err := f.module.PurgePII(context.Background())
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE store_id=$1 AND id_card_hash IS NOT NULL", s.StoreID) != 0 {
		t.Fatal("KYC 的 hash 隨個資清理")
	}
	if f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID) == 0 {
		t.Fatal("個資清理之後身分快照仍在")
	}
	if _, err := f.module.Apply(context.Background(), validApply(accountID)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("撤銷之前仍禁止入駐：%v", err)
	}
	f.revoke(t, m)
	if f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID) != 0 {
		t.Fatal("撤銷之後快照刪除")
	}
	fresh, err := f.module.Apply(context.Background(), validApply(accountID))
	must(t, err)
	if fresh.StoreID == s.StoreID {
		t.Fatal("新 store_id")
	}
}

// 關聯帳號（W9）：admin 的措施詳情列出同身分的其他帳號（不列本帳號、不列未命中者）；建立關聯措施的各種拒絕；來源撤銷時關聯措施在同一個
// transaction 一併撤銷（RevokeMeasure、DecideAppeal(ACCEPTED)），待處理請求以 CLOSED 結案，申覆期間內的關聯店家重開；關聯措施自己的申覆成立只撤銷它。
// mutation：連帶撤銷移到 transaction 之後；只在 RevokeMeasure 觸發；反向連動。
func TestLinkedAccountsListCreateAndCascade(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	_, sC, _ := f.submitted(t, idC, "555444333222")
	source := f.terminate(t, sA.StoreID, GroundFraud)

	detail := f.measureOf(t, source.ID)
	if len(detail.LinkedAccounts) != 1 {
		t.Fatalf("linked=%+v", detail.LinkedAccounts)
	}
	l := detail.LinkedAccounts[0]
	if l.AccountID != accountB || l.StoreID != sB.StoreID || l.StoreCode != sB.StoreCode || l.Matched != LinkedNationalID || l.LinkedMeasureID != nil {
		t.Fatalf("linked=%+v", l)
	}
	for _, other := range detail.LinkedAccounts {
		if other.AccountID == accountA || other.StoreID == sC.StoreID {
			t.Fatalf("不列本帳號與未命中者：%+v", other)
		}
	}
	linkedInput := func(storeID int64, src uuid.UUID) func(*CreateMeasureInput) {
		return func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src }
	}
	// 拒絕：來源不存在／同帳號／目標未命中／預告終止當來源。
	if _, err := f.createTermination(sB.StoreID, GroundLinkedAccount, linkedInput(sB.StoreID, uuid.NewV7())); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不存在的來源：%v", err)
	}
	if _, err := f.createTermination(sA.StoreID, GroundLinkedAccount, linkedInput(sA.StoreID, source.ID)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("來源在同一個帳號：%v", err)
	}
	if _, err := f.createTermination(sC.StoreID, GroundLinkedAccount, linkedInput(sC.StoreID, source.ID)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("目標沒有命中來源的身分快照：%v", err)
	}
	_, sN := f.open(t)
	notice := f.terminate(t, sN.StoreID, GroundNotice)
	if _, err := f.createTermination(sB.StoreID, GroundLinkedAccount, linkedInput(sB.StoreID, notice.ID)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("預告終止不能當來源：%v", err)
	}
	// 建立。
	linked, err := f.createTermination(sB.StoreID, GroundLinkedAccount, linkedInput(sB.StoreID, source.ID))
	must(t, err)
	lm := linked.Measure
	if lm.Basis != BasisContract || lm.SourceMeasureID == nil || *lm.SourceMeasureID != source.ID || lm.State != StateActive || *lm.TerminationGround != GroundLinkedAccount {
		t.Fatalf("lm=%+v", lm)
	}
	if f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", lm.ID) == 0 {
		t.Fatal("關聯終止取身分快照")
	}
	if _, err := f.createTermination(sB.StoreID, GroundLinkedAccount, linkedInput(sB.StoreID, source.ID)); apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("同來源同帳號第二筆：%v", err)
	}
	if got := f.measureOf(t, source.ID).LinkedAccounts; len(got) != 1 || got[0].LinkedMeasureID == nil || *got[0].LinkedMeasureID != lm.ID {
		t.Fatalf("linked_measure_id：%+v", got)
	}
	// 不成鏈：關聯終止不能當來源。
	if _, err := f.createTermination(sA.StoreID, GroundLinkedAccount, linkedInput(sA.StoreID, lm.ID)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("關聯終止不能當來源：%v", err)
	}

	// 關聯措施自己申覆成立：只撤銷它，來源不變。
	f.effect(t, f.measureOf(t, lm.ID))
	f.appeal(t, accountB, f.measureOf(t, lm.ID), "我不是同一人")
	cur := f.measureOf(t, lm.ID)
	_, err = f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: lm.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestAccepted, Reason: "查證"})
	must(t, err)
	if f.measureOf(t, lm.ID).Status != MeasureStatusRevoked || f.measureOf(t, source.ID).Status != MeasureStatusEnforced || f.storeRow(t, sB.StoreID).Status != Active {
		t.Fatal("關聯措施撤銷不影響來源、店家重開")
	}
	_ = accountA
}

// 來源撤銷的三條路徑都在同一個 transaction 連帶撤銷關聯終止（SOURCE_REVOKED），待處理請求 CLOSED，申覆期間內的關聯店家重開。
func TestSourceRevocationCascadesInTheSameTransactionOnEveryPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"revoke", "appeal", "finding"} {
		t.Run(path, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			revoked := record[MeasureRevoked](f)
			accountA, sA, _ := f.submitted(t, idA, bankAcct)
			accountB, sB, _ := f.submitted(t, idA, "999888777666")
			var source Measure
			var finding Measure
			switch path {
			case "finding":
				finding = f.violate(t, sA.StoreID, SeverityLow, MeasureWarning, nil)
				f.now = f.now.Add(8 * 24 * time.Hour)
				res, err := f.createTermination(sA.StoreID, GroundMaterialBreach, func(in *CreateMeasureInput) {
					in.Termination.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{finding.Findings[0].ID}}
				})
				must(t, err)
				source = res.Measure
			default:
				source = f.terminate(t, sA.StoreID, GroundFraud)
			}
			res, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
			must(t, err)
			lm := res.Measure
			f.effect(t, source)
			f.effect(t, lm)
			if f.storeRow(t, sA.StoreID).Status != Closed || f.storeRow(t, sB.StoreID).Status != Closed {
				t.Fatal("兩家都因終止而關店")
			}
			f.appeal(t, accountB, f.measureOf(t, lm.ID), "異議")
			cur := f.measureOf(t, source.ID)
			switch path {
			case "revoke":
				_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: source.ID, AdminID: 9, ExpectedVersion: cur.Version, Reason: RevokeOther})
			case "appeal":
				f.appeal(t, accountA, cur, "異議")
				cur = f.measureOf(t, source.ID)
				_, err = f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: source.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestAccepted, Reason: "成立"})
			case "finding":
				origin := f.measureOf(t, finding.ID)
				_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: origin.ID, AdminID: 9, ExpectedVersion: origin.Version, Reason: RevokeOther, RevokeFindings: true})
			}
			must(t, err)
			after := f.measureOf(t, lm.ID)
			if after.Status != MeasureStatusRevoked || after.RevokedReason == nil || *after.RevokedReason != RevokeSourceRevoked {
				t.Fatalf("關聯終止要連帶撤銷：%+v", after)
			}
			if len(after.Requests) != 1 || after.Requests[0].State != RequestClosed || after.Requests[0].CloseCause == nil || *after.Requests[0].CloseCause != CloseCauseRevoked {
				t.Fatalf("待處理請求以 CLOSED 結案：%+v", after.Requests)
			}
			if f.measureOf(t, source.ID).Status != MeasureStatusRevoked {
				t.Fatal("來源撤銷")
			}
			for _, id := range []int64{sA.StoreID, sB.StoreID} {
				if got := f.storeRow(t, id); got.Status != Active || got.Closure != nil {
					t.Fatalf("申覆期間內兩家店都重開：%+v", got)
				}
			}
			reasons := map[uuid.UUID]RevokeReason{}
			for _, e := range revoked.all() {
				reasons[e.MeasureID] = e.Reason
			}
			if reasons[lm.ID] != RevokeSourceRevoked {
				t.Fatalf("MeasureRevoked 帶原因：%+v", reasons)
			}
		})
	}
}

// 來源的申覆待處理時，關聯帳號的已關閉 stint 也不清理（legal hold 看來源措施）。
func TestLinkedAccountStoresAreHeldWhileTheSourceHasAPendingAppeal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	res, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
	must(t, err)
	// B 先自己關店（賣家自己關閉，保留期 184 天）。
	_, err = f.module.CloseCurrent(ctx, accountB)
	must(t, err)
	f.effect(t, source)
	f.now = f.now.Add(24 * time.Hour)
	f.appeal(t, accountA, f.measureOf(t, source.ID), "異議")
	f.now = f.now.Add(60 * 24 * time.Hour)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if f.purged(t, sB.StoreID) || n != 0 {
		t.Fatalf("來源的申覆待處理：關聯帳號的 stint 也不清（n=%d）", n)
	}
	_ = res
}

// 鎖：來源與關聯帳號並發撤銷與建立，重複多次沒有 deadlock（壓力測試；不當 mutation 斷言）。
func TestConcurrentSourceRevokeAndLinkedCreateNeverDeadlock(t *testing.T) {
	t.Parallel()
	const rounds = 200
	f := setup(t)
	ctx := context.Background()
	deadlocks := 0
	for i := 0; i < rounds; i++ {
		_, sA, _ := f.submitted(t, idA, bankAcct)
		_, sB, _ := f.submitted(t, idA, "999888777666")
		source := f.terminate(t, sA.StoreID, GroundFraud)
		cur := f.measureOf(t, source.ID)
		done := make(chan error, 2)
		go func() {
			_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: source.ID, AdminID: 9, ExpectedVersion: cur.Version, Reason: RevokeOther})
			done <- err
		}()
		go func() {
			_, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
			done <- err
		}()
		for range 2 {
			if err := <-done; err != nil && (strings.Contains(err.Error(), "40P01") || strings.Contains(err.Error(), "deadlock")) {
				deadlocks++
				t.Errorf("round %d: %v", i, err)
			}
		}
	}
	if deadlocks != 0 {
		t.Fatalf("deadlocks=%d", deadlocks)
	}
}

// 鎖：並發的「建立措施」與「關店」在同一個有兩個 stint 的帳號上重複 200 次，沒有 deadlock（先鎖帳號全部的 stint 再寫措施；改回 LockStore(current) 時預期
// 出現 40P01，非確定性，不當 mutation 斷言）。
func TestConcurrentMeasureCreationAndCloseOnAnAccountWithTwoStintsNeverDeadlock(t *testing.T) {
	t.Parallel()
	const rounds = 200
	f := setup(t)
	ctx := context.Background()
	for i := 0; i < rounds; i++ {
		accountID, first := f.open(t)
		_, err := f.module.CloseCurrent(ctx, accountID)
		must(t, err)
		second, err := f.module.Apply(ctx, validApply(accountID))
		must(t, err)
		done := make(chan error, 2)
		go func() {
			_, err := f.module.CloseCurrent(ctx, accountID)
			done <- err
		}()
		go func() {
			_, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
				AdminID: 7, StoreID: second.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
				Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}},
			})
			done <- err
		}()
		for range 2 {
			if err := <-done; err != nil && (strings.Contains(err.Error(), "40P01") || strings.Contains(err.Error(), "deadlock")) {
				t.Errorf("round %d: %v", i, err)
			}
		}
		_ = first
	}
}

// 21.3 之後的清單與篩選：SCHEDULED 篩選只含預定生效的；ENDED 不含 SCHEDULED（預告期間內因店家關閉而結束的是 ENDED，不在 SCHEDULED 清單）。
func TestSchedulingAndEndingAreSeparatedInTheMeasureLists(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	list := func(state MeasureState) []uuid.UUID {
		page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{AccountID: accountID, State: &state, PageSize: 50})
		must(t, err)
		var out []uuid.UUID
		for _, m := range page.Items {
			out = append(out, m.ID)
		}
		return out
	}
	if got := list(StateScheduled); !slices.Equal(got, []uuid.UUID{notice.ID}) || len(list(StateEnded)) != 0 || len(list(StateActive)) != 0 {
		t.Fatalf("scheduled=%v ended=%v active=%v", got, list(StateEnded), list(StateActive))
	}
	f.now = f.now.Add(24 * time.Hour)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if len(list(StateScheduled)) != 0 || !slices.Equal(list(StateEnded), []uuid.UUID{notice.ID}) || len(list(StateActive)) != 0 {
		t.Fatalf("關店之後：scheduled=%v ended=%v active=%v", list(StateScheduled), list(StateEnded), list(StateActive))
	}
	if page, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, State: ptr(StateEnded)}); err != nil || len(page.Items) != 1 {
		t.Fatalf("賣家的清單：%+v err=%v", page, err)
	}
}

// 終止合作的資料集：逐帳號的 EffectiveMeasures、全部帳號的 EffectiveMeasuresAll、ScheduledMeasureIDs 與 model.StateOf 對同一份資料回傳同一批狀態，
// 並且與人工標註相等（開始時間剛好是 now 與 ±1µs、預告期間內因店家關閉而結束的措施——結束時間早於開始時間——是 ENDED，不在 SCHEDULED 的清單）。
// mutation：StateOf 把 SCHEDULED 的判斷移到 ENDED 之前；ScheduledMeasureIDs 拿掉 ends_at 的條件。
func TestTerminationShapesAgreeAcrossStateOfAndTheStateQueries(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	now := f.now
	us := time.Microsecond
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	type shape struct {
		name    string
		kind    string
		ground  string
		created time.Time
		ends    *time.Time
		revoked bool
		want    MeasureState
	}
	notice := 720 * time.Hour
	shapes := []shape{
		{"預告：尚未開始", "TERMINATION", "NOTICE", now.Add(-time.Hour), nil, false, StateScheduled},
		{"預告：剛好開始", "TERMINATION", "NOTICE", now.Add(-notice), nil, false, StateActive},
		{"預告：晚 1µs 才開始", "TERMINATION", "NOTICE", now.Add(-notice + us), nil, false, StateScheduled},
		{"預告：早 1µs 已開始", "TERMINATION", "NOTICE", now.Add(-notice - us), nil, false, StateActive},
		{"預告期間內關店（結束早於開始）", "TERMINATION", "NOTICE", now.Add(-time.Hour), at(-30 * time.Minute), false, StateEnded},
		{"預告期間內關店（結束剛好是 now）", "TERMINATION", "NOTICE", now.Add(-time.Hour), at(0), false, StateEnded},
		{"預告期間內關店（結束晚 now 1µs）", "TERMINATION", "NOTICE", now.Add(-time.Hour), at(us), false, StateScheduled},
		{"預告：生效後關店", "TERMINATION", "NOTICE", now.Add(-notice - time.Hour), at(-time.Minute), false, StateEnded},
		{"21.3 永久", "TERMINATION", "CEASED_BUSINESS", now.Add(-5 * 24 * time.Hour), nil, false, StateActive},
		{"21.3 剛建立", "TERMINATION", "FRAUD", now, nil, false, StateActive},
		{"21.3 撤銷", "TERMINATION", "FRAUD", now.Add(-time.Hour), nil, true, StateRevoked},
		{"預告撤銷", "TERMINATION", "NOTICE", now.Add(-time.Hour), nil, true, StateRevoked},
		{"停權：晚 1µs 才開始", "SUSPENSION", "", now.Add(us), at(time.Hour), false, StateScheduled},
		{"停權：剛好開始", "SUSPENSION", "", now, at(time.Hour), false, StateActive},
	}
	ids := make([]uuid.UUID, len(shapes))
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		for i, sh := range shapes {
			ids[i] = uuid.NewV7()
			account := int64(200000 + i)
			var storeID int64
			if err := c.QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, 'x', $3, $3) RETURNING store_id`,
				3100000000+int64(i), account, now).Scan(&storeID); err != nil {
				return err
			}
			starts := sh.created
			if sh.ground == "NOTICE" {
				starts = sh.created.Add(notice)
			}
			status, reason, by, revokedAt := "ENFORCED", any(nil), any(nil), any(nil)
			if sh.revoked {
				status, reason, by, revokedAt = "REVOKED", "OTHER", int64(1), now.Add(-time.Minute)
			}
			basis, risk, review := "CONTRACT", any(nil), any(nil)
			if sh.kind == "SUSPENSION" {
				basis, risk, review = "RISK", "FRAUD", sh.created.Add(24*time.Hour)
			}
			if _, err := c.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, termination_ground, statement_to_seller, ends_at, status,
					current_revision, created_by_admin_id, created_at, updated_at, starts_at, revoked_reason, revoked_by_admin_id, revoked_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), 'x', $9, $10, 1, 1, $11, $11, $12, $13, $14, $15)`,
				ids[i], account, storeID, sh.kind, basis, risk, review, sh.ground, sh.ends, status, sh.created, starts, reason, by, revokedAt); err != nil {
				return err
			}
			if _, err := c.Exec(ctx, `INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at) VALUES ($1, 1, 'x', $2, 1, $3)`,
				ids[i], sh.ends, sh.created); err != nil {
				return err
			}
		}
		return nil
	}))
	q := store.New(f.db.Conn(ctx))
	var accounts []int64
	for i := range shapes {
		accounts = append(accounts, int64(200000+i))
	}
	perAccount := map[uuid.UUID]bool{}
	byAccount, err := stint.EffectiveMany(ctx, q, accounts, now)
	must(t, err)
	for _, s := range byAccount {
		for _, id := range s.IDs() {
			perAccount[id] = true
		}
	}
	all := map[uuid.UUID]bool{}
	rows, err := q.EffectiveMeasuresAll(ctx, now)
	must(t, err)
	for _, r := range rows {
		all[r.MeasureID] = true
	}
	scheduledIDs, err := stint.ScheduledMeasureIDs(ctx, q, 0, now)
	must(t, err)
	scheduled := map[uuid.UUID]bool{}
	for _, id := range scheduledIDs {
		scheduled[id] = true
	}
	for i, sh := range shapes {
		var row store.SellerMeasure
		stored, err := q.MeasureByID(ctx, ids[i])
		must(t, err)
		row = stored
		got := model.StateOf(model.MeasureStatus(row.Status), model.MeasureKind(row.Kind), row.StartsAt, row.EndsAt, now)
		if got != sh.want {
			t.Errorf("%s：StateOf=%s want=%s", sh.name, got, sh.want)
		}
		wantActive, wantScheduled := sh.want == StateActive, sh.want == StateScheduled
		if perAccount[ids[i]] != wantActive || all[ids[i]] != wantActive {
			t.Errorf("%s：EffectiveMeasures=%v EffectiveMeasuresAll=%v want active=%v", sh.name, perAccount[ids[i]], all[ids[i]], wantActive)
		}
		if scheduled[ids[i]] != wantScheduled {
			t.Errorf("%s：ScheduledMeasureIDs=%v want=%v", sh.name, scheduled[ids[i]], wantScheduled)
		}
	}
	// ENDED 的篩選是「不在生效中也不在預定生效」。
	state := StateEnded
	page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{State: &state, PageSize: 100})
	must(t, err)
	endedWant := map[uuid.UUID]bool{}
	for i, sh := range shapes {
		if sh.want == StateEnded {
			endedWant[ids[i]] = true
		}
	}
	endedGot := map[uuid.UUID]bool{}
	for _, m := range page.Items {
		endedGot[m.ID] = true
	}
	if len(endedGot) != len(endedWant) {
		t.Errorf("ENDED 清單=%d 筆，預期 %d", len(endedGot), len(endedWant))
	}
	for id := range endedWant {
		if !endedGot[id] {
			t.Errorf("ENDED 清單缺 %s", id)
		}
	}
	f.assertCurrentValuesMatchHistory(t)
}

// 關聯帳號的比對用金鑰環的每一把金鑰重算，不比申請上已存的 hash：HMAC 輪替之後（新的金鑰在前、舊的留著），不論快照或對方的申請是哪一把金鑰寫的，
// 同身分的帳號都列得出來、都能建立關聯終止。mutation：LinkedAccountHits 只用第一把金鑰（s.p.hashes(input)[:1]）或回到比對存的 hash → 兩個方向都失敗。
func TestLinkedAccountsAreFoundAcrossHMACKeyRotation(t *testing.T) {
	t.Parallel()
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	oldEnc, oldMAC := "test1:"+b64("test-kyc-encryption-key-00000001"), "test1:"+b64("test-kyc-hash-hmac-key-000000001")
	newEnc, newMAC := "rot2:"+b64("rotated-kyc-encryption-key-00002"), "rot2:"+b64("rotated-kyc-hash-hmac-key-0000002")
	for name, sourceFirst := range map[string]bool{"快照用舊金鑰、對方的申請用新金鑰": true, "快照用新金鑰、對方的申請用舊金鑰": false} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			rotate := func() {
				enc, err := seal.Parse(newEnc + "," + oldEnc)
				must(t, err)
				mac, err := seal.ParseMAC(newMAC + "," + oldMAC)
				must(t, err)
				f.module = f.newModuleWithKeys(t, f.payuni, f.jobs, enc, mac)
			}
			var sA, sB Store
			var accountB int64
			if sourceFirst {
				_, sA, _ = f.submitted(t, idA, bankAcct)
				rotate()
				accountB, sB, _ = f.submitted(t, idA, "999888777666")
			} else {
				_, sB0, _ := f.submitted(t, idA, "999888777666")
				sB = sB0
				rotate()
				_, sA, _ = f.submitted(t, idA, bankAcct)
				accountB = sB.AccountID
			}
			var hashKeys []string
			rows, err := f.db.Conn(context.Background()).Query(context.Background(), "SELECT DISTINCT id_card_hash_key FROM seller.kyc_applications ORDER BY 1")
			must(t, err)
			for rows.Next() {
				var k string
				must(t, rows.Scan(&k))
				hashKeys = append(hashKeys, k)
			}
			must(t, rows.Err())
			if len(hashKeys) != 2 {
				t.Fatalf("兩筆申請要由不同的金鑰寫入：%v", hashKeys)
			}
			source := f.terminate(t, sA.StoreID, GroundFraud)
			linked := f.measureOf(t, source.ID).LinkedAccounts
			if len(linked) != 1 || linked[0].AccountID != accountB || linked[0].Matched != LinkedNationalID {
				t.Fatalf("輪替後仍要列出同身分的帳號：%+v", linked)
			}
			if _, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID }); err != nil {
				t.Fatalf("輪替後建立關聯終止：%v", err)
			}
		})
	}
}

// 來源措施已撤銷：不能再當關聯終止的來源（invalid_argument，指 source_measure_id，原因是來源已撤銷，不是身分快照沒有相同）。
// mutation：拿掉來源狀態的檢查 → 錯誤原因變成「沒有相同」，訊息斷言失敗。
func TestLinkedTerminationRejectsARevokedSource(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, sA, _ := f.submitted(t, idA, bankAcct)
	_, sB, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	f.revoke(t, source)
	_, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
	if apperr.KindOf(err) != apperr.KindInvalidInput || !strings.Contains(err.Error(), "來源措施已撤銷") {
		t.Fatalf("已撤銷的來源：%v", err)
	}
}

// 連帶撤銷與來源的撤銷在同一個 transaction：關聯終止撤銷時的事件處理失敗（事件在 transaction 內送出），整筆回滾，來源與關聯終止都還是生效中；
// 排除失敗後重試，兩者一起撤銷。mutation：連帶撤銷的錯誤被吞掉或移到來源 commit 之後 → 第一次呼叫沒有回錯或來源已被撤銷，斷言失敗。
func TestSourceRevocationRollsBackTogetherWithTheCascade(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"revoke", "appeal"} {
		t.Run(path, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			accountA, sA, _ := f.submitted(t, idA, bankAcct)
			accountB, sB, _ := f.submitted(t, idA, "999888777666")
			source := f.terminate(t, sA.StoreID, GroundFraud)
			res, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
			must(t, err)
			lm := res.Measure
			f.effect(t, source)
			f.effect(t, lm)
			f.appeal(t, accountB, f.measureOf(t, lm.ID), "異議")
			if path == "appeal" {
				f.appeal(t, accountA, f.measureOf(t, source.ID), "異議")
			}
			var fail atomic.Bool
			fail.Store(true)
			events.Subscribe(f.bus, func(_ context.Context, e MeasureRevoked) error {
				if fail.Load() && e.MeasureID == lm.ID {
					return errors.New("注入的失敗")
				}
				return nil
			})
			run := func() error {
				cur := f.measureOf(t, source.ID)
				if path == "appeal" {
					_, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: source.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestAccepted, Reason: "成立"})
					return err
				}
				_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: source.ID, AdminID: 9, ExpectedVersion: cur.Version, Reason: RevokeOther})
				return err
			}
			if err := run(); err == nil {
				t.Fatal("連帶撤銷失敗時，來源的撤銷要回錯")
			}
			if got := f.measureOf(t, source.ID); got.Status != MeasureStatusEnforced {
				t.Fatalf("來源要回滾：%+v", got)
			}
			after := f.measureOf(t, lm.ID)
			if after.Status != MeasureStatusEnforced || len(after.Requests) != 1 || after.Requests[0].State != RequestPending {
				t.Fatalf("關聯終止與它的申覆要回滾：%+v", after)
			}
			if f.storeRow(t, sA.StoreID).Status != Closed || f.storeRow(t, sB.StoreID).Status != Closed {
				t.Fatal("店家不該被恢復")
			}
			fail.Store(false)
			must(t, run())
			if f.measureOf(t, source.ID).Status != MeasureStatusRevoked || f.measureOf(t, lm.ID).Status != MeasureStatusRevoked {
				t.Fatal("排除失敗後重試：兩者一起撤銷")
			}
		})
	}
}

// legal hold 只看未撤銷的終止合作：關聯終止已被撤銷（自己的撤銷，來源不受影響），來源上的待處理申覆不再暫停該帳號已關閉 stint 的清理；
// 來源所在的帳號自己的申覆仍然暫停。mutation：RetireDueStores 拿掉 tm.status = 'ENFORCED' → 關聯帳號被暫停，斷言失敗。
func TestRevokedLinkedTerminationDoesNotHoldThePurge(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	res, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
	must(t, err)
	_, err = f.module.CloseCurrent(ctx, accountB)
	must(t, err)
	closedAt := f.now
	f.effect(t, source)
	f.revoke(t, res.Measure)
	f.now = f.now.Add(24 * time.Hour)
	f.appeal(t, accountA, f.measureOf(t, source.ID), "異議")
	f.now = closedAt.Add(StoreRetention + time.Hour)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if !f.purged(t, sB.StoreID) || f.purged(t, sA.StoreID) || n != 1 {
		t.Fatalf("B 的關聯終止已撤銷：B 要清、A 的申覆待處理不清（n=%d）", n)
	}
}

// measures 的結束時間一定晚於建立時間（T5a：預告終止在關店時結束，關店晚於建立）；資料庫層擋下 ends_at <= created_at。
// mutation：拿掉 measures_ends_check → 更新成功，斷言失敗。
func TestMeasureEndsAfterItIsCreated(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	m := f.suspend(t, s.StoreID)
	_, err := f.db.Conn(context.Background()).Exec(context.Background(), "UPDATE seller.measures SET ends_at = created_at WHERE id = $1", m.ID)
	if err == nil || !strings.Contains(err.Error(), "measures_ends_check") {
		t.Fatalf("ends_at = created_at 要被 CHECK 擋下：%v", err)
	}
}
