package model

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
	"uuid"
)

// MeasureKind 是措施的種類：警告沒有限制，停權在期間內限制帳號，終止合作（賣家合作條款第 21 條）從生效起限制帳號、沒有固定結束時間，
// 內容下架讓店家的指定內容不再公開、不影響帳號的能力與曝光。
type MeasureKind string

const (
	// MeasureWarning 警告：沒有期間，也沒有效果。
	MeasureWarning MeasureKind = "WARNING"
	// MeasureSuspension 停權：未撤銷、開始之後、結束之前帳號被限制。
	MeasureSuspension MeasureKind = "SUSPENSION"
	// MeasureTermination 終止合作：未撤銷、開始之後帳號被限制（停權的效果），未結訂單完成後店家關閉；只能撤銷或申覆成立解除。
	MeasureTermination MeasureKind = "TERMINATION"
	// MeasureContentTakedown 內容下架：建立即生效、沒有結束時間、不限制帳號；對象是 Measure.Contents 的內容，建立後不可修訂；解除只走申覆。
	MeasureContentTakedown MeasureKind = "CONTENT_TAKEDOWN"
)

// MeasureKindValues 回傳全部措施種類，與 DB 的 measures_kind_check 一致。
func MeasureKindValues() []MeasureKind {
	return []MeasureKind{MeasureWarning, MeasureSuspension, MeasureTermination, MeasureContentTakedown}
}

// Restrictive 回報這個種類的措施有沒有限制帳號的效果（停權與終止合作），是「生效中」讀取的 kind 條件（EffectiveMeasures 的 kind IN）。
func (k MeasureKind) Restrictive() bool { return k == MeasureSuspension || k == MeasureTermination }

// HasEndTime 回報這個種類的措施可以有結束時間：停權與終止合作（終止合作只在預告期間內店家關閉時由系統寫入）；警告與內容下架沒有。
func (k MeasureKind) HasEndTime() bool { return k == MeasureSuspension || k == MeasureTermination }

// ContentKind 是內容下架措施的對象種類，儲存的代碼（measure_contents_content_kind_check）；值由 platform/contentkind 擁有，與 social 共用同一份。
type ContentKind = contentkind.Kind

const (
	// ContentPost 店家貼文。
	ContentPost = contentkind.Post
	// ContentComment 留言。
	ContentComment = contentkind.Comment
	// ContentForumEntry 討論區留言。
	ContentForumEntry = contentkind.ForumEntry
)

// ContentKindValues 回傳全部內容種類，與 DB 的 measure_contents_content_kind_check 一致。
func ContentKindValues() []ContentKind { return contentkind.Values() }

// ContentRef 是內容下架措施的一個對象：種類加上 social 的列 ID（不透明的外部參照，seller 不讀 social）。
type ContentRef struct {
	Kind ContentKind
	ID   uuid.UUID
}

// MaxTakedownContents 是一個內容下架措施最多的對象數。
const MaxTakedownContents = 20

// ValidateContentTargets 檢查內容下架對象清單的形狀：1 到 MaxTakedownContents 筆、種類合法、ID 不為空、(種類, ID) 不重複。內容屬於哪個店家由呼叫方（moderation）驗證。
func ValidateContentTargets(field string, refs []ContentRef) error {
	if n := len(refs); n == 0 || n > MaxTakedownContents {
		return apperr.InvalidInputOn([]string{field}, "內容下架的對象必須是 1 到 %d 則內容", MaxTakedownContents)
	}
	seen := make(map[ContentRef]bool, len(refs))
	for i, r := range refs {
		at := apperr.IndexedField(field, i)
		switch {
		case !r.Kind.Valid():
			return apperr.InvalidInputOn([]string{at}, "內容種類不合法")
		case r.ID == uuid.Nil():
			return apperr.InvalidInputOn([]string{at}, "內容 ID 不可為空")
		case seen[r]:
			return apperr.InvalidInputOn([]string{at}, "內容不可重複")
		}
		seen[r] = true
	}
	return nil
}

// Basis 是措施的依據：VIOLATION 依據違規的認定（固定期間的處分），RISK 依據條款第 17.1、17.5 條的風險類型（沒有固定期間時
// 由平台定期複審）。
type Basis string

const (
	// BasisViolation 依據認定。
	BasisViolation Basis = "VIOLATION"
	// BasisRisk 依據風險類型。
	BasisRisk Basis = "RISK"
	// BasisContract 依據契約條款（只配終止合作：預告終止、關聯帳號與沒有認定的第 21.3 條事由）。
	BasisContract Basis = "CONTRACT"
)

// BasisValues 回傳全部依據，與 DB 的 measures_basis_value_check 一致。
func BasisValues() []Basis { return []Basis{BasisViolation, BasisRisk, BasisContract} }

// TerminationGround 是終止合作的事由（賣家合作條款第 21.2、21.3 條）。
type TerminationGround string

const (
	// GroundNotice 預告終止（第 21.2 條）：建立後 NoticePeriod 生效，無過錯，依據 CONTRACT，不禁止日後重新申請開店。
	GroundNotice TerminationGround = "NOTICE"
	// GroundMaterialBreach 重大違約經通知七日內未改善（第 21.3 條第 1 款）：依據 VIOLATION，引用的認定至少一筆已存在 MaterialBreachCure 以上。
	GroundMaterialBreach TerminationGround = "MATERIAL_BREACH"
	// GroundCeasedBusiness 停業。
	GroundCeasedBusiness TerminationGround = "CEASED_BUSINESS"
	// GroundDissolution 解散。
	GroundDissolution TerminationGround = "DISSOLUTION"
	// GroundBankruptcy 破產。
	GroundBankruptcy TerminationGround = "BANKRUPTCY"
	// GroundLicenseRevoked 必要許可遭撤銷。
	GroundLicenseRevoked TerminationGround = "LICENSE_REVOKED"
	// GroundFoodSafetyIncident 重大食安事件。
	GroundFoodSafetyIncident TerminationGround = "FOOD_SAFETY_INCIDENT"
	// GroundFraud 詐欺事件。
	GroundFraud TerminationGround = "FRAUD"
	// GroundPlatformSecurity 危害平台安全。
	GroundPlatformSecurity TerminationGround = "PLATFORM_SECURITY"
	// GroundOtherSerious 其他足以嚴重影響履約的情形。
	GroundOtherSerious TerminationGround = "OTHER_SERIOUS"
	// GroundLinkedAccount 關聯帳號：與另一個帳號的終止（來源措施）使用同一個身分證明文件或統一編號；依據 CONTRACT，來源撤銷時連帶撤銷。
	GroundLinkedAccount TerminationGround = "LINKED_ACCOUNT"
)

// TerminationGroundValues 回傳全部終止事由，與 DB 的 measures_termination_ground_check 一致。
func TerminationGroundValues() []TerminationGround {
	return []TerminationGround{
		GroundNotice, GroundMaterialBreach, GroundCeasedBusiness, GroundDissolution, GroundBankruptcy, GroundLicenseRevoked,
		GroundFoodSafetyIncident, GroundFraud, GroundPlatformSecurity, GroundOtherSerious, GroundLinkedAccount,
	}
}

// ContractOnly 回報這個事由只能依據契約條款（CONTRACT），不能帶認定。
func (g TerminationGround) ContractOnly() bool { return g == GroundNotice || g == GroundLinkedAccount }

// TerminationViolationRequirement 是建立某個事由的終止合作時，依據（TerminationInput.violation）的需求。
type TerminationViolationRequirement string

const (
	// ViolationForbidden 不可帶依據：只能依據契約條款（CONTRACT）。
	ViolationForbidden TerminationViolationRequirement = "FORBIDDEN"
	// ViolationRequired 必須引用至少一筆認定。
	ViolationRequired TerminationViolationRequirement = "REQUIRED"
	// ViolationOptional 可以帶依據，也可以不帶（第 21.3 條的其他事由）。
	ViolationOptional TerminationViolationRequirement = "OPTIONAL"
)

// TerminationViolationRequirementValues 回傳全部需求值（不含 UNSPECIFIED），與 API 的 TerminationViolationRequirement 對得上（EnumMismatches 測試）。
func TerminationViolationRequirementValues() []TerminationViolationRequirement {
	return []TerminationViolationRequirement{ViolationRequired, ViolationOptional, ViolationForbidden}
}

// CitedFindingMinAge 是這個事由引用的認定至少要存在多久（有狀態的檢查在 CreateMeasure，也是 TerminationEffect.Message 與錯誤句的天數來源）：
// 需求是 REQUIRED 的事由是 MaterialBreachCure，其他事由沒有這條條件（0）。與 ViolationRequirement 同一個規則的兩面，不各自比較事由。
func (g TerminationGround) CitedFindingMinAge() time.Duration {
	if g.ViolationRequirement() == ViolationRequired {
		return MaterialBreachCure
	}
	return 0
}

// ViolationRequirement 是這個事由對依據的需求，CreateMeasure 的無狀態驗證與 API 的 TerminationOption.violation_requirement 都只看這一個函式。
// 條件（引用的認定要未撤銷、已存在 MaterialBreachCure 以上）是有狀態的檢查，在 CreateMeasure；它的天數由 TerminationEffect.Message 的說明句呈現。
func (g TerminationGround) ViolationRequirement() TerminationViolationRequirement {
	switch {
	case g.ContractOnly():
		return ViolationForbidden
	case g == GroundMaterialBreach:
		return ViolationRequired
	default:
		return ViolationOptional
	}
}

// 終止合作與關店後清理的時間常數。用固定的時數而不是日數：timestamptz 加日數依 session 時區計算，Go 與 SQL 的固定時數才處處相同；
// SQL 的 CHECK（measures_starts_check）與清理的查詢（RetireDueStores 的 hours 參數（第一段與第二段各一個））與這裡是同一組數字。
const (
	// NoticePeriod 是預告終止（第 21.2 條）的預告期間：30 日。
	NoticePeriod = 720 * time.Hour
	// AppealWindow 是終止合作的申覆期間：生效後 184 天，是任何連續六個曆月的最大長度，所以不論起算日都不短於條款的「六個月」與 DSA 20(1) 的 at
	// least six months。
	AppealWindow = 4416 * time.Hour
	// MaterialBreachCure 是第 21.3 條第 1 款（重大違約經通知後七日內未改善）的改善期：引用的認定至少要存在這麼久。
	MaterialBreachCure = 7 * 24 * time.Hour
	// StoreRetireAfter 是關店後的第一段清理（退場）：一般關店（賣家自己、隨帳號刪除）後 30 天，店家的公開內容（說明、獎項、網站、logo 與封面）清掉、
	// 發 StoreRetired 讓下游刪除（商品、貼文、聊天的個資等），之後不能恢復營業；終止合作關店不早於申覆期限（ADR 2026-10-19-seller-kyc-payuni-consolidated
	// 的 2026-10-20 修訂，Will 改選 Q2 (b)）。時數只從這一個常數來。
	StoreRetireAfter = 720 * time.Hour
	// StoreRetention 是關店後的第二段清理：關店後 184 天清註冊、身分與連線資料（店名、聯絡、地址、KYC、PayUni 的自由文字、食品登錄字號、同意紀錄的
	// IP 與 User-Agent）。184 天是任何連續六個曆月的最大長度，所以不論關店日都不短於詐欺犯罪危害防制條例第 37 條第 1 項的「六個月」。時數只從這一個常數來。
	StoreRetention = 4416 * time.Hour
	// IdentityHashRetention 是防詐保存的身分 blind index（account_identity_hashes）的保存期：來源 stint 目前這次關店滿 5 年才刪（Stripe 計畫 D3，Will 2026-10-29；
	// 參考金流業「關係結束後 5 年」的紀錄保存慣例，法源未確認，上線前與條款一起核對）。用固定時數：1826 天不短於任何連續五個曆年（最多含兩個閏日，1826 天）的長度，
	// 與 StoreRetention 同一個做法。與 StoreRetention 是不同的常數：理由不同（個資清理對比防詐比對），一個改了另一個不必跟著改。時數只從這一個常數來。
	IdentityHashRetention = 43824 * time.Hour
)

// 清理與申覆期間的整數時數，給清理查詢的 hours 參數（Go 的常數是唯一來源）。
const (
	AppealWindowHours     = int32(AppealWindow / time.Hour)
	StoreRetireAfterHours = int32(StoreRetireAfter / time.Hour)
	StoreRetentionHours   = int32(StoreRetention / time.Hour)

	// IdentityHashRetentionHours 是 IdentityHashRetention 的整數時數，給 ExpiredIdentityHashStores 與 DeleteExpiredAccountIdentityHashes 的 hours 參數。
	IdentityHashRetentionHours = int32(IdentityHashRetention / time.Hour)
)

// RiskGround 是 RISK 措施的風險類型（條款第 17.1 條）與外部要求（第 17.5 條）。
type RiskGround string

const (
	// RiskTransaction 交易風險。
	RiskTransaction RiskGround = "TRANSACTION"
	// RiskFoodSafety 食品安全。
	RiskFoodSafety RiskGround = "FOOD_SAFETY"
	// RiskFraud 詐欺。
	RiskFraud RiskGround = "FRAUD"
	// RiskPersonalData 個人資料。
	RiskPersonalData RiskGround = "PERSONAL_DATA"
	// RiskInfringement 侵權。
	RiskInfringement RiskGround = "INFRINGEMENT"
	// RiskSecurity 資安。
	RiskSecurity RiskGround = "SECURITY"
	// RiskFulfillment 履約。
	RiskFulfillment RiskGround = "FULFILLMENT"
	// RiskExternalOrder 主管機關或司法機關的要求。
	RiskExternalOrder RiskGround = "EXTERNAL_ORDER"
)

// RiskGroundValues 回傳全部風險類型，與 DB 的 measures_risk_ground_check 一致。
func RiskGroundValues() []RiskGround {
	return []RiskGround{
		RiskTransaction, RiskFoodSafety, RiskFraud, RiskPersonalData, RiskInfringement, RiskSecurity, RiskFulfillment, RiskExternalOrder,
	}
}

// MeasureStatus 是儲存的措施狀態；對外的狀態是 MeasureState，由它與時間推導。
type MeasureStatus string

const (
	// MeasureStatusEnforced 未撤銷。
	MeasureStatusEnforced MeasureStatus = "ENFORCED"
	// MeasureStatusRevoked 已撤銷。
	MeasureStatusRevoked MeasureStatus = "REVOKED"
)

// MeasureStatusValues 回傳全部儲存的措施狀態，與 DB 的 measures_status_check 一致。
func MeasureStatusValues() []MeasureStatus {
	return []MeasureStatus{MeasureStatusEnforced, MeasureStatusRevoked}
}

// MeasureState 是措施此刻的狀態，在讀取時由 StateOf 推導，不儲存。
type MeasureState string

const (
	// StateIssued 沒有限制效果的措施（警告、內容下架），未撤銷。
	StateIssued MeasureState = "ISSUED"
	// StateActive 停權生效中。
	StateActive MeasureState = "ACTIVE"
	// StateEnded 停權已結束（到期或平台結束）。
	StateEnded MeasureState = "ENDED"
	// StateRevoked 已撤銷。
	StateRevoked MeasureState = "REVOKED"
	// StateScheduled 預定生效：已建立、還沒到開始時間（預告終止的預告期間）；帳號此刻不受限制。
	StateScheduled MeasureState = "SCHEDULED"
)

// MeasureStateValues 回傳全部對外狀態。
func MeasureStateValues() []MeasureState {
	return []MeasureState{StateIssued, StateActive, StateEnded, StateRevoked, StateScheduled}
}

// StateOf 由儲存的欄位與時間推導措施的狀態。「生效中」的定義是 EffectiveMeasures 那一支查詢（status = ENFORCED、kind IN (SUSPENSION,
// TERMINATION)、starts_at <= now、measures.ends_at 為空或晚於 now），「預定生效」是 ScheduledMeasureIDs（starts_at > now 而且 ends_at 為空或晚於 now），
// 這裡是同一個定義的另一種寫法，以邊界測試鎖住它們一致；now 截到微秒，與資料庫比較時的精度相同。判斷順序：已結束先於預定生效（預告期間內
// 因店家關閉而結束的措施 ends_at 早於 starts_at，是 ENDED）；沒有限制效果的種類以外的未知種類 panic，不落進任何時間分支。
func StateOf(status MeasureStatus, kind MeasureKind, startsAt time.Time, endsAt *time.Time, now time.Time) MeasureState {
	now = now.UTC().Truncate(time.Microsecond)
	if status == MeasureStatusRevoked {
		return StateRevoked
	}
	switch {
	case kind == MeasureWarning || kind == MeasureContentTakedown:
		return StateIssued
	case !kind.Restrictive():
		panic("seller/model：StateOf 遇到不認得的措施種類 " + string(kind))
	case endsAt != nil && !endsAt.After(now):
		return StateEnded
	case startsAt.After(now):
		return StateScheduled
	}
	return StateActive
}

// RequestKind 是措施上賣家請求的種類。
type RequestKind string

const (
	// RequestKindAppeal 是申覆（條款第 18.4 條）：每個措施一次，接受時措施撤銷。
	RequestKindAppeal RequestKind = "APPEAL"
	// RequestKindRemediation 是解除請求（條款第 17.4 條）：只限依據風險的措施，可以重複，接受時措施結束。
	RequestKindRemediation RequestKind = "REMEDIATION"
)

// RequestKindValues 回傳全部請求種類，與 DB 的 measure_requests_kind_check 一致。
func RequestKindValues() []RequestKind {
	return []RequestKind{RequestKindAppeal, RequestKindRemediation}
}

// RequestState 是請求的狀態。
type RequestState string

const (
	// RequestPending 待處理。
	RequestPending RequestState = "PENDING"
	// RequestAccepted 接受：申覆是措施同時撤銷，解除請求是措施結束（不是撤銷）。
	RequestAccepted RequestState = "ACCEPTED"
	// RequestRejected 駁回。
	RequestRejected RequestState = "REJECTED"
	// RequestClosed 請求沒有經裁定就結案，原因是 RequestCloseCause。
	RequestClosed RequestState = "CLOSED"
)

// RequestCloseCause 是請求以 CLOSED 結案的原因。
type RequestCloseCause string

const (
	// CloseCauseRevoked 措施被撤銷，待處理的請求由撤銷的管理員結案。
	CloseCauseRevoked RequestCloseCause = "REVOKED"
	// CloseCauseEnded 措施已結束而待處理的解除請求失去標的，由系統結案（沒有管理員）。
	CloseCauseEnded RequestCloseCause = "ENDED"
)

// RequestCloseCauseValues 回傳全部結案原因，與 DB 的 measure_requests_close_cause_check 一致。
func RequestCloseCauseValues() []RequestCloseCause {
	return []RequestCloseCause{CloseCauseRevoked, CloseCauseEnded}
}

// 解除請求的冷卻（條款第 17.4 條）：同一措施上被駁回（REJECTED，不分種類，CLOSED 不計）的請求累計 RemediationFreeRejections 次後，
// 每再被駁回一次就從那次駁回起算一段冷卻，第一段 RemediationCooldownBase，之後每次加倍，上限 RemediationCooldownMax。
const (
	RemediationFreeRejections = 2
	RemediationCooldownBase   = 7 * 24 * time.Hour
	RemediationCooldownMax    = 28 * 24 * time.Hour
)

// CooldownAfterRejections 是累計 n 次駁回後（最近一次駁回起算）的冷卻長度；n 不超過 RemediationFreeRejections 時是 0。
func CooldownAfterRejections(n int) time.Duration {
	if n <= RemediationFreeRejections {
		return 0
	}
	d := RemediationCooldownBase
	for i := RemediationFreeRejections + 1; i < n && d < RemediationCooldownMax; i++ {
		d *= 2
	}
	return min(d, RemediationCooldownMax)
}

// RequestStateValues 回傳全部請求狀態，與 DB 的 measure_requests_state_check 一致。
func RequestStateValues() []RequestState {
	return []RequestState{RequestPending, RequestAccepted, RequestRejected, RequestClosed}
}

// ReviewWindow 是 RISK 措施複審期限的上限：建立與重排時 review_by 落在現在之後、這段時間以內。
const ReviewWindow = 30 * 24 * time.Hour

// MaxCitedFindings 是一個措施一次最多引用的既有認定數。
const MaxCitedFindings = 20

// MaxSourceModerationReports 是一個認定最多的 moderation 來源檢舉數。
const MaxSourceModerationReports = 20

// MaxSourceReports 是一個認定最多的來源通報數。
const MaxSourceReports = 20

// MeasureRequest 是措施上賣家的請求（申覆與解除請求）。CloseCause 只在 CLOSED 時有值。Revision 是提出時措施的那一版（measure_revisions），之後措施再被修改
// 也不變。Evidence 只在讀取單筆時載入。
type MeasureRequest struct {
	ID               uuid.UUID
	Kind             RequestKind
	State            RequestState
	Reason           string
	Revision         int32
	SubmittedAt      time.Time
	DecisionReason   *string
	DecidedByAdminID *int64
	DecidedAt        *time.Time
	CloseCause       *RequestCloseCause
	Evidence         []Evidence
	// DecideDeadline 是裁定的期限（seller.review_measure_appeal、seller.review_measure_lift_request）；請求不是待處理時 nil。
	DecideDeadline *Deadline
}

// MeasureRevision 是措施的一個版本：賣家看得到的完整內容與誰、何時寫入。EndsAt 為 nil 是沒有固定期間（與警告）。作者是管理員（AdminID），
// 或系統（SystemCause，沒有管理員：店家關閉時結束預告終止），兩者剛好一個。
type MeasureRevision struct {
	Revision          int32
	StatementToSeller string
	EndsAt            *time.Time
	AdminID           *int64
	SystemCause       *RevisionSystemCause
	CreatedAt         time.Time
}

// RevisionSystemCause 是系統寫入措施版本的原因。
type RevisionSystemCause string

// RevisionStoreClosed 店家關閉，預告終止（條款第 21.2 條）隨之結束。
const RevisionStoreClosed RevisionSystemCause = "STORE_CLOSED"

// RevisionSystemCauseValues 回傳全部系統寫入版本的原因，與 DB 的 measure_revisions_system_cause_check 一致。
func RevisionSystemCauseValues() []RevisionSystemCause {
	return []RevisionSystemCause{RevisionStoreClosed}
}

// GovernanceNote 是措施或案件的一則內部註記：平台人員之間的說明，賣家看不到；只新增，不能改也不能刪。SeverityChange 是改案件嚴重度時寫的那則註記帶的改前與改後，
// 只有案件的註記可能有，措施的註記是 nil。
type GovernanceNote struct {
	ID             uuid.UUID
	AdminID        int64
	Body           string
	SeverityChange *SeverityChange
	CreatedAt      time.Time
}

// SeverityChange 是案件嚴重度的一次修改（改前與改後）。
type SeverityChange struct{ From, To Severity }

// EffectiveMeasure 是帳號此刻生效中的一個停權或終止合作措施：legacy 只需要種類與事由、依據與風險類型、主要依據的類別。
type EffectiveMeasure struct {
	MeasureID         uuid.UUID
	Kind              MeasureKind
	TerminationGround *TerminationGround
	Basis             Basis
	RiskGround        *RiskGround
	LeadCategory      *policy.Category
}

// Measure 是一個措施。StatementToSeller 與 EndsAt 是目前值（measures 的欄位）；State 與冷卻是讀取當下（ReadAt）推導的；Findings 的第一筆是主要依據
// （建立時由規則決定，見 ViolationBasis），其餘依建立時間（只有 VIOLATION 依據有）；Requests 新到舊（清單載入，佐證只在讀取單筆時載入）；Revisions（全部版本，舊到新）只在 admin
// 讀取單筆與寫入的結果載入。內部註記不在措施裡（只新增、不經 Version 的子集合，用 MeasureNotes 分頁讀）。
type Measure struct {
	ID                   uuid.UUID
	AccountID, StoreID   int64
	Kind                 MeasureKind
	Basis                Basis
	RiskGround           *RiskGround
	ReviewBy             *time.Time
	Status               MeasureStatus
	Version              int64
	CurrentRevision      int32
	CreatedByAdminID     int64
	CreatedAt, UpdatedAt time.Time
	RevokedReason        *RevokeReason
	RevokedByAdminID     *int64
	RevokedAt            *time.Time
	StatementToSeller    string
	StartsAt             time.Time
	EndsAt               *time.Time
	TerminationGround    *TerminationGround
	SourceMeasureID      *uuid.UUID
	State                MeasureState
	ReadAt               time.Time
	// ReviewDeadline 是複審的期限（seller.review_measure_review）：風險類型、生效中、已進入 review_by 前 24 小時時有值；其餘 nil。
	ReviewDeadline *Deadline
	// StoreName 是店名；個資清理之後是空字串。
	StoreName string
	Findings  []Violation
	// Contents 是內容下架的對象（measure_contents，依種類、ID 排序）；其他種類沒有。
	Contents  []ContentRef
	Requests  []MeasureRequest
	Revisions []MeasureRevision
	// LinkedAccounts 是與這筆措施的帳號使用同一個身分證明文件或統一編號的其他帳號（身分快照反查）；只有 admin 讀取單筆時載入。
	LinkedAccounts []LinkedAccount
}

// LinkedIdentity 是兩個帳號相同的身分類別。
type LinkedIdentity string

const (
	// LinkedNationalID 賣家本人的身分證號相同。
	LinkedNationalID LinkedIdentity = "NATIONAL_ID"
	// LinkedUBN 統一編號相同。
	LinkedUBN LinkedIdentity = "UBN"
	// LinkedResponsiblePersonID 企業負責人的身分證號相同。
	LinkedResponsiblePersonID LinkedIdentity = "RESPONSIBLE_PERSON_ID"
	// LinkedBankAccount 銀行帳號（連同銀行代碼）相同。只列出、不能據以建立關聯終止（Stripe 計畫 D4：家人或配偶共用帳戶是常態，與 ADR
	// 2026-10-19-seller-kyc-payuni-consolidated「銀行帳號命中只標記，不擋」同一個判斷）。
	LinkedBankAccount LinkedIdentity = "BANK_ACCOUNT"
)

// LinkedIdentityValues 回傳全部命中的身分類別，依同時命中多種時的取用順序。
func LinkedIdentityValues() []LinkedIdentity {
	return []LinkedIdentity{LinkedNationalID, LinkedUBN, LinkedResponsiblePersonID, LinkedBankAccount}
}

// CountsAsGround 回報這種命中能不能當作建立關聯終止（LINKED_ACCOUNT）的依據：身分證號與統編可以，銀行帳號只列出。列出關聯帳號（linkedAccounts）與
// 建立時的再驗證（CreateMeasure）都只問它，所以「只列出、不能建立」只有這一個判斷點。
func (i LinkedIdentity) CountsAsGround() bool { return i != LinkedBankAccount }

// LinkedHit 是身分快照反查的一筆命中：別的帳號的 KYC 申請裡，有一種身分（含銀行帳號）與措施的快照相同。
type LinkedHit struct {
	AccountID int64
	Identity  LinkedIdentity
}

// LinkedAccount 是措施詳情上「相同身分的其他帳號」的一列：StoreID、StoreCode 是帳號目前的店家，沒有目前店家時是最近的一個 stint；Matched 同時命中多種
// 時取 LinkedIdentityValues 的第一種；LinkedMeasureID 是以本措施為來源、仍未撤銷的關聯終止（還沒有時是 nil）；CanCreateLinkedTermination 是此刻能不能
// 對這個帳號建立關聯終止（CanCreateLinkedTermination 函式的結果，與 CreateMeasure 的前置條件同一份規則）。
type LinkedAccount struct {
	AccountID, StoreID, StoreCode int64
	Matched                       LinkedIdentity
	LinkedMeasureID               *uuid.UUID
	CanCreateLinkedTermination    bool
	// TerminationEffect 是 CanCreateLinkedTermination 為 true 時建立這筆關聯終止的效果；false 時是 nil。
	TerminationEffect *TerminationEffect
}

// TerminationTarget 是建立終止合作時目標店家（stint）的狀態：Closed 是 stint 已關閉；LatestOfAccount 是它為帳號最近的一個 stint。
type TerminationTarget struct {
	Closed, LatestOfAccount bool
}

// AllowsMeasure 回報能不能在目標店家建立終止合作以外的措施（警告、停權、風險措施）：店家必須還沒關閉。CreateMeasure 的前置條件與來函的建立措施入口
// （AuthorityTargetStore.can_create_measure）都只看它。
func (t TerminationTarget) AllowsMeasure() bool { return !t.Closed }

// TerminationBlock 是目標店家擋下某個終止事由的原因；空字串表示不擋。
type TerminationBlock string

const (
	// TerminationBlockStoreClosed 預告終止只能建在開著的店（關閉的店沒有預告期可言）。
	TerminationBlockStoreClosed TerminationBlock = "STORE_CLOSED"
	// TerminationBlockStoreSuperseded 店家已關閉而且帳號有較新的 stint：第 21.3 條的限制效果記在帳號最近的一個 stint。
	TerminationBlockStoreSuperseded TerminationBlock = "STORE_SUPERSEDED"
)

// BlockFor 回答目標店家擋不擋事由 g 的終止：開著的店不擋任何事由；已關閉的店擋預告終止，其他事由只允許建在帳號最近的一個 stint
// （店關閉之後才發現的 21.3 事由，效果只剩禁止入駐）。CreateMeasure 的前置條件與 AvailableTerminationGrounds 都只看這一份。
func (t TerminationTarget) BlockFor(g TerminationGround) TerminationBlock {
	switch {
	case !t.Closed:
		return ""
	case g == GroundNotice:
		return TerminationBlockStoreClosed
	case !t.LatestOfAccount:
		return TerminationBlockStoreSuperseded
	}
	return ""
}

// AvailableTerminationGrounds 是此刻能對目標店家所屬的帳號建立的終止事由（依 TerminationGroundValues 的順序）。關聯帳號不在內：它需要來源措施，
// 能不能建立看 LinkedAccount.CanCreateLinkedTermination。MATERIAL_BREACH 要另外帶符合條件的認定，是輸入的條件，不是帳號的狀態。
func AvailableTerminationGrounds(t TerminationTarget) []TerminationGround {
	var out []TerminationGround
	for _, g := range TerminationGroundValues() {
		if g != GroundLinkedAccount && t.BlockFor(g) == "" {
			out = append(out, g)
		}
	}
	return out
}

// LinkedSourceBlock 是來源措施擋下關聯終止的原因；空字串表示不擋。
type LinkedSourceBlock string

const (
	// LinkedSourceRevoked 來源措施已撤銷。
	LinkedSourceRevoked LinkedSourceBlock = "REVOKED"
	// LinkedSourceNotEligible 來源措施不是第 21.3 條事由的終止合作（預告終止與關聯帳號的終止不能當來源，關聯終止不成鏈）。
	LinkedSourceNotEligible LinkedSourceBlock = "NOT_ELIGIBLE"
	// LinkedSourceSameAccount 來源措施和目標是同一個帳號。
	LinkedSourceSameAccount LinkedSourceBlock = "SAME_ACCOUNT"
)

// LinkedSource 是關聯終止的來源措施的事實。
type LinkedSource struct {
	Status    MeasureStatus
	Kind      MeasureKind
	Ground    *TerminationGround
	AccountID int64
}

// LinkedSourceOf 由措施列的欄位值組出 LinkedSource（ground 是列上的 termination_ground，可為 nil）。
func LinkedSourceOf(status, kind string, ground *string, accountID int64) LinkedSource {
	src := LinkedSource{Status: MeasureStatus(status), Kind: MeasureKind(kind), AccountID: accountID}
	if ground != nil {
		g := TerminationGround(*ground)
		src.Ground = &g
	}
	return src
}

// BlockFor 回答來源措施擋不擋對 targetAccountID 的關聯終止（依優先序：已撤銷、不是 21.3 事由的終止、同一個帳號）。
func (s LinkedSource) BlockFor(targetAccountID int64) LinkedSourceBlock {
	switch {
	case s.Status != MeasureStatusEnforced:
		return LinkedSourceRevoked
	case s.Kind != MeasureTermination || s.Ground == nil || s.Ground.ContractOnly():
		return LinkedSourceNotEligible
	case s.AccountID == targetAccountID:
		return LinkedSourceSameAccount
	}
	return ""
}

// CanCreateLinkedTermination 判斷此刻能不能以 source 為來源對 targetAccountID 建立關聯終止：來源措施不擋、這個帳號還沒有以它為來源的生效中關聯終止
// （alreadyLinked，唯一索引 measures_linked_source），目標店家不擋關聯帳號事由，而且至少有一種命中能當依據（groundHit：LinkedIdentity.CountsAsGround，只命中銀行帳號的
// 帳號只列出）。身分快照命中是呼叫方列出帳號的前提（命中才會有這一列），不在這裡。
// CreateMeasure 的前置條件逐項用同一組 BlockFor；alreadyLinked 在那裡由唯一索引回 AlreadyExists。
func CanCreateLinkedTermination(source LinkedSource, targetAccountID int64, groundHit, alreadyLinked bool, target TerminationTarget) bool {
	return groundHit && source.BlockFor(targetAccountID) == "" && !alreadyLinked && target.BlockFor(GroundLinkedAccount) == ""
}

// RemediationUnavailableReason 是賣家此刻不能請求解除的原因。
type RemediationUnavailableReason string

const (
	// ReasonRemediationPending 有待處理的解除請求。
	ReasonRemediationPending RemediationUnavailableReason = "REMEDIATION_PENDING"
	// ReasonAppealPending 有待處理的申覆。
	ReasonAppealPending RemediationUnavailableReason = "APPEAL_PENDING"
	// ReasonCooldown 在解除請求的冷卻期內。
	ReasonCooldown RemediationUnavailableReason = "COOLDOWN"
	// ReasonNotRisk 措施的依據不是風險類型。
	ReasonNotRisk RemediationUnavailableReason = "NOT_RISK"
	// ReasonNotActive 措施不在生效中（已撤銷、已結束或是警告）。
	ReasonNotActive RemediationUnavailableReason = "NOT_ACTIVE"
)

// RemediationUnavailableReasonValues 回傳全部不能請求解除的原因（與 API 的 RemediationUnavailableReason 一致）。
func RemediationUnavailableReasonValues() []RemediationUnavailableReason {
	return []RemediationUnavailableReason{ReasonRemediationPending, ReasonAppealPending, ReasonCooldown, ReasonNotRisk, ReasonNotActive}
}

// SellerVersion 是賣家的 etag 對應的版本：賣家看得到的版本（current_revision）。內部註記、複審期限不改變它。
func (m Measure) SellerVersion() int64 { return int64(m.CurrentRevision) }

// EffectiveRequest 是請求對外的狀態，也是它唯一的推導點（賣家與 admin 的輸出、列表與寫入的回應都經它）：解除請求只在措施生效中才有標的，
// 措施已不在生效中而儲存為 PENDING 的解除請求輸出 CLOSED（ENDED）、結案時間是措施的結束時間、沒有裁定的管理員；系統結案（SubmitAppeal 的
// transaction 與措施的結案掃描）寫回的就是這些值，所以收斂前後的輸出相同。其餘請求照儲存值。資料庫的唯一索引與 SubmitAppeal 的
// 結案看儲存值（MootRemediation）。
func (m Measure) EffectiveRequest(r MeasureRequest) MeasureRequest {
	if !m.moot(r) {
		return r
	}
	cause := CloseCauseEnded
	at := r.SubmittedAt
	if m.EndsAt != nil {
		at = *m.EndsAt
	}
	r.State, r.CloseCause, r.DecidedAt = RequestClosed, &cause, &at
	r.DecisionReason, r.DecidedByAdminID = nil, nil
	return r
}

func (m Measure) moot(r MeasureRequest) bool {
	return r.Kind == RequestKindRemediation && r.State == RequestPending && m.State != StateActive
}

// EffectiveRequests 回傳措施上全部的請求（對外的狀態，EffectiveRequest），新到舊。
func (m Measure) EffectiveRequests() []MeasureRequest {
	out := make([]MeasureRequest, len(m.Requests))
	for i, r := range m.Requests {
		out[i] = m.EffectiveRequest(r)
	}
	return out
}

// MootRemediation 回傳儲存為 PENDING、但措施已不在生效中而失去標的的解除請求（等著系統結案）；沒有時回 nil。
func (m Measure) MootRemediation() *MeasureRequest {
	for i := range m.Requests {
		if m.moot(m.Requests[i]) {
			return &m.Requests[i]
		}
	}
	return nil
}

// Appeal 回傳措施的申覆（每個措施至多一次）；沒有時回 nil。
func (m Measure) Appeal() *MeasureRequest { return m.request(RequestKindAppeal) }

func (m Measure) request(kind RequestKind) *MeasureRequest {
	for _, r := range m.Requests {
		if r.Kind == kind {
			e := m.EffectiveRequest(r)
			return &e
		}
	}
	return nil
}

// Pending 回傳待處理的請求（同時至多一筆，不分種類，對外的狀態：失去標的的解除請求不算）；沒有時回 nil。
func (m Measure) Pending() *MeasureRequest {
	for _, r := range m.Requests {
		if e := m.EffectiveRequest(r); e.State == RequestPending {
			return &e
		}
	}
	return nil
}

// PendingRemediation 回傳待處理的解除請求；沒有時回 nil。
func (m Measure) PendingRemediation() *MeasureRequest {
	if p := m.Pending(); p != nil && p.Kind == RequestKindRemediation {
		return p
	}
	return nil
}

func (m Measure) rejections() (count int, latest time.Time) {
	for _, r := range m.Requests {
		if r.State != RequestRejected || r.DecidedAt == nil {
			continue
		}
		count++
		if r.DecidedAt.After(latest) {
			latest = *r.DecidedAt
		}
	}
	return count, latest
}

// NextRemediationTime 是解除請求冷卻的結束時間：RISK 措施上被駁回的請求（申覆與解除請求合計）累計超過 RemediationFreeRejections 次後，
// 最近一次駁回起算的冷卻還沒過（以 ReadAt 為現在）時才有值；只在生效中的措施有值（已結束或撤銷的措施沒有解除請求入口）。由請求推導，
// 不另外儲存。
func (m Measure) NextRemediationTime() *time.Time {
	if m.Basis != BasisRisk || m.State != StateActive {
		return nil
	}
	n, latest := m.rejections()
	d := CooldownAfterRejections(n)
	if d == 0 {
		return nil
	}
	end := latest.Add(d)
	if !end.After(m.ReadAt) {
		return nil
	}
	return &end
}

// RejectionCooldown 是這個措施的下一個請求若被駁回會開始的冷卻長度；0 表示不會有冷卻（VIOLATION 依據與非生效中的措施都不適用）。
func (m Measure) RejectionCooldown() time.Duration {
	if m.Basis != BasisRisk || m.State != StateActive {
		return 0
	}
	n, _ := m.rejections()
	return CooldownAfterRejections(n + 1)
}

// 以下是每個動作的前置條件，方法的檢查與 API 的 can_* 共用這一份。

// MeasureActionUnavailableReason 是 admin 此刻不能修改措施（改理由、延長、縮短）或撤銷的原因。
type MeasureActionUnavailableReason string

const (
	// ActionReasonRevoked 措施已撤銷。
	ActionReasonRevoked MeasureActionUnavailableReason = "REVOKED"
	// ActionReasonNotActive 措施不在生效中：已結束、已撤銷，或還沒開始（預定生效的停權與終止合作）；延長與縮短時警告也算（警告沒有生效中的期間）。
	ActionReasonNotActive MeasureActionUnavailableReason = "NOT_ACTIVE"
	// ActionReasonKindHasNoEndTime 警告沒有結束時間。
	ActionReasonKindHasNoEndTime MeasureActionUnavailableReason = "KIND_HAS_NO_END_TIME"
	// ActionReasonTerminationRecreate 生效或預定生效（未撤銷）的終止合作沒有可調的期間，要改就撤銷後重建；已撤銷的終止合作只有 REVOKED，因為已經不能撤銷。
	ActionReasonTerminationRecreate MeasureActionUnavailableReason = "TERMINATION_RECREATE"
	// ActionReasonRequestPendingFrozen 賣家已提出申覆，或有待處理的解除請求：給賣家的理由與延長凍結；縮短與撤銷不受影響。
	ActionReasonRequestPendingFrozen MeasureActionUnavailableReason = "REQUEST_PENDING_FROZEN"
	// ActionReasonAppealPending 申覆待裁定，只能經 DecideAppeal。
	ActionReasonAppealPending MeasureActionUnavailableReason = "APPEAL_PENDING"
	// ActionReasonViolationBasis 依據違規認定的措施沒有結束入口：認定是只讀的，撤銷與申覆裁定都對措施進行（只出現在結束）。
	ActionReasonViolationBasis MeasureActionUnavailableReason = "VIOLATION_BASIS"
	// ActionReasonRemediationPending 有待處理的解除請求，要先裁定；接受解除請求就是結束（只出現在結束）。
	ActionReasonRemediationPending MeasureActionUnavailableReason = "REMEDIATION_PENDING"
	// ActionReasonContentTakedownRevokeOnly 內容下架沒有結束時間，也沒有結束入口（延長、縮短、結束）；要讓內容恢復只能撤銷或接受申覆。
	// 未撤銷的內容下架在延長、縮短與結束只回這一個原因（取代警告、不在生效中與依據違規認定的說法）。
	ActionReasonContentTakedownRevokeOnly MeasureActionUnavailableReason = "CONTENT_TAKEDOWN_REVOKE_ONLY"
)

// MeasureActionUnavailableReasonValues 回傳全部不能修改或撤銷的原因（與 API 的 MeasureActionUnavailableReason 一致）。
func MeasureActionUnavailableReasonValues() []MeasureActionUnavailableReason {
	return []MeasureActionUnavailableReason{
		ActionReasonRevoked, ActionReasonNotActive, ActionReasonKindHasNoEndTime, ActionReasonTerminationRecreate,
		ActionReasonRequestPendingFrozen, ActionReasonAppealPending, ActionReasonViolationBasis, ActionReasonRemediationPending,
		ActionReasonContentTakedownRevokeOnly,
	}
}

// MeasureAction 是 admin 對措施做的、有前置條件的動作；原因清單與文字都依動作決定。
type MeasureAction string

const (
	// MeasureActionUpdateStatement 改給賣家的理由。
	MeasureActionUpdateStatement MeasureAction = "UPDATE_STATEMENT"
	// MeasureActionExtend 延長結束時間。
	MeasureActionExtend MeasureAction = "EXTEND"
	// MeasureActionShorten 縮短結束時間。
	MeasureActionShorten MeasureAction = "SHORTEN"
	// MeasureActionRevoke 撤銷。
	MeasureActionRevoke MeasureAction = "REVOKE"
	// MeasureActionEnd 結束（風險已排除）。
	MeasureActionEnd MeasureAction = "END"
)

// MeasureActionValues 回傳全部有前置條件的動作。
func MeasureActionValues() []MeasureAction {
	return []MeasureAction{MeasureActionUpdateStatement, MeasureActionExtend, MeasureActionShorten, MeasureActionRevoke, MeasureActionEnd}
}

// ActionUnavailableReasons 回傳 admin 此刻不能做 action 的原因（依優先序，空表示可以）；未知的動作回 nil。
func (m Measure) ActionUnavailableReasons(action MeasureAction) []MeasureActionUnavailableReason {
	switch action {
	case MeasureActionUpdateStatement:
		return m.UpdateStatementUnavailableReasons()
	case MeasureActionExtend:
		return m.ExtendUnavailableReasons()
	case MeasureActionShorten:
		return m.ShortenUnavailableReasons()
	case MeasureActionRevoke:
		return m.RevokeUnavailableReasons()
	case MeasureActionEnd:
		return m.EndUnavailableReasons()
	}
	return nil
}

// ActionUnavailableText 是原因給使用者看的一句話（後台員工受眾，只有 zh-TW），API 的 MeasureActionUnavailability.message 與方法被拒絕時錯誤的 LocalizedMessage 都由它產生，
// 只有這一個函式。每句有主詞、不含 client 的畫面名稱；只有「凍結」依動作換句子（它只在改理由與延長出現）。不認得的原因或組合回零值（Text 是空字串）。
func ActionUnavailableText(reason MeasureActionUnavailableReason, action MeasureAction) uitext.Message {
	switch reason {
	case ActionReasonRevoked:
		return text.ActionUnavailableRevoked()
	case ActionReasonNotActive:
		return text.ActionUnavailableNotActive()
	case ActionReasonKindHasNoEndTime:
		return text.ActionUnavailableKindHasNoEndTime()
	case ActionReasonTerminationRecreate:
		return text.ActionUnavailableTerminationRecreate()
	case ActionReasonRequestPendingFrozen:
		switch action {
		case MeasureActionUpdateStatement:
			return text.ActionUnavailableRequestPendingFrozenUpdateStatement()
		case MeasureActionExtend:
			return text.ActionUnavailableRequestPendingFrozenExtend()
		case MeasureActionShorten, MeasureActionRevoke, MeasureActionEnd:
		}
	case ActionReasonAppealPending:
		return text.ActionUnavailableAppealPending()
	case ActionReasonViolationBasis:
		return text.ActionUnavailableViolationBasis()
	case ActionReasonRemediationPending:
		return text.ActionUnavailableRemediationPending()
	case ActionReasonContentTakedownRevokeOnly:
		return text.ActionUnavailableContentTakedownRevokeOnly()
	}
	return uitext.Message{}
}

// frozenByRequest 是賣家已提出申覆（含已裁定）或有待處理的解除請求：提出申覆起凍結理由與延長，解除請求待處理期間凍結加重，縮短與解除不受限。
func (m Measure) frozenByRequest() bool { return m.Appeal() != nil || m.PendingRemediation() != nil }

// UpdateStatementUnavailableReasons 是 admin 此刻不能改給賣家的理由的原因，全部列出並依優先序：結構性的在前（已撤銷、已結束），最後是賣家的請求造成的
// 凍結；空表示可以改。CanUpdateStatement 與 UpdateMeasure 的前置檢查、API 的 can_update_statement 都只看這一份。警告、預定生效與生效中的措施都可以改。
func (m Measure) UpdateStatementUnavailableReasons() []MeasureActionUnavailableReason {
	var out []MeasureActionUnavailableReason
	if m.Status != MeasureStatusEnforced {
		out = append(out, ActionReasonRevoked)
	}
	if m.State != StateIssued && m.State != StateActive && m.State != StateScheduled {
		out = append(out, ActionReasonNotActive)
	}
	if m.frozenByRequest() {
		out = append(out, ActionReasonRequestPendingFrozen)
	}
	return out
}

// CanUpdateStatement 判斷 admin 此刻能不能改給賣家的理由：沒有任何不能改的原因（UpdateStatementUnavailableReasons）。
func (m Measure) CanUpdateStatement() bool { return len(m.UpdateStatementUnavailableReasons()) == 0 }

// takedownRevokeOnly 補上內容下架在延長、縮短與結束的唯一結構性原因：未撤銷時是 CONTENT_TAKEDOWN_REVOKE_ONLY；已撤銷的只有 REVOKED（out 已帶），
// 同終止合作，已經不能再撤銷，不再多說「只能撤銷」。
func (m Measure) takedownRevokeOnly(out []MeasureActionUnavailableReason) []MeasureActionUnavailableReason {
	if m.Status == MeasureStatusEnforced {
		out = append(out, ActionReasonContentTakedownRevokeOnly)
	}
	return out
}

// endTimeUnavailableReasons 是結束時間可調的結構性前置，全部列出並依優先序：已撤銷、內容下架（未撤銷時只能撤銷，其餘原因都不列）、警告（沒有結束時間）、終止合作（未撤銷時，要撤銷後重建）、停權不在生效中；
// 空表示結構上可以調整。
func (m Measure) endTimeUnavailableReasons() []MeasureActionUnavailableReason {
	var out []MeasureActionUnavailableReason
	if m.Status != MeasureStatusEnforced {
		out = append(out, ActionReasonRevoked)
	}
	if m.Kind == MeasureContentTakedown {
		return m.takedownRevokeOnly(out)
	}
	if m.Kind == MeasureWarning {
		out = append(out, ActionReasonKindHasNoEndTime)
	}
	if m.Kind == MeasureTermination && m.Status == MeasureStatusEnforced {
		out = append(out, ActionReasonTerminationRecreate)
	}
	if m.State != StateActive {
		out = append(out, ActionReasonNotActive)
	}
	return out
}

// ExtendUnavailableReasons 是 admin 此刻不能延長結束時間的原因，全部列出並依優先序：結構性的在前（已撤銷、警告、終止合作、停權不在生效中），最後是賣家的請求
// 造成的凍結；空表示可以延長。已結束的措施不能延長，再限制要另立措施。CanExtend 與 UpdateMeasure 的前置檢查、API 的 can_extend 都只看這一份。
func (m Measure) ExtendUnavailableReasons() []MeasureActionUnavailableReason {
	out := m.endTimeUnavailableReasons()
	if m.frozenByRequest() && m.Kind != MeasureContentTakedown { // 內容下架沒有結束時間可延長，凍結不是它的原因
		out = append(out, ActionReasonRequestPendingFrozen)
	}
	return out
}

// CanExtend 判斷能不能延長結束時間：沒有任何不能延長的原因（ExtendUnavailableReasons）。
func (m Measure) CanExtend() bool { return len(m.ExtendUnavailableReasons()) == 0 }

// ShortenUnavailableReasons 是 admin 此刻不能縮短結束時間的原因，只有結構性的（凍結後仍可縮短）；空表示可以縮短。
// CanShorten 與 UpdateMeasure 的前置檢查、API 的 can_shorten 都只看這一份。
func (m Measure) ShortenUnavailableReasons() []MeasureActionUnavailableReason {
	return m.endTimeUnavailableReasons()
}

// CanShorten 判斷能不能縮短結束時間：沒有任何不能縮短的原因（ShortenUnavailableReasons）。
func (m Measure) CanShorten() bool { return len(m.ShortenUnavailableReasons()) == 0 }

// EndUnavailableReasons 是 admin 此刻不能結束措施的原因，全部列出並依優先序：已撤銷、不在生效中、依據不是風險類型（終止合作是要撤銷後重建，依據違規認定的
// 措施要撤銷或縮短），最後是待處理的解除請求（要先裁定，接受解除請求就是結束）；空表示可以結束（只限生效中的 RISK 停權）。
// CanEnd 與 End 的前置檢查、API 的 can_end 都只看這一份。
func (m Measure) EndUnavailableReasons() []MeasureActionUnavailableReason {
	var out []MeasureActionUnavailableReason
	if m.Status != MeasureStatusEnforced {
		out = append(out, ActionReasonRevoked)
	}
	if m.Kind == MeasureContentTakedown {
		return m.takedownRevokeOnly(out)
	}
	if m.State != StateActive {
		out = append(out, ActionReasonNotActive)
	}
	if m.Basis != BasisRisk {
		if m.Kind == MeasureTermination {
			out = append(out, ActionReasonTerminationRecreate)
		} else {
			out = append(out, ActionReasonViolationBasis)
		}
	}
	if m.PendingRemediation() != nil {
		out = append(out, ActionReasonRemediationPending)
	}
	return out
}

// CanEnd 判斷能不能結束：沒有任何不能結束的原因（EndUnavailableReasons）。
func (m Measure) CanEnd() bool { return len(m.EndUnavailableReasons()) == 0 }

// RevokeUnavailableReasons 是 admin 此刻不能撤銷措施的原因，全部列出並依優先序：已撤銷、申覆待裁定（只能經 DecideAppeal）；空表示可以撤銷。已結束的措施可以撤銷。
// CanRevoke 與 RevokeMeasure 的前置檢查、API 的 can_revoke 都只看這一份。
func (m Measure) RevokeUnavailableReasons() []MeasureActionUnavailableReason {
	var out []MeasureActionUnavailableReason
	if m.Status != MeasureStatusEnforced {
		out = append(out, ActionReasonRevoked)
	}
	if pending := m.Pending(); pending != nil && pending.Kind == RequestKindAppeal {
		out = append(out, ActionReasonAppealPending)
	}
	return out
}

// CanRevoke 判斷能不能撤銷：沒有任何不能撤銷的原因（RevokeUnavailableReasons）。
func (m Measure) CanRevoke() bool { return len(m.RevokeUnavailableReasons()) == 0 }

// CanRevokeFindings 判斷撤銷（或接受申覆）時能不能一併撤銷認定：依據是 VIOLATION，而且措施自己建立（認定的發動措施是它）了至少一筆尚未撤銷的認定。
// 只引用別的措施建立的認定時沒有可撤銷的認定。RevokeMeasure、DecideAppeal 的前置檢查與 API 的 can_revoke_findings 都只看這一份；Findings 要已載入。
func (m Measure) CanRevokeFindings() bool {
	if m.Basis != BasisViolation {
		return false
	}
	for _, f := range m.Findings {
		if f.OriginMeasureID == m.ID && f.Status == ViolationStatusEnforced {
			return true
		}
	}
	return false
}

// CanScheduleReview 判斷能不能重排複審期限：生效中的 RISK 停權。
func (m Measure) CanScheduleReview() bool { return m.State == StateActive && m.Basis == BasisRisk }

// CanDecideAppeal 判斷有沒有待裁定的申覆。
func (m Measure) CanDecideAppeal() bool {
	appeal := m.Appeal()
	return appeal != nil && appeal.State == RequestPending
}

// CanDecideRemediation 判斷有沒有可以裁定的解除請求：待處理的解除請求，而且措施生效中（解除請求只在措施生效中才有標的）。
func (m Measure) CanDecideRemediation() bool {
	return m.State == StateActive && m.PendingRemediation() != nil
}

// AppealUnavailableReason 是賣家此刻不能對措施提出申覆的原因。
type AppealUnavailableReason string

const (
	// AppealReasonRevoked 措施已撤銷。
	AppealReasonRevoked AppealUnavailableReason = "REVOKED"
	// AppealReasonAlreadyAppealed 同一措施只能申覆一次。
	AppealReasonAlreadyAppealed AppealUnavailableReason = "ALREADY_APPEALED"
	// AppealReasonWindowEnded 終止合作的申覆期間已結束（讀取時間不早於 AppealDeadline）。
	AppealReasonWindowEnded AppealUnavailableReason = "APPEAL_WINDOW_ENDED"
	// AppealReasonRequestPending 措施上有待處理的請求（申覆或解除請求）。
	AppealReasonRequestPending AppealUnavailableReason = "REQUEST_PENDING"
)

// AppealUnavailableReasonValues 回傳全部不能申覆的原因（與 API 的 AppealUnavailableReason 一致）。
func AppealUnavailableReasonValues() []AppealUnavailableReason {
	return []AppealUnavailableReason{AppealReasonRevoked, AppealReasonAlreadyAppealed, AppealReasonWindowEnded, AppealReasonRequestPending}
}

// AppealDeadline 是終止合作的申覆期限：生效（StartsAt）後 AppealWindow；其他種類沒有。
func (m Measure) AppealDeadline() *time.Time {
	if m.Kind != MeasureTermination {
		return nil
	}
	d := m.StartsAt.Add(AppealWindow)
	return &d
}

// AppealUnavailableReasons 是賣家此刻不能申覆的原因，依優先序：已撤銷、已申覆過、申覆期間已過（終止合作，讀取時間 ReadAt 不早於期限）、有待處理的請求
// （措施已結束而仍待處理的解除請求不算）；空表示可以申覆。CanAppeal、SubmitAppeal 的前置檢查與 API 的 can_appeal 都只看這一份。已結束的措施
// 也可以申覆（條款第 18.4 條不設期限）；預定生效的措施也可以（預告期間就是申覆期）。
func (m Measure) AppealUnavailableReasons() []AppealUnavailableReason {
	var out []AppealUnavailableReason
	if m.Status != MeasureStatusEnforced {
		out = append(out, AppealReasonRevoked)
	}
	if m.Appeal() != nil {
		out = append(out, AppealReasonAlreadyAppealed)
	}
	if d := m.AppealDeadline(); d != nil && !m.ReadAt.Before(*d) {
		out = append(out, AppealReasonWindowEnded)
	}
	if m.Pending() != nil {
		out = append(out, AppealReasonRequestPending)
	}
	return out
}

// CanAppeal 判斷賣家此刻能不能提出申覆：沒有任何不能申覆的原因（AppealUnavailableReasons）。
func (m Measure) CanAppeal() bool { return len(m.AppealUnavailableReasons()) == 0 }

// RemediationUnavailableReasons 是賣家此刻不能請求解除的原因，依優先序：結構性的原因在前（依據不是 RISK、不在生效中：已撤銷、已結束與警告都算），因為它們也回答「有沒有解除請求入口」，再來是待處理的請求，最後是冷卻期；空表示可以請求。CanRequestRemediation 與 SubmitAppeal 的失敗訊息都只看這一份。
func (m Measure) RemediationUnavailableReasons() []RemediationUnavailableReason {
	var out []RemediationUnavailableReason
	if m.Basis != BasisRisk {
		out = append(out, ReasonNotRisk)
	}
	if m.State != StateActive {
		out = append(out, ReasonNotActive)
	}
	if p := m.Pending(); p != nil {
		if p.Kind == RequestKindAppeal {
			out = append(out, ReasonAppealPending)
		} else {
			out = append(out, ReasonRemediationPending)
		}
	}
	if m.NextRemediationTime() != nil {
		out = append(out, ReasonCooldown)
	}
	return out
}

// CanRequestRemediation 判斷賣家此刻能不能請求解除：沒有任何不能請求的原因（RISK 依據、未撤銷、生效中、沒有待處理的請求、不在冷卻期）。
func (m Measure) CanRequestRemediation() bool { return len(m.RemediationUnavailableReasons()) == 0 }

// ViolationBasis 是 VIOLATION 依據的認定：新的認定與引用的既有認定，合計至少一筆。CitedFindingIDs 是集合，順序沒有意義；主要依據
// （lead_finding_id）由規則決定，不看位置：有新的認定時是新的認定，否則是引用的認定中最嚴重的，同嚴重度取最新建立的（LeadFindingOf）。
type ViolationBasis struct {
	Finding         *NewFinding
	CitedFindingIDs []uuid.UUID
}

// NewFinding 是要新建的認定；SourceReportIDs 是成立它的通報（最多 MaxSourceReports 筆、不重複，每一筆都是同一個店家、不能是不成立的）。
type NewFinding struct {
	Category        policy.Category
	Severity        Severity
	SourceReportIDs []uuid.UUID
	// SourceAuthorityOrderIDs 是成立它的主管機關來函（最多 MaxSourceAuthorityOrders 筆、不重複，是集合）；每一筆都必須指名這個措施的帳號
	// （指名店家時只算措施的那一家）。
	SourceAuthorityOrderIDs []uuid.UUID
	// SourceModerationReportIDs 是成立它的 moderation 檢舉（最多 MaxSourceModerationReports 筆、不重複，是集合）：不透明的外部參照，seller 不驗證它們存在，
	// 驗證與冪等由呼叫方（moderation）擁有。
	SourceModerationReportIDs []uuid.UUID
}

// RiskBasis 是 RISK 依據。
type RiskBasis struct{ Ground RiskGround }

// CreateMeasureInput 是建立措施的輸入。RequestID 是選填的冪等鍵（範圍 AdminID，永久保存）；EndsAt、ReviewBy 的規則見
// measure.Create。Note 是第一則內部註記（必填）。Violation、Risk 與 Termination 剛好一個（終止合作的依據在 Termination 裡）。
type CreateMeasureInput struct {
	AdminID           int64
	RequestID         *uuid.UUID
	StoreID           int64
	Kind              MeasureKind
	StatementToSeller string
	EndsAt            *time.Time
	ReviewBy          *time.Time
	Note              string
	Violation         *ViolationBasis
	Risk              *RiskBasis
	// SourceCaseID 是從案件建立時的來源案件（選填；任何種類與依據都可以）：措施的店家必須是案件的店家、案件必須進行中；之後這個措施算是案件導出的措施。
	SourceCaseID *uuid.UUID
	// SourceAuthorityOrderID 是從主管機關來函建立時的來函（選填；任何種類與依據都可以）：來函必須指名這個措施的帳號（指名店家時只算那一家），來函已完成也可以；
	// 之後這個措施算是來函導出的措施。
	SourceAuthorityOrderID *uuid.UUID
	// Termination 是終止合作（Kind 是 MeasureTermination）的事由與依據；其他種類不可以設。依據由事由與 Violation 推導（有 Violation 是
	// VIOLATION，否則 CONTRACT）。
	Termination *TerminationBasis
	// Takedown 是內容下架（Kind 是 MeasureContentTakedown）的對象；其他種類不可以設。依據一定是 Violation。
	Takedown *ContentTakedown
}

// ContentTakedown 是內容下架措施的對象。
type ContentTakedown struct{ Contents []ContentRef }

// TerminationBasis 是終止合作的事由與依據：Violation 只有第 21.3 條的事由可以帶（MATERIAL_BREACH 必帶），SourceMeasureID 只有
// LINKED_ACCOUNT 帶（也必帶）。
type TerminationBasis struct {
	Ground          TerminationGround
	Violation       *ViolationBasis
	SourceMeasureID *uuid.UUID
}

// CreateMeasureResult 是建立措施的結果；Replayed 表示這是冪等的重送，回傳的是原本措施的目前狀態，沒有新增。
type CreateMeasureResult struct {
	Measure  Measure
	Replayed bool
}

// UpdateMeasureInput 是措施賣家可見內容的部分更新：nil 保留原值。ChangeEndsAt 為 true 時 EndsAt 是新的結束時間，nil 是清除
// （RISK 改成沒有固定期間）。ExpectedVersion 必填（後台的 version）。
type UpdateMeasureInput struct {
	MeasureID         uuid.UUID
	AdminID           int64
	ExpectedVersion   int64
	StatementToSeller *string
	ChangeEndsAt      bool
	EndsAt            *time.Time
}

// EndMeasureInput 是結束 RISK 措施的輸入。
type EndMeasureInput struct {
	MeasureID       uuid.UUID
	AdminID         int64
	ExpectedVersion int64
}

// RevokeMeasureInput 是 admin 撤銷措施的輸入；Reason 要是 RevokeReason 的值（管理員能選的子集由契約的 AdminRevokeReason 限定，模組自己也用 ManualRevokeReasonValues 檢查，不靠契約層）。RevokeFindings 為 true 時
// 一併撤銷這個措施自己建立的認定，並連帶撤銷引用那些認定的其他措施。
type RevokeMeasureInput struct {
	MeasureID       uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	Reason          RevokeReason
	RevokeFindings  bool
}

// ScheduleReviewInput 是重排 RISK 措施複審期限的輸入；Note 是同一個 transaction 寫入的內部註記（必填）。
type ScheduleReviewInput struct {
	MeasureID       uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	ReviewBy        time.Time
	Note            string
}

// SubmitAppealInput 是賣家提出申覆的輸入；AccountID 是登入的帳號（也是佐證的上傳者）。ExpectedVersion 必填，是賣家讀到的版本
// （Measure.SellerVersion）：內容在那之後被管理員改過就要重新讀取。
type SubmitAppealInput struct {
	AccountID       int64
	MeasureID       uuid.UUID
	ExpectedVersion int64
	Reason          string
	EvidenceTokens  []string
}

// DecideAppealInput 是 admin 裁定申覆的輸入：Outcome 是 ACCEPTED 或 REJECTED；接受與駁回都必須附理由（條款第 18.4 條）。
// RevokeFindings 只在 ACCEPTED 時可以為 true。
type DecideAppealInput struct {
	MeasureID       uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	Outcome         RequestState
	Reason          string
	RevokeFindings  bool
}

// RequestRemediationInput 是賣家請求解除的輸入；欄位同 SubmitAppealInput。
type RequestRemediationInput struct {
	AccountID       int64
	MeasureID       uuid.UUID
	ExpectedVersion int64
	Reason          string
	EvidenceTokens  []string
}

// DecideRemediationInput 是 admin 裁定待處理解除請求的輸入：Outcome 是 ACCEPTED 或 REJECTED，都必須附理由。不帶請求 ID：同一措施
// 同時最多一筆待處理請求，版本保證裁定的就是 admin 讀到的那一筆。
type DecideRemediationInput struct {
	MeasureID       uuid.UUID
	AdminID         int64
	ExpectedVersion int64
	Outcome         RequestState
	Reason          string
}

// CreateMeasureNoteInput 是新增措施內部註記的輸入。註記不帶版本：它不是賣家看得到的內容，也不改措施的 version。
type CreateMeasureNoteInput struct {
	MeasureID uuid.UUID
	AdminID   int64
	Body      string
}

// CreateCaseNoteInput 是新增案件內部註記的輸入。註記不帶版本，也不改案件的 version；改嚴重度的改前改後只由 ChangeCaseSeverity 寫，這裡沒有。
type CreateCaseNoteInput struct {
	CaseID  uuid.UUID
	AdminID int64
	Body    string
}

// NoteCursor 是內部註記的 keyset 位置（舊到新）。
type NoteCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c NoteCursor) Valid() bool { return c.ID != uuid.Nil() && !c.CreatedAt.IsZero() }

// MeasureNoteListInput 是措施內部註記的列表輸入。
type MeasureNoteListInput struct {
	MeasureID uuid.UUID
	PageSize  int32
	After     *NoteCursor
}

// CaseNoteListInput 是案件內部註記的列表輸入。
type CaseNoteListInput struct {
	CaseID   uuid.UUID
	PageSize int32
	After    *NoteCursor
}

// NotePage 是一頁內部註記；Next 為 nil 表示沒有下一頁。
type NotePage struct {
	Items []GovernanceNote
	Next  *NoteCursor
}

// MeasureCursor 是措施列表的 keyset 位置：SortAt 是列表排序用的時間（預設是建立時間，待處理申覆的收件匣是申覆提出時間，
// 複審到期的收件匣是 review_by）。
type MeasureCursor struct {
	SortAt time.Time
	ID     uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c MeasureCursor) Valid() bool { return c.ID != uuid.Nil() && !c.SortAt.IsZero() }

// AdminMeasureListInput 是 admin 的措施列表；StoreID、AccountID 為 0、其餘為 nil 或 false 時不篩。ReviewDue 只列複審到期的
// RISK 措施（成員是 seller.review_measure_review(now)：生效中、現在不早於 review_by 前 24 小時），依 review_by 舊到新；RequestState 是集合篩選（有任一筆請求，申覆與解除請求，對外的狀態
// 是這個狀態，EffectiveRequest），是 PENDING 時是待處理請求的收件匣，依請求提出時間舊到新；RequestKind 只看那個種類的請求；
// 收件匣排序的優先序是 ReviewDue、RequestState PENDING，條件可以組合；其餘新到舊。
type AdminMeasureListInput struct {
	StoreID, AccountID int64
	State              *MeasureState
	Basis              *Basis
	RequestState       *RequestState
	RequestKind        *RequestKind
	ReviewDue          bool
	// SourceCaseID 只列這個案件導出的措施（來源案件是它，或依據的認定以它的通報為來源）；SourceReportID 只列依據的認定以這筆通報為來源的措施。
	SourceCaseID, SourceReportID *uuid.UUID
	// SourceAuthorityOrderID 只列這份來函導出的措施（來源來函是它，或依據的認定引用了它）。
	SourceAuthorityOrderID *uuid.UUID
	PageSize               int32
	After                  *MeasureCursor
}

// AccountMeasureListInput 是賣家自己（帳號所有 stint）的措施列表，新到舊；State 為 nil 時不篩。
type AccountMeasureListInput struct {
	AccountID int64
	State     *MeasureState
	PageSize  int32
	After     *MeasureCursor
}

// MeasurePage 是一頁措施；Next 為 nil 表示沒有下一頁。
type MeasurePage struct {
	Items []Measure
	Next  *MeasureCursor
}

// MeasureCreated 在建立措施的 transaction 內發出。StartsAt 是措施開始的時間（不可變；預告終止在未來，其餘是建立的時間），EndsAt 是第 1 版的
// 結束時間；TerminationGround 是終止合作的事由。StoreSuspended 另由停權期間的變化發出（帳號已經停權時不會再發），預告終止生效時由
// seller.termination_effect 發出，沒有獨立的「終止預告」事件。
type MeasureCreated struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	StoreID, AccountID int64
	Kind               MeasureKind
	Basis              Basis
	RiskGround         *RiskGround
	TerminationGround  *TerminationGround
	StartsAt           time.Time
	EndsAt             *time.Time
	// Contents 是內容下架的對象（其他種類是空）。
	Contents []ContentRef
}

// MeasureRevised 在賣家可見的內容（理由、結束時間）實際改變的 transaction 內發出；Revision 是新的那一版。結束（EndMeasure）
// 發 MeasureEnded，不發這個。
type MeasureRevised struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	StoreID, AccountID int64
	Revision           int32
	EndsAt             *time.Time
}

// MeasureEnded 在 admin 結束 RISK 措施、接受解除請求（同一個 transaction 也發 MeasureRequestDecided{REMEDIATION, ACCEPTED}），或店家關閉而結束
// 預告終止（條款第 21.2 條，作者是系統）的 transaction 內發出。自然到期不發。
type MeasureEnded struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	StoreID, AccountID int64
}

// MeasureRevoked 在措施撤銷的 transaction 內發出（admin 撤銷、接受申覆或連帶撤銷）；效果的解除另由 StoreUnsuspended 表示。
type MeasureRevoked struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	StoreID, AccountID int64
	Reason             RevokeReason
	// Contents 是撤銷的內容下架措施的對象（其他種類是空）。
	Contents []ContentRef
}

// MeasureRequestSubmitted 在賣家提出請求的 transaction 內發出（A8 的 notification 提醒 admin）。
type MeasureRequestSubmitted struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	RequestID          uuid.UUID
	Kind               RequestKind
	StoreID, AccountID int64
}

// MeasureRequestDecided 在 admin 裁定請求的 transaction 內發出；Outcome 是 ACCEPTED 或 REJECTED。接受申覆時同一個 transaction
// 也發 MeasureRevoked，接受解除請求時發 MeasureEnded。
type MeasureRequestDecided struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	RequestID          uuid.UUID
	Kind               RequestKind
	StoreID, AccountID int64
	Outcome            RequestState
}

// MeasureRequestClosed 在待處理的請求以 CLOSED 結案的 transaction 內發出；Cause 是結案原因（措施撤銷，或措施已結束而解除請求失去標的）。
type MeasureRequestClosed struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	MeasureID          uuid.UUID
	RequestID          uuid.UUID
	Kind               RequestKind
	Cause              RequestCloseCause
	StoreID, AccountID int64
}

// StoreMeasures 是店家與帳號此刻生效中的停權措施的同一次讀取（後台的店家詳情）：Store.Suspended、Store.EffectiveMeasures 與 Active
// 來自同一個快照、同一個時間點。Active 新到舊，帶依據與請求，不帶佐證與全部版本。
type StoreMeasures struct {
	Store  Store
	Active []Measure
	// Terminations 是此刻能對這個店家所屬的帳號建立的終止合作，每個事由一筆（事由來自 AvailableTerminationGrounds，與 CreateMeasure 的前置條件同一份規則，
	// 效果在讀取時間點建立，見 NewTerminationEffects）；空表示不能建立。
	Terminations []TerminationEffect
	// CreateMeasure 是此刻不能建立警告、停權與風險措施的原因（Store.ActionUnavailableReasons(StoreActionCreateMeasure)，與 CreateMeasure 的前置條件同一個規則；空表示可以）。
	CreateMeasure []StoreActionUnavailableReason
}
