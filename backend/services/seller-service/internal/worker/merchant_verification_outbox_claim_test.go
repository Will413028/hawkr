package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

type fakeMerchantVerificationOutboxRepo struct {
	pending    []*domain.MerchantVerificationOutbox
	ClaimLost  bool
	ClaimErr   error
	ClaimCalls []struct {
		ID                          int64
		LeaseUntil, ClaimableBefore time.Time
	}
	MarkDoneCalls      []int64
	UpdateAttemptCalls []int64
}

func (r *fakeMerchantVerificationOutboxRepo) Insert(ctx context.Context, row *domain.MerchantVerificationOutbox) error {
	return nil
}
func (r *fakeMerchantVerificationOutboxRepo) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*domain.MerchantVerificationOutbox, error) {
	return r.pending, nil
}
func (r *fakeMerchantVerificationOutboxRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	r.ClaimCalls = append(r.ClaimCalls, struct {
		ID                          int64
		LeaseUntil, ClaimableBefore time.Time
	}{id, leaseUntil, claimableBefore})
	if r.ClaimErr != nil {
		return 0, r.ClaimErr
	}
	if r.ClaimLost {
		return 0, nil
	}
	return 1, nil
}
func (r *fakeMerchantVerificationOutboxRepo) UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error {
	r.UpdateAttemptCalls = append(r.UpdateAttemptCalls, id)
	return nil
}
func (r *fakeMerchantVerificationOutboxRepo) MarkDone(ctx context.Context, id int64, now time.Time) error {
	r.MarkDoneCalls = append(r.MarkDoneCalls, id)
	return nil
}
func (r *fakeMerchantVerificationOutboxRepo) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	return nil
}
func (r *fakeMerchantVerificationOutboxRepo) CountPending(ctx context.Context) (int, error) {
	return len(r.pending), nil
}
func (r *fakeMerchantVerificationOutboxRepo) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (r *fakeMerchantVerificationOutboxRepo) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return 0, nil
}

type fakeMerchantVerificationPublisher struct {
	published []domain.MerchantVerificationCompletedEvent
	err       error
}

func (p *fakeMerchantVerificationPublisher) PublishVerificationCompleted(ctx context.Context, event domain.MerchantVerificationCompletedEvent) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, event)
	return nil
}

func newPendingVerificationRow(id, accountID int64) *domain.MerchantVerificationOutbox {
	return &domain.MerchantVerificationOutbox{
		ID: id, AccountID: accountID, EventType: "approved",
		Payload: []byte(`{"event_id":"1","status":"approved","merchant_id":100}`),
		Status:  domain.MerchantVerificationOutboxPending,
	}
}

func TestMerchantVerificationOutboxWorker_ClaimLost_DoesNotPublish(t *testing.T) {
	repo := &fakeMerchantVerificationOutboxRepo{pending: []*domain.MerchantVerificationOutbox{newPendingVerificationRow(1, 100)}, ClaimLost: true}
	pub := &fakeMerchantVerificationPublisher{}
	w := NewMerchantVerificationOutboxWorker(repo, pub, testMerchantVerificationOutboxMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.ClaimCalls) != 1 {
		t.Fatalf("ClaimCalls = %+v, want exactly one", repo.ClaimCalls)
	}
	if len(pub.published) != 0 {
		t.Fatalf("claim lost but publish happened: %+v", pub.published)
	}
}

func TestMerchantVerificationOutboxWorker_PublishSuccess_MarksDone(t *testing.T) {
	repo := &fakeMerchantVerificationOutboxRepo{pending: []*domain.MerchantVerificationOutbox{newPendingVerificationRow(1, 100)}}
	pub := &fakeMerchantVerificationPublisher{}
	w := NewMerchantVerificationOutboxWorker(repo, pub, testMerchantVerificationOutboxMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("published = %+v, want exactly one event", pub.published)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 1 {
		t.Fatalf("MarkDoneCalls = %v, want [1]", repo.MarkDoneCalls)
	}
}

func TestMerchantVerificationOutboxWorker_PublishError_SchedulesRetry(t *testing.T) {
	repo := &fakeMerchantVerificationOutboxRepo{pending: []*domain.MerchantVerificationOutbox{newPendingVerificationRow(1, 100)}}
	pub := &fakeMerchantVerificationPublisher{err: errors.New("broker unavailable")}
	w := NewMerchantVerificationOutboxWorker(repo, pub, testMerchantVerificationOutboxMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.MarkDoneCalls) != 0 {
		t.Fatalf("MarkDoneCalls = %v, want none", repo.MarkDoneCalls)
	}
	if len(repo.UpdateAttemptCalls) != 1 {
		t.Fatalf("UpdateAttemptCalls = %v, want exactly one retry scheduled", repo.UpdateAttemptCalls)
	}
}

// testMerchantVerificationOutboxMetrics mirrors testOutboxMetrics in
// seller_deleted_outbox_worker_test.go (var testOutboxMetrics =
// outbox.NewMetrics("seller_deleted_test")) — same package (worker), own
// outboxName so this doesn't share Prometheus series with that unrelated
// outbox's test metrics.
var testMerchantVerificationOutboxMetrics = outbox.NewMetrics("merchant_verification_test")
