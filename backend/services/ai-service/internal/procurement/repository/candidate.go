// candidate.go 是候選搜尋 run（candidate_set）與其候選、勾選的持久層。
//
// candidate_set 同時扮演兩個角色：一次候選搜尋的「run」生命週期紀錄（QUEUED →
// RUNNING → COMPLETED/PARTIAL/FAILED，帶 lease + fencing_token）與最終候選結果的
// 容器。這個雙重身分的理由見計畫 Design Decisions #1——本檔案的方法簽名沿用那個設計。
//
// Lease／fencing token 機制：每個 run 建立時分配一個 lease（lease_owner + lease_expires_at）
// 與一個 fencing_token（首次固定為 1）。worker 認領 run 時要帶對的 fencing_token 才能把
// QUEUED 轉成 RUNNING；完工時同樣要帶對的 fencing_token 才能寫入終態。若 worker 跑得比
// lease TTL 還久（crash、GC pause、網路分區），GetCandidateSetWithCandidates 會在下一次
// 讀取時就地把這個 run 標成 FAILED/WORKER_LOST（lazy reap，理由見該函式註解）；即使原
// worker 之後真的完工回來，FinalizeCandidateSet 的 WHERE 條件（fencing_token 且
// status NOT IN 終態）會讓它的寫入影響 0 列而被安全丟棄，不會覆蓋掉買家已經看到的失敗結果。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	// observability 是 infrastructure 層的 leaf package（只定義 OTel instruments，
	// 不 import 任何 procurement package），所以 repository import 它不違反 module.go／
	// di.go 建立的「下層不得 import usecase/handler」分層規則——usecase 層本來就以同樣
	// 方式 import 它。
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrMissingRequestRevisionID 是 FinalizeCandidateSet 的輸入驗證，動機與
// revision.go 的 ErrMissingRequestStatus 完全相同：忘記填這個欄位不會壞掉，只會
// 靜默做錯事。RequestRevisionID 零值是 0，pointerInt64(0) 綁出來是一個合法的
// 非 NULL 0，SetProcurementRequestStatusIfActiveRevision 的守衛條件
// （active_revision_id = $3）於是永遠對不上任何真實的列——真實 request 的
// active_revision_id 不可能是 0——那句 UPDATE 就靜默變成 no-op：不報錯、不 panic，
// candidate_set 照樣落地終態，只有 request.status 永遠停在 MATCHING，而且沒有任何
// 路徑會自我修復。寧可在進交易前就大聲擋下來。
//
// 這道閘門在目前唯一的正式呼叫端（usecase.run，永遠帶 set.RequestRevisionID）不會
// 觸發，純粹是防止未來新增呼叫端時漏填。
var ErrMissingRequestRevisionID = errors.New("procurement: FinalizeCandidateSetInput.RequestRevisionID must not be zero")

// FindCandidateSet 依冪等鍵 (request_revision_id, policy_version) 查找既有 candidate_set。
func (r *Repo) FindCandidateSet(ctx context.Context, requestRevisionID int64, policyVersion int) (CandidateSet, error) {
	row, err := r.q(ctx).FindProcurementCandidateSet(ctx, gen.FindProcurementCandidateSetParams{
		RequestRevisionID: requestRevisionID, PolicyVersion: int32(policyVersion),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateSet{}, ErrNotFound
	}
	if err != nil {
		return CandidateSet{}, err
	}
	return toCandidateSetEntity(row), nil
}

type CreateCandidateSetInput struct {
	RequestID         int64
	RequestRevisionID int64
	BuyerID           identity.BuyerID
	PolicyVersion     int
	LeaseOwner        string
	LeaseTTL          time.Duration
}

// CreateCandidateSet 建立一個 QUEUED 狀態的 candidate_set（= run），並在同一交易內把
// procurement_requests.status 轉成 MATCHING（doc 55 §7.5 狀態圖 REQUIREMENT_READY → MATCHING）。
// 冪等鍵撞到時（併發下兩個請求同時建立同一個 revision 的搜尋），回傳既有那筆，不視為錯誤——
// 與 AppendRevision 對 uq_procurement_answer_patches_idempotency 的處理同一個模式。
//
// request 狀態一律走 SetProcurementRequestStatusIfActiveRevision（見 reapCandidateSet
// 註解的完整理由）：本函式雖然跑在買家自己那次 HTTP request 的同步路徑上、race window
// 比背景 finalize 窄，但「這個 run 屬於的 revision 已經不是 request 目前的
// active_revision_id 了就不准寫 request.status」這條不變式與寫入時機無關。三個寫入點
// （Create／Retry／Finalize）統一用同一把守衛，代價是零（守衛條件成立時行為完全相同），
// 換來的是不會有人日後照著其中一個沒守衛的寫法複製出新的漏洞。
//
// ⛔ 交易用 r.withNestableTx 而不是裸的 WithTx：這支沒有交易內的冪等鍵預檢，序列重送
// 就會撞鍵，而 WithTx 巢狀時沿用外層交易，撞鍵因此會把**呼叫端的**交易打成 aborted，
// 下面那次 FindCandidateSet 回查會拿到 25P02。哨兵：
// TestCreateCandidateSet_ReplaysInsideCallerTransaction。
func (r *Repo) CreateCandidateSet(ctx context.Context, in CreateCandidateSetInput) (CandidateSet, error) {
	var out CandidateSet
	leaseExpiresAt := nowUTC().Add(in.LeaseTTL)

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		setID := r.newID()
		row, err := q.CreateProcurementCandidateSet(txCtx, gen.CreateProcurementCandidateSetParams{
			// id 與 run_id 各配一次號：兩者是不同的對外身分（設計書 57 §1.2-3），共用同一個
			// 值就是那條不變式禁止的東西。TSID 產生器同一次程序內不會回同一個值。
			ID: setID, RunID: pointerInt64(r.newID()),
			RequestID: in.RequestID, RequestRevisionID: in.RequestRevisionID,
			BuyerID: int64(in.BuyerID), PolicyVersion: int32(in.PolicyVersion),
			LeaseOwner: in.LeaseOwner, LeaseExpiresAt: nullableTime(leaseExpiresAt),
			// schema DEFAULT 是 0；顯式帶 1 讓它真的寫進 DB row（不只是回傳值），
			// 否則第一個 worker 之後拿著「以為是 1」的 token 呼叫 MarkCandidateSetRunning／
			// FinalizeCandidateSet 時會因為跟 DB 裡的 0 對不上而永遠影響 0 列——這正是
			// fencing_token 用來區分「從未被認領」(0) 與「被 worker 1 認領」(1) 的判準，
			// 只在記憶體裡的回傳值上動手腳並不會讓這個判準生效。
			FencingToken: 1,
		})
		if err != nil {
			return err
		}
		out = toCandidateSetEntity(row)
		// row.ID != setID ⇒ 撞冪等鍵、回傳的是既有列。既有列代表這批候選早就建過，
		// request 狀態也早就被推過，這次不再重推。
		if row.ID != setID {
			return nil
		}

		_, err = q.SetProcurementRequestStatusIfActiveRevision(txCtx, gen.SetProcurementRequestStatusIfActiveRevisionParams{
			ID: in.RequestID, Status: "MATCHING", ActiveRevisionID: pointerInt64(in.RequestRevisionID),
		})
		return err
	})

	if err != nil {
		return CandidateSet{}, err
	}
	return out, nil
}

// RetryCandidateSet 把一個已被 reap 的 FAILED run 轉回 QUEUED、配發新的 lease、遞增
// fencing_token，讓 StartSearch 對「冪等鍵撞到一個死掉的 run」時可以重新派工，而不是
// 永遠回傳同一筆 FAILED 的紀錄——見計畫 Design Decision #7「買家重新呼叫即可拿到新的
// run」。不新增第二列 candidate_set：`(request_revision_id, policy_version)` 上的 unique
// index 本來就會擋掉第二筆，而且「這批候選」確實還是同一批。
//
// **但 run 身分每次嘗試都換新**（`run_id = r.newID()`）：one run = one immutable
// execution attempt。同一個 run_id 重新開機在 ledger 上寫不出第二次嘗試——
// `TerminalizeProcurementAgentRunStatus` 的守衛是「現在還不是終態」，而那一列在第一次
// 嘗試就已經是終態了，於是第二次執行的終態被整段丟掉、只留一行 Warn，串流端永遠等不到
// 收線訊號。呼叫端（usecase.StartSearch）拿到新的 run 身分之後必須為它建一列 ledger run
// （CreateLedgerRunForCandidateSearch），那一列的 domain_ref 仍指回同一個 candidate set
// ——`candidate_sets : agent_runs` 因此是 1:N（doc 57 §6.2 預告的 additive 基數變化），
// 本欄 run_id 是**當前**嘗試的身分，歷史嘗試只在 procurement_agent_runs 上留存。
//
// 新 run 身分在**交易內**配號並隨 RETURNING * 讀回，不由呼叫端事先算好傳進來：呼叫端
// 算好的話，一個輸掉 race（影響 0 列）的呼叫仍然握著一個它以為屬於自己的 run id，很容易
// 拿去建 ledger run，變成一列沒有任何 candidate_set 指向的孤兒 run。
//
// 只對 status='FAILED' 生效（COMPLETED／PARTIAL 這種真正成功的終態不能被覆蓋）；
// 影響 0 列（併發下輸給另一個同時重試的呼叫，或這個 row 已經不是 FAILED 了）回傳
// pgx.ErrNoRows，呼叫端（usecase 層）判斷後退回原本的 idempotent 行為，不視為錯誤。
//
// 跟 CreateCandidateSet 同一個交易形狀：重試也是「(重新)啟動一次搜尋」，必須跟建立
// 時一樣把 procurement_requests.status 轉成 MATCHING（doc 55 §7.5 狀態圖
// REQUIREMENT_READY → MATCHING 這個邊沒有限定只能從「建立新 run」觸發）。少了這一步，
// 重試後的 run 雖然 candidate_set.status 正確顯示 RUNNING，request.status 卻停在
// REQUIREMENT_READY 直到完工才跳 CANDIDATES_READY——買家輪詢 request 快照時會誤以為
// 系統沒有在搜尋，跟首次建立 run 的體驗不一致。request_id 從這句 UPDATE ...
// RETURNING * 的結果取得（candidate_set 本來就帶 request_id 這個欄位），不需要在
// 函式簽名另外加一個 requestID 參數；request_revision_id 也從同一個 RETURNING * 取得，
// 當作 SetProcurementRequestStatusIfActiveRevision 的守衛值（理由見 CreateCandidateSet
// 註解最後一段）。
func (r *Repo) RetryCandidateSet(ctx context.Context, id int64, leaseOwner string, leaseTTL time.Duration) (CandidateSet, error) {
	var out CandidateSet
	leaseExpiresAt := nowUTC().Add(leaseTTL)

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		row, err := q.RetryProcurementCandidateSet(txCtx, gen.RetryProcurementCandidateSetParams{
			ID: id, LeaseOwner: leaseOwner, LeaseExpiresAt: nullableTime(leaseExpiresAt),
			// 每次嘗試各配一次號，同 CreateCandidateSet 對 id／run_id 的配號方式。
			RunID: pointerInt64(r.newID()),
		})
		if err != nil {
			return err
		}
		out = toCandidateSetEntity(row)

		_, err = q.SetProcurementRequestStatusIfActiveRevision(txCtx, gen.SetProcurementRequestStatusIfActiveRevisionParams{
			ID: out.RequestID, Status: "MATCHING", ActiveRevisionID: pointerInt64(out.RequestRevisionID),
		})
		return err
	})
	if err != nil {
		return CandidateSet{}, err
	}
	return out, nil
}

// GetCandidateSetWithCandidates 讀取 candidate_set 與其候選（若已終態）。買家不符回 ErrNotFound。
//
// 兼任 reaper：若讀到的列仍是 QUEUED/RUNNING 但 lease 已過期，就地標成
// FAILED/WORKER_LOST 並把該終態一併寫進 ledger 再回傳——理由見計畫 Design Decisions #7
// （讀取時惰性回收，不開獨立背景 daemon）。買家的一次 GET 因此可能觸發一個跨四張表的
// 寫入交易；那是惰性回收本來就有的代價，ledger 那兩張只是讓「終態必須寫進 ledger」這條
// 硬約束在這條路徑上也成立（詳見 reapCandidateSet）。這個更新不檢查 fencing_token：
// reaper 不打算接手執行，
// 只是宣告「這個 run 死了」，即使原 worker 剛好也在同時間完成，`status` 條件會讓
// 兩者之中只有一個更新真正生效（見 FinalizeCandidateSet 的 WHERE 條件）；reap 本身
// 也會把 fencing_token 遞增一次（見 ReapExpiredProcurementCandidateSet），讓原 worker
// 手上那把已經過期的 token 之後連 MarkCandidateSetRunning／FinalizeCandidateSet 都通不過。
func (r *Repo) GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (CandidateSet, []Candidate, error) {
	row, err := r.q(ctx).GetProcurementCandidateSet(ctx, gen.GetProcurementCandidateSetParams{
		ID: candidateSetID, RequestID: requestID, BuyerID: int64(buyerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateSet{}, nil, ErrNotFound
	}
	if err != nil {
		return CandidateSet{}, nil, err
	}
	return r.candidateSetWithCandidates(ctx, toCandidateSetEntity(row))
}

// GetCandidateSetRunWithCandidates 與上面同樣的讀取，但查詢鍵是 **run 身分**而不是
// candidate set 身分——對外 `GET .../candidate-searches/{runId}` 走這一支。
//
// 兩支不合併成一支帶旗標的函式：ownership 與 reaper 行為完全相同，但「用哪一把身分查」
// 是契約層的區分（設計書 57 §1.2-3），寫成兩個具名入口，呼叫端就不會不小心把
// candidateSetId 餵進 runId 的位置而毫無徵兆地成功。
//
// 三條解析路徑，依序嘗試，全部 miss 才回 ErrNotFound：
//
//  1. **當前嘗試**：candidate_sets.run_id 命中。
//  2. **expand 階段的 legacy 分支**：run_id 欄位是 20260822061013 才加的。在那之前建立
//     的列 run_id 為 NULL，而且部署當下正在輪詢的 browser 手上拿的 runId 就是舊的
//     candidate set id。這兩種情況都會在第一段查詢 miss，於是退回用 id 查一次。
//  3. **被取代的嘗試**：重試會把 candidate_sets.run_id 換成新一次嘗試的身分
//     （見 RetryCandidateSet），前一次嘗試的 run 身分於是在第 1 段永遠 miss。經
//     procurement_agent_runs 的 domain_ref 反查回同一批候選——那是 1:N 的多對一方向。
//     少了這一段，一個在重試發生前就開始輪詢的 browser 會從某一刻起固定收到 404。
//
// 三段都用**逐字相同**的 ownership 條件（requestId ＋ buyerId）。換一把查詢鍵不是放寬
// ownership 的理由。
//
// ⚠️ 退回這條路時，**對外回覆的 runId 必須原樣回傳呼叫端問的那個值**——
// grpc/procurement_handler.go 的 GetCandidateSearch 已經這樣做（echo-back，不是從查到的
// 列重新推導）：client 用 `env.data.runId !== runId` 做輪詢的 freshness guard，回一個它
// 沒問過的值會讓它靜默停在「正在搜尋候選」——不重排 timer、不設 pollError、零錯誤訊號。
// 這支函式本身不需要做任何事來滿足這個不變式（它只負責「查不查得到」），但**不要把
// 它回傳的 CandidateSet.RunID／PublicRunID() 拿去組 handler 的對外 runId**——那正是這裡
// 要避免的推導。contract 階段把 legacy 分支與這條路徑一起移除，echo-back 屆時仍然成立
// （query param 與回覆值永遠相等），不需要跟著改。
func (r *Repo) GetCandidateSetRunWithCandidates(ctx context.Context, runID, requestID int64, buyerID identity.BuyerID) (CandidateSet, []Candidate, error) {
	q := r.q(ctx)

	row, err := q.GetProcurementCandidateSetByRunID(ctx, gen.GetProcurementCandidateSetByRunIDParams{
		RunID: pointerInt64(runID), RequestID: requestID, BuyerID: int64(buyerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		legacy, legacyErr := q.GetProcurementCandidateSet(ctx, gen.GetProcurementCandidateSetParams{
			ID: runID, RequestID: requestID, BuyerID: int64(buyerID),
		})
		if errors.Is(legacyErr, pgx.ErrNoRows) {
			// 第三條解析路徑：這個 runId 可能是**前一次嘗試**的身分。重試會把
			// candidate_sets.run_id 換成新的一次嘗試（見 RetryCandidateSet），舊值於是
			// 在上面兩句都 miss；它仍然是一個合法的、這個買家問得出來的 run 身分，只是
			// 已經被取代。查不到才回 ErrNotFound。
			superseded, supersededErr := q.GetProcurementCandidateSetBySupersededRunID(ctx, gen.GetProcurementCandidateSetBySupersededRunIDParams{
				ID: runID, RequestID: requestID, BuyerID: int64(buyerID),
			})
			if errors.Is(supersededErr, pgx.ErrNoRows) {
				return CandidateSet{}, nil, ErrNotFound
			}
			if supersededErr != nil {
				return CandidateSet{}, nil, supersededErr
			}
			row = superseded
		} else if legacyErr != nil {
			return CandidateSet{}, nil, legacyErr
		} else {
			row = legacy
		}
	} else if err != nil {
		return CandidateSet{}, nil, err
	}

	return r.candidateSetWithCandidates(ctx, toCandidateSetEntity(row))
}

// candidateSetWithCandidates 是兩個查詢入口共用的後半段：lazy reap ＋（成功終態才）讀候選。
func (r *Repo) candidateSetWithCandidates(ctx context.Context, set CandidateSet) (CandidateSet, []Candidate, error) {
	q := r.q(ctx)

	if (set.Status == codemeta.ProcurementRunStatusQueued.Value || set.Status == codemeta.ProcurementRunStatusRunning.Value) &&
		set.LeaseExpiresAt != nil && set.LeaseExpiresAt.Before(nowUTC()) {
		reaped, err := r.reapCandidateSet(ctx, set)
		if err != nil {
			return CandidateSet{}, nil, err
		}
		set = reaped
	}

	// 只有成功終態才讀候選。expand 階段要同時認新舊兩組值：新版寫 SUCCEEDED，
	// rolling update 期間舊 pod 仍在寫 COMPLETED／PARTIAL。漏認任一個，那些 run 的候選
	// 就讀不出來，對外變成「終態但零候選」——靜默失效，沒有錯誤訊號。
	// contract 階段舊值消失後，這裡收斂成只比對 SUCCEEDED。
	var candidates []Candidate
	if isSucceededRunStatus(set.Status) {
		rows, err := q.ListProcurementCandidatesBySet(ctx, set.ID)
		if err != nil {
			return CandidateSet{}, nil, err
		}
		candidates = make([]Candidate, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, toCandidateEntity(row))
		}
	}
	return set, candidates, nil
}

// reapCandidateSet 把一個 lease 過期的 QUEUED/RUNNING run 標成 FAILED/WORKER_LOST，
// 把這次終態寫進 ledger（terminalize agent_runs ＋ append RUN_FAILED），並（只在條件
// 成立時）把 request 狀態退回 REQUIREMENT_READY。
//
// 三個寫入包在同一個交易內：原本各自獨立呼叫時，reap 那句一旦成功、緊接著的 request 狀態
// 寫入若失敗（連線中斷／timeout），candidate_set 會停在 FAILED 但 request 永遠卡在
// MATCHING，沒有任何路徑能自我修復，而錯誤還被原地吞掉（`_ = ...`）從不曝光。三者現在
// 要嘛一起 commit、要嘛一起 rollback，呼叫端會收到錯誤（下一次讀取會重新嘗試 reap）。
//
// request 狀態的寫入額外用 SetProcurementRequestStatusIfActiveRevision 而非無條件的
// SetProcurementRequestStatus：買家可能在這個 run 的 lease 過期之前就已經送出下一輪
// turn，把 request 推進到一個更新的 revision／狀態（例如又回到 CLARIFYING）。一個遲來
// 的 reap 不該對著一個「早就不是目前這一版」的 candidate_set 動手、把 request 的最新
// 狀態蓋回 REQUIREMENT_READY——用 candidate_set.RequestRevisionID 是否仍等於 request 目前
// 的 active_revision_id 當守門條件，不符合就讓這句 UPDATE 靜默變成 no-op。
//
// 傳進來的 set 是呼叫端在「開這個交易之前」讀到的快照，其中「lease 已過期」這個判斷可能
// 在這中間就失效了（另一個讀者先 reap 成 FAILED、接著一次 POST candidate-searches 把同
// 一列重試回 QUEUED 並配發新 lease）。ReapExpiredProcurementCandidateSet 因此在 SQL 的
// WHERE 裡自己再驗一次 `lease_expires_at < now()`，不倚賴這份快照——affected==0 時走下面
// 的重讀分支，回傳 DB 當下的真實狀態。
func (r *Repo) reapCandidateSet(ctx context.Context, set CandidateSet) (CandidateSet, error) {
	out := set
	var reaped, terminalized bool
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		row, err := q.ReapExpiredProcurementCandidateSet(txCtx, set.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			// 影響 0 列：輸給真正的 worker 或另一個併發的 reap，重讀一次拿最新狀態。
			refreshed, err := q.GetProcurementCandidateSetByID(txCtx, set.ID)
			if err != nil {
				return err
			}
			out = toCandidateSetEntity(refreshed)
			return nil
		}
		if err != nil {
			return err
		}

		// 直接用 RETURNING 讀回的那一列，不在 Go 端手抄一份 SET 子句的鏡像。
		out = toCandidateSetEntity(row)
		reaped = true

		// 這次 reap 就是這個 run 的終態，必須同步寫進 ledger——「每個 run 的終態必須寫進
		// ledger，不論它由哪一條路徑產生」是硬約束（2026-09-07 決策）：ledger 是串流收線
		// 訊號的唯一來源，少了它，worker 死掉的 run 會停在 QUEUED／RUNNING，SSE 一路開到
		// gateway 的 deadline，前端 onRunFailed 永遠不會被呼叫。
		//
		// 終態化的對象是 out.PublicRunID()——**這一列當前掛著的那次嘗試**，取自剛剛
		// RETURNING 回來的交易內狀態，不是呼叫端開交易前的那份快照（重試可能已經換過
		// run_id，見該 query 的註解）。
		//
		// 跟 candidate_set 的終態同一個交易：兩者要嘛一起 commit、要嘛一起 rollback。
		// 分開寫的話會出現「candidate_set 已 FAILED 但 ledger 沒有終態」——那正是本來就
		// 存在、且沒有任何路徑能自我修復的狀態。
		//
		// 只補 ledger 終態，**不產 ERROR_RECOVERY artifact**：那是刻意的不對稱（同一份
		// 決策的選項 A）。ErrorRecoveryData 的三欄在 RUN_FAILED 的 payload 裡已經有了，
		// 而 artifact 額外提供的 version／checksum／可回放對「worker 掛了、重試一次就好」
		// 沒有消費端；要產它得讓 producer 下沉到本層的交易或讓 reap 上浮到 usecase，兩種
		// 都動分層。見 TestReapCandidateSet_DoesNotProduceErrorRecoveryArtifact。
		//
		// wrote=false 不是錯誤，而且是**可達**的正常情況：expand 階段之前的舊列（run_id
		// 為 NULL）與 CreateLedgerRunForCandidateSearch 失敗那條只記 log 的路徑，都沒有
		// 對應的 agent_runs 列。不能讓那些列的 reap 跟著 rollback——那會讓它們永遠卡在
		// QUEUED，而 ledger 本來就是投影不是 SSOT。
		terminalized, err = r.terminalizeRunTx(txCtx, TerminalizeRunInput{
			RunID: out.PublicRunID(), BuyerID: out.BuyerID,
			Status:        codemeta.ProcurementRunStatusFailed.Value,
			FailureReason: codemeta.ProcurementRunFailureReasonWorkerLost.Value,
			// 這一支的 Version 只有非終態的 park 分支會用到，FAILED 走終態分支，
			// 守衛是「現在還不是終態」——刻意不帶（2026-09-07 決策）。
			TerminalEventType: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
			PublicPayloadJSON: domain.RunFailedPayload(codemeta.ProcurementRunFailureReasonWorkerLost.Value),
		})
		if err != nil {
			return err
		}

		_, err = q.SetProcurementRequestStatusIfActiveRevision(txCtx, gen.SetProcurementRequestStatusIfActiveRevisionParams{
			ID: out.RequestID, Status: "REQUIREMENT_READY", ActiveRevisionID: pointerInt64(out.RequestRevisionID),
		})
		return err
	})
	if err != nil {
		return CandidateSet{}, err
	}

	// 喚醒訂閱端。跟下面的 metric 同樣以 commit 成功為前提、也同樣放在交易外：從交易內發
	// NOTIFY 會早於 commit，訂閱端被叫醒去查 DB 卻什麼都看不到，然後回去等下一個永遠不會
	// 來的訊號——收線因此延後到 SSE 自己的 poll 間隔或 deadline。
	if terminalized {
		r.notifyRunWake(out.PublicRunID())
	}

	// 一次成功的 reap 就是這個 run 的終態寫入——跟 FinalizeCandidateSet 寫終態是同一件事的
	// 另一條路徑，所以同樣要計進 procurement_run_total（doc 55 §8.2 指定的主要 run 健康
	// 訊號）。少了這一句，WORKER_LOST 這整類終態對那個 counter 完全隱形：worker 大批死亡時
	// counter 只會「安靜地不再增加」，跟流量下降無法區分。
	//
	// 計數點刻意放在交易外、且以 commit 成功為前提：放在交易函式裡的話，後面那句
	// SetProcurementRequestStatusIfActiveRevision 失敗導致整個交易 rollback 時，metric 已經
	// 加過了——counter 會記到一個從未落地的終態，而且不可逆（monotonic counter 不能減）。
	// reaped == false（affected == 0）代表這次 reap 落空、輸給真正的 worker 或另一個併發
	// reap：那筆終態不是這裡寫的，由真正寫進去的人計。
	if reaped {
		observability.RecordProcurementRun(ctx, codemeta.ProcurementRunStatusFailed.Value)
	}
	return out, nil
}

// MarkCandidateSetRunning 把 QUEUED 轉成 RUNNING，以 fencing_token 防呆
// （若 token 不符，代表這個 run 的 lease 已經被 reaper 或另一個 worker 動過）。
//
// 回傳的 bool 是「這個 worker 有沒有真的認領到這個 run」：底層 SQL 的 WHERE 是
// `id = $1 AND fencing_token = $2 AND status = 'QUEUED'`，本來就是一個 claim-check，
// 影響 0 列代表 row 已經是終態、或已經被 reaper／另一次重試搶走。影響 0 列不是 error
// （沒有任何東西壞掉），但也絕不是成功——呼叫端把它跟「認領成功」混為一談的話，會繼續
// 去做整趟雙 lane 搜尋（兩次真實的跨服務 gRPC 呼叫：product-service 與
// seller-service），而那些結果之後一定會被 FinalizeCandidateSet 的同一組防呆條件丟掉，純屬白工，
// 還會讓 procurement_run_total 這個 metric 多算一筆。所以這裡把 rows-affected 如實回報，
// 由呼叫端（usecase.run）決定放棄。
func (r *Repo) MarkCandidateSetRunning(ctx context.Context, id, fencingToken int64) (bool, error) {
	affected, err := r.q(ctx).MarkProcurementCandidateSetRunning(ctx, gen.MarkProcurementCandidateSetRunningParams{
		ID: id, FencingToken: fencingToken,
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

type LaneResult struct {
	Status     string // OK|FAILED
	Candidates []Candidate
}

type FinalizeCandidateSetInput struct {
	CandidateSetID int64
	FencingToken   int64
	RequestID      int64
	// RequestRevisionID 是這個 run 所屬的 revision，只用來當 request 狀態寫入的守衛值
	// （見 FinalizeCandidateSet 註解第二段）——不是拿來寫進 candidate_set 的資料。
	RequestRevisionID int64
	Platform          LaneResult
	External          LaneResult
}

// FinalizeCandidateSet 寫入雙 lane 的候選並把 candidate_set／request 轉成終態，
// 全程以 (id, fencing_token, status NOT IN 終態) 防呆：若 reaper 已經在這之間把這個 run
// 標成 FAILED，這裡的更新會影響 0 列，直接視為「run 已死，放棄寫入」，不覆蓋 reaper 的結果——
// 避免一個跑得比 lease TTL 還久的 worker 復活後把候選寫進一個買家已經被告知失敗的 run。
//
// request 狀態的寫入另外用 SetProcurementRequestStatusIfActiveRevision（同 reapCandidateSet）：
// fencing_token 只保護 candidate_set 這一列，擋不住「run 還活著、但買家已經送出下一輪
// turn」這條路徑。run() 是背景 goroutine，HTTP handler 回應後最久可能再跑 10 秒
// （candidateSearchBudget）；這段期間買家若再送一次對話，AppendRevision 會在同一個交易裡
// 把 active_revision_id 前推、並把 request.status 寫成 CLARIFYING。舊 run 接著 finalize，
// 無守衛的寫入就會把 CLARIFYING 蓋成 CANDIDATES_READY／REQUIREMENT_READY——doc 55 §7.5
// 狀態圖沒有這條邊。REQUIREMENT_READY 那個分支尤其危險：handler 的 PostCandidateSearch
// 直接拿 req.Status == "REQUIREMENT_READY" 當 RequirementReady，等於讓下一次
// POST candidate-searches 對著一個必填 slot 還沒補齊的需求通過 ErrRequirementNotReady
// 那道閘門。守衛條件用「這個 run 的 request_revision_id 是否仍是 request 的
// active_revision_id」，不符就讓這句 UPDATE 靜默 no-op（candidate_set 本身仍然正常落地
// 終態與候選——那些資料屬於舊 revision，買家改用舊 candidate_set_id 讀取時仍是正確的）。
//
// 每個候選的 ID 在這裡才用 r.newID() 指派（Task 6/7 的 lane searcher 回傳的
// repository.Candidate 不帶 ID）：指派 TSID 主鍵是持久層職責，不是業務邏輯職責。
//
// 回傳的 wrote 是「終態真的被寫進去了嗎」。上面那段防呆讓「run 已死／已定案」與「寫入
// 成功」都回傳 nil error——兩者對持久層而言確實都不是錯誤，但對呼叫端不是同一件事：
// usecase.run 靠它決定要不要記一筆 procurement_run_total。沒有這個布林值時，一個 zombie
// worker 被丟掉的 finalize 也會被計成一次真實的 COMPLETED/PARTIAL/FAILED，讓 doc 55 §8.2
// 指定的主要 run 健康訊號憑空多算（同 MarkCandidateSetRunning 回傳 claimed 的理由）。
// 回傳的 finalStatus 是「這次真的寫進 DB 的 run 終態」。它存在的理由是消掉一份重複的判定：
// 呼叫端要用終態當 procurement_run_total 的 label，而在此之前 usecase 自己照著同一組 lane
// 狀態再推導一次——兩份邏輯各自演化的話，metric 上的終態會與 DB 裡的終態悄悄分家，
// 而那正是這個 metric 唯一的用途（doc 55 §8.2 的主要 run 健康訊號）。
// wrote 為 false 時 finalStatus 是空字串（沒有東西被寫入，呼叫端本來就不該計數）。
// failureReason 是第三個回傳值，理由與 finalStatus 逐字相同：run 失敗的原因由這裡
// 依 lane 狀態判定並寫進 DB，呼叫端若自己再推導一次，wire 上的原因就會與 DB 裡的
// 原因各自演化。RUN_FAILED 的 ledger payload 需要它（doc57 §4.2 → 原生 RUN_ERROR
// 的 reason）。非 FAILED 時為空字串。
func (r *Repo) FinalizeCandidateSet(ctx context.Context, in FinalizeCandidateSetInput) (wrote bool, finalStatus string, failureReasonOut string, err error) {
	if in.RequestRevisionID == 0 {
		return false, "", "", ErrMissingRequestRevisionID
	}

	// 只有「兩條 lane 都失敗」才是 run 失敗。單一 lane 失敗曾經寫成 PARTIAL，該值已從
	// ProcurementRunStatus 移除——lane 之下沒有子單位，「部分成功」在 run 主軸上沒有定義，
	// 改由「run SUCCEEDED + 該 lane FAILED」表達（同 Elasticsearch 用 _shards.failed 而非
	// 在總狀態上開一個 partial 值）。「哪條 lane 失敗」原本就記在 lane 欄位、不在 status，
	// 所以刪掉 PARTIAL 不遺失任何資訊。
	status := codemeta.ProcurementRunStatusSucceeded.Value
	failureReason := ""
	if in.Platform.Status == codemeta.ProcurementLaneStatusFailed.Value &&
		in.External.Status == codemeta.ProcurementLaneStatusFailed.Value {
		status = codemeta.ProcurementRunStatusFailed.Value
		// 在此之前這條路徑不寫 failure reason，於是 CodeMeta 登記的 LANE_ALL_FAILED 是一個
		// 沒有任何寫入路徑的死值，而契約明寫「FAILED 時 reason 必非空」。唯一會寫 reason 的
		// 是 reap 的 WORKER_LOST，等於 run 失敗的兩種成因裡有一種永遠查不到原因。
		failureReason = codemeta.ProcurementRunFailureReasonLaneAllFailed.Value
	}

	requestStatus := "CANDIDATES_READY"
	if status == codemeta.ProcurementRunStatusFailed.Value {
		requestStatus = "REQUIREMENT_READY"
	}

	err = r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		// 這句條件式 UPDATE 刻意排在候選 INSERT 之前，不是之後：它同時扮演「這個 run
		// 還活著嗎」的判斷式，與（透過 UPDATE 隱含的 row lock）序列化併發 finalize 呼叫
		// 的手段。若晚於 INSERT 才做這個檢查，即使這句因為 fencing_token／status 對不上
		// 而影響 0 列、函式接著提早 return nil，前面已經執行的 INSERT 仍然會隨著這個
		// transaction 一起 commit——0 列只會擋住 status 欄位跟 request 狀態的寫入，完全
		// 不會擋住候選資料本身，等於白防（曾經的真實 bug：reap 之後的 zombie finalize、
		// 或單純對同一個已終態的 run 重送 finalize，都會讓候選被寫入第二次）。
		affected, err := q.FinalizeProcurementCandidateSet(txCtx, gen.FinalizeProcurementCandidateSetParams{
			ID: in.CandidateSetID, Status: status, PlatformLaneStatus: in.Platform.Status,
			ExternalLaneStatus: in.External.Status, FailureReason: failureReason,
			FencingToken: in.FencingToken,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			// lease 已被 reaper 收回、或這個 run 已經被別的呼叫終結過——run 已死或已經
			// 定案，整段放棄寫入（含候選本身，因為候選 INSERT 根本還沒執行到），不能讓
			// 遲到／重送的呼叫把資料寫進一個買家已經看到結果的 run。
			// wrote 維持 false：呼叫端據此不記 metric（見函式註解最後一段）。
			return nil
		}
		wrote = true

		all := make([]Candidate, 0, len(in.Platform.Candidates)+len(in.External.Candidates))
		all = append(all, in.Platform.Candidates...)
		all = append(all, in.External.Candidates...)
		for _, c := range all {
			if _, err := q.CreateProcurementCandidate(txCtx, gen.CreateProcurementCandidateParams{
				ID: r.newID(), CandidateSetID: in.CandidateSetID, Lane: c.Lane, SourceID: c.SourceID,
				DisplayName: c.DisplayName, Score: int32(c.Score),
				MatchedFactsJson: c.MatchedFactsJSON, UnverifiableFactsJson: c.UnverifiableFactsJSON,
				FactsAsOf: timestamptzParam(c.FactsAsOf), Rank: int32(c.Rank),
				// #2377 賣家身分快照。StoreID==0（EXTERNAL lane 恆為此）寫成 SQL NULL 而不是
				// 0，讓「沒有平台賣家」在 DB 層就與「store 0」分得開。
				StoreID:       storeIDParam(c.StoreID),
				SellerName:    c.SellerName,
				LogoKey:       c.LogoKey,
				StoreCode:     c.StoreCode,
				BrandLogoKey:  c.BrandLogoKey,
				BrandCoverKey: c.BrandCoverKey,
				// EXTERNAL lane 恆有值、PLATFORM 恆為 NULL——與 StoreID 互斥
				// （StoreID 的既有註解已說明 EXTERNAL 恆為 0 寫成 SQL NULL）。
				ProspectID: c.ProspectID,
			}); err != nil {
				return err
			}
		}

		_, err = q.SetProcurementRequestStatusIfActiveRevision(txCtx, gen.SetProcurementRequestStatusIfActiveRevisionParams{
			ID: in.RequestID, Status: requestStatus, ActiveRevisionID: pointerInt64(in.RequestRevisionID),
		})
		return err
	})
	if err != nil {
		// 交易 rollback 了，wrote 在交易函式內設的 true 不算數——沒有任何東西落地。
		return false, "", "", err
	}
	if !wrote {
		return false, "", "", nil
	}
	return true, status, failureReason, nil
}

type UpsertSelectionInput struct {
	RequestID         int64
	RequestRevisionID int64
	BuyerID           identity.BuyerID
	CandidateSetID    int64
	Lane              string
	CandidateID       int64
	Selected          bool
}

// UpsertSelection 以 (request_revision_id, lane, candidate_id) 為鍵 upsert 勾選狀態
// （doc 55 §8.4：重複同值勾選為 no-op，取消勾選是真正的狀態轉換）。`RETURNING *` 直接拿回
// 持久化後的正確 row（id 沿用第一次 insert 的值，因為 id 不在 ON CONFLICT 的 SET 子句
// 裡）——比原 GORM 版本簡單，GORM 那邊因為 OnConflict 之後拿不到可靠的 RETURNING，
// 才需要額外一次 SELECT 才能確保拿到的是持久化後的正確狀態。
func (r *Repo) UpsertSelection(ctx context.Context, in UpsertSelectionInput) (CandidateSelection, error) {
	row, err := r.q(ctx).UpsertProcurementCandidateSelection(ctx, gen.UpsertProcurementCandidateSelectionParams{
		ID: r.newID(), RequestID: in.RequestID, RequestRevisionID: in.RequestRevisionID,
		BuyerID: int64(in.BuyerID), CandidateSetID: in.CandidateSetID, Lane: in.Lane,
		CandidateID: in.CandidateID, Selected: in.Selected,
	})
	if err != nil {
		return CandidateSelection{}, err
	}
	return toCandidateSelectionEntity(row), nil
}

// GetCandidate 讀單一候選，供勾選前驗證候選確實屬於指定的 candidate_set。
func (r *Repo) GetCandidate(ctx context.Context, candidateSetID, candidateID int64) (Candidate, error) {
	row, err := r.q(ctx).GetProcurementCandidate(ctx, gen.GetProcurementCandidateParams{
		ID: candidateID, CandidateSetID: candidateSetID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{}, ErrNotFound
	}
	if err != nil {
		return Candidate{}, err
	}
	return toCandidateEntity(row), nil
}

// ListSelectedCandidateIDs 回傳一個 candidate_set 底下目前有效勾選（selected=true）的
// candidate_id 集合，供 GetRun 組裝候選卡時附加 selected 欄位。回傳 map 而非 slice：
// 呼叫端要做的是逐張候選卡查「這張選了嗎」，map 查詢是 O(1)，slice 每次都要線性掃。
// 沒有任何列被勾選時回傳空 map（非 nil），呼叫端對空 map 的查詢一律得到 false，
// 不需要額外判斷 nil。
func (r *Repo) ListSelectedCandidateIDs(ctx context.Context, candidateSetID int64) (map[int64]bool, error) {
	ids, err := r.q(ctx).ListSelectedProcurementCandidateIDs(ctx, candidateSetID)
	if err != nil {
		return nil, err
	}
	selected := make(map[int64]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	return selected, nil
}

// storeIDParam 把 identity.StoreID 轉成 nullable *int64，0 視為 NULL。
//
// EXTERNAL lane 的候選來自 seller-service 的 seller_prospects（站外業者），沒有平台 store，所以 0 是
// 「沒有」而不是「store 編號 0」。寫成 NULL 讓這個區別在 DB 層就成立：`WHERE
// store_id IS NOT NULL` 才是「有平台賣家的候選」，用 `!= 0` 過濾會漏掉語意。
func storeIDParam(id identity.StoreID) *int64 {
	if id == 0 {
		return nil
	}
	return pointerInt64(int64(id))
}
