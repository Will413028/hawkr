package model

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 案件的詢問（step10e-reports-brief R4；設計 step10e-3cb-design §3.1）：平台對案件的賣家的一次要求，有期限、一則回覆、可撤回；每個案件同時最多一筆
// 還沒結束的詢問（DB 的 case_inquiries_one_open）。回覆在 3c-b 只由管理員記下（系統外收到）。處理狀態（InquiryState）與可以做的動作（Availability）由事實推導，不存。

// InquiryKind 是平台要賣家做的事；值與 DB 的 case_inquiries_kind_check、契約的 CaseInquiryKind 一致。
type InquiryKind string

const (
	// InquiryKindRecords 提供紀錄：賣家合作條款第 11.3 條的來源、批次、效期、進出貨及流向。
	InquiryKindRecords InquiryKind = "RECORDS"
	// InquiryKindExplanation 請說明。
	InquiryKindExplanation InquiryKind = "EXPLANATION"
)

// InquiryKindValues 回傳全部種類，順序固定。
func InquiryKindValues() []InquiryKind {
	return []InquiryKind{InquiryKindRecords, InquiryKindExplanation}
}

// InquiryReplyChannel 是回覆怎麼收到的；值與 DB 的 case_inquiries_reply_check、契約的 CaseInquiryReplyChannel 一致。
type InquiryReplyChannel string

const (
	InquiryChannelPhone     InquiryReplyChannel = "PHONE"
	InquiryChannelEmail     InquiryReplyChannel = "EMAIL"
	InquiryChannelMessaging InquiryReplyChannel = "MESSAGING"
	InquiryChannelOther     InquiryReplyChannel = "OTHER"
)

// InquiryReplyChannelValues 回傳全部管道，順序固定。
func InquiryReplyChannelValues() []InquiryReplyChannel {
	return []InquiryReplyChannel{InquiryChannelPhone, InquiryChannelEmail, InquiryChannelMessaging, InquiryChannelOther}
}

// InquiryWithdrawCause 是撤回的原因；值與 DB 的 case_inquiries_withdraw_check、契約的 CaseInquiryWithdrawCause 一致。
type InquiryWithdrawCause string

const (
	// InquiryWithdrawAdmin 管理員撤回。
	InquiryWithdrawAdmin InquiryWithdrawCause = "ADMIN"
	// InquiryWithdrawCaseClosed 案件結案（決定，或通報都移走）時系統撤回。
	InquiryWithdrawCaseClosed InquiryWithdrawCause = "CASE_CLOSED"
)

// InquiryWithdrawCauseValues 回傳全部撤回原因，順序固定。
func InquiryWithdrawCauseValues() []InquiryWithdrawCause {
	return []InquiryWithdrawCause{InquiryWithdrawAdmin, InquiryWithdrawCaseClosed}
}

// InquiryState 是詢問此刻的狀態，讀取時推導（不儲存）。逾期不是狀態：OPEN 而且 Overdue。
type InquiryState string

const (
	// InquiryStateOpen 等賣家回覆。
	InquiryStateOpen InquiryState = "OPEN"
	// InquiryStateReplied 已記下回覆（終態）。
	InquiryStateReplied InquiryState = "REPLIED"
	// InquiryStateWithdrawn 已撤回（終態）。
	InquiryStateWithdrawn InquiryState = "WITHDRAWN"
)

// InquiryStateValues 回傳全部狀態，順序固定。
func InquiryStateValues() []InquiryState {
	return []InquiryState{InquiryStateOpen, InquiryStateReplied, InquiryStateWithdrawn}
}

// 詢問的期限與長度（rb R4；範圍是起始值，設計 Q6）。期限是日曆時間（工作日行事曆還沒有，設計 §0.5）。
const (
	// InquiryDefaultDue 是沒有指定期限時，建立後的期限（rb R4 的預設 3 天）。
	InquiryDefaultDue = 72 * time.Hour
	// InquiryMinDue 與 InquiryMaxDue 是建立時可以指定的期限範圍（距離建立的時間）。
	InquiryMinDue = 24 * time.Hour
	InquiryMaxDue = 14 * 24 * time.Hour
	// InquiryMessageMaxRunes 與 InquiryReplyMaxRunes 是給賣家的說明與回覆內容的字數上限（DB 的 CHECK 同值）。
	InquiryMessageMaxRunes = 2000
	InquiryReplyMaxRunes   = 4000
)

// CaseInquiry 是案件上對賣家的一筆詢問。StoreID 是案件的店家（DB 的複合外鍵保證相等）。
type CaseInquiry struct {
	ID, CaseID       uuid.UUID
	StoreID          int64
	Kind             InquiryKind
	Message          string
	CreatedAt        time.Time
	CreatedByAdminID int64
	DueAt            time.Time
	Reply            *InquiryReply
	Withdrawn        *InquiryWithdrawal
}

// InquiryReply 是管理員記下的賣家回覆（系統外收到）。ReceivedAt 是收到的時間，RecordedAt 是記進系統的時間（不早於收到）；案件的「待決定」從收到的時間重新起算。
type InquiryReply struct {
	Body              string
	Channel           InquiryReplyChannel
	ReceivedAt        time.Time
	RecordedAt        time.Time
	RecordedByAdminID int64
	// Evidence 是附件（機敏檔）；Position 是連號，從 0 起。只在讀取詢問時載入。
	Evidence []Evidence
}

// InquiryWithdrawal 是撤回；系統撤回（案件結案）時 AdminID 是結案的管理員。
type InquiryWithdrawal struct {
	At      time.Time
	AdminID int64
	Cause   InquiryWithdrawCause
}

// Open 回報詢問是否還沒結束（沒有回覆也沒有撤回）；與 SQL 的 reply_received_at IS NULL AND withdrawn_at IS NULL 同一個定義。
func (i CaseInquiry) Open() bool { return i.Reply == nil && i.Withdrawn == nil }

// State 是詢問此刻的狀態。
func (i CaseInquiry) State() InquiryState {
	switch {
	case i.Reply != nil:
		return InquiryStateReplied
	case i.Withdrawn != nil:
		return InquiryStateWithdrawn
	}
	return InquiryStateOpen
}

// InquiryBlock 是詢問此刻不能做某個動作的一個原因；原因的句子是 Text。
type InquiryBlock string

const (
	// InquiryBlockWithdrawn 已撤回。
	InquiryBlockWithdrawn InquiryBlock = "WITHDRAWN"
	// InquiryBlockReplied 已記下回覆。
	InquiryBlockReplied InquiryBlock = "REPLIED"
)

// InquiryBlockValues 回傳全部詢問不能做動作的原因。
func InquiryBlockValues() []InquiryBlock {
	return []InquiryBlock{InquiryBlockWithdrawn, InquiryBlockReplied}
}

// Text 是原因給後台看的一句話（只有 zh-TW）；指令被擋下時錯誤的訊息取第一個原因的句子。不認得的原因回零值（Text 是空字串）。
func (b InquiryBlock) Text() uitext.Message {
	switch b {
	case InquiryBlockWithdrawn:
		return text.InquiryUnavailableWithdrawn()
	case InquiryBlockReplied:
		return text.InquiryUnavailableReplied()
	}
	return uitext.Message{}
}

// InquiryAvailability 是詢問每個動作不能做的原因，空的就是可以做。
type InquiryAvailability struct {
	RecordReply, Withdraw []InquiryBlock
}

// Availability 是詢問每個動作不能做的原因（依序列出，錯誤取第一個）。這是唯一的寫法：API 的 can_* 與指令的前置檢查都只看它。
// 記下回覆不看案件是否進行中：案件結案時詢問已被撤回，原因就是 WITHDRAWN。已回覆或已撤回的詢問再記回覆或再撤回，指令回原本的紀錄（不是錯誤），
// 由指令從這裡的第一個原因判斷。
func (i CaseInquiry) Availability() InquiryAvailability {
	var a InquiryAvailability
	switch i.State() {
	case InquiryStateOpen:
	case InquiryStateWithdrawn:
		a.RecordReply = []InquiryBlock{InquiryBlockWithdrawn}
		a.Withdraw = []InquiryBlock{InquiryBlockWithdrawn}
	case InquiryStateReplied:
		a.RecordReply = []InquiryBlock{InquiryBlockReplied}
		a.Withdraw = []InquiryBlock{InquiryBlockReplied}
	}
	return a
}

// AdminInquiry 是後台看到的一筆詢問：詢問本身加上此刻可以做的動作。
type AdminInquiry struct {
	CaseInquiry
	Availability InquiryAvailability
}

// CreateCaseInquiryInput 是發詢問的輸入。DueAt 為 nil 是建立後 InquiryDefaultDue；指定時必須在建立後 InquiryMinDue 到 InquiryMaxDue 之間。RequestID 選填（AIP-155）。
type CreateCaseInquiryInput struct {
	CaseID    uuid.UUID
	AdminID   int64
	Kind      InquiryKind
	Message   string
	DueAt     *time.Time
	RequestID *uuid.UUID
}

// CreateCaseInquiryResult 是發詢問的結果；Replayed 表示這是同一個 request_id 的重送，回的是原本那一筆。
type CreateCaseInquiryResult struct {
	Inquiry  AdminInquiry
	Case     AdminCase
	Replayed bool
}

// RecordCaseInquiryReplyInput 是記下回覆的輸入：ReceivedAt 是收到的時間（不早於詢問建立、不晚於現在），EvidenceTokens 是附件（機敏上傳的 staged token，最多 MaxEvidence 份）。
type RecordCaseInquiryReplyInput struct {
	InquiryID      uuid.UUID
	AdminID        int64
	Channel        InquiryReplyChannel
	ReceivedAt     time.Time
	Body           string
	EvidenceTokens []string
}

// RecordCaseInquiryReplyResult 是記下回覆的結果；AlreadyRecorded 表示之前已經記過，回傳的詢問帶的是原本那一則。
type RecordCaseInquiryReplyResult struct {
	Inquiry         AdminInquiry
	Case            AdminCase
	AlreadyRecorded bool
}

// WithdrawCaseInquiryInput 是撤回詢問的輸入。
type WithdrawCaseInquiryInput struct {
	InquiryID uuid.UUID
	AdminID   int64
}

// WithdrawCaseInquiryResult 是撤回詢問的結果；AlreadyWithdrawn 表示之前已經撤回。
type WithdrawCaseInquiryResult struct {
	Inquiry          AdminInquiry
	Case             AdminCase
	AlreadyWithdrawn bool
}

// InquiryCursor 是詢問列表的 keyset 位置（建立時間舊到新）。
type InquiryCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c InquiryCursor) Valid() bool { return c.ID != uuid.Nil() && !c.CreatedAt.IsZero() }

// CaseInquiryListInput 是案件詢問清單的輸入。
type CaseInquiryListInput struct {
	CaseID   uuid.UUID
	PageSize int32
	After    *InquiryCursor
}

// AdminInquiryPage 是一頁詢問；Next 為 nil 表示沒有下一頁。
type AdminInquiryPage struct {
	Items []AdminInquiry
	Next  *InquiryCursor
}
