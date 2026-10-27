package usecase

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

type cropSelectionResolver interface {
	ResolveSelection(context.Context, string, *domain.CropSelection, []domain.CropDisambiguationOption, string, string) (cropidentity.DisambiguationResult, error)
}

func validateCropResumeAnswer(in AnswerInput) error {
	if in.PendingCrop == nil {
		return ErrInvalidOptionID
	}
	selection := in.PendingCrop.Selection
	if in.OptionID == domain.CropOtherOptionID {
		_, err := buildCropDisambiguationPatch(in)
		return err
	}
	if in.SelfEnteredPriceAmount != "" {
		return ErrInvalidOptionID
	}
	if in.OptionID == "" {
		text := strings.TrimSpace(in.FreeText)
		if selection == nil || (selection.Stage != domain.CropSelectionGroup && selection.Stage != domain.CropSelectionItem) || text == "" || utf8.RuneCountInString(text) > 200 {
			return ErrInvalidOptionID
		}
		return nil
	}
	if in.FreeText != "" {
		return ErrInvalidOptionID
	}
	chosen, ok := findCropDisambiguationOption(in.PendingCrop.Candidates, in.OptionID)
	if !ok {
		return ErrInvalidOptionID
	}
	if chosen.CategoryID != "" {
		if selection == nil || (selection.Stage != domain.CropSelectionGroup && selection.Stage != domain.CropSelectionItem) {
			return ErrInvalidOptionID
		}
		return nil
	}
	_, err := buildCropDisambiguationPatch(in)
	return err
}

func resolveResumedCropGate(ctx context.Context, resolver cropidentity.Resolver, current domain.Slots, in AnswerInput) (CropGateOutput, error) {
	if err := validateCropResumeAnswer(in); err != nil {
		return CropGateOutput{}, err
	}
	if in.PendingCrop.Selection == nil || in.OptionID == domain.CropOtherOptionID {
		patch, err := buildCropDisambiguationPatch(in)
		if err != nil {
			return CropGateOutput{}, err
		}
		if in.OptionID == domain.CropOtherOptionID {

			sourceItem := ""
			if current.Item != nil {
				sourceItem = cropidentity.NormalizeCategoryText(*current.Item)
			}
			if sourceItem == "" || sourceItem != cropidentity.NormalizeCategoryText(*patch.Item) {
				patch.ItemResolution.CategoryID = ""
				patch.ItemResolution.Label = ""
			}
		}
		return buildResumedCropGateOutput(current, patch), nil
	}
	selector, ok := resolver.(cropSelectionResolver)
	if !ok {
		return CropGateOutput{}, ErrInvalidOptionID
	}
	itemText := ""
	if current.Item != nil {
		itemText = *current.Item
	}
	result, err := selector.ResolveSelection(ctx, itemText, in.PendingCrop.Selection, in.PendingCrop.Candidates, in.OptionID, strings.TrimSpace(in.FreeText))
	if err != nil {
		return CropGateOutput{}, err
	}
	out := CropGateOutput{Item: current.Item, Selection: result.Selection, FactsAsOf: time.Now().UTC()}
	switch {
	case result.NeedsDisambiguation:
		out.Pending = true
		out.PendingReason = domain.CropPendingReasonAmbiguous
		out.Candidates = result.Candidates
	case result.Unavailable:
		out.Pending = true
		out.PendingReason = domain.CropPendingReasonUnavailable
		out.Candidates = []domain.CropDisambiguationOption{}
	default:
		out.Resolution = domain.NewSystemItemResolution(result.CropCode, result.GroupCode, result.CommodityCode, result.FallbackGroupCode, result.Label, domain.CategoryFromSelection(result.Selection))
		if out.Resolution == nil {
			return CropGateOutput{}, ErrInvalidOptionID
		}
		out.Resolution.Provenance = domain.ItemResolutionByBuyerChoice
		if result.Selection != nil {
			out.Resolution.CategoryID = result.Selection.CategoryID
		}
	}
	return out, nil
}
