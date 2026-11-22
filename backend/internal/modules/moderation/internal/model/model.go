// Package model 是 moderation 的對外型別與值域：檢舉對象、案件決定、處置種類與檢舉結果的推導。
package model

import (
	"slices"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const (
	// DailyReportLimit 是每位檢舉人每個台北日最多送出的檢舉數（只算新增的檢舉，重送已檢舉過的對象不算）。
	DailyReportLimit = 20
	// SnapshotRetention 是結案後保留內容快照的期間：184 天，與 seller 措施申覆期限（model.StoreRetention）一致，不短於它（DSA 第 20 條的內部申訴期至少六個月）。
	SnapshotRetention = 184 * 24 * time.Hour
	// MaxDescriptionRunes 是檢舉說明的上限。
	MaxDescriptionRunes = 500
	// MaxSourceReports 是一個內容下架的認定最多引用的來源檢舉數（seller.MaxSourceModerationReports）。
	MaxSourceReports = 20
	// MaxTextRunes 是處置給作者的理由文字的上限（social 的隱藏與警告理由、seller 的 StatementToSeller 共同的較小者）。
	MaxTextRunes = 500
	// ChatContextWindow 是聊天對象的證據快照裡，被檢舉訊息前後各保留的訊息數（A6-D3）。
	ChatContextWindow = 5
)

// TargetKind 是檢舉對象的種類。
type TargetKind string

const (
	TargetPost       TargetKind = "POST"
	TargetComment    TargetKind = "COMMENT"
	TargetForumEntry TargetKind = "FORUM_ENTRY"
	TargetBuyer      TargetKind = "BUYER"
	// TargetReview 是買家寫的評價（review 模組）；TargetChatMessage 是買家與店家之間的一則聊天訊息（im 模組）。
	TargetReview      TargetKind = "REVIEW"
	TargetChatMessage TargetKind = "CHAT_MESSAGE"
	// TargetReviewReply 是店家對評價的回覆（review 模組）；ContentID 是評價 ID（回覆沒有自己的 ID，一則評價至多一則回覆）。
	TargetReviewReply TargetKind = "REVIEW_REPLY"
)

// TargetKinds 回傳全部檢舉對象的種類，與 cases_target_kind_check 一致。
func TargetKinds() []TargetKind {
	return []TargetKind{TargetPost, TargetComment, TargetForumEntry, TargetBuyer, TargetReview, TargetChatMessage, TargetReviewReply}
}

// Valid 說明這是不是檢舉對象的種類。
func (k TargetKind) Valid() bool { return slices.Contains(TargetKinds(), k) }

// Target 是一個檢舉對象：內容（貼文、留言、討論區留言、評價、聊天訊息）用 ContentID，買家用 BuyerID，恰好填一欄。
type Target struct {
	Kind      TargetKind
	ContentID uuid.UUID
	BuyerID   int64
}

// ContentIDPtr 是內容對象的 ID（買家對象為 nil），給 cases.content_id。
func (t Target) ContentIDPtr() *uuid.UUID {
	if t.Kind == TargetBuyer {
		return nil
	}
	return &t.ContentID
}

// BuyerIDPtr 是買家對象的 ID（內容對象為 nil），給 cases.buyer_id。
func (t Target) BuyerIDPtr() *int64 {
	if t.Kind != TargetBuyer {
		return nil
	}
	return &t.BuyerID
}

// Validate 檢查對象的形狀。
func (t Target) Validate() bool {
	if !t.Kind.Valid() {
		return false
	}
	if t.Kind == TargetBuyer {
		return t.BuyerID > 0 && t.ContentID == uuid.UUID{}
	}
	return t.ContentID != uuid.UUID{} && t.BuyerID == 0
}

// Decision 是案件的決定。
type Decision string

const (
	// DecisionViolation 違規：帶認定的政策分類與處置種類。
	DecisionViolation Decision = "VIOLATION"
	// DecisionNoViolation 不違規：對內容而言就是不處置。
	DecisionNoViolation Decision = "NO_VIOLATION"
)

// Action 是違規的處置種類。
type Action string

const (
	// ActionHide 隱藏買家寫的留言、討論區留言（紀錄在 social）、評價（review）或聊天訊息（im）。
	ActionHide Action = "HIDE"
	// ActionWarn 警告買家（紀錄在 social）。
	ActionWarn Action = "WARN"
	// ActionBan 停權買家的帳號（紀錄在 identity，經 accountsuspension）。
	ActionBan Action = "BAN"
	// ActionTakedown 下架店家寫的內容（seller 的內容下架措施）。
	ActionTakedown Action = "TAKEDOWN"
)

// Actions 回傳全部處置種類，與 cases_decision_action_check 一致。
func Actions() []Action { return []Action{ActionHide, ActionWarn, ActionBan, ActionTakedown} }

// AllowedOn 說明這個處置種類可不可以用在這種對象上，與 cases_action_target_check 一致；作者是買家或店家另由處理時向 social 核對。
func (a Action) AllowedOn(k TargetKind) bool {
	switch a {
	case ActionHide:
		return k == TargetComment || k == TargetForumEntry || k == TargetReview || k == TargetChatMessage
	case ActionWarn, ActionBan:
		return k == TargetComment || k == TargetForumEntry || k == TargetBuyer || k == TargetReview || k == TargetChatMessage
	case ActionTakedown:
		return k == TargetPost || k == TargetComment || k == TargetForumEntry || k == TargetChatMessage || k == TargetReviewReply
	}
	return false
}

// CategoryAllowed 說明這個處置種類接受的認定類別：店家分支（下架）用 seller 的 Assessable，買家分支用社群內容可用的類別。
func (a Action) CategoryAllowed(c policy.Category) bool {
	if a == ActionTakedown {
		return c.Assessable()
	}
	return c.CommunityAssessable()
}

// Outcome 是一筆檢舉的結果，由所屬案件的決定推導。
type Outcome string

const (
	// OutcomeNone 案件進行中，還沒有結果。
	OutcomeNone Outcome = ""
	// OutcomeViolation 案件認定違規。
	OutcomeViolation Outcome = "VIOLATION"
	// OutcomeNoViolation 案件認定不違規。
	OutcomeNoViolation Outcome = "NO_VIOLATION"
)

// OutcomeOf 由案件的決定推導檢舉的結果：decision 為 nil 是進行中（OutcomeNone）。
func OutcomeOf(decision *string) Outcome {
	switch {
	case decision == nil:
		return OutcomeNone
	case Decision(*decision) == DecisionViolation:
		return OutcomeViolation
	default:
		return OutcomeNoViolation
	}
}

// Case 是一個案件。
type Case struct {
	ID             uuid.UUID
	Target         Target
	Version        int64
	OpenedAt       time.Time
	Decision       *Decision
	Category       *policy.Category
	Action         *Action
	DecidedByAdmin *int64
	ClosedAt       *time.Time
	// ReportCount 是案件的全部檢舉數；MeasureID 是 TAKEDOWN 決定建立的 seller 措施；StoreID 是快照凍結的作者店家
	// （買家對象、沒有快照或內容不是店家寫的是 nil）。
	ReportCount int64
	MeasureID   *uuid.UUID
	StoreID     *int64
	// Reversal 是 HIDE 決定的撤銷（ReverseHide）；沒有撤銷是 nil。決定本身不改。
	Reversal *Reversal
}

// Reversal 是案件上 HIDE 決定的撤銷紀錄。
type Reversal struct {
	At      time.Time
	AdminID int64
	Reason  string
}

// Report 是一筆檢舉；Outcome 由所屬案件推導。
type Report struct {
	ID     uuid.UUID
	CaseID uuid.UUID
	// ReporterBuyerID 與 ReporterStoreID 恰好一個是正數（買家或店家檢舉人，A6-D21 W3）。
	ReporterBuyerID int64
	ReporterStoreID int64
	Category        policy.Category
	Description     string
	SnapshotID      *uuid.UUID
	CreatedAt       time.Time
	Outcome         Outcome
}

// SubmitInput 是送出檢舉的輸入。
type SubmitInput struct {
	// ReporterBuyerID 與 ReporterStoreID 恰好一個是正數；店家檢舉人只能檢舉 CHAT_MESSAGE（而且要是那段對話的一方）。
	ReporterBuyerID int64
	ReporterStoreID int64
	Target          Target
	Category        policy.Category
	Description     string
	// RequestID 是選填的冪等鍵（AIP-155）：同一位檢舉人以同一個 RequestID 重送，內容相同一律回傳第一次的檢舉（不看案件現在的狀態、不吃每日額度），
	// 內容不同是 InvalidInput。
	RequestID *uuid.UUID
}

// SubmitResult 是送出檢舉的結果；Created 為假表示這位檢舉人對這個對象在進行中案件裡已經有檢舉，回傳的是既有的那一筆（不吃每日額度）。
type SubmitResult struct {
	Report  Report
	Case    Case
	Created bool
}

// DecideInput 是處理案件的輸入。Version 是管理員讀到的案件版本。
type DecideInput struct {
	CaseID  uuid.UUID
	Version int64
	// Actor 是下決定的管理員與後台 RPC 的 procedure 名稱（handler 組成，platform/adminaudit）；缺漏是接線錯誤，不是使用者的輸入。
	Actor    adminaudit.Actor
	Decision Decision
	// 以下只有 DecisionViolation 用：認定的類別、處置種類、給作者的理由；店家分支（ActionTakedown）另外要 Severity（認定的嚴重度，
	// 字串值 seller.Severity）。內容所屬的店家取自案件的內容快照，不由呼叫端帶。
	Category policy.Category
	Action   Action
	Text     string
	Severity string
}

// ReverseInput 是撤銷 HIDE 決定的輸入：案件維持結案、決定不改，擁有內容的模組還原目前未還原的隱藏。
type ReverseInput struct {
	CaseID uuid.UUID
	// Actor 同 DecideInput.Actor。
	Actor  adminaudit.Actor
	Reason string
}

// CaseDetail 是案件加上全部檢舉（各自帶推導出的結果）。
type CaseDetail struct {
	Case    Case
	Reports []Report
}

const (
	// DefaultPageSize 與 MaxPageSize 是檢舉人清單與後台案件清單的每頁筆數。
	DefaultPageSize int32 = 20
	MaxPageSize     int32 = 50
)

// ReportItem 是檢舉人讀自己檢舉時的一筆：檢舉、被檢舉的對象，以及案件結案的時間（進行中是 nil）。結果（Report.Outcome）由案件的決定推導，不存逐筆結果；
// 不含案件 ID（A7-D28 第 4 點）。
type ReportItem struct {
	Report   Report
	Target   Target
	ClosedAt *time.Time
}

// cursorTimeMax 與 cursorIDMax 是 keyset 第一頁的游標參數：由大到小排序的極值，讓每一頁走同一個 query（同 social 的 keyset 慣例）。
var (
	cursorTimeMax = time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC)
	cursorIDMax   = uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
)

// TrimPage 把多讀一筆（size+1）的 keyset 結果截成一頁，more 為真表示還有下一頁。
func TrimPage[T any](rows []T, size int32) (page []T, more bool) {
	if len(rows) > int(size) {
		return rows[:size], true
	}
	return rows, false
}

// ReportCursor 是檢舉清單的排序鍵：(CreatedAt, ID) 由大到小。
type ReportCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Valid 說明 cursor 的內容是否合法（解自 client 回傳的 page token，不可信）。
func (c ReportCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

// Key 回 SQL 的游標參數；nil（第一頁）回由大到小排序的極值，讓每一頁走同一個 query（同 social 的 keyset 慣例）。
func (c *ReportCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return cursorTimeMax, cursorIDMax
	}
	return c.CreatedAt, c.ID
}

// ReportPage 是一頁檢舉；Next 是下一頁的游標（沒有下一頁是 nil）。
type ReportPage struct {
	Items []ReportItem
	Next  *ReportCursor
}

// CaseState 是案件的狀態篩選：進行中（沒有結案）或已結案。
type CaseState string

const (
	CaseOpen   CaseState = "OPEN"
	CaseClosed CaseState = "CLOSED"
)

// CaseStates 回傳全部案件狀態。
func CaseStates() []CaseState { return []CaseState{CaseOpen, CaseClosed} }

// CaseFilter 是列案件的篩選；欄位空值是不篩。
type CaseFilter struct {
	State      CaseState
	TargetKind TargetKind
}

// CaseCursor 是案件清單的排序鍵：(OpenedAt, ID) 由大到小。
type CaseCursor struct {
	OpenedAt time.Time
	ID       uuid.UUID
}

// Valid 說明 cursor 的內容是否合法（解自 client 回傳的 page token，不可信）。
func (c CaseCursor) Valid() bool { return !c.OpenedAt.IsZero() && c.ID != uuid.UUID{} }

// Key 回 SQL 的游標參數；nil（第一頁）回由大到小排序的極值，讓每一頁走同一個 query。
func (c *CaseCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return cursorTimeMax, cursorIDMax
	}
	return c.OpenedAt, c.ID
}

// CasePage 是一頁案件（不含檢舉）；Next 是下一頁的游標（沒有下一頁是 nil）。
type CasePage struct {
	Cases []Case
	Next  *CaseCursor
}
