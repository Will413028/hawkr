// services/ai-service/internal/procurement/usecase/provisioning.go
package usecase

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// ---------------------------------------------------------------------------
// 採購助手 LLM 未開通時的停用實作（backend#3329）
//
// 「開通了沒」的判定與 client 建構都不在這裡——它們收斂在 internal/llmprovider，
// 理由見該套件的 doc comment（一句話：判定與建構被複製成 N 份、而「記得參加」
// 是 opt-in 的，已經各咬過一次）。本檔只剩各介面的 fail-closed 停用實作，
// 建構子拿到未開通的 llmprovider.Procurement 時回傳它們。
//
// 三支的共同原則：**不得回「零值 ＋ nil error」**。停用要能跟「模型正常運作但
// 這次沒話說」區分開，否則沒有人會去修。
// ---------------------------------------------------------------------------

// ErrProcurementLLMDisabled 是本 package 對 llmprovider.ErrDisabled 的別名。
// 定義只有一份（llmprovider），這裡保留具名是為了讓 usecase 內的閱讀者不必
// 跨套件才知道回的是什麼。
var ErrProcurementLLMDisabled = llmprovider.ErrDisabled

// disabledSlotExtractor 是 SlotExtractor 的 fail-closed 停用實作。
//
// 🔴 **不得**回「空的 SlotPatch ＋ nil error」。那是這一支最危險的退化方向：
// 空 patch 是抽取器的**合法輸出**（使用者那句話沒帶新資訊時本來就回空），
// 呼叫端會照常續行、問下一題 ⇒ 買家會經歷一段「問什麼都沒反應」的對話，
// 而 log 與指標上完全看不出 LLM 根本沒接上。
type disabledSlotExtractor struct{}

func (disabledSlotExtractor) Extract(context.Context, string, domain.Slots) (ExtractResult, error) {
	return ExtractResult{}, ErrProcurementLLMDisabled
}

// disabledStepNarrator 是 StepNarrator 的 fail-closed 停用實作。
//
// 🔴 同理不得回空字串：敘述句為空在 UI 上就是「少一行字」，沒有人會發現。
type disabledStepNarrator struct{}

func (disabledStepNarrator) Narrate(context.Context, domain.StepCode, string) (string, error) {
	return "", ErrProcurementLLMDisabled
}

// disabledSurfaceRealizer 是 SurfaceRealizer 的 fail-closed 停用實作。
//
// 與另外兩支不同的是：它的錯誤**預期會每輪發生**（採購對話只要走到敘述層就會
// 呼叫一次），所以 realizeOrFallback 對 ErrProcurementLLMDisabled 刻意不寫 log
// ——停用這件事已經由 llmprovider 在啟動時講清楚（含 impact 與 fix），每輪再吼
// 一次只會把「非預期失敗」淹掉，那正是那行 log 要守的東西。
type disabledSurfaceRealizer struct{}

func (disabledSurfaceRealizer) Realize(context.Context, SurfaceContext) ([]RealizedBlock, error) {
	return nil, ErrProcurementLLMDisabled
}
