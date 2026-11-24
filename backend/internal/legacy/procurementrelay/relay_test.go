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
	stale        []procurement.StaleInvite
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

func afterCursor(inv, after procurement.StaleInvite) bool {
	return inv.CreatedAt.After(after.CreatedAt) || inv.CreatedAt.Equal(after.CreatedAt) && inv.ID.String() > after.ID.String()
}

func (f *fakeInvites) StaleQueuedInvites(_ context.Context, before time.Time, after procurement.StaleInvite, limit int32) ([]procurement.StaleInvite, error) {
	f.staleBefore = before
	var out []procurement.StaleInvite
	for _, inv := range f.stale {
		if len(out) == int(limit) {
			break
		}
		if after != (procurement.StaleInvite{}) && !afterCursor(inv, after) {
			continue
		}
		out = append(out, inv)
	}
	return out, nil
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
	if err := NewWorker(invites, out, nil, &recordingJobs{}).inviteCreated(t.Context(), args(), false); err != nil {
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
	if err := NewWorker(invites, out, nil, &recordingJobs{}).inviteCreated(t.Context(), args(), false); err == nil {
		t.Fatal("送出失敗要回錯誤讓 River 重試")
	}
	if len(invites.sent)+len(invites.failed) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestFailedSendOnTheLastAttemptMarksTheInviteFailed(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	if err := NewWorker(invites, out, nil, &recordingJobs{}).inviteCreated(t.Context(), args(), true); err == nil {
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
	w := &typedWorker{w: NewWorker(invites, out, nil, &recordingJobs{})}
	if err := w.Work(t.Context(), job(maxAttempts, maxAttempts)); err == nil {
		t.Fatal("用盡仍要回送出的錯誤")
	}
	if len(invites.failed) != 1 {
		t.Fatalf("Attempt == MaxAttempts 要標 FAILED，failed=%v", invites.failed)
	}
}

func TestWorkBeforeMaxAttemptsMarksNothing(t *testing.T) {
	invites, out := &fakeInvites{}, &fakeSender{err: errors.New("broker down")}
	w := &typedWorker{w: NewWorker(invites, out, nil, &recordingJobs{})}
	if err := w.Work(t.Context(), job(maxAttempts-1, maxAttempts)); err == nil {
		t.Fatal("送出失敗要回錯誤")
	}
	if len(invites.failed)+len(invites.sent) != 0 {
		t.Fatalf("sent=%v failed=%v", invites.sent, invites.failed)
	}
}

func TestStateWriteFailureAfterASuccessfulSendQueuesAStateJobAndNeverResends(t *testing.T) {
	invites, out, q := &fakeInvites{markErr: errors.New("db down")}, &fakeSender{}, &recordingJobs{}
	for _, last := range []bool{false, true} {
		if err := NewWorker(invites, out, nil, q).inviteCreated(t.Context(), args(), last); err != nil {
			t.Fatalf("last=%v: err = %v，送出已成功，狀態寫入失敗要交給狀態 job 而不是讓送出 job 重跑", last, err)
		}
	}
	if len(out.sent) != 2 {
		t.Fatalf("sent = %d，每次呼叫各送一次、狀態失敗本身不得再送", len(out.sent))
	}
	want := InviteStateArgs{InviteID: inviteA, State: "SENT"}
	if len(q.args) != 2 || q.args[0] != want {
		t.Fatalf("jobs = %v，預期狀態 job %v", q.args, want)
	}
}

func TestStateJobRetriesOnlyTheWriteAndEventuallyMarksSent(t *testing.T) {
	invites, out := &fakeInvites{markErr: errors.New("db down")}, &fakeSender{}
	w := NewWorker(invites, out, nil, &recordingJobs{})
	sw := &stateWorker{w: w}
	job := &river.Job[InviteStateArgs]{JobRow: &rivertype.JobRow{}, Args: InviteStateArgs{InviteID: inviteA, State: "SENT"}}
	if err := sw.Work(t.Context(), job); err == nil {
		t.Fatal("寫入失敗要回錯誤讓 River 重試")
	}
	invites.markErr = nil
	if err := sw.Work(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if len(invites.sent) != 1 || len(out.sent) != 0 {
		t.Fatalf("sent=%v 送出次數=%d，狀態 job 只寫狀態、不送", invites.sent, len(out.sent))
	}
}

func TestStateJobWithAnUnknownStateIsCancelled(t *testing.T) {
	sw := &stateWorker{w: NewWorker(&fakeInvites{}, &fakeSender{}, nil, &recordingJobs{})}
	err := sw.Work(t.Context(), &river.Job[InviteStateArgs]{JobRow: &rivertype.JobRow{}, Args: InviteStateArgs{InviteID: inviteA, State: "NOPE"}})
	var cancel *rivertype.JobCancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("err = %v，未知狀態不重試", err)
	}
}

func TestStateJobHasFiniteAttemptsAndIsUniqueByInvite(t *testing.T) {
	o := InviteStateArgs{}.InsertOpts()
	if o.MaxAttempts != stateMaxAttempts || stateMaxAttempts <= 0 || !o.UniqueOpts.ByArgs {
		t.Fatalf("InsertOpts = %+v", o)
	}
}

func TestExhaustedSendWithAFailingWriteQueuesAFailedStateJobAndKeepsTheSendError(t *testing.T) {
	sendErr := errors.New("broker down")
	invites, q := &fakeInvites{markErr: errors.New("db down")}, &recordingJobs{}
	err := NewWorker(invites, &fakeSender{err: sendErr}, nil, q).inviteCreated(t.Context(), args(), true)
	if !errors.Is(err, sendErr) {
		t.Fatalf("err = %v，要保留送出的錯誤", err)
	}
	if len(q.args) != 1 || q.args[0] != (InviteStateArgs{InviteID: inviteA, State: "FAILED"}) {
		t.Fatalf("jobs = %v", q.args)
	}
}

func TestSendWriteAndEnqueueFailuresAreAllReported(t *testing.T) {
	sendErr, markErr, enqErr := errors.New("broker down"), errors.New("db down"), errors.New("queue down")
	invites := &fakeInvites{markErr: markErr}
	err := NewWorker(invites, &fakeSender{err: sendErr}, nil, &recordingJobs{fail: enqErr}).inviteCreated(t.Context(), args(), true)
	for _, want := range []error{sendErr, markErr, enqErr} {
		if !errors.Is(err, want) {
			t.Fatalf("err = %v，缺少 %v", err, want)
		}
	}
	err = NewWorker(invites, &fakeSender{}, nil, &recordingJobs{fail: enqErr}).inviteCreated(t.Context(), args(), false)
	if !errors.Is(err, markErr) || !errors.Is(err, enqErr) {
		t.Fatalf("err = %v，送出成功但寫入與排入都失敗時要回兩個錯誤", err)
	}
}

func TestStateIsWrittenEvenWhenTheSendContextIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	invites := &fakeInvites{}
	if err := NewWorker(invites, &fakeSender{}, nil, &recordingJobs{}).inviteCreated(ctx, args(), false); err != nil {
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

func staleAt(n int, id uuid.UUID) procurement.StaleInvite {
	return procurement.StaleInvite{ID: id, CreatedAt: time.Date(2026, 11, 24, 0, n, 0, 0, time.UTC)}
}

func TestSweepMarksOnlyStaleInvitesWithoutALiveJob(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	invites := &fakeInvites{stale: []procurement.StaleInvite{staleAt(1, inviteA), staleAt(2, inviteB)}}
	w := NewWorker(invites, nil, fakeLive{inviteB: true}, &recordingJobs{})
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

func TestSweepPagesPastLiveRowsToReachALostInvite(t *testing.T) {
	live := fakeLive{}
	var stale []procurement.StaleInvite
	for i := 0; i < 6; i++ {
		id := uuid.NewV7()
		live[id] = true
		stale = append(stale, staleAt(i, id))
	}
	stale = append(stale, staleAt(10, inviteA))
	invites := &fakeInvites{stale: stale}
	w := NewWorker(invites, nil, live, &recordingJobs{})
	w.batch = 2
	n, err := w.Sweep(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(invites.failed) != 1 || invites.failed[0] != inviteA {
		t.Fatalf("n=%d failed=%v，排在前面的活躍列不得擋住後面遺失的邀請", n, invites.failed)
	}
}
