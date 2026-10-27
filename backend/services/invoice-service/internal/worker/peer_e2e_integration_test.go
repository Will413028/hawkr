//go:build integration

// 本檔是 ORD／PAY peer client 落地之後的端到端測試：真的 PostgreSQL ＋ 真的
// `internal/client/order` / `internal/client/payment`（跑在 bufconn 上的真 gRPC server）
// ＋ 真的 amego client（指向 httptest）＋ 真的 `pkg/outbox.Worker`。
//
// 與同 package 的 integration_test.go 的差別：那一支用 interface 假物件供應訂單與付款
// 事實（peer client 當時還不存在），本支讓**真的 client** 走一趟 wire。差別不是形式：
// 欄位對映（string → int64）、envelope code 檢查、載具解析與 I-47 的歸屬述詞
// 全部只在真 client 這條路徑上，假物件一條都覆蓋不到。
//
// 跑法：`go test -tags=integration ./internal/worker/`（需要本機 Docker daemon）。
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	paymentpb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	orderclient "github.com/Will413028/hawkr/invoice-service/internal/client/order"
	paymentclient "github.com/Will413028/hawkr/invoice-service/internal/client/payment"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

const (
	peerTaxID  = "12345678"
	peerAppKey = "peer-e2e-app-key"
)

// ---------------------------------------------------------------------------
// 對端替身：真的 gRPC server，跑在 bufconn 上
// ---------------------------------------------------------------------------

type peerOrderServer struct {
	orderpb.UnimplementedOrderServiceServer
	resp  *orderpb.GetOrderInvoiceSnapshotInternalResponse
	calls atomic.Int64
}

func (s *peerOrderServer) GetOrderInvoiceSnapshotInternal(context.Context, *orderpb.GetOrderInvoiceSnapshotInternalRequest) (*orderpb.GetOrderInvoiceSnapshotInternalResponse, error) {
	s.calls.Add(1)
	return s.resp, nil
}

type peerPaymentServer struct {
	paymentpb.UnimplementedPaymentServiceServer
	resp *paymentpb.GetPaymentFactsInternalResponse
}

func (s *peerPaymentServer) GetPaymentFactsInternal(context.Context, *paymentpb.GetPaymentFactsInternalRequest) (*paymentpb.GetPaymentFactsInternalResponse, error) {
	return s.resp, nil
}

func peerDial(t *testing.T, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	register(gs)
	go func() { _ = gs.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		gs.Stop()
		_ = lis.Close()
	})
	return conn
}

type peerAmego struct {
	srv      *httptest.Server
	requests atomic.Int64
}

func newPeerAmego(t *testing.T) *peerAmego {
	t.Helper()
	f := &peerAmego{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != amego.EndpointIssue {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "", "invoice_number": "CD87654321",
			"invoice_time": time.Now().Unix(), "random_number": "1234",
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// ---------------------------------------------------------------------------
// 環境
// ---------------------------------------------------------------------------

type peerEnv struct {
	pool     *pgxpool.Pool
	amego    *peerAmego
	ordSrv   *peerOrderServer
	paid     *invoicemq.OrderPaidConsumer
	issue    *IssueWorker
	ob       *OutboxWorker
	orderNo  string
	storeID  int64
	buyerID  int64
	prefID   int64
	issuerID int64
}

// peerSnapshot 是 ORD 回的快照：EXPLICIT_PREFERENCE 指向一列**真的**落庫的
// buyer_invoice_preferences（DONATION kind ⇒ 值是明文欄位，不需要解密路徑）。
func peerSnapshot(orderNo string, prefID int64) *orderpb.OrderInvoiceSnapshotData {
	return &orderpb.OrderInvoiceSnapshotData{
		OrderNo:                  orderNo,
		SnapshotVersion:          "1",
		InvoiceKind:              "B2C",
		CarrierSelectionMode:     codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value,
		BuyerInvoicePreferenceId: fmt.Sprintf("%d", prefID),
		NotifyEmail:              "buyer@example.com",
		TotalOriginalAmount:      "300",
		ShippingFee:              "0",
		DiscountAmount:           "0",
		TotalActualAmount:        "300",
		DiscountFunding:          "SELLER",
		Items: []*orderpb.OrderInvoiceSnapshotItem{
			{OrderItemId: "9007199254740993", SkuId: "9007199254740994",
				ProductName: "有機高麗菜", SkuName: "大顆", Quantity: "3", UnitPrice: "100"},
		},
	}
}

// setupPeerE2E 起一套完整環境。prefOwner 決定那列常用發票資料屬於誰——
// 傳 env.buyerID ＝ 歸屬正確；傳別人 ＝ I-47 的攻擊形狀。
func setupPeerE2E(t *testing.T, prefOwner func(buyerID int64) int64) *peerEnv {
	t.Helper()
	ctx := context.Background()

	db, _ := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	cur, err := crypto.New("peer-e2e-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	ring := &postgres.KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "peer-e2e-lookup-key"}

	seq := time.Now().UnixNano()
	issuerID := seq
	storeID := seq%1_000_000_000 + 1
	buyerID := seq%1_000_000_000 + 2
	prefID := seq + 7
	orderNo := fmt.Sprintf("P%09d", seq%1_000_000_000)

	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1,'SELLER',$2,$3,'active')`, issuerID, storeID, "peer 端到端賣場"); err != nil {
		t.Fatalf("seed issuer: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO seller_tax_profiles
		   (id, issuer_id, review_status, claimed_obligation, default_tax_category,
		    legal_name, tax_id, approved_at)
		 VALUES ($1,$2,$3,'REQUIRED',1,$4,$5,NOW())`,
		issuerID+1, issuerID,
		codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		"peer 端到端股份有限公司", peerTaxID); err != nil {
		t.Fatalf("seed tax profile: %v", err)
	}

	accounts := postgres.NewProviderAccountRepository(pool, ring)
	if err := accounts.Create(ctx, domain.NewProviderAccount{
		ID: issuerID + 2, IssuerID: issuerID, Provider: "AMEGO",
		ProviderTaxID: peerTaxID, AppKey: peerAppKey,
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

	// 🔴 一列**真的** buyer_invoice_preferences：歸屬驗證用的是它的 buyer_id 欄，
	// 而述詞寫在 GetBuyerInvoicePreferenceByID 的 SQL 裡（不是查回來再比對）。
	if _, err := pool.Exec(ctx,
		`INSERT INTO buyer_invoice_preferences (id, buyer_id, preference_kind, donation_code, is_default)
		 VALUES ($1,$2,'DONATION','9527',false)`, prefID, prefOwner(buyerID)); err != nil {
		t.Fatalf("seed buyer preference: %v", err)
	}

	fa := newPeerAmego(t)
	amegoClient, err := amego.New(amego.Options{
		BaseURL:          fa.srv.URL,
		HTTPClient:       fa.srv.Client(),
		AllowRealInvoice: false,
		TestInvoice:      peerTaxID,
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
		func() int64 { idSeq++; return issuerID + 2000 + idSeq },
		usecase.IssuanceConfig{
			MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
			IssueLease: 120 * time.Second, RecheckBase: 30 * time.Second,
		},
		nil,
	)

	// 🔴 真的 peer client：走 bufconn 上的真 gRPC server，且 preference reader 是
	// **真的 repository**（PartialPreferenceReader 只在 GetDefault／解密兩支上 fail-loud，
	// GetByID ＝ I-47 的歸屬述詞走的是真 SQL）。
	ordSrv := &peerOrderServer{resp: &orderpb.GetOrderInvoiceSnapshotInternalResponse{
		Code: 0, Message: "success", Data: peerSnapshot(orderNo, prefID),
	}}
	ordConn := peerDial(t, func(gs *grpc.Server) { orderpb.RegisterOrderServiceServer(gs, ordSrv) })
	ordClient := orderclient.NewClientWithStub(
		orderpb.NewOrderServiceClient(ordConn),
		orderclient.RepoPreferenceReader{Repo: postgres.NewBuyerPreferenceRepository(pool, ring)},
		2*time.Second)

	payTime := time.Now().UTC().Truncate(time.Second)
	providerPaidAt := payTime.Add(-90 * time.Second).Format("2006-01-02T15:04:05Z")
	paySrv := &peerPaymentServer{resp: &paymentpb.GetPaymentFactsInternalResponse{
		Code: 0, Message: "success",
		Data: &paymentpb.PaymentFactsData{
			OrderNo: orderNo, TradeNo: orderNo + "001",
			ProviderPaidAt: &providerPaidAt,
			PayTime:        payTime.Format("2006-01-02T15:04:05Z"),
			TradeAmt:       "300", PaymentType: "2201", IsDuplicate: false,
		},
	}}
	payConn := peerDial(t, func(gs *grpc.Server) { paymentpb.RegisterPaymentServiceServer(gs, paySrv) })
	payClient := paymentclient.NewClientWithStub(paymentpb.NewPaymentServiceClient(payConn), 2*time.Second)

	paid := invoicemq.NewOrderPaidConsumer(
		inbox, uc, ordClient, payClient, invoicemq.UnimplementedAdjustmentApplier{}, docs, nil)

	return &peerEnv{
		pool: pool, amego: fa, ordSrv: ordSrv, paid: paid,
		issue:   NewIssueWorker(ops, docs, uc, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil),
		ob:      NewOutboxWorker(outboxRepo, &fakePublisher{}, UnresolvedSellerAccountResolver{}, testOutboxCfg(), obParkInterval, testObsB, nil),
		orderNo: orderNo, storeID: storeID, buyerID: buyerID, prefID: prefID, issuerID: issuerID,
	}
}

func peerPaidBody(t *testing.T, env *peerEnv, eventID string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"event_id":            eventID,
		"order_no":            env.orderNo,
		"buyer_id":            env.buyerID,
		"store_id":            env.storeID,
		"total_actual_amount": 300,
		// C1 的格式：LocalDateTime.toString()，無 Z、無時區（§6.3.2.2 陷阱表第 1 列）。
		"occurred_at": time.Now().UTC().Format("2006-01-02T15:04:05.999999999"),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func peerCount(t *testing.T, env *peerEnv, q string, args ...any) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// ① happy path：order.paid → 真 ORD／PAY client → ISSUED → outbox 派送
// ---------------------------------------------------------------------------

func TestIntegration_PeerClients_PaidToOutbox(t *testing.T) {
	ctx := context.Background()
	env := setupPeerE2E(t, func(buyerID int64) int64 { return buyerID }) // 歸屬正確

	if err := env.paid.Handle(ctx, "order.paid", peerPaidBody(t, env, "peer-evt-1")); err != nil {
		t.Fatalf("order.paid handle: %v", err)
	}
	if n := env.ordSrv.calls.Load(); n != 1 {
		t.Fatalf("ORD 被呼叫 %d 次，want 1", n)
	}

	var docID, opID int64
	var docStatus, opStatus, donation string
	var totalAmount int64
	if err := env.pool.QueryRow(ctx,
		`SELECT d.id, d.status, d.total_amount, coalesce(d.donation_code,''), o.id, o.status
		   FROM invoice_documents d JOIN invoice_operations o ON o.document_id = d.id
		  WHERE d.source_ref = $1`, env.orderNo).
		Scan(&docID, &docStatus, &totalAmount, &donation, &opID, &opStatus); err != nil {
		t.Fatalf("read document/operation: %v", err)
	}
	if docStatus != codemeta.InvoiceDocumentStatusPending.Value ||
		opStatus != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("consumer 落點錯：doc=%s op=%s", docStatus, opStatus)
	}
	// 🔴 金額真的過了一趟 string → int64。它是 mutation ① 的偵測器：
	// ORD client 若在下游出錯時回零值 ＋ nil error，這裡會是 0。
	if totalAmount != 300 {
		t.Fatalf("total_amount = %d，want 300（品項 3 × 100，全部走 proto 的數字字串）", totalAmount)
	}
	// 載具解析真的發生了：捐贈碼來自那列 buyer_invoice_preferences，不在 proto 裡。
	if donation != "9527" {
		t.Fatalf("donation_code = %q，want 9527（載具解析沒有被插進 ORD adapter）", donation)
	}
	// 明細行的 TSID 不得掉精度。
	var orderItemID int64
	if err := env.pool.QueryRow(ctx,
		`SELECT order_item_id FROM invoice_document_lines
		  WHERE document_id = $1 AND line_type = 'PRODUCT' ORDER BY line_seq LIMIT 1`, docID).
		Scan(&orderItemID); err != nil {
		t.Fatalf("read line: %v", err)
	}
	if orderItemID != 9007199254740993 {
		t.Fatalf("order_item_id = %d，TSID 在某一層掉了精度", orderItemID)
	}

	// ② INV.op：送 AMEGO → ISSUED
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("INV.op RunOnce: %v", err)
	}
	if n := env.amego.requests.Load(); n != 1 {
		t.Fatalf("AMEGO 收到 %d 次請求，want 1", n)
	}
	var invoiceNo string
	if err := env.pool.QueryRow(ctx,
		`SELECT status, coalesce(provider_invoice_no,'') FROM invoice_documents WHERE id = $1`, docID).
		Scan(&docStatus, &invoiceNo); err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if docStatus != codemeta.InvoiceDocumentStatusIssued.Value || invoiceNo != "CD87654321" {
		t.Fatalf("document = %s / %q，want ISSUED / CD87654321", docStatus, invoiceNo)
	}

	// ③ INV.ob：那一列 outbox 被派送
	if n := peerCount(t, env, `SELECT count(*) FROM invoice_outbox_events`); n != 1 {
		t.Fatalf("outbox 列數 = %d，want 1", n)
	}
	if err := env.ob.RunOnce(ctx); err != nil {
		t.Fatalf("INV.ob RunOnce: %v", err)
	}
	if n := peerCount(t, env,
		`SELECT count(*) FROM invoice_outbox_events WHERE completed_at IS NOT NULL`); n != 1 {
		t.Fatalf("已派送的 outbox 列數 = %d，want 1", n)
	}
}

// ---------------------------------------------------------------------------
// ② I-47：常用發票資料不屬於本訂單的買家 ⇒ AWAITING_MANUAL，不是無載具照常開票
// ---------------------------------------------------------------------------

func TestIntegration_PeerClients_PreferenceOwnershipMismatch(t *testing.T) {
	ctx := context.Background()
	// 那一列屬於別人（buyerID + 1）——這正是 IDOR 的形狀：id 猜得到、擁有者不同。
	env := setupPeerE2E(t, func(buyerID int64) int64 { return buyerID + 1 })

	if err := env.paid.Handle(ctx, "order.paid", peerPaidBody(t, env, "peer-evt-idor")); err != nil {
		t.Fatalf("歸屬錯誤不是傳輸錯誤，handler 不該回 error：%v", err)
	}

	var docStatus, manualReason, donation string
	if err := env.pool.QueryRow(ctx,
		`SELECT status, coalesce(manual_reason_code,''), coalesce(donation_code,'')
		   FROM invoice_documents WHERE source_ref = $1`, env.orderNo).
		Scan(&docStatus, &manualReason, &donation); err != nil {
		t.Fatalf("read document: %v", err)
	}
	if docStatus != codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		t.Fatalf("status = %s，want AWAITING_MANUAL", docStatus)
	}
	if manualReason != codemeta.InvoiceManualReasonPreferenceOwnershipMismatch.Value {
		t.Fatalf("manual_reason_code = %q，want PREFERENCE_OWNERSHIP_MISMATCH", manualReason)
	}
	// 🔴 陰性三連：別人的捐贈碼不得外洩、不得建 operation、不得打 AMEGO。
	// 「fallback 成無載具」的形狀在這裡的表現是 donation="" ＋ status=PENDING，
	// 上面那一條會先抓到；這裡再釘一次「連值都沒漏出去」。
	if donation != "" {
		t.Fatalf("donation_code = %q，別的買家的資料不得寫進這張發票", donation)
	}
	if n := peerCount(t, env, `SELECT count(*) FROM invoice_operations`); n != 0 {
		t.Fatalf("operation 列數 = %d，want 0", n)
	}
	if err := env.issue.RunOnce(ctx); err != nil {
		t.Fatalf("INV.op RunOnce: %v", err)
	}
	if n := env.amego.requests.Load(); n != 0 {
		t.Fatalf("AMEGO 收到 %d 次請求，want 0", n)
	}
}

// ---------------------------------------------------------------------------
// ③ ORD 回非 0 業務碼（gRPC status 是 OK）⇒ handler 必須 fail-loud，一列都不建
// ---------------------------------------------------------------------------

func TestIntegration_PeerClients_OrderEnvelopeErrorFailsLoud(t *testing.T) {
	ctx := context.Background()
	env := setupPeerE2E(t, func(buyerID int64) int64 { return buyerID })
	// 🔴 刻意**同時**回一份看起來完整的 data：只有這樣，這支測試才驗得到「client 真的
	// 讀了 envelope 的 code」。若 data 留空，拿掉 code 檢查之後 handler 仍會因為
	// snapshot_version=0 撞上 DB 約束而回 error ⇒ 測試照樣綠，鑑別力是假的。
	env.ordSrv.resp = &orderpb.GetOrderInvoiceSnapshotInternalResponse{
		Code: 4004, Message: "查無此訂單的發票快照",
		Data: peerSnapshot(env.orderNo, env.prefID),
	}

	err := env.paid.Handle(ctx, "order.paid", peerPaidBody(t, env, "peer-evt-envelope"))
	if err == nil {
		t.Fatalf("下游回非 0 業務碼時 handler 必須回 error（→ 重試三次進 DLQ）；" +
			"回 nil 代表 client 只看了 gRPC status，會拿到一個全零的快照")
	}
	// 陰性：一張金額為 0 的發票都不得留下來。
	if n := peerCount(t, env, `SELECT count(*) FROM invoice_documents`); n != 0 {
		t.Fatalf("document 列數 = %d，want 0", n)
	}
	// inbox 列**要**留著（去重鍵已落庫，重投會走 ON CONFLICT DO NOTHING）。
	if n := peerCount(t, env, `SELECT count(*) FROM invoice_inbox_events WHERE business_dedup_key = $1`,
		"ORDER_PAID:"+env.orderNo); n != 1 {
		t.Fatalf("inbox 列數 = %d，want 1", n)
	}
}
