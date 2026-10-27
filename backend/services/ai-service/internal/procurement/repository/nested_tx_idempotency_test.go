//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// ---------------------------------------------------------------------------
// 「呼叫端已經在交易裡」的冪等回放
//
// 本檔三支測試釘住同一條不變式：以「INSERT 先試、撞具名 unique constraint 才回查既有
// 列」實作冪等的 repository 方法，在**呼叫端的交易內**被呼叫時仍然必須回放，不得回錯。
//
// 為什麼這件事需要機械保證：Postgres 的交易只要有一句語句失敗就整段進入 aborted
// （SQLSTATE 25P02），之後同一個交易裡的每一句——包含冪等路徑賴以復原的那次回查——都
// 會被拒絕。換句話說，冪等的實作本身在巢狀情境下會被它自己觸發的錯誤打掉，而且**只有
// 真的撞鍵時才會發作**：正常流程與現有測試（全部在交易外呼叫）都看不到。
//
// 修法是讓撞鍵根本不拋錯：每個方法的冪等 INSERT 都用 ON CONFLICT DO UPDATE
// （見 PostgreSQL 規範第十二章），交易因此始終健康，回查拿得到既有那一列。
//
// ⛔ 把任何一個方法的 ON CONFLICT 拿掉、退回「INSERT 先試、撞了才查」，對應的那支測試
// 就會在第二次呼叫拿到 25P02 而失敗——這才是這幾支測試守的東西，不是「回放會回同一個
// ID」本身（那件事交易外的既有測試已經涵蓋）。
// ---------------------------------------------------------------------------

// callerTxProbe 是「外層交易到現在還活著」的證據載體。
//
// 光斷言第二次呼叫沒回錯不夠：那隻證明了那一支方法自己有回放，證不到外層交易沒有被
// 連坐 abort。所以每支測試都在撞鍵之後再寫一列、再讓外層交易 commit，最後回連線池
// 確認那一列真的落地——若外層交易被 abort，這次寫入會回 25P02，commit 會被當成
// ROLLBACK，兩種結果都會讓測試紅。
type callerTxProbe struct {
	beforeRequestID int64
	afterRequestID  int64
}

func (p callerTxProbe) requireBothCommitted(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	require.NotZero(t, p.beforeRequestID)
	require.NotZero(t, p.afterRequestID)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_requests WHERE id = ANY($1::bigint[])`,
		[]int64{p.beforeRequestID, p.afterRequestID}).Scan(&count))
	require.Equal(t, 2, count,
		"撞鍵前後兩次寫入都必須隨外層交易 commit——只要外層被連坐 abort，"+
			"撞鍵之後那一列寫不進去、commit 也會被當成 ROLLBACK")
}

func nestedTxIdempotencySnapshotInput(clientRequestID string) InsertConfirmedRfqSnapshotInput {
	return InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		PricePreviewID: 1, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"寶島梨"}`),
		PayloadHash:          "hash-snapshot", ClientRequestID: clientRequestID,
		ExpiresAt: refTime().Add(24 * time.Hour),
	}
}

// InsertConfirmedRfqSnapshot 在呼叫端交易內重送同一把 client_request_id 必須回放。
func TestInsertConfirmedRfqSnapshot_ReplaysInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := nestedTxIdempotencySnapshotInput("confirm-key-nested")

	var probe callerTxProbe
	var first, second ConfirmedRfqSnapshot
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if probe.beforeRequestID, err = probeWrite(txCtx, repo); err != nil {
			return err
		}
		if first, err = repo.InsertConfirmedRfqSnapshot(txCtx, in); err != nil {
			return err
		}
		if second, err = repo.InsertConfirmedRfqSnapshot(txCtx, in); err != nil {
			return err
		}
		probe.afterRequestID, err = probeWrite(txCtx, repo)
		return err
	})
	require.NoError(t, err,
		"同一把 client_request_id 在呼叫端交易內重送必須回放既有列，不得讓撞鍵把整個交易打成 aborted")
	require.Equal(t, first.ID, second.ID, "重送必須回同一列")

	probe.requireBothCommitted(ctx, t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_confirmed_rfq_snapshots WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), in.ClientRequestID).Scan(&count))
	require.Equal(t, 1, count, "回放不得建立第二筆")
}

// 撞鍵之後的內容比對（ErrConfirmedRfqSnapshotIdempotencyConflict）在交易內同樣要成立：
// ON CONFLICT 讓這次 INSERT 不留痕跡也不拋錯，回查與比對走的仍是原本那條路徑。
func TestInsertConfirmedRfqSnapshot_ConflictInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := nestedTxIdempotencySnapshotInput("confirm-key-nested-conflict")
	drifted := in
	drifted.PricePreviewID = 99

	var conflictErr error
	var probe callerTxProbe
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if probe.beforeRequestID, err = probeWrite(txCtx, repo); err != nil {
			return err
		}
		if _, err = repo.InsertConfirmedRfqSnapshot(txCtx, in); err != nil {
			return err
		}
		_, conflictErr = repo.InsertConfirmedRfqSnapshot(txCtx, drifted)
		probe.afterRequestID, err = probeWrite(txCtx, repo)
		return err
	})
	require.NoError(t, err)
	require.ErrorIs(t, conflictErr, ErrConfirmedRfqSnapshotIdempotencyConflict,
		"同 key 換內容在交易內也必須是 conflict，不是 25P02")
	probe.requireBothCommitted(ctx, t, pool)
}

// InsertPublicationApproval 在呼叫端交易內重送同一把 preview_client_request_id 必須回放。
//
// 這一支跟另外兩支的形狀不同：它本身已經用 r.txManager.WithTx 把 approval ＋ ordered
// recipient list 綁成一個原子單位，而 WithTx 巢狀時是**沿用**外層交易（不開新的），
// 所以撞鍵照樣是打在外層交易上。
func TestInsertPublicationApproval_ReplaysInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := seedApprovalInput(ctx, t, repo)

	var probe callerTxProbe
	var first, second PublicationApproval
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if probe.beforeRequestID, err = probeWrite(txCtx, repo); err != nil {
			return err
		}
		if first, err = repo.InsertPublicationApproval(txCtx, in); err != nil {
			return err
		}
		if second, err = repo.InsertPublicationApproval(txCtx, in); err != nil {
			return err
		}
		probe.afterRequestID, err = probeWrite(txCtx, repo)
		return err
	})
	require.NoError(t, err,
		"同一把 preview_client_request_id 在呼叫端交易內重送必須回放既有列")
	require.Equal(t, first.ID, second.ID, "重送必須回同一列")

	probe.requireBothCommitted(ctx, t, pool)

	var approvals, recipients int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_approvals WHERE buyer_id = $1 AND preview_client_request_id = $2`,
		int64(testBuyerID), in.PreviewClientRequestID).Scan(&approvals))
	require.Equal(t, 1, approvals, "回放不得建立第二筆 approval")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_approval_recipients WHERE approval_id = $1`,
		first.ID).Scan(&recipients))
	require.Equal(t, len(in.Recipients), recipients,
		"回放不得重寫第二份 recipients，也不得吃掉第一份")
}

// ClaimPublicationCommand 在呼叫端交易內重送同一把 client_request_id 必須回放。
//
// ⛔ 這一支刻意**不**經過 usecase 的編排路徑（publication_execute.go 的
// claimCommand）：Slice 2a 在呼叫端解掉的是它自己那條路徑，這裡問的是方法本身在
// 沒有呼叫端保護時成不成立——因為「沒有人會這樣呼叫」不是機械保證。
func TestClaimPublicationCommand_ReplaysInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := seedPublicationCommandInput(ctx, t, repo)

	var probe callerTxProbe
	var first, second PublicationCommand
	var firstIsNew, secondIsNew bool
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if probe.beforeRequestID, err = probeWrite(txCtx, repo); err != nil {
			return err
		}
		if first, firstIsNew, err = repo.ClaimPublicationCommand(txCtx, in); err != nil {
			return err
		}
		if second, secondIsNew, err = repo.ClaimPublicationCommand(txCtx, in); err != nil {
			return err
		}
		probe.afterRequestID, err = probeWrite(txCtx, repo)
		return err
	})
	require.NoError(t, err,
		"同一把 client_request_id 在呼叫端交易內重送必須回放既有 claim")
	require.True(t, firstIsNew)
	require.False(t, secondIsNew, "第二次是冪等命中，不是新 claim")
	require.Equal(t, first.ID, second.ID)

	probe.requireBothCommitted(ctx, t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), in.ClientRequestID).Scan(&count))
	require.Equal(t, 1, count, "回放不得建立第二筆")
}

// 同 key 換 canonical hash 在交易內同樣是 conflict（#2450 Scope 7），零 mutation。
func TestClaimPublicationCommand_ConflictInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := seedPublicationCommandInput(ctx, t, repo)
	drifted := in
	drifted.CanonicalRequestHash = "canon-hash-DIFFERENT"

	var conflictErr error
	var probe callerTxProbe
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if probe.beforeRequestID, err = probeWrite(txCtx, repo); err != nil {
			return err
		}
		if _, _, err = repo.ClaimPublicationCommand(txCtx, in); err != nil {
			return err
		}
		_, _, conflictErr = repo.ClaimPublicationCommand(txCtx, drifted)
		probe.afterRequestID, err = probeWrite(txCtx, repo)
		return err
	})
	require.NoError(t, err)
	require.ErrorIs(t, conflictErr, ErrPublicationCommandConflict,
		"同 key 換內容在交易內也必須是 conflict，不是 25P02")
	probe.requireBothCommitted(ctx, t, pool)
}

// probeWrite 在目前的交易裡寫一列 procurement_requests，回傳它的 id。用 CreateRequest
// 而不是裸 SQL：它是這個 repo 真正的寫入路徑（WithTx 巢狀時沿用外層交易），寫得進去就
// 代表外層交易此刻是可寫的。
func probeWrite(ctx context.Context, repo *Repo) (int64, error) {
	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	if err != nil {
		return 0, err
	}
	return req.ID, nil
}

// ---------------------------------------------------------------------------
// 其餘四個同形位置（2026-09-08 coordinator 掃描：整個 package 共七處同形，上面三處
// 已修）。它們分成兩種形狀，測試方式必須跟著不同——不是為了「一致」硬套同一支測試。
//
// 形狀一（InsertPricePreview、CreateCandidateSet）：交易內**沒有**冪等鍵預檢，直接
// INSERT。跟上面三處完全同形，序列重送就會撞鍵，所以「同一個交易內連呼叫兩次」就紅得
// 起來。
//
// 形狀二（ClaimRun、AppendRevision、RewindTo）：交易**內**先 SELECT 一次冪等鍵，命中
// 就直接回放、根本不 INSERT。所以序列重送（含巢狀）走的是預檢那條路，撞不到 23505——
// 「同一個交易內連呼叫兩次」對它們是**綠的**，寫成那樣等於寫了一支證明不了任何事的測試。
// 它們真正的撞鍵路徑是**併發**：兩個交易都在對方 commit 前預檢落空，輸的那個在 INSERT
// 撞 23505。那條路徑上的復原查詢（recoverIdempotentAppend／recoverClaimedRun）用的是
// r.q(ctx)——ctx 帶交易時就是**呼叫端的**交易，於是同樣拿 25P02。
//
// 形狀二的測試因此照抄本 package 既有的 race 佈局（run_test.go 的
// TestClaimRun_ConcurrentSameKey_UniqueViolationRecovered）：手動驅動贏家那一側、停在
// COMMIT 之前，讓輸家真的卡在唯一索引的 value lock 上（用 pg_stat_activity 輪詢確認，
// 不用 sleep），再 commit 贏家。唯一的差別是**輸家跑在一個呼叫端交易裡**。
// ---------------------------------------------------------------------------

// waitUntilBlockedOn 輪詢 pg_stat_activity，直到有 backend 真的卡在 table 相關語句的
// Lock 上。形狀取自 run_test.go 既有的同一段輪詢（不是 sleep：race window 有沒有真的
// 打開是可觀測的事實，用猜的會讓測試在快機器上假綠）。
func waitUntilBlockedOn(ctx context.Context, t *testing.T, pool *pgxpool.Pool, table string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND state = 'active'
			  AND query ILIKE '%' || $1 || '%'
		`, table).Scan(&blocked); err != nil {
			return false
		}
		return blocked > 0
	}, 10*time.Second, 5*time.Millisecond,
		"輸家必須真的卡在 "+table+" 的唯一索引 value lock 上，這個測試才有意義")
}

// InsertPricePreview 在呼叫端交易內重送同一把 client_request_id 必須回放。
// 形狀一：跟 InsertConfirmedRfqSnapshot 逐行同形。
func TestInsertPricePreview_ReplaysInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	in := InsertPricePreviewInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricingMode: "PRICEBOOK_GUIDED", QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		ReferenceAvailability:   "AVAILABLE",
		BuyerRequestedRangeJSON: json.RawMessage(`{"min":"80","max":"120"}`),
		DealTemperatureJSON:     json.RawMessage(`{"status":"UNKNOWN"}`),
		ClientRequestID:         "preview-key-nested",
		PreviewInputHash:        "nested-input-hash",
		PayloadHash:             "nested-payload-hash",
		ExpiresAt:               refTime().Add(time.Hour),
	}

	var probe callerTxProbe
	var first, second PricePreview
	err = tm.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		if probe.beforeRequestID, txErr = probeWrite(txCtx, repo); txErr != nil {
			return txErr
		}
		if first, txErr = repo.InsertPricePreview(txCtx, in); txErr != nil {
			return txErr
		}
		if second, txErr = repo.InsertPricePreview(txCtx, in); txErr != nil {
			return txErr
		}
		probe.afterRequestID, txErr = probeWrite(txCtx, repo)
		return txErr
	})
	require.NoError(t, err,
		"同一把 client_request_id 在呼叫端交易內重送必須回放既有列")
	require.Equal(t, first.ID, second.ID, "重送必須回同一列")

	probe.requireBothCommitted(ctx, t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_price_previews WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), in.ClientRequestID).Scan(&count))
	require.Equal(t, 1, count, "回放不得建立第二筆")
}

// CreateCandidateSet 在呼叫端交易內重送同一組 (request_revision_id, policy_version)
// 必須回放。形狀一：WithTx 內兩句寫入、沒有冪等鍵預檢。
func TestCreateCandidateSet_ReplaysInsideCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	in := CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: testBuyerID,
		PolicyVersion: 1, LeaseOwner: "worker-nested", LeaseTTL: 30 * time.Second,
	}

	var probe callerTxProbe
	var first, second CandidateSet
	err = tm.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		if probe.beforeRequestID, txErr = probeWrite(txCtx, repo); txErr != nil {
			return txErr
		}
		if first, txErr = repo.CreateCandidateSet(txCtx, in); txErr != nil {
			return txErr
		}
		if second, txErr = repo.CreateCandidateSet(txCtx, in); txErr != nil {
			return txErr
		}
		probe.afterRequestID, txErr = probeWrite(txCtx, repo)
		return txErr
	})
	require.NoError(t, err,
		"同一組冪等鍵在呼叫端交易內重送必須回放既有 candidate set")
	require.Equal(t, first.ID, second.ID, "重送必須回同一列")

	probe.requireBothCommitted(ctx, t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidate_sets WHERE request_revision_id = $1 AND policy_version = 1`,
		rev.ID).Scan(&count))
	require.Equal(t, 1, count, "回放不得建立第二筆")
}

// ClaimRun 在呼叫端交易內輸掉冪等鍵 race 時必須回放，而不是把呼叫端交易打成 aborted。
// 形狀二：序列重送走交易內預檢、撞不到鍵，所以這裡必須佈一個真的 race。
func TestClaimRun_ConcurrentLoserInsideCallerTransactionReplays(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	in := ClaimRunInput{
		BuyerID: testBuyerID, ClientRequestID: "claimrun-nested-race", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-nested", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	}

	// 贏家：手動驅動到「冪等鍵那一列已寫、尚未 commit」，形狀取自 run_test.go 的
	// TestClaimRun_ConcurrentSameKey_UniqueViolationRecovered。
	winnerTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = winnerTx.Rollback(ctx) })
	winnerQ := gen.New(winnerTx)

	winnerRunID := repo.newID()
	_, err = winnerQ.CreateProcurementAgentRun(ctx, gen.CreateProcurementAgentRunParams{
		ID: winnerRunID, BuyerID: int64(testBuyerID),
		RunType: string(in.RunType), Status: domain.RunStatusQueued(), LastEventSeq: 1,
		ControllerKind: string(in.ControllerKind),
	})
	require.NoError(t, err)
	_, err = winnerQ.CreateProcurementAgentRunCommand(ctx, gen.CreateProcurementAgentRunCommandParams{
		ID: repo.newID(), BuyerID: int64(testBuyerID), CommandKind: in.CommandKind,
		ClientRequestID: in.ClientRequestID, PayloadHash: in.PayloadHash,
		RunID: winnerRunID, Status: "ACCEPTED",
	})
	require.NoError(t, err)

	// 輸家：跑在一個**呼叫端交易**裡——這正是與 run_test.go 那支既有測試唯一的差別。
	type loserResult struct {
		run   Run
		isNew bool
		probe callerTxProbe
		err   error
	}
	resultCh := make(chan loserResult, 1)
	go func() {
		var res loserResult
		res.err = tm.WithTx(ctx, func(txCtx context.Context) error {
			var txErr error
			if res.probe.beforeRequestID, txErr = probeWrite(txCtx, repo); txErr != nil {
				return txErr
			}
			if res.run, res.isNew, txErr = repo.ClaimRun(txCtx, in); txErr != nil {
				return txErr
			}
			res.probe.afterRequestID, txErr = probeWrite(txCtx, repo)
			return txErr
		})
		resultCh <- res
	}()

	waitUntilBlockedOn(ctx, t, pool, "procurement_agent_run_commands")
	require.NoError(t, winnerTx.Commit(ctx))

	res := <-resultCh
	require.NoError(t, res.err,
		"輸掉 race 的那一次 claim 必須完成回放——它跑在呼叫端交易裡，撞鍵不得把那個交易打成 aborted")
	require.Equal(t, winnerRunID, res.run.ID, "回放出來的 run 必須是贏家那一個")
	require.False(t, res.isNew, "輸家沒有建立新 run")
	res.probe.requireBothCommitted(ctx, t, pool)

	var cmdCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), in.ClientRequestID).Scan(&cmdCount))
	require.Equal(t, 1, cmdCount, "輸家那一段必須完整回捲，不留第二筆 run command")
}

// seedAnswerPatchWinner 開一個未 commit 的交易，把 (buyer_id, client_request_id) 這把
// 冪等鍵先佔住（procurement_answer_patches），回傳那個交易讓呼叫端決定何時 commit。
func seedAnswerPatchWinner(
	ctx context.Context, t *testing.T, pool *pgxpool.Pool, repo *Repo,
	requestID, resultRevisionID int64, clientRequestID string,
) pgx.Tx {
	t.Helper()
	winnerTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = winnerTx.Rollback(ctx) })

	_, err = gen.New(winnerTx).CreateProcurementAnswerPatch(ctx, gen.CreateProcurementAnswerPatchParams{
		ID: repo.newID(), RequestID: requestID, BuyerID: int64(testBuyerID),
		ResultRevisionID: resultRevisionID, Source: "LLM_EXTRACT",
		PayloadJson: []byte(`{}`), ClientRequestID: clientRequestID,
	})
	require.NoError(t, err)
	return winnerTx
}

// AppendRevision 在呼叫端交易內輸掉冪等鍵 race 時必須回放。形狀二。
func TestAppendRevision_ConcurrentLoserInsideCallerTransactionReplays(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("巢狀 race")}, refTime())

	const key = "append-nested-race"
	winnerTx := seedAnswerPatchWinner(ctx, t, pool, repo, req.ID, rev.ID, key)

	in := AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: key, PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	}

	type loserResult struct {
		revision Revision
		probe    callerTxProbe
		err      error
	}
	resultCh := make(chan loserResult, 1)
	go func() {
		var res loserResult
		res.err = tm.WithTx(ctx, func(txCtx context.Context) error {
			var txErr error
			if res.probe.beforeRequestID, txErr = probeWrite(txCtx, repo); txErr != nil {
				return txErr
			}
			if res.revision, txErr = repo.AppendRevision(txCtx, in); txErr != nil {
				return txErr
			}
			res.probe.afterRequestID, txErr = probeWrite(txCtx, repo)
			return txErr
		})
		resultCh <- res
	}()

	waitUntilBlockedOn(ctx, t, pool, "procurement_answer_patches")
	require.NoError(t, winnerTx.Commit(ctx))

	res := <-resultCh
	require.NoError(t, res.err,
		"輸掉 race 的 AppendRevision 必須完成回放——它跑在呼叫端交易裡，撞鍵不得把那個交易打成 aborted")
	require.Equal(t, rev.ID, res.revision.ID, "回放出來的必須是贏家那一列 answer_patch 指向的 revision")
	res.probe.requireBothCommitted(ctx, t, pool)

	var patches int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_answer_patches WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), key).Scan(&patches))
	require.Equal(t, 1, patches, "輸家那一段必須完整回捲，不留第二筆 answer patch")
}

// RewindTo 在呼叫端交易內輸掉冪等鍵 race 時必須回放。形狀二，與 AppendRevision 共用
// 同一把冪等鍵與同一條復原路徑（recoverIdempotentAppend）。
func TestRewindTo_ConcurrentLoserInsideCallerTransactionReplays(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	tm := database.NewTxManager(pool)
	repo := New(pool, tm, testIDGen())

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	const key = "rewind-nested-race"
	winnerTx := seedAnswerPatchWinner(ctx, t, pool, repo, req.ID, rev.ID, key)

	in := RewindInput{
		RequestID: req.ID, BuyerID: testBuyerID, TargetRevisionID: rev.ID,
		ClientRequestID: key, PayloadJSON: []byte(`{}`),
	}

	type loserResult struct {
		revision Revision
		probe    callerTxProbe
		err      error
	}
	resultCh := make(chan loserResult, 1)
	go func() {
		var res loserResult
		res.err = tm.WithTx(ctx, func(txCtx context.Context) error {
			var txErr error
			if res.probe.beforeRequestID, txErr = probeWrite(txCtx, repo); txErr != nil {
				return txErr
			}
			if res.revision, txErr = repo.RewindTo(txCtx, in); txErr != nil {
				return txErr
			}
			res.probe.afterRequestID, txErr = probeWrite(txCtx, repo)
			return txErr
		})
		resultCh <- res
	}()

	waitUntilBlockedOn(ctx, t, pool, "procurement_answer_patches")
	require.NoError(t, winnerTx.Commit(ctx))

	res := <-resultCh
	require.NoError(t, res.err,
		"輸掉 race 的 RewindTo 必須完成回放——它跑在呼叫端交易裡，撞鍵不得把那個交易打成 aborted")
	require.Equal(t, rev.ID, res.revision.ID, "回放出來的必須是贏家那一列 answer_patch 指向的 revision")
	res.probe.requireBothCommitted(ctx, t, pool)

	var patches int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_answer_patches WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), key).Scan(&patches))
	require.Equal(t, 1, patches, "輸家那一段必須完整回捲，不留第二筆 answer patch")
}
