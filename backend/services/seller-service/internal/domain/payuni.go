package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type PayuniRegistration struct {
	UpdatedAt              time.Time
	RegisteredAt           *time.Time
	LastSyncedAt           *time.Time
	LastClaimedAt          *time.Time
	ApprovedAt             *time.Time
	ErrorMessage           string
	ErrorCode              string
	ATMStatus              string
	MerStatus              string
	MemberStatus           string
	CreditStatus           string
	ApplePayStatus         string
	GooglePayStatus        string
	Reason                 string
	PayuniMerID            string
	RawRequest             json.RawMessage
	RawResponse            json.RawMessage
	ID                     int64
	Status                 PayuniStatus
	ApplicationID          int64
	AccountID              int64
	StoreID                int64
	PaymentEnabled         bool
	ContactName            string
	ContactMobile          string
	ContactTel             string
	MerEmail               string
	MerAddr                string
	MerAddrPost            string
	MerTel                 string
	MerchantProfileVersion int64
}

type PayuniStatusHistory struct {
	RecordedAt      time.Time
	PayuniMerID     string
	Source          string
	MerStatus       string
	MemberStatus    string
	CreditStatus    string
	ApplePayStatus  string
	GooglePayStatus string
	ATMStatus       string
	Reason          string
	RawResponse     json.RawMessage
	ID              int64
	AccountID       int64
	StoreID         int64
}

type PayuniStatusObservation struct {
	ObservedAt      time.Time
	Source          string
	MerStatus       string
	MemberStatus    string
	CreditStatus    string
	ApplePayStatus  string
	GooglePayStatus string
	ATMStatus       string
	Reason          string
	Raw             json.RawMessage
}

func (p *PayuniRegistration) SetRegistered(merID string, now time.Time) {
	p.PayuniMerID = merID
	p.Status = PayuniStatusPending
	p.RegisteredAt = &now
	p.UpdatedAt = now
}

func (p *PayuniRegistration) SetActive(now time.Time) {
	p.Status = PayuniStatusActive
	p.PaymentEnabled = true
	p.ApprovedAt = &now
	p.ErrorCode = ""
	p.UpdatedAt = now
}

func (p *PayuniRegistration) SetFailed(code, message string, now time.Time) {
	p.Status = PayuniStatusFailed
	p.ErrorCode = code
	p.ErrorMessage = message
	p.UpdatedAt = now
}

func (p *PayuniRegistration) Apply(obs PayuniStatusObservation) {
	p.MerStatus = obs.MerStatus
	p.MemberStatus = obs.MemberStatus
	p.CreditStatus = obs.CreditStatus
	p.ApplePayStatus = obs.ApplePayStatus
	p.GooglePayStatus = obs.GooglePayStatus
	p.ATMStatus = obs.ATMStatus
	p.Reason = obs.Reason
	observedAt := obs.ObservedAt
	p.LastSyncedAt = &observedAt
	p.UpdatedAt = observedAt
}

func (p *PayuniRegistration) ChangedBy(obs PayuniStatusObservation) bool {
	return p.MerStatus != obs.MerStatus ||
		p.MemberStatus != obs.MemberStatus ||
		p.CreditStatus != obs.CreditStatus ||
		p.ApplePayStatus != obs.ApplePayStatus ||
		p.GooglePayStatus != obs.GooglePayStatus ||
		p.ATMStatus != obs.ATMStatus ||
		p.Reason != obs.Reason
}

func (p *PayuniRegistration) KYCChangedBy(obs PayuniStatusObservation) bool {
	return p.MerStatus != obs.MerStatus || p.MemberStatus != obs.MemberStatus
}

func (p *PayuniRegistration) NewStatusHistory(id int64, obs PayuniStatusObservation) *PayuniStatusHistory {
	return &PayuniStatusHistory{
		ID:              id,
		AccountID:       p.AccountID,
		StoreID:         p.StoreID,
		PayuniMerID:     p.PayuniMerID,
		Source:          obs.Source,
		MerStatus:       obs.MerStatus,
		MemberStatus:    obs.MemberStatus,
		CreditStatus:    obs.CreditStatus,
		ApplePayStatus:  obs.ApplePayStatus,
		GooglePayStatus: obs.GooglePayStatus,
		ATMStatus:       obs.ATMStatus,
		Reason:          obs.Reason,
		RawResponse:     obs.Raw,
		RecordedAt:      obs.ObservedAt,
	}
}

func FormatMerID(storeCode int64) string {
	return fmt.Sprintf("HAWK%d", storeCode)
}

type PayuniRepository interface {
	GetByStoreID(ctx context.Context, storeID int64) (*PayuniRegistration, error)
	GetByMerID(ctx context.Context, merID string) (*PayuniRegistration, error)
	GetPending(ctx context.Context, limit int, staleBefore time.Time) ([]*PayuniRegistration, error)
	ClaimForSync(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error)
	Create(ctx context.Context, reg *PayuniRegistration) error
	Update(ctx context.Context, reg *PayuniRegistration) error
	Upsert(ctx context.Context, reg *PayuniRegistration) error
	UpdateMerchantProfile(ctx context.Context, storeID, expectedVersion int64,
		contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string,
	) (*PayuniRegistration, error)
	AcquireMerchantProfileUpdateLock(ctx context.Context, storeID int64) error
	InsertStatusHistory(ctx context.Context, history *PayuniStatusHistory) error
	DeleteStatusHistoryOlderThan(ctx context.Context, before time.Time) (int64, error)
}
