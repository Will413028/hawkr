package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/handler"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type AIProcurementGrpcServer struct {
	aipb.UnimplementedAIProcurementServiceServer
	snapshot              handler.SnapshotReader
	candidateSearch       *usecase.CandidateSearchUseCase
	selection             *usecase.SelectionUseCase
	answer                *usecase.AnswerUseCase
	rewind                *usecase.RewindUseCase
	crop                  *cropidentity.DisambiguationUseCase
	conversation          *usecase.ConversationUseCase
	previewRfqPrice       *pricing.PreviewRfqPriceUseCase
	confirmRfqSnapshot    *commerce.ConfirmRfqSnapshotUseCase
	previewRfqPublication *commerce.PublicationPreviewUseCase
	executeRfqPublication *commerce.PublicationExecuteUseCase
	durableRun            DurableRunDeps
	runStore              RunStore
	graphController       usecase.GraphControllerDeps
	inviteSellerView      *inviteSellerViewHandler
	sellerCommands        SellerCommandDeps
}

type SellerCommandDeps struct {
	SellerResponse  *commerce.SellerResponseUseCase
	BuyerAcceptance *commerce.BuyerAcceptanceUseCase
}

type DurableRunDeps struct {
	RunStore   usecase.ResumeRunStore
	Dispatcher *usecase.RunDispatcher
}

func NewAIProcurementGrpcServer(
	snapshot handler.SnapshotReader,
	candidateSearch *usecase.CandidateSearchUseCase,
	selection *usecase.SelectionUseCase,
	answer *usecase.AnswerUseCase,
	rewind *usecase.RewindUseCase,
	crop *cropidentity.DisambiguationUseCase,
	conversation *usecase.ConversationUseCase,
	previewRfqPrice *pricing.PreviewRfqPriceUseCase,
	confirmRfqSnapshot *commerce.ConfirmRfqSnapshotUseCase,
	previewRfqPublication *commerce.PublicationPreviewUseCase,
	executeRfqPublication *commerce.PublicationExecuteUseCase,
	durableRun DurableRunDeps,
	runStore RunStore,
	graphController usecase.GraphControllerDeps,
	inviteSellerViewRepo InviteSellerViewRepository,
	sellerCommands SellerCommandDeps,
) *AIProcurementGrpcServer {
	return &AIProcurementGrpcServer{
		snapshot: snapshot, candidateSearch: candidateSearch,
		selection: selection, answer: answer, rewind: rewind, crop: crop,
		conversation: conversation, previewRfqPrice: previewRfqPrice,
		confirmRfqSnapshot: confirmRfqSnapshot, previewRfqPublication: previewRfqPublication,
		executeRfqPublication: executeRfqPublication,
		durableRun:            durableRun, runStore: runStore,
		graphController:  graphController,
		inviteSellerView: newInviteSellerViewHandler(inviteSellerViewRepo),
		sellerCommands:   sellerCommands,
	}
}

func (s *AIProcurementGrpcServer) GetRfqPublicationInviteSellerView(ctx context.Context, req *aipb.GetRfqPublicationInviteSellerViewRequest) (*aipb.RfqPublicationInviteSellerViewResponse, error) {
	return s.inviteSellerView.GetRfqPublicationInviteSellerView(ctx, req)
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func storeIDString(id identity.StoreID) string {
	if id == 0 {
		return ""
	}
	return itoa64(int64(id))
}

func buyerIDFromContext(ctx context.Context) (identity.BuyerID, error) {
	a, ok := ActorFromContext(ctx)
	if !ok {
		return 0, procurementStatus(codes.PermissionDenied, "", "actor not resolved")
	}
	v, err := parseInt64(a.UserID)
	if err != nil {
		return 0, procurementStatus(codes.Internal, "", "invalid buyer id in resolved actor")
	}
	return identity.BuyerID(v), nil
}

func toSlotsProto(s domain.Slots) *aipb.Slots {
	out := &aipb.Slots{}
	if s.Item != nil {
		out.Item = s.Item
	}
	if s.Quantity != nil {
		v := itoa(*s.Quantity)
		out.Quantity = &v
	}
	if s.Unit != nil {
		out.Unit = s.Unit
	}
	if s.Packaging != nil {
		out.Packaging = s.Packaging
	}
	if s.BudgetPerUnit != nil {
		v := itoa(*s.BudgetPerUnit)
		out.BudgetPerUnit = &v
	}
	if s.DeliveryCity != nil {
		out.DeliveryCity = s.DeliveryCity
	}
	if s.DeliveryDate != nil {
		out.DeliveryDate = s.DeliveryDate
	}
	if s.ExactMatchRequired != nil {
		out.ExactMatchRequired = s.ExactMatchRequired
	}
	out.ItemResolution = toItemResolutionProto(s.ItemResolution)
	return out
}

func toItemResolutionProto(r *domain.ItemResolution) *aipb.ItemResolution {
	if r == nil {
		return nil
	}
	out := &aipb.ItemResolution{
		Kind:         string(r.Kind),
		Label:        r.Label,
		Provenance:   string(r.Provenance),
		DisplayValue: r.DisplayValue(),
	}
	if r.PricePerKg != nil {
		v := itoa(*r.PricePerKg)
		out.PricePerKg = &v
	}
	return out
}

func toClarifyQuestionProto(q *domain.ClarifyQuestion) *aipb.ClarifyQuestion {
	if q == nil {
		return nil
	}
	options := make([]*aipb.QuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = &aipb.QuestionOption{OptionId: o.OptionID, Label: o.Label, Value: o.Value}
	}
	unitOptions := make([]*aipb.QuestionOption, len(q.UnitOptions))
	for i, o := range q.UnitOptions {
		unitOptions[i] = &aipb.QuestionOption{OptionId: o.OptionID, Label: o.Label, Value: o.Value}
	}
	return &aipb.ClarifyQuestion{
		SlotKey: string(q.SlotKey), Prompt: q.Prompt, Options: options, UnitOptions: unitOptions,
		AllowFreeText: q.AllowFreeText, Source: q.Source, CanSkip: q.CanSkip,
	}
}

func toStepsProto(steps []domain.Step) []*aipb.Step {
	out := make([]*aipb.Step, len(steps))
	for i, s := range steps {
		out[i] = &aipb.Step{Code: string(s.Code), Status: s.Status, Label: s.Label, Text: s.Text, DurationMs: s.DurationMs, SafeArgs: s.SafeArgs}
	}
	return out
}

func toRecordedFieldsProto(fields []domain.RecordedField) []*aipb.RecordedField {
	out := make([]*aipb.RecordedField, len(fields))
	for i, f := range fields {
		out[i] = &aipb.RecordedField{Key: f.Key, Label: f.Label, Value: f.Value}
	}
	return out
}

func toRejectedFieldsProto(fields []domain.RejectedField) []*aipb.RejectedField {
	out := make([]*aipb.RejectedField, len(fields))
	for i, f := range fields {
		out[i] = &aipb.RejectedField{Key: f.Key, Reason: f.Reason}
	}
	return out
}

func toMissingRequiredProto(keys []domain.SlotKey) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func toCropDisambiguationOptionsProto(opts []domain.CropDisambiguationOption) []*aipb.CropDisambiguationOption {
	out := make([]*aipb.CropDisambiguationOption, len(opts))
	for i, o := range opts {
		out[i] = &aipb.CropDisambiguationOption{Label: o.Label}
	}
	return out
}

func toTurnResultProto(r usecase.TurnResult) *aipb.TurnResult {
	out := &aipb.TurnResult{
		RequestId: itoa64(r.RequestID), ConversationId: itoa64(r.ConversationID), RevisionId: itoa64(r.RevisionID),
		Slots: toSlotsProto(r.Slots), Rejected: toRejectedFieldsProto(r.Rejected),
		MissingRequired: toMissingRequiredProto(r.MissingRequired), RecordedThisTurn: toRecordedFieldsProto(r.RecordedThisTurn),
		Degraded: r.Degraded, DegradedReason: r.DegradedReason, Reply: r.Reply,
		Steps: toStepsProto(r.Steps), Question: toClarifyQuestionProto(r.Question),
		NextAction: r.NextAction,
		DialoguePlan: &aipb.DialoguePlanSummary{
			ActCode: string(r.DialoguePlan.ActCode), PlanHash: r.DialoguePlan.PlanHash,
			PolicyVersion: r.DialoguePlan.PolicyVersion, RenderMode: r.RenderMode,
			TemplateVersion: r.DialoguePlan.TemplateVersion,
		},
		Blocks:       toBlocksProto(r.Blocks),
		TextFallback: r.Reply,
		EvidenceRefs: nil,
		MessageId:    itoa64(r.RevisionID),
		CreatedAt:    timeutil.FormatISO8601(time.Now()),
	}
	if r.Quota.PlanCode != "" {
		out.Quota = &aipb.AgentQuotaSnapshot{
			PlanCode:  r.Quota.PlanCode,
			Remaining: itoa(r.Quota.Remaining),
			Limit:     itoa(r.Quota.Limit),
			ResetAt:   timeutil.FormatISO8601(r.Quota.ResetAt),
		}
	}
	return out
}

func toBlocksProto(blocks []domain.Block) []*aipb.Block {
	out := make([]*aipb.Block, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, &aipb.Block{Kind: string(b.Kind), Text: b.Text})
	}
	return out
}

func procurementUsecaseErrorStatus(ctx context.Context, err error) error {
	var quotaErr *usecase.QuotaExceededError
	if st := sellerResponseErrorStatus(err); st != nil {
		return st
	}
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return procurementStatus(codes.NotFound, "REQUEST_NOT_FOUND", "request not found")
	case errors.Is(err, domain.ErrIdempotencyKeyConflict), errors.Is(err, repository.ErrIdempotencyConflict),
		errors.Is(err, repository.ErrSellerResponseIdempotencyConflict),
		errors.Is(err, repository.ErrBuyerQuoteAcceptanceIdempotencyConflict):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonIdempotencyKeyConflict.Value,
			"clientRequestId 已用於其他需求，請改用新的 clientRequestId")
	case errors.As(err, &quotaErr):
		return procurementQuotaExceededStatus(quotaErr.Limit, quotaErr.ResetAt, quotaErr.Error())
	case errors.Is(err, usecase.ErrRunResultUnavailable):
		return procurementStatus(codes.FailedPrecondition, codemeta.ProcurementErrorReasonRunResultNotReady.Value,
			"這個命令已受理，結果尚未就緒，請稍後重試")
	default:
		pkglogger.Error(ctx, "procurement: unmapped usecase error surfaced as internal",
			zap.Error(err))
		return procurementStatus(codes.Internal, "", "internal error")
	}
}

func (s *AIProcurementGrpcServer) SubmitTurn(ctx context.Context, req *aipb.TurnRequest) (*aipb.TurnResult, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	if req.GetMessage() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "message is required")
	}
	if len([]rune(req.GetMessage())) > handler.MaxMessageRunes {
		return nil, procurementStatus(codes.FailedPrecondition, "INPUT_TOO_LONG", "單則訊息上限 2000 字")
	}

	var requestID int64
	if req.GetRequestId() != "" {
		parsed, err := parseInt64(req.GetRequestId())
		if err != nil {
			return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
		}
		requestID = parsed
	}

	in := usecase.TurnInput{
		BuyerID: buyerID, RequestID: requestID, Message: req.GetMessage(), ClientRequestID: req.GetClientRequestId(),
	}

	out, err := usecase.ClaimAndDispatchTurnViaGraph(ctx, in, s.graphController, usecase.TurnStreamHooks{})
	if err != nil {
		if errors.Is(err, usecase.ErrInterruptPending) {
			return nil, procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalPending.Value,
				"這個需求有一個尚未回覆的中斷，請改用 resume 回覆")
		}
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toDurableTurnResultProto(out), nil
}

func toDurableTurnResultProto(out usecase.SubmitTurnResult) *aipb.TurnResult {
	proto := toTurnResultProto(out.Result)
	proto.RunId = itoa64(out.RunID)
	proto.InterruptId = out.InterruptID
	proto.InterruptKind = out.InterruptKind
	return proto
}

func (s *AIProcurementGrpcServer) SubmitTurnStream(req *aipb.TurnRequest, stream aipb.AIProcurementService_SubmitTurnStreamServer) error {
	buyerID, err := buyerIDFromContext(stream.Context())
	if err != nil {
		return err
	}
	if req.GetClientRequestId() == "" {
		return procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	if req.GetMessage() == "" {
		return procurementStatus(codes.InvalidArgument, "", "message is required")
	}
	if len([]rune(req.GetMessage())) > handler.MaxMessageRunes {
		return procurementStatus(codes.FailedPrecondition, "INPUT_TOO_LONG", "單則訊息上限 2000 字")
	}

	var requestID int64
	if req.GetRequestId() != "" {
		parsed, err := parseInt64(req.GetRequestId())
		if err != nil {
			return procurementStatus(codes.InvalidArgument, "", "invalid requestId")
		}
		requestID = parsed
	}

	ctx := context.WithoutCancel(stream.Context())

	var sendErr error
	send := func(ev *aipb.StreamEvent) {
		if sendErr != nil {
			return
		}
		if err := stream.Send(ev); err != nil {
			sendErr = err
		}
	}

	in := usecase.TurnInput{
		BuyerID: buyerID, RequestID: requestID, Message: req.GetMessage(), ClientRequestID: req.GetClientRequestId(),
	}

	runID, interruptID, interruptKind := "", "", ""
	seq := int32(0)
	emit := func(event usecase.TurnEvent) {
		switch event.Kind {
		case usecase.TurnEventStep:
			send(&aipb.StreamEvent{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
				TurnId: req.GetClientRequestId(), Seq: seq, Code: string(event.Step.Code),
				Status: event.Step.Status, Label: event.Step.Label, DurationMs: event.Step.DurationMs,
				SafeArgs: event.Step.SafeArgs,
			}}})
			seq++
		case usecase.TurnEventResult:
			send(&aipb.StreamEvent{Payload: &aipb.StreamEvent_Message{Message: &aipb.MessageEvent{
				TurnId: req.GetClientRequestId(), Reply: event.Result.Reply,
			}}})
			seq++
			terminal := toTurnResultProto(event.Result)
			terminal.RunId = runID
			terminal.InterruptId, terminal.InterruptKind = interruptID, interruptKind
			send(&aipb.StreamEvent{Payload: &aipb.StreamEvent_TurnResult{TurnResult: terminal}})
		}
	}

	hooks := usecase.TurnStreamHooks{
		ActivityCh: make(chan []byte, 64),
		OnClaimed: func(id int64) {
			runID = itoa64(id)
			send(&aipb.StreamEvent{Payload: &aipb.StreamEvent_RunAccepted{RunAccepted: &aipb.RunAccepted{RunId: runID}}})
		},
		OnActivity: func(payload []byte) {
			send(&aipb.StreamEvent{Payload: &aipb.StreamEvent_Activity{Activity: &aipb.ActivityEvent{DataJson: payload}}})
		},
	}

	out, err := usecase.ClaimAndDispatchTurnViaGraph(ctx, in, s.graphController, hooks)
	if err != nil {
		if errors.Is(err, usecase.ErrInterruptPending) {
			return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalPending.Value,
				"這個需求有一個尚未回覆的中斷，請改用 resume 回覆")
		}
		return procurementUsecaseErrorStatus(ctx, err)
	}
	interruptID, interruptKind = out.InterruptID, out.InterruptKind
	for _, event := range out.Events {
		emit(event)
	}
	return sendErr
}

func (s *AIProcurementGrpcServer) GetSnapshot(ctx context.Context, req *aipb.GetSnapshotRequest) (*aipb.SnapshotResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}

	reqEntity, rev, err := s.snapshot.GetSnapshot(ctx, requestID, buyerID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}

	var slots domain.Slots
	if err := json.Unmarshal(rev.SlotsJSON, &slots); err != nil {
		return nil, procurementStatus(codes.Internal, "", "internal error")
	}

	conversationID, err := s.runStore.GetConversationIDForRequest(ctx, requestID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	runtimeView, err := s.buildConversationRuntimeView(ctx, conversationID, buyerID)
	if err != nil {
		return nil, err
	}
	candidates := runtimeView.Candidates

	published, pubErr := s.snapshot.FindLatestPublicationForRequest(ctx, requestID, buyerID)
	switch {
	case pubErr == nil:
	case errors.Is(pubErr, repository.ErrPublicationNotFound):
		published = repository.LatestPublicationForRequest{}
	default:
		return nil, procurementUsecaseErrorStatus(ctx, pubErr)
	}

	return &aipb.SnapshotResponse{
		RequestId: itoa64(reqEntity.ID), RevisionId: itoa64(rev.ID), Status: reqEntity.Status,
		Slots: toSlotsProto(slots), MissingRequired: toMissingRequiredProto(slots.MissingRequired()),
		CropResolution: &aipb.CropResolution{
			Candidates: toCropDisambiguationOptionsProto(candidates),
		},
		Runs: runtimeView.Runs, ActiveRunId: runtimeView.ActiveRunID,
		OpenConfirmation:     runtimeView.OpenConfirmation,
		PublishedPublication: toPublishedPublicationProto(published),
	}, nil
}

func toPublishedPublicationProto(p repository.LatestPublicationForRequest) *aipb.PublishedRfqPublication {
	if p.ID == 0 {
		return nil
	}
	return &aipb.PublishedRfqPublication{
		PublicationId: itoa64(p.ID),
		Version:       itoa32(p.Version),
		PublishedAt:   timeutil.FormatISO8601(p.PublishedAt),
	}
}

type conversationRuntimeView struct {
	Runs             []*aipb.AgentRunSummary
	ActiveRunID      string
	OpenConfirmation *aipb.AgentRunOpenConfirmation
	Candidates       []domain.CropDisambiguationOption
}

func (s *AIProcurementGrpcServer) buildConversationRuntimeView(
	ctx context.Context, conversationID int64, buyerID identity.BuyerID,
) (conversationRuntimeView, error) {
	view := conversationRuntimeView{Candidates: []domain.CropDisambiguationOption{}}

	runs, err := s.runStore.ListRunsByConversation(ctx, conversationID, buyerID)
	if err != nil {
		return conversationRuntimeView{}, procurementUsecaseErrorStatus(ctx, err)
	}
	view.Runs = make([]*aipb.AgentRunSummary, len(runs))
	var activeRunID int64
	var activeRunStatus string
	for i, run := range runs {
		view.Runs[i] = &aipb.AgentRunSummary{
			RunId: itoa64(run.ID), RunType: string(run.RunType), Status: run.Status,
			LastSeqId: itoa64(run.LastEventSeq), MinAvailableSeqId: itoa64(run.MinAvailableSeq),
			Terminal: domain.IsTerminalRunStatus(run.Status),
			// CancelRun 的 CAS token，理由見 ai.proto 的 AgentRunSummary.version。
			// run.Version 由 toRunEntity 從 procurement_agent_runs.version 直接
			// 帶出，這裡不做任何轉換或預設值——呼叫端拿到的必須是那一刻 DB 裡的
			// 真實版本，補值等於讓 CAS 失去意義。
			Version: itoa64(run.Version),
		}
		if (run.RunType == domain.RunTypeTurn || run.RunType == domain.RunTypeClarificationResume) &&
			!domain.IsTerminalRunStatus(run.Status) && activeRunID == 0 {
			activeRunID = run.ID
			activeRunStatus = run.Status
		}
	}
	if activeRunID == 0 {
		return view, nil
	}
	view.ActiveRunID = itoa64(activeRunID)
	if activeRunStatus != domain.RunStatusWaiting() {
		return view, nil
	}
	interrupt, ok, err := s.runStore.GetOpenInterruptForRun(ctx, activeRunID, buyerID)
	if err != nil {
		return conversationRuntimeView{}, procurementUsecaseErrorStatus(ctx, err)
	}
	if !ok {
		return view, nil
	}
	view.OpenConfirmation = &aipb.AgentRunOpenConfirmation{
		InterruptId: interrupt.ID, ConfirmationKind: interrupt.InterruptKind,
		ExpiresAt: "",
	}
	var payload usecase.CropDisambiguationInterruptPayload
	if err := json.Unmarshal(interrupt.PayloadJSON, &payload); err == nil && payload.Kind == string(domain.SlotCropDisambiguation) {
		if payload.Candidates != nil {
			view.Candidates = payload.Candidates
		}
		view.OpenConfirmation.Question = toClarifyQuestionProto(
			domain.BuildCropDisambiguationQuestion(payload.Candidates, payload.Selection))
	}
	return view, nil
}

func toCandidateLanesProto(candidates []repository.Candidate, selected map[int64]bool) *aipb.CandidateLanes {
	platform := make([]*aipb.CandidateItem, 0)
	external := make([]*aipb.CandidateItem, 0)
	for _, c := range candidates {
		item := &aipb.CandidateItem{
			CandidateId: itoa64(c.ID), SourceId: c.SourceID, Lane: c.Lane, DisplayName: c.DisplayName,
			Score: float64(c.Score), MatchedFactsJson: string(c.MatchedFactsJSON), UnverifiableFactsJson: string(c.UnverifiableFactsJSON),
			FactsAsOf: timeutil.FormatISO8601(c.FactsAsOf), Selected: selected[c.ID],
			StoreId:    storeIDString(c.StoreID),
			SellerName: c.SellerName,
			LogoKey:    c.LogoKey,
			StoreCode:  c.StoreCode,
			// 站外業者的品牌素材，與上面四欄的平台身分快照分屬兩軸（ai.proto 14–15）。
			BrandLogoKey:  c.BrandLogoKey,
			BrandCoverKey: c.BrandCoverKey,
		}
		if c.Lane == string(domain.LanePlatform) {
			platform = append(platform, item)
		} else {
			external = append(external, item)
		}
	}
	return &aipb.CandidateLanes{Platform: platform, External: external}
}

func (s *AIProcurementGrpcServer) StartCandidateSearch(ctx context.Context, req *aipb.StartCandidateSearchRequest) (*aipb.StartCandidateSearchResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}

	reqEntity, rev, err := s.snapshot.GetSnapshot(ctx, requestID, buyerID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	var slots domain.Slots
	if err := json.Unmarshal(rev.SlotsJSON, &slots); err != nil {
		return nil, procurementStatus(codes.Internal, "", "internal error")
	}

	result, err := s.candidateSearch.StartSearch(ctx, usecase.StartSearchInput{
		RequestID: requestID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		RequirementReady: reqEntity.Status == "REQUIREMENT_READY", Slots: slots,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrRequirementNotReady) {
			return nil, procurementStatus(codes.Aborted, "REQUIREMENT_NOT_READY", "需求尚未就緒，仍有必填欄位缺漏")
		}
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.StartCandidateSearchResponse{
		RequestId: itoa64(requestID), RunId: wireRunID(result.RunID),
		CandidateSetId: itoa64(result.CandidateSetID), Status: result.Status,
	}, nil
}

func wireRunID(runID int64) string {
	return itoa64(runID)
}

func (s *AIProcurementGrpcServer) GetCandidateSearch(ctx context.Context, req *aipb.GetCandidateSearchRequest) (*aipb.CandidateSetResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	runID, err := parseInt64(req.GetRunId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid runId")
	}

	set, candidates, selected, err := s.candidateSearch.GetRun(ctx, usecase.GetRunInput{RunID: runID, RequestID: requestID, BuyerID: buyerID})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.CandidateSetResponse{
		RequestId: itoa64(set.RequestID), RunId: itoa64(runID),
		CandidateSetId: itoa64(set.ID), Status: set.Status,
		PlatformLaneStatus: set.PlatformLaneStatus, ExternalLaneStatus: set.ExternalLaneStatus,
		FailureReason: set.FailureReason,
		Candidates:    toCandidateLanesProto(candidates, selected),
	}, nil
}

func (s *AIProcurementGrpcServer) ToggleSelection(ctx context.Context, req *aipb.ToggleSelectionRequest) (*aipb.ToggleSelectionResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	if req.GetCandidateSetId() == "" || req.GetLane() == "" || req.GetCandidateId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "candidateSetId, lane and candidateId are required")
	}
	candidateSetID, err := parseInt64(req.GetCandidateSetId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid candidateSetId")
	}
	candidateID, err := parseInt64(req.GetCandidateId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid candidateId")
	}

	result, err := s.selection.Toggle(ctx, usecase.SelectionInput{
		RequestID: requestID, BuyerID: buyerID, CandidateSetID: candidateSetID,
		Lane: req.GetLane(), CandidateID: candidateID, Selected: req.GetSelected(),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrCandidateFactsStale):
			return nil, procurementStatus(codes.Aborted, "CANDIDATE_FACTS_STALE", "候選資料已更新，請重新抓取候選並確認")
		case errors.Is(err, usecase.ErrLaneMismatch):
			return nil, procurementStatus(codes.InvalidArgument, "LANE_MISMATCH", "lane 與候選實際所屬 lane 不符")
		default:
			return nil, procurementUsecaseErrorStatus(ctx, err)
		}
	}
	return &aipb.ToggleSelectionResponse{
		RequestId: itoa64(requestID), CandidateSetId: itoa64(result.CandidateSetID), Lane: result.Lane,
		CandidateId: itoa64(result.CandidateID), Selected: result.Selected,
	}, nil
}

func (s *AIProcurementGrpcServer) SubmitAnswer(ctx context.Context, req *aipb.SubmitAnswerRequest) (*aipb.TurnResult, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	if req.GetSlotKey() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "slotKey is required")
	}
	isQuantityAnswer := req.GetSlotKey() == string(domain.SlotQuantity) && !req.GetSkip()
	isCropDisambiguationAnswer := req.GetSlotKey() == string(domain.SlotCropDisambiguation) && !req.GetSkip()
	switch {
	case isQuantityAnswer:
		if req.GetOptionId() != "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "quantity 不接受 optionId")
		}
		if req.GetFreeText() == "" && req.GetUnitOptionId() == "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "freeText 或 unitOptionId 必須至少擇一")
		}
	case isCropDisambiguationAnswer:
		if req.GetUnitOptionId() != "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "unitOptionId 只能在回答 quantity 時使用")
		}
		if req.GetOptionId() == "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "crop_disambiguation 必須提供 optionId")
		}
		if req.GetOptionId() == domain.CropOtherOptionID {
			if req.GetFreeText() == "" {
				return nil, procurementStatus(codes.InvalidArgument, "", "crop_disambiguation 選擇 crop-other 時 freeText 為必填")
			}
		} else if req.GetFreeText() != "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "crop_disambiguation 選擇非 crop-other 選項時不可帶 freeText")
		}
	default:
		if req.GetUnitOptionId() != "" {
			return nil, procurementStatus(codes.InvalidArgument, "", "unitOptionId 只能在回答 quantity 時使用")
		}
		answerFieldsSet := 0
		if req.GetOptionId() != "" {
			answerFieldsSet++
		}
		if req.GetFreeText() != "" {
			answerFieldsSet++
		}
		if req.GetSkip() {
			answerFieldsSet++
		}
		if answerFieldsSet == 0 {
			return nil, procurementStatus(codes.InvalidArgument, "", "optionId, freeText 或 skip 必須擇一")
		}
		if answerFieldsSet > 1 {
			return nil, procurementStatus(codes.InvalidArgument, "", "optionId, freeText 和 skip 最多只能擇一")
		}
	}
	expectedRevisionID, err := parseInt64(req.GetExpectedRevisionId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRevisionId")
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	in := usecase.AnswerInput{
		RequestID: requestID, BuyerID: buyerID, SlotKey: domain.SlotKey(req.GetSlotKey()),
		OptionID: req.GetOptionId(), FreeText: req.GetFreeText(), UnitOptionID: req.GetUnitOptionId(),
		SelfEnteredPriceAmount: req.GetSelfEnteredPriceAmount(),
		Skip:                   req.GetSkip(), ExpectedRevisionID: expectedRevisionID, ClientRequestID: req.GetClientRequestId(),
	}

	out, err := s.answer.ClaimAndDispatchAnswer(ctx, in, s.durableRun.RunStore, s.durableRun.Dispatcher)
	if err != nil {
		return nil, answerUsecaseErrorStatus(ctx, err)
	}
	return toDurableTurnResultProto(out), nil
}

func answerUsecaseErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrStaleRevision):
		return procurementStatus(codes.Aborted, "STALE_REVISION", "需求已被其他裝置改動")
	case errors.Is(err, usecase.ErrInvalidOptionID):
		return procurementStatus(codes.FailedPrecondition, "ANSWER_SCHEMA_INVALID", "答案不符合這個欄位的格式")
	case errors.Is(err, usecase.ErrCannotSkipQuoteRequiredSlot):
		return procurementStatus(codes.FailedPrecondition, "SLOT_NOT_SKIPPABLE", "此欄位為必要資訊，無法跳過")
	case errors.Is(err, usecase.ErrInterruptPending):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalPending.Value,
			"這個需求有一個尚未回覆的中斷，請改用 resume 回覆")
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func (s *AIProcurementGrpcServer) Rewind(ctx context.Context, req *aipb.RewindRequest) (*aipb.TurnResult, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	if req.GetSlotKey() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "slotKey is required")
	}
	in := usecase.RewindInput{
		RequestID: requestID, BuyerID: buyerID, SlotKey: domain.SlotKey(req.GetSlotKey()), ClientRequestID: req.GetClientRequestId(),
	}

	out, err := s.rewind.ClaimAndDispatchRewind(ctx, in, s.durableRun.RunStore, s.durableRun.Dispatcher)
	if err != nil {
		return nil, rewindUsecaseErrorStatus(ctx, err)
	}
	return toDurableTurnResultProto(out), nil
}

func rewindUsecaseErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrSlotNeverWritten):
		return procurementStatus(codes.FailedPrecondition, "ANSWER_SCHEMA_INVALID", "這個欄位從未被填寫過，無法回溯")
	case errors.Is(err, usecase.ErrInterruptPending):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalPending.Value,
			"這個需求有一個尚未回覆的中斷，請改用 resume 回覆")
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func (s *AIProcurementGrpcServer) PreviewRfqPrice(ctx context.Context, req *aipb.RfqPricePreviewRequest) (*aipb.RfqPricePreviewResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	expectedRevisionID, err := parseInt64(req.GetExpectedRevisionId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRevisionId")
	}

	var buyerRange *pricing.BuyerRequestedRange
	if r := req.GetBuyerRequestedRange(); r != nil {
		buyerRange = &pricing.BuyerRequestedRange{
			Currency: r.GetCurrency(), PriceComponent: r.GetPriceComponent(),
			LowerBoundAmount: r.GetLowerBoundAmount(), UpperBoundAmount: r.GetUpperBoundAmount(),
		}
	}

	result, err := s.previewRfqPrice.PreviewRfqPrice(ctx, pricing.PreviewRfqPriceInput{
		RequestID: requestID, BuyerID: buyerID, ClientRequestID: req.GetClientRequestId(),
		ExpectedRevisionID: expectedRevisionID,
		DeliveryCity:       req.GetDeliveryCity(), BuyerRequestedRange: buyerRange,
	})
	if err != nil {
		return nil, previewRfqPriceErrorStatus(ctx, err)
	}
	return toRfqPricePreviewResponseProto(result), nil
}

func previewRfqPriceErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, pricing.ErrPriceBasisUnavailable):
		return procurementStatus(codes.FailedPrecondition, "PRICE_BASIS_UNAVAILABLE", "currency/quantity/unit/conversion basis cannot be canonicalized")
	case errors.Is(err, pricing.ErrStalePricePreviewRevision):
		return procurementStatus(codes.Aborted, "STALE_REVISION", "需求已被其他裝置改動")
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func formatPriceAmount(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func toRfqPricePreviewResponseProto(r pricing.PricePreviewResult) *aipb.RfqPricePreviewResponse {
	resp := &aipb.RfqPricePreviewResponse{
		PricePreviewId: itoa64(r.PricePreviewID), Version: itoa64(r.Version),
		QualityKnowledgeState: r.QualityKnowledgeState,
		PayloadHash:           r.PayloadHash,
	}
	if !r.BuyerRequestedRange.IsUnset() {
		resp.BuyerRequestedRange = &aipb.BuyerRequestedRange{
			Currency: r.BuyerRequestedRange.Currency, PriceComponent: r.BuyerRequestedRange.PriceComponent,
			LowerBoundAmount: r.BuyerRequestedRange.LowerBoundAmount, UpperBoundAmount: r.BuyerRequestedRange.UpperBoundAmount,
			BasisQuantity: r.BuyerRequestedRange.BasisQuantity, BasisUnit: r.BuyerRequestedRange.BasisUnit,
		}
	}
	if q := r.QuoteTotalRange; q != nil {
		resp.QuoteTotalRange = &aipb.QuoteTotalRange{
			Currency:         q.Currency,
			LowerBoundAmount: q.LowerBoundAmount,
			UpperBoundAmount: q.UpperBoundAmount,
			Quantity:         q.Quantity,
			QuantityUnit:     q.QuantityUnit,
		}
	}
	return resp
}

func (s *AIProcurementGrpcServer) ConfirmRfqSnapshot(ctx context.Context, req *aipb.RfqConfirmationRequest) (*aipb.RfqConfirmationResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	expectedRevisionID, err := parseInt64(req.GetExpectedRevisionId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRevisionId")
	}
	pricePreviewID, err := parseInt64(req.GetPricePreviewId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid pricePreviewId")
	}
	pricePreviewVersion, err := parseInt64(req.GetPricePreviewVersion())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid pricePreviewVersion")
	}

	result, err := s.confirmRfqSnapshot.ConfirmRfqSnapshot(ctx, commerce.ConfirmRfqSnapshotInput{
		RequestID: requestID, BuyerID: buyerID, ClientRequestID: req.GetClientRequestId(),
		ExpectedRevisionID: expectedRevisionID, PricePreviewID: pricePreviewID,
		PricePreviewVersion: pricePreviewVersion, PayloadHash: req.GetPayloadHash(),
		BuyerNote: req.GetBuyerNote(),
	})
	if err != nil {
		return nil, confirmRfqSnapshotErrorStatus(ctx, err)
	}
	return &aipb.RfqConfirmationResponse{
		ConfirmedRfqSnapshotId: itoa64(result.ConfirmedRfqSnapshotID), PayloadHash: result.PayloadHash,
		ExpiresAt: result.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
}

func confirmRfqSnapshotErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, pricing.ErrStalePricePreviewRevision):
		return procurementStatus(codes.Aborted, "STALE_REVISION", "需求已被其他裝置改動")
	case errors.Is(err, commerce.ErrStalePricePreview):
		return procurementStatus(codes.Aborted, "STALE_REVISION", "報價預覽已過期，請重新試算")
	case errors.Is(err, repository.ErrPricePreviewNotFound):
		return procurementStatus(codes.NotFound, "RESOURCE_NOT_FOUND", "報價預覽不存在")
	case errors.Is(err, commerce.ErrBuyerNoteTooLong):
		return procurementStatus(codes.InvalidArgument, "",
			fmt.Sprintf("補充說明最多 %d 個字", domain.MaxBuyerNoteLen))
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func (s *AIProcurementGrpcServer) PreviewRfqPublication(ctx context.Context, req *aipb.RfqPublicationPreviewRequest) (*aipb.RfqPublicationPreviewResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetPreviewClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "previewClientRequestId is required")
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	expectedRevisionID, err := parseInt64(req.GetExpectedRevisionId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRevisionId")
	}
	confirmedRfqSnapshotID, err := parseInt64(req.GetConfirmedRfqSnapshotId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid confirmedRfqSnapshotId")
	}
	var candidateSetID int64
	if raw := req.GetCandidateSetId(); raw != "" {
		candidateSetID, err = parseInt64(raw)
		if err != nil {
			return nil, procurementStatus(codes.InvalidArgument, "", "invalid candidateSetId")
		}
	}
	selected := make([]int64, 0, len(req.GetSelectedCandidateIds()))
	for _, raw := range req.GetSelectedCandidateIds() {
		id, parseErr := parseInt64(raw)
		if parseErr != nil {
			return nil, procurementStatus(codes.InvalidArgument, "", "invalid selectedCandidateIds")
		}
		selected = append(selected, id)
	}

	result, err := s.previewRfqPublication.PreviewRfqPublication(ctx, commerce.PreviewRfqPublicationInput{
		BuyerID: buyerID, RequestID: requestID,
		PreviewClientRequestID: req.GetPreviewClientRequestId(),
		ExpectedRevisionID:     expectedRevisionID,
		ConfirmedRfqSnapshotID: confirmedRfqSnapshotID,
		AudienceMode:           req.GetAudienceMode(),
		CandidateSetID:         candidateSetID,
		SelectedCandidateIDs:   selected,
		ResponseDeadline:       req.ResponseDeadline,
	})
	if err != nil {
		return nil, previewRfqPublicationErrorStatus(ctx, err)
	}
	return &aipb.RfqPublicationPreviewResponse{
		ApprovalId:              itoa64(result.ApprovalID),
		ApprovalVersion:         itoa(result.ApprovalVersion),
		AudienceHash:            result.AudienceHash,
		SellerFacingPayloadHash: result.SellerFacingPayloadHash,
		SellerFacingPayloadJson: string(result.SellerFacingPayloadJSON),
		ExpiresAt:               result.ExpiresAt.UTC().Format(time.RFC3339),
		AudiencePolicyId:        result.AudiencePolicyID,
		AudiencePolicyVersion:   itoa(result.AudiencePolicyVersion),
		RecipientCount:          itoa(result.RecipientCount),
	}, nil
}

func previewRfqPublicationErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, commerce.ErrRevisionStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStaleRevision.Value,
			"需求已被其他裝置改動")
	case errors.Is(err, commerce.ErrConfirmedSnapshotStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
			"這份定案內容已不對應目前的需求，請重新確認")
	case errors.Is(err, commerce.ErrCandidateIdentityUnresolved):
		return procurementStatus(codes.Internal, codemeta.ProcurementErrorReasonInternalError.Value,
			"候選資料異常，請稍後再試")
	case errors.Is(err, commerce.ErrCandidateSetMismatch):
		return procurementStatus(codes.InvalidArgument, codemeta.ProcurementErrorReasonInvalidArgument.Value,
			"發布對象與這次需求對不上，請重新選擇")
	case errors.Is(err, commerce.ErrInvalidResponseDeadline):
		return procurementStatus(codes.InvalidArgument, codemeta.ProcurementErrorReasonInvalidArgument.Value,
			"報價截止日格式須為 YYYY-MM-DD")
	case errors.Is(err, commerce.ErrUnknownAudienceMode):
		return procurementStatus(codes.InvalidArgument, codemeta.ProcurementErrorReasonInvalidArgument.Value,
			"不支援的發布對象種類")
	case errors.Is(err, repository.ErrConfirmedRfqSnapshotNotFound):
		return procurementStatus(codes.NotFound, codemeta.ProcurementErrorReasonResourceNotFound.Value,
			"定案內容不存在")
	case errors.Is(err, repository.ErrPricePreviewNotFound):
		return procurementStatus(codes.NotFound, codemeta.ProcurementErrorReasonResourceNotFound.Value,
			"報價預覽不存在")
	case errors.Is(err, repository.ErrPublicationApprovalIdempotencyConflict):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonIdempotencyKeyConflict.Value,
			"previewClientRequestId 已用於其他核准，請改用新的 previewClientRequestId")
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func (s *AIProcurementGrpcServer) ExecuteRfqPublication(ctx context.Context, req *aipb.RfqPublicationExecuteRequest) (*aipb.RfqPublicationExecuteResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	approvalID, err := parseInt64(req.GetApprovalId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid approvalId")
	}
	approvalVersion, err := strconv.Atoi(req.GetApprovalVersion())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid approvalVersion")
	}
	expectedRevisionID, err := parseInt64(req.GetExpectedRevisionId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRevisionId")
	}

	result, err := s.executeRfqPublication.ExecuteRfqPublication(ctx, commerce.ExecuteRfqPublicationInput{
		BuyerID: buyerID, RequestID: requestID, ClientRequestID: req.GetClientRequestId(),
		ApprovalID: approvalID, ApprovalVersion: approvalVersion,
		ExpectedRevisionID: expectedRevisionID, PayloadHash: req.GetPayloadHash(),
	})
	if err != nil {
		return nil, executeRfqPublicationErrorStatus(ctx, err)
	}
	return &aipb.RfqPublicationExecuteResponse{
		PublicationId:           itoa64(result.PublicationID),
		PublicationVersion:      itoa(result.PublicationVersion),
		State:                   result.State,
		AudienceMode:            result.AudienceMode,
		AudienceHash:            result.AudienceHash,
		SellerFacingPayloadHash: result.SellerFacingPayloadHash,
		PublishedAt:             result.PublishedAt.UTC().Format(time.RFC3339),
		Replayed:                result.Replayed,
	}, nil
}

func executeRfqPublicationErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, commerce.ErrApprovalStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
			"這份核准已經不是可執行的那一份，請重新確認後再試")
	case errors.Is(err, commerce.ErrPayloadHashMismatch):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
			"payloadHash 與核准當下的內容不符，請重新確認後再試")
	case errors.Is(err, commerce.ErrApprovalExpired):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
			"這份核准已經過期，請重新確認後再試")
	case errors.Is(err, commerce.ErrRevisionStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStaleRevision.Value,
			"需求已被其他裝置改動")
	case errors.Is(err, pricing.ErrPriceBasisUnavailable):
		return procurementStatus(codes.FailedPrecondition, codemeta.ProcurementErrorReasonPriceBasisUnavailable.Value,
			"這份核准沒有可用的價格依據，請重新試算價格後再核准一次")
	case errors.Is(err, repository.ErrPublicationCommandConflict):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonIdempotencyKeyConflict.Value,
			"clientRequestId 已用於其他發布，請改用新的 clientRequestId")
	case errors.Is(err, commerce.ErrUnknownAudienceMode):
		return procurementStatus(codes.Internal, codemeta.ProcurementErrorReasonInternalError.Value,
			"發布對象資料異常，請稍後再試")
	case errors.Is(err, repository.ErrPublicationNotFound):
		return procurementStatus(codes.Internal, codemeta.ProcurementErrorReasonInternalError.Value,
			"發布資料異常，請稍後再試")
	default:
		return procurementUsecaseErrorStatus(ctx, err)
	}
}

func (s *AIProcurementGrpcServer) ListConversations(ctx context.Context, req *aipb.ListConversationsRequest) (*aipb.ListConversationsResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pageSize := 20
	if req.GetPageSize() != "" {
		if n, err := parseInt64(req.GetPageSize()); err == nil {
			pageSize = int(n)
		}
	}
	result, err := s.conversation.List(ctx, buyerID, req.GetCursorId(), pageSize)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	records := make([]*aipb.ConversationSummary, len(result.Records))
	for i, r := range result.Records {
		records[i] = &aipb.ConversationSummary{
			ConversationId: itoa64(r.ConversationID), Title: r.Title, LastMessagePreview: r.LastMessagePreview,
			UpdatedAt: timeutil.FormatISO8601(r.UpdatedAt),
		}
	}
	return &aipb.ListConversationsResponse{Records: records, HasMore: result.HasMore, NextCursorId: result.NextCursor}, nil
}

func (s *AIProcurementGrpcServer) GetConversationMessages(ctx context.Context, req *aipb.GetConversationMessagesRequest) (*aipb.ConversationMessagesResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := parseInt64(req.GetConversationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid conversationId")
	}
	result, err := s.conversation.Messages(ctx, conversationID, buyerID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	records := make([]*aipb.ConversationMessage, len(result.Records))
	for i, r := range result.Records {
		records[i] = &aipb.ConversationMessage{
			Id: r.ID, Role: r.Role, Text: r.Text, Steps: toStepsProto(r.Steps),
			RecordedThisTurn: toRecordedFieldsProto(r.RecordedThisTurn),
			CreatedAt:        timeutil.FormatISO8601(r.CreatedAt),
		}
	}
	return &aipb.ConversationMessagesResponse{
		ConversationId: itoa64(result.ConversationID), RequestId: itoa64(result.RequestID),
		Status: result.Status, Title: result.Title, Records: records,
	}, nil
}

func (s *AIProcurementGrpcServer) GetRuntimeSnapshot(ctx context.Context, req *aipb.GetRuntimeSnapshotRequest) (*aipb.RuntimeSnapshotResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := parseInt64(req.GetConversationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid conversationId")
	}

	result, err := s.conversation.Messages(ctx, conversationID, buyerID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	messages := make([]*aipb.ConversationMessage, len(result.Records))
	for i, r := range result.Records {
		messages[i] = &aipb.ConversationMessage{
			Id: r.ID, Role: r.Role, Text: r.Text, Steps: toStepsProto(r.Steps),
			RecordedThisTurn: toRecordedFieldsProto(r.RecordedThisTurn),
			CreatedAt:        timeutil.FormatISO8601(r.CreatedAt),
		}
	}

	runtimeView, err := s.buildConversationRuntimeView(ctx, conversationID, buyerID)
	if err != nil {
		return nil, err
	}

	artifacts, err := s.runStore.ListLatestArtifactsByConversation(ctx, conversationID, buyerID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	artifactViews := make([]*aipb.AgentRunArtifactView, len(artifacts))
	for i, a := range artifacts {
		artifactViews[i] = &aipb.AgentRunArtifactView{
			ArtifactId: itoa64(a.ArtifactID), RunId: itoa64(a.RunID),
			ArtifactType:  a.ArtifactType,
			Version:       strconv.FormatInt(int64(a.Version), 10),
			Status:        a.Status,
			DataJson:      string(a.PublicDataJSON),
			Checksum:      a.Checksum,
			DomainRefType: a.DomainRefType, DomainRefId: a.DomainRefID,
			AsOf:      timeutil.FormatISO8601(a.AsOf),
			CreatedAt: timeutil.FormatISO8601(a.CreatedAt),
		}
	}

	return &aipb.RuntimeSnapshotResponse{
		ConversationId: itoa64(result.ConversationID), RequestId: itoa64(result.RequestID),
		Status: result.Status, Title: result.Title,
		Messages: messages, Runs: runtimeView.Runs,
		ActiveRunId: runtimeView.ActiveRunID, Artifacts: artifactViews,
		OpenConfirmation: runtimeView.OpenConfirmation,
	}, nil
}

const maxConversationTitleRunes = 40

func (s *AIProcurementGrpcServer) RenameConversation(ctx context.Context, req *aipb.RenameConversationRequest) (*emptypb.Empty, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := parseInt64(req.GetConversationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid conversationId")
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" || len([]rune(title)) > maxConversationTitleRunes || strings.ContainsAny(title, "\n\r") {
		return nil, procurementStatus(codes.FailedPrecondition, "CONVERSATION_TITLE_INVALID", "標題需為 1-40 字且不含換行")
	}
	found, err := s.conversation.Rename(ctx, conversationID, buyerID, title, req.GetClientRequestId())
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	if !found {
		return nil, procurementStatus(codes.NotFound, "REQUEST_NOT_FOUND", "conversation not found")
	}
	return &emptypb.Empty{}, nil
}

func (s *AIProcurementGrpcServer) DeleteConversation(ctx context.Context, req *aipb.DeleteConversationRequest) (*emptypb.Empty, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := parseInt64(req.GetConversationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid conversationId")
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}
	found, err := s.conversation.Delete(ctx, conversationID, buyerID, req.GetClientRequestId())
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	if !found {
		return nil, procurementStatus(codes.NotFound, "REQUEST_NOT_FOUND", "conversation not found")
	}
	return &emptypb.Empty{}, nil
}

func (s *AIProcurementGrpcServer) CancelRun(ctx context.Context, req *aipb.CancelRunRequest) (*aipb.CancelRunResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	runID, err := parseInt64(req.GetRunId())
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, repository.ErrNotFound)
	}
	expectedVersion, err := parseInt64(req.GetExpectedRunVersion())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedRunVersion")
	}

	run, disposition, err := s.runStore.CancelRun(ctx, repository.CancelRunInput{
		RunID: runID, BuyerID: buyerID, ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, repository.ErrCancelVersionConflict) {
			return nil, procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
				"取消命令所依據的版本已過期，run 目前仍在執行中，請重新查詢最新狀態後再試")
		}
		if errors.Is(err, repository.ErrCancelInterruptPending) {
			return nil, procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalPending.Value,
				"這個需求有一個尚未回覆的中斷，請改用 resume 回覆")
		}
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.CancelRunResponse{
		RunId: itoa64(run.ID), RunStatus: run.Status,
		CancelDisposition: disposition, RunVersion: itoa64(run.Version),
	}, nil
}

func (s *AIProcurementGrpcServer) ResumeRun(ctx context.Context, req *aipb.ResumeRunRequest) (*aipb.ResumeRunResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetInterruptId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "interruptId is required")
	}
	if req.GetClientRequestId() == "" {
		return nil, procurementStatus(codes.InvalidArgument, "", "clientRequestId is required")
	}

	out, err := usecase.ClaimAndDispatchResume(ctx, usecase.ResumeInput{
		BuyerID: buyerID, InterruptID: req.GetInterruptId(),
		ClientRequestID: req.GetClientRequestId(), ResponsePayload: req.GetResponsePayload(),
	}, s.durableRun.RunStore, s.durableRun.Dispatcher, s.graphController)
	if err != nil {
		return nil, resumeUsecaseErrorStatus(ctx, err)
	}
	resp := &aipb.ResumeRunResponse{RunId: itoa64(out.RunID), RunStatus: out.RunStatus}
	if out.Result != nil {
		resp.Result = toTurnResultProto(*out.Result)
	}
	return resp, nil
}

func resumeUsecaseErrorStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, repository.ErrInterruptAlreadyResolved):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalAlreadyResolved.Value,
			"這個核准請求已被處理，請重新整理取得最新狀態")
	case errors.Is(err, repository.ErrRunNotResumable):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRunNotActive.Value,
			"run 目前不接受 resume，請重新查詢最新狀態")
	case errors.Is(err, repository.ErrResumeVersionConflict):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonApprovalStale.Value,
			"resume 所依據的版本已過期，請重新查詢最新狀態後再試")
	case errors.Is(err, usecase.ErrUnsupportedInterruptKind):
		return procurementStatus(codes.InvalidArgument, codemeta.ProcurementErrorReasonSchemaVersionUnsupported.Value,
			"不支援的 interrupt kind 或 response schema version")
	default:
		return answerUsecaseErrorStatus(ctx, err)
	}
}

func init() {
	const buyer = ActorBuyer
	registerMethodPolicy("/hawkr.ai.AIProcurementService/SubmitTurn", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/SubmitTurnStream", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetSnapshot", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/StartCandidateSearch", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetCandidateSearch", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ToggleSelection", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/SubmitAnswer", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/Rewind", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/PreviewRfqPrice", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ConfirmRfqSnapshot", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/PreviewRfqPublication", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ExecuteRfqPublication", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ListConversations", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetConversationMessages", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetRuntimeSnapshot", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/RenameConversation", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/DeleteConversation", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/CancelRun", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ResumeRun", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetRfqPublicationInviteSellerView", MethodPolicy{Actor: ActorNone})
}

func optionalPriceAmount(v *float64) string {
	if v == nil {
		return ""
	}
	return formatPriceAmount(*v)
}
