package usecase

import (
	"regexp"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// 買家可見的模型短句共用的事實邊界判準——doc 57 §3.1 第 3 條「不承載事實」的
// 執行期實作。ShortIntent（Activity 那一行）與 step 敘述都走這裡，兩個表面共用
// 同一份規則與同一組 rejected reason。
//
// 為什麼需要它：doc 57 §3.2 的紅線寫著「這條放寬成立的唯一條件是 §3.1 四條約束
// **全部在執行期強制**」，而第 3 條在此之前只存在於文件裡——guardShortIntent 只
// 檢查空白與長度，narrateStepOutcome 只檢查 provider 錯誤、空白與長度。
//
// ⛔ 刻意不套 surface validator 的第一層（block 必須自報 claim code 且落在
// plan.AllowedClaimCodes 內）。那一層管的是「這個 block 宣稱了什麼 act」，屬
// §3.3.1 的 block claim 模型，與本條約束不是同一件事；而且這兩個表面產出的是
// 裸字串、沒有 claim codes，套上去會必定 reject 成 fallback。要讓短句也有 claim
// 模型是一個尚未拍板的設計題（唯一清單的 P02，連帶要決定 NEXT_STEP 語意），
// 不在這裡順手決定。
//
// ⛔ 也不宣稱這兩條規則等於「這句話是有根據的」。店家名稱這類事實字面上抓不到，
// 要靠 blocks 那條路的 fact-echo（它拿得到 committed slots 與 render args），而
// 自然語言整體的 groundedness 只能靠持續評估，不是黑名單能證明的。

// quantifiedFactPattern 比對「數字＋金額／重量／包裝單位」。
//
// 只收金額與貨品單位，不收裸數字：短句講「還缺 2 個條件」是 prompt 明文授權的
// 推導計數，擋掉它只會讓每一輪都退回固定標籤。價格與數量才是 §3.1 第 3 條列名
// 要擋的事實。中文數字一併收——prompt 會要求模型用中文寫數量，只認阿拉伯數字
// 等於留一道明擺著的繞道（surface_validator.go 的 forbiddenPhrases 也記過同一個
// 教訓）。
// 單位那一組同時收中文寫法與 wire 的正式值。⛔ 只收中文是一道明擺著的繞道，
// 而且模型真的會走：slots.unit 存的就是 AllowedUnits 的 enum 字面（"KG"），
// prompt 把它餵進去，於是 dev 實測到 VALIDATE_SLOTS 產出「已記下高麗菜 20KG、
// 單價 60、…」並通過——數量與單價都講了，正是第 3 條要擋的。這跟上面那段「只認
// 阿拉伯數字等於留一道繞道」是同一個推理，當時只套到數字、沒套到單位。
//
// enum 那一半從 domain.AllowedUnits 生成，不在這裡複寫第二份清單：單位值域再收斂
// 一次（像先前收成純重量單位那次）時 policy 自動跟上，不會默默漏掉新值。
//
// 加 (?i) 只影響英文那幾個（中文沒有大小寫），讓 "20kg"／"20Kg" 一併命中。三個
// 正式值都 ≥2 字母，且 pattern 要求緊鄰在數字之後，所以不會有 domain 那邊
// unitAliases 註解記的子字串誤命中問題（"logo"／"program" 前面沒有數字）。
var quantifiedFactPattern = buildQuantifiedFactPattern()

func buildQuantifiedFactPattern() *regexp.Regexp {
	written := []string{
		"元", "塊", "圓", "公斤", "公克", "台斤", "臺斤", "斤", "克", "噸",
		"箱", "顆", "包", "袋", "件", "把", "盒", "折",
		// 貨幣的英文寫法，與「元」同一個理由。
		"TWD", "NTD",
	}
	units := make([]string, 0, len(written)+len(domain.AllowedUnits))
	units = append(units, written...)
	for _, u := range domain.AllowedUnits {
		units = append(units, regexp.QuoteMeta(u))
	}
	return regexp.MustCompile(
		`(?i)[0-9０-９一二三四五六七八九十百千萬兩]+\s*(` + strings.Join(units, "|") + `)`)
}

// publicTextRejection 回傳低基數的拒絕原因；通過時回空字串。
//
// ⛔ 回傳值只能是固定字面值：它會成為 metric label，放原始句子或買家輸入等於
// 把自由文字灌進時序資料庫，也可能把買家講的內容洩進監控。
func publicTextRejection(text string) string {
	if containsForbiddenPhrase(text) {
		return "forbidden_phrase"
	}
	if quantifiedFactPattern.MatchString(text) {
		return "quantified_fact"
	}
	return ""
}
