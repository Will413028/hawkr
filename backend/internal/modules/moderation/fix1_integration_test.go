//go:build integration

package moderation

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) snapshotRef(id uuid.UUID) media.Ref {
	return media.Ref{Type: media.RefTypeGovernanceEvidence, ID: id.String()}
}

func (f *fixture) deleteComment(t *testing.T, buyerID int64, id uuid.UUID) {
	t.Helper()
	must(t, f.social.DeleteComment(context.Background(), social.Caller{BuyerID: buyerID}, id))
}

// 作者刪掉被檢舉的內容不能躲過警告與停權：處置用案件快照裡凍結的作者；內容已不存在時隱藏沒有東西可以處置，案件維持進行中。
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

	hidden := f.comment(t, w.post.ID, w.author, "已刪除的留言")
	hideCase := f.openCase(t, commentTarget(hidden))
	f.deleteComment(t, w.author, hidden.ID)
	if _, err := f.decide(t, hideCase, ActionHide, policy.Spam); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("內容已刪除時隱藏沒有東西可以處置：%v", err)
	}
	if mustV[CaseDetail](t)(f.module.Case(ctx, hideCase.ID)).Case.ClosedAt != nil {
		t.Fatal("被拒絕的處置不能結案")
	}
	if n := f.count(t, "SELECT count(*) FROM social.hides"); n != 0 {
		t.Fatalf("拒絕後留下 %d 筆隱藏", n)
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
		CaseID: read.ID, Version: read.Version, AdminID: 9, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "不實", StoreID: w.storeID, Severity: string(seller.SeverityValues()[0]),
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
		CaseID: c.ID, Version: c.Version, AdminID: 9, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "不實", StoreID: w.storeID, Severity: string(seller.SeverityValues()[0]),
	}))
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 20 {
		t.Fatalf("引用 %d 筆，預期上限 20", n)
	}
	var note string
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT note FROM seller.measures WHERE store_id = $1", w.storeID).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note, "25 筆") {
		t.Fatalf("備註要寫真正的檢舉數 25：%q", note)
	}
}

// A7-D21 證據保全：快照引用的圖片在 media 有一筆以快照為 ref 的引用；貼文被清除、作者的引用釋放之後圖片仍被快照引用，快照刪除之後才釋放。
func TestASnapshotHoldsItsImageUntilTheSnapshotIsPurged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	postRef := media.Ref{Type: media.RefTypePost, ID: w.post.ID.String()}
	mustV[media.SyncResult](t)(f.social.SyncMedia(ctx, postRef))
	assets := f.media.Referenced(postRef)
	if len(assets) != 1 {
		t.Fatalf("貼文的圖片引用 %v", assets)
	}
	asset := assets[0]

	reporter, _ := f.newBuyer(t)
	r := f.submit(t, reporter, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo)
	snap := *r.Report.SnapshotID
	ref := f.snapshotRef(snap)
	if f.media.HasRef(ref, asset) {
		t.Fatal("引用由同步 job 寫入，送出的 transaction 內不呼叫 media")
	}
	must(t, f.module.Retention().SyncSnapshotMedia(ctx, snap))
	if !f.media.HasRef(ref, asset) {
		t.Fatalf("同步之後快照要引用圖片：%v", f.media.Referenced(ref))
	}

	must(t, f.social.PurgeStoreContent(ctx, w.storeID))
	mustV[media.SyncResult](t)(f.social.SyncMedia(ctx, postRef))
	if len(f.media.Referenced(postRef)) != 0 {
		t.Fatalf("貼文清除之後作者的引用釋放：%v", f.media.Referenced(postRef))
	}
	if !f.media.HasRef(ref, asset) {
		t.Fatal("貼文清除之後圖片仍要被快照引用")
	}

	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: r.Case.ID, Version: r.Case.Version, AdminID: 9, Decision: DecisionNoViolation}))
	f.advance(SnapshotRetention + time.Hour)
	if n := mustV[int64](t)(f.module.PurgeSnapshots(ctx)); n != 1 {
		t.Fatalf("刪了 %d 份快照", n)
	}
	if !f.media.HasRef(ref, asset) {
		t.Fatal("引用只由同步 job 解除，刪快照的 transaction 內不呼叫 media")
	}
	must(t, f.module.Retention().SyncSnapshotMedia(ctx, snap))
	if f.media.HasRef(ref, asset) {
		t.Fatalf("快照刪除之後要釋放圖片引用：%v", f.media.Referenced(ref))
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
	mustV[CaseDetail](t)(f.module.DecideCase(ctx, DecideInput{CaseID: closed.Case.ID, Version: closed.Case.Version, AdminID: 9, Decision: DecisionNoViolation}))
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
	mine := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, Description: "我的說明"}))
	theirs := mustV[SubmitResult](t)(f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: bystander, Target: commentTarget(w.comment), Category: policy.Spam, Description: "別人的說明"}))
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
}
