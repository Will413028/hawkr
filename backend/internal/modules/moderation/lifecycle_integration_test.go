//go:build integration

package moderation

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) deleteComment(t *testing.T, buyerID int64, id uuid.UUID) {
	t.Helper()
	must(t, f.social.DeleteComment(context.Background(), social.Caller{BuyerID: buyerID}, id, 0))
}

// 作者刪掉被檢舉的內容不能躲過警告與停權：處置用案件快照裡凍結的作者。
func TestWarnAndBanUseTheFrozenAuthorWhenTheContentIsGone(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()

	warned := f.comment(t, w.post.ID, w.author, "會被警告的留言")
	warnCase := f.openCase(t, commentTarget(warned))
	f.deleteComment(t, w.author, warned.ID)
	mustV[CaseDetail](t)(f.decide(t, warnCase, ActionWarn, policy.Spam))
	page := mustV[social.SanctionPage](t)(f.social.MySanctions(ctx, w.author, nil, 10))
	if len(page.Sanctions) != 1 || page.Sanctions[0].Kind != social.SanctionWarning || page.Sanctions[0].SourceRef != SourceRefPrefix+warnCase.ID.String() {
		t.Fatalf("內容刪掉之後警告仍要落在作者身上：%+v", page)
	}

	banned := f.comment(t, w.post.ID, w.author, "會被停權的留言")
	banCase := f.openCase(t, commentTarget(banned))
	f.deleteComment(t, w.author, banned.ID)
	mustV[CaseDetail](t)(f.decide(t, banCase, ActionBan, policy.Fraud))
	if !f.suspended(t, w.authorAccount) {
		t.Fatal("內容刪掉之後停權仍要生效")
	}
}

// 重送回傳既有的檢舉，不管內容現在還看不看得到；別的檢舉人看不到被隱藏的內容，仍是 NotFound。
func TestResendAfterTheContentIsHiddenReturnsTheExistingReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	first := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
	mustV[social.Hide](t)(f.social.Hide(context.Background(), social.HideInput{
		Target: social.Target{Kind: social.TargetComment, ID: w.comment.ID}, Category: string(policy.Spam), Text: "隱藏", SourceRef: "test", AdminID: 9,
	}))
	again, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam})
	if err != nil || again.Created || again.Report.ID != first.Report.ID {
		t.Fatalf("內容被隱藏之後重送要回傳既有的檢舉：%+v %v", again, err)
	}
	other, _ := f.newBuyer(t)
	if _, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: other, Target: commentTarget(w.comment), Category: policy.Spam}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("別的檢舉人看不到被隱藏的內容：%v", err)
	}
}

// 新檢舉掛進案件讓版本加一：管理員讀到版本之後又有人檢舉，用舊版本結案回 VersionConflict，處置整個回滾；用新版本才成功，而且認定引用的來源檢舉含新的那筆。
func TestAReportAttachedAfterTheAdminReadMakesTheCloseConflict(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	r1, _ := f.newBuyer(t)
	r2, _ := f.newBuyer(t)
	read := f.submit(t, r1, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	if read.Version != 1 {
		t.Fatalf("開案的版本 %d，預期 1", read.Version)
	}
	later := f.submit(t, r2, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo)
	if later.Case.ID != read.ID || later.Case.Version != 2 {
		t.Fatalf("掛進同一個案件、版本加一：%+v", later.Case)
	}
	in := DecideInput{
		CaseID: read.ID, Version: read.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "不實", Severity: string(seller.SeverityValues()[0]),
	}
	if _, err := f.module.DecideCase(context.Background(), in); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本結案要 VersionConflict：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 || !f.readable(t, social.TargetPost, w.post.ID) {
		t.Fatalf("處置要回滾：措施 %d 個", n)
	}
	if mustV[CaseDetail](t)(f.module.Case(context.Background(), read.ID)).Case.ClosedAt != nil {
		t.Fatal("衝突之後案件仍是進行中")
	}
	in.Version = later.Case.Version
	mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), in))
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 2 {
		t.Fatalf("認定引用的來源檢舉 %d 筆，預期 2（含後來掛進來的）", n)
	}
}

// 來源檢舉最多引用最早的 20 筆，備註寫真正的檢舉數。
func TestTakedownNoteCountsEveryReportButCitesTheEarliestTwenty(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var c Case
	for range 25 {
		r, _ := f.newBuyer(t)
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "不實", Severity: string(seller.SeverityValues()[0]),
	}))
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 20 {
		t.Fatalf("引用 %d 筆，預期上限 20", n)
	}
	var note string
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT n.body FROM seller.governance_notes n JOIN seller.measures m ON m.id = n.measure_id WHERE m.store_id = $1", w.storeID).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note, "25 筆") {
		t.Fatalf("備註要寫真正的檢舉數 25：%q", note)
	}
}

// B1-D20 證據保全：快照裡的圖片是 moderation 自己確認的複本；擁有者清除貼文（MarkDeleted 來源圖片）之後複本仍是 ACTIVE，快照刪除之後複本也不被 moderation 刪除。
func TestASnapshotKeepsItsOwnConfirmedCopyOfTheImage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	var source uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT image_asset_id FROM social.posts WHERE id = $1", w.post.ID).Scan(&source); err != nil {
		t.Fatal(err)
	}

	reporter, _ := f.newBuyer(t)
	r := f.submit(t, reporter, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo)
	snap := *r.Report.SnapshotID
	var owned []uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT asset_ids FROM moderation.case_snapshots WHERE id = $1", snap).Scan(&owned); err != nil {
		t.Fatal(err)
	}
	if len(owned) != 1 || owned[0] == source {
		t.Fatalf("快照要擁有一份不同於來源的複本：來源 %v，快照的圖片 %v", source, owned)
	}
	var content string
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT content::text FROM moderation.case_snapshots WHERE id = $1", snap).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, owned[0].String()) || strings.Contains(content, source.String()) {
		t.Fatalf("快照內容的圖片參照要換成複本 %v、不留來源 %v：%s", owned[0], source, content)
	}
	if st := f.assetState(t, owned[0]); st != "ACTIVE" {
		t.Fatalf("複本在送出的 transaction 內確認，狀態 %s，預期 ACTIVE", st)
	}

	must(t, f.social.PurgeStoreContent(ctx, w.storeID))
	if st := f.assetState(t, source); st == "ACTIVE" {
		t.Fatal("擁有者清除貼文之後來源圖片要被標記刪除")
	}
	if st := f.assetState(t, owned[0]); st != "ACTIVE" {
		t.Fatalf("擁有者刪圖之後快照的複本狀態 %s，預期仍是 ACTIVE", st)
	}

	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: r.Case.ID, Version: r.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}))
	f.advance(SnapshotRetention + time.Hour)
	if n := mustV[int64](t)(f.module.PurgeSnapshots(ctx)); n != 1 {
		t.Fatalf("刪了 %d 份快照", n)
	}
	if st := f.assetState(t, owned[0]); st == "ACTIVE" {
		t.Fatalf("快照過保留期被刪除之後複本要被標記刪除，狀態 %s", st)
	}
}

// 被檢舉買家的頭像與其他證據圖片同一條保全路徑：快照擁有自己的複本，買家換頭像之後複本仍是 ACTIVE，快照過保留期才標記刪除。
func TestASnapshotKeepsItsOwnCopyOfTheReportedBuyersAvatar(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	target, targetAccount := f.newBuyer(t)
	p, err := f.buyer.Profile(ctx, targetAccount)
	must(t, err)
	token := f.stage(t)
	_, err = f.buyer.UpdateProfile(ctx, buyer.UpdateProfileInput{BuyerID: target, ExpectedVersion: p.Version, AvatarStagedToken: &token})
	must(t, err)
	var source uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT avatar_asset_id FROM buyer.buyers WHERE buyer_id = $1", target).Scan(&source); err != nil {
		t.Fatal(err)
	}

	reporter, _ := f.newBuyer(t)
	r := f.submit(t, reporter, Target{Kind: TargetBuyer, BuyerID: target}, policy.Spam)
	snap := *r.Report.SnapshotID
	var owned []uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT asset_ids FROM moderation.case_snapshots WHERE id = $1", snap).Scan(&owned); err != nil {
		t.Fatal(err)
	}
	if len(owned) != 1 || owned[0] == source {
		t.Fatalf("快照要擁有一份不同於買家頭像的複本：頭像 %v，快照的圖片 %v", source, owned)
	}
	var content string
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT content::text FROM moderation.case_snapshots WHERE id = $1", snap).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, owned[0].String()) || strings.Contains(content, source.String()) {
		t.Fatalf("快照內容的頭像資產要換成複本 %v、不留來源 %v：%s", owned[0], source, content)
	}

	p, err = f.buyer.Profile(ctx, targetAccount)
	must(t, err)
	next := f.stage(t)
	_, err = f.buyer.UpdateProfile(ctx, buyer.UpdateProfileInput{BuyerID: target, ExpectedVersion: p.Version, AvatarStagedToken: &next})
	must(t, err)
	if st := f.assetState(t, source); st == "ACTIVE" {
		t.Fatal("買家換頭像之後來源頭像要被標記刪除")
	}
	if st := f.assetState(t, owned[0]); st != "ACTIVE" {
		t.Fatalf("買家換頭像之後快照的複本狀態 %s，預期仍是 ACTIVE", st)
	}

	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: r.Case.ID, Version: r.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}))
	f.advance(SnapshotRetention + time.Hour)
	if n := mustV[int64](t)(f.module.PurgeSnapshots(ctx)); n != 1 {
		t.Fatalf("刪了 %d 份快照", n)
	}
	if st := f.assetState(t, owned[0]); st == "ACTIVE" {
		t.Fatalf("快照過保留期被刪除之後複本要被標記刪除，狀態 %s", st)
	}
}

// 建快照的 transaction 回滾時，這一輪確認的複本回到未確認（STAGED），由 media 回收；沒有快照列指著它。
func TestASnapshotRollbackLeavesTheCopyUnconfirmed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`CREATE FUNCTION moderation.fail_report() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced failure after the copy is confirmed'; END $$`,
		`CREATE TRIGGER fail_report BEFORE INSERT ON moderation.reports FOR EACH ROW EXECUTE FUNCTION moderation.fail_report()`,
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	before := f.stagedAssets(t)
	reporter, _ := f.newBuyer(t)
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: Target{Kind: TargetPost, ContentID: w.post.ID}, Category: policy.MisleadingInfo}); err == nil {
		t.Fatal("寫入檢舉被觸發器擋下，送出要失敗")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots"); n != 0 {
		t.Fatalf("回滾之後不該有快照列：%d", n)
	}
	if n := f.stagedAssets(t); n != before+1 {
		t.Fatalf("回滾之後複本要留在 STAGED（未確認）：之前 %d，之後 %d", before, n)
	}
}

// 檢舉說明跟快照同一天清（結案滿 184 天）；進行中案件的說明不清。
func TestDescriptionsAreClearedWithTheSnapshots(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	other := f.comment(t, w.post.ID, w.author, "另一則")
	ctx := context.Background()
	reporter, _ := f.newBuyer(t)
	closed := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, Description: "已結案的說明"}))
	open := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(other), Category: policy.Spam, Description: "進行中的說明"}))
	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: closed.Case.ID, Version: closed.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}))
	f.advance(SnapshotRetention - time.Hour)
	mustV[int64](t)(f.module.PurgeSnapshots(ctx))
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE description IS NOT NULL"); n != 2 {
		t.Fatalf("未滿保留期說明都要留著：%d", n)
	}
	f.advance(2 * time.Hour)
	mustV[int64](t)(f.module.PurgeSnapshots(ctx))
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND description IS NULL", closed.Report.ID); n != 1 {
		t.Fatal("結案滿 184 天的說明要清掉，檢舉列留著")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND description IS NOT NULL", open.Report.ID); n != 1 {
		t.Fatal("進行中案件的說明不能清")
	}
}

// A7-D21 檢舉人刪帳：同 transaction 清掉他全部檢舉的說明與每日計數列；檢舉列與 reporter_buyer_id 留著，別人的說明不動。
func TestReporterDeletionClearsDescriptionsAndDailyCountButKeepsTheReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	reporter, reporterAccount := f.newBuyer(t)
	bystander, _ := f.newBuyer(t)
	mineRID, theirRID := uuid.New(), uuid.New()
	mine := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, Description: "我的說明", RequestID: &mineRID}))
	theirs := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: bystander, Target: commentTarget(w.comment), Category: policy.Spam, Description: "別人的說明", RequestID: &theirRID}))
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id IN ($1, $2)", reporter, bystander); n != 2 {
		t.Fatalf("送出之後各有一筆冪等紀錄：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1", reporter); n != 1 {
		t.Fatalf("送出之後有每日計數列：%d", n)
	}
	mustV[accountdeletion.State](t)(f.deletion.Request(ctx, accountdeletion.RequestInput{AccountID: reporterAccount, Requester: accountdeletion.AccountOwner}))
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND reporter_buyer_id = $2 AND description IS NULL", mine.Report.ID, reporter); n != 1 {
		t.Fatal("刪帳之後說明要清掉，檢舉列與 reporter_buyer_id 留著")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1", reporter); n != 0 {
		t.Fatalf("每日計數列要刪：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND description = '別人的說明'", theirs.Report.ID); n != 1 {
		t.Fatal("別人的說明不能動")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1", reporter); n != 0 {
		t.Fatalf("說明的雜湊（request_hash）不能比檢舉人的帳號活得久：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1", bystander); n != 1 {
		t.Fatal("別人的冪等紀錄不能動")
	}
}

// A6-D22 店家退場：同 transaction 清掉該店作為檢舉人的說明、冪等紀錄與每日計數列；檢舉列與 reporter_store_id 留著，買家檢舉人與別的店家的資料不動；重送事件是 no-op。
func TestStoreRetirementClearsStoreReporterDataButKeepsTheReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, storeAccount := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 4, 0)
	_, otherIDs := f.seedChat(t, b, otherStore, 4, 0)
	mineRID, otherRID, buyerRID := uuid.New(), uuid.New(), uuid.New()
	// 店家檢舉買家寄的單數訊息；買家只能檢舉店家寄的雙數訊息（檢舉自己的內容回 InvalidInput）。
	target, otherTarget := Target{Kind: TargetChatMessage, ContentID: ids[1]}, Target{Kind: TargetChatMessage, ContentID: otherIDs[1]}
	buyerTarget := Target{Kind: TargetChatMessage, ContentID: ids[2]}
	mine := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: storeID, Target: target, Category: policy.Harassment, Description: "店家的說明", RequestID: &mineRID}))
	theirs := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: otherStore, Target: otherTarget, Category: policy.Harassment, Description: "別的店家的說明", RequestID: &otherRID}))
	buyerReport := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: b, Target: buyerTarget, Category: policy.Harassment, Description: "買家的說明", RequestID: &buyerRID}))
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_store_id = $1", storeID); n != 1 {
		t.Fatalf("送出之後店家有一筆冪等紀錄：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_store_id = $1", storeID); n != 1 {
		t.Fatalf("送出之後店家有每日計數列：%d", n)
	}
	retire := func() {
		t.Helper()
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, seller.StoreRetired{OccurredAt: f.clock(), EventID: uuid.NewV7(), StoreID: storeID, AccountID: storeAccount})
		})
		if err != nil {
			t.Fatalf("發 StoreRetired：%v", err)
		}
	}
	check := func(round string) {
		t.Helper()
		if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND reporter_store_id = $2 AND description IS NULL", mine.Report.ID, storeID); n != 1 {
			t.Fatalf("%s：店家的說明要清掉，檢舉列與 reporter_store_id 留著", round)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_store_id = $1", storeID); n != 0 {
			t.Fatalf("%s：店家的冪等紀錄要刪：%d", round, n)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_store_id = $1", storeID); n != 0 {
			t.Fatalf("%s：店家的每日計數列要刪：%d", round, n)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND description = '別的店家的說明'", theirs.Report.ID); n != 1 {
			t.Fatalf("%s：別的店家的說明不能動", round)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE id = $1 AND reporter_buyer_id = $2 AND description = '買家的說明'", buyerReport.Report.ID, b); n != 1 {
			t.Fatalf("%s：買家檢舉人的說明不能動", round)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1 OR reporter_store_id = $2", b, otherStore); n != 2 {
			t.Fatalf("%s：買家與別的店家的冪等紀錄不能動：%d", round, n)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1 OR reporter_store_id = $2", b, otherStore); n != 2 {
			t.Fatalf("%s：買家與別的店家的每日計數列不能動：%d", round, n)
		}
	}
	retire()
	check("退場")
	retire()
	check("重送事件")
}

// A7-D21：冪等紀錄（request_hash 是說明的雜湊）跟說明同一天清（結案滿 184 天）；進行中案件的不清。
func TestReportRequestsArePurgedWithTheDescriptions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	other := f.comment(t, w.post.ID, w.author, "另一則")
	ctx := context.Background()
	reporter, _ := f.newBuyer(t)
	closedRID, openRID := uuid.New(), uuid.New()
	closed := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, Description: "已結案的說明", RequestID: &closedRID}))
	mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(other), Category: policy.Spam, Description: "進行中的說明", RequestID: &openRID}))
	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: closed.Case.ID, Version: closed.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}))
	f.advance(SnapshotRetention - time.Hour)
	mustV[int64](t)(f.module.PurgeSnapshots(ctx))
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1", reporter); n != 2 {
		t.Fatalf("未滿保留期冪等紀錄都要留著：%d", n)
	}
	f.advance(2 * time.Hour)
	mustV[int64](t)(f.module.PurgeSnapshots(ctx))
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE report_id = $1", closed.Report.ID); n != 0 {
		t.Fatalf("結案滿 184 天的冪等紀錄要清掉：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1 AND request_id = $2", reporter, openRID); n != 1 {
		t.Fatal("進行中案件的冪等紀錄不能清")
	}
}

// A7-D7 第 10 項：買家寫的內容已被刪除時，隱藏視為已達成：成功結案、不呼叫 social 的隱藏（沒有隱藏紀錄），案件結為違規。
func TestHideOfDeletedBuyerContentSucceedsWithoutCallingSocial(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	gone := f.comment(t, w.post.ID, w.author, "已刪除的留言")
	c := f.openCase(t, commentTarget(gone))
	f.deleteComment(t, w.author, gone.ID)
	d := mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	if d.Case.ClosedAt == nil || d.Case.Decision == nil || *d.Case.Decision != DecisionViolation {
		t.Fatalf("案件要結為違規：%+v", d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM social.hides"); n != 0 {
		t.Fatalf("內容已刪除不呼叫 Hide，卻有 %d 筆隱藏", n)
	}
	if mustV[CaseDetail](t)(f.module.Case(ctx, c.ID)).Case.ClosedAt == nil {
		t.Fatal("案件要已結案")
	}
}

// A7-D7 第 10 項：店家的貼文已被刪除時，內容下架仍用快照凍結的作者與店家驗證歸屬，建立認定與措施（對象是已刪除的內容 id）；店家取自快照。
func TestTakedownOfDeletedStoreContentVerifiesAgainstTheSnapshotAndStillCreatesTheMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	reporter, _ := f.newBuyer(t)
	c := f.submit(t, reporter, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	st, err := f.sellerRd.Store(ctx, w.storeID)
	if err != nil {
		t.Fatal(err)
	}
	must(t, f.social.DeleteOwnPost(ctx, st, w.post.ID, 0))

	in := DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	}
	d := mustV[CaseDetail](t)(f.module.DecideCase(ctx, in))
	if d.Case.ClosedAt == nil {
		t.Fatalf("案件要結案：%+v", d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE store_id = $1 AND kind = 'CONTENT_TAKEDOWN'", w.storeID); n != 1 {
		t.Fatalf("內容下架措施 %d 個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_contents WHERE content_kind = 'POST' AND content_id = $1", w.post.ID); n != 1 {
		t.Fatalf("措施的對象要是已刪除的貼文：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 1 {
		t.Fatalf("認定引用的來源檢舉 %d 筆，預期 1", n)
	}
}

// A6-D22 退場的店不能再以檢舉人身分送出：店在營運中先送出一筆進行中的檢舉，關店退場後換新的 request_id 送同一個對象，走的是凍結之前的「併入進行中案件」
// （recordRequest）路徑，這條路不經過 CapChatRead，只有 report.attempt 開頭的 LockNotRetired 擋得住，回 NotFound，不新增冪等紀錄也不加每日計數；同一個 request_id 重送也一樣被擋。
// mutation：拿掉 report.lockReporter 的店家分支 → 退場後的送出成功，斷言失敗。
func TestRetiredStoreCannotSubmitAsReporter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, storeAccount := f.exposableStoreOnIdentityAccount(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 4, 0)
	target := Target{Kind: TargetChatMessage, ContentID: ids[1]}
	firstRID, secondRID := uuid.New(), uuid.New()
	in := SubmitInput{ReporterStoreID: storeID, Target: target, Category: policy.Harassment, Description: "退場前的說明", RequestID: &firstRID}
	mustV[SubmitResult](t)(f.module.SubmitReport(ctx, in))
	if _, err := f.seller.CloseCurrent(ctx, storeAccount); err != nil {
		t.Fatalf("關店：%v", err)
	}
	f.exec(t, `UPDATE seller.stores SET retired_at = $2, description = NULL, prize = NULL, website_url = NULL,
		logo_key = NULL, logo_asset_id = NULL, cover_key = NULL, cover_asset_id = NULL WHERE store_id = $1`, storeID, f.clock())
	counts := func() (reports, requests, submitted int) {
		reports = f.count(t, "SELECT count(*) FROM moderation.reports WHERE reporter_store_id = $1", storeID)
		requests = f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_store_id = $1", storeID)
		submitted = f.count(t, "SELECT coalesce(sum(submitted), 0) FROM moderation.reporter_daily_counts WHERE reporter_store_id = $1", storeID)
		return
	}
	wantReports, wantRequests, wantSubmitted := counts()
	if wantReports != 1 || wantRequests != 1 || wantSubmitted != 1 {
		t.Fatalf("退場前各有一筆：%d %d %d", wantReports, wantRequests, wantSubmitted)
	}
	for name, rid := range map[string]uuid.UUID{"新的 request_id（併入進行中案件）": secondRID, "同一個 request_id（重送）": firstRID} {
		in.RequestID = &rid
		_, err := f.module.SubmitReport(ctx, in)
		if kindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：已退場的店送出要 NotFound：%v", name, err)
		}
		if r, q, s := counts(); r != wantReports || q != wantRequests || s != wantSubmitted {
			t.Fatalf("%s：不能新增檢舉、冪等紀錄與每日計數：%d %d %d", name, r, q, s)
		}
	}
}

// 買家檢舉人沒有凍結前的擋：關閉（帳號刪除完成）的買家換新的 request_id 送同一個對象，走併入進行中案件的 recordRequest 路徑，只有 LockNotClosed 擋得住。
// mutation：拿掉 report.lockReporter 的買家分支 → 關閉後的送出成功，斷言失敗。
func TestClosedBuyerCannotSubmitAsReporter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	reporter, _ := f.newBuyer(t)
	firstRID, secondRID := uuid.New(), uuid.New()
	in := SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, Description: "關閉前的說明", RequestID: &firstRID}
	mustV[SubmitResult](t)(f.module.SubmitReport(ctx, in))
	must(t, f.buyer.BeginDeletion(ctx, reporter))
	must(t, f.buyer.CloseForDeletion(ctx, reporter))
	counts := func() (requests, submitted int) {
		requests = f.count(t, "SELECT count(*) FROM moderation.report_requests WHERE reporter_buyer_id = $1", reporter)
		submitted = f.count(t, "SELECT coalesce(sum(submitted), 0) FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1", reporter)
		return
	}
	wantRequests, wantSubmitted := counts()
	for name, rid := range map[string]uuid.UUID{"新的 request_id（併入進行中案件）": secondRID, "同一個 request_id（重送）": firstRID} {
		in.RequestID = &rid
		_, err := f.module.SubmitReport(ctx, in)
		if kindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：已關閉的買家送出要 NotFound：%v", name, err)
		}
		if q, s := counts(); q != wantRequests || s != wantSubmitted {
			t.Fatalf("%s：不能新增冪等紀錄與每日計數：%d %d", name, q, s)
		}
	}
}

// MyStoreReports 只回這家店送出的檢舉（只有聊天訊息），新到舊、keyset 分頁；買家送出的與別家店的都不出現。
func TestMyStoreReportsListsOnlyTheStoresOwnReportsPaged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 6, 0)
	_, otherIDs := f.seedChat(t, b, otherStore, 4, 0)
	chat := func(id uuid.UUID) Target { return Target{Kind: TargetChatMessage, ContentID: id} }
	first := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: storeID, Target: chat(ids[1]), Category: policy.Harassment}))
	f.advance(time.Second)
	second := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: storeID, Target: chat(ids[3]), Category: policy.Harassment}))
	mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: otherStore, Target: chat(otherIDs[1]), Category: policy.Harassment}))
	mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: b, Target: chat(ids[2]), Category: policy.Harassment}))

	p1, err := f.module.MyStoreReports(ctx, storeID, nil, 1)
	must(t, err)
	if len(p1.Items) != 1 || p1.Next == nil || p1.Items[0].Report.ID != second.Report.ID || p1.Items[0].Target != chat(ids[3]) {
		t.Fatalf("第一頁要是最新一筆且有下一頁：%+v", p1)
	}
	p2, err := f.module.MyStoreReports(ctx, storeID, p1.Next, 1)
	must(t, err)
	if len(p2.Items) != 1 || p2.Next != nil || p2.Items[0].Report.ID != first.Report.ID {
		t.Fatalf("第二頁：%+v", p2)
	}
	for _, it := range append(p1.Items, p2.Items...) {
		if it.Report.ReporterStoreID != storeID || it.Report.ReporterBuyerID != 0 {
			t.Fatalf("只能有這家店送出的檢舉：%+v", it.Report)
		}
	}
	none, err := f.module.MyStoreReports(ctx, otherStore+1000, nil, 50)
	must(t, err)
	if len(none.Items) != 0 {
		t.Fatalf("沒送過檢舉的店：%+v", none)
	}
}
