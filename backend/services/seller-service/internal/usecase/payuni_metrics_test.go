package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestSyncPendingStatuses_CountsAPIErrorsAndDoesNotPersist(t *testing.T) {
	before := testutil.ToFloat64(payuniSyncAPIErrorTotal)
	updateCalled := false

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			return true, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updateCalled = true
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return nil, errors.New("payuni api status 503: unavailable")
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("一筆失敗不該讓整個 tick 回錯（其餘的還要繼續）: %v", err)
	}

	if updateCalled {
		t.Error("PAYUNi 呼叫失敗時不得寫回註冊列；寫回會讓失敗看起來像一次成功的觀測")
	}
	if got := testutil.ToFloat64(payuniSyncAPIErrorTotal) - before; got != 1 {
		t.Errorf("payuni_sync_api_error_total 增加 %v, want 1", got)
	}
}

func TestApplyObservation_RecordsSourceAndChanged(t *testing.T) {
	sourceOf := func(source, changed string) float64 {
		return testutil.ToFloat64(payuniStatusObservationTotal.WithLabelValues(source, changed))
	}
	beforeChanged := sourceOf(sourceSync, "true")
	beforeUnchanged := sourceOf(sourceSync, "false")

	uc := &PayuniUseCase{payuniRepo: &mockPayuniRepo{}, payuniClient: &mockPayuniClient{}}

	info := &domain.PayuniRegistration{
		ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
	}
	obs := domain.PayuniStatusObservation{
		MerStatus: "9", Source: sourceSync, ObservedAt: time.Now().UTC(),
		Raw: json.RawMessage(`{}`),
	}

	uc.ApplyObservation(context.Background(), info, obs)
	if got := sourceOf(sourceSync, "true") - beforeChanged; got != 1 {
		t.Errorf("changed=true 計數增加 %v, want 1", got)
	}

	uc.ApplyObservation(context.Background(), info, obs)
	if got := sourceOf(sourceSync, "false") - beforeUnchanged; got != 1 {
		t.Errorf("changed=false 計數增加 %v, want 1", got)
	}
}
