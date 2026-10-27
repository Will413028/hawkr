package usecase

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// maxBlockTextRunes 是單一 block 文字的長度上限，粗略對應 MaxSentences=2
// 這種簡潔語氣的合理上限（一個中文句子約 20-40 字，2 句留餘裕抓 120）。
const maxBlockTextRunes = 120

var validBlockKinds = map[domain.BlockKind]bool{
	domain.BlockUnderstood: true, domain.BlockContext: true, domain.BlockRationale: true,
	domain.BlockWarning: true, domain.BlockNextStep: true, domain.BlockComplete: true,
}

// forbiddenPhrases 是第二層防禦性關鍵詞 blocklist（spec §4）：即使模型沒有
// 誠實自報 claimCodes，文字裡出現這些片語一律 reject——不管有沒有命中
// 第一層。片語刻意涵蓋比 domain.GlobalForbiddenClaimCodes 更口語的講法，
// 因為模型的自由文字不會照抄 claim code 字面值。
//
// 比對規則見 containsForbiddenPhrase：命中的條件是「片語以肯定形式出現」，
// 被同一子句內的否定／禁止引導詞帶到的片語不算違規。因此這裡每一條都寫
// 語意完整的肯定講法就好，不需要（也不該）為了閃避「不代表 X」「尚未 X」
// 這種 v6 要求的誠實揭露句型而加尾詞限定——那條路走不通：中文的否定句可以
// 把同一個受詞與完成貌尾詞整個包進去（前三輪的教訓，加長尾詞只是把碰撞面
// 挪到下一個一樣自然的句子）。
//
// final review finding 1：「已」與「已經」是同義的完成貌標記，模型的自由
// 文字兩種講法都可能出現，字面比對不會自動涵蓋另一種——因此每個語意詞都
// 刻意同時列「已X」與「已經X」兩種形式，缺一種就會被對應的另一種講法繞過
// （曾經發生：只列「已經送出」，模型講「詢價單已送出」就漏放）。"送出了"、
// "幫你送出" 是另外補的兩個獨立危險片語："了" 本身在中文就是完成貌標記
// （"送出了"＝已經送出的意思），"幫你送出" 常見於模型自己組的「已經幫你
// 送出」講法。
//
// 仍然刻意不列裸字「送出」與「找到」：擋不住它們的理由已經不是否定句（那
// 由 containsForbiddenPhrase 承接），而是未來式／條件句——「等你核准後才會
// 正式送出給賣家」「找到之後我再通知你」都是合法敘述，句子裡沒有任何否定
// 詞可以辨識，只有「已／已經／了」這個完成貌標記能把「已經送出」跟「將要
// 送出」分開，所以這兩個動詞只列帶完成貌標記的形式。
//
// 列表刻意不收「另一條片語的嚴格超集」（例如「已送出給賣家」之於「已送出」、
// 「已找到候選」之於「已找到」）：超集永遠不會比子集先命中，只會讓人以為
// 防護面比實際更寬。
var forbiddenPhrases = []string{
	// 搜尋動作
	"已經開始搜尋", "已開始搜尋", "正在搜尋候選", "搜尋完成",
	"候選已產生", "候選已經產生",
	// 候選數量／賣家聯絡的口語改寫（whole-branch review I3）。上面那組只擋
	// 「候選」這個詞，但模型被 prompt 要求用中文數字寫數量，而第三層 fact-echo
	// 的 arabicNumberPattern 只認阿拉伯數字——「已經幫你找到三個合適的賣家」
	// 三層全過，卻同時捏造了 candidate count 與 search-completed 副作用，兩者
	// 都在設計書 55 §8.1 規則 1 的明文禁止清單裡。
	// 「幫你」是模型很自然會插進「已經」與動詞之間的插入語，會把「已經找到」
	// 拆開，字面比對抓不到，因此「已／已經」×「（幫你）」四種組合都要列。
	"已找到", "已經找到", "已幫你找到", "已經幫你找到",
	"已幫你聯絡", "已經幫你聯絡", "已幫你通知", "已經幫你通知",
	// 送出／發布（doc57 紅線：PUBLICATION_APPROVAL_REQUIRED 階段絕不能讓
	// 模型講出「已經送出去了」的任何等價講法）
	"已經送出", "已送出", "送出了", "幫你送出",
	"已發布", "已經發布", "已寄出", "已經寄出",
	"已投遞", "已經投遞",
	// 通知賣家
	"已通知賣家", "已經通知賣家",
	"賣家已收到", "賣家已經收到",
	"已聯絡賣家", "已經聯絡賣家",
	// 交易／訂單
	"已成交", "已經成交", "交易已完成", "交易已經完成",
	"已下單", "已經下單",
	"訂單已建立", "訂單已經建立", "已建立訂單", "已經建立訂單",
	// 金流
	"已扣款", "已經扣款", "已付款", "已經付款", "款項已處理", "款項已經處理",
	// v6 §2597／§2612：Publication 只表示 canonical publication transaction
	// commit，不得寫成已送達（Seller／email／push 已實際收到）。
	// 上一輪為了閃避 §2597「不得把 PUBLISHED 寫成已送達」與 §2612「不表示…
	// email／push 已送達」這兩句核可否定敘述，退而列「已送達給賣家」；那只是
	// 把碰撞面挪到「這不表示通知已送達給賣家」。否定偵測到位後改回裸字，
	// 涵蓋面更廣（「通知已送達了」也擋得到）而不再有誤傷代價。
	"已送達", "已經送達",
	// v6 §2612／§2880：discoverability／notification scheduling 只是排程狀態，
	// 未取得 discoverability ACK 前不得寫「賣家已看到」。主詞「賣家」是語意的
	// 一部分（v6 禁的是賣家端可見性，不是買家看到什麼），保留；上一輪為了閃避
	// 否定句而加的尾詞「你的」不是語意，拿掉。
	"賣家已看到", "賣家已經看到",
	// v6 §2597／§2612：不得把 PUBLISHED 寫成已報價，也不得暗示已有人回應報價。
	// 前兩輪刻意不列裸字「已報價」的理由——§2880 核可的 CTA 免責文案「這會發布
	// 一筆詢價需求，不代表供應商已報價、供貨或成交」含有同一子字串——已經由
	// 否定偵測承接：那句話的「不代表」與「已報價」在同一子句內，判為否定敘述
	// 放行。同理「已有人報價」不必再用「了」收尾去閃 §2612 的「也不代表已有人
	// 報價」。改回裸字後，「供應商已報價」「有人已經報價了」這些原本漏掉的
	// 肯定講法一併擋得到。
	"已報價", "已經報價", "已有人報價", "已經有人報價",
	// v6 §2960：禁止對供應商排序或成交結果做絕對化宣稱——候選比較
	// 不做排名，系統也不保證任何一筆 RFQ 會成交。
	"最佳供應商", "保證成交",
	// v6 §22／§686／§2959／§2960：付款成功前不得宣稱正式成交或訂單成立
	// （acceptedDealPrice／PENDING_PAYMENT order shell 都不是正式交易）。
	"已正式成交", "已經正式成交", "訂單已成立", "訂單已經成立",
}

// RealizedBlock 是模型輸出的單一段落，含自報的 claim 分類（spec §4 第一層
// 防線的輸入）。ClaimCodes 只在驗證階段使用，通過驗證後轉成 domain.Block
// （Kind＋Text）才進入既有持久化／wire 路徑——ClaimCodes 不上 wire，不是
// 對外契約的一部分。
type RealizedBlock struct {
	Kind       domain.BlockKind `json:"kind"`
	Text       string           `json:"text"`
	ClaimCodes []string         `json:"claimCodes"`
}

// validateSurfaceBlocks 是 semantic validator 的入口（spec §4，issue #2549
// §E）：純函式、無外部 I/O，跟 domain.RenderPlan 同一原則。任一條規則不過
// 就整包 reject，呼叫端 realizeOrFallback 退回 deterministic 模板。
//
// committed 是這一輪定案的 domain.Slots，只給第三層 fact-echo 當允許值域用
// （見 factEchoViolated）。
func validateSurfaceBlocks(plan domain.DialoguePlanV1, args domain.SafeRenderArgs, committed domain.Slots, blocks []RealizedBlock) (ok bool, reason string) {
	if len(blocks) == 0 {
		return false, "blank_text"
	}

	forbidden := make(map[string]bool, len(plan.ForbiddenClaimCodes))
	for _, c := range plan.ForbiddenClaimCodes {
		forbidden[c] = true
	}
	allowed := make(map[string]bool, len(plan.AllowedClaimCodes))
	for _, c := range plan.AllowedClaimCodes {
		allowed[c] = true
	}

	for _, b := range blocks {
		if !validBlockKinds[b.Kind] {
			return false, "unknown_kind"
		}
		if strings.TrimSpace(b.Text) == "" {
			return false, "blank_text"
		}
		if len([]rune(b.Text)) > maxBlockTextRunes {
			return false, "too_long"
		}
		if len(b.ClaimCodes) == 0 {
			return false, "claim_violation" // 每個 block 必須至少自報一個 claim（spec §4 第一層）
		}
		for _, c := range b.ClaimCodes {
			if forbidden[c] {
				return false, "claim_violation"
			}
			if !allowed[c] {
				return false, "claim_violation" // 自報了不在 allowlist 內的 claim，同樣視為違規
			}
		}
		if containsForbiddenPhrase(b.Text) {
			return false, "keyword_blocklist" // 第二層：不管有沒有誠實自報，文字本身講了危險片語
		}
		if factEchoViolated(committed, args, b.Text) {
			return false, "fact_mismatch" // 第三層：講出了事實清單以外的數字／城市／單位
		}
	}
	return true, ""
}

// negationCues 是否定／禁止引導詞。命中的 forbiddenPhrases 片語若在同一子句
// 內被其中一個引導，那是 v6 要求的誠實揭露，不是宣稱——v6 的凍結文案本身就
// 是這種句型：§2597「不得把 PUBLISHED 寫成已送達、已報價或已成交」、
// §2612「不表示符合資格的 Seller 已看到、email／push 已送達或已有人報價」、
// §2880「未取得 discoverability ACK 前不得寫『賣家已看到』」、
// §2960「禁止『最佳供應商』『保證成交』『已下單』」。
//
// 「不得」「禁止」是 v6 原文用的禁令標記，跟「不代表」同樣是否定引導，列進來
// 才能讓引用禁令的敘述通過。
//
// 「不能」沒有 v6 §2597／§2612／§2880／§2960 原文出處，理由是模態否定改寫，
// 不是逐字引用：「不保證」是「不」直接接「保證」兩字相鄰，字面比對抓得到；
// 「不能保證」在「不」與「保證」之間插了一個「能」，字面上完全是另一個
// substring，抓不到「不保證」這個 cue，但語意跟「不保證」等價（「系統不能
// 保證成交」＝「系統不保證成交」的自然說法）。scratchpad 實測：拿掉「不能」，
// 「系統不能保證成交，也不會自動下單。」會被誤擋（句子裡沒有任何其他 cue
// 命中）；加回「不能」才正確放行。保留。
//
// 刻意不列裸字「不」與「別」：「不只代表」「別的賣家」這種非否定用法會把整個
// 子句的防護解除，等於送給模型一個一個字的萬用鑰匙。只列完整的否定片語。
var negationCues = []string{
	"不代表", "不表示", "不是", "不會", "不能", "不保證", "不等於", "不得",
	"並非", "尚未", "還沒", "沒有", "未", "禁止",
}

// clauseBoundaryRunes 是切斷否定作用域的標點。否定引導詞只管到子句結束，
// 這是防繞過的關鍵：沒有這個限制，模型只要在段落開頭寫一句「這不代表…」，
// 後面整段都可以自由宣稱副作用。
//
// 刻意不收頓號「、」：它是列舉分隔符不是子句終結符，中文的「不表示 A、B、C」
// 一個否定詞管整串——v6 §2597／§2612 的原文正是這個句型（「不得把 PUBLISHED
// 寫成已送達、已報價或已成交」），把「、」當邊界會直接誤傷 v6 凍結文案本身。
// 同理不收冒號與引號：它們引入否定的內容，不終結它（§2880 的
// 「不得寫『賣家已看到』」就靠這點）。
const clauseBoundaryRunes = "，。；！？\n\r;!?"

// clauseBoundaryWords 是能在中文裡不靠標點就結束前一個子句、開啟一個新肯定
// 子句的連接詞。最早只收轉折詞（但／不過／然而／可是／而是），但「還沒核准
// 但已經送出給賣家了」成立的論證——中文子句可以只靠一個連接詞收尾、不需要
// 標點——對因果、承接、讓步連接詞同樣成立，只收轉折詞會漏掉同一類的繞過：
//
//	還沒核准所以我已經送出給賣家了。   （因果，所以）
//	你還沒回覆我就先幫你送出了。       （承接，就）
//
// 前者跟已凍結的「但」版本只差一個連接詞；後者是 ActPublicationApprovalRequired
// 階段最典型的幻覺句型之一——這裡的否定掛在一個獨立的前置子句上，不是包裝在
// 關係子句或否定框架裡，模型不需要任何刻意繞過的意圖就會自然寫出來，而第一層
// （模型自報，對不誠實自報無效）與第三層（fact-echo 只認阿拉伯數字，這句沒有
// 數字）都接不住。
//
// 完整清單（scratchpad 逐一實測：每條都對 11 條合法否定 fixture、v6 §2597／
// §2612／§2880／§2960 原文與自然改寫、既有全部 pass fixture 跑過，結果見
// clause-boundary-completion-report.md）：
//
//   - 因果：所以、因此、因而、於是、結果
//   - 承接：然後、接著、就
//   - 讓步：還是、仍然、仍
//
// 刻意不另收「雖然」：讓步句「雖然…還是…」的子句邊界在「還是」，不在「雖然」
// ——「雖然」開啟的是被讓步掉的那個（否定）子句本身，「還是」才是切回肯定
// 子句的位置。「還是」單獨已經擋得住「雖然還沒核准還是已經送出給賣家了」（無
// 標點），不需要再收「雖然」。
//
// 刻意排除「才」與「而」（reviewer 建議清單裡本來有，實測後主動拿掉，不是
// 沒發現）：兩個都是常見詞的組成部分——剛才／方才（才）、反而／進而／從而
// （而）——當這些詞出現在一個否定詞與一個 forbiddenPhrases 片語之間、且中間
// 沒有標點時，會被誤判成子句邊界，把否定詞排除在搜尋窗口外。例如加了「才」
// 之後，「這不代表剛才已經送出。」會被誤擋；加了「而」之後，「這不代表反而
// 已經送出。」「這不代表進而已經送出。」都會被誤擋。這兩個詞都不是本次要
// 收斂的兩個 controller 實測繞過（還沒核准所以我已經送出給賣家了／你還沒
// 回覆我就先幫你送出了）需要的連接詞，「而」的轉折功能也跟已經收錄的
// 但／不過／然而／可是／而是重疊——保留它們換到的邊際覆蓋（各自只多擋一句
// 自行構造的類比繞過，不是 controller 點名的真實案例）不足以蓋過「剛才」
// 「方才」「反而」「進而」「從而」都是日常高頻詞的誤擋成本，所以刻意不收。
//
// 殘留風險（已實測、刻意保留，不是沒發現）：就／還是／結果仍然有同一類代價
// ——早就／本來就／成就（就）、「A 還是 B」選擇問句（還是）、「搜尋結果」
// 這類名詞用法（結果，跟這個 domain 本身的「候選／搜尋」概念相關，風險比其他
// 幾個更貼近實際場景）。「就」沒有更窄的收法：它是 controller 點名繞過
// 「你還沒回覆我就先幫你送出了」裡唯一的連接詞，找不到比裸字「就」更窄、又
// 不漏掉這句真實案例的寫法，所以就算代價比「才」更大（「就」是更高頻的字），
// 仍然保留。「還是」「結果」除了 controller 未點名的自行構造繞過之外，也各自
// 對應一類合理、真實會出現的因果／讓步講法（見上），對既有全部 pass fixture
// （11 條合法否定＋v6 原文與改寫＋其餘既有測試）零回歸，只有刻意構造的對抗
// 例句才會踩到；風險與效益的取捨留給 review 判斷，不在這裡默默拿掉。
var clauseBoundaryWords = []string{
	"但", "不過", "然而", "可是", "而是",
	"所以", "因此", "因而", "於是", "結果",
	"然後", "接著", "就",
	"還是", "仍然", "仍",
}

// containsForbiddenPhrase 回報 text 裡是否有 forbiddenPhrases 的片語「以肯定
// 形式」出現（spec §4 第二層）。
//
// 純 strings.Contains 無法區分「X」與「不X」：任何片語 X 都會被「不代表 X」
// 「尚未 X」命中，而那些正是 v6 §2597／§2612／§2880 要求的誠實揭露講法。
// 這裡改成：片語每一次出現都往前看到子句開頭，找得到否定引導詞就視為否定
// 敘述放行，否則就是違規。
//
// 逐次出現都要檢查，不能只看第一次：「這不代表已經送出。我已經送出給賣家了。」
// 第一次出現被否定引導、第二次是真的宣稱，只檢查第一次等於整段放行。
func containsForbiddenPhrase(text string) bool {
	for _, phrase := range forbiddenPhrases {
		for offset := 0; offset <= len(text)-len(phrase); {
			i := strings.Index(text[offset:], phrase)
			if i < 0 {
				break
			}
			at := offset + i
			if !negatedInSameClause(text, at, len(phrase)) {
				return true
			}
			offset = at + len(phrase)
		}
	}
	return false
}

// negatedInSameClause 檢查 text 裡 [at, at+phraseLen) 這次片語命中，是不是被
// 同一子句內的否定引導詞帶到的。
//
// 搜尋範圍刻意延伸到片語結尾，而不是停在片語開頭：引導詞可以跟片語重疊。
// 「不保證成交」是 §2960「保證成交」唯一誠實的講法，而它的引導詞「不保證」
// 橫跨片語起點——只看片語之前的文字會看到「不」而找不到任何完整引導詞，
// 於是把 v6 自己的否定講法判成違規（這是寫測試時才發現的，見
// TestValidateSurfaceBlocks_Passes_NegatedClaimsInSameClause 的「不保證成交」）。
//
// 延伸範圍不會放寬判準：條件仍然是引導詞必須「開始」在片語之前（start+i < at）。
//
// 這個條件同時保護了 TestForbiddenPhrases_ContainNoNegationCue 釘住的不變式
// ——forbiddenPhrases 每一條都不含任何 negationCues 子字串——但它保護的不是
// 「片語會不會被自己內建的引導詞放行自己」：start+i < at 排除了落在片語內部
// （位置 >= at）的引導詞命中，所以一個片語自己那次命中，看的仍然是片語之前
// 有沒有別的引導詞，不會因為自己帶了引導詞就永遠放行自己（scratchpad 實測：
// 假設 forbiddenPhrases 混進一條含「未」的片語「尚未送出就已送出」，「尚未
// 送出就已送出給賣家了。」仍然 BLOCK——這條片語自己的那次命中沒有被自己的
// 「尚未」放行）。
//
// 這個不變式真正擋的是同一子句「後面」的其他片語：如果 forbiddenPhrases 有
// 一條帶引導詞的片語出現在子句前段，它內建的引導詞會落在後面其他片語的搜尋
// 窗口裡，start+i < at 對那些後面的片語一樣成立，會把它們一起放行——等於這
// 一條片語變成一個沒人審查過、跟著 blocklist 一起偷渡進來的否定 cue
// （scratchpad 實測：混進「尚未送出就已送出」後，同一子句後面接一句本該被擋
// 的「賣家已看到你的需求了」，那次命中被單獨判成 negated=true——只是這個
// fixture 裡「尚未送出就已送出」自己那次命中仍然 BLOCK，把整段擋下來，掩蓋
// 了這句「賣家已看到」本身已經被偷渡放行的事實）。TestForbiddenPhrases_
// ContainNoNegationCue 釘住的就是「forbiddenPhrases 不能意外帶有這種偷渡
// cue 的能力」。
func negatedInSameClause(text string, at, phraseLen int) bool {
	start := clauseStartIndex(text[:at])
	window := text[start : at+phraseLen]
	for _, cue := range negationCues {
		// strings.Index 回傳最早的出現位置，所以它 >= at 就代表這個引導詞
		// 在這個 window 裡的每一次出現都在片語起點之後，不構成否定。
		if i := strings.Index(window, cue); i >= 0 && start+i < at {
			return true
		}
	}
	return false
}

// clauseStartIndex 回傳 before 裡最後一個子句的起始 byte index。
// jiuFixedAdverbPrefixes 是「就」作為固定副詞一部分時的前綴。這些組合裡的「就」
// 不開啟新子句——「這不代表早就已經送出」整句是一個否定敘述，把「早就」的
// 「就」當成邊界會把否定詞切在搜尋窗口之外，讓合法的誠實揭露被誤擋（2026-09-11
// 實測三句皆誤擋）。
//
// 只收窄「就」：`還是`（A 還是 B 選擇問句）與 `結果`（搜尋結果類名詞）同樣有
// 誤擋，但它們沒有這種可枚舉的固定前綴，收窄需要詞性判斷；而誤擋的代價是退回
// 模板（方向安全），漏擋的代價是買家被告知沒發生的事，所以維持現狀不動。
var jiuFixedAdverbPrefixes = []string{"早", "本來", "也", "這", "那", "原本"}

// inFixedAdverb 判斷 before[j:] 起的邊界詞是否只是某個固定副詞的尾巴。
func inFixedAdverb(before string, j int, w string) bool {
	if w != "就" {
		return false
	}
	for _, p := range jiuFixedAdverbPrefixes {
		if strings.HasSuffix(before[:j], p) {
			return true
		}
	}
	return false
}

func clauseStartIndex(before string) int {
	start := 0
	for i := 0; i < len(before); {
		r, size := utf8.DecodeRuneInString(before[i:])
		if isClauseBoundaryRune(before, i, r) {
			start = i + size
		}
		i += size
	}
	for _, w := range clauseBoundaryWords {
		j := strings.LastIndex(before, w)
		if j < 0 || j+len(w) <= start {
			continue
		}
		if inFixedAdverb(before, j, w) {
			continue
		}
		start = j + len(w)
	}
	return start
}

// isClauseBoundaryRune 判斷 before[i] 這個 rune 是不是子句邊界。
//
// 半形句點與逗號同時是數字的小數點與千分位（「1.5 噸」「1,000 公斤」），
// 只有後面不接數字時才算邊界——否則「不代表 1,000 公斤已送達」會被誤擋。
func isClauseBoundaryRune(before string, i int, r rune) bool {
	if strings.ContainsRune(clauseBoundaryRunes, r) {
		return true
	}
	if r != '.' && r != ',' {
		return false
	}
	next, _ := utf8.DecodeRuneInString(before[i+len(string(r)):])
	return next < '0' || next > '9'
}

// factEchoViolated 是第三層防線：模型只能轉述已 commit 的事實值，講出清單以外
// 的數字、城市或單位一律整包 reject（spec §6，設計書 55 §8.1 規則 1）。
//
// 只檢查封閉值域的 token，這是避免誤擋的關鍵——中文數量詞（「兩項」「下週」）
// 不含阿拉伯數字，`AllowedCities`／`AllowedUnits` 以外的名詞不會被當成事實。
//
// 允許值域是「當輪 committed 的 Slots」∪「SafeRenderArgs 四欄」：
//   - Slots 是主來源，直接對應 §8.1 規則 1 的「已 commit 的事實」。只取
//     SafeRenderArgs 會系統性誤擋：SalientFactLabels 只放這一輪 recorded 的
//     最多 2 筆（turn.go 的 salientFactLabels），/rewinds 更是恆為空，但
//     buildSurfaceRealizePrompt 會把最近 2 則歷史回覆整段餵給模型——模型手上
//     握著第 2 輪的「配送城市：桃園市、數量：50」，第 3 輪任何承接前文的自然句
//     都會被 reject。我們餵給模型的東西必須在我們允許它講的清單裡。
//   - 刻意用 committed Slots 而不是直接把 RecentNarratives 加進 haystack：被
//     更正過的舊值（quantity 50→80）不在 committed Slots 裡，會自然落在集合外，
//     而 RecentNarratives 會把它一起放行。
//   - SafeRenderArgs 那部分保留：下一題的問句（NextQuestionPrompt）本身就可能
//     帶日期或數量，那些數字模型當然可以複述；rejected／missing 的標籤同理。
func factEchoViolated(committed domain.Slots, args domain.SafeRenderArgs, text string) bool {
	haystack := strings.Join(append(append(append(append(
		[]string{committedSlotsHaystack(committed)},
		args.SalientFactLabels...),
		args.MissingRequiredLabels...),
		args.RejectedLabels...),
		args.NextQuestionPrompt), "\n")

	// 數字用逐 token 相等，不是子字串：haystack 有 "500" 時模型講 "50"
	// 必須被擋下，子字串比對會誤放。
	// 允許的數字＝haystack 裡的事實值 ＋「還缺幾個欄位」這個推導計數。
	// 後者是 prompt 明文授權模型陳述的唯一數量（surface_realizer.go 的
	// surfaceRealizeInstruction），而它不會以字面形式出現在任何標籤裡——
	// MissingRequiredLabels 是 ["配送城市","單位"]，"2" 不在其中。不補這個推導值，
	// 每一句「還缺 N 項」都會被判成捏造。
	//
	// 刻意只補這一個推導值：允許集合每多一個小整數，就多一組巧合碰撞的機會
	// （缺 3 項時模型講「找到 3 個賣家」會被放行）。SalientFactLabels／
	// RejectedLabels 的長度不補，因為 prompt 沒有授權模型陳述它們。
	allowedNums := domain.NumberTokensIn(haystack)
	if n := len(args.MissingRequiredLabels); n > 0 {
		allowedNums = append(allowedNums, strconv.Itoa(n))
	}
	if !subset(domain.NumberTokensIn(text), allowedNums) {
		return true
	}
	if !subset(domain.CanonicalCitiesIn(text), domain.CanonicalCitiesIn(haystack)) {
		return true
	}
	return !subset(domain.CanonicalUnitsIn(text), domain.CanonicalUnitsIn(haystack))
}

// committedSlotsHaystack 把已 commit 的 Slots 攤成一段文字給 fact-echo 抽 token。
//
// 只收買家自己講過、也會被複述的欄位。刻意排除兩個：
//   - ItemResolution 是伺服器端的分類解析結果（含分類 id），買家從沒看過，把它
//     的數字放進允許值域等於讓模型可以講出任意 id。允許值域每多一個 token 就
//     多放行一種竄改，寧可窄。
//   - ExactMatchRequired 是 bool，不產生任何 token。
func committedSlotsHaystack(s domain.Slots) string {
	parts := make([]string, 0, 7)
	appendStr := func(p *string) {
		if p != nil {
			parts = append(parts, *p)
		}
	}
	appendInt := func(p *int) {
		if p != nil {
			parts = append(parts, strconv.Itoa(*p))
		}
	}
	appendStr(s.Item)
	appendInt(s.Quantity)
	appendStr(s.Unit)
	appendStr(s.Packaging)
	appendInt(s.BudgetPerUnit)
	appendStr(s.DeliveryCity)
	appendStr(s.DeliveryDate)
	return strings.Join(parts, "\n")
}

func subset(got, allowed []string) bool {
	if len(got) == 0 {
		return true
	}
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	for _, g := range got {
		if !set[g] {
			return false
		}
	}
	return true
}
