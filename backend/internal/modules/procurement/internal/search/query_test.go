package search

import (
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

func revision(kind, label *string, category *uuid.UUID) store.ProcurementRequestRevision {
	return store.ProcurementRequestRevision{ID: uuid.NewV7(), ItemResolutionKind: kind, ItemResolutionLabel: label, ItemResolutionCategoryID: category}
}

func sp(s string) *string { return &s }

func TestQueryUsesTheCategoryIDAndNoKeywordWhenTheCategoryIsConfirmed(t *testing.T) {
	cat := categoryID(18)
	got, err := queryOf(revision(sp("CATEGORY_CONFIRMED"), sp("小白菜"), &cat))
	if err != nil || got != (candidate.Query{CategoryID: categoryID(18)}) {
		t.Fatalf("query = %+v, %v", got, err)
	}
}

func TestQueryUsesTheDescribedTextAsKeywordForBuyerDescribed(t *testing.T) {
	got, err := queryOf(revision(sp("BUYER_DESCRIBED"), sp("有機高麗菜"), nil))
	if err != nil || got != (candidate.Query{Keyword: "有機高麗菜"}) {
		t.Fatalf("query = %+v, %v", got, err)
	}
}

func TestQueryRejectsRevisionsWithoutAUsableResolution(t *testing.T) {
	zero := uuid.Nil()
	for name, rev := range map[string]store.ProcurementRequestRevision{
		"no resolution":     revision(nil, nil, nil),
		"confirmed no id":   revision(sp("CATEGORY_CONFIRMED"), sp("x"), nil),
		"confirmed zero id": revision(sp("CATEGORY_CONFIRMED"), sp("x"), &zero),
		"unknown kind":      revision(sp("GUESSED"), sp("x"), nil),
	} {
		if _, err := queryOf(rev); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
