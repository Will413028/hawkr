// Package worker 是 Layer 4.5：排程與搶佔。
//
// claim → 呼叫 usecase → 標記結果；把 ErrorClass 翻成 outbox 的重試／終結行為。
// 不含業務規則。
package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	outboxNameIssue    = "invoice_issue"
	outboxNameOutbox   = "invoice_outbox"
	outboxNameDeadline = "invoice_deadline"
)

const obDispatchInterval = 5 * time.Second

const obMonitorInterval = 30 * time.Second

const obParkInterval = 1 * time.Hour

const (
	dlScanInterval = 5 * time.Minute

	dlInitialDelay = 41 * time.Second

	dlFetchBatch = 20

	dlClaimLease = 5 * time.Minute

	dlMaxAttempts = 3
	dlBaseBackoff = 60 * time.Second
	dlMaxBackoff  = 10 * time.Minute

	dlParkInterval = 6 * time.Hour
)

// Module 是 Layer 4.5 的組裝。
//
// 兩條硬規則（§4.2.1）：
//   - 每個 outbox_name 的 NewMetrics 只能 provide 一次，重名即 boot 期 panic
//   - 排程一律走 lease claim，不用永久旗標（《多副本搶佔規範》規則⑧）
var Module = fx.Module("worker",
	fx.Provide(NewMetrics),
	fx.Provide(fx.Annotate(newIssueOutboxMetrics, fx.ResultTags(`name:"obs_issue"`))),
	fx.Provide(fx.Annotate(newOutboxOutboxMetrics, fx.ResultTags(`name:"obs_outbox"`))),
	fx.Provide(fx.Annotate(newDeadlineOutboxMetrics, fx.ResultTags(`name:"obs_deadline"`))),
	fx.Provide(fx.Annotate(newSellerAccountResolver, fx.As(new(SellerAccountResolver)))),
	fx.Provide(newIssueWorker),
	fx.Provide(newOutboxWorker),
	fx.Provide(fx.Annotate(newInboxReprocessor, fx.As(new(InboxOrderPaidReprocessor)))),
	fx.Provide(fx.Annotate(newRefundReprocessor, fx.As(new(InboxRefundReprocessor)))),
	fx.Provide(fx.Annotate(newOverdueComplaintSweeper, fx.As(new(OverdueComplaintSweeper)))),
	fx.Provide(fx.Annotate(newStatutoryDeadlineSweeper, fx.As(new(StatutoryDeadlineSweeper)))),
	fx.Provide(newDeadlineWorker),
	fx.Provide(fx.Annotate(newTrackObserver, fx.As(new(usecase.TrackObserver)))),
	fx.Provide(newTrackWorker),
	fx.Provide(newHealthCheckWorker),
	fx.Provide(newRecheckWorker),
	fx.Provide(fx.Annotate(newGovernanceObserver, fx.As(new(usecase.GovernanceObserver)))),
	fx.Provide(newGovernanceWorker),
	fx.Provide(newPDFWorker),
	fx.Invoke(startSchedulers),
)

func newIssueOutboxMetrics() *outbox.Metrics    { return outbox.NewMetrics(outboxNameIssue) }
func newOutboxOutboxMetrics() *outbox.Metrics   { return outbox.NewMetrics(outboxNameOutbox) }
func newDeadlineOutboxMetrics() *outbox.Metrics { return outbox.NewMetrics(outboxNameDeadline) }

func newInboxReprocessor(c *invoicemq.OrderPaidConsumer) InboxOrderPaidReprocessor { return c }

func newRefundReprocessor(c *invoicemq.RefundFinalizedConsumer) InboxRefundReprocessor { return c }

func newOverdueComplaintSweeper(c *usecase.Complaint) OverdueComplaintSweeper { return c }

func newStatutoryDeadlineSweeper(i *usecase.Issuance) StatutoryDeadlineSweeper { return i }

func newSellerAccountResolver(c *seller.Client) SellerAccountResolver { return c }

type issueWorkerDeps struct {
	fx.In

	Ops       domain.OperationRepository
	Docs      domain.DocumentRepository
	UC        *usecase.Issuance
	Void      *usecase.Voiding
	Allowance *usecase.Allowance
	Cfg       *config.Config
	Metrics   *Metrics
	Obs       *outbox.Metrics `name:"obs_issue"`
	Log       *zap.Logger
}

func newIssueWorker(d issueWorkerDeps) *IssueWorker {
	return NewIssueWorker(d.Ops, d.Docs, d.UC, d.Void, d.Allowance, outbox.Config{
		MaxAttempts: d.Cfg.Schedule.IssueMaxRetry,
		BaseBackoff: time.Duration(d.Cfg.Schedule.BackoffBaseSeconds) * time.Second,
		MaxBackoff:  time.Duration(d.Cfg.Schedule.BackoffMaxSeconds) * time.Second,
		FetchBatch:  d.Cfg.Schedule.IssueBatchSize,
		ClaimLease:  time.Duration(d.Cfg.Schedule.IssueLeaseSeconds) * time.Second,
	}, d.Metrics, d.Obs, d.Log)
}

type outboxWorkerDeps struct {
	fx.In

	Repo    domain.OutboxRepository
	Pub     *invoicemq.EventPublisher
	Sellers SellerAccountResolver
	Cfg     *config.Config
	Obs     *outbox.Metrics `name:"obs_outbox"`
	Log     *zap.Logger
}

func newOutboxWorker(d outboxWorkerDeps) *OutboxWorker {
	return NewOutboxWorker(d.Repo, d.Pub, d.Sellers, outbox.Config{
		MaxAttempts: 8,
		BaseBackoff: 5 * time.Second,
		MaxBackoff:  10 * time.Minute,
		FetchBatch:  100,
		ClaimLease:  outboxLeaseFor(d.Cfg),
	}, obParkInterval, d.Obs, d.Log)
}

func outboxLeaseFor(cfg *config.Config) time.Duration {
	confirm := time.Duration(cfg.MQ.PublishTimeoutSecond) * time.Second
	lease := confirm*2 + 10*time.Second
	if lease < 30*time.Second {
		return 30 * time.Second
	}
	return lease
}

type deadlineWorkerDeps struct {
	fx.In

	Inbox     domain.InboxRepository
	UC        InboxOrderPaidReprocessor
	Refund    InboxRefundReprocessor
	Overdue   OverdueComplaintSweeper
	Statutory StatutoryDeadlineSweeper
	Obs       *outbox.Metrics `name:"obs_deadline"`
	Log       *zap.Logger
}

func deadlineCfg() outbox.Config {
	return outbox.Config{
		MaxAttempts: dlMaxAttempts,
		BaseBackoff: dlBaseBackoff,
		MaxBackoff:  dlMaxBackoff,
		FetchBatch:  dlFetchBatch,
		ClaimLease:  dlClaimLease,
	}
}

func newDeadlineWorker(d deadlineWorkerDeps) *DeadlineWorker {
	return NewDeadlineWorker(d.Inbox, d.UC, d.Refund, deadlineCfg(), dlParkInterval, d.Overdue, d.Obs, d.Log).
		WithStatutorySweeper(d.Statutory)
}

func startSchedulers(
	lc fx.Lifecycle,
	cfg *config.Config,
	issue *IssueWorker,
	ob *OutboxWorker,
	dl *DeadlineWorker,
	trk *trackWorker,
	hc *healthCheckWorker,
	rc *recheckWorker,
	gov *governanceWorker,
	pdf *pdfWorker,
	metrics *Metrics,
	log *zap.Logger,
) {
	l := orNopLogger(log)

	metrics.AmegoRateLimitQPS.Set(float64(cfg.Amego.RateLimitQPS))

	issueInterval := time.Duration(cfg.Schedule.IssuePollIntervalSeconds) * time.Second
	metrics.WorkerTickInterval.WithLabelValues(workerNameIssue).Set(issueInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameOutbox).Set(obDispatchInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameDeadline).Set(dlScanInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameTrack).Set(trackTick.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameHealth).Set(hcInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameRecheck).Set(rcInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameGov).Set((24 * time.Hour).Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNamePDF).Set(pdfInterval.Seconds())
	registerGovernanceMetrics()
	registerTrackMetrics()

	schedulers := []*outbox.Scheduler{
		outbox.NewScheduler(issueInterval,
			timedTick(metrics, workerNameIssue, issue.RunOnce),
			panicHook(l, "INV.op"), errorHook(l, "INV.op")),
		outbox.NewScheduler(obDispatchInterval,
			timedTick(metrics, workerNameOutbox, ob.RunOnce),
			panicHook(l, "INV.ob"), errorHook(l, "INV.ob")),
		outbox.NewScheduler(obMonitorInterval,
			ob.RefreshMetrics,
			panicHook(l, "INV.ob.monitor"), errorHook(l, "INV.ob.monitor")),
		outbox.NewScheduler(dlScanInterval,
			timedTick(metrics, workerNameDeadline, dl.RunOnce),
			panicHook(l, "INV.dl"), errorHook(l, "INV.dl"),
			outbox.WithInitialDelay(dlInitialDelay)),
		outbox.NewScheduler(trackTick,
			timedTick(metrics, workerNameTrack, trk.RunOnce),
			panicHook(l, "INV.trk"), errorHook(l, "INV.trk")),
		outbox.NewScheduler(hcInterval,
			timedTick(metrics, workerNameHealth, hc.RunOnce),
			panicHook(l, "INV.hc"), errorHook(l, "INV.hc")),
		outbox.NewScheduler(rcInterval,
			timedTick(metrics, workerNameRecheck, rc.RunOnce),
			panicHook(l, "INV.rc"), errorHook(l, "INV.rc")),
		outbox.NewScheduler(pdfInterval,
			timedTick(metrics, workerNamePDF, pdf.RunOnce),
			panicHook(l, "INV.pdf"), errorHook(l, "INV.pdf")),
	}

	govSched := outbox.NewDailyScheduler(govDailyHourUTC, govDailyMinuteUTC, "INV.gov", gov.RunDaily, l)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				ctx := context.Background()
				for attempt := 1; attempt <= govCatchUpAttempts; attempt++ {
					if err := gov.RunOnce(ctx); err == nil {
						return
					} else if attempt == govCatchUpAttempts {
						l.Error("invoice.gov.catchup_gave_up",
							zap.Int("attempts", attempt), zap.Error(err))
						return
					}
					select {
					case <-time.After(govCatchUpInterval):
					case <-ctx.Done():
						return
					}
				}
			}()
			govSched.Start(context.Background())
			for _, s := range schedulers {
				s.Start(context.Background())
			}
			l.Info("invoice.worker.started",
				zap.Duration("issue_interval", issueInterval),
				zap.Duration("outbox_interval", obDispatchInterval),
				zap.Duration("deadline_interval", dlScanInterval))
			return nil
		},
		OnStop: func(context.Context) error {
			for _, s := range schedulers {
				s.Stop()
			}
			return nil
		},
	})
}

func timedTick(m *Metrics, worker string, run func(context.Context) error) func(context.Context) error {
	obs := m.WorkerTickDuration.WithLabelValues(worker)
	return func(ctx context.Context) error {
		start := time.Now()
		err := run(ctx)
		obs.Observe(time.Since(start).Seconds())
		return err
	}
}

func panicHook(l *zap.Logger, name string) func(context.Context, any) {
	return func(_ context.Context, recovered any) {
		l.Error("invoice.worker.tick_panic", zap.String("worker", name), zap.Any("recovered", recovered))
	}
}

func errorHook(l *zap.Logger, name string) func(context.Context, error) {
	return func(_ context.Context, err error) {
		l.Error("invoice.worker.tick_error", zap.String("worker", name), zap.Error(err))
	}
}

func newTrackObserver() trackObserver { return trackObserver{} }

func newTrackWorker(uc *usecase.Track, cfg *config.Config) *trackWorker {
	return &trackWorker{uc: uc, baseHourUTC: cfg.Schedule.TrackBalanceCheckHourUTC, now: time.Now}
}

func newHealthCheckWorker(uc *usecase.HealthCheck) *healthCheckWorker {
	return &healthCheckWorker{uc: uc}
}

func newRecheckWorker(uc *usecase.Recheck) *recheckWorker { return &recheckWorker{uc: uc} }

func newGovernanceObserver() governanceObserver { return governanceObserver{} }

func newGovernanceWorker(uc *usecase.Governance) *governanceWorker { return &governanceWorker{uc: uc} }

func newPDFWorker(uc *usecase.PDF) *pdfWorker { return &pdfWorker{uc: uc} }
