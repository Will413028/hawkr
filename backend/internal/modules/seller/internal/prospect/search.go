package prospect

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// 搜尋結果的筆數：預設與上限（服務端的安全上限，不是相關性排序的截斷點）。
const (
	defaultSearchLimit = 500
	maxSearchLimit     = 1000
)

// Search 是採購候選的搜尋（P5，給 ai-service）：只看 ACTIVE 的業者（已入駐、封存與被合併的不會出現）；填了的條件是加權 OR，
// 不是 AND——分類與地區都對、描述沒有字面提到關鍵字的候選不能消失；category、region 與 keywords 是部分比對（ILIKE，使用者的 %
// 與 _ 不當萬用字元），分類樹 ID 走子表。全部沒填回全部 ACTIVE 的業者。結果不排序：算分與排序是呼叫端的事，Limit 只是安全上限。
func (s *Service) Search(ctx context.Context, in model.ProspectSearchInput) ([]model.ProspectSearchHit, error) {
	limit, err := pagination.PageSize(in.Limit, defaultSearchLimit, maxSearchLimit)
	if err != nil {
		return nil, err
	}
	categoryIDs := make([]string, 0, len(in.CategoryIDs))
	for _, id := range in.CategoryIDs {
		if id = strings.TrimSpace(id); id != "" {
			categoryIDs = append(categoryIDs, id)
		}
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).SearchProspects(ctx, store.SearchProspectsParams{
		Category: model.LikePattern(in.Category), Region: model.LikePattern(in.Region), Keywords: model.LikePattern(in.Keywords),
		CategoryIds: categoryIDs, RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	hits := make([]model.ProspectSearchHit, len(rows))
	for i, row := range rows {
		hits[i] = model.ProspectSearchHit{
			Prospect: toProspect(row.SellerProspect), Website: row.PrimaryWebsite, HitCategory: row.HitCategory, HitRegion: row.HitRegion,
			HitKeywords: row.HitKeywords, HitCategoryIDs: row.HitCategoryIds,
		}
	}
	return hits, nil
}

// Contact 回傳業者的主要 email 與狀態（vendor portal 邀請供應商用），任何狀態都讀得到；沒有 email 時 Email 是 nil。
// 讀取對那一列帶 FOR SHARE：呼叫方在自己的 transaction 內讀，就和封存、合併（FOR UPDATE）與編輯（FOR NO KEY UPDATE）
// 序列化，讀到 ACTIVE 之後到呼叫方提交之前，業者不會被封存。transaction 外呼叫時鎖在語句結束就釋放，等於普通的讀取。
func (s *Service) Contact(ctx context.Context, id int64) (model.ProspectContact, error) {
	if err := model.Positive(id); err != nil {
		return model.ProspectContact{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).ProspectContactForShare(ctx, id)
	if err != nil {
		return model.ProspectContact{}, notFound(err)
	}
	return model.ProspectContact{ProspectID: row.ProspectID, DisplayName: row.DisplayName, Email: row.PrimaryEmail, Status: model.ProspectStatus(row.Status)}, nil
}

// Duplicates 是疑似重複的清單：尚未處理的候選配對（ACTIVE 的業者之間比對鍵相同或名稱相同，排除已標記為不是同一家的，ID 小的在
// 左），keyset，預設 50、上限 200。管理員確認不是同一家（建立、修改、還原時帶 ConfirmedDistinct）或合併之後，那一對就不再出現。
func (s *Service) Duplicates(ctx context.Context, pageSize int32, after *model.DuplicateCursor) (model.DuplicatePage, error) {
	size, err := pagination.PageSize(pageSize, 50, 200)
	if err != nil {
		return model.DuplicatePage{}, err
	}
	params := store.DuplicateProspectPairsParams{PageSize: size + 1}
	if after != nil {
		params.AfterLeftID, params.AfterRightID = &after.LeftID, &after.RightID
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).DuplicateProspectPairs(ctx, params)
	if err != nil {
		return model.DuplicatePage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	page := model.DuplicatePage{Items: make([]model.DuplicatePair, len(rows))}
	for i, row := range rows {
		page.Items[i] = model.DuplicatePair{Left: toProspect(row.SellerProspect), Right: toProspect(row.SellerProspect_2), MatchedOn: reasonsOf(row.MatchedOn)}
	}
	if hasMore {
		last := page.Items[len(page.Items)-1]
		page.Next = &model.DuplicateCursor{LeftID: last.Left.ID, RightID: last.Right.ID}
	}
	return page, nil
}

// TaxIDMatches 是統編對帳：ACTIVE 業者的統編對得上某個目前店家的 KYC 申請、但還沒有連起來（轉成店家用 Convert）。申請以
// store_id 為鍵，不分申請狀態（草稿與未核准的也代表這家已經來了），同一個店家取最新一筆申請；keyset，預設 50、上限 200。
func (s *Service) TaxIDMatches(ctx context.Context, pageSize int32, after *model.TaxIDMatchCursor) (model.TaxIDMatchPage, error) {
	size, err := pagination.PageSize(pageSize, 50, 200)
	if err != nil {
		return model.TaxIDMatchPage{}, err
	}
	params := store.ProspectTaxIDMatchesParams{PageSize: size + 1}
	if after != nil {
		params.AfterProspectID, params.AfterStoreID = &after.ProspectID, &after.StoreID
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.ProspectTaxIDMatches(ctx, params)
	if err != nil {
		return model.TaxIDMatchPage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ProspectID
	}
	prospects, err := q.ProspectsByIDs(ctx, ids)
	if err != nil {
		return model.TaxIDMatchPage{}, err
	}
	byID := make(map[int64]model.Prospect, len(prospects))
	for _, row := range prospects {
		byID[row.ProspectID] = toProspect(row)
	}
	page := model.TaxIDMatchPage{Items: make([]model.TaxIDMatch, len(rows))}
	for i, row := range rows {
		page.Items[i] = model.TaxIDMatch{
			Prospect: byID[row.ProspectID], StoreID: row.StoreID, StoreName: row.StoreName, CompanyName: row.CompanyName,
			KYCStatus: model.KYCStatus(row.KycStatus),
		}
	}
	if hasMore {
		last := rows[len(rows)-1]
		page.Next = &model.TaxIDMatchCursor{ProspectID: last.ProspectID, StoreID: last.StoreID}
	}
	return page, nil
}

// reasonsOf 把查詢回的原因（'NAME' 或比對鍵的前綴）轉成模組的值。
func reasonsOf(in []string) []model.MatchReason {
	out := make([]model.MatchReason, len(in))
	for i, s := range in {
		out[i] = model.MatchReason(s)
	}
	return out
}
