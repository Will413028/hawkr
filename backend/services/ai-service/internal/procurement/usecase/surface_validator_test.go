package usecase

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func TestValidateSurfaceBlocks_Passes_WhenClaimsAreAllowed(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActAckAndAskRequired,
		AllowedClaimCodes:   []string{"REQUEST_FACT_RECORDED", "NEXT_QUESTION_REQUIRED"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockUnderstood, Text: "已記錄品項。", ClaimCodes: []string{"REQUEST_FACT_RECORDED"}},
		{Kind: domain.BlockNextStep, Text: "還想請你補充數量。", ClaimCodes: []string{"NEXT_QUESTION_REQUIRED"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.True(t, ok, "reason=%s", reason)
}

func TestValidateSurfaceBlocks_Rejects_UndeclaredForbiddenClaim(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActRequestBasicsReady,
		AllowedClaimCodes:   []string{"BASICS_COMPLETE"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockUnderstood, Text: "需求齊全了。", ClaimCodes: []string{"BASICS_COMPLETE"}},
		{Kind: domain.BlockNextStep, Text: "我已經開始搜尋合適的候選了。", ClaimCodes: []string{"BASICS_COMPLETE"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "keyword_blocklist", reason)
}

func TestValidateSurfaceBlocks_Rejects_SelfReportedForbiddenClaim(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockComplete, Text: "報價單已確認。", ClaimCodes: []string{"RFQ_PENDING_APPROVAL", "RFQ_PUBLISHED"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "claim_violation", reason)
}

func TestValidateSurfaceBlocks_Rejects_UnknownBlockKind(t *testing.T) {
	plan := domain.DialoguePlanV1{ActCode: domain.ActAckAndAskRequired, AllowedClaimCodes: []string{"X"}, MaxSentences: 2}
	blocks := []RealizedBlock{{Kind: domain.BlockKind("MADE_UP"), Text: "文字", ClaimCodes: []string{"X"}}}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "unknown_kind", reason)
}

func TestValidateSurfaceBlocks_Rejects_EmptyClaimCodes(t *testing.T) {
	plan := domain.DialoguePlanV1{ActCode: domain.ActAckAndAskRequired, AllowedClaimCodes: []string{"X"}, MaxSentences: 2}
	blocks := []RealizedBlock{{Kind: domain.BlockUnderstood, Text: "文字", ClaimCodes: nil}}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "claim_violation", reason)
}

func TestValidateSurfaceBlocks_Rejects_EmptyBlocks(t *testing.T) {
	ok, reason := validateSurfaceBlocks(domain.DialoguePlanV1{}, domain.SafeRenderArgs{}, domain.Slots{}, nil)
	require.False(t, ok)
	require.Equal(t, "blank_text", reason)
}

// TestValidateSurfaceBlocks_Rejects_BlankBlockText 涵蓋「有 block 但文字是空白」
// 這條跟「完全沒有 block」共用同一個 reason 字串的路徑（final review finding 4：
// reason 字串已改名為 "blank_text"，這裡跟上面 EmptyBlocks 測試分別覆蓋兩個
// 觸發點，避免改名漏掉其中一處）。
func TestValidateSurfaceBlocks_Rejects_BlankBlockText(t *testing.T) {
	plan := domain.DialoguePlanV1{ActCode: domain.ActAckAndAskRequired, AllowedClaimCodes: []string{"X"}, MaxSentences: 2}
	blocks := []RealizedBlock{{Kind: domain.BlockUnderstood, Text: "   ", ClaimCodes: []string{"X"}}}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "blank_text", reason)
}

// 以下三個測試是 final review finding 1 的直接回歸測試：forbiddenPhrases 此前
// 「已」／「已經」形式列得不一致，導致模型講「詢價單已送出」（沒有「經」字）
// 能繞過兩層驗證——這是 doc57 明講的紅線（PUBLICATION_APPROVAL_REQUIRED 階段
// 買家必須清楚知道 RFQ 還沒送出）。

// TestValidateSurfaceBlocks_Rejects_AlreadySentWithoutJing 是關鍵回歸測試：
// 「已送出」（無「經」字）之前不在 blocklist 裡，且 claim codes 沒有誠實自報
// RFQ_PUBLISHED 時，兩層防線都會放行——這正是 finding 1 描述的漏洞本身。
func TestValidateSurfaceBlocks_Rejects_AlreadySentWithoutJing(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockComplete, Text: "詢價單已送出。", ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "keyword_blocklist", reason)
}

// TestValidateSurfaceBlocks_Rejects_AlreadyStartedSearchingWithJing 釘住既有的
// 「已經」形式覆蓋沒有因為這次改動而退化（regression guard，不是新行為）。
func TestValidateSurfaceBlocks_Rejects_AlreadyStartedSearchingWithJing(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActRequestBasicsReady,
		AllowedClaimCodes:   []string{"BASICS_COMPLETE"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockUnderstood, Text: "需求齊全，已經開始搜尋候選了。", ClaimCodes: []string{"BASICS_COMPLETE"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.False(t, ok)
	require.Equal(t, "keyword_blocklist", reason)
}

// TestValidateSurfaceBlocks_Passes_NotYetSentNegation 是最重要的假陽性防護測試：
// PUBLICATION_APPROVAL_REQUIRED 唯一合法、且必要的敘述就是講清楚 RFQ「尚未」
// 送出。兩句 fixture 同時涵蓋否定式（"尚未送出"／"還沒送出"）與未來式
// （「等你核准後才會正式送出給賣家」）——後者是 forbiddenPhrases 至今仍不列
// 裸字「送出」的真正理由：未來式句子裡沒有任何否定詞可供辨識，只有
// 「已／已經／了」這個完成貌標記能把它跟「已經送出」分開。
func TestValidateSurfaceBlocks_Passes_NotYetSentNegation(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"報價單已經確認好了，等你核准後才會正式送出給賣家，目前尚未送出。",
		"報價單已經確認好了，還沒送出，等你核准之後才會送出。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		require.True(t, ok, "text=%q reason=%s", text, reason)
	}
}

// 以下測試是 v6 設計書 §2597／§2612／§2880／§2960／§22／§686 copy-alignment
// 的回歸測試：forbiddenPhrases 原本只涵蓋搜尋／送出／發布／通知賣家／交易／
// 金流，沒有涵蓋 v6 凍結的專屬禁止宣稱。

// TestValidateSurfaceBlocks_Rejects_DeliveredOrSeenClaims 涵蓋 §2597
// （不得把 PUBLISHED 寫成已送達）與 §2612／§2880（未取得 discoverability ACK
// 前不得寫「賣家已看到」；email／push 已送達同樣不能宣稱）。
func TestValidateSurfaceBlocks_Rejects_DeliveredOrSeenClaims(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"通知已送達給賣家了。",
		"通知已經送達給賣家了。",
		"賣家已看到你的需求了。",
		"賣家已經看到你的需求了。",
		// 否定偵測到位後 forbiddenPhrases 改列裸字，這四句是原本靠尾詞限定
		// （「已送達給賣家」「賣家已看到你的」）漏掉的肯定宣稱。
		"通知已送達了。",
		"通知已經送達了。",
		"賣家已看到，等回覆就好。",
		"賣家已經看到，等回覆就好。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if ok || reason != "keyword_blocklist" {
			t.Fatalf("text=%q ok=%v reason=%q, want false/keyword_blocklist", text, ok, reason)
		}
	}
}

// TestValidateSurfaceBlocks_Passes_NotYetDeliveredOrSeenNegation 釘住 §2597／
// §2612 的誠實揭露用語：它們本身就會提到「已送達」「已看到」這兩個詞，只是
// 包在否定句裡——「不表示符合資格的賣家已看到、email／push 已送達，也不代表
// 已有人報價」（v6 text.txt:2612 的自然中文改寫）與「未取得 discoverability
// ACK 前不得寫『賣家已看到』」（v6 text.txt:2880）都是合法且必要的否定敘述。
// 現在由 containsForbiddenPhrase 的否定偵測放行（第一句靠「不表示」、第二句
// 靠「未」與「不得」），forbiddenPhrases 因此可以列回裸字。
func TestValidateSurfaceBlocks_Passes_NotYetDeliveredOrSeenNegation(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"不表示符合資格的賣家已看到、email 或 push 已送達，也不代表已有人報價。",
		"未取得 discoverability ACK 前不得寫「賣家已看到」。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockRationale, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		require.True(t, ok, "text=%q reason=%s", text, reason)
	}
}

// TestValidateSurfaceBlocks_Rejects_QuotedClaims 涵蓋 §2597／§2612：不得暗示
// 已有供應商回應報價。否定偵測到位後 forbiddenPhrases 改列裸字「已報價」，
// 因此這裡連「供應商已報價」這種原本漏掉的肯定講法也一起釘住；§2880 核可
// 免責文案含有同一子字串但在否定句裡，見下面
// TestValidateSurfaceBlocks_Passes_NoSupplierQuoteYetNegation。
func TestValidateSurfaceBlocks_Rejects_QuotedClaims(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"已有人報價了，要不要先看看。",
		"已經有人報價了，要不要先看看。",
		"賣家已報價，內容如下。",
		"賣家已經報價，內容如下。",
		"供應商已報價，內容如下。",
		"供應商已經報價，內容如下。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if ok || reason != "keyword_blocklist" {
			t.Fatalf("text=%q ok=%v reason=%q, want false/keyword_blocklist", text, ok, reason)
		}
	}
}

// TestValidateSurfaceBlocks_Passes_NoSupplierQuoteYetNegation 是 §2880 核可
// CTA 免責文案的假陽性防護：「不代表供應商已報價、供貨或成交」本身含有
// 「已報價」這個子字串，是合法且必要的否定敘述。「不代表」與「已報價」在
// 同一子句內（頓號是列舉分隔符，不切斷否定作用域），否定偵測判為否定敘述
// 放行——這裡直接釘住這句話能通過驗證。
func TestValidateSurfaceBlocks_Passes_NoSupplierQuoteYetNegation(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{
		{Kind: domain.BlockRationale, Text: "這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。", ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
	}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.True(t, ok, "reason=%s", reason)
}

// TestValidateSurfaceBlocks_Rejects_AbsoluteSupplierOrDealClaims 涵蓋 §2960：
// 禁止「最佳供應商」「保證成交」這類排名／保證化宣稱——候選比較
// 不做供應商排名，系統也不保證任何一筆 RFQ 會成交。
func TestValidateSurfaceBlocks_Rejects_AbsoluteSupplierOrDealClaims(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"這是目前的最佳供應商。",
		"這筆保證成交，放心。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if ok || reason != "keyword_blocklist" {
			t.Fatalf("text=%q ok=%v reason=%q, want false/keyword_blocklist", text, ok, reason)
		}
	}
}

// TestValidateSurfaceBlocks_Rejects_FormalDealOrOrderClaims 涵蓋 §22／§686／
// §2959／§2960：付款成功前不得宣稱正式成交或訂單成立——acceptedDealPrice 與
// PENDING_PAYMENT order shell 都還不是正式交易。
func TestValidateSurfaceBlocks_Rejects_FormalDealOrOrderClaims(t *testing.T) {
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	for _, text := range []string{
		"已正式成交，恭喜！",
		"已經正式成交，恭喜！",
		"訂單已成立，等待出貨。",
		"訂單已經成立，等待出貨。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if ok || reason != "keyword_blocklist" {
			t.Fatalf("text=%q ok=%v reason=%q, want false/keyword_blocklist", text, ok, reason)
		}
	}
}

func planForFactEcho() domain.DialoguePlanV1 {
	return domain.DialoguePlanV1{
		ActCode:             domain.ActAckAndAskOptional,
		AllowedClaimCodes:   []string{string(domain.ClaimRequestFactRecorded), string(domain.ClaimNextQuestionOptional)},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
	}
}

func argsForFactEcho() domain.SafeRenderArgs {
	return domain.SafeRenderArgs{
		SalientFactLabels:  []string{"配送城市：桃園市", "數量：50", "單位：KG"},
		NextQuestionPrompt: "希望什麼時候送達？",
	}
}

func blockWithText(text string) []RealizedBlock {
	return []RealizedBlock{{
		Kind:       domain.BlockUnderstood,
		Text:       text,
		ClaimCodes: []string{string(domain.ClaimRequestFactRecorded)},
	}}
}

func TestValidateSurfaceBlocks_RejectsWrongCity(t *testing.T) {
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("已經幫你記下台北市的需求"))
	if ok || reason != "fact_mismatch" {
		t.Fatalf("ok=%v reason=%q, want false/fact_mismatch", ok, reason)
	}
}

func TestValidateSurfaceBlocks_RejectsInflatedQuantity(t *testing.T) {
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("已經記下 500 公斤"))
	if ok || reason != "fact_mismatch" {
		t.Fatalf("ok=%v reason=%q, want false/fact_mismatch", ok, reason)
	}
}

func TestValidateSurfaceBlocks_AllowsCommittedFacts(t *testing.T) {
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("桃園市 50 公斤已經記下了"))
	if !ok {
		t.Fatalf("committed 事實不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_AllowsChineseNumerals(t *testing.T) {
	// 「兩項」「下週」不含阿拉伯數字，完全不進 fact-echo 檢查範圍。
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("還有兩項要補，下週再確認"))
	if !ok {
		t.Fatalf("中文數量詞不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_AllowsCityAlias(t *testing.T) {
	// 「桃園」是 committed「桃園市」的別名，正規化後相同。
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("桃園的貨已經記下"))
	if !ok {
		t.Fatalf("城市別名不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_AllowsNumberFromNextQuestion(t *testing.T) {
	// allowed set 取自 SafeRenderArgs 全部四個欄位，不是只有 SalientFactLabels。
	args := argsForFactEcho()
	args.NextQuestionPrompt = "要不要指定 2026-09-30 之前送達？"
	ok, reason := validateSurfaceBlocks(planForFactEcho(), args, domain.Slots{}, blockWithText("2026-09-30 之前送到嗎？"))
	if !ok {
		t.Fatalf("下一題帶的數字不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_AllowsIsoDateRewrittenAsSpoken(t *testing.T) {
	args := argsForFactEcho()
	args.SalientFactLabels = []string{"到貨日期：2026-09-30"}
	ok, reason := validateSurfaceBlocks(planForFactEcho(), args, domain.Slots{}, blockWithText("9月30日送達"))
	if !ok {
		t.Fatalf("ISO 日期改寫成口語不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_RejectsWrongUnit(t *testing.T) {
	ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText("已經記下 50 台斤"))
	if ok || reason != "fact_mismatch" {
		t.Fatalf("ok=%v reason=%q, want false/fact_mismatch", ok, reason)
	}
}

func TestValidateSurfaceBlocks_AllowsTaiwanVariantCityForm(t *testing.T) {
	// whole-branch review I1：committed 寫「臺北市」、模型寫「台北」（或反過來）
	// 都是同一個城市，不得判成 fact_mismatch。delivery_city 是必填欄位，記錄它
	// 的那一輪必定進到 fact-echo，所以這是每一個台北買家都會踩到的路。
	for _, tc := range []struct{ committed, text string }{
		{"配送城市：臺北市", "台北的貨已經記下"},
		{"配送城市：台北市", "臺北的貨已經記下"},
		{"配送城市：臺中市", "台中的貨已經記下"},
	} {
		args := domain.SafeRenderArgs{SalientFactLabels: []string{tc.committed}}
		ok, reason := validateSurfaceBlocks(planForFactEcho(), args, domain.Slots{}, blockWithText(tc.text))
		if !ok {
			t.Fatalf("committed=%q text=%q 不該被擋：reason=%q", tc.committed, tc.text, reason)
		}
	}
}

func TestValidateSurfaceBlocks_AllowsFactsCommittedInEarlierTurns(t *testing.T) {
	// whole-branch review I2：SalientFactLabels 只放「這一輪」recorded 的最多
	// 2 筆，但 buildSurfaceRealizePrompt 會把最近 2 則歷史回覆整段餵給模型。
	// 第 3 輪買家只補了包裝方式時，模型手上握著第 2 輪的城市與數量——承接前文
	// 的自然句必須放行，否則我們餵給模型的東西不在我們允許它講的清單裡。
	committed := domain.Slots{
		Quantity: intPtr(50), Unit: strPtr("KG"), DeliveryCity: strPtr("桃園市"),
		Packaging: strPtr("紙箱"),
	}
	args := domain.SafeRenderArgs{SalientFactLabels: []string{"包裝方式：紙箱"}}
	ok, reason := validateSurfaceBlocks(planForFactEcho(), args, committed, blockWithText("桃園的 50 公斤已經記下"))
	if !ok {
		t.Fatalf("跨輪 committed 事實不該被擋：reason=%q", reason)
	}
}

func TestValidateSurfaceBlocks_StillRejectsValuesOutsideCommittedSlots(t *testing.T) {
	// committed Slots 只擴大到「真的 commit 過的值」：被更正掉的舊值（50→80）
	// 與從未出現過的城市仍然要擋，否則 I2 的修法會變成整層失效。
	committed := domain.Slots{Quantity: intPtr(80), Unit: strPtr("KG"), DeliveryCity: strPtr("桃園市")}
	for _, text := range []string{"已經記下 50 公斤", "台中的貨已經記下"} {
		ok, reason := validateSurfaceBlocks(planForFactEcho(), domain.SafeRenderArgs{}, committed, blockWithText(text))
		if ok || reason != "fact_mismatch" {
			t.Fatalf("text=%q ok=%v reason=%q, want false/fact_mismatch", text, ok, reason)
		}
	}
}

func TestValidateSurfaceBlocks_RejectsCandidateCountInChineseNumerals(t *testing.T) {
	// whole-branch review I3：prompt 指示模型用中文數字，而第三層 fact-echo 的
	// arabicNumberPattern 只認阿拉伯數字，「三個合適的賣家」對它是隱形的。
	// 這句同時捏造 candidate count 與 search-completed 副作用，必須由第二層擋下。
	plan := domain.DialoguePlanV1{
		ActCode:             domain.ActRequestBasicsReady,
		AllowedClaimCodes:   []string{"BASICS_COMPLETE"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
	blocks := []RealizedBlock{{
		Kind: domain.BlockComplete, Text: "需求齊全了，已經幫你找到三個合適的賣家。",
		ClaimCodes: []string{"BASICS_COMPLETE"},
	}}
	ok, reason := validateSurfaceBlocks(plan, domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	if ok || reason != "keyword_blocklist" {
		t.Fatalf("ok=%v reason=%q, want false/keyword_blocklist", ok, reason)
	}
}

func TestValidateSurfaceBlocks_AllowsNegatedSearchStatements(t *testing.T) {
	// 「還沒找到」「尚未找到」是合法且必要的否定敘述。這三句現在有兩道保護：
	// forbiddenPhrases 只列帶完成貌標記的「已找到」（裸字「找到」擋不住的是
	// 未來式「找到之後再通知你」，不是否定句），且否定偵測會放行同一子句內
	// 被「還沒」「尚未」引導的命中。
	for _, text := range []string{
		"還沒找到合適的賣家，需要再等一下。",
		"尚未找到符合條件的候選。",
		"還沒幫你聯絡賣家。",
	} {
		ok, reason := validateSurfaceBlocks(planForFactEcho(), argsForFactEcho(), domain.Slots{}, blockWithText(text))
		if !ok {
			t.Fatalf("否定敘述不該被擋：text=%q reason=%q", text, reason)
		}
	}
}

// ── 否定感知比對（negation-aware matching）─────────────────────────────────
//
// 第二層原本是純 strings.Contains：任何片語 X 都會被「不代表 X」「不表示 X」
// 「尚未 X」命中，而那些正是 v6 要求的誠實揭露講法。前三輪用「加受詞／完成貌
// 尾詞把肯定句與否定句分開」的做法在中文裡不成立（否定句可以、也很自然地把
// 同一個受詞與尾詞整個包進去），現在改判「片語是否以肯定形式出現」：片語每
// 一次命中都往前看到子句開頭，找得到否定引導詞才放行。
//
// 下面三支測試就是這個機制的三類驗收：合法否定放行、肯定宣稱仍被擋、否定詞
// 與宣稱分屬不同子句的繞過嘗試被擋。

func planForNegation() domain.DialoguePlanV1 {
	return domain.DialoguePlanV1{
		ActCode:             domain.ActPublicationApprovalRequired,
		AllowedClaimCodes:   []string{"RFQ_PENDING_APPROVAL"},
		ForbiddenClaimCodes: domain.GlobalForbiddenClaimCodes,
		MaxSentences:        2,
	}
}

// TestValidateSurfaceBlocks_Passes_NegatedClaimsInSameClause 是第一類：合法
// 否定放行。前三句是實測會被舊比對誤傷的自然句——第一句就是上面
// Rejects_DeliveredOrSeenClaims 的 fixture「賣家已看到你的需求了。」去掉
// 「了」加上「不代表」，不是構造出來的邊界案例。其餘是 v6 §2597／§2612／
// §2880／§2960 的原文與自然改寫。
func TestValidateSurfaceBlocks_Passes_NegatedClaimsInSameClause(t *testing.T) {
	for _, text := range []string{
		// 前三輪實測仍然碰撞的三個自然句
		"這不代表賣家已經看到你的需求。",
		"這不代表已經有人報價了。",
		"這不表示通知已送達給賣家。",
		// v6 §2597 原文（「不得」引導，頓號串起三個被禁止的講法）
		"不得把 PUBLISHED 寫成已送達、已報價或已成交。",
		// v6 §2612 自然改寫（Seller → 賣家；一個「不表示」管整串列舉）
		"排程狀態不表示符合資格的賣家已看到、email／push 已送達或已有人報價。",
		// v6 §2960 Copy freeze 的禁令句（「禁止」引導）
		"禁止「最佳供應商」「保證成交」「已下單」這類講法。",
		// 其餘常見否定引導詞的自然改寫
		"這不代表已送達，也不代表已有人報價。",
		"並非已經成交，只是先把需求確認好。",
		// 「不保證成交」是 §2960「保證成交」唯一誠實的講法，而它的引導詞
		// 「不保證」橫跨片語起點——negatedInSameClause 把搜尋範圍延伸到片語
		// 結尾就是為了這一句。
		"系統不保證成交，也不會自動下單。",
		"尚未確認賣家已看到你的需求。",
		"沒有任何賣家已看到你的需求。",
	} {
		// 先確認這句 fixture 真的含有 blocklist 片語，否則它根本沒有經過
		// 否定偵測，是一支永遠不會失敗的測試。
		if !containsPhraseLiterally(text) {
			t.Errorf("fixture 不含任何 forbiddenPhrases 片語，測不到否定偵測：text=%q", text)
			continue
		}
		blocks := []RealizedBlock{
			{Kind: domain.BlockRationale, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if !ok {
			t.Errorf("合法否定敘述被擋：text=%q reason=%q", text, reason)
		}
	}
}

// containsPhraseLiterally 是改動前那個純子字串比對，只給測試用：用來證明
// 「合法否定放行」的 fixture 確實會被舊比對命中，否定偵測才是被測到的東西。
func containsPhraseLiterally(text string) bool {
	for _, phrase := range forbiddenPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// TestValidateSurfaceBlocks_Rejects_AffirmativeClaimsAfterNegationAwareness 是
// 第二類的補充：既有的每一支 reject 測試都是這次改動的迴歸網（一條都沒動），
// 這裡再釘住幾句「沒有任何否定引導詞的裸肯定句」，確保否定偵測沒有把預設放寬。
func TestValidateSurfaceBlocks_Rejects_AffirmativeClaimsAfterNegationAwareness(t *testing.T) {
	for _, text := range []string{
		"已送達給賣家了。",
		"賣家已經看到你的需求。",
		"已經有人報價了。",
		"供應商已報價、也願意供貨。",
		"我已經送出給賣家了。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockComplete, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if ok || reason != "keyword_blocklist" {
			t.Errorf("text=%q ok=%v reason=%q, want false/keyword_blocklist", text, ok, reason)
		}
	}
}

// TestValidateSurfaceBlocks_Rejects_NegationInDifferentClause 是第三類：繞過
// 嘗試。否定引導詞只管到子句結束——沒有這個限制，模型只要在段落開頭放一句
// 「這不代表…」就能解除整段的防護。每個變體用不同的子句邊界形式。
func TestValidateSurfaceBlocks_Rejects_NegationInDifferentClause(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"全形逗號", "這不代表我沒做事，我已經送出給賣家了。"},
		{"全形句號＋第二次出現", "這不代表已經送出。我已經送出給賣家了。"},
		{"換行", "這不表示賣家已看到\n賣家已看到你的需求了。"},
		{"分號", "這不代表已送達；通知已送達了。"},
		{"半形逗號", "這不代表我沒做事, 我已經送出給賣家了。"},
		{"半形句號", "這不代表已經送出. 我已經送出給賣家了."},
		{"轉折連接詞但（無標點）", "還沒核准但已經送出給賣家了。"},
		{"轉折連接詞而是（無標點）", "這不是取消而是已經送出給賣家了。"},
		{"轉折連接詞不過（無標點）", "尚未核准不過已經送出給賣家了。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := []RealizedBlock{
				{Kind: domain.BlockComplete, Text: tc.text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
			}
			ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
			if ok || reason != "keyword_blocklist" {
				t.Errorf("text=%q ok=%v reason=%q, want false/keyword_blocklist", tc.text, ok, reason)
			}
		})
	}
}

// TestValidateSurfaceBlocks_Rejects_NegationInDifferentClause_CausalAndContinuativeConnectors
// 是 clauseBoundaryWords 從「只收轉折詞」擴成「因果／承接／讓步」整類連接詞
// 之後的回歸測試（I1）。前兩句是 controller 獨立實測確認過的繞過（第一句只
// 比 TestValidateSurfaceBlocks_Rejects_NegationInDifferentClause 的「轉折連接詞
// 但」多換一個連接詞；第二句是 ActPublicationApprovalRequired 階段最典型的
// 幻覺句型之一，否定掛在一個獨立前置子句，不需要任何否定框架的包裝意圖）。
// 其餘是同一類連接詞的變體，逐一驗證擴充後的清單真的收齊。
func TestValidateSurfaceBlocks_Rejects_NegationInDifferentClause_CausalAndContinuativeConnectors(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"因果連接詞所以（無標點，controller 實測確認的繞過）", "還沒核准所以我已經送出給賣家了。"},
		{"承接連接詞就（無標點，controller 實測確認的繞過）", "你還沒回覆我就先幫你送出了。"},
		{"因果連接詞因此（無標點）", "還沒核准因此已經送出給賣家了。"},
		{"因果連接詞因而（無標點）", "還沒核准因而已經送出給賣家了。"},
		{"因果連接詞於是（無標點）", "還沒核准於是已經送出給賣家了。"},
		{"因果連接詞結果（無標點）", "還沒核准結果已經送出給賣家了。"},
		{"承接連接詞然後（無標點）", "還沒核准然後已經送出給賣家了。"},
		{"承接連接詞接著（無標點）", "還沒核准接著已經送出給賣家了。"},
		{"讓步連接詞還是（無標點）", "你還沒回覆我還是先幫你送出了。"},
		{"讓步句型雖然…還是（無標點，驗證還是單獨已擋得住不需另收雖然）", "雖然還沒核准還是已經送出給賣家了。"},
		{"承接連接詞仍然（無標點）", "你還沒回覆我仍然已經送出給賣家了。"},
		{"承接連接詞仍（無標點）", "你還沒回覆我仍已經送出給賣家了。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := []RealizedBlock{
				{Kind: domain.BlockComplete, Text: tc.text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
			}
			ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
			if ok || reason != "keyword_blocklist" {
				t.Errorf("text=%q ok=%v reason=%q, want false/keyword_blocklist", tc.text, ok, reason)
			}
		})
	}
}

// TestValidateSurfaceBlocks_Passes_ModalNegationParaphrase 是 negationCues 加入
// 「不能」的回歸測試（m1）。「不能保證」在「不」與「保證」之間插了「能」，字面
// 上不是「不保證」這個 cue 的子字串，但語意等價；scratchpad 實測過拿掉「不能」
// 這句會被誤擋（沒有其他 cue 命中）。
func TestValidateSurfaceBlocks_Passes_ModalNegationParaphrase(t *testing.T) {
	blocks := []RealizedBlock{
		{Kind: domain.BlockRationale, Text: "系統不能保證成交，也不會自動下單。", ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
	}
	ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
	require.True(t, ok, "reason=%s", reason)
}

// TestValidateSurfaceBlocks_Passes_ExcludedFromClauseBoundaryWords 釘住 I1 的
// 排除決定：「才」與「而」在 reviewer 建議清單裡，但 scratchpad 實測後主動不
// 收——它們是常見詞「剛才」「方才」「反而」「進而」「從而」的組成部分，加進
// clauseBoundaryWords 會把這些詞誤判成子句邊界，讓否定詞被排除在窗口外。這裡
// 釘住這幾句仍然合法放行，防止日後有人照抄 reviewer 清單原樣把「才」「而」
// 加回去、卻沒重新踩到這個誤擋。
func TestValidateSurfaceBlocks_Passes_ExcludedFromClauseBoundaryWords(t *testing.T) {
	for _, text := range []string{
		"這不代表剛才已經送出。",
		"這不代表方才已經送出。",
		"這不代表反而已經送出。",
		"這不代表進而已經送出。",
		"這不代表從而已經送出。",
	} {
		blocks := []RealizedBlock{
			{Kind: domain.BlockRationale, Text: text, ClaimCodes: []string{"RFQ_PENDING_APPROVAL"}},
		}
		ok, reason := validateSurfaceBlocks(planForNegation(), domain.SafeRenderArgs{}, domain.Slots{}, blocks)
		if !ok {
			t.Errorf("text=%q 不該被擋（若被擋，代表「才」或「而」被重新加回 clauseBoundaryWords 了）：reason=%q", text, reason)
		}
	}
}

// TestContainsForbiddenPhrase_ClauseBoundaryMechanics 直接釘住比對機制本身，
// 不經過 validateSurfaceBlocks——這裡要測的字串帶阿拉伯數字，走完整驗證會先
// 被第三層 fact-echo 攔下，測不到第二層。
//
// 半形句點與逗號同時是小數點與千分位，只有後面不接數字時才算子句邊界；
// 頓號則從來不是邊界（列舉分隔符，v6 §2597／§2612 的否定原文靠它串列舉）。
func TestContainsForbiddenPhrase_ClauseBoundaryMechanics(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"千分位逗號不切斷否定", "這不代表 1,000 公斤已送達。", false},
		{"小數點不切斷否定", "這不代表 1.5 噸已送達。", false},
		{"句末半形句號仍是邊界", "這不代表已送達. 貨已送達了。", true},
		{"頓號不是子句邊界", "不表示賣家已看到、已送達或已有人報價。", false},
		{"引號不是子句邊界", "不得寫「賣家已看到」。", false},
		{"冒號不是子句邊界", "提醒：不代表已送達。", false},
		{"否定詞在片語之後不算否定", "已送達給賣家了，這不代表什麼。", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsForbiddenPhrase(tc.text); got != tc.want {
				t.Errorf("containsForbiddenPhrase(%q)=%v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// TestForbiddenPhrases_ContainNoNegationCue 釘住 negatedInSameClause 延伸搜尋
// 範圍所依賴的不變式：沒有任何否定引導詞是某條 forbiddenPhrases 的子字串。
//
// 這條不變式擋的不是「片語會被自己內建的引導詞放行自己」——start+i < at 排除
// 了落在片語內部的引導詞命中，一個片語自己那次命中不會因為自帶引導詞就永遠
// 放行（scratchpad 實測過：混進一條含「未」的片語「尚未送出就已送出」，「尚未
// 送出就已送出給賣家了。」仍然 BLOCK）。
//
// 它真正擋的是同一子句「後面」的其他片語：一條帶引導詞的片語只要出現在子句
// 前段，它內建的引導詞就會落在後面其他片語的搜尋窗口裡，把它們一起放行——
// 等於這條片語變成一個沒人審查過、跟著 blocklist 一起偷渡進來的否定 cue
// （scratchpad 實測：混進「尚未送出就已送出」後，同一子句後面接一句本該被擋
// 的「賣家已看到你的需求了」，那次命中被單獨判成 negated=true）。這條不變式
// 一旦被破壞，日後有人加片語時很難注意到自己順手放寬了同子句裡所有其他片語
// 的防護——不是那一條片語自己失效，是它偷偷幫別的片語開後門。
func TestForbiddenPhrases_ContainNoNegationCue(t *testing.T) {
	for _, phrase := range forbiddenPhrases {
		for _, cue := range negationCues {
			if strings.Contains(phrase, cue) {
				t.Fatalf("forbiddenPhrase %q 內含否定引導詞 %q，negatedInSameClause 會把同一子句後面其他片語的命中一起放行", phrase, cue)
			}
		}
	}
}

// --- ① 阿拉伯數字改口徑：缺漏欄位數是模型可以講的推導值 ---

func TestFactEcho_AllowsMissingRequiredCount(t *testing.T) {
	args := domain.SafeRenderArgs{MissingRequiredLabels: []string{"配送城市", "單位"}}
	// prompt 授權模型說明「還缺幾個欄位」；那個數字不在任何標籤字面裡，
	// 必須由 MissingRequiredLabels 的長度推導進允許集合。
	if factEchoViolated(domain.Slots{}, args, "還缺 2 項要補。") {
		t.Fatal("缺漏欄位數 2 應該放行——它是 prompt 明文授權模型陳述的推導值")
	}
}

func TestFactEcho_StillRejectsFabricatedCount(t *testing.T) {
	args := domain.SafeRenderArgs{MissingRequiredLabels: []string{"配送城市", "單位"}}
	// 候選/賣家數量不是任何推導值，模型無從知道 ⇒ 必須擋。
	if !factEchoViolated(domain.Slots{}, args, "已經幫你找到 3 個合適的賣家。") {
		t.Fatal("捏造的賣家數量 3 必須被擋——它不是任何 committed 事實也不是推導計數")
	}
}

func TestFactEcho_OnlyMissingCountIsDerived(t *testing.T) {
	// 刻意只把「缺漏欄位數」放進允許集合：多開一個推導值就多一組巧合碰撞。
	args := domain.SafeRenderArgs{
		MissingRequiredLabels: []string{"單位"},
		SalientFactLabels:     []string{"品項：高麗菜", "配送城市：桃園市"},
	}
	if !factEchoViolated(domain.Slots{}, args, "已經記下 2 項。") {
		t.Fatal("SalientFactLabels 的長度不該進允許集合——prompt 沒有授權模型陳述它")
	}
}

// --- ③ 「早就」「本來就」裡的「就」不是子句邊界 ---

func TestClauseBoundary_JiuInFixedAdverbIsNotBoundary(t *testing.T) {
	for _, s := range []string{
		"這不代表早就已經送出給賣家了。",
		"這不表示本來就已經通知賣家。",
	} {
		if containsForbiddenPhrase(s) {
			t.Fatalf("合法否定敘述被誤擋：%q（「早就／本來就」裡的「就」是固定副詞的一部分，不是子句邊界）", s)
		}
	}
}

func TestClauseBoundary_JiuAsConnectiveStillBlocks(t *testing.T) {
	// 收窄「就」不得放掉它原本擋住的真繞過。
	if !containsForbiddenPhrase("你還沒回覆我就先幫你送出了。") {
		t.Fatal("「就」當承接連接詞時仍必須是子句邊界——否則否定詞會被錯誤地涵蓋到後面的肯定宣稱")
	}
}
