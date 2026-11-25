// Package taxonomy 讀平台分類樹：兩層、由 migration 的種子定義，沒有執行期寫入。
package taxonomy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是分類讀取的依賴。
type Deps struct{ DB *db.DB }

// Service 是分類的 use case。
type Service struct{ d Deps }

// New 建立分類 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// List 回傳整棵樹，頂層在前，同層依 position。
func (s *Service) List(ctx context.Context) ([]model.Category, error) {
	rows, err := s.q(ctx).ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.Category, 0, len(rows))
	for _, r := range rows {
		c, err := convert(r.ID, r.ParentID, r.Name, r.ShopifyTaxonomyID, r.Position)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// RequireLeaf 在分類不存在、是頂層分組或有子節點時回 false：商品只能掛葉節點。
func (s *Service) RequireLeaf(ctx context.Context, id uuid.UUID) (bool, error) {
	return s.q(ctx).IsLeafCategory(ctx, id)
}

// Get 讀一個分類；不存在回 (零值, false)。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (model.Category, bool, error) {
	r, err := s.q(ctx).GetCategory(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Category{}, false, nil
	}
	if err != nil {
		return model.Category{}, false, err
	}
	c, err := convert(r.ID, r.ParentID, r.Name, r.ShopifyTaxonomyID, r.Position)
	return c, err == nil, err
}

func convert(id uuid.UUID, parent *uuid.UUID, name []byte, shopify *string, pos int32) (model.Category, error) {
	var n struct {
		ZhTW string `json:"zh-TW"`
		En   string `json:"en"`
	}
	if err := json.Unmarshal(name, &n); err != nil {
		return model.Category{}, fmt.Errorf("catalog: category %s name: %w", id, err)
	}
	c := model.Category{ID: id, ParentID: parent, NameZhTW: n.ZhTW, NameEn: n.En, Position: pos}
	if shopify != nil {
		c.ShopifyTaxonomyID = *shopify
	}
	return c, nil
}
