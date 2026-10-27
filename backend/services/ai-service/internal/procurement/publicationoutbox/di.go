package publicationoutbox

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
)

// ErrPublisherUnavailable 是外送管道當下不可用（連不上 broker）。它是**可重試**的
// 錯誤：worker 會照 backoff 重排這一列，下一輪重新嘗試連線。
var ErrPublisherUnavailable = errors.New("rfq-publication-outbox: publisher unavailable")

const (
	// schedulerInterval 是 Worker.RunOnce 的節奏。1 分鐘，與既有 procurement
	// outbox 相同——收件人在等通知，這條線與它一樣對延遲敏感。
	schedulerInterval = 1 * time.Minute

	// retention 界住一列 done 存活多久才被 cleanup 刪掉。上線前量體低，30 天是
	// 保守預設值，不是調校過的數字（同 buyer_deleted_outbox）。
	retention = 30 * 24 * time.Hour
	// cleanupBatchLimit 界住單次 cleanup tick 刪幾列。
	cleanupBatchLimit = 500

	// publishConfirmTimeout 讓每次 broker confirm 遠低於 claimLease，卡住的
	// broker 才不會把整個 lease 吃完、擋掉重試接手。
	publishConfirmTimeout = 5 * time.Second
)

// Module 只提供本 package 自己的建構子與自己的兩個排程器；介面綁定
// （*repository.Repo 實作 InviteSentMarker）放 procurement/module.go——那個介面是
// 本層宣告的，若在這裡去 import repository 綁定它，依賴方向就反了。
var Module = fx.Module("procurement_publication_outbox",
	fx.Provide(NewRepository),
	fx.Provide(newMetrics),
	fx.Provide(newRawPublisher),
	fx.Provide(newPublishFunc),
	fx.Provide(NewWorker),
	fx.Invoke(startSchedulers),
)

// newMetrics 只 provide 一次，由 Worker（終態失敗計數）與 monitoring tick
// （backlog／staleness gauge）共用。共用是**必要**不只是整齊：outbox.NewMetrics
// 走 promauto，同一個 outbox_name 註冊兩次會在啟動時 panic。
func newMetrics() *outbox.Metrics {
	return outbox.NewMetrics(outboxName)
}

// rawConn 是 lazyRawPublisher 持有的連線，讓「連線失敗後會不會自己復原」這件事
// 測得起來（*mq.Publisher 結構上滿足它）。
type rawConn interface {
	PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error
	Close() error
}

// lazyRawPublisher 在**第一次真的要發**的時候才連 RabbitMQ，連不上就回錯讓那一列
// 重試，下一輪重新嘗試。
//
// ⛔ 刻意不是「DI 時就連、連不上回 nil、之後整個 tick 跳過」：那種寫法下，
// ai-service 滾動重啟剛好撞上 broker 短暫不可用，這條線就永久停擺——broker 兩分鐘
// 後恢復，隔壁 legacy quote-invite 線自己接回去，這條線卻要等到有人看到 backlog
// gauge 才去重啟 pod。復原能力必須在資料面，不能靠人。
type lazyRawPublisher struct {
	connect func() (rawConn, error)
	logger  *zap.Logger

	mu   sync.Mutex
	conn rawConn
}

func newLazyRawPublisher(url string, logger *zap.Logger) *lazyRawPublisher {
	return &lazyRawPublisher{
		logger: logger,
		connect: func() (rawConn, error) {
			// confirm timeout 遠低於 claimLease，卡住的 broker 才不會把整個 lease
			// 吃完、擋掉重試接手。
			publisher, err := mq.NewPublisher(url, mq.WithConfirmTimeout(publishConfirmTimeout))
			if err != nil {
				return nil, err
			}
			return publisher, nil
		},
	}
}

func (p *lazyRawPublisher) PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error {
	p.mu.Lock()
	if p.conn == nil {
		conn, err := p.connect()
		if err != nil {
			p.mu.Unlock()
			// 沒有把失敗記在任何地方 ⇒ 下一次呼叫會再連一次。這就是自癒。
			return fmt.Errorf("%w: connect RabbitMQ: %w", ErrPublisherUnavailable, err)
		}
		p.conn = conn
		p.logger.Info("rfq-publication-outbox: RabbitMQ publisher connected")
	}
	conn := p.conn
	p.mu.Unlock()

	// 連上之後的斷線由 *mq.Publisher 自己的 watchReconnect 處理。
	return conn.PublishRaw(ctx, exchange, routingKey, body)
}

func (p *lazyRawPublisher) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return nil
	}
	err := p.conn.Close()
	p.conn = nil
	return err
}

// newRawPublisher 回 nil **只在 URL 根本沒設定**時——那是 config 層的固定條件，不是
// broker 健康度，worker 會整個 tick 跳過（見 Worker.RunOnce），列留在 pending 而不
// 是被燒掉重試次數。broker 不可用是另一回事，交給 lazyRawPublisher 自己重試。
func newRawPublisher(cfg *config.Config, logger *zap.Logger, lc fx.Lifecycle) RawPublisher {
	if cfg.RabbitMQ.URL == "" {
		logger.Warn("rfq-publication-outbox: RabbitMQ URL not configured, worker will skip ticks")
		return nil
	}
	publisher := newLazyRawPublisher(cfg.RabbitMQ.URL, logger)
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error { return publisher.close() },
	})
	return publisher
}

func newPublishFunc(raw RawPublisher) PublishFunc {
	if raw == nil {
		return nil
	}
	return NewPublishFunc(raw)
}

// startSchedulers 掛上兩個 tick：排空與監測。
//
// 監測那一支是既有 internal/procurement/outbox **沒有**的東西（ADR D3：那份不
// 遷移），本 package 不重蹈：沒有 Metrics.Refresh 就沒有 backlog／staleness 可看，
// 沒有 CleanupDone 就沒有東西界住 done 列的成長。兩件事都便宜且冪等，所以與排空
// 共用同一個節奏常數，是刻意的簡單而不是疏漏（同 buyer-service 的同款註解）。
func startSchedulers(lc fx.Lifecycle, worker *Worker, repo *Repository, metrics *outbox.Metrics, logger *zap.Logger) {
	drain := outbox.NewScheduler(schedulerInterval, worker.RunOnce,
		func(_ context.Context, r any) {
			logger.Error("rfq-publication-outbox scheduler panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(_ context.Context, err error) {
			logger.Error("rfq-publication-outbox RunOnce failed", zap.Error(err))
		},
	)

	monitor := outbox.NewScheduler(schedulerInterval, func(ctx context.Context) error {
		if err := metrics.Refresh(ctx, repo, time.Now()); err != nil {
			return err
		}
		deleted, err := outbox.CleanupDone(ctx, repo, time.Now(), retention, cleanupBatchLimit)
		if err != nil {
			return err
		}
		if deleted > 0 {
			logger.Info("rfq-publication-outbox cleanup deleted done rows", zap.Int("deleted", deleted))
		}
		return nil
	},
		func(_ context.Context, r any) {
			logger.Error("rfq-publication-outbox monitoring panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		func(_ context.Context, err error) {
			logger.Warn("rfq-publication-outbox monitoring tick failed", zap.Error(err))
		},
	)

	lc.Append(fx.Hook{
		// ⛔ Start 收 context.Background() 而不是 OnStart 拿到的 ctx：後者是 fx
		// 自己的 start-timeout context，OnStart 一返回就被 cancel，迴圈會立刻
		// 無聲結束（pkg/outbox.Scheduler.Start 的註解）。
		OnStart: func(context.Context) error {
			drain.Start(context.Background())
			monitor.Start(context.Background())
			return nil
		},
		OnStop: func(context.Context) error {
			drain.Stop()
			monitor.Stop()
			return nil
		},
	})
}
