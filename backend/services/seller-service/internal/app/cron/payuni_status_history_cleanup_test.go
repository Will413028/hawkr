package cron_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Will413028/hawkr/seller-service/internal/app/cron"
)

type fakePayuniStatusHistoryRepo struct {
	deleteCalledWith time.Time
	deleteCount      int64
	deleteErr        error
}

func (r *fakePayuniStatusHistoryRepo) DeleteStatusHistoryOlderThan(_ context.Context, before time.Time) (int64, error) {
	r.deleteCalledWith = before
	return r.deleteCount, r.deleteErr
}

func TestPayuniStatusHistoryCleanup_DeletesOlderThanRetention(t *testing.T) {
	repo := &fakePayuniStatusHistoryRepo{deleteCount: 42}
	job := cron.NewPayuniStatusHistoryCleanupJob(repo, zap.NewNop())

	job.Run(context.Background())

	wantCutoff := time.Now().AddDate(0, 0, -180)
	diff := repo.deleteCalledWith.Sub(wantCutoff)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("cutoff = %v, want ~180 days before now (diff %v)", repo.deleteCalledWith, diff)
	}
}

func TestPayuniStatusHistoryCleanup_LogsOnDeleteError(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	repo := &fakePayuniStatusHistoryRepo{deleteErr: errors.New("db unavailable")}
	job := cron.NewPayuniStatusHistoryCleanupJob(repo, zap.New(core))

	job.Run(context.Background())

	if logs.Len() != 1 {
		t.Fatalf("expected 1 error log entry when delete fails, got %d", logs.Len())
	}
}
