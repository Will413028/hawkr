package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const (
	providerHealthCheckInterval = 15 * time.Minute

	providerReasonMaxRunes = adminReasonMaxRunes

	providerAppKeyMaxRunes = 256

	providerReasonCredentialSubmitted = "SELLER_PROVIDER_CREDENTIAL_SUBMITTED"
	providerReasonVerifyStarted       = "PROVIDER_VERIFICATION_STARTED"
	providerReasonVerifyLanded        = "PROVIDER_VERIFICATION_RESULT"
	providerReasonCutoverSet          = "SELLER_PROVIDER_CUTOVER_SET"
	providerReasonRotationStarted     = "SELLER_PROVIDER_ROTATION_STARTED"
	providerReasonRotationLanded      = "SELLER_PROVIDER_ROTATION_RESULT"
	providerReasonAdminDeactivate     = "ADMIN_PROVIDER_DEACTIVATE"
	providerReasonAdminReactivate     = "ADMIN_PROVIDER_REACTIVATE"
	providerReasonAdminRevoke         = "ADMIN_PROVIDER_REVOKE"

	providerAuditActionHealthCheck = "INVOICE_PROVIDER_HEALTH_CHECK"
	providerAuditActionDeactivate  = "INVOICE_PROVIDER_DEACTIVATE"
	providerAuditActionReactivate  = "INVOICE_PROVIDER_REACTIVATE"
	providerAuditActionRevoke      = "INVOICE_PROVIDER_REVOKE"

	providerSubjectType  = "PROVIDER_ACCOUNT"
	providerResourceType = "PROVIDER_ACCOUNT"

	providerActorSeller = "SELLER"
	providerActorAdmin  = "ADMIN"

	providerAffectedScopeNotice = "只影響停用時點之後付款的訂單；較早付款的訂單維持原開票方式。"
)

var providerTaxIDPattern = regexp.MustCompile(`^[0-9]{8}$`)

// 型別邊界錯誤（契約違規 ⇒ 呼叫端映成 InvalidArgument／400，不借業務碼）。
//
// 借一個近似的業務碼會讓前端把「你的統編少一碼」顯示成「憑證驗證失敗，請回光貿後台確認」，
// 而那兩件事的下一步完全不同。
var (
	ErrProviderUnsupported      = errors.New("invoice: provider must be AMEGO")
	ErrProviderTaxIDInvalid     = errors.New("invoice: seller_tax_id must be 8 digits")
	ErrProviderAppKeyRequired   = errors.New("invoice: app_key is required")
	ErrProviderAppKeyTooLong    = errors.New("invoice: app_key exceeds 256 characters")
	ErrProviderCutoverInvalid   = errors.New("invoice: cutover_at must be an ISO 8601 UTC timestamp")
	ErrProviderCutoverInPast    = errors.New("invoice: cutover_at must not be earlier than now")
	ErrProviderReasonRequired   = errors.New("invoice: reason is required")
	ErrProviderReasonTooLong    = errors.New("invoice: reason exceeds 500 characters")
	ErrProviderConfirmRequired  = errors.New("invoice: this action requires confirmed=true")
	ErrProviderConsentRequired  = errors.New("invoice: consent_version is required")
	ErrProviderConsentTooLong   = errors.New("invoice: consent_version exceeds 32 characters")
	errProviderVerifyStateDrift = errors.New(
		"invoice: provider account left CREDENTIAL_VERIFYING before the verification result could land")
)

// ProviderAccount 是 M4 的寫入 usecase。
//
// 它與 Query 分開的理由同 Complaint／TaxProfile：只有它需要 TxManager、稽核與 ProviderClient。
// 把寫入掛進 Query，「唯讀查詢面」那句話就變成假的。
type ProviderAccount struct {
	tx          database.TxManager
	issuers     domain.IssuerRepository
	profiles    domain.SellerTaxProfileRepository
	accounts    domain.ProviderAccountRepository
	ops         domain.OperationRepository
	adjustments domain.AdjustmentRepository
	audit       domain.AuditRepository
	cutovers    domain.ProviderCutoverRepository
	provider    ProviderClient

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

// NewProviderAccount 組出 M4 的寫入 usecase。
func NewProviderAccount(
	tx database.TxManager,
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	accounts domain.ProviderAccountRepository,
	ops domain.OperationRepository,
	adjustments domain.AdjustmentRepository,
	audit domain.AuditRepository,
	cutovers domain.ProviderCutoverRepository,
	provider ProviderClient,
	newID func() int64,
	log *zap.Logger,
) *ProviderAccount {
	return &ProviderAccount{
		tx: tx, issuers: issuers, profiles: profiles, accounts: accounts, ops: ops,
		adjustments: adjustments, audit: audit, cutovers: cutovers, provider: provider, newID: newID,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *ProviderAccount) WithClock(now func() time.Time) *ProviderAccount {
	if now != nil {
		u.now = now
	}
	return u
}

// ProviderAdminActionResult 是端點 45／67 的回應（proto Deactivate／RevokeProviderAccountForAdminData）。
//
// 🔴 PendingOrderCount 是**事後值不是 preflight**：它與判定在同一交易內讀出（§7.4 C-8）。
// 分兩個交易讀，ADM 看到的數字與實際被打掉的數量不一致，而 dialog 開啟到按下之間又付款
// 了一筆正是它要防的那個窗口。
type ProviderAdminActionResult struct {
	StoreID           int64
	Status            string
	OccurredAt        time.Time
	PendingOrderCount int64
	AuditLogID        int64
	AffectedScope     string
}

// PutProviderCredentialsInput 是端點 24 的輸入。
//
// 🔴 AppKey 是**明文**且只往內：它從這裡進入 repository 的 deriveCredential 就結束了，
// 不進任何回應、不進 log、不進稽核（§8.1.2 層 1）。
type PutProviderCredentialsInput struct {
	StoreID         int64
	Provider        string
	SellerTaxID     string
	AppKey          string
	ConsentVersion  string
	ExpectedVersion int64
	IdempotencyKey  string
}

// PutCredentials 寫入（或覆寫）AMEGO 憑證（端點 24 ＝ P01＋P02）。
//
// 🔴 落地即 CREDENTIAL_VERIFYING（§6.1.8 端點 24 逐字「寫入後 status = CREDENTIAL_VERIFYING」）。
// P01 的 CREDENTIAL_SUBMITTED 是狀態機上的中間態，**不對外呈現**：賣家在 S03 按下儲存之後
// 看到的下一格是「驗證中」，而不是一個什麼都不會自己發生的「已送出」。
//
// 三個守衛的順序是刻意的（先便宜的、先能給出明確指引的）：
//
//	稅務身分未核准        → 9321（賣家的下一步是去 S02 送審，不是改 AppKey）
//	統編與已核定稅籍不符  → 9334（發票開錯抬頭不能改只能作廢重開 ⇒ 這一格必須在寫入前擋）
//	這組憑證已被別人綁走  → 9337（賣家的下一步是回光貿後台換一組，不是重打同一把）
func (u *ProviderAccount) PutCredentials(ctx context.Context, in PutProviderCredentialsInput) (ProviderAccountView, error) {
	if err := requireProviderAMEGO(in.Provider); err != nil {
		return ProviderAccountView{}, err
	}
	if !providerTaxIDPattern.MatchString(in.SellerTaxID) {
		return ProviderAccountView{}, ErrProviderTaxIDInvalid
	}
	if err := requireAppKey(in.AppKey); err != nil {
		return ProviderAccountView{}, err
	}
	if err := requireConsentVersion(in.ConsentVersion); err != nil {
		return ProviderAccountView{}, err
	}

	var out ProviderAccountView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, err := u.sellerIssuer(ctx, in.StoreID)
		if err != nil {
			return err
		}
		profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			if isBizCode(err, bizcode.SellerTaxProfileNotFound) {
				return providerBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			return err
		}
		if profile.ReviewStatus != codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value {
			return providerBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		if profile.TaxID == nil || *profile.TaxID != in.SellerTaxID {
			return providerBiz(bizcode.InvoiceProviderTaxidMismatch)
		}

		now := u.now()
		account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
		switch {
		case err == nil:
			if account.Version != in.ExpectedVersion {
				return providerVersionConflict()
			}
			ok, taken, rerr := u.accounts.ResubmitCredential(ctx, domain.ResubmitProviderCredential{
				ID:                account.ID,
				ProviderTaxID:     in.SellerTaxID,
				AppKey:            in.AppKey,
				HealthCheckNextAt: now,
				ExpectedVersion:   in.ExpectedVersion,
			})
			if rerr != nil {
				return rerr
			}
			if taken {
				return providerBiz(bizcode.InvoiceProviderCredentialTaken)
			}
			if !ok {
				return providerBiz(bizcode.InvoiceProviderStateInvalid)
			}
			if herr := u.appendHistory(ctx, account.ID, account.Status,
				codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
				providerReasonCredentialSubmitted, "consentVersion="+in.ConsentVersion,
				providerActorSeller, in.StoreID, now); herr != nil {
				return herr
			}
		case isBizCode(err, bizcode.InvoiceProviderAccountNotFound):
			if in.ExpectedVersion != 0 {
				return providerVersionConflict()
			}
			id := u.newID()
			if cerr := u.accounts.Create(ctx, domain.NewProviderAccount{
				ID:                id,
				IssuerID:          issuer.ID,
				Provider:          providerAMEGO,
				ProviderTaxID:     in.SellerTaxID,
				AppKey:            in.AppKey,
				Status:            codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
				HealthCheckNextAt: now,
			}); cerr != nil {
				return cerr
			}
			if herr := u.appendHistory(ctx, id, "",
				codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
				providerReasonCredentialSubmitted, "consentVersion="+in.ConsentVersion,
				providerActorSeller, in.StoreID, now); herr != nil {
				return herr
			}
		default:
			return err
		}

		out, err = u.viewOf(ctx, in.StoreID, issuer)
		return err
	})
	if err != nil {
		return ProviderAccountView{}, err
	}
	return out, nil
}

// VerifyProviderAccountInput 是端點 25 的輸入。
type VerifyProviderAccountInput struct {
	StoreID         int64
	ExpectedVersion int64
	IdempotencyKey  string
}

// Verify 是賣家的「測試連線」（端點 25）。
//
// 🔴 **不得產生任何發票**：它只打兩支唯讀端點（ban_query／track_all）。這條規則寫在
// ProviderClient 介面的註解裡，而那個介面刻意沒有 IssueInvoice 以外的寫入方法。
//
// 失敗的三個碼**必須分得開**，因為賣家的下一步完全不同：
//
//	9331 憑證驗證失敗   → 回光貿後台確認 AppKey
//	9332 服務暫時不可用 → 等一下再試，**不要**改 AppKey
//	9333 狀態不允許     → 先去端點 24 填憑證
//
// 混成一種會讓賣家在 AMEGO 停機時把正確的 AppKey 刪掉重打。
func (u *ProviderAccount) Verify(ctx context.Context, in VerifyProviderAccountInput) (ProviderAccountView, error) {
	return u.verifyFlow(ctx, providerVerifyRequest{
		StoreID:         in.StoreID,
		ExpectedVersion: in.ExpectedVersion,
		ActorType:       providerActorSeller,
		ActorAccountID:  in.StoreID,
		AuditAction:     "",
	})
}

// AdminProviderHealthCheckInput 是端點 44 的輸入。
type AdminProviderHealthCheckInput struct {
	ActorAccountID int64
	StoreID        int64
	IdempotencyKey string
}

// HealthCheckForAdmin 是 ADM 的「重新測試」（端點 44）。
//
// 四條定版（§6.1.13）：① 同步等 AMEGO ② 唯讀端點、不產生任何發票 ③ **檢查失敗不改
// status 成終態**——憑證真的壞了才落 INVALID_CREDENTIAL／PROVIDER_REJECTED（賣家自己
// 修得掉），而 PROVIDER_UNAVAILABLE 一律 **status 完全不動** ④ provider 不可用回
// 409 ＋ 9332（不是 503）。
//
// ⚠️ 本支**沒有 expectedVersion**（proto 逐字），所以版本由本層在讀出當下取。
// 這不是放寬樂觀鎖：ADM 按的是「再測一次」，它不會覆蓋任何賣家填的東西。
func (u *ProviderAccount) HealthCheckForAdmin(ctx context.Context, in AdminProviderHealthCheckInput) (ProviderAccountView, error) {
	return u.verifyFlow(ctx, providerVerifyRequest{
		StoreID:        in.StoreID,
		UseCurrent:     true,
		ActorType:      providerActorAdmin,
		ActorAccountID: in.ActorAccountID,
		AuditAction:    providerAuditActionHealthCheck,
		AuditReason:    "ADM 主動連線檢查",
		IdempotencyKey: in.IdempotencyKey,
	})
}

// AdminProviderActionInput 是端點 45／66／67 的共通輸入。
type AdminProviderActionInput struct {
	ActorAccountID  int64
	StoreID         int64
	Reason          string
	Confirmed       bool
	ExpectedVersion int64
	IdempotencyKey  string
}

// ReactivateForAdmin 是 ADM 恢復連線（端點 66 ＝ P15）。
//
// 🔴 **恢復前強制重驗，不得直接回 ACTIVE**：SUSPENDED → CREDENTIAL_VERIFYING，
// 結果再走 P03–P06 分流。直接回 ACTIVE 的後果是「憑證其實已被光貿撤銷，而系統宣告
// 恢復正常」，然後每一筆開票都失敗。
//
// 🔴 被端點 45 打成 FAILED_PERMANENT 的那批 operation **不自動復活**（§7.3.4 P15）：
// 由 ADM 在 A06 逐筆按端點 52 安全重試。自動復活會在憑證仍不對時再打一輪 AMEGO，
// 而那一輪的失敗會被算進 attempt_count。
func (u *ProviderAccount) ReactivateForAdmin(ctx context.Context, in AdminProviderActionInput) (ProviderAccountView, error) {
	if err := requireProviderReason(in.Reason); err != nil {
		return ProviderAccountView{}, err
	}
	if !in.Confirmed {
		return ProviderAccountView{}, ErrProviderConfirmRequired
	}
	return u.verifyFlow(ctx, providerVerifyRequest{
		StoreID:         in.StoreID,
		ExpectedVersion: in.ExpectedVersion,
		RequireStatus:   codemeta.InvoiceProviderAccountStatusSuspended.Value,
		ActorType:       providerActorAdmin,
		ActorAccountID:  in.ActorAccountID,
		AuditAction:     providerAuditActionReactivate,
		AuditReason:     in.Reason,
		HistoryReason:   providerReasonAdminReactivate,
		IdempotencyKey:  in.IdempotencyKey,
	})
}

type providerVerifyRequest struct {
	StoreID         int64
	ExpectedVersion int64
	UseCurrent      bool
	RequireStatus   string

	ActorType      string
	ActorAccountID int64

	AuditAction    string
	AuditReason    string
	HistoryReason  string
	IdempotencyKey string
}

func (u *ProviderAccount) verifyFlow(ctx context.Context, req providerVerifyRequest) (ProviderAccountView, error) {
	var (
		issuer   domain.Issuer
		account  domain.ProviderAccount
		fromLive bool
		version  int64
	)
	now := u.now()
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		issuer, account, err = u.loadAccount(ctx, req.StoreID)
		if err != nil {
			return err
		}
		version = account.Version
		if !req.UseCurrent && account.Version != req.ExpectedVersion {
			return providerVersionConflict()
		}
		if req.RequireStatus != "" && account.Status != req.RequireStatus {
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}
		fromLive = providerIsLive(account.Status)
		if fromLive {
			return nil
		}
		ok, err := u.accounts.StartVerification(ctx, account.ID, version, now)
		if err != nil {
			return err
		}
		if !ok {
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}
		version++
		return u.appendHistory(ctx, account.ID, account.Status,
			codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
			firstNonEmpty(req.HistoryReason, providerReasonVerifyStarted), req.AuditReason,
			req.ActorType, req.ActorAccountID, now)
	})
	if err != nil {
		return ProviderAccountView{}, err
	}

	outcome := u.runVerification(ctx, account, issuer.ID)

	var view ProviderAccountView
	landedAt := u.now()
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		if lerr := u.landVerification(ctx, account, version, fromLive, outcome, landedAt); lerr != nil {
			return lerr
		}
		if herr := u.appendHistory(ctx, account.ID, providerFromStatus(account.Status, fromLive),
			outcome.NextStatus, providerReasonVerifyLanded, outcome.Reason,
			req.ActorType, req.ActorAccountID, landedAt); herr != nil {
			return herr
		}
		if req.AuditAction != "" {
			if _, aerr := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
				ActorAccountID: req.ActorAccountID,
				Action:         req.AuditAction,
				ResourceType:   providerResourceType,
				ResourceID:     account.ID,
				BeforeState:    jobAuditState(account.Status, nil),
				AfterState:     jobAuditState(outcome.NextStatus, map[string]string{"providerCode": outcome.ProviderCode}),
				ReasonCode:     req.AuditAction,
				ReasonDetail:   firstNonEmpty(req.AuditReason, "ADM 連線檢查"),
				RequestID:      nilIfEmpty(req.IdempotencyKey),
				Now:            landedAt,
			}); aerr != nil {
				return aerr
			}
		}
		var verr error
		view, verr = u.viewOf(ctx, req.StoreID, issuer)
		return verr
	})
	if err != nil {
		return ProviderAccountView{}, err
	}
	if outcome.BizCode != 0 {
		return view, providerBizWithMessage(outcome.BizCode, outcome.Reason)
	}
	return view, nil
}

func (u *ProviderAccount) landVerification(
	ctx context.Context, account domain.ProviderAccount, version int64,
	fromLive bool, outcome providerVerifyOutcome, now time.Time,
) error {
	next := now.Add(providerHealthCheckInterval)
	if fromLive {
		if outcome.CredentialFault {
			ok, err := u.accounts.FailVerificationFromActive(ctx, domain.LandProviderVerification{
				ID: account.ID, NextStatus: outcome.NextStatus, StatusReason: outcome.Reason,
				NextCheckAt: next, ExpectedVersion: version, Now: now,
			})
			if err != nil {
				return err
			}
			if !ok {
				return errProviderVerifyStateDrift
			}
			return nil
		}
		ok, err := u.accounts.TouchHealthCheck(ctx, account.ID, version, now, next)
		if err != nil {
			return err
		}
		if !ok {
			return errProviderVerifyStateDrift
		}
		return nil
	}

	verifiedAt := account.VerifiedAt
	if outcome.NextStatus == codemeta.InvoiceProviderAccountStatusActive.Value {
		verifiedAt = &now
	}

	var breakerOpenedAt *time.Time
	if outcome.NextStatus == codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value {
		if account.BreakerOpenedAt != nil {
			breakerOpenedAt = account.BreakerOpenedAt
		} else {
			breakerOpenedAt = &now
		}
	}

	ok, err := u.accounts.LandVerification(ctx, domain.LandProviderVerification{
		ID: account.ID, NextStatus: outcome.NextStatus, StatusReason: outcome.Reason,
		VerifiedAt: verifiedAt, BreakerOpenedAt: breakerOpenedAt,
		NextCheckAt: next, ExpectedVersion: version, Now: now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return errProviderVerifyStateDrift
	}
	return nil
}

func providerFromStatus(original string, fromLive bool) string {
	if fromLive {
		return original
	}
	return codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value
}

func providerIsLive(status string) bool {
	switch status {
	case codemeta.InvoiceProviderAccountStatusActive.Value,
		codemeta.InvoiceProviderAccountStatusDegraded.Value:
		return true
	default:
		return false
	}
}

// ActivateProviderAccountInput 是端點 26 的輸入。
type ActivateProviderAccountInput struct {
	StoreID         int64
	CutoverAt       time.Time
	ExpectedVersion int64
	IdempotencyKey  string
}

// Activate 設定自動開立切點（端點 26 ＝ 不變式 I-61）。
//
// 三條硬規則（§6.1.8）：
//
//	① 必須先驗證通過（allowed-from ＝ ACTIVE，在 SQL）。**沒有「測試成功待啟用」的
//	   中間態**——步驟 4 與 5 的分界是 automationCutoverAt 有沒有值，不是 status。
//	② cutoverAt 以 paid_at 判定適用範圍：之後付款的訂單走 AMEGO_AUTO，之前的維持原樣。
//	③ **不接受回溯**：cutoverAt < now 一律 400。把新設定套用到歷史訂單，等於對一批
//	   已經以別的方式處理過的訂單重新開票。
//
// ⚠️ 本支**不寫 status**：§7.3.4 P03 是「三項全過直接進 ACTIVE」，這裡只補切點。
func (u *ProviderAccount) Activate(ctx context.Context, in ActivateProviderAccountInput) (ProviderAccountView, error) {
	if in.CutoverAt.IsZero() {
		return ProviderAccountView{}, ErrProviderCutoverInvalid
	}
	now := u.now()
	if in.CutoverAt.Before(now) {
		return ProviderAccountView{}, ErrProviderCutoverInPast
	}

	var out ProviderAccountView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, account, err := u.loadAccount(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if account.Version != in.ExpectedVersion {
			return providerVersionConflict()
		}
		ok, err := u.accounts.SetAutomationCutover(ctx, account.ID, in.ExpectedVersion, in.CutoverAt, now)
		if err != nil {
			return err
		}
		if !ok {
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}
		if herr := u.appendHistory(ctx, account.ID, account.Status, account.Status,
			providerReasonCutoverSet, "cutoverAt="+in.CutoverAt.UTC().Format(time.RFC3339),
			providerActorSeller, in.StoreID, now); herr != nil {
			return herr
		}

		if _, cerr := u.cutovers.CloseOpen(ctx, account.ID, now); cerr != nil {
			return cerr
		}
		if _, cerr := u.cutovers.Create(ctx, domain.NewProviderCutover{
			ProviderAccountID:  account.ID,
			IssuerID:           account.IssuerID,
			AccountVersion:     in.ExpectedVersion + 1,
			CutoverAt:          in.CutoverAt,
			CreatedByAccountID: in.StoreID,
			Reason:             providerReasonCutoverSet,
			Now:                now,
		}); cerr != nil {
			return cerr
		}

		out, err = u.viewOf(ctx, in.StoreID, issuer)
		return err
	})
	if err != nil {
		return ProviderAccountView{}, err
	}
	return out, nil
}

// RotateProviderCredentialsInput 是端點 27 的輸入。
type RotateProviderCredentialsInput struct {
	StoreID         int64
	AppKey          string
	ExpectedVersion int64
	IdempotencyKey  string
}

// RotateCredentials 輪替 AMEGO 憑證（端點 27 ＝ P11 → P12／P13）。
//
// 🔴 **必須先驗證新憑證再覆蓋**（§6.1.8 端點 27）：覆蓋後才驗證失敗 ＝ 該賣家從那一刻
// 起每一筆開票都失敗，而 UI 上 status 仍是 ACTIVE。所以順序固定是
// 「ACTIVE → ROTATION_PENDING（寫 pending_*，舊 key 原封不動）→ 打 AMEGO → 成功才扶正」。
//
// 🔴 U3（§8.1.1(f)）：送來的 key 就是現行那把時**什麼都不做**——不寫欄位、不打 AMEGO。
// 放行的後果是賣家以為換過了，而外洩的那一把還在用。
func (u *ProviderAccount) RotateCredentials(ctx context.Context, in RotateProviderCredentialsInput) (ProviderAccountView, error) {
	if err := requireAppKey(in.AppKey); err != nil {
		return ProviderAccountView{}, err
	}

	var (
		issuer  domain.Issuer
		account domain.ProviderAccount
		version int64
	)
	now := u.now()
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		issuer, account, err = u.loadAccount(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if account.Version != in.ExpectedVersion {
			return providerVersionConflict()
		}
		same, err := u.accounts.MatchesCurrentCredential(ctx, account.ID, account.ProviderTaxID, in.AppKey)
		if err != nil {
			return err
		}
		if same {
			return providerBizWithMessage(bizcode.InvoiceProviderStateInvalid,
				"送出的 AppKey 與現行憑證相同，未進行輪替")
		}
		if rerr := u.accounts.StartCredentialRotation(ctx, domain.StartCredentialRotation{
			ID:              account.ID,
			ProviderTaxID:   account.ProviderTaxID,
			AppKey:          in.AppKey,
			ExpectedVersion: in.ExpectedVersion,
		}); rerr != nil {
			return rerr
		}
		version = in.ExpectedVersion + 1
		return u.appendHistory(ctx, account.ID, account.Status,
			codemeta.InvoiceProviderAccountStatusRotationPending.Value,
			providerReasonRotationStarted, "", providerActorSeller, in.StoreID, now)
	})
	if err != nil {
		return ProviderAccountView{}, err
	}

	outcome := u.verifyCredential(ctx, amego.Credential{
		IssuerID: issuer.ID, Invoice: account.ProviderTaxID, AppKey: in.AppKey,
	})

	var out ProviderAccountView
	landedAt := u.now()
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		if outcome.BizCode == 0 {
			if perr := u.accounts.PromoteCredential(ctx, domain.PromoteCredential{
				ID:              account.ID,
				ProviderTaxID:   account.ProviderTaxID,
				AppKey:          in.AppKey,
				NextStatus:      codemeta.InvoiceProviderAccountStatusActive.Value,
				ExpectedVersion: version,
			}); perr != nil {
				return perr
			}
		} else {
			ok, aerr := u.accounts.AbortCredentialRotation(ctx, account.ID, version, outcome.Reason, landedAt)
			if aerr != nil {
				return aerr
			}
			if !ok {
				return errProviderVerifyStateDrift
			}
		}
		if herr := u.appendHistory(ctx, account.ID,
			codemeta.InvoiceProviderAccountStatusRotationPending.Value,
			codemeta.InvoiceProviderAccountStatusActive.Value,
			providerReasonRotationLanded, firstNonEmpty(outcome.Reason, "rotated"),
			providerActorSeller, in.StoreID, landedAt); herr != nil {
			return herr
		}
		var verr error
		out, verr = u.viewOf(ctx, in.StoreID, issuer)
		return verr
	})
	if err != nil {
		return ProviderAccountView{}, err
	}
	if outcome.BizCode != 0 {
		return out, providerBizWithMessage(outcome.BizCode, outcome.Reason)
	}
	return out, nil
}

// DeactivateForAdmin 停用 provider 連線（端點 45 ＝ P14）。可逆。
//
// 🔴 **同一交易內把該 issuer 全部未終態 operation 轉 FAILED_PERMANENT**
// （cancel_reason_code=PROVIDER_SUSPENDED_BY_ADMIN）。放著不動的病是：憑證停用後 job
// 仍在重試，每次拿 code 13（商戶未啟用），反覆重試到 FAILED_RETRY_EXHAUSTED，而法定期限
// 持續倒數、賣家與 ADM 都看不出根因是自己按的那顆停用鈕。
//
// ⚠️ **P7 `invoice.operation.review_required` 本波未發**：§7.3.4 P14 要求「各發一則」，
// 但 §6.3.3.3 把該事件的 review_reason_code 值域限死在 CodeMeta InvoiceManualReason（32 值），
// 而 PROVIDER_SUSPENDED_BY_ADMIN 屬 InvoiceCancelReason、不在那 32 值內 ⇒ 送出前的
// CodeMetaValidator 會擋下，硬湊一個近似碼會讓賣家收到一則說錯原因的通知。
// 契約缺口已登記在交付回報，不在本層自行擴充碼表。
func (u *ProviderAccount) DeactivateForAdmin(ctx context.Context, in AdminProviderActionInput) (ProviderAdminActionResult, error) {
	return u.adminLifecycle(ctx, in, providerLifecycleSpec{
		NextStatus:    codemeta.InvoiceProviderAccountStatusSuspended.Value,
		AuditAction:   providerAuditActionDeactivate,
		HistoryReason: providerReasonAdminDeactivate,
		apply: func(ctx context.Context, account domain.ProviderAccount, now time.Time) (bool, error) {
			return u.accounts.Suspend(ctx, account.ID, in.ExpectedVersion, in.Reason, now)
		},
		failOpen: func(domain.ProviderAccount) bool { return true },
	})
}

// RevokeForAdmin 撤銷 provider 帳號（端點 67 ＝ P16）。**不可逆**：REVOKED 是 M4 唯一終態。
//
// 🔴 破壞性防護逐條（每一條都在 SQL 的 WHERE 或本層的顯式判斷裡，沒有一條靠「查不到就
// 當可以撤銷」）：
//
//	① 帳號必須存在 —— loadAccount 查無回 9330 並**中止**；沒有「找不到就視為已撤銷」的分支
//	② 來源狀態只有 ACTIVE／DEGRADED／SUSPENDED —— 寫在 RevokeProviderAccount 的 WHERE，
//	   rows=0 回 9333。已 REVOKED 的列因此撤不了第二次（也不會靜默成功）
//	③ expectedVersion 必須相符 —— 同一句 WHERE。ADM 看到的若是三分鐘前那一版，該按的
//	   可能是別顆鈕
//	④ confirmed 必須為 true —— §7.3.4 P16 的「二次確認」，缺了回 400
//	⑤ reason 必填且 ≤ 500 字 —— chk_invadmaudit_reason 要求 length ≥ 1，而空理由的稽核列
//	   等於沒有稽核
//	⑥ **不是雙人確認**（P16 逐字）：全案唯一的雙人機制是端點 54／55 的人工解鎖。UI 不得暗示
//	⑦ automation_cutover_at **保留不清空** —— 歷史訂單的模式判定要用它（在 SQL 裡就是
//	   「不寫那一欄」）
//	⑧ 來源非 SUSPENDED 時，同一交易內比照 P14 打掉在途 job —— 否則憑證作廢而 job 還在重試
func (u *ProviderAccount) RevokeForAdmin(ctx context.Context, in AdminProviderActionInput) (ProviderAdminActionResult, error) {
	return u.adminLifecycle(ctx, in, providerLifecycleSpec{
		NextStatus:    codemeta.InvoiceProviderAccountStatusRevoked.Value,
		AuditAction:   providerAuditActionRevoke,
		HistoryReason: providerReasonAdminRevoke,
		apply: func(ctx context.Context, account domain.ProviderAccount, now time.Time) (bool, error) {
			return u.accounts.Revoke(ctx, account.ID, in.ExpectedVersion, in.Reason, now)
		},
		failOpen: func(a domain.ProviderAccount) bool {
			return a.Status != codemeta.InvoiceProviderAccountStatusSuspended.Value
		},
	})
}

type providerLifecycleSpec struct {
	NextStatus    string
	AuditAction   string
	HistoryReason string
	apply         func(ctx context.Context, account domain.ProviderAccount, now time.Time) (bool, error)
	failOpen      func(domain.ProviderAccount) bool
}

func (u *ProviderAccount) adminLifecycle(
	ctx context.Context, in AdminProviderActionInput, spec providerLifecycleSpec,
) (ProviderAdminActionResult, error) {
	if err := requireProviderReason(in.Reason); err != nil {
		return ProviderAdminActionResult{}, err
	}
	if !in.Confirmed {
		return ProviderAdminActionResult{}, ErrProviderConfirmRequired
	}

	var out ProviderAdminActionResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		_, account, err := u.loadAccount(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if account.Version != in.ExpectedVersion {
			return providerVersionConflict()
		}
		now := u.now()

		pending, err := u.ops.CountOpenForIssuer(ctx, account.IssuerID)
		if err != nil {
			return err
		}

		ok, err := spec.apply(ctx, account, now)
		if err != nil {
			return err
		}
		if !ok {
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}

		if spec.failOpen(account) {
			if _, ferr := u.ops.FailOpenForIssuer(ctx, domain.FailOpenOperationsForIssuer{
				IssuerID:         account.IssuerID,
				CancelReasonCode: codemeta.InvoiceCancelReasonProviderSuspendedByAdmin.Value,
				LastError:        spec.AuditAction + ": " + truncateRunes(in.Reason, providerReasonMaxRunes),
				Now:              now,
			}); ferr != nil {
				return ferr
			}
			n, aerr := u.adjustments.CancelUnsentForIssuer(ctx, account.IssuerID,
				codemeta.InvoiceCancelReasonProviderSuspendedByAdmin.Value, now)
			if aerr != nil {
				return aerr
			}
			if n > 0 {
				u.log.Info("invoice.provider_account.adjustments_cancelled",
					zap.Int64("issuer_id", account.IssuerID), zap.Int64("rows", n),
					zap.String("action", spec.AuditAction))
			}
		}

		if spec.NextStatus == codemeta.InvoiceProviderAccountStatusRevoked.Value {
			if _, cerr := u.cutovers.CloseOpen(ctx, account.ID, now); cerr != nil {
				return cerr
			}
		}

		if herr := u.appendHistory(ctx, account.ID, account.Status, spec.NextStatus,
			spec.HistoryReason, in.Reason, providerActorAdmin, in.ActorAccountID, now); herr != nil {
			return herr
		}

		auditID, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: in.ActorAccountID,
			Action:         spec.AuditAction,
			ResourceType:   providerResourceType,
			ResourceID:     account.ID,
			BeforeState:    jobAuditState(account.Status, nil),
			AfterState: jobAuditState(spec.NextStatus, map[string]string{
				"pendingOrderCount": strconv.FormatInt(pending, 10),
			}),
			ReasonCode:   spec.AuditAction,
			ReasonDetail: in.Reason,
			RequestID:    nilIfEmpty(in.IdempotencyKey),
			Now:          now,
		})
		if err != nil {
			return err
		}

		out = ProviderAdminActionResult{
			StoreID:           in.StoreID,
			Status:            spec.NextStatus,
			OccurredAt:        now,
			PendingOrderCount: pending,
			AuditLogID:        auditID,
			AffectedScope:     providerAffectedScopeNotice,
		}
		return nil
	})
	if err != nil {
		return ProviderAdminActionResult{}, err
	}
	return out, nil
}

type providerVerifyOutcome struct {
	NextStatus      string
	Reason          string
	BizCode         int32
	ProviderCode    string
	CredentialFault bool
}

func (u *ProviderAccount) runVerification(ctx context.Context, account domain.ProviderAccount, issuerID int64) providerVerifyOutcome {
	var outcome providerVerifyOutcome
	err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(taxID, appKey string) error {
		outcome = u.verifyCredential(ctx, amego.Credential{IssuerID: issuerID, Invoice: taxID, AppKey: appKey})
		return nil
	})
	if err != nil {
		u.logger().Error("invoice.provider.credential_open.failed", zap.Error(err))
		return providerVerifyOutcome{
			NextStatus: account.Status,
			Reason:     "憑證解密失敗，請聯絡平台",
			BizCode:    bizcode.InvoiceProviderUnavailable,
		}
	}
	return outcome
}

func (u *ProviderAccount) verifyCredential(ctx context.Context, cred amego.Credential) providerVerifyOutcome {
	if _, err := u.provider.QueryCompanyNames(ctx, cred, []string{cred.Invoice}); err != nil {
		return providerOutcomeFromError(err)
	}
	year, period := domain.AmegoPeriod(u.now())
	tracks, err := u.provider.ListTracks(ctx, cred, year, period)
	if err != nil {
		return providerOutcomeFromError(err)
	}
	if !providerHasUsableTrack(tracks) {
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			Reason:          "本期沒有可用的發票字軌，請先於光貿後台完成字軌配號",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			CredentialFault: true,
		}
	}
	return providerVerifyOutcome{NextStatus: codemeta.InvoiceProviderAccountStatusActive.Value}
}

func providerHasUsableTrack(tracks []amego.Track) bool {
	for _, t := range tracks {
		if t.TrackAPICode != "" {
			return true
		}
		if providerHasUsableTrack(t.Data) {
			return true
		}
	}
	return false
}

func providerOutcomeFromError(err error) providerVerifyOutcome {
	var apiErr *amego.APIError
	if !errors.As(err, &apiErr) {
		return providerVerifyOutcome{
			NextStatus: codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			Reason:     "無法連線至電子發票服務商，請稍後再試",
			BizCode:    bizcode.InvoiceProviderUnavailable,
		}
	}
	switch apiErr.Code {
	case "12", "16":
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusInvalidCredential.Value,
			Reason:          "憑證格式或內容錯誤，請確認統一編號與 AppKey",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			ProviderCode:    apiErr.Code,
			CredentialFault: true,
		}
	case "13", "19", "22":
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			Reason:          "光貿帳號尚未開通或已停權，請先於光貿後台完成開通",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			ProviderCode:    apiErr.Code,
			CredentialFault: true,
		}
	case amego.CodeEnvGuard:
		return providerVerifyOutcome{
			NextStatus:   codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			Reason:       "本環境未開放對外開立發票，無法驗證憑證",
			BizCode:      bizcode.InvoiceProviderUnavailable,
			ProviderCode: apiErr.Code,
		}
	default:
		if apiErr.Class == amego.ClassManual || apiErr.Class == amego.ClassPermanent {
			return providerVerifyOutcome{
				NextStatus:   codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
				Reason:       providerManualReason(apiErr),
				BizCode:      bizcode.InvoiceProviderCredentialInvalid,
				ProviderCode: apiErr.Code,
			}
		}
		return providerVerifyOutcome{
			NextStatus:   codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			Reason:       "電子發票服務商暫時無法連線，請稍後再試（請勿更改 AppKey）",
			BizCode:      bizcode.InvoiceProviderUnavailable,
			ProviderCode: apiErr.Code,
		}
	}
}

const providerManualReasonMaxRunes = 120

func providerManualReason(apiErr *amego.APIError) string {
	msg := strings.TrimSpace(apiErr.Msg)
	if r := []rune(msg); len(r) > providerManualReasonMaxRunes {
		msg = string(r[:providerManualReasonMaxRunes]) + "…"
	}
	if msg == "" {
		return fmt.Sprintf("電子發票服務商回報錯誤（代碼 %s）。重試不會改善，請聯繫光貿或平台客服", apiErr.Code)
	}
	return fmt.Sprintf("電子發票服務商回報：%s（代碼 %s）。重試不會改善，請依訊息處理或聯繫光貿", msg, apiErr.Code)
}

func (u *ProviderAccount) loadAccount(ctx context.Context, storeID int64) (domain.Issuer, domain.ProviderAccount, error) {
	issuer, err := u.sellerIssuer(ctx, storeID)
	if err != nil {
		return domain.Issuer{}, domain.ProviderAccount{}, err
	}
	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
	if err != nil {
		return domain.Issuer{}, domain.ProviderAccount{}, err
	}
	return issuer, account, nil
}

func (u *ProviderAccount) sellerIssuer(ctx context.Context, storeID int64) (domain.Issuer, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err == nil {
		return issuer, nil
	}
	if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
		return domain.Issuer{}, providerBiz(bizcode.InvoiceProviderAccountNotFound)
	}
	return domain.Issuer{}, err
}

func (u *ProviderAccount) viewOf(ctx context.Context, storeID int64, issuer domain.Issuer) (ProviderAccountView, error) {
	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
	if err != nil {
		return ProviderAccountView{}, err
	}
	view := ProviderAccountView{StoreID: storeID, Account: account}
	if profile, perr := u.profiles.GetByIssuer(ctx, issuer.ID); perr == nil {
		view.Profile, view.HasProfile = profile, true
	} else if !isBizCode(perr, bizcode.SellerTaxProfileNotFound) {
		return ProviderAccountView{}, perr
	}
	return view, nil
}

func (u *ProviderAccount) appendHistory(
	ctx context.Context, accountID int64, from, to, reasonCode, reasonDetail, actorType string,
	actorAccountID int64, now time.Time,
) error {
	return u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    providerSubjectType,
		SubjectID:      accountID,
		FromStatus:     nilIfEmpty(from),
		ToStatus:       to,
		ReasonCode:     nilIfEmpty(reasonCode),
		ReasonDetail:   nilIfEmpty(truncateRunes(reasonDetail, providerReasonMaxRunes)),
		ActorType:      actorType,
		ActorAccountID: &actorAccountID,
		Now:            now,
	})
}

func (u *ProviderAccount) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

func requireProviderAMEGO(provider string) error {
	if provider == "" || provider == providerAMEGO {
		return nil
	}
	return ErrProviderUnsupported
}

func requireAppKey(appKey string) error {
	if isBlank(appKey) {
		return ErrProviderAppKeyRequired
	}
	if utf8.RuneCountInString(appKey) > providerAppKeyMaxRunes {
		return ErrProviderAppKeyTooLong
	}
	return nil
}

func requireConsentVersion(v string) error {
	if isBlank(v) {
		return ErrProviderConsentRequired
	}
	if utf8.RuneCountInString(v) > taxAttestationVersionMax {
		return ErrProviderConsentTooLong
	}
	return nil
}

func requireProviderReason(reason string) error {
	if isBlank(reason) {
		return ErrProviderReasonRequired
	}
	if utf8.RuneCountInString(reason) > providerReasonMaxRunes {
		return ErrProviderReasonTooLong
	}
	return nil
}

func providerBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func providerBizWithMessage(code int32, msg string) error {
	return pkgerrors.NewBusiness(code, firstNonEmpty(msg, bizcode.Message(code)))
}

func providerVersionConflict() error {
	return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
		bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
}
