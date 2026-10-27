package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// runCancelCheckInterval 是取消觀察器兩次 DB 查詢之間的最短間隔。
//
// 為什麼要節流：唯一的呼叫點是 graph turn 的事件迴圈，而那個迴圈每收到一個
// ADK event 就轉一圈——一次 turn 可能有數十到上百個事件（step 敘述、activity
// 事件、function call）。不節流等於把「有沒有人按停止」問成一個 per-event 的
// 查詢，對一個極少發生的操作付出與事件流同階的 DB 成本。
//
// 為什麼是 1 秒：取消的對端是一個正在盯著畫面等的人，而 doc57 §5.4 的
// disposition 值域本來就承認 TOO_LATE（側效已 commit 時不得謊報取消成功）
// ⇒ 這條路徑要的是「儘快停」，不是「精確在第 N 個事件停」。1 秒把最壞情況的
// 額外等待壓在人感覺得到的範圍內，同時讓一次 turn 的取消查詢降到個位數。
const runCancelCheckInterval = time.Second

// runCancelStatusReader 是取消觀察器唯一需要的能力：把一個 run 的當前狀態讀
// 回來。刻意寫成獨立的窄介面而不是直接吃 RunClaimStore——觀察器只讀不寫，讓
// 它的型別自己講清楚這件事，測試也只需要準備一個方法。
type runCancelStatusReader interface {
	GetRun(ctx context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error)
}

// runCancelObserver 回答背景執行路徑唯一關心的那個問題：「買家是不是已經按了
// 停止？」
//
// 🔴 **這個型別存在的理由**：CancelRun 把 run 轉成 CANCEL_REQUESTED 之後，在
// 它出現之前**沒有任何執行路徑會讀那個狀態**——命令是唯寫的，寫入點只有
// repository/run.go，讀取點只有 CancelRun 自己用來分類 disposition。淨結果是
// 停止鈕按下去安靜地什麼都沒發生：搜尋照跑完、結果照出現、終態照樣寫
// SUCCEEDED。CANCELLED 這個終態因此不可達（domain.RunStatusCancelled 在全
// repo 只有定義沒有寫入者）。
//
// 觀察器不自己做任何狀態轉移——它只回答問題，終態由呼叫端走既有的
// TerminalizeRun 寫入，確保取消與成功、失敗共用同一條終態路徑與同一組守衛。
type runCancelObserver struct {
	store   runCancelStatusReader
	runID   int64
	buyerID identity.BuyerID

	// interval 與 now 抽成欄位是為了讓測試注入，不靠 sleep 驗節流行為
	// （CLAUDE.md §21：時間敏感邏輯注入 clock，不用 sleep）。
	interval time.Duration
	now      func() time.Time

	// nextCheckAt 是下一次允許查 DB 的時刻。零值代表「還沒查過」，所以第一次
	// 呼叫一定會真的查一次——買家在 turn 剛開始就按停止是完全正常的操作，把
	// 第一次也節流掉會讓最快的那個取消反而最慢生效。
	nextCheckAt time.Time

	// observed 記住已經看到取消，之後不再查 DB。CANCEL_REQUESTED 對執行中的
	// run 是單向的（只會往終態走，不會回到 RUNNING），所以看到一次就夠。
	observed bool
}

func newRunCancelObserver(store runCancelStatusReader, runID int64, buyerID identity.BuyerID) *runCancelObserver {
	return &runCancelObserver{
		store: store, runID: runID, buyerID: buyerID,
		interval: runCancelCheckInterval, now: time.Now,
	}
}

// cancelRequested 回報這個 run 是否已被要求取消。
//
// ⚠️ **查詢失敗一律回 false**，而且不往上拋。理由是這個方法的失敗模式必須偏向
// 「繼續跑完」而不是「中止」：讀不到狀態時中止，等於讓一次暫時性的 DB 抖動
// 把買家正在等的結果丟掉，而那個結果本來會成功——那是把一個可恢復的基礎設施
// 問題升級成資料損失。反過來漏掉一次取消的代價只是這一輪跑完（買家再按一次，
// 或下一次檢查就會看到），side effect 一樣受 doc57 §5.4 的 TOO_LATE 語意保護。
func (o *runCancelObserver) cancelRequested(ctx context.Context) bool {
	if o == nil {
		return false
	}
	if o.observed {
		return true
	}
	now := o.now()
	if now.Before(o.nextCheckAt) {
		return false
	}
	o.nextCheckAt = now.Add(o.interval)

	run, err := o.store.GetRun(ctx, o.runID, o.buyerID)
	if err != nil {
		return false
	}
	if run.Status == domain.RunStatusCancelReq() {
		o.observed = true
		return true
	}
	return false
}
