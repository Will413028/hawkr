// Package payout 是 seller 對收款帳戶（payment 模組擁有）的訂閱者：payment 的同步 job 在寫帳戶快照的 transaction 內發 PayoutAccountChanged，
// 這裡在同一個 transaction 內把轉帳能力複製到開通閘門讀的副本（payout_capabilities）、把外部銀行帳號 fingerprint 寫成防詐比對 hash（D12），
// 閘門的輸入有變時重新公告可曝光（Stripe 計畫 D28）。全域鎖順序 payment 先於 seller：payment 的帳戶列已先鎖住，這裡依序鎖副本列、公告列。
package payout

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Identities 是防詐比對 hash 的寫入（kyc.Fingerprints）。
type Identities interface {
	RecordBankFingerprints(ctx context.Context, q *store.Queries, storeID int64, provider string, fingerprints []string, now time.Time) error
}

// Deps 外部依賴；全部必填。
type Deps struct {
	DB         *db.DB
	Jobs       jobs.Enqueuer
	Exposure   *exposure.Service
	Identities Identities
}

// Service 是訂閱者。
type Service struct{ d Deps }

// New 建立訂閱者。
func New(d Deps) *Service {
	if d.DB == nil || d.Jobs == nil || d.Exposure == nil || d.Identities == nil {
		panic("seller/payout: missing dependency")
	}
	return &Service{d: d}
}

// Change 是 PayoutAccountChanged 的內容（root 套件從事件轉過來）。Fingerprints 是明文：只交給 Identities 寫成 hash，不保存、不寫 log。
type Change struct {
	StoreID         int64
	Provider        string
	TransfersActive bool
	Fingerprints    []string
	At              time.Time
}

// Apply 在發布方的 transaction 內（ctx）套用一次變更：寫副本與 fingerprint hash，轉帳能力有變時重新公告並讓店家的 stint 快取失效。
func (s *Service) Apply(ctx context.Context, c Change) error {
	q := store.New(s.d.DB.Conn(ctx))
	before, err := q.LockPayoutCapability(ctx, c.StoreID)
	if errors.Is(err, pgx.ErrNoRows) {
		before, err = false, nil
	}
	if err != nil {
		return err
	}
	if err := q.UpsertPayoutCapability(ctx, store.UpsertPayoutCapabilityParams{StoreID: c.StoreID, Provider: c.Provider, TransfersActive: c.TransfersActive, Now: c.At}); err != nil {
		return err
	}
	if err := s.d.Identities.RecordBankFingerprints(ctx, q, c.StoreID, c.Provider, c.Fingerprints, c.At); err != nil {
		return err
	}
	if before == c.TransfersActive {
		return nil
	}
	if err := s.d.Exposure.Announce(ctx, q, c.At, c.StoreID); err != nil {
		return err
	}
	return stint.InvalidateByID(ctx, q, s.d.Jobs, c.StoreID)
}
