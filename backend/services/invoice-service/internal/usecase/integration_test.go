//go:build integration

// 本檔跑真的 SQL ＋ 真的 amego.HTTPClient（指向 httptest.Server）。
//
// 它存在的理由與 repository 那支相同：開票主鏈幾乎每一條規則的失效形狀都**不是編譯錯**——
// 雙向等式 CHECK、ON CONFLICT DO NOTHING 的 rows=0、CAS 的 rows=0、「outbox 與業務寫入
// 同一交易」，靜態測試一條都驗不到。用真的 client 而不是 fakeProvider，是因為簽章、
// 環境閘門與錯誤分類都在那一層，換成 fake 就等於沒測到「送出去的長什麼樣」。
//
// 跑法：`make test-integration`（需要本機 Docker daemon）。容器由 pkg/testutil/dbtest
// 起一顆 postgres:16-alpine、重放 db/migrations，每支測試拿到一個自己的 database。
//
// 🔴 刻意不用「環境變數沒設就 t.Skip」：那會讓 CI 的 job 永久綠而測試從未執行過。
// 這條與 internal/repository/postgres/integration_test.go 檔頭是同一條，兩處不得分歧。
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_usecase_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

const (
	itTaxID  = "12345678"
	itAppKey = "integration-app-key"
)

func itPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	db, _ := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func itKeyRing(t *testing.T) *postgres.KeyRing {
	t.Helper()
	cur, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return &postgres.KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "integration-lookup-key"}
}

type fakeAmego struct {
	srv      *httptest.Server
	requests atomic.Int64
	respond  func(w http.ResponseWriter)
}

func newFakeAmego(t *testing.T) *fakeAmego {
	t.Helper()
	f := &fakeAmego{}
	f.respond = func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "", "invoice_number": "AB12345678",
			"invoice_time": time.Now().Unix(), "random_number": "4321",
		})
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != amego.EndpointIssue {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		f.respond(w)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAmego) client(t *testing.T) amego.Client {
	t.Helper()
	c, err := amego.New(amego.Options{
		BaseURL:    f.srv.URL,
		HTTPClient: f.srv.Client(),
		AllowRealInvoice: false,
		TestInvoice:      itTaxID,
		RateLimitQPS:     50,
		Deadlines:        amego.DefaultDeadlines(),
	})
	if err != nil {
		t.Fatalf("amego.New: %v", err)
	}
	return c
}

type itEnv struct {
	pool     *pgxpool.Pool
	uc       *Issuance
	amego    *fakeAmego
	issuerID int64
	orderNo  string
	docs     *postgres.DocumentRepository
	ops      *postgres.OperationRepository
}

func setupEnv(t *testing.T, reviewStatus string, taxCategory int16) *itEnv {
	t.Helper()
	ctx := context.Background()
	pool := itPool(t)
	ring := itKeyRing(t)

	seq := time.Now().UnixNano()
	issuerID := seq
	storeID := seq%1_000_000_000 + 1
	orderNo := fmt.Sprintf("O%09d", seq%1_000_000_000)

	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1,'SELLER',$2,$3,'active')`, issuerID, storeID, "整合測試賣場"); err != nil {
		t.Fatalf("seed issuer: %v", err)
	}

	var taxIDArg, legalArg any = itTaxID, "整合測試股份有限公司"
	if reviewStatus == codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value {
		taxIDArg = nil
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO seller_tax_profiles
		   (id, issuer_id, review_status, claimed_obligation, default_tax_category,
		    legal_name, tax_id, approved_at)
		 VALUES ($1,$2,$3,'REQUIRED',$4,$5,$6,NOW())`,
		issuerID+1, issuerID, reviewStatus, taxCategory, legalArg, taxIDArg); err != nil {
		t.Fatalf("seed tax profile: %v", err)
	}

	accounts := postgres.NewProviderAccountRepository(pool, ring)
	if err := accounts.Create(ctx, domain.NewProviderAccount{
		ID: issuerID + 2, IssuerID: issuerID, Provider: "AMEGO",
		ProviderTaxID: itTaxID, AppKey: itAppKey,
		Status:            codemeta.InvoiceProviderAccountStatusCredentialSubmitted.Value,
		HealthCheckNextAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed provider account: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts
		    SET status='ACTIVE', verified_at=NOW() - interval '1 day',
		        automation_cutover_at=NOW() - interval '1 day'
		  WHERE id=$1`, issuerID+2); err != nil {
		t.Fatalf("activate provider account: %v", err)
	}


	fa := newFakeAmego(t)
	docs := postgres.NewDocumentRepository(pool, ring)
	ops := postgres.NewOperationRepository(pool)

	var idSeq int64
	uc := NewIssuance(
		database.NewTxManager(pool),
		postgres.NewIssuerRepository(pool),
		postgres.NewSellerTaxProfileRepository(pool),
		accounts, docs, ops,
		postgres.NewCorrectionRequestRepository(pool),
		postgres.NewOutboxRepository(pool),
		postgres.NewAuditRepository(pool),
		fa.client(t),
		&fakeReceiptRenderer{}, &fakeReceiptUploader{},
		func() int64 { idSeq++; return issuerID + 1000 + idSeq },
		IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
			IssueLease: 120 * time.Second, RecheckBase: 30 * time.Second},
		nil,
	)

	return &itEnv{pool: pool, uc: uc, amego: fa, issuerID: issuerID, orderNo: orderNo, docs: docs, ops: ops}
}

func itInput(env *itEnv) OrderPaidInput {
	return OrderPaidInput{
		OrderNo: env.orderNo, BuyerID: 2623900520,
		StoreID: mustStoreID(env), SnapshotVersion: 1, PaidAt: time.Now().UTC(),
		NotifyEmail: "buyer@example.com",
		Snapshot: domain.OrderSnapshot{
			TotalOriginalAmount: 100, TotalActualAmount: 100,
			DiscountFunding: domain.DiscountFundingSeller,
			Items:           []domain.OrderItem{{OrderItemID: 1, SkuID: "0192a3b4-0000-7000-8000-000000000011", ProductName: "有機高麗菜", Quantity: 1, UnitPrice: 100}},
		},
	}
}

func mustStoreID(env *itEnv) int64 {
	var v int64
	if err := env.pool.QueryRow(context.Background(),
		`SELECT issuer_ref_id FROM invoice_issuers WHERE id=$1`, env.issuerID).Scan(&v); err != nil {
		panic(err)
	}
	return v
}


func TestIntegration_Issuance_HappyPath(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)

	res, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if res.DocumentStatus != codemeta.InvoiceDocumentStatusPending.Value ||
		res.OperationStatus != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("Prepare 落點錯：doc=%s op=%s", res.DocumentStatus, res.OperationStatus)
	}

	var nLines int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM invoice_document_lines WHERE document_id=$1`, res.DocumentID).Scan(&nLines); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if nLines != 1 {
		t.Fatalf("明細行 = %d，want 1", nLines)
	}

	claimed, err := env.uc.ClaimForSubmit(ctx, res.OperationID)
	if err != nil || !claimed {
		t.Fatalf("ClaimForSubmit: ok=%v err=%v", claimed, err)
	}
	assertDocStatus(t, env, res.DocumentID, codemeta.InvoiceDocumentStatusIssuing.Value)

	if err := env.uc.Submit(ctx, res.OperationID); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if n := env.amego.requests.Load(); n != 1 {
		t.Fatalf("AMEGO 收到 %d 次請求，want 1", n)
	}

	assertDocStatus(t, env, res.DocumentID, codemeta.InvoiceDocumentStatusIssued.Value)
	assertOpStatus(t, env, res.OperationID, codemeta.InvoiceOperationStatusSucceeded.Value)

	var invoiceNo, randomCode string
	if err := env.pool.QueryRow(ctx,
		`SELECT provider_invoice_no, provider_random_code FROM invoice_documents WHERE id=$1`,
		res.DocumentID).Scan(&invoiceNo, &randomCode); err != nil {
		t.Fatalf("read invoice no: %v", err)
	}
	if invoiceNo != "AB12345678" || randomCode != "4321" {
		t.Errorf("號碼未回填：%q/%q", invoiceNo, randomCode)
	}

	var nOutbox int
	var routingKey string
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*), coalesce(max(routing_key),'') FROM invoice_outbox_events WHERE aggregate_id=$1`,
		res.DocumentID).Scan(&nOutbox, &routingKey); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if nOutbox != 1 || routingKey != "invoice.issued" {
		t.Fatalf("outbox = %d 列 routing_key=%q", nOutbox, routingKey)
	}
}


func TestIntegration_Issuance_OutboxIsInsideTheBusinessTransaction(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)

	res, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := env.uc.ClaimForSubmit(ctx, res.OperationID); err != nil {
		t.Fatalf("ClaimForSubmit: %v", err)
	}

	tm := database.NewTxManager(env.pool)
	sentinel := fmt.Errorf("deliberate rollback")
	err = tm.WithTx(ctx, func(ctx context.Context) error {
		if serr := env.uc.Submit(ctx, res.OperationID); serr != nil {
			return serr
		}
		return sentinel
	})
	if err == nil {
		t.Fatal("WithTx 應把 sentinel 傳出來")
	}

	var nOutbox int
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_outbox_events WHERE aggregate_id=$1`, res.DocumentID).Scan(&nOutbox); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if nOutbox != 0 {
		t.Fatalf("rollback 之後仍有 %d 列 outbox —— 事件寫入沒有跟隨業務交易", nOutbox)
	}
	assertDocStatus(t, env, res.DocumentID, codemeta.InvoiceDocumentStatusIssuing.Value)
}

// 鑑別力更強的一支：讓交易在 outbox 寫入**之後**失敗。
//
// 手法是把 document 從 ISSUING 移走 ⇒ MarkIssued 的 allowed-from 不成立 ⇒ rows=0 ⇒ 回 error
// ⇒ 整個交易回滾。事件寫入若沒有跟隨交易（用 pool 而不是 tx），這一列會留下來，
// 而那正是「發票沒開成、買家卻收到已開立通知」的資料形狀。
func TestIntegration_Issuance_OutboxRollsBackWhenDocumentTransitionFails(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)

	res, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := env.uc.ClaimForSubmit(ctx, res.OperationID); err != nil {
		t.Fatalf("ClaimForSubmit: %v", err)
	}
	if _, err := env.pool.Exec(ctx,
		`UPDATE invoice_documents SET status='PENDING' WHERE id=$1`, res.DocumentID); err != nil {
		t.Fatalf("move document out of ISSUING: %v", err)
	}

	if err := env.uc.Submit(ctx, res.OperationID); err == nil {
		t.Fatal("MarkIssued 的守衛不成立時 Submit 必須回 error，不得當成功繼續")
	}

	var nOutbox int
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_outbox_events WHERE aggregate_id=$1`, res.DocumentID).Scan(&nOutbox); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if nOutbox != 0 {
		t.Fatalf("交易回滾後仍有 %d 列 outbox —— 事件寫入沒有跟隨業務交易", nOutbox)
	}
}


func TestIntegration_Issuance_RetryableSchedulesBackoff(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)
	env.amego.respond = func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 10, "msg": "系統停機維護中"})
	}

	res, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := env.uc.ClaimForSubmit(ctx, res.OperationID); err != nil {
		t.Fatalf("ClaimForSubmit: %v", err)
	}
	before := time.Now().UTC()
	if err := env.uc.Submit(ctx, res.OperationID); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	var status string
	var attempt int32
	var nextAt time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT status, attempt_count, next_attempt_at FROM invoice_operations WHERE id=$1`,
		res.OperationID).Scan(&status, &attempt, &nextAt); err != nil {
		t.Fatalf("read operation: %v", err)
	}
	if status != codemeta.InvoiceOperationStatusRetryScheduled.Value {
		t.Fatalf("operation.status = %q, want RETRY_SCHEDULED", status)
	}
	if attempt != 1 {
		t.Errorf("attempt_count = %d, want 1", attempt)
	}
	if !nextAt.After(before.Add(50 * time.Second)) {
		t.Errorf("next_attempt_at = %v，未被推遲（退避沒生效 ⇒ 下一輪立刻重打 AMEGO）", nextAt)
	}
	assertDocStatus(t, env, res.DocumentID, codemeta.InvoiceDocumentStatusPending.Value)

	var nOutbox int
	_ = env.pool.QueryRow(ctx, `SELECT count(*) FROM invoice_outbox_events WHERE aggregate_id=$1`, res.DocumentID).Scan(&nOutbox)
	if nOutbox != 0 {
		t.Errorf("可重試失敗不得發事件，卻發了 %d 則", nOutbox)
	}
}


func TestIntegration_Issuance_InvariantFailureNeverReachesProvider(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)

	in := itInput(env)
	in.Snapshot.TotalActualAmount = 999

	res, err := env.uc.Prepare(ctx, in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if res.OperationID != 0 {
		if claimed, cerr := env.uc.ClaimForSubmit(ctx, res.OperationID); cerr == nil && claimed {
			_ = env.uc.Submit(ctx, res.OperationID)
		}
	}
	if n := env.amego.requests.Load(); n != 0 {
		t.Fatalf("斷言失敗的草稿被送出去了 %d 次（§5A.11 是 fail-closed，不是警告）", n)
	}

	if res.DocumentStatus != codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		t.Fatalf("status = %q, want AWAITING_MANUAL", res.DocumentStatus)
	}
	if res.ManualReasonCode != domain.CodeAmountMismatch {
		t.Fatalf("manual_reason_code = %q, want %q", res.ManualReasonCode, domain.CodeAmountMismatch)
	}
	if res.OperationID != 0 {
		t.Fatalf("AWAITING_MANUAL 不應建立 operation（§7.1.3 的 AM 節點明文「operation 不建」）")
	}

	var status, reason string
	if err := env.pool.QueryRow(ctx,
		`SELECT status, coalesce(manual_reason_code,'') FROM invoice_documents WHERE id=$1`,
		res.DocumentID).Scan(&status, &reason); err != nil {
		t.Fatalf("read document: %v", err)
	}
	if status != "AWAITING_MANUAL" || reason != domain.CodeAmountMismatch {
		t.Fatalf("落庫值 = %q/%q", status, reason)
	}
}


func TestIntegration_Issuance_DuplicateOrderPaidCreatesOneDocument(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 1)

	first, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare#1: %v", err)
	}
	second, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare#2: %v", err)
	}
	if !second.AlreadyExisted {
		t.Fatal("第二次消費應命中 uq_invdoc_order_identity")
	}

	var n int
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_documents WHERE issuer_id=$1`, env.issuerID).Scan(&n); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if n != 1 {
		t.Fatalf("建了 %d 張發票，want 1（document_id=%d）", n, first.DocumentID)
	}
}


func TestIntegration_Issuance_ReceiptGetsReceiptNoAndNoDeadline(t *testing.T) {
	ctx := context.Background()
	env := setupEnv(t, codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, 1)

	res, err := env.uc.Prepare(ctx, itInput(env))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if res.DocumentKind != codemeta.InvoiceDocumentKindReceipt.Value {
		t.Fatalf("document_kind = %q, want RECEIPT", res.DocumentKind)
	}
	var receiptNo *string
	var deadline *time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT receipt_no, statutory_deadline_at FROM invoice_documents WHERE id=$1`,
		res.DocumentID).Scan(&receiptNo, &deadline); err != nil {
		t.Fatalf("read document: %v", err)
	}
	if receiptNo == nil || *receiptNo == "" {
		t.Error("RECEIPT 必須有 receipt_no（chk_invdoc_receipt）")
	}
	if deadline != nil {
		t.Error("RECEIPT 無法定期限（chk_invdoc_deadline_presence）")
	}
}

func assertDocStatus(t *testing.T, env *itEnv, id int64, want string) {
	t.Helper()
	var got string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT status FROM invoice_documents WHERE id=$1`, id).Scan(&got); err != nil {
		t.Fatalf("read document status: %v", err)
	}
	if got != want {
		t.Fatalf("document.status = %q, want %q", got, want)
	}
}

func assertOpStatus(t *testing.T, env *itEnv, id int64, want string) {
	t.Helper()
	var got string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT status FROM invoice_operations WHERE id=$1`, id).Scan(&got); err != nil {
		t.Fatalf("read operation status: %v", err)
	}
	if got != want {
		t.Fatalf("operation.status = %q, want %q", got, want)
	}
}
