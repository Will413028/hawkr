//go:build integration

package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

const (
	acceptRaceBuyerID      = identity.BuyerID(4321)
	acceptRaceStoreID      = int64(8300)
	acceptRaceSnapshotHash = "accept-race-snapshot-hash"
)

func acceptRaceIDGen() func() int64 {
	var n atomic.Int64
	n.Store(9000)
	return func() int64 { return n.Add(1) }
}

type acceptRaceFixture struct {
	repo       *repository.Repo
	tx         database.TxManager
	pool       *pgxpool.Pool
	requestID  int64
	identityID int64
}

// seedAcceptableSellerResponse 種出一份「買家現在就能接受」的真實狀態：一張
// publication、一份 confirmed snapshot、一列 seller response identity（latest=1）與
// 對應的 immutable version。procurement 的表刻意沒有 FK（見 schema 註解），所以
// approval／command／preview 用不衝突的常數 ID 即可。
func seedAcceptableSellerResponse(ctx context.Context, t *testing.T, pool *pgxpool.Pool) acceptRaceFixture {
	t.Helper()

	tm := database.NewTxManager(pool)
	repo := repository.New(pool, tm, acceptRaceIDGen())

	req, rev, err := repo.CreateRequest(ctx, acceptRaceBuyerID)
	require.NoError(t, err)

	_, err = repo.InsertConfirmedRfqSnapshot(ctx, repository.InsertConfirmedRfqSnapshotInput{
		BuyerID: acceptRaceBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricePreviewID: 8001, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"芭樂","quantity":100,"unit":"KG"}`),
		PayloadHash:          acceptRaceSnapshotHash,
		ClientRequestID:      "accept-race-snapshot",
		ExpiresAt:            sellerResponseTestNow.Add(time.Hour),
	})
	require.NoError(t, err)

	// InsertConfirmedRfqSnapshot 自己配 ID，publication 要指回那一列。
	var snapID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_confirmed_rfq_snapshots WHERE payload_hash = $1`,
		acceptRaceSnapshotHash).Scan(&snapID))

	pub, err := repo.InsertPublication(ctx, repository.InsertPublicationInput{
		BuyerID: acceptRaceBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		ApprovalID: 8401, ApprovalVersion: 1, CommandID: 8501,
		ConfirmedRfqSnapshotID:  snapID,
		SellerFacingPayloadHash: "accept-race-seller-facing-hash",
		AudienceMode:            "DIRECT_RECIPIENTS", AudienceHash: "accept-race-audience-hash",
		State: "PUBLISHED", SchemaVersion: "hawkr.agent.publication.v1",
		PublishedAt: sellerResponseTestNow,
	})
	require.NoError(t, err)

	id, err := repo.EnsureSellerResponseIdentityForStore(ctx, pub.ID, acceptRaceBuyerID, req.ID, acceptRaceStoreID)
	require.NoError(t, err)

	_, err = repo.InsertSellerResponseVersion(ctx, repository.InsertSellerResponseVersionInput{
		SellerResponseID: id.ID, Version: 1, ResponseType: "MANUAL_QUOTE",
		OfferedQualityGrade: "A", OfferedQualityInspectionNotes: "整箱檢視",
		UnitBasisQuantity: "1", UnitBasisUnit: "KG",
		OfferedQuantity: "100", OfferedQuantityUnit: "KG", ConversionBasis: "DIRECT",
		Currency: "TWD", GoodsAmount: 30000, ShippingAmount: 800, TaxAmount: 0,
		TaxInclusionBasis: "TAX_EXCLUDED", TotalAmount: 30800,
		ValidUntil:  sellerResponseTestNow.Add(24 * time.Hour),
		PayloadHash: "accept-race-version-1", PublicationVersion: 1,
		SellerFacingPayloadHash: "accept-race-seller-facing-hash",
		ClientRequestID:         "accept-race-quote-1", SubmittedAt: sellerResponseTestNow,
	})
	require.NoError(t, err)

	bumped, err := repo.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, bumped.LatestVersion)

	return acceptRaceFixture{repo: repo, tx: tm, pool: pool, requestID: req.ID, identityID: id.ID}
}

func (f acceptRaceFixture) useCase() *BuyerAcceptanceUseCase {
	return NewBuyerAcceptanceUseCase(f.repo, f.tx, nil, sellerResponseNow)
}

func (f acceptRaceFixture) acceptInput(clientRequestID string) AcceptSellerQuoteInput {
	return AcceptSellerQuoteInput{
		BuyerID: acceptRaceBuyerID, RequestID: f.requestID, ClientRequestID: clientRequestID,
		SellerResponseID: f.identityID, ExpectedSellerResponseVersion: 1,
		ExpectedRfqSnapshotHash: acceptRaceSnapshotHash,
	}
}

func (f acceptRaceFixture) count(ctx context.Context, t *testing.T, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(ctx, sql, f.identityID).Scan(&n))
	return n
}

func (f acceptRaceFixture) acceptanceCount(ctx context.Context, t *testing.T) int {
	return f.count(ctx, t, `SELECT count(*) FROM procurement_buyer_quote_acceptances WHERE seller_response_id = $1`)
}

func (f acceptRaceFixture) outboxCount(ctx context.Context, t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_order_provisioning_outbox o
		 JOIN procurement_buyer_quote_acceptances a ON a.id = o.acceptance_id
		 WHERE a.seller_response_id = $1`, f.identityID).Scan(&n))
	return n
}

// awaitRowLockWaiter 等到「有另一個 backend 正在等鎖」為止。這是真的 barrier：
// 觀察 pg_stat_activity 的等待狀態，不是猜一個 sleep 長度。
//
// ⛔ 不在這裡用 require：它會在非測試 goroutine 上 Goexit，把測試掛成逾時而不是
// 失敗（第一版就是這樣卡滿 10 分鐘）。錯誤一律往上回，由測試 goroutine 判定。
func awaitRowLockWaiter(ctx context.Context, pool *pgxpool.Pool, deadline time.Duration) (bool, error) {
	stop := time.Now().Add(deadline)
	for time.Now().Before(stop) {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity
			 WHERE wait_event_type = 'Lock' AND state = 'active' AND pid <> pg_backend_pid()`,
		).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
		time.Sleep(10 * time.Millisecond) // poll interval，不是用來等競態發生的猜測值
	}
	return false, nil
}

// runAcceptanceUnderSellerWrite 讓賣家側先鎖住 identity 列，acceptance 在鎖上等，
// 賣家提交自己的寫入後才放行。mutate 是賣家那一側要做的事。
//
// 賣家的 transaction 跑在測試 goroutine 上，只有 acceptance 另開 goroutine，而且
// 那個 goroutine 只把結果塞進 channel，不做任何斷言。
func runAcceptanceUnderSellerWrite(
	ctx context.Context, t *testing.T, f acceptRaceFixture,
	mutate func(txCtx context.Context, locked repository.SellerResponseIdentity) error,
) error {
	t.Helper()

	acceptErr := make(chan error, 1)
	var lockObserved bool

	txErr := f.tx.WithTx(ctx, func(txCtx context.Context) error {
		locked, lockErr := f.repo.FindSellerResponseByIDForUpdate(txCtx, f.identityID)
		if lockErr != nil {
			return lockErr
		}
		if locked.LatestVersion != 1 {
			return fmt.Errorf("seed latest_version = %d, want 1", locked.LatestVersion)
		}

		// 鎖已握在手上，這時才放 acceptance 進場；它必須卡在鎖上。
		go func() {
			_, err := f.useCase().AcceptSellerQuote(ctx, f.acceptInput("accept-race-key"))
			acceptErr <- err
		}()

		observed, waitErr := awaitRowLockWaiter(ctx, f.pool, 5*time.Second)
		if waitErr != nil {
			return waitErr
		}
		lockObserved = observed
		// ⛔ 傳鎖定當下讀到的那一列：identity 的樂觀 version 每次更新都會遞增，
		// 種子自己的 bump 已經動過一次，寫死常數會誤判成 stale。
		return mutate(txCtx, locked)
	})
	require.NoError(t, txErr)
	require.True(t, lockObserved,
		"acceptance 沒有在 identity 列的鎖上等待 ⇒ CAS 讀不在交易內，賣家仍可在讀與寫之間 commit")

	select {
	case err := <-acceptErr:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("acceptance did not finish after the seller transaction committed")
		return nil
	}
}

// TestAcceptanceAtomic_SellerRevisionDuringAcceptance：賣家在買家按下接受的同時改版。
// doc 64 §2-3「不得讓接受落在一個已被 supersede 的版本上而不報錯」。CAS 讀在交易外時，
// acceptance 讀到 latest=1、賣家隨後 commit latest=2，兩邊都成功。
func TestAcceptanceAtomic_SellerRevisionDuringAcceptance(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	t.Cleanup(cleanup)
	f := seedAcceptableSellerResponse(ctx, t, pool)

	err := runAcceptanceUnderSellerWrite(ctx, t, f, func(txCtx context.Context, locked repository.SellerResponseIdentity) error {
		if _, insertErr := f.repo.InsertSellerResponseVersion(txCtx, repository.InsertSellerResponseVersionInput{
			SellerResponseID: f.identityID, Version: 2, ResponseType: "MANUAL_QUOTE",
			OfferedQualityGrade: "A", OfferedQualityInspectionNotes: "整箱檢視",
			UnitBasisQuantity: "1", UnitBasisUnit: "KG",
			OfferedQuantity: "100", OfferedQuantityUnit: "KG", ConversionBasis: "DIRECT",
			Currency: "TWD", GoodsAmount: 31000, ShippingAmount: 800, TaxAmount: 0,
			TaxInclusionBasis: "TAX_EXCLUDED", TotalAmount: 31800,
			ValidUntil:  sellerResponseTestNow.Add(24 * time.Hour),
			PayloadHash: "accept-race-version-2", PublicationVersion: 1,
			SellerFacingPayloadHash: "accept-race-seller-facing-hash",
			ClientRequestID:         "accept-race-quote-2", SubmittedAt: sellerResponseTestNow,
		}); insertErr != nil {
			return insertErr
		}
		_, bumpErr := f.repo.BumpSellerResponseLatestVersion(txCtx, f.identityID, locked.Version, 2)
		return bumpErr
	})

	require.ErrorIs(t, err, repository.ErrSellerResponseStale,
		"接受一個已被 supersede 的版本必須報錯")
	require.Zero(t, f.acceptanceCount(ctx, t), "被拒絕的接受不得留下 acceptance")
	require.Zero(t, f.outboxCount(ctx, t), "被拒絕的接受不得留下建單命令")
}

// TestAcceptanceAtomic_WithdrawDuringAcceptance：同一條路徑的撤回版本。
func TestAcceptanceAtomic_WithdrawDuringAcceptance(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	t.Cleanup(cleanup)
	f := seedAcceptableSellerResponse(ctx, t, pool)

	err := runAcceptanceUnderSellerWrite(ctx, t, f, func(txCtx context.Context, locked repository.SellerResponseIdentity) error {
		_, withdrawErr := f.repo.WithdrawSellerResponse(
			txCtx, f.identityID, locked.Version, locked.LatestVersion)
		return withdrawErr
	})

	require.ErrorIs(t, err, repository.ErrSellerResponseStale,
		"接受一個已撤回的版本必須報錯")
	require.Zero(t, f.acceptanceCount(ctx, t))
	require.Zero(t, f.outboxCount(ctx, t))
}

// TestAcceptanceAtomic_NoContentionStillSucceeds 是正控制：沒有競爭時照樣成交，
// 且 acceptance 與建單命令各一列。少了它，「全部都拒絕」也會讓上面兩條變綠。
func TestAcceptanceAtomic_NoContentionStillSucceeds(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	t.Cleanup(cleanup)
	f := seedAcceptableSellerResponse(ctx, t, pool)

	got, err := f.useCase().AcceptSellerQuote(ctx, f.acceptInput("accept-nocontention-key"))
	require.NoError(t, err)
	require.False(t, got.Replayed)
	require.EqualValues(t, 30800, got.Acceptance.AcceptedDealPrice)
	require.Equal(t, 1, f.acceptanceCount(ctx, t))
	require.Equal(t, 1, f.outboxCount(ctx, t))
}
