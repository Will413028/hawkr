//go:build ignore

// A7-1a §9.3 的量測：go-jet 建的 query 直接呼叫 internal/reads 的建構函式（不手抄），sqlc 的 query 讀 social/db/queries 的 .sql 原文，
// 逐支執行並量 p95，另外輸出 EXPLAIN (ANALYZE, BUFFERS)。檔案放在 social 底下才能 import 模組的 internal 套件。
//
//	cd backend && GOWORK=off go run ./internal/modules/social/perf/measure.go \
//	    -dsn 'postgres://postgres:perf@127.0.0.1:55477/perf?sslmode=disable' -reps 200 -out /path/to/out
//
// 每個情境用 prepared statement、plan_cache_mode = force_generic_plan（量的是 generic plan，與連線池裡反覆執行的實際狀況一致）；
// 先暖機 10 次，再計時 reps 次，回報 p50、p95、max。EXPLAIN 同樣在 generic plan 下做：PREPARE 之後 EXPLAIN (ANALYZE, BUFFERS) EXECUTE。
// 時間是 client 量到的往返時間（單一連線、同機），不含網路；page_size 一律 50（max_rows）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

type namedQuery struct {
	name   string
	sql    string
	params []string
}

var (
	nameLine = regexp.MustCompile(`(?m)^-- name: (\w+) :\w+\s*$`)
	argRe    = regexp.MustCompile(`sqlc\.n?arg\((\w+)\)`)
	embedRe  = regexp.MustCompile(`sqlc\.embed\((\w+)\)`)
)

func loadQueries(dir string) (map[string]namedQuery, error) {
	out := map[string]namedQuery{}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		text := string(raw)
		locs := nameLine.FindAllStringSubmatchIndex(text, -1)
		for i, loc := range locs {
			end := len(text)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			body := text[loc[1]:end]
			var lines []string
			for _, l := range strings.Split(body, "\n") {
				if strings.HasPrefix(strings.TrimSpace(l), "--") {
					continue
				}
				lines = append(lines, l)
			}
			body = strings.TrimSpace(strings.Join(lines, "\n"))
			body = strings.TrimSuffix(body, ";")
			var params []string
			seen := map[string]int{}
			body = argRe.ReplaceAllStringFunc(body, func(m string) string {
				n := argRe.FindStringSubmatch(m)[1]
				if _, ok := seen[n]; !ok {
					params = append(params, n)
					seen[n] = len(params)
				}
				return fmt.Sprintf("$%d", seen[n])
			})
			body = embedRe.ReplaceAllString(body, "$1.*")
			out[text[loc[2]:loc[3]]] = namedQuery{name: text[loc[2]:loc[3]], sql: body, params: params}
		}
	}
	return out, nil
}

type scenario struct {
	label string
	query string
	// build 回傳語句與參數：go-jet 的 query 是 internal/reads 的建構函式，sqlc 的 query 是 .sql 原文加具名參數。
	build func() (string, []any)
	// anon 是觀看者變體的匿名對照（同一個 query、同一份參數，只是觀看者換成未登入）；有值時 build 與 anon 在同一輪交錯計時，
	// 判斷看兩者的比值，不看絕對值（機器的 load 會讓同一個語句在兩次量測之間差數倍）。
	anon     func() (string, []any)
	keyset   bool
	rollback bool
	// exposureOnce 把每一列各探測一次的 `EXISTS (SELECT 1 FROM seller.pub_exposable_stores(at) e WHERE e.store_id = p.store_id)` 改成先把
	// 可曝光的店家算成一份（MATERIALIZED CTE）再比對：量它是為了看「每個貼文各探測一次」的成本占多少，query 檔本身不變。
	exposureOnce bool
}

var exposureExists = regexp.MustCompile(`EXISTS \(SELECT 1 FROM seller\.pub_exposable_stores\((\$\d+)::timestamp with time zone\) AS pub WHERE pub\.store_id = (\w+)\.store_id\)`)

func exposureOnceForm(sql string) string {
	m := exposureExists.FindStringSubmatch(sql)
	if m == nil {
		return sql
	}
	sql = exposureExists.ReplaceAllString(sql, "EXISTS (SELECT 1 FROM ex WHERE ex.store_id = $2.store_id)")
	return "WITH ex AS MATERIALIZED (SELECT store_id FROM seller.pub_exposable_stores(" + m[1] + "::timestamptz)) " + sql
}

// keyset 的第一頁用方向的極值游標（與 model.DescStart／AscStart 相同的值）：query 沒有布林開關，所有頁走同一個計畫。
var ascQueries = map[string]bool{"ListTopLevelComments": true, "ListOwnerTopLevelComments": true, "ListReplies": true, "ListOwnerReplies": true, "ListTopLevelForumEntries": true, "ListForumReplies": true}

func sentinelArg(query, param string) (any, bool) {
	asc := ascQueries[query]
	switch param {
	case "after_published", "after_created", "after_hidden":
		if asc {
			return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), true
		}
		return time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC), true
	case "after_id":
		if asc {
			return "00000000-0000-0000-0000-000000000000", true
		}
		return "ffffffff-ffff-ffff-ffff-ffffffffffff", true
	}
	return nil, false
}

// setDriven 是計畫由「集合」驅動、排序鍵不在驅動的索引上的 query：游標條件只能是 join 上的 Filter，成本由集合大小決定，與游標深度無關
// （ListHiddenContentOf：該買家自己的留言與討論區留言，作者自己的有界集合）。ListHashtagPosts 的排序鍵複製在 post_hashtags 上，要求 Index Cond。
var setDriven = map[string]bool{"ListHiddenContentOf": true}

// rowCmpIndexCond 判斷計畫裡有沒有「列比較」的 Index Cond（keyset 的游標條件）。
var rowCmpIndexCond = regexp.MustCompile(`Index Cond: .*ROW\([\w.]+, [\w.]+\) [<>] ROW\(\$\d+, \$\d+\)`)

func main() {
	dsn := flag.String("dsn", "", "資料庫連線字串")
	reps := flag.Int("reps", 200, "每個情境計時的次數")
	outDir := flag.String("out", "", "EXPLAIN 輸出的目錄（空字串不輸出）")
	queriesDir := flag.String("queries", "internal/modules/social/db/queries", "social 的 queries 目錄（相對於 backend/）")
	only := flag.String("only", "", "只跑標籤符合這個正規表示式的情境（空字串是全部）")
	ratio := flag.Float64("ratio", 3, "觀看者變體的 p95 對匿名對照的 p95 的倍數上限（超過且觀看者 p95 高於 -floor 毫秒才算退步）")
	floor := flag.Float64("floor", 10, "比值判斷的雜訊下限（毫秒）：觀看者 p95 低於它的情境不比倍數")
	viewer := flag.Int64("viewer", 0, "有觀看者的變體用的買家 ID（要先跑 step-1b-perf/seed-blocks.sql；0 是不跑變體）")
	flag.Parse()
	if err := run(context.Background(), *dsn, *reps, *outDir, *queriesDir, *only, *viewer, *ratio, *floor); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func scalar[T any](ctx context.Context, c *pgx.Conn, sql string, args ...any) T {
	var v T
	if err := c.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		fmt.Fprintf(os.Stderr, "取參數失敗：%v\n%s\n", err, sql)
		os.Exit(1)
	}
	return v
}

func run(ctx context.Context, dsn string, reps int, outDir, queriesDir, only string, viewerID int64, ratioLimit, floorMs float64) error {
	queries, err := loadQueries(queriesDir)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SET plan_cache_mode = force_generic_plan"); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)

	// 依資料挑工作量的參數：熱門與冷門的店家、標籤、貼文，以及 keyset 的游標位置。挑參數的 SQL 是量測用的手寫 SQL（不是規則的定義）。
	published := `p.deleted_at IS NULL AND p.status = 'PUBLISHED'`
	exposable := `EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamptz) e WHERE e.store_id = p.store_id)`
	hotStore := scalar[int64](ctx, conn, `SELECT p.store_id FROM social.posts p WHERE `+published+` AND `+exposable+` GROUP BY p.store_id ORDER BY count(*) DESC LIMIT 1`, now)
	coldStore := scalar[int64](ctx, conn, `SELECT p.store_id FROM social.posts p WHERE `+published+` AND `+exposable+` GROUP BY p.store_id ORDER BY abs(count(*) - 3), p.store_id LIMIT 1`, now)
	hotTag := scalar[string](ctx, conn, `SELECT h.name FROM social.post_hashtags h JOIN social.posts p ON p.id = h.post_id WHERE `+published+` GROUP BY h.name ORDER BY count(*) DESC LIMIT 1`)
	coldTag := scalar[string](ctx, conn, `SELECT h.name FROM social.post_hashtags h JOIN social.posts p ON p.id = h.post_id WHERE `+published+` GROUP BY h.name ORDER BY count(*), h.name LIMIT 1`)
	cjkPrefix := scalar[string](ctx, conn, `SELECT left(h.name, 1) FROM social.post_hashtags h WHERE h.name >= E'一' GROUP BY 1 ORDER BY count(*) DESC LIMIT 1`)
	latinPrefix := scalar[string](ctx, conn, `SELECT left(h.name, 1) FROM social.post_hashtags h WHERE h.name < E'\u4e00' GROUP BY 1 ORDER BY count(*) DESC LIMIT 1`)
	cjk2 := scalar[string](ctx, conn, `SELECT substr(h.name, 2, 2) FROM social.post_hashtags h WHERE h.name >= E'\u4e00' AND char_length(h.name) >= 3 GROUP BY 1 ORDER BY count(*) DESC LIMIT 1`)
	latinContains := scalar[string](ctx, conn, `SELECT substr(h.name, 2, 2) FROM social.post_hashtags h WHERE h.name < E'\u4e00' GROUP BY 1 ORDER BY count(*) DESC LIMIT 1`)
	hotPost := scalar[string](ctx, conn, `SELECT c.post_id::text FROM social.comments c JOIN social.posts p ON p.id = c.post_id WHERE `+published+` AND `+exposable+` GROUP BY c.post_id ORDER BY count(*) DESC LIMIT 1`, now)
	coldPost := scalar[string](ctx, conn, `SELECT c.post_id::text FROM social.comments c JOIN social.posts p ON p.id = c.post_id WHERE `+published+` AND `+exposable+` GROUP BY c.post_id ORDER BY abs(count(*) - 5), c.post_id LIMIT 1`, now)
	// 批次重算的範圍：500 列的 stats 主鍵範圍，一批含最熱門的貼文、一批在中段。
	hotBatchAfter := scalar[string](ctx, conn, `SELECT COALESCE((SELECT post_id::text FROM social.post_stats WHERE post_id < $1::uuid ORDER BY post_id DESC OFFSET 249 LIMIT 1), '00000000-0000-0000-0000-000000000000')`, hotPost)
	hotBatchUpper := scalar[string](ctx, conn, `SELECT COALESCE((SELECT post_id::text FROM social.post_stats WHERE post_id >= $1::uuid ORDER BY post_id OFFSET 250 LIMIT 1), 'ffffffff-ffff-ffff-ffff-ffffffffffff')`, hotPost)
	coldBatchAfter := scalar[string](ctx, conn, `SELECT post_id::text FROM social.post_stats ORDER BY post_id OFFSET 40000 LIMIT 1`)
	coldBatchUpper := scalar[string](ctx, conn, `SELECT post_id::text FROM social.post_stats ORDER BY post_id OFFSET 40500 LIMIT 1`)
	hotParent := scalar[string](ctx, conn, `SELECT parent_id::text FROM social.comments WHERE post_id = $1::uuid AND parent_id IS NOT NULL GROUP BY parent_id ORDER BY count(*) DESC LIMIT 1`, hotPost)
	hotReply := scalar[string](ctx, conn, `SELECT id::text FROM social.comments WHERE parent_id = $1::uuid ORDER BY created_at, id LIMIT 1`, hotParent)
	hotForumStore := scalar[int64](ctx, conn, `SELECT store_id FROM social.forum_entries GROUP BY store_id ORDER BY count(*) DESC LIMIT 1`)
	hotForumParent := scalar[string](ctx, conn, `SELECT parent_id::text FROM social.forum_entries WHERE store_id = $1 AND parent_id IS NOT NULL GROUP BY parent_id ORDER BY count(*) DESC LIMIT 1`, hotForumStore)
	hotForumReply := scalar[string](ctx, conn, `SELECT id::text FROM social.forum_entries WHERE parent_id = $1::uuid ORDER BY created_at, id LIMIT 1`, hotForumParent)
	hotPostStore := scalar[int64](ctx, conn, `SELECT store_id FROM social.posts WHERE id = $1::uuid`, hotPost)

	type cursor struct {
		ts time.Time
		id string
	}
	sqlCursor := func(q string, args ...any) cursor {
		var c cursor
		if err := conn.QueryRow(ctx, q, args...).Scan(&c.ts, &c.id); err != nil {
			fmt.Fprintln(os.Stderr, "cursor:", q, err)
			os.Exit(1)
		}
		return c
	}
	feedCursor := func(frac float64) cursor {
		return sqlCursor(`SELECT p.published_at, p.id::text FROM social.posts p WHERE `+published+` AND `+exposable+`
			ORDER BY p.published_at DESC, p.id DESC OFFSET (SELECT (count(*) * $2::float8)::int FROM social.posts p WHERE `+published+`) LIMIT 1`, now, frac)
	}
	feedMid, feedDeep := feedCursor(0.5), feedCursor(0.9)
	hotStoreMid := sqlCursor(`SELECT p.published_at, p.id::text FROM social.posts p WHERE `+published+` AND p.store_id = $1
		ORDER BY p.published_at DESC, p.id DESC OFFSET 200 LIMIT 1`, hotStore)
	hotTagMid := sqlCursor(`SELECT p.published_at, p.id::text FROM social.posts p WHERE `+published+` AND EXISTS (SELECT 1 FROM social.post_hashtags h WHERE h.post_id = p.id AND h.name = $1)
		ORDER BY p.published_at DESC, p.id DESC OFFSET 100 LIMIT 1`, hotTag)
	commentMid := sqlCursor(`SELECT created_at, id::text FROM social.comments WHERE post_id = $1::uuid AND parent_id IS NULL ORDER BY created_at, id OFFSET 200 LIMIT 1`, hotPost)
	ownMid := sqlCursor(`SELECT created_at, id::text FROM social.posts WHERE store_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC, id DESC OFFSET 200 LIMIT 1`, hotStore)
	replyMid := sqlCursor(`SELECT created_at, id::text FROM social.comments WHERE parent_id = $1::uuid ORDER BY created_at, id OFFSET (SELECT count(*) / 2 FROM social.comments WHERE parent_id = $1::uuid) LIMIT 1`, hotParent)
	forumMid := sqlCursor(`SELECT created_at, id::text FROM social.forum_entries WHERE store_id = $1 AND parent_id IS NULL ORDER BY created_at, id OFFSET (SELECT count(*) / 2 FROM social.forum_entries WHERE store_id = $1 AND parent_id IS NULL) LIMIT 1`, hotForumStore)
	forumReplyMid := sqlCursor(`SELECT created_at, id::text FROM social.forum_entries WHERE parent_id = $1::uuid ORDER BY created_at, id OFFSET (SELECT count(*) / 2 FROM social.forum_entries WHERE parent_id = $1::uuid) LIMIT 1`, hotForumParent)
	hotHider := scalar[int64](ctx, conn, `SELECT c.actor_id FROM social.hides h JOIN social.comments c ON c.id = h.comment_id WHERE h.restored_at IS NULL AND c.actor_kind = 'BUYER' GROUP BY c.actor_id ORDER BY count(*) DESC LIMIT 1`)
	hotSanctioned := scalar[int64](ctx, conn, `SELECT buyer_id FROM social.sanctions GROUP BY buyer_id ORDER BY count(*) DESC, buyer_id LIMIT 1`)
	sanctionMid := sqlCursor(`SELECT created_at, id::text FROM social.sanctions WHERE buyer_id = $1 ORDER BY created_at DESC, id DESC OFFSET 4 LIMIT 1`, hotSanctioned)
	hiddenMid := sqlCursor(`SELECT h.hidden_at, h.id::text FROM social.hides h JOIN social.comments c ON c.id = h.comment_id WHERE h.restored_at IS NULL AND c.actor_kind = 'BUYER' AND c.actor_id = $1 ORDER BY h.hidden_at DESC, h.id DESC OFFSET 1 LIMIT 1`, hotHider)

	// go-jet 的 query：呼叫 internal/reads 的建構函式；第一頁用方向的極值游標（model.DescStart／AscStart），所有頁走同一個 prepared statement。
	const pageSize = 50
	parseID := func(s string) uuid.UUID {
		id, err := uuid.Parse(s)
		if err != nil {
			fmt.Fprintln(os.Stderr, "uuid:", s, err)
			os.Exit(1)
		}
		return id
	}
	postPage := func(c *cursor) reads.PostPage {
		if c == nil {
			at, id := model.DescStart()
			return reads.PostPage{AfterPublished: at, AfterID: id, Limit: pageSize}
		}
		return reads.PostPage{AfterPublished: c.ts, AfterID: parseID(c.id), Limit: pageSize}
	}
	cmtPage := func(c *cursor) reads.CommentPage {
		if c == nil {
			at, id := model.AscStart()
			return reads.CommentPage{AfterCreated: at, AfterID: id, Limit: pageSize}
		}
		return reads.CommentPage{AfterCreated: c.ts, AfterID: parseID(c.id), Limit: pageSize}
	}
	v := visibility.Anonymous
	vv := visibility.Viewer{BuyerID: viewerID}
	jet := func(st reads.Stmt) func() (string, []any) { return st.Sql }
	// sqlc 的 query：.sql 原文加具名參數；游標參數缺時用極值（與 model 的 Key 相同）。
	sqlc := func(name string, args map[string]any) func() (string, []any) {
		return func() (string, []any) {
			q, ok := queries[name]
			if !ok {
				fmt.Fprintln(os.Stderr, "找不到 query", name)
				os.Exit(1)
			}
			out := make([]any, len(q.params))
			for i, p := range q.params {
				val, ok := args[p]
				if !ok {
					val, ok = sentinelArg(name, p)
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "%s 缺參數 %s\n", name, p)
					os.Exit(1)
				}
				out[i] = val
			}
			return q.sql, out
		}
	}
	zeroID := "00000000-0000-0000-0000-000000000000"
	sqlcArgs := func(extra map[string]any) map[string]any {
		m := map[string]any{"now": now, "max_rows": int32(50)}
		for k, val := range extra {
			m[k] = val
		}
		return m
	}
	sqlcCur := func(c cursor, key string, extra map[string]any) map[string]any {
		m := sqlcArgs(extra)
		m[key], m["after_id"] = c.ts, c.id
		return m
	}
	pf, pd := feedMid, feedDeep
	hsm, htm, cm, rm, fm, frm := hotStoreMid, hotTagMid, commentMid, replyMid, forumMid, forumReplyMid
	trend := func(days int) func() (string, []any) {
		return jet(reads.TrendingHashtags(now.AddDate(0, 0, -days), now, now, pageSize))
	}

	scenarios := []scenario{
		{label: "feed 第一頁", query: "ListFeedPosts", keyset: true, build: jet(reads.ListFeedPosts(now, v, postPage(nil)))},
		{label: "feed keyset 游標在 50%", query: "ListFeedPosts", keyset: true, build: jet(reads.ListFeedPosts(now, v, postPage(&pf)))},
		{label: "feed keyset 游標在 90%", query: "ListFeedPosts", keyset: true, build: jet(reads.ListFeedPosts(now, v, postPage(&pd)))},
		{label: "店家頁 熱門店 第一頁", query: "ListStorePublicPosts", keyset: true, build: jet(reads.ListStorePublicPosts(hotStore, now, v, postPage(nil)))},
		{label: "店家頁 熱門店 游標第 200 篇", query: "ListStorePublicPosts", keyset: true, build: jet(reads.ListStorePublicPosts(hotStore, now, v, postPage(&hsm)))},
		{label: "店家頁 冷門店 第一頁", query: "ListStorePublicPosts", keyset: true, build: jet(reads.ListStorePublicPosts(coldStore, now, v, postPage(nil)))},
		{label: "hashtag 貼文 熱門標籤 第一頁", query: "ListHashtagPosts", keyset: true, build: jet(reads.ListHashtagPosts(hotTag, now, v, postPage(nil)))},
		{label: "hashtag 貼文 熱門標籤 游標第 100 篇", query: "ListHashtagPosts", keyset: true, build: jet(reads.ListHashtagPosts(hotTag, now, v, postPage(&htm)))},
		{label: "hashtag 貼文 冷門標籤", query: "ListHashtagPosts", keyset: true, build: jet(reads.ListHashtagPosts(coldTag, now, v, postPage(nil)))},
		{label: "hashtag 建議 中文前綴 1 字", query: "SuggestHashtags", build: jet(reads.SuggestHashtags(cjkPrefix+"%", now, pageSize))},
		{label: "hashtag 建議 英數前綴 1 字母", query: "SuggestHashtags", build: jet(reads.SuggestHashtags(latinPrefix+"%", now, pageSize))},
		{label: "hashtag 建議 包含 中文 1 字", query: "SuggestHashtags", build: jet(reads.SuggestHashtags("%"+cjkPrefix+"%", now, pageSize))},
		{label: "hashtag 建議 包含 中文 2 字", query: "SuggestHashtags", build: jet(reads.SuggestHashtags("%"+cjk2+"%", now, pageSize))},
		{label: "hashtag 建議 包含 英數 2 字", query: "SuggestHashtags", build: jet(reads.SuggestHashtags("%"+latinContains+"%", now, pageSize))},
		{label: "hashtag 熱門 近 30 天", query: "TrendingHashtags", build: trend(30)},
		{label: "計數 單篇貼文（含 stats）熱門", query: "GetPublicPost", build: jet(reads.GetPublicPost(parseID(hotPost), now, v))},
		{label: "計數 熱門貼文所在的一批重算（RecountPostStatsBatch，500 列）", query: "RecountPostStatsBatch", build: jet(reads.RecountPostStatsBatch(parseID(hotBatchAfter), parseID(hotBatchUpper))), rollback: true},
		{label: "計數 一般的一批重算（RecountPostStatsBatch，500 列）", query: "RecountPostStatsBatch", build: jet(reads.RecountPostStatsBatch(parseID(coldBatchAfter), parseID(coldBatchUpper))), rollback: true},
		{label: "計數 每日對帳的分頁（PostStatsPage）", query: "PostStatsPage", build: sqlc("PostStatsPage", sqlcArgs(map[string]any{"after_id": zeroID}))},
		{label: "店主的貼文 熱門店第一頁", query: "ListOwnPosts", keyset: true, build: sqlc("ListOwnPosts", sqlcArgs(map[string]any{"store_id": hotStore, "status": ""}))},
		{label: "留言 熱門貼文頂層 第一頁（含佔位 EXISTS）", query: "ListTopLevelComments", keyset: true, build: jet(reads.ListTopLevelComments(parseID(hotPost), now, v, cmtPage(nil)))},
		{label: "留言 熱門貼文頂層 游標第 200 則", query: "ListTopLevelComments", keyset: true, build: jet(reads.ListTopLevelComments(parseID(hotPost), now, v, cmtPage(&cm)))},
		{label: "留言 一般貼文頂層 第一頁", query: "ListTopLevelComments", keyset: true, build: jet(reads.ListTopLevelComments(parseID(coldPost), now, v, cmtPage(nil)))},
		{label: "留言 店主讀熱門貼文頂層", query: "ListOwnerTopLevelComments", keyset: true, build: jet(reads.ListOwnerTopLevelComments(parseID(hotPost), hotPostStore, cmtPage(nil)))},
		{label: "留言 回覆列表", query: "ListReplies", keyset: true, build: jet(reads.ListReplies(parseID(hotParent), now, v, cmtPage(nil)))},
		{label: "留言 回覆列表 游標在中段", query: "ListReplies", keyset: true, build: jet(reads.ListReplies(parseID(hotParent), now, v, cmtPage(&rm)))},
		{label: "留言 店主讀回覆 第一頁", query: "ListOwnerReplies", keyset: true, build: jet(reads.ListOwnerReplies(parseID(hotParent), hotPostStore, cmtPage(nil)))},
		{label: "留言 店主讀回覆 游標在中段", query: "ListOwnerReplies", keyset: true, build: jet(reads.ListOwnerReplies(parseID(hotParent), hotPostStore, cmtPage(&rm)))},
		{label: "店主的貼文 熱門店游標第 200 篇", query: "ListOwnPosts", keyset: true, build: sqlc("ListOwnPosts", sqlcCur(ownMid, "after_created", map[string]any{"store_id": hotStore, "status": ""}))},
		{label: "討論區 熱門店頂層 游標在中段", query: "ListTopLevelForumEntries", keyset: true, build: jet(reads.ListTopLevelForumEntries(hotForumStore, now, v, cmtPage(&fm)))},
		{label: "討論區 回覆列表 游標在中段", query: "ListForumReplies", keyset: true, build: jet(reads.ListForumReplies(parseID(hotForumParent), now, v, cmtPage(&frm)))},
		{label: "處分紀錄 第一頁", query: "ListSanctions", keyset: true, build: sqlc("ListSanctions", sqlcArgs(map[string]any{"buyer_id": hotSanctioned}))},
		{label: "處分紀錄 游標第 5 筆", query: "ListSanctions", keyset: true, build: sqlc("ListSanctions", sqlcCur(sanctionMid, "after_created", map[string]any{"buyer_id": hotSanctioned}))},
		{label: "被隱藏的內容 第一頁", query: "ListHiddenContentOf", keyset: true, build: sqlc("ListHiddenContentOf", sqlcArgs(map[string]any{"buyer_id": hotHider}))},
		{label: "被隱藏的內容 游標第 2 則", query: "ListHiddenContentOf", keyset: true, build: sqlc("ListHiddenContentOf", sqlcCur(hiddenMid, "after_hidden", map[string]any{"buyer_id": hotHider}))},
		{label: "討論區 熱門店頂層 第一頁（含佔位 EXISTS）", query: "ListTopLevelForumEntries", keyset: true, build: jet(reads.ListTopLevelForumEntries(hotForumStore, now, v, cmtPage(nil)))},
		{label: "討論區 回覆列表", query: "ListForumReplies", keyset: true, build: jet(reads.ListForumReplies(parseID(hotForumParent), now, v, cmtPage(nil)))},
		{label: "熱門貼文 TopPosts（最新 30 篇取瀏覽最高 5 篇）", query: "TopPosts", build: jet(reads.TopPosts(now, v, model.MaxTopPosts, 5))},
		{label: "歸屬 GetPostOwner（moderation 驗證）", query: "GetPostOwner", build: jet(reads.GetPostOwner(parseID(hotPost), now))},
		{label: "歸屬 ReadableCommentPost（留言按讚）", query: "ReadableCommentPost", build: jet(reads.ReadableCommentPost(parseID(hotParent), now, v))},
	}

	// 有觀看者的變體（A7-1b）：觀看者封鎖了數十個店與買家，也被一些買家封鎖（seed-blocks.sql）。每個變體帶匿名對照（同一個 query、同一份參數），
	// 兩者在同一輪交錯計時，判斷看比值與計畫形狀，不看絕對值。涵蓋列表、單列歸屬與寫入前置檢查用的單列查詢（含回覆的 ParentAdmitted）。
	if viewerID > 0 {
		pair := func(label, query string, keyset bool, mk func(visibility.Viewer) func() (string, []any)) scenario {
			return scenario{label: "觀看者 " + label, query: query, keyset: keyset, build: mk(vv), anon: mk(v)}
		}
		hot, cold, hotThread, hotFThread := parseID(hotPost), parseID(coldPost), parseID(hotParent), parseID(hotForumParent)
		viewerScenarios := []scenario{
			pair("feed 第一頁", "ListFeedPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListFeedPosts(now, x, postPage(nil)))
			}),
			pair("feed keyset 游標在 50%", "ListFeedPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListFeedPosts(now, x, postPage(&pf)))
			}),
			pair("feed keyset 游標在 90%", "ListFeedPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListFeedPosts(now, x, postPage(&pd)))
			}),
			pair("店家頁 熱門店 第一頁", "ListStorePublicPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListStorePublicPosts(hotStore, now, x, postPage(nil)))
			}),
			pair("店家頁 熱門店 游標第 200 篇", "ListStorePublicPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListStorePublicPosts(hotStore, now, x, postPage(&hsm)))
			}),
			pair("hashtag 貼文 熱門標籤 第一頁", "ListHashtagPosts", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListHashtagPosts(hotTag, now, x, postPage(nil)))
			}),
			pair("熱門貼文 TopPosts", "TopPosts", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.TopPosts(now, x, model.MaxTopPosts, 5))
			}),
			pair("單篇貼文（含 stats）熱門", "GetPublicPost", false, func(x visibility.Viewer) func() (string, []any) { return jet(reads.GetPublicPost(hot, now, x)) }),
			pair("寫入前置 PostIsPublic", "PostIsPublic", false, func(x visibility.Viewer) func() (string, []any) { return jet(reads.PostIsPublic(hot, now, x)) }),
			pair("留言 熱門貼文頂層 第一頁", "ListTopLevelComments", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListTopLevelComments(hot, now, x, cmtPage(nil)))
			}),
			pair("留言 熱門貼文頂層 游標第 200 則", "ListTopLevelComments", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListTopLevelComments(hot, now, x, cmtPage(&cm)))
			}),
			pair("留言 一般貼文頂層 第一頁", "ListTopLevelComments", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListTopLevelComments(cold, now, x, cmtPage(nil)))
			}),
			pair("留言 回覆列表", "ListReplies", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListReplies(hotThread, now, x, cmtPage(nil)))
			}),
			pair("留言 回覆列表 游標在中段", "ListReplies", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListReplies(hotThread, now, x, cmtPage(&rm)))
			}),
			pair("討論區 熱門店頂層 第一頁", "ListTopLevelForumEntries", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListTopLevelForumEntries(hotForumStore, now, x, cmtPage(nil)))
			}),
			pair("討論區 熱門店頂層 游標在中段", "ListTopLevelForumEntries", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListTopLevelForumEntries(hotForumStore, now, x, cmtPage(&fm)))
			}),
			pair("討論區 回覆列表", "ListForumReplies", true, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ListForumReplies(hotFThread, now, x, cmtPage(nil)))
			}),
			pair("寫入前置 ForumIsReadable", "ForumIsReadable", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ForumIsReadable(hotForumStore, now, x))
			}),
			pair("歸屬 ReadableCommentPost 頂層留言（留言按讚）", "ReadableCommentPost", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ReadableCommentPost(hotThread, now, x))
			}),
			pair("歸屬 ReadableCommentPost 回覆（含 ParentAdmitted）", "ReadableCommentPost", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ReadableCommentPost(parseID(hotReply), now, x))
			}),
			pair("寫入前置 ReplyTargetIsReadable", "ReplyTargetIsReadable", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ReplyTargetIsReadable(parseID(hotReply), hot, hotThread, x))
			}),
			pair("寫入前置 ForumReplyTargetIsReadable", "ForumReplyTargetIsReadable", false, func(x visibility.Viewer) func() (string, []any) {
				return jet(reads.ForumReplyTargetIsReadable(parseID(hotForumReply), hotForumStore, hotFThread, x))
			}),
		}
		scenarios = append(scenarios, viewerScenarios...)
	}

	var extra []scenario
	for _, sc := range scenarios {
		switch sc.label {
		case "hashtag 熱門 近 30 天", "hashtag 貼文 熱門標籤 第一頁", "hashtag 建議 中文前綴 1 字", "hashtag 建議 包含 中文 1 字":
			cp := sc
			cp.label += "（可曝光店家先算一份）"
			cp.exposureOnce = true
			extra = append(extra, cp)
		}
	}
	scenarios = append(scenarios, extra...)

	if only != "" {
		re, err := regexp.Compile(only)
		if err != nil {
			return err
		}
		var kept []scenario
		for _, sc := range scenarios {
			if re.MatchString(sc.label) {
				kept = append(kept, sc)
			}
		}
		scenarios = kept
	}

	fmt.Printf("| 情境 | p50 ms | p95 ms | max ms | 匿名對照 p50 / p95 ms | 比值 p50 / p95 | Function Scan | 計畫的 Index Cond 之外的 Seq Scan | 列比較是 Index Cond | 封鎖表的 Seq／Function Scan |\n|---|---|---|---|---|---|---|---|---|---|\n")
	var explains strings.Builder
	exceeded, noIndexCond, blocksScans, regressed := 0, 0, 0, 0
	for _, sc := range scenarios {
		sql, args := sc.build()
		if sc.exposureOnce {
			sql = exposureOnceForm(sql)
		}
		var durs, anonDurs []time.Duration
		var err error
		if sc.anon != nil {
			anonSQL, anonArgs := sc.anon()
			durs, anonDurs, err = timePair(ctx, conn, sql, args, anonSQL, anonArgs, reps)
		} else {
			durs, err = timeIt(ctx, conn, sql, args, reps, sc.rollback)
		}
		if err != nil {
			return fmt.Errorf("%s：%w", sc.label, err)
		}
		plan, err := explain(ctx, conn, sql, args, sc.rollback)
		if err != nil {
			return fmt.Errorf("%s 的 EXPLAIN：%w", sc.label, err)
		}
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
		p50, p95, pmax := durs[len(durs)/2], durs[len(durs)*95/100], durs[len(durs)-1]
		funcScan := strings.Contains(plan, "Function Scan")
		seq := seqScans(plan)
		flag := ""
		if p95 > 300*time.Millisecond && sc.anon == nil {
			flag = " **超過門檻**"
			exceeded++
		}
		anonCol, ratioCol := "—", "—"
		if sc.anon != nil {
			sort.Slice(anonDurs, func(i, j int) bool { return anonDurs[i] < anonDurs[j] })
			a50, a95 := anonDurs[len(anonDurs)/2], anonDurs[len(anonDurs)*95/100]
			anonCol = fmt.Sprintf("%.2f / %.2f", ms(a50), ms(a95))
			r50, r95 := ms(p50)/ms(a50), ms(p95)/ms(a95)
			ratioCol = fmt.Sprintf("%.2f / %.2f", r50, r95)
			if r95 > ratioLimit && ms(p95) > floorMs {
				flag = " **比值超過**"
				regressed++
			}
		}
		keyset := "—"
		if sc.keyset {
			if rowCmpIndexCond.MatchString(plan) {
				keyset = "是"
			} else if setDriven[sc.query] {
				keyset = "集合驅動（Filter）"
			} else {
				keyset = "**否**"
				noIndexCond++
			}
		}
		blocksScan := "無"
		if blocksScanRe.MatchString(plan) {
			blocksScan = "**有**"
			blocksScans++
		}
		fmt.Printf("| %s | %.2f | %.2f%s | %.2f | %s | %s | %v | %s | %s | %s |\n", sc.label, ms(p50), ms(p95), flag, ms(pmax), anonCol, ratioCol, funcScan, seq, keyset, blocksScan)
		fmt.Fprintf(&explains, "=== %s（%s）p95 %.2f ms\n%s\n\n", sc.label, sc.query, ms(p95), plan)
	}
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, "explain.txt"), []byte(explains.String()), 0o644); err != nil {
			return err
		}
	}
	if blocksScans > 0 {
		fmt.Printf("\n%d 個情境的計畫對封鎖表做 Seq Scan 或 Function Scan\n", blocksScans)
		os.Exit(4)
	}
	if noIndexCond > 0 {
		fmt.Printf("\n%d 個 keyset 情境的計畫沒有列比較的 Index Cond\n", noIndexCond)
		os.Exit(3)
	}
	if regressed > 0 {
		fmt.Printf("\n%d 個觀看者變體的 p95 超過匿名對照的 %.1f 倍（且觀看者 p95 高於 %.0f ms）\n", regressed, ratioLimit, floorMs)
		os.Exit(5)
	}
	if exceeded > 0 {
		fmt.Printf("\n%d 個情境的 p95 超過 300 ms\n", exceeded)
		os.Exit(2)
	}
	return nil
}

// blocksScanRe 是對封鎖表的整表掃描：pub_blocked_*() 沒被內聯成封鎖表的索引探測時，會是 Function Scan on pub_blocked_* 或 Seq Scan on blocked_*。
var blocksScanRe = regexp.MustCompile(`Seq Scan on blocked_(stores|buyers)|Function Scan on pub_blocked_(stores|buyers)`)

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// seqScans 列出計畫裡的 Seq Scan 所掃的表（小表可以接受，由閱讀者判斷）。
func seqScans(plan string) string {
	re := regexp.MustCompile(`Seq Scan on (\S+)`)
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(plan, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		return "無"
	}
	return strings.Join(out, "、")
}

func timeIt(ctx context.Context, conn *pgx.Conn, sql string, args []any, reps int, rollback bool) ([]time.Duration, error) {
	exec := func() error {
		if rollback {
			tx, err := conn.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback(ctx) }()
			return drain(tx.Query(ctx, sql, args...))
		}
		return drain(conn.Query(ctx, sql, args...))
	}
	for range 10 {
		if err := exec(); err != nil {
			return nil, err
		}
	}
	durs := make([]time.Duration, 0, reps)
	for range reps {
		t0 := time.Now()
		if err := exec(); err != nil {
			return nil, err
		}
		durs = append(durs, time.Since(t0))
	}
	return durs, nil
}

// timePair 交錯計時一個觀看者語句與它的匿名對照：每一輪先匿名、再觀看者，兩者吃到同樣的機器負載。
func timePair(ctx context.Context, conn *pgx.Conn, sql string, args []any, anonSQL string, anonArgs []any, reps int) (viewer, anon []time.Duration, err error) {
	run := func(q string, a []any) (time.Duration, error) {
		t0 := time.Now()
		if err := drain(conn.Query(ctx, q, a...)); err != nil {
			return 0, err
		}
		return time.Since(t0), nil
	}
	for range 10 {
		if _, err := run(anonSQL, anonArgs); err != nil {
			return nil, nil, err
		}
		if _, err := run(sql, args); err != nil {
			return nil, nil, err
		}
	}
	for range reps {
		d, err := run(anonSQL, anonArgs)
		if err != nil {
			return nil, nil, err
		}
		anon = append(anon, d)
		if d, err = run(sql, args); err != nil {
			return nil, nil, err
		}
		viewer = append(viewer, d)
	}
	return viewer, anon, nil
}

func drain(rows pgx.Rows, err error) error {
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func literal(v any) string {
	switch x := v.(type) {
	case time.Time:
		return fmt.Sprintf("'%s'::timestamptz", x.Format("2006-01-02 15:04:05.999999-07"))
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case bool:
		return fmt.Sprint(x)
	default:
		return fmt.Sprint(x)
	}
}

func explain(ctx context.Context, conn *pgx.Conn, sql string, args []any, rollback bool) (string, error) {
	lits := make([]string, len(args))
	for i, a := range args {
		lits[i] = literal(a)
	}
	// PREPARE 讓 EXPLAIN 看到的是 generic plan；名稱每次不同以免衝突。
	name := fmt.Sprintf("q_%d", time.Now().UnixNano())
	var tx pgx.Tx
	var err error
	if tx, err = conn.Begin(ctx); err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "PREPARE "+name+" AS "+sql); err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) EXECUTE "+name+"("+strings.Join(lits, ", ")+")")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", err
		}
		b.WriteString(line + "\n")
	}
	return b.String(), rows.Err()
}
