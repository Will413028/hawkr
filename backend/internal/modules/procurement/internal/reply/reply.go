// Package reply 把規劃結果（planner）與代碼（slots）換成買家看的文字：每個 act 一個模板，文字全在模組的 text 目錄，沒有 LLM 敘述。
package reply

import (
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// BlockKind 是敘述段落的語意分類，值等於 proto BlockKind 的值名（去掉前綴）。
type BlockKind string

// 段落種類。
const (
	Understood BlockKind = "UNDERSTOOD"
	Context    BlockKind = "CONTEXT"
	Rationale  BlockKind = "RATIONALE"
	Warning    BlockKind = "WARNING"
	NextStep   BlockKind = "NEXT_STEP"
	Complete   BlockKind = "COMPLETE"
)

// Block 是一個型別化的敘述段落。
type Block struct {
	Kind BlockKind
	Text string
}

// Args 是模板要的資料。Missing 與 Rejected 是這一輪的必填缺項與被拒欄位；問句與本輪記下的事實在 Plan 裡。
type Args struct {
	Missing  []model.SlotKey
	Rejected []slots.Rejection
}

// Render 依 act 選模板組出段落。act 是封閉集合，沒有「找不到模板」的退路：每個 act 都有測試窮舉。
func Render(l uitext.Locale, p planner.Plan, a Args) []Block {
	switch p.Act {
	case planner.AckAndAskRequired:
		var blocks []Block
		if len(p.Salient) > 0 {
			blocks = append(blocks, Block{Understood, text.ReplyAckRequiredRecorded(l, text.ReplyAckRequiredRecordedArgs{Facts: join(l, facts(l, p.Salient))}).Text})
		} else {
			blocks = append(blocks, Block{Understood, text.ReplyAckRequiredReceived(l).Text})
		}
		if len(a.Missing) > 0 {
			labels := make([]string, 0, len(a.Missing))
			for _, k := range a.Missing {
				labels = append(labels, SlotLabel(l, k))
			}
			blocks = append(blocks, Block{NextStep, text.ReplyAckRequiredMissing(l, text.ReplyAckRequiredMissingArgs{Missing: join(l, labels)}).Text})
		}
		return blocks
	case planner.AckAndAskOptional:
		return []Block{
			{Understood, text.ReplyAckOptionalUnderstood(l).Text},
			{NextStep, questionPrompt(l, p)},
		}
	case planner.CorrectionApplied:
		return []Block{{Understood, text.ReplyCorrectionUpdated(l, text.ReplyCorrectionUpdatedArgs{Facts: join(l, facts(l, p.Salient))}).Text}}
	case planner.InputRejected:
		items := make([]string, 0, len(a.Rejected))
		for _, r := range a.Rejected {
			items = append(items, pair(l, SlotLabel(l, r.Key), RejectionReason(l, r)))
		}
		return []Block{
			{Warning, text.ReplyRejectedWarning(l, text.ReplyRejectedWarningArgs{Items: join(l, items)}).Text},
			{NextStep, text.ReplyRejectedNext(l).Text},
		}
	case planner.ExtractionDegraded:
		return []Block{
			{Warning, text.ReplyDegradedWarning(l).Text},
			{Context, text.ReplyDegradedContext(l).Text},
		}
	case planner.RequestBasicsReady:
		return []Block{
			{Understood, text.ReplyBasicsReadyUnderstood(l).Text},
			{NextStep, text.ReplyBasicsReadyNext(l).Text},
		}
	case planner.PricePreviewAvailable:
		return []Block{
			{Context, text.ReplyPricePreviewContext(l).Text},
			{NextStep, text.ReplyPricePreviewNext(l).Text},
		}
	case planner.RfqReconfirmRequired:
		return []Block{
			{Warning, text.ReplyReconfirmWarning(l).Text},
			{NextStep, text.ReplyReconfirmNext(l).Text},
		}
	case planner.PublicationApprovalRequired:
		return []Block{
			{Complete, text.ReplyApprovalComplete(l).Text},
			{Rationale, text.ReplyApprovalRationale(l).Text},
		}
	case planner.ItemNeedsDisambiguation:
		return []Block{{NextStep, questionPrompt(l, p)}}
	}
	return nil
}

// TextFallback 把段落依序以空格接成純文字，供 text_fallback 與對話紀錄的回覆文字用：唯一的接法，兩處不各自維護。
func TextFallback(blocks []Block) string {
	texts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		texts = append(texts, b.Text)
	}
	return strings.Join(texts, " ")
}

func questionPrompt(l uitext.Locale, p planner.Plan) string {
	if p.Question == nil {
		return ""
	}
	return Prompt(l, p.Question.Prompt)
}

func facts(l uitext.Locale, recorded []slots.Recorded) []string {
	out := make([]string, 0, len(recorded))
	for _, r := range recorded {
		out = append(out, pair(l, SlotLabel(l, r.Key), RecordedValue(l, r)))
	}
	return out
}

func pair(l uitext.Locale, label, content string) string {
	return text.FactPair(l, text.FactPairArgs{Label: label, Content: content}).Text
}

func join(l uitext.Locale, items []string) string {
	return strings.Join(items, text.ListSeparator(l).Text)
}
