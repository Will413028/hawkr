package media_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func TestRetiredAssetsIsADeduplicatedSetDifference(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	for name, tc := range map[string]struct {
		before, after, want []uuid.UUID
	}{
		"same asset at two positions, one replaced": {[]uuid.UUID{a, a}, []uuid.UUID{b, a}, nil},
		"both positions replaced retire it once":    {[]uuid.UUID{a, a}, []uuid.UUID{b, c}, []uuid.UUID{a}},
		"unchanged":                                 {[]uuid.UUID{a, b}, []uuid.UUID{a, b}, nil},
		"removed":                                   {[]uuid.UUID{a, b}, nil, []uuid.UUID{a, b}},
		"nothing before":                            {nil, []uuid.UUID{a}, nil},
	} {
		if got := media.RetiredAssets(tc.before, tc.after); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

func TestAssetIDsSkipsEmptyPositions(t *testing.T) {
	a := uuid.New()
	if got := media.AssetIDs(nil, &a, nil); !slices.Equal(got, []uuid.UUID{a}) {
		t.Fatalf("got %v", got)
	}
	if got := media.AssetIDs(); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
