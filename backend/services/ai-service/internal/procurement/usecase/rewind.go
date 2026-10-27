package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"
)

var ErrSlotNeverWritten = errors.New("procurement: slotKey has never been written, nothing to rewind")

type RewindStore interface {
	ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error)
	RewindTo(ctx context.Context, in repository.RewindInput) (repository.Revision, error)
	FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error)
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
}

type RewindInput struct {
	BuyerID         identity.BuyerID
	RequestID       int64
	SlotKey         domain.SlotKey
	ClientRequestID string
}

type RewindUseCase struct {
	store                 RewindStore
	cropResolver          cropidentity.Resolver
	pricePreviewChecker   PricePreviewChecker
	rfqConfirmationReader RfqConfirmationReader
	narrator              StepNarrator
	realizer              SurfaceRealizer
}

func NewRewindUseCase(store RewindStore, cropResolver cropidentity.Resolver, pricePreviewChecker PricePreviewChecker, rfqConfirmationReader RfqConfirmationReader, narrator StepNarrator, realizer SurfaceRealizer) *RewindUseCase {
	return &RewindUseCase{
		store: store, cropResolver: cropResolver,
		pricePreviewChecker: pricePreviewChecker, rfqConfirmationReader: rfqConfirmationReader,
		narrator: narrator, realizer: realizer,
	}
}

func (uc *RewindUseCase) composeRewindViaGraph(ctx context.Context, in RewindInput) (TurnResult, error) {
	prev, err := uc.store.FindByIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID)
	switch {
	case err == nil:
		if in.RequestID != 0 && in.RequestID != prev.RequestID {
			return TurnResult{}, domain.ErrIdempotencyKeyConflict
		}
		return replayResult(prev)
	case errors.Is(err, repository.ErrNotFound):
	default:
		return TurnResult{}, err
	}

	history, err := uc.store.ListBranchRevisions(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return TurnResult{}, err
	}

	targetID, targetSlots, err := resolveRewindTarget(history, in.SlotKey)
	if err != nil {
		return TurnResult{}, err
	}

	var currentSlots domain.Slots
	if err := json.Unmarshal(history[len(history)-1].SlotsJSON, &currentSlots); err != nil {
		return TurnResult{}, err
	}

	cropGate := cropidentity.ComputeGate(ctx, uc.cropResolver, currentSlots, targetSlots)
	rejected := []domain.RejectedField{}

	missing := targetSlots.MissingRequired()
	status := "CLARIFYING"
	if len(missing) == 0 {
		status = "REQUIREMENT_READY"
	}
	ready := status == "REQUIREMENT_READY"

	nextAction := ""
	if ready && uc.pricePreviewChecker != nil {
		hasPreview, err := uc.pricePreviewChecker.HasPricePreview(ctx, in.RequestID)
		if err == nil && !hasPreview {
			nextAction = "REVIEW_PRICE_PREVIEW"
		}
	}

	questionKey := in.SlotKey
	if questionKey == domain.SlotUnit {
		questionKey = domain.SlotQuantity
	}
	question := domain.BuildQuestionFor(questionKey, targetSlots)
	if cropGate.Pending {
		question = domain.BuildCropDisambiguationQuestion(cropGate.Candidates, cropGate.Selection)
	}

	summary := "重新回答：" + domain.SlotLabels[in.SlotKey]

	rfqState := domain.RfqNone
	if uc.rfqConfirmationReader != nil {
		snap, err := uc.rfqConfirmationReader.FindLatestConfirmedRfqSnapshotByRequestID(ctx, in.RequestID, in.BuyerID)
		switch {
		case err == nil && snap.RequestRevisionID == targetID:
			rfqState = domain.RfqConfirmed
		case err == nil:
			rfqState = domain.RfqStale
		}
	}
	hasPreview := false
	if uc.pricePreviewChecker != nil {
		if hp, err := uc.pricePreviewChecker.HasPricePreview(ctx, in.RequestID); err == nil {
			hasPreview = hp
		}
	}
	plan := domain.PlanDialogue(domain.DialoguePlanInput{
		Current: targetSlots, Next: targetSlots, Missing: missing, Ready: ready,
		Rejected: rejected, HasRejected: len(rejected) > 0, Degraded: false,
		Recorded: []domain.RecordedField{}, Question: question,
		CropDisambiguationPending:    cropGate.Pending,
		CropDisambiguationCandidates: cropGate.Candidates,
		PricePreviewAvailable:        hasPreview,
		RfqConfirmationState:         rfqState,
	})
	plan.StateVersion = strconv.FormatInt(in.RequestID, 10) + "-rev" + strconv.FormatInt(targetID, 10)
	plan.PlanHash = hashDialoguePlan(plan)

	renderArgs := domain.SafeRenderArgs{
		MissingRequiredLabels: missingRequiredLabels(missing),
		SalientFactLabels:     nil,
		NextQuestionPrompt:    questionPrompt(question),
		RejectedLabels:        rejectedLabels(rejected),
	}
	blocks, renderMode := domain.RenderPlan(plan, renderArgs), "TEMPLATE"

	g := syncutil.NewGroup(ctx)
	g.Go(func() error {
		blocks, renderMode = realizeOrFallback(ctx, uc.realizer, plan, renderArgs, targetSlots, buildRecentNarratives(history))
		return nil
	})

	narration := domain.StepNarration{}
	var jobs []struct {
		code    domain.StepCode
		summary string
	}
	var texts []string
	if uc.narrator != nil {
		finalCode := domain.StepPickNextQuestion
		if ready {
			finalCode = domain.StepRequirementReady
		}
		jobs = []struct {
			code    domain.StepCode
			summary string
		}{
			{finalCode, summarizeNextQuestion(question, missing, ready)},
		}
		if cropGate.Evaluated() {
			jobs = append(jobs, struct {
				code    domain.StepCode
				summary string
			}{domain.StepCropGate, summarizeCropGate(cropGate)})
		}

		texts = make([]string, len(jobs))
		for i, j := range jobs {
			g.Go(func() error {
				texts[i] = narrateStepOutcome(ctx, uc.narrator, j.code, j.summary,
					domain.StepLabels[j.code])
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		pkglogger.Error(ctx, "procurement: rewind tail composition panicked", zap.Error(err))
	}
	for i, j := range jobs {
		narration[j.code] = texts[i]
	}
	reply := domain.TextFallback(blocks)
	steps := domain.ComputeRewindSteps(0, targetSlots, ready, cropGate.Evaluated(), narration)

	observability.RecordProcurementDialoguePlan(ctx, string(plan.ActCode))
	payload, err := json.Marshal(turnPayload{
		Message: summary, Rejected: rejected,
		Reply: reply, Blocks: blocks, RenderMode: renderMode, Steps: steps,
		RecordedThisTurn: []domain.RecordedField{}, Question: question,
		OptionalStreak: 0,
		DialoguePlan:   &plan,
	})
	if err != nil {
		return TurnResult{}, err
	}

	persistCtx := context.WithoutCancel(ctx)
	rev, err := uc.store.RewindTo(persistCtx, repository.RewindInput{
		RequestID: in.RequestID, BuyerID: in.BuyerID, TargetRevisionID: targetID,
		ClientRequestID: in.ClientRequestID, PayloadJSON: payload,
	})
	if err != nil {
		return TurnResult{}, err
	}

	return TurnResult{
		RequestID: in.RequestID, ConversationID: rev.ConversationID, RevisionID: rev.ID, Slots: targetSlots,
		Rejected: rejected, MissingRequired: missing, RecordedThisTurn: []domain.RecordedField{},
		Degraded: false, Reply: reply, Blocks: blocks, RenderMode: renderMode, Steps: steps, Question: question,
		NextAction: nextAction, DialoguePlan: plan,
	}, nil
}

func resolveRewindTarget(chain []repository.RevisionWithPatch, slotKey domain.SlotKey) (targetRevisionID int64, targetSlots domain.Slots, err error) {
	if len(chain) == 0 {
		return 0, domain.Slots{}, ErrSlotNeverWritten
	}
	var prevSlots domain.Slots
	if err := json.Unmarshal(chain[0].SlotsJSON, &prevSlots); err != nil {
		return 0, domain.Slots{}, err
	}
	found := false
	for i := 1; i < len(chain); i++ {
		var cur domain.Slots
		if err := json.Unmarshal(chain[i].SlotsJSON, &cur); err != nil {
			return 0, domain.Slots{}, err
		}
		if slotValueChanged(prevSlots, cur, slotKey) {
			targetRevisionID = chain[i-1].ID
			targetSlots = prevSlots
			found = true
		}
		prevSlots = cur
	}
	if !found {
		return 0, domain.Slots{}, ErrSlotNeverWritten
	}
	return targetRevisionID, targetSlots, nil
}

func slotValueChanged(prev, cur domain.Slots, key domain.SlotKey) bool {
	return slotValueString(prev, key) != slotValueString(cur, key)
}

func slotValueString(s domain.Slots, key domain.SlotKey) string {
	switch key {
	case domain.SlotItem:
		return derefStr(s.Item)
	case domain.SlotUnit:
		return derefStr(s.Unit)
	case domain.SlotPackaging:
		return derefStr(s.Packaging)
	case domain.SlotDeliveryCity:
		return derefStr(s.DeliveryCity)
	case domain.SlotDeliveryDate:
		return derefStr(s.DeliveryDate)
	case domain.SlotQuantity:
		if s.Quantity == nil {
			return ""
		}
		return strconv.Itoa(*s.Quantity)
	case domain.SlotBudgetPerUnit:
		if s.BudgetPerUnit == nil {
			return ""
		}
		return strconv.Itoa(*s.BudgetPerUnit)
	case domain.SlotExactMatchRequired:
		if s.ExactMatchRequired == nil {
			return ""
		}
		return strconv.FormatBool(*s.ExactMatchRequired)
	default:
		return ""
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

var errRewindPipelinePanicked = errors.New("procurement: rewind pipeline panicked")

func canonicalRewindHash(in RewindInput) string {
	return canonicalHash(struct {
		BuyerID   identity.BuyerID
		RequestID int64
		SlotKey   domain.SlotKey
	}{in.BuyerID, in.RequestID, in.SlotKey})
}

type rewindDispatchResult struct {
	outcome turnPipelineOutcome
	err     error
}

func (uc *RewindUseCase) ClaimAndDispatchRewind(ctx context.Context, in RewindInput, runStore RunClaimStore, dispatcher *RunDispatcher) (SubmitTurnResult, error) {
	isRetry, err := runStore.HasIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID)
	if err != nil {
		return SubmitTurnResult{}, err
	}
	if !isRetry {
		if err := GuardNoOpenInterrupt(ctx, runStore, in.RequestID, in.BuyerID); err != nil {
			return SubmitTurnResult{}, err
		}
	}

	run, isNew, err := runStore.ClaimRun(ctx, repository.ClaimRunInput{
		BuyerID:         in.BuyerID,
		ClientRequestID: in.ClientRequestID,
		CommandKind:     "REWIND",
		PayloadHash:     canonicalRewindHash(in),
		RunType:         domain.RunTypeTurn,
		ControllerKind:  domain.ControllerKindGraph,
		RequestID:       in.RequestID,
	})
	if err != nil {
		return SubmitTurnResult{}, err
	}
	if !isNew {
		result, err := uc.replayExistingRun(ctx, run, in)
		if err != nil {
			return SubmitTurnResult{}, err
		}
		return SubmitTurnResult{RunID: run.ID, Result: result}, nil
	}

	done := make(chan rewindDispatchResult, 1)
	dispatcher.Dispatch(ctx, func(bgCtx context.Context) {
		out := rewindDispatchResult{err: errRewindPipelinePanicked}
		defer func() {
			r := recover()
			if r != nil {
				uc.terminalizeFailedRun(bgCtx, run, in.BuyerID, runStore)
			}
			done <- out
			if r != nil {
				panic(r)
			}
		}()
		outcome, execErr := uc.executeRewindPipeline(bgCtx, run, in, runStore)
		out = rewindDispatchResult{outcome: outcome, err: execErr}
	})

	dispatched := <-done
	if dispatched.err != nil {
		return SubmitTurnResult{RunID: run.ID}, dispatched.err
	}
	return SubmitTurnResult{
		RunID:         run.ID,
		Result:        dispatched.outcome.result,
		InterruptID:   dispatched.outcome.interruptID,
		InterruptKind: dispatched.outcome.interruptKind,
	}, nil
}

func (uc *RewindUseCase) executeRewindPipeline(ctx context.Context, run repository.Run, in RewindInput, runStore RunClaimStore) (turnPipelineOutcome, error) {
	result, err := uc.composeRewindViaGraph(ctx, in)
	var createdInterrupt repository.Interrupt

	for _, step := range result.Steps {
		if _, appendErr := runStore.AppendRunEvent(ctx, repository.AppendRunEventInput{
			RunID: run.ID, BuyerID: in.BuyerID, EventType: ledgerStepEventType(step),
			SchemaVersion: "v1", PublicPayloadJSON: mustMarshalStep(step),
		}); appendErr != nil {
			pkglogger.Error(ctx, "procurement: failed to append run event for rewind step",
				zap.Error(appendErr), zap.Int64("run_id", run.ID), zap.String("step_code", string(step.Code)))
		}
	}

	if run.ConversationID == 0 && result.ConversationID != 0 {
		if err := runStore.SetRunConversationID(ctx, run.ID, result.ConversationID); err != nil {
			pkglogger.Error(ctx, "procurement: failed to backfill run conversation_id",
				zap.Error(err), zap.Int64("run_id", run.ID), zap.Int64("conversation_id", result.ConversationID))
		}
	}

	terminalStatus := domain.RunStatusSucceeded()
	terminalEventType := codemeta.ProcurementLedgerEventTypeRunFinished.Value
	failureReason := ""
	terminalPayload := []byte(`{}`)
	interruptKind, needsInterrupt := interruptOutcomeFor(result)

	if needsInterrupt && err == nil {
		interrupt, ierr := runStore.CreateInterrupt(ctx, repository.CreateInterruptInput{
			RunID: run.ID, InterruptKind: interruptKind, ResponseSchemaVersion: "v1",
		})
		if ierr != nil {
			pkglogger.Error(ctx, "procurement: failed to create interrupt for crop disambiguation, falling back to normal completion",
				zap.Error(ierr), zap.Int64("run_id", run.ID))
		} else {
			createdInterrupt = interrupt
			terminalStatus = domain.RunStatusWaiting()
			terminalEventType = codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value
			terminalPayload, _ = json.Marshal(map[string]string{
				"interruptId": interrupt.ID, "interruptKind": interrupt.InterruptKind,
			})
		}
	}
	if err != nil {
		terminalStatus = domain.RunStatusFailed()
		terminalEventType = codemeta.ProcurementLedgerEventTypeRunFailed.Value
		failureReason = codemeta.ProcurementRunFailureReasonTurnPipelineError.Value
		terminalPayload = []byte(`{}`)
		pkglogger.Error(ctx, "procurement: rewind pipeline failed",
			zap.Error(err), zap.Int64("run_id", run.ID), zap.Int64("buyer_id", int64(in.BuyerID)))
	}

	wrote, termErr := runStore.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: run.ID, BuyerID: in.BuyerID, Version: run.Version, Status: terminalStatus,
		FailureReason: failureReason, TerminalEventType: terminalEventType,
		ResultRevisionID:  result.RevisionID,
		PublicPayloadJSON: terminalPayload,
	})
	switch {
	case termErr != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize rewind run", zap.Error(termErr), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize rewind run CAS lost, this rewind's terminal state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version),
			zap.String("attempted_status", terminalStatus), zap.String("attempted_event_type", terminalEventType))
	}

	return turnPipelineOutcome{
		result: result, interruptID: createdInterrupt.ID, interruptKind: createdInterrupt.InterruptKind,
	}, err
}

func (uc *RewindUseCase) terminalizeFailedRun(ctx context.Context, run repository.Run, buyerID identity.BuyerID, runStore RunClaimStore) {
	wrote, err := runStore.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusFailed(),
		FailureReason:     codemeta.ProcurementRunFailureReasonTurnPipelineError.Value,
		TerminalEventType: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		PublicPayloadJSON: domain.RunFailedPayload(codemeta.ProcurementRunFailureReasonTurnPipelineError.Value),
	})
	switch {
	case err != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize panicked rewind run",
			zap.Error(err), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize panicked rewind run CAS lost, FAILED state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version))
	}
}

func (uc *RewindUseCase) replayExistingRun(ctx context.Context, run repository.Run, in RewindInput) (TurnResult, error) {
	if run.ResultRevisionID == 0 {
		return TurnResult{}, ErrRunResultUnavailable
	}
	prev, err := uc.store.FindByIdempotencyKey(ctx, run.BuyerID, in.ClientRequestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return TurnResult{}, ErrRunResultUnavailable
		}
		return TurnResult{}, err
	}
	if in.RequestID != 0 && in.RequestID != prev.RequestID {
		return TurnResult{}, domain.ErrIdempotencyKeyConflict
	}
	return replayResult(prev)
}
