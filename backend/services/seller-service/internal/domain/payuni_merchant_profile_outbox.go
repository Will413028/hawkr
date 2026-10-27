// services/seller-service/internal/domain/payuni_merchant_profile_outbox.go
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type PayuniMerchantProfileOutboxStatus string

const (
	PayuniMerchantProfileOutboxPending PayuniMerchantProfileOutboxStatus = "pending"
	PayuniMerchantProfileOutboxDone    PayuniMerchantProfileOutboxStatus = "done"
	PayuniMerchantProfileOutboxFailed  PayuniMerchantProfileOutboxStatus = "failed"
)

// PayuniMerchantProfilePayload 是要送到 PayUni update_merchant 的欄位子集。
// 只有地址兩欄：contactPhone 不在同步範圍（PayUni 的 ContactMobile 取自帳號
// 電話而非 seller.ContactPhone），其餘可改欄位本次不動。
type PayuniMerchantProfilePayload struct {
	MerAddr     string `json:"merAddr"`
	MerAddrPost string `json:"merAddrPost"`
}

// PayuniMerchantProfileOutbox 是一筆「把商店地址送到 PayUni」的保證投遞任務。
// 與 MerchantVerificationOutbox 不同 bounded context：那張表發 MQ 通知賣家
// 審核結果，這張表呼叫 PayUni update_merchant 改登記資料。
type PayuniMerchantProfileOutbox struct {
	ID            int64
	StoreID       int64
	AccountID     int64
	Payload       json.RawMessage
	Status        PayuniMerchantProfileOutboxStatus
	AttemptCount  int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// OutboxID 與 OutboxAttemptCount 滿足 github.com/Will413028/hawkr/go-pkg/outbox.Row。
func (o *PayuniMerchantProfileOutbox) OutboxID() int64         { return o.ID }
func (o *PayuniMerchantProfileOutbox) OutboxAttemptCount() int { return o.AttemptCount }

// PayuniMerchantProfileOutboxRepository 由 postgres 套件實作，涵蓋
// pkg/outbox.Repository[*PayuniMerchantProfileOutbox] 與 pkg/outbox.MonitorRepo
// 需要的監控方法。
type PayuniMerchantProfileOutboxRepository interface {
	Insert(ctx context.Context, row *PayuniMerchantProfileOutbox) error
	FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*PayuniMerchantProfileOutbox, error)
	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error)
	UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error
	MarkDone(ctx context.Context, id int64, now time.Time) error
	MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error
	CountPending(ctx context.Context) (int, error)
	OldestPendingAge(ctx context.Context, now time.Time) (age time.Duration, ok bool, err error)
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (deleted int, err error)
}

// NewPayuniMerchantProfileOutboxRow 是純建構函式，id/now 由呼叫端提供
// （tsid + 交易時戳），比照 NewMerchantVerificationOutboxRow。
func NewPayuniMerchantProfileOutboxRow(id, storeID, accountID int64, merAddr, merAddrPost string, now time.Time) (*PayuniMerchantProfileOutbox, error) {
	payload, err := json.Marshal(PayuniMerchantProfilePayload{
		MerAddr:     merAddr,
		MerAddrPost: merAddrPost,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payuni merchant profile payload: %w", err)
	}
	return &PayuniMerchantProfileOutbox{
		ID:            id,
		StoreID:       storeID,
		AccountID:     accountID,
		Payload:       payload,
		Status:        PayuniMerchantProfileOutboxPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}
