package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

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

var ErrStaleRevision = errors.New("procurement: expectedRevisionId doesn't match current active revision")

var ErrInvalidOptionID = errors.New("procurement: answer does not conform to this slot's schema")

var ErrCannotSkipQuoteRequiredSlot = errors.New("procurement: this field is required for quote generation and cannot be skipped")

type AnswerStore interface {
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
	ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error)
	AppendRevision(ctx context.Context, in repository.AppendInput) (repository.Revision, error)
	FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error)
}

type AnswerInput struct {
	BuyerID                identity.BuyerID
	RequestID              int64
	SlotKey                domain.SlotKey
	OptionID               string
	FreeText               string
	UnitOptionID           string
	SelfEnteredPriceAmount string
	Skip                   bool
	ExpectedRevisionID     int64
	ClientRequestID        string
	PendingCrop            *CropDisambiguationInterruptPayload
}

type AnswerUseCase struct {
	store                 AnswerStore
	cropResolver          cropidentity.Resolver
	pricePreviewChecker   PricePreviewChecker
	rfqConfirmationReader RfqConfirmationReader
	narrator              StepNarrator
	realizer              SurfaceRealizer
	now                   func() time.Time
}

func NewAnswerUseCase(store AnswerStore, cropResolver cropidentity.Resolver, pricePreviewChecker PricePreviewChecker, rfqConfirmationReader RfqConfirmationReader, narrator StepNarrator, realizer SurfaceRealizer) *AnswerUseCase {
	return &AnswerUseCase{
		store: store, cropResolver: cropResolver,
		pricePreviewChecker: pricePreviewChecker, rfqConfirmationReader: rfqConfirmationReader,
		narrator: narrator, realizer: realizer,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (uc *AnswerUseCase) composeAnswerViaGraph(ctx context.Context, in AnswerInput) (TurnResult, error) {
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

	_, activeRev, err := uc.store.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return TurnResult{}, err
	}
	if activeRev.ID != in.ExpectedRevisionID {
		return TurnResult{}, ErrStaleRevision
	}
	var current domain.Slots
	if err := json.Unmarshal(activeRev.SlotsJSON, &current); err != nil {
		return TurnResult{}, err
	}

	if in.SlotKey == domain.SlotUnit || in.SlotKey == domain.SlotCropDisambiguation {
		return TurnResult{}, ErrInvalidOptionID
	}

	var patch domain.SlotPatch
	var skipKey *domain.SlotKey
	switch {
	case in.Skip:
		if domain.IsRequiredSlot(in.SlotKey) || domain.QuoteRequiredOptionalSlots[in.SlotKey] {
			return TurnResult{}, ErrCannotSkipQuoteRequiredSlot
		}
		skipKey = &in.SlotKey
	case in.SlotKey == domain.SlotQuantity:
		patch, err = buildQuantityPatch(current, in)
		if err != nil {
			return TurnResult{}, err
		}
	default:
		value, err := resolveAnswerValue(in.SlotKey, in.OptionID, in.FreeText)
		if err != nil {
			return TurnResult{}, err
		}
		patch, err = buildSinglePatch(in.SlotKey, value)
		if err != nil {
			return TurnResult{}, err
		}
	}

	next, rejected := current.Apply(patch, uc.now())
	if rejected == nil {
		rejected = []domain.RejectedField{}
	}

	cropGate := cropidentity.ComputeGate(ctx, uc.cropResolver, current, next)

	missing := next.MissingRequired()
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

	history, err := uc.store.ListBranchRevisions(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return TurnResult{}, err
	}
	skipped := deriveSkippedSlots(history)
	if skipKey != nil {
		skipped[*skipKey] = true
	}
	prevStreak := previousOptionalStreak(history)
	question, capExhausted := domain.PickNextQuestion(next, missing, skipped, prevStreak)
	if cropGate.Pending {
		question = domain.BuildCropDisambiguationQuestion(cropGate.Candidates, cropGate.Selection)
		capExhausted = false
	}
	optionalStreak := computeOptionalStreak(status, question, capExhausted, prevStreak)

	summary := answerMessageSummary(in.SlotKey, in.Skip, in.FreeText, in.OptionID, in.UnitOptionID)
	rfqState := domain.RfqNone
	if uc.rfqConfirmationReader != nil {
		snap, err := uc.rfqConfirmationReader.FindLatestConfirmedRfqSnapshotByRequestID(ctx, in.RequestID, in.BuyerID)
		switch {
		case err == nil && snap.RequestRevisionID == activeRev.ID:
			rfqState = domain.RfqConfirmed
		case err == nil:
			rfqState = domain.RfqStale
		}
	}
	recorded := domain.RecordedThisTurn(patch, current, next)
	hasPreview := false
	if uc.pricePreviewChecker != nil {
		if hp, err := uc.pricePreviewChecker.HasPricePreview(ctx, in.RequestID); err == nil {
			hasPreview = hp
		}
	}
	plan := domain.PlanDialogue(domain.DialoguePlanInput{
		Current: current, Next: next, Missing: missing, Ready: ready,
		Rejected: rejected, HasRejected: len(rejected) > 0, Degraded: false,
		Recorded: recorded, Question: question,
		CropDisambiguationPending:    cropGate.Pending,
		CropDisambiguationCandidates: cropGate.Candidates,
		PricePreviewAvailable:        hasPreview,
		RfqConfirmationState:         rfqState,
	})
	plan.StateVersion = strconv.FormatInt(in.RequestID, 10) + "-rev" + strconv.FormatInt(activeRev.ID, 10)
	plan.PlanHash = hashDialoguePlan(plan)

	renderArgs := domain.SafeRenderArgs{
		MissingRequiredLabels: missingRequiredLabels(missing),
		SalientFactLabels:     salientFactLabels(plan, recorded),
		NextQuestionPrompt:    questionPrompt(question),
		RejectedLabels:        rejectedLabels(rejected),
	}
	blocks, renderMode := domain.RenderPlan(plan, renderArgs), "TEMPLATE"

	g := syncutil.NewGroup(ctx)
	g.Go(func() error {
		blocks, renderMode = realizeOrFallback(ctx, uc.realizer, plan, renderArgs, next, buildRecentNarratives(history))
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
			{domain.StepValidateSlots, summarizeValidateSlots(next, rejected, recorded)},
			{finalCode, summarizeNextQuestion(question, missing, ready)},
		}
		if cropGate.Evaluated() {
			jobs = append(jobs, struct {
				code    domain.StepCode
				summary string
			}{domain.StepCropGate, summarizeCropGate(cropGate)})
		}
		if len(rejected) > 0 {
			jobs = append(jobs, struct {
				code    domain.StepCode
				summary string
			}{domain.StepSlotsRejected, summarizeSlotsRejected(rejected)})
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
		pkglogger.Error(ctx, "procurement: answer tail composition panicked", zap.Error(err))
	}
	for i, j := range jobs {
		if texts[i] != "" {
			narration[j.code] = texts[i]
		}
	}
	reply := domain.TextFallback(blocks)
	steps := domain.ComputeAnswerSteps(0, 0, next, len(rejected) > 0, ready, cropGate.Evaluated(), narration)

	observability.RecordProcurementDialoguePlan(ctx, string(plan.ActCode))
	payload, err := json.Marshal(turnPayload{
		Message: summary, Patch: patch, Rejected: rejected, Degraded: false,
		Reply: reply, Blocks: blocks, RenderMode: renderMode, Steps: steps,
		RecordedThisTurn: recorded, Question: question, Skip: skipKey,
		OptionalStreak: optionalStreak,
		DialoguePlan:   &plan,
	})
	if err != nil {
		return TurnResult{}, err
	}

	persistCtx := context.WithoutCancel(ctx)
	rev, err := uc.store.AppendRevision(persistCtx, repository.AppendInput{
		RequestID: in.RequestID, BuyerID: in.BuyerID, Slots: next,
		Source: "ANSWER", ClientRequestID: in.ClientRequestID, PayloadJSON: payload,
		RequestStatus: status,
	})
	if err != nil {
		return TurnResult{}, err
	}

	return TurnResult{
		RequestID: in.RequestID, ConversationID: rev.ConversationID, RevisionID: rev.ID, Slots: next,
		Rejected: rejected, MissingRequired: missing, RecordedThisTurn: recorded,
		Degraded: false, Reply: reply, Blocks: blocks, RenderMode: renderMode, Steps: steps, Question: question,
		NextAction: nextAction, DialoguePlan: plan,
	}, nil
}

func resolveAnswerValue(slotKey domain.SlotKey, optionID, freeText string) (string, error) {
	if optionID != "" {
		for _, opt := range domain.SlotOptions[slotKey] {
			if opt.OptionID == optionID {
				return opt.Value, nil
			}
		}
		return "", ErrInvalidOptionID
	}
	return freeText, nil
}

func buildQuantityPatch(current domain.Slots, in AnswerInput) (domain.SlotPatch, error) {
	patch := domain.SlotPatch{}

	if in.FreeText != "" {
		n, err := strconv.Atoi(in.FreeText)
		if err != nil {
			return domain.SlotPatch{}, ErrInvalidOptionID
		}
		patch.Quantity = &n
	} else if current.Quantity == nil {
		return domain.SlotPatch{}, ErrInvalidOptionID
	}

	if in.UnitOptionID != "" {
		value, err := resolveAnswerValue(domain.SlotUnit, in.UnitOptionID, "")
		if err != nil {
			return domain.SlotPatch{}, err
		}
		patch.Unit = &value
	} else if current.Unit == nil {
		return domain.SlotPatch{}, ErrInvalidOptionID
	}

	return patch, nil
}

func buildSinglePatch(key domain.SlotKey, value string) (domain.SlotPatch, error) {
	switch key {
	case domain.SlotItem:
		return domain.SlotPatch{Item: &value}, nil
	case domain.SlotPackaging:
		return domain.SlotPatch{Packaging: &value}, nil
	case domain.SlotDeliveryCity:
		return domain.SlotPatch{DeliveryCity: &value}, nil
	case domain.SlotDeliveryDate:
		return domain.SlotPatch{DeliveryDate: &value}, nil
	case domain.SlotQuantity:
		n, err := strconv.Atoi(value)
		if err != nil {
			return domain.SlotPatch{}, ErrInvalidOptionID
		}
		return domain.SlotPatch{Quantity: &n}, nil
	case domain.SlotBudgetPerUnit:
		n, err := strconv.Atoi(value)
		if err != nil {
			return domain.SlotPatch{}, ErrInvalidOptionID
		}
		return domain.SlotPatch{BudgetPerUnit: &n}, nil
	default:
		return domain.SlotPatch{}, ErrInvalidOptionID
	}
}

func answerMessageSummary(slotKey domain.SlotKey, skip bool, freeText, optionID, unitOptionID string) string {
	label := domain.SlotLabels[slotKey]
	if skip {
		return "略過：" + label
	}
	if slotKey == domain.SlotQuantity {
		return quantityAnswerSummary(freeText, unitOptionID)
	}
	if optionID != "" {
		return label + "：" + optionLabel(slotKey, optionID)
	}
	return label + "：" + freeText
}

func quantityAnswerSummary(freeText, unitOptionID string) string {
	unitLabel := ""
	if unitOptionID != "" {
		unitLabel = optionLabel(domain.SlotUnit, unitOptionID)
	}
	if freeText == "" {
		return domain.SlotLabels[domain.SlotUnit] + "：" + unitLabel
	}
	if unitLabel == "" {
		return domain.SlotLabels[domain.SlotQuantity] + "：" + freeText
	}
	return domain.SlotLabels[domain.SlotQuantity] + "：" + freeText + " " + unitLabel
}

func optionLabel(slotKey domain.SlotKey, optionID string) string {
	for _, opt := range domain.SlotOptions[slotKey] {
		if opt.OptionID == optionID {
			return opt.Label
		}
	}
	return optionID
}

func buildCropDisambiguationPatch(in AnswerInput) (patch domain.SlotPatch, err error) {
	if in.PendingCrop == nil {
		return domain.SlotPatch{}, ErrInvalidOptionID
	}

	if in.OptionID == domain.CropOtherOptionID {
		description := strings.TrimSpace(in.FreeText)
		price := strings.TrimSpace(in.SelfEnteredPriceAmount)
		if description == "" {
			return domain.SlotPatch{}, ErrInvalidOptionID
		}
		amount, atoiErr := strconv.Atoi(price)
		if atoiErr != nil || amount <= 0 || amount > domain.MaxPricePerKg {
			return domain.SlotPatch{}, ErrInvalidOptionID
		}
		_, selLabel := in.PendingCrop.Selection.ConfirmedCategory()
		resolution := domain.NewSelfPricedItemResolution(amount, selLabel,
			domain.CategoryFromSelection(in.PendingCrop.Selection))
		return domain.SlotPatch{Item: &description, ItemResolution: resolution}, nil
	}

	return domain.SlotPatch{}, ErrInvalidOptionID
}

func findCropDisambiguationOption(candidates []domain.CropDisambiguationOption, optionID string) (domain.CropDisambiguationOption, bool) {
	for _, c := range candidates {
		if domain.CropDisambiguationOptionID(c) == optionID {
			return c, true
		}
	}
	return domain.CropDisambiguationOption{}, false
}

var errAnswerPipelinePanicked = errors.New("procurement: answer pipeline panicked")

func canonicalAnswerHash(in AnswerInput) string {
	return canonicalHash(struct {
		BuyerID                identity.BuyerID
		RequestID              int64
		SlotKey                domain.SlotKey
		OptionID               string
		FreeText               string
		UnitOptionID           string
		SelfEnteredPriceAmount string
		Skip                   bool
		ExpectedRevisionID     int64
	}{
		in.BuyerID, in.RequestID, in.SlotKey, in.OptionID, in.FreeText,
		in.UnitOptionID, in.SelfEnteredPriceAmount, in.Skip, in.ExpectedRevisionID,
	})
}

type answerDispatchResult struct {
	outcome turnPipelineOutcome
	err     error
}

func (uc *AnswerUseCase) ClaimAndDispatchAnswer(ctx context.Context, in AnswerInput, runStore RunClaimStore, dispatcher *RunDispatcher) (SubmitTurnResult, error) {
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
		CommandKind:     "SUBMIT_ANSWER",
		PayloadHash:     canonicalAnswerHash(in),
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

	done := make(chan answerDispatchResult, 1)
	dispatcher.Dispatch(ctx, func(bgCtx context.Context) {
		out := answerDispatchResult{err: errAnswerPipelinePanicked}
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
		outcome, execErr := uc.executeAnswerPipeline(bgCtx, run, in, runStore)
		out = answerDispatchResult{outcome: outcome, err: execErr}
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

func (uc *AnswerUseCase) executeAnswerPipeline(ctx context.Context, run repository.Run, in AnswerInput, runStore RunClaimStore) (turnPipelineOutcome, error) {
	result, err := uc.composeAnswerViaGraph(ctx, in)
	var createdInterrupt repository.Interrupt

	for _, step := range result.Steps {
		if _, appendErr := runStore.AppendRunEvent(ctx, repository.AppendRunEventInput{
			RunID: run.ID, BuyerID: in.BuyerID, EventType: ledgerStepEventType(step),
			SchemaVersion: "v1", PublicPayloadJSON: mustMarshalStep(step),
		}); appendErr != nil {
			pkglogger.Error(ctx, "procurement: failed to append run event for answer step",
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
		pkglogger.Error(ctx, "procurement: answer pipeline failed",
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
		pkglogger.Error(ctx, "procurement: failed to terminalize answer run", zap.Error(termErr), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize answer run CAS lost, this answer's terminal state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version),
			zap.String("attempted_status", terminalStatus), zap.String("attempted_event_type", terminalEventType))
	}

	return turnPipelineOutcome{
		result: result, interruptID: createdInterrupt.ID, interruptKind: createdInterrupt.InterruptKind,
	}, err
}

func (uc *AnswerUseCase) terminalizeFailedRun(ctx context.Context, run repository.Run, buyerID identity.BuyerID, runStore RunClaimStore) {
	wrote, err := runStore.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusFailed(),
		FailureReason:     codemeta.ProcurementRunFailureReasonTurnPipelineError.Value,
		TerminalEventType: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		PublicPayloadJSON: domain.RunFailedPayload(codemeta.ProcurementRunFailureReasonTurnPipelineError.Value),
	})
	switch {
	case err != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize panicked answer run",
			zap.Error(err), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize panicked answer run CAS lost, FAILED state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version))
	}
}

func (uc *AnswerUseCase) replayExistingRun(ctx context.Context, run repository.Run, in AnswerInput) (TurnResult, error) {
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

func ledgerStepEventType(step domain.Step) string {
	if step.Status == codemeta.ProcurementStepStatusSucceeded.Value {
		return codemeta.ProcurementLedgerEventTypeStepFinished.Value
	}
	return codemeta.ProcurementLedgerEventTypeStepStarted.Value
}

func mustMarshalStep(step domain.Step) []byte {
	b, err := json.Marshal(step)
	if err != nil {
		return []byte(`{"error":"step marshal failed"}`)
	}
	return b
}
