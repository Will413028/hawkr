//go:build integration

package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ---------------------------------------------------------------------------
// ExecuteRfqPublication 的真 DB 驗證（#2450 Scope 6）
//
// ⛔ 這三支測試存在的理由是 publication_execute_test.go 的 fake **證明不了**的兩件事：
//
//  1. **原子性**：任一步失敗時，publication、command、ledger run 與 approval 的
//     PENDING → EXECUTING CAS 必須一起消失。fake 的 rollback 是我們自己寫的模型，
//     真正的 ROLLBACK 只有真 Postgres 給得出來。
//  2. **併發 exactly-one**：兩個同時的 execute 只能產生一張 publication。仲裁者是
//     Postgres 的列鎖 ＋ CAS 述詞在鎖釋放後的重新求值，fake 沒有列鎖。
//
// 第三支（冪等回放）驗的是撞冪等鍵在真 DB 上不打爆交易——見該支測試自己的說明。
// ---------------------------------------------------------------------------

const (
	pubExecBuyerID   = identity.BuyerID(4477)
	pubExecCandSetID = int64(910)

	// pubExecCandSetPolicyVersion 與 pubExecCandSetID 一起進 audience preimage，
	// 所以它們必須與寫進 approval 的值逐字相同，否則重算出來的 hash 對不上。
	pubExecCandSetPolicyVersion = 1

	// pubExecBarrierTimeout 是併發測試裡兩道等待的上限。到期不是「就當作沒事」——
	// 兩處都有對應的斷言會因此變紅（見各自的說明），寧可紅得清楚也不要 hang。
	pubExecBarrierTimeout = 20 * time.Second
)

// pubExecNow 是三支測試共用的固定時鐘：approval 的 expires_at 相對它設定，
// 「還沒過期」因此是確定的，不受測試機器速度或時區影響（CLAUDE.md §21）。
var pubExecNow = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

// pubExecRecipients 是核准當下那份 **ordered** recipient list。順序是契約的一部分：
// execute 會把它從 procurement_rfq_publication_approval_recipients 讀回來、用同一支
// domain helper 重算 audienceHash 再比對（publication_execute.go 的 verifyAudience）。
var pubExecRecipients = []domain.PublicationRecipient{
	domain.PlatformRecipient(501),
	domain.ExternalRecipient(9001),
	domain.PlatformRecipient(502),
}

// pubExecIDGen 是 goroutine-safe 的遞增 ID 產生器。
//
// 不用 dbtest_test.go 的 testIDGen()：那一支是裸 `n++`，併發測試的兩條 goroutine 共用
// 同一個 *repository.Repo 時會是 data race（-race 下必爆，而且會發出重複 ID）。
func pubExecIDGen() func() int64 {
	var n atomic.Int64
	return func() int64 { return n.Add(1) }
}

// pubExecFixture 是一組在真 DB 上**可以成功 execute** 的完整種子。
//
// ⛔ 刻意不沿用 repository 套件測試裡的 seedApprovalInput：那一份的 expires_at 綁在
// refTime()（2026-08-07，已是過去），audience_hash 是字面常數且沒有任何 recipient 列。
// Task 5 之後 execute 有到期 gate 與 audience-hash 重算 gate，那份種子**照定義**過不了
// ——過不了時要修的是種子，不是 gate。
type pubExecFixture struct {
	repo     *repository.Repo
	tx       TxRunner
	request  repository.Request
	revision repository.Revision
	approval repository.PublicationApproval
}

// input 回傳一組指向這份種子的合法 execute 輸入；clientRequestID 由呼叫端決定
// （execute 的冪等鍵，與 approval 的 preview_client_request_id 是兩把不同的鍵）。
func (f pubExecFixture) input(clientRequestID string) ExecuteRfqPublicationInput {
	return ExecuteRfqPublicationInput{
		BuyerID:            pubExecBuyerID,
		RequestID:          f.request.ID,
		ClientRequestID:    clientRequestID,
		ApprovalID:         f.approval.ID,
		ApprovalVersion:    f.approval.Version,
		ExpectedRevisionID: f.revision.ID,
		PayloadHash:        f.approval.SellerFacingPayloadHash,
	}
}

// newUseCase 用真 repo 組一個 usecase，並允許呼叫端替換其中任何一個 port
// （失敗注入與 race barrier 都靠這個）。nil 代表「用真的那一個」。
func (f pubExecFixture) newUseCase(
	commands PublicationCommandRepo,
	approvals PublicationApprovalExecuteRepo,
	runs PublicationRunWriter,
) *PublicationExecuteUseCase {
	if commands == nil {
		commands = f.repo
	}
	if approvals == nil {
		approvals = f.repo
	}
	if runs == nil {
		runs = f.repo
	}
	uc := NewPublicationExecuteUseCase(commands, approvals, runs, f.repo, f.repo, f.tx)
	uc.now = func() time.Time { return pubExecNow }
	return uc
}

// seedExecutablePublicationApproval 建 request → revision → price preview →
// confirmed snapshot → DIRECT_RECIPIENTS approval（含 ordered recipient list）這條鏈。
//
// recipients 由呼叫端給：audience_hash 是從它算出來的，所以「換一份名單」必須連同 hash
// 一起換（publication_execute_direct_integration_test.go 的 DIRECT 投遞測試需要指定
// 自己的 lane 組合）。本檔三支一律傳 pubExecRecipients。
//
// procurement 域刻意沒有外鍵（schema 註解：整包可抽成獨立服務），所以這條鏈不是 DB
// 完整性需要的，而是為了讓輸入裡的 id 與 hash 是真的從上游產出來的值，不是憑空編的
// 常數——audience_hash 尤其如此：它用 domain.DirectRecipientsAudience(...).Hash() 算，
// 與 preview 端編譯受眾用的是同一支 helper，execute 的重算 gate 才驗得過。
func seedExecutablePublicationApproval(ctx context.Context, t *testing.T, pool *pgxpool.Pool, recipients []domain.PublicationRecipient) pubExecFixture {
	t.Helper()

	tm := database.NewTxManager(pool)
	repo := repository.New(pool, tm, pubExecIDGen())

	req, rev, err := repo.CreateRequest(ctx, pubExecBuyerID)
	require.NoError(t, err)

	preview, err := repo.InsertPricePreview(ctx, repository.InsertPricePreviewInput{
		BuyerID: pubExecBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricingMode: "PRICEBOOK_GUIDED", QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		// ⛔ "AVAILABLE" 是 price_preview.go 真的會寫出來的值之一。不要改成
		// "UNAVAILABLE"：那個值只有搭配 BUYER_ENTERED_NO_REFERENCE 才放行
		// （2026-09-08 controller 裁決），配 PRICEBOOK_GUIDED 會被 Scope 5 的
		// hard block 擋下，而那是正確行為。
		ReferenceAvailability:   "AVAILABLE",
		BuyerRequestedRangeJSON: json.RawMessage(`{"min":"80","max":"120"}`),
		DealTemperatureJSON:     json.RawMessage(`{"status":"UNKNOWN"}`),
		ClientRequestID:         "pubexec-preview",
		PreviewInputHash:        "pubexec-input-hash",
		PayloadHash:             "pubexec-preview-payload-hash",
		ExpiresAt:               pubExecNow.Add(time.Hour),
	})
	require.NoError(t, err)

	canonical := json.RawMessage(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","pricingMode":"PRICEBOOK_GUIDED","buyerRequestedRange":{"min":"80","max":"120"},"item":"高麗菜","quantity":100,"unit":"公斤","deliveryCity":"台中市"}`)
	snap, err := repo.InsertConfirmedRfqSnapshot(ctx, repository.InsertConfirmedRfqSnapshotInput{
		BuyerID: pubExecBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricePreviewID: preview.ID, PricePreviewVersion: preview.Version,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: canonical, PayloadHash: "pubexec-snapshot-hash",
		ClientRequestID: "pubexec-snapshot", ExpiresAt: pubExecNow.Add(time.Hour),
	})
	require.NoError(t, err)

	candSetID := pubExecCandSetID
	candSetPolicyVersion := pubExecCandSetPolicyVersion
	audienceHash, err := domain.DirectRecipientsAudience(candSetID, candSetPolicyVersion, recipients).Hash()
	require.NoError(t, err)

	approval, err := repo.InsertPublicationApproval(ctx, repository.InsertPublicationApprovalInput{
		BuyerID: pubExecBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		ConfirmedRfqSnapshotID: snap.ID, ConfirmedRfqPayloadHash: snap.PayloadHash,
		PricePreviewID: preview.ID, PricePreviewVersion: preview.Version,
		PricePreviewPayloadHash: preview.PayloadHash, PricePreviewInputHash: preview.PreviewInputHash,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		ReferenceAvailability: "AVAILABLE", BuyerRequestedRangeHash: "pubexec-range-hash",

		AudienceMode: "DIRECT_RECIPIENTS", AudiencePolicyID: "", AudiencePolicyVersion: 0,
		AudienceHash:   audienceHash,
		CandidateSetID: &candSetID, CandidateSetPolicyVersion: &candSetPolicyVersion,
		Recipients: recipients,

		SellerFacingPayloadJSON: json.RawMessage(`{"item":"高麗菜","quantity":"100","unit":"公斤"}`),
		SellerFacingPayloadHash: "pubexec-seller-facing-hash",

		DealTemperatureStatus: "UNKNOWN", State: "PENDING",
		SchemaVersion:          "hawkr.agent.publication-approval.v1",
		PreviewClientRequestID: "pubexec-approval",
		// ⛔ 未來式。到期 gate（uc.now().After(approval.ExpiresAt)）是 Scope 5 的
		// hard block，種子過期時要修的是這一行。
		GeneratedAt: pubExecNow, ExpiresAt: pubExecNow.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, 1, approval.Version)
	require.Equal(t, "PENDING", approval.State)

	return pubExecFixture{repo: repo, tx: tm, request: req, revision: rev, approval: approval}
}

// ---- 真 DB 上的計數與狀態讀取（不經 repository，直接看表） ----

func pubExecCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&n))
	return n
}

func pubExecPublicationCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_rfq_publications WHERE buyer_id = $1`, int64(pubExecBuyerID))
}

func pubExecCommandCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_rfq_publication_commands WHERE buyer_id = $1`, int64(pubExecBuyerID))
}

func pubExecRunCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_agent_runs WHERE buyer_id = $1 AND run_type = 'PUBLICATION'`,
		int64(pubExecBuyerID))
}

func pubExecArtifactCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_agent_run_artifacts WHERE artifact_type = 'RFQ_PUBLICATION_RESULT'`)
}

// pubExecInviteCount／pubExecOutboxCount 刻意**不**經由 publication join。
//
// ⛔ rollback 之後 publication 那一列本來就不在了，任何 `WHERE publication_id IN
// (SELECT id FROM procurement_rfq_publications ...)` 的計數都會恆為 0——那正好是這道
// 哨兵要抓的失效模式（孤兒 invite），寫成 join 等於把哨兵關掉。invites 自己帶
// buyer_id 所以照它篩；outbox 沒有 buyer_id，直接整張表計數（每支測試有自己的
// database，不會混到兄弟測試的列）。
func pubExecInviteCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_rfq_publication_invites WHERE buyer_id = $1`, int64(pubExecBuyerID))
}

func pubExecOutboxCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	return pubExecCount(ctx, t, pool, `SELECT count(*) FROM procurement_rfq_publication_outbox`)
}

// pubExecApprovalState 回傳 approval 目前的 state 與 version——原子性測試真正要問的
// 就是這兩個值。
func pubExecApprovalState(ctx context.Context, t *testing.T, pool *pgxpool.Pool, approvalID int64) (string, int) {
	t.Helper()
	var state string
	var version int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT state, version FROM procurement_rfq_publication_approvals WHERE id = $1`,
		approvalID).Scan(&state, &version))
	return state, version
}

// ---------------------------------------------------------------------------
// 1. 併發 execute 只有一張 publication
// ---------------------------------------------------------------------------

// executeApprovalRaceRepo 在真正發出 CAS UPDATE 的前後各插一個掛鉤，把「兩個
// execute 真的同時撞上同一列 approval」變成**確定會發生**的事，而不是碰運氣。
//
// ⛔ 為什麼需要它：沒有掛鉤時兩條 goroutine 極可能先後跑完（第二條在第一條 commit
// **之後**才發出 UPDATE），那時輸家拿到 ErrApprovalStale 靠的是「讀到已 commit 的
// EXECUTING/SUCCEEDED」，跟併發無關——測試會綠，但它證明的是序列重送，不是 Scope 6
// 的 exactly-one。那種綠比紅還糟。
type executeApprovalRaceRepo struct {
	PublicationApprovalExecuteRepo

	// arriveCAS：兩條 goroutine 都到齊（各自的交易都已開啟、command 都已 claim）
	// 才一起放行去發 UPDATE。
	arriveCAS func()
	// wonCAS：贏家 CAS 成功之後按住那一列的鎖不放，直到主 goroutine 從
	// pg_stat_activity 觀察到真的有另一個 session 被擋住為止。
	wonCAS func()
}

func (r *executeApprovalRaceRepo) ClaimPublicationApprovalForExecute(
	ctx context.Context, approvalID int64, expectedVersion int, buyerID identity.BuyerID,
) (repository.PublicationApproval, error) {
	r.arriveCAS()
	approval, err := r.PublicationApprovalExecuteRepo.ClaimPublicationApprovalForExecute(ctx, approvalID, expectedVersion, buyerID)
	if err == nil {
		r.wonCAS()
	}
	return approval, err
}

// TestExecuteRfqPublicationIntegration_ConcurrentExecutes_YieldExactlyOnePublication
// 對同一份 approval 開兩條 goroutine 同時 execute（**不同** clientRequestId，所以
// command 那把冪等鍵不是仲裁者），斷言恰好一個成功、另一個回 ErrApprovalStale，
// 且 procurement_rfq_publications 只有一列。
//
// 仲裁者是 approval 的 CAS：UPDATE ... WHERE id AND version AND buyer_id AND
// state = 'PENDING'。輸的那個 UPDATE 會在贏家的列鎖上等，贏家 commit 之後
// Postgres（READ COMMITTED 的 EvalPlanQual）拿更新後的那一列重新求值述詞，state
// 已是 EXECUTING → 命中 0 列 → ErrPublicationApprovalCASFailed → ErrApprovalStale。
//
// 「這個 race 是真的」由 sawLockWait 斷言：主 goroutine 必須在 pg_stat_activity 上
// 真的看到一個被別人擋住的 session。看不到就代表兩條 goroutine 其實是先後跑的，
// 這支測試沒有覆蓋到它宣稱覆蓋的路徑——那時它會紅，不會假綠。
func TestExecuteRfqPublicationIntegration_ConcurrentExecutes_YieldExactlyOnePublication(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	f := seedExecutablePublicationApproval(ctx, t, pool, pubExecRecipients)

	const workers = 2

	var arrived atomic.Int32
	gate := make(chan struct{})
	arriveCAS := func() {
		if arrived.Add(1) == workers {
			close(gate)
		}
		select {
		case <-gate:
		case <-time.After(pubExecBarrierTimeout):
			// 只有另一條 goroutine 根本沒走到 CAS 才會發生。不 panic：讓
			// 下面的錯誤斷言把真正的原因印出來。
		}
	}

	// blocked 由觀察者 goroutine 關閉；sawLockWait 記錄它到底有沒有真的看到
	// 一個被擋住的 session。
	blocked := make(chan struct{})
	var sawLockWait atomic.Bool
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()

	go func() {
		defer close(blocked)
		deadline := time.Now().Add(pubExecBarrierTimeout)
		for time.Now().Before(deadline) {
			var n int
			// pg_blocking_pids(pid) 非空 ＝ 這個 session 正在等別人手上的鎖。
			// 每個測試有自己的 database（dbtest.Clone），所以 datname 過濾之後
			// 不會混到兄弟測試的 session。
			err := pool.QueryRow(watchCtx, `
				SELECT count(*) FROM pg_stat_activity
				 WHERE datname = current_database()
				   AND cardinality(pg_blocking_pids(pid)) > 0`).Scan(&n)
			if err == nil && n > 0 {
				sawLockWait.Store(true)
				return
			}
			// ctx-aware：測試結束（stopWatch）之後不要繼續對已關閉的 pool 輪詢。
			select {
			case <-watchCtx.Done():
				return
			case <-time.After(25 * time.Millisecond):
			}
		}
	}()

	wonCAS := func() {
		select {
		case <-blocked:
		case <-time.After(pubExecBarrierTimeout):
		}
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []ExecuteRfqPublicationResult
		errs    []error
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// 每條 goroutine 一個自己的 decorator，但共用同兩個掛鉤。
			uc := f.newUseCase(nil, &executeApprovalRaceRepo{
				PublicationApprovalExecuteRepo: f.repo,
				arriveCAS:                      arriveCAS,
				wonCAS:                         wonCAS,
			}, nil)

			in := f.input(fmt.Sprintf("pubexec-concurrent-%d", i))
			got, err := uc.ExecuteRfqPublication(ctx, in)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			results = append(results, got)
		}(i)
	}
	wg.Wait()
	stopWatch()

	require.True(t, sawLockWait.Load(),
		"沒有觀察到任何 session 在等鎖——兩條 goroutine 其實是先後跑完的，"+
			"這支測試並沒有覆蓋到併發路徑（Scope 6 的 exactly-one 未被驗證）")

	require.Len(t, results, 1, "恰好一個 execute 成功")
	require.Len(t, errs, 1, "另一個必須失敗，不是兩個都成功")
	require.ErrorIs(t, errs[0], ErrApprovalStale,
		"輸家拿到的必須是 ErrApprovalStale（呼叫端可據以重讀 approval），不是往上冒的 DB 錯誤")

	require.Equal(t, "PUBLISHED", results[0].State)
	require.False(t, results[0].Replayed, "贏家是真的建了一張，不是回放")

	require.Equal(t, 1, pubExecPublicationCount(ctx, t, pool), "procurement_rfq_publications 恰好一列")
	require.Equal(t, 1, pubExecCommandCount(ctx, t, pool),
		"輸家的 command claim 必須隨它的交易一起 rollback——冪等鍵不得被一次沒發成的 execute 燒掉")
	require.Equal(t, 1, pubExecRunCount(ctx, t, pool), "只有贏家留下 ledger run")
	require.Equal(t, 1, pubExecArtifactCount(ctx, t, pool))
	require.Equal(t, len(pubExecRecipients), pubExecInviteCount(ctx, t, pool),
		"⛔ 併發 exactly-one 必須延伸到 invite：贏家剛好每位收件人一列，輸家的整筆交易（含它試圖寫的 "+
			"invite）隨 ErrApprovalStale 一起 rollback。若 invite 寫入被搬到交易外，或 "+
			"ClaimPublicationApprovalForExecute 的 CAS 被鬆綁，這裡會多出重複列或孤兒列")
	require.Equal(t, len(pubExecRecipients), pubExecOutboxCount(ctx, t, pool),
		"同上，Vendor Portal E1 起兩個 lane 都進 outbox；"+
			"輸家的 outbox 寫入必須跟它的 invite 一起消失，不會讓 worker 為同一份 RFQ 重複發送")

	state, version := pubExecApprovalState(ctx, t, pool, f.approval.ID)
	require.Equal(t, "SUCCEEDED", state)
	require.Equal(t, 3, version, "PENDING(1) → EXECUTING(2) → SUCCEEDED(3)，輸家沒有再推進任何一版")
}

// ---------------------------------------------------------------------------
// 2. 任一步失敗 → 全部 rollback
// ---------------------------------------------------------------------------

var errPubExecArtifactBoom = errors.New("injected: artifact writer failed")

// failingArtifactRunWriter 在「publication 已建立、approval 尚未收尾」之間製造失敗
// （brief 指定的注入點）。刻意**不**發任何 SQL 就回錯：這樣外層交易在失敗當下仍是
// 健康的，rollback 是編排層自己決定的結果，不是被 Postgres 的 aborted 狀態強制的。
type failingArtifactRunWriter struct {
	PublicationRunWriter
	upsertCalls int
}

func (w *failingArtifactRunWriter) UpsertRunArtifact(context.Context, repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	w.upsertCalls++
	return repository.RunArtifact{}, errPubExecArtifactBoom
}

// countingCommandRepo 只數 InsertPublication 被叫了幾次。用途是釘住「失敗真的發生在
// publication 寫入**之後**」——沒有它，一個在更早階段就擋掉的實作也能讓下面那些
// 「0 列」斷言變綠，這支測試就退化成什麼都沒證明。
type countingCommandRepo struct {
	PublicationCommandRepo
	insertPublicationCalls int
}

func (r *countingCommandRepo) InsertPublication(ctx context.Context, in repository.InsertPublicationInput) (repository.Publication, error) {
	r.insertPublicationCalls++
	return r.PublicationCommandRepo.InsertPublication(ctx, in)
}

// TestExecuteRfqPublicationIntegration_ArtifactFailure_RollsBackEverything
// 在 publication 建好之後、approval 收尾之前注入失敗，斷言真 DB 上零殘留。
//
// ⛔ 這支紅的話要修的是 **transaction 邊界**，不是測試：最常見的成因是某個 repository
// 方法自己開了 WithTx 卻沒有沿用 ctx 上的交易，形成一個提早 commit 的巢狀交易。
// （pkg/database 的 WithTx 巢狀時直接 fn(ctx)、不另開交易，不會獨立 commit。）
//
// 最關鍵的一項是 approval 仍是 PENDING／version = 1：PENDING → EXECUTING 的 CAS 排在
// 所有 gate 之前，如果它沒有被 rollback 涵蓋，一次失敗的 execute 會把買家的 approval
// 永久卡在 EXECUTING——沒有任何路徑救得回來（沒有 EXECUTING → PENDING 的轉移）。
func TestExecuteRfqPublicationIntegration_ArtifactFailure_RollsBackEverything(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	f := seedExecutablePublicationApproval(ctx, t, pool, pubExecRecipients)

	commands := &countingCommandRepo{PublicationCommandRepo: f.repo}
	runs := &failingArtifactRunWriter{PublicationRunWriter: f.repo}
	uc := f.newUseCase(commands, nil, runs)

	_, err := uc.ExecuteRfqPublication(ctx, f.input("pubexec-rollback"))
	require.ErrorIs(t, err, errPubExecArtifactBoom)

	require.Equal(t, 1, commands.insertPublicationCalls,
		"注入點必須在 publication 寫入之後——否則下面的零殘留斷言證明不了 rollback")
	require.Equal(t, 1, runs.upsertCalls, "失敗真的被觸發過")

	require.Equal(t, 0, pubExecPublicationCount(ctx, t, pool),
		"publication 必須隨交易一起消失（Scope 6：任一步失敗全部 rollback）")
	require.Equal(t, 0, pubExecCommandCount(ctx, t, pool),
		"command claim 也必須被 rollback——失敗不得燒掉買家的 clientRequestId")
	require.Equal(t, 0, pubExecRunCount(ctx, t, pool), "ledger run 不得留下孤兒")
	require.Equal(t, 0, pubExecArtifactCount(ctx, t, pool))
	require.Equal(t, 0, pubExecInviteCount(ctx, t, pool),
		"⛔ invite 必須隨交易一起消失。把 execute 接線的 txCtx 換成 ctx，Repo.q 的 "+
			"GetDBTX 會靜默退回 pool，這些列就在 autocommit 下留下來——而下面那次重送會建**新的** "+
			"publication_id，uq_prpi_publication_store 也撞不到，所以除了這一行沒有任何測試看得見它")
	require.Equal(t, 0, pubExecOutboxCount(ctx, t, pool),
		"outbox command 同上，而且後果更重：worker 會為一張根本不存在的 publication 把 RFQ 發出去")
	require.Equal(t, len(pubExecRecipients), pubExecCount(ctx, t, pool,
		`SELECT count(*) FROM procurement_rfq_publication_approval_recipients WHERE approval_id = $1`,
		f.approval.ID),
		"rollback 只能回捲這次 execute 寫的東西——核准當下那份 ordered recipient list 屬於 approval，不得被牽連")

	state, version := pubExecApprovalState(ctx, t, pool, f.approval.ID)
	require.Equal(t, "PENDING", state,
		"⛔ approval 必須回到 PENDING。卡在 EXECUTING 沒有任何復原路徑——"+
			"那正是把整段流程放進單一交易要防的事")
	require.Equal(t, 1, version, "CAS 推進的 version 也必須被 rollback，否則買家手上的 token 當場作廢")

	// rollback 之後同一把 clientRequestId 必須還能用——這是「失敗不燒鍵」的
	// 端到端驗證，不是推論。
	uc2 := f.newUseCase(nil, nil, nil)
	got, err := uc2.ExecuteRfqPublication(ctx, f.input("pubexec-rollback"))
	require.NoError(t, err)
	require.False(t, got.Replayed)
	require.Equal(t, "PUBLISHED", got.State)
	require.Equal(t, 1, pubExecPublicationCount(ctx, t, pool))
	// 正向對照：證明上面那兩個 0 不是因為這條路徑從來不寫 invite／outbox。
	require.Equal(t, len(pubExecRecipients), pubExecInviteCount(ctx, t, pool),
		"成功的 execute 必須為每位收件人建一列 invite")
	require.Equal(t, len(pubExecRecipients), pubExecOutboxCount(ctx, t, pool),
		"Vendor Portal E1 起兩個 lane 都排投遞，每位收件人一列 outbox")
}

// ---------------------------------------------------------------------------
// 3. 冪等回放在真 DB 上成立
// ---------------------------------------------------------------------------

// claimSpyCommandRepo 記錄每一次 ClaimPublicationCommand 回了 (isNew, err)。
//
// 它是這支測試的證據來源：光看「第二次回 Replayed == true」證明不了那次 INSERT 真的
// 撞到了唯一鍵——一個在 INSERT 之前先查一次的實作也能給出同樣的結果。isNew 才是
// 分辨得出來的那一個訊號，見下面的斷言。
type claimSpyCommandRepo struct {
	PublicationCommandRepo
	claims []claimSpyCall
}

type claimSpyCall struct {
	isNew bool
	err   error
}

func (r *claimSpyCommandRepo) ClaimPublicationCommand(
	ctx context.Context, in repository.ClaimPublicationCommandInput,
) (repository.PublicationCommand, bool, error) {
	cmd, isNew, err := r.PublicationCommandRepo.ClaimPublicationCommand(ctx, in)
	r.claims = append(r.claims, claimSpyCall{isNew: isNew, err: err})
	return cmd, isNew, err
}

// TestExecuteRfqPublicationIntegration_SameClientRequestID_ReplaysOnRealDB
// 用同一把 clientRequestId 連送兩次（第二次是**全新的交易**，第一次已經 commit），
// 斷言第二次 Replayed == true、回同一張 publication、且資料庫沒有多出任何一列。
//
// ⛔ 這支是「撞冪等鍵不打爆呼叫端交易」在真 DB 上的端到端驗證：
//
// repository.ClaimPublicationCommand 的 INSERT 用 ON CONFLICT DO UPDATE，撞
// uq_procurement_rfq_publication_commands_idempotency 時回既有列而不拋錯，交易因此
// 保持健康。方法層哨兵是 repository 套件的
// TestClaimPublicationCommand_ReplaysInsideCallerTransaction。
//
// 下面三項斷言釘住這件事在**真 DB** 上的結果：
//
//   - claims[1].isNew == false → 證明第二次真的撞到了冪等鍵。
//   - claims[1].err == nil → 撞鍵不產生錯誤，什麼都不會冒到編排層。
//   - 第二次呼叫整體成功、外層交易 commit 得掉（下方的 count 與 approval 狀態斷言）。
//
// ⚠️ **這個設計依賴 READ COMMITTED**：外層交易必須看得見第一次呼叫已 commit 的那一列
// command，回查才找得到它。REPEATABLE READ 之下外層交易的 snapshot 停在交易開始的那
// 一刻，看不到那一列，回放會退化成「找不到 → 回原始錯誤」。今天沒有任何地方把
// isolation level 釘住，靠的是 Postgres server 的預設值——下面那道 SHOW 斷言就是
// 這件事的哨兵：它紅了，先去看 pool／pgx TxOptions 的 isolation 設定有沒有被改。
func TestExecuteRfqPublicationIntegration_SameClientRequestID_ReplaysOnRealDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	var isolation string
	require.NoError(t, pool.QueryRow(ctx, `SHOW default_transaction_isolation`).Scan(&isolation))
	require.Equal(t, "read committed", isolation,
		"⛔ 冪等回放依賴 READ COMMITTED（見本函式的說明）——isolation 被改掉的話，"+
			"撞鍵時的 ON CONFLICT 會看不到別的交易已 commit 的 command 列")

	f := seedExecutablePublicationApproval(ctx, t, pool, pubExecRecipients)

	commands := &claimSpyCommandRepo{PublicationCommandRepo: f.repo}
	uc := f.newUseCase(commands, nil, nil)

	in := f.input("pubexec-replay")

	first, err := uc.ExecuteRfqPublication(ctx, in)
	require.NoError(t, err)
	require.False(t, first.Replayed)
	require.NotZero(t, first.PublicationID)
	require.Equal(t, "PUBLISHED", first.State)

	// 第二次：全新的交易，第一次已經 commit。
	second, err := uc.ExecuteRfqPublication(ctx, in)
	require.NoError(t, err)
	require.True(t, second.Replayed, "同 clientRequestId 重送必須是回放")
	require.Equal(t, first.PublicationID, second.PublicationID, "回放必須回同一張 publication")
	require.Equal(t, first.PublicationVersion, second.PublicationVersion)
	require.Equal(t, first.SellerFacingPayloadHash, second.SellerFacingPayloadHash)
	require.Equal(t, first.AudienceHash, second.AudienceHash)

	require.Len(t, commands.claims, 2)
	require.NoError(t, commands.claims[0].err, "第一次 claim 是乾淨的 INSERT")
	require.True(t, commands.claims[0].isNew, "第一次 claim 必須是新建，不是回放")

	require.NoError(t, commands.claims[1].err,
		"撞冪等鍵不拋錯（ON CONFLICT），第二次 claim 因此直接完成回放")
	require.False(t, commands.claims[1].isNew,
		"第二次 claim 必須真的撞上唯一索引並走復原分支——(isNew=false, err=nil) 這個組合"+
			"只有 ClaimPublicationCommand 撞 23505 之後的復原路徑給得出來；"+
			"isNew=true 代表它建了第二列，非 nil 的錯誤代表隔離沒生效")

	// 零額外寫入：回放不建第二張 publication、不建第二個 command、不建第二個 run。
	require.Equal(t, 1, pubExecPublicationCount(ctx, t, pool))
	require.Equal(t, 1, pubExecCommandCount(ctx, t, pool))
	require.Equal(t, 1, pubExecRunCount(ctx, t, pool), "回放不得再投影一個 ledger run")
	require.Equal(t, 1, pubExecArtifactCount(ctx, t, pool))

	// approval 停在第一次 execute 收尾的狀態：回放在 gate 之前就短路，不再推進 CAS。
	state, version := pubExecApprovalState(ctx, t, pool, f.approval.ID)
	require.Equal(t, "SUCCEEDED", state)
	require.Equal(t, 3, version)
}
