package images

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestRequireNamesTheSlotWithoutAnyImage(t *testing.T) {
	err := Require(t.Context(), Slot{Field: "PreviewImageToken", Current: "k"}, Slot{Field: "PopupImageToken"})
	var got *apperr.Error
	if !errors.As(err, &got) || len(got.FieldViolations()) != 1 || got.FieldViolations()[0].Field != "PopupImageToken" {
		t.Fatalf("err = %v，預期指出 PopupImageToken", err)
	}
	if err := Require(t.Context(), Slot{Field: "A", Token: "t"}, Slot{Field: "B", Current: "k"}); err != nil {
		t.Fatalf("有 token 或現有圖都合法：%v", err)
	}
}

func TestRetiredAssetsAcrossAllSlotsOfTheRef(t *testing.T) {
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	old := []uuid.UUID{a, a}
	replacedOne := Resolved{Key: "k", AssetID: &b}
	kept := Resolved{Key: "k2"}
	if got := media.RetiredAssets(old, After(old, replacedOne, kept)); len(got) != 0 {
		t.Fatalf("資產仍被別的位置指著，不該退場：%v", got)
	}
	if got := media.RetiredAssets(old, After(old, replacedOne, Resolved{Key: "k3", AssetID: &c})); !slices.Equal(got, []uuid.UUID{a}) {
		t.Fatalf("兩個位置都換掉：%v", got)
	}
	two := []uuid.UUID{a, b}
	if got := media.RetiredAssets(two, After(two, kept, Resolved{Key: "k4", AssetID: &b})); len(got) != 0 {
		t.Fatalf("沿用與同一個資產：%v", got)
	}
	if got := media.RetiredAssets(old, nil); !slices.Equal(got, []uuid.UUID{a}) {
		t.Fatalf("刪除：%v", got)
	}
}

func TestHasNewIsTrueOnlyWhenSomeSlotWasConfirmed(t *testing.T) {
	id := uuid.NewV7()
	if HasNew(Resolved{Key: "a"}, Resolved{Key: "b"}) {
		t.Fatal("全部沿用時不該有新資產")
	}
	if !HasNew(Resolved{Key: "a"}, Resolved{Key: "b", AssetID: &id}) {
		t.Fatal("有一張新資產")
	}
}
