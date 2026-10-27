package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// IssuerRepository 讀 invoice_issuers。
//
// issuer 不是 store_id：平台自身也是 issuer 而它沒有 store（§8.1.1(b)）。賣家 issuer
// 才由 issuer_ref_id 對到 sellers.store_id。
type IssuerRepository struct {
	baseRepo
}

// NewIssuerRepository 建立 repository。
func NewIssuerRepository(pool *pgxpool.Pool) *IssuerRepository {
	return &IssuerRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.IssuerRepository = (*IssuerRepository)(nil)

// GetByID 查單一開票主體。
func (r *IssuerRepository) GetByID(ctx context.Context, id int64) (domain.Issuer, error) {
	row, err := r.q(ctx).GetInvoiceIssuerByID(ctx, id)
	if err != nil {
		return domain.Issuer{}, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	return toIssuer(row), nil
}

// GetByRef 依外部身分（issuer_type ＋ ref id，賣家即 store_id）查。
//
// ⚠️ store incarnation：同一個實體賣家關店重開會拿到新的 store_id ⇒ 新的 issuer 列。
// 「查得到舊 issuer」不等於「這位賣家現在能開票」，那由 status 與 provider account 決定。
func (r *IssuerRepository) GetByRef(ctx context.Context, issuerType string, issuerRefID int64) (domain.Issuer, error) {
	row, err := r.q(ctx).GetInvoiceIssuerByRef(ctx, gen.GetInvoiceIssuerByRefParams{
		IssuerType:  issuerType,
		IssuerRefID: issuerRefID,
	})
	if err != nil {
		return domain.Issuer{}, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	return toIssuer(row), nil
}

// Create 建一列開票主體（賣家第一次存稅務資格草稿時，§6.1.7）。
//
// inserted=false ＝ ON CONFLICT DO NOTHING 的 rows=0。**不指名 conflict target** 是
// db/queries 那一側的決定：在 PostgreSQL 上交易內任一句失敗即整個交易 aborted（25P02），
// 讓它拋 23505 之後再 catch 是救不回來的（10 檔 §11.1）。
func (r *IssuerRepository) Create(ctx context.Context, in domain.NewIssuer) (domain.Issuer, bool, error) {
	row, err := r.q(ctx).InsertInvoiceIssuer(ctx, gen.InsertInvoiceIssuerParams{
		ID:          in.ID,
		IssuerType:  in.IssuerType,
		IssuerRefID: in.IssuerRefID,
		DisplayName: in.DisplayName,
		Now:         ts(in.Now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// RETURNING 沒有列 ＝ ON CONFLICT DO NOTHING 生效。這不是錯誤。
			return domain.Issuer{}, false, nil
		}
		return domain.Issuer{}, false, err
	}
	return toIssuer(row), true, nil
}

// UpdateDisplayName 把法定名稱同步進 display_name。
//
// rows=0 在本句是 **no-op**（值沒變或該 issuer 已退休），不是守衛不成立——呼叫端不得
// 因此中止整個請求。
func (r *IssuerRepository) UpdateDisplayName(ctx context.Context, id int64, displayName string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).UpdateInvoiceIssuerDisplayName(ctx, gen.UpdateInvoiceIssuerDisplayNameParams{
		DisplayName: displayName,
		Now:         ts(now),
		ID:          id,
	})
	return rows > 0, err
}

// SellerTaxProfileRepository 讀寫 seller_tax_profiles。
type SellerTaxProfileRepository struct {
	baseRepo
}

// NewSellerTaxProfileRepository 建立 repository。
func NewSellerTaxProfileRepository(pool *pgxpool.Pool) *SellerTaxProfileRepository {
	return &SellerTaxProfileRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.SellerTaxProfileRepository = (*SellerTaxProfileRepository)(nil)

// GetByID 查單筆稅務資格。
func (r *SellerTaxProfileRepository) GetByID(ctx context.Context, id int64) (domain.SellerTaxProfile, error) {
	row, err := r.q(ctx).GetSellerTaxProfileByID(ctx, id)
	if err != nil {
		return domain.SellerTaxProfile{}, notFound(err, bizcode.SellerTaxProfileNotFound)
	}
	return toSellerTaxProfile(row), nil
}

// GetByIssuer 依 issuer 查。
//
// DefaultTaxCategory 是 smallint，讀出來要走 domain.TaxCategoryFromDB 再用；
// 🔴 轉換失敗不得 fallback 成 1（應稅）——那會把一張免稅發票靜默開成應稅（§C.10 ③）。
func (r *SellerTaxProfileRepository) GetByIssuer(ctx context.Context, issuerID int64) (domain.SellerTaxProfile, error) {
	row, err := r.q(ctx).GetSellerTaxProfileByIssuer(ctx, issuerID)
	if err != nil {
		return domain.SellerTaxProfile{}, notFound(err, bizcode.SellerTaxProfileNotFound)
	}
	return toSellerTaxProfile(row), nil
}

// CountApprovalRecheckDue 給 ADM 儀表板與告警用。
func (r *SellerTaxProfileRepository) CountApprovalRecheckDue(ctx context.Context, now time.Time) (int64, error) {
	return r.q(ctx).CountApprovalRecheckDue(ctx, tp(now))
}

// ExpirePending 是集合式前置 UPDATE（寬限期到期 → 冷卻），無外部副作用 ⇒ 不需要 claim。
// 回被改到的列，讓呼叫端據此發通知。
func (r *SellerTaxProfileRepository) ExpirePending(ctx context.Context, now, cooldownUntil time.Time, limit int32) ([]domain.SellerTaxProfile, error) {
	rows, err := r.q(ctx).ExpirePendingTaxProfiles(ctx, gen.ExpirePendingTaxProfilesParams{
		CooldownUntil: tp(cooldownUntil),
		Now:           ts(now),
		RowLimit:      limit,
	})
	if err != nil {
		return nil, err
	}
	return toSellerTaxProfiles(rows), nil
}

// ConfirmReceiptAuthorization 是端點 29（S05 賣家同意／取消平台代製收據）。
//
// 🔴 回 false ＝ 守衛不成立（資格不是 APPROVED_INVOICE_EXEMPT，或版本被搶先）。
// 呼叫端必須分辨這兩者再回碼（9335 vs 9322），**不得把 false 當成功往下走**。
func (r *SellerTaxProfileRepository) ConfirmReceiptAuthorization(
	ctx context.Context, in domain.ConfirmReceiptAuthorization,
) (bool, error) {
	rows, err := r.q(ctx).ConfirmSellerReceiptAuthorization(ctx, gen.ConfirmSellerReceiptAuthorizationParams{
		AuthorizedAt:    in.AuthorizedAt,
		ConsentVersion:  in.ConsentVersion,
		Now:             ts(in.Now),
		ID:              in.ID,
		ExpectedVersion: in.ExpectedVersion,
	})
	return rows > 0, err
}

// ---------------------------------------------------------------------------
// 寫入面：M3 稅務資格狀態機（§7.3.3 T01–T16）
//
// 每一支都是一句 CAS，回的 bool ＝ rows>0。allowed-from、expectedVersion、7 天冷卻與
// 「只能延一次」全部住在 db/queries/seller_tax_profiles.sql 的 WHERE 裡——本層一個
// 判斷都不加。加了就會有兩份 allowed-from，而它們必然漂移（漂移的症狀是 CAS rows=0
// 靜默失敗，不是編譯錯）。
// ---------------------------------------------------------------------------

// Create 建 DRAFT（T01 的 `[*]` 形態）。
func (r *SellerTaxProfileRepository) Create(
	ctx context.Context, id, issuerID int64, now time.Time,
) (domain.SellerTaxProfile, bool, error) {
	row, err := r.q(ctx).InsertSellerTaxProfile(ctx, gen.InsertSellerTaxProfileParams{
		ID:       id,
		IssuerID: issuerID,
		Now:      ts(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// uq_seller_tax_profiles_issuer 已有一列 ⇒ 呼叫端改讀 GetByIssuer。
			return domain.SellerTaxProfile{}, false, nil
		}
		return domain.SellerTaxProfile{}, false, err
	}
	return toSellerTaxProfile(row), true, nil
}

// ResetToDraft 是 T01 的第二形態（REJECTED／EXPIRED → DRAFT）。
//
// false 的兩個成因（狀態不是那兩個 / 冷卻未到）在本層不可分辨——它們是同一句 WHERE
// 的兩段。呼叫端先讀一次以回可分辨的碼（9321 vs 9324）。
func (r *SellerTaxProfileRepository) ResetToDraft(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).ResetSellerTaxProfileToDraft(ctx, gen.ResetSellerTaxProfileToDraftParams{
		Now: ts(now),
		ID:  id,
	})
	return rows > 0, err
}

// UpdateDraft 是端點 18 的草稿存檔。
//
// 🔴 六個可空欄一律走 snp（空字串 → NULL）。tax_id 尤其不可存空字串：
// chk_stp_tax_id 是 `IS NULL OR ~ '^[0-9]{8}$'`，空字串當場 23514。
func (r *SellerTaxProfileRepository) UpdateDraft(ctx context.Context, in domain.UpdateSellerTaxDraft) (bool, error) {
	keys, err := json.Marshal(in.EvidenceObjectKeys)
	if err != nil {
		return false, err
	}
	rows, err := r.q(ctx).UpdateSellerTaxProfileDraft(ctx, gen.UpdateSellerTaxProfileDraftParams{
		ClaimedObligation:  in.ClaimedObligation,
		EntityTaxType:      snp(in.EntityTaxType),
		LegalName:          snp(in.LegalName),
		TaxID:              snp(in.TaxID),
		ResponsiblePerson:  snp(in.ResponsiblePerson),
		BusinessAddress:    snp(in.BusinessAddress),
		AddressCity:        snp(in.AddressCity),
		AddressDistrict:    snp(in.AddressDistrict),
		DefaultTaxCategory: in.DefaultTaxCategory,
		EvidenceObjectKeys: keys,
		ReviewNote:         snp(in.ReviewNote),
		Now:                ts(in.Now),
		ID:                 in.ID,
		ExpectedVersion:    in.ExpectedVersion,
	})
	return rows > 0, err
}

// Submit 是 T02 DRAFT → SUBMITTED，**同一句連條款簽署記錄一起寫**（裁決 C）。
//
// 🔴 attestationVersion **不走 snp**（那支把空字串折成 NULL）：這裡要的正好相反——
// 空字串必須原樣送進 DB，讓 chk_stp_attestation_version_nonblank 當場 23514。
// 折成 NULL 會撞 chk_stp_attestation_pair（attested_at 已寫、版本卻是 NULL），
// 一樣擋得住，但錯誤訊息會指向錯的那一欄。兩條路都不再有「靜默寫成 SUBMITTED」。
func (r *SellerTaxProfileRepository) Submit(
	ctx context.Context, id, expectedVersion int64, attestationVersion string, attestedBy int64,
	pendingValidUntil, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).SubmitSellerTaxProfile(ctx, gen.SubmitSellerTaxProfileParams{
		Now:                 tp(now),
		PendingValidUntil:   tp(pendingValidUntil),
		AttestationVersion:  &attestationVersion,
		AttestedByAccountID: nzp(attestedBy),
		ID:                  id,
		ExpectedVersion:     expectedVersion,
	})
	return rows > 0, err
}

// Attest 是 T04 SUBMITTED → SELF_ISSUE_PENDING。
func (r *SellerTaxProfileRepository) Attest(
	ctx context.Context, id, expectedVersion int64, attestationVersion string, attestedBy int64, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AttestSellerSelfIssue(ctx, gen.AttestSellerSelfIssueParams{
		AttestationVersion:  snp(attestationVersion),
		AttestedByAccountID: nzp(attestedBy),
		Now:                 tp(now),
		ID:                  id,
		ExpectedVersion:     expectedVersion,
	})
	return rows > 0, err
}

// RequestSupplement 是 T05 UNDER_REVIEW → SUPPLEMENT_REQUIRED。
func (r *SellerTaxProfileRepository) RequestSupplement(
	ctx context.Context, id, expectedVersion int64, note string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).RequestSellerTaxProfileSupplement(ctx, gen.RequestSellerTaxProfileSupplementParams{
		ReviewNote:      snp(note),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// Approve 是 T07／T08。
func (r *SellerTaxProfileRepository) Approve(
	ctx context.Context, in domain.ApproveSellerTaxProfile,
) (bool, error) {
	rows, err := r.q(ctx).ApproveSellerTaxProfile(ctx, gen.ApproveSellerTaxProfileParams{
		NextStatus:           in.NextStatus,
		Now:                  tp(in.Now),
		ApprovalValidUntil:   tp(in.ApprovalValidUntil),
		ApprovalBasisKey:     in.ApprovalBasisKey,
		ApprovalBasisAssetID: in.ApprovalBasisAssetID,
		ID:                   in.ID,
		ExpectedVersion:      in.ExpectedVersion,
	})
	return rows > 0, err
}

// Reject 是 T09。
func (r *SellerTaxProfileRepository) Reject(
	ctx context.Context, id, expectedVersion int64, note string, cooldownUntil, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).RejectSellerTaxProfile(ctx, gen.RejectSellerTaxProfileParams{
		CooldownUntil:   tp(cooldownUntil),
		ReviewNote:      snp(note),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// ExtendPending 是 T11（狀態不變）。
func (r *SellerTaxProfileRepository) ExtendPending(
	ctx context.Context, id, expectedVersion int64, pendingValidUntil time.Time, note string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).ExtendSellerTaxProfilePending(ctx, gen.ExtendSellerTaxProfilePendingParams{
		PendingValidUntil: tp(pendingValidUntil),
		ReviewNote:        snp(note),
		Now:               ts(now),
		ID:                id,
		ExpectedVersion:   expectedVersion,
	})
	return rows > 0, err
}

// ReopenReview 是 T12／T13／T15 → UNDER_REVIEW。
func (r *SellerTaxProfileRepository) ReopenReview(
	ctx context.Context, id, expectedVersion int64, recheckReason string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AdminReopenSellerTaxProfileReview(ctx, gen.AdminReopenSellerTaxProfileReviewParams{
		RecheckReason:   snp(recheckReason),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// Revoke 是 T14 APPROVED_* → REVOKED。
func (r *SellerTaxProfileRepository) Revoke(
	ctx context.Context, id, expectedVersion int64, note string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).RevokeSellerTaxProfile(ctx, gen.RevokeSellerTaxProfileParams{
		ReviewNote:      snp(note),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// KeepUnderReview 是 T16 的自轉移。
func (r *SellerTaxProfileRepository) KeepUnderReview(
	ctx context.Context, id, expectedVersion int64, note string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).KeepSellerTaxProfileUnderReview(ctx, gen.KeepSellerTaxProfileUnderReviewParams{
		ReviewNote:      snp(note),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// BuyerPreferenceRepository 讀 buyer_invoice_preferences。
//
// 帶 KeyRing 是因為 carrier_value_ciphertext 是 AES-256-GCM 密文（§5.1.4）：
// 明文出口收斂在本層，與 ProviderAccountRepository.WithDecryptedCredential 同形。
type BuyerPreferenceRepository struct {
	baseRepo
	keys *KeyRing
}

// NewBuyerPreferenceRepository 建立 repository。
func NewBuyerPreferenceRepository(pool *pgxpool.Pool, keys *KeyRing) *BuyerPreferenceRepository {
	return &BuyerPreferenceRepository{baseRepo: baseRepo{pool: pool}, keys: keys}
}

var _ domain.BuyerPreferenceRepository = (*BuyerPreferenceRepository)(nil)

// GetByID 查一筆常用發票資料。buyerID 是授權述詞的一部分（寫在 SQL 的 WHERE），
// 不是可省略的參數：省掉它，任何買家都能讀別人的載具遮罩與抬頭。
func (r *BuyerPreferenceRepository) GetByID(ctx context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	row, err := r.q(ctx).GetBuyerInvoicePreferenceByID(ctx, gen.GetBuyerInvoicePreferenceByIDParams{
		ID:      id,
		BuyerID: buyerID,
	})
	if err != nil {
		return domain.BuyerPreference{}, notFound(err, bizcode.InvoiceProfileNotFound)
	}
	return toBuyerPreference(row), nil
}

// GetDefault 取這個買家標成預設的那一筆（`USE_BUYER_DEFAULT` 的第一步，§6.1.2(b)）。
//
// 🔴 **不可用 ListByBuyer ＋ Go 端過濾代替**：那支帶 LIMIT，預設那一列排在 limit 之外時
// 會靜默回「沒有預設」⇒ 該張發票退成無載具，而買家設過。沒有任何東西會報錯。
//
// 查不到預設是**正常分支**（多數買家沒設過），回 (zero, false, nil)；
// 只有真的 DB 錯誤才回 error。把「沒設過」表現成 error 會讓 consumer 整批進 DLQ。
func (r *BuyerPreferenceRepository) GetDefault(ctx context.Context, buyerID int64) (domain.BuyerPreference, bool, error) {
	row, err := r.q(ctx).GetDefaultBuyerInvoicePreference(ctx, buyerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.BuyerPreference{}, false, nil
		}
		return domain.BuyerPreference{}, false, err
	}
	return toBuyerPreference(row), true, nil
}

// WithDecryptedCarrierValue 是載具顯碼明文的唯一出口，形狀照
// ProviderAccountRepository.WithDecryptedCredential：明文只活在 fn 的作用域內，
// 不進 domain struct、不進 log、不跨層傳遞。
//
// buyerID 是授權述詞的一部分（寫在 SQL 的 WHERE，I-47）——省掉它，任何買家都能解出
// 別人的載具號碼。查不到與不屬於你刻意不可分辨。
//
// 非載具型（統編／捐贈）的 preference 沒有密文欄 ⇒ 回 InvoiceProfileNotFound 而不是
// 空字串：呼叫端拿到空字串會以為「這個買家的載具是空的」，那是兩件不同的事。
func (r *BuyerPreferenceRepository) WithDecryptedCarrierValue(
	ctx context.Context, id, buyerID int64, fn func(carrierType, carrierValue string) error,
) error {
	row, err := r.q(ctx).GetBuyerInvoicePreferenceByID(ctx, gen.GetBuyerInvoicePreferenceByIDParams{
		ID:      id,
		BuyerID: buyerID,
	})
	if err != nil {
		return notFound(err, bizcode.InvoiceProfileNotFound)
	}
	if row.CarrierValueCiphertext == nil || row.CarrierType == nil {
		return pkgerrors.NewBusiness(bizcode.InvoiceProfileNotFound, "此常用發票資料不是載具型，沒有載具顯碼")
	}
	plain, err := r.keys.decryptOptional(row.CarrierValueCiphertext)
	if err != nil {
		return err
	}
	return fn(*row.CarrierType, plain)
}

// ListByBuyer 回這位買家存活的常用發票資料（B08 列表）。
//
// buyerID 進 SQL 的 WHERE（Tier A，§8.1.4(b)）；limit 進 SQL 的 LIMIT。
// 🔴 密文欄（carrier_value_ciphertext）**不解密**：列表只需要 carrier_value_mask，
// 每一次解密都是一次明碼可能外洩的機會（§8.1.3）。明文出口只有
// WithDecryptedCarrierValue 一條。
func (r *BuyerPreferenceRepository) ListByBuyer(ctx context.Context, buyerID int64, limit int32) ([]domain.BuyerPreference, error) {
	rows, err := r.q(ctx).ListBuyerInvoicePreferences(ctx, gen.ListBuyerInvoicePreferencesParams{
		BuyerID:  buyerID,
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.BuyerPreference, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBuyerPreference(row))
	}
	return out, nil
}

// Create 新增一列常用發票資料（B08 端點 2；端點 3 的編輯也先走它）。
//
// 🔴 明碼在本層落地：encryptOptional ＋ hashOptional 是「任何寫路徑必經加密」的實作點
// （mapper.go 檔頭）。上層拿不到密文、也不需要知道用了哪一版金鑰。
//
// inserted=false ＝ ON CONFLICT DO NOTHING 的 rows=0。ON CONFLICT **不指名 target**
// （本表三個 partial unique），所以這裡分不出撞的是載具去重鍵還是統編去重鍵——
// 呼叫端只需要知道「這位買家已經有一列相同的存活資料」。
func (r *BuyerPreferenceRepository) Create(
	ctx context.Context, in domain.NewBuyerPreference, now time.Time,
) (domain.BuyerPreference, bool, error) {
	ciphertext, err := r.keys.encryptOptional(in.CarrierValue)
	if err != nil {
		return domain.BuyerPreference{}, false, err
	}
	row, err := r.q(ctx).InsertBuyerInvoicePreference(ctx, gen.InsertBuyerInvoicePreferenceParams{
		ID:                     in.ID,
		BuyerID:                in.BuyerID,
		PreferenceKind:         in.PreferenceKind,
		CarrierType:            snp(in.CarrierType),
		CarrierValueCiphertext: ciphertext,
		// chk_bip_hash_pair 是雙向等式（密文與 hash 同時 NULL 或同時非 NULL）⇒
		// 兩者必須由同一個輸入決定，不能一個看 CarrierValue、另一個看別的欄位。
		CarrierValueHash: r.keys.hashOptional(in.CarrierValue),
		CarrierValueMask: snp(in.CarrierMask),
		TaxID:            snp(in.TaxID),
		TaxTitle:         snp(in.TaxTitle),
		DonationCode:     snp(in.DonationCode),
		Label:            snp(in.Label),
		IsDefault:        in.IsDefault,
		Now:              ts(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// RETURNING 沒有列 ＝ ON CONFLICT DO NOTHING 生效。這不是錯誤。
			return domain.BuyerPreference{}, false, nil
		}
		return domain.BuyerPreference{}, false, err
	}
	return toBuyerPreference(row), true, nil
}

// ClearDefault 清掉這位買家所有存活列的 is_default。與 SetDefault 成對，必須同交易。
func (r *BuyerPreferenceRepository) ClearDefault(ctx context.Context, buyerID int64, now time.Time) (int64, error) {
	return r.q(ctx).ClearDefaultBuyerInvoicePreferences(ctx, gen.ClearDefaultBuyerInvoicePreferencesParams{
		Now:     ts(now),
		BuyerID: buyerID,
	})
}

// SetDefault 把一列標成預設。buyerID 在 SQL 的 WHERE（Tier A）。
func (r *BuyerPreferenceRepository) SetDefault(ctx context.Context, id, buyerID int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).SetDefaultBuyerInvoicePreference(ctx, gen.SetDefaultBuyerInvoicePreferenceParams{
		Now:     ts(now),
		ID:      id,
		BuyerID: buyerID,
	})
	return rows > 0, err
}

// Supersede 把舊列指向新列。呼叫端必須已經建好 supersededByID 那一列（fk_bip_superseded）。
func (r *BuyerPreferenceRepository) Supersede(
	ctx context.Context, id, buyerID, supersededByID int64, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).SupersedeBuyerInvoicePreference(ctx, gen.SupersedeBuyerInvoicePreferenceParams{
		Now:            tp(now),
		SupersededByID: &supersededByID,
		ID:             id,
		BuyerID:        buyerID,
	})
	return rows > 0, err
}

// SoftDelete 軟刪一列（本表是全 schema 唯一有 soft delete 的表）。
func (r *BuyerPreferenceRepository) SoftDelete(ctx context.Context, id, buyerID int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).SoftDeleteBuyerInvoicePreference(ctx, gen.SoftDeleteBuyerInvoicePreferenceParams{
		Now:     tp(now),
		ID:      id,
		BuyerID: buyerID,
	})
	return rows > 0, err
}

// BumpPendingOrderCounters 累加待審期間的接單額度。
//
// 不回 rows：呼叫端已經在同一交易內建了 document，profile 必然存在；rows=0 只可能
// 是 id 傳錯，而那會在更上游炸掉。回 rows 反而會誘使呼叫端寫一段永遠不會執行的分支。
func (r *SellerTaxProfileRepository) BumpPendingOrderCounters(ctx context.Context, id, orderAmount int64, now time.Time) error {
	_, err := r.q(ctx).BumpSellerTaxProfilePendingOrderCounters(ctx, gen.BumpSellerTaxProfilePendingOrderCountersParams{
		OrderAmount: orderAmount,
		Now:         ts(now),
		ID:          id,
	})
	return err
}
