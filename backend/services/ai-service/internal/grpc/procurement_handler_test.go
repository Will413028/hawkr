package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/actor"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/handler"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func buyerActorContext(buyerID string) context.Context {
	return context.WithValue(context.Background(), actorContextKey{}, actor.Actor{StateKey: actor.StateKeyBuyer, UserID: buyerID})
}

func TestSubmitTurn_Registered(t *testing.T) {
	fullMethod := "/hawkr.ai.AIProcurementService/SubmitTurn"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestSubmitTurnStream_Registered(t *testing.T) {
	fullMethod := "/hawkr.ai.AIProcurementService/SubmitTurnStream"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestGetSnapshot_Registered(t *testing.T) {
	fullMethod := "/hawkr.ai.AIProcurementService/GetSnapshot"
	if _, ok := methodPolicies[fullMethod]; !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
}

func TestSubmitTurn_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitTurn(context.Background(), &aipb.TurnRequest{Message: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubmitTurn_MessageOverLimit_ReturnsFailedPrecondition(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	long := strings.Repeat("梨", handler.MaxMessageRunes+1)
	_, err := s.SubmitTurn(buyerActorContext("1"), &aipb.TurnRequest{ClientRequestId: "cr-1", Message: long})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want FailedPrecondition (→ HTTP 422)", got)
	}
	wantMsg := "單則訊息上限 2000 字"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitTurnStream_MessageOverLimit_ReturnsFailedPrecondition(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	long := strings.Repeat("梨", handler.MaxMessageRunes+1)
	stream := &fakeAssistantStream{ctx: buyerActorContext("1")}
	err := s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-1", Message: long}, stream)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want FailedPrecondition (→ HTTP 422)", got)
	}
	wantMsg := "單則訊息上限 2000 字"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestStartCandidateSearch_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/StartCandidateSearch"]; !ok {
		t.Fatal("StartCandidateSearch not registered")
	}
}

func TestGetCandidateSearch_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/GetCandidateSearch"]; !ok {
		t.Fatal("GetCandidateSearch not registered")
	}
}

func TestToggleSelection_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/ToggleSelection"]; !ok {
		t.Fatal("ToggleSelection not registered")
	}
}

func TestToggleSelection_MissingRequiredFields_ReturnsInvalidArgumentBeforeDB(t *testing.T) {
	cases := []struct {
		name string
		req  *aipb.ToggleSelectionRequest
	}{
		{"empty lane", &aipb.ToggleSelectionRequest{RequestId: "1", CandidateSetId: "2", Lane: "", CandidateId: "3"}},
		{"empty candidateSetId", &aipb.ToggleSelectionRequest{RequestId: "1", CandidateSetId: "", Lane: "PLATFORM", CandidateId: "3"}},
		{"empty candidateId", &aipb.ToggleSelectionRequest{RequestId: "1", CandidateSetId: "2", Lane: "PLATFORM", CandidateId: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
			_, err := s.ToggleSelection(buyerActorContext("1"), tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Fatalf("status code = %v, want InvalidArgument", got)
			}
			wantMsg := "candidateSetId, lane and candidateId are required"
			if got := status.Convert(err).Message(); got != wantMsg {
				t.Fatalf("message = %q, want %q", got, wantMsg)
			}
		})
	}
}

func TestSubmitAnswer_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/SubmitAnswer"]; !ok {
		t.Fatal("SubmitAnswer not registered")
	}
}

func TestSubmitAnswer_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{RequestId: "1", SlotKey: "item", FreeText: "apple", ExpectedRevisionId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "clientRequestId is required"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_MissingSlotKey_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{RequestId: "1", ClientRequestId: "cr-1", FreeText: "apple", ExpectedRevisionId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "slotKey is required"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_QuantitySlot_OptionIdForbidden_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "quantity", OptionId: "opt-1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "quantity 不接受 optionId"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_QuantitySlot_MissingFreeTextAndUnitOptionId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "quantity",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "freeText 或 unitOptionId 必須至少擇一"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_NonQuantitySlot_UnitOptionIdForbidden_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "item", UnitOptionId: "u-1", FreeText: "apple",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "unitOptionId 只能在回答 quantity 時使用"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_NonQuantitySlot_NoAnswerFieldsSet_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "item",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "optionId, freeText 或 skip 必須擇一"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_NonQuantitySlot_MultipleAnswerFieldsSet_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "item", OptionId: "opt-1", FreeText: "apple",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "optionId, freeText 和 skip 最多只能擇一"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_CropDisambiguationSlot_MissingOptionId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation", FreeText: "野生黑木耳",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "crop_disambiguation 必須提供 optionId"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_CropDisambiguationSlot_OtherMissingFreeText_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation", OptionId: "crop-other",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "crop_disambiguation 選擇 crop-other 時 freeText 為必填"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_CropDisambiguationSlot_RealCandidateWithFreeText_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation", OptionId: "crop-CROP-A", FreeText: "野生黑木耳",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "crop_disambiguation 選擇非 crop-other 選項時不可帶 freeText"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_CropDisambiguationSlot_UnitOptionIdForbidden_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation", OptionId: "crop-other",
		FreeText: "野生黑木耳", UnitOptionId: "unit-kg",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "unitOptionId 只能在回答 quantity 時使用"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

type fakeCropAnswerStore struct {
	revisionID int64
}

func (f fakeCropAnswerStore) FindByIdempotencyKey(context.Context, identity.BuyerID, string) (repository.IdempotentResult, error) {
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func (f fakeCropAnswerStore) GetSnapshot(_ context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	slotsJSON, _ := json.Marshal(domain.Slots{})
	return repository.Request{
			ID: requestID, BuyerID: buyerID, ActiveRevisionID: f.revisionID,
		},
		repository.Revision{ID: f.revisionID, RequestID: requestID, SlotsJSON: slotsJSON, Status: "ACTIVE"}, nil
}

func (f fakeCropAnswerStore) ListBranchRevisions(context.Context, int64, identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	return nil, nil
}

func (f fakeCropAnswerStore) AppendRevision(_ context.Context, in repository.AppendInput) (repository.Revision, error) {
	return repository.Revision{ID: f.revisionID + 1, RequestID: in.RequestID, Status: "ACTIVE"}, nil
}

func newAnswerTestServer(revisionID int64, runStore usecase.ResumeRunStore) *AIProcurementGrpcServer {
	answerUC := usecase.NewAnswerUseCase(
		fakeCropAnswerStore{revisionID: revisionID}, nil, nil, nil, nil, nil,
	)
	return NewAIProcurementGrpcServer(nil, nil, nil, answerUC, nil, nil, nil, nil, nil, nil, nil, durableRunDeps(runStore, true), nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func newCropAnswerServerForTest(revisionID int64) *AIProcurementGrpcServer {
	return newAnswerTestServer(revisionID, &stubRunStore{runID: revisionID})
}

func TestSubmitAnswer_CropDisambiguationSlot_OtherWithFreeTextButNoPriceAmount_ReachesUsecase_ReturnsFailedPrecondition(t *testing.T) {
	s := newCropAnswerServerForTest(10)
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation",
		OptionId: "crop-other", FreeText: "野生黑木耳", ExpectedRevisionId: "10",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want FailedPrecondition (proves it reached the usecase, not the handler-layer guard)", got)
	}
	wantMsg := "答案不符合這個欄位的格式"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestSubmitAnswer_CropDisambiguationSlot_Other_StillReachesUsecase(t *testing.T) {
	s := newCropAnswerServerForTest(10)
	_, err := s.SubmitAnswer(buyerActorContext("1"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-1", SlotKey: "crop_disambiguation",
		OptionId: "crop-other", FreeText: "野生黑木耳", ExpectedRevisionId: "10",
	})
	if err == nil {
		t.Fatal("直接打 /answers 回答消歧必須被擋下——唯一合法入口是 resume")
	}
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want FailedPrecondition（證明它走到 usecase 的白名單那一關，不是卡在 handler guard）", got)
	}
}

func TestSubmitAnswer_AlwaysDispatchesViaGraphExecutor(t *testing.T) {
	runStore := &stubRunStore{runID: 6100}
	s := newAnswerTestServer(10, runStore)
	_, err := s.SubmitAnswer(buyerActorContext("8001"), &aipb.SubmitAnswerRequest{
		RequestId: "1", ClientRequestId: "cr-answer-graph-1", SlotKey: "packaging",
		FreeText: "紙箱", ExpectedRevisionId: "10",
	})
	require.NoError(t, err)
	require.Equal(t, 1, runStore.claims, "SubmitAnswer must claim exactly one run, unconditionally")
	require.Len(t, runStore.terminals, 1)
	require.Len(t, runStore.claimedControllerKinds, 1)
	require.Equal(t, domain.ControllerKindGraph, runStore.claimedControllerKinds[0],
		"ClaimAndDispatchAnswer must claim ControllerKindGraph, not the retired Durable path")
}

func TestRewind_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/Rewind"]; !ok {
		t.Fatal("Rewind not registered")
	}
}

func TestRewind_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.Rewind(buyerActorContext("1"), &aipb.RewindRequest{RequestId: "1", SlotKey: "item"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "clientRequestId is required"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

func TestRewind_MissingSlotKey_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.Rewind(buyerActorContext("1"), &aipb.RewindRequest{RequestId: "1", ClientRequestId: "cr-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
	wantMsg := "slotKey is required"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Fatalf("message = %q, want %q", got, wantMsg)
	}
}

type fakeRewindStore struct{}

func (fakeRewindStore) FindByIdempotencyKey(context.Context, identity.BuyerID, string) (repository.IdempotentResult, error) {
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func (fakeRewindStore) ListBranchRevisions(context.Context, int64, identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	before, _ := json.Marshal(domain.Slots{})
	packaging := "紙箱"
	after, _ := json.Marshal(domain.Slots{Packaging: &packaging})
	return []repository.RevisionWithPatch{
		{Revision: repository.Revision{ID: 20, SlotsJSON: before}},
		{Revision: repository.Revision{ID: 21, SlotsJSON: after}},
	}, nil
}

func (fakeRewindStore) RewindTo(_ context.Context, in repository.RewindInput) (repository.Revision, error) {
	return repository.Revision{ID: in.TargetRevisionID, RequestID: in.RequestID, Status: "ACTIVE"}, nil
}

func (fakeRewindStore) GetSnapshot(_ context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	return repository.Request{ID: requestID, BuyerID: buyerID}, repository.Revision{ID: 20}, nil
}

func newRewindTestServer(runStore usecase.ResumeRunStore) *AIProcurementGrpcServer {
	rewindUC := usecase.NewRewindUseCase(fakeRewindStore{}, nil, nil, nil, nil, nil)
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, rewindUC, nil, nil, nil, nil, nil, nil, durableRunDeps(runStore, true), nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func TestRewind_AlwaysDispatchesViaGraphExecutor(t *testing.T) {
	runStore := &stubRunStore{runID: 6101}
	s := newRewindTestServer(runStore)
	_, err := s.Rewind(buyerActorContext("8001"), &aipb.RewindRequest{
		RequestId: "1", ClientRequestId: "cr-rewind-graph-1", SlotKey: "packaging",
	})
	require.NoError(t, err)
	require.Equal(t, 1, runStore.claims, "Rewind must claim exactly one run, unconditionally")
	require.Len(t, runStore.terminals, 1)
	require.Len(t, runStore.claimedControllerKinds, 1)
	require.Equal(t, domain.ControllerKindGraph, runStore.claimedControllerKinds[0],
		"ClaimAndDispatchRewind must claim ControllerKindGraph, not the retired Durable path")
}

func TestPreviewRfqPrice_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/PreviewRfqPrice"]
	if !ok {
		t.Fatal("PreviewRfqPrice not registered")
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestPreviewRfqPrice_NoActorInContext_ReturnsError(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.PreviewRfqPrice(context.Background(), &aipb.RfqPricePreviewRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("status code = %v, want PermissionDenied", got)
	}
}

func TestPreviewRfqPrice_InvalidRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.PreviewRfqPrice(buyerActorContext("1"), &aipb.RfqPricePreviewRequest{RequestId: "not-a-number"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPrice_InvalidExpectedRevisionId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.PreviewRfqPrice(buyerActorContext("1"), &aipb.RfqPricePreviewRequest{RequestId: "1", ExpectedRevisionId: "not-a-number"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPrice_EmptyClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.PreviewRfqPrice(buyerActorContext("1"), &aipb.RfqPricePreviewRequest{RequestId: "1", ExpectedRevisionId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPriceErrorStatus_MapsTypedUsecaseErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantReason string
	}{
		{"buyer requested range required", pricing.ErrBuyerRequestedRangeRequired, codes.FailedPrecondition, "BUYER_REQUESTED_RANGE_REQUIRED"},
		{"quantity unit unconvertible", pricing.ErrQuantityUnitUnconvertible, codes.FailedPrecondition, "QUANTITY_UNIT_UNCONVERTIBLE"},
		{"stale revision", pricing.ErrStalePricePreviewRevision, codes.Aborted, "STALE_REVISION"},
		{"not found falls through to default mapping", repository.ErrNotFound, codes.NotFound, "REQUEST_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := previewRfqPriceErrorStatus(t.Context(), tc.err)
			if got := status.Code(err); got != tc.wantCode {
				t.Errorf("status code = %v, want %v", got, tc.wantCode)
			}
			st := status.Convert(err)
			var reason string
			for _, d := range st.Details() {
				if be, ok := d.(*aipb.BusinessError); ok {
					reason = be.GetReason()
				}
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestConfirmRfqSnapshot_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/ConfirmRfqSnapshot"]
	if !ok {
		t.Fatal("ConfirmRfqSnapshot not registered")
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestConfirmRfqSnapshot_NoActorInContext_ReturnsError(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(context.Background(), &aipb.RfqConfirmationRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("status code = %v, want PermissionDenied", got)
	}
}

func TestConfirmRfqSnapshot_InvalidRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(buyerActorContext("1"), &aipb.RfqConfirmationRequest{RequestId: "not-a-number"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestConfirmRfqSnapshot_InvalidExpectedRevisionId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(buyerActorContext("1"), &aipb.RfqConfirmationRequest{RequestId: "1", ExpectedRevisionId: "not-a-number"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestConfirmRfqSnapshot_InvalidPricePreviewId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(buyerActorContext("1"), &aipb.RfqConfirmationRequest{
		RequestId: "1", ExpectedRevisionId: "1", PricePreviewId: "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestConfirmRfqSnapshot_InvalidPricePreviewVersion_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(buyerActorContext("1"), &aipb.RfqConfirmationRequest{
		RequestId: "1", ExpectedRevisionId: "1", PricePreviewId: "1", PricePreviewVersion: "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestConfirmRfqSnapshot_EmptyClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.ConfirmRfqSnapshot(buyerActorContext("1"), &aipb.RfqConfirmationRequest{
		RequestId: "1", ExpectedRevisionId: "1", PricePreviewId: "1", PricePreviewVersion: "1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestConfirmRfqSnapshotErrorStatus_MapsTypedUsecaseErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantReason string
	}{
		{"stale revision", pricing.ErrStalePricePreviewRevision, codes.Aborted, "STALE_REVISION"},
		{"stale price preview", commerce.ErrStalePricePreview, codes.Aborted, "STALE_REVISION"},
		{"price preview not found", repository.ErrPricePreviewNotFound, codes.NotFound, "RESOURCE_NOT_FOUND"},
		{"not found falls through to default mapping", repository.ErrNotFound, codes.NotFound, "REQUEST_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := confirmRfqSnapshotErrorStatus(t.Context(), tc.err)
			if got := status.Code(err); got != tc.wantCode {
				t.Errorf("status code = %v, want %v", got, tc.wantCode)
			}
			st := status.Convert(err)
			var reason string
			for _, d := range st.Details() {
				if be, ok := d.(*aipb.BusinessError); ok {
					reason = be.GetReason()
				}
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestToRfqPricePreviewResponseProto_CarriesBasisQuantityAndBasisUnit(t *testing.T) {
	result := pricing.PricePreviewResult{
		PricePreviewID: 3, Version: 1,
		BuyerRequestedRange: pricing.BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "30", UpperBoundAmount: "42",
			BasisQuantity: "1000", BasisUnit: "GRAM",
		},
		PayloadHash: "ghi",
	}
	resp := toRfqPricePreviewResponseProto(result)
	if got := resp.GetBuyerRequestedRange().GetBasisQuantity(); got != "1000" {
		t.Fatalf("BuyerRequestedRange.BasisQuantity = %q, want 1000", got)
	}
	if got := resp.GetBuyerRequestedRange().GetBasisUnit(); got != "GRAM" {
		t.Fatalf("BuyerRequestedRange.BasisUnit = %q, want GRAM", got)
	}
}

func TestToRfqPricePreviewResponseProto_CarriesQuoteTotal(t *testing.T) {
	result := pricing.PricePreviewResult{
		PricePreviewID: 4, Version: 1,
		BuyerRequestedRange: pricing.BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "650", UpperBoundAmount: "850",
			BasisQuantity: "1", BasisUnit: "KG",
		},
		QuoteTotalRange: &pricing.QuoteTotalRange{
			Currency: "TWD", LowerBoundAmount: "32500", UpperBoundAmount: "42500",
			Quantity: "50", QuantityUnit: "KG",
		},
		PayloadHash: "jkl",
	}
	resp := toRfqPricePreviewResponseProto(result)

	total := resp.GetQuoteTotalRange()
	if total == nil {
		t.Fatal("QuoteTotalRange = nil, want non-nil")
	}
	if total.GetLowerBoundAmount() != "32500" || total.GetUpperBoundAmount() != "42500" ||
		total.GetQuantity() != "50" || total.GetQuantityUnit() != "KG" || total.GetCurrency() != "TWD" {
		t.Errorf("QuoteTotalRange = %+v, want {TWD 32500 42500 50 KG}", total)
	}
	if resp.GetPayloadHash() != "jkl" {
		t.Errorf("PayloadHash = %q", resp.GetPayloadHash())
	}
}

func TestToRfqPricePreviewResponseProto_NilQuoteTotalStaysNil(t *testing.T) {
	result := pricing.PricePreviewResult{
		PricePreviewID: 5, Version: 1,
		BuyerRequestedRange: pricing.BuyerRequestedRange{Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850"},
		PayloadHash:         "mno",
	}
	resp := toRfqPricePreviewResponseProto(result)
	if resp.QuoteTotalRange != nil {
		t.Errorf("QuoteTotalRange = %+v, want nil", resp.QuoteTotalRange)
	}
}

func TestListConversations_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/ListConversations"]; !ok {
		t.Fatal("ListConversations not registered")
	}
}

func TestGetConversationMessages_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/GetConversationMessages"]; !ok {
		t.Fatal("GetConversationMessages not registered")
	}
}

func TestRenameConversation_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/RenameConversation"]; !ok {
		t.Fatal("RenameConversation not registered")
	}
}

func TestDeleteConversation_Registered(t *testing.T) {
	if _, ok := methodPolicies["/hawkr.ai.AIProcurementService/DeleteConversation"]; !ok {
		t.Fatal("DeleteConversation not registered")
	}
}

func TestRenameConversation_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.RenameConversation(buyerActorContext("1"), &aipb.RenameConversationRequest{ConversationId: "1", Title: "foo"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestDeleteConversation_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
	_, err := s.DeleteConversation(buyerActorContext("1"), &aipb.DeleteConversationRequest{ConversationId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

type fakeTurnEntitlementClient struct {
	entitlement usecase.Entitlement
	calls       int
}

func (f *fakeTurnEntitlementClient) GetBuyerEntitlement(context.Context, identity.BuyerID) (usecase.Entitlement, error) {
	f.calls++
	return f.entitlement, nil
}

type fakeTurnQuotaReserver struct {
	reserved bool
	calls    int
}

func (f *fakeTurnQuotaReserver) ReserveQuota(context.Context, identity.BuyerID, time.Time, int) (int64, bool, int, error) {
	f.calls++
	return 1, f.reserved, 0, nil
}

func (f *fakeTurnQuotaReserver) ReleaseQuota(context.Context, int64, identity.BuyerID) {}

func TestSubmitTurn_QuotaExceeded_ReturnsResourceExhaustedWithReason(t *testing.T) {
	resetAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:   &stubRunStore{runID: 8001},
			Dispatcher: usecase.NewRunDispatcher(),
			Entitlement: &fakeTurnEntitlementClient{entitlement: usecase.Entitlement{
				PlanCode: "PRO", MonthlyAnswerLimit: 100,
				PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: resetAt,
			}},
			Quota: &fakeTurnQuotaReserver{reserved: false},
		}, nil, SellerCommandDeps{})

	_, err := s.SubmitTurn(buyerActorContext("42"), &aipb.TurnRequest{Message: "我要買梨", ClientRequestId: "cr-quota-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st := status.Convert(err)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("status code = %v, want ResourceExhausted (→ HTTP 429)", st.Code())
	}

	var found *aipb.BusinessError
	for _, d := range st.Details() {
		if be, ok := d.(*aipb.BusinessError); ok {
			found = be
		}
	}
	if found == nil {
		t.Fatal("no BusinessError detail attached — 前端拿不到機器可讀的 reason")
	}
	if got := found.GetReason(); got != "AGENT_MONTHLY_QUOTA_EXCEEDED" {
		t.Fatalf("reason = %q, want AGENT_MONTHLY_QUOTA_EXCEEDED", got)
	}
	if got := found.GetRemaining(); got != "0" {
		t.Fatalf("remaining = %q, want \"0\"", got)
	}
	if got := found.GetResetAt(); got != "2026-03-01T00:00:00Z" {
		t.Fatalf("resetAt = %q, want 2026-03-01T00:00:00Z (RFC3339 UTC)", got)
	}
	if found.GetMessage() == "" {
		t.Fatal("message is empty — 買家看不到任何說明")
	}
}

func TestToTurnResultProto_Quota_NilWhenSnapshotAbsent(t *testing.T) {
	got := toTurnResultProto(usecase.TurnResult{RequestID: 7})
	if got.Quota != nil {
		t.Fatalf("quota = %v, want nil for a zero-value QuotaSnapshot", got.Quota)
	}
}

func TestToTurnResultProto_Quota_PopulatedWhenSnapshotPresent(t *testing.T) {
	got := toTurnResultProto(usecase.TurnResult{
		Quota: usecase.QuotaSnapshot{
			PlanCode: "PRO", Remaining: 92, Limit: 100,
			ResetAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	})

	q := got.GetQuota()
	if q == nil {
		t.Fatal("quota is nil despite a real snapshot")
	}
	if q.GetPlanCode() != "PRO" {
		t.Fatalf("planCode = %q, want PRO", q.GetPlanCode())
	}
	if q.GetRemaining() != "92" {
		t.Fatalf("remaining = %q, want \"92\"", q.GetRemaining())
	}
	if q.GetLimit() != "100" {
		t.Fatalf("limit = %q, want \"100\"", q.GetLimit())
	}
	if q.GetResetAt() != "2026-03-01T00:00:00Z" {
		t.Fatalf("resetAt = %q, want 2026-03-01T00:00:00Z", q.GetResetAt())
	}
}

type fakeSnapshotStore struct {
	req          repository.Request
	rev          repository.Revision
	err          error
	published    repository.LatestPublicationForRequest
	publishedErr error
}

func (f fakeSnapshotStore) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	return f.req, f.rev, f.err
}

func (f fakeSnapshotStore) FindLatestPublicationForRequest(context.Context, int64, identity.BuyerID) (repository.LatestPublicationForRequest, error) {
	if f.publishedErr != nil {
		return repository.LatestPublicationForRequest{}, f.publishedErr
	}
	if f.published.ID == 0 {
		return repository.LatestPublicationForRequest{}, repository.ErrPublicationNotFound
	}
	return f.published, nil
}

func newSnapshotTestServer(snapshot handler.SnapshotReader, runStore RunStore) *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(snapshot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, runStore, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func emptySlotsSnapshot(requestID, revisionID int64) fakeSnapshotStore {
	return fakeSnapshotStore{
		req: repository.Request{ID: requestID, Status: "CLARIFYING"},
		rev: repository.Revision{ID: revisionID, RequestID: requestID, SlotsJSON: []byte(`{}`)},
	}
}

func TestGetSnapshot_NoRuns_EmptyRunsNoActiveRun(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{conversationIDForRequest: 42}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetRuns()) != 0 {
		t.Fatalf("runs = %v, want empty", got.GetRuns())
	}
	if got.GetActiveRunId() != "" {
		t.Fatalf("activeRunId = %q, want empty", got.GetActiveRunId())
	}
	if got.GetOpenConfirmation() != nil {
		t.Fatalf("openConfirmation = %v, want nil", got.GetOpenConfirmation())
	}
	if len(runStore.getOpenInterruptForRunCalls) != 0 {
		t.Fatal("GetOpenInterruptForRun must not be called when there is no active run")
	}
	if len(runStore.getConversationIDForRequestCalls) != 1 || runStore.getConversationIDForRequestCalls[0] != 1 {
		t.Fatalf("GetConversationIDForRequest calls = %v, want one call with requestId 1", runStore.getConversationIDForRequestCalls)
	}
	if len(runStore.listRunsCalls) != 1 || runStore.listRunsCalls[0].conversationID != 42 || runStore.listRunsCalls[0].buyerID != 1234 {
		t.Fatalf("ListRunsByConversation calls = %+v, want one call with (42, 1234)", runStore.listRunsCalls)
	}
}

func TestGetSnapshot_ActiveRunNotWaiting_RunsAndActiveRunIdSetNoOpenConfirmation(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 900, RunType: domain.RunTypeTurn, Status: domain.RunStatusRunning(), LastEventSeq: 3, MinAvailableSeq: 1},
		},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetRuns()) != 1 {
		t.Fatalf("runs = %v, want 1 entry", got.GetRuns())
	}
	r := got.GetRuns()[0]
	if r.GetRunId() != "900" || r.GetRunType() != "TURN" || r.GetStatus() != domain.RunStatusRunning() ||
		r.GetLastSeqId() != "3" || r.GetMinAvailableSeqId() != "1" {
		t.Fatalf("run summary = %+v, want run 900 fields to round-trip", r)
	}
	if got.GetActiveRunId() != "900" {
		t.Fatalf("activeRunId = %q, want 900", got.GetActiveRunId())
	}
	if got.GetOpenConfirmation() != nil {
		t.Fatalf("openConfirmation = %v, want nil (active run isn't WAITING_FOR_INPUT)", got.GetOpenConfirmation())
	}
	if len(runStore.getOpenInterruptForRunCalls) != 0 {
		t.Fatal("GetOpenInterruptForRun must not be called for a non-WAITING_FOR_INPUT active run")
	}
}

func TestGetSnapshot_ActiveRunWaitingWithOpenInterrupt_SetsOpenConfirmation(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 901, RunType: domain.RunTypeTurn, Status: domain.RunStatusWaiting(), LastEventSeq: 5, MinAvailableSeq: 1},
		},
		hasOpenInterruptForRun: true,
		openInterruptForRun:    repository.Interrupt{ID: "int-1", RunID: 901, InterruptKind: "CLARIFICATION", Status: "PENDING"},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetActiveRunId() != "901" {
		t.Fatalf("activeRunId = %q, want 901", got.GetActiveRunId())
	}
	oc := got.GetOpenConfirmation()
	if oc == nil {
		t.Fatal("openConfirmation is nil, want a populated confirmation")
	}
	if oc.GetInterruptId() != "int-1" || oc.GetConfirmationKind() != "CLARIFICATION" || oc.GetExpiresAt() != "" {
		t.Fatalf("openConfirmation = %+v, want {interruptId:int-1 confirmationKind:CLARIFICATION expiresAt:\"\"}", oc)
	}
	if len(runStore.getOpenInterruptForRunCalls) != 1 ||
		runStore.getOpenInterruptForRunCalls[0].runID != 901 || runStore.getOpenInterruptForRunCalls[0].buyerID != 1234 {
		t.Fatalf("GetOpenInterruptForRun calls = %v, want exactly one call with (runId 901, buyerId 1234)", runStore.getOpenInterruptForRunCalls)
	}
}

func TestGetSnapshot_ActiveRunWaitingNoOpenInterrupt_OpenConfirmationNil(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 902, RunType: domain.RunTypeTurn, Status: domain.RunStatusWaiting(), LastEventSeq: 2, MinAvailableSeq: 1},
		},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetOpenConfirmation() != nil {
		t.Fatalf("openConfirmation = %v, want nil", got.GetOpenConfirmation())
	}
	if len(runStore.getOpenInterruptForRunCalls) != 1 || runStore.getOpenInterruptForRunCalls[0].runID != 902 {
		t.Fatalf("GetOpenInterruptForRun calls = %v, want exactly one call with runId 902", runStore.getOpenInterruptForRunCalls)
	}
}

func TestGetSnapshot_TerminalThenNonTerminalRun_PicksFirstNonTerminalAsActive(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 903, RunType: domain.RunTypeTurn, Status: domain.RunStatusSucceeded(), LastEventSeq: 4, MinAvailableSeq: 1},
			{ID: 904, RunType: domain.RunTypeTurn, Status: domain.RunStatusRunning(), LastEventSeq: 2, MinAvailableSeq: 1},
		},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetRuns()) != 2 {
		t.Fatalf("runs = %v, want 2 entries (both runs still surface in the list)", got.GetRuns())
	}
	if got.GetActiveRunId() != "904" {
		t.Fatalf("activeRunId = %q, want 904 (the first non-terminal run encountered)", got.GetActiveRunId())
	}
}

func TestGetSnapshot_CandidateSearchZombieRun_DoesNotShadowActiveTurnRun(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 910, RunType: domain.RunTypeCandidateSearch, Status: domain.RunStatusQueued(), LastEventSeq: 1, MinAvailableSeq: 1},
			{ID: 911, RunType: domain.RunTypeTurn, Status: domain.RunStatusWaiting(), LastEventSeq: 5, MinAvailableSeq: 1},
		},
		hasOpenInterruptForRun: true,
		openInterruptForRun:    repository.Interrupt{ID: "int-2", RunID: 911, InterruptKind: "CLARIFICATION", Status: "PENDING"},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetRuns()) != 2 {
		t.Fatalf("runs = %v, want both runs still listed (only the active-run scan excludes CANDIDATE_SEARCH, not runs[] itself)", got.GetRuns())
	}
	if got.GetActiveRunId() != "911" {
		t.Fatalf("activeRunId = %q, want 911 (the TURN run) — a zombie QUEUED CANDIDATE_SEARCH row must never win", got.GetActiveRunId())
	}
	if got.GetOpenConfirmation() == nil || got.GetOpenConfirmation().GetInterruptId() != "int-2" {
		t.Fatalf("openConfirmation = %v, want the TURN run's interrupt int-2", got.GetOpenConfirmation())
	}
}

func TestGetSnapshot_ClarificationResumeRunWaiting_CountsAsActive(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 912, RunType: domain.RunTypeClarificationResume, Status: domain.RunStatusWaiting(), LastEventSeq: 3, MinAvailableSeq: 1},
			{ID: 913, RunType: domain.RunTypeTurn, Status: domain.RunStatusResumed(), LastEventSeq: 5, MinAvailableSeq: 1},
		},
		hasOpenInterruptForRun: true,
		openInterruptForRun:    repository.Interrupt{ID: "int-3", RunID: 912, InterruptKind: "CLARIFICATION", Status: "PENDING"},
	}
	s := newSnapshotTestServer(snapshot, runStore)

	got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetActiveRunId() != "912" {
		t.Fatalf("activeRunId = %q, want 912 (the newer CLARIFICATION_RESUME run — the older parent TURN run is RESUMED, a terminal status, so it wouldn't win even if the scan reached it)", got.GetActiveRunId())
	}
	if got.GetOpenConfirmation() == nil || got.GetOpenConfirmation().GetInterruptId() != "int-3" {
		t.Fatalf("openConfirmation = %v, want the resume child run's interrupt int-3", got.GetOpenConfirmation())
	}
}

func TestGetSnapshot_ListRunsByConversationError_PropagatesAsError(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{conversationIDForRequest: 42, listRunsErr: errors.New("db unavailable")}
	s := newSnapshotTestServer(snapshot, runStore)

	_, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}

func TestGetSnapshot_GetOpenInterruptForRunError_PropagatesAsError(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{
		conversationIDForRequest: 42,
		runsByConversation: []repository.Run{
			{ID: 905, RunType: domain.RunTypeTurn, Status: domain.RunStatusWaiting(), LastEventSeq: 1, MinAvailableSeq: 1},
		},
		openInterruptForRunErr: errors.New("db unavailable"),
	}
	s := newSnapshotTestServer(snapshot, runStore)

	_, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}

func TestGetSnapshot_GetConversationIDForRequestError_PropagatesAsError(t *testing.T) {
	snapshot := emptySlotsSnapshot(1, 2)
	runStore := &fakeRunStore{conversationIDForRequestErr: errors.New("db unavailable")}
	s := newSnapshotTestServer(snapshot, runStore)

	_, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if len(runStore.listRunsCalls) != 0 {
		t.Fatal("ListRunsByConversation must not be called once GetConversationIDForRequest fails")
	}
}

func TestPreviewRfqPublication_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/PreviewRfqPublication"]
	if !ok {
		t.Fatal("PreviewRfqPublication not registered")
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestPreviewRfqPublication_NoActorInContext_ReturnsError(t *testing.T) {
	s := newBarePreviewRfqPublicationServer()
	_, err := s.PreviewRfqPublication(context.Background(), &aipb.RfqPublicationPreviewRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("status code = %v, want PermissionDenied", got)
	}
}

func TestPreviewRfqPublication_MissingPreviewClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := newBarePreviewRfqPublicationServer()
	_, err := s.PreviewRfqPublication(buyerActorContext("1"), &aipb.RfqPublicationPreviewRequest{
		RequestId: "1", ExpectedRevisionId: "1", ConfirmedRfqSnapshotId: "1", AudienceMode: "WISH_POOL",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPublication_InvalidRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := newBarePreviewRfqPublicationServer()
	_, err := s.PreviewRfqPublication(buyerActorContext("1"), &aipb.RfqPublicationPreviewRequest{
		RequestId: "not-a-number", PreviewClientRequestId: "k1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPublication_InvalidSelectedCandidateId_ReturnsInvalidArgument(t *testing.T) {
	s := newBarePreviewRfqPublicationServer()
	_, err := s.PreviewRfqPublication(buyerActorContext("1"), &aipb.RfqPublicationPreviewRequest{
		RequestId: "1", PreviewClientRequestId: "k1", ExpectedRevisionId: "1",
		ConfirmedRfqSnapshotId: "1", AudienceMode: "DIRECT_RECIPIENTS",
		CandidateSetId: "900", SelectedCandidateIds: []string{"11", "not-a-number"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestPreviewRfqPublicationErrorStatus_MapsTypedErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantReason string
	}{
		{"revision stale", commerce.ErrRevisionStale, codes.Aborted, "STALE_REVISION"},
		{"confirmed snapshot stale", commerce.ErrConfirmedSnapshotStale, codes.Aborted, "APPROVAL_STALE"},
		{"candidate set mismatch", commerce.ErrCandidateSetMismatch, codes.InvalidArgument, "INVALID_ARGUMENT"},
		{"unknown audience mode", commerce.ErrUnknownAudienceMode, codes.InvalidArgument, "INVALID_ARGUMENT"},
		{"candidate identity unresolved", commerce.ErrCandidateIdentityUnresolved, codes.Internal, "INTERNAL_ERROR"},
		{"malformed response deadline", commerce.ErrInvalidResponseDeadline, codes.InvalidArgument, "INVALID_ARGUMENT"},
		{"snapshot not found", repository.ErrConfirmedRfqSnapshotNotFound, codes.NotFound, "RESOURCE_NOT_FOUND"},
		{"price preview not found", repository.ErrPricePreviewNotFound, codes.NotFound, "RESOURCE_NOT_FOUND"},
		{"idempotency conflict", repository.ErrPublicationApprovalIdempotencyConflict, codes.Aborted, "IDEMPOTENCY_KEY_CONFLICT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := previewRfqPublicationErrorStatus(context.Background(), tc.err)
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("not a status error: %v", err)
			}
			if st.Code() != tc.wantCode {
				t.Errorf("code = %v, want %v", st.Code(), tc.wantCode)
			}
			var reason string
			for _, d := range st.Details() {
				if be, ok := d.(*aipb.BusinessError); ok {
					reason = be.GetReason()
				}
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func newBarePreviewRfqPublicationServer() *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func TestExecuteRfqPublication_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/ExecuteRfqPublication"]
	if !ok {
		t.Fatal("ExecuteRfqPublication not registered")
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestExecuteRfqPublication_NoActorInContext_ReturnsError(t *testing.T) {
	s := newBareExecuteRfqPublicationServer()
	_, err := s.ExecuteRfqPublication(context.Background(), &aipb.RfqPublicationExecuteRequest{RequestId: "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("status code = %v, want PermissionDenied", got)
	}
}

func TestExecuteRfqPublication_MissingClientRequestId_ReturnsInvalidArgument(t *testing.T) {
	s := newBareExecuteRfqPublicationServer()
	_, err := s.ExecuteRfqPublication(buyerActorContext("1"), &aipb.RfqPublicationExecuteRequest{
		RequestId: "1", ApprovalId: "1", ApprovalVersion: "1", ExpectedRevisionId: "1", PayloadHash: "h",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", got)
	}
}

func TestExecuteRfqPublicationErrorStatus_MapsTypedErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   codes.Code
		wantReason string
	}{
		{"approval stale", commerce.ErrApprovalStale, codes.Aborted, "APPROVAL_STALE"},
		{"revision stale", commerce.ErrRevisionStale, codes.Aborted, "STALE_REVISION"},
		{"payload hash mismatch", commerce.ErrPayloadHashMismatch, codes.Aborted, "APPROVAL_STALE"},
		{"approval expired", commerce.ErrApprovalExpired, codes.Aborted, "APPROVAL_STALE"},
		{"idempotency conflict", repository.ErrPublicationCommandConflict, codes.Aborted, "IDEMPOTENCY_KEY_CONFLICT"},
		{"unknown audience mode", commerce.ErrUnknownAudienceMode, codes.Internal, "INTERNAL_ERROR"},
		{"publication not found", repository.ErrPublicationNotFound, codes.Internal, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := executeRfqPublicationErrorStatus(context.Background(), tc.err)
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("not a status error: %v", err)
			}
			if st.Code() != tc.wantCode {
				t.Errorf("code = %v, want %v", st.Code(), tc.wantCode)
			}
			var reason string
			for _, d := range st.Details() {
				if be, ok := d.(*aipb.BusinessError); ok {
					reason = be.GetReason()
				}
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func newBareExecuteRfqPublicationServer() *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func TestProcurementCommandStatus_RunResultNotReady_IsFailedPrecondition(t *testing.T) {
	err := procurementUsecaseErrorStatus(context.Background(), usecase.ErrRunResultUnavailable)
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("回傳值不是 gRPC status: %v", err)
	}

	if got := st.Code(); got != codes.FailedPrecondition {
		t.Errorf("code = %v，預期 FailedPrecondition", got)
	}
	if st.Code() == codes.Unavailable {
		t.Error("回了 codes.Unavailable —— 那會被 gateway 的 circuit breaker 計入 transport 失敗")
	}
	if !strings.Contains(st.Message(), "尚未就緒") {
		t.Errorf("message = %q，預期含「尚未就緒」", st.Message())
	}
	if want := codemeta.ProcurementErrorReasonRunResultNotReady.Value; want == "" {
		t.Error("codemeta 的 RunResultNotReady 值為空 —— 碼表可能被改名")
	}
}

func f64p(f float64) *float64 { return &f }

func TestGetSnapshot_PublishedPublication(t *testing.T) {
	publishedAt := time.Date(2026, 9, 14, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		name    string
		store   fakeSnapshotStore
		wantNil bool
		wantID  string
	}{
		{
			name:    "從未發布過：欄位不得出現，也不得回一個零值物件",
			store:   emptySlotsSnapshot(1, 2),
			wantNil: true,
		},
		{
			name: "發布過：帶身分與時間",
			store: func() fakeSnapshotStore {
				s := emptySlotsSnapshot(1, 2)
				s.published = repository.LatestPublicationForRequest{
					ID: 777, Version: 3, PublishedAt: publishedAt,
				}
				return s
			}(),
			wantID: "777",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSnapshotTestServer(tc.store, &fakeRunStore{conversationIDForRequest: 42})
			got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			pp := got.GetPublishedPublication()
			if tc.wantNil {
				if pp != nil {
					t.Fatalf("publishedPublication = %+v, want nil（沒發布過就不該有這個欄位）", pp)
				}
				return
			}
			if pp == nil {
				t.Fatal("publishedPublication = nil, want a value")
			}
			if pp.GetPublicationId() != tc.wantID {
				t.Fatalf("publicationId = %q, want %q", pp.GetPublicationId(), tc.wantID)
			}
			if pp.GetVersion() != "3" {
				t.Fatalf("version = %q, want \"3\"", pp.GetVersion())
			}
			if pp.GetPublishedAt() != "2026-09-14T03:04:05Z" {
				t.Fatalf("publishedAt = %q", pp.GetPublishedAt())
			}
		})
	}
}
