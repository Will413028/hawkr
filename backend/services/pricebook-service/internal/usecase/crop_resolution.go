// services/pricebook-service/internal/usecase/crop_resolution.go
package usecase

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// ErrMarketCodeNotAllowedForGroup: cropName resolved to a crop_group but
// the caller supplied a market_code. GroupForecast aggregates across all
// markets by design (see cmd/pricebook-forecast's existing --group/--market
// mutual-exclusion rule) — accepting a market filter here would produce a
// result that looks scoped to one market but isn't.
var ErrMarketCodeNotAllowedForGroup = errors.New("usecase: market_code is not allowed when crop_name resolves to a crop group")

// ErrMarketCodeRequiredForCrop: cropName resolved to a specific crop
// (sub-variant) but the caller omitted market_code — SeriesForecast needs
// an exact (market, crop) pair, there's no cross-market default for a
// single series.
var ErrMarketCodeRequiredForCrop = errors.New("usecase: market_code is required when crop_name resolves to a specific crop")

// ErrConflictingResolutionInputs: more than one of crop_name/crop_code/
// group_code was supplied — the caller must send exactly one. The three are
// alternative ways of naming the same thing, so honouring a precedence order
// instead would silently ignore part of the request (and, worse, could
// answer about a different crop than the one whose name the caller sent).
var ErrConflictingResolutionInputs = errors.New("usecase: exactly one of crop_name, crop_code, or group_code must be set")

// ErrNoResolutionInput: none of crop_name/crop_code/group_code was supplied.
// Distinct from ErrConflictingResolutionInputs so the gRPC layer can return
// a message that actually tells the caller what to fix, even though both map
// to codes.InvalidArgument.
var ErrNoResolutionInput = errors.New("usecase: one of crop_name, crop_code, or group_code is required")

// validateResolutionInputs enforces the "exactly one of crop_name /
// crop_code / group_code" caller contract shared by resolveCropTarget and
// ObservedPrice.Get (which can't use resolveCropTarget itself — its
// downstream shape is raw observation rows, not a forecast target — but must
// accept exactly the same request contract).
func validateResolutionInputs(cropName, cropCode, groupCode string) error {
	setCount := 0
	for _, v := range []string{cropName, cropCode, groupCode} {
		if v != "" {
			setCount++
		}
	}
	if setCount > 1 {
		return ErrConflictingResolutionInputs
	}
	if setCount == 0 {
		return ErrNoResolutionInput
	}
	return nil
}

// resolvedTarget is what resolveCropTarget determines. Exactly one of
// three shapes:
//   - Unavailable=true: a business "can't produce a result" outcome
//     (unresolved name, zero-match group, model doesn't cover the resolved
//     crop/group). ResolvedCropCode/ResolvedCropGroupCode/MarketCode may
//     still be set when resolution itself succeeded but coverage didn't —
//     callers report these alongside the unavailable status so a caller
//     debugging "why no result" isn't left with zero information.
//   - IsCrop=true: resolved to a specific crop, covered by the model.
//     CropCode is set; the caller already has marketCode (it was a
//     required input to get here).
//   - IsGroup=true: resolved to a crop group, covered by the model.
//     GroupCode is set.
type resolvedTarget struct {
	Unavailable bool

	ResolvedCropCode      *string
	ResolvedCropGroupCode *string
	MarketCode            *string

	IsCrop   bool
	CropCode string

	IsGroup   bool
	GroupCode string
}

// resolveCropTarget resolves cropName via the P4 mapping seed table,
// validates market_code usage (required for a specific crop, forbidden for
// a crop group), and pre-checks whether the model artifact actually covers
// the resolved target — distinguishing "model doesn't cover this
// crop/group" (a business unavailable outcome) from SeriesForecast/
// GroupForecast's own infra failures, without duplicating their calendar/
// quality-gate/status logic (see EstimatedItemPrice's own doc comment for
// why that logic must not be duplicated — this function doesn't touch it,
// it only decides whether to call it).
//
// Shared by EstimatedItemPrice.Get and ForecastCurve.Get — both need
// identical resolution + coverage-check semantics; only what they do with
// a *covered* resolution differs (one converts to a price, the other
// returns the raw 7-point curve).
//
// cropCodeBypass/groupCodeBypass are the direct-lookup path: a caller that
// already holds a canonical crop_code/group_code (e.g. the seller app after
// picking a row out of SearchCropCatalog) skips the seed table entirely.
// That matters because seed resolution is both lossy (most crops have no
// consumer alias at all) and occasionally ambiguous (ErrAmbiguousCropGroup)
// — neither of which should stand between a caller and a code it already
// knows is canonical. Everything *after* resolution (market_code validation
// and the artifact coverage pre-check) is identical on both paths, so the
// two entry points can never disagree about whether a target is usable.
func resolveCropTarget(ctx context.Context, seed mapping.Seed, repo domain.ForecastRepository, artifact forecast.ModelArtifact, cropName, marketCode, cropCodeBypass, groupCodeBypass string) (resolvedTarget, error) {
	if err := validateResolutionInputs(cropName, cropCodeBypass, groupCodeBypass); err != nil {
		return resolvedTarget{}, err
	}

	if cropCodeBypass != "" {
		if marketCode == "" {
			return resolvedTarget{}, ErrMarketCodeRequiredForCrop
		}
		if !artifact.HasSeries(marketCode, cropCodeBypass) {
			cropCode := cropCodeBypass
			mc := marketCode
			return resolvedTarget{Unavailable: true, ResolvedCropCode: &cropCode, MarketCode: &mc}, nil
		}
		return resolvedTarget{IsCrop: true, CropCode: cropCodeBypass}, nil
	}

	if groupCodeBypass != "" {
		if marketCode != "" {
			return resolvedTarget{}, ErrMarketCodeNotAllowedForGroup
		}
		crops, err := repo.CropsInGroup(ctx, groupCodeBypass)
		if err != nil {
			return resolvedTarget{}, err
		}
		activeCropCodes := make([]string, 0, len(crops))
		for _, c := range crops {
			if c.Active {
				activeCropCodes = append(activeCropCodes, c.CropCode)
			}
		}
		if len(activeCropCodes) == 0 || !artifact.HasAnySeriesForCrops(activeCropCodes) {
			gc := groupCodeBypass
			return resolvedTarget{Unavailable: true, ResolvedCropGroupCode: &gc}, nil
		}
		return resolvedTarget{IsGroup: true, GroupCode: groupCodeBypass}, nil
	}

	entry, found := seed.Resolve(cropName)
	if !found {
		return resolvedTarget{Unavailable: true}, nil
	}

	if entry.CropCode != "" {
		if marketCode == "" {
			return resolvedTarget{}, ErrMarketCodeRequiredForCrop
		}
		if !artifact.HasSeries(marketCode, entry.CropCode) {
			cropCode := entry.CropCode
			mc := marketCode
			return resolvedTarget{Unavailable: true, ResolvedCropCode: &cropCode, MarketCode: &mc}, nil
		}
		return resolvedTarget{IsCrop: true, CropCode: entry.CropCode}, nil
	}

	if marketCode != "" {
		return resolvedTarget{}, ErrMarketCodeNotAllowedForGroup
	}

	matches, err := repo.CropGroupsByName(ctx, entry.CropGroupName)
	if err != nil {
		return resolvedTarget{}, err
	}
	if len(matches) == 0 {
		return resolvedTarget{Unavailable: true}, nil
	}
	if len(matches) > 1 {
		return resolvedTarget{}, ErrAmbiguousCropGroup
	}
	groupCode := matches[0].GroupCode

	crops, err := repo.CropsInGroup(ctx, groupCode)
	if err != nil {
		return resolvedTarget{}, err
	}
	activeCropCodes := make([]string, 0, len(crops))
	for _, c := range crops {
		if c.Active {
			activeCropCodes = append(activeCropCodes, c.CropCode)
		}
	}
	if len(activeCropCodes) == 0 || !artifact.HasAnySeriesForCrops(activeCropCodes) {
		gc := groupCode
		return resolvedTarget{Unavailable: true, ResolvedCropGroupCode: &gc}, nil
	}

	return resolvedTarget{IsGroup: true, GroupCode: groupCode}, nil
}
