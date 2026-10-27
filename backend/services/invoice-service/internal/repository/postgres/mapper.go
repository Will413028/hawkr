package postgres

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// ============================================================================
// AppKey 密文 ↔ 明文的唯一收斂點（58 §8.1.1(c)）
// ============================================================================
//
// 放在 mapper 而不是 usecase：「任何讀路徑必經解密、任何寫路徑必經加密」由唯一的映射
// 函式保證；放 usecase 則每新增一個 usecase 就多一次「這次記得加密了嗎」。
//
// 「誰能解密 AppKey」＝「誰能部署 invoice-service」——所以本區塊的每個硬錯都不是防禦性
// 程式碼，是那句話唯一的實作。

// KeyRing 統一持有 master key 的現行版與前一版，依列上的 credential_key_version
// （smallint，不是字串標籤）選 cipher。
//
// 為什麼住在 repository 而不是 config：它唯一的消費者是本檔的 decryptCredential，
// 而 cipherFor 的語意（版本對不上就硬錯、不 try-and-fallback）是加解密收斂點的一部分。
type KeyRing struct {
	Current        *crypto.Cipher // 一定非 nil（Config.Validate ① 保證 master key 非空）
	Prev           *crypto.Cipher // 可能為 nil：非輪替期間就是 nil，解密端必須自己判
	CurrentVersion int16
	PrevVersion    int16 // 0 ＝ 沒有前一版（版本號從 1 起算）
	LookupHMACKey  string
}

// cipherFor 依版本選 cipher；對不上就硬錯，**不做 try-and-fallback**。
//
// AES-GCM 的認證標籤讓 fallback 在安全上沒問題，但它會把「這一列真的損毀了」與
// 「這一列是舊 key 加密的」壓成同一條路徑 ⇒ master key 輪替步驟 3 的驗證述詞
// （SELECT count(*) WHERE credential_key_version <> 新版本號 回 0）失去意義，
// 而輪替最容易出錯的正是「以為換完了其實沒換完」。
func (k *KeyRing) cipherFor(v int16) (*crypto.Cipher, error) {
	switch v {
	case k.CurrentVersion:
		return k.Current, nil
	case k.PrevVersion:
		if k.Prev == nil { // _PREV 已在輪替步驟 4 移除，卻還有殘列
			return nil, fmt.Errorf("row encrypted with retired key version %d", v)
		}
		return k.Prev, nil
	default:
		return nil, fmt.Errorf("unknown credential_key_version %d", v)
	}
}

// normalizeAppKey 是 AppKey 進入本層之後的**唯一**正規化：只 trim 前後空白，
// 不改大小寫、不做 Unicode 正規化（§8.1.1(f) 逐字）。
//
// 為什麼一定要在同一個地方做：密文、fingerprint、last4 三者都由它推導，任何一個用了
// 未 trim 的版本，就會出現「fingerprint 對得上但解出來的 key 打不通 AMEGO」這種
// 沒有任何斷言擋得住的錯位。
func normalizeAppKey(appKey string) string { return strings.TrimSpace(appKey) }

// encryptCredential 加密 AMEGO AppKey。
//
// 空明文一律硬錯，不映射成 NULL：§5.1.3 的 credential_ciphertext 是 NOT NULL，
// 「尚未設定憑證」在資料層的表達是「這一列不存在」。讓空字串走進來會在 DB 端撞 23502，
// 而那個錯誤看起來像 schema 壞掉而不是「呼叫端傳了空字串」。
//
// ⚠️ 這裡**不做長度檢查**（§8.1.1(b) 衍生要求 ①）：`len(appKey)==20` 之類只是把同一個
// 炸點從 DB 搬到 handler；欄位型別是 text 正是為了讓「AppKey 有多長」不再是 schema 風險。
func encryptCredential(enc *crypto.Cipher, plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("refusing to encrypt an empty app_key (no credential ⇒ no row)")
	}
	return enc.EncryptToBase64(plaintext)
}

// decryptCredential 解不開一律硬錯，且錯誤訊息**刻意不回顯 stored**——它要嘛是密文、
// 要嘛是一把還沒加密的真 AppKey，兩者都不該進 log。
//
// 三種成因（未加密的殘列／損毀／key 版本錯）沒有一種可以「當作沒事繼續」：拿錯的字串
// 去算 md5 sign，AMEGO 回 code 16，看起來像賣家貼錯 key ⇒ 我們會請賣家去改一把其實
// 完全正確的金鑰。
func decryptCredential(enc *crypto.Cipher, stored string) (string, error) {
	plaintext, err := enc.DecryptFromBase64(stored)
	if err != nil {
		return "", errors.New("credential did not decrypt (unencrypted row, corrupted value, or wrong key version)")
	}
	return plaintext, nil
}

// credentialFingerprint 逐字實作 §8.1.1(f) 的組成：provider_tax_id ＋ 換行 ＋ app_key。
//
// 含統編而不是裸 app_key 的理由是**特定形狀**：「B 把 A 的統編＋A 的 AppKey 一起貼進來」
// （B 開的票掛在 A 的統編下）在含統編之下 fingerprint 必然相同、被 uq_ipa_fingerprint
// 擋住。另一形狀「B 用自己的統編＋A 的 AppKey」不需要被這裡擋：統編與 AppKey 不配對時
// AMEGO 直接回 code 16，連線測試當場失敗、狀態進 INVALID_CREDENTIAL，自動 fail-closed。
//
// ⚠️ 拿掉統編會多擋一種**合法**情形：光貿若對不同帳戶換發同一把 AppKey，兩個真實無關的
// 賣家會被誤判成重複。
func credentialFingerprint(lookupHMACKey, providerTaxID, appKey string) string {
	return crypto.HMACLookup(lookupHMACKey, providerTaxID+"\n"+appKey)
}

// credentialLast4 取明文尾四碼供 UI 遮罩顯示（§8.1.3 B 類）。
// 以 rune 取尾，避免非 ASCII 輸入把多位元組字元切一半（欄位是 varchar(4)）。
func credentialLast4(appKey string) string {
	r := []rune(appKey)
	if len(r) <= 4 {
		return string(r)
	}
	return string(r[len(r)-4:])
}

// credentialMaterial 是「一把明文 AppKey」在 DB 上的三個衍生值。三者永遠一起算、
// 一起寫：只寫其中一部分會撞 §5.1.3 的 NOT NULL 或讓 fingerprint 與密文指向不同的 key。
type credentialMaterial struct {
	ciphertext  string
	keyVersion  int16
	fingerprint string
	last4       string
}

// deriveCredential 是本層所有寫入路徑（首次綁定／輪替提交／輪替扶正）的共同入口。
// 一律用 **current** master key 加密：pending_ciphertext 與 credential_ciphertext 都是
// 「寫的當下」加密的，所以永遠同版本——§5.1.3 刻意沒有 pending_key_version 欄，正是
// 因為這個不變式成立。
func (k *KeyRing) deriveCredential(providerTaxID, appKey string) (credentialMaterial, error) {
	plain := normalizeAppKey(appKey)
	ciphertext, err := encryptCredential(k.Current, plain)
	if err != nil {
		return credentialMaterial{}, err
	}
	return credentialMaterial{
		ciphertext:  ciphertext,
		keyVersion:  k.CurrentVersion,
		fingerprint: credentialFingerprint(k.LookupHMACKey, providerTaxID, plain),
		last4:       credentialLast4(plain),
	}, nil
}

// openCredential 解出某一列的 AppKey 明文。**只有 WithDecryptedCredential 會呼叫它**，
// 回傳值不得離開那個 callback。
func (k *KeyRing) openCredential(row gen.InvoiceProviderAccount) (string, error) {
	enc, err := k.cipherFor(row.CredentialKeyVersion)
	if err != nil {
		return "", err
	}
	return decryptCredential(enc, row.CredentialCiphertext)
}

// ============================================================================
// row → domain
// ============================================================================

// toProviderAccount 是唯一手寫的映射：它**丟掉**密文與 fingerprint 四欄。
// 逐欄理由見 domain.ProviderAccount 的註解。
func toProviderAccount(row gen.InvoiceProviderAccount) domain.ProviderAccount {
	return domain.ProviderAccount{
		ID:                   row.ID,
		IssuerID:             row.IssuerID,
		Provider:             row.Provider,
		ProviderTaxID:        row.ProviderTaxID,
		CredentialKeyVersion: row.CredentialKeyVersion,
		CredentialLast4:      row.CredentialLast4,
		Status:               row.Status,
		StatusReason:         row.StatusReason,
		VerifiedAt:           row.VerifiedAt,
		AutomationCutoverAt:  row.AutomationCutoverAt,
		LastHealthCheckedAt:  row.LastHealthCheckedAt,
		HealthCheckNextAt:    row.HealthCheckNextAt.Time,
		BreakerOpenedAt:      row.BreakerOpenedAt,
		BreakerProbeSuccess:  row.BreakerProbeSuccess,
		HasPendingCredential: row.PendingCiphertext != nil,
		PendingLast4:         row.PendingLast4,
		RotationStartedAt:    row.RotationStartedAt,
		Version:              row.Version,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
		CredentialUpdatedAt:  row.CredentialUpdatedAt,
	}
}

func toProviderAccounts(rows []gen.InvoiceProviderAccount) []domain.ProviderAccount {
	out := make([]domain.ProviderAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProviderAccount(row))
	}
	return out
}

// ============================================================================
// 參數轉換小工具
// ============================================================================

// ts 把 time.Time 轉成 pgtype.Timestamptz 參數。
func ts(t time.Time) pgtype.Timestamptz { return pgxutil.TimestamptzParam(t) }

// tp 取 time.Time 的位址；sqlc 對 nullable timestamptz 產出的參數型別是 *time.Time，
// 而多數呼叫端手上是值。分開一個函式是因為 &t 對迴圈變數以外的表達式不合法。
func tp(t time.Time) *time.Time { return &t }

// sp 取 string 的位址（nullable text 參數）。
func sp(s string) *string { return &s }

// dateParam 把 UTC 時間轉成 pgtype.Date。呼叫端負責先換算到 Asia/Taipei——
// 本層不做時區換算（那是 domain/period.go 的職責）。
func dateParam(t time.Time) pgtype.Date { return pgxutil.DateParam(&t) }
func toIssuer(row gen.InvoiceIssuer) domain.Issuer {
	return domain.Issuer{
		ID:          row.ID,
		IssuerType:  row.IssuerType,
		IssuerRefID: row.IssuerRefID,
		DisplayName: row.DisplayName,
		Status:      row.Status,
		RetiredAt:   row.RetiredAt,
		Version:     row.Version,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

func toSellerTaxProfile(row gen.SellerTaxProfile) domain.SellerTaxProfile {
	return domain.SellerTaxProfile{
		ID:                          row.ID,
		IssuerID:                    row.IssuerID,
		ReviewStatus:                row.ReviewStatus,
		ClaimedObligation:           row.ClaimedObligation,
		EntityTaxType:               row.EntityTaxType,
		LegalName:                   row.LegalName,
		TaxID:                       row.TaxID,
		ResponsiblePerson:           row.ResponsiblePerson,
		BusinessAddress:             row.BusinessAddress,
		AttestedByAccountID:         row.AttestedByAccountID,
		AddressCity:                 row.AddressCity,
		AddressDistrict:             row.AddressDistrict,
		DefaultTaxCategory:          row.DefaultTaxCategory,
		EvidenceObjectKeys:          pgxutil.JSONRawMessage(row.EvidenceObjectKeys),
		AttestationVersion:          row.AttestationVersion,
		AttestedAt:                  row.AttestedAt,
		SubmittedAt:                 row.SubmittedAt,
		PendingValidUntil:           row.PendingValidUntil,
		ExtensionCount:              row.ExtensionCount,
		PendingOrderLimitCount:      row.PendingOrderLimitCount,
		PendingOrderLimitAmount:     row.PendingOrderLimitAmount,
		PendingOrderCount:           row.PendingOrderCount,
		PendingOrderAmount:          row.PendingOrderAmount,
		ReviewNote:                  row.ReviewNote,
		ReviewNoteAt:                row.ReviewNoteAt,
		ApprovedAt:                  row.ApprovedAt,
		CooldownUntil:               row.CooldownUntil,
		ApprovalValidUntil:          row.ApprovalValidUntil,
		RecheckReason:               row.RecheckReason,
		ReceiptAuthorizedAt:         row.ReceiptAuthorizedAt,
		ReceiptConsentVersion:       row.ReceiptConsentVersion,
		ReceiptApprovalBasisKey:     row.ReceiptApprovalBasisKey,
		ReceiptApprovalBasisAssetID: row.ReceiptApprovalBasisAssetID,
		ViolationWindowDays:         row.ViolationWindowDays,
		ViolationThreshold:          row.ViolationThreshold,
		SuspensionDays:              row.SuspensionDays,
		Version:                     row.Version,
		CreatedAt:                   row.CreatedAt.Time,
		UpdatedAt:                   row.UpdatedAt.Time,
	}
}

func toSellerTaxProfiles(rows []gen.SellerTaxProfile) []domain.SellerTaxProfile {
	out := make([]domain.SellerTaxProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSellerTaxProfile(row))
	}
	return out
}

// toDocument 另外守一件 SQL 守不住的事：表頭 tax_type 的值域。
//
// chk_invdoc_tax_type 允許 1/2/3/9，而行級的 chk_invline_tax_category 只允許 1/2/3
// （同一個 Go 型別承載兩種值域是刻意的，代價是「行級不得為 9」只能由這裡守）。
// 🔴 值域外一律硬錯，不 fallback 成 1（應稅）——那會把一張免稅發票靜默開成應稅（§C.10 ③）。
func toDocument(row gen.InvoiceDocument) (domain.Document, error) {
	if !domain.TaxCategory(row.TaxType).IsHeaderLevel() {
		return domain.Document{}, fmt.Errorf("invoice_documents.tax_type out of range: %d (document_id=%d)", row.TaxType, row.ID)
	}

	return domain.Document{
		ID:                          row.ID,
		IssuerID:                    row.IssuerID,
		IssuerNameSnapshot:          row.IssuerNameSnapshot,
		IssuerTaxIDSnapshot:         row.IssuerTaxIDSnapshot,
		SourceType:                  row.SourceType,
		SourceRef:                   row.SourceRef,
		TriggerType:                 row.TriggerType,
		RecipientType:               row.RecipientType,
		RecipientRefID:              row.RecipientRefID,
		RecipientName:               row.RecipientName,
		RecipientTaxID:              row.RecipientTaxID,
		RecipientEmailCiphertext:    row.RecipientEmailCiphertext,
		RecipientEmailHash:          row.RecipientEmailHash,
		RecipientPhoneCiphertext:    row.RecipientPhoneCiphertext,
		CarrierType:                 row.CarrierType,
		CarrierId1Ciphertext:        row.CarrierId1Ciphertext,
		CarrierId2Ciphertext:        row.CarrierId2Ciphertext,
		CarrierValueHash:            row.CarrierValueHash,
		DonationCode:                row.DonationCode,
		DocumentKind:                row.DocumentKind,
		FulfillmentMode:             row.FulfillmentMode,
		Status:                      row.Status,
		NotApplicableReason:         row.NotApplicableReason,
		ManualReasonCode:            row.ManualReasonCode,
		CancelReasonCode:            row.CancelReasonCode,
		CancelReason:                row.CancelReason,
		SnapshotVersion:             row.SnapshotVersion,
		IssueAttemptSeq:             row.IssueAttemptSeq,
		ProviderAccountVersion:      row.ProviderAccountVersion,
		SupersededByID:              row.SupersededByID,
		OriginalDocumentID:          row.OriginalDocumentID,
		SalesAmount:                 row.SalesAmount,
		FreeTaxSalesAmount:          row.FreeTaxSalesAmount,
		ZeroTaxSalesAmount:          row.ZeroTaxSalesAmount,
		TaxAmount:                   row.TaxAmount,
		TotalAmount:                 row.TotalAmount,
		TaxType:                     row.TaxType,
		TaxRateBps:                  row.TaxRateBps,
		AllowanceTotalAmount:        row.AllowanceTotalAmount,
		VoidedAt:                    row.VoidedAt,
		Provider:                    row.Provider,
		ProviderRequestNo:           row.ProviderRequestNo,
		ProviderInvoiceNo:           row.ProviderInvoiceNo,
		ProviderRandomCode:          row.ProviderRandomCode,
		ProviderIssuedAt:            row.ProviderIssuedAt,
		ExternalInvoiceNo:           row.ExternalInvoiceNo,
		ExternalInvoiceDate:         pgxutil.DateValue(row.ExternalInvoiceDate),
		ExternalReportedAt:          row.ExternalReportedAt,
		ProviderUploadStatus:        row.ProviderUploadStatus,
		ProviderUploadCheckedAt:     row.ProviderUploadCheckedAt,
		UploadRecheckNextAt:         row.UploadRecheckNextAt,
		UploadRecheckAttempts:       row.UploadRecheckAttempts,
		ReceiptNo:                   row.ReceiptNo,
		ReceiptAuthorizationVersion: row.ReceiptAuthorizationVersion,
		PdfObjectKey:                row.PdfObjectKey,
		PdfFetchNextAt:              row.PdfFetchNextAt,
		PdfFetchAttempts:            row.PdfFetchAttempts,
		InvoiceDateTaipei:           pgxutil.DateValue(row.InvoiceDateTaipei),
		PaidAt:                      row.PaidAt,
		TaxPeriodCode:               row.TaxPeriodCode,
		IssueNotBefore:              row.IssueNotBefore.Time,
		StatutoryDeadlineAt:         row.StatutoryDeadlineAt,
		SourceSnapshot:              pgxutil.JSONRawMessage(row.SourceSnapshot),
		NotificationResentAt:        row.NotificationResentAt,
		NotificationResendCount:     row.NotificationResendCount,
		Version:                     row.Version,
		CreatedAt:                   row.CreatedAt.Time,
		UpdatedAt:                   row.UpdatedAt.Time,
	}, nil
}

func toDocuments(rows []gen.InvoiceDocument) ([]domain.Document, error) {
	out := make([]domain.Document, 0, len(rows))
	for _, row := range rows {
		v, err := toDocument(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// toDocumentLine 在讀路徑上驗行級課稅別：值必須有對應的 CodeMeta 條目。
//
// 讀回來要 strconv.Itoa 再 FromValue（domain.TaxCategoryFromDB 已封裝），拿 Code 去
// FromValue 必然 ok=false（§C.10 ②）。失敗一律 fail-closed，呼叫端據此轉 AWAITING_MANUAL
// ＋ manual_reason_code='MIXED_TAX_CATEGORY'，**不得**在這裡補一個預設值。
func toDocumentLine(row gen.InvoiceDocumentLine) (domain.DocumentLine, error) {
	if _, ok := domain.TaxCategoryFromDB(row.TaxCategory); !ok {
		return domain.DocumentLine{}, fmt.Errorf("invoice_document_lines.tax_category has no CodeMeta entry: %d (line_id=%d)", row.TaxCategory, row.ID)
	}

	return domain.DocumentLine{
		ID:                row.ID,
		DocumentID:        row.DocumentID,
		LineSeq:           row.LineSeq,
		LineType:          row.LineType,
		OrderItemID:       row.OrderItemID,
		SkuID:             row.SkuID,
		Description:       row.Description,
		Unit:              row.Unit,
		Quantity:          row.Quantity,
		UnitPrice:         row.UnitPrice,
		NetAmount:         row.NetAmount,
		TaxCategory:       row.TaxCategory,
		TaxRateBps:        row.TaxRateBps,
		Remark:            row.Remark,
		AllocatedDiscount: row.AllocatedDiscount,
		AllowanceableBase: row.AllowanceableBase,
		AllowancedAmount:  row.AllowancedAmount,
		CreatedAt:         row.CreatedAt.Time,
	}, nil
}

func toDocumentLines(rows []gen.InvoiceDocumentLine) ([]domain.DocumentLine, error) {
	out := make([]domain.DocumentLine, 0, len(rows))
	for _, row := range rows {
		v, err := toDocumentLine(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func toOperation(row gen.InvoiceOperation) domain.Operation {
	return domain.Operation{
		ID:                  row.ID,
		DocumentID:          row.DocumentID,
		AdjustmentID:        row.AdjustmentID,
		OperationType:       row.OperationType,
		OperationKey:        row.OperationKey,
		ProviderRequestNo:   row.ProviderRequestNo,
		Status:              row.Status,
		RequestSnapshotHash: row.RequestSnapshotHash,
		AttemptCount:        row.AttemptCount,
		MaxAttempts:         row.MaxAttempts,
		NextAttemptAt:       row.NextAttemptAt.Time,
		HeldUntil:           row.HeldUntil,
		RecheckListPage:     row.RecheckListPage,
		StatutoryDeadlineAt: row.StatutoryDeadlineAt,
		ProviderCode:        row.ProviderCode,
		ProviderMessage:     row.ProviderMessage,
		UnknownResult:       row.UnknownResult,
		UnknownSince:        row.UnknownSince,
		CancelRequestedBy:   row.CancelRequestedBy,
		CancelRequestedAt:   row.CancelRequestedAt,
		CancelConfirmedBy:   row.CancelConfirmedBy,
		CancelConfirmedAt:   row.CancelConfirmedAt,
		CancelReasonCode:    row.CancelReasonCode,
		CancelReason:        row.CancelReason,
		StartedAt:           row.StartedAt,
		CompletedAt:         row.CompletedAt,
		LastError:           row.LastError,
		Version:             row.Version,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}

func toOperations(rows []gen.InvoiceOperation) []domain.Operation {
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOperation(row))
	}
	return out
}

func toAdjustment(row gen.InvoiceAdjustment) domain.Adjustment {
	return domain.Adjustment{
		ID:                      row.ID,
		DocumentID:              row.DocumentID,
		AdjustmentType:          row.AdjustmentType,
		Status:                  row.Status,
		TriggerKind:             row.TriggerKind,
		RefundNo:                row.RefundNo,
		CorrectionRequestID:     row.CorrectionRequestID,
		AllowanceNumber:         row.AllowanceNumber,
		AllowanceDateTaipei:     pgxutil.DateValue(row.AllowanceDateTaipei),
		NetAmountExTax:          row.NetAmountExTax,
		TaxAmount:               row.TaxAmount,
		GrossAmount:             row.GrossAmount,
		VoidReasonCode:          row.VoidReasonCode,
		VoidReasonDetail:        row.VoidReasonDetail,
		BlockedReasonCode:       row.BlockedReasonCode,
		CancelReasonCode:        row.CancelReasonCode,
		ProviderConfirmedAt:     row.ProviderConfirmedAt,
		ProviderUploadStatus:    row.ProviderUploadStatus,
		ProviderUploadCheckedAt: row.ProviderUploadCheckedAt,
		PdfObjectKey:            row.PdfObjectKey,
		Version:                 row.Version,
		CreatedAt:               row.CreatedAt.Time,
		UpdatedAt:               row.UpdatedAt.Time,
	}
}

func toAdjustments(rows []gen.InvoiceAdjustment) []domain.Adjustment {
	out := make([]domain.Adjustment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdjustment(row))
	}
	return out
}

// toAdjustmentLine 與 toDocumentLine 同一條規則：折讓明細的課稅別值域也只有 1/2/3。
func toAdjustmentLine(row gen.InvoiceAdjustmentLine) (domain.AdjustmentLine, error) {
	if _, ok := domain.TaxCategoryFromDB(row.TaxCategory); !ok {
		return domain.AdjustmentLine{}, fmt.Errorf("invoice_adjustment_lines.tax_category has no CodeMeta entry: %d (line_id=%d)", row.TaxCategory, row.ID)
	}

	return domain.AdjustmentLine{
		ID:                  row.ID,
		AdjustmentID:        row.AdjustmentID,
		LineSeq:             row.LineSeq,
		SourceLineID:        row.SourceLineID,
		OriginalInvoiceNo:   row.OriginalInvoiceNo,
		OriginalInvoiceDate: pgxutil.DateValue(row.OriginalInvoiceDate),
		OriginalDescription: row.OriginalDescription,
		Quantity:            row.Quantity,
		UnitPriceExTax:      row.UnitPriceExTax,
		NetAmountExTax:      row.NetAmountExTax,
		TaxAmount:           row.TaxAmount,
		TaxCategory:         row.TaxCategory,
		CreatedAt:           row.CreatedAt.Time,
	}, nil
}

func toAdjustmentLines(rows []gen.InvoiceAdjustmentLine) ([]domain.AdjustmentLine, error) {
	out := make([]domain.AdjustmentLine, 0, len(rows))
	for _, row := range rows {
		v, err := toAdjustmentLine(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func toComplaint(row gen.InvoiceComplaint) domain.Complaint {
	return domain.Complaint{
		ID:                 row.ID,
		DocumentID:         row.DocumentID,
		IssuerID:           row.IssuerID,
		OrderNo:            row.OrderNo,
		ComplainantBuyerID: row.ComplainantBuyerID,
		ReasonCode:         row.ReasonCode,
		Description:        row.Description,
		Status:             row.Status,
		SlaDueAt:           row.SlaDueAt.Time,
		OverdueAt:          row.OverdueAt,
		BuyerReviewDueAt:   row.BuyerReviewDueAt,
		FirstRespondedAt:   row.FirstRespondedAt,
		ResolvedAt:         row.ResolvedAt,
		ResolutionCode:     row.ResolutionCode,
		ResolutionNote:     row.ResolutionNote,
		ReopenCount:        row.ReopenCount,
		ReopenAllowedAfter: row.ReopenAllowedAfter,
		DecidedByAccountID: row.DecidedByAccountID,
		DecidedAt:          row.DecidedAt,
		Version:            row.Version,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func toComplaints(rows []gen.InvoiceComplaint) []domain.Complaint {
	out := make([]domain.Complaint, 0, len(rows))
	for _, row := range rows {
		out = append(out, toComplaint(row))
	}
	return out
}

func toComplaintEvent(row gen.InvoiceComplaintEvent) domain.ComplaintEvent {
	return domain.ComplaintEvent{
		ID:                   row.ID,
		ComplaintID:          row.ComplaintID,
		EventType:            row.EventType,
		Action:               row.Action,
		ActorType:            row.ActorType,
		ActorAccountID:       row.ActorAccountID,
		Message:              row.Message,
		AttachmentObjectKeys: pgxutil.JSONRawMessage(row.AttachmentObjectKeys),
		OccurredAt:           row.OccurredAt.Time,
	}
}

func toComplaintEvents(rows []gen.InvoiceComplaintEvent) []domain.ComplaintEvent {
	out := make([]domain.ComplaintEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toComplaintEvent(row))
	}
	return out
}

func toCorrectionRequest(row gen.InvoiceCorrectionRequest) domain.CorrectionRequest {
	return domain.CorrectionRequest{
		ID:                      row.ID,
		DocumentID:              row.DocumentID,
		RequestedByBuyerID:      row.RequestedByBuyerID,
		ExpectedSnapshotVersion: row.ExpectedSnapshotVersion,
		BeforePayload:           pgxutil.JSONRawMessage(row.BeforePayload),
		AfterPayload:            pgxutil.JSONRawMessage(row.AfterPayload),
		Status:                  row.Status,
		HandlingMode:            row.HandlingMode,
		RejectReasonCode:        row.RejectReasonCode,
		RejectReasonDetail:      row.RejectReasonDetail,
		VoidAdjustmentID:        row.VoidAdjustmentID,
		ReissuedDocumentID:      row.ReissuedDocumentID,
		Version:                 row.Version,
		CreatedAt:               row.CreatedAt.Time,
		UpdatedAt:               row.UpdatedAt.Time,
		CompletedAt:             row.CompletedAt,
	}
}

func toBuyerPreference(row gen.BuyerInvoicePreference) domain.BuyerPreference {
	return domain.BuyerPreference{
		ID:                     row.ID,
		BuyerID:                row.BuyerID,
		PreferenceKind:         row.PreferenceKind,
		CarrierType:            row.CarrierType,
		CarrierValueCiphertext: row.CarrierValueCiphertext,
		CarrierValueHash:       row.CarrierValueHash,
		CarrierValueMask:       row.CarrierValueMask,
		TaxID:                  row.TaxID,
		TaxTitle:               row.TaxTitle,
		DonationCode:           row.DonationCode,
		Label:                  row.Label,
		IsDefault:              row.IsDefault,
		SupersededAt:           row.SupersededAt,
		SupersededByID:         row.SupersededByID,
		DeletedAt:              row.DeletedAt,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}
}

func toInboxEvent(row gen.InvoiceInboxEvent) domain.InboxEvent {
	return domain.InboxEvent{
		ID:                row.ID,
		EventID:           row.EventID,
		EventType:         row.EventType,
		RoutingKey:        row.RoutingKey,
		Exchange:          row.Exchange,
		BusinessDedupKey:  row.BusinessDedupKey,
		AggregateRef:      row.AggregateRef,
		Payload:           pgxutil.JSONRawMessage(row.Payload),
		Status:            row.Status,
		Disposition:       row.Disposition,
		DispositionReason: row.DispositionReason,
		AttemptCount:      row.AttemptCount,
		NextAttemptAt:     row.NextAttemptAt.Time,
		DeferredUntil:     row.DeferredUntil,
		LastError:         row.LastError,
		OccurredAt:        row.OccurredAt.Time,
		ReceivedAt:        row.ReceivedAt.Time,
		ProcessedAt:       row.ProcessedAt,
		PayloadPurgedAt:   row.PayloadPurgedAt,
	}
}

func toInboxEvents(rows []gen.InvoiceInboxEvent) []domain.InboxEvent {
	out := make([]domain.InboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInboxEvent(row))
	}
	return out
}

func toOutboxEvent(row gen.InvoiceOutboxEvent) domain.OutboxEvent {
	return domain.OutboxEvent{
		ID:            row.ID,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID,
		EventType:     row.EventType,
		Exchange:      row.Exchange,
		RoutingKey:    row.RoutingKey,
		Payload:       pgxutil.JSONRawMessage(row.Payload),
		Status:        row.Status,
		AttemptCount:  row.AttemptCount,
		NextAttemptAt: row.NextAttemptAt.Time,
		LastError:     row.LastError,
		CreatedAt:     row.CreatedAt.Time,
		CompletedAt:   row.CompletedAt,
	}
}

func toOutboxEvents(rows []gen.InvoiceOutboxEvent) []domain.OutboxEvent {
	out := make([]domain.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOutboxEvent(row))
	}
	return out
}

func toStatusHistoryEntry(row gen.InvoiceStatusHistory) domain.StatusHistoryEntry {
	return domain.StatusHistoryEntry{
		ID:             row.ID,
		SubjectType:    row.SubjectType,
		SubjectID:      row.SubjectID,
		FromStatus:     row.FromStatus,
		ToStatus:       row.ToStatus,
		ReasonCode:     row.ReasonCode,
		ReasonDetail:   row.ReasonDetail,
		ActorType:      row.ActorType,
		ActorAccountID: row.ActorAccountID,
		CorrelationID:  row.CorrelationID,
		OccurredAt:     row.OccurredAt.Time,
	}
}

func toStatusHistoryEntrys(rows []gen.InvoiceStatusHistory) []domain.StatusHistoryEntry {
	out := make([]domain.StatusHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStatusHistoryEntry(row))
	}
	return out
}

func toProviderAuditLogEntry(row gen.InvoiceProviderAuditLog) domain.ProviderAuditLogEntry {
	return domain.ProviderAuditLogEntry{
		ID:              row.ID,
		OperationID:     row.OperationID,
		IssuerID:        row.IssuerID,
		Provider:        row.Provider,
		Endpoint:        row.Endpoint,
		AttemptSeq:      row.AttemptSeq,
		RequestMasked:   pgxutil.JSONRawMessage(row.RequestMasked),
		ResponseMasked:  pgxutil.JSONRawMessage(row.ResponseMasked),
		RawBodyHash:     row.RawBodyHash,
		HttpStatus:      row.HttpStatus,
		ProviderCode:    row.ProviderCode,
		ProviderMessage: row.ProviderMessage,
		LatencyMs:       row.LatencyMs,
		ErrorKind:       row.ErrorKind,
		CorrelationID:   row.CorrelationID,
		OccurredAt:      row.OccurredAt.Time,
	}
}

func toProviderAuditLogEntrys(rows []gen.InvoiceProviderAuditLog) []domain.ProviderAuditLogEntry {
	out := make([]domain.ProviderAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProviderAuditLogEntry(row))
	}
	return out
}

func toAdminAuditLogEntry(row gen.InvoiceAdminAuditLog) domain.AdminAuditLogEntry {
	return domain.AdminAuditLogEntry{
		ID:                     row.ID,
		ActorAccountID:         row.ActorAccountID,
		Action:                 row.Action,
		ResourceType:           row.ResourceType,
		ResourceID:             row.ResourceID,
		BeforeState:            pgxutil.JSONRawMessage(row.BeforeState),
		AfterState:             pgxutil.JSONRawMessage(row.AfterState),
		ReasonCode:             row.ReasonCode,
		ReasonDetail:           row.ReasonDetail,
		ApprovalActorAccountID: row.ApprovalActorAccountID,
		TraceID:                row.TraceID,
		RequestID:              row.RequestID,
		CorrelationID:          row.CorrelationID,
		ClientIp:               row.ClientIp,
		UserAgent:              row.UserAgent,
		OccurredAt:             row.OccurredAt.Time,
	}
}

func toAdminAuditLogEntrys(rows []gen.InvoiceAdminAuditLog) []domain.AdminAuditLogEntry {
	out := make([]domain.AdminAuditLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdminAuditLogEntry(row))
	}
	return out
}

// ============================================================================
// 收件人明文 ↔ 密文（invoice_documents 的四個 recipient/carrier 欄）
//
// 與 AppKey 的差別只有一個，但它決定了實作形狀：invoice_documents **沒有**
// key_version 欄（AppKey 有 credential_key_version）⇒ 解密只能「先試 current、
// 失敗再試 prev」。加密一律用 current，與 deriveCredential 同一個約定。
// ============================================================================

// encryptOptional 把明文加密；空字串回 nil（＝ DB 的 NULL）。
//
// ""→NULL 不是便利而是語意：chk_invdoc_carrier_pair 與 chk_invdoc_exclusive 都以
// IS NULL 判斷，寫進一個「加密後的空字串」會讓那兩條 CHECK 認為載具存在。
func (k *KeyRing) encryptOptional(plaintext string) (*string, error) {
	if plaintext == "" {
		return nil, nil
	}
	ct, err := encryptCredential(k.Current, plaintext)
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

// hashOptional 算 HMAC-SHA256 lookup hash；空字串回 nil。
func (k *KeyRing) hashOptional(plaintext string) *string {
	if plaintext == "" {
		return nil
	}
	h := crypto.HMACLookup(k.LookupHMACKey, plaintext)
	return &h
}

// decryptOptional 解一個 nullable 密文欄。NULL 回空字串。
//
// 先試 current 再試 prev：本表沒有 key_version 欄，而 master key 輪替期間同一張表裡
// 會同時存在兩版密文。兩把都失敗才是真的錯 —— 靜默回空字串會讓一張帶載具的發票被當成
// 沒有載具送出去，而 AMEGO 會照常回 code 0。
func (k *KeyRing) decryptOptional(stored *string) (string, error) {
	if stored == nil || *stored == "" {
		return "", nil
	}
	plain, err := decryptCredential(k.Current, *stored)
	if err == nil {
		return plain, nil
	}
	if k.Prev == nil {
		return "", err
	}
	plain, prevErr := decryptCredential(k.Prev, *stored)
	if prevErr != nil {
		return "", fmt.Errorf("invoice: decrypt with current and prev master key both failed: %w", err)
	}
	return plain, nil
}

// openRecipient 解出一列文件的全部收件人明文。只有 WithDecryptedRecipient 會呼叫它。
func (k *KeyRing) openRecipient(row gen.InvoiceDocument) (domain.RecipientSecrets, error) {
	var out domain.RecipientSecrets
	var err error
	if out.Email, err = k.decryptOptional(row.RecipientEmailCiphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.Phone, err = k.decryptOptional(row.RecipientPhoneCiphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.CarrierID1, err = k.decryptOptional(row.CarrierId1Ciphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	if out.CarrierID2, err = k.decryptOptional(row.CarrierId2Ciphertext); err != nil {
		return domain.RecipientSecrets{}, err
	}
	return out, nil
}

// snp 是 sp 的 nullable 版：空字串回 nil（＝ NULL），非空回位址。
//
// 分開一個函式而不是讓 sp 自己判斷：既有呼叫端傳的都是「一定要寫進去」的值，
// 讓 sp 悄悄把 "" 變成 NULL 會改變那些路徑的行為。
// nzp 把 0 折成 NULL：0 不是合法的 account_id，而「沒有簽署人」與「簽署人是 0」
// 在 DB 上必須分得開——後者會讓稽核以為有一個 id 為 0 的自然人簽了字。
func nzp(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func snp(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// inp 是 int64 的 nullable 版：0 回 nil。用於 order_item_id／sku_id／adjustment_id
// 這類「0 不是合法業務值」的 FK 欄。
func inp(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
