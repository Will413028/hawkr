package worker

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

var Module = fx.Module("worker",
	fx.Invoke(startConsumer),
	fx.Invoke(startReconciler),
	fx.Invoke(startPostSchedulePublisher),
)

// startPostSchedulePublisher 掛上排程發布 worker（56 §7.6）。
//
// 沒有 enable 開關：這支的候選述詞是 schedule_state = 'pending'，而只有代行路徑寫得出
// 這個值 ⇒ 代行未開通時它每個 tick 撈到空集合，什麼都不做。多一個旗標只會多一個
// 「開了代行卻忘了開 worker」的失敗模式，而那個失敗是靜默的（貼文排了永遠不發）。
func startPostSchedulePublisher(
	lc fx.Lifecycle,
	cfg *config.Config,
	repo domain.ScheduledPostRepository,
	hashtags *usecase.HashtagUseCase,
	audit usecase.DelegationAuditClient,
	reval *revalidate.Client,
	logger *zap.Logger,
) {
	publisher := newPostSchedulePublisher(cfg.ScheduledPostPublish, repo, hashtags, audit, reval, logger)

	// 走 outbox.Scheduler 而非自己的 ticker 迴圈。差別不是風格：裸迴圈裡 tick 一旦
	// panic，goroutine 就地死掉，之後每一則排程貼文永不發布，而且**沒有任何訊號**
	// —— 服務仍然健康、log 只有那一次 stack、逾期告警要等 24 小時窗口過完才響。
	// Scheduler 的 onPanic 讓它記錄後繼續下一個 tick。
	s := outbox.NewScheduler(
		time.Duration(cfg.ScheduledPostPublish.PollIntervalSeconds)*time.Second,
		func(ctx context.Context) error {
			publisher.tick(ctx)
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("scheduled post publisher panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil, // tick 自己吞掉並記錄每一列的錯誤；沒有需要往上拋的
	)

	lc.Append(fx.Hook{
		// 傳 context.Background() 而非 OnStart 收到的 ctx —— 後者是 fx 自己的
		// start-timeout context，OnStart 一返回就被取消（見 pkg/outbox/scheduler.go）。
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop: func(_ context.Context) error {
			// 停止只讓迴圈在下一個判斷點退出；進行中的那一列不中斷，它的租約到期後
			// 會自己回到候選集合。刻意不做「關閉時守衛式釋放租約」——那需要知道副作用
			// 走到哪一步，而終寫 CAS 已經讓重跑是安全的。
			s.Stop()
			return nil
		},
	})
}

func startReconciler(lc fx.Lifecycle, postRepo domain.PostRepository, commentRepo domain.CommentRepository, logger *zap.Logger) {
	s := newCounterReconcileScheduler(counterReconcileInterval, postRepo, commentRepo, logger)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(context.Background()); return nil },
		OnStop:  func(_ context.Context) error { s.Stop(); return nil },
	})
}

func startConsumer(lc fx.Lifecycle, cfg *config.Config, commentRepo domain.CommentRepository, forumRepo domain.ForumRepository, postRepo domain.PostRepository, mediaRef usecase.MediaClient, logger *zap.Logger) {
	if cfg.RabbitMQ.URL == "" {
		logger.Warn("RabbitMQ URL not configured, skipping consumer")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	consumer := mq.NewResilientConsumer(cfg.RabbitMQ.URL, subscriptions(commentRepo, forumRepo, postRepo, mediaRef, logger))

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go consumer.Run(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}

func subscriptions(commentRepo domain.CommentRepository, forumRepo domain.ForumRepository, postRepo domain.PostRepository, mediaRef usecase.MediaClient, logger *zap.Logger) []mq.Subscription {
	return []mq.Subscription{
		{
			Exchange:    mq.ExchangeBuyer,
			Queue:       mq.QueueSocialBuyerDeleted,
			RoutingKeys: []string{mq.RoutingKeyBuyerDeleted},
			Handler:     newBuyerDeletedHandler(postRepo, commentRepo, forumRepo, logger),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeSeller,
			Queue:       mq.QueueSocialSellerDeleted,
			RoutingKeys: []string{mq.RoutingKeySellerDeleted},
			Handler:     newSellerDeletedHandler(postRepo, commentRepo, mediaRef, logger),
			MaxRetries:  3,
		},
		{
			Exchange:    mq.ExchangeContent,
			Queue:       mq.QueueSocialCounterDelta,
			RoutingKeys: []string{mq.RoutingKeyCounterDelta},
			Handler:     newCounterDeltaHandler(postRepo, commentRepo, logger),
			MaxRetries:  3,
		},
	}
}
