package zhtw

import "testing"

// LLM 抽取的中文語域是機率性的——不能靠 prompt 保證輸出繁體，這支測試釘住的是
// 讓「高丽菜」（簡體，2026-08-15 買家實測回報的真實案例，導致跟
// product 目錄的繁體字比對失敗、候選搜尋 0 家）轉成「高麗菜」。
func TestToTraditional_ConvertsSimplifiedToTraditional(t *testing.T) {
	got := ToTraditional("高丽菜")
	want := "高麗菜"
	if got != want {
		t.Errorf("ToTraditional(%q) = %q, want %q", "高丽菜", got, want)
	}
}

// 已經是繁體的輸入必須是 no-op，不能因為轉換而意外改字（例如台灣正體用語
// 轉換過頭變成香港或其他變體）。
func TestToTraditional_TraditionalInputIsNoop(t *testing.T) {
	got := ToTraditional("高麗菜")
	want := "高麗菜"
	if got != want {
		t.Errorf("ToTraditional(%q) = %q, want %q (no-op)", "高麗菜", got, want)
	}
}

// 非中文輸入（英文品項名、空字串）必須原樣通過，不能因為呼叫轉換函式而出錯或
// 意外改動。
func TestToTraditional_NonChineseInputPassesThrough(t *testing.T) {
	cases := []string{"Apple", "", "iPhone 15 Pro"}
	for _, in := range cases {
		if got := ToTraditional(in); got != in {
			t.Errorf("ToTraditional(%q) = %q, want unchanged %q", in, got, in)
		}
	}
}

// TestToTraditional_ConvertsSimplifiedSentence 是 #2549 Phase 1 Task 9 從
// usecase 套件一個已刪除的自然語回覆整合測試遷移過來的：那支測試原本透過假 LLM
// 走完整套回覆機制斷言簡體回覆會被轉成繁體，測的其實一直是 ToTraditional 本身
// （不是那套已刪除機制的專屬邏輯）——這個真實案例（2026-08-15 買家實測：LLM
// 偶爾吐出整句簡體，不只是單一詞彙）直接搬來這裡繼續測 ToTraditional，涵蓋比
// 上面「高丽菜」更長的完整句子輸入。ToTraditional 本身這個函式唯一剩下的消費者是
// usecase/extractor_provider.go：LLM 抽取出來的 slot 值在進入 domain 層之前、於
// 抽取邊界就地正規化成繁體（見該檔案呼叫點）。domain/ 完全不 import zhtw——
// RenderPlan 的 SafeRenderArgs 只內插伺服器自己擁有的固定標籤（domain.SlotLabels
// 等靜態字串），從不內插模型原始輸出，所以這個「使用者看到的一定是繁體」的不變式
// 是靠「render 層根本不碰模型輸出」這個結構性保證維持的，不是靠 render 層再做一次
// 正規化。
func TestToTraditional_ConvertsSimplifiedSentence(t *testing.T) {
	got := ToTraditional("已经记下你的需求。")
	want := "已經記下你的需求。"
	if got != want {
		t.Errorf("ToTraditional(%q) = %q, want %q", "已经记下你的需求。", got, want)
	}
}
