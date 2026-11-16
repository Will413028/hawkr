// Package model 是 moderation 的對外型別與值域：檢舉對象、案件決定、處置種類與檢舉結果的推導。
package model

import (
	"slices"
	"time"

	"uuid"

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
)

// TargetKind 是檢舉對象的種類；聊天與評價對象在 A6。
type TargetKind string

const (
	TargetPost       TargetKind = "POST"
	TargetComment    TargetKind = "COMMENT"
	TargetForumEntry TargetKind = "FORUM_ENTRY"
	TargetBuyer      TargetKind = "BUYER"
)

// TargetKinds 回傳全部檢舉對象的種類，與 cases_target_kind_check 一致。
func TargetKinds() []TargetKind {
	return []TargetKind{TargetPost, TargetComment, TargetForumEntry, TargetBuyer}
}

// Valid 說明這是不是檢舉對象的種類。
func (k TargetKind) Valid() bool { return slices.Contains(TargetKinds(), k) }

// Target 是一個檢舉對象：內容（貼文、留言、討論區留言）用 ContentID，買家用 BuyerID，恰好填一欄。
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
	// ActionHide 隱藏買家寫的留言或討論區留言（紀錄在 social）。
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
		return k == TargetComment || k == TargetForumEntry
	case ActionWarn, ActionBan:
		return k == TargetComment || k == TargetForumEntry || k == TargetBuyer
	case ActionTakedown:
		return k == TargetPost || k == TargetComment || k == TargetForumEntry
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
	// OutcomePending 案件進行中。
	OutcomePending Outcome = "PENDING"
	// OutcomeViolation 案件認定違規。
	OutcomeViolation Outcome = "VIOLATION"
	// OutcomeNoViolation 案件認定不違規。
	OutcomeNoViolation Outcome = "NO_VIOLATION"
)

// OutcomeOf 由案件的決定推導檢舉的結果：decision 為 nil 是進行中。
func OutcomeOf(decision *string) Outcome {
	switch {
	case decision == nil:
		return OutcomePending
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
}

// Report 是一筆檢舉；Outcome 由所屬案件推導。
type Report struct {
	ID              uuid.UUID
	CaseID          uuid.UUID
	ReporterBuyerID int64
	Category        policy.Category
	Description     string
	SnapshotID      *uuid.UUID
	CreatedAt       time.Time
	Outcome         Outcome
}

// SubmitInput 是送出檢舉的輸入。
type SubmitInput struct {
	ReporterBuyerID int64
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
	CaseID   uuid.UUID
	Version  int64
	AdminID  int64
	Decision Decision
	// 以下只有 DecisionViolation 用：認定的類別、處置種類、給作者的理由；店家分支（ActionTakedown）另外要 StoreID（管理員確認的內容所屬店家，
	// 向 social 核對）與 Severity（認定的嚴重度，字串值 seller.Severity）。
	Category policy.Category
	Action   Action
	Text     string
	StoreID  int64
	Severity string
}

// CaseDetail 是案件加上全部檢舉（各自帶推導出的結果）。
type CaseDetail struct {
	Case    Case
	Reports []Report
}

const (
	// DefaultPageSize 與 MaxPageSize 是檢舉人讀自己檢舉清單的每頁筆數。
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
		return time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC), uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	}
	return c.CreatedAt, c.ID
}

// ReportPage 是一頁檢舉；Next 是下一頁的游標（沒有下一頁是 nil）。
type ReportPage struct {
	Items []ReportItem
	Next  *ReportCursor
}
