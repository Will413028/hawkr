//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestCreateCandidateSet_SetsQueuedStatusAndMatchingRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)

	if set.Status != "QUEUED" {
		t.Errorf("Status = %q, want QUEUED", set.Status)
	}
	if set.FencingToken != 1 {
		t.Errorf("FencingToken = %d, want 1", set.FencingToken)
	}

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	if gotReq.Status != "MATCHING" {
		t.Errorf("request status = %q, want MATCHING", gotReq.Status)
	}

	// FencingToken 不能只是回傳值裡的假象——實際 DB row 也必須是 1，否則
	// MarkCandidateSetRunning／FinalizeCandidateSet 的 fencing_token 比對會永遠對不上。
	var gotFencingToken int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT fencing_token FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&gotFencingToken))
	require.Equal(t, int64(1), gotFencingToken, "fencing_token 必須真的寫進 DB，不只是回傳的 struct")
}

func TestCreateCandidateSet_IdempotentOnSameRevisionAndPolicyVersion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	first, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)

	found, err := repo.FindCandidateSet(ctx, rev.ID, 1)
	require.NoError(t, err)
	if found.ID != first.ID {
		t.Errorf("FindCandidateSet returned different id: got %d, want %d", found.ID, first.ID)
	}
}

func TestFindCandidateSet_NotFoundWhenNoneExists(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.FindCandidateSet(ctx, 999999, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// markRunning 是 MarkCandidateSetRunning 的測試輔助：大多數測試只是要把 run 推進到
// RUNNING，claim 失敗代表測試前提本身壞了，不是被測行為。
func markRunning(t *testing.T, ctx context.Context, repo *Repo, id, fencingToken int64) {
	t.Helper()
	claimed, err := repo.MarkCandidateSetRunning(ctx, id, fencingToken)
	require.NoError(t, err)
	require.True(t, claimed, "test precondition: MarkCandidateSetRunning 必須認領成功")
}

// TestMarkCandidateSetRunning_RejectsWrongFencingToken 驗證 fencing_token 防呆的核心
// 保證：帶錯 token（例如 reaper 已經動過、或另一個 worker 的 token）不能把 QUEUED 轉成
// RUNNING，且不報錯（沒有東西壞掉，只是沒認領到）——但必須回報 claimed=false，呼叫端
// 靠這個布林值決定放棄整趟搜尋，不是靠 error。
func TestMarkCandidateSetRunning_RejectsWrongFencingToken(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)

	// 錯的 token（99）不應該讓狀態轉成 RUNNING。
	claimed, err := repo.MarkCandidateSetRunning(ctx, set.ID, 99)
	require.NoError(t, err, "沒認領到不是 error")
	require.False(t, claimed, "錯誤的 fencing_token 必須回報 claimed=false，不能被呼叫端當成認領成功")

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&status))
	require.Equal(t, "QUEUED", status, "錯誤的 fencing_token 不得轉動狀態")

	// 對的 token（1）才能轉成 RUNNING。
	markRunning(t, ctx, repo, set.ID, set.FencingToken)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&status))
	require.Equal(t, "RUNNING", status)
}

// TestFinalizeCandidateSet_WritesCandidatesAndTerminalStatus 驗證 FinalizeCandidateSet
// 正常路徑：兩個 lane 都 OK → COMPLETED，候選寫入且各自被指派了 ID，request 狀態轉
// CANDIDATES_READY。
func TestFinalizeCandidateSet_WritesCandidatesAndTerminalStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	platformCandidate := Candidate{
		Lane: "PLATFORM", SourceID: "prod-1", DisplayName: "台灣好蔬果",
		Score: 90, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
		FactsAsOf: refTime(), Rank: 1,
	}
	externalCandidate := Candidate{
		Lane: "EXTERNAL", SourceID: "lead-1", DisplayName: "阿明水果行",
		Score: 80, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
		FactsAsOf: refTime(), Rank: 1,
	}

	wrote, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{platformCandidate}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{externalCandidate}},
	})
	require.NoError(t, err)
	require.True(t, wrote, "終態真的寫進去了，wrote 必須是 true——呼叫端靠它決定要不要記 procurement_run_total")

	gotSet, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, codemeta.ProcurementRunStatusSucceeded.Value, gotSet.Status)
	require.Len(t, candidates, 2)
	for _, c := range candidates {
		require.NotZero(t, c.ID, "候選 ID 必須由 FinalizeCandidateSet 內部指派")
	}

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CANDIDATES_READY", gotReq.Status)
}

// testCandidate 是 FinalizeCandidateSetInput 測試用的候選，兩個欄位變動：lane 與
// source_id，其他值固定，讓每個測試案例的候選內容一望即知彼此不同。
func testCandidate(lane, sourceID string) Candidate {
	return Candidate{
		Lane: lane, SourceID: sourceID, DisplayName: "候選-" + sourceID,
		Score: 90, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
		FactsAsOf: refTime(), Rank: 1,
	}
}

// TestFinalizeCandidateSet_SkipsWriteWhenFencingTokenStale 驗證 fencing_token 對不上時
// （例如 reaper 已經搶先把這個 run 標成 FAILED）FinalizeCandidateSet 靜默放棄寫入，
// 不覆蓋已經定案的終態——且候選資料本身也真的沒有落地，不只是 status 欄位沒被動到。
//
// 兩個 lane 都帶「非空」的候選列表：早期版本這裡兩個 lane 都是空的
// LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value}（zero-value Candidates == nil），所以下面的
// `count(*) == 0` 斷言不管候選 INSERT 有沒有真的被擋下都會通過——等於沒測到
// Critical #1（INSERT 排在防呆 UPDATE 之前）想擋的那個 bug。
func TestFinalizeCandidateSet_SkipsWriteWhenFencingTokenStale(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)

	// 帶錯的 fencing_token（模擬 stale worker），且兩個 lane 都帶著非空候選——
	// 如果 INSERT 沒有被防呆擋在前面，這裡就會真的寫進 2 筆候選。
	wrote, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: 999, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("PLATFORM", "prod-1")}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("EXTERNAL", "lead-1")}},
	})
	require.NoError(t, err, "fencing_token 不符不應該回傳錯誤，只是放棄寫入")
	require.False(t, wrote, "放棄寫入必須回報 wrote=false，否則呼叫端會把這次被丟掉的寫入計成一次真實的 run")

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&status))
	require.Equal(t, "QUEUED", status, "錯誤的 fencing_token 不得改動 candidate_set 狀態")

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidates WHERE candidate_set_id = $1`, set.ID,
	).Scan(&count))
	require.Equal(t, int64(0), count, "fencing_token 不符時候選必須整個放棄寫入，不能只是 status 沒被動到")
}

// TestFinalizeCandidateSet_SecondCallOnAlreadyTerminalIsNoOp 驗證「同一個 run 被
// finalize 兩次」這個最貼近現實的情境——不需要任何 reap／崩潰，單純是 at-least-once
// 重試／重送（例如訊息佇列 redelivery）——第二次呼叫不會讓候選數翻倍。這正是 Critical #1
// 修復前會直接讓買家看到重複候選的路徑：兩次呼叫用同一個（正確的）fencing_token，
// 第一次讓 run 進入 COMPLETED，第二次因為 status 已經是終態而被 `status NOT IN (...)`
// 擋下、affected=0，此時候選 INSERT 若排在防呆 UPDATE 之前就已經執行過了。
func TestFinalizeCandidateSet_SecondCallOnAlreadyTerminalIsNoOp(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	in := FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("PLATFORM", "prod-1")}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("EXTERNAL", "lead-1")}},
	}

	wrote, _, _, err := repo.FinalizeCandidateSet(ctx, in)
	require.NoError(t, err)
	require.True(t, wrote, "第一次 finalize 真的寫入了終態")

	var firstCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidates WHERE candidate_set_id = $1`, set.ID,
	).Scan(&firstCount))
	require.Equal(t, int64(2), firstCount, "第一次 finalize 應該寫入兩個 lane 各一筆候選")

	// 同一個（正確的）fencing_token，重打一次——模擬 at-least-once 重送，不是崩潰／reap。
	wrote, _, _, err = repo.FinalizeCandidateSet(ctx, in)
	require.NoError(t, err)
	require.False(t, wrote,
		"對已終態的 run 重打 finalize 必須回報 wrote=false——否則同一個 run 會被 procurement_run_total 計兩次")

	var secondCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidates WHERE candidate_set_id = $1`, set.ID,
	).Scan(&secondCount))
	require.Equal(t, firstCount, secondCount, "對已終態的 run 重打 finalize 不得讓候選數翻倍")

	_, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Len(t, candidates, 2, "買家讀到的候選數必須維持第一次寫入的數量，不是重複的 4 筆")
}

// TestFinalizeCandidateSet_DoesNotClobberNewerRequestStatus 驗證 finalize 對 request
// 狀態的寫入跟 reap 一樣有 staleness 守衛。這是三個 request.status 寫入點裡 race window
// 最寬的一個：run() 跑在背景 goroutine，HTTP handler 早就回應了，最久還會再跑 10 秒
// （candidateSearchBudget）。買家在這段期間送出下一輪 turn 時，AppendRevision 會原子地
// 把 active_revision_id 前推、把 request.status 寫成 CLARIFYING；舊 run 接著 finalize，
// 無守衛的寫入就會把 CLARIFYING 蓋成 CANDIDATES_READY／REQUIREMENT_READY。
//
// REQUIREMENT_READY 那條路徑是真正會咬人的：handler.PostCandidateSearch 直接拿
// req.Status == "REQUIREMENT_READY" 當 RequirementReady，所以被蓋回去之後，下一次
// POST candidate-searches 會對一個必填 slot 尚未補齊的需求通過 ErrRequirementNotReady
// 閘門、真的開始搜尋。這裡用「兩個 lane 都 OK」的版本斷言（會蓋成 CANDIDATES_READY），
// 因為它同時證明了 candidate_set 本身仍然正常落地終態——守衛只該擋 request 那一句。
func TestFinalizeCandidateSet_DoesNotClobberNewerRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev1, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	// 對 rev1 開一個 lease 還沒過期的 run（所以不會被 lazy reaper 收走——這個測試要驗的
	// 是「活著的 run 遲到完工」，不是 reap）。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev1.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	// 背景 run 還在跑的時候，買家送出下一輪 turn：active_revision_id 前進到 rev2，
	// 必填 slot 又不齊，request 狀態回到 CLARIFYING。
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{},
		Source: "ANSWER", ClientRequestID: "cr-during-inflight-run", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	// 舊 run（仍持有正確的 fencing_token）現在才完工。
	_, _, _, err = repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev1.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("PLATFORM", "prod-1")}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("EXTERNAL", "lead-1")}},
	})
	require.NoError(t, err)

	gotSet, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, codemeta.ProcurementRunStatusSucceeded.Value, gotSet.Status,
		"candidate_set 本身仍然要正常落地終態——守衛只擋 request 狀態那一句寫入")
	require.Len(t, candidates, 2, "候選資料屬於舊 revision，仍然要寫入，買家用舊 candidate_set_id 讀得到")

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"finalize 一個已經不是 active revision 的 run，不得把 request 已經前進的狀態蓋掉")
}

// TestFinalizeCandidateSet_FailedRunDoesNotClobberNewerRequestStatus 是上一個測試的
// REQUIREMENT_READY 分支版本（兩個 lane 都 FAILED）——這才是真正會讓下一次
// POST candidate-searches 誤過 ErrRequirementNotReady 閘門的那條路徑，值得獨立守住。
func TestFinalizeCandidateSet_FailedRunDoesNotClobberNewerRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev1, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev1.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{},
		Source: "ANSWER", ClientRequestID: "cr-during-inflight-failing-run", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	_, _, _, err = repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev1.ID,
		Platform:          LaneResult{Status: "FAILED"},
		External:          LaneResult{Status: "FAILED"},
	})
	require.NoError(t, err)

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"一個屬於舊 revision 的失敗 run 不得把 request 打回 REQUIREMENT_READY——那會讓下一次 POST candidate-searches 對必填 slot 未齊的需求通過就緒閘門")
}

// TestReapExpiredCandidateSet_BumpsFencingTokenSoStaleTokenNoLongerWorks 驗證 reap
// 不只改 status，也遞增 fencing_token：一個在 lease 過期前就已經拿到 token 的 zombie
// worker，reap 之後拿著原本的（現在已經過期的）token 呼叫 FinalizeCandidateSet 必須
// 影響 0 列——不能只靠 status NOT IN (...) 擋，fencing_token 比對本身也要是真的防線。
func TestReapExpiredCandidateSet_BumpsFencingTokenSoStaleTokenNoLongerWorks(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// 立刻過期的 lease，worker 認領到的 token 是 CreateCandidateSet 保證的 1。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)
	originalToken := set.FencingToken
	require.Equal(t, int64(1), originalToken)

	// 觸發 lazy reap。
	gotSet, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", gotSet.Status)

	var dbToken int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT fencing_token FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&dbToken))
	require.Equal(t, originalToken+1, dbToken, "reap 必須把 fencing_token 遞增，不能維持原值")

	// zombie worker 帶著「原本」的 token 回來 finalize——即使它以為自己還握有效的 token，
	// 這句也必須影響 0 列，且不寫入任何候選。
	wrote, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: originalToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("PLATFORM", "prod-1")}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{testCandidate("EXTERNAL", "lead-1")}},
	})
	require.NoError(t, err, "fencing_token 過期不應該回傳錯誤，只是放棄寫入")
	require.False(t, wrote,
		"zombie worker 的寫入被丟掉時必須回報 wrote=false——那個 run 的終態是 reaper 寫的 WORKER_LOST，已經由 reaper 計過一次")

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidates WHERE candidate_set_id = $1`, set.ID,
	).Scan(&count))
	require.Equal(t, int64(0), count, "reap 之後，zombie worker 的舊 token 不得再讓候選落地")
}

// TestGetCandidateSetWithCandidates_ReapDoesNotClobberNewerRequestStatus 驗證 reap 對
// request 狀態的寫入有做 staleness 防呆：買家在這個 candidate_set 的 lease 過期前，
// 已經先送出下一輪 turn（active_revision_id 前進、request 狀態變成別的值），這時候一個
// 遲來的 reap 讀到「舊 run 過期了」，不該把 request 的最新狀態蓋回 REQUIREMENT_READY。
func TestGetCandidateSetWithCandidates_ReapDoesNotClobberNewerRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev1, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// 對 rev1 建立一個立刻過期的 candidate_set（模擬一個 worker 對舊版本跑搜尋、
	// 但買家沒等它跑完就自己送出下一輪 turn 了）。
	staleSet, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev1.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	// 買家送出下一輪 turn：新增 rev2，active_revision_id 前進，request 狀態變成
	// CLARIFYING（比 REQUIREMENT_READY 更「新」的狀態，用來跟 reap 想蓋回去的值區分）。
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{},
		Source: "ANSWER", ClientRequestID: "cr-after-stale-set", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	// 讀取（並藉此 reap）那個屬於 rev1、早就過期的 candidate_set。
	gotSet, _, err := repo.GetCandidateSetWithCandidates(ctx, staleSet.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", gotSet.Status, "candidate_set 本身仍然要被正確 reap")

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"reap 一個舊 revision 的 candidate_set 不得把 request 已經前進的狀態蓋回 REQUIREMENT_READY")
}

// TestGetCandidateSetWithCandidates_ReapsExpiredLease 驗證 lazy reaper：讀取一個
// lease 已過期但仍是 QUEUED 的 run，應就地轉成 FAILED/WORKER_LOST，並把 request 狀態
// 退回 REQUIREMENT_READY 讓買家可以重新觸發搜尋。
func TestGetCandidateSetWithCandidates_ReapsExpiredLease(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// 建立一個立刻過期的 lease（TTL 負值）。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	gotSet, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", gotSet.Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonWorkerLost.Value, gotSet.FailureReason)
	require.Empty(t, candidates)

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "REQUIREMENT_READY", gotReq.Status)
}

// TestGetCandidateSetWithCandidates_ReapCountsAsFailedRun 驗證被 reaper 收斂的 run 會計進
// procurement_run_total{status="FAILED"}。
//
// 一次成功的 reap 就是這個 run 的終態寫入，跟 FinalizeCandidateSet 寫終態是同一件事的另一
// 條路徑。少了這個計數，WORKER_LOST 這整類終態對 doc 55 §8.2 指定的主要 run 健康訊號完全
// 隱形：worker 大批死亡時 counter 只會「安靜地不再增加」，跟流量下降無法區分——而那正是
// 最需要這個 metric 說話的時候。
//
// 第二段（對同一列再讀一次）守的是「只有真的寫進去的那次才計數」：run 已經是 FAILED，
// 不會再觸發 reap，counter 不得再動。
func TestGetCandidateSetWithCandidates_ReapCountsAsFailedRun(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	reader := installTestMeterProvider(t)
	before := counterValue(t, reader, "procurement_run_total", "status", "FAILED")

	gotSet, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", gotSet.Status, "test precondition: reap 必須真的發生")

	afterReap := counterValue(t, reader, "procurement_run_total", "status", "FAILED")
	// int64 而非 float64：counterValue 回傳 int64（metricdata.Sum[int64] 的 point.Value），
	// 而 require.Equal 比型別，float64(1) 對 int64(1) 會直接紅。
	require.Equal(t, int64(1), afterReap-before,
		"一次真實的 reap 必須計一筆 procurement_run_total{status=FAILED}")

	// 同一列再讀一次：已經是終態，不會再 reap，counter 不得再動。
	_, _, err = repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, afterReap, counterValue(t, reader, "procurement_run_total", "status", "FAILED"),
		"沒有發生 reap 的讀取不得計數")
}

// TestRetryCandidateSet_SetsQueuedStatusAndMatchingRequestStatus 驗證 RetryCandidateSet
// 跟 CreateCandidateSet 走同一個「(重新)啟動搜尋」交易形狀：不只把 candidate_set 從
// FAILED 轉回 QUEUED、遞增 fencing_token，也要把 procurement_requests.status 轉成
// MATCHING（doc 55 §7.5 REQUIREMENT_READY → MATCHING 這個邊，沒有限定只能從「建立新
// run」觸發）。Task 8 review round 2 抓到的問題：少了這一步，候選搜尋本身正常執行，
// request.status 卻卡在 REQUIREMENT_READY 直到完工才跳 CANDIDATES_READY，買家輪詢
// request 快照時會在重試搜尋進行中誤判成「沒有在搜尋」。
func TestRetryCandidateSet_SetsQueuedStatusAndMatchingRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// 用立刻過期的 lease + lazy reap 讓 candidate_set 真的走到 FAILED（跟
	// TestGetCandidateSetWithCandidates_ReapsExpiredLease 同一招），而不是手動 UPDATE
	// 硬塞一個 FAILED 狀態進去——這樣測試走的是真實會發生的路徑（worker 死掉 → lease
	// 過期 → reaper 收斂 → 買家重新呼叫）。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status, "test precondition: reap 必須先把 run 收斂成 FAILED")

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "REQUIREMENT_READY", gotReq.Status, "test precondition: reap 之後 request 應該退回 REQUIREMENT_READY")

	retried, err := repo.RetryCandidateSet(ctx, reaped.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)

	if retried.ID != set.ID {
		t.Errorf("RetryCandidateSet 回傳了不同的 id (%d)，want 沿用原本的 candidate_set (%d)——重試必須重用同一列", retried.ID, set.ID)
	}
	if retried.Status != "QUEUED" {
		t.Errorf("Status = %q, want QUEUED", retried.Status)
	}
	if retried.FencingToken != reaped.FencingToken+1 {
		t.Errorf("FencingToken = %d, want %d（剛好被這一次 RetryCandidateSet 遞增一次）", retried.FencingToken, reaped.FencingToken+1)
	}

	gotReq, _, err = repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	if gotReq.Status != "MATCHING" {
		t.Errorf("request status after retry = %q, want MATCHING", gotReq.Status)
	}
}

// TestRetryCandidateSet_ResetsLaneStatuses 驗證重試會把兩條 lane 的狀態歸零回 PENDING。
//
// 走 reap 進 FAILED 的 run，lane 狀態本來就還停在 PENDING，所以看不出問題；真正會露餡的
// 是「兩條 lane 都真的失敗」而由 FinalizeCandidateSet 判定 FAILED 的 run——那時
// platform_lane_status／external_lane_status 都是 'FAILED'。少了重置，重試之後整段新嘗試
// 期間輪詢的 client 會讀到 status=RUNNING 但兩條 lane 都 FAILED：上一次嘗試的失敗資訊被
// 當成這一次的進度，一路顯示到新的 finalize 才被蓋掉。這裡刻意在 retry 之後、任何新的
// finalize 之前就斷言，就是為了鎖住那個中間視窗。
func TestRetryCandidateSet_ResetsLaneStatuses(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	// 兩條 lane 都失敗 → run 進 FAILED，且 lane 狀態真的被寫成 FAILED（不是 reap 那條
	// lane 狀態維持 PENDING 的路徑）。
	_, _, _, err = repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: "FAILED"},
		External:          LaneResult{Status: "FAILED"},
	})
	require.NoError(t, err)

	var status, platformLane, externalLane string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, platform_lane_status, external_lane_status
		 FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&status, &platformLane, &externalLane))
	require.Equal(t, "FAILED", status, "test precondition")
	require.Equal(t, "FAILED", platformLane, "test precondition: lane 狀態必須真的是 FAILED，不是 PENDING")
	require.Equal(t, "FAILED", externalLane, "test precondition")

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, "PENDING", retried.PlatformLaneStatus,
		"重試回傳的實體必須顯示 lane 已歸零，client 立刻輪詢就會拿到這個值")
	require.Equal(t, "PENDING", retried.ExternalLaneStatus)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, platform_lane_status, external_lane_status
		 FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&status, &platformLane, &externalLane))
	require.Equal(t, "QUEUED", status)
	require.Equal(t, "PENDING", platformLane,
		"重試＝重新開始一次 run，lane 狀態不得殘留上一次嘗試的 FAILED")
	require.Equal(t, "PENDING", externalLane,
		"重試＝重新開始一次 run，lane 狀態不得殘留上一次嘗試的 FAILED")
}

// TestCreateCandidateSet_DoesNotClobberNewerRequestStatus 驗證第三個 request.status
// 寫入點（建立 run）也套了同一把 staleness 守衛。這條路徑的 race window 比背景 finalize
// 窄，但不是零：handler 先讀 snapshot（rev1 / REQUIREMENT_READY）決定可以開搜尋，買家在
// 那之後、CreateCandidateSet 之前送出下一輪 turn 把 request 推到 rev2 / CLARIFYING，
// 這一句仍會拿著 rev1 進來。三個寫入點統一守衛，就不會有人日後照著沒守衛的那一個複製。
func TestCreateCandidateSet_DoesNotClobberNewerRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev1, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	// 買家送出下一輪 turn，active_revision_id 前進到 rev2、狀態回到 CLARIFYING。
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{},
		Source: "ANSWER", ClientRequestID: "cr-before-create", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	// 一個拿著 rev1 這個舊快照的建立呼叫。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev1.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, "QUEUED", set.Status, "candidate_set 本身仍然要正常建立")

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"對舊 revision 建立 run 不得把 request 已經前進的狀態蓋成 MATCHING")
}

// TestRetryCandidateSet_DoesNotClobberNewerRequestStatus 同上，針對重試那個寫入點：
// reap 已經把 run 收斂成 FAILED，買家接著送出下一輪 turn，然後一個併發／遲到的
// StartSearch 對舊 run 呼叫 RetryCandidateSet——candidate_set 可以重新排隊，但
// request 的最新狀態不得被蓋成 MATCHING。
func TestRetryCandidateSet_DoesNotClobberNewerRequestStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev1, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev1.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status, "test precondition: reap 必須先把 run 收斂成 FAILED")

	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{},
		Source: "ANSWER", ClientRequestID: "cr-before-retry", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	retried, err := repo.RetryCandidateSet(ctx, reaped.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, "QUEUED", retried.Status, "candidate_set 本身仍然要重新排隊")

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"重試一個屬於舊 revision 的 run 不得把 request 已經前進的狀態蓋成 MATCHING")
}

// TestReapExpiredCandidateSet_StaleSnapshotDoesNotKillFreshlyReleasedRun 把
// TestReapExpiredCandidateSet_* 與 TestRetryCandidateSet_* 組起來跑——這兩組測試原本各自
// 存在但從未被組合過，而那個組合正是 bug 藏身處。
//
// ReapExpiredProcurementCandidateSet 這句名字裡有 Expired，但過期判斷原本完全在 Go 端：
// 呼叫端讀 row → 在記憶體裡比對 LeaseExpiresAt → 才開交易呼叫這句。讀取與 UPDATE 不在同
// 一個交易，中間狀態可能已經翻轉。真實可達的交錯（兩個併發 GET 輪詢）：
//
//	讀者 A、讀者 B 都讀到同一列過期的 lease
//	→ A 的 reap 交易先 commit，row 變 FAILED
//	→ 一次 POST candidate-searches 走 RetryCandidateSet 把同一列救回 QUEUED（新 lease
//	  還有 30 秒、fencing_token 再 +1、新的 worker 已經派出去了）
//	→ B 這時才進 reap 交易，拿著自己那份「它過期了」的舊快照
//
// 只比對 status IN ('QUEUED','RUNNING') 的話，B 會直接殺掉這個合法、活著的 run。
// 這裡直接呼叫未匯出的 reapCandidateSet 並餵它 retry 之前的舊快照，就是在重現 B。
func TestReapExpiredCandidateSet_StaleSnapshotDoesNotKillFreshlyReleasedRun(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// staleSnapshot 就是兩個讀者共同看到的那一份：QUEUED + lease 已過期。
	staleSnapshot, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, staleSnapshot.LeaseExpiresAt)
	require.True(t, staleSnapshot.LeaseExpiresAt.Before(time.Now().UTC()),
		"test precondition: 兩個讀者手上的快照都必須是「已過期」")

	// 讀者 A 先 reap 成功。
	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, staleSnapshot.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status)

	// 買家重新呼叫 POST candidate-searches：同一列被救回 QUEUED，配發還有 30 秒的新 lease。
	retried, err := repo.RetryCandidateSet(ctx, staleSnapshot.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, "QUEUED", retried.Status)

	// 讀者 B 這時才進場，拿的是 retry 之前的舊快照。
	out, err := repo.reapCandidateSet(ctx, staleSnapshot)
	require.NoError(t, err)
	require.Equal(t, "QUEUED", out.Status,
		"reapCandidateSet 必須回傳 DB 當下的真實狀態，不是它以為自己寫進去的 FAILED")

	var status, failureReason string
	var fencingToken int64
	var leaseExpiresAt time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, failure_reason, fencing_token, lease_expires_at
		 FROM procurement_candidate_sets WHERE id = $1`, staleSnapshot.ID,
	).Scan(&status, &failureReason, &fencingToken, &leaseExpiresAt))

	require.Equal(t, "QUEUED", status,
		"一份過時的快照不得殺掉一個 lease 還有效的 run——過期條件必須在 UPDATE 的 WHERE 裡重驗")
	require.Equal(t, "", failureReason, "活著的 run 不該被寫上 WORKER_LOST")
	require.Equal(t, retried.FencingToken, fencingToken,
		"落空的 reap 不得遞增 fencing_token——否則新派出的 worker 手上那把 token 會憑空作廢")
	require.True(t, leaseExpiresAt.After(time.Now().UTC()), "新配發的 lease 必須原封不動")

	// request 狀態同樣不該被那次落空的 reap 打回 REQUIREMENT_READY。
	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "MATCHING", gotReq.Status,
		"重試已經把 request 推回 MATCHING，落空的 reap 不得把它蓋回 REQUIREMENT_READY")
}

// TestUpsertSelection_TogglesSelectedOnConflict 驗證同一鍵 (revision, lane, candidate)
// 重複 upsert 會更新既有列的 selected，而不是產生第二列，且 id 沿用第一次 insert 的值。
func TestUpsertSelection_TogglesSelectedOnConflict(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)
	_, _, _, err = repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{{
			Lane: "PLATFORM", SourceID: "prod-1", DisplayName: "台灣好蔬果",
			Score: 90, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
			FactsAsOf: refTime(), Rank: 1,
		}}},
		External: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
	})
	require.NoError(t, err)

	_, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	candidateID := candidates[0].ID

	first, err := repo.UpsertSelection(ctx, UpsertSelectionInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		CandidateSetID: set.ID, Lane: "PLATFORM", CandidateID: candidateID, Selected: true,
	})
	require.NoError(t, err)
	require.True(t, first.Selected)

	second, err := repo.UpsertSelection(ctx, UpsertSelectionInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		CandidateSetID: set.ID, Lane: "PLATFORM", CandidateID: candidateID, Selected: false,
	})
	require.NoError(t, err)
	require.False(t, second.Selected)
	require.Equal(t, first.ID, second.ID, "同鍵 upsert 必須沿用第一次 insert 的 id")

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidate_selections WHERE candidate_set_id = $1`, set.ID,
	).Scan(&count))
	require.Equal(t, int64(1), count, "同鍵重複 upsert 不得產生第二列")

	gotCandidate, err := repo.GetCandidate(ctx, set.ID, candidateID)
	require.NoError(t, err)
	require.Equal(t, candidateID, gotCandidate.ID)

	_, err = repo.GetCandidate(ctx, set.ID, 999999)
	require.ErrorIs(t, err, ErrNotFound)
}

// TestListSelectedCandidateIDs_ReflectsCurrentSelectionState 驗證三種狀態都讀得對：
// 勾選中的候選在 map 裡是 true、取消勾選後（UpsertSelection selected=false，upsert 不刪列）
// 必須是 false（不是仍然殘留 true）、從未被勾選過的候選完全不在 map 裡查詢時得到 false
// 而不是 panic 或 nil map 的零值混淆。
func TestListSelectedCandidateIDs_ReflectsCurrentSelectionState(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)
	_, _, _, err = repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{
			{Lane: "PLATFORM", SourceID: "prod-1", DisplayName: "候選A",
				Score: 90, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
				FactsAsOf: refTime(), Rank: 1},
			{Lane: "PLATFORM", SourceID: "prod-2", DisplayName: "候選B",
				Score: 80, MatchedFactsJSON: []byte(`[]`), UnverifiableFactsJSON: []byte(`[]`),
				FactsAsOf: refTime(), Rank: 2},
		}},
		External: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
	})
	require.NoError(t, err)

	_, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	candidateA, candidateB := candidates[0].ID, candidates[1].ID

	// 候選 A：勾選
	_, err = repo.UpsertSelection(ctx, UpsertSelectionInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		CandidateSetID: set.ID, Lane: "PLATFORM", CandidateID: candidateA, Selected: true,
	})
	require.NoError(t, err)
	// 候選 B：勾選後又取消（測「取消後必須是 false，不是殘留 true」）
	_, err = repo.UpsertSelection(ctx, UpsertSelectionInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		CandidateSetID: set.ID, Lane: "PLATFORM", CandidateID: candidateB, Selected: true,
	})
	require.NoError(t, err)
	_, err = repo.UpsertSelection(ctx, UpsertSelectionInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		CandidateSetID: set.ID, Lane: "PLATFORM", CandidateID: candidateB, Selected: false,
	})
	require.NoError(t, err)

	selected, err := repo.ListSelectedCandidateIDs(ctx, set.ID)
	require.NoError(t, err)

	require.True(t, selected[candidateA], "候選 A 目前是勾選狀態，必須是 true")
	require.False(t, selected[candidateB], "候選 B 已取消勾選，必須是 false，不能殘留 true")
	require.False(t, selected[999999], "從未被勾選過的候選查詢必須是 false，不是 panic 或誤判成 true")
}

// TestFinalizeCandidateSet_RoundTripsSellerSnapshot 驗證 #2377 的四個賣家身分欄位真的
// 寫進 DB 並且讀得回來，且 EXTERNAL lane 的 store_id 落成 SQL NULL（讀回來是 0）。
//
// 為什麼要一支 DB 層測試而不只靠 usecase 層的組裝測試：這四欄要通過
// schema → sqlc 生成的 INSERT 參數 → toCandidateEntity 三個接縫，其中任何一段漏接
// （例如 INSERT 少列一欄、或 toCandidateEntity 忘了 map）都是編譯得過的靜默資料遺失，
// usecase 層的測試看不到 DB。
func TestFinalizeCandidateSet_RoundTripsSellerSnapshot(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	platform := testCandidate("PLATFORM", "prod-1")
	platform.StoreID = identity.StoreID(1001)
	platform.SellerName = "青果行 A"
	platform.LogoKey = "seller/1001/logo.jpg"
	platform.StoreCode = "25ABCDEFGH"

	// EXTERNAL 刻意完全不設這四欄，模擬站外候選。
	external := testCandidate("EXTERNAL", "lead-1")

	wrote, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: req.ID,
		RequestRevisionID: rev.ID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{platform}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{external}},
	})
	require.NoError(t, err)
	require.True(t, wrote)

	_, candidates, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Len(t, candidates, 2)

	byLane := map[string]Candidate{}
	for _, c := range candidates {
		byLane[c.Lane] = c
	}

	got := byLane["PLATFORM"]
	require.Equal(t, identity.StoreID(1001), got.StoreID)
	require.Equal(t, "青果行 A", got.SellerName)
	require.Equal(t, "seller/1001/logo.jpg", got.LogoKey)
	require.Equal(t, "25ABCDEFGH", got.StoreCode)

	ext := byLane["EXTERNAL"]
	require.Zero(t, ext.StoreID, "EXTERNAL 候選不得有平台 store_id（#2377 驗收條件 3）")
	require.Empty(t, ext.SellerName)
	require.Empty(t, ext.LogoKey)
	require.Empty(t, ext.StoreCode)

	// store_id 必須是真的 SQL NULL，不是 0：`WHERE store_id IS NOT NULL` 才能等價於
	// 「有平台賣家的候選」。讀回來的 Go 零值分不出這兩者，所以直接問 DB。
	var nullCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_candidates WHERE candidate_set_id = $1 AND store_id IS NULL`,
		set.ID).Scan(&nullCount))
	require.Equal(t, 1, nullCount, "EXTERNAL 候選的 store_id 應為 SQL NULL 而非 0")
}
