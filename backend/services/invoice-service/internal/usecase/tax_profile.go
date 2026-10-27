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

// 本檔是 M3 稅務資格狀態機（§7.3.3 T01–T16）的**寫入面**：賣家側 §6.1.7 的端點 18／19／
// 20／22，ADM 側 §6.1.12 的端點 40／41。讀取面（端點 17／21／39）在 query.go。
//
// 五條貫穿全檔的約束：
//
//	① **allowed-from 只有一份，住在 SQL 的 WHERE**（db/queries/seller_tax_profiles.sql）。
//	   本層先讀一次是為了回**可分辨的碼**（9321／9322／9323／9324／9348），不是正確性
//	   防線——兩個分頁同時送出時應用層的 if 會各自讀到舊值。真正的守衛是那句 CAS，
//	   而 rows=0 一律回明確業務碼，**絕不當成功往下走**。
//	② **一次動作 ＝ 一個交易**：狀態轉移 ＋ 狀態歷程 ＋（ADM 的）稽核列 ＋ outbox 必須
//	   同進同退。少了交易會出現「狀態轉了但沒有人知道是誰轉的」，而那正是事後查不出來
//	   的來源；ADM 側更嚴重——`invoice_admin_audit_logs` 是 5 年保存義務（§9.5）。
//	③ **Tier A**：賣家側的歸屬鍵是 `store_id`，而它由 Gateway 的 sellerid resolver 注入、
//	   前端不可控。解析路徑是 `invoice_issuers.issuer_ref_id = store_id AND
//	   issuer_type='SELLER'`（GetByRef）⇒ 別的 store_id 解出別的 issuer，拿不到這一列。
//	   **越權與查無共用 9320、同一段 message、同一行程式碼**。
//	④ **60 天是單一時鐘**：`submitted_at` 是唯一起算點（決策 C4），`pending_valid_until`
//	   ＝ 它 + 60 天。補件（T05）不重算、接受條款（T04）不重算——重算等於讓賣家用
//	   「停在某個狀態」把窗口無限延長。
//	⑤ **媒體換發不在交易內**：FinalizeImages／LinkImages 是網路呼叫，包進交易等於讓一把
//	   資料庫連線被外部服務的延遲綁住。順序契約（§6.1.7）因此展開成
//	   「建列（交易）→ 換發（網路，致命）→ 更新（交易）→ 回收（網路，best-effort）」。

const (
	// taxSubmitWindow 是 T02 寫入 pending_valid_until 的長度（決策 C4 的 60 天）。
	taxSubmitWindow = 60 * 24 * time.Hour
	// taxReapplyCooldown 是 T09／T10 之後的重申冷卻（決策 C3 的 7 天）。
	taxReapplyCooldown = 7 * 24 * time.Hour
	// taxApprovalValidity 是 approval_valid_until 的預設長度（T07／T08 的 365 天）。
	//
	// ⚠️ 它是「該複審了」的到期日，**不是資格失效日**：§7.3.3 沒有任何一條邊會因為它
	// 屆滿而改狀態。自動改狀態會讓賣家在毫無操作下，於核准滿一年那天起全部新訂單靜默
	// 改判 LEGACY_MANUAL，而發票人是賣家、罰的也是賣家。
	taxApprovalValidity = 365 * 24 * time.Hour
	// taxExtensionDefaultDays 是端點 41 未指定 extendDays 時的天數（§6.1.12 逐字的 "30"）。
	taxExtensionDefaultDays = 30
	// taxExtensionMaxDays 是 extendDays 的上界。60 天窗口只能延一次，延超過一年等於
	// 讓「待審」變成一個沒有期限的狀態。
	taxExtensionMaxDays = 365

	// taxEvidenceMax 是 evidence_object_keys 的筆數上限（§6.1.7：保留清單 ∪ 新 key ≤ 10）。
	taxEvidenceMax = 10
	// taxReasonMaxRunes 是理由類自由文字的長度上限，與 B07 同值。
	//
	// 在這裡擋而不是讓 DB 擋：review_note／reason_detail 都是 text 欄，DB 根本不會擋，
	// 少了本檢查，前端的 maxLength 是唯一防線，而那是 client-side 的。
	taxReasonMaxRunes = 500
	// taxAttestationVersionMax 對齊 seller_tax_profiles.attestation_version 的 varchar(32)。
	// 超長在 DB 是 22001（字串截斷）而不是可讀的 400。
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
	// taxLegalNameMax 對齊 legal_name varchar(120)／invoice_issuers.display_name varchar(120)。
	taxLegalNameMax = 120
	// 以下三個逐欄對齊 seller_tax_profiles 的 DDL（#3089 稽核 A-4）：
	// entity_tax_type varchar(32)／responsible_person varchar(60)／
	// address_city varchar(32)／address_district varchar(32)。
	// 超長的症狀是 22001 ⇒ 整個交易 aborted ⇒ HTTP 500。
	taxEntityTypeMax      = 32
	taxResponsibleNameMax = 60
	taxAddressPartMax     = 32

	// taxIssuerPlaceholderPrefix 是 issuer 建列當下的 display_name 佔位字串。
	//
	// display_name ＝ 法定名稱（§5.1.1），但 issuer 必須先存在（seller_tax_profiles 的 FK）
	// 而法定名稱要等賣家填。⇒ 先放佔位、法定名稱一出現就由 UpdateDisplayName 覆蓋。
	// 佔位字串刻意帶 store_id：留在 DB 裡時看得出是哪一家，而不是一串「未命名」。
	taxIssuerPlaceholderPrefix = "store-"
)

// 稽核與歷程的固定字串。
const (
	taxSubjectType  = "TAX_PROFILE" // invoice_status_history.subject_type（chk_invhist_subject 七值之一）
	taxResourceType = "SELLER_TAX_PROFILE"

	taxActorSeller = "SELLER"
	taxActorAdmin  = "ADMIN"

	// invoice_status_history.reason_code 的稅務軸取值。它們不是 CodeMeta 碼值——
	// 該欄是自由的 varchar(48)，而 CodeMeta 沒有「歷程原因」這張碼表。
	taxReasonCreated            = "SELLER_TAX_DRAFT_CREATED"
	taxReasonReapply            = "SELLER_TAX_REAPPLY"
	taxReasonSubmitted          = "SELLER_TAX_SUBMITTED"
	taxReasonAttested           = "SELLER_TAX_SELF_ISSUE_ATTESTED"
	taxReasonExtensionRequested = "SELLER_TAX_EXTENSION_REQUESTED"
	taxReasonExtensionApproved  = "ADMIN_TAX_EXTENSION_APPROVED"
	taxReasonAdminDecision      = "ADMIN_TAX_DECISION"

	// invoice_admin_audit_logs.action。前綴與既有的 INVOICE_* 一致。
	taxAuditActionDecision  = "INVOICE_TAX_REVIEW_DECIDE"
	taxAuditActionExtension = "INVOICE_TAX_EXTENSION_DECIDE"

	// outbox aggregate_type 的稅務軸（schema 欄位註解列的四值之一）。
	outboxAggregateTaxProfile = "TAX_PROFILE"
)

// 端點 41 的兩個 decision 值。它們**不是** SellerTaxReviewDecision 的成員：
// §6.1.12 逐字「端點 41 自己的 Decision 欄（APPROVE／REJECT）是另一組值域」。
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

// ---------------------------------------------------------------------------
// 賣家：端點 18 草稿存檔
// ---------------------------------------------------------------------------

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
	// #3089 B 組第 4 項：擋在應用層，不要讓 chk_stp_tax_id 的 23514 冒泡成 500。
	// 空字串是合法輸入（還沒填／清掉重填），非空就必須是 8 位數字。
	if in.TaxID != "" && !domain.IsBusinessTaxID(in.TaxID) {
		return SellerTaxProfileView{}, ErrTaxIDInvalid
	}
	// 🔴 同一個缺陷類的另一半：**超長字串是 22001 不是 23514**，但症狀一模一樣
	// （SQLSTATE 冒泡成 HTTP 500，賣家看到「系統錯誤」）。
	// 在此之前這四欄一個都沒守——只有 legal_name(120) 被守著。
	// 長度逐欄取自 DDL，不是抓一個整數：varchar 的界線就是那些數字。
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
	// entity_tax_type 對應 CodeMeta SellerTaxEntityType ⇒ 寫入 DB 前必須過共用
	// validator（CLAUDE.md §8）。在此之前它**完全沒被驗過**，任何字串都寫得進去
	// ——那不是 500，是更難發現的髒資料：ADM 審件時看到一個不在值域裡的型態。
	// 空字串合法（草稿階段還沒選）。
	if in.EntityType != "" {
		if _, ok := codemeta.SellerTaxEntityTypeFromValue(in.EntityType); !ok {
			return SellerTaxProfileView{}, ErrTaxEntityTypeInvalid
		}
	}
	// 先擋筆數再換發：換發是有副作用的（資產轉正 ＋ 建引用），為了一個一定會被拒的
	// 請求去製造 11 份正式資產，是把清理成本留給別人。
	if len(in.EvidenceObjectKeys)+len(in.EvidenceStagedTokens) > taxEvidenceMax {
		return SellerTaxProfileView{}, ErrTaxEvidenceTooMany
	}

	// ① 建列。issuer 與 profile 各自 ON CONFLICT DO NOTHING ⇒ 併發的兩個分頁只會有
	//    一個真的插入，另一個讀既有列，兩者都不報錯。
	issuer, profile, err := u.ensureProfile(ctx, in.StoreID, in.LegalName, in.SavedByAccountID)
	if err != nil {
		return SellerTaxProfileView{}, err
	}

	// 保留清單必須是這位賣家**當前**的 key 的子集。
	//
	// 🔴 少了本檢查就是 IDOR：賣家可以把別人的 objectKey 塞進自己的佐證列，而那些 key
	// 會被 ADM 在複審時下載（§6.1.7 逐字）。比對在換發之前做——這樣越權請求連一份資產
	// 都不會被轉正。
	current := profile.EvidenceKeys()
	if !isSubset(in.EvidenceObjectKeys, current) {
		return SellerTaxProfileView{}, ErrTaxEvidenceNotOwned
	}

	// ② 換發。致命：失敗即中止，DB 一個字都不寫。
	newKeys, assetIDs, err := u.evidence.FinalizeEvidence(ctx, in.EvidenceStagedTokens, profile.ID)
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	merged := dedupeStrings(append(append([]string{}, in.EvidenceObjectKeys...), newKeys...))
	if len(merged) > taxEvidenceMax {
		return SellerTaxProfileView{}, ErrTaxEvidenceTooMany
	}

	// ③ 更新。
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

		// T01 第二形態：REJECTED／EXPIRED 的賣家要編輯，先把它推回 DRAFT。
		// 冷卻守衛住在 SQL 的 WHERE（決策 C3）——寫在這裡的 if 會在併發重送時
		// 同時通過兩次。rows=0 ⇒ 冷卻未到，回 9321（端點 18 的碼表只有 9321／9322；
		// 冷卻的到期時刻本來就在 S02 讀得到的 reapplyCooldownUntil 上）。
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
			// 重讀而不是就地 +1：version 的新值由 SQL 算出（version + 1），
			// Go 端拼一份等於維護第二套算式。
			if fresh, err = u.profiles.GetByIssuer(ctx, issuer.ID); err != nil {
				return err
			}
		}

		if fresh.ReviewStatus != codemeta.SellerTaxReviewStatusDraft.Value {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}

		ok, err := u.profiles.UpdateDraft(ctx, domain.UpdateSellerTaxDraft{
			ID:                fresh.ID,
			ExpectedVersion:   fresh.Version,
			ClaimedObligation: normalizeObligation(in.ClaimedStatus),
			EntityTaxType:     in.EntityType,
			LegalName:         in.LegalName,
			TaxID:             in.TaxID,
			ResponsiblePerson: in.ResponsibleName,
			BusinessAddress:   joinAddress(in.AddressCity, in.AddressDistrict, in.AddressDetail),
			// #3031：三段分開存。串接結果留在 business_address（AMEGO 送件與單行顯示），
			// 前兩段另外落庫，讓回程還原得回表單的三格。
			AddressCity:     strings.TrimSpace(in.AddressCity),
			AddressDistrict: strings.TrimSpace(in.AddressDistrict),
			// default_tax_category 不由端點 18 決定：它是**核定**的結果（§7.1.3 的唯一
			// 稅別來源），賣家自述只寫進 claimed_obligation。沿用現值，不讓草稿改稅別。
			DefaultTaxCategory: fresh.DefaultTaxCategory,
			EvidenceObjectKeys: merged,
			// review_note 是 ADM 的欄位（補件說明／駁回理由），賣家的草稿不得覆蓋它。
			ReviewNote: derefString(fresh.ReviewNote),
			Now:        now,
		})
		if err != nil {
			return err
		}
		if !ok {
			// 🔴 rows=0 ＝ 守衛不成立（狀態被搶先改掉／版本被搶先 +1），
			// 不得當成功往下走——那正是靜默失敗的形狀。
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}

		// display_name ＝ 法定名稱。同交易同步，否則 issuer 上會留著佔位字串直到開票。
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

	// ④ 回收。best-effort：失敗的後果是舊資產永遠不被 GC 回收（洩漏），
	// 而洩漏可以事後清、實體刪除不能 ⇒ 不讓它回滾已經成功的業務寫入。
	//
	// ⚠️ 只在本次真的有換發時才呼叫：keep 名單認的是 assetId，而保留清單存的是
	// objectKey（本服務沒有 objectKey → assetId 的反查）⇒ 純刪除的草稿存檔若也呼叫，
	// 會把所有既有佐證的引用一起釋放掉。
	if len(assetIDs) > 0 {
		if err := u.evidence.ReleaseEvidence(ctx, profile.ID, assetIDs); err != nil {
			u.logger().Warn("invoice.tax_profile.release_evidence.failed",
				zap.Int64("tax_profile_id", profile.ID), zap.Error(err))
		}
	}
	return out, nil
}

// ensureProfile 保證這位賣家有 issuer 與 seller_tax_profiles 兩列，並回它們。
//
// 🔴 它是**冪等的**：兩句 INSERT 都是 ON CONFLICT DO NOTHING，rows=0 就改讀既有列。
// 這也是本服務唯一的 invoice_issuers 建列點——在它接上之前，seller_tax_profiles 的 FK
// 永遠找不到對象，整條 S02 一列都建不出來。
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
				// 併發的另一個請求搶先建了 ⇒ 讀它建的那一列。
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
			// 建列也是一次狀態轉移（T01 的 `[*]` → DRAFT）：from_status = NULL。
			// 不記的話，「這位賣家什麼時候開始填的」在事後查不出來。
			return u.appendHistory(ctx, profile.ID, nil,
				codemeta.SellerTaxReviewStatusDraft.Value, taxReasonCreated, "",
				taxActorSeller, actorAccountIDPtr(actorAccountID), now)
		default:
			return err
		}
	})
	return issuer, profile, err
}

// ---------------------------------------------------------------------------
// 賣家：端點 19 送出審核
// ---------------------------------------------------------------------------

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
				// 冷卻未到。回當前單筆讓前端顯示倒數——這是全檔唯一一個
				// 「非零碼 ＋ data」的出口。
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
		// T02 的兩個守衛（§7.3.3）。兩者都回 400 而不是業務碼：賣家的下一步是
		// 「回去把欄位填完／把檔案傳上來」，不是「等審核」。
		if err := validateTaxIdentity(profile); err != nil {
			return err
		}
		if len(profile.EvidenceKeys()) == 0 {
			return ErrTaxEvidenceRequired
		}

		// 單一時鐘：submitted_at ＝ now，pending_valid_until ＝ now + 60 天。
		// 兩欄必須同一句寫（chk_stp_pending_window 對下游兩個狀態要求兩者皆非空）。
		//
		// 條款版本取服務端當期常數，**不是** request 帶進來的值：accepted_terms 只是
		// 一個 bool，「簽的是哪一版」由平台認定（見 SelfIssueTermsVersion 的紅字）。
		ok, err := u.profiles.Submit(ctx, profile.ID, profile.Version,
			SelfIssueTermsVersion, in.AcceptedByAccountID, now.Add(taxSubmitWindow), now)
		if err != nil {
			return err
		}
		if !ok {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		// detail 記下簽的是哪一版：稽核要能回答「這家在什麼時候同意了哪一版條款」，
		// 而 seller_tax_profiles 只留得住**最後一次**（T04 補簽會覆寫同兩欄）。
		// actor 記下是**誰**簽的：裁決 C 讓送審同時是簽署，而條款是自然人的意思表示
		// （§6.1.7）——只有版本沒有人，爭議時舉不出簽署主體。
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

// ---------------------------------------------------------------------------
// 賣家：端點 20 申請延長
// ---------------------------------------------------------------------------

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
		// allowed-from 逐字取自 §6.1.12 的相容表：EXTEND 只在 SELF_ISSUE_PENDING。
		//
		// ⚠️ 這兩道守衛與端點 17 的 canRequestExtension 旗標**是同一個判準的兩個出口**，
		// 唯一定義在 query_read.go 的 canRequestTaxExtension。改這裡要一起改那裡：
		// 2026-09-02 之前旗標只做了第二道（次數），於是 SUBMITTED 的賣家看得到
		// 「申請延長期限」按鈕、按下去必回 9321（#3021）。
		if profile.ReviewStatus != codemeta.SellerTaxReviewStatusSelfIssuePending.Value {
			return taxBiz(bizcode.SellerTaxProfileStateInvalid)
		}
		// 只能延一次（T11）。旗標為 false 時 UI 不渲染 CTA，但後端仍必須擋
		// ——UI 隱藏不是授權。
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

// ---------------------------------------------------------------------------
// 賣家：端點 22 接受自行開票責任條款
// ---------------------------------------------------------------------------

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
	// 58A §S04 逐字：termsVersion **必須原樣回送**，不符後端回 400。
	// 少了這一句，賣家可以簽下一個平台從未發布過的版本字串，而那份記錄是
	// 「審核期間由賣家自行開票、責任在賣家」的唯一書面憑據。
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

// ---------------------------------------------------------------------------
// ADM：端點 40 稅務審核裁決
// ---------------------------------------------------------------------------

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

// taxDecisionAllowedFrom 是「決策 × 允許的來源狀態」相容表。
//
// 🔴 **每一列逐字等於 §7.3.3 對應轉移的來源狀態欄（＝ CAS 的 allowed-from）**：
// 多一個值 ⇒ 該狀態下 CAS rows=0 而本層已經放行 ⇒ 靜默失敗（ADM 按下去沒反應，
// 稽核也沒有紀錄）；少一個值 ⇒ 合法決策被 9348 擋掉。
//
// ⚠️ SUPPLEMENT_REQUIRED 不在任何一列：賣家補齊後由 T06 先推回 UNDER_REVIEW。
// A03 對該狀態不顯示決策按鈕（§6.1.12），否則 ADM 按下去會拿到 9348 而不知道為什麼。
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

// taxDecisionNeedsConfirm 是需要二次確認的兩個決策（§6.1.12）。
//
// 🔴 confirmed 缺席或 false ⇒ **400**，不得用 9348：那是「狀態不相容」，語意不同。
// 二次確認是授權設計的一部分（B4：以強制填理由 ＋ 二次確認 ＋ 完整稽核取代細粒度
// capability），不是 UX。
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
		// P11 只在狀態**真的**改變時發（T16 是自轉移 ⇒ 不發）：
		// 事件名字叫 status_changed，發一則狀態沒變的出去，下游的去重鍵會撞。
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

// applyDecision 把七個決策折成一次 CAS，回「目標狀態」與（REVOKE 專有的）分界時點。
//
// 🔴 每一支的 rows=0 都回 9348 而不是靜默成功：本層剛剛才讀過狀態，rows=0 只可能是
// 「在讀與寫之間被別人改掉了」——那對 ADM 而言就是「這個決策現在不適用」。
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
		// 🔴 REQUIRED 需要 tax_id ＋ legal_name 齊備（chk_stp_required_needs_taxid）。
		// 少了這道守衛，DB CHECK 就是第一道防線：23514 冒泡成 500，ADM 看到 Postgres
		// 原文而不是「這個賣家要先補件」（#3049）。
		//
		// 這不是邊角案例：validateTaxIdentity（T02）只在賣家**自述 REQUIRED** 時要求統編，
		// 而「賣家自述免用、ADM 判定應開」正是 A03 這個頁面存在的理由 ⇒ 走完正常流程
		// 就會撞上。EXEMPT 那條路不檢查（chk_stp_exempt_no_taxid 反過來禁止帶統編）。
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value &&
			(derefString(profile.TaxID) == "" || derefString(profile.LegalName) == "") {
			return "", nil, taxBiz(bizcode.InvoiceTaxApprovalIdentityIncomplete)
		}
		// 🔴 **鏡像的另一半**（#3089 B 組第 5 項）：EXEMPT 反過來禁止帶統編
		// （chk_stp_exempt_no_taxid）。上面那道守衛在 2026-09-03 就補了（#3049），
		// 而這一邊當時只在註解裡寫了「EXEMPT 那條路不檢查」——於是同一個形狀
		// 在對稱的另一格原封不動地留著：profile 已有 tax_id 且狀態 UNDER_REVIEW 時
		// 核准免開，實打 **HTTP 500**。可達性與 #3049 逐字對稱。
		//
		// 回 9348（此審核決定與目前的稅務資格狀態不相容）而不是新增碼：ADM 的下一步
		// 是「改核應開，或先要求賣家清掉統編」——那正是 9348 描述的情境。
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value &&
			derefString(profile.TaxID) != "" {
			return "", nil, taxBiz(bizcode.InvoiceTaxDecisionInvalid)
		}
		// 🔴 **T08 的守衛本體**（2026-09-05 使用者裁決 DEC-20260905-015836 選 A）：
		// 核定免用統一發票之前，賣家必須已經簽過平台代製收據授權（S05）。
		//
		// 在此之前這道守衛**根本不存在**，而 S05 的 allowed-from 又只有
		// APPROVED_INVOICE_EXEMPT ⇒ 授權只可能發生在核定之後 ⇒ 設計書 §7 T08 寫的
		// 「核定免用統一發票需已取得收據代製授權」在實作上是反的。裁決 A 的兩半：
		// 放寬 S05 讓賣家提早簽（見 seller_tax_profiles.sql 的 ConfirmSellerReceiptAuthorization），
		// 核定側補上這道守衛，兩者缺一都回不到設計書寫的順序。
		//
		// 🔴 **寫在應用層而不是那句 CAS 的 WHERE**：塞進 WHERE 之後 rows=0 會同時代表
		// 「版本衝突」「狀態不對」「未授權」三件事，呼叫端只能猜——那正是本輪 P0-B
		// （createOperation 的 `if !inserted { return nil }`）的形狀。這裡與上面兩道
		// 對稱守衛同一個位置、同一個風格，各自回自己的碼。
		// ⚠️ 讀後寫的窗口由 version CAS 封住：賣家在這中間取消授權會使 version+1，
		//    下面那句 Approve 於是 rows=0 回 9348，不會核定出一張沒有授權的 EXEMPT。
		if next == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value &&
			profile.ReceiptAuthorizedAt == nil {
			return "", nil, taxBiz(bizcode.InvoiceTaxApprovalReceiptAuthMissing)
		}
		// approval_valid_until 恆由 approved_at 算出（I-63）。請求沒帶就用 365 天；
		// 帶了就必須嚴格大於 approved_at（chk_stp_valid_until_needs_approved），
		// 不然是 23514 而不是一句看得懂的 400。
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
		// T16 自轉移：狀態不變、**不動任何時鐘欄位**。留痕落在 invoice_status_history
		// （from_status = to_status），不在本表。
		next := codemeta.SellerTaxReviewStatusUnderReview.Value
		ok, err := u.profiles.KeepUnderReview(ctx, profile.ID, profile.Version, in.Reason, now)
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionReject.Value:
		next := codemeta.SellerTaxReviewStatusRejected.Value
		// cooldown_until 是 T01 第二形態那個守衛讀的欄位——少寫它，被駁回的賣家
		// 可以立刻無限重送。
		ok, err := u.profiles.Reject(ctx, profile.ID, profile.Version, in.Reason, now.Add(taxReapplyCooldown), now)
		return next, nil, taxCASResult(ok, err, next)

	case codemeta.SellerTaxReviewDecisionRevoke.Value:
		next := codemeta.SellerTaxReviewStatusRevoked.Value
		ok, err := u.profiles.Revoke(ctx, profile.ID, profile.Version, in.Reason, now)
		if e := taxCASResult(ok, err, next); e != nil {
			return "", nil, e
		}
		// 已代製收據不追溯，但要留分界時點（T-C04）：A09 的版本歷程據此顯示。
		effective := now
		return next, &effective, nil

	case codemeta.SellerTaxReviewDecisionReopenReview.Value:
		next := codemeta.SellerTaxReviewStatusUnderReview.Value
		ok, err := u.profiles.ReopenReview(ctx, profile.ID, profile.Version, in.Reason, now)
		return next, nil, taxCASResult(ok, err, next)
	}
	// known 已在呼叫端驗過，走到這裡代表 taxDecisionAllowedFrom 與本 switch 漂移了。
	return "", nil, ErrTaxDecisionUnknown
}

// ---------------------------------------------------------------------------
// ADM：端點 41 延長裁決
// ---------------------------------------------------------------------------

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
				// chk_stp_pending_window 保證 SELF_ISSUE_PENDING 必有這一欄；
				// 走到這裡代表資料已經不自洽 ⇒ 不要拿 now 當基準硬算一個新期限，
				// 那會把「原本什麼時候到期」這件事永久抹掉。
				return taxBiz(bizcode.SellerTaxProfileStateInvalid)
			}
			// 基準是**原到期日**不是 now：以 now 起算會讓一個早就快到期的案子
			// 被延成完整的 30 天，而 §6.1.12 要求「必填原到期日／新到期日」。
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
			// reason_code 用 EXTEND／REJECT：EXTEND 是 SellerTaxReviewDecision 的第 8 值，
			// 讓決策歷程看得出「誰在什麼時候、用什麼理由准了延長」——extension_count
			// 只說得出**有沒有**（§6.1.12）。
			ReasonCode:   taxExtensionReasonCode(in.Decision),
			ReasonDetail: in.Reason,
			Now:          now,
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

// ---------------------------------------------------------------------------
// 共用
// ---------------------------------------------------------------------------

// loadForSeller 解析 store_id → issuer → profile（Tier A 的兩段述詞都在 SQL）。
//
// 🔴 issuer 查無與 profile 查無**必須回同一個碼**（9320）：GetByRef 內部把 pgx.ErrNoRows
// 映成 9330（provider 語意），在稅務資格這條路徑上直接回它，會讓「這個 store 還沒有
// issuer」與「有 issuer 但沒送過審」對外可分辨，而兩者對外都只是「還沒設定」。
// ADM 路徑用同一支：越權在這條路上不成立（ADM 本來就看得到全部），但「查無」的形狀
// 必須與賣家側一致，否則同一份資料兩個入口回兩種碼。
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

// appendHistory 記一次狀態轉移（與造成它的那次寫入同交易）。
// actorAccountIDPtr 把 Gateway 注入的登入 principal 折成 status history 的 actor。
//
// 接受條款是**自然人的意思表示**（§6.1.7）：稽核要能回答「是誰按的」，而那是 account
// 層的事實不是 store 層的。0 ＝ 沒有可信身分 ⇒ NULL（誠實記成「查不到人」），
// 不用 store_id 頂替——賣家換手後 store_id 會指向新任賣家，而條款是前任簽的。
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

// taxStatusChangedEvent 是 P11 的 payload。
//
// 🔴 payload 邊界（36 號）：不放載具識別碼、買受人統編、email、電話，也不放 AppKey。
// 稅務身分事件本來就沒有那些欄位，但明寫在這裡是為了讓後來加欄位的人看得到界線。
// 🔴 wire 欄名一律 snake_case（§6.3.4「業務欄位一律 snake_case」）。
// 這支曾是全 12 個 invoice 事件裡唯一的 camelCase，後果不是風格不一致而是**功能失效**：
// outbox dispatcher 的 withNotifyRouting 找的是 `payload["store_id"]`，而這支只有
// `storeId` ⇒ 找不到 ⇒ notify_seller_account_id 恆為 "0" ⇒ 賣家永遠收不到稅務狀態通知，
// 全程沒有錯誤、沒有 DLQ、沒有 metric。
type taxStatusChangedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	TaxProfileID      string            `json:"tax_profile_id"`
	FromStatus        string            `json:"from_status"`
	ToStatus          string            `json:"to_status"`
	OccurredAt        string            `json:"occurred_at"`
}

// emitStatusChanged 把 P11 寫進 outbox（與狀態轉移同交易）。
func (u *TaxProfile) emitStatusChanged(
	ctx context.Context, issuer domain.Issuer, profile domain.SellerTaxProfile, next string, now time.Time,
) error {
	payload, err := json.Marshal(taxStatusChangedEvent{
		// 去重鍵帶目標狀態：同一列會經過多次轉移（REVOKE → REOPEN → APPROVE 是合法
		// 路徑），只用 profile id 當鍵會讓第二次之後的通知被 notification 去重掉。
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

// taxCASResult 把 CAS 的 (ok, err) 折成一個錯誤。
//
// 🔴 ok=false 一律回 9348，**不得**回 nil：那是 ADM 側靜默失敗的唯一形狀。
func taxCASResult(ok bool, err error, _ string) error {
	if err != nil {
		return err
	}
	if !ok {
		return taxBiz(bizcode.InvoiceTaxDecisionInvalid)
	}
	return nil
}

// taxBiz 造一個帶業務碼的 DomainError，message 一律取自 CodeMeta。
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

// validateTaxIdentity 是 T02 的「必填欄位齊全」守衛。
//
// 統編只在賣家自述 REQUIRED 時必填：EXEMPT 的賣家依 chk_stp_exempt_no_taxid 根本
// 不得帶統編，把它列成無條件必填會讓免用統一發票的賣家永遠送不出去。
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

// normalizeObligation 把賣家自述折進 chk_stp_obligation 的三值。
//
// 🔴 未知值一律落 UNKNOWN 而不是原樣寫入：原樣寫入是 23514（整個交易 aborted），
// 而 UNKNOWN 的語意正好是「尚未填答」——它也讓 T02 的守衛擋下這筆，賣家看到的是
// 「請把資料填完」而不是 500。
func normalizeObligation(v string) string {
	if _, ok := codemeta.SellerTaxObligationFromValue(v); ok {
		return v
	}
	return codemeta.SellerTaxObligationUnknown.Value
}

// joinAddress 把三段地址折成 business_address 單欄（DDL 只有一欄 varchar(255)）。
//
// ⚠️ **折起來就拆不回去**，所以 city／district 另外分開落庫（#3031，2026-09-02）：
// 本函式的輸出只進 business_address（AMEGO 送件與單行顯示用），而回程要還原到 S02
// 表單的三格靠的是 seller_tax_profiles.address_city／address_district。
// 在那之前 toSellerTaxIdentity 只填得出 detail ⇒ 賣家重新編輯時前兩格是空的。
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

// taxAuditStates 產生稽核列的 before／after。
//
// 只放狀態一欄：before_state／after_state 是 jsonb，塞整列 profile 會把統編、法定名稱、
// 負責人姓名複製進一張**保存 5 年**的表（§9.5）——那是憑空造一個五年期的 PII 倉庫，
// 而稽核要回答的問題只是「誰把它從什麼狀態改成什麼狀態」。
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

// taxExtensionReasonCode 把端點 41 的 APPROVE 映成 SellerTaxReviewDecision 的第 8 值。
//
// §6.1.12：端點 41 成功時要往決策歷程寫一筆 decision = "EXTEND"，否則歷程會漏掉
// 「誰在什麼時候、用什麼理由准了延長」。REJECT 沒有對應的 decision 值，原樣留著。
func taxExtensionReasonCode(decision string) string {
	if decision == taxExtensionApprove {
		return codemeta.SellerTaxReviewDecisionExtend.Value
	}
	return taxExtensionReject
}

// resolveReceiptApprovalBasis 驗並折出核定依據文件（§6.1.9，端點 40 的 APPROVE_INVOICE_EXEMPT）。
//
// 三條規則：
//
//	① **兩欄同進同出**：只給其中一個回 400（DB 的 chk_stp_receipt_basis_pair 會回 23514，
//	   而那對 ADM 是一句 500）。
//	② **只有 APPROVE_INVOICE_EXEMPT 收**：核定為「應開發票」的賣家不會走收據流程，
//	   給了它一份依據文件只會讓端點 28 顯示一個永遠用不到的下載連結。
//	③ 🔴 **必須是該賣家自己在 S02 上傳的稅籍文件之一**：媒體的授權述詞等價於
//	   「只有上傳者本人」⇒ 指名一份 ADM 自己上傳的檔案，賣家點下載一律 404，而
//	   端點 28 的三態只看得出 UPLOADER_NOT_SELLER、看不出是誰指錯的。在寫入時擋掉，
//	   ADM 當場就知道自己選錯了。
func resolveReceiptApprovalBasis(
	in TaxDecisionInput, profile domain.SellerTaxProfile, nextStatus string,
) (*string, *int64, error) {
	key, assetID := strings.TrimSpace(in.ApprovalBasisKey), strings.TrimSpace(in.ApprovalBasisAssetID)
	if key == "" && assetID == "" {
		return nil, nil, nil // 不指名（端點 28 回 NOT_DESIGNATED）
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
