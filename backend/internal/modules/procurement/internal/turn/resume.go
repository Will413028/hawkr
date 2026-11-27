package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// MaxFreeTextRunes 是文字回答的長度上限，與 agent_run_inputs.buyer_message 的 CHECK 一致。
const MaxFreeTextRunes = 2000

// ResumeInput 是 ResumeRun 的輸入（ClarificationResponse 的 option_id 與 free_text 加上中斷與冪等鍵）。
type ResumeInput struct {
	BuyerID         int64
	InterruptID     uuid.UUID
	ClientRequestID uuid.UUID
	OptionID        string
	FreeText        string
	Locale          uitext.Locale
}

// Resumed 是 ResumeRun 的結果：回答 run 的 ID 與它當下的狀態（重送時是現況）。
type Resumed struct {
	RunID  uuid.UUID
	Status string
}

// Resumer 是 ResumeRun 的 use case（A9-D30）：同步檢查回答的形狀，再經 run.Submitter.Resume 建回答 run 並排入 job，立即回傳 run ID；分類解析在 AnswerExecutor。
type Resumer struct {
	Submitter run.Submitter
}

const (
	fieldOptionID = "OptionID"
	fieldFreeText = "FreeText"
)

// Resume 回答買家自己的中斷。不是這位買家的中斷回 NOT_FOUND；中斷不在 PENDING（已回答、已取消）不看回答形狀，一律交給 run.Submitter.Resume（重送回原回答 run，否則 APPROVAL_ALREADY_RESOLVED／RUN_NOT_ACTIVE），同一個中斷不論輸入都得到同一個代碼；
// PENDING 時回答形狀不合回 INVALID_ARGUMENT 並指出欄位（以 CheckAnswer 為準：free_text 先去頭尾空白，option_id 與非空白文字恰一個，只有空白的文字加選項算只答選項，選項在待回答的卡上；另加不超過 2000 字元），沒有新增 ErrorReason；其餘（同需求進行中、上限）依 run.Submitter.Resume。
func (r Resumer) Resume(ctx context.Context, in ResumeInput) (Resumed, error) {
	q := store.New(r.Submitter.DB.Conn(ctx))
	irq, err := q.GetInterruptForBuyer(ctx, store.GetInterruptForBuyerParams{ID: in.InterruptID, BuyerID: in.BuyerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Resumed{}, apperr.NotFound("找不到這個中斷")
	}
	if err != nil {
		return Resumed{}, fmt.Errorf("turn：讀取中斷：%w", err)
	}
	if irq.Status == "PENDING" {
		var p InterruptPayload
		if err := json.Unmarshal(irq.Payload, &p); err != nil {
			return Resumed{}, fmt.Errorf("turn：解析中斷 payload：%w", err)
		}
		if err := checkShape(p, in); err != nil {
			return Resumed{}, err
		}
	}
	id, err := r.Submitter.Resume(ctx, run.ResumeInput{
		BuyerID: in.BuyerID, InterruptID: in.InterruptID, ClientRequestID: in.ClientRequestID,
		Message: trimtext.Trim(in.FreeText), OptionID: in.OptionID, Locale: in.Locale,
	})
	if err != nil {
		return Resumed{}, err
	}
	got, err := q.GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: id, BuyerID: in.BuyerID})
	if err != nil {
		return Resumed{}, fmt.Errorf("turn：讀取回答 run：%w", err)
	}
	return Resumed{RunID: got.ID, Status: got.Status}, nil
}

func checkShape(p InterruptPayload, in ResumeInput) error {
	if utf8.RuneCountInString(trimtext.Trim(in.FreeText)) > MaxFreeTextRunes {
		return apperr.InvalidInputOn([]string{fieldFreeText}, "回答的文字不能超過 %d 個字元", MaxFreeTextRunes)
	}
	switch CheckAnswer(p, Answer{OptionID: in.OptionID, FreeText: in.FreeText}) {
	case "":
		return nil
	case InvalidNeedExactlyOne:
		return apperr.InvalidInputOn([]string{fieldOptionID, fieldFreeText}, "要選一個選項，或輸入文字")
	case InvalidBlankText:
		return apperr.InvalidInputOn([]string{fieldFreeText}, "回答的文字不能是空白")
	case InvalidOptionNotOnCard:
		return apperr.InvalidInputOn([]string{fieldOptionID}, "這個選項不在待回答的問題上")
	case InvalidUnknownStage, InvalidResolutionReject:
	}
	return apperr.InvalidInputOn([]string{fieldOptionID, fieldFreeText}, "這個問題無法用這個回答接續")
}
