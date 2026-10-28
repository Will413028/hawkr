package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	actorpkg "github.com/Will413028/hawkr/ai-service/internal/actor"
	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"go.uber.org/zap"
)

// DefaultMessages 是按鈕端點的預設訊息（orders 已改為直接查詢，不再使用 LLM）。
// Exported 是因為 internal/grpc 的 10 支通用 streaming RPC 沿用同一張查表（見
// assistant_handler.go），不在該 package 重新複製一份常數。
var DefaultMessages = map[string]string{
	"inventory": "檢查庫存狀態",
	"payment":   "查詢本週收款狀況",
}

// ButtonSpecialistKey 把按鈕 endpointKey 對映到 Runners.Specialists 的 map key（role:specKey）。
// Exported 是因為 internal/grpc 的通用 streaming RPC 沿用同一張表。
var ButtonSpecialistKey = map[string]string{
	"inventory":    "seller:inventory",
	"payment":      "seller:payment",
	"orders":       "buyer:order",
	"tracking":     "buyer:tracking",
	"seller-match": "buyer:seller_match",
}

// ChatRunner 回傳 chat 端點該用的 runner。是 package-level 函式讓 internal/grpc 的
// 通用 streaming RPC 也能呼叫，不需要持有 HTTP handler；可獨立單元測試（不需 API key）。
//
// all-tools fallback 已於 2026-09-20 移除，見
// docs/adr/2026-09-20-coordinator-vs-all-tools-fallback.md：實測 28 條 query，
// coordinator 27/28 而 fallback 20/28，且 fallback 的主要失敗是**一個 tool 都不叫、
// 直接用 prompt 回答**——作為緊急備援的價值是負的，使用者分不出那是真資料還是模型
// 自己講的。cfg 參數保留在簽章上：呼叫端已經持有它，日後若要依設定選 runner 不必
// 改所有呼叫點。
func ChatRunner(_ *config.Config, runners *agentpkg.Runners, role string) *runner.Runner {
	return runners.Coordinators[role]
}

// ErrRunnerNotReady／ErrSessionCreate 是 RunAssistantStream 兩種「呼叫前就能判定的
// 失敗」的 sentinel error，讓呼叫端用 errors.Is 分辨並映回各自的錯誤文案／
// gRPC status code——這兩種失敗原因不同，不能共用同一句訊息（行為不變）。
var (
	ErrRunnerNotReady = errors.New("assistant runner not ready")
	ErrSessionCreate  = errors.New("failed to create session")
)

// RunAssistantStream 建立 session（若未提供）並執行 ADK runner，把事件轉譯後推給
// sink（gRPC server-stream 用 internal/grpc.GRPCEventSink；HTTP SSE 版
// sseEventSink 已隨 Phase C 移除 ai-service HTTP business surface 刪除）。
// act 是呼叫端已解析好的現任 store_id 或 buyer_id 行為者，不是原始登入 account_id。
//
// 回傳值語意：只有「呼叫前就能判定的失敗」（r == nil → ErrRunnerNotReady、建立
// session 失敗 → ErrSessionCreate）才回傳非 nil error，這種情況 sink 完全不會被
// 呼叫，呼叫端要把它當成真正的錯誤處理（對應不同的 gRPC status code）。ADK runner
// 執行中途的錯誤透過 sink.Error(...) 送出真正的 "error" 事件，已經在函式內部處理
// 完，函式仍回傳 nil。
func RunAssistantStream(ctx context.Context, sessionSvc session.Service, sink EventSink, r *runner.Runner, act actorpkg.Actor, sessionID string, userMessage *genai.Content) error {
	// 防護：runner map 查表若 key 打錯會回 nil，早回錯誤而非在 r.Run 時 panic。
	if r == nil {
		pkglogger.Error(ctx, "[Assistant] runner 未就緒（nil）", zap.String("stateKey", act.StateKey))
		return ErrRunnerNotReady
	}
	userID := act.UserID
	if sessionID == "" {
		// stateKey 的值必須以字串寫入，不能是裸 int64——adk-go 的 database session
		// service（google.golang.org/adk/v2/session/database, stateMap.Scan）把
		// state 序列化進 Postgres JSONB 後，讀回時用 encoding/json.Unmarshal 進
		// map[string]any，JSON 數字一律變成 float64，19 位 TSID 遠超 float64 的
		// 53-bit 精確整數上限（2^53）會被靜默改寫成鄰近值且不可逆——寫入端一律先轉字串
		// 才能讓 JSON 往返後精確不失真（讀取端見 tools.go 的 getInt64State）。
		createResp, err := sessionSvc.Create(ctx, &session.CreateRequest{
			AppName: string(agentpkg.AppNameAssistant),
			UserID:  userID,
			State:   map[string]any{act.StateKey: act.StateVal},
		})
		if err != nil {
			pkglogger.Error(ctx, "[Assistant] 建立 session 失敗", zap.Error(err))
			return fmt.Errorf("%w: %w", ErrSessionCreate, err)
		}
		sessionID = createResp.Session.ID()
	}

	// 執行 agent
	// ADK StreamingModeSSE 會先 yield 多個 Partial=true chunk，
	// 最後再 yield 一個 Partial=false 完整回應。
	// 以 sentText 追蹤本輪已送出的內容；Partial=false 到達時只補送尚未送出的 suffix，
	// 避免重複，同時確保 Partial=true chunks 未涵蓋的尾端內容（如引導問句）能送達。
	// 工具回應（FunctionResponse）代表新一輪文字生成開始，重置 sentText。
	var sentText strings.Builder
	for event, err := range r.Run(ctx, userID, sessionID, userMessage, adkagent.RunConfig{
		StreamingMode: adkagent.StreamingModeSSE,
	}) {
		if ctx.Err() != nil {
			pkglogger.Info(ctx, "[Assistant] client disconnected")
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			pkglogger.Warn(ctx, "[Assistant] runner error", zap.Error(err))
			if sentText.Len() > 0 {
				sink.Error(500, "回應中斷，請稍後再試。")
			} else {
				sink.Error(500, "查詢失敗，請稍後再試。")
			}
			break
		}
		if event == nil || event.Content == nil {
			continue
		}

		// 先處理 function response（例：update_draft → draft_patch SSE event）
		// 必須在文字處理之前，避免被 Partial 判斷過濾掉。
		hasFuncResp := false
		for _, part := range event.Content.Parts {
			if part.FunctionResponse == nil {
				continue
			}
			hasFuncResp = true
			switch part.FunctionResponse.Name {
			case "update_draft":
				if patchRaw, ok := part.FunctionResponse.Response["patch"]; ok {
					if patchJSON, err := json.Marshal(patchRaw); err == nil {
						sink.DraftPatch(string(patchJSON))
					}
				}
			case "search_candidate_sellers":
				// candidatesRaw 的欄位形狀對應 agent.CandidateSellerResult 的 json tag（id/category/product_summary/...）。
				if candidatesRaw, ok := part.FunctionResponse.Response["candidates"]; ok {
					resultJSON, err := json.Marshal(map[string]any{
						"type":       "seller_candidates",
						"candidates": candidatesRaw,
					})
					if err == nil {
						sink.ToolResult(string(resultJSON))
					}
				}
			}
		}
		// 工具執行完畢，新一輪文字生成開始，重置追蹤器
		if hasFuncResp {
			sentText.Reset()
		}

		// 處理文字部分
		if !event.Partial && sentText.Len() > 0 {
			// Partial=false：組出本事件的完整文字，只補送尚未送出的 suffix
			var full strings.Builder
			for _, part := range event.Content.Parts {
				if part.Thought {
					// assistant 目前無 provider 產 Thought part；一旦有（開 thinking
					// 的新 provider）也不得以一般文字外流（#2447 BE-01A latent guard）。
					continue
				}
				full.WriteString(part.Text)
			}
			already := sentText.String()
			fullStr := full.String()
			if strings.HasPrefix(fullStr, already) {
				if tail := fullStr[len(already):]; tail != "" {
					sink.Text(tail)
				}
			}
		} else {
			for _, part := range event.Content.Parts {
				if part.Thought {
					continue // 同上：Thought part 不轉發（#2447 BE-01A）
				}
				if part.Text != "" {
					sink.Text(part.Text)
					sentText.WriteString(part.Text)
				}
			}
		}
	}

	// done event 回傳 sessionId（與 REST /assistant/sessions/latest 同名，10 檔 §2）
	sink.Done(sessionID)
	return nil
}

// ValidWeightUnits 合法的 WeightUnit CodeMeta 值。Exported 是因為 internal/grpc
// 的 SellerProductAutofill 沿用同一張表做 SKU 單位驗證（同 DefaultMessages／
// ButtonSpecialistKey 的 export 模式），不在該 package 重新複製一份常數。
var ValidWeightUnits = map[string]bool{
	"KG":    true,
	"CATTY": true,
	"GRAM":  true,
	"":      true,
}

// LatestSessionMessage 是 FetchLatestSession 的訊息項（role/text/timestamp）。
type LatestSessionMessage struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// FetchLatestSession 列出 actor 的所有 session，回傳最新一筆的完整歷史。沒有
// 對話時回傳 sessionID="" 與空列表。唯一呼叫端是 internal/grpc 的
// SellerLatestSession／BuyerLatestSession（HTTP 版 GetLatestSession 已隨 Phase C
// 刪除）。
func FetchLatestSession(ctx context.Context, sessionSvc session.Service, act actorpkg.Actor) (sessionID string, messages []LatestSessionMessage, err error) {
	userID := act.UserID

	// 列出所有 session，找最新的
	listResp, err := sessionSvc.List(ctx, &session.ListRequest{
		AppName: string(agentpkg.AppNameAssistant),
		UserID:  userID,
	})
	if err != nil {
		pkglogger.Error(ctx, "[Assistant] 取得 session 列表失敗", zap.Error(err))
		return "", nil, err
	}

	// 沒有對話 → 回空
	if len(listResp.Sessions) == 0 {
		return "", []LatestSessionMessage{}, nil
	}

	// 找最新的 session
	latest := listResp.Sessions[0]
	for _, s := range listResp.Sessions[1:] {
		if s.LastUpdateTime().After(latest.LastUpdateTime()) {
			latest = s
		}
	}

	// 取得該 session 的歷史
	getResp, err := sessionSvc.Get(ctx, &session.GetRequest{
		AppName:   string(agentpkg.AppNameAssistant),
		UserID:    userID,
		SessionID: latest.ID(),
	})
	if err != nil {
		pkglogger.Error(ctx, "[Assistant] 取得 session 失敗", zap.Error(err))
		return "", nil, err
	}

	msgs := make([]LatestSessionMessage, 0)
	for event := range getResp.Session.Events().All() {
		if event.Content == nil {
			continue
		}
		var text string
		for _, part := range event.Content.Parts {
			if part.Thought {
				// ADK runner 持久化的是完整 event（含 Thought parts）——live stream
				// 的 guard（RunAssistantStream）擋不住這條歷史回放面，這裡要再擋
				// 一次（#2447 BE-01A latent guard 的鏡像面）。
				continue
			}
			if part.Text != "" {
				text += part.Text
			}
		}
		if text == "" {
			continue
		}

		role := "model"
		if event.Content.Role == string(genai.RoleUser) {
			role = "user"
		}

		msgs = append(msgs, LatestSessionMessage{
			Role:      role,
			Text:      text,
			Timestamp: timeutil.FormatISO8601(event.Timestamp),
		})
	}

	return latest.ID(), msgs, nil
}
