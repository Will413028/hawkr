package postgres

import (
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

func TestToProspectSearchHitNilDerivedFacetsDefaultToFalse(t *testing.T) {
	got := toProspectSearchHit(gen.SearchSellerProspectsRow{})

	if got.HitCategory || got.HitRegion || got.HitKeywords || got.HitCategoryIDs {
		t.Fatalf("nil derived facet pointers = %+v, want all false", got)
	}
}

func TestToProspectSearchHitPreservesDerivedFacetValues(t *testing.T) {
	category, region, keywords, categoryIDs := true, false, true, true
	got := toProspectSearchHit(gen.SearchSellerProspectsRow{
		HitCategory:    &category,
		HitRegion:      &region,
		HitKeywords:    &keywords,
		HitCategoryIds: &categoryIDs,
	})

	if !got.HitCategory || got.HitRegion || !got.HitKeywords || !got.HitCategoryIDs {
		t.Fatalf("derived facet pointers = %+v, want true/false/true/true", got)
	}
}

func TestToProspectSearchHitMapsProspectFields(t *testing.T) {
	row := gen.SearchSellerProspectsRow{
		ID:              999,
		DisplayName:     "field-round-trip-test-display-name",
		VendorType:      "business",
		Status:          "ACTIVE",
		OutreachStatus:  "NOT_CONTACTED",
		Category:        "field-round-trip-test-category",
		Region:          "field-round-trip-test-region",
		ProductSummary:  "field-round-trip-test-product",
		Positioning:     "field-round-trip-test-positioning",
		ResearchSummary: "field-round-trip-test-research",
		PublicWebsite:   "field-round-trip-test-website",
		PriorityLetter:  "Z",
		PriorityStars:   7,
	}

	got := toProspectSearchHit(row)

	if got.Prospect.ID != row.ID {
		t.Errorf("Prospect.ID = %d, want %d", got.Prospect.ID, row.ID)
	}
	if got.Prospect.DisplayName != row.DisplayName {
		t.Errorf("Prospect.DisplayName = %q, want %q", got.Prospect.DisplayName, row.DisplayName)
	}
	if got.Prospect.Category != row.Category {
		t.Errorf("Prospect.Category = %q, want %q", got.Prospect.Category, row.Category)
	}
	if got.Prospect.Region != row.Region {
		t.Errorf("Prospect.Region = %q, want %q", got.Prospect.Region, row.Region)
	}
	if got.Prospect.ProductSummary != row.ProductSummary {
		t.Errorf("Prospect.ProductSummary = %q, want %q", got.Prospect.ProductSummary, row.ProductSummary)
	}
	if got.Prospect.Positioning != row.Positioning {
		t.Errorf("Prospect.Positioning = %q, want %q", got.Prospect.Positioning, row.Positioning)
	}
	if got.Prospect.ResearchSummary != row.ResearchSummary {
		t.Errorf("Prospect.ResearchSummary = %q, want %q", got.Prospect.ResearchSummary, row.ResearchSummary)
	}
	if got.Prospect.PublicWebsite != row.PublicWebsite {
		t.Errorf("Prospect.PublicWebsite = %q, want %q", got.Prospect.PublicWebsite, row.PublicWebsite)
	}
	if got.Prospect.PriorityLetter != row.PriorityLetter {
		t.Errorf("Prospect.PriorityLetter = %q, want %q", got.Prospect.PriorityLetter, row.PriorityLetter)
	}
	if got.Prospect.PriorityStars != int(row.PriorityStars) {
		t.Errorf("Prospect.PriorityStars = %d, want %d", got.Prospect.PriorityStars, row.PriorityStars)
	}
}
