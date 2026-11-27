package usecase

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 是 Layer 4 的組裝。
//
// tsid.NewGenerator 放這一層是 §4.2.1 的定版：ID 的產生點是「業務決定要建立一個新實體」
// 的那一刻，不是 repository 收到參數的那一刻——把它下放到 repository，冪等鍵就會在每次
// 重試各自產生一個新 ID，而 ON CONFLICT DO NOTHING 是拿 operation_key 去比對的，
// 不是拿 ID。
var Module = fx.Module("usecase",
	fx.Provide(newIDGenerator),
	fx.Provide(newIssuanceConfig),
	fx.Provide(fx.Annotate(newProviderClient, fx.As(new(ProviderClient)))),
	fx.Provide(NewIssuance),
	fx.Provide(NewVoiding),
	fx.Provide(newAllowance),
	fx.Provide(newRefundAdjustment),
	fx.Provide(NewCheckout),
	fx.Provide(newTrack),
	fx.Provide(newHealthCheck),
	fx.Provide(newRecheck),
	fx.Provide(newGovernance),
	fx.Provide(newBusinessCalendarMirror),
	fx.Provide(newPDF),
	fx.Provide(NewQuery),
	fx.Provide(NewDocumentFile),
	fx.Provide(NewTaxEvidenceFile),
	fx.Provide(newPreference),
	fx.Provide(NewComplaint),
	fx.Provide(newCorrection),
	fx.Provide(newCorrectionIntake),
	fx.Provide(NewTaxProfile),
	fx.Provide(NewAdminJob),
	fx.Provide(NewAdminIssuance),
	fx.Provide(NewAdminComplaint),
	fx.Provide(NewProviderAccount),
	fx.Provide(NewSellerComplaint),
	fx.Provide(NewSellerJob),
	fx.Provide(NewNotificationResend),
	fx.Provide(newManualIssuanceConfig),
	fx.Provide(fx.Annotate(newTaxEvidenceStore, fx.As(new(TaxEvidenceStore)))),
	fx.Provide(fx.Annotate(newReceiptRenderer, fx.As(new(ReceiptRenderer)))),
	fx.Provide(fx.Annotate(newReceiptUploader, fx.As(new(ReceiptUploader)))),
)

func newReceiptRenderer(cfg *config.Config, log *zap.Logger) (*domain.ReceiptRenderer, error) {
	font, err := os.ReadFile(cfg.Receipt.FontPath)
	if err != nil {
		return nil, fmt.Errorf("invoice: 收據字型讀取失敗（RECEIPT_FONT_PATH=%s）：%w", cfg.Receipt.FontPath, err)
	}
	r, err := domain.NewReceiptRenderer(font)
	if err != nil {
		return nil, fmt.Errorf("invoice: 收據字型無法載入（RECEIPT_FONT_PATH=%s，必須是 TTF 不是 TTC）：%w",
			cfg.Receipt.FontPath, err)
	}
	log.Info("invoice.receipt.renderer_ready",
		zap.String("font_path", cfg.Receipt.FontPath),
		zap.Int("font_bytes", len(font)),
		zap.String("layout_version", domain.ReceiptLayoutVersion))
	return r, nil
}

func newReceiptUploader(c *media.Client) *media.Client { return c }

func newTaxEvidenceStore(c *media.Client) *media.Client { return c }

func newIDGenerator() func() int64 { return tsid.NewGenerator() }

func newProviderClient(c amego.Client) amego.Client { return c }

func newIssuanceConfig(cfg *config.Config) IssuanceConfig {
	return IssuanceConfig{
		MaxAttempts: int32(cfg.Schedule.IssueMaxRetry), //nolint:gosec // 值域由 Config.Validate 守
		BaseBackoff: secondsOf(cfg.Schedule.BackoffBaseSeconds),
		MaxBackoff:  secondsOf(cfg.Schedule.BackoffMaxSeconds),
		IssueLease:  secondsOf(cfg.Schedule.IssueLeaseSeconds),
		RecheckBase: secondsOf(cfg.Schedule.BackoffBaseSeconds),
	}
}

func newPreference(
	tx database.TxManager,
	prefs domain.BuyerPreferenceRepository,
	idem domain.IdempotencyRepository,
	provider ProviderClient,
	cfg *config.Config,
	newID func() int64,
	log *zap.Logger,
) *Preference {
	u := NewPreference(tx, prefs, idem, newID, log)
	switch {
	case cfg.Amego.PlatformInvoice != "" && cfg.Amego.PlatformAppKey != "":
		u = u.WithBarcodeVerifier(platformBarcodeVerifier{
			provider: provider,
			cred: amego.Credential{
				Invoice: cfg.Amego.PlatformInvoice,
				AppKey:  cfg.Amego.PlatformAppKey,
			},
		})
		platformCredentialIncomplete.Set(0)
		mobileBarcodeVerifierEnabled.Set(1)
	case cfg.Amego.PlatformInvoice == "" && cfg.Amego.PlatformAppKey == "":
		platformCredentialIncomplete.Set(0)
		mobileBarcodeVerifierEnabled.Set(0)
	default:
		platformCredentialIncomplete.Set(1)
		mobileBarcodeVerifierEnabled.Set(0)
		log.Error("平台 AMEGO 憑證只有一半，手機條碼存在性驗證已靜默停用",
			zap.Bool("has_platform_invoice", cfg.Amego.PlatformInvoice != ""),
			zap.Bool("has_platform_app_key", cfg.Amego.PlatformAppKey != ""),
			zap.String("impact", "買家建立／修改常用發票資料時，無效的手機條碼會被接受（HTTP 200），錯誤要到開票前那一層才會被擋下"),
			zap.String("fix", "backend#3269：把 AMEGO_PLATFORM_APP_KEY 放進 hawkr-amego secret"))
	}
	return u
}

type platformBarcodeVerifier struct {
	provider ProviderClient
	cred     amego.Credential
}

func (v platformBarcodeVerifier) VerifyMobileBarcode(ctx context.Context, barcode string) error {
	return v.provider.VerifyMobileBarcode(ctx, v.cred, barcode)
}

func newManualIssuanceConfig(cfg *config.Config) ManualIssuanceConfig {
	return ManualIssuanceConfig{PlatformTaxID: cfg.Amego.PlatformInvoice}
}

func secondsOf(v int) time.Duration { return time.Duration(v) * time.Second }

func newTrack(
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	obs TrackObserver,
	ops domain.OperationRepository,
	log *zap.Logger,
	cfg *config.Config,
) *Track {
	return NewTrack(accounts, client, obs, ops, log, time.Now,
		int64(cfg.Schedule.TrackBalanceWarnThresh), cfg.Governance.TrackGrowthHeadroom)
}

func newHealthCheck(
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	log *zap.Logger,
) *HealthCheck {
	return NewHealthCheck(accounts, client, log, time.Now, healthCheckBatch)
}

func newRecheck(
	ops domain.OperationRepository,
	docs domain.DocumentRepository,
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	voids *Voiding,
	allowances *Allowance,
	audit domain.AuditRepository,
	log *zap.Logger,
) *Recheck {
	return NewRecheck(ops, docs, accounts, client, voids, allowances, audit, log, time.Now)
}

func newGovernance(
	profiles domain.SellerTaxProfileRepository,
	ops domain.OperationRepository,
	obs GovernanceObserver,
	inbox domain.InboxRepository,
	outboxRows domain.OutboxRepository,
	log *zap.Logger,
) *Governance {
	return NewGovernance(profiles, ops, obs, log, time.Now).
		WithRetention(inbox, outboxRows)
}

func newPDF(
	docs domain.DocumentRepository,
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	uploader *media.Client,
	log *zap.Logger,
	cfg *config.Config,
) *PDF {
	return NewPDF(docs, accounts, client, uploader, log, time.Now, cfg.Schedule.PDFMaxBytes)
}

func newCorrection(
	tx database.TxManager,
	corrections domain.CorrectionRequestRepository,
	operations domain.OperationRepository,
	documents domain.DocumentRepository,
	adjustments domain.AdjustmentRepository,
	orders OrderSnapshotReviser,
	audit domain.AuditRepository,
	log *zap.Logger,
	newID func() int64,
	cfg IssuanceConfig,
) *Correction {
	return NewCorrection(tx, corrections, operations, documents, adjustments, orders, audit,
		log, func() time.Time { return time.Now().UTC() }, newID, cfg.MaxAttempts)
}

func newCorrectionIntake(
	documents domain.DocumentRepository,
	operations domain.OperationRepository,
	accounts domain.ProviderAccountRepository,
	prefs domain.BuyerPreferenceRepository,
	corrections domain.CorrectionRequestRepository,
	client amego.Client,
	correction *Correction,
) *CorrectionIntake {
	return NewCorrectionIntake(CorrectionIntakeDeps{
		Documents: documents, Operations: operations, Accounts: accounts,
		Preferences: prefs, Corrections: corrections, Tracks: client,
	}, correction)
}

func newRefundAdjustment(
	tx database.TxManager,
	adjustments domain.AdjustmentRepository,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	audit domain.AuditRepository,
	newID func() int64,
	cfg IssuanceConfig,
	appCfg *config.Config,
	log *zap.Logger,
) *RefundAdjustment {
	return NewRefundAdjustment(tx, adjustments, docs, ops, audit, newID, cfg,
		appCfg.Governance.AllowanceB2CSplitTax, log, func() time.Time { return time.Now().UTC() })
}

func newAllowance(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	adjustments domain.AdjustmentRepository,
	accounts domain.ProviderAccountRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	provider ProviderClient,
	cfg IssuanceConfig,
	appCfg *config.Config,
	log *zap.Logger,
) *Allowance {
	return NewAllowance(tx, docs, ops, adjustments, accounts, issuers, audit, outbox, provider,
		cfg, appCfg.Governance.AllowanceB2CSplitTax, log)
}

func newBusinessCalendarMirror(
	businessDays domain.BusinessDayRepository,
	tx database.TxManager,
	log *zap.Logger,
) *BusinessCalendarMirror {
	return NewBusinessCalendarMirror(businessDays, tx, log, time.Now)
}
