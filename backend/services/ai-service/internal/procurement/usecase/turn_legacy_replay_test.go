// services/ai-service/internal/procurement/usecase/turn_legacy_replay_test.go
package usecase

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
)

// TestReplayResult_LegacyRowWithoutBlocks_FallsBackToSingleLegacyBlock 確保
// Phase 2 上線前寫入（只有 reply，沒有 blocks）的舊資料列 replay 時不會回傳
// 空 blocks——比照前端 legacyReplyToBlocks 的邏輯，包成單一 legacy block。
func TestReplayResult_LegacyRowWithoutBlocks_FallsBackToSingleLegacyBlock(t *testing.T) {
	legacyPayload, err := json.Marshal(map[string]any{
		"message":  "我要訂高麗菜",
		"reply":    "已收到你的訊息。",
		"rejected": []any{},
	}) // 刻意不含 "blocks" key，模擬 Phase 2 上線前的舊列
	require.NoError(t, err)

	result, err := replayResult(repository.IdempotentResult{
		RequestID: 1, ConversationID: 1, RevisionID: 1,
		Slots:       domain.Slots{},
		PayloadJSON: legacyPayload,
	})
	require.NoError(t, err)
	require.Len(t, result.Blocks, 1)
	require.Equal(t, domain.BlockUnderstood, result.Blocks[0].Kind)
	require.Equal(t, "已收到你的訊息。", result.Blocks[0].Text)
	require.Equal(t, "已收到你的訊息。", result.Reply)
}

func TestReplayResult_RowWithBlocks_ReturnsThemDirectly(t *testing.T) {
	payload, err := json.Marshal(turnPayload{
		Reply:    "已記錄：品項：寶島梨 還想請你補充：配送城市",
		Rejected: []domain.RejectedField{},
		Blocks: []domain.Block{
			{Kind: domain.BlockUnderstood, Text: "已記錄：品項：寶島梨"},
			{Kind: domain.BlockNextStep, Text: "還想請你補充：配送城市"},
		},
	})
	require.NoError(t, err)

	result, err := replayResult(repository.IdempotentResult{
		RequestID: 1, ConversationID: 1, RevisionID: 1,
		Slots:       domain.Slots{},
		PayloadJSON: payload,
	})
	require.NoError(t, err)
	require.Len(t, result.Blocks, 2)
	require.Equal(t, domain.BlockNextStep, result.Blocks[1].Kind)
}

// TestReplayResult_LegacyRowWithoutRenderMode_DefaultsToTemplate 是 final
// review finding 3 的回歸測試：Phase 3 上線前寫入的舊列沒有 renderMode
// key，unmarshal 出來是空字串——replayResult 必須把它預設回 "TEMPLATE"，
// 不能讓 toTurnResultProto 把這個空字串原樣送上 wire（那是修這個 bug 之前
// 的行為：Phase 2 一律寫死 "TEMPLATE"，這裡的預設值是延續同一個保證）。
func TestReplayResult_LegacyRowWithoutRenderMode_DefaultsToTemplate(t *testing.T) {
	legacyPayload, err := json.Marshal(map[string]any{
		"message":  "我要訂高麗菜",
		"reply":    "已收到你的訊息。",
		"rejected": []any{},
	}) // 刻意不含 "renderMode" key，模擬 Phase 3 上線前的舊列
	require.NoError(t, err)

	result, err := replayResult(repository.IdempotentResult{
		RequestID: 1, ConversationID: 1, RevisionID: 1,
		Slots:       domain.Slots{},
		PayloadJSON: legacyPayload,
	})
	require.NoError(t, err)
	require.Equal(t, "TEMPLATE", result.RenderMode)
}

// TestReplayResult_RowWithModelRenderMode_ReplaysAsModel 涵蓋 final review
// finding 3 指出「完全沒有 MODEL 模式重播測試」的缺口：持久化的 renderMode
// 若明確是 "MODEL"，重播必須原樣保留，不能被上面新增的預設邏輯誤判成
// "TEMPLATE"，也不能重新呼叫 realizer。
func TestReplayResult_RowWithModelRenderMode_ReplaysAsModel(t *testing.T) {
	payload, err := json.Marshal(turnPayload{
		Reply:    "報價單已經確認好了。",
		Rejected: []domain.RejectedField{},
		Blocks: []domain.Block{
			{Kind: domain.BlockComplete, Text: "報價單已經確認好了。"},
		},
		RenderMode: "MODEL",
	})
	require.NoError(t, err)

	result, err := replayResult(repository.IdempotentResult{
		RequestID: 1, ConversationID: 1, RevisionID: 1,
		Slots:       domain.Slots{},
		PayloadJSON: payload,
	})
	require.NoError(t, err)
	require.Equal(t, "MODEL", result.RenderMode)
}
