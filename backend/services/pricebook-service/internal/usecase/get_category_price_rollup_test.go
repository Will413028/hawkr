package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

func TestGetCategoryPriceRollup_Get_PassesThroughRepository(t *testing.T) {
	want := &domain.CategoryPriceRollup{CategoryID: "cat-a", Level: 3, Status: domain.RollupStatusReady, LatestPrice: floatPtr(40)}
	repo := &fakeRollupRepo{written: map[string]domain.CategoryPriceRollup{"cat-a": *want}}
	u := NewGetCategoryPriceRollup(repo)

	got, err := u.Get(context.Background(), "cat-a")
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if got == nil || got.CategoryID != "cat-a" || got.LatestPrice == nil || *got.LatestPrice != 40 {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
}

func TestGetCategoryPriceRollup_Get_NoRowReturnsNil(t *testing.T) {
	u := NewGetCategoryPriceRollup(&fakeRollupRepo{})
	got, err := u.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if got != nil {
		t.Errorf("Get = %+v, want nil", got)
	}
}
