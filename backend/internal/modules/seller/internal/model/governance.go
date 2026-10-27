package model

import (
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Severity 是嚴重度：案件在開案時決定（之後只經 ChangeCaseSeverity 修改），認定在建立時決定（可能被升級規則提高）。
type Severity string

const (
	// SeverityLow 低。
	SeverityLow Severity = "LOW"
	// SeverityMedium 中；90 天內第 3 次升為 HIGH。
	SeverityMedium Severity = "MEDIUM"
	// SeverityHigh 高。
	SeverityHigh Severity = "HIGH"
	// SeverityCritical 緊急。
	SeverityCritical Severity = "CRITICAL"
)

// SeverityValues 回傳全部嚴重度，與 DB 的 cases_severity_check、violations_severity_check 一致；由低到高。
func SeverityValues() []Severity {
	return []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
}

// Rank 是嚴重度由低到高的名次（LOW 0、MEDIUM 1、HIGH 2、CRITICAL 3）；不認得的值是 -1。
func (s Severity) Rank() int { return slices.Index(SeverityValues(), s) }

// FDAChannel 是食安事件轉報主管機關的管道（手動轉報，沒有 API）。
type FDAChannel string

const (
	// FDAHotline1919 1919 食安專線。
	FDAHotline1919 FDAChannel = "HOTLINE_1919"
	// FDAOnlineForm 線上表單。
	FDAOnlineForm FDAChannel = "ONLINE_FORM"
	// FDAFaxHealthDept 傳真衛生局。
	FDAFaxHealthDept FDAChannel = "FAX_HEALTH_DEPT"
	// FDAInPerson 親自送件。
	FDAInPerson FDAChannel = "IN_PERSON"
	// FDAOther 其他管道。
	FDAOther FDAChannel = "OTHER"
)

// FDAChannelValues 回傳全部轉報管道，與 DB 的 case_referrals_channel_check 一致。
func FDAChannelValues() []FDAChannel {
	return []FDAChannel{FDAHotline1919, FDAOnlineForm, FDAFaxHealthDept, FDAInPerson, FDAOther}
}

// ViolationStatus 是認定（違規）的狀態；措施是否仍生效看措施，不在認定上。
type ViolationStatus string

const (
	// ViolationStatusEnforced 生效中的認定。
	ViolationStatusEnforced ViolationStatus = "ENFORCED"
	// ViolationStatusRevoked 已撤銷（admin 撤銷措施並要求撤銷依據，或接受申覆）。
	ViolationStatusRevoked ViolationStatus = "REVOKED"
)

// ViolationStatusValues 回傳全部違規狀態，與 DB 的 violations_status_check 一致。
func ViolationStatusValues() []ViolationStatus {
	return []ViolationStatus{ViolationStatusEnforced, ViolationStatusRevoked}
}

// RevokeReason 是撤銷措施或認定的原因。認定只會有前四個（連帶撤銷不撤銷認定）。
type RevokeReason string

const (
	// RevokeAppealAccepted 接受申覆；只由裁定申覆寫入。
	RevokeAppealAccepted RevokeReason = "APPEAL_ACCEPTED"
	// RevokeAdminCorrection 平台更正誤判。
	RevokeAdminCorrection RevokeReason = "ADMIN_CORRECTION"
	// RevokeExternalDecision 主管機關或法院的決定。
	RevokeExternalDecision RevokeReason = "EXTERNAL_DECISION"
	// RevokeOther 其他。
	RevokeOther RevokeReason = "OTHER"
	// RevokeFindingRevoked 措施引用的認定被撤銷，措施連帶撤銷；只由連帶撤銷寫入。
	RevokeFindingRevoked RevokeReason = "FINDING_REVOKED"
	// RevokeSourceRevoked 關聯帳號的終止所依據的來源終止被撤銷，措施連帶撤銷；只由連帶撤銷寫入。
	RevokeSourceRevoked RevokeReason = "SOURCE_REVOKED"
)

// RevokeReasonValues 回傳全部撤銷原因，與 DB 的 measures_revoked_reason_check 一致。
func RevokeReasonValues() []RevokeReason {
	return []RevokeReason{RevokeAppealAccepted, RevokeAdminCorrection, RevokeExternalDecision, RevokeOther, RevokeFindingRevoked, RevokeSourceRevoked}
}

// ManualRevokeReasonValues 回傳管理員撤銷措施時可以選的原因：接受申覆只由裁定寫入，連帶撤銷只由認定或來源終止的撤銷寫入。
// 契約的 AdminRevokeReason 與它一致（internal/httpapi/seller 的 EnumMismatches 測試）。
func ManualRevokeReasonValues() []RevokeReason {
	return []RevokeReason{RevokeAdminCorrection, RevokeExternalDecision, RevokeOther}
}

// ContactRetention 是匿名檢舉人聯絡欄位的保留期限（設計書 32 的 90 天），從建立時間起算；有結果的，清除時間是 greatest(建立 + ContactRetention, 結果 + ContactRetentionAfterOutcome)；
// 不論有沒有結果，建立滿 ContactRetentionCap 一律清除。
const ContactRetention = 90 * 24 * time.Hour

// ContactRetentionCap 是匿名檢舉人聯絡欄位的絕對保留上限（Will 2026-10-18：GDPR 5(1)(e) 的儲存限制，保留要有期限）：從建立時間起算 365 天，不論有沒有結果、案件有沒有決定。
// 清理 job 記下被這條上限強制清除的筆數（沒有結果的匿名通報拖到這裡，代表處理流程卡住了）。
const ContactRetentionCap = 365 * 24 * time.Hour

// EscalationWindow 與 EscalationCount 是升級規則：帳號在 EscalationWindow 內第 EscalationCount 次 MEDIUM 升為 HIGH。
const (
	EscalationWindow = 90 * 24 * time.Hour
	EscalationCount  = 3
)

// MaxEvidence 是一次通報、申覆或詢問回覆最多的佐證圖片數。
const MaxEvidence = 5

// Evidence 是一張佐證：機敏檔（Will 2026-10-04，8g），只經後台的擁有者端點讀（ReadEvidence）。ObjectKey 給
// media.ReadSecure，AssetID 給引用的收斂與讀取紀錄；兩者都不對外。
type Evidence struct {
	AssetID, ObjectKey string
	// Position 與 SupplementID 只在讀取通報的佐證時有值：位置（連號，從 0 起）與帶來這張佐證的補充（原通報的佐證是 nil）。
	Position     int
	SupplementID *uuid.UUID
}

// EvidenceKind 是佐證屬於哪一種紀錄：通報（以通報 ID 定址）、措施上的請求（申覆，以請求 ID 定址）或詢問的回覆（以詢問 ID 定址）。
type EvidenceKind string

const (
	// EvidenceReport 通報的佐證。
	EvidenceReport EvidenceKind = "REPORT"
	// EvidenceRequest 措施上請求的佐證。
	EvidenceRequest EvidenceKind = "REQUEST"
	// EvidenceInquiry 詢問回覆的附件。
	EvidenceInquiry EvidenceKind = "INQUIRY"
)

// ReadEvidenceInput 是後台讀一張佐證的輸入：RefID 是通報 ID（EvidenceReport）、請求 ID（EvidenceRequest）或詢問 ID（EvidenceInquiry），Position 是
// 佐證在那一筆紀錄裡的位置（0 起算）。
type ReadEvidenceInput struct {
	AdminID  int64
	Kind     EvidenceKind
	RefID    uuid.UUID
	Position int
}

// ReporterContact 是匿名檢舉人的聯絡方式；保留期限後全部清成 nil。
type ReporterContact struct {
	Phone, Email, Name *string
}

// Violation 是一筆認定（違規）：admin 對一次違規事實的認定，只讀，不帶處分（處分是措施）。Severity 是套用的嚴重度，
// EscalatedFrom 是升級前的嚴重度（沒有升級時為 nil）。認定只由建立措施的 transaction 寫入，OriginMeasureID 是建立它的措施。
type Violation struct {
	ID                   uuid.UUID
	StoreID, AccountID   int64
	SourceReportIDs      []uuid.UUID
	Category             policy.Category
	Severity             Severity
	EscalatedFrom        *Severity
	Status               ViolationStatus
	OriginMeasureID      uuid.UUID
	CreatedByAdminID     int64
	CreatedAt, UpdatedAt time.Time
	RevokedReason        *RevokeReason
	RevokedByAdminID     *int64
	RevokedAt            *time.Time
}

// LeadFindingOf 回傳 cited 當中的主要依據（措施的 lead_finding_id）：最嚴重的認定（CRITICAL > HIGH > MEDIUM > LOW），同嚴重度取最新建立的
// （CreatedAt 較晚，再相同取 ID 較大）。順序是全序，結果只取決於這一組認定，和清單的排列無關：引用的認定是集合，主要依據不編碼在位置裡。
// 建立措施時有新的認定就用新的認定當主要依據，沒有新的認定才用這個規則挑引用的認定。cited 是空的時候 ok 為 false。
func LeadFindingOf(cited []Violation) (lead Violation, ok bool) {
	for i, v := range cited {
		if i == 0 || leadsOver(v, lead) {
			lead, ok = v, true
		}
	}
	return lead, ok
}

// leadsOver 回報 a 是否比 b 更該當主要依據。
func leadsOver(a, b Violation) bool {
	if ra, rb := a.Severity.Rank(), b.Severity.Rank(); ra != rb {
		return ra > rb
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return a.ID.Compare(b.ID) > 0
}

// ViolationCursor 是認定列表的 keyset 位置（新到舊）。
type ViolationCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c ViolationCursor) Valid() bool { return c.ID != uuid.Nil() && !c.CreatedAt.IsZero() }

// ViolationListInput 是 admin 的認定列表；StoreID、AccountID 為 0、Status 為 nil 時不篩。
type ViolationListInput struct {
	StoreID, AccountID int64
	Status             *ViolationStatus
	PageSize           int32
	After              *ViolationCursor
}

// ViolationPage 是一頁認定；Next 為 nil 表示沒有下一頁。
type ViolationPage struct {
	Items []Violation
	Next  *ViolationCursor
}
