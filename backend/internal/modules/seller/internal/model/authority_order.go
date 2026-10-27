package model

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 主管機關來函（step10e-reports-brief R5；設計 step10e-3cb-design §1.4、§3.1）：主管機關給平台的公文，獨立的收件，不是通報。來函有期限（收文時間起算）、
// 指名的對象（任何帳號，選填店家與商品）、保存期限（指名的帳號或店家在 seller 的個資在這之前不被關店後的清理刪除；只能延長）與完成時每個對象的告知。
// 處理狀態（AuthorityOrderState）與可以做的動作（Availability）由事實推導，不存。

// AuthorityOrderKind 是來函要平台做的事；值與 DB 的 authority_orders_kind_check、契約的 AuthorityOrderKind 一致。
type AuthorityOrderKind string

const (
	// AuthorityRestrictProduct 限制或移除商品（食安法草案第 43-1 條、食藥署請下架）。商品限制措施在 B4；在那之前處理方式寫在完成的處理摘要。
	AuthorityRestrictProduct AuthorityOrderKind = "RESTRICT_PRODUCT"
	// AuthorityDataRequest 調取資料（詐欺犯罪危害防制條例第 37 條第 2 項：三日內提供，並保存六個月）。
	AuthorityDataRequest AuthorityOrderKind = "DATA_REQUEST"
	// AuthoritySuspendAccount 暫停帳號（同條例第 36 條第 3 項）。
	AuthoritySuspendAccount AuthorityOrderKind = "SUSPEND_ACCOUNT"
	// AuthorityOther 其他。
	AuthorityOther AuthorityOrderKind = "OTHER"
)

// AuthorityOrderKindValues 回傳全部種類，順序固定。
func AuthorityOrderKindValues() []AuthorityOrderKind {
	return []AuthorityOrderKind{AuthorityRestrictProduct, AuthorityDataRequest, AuthoritySuspendAccount, AuthorityOther}
}

// DefaultAuthorityDue 是來函沒有指定期限時，收文後的期限（rb R3：限制、暫停 48 小時，調取 72 小時；其他是起始值 72 小時，設計 Q6）。
func DefaultAuthorityDue(kind AuthorityOrderKind) time.Duration {
	switch kind {
	case AuthorityRestrictProduct, AuthoritySuspendAccount:
		return 48 * time.Hour
	case AuthorityDataRequest, AuthorityOther:
		return 72 * time.Hour
	}
	return 72 * time.Hour
}

// MeasureCanNotify 回報這個種類的來函，被指名者可以由導出的措施告知（措施的理由書就是告知）：暫停帳號與限制或移除商品。
// 調取資料與其他不會產生措施，所以不能。
func (k AuthorityOrderKind) MeasureCanNotify() bool {
	return k == AuthoritySuspendAccount || k == AuthorityRestrictProduct
}

// 來函的期限、保存與長度（rb R5；範圍是起始值，設計 Q6）。
const (
	// AuthorityHold 是保存期限的預設長度：收文後 184 天（詐欺犯罪危害防制條例的六個月與草案的 180 日取較長者；任何連續六個曆月最長 184 天）。
	// 與 AppealWindow、StoreRetention 同值但是另一個常數：理由不同，一個改了另一個不必跟著改。
	AuthorityHold = 4416 * time.Hour
	// AuthorityDueMin 與 AuthorityDueMax 是來函自訂期限的範圍（距離收文時間）。
	AuthorityDueMin = time.Hour
	AuthorityDueMax = 30 * 24 * time.Hour
	// AuthorityReceiveLookback 是收文時間最多回溯多久。
	AuthorityReceiveLookback = 365 * 24 * time.Hour
	// MaxAuthorityTargets 與 MaxAuthorityTargetProducts 是一份來函最多指名的對象數與每個對象最多指名的商品數。
	MaxAuthorityTargets        = 20
	MaxAuthorityTargetProducts = 50
	// MaxSourceAuthorityOrders 是一個認定最多引用的來函數。
	MaxSourceAuthorityOrders = 5
	// AuthorityAgencyMaxRunes、AuthorityDocumentNoMaxRunes、AuthoritySummaryMaxRunes、AuthorityNoteMaxRunes 是文字的字數上限（DB 的 CHECK 同值）。
	AuthorityAgencyMaxRunes     = 100
	AuthorityDocumentNoMaxRunes = 100
	AuthoritySummaryMaxRunes    = 4000
	AuthorityNoteMaxRunes       = 2000
)

// AuthorityOrderState 是來函此刻的狀態，由完成時間推導。
type AuthorityOrderState string

const (
	// AuthorityStateOpen 處理中。
	AuthorityStateOpen AuthorityOrderState = "OPEN"
	// AuthorityStateCompleted 已完成（終態；保存期限不因完成而解除）。
	AuthorityStateCompleted AuthorityOrderState = "COMPLETED"
	// AuthorityStateRevoked 已撤銷登記（登記錯誤，終態）：保存解除，不在處理中，不能再當作措施的來源；紀錄保留。
	AuthorityStateRevoked AuthorityOrderState = "REVOKED"
)

// AuthorityOrderStateValues 回傳全部狀態，順序固定。
func AuthorityOrderStateValues() []AuthorityOrderState {
	return []AuthorityOrderState{AuthorityStateOpen, AuthorityStateCompleted, AuthorityStateRevoked}
}

// AuthorityNoticeKind 是被指名者的告知方式；值與 DB 的 authority_order_targets_notice_check、契約的 AuthorityTargetNoticeKind 一致。
type AuthorityNoticeKind string

const (
	// AuthorityNoticeNotified 已告知（時間與方式）。
	AuthorityNoticeNotified AuthorityNoticeKind = "NOTIFIED"
	// AuthorityNoticeWithheld 機關要求暫不告知；必填向機關確認的日期，到期進 3d 的待辦「告知被指名者」。
	AuthorityNoticeWithheld AuthorityNoticeKind = "WITHHELD"
	// AuthorityNoticeMeasure 由措施告知：只限暫停帳號、限制或移除商品，而且這個對象已有這筆來函導出的措施。
	AuthorityNoticeMeasure AuthorityNoticeKind = "MEASURE_IS_NOTICE"
)

// AuthorityNoticeKindValues 回傳全部告知方式，順序固定。
func AuthorityNoticeKindValues() []AuthorityNoticeKind {
	return []AuthorityNoticeKind{AuthorityNoticeNotified, AuthorityNoticeWithheld, AuthorityNoticeMeasure}
}

// AuthorityNotice 是一個對象的告知。NotifiedAt 只在 Notified，RecheckOn（UTC 午夜的台北日期）只在 Withheld。
type AuthorityNotice struct {
	Kind              AuthorityNoticeKind
	NotifiedAt        *time.Time
	Note              string
	RecheckOn         *time.Time
	RecordedAt        time.Time
	RecordedByAdminID int64
}

// HoldExtensionCause 是保存期限被延長的原因；值與 DB 的 authority_order_hold_extensions_cause_check 一致。
type HoldExtensionCause string

const (
	// HoldExtensionAdmin 管理員延長（例如進入訴訟，延長到裁判確定後 3 個月）。
	HoldExtensionAdmin HoldExtensionCause = "ADMIN"
	// HoldExtensionDataProvided 調取資料的來函完成時，依提供資料的時間自動延到「提供之後 AuthorityHold」。
	HoldExtensionDataProvided HoldExtensionCause = "DATA_PROVIDED"
)

// HoldExtensionCauseValues 回傳全部延長原因，順序固定。
func HoldExtensionCauseValues() []HoldExtensionCause {
	return []HoldExtensionCause{HoldExtensionAdmin, HoldExtensionDataProvided}
}

// AuthorityHoldExtension 是保存期限的一次延長（只新增）。Reason 只有 Admin 才有。
type AuthorityHoldExtension struct {
	From, To  time.Time
	Cause     HoldExtensionCause
	Reason    string
	AdminID   int64
	CreatedAt time.Time
}

// StoreCoverage 是保存對指名範圍內某一家店的涵蓋狀態（推導，逐家列出）；值與契約的 AuthorityStoreCoverage 一致。
type StoreCoverage string

const (
	// CoverageHeld 保存中：保存期限還沒到，這家店的個資不會被清理。
	CoverageHeld StoreCoverage = "HELD"
	// CoveragePurgedBeforeRegistration 登記之前這家店的個資已依保留期限清除（登記太晚，要回覆機關「資料已依保留期限刪除」）。
	CoveragePurgedBeforeRegistration StoreCoverage = "PURGED_BEFORE_REGISTRATION"
	// CoverageHoldEnded 保存期限已過，個資還沒被清理。
	CoverageHoldEnded StoreCoverage = "HOLD_ENDED"
	// CoveragePurgedAfterHoldEnded 登記之後、保存期限已過時個資被依保留期限清除；之後即使延長保存，已清除的資料也回不來，所以與目前的 hold_active 無關。
	CoveragePurgedAfterHoldEnded StoreCoverage = "PURGED_AFTER_HOLD_ENDED"
	// CoverageHoldReleasedByRevocation 來函已撤銷登記，保存在撤銷時解除（保存期限可能還沒到，所以不能說「保存期限已過」）；與這家店的個資是否已清理無關，
	// 只有登記當下就已清理的店家仍是 PurgedBeforeRegistration（那是登記時的事實）。
	CoverageHoldReleasedByRevocation StoreCoverage = "HOLD_RELEASED_BY_REVOCATION"
)

// AuthorityTargetStore 是被指名範圍內的一家店；StoreName 在個資已清理後是 nil，PurgedAt 是個資清理的時間。Target 是建立措施的入口用的事實
// （店家是否關閉、是不是帳號最近的 stint），CanCreateMeasure 與 CanCreateTermination 由它與來函是否撤銷登記推導（AuthorityOrder.storeActions）。
type AuthorityTargetStore struct {
	StoreID              int64
	StoreName            *string
	PurgedAt             *time.Time
	Coverage             StoreCoverage
	Target               TerminationTarget
	CanCreateMeasure     bool
	CanCreateTermination bool
}

// AuthorityOrderTarget 是來函指名的對象。StoreID 為 nil 是帳號全部的店家（包含之後的新 stint）；指定時只算那一家。Stores 是範圍內的店家（逐家列出）；
// 帳號在 seller 沒有店家時 NoSellerData 為 true（買家與帳號資料的保存不在 seller，尚未支援）。
type AuthorityOrderTarget struct {
	Position     int
	AccountID    int64
	StoreID      *int64
	ProductIDs   []string
	Stores       []AuthorityTargetStore
	NoSellerData bool
	Notice       *AuthorityNotice
}

// Names 回報這個對象是否指名了（帳號 accountID 的）店家 storeID：不指定店家時帳號的任何店家都算，指定時只算那一家。
// 措施的來源來函、認定引用的來函都用這個定義（SQL 的 OrdersNamingMeasureTarget 與它一致）。
func (t AuthorityOrderTarget) Names(accountID, storeID int64) bool {
	return t.AccountID == accountID && (t.StoreID == nil || *t.StoreID == storeID)
}

// AuthorityCompletion 是來函的完成。
type AuthorityCompletion struct {
	At      time.Time
	AdminID int64
	Summary string
}

// AuthorityRevocation 是來函的撤銷登記（登記錯誤）。
type AuthorityRevocation struct {
	At      time.Time
	AdminID int64
	Reason  string
}

// AuthorityOrder 是主管機關的一份來函。HoldUntil 是保存期限（只增不減，等於最後一列延長的 To；沒有延長時是 ReceivedAt + AuthorityHold；撤銷登記不改它，
// 但保存不再有效）；Version 是完成、延長保存、記下告知與撤銷登記的樂觀鎖。Targets 與 HoldExtensions 只在讀取時載入。
type AuthorityOrder struct {
	ID                  uuid.UUID
	Agency, DocumentNo  string
	ReceivedAt          time.Time
	Kind                AuthorityOrderKind
	Summary             string
	DueAt               time.Time
	HoldUntil           time.Time
	RegisteredAt        time.Time
	RegisteredByAdminID int64
	Completed           *AuthorityCompletion
	Revoked             *AuthorityRevocation
	Version             int64
	Targets             []AuthorityOrderTarget
	HoldExtensions      []AuthorityHoldExtension
	// Deadline 是處理來函的期限（seller.review_authority_order：收文到 DueAt）；已完成或撤銷時 nil。由讀取來函的路徑填入。
	Deadline *Deadline
}

// Open 回報來函是否還在處理中（還沒完成也沒撤銷登記）；與 SQL 的 completed_at IS NULL AND revoked_at IS NULL 同一個定義（3d「主管機關來函」的成員判定式：
// 開始時間是 ReceivedAt，期限是 DueAt）。
func (o AuthorityOrder) Open() bool { return o.Completed == nil && o.Revoked == nil }

// State 是來函此刻的狀態。
func (o AuthorityOrder) State() AuthorityOrderState {
	switch {
	case o.Revoked != nil:
		return AuthorityStateRevoked
	case o.Completed != nil:
		return AuthorityStateCompleted
	}
	return AuthorityStateOpen
}

// HoldActive 回報保存此刻仍有效：沒有撤銷登記，而且保存期限還沒到（與 SQL 的 StoreHolds 同一個定義）。
func (o AuthorityOrder) HoldActive(now time.Time) bool {
	return o.Revoked == nil && now.Before(o.HoldUntil)
}

// DefaultHoldUntil 是登記時的保存期限：收文後 AuthorityHold。
func DefaultHoldUntil(receivedAt time.Time) time.Time { return receivedAt.Add(AuthorityHold) }

// HoldAfterProvision 是調取資料的來函完成時，依提供資料的時間算出的保存期限：目前的與「提供之後 AuthorityHold」較晚者（設計 Q5 (b)：收文與提供取較晚的，
// 條文的「六個月」兩種讀法都合規）；第二個回傳值表示要延長（新的期限晚於目前的）。
func HoldAfterProvision(current, provideTime time.Time) (time.Time, bool) {
	candidate := provideTime.Add(AuthorityHold)
	if candidate.After(current) {
		return candidate, true
	}
	return current, false
}

// StoreCoverageOf 是保存對某一家店的涵蓋狀態。purgedAt 是個資清理的時間（沒清理是 nil）；purgedAtRegistration 是登記時（鎖住店家列之後）讀到個資已清理的快照
// （authority_order_prepurged_stores），這是事實，不比較時間：清理與登記可能在不同實例，應用程式時鐘有差。判斷順序：
// 登記時已清理是 PurgedBeforeRegistration（保存來不及）；其餘已清理的是 PurgedAfterHoldEnded（清理要在保存期限已過才會發生，之後延長也不影響）；
// 沒清理而且保存期限還沒到是 Held；其餘是 HoldEnded。已清理的店家不論保存期限現在是否有效，都不會是 Held 或 HoldEnded。
// 已撤銷登記的來函（HoldActive 為假的另一個原因）在登記時已清理之後的判斷之前先回 HoldReleasedByRevocation，與 HoldActive 同一個定義：
// 保存解除之後 hold_until 與清理時間都不能說明「為什麼不再保存」，所以不套用「保存期限已過」的兩個狀態。
func (o AuthorityOrder) StoreCoverageOf(purgedAt *time.Time, purgedAtRegistration bool, now time.Time) StoreCoverage {
	switch {
	case purgedAtRegistration:
		return CoveragePurgedBeforeRegistration
	case o.Revoked != nil:
		return CoverageHoldReleasedByRevocation
	case purgedAt != nil:
		return CoveragePurgedAfterHoldEnded
	case o.HoldActive(now):
		return CoverageHeld
	}
	return CoverageHoldEnded
}

// AuthorityOrderBlock 是來函此刻不能做某個動作的一個原因；原因的句子是 Text。
type AuthorityOrderBlock string

const (
	// AuthorityBlockRevoked 來函已撤銷登記。
	AuthorityBlockRevoked AuthorityOrderBlock = "REVOKED"
	// AuthorityBlockCompleted 來函已完成。
	AuthorityBlockCompleted AuthorityOrderBlock = "COMPLETED"
	// AuthorityBlockNoticeNotWithheld 對象目前不是暫不告知（來函還沒完成所以還沒有告知，或已經告知、由措施告知）。
	AuthorityBlockNoticeNotWithheld AuthorityOrderBlock = "NOTICE_NOT_WITHHELD"
)

// AuthorityOrderBlockValues 回傳全部來函不能做動作的原因（優先序；與契約的 AuthorityOrderUnavailableReason 一致）。
func AuthorityOrderBlockValues() []AuthorityOrderBlock {
	return []AuthorityOrderBlock{AuthorityBlockRevoked, AuthorityBlockCompleted, AuthorityBlockNoticeNotWithheld}
}

// Text 是原因給後台看的一句話（只有 zh-TW）；指令被擋下時錯誤的訊息取第一個原因的句子，API 的 *_unavailabilities 是同一句。不認得的原因回零值（Text 是空字串）。
func (b AuthorityOrderBlock) Text() uitext.Message {
	switch b {
	case AuthorityBlockRevoked:
		return text.AuthorityOrderUnavailableRevoked()
	case AuthorityBlockCompleted:
		return text.AuthorityOrderUnavailableCompleted()
	case AuthorityBlockNoticeNotWithheld:
		return text.AuthorityOrderNoticeNotWithheld()
	}
	return uitext.Message{}
}

// AuthorityOrderAvailability 是來函每個動作不能做的原因，空的就是可以做。RecordNotice 依對象的位置。
type AuthorityOrderAvailability struct {
	Complete, ExtendHold, Revoke []AuthorityOrderBlock
	RecordNotice                 [][]AuthorityOrderBlock
}

// Availability 是來函每個動作不能做的原因（依序列出，錯誤取第一個）。這是唯一的寫法：API 的 can_*、*_unavailabilities 與指令的前置檢查都只看它。
// 撤銷登記只在還沒完成時可以（登記錯誤要在處理之前更正；完成之後的錯誤寫在處理摘要）。延長保存只在撤銷登記之後不行：已完成或已到期的來函也能延長
// （資料可能還沒被清，或訴訟在完成之後才發生）；新的保存期限必須晚於目前的，那一項依賴輸入，在指令裡檢查。記下告知只對目前是暫不告知的對象。
func (o AuthorityOrder) Availability() AuthorityOrderAvailability {
	var a AuthorityOrderAvailability
	switch {
	case o.Revoked != nil:
		revoked := []AuthorityOrderBlock{AuthorityBlockRevoked}
		a.Complete, a.ExtendHold, a.Revoke = revoked, revoked, revoked
	case o.Completed != nil:
		a.Complete = []AuthorityOrderBlock{AuthorityBlockCompleted}
		a.Revoke = []AuthorityOrderBlock{AuthorityBlockCompleted}
	}
	a.RecordNotice = make([][]AuthorityOrderBlock, len(o.Targets))
	for i, t := range o.Targets {
		switch {
		case o.Revoked != nil:
			a.RecordNotice[i] = []AuthorityOrderBlock{AuthorityBlockRevoked}
		case t.Notice == nil || t.Notice.Kind != AuthorityNoticeWithheld:
			a.RecordNotice[i] = []AuthorityOrderBlock{AuthorityBlockNoticeNotWithheld}
		}
	}
	return a
}

// CanCreateMeasureOn 回報能不能從這份來函對指名範圍內的這家店建立警告、停權或風險措施（與 CreateMeasure 同一份規則 TerminationTarget.AllowsMeasure），
// CanCreateTerminationOn 是終止合作（AvailableTerminationGrounds 不為空，與 GetStoreResponse.can_create_termination 同一份規則）；撤銷登記的來函都不行
// （不能當作措施的來源）。
func (o AuthorityOrder) CanCreateMeasureOn(st AuthorityTargetStore) bool {
	return o.Revoked == nil && st.Target.AllowsMeasure()
}

// CanCreateTerminationOn 見 CanCreateMeasureOn。
func (o AuthorityOrder) CanCreateTerminationOn(st AuthorityTargetStore) bool {
	return o.Revoked == nil && len(AvailableTerminationGrounds(st.Target)) > 0
}

// AdminAuthorityOrder 是後台看到的一份來函：來函本身加上此刻可以做的動作與保存是否仍有效（讀取時的 now）。
type AdminAuthorityOrder struct {
	AuthorityOrder
	Availability AuthorityOrderAvailability
	HoldActive   bool
}

// AuthorityTargetInput 是登記來函時指名的一個對象。StoreID 為 nil 是帳號全部的店家。
type AuthorityTargetInput struct {
	AccountID  int64
	StoreID    *int64
	ProductIDs []string
}

// CreateAuthorityOrderInput 是登記來函的輸入。DueAt 為 nil 是收文後 DefaultAuthorityDue(Kind)，指定時必須在收文後 AuthorityDueMin 到 AuthorityDueMax 之間。
// RequestID 選填（AIP-155），範圍是（登記的管理員, request_id）。
type CreateAuthorityOrderInput struct {
	AdminID    int64
	RequestID  *uuid.UUID
	Agency     string
	DocumentNo string
	ReceivedAt time.Time
	Kind       AuthorityOrderKind
	Summary    string
	Targets    []AuthorityTargetInput
	DueAt      *time.Time
}

// CreateAuthorityOrderResult 是登記來函的結果；Replayed 表示這是同一個 request_id 的重送，回的是原本那一份。
type CreateAuthorityOrderResult struct {
	Order    AdminAuthorityOrder
	Replayed bool
}

// AuthorityNoticeInput 是一個對象的告知輸入；Kind 之外的欄位只看對應的種類（NOTIFIED 看 NotifiedAt，WITHHELD 看 RecheckOn），其餘忽略。
type AuthorityNoticeInput struct {
	Kind       AuthorityNoticeKind
	NotifiedAt *time.Time
	Note       string
	RecheckOn  *time.Time
}

// TargetNoticeInput 是完成來函時一個對象的告知。
type TargetNoticeInput struct {
	Position int
	Notice   AuthorityNoticeInput
}

// CompleteAuthorityOrderInput 是完成來函的輸入。ProvideTime 只給調取資料的來函（nil 是現在），必須在 [收文, 現在]；TargetNotices 每個對象剛好一筆。
type CompleteAuthorityOrderInput struct {
	ID                uuid.UUID
	AdminID           int64
	ExpectedVersion   int64
	ResolutionSummary string
	ProvideTime       *time.Time
	TargetNotices     []TargetNoticeInput
}

// ExtendAuthorityOrderHoldInput 是延長保存期限的輸入；HoldUntil 必須晚於目前的保存期限。
type ExtendAuthorityOrderHoldInput struct {
	ID              uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	HoldUntil       time.Time
	Reason          string
}

// RevokeAuthorityOrderInput 是撤銷登記的輸入；Reason 1 到 AuthorityNoteMaxRunes 字。
type RevokeAuthorityOrderInput struct {
	ID              uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	Reason          string
}

// RevokeAuthorityOrderResult 是撤銷登記的結果；AlreadyRevoked 表示這份來函之前已由同一位管理員撤銷（重送），回的是目前的紀錄。
type RevokeAuthorityOrderResult struct {
	Order          AdminAuthorityOrder
	AlreadyRevoked bool
}

// RecordAuthorityTargetNoticeInput 是記下暫不告知的對象後來的告知：Notice 只能是 NOTIFIED 或新的 WITHHELD。
type RecordAuthorityTargetNoticeInput struct {
	ID              uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	Position        int
	Notice          AuthorityNoticeInput
}

// AuthorityCursor 是來函清單的 keyset 位置：OPEN 是（期限, id）舊到新，其他是（收文時間, id）新到舊。
type AuthorityCursor struct {
	At time.Time
	ID uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c AuthorityCursor) Valid() bool { return c.ID != uuid.Nil() && !c.At.IsZero() }

// AuthorityOrderListInput 是來函清單的輸入；State 為 nil 不篩選（依收文時間新到舊），AccountID 為 0 不篩選。
type AuthorityOrderListInput struct {
	State     *AuthorityOrderState
	AccountID int64
	PageSize  int32
	After     *AuthorityCursor
}

// AdminAuthorityOrderPage 是一頁來函；Next 為 nil 表示沒有下一頁。
type AdminAuthorityOrderPage struct {
	Items []AdminAuthorityOrder
	Next  *AuthorityCursor
}
