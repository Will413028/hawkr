//go:build integration

package procurement

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type fakeJobs struct{ n int64 }

func (j *fakeJobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	j.n++
	return j.n, nil
}

func (j *fakeJobs) CancelJob(context.Context, int64) (rivertype.JobState, bool, error) {
	return rivertype.JobStateRunning, true, nil
}

func runtimeModule(t *testing.T) (*Module, *db.DB) {
	t.Helper()
	d := newDB(t)
	m := New(Deps{DB: d, Now: func() time.Time { return epoch }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Jobs: &fakeJobs{}, Events: events.NewBus()})
	return m, d
}

const itemCard = `{"stage":"ITEM","query":"蘋果","candidates":[{"categoryId":"00000000-0000-7000-8000-000000000021","label":"蘋果","path":"水果 > 蘋果"}],` +
	`"question":{"slotKey":"SLOT_KEY_ITEM_DISAMBIGUATION","prompt":"你說的是哪一個？","options":[{"optionId":"category-00000000-0000-7000-8000-000000000021","label":"蘋果"},{"optionId":"item-other","label":"其他"}],"allowFreeText":true}}`

func ask(ctx context.Context, t *testing.T, d *db.DB, c chain) uuid.UUID {
	t.Helper()
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'WAITING_FOR_INPUT' WHERE id = $1`, c.run); err != nil {
		t.Fatal(err)
	}
	irq := id()
	if err := store.New(d.Conn(ctx)).InsertInterrupt(ctx, store.InsertInterruptParams{ID: irq, RunID: c.run, Payload: []byte(itemCard), Now: epoch}); err != nil {
		t.Fatal(err)
	}
	return irq
}

func runsOf(m model.RuntimeSnapshot) []uuid.UUID {
	var out []uuid.UUID
	for _, r := range m.Runs {
		out = append(out, r.RunID)
	}
	return out
}

func TestGetRuntimeSnapshotMirrorsTheConversation(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, true)
	s, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	if s.ConversationID != c.conversation || s.RequestID != c.request || s.Status != "CLARIFYING" {
		t.Errorf("識別與狀態：%+v", s)
	}
	if len(s.Messages) != 2 || s.Messages[0].Role != "USER" || s.Messages[0].Text != "我要三件外套" ||
		s.Messages[1].Role != "ASSISTANT" || s.Messages[1].Text != "好的" || s.Messages[1].ID != c.turn {
		t.Errorf("每一輪拆成買家與助理兩則：%+v", s.Messages)
	}
	if got := runsOf(s); !slices.Equal(got, []uuid.UUID{c.run}) {
		t.Errorf("runs = %v", got)
	}
	if r := s.Runs[0]; r.RunType != "TURN" || r.Status != "QUEUED" || r.Terminal || r.LastCursor != "" {
		t.Errorf("run 摘要：%+v", r)
	}
	if s.ActiveRunID == nil || *s.ActiveRunID != c.run {
		t.Errorf("進行中的 run = %v", s.ActiveRunID)
	}
	if s.OpenConfirmation != nil {
		t.Errorf("沒有中斷：%+v", s.OpenConfirmation)
	}
	p := s.PublishedRfq
	if p == nil || p.PublicationID != c.publication || p.RecipientCount != 1 || p.QueuedCount != 1 || p.Version != 1 {
		t.Errorf("發布結果：%+v", p)
	}
}

func TestGetRuntimeSnapshotWithoutPublicationOrRunsOmitsThem(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'SUCCEEDED', completed_at = $2 WHERE id = $1`, c.run, epoch); err != nil {
		t.Fatal(err)
	}
	s, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	if s.PublishedRfq != nil || s.ActiveRunID != nil || s.OpenConfirmation != nil {
		t.Errorf("沒有發布、進行中的 run 與中斷時都不填：%+v", s)
	}
	if len(s.Runs) != 1 || !s.Runs[0].Terminal {
		t.Errorf("終態的 run 仍列出並標 terminal：%+v", s.Runs)
	}
}

func TestGetRuntimeSnapshotIsNotFoundForAnotherBuyersOrDeletedConversation(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	q := store.New(d.Conn(ctx))
	c := seed(ctx, t, q, 7, false)
	if _, err := m.GetRuntimeSnapshot(ctx, 8, c.conversation); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("別的買家：%v，要 NOT_FOUND", err)
	}
	if _, err := m.GetRuntimeSnapshot(ctx, 7, id()); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("不存在：%v，要 NOT_FOUND", err)
	}
	if _, err := q.SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: c.conversation, BuyerID: 7, Now: &epoch}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("已刪除：%v，要 NOT_FOUND", err)
	}
}

func TestSnapshotRunsAreTheCurrentRequestsNewestFirstAtMostTwenty(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	q := store.New(d.Conn(ctx))
	c := seed(ctx, t, q, 7, false)
	other := seed(ctx, t, q, 7, false)
	ids := []uuid.UUID{c.run}
	for i := 1; i <= 24; i++ {
		rid := id()
		if n, err := q.InsertRun(ctx, store.InsertRunParams{ID: rid, BuyerID: 7, RequestID: c.request, ConversationID: &c.conversation, RunType: "TURN", ClientRequestID: id(), QuotaPeriodStart: &epoch, Now: epoch.Add(time.Duration(i) * time.Second)}); err != nil || n != 1 {
			t.Fatal(n, err)
		}
		ids = append(ids, rid)
	}
	s, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(ids[len(ids)-20:])
	slices.Reverse(want)
	if got := runsOf(s); !slices.Equal(got, want) {
		t.Errorf("runs = %v\n要最新的 20 筆、新到舊 %v", got, want)
	}
	if slices.Contains(runsOf(s), other.run) {
		t.Error("別的需求的 run 不得出現")
	}
	so, err := m.GetRuntimeSnapshot(ctx, 7, other.conversation)
	if err != nil || len(so.Runs) != 1 || so.Runs[0].RunID != other.run {
		t.Errorf("另一個需求只有自己的 run：%+v %v", so.Runs, err)
	}
}

func TestSnapshotOpenConfirmationShowsProcessingWhileAnAnswerRunIsInProgress(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	q := store.New(d.Conn(ctx))
	c := seed(ctx, t, q, 7, false)
	irq := ask(ctx, t, d, c)
	s, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	oc := s.OpenConfirmation
	if oc == nil || oc.InterruptID != irq || oc.Kind != "CLARIFICATION" || oc.Processing {
		t.Fatalf("等回答：%+v", oc)
	}
	if oc.Question.SlotKey != "ITEM_DISAMBIGUATION" || oc.Question.Prompt != "你說的是哪一個？" || !oc.Question.AllowFreeText || len(oc.Question.Options) != 2 || oc.Question.Options[0].OptionID != "category-00000000-0000-7000-8000-000000000021" {
		t.Errorf("問題要還原同一個畫面：%+v", oc.Question)
	}
	if s.ActiveRunID != nil {
		t.Errorf("等待輸入的 run 不算進行中：%v", s.ActiveRunID)
	}

	answer := id()
	if n, err := q.InsertRun(ctx, store.InsertRunParams{ID: answer, BuyerID: 7, RequestID: c.request, ConversationID: &c.conversation, InterruptID: &irq, RunType: "CLARIFICATION_RESUME", ClientRequestID: id(), Now: epoch.Add(time.Second)}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	s, err = m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	if s.OpenConfirmation == nil || !s.OpenConfirmation.Processing || s.ActiveRunID == nil || *s.ActiveRunID != answer {
		t.Errorf("有回答 run 進行中：processing = %+v、active = %v", s.OpenConfirmation, s.ActiveRunID)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'FAILED', failure_reason = 'ANSWER_REJECTED', completed_at = $2 WHERE id = $1`, answer, epoch); err != nil {
		t.Fatal(err)
	}
	s, err = m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	if s.OpenConfirmation == nil || s.OpenConfirmation.Processing || s.ActiveRunID != nil {
		t.Errorf("回答 run 失敗後回到等回答：%+v、active = %v", s.OpenConfirmation, s.ActiveRunID)
	}
}

func TestResumeRunRejectsABadShapeWithoutCreatingARun(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	runs := func() int { return len(must(m.GetRuntimeSnapshot(ctx, 7, c.conversation)).Runs) }
	before := runs()
	for name, tc := range map[string]struct {
		in     ResumeRunInput
		fields []string
	}{
		"兩個都給":   {ResumeRunInput{OptionID: "category-00000000-0000-7000-8000-000000000021", FreeText: "蘋果"}, []string{"OptionID", "FreeText"}},
		"都沒給":    {ResumeRunInput{}, []string{"OptionID", "FreeText"}},
		"空白文字":   {ResumeRunInput{FreeText: " 　 "}, []string{"FreeText"}},
		"選項不在卡上": {ResumeRunInput{OptionID: "category-999"}, []string{"OptionID"}},
		"文字太長":   {ResumeRunInput{FreeText: strings.Repeat("字", 2001)}, []string{"FreeText"}},
	} {
		in := tc.in
		in.BuyerID, in.InterruptID, in.ClientRequestID, in.Locale = 7, irq, id(), uitext.ZhTW
		_, err := m.ResumeRun(ctx, in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
			t.Errorf("%s：%v，要 INVALID_ARGUMENT", name, err)
			continue
		}
		var got []string
		for _, v := range ae.FieldViolations() {
			got = append(got, v.Field)
		}
		if !slices.Equal(got, tc.fields) || ae.Reason != (apperr.Reason{}) {
			t.Errorf("%s：欄位 %v、reason %v，要 %v 且沒有 reason", name, got, ae.Reason, tc.fields)
		}
	}
	if after := runs(); after != before {
		t.Errorf("形狀不合不得建 run：%d → %d", before, after)
	}
	if _, err := m.ResumeRun(ctx, ResumeRunInput{BuyerID: 8, InterruptID: irq, ClientRequestID: id(), OptionID: "category-00000000-0000-7000-8000-000000000021", Locale: uitext.ZhTW}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("別的買家的中斷：%v，要 NOT_FOUND", err)
	}
}

func TestResumeRunReportsAnAnsweredInterruptTheSameWhateverTheInput(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_run_interrupts SET status = 'RESOLVED', closed_at = now() WHERE id = $1`, irq); err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]ResumeRunInput{
		"卡外選項": {OptionID: "category-999"},
		"都沒給":  {},
		"卡上選項": {OptionID: "category-00000000-0000-7000-8000-000000000021"},
	} {
		in.BuyerID, in.InterruptID, in.ClientRequestID, in.Locale = 7, irq, id(), uitext.ZhTW
		_, err := m.ResumeRun(ctx, in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Reason != apperr.ReasonApprovalAlreadyResolved {
			t.Errorf("%s：%v，要 APPROVAL_ALREADY_RESOLVED", name, err)
		}
	}
}

func TestResumeRunTreatsAnOptionBesideBlankTextAsOptionOnly(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	got, err := m.ResumeRun(ctx, ResumeRunInput{BuyerID: 7, InterruptID: irq, ClientRequestID: id(), OptionID: "category-00000000-0000-7000-8000-000000000021", FreeText: " ", Locale: uitext.ZhTW})
	if err != nil || got.Status != "QUEUED" {
		t.Errorf("選項加空白文字算只答選項：%+v %v", got, err)
	}
}

func TestResumeRunQueuesTheAnswerRunAndReturnsItImmediately(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	in := ResumeRunInput{BuyerID: 7, InterruptID: irq, ClientRequestID: id(), OptionID: "category-00000000-0000-7000-8000-000000000021", Locale: uitext.ZhTW}
	got, err := m.ResumeRun(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "QUEUED" {
		t.Errorf("status = %s，立即回傳時回答 run 還在排隊", got.Status)
	}
	var option *string
	var runType string
	if err := d.Pool().QueryRow(ctx, `SELECT i.option_id, r.run_type FROM procurement.agent_run_inputs i JOIN procurement.agent_runs r ON r.id = i.run_id WHERE i.run_id = $1`, got.RunID).Scan(&option, &runType); err != nil {
		t.Fatal(err)
	}
	if option == nil || *option != "category-00000000-0000-7000-8000-000000000021" || runType != "CLARIFICATION_RESUME" {
		t.Errorf("輸入 = %v %s", option, runType)
	}
	if again, err := m.ResumeRun(ctx, in); err != nil || again.RunID != got.RunID {
		t.Errorf("重送回同一個回答 run：%+v %v", again, err)
	}
}

func TestResumeRunResendAfterTheAnswerRunSucceededReplaysTheOriginalRun(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	in := ResumeRunInput{BuyerID: 7, InterruptID: irq, ClientRequestID: id(), OptionID: "category-00000000-0000-7000-8000-000000000021", Locale: uitext.ZhTW}
	first, err := m.ResumeRun(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'SUCCEEDED', completed_at = $2 WHERE id = $1`, first.RunID, epoch); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_run_interrupts SET status = 'RESOLVED', closed_at = $2 WHERE id = $1`, irq, epoch); err != nil {
		t.Fatal(err)
	}
	again, err := m.ResumeRun(ctx, in)
	if err != nil || again.RunID != first.RunID {
		t.Errorf("中斷已 RESOLVED 後重送同一個請求要回原回答 run，不是 APPROVAL_ALREADY_RESOLVED：%+v %v", again, err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
