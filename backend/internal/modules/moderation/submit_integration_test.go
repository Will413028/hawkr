//go:build integration

package moderation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// world 是一家可曝光的店、一篇貼文與一位寫了留言的買家。
type world struct {
	storeID, storeAccount int64
	post                  social.Post
	author, authorAccount int64
	comment               social.Comment
}

func (f *fixture) world(t *testing.T) world {
	t.Helper()
	w := world{}
	w.storeID, w.storeAccount = f.exposableStore(t)
	w.post = f.post(t, w.storeID, "貼文")
	w.author, w.authorAccount = f.newBuyer(t)
	w.comment = f.comment(t, w.post.ID, w.author, "留言內容")
	return w
}

func (f *fixture) submit(t *testing.T, reporter int64, target Target, category policy.Category) SubmitResult {
	t.Helper()
	return mustV[SubmitResult](t)(f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: target, Category: category}))
}

func (f *fixture) decideHide(t *testing.T, caseID uuid.UUID, version int64) (CaseDetail, error) {
	t.Helper()
	return f.module.DecideCase(context.Background(), DecideInput{
		CaseID: caseID, Version: version, AdminID: 9, Decision: DecisionViolation, Category: policy.Spam, Action: ActionHide, Text: "垃圾訊息",
	})
}

// 30 筆檢舉同一對象只有一個進行中案件；結案後 30 筆的推導結果都等於案件決定。
func TestThirtyReportsShareOneOpenCaseAndOutcomesFollowTheDecision(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	var caseID uuid.UUID
	var version int64
	for i := range 30 {
		reporter, _ := f.newBuyer(t)
		r := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
		if !r.Created || r.Report.Outcome != OutcomePending {
			t.Fatalf("第 %d 筆：%+v", i, r)
		}
		if i == 0 {
			caseID = r.Case.ID
		} else if r.Case.ID != caseID {
			t.Fatalf("第 %d 筆開了另一個案件", i)
		}
		version = r.Case.Version
		if version != int64(i+1) {
			t.Fatalf("第 %d 筆之後案件版本 %d，預期每筆掛進來加一（從 1 起）", i, version)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE closed_at IS NULL"); n != 1 {
		t.Fatalf("進行中案件 %d 個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots"); n != 1 {
		t.Fatalf("內容沒有改過，快照要只有一份，得到 %d", n)
	}
	closed := mustV[CaseDetail](t)(f.decideHide(t, caseID, version))
	if closed.Case.Decision == nil || *closed.Case.Decision != DecisionViolation || len(closed.Reports) != 30 {
		t.Fatalf("%+v", closed.Case)
	}
	for _, r := range mustV[CaseDetail](t)(f.module.Case(context.Background(), caseID)).Reports {
		if r.Outcome != OutcomeViolation {
			t.Fatalf("檢舉 %s 的結果 %s，預期 VIOLATION", r.ID, r.Outcome)
		}
	}
	if f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatal("違規的留言要被隱藏")
	}
}

// 不違規：每筆檢舉的結果是 NO_VIOLATION，內容不處置。
func TestNoViolationLeavesTheContentAndDerivesTheOutcome(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	r := f.submit(t, reporter, commentTarget(w.comment), policy.Harassment)
	d := mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{CaseID: r.Case.ID, Version: 1, AdminID: 9, Decision: DecisionNoViolation}))
	if d.Reports[0].Outcome != OutcomeNoViolation || !f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatalf("%+v", d)
	}
	if n := f.count(t, "SELECT count(*) FROM social.hides"); n != 0 {
		t.Fatalf("不違規不處置，得到 %d 筆隱藏", n)
	}
}

// 結案後對同一對象的新檢舉開新案件；進行中重送回傳既有的檢舉，結案後同一位檢舉人可以再檢舉。
func TestResendReturnsTheExistingReportAndAClosedCaseAllowsANewOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	first := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
	again := f.submit(t, reporter, commentTarget(w.comment), policy.Fraud)
	if again.Created || again.Report.ID != first.Report.ID || again.Report.Category != policy.Spam || again.Case.ID != first.Case.ID {
		t.Fatalf("重送要回傳既有的檢舉：%+v", again)
	}
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{CaseID: first.Case.ID, Version: 1, AdminID: 9, Decision: DecisionNoViolation})))
	next := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
	if !next.Created || next.Case.ID == first.Case.ID || next.Report.ID == first.Report.ID {
		t.Fatalf("結案後要開新案件：%+v", next)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE closed_at IS NULL"); n != 1 {
		t.Fatalf("進行中案件 %d 個", n)
	}
}

func errOf[T any](_ T, err error) error { return err }

// 並行結案同一案只有一個成功，其餘回 VersionConflict。
func TestConcurrentCloseHasExactlyOneWinner(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	c := f.submit(t, reporter, commentTarget(w.comment), policy.Spam).Case
	const n = 8
	var wins, conflicts atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.module.DecideCase(context.Background(), DecideInput{CaseID: c.ID, Version: 1, AdminID: 9, Decision: DecisionNoViolation})
			switch {
			case err == nil:
				wins.Add(1)
			case kindOf(err) == apperr.KindVersionConflict:
				conflicts.Add(1)
			default:
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != n-1 {
		t.Fatalf("成功 %d、衝突 %d", wins.Load(), conflicts.Load())
	}
}

// 並行送出 25 筆不同對象的檢舉，成功數恰好 20。
func TestConcurrentSubmitsAreCappedAtTheDailyLimit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	var targets []Target
	for range 25 {
		targets = append(targets, commentTarget(f.comment(t, w.post.ID, w.author, "留言")))
	}
	var ok, exhausted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, tg := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: tg, Category: policy.Spam})
			switch {
			case err == nil:
				ok.Add(1)
			case kindOf(err) == apperr.KindResourceExhausted:
				exhausted.Add(1)
			default:
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if ok.Load() != DailyReportLimit || exhausted.Load() != 5 {
		t.Fatalf("成功 %d、被上限拒絕 %d，預期 20 與 5", ok.Load(), exhausted.Load())
	}
	if n := f.count(t, "SELECT submitted FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1", reporter); n != DailyReportLimit {
		t.Fatalf("計數 %d", n)
	}
}

// 已到上限的檢舉人重送已檢舉過的對象回傳既有檢舉且成功，對新對象被拒絕（重送不吃額度）；換台北日後額度重算。
func TestAtTheLimitAResendSucceedsButANewTargetIsRefused(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	var targets []Target
	for range DailyReportLimit {
		targets = append(targets, commentTarget(f.comment(t, w.post.ID, w.author, "留言")))
	}
	for _, tg := range targets {
		f.submit(t, reporter, tg, policy.Spam)
	}
	again := f.submit(t, reporter, targets[0], policy.Spam)
	if again.Created {
		t.Fatalf("重送要回傳既有的：%+v", again)
	}
	extra := commentTarget(f.comment(t, w.post.ID, w.author, "第 21 則"))
	_, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: extra, Category: policy.Spam})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.RetryAfter <= 0 {
		t.Fatalf("對新對象要被上限拒絕：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE reporter_buyer_id = $1", reporter); n != DailyReportLimit {
		t.Fatalf("檢舉 %d 筆", n)
	}
	f.advance(25 * time.Hour)
	if r := f.submit(t, reporter, extra, policy.Spam); !r.Created {
		t.Fatalf("換日後額度重算：%+v", r)
	}
}

// SD21：送出時凍結快照；兩筆檢舉之間內容被改，兩版都留下，各檢舉引用送出時的那份。
func TestSnapshotIsFrozenPerContentVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	r1, _ := f.newBuyer(t)
	r2, _ := f.newBuyer(t)
	r3, _ := f.newBuyer(t)
	a := f.submit(t, r1, commentTarget(w.comment), policy.Spam)
	b := f.submit(t, r2, commentTarget(w.comment), policy.Spam)
	if a.Report.SnapshotID == nil || b.Report.SnapshotID == nil || *a.Report.SnapshotID != *b.Report.SnapshotID {
		t.Fatalf("內容沒變，兩筆引用同一份：%+v %+v", a.Report.SnapshotID, b.Report.SnapshotID)
	}
	mustV[social.Comment](t)(f.social.UpdateComment(context.Background(), social.Caller{BuyerID: w.author}, w.comment.ID, "改過的留言"))
	c := f.submit(t, r3, commentTarget(w.comment), policy.Spam)
	if c.Report.SnapshotID == nil || *c.Report.SnapshotID == *a.Report.SnapshotID {
		t.Fatal("內容改過，要多一份快照")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots WHERE case_id = $1", a.Case.ID); n != 2 {
		t.Fatalf("兩版都要留下，得到 %d 份", n)
	}
	var content string
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT content FROM moderation.case_snapshots WHERE id = $1", *a.Report.SnapshotID).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if want := "留言內容"; !contains(content, want) || contains(content, "改過") {
		t.Fatalf("第一份快照要是送出當時的內容：%s", content)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// 結案 184 天後快照被刪，案件與檢舉列仍在；進行中案件的快照不刪；未滿 184 天不刪。
func TestSnapshotsArePurgedAfterTheRetentionAndCasesStay(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	other := f.comment(t, w.post.ID, w.author, "另一則")
	reporter, _ := f.newBuyer(t)
	closedCase := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
	openCase := f.submit(t, reporter, commentTarget(other), policy.Spam)
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{CaseID: closedCase.Case.ID, Version: 1, AdminID: 9, Decision: DecisionNoViolation})))
	f.advance(SnapshotRetention - time.Hour)
	if n := mustV[int64](t)(f.module.PurgeSnapshots(context.Background())); n != 0 {
		t.Fatalf("未滿保留期不能刪，刪了 %d", n)
	}
	f.advance(2 * time.Hour)
	if n := mustV[int64](t)(f.module.PurgeSnapshots(context.Background())); n != 1 {
		t.Fatalf("刪了 %d 份，預期 1", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots WHERE case_id = $1", closedCase.Case.ID); n != 0 {
		t.Fatalf("結案案件的快照還在 %d 份", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots WHERE case_id = $1", openCase.Case.ID); n != 1 {
		t.Fatalf("進行中案件的快照不能刪，剩 %d 份", n)
	}
	d := mustV[CaseDetail](t)(f.module.Case(context.Background(), closedCase.Case.ID))
	if len(d.Reports) != 1 || d.Reports[0].SnapshotID != nil || d.Reports[0].Outcome != OutcomeNoViolation {
		t.Fatalf("案件與檢舉列要留著、snapshot_id 清空：%+v", d)
	}
	if n := mustV[int64](t)(f.module.PurgeSnapshots(context.Background())); n != 0 {
		t.Fatalf("冪等，第二次刪了 %d", n)
	}
}

// SD23：不驗證目標存在、不禁止檢舉自己。
func TestSubmitValidatesTheTargetAndForbidsSelfReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	other, _ := f.newBuyer(t)
	submit := func(reporter int64, tg Target, c policy.Category) error {
		_, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: tg, Category: c})
		return err
	}
	for name, c := range map[string]struct {
		err  error
		want apperr.Kind
	}{
		"不存在的留言":      {submit(other, Target{Kind: TargetComment, ContentID: uuid.NewV7()}, policy.Spam), apperr.KindNotFound},
		"不存在的買家":      {submit(other, Target{Kind: TargetBuyer, BuyerID: 987654}, policy.Spam), apperr.KindNotFound},
		"檢舉自己的留言":     {submit(w.author, commentTarget(w.comment), policy.Spam), apperr.KindInvalidInput},
		"檢舉自己":        {submit(w.author, Target{Kind: TargetBuyer, BuyerID: w.author}, policy.Spam), apperr.KindInvalidInput},
		"店家本人檢舉自己的貼文": {submit(f.storeOwnerBuyer(t, w), Target{Kind: TargetPost, ContentID: w.post.ID}, policy.Spam), apperr.KindInvalidInput},
		"店家專用的類別":     {submit(other, commentTarget(w.comment), policy.PlatformAbuse), apperr.KindInvalidInput},
		"不是社群檢舉的類別":   {submit(other, commentTarget(w.comment), policy.Counterfeit), apperr.KindInvalidInput},
		"未指定類別":       {submit(other, commentTarget(w.comment), ""), apperr.KindInvalidInput},
		"對象形狀不合法":     {submit(other, Target{Kind: TargetBuyer, ContentID: w.post.ID}, policy.Spam), apperr.KindInvalidInput},
	} {
		if kindOf(c.err) != c.want {
			t.Errorf("%s：%v，預期 %v", name, c.err, c.want)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports"); n != 0 {
		t.Fatalf("被拒絕的檢舉不能留下列：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reporter_daily_counts"); n != 0 {
		t.Fatalf("被拒絕的檢舉不能吃額度：%d", n)
	}
	// 被隱藏的內容不能再被檢舉（對檢舉人等於不存在）。
	reporter, _ := f.newBuyer(t)
	c := f.submit(t, reporter, commentTarget(w.comment), policy.Spam).Case
	mustV[CaseDetail](t)(f.decideHide(t, c.ID, 1))
	if kindOf(submit(other, commentTarget(w.comment), policy.Spam)) != apperr.KindNotFound {
		t.Fatal("被隱藏的內容要回 NotFound")
	}
}

// storeOwnerBuyer 讓店家所屬的帳號也有一位買家：店家本人用買家身分檢舉自己店的貼文要被擋。
func (f *fixture) storeOwnerBuyer(t *testing.T, w world) int64 {
	t.Helper()
	if err := f.buyer.OnAccountCreated(context.Background(), identityCreated(w.storeAccount)); err != nil {
		t.Fatal(err)
	}
	p, err := f.buyer.Profile(context.Background(), w.storeAccount)
	if err != nil {
		t.Fatal(err)
	}
	return p.BuyerID
}
