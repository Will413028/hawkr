//go:build integration

package social_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

func planLiteral(a any) string {
	switch x := a.(type) {
	case time.Time:
		return "'" + x.UTC().Format(time.RFC3339Nano) + "'::timestamptz"
	case uuid.UUID:
		return "'" + x.String() + "'::uuid"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	default:
		return fmt.Sprint(x)
	}
}

var rowCondIndex = regexp.MustCompile(`Index Cond: .*ROW\(`)

var errPlanDone = errors.New("plan done")

// 公開讀取加上內容下架的 NOT EXISTS 之後，各個貼文讀取在 generic plan 下仍然沒有 Function Scan、不對 posts 做 Seq Scan，keyset 的列比較仍是 Index Cond。
// 資料量夠大（planner 才會選索引），其中一部分貼文被下架。
func TestTakedownPredicateKeepsTheReadPlansIndexed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	storeID, _ := f.exposableStore(t)
	f.exec(t, `INSERT INTO social.posts (id, store_id, body, status, published_at, created_at, updated_at)
		SELECT uuidv7(), $1, 'p', 'PUBLISHED', $2::timestamptz - make_interval(secs => g), $2, $2 FROM generate_series(1, 6000) AS g`, storeID, f.clock())
	f.exec(t, `INSERT INTO social.post_hashtags (post_id, name, display_name, position)
		SELECT id, 'organic', 'organic', 0 FROM social.posts`)
	pick := f.scalar(t, "SELECT count(*) FROM social.posts")
	if pick != 6000 {
		t.Fatalf("前提：貼文 %d", pick)
	}
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT id FROM social.posts ORDER BY published_at DESC LIMIT 30")
	must(t, err)
	var refs []seller.ContentRef
	for rows.Next() {
		var id uuid.UUID
		must(t, rows.Scan(&id))
		refs = append(refs, ref(seller.ContentPost, id))
	}
	rows.Close()
	for i := 0; i < len(refs); i += 10 {
		td.create(t, storeID, refs[i:i+10]...)
	}
	f.exec(t, "ANALYZE social.posts")
	f.exec(t, "ANALYZE social.post_hashtags")
	f.exec(t, "ANALYZE social.post_stats")
	f.exec(t, "ANALYZE social.content_takedowns")

	now := f.clock()
	page := reads.PostPage{AfterPublished: now, AfterID: uuid.UUID{0xff}, Limit: 51}
	someID := refs[0].ID
	for _, sc := range []struct {
		name   string
		stmt   SelectStatement
		keyset bool
	}{
		{"動態牆", reads.ListFeedPosts(now, visibility.Anonymous, page), true},
		{"店面", reads.ListStorePublicPosts(storeID, now, visibility.Anonymous, page), true},
		{"標籤", reads.ListHashtagPosts("organic", now, visibility.Anonymous, page), true},
		{"單篇", reads.GetPublicPost(someID, now, visibility.Anonymous), false},
	} {
		sql, args := sc.stmt.Sql()
		lits := make([]string, len(args))
		for i, a := range args {
			lits[i] = planLiteral(a)
		}
		name := "plan_" + strings.ReplaceAll(uuid.NewV7().String(), "-", "")
		var plan strings.Builder
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			c := f.db.Conn(ctx)
			if _, err := c.Exec(ctx, "SET LOCAL plan_cache_mode = force_generic_plan"); err != nil {
				return err
			}
			if _, err := c.Exec(ctx, "PREPARE "+name+" AS "+sql); err != nil {
				return fmt.Errorf("%s：%w\n%s", sc.name, err, sql)
			}
			r, err := c.Query(ctx, "EXPLAIN (COSTS OFF) EXECUTE "+name+"("+strings.Join(lits, ", ")+")")
			if err != nil {
				return err
			}
			defer r.Close()
			for r.Next() {
				var line string
				if err := r.Scan(&line); err != nil {
					return err
				}
				plan.WriteString(line + "\n")
			}
			if err := r.Err(); err != nil {
				return err
			}
			return errPlanDone
		})
		if !errors.Is(err, errPlanDone) {
			t.Fatal(err)
		}
		p := plan.String()
		if strings.Contains(p, "Function Scan") {
			t.Errorf("%s：計畫有 Function Scan\n%s", sc.name, p)
		}
		if strings.Contains(p, "Seq Scan on posts") {
			t.Errorf("%s：計畫對 posts 做 Seq Scan\n%s", sc.name, p)
		}
		if sc.keyset && !rowCondIndex.MatchString(p) {
			t.Errorf("%s：計畫沒有列比較的 Index Cond\n%s", sc.name, p)
		}
		t.Logf("%s\n%s", sc.name, p)
	}
}
