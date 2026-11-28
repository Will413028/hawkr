package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const healthCheckInterval = 15 * time.Minute

const healthCheckLease = 120 * time.Second

const healthCheckBatch = 50

const halfOpenProbesToClose = 3

const halfOpenAfter = 60 * time.Second

const providerUnavailableReverifyWindow = 24 * time.Hour

// HealthProbe 是 INV.hc 需要的出站面。刻意只要 ListTracks 一支——
// 介面越窄，「這支 worker 會打哪些 AMEGO 端點」在型別上就越說得清楚。
type HealthProbe interface {
	ListTracks(ctx context.Context, c amego.Credential, year, period int) ([]amego.Track, error)
}

// HealthAccountStore 是 INV.hc 需要的 repository 面。
type HealthAccountStore interface {
	FetchDueHealthChecks(ctx context.Context, now, unavailableSinceFloor time.Time, limit int32) ([]domain.ProviderAccount, error)
	ClaimHealthCheck(ctx context.Context, id int64, now, leaseUntil, unavailableSinceFloor time.Time) (bool, error)
	RecordHealthCheckResult(ctx context.Context, in domain.RecordHealthCheckResult) error
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// HealthCheck 是 INV.hc 的 usecase。
type HealthCheck struct {
	accounts HealthAccountStore
	probe    HealthProbe
	log      *zap.Logger
	now      func() time.Time
	batch    int32
}

// NewHealthCheck 建 INV.hc usecase。
func NewHealthCheck(accounts HealthAccountStore, probe HealthProbe, log *zap.Logger, now func() time.Time, batch int32) *HealthCheck {
	return &HealthCheck{accounts: accounts, probe: probe, log: orNop(log), now: now, batch: batch}
}

// RunOnce 跑一輪健康巡檢。
//
// 🔴 單一帳戶失敗不中斷整輪：一家的憑證壞掉正是要被記錄的事實，不該讓其餘帳戶
// 整輪不被檢查。
func (u *HealthCheck) RunOnce(ctx context.Context) error {
	now := u.now()
	due, err := u.accounts.FetchDueHealthChecks(ctx, now, now.Add(-providerUnavailableReverifyWindow), u.batch)
	if err != nil {
		return err
	}
	var checked, skipped, recovered, degraded int
	for _, acct := range due {
		at := u.now()
		ok, err := u.accounts.ClaimHealthCheck(ctx, acct.ID, at, at.Add(healthCheckLease),
			at.Add(-providerUnavailableReverifyWindow))
		if err != nil {
			u.log.Error("invoice.hc.claim_failed", zap.Int64("account_id", acct.ID), zap.Error(err))
			continue
		}
		if !ok {
			skipped++
			continue
		}
		res := u.checkOne(ctx, acct)
		if err := u.accounts.RecordHealthCheckResult(ctx, res.record); err != nil {
			u.log.Error("invoice.hc.record_failed", zap.Int64("account_id", acct.ID), zap.Error(err))
			continue
		}
		checked++
		if res.recovered {
			recovered++
		}
		if res.reopened {
			degraded++
		}
	}
	u.log.Info("invoice.hc.scanned",
		zap.Int("due", len(due)), zap.Int("checked", checked), zap.Int("skipped", skipped),
		zap.Int("recovered_to_active", recovered), zap.Int("breaker_reopened", degraded))
	return nil
}

type healthOutcome struct {
	record    domain.RecordHealthCheckResult
	recovered bool
	reopened  bool
}

func (u *HealthCheck) checkOne(ctx context.Context, acct domain.ProviderAccount) healthOutcome {
	at := u.now()
	year, period := domain.AmegoPeriod(at)

	var probeErr error
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		_, probeErr = u.probe.ListTracks(ctx,
			amego.Credential{IssuerID: acct.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			year, period)
		return nil
	}); err != nil {
		probeErr = err
	}

	return decideHealthOutcome(acct, at, probeErr)
}

func decideHealthOutcome(acct domain.ProviderAccount, at time.Time, probeErr error) healthOutcome {
	out := healthOutcome{record: domain.RecordHealthCheckResult{
		ID:                  acct.ID,
		CheckedAt:           at,
		NextCheckAt:         at.Add(healthCheckInterval),
		NextStatus:          acct.Status,
		StatusReason:        acct.StatusReason,
		BreakerOpenedAt:     acct.BreakerOpenedAt,
		BreakerProbeSuccess: acct.BreakerProbeSuccess,
	}}

	if acct.Status == codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value {
		if probeErr == nil {
			out.record.NextStatus = codemeta.InvoiceProviderAccountStatusActive.Value
			out.record.StatusReason = nil
			out.record.BreakerOpenedAt = nil
			out.record.BreakerProbeSuccess = 0
			out.recovered = true
			return out
		}
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		return out
	}

	if acct.Status != codemeta.InvoiceProviderAccountStatusDegraded.Value {
		if probeErr == nil {
			out.record.StatusReason = nil
			out.record.BreakerProbeSuccess = 0
			return out
		}
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		return out
	}

	inHalfOpen := acct.BreakerOpenedAt == nil || !at.Before(acct.BreakerOpenedAt.Add(halfOpenAfter))
	if !inHalfOpen {
		return out
	}
	if probeErr != nil {
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		out.record.BreakerProbeSuccess = 0
		opened := at
		out.record.BreakerOpenedAt = &opened
		out.reopened = true
		return out
	}
	next := acct.BreakerProbeSuccess + 1
	if int(next) >= halfOpenProbesToClose {
		out.record.NextStatus = codemeta.InvoiceProviderAccountStatusActive.Value
		out.record.StatusReason = nil
		out.record.BreakerOpenedAt = nil
		out.record.BreakerProbeSuccess = 0
		out.recovered = true
		return out
	}
	out.record.BreakerProbeSuccess = next
	return out
}
