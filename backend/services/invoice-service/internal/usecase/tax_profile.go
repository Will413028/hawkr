package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

const (
	taxSubmitWindow         = 60 * 24 * time.Hour
	taxReapplyCooldown      = 7 * 24 * time.Hour
	taxApprovalValidity     = 365 * 24 * time.Hour
	taxExtensionDefaultDays = 30
	taxExtensionMaxDays     = 365

	taxEvidenceMax           = 10
	taxReasonMaxRunes        = 500
	taxAttestationVersionMax = 32
	// SelfIssueTermsVersion 是**當期生效**的自行開票責任條款版本，服務端唯一真實來源。
	//
	// 🔴 條款版本不是前端可控值。端點 19 的 accepted_terms 只表達「我同意」，簽下去的
	// 版本一律取本常數；端點 22 收 termsVersion 只為了驗「賣家看到的是不是這一版」
	// （58A §S04 逐字：必須原樣回送，不符回 400）——不是拿來當寫入值的來源。
	// 少了這個常數，賣家可以簽一個平台從未發布過的版本字串，而那份記錄正是
	// 「審核期間發票由賣家自行開立、責任在賣家」的唯一書面憑據。
	//
	// 形狀與買家側法定告知的 invoiceDisclosureTermsVersion（checkout.go）一致：
	// 日期字串、改版時同時改本常數與 docs/legal/ 的條款全文。
	SelfIssueTermsVersion = "2026-10-25"
	taxLegalNameMax       = 120
	taxEntityTypeMax      = 32
	taxResponsibleNameMax = 60
	taxAddressPartMax     = 32

	taxIssuerPlaceholderPrefix = "store-"
)

const (
	taxSubjectType  = "TAX_PROFILE"
	taxResourceType = "SELLER_TAX_PROFILE"

	taxActorSeller = "SELLER"
	taxActorAdmin  = "ADMIN"

	taxReasonCreated            = "SELLER_TAX_DRAFT_CREATED"
	taxReasonReapply            = "SELLER_TAX_REAPPLY"
	taxReasonSubmitted          = "SELLER_TAX_SUBMITTED"
	taxReasonAttested           = "SELLER_TAX_SELF_ISSUE_ATTESTED"
	taxReasonExtensionRequested = "SELLER_TAX_EXTENSION_REQUESTED"
	taxReasonExtensionApproved  = "ADMIN_TAX_EXTENSION_APPROVED"
	taxReasonAdminDecision      = "ADMIN_TAX_DECISION"

	taxAuditActionDecision  = "INVOICE_TAX_REVIEW_DECIDE"
	taxAuditActionExtension = "INVOICE_TAX_EXTENSION_DECIDE"

	outboxAggregateTaxProfile = "TAX_PROFILE"
)

const (
	taxExtensionApprove = "APPROVE"
	taxExtensionReject  = "REJECT"
)

// 型別邊界錯誤（契約違規 ⇒ 呼叫端映成 InvalidArgument／400，不借業務碼）。
//
// 🔴 ErrTaxEvidenceRequired 的 message 是給**賣家**看的動作，不是給工程師看的狀態描述：
// §7.3.3 T02 逐字要求「請至少上傳一份稅籍證明文件」，且明說**不要回 9321**——
// 兩者的下一步完全不同（一個要補檔、一個要等審核）。
var (
	ErrTaxEvidenceRequired    = errors.New("請至少上傳一份稅籍證明文件")
	ErrTaxEvidenceTooMany     = errors.New("invoice: tax evidence exceeds 10 items")
	ErrTaxEvidenceNotOwned    = errors.New("invoice: retained evidence key does not belong to this seller")
	ErrTaxIdentityIncomplete  = errors.New("invoice: tax identity is incomplete")
	ErrTaxReasonRequired      = errors.New("invoice: reason is required")
	ErrTaxReasonTooLong       = errors.New("invoice: reason exceeds 500 characters")
	ErrTaxTermsVersionInvalid = errors.New("invoice: terms_version is required and must be at most 32 characters")
	// ErrTaxTermsVersionStale ＝ 前端回送的條款版本不是當期版本（58A §S04「不符後端回 400」）。
	// 與 ErrTaxTermsVersionInvalid 分開是因為賣家的下一步不同：一個是重整頁面拿到新版條款
	// 再讀一次，一個是這個請求根本沒帶版本。
	ErrTaxTermsVersionStale = errors.New("invoice: terms_version is not the current version; reload the terms page")
	ErrTaxTermsNotAccepted  = errors.New("invoice: accepted must be true")
	ErrTaxDecisionUnknown   = errors.New("invoice: decision is outside the defined value set")
	ErrTaxConfirmRequired   = errors.New("invoice: this decision requires confirmed=true")
	ErrTaxExtendDaysInvalid = errors.New("invoice: extend_days must be between 1 and 365")
	ErrTaxValidUntilInvalid = errors.New("invoice: approval_valid_until must be in the future")

	// 核定依據文件（§6.1.9）的四個契約違規。刻意不借業務碼：沒有任何 9300 段的碼
	// 描述「你只填了一半」或「你選的那份不是這位賣家上傳的」。
	ErrTaxApprovalBasisIncomplete   = errors.New("invoice: approval_basis_key and approval_basis_asset_id must be given together")
	ErrTaxApprovalBasisNotExempt    = errors.New("invoice: approval_basis is only accepted with APPROVE_INVOICE_EXEMPT")
	ErrTaxApprovalBasisNotOwned     = errors.New("invoice: approval_basis_key must be one of the seller's own uploaded tax documents")
	ErrTaxApprovalBasisAssetInvalid = errors.New("invoice: approval_basis_asset_id must be a positive integer")
	ErrTaxLegalNameTooLong          = errors.New("invoice: legal_name exceeds 120 characters")
	// ErrTaxIDInvalid ＝ 賣家自填的統一編號不是 8 位數字（#3089 B 組第 4 項）。
	//
	// 🔴 在此之前這個欄位**零格式驗證**，直接落庫交給 chk_stp_tax_id 擋 ⇒
	// taxId="1234567" 或 "ABCDEFGH" 都是 **HTTP 500**，賣家看到「系統錯誤」。
	// 而買家側的同型欄位一直都走 domain.IsBusinessTaxID ⇒ 兩側標準不一致。
	//
	// 判定複用 domain.IsBusinessTaxID（同一份規則，不另寫第二份）：只驗形狀不驗
	// 檢查碼——檢查碼規則有歷史版本差異，誤判會把一個合法賣家擋在門外，
	// 真正的驗證是 AMEGO 的 ban_query。
	// 空字串合法（chk_stp_tax_id 允許 NULL）：那是「還沒填」或「清掉重填」。
	ErrTaxIDInvalid = errors.New("統一編號必須是 8 位數字")
	// 以下三個是 22001（欄位超長）與 CodeMeta 值域的守衛（#3089 稽核 A-4）。
	// 沒有合適的 9300 段業務碼描述「你這一格填太長了」⇒ 走契約違規（400），
	// 與同檔的 ErrTaxLegalNameTooLong 一致。
	ErrTaxEntityTypeInvalid      = errors.New("組織型態不在可選範圍內")
	ErrTaxResponsibleNameTooLong = errors.New("負責人姓名不得超過 60 字")
	ErrTaxAddressPartTooLong     = errors.New("縣市與行政區各不得超過 32 字")
)

// TaxEvidenceStore 是稅籍證明的媒體換發出口（§6.1.7 換發表）。
//
// 它是介面而不是直接吃 *media.Client，理由與 ProviderClient 相同：本層要能在沒有
// media-service 的情況下被測到，而「換發失敗即中止、不寫 DB」這條規則正是要測的東西。
type TaxEvidenceStore interface {
	// FinalizeEvidence 把 staged token 換成 objectKey 並建立 asset_reference。
	// 任一顆失敗即回 error——呼叫端必須中止整個請求、不寫 DB（§6.1.7「兩步都是致命錯誤」）。
	FinalizeEvidence(ctx context.Context, stagedTokens []string, taxProfileID int64) (objectKeys, assetIDs []string, err error)
	// ReleaseEvidence 回收沒有被留下的資產（best-effort，必須排在業務列寫入之後）。
	ReleaseEvidence(ctx context.Context, taxProfileID int64, keepAssetIDs []string) error
}

// TaxProfile 是 M3 的寫入 usecase。
//
// 它與 Query 分開的理由同 Complaint：只有它需要 TxManager、outbox 與媒體換發。
// 把寫入掛進 Query，「唯讀查詢面」那句話就變成假的。
type TaxProfile struct {
	tx       database.TxManager
	issuers  domain.IssuerRepository
	profiles domain.SellerTaxProfileRepository
	audit    domain.AuditRepository
	outbox   domain.OutboxRepository
	evidence TaxEvidenceStore

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

// NewTaxProfile 組出 M3 的寫入 usecase。
func NewTaxProfile(
	tx database.TxManager,
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	evidence TaxEvidenceStore,
	newID func() int64,
	log *zap.Logger,
) *TaxProfile {
	return &TaxProfile{
		tx: tx, issuers: issuers, profiles: profiles, audit: audit, outbox: outbox,
		evidence: evidence, newID: newID,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *TaxProfile) WithClock(now func() time.Time) *TaxProfile {
	if now != nil {
		u.now = now
	}
	return u
}

// SaveTaxDraftInput 是端點 18 的輸入。
//
// EvidenceObjectKeys 是**保留清單**不是新增清單（§6.1.7）：後端以「保留清單 ∪ 本次換發
// 出的新 key」**覆蓋** DB 欄 ⇒ 不帶它（或空陣列）＝ 清空既有佐證。
type SaveTaxDraftInput struct {
	StoreID              int64
	ExpectedVersion      int64
	EntityType           string
	TaxID                string
	LegalName            string
	ResponsibleName      string
	AddressCity          string
	AddressDistrict      string
	AddressDetail        string
	ClaimedStatus        string
	EvidenceObjectKeys   []string
	EvidenceStagedTokens []string
	// SavedByAccountID 是登入 principal（§4 例外①，Gateway 注入；backend#3048）。
	// 稅籍資料被誰改過屬保存 5 年的稽核軌跡，一個店有多個可登入帳號時只有它答得出人。
	SavedByAccountID int64
}

// SaveDraft 建立或更新稅務資格草稿（端點 18 ＝ T01 ＋ 草稿存檔）。
//
// 四段順序是 §6.1.7「換發」表的順序契約展開：
//
//	① 建列（交易）——refID 必須是 seller_tax_profiles.id，所以列要先在
//	② 換發（網路，致命）——任一顆失敗即中止，一個字都不寫 DB
//	③ 更新（交易）——狀態守衛、CAS、歷程、display_name 同步
//	④ 回收（網路，best-effort）——**只能在 ③ 之後**：顛倒順序會把正在被引用的檔案
//	   交給 media GC 連同 R2 物件實體刪除，而 evidence_object_keys 還指著它
func (u *TaxProfile) SaveDraft(ctx context.Context, in SaveTaxDraftInput) (SellerTaxProfileView, error) {
	if utf8.RuneCountInString(in.LegalName) > taxLegalNameMax {
		return SellerTaxProfileView{}, ErrTaxLegalNameTooLong
	}
	if in.TaxID != "" && !domain.IsBusinessTaxID(in.TaxID) {
		return SellerTaxProfileView{}, ErrTaxIDInvalid
	}
	for _, f := range []struct {
		value string
		max   int
		err   error
	}{
		{in.EntityType, taxEntityTypeMax, ErrTaxEntityTypeInvalid},
		{in.ResponsibleName, taxResponsibleNameMax, ErrTaxResponsibleNameTooLong},
		{in.AddressCity, taxAddressPartMax, ErrTaxAddressPartTooLong},
		{in.AddressDistrict, taxAddressPartMax, ErrTaxAddressPartTooLong},
	} {
		if utf8.RuneCountInString(f.value) > f.max {
			return SellerTaxProfileView{}, f.err
		}
	}
	if in.EntityType != "" {
		if _, ok := codemeta.SellerTaxEntityTypeFromValue(in.EntityType); !ok {
			return SellerTaxProfileView{}, ErrTaxEntityTypeInvalid
		}
	}
	if len(in.EvidenceObjectKeys)+len(in.EvidenceStagedTokens) > taxEvidenceMax {
		return SellerTaxProfileView{}, ErrTaxEvidenceTooMany
	}

	issuer, profile, err := u.ensureProfile(ctx, in.StoreID, in.LegalName, in.SavedByAccountID)
	if err != nil {
		return SellerTaxProfileView{}, err
	}

	current := profile.EvidenceKeys()
	if !isSubset(in.EvidenceObjectKeys, current) {
		return SellerTaxProfileView{}, ErrTaxEvidenceNotOwned
	}

	newKeys, assetIDs, err := u.evidence.FinalizeEvidence(ctx, in.EvidenceStagedTokens, profile.ID)
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	merged := dedupeStrings(append(append([]string{}, in.EvidenceObjectKeys...), newKeys...))
	if len(merged) > taxEvidenceMax {
		return SellerTaxProfileView{}, ErrTaxEvidenceTooMany
	}

	var out SellerTaxProfileView
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		fresh, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		if fresh.Version != in.ExpectedVersion {
			return taxBiz(bizcode.SellerTaxProfileVersionConflict)
		}
		now := u.now()

		if fresh.ReviewStatus == codemeta.SellerTaxReviewStatusRejected.Value ||
			fresh.ReviewStatus == codemeta.SellerTaxReviewStatusExpired.Value {
			ok, err := u.profiles.ResetToDraft(ctx, fresh.ID, now)
			if err != nil {
				return err
			}
			if !ok {
				return taxBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			if err := u.appendHistory(ctx, fresh.ID, &fresh.ReviewStatus,
				codemeta.SellerTaxReviewStatusDraft.Value, taxReasonReapply, "",
				taxActorSeller, actorAccountIDPtr(in.SavedByAccountID), now); err != nil {
				return err
			}
			if fresh, err = u.profiles.GetByIssuer(ctx, issuer.ID); err != nil {
				return err
			}
		}

		if fresh.ReviewStatus != codemeta.SellerTaxReviewStatusDraft.Value {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}

		ok, err := u.profiles.UpdateDraft(ctx, domain.UpdateSellerTaxDraft{
			ID:                 fresh.ID,
			ExpectedVersion:    fresh.Version,
			ClaimedObligation:  normalizeObligation(in.ClaimedStatus),
			EntityTaxType:      in.EntityType,
			LegalName:          in.LegalName,
			TaxID:              in.TaxID,
			ResponsiblePerson:  in.ResponsibleName,
			BusinessAddress:    joinAddress(in.AddressCity, in.AddressDistrict, in.AddressDetail),
			AddressCity:        strings.TrimSpace(in.AddressCity),
			AddressDistrict:    strings.TrimSpace(in.AddressDistrict),
			DefaultTaxCategory: fresh.DefaultTaxCategory,
			EvidenceObjectKeys: merged,
			ReviewNote:         derefString(fresh.ReviewNote),
			Now:                now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}

		if in.LegalName != "" {
			if _, err := u.issuers.UpdateDisplayName(ctx, issuer.ID, in.LegalName, now); err != nil {
				return err
			}
		}

		updated, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		out = SellerTaxProfileView{StoreID: in.StoreID, Issuer: issuer, Profile: updated}
		return nil
	})
	if err != nil {
		return SellerTaxProfileView{}, err
	}

	if len(assetIDs) > 0 {
		if err := u.evidence.ReleaseEvidence(ctx, profile.ID, assetIDs); err != nil {
			u.logger().Warn("invoice.tax_profile.release_evidence.failed",
				zap.Int64("tax_profile_id", profile.ID), zap.Error(err))
		}
	}
	return out, nil
}

func (u *TaxProfile) ensureProfile(
	ctx context.Context, storeID int64, legalName string, actorAccountID int64,
) (domain.Issuer, domain.SellerTaxProfile, error) {
	var issuer domain.Issuer
	var profile domain.SellerTaxProfile
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		now := u.now()

		var err error
		issuer, err = u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
		switch {
		case err == nil:
		case isBizCode(err, bizcode.InvoiceProviderAccountNotFound):
			displayName := legalName
			if displayName == "" {
				displayName = taxIssuerPlaceholderPrefix + strconv.FormatInt(storeID, 10)
			}
			created, inserted, cerr := u.issuers.Create(ctx, domain.NewIssuer{
				ID:          u.newID(),
				IssuerType:  codemeta.InvoiceIssuerTypeSeller.Value,
				IssuerRefID: storeID,
				DisplayName: displayName,
				Now:         now,
			})
			if cerr != nil {
				return cerr
			}
			if !inserted {
				if created, cerr = u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID); cerr != nil {
					return cerr
				}
			}
			issuer = created
		default:
			return err
		}

		profile, err = u.profiles.GetByIssuer(ctx, issuer.ID)
		switch {
		case err == nil:
			return nil
		case isBizCode(err, bizcode.SellerTaxProfileNotFound):
			created, inserted, cerr := u.profiles.Create(ctx, u.newID(), issuer.ID, now)
			if cerr != nil {
				return cerr
			}
			if !inserted {
				if created, cerr = u.profiles.GetByIssuer(ctx, issuer.ID); cerr != nil {
					return cerr
				}
				profile = created
				return nil
			}
			profile = created
			return u.appendHistory(ctx, profile.ID, nil,
				codemeta.SellerTaxReviewStatusDraft.Value, taxReasonCreated, "",
				taxActorSeller, actorAccountIDPtr(actorAccountID), now)
		default:
			return err
		}
	})
	return issuer, profile, err
}

// SubmitTaxProfileInput 是端點 19 的輸入。
type SubmitTaxProfileInput struct {
	StoreID         int64
	ExpectedVersion int64
	// AcceptedTerms 是賣家在送審表單上勾選的「已閱讀並同意自行開票責任條款」。
	//
	// 🔴 false ⇒ 9325 且**一筆都不寫**（連 T01 的 ResetToDraft 都不做）。
	// true ⇒ 條款簽署記錄與 T02 的狀態、兩個時鐘欄在**同一句 UPDATE** 落庫。
	AcceptedTerms bool
	// AcceptedByAccountID 是按下「同意條款」的自然人（Gateway 注入的登入 principal）。
	//
	// 🔴 裁決 C 之後端點 19 是絕大多數賣家**唯一**的簽署動作，而這份
	// 「審核期間發票由賣家自行開立、責任在賣家」的憑據必須答得出是誰簽的
	// （§6.1.7 對條款的逐字要求）。0 ⇒ 記成 NULL（查不到人），不擋流程。
	AcceptedByAccountID int64
}

// SubmitTaxProfileResult 是端點 19 的輸出。
//
// CooldownBlocked=true ⇒ 回應是 **9324 ＋ 當前單筆**（HTTP 200，preserveData）：
// 前端要靠 data.reapplyCooldownUntil 顯示「還要等幾天」。少了 data，賣家只看到一句
// 紅字而不知道什麼時候可以再試。
type SubmitTaxProfileResult struct {
	View            SellerTaxProfileView
	CooldownBlocked bool
}

// Submit 送出稅務資格審核（端點 19 ＝ T02，必要時先走 T01 重申）。
//
// 兩條入邊：
//   - DRAFT ⇒ 直接 T02
//   - REJECTED／EXPIRED ⇒ 先 T01 把它推回 DRAFT（7 天冷卻寫在 SQL 的 WHERE），
//     再 T02。冷卻未到 ⇒ 9324 ＋ data，**不是** 9321：兩者的下一步不同
//     （一個要等到某個時刻、一個要先改狀態）
//
// 🔴 **不變式：送審成功 ⇒ 自行開票責任條款必已簽。**（裁決 C，2026-08-31）
//
// 條款簽署不是另一支端點的事：§6.1.7／§8 要求「未接受條款不得送審」，而 T04
// （端點 22）的 allowed-from 是 SUBMITTED ⇒ 「送審要先簽、簽要先送審」互為前提，
// DRAFT 的新賣家永遠走不進那個環。裁決 C 的解法是把簽署併進送審這一步：
// accepted_terms=true 時，attestation_version／attested_at 與 review_status／
// submitted_at／pending_valid_until 在**同一句 UPDATE** 落庫（SQL 註解有全文）。
//
// ⚠️ 9325 的判定刻意放在 `WithTx` **之外**：REJECTED／EXPIRED 的入邊會先做 T01
// ResetToDraft，那是真正的狀態改變。判定放進交易內雖然也會 rollback，但「未同意
// 條款的請求連 rollback 都不需要」是更強也更好驗的性質——陰性測試可以直接斷言
// repo 零呼叫，而不是斷言「寫了又還原」。
func (u *TaxProfile) Submit(ctx context.Context, in SubmitTaxProfileInput) (SubmitTaxProfileResult, error) {
	if !in.AcceptedTerms {
		return SubmitTaxProfileResult{}, taxBiz(bizcode.SellerSelfIssueTermsNotAccepted)
	}
	var out SubmitTaxProfileResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, profile, err := u.loadForSeller(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if profile.Version != in.ExpectedVersion {
			return taxBiz(bizcode.SellerTaxProfileVersionConflict)
		}
		now := u.now()

		if profile.ReviewStatus == codemeta.SellerTaxReviewStatusRejected.Value ||
			profile.ReviewStatus == codemeta.SellerTaxReviewStatusExpired.Value {
			ok, err := u.profiles.ResetToDraft(ctx, profile.ID, now)
			if err != nil {
				return err
			}
			if !ok {
				out = SubmitTaxProfileResult{
					View:            SellerTaxProfileView{StoreID: in.StoreID, Issuer: issuer, Profile: profile},
					CooldownBlocked: true,
				}
				return nil
			}
			if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus,
				codemeta.SellerTaxReviewStatusDraft.Value, taxReasonReapply, "",
				taxActorSeller, actorAccountIDPtr(in.AcceptedByAccountID), now); err != nil {
				return err
			}
			if profile, err = u.profiles.GetByIssuer(ctx, issuer.ID); err != nil {
				return err
			}
		}

		if profile.ReviewStatus != codemeta.SellerTaxReviewStatusDraft.Value {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		if err := validateTaxIdentity(profile); err != nil {
			return err
		}
		if len(profile.EvidenceKeys()) == 0 {
			return ErrTaxEvidenceRequired
		}

		ok, err := u.profiles.Submit(ctx, profile.ID, profile.Version,
			SelfIssueTermsVersion, in.AcceptedByAccountID, now.Add(taxSubmitWindow), now)
		if err != nil {
			return err
		}
		if !ok {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus,
			codemeta.SellerTaxReviewStatusSubmitted.Value, taxReasonSubmitted, SelfIssueTermsVersion,
			taxActorSeller, actorAccountIDPtr(in.AcceptedByAccountID), now); err != nil {
			return err
		}
		updated, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		out = SubmitTaxProfileResult{View: SellerTaxProfileView{StoreID: in.StoreID, Issuer: issuer, Profile: updated}}
		return nil
	})
	if err != nil {
		return SubmitTaxProfileResult{}, err
	}
	return out, nil
}

// RequestTaxExtensionInput 是端點 20 的輸入。
type RequestTaxExtensionInput struct {
	StoreID         int64
	ExpectedVersion int64
	Reason          string
	// RequestedByAccountID 是登入 principal（§4 例外①；backend#3048）：延長會把平台
	// 補件期限往後推最多 30 天且只能一次，「是誰推的」必須留痕。
	RequestedByAccountID int64
}

// RequestExtension 是賣家申請延長待審期限（端點 20）。
//
// 🔴 **它不改任何狀態、也不動 pending_valid_until**：真正的延長是 ADM 的 T11
// （端點 41），賣家這一支只是「提出請求」。把它做成直接延長，等於把 T11 的守衛
// （ADM 明示 ＋ 必填理由 ＋ 只能延一次）交給申請人自己執行。
//
// ⚠️ 因此它也**沒有 CAS**：本服務的 §5 沒有 tax_extension_requests 表，唯一的落點是
// invoice_status_history 的一列自轉移（from_status = to_status）。expectedVersion 只在
// 本層比對以回 9322——這是全檔唯一一處「守衛不在 SQL」，而它成立的理由是這支不寫
// seller_tax_profiles 的任何一欄，版本過期不會造成任何不一致。缺口登記在交付回報。
func (u *TaxProfile) RequestExtension(ctx context.Context, in RequestTaxExtensionInput) (SellerTaxProfileView, error) {
	if err := requireReason(in.Reason); err != nil {
		return SellerTaxProfileView{}, err
	}
	var out SellerTaxProfileView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, profile, err := u.loadForSeller(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if profile.Version != in.ExpectedVersion {
			return taxBiz(bizcode.SellerTaxProfileVersionConflict)
		}
		if profile.ReviewStatus != codemeta.SellerTaxReviewStatusSelfIssuePending.Value {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		if profile.ExtensionCount != 0 {
			return taxBiz(bizcode.SellerTaxExtensionAlreadyUsed)
		}
		now := u.now()
		if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus,
			profile.ReviewStatus, taxReasonExtensionRequested, in.Reason,
			taxActorSeller, actorAccountIDPtr(in.RequestedByAccountID), now); err != nil {
			return err
		}
		out = SellerTaxProfileView{StoreID: in.StoreID, Issuer: issuer, Profile: profile}
		return nil
	})
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	return out, nil
}

// AcceptAttestationInput 是端點 22 的輸入。
type AcceptAttestationInput struct {
	StoreID             int64
	AcceptedByAccountID int64
	TermsVersion        string
	Accepted            bool
}

// AcceptAttestation 接受自行開票責任條款（端點 22 ＝ T04 SUBMITTED → SELF_ISSUE_PENDING）。
//
// ⚠️ **不重算 pending_valid_until**（決策 C4 的單一時鐘）：重算等於讓賣家用「停在未接受
// 條款」把 60 天窗口無限延長。那一欄由 SQL 那一句刻意不碰。
//
// ⚠️ request **沒有 expectedVersion**（proto 逐欄查過）⇒ 版本在交易內現讀、現 CAS。
// 這不是「不做樂觀鎖」：CAS 仍然帶 version，併發的另一次寫入會讓 rows=0 而回 9321。
// 差別只是拿不到「是版本衝突還是狀態不對」的區分。
func (u *TaxProfile) AcceptAttestation(ctx context.Context, in AcceptAttestationInput) (SellerTaxProfileView, error) {
	if !in.Accepted {
		return SellerTaxProfileView{}, ErrTaxTermsNotAccepted
	}
	v := strings.TrimSpace(in.TermsVersion)
	if v == "" || utf8.RuneCountInString(v) > taxAttestationVersionMax {
		return SellerTaxProfileView{}, ErrTaxTermsVersionInvalid
	}
	if v != SelfIssueTermsVersion {
		return SellerTaxProfileView{}, ErrTaxTermsVersionStale
	}
	var out SellerTaxProfileView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, profile, err := u.loadForSeller(ctx, in.StoreID)
		if err != nil {
			return err
		}
		now := u.now()
		ok, err := u.profiles.Attest(ctx, profile.ID, profile.Version, v, in.AcceptedByAccountID, now)
		if err != nil {
			return err
		}
		if !ok {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus,
			codemeta.SellerTaxReviewStatusSelfIssuePending.Value, taxReasonAttested, v,
			taxActorSeller, actorAccountIDPtr(in.AcceptedByAccountID), now); err != nil {
			return err
		}
		updated, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		out = SellerTaxProfileView{StoreID: in.StoreID, Issuer: issuer, Profile: updated}
		return nil
	})
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	return out, nil
}

// TaxDecisionInput 是端點 40 的輸入。
type TaxDecisionInput struct {
	ActorAccountID     int64
	StoreID            int64
	Decision           string
	Reason             string
	ApprovalValidUntil *time.Time
	Confirmed          bool
	ExpectedVersion    int64
	// 收據代製授權的核定依據文件（§6.1.9，只有 APPROVE_INVOICE_EXEMPT 用得到）。
	//
	// 🔴 兩欄同進同出，且必須是**該賣家自己在 S02 上傳的**稅籍文件之一
	// （media 的授權述詞等價於「只有上傳者本人」⇒ ADM 自己上傳的那份，賣家點下載一律 404）。
	// 本層驗「在不在 evidence_object_keys 裡」；DB 的 chk_stp_receipt_basis_pair 守成對。
	ApprovalBasisKey     string
	ApprovalBasisAssetID string
}

// TaxDecisionResult 是端點 40／41 的共通輸出（proto 共用 DecideTaxReviewForAdminData）。
type TaxDecisionResult struct {
	StoreID            int64
	ReviewStatus       string
	Version            int64
	AuditLogID         int64
	RevokedEffectiveAt *time.Time
}

var taxDecisionAllowedFrom = map[string][]string{
	codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value: {
		codemeta.SellerTaxReviewStatusUnderReview.Value,
		codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
	},
	codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value: {
		codemeta.SellerTaxReviewStatusUnderReview.Value,
		codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
	},
	codemeta.SellerTaxReviewDecisionReject.Value: {
		codemeta.SellerTaxReviewStatusUnderReview.Value,
		codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
	},
	codemeta.SellerTaxReviewDecisionRequestSupplement.Value: {
		codemeta.SellerTaxReviewStatusUnderReview.Value,
	},
	codemeta.SellerTaxReviewDecisionKeepUnderReview.Value: {
		codemeta.SellerTaxReviewStatusUnderReview.Value,
	},
	codemeta.SellerTaxReviewDecisionRevoke.Value: {
		codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
	},
	codemeta.SellerTaxReviewDecisionReopenReview.Value: {
		codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
		codemeta.SellerTaxReviewStatusRevoked.Value,
		codemeta.SellerTaxReviewStatusExpired.Value,
	},
}

var taxDecisionNeedsConfirm = map[string]bool{
	codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value: true,
	codemeta.SellerTaxReviewDecisionRevoke.Value:               true,
}

// Decide 執行 ADM 的稅務審核裁決（端點 40）。
//
// 一次動作寫四樣東西，同一個交易：狀態 CAS ＋ invoice_status_history ＋
// invoice_admin_audit_logs ＋（狀態真的變了才有的）outbox。
// 拆開任何一樣的後果都是「事後查不出來」，而 ADM 稽核是 5 年保存義務。
func (u *TaxProfile) Decide(ctx context.Context, in TaxDecisionInput) (TaxDecisionResult, error) {
	allowed, known := taxDecisionAllowedFrom[in.Decision]
	if !known {
		return TaxDecisionResult{}, ErrTaxDecisionUnknown
	}
	if err := requireReason(in.Reason); err != nil {
		return TaxDecisionResult{}, err
	}
	if taxDecisionNeedsConfirm[in.Decision] && !in.Confirmed {
		return TaxDecisionResult{}, ErrTaxConfirmRequired
	}

	var out TaxDecisionResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, profile, err := u.loadForSeller(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if profile.Version != in.ExpectedVersion {
			return taxBiz(bizcode.SellerTaxProfileVersionConflict)
		}
		if !containsString(allowed, profile.ReviewStatus) {
			return taxBiz(bizcode.InvoiceTaxDecisionInvalid)
		}
		now := u.now()

		nextStatus, revokedAt, err := u.applyDecision(ctx, in, profile, now)
		if err != nil {
			return err
		}

		if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus, nextStatus,
			taxReasonAdminDecision, in.Decision+": "+in.Reason,
			taxActorAdmin, &in.ActorAccountID, now); err != nil {
			return err
		}

		before, after := taxAuditStates(profile.ReviewStatus, nextStatus)
		auditID, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: in.ActorAccountID,
			Action:         taxAuditActionDecision,
			ResourceType:   taxResourceType,
			ResourceID:     profile.ID,
			BeforeState:    before,
			AfterState:     after,
			ReasonCode:     in.Decision,
			ReasonDetail:   in.Reason,
			Now:            now,
		})
		if err != nil {
			return err
		}

		updated, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		if nextStatus != profile.ReviewStatus {
			if err := u.emitStatusChanged(ctx, issuer, profile, nextStatus, now); err != nil {
				return err
			}
		}
		out = TaxDecisionResult{
			StoreID: in.StoreID, ReviewStatus: updated.ReviewStatus,
			Version: updated.Version, AuditLogID: auditID, RevokedEffectiveAt: revokedAt,
		}
		return nil
	})
	if err != nil {
		return TaxDecisionResult{}, err
	}
	return out, nil
}

func (u *TaxProfile) applyDecision(
	ctx context.Context, in TaxDecisionInput, profile domain.SellerTaxProfile, now time.Time,
) (string, *time.Time, error) {
	switch in.Decision {
	case codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
		codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value:
		next := codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value
		if in.Decision == codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value {
			next = codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value
		}
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value &&
			(derefString(profile.TaxID) == "" || derefString(profile.LegalName) == "") {
			return "", nil, taxBiz(bizcode.InvoiceTaxApprovalIdentityIncomplete)
		}
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value &&
			derefString(profile.TaxID) != "" {
			return "", nil, taxBiz(bizcode.InvoiceTaxDecisionInvalid)
		}
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value &&
			profile.ReceiptAuthorizedAt == nil {
			return "", nil, taxBiz(bizcode.InvoiceTaxApprovalReceiptAuthMissing)
		}
		validUntil := now.Add(taxApprovalValidity)
		if in.ApprovalValidUntil != nil {
			if !in.ApprovalValidUntil.After(now) {
				return "", nil, ErrTaxValidUntilInvalid
			}
			validUntil = *in.ApprovalValidUntil
		}
		basisKey, basisAssetID, err := resolveReceiptApprovalBasis(in, profile, next)
		if err != nil {
			return "", nil, err
		}
		ok, err := u.profiles.Approve(ctx, domain.ApproveSellerTaxProfile{
			ID:                   profile.ID,
			ExpectedVersion:      profile.Version,
			NextStatus:           next,
			ApprovalValidUntil:   validUntil,
			ApprovalBasisKey:     basisKey,
			ApprovalBasisAssetID: basisAssetID,
			Now:                  now,
		})
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionRequestSupplement.Value:
		next := codemeta.SellerTaxReviewStatusSupplementRequired.Value
		ok, err := u.profiles.RequestSupplement(ctx, profile.ID, profile.Version, in.Reason, now)
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionKeepUnderReview.Value:
		next := codemeta.SellerTaxReviewStatusUnderReview.Value
		ok, err := u.profiles.KeepUnderReview(ctx, profile.ID, profile.Version, in.Reason, now)
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionReject.Value:
		next := codemeta.SellerTaxReviewStatusRejected.Value
		ok, err := u.profiles.Reject(ctx, profile.ID, profile.Version, in.Reason, now.Add(taxReapplyCooldown), now)
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionRevoke.Value:
		next := codemeta.SellerTaxReviewStatusRevoked.Value
		ok, err := u.profiles.Revoke(ctx, profile.ID, profile.Version, in.Reason, now)
		if e := taxCASResult(ok, err, next); e != nil {
			return "", nil, e
		}
		effective := now
		return next, &effective, nil

	case codemeta.SellerTaxReviewDecisionReopenReview.Value:
		next := codemeta.SellerTaxReviewStatusUnderReview.Value
		ok, err := u.profiles.ReopenReview(ctx, profile.ID, profile.Version, in.Reason, now)
		return next, nil, taxCASResult(ok, err, next)
	}
	return "", nil, ErrTaxDecisionUnknown
}

// TaxExtensionDecisionInput 是端點 41 的輸入。
type TaxExtensionDecisionInput struct {
	ActorAccountID  int64
	StoreID         int64
	Decision        string // APPROVE / REJECT（**不是** SellerTaxReviewDecision 的值域）
	ExtendDays      int
	Reason          string
	ExpectedVersion int64
}

// DecideExtension 執行 ADM 的延長裁決（端點 41 ＝ T11）。
//
// 🔴 APPROVE 走 T11：**狀態不變**，就地改寫 pending_valid_until ＋ extension_count = 1。
// 刻意不設 EXTENDED 狀態——設了的話 idx_stp_pending_expiry 與到期掃描的述詞都得多一個
// 值，漏寫任一個 ⇒ 被延長過的賣家永不逾期（I-34）。
//
// 🔴 REJECT **不寫 decisionHistory 但仍要寫 invoice_admin_audit_logs**（§6.1.12 逐字）：
// 「駁回了延長申請」本身就是一次需要留痕的 ADM 行為。
func (u *TaxProfile) DecideExtension(ctx context.Context, in TaxExtensionDecisionInput) (TaxDecisionResult, error) {
	if in.Decision != taxExtensionApprove && in.Decision != taxExtensionReject {
		return TaxDecisionResult{}, ErrTaxDecisionUnknown
	}
	if err := requireReason(in.Reason); err != nil {
		return TaxDecisionResult{}, err
	}
	days := in.ExtendDays
	if days == 0 {
		days = taxExtensionDefaultDays
	}
	if in.Decision == taxExtensionApprove && (days < 1 || days > taxExtensionMaxDays) {
		return TaxDecisionResult{}, ErrTaxExtendDaysInvalid
	}

	var out TaxDecisionResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		issuer, profile, err := u.loadForSeller(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if profile.Version != in.ExpectedVersion {
			return taxBiz(bizcode.SellerTaxProfileVersionConflict)
		}
		now := u.now()

		if in.Decision == taxExtensionApprove {
			if profile.ReviewStatus != codemeta.SellerTaxReviewStatusSelfIssuePending.Value {
				return taxBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			if profile.ExtensionCount != 0 {
				return taxBiz(bizcode.SellerTaxExtensionAlreadyUsed)
			}
			if profile.PendingValidUntil == nil {
				return taxBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			newDeadline := profile.PendingValidUntil.AddDate(0, 0, days)
			ok, err := u.profiles.ExtendPending(ctx, profile.ID, profile.Version, newDeadline, in.Reason, now)
			if err != nil {
				return err
			}
			if !ok {
				return taxBiz(bizcode.SellerTaxExtensionAlreadyUsed)
			}
			if err := u.appendHistory(ctx, profile.ID, &profile.ReviewStatus, profile.ReviewStatus,
				taxReasonExtensionApproved,
				"EXTEND(+"+strconv.Itoa(days)+"d): "+in.Reason,
				taxActorAdmin, &in.ActorAccountID, now); err != nil {
				return err
			}
		}

		before, after := taxExtensionAuditStates(profile, in.Decision, days)
		auditID, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: in.ActorAccountID,
			Action:         taxAuditActionExtension,
			ResourceType:   taxResourceType,
			ResourceID:     profile.ID,
			BeforeState:    before,
			AfterState:     after,
			ReasonCode:     taxExtensionReasonCode(in.Decision),
			ReasonDetail:   in.Reason,
			Now:            now,
		})
		if err != nil {
			return err
		}
		updated, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return err
		}
		out = TaxDecisionResult{
			StoreID: in.StoreID, ReviewStatus: updated.ReviewStatus,
			Version: updated.Version, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return TaxDecisionResult{}, err
	}
	return out, nil
}

func (u *TaxProfile) loadForSeller(ctx context.Context, storeID int64) (domain.Issuer, domain.SellerTaxProfile, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
			return domain.Issuer{}, domain.SellerTaxProfile{}, taxBiz(bizcode.SellerTaxProfileNotFound)
		}
		return domain.Issuer{}, domain.SellerTaxProfile{}, err
	}
	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil {
		return domain.Issuer{}, domain.SellerTaxProfile{}, err
	}
	return issuer, profile, nil
}

func actorAccountIDPtr(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

func (u *TaxProfile) appendHistory(
	ctx context.Context, subjectID int64, from *string, to, reasonCode, reasonDetail, actorType string,
	actorAccountID *int64, now time.Time,
) error {
	var fromCopy *string
	if from != nil {
		v := *from
		fromCopy = &v
	}
	var detail *string
	if reasonDetail != "" {
		detail = &reasonDetail
	}
	code := reasonCode
	return u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    taxSubjectType,
		SubjectID:      subjectID,
		FromStatus:     fromCopy,
		ToStatus:       to,
		ReasonCode:     &code,
		ReasonDetail:   detail,
		ActorType:      actorType,
		ActorAccountID: actorAccountID,
		Now:            now,
	})
}

type taxStatusChangedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	TaxProfileID      string            `json:"tax_profile_id"`
	FromStatus        string            `json:"from_status"`
	ToStatus          string            `json:"to_status"`
	OccurredAt        string            `json:"occurred_at"`
}

func (u *TaxProfile) emitStatusChanged(
	ctx context.Context, issuer domain.Issuer, profile domain.SellerTaxProfile, next string, now time.Time,
) error {
	payload, err := json.Marshal(taxStatusChangedEvent{
		NotificationRefID: "INVSTP:" + strconv.FormatInt(profile.ID, 10) + ":" + next,
		StoreID:           eventwire.StoreIDOf(issuer),
		TaxProfileID:      strconv.FormatInt(profile.ID, 10),
		FromStatus:        profile.ReviewStatus,
		ToStatus:          next,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateTaxProfile,
		AggregateID:   profile.ID,
		EventType:     string(pkgmq.EventTypeInvoiceSellerTaxProfileStatusChange),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceSellerTaxProfileStatusChange,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	})
	return err
}

func (u *TaxProfile) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

func taxCASResult(ok bool, err error, _ string) error {
	if err != nil {
		return err
	}
	if !ok {
		return taxBiz(bizcode.InvoiceTaxDecisionInvalid)
	}
	return nil
}

func taxBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func requireReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrTaxReasonRequired
	}
	if utf8.RuneCountInString(reason) > taxReasonMaxRunes {
		return ErrTaxReasonTooLong
	}
	return nil
}

func validateTaxIdentity(p domain.SellerTaxProfile) error {
	if derefString(p.EntityTaxType) == "" ||
		derefString(p.LegalName) == "" ||
		derefString(p.ResponsiblePerson) == "" ||
		derefString(p.BusinessAddress) == "" {
		return ErrTaxIdentityIncomplete
	}
	if p.ClaimedObligation == codemeta.SellerTaxObligationRequired.Value && derefString(p.TaxID) == "" {
		return ErrTaxIdentityIncomplete
	}
	if p.ClaimedObligation == codemeta.SellerTaxObligationUnknown.Value {
		return ErrTaxIdentityIncomplete
	}
	return nil
}

func normalizeObligation(v string) string {
	if _, ok := codemeta.SellerTaxObligationFromValue(v); ok {
		return v
	}
	return codemeta.SellerTaxObligationUnknown.Value
}

func joinAddress(city, district, detail string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{city, district, detail} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	joined := strings.Join(parts, "")
	if len([]rune(joined)) > 255 {
		joined = string([]rune(joined)[:255])
	}
	return joined
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func isSubset(sub, super []string) bool {
	set := make(map[string]struct{}, len(super))
	for _, v := range super {
		set[v] = struct{}{}
	}
	for _, v := range sub {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}

func taxAuditStates(from, to string) (json.RawMessage, json.RawMessage) {
	before, _ := json.Marshal(map[string]string{"reviewStatus": from})
	after, _ := json.Marshal(map[string]string{"reviewStatus": to})
	return before, after
}

func taxExtensionAuditStates(p domain.SellerTaxProfile, decision string, days int) (json.RawMessage, json.RawMessage) {
	beforeMap := map[string]string{
		"reviewStatus":   p.ReviewStatus,
		"extensionCount": strconv.Itoa(int(p.ExtensionCount)),
	}
	if p.PendingValidUntil != nil {
		beforeMap["deadlineAt"] = timeutil.FormatISO8601(*p.PendingValidUntil)
	}
	afterMap := map[string]string{"decision": decision}
	if decision == taxExtensionApprove && p.PendingValidUntil != nil {
		afterMap["extensionCount"] = "1"
		afterMap["deadlineAt"] = timeutil.FormatISO8601(p.PendingValidUntil.AddDate(0, 0, days))
	}
	before, _ := json.Marshal(beforeMap)
	after, _ := json.Marshal(afterMap)
	return before, after
}

func taxExtensionReasonCode(decision string) string {
	if decision == taxExtensionApprove {
		return codemeta.SellerTaxReviewDecisionExtend.Value
	}
	return taxExtensionReject
}

func resolveReceiptApprovalBasis(
	in TaxDecisionInput, profile domain.SellerTaxProfile, nextStatus string,
) (*string, *int64, error) {
	key, assetID := strings.TrimSpace(in.ApprovalBasisKey), strings.TrimSpace(in.ApprovalBasisAssetID)
	if key == "" && assetID == "" {
		return nil, nil, nil
	}
	if key == "" || assetID == "" {
		return nil, nil, ErrTaxApprovalBasisIncomplete
	}
	if nextStatus != codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value {
		return nil, nil, ErrTaxApprovalBasisNotExempt
	}
	if !containsEvidenceKey(profile.EvidenceObjectKeys, key) {
		return nil, nil, ErrTaxApprovalBasisNotOwned
	}
	id, err := strconv.ParseInt(assetID, 10, 64)
	if err != nil || id <= 0 {
		return nil, nil, ErrTaxApprovalBasisAssetInvalid
	}
	return &key, &id, nil
}
