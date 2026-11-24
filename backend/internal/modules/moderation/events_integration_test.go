//go:build integration

package moderation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type recorded[E any] struct {
	mu  sync.Mutex
	got []E
}

func record[E any](f *fixture) *recorded[E] {
	r := &recorded[E]{}
	events.Subscribe(f.bus, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.got = append(r.got, e)
		return nil
	})
	return r
}

func (r *recorded[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]E(nil), r.got...)
}

func TestSubmitPublishesReportReceivedOnlyForANewReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	received := record[ReportReceived](f)
	reporter, _ := f.newBuyer(t)
	rid := uuid.NewV4()
	first := f.submitWith(t, reporter, commentTarget(w.comment), policy.Harassment, &rid)
	got := received.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.ReportID != first.Report.ID || e.CaseID != first.Case.ID || e.Target != commentTarget(w.comment) || e.ReporterBuyerID != reporter || e.ReporterStoreID != 0 ||
		e.Category != policy.Harassment || e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v report=%+v", e, first.Report)
	}
	if replay := f.submitWith(t, reporter, commentTarget(w.comment), policy.Harassment, &rid); replay.Created || len(received.all()) != 1 {
		t.Fatalf("同一個 request_id 重送不得再發事件：%+v events=%d", replay, len(received.all()))
	}
	if merged := f.submit(t, reporter, commentTarget(w.comment), policy.Harassment); merged.Created || len(received.all()) != 1 {
		t.Fatalf("併入自己既有的檢舉不得再發事件：%+v events=%d", merged, len(received.all()))
	}
	other, _ := f.newBuyer(t)
	second := f.submit(t, other, commentTarget(w.comment), policy.Spam)
	if all := received.all(); len(all) != 2 || all[1].ReportID != second.Report.ID || all[1].CaseID != first.Case.ID || all[1].ReporterBuyerID != other || all[1].Category != policy.Spam {
		t.Fatalf("別的檢舉人掛進同一案是新的檢舉：%+v", all)
	}
}

func TestSubmitEventFailureRollsBackTheReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	events.Subscribe(f.bus, func(context.Context, ReportReceived) error { return errors.New("reject") })
	reporter, _ := f.newBuyer(t)
	if _, err := f.module.SubmitReport(context.Background(), SubmitInput{ReporterBuyerID: reporter, Target: commentTarget(w.comment), Category: policy.Spam}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.reports"); n != 0 {
		t.Fatalf("reports=%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases"); n != 0 {
		t.Fatalf("cases=%d", n)
	}
}

func TestDecidePublishesCaseClosedWithOutcomeAndCategoryOnly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	closed := record[CaseClosedEvent](f)

	violation := f.openCase(t, commentTarget(w.comment))
	mustV[CaseDetail](t)(f.decide(t, violation, ActionHide, policy.Harassment))
	got := closed.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.CaseID != violation.ID || e.TargetKind != TargetComment || e.Outcome != OutcomeViolation || e.Category == nil || *e.Category != policy.Harassment ||
		e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v", e)
	}

	post := f.post(t, w.storeID, "另一篇")
	clean := f.comment(t, post.ID, w.author, "沒問題的留言")
	noViolation := f.openCase(t, commentTarget(clean))
	mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{
		CaseID: noViolation.ID, Version: noViolation.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation,
	}))
	all := closed.all()
	if len(all) != 2 || all[1].CaseID != noViolation.ID || all[1].Outcome != OutcomeNoViolation || all[1].Category != nil {
		t.Fatalf("不違規的結案不帶政策分類：%+v", all)
	}
}

func TestStaleDecidePublishesNoCaseClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	closed := record[CaseClosedEvent](f)
	c := f.openCase(t, commentTarget(w.comment))
	f.bumpVersion(t, c.ID)
	if _, err := f.decide(t, c, ActionHide, policy.Spam); err == nil {
		t.Fatal("版本過期要失敗")
	}
	if n := len(closed.all()); n != 0 {
		t.Fatalf("events=%d", n)
	}
}

func TestCaseClosedEventFailureRollsBackTheDecisionAndTheHide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	events.Subscribe(f.bus, func(context.Context, CaseClosedEvent) error { return errors.New("reject") })
	c := f.openCase(t, commentTarget(w.comment))
	if _, err := f.decide(t, c, ActionHide, policy.Spam); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND closed_at IS NULL", c.ID); n != 1 {
		t.Fatalf("案件要維持進行中：%d", n)
	}
	if !f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatal("隱藏要跟著回滾")
	}
}

func TestCaseReportersPagesEveryReporterOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	ctx := context.Background()
	want := map[int64]uuid.UUID{}
	var c Case
	for range 5 {
		reporter, _ := f.newBuyer(t)
		res := f.submit(t, reporter, commentTarget(w.comment), policy.Spam)
		want[reporter], c = res.Report.ID, res.Case
		f.advance(1)
	}
	detail := mustV[CaseDetail](t)(f.module.Case(ctx, c.ID))
	var ordered []int64
	for _, r := range detail.Reports {
		ordered = append(ordered, r.ReporterBuyerID)
	}
	var got []int64
	var after *ReportCursor
	pages := 0
	for {
		page := mustV[ReporterPage](t)(f.module.CaseReporters(ctx, c.ID, after, 2))
		pages++
		for _, r := range page.Items {
			if r.StoreID != 0 || want[r.BuyerID] != r.ReportID {
				t.Fatalf("reporter=%+v", r)
			}
			got = append(got, r.BuyerID)
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if pages != 3 || len(got) != len(ordered) {
		t.Fatalf("pages=%d got=%v ordered=%v", pages, got, ordered)
	}
	for i := range got {
		if got[i] != ordered[i] {
			t.Fatalf("順序要同檢舉時間：got=%v ordered=%v", got, ordered)
		}
	}
	if empty := mustV[ReporterPage](t)(f.module.CaseReporters(ctx, uuid.NewV4(), nil, 10)); len(empty.Items) != 0 || empty.Next != nil {
		t.Fatalf("不存在的案件回空頁：%+v", empty)
	}
}
