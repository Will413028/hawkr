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

// 本檔是 M4「AMEGO 帳戶」狀態機（§7.3.4 P01–P16）的**寫入面**：賣家側 §6.1.8 的端點
// 24／25／26／27，ADM 側 §6.1.13 的端點 44／45／66／67。讀取面（端點 23／42／43）在
// query.go 與 query_read.go。
//
// 六條貫穿全檔的約束：
//
//	① **allowed-from 只有一份，住在 SQL 的 WHERE**（db/queries/invoice_provider_accounts.sql）。
//	   本層先讀一次是為了回**可分辨的碼**（9321／9322／9330／9331／9332／9333／9334／9337），
//	   不是正確性防線——兩個分頁同時送出時應用層的 if 會各自讀到舊值。真正的守衛是那句
//	   CAS，而 **rows=0 一律回明確業務碼，絕不當成功往下走**。
//	② **AppKey 明文只往內、不往外**：request → usecase 區域變數 → repository 的
//	   deriveCredential。它不進任何 struct 欄位（除了 Input 本身）、不進 log、不進回應、
//	   不進稽核。要讀既有那把只能經 accounts.WithDecryptedCredential，而它連回傳值都沒有。
//	③ **打 AMEGO 一律在交易之外**：驗證是同步等上游的網路呼叫（deadline 15 s），包進
//	   交易等於讓一把資料庫連線被 AMEGO 的延遲綁住。三段式因此是
//	   「tx1 推進 CREDENTIAL_VERIFYING → 網路 → tx2 落地結果」。
//	④ **ADM 側每一支都寫 invoice_admin_audit_logs**，而 before/after **只放狀態欄**。
//	   那張表保存 5 年，把統編／AppKey／last4 複製進去等於憑空造一個五年期的憑證倉庫。
//	⑤ **Tier A**：賣家側的歸屬鍵是 store_id，經 invoice_issuers.issuer_ref_id 解析。
//	   別的 store_id 解出別的 issuer ⇒ 拿不到這一列。越權與查無共用 9330、同一段 message、
//	   同一行程式碼。
//	⑥ **環境閘門不在本層**：AMEGO_ALLOW_REAL_INVOICE 由 amego client 的 guardEnvironment
//	   負責（§4.5.8）。本層看到 ENV_GUARD 只做一件事——當成「上游不可用」（9332）並且
//	   **不改 status**，因為那是我方的環境設定問題，不是這位賣家的憑證問題。

const (
	// providerHealthCheckInterval 是一次驗證之後的下一次巡檢時點（§7.3.4 P06 的 15 分鐘，
	// 與 db/queries 的 RecordHealthCheckResult 同值）。
	//
	// 兩處同值而不共用常數是既有結構的現況（那一支在 SQL 註解裡）；本層寫死同一個數字，
	// 改週期時兩邊都要動——缺口登記在交付回報。
	providerHealthCheckInterval = 15 * time.Minute

	// providerReasonMaxRunes 是 ADM 理由欄的長度上限，與 A05／A06 同值（58A maxLength=500）。
	providerReasonMaxRunes = adminReasonMaxRunes

	// providerAppKeyMaxRunes 是 AppKey 的長度上界。
	//
	// AMEGO 沒有公告長度，這裡擋的是「有人把一整份 PEM 貼進來」：credential_ciphertext
	// 是 text 欄、DB 不會擋，而加密後的密文會一路寫進去。取 256 是「遠大於任何實際
	// AppKey、又遠小於一份憑證檔」的區間。
	providerAppKeyMaxRunes = 256

	// invoice_status_history.reason_code 的 provider 軸（自由 varchar(48)，不是 CodeMeta）。
	providerReasonCredentialSubmitted = "SELLER_PROVIDER_CREDENTIAL_SUBMITTED"
	providerReasonVerifyStarted       = "PROVIDER_VERIFICATION_STARTED"
	providerReasonVerifyLanded        = "PROVIDER_VERIFICATION_RESULT"
	providerReasonCutoverSet          = "SELLER_PROVIDER_CUTOVER_SET"
	providerReasonRotationStarted     = "SELLER_PROVIDER_ROTATION_STARTED"
	providerReasonRotationLanded      = "SELLER_PROVIDER_ROTATION_RESULT"
	providerReasonAdminDeactivate     = "ADMIN_PROVIDER_DEACTIVATE"
	providerReasonAdminReactivate     = "ADMIN_PROVIDER_REACTIVATE"
	providerReasonAdminRevoke         = "ADMIN_PROVIDER_REVOKE"

	// invoice_admin_audit_logs.action（前綴與既有的 INVOICE_* 一致）。
	providerAuditActionHealthCheck = "INVOICE_PROVIDER_HEALTH_CHECK"
	providerAuditActionDeactivate  = "INVOICE_PROVIDER_DEACTIVATE"
	providerAuditActionReactivate  = "INVOICE_PROVIDER_REACTIVATE"
	providerAuditActionRevoke      = "INVOICE_PROVIDER_REVOKE"

	// invoice_status_history.subject_type（chk_invhist_subject 七值之一）
	// ＋ invoice_admin_audit_logs.resource_type。
	providerSubjectType  = "PROVIDER_ACCOUNT"
	providerResourceType = "PROVIDER_ACCOUNT"

	// invoice_status_history.actor_type（chk_invhist_actor 五值之一）。
	providerActorSeller = "SELLER"
	providerActorAdmin  = "ADMIN"

	// providerAffectedScopeNotice 是端點 45／67 回應的 affectedScopeNotice。
	//
	// 🔴 由**後端**產生且內容固定（§6.1.13 端點 45 逐字）：讓前端各自寫一句話，
	// 兩支端點與兩個 repo 會長出四個不同的說法，而這句話講的是稅務效力的範圍。
	providerAffectedScopeNotice = "只影響停用時點之後付款的訂單；較早付款的訂單維持原開票方式。"
)

// providerTaxIDPattern 是統編格式（chk_ipa_tax_id 的同一條）。
//
// 🔴 DB 端那道回的是 23514（對賣家是一句 500），本層這道才回得出「統一編號格式不正確」。
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
	tx       database.TxManager
	issuers  domain.IssuerRepository
	profiles domain.SellerTaxProfileRepository
	accounts domain.ProviderAccountRepository
	ops      domain.OperationRepository
	// adjustments 是 P14／P16 的第三個副作用（#3073）：憑證停用之後，該 issuer 底下
	// 還沒送出去的調整永遠送不出去了。少了它，那些列停在 PENDING／SENDING 而
	// operation 已經 FAILED_PERMANENT——dev 上實測 8 筆，最老的三天前。
	adjustments domain.AdjustmentRepository
	audit       domain.AuditRepository
	// cutovers 是自動開立切點的**歷史**（§5.1.15）。它與 accounts 分開是因為
	// invoice_provider_accounts 只存現值：改一次就把上一次抹掉，而端點 46 要的是清單。
	cutovers domain.ProviderCutoverRepository
	provider ProviderClient

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

// ---------------------------------------------------------------------------
// 賣家：端點 24 憑證寫入（write-only）
// ---------------------------------------------------------------------------

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
				// 還沒有稅務資格列 ⇒ 「當前狀態不允許」而不是「找不到 provider 帳號」：
				// 賣家的下一步是去 S02 送審。
				return providerBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			return err
		}
		// §6.1.8 端點 24：稅務身分未核准 → 9321。
		//
		// 只認 APPROVED_INVOICE_REQUIRED 一個值（正面列舉）：免用統一發票
		// （APPROVED_INVOICE_EXEMPT）走的是收據授權 S05，不會有 AMEGO 憑證。
		if profile.ReviewStatus != codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value {
			return providerBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		// 9334：送出統編與已核定稅籍不符。
		//
		// 🔴 這一格必須在寫入**之前**擋：開錯抬頭的發票不能改，只能作廢重開，而作廢
		// 有法定窗口。核定統編缺席也走這裡——那代表核定資料不完整，一樣不能放行。
		if profile.TaxID == nil || *profile.TaxID != in.SellerTaxID {
			return providerBiz(bizcode.InvoiceProviderTaxidMismatch)
		}

		now := u.now()
		account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
		switch {
		case err == nil:
			// 既有列 ⇒ 覆寫分支。allowed-from 在 SQL（不含 ACTIVE／DEGRADED：
			// 那兩個狀態換 key 一律走端點 27 輪替）。
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
				// 🔴 交易此刻已 aborted（25P02）⇒ 直接回，不得再下任何一句 SQL
				// （包含狀態歷程）。整筆 rollback 正是我們要的。
				return providerBiz(bizcode.InvoiceProviderCredentialTaken)
			}
			if !ok {
				// allowed-from 不符（ACTIVE／DEGRADED／ROTATION_PENDING／SUSPENDED／
				// REVOKED）或版本被搶先。兩者共用 9333，訊息由 CodeMeta 給。
				return providerBiz(bizcode.InvoiceProviderStateInvalid)
			}
			if herr := u.appendHistory(ctx, account.ID, account.Status,
				codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
				providerReasonCredentialSubmitted, "consentVersion="+in.ConsentVersion,
				providerActorSeller, in.StoreID, now); herr != nil {
				return herr
			}
		case isBizCode(err, bizcode.InvoiceProviderAccountNotFound):
			// 首次綁定（P01）。**沒有列 ⇒ 沒有版本可以對**，所以契約上的
			// expectedVersion 只能是 0；帶別的值代表前端手上那一份不是「還沒設定」。
			if in.ExpectedVersion != 0 {
				return providerVersionConflict()
			}
			id := u.newID()
			if cerr := u.accounts.Create(ctx, domain.NewProviderAccount{
				ID:            id,
				IssuerID:      issuer.ID,
				Provider:      providerAMEGO,
				ProviderTaxID: in.SellerTaxID,
				AppKey:        in.AppKey,
				// 建列即 CREDENTIAL_VERIFYING：P01＋P02 合併，理由見函式註解。
				Status:            codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value,
				HealthCheckNextAt: now,
			}); cerr != nil {
				// Create 已把 uq_ipa_fingerprint 的 rows=0 翻成 9337（它走
				// ON CONFLICT DO NOTHING，交易沒有 abort）。
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

// ---------------------------------------------------------------------------
// 賣家：端點 25 測試連線／ADM：端點 44 主動連線檢查、端點 66 恢復連線
// ---------------------------------------------------------------------------

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
		// 賣家側不寫 ADM 稽核（那張表是 ADM 動作的 5 年保存紀錄）。
		AuditAction: "",
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
		// P15 的 allowed-from 只有 SUSPENDED。本層先讀一次是為了回可分辨的 9333。
		RequireStatus:  codemeta.InvoiceProviderAccountStatusSuspended.Value,
		ActorType:      providerActorAdmin,
		ActorAccountID: in.ActorAccountID,
		AuditAction:    providerAuditActionReactivate,
		AuditReason:    in.Reason,
		HistoryReason:  providerReasonAdminReactivate,
		IdempotencyKey: in.IdempotencyKey,
	})
}

// providerVerifyRequest 是端點 25／44／66 的共通形狀。
type providerVerifyRequest struct {
	StoreID         int64
	ExpectedVersion int64
	// UseCurrent＝true 時忽略 ExpectedVersion，改用讀出當下的版本（端點 44 沒有這個欄位）。
	UseCurrent bool
	// RequireStatus 非空時要求來源狀態恰為該值（端點 66 的 SUSPENDED）。
	RequireStatus string

	ActorType      string
	ActorAccountID int64

	// AuditAction 為空 ⇒ 不寫 ADM 稽核（賣家側）。
	AuditAction    string
	AuditReason    string
	HistoryReason  string
	IdempotencyKey string
}

// verifyFlow 是端點 25／44／66 的三段式：tx1 推進 → 網路 → tx2 落地。
//
// 🔴 **AMEGO 呼叫必須在兩個交易之間**（約束③）：15 s 的同步等待包進交易，等於讓一把
// 資料庫連線被上游的延遲綁住，而 A04 的「重新測試」是 ADM 會連按的按鈕。
//
// ⚠️ 兩段之間崩潰 ⇒ 該列停在 CREDENTIAL_VERIFYING。健康檢查排程撈不到它
// （候選集合只有 ACTIVE／DEGRADED），只能由賣家或 ADM 再按一次回來——這正是
// StartVerification 的 allowed-from 含 CREDENTIAL_VERIFYING 的理由。
func (u *ProviderAccount) verifyFlow(ctx context.Context, req providerVerifyRequest) (ProviderAccountView, error) {
	// ── 第一段：讀出、判可分辨的碼、推進到 CREDENTIAL_VERIFYING ──
	var (
		issuer   domain.Issuer
		account  domain.ProviderAccount
		fromLive bool // 來源是 ACTIVE／DEGRADED ⇒ 走「不改 status」那條路（定版③）
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
			// §7.3.4 沒有 ACTIVE → CREDENTIAL_VERIFYING 這條邊。ACTIVE／DEGRADED
			// 的重測**不推進中間態**，結果直接落在 TouchHealthCheck 或
			// FailVerificationFromActive 上。
			return nil
		}
		ok, err := u.accounts.StartVerification(ctx, account.ID, version, now)
		if err != nil {
			return err
		}
		if !ok {
			// allowed-from 不符（NOT_CONFIGURED／ROTATION_PENDING／REVOKED）或版本被搶先。
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

	// ── 第二段：打 AMEGO（唯讀，交易之外）──
	outcome := u.runVerification(ctx, account, issuer.ID)

	// ── 第三段：落地 ──
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
				// 🔴 before/after 只放狀態欄：這張表保存 5 年，把統編／last4 複製進去
				// 等於憑空造一個五年期的憑證倉庫（§6.1.14 端點 48 的同一條）。
				BeforeState:  jobAuditState(account.Status, nil),
				AfterState:   jobAuditState(outcome.NextStatus, map[string]string{"providerCode": outcome.ProviderCode}),
				ReasonCode:   req.AuditAction,
				ReasonDetail: firstNonEmpty(req.AuditReason, "ADM 連線檢查"),
				RequestID:    nilIfEmpty(req.IdempotencyKey),
				Now:          landedAt,
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
	// 業務失敗的碼在狀態落地**之後**才回：狀態必須先寫進去，賣家 S03 才看得到
	// 「憑證錯誤」而不是一個什麼都沒發生的紅字。
	if outcome.BizCode != 0 {
		return view, providerBizWithMessage(outcome.BizCode, outcome.Reason)
	}
	return view, nil
}

// landVerification 依「來源是不是 ACTIVE／DEGRADED」選一條落地路徑。
//
// 三格（§6.1.13 端點 44 定版③）：
//
//	來源 ACTIVE/DEGRADED ＋ 憑證類錯誤 → FailVerificationFromActive（賣家修得掉的狀態）
//	來源 ACTIVE/DEGRADED ＋ 通過或上游不可用 → TouchHealthCheck（**status 完全不動**）
//	其餘來源                              → LandVerification（P03–P06）
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

	// 🔴 verified_at 逐次明給：落 ACTIVE 時 now，落失敗狀態時**沿用原值**。
	// 傳 nil 會把它清成 NULL，而那一列若已設過 automation_cutover_at 就當場 23514
	// （chk_ipa_cutover_requires_verified）。
	verifiedAt := account.VerifiedAt
	if outcome.NextStatus == codemeta.InvoiceProviderAccountStatusActive.Value {
		verifiedAt = &now
	}

	// 🔴 breaker_opened_at 是 §7.3.4 P08「自動重驗最多 24 小時」的錨（起算點）。
	// 落 PROVIDER_UNAVAILABLE 時起錶；落其餘三個狀態時清成 NULL——INV.hc 不重驗
	// INVALID_CREDENTIAL／PROVIDER_REJECTED（那兩個要賣家自己改憑證），留著錶只會
	// 讓下一次真的不可用時錨在一個過期的時間點、上界當場已過。
	//
	// **既有的錶不重設**：同一個帳戶連續多輪重驗失敗時，每輪都寫 now 會讓 24 小時
	// 上界永遠不到（每 15 分鐘往後推一次）⇒ 無限重驗，正是上界要防的事。
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
		// 我們自己剛把它推進 CREDENTIAL_VERIFYING、版本也是自己算的 ⇒ rows=0 代表
		// 中間有別人動過這一列。回錯讓整筆 rollback：放行會留下一個「驗證結果沒有
		// 落地、狀態停在驗證中」的組合，而那是沒有任何人會發現的。
		return errProviderVerifyStateDrift
	}
	return nil
}

// providerFromStatus 回「狀態歷程上的來源值」。
//
// 來源是 ACTIVE／DEGRADED 時我們沒有推進中間態 ⇒ from 就是原狀態；其餘來源在第一段
// 已被推到 CREDENTIAL_VERIFYING ⇒ from 是那個中間態。歷程要能還原真的發生過什麼。
func providerFromStatus(original string, fromLive bool) string {
	if fromLive {
		return original
	}
	return codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value
}

// providerIsLive 正面列舉「憑證還在服役」的兩個狀態。
//
// 正面列舉而非 NOT IN：值域日後長胖時，新狀態預設走「一般落地」而不是靜默套用
// 定版③ 的「status 完全不動」。
func providerIsLive(status string) bool {
	switch status {
	case codemeta.InvoiceProviderAccountStatusActive.Value,
		codemeta.InvoiceProviderAccountStatusDegraded.Value:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// 賣家：端點 26 確認啟用切點
// ---------------------------------------------------------------------------

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
			// 未先驗證（status 不是 ACTIVE）或版本被搶先 ⇒ 9333（§6.1.8 端點 26）。
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}
		if herr := u.appendHistory(ctx, account.ID, account.Status, account.Status,
			providerReasonCutoverSet, "cutoverAt="+in.CutoverAt.UTC().Format(time.RFC3339),
			providerActorSeller, in.StoreID, now); herr != nil {
			return herr
		}

		// 切點歷史（§5.1.15，端點 46 的唯一資料來源）。
		//
		// 🔴 **先關舊列再插新列，且與上面那句 CAS 同一交易**：uq_invpc_open_per_account
		// 是「同時只有一列現行有效」的唯一守衛，而它是 partial unique（WHERE
		// deactivated_at IS NULL）—— 少了 CloseOpen，第二次設定切點會撞 23505 而不是
		// 任何業務碼；分兩個交易寫則歷史與現值會不一致，而端點 46 是唯一會讀它的地方。
		//
		// ⚠️ CloseOpen 的 rows=0 **不是守衛不成立**：第一次設定時本來就沒有前一列。
		if _, cerr := u.cutovers.CloseOpen(ctx, account.ID, now); cerr != nil {
			return cerr
		}
		if _, cerr := u.cutovers.Create(ctx, domain.NewProviderCutover{
			ProviderAccountID: account.ID,
			IssuerID:          account.IssuerID,
			// 設定之後的版本：SetAutomationCutover 那一句已經 version = version + 1。
			// 記設定前的值會讓 ADM 拿 ProviderCutoverItem.version 對不回任何一版。
			AccountVersion: in.ExpectedVersion + 1,
			CutoverAt:      in.CutoverAt,
			// 賣家路徑的 actor 是 **store surrogate** 不是登入 principal——與
			// invoice_status_history.actor_account_id 同一個既有慣例（§6 身分分離）。
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

// ---------------------------------------------------------------------------
// 賣家：端點 27 憑證輪替
// ---------------------------------------------------------------------------

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
			// 沒有專屬業務碼描述這一格 ⇒ 借 9333 並自帶訊息。缺口登記在交付回報。
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

	// 新 key 的明文就在 request 裡 ⇒ 驗證不需要解 pending_ciphertext（那條路徑不存在，
	// 而且不該存在：pending 密文的唯一讀者是扶正時的重新加密）。
	outcome := u.verifyCredential(ctx, amego.Credential{
		IssuerID: issuer.ID, Invoice: account.ProviderTaxID, AppKey: in.AppKey,
	})

	var out ProviderAccountView
	landedAt := u.now()
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		if outcome.BizCode == 0 {
			// P12：扶正。一律用 current master key 重新加密（不是把 pending 密文搬過去）。
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
			// P13：保留原 key、不覆蓋。credential_* 一個字都不碰。
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

// ---------------------------------------------------------------------------
// ADM：端點 45 停用連線／端點 67 撤銷
// ---------------------------------------------------------------------------

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
		// 停用一律打掉在途 job。
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
		// 🔴 來源是 SUSPENDED 時**不再打一次**：那批 operation 已經在端點 45 被打成
		// FAILED_PERMANENT 了，再打一次會把 completed_at 往後推、洗掉「什麼時候被停的」。
		failOpen: func(a domain.ProviderAccount) bool {
			return a.Status != codemeta.InvoiceProviderAccountStatusSuspended.Value
		},
	})
}

// providerLifecycleSpec 是端點 45／67 的差異部分。
type providerLifecycleSpec struct {
	NextStatus    string
	AuditAction   string
	HistoryReason string
	apply         func(ctx context.Context, account domain.ProviderAccount, now time.Time) (bool, error)
	failOpen      func(domain.ProviderAccount) bool
}

// adminLifecycle 跑端點 45／67 共通的那條路徑。
//
// 五件事必須同一交易（§7.4 C-8）：狀態 CAS、未終態 operation 集合式轉終態、
// pendingOrderCount 的計數、狀態歷程、ADM 稽核。少了交易會出現「憑證停了但 job 還在跑」
// 或「狀態轉了但沒有人知道是誰轉的」，而 invoice_admin_audit_logs 是 5 年保存義務。
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

		// ① pendingOrderCount 與判定同一交易讀出（事後值不是 preflight）。
		pending, err := u.ops.CountOpenForIssuer(ctx, account.IssuerID)
		if err != nil {
			return err
		}

		// ② 狀態 CAS。allowed-from 在 SQL；rows=0 一律 9333。
		ok, err := spec.apply(ctx, account, now)
		if err != nil {
			return err
		}
		if !ok {
			return providerBiz(bizcode.InvoiceProviderStateInvalid)
		}

		// ③ 未終態 operation 一次轉 FAILED_PERMANENT，
		//    **以及**該 issuer 底下還沒送出去的 adjustment 一次轉 CANCELLED。
		//
		// 🔴 第二句在 #3073 之前不存在，而症狀完全無聲：operation 被打成
		// FAILED_PERMANENT，adjustment 留在 PENDING／SENDING 而沒有任何執行者
		// ——dev 上實測 8 筆，最老的三天前，last_error 全是這條路徑寫的
		// 「INV58R2 測試：暫時停用」。
		//
		// ⚠️ 兩句的 allowed-from **刻意不同**：operation 取「全部未終態」，
		// adjustment 只取「還沒送出去」的四個。WAITING_CONFIRMATION／FAILED_UNKNOWN
		// 的折讓**可能已經在 AMEGO 端生效了**，cancel 它會讓帳對不上；它們留在原狀
		// 由 invoice_adjustment_stuck_rows 曝光給人到 AMEGO 後台對帳。
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

		// ④ 自動開立就此停止 ⇒ 關掉現行那一列切點歷史（§5.1.15：撤銷不新增列）。
		//
		// 🔴 `automation_cutover_at` 欄位本身**保留不清空**（§7.3.4 P16：歷史訂單的模式
		// 判定要用它）；本段關的是歷史列的 deactivated_at，兩者不是同一件事。少了它，
		// 端點 46 會顯示一個「至今仍有效」的切點，而該賣家的連線早就被撤銷了。
		//
		// ⚠️ 只在終態（REVOKED）關：端點 45 的 SUSPENDED 是可逆的（P15 可恢復），
		// 關掉再開會在歷史上造出一段假的「重新設定」，而賣家從未按過端點 26。
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
			// 🔴 只放狀態欄。pendingOrderCount 是「這次打掉了幾筆」，屬狀態不屬 PII。
			BeforeState: jobAuditState(account.Status, nil),
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

// ---------------------------------------------------------------------------
// 驗證（§7.3.4 P03 的三項）
// ---------------------------------------------------------------------------

// providerVerifyOutcome 是一次驗證的結論。
//
// CredentialFault 與 BizCode 是兩個正交的問題：前者答「這是不是這位賣家自己的憑證問題」
// （決定 ACTIVE／DEGRADED 的來源要不要落成錯誤狀態），後者答「要回哪個碼給前端」。
type providerVerifyOutcome struct {
	NextStatus      string
	Reason          string
	BizCode         int32
	ProviderCode    string
	CredentialFault bool
}

// runVerification 解出憑證明文並跑三項檢查。
//
// 明文只以參數形式進入 fn 的區域變數（WithDecryptedCredential 沒有回傳值）⇒ 不可能被
// 誤存進 struct、log 或 response（§8.1.1(h)）。
func (u *ProviderAccount) runVerification(ctx context.Context, account domain.ProviderAccount, issuerID int64) providerVerifyOutcome {
	var outcome providerVerifyOutcome
	err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(taxID, appKey string) error {
		outcome = u.verifyCredential(ctx, amego.Credential{IssuerID: issuerID, Invoice: taxID, AppKey: appKey})
		return nil
	})
	if err != nil {
		// 解不開密文（master key 換過而 _PREV 沒配好）是故障不是業務結果 ⇒ 不落成
		// 賣家的錯誤狀態，回「上游不可用」讓 ADM 去查，並保留 status 不動。
		u.logger().Error("invoice.provider.credential_open.failed", zap.Error(err))
		return providerVerifyOutcome{
			NextStatus: account.Status,
			Reason:     "憑證解密失敗，請聯絡平台",
			BizCode:    bizcode.InvoiceProviderUnavailable,
		}
	}
	return outcome
}

// verifyCredential 跑 §7.3.4 P03 的三項檢查。
//
//	① 簽章：打 /json/ban_query。簽章不過 AMEGO 回 code 16、統編錯回 code 12，
//	   帳號層問題回 13／19／22——三類在回應裡可分辨，這正是它被選為驗證端點的理由。
//	   ⚠️ 設計書 P03 寫的是「故意傳物件、回 code 23 即證明簽章已過」。本實作改送**合法的
//	   陣列**（賣家自己的統編）：同樣是唯讀、同樣不建立任何資料，而錯誤分類完全相同，
//	   差別只在成功時多回一筆公司名稱。刻意送壞資料去換一個錯誤碼，會讓「錯誤」與
//	   「真的壞了」在 provider 稽核表裡混在一起。偏離已登記在交付回報。
//	② 統編相符：cred.Invoice 就是 provider_tax_id，而端點 24 已在寫入前擋過它與已核定
//	   稅籍不符的情形（9334）⇒ 這一項在資料面恆成立，不重複打一次網路。
//	③ 至少一組可用字軌：打 /json/track_all。少了它，賣家會通過全部檢查、看到綠色
//	   「已啟用」，然後每一筆開票都回 3040111 字軌不足——那是重試多久都不會好的錯誤，
//	   而失敗高峰正好落在期初。
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
		// 沒有可用字軌不是憑證錯（AppKey 是對的）也不是上游不可用，而是「請先於光貿
		// 後台完成開通」那一類 ⇒ P05。
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			Reason:          "本期沒有可用的發票字軌，請先於光貿後台完成字軌配號",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			CredentialFault: true,
		}
	}
	return providerVerifyOutcome{NextStatus: codemeta.InvoiceProviderAccountStatusActive.Value}
}

// providerHasUsableTrack 回報 track_all 的結果裡有沒有一組「API 配號」字軌。
//
// 判準取 TrackAPICode 非空：track_status 的說明逐字是「只回**API 配號**類型的字軌」，
// 而那個型別的標記就是這個欄位。Track 是三層巢狀，子層要一起看。
//
// ⚠️ 這是「有沒有配到字軌」而不是「剩幾張」：Track.Status 的值域在 AMEGO 文件與本 repo
// 皆無定義（登記在交付回報），拿一個猜出來的數字當放行條件，會在猜錯的方向上靜默擋住
// 所有賣家。餘量另有 INV.trk 巡檢負責。
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

// providerOutcomeFromError 把 AMEGO 的錯誤折成 P04／P05／P06 三格。
//
// 🔴 default 走 P06（PROVIDER_UNAVAILABLE ＋ 9332）而不是 P04：未知碼判成「你的憑證錯了」
// 會讓賣家把一把正確的 AppKey 刪掉重打，而判成「等一下再試」最壞只是多等一輪。
// 兩種誤判的代價不對稱。
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
		// P04：簽章／統編不符。賣家重新輸入即可修復。
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusInvalidCredential.Value,
			Reason:          "憑證格式或內容錯誤，請確認統一編號與 AppKey",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			ProviderCode:    apiErr.Code,
			CredentialFault: true,
		}
	case "13", "19", "22":
		// P05：帳號狀態問題（未啟用／停權／未申請 API），無自助恢復途徑。
		return providerVerifyOutcome{
			NextStatus:      codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			Reason:          "光貿帳號尚未開通或已停權，請先於光貿後台完成開通",
			BizCode:         bizcode.InvoiceProviderCredentialInvalid,
			ProviderCode:    apiErr.Code,
			CredentialFault: true,
		}
	case amego.CodeEnvGuard:
		// 我方的環境閘門擋下了，根本沒有外呼 ⇒ 這**不是**這位賣家的憑證問題。
		// 回 9332 並讓 status 完全不動：把它記成賣家的帳號狀態，會在環境設定修好之後
		// 留下一個假的錯誤狀態，而沒有人會回來清。
		return providerVerifyOutcome{
			NextStatus:   codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			Reason:       "本環境未開放對外開立發票，無法驗證憑證",
			BizCode:      bizcode.InvoiceProviderUnavailable,
			ProviderCode: apiErr.Code,
		}
	default:
		// 🔴 **先看 Class 再談重試**。上面五個明確碼之外還有一整類「程式無法自行恢復」
		// 的碼（碼表的 ClassManual／ClassPermanent），全部丟進「暫時無法連線，請稍後
		// 再試」等於叫賣家對一個永遠不會好的錯誤重試到底。
		//
		// 2026-09-01 實例：AMEGO 對 ban_query 回 code 99「第1筆 統一編號格式錯誤」，
		// 而賣家看到的是「暫時無法連線，請稍後再試（**請勿更改 AppKey**）」——
		// 兩句話都把他推向錯的下一步。這條 production 走得到：端點 24 對統編只驗
		// `^[0-9]{8}$`、不驗檢查碼，八碼但檢查碼不對的統編存得進來。
		if apiErr.Class == amego.ClassManual || apiErr.Class == amego.ClassPermanent {
			return providerVerifyOutcome{
				NextStatus:   codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
				Reason:       providerManualReason(apiErr),
				BizCode:      bizcode.InvoiceProviderCredentialInvalid,
				ProviderCode: apiErr.Code,
				// CredentialFault 刻意留 false：這一類碼**分不出**是賣家的憑證問題還是
				// 我方送錯參數（99 自己就是「一碼多義」）。把一個正在正常開票的 ACTIVE
				// 帳號依一個分不出來的碼降級，比少降級一次貴得多。
			}
		}
		// P06：停機／連線失敗／限流／斷路器開路／未知碼——這些才真的是「等一下再試」。
		return providerVerifyOutcome{
			NextStatus:   codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			Reason:       "電子發票服務商暫時無法連線，請稍後再試（請勿更改 AppKey）",
			BizCode:      bizcode.InvoiceProviderUnavailable,
			ProviderCode: apiErr.Code,
		}
	}
}

// providerManualReasonMaxRunes 夾住轉述長度。status_reason 是 text 沒有上限，但這句話
// 會原樣顯示在賣家的頁面上。
const providerManualReasonMaxRunes = 120

// providerManualReason 把 AMEGO 的原文轉述給賣家。
//
// 碼表對 99 的註解逐字是「一碼多義，只能解析中文 msg」⇒ 我方**沒有能力**把它翻成一句
// 固定的中文。與其猜一句（猜錯就是把賣家推向錯的下一步），不如原樣轉述並明說這不是
// 重試能解決的。
//
// ⚠️ 只用來**顯示**，不用來分支——APIError.Msg 的註解逐字要求「永不用來做分支」。
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

// ---------------------------------------------------------------------------
// 共用小工具
// ---------------------------------------------------------------------------

// loadAccount 解出 issuer 與 provider 帳號。
//
// 🔴 Tier A：store_id 是 GetByRef 的述詞之一 ⇒ 別的 store_id 解出別的 issuer、拿不到
// 這一列。issuer 查無與 account 查無**共用 9330**——對賣家而言兩者都只是「尚未設定
// 電子發票服務商連線」，而給它們不同的碼等於回答「這個 store 存在，只是還沒設定」。
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

// sellerIssuer 由 store_id 解出賣家 issuer，查無一律 9330。
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

// viewOf 重讀轉移後的單筆。
//
// 重讀而不是就地拼裝：version 的新值由 SQL 算出（version+1），Go 端拼一份等於維護
// 第二套算式，而兩份一旦漂移，前端的下一次 expectedVersion 就永遠對不上。
func (u *ProviderAccount) viewOf(ctx context.Context, storeID int64, issuer domain.Issuer) (ProviderAccountView, error) {
	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
	if err != nil {
		return ProviderAccountView{}, err
	}
	view := ProviderAccountView{StoreID: storeID, Account: account}
	// 沒有稅務資格列不是錯誤：pendingOrderCount 就不填（與 Query.GetSellerProviderAccount
	// 同一條規則）。
	if profile, perr := u.profiles.GetByIssuer(ctx, issuer.ID); perr == nil {
		view.Profile, view.HasProfile = profile, true
	} else if !isBizCode(perr, bizcode.SellerTaxProfileNotFound) {
		return ProviderAccountView{}, perr
	}
	return view, nil
}

// appendHistory 記一次 provider 帳號的狀態轉移。
func (u *ProviderAccount) appendHistory(
	ctx context.Context, accountID int64, from, to, reasonCode, reasonDetail, actorType string,
	actorAccountID int64, now time.Time,
) error {
	return u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType: providerSubjectType,
		SubjectID:   accountID,
		FromStatus:  nilIfEmpty(from),
		ToStatus:    to,
		ReasonCode:  nilIfEmpty(reasonCode),
		// 🔴 reason_detail 永遠不含 AppKey、last4 或 fingerprint：這張表是 append-only
		// 且與稽核同保存期，寫進去就再也拿不出來。
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

// providerVersionConflict 借 9322（碼名是稅務側的）。
//
// ⚠️ 碼名與語意不一致是設計書指定的（§6.1.8 端點 24／26 逐字「版本不符 → 9322」）。
// 正解是新增一個通用的樂觀鎖碼，登記在交付回報。
func providerVersionConflict() error {
	return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
		bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
}
