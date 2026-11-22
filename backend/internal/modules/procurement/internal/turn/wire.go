package turn

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/reply"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"uuid"
)

// 這個檔案是 turn_result 事件與中斷 payload 的線格式：proto JSON 的手工對應（模組不能 import httpapi/gen，arch/check.go），
// 欄位名與值名依 api/hawkr/procurement/v1alpha1/procurement.proto 的 TurnResult 與 ClarifyQuestion。
// internal/httpapi 下的 wire_test 以 protojson（DiscardUnknown=false）解析它們，擋住欄位漂移。

type slotsJSON struct {
	Item               *string         `json:"item,omitempty"`
	Quantity           *int32          `json:"quantity,omitempty"`
	Unit               *string         `json:"unit,omitempty"`
	Packaging          *string         `json:"packaging,omitempty"`
	BudgetPerUnit      *string         `json:"budgetPerUnit,omitempty"` // int64 在 proto JSON 是字串
	DeliveryCity       *string         `json:"deliveryCity,omitempty"`
	DeliveryDate       *dateJSON       `json:"deliveryDate,omitempty"`
	ExactMatchRequired *bool           `json:"exactMatchRequired,omitempty"`
	ItemResolution     *resolutionJSON `json:"itemResolution,omitempty"`
}

type dateJSON struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type resolutionJSON struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	DisplayValue string `json:"displayValue"`
}

type rejectedJSON struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type recordedJSON struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type blockJSON struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type optionJSON struct {
	OptionID string `json:"optionId"`
	Label    string `json:"label"`
}

// QuestionJSON 是 ClarifyQuestion 的 proto JSON。
type QuestionJSON struct {
	SlotKey       string       `json:"slotKey"`
	Prompt        string       `json:"prompt"`
	Options       []optionJSON `json:"options,omitempty"`
	UnitOptions   []optionJSON `json:"unitOptions,omitempty"`
	AllowFreeText bool         `json:"allowFreeText,omitempty"`
	CanSkip       bool         `json:"canSkip,omitempty"`
}

type interruptJSON struct {
	InterruptID string `json:"interruptId"`
	Kind        string `json:"kind"`
}

type turnResultJSON struct {
	RequestID        string         `json:"requestId"`
	ConversationID   string         `json:"conversationId"`
	RevisionID       string         `json:"revisionId"`
	Slots            slotsJSON      `json:"slots"`
	Rejected         []rejectedJSON `json:"rejected,omitempty"`
	MissingRequired  []string       `json:"missingRequired,omitempty"`
	RecordedThisTurn []recordedJSON `json:"recordedThisTurn,omitempty"`
	Reply            string         `json:"reply"`
	Question         *QuestionJSON  `json:"question,omitempty"`
	Blocks           []blockJSON    `json:"blocks,omitempty"`
	TextFallback     string         `json:"textFallback"`
	MessageID        string         `json:"messageId"`
	CreateTime       string         `json:"createTime"`
	RunID            string         `json:"runId"`
	Interrupt        *interruptJSON `json:"interrupt,omitempty"`
}

const slotKeyPrefix = "SLOT_KEY_"

func slotKey(k model.SlotKey) string { return slotKeyPrefix + string(k) }

func slotsWire(s slots.Set) slotsJSON {
	out := slotsJSON{
		Item: s.Item, Quantity: s.Quantity, Unit: s.Unit, Packaging: s.Packaging, DeliveryCity: s.DeliveryCity, ExactMatchRequired: s.ExactMatchRequired,
	}
	if s.BudgetPerUnit != nil {
		v := fmt.Sprintf("%d", *s.BudgetPerUnit)
		out.BudgetPerUnit = &v
	}
	if s.DeliveryDate != nil {
		out.DeliveryDate = &dateJSON{Year: s.DeliveryDate.Year(), Month: int(s.DeliveryDate.Month()), Day: s.DeliveryDate.Day()}
	}
	if s.Resolution != nil {
		out.ItemResolution = &resolutionJSON{Kind: "ITEM_RESOLUTION_KIND_" + string(s.Resolution.Kind), Label: s.Resolution.Label, DisplayValue: s.Resolution.DisplayValue()}
	}
	return out
}

func questionWire(q *reply.RenderedQuestion) *QuestionJSON {
	if q == nil {
		return nil
	}
	conv := func(in []reply.RenderedOption) []optionJSON {
		var out []optionJSON
		for _, o := range in {
			out = append(out, optionJSON{OptionID: o.ID, Label: o.Label})
		}
		return out
	}
	return &QuestionJSON{
		SlotKey: slotKey(q.Slot), Prompt: q.Prompt, Options: conv(q.Options), UnitOptions: conv(q.UnitOptions),
		AllowFreeText: q.AllowFreeText, CanSkip: q.CanSkip,
	}
}

// turnResultEvent 組 turn_result 事件的 payload（{"turn_result": {...}}）。interruptID 非零時是停在等待輸入的一輪。
func turnResultEvent(d decision, ids ids, now time.Time, interruptID uuid.UUID) ([]byte, error) {
	tr := turnResultJSON{
		RequestID: ids.request.String(), ConversationID: ids.conversation.String(), RevisionID: ids.revision.String(),
		Slots: slotsWire(d.next), Reply: d.replyText, Question: questionWire(d.question),
		TextFallback: d.replyText, MessageID: ids.turn.String(), CreateTime: now.UTC().Format(time.RFC3339Nano), RunID: ids.run.String(),
	}
	for _, r := range d.rejected {
		name := r.Reason.ProtoName()
		if name == "" {
			return nil, fmt.Errorf("turn：沒有對應的 proto 拒絕原因：%q", r.Reason)
		}
		tr.Rejected = append(tr.Rejected, rejectedJSON{Key: slotKey(r.Key), Reason: name})
	}
	for _, k := range d.missing {
		tr.MissingRequired = append(tr.MissingRequired, slotKey(k))
	}
	tr.RecordedThisTurn = d.recorded
	for _, b := range d.blocks {
		tr.Blocks = append(tr.Blocks, blockJSON{Kind: "BLOCK_KIND_" + string(b.Kind), Text: b.Text})
	}
	if interruptID != (uuid.UUID{}) {
		tr.Interrupt = &interruptJSON{InterruptID: interruptID.String(), Kind: "INTERRUPT_KIND_CLARIFICATION"}
	}
	b, err := json.Marshal(map[string]any{run.EventTurnResult: tr})
	if err != nil {
		return nil, fmt.Errorf("turn：組 turn_result 事件：%w", err)
	}
	return b, nil
}

// 中斷 payload 的階段。
const (
	StageItem = "ITEM"
)

// InterruptPayload 是品項消歧中斷的 payload，也是接續的唯一狀態（A9-D24(2)、A9-D28(1)）。
// Question 是當時對買家顯示的那張卡（已依送出語系渲染），讀取模型（OpenConfirmation.question）直接用它還原同一個畫面；
// Candidates 是卡上選項背後的分類，回答時以選項 ID（category-<id>）對回分類，不信任 client 傳的分類 ID。
type InterruptPayload struct {
	Stage      string             `json:"stage"`
	Query      string             `json:"query"`
	Candidates []PayloadCandidate `json:"candidates"`
	Question   QuestionJSON       `json:"question"`
}

// PayloadCandidate 是卡上的一個候選分類。
type PayloadCandidate struct {
	CategoryID int64  `json:"categoryId,string"`
	Label      string `json:"label"`
	Path       string `json:"path"`
}
