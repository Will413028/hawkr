package model

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// PayUniBindStatus 是 PayUni 子商店註冊的綁定狀態（ADR 2026-10-19-seller-kyc-payuni-consolidated「核准與 PayUni 綁定」）：
// 核准寫入意圖時是 PENDING，job 呼叫 PayUni 成功是 BOUND，被永久拒絕、資料不完整或重試用完是 FAILED（管理員重新綁定）。
type PayUniBindStatus string

const (
	// BindPending 意圖已寫入，job 還沒有送出或正在重試。
	BindPending PayUniBindStatus = "PENDING"
	// BindBound PayUni 已收下註冊（或查詢確認它已存在並認領）。
	BindBound PayUniBindStatus = "BOUND"
	// BindFailed 綁定失敗，要管理員重新綁定。
	BindFailed PayUniBindStatus = "FAILED"
)

// PayUniBindStatusValues 回傳全部綁定狀態，與 DB 的 payuni_registrations_bind_status_check 一致。
func PayUniBindStatusValues() []PayUniBindStatus {
	return []PayUniBindStatus{BindPending, BindBound, BindFailed}
}

// PayUniOutcome 是 PayUni 回報的原碼（MerStatus、MemberStatus）推導出的審核結果；原碼是外部事實，這是我們的解讀。
type PayUniOutcome string

const (
	// OutcomeNone 還沒有任何狀態的觀測。
	OutcomeNone PayUniOutcome = "NONE"
	// OutcomeUnderReview PayUni 審核中。
	OutcomeUnderReview PayUniOutcome = "UNDER_REVIEW"
	// OutcomePendingInfo PayUni 要求補件。
	OutcomePendingInfo PayUniOutcome = "PENDING_INFO"
	// OutcomeApproved PayUni 核准，可以收款。
	OutcomeApproved PayUniOutcome = "APPROVED"
	// OutcomeRejected PayUni 拒絕或關閉。
	OutcomeRejected PayUniOutcome = "REJECTED"
	// OutcomeUnrecognized PayUni 回了沒有記載的狀態碼：fail closed，不當成核准，也不發通知。
	OutcomeUnrecognized PayUniOutcome = "UNRECOGNIZED"
)

// PayUniOutcomeValues 回傳全部 outcome，與 DB 的 payuni_registrations_outcome_check 一致。
func PayUniOutcomeValues() []PayUniOutcome {
	return []PayUniOutcome{OutcomeNone, OutcomeUnderReview, OutcomePendingInfo, OutcomeApproved, OutcomeRejected, OutcomeUnrecognized}
}

// PayUniSource 是一筆觀測的來源。
type PayUniSource string

const (
	// SourceWebhook PayUni 的 AGENT_NOTIFY（經 Java payuni-webhook-service 與 legacy consumer）。
	SourceWebhook PayUniSource = "WEBHOOK"
	// SourcePoll 定期輪詢尚未有定論的註冊。
	SourcePoll PayUniSource = "POLL"
	// SourceRefresh 管理員手動刷新。
	SourceRefresh PayUniSource = "REFRESH"
	// SourceBind 綁定（含 AGENT04017 認領時查到的狀態）。
	SourceBind PayUniSource = "BIND"
)

// PayUniSourceValues 回傳全部來源，與 DB 的 payuni_observations_source_check 一致。
func PayUniSourceValues() []PayUniSource {
	return []PayUniSource{SourceWebhook, SourcePoll, SourceRefresh, SourceBind}
}

// 我們自己的錯誤碼（PayUni 的碼是 AGENT 開頭，原樣保存）。bind_error_code 與 sync_error_code 都只存這兩種碼。
const (
	// ErrIncompleteData 綁定需要的資料不完整（例如店家沒有地址）：不送假資料給 PayUni（盤點 D15）。資料補齊後（店家改地址）
	// 會自動回到 PENDING 重送。
	ErrIncompleteData = "INCOMPLETE_DATA"
	// ErrNotConfigured 沒有設定 PayUni：設定好之後由管理員重新綁定。
	ErrNotConfigured = "PAYUNI_NOT_CONFIGURED"
	// ErrUnavailable PayUni 暫時不可用，重試用完。
	ErrUnavailable = "PAYUNI_UNAVAILABLE"
	// ErrMerchantTerminal MerID 在 PayUni 端已存在而且是退件或關閉的終止狀態：同一個 stint 無法再綁定，要關店重新入駐。
	ErrMerchantTerminal = "MERCHANT_TERMINAL"
)

// PayUniRemedy 是使用者該採取的動作類別（舊 CodeMeta 的 PayuniErrorRemedy；顯示文字在各 client 的呈現層）。
type PayUniRemedy string

const (
	// RemedySellerFix 賣家可以修正。
	RemedySellerFix PayUniRemedy = "SELLER_FIX"
	// RemedyContactSupport 賣家改不動，聯絡客服。
	RemedyContactSupport PayUniRemedy = "CONTACT_SUPPORT"
	// RemedyRetryLater PayUni 暫時性的失敗，稍後重試。
	RemedyRetryLater PayUniRemedy = "RETRY_LATER"
)

// 賣家可修的錯誤要補的申請欄位（KYCFieldNames 的路徑）與店家欄位（StoreFieldNames 的路徑）：PayUni 的錯誤碼依參數來源分群，每群對應申請上或店家上的一組欄位，
// 與退回時的 KYCIssue.Field 是同一套語彙。個人與企業專屬的欄位都列在群組裡，ProblemOf 再依申請的賣家型態過濾。
var (
	// 身分與姓名。
	identityFields = []string{
		"real_name", "birth_date", "id_card_issue_date", "id_card_issue_type", "id_card_issue_location",
		"individual.id_card_no", "business.rep_id_card_no", "id_card_front", "id_card_back",
	}
	// 商店與企業資料。
	businessFields = []string{
		"english_name", "product_type", "website_url", "business.company_name",
		"business.tax_id", "business.company_register_date", "business.company_address",
		"business.company_phone", "business.company_documents",
	}
	// 聯絡方式：店家的聯絡人姓名、聯絡人手機與客服信箱（店家欄位，不是申請欄位）。
	contactStoreFields = []string{StoreFieldContactName, StoreFieldContactMobile, StoreFieldSupportEmail}
	// 商店電話：店家的聯絡電話。
	merTelStoreFields = []string{StoreFieldContactPhone}
	// 整份店家資料：PayUni 退件時，哪個欄位都可能要改。
	allStoreFields = StoreFieldNames()
)

// 店家上可以指出「有問題」的欄位路徑（與對外 Store 的欄位一對一）：送出 KYC 之前還缺的店家欄位與 PayUni 問題要補的店家欄位
// 都用這一套。
const (
	StoreFieldAddress       = "address"
	StoreFieldContactName   = "contact_name"
	StoreFieldContactMobile = "contact_mobile"
	StoreFieldSupportEmail  = "support_email"
	StoreFieldContactPhone  = "contact_phone"
)

// StoreFieldNames 回傳全部店家欄位路徑，依對外 StoreField 的數字順序。
func StoreFieldNames() []string {
	return []string{StoreFieldAddress, StoreFieldContactName, StoreFieldContactMobile, StoreFieldSupportEmail, StoreFieldContactPhone}
}

type payuniErrorGroup struct {
	remedy PayUniRemedy
	// wholeForm 表示碼指出的是整份表單（哪個欄位都可能要改）：賣家端不列具體欄位，只告訴他整份要重看。
	wholeForm   bool
	fields      []string // 只有 RemedySellerFix 用：要補的申請欄位；空表示不是申請上的欄位
	storeFields []string // 只有 RemedySellerFix 用：要補的店家欄位；空表示不是店家上的欄位
	codes       []string
}

// payuniErrorGroups 依「參數來源」分群，不是依 PayUni 文件的章節：賣家該不該動手取決於那個欄位是誰填的。這是 PayUni
// 規格的錯誤碼目錄，改動的是外部規格，不是執行期的資料，所以放在 Go（有測試守住分類）而不是表。舊 seller-service 的
// `payuni_error_catalog.go` 搬來，文案（CodeMeta 的 label）屬於 client 的呈現層，不搬。
var payuniErrorGroups = []payuniErrorGroup{
	// 平台憑證與代理商設定：賣家碰不到。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT01001", "AGENT01002", "AGENT01003", "AGENT01004", "AGENT01005",
		"AGENT05001", "AGENT05002", "AGENT05003", "AGENT05004", "AGENT05005", "AGENT05006", "AGENT05007", "AGENT05008",
	}},
	// 物流 API 專用，目前沒有呼叫端。
	{remedy: RemedyContactSupport, codes: []string{"AGENT02001", "AGENT02002", "AGENT02003", "AGENT02004", "AGENT02005"}},
	// 平台送出的固定參數或平台生成的值被拒。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT04000", "AGENT04024", "AGENT04025", // MerID
		"AGENT04005", "AGENT04006", "AGENT07026", // MerURL
		"AGENT04012", "AGENT04013", "AGENT04014", // SellType
		"AGENT04015", "AGENT04016", "AGENT04026", // MerMCC
		"AGENT04018", "AGENT04019", "AGENT04020", // GoodsRate
		"AGENT07001", "AGENT07002", "AGENT07025", "AGENT07030", "AGENT07031", // Payment
		"AGENT07004", "AGENT07005", "AGENT07020", "AGENT07021", "AGENT07024", // 費率、撥款、提領
		"AGENT07009",                             // MerType
		"AGENT07011", "AGENT07012", "AGENT07013", // DeliverTime
		"AGENT07014", "AGENT07022", // GoodsRate 比例
		"AGENT07015",                             // EscrowType
		"AGENT07016", "AGENT07017", "AGENT07028", // MerUnitPrice
		"AGENT07018", "AGENT07019", // 交易類型
		"AGENT07010", "AGENT07023", "AGENT07027", // 出口 IP、商店歸屬
	}},
	// PayUni 要的欄位我方沒送（bind_merchant 不送銀行欄位），賣家改什麼都不會變。
	{remedy: RemedyContactSupport, codes: []string{
		"AGENT03011", "AGENT03012", // 會員登入帳號
		"AGENT03013", "AGENT03014", "AGENT03015", "AGENT03016", "AGENT03017", // 銀行帳戶
		"AGENT03020", "AGENT03022", "AGENT03023", // 銀行資料操作
		"AGENT03024", // 會員編號
		"AGENT06002", // 聯絡人英文姓名
	}},
	// PayUni 端的帳戶狀態，我方改不動。
	{remedy: RemedyContactSupport, codes: []string{"AGENT03019", "AGENT03031"}},
	// AGENT04017（MerID 已存在）由 bind job 先走查詢認領；認領不了才會是這個結果。
	{remedy: RemedyContactSupport, codes: []string{"AGENT04017"}},
	// PayUni 內部的暫時性失敗：job 重試。
	{remedy: RemedyRetryLater, codes: []string{
		"AGENT03021", "AGENT03025", "AGENT03026", "AGENT04021", "AGENT07003", "AGENT07006", "AGENT07007", "AGENT07008",
	}},
	// 賣家可修：身分與姓名。
	{remedy: RemedySellerFix, fields: identityFields, codes: []string{
		"AGENT03001", "AGENT03002", "AGENT03003", "AGENT03004", "AGENT03027", "AGENT03028", "AGENT08001", "AGENT08002", "AGENT08004",
	}},
	// 賣家可修：商店與企業資料。
	{remedy: RemedySellerFix, fields: businessFields, codes: []string{
		"AGENT03029", "AGENT04003", "AGENT04004", "AGENT04007", "AGENT04011", "AGENT04027", "AGENT04022", "AGENT04023",
		"AGENT04028", "AGENT04029",
	}},
	// 賣家可修：商店電話（店家的聯絡電話）。
	{remedy: RemedySellerFix, storeFields: merTelStoreFields, codes: []string{"AGENT04008", "AGENT04009", "AGENT04010"}},
	// 賣家可修：聯絡方式（店家的聯絡人與客服信箱）。
	{remedy: RemedySellerFix, storeFields: contactStoreFields, codes: []string{
		"AGENT03005", "AGENT03006", "AGENT03007", "AGENT03008", "AGENT03009", "AGENT03010", "AGENT04001", "AGENT04002",
		"AGENT06001", "AGENT06003", "AGENT06004", "AGENT06005", "AGENT06006", "AGENT06007", "AGENT06008", "AGENT06009",
		"AGENT08005", "AGENT09001", "AGENT09002", "AGENT09003",
	}},
	// 兩軸的錯誤態（PayUniStatusCode 的輸出）。
	{remedy: RemedySellerFix, wholeForm: true, fields: allFields, storeFields: allStoreFields, codes: []string{"MER_8", "MER_91", "MER_92", "MEMBER_5", "MEMBER_8", "MEMBER_92"}},
	{remedy: RemedyContactSupport, codes: []string{"MER_2", "MER_99", "MEMBER_99"}},
	// 兩軸的正常態：不會下發，remedy 留空；收進表是為了讓覆蓋率測試對整個值域做全稱檢查。
	{remedy: "", codes: []string{"MER_1", "MER_9", "MEMBER_1", "MEMBER_9"}},
	// 我們自己的碼。
	{remedy: RemedySellerFix, codes: []string{ErrIncompleteData}},
	{remedy: RemedyContactSupport, codes: []string{ErrNotConfigured, ErrMerchantTerminal}},
	{remedy: RemedyRetryLater, codes: []string{ErrUnavailable}},
}

var payuniErrorIndex = func() map[string]payuniErrorGroup {
	idx := make(map[string]payuniErrorGroup)
	for _, g := range payuniErrorGroups {
		for _, code := range g.codes {
			idx[code] = g
		}
	}
	return idx
}()

// PayUniProblem 是註冊目前的問題的事實：碼、處置類別與碼指出的申請欄位、店家欄位（未依身分鎖過濾）；Reason 是 PayUni
// 針對這家店寫的審核意見（去掉前後空白，沒有時是 nil）。WholeForm 表示碼指出的是整份表單，Fields 與 StoreFields 是全部欄位。
// 賣家看的是 SellerPayUniOf 的推導。
type PayUniProblem struct {
	Code        string
	Remedy      PayUniRemedy
	WholeForm   bool
	Fields      []string
	StoreFields []string
	Reason      *string
}

// allFields 是整份申請：PayUni 退件、要賣家從頭修正時，哪個欄位都可能要改。
var allFields = KYCFieldNames()

// ClassifyPayUniCode 回傳碼的處置類別與賣家要補的申請欄位、店家欄位。不在目錄裡的碼 fail closed：CONTACT_SUPPORT，絕不猜測
// 分類，known 為 false 讓呼叫方記 log。
func ClassifyPayUniCode(code string) (remedy PayUniRemedy, fields, storeFields []string, known bool) {
	g, ok := payuniErrorIndex[code]
	if !ok {
		return RemedyContactSupport, nil, nil, false
	}
	return g.remedy, slices.Clone(g.fields), slices.Clone(g.storeFields), true
}

// PayUniErrorCodes 回傳目錄裡的全部碼（給覆蓋率測試）。
func PayUniErrorCodes() []string {
	out := make([]string, 0, len(payuniErrorIndex))
	for code := range payuniErrorIndex {
		out = append(out, code)
	}
	slices.Sort(out)
	return out
}

// PayUniAlreadyBound 表示 MerID 在 PayUni 端已存在（AGENT04017）：bind job 先查詢狀態認領，而不是當成失敗。
func PayUniAlreadyBound(code string) bool { return code == "AGENT04017" }

// PayUniRetryable 表示碼是 PayUni 暫時性的失敗，job 該重試而不是記為失敗。
func PayUniRetryable(code string) bool {
	remedy, _, _, known := ClassifyPayUniCode(code)
	return known && remedy == RemedyRetryLater
}

// TerminalMerStatus 表示商家層級的終止狀態（退件、關閉）：同一個 MerID 不會再回到可用，同一個 stint 只能關店重新入駐。
func TerminalMerStatus(merStatus string) bool {
	switch merStatus {
	case "2", "8", "92", "99":
		return true
	}
	return false
}

// payuniAmbiguousOutcomes 是 MerStatus 為 1（核准）或 9（審核中）時，MemberStatus 的各種組合。
var payuniAmbiguousOutcomes = map[[2]string]PayUniOutcome{
	{"1", "1"}: OutcomeApproved, {"1", "5"}: OutcomePendingInfo, {"1", "9"}: OutcomeUnderReview, {"1", ""}: OutcomeUnderReview,
	{"9", "1"}: OutcomeUnderReview, {"9", "5"}: OutcomePendingInfo, {"9", "9"}: OutcomeUnderReview, {"9", ""}: OutcomeUnderReview,
}

// PayUniOutcomeOf 把 PayUni 的 (MerStatus, MemberStatus) 推導成審核結果（舊 `derivePayuniOutcome`，語意不變）。
// 兩個都沒有是 OutcomeNone；認不得的碼是 OutcomeUnrecognized（fail closed）。
func PayUniOutcomeOf(merStatus, memberStatus string) PayUniOutcome {
	if merStatus == "" && memberStatus == "" {
		return OutcomeNone
	}
	switch merStatus {
	case "2", "8", "92", "99": // 終止，MemberStatus 無關
		return OutcomeRejected
	case "91": // 商家層級待補件，MemberStatus 無關
		return OutcomePendingInfo
	}
	if merStatus == "1" || merStatus == "9" {
		switch memberStatus {
		case "8", "92", "99":
			return OutcomeRejected
		}
	}
	if outcome, ok := payuniAmbiguousOutcomes[[2]string{merStatus, memberStatus}]; ok {
		return outcome
	}
	if merStatus == "1" || merStatus == "9" {
		return OutcomeUnderReview
	}
	if memberStatus == "5" {
		return OutcomePendingInfo
	}
	return OutcomeUnrecognized
}

// PayUniStatusCode 是目前狀態在錯誤目錄裡的碼（MER_x 或 MEMBER_x），沒有狀態時空字串。
func PayUniStatusCode(merStatus, memberStatus string) string {
	switch merStatus {
	case "2", "8", "91", "92", "99":
		return "MER_" + merStatus
	}
	switch memberStatus {
	case "5", "8", "92", "99":
		return "MEMBER_" + memberStatus
	}
	if merStatus != "" {
		return "MER_" + merStatus
	}
	return ""
}

// PayUniStatus 是 PayUni 目前回報的狀態：原碼加推導的 outcome。沒有回報的欄位是 nil。
type PayUniStatus struct {
	MerStatus, MemberStatus                       *string
	CreditStatus, ApplePayStatus, GooglePayStatus *string
	ATMStatus                                     *string
	Reason                                        *string
	Outcome                                       PayUniOutcome
	// ObservedAt 是目前狀態的事件時間（PayUni 的 ExecuteTime 或我們收到回應的時間）；CheckedAt 是最近一次問過 PayUni。
	ObservedAt, CheckedAt *time.Time
}

// PayUniRegistration 是店家的 PayUni 子商店註冊（綁定意圖與 PayUni 的狀態）。
type PayUniRegistration struct {
	StoreID       int64
	MerID         string
	ApplicationID uuid.UUID
	BindStatus    PayUniBindStatus
	BindErrorCode *string
	BindAttempts  int
	BoundAt       *time.Time
	// IncompleteStoreFields 是綁定因資料不完整（INCOMPLETE_DATA）而失敗時，店家現在實際缺的欄位（讀註冊時帶上）；問題只指向它們，
	// 缺的不是店家欄位（例如沒有網站根網址的設定）時是空的。
	IncompleteStoreFields []string
	// SellerType 是這份註冊的申請的賣家型態（讀註冊時帶上；要補的欄位依它過濾：個人賣家沒有企業專屬的欄位）。
	SellerType KYCSellerType
	// SyncPending 表示還有資料沒送給 PayUni；SyncErrorCode 是上一次同步被 PayUni 永久拒絕的碼。
	SyncPending   bool
	SyncErrorCode *string
	Status        PayUniStatus
	// PaymentEnabled 是推導的：已綁定而且 PayUni 核准。
	PaymentEnabled bool
	// NeedsReonboarding 表示 MerID 在 PayUni 端是終止狀態：同一個 stint 無法再綁定，要關店重新入駐。
	NeedsReonboarding    bool
	CreatedAt, UpdatedAt time.Time
}

// RebindPayUniInput 是管理員對 FAILED 的註冊重新綁定（新的 job，不是再核准一次）。
type RebindPayUniInput struct {
	StoreID, AdminID int64
}

// PayUniNotification 是 PayUni 的 AGENT_NOTIFY 通知（經 Java payuni-webhook-service 的 envelope）：EventKey 是它的冪等鍵
// （merId:MerStatus:ExecuteTime），ExecuteTime 是 PayUni 的事件時間（yyyyMMddHHmmss，台北時間）。
type PayUniNotification struct {
	MerID, EventKey, MerStatus, MerStatusNote, ExecuteTime string
}

// PayUniObservation 是一筆觀測紀錄。
type PayUniObservation struct {
	ID                                            uuid.UUID
	Source                                        PayUniSource
	EventKey                                      *string
	MerStatus, MemberStatus                       *string
	CreditStatus, ApplePayStatus, GooglePayStatus *string
	ATMStatus, Reason                             *string
	ExecutedAt, ReceivedAt                        time.Time
}

// PayUniObservationCursor 是觀測紀錄的 keyset 位置（新到舊）：最後一筆的事件時間與觀測 ID。
type PayUniObservationCursor struct {
	ExecutedAt    time.Time
	ObservationID uuid.UUID
}

// Valid 回報位置是不是完整的。
func (c PayUniObservationCursor) Valid() bool {
	return c.ObservationID != uuid.Nil() && !c.ExecutedAt.IsZero()
}

// PayUniObservationListInput 是店家觀測紀錄的分頁輸入；PageSize 為 0 用預設值，After 為 nil 從第一頁開始。
type PayUniObservationListInput struct {
	StoreID  int64
	PageSize int32
	After    *PayUniObservationCursor
}

// PayUniObservationPage 是一頁觀測紀錄（新到舊）；Next 為 nil 表示沒有下一頁。
type PayUniObservationPage struct {
	Items []PayUniObservation
	Next  *PayUniObservationCursor
}

// KYCFlowStatus 是 cert 軸與 PayUni 軸合成的流程狀態，給賣家與後台顯示（舊 `cert_status_derive.go` 的推導）。
// 它是推導出來的，不存。
type KYCFlowStatus string

const (
	// FlowNotStarted 沒有申請，或申請已取消。
	FlowNotStarted KYCFlowStatus = "NOT_STARTED"
	// FlowDraft 賣家填寫中。
	FlowDraft KYCFlowStatus = "DRAFT"
	// FlowSubmitted 已送出，等待管理員審核。
	FlowSubmitted KYCFlowStatus = "SUBMITTED"
	// FlowRejected 管理員退回。
	FlowRejected KYCFlowStatus = "REJECTED"
	// FlowBinding 管理員核准，正在向 PayUni 註冊。
	FlowBinding KYCFlowStatus = "BINDING"
	// FlowBindFailed 向 PayUni 註冊失敗，等管理員處理（重新綁定）。
	FlowBindFailed KYCFlowStatus = "BIND_FAILED"
	// FlowForwarded 已向 PayUni 註冊，還沒有審核結果。
	FlowForwarded KYCFlowStatus = "FORWARDED"
	// FlowUnderReview PayUni 審核中。
	FlowUnderReview KYCFlowStatus = "UNDER_REVIEW"
	// FlowPendingInfo PayUni 要求補件。
	FlowPendingInfo KYCFlowStatus = "PENDING_INFO"
	// FlowApproved PayUni 核准，可以收款。
	FlowApproved KYCFlowStatus = "APPROVED"
	// FlowPayUniRejected PayUni 拒絕或關閉。
	FlowPayUniRejected KYCFlowStatus = "PAYUNI_REJECTED"
)

// KYCFlowStatusValues 回傳全部流程狀態。
func KYCFlowStatusValues() []KYCFlowStatus {
	return []KYCFlowStatus{
		FlowNotStarted, FlowDraft, FlowSubmitted, FlowRejected, FlowBinding, FlowBindFailed, FlowForwarded, FlowUnderReview,
		FlowPendingInfo, FlowApproved, FlowPayUniRejected,
	}
}

// IdentityLockedBy 回答註冊（有 MerID）是否已讓 PayUni 端的商店身分欄位固定：PayUni 已經（或正要）收下（bind_status 不是
// FAILED）。update_merchant 不能改核心身分欄位（PayUni 代理商專用 API 總覽 §8），所以 bind_merchant 送出的身分欄位在這之後
// 都不能改（見 CARRYOVER）；更正只能關店、以新的 store_code 重新入駐。
func IdentityLockedBy(reg PayUniRegistration) bool {
	return reg.BindStatus != BindFailed
}

// KYCFlow 是申請的流程狀態與衍生旗標。CanResubmit：草稿、被退回，或已核准而且 SellerPayUniOf 的推導是 FIXABLE（PayUni 拒絕或綁定失敗，
// 而且賣家能自己修正）；IdentityLocked 為真時（註冊已有 MerID），重送只能更正聯絡欄位與地址，身分資料（店名、介紹、姓名或公司名稱、
// 證號、統編、英文名稱、官網、商品類型、賣家型態）鎖住，身分的錯誤只能關店重新入駐。PayUni 的事實（綁定狀態、審核結果、問題、
// 是否終止或可收款）只在 PayUniRegistration，賣家看到的 PayUni 狀態是 SellerPayUniOf，流程不存第二份。
type KYCFlow struct {
	Status         KYCFlowStatus
	CanResubmit    bool
	IdentityLocked bool
}

// KYCFlowStatusOf 是流程狀態（階段）：cert 軸只記人工決定，已核准之後看 PayUni 註冊（沒有時為 nil，綁定中）的階段（PayUniPhaseOf）。
// 只需要階段的呼叫方（批次讀、清單）用它，不必準備 DeriveKYCFlow 的 CanResubmit 要的賣家型態與店家缺的欄位。
func KYCFlowStatusOf(status KYCStatus, reg *PayUniRegistration) KYCFlowStatus {
	switch status {
	case KYCStatusDraft:
		return FlowDraft
	case KYCStatusSubmitted:
		return FlowSubmitted
	case KYCStatusRejected:
		return FlowRejected
	case KYCStatusCancelled:
		// 已取消的申請不是目前的申請：與「還沒有申請」相同。
		return FlowNotStarted
	case KYCStatusApproved:
	}
	if reg == nil {
		return FlowBinding
	}
	return PayUniPhaseOf(*reg)
}

// DeriveKYCFlow 由 cert 軸與 PayUni 註冊（沒有時為 nil）推導流程狀態與旗標。已核准時 CanResubmit 是 sellerPayUniOf 的結果為 FIXABLE，
// 所以 reg 要帶賣家型態與店家缺的欄位（payuni.SellerViewOf 載入的那一份）；SellerPayUniOf 吃同一份。已取消的申請能重送（SaveKYCDraft 版本 0
// 會建立新的），與沒有申請相同。
func DeriveKYCFlow(status KYCStatus, reg *PayUniRegistration) KYCFlow {
	flow := KYCFlow{Status: KYCFlowStatusOf(status, reg), IdentityLocked: reg != nil && IdentityLockedBy(*reg)}
	switch status {
	case KYCStatusDraft, KYCStatusRejected, KYCStatusCancelled:
		flow.CanResubmit = true
	case KYCStatusSubmitted:
	case KYCStatusApproved:
		flow.CanResubmit = reg != nil && sellerPayUniOf(reg).State == PayUniStateFixable
	}
	return flow
}

// PayUniPhaseOf 是註冊目前在 PayUni 的哪個階段：綁定狀態優先，再看審核結果。DeriveKYCFlow 與 SellerPayUniOf 都以它為準。
func PayUniPhaseOf(reg PayUniRegistration) KYCFlowStatus {
	switch reg.BindStatus {
	case BindPending:
		return FlowBinding
	case BindFailed:
		return FlowBindFailed
	case BindBound:
	}
	switch reg.Status.Outcome {
	case OutcomeApproved:
		return FlowApproved
	case OutcomeRejected:
		return FlowPayUniRejected
	case OutcomePendingInfo:
		return FlowPendingInfo
	case OutcomeUnderReview:
		return FlowUnderReview
	case OutcomeNone, OutcomeUnrecognized:
	}
	return FlowForwarded
}

func problemOf(code string, reason *string) *PayUniProblem {
	if code == "" {
		return nil
	}
	remedy, fields, storeFields, _ := ClassifyPayUniCode(code)
	return &PayUniProblem{Code: code, Remedy: remedy, WholeForm: payuniErrorIndex[code].wholeForm, Fields: fields, StoreFields: storeFields, Reason: trimmedReason(reason)}
}

// trimmedReason 去掉審核意見的前後空白；沒有或全是空白時是 nil。
func trimmedReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	t := trimtext.Trim(*reason)
	if t == "" {
		return nil
	}
	return &t
}

// ProblemOf 是註冊目前要讓賣家處理的問題（綁定失敗、PayUni 退件或要求補件），沒有時是 nil；流程狀態（DeriveKYCFlow）與
// 只讀註冊的呼叫方用同一個推導。
func ProblemOf(reg PayUniRegistration) *PayUniProblem {
	var p *PayUniProblem
	switch reg.BindStatus {
	case BindFailed:
		p = problemOf(Deref(reg.BindErrorCode), nil)
	case BindBound:
		switch reg.Status.Outcome {
		case OutcomeRejected, OutcomePendingInfo:
			p = problemOf(PayUniStatusCode(Deref(reg.Status.MerStatus), Deref(reg.Status.MemberStatus)), reg.Status.Reason)
		case OutcomeNone, OutcomeUnderReview, OutcomeApproved, OutcomeUnrecognized:
		}
	case BindPending:
	}
	if p != nil && p.Code == ErrIncompleteData {
		p.StoreFields = slices.Clone(reg.IncompleteStoreFields)
	}
	if p != nil && reg.SellerType != "" {
		p.Fields = slices.DeleteFunc(p.Fields, func(f string) bool { return !FieldAppliesTo(f, reg.SellerType) })
	}
	return p
}

// merIDRe 是 MerID 的格式：HAWK 加 store_code。
var merIDRe = regexp.MustCompile(`^HAWK([0-9]{1,10})$`)

// FormatMerID 由 store_code 組成 PayUni 的 MerID。
func FormatMerID(storeCode int64) string { return "HAWK" + strconv.FormatInt(storeCode, 10) }

// ParseMerID 取出 MerID 裡的 store_code；格式不合時 ok 為 false。
func ParseMerID(merID string) (storeCode int64, ok bool) {
	m := merIDRe.FindStringSubmatch(merID)
	if m == nil {
		return 0, false
	}
	code, err := strconv.ParseInt(m[1], 10, 64)
	return code, err == nil
}

// executeTimeLayout 是 PayUni 的 ExecuteTime 格式；PayUni 的時間是台灣時間（台灣自 1979 年起沒有日光節約時間）。
const executeTimeLayout = "20060102150405"

// ParseExecuteTime 解析 PayUni 的 ExecuteTime（yyyyMMddHHmmss，台北時間）成 UTC；格式不合時 ok 為 false。
func ParseExecuteTime(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(executeTimeLayout, strings.TrimSpace(s), twtime.Taipei)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// PayUni 的模組事件。EventID 是穩定的 UUIDv7（等於寫進去的那筆觀測紀錄的 ID），relay 重送時不變；wire 的格式
// `{event_id, status, reason?, merchant_id（= AccountID）, store_id}` 與 routing key 與舊系統相同，由 `internal/legacy/sellerrelay` 轉送。

// PayUniForwarded 在綁定成功（PayUni 收下註冊）的 transaction 內發出，對應舊的 merchant.verification.forwarded
// （wire 的 status 是 forwarded_to_payuni）。
type PayUniForwarded struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
}

// PayUniApproved 在套用觀測、PayUni 的結果變成核准的 transaction 內發出，對應 merchant.verification.approved。
type PayUniApproved struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
	Reason             string
}

// PayUniPendingInfo 在 PayUni 要求補件時發出，對應 merchant.verification.pending_info；Reason 是 PayUni 的審核意見
// （沒有時空字串，預設文案由 relay 或 client 補）。
type PayUniPendingInfo struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
	Reason             string
}

// PayUniUnderReview 在 PayUni 從待補件回到審核中時發出，對應 merchant.verification.under_review。
type PayUniUnderReview struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
}

// PayUniRejected 在 PayUni 拒絕或關閉時發出，對應 merchant.verification.rejected（管理員退回的是 KYCRejected，
// 兩者共用同一個 routing key）；Reason 是 PayUni 的審核意見，StatusCode 是同一筆觀測的狀態在錯誤目錄裡的碼
// （PayUniStatusCode，MER_x 或 MEMBER_x），沒有審核意見時 relay 依它取通知文案。
type PayUniRejected struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
	Reason             string
	StatusCode         string
}

// PayUniBindFailed 在綁定失敗（被永久拒絕、資料不完整或重試用完）的 transaction 內發出，與 bind_status 的寫入同一個
// transaction。舊系統沒有這個事件（失敗只 warn，盤點 D7），沒有舊 routing key，也還沒有訂閱者。
type PayUniBindFailed struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
	ErrorCode          string
}
