package recall

import (
	"context"
	"fmt"

	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

// DefaultEntityLimit 是 SearchPriceableEntities 的候選上限（去重後的實體數）。
// 固定 20：卡片最多顯示 8–10 個，多取一倍讓過濾（無分類的 commodity、算不出價的 crop）
// 之後仍有材料；再大只是把 crop_category_model 的 256 候選那種硬上限往這裡搬。
const DefaultEntityLimit int32 = 20

type entityClient interface {
	SearchPriceableEntities(ctx context.Context, variants []string, limit int32) ([]pbusecase.PriceableEntityMatch, error)
}

// EntityRetriever 打 pricebook 的 SearchPriceableEntities：crop／group／commodity 同場。
// 它是主要來源，失敗往上拋（spec §錯誤處理刻意不一致）。
type EntityRetriever struct {
	client entityClient
	limit  int32
}

func NewEntityRetriever(c entityClient) *EntityRetriever {
	return &EntityRetriever{client: c, limit: DefaultEntityLimit}
}

func (r *EntityRetriever) Name() string { return "entity" }

func (r *EntityRetriever) Recall(ctx context.Context, q Query) ([]Candidate, error) {
	matches, err := r.client.SearchPriceableEntities(ctx, q.Variants, r.limit)
	if err != nil {
		return nil, fmt.Errorf("entity retriever: %w", err)
	}
	out := make([]Candidate, 0, len(matches))
	for _, m := range matches {
		kind := Kind(m.Kind)
		out = append(out, Candidate{
			Key: CandidateKey(kind, m.Code), Kind: kind, Code: m.Code, Label: m.Name,
			CategoryIDs: append([]string(nil), m.CategoryIDs...),
			GroupCode:   m.GroupCode, HasForecast: m.HasForecast,
			MatchedBy: m.MatchedForm, Quality: QualityOf(m.FormKind, m.Tier), Rank: m.Rank,
		})
	}
	return out, nil
}

// QualityOf：精確命中在正式名／來源術語／人工別名是強訊號；精確命中在品種名是中；
// 前綴與子串一律弱。ALIAS 精確視為強——那是人工對應，不是模型猜的。
func QualityOf(formKind string, tier int) Quality {
	if tier != 0 {
		return Weak
	}
	switch formKind {
	case "CANONICAL", "SOURCE_TERM", "ALIAS":
		return Strong
	case "VARIANT":
		return Medium
	}
	return Weak
}
