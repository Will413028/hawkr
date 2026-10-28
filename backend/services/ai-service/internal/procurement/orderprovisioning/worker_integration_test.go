//go:build integration

package orderprovisioning

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// 自己的 shared-container-per-package／own-database-per-test harness：不能與隔壁
// package 共用（那些 pg 都是 package-scoped 的未匯出變數），所以 template DB 名字
// 也要各自不同，併發跑時容器／migration 才不會撞在一起。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_order_provisioning_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// testMetrics 只建一次。outbox.NewMetrics 走 promauto，同一個 outbox_name 在同一個
// 程序註冊第二次會 panic；名字刻意不是 production 的 "rfq_order_provisioning"。
var testMetrics = outbox.NewMetrics("rfq_order_provisioning_integration_test")

func setupTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	db, drop := pg.Clone(ctx, t)
	// 走 database.NewPool（production 用的同一條路），pool 的 QueryExecMode 才與
	// production 一致。
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		drop()
	}
}

type stubCreator struct {
	result RfqOrderResult
	err    error
	calls  int
}

func (s *stubCreator) CreateRfqOrder(_ context.Context, _ RfqOrderCommand) (RfqOrderResult, error) {
	s.calls++
	return s.result, s.err
}

type itFixture struct {
	pool         *pgxpool.Pool
	inner        *repository.Repo
	repo         *Repository
	acceptanceID int64
	outboxID     int64
}

// seed 種出一筆「已接受、等著建單」的真實狀態：acceptance 在 CREATING_ORDER、outbox
// 有它唯一的命令，外加 worker 會讀的那一版報價與 confirmed snapshot。
//
// ⚠️ 兩張唯讀表用原生 SQL 種：procurement schema 全檔零外鍵（實查 REFERENCES 無命中），
// 不需要把整條 accept 流程搬進本 package——那會讓這支測試綁在別條線的 usecase 上。
func seed(ctx context.Context, t *testing.T, pool *pgxpool.Pool, lane string, storeID *int64) *itFixture {
	t.Helper()
	var n int64
	inner := repository.New(pool, database.NewTxManager(pool), func() int64 { n++; return n })

	const (
		buyerID          = identity.BuyerID(9401)
		requestID        = int64(6001)
		publicationID    = int64(6002)
		sellerResponseID = int64(6003)
		snapshotID       = int64(6004)
	)
	// 排在過去，Worker 這一輪就撈得到（FetchPendingDue 的述詞是 next_attempt_at <= now）。
	due := time.Now().UTC().Add(-time.Minute)

	_, err := pool.Exec(ctx, `
		INSERT INTO procurement_seller_response_versions (
			id, seller_response_id, version, response_type,
			offered_quality_grade, offered_quality_inspection_notes,
			unit_basis_quantity, unit_basis_unit, offered_quantity, offered_quantity_unit,
			conversion_basis, currency, goods_amount, shipping_amount, tax_amount,
			tax_inclusion_basis, total_amount, valid_until, outside_buyer_requested_range,
			payload_hash, publication_version, seller_facing_payload_hash,
			client_request_id, submitted_at
		) VALUES (
			$1, $2, 1, 'MANUAL_QUOTE', 'A', '',
			1, '公斤', 200, '公斤',
			'PER_UNIT', 'TWD', 28000, 800, 2000,
			'TAX_INCLUDED', 30800, $3, false,
			'hash-version', 1, 'hash-seller-facing', 'seed-version', $4
		)`, int64(6005), sellerResponseID, due.Add(72*time.Hour), due)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO procurement_confirmed_rfq_snapshots (
			id, buyer_id, request_id, request_revision_id, price_preview_id, price_preview_version,
			quality_knowledge_state, canonical_payload_json, payload_hash,
			client_request_id, expires_at
		) VALUES ($1, $2, $3, 1, 1, 1, 'KNOWN',
			$4, 'hash-snapshot', 'seed-snapshot', $5)`,
		snapshotID, int64(buyerID), requestID,
		// ⚠️ 傳 string 不是 []byte：DSN 帶 default_query_exec_mode=exec，參數以文字送出，
		// []byte 會被編成 bytea 的 hex 字面值，jsonb 欄位收到就是 22P02。
		`{"item":"文旦柚","itemResolution":{"kind":"CATEGORY_CONFIRMED","label":"麻豆文旦"}}`,
		due.Add(72*time.Hour))
	require.NoError(t, err)

	acceptance, err := inner.InsertBuyerQuoteAcceptance(ctx, repository.InsertBuyerQuoteAcceptanceInput{
		BuyerID:                   buyerID,
		RequestID:                 requestID,
		PublicationID:             publicationID,
		SellerResponseID:          sellerResponseID,
		SellerResponseVersion:     1,
		SellerResponsePayloadHash: "hash-version",
		ConfirmedRfqSnapshotID:    snapshotID,
		ConfirmedRfqPayloadHash:   "hash-snapshot",
		Lane:                      lane,
		StoreID:                   storeID,
		ProspectID:                prospectFor(lane),
		AcceptedDealPrice:         30800,
		Currency:                  "TWD",
		GoodsAmount:               28000,
		ShippingAmount:            800,
		TaxAmount:                 2000,
		ClientRequestID:           "seed-accept",
		RequestHash:               "hash-version",
		AcceptedAt:                due,
	})
	require.NoError(t, err)
	require.Equal(t, repository.OrderProvisioningCreating, acceptance.OrderProvisioningState)

	payload, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"acceptanceId":  "1",
		"buyerId":       "9401",
		"requestId":     "6001",
	})
	require.NoError(t, err)
	require.NoError(t, inner.InsertOrderProvisioningOutbox(ctx, acceptance.ID, payload, due))

	repo := NewRepository(pool, database.NewTxManager(pool), inner)
	rows, err := repo.FetchPendingDue(ctx, time.Now().UTC(), 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	return &itFixture{pool: pool, inner: inner, repo: repo,
		acceptanceID: acceptance.ID, outboxID: rows[0].ID}
}

func prospectFor(lane string) *int64 {
	if lane != repository.SellerResponseLaneExternal {
		return nil
	}
	// chk_prbqa_identity_matches_lane：EXTERNAL 必有 prospect_id 且 store_id 為空。
	id := int64(7001)
	return &id
}

type outboxState struct {
	status       string
	attemptCount int32
	lastError    *string
	completedAt  *time.Time
}

func readOutbox(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) outboxState {
	t.Helper()
	var got outboxState
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, attempt_count, last_error, completed_at
		   FROM procurement_order_provisioning_outbox WHERE id = $1`, id,
	).Scan(&got.status, &got.attemptCount, &got.lastError, &got.completedAt))
	return got
}

func readAcceptance(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) (state string, orderNo *string, reason *string, deadline *time.Time) {
	t.Helper()
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT order_provisioning_state, order_no, order_failure_reason, payment_deadline
		   FROM procurement_buyer_quote_acceptances WHERE id = $1`, id,
	).Scan(&state, &orderNo, &reason, &deadline))
	return
}

func newWorkerFor(t *testing.T, f *itFixture, creator RfqOrderCreator, settlementStoreID int64) *Worker {
	t.Helper()
	return NewWorker(f.repo, f.inner, creator, settlementStoreID, testMetrics, zaptest.NewLogger(t))
}

// ① 成功路徑：建單 → acceptance 轉 ORDER_CREATED ＋ order_no ＋ payment_deadline，outbox 轉 done。
//
// ⛔ 判準是 DB 真的有那些值，不是「RunOnce 沒回錯」。
func TestWorker_Success_AttachesOrderAndMarksDone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	storeID := int64(2636840881)
	f := seed(ctx, t, pool, repository.SellerResponseLanePlatform, &storeID)
	deadline := time.Now().UTC().Add(12 * time.Hour).Truncate(time.Second)
	creator := &stubCreator{result: RfqOrderResult{Code: 0, OrderNo: "RFQ20260921001", PaymentDeadline: deadline}}

	require.NoError(t, newWorkerFor(t, f, creator, 0).RunOnce(ctx))

	require.Equal(t, 1, creator.calls)
	state, orderNo, reason, gotDeadline := readAcceptance(ctx, t, pool, f.acceptanceID)
	require.Equal(t, repository.OrderProvisioningCreated, state)
	require.NotNil(t, orderNo)
	require.Equal(t, "RFQ20260921001", *orderNo)
	require.Nil(t, reason, "成功路徑不得留下失敗成因")
	require.NotNil(t, gotDeadline)
	require.WithinDuration(t, deadline, gotDeadline.UTC(), time.Second)

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "done", got.status)
	require.NotNil(t, got.completedAt)
	require.EqualValues(t, 0, got.attemptCount)
}

// ② 終態拒絕：order-service 回 9505 ⇒ acceptance 轉 ORDER_FAILED ＋
// CONFIGURATION_MISSING，outbox 轉 done（不再重試）。
func TestWorker_TerminalRejection_WritesReasonAndStopsRetrying(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, repository.SellerResponseLaneExternal, nil)
	creator := &stubCreator{result: RfqOrderResult{Code: bizcode.PlatformCollectionNotConfigured, Message: "平台代收設定尚未完成"}}

	require.NoError(t, newWorkerFor(t, f, creator, 9001).RunOnce(ctx))

	state, orderNo, reason, _ := readAcceptance(ctx, t, pool, f.acceptanceID)
	require.Equal(t, repository.OrderProvisioningFailed, state)
	require.Nil(t, orderNo, "chk_prbqa_order_no_matches_state：非 ORDER_CREATED 必然沒有 order_no")
	require.NotNil(t, reason)
	require.Equal(t, FailureReasonConfigurationMissing, *reason)
	require.Equal(t, "done", readOutbox(ctx, t, pool, f.outboxID).status)
}

// ③ 重試耗盡：暫時性錯誤累積到 MaxAttempts ⇒ outbox 轉 failed，**同一交易內** acceptance
// 轉 ORDER_FAILED ＋ RETRY_EXHAUSTED。
//
// 這支才是 MarkFailed 原子性的證據：若把它改成 pkg/outbox.Hooks，兩者之間會出現
// 「outbox 已 failed、acceptance 仍 CREATING_ORDER」的窗口。
func TestWorker_RetryExhausted_MarksBothTerminalTogether(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	storeID := int64(2636840881)
	f := seed(ctx, t, pool, repository.SellerResponseLanePlatform, &storeID)
	creator := &stubCreator{err: errors.New("connection refused")}
	worker := newWorkerFor(t, f, creator, 0)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		require.NoError(t, worker.RunOnce(ctx))
		if attempt == maxAttempts {
			break
		}
		// backoff 把 next_attempt_at 推到未來；把它拉回過去，讓下一輪撈得到。
		_, err := pool.Exec(ctx,
			`UPDATE procurement_order_provisioning_outbox SET next_attempt_at = $2 WHERE id = $1`,
			f.outboxID, time.Now().UTC().Add(-time.Minute))
		require.NoError(t, err)
	}

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "failed", got.status)
	require.NotNil(t, got.lastError, "終態也要寫 last_error：細節只存在這裡（doc 64 §6.4.14）")

	state, _, reason, _ := readAcceptance(ctx, t, pool, f.acceptanceID)
	require.Equal(t, repository.OrderProvisioningFailed, state)
	require.NotNil(t, reason)
	require.Equal(t, FailureReasonRetryExhausted, *reason)
}

// ③b MarkFailed 的**原子性**本身：acceptance 那半寫失敗時，outbox 那半必須跟著 rollback。
//
// 🔴 為什麼要單獨一支：③ 只斷言「兩者最後都是終態」，而**非原子的實作也會通過它**
// （先寫 outbox、再寫 acceptance，兩句都成功時結果一模一樣）。能分辨兩者的只有失敗路徑——
// 這支把 acceptance 那半打成必然失敗，然後斷言 outbox 列**沒有**被留在 failed。
//
// 非原子的實作在這裡會留下「outbox 已 failed、acceptance 仍 CREATING_ORDER」，而那正是
// 本設計要消滅的窗口：CREATING_ORDER 會讓前端一直顯示「訂單建立中」，沒有東西推得動它。
type failingAcceptanceStore struct {
	AcceptanceStore
	err error
}

func (f failingAcceptanceStore) MarkAcceptanceOrderFailedByOutboxID(context.Context, int64, string) error {
	return f.err
}

func TestRepository_MarkFailed_RollsBackOutboxWhenAcceptanceWriteFails(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	storeID := int64(2636840881)
	f := seed(ctx, t, pool, repository.SellerResponseLanePlatform, &storeID)

	boom := errors.New("acceptance write exploded")
	repo := NewRepository(pool, database.NewTxManager(pool),
		failingAcceptanceStore{AcceptanceStore: f.inner, err: boom})

	err := repo.MarkFailed(ctx, f.outboxID, "whatever", time.Now().UTC())
	require.ErrorIs(t, err, boom, "第二句失敗必須回錯，不得吞掉")

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "pending", got.status,
		"⛔ 第二句失敗時第一句必須一起 ROLLBACK——留下 failed 就是非原子")
	require.Nil(t, got.completedAt)

	state, _, reason, _ := readAcceptance(ctx, t, pool, f.acceptanceID)
	require.Equal(t, repository.OrderProvisioningCreating, state)
	require.Nil(t, reason)
}

// ④ 重入：建單成功但 MarkDone 沒寫成 ⇒ 這一列重跑。
//
// 第 2 步的冪等回查必須擋住第二次建單——⛔ 不是靠 order-service 的 uk_orders_acceptance_id
// 兜底，那是第二道。斷言 creator 只被呼叫一次。
func TestWorker_Reentry_DoesNotCreateASecondOrder(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	storeID := int64(2636840881)
	f := seed(ctx, t, pool, repository.SellerResponseLanePlatform, &storeID)
	creator := &stubCreator{result: RfqOrderResult{Code: 0, OrderNo: "RFQ20260921001",
		PaymentDeadline: time.Now().UTC().Add(12 * time.Hour)}}
	worker := newWorkerFor(t, f, creator, 0)

	require.NoError(t, worker.RunOnce(ctx))
	require.Equal(t, 1, creator.calls)

	// 把 MarkDone 那一次寫入「收回去」——這正是建單成功之後、MarkDone 之前崩潰
	// （或 lease 到期）留下的狀態。
	_, err := pool.Exec(ctx,
		`UPDATE procurement_order_provisioning_outbox
		    SET status = 'pending', completed_at = NULL, next_attempt_at = $2 WHERE id = $1`,
		f.outboxID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)

	require.NoError(t, worker.RunOnce(ctx))
	require.Equal(t, 1, creator.calls, "acceptance 已是 ORDER_CREATED ⇒ 第 2 步就該收手")

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "done", got.status)
	require.EqualValues(t, 0, got.attemptCount, "0 列 CAS 與冪等回查都不得被當成錯誤")
}
