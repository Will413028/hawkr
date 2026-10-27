package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

func TestCancelRun_Registered(t *testing.T) {
	fullMethod := "/hawkr.ai.AIProcurementService/CancelRun"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestCancelRun_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := newWatchEventsServer(nil)

	_, err := s.CancelRun(context.Background(), &aipb.CancelRunRequest{RunId: "1", ExpectedRunVersion: "1"})
	require.Error(t, err)
	code, _ := statusCodeAndReason(t, err)
	require.Equal(t, codes.PermissionDenied, code)
}

func TestCancelRun_InvalidRunId_ReturnsNotFoundShape(t *testing.T) {
	s := newWatchEventsServer(nil)

	_, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "not-a-number", ExpectedRunVersion: "1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.NotFound, code)
	require.Equal(t, "REQUEST_NOT_FOUND", reason)
}

func TestCancelRun_InvalidExpectedRunVersion_ReturnsInvalidArgument(t *testing.T) {
	store := &fakeRunStore{}
	s := newWatchEventsServer(store)

	_, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "5150", ExpectedRunVersion: "not-a-number"})
	code, _ := statusCodeAndReason(t, err)
	require.Equal(t, codes.InvalidArgument, code)
	require.Empty(t, store.cancelCalls, "expectedRunVersion 解析失敗時不得呼叫 CancelRun")
}

func TestCancelRun_Success_WiresRequestThroughAndReturnsResponse(t *testing.T) {
	store := &fakeRunStore{
		cancelRun:         repository.Run{ID: 5150, Status: domain.RunStatusCancelReq(), Version: 6},
		cancelDisposition: repository.CancelDispositionRequested,
	}
	s := newWatchEventsServer(store)

	resp, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{
		RunId: "5150", ExpectedRunVersion: "5", ClientRequestId: "cr-1", Reason: "buyer changed mind",
	})
	require.NoError(t, err)
	require.Equal(t, "5150", resp.GetRunId())
	require.Equal(t, domain.RunStatusCancelReq(), resp.GetRunStatus())
	require.Equal(t, repository.CancelDispositionRequested, resp.GetCancelDisposition())
	require.Equal(t, "6", resp.GetRunVersion())

	require.Len(t, store.cancelCalls, 1)
	got := store.cancelCalls[0]
	require.Equal(t, int64(5150), got.RunID)
	require.Equal(t, identity.BuyerID(8001), got.BuyerID)
	require.Equal(t, int64(5), got.ExpectedVersion)
}

func TestCancelRun_TerminalDispositions_PassThroughAsSuccess(t *testing.T) {
	dispositions := []string{
		repository.CancelDispositionRequested,
		repository.CancelDispositionAlreadyCancelled,
		repository.CancelDispositionAlreadyTerminal,
		repository.CancelDispositionTooLate,
	}
	for _, disp := range dispositions {
		t.Run(disp, func(t *testing.T) {
			store := &fakeRunStore{
				cancelRun:         repository.Run{ID: 5150, Status: "SUCCEEDED", Version: 9},
				cancelDisposition: disp,
			}
			s := newWatchEventsServer(store)

			resp, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "5150", ExpectedRunVersion: "9"})
			require.NoError(t, err)
			require.Equal(t, disp, resp.GetCancelDisposition())
		})
	}
}

func TestCancelRun_VersionConflict_ReturnsAbortedApprovalStale(t *testing.T) {
	store := &fakeRunStore{cancelErr: repository.ErrCancelVersionConflict}
	s := newWatchEventsServer(store)

	_, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "5150", ExpectedRunVersion: "3"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "APPROVAL_STALE", reason)
}

func TestCancelRun_InterruptPending_ReturnsAbortedApprovalPending(t *testing.T) {
	store := &fakeRunStore{cancelErr: repository.ErrCancelInterruptPending}
	s := newWatchEventsServer(store)

	_, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "5150", ExpectedRunVersion: "3"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "APPROVAL_PENDING", reason)
}

func TestCancelRun_RunNotFound_ReturnsNotFoundShape(t *testing.T) {
	store := &fakeRunStore{cancelErr: repository.ErrNotFound}
	s := newWatchEventsServer(store)

	_, err := s.CancelRun(buyerActorContext("8001"), &aipb.CancelRunRequest{RunId: "5150", ExpectedRunVersion: "3"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.NotFound, code)
	require.Equal(t, "REQUEST_NOT_FOUND", reason)
}

func newResumeCommandServer(store *stubRunStore, enabled bool) *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		durableRunDeps(store, enabled), nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func TestResumeRun_Registered(t *testing.T) {
	fullMethod := "/hawkr.ai.AIProcurementService/ResumeRun"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestResumeRun_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := newResumeCommandServer(&stubRunStore{}, true)

	_, err := s.ResumeRun(context.Background(), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	require.Error(t, err)
	code, _ := statusCodeAndReason(t, err)
	require.Equal(t, codes.PermissionDenied, code)
}

func TestResumeRun_MissingInterruptId_ReturnsInvalidArgument(t *testing.T) {
	store := &stubRunStore{}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{ClientRequestId: "cr-1"})
	code, _ := statusCodeAndReason(t, err)
	require.Equal(t, codes.InvalidArgument, code)
	require.Empty(t, store.resumeCalls, "interruptId 缺漏時不得呼叫 ResumeRun")
}

func TestResumeRun_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	store := &stubRunStore{}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1"})
	code, _ := statusCodeAndReason(t, err)
	require.Equal(t, codes.InvalidArgument, code)
	require.Empty(t, store.resumeCalls, "clientRequestId 缺漏時不得呼叫 ResumeRun")
}

func TestResumeRun_Succeeds_NoFlagsConfigured(t *testing.T) {
	store := &stubRunStore{
		resumeOut: repository.ResumeRunOutput{
			Child: repository.Run{ID: 6002, Status: domain.RunStatusQueued()},
			IsNew: false,
		},
	}
	s := newResumeCommandServer(store, false)

	resp, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	require.NoError(t, err, "ResumeRun must succeed unconditionally, flag configuration must not matter")
	require.Equal(t, "6002", resp.GetRunId())
	require.Len(t, store.resumeCalls, 1, "ResumeRun must reach usecase.ClaimAndDispatchResume with no flags configured")
}

func TestResumeRun_Success_WiresRequestThroughAndReturnsResponse(t *testing.T) {
	store := &stubRunStore{
		resumeOut: repository.ResumeRunOutput{
			Child: repository.Run{ID: 6001, Status: domain.RunStatusQueued()},
			IsNew: false,
		},
	}
	s := newResumeCommandServer(store, true)

	payload := []byte(`{"optionId":"crop-a"}`)
	resp, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{
		InterruptId: "int-1", ClientRequestId: "cr-1", ResponsePayload: payload,
	})
	require.NoError(t, err)
	require.Equal(t, "6001", resp.GetRunId())
	require.Equal(t, domain.RunStatusQueued(), resp.GetRunStatus())

	require.Len(t, store.resumeCalls, 1)
	got := store.resumeCalls[0]
	require.Equal(t, "int-1", got.InterruptID)
	require.Equal(t, identity.BuyerID(8001), got.BuyerID)
	require.Equal(t, "cr-1", got.ClientRequestID)
	require.Equal(t, payload, got.ResponsePayload)
	require.Equal(t, usecase.CanonicalResumePayloadHash(identity.BuyerID(8001), "int-1", payload), got.PayloadHash)
	require.NotEmpty(t, got.PayloadHash)
}

func TestResumeRun_AlreadyResolved_ReturnsAbortedApprovalAlreadyResolved(t *testing.T) {
	store := &stubRunStore{resumeErr: repository.ErrInterruptAlreadyResolved}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "APPROVAL_ALREADY_RESOLVED", reason)
}

func TestResumeRun_InterruptNotFound_ReturnsNotFoundShape(t *testing.T) {
	store := &stubRunStore{resumeErr: repository.ErrNotFound}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.NotFound, code)
	require.Equal(t, "REQUEST_NOT_FOUND", reason)
}

func TestResumeRun_IdempotencyConflict_ReturnsAbortedIdempotencyKeyConflict(t *testing.T) {
	store := &stubRunStore{resumeErr: repository.ErrIdempotencyConflict}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "IDEMPOTENCY_KEY_CONFLICT", reason)
}

func TestResumeRun_RunNotResumable_ReturnsAbortedRunNotActive(t *testing.T) {
	store := &stubRunStore{resumeErr: repository.ErrRunNotResumable}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.Aborted, code)
	require.Equal(t, "RUN_NOT_ACTIVE", reason)
}

func TestResumeRun_UnsupportedInterruptKind_ReturnsInvalidArgumentSchemaVersionUnsupported(t *testing.T) {
	store := &stubRunStore{resumeErr: usecase.ErrUnsupportedInterruptKind}
	s := newResumeCommandServer(store, true)

	_, err := s.ResumeRun(buyerActorContext("8001"), &aipb.ResumeRunRequest{InterruptId: "int-1", ClientRequestId: "cr-1"})
	code, reason := statusCodeAndReason(t, err)
	require.Equal(t, codes.InvalidArgument, code)
	require.Equal(t, "SCHEMA_VERSION_UNSUPPORTED", reason)
}
