package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// ProviderAccountRepository 是 invoice_provider_accounts 的 thin adapter。
//
// 本檔是全服務唯一持有 KeyRing 的地方。所有寫入路徑都必須經過 keys.deriveCredential，
// 所有讀出明文的路徑都必須經過 WithDecryptedCredential——中間沒有第三條路。
type ProviderAccountRepository struct {
	baseRepo
	keys *KeyRing
}

// NewProviderAccountRepository 建立 repository。
func NewProviderAccountRepository(pool *pgxpool.Pool, keys *KeyRing) *ProviderAccountRepository {
	return &ProviderAccountRepository{baseRepo: baseRepo{pool: pool}, keys: keys}
}

var _ domain.ProviderAccountRepository = (*ProviderAccountRepository)(nil)

// GetByID 查單列。查無列回 9330（尚未設定電子發票服務商連線）。
func (r *ProviderAccountRepository) GetByID(ctx context.Context, id int64) (domain.ProviderAccount, error) {
	row, err := r.q(ctx).GetProviderAccountByID(ctx, id)
	if err != nil {
		return domain.ProviderAccount{}, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	return toProviderAccount(row), nil
}

// GetByIssuer 依 issuer ＋ provider 查。
//
// 查無列 ＝「未設定」，這是 §8.1.1(b) 明文規定的表達方式：NOT_CONFIGURED 在四個
// NOT NULL 之下根本寫不進來，所以不存在「有列但沒憑證」這個狀態。
func (r *ProviderAccountRepository) GetByIssuer(ctx context.Context, issuerID int64, provider string) (domain.ProviderAccount, error) {
	row, err := r.q(ctx).GetProviderAccountByIssuer(ctx, gen.GetProviderAccountByIssuerParams{
		IssuerID: issuerID,
		Provider: provider,
	})
	if err != nil {
		return domain.ProviderAccount{}, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	return toProviderAccount(row), nil
}

// ListActive 回全部 ACTIVE 的憑證（INV.trk 字軌餘量掃描用，唯讀、不帶租約）。
func (r *ProviderAccountRepository) ListActive(ctx context.Context) ([]domain.ProviderAccount, error) {
	rows, err := r.q(ctx).ListActiveProviderAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return toProviderAccounts(rows), nil
}

// FetchDueHealthChecks 是 INV.hc 的候選。
// FetchDueHealthChecks 的 unavailableSinceFloor 是 §7.3.4 P08「自動重驗最多 24 小時」的
// 下界（呼叫端算 now-24h）：breaker_opened_at 早於它的 PROVIDER_UNAVAILABLE 列已逾期，
// 不再自動重驗、只留 ADM 端點 44 手動。policy 在 usecase，本層只轉手。
func (r *ProviderAccountRepository) FetchDueHealthChecks(ctx context.Context, now, unavailableSinceFloor time.Time, limit int32) ([]domain.ProviderAccount, error) {
	rows, err := r.q(ctx).FetchDueHealthChecks(ctx, gen.FetchDueHealthChecksParams{
		Now:                   ts(now),
		UnavailableSinceFloor: tp(unavailableSinceFloor),
		RowLimit:              limit,
	})
	if err != nil {
		return nil, err
	}
	return toProviderAccounts(rows), nil
}

// ClaimHealthCheck 搶佔一列。
//
// leaseUntil 必須由呼叫端在**處理單筆的當下**現算（time.Now().Add(ClaimLease)），
// 不可在批次迴圈外算一次——那會讓不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 <
// 租約」，隨 batch-size 一起破（《多副本排程搶佔規範》）。
//
// 回 false ＝ 沒搶到（別的副本先到、或述詞已不成立），不是錯誤。
// ClaimHealthCheck 的述詞與 FetchDueHealthChecks 逐字鏡射（含 unavailableSinceFloor）：
// 兩者不一致時，撈得到卻 claim 不到的列會每輪重試、每輪失敗，而且完全無聲。
func (r *ProviderAccountRepository) ClaimHealthCheck(ctx context.Context, id int64, now, leaseUntil, unavailableSinceFloor time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimHealthCheck(ctx, gen.ClaimHealthCheckParams{
		LeaseUntil:            ts(leaseUntil),
		Now:                   ts(now),
		ID:                    id,
		ClaimableBefore:       ts(now),
		UnavailableSinceFloor: tp(unavailableSinceFloor),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// RecordHealthCheckResult 落一輪巡檢的結果。
//
// CheckedAt 寫進 last_health_checked_at（事實時間戳：真的打了 AMEGO），NextCheckAt 寫進
// 租約欄。兩者不同源是刻意的——把租約推到 last_health_checked_at 會讓「這個帳戶多久沒被
// 檢查過」那條告警永遠不響（§4.6 W-02）。
func (r *ProviderAccountRepository) RecordHealthCheckResult(ctx context.Context, in domain.RecordHealthCheckResult) error {
	_, err := r.q(ctx).RecordHealthCheckResult(ctx, gen.RecordHealthCheckResultParams{
		Now:                 tp(in.CheckedAt),
		NextCheckAt:         ts(in.NextCheckAt),
		NextStatus:          in.NextStatus,
		StatusReason:        in.StatusReason,
		BreakerOpenedAt:     in.BreakerOpenedAt,
		BreakerProbeSuccess: in.BreakerProbeSuccess,
		ID:                  in.ID,
	})
	return err
}

// Create 首次綁定憑證。
//
// 走 INSERT … ON CONFLICT DO NOTHING（**不指名 conflict target**：本表有兩個唯一索引，
// 指名其一 ⇒ 另一個衝突時拋 23505 ⇒ PostgreSQL 整個交易 aborted 25P02，catch 了也救不回來）。
//
// rows=0 ⇒ 這組統編＋AppKey 已被另一個**同時有效**的 issuer 綁走（uq_ipa_fingerprint，
// 擋 AMEGO S17「一組字軌只能由一套系統使用」），或該 issuer 已有同 provider 的列。
// 🔴 不得改成「先查再寫」：先查再寫在併發下擋不住（§8.1.1(f) U1）。
func (r *ProviderAccountRepository) Create(ctx context.Context, in domain.NewProviderAccount) error {
	mat, err := r.keys.deriveCredential(in.ProviderTaxID, in.AppKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rows, err := r.q(ctx).InsertProviderAccount(ctx, gen.InsertProviderAccountParams{
		ID:                    in.ID,
		IssuerID:              in.IssuerID,
		Provider:              in.Provider,
		ProviderTaxID:         in.ProviderTaxID,
		CredentialCiphertext:  mat.ciphertext,
		CredentialKeyVersion:  mat.keyVersion,
		CredentialFingerprint: mat.fingerprint,
		CredentialLast4:       mat.last4,
		Status:                in.Status,
		HealthCheckNextAt:     ts(in.HealthCheckNextAt),
		Now:                   ts(now),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		// rows=0 的唯一成因是 uq_ipa_fingerprint 撞到：這組（統編, AppKey）已被
		// 另一個 issuer 綁走（§8.1.1(f) U1——由 DB 唯一約束承擔，不是應用層先查再寫，
		// 後者在併發下擋不住）。
		//
		// ⚠️ **不能回 9331**（憑證驗證失敗，請確認統一編號與 AppKey）：那會把賣家
		// 引導去改一把其實正確的金鑰，而正解是「這組憑證已被別人綁走」。兩者的處置
		// 完全相反。9337 是 2026-08-29 為此新增的專屬碼。
		return biz(bizcode.InvoiceProviderCredentialTaken)
	}
	return nil
}

// StartCredentialRotation 寫 pending_* 三欄並把 status 推到 ROTATION_PENDING。
//
// 三欄與 status 必須同一句：chk_ipa_rotation 是雙向等式、chk_ipa_rotation_triplet 要求
// 三欄同進同出，少做一半當場 23514。
//
// rows=0 ＝ 樂觀鎖衝突或 status 已不是可輪替的狀態 ⇒ 9322，呼叫端不得當成功。
func (r *ProviderAccountRepository) StartCredentialRotation(ctx context.Context, in domain.StartCredentialRotation) error {
	mat, err := r.keys.deriveCredential(in.ProviderTaxID, in.AppKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rows, err := r.q(ctx).StartProviderCredentialRotation(ctx, gen.StartProviderCredentialRotationParams{
		PendingCiphertext:  sp(mat.ciphertext),
		PendingFingerprint: sp(mat.fingerprint),
		PendingLast4:       sp(mat.last4),
		Now:                tp(now),
		ID:                 in.ID,
		ExpectedVersion:    in.ExpectedVersion,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return biz(bizcode.SellerTaxProfileVersionConflict)
	}
	return nil
}

// PromoteCredential 把驗證通過的新憑證扶正。
//
// 一律用 current master key 重新加密（不是把 pending 密文原樣搬過去）：credential_key_version
// 因此永遠等於 CurrentVersion，這正是 §5.1.3 沒有 pending_key_version 欄的前提。
// 同一句清空 pending 三欄並移出 ROTATION_PENDING，理由同 StartCredentialRotation。
func (r *ProviderAccountRepository) PromoteCredential(ctx context.Context, in domain.PromoteCredential) error {
	mat, err := r.keys.deriveCredential(in.ProviderTaxID, in.AppKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rows, err := r.q(ctx).UpdateProviderAccountCredential(ctx, gen.UpdateProviderAccountCredentialParams{
		CredentialCiphertext:  mat.ciphertext,
		CredentialKeyVersion:  mat.keyVersion,
		CredentialFingerprint: mat.fingerprint,
		CredentialLast4:       mat.last4,
		NextStatus:            in.NextStatus,
		Now:                   ts(now),
		ID:                    in.ID,
		ExpectedVersion:       in.ExpectedVersion,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return biz(bizcode.SellerTaxProfileVersionConflict)
	}
	return nil
}

// UpdateStatus 是 ADM／賣家路徑的狀態轉移，走 expectedVersion CAS。
//
// rows=0 必須當衝突處理，不可當成功：P07 缺這條邊時靜默失敗 ⇒ 前端顯示成功而狀態沒變
// ⇒ 賣家永遠串不上。
func (r *ProviderAccountRepository) UpdateStatus(ctx context.Context, in domain.UpdateProviderAccountStatus) error {
	now := time.Now().UTC()
	rows, err := r.q(ctx).UpdateProviderAccountStatus(ctx, gen.UpdateProviderAccountStatusParams{
		NextStatus:        in.NextStatus,
		StatusReason:      in.StatusReason,
		VerifiedAt:        in.VerifiedAt,
		HealthCheckNextAt: ts(in.HealthCheckNextAt),
		Now:               ts(now),
		ID:                in.ID,
		ExpectedVersion:   in.ExpectedVersion,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return biz(bizcode.SellerTaxProfileVersionConflict)
	}
	return nil
}

// MatchesCurrentCredential 回答 §8.1.1(f) U3：賣家送來的這把 key 是不是就是現行那把。
//
// 比對走 fingerprint 而不是解密後比字串，理由是前者不需要把明文解出來；而 fingerprint
// 本身不離開本層（§8.1.3 C 類）。相同 ⇒ 呼叫端回「這與現行憑證相同」，**不寫任何欄位、
// 不打 AMEGO**——否則賣家以為換過了，實際上外洩的那一把還在用。
func (r *ProviderAccountRepository) MatchesCurrentCredential(ctx context.Context, id int64, providerTaxID, appKey string) (bool, error) {
	row, err := r.q(ctx).GetProviderAccountByID(ctx, id)
	if err != nil {
		return false, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	fp := credentialFingerprint(r.keys.LookupHMACKey, providerTaxID, normalizeAppKey(appKey))
	return fp == row.CredentialFingerprint, nil
}

// WithDecryptedCredential 是 AppKey 明文唯一的出口，而它其實沒有出口：明文以參數形式
// 進入 fn 的區域變數，永遠不是回傳值。
//
// 開票路徑（§8.1.1(h) 第 5 段）就走這裡：明文只存在於 AmegoClient.sign 計算 md5 的那一刻，
// 且 AppKey 不出現在送出的 HTTP body（AMEGO 只有 invoice/data/time/sign 四欄）⇒ provider
// 稽核表存 request 原文並不會存到 AppKey。
func (r *ProviderAccountRepository) WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error {
	row, err := r.q(ctx).GetProviderAccountByID(ctx, id)
	if err != nil {
		return notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	appKey, err := r.keys.openCredential(row)
	if err != nil {
		return err
	}
	return fn(row.ProviderTaxID, appKey)
}

// ---------------------------------------------------------------------------
// §6.1.8／§6.1.13 的狀態轉移（端點 24／25／26／27／44／45／66／67）
//
// 七支共通的形狀：allowed-from 與 expectedVersion 都在 SQL 的 WHERE，本層只把
// rows>0 折成 bool。**rows=0 不是錯誤也不是成功**，是「守衛不成立」——呼叫端必須
// 回明確業務碼（§7.3.0 規則 4）。
// ---------------------------------------------------------------------------

// ResubmitCredential 是端點 24 的覆寫分支。
//
// taken=true 的唯一成因是 uq_ipa_fingerprint：這組（統編, AppKey）已被另一個**同時有效**
// 的 issuer 綁走。UPDATE 沒有 ON CONFLICT ⇒ 只會拋 23505，而 PostgreSQL 交易內任一句失敗
// 即整個交易 aborted（25P02）⇒ 呼叫端拿到 taken=true 之後**不得再下任何一句 SQL**，
// 只能讓交易 rollback 並回 9337。
//
// 🔴 不得回 9331（憑證驗證失敗）：那會把賣家引導去改一把其實正確的金鑰，而正解是
// 「這組憑證已被別人綁走」。兩者的處置完全相反。
func (r *ProviderAccountRepository) ResubmitCredential(ctx context.Context, in domain.ResubmitProviderCredential) (bool, bool, error) {
	mat, err := r.keys.deriveCredential(in.ProviderTaxID, in.AppKey)
	if err != nil {
		return false, false, err
	}
	now := time.Now().UTC()
	rows, err := r.q(ctx).ResubmitProviderCredential(ctx, gen.ResubmitProviderCredentialParams{
		CredentialCiphertext:  mat.ciphertext,
		CredentialKeyVersion:  mat.keyVersion,
		CredentialFingerprint: mat.fingerprint,
		CredentialLast4:       mat.last4,
		ProviderTaxID:         in.ProviderTaxID,
		HealthCheckNextAt:     ts(in.HealthCheckNextAt),
		Now:                   ts(now),
		ID:                    in.ID,
		ExpectedVersion:       in.ExpectedVersion,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return false, true, nil
		}
		return false, false, err
	}
	return rows > 0, false, nil
}

// StartVerification 是 P02／P07／P15 的共同入口。
func (r *ProviderAccountRepository) StartVerification(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).StartProviderVerification(ctx, gen.StartProviderVerificationParams{
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// LandVerification 是 P03–P06 的共同落點。
func (r *ProviderAccountRepository) LandVerification(ctx context.Context, in domain.LandProviderVerification) (bool, error) {
	rows, err := r.q(ctx).LandProviderVerification(ctx, gen.LandProviderVerificationParams{
		NextStatus:        in.NextStatus,
		StatusReason:      nilIfBlank(in.StatusReason),
		VerifiedAt:        in.VerifiedAt,
		BreakerOpenedAt:   in.BreakerOpenedAt,
		Now:               tp(in.Now),
		HealthCheckNextAt: ts(in.NextCheckAt),
		ID:                in.ID,
		ExpectedVersion:   in.ExpectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// FailVerificationFromActive 是端點 44 定版 ③ 的前半（ACTIVE／DEGRADED → 憑證類錯誤狀態）。
//
// in.VerifiedAt 在本句被忽略：verified_at 是「上次驗過」的事實，不因為這次失敗而消失。
func (r *ProviderAccountRepository) FailVerificationFromActive(ctx context.Context, in domain.LandProviderVerification) (bool, error) {
	rows, err := r.q(ctx).FailProviderVerificationFromActive(ctx, gen.FailProviderVerificationFromActiveParams{
		NextStatus:        in.NextStatus,
		StatusReason:      nilIfBlank(in.StatusReason),
		Now:               tp(in.Now),
		HealthCheckNextAt: ts(in.NextCheckAt),
		ID:                in.ID,
		ExpectedVersion:   in.ExpectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// TouchHealthCheck 是端點 44 定版 ③ 的後半（status 完全不動）。
func (r *ProviderAccountRepository) TouchHealthCheck(ctx context.Context, id, expectedVersion int64, now, nextCheckAt time.Time) (bool, error) {
	rows, err := r.q(ctx).TouchProviderHealthCheck(ctx, gen.TouchProviderHealthCheckParams{
		Now:               tp(now),
		HealthCheckNextAt: ts(nextCheckAt),
		ID:                id,
		ExpectedVersion:   expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// AbortCredentialRotation 是 P13（保留原 key）。
func (r *ProviderAccountRepository) AbortCredentialRotation(ctx context.Context, id, expectedVersion int64, statusReason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).AbortProviderCredentialRotation(ctx, gen.AbortProviderCredentialRotationParams{
		StatusReason:    nilIfBlank(statusReason),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// SetAutomationCutover 是端點 26（I-61）。
func (r *ProviderAccountRepository) SetAutomationCutover(ctx context.Context, id, expectedVersion int64, cutoverAt, now time.Time) (bool, error) {
	rows, err := r.q(ctx).SetProviderAutomationCutover(ctx, gen.SetProviderAutomationCutoverParams{
		CutoverAt:       tp(cutoverAt),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// Suspend 是 P14（端點 45）。
func (r *ProviderAccountRepository) Suspend(ctx context.Context, id, expectedVersion int64, reason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).SuspendProviderAccount(ctx, gen.SuspendProviderAccountParams{
		StatusReason:    nilIfBlank(reason),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// Revoke 是 P16（端點 67）。M4 唯一終態。
func (r *ProviderAccountRepository) Revoke(ctx context.Context, id, expectedVersion int64, reason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).RevokeProviderAccount(ctx, gen.RevokeProviderAccountParams{
		StatusReason:    nilIfBlank(reason),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
