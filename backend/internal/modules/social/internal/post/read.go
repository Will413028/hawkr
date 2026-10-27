package post

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/hashtag"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func toPost(p store.SocialPost, s store.SocialPostStat, tags []model.Hashtag) model.Post {
	out := model.Post{
		ID: p.ID, StoreID: p.StoreID, Body: p.Body, ProductID: p.ProductID, Status: model.PostStatus(p.Status),
		PublishedAt: p.PublishedAt, Version: p.Version, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, Hashtags: tags,
		Stats: model.PostStats{Likes: s.LikeCount, Comments: s.CommentCount, Views: s.ViewCount, Shares: s.ShareCount},
	}
	if p.ImageKey != nil {
		out.ImageKey = *p.ImageKey
	}
	return out
}

type postRow struct {
	post  store.SocialPost
	stats store.SocialPostStat
}

// withTags 補上每篇貼文的標籤（一次批次讀取）。
func (s *Service) withTags(ctx context.Context, rows []postRow) ([]model.Post, error) {
	out := make([]model.Post, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.post.ID
	}
	tagRows, err := s.q(ctx).ListHashtagsOfPosts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("social：讀取貼文標籤：%w", err)
	}
	byPost := make(map[uuid.UUID][]model.Hashtag, len(rows))
	for _, t := range tagRows {
		byPost[t.PostID] = append(byPost[t.PostID], model.Hashtag{Name: t.Name, Display: t.DisplayName})
	}
	for i, r := range rows {
		out[i] = toPost(r.post, r.stats, byPost[r.post.ID])
	}
	return out, nil
}

// Own 讀店家自己的貼文（任何狀態，不含刪除）；別家的貼文與不存在一樣回 NotFound。
func (s *Service) Own(ctx context.Context, storeID int64, id uuid.UUID) (model.Post, error) {
	row, err := s.q(ctx).GetPost(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.SocialPost.StoreID != storeID {
		return model.Post{}, notFound(ctx)
	}
	if err != nil {
		return model.Post{}, fmt.Errorf("social：讀取貼文 %s：%w", id, err)
	}
	posts, err := s.withTags(ctx, []postRow{{row.SocialPost, row.SocialPostStat}})
	if err != nil {
		return model.Post{}, err
	}
	return posts[0], nil
}

// viewerOf 是公開讀取的觀看者：呼叫者的買家身分；零值是未登入。
func viewerOf(c model.Caller) visibility.Viewer { return visibility.Viewer{BuyerID: c.BuyerID} }

// Public 讀觀看者可讀的貼文（已發布、沒有刪除、店家可曝光、觀看者沒有封鎖這家店）；其他一律 NotFound（SD1）。讀取不計瀏覽數（RecordView 另外呼叫）。
func (s *Service) Public(ctx context.Context, c model.Caller, id uuid.UUID) (model.Post, error) {
	row, err := reads.One[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.GetPublicPost(id, s.d.Now().UTC(), viewerOf(c)))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Post{}, notFound(ctx)
	}
	if err != nil {
		return model.Post{}, fmt.Errorf("social：讀取公開貼文 %s：%w", id, err)
	}
	posts, err := s.withTags(ctx, []postRow{{row.SocialPost, row.SocialPostStat}})
	if err != nil {
		return model.Post{}, err
	}
	return posts[0], nil
}

// page 把多取一筆的結果切成一頁。
func page[R any](rows []R, size int32) ([]R, bool) {
	if len(rows) > int(size) {
		return rows[:size], true
	}
	return rows, false
}

func (s *Service) feedPage(ctx context.Context, rows []postRow, size int32, more bool) (model.PostPage, error) {
	posts, err := s.withTags(ctx, rows)
	if err != nil {
		return model.PostPage{}, err
	}
	out := model.PostPage{Posts: posts}
	if more {
		last := rows[len(rows)-1].post
		out.Next = &model.PostCursor{PublishedAt: *last.PublishedAt, ID: last.ID}
	}
	return out, nil
}

// ListFeed 是動態牆：公開貼文，最新發布的在前（keyset，不回總數）。
func (s *Service) ListFeed(ctx context.Context, c model.Caller, after *model.PostCursor, size int32) (model.PostPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.PostPage{}, err
	}
	at, id := after.Key()
	rows, err := reads.Many[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.ListFeedPosts(s.d.Now().UTC(), viewerOf(c), reads.PostPage{AfterPublished: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.PostPage{}, fmt.Errorf("social：列出動態牆：%w", err)
	}
	rows, more := page(rows, size)
	pr := make([]postRow, len(rows))
	for i, r := range rows {
		pr[i] = postRow{r.SocialPost, r.SocialPostStat}
	}
	return s.feedPage(ctx, pr, size, more)
}

// ListStore 是店家頁：該店公開的貼文。
func (s *Service) ListStore(ctx context.Context, c model.Caller, storeID int64, after *model.PostCursor, size int32) (model.PostPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.PostPage{}, err
	}
	at, id := after.Key()
	rows, err := reads.Many[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.ListStorePublicPosts(storeID, s.d.Now().UTC(), viewerOf(c), reads.PostPage{AfterPublished: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.PostPage{}, fmt.Errorf("social：列出店家的貼文：%w", err)
	}
	rows, more := page(rows, size)
	pr := make([]postRow, len(rows))
	for i, r := range rows {
		pr[i] = postRow{r.SocialPost, r.SocialPostStat}
	}
	return s.feedPage(ctx, pr, size, more)
}

// ListByHashtag 列出某個標籤的公開貼文。標籤正規化後是空的回空頁（不是錯誤）；字元集無效回 InvalidInput。
func (s *Service) ListByHashtag(ctx context.Context, c model.Caller, tag string, after *model.PostCursor, size int32) (model.PostPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.PostPage{}, err
	}
	name, _, ok := hashtag.Normalize(tag)
	if !ok {
		return model.PostPage{}, nil
	}
	if err := hashtag.ValidName(uitext.LocaleFrom(ctx), name); err != nil {
		return model.PostPage{}, err
	}
	at, id := after.Key()
	rows, err := reads.Many[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.ListHashtagPosts(name, s.d.Now().UTC(), viewerOf(c), reads.PostPage{AfterPublished: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.PostPage{}, fmt.Errorf("social：列出標籤的貼文：%w", err)
	}
	rows, more := page(rows, size)
	pr := make([]postRow, len(rows))
	for i, r := range rows {
		pr[i] = postRow{r.SocialPost, r.SocialPostStat}
	}
	return s.feedPage(ctx, pr, size, more)
}

// ListOwn 是店主自己的貼文（含草稿與下架）。status 為空字串不篩選，否則要是 PostStatus 的值。
func (s *Service) ListOwn(ctx context.Context, storeID int64, status model.PostStatus, after *model.OwnPostCursor, size int32) (model.OwnPostPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.OwnPostPage{}, err
	}
	if status != "" && !validStatus(status) {
		return model.OwnPostPage{}, errInvalidStatus(ctx)
	}
	at, id := after.Key()
	p := store.ListOwnPostsParams{StoreID: storeID, Status: string(status), AfterCreated: at, AfterID: id, MaxRows: size + 1}
	rows, err := s.q(ctx).ListOwnPosts(ctx, p)
	if err != nil {
		return model.OwnPostPage{}, fmt.Errorf("social：列出自己的貼文：%w", err)
	}
	rows, more := page(rows, size)
	pr := make([]postRow, len(rows))
	for i, r := range rows {
		pr[i] = postRow{r.SocialPost, r.SocialPostStat}
	}
	posts, err := s.withTags(ctx, pr)
	if err != nil {
		return model.OwnPostPage{}, err
	}
	out := model.OwnPostPage{Posts: posts}
	if more {
		last := rows[len(rows)-1].SocialPost
		out.Next = &model.OwnPostCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}

// Top 是「本週主題」：最近發布的 30 篇公開貼文中瀏覽數最高的 n 篇（n 最多 30；同分以發布時間、ID 排），不分頁。
func (s *Service) Top(ctx context.Context, c model.Caller, n int32) ([]model.Post, error) {
	if n < 0 {
		return nil, errNegative(ctx)
	}
	n = min(n, model.MaxTopPosts)
	rows, err := reads.Many[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.TopPosts(s.d.Now().UTC(), viewerOf(c), model.MaxTopPosts, int64(n)))
	if err != nil {
		return nil, fmt.Errorf("social：讀取熱門貼文：%w", err)
	}
	pr := make([]postRow, len(rows))
	for i, r := range rows {
		pr[i] = postRow{r.SocialPost, r.SocialPostStat}
	}
	return s.withTags(ctx, pr)
}

// SuggestHashtags 是標籤的建議（全域彙總，計數不依觀看者與封鎖）：公開貼文上名稱符合的標籤與貼文數（limit 預設 10、上限 20）。contains 為真是「包含」搜尋，否則是前綴。
func (s *Service) SuggestHashtags(ctx context.Context, q string, contains bool, limit int32) ([]model.HashtagCount, error) {
	pattern, ok, err := hashtag.SearchPattern(uitext.LocaleFrom(ctx), q, contains)
	if err != nil || !ok {
		return nil, err
	}
	limit, err = pagination.PageSize(limit, 10, 20)
	if err != nil {
		return nil, err
	}
	rows, err := reads.Many[reads.HashtagCountRow](ctx, s.d.DB.Conn(ctx), reads.SuggestHashtags(pattern, s.d.Now().UTC(), int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("social：標籤建議：%w", err)
	}
	out := make([]model.HashtagCount, len(rows))
	for i, r := range rows {
		out[i] = model.HashtagCount{Name: r.Name, Posts: r.PostCount}
	}
	return out, nil
}

// TrendingHashtags 是熱門標籤（全域彙總，計數不依觀看者與封鎖）：發布時間在 [from, to) 內的公開貼文上各標籤的貼文數（limit 預設 20、上限 50）。窗口最長
// model.MaxTrendingWindow（30 天），超過是 InvalidInput。
func (s *Service) TrendingHashtags(ctx context.Context, from, to time.Time, limit int32) ([]model.HashtagCount, error) {
	if !from.Before(to) {
		return nil, errBadWindow(ctx)
	}
	if to.Sub(from) > model.MaxTrendingWindow {
		return nil, errWindowTooLong(ctx)
	}
	limit, err := pagination.PageSize(limit, 20, 50)
	if err != nil {
		return nil, err
	}
	rows, err := reads.Many[reads.HashtagCountRow](ctx, s.d.DB.Conn(ctx), reads.TrendingHashtags(from, to, s.d.Now().UTC(), int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("social：熱門標籤：%w", err)
	}
	out := make([]model.HashtagCount, len(rows))
	for i, r := range rows {
		out[i] = model.HashtagCount{Name: r.Name, Posts: r.PostCount}
	}
	return out, nil
}
