//go:build integration

package moderation

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

func (f *fixture) openCase(t *testing.T, target Target) Case {
	t.Helper()
	reporter, _ := f.newBuyer(t)
	return f.submit(t, reporter, target, policy.Spam).Case
}

func (f *fixture) bumpVersion(t *testing.T, id uuid.UUID) {
	t.Helper()
	f.exec(t, "UPDATE moderation.cases SET version = version + 1 WHERE id = $1", id)
}

func (f *fixture) decide(t *testing.T, c Case, action Action, category policy.Category) (CaseDetail, error) {
	t.Helper()
	return f.module.DecideCase(context.Background(), DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: category, Action: action, Text: "違反社群規範",
	})
}

func (f *fixture) suspended(t *testing.T, accountID int64) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM identity.accounts WHERE id = $1 AND suspended_at IS NOT NULL", accountID) == 1
}

// SD11 的處置面與 SD10：隱藏、警告、停權各自經擁有處分的模組留下紀錄，來源參照指到案件。
func TestBuyerBranchActionsLeaveRecordsInTheOwningModules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()

	hideCase := f.openCase(t, commentTarget(w.comment))
	mustV[CaseDetail](t)(f.decide(t, hideCase, ActionHide, policy.Harassment))
	if f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatal("留言要被隱藏")
	}
	if n := f.count(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1 AND source_ref = $2 AND admin_id = 9 AND reason_category = 'HARASSMENT'", w.comment.ID, SourceRefPrefix+hideCase.ID.String()); n != 1 {
		t.Fatalf("隱藏紀錄 %d 筆", n)
	}

	warnComment := f.comment(t, w.post.ID, w.author, "第二則")
	warnCase := f.openCase(t, commentTarget(warnComment))
	mustV[CaseDetail](t)(f.decide(t, warnCase, ActionWarn, policy.Spam))
	page := mustV[social.SanctionPage](t)(f.social.MySanctions(ctx, w.author, nil, 10))
	if len(page.Sanctions) != 1 || page.Sanctions[0].Kind != social.SanctionWarning || page.Sanctions[0].SourceRef != SourceRefPrefix+warnCase.ID.String() {
		t.Fatalf("作者的處分列表要有這筆警告：%+v", page)
	}
	if !f.readable(t, social.TargetComment, warnComment.ID) {
		t.Fatal("警告不隱藏內容")
	}

	banCase := f.openCase(t, Target{Kind: TargetBuyer, BuyerID: w.author})
	mustV[CaseDetail](t)(f.decide(t, banCase, ActionBan, policy.Fraud))
	if !f.suspended(t, w.authorAccount) {
		t.Fatal("帳號要被停權")
	}
	if n := f.count(t, "SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1 AND source_ref = $2 AND released_at IS NULL", w.authorAccount, SourceRefPrefix+banCase.ID.String()); n != 1 {
		t.Fatalf("停權紀錄 %d 筆", n)
	}
	closed := mustV[CaseDetail](t)(f.module.Case(ctx, banCase.ID))
	if closed.Case.Action == nil || *closed.Case.Action != ActionBan || closed.Case.Category == nil || *closed.Case.Category != policy.Fraud || closed.Case.DecidedByAdmin == nil || *closed.Case.DecidedByAdmin != 9 {
		t.Fatalf("決定要寫在案件上：%+v", closed.Case)
	}
}

// 回滾：處置之後結案 UPDATE 失敗（版本過期），處置與紀錄全部回滾，被處置的內容與帳號狀態不變。
func TestDecideRollsBackTheActionWhenCloseLosesTheRace(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	assertConflict := func(t *testing.T, err error) {
		t.Helper()
		if kindOf(err) != apperr.KindVersionConflict {
			t.Fatalf("要回 VersionConflict：%v", err)
		}
	}

	t.Run("Hide", func(t *testing.T) {
		c := f.openCase(t, commentTarget(w.comment))
		f.bumpVersion(t, c.ID)
		_, err := f.decide(t, c, ActionHide, policy.Spam)
		assertConflict(t, err)
		if !f.readable(t, social.TargetComment, w.comment.ID) || f.count(t, "SELECT count(*) FROM social.hides") != 0 {
			t.Fatal("隱藏要整個回滾")
		}
	})
	t.Run("Warn", func(t *testing.T) {
		c := f.openCase(t, commentTarget(w.comment))
		f.bumpVersion(t, c.ID)
		_, err := f.decide(t, c, ActionWarn, policy.Spam)
		assertConflict(t, err)
		if page := mustV[social.SanctionPage](t)(f.social.MySanctions(ctx, w.author, nil, 10)); len(page.Sanctions) != 0 {
			t.Fatalf("警告要整個回滾：%+v", page)
		}
	})
	t.Run("Suspend", func(t *testing.T) {
		c := f.openCase(t, Target{Kind: TargetBuyer, BuyerID: w.author})
		f.bumpVersion(t, c.ID)
		_, err := f.decide(t, c, ActionBan, policy.Spam)
		assertConflict(t, err)
		if f.suspended(t, w.authorAccount) || f.count(t, "SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1", w.authorAccount) != 0 {
			t.Fatal("停權要整個回滾")
		}
	})
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE closed_at IS NOT NULL"); n != 0 {
		t.Fatalf("沒有案件可以結案：%d", n)
	}
}

// SD10：店家不能被停權，店家寫的內容只能走內容下架；處置種類與對象或作者不搭配時什麼都不寫、案件仍是進行中。
func TestActionsThatDoNotFitTheTargetAreRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	postCase := f.openCase(t, Target{Kind: TargetPost, ContentID: w.post.ID})
	for _, a := range []Action{ActionHide, ActionWarn, ActionBan} {
		_, err := f.decide(t, postCase, a, policy.Spam)
		if kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("貼文的處置 %s：%v，預期 InvalidInput", a, err)
		}
	}
	commentCase := f.openCase(t, commentTarget(w.comment))
	_, err := f.module.DecideCase(context.Background(), DecideInput{
		CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.Spam, Action: ActionTakedown,
		Text: "下架", Severity: string(seller.SeverityValues()[0]),
	})
	if kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("買家寫的留言不能走內容下架：%v", err)
	}
	for name, in := range map[string]DecideInput{
		"類別不適用社群":  {CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.Counterfeit, Action: ActionHide, Text: "x"},
		"沒有理由":     {CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.Spam, Action: ActionHide},
		"不違規帶了處置":  {CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation, Action: ActionHide},
		"決定不合法":    {CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: "MAYBE"},
		"非下架帶了嚴重度": {CaseID: commentCase.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.Spam, Action: ActionHide, Text: "x", Severity: string(seller.SeverityValues()[0])},
	} {
		if _, err := f.module.DecideCase(context.Background(), in); kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 InvalidInput", name, err)
		}
	}
	if _, err := f.module.DecideCase(context.Background(), DecideInput{CaseID: uuid.NewV7(), Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的案件：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE closed_at IS NOT NULL"); n != 0 {
		t.Fatalf("被拒絕的處理不能結案：%d", n)
	}
}

// 有店的帳號不能停權：案件仍是進行中，帳號沒有被停權。
func TestBanRefusesAnAccountThatOwnsAStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	buyerID, accountID := f.newBuyer(t)
	if _, err := f.seller.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: accountID, StoreName: "測試小店"}); err != nil {
		t.Fatal(err)
	}
	c := f.openCase(t, Target{Kind: TargetBuyer, BuyerID: buyerID})
	_, err := f.decide(t, c, ActionBan, policy.Fraud)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Reason != apperr.ReasonAccountHasStore {
		t.Fatalf("要回 ACCOUNT_HAS_STORE：%v", err)
	}
	if f.suspended(t, accountID) || mustV[CaseDetail](t)(f.module.Case(context.Background(), c.ID)).Case.ClosedAt != nil {
		t.Fatal("拒絕後帳號與案件都不能變")
	}
}

// 店家分支：店家取自案件快照並向 social 核對內容歸屬，案件記下建立的措施；成功時措施、認定的來源檢舉與結案同一個 transaction，貼文隨之下架。
func TestTakedownUsesTheSnapshotStoreAndRecordsTheMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var reporters []int64
	for range 3 {
		r, _ := f.newBuyer(t)
		reporters = append(reporters, r)
	}
	var c Case
	for _, r := range reporters {
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	in := DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	}
	d := mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), in))
	if d.Case.ClosedAt == nil || len(d.Reports) != 3 || d.Case.ReportCount != 3 {
		t.Fatalf("%+v", d)
	}
	if d.Case.StoreID == nil || *d.Case.StoreID != w.storeID {
		t.Fatalf("案件的店家要是快照的店家 %d：%v", w.storeID, d.Case.StoreID)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE id = $1 AND store_id = $2", d.Case.MeasureID, w.storeID); d.Case.MeasureID == nil || n != 1 {
		t.Fatalf("案件的 measure_id 要指向下架措施：%v", d.Case.MeasureID)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE store_id = $1 AND kind = 'CONTENT_TAKEDOWN'", w.storeID); n != 1 {
		t.Fatalf("內容下架措施 %d 個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 3 {
		t.Fatalf("認定引用的來源檢舉 %d 筆，預期 3", n)
	}
	if f.readable(t, social.TargetPost, w.post.ID) {
		t.Fatal("貼文要隨措施下架")
	}
	if n := f.count(t, "SELECT count(*) FROM adminaudit.records WHERE target_type = 'moderation_case' AND target_id = $1 AND detail->>'action' = 'TAKEDOWN' AND detail->>'store_id' = $2 AND detail->>'severity' = $3",
		c.ID.String(), strconv.FormatInt(w.storeID, 10), in.Severity); n != 1 {
		t.Fatalf("下架決定的稽核 detail 要有 store_id 與 severity：%d", n)
	}
}

// 內容已在生效中的下架措施之下（例如後台貼文頁先下架）：DecideCase TAKEDOWN 綁定既有措施並結案，不建第二個措施、不產生新的認定。
func TestTakedownBindsToTheExistingActiveMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var c Case
	for range 2 {
		r, _ := f.newBuyer(t)
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	existing := mustV[seller.CreateMeasureResult](t)(f.seller.CreateMeasure(context.Background(), seller.CreateMeasureInput{
		AdminID: 3, StoreID: w.storeID, Kind: seller.MeasureContentTakedown, StatementToSeller: "後台先下架", Note: "貼文頁",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.MisleadingInfo, Severity: seller.Severity(seller.SeverityValues()[0])}},
		Takedown: &seller.ContentTakedown{Contents: []seller.TakedownContent{{
			ContentRef: seller.ContentRef{Kind: seller.ContentPost, ID: w.post.ID},
			Snapshot:   &seller.ContentSnapshot{Body: "後台先下架的本文", AuthoredAt: f.clock(), Link: seller.ContentLink{PostID: w.post.ID}},
		}}},
	}))
	in := DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	}
	d := mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), in))
	if d.Case.ClosedAt == nil || d.Case.MeasureID == nil || *d.Case.MeasureID != existing.Measure.ID {
		t.Fatalf("案件要結案並綁定既有措施 %s：%+v", existing.Measure.ID, d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE store_id = $1 AND kind = 'CONTENT_TAKEDOWN'", w.storeID); n != 1 {
		t.Fatalf("不建第二個措施：%d 個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_moderation_reports"); n != 0 {
		t.Fatalf("不產生新的認定：%d 筆來源檢舉", n)
	}
}

// 被檢舉的貼文在決議前已被清除：歸屬以案件快照核對，措施照常建立。
func TestTakedownAfterThePostIsPurgedStillCreatesTheMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var c Case
	for range 3 {
		r, _ := f.newBuyer(t)
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	f.exec(t, `WITH c AS (SELECT id FROM social.comments WHERE post_id = $1),
a AS (DELETE FROM social.comment_likes WHERE comment_id IN (SELECT id FROM c)),
b AS (DELETE FROM social.comment_stats WHERE comment_id IN (SELECT id FROM c)),
d AS (DELETE FROM social.hides WHERE comment_id IN (SELECT id FROM c)),
e AS (DELETE FROM social.content_takedowns WHERE post_id = $1 OR comment_id IN (SELECT id FROM c)),
g AS (DELETE FROM social.post_hashtags WHERE post_id = $1),
h AS (DELETE FROM social.post_likes WHERE post_id = $1),
i AS (DELETE FROM social.post_stats WHERE post_id = $1),
j AS (DELETE FROM social.post_views WHERE post_id = $1),
k AS (DELETE FROM social.post_shares WHERE post_id = $1),
l AS (DELETE FROM social.comments WHERE post_id = $1)
DELETE FROM social.posts WHERE id = $1`, w.post.ID)
	d := mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	}))
	if d.Case.ClosedAt == nil {
		t.Fatalf("%+v", d)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE store_id = $1 AND kind = 'CONTENT_TAKEDOWN'", w.storeID); n != 1 {
		t.Fatalf("內容下架措施 %d 個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_contents WHERE content_id = $1 AND snapshot_state = 'CAPTURED' AND link_post_id = $1", w.post.ID); n != 1 {
		t.Fatalf("貼文已清除時措施仍帶案件快照（連結是貼文）：%d", n)
	}
}

// 決議下架時措施存的是案件最近一份凍結的內容（檢舉當下的版本），不是決議當下社群的現況，也不是最早那份；存入發文時間、圖片張數與貼文連結。
// mutation：LatestSnapshotContent 改成 ORDER BY captured_at ASC → 存到第一版本文；改成讀 social 現況 → 存到「決議前再改的本文」。
func TestTakedownStoresTheLatestCaseSnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var c Case
	for i, body := range []string{"第一版本文", "第二版本文"} {
		if i > 0 {
			f.advance(time.Minute)
		}
		f.exec(t, "UPDATE social.posts SET body = $1 WHERE id = $2", body, w.post.ID)
		r, _ := f.newBuyer(t)
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	f.advance(time.Minute)
	f.exec(t, "UPDATE social.posts SET body = '決議前再改的本文' WHERE id = $1", w.post.ID)
	d := mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	}))
	if d.Case.MeasureID == nil {
		t.Fatalf("%+v", d)
	}
	var body string
	var images int
	var post uuid.UUID
	var authored time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT snapshot_body, snapshot_image_count, link_post_id, snapshot_authored_at FROM seller.measure_contents WHERE measure_id = $1", *d.Case.MeasureID).Scan(&body, &images, &post, &authored))
	if body != "第二版本文" || post != w.post.ID || images != 1 || authored.IsZero() {
		t.Fatalf("措施的快照 body=%q images=%d post=%s authored=%s", body, images, post, authored)
	}
}

// 鎖順序：moderation 建立內容下架與 accountdeletion 並行，不死結，也沒有半套狀態。
func TestTakedownAndAccountDeletionRunConcurrentlyWithoutDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, ownerAccount := f.newBuyer(t)
	w := world{storeAccount: ownerAccount}
	var err error
	w.storeID, err = sellerfixture.ExposableStore(context.Background(), f.db, ownerAccount, f.clock())
	if err != nil {
		t.Fatal(err)
	}
	w.post = f.post(t, w.storeID, "貼文")
	reporter, _ := f.newBuyer(t)
	c := f.submit(t, reporter, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	var wg sync.WaitGroup
	start := make(chan struct{})
	var decideErr, deleteErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, decideErr = f.module.DecideCase(context.Background(), DecideInput{
			CaseID: c.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
			Text: "不實", Severity: string(seller.SeverityValues()[0]),
		})
	}()
	go func() {
		defer wg.Done()
		<-start
		_, deleteErr = f.deletion.Request(context.Background(), accountdeletion.RequestInput{AccountID: w.storeAccount, Requester: accountdeletion.AccountOwner})
	}()
	close(start)
	wg.Wait()
	for _, err := range []error{decideErr, deleteErr} {
		if err != nil && apperr.KindOf(err) == apperr.KindInternal {
			t.Fatalf("不能有死結或內部錯誤：decide=%v delete=%v", decideErr, deleteErr)
		}
	}
	if deleteErr != nil {
		t.Fatalf("真帳號的刪帳申請要完成：%v", deleteErr)
	}
	if acct, err := f.identity.Account(context.Background(), w.storeAccount); err != nil || !acct.Deleted {
		t.Fatalf("刪帳要真的發生：account=%+v err=%v", acct, err)
	}
	closed := mustV[CaseDetail](t)(f.module.Case(context.Background(), c.ID)).Case.ClosedAt != nil
	measures := f.count(t, "SELECT count(*) FROM seller.measures WHERE store_id = $1 AND kind = 'CONTENT_TAKEDOWN'", w.storeID)
	if closed != (measures == 1) || (decideErr == nil) != closed {
		t.Fatalf("結案 %v 與措施 %d 個要同進退（decide=%v）", closed, measures, decideErr)
	}
}

// 內容還在、案件卻沒有快照：內容下架回「沒有快照無法確認店家」，不是「內容已不存在」。
func TestTakedownWithoutSnapshotWhileContentExistsSaysSo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var c Case
	for range 3 {
		r, _ := f.newBuyer(t)
		c = f.submit(t, r, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.MisleadingInfo).Case
	}
	f.exec(t, "DELETE FROM moderation.case_snapshots WHERE case_id = $1", c.ID)
	_, err := f.module.DecideCase(context.Background(), DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.MisleadingInfo, Action: ActionTakedown,
		Text: "貼文含不實資訊", Severity: string(seller.SeverityValues()[0]),
	})
	if kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), text.DecideTakedownSnapshotMissing().Text) {
		t.Fatalf("%v", err)
	}
}
