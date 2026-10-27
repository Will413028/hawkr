package grpc

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

// RunStore 是 internal/grpc 這一層需要的 run ledger 讀寫能力（比
// usecase.RunClaimStore 寬——這一層還要服務 replay／cancel／snapshot／
// open-interrupt guard 幾個不同 RPC，各自需要的方法集中宣告在一起，同套件內
// 共用一個 interface，不比照 usecase 那邊拆更細——差別在於
// usecase.RunClaimStore 只服務三個高度對稱的 claim+dispatch 呼叫端，這裡服務
// 形狀不同的 RPC，拆細只會製造重複宣告）。本檔先加 replay 需要的三個方法，
// Task 14 加上 CancelRun，Task 15 加上 ListRunsByConversation／
// GetOpenInterruptForRun（GetSnapshot 用，procurement_handler.go）。
//
// **不含 ResumeRun**（Task 14b 移除）：resume 現在需要真的執行 child run
// （usecase.ClaimAndDispatchResume 呼叫 executeAnswerPipeline），走的是
// `s.durableRun.RunStore`（usecase.ResumeRunStore，procurement_handler.go），
// 不再由這一層呼叫 repository.ResumeRun。
//
// **不含 FindOpenInterruptForRequest**：open-interrupt guard（Task 14b B3）
// 走 `s.durableRun.RunStore`（usecase.ResumeRunStore 內嵌的
// usecase.RunClaimStore 已經宣告這個方法）。procurement graph cutover
// （2026-08-31，Task 5）之後這個 guard 無條件執行，不再由
// `s.durableRun.enabled()` 管轄；改走 `s.runStore`（本介面）沒有額外理由，
// 維持現狀即可。GetOpenInterruptForRun 不受這個顧慮限制——GetSnapshot
// （Task 15）沒有等價的 durableRun 依賴可以改走，本來就只有 `s.runStore`
// 這一條路。
//
// CancelRun／ListRunsByConversation／GetOpenInterruptForRun 的簽章逐字對齊
// repository.Repo 對應方法的簽章（repository/run.go）——*repository.Repo 已經
// 滿足這個介面，procurement/module.go 既有的 `func(r *repository.Repo)
// grpcLayer.RunStore { return r }` DI binding 不需要變動。
type RunStore interface {
	GetRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error)
	ListRunEventsAfterSeq(ctx context.Context, runID, afterSeq int64) ([]repository.RunEvent, error)
	WatchRun(ctx context.Context, runID int64) (chan int64, func(), error)
	CancelRun(ctx context.Context, in repository.CancelRunInput) (repository.Run, string, error)
	// ListRunsByConversation 供 GetSnapshot 組 runs[]／掃出 active run（Task 15，
	// doc57 §5.6）。
	ListRunsByConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID) ([]repository.Run, error)
	// GetOpenInterruptForRun 供 GetSnapshot 在 active run 是 WAITING_FOR_INPUT
	// 時組 openConfirmation（Task 15）。ok=false 表示沒有未解的 interrupt，不是
	// error（見 repository.GetOpenInterruptForRun 文件）。buyerID 是租戶邊界
	// （review round 2 Minor #4）。
	GetOpenInterruptForRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (repository.Interrupt, bool, error)
	// GetConversationIDForRequest 供 GetSnapshot 直接解析 conversationId（Task 15
	// write-path 修復 round 2 Minor #3）——不再疊在共用的 repository.GetSnapshot
	// 上，因為只有這一個呼叫端需要這個值，見 repository.GetConversationIDForRequest
	// 與 revision.go GetSnapshot 的文件。0, nil 表示目前查不到，不是 error。
	GetConversationIDForRequest(ctx context.Context, requestID int64) (int64, error)
	// ListLatestArtifactsByConversation 供 GetRuntimeSnapshot 組 artifacts[]（doc57 §5.6）。
	ListLatestArtifactsByConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID) ([]repository.RunArtifact, error)
}

// replayCursorExpiredStatus 把 REPLAY_CURSOR_EXPIRED 映射成 gRPC status（doc57
// §2.5-6：cursor 早於這個 run 目前可 replay 的最小 seq，等同 HTTP 語意上的
// 410 Gone——那段資料已經不在 retention window 內，重試同一個 cursor 永遠不會
// 成功）。gRPC 的標準 code 集合沒有 410 的直接對應；OutOfRange 是官方文件
// 建議的語意最接近者（"attempted past the valid range"），也刻意跟
// REPLAY_CURSOR_AHEAD 用的 Aborted 分開——呼叫端要能用 gRPC code 就分辨這是
// 「永遠不會好」還是「重新整理後可能會好」，不是只能靠 BusinessError.reason
// 才分得出來。Task 12 的 gateway 端點把這個 bucket 映回 HTTP 410 時需要在
// gateway/internal/procurement/wire.go 的 GRPCCodeToHTTPStatus 補上
// OutOfRange 這個 case（目前落 default 500）——見本 task 報告。
func replayCursorExpiredStatus(run repository.Run) error {
	return procurementStatus(codes.OutOfRange, codemeta.ProcurementErrorReasonReplayCursorExpired.Value,
		"游標已早於此 run 目前可 replay 的最小 seq（minAvailableSeq="+itoa64(run.MinAvailableSeq)+
			"），請改用 GetSnapshot／GetRun 重新取得最新狀態")
}

// replayCursorAheadStatus 把 REPLAY_CURSOR_AHEAD 映射成 gRPC status（doc57
// §2.5-6：cursor 高於目前 high-water mark，呼叫端手上的 cursor 比伺服器已知
// 的還新，通常代表呼叫端狀態跟伺服器不同步）。跟既有 STALE_REVISION／
// INVITE_ALREADY_RESOLVED／IDEMPOTENCY_KEY_CONFLICT 等「需要呼叫端重新同步
// 狀態再試」情境同一個 bucket（見 procurementUsecaseErrorStatus 與本檔其餘
// codes.Aborted 用法）。
func replayCursorAheadStatus(run repository.Run) error {
	return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonReplayCursorAhead.Value,
		"游標高於此 run 目前的 high-water mark（lastEventSeq="+itoa64(run.LastEventSeq)+"），請重新查詢最新狀態")
}

// WatchRunEvents 落地 doc57 §2.5 replay 演算法：驗證買家擁有權（random ID 與
// 他人 ID 回相同 404 外形，§2.5-1）、解析 cursor（§2.5-2）、判斷 cursor 是否
// 落在可 replay 的視窗內（§2.5-6），然後**先註冊 live wake channel 再重查一次**
// 封住 query-to-subscribe race（§2.5-4），之後每次收到 wake 都重新從 DB 撈
// （§2.5-5：wake 只是「可能有新資料」的提示，不是 SSOT）。
//
// disconnect（ctx.Done()）只 abort 這個 subscription，不改變 run 本身的狀態
// （§2.5-8）——這裡回 nil，讓 gRPC 層把它當成正常結束的 stream，不是錯誤。
func (s *AIProcurementGrpcServer) WatchRunEvents(req *aipb.WatchRunEventsRequest, stream aipb.AIProcurementService_WatchRunEventsServer) error {
	ctx := stream.Context()
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return err
	}

	runID, err := parseInt64(req.GetRunId())
	if err != nil {
		return procurementUsecaseErrorStatus(ctx, repository.ErrNotFound) // random ID 與他人 ID 回相同 404 外形（doc57 §2.5-1）
	}
	run, err := s.runStore.GetRun(ctx, runID, buyerID)
	if err != nil {
		return procurementUsecaseErrorStatus(ctx, err)
	}

	afterSeq := int64(0)
	if req.GetAfterSeq() != "" {
		afterSeq, err = parseInt64(req.GetAfterSeq())
		if err != nil {
			return procurementUsecaseErrorStatus(ctx, repository.ErrNotFound)
		}
	}
	if afterSeq > 0 && afterSeq < run.MinAvailableSeq {
		return replayCursorExpiredStatus(run)
	}
	if afterSeq > run.LastEventSeq {
		return replayCursorAheadStatus(run)
	}

	// doc57 §2.5-4：先註冊 LISTEN 再重查一次，封住 query-to-subscribe race。
	wake, cancel, err := s.runStore.WatchRun(ctx, runID)
	if err != nil {
		return err
	}
	defer cancel()

	// conversationID 從這裡的 run row 讀一次就固定：run 與 conversation 的關係
	// 在 run 建立時就寫定，這條訂閱期間不會變，所以不必每批事件重查。
	conversationID := run.ConversationID

	sent := afterSeq
	if sent, err = s.sendEventsAfter(stream, runID, conversationID, sent); err != nil {
		return err
	}
	// 這一次判斷服務的是「run 早就結束了，client 現在才連上來補歷史」：把 backlog
	// 送完就沒有下一筆可等，不該讓它進迴圈空等到 deadline。
	if s.subscriptionExhausted(ctx, runID, buyerID, sent) {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil // doc57 §2.5-8：disconnect 只 abort subscription，不改 run status
		case _, ok := <-wake:
			if !ok {
				return nil // watch 連線斷線，交給 client 重連（不同於 run 失敗）
			}
			if sent, err = s.sendEventsAfter(stream, runID, conversationID, sent); err != nil {
				return err
			}
			// 終態事件送出後主動收線，而不是留著連線等 client 或 gateway 的
			// deadline 把它回收。gateway 對「上游乾淨結束」已經有正確處理
			// （io.EOF 分支安靜關閉 SSE、不送 RUN_ERROR 等價 frame），所以
			// 這裡 return nil 就是整條鏈的修法，下游不需要任何改動。
			if s.subscriptionExhausted(ctx, runID, buyerID, sent) {
				return nil
			}
		}
	}
}

// subscriptionExhausted 回答「這條訂閱還有沒有下一筆事件可等」。
//
// 兩個條件都必須成立：run 已進終態，且游標已追上該 run 的最後一筆事件。
//
//   - 終態的判準用 domain.IsTerminalRunStatus，不在這裡重寫一份清單——那個函式
//     的註解明寫它存在的理由就是「避免各呼叫端各自維護一份終態清單而漂移」。
//     也因此**不能改用事件型別判斷**：CONFIRMATION_REQUIRED 在 graph_dispatch
//     裡被稱作 TerminalEventType，但它對應的 run 狀態不是終態（run 在等使用者
//     確認，之後還會有 resume 的事件），照事件型別關就會把 confirm→resume 那條
//     流提前收線。
//   - 追上游標這一半同樣是必要條件：終態事件本身是最後才寫進 ledger 的，存在
//     「status 已翻、事件還沒被這條訂閱送出」的窗口，那一瞬間關掉等於 client
//     少收終態 frame。
//
// 錯誤方向刻意 fail-open：重讀 run 失敗時回 false，訂閱維持原本行為（撐到
// gateway 的 deadline 才被回收）。提早收線是最佳化不是正確性閘門，為了一次
// 狀態重讀失敗而中斷一條健康的訂閱是把代價放錯地方。
func (s *AIProcurementGrpcServer) subscriptionExhausted(
	ctx context.Context, runID int64, buyerID identity.BuyerID, sent int64,
) bool {
	run, err := s.runStore.GetRun(ctx, runID, buyerID)
	if err != nil {
		pkglogger.Warn(ctx, "procurement: run status re-read failed; keeping the run-events subscription open",
			zap.Int64("run_id", runID), zap.Error(err))
		return false
	}
	return domain.IsTerminalRunStatus(run.Status) && sent >= run.LastEventSeq
}

// sendEventsAfter 是 doc57 §2.5-5：wake 只表示「可能有新資料」，每次仍從
// DB 撈；in-memory channel／wake 訊號本身不是 SSOT。回傳新的 sent 游標。
func (s *AIProcurementGrpcServer) sendEventsAfter(stream aipb.AIProcurementService_WatchRunEventsServer, runID, conversationID, afterSeq int64) (int64, error) {
	events, err := s.runStore.ListRunEventsAfterSeq(stream.Context(), runID, afterSeq)
	if err != nil {
		return afterSeq, err
	}
	sent := afterSeq
	for _, ev := range events {
		if err := stream.Send(&aipb.RunEvent{
			EventId: itoa64(ev.ID), RunId: itoa64(ev.RunID), Seq: itoa64(ev.Seq),
			Type: ev.EventType, OccurredAt: timeutil.FormatISO8601(ev.OccurredAt),
			DataJson: ev.PublicPayloadJSON, ConversationId: itoa64OrEmpty(conversationID),
		}); err != nil {
			return sent, err
		}
		sent = ev.Seq
	}
	return sent, nil
}

// itoa64OrEmpty 把零值轉成空字串而不是 "0"。conversation_id 的零值代表這個 run
// 沒有掛在任何對話上（`procurement_agent_runs.conversation_id` 可為 NULL，
// `toRunEntity` 用 int64Value 把 NULL 讀成 0），而 "0" 在 wire 上會被讀成
// 「id 為 0 的對話」——缺席與零值不是同一件事，同 Activity payload 的
// `count` 缺席 ≠ 0 的既有紀律。
func itoa64OrEmpty(v int64) string {
	if v == 0 {
		return ""
	}
	return itoa64(v)
}

func init() {
	registerMethodPolicy("/hawkr.ai.AIProcurementService/WatchRunEvents", MethodPolicy{Actor: ActorBuyer})
}
