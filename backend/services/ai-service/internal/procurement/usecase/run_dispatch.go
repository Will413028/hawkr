package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"
)

// RunClaimStore 是 Task 7-9（TURN/ANSWER/REWIND 的 claim+dispatch 包裝）需要
// 的最小持久層能力，窄化自 *repository.Repo（同 RevisionStore／AnswerStore
// 等既有介面宣告在消費端 package 的既有慣例，見 module.go 檔頭註解）。
type RunClaimStore interface {
	ClaimRun(ctx context.Context, in repository.ClaimRunInput) (repository.Run, bool, error)
	AppendRunEvent(ctx context.Context, in repository.AppendRunEventInput) (repository.RunEvent, error)
	// GetRun 供 runCancelObserver 在背景執行路徑上讀回 run 的當前狀態，回答
	// 「買家是不是按了停止」。在它之前 CANCEL_REQUESTED 是唯寫的——沒有任何
	// 執行路徑讀它，停止鈕因此按下去什麼都不會停（見 run_cancel_observer.go）。
	GetRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error)
	TerminalizeRun(ctx context.Context, in repository.TerminalizeRunInput) (bool, error)
	// CreateInterrupt 供 Task 7／8 在 crop 消歧待選時呼叫（spec §10.1，2026-08-27
	// 更新）——本輪 v1 唯一已知呼叫情境，見 interruptOutcomeFor。
	CreateInterrupt(ctx context.Context, in repository.CreateInterruptInput) (repository.Interrupt, error)
	// HasIdempotencyKey 供 answer.go 的 ClaimAndDispatchAnswer 開頭判斷這把
	// (buyerID, clientRequestID) 是不是真正的新命令（Task 14b addendum）：
	// GuardNoOpenInterrupt 只能對真正的新命令生效，冪等重送必須放行到
	// ClaimRun 自己的既有回放路徑，否則會複製 C3 修掉的同一種失效形狀（同鍵
	// 重送拿到錯誤而非同一個結果），只是從 resume 搬到 /answers。
	HasIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (bool, error)
	// FindOpenInterruptForRequest 供 GuardNoOpenInterrupt 用（Task 14b B3）。
	FindOpenInterruptForRequest(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Interrupt, error)
	// SetRunConversationID 供 executeTurnPipeline 在「全新對話」的第一個 TURN
	// run 上回填 conversation_id（Task 15 write-path 修復）：ClaimRun 在 claim
	// 當下（TurnInput.RequestID==0）解不出 conversation_id，要等 pipeline 呼叫
	// CreateRequest／AppendRevision 之後才知道——見
	// repository.ClaimRunInput.RequestID 與 repository.SetRunConversationID
	// 的文件。ANSWER／REWIND 的 run 一律在 claim 當下就解出 conversation_id
	// （RequestID 恆非零），不需要呼叫這個方法。
	SetRunConversationID(ctx context.Context, runID, conversationID int64) error
	// FindByIdempotencyKey 供 turn_run_lifecycle.go 的 replayTurnRun 使用
	// （2026-08-30，turn run lifecycle 共用層）——把 TurnUseCase.replayExistingRun
	// 原本走 uc.store（RevisionStore）的這個方法一併搬進這個窄介面，讓
	// replayTurnRun 能在不依賴 TurnUseCase 整個型別的情況下運作，供 graph
	// executor 共用。*repository.Repo 已經實作這個方法（revision.go），這裡只是
	// 把既有方法一併暴露進這個窄介面，同 turn.go RevisionStore 的既有慣例。
	FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error)
}

// interruptOutcomeFor 判斷這次 pipeline 的結果是不是「crop 消歧待選」，供
// Task 7（executeTurnPipeline）／Task 8（executeAnswerPipeline）共用，terminal
// 判斷邏輯完全對稱，不重複寫兩份。是的話回傳非空 interruptKind，呼叫端據此把
// terminal status 改成 WAITING_FOR_INPUT 並呼叫 CreateInterrupt；不是的話回
// ok=false，呼叫端走既有的 Succeeded／Failed 判斷，行為不變。
//
// 判斷依據：這一輪最終要問買家的問題是不是那張「品項消歧」卡，也就是
// `result.Question.SlotKey == domain.SlotCropDisambiguation`。
//
// **不能用 `DialoguePlan.ActCode == ActItemNeedsDisambiguation` 判斷**（本函式
// 原本的寫法，Task 7 接線時發現是錯的）：`selectActCode`
// （domain/dialogue_planner.go）的 switch 把 `CropDisambiguationPending` 排在
// `Degraded`／`HasRejected`／`computeHasCorrection` **之後**，所以一輪「品項待
// 消歧、同時買家又修正了某個已填欄位」的對話會選到 `ActCorrectionApplied`，
// act code 因此看不出待消歧——但消歧卡**還是會出現**（turn.go 在
// `cropGate.Pending` 時無條件覆蓋 question，與 act code 的選擇互相獨立）。
// 結果就是：買家眼前卡著一張非答不可的消歧卡，run 卻以 SUCCEEDED 收尾、沒有
// 任何 interrupt row，resume 機制永遠不會被觸發。
//
// 改用 Question.SlotKey 是與 spec §10.1 所述觸發條件（`cropidentity.ComputeGate` 回
// `Pending: true`）1:1 等價的判準：`domain.SlotCropDisambiguation` 這個哨兵
// SlotKey 只由 `domain.BuildCropDisambiguationQuestion`
// （domain/pick_next_question.go）產生，而它的三個呼叫點（turn.go／answer.go／
// rewind.go）都被 `if cropGate.Pending` 包著——沒有第二條路徑會產生這個值。
// `PickNextQuestion` 本身永遠不會回傳它（它只走 domain.Slots 的真實欄位）。
//
// 這個判準對 Task 8／9 的 answer／rewind 一樣成立（同樣三個呼叫點），所以兩個
// task 直接沿用本函式即可，不需要各自另寫一份。
func interruptOutcomeFor(result TurnResult) (interruptKind string, ok bool) {
	if result.Question != nil && result.Question.SlotKey == domain.SlotCropDisambiguation {
		return "CLARIFICATION", true
	}
	return "", false
}

// turnPipelineOutcome 是 executeTurnPipeline（Task 7）／executeAnswerPipeline
// （Task 8）共用的內部回傳形狀——interruptID／interruptKind 非空表示這次終態是
// WAITING_FOR_INPUT，供 ClaimAndDispatchTurn／ClaimAndDispatchAnswer 往上傳給
// SubmitTurnResult（見 Task 7 開頭該型別的欄位註解）。
type turnPipelineOutcome struct {
	result TurnResult
	// events 是這次執行 HandleStream 收集到的完整 TurnEvent 序列，供串流入口在
	// pipeline 跑完後把既有的 StepEvent／MessageEvent／TurnResult frame 照原順序
	// 送出（Task 7；本輪串流即時性的暫時降級說明見 SubmitTurnResult.Events）。
	events        []TurnEvent
	interruptID   string
	interruptKind string
}

// runDispatchBudget 給背景執行的預算上限，必須高於 gateway 的
// streamTurnDeadline（120s）——EXTRACT_SLOTS 實測 7.3-9.5s，DialoguePlanner／
// SurfaceRealizer 等後續步驟另計，150s 是保守值，之後有實測數據可再調整。
const runDispatchBudget = 150 * time.Second

// RunDispatcher 把「claim 交易已 commit 的 run」派給背景執行，統一
// TURN／CANDIDATE_SEARCH／CLARIFICATION_RESUME 三種 run type 的執行模型
// （doc57 §8.1 Track 1 baseline：candidate_search.go 的既有 dispatch 是這個
// 模式的原型，本檔把它抽成共用元件供其餘 run type 使用）。
type RunDispatcher struct{}

func NewRunDispatcher() *RunDispatcher { return &RunDispatcher{} }

// Dispatch 用 SafeGo 派發 work，帶一個用 context.WithoutCancel(ctx) 衍生的
// 背景 context（request 生命週期結束不影響背景執行，理由同 candidate_search.go
// dispatch 的既有註解）——WithoutCancel 只切斷取消訊號，trace span 與
// request-scoped logger 都保留，所以背景執行不會在最容易卡住的這段從 trace
// 上消失（issue #3519 slice B Task 5：先前用 context.Background() 兩者一起
// 掉）。work 內部的 panic 由 SafeGo 接住，不會帶走整個 process；本 helper
// 不負責把 panic 轉成任何終態寫入——那是每個呼叫端（Task 7-9）自己 work
// closure 內的責任，因為只有呼叫端知道要寫哪個 run_id 的 RUN_FAILED。
func (d *RunDispatcher) Dispatch(ctx context.Context, work func(bgCtx context.Context)) {
	// ⚠️ context.WithoutCancel(ctx) 只切斷取消訊號，context **values** 全部
	// 保留——這正是它相對 context.Background() 的優點（trace／logger 不丟），
	// 但也代表 pkg/database.WithTx 塞進 ctx 的交易（txCtxKey，一個未匯出的
	// context key）會原封不動跟著 detached 一起傳進背景 goroutine。若未來有
	// 呼叫端在 pkg/database.WithTx 的 closure 內呼叫 Dispatch，
	// database.GetDBTX(bgCtx, pool) 會在背景 goroutine 裡解析出呼叫端的
	// pgx.Tx，而該交易在呼叫端的 WithTx 返回時就會被 commit／rollback——
	// pgx.Tx 並非 goroutine-safe，這會是一個沒有編譯錯誤、沒有測試失敗、也
	// 不會留 log 的靜默資料損毀路徑（final review Finding 5；今天所有 8 個
	// Dispatch／dispatch 呼叫點都不在任何 WithTx closure 內，經逐一核對 34
	// 個 WithTx 呼叫點證實不可達，但 context.Background() 曾經結構性地擋住
	// 這件事，我們為了保留 trace 刻意放棄了那層保護）。⛔ 不可從
	// pkg/database.WithTx 的 closure 內呼叫本函式。若未來真的需要在交易內
	// 觸發背景派工：把 Dispatch 需要的資料在呼叫 WithTx 之前準備好、在
	// WithTx 之外呼叫 Dispatch，而不是把交易 context 一路往下傳。
	detached := context.WithoutCancel(ctx)
	syncutil.SafeGo(detached, func() {
		bgCtx, cancel := context.WithTimeout(detached, runDispatchBudget)
		defer cancel()
		work(bgCtx)
	})
}

// canonicalHash 是 claim transaction 冪等判準的共用底層實作（doc57
// §6.4-2：同 key 同 hash 回原 run，不同 hash 回 409）。用 JSON marshal 後
// sha256，對 []byte（Task 14 的 ResumeRun payload）與具名 struct（Task 7-9
// 的各自 Input）都適用。
//
// canonicalTurnHash／canonicalAnswerHash／canonicalRewindHash（Task 7-9 呼叫）
// 不在本檔實作——它們是窄化自各自 TurnInput／AnswerInput／RewindInput 的
// wrapper，只挑「同一個命令」在語意上應該相同的欄位（通常是 buyerID＋這次命令
// 的實質內容），不是整個 struct 直接丟給 canonicalHash。實作者屆時開工前先讀
// 對應 Input struct 的實際欄位定義，寫成類似：
//
//	func canonicalTurnHash(in TurnInput) string {
//		return canonicalHash(struct {
//			BuyerID identity.BuyerID
//			Message string
//		}{in.BuyerID, in.Message})
//	}
//
// 的窄化 wrapper，canonicalAnswerHash／canonicalRewindHash 依樣畫葫蘆。
func canonicalHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// canonical hash 算不出來代表輸入本身序列化失敗，這是呼叫端輸入的
		// 問題（例如帶了不可序列化的型別），不是可以吞掉的錯誤——用一個
		// 固定的 sentinel 值讓所有序列化失敗的呼叫都撞同一個 hash，claim
		// 交易會在應用層之外由 DB 唯一索引擋下重複，不會靜默通過。
		b = []byte("canonical-hash-marshal-error")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CanonicalResumePayloadHash 是 canonicalHash 的匯出 wrapper，供
// internal/grpc 的 ResumeRun handler（Task 14／14b）計算
// repository.ResumeRunInput.PayloadHash。ResumeRun 沒有 canonicalTurnHash／
// canonicalAnswerHash／canonicalRewindHash 那樣的 usecase 層
// ClaimAndDispatchXxx 包裝可以順路呼叫這個不匯出的 canonicalHash——
// repository.ResumeRun（Task 4）本身就是這個命令唯一的 durable 交易，直接由
// grpc 層呼叫（見 procurement_run_events_handler.go RunStore 介面的文件）。
// 匯出這一個窄 wrapper，而不是把 canonicalHash 本身改成匯出：後者會讓
// internal/grpc 誤以為可以拿它去雜湊任意型別，這裡刻意只開放「對 response
// payload 這個 []byte 算 hash」這一種用法，呼叫端不需要也不應該知道底層是
// JSON marshal + sha256。
//
// Task 14b M4：納入 BuyerID 與 InterruptID——與 canonicalAnswerHash（answer.go）
// 刻意納入 RequestID 的理由逐字相同：「任何一個不同都代表這把鍵被挪去用在
// 另一個命令上，理當被 hash 不符擋下」。resume 的「另一個命令」就是「另一個
// interrupt」，把它排除在 hash 外等於讓最關鍵的那一維失去保護——買家對
// interrupt I1 用 key X 成功後，對**另一個** interrupt I2 送同一把 key、
// 同一串 response bytes，若 hash 不含 InterruptID 就會命中 I1 的冪等鍵、
// 回放 I1 的 child，而 I2 從未被真正解析。
func CanonicalResumePayloadHash(buyerID identity.BuyerID, interruptID string, payload []byte) string {
	return canonicalHash(struct {
		BuyerID     identity.BuyerID
		InterruptID string
		Payload     []byte
	}{buyerID, interruptID, payload})
}

// ---------------------------------------------------------------------------
// Resume claim + dispatch（Task 14b：ResumeRun 派工執行，root-cause fix）
//
// 取消原設計新檔 resume.go／ResumeUseCase type／NewResumeUseCase 建構子／
// module.go DI binding：全部改寫成本檔的 package-level 宣告，理由見設計文件
// §3——interrupt kind 的消費端 routing 與生產端 interruptOutcomeFor 落在同一
// 個檔案，#2450 要加一種 kind 時兩個方向都在眼前；本檔已經承載 kind 知識
// （interruptOutcomeFor 自己就寫死 "CLARIFICATION"），這不是污染一個
// kind-agnostic 的檔案。
// ---------------------------------------------------------------------------

// ResumeRunStore 內嵌 RunClaimStore：child run 的執行走的就是既有 answer
// pipeline，需要同一組 AppendRunEvent／TerminalizeRun。內嵌讓 resume 也拿得到
// 它絕對不該呼叫的 ClaimRun（v1 草案「child 仍 QUEUED 就補派」方案的錯誤），
// 這是被 executeAnswerPipeline 的參數型別逼出來的，接受。
type ResumeRunStore interface {
	RunClaimStore
	ResumeRun(ctx context.Context, in repository.ResumeRunInput) (repository.ResumeRunOutput, error)
	RestoreInterruptAfterFailedResume(ctx context.Context, in repository.RestoreInterruptAfterFailedResumeInput) (bool, error)
}

// ResumeResult 只有兩個欄位。刻意不帶 TurnResult：那是 answer 形狀的型別，
// 塞進一個 kind-agnostic 的結果型別等於預先替 QUOTE_APPROVAL 決定它的結果長
// 什麼樣。wire（ResumeRunResponse，ai.proto:1096-1099）本來就只有這兩欄。
type ResumeResult struct {
	RunID     int64
	RunStatus string
	// Result 是這一輪 resume 真正跑出來的 TurnResult。pipeline 本來就會算出
	// 它（turnPipelineOutcome.result）並持久化，先前只是沒有交出去，client
	// 因此拿不到「買家回答之後的下一題」——snapshot 沒有 question 欄位，
	// run ledger 的 RUN_FINISHED public_payload_json 是空的 `{}`，replay
	// 端點又是 flag-gated（2026-09-03 dev 實測三條路都試過）。
	//
	// nil 代表這一輪沒有跑 pipeline：冪等重播（同一組 buyer_id +
	// client_request_id 重送）只回現況，見 C2 裁決。
	Result *TurnResult
}

type ResumeInput struct {
	BuyerID         identity.BuyerID
	InterruptID     string
	ClientRequestID string
	ResponsePayload []byte
}

// ErrUnsupportedInterruptKind 表示這個 interrupt 的 kind／response schema
// version 不是本次 pass 認得的（v1 唯一支援 interrupt_kind="CLARIFICATION" +
// response_schema_version="v1"，或 payload 語意上解不出來——見
// ClaimAndDispatchResume 的 ResolveInput closure）。對映 Task 14b
// SCHEMA_VERSION_UNSUPPORTED（400）。這個 guard 在 T0 交易內、任何 mutation
// 之前執行（M5），失敗時 ledger 零 mutation，不會像方案 A 那樣先燒毀 parent
// 才發現 kind 不支援。
var ErrUnsupportedInterruptKind = errors.New("procurement: unsupported interrupt kind or response schema version")

// errResumePipelinePanicked／resumeDispatchResult 同 answer.go
// errAnswerPipelinePanicked／answerDispatchResult 的既有模式：panic 由
// RunDispatcher.Dispatch 內的 SafeGo 接住，呼叫端拿到的是這個具名 error，不
// 會永遠卡在 <-done。
var errResumePipelinePanicked = errors.New("procurement: resume pipeline panicked")

type resumeDispatchResult struct {
	outcome turnPipelineOutcome
	err     error
}

// cropDisambiguationResponseV1 是 interrupt_kind="CLARIFICATION" +
// response_schema_version="v1" 的唯一 payload schema，欄位與
// buildCropDisambiguationPatch（answer.go）1:1 對應。
type cropDisambiguationResponseV1 struct {
	OptionID               string `json:"optionId"`
	FreeText               string `json:"freeText,omitempty"`
	SelfEnteredPriceAmount string `json:"selfEnteredPriceAmount,omitempty"`
}

// resumeInputRefV1 是落進 procurement_agent_run_inputs.input_ref 的伺服器
// 推導完成的 resolved input（Task 14b B2）——RequestID／ExpectedRevisionID
// 是伺服器從 parent.result_revision_id 推導出來的，不是 client 送來的原始
// payload；child run 因此是 self-describing 的，不只活在這次 HTTP 請求的
// goroutine 裡。ID 欄位一律字串（CLAUDE.md §4：對外 ID 避免 JS 精度問題）。
type resumeInputRefV1 struct {
	RequestID              string `json:"requestId"`
	ExpectedRevisionID     string `json:"expectedRevisionId"`
	SlotKey                string `json:"slotKey"`
	OptionID               string `json:"optionId"`
	FreeText               string `json:"freeText,omitempty"`
	SelfEnteredPriceAmount string `json:"selfEnteredPriceAmount,omitempty"`
}

// ClaimAndDispatchResume 是 ResumeRun gRPC handler（flag on）走的入口——doc57
// §5.5 + Task 14b 的 root-cause 修正：repository.ResumeRun（Task 4）建立
// child run 之後，這裡才是真正執行它的地方；在本 task 之前完全沒有任何程式碼
// 呼叫 executeAnswerPipeline，child 永遠停在 QUEUED。
//
// 複用 executeAnswerPipeline，不新增第二套執行骨架（設計文件 §2 的核心裁決）：
// child run 以 SlotCropDisambiguation 答題的形狀送進既有、完全未修改的
// AnswerUseCase.Handle，「child 不會遞迴 park」是結構性保證（見設計文件 §2）。
//
// !IsNew（冪等命中）分支**不 dispatch、不補派、不回放 TurnResult**——與
// ClaimAndDispatchTurn／ClaimAndDispatchAnswer／ClaimAndDispatchRewind 的
// !isNew 分支同形狀（v1 草案「child 仍 QUEUED 就補派」的方案已裁決刪除：
// QUEUED 無法區分「crash 了沒人跑」與「此刻正在跑」，補派會讓最普通的
// client timeout 重試就併發跑同一個 child）。
//
// 已知缺口（與 turn.go 既有的 unrecoverable-mid-pipeline-crash 缺口同級，
// Track 2 lease executor #2452 接手，v1 不做 sweeper）：commit 後、
// dispatch 前／中 crash 會讓 child 永遠停在 QUEUED，無人重驅。
func ClaimAndDispatchResume(
	ctx context.Context, in ResumeInput,
	runStore ResumeRunStore, dispatcher *RunDispatcher, graphDeps GraphControllerDeps,
) (ResumeResult, error) {
	var answerIn AnswerInput

	out, err := runStore.ResumeRun(ctx, repository.ResumeRunInput{
		InterruptID: in.InterruptID, BuyerID: in.BuyerID,
		ClientRequestID: in.ClientRequestID, ResponsePayload: in.ResponsePayload,
		PayloadHash: CanonicalResumePayloadHash(in.BuyerID, in.InterruptID, in.ResponsePayload),
		// ResolveInput 在 T0 交易內、任何 mutation 之前執行（M5）——契約見
		// repository.ResumeRunInput.ResolveInput 的 doc comment：只能做 DB
		// 以外零 I/O 的事，這裡只有 JSON 解碼＋常數比對，天然滿足。
		ResolveInput: func(_ context.Context, rc repository.ResumeResolveContext) (repository.ResolvedRunInput, error) {
			if rc.InterruptKind != "CLARIFICATION" || rc.ResponseSchemaVersion != "v1" {
				return repository.ResolvedRunInput{}, ErrUnsupportedInterruptKind
			}
			var body cropDisambiguationResponseV1
			if jsonErr := json.Unmarshal(rc.ResponsePayload, &body); jsonErr != nil {
				return repository.ResolvedRunInput{}, ErrUnsupportedInterruptKind
			}
			if body.OptionID == "" && strings.TrimSpace(body.FreeText) == "" {
				return repository.ResolvedRunInput{}, ErrInvalidOptionID
			}
			// 伺服器當初問的那一題——買家答案的白名單來源。
			//
			// Kind 必須是消歧 payload 才放行，不是「解得開就算數」：一個空的
			// payload（`{}`，欄位的 DEFAULT）解得開但 Candidates 是空的，而
			// buildCropDisambiguationPatch 對「非 nil 但沒有候選」的判定是
			// UNAVAILABLE——也就是逃生艙可用。少了這道檢查，任何一個 payload 沒寫
			// 成功的 CLARIFICATION 中斷都會變成「買家可以自填任意品項與價格」的
			// 缺口，正是這條白名單存在要擋的東西。
			var asked CropDisambiguationInterruptPayload
			if jsonErr := json.Unmarshal(rc.InterruptPayload, &asked); jsonErr != nil {
				return repository.ResolvedRunInput{}, ErrUnsupportedInterruptKind
			}
			if asked.Kind != cropDisambiguationInterruptKind {
				return repository.ResolvedRunInput{}, ErrUnsupportedInterruptKind
			}
			answerIn = AnswerInput{
				BuyerID: in.BuyerID, RequestID: rc.RequestID,
				SlotKey: domain.SlotCropDisambiguation, OptionID: body.OptionID,
				FreeText: body.FreeText, SelfEnteredPriceAmount: body.SelfEnteredPriceAmount,
				ExpectedRevisionID: rc.ExpectedRevisionID, ClientRequestID: in.ClientRequestID,
				PendingCrop: &asked,
			}
			if err := validateCropResumeAnswer(answerIn); err != nil {
				return repository.ResolvedRunInput{}, err
			}
			inputRef, marshalErr := json.Marshal(resumeInputRefV1{
				RequestID: strconv.FormatInt(rc.RequestID, 10), ExpectedRevisionID: strconv.FormatInt(rc.ExpectedRevisionID, 10),
				SlotKey: string(domain.SlotCropDisambiguation), OptionID: body.OptionID,
				FreeText: body.FreeText, SelfEnteredPriceAmount: body.SelfEnteredPriceAmount,
			})
			if marshalErr != nil {
				return repository.ResolvedRunInput{}, marshalErr
			}
			return repository.ResolvedRunInput{
				InputRefJSON: inputRef, InputSchemaVersion: rc.ResponseSchemaVersion,
				RetentionClass: "BUYER_INPUT", // 新立的字面值，不綁既有 CodeMeta 值域（設計文件 §1-3）
			}, nil
		},
	})
	if err != nil {
		// ResolveInput 失敗（ErrUnsupportedInterruptKind／ErrInvalidOptionID）
		// 或任何其他 repository 層錯誤：T0 已完整 ROLLBACK 或根本沒有走到
		// mutation，零 mutation，直接把錯誤原樣往上傳給 grpc 層映射。
		return ResumeResult{}, err
	}

	if !out.IsNew {
		// C2 裁決：冪等命中只回現況，不重跑 pipeline、不補派。
		return ResumeResult{RunID: out.Child.ID, RunStatus: out.Child.Status}, nil
	}

	done := make(chan resumeDispatchResult, 1)
	dispatcher.Dispatch(ctx, func(bgCtx context.Context) {
		res := resumeDispatchResult{err: errResumePipelinePanicked}
		defer func() {
			r := recover()
			if r != nil {
				// 這個 goroutine 是這個 child run 的唯一擁有者，panic 展開
				// 之後沒有別人會呼叫 TerminalizeRun——先把 run 收成
				// FAILED，避免它永遠卡在非終態。
				terminalizeFailedRun(bgCtx, out.Child, in.BuyerID, runStore)
			}
			done <- res
			if r != nil {
				panic(r) // 交還給 SafeGo 記錄完整 stack，不在這一層靜默吞掉
			}
		}()
		// resume 只有一條控制流：graph。ControllerKind 分支（Sub-project 3b
		// Task 8 引入，非 GRAPH 走 executeAnswerPipeline）於 2026-09-05 退場
		// ——它結構上不可達：child 的 ControllerKind 繼承 parent（ResumeRun
		// 步驟 9），而唯一寫出 DURABLE 的 CreateLedgerRunForCandidateSearch 產
		// 的是 CANDIDATE_SEARCH 的 ledger 投影，不寫 result_revision_id、也不
		// 建 interrupt，所以 ResumeRun 的步驟 7/8 會先回 ErrRunNotResumable；
		// ClaimRun 則從未被以 DURABLE 呼叫（TURN／ANSWER／REWIND 在 procurement
		// graph cutover 之後一律 claim GRAPH）。dev 覆核：164 個 interrupt 與
		// 94 個 resume child 全部是 GRAPH，DURABLE 零。
		outcome, execErr := resumeGraphTurn(bgCtx, graphDeps, out.Child, answerIn)
		res = resumeDispatchResult{outcome: outcome, err: execErr}
	})

	dispatched := <-done
	if dispatched.err != nil {
		if resumeFailureLeavesInterruptAnswerable(dispatched.err) {
			restoreInterruptAfterFailedResume(ctx, runStore, in.InterruptID, in.BuyerID, out.Child.ID)
		}
		// 失敗時仍然交出 RunID，不回零值：這個 run 在 ledger 裡確實存在
		// （T0 已經 commit 了 RUN_ACCEPTED，executeAnswerPipeline 也已經補上
		// RUN_FAILED），呼叫端沒有 run_id 就既無法回頭查這個 run，也無法對
		// 買家解釋發生了什麼——理由與 ClaimAndDispatchAnswer 完全相同。
		return ResumeResult{RunID: out.Child.ID}, dispatched.err
	}
	// 終態 status：interruptID 非空代表這一輪又消歧出一張待選卡，
	// executeAnswerPipeline 把 child 收成 WAITING_FOR_INPUT；否則是一般
	// SUCCEEDED（結構性保證見本函式文件——child 不會遞迴 park，interruptKind
	// 只可能是空字串或再次 "CLARIFICATION"，見 interruptOutcomeFor）。
	runStatus := domain.RunStatusSucceeded()
	if dispatched.outcome.interruptID != "" {
		runStatus = domain.RunStatusWaiting()
	}
	result := dispatched.outcome.result
	return ResumeResult{RunID: out.Child.ID, RunStatus: runStatus, Result: &result}, nil
}

// OpenInterruptStore 是 GuardNoOpenInterrupt 需要的最小持久層能力。
type OpenInterruptStore interface {
	FindOpenInterruptForRequest(ctx context.Context, requestID int64,
		buyerID identity.BuyerID) (repository.Interrupt, error) // 無 open interrupt → repository.ErrNotFound
}

// ErrInterruptPending 表示這個 request 目前被一個「已 park 且尚未回覆」的 run
// 佔用——legacy 寫入路徑（/answers、/rewinds）在這期間不得直接寫入，否則會讓
// interrupt 孤兒化（Task 14b Blocking 3）。對映 APPROVAL_PENDING（409）。
var ErrInterruptPending = errors.New("procurement: request is parked on an open interrupt; answer it via resume")

// GuardNoOpenInterrupt 在 request 目前有一個 open interrupt 時擋下 legacy
// 寫入路徑（/answers 的 ClaimAndDispatchAnswer、/rewinds 的
// ClaimAndDispatchRewind、以及 graph dispatch 自己的 /turns 入口），把買家
// 導向 resume——這是 doc57 Blocking 3 的裁決：parent 處於 WAITING_FOR_INPUT 時
// 打這些路徑會另開一個 TURN run，讓 interrupt 永遠孤兒化、parent 永遠卡在
// WAITING_FOR_INPUT。
//
// 匯出（不是 unexported）：曾經有一個 internal/grpc package 的呼叫點
// （/crop-disambiguation，已隨該端點退場移除）。維持匯出：同
// CanonicalResumePayloadHash 已經在用的「窄 wrapper 匯出給 grpc 層」既有模式，
// 且它是這條不變量對外的唯一名字。
//
// check-then-act race（設計文件 §7.5，逐字保留）：這是無鎖讀，resume 的 T0
// 對 interrupt 持 FOR UPDATE。四種交錯中沒有一種產生錯誤寫入，唯一殘留窗口
// 是 CreateInterrupt 與 TerminalizeRun 之間（兩個獨立呼叫，毫秒級）：guard
// 讀到 no-open（interrupt 已建立但 parent 尚未 terminalize 成 WAITING）→
// 放行 → /answers 自行解掉 crop gate、寫新 revision；parent 稍後 terminalize
// 成 WAITING 並掛一個已失效的 interrupt。買家之後 resume → child 的
// ExpectedRevisionID 已過期 → ErrStaleRevision → child FAILED。
//
// 裁決：不把這個 guard 改成持鎖讀。理由三點：(1) 要真正關掉它，guard 必須與
// AppendRevision 落在同一個交易並對 interrupt 下 FOR UPDATE——那等於把
// Handle() 整條 answer pipeline 包進一個橫跨外部呼叫的長交易，或重構
// Handle()，兩者都是為了一個毫秒級窗口付出線上回歸風險；(2) 這條 race 的壞
// 結果集合是空的——不會產生錯誤的 revision、不會讓兩份寫入互相覆蓋、不會讓
// ledger 說謊，只會把一次 resume 導向 ErrStaleRevision，那是已裁決的正確
// 答案；(3) 本 repo 對同型 race 的既有處理就是「narrow, honestly-documented
// residual」（ClaimRun 的 recoverClaimedRun、CreateInterrupt／TerminalizeRun
// 刻意分兩個交易），這裡採同一種誠實度。
func GuardNoOpenInterrupt(ctx context.Context, store OpenInterruptStore, requestID int64, buyerID identity.BuyerID) error {
	_, err := store.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	if err == nil {
		return ErrInterruptPending
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return err
}

const restoreInterruptTimeout = 5 * time.Second

func resumeFailureLeavesInterruptAnswerable(err error) bool {
	return !errors.Is(err, ErrStaleRevision)
}

func restoreInterruptAfterFailedResume(
	ctx context.Context, store ResumeRunStore, interruptID string, buyerID identity.BuyerID, childRunID int64,
) {
	restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), restoreInterruptTimeout)
	defer cancel()

	restored, err := store.RestoreInterruptAfterFailedResume(restoreCtx, repository.RestoreInterruptAfterFailedResumeInput{
		InterruptID: interruptID, BuyerID: buyerID, ChildRunID: childRunID,
	})
	switch {
	case err != nil:
		pkglogger.Error(restoreCtx, "procurement: failed to restore interrupt after failed resume; buyer cannot answer this disambiguation again",
			zap.Error(err), zap.String("interrupt_id", interruptID), zap.Int64("child_run_id", childRunID))
	case !restored:
		pkglogger.Warn(restoreCtx, "procurement: interrupt not restored after failed resume; another writer already moved it",
			zap.String("interrupt_id", interruptID), zap.Int64("child_run_id", childRunID))
	}
}
