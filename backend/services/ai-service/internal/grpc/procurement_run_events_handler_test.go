package grpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

type fakeRunEventStream struct {
	ctx context.Context

	mu   sync.Mutex
	sent []*aipb.RunEvent
}

func (f *fakeRunEventStream) Send(ev *aipb.RunEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, ev)
	return nil
}
func (f *fakeRunEventStream) Context() context.Context { return f.ctx }

func (f *fakeRunEventStream) Sent() []*aipb.RunEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*aipb.RunEvent, len(f.sent))
	copy(out, f.sent)
	return out
}

func (f *fakeRunEventStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeRunEventStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeRunEventStream) SetTrailer(metadata.MD)       {}
func (f *fakeRunEventStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeRunEventStream) RecvMsg(m interface{}) error  { return nil }

type fakeRunStore struct {
	run       repository.Run
	getRunErr error

	getRunCalls     int
	getRunErrOnCall int

	events  []repository.RunEvent
	listErr error

	watchCh  chan int64
	watchErr error

	watchCalls int
	listCalls  []int64

	cancelRun         repository.Run
	cancelDisposition string
	cancelErr         error
	cancelCalls       []repository.CancelRunInput

	openInterrupt    repository.Interrupt
	hasOpenInterrupt bool
	openInterruptErr error

	runsByConversation []repository.Run
	listRunsErr        error
	listRunsCalls      []struct {
		conversationID int64
		buyerID        identity.BuyerID
	}

	artifactsByConversation []repository.RunArtifact
	listArtifactsErr        error
	listArtifactsCalls      []struct {
		conversationID int64
		buyerID        identity.BuyerID
	}

	openInterruptForRun         repository.Interrupt
	hasOpenInterruptForRun      bool
	openInterruptForRunErr      error
	getOpenInterruptForRunCalls []struct {
		runID   int64
		buyerID identity.BuyerID
	}

	conversationIDForRequest         int64
	conversationIDForRequestErr      error
	getConversationIDForRequestCalls []int64
}

func (f *fakeRunStore) CancelRun(_ context.Context, in repository.CancelRunInput) (repository.Run, string, error) {
	f.cancelCalls = append(f.cancelCalls, in)
	if f.cancelErr != nil {
		return repository.Run{}, "", f.cancelErr
	}
	return f.cancelRun, f.cancelDisposition, nil
}

func (f *fakeRunStore) FindOpenInterruptForRequest(_ context.Context, _ int64, _ identity.BuyerID) (repository.Interrupt, error) {
	if f.openInterruptErr != nil {
		return repository.Interrupt{}, f.openInterruptErr
	}
	if f.hasOpenInterrupt {
		return f.openInterrupt, nil
	}
	return repository.Interrupt{}, repository.ErrNotFound
}

func (f *fakeRunStore) GetRun(_ context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error) {
	f.getRunCalls++
	if f.getRunErr != nil && (f.getRunErrOnCall == 0 || f.getRunCalls == f.getRunErrOnCall) {
		return repository.Run{}, f.getRunErr
	}
	if runID != f.run.ID || buyerID != f.run.BuyerID {
		return repository.Run{}, repository.ErrNotFound
	}
	return f.run, nil
}

func (f *fakeRunStore) ListRunEventsAfterSeq(_ context.Context, _ int64, afterSeq int64) ([]repository.RunEvent, error) {
	f.listCalls = append(f.listCalls, afterSeq)
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []repository.RunEvent
	for _, ev := range f.events {
		if ev.Seq > afterSeq {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (f *fakeRunStore) ListRunsByConversation(_ context.Context, conversationID int64, buyerID identity.BuyerID) ([]repository.Run, error) {
	f.listRunsCalls = append(f.listRunsCalls, struct {
		conversationID int64
		buyerID        identity.BuyerID
	}{conversationID, buyerID})
	if f.listRunsErr != nil {
		return nil, f.listRunsErr
	}
	return f.runsByConversation, nil
}

func (f *fakeRunStore) ListLatestArtifactsByConversation(_ context.Context, conversationID int64, buyerID identity.BuyerID) ([]repository.RunArtifact, error) {
	f.listArtifactsCalls = append(f.listArtifactsCalls, struct {
		conversationID int64
		buyerID        identity.BuyerID
	}{conversationID, buyerID})
	if f.listArtifactsErr != nil {
		return nil, f.listArtifactsErr
	}
	return f.artifactsByConversation, nil
}

func (f *fakeRunStore) GetOpenInterruptForRun(_ context.Context, runID int64, buyerID identity.BuyerID) (repository.Interrupt, bool, error) {
	f.getOpenInterruptForRunCalls = append(f.getOpenInterruptForRunCalls, struct {
		runID   int64
		buyerID identity.BuyerID
	}{runID, buyerID})
	if f.openInterruptForRunErr != nil {
		return repository.Interrupt{}, false, f.openInterruptForRunErr
	}
	if f.hasOpenInterruptForRun {
		return f.openInterruptForRun, true, nil
	}
	return repository.Interrupt{}, false, nil
}

func (f *fakeRunStore) GetConversationIDForRequest(_ context.Context, requestID int64) (int64, error) {
	f.getConversationIDForRequestCalls = append(f.getConversationIDForRequestCalls, requestID)
	if f.conversationIDForRequestErr != nil {
		return 0, f.conversationIDForRequestErr
	}
	return f.conversationIDForRequest, nil
}

func (f *fakeRunStore) WatchRun(context.Context, int64) (chan int64, func(), error) {
	f.watchCalls++
	if f.watchErr != nil {
		return nil, nil, f.watchErr
	}
	if f.watchCh != nil {
		return f.watchCh, func() {}, nil
	}
	ch := make(chan int64)
	close(ch)
	return ch, func() {}, nil
}

func newWatchEventsServer(store RunStore) *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, store, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func statusCodeAndReason(t *testing.T, err error) (codes.Code, string) {
	t.Helper()
	st := status.Convert(err)
	var reason string
	for _, d := range st.Details() {
		if be, ok := d.(*aipb.BusinessError); ok {
			reason = be.GetReason()
		}
	}
	return st.Code(), reason
}

func TestWatchRunEvents_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/WatchRunEvents"]
	if !ok {
		t.Fatal("WatchRunEvents not registered in methodPolicies")
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestWatchRunEvents_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := newWatchEventsServer(nil)
	stream := &fakeRunEventStream{ctx: context.Background()}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "1"}, stream)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code, _ := statusCodeAndReason(t, err); code != codes.PermissionDenied {
		t.Fatalf("code = %v, want PermissionDenied", code)
	}
}

func TestWatchRunEvents_InvalidRunID_ReturnsNotFoundShape(t *testing.T) {
	s := newWatchEventsServer(nil)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "not-a-number"}, stream)
	code, reason := statusCodeAndReason(t, err)
	if code != codes.NotFound || reason != "REQUEST_NOT_FOUND" {
		t.Fatalf("code=%v reason=%q, want NotFound/REQUEST_NOT_FOUND", code, reason)
	}
}

func TestWatchRunEvents_RunNotOwnedByBuyer_ReturnsNotFoundShape(t *testing.T) {
	store := &fakeRunStore{run: repository.Run{ID: 5150, BuyerID: 8001}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("9999")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream)
	code, reason := statusCodeAndReason(t, err)
	if code != codes.NotFound || reason != "REQUEST_NOT_FOUND" {
		t.Fatalf("code=%v reason=%q, want NotFound/REQUEST_NOT_FOUND", code, reason)
	}
	if store.watchCalls != 0 {
		t.Fatal("ownership check 失敗時不得呼叫 WatchRun")
	}
}

func TestWatchRunEvents_InvalidAfterSeq_ReturnsNotFoundShape(t *testing.T) {
	store := &fakeRunStore{run: repository.Run{ID: 5150, BuyerID: 8001, LastEventSeq: 3, MinAvailableSeq: 1}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150", AfterSeq: "not-a-number"}, stream)
	code, reason := statusCodeAndReason(t, err)
	if code != codes.NotFound || reason != "REQUEST_NOT_FOUND" {
		t.Fatalf("code=%v reason=%q, want NotFound/REQUEST_NOT_FOUND", code, reason)
	}
	if store.watchCalls != 0 {
		t.Fatal("cursor 解析失敗時不得呼叫 WatchRun")
	}
}

func fixtureRunEvent(runID, seq int64, eventType string) repository.RunEvent {
	return repository.RunEvent{
		ID: 9000 + seq, RunID: runID, Seq: seq, EventType: eventType,
		SchemaVersion: "v1", PublicPayloadJSON: []byte(`{"k":"v"}`),
		OccurredAt: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
	}
}

func TestWatchRunEvents_ReplayFromCursor(t *testing.T) {
	run := repository.Run{ID: 5150, BuyerID: 8001, RunType: domain.RunTypeTurn, LastEventSeq: 3, MinAvailableSeq: 1}
	events := []repository.RunEvent{
		fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED"),
		fixtureRunEvent(run.ID, 2, "TURN_APPLIED"),
		fixtureRunEvent(run.ID, 3, "RUN_SUCCEEDED"),
	}

	cases := []struct {
		name      string
		afterSeq  string
		wantSeqs  []string
		wantTypes []string
	}{
		{"from start", "", []string{"1", "2", "3"}, []string{"RUN_ACCEPTED", "TURN_APPLIED", "RUN_SUCCEEDED"}},
		{"from seq 1", "1", []string{"2", "3"}, []string{"TURN_APPLIED", "RUN_SUCCEEDED"}},
		{"from seq 2", "2", []string{"3"}, []string{"RUN_SUCCEEDED"}},
		{"caught up (seq 3, the high-water mark)", "3", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeRunStore{run: run, events: events}
			s := newWatchEventsServer(store)
			stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

			err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150", AfterSeq: tc.afterSeq}, stream)
			require.NoError(t, err)

			got := stream.Sent()
			require.Len(t, got, len(tc.wantSeqs))
			for i, ev := range got {
				require.Equal(t, tc.wantSeqs[i], ev.GetSeq())
				require.Equal(t, tc.wantTypes[i], ev.GetType())
				require.Equal(t, "5150", ev.GetRunId())
				require.Equal(t, "2026-08-27T12:00:00Z", ev.GetOccurredAt())
				require.JSONEq(t, `{"k":"v"}`, string(ev.GetDataJson()))
			}
			require.Equal(t, 1, store.watchCalls, "replay 前必須先呼叫 WatchRun 註冊 live wake（doc57 §2.5-4）")
		})
	}
}

func TestWatchRunEvents_CursorEarlierThanMinAvailableSeq_ReturnsExpiredStatus(t *testing.T) {
	store := &fakeRunStore{run: repository.Run{ID: 5150, BuyerID: 8001, LastEventSeq: 10, MinAvailableSeq: 5}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150", AfterSeq: "3"}, stream)
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.OutOfRange, code)
	require.Equal(t, "REPLAY_CURSOR_EXPIRED", reason)
	require.Zero(t, store.watchCalls, "游標過期時不得呼叫 WatchRun")
	require.Empty(t, stream.Sent())
}

func TestWatchRunEvents_CursorAheadOfHighWaterMark_ReturnsAheadStatus(t *testing.T) {
	store := &fakeRunStore{run: repository.Run{ID: 5150, BuyerID: 8001, LastEventSeq: 3, MinAvailableSeq: 1}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150", AfterSeq: "99"}, stream)
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "REPLAY_CURSOR_AHEAD", reason)
	require.Zero(t, store.watchCalls, "游標超前時不得呼叫 WatchRun")
	require.Empty(t, stream.Sent())
}

func TestWatchRunEvents_WatchRunFails_ReturnsErrorWithoutSendingReplay(t *testing.T) {
	watchErr := context.DeadlineExceeded
	store := &fakeRunStore{
		run:      repository.Run{ID: 5150, BuyerID: 8001, LastEventSeq: 3, MinAvailableSeq: 1},
		events:   []repository.RunEvent{fixtureRunEvent(5150, 1, "RUN_ACCEPTED")},
		watchErr: watchErr,
	}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	err := s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream)
	require.ErrorIs(t, err, watchErr)
	require.Empty(t, stream.Sent())
}

func TestWatchRunEvents_ContextCancelled_ReturnsNilNotError(t *testing.T) {
	run := repository.Run{ID: 5150, BuyerID: 8001, LastEventSeq: 1, MinAvailableSeq: 1}
	store := &fakeRunStore{
		run:     run,
		events:  []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED")},
		watchCh: make(chan int64),
	}
	s := newWatchEventsServer(store)

	ctx, cancel := context.WithCancel(buyerActorContext("8001"))
	stream := &fakeRunEventStream{ctx: ctx}

	errCh := make(chan error, 1)
	go func() { errCh <- s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream) }()

	require.Eventually(t, func() bool { return len(stream.Sent()) >= 1 }, time.Second, 5*time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("WatchRunEvents did not return after ctx cancel")
	}
}

func TestWatchRunEvents_CarriesConversationID(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, ConversationID: 777,
		RunType: domain.RunTypeCandidateSearch, LastEventSeq: 1, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{run: run, events: []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_FINISHED")}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	require.NoError(t, s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream))

	got := stream.Sent()
	require.Len(t, got, 1)
	require.Equal(t, "777", got[0].GetConversationId())
}

func TestWatchRunEvents_AbsentConversationIDIsEmptyNotZero(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, ConversationID: 0,
		RunType: domain.RunTypeTurn, LastEventSeq: 1, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{run: run, events: []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_FINISHED")}}
	s := newWatchEventsServer(store)
	stream := &fakeRunEventStream{ctx: buyerActorContext("8001")}

	require.NoError(t, s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream))

	got := stream.Sent()
	require.Len(t, got, 1)
	require.Empty(t, got[0].GetConversationId(), "缺席的 conversation 必須是空字串，不是 \"0\"")
}

func assertDoesNotReturn(t *testing.T, errCh <-chan error, cancel context.CancelFunc, window time.Duration) {
	t.Helper()
	select {
	case err := <-errCh:
		t.Fatalf("subscription closed but must have stayed open (err=%v)", err)
	case <-time.After(window):
	}
	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("WatchRunEvents did not return after ctx cancel")
	}
}

func startWatch(t *testing.T, store *fakeRunStore) (<-chan error, *fakeRunEventStream, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(buyerActorContext("8001"))
	stream := &fakeRunEventStream{ctx: ctx}
	errCh := make(chan error, 1)
	go func() { errCh <- store.serve(t, stream) }()
	return errCh, stream, cancel
}

func (f *fakeRunStore) serve(t *testing.T, stream *fakeRunEventStream) error {
	t.Helper()
	return newWatchEventsServer(f).WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: "5150"}, stream)
}

func TestWatchRunEvents_TerminalRunClosesStream(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, Status: domain.RunStatusSucceeded(),
		LastEventSeq: 2, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{
		run: run,
		events: []repository.RunEvent{
			fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED"),
			fixtureRunEvent(run.ID, 2, "RUN_FINISHED"),
		},
		watchCh: make(chan int64),
	}
	errCh, stream, cancel := startWatch(t, store)
	defer cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("terminal run did not close the subscription")
	}
	require.Len(t, stream.Sent(), 2)
	require.Equal(t, "RUN_FINISHED", stream.Sent()[1].GetType())
}

func TestWatchRunEvents_TerminalButTailNotSentStaysOpen(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, Status: domain.RunStatusSucceeded(),
		LastEventSeq: 5, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{
		run:     run,
		events:  []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED")},
		watchCh: make(chan int64),
	}
	errCh, stream, cancel := startWatch(t, store)
	require.Eventually(t, func() bool { return len(stream.Sent()) >= 1 }, time.Second, 5*time.Millisecond)
	assertDoesNotReturn(t, errCh, cancel, 200*time.Millisecond)
}

func TestWatchRunEvents_NonTerminalRunStaysOpen(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, Status: domain.RunStatusRunning(),
		LastEventSeq: 1, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{
		run:     run,
		events:  []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED")},
		watchCh: make(chan int64),
	}
	errCh, stream, cancel := startWatch(t, store)
	require.Eventually(t, func() bool { return len(stream.Sent()) >= 1 }, time.Second, 5*time.Millisecond)
	assertDoesNotReturn(t, errCh, cancel, 200*time.Millisecond)
}

func TestWatchRunEvents_RunRereadFailureStaysOpen(t *testing.T) {
	run := repository.Run{
		ID: 5150, BuyerID: 8001, Status: domain.RunStatusSucceeded(),
		LastEventSeq: 1, MinAvailableSeq: 1,
	}
	store := &fakeRunStore{
		run:             run,
		events:          []repository.RunEvent{fixtureRunEvent(run.ID, 1, "RUN_ACCEPTED")},
		watchCh:         make(chan int64),
		getRunErr:       errors.New("status re-read boom"),
		getRunErrOnCall: 2,
	}
	errCh, stream, cancel := startWatch(t, store)
	require.Eventually(t, func() bool { return len(stream.Sent()) >= 1 }, time.Second, 5*time.Millisecond)
	assertDoesNotReturn(t, errCh, cancel, 200*time.Millisecond)
	require.Equal(t, 2, store.getRunCalls)
}
