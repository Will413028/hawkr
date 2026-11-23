package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AnswerExecutor 是 CLARIFICATION_RESUME 類型 run 的 executor（A9-D26、A9-D30）：把回答套到被回答中斷的 payload（ApplyAnswer，沒有模型呼叫），
// 成功時走與 TURN 相同的 commit 寫一個 source = ANSWER 的新版本，提交點（Ledger 的 commitAnswer）同 transaction 解決中斷與提問的 run。
// 答案被拒絕時 run 以 ANSWER_REJECTED 失敗，中斷維持 PENDING，不還原任何東西。不扣額度（run.ChargesQuota）。
type AnswerExecutor struct {
	DB       *db.DB
	Ledger   run.Ledger
	Resolver Resolver
	Now      func() time.Time
	Logger   *slog.Logger
}

var _ run.Executor = AnswerExecutor{}

// Execute 執行一個 CLARIFICATION_RESUME run。回傳 nil 之前 run 已 SUCCEEDED 或 WAITING_FOR_INPUT（又問了下一張卡）；錯誤由 Dispatcher 轉 FAILED。
func (a AnswerExecutor) Execute(ctx context.Context, r store.ProcurementAgentRun) error {
	e := Executor{DB: a.DB, Ledger: a.Ledger, Resolver: a.Resolver, Now: a.Now, Logger: a.Logger}
	if r.InterruptID == nil {
		return fmt.Errorf("turn：回答 run %s 沒有 interrupt_id", r.ID)
	}
	ld, err := e.load(ctx, r)
	if err != nil {
		return err
	}
	irq, err := store.New(e.DB.Conn(ctx)).GetInterruptForBuyer(ctx, store.GetInterruptForBuyerParams{ID: *r.InterruptID, BuyerID: r.BuyerID})
	if err != nil {
		return fmt.Errorf("turn：讀取被回答的中斷：%w", err)
	}
	var p InterruptPayload
	if err := json.Unmarshal(irq.Payload, &p); err != nil {
		return fmt.Errorf("turn：解析中斷 payload：%w", err)
	}
	now := e.Now()
	out, err := ApplyAnswer(ctx, e.Resolver, ld.locale, now, p, Answer{OptionID: ld.optionID, FreeText: ld.message})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d, err := decideAnswer(ld.locale, now, ld.prior, out)
	if err != nil {
		return err
	}
	ld.source = sourceAnswer
	ld.message = answerText(p, ld.optionID, ld.message)
	return e.commit(ctx, ld, d, now)
}

// decideAnswer 把 ApplyAnswer 的結果接成這一輪的 decision：解析成功時只帶品項解析的 patch，再問時沿用新卡；只有 OutcomeInvalid 是 ANSWER_REJECTED，其他結果是一般錯誤。
func decideAnswer(l uitext.Locale, now time.Time, prior priorState, out Outcome) (decision, error) {
	switch out.Kind {
	case OutcomeResolved:
		patch := slots.Patch{Resolution: out.Resolution}
		next, rejected := prior.current.Apply(patch, now)
		if next.Resolution == nil {
			return decision{}, &run.FailureError{Reason: run.FailureAnswerRejected, Err: errors.New("turn：品項解析沒有被需求收下")}
		}
		return finish(l, prior, patch, next, rejected, nil), nil
	case OutcomeInterrupt:
		if out.card == nil {
			return decision{}, fmt.Errorf("turn：再問的結果沒有卡")
		}
		return finish(l, prior, slots.Patch{}, prior.current, nil, out.card), nil
	case OutcomeInvalid:
		return decision{}, &run.FailureError{Reason: run.FailureAnswerRejected, Err: fmt.Errorf("turn：答案被拒：%s", out.Reason)}
	}
	return decision{}, fmt.Errorf("turn：未知的回答結果：%d", out.Kind)
}

// answerText 是對話裡顯示的買家回答：選項是卡上的標籤，文字是去頭尾空白後的原文。
func answerText(p InterruptPayload, optionID, freeText string) string {
	if optionID == "" {
		return trimtext.Trim(freeText)
	}
	for _, o := range p.Question.Options {
		if o.OptionID == optionID {
			return o.Label
		}
	}
	return optionID
}
