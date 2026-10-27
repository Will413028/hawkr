//go:build integration

// 本檔跑真的 SQL。它存在的理由只有一個：**Tier A 的擁有者述詞住在 SQL 的 WHERE 裡，
// 而任何 fake repository 都會忠實地照著述詞回答**——把 `AND buyer_id = $2` 從 query 拿掉，
// 單元測試的 fake 照樣正確、照樣全綠，只有真的 PostgreSQL 會把別人的那一列交出來。
//
// 同一條理由適用於 §8.1.3(c) 的 PII 揭露稽核：fake 記得下 append，但「那一列真的寫進
// invoice_admin_audit_logs 了嗎」只有真 DB 答得出來（reason_code／reason_detail 都是
// NOT NULL，少填一個就是寫入失敗而不是少一欄）。
//
// 跑法：`make test-integration`（需要本機 Docker daemon）。harness（pg／itPool／itKeyRing）
// 與 integration_test.go 共用，見該檔檔頭。
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
)

const (
	qitBuyerA = int64(2623900520)
	qitBuyerB = int64(2688637900)

	// 這兩個值刻意寫成一眼看得出的字串：任何一個出現在回應裡都是明碼外洩。
	qitCarrierPlain = "/T7R3U+P"
	qitEmailPlain   = "buyer-qit@example.com"
)

type qitEnv struct {
	pool     *pgxpool.Pool
	query    *Query
	docs     *postgres.DocumentRepository
	issuerID int64
	storeID  int64
}

// qitSetup 建一位賣家 issuer ＋ 稅務資格 ＋ 一份已開立的發票文件，並組出真 repo 的 Query。
func qitSetup(t *testing.T) *qitEnv {
	t.Helper()
	ctx := context.Background()
	pool := itPool(t)
	ring := itKeyRing(t)

	seq := time.Now().UnixNano()
	issuerID := seq
	storeID := seq%1_000_000_000 + 1

	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1,'SELLER',$2,'查詢整合測試賣場','active')`, issuerID, storeID); err != nil {
		t.Fatalf("seed issuer: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO seller_tax_profiles
		   (id, issuer_id, review_status, claimed_obligation, default_tax_category, legal_name, tax_id)
		 VALUES ($1,$2,'APPROVED_INVOICE_REQUIRED','REQUIRED',1,'查詢整合測試股份有限公司','12345678')`,
		issuerID+1, issuerID); err != nil {
		t.Fatalf("seed tax profile: %v", err)
	}

	docs := postgres.NewDocumentRepository(pool, ring)
	ops := postgres.NewOperationRepository(pool)
	audit := postgres.NewAuditRepository(pool)
	prefs := postgres.NewBuyerPreferenceRepository(pool, ring)
	complaints := postgres.NewComplaintRepository(pool)

	q := NewQuery(
		postgres.NewIssuerRepository(pool),
		postgres.NewSellerTaxProfileRepository(pool),
		postgres.NewProviderAccountRepository(pool, ring),
		prefs, docs, ops, complaints, audit,
		postgres.NewAdjustmentRepository(pool),
		postgres.NewCorrectionRequestRepository(pool),
		postgres.NewOverviewRepository(pool),
		postgres.NewBusinessDayRepository(pool),
		postgres.NewProviderCutoverRepository(pool),
		postgres.NewManualUnlockRepository(pool),
		postgres.NewCancelledOrderRepository(pool),
		IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	)
	return &qitEnv{pool: pool, query: q, docs: docs, issuerID: issuerID, storeID: storeID}
}

// qitSeedDocument 建一份帶載具與 email 的已開立發票（明文由 repository 加密落庫）。
func qitSeedDocument(t *testing.T, env *qitEnv, docID int64, buyerID int64, orderNo string) domain.Document {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(48 * time.Hour)
	taipei := time.Now().UTC()
	doc, inserted, err := env.docs.Create(ctx, domain.NewDocument{
		ID: docID, IssuerID: env.issuerID, IssuerNameSnapshot: "查詢整合測試賣場",
		SourceType: "ORDER", SourceRef: orderNo, TriggerType: "AUTO", // chk_invdoc_trigger 只認 AUTO／ADM_MANUAL
		RecipientType: "BUYER", RecipientRefID: &buyerID,
		RecipientName: "測試買家",
		// B2C 的 chk_invdoc_b2c 要求買受人統編恆為 10 個 0（AMEGO 官方形狀），
		// 空字串會被 chk_invdoc_taxid_shape 擋下。
		RecipientTaxID: "0000000000",
		RecipientEmail: qitEmailPlain,
		CarrierID1:     qitCarrierPlain,
		CarrierType:    "3J0002",
		DocumentKind:   "B2C", FulfillmentMode: "AMEGO_AUTO", Status: "PENDING",
		SnapshotVersion: 1, IssueAttemptSeq: 1, // chk_invdoc_versions：兩者都必須 >= 1
		SalesAmount: 100, TaxAmount: 0, TotalAmount: 100, TaxType: 1, TaxRateBps: 500,
		Provider: "AMEGO", ProviderRequestNo: fmt.Sprintf("R%d", docID),
		InvoiceDateTaipei: &taipei, TaxPeriodCode: "11507",
		IssueNotBefore:      time.Now().UTC(),
		StatutoryDeadlineAt: &deadline,
		SourceSnapshot:      json.RawMessage(`{"orderNo":"` + orderNo + `"}`),
		Now:                 time.Now().UTC(),
	})
	if err != nil || !inserted {
		t.Fatalf("seed document: err=%v inserted=%v", err, inserted)
	}
	return doc
}

// ---------------------------------------------------------------------------
// Tier A：擁有者述詞必須在 SQL 裡
// ---------------------------------------------------------------------------

// TestIntegration_Query_TierA_ComplaintOwnerIsInSQLPredicate 是「把擁有者條件從 WHERE
// 拿掉」這個變異的唯一偵測點。
//
// 判準有兩層，缺一都會讓變異溜過去：
//
//	① 別人的申訴**查不到**（若 complainant_buyer_id 不在 WHERE，這裡會拿到那一列）
//	② 查不到與不存在回**完全相同**的業務碼與訊息（§8.1.4(c)）
func TestIntegration_Query_TierA_ComplaintOwnerIsInSQLPredicate(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	doc := qitSeedDocument(t, env, env.issuerID+100, qitBuyerA, "O000000101")

	complaintID := env.issuerID + 200
	if _, err := env.pool.Exec(ctx,
		`INSERT INTO invoice_complaints
		   (id, document_id, issuer_id, order_no, complainant_buyer_id, reason_code, description, status, sla_due_at)
		 VALUES ($1,$2,$3,$4,$5,'NOT_RECEIVED','沒收到發票','OPEN', NOW() + interval '3 days')`,
		complaintID, doc.ID, env.issuerID, "O000000101", qitBuyerA); err != nil {
		t.Fatalf("seed complaint: %v", err)
	}

	// 擁有者本人讀得到。
	if _, err := env.query.GetBuyerComplaint(ctx, qitBuyerA, "O000000101", complaintID); err != nil {
		t.Fatalf("擁有者應該讀得到自己的申訴：%v", err)
	}

	// (a) 另一位買家讀同一件申訴 —— 這一列**存在**，只是不屬於他。
	_, foreignErr := env.query.GetBuyerComplaint(ctx, qitBuyerB, "O000000101", complaintID)
	if foreignErr == nil {
		t.Fatal("🔴 越權讀取成功了 —— complainant_buyer_id 沒有進 SQL 的 WHERE（§8.1.4(b)）")
	}
	// (b) 同一位買家讀一個不存在的 id。
	_, missingErr := env.query.GetBuyerComplaint(ctx, qitBuyerB, "O000000101", complaintID+9999)
	if missingErr == nil {
		t.Fatal("不存在的 complaint_id 應該回錯誤")
	}

	assertSameDomainError(t, foreignErr, missingErr,
		"越權與不存在必須回同一個碼與同一段訊息（§8.1.4(c)：由同一行程式碼產生）")
	if !isBizCode(foreignErr, bizcode.InvoiceComplaintNotFound) {
		t.Fatalf("預期業務碼 %d，實得 %v", bizcode.InvoiceComplaintNotFound, foreignErr)
	}
}

// TestIntegration_Query_TierA_BuyerProfilesOwnerIsInSQLPredicate 同上，列表版。
//
// 順帶釘住「明碼不出服務邊界」：display_value 取的是 carrier_value_mask，
// 而 carrier_value_ciphertext 是真的密文（本測試自己加密後落庫）。
// 把 mapper 改成解密回明碼，第二段斷言當場紅。
func TestIntegration_Query_TierA_BuyerProfilesOwnerIsInSQLPredicate(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)

	cipher, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	blob, err := cipher.EncryptToBase64(qitCarrierPlain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	prefID := env.issuerID + 300
	if _, err := env.pool.Exec(ctx,
		`INSERT INTO buyer_invoice_preferences
		   (id, buyer_id, preference_kind, carrier_type, carrier_value_ciphertext,
		    carrier_value_hash, carrier_value_mask, is_default)
		 VALUES ($1,$2,'MOBILE_BARCODE','3J0002',$3,'deadbeef','/T••••+P',true)`,
		prefID, qitBuyerA, blob); err != nil {
		t.Fatalf("seed preference: %v", err)
	}

	mine, err := env.query.ListBuyerProfiles(ctx, qitBuyerA, 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mine.Total != 1 || len(mine.Records) != 1 {
		t.Fatalf("擁有者應該看到 1 筆，實得 total=%d len=%d", mine.Total, len(mine.Records))
	}

	// 🔴 明碼不出服務邊界：本路徑不解密。
	if got := mine.Records[0].DisplayValue; got != "/T••••+P" {
		t.Fatalf("display_value 應為已落庫的遮罩值，實得 %q", got)
	}
	if strings.Contains(fmt.Sprintf("%+v", mine), qitCarrierPlain) {
		t.Fatalf("🔴 載具明碼出現在回應裡：%+v", mine)
	}

	other, err := env.query.ListBuyerProfiles(ctx, qitBuyerB, 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if other.Total != 0 || len(other.Records) != 0 {
		t.Fatalf("🔴 另一位買家看到了 %d 筆 —— buyer_id 沒有進 SQL 的 WHERE", other.Total)
	}
}

// ---------------------------------------------------------------------------
// §8.1.3(c) 裁定 2：ADM 不遮罩，但每次揭露留一列稽核
// ---------------------------------------------------------------------------

// TestIntegration_Query_AdminDocumentDetail_RevealsPlaintextAndWritesAuditRow 驗兩半。
//
// 兩半都要，而且必須是同一支測試：只驗「看得到明碼」⇒ 把稽核拿掉不會紅；
// 只驗「有稽核」⇒ 把明碼遮回去不會紅，而 ADM 就查不了申訴案。
func TestIntegration_Query_AdminDocumentDetail_RevealsPlaintextAndWritesAuditRow(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)
	doc := qitSeedDocument(t, env, env.issuerID+400, qitBuyerA, "O000000401")

	// 前提：落庫的是密文，不是明文。這一句同時證明後面看到的明碼真的來自解密。
	var stored string
	if err := env.pool.QueryRow(ctx,
		`SELECT carrier_id1_ciphertext FROM invoice_documents WHERE id=$1`, doc.ID).Scan(&stored); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if stored == qitCarrierPlain {
		t.Fatal("🔴 載具明碼直接落庫了（§5.1.5 要求密文儲存）")
	}

	const actorID = int64(90001)
	view, err := env.query.GetDocumentForAdmin(ctx, actorID, doc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 半一：ADM 端不遮罩。
	if view.RevealedCarrierID != qitCarrierPlain {
		t.Fatalf("ADM 端載具應為明碼（§8.1.3(c) 裁定 2），實得 %q", view.RevealedCarrierID)
	}
	if view.RevealedEmail != qitEmailPlain {
		t.Fatalf("ADM 端 email 應為明碼，實得 %q", view.RevealedEmail)
	}

	// 半二：稽核列真的寫進去了（reason_code／reason_detail 都是 NOT NULL，
	// 少填一個就是寫入失敗而不是少一欄 ⇒ 這裡的 COUNT 也會是 0）。
	var n int
	if err := env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoice_admin_audit_logs
		  WHERE action='INVOICE_PII_REVEAL' AND resource_type='INVOICE_DOCUMENT'
		    AND resource_id=$1 AND actor_account_id=$2`, doc.ID, actorID).Scan(&n); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if n != 1 {
		t.Fatalf("🔴 每次揭露必須留一列 INVOICE_PII_REVEAL，實得 %d 列", n)
	}

	// 🔴 第二次讀取必須再留一列：uq_invadmaudit_client_request(actor, action, request_id)
	// 只在 request_id IS NOT NULL 時生效 ⇒ 帶了 request_id 的實作會在這裡只剩 1 列，
	// 而那等於「同一位 ADM 反覆讀取只被記錄一次」。
	if _, err := env.query.GetDocumentForAdmin(ctx, actorID, doc.ID); err != nil {
		t.Fatalf("second read: %v", err)
	}
	if err := env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoice_admin_audit_logs
		  WHERE action='INVOICE_PII_REVEAL' AND resource_id=$1`, doc.ID).Scan(&n); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if n != 2 {
		t.Fatalf("🔴 第二次揭露必須再留一列（否則反覆讀取只被記錄一次），實得 %d 列", n)
	}
}

// TestIntegration_Query_SellerTaxProfile_ForeignStoreGetsSameNotFound 是賣家側的
// SQL 層 Tier A：issuer_ref_id = store_id 這條述詞在 invoice_issuers 上。
func TestIntegration_Query_SellerTaxProfile_ForeignStoreGetsSameNotFound(t *testing.T) {
	ctx := context.Background()
	env := qitSetup(t)

	if _, err := env.query.GetSellerTaxProfile(ctx, env.storeID); err != nil {
		t.Fatalf("擁有者應該讀得到自己的稅務資格：%v", err)
	}

	_, foreignErr := env.query.GetSellerTaxProfile(ctx, env.storeID+1)
	_, missingErr := env.query.GetSellerTaxProfile(ctx, env.storeID+999_999)
	if foreignErr == nil || missingErr == nil {
		t.Fatal("別的 store_id 應該一律讀不到")
	}
	assertSameDomainError(t, foreignErr, missingErr, "別的 store_id 與不存在的 store_id 必須完全相同")
	if !isBizCode(foreignErr, bizcode.SellerTaxProfileNotFound) {
		t.Fatalf("預期 %d（不是 provider 的 9330），實得 %v", bizcode.SellerTaxProfileNotFound, foreignErr)
	}
}

// assertSameDomainError 比對兩個錯誤的碼與訊息。
//
// 判準不是「都是錯誤」：越權與不存在回不同的 message（甚至只差一個字）就足以讓
// 攻擊者枚舉出「這個 id 存在」，而只斷言「有錯」的測試對此完全無感。
func assertSameDomainError(t *testing.T, a, b error, why string) {
	t.Helper()
	if a.Error() != b.Error() {
		t.Fatalf("%s\n  a = %v\n  b = %v", why, a, b)
	}
}

// 端點 16 的 `selfIssueEffective` 錨在 review_status，不是 attested_at（裁決 C）。
//
// 🔴 這一支跑真 DB，守的是 S01 那顆琥珀 pill「自行開票責任生效中」與 banner 追加句
// 「審核期間由你自行開立與交付發票。」的資料源。判準若退回 `attested_at != nil`，
// 賣家一送審就會看到那句法律告知，而同一刻後端給的是 LEGACY_MANUAL／
// TAX_PROFILE_NOT_APPROVED、待審接單上限也不適用 —— 畫面與系統行為相反，
// 且沒有任何編譯錯或執行期訊號。
//
// 陰性（SUBMITTED）才是重點：陽性那半在判準壞掉時照樣是綠的。
func TestIntegration_Query_SellerCapability_SelfIssueEffectiveAnchorsOnStatus(t *testing.T) {
	env := qitSetup(t)
	ctx := context.Background()

	// 兩個狀態都寫滿簽署記錄 —— 差別只在 review_status，
	// 這樣才分得出判準讀的是哪一欄（chk_stp_attested_when_committed 也要求它們非空）。
	setStatus := func(status string) {
		t.Helper()
		if _, err := env.pool.Exec(ctx,
			`UPDATE seller_tax_profiles
			    SET review_status = $2, submitted_at = now(), pending_valid_until = now() + interval '30 days',
			        attestation_version = $3, attested_at = now(), tax_id = NULL
			  WHERE issuer_id = $1`,
			env.issuerID, status, SelfIssueTermsVersion); err != nil {
			t.Fatalf("seed %s: %v", status, err)
		}
	}

	setStatus("SUBMITTED")
	cap1, err := env.query.GetSellerCapability(ctx, env.storeID)
	if err != nil {
		t.Fatalf("GetSellerCapability: %v", err)
	}
	if cap1.SelfIssueEffective {
		t.Fatal("SUBMITTED（已簽、尚未具結）不得回 selfIssueEffective=true —— " +
			"那會讓 S01 對賣家宣告一個後端不會執行的責任")
	}
	if cap1.IntakeLimit.Applicable {
		t.Fatal("SUBMITTED 不在待審接單窗口內，intakeLimit 不該適用（與上面同一組事實）")
	}

	setStatus("SELF_ISSUE_PENDING")
	cap2, err := env.query.GetSellerCapability(ctx, env.storeID)
	if err != nil {
		t.Fatalf("GetSellerCapability: %v", err)
	}
	if !cap2.SelfIssueEffective {
		t.Fatal("SELF_ISSUE_PENDING 就是「自行開票責任生效中」，必須回 true")
	}
	if !cap2.IntakeLimit.Applicable {
		t.Fatal("SELF_ISSUE_PENDING 在待審接單窗口內，intakeLimit 必須適用")
	}
}
