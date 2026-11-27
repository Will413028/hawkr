//go:build integration

package turn_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"uuid"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func (f *fixture) answerExecutor() turn.AnswerExecutor {
	return turn.AnswerExecutor{
		DB: f.d, Ledger: f.ledger(), Resolver: resolver.New(taxonomy{}, resolver.Synonyms{}, func() time.Time { return now }),
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ask 讓一個 TURN run 因為「蘋果」有兩個分類而停在等待輸入，回傳提問的 run 與它的中斷。
func (f *fixture) ask(ctx context.Context) (uuid.UUID, uuid.UUID) {
	f.t.Helper()
	id := f.submit("蘋果", uitext.ZhTW)
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"item":"蘋果"}`)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		f.t.Fatal(err)
	}
	if r := f.run(id); r.Status != run.StatusWaitingForInput {
		f.t.Fatalf("前置：提問的 run = %s %v", r.Status, r.FailureReason)
	}
	irq, found, err := f.ledger().OpenInterruptForRun(ctx, buyer, id)
	if err != nil || !found {
		f.t.Fatalf("前置：中斷 = %+v %v %v", irq, found, err)
	}
	return id, irq.ID
}

func (f *fixture) resume(irq uuid.UUID, optionID, text string) uuid.UUID {
	f.t.Helper()
	id, err := run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}.Resume(context.Background(), run.ResumeInput{
		BuyerID: buyer, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: text, OptionID: optionID, Locale: uitext.ZhTW,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) runAnswer(ctx context.Context, id uuid.UUID) error {
	f.t.Helper()
	return run.Dispatcher{DB: f.d, Ledger: f.ledger(), Executors: map[string]run.Executor{run.RunTypeResume: f.answerExecutor()}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}.
		Run(ctx, run.RunArgs{RunID: id})
}

func (f *fixture) interruptStatus(id uuid.UUID) string {
	f.t.Helper()
	var s string
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT status FROM procurement.agent_run_interrupts WHERE id = $1`, id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) failure(r store.ProcurementAgentRun) string {
	if r.FailureReason == nil {
		return ""
	}
	return *r.FailureReason
}

func TestAnswerRunCommitsAnAnswerTurnResolvesTheInterruptAndSucceedsTheAskingRun(t *testing.T) { // A9-D26(3)
	ctx := context.Background()
	f := newFixture(t)
	parent, irq := f.ask(ctx)
	answer := f.resume(irq, optID(21), "")
	if f.interruptStatus(irq) != "PENDING" || f.run(parent).Status != run.StatusWaitingForInput {
		t.Fatal("送出回答時不解決中斷、不動提問的 run")
	}
	if err := f.runAnswer(ctx, answer); err != nil {
		t.Fatal(err)
	}
	r := f.run(answer)
	if r.Status != run.StatusSucceeded {
		t.Fatalf("回答 run = %s %s", r.Status, f.failure(r))
	}
	if got := f.interruptStatus(irq); got != "RESOLVED" {
		t.Errorf("中斷 = %s，要 RESOLVED", got)
	}
	if got := f.run(parent).Status; got != run.StatusSucceeded {
		t.Errorf("提問的 run = %s，要 SUCCEEDED", got)
	}
	if got := strings.Join(f.eventTypes(answer), ","); got != "run_accepted,turn_result,finished" {
		t.Errorf("events = %s", got)
	}
	if n := f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1 AND source = 'ANSWER' AND buyer_message <> '' AND buyer_message <> $3 AND client_request_id = $2`, f.request, r.ClientRequestID, optID(21)); n != 1 {
		t.Errorf("source=ANSWER 的輪次 %d 筆，要 1 筆，買家訊息是選項的標籤", n)
	}
	if n := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1 AND status = 'ACTIVE' AND item_resolution_kind = 'CATEGORY_CONFIRMED' AND item_resolution_category_id = $2`, f.request, catID(21)); n != 1 {
		t.Errorf("現行版本要存 item_resolution_category_id = 21：%d", n)
	}
	if n := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1`, f.request); n != 2 {
		t.Errorf("版本 %d 個，要 2 個（回答寫新版本）", n)
	}
	if f.used() != 1 {
		t.Errorf("used = %d，回答 run 不扣額度", f.used())
	}
	f.requireStoredTurnResult(answer)
}

func TestAnswerRunThatAsksAgainResolvesTheOldInterruptAndLeavesExactlyOnePending(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	parent, irq := f.ask(ctx)
	answer := f.resume(irq, "", "蘋果")
	if err := f.runAnswer(ctx, answer); err != nil {
		t.Fatal(err)
	}
	if r := f.run(answer); r.Status != run.StatusWaitingForInput {
		t.Fatalf("再問時回答 run = %s %s", r.Status, f.failure(r))
	}
	if f.interruptStatus(irq) != "RESOLVED" || f.run(parent).Status != run.StatusSucceeded {
		t.Error("再問也是提交點：舊中斷 RESOLVED、提問的 run SUCCEEDED")
	}
	if n := f.count(`SELECT count(*) FROM procurement.agent_run_interrupts WHERE request_id = $1 AND status = 'PENDING' AND run_id = $2`, f.request, answer); n != 1 {
		t.Errorf("新的 PENDING 中斷 %d 筆，要 1 筆且屬於回答 run", n)
	}
	if n := f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1 AND source = 'ANSWER' AND buyer_message = '蘋果'`, f.request); n != 1 {
		t.Errorf("打字的回答要以原文入輪次：%d", n)
	}
}

func TestRejectedAnswerFailsWithAnswerRejectedKeepsTheInterruptPendingAndRestoresNothing(t *testing.T) { // A9-D26(4)
	ctx := context.Background()
	f := newFixture(t)
	parent, irq := f.ask(ctx)
	before := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1`, f.request)
	turnsBefore := f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1`, f.request)
	bad := f.resume(irq, optID(999), "")
	if err := f.runAnswer(ctx, bad); err != nil {
		t.Fatal(err)
	}
	r := f.run(bad)
	if r.Status != run.StatusFailed || f.failure(r) != run.FailureAnswerRejected {
		t.Fatalf("回答 run = %s %q，要 FAILED／ANSWER_REJECTED", r.Status, f.failure(r))
	}
	if f.interruptStatus(irq) != "PENDING" || f.run(parent).Status != run.StatusWaitingForInput {
		t.Error("答案被拒：中斷維持 PENDING、提問的 run 維持等待")
	}
	if got := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1`, f.request); got != before {
		t.Errorf("不該寫版本：%d → %d", before, got)
	}
	if got := f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1`, f.request); got != turnsBefore {
		t.Errorf("不該寫輪次：%d → %d", turnsBefore, got)
	}
	if f.used() != 1 {
		t.Errorf("used = %d，失敗的回答 run 沒扣也不退額度", f.used())
	}
	if got := strings.Join(f.eventTypes(bad), ","); got != "run_accepted,finished" {
		t.Errorf("events = %s", got)
	}
	good := f.resume(irq, optID(22), "")
	if err := f.runAnswer(ctx, good); err != nil {
		t.Fatal(err)
	}
	if f.run(good).Status != run.StatusSucceeded || f.interruptStatus(irq) != "RESOLVED" {
		t.Error("被拒之後，買家用新的 client_request_id 再答要能成功")
	}
}

// waitForLockWaiter 等到有一個 backend 在等鎖（不斷言時間，只輪詢到條件成立）。
func (f *fixture) waitForLockWaiter(ctx context.Context) {
	f.t.Helper()
	for {
		if f.count(`SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid WHERE NOT l.granted AND a.datname = current_database()`) > 0 {
			return
		}
		select {
		case <-ctx.Done():
			f.t.Fatal("等不到 executor 開始等鎖")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// R6：回答 run 的提交鎖順序是提問的 run、中斷、回答的 run（ledger 的 lockAskingOf）。asking run 被別人鎖住時，
// 提交的 transaction 必須卡在 asking run 上、還沒鎖自己的 run：舊的順序（先鎖自己的 run）會讓回答 run 的列被鎖住。
func TestAnswerCommitLocksTheAskingRunBeforeItsOwnRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f := newFixture(t)
	parent, irq := f.ask(ctx)
	answer := f.resume(irq, optID(21), "")
	claimed, err := f.ledger().Transition(ctx, answer, run.StatusRunning, "")
	if err != nil {
		t.Fatal(err)
	}

	hold, err := f.d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = hold.Rollback(ctx)
		}
	}()
	if _, err := hold.Exec(ctx, `SELECT 1 FROM procurement.agent_runs WHERE id = $1 FOR UPDATE`, parent); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- f.answerExecutor().Execute(ctx, claimed) }()
	f.waitForLockWaiter(ctx)

	probe, err := f.d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Exec(ctx, `SELECT 1 FROM procurement.agent_runs WHERE id = $1 FOR UPDATE NOWAIT`, answer); err != nil {
		t.Errorf("提交在等提問的 run 時，回答 run 的列不該已被鎖：%v", err)
	}
	_ = probe.Rollback(ctx)

	if err := hold.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	released = true
	if err := <-done; err != nil && !errors.Is(err, run.ErrRunTerminal) {
		t.Fatalf("放開之後提交要成功：%v", err)
	}
	if got := f.run(answer).Status; got != run.StatusSucceeded {
		t.Errorf("回答 run = %s", got)
	}
}
