package model

import (
	"time"
	"uuid"
)

// FoodRegState 是食品業者登錄的狀態，與 DB 的 food_registrations_state_check 一致。
type FoodRegState string

const (
	// FoodRegPending 賣家送出、等平台核對。
	FoodRegPending FoodRegState = "PENDING"
	// FoodRegApproved 目前有效的登錄：店家公開的字號只讀這一筆。
	FoodRegApproved FoodRegState = "APPROVED"
	// FoodRegRejected 被退回，有退回理由。
	FoodRegRejected FoodRegState = "REJECTED"
	// FoodRegWithdrawn 賣家撤回，或被新送出的取代、隨清除撤回；沒有裁定。
	FoodRegWithdrawn FoodRegState = "WITHDRAWN"
	// FoodRegEnded 曾經核准而後結束，原因是 FoodRegEndCause。
	FoodRegEnded FoodRegState = "ENDED"
)

// FoodRegStateValues 回傳全部儲存的狀態，與 DB 的 food_registrations_state_check 一致。
func FoodRegStateValues() []FoodRegState {
	return []FoodRegState{FoodRegPending, FoodRegApproved, FoodRegRejected, FoodRegWithdrawn, FoodRegEnded}
}

// FoodRegInbox 是審核收件匣的隊列：待審、店家營運中的件依店家的 KYC 是否已核准分開。
type FoodRegInbox string

const (
	// FoodRegInboxDecidable 現在可以裁定：店家的 KYC 申請是 APPROVED。收件匣的預設。
	FoodRegInboxDecidable FoodRegInbox = "DECIDABLE"
	// FoodRegInboxAwaitingKYC 還在等 KYC 核准：店家沒有 APPROVED 的 KYC 申請（沒申請、草稿、審核中、已退回）。
	FoodRegInboxAwaitingKYC FoodRegInbox = "AWAITING_KYC"
)

// FoodRegInboxValues 回傳全部隊列。
func FoodRegInboxValues() []FoodRegInbox {
	return []FoodRegInbox{FoodRegInboxDecidable, FoodRegInboxAwaitingKYC}
}

// FoodRegEndCause 是核准的登錄結束的原因，與 DB 的 food_registrations_end_cause_check 一致。
type FoodRegEndCause string

const (
	// FoodRegSuperseded 被新核准的字號取代。
	FoodRegSuperseded FoodRegEndCause = "SUPERSEDED"
	// FoodRegCleared 賣家清除。
	FoodRegCleared FoodRegEndCause = "CLEARED"
	// FoodRegRevoked 平台撤銷：資源層級的更正，帶給賣家看的理由。
	FoodRegRevoked FoodRegEndCause = "REVOKED"
	// FoodRegStoreRetired 店家退場（第一段清理）時，仍核准中的登錄隨之結束；字號留到個資清理（第二段，關店滿保留期 StoreRetention）才清。
	FoodRegStoreRetired FoodRegEndCause = "STORE_RETIRED"
)

// FoodRegEndCauseValues 回傳全部結束原因。
func FoodRegEndCauseValues() []FoodRegEndCause {
	return []FoodRegEndCause{FoodRegSuperseded, FoodRegCleared, FoodRegRevoked, FoodRegStoreRetired}
}

// FoodRegistration 是店家的一筆食品業者登錄（資源只有一個，狀態是 State）；賣家、後台與 legacy 讀到的都是它：Store.ApprovedFoodRegistration
// 是核准中的那一筆（公開、後台與 legacy 的字號唯一來源）、Store.OpenFoodRegistration 是待審或被退回的那一筆、Store.RevokedFoodRegistration
// 是被平台撤銷的那一筆（State 是 ENDED，EndedAt 與 EndReason 有值；其他的兩者是零值）。
type FoodRegistration struct {
	ID                 uuid.UUID
	RegistrationNumber string
	State              FoodRegState
	SubmittedAt        time.Time
	// DecidedAt 是平台核准或退回的時間；待審時是 nil。
	DecidedAt    *time.Time
	RejectReason string
	EndedAt      *time.Time
	EndReason    string
}

// FoodRegistrationSubject 是後台核對登錄時要比對的 KYC 主體：企業是公司名稱與統編，個人是真實姓名。只來自已核准的 KYC 申請。
type FoodRegistrationSubject struct {
	SellerType  KYCSellerType
	RealName    string
	CompanyName string
	TaxID       string
}

// FoodRegistrationReview 是後台看到的一筆登錄：登錄本身、店家、店家目前核准的登錄，以及店家 KYC 的狀態與主體。KYCStatus 是 nil
// 表示店家還沒有（未取消的）申請；Subject 只在 KYC 已核准時有值，其他狀態的申請（含賣家的草稿）後台看不到。
type FoodRegistrationReview struct {
	ID                 uuid.UUID
	StoreID            int64
	StoreName          string
	StoreStatus        Status
	RegistrationNumber string
	State              FoodRegState
	SubmittedAt        time.Time
	DecidedByAdminID   *int64
	DecidedAt          *time.Time
	RejectReason       string
	InternalNote       string
	EndedAt            *time.Time
	EndCause           FoodRegEndCause
	EndReason          string
	EndedByAdminID     *int64
	KYCStatus          *KYCStatus
	Subject            *FoodRegistrationSubject
	// Approved 是店家目前核准的那一筆（可能就是這一筆）；沒有時是 nil。
	Approved *FoodRegistration
	// DecideDeadline 是裁定的期限（seller.review_food_registration）：現在可以裁定的才有；等商家認證、已裁定或店家關閉時 nil。
	DecideDeadline *Deadline
}

// SubmitFoodRegistrationInput 是賣家送出食品業者登錄；RegistrationNumber 去掉前後空白後 1 到 50 字。AcceptedAgreements 剛好是
// SubmitFoodRegistrationAgreements 的種類、版本是現行版本；ClientIP 與 UserAgent 是確認當下的來源，取不到是空字串。
type SubmitFoodRegistrationInput struct {
	StoreID             int64
	RegistrationNumber  string
	AcceptedAgreements  []AgreementAcceptance
	ClientIP, UserAgent string
}

// FoodRegistrationCursor 是收件匣與歷史清單的 keyset 位置：最後一筆的 (排序時間, id)。排序時間在「現在可以裁定」的隊列是期限
// （seller.review_food_registration 的 due_at），在「等 KYC」的隊列與歷史是送出時間；token 綁住 state 與 inbox（FilterScope），所以兩種不會混用。
type FoodRegistrationCursor struct {
	SortAt time.Time
	ID     uuid.UUID
}

// Valid 回報位置是不是完整的。
func (c FoodRegistrationCursor) Valid() bool { return c.ID != uuid.Nil() && !c.SortAt.IsZero() }

// FoodRegistrationListInput 是後台的清單：State 為 nil 或 PENDING 是收件匣（營運中的店家的待審件，舊到新），再由 Inbox 分隊列
// （nil 是 FoodRegInboxDecidable）；其他狀態是歷史（新到舊，店家的狀態不限），不可以指定 Inbox。Query 是字號或店名的子字串；
// PageSize 為 0 用預設值。
type FoodRegistrationListInput struct {
	State    *FoodRegState
	Inbox    *FoodRegInbox
	Query    string
	PageSize int32
	After    *FoodRegistrationCursor
}

// FoodRegistrationPage 是一頁清單；Next 為 nil 表示沒有下一頁。
type FoodRegistrationPage struct {
	Items []FoodRegistrationReview
	Next  *FoodRegistrationCursor
}

// DecideFoodRegistrationInput 是管理員裁定待審的登錄：Outcome 只能是 FoodRegApproved 或 FoodRegRejected；退回時 Reason 必填，
// 核准時不帶；InternalNote 只給後台看（核准或退回時都可以寫，例如「僅核對有效」）。
type DecideFoodRegistrationInput struct {
	ID                   uuid.UUID
	AdminID              int64
	Outcome              FoodRegState
	Reason, InternalNote string
}

// RevokeFoodRegistrationInput 是管理員撤銷核准中的登錄：Reason 必填（給賣家看），沒有內部註記。
type RevokeFoodRegistrationInput struct {
	ID      uuid.UUID
	AdminID int64
	Reason  string
}

// FoodRegistrationRevoked 在管理員撤銷核准中的登錄的 transaction 內發出，帶被撤銷的字號與給賣家的理由。送達賣家是 A8 的
// notification 的工作，目前沒有訂閱者。
type FoodRegistrationRevoked struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	RegistrationID     uuid.UUID
	StoreID            int64
	AccountID          int64
	RegistrationNumber string
	Reason             string
}

// FoodRegistrationDecided 在管理員裁定（核准或退回）的 transaction 內發出；Outcome 是 FoodRegApproved 或 FoodRegRejected，
// RejectReason 只在退回時有值（賣家看得到的理由）。送達賣家是 A8 的 notification 的工作，目前沒有訂閱者。
type FoodRegistrationDecided struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	RegistrationID     uuid.UUID
	StoreID            int64
	AccountID          int64
	RegistrationNumber string
	Outcome            FoodRegState
	RejectReason       string
}

// CanReject 回報現在能不能退回：不能退回的原因（FoodRegistrationUnavailableReasons）是空的，也就是待審而且店家營運中（關店與等待刪除的店家不裁定）。
func (r FoodRegistrationReview) CanReject() bool {
	return len(FoodRegistrationUnavailableReasons(r, StoreActionRejectFoodRegistration)) == 0
}

// CanRevoke 回報現在能不能撤銷：核准中（已結束的，包括店家個資清理時結束的，不能）。
func (r FoodRegistrationReview) CanRevoke() bool { return r.State == FoodRegApproved }

// CanApprove 回報現在能不能核准：不能核准的原因是空的，也就是能退回，而且店家的 KYC 已核准（核對的是已驗證過的主體）。
func (r FoodRegistrationReview) CanApprove() bool {
	return len(FoodRegistrationUnavailableReasons(r, StoreActionApproveFoodRegistration)) == 0
}

// ApproveUnavailable 是此刻不能核准的原因（空表示可以）；CanApprove、DecideFoodRegistration 的前置檢查與 API 的 approve_unavailabilities 是同一個推導。
func (r FoodRegistrationReview) ApproveUnavailable() []StoreActionUnavailableReason {
	return FoodRegistrationUnavailableReasons(r, StoreActionApproveFoodRegistration)
}

// RejectUnavailable 是此刻不能退回的原因（空表示可以）。
func (r FoodRegistrationReview) RejectUnavailable() []StoreActionUnavailableReason {
	return FoodRegistrationUnavailableReasons(r, StoreActionRejectFoodRegistration)
}
