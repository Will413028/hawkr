package procurementrelay

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

type recordingJobs struct {
	args []river.JobArgs
	fail error
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if q.fail != nil {
		return q.fail
	}
	q.args = append(q.args, args)
	return nil
}

type fakeInvites struct {
	sent, failed []uuid.UUID
	markErr      error
	ctxErr       error
	stale        []uuid.UUID
	staleBefore  time.Time
}

func (f *fakeInvites) MarkInviteSent(ctx context.Context, id uuid.UUID) error {
	f.ctxErr = ctx.Err()
	if f.markErr != nil {
		return f.markErr
	}
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeInvites) MarkInviteFailed(ctx context.Context, id uuid.UUID) error {
	f.ctxErr = ctx.Err()
	if f.markErr != nil {
		return f.markErr
	}
	f.failed = append(f.failed, id)
	return nil
}

func (f *fakeInvites) StaleQueuedInvites(_ context.Context, before time.Time, _ int32) ([]uuid.UUID, error) {
	f.staleBefore = before
	return f.stale, nil
}

type fakeLive map[uuid.UUID]bool

func (f fakeLive) HasLiveJob(_ context.Context, id uuid.UUID) (bool, error) { return f[id], nil }

type fakeSender struct {
	sent []rabbitmq.PublishArgs
	err  error
}

func (f *fakeSender) Send(_ context.Context, a rabbitmq.PublishArgs) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, a)
	return nil
}

var (
	publicationID = uuid.MustParse("0192a000-0000-7000-8000-000000000001")
	inviteA       = uuid.MustParse("0192a000-0000-7000-8000-0000000000a1")
	inviteB       = uuid.MustParse("0192a000-0000-7000-8000-0000000000b2")
)

func args() InviteCreatedArgs {
	return InviteCreatedArgs{InviteID: inviteA, PublicationID: publicationID, StoreID: 7496483081637855232}
}

func TestOnRfqPublishedEnqueuesOneJobPerInvite(t *testing.T) {
	q := &recordingJobs{}
	err := OnRfqPublished(q)(t.Context(), procurement.RfqPublished{
		PublicationID: publicationID,
		Invites:       []procurement.RfqInvite{{InviteID: inviteA, StoreID: 11}, {InviteID: inviteB, StoreID: 22}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []river.JobArgs{
		InviteCreatedArgs{InviteID: inviteA, PublicationID: publicationID, StoreID: 11},
		InviteCreatedArgs{InviteID: inviteB, PublicationID: publicationID, StoreID: 22},
	}
	if len(q.args) != 2 || q.args[0] != want[0] || q.args[1] != want[1] {
		t.Fatalf("jobs = %v，預期 %v", q.args, want)
	}
}

func TestOnRfqPublishedReturnsTheEnqueueError(t *testing.T) {
	boom := errors.New("boom")
	err := OnRfqPublished(&recordingJobs{fail: boom})(t.Context(), procurement.RfqPublished{Invites: []procurement.RfqInvite{{InviteID: inviteA, StoreID: 1}}})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v，排入失敗要讓發布回滾", err)
	}
}

func TestSuccessfulSendMarksTheInviteSent(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{}
	if err := NewWorker(invites, out, nil).inviteCreated(t.Context(), args(), false); err != nil {
		t.Fatal(err)
	}
	if len(out.sent) != 1 || out.sent[0].Exchange != "procurement.events" || out.sent[0].RoutingKey != "procurement.rfq_publication_invite.created" {
		t.Fatalf("sent = %+v", out.sent)
	}
	if len(invites.sent) != 1 || invites.sent[0] != inviteA || len(invites.failed) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestFailedSendBeforeTheLastAttemptMarksNothing(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	if err := NewWorker(invites, out, nil).inviteCreated(t.Context(), args(), false); err == nil {
		t.Fatal("送出失敗要回錯誤讓 River 重試")
	}
	if len(invites.sent)+len(invites.failed) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestFailedSendOnTheLastAttemptMarksTheInviteFailed(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	if err := NewWorker(invites, out, nil).inviteCreated(t.Context(), args(), true); err == nil {
		t.Fatal("用盡仍要回送出的錯誤")
	}
	if len(invites.failed) != 1 || invites.failed[0] != inviteA || len(invites.sent) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestPayloadIsExactlyThreeStringKeys(t *testing.T) {
	body, err := payload(args())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"inviteId": inviteA.String(), "publicationId": publicationID.String(), "storeId": "7496483081637855232"}
	if len(got) != len(want) {
		t.Fatalf("payload = %s", body)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("payload[%s] = %v，預期 %v", k, got[k], v)
		}
	}
}

func job(attempt, max int) *river.Job[InviteCreatedArgs] {
	return &river.Job[InviteCreatedArgs]{JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: max}, Args: args()}
}

func TestWorkTreatsAttemptEqualToMaxAttemptsAsTheLast(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	w := &typedWorker{w: NewWorker(invites, out, nil)}
	if err := w.Work(t.Context(), job(maxAttempts, maxAttempts)); err == nil {
		t.Fatal("用盡仍要回送出的錯誤")
	}
	if len(invites.failed) != 1 {
		t.Fatalf("Attempt == MaxAttempts 要標 FAILED，failed=%v", invites.failed)
	}
}

func TestWorkBeforeMaxAttemptsMarksNothing(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	w := &typedWorker{w: NewWorker(invites, out, nil)}
	if err := w.Work(t.Context(), job(maxAttempts-1, maxAttempts)); err == nil {
		t.Fatal("送出失敗要回錯誤")
	}
	if len(invites.failed)+len(invites.sent) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestStateWriteFailureOnTheLastAttemptSnoozesInsteadOfDiscarding(t *testing.T) {
	for name, tc := range map[string]struct {
		sendErr error
	}{"sent": {nil}, "exhausted": {errors.New("broker down")}} {
		t.Run(name, func(t *testing.T) {
			invites, out := &fakeInvites{markErr: errors.New("db down")}, &fakeSender{err: tc.sendErr}
			err := NewWorker(invites, out, nil).inviteCreated(t.Context(), args(), true)
			var snooze *rivertype.JobSnoozeError
			if !errors.As(err, &snooze) {
				t.Fatalf("err = %v，最後一次嘗試狀態寫入失敗要延後重跑而不是丟棄", err)
			}
		})
	}
}

func TestStateWriteFailureBeforeTheLastAttemptRetries(t *testing.T) {
	invites := &fakeInvites{markErr: errors.New("db down")}
	err := NewWorker(invites, &fakeSender{}, nil).inviteCreated(t.Context(), args(), false)
	var snooze *rivertype.JobSnoozeError
	if err == nil || errors.As(err, &snooze) {
		t.Fatalf("err = %v，非最後一次要回一般錯誤走重試", err)
	}
}

func TestStateIsWrittenEvenWhenTheSendContextIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	invites := &fakeInvites{}
	if err := NewWorker(invites, &fakeSender{}, nil).inviteCreated(ctx, args(), false); err != nil {
		t.Fatal(err)
	}
	if invites.ctxErr != nil || len(invites.sent) != 1 {
		t.Fatalf("狀態寫入的 ctx 不得受送出 ctx 影響：ctxErr=%v sent=%v", invites.ctxErr, invites.sent)
	}
}

func TestRetryScheduleMatchesTheLegacyOutbox(t *testing.T) {
	want := []time.Duration{2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute}
	for i, w := range want {
		if got := retryDelay(i + 1); got != w {
			t.Fatalf("retryDelay(%d) = %v，預期 %v", i+1, got, w)
		}
	}
	if got := retryDelay(30); got != time.Hour {
		t.Fatalf("retryDelay(30) = %v，預期上限 1h", got)
	}
	if maxAttempts != 5 {
		t.Fatalf("maxAttempts = %d，舊 outbox 是 5", maxAttempts)
	}
	if got, min := staleAfter(), 30*time.Minute+maxAttempts*sendTimeout; got <= min {
		t.Fatalf("staleAfter = %v，必須超過重試期間 %v", got, min)
	}
}

func TestSweepMarksOnlyStaleInvitesWithoutALiveJob(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	invites := &fakeInvites{stale: []uuid.UUID{inviteA, inviteB}}
	w := NewWorker(invites, nil, fakeLive{inviteB: true})
	w.Now = func() time.Time { return now }
	n, err := w.Sweep(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(invites.failed) != 1 || invites.failed[0] != inviteA {
		t.Fatalf("n=%d failed=%v，只該標沒有進行中 job 的 A", n, invites.failed)
	}
	if want := now.Add(-staleAfter()); !invites.staleBefore.Equal(want) {
		t.Fatalf("before = %v，預期 %v", invites.staleBefore, want)
	}
}
