package grpc

import (
	"context"
	"strconv"
	"time"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

type prospectSearcher interface {
	Search(ctx context.Context, params domain.ProspectSearchParams) ([]domain.ProspectSearchHit, error)
}

type prospectCreator interface {
	Match(ctx context.Context, in domain.ProspectContactInput) ([]usecase.ProspectMatchCandidate, error)
	Create(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error)
}

func (s *SellerServer) SearchSellerProspects(ctx context.Context, req *sellerpb.SearchSellerProspectsRequest) (*sellerpb.SearchSellerProspectsResponse, error) {
	hits, err := s.prospectSearcher.Search(ctx, domain.ProspectSearchParams{
		Category:    req.GetCategory(),
		Region:      req.GetRegion(),
		Keywords:    req.GetKeywords(),
		CategoryIDs: req.GetCategoryIds(),
		Limit:       int(req.GetLimit()),
	})
	if err != nil {
		return nil, err
	}
	data := make([]*sellerpb.SellerProspectHit, 0, len(hits))
	for _, h := range hits {
		data = append(data, &sellerpb.SellerProspectHit{
			Prospect:       toProspectSearchItemPB(h.Prospect),
			HitCategory:    h.HitCategory,
			HitRegion:      h.HitRegion,
			HitKeywords:    h.HitKeywords,
			HitCategoryIds: h.HitCategoryIDs,
		})
	}
	return &sellerpb.SearchSellerProspectsResponse{Code: 0, Data: data}, nil
}

func toProspectItemPB(p domain.SellerProspect) *sellerpb.SellerProspectItem {
	return &sellerpb.SellerProspectItem{
		Id:              strconv.FormatInt(p.ID, 10),
		DisplayName:     p.DisplayName,
		VendorType:      p.VendorType,
		Category:        p.Category,
		Region:          p.Region,
		ProductSummary:  p.ProductSummary,
		Positioning:     p.Positioning,
		ResearchSummary: p.ResearchSummary,
		PriorityLetter:  p.PriorityLetter,
		PriorityStars:   strconv.Itoa(p.PriorityStars),
		PublicEmail:     p.PublicEmail,
		PublicPhone:     p.PublicPhone,
		PublicWebsite:   p.PublicWebsite,
		PublicSocial:    p.PublicSocial,
		UpdatedAt:       p.UpdatedAt.UTC().Format(time.RFC3339),
		// 品牌素材。窄投影刻意保留這兩欄（不像 public_email／phone／social 被清空）：
		// 它們不是聯絡方式也不是識別鍵，是候選卡要顯示的品牌圖。
		LogoKey:  p.LogoKey,
		CoverKey: p.CoverKey,
	}
}

func toProspectSearchItemPB(p domain.SellerProspect) *sellerpb.SellerProspectItem {
	item := toProspectItemPB(p)
	item.PublicEmail = ""
	item.PublicPhone = ""
	item.PublicSocial = ""
	return item
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func (s *SellerServer) CreateSellerProspect(ctx context.Context, req *sellerpb.CreateSellerProspectRequest) (*sellerpb.CreateSellerProspectResponse, error) {
	contact := domain.ProspectContactInput{
		TaxID:   req.GetTaxId(),
		Email:   req.GetPublicEmail(),
		Phone:   req.GetPublicPhone(),
		Website: req.GetPublicWebsite(),
		Social:  req.GetPublicSocial(),
	}

	matches, err := s.prospectCreator.Match(ctx, contact)
	if err != nil {
		return nil, err
	}
	pbMatches := make([]*sellerpb.ProspectMatchCandidate, 0, len(matches))
	for _, m := range matches {
		pbMatches = append(pbMatches, &sellerpb.ProspectMatchCandidate{
			Prospect: toProspectItemPB(m.Prospect), Tier: string(m.Tier), MatchedOn: m.MatchedOn,
		})
	}

	if req.GetDryRun() {
		return &sellerpb.CreateSellerProspectResponse{Code: 0, Matches: pbMatches}, nil
	}

	keys := domain.BuildProspectMatchKeys(contact)
	in := domain.SellerProspect{
		DisplayName: req.GetDisplayName(), VendorType: req.GetVendorType(),
		Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED",
		MatchKeys:       keys,
		PublicEmail:     req.GetPublicEmail(),
		PublicPhone:     req.GetPublicPhone(),
		PublicWebsite:   req.GetPublicWebsite(),
		PublicSocial:    req.GetPublicSocial(),
		Category:        req.GetCategory(),
		Region:          req.GetRegion(),
		ProductSummary:  req.GetProductSummary(),
		Positioning:     req.GetPositioning(),
		ResearchSummary: req.GetResearchSummary(),
		PriorityLetter:  req.GetPriorityLetter(),

		PriorityStars: atoiOrZero(req.GetPriorityStars()),
	}
	if keys.TaxID != "" {
		taxID := keys.TaxID
		in.TaxID = &taxID
	}

	if createdBy, err := strconv.ParseInt(req.GetCreatedByAccountId(), 10, 64); err == nil && createdBy != 0 {
		in.CreatedByAccountID = &createdBy
		in.UpdatedByAccountID = &createdBy
	}

	if raw := req.GetFactsAsOf(); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			in.UpdatedAt = t
		}
	}

	created, err := s.prospectCreator.Create(ctx, in)
	if err != nil {

		if cs := prospectConflictStatus(err); cs != nil {
			return nil, cs
		}
		return nil, err
	}
	return &sellerpb.CreateSellerProspectResponse{
		Code: 0, Data: toProspectItemPB(created), Matches: pbMatches,
	}, nil
}
