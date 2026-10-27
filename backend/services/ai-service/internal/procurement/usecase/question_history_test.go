package usecase

import (
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/stretchr/testify/require"
)

func mustPayload(t *testing.T, p turnPayload) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	require.NoError(t, err)
	return b
}

// --- deriveSkippedSlots ------------------------------------------------
//
// 這段邏輯（正向掃描全歷史收集 Skip 欄位）不是這輪修的 bug，維持原本測試涵蓋。

func TestDeriveSkippedSlots_CollectsSkippedSlots(t *testing.T) {
	pkg := domain.SlotPackaging
	history := []repository.RevisionWithPatch{
		{PatchSource: "ANSWER", PatchPayloadJSON: mustPayload(t, turnPayload{
			Skip: &pkg, Question: &domain.ClarifyQuestion{SlotKey: domain.SlotQuantity},
		})},
	}

	skipped := deriveSkippedSlots(history)

	require.True(t, skipped[domain.SlotPackaging])
}

func TestDeriveSkippedSlots_EmptyHistory_ReturnsEmptyMap(t *testing.T) {
	skipped := deriveSkippedSlots(nil)

	require.NotNil(t, skipped)
	require.Empty(t, skipped)
}

// --- previousOptionalStreak ---------------------------------------------
//
// 新設計：streak 不再從歷史反推，只讀「tip（最近一筆）persisted 的值」——
// O(1)，且不需要對 Question/Skip 欄位做任何猜測性解讀。

func TestPreviousOptionalStreak_EmptyHistory_ReturnsZero(t *testing.T) {
	require.Equal(t, 0, previousOptionalStreak(nil))
}

func TestPreviousOptionalStreak_RootRevisionWithoutPatch_ReturnsZero(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{PatchPayloadJSON: nil}, // 根版本，沒有 patch
	}

	require.Equal(t, 0, previousOptionalStreak(history))
}

func TestPreviousOptionalStreak_UndecodablePayload_ReturnsZero(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{PatchPayloadJSON: []byte(`not json`)},
	}

	require.Equal(t, 0, previousOptionalStreak(history))
}

func TestPreviousOptionalStreak_ReadsTipPersistedValue(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{PatchPayloadJSON: mustPayload(t, turnPayload{OptionalStreak: 1})},
		{PatchPayloadJSON: mustPayload(t, turnPayload{OptionalStreak: 2})},
		{PatchPayloadJSON: mustPayload(t, turnPayload{OptionalStreak: 3})}, // tip
	}

	require.Equal(t, 3, previousOptionalStreak(history),
		"只讀最後一筆（tip），不管更早的紀錄寫了什麼值")
}

// TestPreviousOptionalStreak_CapFiredTurnPersistsStreakForNextTurn 是舊版
// TestDeriveQuestionHistory_StreakSurvivesCapFiredTurn 的等價替代：舊版靠反向
// 掃描猜「cap 觸發那一輪的 nil question 該不該打斷 streak」，新版不用猜——
// cap 觸發那一輪自己在 computeOptionalStreak 就已經把 previousStreak 原樣寫進
// 自己的 payload（見 TestComputeOptionalStreak_CapExhausted_KeepsPreviousStreak），
// 這裡驗證的是「下一輪讀那個值」這一半的契約：tip 存了什麼，previousOptionalStreak
// 就原樣讀出什麼，不會因為 Question 是 nil 就誤判成該歸零。
func TestPreviousOptionalStreak_CapFiredTurnPersistsStreakForNextTurn(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{PatchSource: "LLM_EXTRACT", PatchPayloadJSON: mustPayload(t, turnPayload{
			Question: &domain.ClarifyQuestion{SlotKey: domain.SlotDeliveryCity}, OptionalStreak: 0,
		})},
		{PatchSource: "ANSWER", PatchPayloadJSON: mustPayload(t, turnPayload{
			Question: &domain.ClarifyQuestion{SlotKey: domain.SlotPackaging}, OptionalStreak: 1,
		})},
		{PatchSource: "ANSWER", PatchPayloadJSON: mustPayload(t, turnPayload{
			Question: &domain.ClarifyQuestion{SlotKey: domain.SlotQuantity}, OptionalStreak: 2,
		})},
		{PatchSource: "ANSWER", PatchPayloadJSON: mustPayload(t, turnPayload{
			Question: &domain.ClarifyQuestion{SlotKey: domain.SlotUnit}, OptionalStreak: 3,
		})},
		{PatchSource: "ANSWER", PatchPayloadJSON: mustPayload(t, turnPayload{
			Question: nil, OptionalStreak: 3, // cap 生效這一輪：question 是 nil，但 streak 原樣延續
		})},
	}

	got := previousOptionalStreak(history)

	require.GreaterOrEqual(t, got, domain.MaxConsecutiveOptionalQuestions,
		"cap 觸發那一輪持久化的 streak 必須原樣被下一輪讀到，不能因為 Question 是 nil 就被讀成 0")
}

// --- computeOptionalStreak -----------------------------------------------
//
// 六規則的核心：這一輪結束後要持久化的 optionalStreak 新值。

func TestComputeOptionalStreak_StillClarifying_ResetsToZero(t *testing.T) {
	got := computeOptionalStreak("CLARIFYING",
		&domain.ClarifyQuestion{SlotKey: domain.SlotDeliveryCity}, false, 5)

	require.Equal(t, 0, got, "必填還沒填完，選填提問階段根本還沒開始")
}

func TestComputeOptionalStreak_OptionalQuestionAsked_Increments(t *testing.T) {
	got := computeOptionalStreak("REQUIREMENT_READY",
		&domain.ClarifyQuestion{SlotKey: domain.SlotPackaging}, false, 2)

	require.Equal(t, 3, got, "這輪確實問了一題選填，延續累加")
}

// TestComputeOptionalStreak_CapExhausted_KeepsPreviousStreak 釘住原本回報的
// under-count bug：cap 剛打滿（本來有符合資格的選填欄位、卻被 cap 擋下）時，
// streak 必須原樣延續，不能歸零——否則下一輪 cap 就形同虛設。
func TestComputeOptionalStreak_CapExhausted_KeepsPreviousStreak(t *testing.T) {
	got := computeOptionalStreak("REQUIREMENT_READY", nil, true, domain.MaxConsecutiveOptionalQuestions)

	require.Equal(t, domain.MaxConsecutiveOptionalQuestions, got,
		"cap 觸發那一輪的 nil question 不該讓 streak 歸零，否則下一輪又會重新問滿選填")
}

// TestComputeOptionalStreak_CropDisambiguationPending_DoesNotIncrementStreak 釘住
// final review finding I2：消歧問句（domain.SlotCropDisambiguation）已經被排除在
// capExhausted 之外（turn.go／answer.go 呼叫端在 cropGate.Pending 時明確把
// capExhausted 設回 false——spec §5「跟 !Ready 同一類的持續性 gate」），但同一份
// 「不算選填連續計數」的豁免先前漏掉了 computeOptionalStreak 這一半：規則 2 只看
// !domain.IsRequiredSlot(question.SlotKey)，消歧問句的 SlotKey 不在 requiredSlots
// 裡，落進規則 2 就會被誤判成「這輪問了一題選填」而累加。買家卡在消歧卡片上的每一
// 輪都會這樣讓 streak 往上疊，疊到 cap 之後，即使消歧最終解開，quantity／
// delivery_date（真正「選填但報價必填」的欄位，QuoteRequiredOptionalSlots）也會
// 因為 streak 已經打滿而永久問不到——買家從未真的被問過這兩題。
//
// 修法是 computeOptionalStreak 內部新增一條規則，插在規則 2 之前：question 指向
// 消歧問句時，streak 原樣延續（不遞增、也不歸零），跟 cap 觸發那一輪（規則 3）用
// 同一種「延續」語意——消歧问句本來就不該讓連續選填計數往任何方向移動。
func TestComputeOptionalStreak_CropDisambiguationPending_DoesNotIncrementStreak(t *testing.T) {
	got := computeOptionalStreak("REQUIREMENT_READY",
		&domain.ClarifyQuestion{SlotKey: domain.SlotCropDisambiguation}, false, 2)

	require.Equal(t, 2, got,
		"消歧問句是持久性 gate，不是選填問題；卡在消歧卡片的每一輪都不該推進選填連續計數")
}

// TestComputeOptionalStreak_NothingLeftToAsk_ResetsToZero 釘住第二輪修復要解決
// 的 over-count regression：question 是 nil，但不是 cap 造成（capExhausted=false，
// 代表根本沒有符合資格的選填欄位）——這種情況沒有任何連續計數需要保護，必須歸零，
// 即使先前的 streak 已經累積到一個非零值。之後若有欄位被 /rewinds 重新打開，
// 才能從乾淨的 0 開始被公平地考慮要不要問，不會被完全不相干的舊 streak 卡住。
func TestComputeOptionalStreak_NothingLeftToAsk_ResetsToZero(t *testing.T) {
	got := computeOptionalStreak("REQUIREMENT_READY", nil, false, 2)

	require.Equal(t, 0, got, "真的沒東西可問時，streak 必須歸零，不是延續舊值")
}

// TestComputeOptionalStreak_QuestionTargetsRequiredSlot_ResetsToZero
// 涵蓋規則第 5 條的防禦分支：理論上 status 已經 READY 時 PickNextQuestion 不會
// 回必填問題（missingRequired 這時必為空），但 computeOptionalStreak 不依賴這個
// 不變式，遇到這種輸入組合仍要回 0，不是 panic 或延續舊值。
func TestComputeOptionalStreak_QuestionTargetsRequiredSlot_ResetsToZero(t *testing.T) {
	got := computeOptionalStreak("REQUIREMENT_READY",
		&domain.ClarifyQuestion{SlotKey: domain.SlotDeliveryCity}, false, 2)

	require.Equal(t, 0, got)
}
