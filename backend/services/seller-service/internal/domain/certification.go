package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/looplab/fsm"
)

type CertificationApplication struct {
	UpdatedAt            time.Time
	CreatedAt            time.Time
	pendingNow           time.Time
	CompanyRegisterDate  *time.Time
	RejectedAt           *time.Time
	ForwardedAt          *time.Time
	SubmittedAt          *time.Time
	BirthDate            *time.Time
	IDCardIssueDate      *time.Time
	certFSM              *fsm.FSM
	CompanyName          string
	CompanyAddressDetail string
	StoreDescription     string
	IDCardFrontAssetID   string
	IDCardBackAssetID    string
	BankBookAssetID      string
	ApplicationNo        string
	CancelReason         string
	BankName             string
	BankCode             string
	BankAccount          string
	CompanyType          string
	SellerType           string
	TaxID                string
	IDCardIssueLocation  string
	CompanyCity          string
	CompanyDistrict      string
	EnglishName          string
	CompanyEmail         string
	CompanyPhone         string
	ProductType          string
	RealName             string
	IDCardNo             string
	IDCardIssueType      string
	StepData             json.RawMessage
	DocumentAssetIDs     []string
	TotalSteps           int
	CurrentStep          int
	Version              int
	ID                   int64
	Status               CertificationStatus
	AccountID            int64
	IsProcessedFood      bool
	FoodBusinessRegNo    string
	PayuniWebsiteURL     string
}

func (a *CertificationApplication) InitFSM() {
	a.certFSM = newCertificationFSM(a, a.Status.String())
}

func (a *CertificationApplication) Submit(now time.Time) error {
	return fsmFireEvent(a.certFSM, EventSubmit, now, a)
}

func (a *CertificationApplication) Forward(now time.Time) error {
	return fsmFireEvent(a.certFSM, EventForward, now, a)
}

func (a *CertificationApplication) ForwardFailed(now time.Time) error {
	return fsmFireEvent(a.certFSM, EventForwardFailed, now, a)
}

func (a *CertificationApplication) Reject(now time.Time) error {
	return fsmFireEvent(a.certFSM, EventReject, now, a)
}

func (a *CertificationApplication) Cancel(now time.Time, reason string) error {
	if err := fsmFireEvent(a.certFSM, EventCancel, now, a); err != nil {
		return err
	}
	a.CancelReason = truncateRunes(reason, maxCancelReasonRunes)
	return nil
}

const maxCancelReasonRunes = 500

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (a *CertificationApplication) UpdateProgress(currentStep int, stepData json.RawMessage, now time.Time) {
	a.CurrentStep = currentStep
	a.StepData = stepData
	a.UpdatedAt = now
}

func (a *CertificationApplication) IsDraft() bool {
	return a.Status == CertStatusDraft
}

func (a *CertificationApplication) IsSubmitted() bool {
	return a.Status == CertStatusSubmitted
}

func (a *CertificationApplication) IsForwarded() bool {
	return a.Status == CertStatusForwarded
}

func (a *CertificationApplication) IsRejected() bool {
	return a.Status == CertStatusRejected
}

func (a *CertificationApplication) Can(event string) bool {
	return a.certFSM.Can(event)
}

func (a *CertificationApplication) ResetToDraft(now time.Time, payuniRejected bool) error {
	if a.Status == CertStatusForwarded && !payuniRejected {
		return &ErrInvalidTransition{From: StateForwarded, To: StateDraft}
	}
	return fsmFireEvent(a.certFSM, EventResetDraft, now, a)
}

type CertificationReview struct {
	CreatedAt       time.Time
	ReviewerID      *int64
	ReviewerName    string
	Comment         string
	RejectionReason string
	RequiredFields  json.RawMessage
	ID              int64
	ApplicationID   int64
	Action          ReviewAction
	Result          ReviewResult
}

func NewReview(applicationID int64, reviewerID *int64, reviewerName string, action ReviewAction, result ReviewResult, comment string, now time.Time) *CertificationReview {
	return &CertificationReview{
		ApplicationID: applicationID,
		ReviewerID:    reviewerID,
		ReviewerName:  reviewerName,
		Action:        action,
		Result:        result,
		Comment:       comment,
		CreatedAt:     now,
	}
}

type CertificationListFilter struct {
	Status  *string
	Keyword *string
	Limit   int32
	Offset  int32
}

type CertificationStats struct {
	Total     int64
	Submitted int64
	Approved  int64
	Rejected  int64
}

type AdminCertificationItem struct {
	UpdatedAt          time.Time
	CreatedAt          time.Time
	SubmittedAt        *time.Time
	RejectedAt         *time.Time
	SellerType         string
	StoreName          string
	RealName           string
	Status             string
	ApplicationNo      string
	IDCardFrontAssetID string
	IDCardBackAssetID  string
	BankBookAssetID    string
	DocumentAssetIDs   []string
	CurrentStep        int
	TotalSteps         int
	ID                 int64
	AccountID          int64
}

type CertHolderNames struct {
	RealName    string
	CompanyName string
}

type CertificationRepository interface {
	GetApplicationByID(ctx context.Context, id int64) (*CertificationApplication, error)
	GetApplicationByNo(ctx context.Context, applicationNo string) (*CertificationApplication, error)
	GetApplicationsBySeller(ctx context.Context, accountID int64) ([]*CertificationApplication, error)
	GetCurrentApplication(ctx context.Context, accountID int64) (*CertificationApplication, error)
	GetDraftApplication(ctx context.Context, accountID int64) (*CertificationApplication, error)
	GetCertHolderNames(ctx context.Context, storeID int64) (CertHolderNames, bool, error)
	CreateApplication(ctx context.Context, app *CertificationApplication) error
	UpdateApplication(ctx context.Context, app *CertificationApplication) error

	ListForAdmin(ctx context.Context, filter CertificationListFilter) ([]AdminCertificationItem, error)
	CountForAdmin(ctx context.Context, filter CertificationListFilter) (int64, error)
	GetStatsForAdmin(ctx context.Context) (*CertificationStats, error)

	GetReviewsByApplication(ctx context.Context, applicationID int64) ([]*CertificationReview, error)
	GetLatestReview(ctx context.Context, applicationID int64) (*CertificationReview, error)
	CreateReview(ctx context.Context, review *CertificationReview) error
}
