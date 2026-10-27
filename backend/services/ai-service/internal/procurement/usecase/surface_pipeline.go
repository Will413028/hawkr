package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// recentNarrativeLimit 是 anti-repetition context 最多帶幾則「已經說過的
// 內容」——issue #2549 §D：「只讀目前 branch 已 commit 的最近 1–2 則 public
// transcript」。
const recentNarrativeLimit = 2

// buildRecentNarratives 從呼叫端手上已有的 history 取最近（最多 2 筆）已持久化
// 的回覆文字，最新的在前。
//
// 刻意不新增任何查詢：三個 compose 點都已經持有 history（graph_terminal.go:135
// 的參數、answer.go:211、rewind.go:108），previousOptionalStreak（turn.go:507）
// 已經示範從 PatchPayloadJSON 解出 turnPayload。被刪版本的 RecentNarrativeReader
// ＋ ListAnswerPatchesByRequestID 每輪多打一次不分頁查詢、撈整包再取尾 2 筆，
// 沒有必要。
//
// 單筆解析失敗跳過該筆而不是整包放棄：anti-repetition 是品質加分項，不是正確性
// 必要條件。
func buildRecentNarratives(history []repository.RevisionWithPatch) []string {
	if len(history) == 0 {
		return nil
	}
	start := len(history) - recentNarrativeLimit
	if start < 0 {
		start = 0
	}
	tail := history[start:]

	out := make([]string, 0, len(tail))
	for i := len(tail) - 1; i >= 0; i-- { // 最新的在前
		if len(tail[i].PatchPayloadJSON) == 0 {
			continue
		}
		var p turnPayload
		if err := json.Unmarshal(tail[i].PatchPayloadJSON, &p); err != nil {
			continue
		}
		if p.Reply == "" {
			continue
		}
		out = append(out, p.Reply)
	}
	return out
}

// realizeOrFallback 是三個 compose 點共用的唯一進入點：先試 SurfaceRealizer，
// 過 validateSurfaceBlocks 三層驗證，任一步失敗（nil realizer／realize error／
// timeout／empty／schema error／validator reject）一律整輪退回
// domain.RenderPlan（純函式、不會失敗）。回傳的 renderMode 只有 "MODEL"／
// "TEMPLATE" 兩個合法值。
//
// committed 是這一輪定案的 slots（/turns 與 /answers 的 next、/rewinds 的
// targetSlots），只往下傳給 fact-echo 當允許值域——模型看不到它，也不影響
// domain.RenderPlan 的輸出。理由見 factEchoViolated 的 doc comment。
func realizeOrFallback(
	ctx context.Context,
	realizer SurfaceRealizer,
	plan domain.DialoguePlanV1,
	args domain.SafeRenderArgs,
	committed domain.Slots,
	recent []string,
) ([]domain.Block, string) {
	if realizer == nil {
		return domain.RenderPlan(plan, args), "TEMPLATE"
	}

	realized, err := realizer.Realize(ctx, SurfaceContext{Plan: plan, Args: args, RecentNarratives: recent})
	if err != nil {
		observability.RecordProcurementSurfaceRealize(ctx, "model", "error")
		observability.RecordProcurementSurfaceFallback(ctx, string(plan.ActCode))
		// 停用實作（#3329 graceful degradation）不寫 log：它每一輪都會回這個錯，
		// 而「尚未開通」這件事 llmprovider 已經在啟動時講清楚（含 impact 與
		// fix）。每輪再吼一次會把下面那種非預期失敗淹掉，那正是這行 log 要守的。
		if errors.Is(err, ErrProcurementLLMDisabled) {
			return domain.RenderPlan(plan, args), "TEMPLATE"
		}
		// err 在這裡是唯一的記錄機會：退場之後它就被丟棄，回應與 metric 都只留下
		// 「退回了模板」這個結果。逾時／HTTP 400／JSON 解析失敗塌縮成同一個
		// result="error" 序列，沒有這行就分不出是哪一種——2026-08-31 復原時的 C1
		// （手刻 client 漏 reasoning.effort=none 造成 provider 一律回 400）正是
		// 100% 走這條路且完全靜默。
		pkglogger.Warn(ctx, "procurement: surface realization failed, falling back to template",
			zap.String("act_code", string(plan.ActCode)),
			zap.Error(err))
		return domain.RenderPlan(plan, args), "TEMPLATE"
	}

	ok, reason := validateSurfaceBlocks(plan, args, committed, realized)
	if !ok {
		observability.RecordProcurementSurfaceRealize(ctx, "model", "reject")
		observability.RecordProcurementSurfaceValidationReject(ctx, reason)
		observability.RecordProcurementSurfaceFallback(ctx, string(plan.ActCode))
		// 被擋的文字只活到這一行：退場之後 realized 被丟棄，wire 上只剩模板輸出。
		// 沒有它，validation_reject_total 只能回答「擋了多少」，不能回答「擋了
		// 什麼」——而收緊或放寬任何一層（第二層按 act 分層、第三層的允許值域）
		// 都需要後者。2026-09-11 dev 實測 PRICE_PREVIEW_AVAILABLE 有 1/3 退場率，
		// 那次能診斷出根因是因為樣本小到可以用推理補上；攔截率一旦上升，這個
		// 方法就失效。
		//
		// 不取樣：reject 是例外路徑（dev 實測 31 次 realize 只有 2 次），取樣只會
		// 在最需要證據的那一筆把它丟掉。ClaimCodes 一起記——第一層的
		// claim_violation 是對「模型自報了什麼」的判定，只看 Text 看不出原因。
		pkglogger.Warn(ctx, "procurement: surface realization rejected, falling back to template",
			zap.String("reason", reason),
			zap.String("act_code", string(plan.ActCode)),
			zap.Strings("rejected_blocks", rejectedBlockDigests(realized)))
		return domain.RenderPlan(plan, args), "TEMPLATE"
	}

	observability.RecordProcurementSurfaceRealize(ctx, "model", "success")
	blocks := make([]domain.Block, len(realized))
	for i, rb := range realized {
		// ClaimCodes 不上 wire：它只是驗證階段的輸入，不是對外契約的一部分。
		blocks[i] = domain.Block{Kind: rb.Kind, Text: rb.Text}
	}
	return blocks, "MODEL"
}

// rejectedBlockTextLimit 是每個 block 進 log 的字元上限。模型輸出的單一段落
// 正常在 60 字以內（prompt 有長度約束，too_long 那條規則也會先擋掉超長的），
// 這個上限只是防禦異常輸出撐爆單筆 log。
const rejectedBlockTextLimit = 200

// rejectedBlockDigests 把被擋下的模型輸出攤成可讀字串，供 realizeOrFallback 的
// reject 分支記錄。格式為 "<kind>|<claimCodes 以逗號相連>|<text>"。
//
// 以 rune 而非 byte 截斷：文字是中文，byte 截斷會切在多位元組字元中間，log 裡
// 出現的替代字元會讓人誤以為模型輸出本身壞掉。
func rejectedBlockDigests(blocks []RealizedBlock) []string {
	digests := make([]string, 0, len(blocks))
	for _, b := range blocks {
		text := []rune(b.Text)
		if len(text) > rejectedBlockTextLimit {
			text = append(text[:rejectedBlockTextLimit:rejectedBlockTextLimit], '…')
		}
		digests = append(digests, string(b.Kind)+"|"+strings.Join(b.ClaimCodes, ",")+"|"+string(text))
	}
	return digests
}
