package repository

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ledgerTraceID 取目前 span 的 trace id，沒有 active span 時回空字串。
//
// procurement_agent_run_events.trace_id 是把一列 ledger 事件接回 log／trace
// 的唯一鍵——ProcurementRunFailureReason 的 CodeMeta 定義刻意讓 failure_reason
// 維持粗粒度（避免 procurement_run_total 的 label cardinality 爆掉），並明文
// 把「細節」指向 log／trace，所以這個欄位不是可有可無的裝飾，它是那句話成立
// 的前提。
//
// 2026-09-03 稽核發現這條路整條是斷的：7 天 1076 筆事件的 trace_id 全部是空
// 字串，7 個寫入點裡有 6 個連參數都沒帶，剩下那一個的
// AppendRunEventInput.TraceID 全 repo 零個呼叫端填過。一個 FAILED 的 run 因此
// 沒有任何鍵可以查回它為什麼失敗。
//
// 從 ctx 取而不是要求呼叫端傳：寫入點分散在 claim／append／terminalize 等多條
// 路徑上，逐一加參數只會讓下一個新增的寫入點再漏一次；放在這一層，現在與未來
// 的呼叫端都自動涵蓋。取法與 pkg/middleware/grpc/interceptor 的 logging
// interceptor 一致，兩邊記出來的 trace_id 因此可以直接對上。
func ledgerTraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// int64OrNil 把「0 代表未設定」的 int64 轉成 *int64：0 → nil（NULL），其餘 →
// 指向該值的指標。UpdateProcurementAgentRunStatus 的 result_revision_id 參數
// 用 COALESCE(sqlc.narg(result_revision_id), result_revision_id) 保留原值後
// （Task 14b C1），呼叫端必須明確傳 nil（而不是字面值 0）才不會把它覆寫成
// 0——那會抹掉 replayExistingRun 的就緒判準（`run.ResultRevisionID == 0`，
// answer.go／turn.go），也會讓 ledger 指向一個不存在的 revision id。
func int64OrNil(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return pointerInt64(v)
}

// ErrIdempotencyConflict 對應 doc57 §6.4-2：同 (buyer_id, client_request_id)
// 但 payload hash 不同——409 IDEMPOTENCY_KEY_CONFLICT，零 mutation。
var ErrIdempotencyConflict = errors.New("procurement: idempotency key conflict")

type Run struct {
	ID               int64
	BuyerID          identity.BuyerID
	ConversationID   int64
	ParentRunID      int64
	RunType          domain.RunType
	DomainRefType    string
	DomainRefID      string
	Status           string
	FailureReason    string
	ResumedIntoRunID int64
	Version          int64
	LastEventSeq     int64
	MinAvailableSeq  int64
	ResultRevisionID int64
	ControllerKind   domain.ControllerKind
}

type RunEvent struct {
	ID                int64
	RunID             int64
	Seq               int64
	EventType         string
	SchemaVersion     string
	PublicPayloadJSON []byte
	OccurredAt        time.Time
}

// ClaimRunInput 是 claim transaction 的完整輸入（doc57 §5.1）。CommandKind 是
// `SUBMIT_TURN`／`CANCEL`／`RESUME`／`START_CANDIDATE_SEARCH` 等，跟
// PayloadHash **各自**組成冪等判準的一半：同 key 換 command kind 一律衝突，
// 而且是由 ClaimRun 顯式比對得到的，不倚賴「兩種 kind 的 hash 必然不同」。
type ClaimRunInput struct {
	BuyerID         identity.BuyerID
	ClientRequestID string
	CommandKind     string
	PayloadHash     string
	// ConversationID 是呼叫端已經確知的 conversation id 時的直接覆寫——目前沒有
	// 任何呼叫端這樣用（TURN／ANSWER／REWIND 都只知道 RequestID，見下欄），保留
	// 這個欄位是給未來若真的有呼叫端一開始就手上有 conversation id 的路徑用，
	// 不強迫它多繞一次 RequestID 解析。
	ConversationID int64
	// RequestID 是 Task 15 write-path 修復新增的欄位：TURN／ANSWER／REWIND 三個
	// 既有呼叫端在 claim 當下通常已經知道自己在哪個 request 上操作（ANSWER／
	// REWIND 恆非零；TURN 只有「這是全新對話」時才是 0），ClaimRun 用它解析出
	// conversation_id 一起寫進新建的 run row——見下方 ClaimRun 內文與
	// conversationIDForRequest。ConversationID 若已非零則優先採用該值，
	// RequestID 只在 ConversationID 為零時才觸發解析。
	RequestID   int64
	ParentRunID int64
	RunType     domain.RunType
	// ControllerKind records which non-legacy path is claiming this run
	// (Sub-project 3b) — ClaimAndDispatchTurn passes DURABLE,
	// ClaimAndDispatchTurnViaGraph passes GRAPH (Task 2).
	ControllerKind domain.ControllerKind
	DomainRefType  string
	DomainRefID    string
}

// ClaimRun 是 doc57 §5.1「Durable command」交易的落地。回傳的 bool 是
// 「這是不是這次呼叫新建的 run」——false 代表命中既有冪等鍵，呼叫端不得再次
// dispatch 背景執行（避免重跑 pipeline）。
//
// 命中但 hash 不同：回 ErrIdempotencyConflict，交易 ROLLBACK，零 mutation
// （doc57 §6.4-2）。
//
// 交易內的「先查再寫」（FindProcurementAgentRunCommandByIdempotencyKey 沒命中
// 才 INSERT）只在無併發時保證正確；真正併發撞上同一個 (buyer_id,
// client_request_id) 時，兩個 goroutine 都可能在對方尚未 commit 前查到「不
// 存在」而同時往下寫——輸的那個 transaction 會在 INSERT run command 時卡住
// 等贏家的 row lock，贏家 commit 後才解鎖，此時 Postgres 重新檢查唯一索引，
// 直接對輸家的 INSERT 噴 23505 unique_violation，交易 ROLLBACK。若原封不動
// 把這個裸錯誤丟給呼叫端，就違背了本函式文件承諾的冪等保證，也讓
// TestClaimRun_ConcurrentSameKey_OnlyOneRunCreated 名不符實。
//
// 修法跟 AppendRevision 對 uq_procurement_answer_patches_idempotency 的處理
// 同一個形狀（revision.go isUniqueViolation／recoverIdempotentAppend）：
// 偵測到撞這一個 constraint 時，在交易外（aborted transaction 內無法繼續
// 查詢）重新查一次冪等鍵，回傳跟贏家一致的結果——hash 相符就回放贏家的
// run，不符就回 ErrIdempotencyConflict，兩者都跟「一開始就命中既有冪等鍵」
// 那條路徑的結果完全一致。
//
// ⛔ command 先插、run 後建：command 帶冪等鍵，撞到就代表併發輸了，此時不能建 run。
// 撞鍵不拋錯（ON CONFLICT DO UPDATE，見 PostgreSQL 規範第十二章），所以交易保持健康，
// recoverClaimedRun 用 r.q(ctx) 查得回來，呼叫端有沒有交易都一樣。
// 交易內那道預檢擋得住序列重送，但擋不住這條路徑——真正的併發下兩邊都會預檢落空。
// 哨兵：TestClaimRun_ConcurrentLoserInsideCallerTransactionReplays。
func (r *Repo) ClaimRun(ctx context.Context, in ClaimRunInput) (Run, bool, error) {
	var out Run
	isNew := false
	lostRace := false

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		existingCmd, err := q.FindProcurementAgentRunCommandByIdempotencyKey(txCtx, gen.FindProcurementAgentRunCommandByIdempotencyKeyParams{
			BuyerID: int64(in.BuyerID), ClientRequestID: in.ClientRequestID,
		})
		if err == nil {
			// kind 與 hash 都相符才算合法 replay。只比 hash 的話，跨 command
			// kind 的撞鍵會靜默回放成另一種命令的結果。今天三支 canonical
			// hash 的 struct 形狀剛好互不相同所以撞不到，但那是巧合不是保證
			// ——canonicalHash 的 marshal-error sentinel 就讓所有序列化失敗
			// 的命令共用同一個 hash。ResumeRun 的同一位置早就有這道檢查
			// （existingCmd.CommandKind != "RESUME"），這裡補上讓兩條路徑
			// 的判準一致。
			if existingCmd.CommandKind != in.CommandKind || existingCmd.PayloadHash != in.PayloadHash {
				return ErrIdempotencyConflict
			}
			runRow, err := q.GetProcurementAgentRun(txCtx, gen.GetProcurementAgentRunParams{
				ID: existingCmd.RunID, BuyerID: int64(in.BuyerID),
			})
			if err != nil {
				return err
			}
			out = toRunEntity(runRow)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// conversationID 解析（Task 15 write-path 修復）：ConversationID 若已由
		// 呼叫端直接帶入則優先採用；否則在 RequestID 非零時經由
		// conversationIDForRequest 查一次——ANSWER／REWIND 的 RequestID 恆非零
		// （續接既有對話），一定解得出來；TURN 只有「這是全新對話」時 RequestID
		// 才是 0（此時 conversation 要等 executeTurnPipeline 呼叫 CreateRequest
		// 才誕生），這裡自然解不出來、conversationID 維持 0，run 建立時
		// conversation_id 寫 NULL，之後由 executeTurnPipeline 呼叫
		// SetRunConversationID 回填（見該函式文件）。
		conversationID := in.ConversationID
		if conversationID == 0 && in.RequestID != 0 {
			conversationID, err = r.conversationIDForRequest(txCtx, in.RequestID)
			if err != nil {
				return err
			}
		}

		runID := r.newID()

		// command 先插、run 後建（兩者之間沒有 FK）：command 帶的是冪等鍵，撞到就代表
		// 併發輸了，此時不能建 run——反過來的順序會先寫出一個沒人指向的 run。
		cmdID := r.newID()
		cmdRow, err := q.CreateProcurementAgentRunCommand(txCtx, gen.CreateProcurementAgentRunCommandParams{
			ID: cmdID, BuyerID: int64(in.BuyerID), CommandKind: in.CommandKind,
			ClientRequestID: in.ClientRequestID, PayloadHash: in.PayloadHash,
			RunID: runID, Status: "ACCEPTED",
		})
		if err != nil {
			return err
		}
		if cmdRow.ID != cmdID {
			lostRace = true
			return nil
		}

		runRow, err := q.CreateProcurementAgentRun(txCtx, gen.CreateProcurementAgentRunParams{
			ID: runID, BuyerID: int64(in.BuyerID),
			ConversationID: int64OrNil(conversationID), ParentRunID: pointerInt64(in.ParentRunID),
			RunType: string(in.RunType), DomainRefType: in.DomainRefType, DomainRefID: in.DomainRefID,
			Status: domain.RunStatusQueued(), LastEventSeq: 1,
			ControllerKind: string(in.ControllerKind),
		})
		if err != nil {
			return err
		}

		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: runID, Seq: 1, EventType: codemeta.ProcurementLedgerEventTypeRunAccepted.Value,
			SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}

		out = toRunEntity(runRow)
		out.LastEventSeq = 1
		isNew = true
		return nil
	})
	if err != nil {
		return Run{}, false, err
	}
	if lostRace {
		out, err = r.recoverClaimedRun(ctx, in.BuyerID, in.ClientRequestID, in.CommandKind, in.PayloadHash)
		if err != nil {
			return Run{}, false, err
		}
	}
	return out, isNew, nil
}

// recoverClaimedRun 在 ClaimRun 因併發撞上 (buyer_id, client_request_id)
// 冪等鍵而 ROLLBACK 之後，於交易外重新讀出贏家寫入的 run command，回傳跟
// 贏家一致的結果——見 ClaimRun 文件的併發說明。isNew 固定回傳 false：這次
// 呼叫沒有真的建立新 run，即使贏家那邊的 isNew 是 true。
func (r *Repo) recoverClaimedRun(ctx context.Context, buyerID identity.BuyerID, clientRequestID, commandKind, payloadHash string) (Run, error) {
	q := r.q(ctx)
	existingCmd, err := q.FindProcurementAgentRunCommandByIdempotencyKey(ctx, gen.FindProcurementAgentRunCommandByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		return Run{}, err
	}
	if existingCmd.CommandKind != commandKind || existingCmd.PayloadHash != payloadHash {
		return Run{}, ErrIdempotencyConflict
	}
	runRow, err := q.GetProcurementAgentRun(ctx, gen.GetProcurementAgentRunParams{
		ID: existingCmd.RunID, BuyerID: int64(buyerID),
	})
	if err != nil {
		return Run{}, err
	}
	return toRunEntity(runRow), nil
}

// CreateLedgerRunForCandidateSearch 是 candidate_search 既有 lease/fencing
// domain 之外的 ledger 投影入口——不經過 ClaimRun 的冪等判斷（那層冪等已由
// procurement_candidate_sets 自己的唯一索引擋掉），直接用 candidate_set 的
// PublicRunID() 當這筆 ledger run 的 id（沿用同一個對外身分，不另外配一個
// TSID，避免同一個 run 在兩個地方各有一個不同的 id）。domain_ref 指向
// candidateSetId（doc57 §1.2-3／`domainRef` 的既有用法）。
//
// 呼叫端對**每一次嘗試**呼叫一次（見 usecase.CandidateSearchUseCase 的
// createLedgerRunProjection：新建與重試兩條路徑都走它）。重試會換一個新的
// run_id，所以這裡建的是同一個 candidate set 底下的第 N 列——`candidate_sets :
// agent_runs` 是 1:N，domain_ref 是那個多對一的連結。
//
// 對**同一個** set 呼叫兩次仍會撞 procurement_agent_runs 的 primary key（同一個
// run_id），回傳的 unique_violation 由呼叫端當一般錯誤處理（記 log、不阻斷候選
// 搜尋本身——ledger 是投影不是 SSOT）。
//
// CreateProcurementAgentRun 與首筆 RUN_ACCEPTED event 包在同一個交易內，同
// ClaimRun 的既有寫法——避免出現「run row 已建立但沒有對應的首筆事件」這種
// 中途失敗留下的不一致狀態。
func (r *Repo) CreateLedgerRunForCandidateSearch(ctx context.Context, set CandidateSet) error {
	runID := set.PublicRunID()
	return r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		// conversationID 解析（Task 15 write-path 修復）：候選搜尋 run 概念上
		// 跟 TURN／ANSWER／REWIND 一樣屬於某個 conversation——set.RequestID 恆非零
		// （StartSearch 只在既有 request 上啟動搜尋，見該函式），這裡跟 ClaimRun
		// 用同一支 conversationIDForRequest 解析，維持「runs[] 能看到這個
		// conversation 底下每一種 run type」的一致性（doc57 §5.6）。
		conversationID, err := r.conversationIDForRequest(txCtx, set.RequestID)
		if err != nil {
			return err
		}

		if _, err := q.CreateProcurementAgentRun(txCtx, gen.CreateProcurementAgentRunParams{
			ID: runID, BuyerID: int64(set.BuyerID),
			ConversationID: int64OrNil(conversationID),
			RunType:        string(domain.RunTypeCandidateSearch),
			DomainRefType:  "candidateSetId", DomainRefID: strconv.FormatInt(set.ID, 10),
			Status: domain.RunStatusQueued(), LastEventSeq: 1,
			ControllerKind: string(domain.ControllerKindDurable),
		}); err != nil {
			return err
		}
		_, err = q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: runID, Seq: 1, EventType: codemeta.ProcurementLedgerEventTypeRunAccepted.Value,
			SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		})
		return err
	})
}

func toRunEntity(row gen.ProcurementAgentRun) Run {
	return Run{
		ID: row.ID, BuyerID: identity.BuyerID(row.BuyerID),
		ConversationID: int64Value(row.ConversationID), ParentRunID: int64Value(row.ParentRunID),
		RunType: domain.RunType(row.RunType), DomainRefType: row.DomainRefType, DomainRefID: row.DomainRefID,
		Status: row.Status, FailureReason: row.FailureReason,
		ResumedIntoRunID: int64Value(row.ResumedIntoRunID), Version: row.Version,
		LastEventSeq: row.LastEventSeq, MinAvailableSeq: row.MinAvailableSeq,
		ResultRevisionID: int64Value(row.ResultRevisionID),
		ControllerKind:   domain.ControllerKind(row.ControllerKind),
	}
}

// toRunEventEntity 對映 sqlc 生成的 gen.ProcurementAgentRunEvent 到本檔案的
// RunEvent。OccurredAt 在 gen 層是 pgtype.Timestamptz（DB 欄位 NOT NULL），
// 對外只暴露 time.Time——同 models.go 裡 Revision.CreatedAt／toRevisionEntity
// 用 `row.X.Time` 直接取值，不額外包一層 conversion wrapper（行為相同，這裡跟隨
// 檔案既有慣例）。
func toRunEventEntity(row gen.ProcurementAgentRunEvent) RunEvent {
	return RunEvent{
		ID: row.ID, RunID: row.RunID, Seq: row.Seq, EventType: row.EventType,
		SchemaVersion: row.SchemaVersion, PublicPayloadJSON: row.PublicPayloadJson,
		OccurredAt: row.OccurredAt.Time,
	}
}

type AppendRunEventInput struct {
	RunID             int64
	BuyerID           identity.BuyerID
	EventType         string
	SchemaVersion     string
	PublicPayloadJSON []byte
	TraceID           string
}

// AppendRunEvent 是 doc57 §6.1 seq 分配不變式的落地：FOR UPDATE 鎖 run row，
// 讀 last_event_seq 算下一個 seq，寫 event，最後在同一交易內把 run row 的
// last_event_seq bump 上去——三件事同一個 transaction，理由同 AppendRevision
// 對 procurement_requests 做的事。
func (r *Repo) AppendRunEvent(ctx context.Context, in AppendRunEventInput) (RunEvent, error) {
	var out RunEvent
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		runRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
			ID: in.RunID, BuyerID: int64(in.BuyerID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		nextSeq := runRow.LastEventSeq + 1
		eventRow, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: in.RunID, Seq: nextSeq, EventType: in.EventType,
			SchemaVersion: in.SchemaVersion, PublicPayloadJson: in.PublicPayloadJSON,
			OccurredAt: timestamptzParam(nowUTC()), TraceID: cmp.Or(in.TraceID, ledgerTraceID(txCtx)),
		})
		if err != nil {
			return err
		}

		if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
			ID: in.RunID, LastEventSeq: nextSeq,
		}); err != nil {
			return err
		}

		out = toRunEventEntity(eventRow)
		return nil
	})
	if err != nil {
		return RunEvent{}, err
	}

	// NOTIFY 在 commit 之後送（doc57 事件不變式 3）——WithTx 這裡已經 commit
	// 完成才會執行到這行。實際 LISTEN/NOTIFY 機制見 Task 5（run_watch.go）；
	// 這裡先只呼叫一個之後會補上的 hook，讓本 task 的介面先穩定。
	r.notifyRunWake(in.RunID)

	return out, nil
}

type TerminalizeRunInput struct {
	RunID   int64
	BuyerID identity.BuyerID
	// Version **只有非終態的 park（Status = WAITING_FOR_INPUT）會用到**——那條
	// 分支維持 version CAS，因為 run 還活著、併發的 cancel 必須贏過這次 park。
	// 真正的終態轉移忽略這個欄位，守衛是狀態轉移合法性（2026-09-07 決策）。
	Version           int64
	Status            string // SUCCEEDED | FAILED | CANCELLED | RESUMED | EXPIRED | WAITING_FOR_INPUT
	FailureReason     string
	ResultRevisionID  int64
	TerminalEventType string
	PublicPayloadJSON []byte
}

// TerminalizeRun 是 doc57 §5.1「Terminal transaction」：terminal status 與
// terminal event 寫在同一交易。
//
// **守衛依 status 類別分流**（見函式內註解）：真正的終態只比對「現在還不是終態」，
// 不比對 version；停進 WAITING_FOR_INPUT 這條非終態的 park 才維持 version CAS。
//
// 回傳 (wrote bool, error)：wrote=false 代表這次轉移被守衛擋下——終態分支是「這個
// run 已經是終態了」，park 分支是「version 已不符」。兩種情況呼叫端都不得把結果當
// 成「成功終結」，但成因不同：前者代表別人已經合法終結過它，後者代表呼叫端手上是
// 一份過期快照。
func (r *Repo) TerminalizeRun(ctx context.Context, in TerminalizeRunInput) (bool, error) {
	wrote := false
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		wrote, err = r.terminalizeRunTx(txCtx, in)
		return err
	})
	if err != nil {
		return false, err
	}
	if wrote {
		r.notifyRunWake(in.RunID)
	}
	return wrote, nil
}

// terminalizeRunTx 是 TerminalizeRun 的交易主體，抽出來給「已經身在一個交易裡」的呼叫端
// 直接用（目前是 reapCandidateSet——lazy reaper 也是一條終態寫入路徑，它的 candidate_set
// 終態與這裡的 ledger 終態必須一起 commit 或一起 rollback）。
//
// **不呼叫 notifyRunWake**：WithTx 是可重入的（巢狀呼叫沿用外層交易），所以從交易內發
// NOTIFY 會早於 commit——訂閱端被叫醒、去 DB 查、什麼都沒看到，然後回去等下一個訊號。
// 喚醒必須由開啟交易的那一層在 commit 成功之後才發，兩個呼叫端各自負責（同 reap 那邊
// metric 計數點刻意放在交易外的理由）。
func (r *Repo) terminalizeRunTx(txCtx context.Context, in TerminalizeRunInput) (bool, error) {
	q := r.q(txCtx)

	// 守衛依 status 類別分流（2026-09-07 決策）。真正的終態走專用 query：
	// 只比對「現在還不是終態」，**不比對 version**——worker 宣告自己的執行
	// 結果不是 read-modify-write，用呼叫端手上那份可能已被 cancel／retry
	// bump 過的 version 做 CAS，會讓終態整個寫不進去且無聲（wrote=false 不
	// 是錯誤），該 run 於是永遠沒有終態 ledger event。
	//
	// 停進 WAITING_FOR_INPUT 這條**不是**終態：run 還活著，一個併發的 cancel
	// 是真實的競爭意圖，必須贏過這次 park，所以維持 version CAS。兩者不能共用
	// 同一個守衛——見 TestTerminalizeRun_ParkIntoWaiting_MustNotEraseCancel。
	var affected int64
	var err error
	if domain.IsTerminalRunStatus(in.Status) {
		affected, err = q.TerminalizeProcurementAgentRunStatus(txCtx, gen.TerminalizeProcurementAgentRunStatusParams{
			ID: in.RunID, Status: in.Status,
			FailureReason: in.FailureReason, ResultRevisionID: int64OrNil(in.ResultRevisionID),
		})
	} else {
		affected, err = q.UpdateProcurementAgentRunStatus(txCtx, gen.UpdateProcurementAgentRunStatusParams{
			ID: in.RunID, Version: in.Version, Status: in.Status,
			FailureReason: in.FailureReason, ResultRevisionID: int64OrNil(in.ResultRevisionID),
		})
	}
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil // 非錯誤——見函式註解
	}

	runRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
		ID: in.RunID, BuyerID: int64(in.BuyerID),
	})
	if err != nil {
		return false, err
	}
	nextSeq := runRow.LastEventSeq + 1
	if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
		ID: r.newID(), RunID: in.RunID, Seq: nextSeq, EventType: in.TerminalEventType,
		SchemaVersion: "v1", PublicPayloadJson: in.PublicPayloadJSON, OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
	}); err != nil {
		return false, err
	}
	if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
		ID: in.RunID, LastEventSeq: nextSeq,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// notifyRunWake 的真正實作（Postgres LISTEN/NOTIFY）見 run_watch.go：
// AppendRunEvent／TerminalizeRun／CancelRun 的呼叫點在 Task 3 就已經穩定
// 接上，Task 5 落地時不需要再回頭改本檔案的交易邏輯或呼叫順序。

// CancelDisposition 值域固定四值（doc57 §5.4，不得新增）——這四值都代表
// 「run 已經到達某個不可變狀態」；CAS 沒過但 run 仍非終態的情境（見
// ErrCancelVersionConflict）不屬於這個值域，用具名 error 表達，不硬套其中
// 一個。
const (
	CancelDispositionRequested        = "REQUESTED"
	CancelDispositionAlreadyCancelled = "ALREADY_CANCELLED"
	CancelDispositionAlreadyTerminal  = "ALREADY_TERMINAL"
	CancelDispositionTooLate          = "TOO_LATE"
)

// ErrCancelVersionConflict 代表 CancelRun 的 CAS（比對 in.ExpectedVersion）
// 沒過，但重新分類過的 run 真實狀態既不是終態、也不是 CANCEL_REQUESTED——run
// 仍在飛，呼叫端手上的只是一份過期快照（例如 run 在呼叫端上次讀取之後又轉了
// 一次 status，像 QUEUED -> RUNNING；version 因此往前推進了但狀態沒變終態）。
// 這不是 doc57 §5.4 四值 disposition 的任何一個：REQUESTED 會謊報這次呼叫
// 真的送出了取消，TOO_LATE／ALREADY_TERMINAL／ALREADY_CANCELLED 則都宣稱 run
// 已經到達某個不可變狀態——對一個還在跑的 run 講其中任何一句都是誤導（違背
// CancelRun 函式文件與 doc57 §5.4「不得謊報」的保證）。用具名 error（同
// ErrIdempotencyConflict／ErrNotFound 的既有模式）把「CAS precondition 沒
// 過」和「這次呼叫的四個合法 disposition 結果之一」明確分開，呼叫端收到後
// 該重新 GetRun 拿最新 version 再決定要不要重試，而不是把它當某個終態回應
// 轉譯出去。
var ErrCancelVersionConflict = errors.New("procurement: cancel version conflict")

// ErrCancelInterruptPending 是 open-interrupt guard 的 CancelRun 分支——跟
// usecase/run_dispatch.go 的 GuardNoOpenInterrupt（/answers、
// /crop-disambiguation 兩條路徑用的版本）是同一個 Task 14b Blocking 3 裁決，
// 只是換到 repository 這一層自己表達，不是呼叫 GuardNoOpenInterrupt 本身：
// usecase 依賴 repository，反過來讓 repository 呼叫 usecase 的函式會是一個
// 層次違規（import cycle 的方向錯了）。
//
// 不變式：一個 run 處於 WAITING_FOR_INPUT，代表它身上掛著一個尚未回覆的
// interrupt（見 usecase/run_dispatch.go interruptOutcomeFor 的文件——目前
// 唯一會把 run 轉成這個狀態的路徑就是建立 interrupt）。classifyCancelDisposition
// 刻意不把 WAITING_FOR_INPUT 當終態分類，所以放著不管的話 CancelRun 會直接把
// 它轉成 CANCEL_REQUESTED（doc57 §5.4 承認的合法可達路徑），但沒有任何後續
// 動作會去關閉那個 interrupt row——run 本身的狀態機沒有錯，是那個 interrupt
// 永遠孤兒化、買家再也沒有一條路徑可以回覆它。CancelRun 因此在觸到
// classifyCancelDisposition／任何狀態更新之前就要用這個 sentinel 擋下，逼
// 買家先走 resume。
var ErrCancelInterruptPending = errors.New("procurement: cannot cancel a run that is parked on an open interrupt; resume it first")

// classifyCancelDisposition 把一個 run status 對應到 doc57 §5.4 四值
// disposition 中「run 已經到達某個不可變狀態，這次 CancelRun 呼叫沒有真的
// mutate 任何東西」的那三個（TOO_LATE／ALREADY_TERMINAL／ALREADY_CANCELLED）。
// CancelRun 有兩個呼叫點都需要這個分類：函式一開始用 FOR UPDATE 鎖讀到的
// runRow，以及 CAS 沒過之後重讀到的 latest——兩處必須用同一套規則，CAS-miss
// 分支才不會在沒有重新檢查真實狀態的情況下就誤報（見 ErrCancelVersionConflict
// 文件與這批 review 抓到的 bug：舊版程式碼曾經對 CAS-miss 無條件回
// ALREADY_TERMINAL，即使重讀出來的 latest.Status 根本不是終態）。ok=false
// 代表這個 status 不屬於這三種，呼叫端必須自行決定接下來怎麼處理。
func classifyCancelDisposition(status string) (disposition string, ok bool) {
	if domain.IsTerminalRunStatus(status) {
		if status == domain.RunStatusSucceeded() {
			return CancelDispositionTooLate, true
		}
		return CancelDispositionAlreadyTerminal, true
	}
	if status == domain.RunStatusCancelReq() {
		return CancelDispositionAlreadyCancelled, true
	}
	return "", false
}

type CancelRunInput struct {
	RunID           int64
	BuyerID         identity.BuyerID
	ExpectedVersion int64
}

// CancelRun 落地 doc57 §5.4：terminal run 一律 disposition=TOO_LATE 且
// runStatus 維持原終態（不得謊報取消成功）；非終態才真的轉 CANCEL_REQUESTED。
//
// CAS（ExpectedVersion）沒過時：這一列從函式一開始就被 FOR UPDATE 鎖住到交易
// 結束，鎖定期間不可能有其他 transaction 插隊改它，所以 CAS 沒過只可能是
// in.ExpectedVersion 本身對到的是呼叫端一份過期快照。重讀出來的 latest 狀態
// 一樣要用 classifyCancelDisposition 重新分類——真的落在終態／
// CANCEL_REQUESTED 就回對應 disposition，否則回 ErrCancelVersionConflict，
// 不得直接假設「CAS 沒過＝已經終結」（見 ErrCancelVersionConflict 文件）。
func (r *Repo) CancelRun(ctx context.Context, in CancelRunInput) (Run, string, error) {
	var out Run
	disposition := ""

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)
		runRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
			ID: in.RunID, BuyerID: int64(in.BuyerID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		out = toRunEntity(runRow)

		// ErrCancelInterruptPending guard（Task 14b Blocking 3 的 CancelRun
		// 分支，見該 sentinel 的文件）：這裡是唯一 RACE-FREE 的檢查點——runRow
		// 從上面 GetProcurementAgentRunForUpdate 那一刻起就被 FOR UPDATE 鎖到
		// 整個交易結束，不像 GuardNoOpenInterrupt 對 /answers、
		// /crop-disambiguation 那樣得另外做一次無鎖的
		// FindOpenInterruptForRequest 查詢、也就不會繼承它文件裡承認的那個
		// check-then-act 殘留窗口：這個 run 的 status 在我們讀到它的這一刻就
		// 已經是最終答案，沒有其他交易能在這個鎖釋放前把它改成別的值。必須
		// 排在 classifyCancelDisposition 與任何狀態更新之前，才能保證這個
		// disposition 是零 mutation 的。
		if runRow.Status == domain.RunStatusWaiting() {
			return ErrCancelInterruptPending
		}

		if disp, ok := classifyCancelDisposition(runRow.Status); ok {
			disposition = disp
			return nil
		}

		affected, err := q.UpdateProcurementAgentRunStatus(txCtx, gen.UpdateProcurementAgentRunStatusParams{
			ID: in.RunID, Version: in.ExpectedVersion, Status: domain.RunStatusCancelReq(),
			// nil（不是 int64OrNil 的字面值 0）：result_revision_id 現在用 COALESCE
			// 保留原值（Task 14b C1）。CancelRun 對一個已 park（WAITING_FOR_INPUT
			// → CANCEL_REQUESTED）的 run 是可達路徑（classifyCancelDisposition
			// 不把 WAITING 當終態），若沿用舊的字面值 0 會把 parent 的
			// replayExistingRun 錨點抹掉。
			FailureReason: "", ResultRevisionID: nil,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			latest, err := q.GetProcurementAgentRun(txCtx, gen.GetProcurementAgentRunParams{ID: in.RunID, BuyerID: int64(in.BuyerID)})
			if err != nil {
				return err
			}
			out = toRunEntity(latest)
			// latest.Status 不可能是 WAITING_FOR_INPUT，不需要在這裡重複上面
			// 那個 ErrCancelInterruptPending 檢查：runRow 從函式一開始就被
			// FOR UPDATE 鎖住到現在，鎖定期間沒有任何其他交易能寫這一列；
			// affected==0 又代表我們自己剛才那次
			// UpdateProcurementAgentRunStatus 也沒有真的改動它（CAS 沒過）。
			// 這一列在本交易全程唯一可能的寫入者只有我們自己，而我們沒有寫
			// 成功，所以 latest.Status 必然逐位元組等於 runRow.Status——而
			// runRow.Status 已經在上面被判斷過不是 WAITING_FOR_INPUT（否則
			// 函式在那裡就已經回傳 ErrCancelInterruptPending，根本不會執行到
			// 這裡）。
			if disp, ok := classifyCancelDisposition(latest.Status); ok {
				disposition = disp
				return nil
			}
			// latest.Status 重新分類後確認仍非終態、非 CANCEL_REQUESTED：run
			// 還在飛，CAS 沒過純粹是呼叫端的 ExpectedVersion 過期——見
			// ErrCancelVersionConflict 文件，不得套用任何一個 disposition 值。
			return ErrCancelVersionConflict
		}

		nextSeq := runRow.LastEventSeq + 1
		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: in.RunID, Seq: nextSeq, EventType: codemeta.ProcurementLedgerEventTypeRunCancelRequested.Value,
			SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}
		if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
			ID: in.RunID, LastEventSeq: nextSeq,
		}); err != nil {
			return err
		}
		out.Status = domain.RunStatusCancelReq()
		out.Version = in.ExpectedVersion + 1
		disposition = CancelDispositionRequested
		return nil
	})
	if err != nil {
		return Run{}, "", err
	}
	if disposition == CancelDispositionRequested {
		r.notifyRunWake(in.RunID)
	}
	return out, disposition, nil
}

type Interrupt struct {
	ID                    string
	RunID                 int64
	InterruptKind         string
	ResponseSchemaVersion string
	Status                string
	ResolvedByChildRunID  int64
	// PayloadJSON 是這個中斷問買家的內容，形狀由 (InterruptKind,
	// ResponseSchemaVersion) 決定——CLARIFICATION/v1 是
	// usecase.CropDisambiguationInterruptPayload。repository 層刻意不解讀它：
	// 這一層不知道 crop 消歧是什麼，跟 procurement_agent_run_events.payload_json
	// 的既有處理方式相同。
	PayloadJSON json.RawMessage
	// FactsAsOf 是組出 PayloadJSON 時讀取分類樹的時刻，零值表示
	// 這個 kind 沒有外部事實，或是搬遷前建立的舊列。
	FactsAsOf time.Time
}

type CreateInterruptInput struct {
	RunID                 int64
	InterruptKind         string
	ResponseSchemaVersion string
	PayloadJSON           json.RawMessage
	FactsAsOf             time.Time
}

// CreateInterrupt 供 terminal transaction 在 run 進入 WAITING_FOR_INPUT 時呼叫
// （v1 呼叫端見 Task 7／8：crop 消歧待選，spec §10.1 2026-08-27 更新）。ID 由
// `r.newID()` 內部產生並字串化（`procurement_agent_run_interrupts.id` 是
// TEXT PRIMARY KEY，不是其餘表常見的 BIGINT——同 run/event 的既有 TSID 產生
// 慣例，只是欄位型別不同，呼叫端不需要也不應該自己組 ID）。
func (r *Repo) CreateInterrupt(ctx context.Context, in CreateInterruptInput) (Interrupt, error) {
	payload := in.PayloadJSON
	if len(payload) == 0 {
		// NOT NULL 欄位：沒有 payload 的 kind 存空物件，不存 SQL NULL——讀取端
		// 因此永遠 unmarshal 得動，不需要每一處都先判 nil。
		payload = json.RawMessage(`{}`)
	}
	row, err := r.q(ctx).CreateProcurementAgentRunInterrupt(ctx, gen.CreateProcurementAgentRunInterruptParams{
		ID: strconv.FormatInt(r.newID(), 10), RunID: in.RunID, InterruptKind: in.InterruptKind,
		PayloadJson: payload, FactsAsOf: nullableTime(in.FactsAsOf),
		ResponseSchemaVersion: in.ResponseSchemaVersion, Status: "PENDING",
	})
	if err != nil {
		return Interrupt{}, err
	}
	return toInterruptEntity(row), nil
}

// ErrInterruptAlreadyResolved 同 ErrNotFound／ErrIdempotencyConflict 的既有
// 具名 error 慣例（見 revision.go 檔頭）——具名才能在 Task 14 用 errors.Is
// 正確比對；errors.New(...) 內聯建立的 error 每次呼叫都是不同的 pointer，
// errors.Is 永遠比對不到。
var ErrInterruptAlreadyResolved = errors.New("procurement: interrupt already resolved")

// ErrRunNotResumable 表示 interrupt 本身是 PENDING、未被解決，但它所屬的
// parent run 不處於可 resume 的狀態（不是 WAITING_FOR_INPUT，或
// result_revision_id 是 NULL——理論上不會出現：CreateInterrupt 只在
// terminalize 成 WAITING_FOR_INPUT 那筆交易內被呼叫，那筆交易一定也寫了
// result_revision_id，見 executeAnswerPipeline／executeTurnPipeline 的
// terminalStatus／ResultRevisionID 賦值順序；這裡仍保留檢查，同
// ErrCancelVersionConflict 文件記載的既有防禦深度慣例）。對映
// Task 14 的 RUN_NOT_ACTIVE（409）。
var ErrRunNotResumable = errors.New("procurement: parent run is not resumable")

// ErrResumeVersionConflict 是步驟 13（UpdateProcurementAgentRunStatus）CAS
// 沒過時的具名 error——在步驟 1／6 的 FOR UPDATE 鎖住 interrupt／parent 兩列
// 直到交易結束的前提下，這個分支在今天的呼叫路徑上不可達（沒有其他交易能在
// 鎖定期間插隊改掉 parentRow.Version）。保留這個檢查是防禦性斷言，不是
// 併發防線——不得宣稱它修掉了「今天會產生不一致結果的狀態」，見 Task 14b
// 設計文件 §5.2 m10 的說明。
var ErrResumeVersionConflict = errors.New("procurement: resume version conflict")

// ResumeResolveContext 是 ResumeRunInput.ResolveInput 收到的唯讀輸入——kind／
// version 來自這次 FOR UPDATE 鎖住的 interrupt row，RequestID／
// ExpectedRevisionID 由 parent.result_revision_id 反查得出（Task 14b M5）。
type ResumeResolveContext struct {
	InterruptKind         string
	ResponseSchemaVersion string
	RequestID             int64 // 由 parent.result_revision_id 反查
	ExpectedRevisionID    int64 // = parent.result_revision_id
	ResponsePayload       []byte
	// InterruptPayload 是**伺服器當初問的那一題**（interrupt row 的
	// payload_json），對比 ResponsePayload 是買家送回來的答案。ResolveInput 用
	// 它驗證答案落在當初給的選項白名單內。
	//
	// 兩者必須都在這裡：白名單如果另外去查一個可被並行寫入覆寫的地方（刀 2 之前
	// 是 procurement_requests 上的欄位），驗的就不一定是買家眼前那張卡。這一行
	// 讀的是同一筆交易裡 FOR UPDATE 鎖住的那一列。
	InterruptPayload []byte
}

// ResolvedRunInput 是 ResolveInput 的輸出：伺服器推導完成、準備寫進
// procurement_agent_run_inputs.input_ref 的內容（Task 14b B2——child run 因此
// 是 self-describing 的，不只活在這次 HTTP 請求的 goroutine 裡）。
type ResolvedRunInput struct {
	InputRefJSON       []byte
	InputSchemaVersion string
	RetentionClass     string
}

type ResumeRunInput struct {
	InterruptID     string
	BuyerID         identity.BuyerID
	ClientRequestID string
	PayloadHash     string
	ResponsePayload []byte

	// ResolveInput 在 T0 交易內、**任何 mutation 之前**被呼叫（步驟 8），把
	// interrupt kind／schema version／伺服器推導出的 request 座標交給
	// usecase 層做 kind routing 與 payload 語意解碼。回傳的 InputRefJSON
	// 直接寫進 procurement_agent_run_inputs.input_ref。
	//
	// 為什麼是 callback 而不是「先在交易外驗證再進來」：kind／version 雖然
	// immutable，但 RequestID／ExpectedRevisionID 是從**這次 FOR UPDATE
	// 鎖住的**parent row 推導的；在交易外先算一次，交易內就必須再驗一次它
	// 沒變，等於同一份推導寫兩遍。callback 讓推導與落庫落在同一把鎖內，一次
	// 做對。
	//
	// 為什麼不是「repository 自己解 payload」：kind → AnswerInput 的對映是
	// usecase 層的業務知識，repository import usecase 會成環。
	//
	// 契約：這個 callback 只能做 DB 以外零 I/O 的事（純 JSON 解碼＋常數
	// 比對）——不得呼叫分類樹／LLM，那會把外部延遲綁進一個持有兩把
	// row lock 的交易。
	//
	// !IsNew（冪等命中）路徑**不呼叫**這個 callback：那次命令的 resolved
	// input 早就落庫了，重算一份沒有寫入處，也沒有意義。
	ResolveInput func(ctx context.Context, rc ResumeResolveContext) (ResolvedRunInput, error)
}

// ResumeRunOutput 是 ResumeRun 的回傳值。Resolved 只在 IsNew 時有值——呼叫端
// 用它組 AnswerInput 去 dispatch；!IsNew 時零值：C2 裁決之下那條路徑不再
// dispatch，不需要它。
type ResumeRunOutput struct {
	Child    Run
	IsNew    bool
	Resolved ResolvedRunInput
}

// ResumeRun 落地 doc57 §5.5 + Task 14b 的 root-cause 修正：
//
//   - 鎖序反轉（C3）：先鎖 interrupt（步驟 1），再查冪等鍵（步驟 2）——READ
//     COMMITTED 下每個 statement 取新 snapshot，後到者解鎖後看得到贏家已
//     commit 的 command row，同鍵同 payload 的併發重試因此是冪等的（不會撞
//     ErrInterruptAlreadyResolved）。
//   - kind／schema 驗證與 payload 解碼透過 ResolveInput，在任何 mutation
//     之前於交易內執行（M5）：失敗 = ROLLBACK、零 mutation，不會像 v1 草案
//     那樣先燒毀 parent 才發現 kind 不支援。
//   - result_revision_id 在步驟 13 傳 NULL（C1）：COALESCE 保留 parent 原本
//     的值，resume 不再把它抹成字面值 0。
//   - 同一 interrupt 只能有一個 child：由步驟 1 的 `FOR UPDATE` 序列化＋
//     步驟 3/4 的 guard＋`ResolveProcurementAgentRunInterrupt` 的
//     `WHERE resolved_by_child_run_id IS NULL`（runs.sql）三者共同保證。
//     **沒有**部分唯一索引，也不需要——反方向的「一個 child 解兩個
//     interrupt」在 childID 由交易內 r.newID() 產生的前提下結構上不可能。
func (r *Repo) ResumeRun(ctx context.Context, in ResumeRunInput) (ResumeRunOutput, error) {
	var out ResumeRunOutput
	var parentID, childID int64

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		// 步驟 1（C3 鎖序反轉）：先鎖 interrupt，這是併發 resume 的序列化點。
		interruptRow, err := q.GetProcurementAgentRunInterruptForUpdate(txCtx, in.InterruptID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		// 步驟 2：冪等鍵查詢排在 interrupt 鎖**之後**。命中時比對 hash，並
		// 額外斷言這把鍵當初真的是用在這一個 interrupt 上（M4 第二層防
		// 線——hash 本身已含 InterruptID，這裡是防禦深度，理論上不會不符）。
		existingCmd, err := q.FindProcurementAgentRunCommandByIdempotencyKey(txCtx, gen.FindProcurementAgentRunCommandByIdempotencyKeyParams{
			BuyerID: int64(in.BuyerID), ClientRequestID: in.ClientRequestID,
		})
		if err == nil {
			if existingCmd.PayloadHash != in.PayloadHash {
				return ErrIdempotencyConflict
			}
			row, err := q.GetProcurementAgentRun(txCtx, gen.GetProcurementAgentRunParams{ID: existingCmd.RunID, BuyerID: int64(in.BuyerID)})
			if err != nil {
				return err
			}
			// M4 第二層防線：這把鍵當初建立的 child，必須真的是這個 interrupt 所屬
			// parent 的 child（hash 本身已含 InterruptID，這裡是防禦深度）。
			//
			// ⛔ 判準只能用不可變欄位。原本比的是 interruptRow.ResolvedByChildRunID，
			// 那個指標是可變的：resume 失敗後 RestoreInterruptAfterFailedResume 會把它
			// 清成 NULL，此後同鍵同內容的重送會被誤判成 IDEMPOTENCY_KEY_CONFLICT，
			// 違反 doc 57 §5.2／§11「同鍵同內容是成功的 replay 路徑，只有同鍵不同
			// payload 才是 conflict」。
			if existingCmd.CommandKind != "RESUME" || int64Value(row.ParentRunID) != interruptRow.RunID {
				return ErrIdempotencyConflict
			}
			out = ResumeRunOutput{Child: toRunEntity(row), IsNew: false}
			return nil // !IsNew：不呼叫 ResolveInput，零額外 mutation。
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// 步驟 3/4：interrupt 必須真的還沒被解決。兩個 guard 對映同一個
		// ErrInterruptAlreadyResolved（Task 14 依此映射 APPROVAL_ALREADY_RESOLVED）。
		if interruptRow.ResolvedByChildRunID != nil {
			return ErrInterruptAlreadyResolved
		}
		if interruptRow.Status != "PENDING" {
			return ErrInterruptAlreadyResolved
		}
		// —— 步驟 5（expiry guard）已刪除：v1 interrupt 不過期（M6）。
		// procurement_agent_run_interrupts.expires_at 永遠是 NULL——
		// CreateInterrupt 不填它，全 procurement package 零處寫入。TTL
		// 政策延到 Track 2（gated on #2452）一起做，這裡不寫一個檢查永遠
		// 為 NULL 欄位的 guard 來紙面滿足 spec。

		// 步驟 6：鎖 parent run。
		parentRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
			ID: interruptRow.RunID, BuyerID: int64(in.BuyerID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		// 步驟 7/8：parent 必須真的處於 WAITING_FOR_INPUT 且帶著一個真實的
		// result_revision_id（resume 的 ExpectedRevisionID 樂觀鎖錨點就是
		// 它）。兩者理論上總是同時成立（見 ErrRunNotResumable 文件），這裡
		// 仍保留檢查。
		if parentRow.Status != domain.RunStatusWaiting() {
			return ErrRunNotResumable
		}
		if parentRow.ResultRevisionID == nil || *parentRow.ResultRevisionID == 0 {
			return ErrRunNotResumable
		}

		revisionRow, err := q.GetProcurementRevisionByID(txCtx, *parentRow.ResultRevisionID)
		if err != nil {
			return err
		}

		// 步驟 8（M5）：kind／schema guard 與 payload 語意解碼在任何
		// mutation 之前於交易內完成——失敗在這裡 return 直接 ROLLBACK，零
		// mutation。這一行是 T0 的「不可逆點」：它之前一個 mutation 都沒有。
		resolved, err := in.ResolveInput(txCtx, ResumeResolveContext{
			InterruptKind: interruptRow.InterruptKind, ResponseSchemaVersion: interruptRow.ResponseSchemaVersion,
			RequestID: revisionRow.RequestID, ExpectedRevisionID: *parentRow.ResultRevisionID,
			ResponsePayload: in.ResponsePayload, InterruptPayload: interruptRow.PayloadJson,
		})
		if err != nil {
			return err
		}

		// 步驟 9：建立 child run。ControllerKind 直接繼承 parentRow（同一個
		// non-legacy path 的延續——resume 出來的 child 不該被重新分類成別的
		// controller kind）。procurement_agent_runs.controller_kind 是
		// NOT NULL + CHECK IN ('DURABLE','GRAPH')（Task 1，spec §4.1）：這裡若
		// 沿用 Task 1 原始草稿遺漏的寫法（不設這個欄位），INSERT 會用 Go
		// zero value ""，直接違反 CHECK constraint——這不是理論推演，是
		// TestResumeRun_SameInterruptTwice_OnlyOneChild 等 6 個既有整合測試
		// 實測撞到的 SQLSTATE 23514，見本次 commit 的 TDD 證據。parentRow 是
		// gen.ProcurementAgentRun（上面 GetProcurementAgentRunForUpdate 讀出
		// 來的列），改過的 sqlc 生成型別已經帶 ControllerKind 欄位，直接讀出
		// 即可，不需要另外查一次。
		childID = r.newID()
		childRow, err := q.CreateProcurementAgentRun(txCtx, gen.CreateProcurementAgentRunParams{
			ID: childID, BuyerID: int64(in.BuyerID),
			ConversationID: parentRow.ConversationID, ParentRunID: pointerInt64(parentRow.ID),
			RunType: string(domain.RunTypeClarificationResume), Status: domain.RunStatusQueued(), LastEventSeq: 1,
			ControllerKind: parentRow.ControllerKind,
		})
		if err != nil {
			return err
		}

		// ⛔ 步驟 10（B2）原本把 resolved input 落進 procurement_agent_run_inputs，
		// 2026-09-20 停寫。那張表自建立以來**零讀取端**（全 repo 沒有任何
		// SELECT，也沒有對應的 sqlc read query，275 列全是寫進去就沒再被碰
		// 過的），它存在的理由是「child run self-describing、讓重驅的 sweeper
		// 拿得回輸入」——而 sweeper 屬 Track 2，已由 ADR
		// 2026-09-20-durability-primitive-stays-pkg-outbox 判定不做。留著一份
		// 沒有人讀的副本，只會讓讀碼的人以為重驅機制存在。
		//
		// ResolveInput 本身保留：它在 T0 交易內做 kind routing 與 payload 語意
		// 解碼（見 ResumeRunInput.ResolveInput 的文件），是這條路徑的驗證閘門，
		// 與落庫無關。表與 CreateProcurementAgentRunInput query 留給 contract
		// 波一併移除。

		// 步驟 11：child 的首筆 RUN_ACCEPTED event。
		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: childID, Seq: 1, EventType: codemeta.ProcurementLedgerEventTypeRunAccepted.Value,
			SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}

		// 步驟 12：parent 的 RUN_RESUMED event。
		parentNextSeq := parentRow.LastEventSeq + 1
		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: parentRow.ID, Seq: parentNextSeq, EventType: codemeta.ProcurementLedgerEventTypeRunResumed.Value,
			SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}
		if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
			ID: parentRow.ID, LastEventSeq: parentNextSeq,
		}); err != nil {
			return err
		}

		// 步驟 13（C1）：parent 轉 RESUMED，result_revision_id 傳 NULL——
		// COALESCE 保留原值，不再抹成字面值 0。affected==1 是防禦性斷言
		// （步驟 1／6 的 FOR UPDATE 之下這個分支不可達，不是併發防線，見
		// ErrResumeVersionConflict 文件）。
		affected, err := q.UpdateProcurementAgentRunStatus(txCtx, gen.UpdateProcurementAgentRunStatusParams{
			ID: parentRow.ID, Version: parentRow.Version, Status: domain.RunStatusResumed(),
			FailureReason: "", ResultRevisionID: nil,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrResumeVersionConflict
		}

		// 步驟 14。
		if err := q.SetProcurementAgentRunResumedInto(txCtx, gen.SetProcurementAgentRunResumedIntoParams{
			ID: parentRow.ID, ResumedIntoRunID: pointerInt64(childID),
		}); err != nil {
			return err
		}

		// 步驟 15：resolve interrupt。execrows==1 同樣是防禦性斷言（理由同
		// 步驟 13）。
		resolvedRows, err := q.ResolveProcurementAgentRunInterrupt(txCtx, gen.ResolveProcurementAgentRunInterruptParams{
			ID: in.InterruptID, Status: "APPROVED", ResolvedByChildRunID: pointerInt64(childID),
		})
		if err != nil {
			return err
		}
		if resolvedRows != 1 {
			return ErrInterruptAlreadyResolved
		}

		// 步驟 16：落庫這次 RESUME 命令的冪等鍵。
		//
		// ⛔ 必須讀 RETURNING 的 id。query 自帶 ON CONFLICT DO UPDATE（撞鍵不拋錯、
		// 保證回一列），呼叫端要靠「回傳的 id 是不是這次產的」區分認領與既有列——
		// runs.sql 的註解就是這樣寫的，ClaimRun 也是這樣用的。丟掉它，兩個**不同**
		// interrupt 的併發 resume（步驟 1 的 FOR UPDATE 只序列化同一個 interrupt）
		// 會各自 commit 一個 child run、事件與 revision，而命令表只留一列。
		cmdID := r.newID()
		cmdRow, err := q.CreateProcurementAgentRunCommand(txCtx, gen.CreateProcurementAgentRunCommandParams{
			ID: cmdID, BuyerID: int64(in.BuyerID), CommandKind: "RESUME",
			ClientRequestID: in.ClientRequestID, PayloadHash: in.PayloadHash,
			RunID: childID, Status: "ACCEPTED",
		})
		if err != nil {
			return err
		}
		if cmdRow.ID != cmdID {
			// 敗方：整個 transaction 回滾，child run／事件／revision 一起退掉。
			return ErrIdempotencyConflict
		}

		childRun := toRunEntity(childRow)
		childRun.LastEventSeq = 1
		out = ResumeRunOutput{Child: childRun, IsNew: true, Resolved: resolved}
		parentID = parentRow.ID
		return nil
	})
	if err != nil {
		return ResumeRunOutput{}, err
	}
	if out.IsNew {
		// notifyRunWake 只在真的建立新 child 時才需要——!IsNew 分支沒有任何
		// mutation，沒有新事件可通知（v1 已抓到、Task 14 遺漏的缺口：
		// AppendRunEvent／TerminalizeRun／CancelRun 都有這個 commit 後通知，
		// 只有 ResumeRun 沒有，導致 /runs/:runId/events 訂閱者看不到
		// RUN_RESUMED／RUN_ACCEPTED）。
		r.notifyRunWake(parentID)
		r.notifyRunWake(childID)
	}
	return out, nil
}

type RestoreInterruptAfterFailedResumeInput struct {
	InterruptID string
	BuyerID     identity.BuyerID
	ChildRunID  int64
}

func (r *Repo) RestoreInterruptAfterFailedResume(ctx context.Context, in RestoreInterruptAfterFailedResumeInput) (bool, error) {
	var restored bool
	var parentID int64

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		interruptRow, err := q.GetProcurementAgentRunInterruptForUpdate(txCtx, in.InterruptID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if interruptRow.ResolvedByChildRunID == nil || *interruptRow.ResolvedByChildRunID != in.ChildRunID {
			return nil
		}

		parentRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
			ID: interruptRow.RunID, BuyerID: int64(in.BuyerID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		runRows, err := q.RestoreProcurementAgentRunToWaitingForInput(txCtx, gen.RestoreProcurementAgentRunToWaitingForInputParams{
			ID: parentRow.ID, BuyerID: int64(in.BuyerID), ChildRunID: pointerInt64(in.ChildRunID),
		})
		if err != nil {
			return err
		}
		if runRows != 1 {
			return nil
		}

		interruptRows, err := q.RestoreProcurementAgentRunInterruptToPending(txCtx, gen.RestoreProcurementAgentRunInterruptToPendingParams{
			ID: in.InterruptID, ChildRunID: pointerInt64(in.ChildRunID),
		})
		if err != nil {
			return err
		}
		if interruptRows != 1 {
			return ErrInterruptAlreadyResolved
		}

		parentNextSeq := parentRow.LastEventSeq + 1
		payload, err := json.Marshal(map[string]string{
			"interruptId": interruptRow.ID, "interruptKind": interruptRow.InterruptKind,
		})
		if err != nil {
			return err
		}
		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: parentRow.ID, Seq: parentNextSeq,
			EventType:     codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value,
			SchemaVersion: "v1", PublicPayloadJson: payload,
			OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}
		if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
			ID: parentRow.ID, LastEventSeq: parentNextSeq,
		}); err != nil {
			return err
		}

		parentID = parentRow.ID
		restored = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if restored {
		r.notifyRunWake(parentID)
	}
	return restored, nil
}

func toInterruptEntity(row gen.ProcurementAgentRunInterrupt) Interrupt {
	return Interrupt{
		ID: row.ID, RunID: row.RunID, InterruptKind: row.InterruptKind,
		ResponseSchemaVersion: row.ResponseSchemaVersion, Status: row.Status,
		ResolvedByChildRunID: int64Value(row.ResolvedByChildRunID),
		PayloadJSON:          row.PayloadJson,
		FactsAsOf:            timeValue(row.FactsAsOf),
	}
}

func (r *Repo) GetRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (Run, error) {
	row, err := r.q(ctx).GetProcurementAgentRun(ctx, gen.GetProcurementAgentRunParams{ID: runID, BuyerID: int64(buyerID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	return toRunEntity(row), nil
}

func (r *Repo) ListRunEventsAfterSeq(ctx context.Context, runID, afterSeq int64) ([]RunEvent, error) {
	rows, err := r.q(ctx).ListProcurementAgentRunEventsAfterSeq(ctx, gen.ListProcurementAgentRunEventsAfterSeqParams{
		RunID: runID, Seq: afterSeq,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RunEvent, len(rows))
	for i, row := range rows {
		out[i] = toRunEventEntity(row)
	}
	return out, nil
}

// HasIdempotencyKey 只讀，回答「這把 (buyerID, clientRequestID) 是否已經有
// command row」，不判斷 hash 是否相符——那是 ClaimRun/ResumeRun 自己內部
// FindProcurementAgentRunCommandByIdempotencyKey 的職責。這裡只用來決定
// usecase.GuardNoOpenInterrupt 該不該執行（Task 14b addendum：guard 只能對
// 「這把鍵還沒有 command row」的真正新命令生效，冪等重送必須放行到 ClaimRun
// 自己的既有回放路徑），本身不做任何寫入。複用 ClaimRun／ResumeRun 已經在用
// 的同一支 query，不新增 query。
func (r *Repo) HasIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (bool, error) {
	_, err := r.q(ctx).FindProcurementAgentRunCommandByIdempotencyKey(ctx, gen.FindProcurementAgentRunCommandByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindOpenInterruptForRequest 落地 Task 14b B3 的 guard 查詢：該 request 目前
// 是否被一個「已 park 且尚未回覆」的 run 佔用。見
// FindOpenProcurementInterruptForRequest（runs.sql）的查詢文件——run→request
// 的連結只有 result_revision_id 這一條。無 open interrupt 時回 ErrNotFound，
// 與本檔其餘 GetXxx 方法找不到列時的既有慣例一致。
func (r *Repo) FindOpenInterruptForRequest(ctx context.Context, requestID int64, buyerID identity.BuyerID) (Interrupt, error) {
	row, err := r.q(ctx).FindOpenProcurementInterruptForRequest(ctx, gen.FindOpenProcurementInterruptForRequestParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Interrupt{}, ErrNotFound
		}
		return Interrupt{}, err
	}
	return toInterruptEntity(row), nil
}

// ---------------------------------------------------------------------------
// Task 15 write-path 修復：procurement_agent_runs.conversation_id 補齊
//
// 這一段是 Task 15 review 過程中發現的既有缺口：run 建立時（ClaimRun／
// CreateLedgerRunForCandidateSearch）從未寫過 conversation_id（見 runs.sql
// FindOpenProcurementInterruptForRequest 的既有註解），導致這欄一直是
// NULL、schema 上為它建的 idx_procurement_agent_runs_conversation 索引形同
// 虛設。這裡補齊寫入路徑，讓 runtime-snapshot 端點（見下方
// ListRunsByConversation／procurement_handler.go GetSnapshot）能真的用
// conversation_id 查出一個 conversation 底下的所有 run，不必像
// FindOpenProcurementInterruptForRequest 那樣繞道 result_revision_id
// join（那支 query 有自己的理由維持原樣，見它自己的文件）。
// ---------------------------------------------------------------------------

// conversationIDForRequest 是純讀取版的 conversation 解析，供 ClaimRun／
// CreateLedgerRunForCandidateSearch 在建立新 run 時使用（呼叫端已經在一個
// r.txManager.WithTx 交易內，這支用同一個 ctx／txCtx 查詢，落在同一個交易
// 裡）。回傳 0, nil（不是 error）代表目前還查不到——不是硬性失敗：
// CreateRequest 保證在自己的交易內建立 conversation row，所以呼叫端只要
// requestID 指向一個真實存在的 request，幾乎總是查得到；例外情境（窄
// rolling-deploy race、或這個 request 建立於 conversation 接線存在之前）見
// GetConversationIDForRequest 與各呼叫端自己的文件——那些情境由下一次
// AppendRevision／RewindTo 的 get-or-create 自我修復，或由
// executeTurnPipeline／executeAnswerPipeline／executeRewindPipeline 事後呼叫
// SetRunConversationID 回填，不勞這支查詢處理（它刻意不做寫入——呼叫端已經
// 在交易裡，不該再疊一個 get-or-create 的隱性寫入進去）。
func (r *Repo) conversationIDForRequest(ctx context.Context, requestID int64) (int64, error) {
	row, err := r.q(ctx).GetProcurementConversationByRequestID(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return row.ID, nil
}

// GetConversationIDForRequest 是 conversationIDForRequest 的匯出版本，供
// internal/grpc 的 GetSnapshot RPC handler 直接呼叫（Task 15 write-path 修復
// round 2 Minor #3）——原本疊在共用的 repository.GetSnapshot（revision.go）
// 裡，但那支被 11 個呼叫點共用，其中只有這一個真的需要 conversationId，見
// revision.go GetSnapshot 的文件。行為與 conversationIDForRequest 完全相同
// （包括「查不到回 0, nil，不是 error」的語意），只是開放給套件外呼叫。
func (r *Repo) GetConversationIDForRequest(ctx context.Context, requestID int64) (int64, error) {
	return r.conversationIDForRequest(ctx, requestID)
}

// SetRunConversationID 回填一個 run 的 conversation_id——目前唯一的呼叫情境
// 是 TURN pipeline 對「全新對話」的第一個 run：ClaimRun 當下 RequestID==0
// （conversation 還不存在），要等 usecase.TurnUseCase.executeTurnPipeline
// 呼叫 CreateRequest／AppendRevision 之後才知道真正的 conversation_id（見
// ClaimRunInput.RequestID 文件）。純粹是 metadata 補寫，不是狀態轉移：不碰
// version／last_event_seq，也不像狀態轉移那樣需要任何並行守衛——
// `conversation_id IS NULL` 這個 guard（SetProcurementAgentRunConversationID
// SQL）本身就讓重複呼叫或呼叫在一個已經有值的 run 上安全地變成 no-op。
func (r *Repo) SetRunConversationID(ctx context.Context, runID, conversationID int64) error {
	return r.q(ctx).SetProcurementAgentRunConversationID(ctx, gen.SetProcurementAgentRunConversationIDParams{
		ID: runID, ConversationID: int64OrNil(conversationID),
	})
}

// ListRunsByConversation 列出一個 conversation 底下的全部 run，最新的排最
// 前面（Task 15，doc57 §5.6）——runtime-snapshot 端點用它組 runs[] 並掃出
// 目前的 active run。buyerID 是租戶邊界：即使呼叫端手上的 conversationID
// 正確，也不允許查到別的買家的 run。
func (r *Repo) ListRunsByConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID) ([]Run, error) {
	rows, err := r.q(ctx).ListProcurementAgentRunsByConversation(ctx, gen.ListProcurementAgentRunsByConversationParams{
		ConversationID: int64OrNil(conversationID), BuyerID: int64(buyerID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Run, len(rows))
	for i, row := range rows {
		out[i] = toRunEntity(row)
	}
	return out, nil
}

// GetOpenInterruptForRun 查出一個 run 目前未解的 interrupt（Task 15）——
// runtime-snapshot 端點只在 active run 真的是 WAITING_FOR_INPUT 時才呼叫
// 這支，組出 openConfirmation 供 website 恢復 pending interrupt UI。
// ok=false（配 Interrupt{}, nil）表示這個 run 沒有未解的 interrupt，不是
// error——呼叫端不需要用 errors.Is 分辨「查詢失敗」與「本來就沒有」。
//
// buyerID 是 review round 2 Minor #4 補上的租戶邊界參數：
// procurement_agent_run_interrupts 本身沒有 buyer_id 欄位，query 改成 join
// procurement_agent_runs 取得它，跟既有姊妹方法 FindOpenInterruptForRequest
// 的邊界寫法一致——不再只靠呼叫端「runID 一定來自這個買家自己查到的 run」這個
// 沒有強制執行的前提。
func (r *Repo) GetOpenInterruptForRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (Interrupt, bool, error) {
	row, err := r.q(ctx).GetOpenProcurementAgentRunInterruptByRunID(ctx, gen.GetOpenProcurementAgentRunInterruptByRunIDParams{
		RunID: runID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Interrupt{}, false, nil
		}
		return Interrupt{}, false, err
	}
	return toInterruptEntity(row), true, nil
}

// ListLatestArtifactsByConversation 的租戶邊界在 query 內 join run row 取得
// （artifact 表沒有 buyer_id），同 GetOpenInterruptForRun。
func (r *Repo) ListLatestArtifactsByConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID) ([]RunArtifact, error) {
	rows, err := r.q(ctx).ListLatestProcurementAgentRunArtifactsByConversation(ctx, gen.ListLatestProcurementAgentRunArtifactsByConversationParams{
		ConversationID: int64OrNil(conversationID), BuyerID: int64(buyerID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]RunArtifact, len(rows))
	for i, row := range rows {
		out[i] = toRunArtifactEntity(row)
	}
	return out, nil
}

// RunArtifact 是 procurement_agent_run_artifacts 的一列（doc 57 §6.2）。
// 對外投影用 domain.Artifact；這裡是持久層形狀。
type RunArtifact struct {
	ArtifactID     int64
	RunID          int64
	ArtifactType   string
	Version        int32
	Status         string
	PublicDataJSON []byte
	Checksum       string
	DomainRefType  string
	DomainRefID    string
	AsOf           time.Time
	CreatedAt      time.Time
}

// UpsertRunArtifactInput 是產製一版 artifact 的完整輸入。
//
// 這一層對 ArtifactType 與 PublicDataJSON 的形狀不做任何判斷——13 型各自的 data
// 形狀是各自的業務設計（doc 57 §4.5：未展開的型別不得預先發明欄位），持久層只負責
// 「寫一版 ＋ 在同一交易 append ARTIFACT_UPSERTED」這件對每一型都一樣的事。
type UpsertRunArtifactInput struct {
	RunID   int64
	BuyerID identity.BuyerID
	// ArtifactType 是 ProcurementArtifactType 值域（CodeMeta）。
	ArtifactType string
	// Status 是 ProcurementArtifactStatus 值域（DRAFT/READY/STALE/CONSUMED/FAILED）。
	Status         string
	PublicDataJSON []byte
	Checksum       string
	// DomainRefType／DomainRefID 是 allowlisted domain 參照（例如
	// candidateSetId），供 owner restore 與 §6.4-6 的 CAS 使用。
	DomainRefType string
	DomainRefID   string
	// AsOf 是這版 artifact 據以產製的**事實**時間，不是寫入時間（doc 57 §21：
	// 事件／業務時間與 DB audit time 不可互相冒充；created_at 才是後者）。
	AsOf time.Time
}

// UpsertRunArtifact 產製一版 typed artifact，並在同一交易內 append 一筆
// ARTIFACT_UPSERTED（doc 57 §4.2、§6.2）。
//
// 三件事同一個 transaction，理由與 AppendRunEvent 相同：
//
//  1. **身分與版本配號在 run row lock 內**。artifact 的身分鍵是 (run_id,
//     artifact_type)——同一型重新產製沿用既有 artifact_id、版本 +1（doc 57 §4.5：
//     version 是「這個 artifact_id 的第幾次產製」，不是 data schema 版本）。配號在
//     鎖內做而不是靠 MAX+1 裸讀（§6.1 明文禁止），PRIMARY KEY (artifact_id, version)
//     是最終防線。
//  2. **artifact 落庫與 ledger 事件不可分離**。只寫 artifact 不寫事件，買家的 replay
//     永遠看不到它出現過；只寫事件不寫 artifact，replay 會指向一個不存在的 id。
//  3. **租戶邊界靠 run row**。artifact 表沒有 buyer_id 欄位，唯一的邊界是這裡對
//     run row 帶 buyer_id 的 FOR UPDATE 查詢——查不到就是 ErrNotFound，不得放寬。
//
// 舊版本不 UPDATE：artifact 是不可變快照，每一版都要留給 replay 與 owner restore 讀。
func (r *Repo) UpsertRunArtifact(ctx context.Context, in UpsertRunArtifactInput) (RunArtifact, error) {
	var out RunArtifact
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		runRow, err := q.GetProcurementAgentRunForUpdate(txCtx, gen.GetProcurementAgentRunForUpdateParams{
			ID: in.RunID, BuyerID: int64(in.BuyerID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		// 既有身分（若有）：同一個 run 的同一型 artifact 是同一個 artifact_id。
		artifactID := r.newID()
		nextVersion := int32(1)
		latest, err := q.FindLatestProcurementAgentRunArtifact(txCtx, gen.FindLatestProcurementAgentRunArtifactParams{
			RunID: in.RunID, ArtifactType: in.ArtifactType,
		})
		switch {
		case err == nil:
			artifactID = latest.ArtifactID
			nextVersion = latest.Version + 1
		case errors.Is(err, pgx.ErrNoRows):
			// 第一次產製，沿用上面鑄好的新 id 與 version 1。
		default:
			return err
		}

		row, err := q.InsertProcurementAgentRunArtifact(txCtx, gen.InsertProcurementAgentRunArtifactParams{
			ArtifactID: artifactID, RunID: in.RunID, ArtifactType: in.ArtifactType,
			Version: nextVersion, Status: in.Status,
			PublicDataJson: json.RawMessage(in.PublicDataJSON), Checksum: in.Checksum,
			DomainRefType: in.DomainRefType, DomainRefID: in.DomainRefID,
			AsOf: timestamptzParam(in.AsOf),
		})
		if err != nil {
			return err
		}

		// ARTIFACT_UPSERTED 只帶參照，不內嵌完整 PublicAgentArtifactV1
		// （doc 57 §4.2 event data 欄明文）：ledger 事件是通知不是內容，內容由
		// runtime-snapshot／artifact 讀取端負責，兩邊各存一份會立刻分裂。
		payload, err := json.Marshal(artifactUpsertedPayload{
			ArtifactID: strconv.FormatInt(artifactID, 10),
			Type:       in.ArtifactType,
			Version:    nextVersion,
		})
		if err != nil {
			return err
		}

		nextSeq := runRow.LastEventSeq + 1
		if _, err := q.AppendProcurementAgentRunEvent(txCtx, gen.AppendProcurementAgentRunEventParams{
			ID: r.newID(), RunID: in.RunID, Seq: nextSeq,
			EventType:     codemeta.ProcurementLedgerEventTypeArtifactUpserted.Value,
			SchemaVersion: "v1", PublicPayloadJson: payload,
			OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(txCtx),
		}); err != nil {
			return err
		}
		if err := q.BumpProcurementAgentRunLastEventSeq(txCtx, gen.BumpProcurementAgentRunLastEventSeqParams{
			ID: in.RunID, LastEventSeq: nextSeq,
		}); err != nil {
			return err
		}

		out = toRunArtifactEntity(row)
		return nil
	})
	if err != nil {
		return RunArtifact{}, err
	}

	// commit 之後才叫醒 watcher（doc 57 事件不變式 3），同 AppendRunEvent。
	r.notifyRunWake(in.RunID)
	return out, nil
}

// artifactUpsertedPayload 是 ARTIFACT_UPSERTED 的 public payload。artifactId 走
// 字串：對外 JSON 的 ID 一律字串（API 規範 §4，避免 JS 精度問題），而這個 payload
// 會原樣經由 gateway 投影成 AG-UI `CUSTOM name="artifact"` 的 frame。
type artifactUpsertedPayload struct {
	ArtifactID string `json:"artifactId"`
	Type       string `json:"type"`
	Version    int32  `json:"version"`
}

func toRunArtifactEntity(row gen.ProcurementAgentRunArtifact) RunArtifact {
	return RunArtifact{
		ArtifactID: row.ArtifactID, RunID: row.RunID, ArtifactType: row.ArtifactType,
		Version: row.Version, Status: row.Status, PublicDataJSON: row.PublicDataJson,
		Checksum: row.Checksum, DomainRefType: row.DomainRefType, DomainRefID: row.DomainRefID,
		AsOf: row.AsOf.Time, CreatedAt: row.CreatedAt.Time,
	}
}
