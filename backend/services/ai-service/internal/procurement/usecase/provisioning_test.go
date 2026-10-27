package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// StepNarrator 那一側同樣要降級 —— 四個建構子共用 llmprovider 的判定，
// 但共用是實作細節，契約要各自驗過（少驗一個，那一支改壞了沒有東西會叫）。
func TestNewStepNarrator_DegradesFailClosed(t *testing.T) {
	n := NewStepNarrator(llmprovider.NewProcurement(&config.Config{ExtractorModel: "gemini-2.5-flash"}, zap.NewNop()))
	require.IsType(t, disabledStepNarrator{}, n)

	// 🔴 陰性斷言：不得回空字串。敘述句為空在 UI 上就是「少一行字」，沒有人會發現。
	out, callErr := n.Narrate(t.Context(), "", "摘要")
	require.Empty(t, out)
	require.Error(t, callErr, "停用實作必須在呼叫時 fail-closed")
	assertProcurementDisabled(t, callErr)
}
