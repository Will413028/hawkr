package reads

import (
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/jet/table"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// PostRow 是貼文加上它的計數（posts 與 post_stats 的全部欄位）。
type PostRow struct {
	store.SocialPost
	store.SocialPostStat
}

// PostPage 是列表的游標與筆數：(published_at, id) 的列比較，第一頁由呼叫端給方向的極值游標（model.DescStart）。
type PostPage struct {
	AfterPublished time.Time
	AfterID        uuid.UUID
	Limit          int64
}

// postsWithStats 是貼文入口 join 它的 stats，投影 PostRow 的全部欄位。
func postsWithStats(src visibility.Posts) SelectStatement {
	s := table.PostStats.AS("s")
	cols := project(plain(src.T.AllColumns), plain(s.AllColumns))
	return SELECT(cols[0], cols[1:]...).FROM(src.T.INNER_JOIN(s, s.PostID.EQ(src.T.ID)))
}

func afterPost(src visibility.Posts, pg PostPage) BoolExpression {
	return ROW(src.T.PublishedAt, src.T.ID).LT(ROW(TimestampzT(pg.AfterPublished), UUID(pg.AfterID)))
}

func newestFirst(src visibility.Posts) []OrderByClause {
	return []OrderByClause{src.T.PublishedAt.DESC(), src.T.ID.DESC()}
}

// GetPublicPost 是觀看者可讀的一篇貼文（已發布、沒有刪除、店家可曝光、觀看者沒有封鎖這家店）與它的計數。
func GetPublicPost(id uuid.UUID, now time.Time, v visibility.Viewer) SelectStatement {
	src := visibility.ReadablePosts("p", now, v)
	return postsWithStats(src).WHERE(src.Where(src.T.ID.EQ(UUID(id))))
}

// BoolRow 是只回一個旗標的 query 的列。
type BoolRow struct {
	Value bool `db:"value"`
}

// PostIsPublic 回貼文對觀看者是不是公開可讀（留言、讚、瀏覽與分享只對讀得到的貼文）。
func PostIsPublic(id uuid.UUID, now time.Time, v visibility.Viewer) SelectStatement {
	src := visibility.ReadablePosts("p", now, v)
	return SELECT(EXISTS(SELECT(src.T.ID).FROM(src.T).WHERE(src.Where(src.T.ID.EQ(UUID(id))))).AS("value"))
}

// ListFeedPosts 是動態牆：最新發布的在前。
func ListFeedPosts(now time.Time, v visibility.Viewer, pg PostPage) SelectStatement {
	src := visibility.ReadablePosts("p", now, v)
	return postsWithStats(src).WHERE(src.Where(afterPost(src, pg))).ORDER_BY(newestFirst(src)...).LIMIT(pg.Limit)
}

// ListStorePublicPosts 是店家頁：該店已發布的貼文，分頁同 ListFeedPosts。
func ListStorePublicPosts(storeID int64, now time.Time, v visibility.Viewer, pg PostPage) SelectStatement {
	src := visibility.ReadablePosts("p", now, v)
	return postsWithStats(src).WHERE(src.Where(src.T.StoreID.EQ(Int64(storeID)), afterPost(src, pg))).ORDER_BY(newestFirst(src)...).LIMIT(pg.Limit)
}

// ListHashtagPosts 是某個標籤（正規化後的名稱）的公開貼文，分頁同 ListFeedPosts。排序鍵用標籤列上貼文 published_at 的複本
// （post_hashtags_feed：(name, published_at DESC, post_id DESC)），游標條件才是 Index Cond；「公開可讀」仍由 visibility 的入口組成。
// 複本在貼文第一次發布時由 trigger 寫入、之後不變，所以與 posts.published_at 相同；草稿的標籤列是 NULL，不在索引裡。
func ListHashtagPosts(name string, now time.Time, v visibility.Viewer, pg PostPage) SelectStatement {
	src := visibility.ReadablePosts("p", now, v)
	h := table.PostHashtags.AS("h")
	s := table.PostStats.AS("s")
	cols := project(plain(src.T.AllColumns), plain(s.AllColumns))
	after := ROW(h.PublishedAt, h.PostID).LT(ROW(TimestampzT(pg.AfterPublished), UUID(pg.AfterID)))
	return SELECT(cols[0], cols[1:]...).
		FROM(h.INNER_JOIN(src.T, src.T.ID.EQ(h.PostID)).INNER_JOIN(s, s.PostID.EQ(src.T.ID))).
		WHERE(src.Where(h.Name.EQ(String(name)), h.PublishedAt.IS_NOT_NULL(), after)).
		ORDER_BY(h.PublishedAt.DESC(), h.PostID.DESC()).
		LIMIT(pg.Limit)
}

// TopPosts 是最近發布的 sampleSize 篇公開貼文中瀏覽數最高的 limit 篇（同分以發布時間、ID 排）。
// 內層取樣、外層再排；外層仍帶公開條件（最多 sampleSize 列，多一次的探測可以忽略）。
func TopPosts(now time.Time, v visibility.Viewer, sampleSize, limit int64) SelectStatement {
	inner := visibility.ReadablePosts("p0", now, v)
	latest := SELECT(inner.T.ID.AS("id")).FROM(inner.T).WHERE(inner.Where()).
		ORDER_BY(inner.T.PublishedAt.DESC(), inner.T.ID.DESC()).LIMIT(sampleSize).AsTable("latest")
	src := visibility.ReadablePosts("p", now, v)
	s := table.PostStats.AS("s")
	cols := project(plain(src.T.AllColumns), plain(s.AllColumns))
	return SELECT(cols[0], cols[1:]...).
		FROM(latest.INNER_JOIN(src.T, src.T.ID.EQ(StringColumn("id").From(latest))).INNER_JOIN(s, s.PostID.EQ(src.T.ID))).
		WHERE(src.Where()).
		ORDER_BY(s.ViewCount.DESC(), src.T.PublishedAt.DESC(), src.T.ID.DESC()).
		LIMIT(limit)
}

// HashtagCountRow 是標籤與它的公開貼文數。
type HashtagCountRow struct {
	Name      string `db:"name"`
	PostCount int64  `db:"post_count"`
}

// hashtagCounts 是全域彙總：計數不依觀看者（封鎖不影響計數），所以以匿名建入口。
func hashtagCounts(now time.Time, limit int64, filter func(h *table.PostHashtagsTable, p *table.PostsTable) BoolExpression) SelectStatement {
	src := visibility.ReadablePosts("p", now, visibility.Anonymous)
	h := table.PostHashtags.AS("h")
	cnt := COUNT(STAR)
	return SELECT(h.Name.AS("name"), cnt.AS("post_count")).
		FROM(h.INNER_JOIN(src.T, src.T.ID.EQ(h.PostID))).
		WHERE(src.Where(filter(h, src.T))).
		GROUP_BY(h.Name).
		ORDER_BY(cnt.DESC(), h.Name.ASC()).
		LIMIT(limit)
}

// SuggestHashtags 是標籤的建議：公開貼文上名稱符合 pattern 的標籤與它的公開貼文數。pattern 由呼叫方組成並先驗證過字元集（沒有 LIKE 的萬用字元）。
func SuggestHashtags(pattern string, now time.Time, limit int64) SelectStatement {
	return hashtagCounts(now, limit, func(h *table.PostHashtagsTable, _ *table.PostsTable) BoolExpression {
		return h.Name.LIKE(String(pattern))
	})
}

// TrendingHashtags 是熱門標籤：發布時間在 [start, end) 內的公開貼文上各標籤的貼文數。
func TrendingHashtags(start, end, now time.Time, limit int64) SelectStatement {
	return hashtagCounts(now, limit, func(_ *table.PostHashtagsTable, p *table.PostsTable) BoolExpression {
		return p.PublishedAt.GT_EQ(TimestampzT(start)).AND(p.PublishedAt.LT(TimestampzT(end)))
	})
}
