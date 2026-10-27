// Package postgres 是 Layer 2：thin adapter。
//
// 每個 repo 用 fx.Annotate + fx.As 綁到 internal/domain 的介面；querier 一律
// gen.New(database.GetDBTX(ctx, pool)) 現取，交易感知，不自己 Begin/Commit。
// mapper.go 是 AppKey 密文↔明文的唯一收斂點（§4.2.1）。
package postgres

import (
	"fmt"
	"math"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/fx"
)

// Module 是 Layer 2 的組裝。
var Module = fx.Module("repository.postgres",
	fx.Provide(NewKeyRing),

	fx.Provide(fx.Annotate(NewProviderAccountRepository, fx.As(new(domain.ProviderAccountRepository)))),
	fx.Provide(fx.Annotate(NewIssuerRepository, fx.As(new(domain.IssuerRepository)))),
	fx.Provide(fx.Annotate(NewSellerTaxProfileRepository, fx.As(new(domain.SellerTaxProfileRepository)))),
	fx.Provide(fx.Annotate(NewBuyerPreferenceRepository, fx.As(new(domain.BuyerPreferenceRepository)))),
	// 冪等鍵（#3015 裁決 A）。與業務寫入共用 ctx 裡的交易，見 IdempotencyRepository 的檔頭。
	fx.Provide(fx.Annotate(NewIdempotencyRepository, fx.As(new(domain.IdempotencyRepository)))),
	fx.Provide(fx.Annotate(NewDocumentRepository, fx.As(new(domain.DocumentRepository)))),
	fx.Provide(fx.Annotate(NewOperationRepository, fx.As(new(domain.OperationRepository)))),
	fx.Provide(fx.Annotate(NewAdjustmentRepository, fx.As(new(domain.AdjustmentRepository)))),
	fx.Provide(fx.Annotate(NewCorrectionRequestRepository, fx.As(new(domain.CorrectionRequestRepository)))),
	fx.Provide(fx.Annotate(NewComplaintRepository, fx.As(new(domain.ComplaintRepository)))),
	fx.Provide(fx.Annotate(NewOutboxRepository, fx.As(new(domain.OutboxRepository)))),
	fx.Provide(fx.Annotate(NewInboxRepository, fx.As(new(domain.InboxRepository)))),
	fx.Provide(fx.Annotate(NewCancelledOrderRepository, fx.As(new(domain.CancelledOrderRepository)))),
	fx.Provide(fx.Annotate(NewAuditRepository, fx.As(new(domain.AuditRepository)))),
	fx.Provide(fx.Annotate(NewBusinessDayRepository, fx.As(new(domain.BusinessDayRepository)))),
	fx.Provide(fx.Annotate(NewOverviewRepository, fx.As(new(domain.OverviewRepository)))),
	fx.Provide(fx.Annotate(NewManualUnlockRepository, fx.As(new(domain.ManualUnlockRepository)))),
	fx.Provide(fx.Annotate(NewProviderCutoverRepository, fx.As(new(domain.ProviderCutoverRepository)))),
)

// NewKeyRing 由 Config ＋ config.Module 已 provide 的 current cipher 組出 KeyRing。
//
// 為什麼 prev cipher 在這裡建而不是在 config：KeyRing 唯一的消費者是本 package 的
// mapper，而「回 nil 代表沒有 fallback 可試」這個約定是解密語意的一部分。config 那層
// provide 一個可能為 nil 的 *crypto.Cipher 進 fx 圖，只會讓其他消費者踩到 nil。
//
// 🔴 三個 fail-closed 檢查都在 boot 期：起不來遠比「跑起來之後每一列都解不開」好，
// 後者的症狀是 AMEGO 回 code 16，看起來像賣家貼錯 key。
func NewKeyRing(cfg *config.Config, current *crypto.Cipher) (*KeyRing, error) {
	cur, err := toKeyVersion(cfg.Encryption.MasterKeyVersion, "INVOICE_ENCRYPTION_MASTER_KEY_VERSION")
	if err != nil {
		return nil, err
	}
	ring := &KeyRing{
		Current:        current,
		CurrentVersion: cur,
		LookupHMACKey:  cfg.Encryption.LookupHMACKey,
	}
	if cfg.Encryption.PrevMasterKey != "" {
		prev, err := crypto.New(cfg.Encryption.PrevMasterKey)
		if err != nil {
			return nil, fmt.Errorf("invoice: prev master key: %w", err)
		}
		prevVer, err := toKeyVersion(cfg.Encryption.PrevMasterKeyVer, "INVOICE_ENCRYPTION_MASTER_KEY_PREV_VERSION")
		if err != nil {
			return nil, err
		}
		ring.Prev = prev
		ring.PrevVersion = prevVer
	}
	if ring.LookupHMACKey == "" {
		// Config.Validate ⑤ 已擋掉，這裡是第二道：fingerprint 用空 key 算得出來，
		// 而算出來的值會被寫進 uq_ipa_fingerprint ⇒ 全站賣家共用同一組可離線重算的指紋。
		return nil, fmt.Errorf("invoice: INVOICE_LOOKUP_HMAC_KEY is empty (fail-closed)")
	}
	return ring, nil
}

// toKeyVersion 把 config 的 int 收斂成 credential_key_version 的 smallint 值域。
//
// ⚠️ Config.Validate 目前允許 1~99999，而 smallint 上限是 32767 —— 落在 (32767, 99999]
// 的值在寫入時會撞 22003 而不是在 boot 期被擋下。這裡先擋一次；根治要收窄 Validate
// 的上界（見交付回報的「設計書／實作矛盾」段）。
func toKeyVersion(v int, envName string) (int16, error) {
	if v < 1 || v > math.MaxInt16 {
		return 0, fmt.Errorf("invoice: %s must be within 1..%d (credential_key_version is smallint), got %d", envName, math.MaxInt16, v)
	}
	return int16(v), nil
}

// RepositorySet 把全部 17 個 repository 介面聚成一個 fx.In，只有一個用途：讓
// di_test.go 一次要求全部，圖裡少任何一個都會在測試裡紅，而不是等到開機。
//
// 業務程式碼**不要**注入它——那會讓 usecase 拿到它用不到的十幾個依賴，
// 也讓「這個 usecase 真正依賴什麼」從簽章上看不出來。
type RepositorySet struct {
	fx.In

	ProviderAccount   domain.ProviderAccountRepository
	Issuer            domain.IssuerRepository
	SellerTaxProfile  domain.SellerTaxProfileRepository
	BuyerPreference   domain.BuyerPreferenceRepository
	Document          domain.DocumentRepository
	Operation         domain.OperationRepository
	Adjustment        domain.AdjustmentRepository
	CorrectionRequest domain.CorrectionRequestRepository
	Complaint         domain.ComplaintRepository
	Outbox            domain.OutboxRepository
	Inbox             domain.InboxRepository
	CancelledOrder    domain.CancelledOrderRepository
	Audit             domain.AuditRepository
	BusinessDay       domain.BusinessDayRepository
	Overview          domain.OverviewRepository
	ManualUnlock      domain.ManualUnlockRepository
	ProviderCutover   domain.ProviderCutoverRepository
}
