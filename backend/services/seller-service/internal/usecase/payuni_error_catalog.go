package usecase

import (
	"strings"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// PayuniRemedy 是賣家該採取的動作類別。值與 CodeMeta PayuniErrorRemedy 一致。
type PayuniRemedy string

const (
	RemedySellerFix      PayuniRemedy = "SELLER_FIX"
	RemedyContactSupport PayuniRemedy = "CONTACT_SUPPORT"
	RemedyRetryLater     PayuniRemedy = "RETRY_LATER"
)

// 補件步驟，值與 hawkr_seller 的 CertificationRetryStep 一致。
//
// stepBankBook 目前沒有任何代碼對應，這是刻意的：BindSellerRequest
// （pkg/payuni/client.go）根本不送銀行欄位，所以 PayUni 的銀行類錯誤碼
// （AGENT03013-03017 等）不可能是賣家在 App 改得掉的，它們歸 CONTACT_SUPPORT。
// 常數保留，等哪天 bind_merchant 真的開始送銀行資料時才會有代碼落到這一步。
const (
	stepIDCard      = "idCard"
	stepBankBook    = "bankBook" //nolint:unused // 見上方說明
	stepCompanyInfo = "companyInfo"
	stepOtherInfo   = "otherInfo"
	stepStart       = "start"
)

// payuniUnknownErrorCode 是 fail-closed 文案在 CodeMeta 目錄裡的 key。它不是
// PayUni 的代碼，是我們自己的——登在同一份目錄是為了讓這句賣家看得到的話與其他
// 136 筆共用一個 SSOT，改文案不必動 Go。
const payuniUnknownErrorCode = "PAYUNI_UNKNOWN"

// payuniUnknownErrorFallback 是目錄本身查不到 PAYUNI_UNKNOWN 時的最後防線。
// 留這份同字串的理由：這條路徑存在的目的就是「不讓賣家看到一片空白」，如果連
// fallback 都依賴目錄、而目錄正好壞了，就會退回它要修的那個原點。
const payuniUnknownErrorFallback = "申請未通過，請聯繫客服協助確認"

// payuniUnknownErrorMessage 回傳 fail-closed 文案。
func payuniUnknownErrorMessage() string {
	if label := codemeta.PayuniSellerFacingErrorLabel(payuniUnknownErrorCode); label != payuniUnknownErrorCode {
		return label
	}
	return payuniUnknownErrorFallback
}

type payuniErrorGroup struct {
	remedy PayuniRemedy
	step   string // 僅 RemedySellerFix 使用
	codes  []string
}

// payuniErrorGroups 依「參數來源」分群，不是依 PayUni 的文件章節分群：
// PayUni 的分組講的是它自己哪個模組報錯，賣家該不該動手取決於那個欄位是誰填的。
// 對照 internal/client/payuni/client.go 的 BindSellerRequest 組裝。
var payuniErrorGroups = []payuniErrorGroup{
	// 平台憑證與代理商設定：賣家完全碰不到。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT01001", "AGENT01002", "AGENT01003", "AGENT01004", "AGENT01005",
		"AGENT05001", "AGENT05002", "AGENT05003", "AGENT05004", "AGENT05005",
		"AGENT05006", "AGENT05007", "AGENT05008",
	}},
	// 物流 API 專用，目前無呼叫端（spec 不做項）。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT02001", "AGENT02002", "AGENT02003", "AGENT02004", "AGENT02005",
	}},
	// 平台送出的固定參數或平台生成值被拒。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT04000", "AGENT04024", "AGENT04025", // MerID
		"AGENT04005", "AGENT04006", "AGENT07026", // MerURL
		"AGENT04012", "AGENT04013", "AGENT04014", // SellType
		"AGENT04015", "AGENT04016", "AGENT04026", // MerMCC
		"AGENT04018", "AGENT04019", "AGENT04020", // GoodsRate
		"AGENT07001", "AGENT07002", "AGENT07025", "AGENT07030", "AGENT07031", // Payment
		"AGENT07004", "AGENT07005", "AGENT07020", "AGENT07021", "AGENT07024", // 費率／撥款／提領
		"AGENT07009",                             // MerType
		"AGENT07011", "AGENT07012", "AGENT07013", // DeliverTime
		"AGENT07014", "AGENT07022", // GoodsRate 比例
		"AGENT07015",                             // EscrowType
		"AGENT07016", "AGENT07017", "AGENT07028", // MerUnitPrice
		"AGENT07018", "AGENT07019", // 交易類型
		"AGENT07010", "AGENT07023", "AGENT07027", // 出口 IP／商店歸屬
	}},
	// PayUni 要的欄位我方根本沒送——賣家在 App 改什麼都不會變。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT03011", "AGENT03012", // 會員登入帳號
		"AGENT03013", "AGENT03014", "AGENT03015", "AGENT03016", "AGENT03017", // 銀行帳戶
		"AGENT03020", "AGENT03022", "AGENT03023", // 銀行資料操作
		"AGENT03024",                             // 會員編號
		"AGENT04008", "AGENT04009", "AGENT04010", // MerTel
		"AGENT06002", // 聯絡人英文姓名
	}},
	// PayUni 端的帳戶狀態，我方改不動。
	{remedy: RemedyContactSupport, codes: []string{"AGENT03019", "AGENT03031"}},
	// AGENT04017 先走 recoverFromExistingMerchant 自癒；自癒失敗才會走到這裡。
	{remedy: RemedyContactSupport, codes: []string{"AGENT04017"}},
	// PayUni 內部暫時性失敗。
	{remedy: RemedyRetryLater, codes: []string{
		"AGENT03021", "AGENT03025", "AGENT03026", "AGENT04021",
		"AGENT07003", "AGENT07006", "AGENT07007", "AGENT07008",
	}},
	// 賣家可修：身分與姓名。
	{remedy: RemedySellerFix, step: stepIDCard, codes: []string{
		"AGENT03001", "AGENT03002", "AGENT03003", "AGENT03004",
		"AGENT03027", "AGENT03028",
		"AGENT08001", "AGENT08002", "AGENT08004",
	}},
	// 賣家可修：商店與企業資料。
	{remedy: RemedySellerFix, step: stepCompanyInfo, codes: []string{
		"AGENT03029",
		"AGENT04003", "AGENT04004", "AGENT04007",
		"AGENT04011", "AGENT04027",
		"AGENT04022", "AGENT04023",
		"AGENT04028", "AGENT04029",
	}},
	// 賣家可修：聯絡方式。
	{remedy: RemedySellerFix, step: stepOtherInfo, codes: []string{
		"AGENT03005", "AGENT03006", "AGENT03007",
		"AGENT03008", "AGENT03009", "AGENT03010",
		"AGENT04001", "AGENT04002",
		"AGENT06001", "AGENT06003", "AGENT06004", "AGENT06005",
		"AGENT06006", "AGENT06007", "AGENT06008", "AGENT06009",
		"AGENT08005", "AGENT09001", "AGENT09002", "AGENT09003",
	}},
	// 兩軸的錯誤態。
	{remedy: RemedySellerFix, step: stepStart, codes: []string{
		"MER_8", "MER_91", "MER_92", "MEMBER_5", "MEMBER_8", "MEMBER_92",
	}},
	{remedy: RemedyContactSupport, codes: []string{"MER_2", "MER_99", "MEMBER_99"}},
	// 兩軸的正常態：不會下發，remedy 留空。收進表是為了讓覆蓋率測試能對整個值域
	// 做全稱檢查——漏掉一個值域成員與漏掉一個錯誤碼一樣是缺口。
	{remedy: "", codes: []string{"MER_1", "MER_9", "MEMBER_1", "MEMBER_9"}},
}

var payuniErrorIndex = buildPayuniErrorIndex()

func buildPayuniErrorIndex() map[string]payuniErrorGroup {
	idx := make(map[string]payuniErrorGroup)
	for _, g := range payuniErrorGroups {
		for _, code := range g.codes {
			idx[code] = g
		}
	}
	return idx
}

// PayuniErrorDisplay 是賣家端契約要下發的四個值。
type PayuniErrorDisplay struct {
	Code      string
	Message   string
	Remedy    PayuniRemedy
	RetryStep string
}

// ResolvePayuniError 把一個 error_code 解析成賣家看得到的內容。
//
// reason 是 PayUni 針對這家店寫的審核意見（search_merchant_status 才有），
// 非空時取代目錄文案——它比任何通用文案都具體。目錄文案只在 reason 空白時上場。
//
// 代碼不在目錄裡時 fail-closed：通用文案 ＋ CONTACT_SUPPORT，絕不顯示 PayUni 原始
// 文字，也絕不猜測分類。呼叫端負責把這個情況記進 log。
func ResolvePayuniError(code, reason string) PayuniErrorDisplay {
	if code == "" {
		return PayuniErrorDisplay{}
	}
	message := strings.TrimSpace(reason)
	group, known := payuniErrorIndex[code]
	if !known {
		// reason 優先序排第一（spec §5.2），即使代碼不在目錄裡——PayUni 回了目錄
		// 沒登錄的新代碼，但同時給了具體審核意見時，不能讓 reason 被通用文案蓋掉。
		// remedy 仍固定 CONTACT_SUPPORT：未知代碼的處置分類不因為有 reason 就改變，
		// 只是訊息換成更具體的文字。
		if message == "" {
			message = payuniUnknownErrorMessage()
		}
		return PayuniErrorDisplay{Code: code, Message: message, Remedy: RemedyContactSupport}
	}
	if message == "" {
		message = codemeta.PayuniSellerFacingErrorLabel(code)
		// CodeMeta 對未登錄值原樣回傳 value，那代表 CodeMeta 與群組表不同步。
		if message == code {
			message = payuniUnknownErrorMessage()
		}
	}
	return PayuniErrorDisplay{Code: code, Message: message, Remedy: group.remedy, RetryStep: group.step}
}

// IsPayuniAlreadyExistsError 表示 MerID 在 PayUni 端已存在，呼叫端應先走
// search_merchant_status 自癒路徑而不是直接把錯誤丟給賣家。
func IsPayuniAlreadyExistsError(code string) bool { return code == "AGENT04017" }

// PayuniErrorDisplayFor 解出一筆註冊「此刻」該讓賣家看到的錯誤內容。
//
// 權威是當前的 MerStatus/MemberStatus，不是 reg.ErrorCode。理由是一個實際會發生
// 的狀況：applyOutcome 的 pending_info 與 under_review 分支都不呼叫 Set*（既有設計，
// ErrorMessage 一直如此），所以一筆曾經 rejected 的註冊在賣家補件後、狀態回到待補件
// 時，reg.ErrorCode 仍留著上一輪的 MER_92。照著它顯示，賣家會看到「審核未通過」，
// 而此刻真實狀態是「等你補件」——顯示過期的原因比不顯示更糟。
//
// DB 的 error_code 因此刻意與畫面不同步：它留著最後一次失敗的碼供客服追查，
// 畫面顯示的是當前狀態。
//
// 例外是 bind 階段：那時 PayUni 還沒給過任何 MerStatus，outcome 無從推導，
// 此時才回頭用 reg.ErrorCode 裡的 AGENT 碼。
func PayuniErrorDisplayFor(reg *domain.PayuniRegistration) PayuniErrorDisplay {
	if reg == nil {
		return PayuniErrorDisplay{}
	}
	switch outcome := derivePayuniOutcome(reg.MerStatus, reg.MemberStatus); outcome {
	case outcomeApproved, outcomeUnderReview:
		// 沒有要給賣家看的錯誤。曾經失敗過也一樣——那是歷史，不是現況。
		return PayuniErrorDisplay{}
	case outcomeRejected, outcomePendingInfo:
		return ResolvePayuniError(payuniStatusCode(reg.MerStatus, reg.MemberStatus), reg.Reason)
	}
	// outcome 無法辨識：bind 階段尚無 MerStatus，或 PayUni 回了未記載的狀態值。
	if reg.ErrorCode != "" {
		return ResolvePayuniError(reg.ErrorCode, reg.Reason)
	}
	if reg.Status == domain.PayuniStatusFailed {
		// 失敗了卻沒有業務代碼：transport／加解密失敗走這條路（error_message 是原始
		// Go error 字串，絕不能下發）。另一種情形是先前的 AGENT 碼被一次連線失敗的
		// upsert 覆蓋成空。無論哪種，讓賣家看到一片空白就是回到這次要修的原點——
		// 走 fail-closed，至少告訴他該找客服。
		return PayuniErrorDisplay{Message: payuniUnknownErrorMessage(), Remedy: RemedyContactSupport}
	}
	return PayuniErrorDisplay{}
}

// IsPayuniUnknownErrorMessage 讓呼叫端辨認「這是 fail-closed 文案」以便記 log。
func IsPayuniUnknownErrorMessage(msg string) bool { return msg == payuniUnknownErrorMessage() }
