// services/ai-service/internal/procurement/usecase/graph_nodes_summary_test.go
//
// summarizeCropGate／summarizeValidateSlots／summarizeSlotsRejected 是
// graph_nodes.go 的未匯出符號，測試留在這個 package——crop_gate_test.go 搬進
// cropidentity 時，這幾支測試跨了群組邊界（narration summary 是 runtime 關注，
// 不是品項身分解析本身），因此從原檔剪下貼到這裡。
package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func item(v string) *string { return &v }

// TestSummarizeValidateSlots_CarriesThisTurnDelta 釘住重複文案的真正修法：
// 摘要必須帶上「這一輪記下了什麼」。兩輪之間 slots 終態相同、只有本輪新記的
// 欄位不同時，餵給 LLM 的摘要就必須不同——否則模型逐字產出同一句。
func TestSummarizeValidateSlots_CarriesThisTurnDelta(t *testing.T) {
	next := domain.Slots{Item: item("高麗菜")}

	cityTurn := summarizeValidateSlots(next, nil, []domain.RecordedField{
		{Key: "delivery_city", Label: "配送城市", Value: "桃園市"},
	})
	quantityTurn := summarizeValidateSlots(next, nil, []domain.RecordedField{
		{Key: "quantity", Label: "數量", Value: "50"},
	})

	require.Contains(t, cityTurn, "配送城市")
	require.Contains(t, cityTurn, "桃園市")
	require.Contains(t, quantityTurn, "數量")
	require.NotEqual(t, cityTurn, quantityTurn,
		"兩輪記下的東西不同，摘要就必須不同；相同的話 LLM 沒有任何差異可講，買家每輪看到同一句")

	idle := summarizeValidateSlots(next, nil, nil)
	require.NotEqual(t, cityTurn, idle, "有記下東西與什麼都沒記下，必須是兩種摘要")
}

// TestSummarizeCropGate_ResolveFailedHasItsOwnSentence 守住降級路徑不得被講成
// 「判斷已完成」——那一輪不但沒判定成功，既有的解析結果還被清空了。
func TestSummarizeCropGate_ResolveFailedHasItsOwnSentence(t *testing.T) {
	failed := summarizeCropGate(cropidentity.GateResult{Item: item("番茄"), ResolveFailed: true})
	done := summarizeCropGate(cropidentity.GateResult{Item: item("番茄")})

	require.NotEqual(t, failed, done)
	require.NotContains(t, failed, "已完成")
	require.Contains(t, done, "已完成")
}

// TestSummarizeSlotsRejected 釘住 SLOTS_REJECTED 的摘要真的帶上「擱了什麼、
// 為什麼」——固定文案「有幾個條件先擱著」兩者都沒說，而這是買家剛被退回輸入
// 的那一輪最需要知道的事。
func TestSummarizeSlotsRejected(t *testing.T) {
	t.Run("欄位名走 SlotLabels，不外洩 slot key，理由原樣帶上", func(t *testing.T) {
		got := summarizeSlotsRejected([]domain.RejectedField{
			{Key: string(domain.SlotDeliveryDate), Reason: "缺少西元年份，請補上年份"},
		})
		require.Contains(t, got, "到貨日期")
		require.Contains(t, got, "缺少西元年份，請補上年份")
		require.NotContains(t, got, "delivery_date", "不得把內部 slot key 餵給模型")
	})

	t.Run("多個被拒欄位都要出現", func(t *testing.T) {
		got := summarizeSlotsRejected([]domain.RejectedField{
			{Key: string(domain.SlotItem), Reason: "查無此品項分類，換個說法試試"},
			{Key: string(domain.SlotQuantity), Reason: "數量必須大於 0"},
		})
		require.Contains(t, got, "品項")
		require.Contains(t, got, "數量")
		require.Contains(t, got, "查無此品項分類，換個說法試試")
		require.Contains(t, got, "數量必須大於 0")
	})

	t.Run("SlotLabels 查不到的 key 用原值，不丟棄", func(t *testing.T) {
		got := summarizeSlotsRejected([]domain.RejectedField{
			{Key: "some_future_key", Reason: "理由"},
		})
		require.Contains(t, got, "some_future_key",
			"漏顯示一個被拒欄位比顯示一個醜名字糟")
	})

	t.Run("不同的被拒內容產生不同摘要（否則模型又會每輪同一句）", func(t *testing.T) {
		a := summarizeSlotsRejected([]domain.RejectedField{{Key: string(domain.SlotItem), Reason: "查無此品項"}})
		b := summarizeSlotsRejected([]domain.RejectedField{{Key: string(domain.SlotQuantity), Reason: "數量必須大於 0"}})
		require.NotEqual(t, a, b)
	})
}

// TestSummarizeCropGate_ResolveFailed_DoesNotClaimTheWriteSucceeded 守住措辭：
// 清空是 persistCropGate 的事，而那支失敗時只記 log、不回報，敘述無從得知寫入
// 成功與否，所以不得宣稱「已清除」。
func TestSummarizeCropGate_ResolveFailed_DoesNotClaimTheWriteSucceeded(t *testing.T) {
	got := summarizeCropGate(cropidentity.GateResult{Item: item("番茄"), ResolveFailed: true})
	require.NotContains(t, got, "已清除")
	require.Contains(t, got, "查詢失敗")
}

// TestSummarizeCropGate_PendingWithNoCandidates_HasItsOwnSentence 釘住 Cut 1
// 新增的分支：0 候選待處理不能落進「N 個候選」那句（N=0 讀起來像「還在算」，
// 而不是「已經查完、真的沒有」），也不能落進 Rejected 那句（那句已經沒有生產者，
// 且語意是硬擋，跟待處理不同）。
func TestSummarizeCropGate_PendingWithNoCandidates_HasItsOwnSentence(t *testing.T) {
	noCandidates := summarizeCropGate(cropidentity.GateResult{Pending: true, Candidates: []domain.CropDisambiguationOption{}})
	twoCandidates := summarizeCropGate(cropidentity.GateResult{Pending: true, Candidates: []domain.CropDisambiguationOption{{Label: "甘藍"}, {Label: "球莖甘藍"}}})

	require.NotEqual(t, noCandidates, twoCandidates)
	require.NotContains(t, noCandidates, "0 個候選", "讀起來會像還在算，不是真的查無")
	require.NotContains(t, noCandidates, "維持原本的品項不變", "待處理不是硬擋，不該用 Rejected 那句")
}

// 敘述不得點名逃生艙按鈕：BuildCropDisambiguationQuestion 帶 Selection 時會把
// 「其他（自己輸入）」改寫掉，而這段摘要是 StepNarrator 的輸入，寫死的按鈕名會
// 被忠實複誦給買家。57 號設計書 §3.2：講錯等於宣稱了未發生的事。
func TestSummarizeCropGate_DoesNotNameTheEscapeHatchButton(t *testing.T) {
	for _, selection := range []*domain.CropSelection{
		nil,
		{Stage: domain.CropSelectionPrice},
		{Stage: domain.CropSelectionItem, ParentLabel: "豬肉"},
	} {
		got := summarizeCropGate(cropidentity.GateResult{
			Pending:    true,
			Candidates: []domain.CropDisambiguationOption{},
			Selection:  selection,
		})
		require.NotContains(t, got, "其他（自己輸入）", "按鈕 label 會被改寫，敘述不能寫死它")
		require.NotContains(t, got, "自己輸入")
		require.Contains(t, got, "查無吻合的品項分類")
	}
}

// 敘述的名詞要跟卡片上列的東西一致：taxonomy 兩階段列的是商品分類（豬肉／牛肉），
// 沒有 Selection 的舊路徑列的才是 Pricebook 作物候選。
func TestSummarizeCropGate_NounTracksSelectionStage(t *testing.T) {
	candidates := []domain.CropDisambiguationOption{{Label: "豬肉"}, {Label: "牛肉"}}

	crops := summarizeCropGate(cropidentity.GateResult{Pending: true, Candidates: candidates})
	require.Contains(t, crops, "2 個候選作物")

	group := summarizeCropGate(cropidentity.GateResult{
		Pending: true, Candidates: candidates,
		Selection: &domain.CropSelection{Stage: domain.CropSelectionGroup},
	})
	require.Contains(t, group, "2 個大品項分類")
	require.NotContains(t, group, "作物")

	item := summarizeCropGate(cropidentity.GateResult{
		Pending: true, Candidates: candidates,
		Selection: &domain.CropSelection{Stage: domain.CropSelectionItem, ParentLabel: "豬肉"},
	})
	require.Contains(t, item, "2 個小品項分類")
	require.NotContains(t, item, "作物")
}
