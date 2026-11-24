package registry

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 治理與交易側型別：賣家店家面（措施、停權、KYC、詢問、撥款、代行、詢價）、作者與檢舉人的通知、刪帳確認信。
// 參數只存 ID、列舉值與事件時間，不存第三方的名稱或自由文字；列舉值是擁有模組的字串值（見各常數），文案在讀取時由列舉組成。
// 店家面型別的收件人由 worker 以 StoreRecipient 解析（店家已關閉不寫），買家面以 BuyerRecipient。

var governanceTypes = []Entry{
	storeSuspended, storeUnsuspended, storeContentTakenDown,
	storeMeasureIssued, storeMeasureRevised, storeMeasureEnded, storeMeasureRevoked, storeMeasureRequestDecided,
	kycApproved, kycRejected, storeCaseInquiry,
	storePendingDeletion, buyerDeletionRequested,
	payoutAccountActivated, payoutAccountDeadline,
	delegationOperationCompleted, rfqInvited,
	contentHidden, contentRestored, authorWarned,
	reportReceived, storeReportReceived, reportResolved, storeReportResolved,
	sellerReportResolved, moderationCaseClosedBuyers, moderationCaseClosedStores,
}

// 措施種類（seller.MeasureKind 的值；SUSPENSION 由 StoreSuspended 通知，CONTENT_TAKEDOWN 是 store_content_taken_down）。
const (
	MeasureKindWarning     = "WARNING"
	MeasureKindTermination = "TERMINATION"
	MeasureKindSuspension  = "SUSPENSION"
	MeasureKindTakedown    = "CONTENT_TAKEDOWN"
)

// 措施請求種類（seller.RequestKind 的值）。
const (
	RequestKindAppeal      = "APPEAL"
	RequestKindRemediation = "REMEDIATION"
)

// 措施請求的裁定結果（seller.RequestState 的 ACCEPTED、REJECTED）。
const (
	RequestOutcomeAccepted = "ACCEPTED"
	RequestOutcomeRejected = "REJECTED"
)

// 詢問種類（seller.InquiryKind 的值）。
const (
	InquiryKindRecords     = "RECORDS"
	InquiryKindExplanation = "EXPLANATION"
)

// 補件期限的檔位（payment.DeadlineTier 的值）。
const (
	DeadlineTierSevenDays = "SEVEN_DAYS"
	DeadlineTierOneDay    = "ONE_DAY"
)

// 被隱藏內容的來源：社群留言（含討論區留言）、評價、聊天訊息。
const (
	ContentSourceSocial  = "SOCIAL"
	ContentSourceReview  = "REVIEW"
	ContentSourceMessage = "MESSAGE"
)

// 檢舉的來源：moderation（社群留言、評價、聊天訊息）與 seller（店家）。
const (
	ReportSourceModeration = "MODERATION"
	ReportSourceSeller     = "SELLER"
)

// 檢舉結果的類別（seller.Outcome 的值；moderation 的 VIOLATION 對到 ACTION_TAKEN，NO_VIOLATION 同名）。
const (
	ReportOutcomeActionTaken  = "ACTION_TAKEN"
	ReportOutcomeNoViolation  = "NO_VIOLATION"
	ReportOutcomeInsufficient = "INSUFFICIENT"
	ReportOutcomeDuplicate    = "DUPLICATE"
	ReportOutcomeAbusive      = "ABUSIVE"
	// ModerationOutcomeViolation、ModerationOutcomeNoViolation 是 moderation.Outcome 的兩個值。
	ModerationOutcomeViolation   = "VIOLATION"
	ModerationOutcomeNoViolation = "NO_VIOLATION"
)

func badParams(format string, a ...any) error {
	return fmt.Errorf("%w："+format, append([]any{ErrBadParams}, a...)...)
}

// messageTime 是內文裡的時間：台北時間、到分鐘。
func messageTime(l uitext.Locale, t time.Time) string { return uitext.FormatDateTime(l, t) }

// withStore 補上店家面型別共同的欄位：受眾 STORE、分類 ACCOUNT、呈現 ACCOUNT。
func withStore[P any](d Definition[P], channels []model.Channel) Definition[P] {
	d.Audiences = []model.Audience{model.AudienceStore}
	d.Category = model.CategoryAccount
	d.Presentation = model.PresentationAccount
	d.Channels = channels
	return d
}

var (
	inboxPushEmail = []model.Channel{model.ChannelInbox, model.ChannelPush, model.ChannelEmail}
	inboxPush      = []model.Channel{model.ChannelInbox, model.ChannelPush}
	emailOnly      = []model.Channel{model.ChannelEmail}
)

func measurePath(id uuid.UUID) string { return "/seller/measures/" + id.String() }

const (
	sellerSuspendedPath  = "/seller-center/suspended"
	sellerOnboardingPath = "/seller/onboarding"
	sellerWalletPath     = "/seller-center/wallet"
)

var (
	exampleMeasureID         = uuid.MustParse("0190a000-0000-7000-8000-000000000002")
	exampleRequestID         = uuid.MustParse("0190a000-0000-7000-8000-000000000003")
	exampleReportID          = uuid.MustParse("0190a000-0000-7000-8000-000000000004")
	exampleCaseID            = uuid.MustParse("0190a000-0000-7000-8000-000000000005")
	exampleSanctionID        = uuid.MustParse("0190a000-0000-7000-8000-000000000006")
	exampleGovernanceEventID = uuid.MustParse("0190a000-0000-7000-8000-000000000007")
	exampleDueAt             = exampleAt.Add(72 * time.Hour)
)

func storeExpand[P any](storeID func(P) int64) func(ctx context.Context, r Resolver, p P) (*Expansion, error) {
	return func(ctx context.Context, r Resolver, p P) (*Expansion, error) {
		return StoreRecipient(ctx, r, storeID(p))
	}
}

func buyerExpand[P any](buyerID func(P) int64) func(ctx context.Context, r Resolver, p P) (*Expansion, error) {
	return func(ctx context.Context, r Resolver, p P) (*Expansion, error) {
		return BuyerRecipient(ctx, r, buyerID(p))
	}
}

// StoreSuspendedParams 是 store_suspended 的參數，欄位取自 seller.StoreSuspended（PeriodID 是這段停權期間的身分，與結束時相同）。
type StoreSuspendedParams struct {
	StoreID    int64     `json:"store_id"`
	PeriodID   int64     `json:"period_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// StoreUnsuspendedParams 是 store_unsuspended 的參數，欄位取自 seller.StoreUnsuspended。
type StoreUnsuspendedParams struct {
	StoreID    int64     `json:"store_id"`
	PeriodID   int64     `json:"period_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func periodKey(storeID, periodID int64) string {
	return strconv.FormatInt(storeID, 10) + "/" + strconv.FormatInt(periodID, 10)
}

var storeSuspended = Define(withStore(Definition[StoreSuspendedParams]{
	Name:              "store_suspended",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.StoreSuspended.StoreID 與 PeriodID",
	IdempotencyKey:    func(p StoreSuspendedParams) string { return periodKey(p.StoreID, p.PeriodID) },
	OccurredAt:        func(p StoreSuspendedParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StoreSuspendedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreSuspendedParams) (Content, error) {
		return Content{Title: text.StoreSuspendedTitle(l), Body: text.StoreSuspendedBody(l, text.StoreSuspendedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(StoreSuspendedParams) string { return sellerSuspendedPath },
	Example:  StoreSuspendedParams{StoreID: 11, PeriodID: 5, OccurredAt: exampleAt},
}, inboxPushEmail))

var storeUnsuspended = Define(withStore(Definition[StoreUnsuspendedParams]{
	Name:              "store_unsuspended",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.StoreUnsuspended.StoreID 與 PeriodID",
	IdempotencyKey:    func(p StoreUnsuspendedParams) string { return periodKey(p.StoreID, p.PeriodID) },
	OccurredAt:        func(p StoreUnsuspendedParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StoreUnsuspendedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreUnsuspendedParams) (Content, error) {
		return Content{Title: text.StoreUnsuspendedTitle(l), Body: text.StoreUnsuspendedBody(l, text.StoreUnsuspendedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	Example: StoreUnsuspendedParams{StoreID: 11, PeriodID: 5, OccurredAt: exampleAt},
}, inboxPushEmail))

// StoreContentTakenDownParams 是 store_content_taken_down 的參數，取自 seller.MeasureCreated（種類 CONTENT_TAKEDOWN）；ItemCount 是下架的內容數。
type StoreContentTakenDownParams struct {
	StoreID    int64     `json:"store_id"`
	MeasureID  uuid.UUID `json:"measure_id"`
	ItemCount  int       `json:"item_count"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storeContentTakenDown = Define(withStore(Definition[StoreContentTakenDownParams]{
	Name:              "store_content_taken_down",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureCreated.MeasureID",
	IdempotencyKey:    func(p StoreContentTakenDownParams) string { return p.MeasureID.String() },
	OccurredAt:        func(p StoreContentTakenDownParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StoreContentTakenDownParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreContentTakenDownParams) (Content, error) {
		return Content{
			Title: text.StoreContentTakenDownTitle(l),
			Body:  text.StoreContentTakenDownBody(l, text.StoreContentTakenDownBodyArgs{Count: p.ItemCount, Time: messageTime(l, p.OccurredAt)}),
		}, nil
	},
	DeepLink: func(p StoreContentTakenDownParams) string { return measurePath(p.MeasureID) },
	Example:  StoreContentTakenDownParams{StoreID: 11, MeasureID: exampleMeasureID, ItemCount: 2, OccurredAt: exampleAt},
}, inboxPushEmail))

// StoreMeasureIssuedParams 是 store_measure_issued 的參數，取自 seller.MeasureCreated（種類 WARNING 或 TERMINATION）。
type StoreMeasureIssuedParams struct {
	StoreID    int64     `json:"store_id"`
	MeasureID  uuid.UUID `json:"measure_id"`
	Kind       string    `json:"kind"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storeMeasureIssued = Define(withStore(Definition[StoreMeasureIssuedParams]{
	Name:              "store_measure_issued",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureCreated.MeasureID",
	IdempotencyKey:    func(p StoreMeasureIssuedParams) string { return p.MeasureID.String() },
	OccurredAt:        func(p StoreMeasureIssuedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p StoreMeasureIssuedParams) (*Expansion, error) {
		if p.Kind != MeasureKindWarning && p.Kind != MeasureKindTermination {
			return nil, badParams("措施種類 %q 不是 WARNING 或 TERMINATION", p.Kind)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text: func(l uitext.Locale, p StoreMeasureIssuedParams) (Content, error) {
		t := messageTime(l, p.OccurredAt)
		switch p.Kind {
		case MeasureKindWarning:
			return Content{Title: text.StoreMeasureIssuedWarningTitle(l), Body: text.StoreMeasureIssuedWarningBody(l, text.StoreMeasureIssuedWarningBodyArgs{Time: t})}, nil
		case MeasureKindTermination:
			return Content{Title: text.StoreMeasureIssuedTerminationTitle(l), Body: text.StoreMeasureIssuedTerminationBody(l, text.StoreMeasureIssuedTerminationBodyArgs{Time: t})}, nil
		}
		return Content{}, fmt.Errorf("措施種類 %q 沒有文案", p.Kind)
	},
	DeepLink: func(p StoreMeasureIssuedParams) string { return measurePath(p.MeasureID) },
	Example:  StoreMeasureIssuedParams{StoreID: 11, MeasureID: exampleMeasureID, Kind: MeasureKindWarning, OccurredAt: exampleAt},
}, inboxPushEmail))

// StoreMeasureRevisedParams 是 store_measure_revised 的參數，取自 seller.MeasureRevised。
type StoreMeasureRevisedParams struct {
	StoreID    int64     `json:"store_id"`
	MeasureID  uuid.UUID `json:"measure_id"`
	Revision   int32     `json:"revision"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storeMeasureRevised = Define(withStore(Definition[StoreMeasureRevisedParams]{
	Name:              "store_measure_revised",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureRevised.MeasureID 與 Revision",
	IdempotencyKey: func(p StoreMeasureRevisedParams) string {
		return p.MeasureID.String() + "/" + strconv.FormatInt(int64(p.Revision), 10)
	},
	OccurredAt: func(p StoreMeasureRevisedParams) time.Time { return p.OccurredAt },
	Expand:     storeExpand(func(p StoreMeasureRevisedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreMeasureRevisedParams) (Content, error) {
		return Content{
			Title: text.StoreMeasureRevisedTitle(l),
			Body:  text.StoreMeasureRevisedBody(l, text.StoreMeasureRevisedBodyArgs{Time: messageTime(l, p.OccurredAt), Revision: int(p.Revision)}),
		}, nil
	},
	DeepLink: func(p StoreMeasureRevisedParams) string { return measurePath(p.MeasureID) },
	Example:  StoreMeasureRevisedParams{StoreID: 11, MeasureID: exampleMeasureID, Revision: 2, OccurredAt: exampleAt},
}, inboxPush))

// StoreMeasureStateParams 是 store_measure_ended 與 store_measure_revoked 的參數：措施結束或撤銷。接受解除請求或申覆而結束、撤銷時，
// 這次處置由 store_measure_request_decided 通知（D3），intake 不登記這兩個型別。
type StoreMeasureStateParams struct {
	StoreID    int64     `json:"store_id"`
	MeasureID  uuid.UUID `json:"measure_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storeMeasureEnded = Define(withStore(Definition[StoreMeasureStateParams]{
	Name:              "store_measure_ended",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureEnded.MeasureID",
	IdempotencyKey:    func(p StoreMeasureStateParams) string { return p.MeasureID.String() },
	OccurredAt:        func(p StoreMeasureStateParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StoreMeasureStateParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreMeasureStateParams) (Content, error) {
		return Content{Title: text.StoreMeasureEndedTitle(l), Body: text.StoreMeasureEndedBody(l, text.StoreMeasureEndedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(p StoreMeasureStateParams) string { return measurePath(p.MeasureID) },
	Example:  StoreMeasureStateParams{StoreID: 11, MeasureID: exampleMeasureID, OccurredAt: exampleAt},
}, inboxPushEmail))

var storeMeasureRevoked = Define(withStore(Definition[StoreMeasureStateParams]{
	Name:              "store_measure_revoked",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureRevoked.MeasureID",
	IdempotencyKey:    func(p StoreMeasureStateParams) string { return p.MeasureID.String() },
	OccurredAt:        func(p StoreMeasureStateParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StoreMeasureStateParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StoreMeasureStateParams) (Content, error) {
		return Content{Title: text.StoreMeasureRevokedTitle(l), Body: text.StoreMeasureRevokedBody(l, text.StoreMeasureRevokedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(p StoreMeasureStateParams) string { return measurePath(p.MeasureID) },
	Example:  StoreMeasureStateParams{StoreID: 11, MeasureID: exampleMeasureID, OccurredAt: exampleAt},
}, inboxPushEmail))

// StoreMeasureRequestDecidedParams 是 store_measure_request_decided 的參數：申覆或解除請求的裁定（接受或駁回，D3）；接受時接著的撤銷或結束不另通知。
type StoreMeasureRequestDecidedParams struct {
	StoreID     int64     `json:"store_id"`
	MeasureID   uuid.UUID `json:"measure_id"`
	RequestID   uuid.UUID `json:"request_id"`
	RequestKind string    `json:"request_kind"`
	Outcome     string    `json:"outcome"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func requestDecidedText(l uitext.Locale, p StoreMeasureRequestDecidedParams) (Content, error) {
	t := messageTime(l, p.OccurredAt)
	switch {
	case p.RequestKind == RequestKindAppeal && p.Outcome == RequestOutcomeAccepted:
		return Content{
			Title: text.StoreMeasureRequestDecidedAppealAcceptedTitle(l),
			Body:  text.StoreMeasureRequestDecidedAppealAcceptedBody(l, text.StoreMeasureRequestDecidedAppealAcceptedBodyArgs{Time: t}),
		}, nil
	case p.RequestKind == RequestKindAppeal && p.Outcome == RequestOutcomeRejected:
		return Content{
			Title: text.StoreMeasureRequestDecidedAppealRejectedTitle(l),
			Body:  text.StoreMeasureRequestDecidedAppealRejectedBody(l, text.StoreMeasureRequestDecidedAppealRejectedBodyArgs{Time: t}),
		}, nil
	case p.RequestKind == RequestKindRemediation && p.Outcome == RequestOutcomeAccepted:
		return Content{
			Title: text.StoreMeasureRequestDecidedRemediationAcceptedTitle(l),
			Body:  text.StoreMeasureRequestDecidedRemediationAcceptedBody(l, text.StoreMeasureRequestDecidedRemediationAcceptedBodyArgs{Time: t}),
		}, nil
	case p.RequestKind == RequestKindRemediation && p.Outcome == RequestOutcomeRejected:
		return Content{
			Title: text.StoreMeasureRequestDecidedRemediationRejectedTitle(l),
			Body:  text.StoreMeasureRequestDecidedRemediationRejectedBody(l, text.StoreMeasureRequestDecidedRemediationRejectedBodyArgs{Time: t}),
		}, nil
	}
	return Content{}, fmt.Errorf("請求種類 %q、裁定 %q 沒有文案", p.RequestKind, p.Outcome)
}

var storeMeasureRequestDecided = Define(withStore(Definition[StoreMeasureRequestDecidedParams]{
	Name:              "store_measure_request_decided",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.MeasureRequestDecided.RequestID",
	IdempotencyKey:    func(p StoreMeasureRequestDecidedParams) string { return p.RequestID.String() },
	OccurredAt:        func(p StoreMeasureRequestDecidedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p StoreMeasureRequestDecidedParams) (*Expansion, error) {
		if p.RequestKind != RequestKindAppeal && p.RequestKind != RequestKindRemediation {
			return nil, badParams("請求種類 %q 不是 APPEAL 或 REMEDIATION", p.RequestKind)
		}
		if p.Outcome != RequestOutcomeAccepted && p.Outcome != RequestOutcomeRejected {
			return nil, badParams("裁定 %q 不是 ACCEPTED 或 REJECTED", p.Outcome)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text:     requestDecidedText,
	DeepLink: func(p StoreMeasureRequestDecidedParams) string { return measurePath(p.MeasureID) },
	Example:  StoreMeasureRequestDecidedParams{StoreID: 11, MeasureID: exampleMeasureID, RequestID: exampleRequestID, RequestKind: RequestKindAppeal, Outcome: RequestOutcomeRejected, OccurredAt: exampleAt},
}, inboxPushEmail))

// KYCDecisionParams 是 kyc_approved 與 kyc_rejected 的參數，取自 seller.KYCApproved、KYCRejected（不帶退回理由）。
type KYCDecisionParams struct {
	StoreID       int64     `json:"store_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

var kycApproved = Define(withStore(Definition[KYCDecisionParams]{
	Name:              "kyc_approved",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.KYCApproved.ApplicationID",
	IdempotencyKey:    func(p KYCDecisionParams) string { return p.ApplicationID.String() },
	OccurredAt:        func(p KYCDecisionParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p KYCDecisionParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p KYCDecisionParams) (Content, error) {
		return Content{Title: text.KycApprovedTitle(l), Body: text.KycApprovedBody(l, text.KycApprovedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(KYCDecisionParams) string { return sellerOnboardingPath },
	Example:  KYCDecisionParams{StoreID: 11, ApplicationID: exampleGovernanceEventID, OccurredAt: exampleAt},
}, inboxPushEmail))

var kycRejected = Define(withStore(Definition[KYCDecisionParams]{
	Name:              "kyc_rejected",
	StateHint:         model.StateHintSellerStatus,
	IdempotencySource: "seller.KYCRejected.ApplicationID",
	IdempotencyKey:    func(p KYCDecisionParams) string { return p.ApplicationID.String() },
	OccurredAt:        func(p KYCDecisionParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p KYCDecisionParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p KYCDecisionParams) (Content, error) {
		return Content{Title: text.KycRejectedTitle(l), Body: text.KycRejectedBody(l, text.KycRejectedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(KYCDecisionParams) string { return sellerOnboardingPath },
	Example:  KYCDecisionParams{StoreID: 11, ApplicationID: exampleGovernanceEventID, OccurredAt: exampleAt},
}, inboxPushEmail))

// StoreCaseInquiryParams 是 store_case_inquiry 的參數，取自 seller.CaseInquiryCreated（不帶詢問內容）。
type StoreCaseInquiryParams struct {
	StoreID    int64     `json:"store_id"`
	InquiryID  uuid.UUID `json:"inquiry_id"`
	Kind       string    `json:"kind"`
	DueAt      time.Time `json:"due_at"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storeCaseInquiry = Define(withStore(Definition[StoreCaseInquiryParams]{
	Name:              "store_case_inquiry",
	IdempotencySource: "seller.CaseInquiryCreated.InquiryID",
	IdempotencyKey:    func(p StoreCaseInquiryParams) string { return p.InquiryID.String() },
	OccurredAt:        func(p StoreCaseInquiryParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p StoreCaseInquiryParams) (*Expansion, error) {
		if p.Kind != InquiryKindRecords && p.Kind != InquiryKindExplanation {
			return nil, badParams("詢問種類 %q 不是 RECORDS 或 EXPLANATION", p.Kind)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text: func(l uitext.Locale, p StoreCaseInquiryParams) (Content, error) {
		due := messageTime(l, p.DueAt)
		switch p.Kind {
		case InquiryKindRecords:
			return Content{Title: text.StoreCaseInquiryTitle(l), Body: text.StoreCaseInquiryRecordsBody(l, text.StoreCaseInquiryRecordsBodyArgs{Due: due})}, nil
		case InquiryKindExplanation:
			return Content{Title: text.StoreCaseInquiryTitle(l), Body: text.StoreCaseInquiryExplanationBody(l, text.StoreCaseInquiryExplanationBodyArgs{Due: due})}, nil
		}
		return Content{}, fmt.Errorf("詢問種類 %q 沒有文案", p.Kind)
	},
	DeepLink: func(p StoreCaseInquiryParams) string { return "/seller/inquiries/" + p.InquiryID.String() },
	Example:  StoreCaseInquiryParams{StoreID: 11, InquiryID: exampleRequestID, Kind: InquiryKindRecords, DueAt: exampleDueAt, OccurredAt: exampleAt},
}, inboxPushEmail))

// StorePendingDeletionParams 是 store_pending_deletion（確認信，只寄 email）的參數，取自 seller.StorePendingDeletion；信箱位址在寄送時才向 identity 讀，不在任何參數裡。
type StorePendingDeletionParams struct {
	StoreID    int64     `json:"store_id"`
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

var storePendingDeletion = Define(withStore(Definition[StorePendingDeletionParams]{
	Name:              "store_pending_deletion",
	IdempotencySource: "seller.StorePendingDeletion.EventID",
	IdempotencyKey:    func(p StorePendingDeletionParams) string { return p.EventID.String() },
	OccurredAt:        func(p StorePendingDeletionParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p StorePendingDeletionParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p StorePendingDeletionParams) (Content, error) {
		return Content{Title: text.StorePendingDeletionTitle(l), Body: text.StorePendingDeletionBody(l, text.StorePendingDeletionBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	Example: StorePendingDeletionParams{StoreID: 11, EventID: exampleGovernanceEventID, OccurredAt: exampleAt},
}, emailOnly))

// BuyerDeletionRequestedParams 是 buyer_deletion_requested（確認信，只寄 email）的參數，取自 buyer.BuyerDeletionRequested。
type BuyerDeletionRequestedParams struct {
	BuyerID    int64     `json:"buyer_id"`
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

var buyerDeletionRequested = Define(Definition[BuyerDeletionRequestedParams]{
	Name:              "buyer_deletion_requested",
	Audiences:         []model.Audience{model.AudienceBuyer},
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          emailOnly,
	IdempotencySource: "buyer.BuyerDeletionRequested.EventID",
	IdempotencyKey:    func(p BuyerDeletionRequestedParams) string { return p.EventID.String() },
	OccurredAt:        func(p BuyerDeletionRequestedParams) time.Time { return p.OccurredAt },
	Expand:            buyerExpand(func(p BuyerDeletionRequestedParams) int64 { return p.BuyerID }),
	Text: func(l uitext.Locale, p BuyerDeletionRequestedParams) (Content, error) {
		return Content{Title: text.BuyerDeletionRequestedTitle(l), Body: text.BuyerDeletionRequestedBody(l, text.BuyerDeletionRequestedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	Example: BuyerDeletionRequestedParams{BuyerID: 42, EventID: exampleGovernanceEventID, OccurredAt: exampleAt},
})

// PayoutAccountActivatedParams 是 payout_account_activated 的參數：收款帳戶第一次啟用（payment.PayoutAccountChanged 的 FirstActivation）。
type PayoutAccountActivatedParams struct {
	StoreID    int64     `json:"store_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

var payoutAccountActivated = Define(Definition[PayoutAccountActivatedParams]{
	Name:              "payout_account_activated",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryPayout,
	Presentation:      model.PresentationPayout,
	StateHint:         model.StateHintSellerStatus,
	Channels:          inboxPushEmail,
	IdempotencySource: "payment.PayoutAccountChanged.StoreID（只在第一次啟用發布一次）",
	IdempotencyKey:    func(p PayoutAccountActivatedParams) string { return strconv.FormatInt(p.StoreID, 10) },
	OccurredAt:        func(p PayoutAccountActivatedParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p PayoutAccountActivatedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, p PayoutAccountActivatedParams) (Content, error) {
		return Content{Title: text.PayoutAccountActivatedTitle(l), Body: text.PayoutAccountActivatedBody(l, text.PayoutAccountActivatedBodyArgs{Time: messageTime(l, p.OccurredAt)})}, nil
	},
	DeepLink: func(PayoutAccountActivatedParams) string { return sellerWalletPath },
	Example:  PayoutAccountActivatedParams{StoreID: 11, OccurredAt: exampleAt},
})

// PayoutAccountDeadlineParams 是 payout_account_deadline 的參數，取自 payment.PayoutDeadlineApproaching；OccurredAt 是掃描的時間。
type PayoutAccountDeadlineParams struct {
	StoreID    int64     `json:"store_id"`
	Deadline   time.Time `json:"deadline"`
	Tier       string    `json:"tier"`
	OccurredAt time.Time `json:"occurred_at"`
}

var payoutAccountDeadline = Define(Definition[PayoutAccountDeadlineParams]{
	Name:              "payout_account_deadline",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryPayout,
	Presentation:      model.PresentationPayout,
	Channels:          inboxPushEmail,
	IdempotencySource: "payment.PayoutDeadlineApproaching 的 StoreID、Deadline 與 Tier",
	IdempotencyKey: func(p PayoutAccountDeadlineParams) string {
		return strconv.FormatInt(p.StoreID, 10) + "/" + strconv.FormatInt(p.Deadline.Unix(), 10) + "/" + p.Tier
	},
	OccurredAt: func(p PayoutAccountDeadlineParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p PayoutAccountDeadlineParams) (*Expansion, error) {
		if p.Tier != DeadlineTierSevenDays && p.Tier != DeadlineTierOneDay {
			return nil, badParams("補件期限檔位 %q 不是 SEVEN_DAYS 或 ONE_DAY", p.Tier)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text: func(l uitext.Locale, p PayoutAccountDeadlineParams) (Content, error) {
		deadline := messageTime(l, p.Deadline)
		switch p.Tier {
		case DeadlineTierSevenDays:
			return Content{
				Title: text.PayoutAccountDeadlineSevenDaysTitle(l),
				Body:  text.PayoutAccountDeadlineSevenDaysBody(l, text.PayoutAccountDeadlineSevenDaysBodyArgs{Deadline: deadline}),
			}, nil
		case DeadlineTierOneDay:
			return Content{
				Title: text.PayoutAccountDeadlineOneDayTitle(l),
				Body:  text.PayoutAccountDeadlineOneDayBody(l, text.PayoutAccountDeadlineOneDayBodyArgs{Deadline: deadline}),
			}, nil
		}
		return Content{}, fmt.Errorf("補件期限檔位 %q 沒有文案", p.Tier)
	},
	DeepLink: func(PayoutAccountDeadlineParams) string { return sellerWalletPath },
	Example:  PayoutAccountDeadlineParams{StoreID: 11, Deadline: exampleDueAt, Tier: DeadlineTierSevenDays, OccurredAt: exampleAt},
})

// DelegationOperationCompletedParams 是 delegation_operation_completed 的參數：只有稽核列 ID、目標店家與帳號、權限與動作（列舉）和時間；
// 不帶代行者的名稱、動作標籤、資源名稱、事由與案件參照，文案由動作的列舉組成。
type DelegationOperationCompletedParams struct {
	AuditLogID            uuid.UUID `json:"audit_log_id"`
	TargetStoreID         int64     `json:"target_store_id"`
	TargetSellerAccountID int64     `json:"target_seller_account_id"`
	Capability            string    `json:"capability"`
	Action                string    `json:"action"`
	OccurredAt            time.Time `json:"occurred_at"`
}

// delegationActionText 是代行動作（delegation.AuditAction 的值）的片語；TestEveryDelegationActionHasText 對照擁有者的全部動作。
var delegationActionText = map[string]func(uitext.Locale) uitext.Message{
	"STOREFRONT_UPDATE":       text.DelegationActionStorefrontUpdate,
	"STOREFRONT_LOGO_UPDATE":  text.DelegationActionStorefrontLogoUpdate,
	"STOREFRONT_COVER_UPDATE": text.DelegationActionStorefrontCoverUpdate,
	"PRODUCT_CREATE":          text.DelegationActionProductCreate,
	"PRODUCT_UPDATE":          text.DelegationActionProductUpdate,
	"PRODUCT_PUBLISH":         text.DelegationActionProductPublish,
	"PRODUCT_UNPUBLISH":       text.DelegationActionProductUnpublish,
	"PRODUCT_BATCH_UNPUBLISH": text.DelegationActionProductBatchUnpublish,
	"POST_CREATE":             text.DelegationActionPostCreate,
	"POST_UPDATE":             text.DelegationActionPostUpdate,
	"POST_PUBLISH":            text.DelegationActionPostPublish,
	"POST_UNPUBLISH":          text.DelegationActionPostUnpublish,
	"POST_SCHEDULE":           text.DelegationActionPostSchedule,
	"ORDER_PII_REVEAL":        text.DelegationActionOrderPiiReveal,
	"STOREFRONT_READ":         text.DelegationActionStorefrontRead,
	"PRODUCT_READ":            text.DelegationActionProductRead,
	"POST_READ":               text.DelegationActionPostRead,
	"ORDER_READ":              text.DelegationActionOrderRead,
	"AUDIT_CORRECTION":        text.DelegationActionAuditCorrection,
	"MEDIA_UPLOAD":            text.DelegationActionMediaUpload,
}

var delegationOperationCompleted = Define(Definition[DelegationOperationCompletedParams]{
	Name:              "delegation_operation_completed",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          inboxPush,
	IdempotencySource: "delegation.DelegationOperationCompleted.AuditLogID",
	IdempotencyKey:    func(p DelegationOperationCompletedParams) string { return p.AuditLogID.String() },
	OccurredAt:        func(p DelegationOperationCompletedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p DelegationOperationCompletedParams) (*Expansion, error) {
		if _, ok := delegationActionText[p.Action]; !ok {
			return nil, badParams("代行動作 %q 沒有文案", p.Action)
		}
		return StoreRecipient(ctx, r, p.TargetStoreID)
	},
	Text: func(l uitext.Locale, p DelegationOperationCompletedParams) (Content, error) {
		label, ok := delegationActionText[p.Action]
		if !ok {
			return Content{}, fmt.Errorf("代行動作 %q 沒有文案", p.Action)
		}
		action := label(l)
		return Content{
			Title:     text.DelegationOperationCompletedTitle(l),
			Body:      text.DelegationOperationCompletedBody(l, text.DelegationOperationCompletedBodyArgs{Time: messageTime(l, p.OccurredAt), Action: action.Text}),
			Fragments: []uitext.Message{action},
		}, nil
	},
	Example: DelegationOperationCompletedParams{
		AuditLogID: exampleGovernanceEventID, TargetStoreID: 11, TargetSellerAccountID: 42, Capability: "products.write", Action: "PRODUCT_UPDATE", OccurredAt: exampleAt,
	},
})

// RfqInvitedParams 是 rfq_invited 的參數，取自 procurement.RfqPublished 的一筆邀請；不帶買家與詢價內容。邀請對象是店家，賣家端沒有可去的畫面。
// 偏好分類是詢價邀請的 PROCUREMENT（D29：店家、預設開、可關）。
type RfqInvitedParams struct {
	StoreID       int64     `json:"store_id"`
	PublicationID uuid.UUID `json:"publication_id"`
	InviteID      uuid.UUID `json:"invite_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

var rfqInvited = Define(Definition[RfqInvitedParams]{
	Name:              "rfq_invited",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryProcurement,
	Presentation:      model.PresentationProcurement,
	Channels:          inboxPush,
	IdempotencySource: "procurement.RfqPublished.Invites[].InviteID",
	IdempotencyKey:    func(p RfqInvitedParams) string { return p.InviteID.String() },
	OccurredAt:        func(p RfqInvitedParams) time.Time { return p.OccurredAt },
	Expand:            storeExpand(func(p RfqInvitedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, _ RfqInvitedParams) (Content, error) {
		return Content{Title: text.RfqInvitedTitle(l), Body: text.RfqInvitedBody(l)}, nil
	},
	Example: RfqInvitedParams{StoreID: 11, PublicationID: exampleReportID, InviteID: exampleRequestID, OccurredAt: exampleAt},
})

func contentSourceLabel(l uitext.Locale, source string) (uitext.Message, error) {
	switch source {
	case ContentSourceSocial:
		return text.ContentSourceSocial(l), nil
	case ContentSourceReview:
		return text.ContentSourceReview(l), nil
	case ContentSourceMessage:
		return text.ContentSourceMessage(l), nil
	}
	return uitext.Message{}, fmt.Errorf("內容來源 %q 沒有文案", source)
}

func checkAppealPath(p policy.AppealPath) error {
	if !p.Valid() {
		return fmt.Errorf("申訴途徑 %q 沒有文案", p)
	}
	return nil
}

// ContentHiddenParams 是 content_hidden 的參數（作者通知，只給買家作者）：取自 social.ContentHidden、review.ReviewHidden、im.MessageHidden，
// Source 是來源；不帶管理員說明。AppealPath 目前只有 NONE（買家沒有申訴流程，不放申訴說明）。
type ContentHiddenParams struct {
	SanctionID uuid.UUID         `json:"sanction_id"`
	BuyerID    int64             `json:"buyer_id"`
	Source     string            `json:"source"`
	Category   policy.Category   `json:"category"`
	AppealPath policy.AppealPath `json:"appeal_path"`
	OccurredAt time.Time         `json:"occurred_at"`
}

// ContentRestoredParams 是 content_restored 的參數：取自 social.ContentRestored、review.ReviewRestored、im.MessageRestored。
type ContentRestoredParams struct {
	SanctionID uuid.UUID `json:"sanction_id"`
	BuyerID    int64     `json:"buyer_id"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AuthorWarnedParams 是 author_warned 的參數：取自 social.BuyerWarned。
type AuthorWarnedParams struct {
	SanctionID uuid.UUID         `json:"sanction_id"`
	BuyerID    int64             `json:"buyer_id"`
	Category   policy.Category   `json:"category"`
	AppealPath policy.AppealPath `json:"appeal_path"`
	OccurredAt time.Time         `json:"occurred_at"`
}

func authorDefinition[P any](d Definition[P], buyerID func(P) int64, sanctionID func(P) uuid.UUID, occurredAt func(P) time.Time) Definition[P] {
	d.Audiences = []model.Audience{model.AudienceBuyer}
	d.Category = model.CategoryAccount
	d.Presentation = model.PresentationAccount
	d.Channels = inboxPush
	d.IdempotencyKey = func(p P) string { return sanctionID(p).String() }
	d.OccurredAt = occurredAt
	d.Expand = buyerExpand(buyerID)
	return d
}

var contentHidden = Define(authorDefinition(Definition[ContentHiddenParams]{
	Name:              "content_hidden",
	IdempotencySource: "social.ContentHidden、review.ReviewHidden、im.MessageHidden 的 SanctionID",
	Text: func(l uitext.Locale, p ContentHiddenParams) (Content, error) {
		if err := checkAppealPath(p.AppealPath); err != nil {
			return Content{}, err
		}
		source, err := contentSourceLabel(l, p.Source)
		if err != nil {
			return Content{}, err
		}
		reason, err := categoryLabel(l, p.Category)
		if err != nil {
			return Content{}, err
		}
		return Content{
			Title:     text.ContentHiddenTitle(l),
			Body:      text.ContentHiddenBody(l, text.ContentHiddenBodyArgs{Source: source.Text, Time: messageTime(l, p.OccurredAt), Reason: reason.Text}),
			Fragments: []uitext.Message{source, reason},
		}, nil
	},
	Example: ContentHiddenParams{SanctionID: exampleSanctionID, BuyerID: 42, Source: ContentSourceSocial, Category: policy.Spam, AppealPath: policy.AppealNone, OccurredAt: exampleAt},
},
	func(p ContentHiddenParams) int64 { return p.BuyerID },
	func(p ContentHiddenParams) uuid.UUID { return p.SanctionID },
	func(p ContentHiddenParams) time.Time { return p.OccurredAt }))

var contentRestored = Define(authorDefinition(Definition[ContentRestoredParams]{
	Name:              "content_restored",
	IdempotencySource: "social.ContentRestored、review.ReviewRestored、im.MessageRestored 的 SanctionID",
	Text: func(l uitext.Locale, p ContentRestoredParams) (Content, error) {
		source, err := contentSourceLabel(l, p.Source)
		if err != nil {
			return Content{}, err
		}
		return Content{
			Title:     text.ContentRestoredTitle(l),
			Body:      text.ContentRestoredBody(l, text.ContentRestoredBodyArgs{Source: source.Text, Time: messageTime(l, p.OccurredAt)}),
			Fragments: []uitext.Message{source},
		}, nil
	},
	Example: ContentRestoredParams{SanctionID: exampleSanctionID, BuyerID: 42, Source: ContentSourceReview, OccurredAt: exampleAt},
},
	func(p ContentRestoredParams) int64 { return p.BuyerID },
	func(p ContentRestoredParams) uuid.UUID { return p.SanctionID },
	func(p ContentRestoredParams) time.Time { return p.OccurredAt }))

var authorWarned = Define(authorDefinition(Definition[AuthorWarnedParams]{
	Name:              "author_warned",
	IdempotencySource: "social.BuyerWarned.SanctionID",
	Text: func(l uitext.Locale, p AuthorWarnedParams) (Content, error) {
		if err := checkAppealPath(p.AppealPath); err != nil {
			return Content{}, err
		}
		reason, err := categoryLabel(l, p.Category)
		if err != nil {
			return Content{}, err
		}
		return Content{
			Title:     text.AuthorWarnedTitle(l),
			Body:      text.AuthorWarnedBody(l, text.AuthorWarnedBodyArgs{Time: messageTime(l, p.OccurredAt), Reason: reason.Text}),
			Fragments: []uitext.Message{reason},
		}, nil
	},
	Example: AuthorWarnedParams{SanctionID: exampleSanctionID, BuyerID: 42, Category: policy.Harassment, AppealPath: policy.AppealNone, OccurredAt: exampleAt},
},
	func(p AuthorWarnedParams) int64 { return p.BuyerID },
	func(p AuthorWarnedParams) uuid.UUID { return p.SanctionID },
	func(p AuthorWarnedParams) time.Time { return p.OccurredAt }))

func reporterSource(source string) error {
	if source != ReportSourceModeration && source != ReportSourceSeller {
		return badParams("檢舉來源 %q 不是 MODERATION 或 SELLER", source)
	}
	return nil
}

// ReportReceivedParams 是 report_received 與 store_report_received 的參數：檢舉人收到確認。ReportSource 是來源（MODERATION 或 SELLER），
// BuyerID 與 StoreID 恰好一個為正（買家檢舉人走 report_received、店家檢舉人走 store_report_received）；SupplementID 只有賣家來源的補充才有。
type ReportReceivedParams struct {
	ReportSource string     `json:"report_source"`
	ReportID     uuid.UUID  `json:"report_id"`
	SupplementID *uuid.UUID `json:"supplement_id,omitempty"`
	BuyerID      int64      `json:"buyer_id,omitempty"`
	StoreID      int64      `json:"store_id,omitempty"`
	OccurredAt   time.Time  `json:"occurred_at"`
}

func reportReceivedKey(p ReportReceivedParams) string {
	if p.SupplementID != nil {
		return p.ReportID.String() + "/" + p.SupplementID.String()
	}
	return p.ReportID.String()
}

// reportPath 只有賣家來源的檢舉有可去的畫面（website 的 /account/reports/{id} 讀賣家檢舉）。
func reportPath(source string, id uuid.UUID) string {
	if source == ReportSourceSeller {
		return "/account/reports/" + id.String()
	}
	return ""
}

func receivedText(l uitext.Locale, _ ReportReceivedParams) (Content, error) {
	return Content{Title: text.ReportReceivedTitle(l), Body: text.ReportReceivedBody(l)}, nil
}

var reportReceived = Define(Definition[ReportReceivedParams]{
	Name:              "report_received",
	Audiences:         []model.Audience{model.AudienceBuyer},
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          inboxPushEmail,
	IdempotencySource: "seller.ReportReceived 的 ReportID 與 SupplementID；moderation.ReportReceived.ReportID",
	IdempotencyKey:    reportReceivedKey,
	OccurredAt:        func(p ReportReceivedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ReportReceivedParams) (*Expansion, error) {
		if err := reporterSource(p.ReportSource); err != nil {
			return nil, err
		}
		if p.StoreID != 0 {
			return nil, badParams("買家檢舉人的通知不能帶 store_id（%d）", p.StoreID)
		}
		return BuyerRecipient(ctx, r, p.BuyerID)
	},
	Text:     receivedText,
	DeepLink: func(p ReportReceivedParams) string { return reportPath(p.ReportSource, p.ReportID) },
	Example:  ReportReceivedParams{ReportSource: ReportSourceSeller, ReportID: exampleReportID, BuyerID: 42, OccurredAt: exampleAt},
})

var storeReportReceived = Define(Definition[ReportReceivedParams]{
	Name:              "store_report_received",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          inboxPushEmail,
	IdempotencySource: "moderation.ReportReceived.ReportID（店家檢舉人）",
	IdempotencyKey:    reportReceivedKey,
	OccurredAt:        func(p ReportReceivedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ReportReceivedParams) (*Expansion, error) {
		if err := reporterSource(p.ReportSource); err != nil {
			return nil, err
		}
		if p.BuyerID != 0 {
			return nil, badParams("店家檢舉人的通知不能帶 buyer_id（%d）", p.BuyerID)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text:    receivedText,
	Example: ReportReceivedParams{ReportSource: ReportSourceModeration, ReportID: exampleReportID, StoreID: 11, OccurredAt: exampleAt},
})

// ReportResolvedParams 是 report_resolved 與 store_report_resolved 的參數（只由 fan-out 建立）：Outcome 只放結果的類別，不放處置種類與對象。
type ReportResolvedParams struct {
	ReportSource string    `json:"report_source"`
	ReportID     uuid.UUID `json:"report_id"`
	BuyerID      int64     `json:"buyer_id,omitempty"`
	StoreID      int64     `json:"store_id,omitempty"`
	Outcome      string    `json:"outcome"`
	OccurredAt   time.Time `json:"occurred_at"`
}

func reportOutcomeLabel(l uitext.Locale, outcome string) (uitext.Message, error) {
	switch outcome {
	case ReportOutcomeActionTaken:
		return text.ReportOutcomeActionTaken(l), nil
	case ReportOutcomeNoViolation:
		return text.ReportOutcomeNoViolation(l), nil
	case ReportOutcomeInsufficient:
		return text.ReportOutcomeInsufficient(l), nil
	case ReportOutcomeDuplicate:
		return text.ReportOutcomeDuplicate(l), nil
	case ReportOutcomeAbusive:
		return text.ReportOutcomeAbusive(l), nil
	}
	return uitext.Message{}, fmt.Errorf("檢舉結果 %q 沒有文案", outcome)
}

func resolvedText(l uitext.Locale, p ReportResolvedParams) (Content, error) {
	outcome, err := reportOutcomeLabel(l, p.Outcome)
	if err != nil {
		return Content{}, err
	}
	return Content{
		Title:     text.ReportResolvedTitle(l),
		Body:      text.ReportResolvedBody(l, text.ReportResolvedBodyArgs{Outcome: outcome.Text}),
		Fragments: []uitext.Message{outcome},
	}, nil
}

var reportResolved = Define(Definition[ReportResolvedParams]{
	Name:         "report_resolved",
	Audiences:    []model.Audience{model.AudienceBuyer},
	Category:     model.CategoryAccount,
	Presentation: model.PresentationAccount,
	Channels:     inboxPush,
	Spawned:      true,
	Expand: func(ctx context.Context, r Resolver, p ReportResolvedParams) (*Expansion, error) {
		if err := reporterSource(p.ReportSource); err != nil {
			return nil, err
		}
		if _, err := reportOutcomeLabel(uitext.ZhTW, p.Outcome); err != nil {
			return nil, badParams("%v", err)
		}
		return BuyerRecipient(ctx, r, p.BuyerID)
	},
	Text:     resolvedText,
	DeepLink: func(p ReportResolvedParams) string { return reportPath(p.ReportSource, p.ReportID) },
	Example:  ReportResolvedParams{ReportSource: ReportSourceSeller, ReportID: exampleReportID, BuyerID: 42, Outcome: ReportOutcomeActionTaken, OccurredAt: exampleAt},
})

var storeReportResolved = Define(Definition[ReportResolvedParams]{
	Name:         "store_report_resolved",
	Audiences:    []model.Audience{model.AudienceStore},
	Category:     model.CategoryAccount,
	Presentation: model.PresentationAccount,
	Channels:     inboxPush,
	Spawned:      true,
	Expand: func(ctx context.Context, r Resolver, p ReportResolvedParams) (*Expansion, error) {
		if err := reporterSource(p.ReportSource); err != nil {
			return nil, err
		}
		if _, err := reportOutcomeLabel(uitext.ZhTW, p.Outcome); err != nil {
			return nil, badParams("%v", err)
		}
		return StoreRecipient(ctx, r, p.StoreID)
	},
	Text:    resolvedText,
	Example: ReportResolvedParams{ReportSource: ReportSourceModeration, ReportID: exampleReportID, StoreID: 11, Outcome: ReportOutcomeNoViolation, OccurredAt: exampleAt},
})

// SellerReportResolvedParams 是 fan-out 母意圖 seller_report_resolved 的參數，取自 seller.ReportResolved：一筆通報一位買家檢舉人，母意圖展開成一個 report_resolved 子意圖。
type SellerReportResolvedParams struct {
	ReportID   uuid.UUID `json:"report_id"`
	BuyerID    int64     `json:"buyer_id"`
	Outcome    string    `json:"outcome"`
	OccurredAt time.Time `json:"occurred_at"`
}

var sellerReportResolved = Define(Definition[SellerReportResolvedParams]{
	Name:              "seller_report_resolved",
	IdempotencySource: "seller.ReportResolved.ReportID",
	IdempotencyKey:    func(p SellerReportResolvedParams) string { return p.ReportID.String() },
	OccurredAt:        func(p SellerReportResolvedParams) time.Time { return p.OccurredAt },
	Broadcast: &Broadcast[SellerReportResolvedParams]{
		Child: "report_resolved",
		Targets: func(_ context.Context, _ Resolver, p SellerReportResolvedParams, req PageRequest) (TargetPage, error) {
			if p.BuyerID <= 0 {
				return TargetPage{}, badParams("檢舉人買家號碼 %d 不合法", p.BuyerID)
			}
			if req.After >= p.BuyerID {
				return TargetPage{Next: req.After, Done: true}, nil
			}
			return TargetPage{
				Targets: []Target{{ID: p.BuyerID, Params: ReportResolvedParams{
					ReportSource: ReportSourceSeller, ReportID: p.ReportID, BuyerID: p.BuyerID, Outcome: p.Outcome, OccurredAt: p.OccurredAt,
				}}},
				Next: p.BuyerID, Done: true,
			}, nil
		},
	},
	Example: SellerReportResolvedParams{ReportID: exampleReportID, BuyerID: 42, Outcome: ReportOutcomeActionTaken, OccurredAt: exampleAt},
})

// ModerationCaseClosedParams 是 fan-out 母意圖 moderation_case_closed_buyers 與 moderation_case_closed_stores 的參數，取自 moderation.CaseClosedEvent：
// 只有案件 ID 與結果（moderation 的 VIOLATION 或 NO_VIOLATION）；檢舉人由 worker 分頁讀。
type ModerationCaseClosedParams struct {
	CaseID     uuid.UUID `json:"case_id"`
	Outcome    string    `json:"outcome"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ModerationCaseClosedParents 是 moderation.CaseClosedEvent 要登記的兩個母意圖：買家檢舉人與店家檢舉人各一個（子型別的受眾不同）。
var ModerationCaseClosedParents = []string{"moderation_case_closed_buyers", "moderation_case_closed_stores"}

func moderationOutcome(outcome string) (string, error) {
	switch outcome {
	case ModerationOutcomeViolation:
		return ReportOutcomeActionTaken, nil
	case ModerationOutcomeNoViolation:
		return ReportOutcomeNoViolation, nil
	}
	return "", badParams("案件結果 %q 不是 VIOLATION 或 NO_VIOLATION", outcome)
}

// caseReporterTargets 分頁展開案件 audience 那一邊的檢舉人：Target.ID 與游標是檢舉人號碼（buyer_id 或 store_id，遞增），每個母意圖只讀自己那一邊。
func caseReporterTargets(ctx context.Context, r Resolver, p ModerationCaseClosedParams, req PageRequest, audience model.Audience) (TargetPage, error) {
	outcome, err := moderationOutcome(p.Outcome)
	if err != nil {
		return TargetPage{}, err
	}
	refs, err := r.CaseReporters(ctx, p.CaseID, audience, req.After, req.Limit)
	if err != nil {
		return TargetPage{}, fmt.Errorf("讀取案件的檢舉人：%w", err)
	}
	page := TargetPage{Next: req.After, Done: len(refs) < req.Limit}
	for _, ref := range refs {
		if ref.ID <= page.Next {
			return TargetPage{}, fmt.Errorf("案件 %s 的檢舉人號碼 %d 沒有大於游標 %d", p.CaseID, ref.ID, page.Next)
		}
		page.Next = ref.ID
		child := ReportResolvedParams{ReportSource: ReportSourceModeration, ReportID: ref.ReportID, Outcome: outcome, OccurredAt: p.OccurredAt}
		if audience == model.AudienceBuyer {
			child.BuyerID = ref.ID
		} else {
			child.StoreID = ref.ID
		}
		page.Targets = append(page.Targets, Target{ID: ref.ID, Params: child})
	}
	return page, nil
}

func caseClosedParent(name, child string, audience model.Audience) Entry {
	return Define(Definition[ModerationCaseClosedParams]{
		Name:              name,
		IdempotencySource: "moderation.CaseClosedEvent.CaseID",
		IdempotencyKey:    func(p ModerationCaseClosedParams) string { return p.CaseID.String() },
		OccurredAt:        func(p ModerationCaseClosedParams) time.Time { return p.OccurredAt },
		Broadcast: &Broadcast[ModerationCaseClosedParams]{
			Child: child,
			Targets: func(ctx context.Context, r Resolver, p ModerationCaseClosedParams, req PageRequest) (TargetPage, error) {
				return caseReporterTargets(ctx, r, p, req, audience)
			},
		},
		Example: ModerationCaseClosedParams{CaseID: exampleCaseID, Outcome: ModerationOutcomeViolation, OccurredAt: exampleAt},
	})
}

var (
	moderationCaseClosedBuyers = caseClosedParent(ModerationCaseClosedParents[0], "report_resolved", model.AudienceBuyer)
	moderationCaseClosedStores = caseClosedParent(ModerationCaseClosedParents[1], "store_report_resolved", model.AudienceStore)
)
