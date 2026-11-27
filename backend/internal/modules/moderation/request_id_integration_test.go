//go:build integration

package moderation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) submitWith(t *testing.T, reporter int64, target Target, category policy.Category, requestID *uuid.UUID) SubmitResult {
	t.Helper()
	return mustV[SubmitResult](t)(f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: target, Category: category, RequestID: requestID}))
}

// 同一位檢舉人的同一個 request_id 一律回第一次的結果，不看案件狀態：進行中、結案後都一樣，不開新案件、不重複寫檢舉、不吃每日額度（A7-D28 第 3 點）。
func TestRequestIDResendReturnsTheFirstResultEvenAfterTheCaseIsClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	rid := uuid.New()
	first := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &rid)
	if !first.Created {
		t.Fatalf("第一次要建立：%+v", first)
	}
	open := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &rid)
	if open.Created || open.Report.ID != first.Report.ID || !open.Report.CreatedAt.Equal(first.Report.CreatedAt) {
		t.Fatalf("進行中重送要回第一次的結果：%+v", open)
	}
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{CaseID: first.Case.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation})))
	closed := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &rid)
	if closed.Created || closed.Report.ID != first.Report.ID || closed.Case.ID != first.Case.ID {
		t.Fatalf("結案後重送也要回第一次的結果，不開新案件：%+v", closed)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE reporter_buyer_id = $1", reporter); n != 1 {
		t.Fatalf("檢舉 %d 筆", n)
	}
	if n := f.count(t, "SELECT submitted FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = $1", reporter); n != 1 {
		t.Fatalf("每日額度用了 %d", n)
	}
}

// 同一個 request_id、內容不同是 InvalidInput；request_id 的範圍是檢舉人，別人用同一個值是各自的檢舉。
func TestRequestIDReuseWithDifferentContentIsRejectedAndIsScopedToTheReporter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	other, _ := f.newBuyer(t)
	rid := uuid.New()
	first := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &rid)
	_, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Fraud, RequestID: &rid})
	if kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("內容不同要回 InvalidInput：%v", err)
	}
	theirs := f.submitWith(t, other, commentTarget(w.comment), policy.Spam, &rid)
	if !theirs.Created || theirs.Report.ID == first.Report.ID {
		t.Fatalf("別的檢舉人用同一個 request_id 是新的檢舉：%+v", theirs)
	}
}

// 不同 request_id：進行中的案件重複檢舉回原檢舉的 ID；結案後再檢舉開新案件。
func TestADifferentRequestIDKeepsTheOpenCaseRuleAndAClosedCaseOpensANewOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	r1, r2, r3 := uuid.New(), uuid.New(), uuid.New()
	first := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &r1)
	dup := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &r2)
	if dup.Created || dup.Report.ID != first.Report.ID {
		t.Fatalf("進行中重複檢舉要回原檢舉：%+v", dup)
	}
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{CaseID: first.Case.ID, Version: 1, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation})))
	next := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &r3)
	if !next.Created || next.Case.ID == first.Case.ID || next.Report.ID == first.Report.ID {
		t.Fatalf("結案後再檢舉要開新案件：%+v", next)
	}
}

// 並行重送同一個 request_id：只有一筆檢舉、一個案件、一份額度，每個呼叫都回同一筆。
func TestConcurrentResendsOfOneRequestIDYieldOneReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	rid := uuid.New()
	const n = 8
	ids := make([]uuid.UUID, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			res, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam, RequestID: &rid})
			ids[i], errs[i] = res.Report.ID, err
		})
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("第 %d 個：%v %s（預期 %s）", i, errs[i], ids[i], ids[0])
		}
	}
	if c := f.count(t, "SELECT count(*) FROM moderation.reports WHERE reporter_buyer_id = $1", reporter); c != 1 {
		t.Fatalf("檢舉 %d 筆", c)
	}
}

// ListMyReports 只回呼叫者自己的檢舉，新到舊、keyset 分頁；結果由案件推導（進行中、違規、不違規），不含案件 ID。
func TestMyReportsListsOnlyTheCallersReportsPagedWithDerivedOutcomes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	other, _ := f.newBuyer(t)
	ctx := context.Background()

	c1 := f.comment(t, w.post.ID, w.author, "留言一")
	c2 := f.comment(t, w.post.ID, w.author, "留言二")
	r1 := f.submit(t, reporter, commentTarget(c1), policy.Spam)
	f.advance(time.Second)
	r2 := f.submit(t, reporter, commentTarget(c2), policy.Harassment)
	f.advance(time.Second)
	r3 := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
	f.submit(t, other, commentTarget(w.comment), policy.Spam)
	must(t, errOf(f.module.DecideCase(ctx, DecideInput{CaseID: r1.Case.ID, Version: r1.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation})))
	d, err := f.decideHide(t, r2.Case.ID, r2.Case.Version)
	must(t, err)
	if d.Case.ClosedAt == nil {
		t.Fatal("要結案")
	}

	p1, err := f.module.MyReports(ctx, reporter, nil, 2)
	must(t, err)
	if len(p1.Items) != 2 || p1.Next == nil || p1.Items[0].Report.ID != r3.Report.ID || p1.Items[1].Report.ID != r2.Report.ID {
		t.Fatalf("第一頁要是最新兩筆且有下一頁：%+v", p1)
	}
	if p1.Items[0].Report.Outcome != OutcomeNone || p1.Items[0].ClosedAt != nil || p1.Items[1].Report.Outcome != OutcomeViolation || p1.Items[1].ClosedAt == nil {
		t.Fatalf("結果要由案件推導：%+v", p1.Items)
	}
	p2, err := f.module.MyReports(ctx, reporter, p1.Next, 2)
	must(t, err)
	if len(p2.Items) != 1 || p2.Next != nil || p2.Items[0].Report.ID != r1.Report.ID || p2.Items[0].Report.Outcome != OutcomeNoViolation || p2.Items[0].Target != commentTarget(c1) {
		t.Fatalf("第二頁：%+v", p2)
	}
	theirs, err := f.module.MyReports(ctx, other, nil, 50)
	must(t, err)
	if len(theirs.Items) != 1 || theirs.Items[0].Report.ReporterBuyerID != other {
		t.Fatalf("別人只看到自己的：%+v", theirs)
	}
}

// 先不帶 request_id 送出 A，再用 R2 對同一對象送出（併入進行中案件的 A）：R2 也要記下；結案後以 R2 重送仍回 A，不開新案件。
func TestRequestIDOnADedupedReportIsRecordedAndSurvivesTheCaseClosing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	r2 := uuid.New()
	a := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, nil)
	deduped := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &r2)
	if deduped.Created || deduped.Report.ID != a.Report.ID {
		t.Fatalf("進行中重複檢舉要回 A：%+v", deduped)
	}
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{CaseID: a.Case.ID, Version: a.Case.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation})))
	again := f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &r2)
	if again.Created || again.Report.ID != a.Report.ID || again.Case.ID != a.Case.ID {
		t.Fatalf("結案後以 R2 重送要回 A，不開新案件：%+v", again)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports WHERE reporter_buyer_id = $1", reporter); n != 1 {
		t.Fatalf("檢舉 %d 筆", n)
	}
}

// 併入既有檢舉的路徑也比對內容：同一個 request_id 先用內容 X 記下，再用內容不同的 Y 重送是 InvalidInput，不是成功。
func TestRequestIDWithDifferentContentIsRejectedOnTheDedupPathToo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	rid := uuid.New()
	f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, nil)
	f.submitWith(t, reporter, commentTarget(w.comment), policy.Spam, &rid)
	_, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Fraud, RequestID: &rid})
	if kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("內容不同要回 InvalidInput：%v", err)
	}
}

// 負的頁大小是 InvalidInput，錯誤指到欄位 PageSize（handler 對應到請求的 page_size）。
func TestMyReportsRejectsANegativePageSizeOnThePageSizeField(t *testing.T) {
	t.Parallel()
	f := setup(t)
	reporter, _ := f.newBuyer(t)
	_, err := f.module.MyReports(context.Background(), reporter, nil, -1)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "PageSize" {
		t.Fatalf("err = %v，預期 InvalidInput 指到 PageSize", err)
	}
}
