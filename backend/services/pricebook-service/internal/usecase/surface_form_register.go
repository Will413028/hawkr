package usecase

import (
	"context"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func surfaceFormsForCropGroup(groupCode, name string) []domain.SurfaceFormRegistration {
	return nonBlankForms([]domain.SurfaceFormRegistration{
		{EntityKind: domain.EntityKindCropGroup, EntityCode: groupCode, FormRaw: name, FormKind: domain.FormKindCanonical},
	})
}

func surfaceFormsForCrop(cropCode, name, variant string) []domain.SurfaceFormRegistration {
	return nonBlankForms([]domain.SurfaceFormRegistration{
		{EntityKind: domain.EntityKindCrop, EntityCode: cropCode, FormRaw: name, FormKind: domain.FormKindCanonical},
		{EntityKind: domain.EntityKindCrop, EntityCode: cropCode, FormRaw: variant, FormKind: domain.FormKindVariant},
	})
}

func surfaceFormsForCommodity(commodityID int64, name string, sourceTerms, buyerAliases []string) []domain.SurfaceFormRegistration {
	code := strconv.FormatInt(commodityID, 10)
	regs := make([]domain.SurfaceFormRegistration, 0, 1+len(sourceTerms)+len(buyerAliases))
	regs = append(regs, domain.SurfaceFormRegistration{
		EntityKind: domain.EntityKindCommodity, EntityCode: code, FormRaw: name, FormKind: domain.FormKindCanonical,
	})
	for _, term := range sourceTerms {
		regs = append(regs, domain.SurfaceFormRegistration{
			EntityKind: domain.EntityKindCommodity, EntityCode: code, FormRaw: term, FormKind: domain.FormKindSourceTerm,
		})
	}
	for _, alias := range buyerAliases {
		regs = append(regs, domain.SurfaceFormRegistration{
			EntityKind: domain.EntityKindCommodity, EntityCode: code, FormRaw: alias, FormKind: domain.FormKindAlias,
		})
	}
	return nonBlankForms(regs)
}

func nonBlankForms(regs []domain.SurfaceFormRegistration) []domain.SurfaceFormRegistration {
	out := make([]domain.SurfaceFormRegistration, 0, len(regs))
	for _, r := range regs {
		if strings.TrimSpace(r.FormRaw) != "" {
			out = append(out, r)
		}
	}
	return out
}

func registerCropGroupForms(ctx context.Context, q *gen.Queries, groupCode, name string) error {
	return postgres.RegisterSurfaceForms(ctx, q, surfaceFormsForCropGroup(groupCode, name))
}

func registerCropForms(ctx context.Context, q *gen.Queries, cropCode, name, variant string) error {
	return postgres.RegisterSurfaceForms(ctx, q, surfaceFormsForCrop(cropCode, name, variant))
}

func registerCommodityForms(ctx context.Context, q *gen.Queries, commodityID int64, name string, sourceTerms, buyerAliases []string) error {
	return postgres.RegisterSurfaceForms(ctx, q, surfaceFormsForCommodity(commodityID, name, sourceTerms, buyerAliases))
}
