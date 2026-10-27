package cropidentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// 精確品名一輪定案、不出卡片時，確認的分類要跟著 gate 的判定走：下游的站外業者比對
// 與候選搜尋都靠 Selection 裡的分類 id。
func TestGateCarriesConfirmedCategoryForExactName(t *testing.T) {
	uc := v2UC(v2CategoryTree(), nil)
	item := "豬肉"

	got := ComputeGate(context.Background(), uc, domain.Slots{}, domain.Slots{Item: &item})

	require.True(t, got.Evaluated(), "%+v", got)
	require.True(t, got.Pending, "分類確認後待買家填單價")
	require.Equal(t, domain.CropPendingReasonUnavailable, got.PendingReason)
	id, label := got.Selection.ConfirmedCategory()
	require.Equal(t, "2008", id,
		"精確品名直接定案時，分類 id 必須跟著判定走。愈精確的買家拿到愈差的結果，就是這條路掉了分類")
	require.Equal(t, "豬肉", label)
}
