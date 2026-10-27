package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	rootusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

type categoryAncestors interface {
	AncestorIDs(ctx context.Context, categoryID string) []string
}

type ExternalLaneSearcher struct {
	searcher  rootusecase.CandidateLeadSearcher
	ancestors categoryAncestors
}

func NewExternalLaneSearcher(searcher rootusecase.CandidateLeadSearcher, ancestors categoryAncestors) *ExternalLaneSearcher {
	return &ExternalLaneSearcher{searcher: searcher, ancestors: ancestors}
}

func (s *ExternalLaneSearcher) Search(ctx context.Context, slots procurementdomain.Slots) ([]repository.Candidate, error) {
	keywords := joinNonEmpty(slots.Item)
	if keywords == "" {
		return nil, nil
	}

	var categoryIDs []string
	if s.ancestors != nil && slots.ItemResolution != nil {
		categoryIDs = s.ancestors.AncestorIDs(ctx, slots.ItemResolution.CategoryID)
	}

	matches, err := s.searcher.Search(ctx, rootusecase.CandidateSearchParams{Keywords: keywords, CategoryIDs: categoryIDs})
	if err != nil {
		return nil, err
	}

	unverifiableKeys := []procurementdomain.SlotKey{
		procurementdomain.SlotQuantity, procurementdomain.SlotUnit, procurementdomain.SlotBudgetPerUnit,
		procurementdomain.SlotPackaging, procurementdomain.SlotDeliveryCity, procurementdomain.SlotDeliveryDate,
	}

	out := make([]repository.Candidate, 0, len(matches))
	for i, m := range matches {

		matched := make([]procurementdomain.MatchFact, 0, 2)

		if len(m.MatchedFacets) > 0 {
			if slots.Item != nil && strings.TrimSpace(*slots.Item) != "" {
				matched = append(matched, procurementdomain.MatchFactFor(procurementdomain.SlotItem))
			}
		}

		unverifiable := make([]procurementdomain.UnverifiableFact, 0, len(unverifiableKeys)+1)
		unverifiable = append(unverifiable, procurementdomain.ExternalLaneDisclosure())
		for _, key := range unverifiableKeys {
			if externalSlotHasValue(slots, key) {
				unverifiable = append(unverifiable, procurementdomain.UnverifiableFact{
					Key: key, Reason: procurementdomain.NeedConfirmReason(key, slots),
				})
			}
		}

		matchedJSON, _ := json.Marshal(matched)
		unverifiableJSON, _ := json.Marshal(unverifiable)

		out = append(out, repository.Candidate{
			Lane: string(procurementdomain.LaneExternal), SourceID: strconv.FormatInt(m.Vendor.ID, 10),
			DisplayName: m.Vendor.DisplayName, Score: m.Score,
			MatchedFactsJSON: matchedJSON, UnverifiableFactsJSON: unverifiableJSON,
			FactsAsOf: m.Vendor.FactsAsOf, Rank: i,
			ProspectID:    &m.Vendor.ID,
			BrandLogoKey:  m.Vendor.LogoKey,
			BrandCoverKey: m.Vendor.CoverKey,
		})
	}
	return out, nil
}

func externalSlotHasValue(s procurementdomain.Slots, key procurementdomain.SlotKey) bool {
	switch key {
	case procurementdomain.SlotQuantity:
		return s.Quantity != nil
	case procurementdomain.SlotUnit:
		return s.Unit != nil
	case procurementdomain.SlotBudgetPerUnit:
		return s.BudgetPerUnit != nil
	case procurementdomain.SlotPackaging:
		return s.Packaging != nil
	case procurementdomain.SlotDeliveryCity:
		return s.DeliveryCity != nil
	case procurementdomain.SlotDeliveryDate:
		return s.DeliveryDate != nil
	}
	return false
}
