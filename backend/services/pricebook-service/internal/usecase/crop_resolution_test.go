// services/pricebook-service/internal/usecase/crop_resolution_test.go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

func TestResolveCropTarget_UnresolvedName_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	target, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, forecast.ModelArtifact{}, "火龍果", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.Unavailable {
		t.Errorf("target = %+v, want Unavailable=true", target)
	}
}

func TestResolveCropTarget_CropResolved_MissingMarketCode_ReturnsError(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	_, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, forecast.ModelArtifact{}, "甘藍-初秋", "", "", "")
	if !errors.Is(err, ErrMarketCodeRequiredForCrop) {
		t.Errorf("err = %v, want ErrMarketCodeRequiredForCrop", err)
	}
}

func TestResolveCropTarget_CropResolved_ModelDoesNotCoverSeries_ReturnsUnavailable(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	target, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, forecast.ModelArtifact{}, "甘藍-初秋", "109", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.Unavailable || target.ResolvedCropCode == nil || *target.ResolvedCropCode != "A1" {
		t.Errorf("target = %+v, want Unavailable with ResolvedCropCode=A1", target)
	}
}

func TestResolveCropTarget_CropResolved_Covered_ReturnsIsCrop(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60))
	target, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, artifact, "甘藍-初秋", "109", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.IsCrop || target.CropCode != "A1" {
		t.Errorf("target = %+v, want IsCrop with CropCode=A1", target)
	}
}

func TestResolveCropTarget_GroupResolved_WithMarketCode_ReturnsError(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	_, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, forecast.ModelArtifact{}, "甘藍菜", "109", "", "")
	if !errors.Is(err, ErrMarketCodeNotAllowedForGroup) {
		t.Errorf("err = %v, want ErrMarketCodeNotAllowedForGroup", err)
	}
}

func TestResolveCropTarget_GroupResolved_ZeroMatches_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return nil, nil
	}}
	target, err := resolveCropTarget(context.Background(), seed, repo, forecast.ModelArtifact{}, "甘藍菜", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.Unavailable {
		t.Errorf("target = %+v, want Unavailable=true", target)
	}
}

func TestResolveCropTarget_GroupResolved_AmbiguousMatches_ReturnsError(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
		return []domain.CropGroupMatch{{GroupCode: "G1"}, {GroupCode: "G2"}}, nil
	}}
	_, err := resolveCropTarget(context.Background(), seed, repo, forecast.ModelArtifact{}, "甘藍菜", "", "", "")
	if !errors.Is(err, ErrAmbiguousCropGroup) {
		t.Errorf("err = %v, want ErrAmbiguousCropGroup", err)
	}
}

func TestResolveCropTarget_GroupResolved_ZeroActiveCrops_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "G1"}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Active: false}}, nil
		},
	}
	target, err := resolveCropTarget(context.Background(), seed, repo, forecast.ModelArtifact{}, "甘藍菜", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.Unavailable || target.ResolvedCropGroupCode == nil || *target.ResolvedCropGroupCode != "G1" {
		t.Errorf("target = %+v, want Unavailable with ResolvedCropGroupCode=G1", target)
	}
}

func TestResolveCropTarget_GroupResolved_NoMatchingSeries_ReturnsUnavailable(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "G1"}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Active: true}}, nil
		},
	}
	// artifact 只覆蓋 B9，不覆蓋 A1
	artifact := testArtifact(testSeriesModel("moa:109:B9", "moa:109", "moa:B9", "N05", 118, 60))
	target, err := resolveCropTarget(context.Background(), seed, repo, artifact, "甘藍菜", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.Unavailable {
		t.Errorf("target = %+v, want Unavailable=true", target)
	}
}

func TestResolveCropTarget_GroupResolved_Covered_ReturnsIsGroup(t *testing.T) {
	seed := seedWithGroup("甘藍菜", "甘藍")
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "G1"}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Active: true}}, nil
		},
	}
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60))
	target, err := resolveCropTarget(context.Background(), seed, repo, artifact, "甘藍菜", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !target.IsGroup || target.GroupCode != "G1" {
		t.Errorf("target = %+v, want IsGroup with GroupCode=G1", target)
	}
}

// --- crop_code / group_code direct-lookup bypass (Task 7) ---
//
// The bypass path exists so a caller that already holds a canonical
// crop_code/group_code (e.g. the seller app after picking a row out of
// SearchCropCatalog) doesn't have to round-trip through the seed table's
// alias resolution — which is lossy (many crops have no alias at all) and
// can go ambiguous (ErrAmbiguousCropGroup). Every bypass test below passes
// an EMPTY cropName plus a zero-value mapping.Seed, so a regression that
// accidentally kept calling seed.Resolve would fail loudly rather than
// silently fall through to "unresolved".

func TestResolveCropTarget_CropCodeBypassSkipsSeedResolution(t *testing.T) {
	repo := &fakeForecastRepo{}
	artifact := testArtifact(testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", "N05", 50, 10))
	target, err := resolveCropTarget(context.Background(), mapping.Seed{}, repo, artifact, "", "109", "crop1", "")

	if err != nil {
		t.Fatalf("resolveCropTarget: unexpected error %v", err)
	}
	if !target.IsCrop || target.CropCode != "crop1" {
		t.Errorf("target = %+v, want IsCrop=true CropCode=crop1", target)
	}
}

func TestResolveCropTarget_CropCodeBypassModelDoesNotCoverSeries_ReturnsUnavailable(t *testing.T) {
	target, err := resolveCropTarget(context.Background(), mapping.Seed{}, &fakeForecastRepo{}, forecast.ModelArtifact{}, "", "109", "crop1", "")
	if err != nil {
		t.Fatalf("resolveCropTarget: unexpected error %v", err)
	}
	if !target.Unavailable || target.ResolvedCropCode == nil || *target.ResolvedCropCode != "crop1" {
		t.Errorf("target = %+v, want Unavailable with ResolvedCropCode=crop1", target)
	}
	if target.MarketCode == nil || *target.MarketCode != "109" {
		t.Errorf("target.MarketCode = %v, want 109", target.MarketCode)
	}
}

func TestResolveCropTarget_CropCodeBypassRequiresMarketCode(t *testing.T) {
	_, err := resolveCropTarget(context.Background(), mapping.Seed{}, &fakeForecastRepo{}, forecast.ModelArtifact{}, "", "", "crop1", "")
	if !errors.Is(err, ErrMarketCodeRequiredForCrop) {
		t.Errorf("err = %v, want ErrMarketCodeRequiredForCrop", err)
	}
}

func TestResolveCropTarget_GroupCodeBypassForbidsMarketCode(t *testing.T) {
	_, err := resolveCropTarget(context.Background(), mapping.Seed{}, &fakeForecastRepo{}, forecast.ModelArtifact{}, "", "109", "", "group1")
	if !errors.Is(err, ErrMarketCodeNotAllowedForGroup) {
		t.Errorf("err = %v, want ErrMarketCodeNotAllowedForGroup", err)
	}
}

func TestResolveCropTarget_GroupCodeBypassSkipsNameResolution(t *testing.T) {
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			t.Fatalf("CropGroupsByName must not be called on the group_code bypass path (name=%q)", name)
			return nil, nil
		},
	}
	artifact := testArtifact(testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", "N05", 50, 10))
	target, err := resolveCropTarget(context.Background(), mapping.Seed{}, repo, artifact, "", "", "", "group1")

	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !target.IsGroup || target.GroupCode != "group1" {
		t.Errorf("target = %+v, want IsGroup=true GroupCode=group1", target)
	}
}

func TestResolveCropTarget_GroupCodeBypassZeroActiveCrops_ReturnsUnavailable(t *testing.T) {
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: false}}, nil
		},
	}
	target, err := resolveCropTarget(context.Background(), mapping.Seed{}, repo, forecast.ModelArtifact{}, "", "", "", "group1")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !target.Unavailable || target.ResolvedCropGroupCode == nil || *target.ResolvedCropGroupCode != "group1" {
		t.Errorf("target = %+v, want Unavailable with ResolvedCropGroupCode=group1", target)
	}
}

func TestResolveCropTarget_ConflictingInputsRejected(t *testing.T) {
	_, err := resolveCropTarget(context.Background(), mapping.Seed{}, &fakeForecastRepo{}, forecast.ModelArtifact{}, "", "", "crop1", "group1")
	if !errors.Is(err, ErrConflictingResolutionInputs) {
		t.Errorf("err = %v, want ErrConflictingResolutionInputs", err)
	}
}

func TestResolveCropTarget_ConflictingInputsRejected_NameAndCode(t *testing.T) {
	seed := seedWithCrop("甘藍-初秋", "A1")
	_, err := resolveCropTarget(context.Background(), seed, &fakeForecastRepo{}, forecast.ModelArtifact{}, "甘藍-初秋", "109", "crop1", "")
	if !errors.Is(err, ErrConflictingResolutionInputs) {
		t.Errorf("err = %v, want ErrConflictingResolutionInputs", err)
	}
}

func TestResolveCropTarget_NoInputsRejected(t *testing.T) {
	_, err := resolveCropTarget(context.Background(), mapping.Seed{}, &fakeForecastRepo{}, forecast.ModelArtifact{}, "", "", "", "")
	if !errors.Is(err, ErrNoResolutionInput) {
		t.Errorf("err = %v, want ErrNoResolutionInput", err)
	}
}
