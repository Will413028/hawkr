package model

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 通報與案件（step10e-reports-brief R2、R3、R7；設計 step10e-3ca-design §3）。通報是一位檢舉人的一次送出，有自己的結果；同一家店、同一個平台判定類別，
// 同時最多一個進行中的案件，決定即結案。處理狀態（ReportState、CaseState）、可以做的動作（Availability）都在讀取時由事實推導，不存。

// Outcome 是一筆通報的結果；值與 DB 的 reports_outcome_check、契約的 ReportOutcome 一致。
type Outcome string

const (
	// OutcomeActionTaken 平台採取了措施；只會是案件裡的通報（DB 的 reports_action_taken_check）。
	OutcomeActionTaken Outcome = "ACTION_TAKEN"
	// OutcomeNoViolation 查過之後沒有違規。
	OutcomeNoViolation Outcome = "NO_VIOLATION"
	// OutcomeInsufficient 內容不足，無法判斷。
	OutcomeInsufficient Outcome = "INSUFFICIENT"
	// OutcomeDuplicate 與另一筆通報重複。
	OutcomeDuplicate Outcome = "DUPLICATE"
	// OutcomeAbusive 濫報。
	OutcomeAbusive Outcome = "ABUSIVE"
)

// OutcomeValues 回傳全部結果，順序固定（可以設定的結果、可以選的決定都照這個順序）：已採取措施、未違規、內容不足、重複、濫報。
func OutcomeValues() []Outcome {
	return []Outcome{OutcomeActionTaken, OutcomeNoViolation, OutcomeInsufficient, OutcomeDuplicate, OutcomeAbusive}
}

// Negates 回報這個結果是否否定了通報（未違規、內容不足或濫報）：被否定的通報不能當認定的來源、不列進轉報資料。
func (o Outcome) Negates() bool {
	return o == OutcomeNoViolation || o == OutcomeInsufficient || o == OutcomeAbusive
}

// Decision 是案件的決定，決定即結案；值與 DB 的 cases_close_check、契約的 CaseDecision 一致。
type Decision string

const (
	// DecisionActionTaken 採取措施：結案時沒有結果的通報帶入「已採取措施」。
	DecisionActionTaken Decision = "ACTION_TAKEN"
	// DecisionNoAction 不採取措施：結案時沒有結果的通報帶入「未違規」。
	DecisionNoAction Decision = "NO_ACTION"
)

// DecisionValues 回傳全部決定，順序固定：採取措施、不採取措施。
func DecisionValues() []Decision { return []Decision{DecisionActionTaken, DecisionNoAction} }

// DefaultOutcome 是結案時帶入沒有結果的通報的結果（API 的 DecisionOption.default_outcome）：採取措施是已採取措施，不採取措施是未違規。
func (d Decision) DefaultOutcome() Outcome {
	if d == DecisionActionTaken {
		return OutcomeActionTaken
	}
	return OutcomeNoViolation
}

// CaseCloseReason 是案件結案的原因；值與 DB 的 cases_close_check、契約的 CaseCloseReason 一致。
type CaseCloseReason string

const (
	// CaseClosedDecided 管理員決定（DecideCase）。
	CaseClosedDecided CaseCloseReason = "DECIDED"
	// CaseClosedNoReports 通報都移到其他案件，系統關閉，不是一次決定。
	CaseClosedNoReports CaseCloseReason = "NO_REPORTS"
)

// CaseCloseReasonValues 回傳全部結案原因。
func CaseCloseReasonValues() []CaseCloseReason {
	return []CaseCloseReason{CaseClosedDecided, CaseClosedNoReports}
}

// ReportState 是通報此刻的處理狀態（契約的 ReportState）：由事實推導，不存。
type ReportState string

const (
	// ReportStateAwaitingTriage 待分案：沒有案件也沒有結果。
	ReportStateAwaitingTriage ReportState = "AWAITING_TRIAGE"
	// ReportStateInCase 在進行中的案件裡、還沒有結果。
	ReportStateInCase ReportState = "IN_CASE"
	// ReportStateResolved 已有結果。
	ReportStateResolved ReportState = "RESOLVED"
)

// ReportStateValues 回傳全部通報狀態。
func ReportStateValues() []ReportState {
	return []ReportState{ReportStateAwaitingTriage, ReportStateInCase, ReportStateResolved}
}

// CaseState 是案件此刻的狀態（契約的 CaseState）：由 closed_at 推導，不存。
type CaseState string

const (
	// CaseStateOpen 進行中。
	CaseStateOpen CaseState = "OPEN"
	// CaseStateClosed 已結案（終態）。
	CaseStateClosed CaseState = "CLOSED"
)

// CaseStateValues 回傳全部案件狀態。
func CaseStateValues() []CaseState { return []CaseState{CaseStateOpen, CaseStateClosed} }

// SupplementMatch 是匿名的補充以哪一種聯絡方式比對到原通報；值與 DB 的 report_supplements_matched_by_check、契約的 SupplementMatch 一致。
type SupplementMatch string

const (
	// MatchedByPhone 電話相同。
	MatchedByPhone SupplementMatch = "PHONE"
	// MatchedByEmail 聯絡信箱相同。
	MatchedByEmail SupplementMatch = "EMAIL"
	// MatchedByBoth 電話與聯絡信箱都相同。
	MatchedByBoth SupplementMatch = "BOTH"
)

// SupplementMatchValues 回傳全部比對方式。
func SupplementMatchValues() []SupplementMatch {
	return []SupplementMatch{MatchedByPhone, MatchedByEmail, MatchedByBoth}
}

const (
	// MaxSupplements 是一筆通報最多的補充則數（超過時，登入的送出回 failed_precondition，匿名的內容不存、只計次數）。
	MaxSupplements = 10
	// MaxReportEvidence 是一筆通報（原通報加全部補充）最多的佐證張數，位置的上界。
	MaxReportEvidence = MaxEvidence * (1 + MaxSupplements)
	// ContactRetentionAfterOutcome 是匿名檢舉人聯絡欄位在結果設定之後的保留期限；清除時間是 greatest(建立 + ContactRetention, 結果 + 這個)。
	ContactRetentionAfterOutcome = 30 * 24 * time.Hour
)

// Supplement 是同一位檢舉人在去重期間內再次送出、併入原通報的內容；只新增。MatchedBy 只有匿名的補充有；EvidencePositions 是這則補充帶來的佐證在
// 通報的佐證裡的位置（單筆讀取時載入）。
type Supplement struct {
	ID                uuid.UUID
	Description       string
	MatchedBy         *SupplementMatch
	EvidencePositions []int
	CreatedAt         time.Time
}

// Report 是一次通報。登入的檢舉人有 ReporterBuyerID、沒有 Contact；匿名的相反（清除後 Contact 為 nil、ContactErasedAt 有值）。Supplements、Evidence 只在
// 讀取單筆時載入；SupplementCount、LatestSupplementSeq、LatestSupplementAt、SupplementsSeenSeq、StoreName 在清單與單筆都帶（StoreName 在個資清理之後是 nil）。
type Report struct {
	ID                   uuid.UUID
	StoreID              int64
	StoreName            *string
	ProductID, OrderNo   *string
	Category             policy.Category
	Description          string
	ReporterBuyerID      *int64
	Contact              *ReporterContact
	ContactErasedAt      *time.Time
	CaseID               *uuid.UUID
	AssessedCategory     *policy.Category
	TriagedAt            *time.Time
	TriagedByAdminID     *int64
	Outcome              *Outcome
	ResolvedAt           *time.Time
	ResolvedByAdminID    *int64
	ResolutionNote       *string
	DiscardedSubmissions int
	Version              int64
	SupplementCount      int
	// LatestSupplementSeq 是最新一則補充的編號（max(Supplement.Seq)；沒有補充時 0），與 SupplementsSeenSeq 比較就是「有新的補充」。
	LatestSupplementSeq int
	LatestSupplementAt  *time.Time
	// SupplementsSeenSeq 是管理員看過的最新補充的編號（Supplement.Seq；0 = 還沒看過任何補充），不是時間。
	SupplementsSeenSeq   int
	Supplements          []Supplement
	Evidence             []Evidence
	CreatedAt, UpdatedAt time.Time
	// TriageDeadline 是待分案的期限（seller.review_report_triage）；不是待分案時 nil。由讀取通報的路徑填入，不是建構時就有。
	TriageDeadline *Deadline
}

// State 是通報此刻的處理狀態；ListReports 的 state 篩選與它由資料集測試鎖住一致。
func (r Report) State() ReportState {
	switch {
	case r.Outcome != nil:
		return ReportStateResolved
	case r.CaseID != nil:
		return ReportStateInCase
	}
	return ReportStateAwaitingTriage
}

// EffectiveCategory 是生效的類別：在案件裡的是平台判定類別，其他是檢舉人選的類別（ListReports 的 category 篩選用的鍵）。
func (r Report) EffectiveCategory() policy.Category {
	if r.AssessedCategory != nil {
		return *r.AssessedCategory
	}
	return r.Category
}

// HasUnseenSupplements 回報有沒有管理員還沒看過的補充：還沒有結果，而且存在編號大於 SupplementsSeenSeq（分案、移動與明確已讀時記下）的補充。
// 比的是最新一則的編號（LatestSupplementSeq，max(seq)）而不是則數：與 SQL（db/queries/cases.sql 的 EXISTS seq > supplements_seen_seq、
// MarkReportSupplementsSeen 記下 max(seq)）同一個定義，不依賴編號連號（DB 只保證 1..10 與每筆通報內唯一）。比編號不比時間，不依賴任何時鐘。
// 讀取、案件結案帶入的預設結果、改案件嚴重度與記錄轉報都不記（它們不是帶著這筆通報的 etag 做的）。有了結果的通報不再提醒（補充只併入還沒有結果的通報；
// 結果是管理員設的，或案件結案時由決定帶入），所以設定結果不需要記。
func (r Report) HasUnseenSupplements() bool {
	return r.Outcome == nil && r.LatestSupplementSeq > r.SupplementsSeenSeq
}

// Supporting 回報結果有沒有否定這筆通報（沒有結果、已採取措施或重複，不是未違規、內容不足或濫報）：可以當認定的來源、列進轉報資料、預填進建立措施
// （rb R2 的 3b 規則）。
func (r Report) Supporting() bool { return r.Outcome == nil || !r.Outcome.Negates() }

// ReferralFollowUpDays 是記錄轉報時沒有指定追蹤日期的預設：轉報日（台北）加這麼多天。
const ReferralFollowUpDays = 30

// ReferralOutcome 是案件的轉報紀錄是哪一種：已轉報，或標記不轉報（兩者互斥，同一列）。
type ReferralOutcome string

const (
	// ReferralReferred 已轉報主管機關。
	ReferralReferred ReferralOutcome = "REFERRED"
	// ReferralWaived 標記不轉報（例如主管機關已經介入）。
	ReferralWaived ReferralOutcome = "WAIVED"
)

// FDAReferral 是食安案件轉報主管機關的紀錄；ReferenceNo 空字串是沒有案號。FollowUpOn 是追蹤日期（台北日期，以 UTC 午夜的 time.Time 表示，同 DATE 欄位的
// 讀法）：到了這天還沒收到主管機關的回覆就聯絡主管機關；nil 是沒有（回覆時清掉，或轉換前就有的紀錄）。Replies 是主管機關的回覆，舊到新。
type FDAReferral struct {
	ReportedAt        time.Time
	Channel           FDAChannel
	ReferenceNo       string
	ReportedByAdminID int64
	FollowUpOn        *time.Time
	// FollowUpSetAt 是設定目前這個追蹤日期的時間（記錄轉報，或記下回覆時換成下一個）；FollowUpOn 是 nil 時也是 nil。
	FollowUpSetAt *time.Time
	Replies       []FDAReferralReply
}

// FollowUpOverdue 是「追蹤逾期」：有追蹤日期、now 的台北日期晚於它，而且設定這個追蹤日期之後還沒有記下主管機關的回覆（記下回覆時換成下一個追蹤日期，
// 那一則回覆就是設定的時間點，不算「之後」）。只在讀取時推導，API 的 FDAReferral.follow_up_overdue 是它；client 不自己比較日期。
func (r FDAReferral) FollowUpOverdue(now time.Time) bool {
	if r.FollowUpOn == nil || r.FollowUpSetAt == nil || !twtime.Day(now).After(*r.FollowUpOn) {
		return false
	}
	for _, reply := range r.Replies {
		if reply.RecordedAt.After(*r.FollowUpSetAt) {
			return false
		}
	}
	return true
}

// FDAReferralReply 是主管機關對轉報的一則回覆（只新增）。ReceivedOn 是收到的台北日期（UTC 午夜的 time.Time）。
type FDAReferralReply struct {
	Body              string
	ReceivedOn        time.Time
	RecordedByAdminID int64
	RecordedAt        time.Time
}

// ReferralWaiver 是標記不轉報的紀錄。
type ReferralWaiver struct {
	Reason          string
	WaivedByAdminID int64
	WaivedAt        time.Time
}

// CaseReferral 是案件唯一的轉報紀錄：Outcome 是 ReferralReferred 時 FDA 有值，是 ReferralWaived 時 Waiver 有值，兩者至多一個（DB 的 CHECK 保證）。
type CaseReferral struct {
	Outcome ReferralOutcome
	FDA     *FDAReferral
	Waiver  *ReferralWaiver
}

// Referred 回報案件已經記錄轉報（不是不轉報）。
func (r *CaseReferral) Referred() bool { return r != nil && r.Outcome == ReferralReferred }

// Waived 回報案件已經標記不轉報。
func (r *CaseReferral) Waived() bool { return r != nil && r.Outcome == ReferralWaived }

// DefaultReferralFollowUp 是沒有指定追蹤日期時的預設：轉報日（台北）加 ReferralFollowUpDays 天。
func DefaultReferralFollowUp(now time.Time) time.Time {
	return twtime.Day(now).AddDate(0, 0, ReferralFollowUpDays)
}

// ReportMove 是一次把通報移到其他案件的紀錄（append-only）。
type ReportMove struct {
	ReportID, FromCaseID, ToCaseID uuid.UUID
	AdminID                        int64
	MovedAt                        time.Time
}

// Case 是案件：同一家店、同一個平台判定類別，同時最多一個進行中的案件。Category 是案件的類別（平台判定類別，Assessable 的子集）。Severity 在開案時決定，之後只經
// ChangeCaseSeverity 修改。ReportCount、UnresolvedReportCount、UnseenSupplementReports 與 StoreName 在清單與單筆都帶；Referral 與 Moves 在單筆讀取時載入
// （Referral 在清單也帶）。UpdatedAt 是上一次「處理」的時間：成員進出、成員有了結果、改嚴重度、記錄轉報、結案；補充、註記與措施不改它。
// UnseenSupplementReports 是有補充還沒被管理員看過的成員數（Report.HasUnseenSupplements 的計數），不看 UpdatedAt。
type Case struct {
	ID                      uuid.UUID
	StoreID                 int64
	StoreName               *string
	Category                policy.Category
	Severity                Severity
	Version                 int64
	OpenedAt                time.Time
	OpenedByAdminID         int64
	ClosedAt                *time.Time
	ClosedByAdminID         *int64
	CloseReason             *CaseCloseReason
	Decision                *Decision
	DecisionNote            *string
	ReportCount             int
	UnresolvedReportCount   int
	UnseenSupplementReports int
	Referral                *CaseReferral
	Moves                   []ReportMove
	UpdatedAt               time.Time
	// OpenInquiryID 是還沒結束的詢問（每個案件最多一筆）；沒有時 nil。在清單與單筆都帶（每頁一支固定查詢：OpenCaseInquiries）。
	// 「待決定」的開始時間只在 DecideDeadline.Start（seller.review_case_decision）。
	OpenInquiryID *uuid.UUID
	// DecideDeadline 是待決定的期限（seller.review_case_decision）、FDAReferralDeadline 是待轉報的期限（seller.review_case_fda_referral）；
	// 義務不在時 nil。由 caseFacts 填入。
	DecideDeadline, FDAReferralDeadline *Deadline
}

// Open 回報案件是否進行中。
func (c Case) Open() bool { return c.ClosedAt == nil }

// State 是案件此刻的狀態。
func (c Case) State() CaseState {
	if c.Open() {
		return CaseStateOpen
	}
	return CaseStateClosed
}

// AwaitingSeller 是「等賣家」：進行中而且有還沒結束的詢問（rb R3 的推導，不儲存）。
func (c Case) AwaitingSeller() bool { return c.Open() && c.OpenInquiryID != nil }

// ReportBlock 是通報此刻不能做某個動作的一個原因；原因的句子是 Text。
type ReportBlock string

const (
	// ReportBlockResolved 已有結果。
	ReportBlockResolved ReportBlock = "RESOLVED"
	// ReportBlockAlreadyTriaged 已在案件裡。
	ReportBlockAlreadyTriaged ReportBlock = "ALREADY_TRIAGED"
	// ReportBlockNeedsCase 還沒分案（已採取措施只能設在案件裡的通報）。
	ReportBlockNeedsCase ReportBlock = "NEEDS_CASE"
	// ReportBlockNoActionBasis 案件還沒有可以說「已採取措施」的作為（沒有導出的措施，也沒有成員的結果是已採取措施）。
	ReportBlockNoActionBasis ReportBlock = "NO_ACTION_BASIS"
	// ReportBlockNotInOpenCase 不在進行中的案件裡。
	ReportBlockNotInOpenCase ReportBlock = "NOT_IN_OPEN_CASE"
	// ReportBlockFindingSource 是某個認定的來源。
	ReportBlockFindingSource ReportBlock = "FINDING_SOURCE"
	// ReportBlockLastReportWithMeasure 是案件唯一的成員，而案件有導出的措施（有導出措施的案件不會因移動而變空）。
	ReportBlockLastReportWithMeasure ReportBlock = "LAST_REPORT_WITH_MEASURE"
	// ReportBlockLastReportWithReferral 是案件唯一的成員，而案件已記錄轉報（有轉報紀錄的案件不會因移動而變空：空案件以 NO_REPORTS 關閉，
	// 而轉報資料要有通報才能匯出）。
	ReportBlockLastReportWithReferral ReportBlock = "LAST_REPORT_WITH_REFERRAL"
	// ReportBlockNegated 結果是未違規、內容不足或濫報。
	ReportBlockNegated ReportBlock = "NEGATED"
	// ReportBlockStoreClosed 店家已關閉。
	ReportBlockStoreClosed ReportBlock = "STORE_CLOSED"
)

// ReportBlockValues 回傳全部通報不能做動作的原因。
func ReportBlockValues() []ReportBlock {
	return []ReportBlock{
		ReportBlockResolved, ReportBlockAlreadyTriaged, ReportBlockNeedsCase, ReportBlockNoActionBasis, ReportBlockNotInOpenCase,
		ReportBlockFindingSource, ReportBlockLastReportWithMeasure, ReportBlockLastReportWithReferral, ReportBlockNegated, ReportBlockStoreClosed,
	}
}

// Text 是原因給後台看的一句話（只有 zh-TW）；指令被擋下時錯誤的訊息取第一個原因的句子。不認得的原因回零值（Text 是空字串）。
func (b ReportBlock) Text() uitext.Message {
	switch b {
	case ReportBlockResolved:
		return text.ReportUnavailableResolved()
	case ReportBlockAlreadyTriaged:
		return text.ReportUnavailableAlreadyTriaged()
	case ReportBlockNeedsCase:
		return text.ReportUnavailableNeedsCase()
	case ReportBlockNoActionBasis:
		return text.ReportUnavailableNoActionBasis()
	case ReportBlockNotInOpenCase:
		return text.ReportUnavailableNotInOpenCase()
	case ReportBlockFindingSource:
		return text.ReportUnavailableFindingSource()
	case ReportBlockLastReportWithMeasure:
		return text.ReportUnavailableLastReportWithMeasure()
	case ReportBlockLastReportWithReferral:
		return text.ReportUnavailableLastReportWithReferral()
	case ReportBlockNegated:
		return text.SourceReportNegated()
	case ReportBlockStoreClosed:
		return StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure)
	}
	return uitext.Message{}
}

// CaseBlock 是案件此刻不能做某個動作的一個原因；原因的句子是 Text（部分原因的句子依動作而不同）。
type CaseBlock string

const (
	// CaseBlockClosed 案件已結案。
	CaseBlockClosed CaseBlock = "CLOSED"
	// CaseBlockNoActionBasis 還沒有可以說「採取措施」的作為（沒有導出的措施，也沒有成員的結果是已採取措施）。
	CaseBlockNoActionBasis CaseBlock = "NO_ACTION_BASIS"
	// CaseBlockUnrevokedMeasureExists 有未撤銷的導出措施。
	CaseBlockUnrevokedMeasureExists CaseBlock = "UNREVOKED_MEASURE_EXISTS"
	// CaseBlockReportActionTaken 有成員的結果是已採取措施。
	CaseBlockReportActionTaken CaseBlock = "REPORT_ACTION_TAKEN"
	// CaseBlockStoreClosed 店家已關閉。
	CaseBlockStoreClosed CaseBlock = "STORE_CLOSED"
	// CaseBlockNotFoodSafety 平台判定類別不是食安。
	CaseBlockNotFoodSafety CaseBlock = "NOT_FOOD_SAFETY"
	// CaseBlockNoReports 以 NO_REPORTS 關閉的案件（沒有通報）。
	CaseBlockNoReports CaseBlock = "NO_REPORTS"
	// CaseBlockReferralRecorded 已經記錄過轉報（記過時指令回原紀錄，不是錯誤）。
	CaseBlockReferralRecorded CaseBlock = "REFERRAL_RECORDED"
	// CaseBlockReferralWaived 已經標記不轉報（標記過時，不轉報的指令回原紀錄，其餘指令是錯誤）。
	CaseBlockReferralWaived CaseBlock = "REFERRAL_WAIVED"
	// CaseBlockNoReferral 還沒有轉報紀錄，沒有東西可以回覆。
	CaseBlockNoReferral CaseBlock = "NO_REFERRAL"
	// CaseBlockInquiryOpen 已經有一筆還沒結束的詢問。
	CaseBlockInquiryOpen CaseBlock = "INQUIRY_OPEN"
	// CaseBlockStorePurged 店家的個資已依保留期限清除，沒有聯絡資料可以詢問賣家（只是關店不擋）。
	CaseBlockStorePurged CaseBlock = "STORE_PURGED"
)

// CaseBlockValues 回傳全部案件不能做動作的原因。
func CaseBlockValues() []CaseBlock {
	return []CaseBlock{
		CaseBlockClosed, CaseBlockNoActionBasis, CaseBlockUnrevokedMeasureExists, CaseBlockReportActionTaken, CaseBlockStoreClosed,
		CaseBlockNotFoodSafety, CaseBlockNoReports, CaseBlockReferralRecorded, CaseBlockReferralWaived, CaseBlockNoReferral, CaseBlockInquiryOpen, CaseBlockStorePurged,
	}
}

// CaseAction 是案件上有前置條件的動作；CaseBlock.Text 依它選句（轉報與匯出的句子不同；已結案時，連結措施的句子說明是不能再從案件建立措施）。
type CaseAction string

const (
	// CaseActionDecide 決定（結案）。
	CaseActionDecide CaseAction = "DECIDE"
	// CaseActionChangeSeverity 改嚴重度。
	CaseActionChangeSeverity CaseAction = "CHANGE_SEVERITY"
	// CaseActionLinkMeasure 把新的措施記為這個案件導出的措施。
	CaseActionLinkMeasure CaseAction = "LINK_MEASURE"
	// CaseActionCreateMeasure 從案件建立警告、停權或風險措施。
	CaseActionCreateMeasure CaseAction = "CREATE_MEASURE"
	// CaseActionRecordReferral 記錄轉報主管機關。
	CaseActionRecordReferral CaseAction = "RECORD_REFERRAL"
	// CaseActionWaiveReferral 標記不轉報。
	CaseActionWaiveReferral CaseAction = "WAIVE_REFERRAL"
	// CaseActionRecordReferralReply 記下主管機關對轉報的回覆。
	CaseActionRecordReferralReply CaseAction = "RECORD_REFERRAL_REPLY"
	// CaseActionCreateInquiry 對案件的賣家發詢問。
	CaseActionCreateInquiry CaseAction = "CREATE_INQUIRY"
	// CaseActionExport 匯出轉報資料。
	CaseActionExport CaseAction = "EXPORT"
)

// Text 是原因給後台看的一句話（只有 zh-TW）；指令被擋下時錯誤的訊息取第一個原因的句子。不認得的原因回零值（Text 是空字串）。
func (b CaseBlock) Text(action CaseAction) uitext.Message {
	switch b {
	case CaseBlockClosed:
		if action == CaseActionLinkMeasure {
			return text.MeasureSourceCaseClosed()
		}
		return text.CaseUnavailableClosed()
	case CaseBlockNoActionBasis:
		return text.CaseUnavailableNoActionBasis()
	case CaseBlockUnrevokedMeasureExists:
		return text.CaseUnavailableUnrevokedMeasureExists()
	case CaseBlockReportActionTaken:
		return text.CaseUnavailableReportActionTaken()
	case CaseBlockStoreClosed:
		return StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure)
	case CaseBlockNotFoodSafety:
		if action == CaseActionExport {
			return text.ExportNotFoodSafety()
		}
		return text.ReferralNotFoodSafety()
	case CaseBlockNoReports:
		if action == CaseActionExport {
			return text.ExportNoReports()
		}
		return text.ReferralNoReports()
	case CaseBlockReferralRecorded:
		return text.CaseUnavailableReferralRecorded()
	case CaseBlockReferralWaived:
		return text.CaseUnavailableReferralWaived()
	case CaseBlockNoReferral:
		return text.CaseUnavailableNoReferral()
	case CaseBlockInquiryOpen:
		return text.CaseUnavailableInquiryOpen()
	case CaseBlockStorePurged:
		return text.CaseUnavailableStorePurged()
	}
	return uitext.Message{}
}

// CaseFacts 是判斷案件能做什麼需要的事實：同一個讀取快照，或指令裡鎖住之後讀。
type CaseFacts struct {
	Case               Case
	StoreClosed        bool // 案件的店家（stint）已關閉
	StorePurged        bool // 案件的店家（stint）的個資已依保留期限清除（pii_purged_at）
	ActionTakenReports int  // 結果是已採取措施的成員數
	ResultingMeasures  int  // 導出的措施數，不論狀態（案件導出的措施的定義見 db/queries/cases.sql 的 CaseMeasureCounts）
	UnrevokedMeasures  int  // 其中未撤銷的
}

// ActionTakenSupported 回報已經有作為可以說「已採取措施」：有導出的措施（已撤銷的也算：措施做過就是採取過措施，撤銷是之後的更正，不改寫當時的決定），
// 或已有成員的結果是已採取措施。案件的「採取措施」與成員個別的「已採取措施」共用它；它也保證每個進行中的案件至少有一個可以選的決定
// （不成立時沒有任何導出措施，也沒有已採取措施的成員，「不採取措施」可選）。
func (f CaseFacts) ActionTakenSupported() bool {
	return f.ResultingMeasures > 0 || f.ActionTakenReports > 0
}

// CaseAvailability 是案件每個動作不能做的原因，空的就是可以做。
type CaseAvailability struct {
	Decide                                                             map[Decision][]CaseBlock
	ChangeSeverity, LinkMeasure, CreateMeasure, RecordReferral, Export []CaseBlock
	// WaiveReferral 是標記不轉報：與記錄轉報的前置條件相同，加上已經有轉報紀錄（已轉報或已標記不轉報）。
	WaiveReferral []CaseBlock
	// RecordReferralReply 是記下主管機關的回覆：食安案件、已記錄轉報（沒有轉報紀錄或已標記不轉報都不行）。
	RecordReferralReply []CaseBlock
	// CreateInquiry 是對賣家發詢問：案件進行中、店家的個資還沒清除（只是關店不擋）、沒有還沒結束的詢問。
	CreateInquiry []CaseBlock
}

// DecisionOptions 是此刻可以選的決定（固定順序）；進行中的案件至少有一個。
func (a CaseAvailability) DecisionOptions() []Decision {
	var out []Decision
	for _, d := range DecisionValues() {
		if len(a.Decide[d]) == 0 {
			out = append(out, d)
		}
	}
	return out
}

// Availability 是案件每個動作不能做的原因（依序列出，錯誤取第一個）。這是唯一的寫法：API 的 can_*、decision_options 與指令的前置檢查都只看它；
// 新增或改動條件時改這個函式，不另寫布林判斷（同 Measure.*UnavailableReasons 的規則）。
func (f CaseFacts) Availability() CaseAvailability {
	c := f.Case
	a := CaseAvailability{Decide: make(map[Decision][]CaseBlock, 2)}
	var closed []CaseBlock
	if !c.Open() {
		closed = []CaseBlock{CaseBlockClosed}
	}
	for _, d := range DecisionValues() {
		blocks := append([]CaseBlock(nil), closed...)
		switch d {
		case DecisionActionTaken:
			if !f.ActionTakenSupported() {
				blocks = append(blocks, CaseBlockNoActionBasis)
			}
		case DecisionNoAction:
			if f.UnrevokedMeasures > 0 {
				blocks = append(blocks, CaseBlockUnrevokedMeasureExists)
			}
			if f.ActionTakenReports > 0 {
				blocks = append(blocks, CaseBlockReportActionTaken)
			}
		}
		a.Decide[d] = blocks
	}
	a.ChangeSeverity = append([]CaseBlock(nil), closed...)
	a.LinkMeasure = append([]CaseBlock(nil), closed...)
	a.CreateMeasure = append([]CaseBlock(nil), closed...)
	if !(TerminationTarget{Closed: f.StoreClosed}).AllowsMeasure() {
		a.CreateMeasure = append(a.CreateMeasure, CaseBlockStoreClosed)
	}
	var referral []CaseBlock
	if c.Category != policy.FoodSafety {
		referral = append(referral, CaseBlockNotFoodSafety)
	}
	if c.CloseReason != nil && *c.CloseReason == CaseClosedNoReports {
		referral = append(referral, CaseBlockNoReports)
	}
	a.Export = referral
	a.RecordReferral = append([]CaseBlock(nil), referral...)
	a.WaiveReferral = append([]CaseBlock(nil), referral...)
	switch {
	case c.Referral.Referred():
		a.RecordReferral = append(a.RecordReferral, CaseBlockReferralRecorded)
		a.WaiveReferral = append(a.WaiveReferral, CaseBlockReferralRecorded)
	case c.Referral.Waived():
		a.RecordReferral = append(a.RecordReferral, CaseBlockReferralWaived)
		a.WaiveReferral = append(a.WaiveReferral, CaseBlockReferralWaived)
	}
	if c.Category != policy.FoodSafety {
		a.RecordReferralReply = append(a.RecordReferralReply, CaseBlockNotFoodSafety)
	}
	switch {
	case c.Referral == nil:
		a.RecordReferralReply = append(a.RecordReferralReply, CaseBlockNoReferral)
	case c.Referral.Waived():
		a.RecordReferralReply = append(a.RecordReferralReply, CaseBlockReferralWaived)
	}
	a.CreateInquiry = append([]CaseBlock(nil), closed...)
	if f.StorePurged {
		a.CreateInquiry = append(a.CreateInquiry, CaseBlockStorePurged)
	}
	if c.OpenInquiryID != nil {
		a.CreateInquiry = append(a.CreateInquiry, CaseBlockInquiryOpen)
	}
	return a
}

// ReportFacts 是判斷通報能做什麼需要的事實；Case 是它所在的案件（沒有時 nil）：帶成員統計、導出的措施數與轉報紀錄，移動要看案件上有沒有措施或轉報。
type ReportFacts struct {
	Report        Report
	StoreClosed   bool
	Case          *CaseFacts
	FindingSource bool // 是某個認定的來源（finding_reports）
}

// ReportAvailability 是通報每個動作不能做的原因，空的就是可以做。Resolve 是每個結果不能設的原因。
type ReportAvailability struct {
	Triage, Move, CiteAsSource []ReportBlock
	Resolve                    map[Outcome][]ReportBlock
}

// AvailableOutcomes 是此刻可以設定的結果（固定順序）。
func (a ReportAvailability) AvailableOutcomes() []Outcome {
	var out []Outcome
	for _, o := range OutcomeValues() {
		if len(a.Resolve[o]) == 0 {
			out = append(out, o)
		}
	}
	return out
}

// Availability 是通報每個動作不能做的原因（依序列出，錯誤取第一個）；唯一的寫法，同 CaseFacts.Availability。
func (f ReportFacts) Availability() ReportAvailability {
	r := f.Report
	resolved := r.Outcome != nil
	a := ReportAvailability{Resolve: make(map[Outcome][]ReportBlock, len(OutcomeValues()))}
	if resolved {
		a.Triage = append(a.Triage, ReportBlockResolved)
	}
	if r.CaseID != nil {
		a.Triage = append(a.Triage, ReportBlockAlreadyTriaged)
	}
	for _, o := range OutcomeValues() {
		var blocks []ReportBlock
		if resolved {
			blocks = append(blocks, ReportBlockResolved)
		}
		if o == OutcomeActionTaken {
			switch {
			case r.CaseID == nil:
				blocks = append(blocks, ReportBlockNeedsCase)
			case f.Case == nil || !f.Case.ActionTakenSupported():
				blocks = append(blocks, ReportBlockNoActionBasis)
			}
		}
		a.Resolve[o] = blocks
	}
	if resolved {
		a.Move = append(a.Move, ReportBlockResolved)
	}
	if f.Case == nil || !f.Case.Case.Open() {
		a.Move = append(a.Move, ReportBlockNotInOpenCase)
	}
	if f.FindingSource {
		a.Move = append(a.Move, ReportBlockFindingSource)
	}
	if f.Case != nil && f.Case.Case.ReportCount == 1 {
		// 案件唯一的成員移走，案件就空了（以 NO_REPORTS 關閉）：案件上已經有措施或轉報紀錄時不行（兩個原因依序列出，錯誤取第一個）。
		if f.Case.ResultingMeasures > 0 {
			a.Move = append(a.Move, ReportBlockLastReportWithMeasure)
		}
		// 只擋已轉報：標記不轉報的案件空了照樣以 NO_REPORTS 關閉（不轉報的原因留在案件上），句子也只說「已記錄轉報」。
		if f.Case.Case.Referral.Referred() {
			a.Move = append(a.Move, ReportBlockLastReportWithReferral)
		}
	}
	if !r.Supporting() {
		a.CiteAsSource = append(a.CiteAsSource, ReportBlockNegated)
	}
	if !(TerminationTarget{Closed: f.StoreClosed}).AllowsMeasure() {
		a.CiteAsSource = append(a.CiteAsSource, ReportBlockStoreClosed)
	}
	return a
}

// ContactErasure 是一次匿名聯絡方式清理的結果：Erased 是清掉的筆數，Capped 是其中只靠絕對上限（ContactRetentionCap）清掉的筆數——
// 沒有結果卻拖滿一年的匿名通報，代表處理流程卡住了，所以單獨計數。
type ContactErasure struct {
	Erased, Capped int64
}

// TriageTarget 是通報分案或移動到平台判定類別 Category 時，這家店此刻的去處：Case 是那個類別進行中的案件（加入它），nil 是沒有（開新案件）。
type TriageTarget struct {
	Category policy.Category
	Case     *Case
}

// AdminReport 是後台看到的一筆通報：通報本身加上此刻可以做的動作（同一個讀取、同一個 Availability）。Targets 只在單筆讀取、而且此刻可以分案或移動時有值
// （每個可判定的類別一筆；移動時不含目前的類別）。
type AdminReport struct {
	Report
	Availability ReportAvailability
	Targets      []TriageTarget
}

// AdminCase 是後台看到的一個案件：案件本身加上此刻可以做的動作。
type AdminCase struct {
	Case
	Availability CaseAvailability
	// ReadAt 是讀取的時間：讀取時推導、會隨時間改變的欄位（轉報的 FollowUpOverdue）以它判斷。
	ReadAt time.Time
}

// SubmitReportInput 是提出通報（檢舉）的輸入；空字串表示沒有提供。ReporterBuyerID 大於零是登入的檢舉人（呼叫方解析的業務鍵），佐證由 AccountID（同一個登入者的帳號）上傳；
// ReporterBuyerID 為零是匿名，必須留電話或 email，不收佐證。AccountID 大於零而店家屬於那個帳號時不能檢舉自己的店（匿名入口在呼叫者登入時也帶上）。
type SubmitReportInput struct {
	StoreID            int64
	ProductID, OrderNo string
	Category           policy.Category
	Description        string
	ReporterBuyerID    int64
	AccountID          int64
	Phone, Email, Name string
	EvidenceTokens     []string
}

// SubmitReportResult 是提出通報的結果。ReportID 一定有值。Supplemented 表示去重期間內已有同一個檢舉人、還沒有結果的通報，這次的內容補充到它、沒有新增；Discarded 表示
// 那一筆的補充已達上限（只會是匿名的送出：登入的回錯誤），內容沒有存下。Report：新增時有值；登入的補充是原本那一筆（去重的鍵含買家，所以一定是呼叫者自己的）；
// 匿名的補充與丟棄是 nil——重複提交的人不一定是原本的檢舉人（例如只知道電話號碼的人），不回它的內容、聯絡方式或任何檢舉人的資料；匿名檢舉的傳輸層不得因此回不同的結果
// （不洩漏這支電話用過）。
type SubmitReportResult struct {
	ReportID     uuid.UUID
	Report       *Report
	Supplemented bool
	Discarded    bool
	// Notice 是補充到原通報時給檢舉人看的那一句（請求語系的伺服器句子：它說的是伺服器的去重規則）；只有 Supplemented 時有值。
	Notice uitext.Message
}

// CaseTarget 是分案或移動的明確意圖，剛好設一個：Join 是加入這家店這個類別進行中的案件（讀到的案件 ID 與版本），OpenNew 是開新案件（嚴重度）。
// 意圖此刻不成立（案件已關閉或版本前進、或這個類別已經有了進行中的案件）時回案件版本衝突，讓使用者重新確認。
type CaseTarget struct {
	Join    *JoinCase
	OpenNew *Severity
}

// JoinCase 是加入一個進行中的案件；ExpectedVersion 是讀到的案件的版本。
type JoinCase struct {
	CaseID          uuid.UUID
	ExpectedVersion int64
}

// TriageReportInput 是分案的輸入：AssessedCategory 是平台判定類別（Assessable 的子集）；Target 是去處（加入進行中的案件，或開新案件並給嚴重度；嚴重度以案件為準）。
// ExpectedVersion 是讀到的通報的版本。
type TriageReportInput struct {
	ReportID         uuid.UUID
	AdminID          int64
	AssessedCategory policy.Category
	Target           CaseTarget
	ExpectedVersion  int64
}

// ResolveReportInput 是設定結果的輸入：Note 是給檢舉人的結果說明（最多 2000 字，選填）。
type ResolveReportInput struct {
	ReportID        uuid.UUID
	AdminID         int64
	Outcome         Outcome
	Note            string
	ExpectedVersion int64
}

// MarkSupplementsSeenInput 是記下管理員看過通報補充的輸入：ExpectedVersion 是讀到補充時的通報版本。
type MarkSupplementsSeenInput struct {
	ReportID        uuid.UUID
	ExpectedVersion int64
}

// MoveReportInput 是移到其他案件（改平台判定類別）的輸入：AssessedCategory 必須和目前的不同；Target 的規則同 TriageReportInput。
type MoveReportInput struct {
	ReportID         uuid.UUID
	AdminID          int64
	AssessedCategory policy.Category
	Target           CaseTarget
	ExpectedVersion  int64
}

// MoveReportResult 是移動的結果；SourceCaseClosed 表示原案件因為通報都移走而以 NO_REPORTS 關閉。
type MoveReportResult struct {
	Report           AdminReport
	SourceCaseClosed bool
}

// DecideCaseInput 是決定（結案）的輸入：Note 是給檢舉人的結果說明（最多 2000 字，選填），帶入還沒有結果的通報。
type DecideCaseInput struct {
	CaseID          uuid.UUID
	AdminID         int64
	Decision        Decision
	Note            string
	ExpectedVersion int64
}

// ChangeCaseSeverityInput 是改嚴重度的輸入：Severity 必須和目前的不同；Note 是原因（1 到 2000 字），寫成案件的一則內部註記。
type ChangeCaseSeverityInput struct {
	CaseID          uuid.UUID
	AdminID         int64
	Severity        Severity
	Note            string
	ExpectedVersion int64
}

// RecordFDAReferralInput 是記下轉報的輸入；ReferenceNo 選填（主管機關的案號，最多 100 字）。FollowUpOn 選填：追蹤日期（台北日期，UTC 午夜），不早於今天；
// nil 時是轉報日（台北）加 ReferralFollowUpDays 天。
type RecordFDAReferralInput struct {
	CaseID      uuid.UUID
	AdminID     int64
	Channel     FDAChannel
	ReferenceNo string
	FollowUpOn  *time.Time
}

// WaiveFDAReferralInput 是標記不轉報的輸入；Reason 必填（1 到 2000 字）。
type WaiveFDAReferralInput struct {
	CaseID  uuid.UUID
	AdminID int64
	Reason  string
}

// WaiveFDAReferralResult 是標記不轉報的結果；AlreadyWaived 表示之前已經標記過，回傳的案件帶的是原本的紀錄。
type WaiveFDAReferralResult struct {
	Case          AdminCase
	AlreadyWaived bool
}

// RecordFDAReferralReplyInput 是記下主管機關回覆的輸入。ReceivedOn 是收到的台北日期（UTC 午夜），不早於轉報日、不晚於今天；Body 1 到 2000 字。
// NextFollowUpOn 設定時取代原本的追蹤日期（不早於今天）；ClearFollowUp 為 true 時清掉；兩者都沒有時不變，兩者同時有是無效的輸入。
type RecordFDAReferralReplyInput struct {
	CaseID         uuid.UUID
	AdminID        int64
	Body           string
	ReceivedOn     time.Time
	NextFollowUpOn *time.Time
	ClearFollowUp  bool
	// RequestID 選填（AIP-155），範圍是（記下的管理員, request_id）：同一個值、同樣的內容重送不會多記一則（回應遺失之後再送一次）。
	RequestID *uuid.UUID
}

// RecordFDAReferralResult 是記下轉報的結果；AlreadyRecorded 表示之前已經記過，回傳的案件帶的是原本的紀錄。
type RecordFDAReferralResult struct {
	Case            AdminCase
	AlreadyRecorded bool
}

// ReportCursor 是通報列表的 keyset 位置（依建立時間；方向由篩選決定：待分案與案件的成員舊到新，其他新到舊）。
type ReportCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c ReportCursor) Valid() bool { return c.ID != uuid.Nil() && !c.CreatedAt.IsZero() }

// ReportListInput 是 admin 通報清單的輸入；State、Category 為 nil、StoreID 為 0、CaseID 為 nil、CitableAsSource 為 false 時不篩。Category 是生效的類別（案件裡的是平台判定類別，
// 其他是檢舉人選的類別）。CitableAsSource 只列此刻可以當認定來源的通報（Availability 的 CiteAsSource 沒有原因：店家沒有關閉、結果沒有否定它）。
type ReportListInput struct {
	State           *ReportState
	Category        *policy.Category
	StoreID         int64
	CaseID          *uuid.UUID
	CitableAsSource bool
	PageSize        int32
	After           *ReportCursor
}

// OldestFirst 回報這個清單是否依建立時間舊到新（待分案與案件成員的處理順序，等最久的在最前面）。
func (in ReportListInput) OldestFirst() bool {
	return in.CaseID != nil || (in.State != nil && *in.State == ReportStateAwaitingTriage)
}

// BuyerReportListInput 是登入的檢舉人自己的通報列表。
type BuyerReportListInput struct {
	BuyerID  int64
	PageSize int32
	After    *ReportCursor
}

// ReportPage 是一頁通報（檢舉人自己的）；Next 為 nil 表示沒有下一頁。
type ReportPage struct {
	Items []Report
	Next  *ReportCursor
}

// AdminReportPage 是一頁後台的通報；Next 為 nil 表示沒有下一頁。
type AdminReportPage struct {
	Items []AdminReport
	Next  *ReportCursor
}

// CaseCursor 是案件列表的 keyset 位置（開案時間新到舊）。
type CaseCursor struct {
	OpenedAt time.Time
	ID       uuid.UUID
}

// Valid 回報 page token 解出來的位置是否完整。
func (c CaseCursor) Valid() bool { return c.ID != uuid.Nil() && !c.OpenedAt.IsZero() }

// CaseListInput 是 admin 案件清單的輸入；State、Category 為 nil、StoreID 為 0 時不篩。
type CaseListInput struct {
	State    *CaseState
	Category *policy.Category
	StoreID  int64
	// AwaitingSeller 為 true 只列進行中而且有還沒結束的詢問的案件（Case.AwaitingSeller）；和 State = CLOSED 一起設回 invalid_argument。
	AwaitingSeller bool
	PageSize       int32
	After          *CaseCursor
}

// AdminCasePage 是一頁案件；Next 為 nil 表示沒有下一頁。
type AdminCasePage struct {
	Items []AdminCase
	Next  *CaseCursor
}

// FDAExport 是轉報主管機關需要的資料：案件、案件裡結果沒有被否定的通報（含補充與佐證的位置，不含檢舉人的聯絡方式）與以案件的 store_id 讀到的店家。呈現（文件格式）在 client。
type FDAExport struct {
	Case    AdminCase
	Reports []AdminReport
	Store   Store
}

// ReportReceived 在每次送出的 transaction 內發出：新的檢舉、補充（SupplementID）、補充已達上限而沒有存下的匿名送出（Discarded）。ConfirmationEmail 是這次送出留的
// email（只有匿名；登入者是 nil，A8 用站內通知）：確認信寄到這裡。它只在事件與 A8 同一個 transaction 排入的 job 參數裡，不落 seller 的表，也不帶原檢舉人的聯絡方式。
// A8 的訂閱者不得在 job 完成或丟棄後留著這個欄位（River 預設保留完成的 job 24 小時、丟棄的 7 天），見 events.go 與遷移計畫 A8。
type ReportReceived struct {
	OccurredAt        time.Time
	EventID           uuid.UUID
	ReportID          uuid.UUID
	SupplementID      *uuid.UUID
	Discarded         bool
	StoreID           int64
	ReporterBuyerID   *int64
	Category          policy.Category
	ConfirmationEmail *string
}

// ReportResolved 在通報有了結果的 transaction 內發出，每筆通報一次：個別設定、直接結掉或案件結案帶入都算。CaseID 是通報所在的案件（沒有案件時 nil）。
type ReportResolved struct {
	OccurredAt      time.Time
	EventID         uuid.UUID
	ReportID        uuid.UUID
	StoreID         int64
	ReporterBuyerID *int64
	Outcome         Outcome
	CaseID          *uuid.UUID
}
