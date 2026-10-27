package seller

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 治理（ADR 2026-10-18-seller-governance-consolidated）的公開型別與方法。

// Severity 是嚴重度。
type Severity = model.Severity

// Outcome 是一筆通報的結果（已採取措施、未違規、內容不足、重複、濫報）。
type Outcome = model.Outcome

// Decision 是案件的決定（採取措施、不採取措施），決定即結案。
type Decision = model.Decision

// CaseCloseReason 是案件結案的原因（管理員決定、通報都移走）。
type CaseCloseReason = model.CaseCloseReason

// ReportState 是通報此刻的處理狀態（待分案、在案件裡、已有結果），讀取時推導。
type ReportState = model.ReportState

// CaseState 是案件此刻的狀態（進行中、已結案），讀取時推導。
type CaseState = model.CaseState

// SupplementMatch 是匿名的補充以哪一種聯絡方式比對到原通報。
type SupplementMatch = model.SupplementMatch

// ReportBlock 是通報此刻不能做某個動作的一個原因。
type ReportBlock = model.ReportBlock

// CaseBlock 是案件此刻不能做某個動作的一個原因。
type CaseBlock = model.CaseBlock

// CaseAction 是案件上有前置條件的動作（CaseBlock.Text 依它選句）。
type CaseAction = model.CaseAction

// FDAChannel 是食安事件轉報主管機關的管道。
type FDAChannel = model.FDAChannel

// InquiryKind 是詢問要賣家做的事（提供紀錄、請說明）。
type InquiryKind = model.InquiryKind

// InquiryReplyChannel 是詢問的回覆怎麼收到的（電話、email、通訊軟體、其他）。
type InquiryReplyChannel = model.InquiryReplyChannel

// InquiryWithdrawCause 是詢問撤回的原因（管理員撤回、案件結案）。
type InquiryWithdrawCause = model.InquiryWithdrawCause

// InquiryState 是詢問此刻的狀態（等賣家回覆、已回覆、已撤回），讀取時推導。
type InquiryState = model.InquiryState

// InquiryBlock 是詢問此刻不能做某個動作的一個原因。
type InquiryBlock = model.InquiryBlock

// ViolationStatus 是認定（違規）的狀態。
type ViolationStatus = model.ViolationStatus

// RevokeReason 是撤銷措施或認定的原因。
type RevokeReason = model.RevokeReason

// EvidenceKind 是佐證屬於通報還是措施上的請求。
type EvidenceKind = model.EvidenceKind

// MeasureKind 是措施的種類（警告、停權、終止合作）。
type MeasureKind = model.MeasureKind

// MeasureBasis 是措施的依據（認定、風險類型、契約條款）。
type MeasureBasis = model.Basis

// RiskGround 是 RISK 措施的風險類型。
type RiskGround = model.RiskGround

// MeasureStatus 是儲存的措施狀態。
type MeasureStatus = model.MeasureStatus

// MeasureState 是措施此刻的狀態（讀取時推導）。
type MeasureState = model.MeasureState

// RequestKind 是措施上請求的種類。
type RequestKind = model.RequestKind

// RequestState 是請求的狀態。
type RequestState = model.RequestState

// RequestCloseCause 是請求以 CLOSED 結案的原因。
type RequestCloseCause = model.RequestCloseCause

// RemediationUnavailableReason 是賣家此刻不能請求解除的原因。
type RemediationUnavailableReason = model.RemediationUnavailableReason

// AppealUnavailableReason 是賣家此刻不能對措施提出申覆的原因。
type AppealUnavailableReason = model.AppealUnavailableReason

// AppealUnavailability 是賣家此刻不能申覆的一個原因與它給使用者看的一句話（Measure.AppealUnavailabilities）。
type AppealUnavailability = model.AppealUnavailability

// RemediationUnavailability 是賣家此刻不能請求解除的一個原因與它給使用者看的一句話（Measure.RemediationUnavailabilities）。
type RemediationUnavailability = model.RemediationUnavailability

// MeasureActionUnavailableReason 是 admin 此刻不能修改措施或撤銷的原因。
type MeasureActionUnavailableReason = model.MeasureActionUnavailableReason

// MeasureAction 是 admin 對措施做的、有前置條件的動作（改理由、延長、縮短、撤銷）。
type MeasureAction = model.MeasureAction

// TerminationGround 是終止合作的事由（賣家合作條款第 21.2、21.3 條）。
type TerminationGround = model.TerminationGround

// TerminationViolationRequirement 是建立某個事由的終止合作時，依據的需求（TerminationGround.ViolationRequirement）。
type TerminationViolationRequirement = model.TerminationViolationRequirement

// RevisionSystemCause 是系統寫入措施版本的原因。
type RevisionSystemCause = model.RevisionSystemCause

// LinkedIdentity 是兩個帳號相同的身分類別。
type LinkedIdentity = model.LinkedIdentity

const (
	SeverityLow                      = model.SeverityLow
	SeverityMedium                   = model.SeverityMedium
	SeverityHigh                     = model.SeverityHigh
	SeverityCritical                 = model.SeverityCritical
	OutcomeActionTaken               = model.OutcomeActionTaken
	OutcomeNoViolation               = model.OutcomeNoViolation
	OutcomeInsufficient              = model.OutcomeInsufficient
	OutcomeDuplicate                 = model.OutcomeDuplicate
	OutcomeAbusive                   = model.OutcomeAbusive
	DecisionActionTaken              = model.DecisionActionTaken
	DecisionNoAction                 = model.DecisionNoAction
	CaseClosedDecided                = model.CaseClosedDecided
	CaseClosedNoReports              = model.CaseClosedNoReports
	ReportStateAwaitingTriage        = model.ReportStateAwaitingTriage
	ReportStateInCase                = model.ReportStateInCase
	ReportStateResolved              = model.ReportStateResolved
	CaseStateOpen                    = model.CaseStateOpen
	CaseStateClosed                  = model.CaseStateClosed
	MatchedByPhone                   = model.MatchedByPhone
	MatchedByEmail                   = model.MatchedByEmail
	MatchedByBoth                    = model.MatchedByBoth
	ReportBlockResolved              = model.ReportBlockResolved
	ReportBlockAlreadyTriaged        = model.ReportBlockAlreadyTriaged
	ReportBlockNeedsCase             = model.ReportBlockNeedsCase
	ReportBlockNoActionBasis         = model.ReportBlockNoActionBasis
	ReportBlockNotInOpenCase         = model.ReportBlockNotInOpenCase
	ReportBlockFindingSource         = model.ReportBlockFindingSource
	ReportBlockLastReportWithMeasure = model.ReportBlockLastReportWithMeasure
	ReportBlockNegated               = model.ReportBlockNegated
	ReportBlockStoreClosed           = model.ReportBlockStoreClosed
	CaseBlockClosed                  = model.CaseBlockClosed
	CaseBlockNoActionBasis           = model.CaseBlockNoActionBasis
	CaseBlockUnrevokedMeasureExists  = model.CaseBlockUnrevokedMeasureExists
	CaseBlockReportActionTaken       = model.CaseBlockReportActionTaken
	CaseBlockStoreClosed             = model.CaseBlockStoreClosed
	CaseBlockNotFoodSafety           = model.CaseBlockNotFoodSafety
	CaseBlockNoReports               = model.CaseBlockNoReports
	CaseBlockReferralRecorded        = model.CaseBlockReferralRecorded
	CaseBlockReferralWaived          = model.CaseBlockReferralWaived
	CaseBlockNoReferral              = model.CaseBlockNoReferral
	CaseBlockInquiryOpen             = model.CaseBlockInquiryOpen
	CaseBlockStorePurged             = model.CaseBlockStorePurged
	InquiryKindRecords               = model.InquiryKindRecords
	InquiryKindExplanation           = model.InquiryKindExplanation
	InquiryChannelPhone              = model.InquiryChannelPhone
	InquiryChannelEmail              = model.InquiryChannelEmail
	InquiryChannelMessaging          = model.InquiryChannelMessaging
	InquiryChannelOther              = model.InquiryChannelOther
	InquiryWithdrawAdmin             = model.InquiryWithdrawAdmin
	InquiryWithdrawCaseClosed        = model.InquiryWithdrawCaseClosed
	InquiryStateOpen                 = model.InquiryStateOpen
	InquiryStateReplied              = model.InquiryStateReplied
	InquiryStateWithdrawn            = model.InquiryStateWithdrawn
	InquiryBlockWithdrawn            = model.InquiryBlockWithdrawn
	InquiryBlockReplied              = model.InquiryBlockReplied
	CaseActionDecide                 = model.CaseActionDecide
	CaseActionChangeSeverity         = model.CaseActionChangeSeverity
	CaseActionLinkMeasure            = model.CaseActionLinkMeasure
	CaseActionCreateMeasure          = model.CaseActionCreateMeasure
	CaseActionRecordReferral         = model.CaseActionRecordReferral
	CaseActionWaiveReferral          = model.CaseActionWaiveReferral
	CaseActionRecordReferralReply    = model.CaseActionRecordReferralReply
	CaseActionExport                 = model.CaseActionExport
	ReferralReferred                 = model.ReferralReferred
	ReferralWaived                   = model.ReferralWaived
	CaseActionCreateInquiry          = model.CaseActionCreateInquiry
	FDAHotline1919                   = model.FDAHotline1919
	FDAOnlineForm                    = model.FDAOnlineForm
	FDAFaxHealthDept                 = model.FDAFaxHealthDept
	FDAInPerson                      = model.FDAInPerson
	FDAOther                         = model.FDAOther
	ViolationStatusEnforced          = model.ViolationStatusEnforced
	ViolationStatusRevoked           = model.ViolationStatusRevoked
	RevokeAppealAccepted             = model.RevokeAppealAccepted
	RevokeAdminCorrection            = model.RevokeAdminCorrection
	RevokeExternalDecision           = model.RevokeExternalDecision
	RevokeOther                      = model.RevokeOther
	RevokeFindingRevoked             = model.RevokeFindingRevoked
	RevokeSourceRevoked              = model.RevokeSourceRevoked
	EvidenceReport                   = model.EvidenceReport
	EvidenceRequest                  = model.EvidenceRequest
	EvidenceInquiry                  = model.EvidenceInquiry
	MeasureWarning                   = model.MeasureWarning
	MeasureSuspension                = model.MeasureSuspension
	MeasureTermination               = model.MeasureTermination
	BasisViolation                   = model.BasisViolation
	BasisRisk                        = model.BasisRisk
	BasisContract                    = model.BasisContract
	GroundNotice                     = model.GroundNotice
	GroundMaterialBreach             = model.GroundMaterialBreach
	GroundCeasedBusiness             = model.GroundCeasedBusiness
	GroundDissolution                = model.GroundDissolution
	GroundBankruptcy                 = model.GroundBankruptcy
	GroundLicenseRevoked             = model.GroundLicenseRevoked
	GroundFoodSafetyIncident         = model.GroundFoodSafetyIncident
	GroundFraud                      = model.GroundFraud
	GroundPlatformSecurity           = model.GroundPlatformSecurity
	GroundOtherSerious               = model.GroundOtherSerious
	GroundLinkedAccount              = model.GroundLinkedAccount
	AppealReasonRevoked              = model.AppealReasonRevoked
	AppealReasonAlreadyAppealed      = model.AppealReasonAlreadyAppealed
	AppealReasonWindowEnded          = model.AppealReasonWindowEnded
	AppealReasonRequestPending       = model.AppealReasonRequestPending
	RevisionStoreClosed              = model.RevisionStoreClosed
	LinkedNationalID                 = model.LinkedNationalID
	LinkedUBN                        = model.LinkedUBN
	LinkedResponsiblePersonID        = model.LinkedResponsiblePersonID
	RiskTransaction                  = model.RiskTransaction
	RiskFoodSafety                   = model.RiskFoodSafety
	RiskFraud                        = model.RiskFraud
	RiskPersonalData                 = model.RiskPersonalData
	RiskInfringement                 = model.RiskInfringement
	RiskSecurity                     = model.RiskSecurity
	RiskFulfillment                  = model.RiskFulfillment
	RiskExternalOrder                = model.RiskExternalOrder
	MeasureStatusEnforced            = model.MeasureStatusEnforced
	MeasureStatusRevoked             = model.MeasureStatusRevoked
	StateIssued                      = model.StateIssued
	StateActive                      = model.StateActive
	StateEnded                       = model.StateEnded
	StateRevoked                     = model.StateRevoked
	StateScheduled                   = model.StateScheduled
	RequestKindAppeal                = model.RequestKindAppeal
	RequestKindRemediation           = model.RequestKindRemediation
	CloseCauseRevoked                = model.CloseCauseRevoked
	CloseCauseEnded                  = model.CloseCauseEnded
	RequestPending                   = model.RequestPending
	RequestAccepted                  = model.RequestAccepted
	RequestRejected                  = model.RequestRejected
	RequestClosed                    = model.RequestClosed
	ReasonRemediationPending         = model.ReasonRemediationPending
	ReasonAppealPending              = model.ReasonAppealPending
	ReasonCooldown                   = model.ReasonCooldown
	ReasonNotRisk                    = model.ReasonNotRisk
	ReasonNotActive                  = model.ReasonNotActive
	MeasureActionUpdateStatement     = model.MeasureActionUpdateStatement
	MeasureActionExtend              = model.MeasureActionExtend
	MeasureActionShorten             = model.MeasureActionShorten
	MeasureActionRevoke              = model.MeasureActionRevoke
	MeasureActionEnd                 = model.MeasureActionEnd
	// ReportBlockLastReportWithReferral 是案件唯一的成員而案件已記錄轉報：最後一筆通報不能移走。
	ReportBlockLastReportWithReferral = model.ReportBlockLastReportWithReferral
	// ContactRetention 是匿名檢舉人聯絡欄位的保留期限（從建立起算）；結果設定之後還要再留 ContactRetentionAfterOutcome。
	ContactRetention = model.ContactRetention
	// ContactRetentionAfterOutcome 是匿名檢舉人聯絡欄位在結果設定之後的保留期限。
	ContactRetentionAfterOutcome = model.ContactRetentionAfterOutcome
	// ContactRetentionCap 是匿名檢舉人聯絡欄位的絕對保留上限（從建立起算，不論有沒有結果）。
	ContactRetentionCap = model.ContactRetentionCap
	// MaxSupplements 是一筆通報最多的補充則數。
	MaxSupplements = model.MaxSupplements
	// ReviewWindow 是 RISK 措施複審期限的上限。
	ReviewWindow = model.ReviewWindow
	// NoticePeriod 是預告終止（條款第 21.2 條）的預告期間：30 日。
	NoticePeriod = model.NoticePeriod
	// AppealWindow 是終止合作的申覆期間：生效後 184 天。
	AppealWindow = model.AppealWindow
	// MaterialBreachCure 是重大違約（條款第 21.3 條第 1 款）的改善期：引用的認定至少存在這麼久。
	MaterialBreachCure = model.MaterialBreachCure
	// StoreRetireAfter 是一般關店之後的第一段清理（退場：公開內容、圖片、StoreRetired）：30 天。
	StoreRetireAfter = model.StoreRetireAfter
	// StoreRetention 是關店之後的第二段清理（註冊、身分與連線資料）：184 天（6 個月）。
	StoreRetention = model.StoreRetention
	// InquiryDefaultDue 是詢問沒有指定期限時，建立後的期限（3 天）。
	InquiryDefaultDue = model.InquiryDefaultDue
	// InquiryMinDue 與 InquiryMaxDue 是建立詢問時可以指定的期限範圍（距離建立）。
	InquiryMinDue = model.InquiryMinDue
	InquiryMaxDue = model.InquiryMaxDue
	// MaxEvidence 是一次通報、申覆或詢問回覆最多的佐證數。
	MaxEvidence = model.MaxEvidence
)

// SeverityValues 回傳全部嚴重度。
func SeverityValues() []Severity { return model.SeverityValues() }

// OutcomeValues 回傳全部通報結果（固定順序）。
func OutcomeValues() []Outcome { return model.OutcomeValues() }

// DecisionValues 回傳全部案件決定（固定順序）。
func DecisionValues() []Decision { return model.DecisionValues() }

// CaseCloseReasonValues 回傳全部案件結案原因。
func CaseCloseReasonValues() []CaseCloseReason { return model.CaseCloseReasonValues() }

// ReportStateValues 回傳全部通報處理狀態。
func ReportStateValues() []ReportState { return model.ReportStateValues() }

// CaseStateValues 回傳全部案件狀態。
func CaseStateValues() []CaseState { return model.CaseStateValues() }

// SupplementMatchValues 回傳全部補充的比對方式。
func SupplementMatchValues() []SupplementMatch { return model.SupplementMatchValues() }

// ReportBlockValues 回傳全部通報不能做動作的原因。
func ReportBlockValues() []ReportBlock { return model.ReportBlockValues() }

// CaseBlockValues 回傳全部案件不能做動作的原因。
func CaseBlockValues() []CaseBlock { return model.CaseBlockValues() }

// InquiryKindValues 回傳全部詢問種類。
func InquiryKindValues() []InquiryKind { return model.InquiryKindValues() }

// InquiryReplyChannelValues 回傳全部詢問回覆的管道。
func InquiryReplyChannelValues() []InquiryReplyChannel { return model.InquiryReplyChannelValues() }

// InquiryWithdrawCauseValues 回傳全部詢問撤回的原因。
func InquiryWithdrawCauseValues() []InquiryWithdrawCause { return model.InquiryWithdrawCauseValues() }

// InquiryStateValues 回傳全部詢問狀態。
func InquiryStateValues() []InquiryState { return model.InquiryStateValues() }

// InquiryBlockValues 回傳全部詢問不能做動作的原因。
func InquiryBlockValues() []InquiryBlock { return model.InquiryBlockValues() }

// FDAChannelValues 回傳全部轉報管道。
func FDAChannelValues() []FDAChannel { return model.FDAChannelValues() }

// ViolationStatusValues 回傳全部違規狀態。
func ViolationStatusValues() []ViolationStatus { return model.ViolationStatusValues() }

// RevokeReasonValues 回傳全部撤銷原因。
func RevokeReasonValues() []RevokeReason { return model.RevokeReasonValues() }

// ManualRevokeReasonValues 回傳管理員撤銷措施時可以選的原因（契約的 AdminRevokeReason）。
func ManualRevokeReasonValues() []RevokeReason { return model.ManualRevokeReasonValues() }

// MeasureKindValues 回傳全部措施種類。
func MeasureKindValues() []MeasureKind { return model.MeasureKindValues() }

// BasisValues 回傳全部措施依據。
func BasisValues() []MeasureBasis { return model.BasisValues() }

// RiskGroundValues 回傳全部風險類型。
func RiskGroundValues() []RiskGround { return model.RiskGroundValues() }

// MeasureStateValues 回傳全部措施狀態。
func MeasureStateValues() []MeasureState { return model.MeasureStateValues() }

// RequestKindValues 回傳全部請求種類。
func RequestKindValues() []RequestKind { return model.RequestKindValues() }

// RequestStateValues 回傳全部請求狀態。
func RequestStateValues() []RequestState { return model.RequestStateValues() }

// TerminationGroundValues 回傳全部終止事由。
func TerminationGroundValues() []TerminationGround { return model.TerminationGroundValues() }

// TerminationViolationRequirementValues 回傳全部依據需求值。
func TerminationViolationRequirementValues() []TerminationViolationRequirement {
	return model.TerminationViolationRequirementValues()
}

// AppealUnavailableReasonValues 回傳全部不能申覆的原因。
func AppealUnavailableReasonValues() []AppealUnavailableReason {
	return model.AppealUnavailableReasonValues()
}

// MeasureActionUnavailableReasonValues 回傳全部 admin 不能修改措施或撤銷的原因。
func MeasureActionUnavailableReasonValues() []MeasureActionUnavailableReason {
	return model.MeasureActionUnavailableReasonValues()
}

// MeasureActionValues 回傳全部有前置條件的動作。
func MeasureActionValues() []MeasureAction { return model.MeasureActionValues() }

// ActionUnavailableText 是 admin 不能對措施做 action 的原因給使用者看的一句話；API 的可用性訊息與方法被拒絕時的錯誤文字都由它產生。
func ActionUnavailableText(reason MeasureActionUnavailableReason, action MeasureAction) uitext.Message {
	return model.ActionUnavailableText(reason, action)
}

// RevisionSystemCauseValues 回傳全部系統寫入措施版本的原因。
func RevisionSystemCauseValues() []RevisionSystemCause { return model.RevisionSystemCauseValues() }

// LinkedIdentityValues 回傳全部命中的身分類別。
func LinkedIdentityValues() []LinkedIdentity { return model.LinkedIdentityValues() }

// RemediationUnavailableReasonValues 回傳全部不能請求解除的原因。
func RemediationUnavailableReasonValues() []RemediationUnavailableReason {
	return model.RemediationUnavailableReasonValues()
}

// RequestCloseCauseValues 回傳全部結案原因。
func RequestCloseCauseValues() []RequestCloseCause { return model.RequestCloseCauseValues() }

type (
	// Evidence 是一張佐證（機敏檔）。
	Evidence = model.Evidence
	// ReadEvidenceInput 是後台讀一張佐證的輸入。
	ReadEvidenceInput = model.ReadEvidenceInput
	// FileContent 是一份機敏檔的內容。
	FileContent = model.FileContent
	// ReporterContact 是匿名檢舉人的聯絡方式。
	ReporterContact = model.ReporterContact
	// FDAReferral 是案件轉報主管機關的紀錄。
	FDAReferral = model.FDAReferral
	// FDAReferralReply 是主管機關對轉報的一則回覆。
	FDAReferralReply = model.FDAReferralReply
	// ReferralWaiver 是標記不轉報的紀錄。
	ReferralWaiver = model.ReferralWaiver
	// CaseReferral 是案件唯一的轉報紀錄（已轉報或不轉報）。
	CaseReferral = model.CaseReferral
	// ReferralOutcome 是轉報紀錄的種類。
	ReferralOutcome = model.ReferralOutcome
	// CaseInquiry 是案件上對賣家的一筆詢問。
	CaseInquiry = model.CaseInquiry
	// InquiryReply 是管理員記下的賣家回覆（系統外收到）。
	InquiryReply = model.InquiryReply
	// InquiryWithdrawal 是詢問的撤回。
	InquiryWithdrawal = model.InquiryWithdrawal
	// InquiryAvailability 是詢問每個動作不能做的原因。
	InquiryAvailability = model.InquiryAvailability
	// AdminInquiry 是後台看到的一筆詢問（含此刻可以做的動作）。
	AdminInquiry = model.AdminInquiry
	// CreateCaseInquiryInput 是發詢問的輸入。
	CreateCaseInquiryInput = model.CreateCaseInquiryInput
	// CreateCaseInquiryResult 是發詢問的結果。
	CreateCaseInquiryResult = model.CreateCaseInquiryResult
	// RecordCaseInquiryReplyInput 是記下詢問回覆的輸入。
	RecordCaseInquiryReplyInput = model.RecordCaseInquiryReplyInput
	// RecordCaseInquiryReplyResult 是記下詢問回覆的結果。
	RecordCaseInquiryReplyResult = model.RecordCaseInquiryReplyResult
	// WithdrawCaseInquiryInput 是撤回詢問的輸入。
	WithdrawCaseInquiryInput = model.WithdrawCaseInquiryInput
	// WithdrawCaseInquiryResult 是撤回詢問的結果。
	WithdrawCaseInquiryResult = model.WithdrawCaseInquiryResult
	// InquiryCursor 是詢問列表的 keyset 位置。
	InquiryCursor = model.InquiryCursor
	// CaseInquiryListInput 是案件詢問清單的輸入。
	CaseInquiryListInput = model.CaseInquiryListInput
	// AdminInquiryPage 是一頁詢問。
	AdminInquiryPage = model.AdminInquiryPage
	// Report 是一次通報（一位檢舉人的一次送出）。
	Report = model.Report
	// Supplement 是併入原通報的一則補充。
	Supplement = model.Supplement
	// Case 是案件：同一家店、同一個平台判定類別，同時最多一個進行中的案件。
	Case = model.Case
	// ReportMove 是一次把通報移到其他案件的紀錄。
	ReportMove = model.ReportMove
	// ReportFacts 是判斷通報能做什麼需要的事實。
	ReportFacts = model.ReportFacts
	// CaseFacts 是判斷案件能做什麼需要的事實。
	CaseFacts = model.CaseFacts
	// ReportAvailability 是通報每個動作不能做的原因。
	ReportAvailability = model.ReportAvailability
	// CaseAvailability 是案件每個動作不能做的原因。
	CaseAvailability = model.CaseAvailability
	// Deadline 是一個有期限的義務：從 Start 起算，到 Due 到期；資源上是 *Deadline，nil 表示義務不在。規則只在 SQL 的審閱政策函式（db/objects.sql）。
	Deadline = model.Deadline
	// AdminReport 是後台看到的一筆通報（含此刻可以做的動作與分案、移動的去處）。
	AdminReport = model.AdminReport
	// TriageTarget 是分案或移動到某個平台判定類別時這家店此刻的去處（加入進行中的案件，或開新案件）。
	TriageTarget = model.TriageTarget
	// CaseTarget 是分案或移動的明確意圖（加入進行中的案件，或開新案件）。
	CaseTarget = model.CaseTarget
	// JoinCase 是加入一個進行中的案件（讀到的案件 ID 與版本）。
	JoinCase = model.JoinCase
	// ContactErasure 是一次聯絡方式清理的結果。
	ContactErasure = model.ContactErasure
	// AdminCase 是後台看到的一個案件（含此刻可以做的動作）。
	AdminCase = model.AdminCase
	// SubmitReportInput 是提出通報（檢舉）的輸入。
	SubmitReportInput = model.SubmitReportInput
	// SubmitReportResult 是提出通報的結果。
	SubmitReportResult = model.SubmitReportResult
	// TriageReportInput 是分案的輸入。
	TriageReportInput = model.TriageReportInput
	// MarkSupplementsSeenInput 是記下看過補充的輸入。
	MarkSupplementsSeenInput = model.MarkSupplementsSeenInput
	// ResolveReportInput 是設定結果的輸入。
	ResolveReportInput = model.ResolveReportInput
	// MoveReportInput 是移到其他案件的輸入。
	MoveReportInput = model.MoveReportInput
	// MoveReportResult 是移動的結果。
	MoveReportResult = model.MoveReportResult
	// DecideCaseInput 是決定（結案）的輸入。
	DecideCaseInput = model.DecideCaseInput
	// ChangeCaseSeverityInput 是改嚴重度的輸入。
	ChangeCaseSeverityInput = model.ChangeCaseSeverityInput
	// RecordFDAReferralInput 是記下轉報的輸入。
	RecordFDAReferralInput = model.RecordFDAReferralInput
	// RecordFDAReferralResult 是記下轉報的結果。
	RecordFDAReferralResult = model.RecordFDAReferralResult
	// WaiveFDAReferralInput 是標記不轉報的輸入。
	WaiveFDAReferralInput = model.WaiveFDAReferralInput
	// WaiveFDAReferralResult 是標記不轉報的結果。
	WaiveFDAReferralResult = model.WaiveFDAReferralResult
	// RecordFDAReferralReplyInput 是記下主管機關回覆的輸入。
	RecordFDAReferralReplyInput = model.RecordFDAReferralReplyInput
	// ReportCursor 是通報列表的 keyset 位置。
	ReportCursor = model.ReportCursor
	// ReportListInput 是 admin 通報清單的輸入。
	ReportListInput = model.ReportListInput
	// BuyerReportListInput 是檢舉人自己的通報列表的輸入。
	BuyerReportListInput = model.BuyerReportListInput
	// ReportPage 是一頁通報（檢舉人自己的）。
	ReportPage = model.ReportPage
	// AdminReportPage 是一頁後台的通報。
	AdminReportPage = model.AdminReportPage
	// CaseCursor 是案件列表的 keyset 位置。
	CaseCursor = model.CaseCursor
	// CaseListInput 是 admin 案件清單的輸入。
	CaseListInput = model.CaseListInput
	// AdminCasePage 是一頁案件。
	AdminCasePage = model.AdminCasePage
	// FDAExport 是轉報主管機關需要的資料。
	FDAExport = model.FDAExport
	// MeasureRequest 是措施上賣家的請求（申覆）。
	MeasureRequest = model.MeasureRequest
	// Violation 是一筆認定（違規）。
	Violation = model.Violation
	// ViolationCursor 是認定列表的 keyset 位置。
	ViolationCursor = model.ViolationCursor
	// ViolationListInput 是 admin 認定列表的輸入。
	ViolationListInput = model.ViolationListInput
	// ViolationPage 是一頁認定。
	ViolationPage = model.ViolationPage
	// Measure 是一個措施。
	Measure = model.Measure
	// MeasureRevision 是措施的一個版本（賣家看得到的完整內容）。
	MeasureRevision = model.MeasureRevision
	// GovernanceNote 是措施或案件的一則內部註記。
	GovernanceNote = model.GovernanceNote
	// SeverityChange 是案件嚴重度的一次修改（改前與改後），記在修改時寫的那則註記上。
	SeverityChange = model.SeverityChange
	// NoteCursor 是內部註記列表的 keyset 位置。
	NoteCursor = model.NoteCursor
	// MeasureNoteListInput 是措施內部註記列表的輸入。
	MeasureNoteListInput = model.MeasureNoteListInput
	// CaseNoteListInput 是案件內部註記列表的輸入。
	CaseNoteListInput = model.CaseNoteListInput
	// NotePage 是一頁內部註記。
	NotePage = model.NotePage
	// CreateMeasureNoteInput 是新增措施內部註記的輸入。
	CreateMeasureNoteInput = model.CreateMeasureNoteInput
	// CreateCaseNoteInput 是新增案件內部註記的輸入。
	CreateCaseNoteInput = model.CreateCaseNoteInput
	// CreateMeasureInput 是建立措施的輸入。
	CreateMeasureInput = model.CreateMeasureInput
	// CreateMeasureResult 是建立措施的結果。
	CreateMeasureResult = model.CreateMeasureResult
	// ViolationBasis 是 VIOLATION 依據：新的認定與引用的既有認定。
	ViolationBasis = model.ViolationBasis
	// NewFinding 是要新建的認定。
	NewFinding = model.NewFinding
	// RiskBasis 是 RISK 依據。
	RiskBasis = model.RiskBasis
	// TerminationBasis 是終止合作的事由與依據。
	TerminationBasis = model.TerminationBasis
	// TerminationEffect 是建立終止合作之後措施何時生效（立即或排程）。
	TerminationEffect = model.TerminationEffect
	// LinkedAccount 是措施詳情上相同身分的其他帳號。
	LinkedAccount = model.LinkedAccount
	// UpdateMeasureInput 是措施賣家可見內容的部分更新。
	UpdateMeasureInput = model.UpdateMeasureInput
	// EndMeasureInput 是結束 RISK 措施的輸入。
	EndMeasureInput = model.EndMeasureInput
	// RevokeMeasureInput 是撤銷措施的輸入。
	RevokeMeasureInput = model.RevokeMeasureInput
	// ScheduleReviewInput 是重排複審期限的輸入。
	ScheduleReviewInput = model.ScheduleReviewInput
	// SubmitAppealInput 是提出申覆的輸入。
	SubmitAppealInput = model.SubmitAppealInput
	// DecideAppealInput 是裁定申覆的輸入。
	DecideAppealInput = model.DecideAppealInput
	// RequestRemediationInput 是請求解除的輸入。
	RequestRemediationInput = model.RequestRemediationInput
	// DecideRemediationInput 是裁定解除請求的輸入。
	DecideRemediationInput = model.DecideRemediationInput
	// MeasureCursor 是措施列表的 keyset 位置。
	MeasureCursor = model.MeasureCursor
	// AdminMeasureListInput 是 admin 措施列表的輸入。
	AdminMeasureListInput = model.AdminMeasureListInput
	// AccountMeasureListInput 是賣家自己的措施列表的輸入。
	AccountMeasureListInput = model.AccountMeasureListInput
	// MeasurePage 是一頁措施。
	MeasurePage = model.MeasurePage
	// StoreMeasures 是店家與帳號此刻生效中的停權措施的同一次讀取。
	StoreMeasures = model.StoreMeasures
	// EffectiveMeasure 是帳號此刻生效中的一個停權措施。
	EffectiveMeasure = model.EffectiveMeasure
)

// SubmitReport 提出檢舉（登入的買家或匿名者）；必須在 transaction 外呼叫（佐證先轉正）。去重的期間內已有同一個檢舉人、還沒有結果的通報時，這次的內容補充到它
// （Supplemented；每筆最多 MaxSupplements 則）。通報的對象只能是曾經公開過的店（StoreCards 的規則：檢舉人看得到的店，已關閉的也算）；從沒公開過的與不存在的是同一個
// NotFound，所以公開的匿名入口不能拿來探測從沒公開過的店家 ID（store_id 是連號）。登入的呼叫者（AccountID）不能檢舉自己的店。
// 這是 A7-D5′ 之後 moderation 會呼叫的入口：依單體規則 3，它不開 InReadSnapshot；在呼叫方 transaction 內建立檢舉要另給加入呼叫方 transaction 的入口（SubmitReportInTx），
// 在 A7 的刀和它的消費者一起加。
func (m *Module) SubmitReport(ctx context.Context, in SubmitReportInput) (SubmitReportResult, error) {
	if in.StoreID > 0 {
		cards, err := m.StoreCards(ctx, []int64{in.StoreID})
		if err != nil {
			return SubmitReportResult{}, err
		}
		if len(cards) == 0 {
			return SubmitReportResult{}, apperr.NotFound("找不到這家店")
		}
	}
	return m.governance.Submit(ctx, in)
}

// TriageReport 分案：判定平台判定類別並掛到這家店那個類別進行中的案件（沒有就開一個，必須給嚴重度）。
func (m *Module) TriageReport(ctx context.Context, in TriageReportInput) (AdminReport, error) {
	return m.governance.Triage(ctx, in)
}

// MarkReportSupplementsSeen 記下管理員看過通報此刻的全部補充；不讓通報的版本前進，冪等。
func (m *Module) MarkReportSupplementsSeen(ctx context.Context, in MarkSupplementsSeenInput) (AdminReport, error) {
	return m.governance.MarkSupplementsSeen(ctx, in)
}

// ResolveReport 設定通報的結果；結果設定後不能修改。
func (m *Module) ResolveReport(ctx context.Context, in ResolveReportInput) (AdminReport, error) {
	return m.governance.Resolve(ctx, in)
}

// MoveReport 把通報移到其他案件（改平台判定類別）；移走最後一筆時原案件以沒有通報關閉。
func (m *Module) MoveReport(ctx context.Context, in MoveReportInput) (MoveReportResult, error) {
	return m.governance.Move(ctx, in)
}

// DecideCase 決定案件（採取措施或不採取措施），決定即結案；沒有結果的通報帶入預設結果。
func (m *Module) DecideCase(ctx context.Context, in DecideCaseInput) (AdminCase, error) {
	return m.governance.Decide(ctx, in)
}

// ChangeCaseSeverity 修改案件的嚴重度並寫一則內部註記（原因，帶改前與改後）。
func (m *Module) ChangeCaseSeverity(ctx context.Context, in ChangeCaseSeverityInput) (AdminCase, error) {
	return m.governance.ChangeSeverity(ctx, in)
}

// RecordFDAReferral 記下食安案件轉報主管機關；已記過時回傳原本的紀錄。
func (m *Module) RecordFDAReferral(ctx context.Context, in RecordFDAReferralInput) (RecordFDAReferralResult, error) {
	return m.governance.RecordFDAReferral(ctx, in)
}

// WaiveFDAReferral 標記食安案件不轉報（附原因）；已標記過時回傳原本的紀錄，已記錄轉報是錯誤。
func (m *Module) WaiveFDAReferral(ctx context.Context, in WaiveFDAReferralInput) (WaiveFDAReferralResult, error) {
	return m.governance.WaiveReferral(ctx, in)
}

// RecordFDAReferralReply 記下主管機關對轉報的回覆（只新增），並依輸入更新或清掉追蹤日期。只限已記錄轉報的案件。
func (m *Module) RecordFDAReferralReply(ctx context.Context, in RecordFDAReferralReplyInput) (AdminCase, error) {
	return m.governance.RecordReferralReply(ctx, in)
}

// CreateCaseInquiry 對案件的賣家發一筆詢問（提供紀錄或請說明，有期限）；不發事件（沒有消費者）。同一位管理員同一個 request_id 的重送回傳原本那一筆。
func (m *Module) CreateCaseInquiry(ctx context.Context, in CreateCaseInquiryInput) (CreateCaseInquiryResult, error) {
	return m.governance.CreateInquiry(ctx, in)
}

// RecordCaseInquiryReply 記下系統外收到的賣家回覆並結束詢問；必須在 transaction 外呼叫（附件先轉正）。已記過時回傳原本那一則。
func (m *Module) RecordCaseInquiryReply(ctx context.Context, in RecordCaseInquiryReplyInput) (RecordCaseInquiryReplyResult, error) {
	return m.governance.RecordInquiryReply(ctx, in)
}

// WithdrawCaseInquiry 撤回還沒回覆的詢問；已撤回時回傳原本的紀錄。
func (m *Module) WithdrawCaseInquiry(ctx context.Context, in WithdrawCaseInquiryInput) (WithdrawCaseInquiryResult, error) {
	return m.governance.WithdrawInquiry(ctx, in)
}

// CaseInquiries 是案件的詢問清單，舊到新。只由 admin 的 handler 呼叫，自己開讀取快照（不是跨模組入口）。
func (m *Module) CaseInquiries(ctx context.Context, in CaseInquiryListInput) (AdminInquiryPage, error) {
	return m.governance.Inquiries(ctx, in)
}

// Report 讀取一筆通報的後台檢視：補充、佐證與此刻可以做的動作。只由 admin 的 handler 呼叫，自己開讀取快照（不是跨模組入口）。
func (m *Module) Report(ctx context.Context, reportID uuid.UUID) (AdminReport, error) {
	return m.governance.Report(ctx, reportID)
}

// Case 讀取一個案件的後台檢視：移動紀錄、轉報紀錄與此刻可以做的動作。只由 admin 的 handler 呼叫，自己開讀取快照。
func (m *Module) Case(ctx context.Context, caseID uuid.UUID) (AdminCase, error) {
	return m.governance.Case(ctx, caseID)
}

// FDAExport 回傳食安案件轉報主管機關需要的資料（案件、結果沒有被否定的通報與以 store_id 讀到的店家）。
func (m *Module) FDAExport(ctx context.Context, caseID uuid.UUID) (FDAExport, error) {
	return m.governance.FDAExport(ctx, caseID)
}

// AdminReports 是 admin 的通報清單。
func (m *Module) AdminReports(ctx context.Context, in ReportListInput) (AdminReportPage, error) {
	return m.governance.AdminReports(ctx, in)
}

// AdminCases 是 admin 的案件清單。
func (m *Module) AdminCases(ctx context.Context, in CaseListInput) (AdminCasePage, error) {
	return m.governance.AdminCases(ctx, in)
}

// BuyerReports 是登入的檢舉人自己的通報。
func (m *Module) BuyerReports(ctx context.Context, in BuyerReportListInput) (ReportPage, error) {
	return m.governance.BuyerReports(ctx, in)
}

// EraseExpiredReportContacts 清掉清除時間已到的匿名檢舉人聯絡欄位（有結果的：建立滿 90 天且結果設定滿 30 天；不論有沒有結果：建立滿 365 天），
// 回傳清掉的筆數與其中只靠 365 天上限清的筆數；它是 seller.erase_report_contacts job 的內容。
func (m *Module) EraseExpiredReportContacts(ctx context.Context) (ContactErasure, error) {
	return m.governance.EraseExpiredContacts(ctx)
}

// CreateCaseNote 新增案件的一則內部註記（只新增、不帶版本、不改案件的任何 etag）；案件可以是已結案的。
func (m *Module) CreateCaseNote(ctx context.Context, in CreateCaseNoteInput) (GovernanceNote, error) {
	return m.governance.CreateCaseNote(ctx, in)
}

// CaseNotes 是案件的內部註記，舊到新分頁；案件不存在回 NotFound。
func (m *Module) CaseNotes(ctx context.Context, in CaseNoteListInput) (NotePage, error) {
	return m.governance.CaseNotes(ctx, in)
}

// CreateMeasure 建立措施（警告或停權）；停權時同一個 transaction 快照身分並重算停權狀態。帶 RequestID 的重送回傳原本的措施。
func (m *Module) CreateMeasure(ctx context.Context, in CreateMeasureInput) (CreateMeasureResult, error) {
	return m.measure.Create(ctx, in)
}

// CreateMeasureNote 新增措施的一則內部註記（只新增、不帶版本、不改措施的任何 etag）；措施可以是已撤銷的。
func (m *Module) CreateMeasureNote(ctx context.Context, in CreateMeasureNoteInput) (GovernanceNote, error) {
	return m.measure.CreateNote(ctx, in)
}

// MeasureNotes 是措施的內部註記，舊到新分頁；措施不存在回 NotFound。
func (m *Module) MeasureNotes(ctx context.Context, in MeasureNoteListInput) (NotePage, error) {
	return m.measure.Notes(ctx, in)
}

// CurrentMeasure 讀一筆措施的目前內容（含依據與請求，不含全部版本與佐證），給需要合併基準的呼叫方；admin 的完整讀取是 Measure。
func (m *Module) CurrentMeasure(ctx context.Context, measureID uuid.UUID) (Measure, error) {
	return m.measure.Current(ctx, measureID)
}

// UpdateMeasure 部分更新措施賣家可見的內容（以版本做樂觀鎖），產生新的一版；改結束時間會重算停權狀態。
func (m *Module) UpdateMeasure(ctx context.Context, in UpdateMeasureInput) (Measure, error) {
	return m.measure.Update(ctx, in)
}

// EndMeasure 結束 RISK 措施（風險已排除）。
func (m *Module) EndMeasure(ctx context.Context, in EndMeasureInput) (Measure, error) {
	return m.measure.End(ctx, in)
}

// RevokeMeasure admin 撤銷措施並解除它的效果；RevokeFindings 時一併撤銷自己建立的認定並連帶撤銷引用它們的其他措施。
func (m *Module) RevokeMeasure(ctx context.Context, in RevokeMeasureInput) (Measure, error) {
	return m.measure.Revoke(ctx, in)
}

// ScheduleMeasureReview 重排 RISK 措施的複審期限並寫一則內部註記。
func (m *Module) ScheduleMeasureReview(ctx context.Context, in ScheduleReviewInput) (Measure, error) {
	return m.measure.ScheduleReview(ctx, in)
}

// SubmitAppeal 帳號對自己的措施提出申覆；資格是帳號擁有這筆措施，不要求賣家角色、店家或任何能力（所有店家都已關閉的帳號
// 照樣受處分）。必須在 transaction 外呼叫。
func (m *Module) SubmitAppeal(ctx context.Context, in SubmitAppealInput) (Measure, error) {
	return m.measure.SubmitAppeal(ctx, in)
}

// DecideAppeal admin 裁定申覆；接受時同一個 transaction 撤銷措施並解除效果。
func (m *Module) DecideAppeal(ctx context.Context, in DecideAppealInput) (Measure, error) {
	return m.measure.DecideAppeal(ctx, in)
}

// RequestRemediation 帳號對自己依據風險類型、生效中的措施請求解除（條款第 17.4 條）；資格與限制同 SubmitAppeal。必須在
// transaction 外呼叫。
func (m *Module) RequestRemediation(ctx context.Context, in RequestRemediationInput) (Measure, error) {
	return m.measure.RequestRemediation(ctx, in)
}

// DecideRemediation admin 裁定待處理的解除請求；接受時同一個 transaction 結束措施（不是撤銷）。
func (m *Module) DecideRemediation(ctx context.Context, in DecideRemediationInput) (Measure, error) {
	return m.measure.DecideRemediation(ctx, in)
}

// Measure 讀取一筆措施（admin）。
func (m *Module) Measure(ctx context.Context, measureID uuid.UUID) (Measure, error) {
	return m.measure.Measure(ctx, measureID)
}

// AccountMeasure 讀取帳號自己的一筆措施；不屬於這個帳號時回 NotFound。
func (m *Module) AccountMeasure(ctx context.Context, accountID int64, measureID uuid.UUID) (Measure, error) {
	return m.measure.AccountMeasure(ctx, accountID, measureID)
}

// AdminMeasures 是 admin 的措施列表。
func (m *Module) AdminMeasures(ctx context.Context, in AdminMeasureListInput) (MeasurePage, error) {
	return m.measure.AdminMeasures(ctx, in)
}

// AccountMeasures 是賣家自己（帳號所有 stint）的措施列表。
func (m *Module) AccountMeasures(ctx context.Context, in AccountMeasureListInput) (MeasurePage, error) {
	return m.measure.AccountMeasures(ctx, in)
}

// StoreWithActiveMeasures 讀取指定的 stint（含已關閉的）與帳號此刻生效中的停權措施（同一個快照）；後台的店家詳情用它。
func (m *Module) StoreWithActiveMeasures(ctx context.Context, storeID int64) (StoreMeasures, error) {
	return m.measure.StoreWithActive(ctx, storeID)
}

// Violation 讀取一筆認定（admin，只讀）。
func (m *Module) Violation(ctx context.Context, violationID uuid.UUID) (Violation, error) {
	return m.measure.Violation(ctx, violationID)
}

// AdminViolations 是 admin 的認定列表（只讀）。
func (m *Module) AdminViolations(ctx context.Context, in ViolationListInput) (ViolationPage, error) {
	return m.measure.Violations(ctx, in)
}

// ReadEvidence 讀一張通報或措施上請求的佐證（後台）：先寫讀取紀錄、commit 之後才向 media 讀內容。
func (m *Module) ReadEvidence(ctx context.Context, in ReadEvidenceInput) (FileContent, error) {
	return m.governance.ReadEvidence(ctx, in)
}
