//go:build integration

// 本檔是本波交付的**端到端**測試：真的 PostgreSQL ＋ 真的 amego client（指向 httptest）
// ＋ 真的 pkg/outbox.Worker，把三塊串起來跑一遍：
//
//	order.paid consumer → invoice_documents ＋ invoice_operations
//	  → INV.op RunOnce  → AMEGO 一次請求、號碼回填、operation SUCCEEDED、outbox 一列
//	  → INV.ob RunOnce  → 那一列被標記已派送
//
// 為什麼非要真 DB：這條路徑上幾乎每一條規則的失效形狀都不是編譯錯——ON CONFLICT
// DO NOTHING 的 rows=0、CAS 的 rows=0、`AND status='SENDING'` 守衛、outbox 與業務寫入
// 同一交易。靜態測試一條都驗不到。
//
// 跑法：`go test -tags=integration ./internal/worker/`（需要本機 Docker daemon）。
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

const (
	e2eTaxID  = "12345678"
	e2eAppKey = "integration-app-key"
)

// e2eAmego 是 AMEGO 的替身。
//
// requests 是 atomic 計數器：「AMEGO 沒有再被打一次」這種**陰性**宣稱只有數它才算數
// ——用「有沒有第二張發票」去推論，遇到「送出去但回應遺失」就會答錯。
type e2eAmego struct {
	srv      *httptest.Server
	requests atomic.Int64
}

func newE2EAmego(t *testing.T) *e2eAmego {
	t.Helper()
	f := &e2eAmego{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != amego.EndpointIssue {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "", "invoice_number": "AB12345678",
			"invoice_time": time.Now().Unix(), "random_number": "4321",
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type e2eEnv struct {
	pool     *pgxpool.Pool
	amego    *e2eAmego
	orderFax *e2eOrderFacts
	uc       *usecase.Issuance
	paid     *invoicemq.OrderPaidConsumer
	issue    *IssueWorker
	ob       *OutboxWorker
	pub      *fakePublisher
	orderNo  string
	storeID  int64
	issuerID int64
}

// e2eOrderFacts 供應 consumer 拿不到的那份訂單快照（真實環境走 ORD 的 gRPC）。
type e2eOrderFacts struct{ calls atomic.Int64 }

func (f *e2eOrderFacts) GetOrderInvoiceSnapshot(context.Context, string, int64) (invoicemq.OrderFacts, error) {
	f.calls.Add(1)
	return invoicemq.OrderFacts{
		SnapshotVersion: 1,
		Snapshot: domain.OrderSnapshot{
			TotalOriginalAmount: 100, TotalActualAmount: 100,
			DiscountFunding: domain.DiscountFundingSeller,
			Items: []domain.OrderItem{
				{OrderItemID: 1, SkuID: 11, ProductName: "有機高麗菜", Quantity: 1, UnitPrice: 100},
			},
		},
		NotifyEmail: "buyer@example.com",
	}, nil
}

type e2ePayFacts struct{ paidAt time.Time }

func (f e2ePayFacts) GetPaymentFacts(context.Context, string) (invoicemq.PaymentFacts, error) {
	return invoicemq.PaymentFacts{TradeNo: "T1", PayTime: f.paidAt}, nil
}

func setupE2E(t *testing.T) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	db, _ := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	cur, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	ring := &postgres.KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "integration-lookup-key"}

	seq := time.Now().UnixNano()
	issuerID := seq
	storeID := seq%1_000_000_000 + 1
	orderNo := fmt.Sprintf("O%09d", seq%1_000_000_000)

	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1,'SELLER',$2,$3,'active')`, issuerID, storeID, "端到端測試賣場"); err != nil {
		t.Fatalf("seed issuer: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO seller_tax_profiles
		   (id, issuer_id, review_status, claimed_obligation, default_tax_category,
		    legal_name, tax_id, approved_at)
		 VALUES ($1,$2,$3,'REQUIRED',1,$4,$5,NOW())`,
		issuerID+1, issuerID,
		codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		"端到端測試股份有限公司", e2eTaxID); err != nil {
		t.Fatalf("seed tax profile: %v", err)
	}

	accounts := postgres.NewProviderAccountRepository(pool, ring)
	if err := accounts.Create(ctx, domain.NewProviderAccount{
		ID: issuerID + 2, IssuerID: issuerID, Provider: "AMEGO",
		ProviderTaxID: e2eTaxID, AppKey: e2eAppKey,
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

	fa := newE2EAmego(t)
	amegoClient, err := amego.New(amego.Options{
		BaseURL:          fa.srv.URL,
		HTTPClient:       fa.srv.Client(),
		AllowRealInvoice: false,
		TestInvoice:      e2eTaxID,
		RateLimitQPS:     50,
		Deadlines:        amego.DefaultDeadlines(),
	})
	if err != nil {
		t.Fatalf("amego.New: %v", err)
	}

	docs := postgres.NewDocumentRepository(pool, ring)
	ops := postgres.NewOperationRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)
	inbox := postgres.NewInboxRepository(pool)

	var idSeq int64
	uc := usecase.NewIssuance(
		database.NewTxManager(pool),
		postgres.NewIssuerRepository(pool),
		postgres.NewSellerTaxProfileRepository(pool),
		accounts, docs, ops, postgres.NewCorrectionRequestRepository(pool),
		outboxRepo, postgres.NewAuditRepository(pool),
		amegoClient,
		stubReceiptRenderer{}, stubReceiptUploader{},
		func() int64 { idSeq++; return issuerID + 1000 + idSeq },
		usecase.IssuanceConfig{
			MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
			IssueLease: 120 * time.Second, RecheckBase: 30 * time.Second,
		},
		nil,
	)

	orderFacts := &e2eOrderFacts{}
	paid := invoicemq.NewOrderPaidConsumer(
		inbox, uc, orderFacts, e2ePayFacts{paidAt: time.Now().UTC()},
		invoicemq.UnimplementedAdjustmentApplier{}, docs, nil)

	pub := &fakePublisher{}
	return &e2eEnv{
		pool: pool, amego: fa, uc: uc, paid: paid, pub: pub, orderFax: orderFacts,
		issue:   NewIssueWorker(ops, docs, uc, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil),
		ob:      NewOutboxWorker(outboxRepo, pub, UnresolvedSellerAccountResolver{}, testOutboxCfg(), obParkInterval, testObsB, nil),
		orderNo: orderNo, storeID: storeID, issuerID: issuerID,
	}
}

func e2ePaidBody(t *testing.T, eventID string, env *e2eEnv) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"event_id":            eventID,
		"order_no":            env.orderNo,
		"buyer_id":            2623900520,
		"store_id":            env.storeID,
		"total_actual_amount": 100,
		// C1 的格式：LocalDateTime.toString()，無 Z、無時區（§6.3.2.2 陷阱表第 1 列）。
		"occurred_at": time.Now().UTC().Format("2006-01-02T15:04:05.999999999"),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// 驗收 3：端到端一條龍
// ---------------------------------------------------------------------------

func TestIntegration_EndToEnd_PaidToPublished(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)

	// ① consumer：order.paid → document ＋ operation
	if err := env.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-1", env)); err != nil {
		t.Fatalf("order.paid handle: %v", err)
	}

	var docID, opID int64
	var docStatus, opStatus string
	if err := env.pool.QueryRow(ctx,
		`SELECT d.id, d.status, o.id, o.status
		   FROM invoice_documents d JOIN invoice_operations o ON o.document_id = d.id
		  WHERE d.source_ref = $1`, env.orderNo).Scan(&docID, &docStatus, &opID, &opStatus); err != nil {
		t.Fatalf("read document/operation: %v", err)
	}
	if docStatus != codemeta.InvoiceDocumentStatusPending.Value ||
		opStatus != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("consumer 落點錯：doc=%s op=%s", docStatus, opStatus)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_inbox_events WHERE business_dedup_key = $1`,
		"ORDER_PAID:"+env.orderNo); n != 1 {
		t.Fatalf("inbox 列數 = %d，want 1", n)
	}

	// ② INV.op：claim → 送 AMEGO → ISSUED ＋ SUCCEEDED ＋ 一列 outbox
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("INV.op RunOnce: %v", err)
	}
	if n := env.amego.requests.Load(); n != 1 {
		t.Fatalf("AMEGO 收到 %d 次請求，want 1", n)
	}

	var invoiceNo, randomCode string
	if err := env.pool.QueryRow(ctx,
		`SELECT status, coalesce(provider_invoice_no,''), coalesce(provider_random_code,'')
		   FROM invoice_documents WHERE id = $1`, docID).Scan(&docStatus, &invoiceNo, &randomCode); err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if docStatus != codemeta.InvoiceDocumentStatusIssued.Value {
		t.Fatalf("document status = %s，want ISSUED", docStatus)
	}
	if invoiceNo != "AB12345678" || randomCode != "4321" {
		t.Fatalf("號碼未回填：%q / %q", invoiceNo, randomCode)
	}
	if err := env.pool.QueryRow(ctx,
		`SELECT status FROM invoice_operations WHERE id = $1`, opID).Scan(&opStatus); err != nil {
		t.Fatalf("reload operation: %v", err)
	}
	if opStatus != codemeta.InvoiceOperationStatusSucceeded.Value {
		t.Fatalf("operation status = %s，want SUCCEEDED", opStatus)
	}

	var outboxID int64
	var routingKey, outboxStatus string
	if err := env.pool.QueryRow(ctx,
		`SELECT id, routing_key, status FROM invoice_outbox_events WHERE aggregate_id = $1`,
		docID).Scan(&outboxID, &routingKey, &outboxStatus); err != nil {
		t.Fatalf("read outbox（應恰一列）: %v", err)
	}
	if routingKey != "invoice.issued" || outboxStatus != "pending" {
		t.Fatalf("outbox 列：routing_key=%s status=%s", routingKey, outboxStatus)
	}

	// ③ INV.ob：派送
	if err := env.ob.RunOnce(ctx); err != nil {
		t.Fatalf("INV.ob RunOnce: %v", err)
	}
	if len(env.pub.calls) != 1 {
		t.Fatalf("publish 次數 = %d，want 1", len(env.pub.calls))
	}
	if env.pub.calls[0].routingKey != "invoice.issued" ||
		env.pub.calls[0].exchange != "hawkr.invoice" {
		t.Fatalf("發到 %s/%s", env.pub.calls[0].exchange, env.pub.calls[0].routingKey)
	}
	if err := env.pool.QueryRow(ctx,
		`SELECT status FROM invoice_outbox_events WHERE id = $1`, outboxID).Scan(&outboxStatus); err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if outboxStatus != "done" {
		t.Fatalf("派送後 outbox status = %s，want done", outboxStatus)
	}

	// payload 逐 key 檢查：notify_seller_account_id 必須在派送當下補上（§6.3.3.2），
	// 解析不到時是字串 "0"，而不是缺欄或裸數字。
	var published map[string]any
	if err := json.Unmarshal(env.pub.calls[0].body, &published); err != nil {
		t.Fatalf("decode published: %v", err)
	}
	if published["notify_seller_account_id"] != "0" {
		t.Errorf("notify_seller_account_id = %#v，want \"0\"", published["notify_seller_account_id"])
	}
	if published["invoice_no"] != "AB12345678" {
		t.Errorf("invoice_no = %#v", published["invoice_no"])
	}
	// 🔴 憑證邊界：invoice_no + random_code 就能在財政部平台查到整張發票 ⇒ random_code
	// 是憑證不是資料，絕不上 MQ（§6.3.3.2 第 2 條）。
	if _, leaked := published["random_code"]; leaked {
		t.Error("random_code 洩漏進 MQ payload —— 它是憑證不是資料")
	}
}

// ---------------------------------------------------------------------------
// 驗收 4：重送同一則 order.paid → 只有一份 document，AMEGO 不再被打
// ---------------------------------------------------------------------------

func TestIntegration_EndToEnd_DuplicateOrderPaidIsIdempotent(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)

	if err := env.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-1", env)); err != nil {
		t.Fatalf("第 1 次 handle: %v", err)
	}
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("INV.op RunOnce: %v", err)
	}
	first := env.amego.requests.Load()
	if first != 1 {
		t.Fatalf("第一輪 AMEGO 請求數 = %d，want 1", first)
	}

	// 重送兩次：一次同 event_id（RabbitMQ 重投），一次換 event_id（上游重發）。
	// 兩把唯一鍵各守一種，只做其中一把的實作會在第二種上破。
	for _, eventID := range []string{"evt-1", "evt-2"} {
		if err := env.paid.Handle(ctx, "order.paid", e2ePaidBody(t, eventID, env)); err != nil {
			t.Fatalf("重送 %s: %v", eventID, err)
		}
	}
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("第二輪 INV.op RunOnce: %v", err)
	}

	if n := countRows(t, env, `SELECT count(*) FROM invoice_documents WHERE source_ref = $1`, env.orderNo); n != 1 {
		t.Fatalf("document 列數 = %d，want 1（重送不得開出第二張發票）", n)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_operations`); n != 1 {
		t.Fatalf("operation 列數 = %d，want 1", n)
	}
	if got := env.amego.requests.Load(); got != first {
		t.Fatalf("AMEGO 請求數從 %d 變成 %d —— 重送打到 provider 了", first, got)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_outbox_events`); n != 1 {
		t.Fatalf("outbox 列數 = %d，want 1", n)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_inbox_events WHERE business_dedup_key = $1`,
		"ORDER_PAID:"+env.orderNo); n != 1 {
		t.Fatalf("inbox 列數 = %d，want 1", n)
	}
	// 🔴 這一條才是**consumer 層**去重的鑑別力所在。
	//
	// 上面那幾條（document 數、AMEGO 請求數）在 consumer 的去重被拿掉之後**仍然會通過**
	// ——因為 Prepare 自己在 DB 層還有一道 ON CONFLICT DO NOTHING。兩層防線是好事，
	// 但它讓「document 只有一份」對「consumer 有沒有去重」完全沒有鑑別力。
	// 取快照的次數只有 consumer 那一層擋得住：去重被拿掉 ⇒ 每次重送都會多打一次 ORD 的 gRPC。
	if n := env.orderFax.calls.Load(); n != 1 {
		t.Fatalf("ORD 快照被取了 %d 次，want 1 —— consumer 層的去重沒有生效", n)
	}
}

// TestIntegration_EndToEnd_OutOfOrderRefundIsDeferred 守 §7.2.5：退款先於付款抵達時
// 不得丟棄，inbox 列落 DEFERRED 並 ACK。
func TestIntegration_EndToEnd_OutOfOrderRefundIsDeferred(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)

	inbox := postgres.NewInboxRepository(env.pool)
	realDocs := postgres.NewDocumentRepository(env.pool, testKeyRing(t))
	refund := invoicemq.NewRefundFinalizedConsumer(inbox, realDocs, invoicemq.UnimplementedAdjustmentApplier{}, nil)

	// 🔴 形狀逐欄照 order-service 的 OrderRefundFinalizedPayload：16 個 component
	// 全是 String（@JsonNaming(SnakeCaseStrategy)）。金額寫成 JSON 數字的 fixture 會
	// 讓這支測試證明一件現實中不存在的事 —— 而那正是原缺陷能活下來的原因。
	nowIso := time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{
		"event_id": "r-1", "order_no": env.orderNo, "occurred_at": nowIso,
		"store_id": fmt.Sprintf("%d", env.storeID), "buyer_id": "2623900520",
		"refund_ref": "TXN-E2E-1", "refund_source": "AFTERSALE_CASE",
		"refund_outcome": "PARTIAL_REFUND", "refund_amount": "100",
		"order_total_actual_amount": "100", "cumulative_refund_amount": "100",
		"case_no": "C2026083000001", "cancel_id": nil,
		"payment_method": "CREDIT_CARD", "refunded_at": nowIso,
		"correlation_id": "0af7651916cd43dd8448eb211c80319c",
	})
	if err := refund.Handle(ctx, "order.refund.finalized", body); err != nil {
		t.Fatalf("退款先到時必須 ACK（不丟棄、不 NACK）：%v", err)
	}

	var status string
	var deferredUntil *time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT status, deferred_until FROM invoice_inbox_events WHERE business_dedup_key = $1`,
		"REFUND:TXN-E2E-1").Scan(&status, &deferredUntil); err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if status != "DEFERRED" {
		t.Fatalf("status = %s，want DEFERRED", status)
	}
	if deferredUntil == nil {
		t.Fatal("deferred_until 必須非空 —— 兜底掃描靠它回到候選集合")
	}
}

// ---------------------------------------------------------------------------
// 驗收 5：出向事件的 store_id 解得出賣家 account_id（P0-3 的端到端證明）
// ---------------------------------------------------------------------------

// resolveOnlyStoreID 只認得**真正的 store_id**，其餘一律回錯。
//
// 🔴 這個 fake 的鑑別力全在「其餘一律回錯」：接受任何 int64 的 resolver 會讓
// publisher 填 invoice_issuers 的 PK 也照樣通過 —— 而那正是原缺陷的形狀。
type resolveOnlyStoreID struct {
	storeID   int64
	accountID int64
	asked     []int64
}

func (r *resolveOnlyStoreID) ResolveSellerAccountID(_ context.Context, storeID int64) (int64, error) {
	r.asked = append(r.asked, storeID)
	if storeID != r.storeID {
		return 0, fmt.Errorf("no seller for store_id=%d", storeID)
	}
	return r.accountID, nil
}

// TestIntegration_EndToEnd_SellerNotifyRoutingResolves 是 (B) 類模板的解除證據。
//
// 原缺陷：submit.go 的 P1／P2、complaint.go 的 P8 把 `store_id` 填成
// `invoice_documents.issuer_id`（＝invoice_issuers 的 **PK**，TSID），而 outbox
// dispatcher 的 withNotifyRouting 拿它去 ResolveSellerAccountID ⇒ 恆解不出賣家
// ⇒ `notify_seller_account_id` 恆為 `"0"` ⇒ 三張賣家向模板靜默 0 投遞。
// 契約 gate 抓不到（它只驗那個 key 在不在 emitted_keys 內），所以三列先以
// enabled=false 進表，解除條件是「publisher 改填 issuer_ref_id 並實發一則確認非 0」。
//
// 🔴 本測試就是那個「實發一則」，而且它比一次性的 dev 觀察強：
//   - 每次 CI 都跑，不是一次性的
//   - env 的 issuer PK 與 issuer_ref_id 是**不同的兩個值**（setupE2E 以
//     seq 與 seq%1e9+1 產生），所以「填錯哪一個」在這裡可觀測
//   - resolver 只認 store_id，填 PK 會當場失敗而不是靜默回 0
func TestIntegration_EndToEnd_SellerNotifyRoutingResolves(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)

	if env.issuerID == env.storeID {
		t.Fatalf("fixture 無鑑別力：issuer PK 與 issuer_ref_id 同值（%d）", env.storeID)
	}

	const sellerAccountID int64 = 4242424242
	sellers := &resolveOnlyStoreID{storeID: env.storeID, accountID: sellerAccountID}
	pub := &fakePublisher{}
	ob := NewOutboxWorker(postgres.NewOutboxRepository(env.pool), pub, sellers,
		testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := env.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "e-notify", env)); err != nil {
		t.Fatalf("order.paid: %v", err)
	}
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("INV.op RunOnce: %v", err)
	}
	if err := ob.RunOnce(ctx); err != nil {
		t.Fatalf("INV.ob RunOnce: %v", err)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("publish 次數 = %d，want 1", len(pub.calls))
	}

	var published map[string]any
	if err := json.Unmarshal(pub.calls[0].body, &published); err != nil {
		t.Fatalf("decode published: %v", err)
	}
	wantStore := fmt.Sprintf("%d", env.storeID)
	if published["store_id"] != wantStore {
		t.Fatalf("payload store_id = %#v，want %q（invoice_issuers.issuer_ref_id）。\n"+
			"若它等於 %d，就是又填回了 invoice_issuers 的 PK。",
			published["store_id"], wantStore, env.issuerID)
	}
	if got := published["notify_seller_account_id"]; got != fmt.Sprintf("%d", sellerAccountID) {
		t.Fatalf("notify_seller_account_id = %#v，want %q —— 它是 \"0\" 就代表賣家向通知會被下游 skip",
			got, fmt.Sprintf("%d", sellerAccountID))
	}
	if len(sellers.asked) != 1 || sellers.asked[0] != env.storeID {
		t.Fatalf("dispatcher 拿去解析的值 = %v，want [%d]", sellers.asked, env.storeID)
	}
}

func testKeyRing(t *testing.T) *postgres.KeyRing {
	t.Helper()
	cur, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return &postgres.KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "integration-lookup-key"}
}

func countRows(t *testing.T, env *e2eEnv, q string, args ...any) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// 平台代製收據的替身（#3026）。本套 worker 整合測試走的是 AMEGO_AUTO 那條線，
// 收據路徑不會被觸發 —— 兩個 stub 因此**故意會失敗**：被呼叫到就代表有東西走錯了路。
// ---------------------------------------------------------------------------

type stubReceiptRenderer struct{}

func (stubReceiptRenderer) Render(domain.ReceiptData) ([]byte, error) {
	return nil, errors.New("stubReceiptRenderer: 本測試不應走到收據產製")
}

type stubReceiptUploader struct{}

func (stubReceiptUploader) UploadSecure(context.Context, []byte, string, string, string) (string, error) {
	return "", errors.New("stubReceiptUploader: 本測試不應走到收據上傳")
}
