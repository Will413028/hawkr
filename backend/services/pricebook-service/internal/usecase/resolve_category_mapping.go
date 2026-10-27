package usecase

import (
	"context"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

type ResolveCategoryMapping struct {
	repo domain.CategoryMappingRepository
}

func NewResolveCategoryMapping(repo domain.CategoryMappingRepository) *ResolveCategoryMapping {
	return &ResolveCategoryMapping{repo: repo}
}

const CategoryMappingVersion = "pricebook-category-v1"

func (u *ResolveCategoryMapping) Get(ctx context.Context, categoryID string) (domain.CategoryMapping, error) {
	row, err := u.repo.Get(ctx, categoryID)
	if err != nil {
		return domain.CategoryMapping{}, err
	}
	if row == nil {
		return domain.CategoryMapping{CategoryID: categoryID, Status: domain.CategoryMappingUnmapped, MappingVersion: CategoryMappingVersion}, nil
	}
	if row.Status != domain.CategoryMappingResolved {
		return *row, nil
	}

	var name *string
	switch {
	case row.GroupCode != nil:
		name, err = u.repo.ActiveGroupName(ctx, *row.GroupCode)
	case row.CommodityID != nil:
		name, err = u.repo.ActiveCommodityName(ctx, *row.CommodityID)
	default:
		return domain.CategoryMapping{CategoryID: row.CategoryID, Status: domain.CategoryMappingUnmapped, MappingVersion: row.MappingVersion}, nil
	}
	if err != nil {
		return domain.CategoryMapping{}, err
	}
	if name == nil {
		return domain.CategoryMapping{CategoryID: row.CategoryID, Status: domain.CategoryMappingUnmapped, MappingVersion: row.MappingVersion}, nil
	}
	row.Label = name

	if row.GroupCode != nil {
		varieties, verr := u.repo.ListCategoryCropVarieties(ctx, row.CategoryID)
		if verr != nil {
			return domain.CategoryMapping{}, verr
		}
		row.CropVarieties = varieties
	}
	return *row, nil
}
