//go:build integration

package social_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// SD17：並發雙擊讚只有一列、只加一、只發一次 PostLiked；取消讚減一；重複取消不會變負數。重複幾輪，並發的窗口才容易撞上。
func TestSD17ConcurrentLikeCountsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	var mu sync.Mutex
	liked := map[uuidT]int{}
	events.Subscribe(f.bus, func(_ context.Context, e social.PostLiked) error {
		mu.Lock()
		defer mu.Unlock()
		liked[e.PostID]++
		return nil
	})
	var first social.Post
	var firstBuyer buyerP
	for round := range 6 {
		p := f.publishedPost(t, storeID, "貼文")
		b := f.newBuyer(t)
		if round == 0 {
			first, firstBuyer = p, b
		}
		var wg sync.WaitGroup
		added := make(chan bool, 16)
		start := make(chan struct{})
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ok, err := f.module.LikePost(ctx, b.BuyerID, p.ID)
				if err != nil {
					t.Error(err)
				}
				added <- ok
			}()
		}
		close(start)
		wg.Wait()
		close(added)
		n := 0
		for ok := range added {
			if ok {
				n++
			}
		}
		mu.Lock()
		events := liked[p.ID]
		mu.Unlock()
		if n != 1 || events != 1 {
			t.Fatalf("第 %d 輪：16 個並發的讚只新增一次：新增 %d、事件 %d", round, n, events)
		}
		if rows := f.scalar(t, "SELECT count(*) FROM social.post_likes WHERE post_id = $1", p.ID); rows != 1 {
			t.Fatalf("只有一列：%d", rows)
		}
		if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Likes != 1 {
			t.Fatalf("讚數 %d", got.Stats.Likes)
		}
	}
	set := mustV[map[uuidT]bool](t)(f.module.PostsLikedBy(ctx, firstBuyer.BuyerID, []uuidT{first.ID}))
	if !set[first.ID] {
		t.Fatal("PostsLikedBy")
	}
	if ok := mustV[bool](t)(f.module.UnlikePost(ctx, firstBuyer.BuyerID, first.ID)); !ok {
		t.Fatal("取消讚")
	}
	if ok := mustV[bool](t)(f.module.UnlikePost(ctx, firstBuyer.BuyerID, first.ID)); ok {
		t.Fatal("重複取消回 false")
	}
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, first.ID)); got.Stats.Likes != 0 {
		t.Fatalf("取消後 %d", got.Stats.Likes)
	}
}

// 留言的讚：互動者依貼文歸屬推導（店主是店家、其他是買家），只對公開可讀的留言按讚；列表帶 liked_by_me。
func TestCommentLikesAreByActorAndOnlyOnReadableComments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	author, fan := f.newBuyer(t), f.newBuyer(t)
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(author), social.CommentInput{PostID: p.ID, Body: "留言"}))
	for _, caller := range []social.Caller{f.asBuyer(fan), f.asStore(t, storeID)} {
		if ok := mustV[bool](t)(f.module.LikeComment(ctx, caller, c.ID)); !ok {
			t.Fatal("按讚")
		}
		if ok := mustV[bool](t)(f.module.LikeComment(ctx, caller, c.ID)); ok {
			t.Fatal("重複按讚回 false")
		}
	}
	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, f.asBuyer(fan), p.ID, nil, 10))
	if page.Comments[0].Likes != 2 || !page.Comments[0].LikedByMe {
		t.Fatalf("%+v", page.Comments[0])
	}
	if page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, f.asBuyer(author), p.ID, nil, 10)); page.Comments[0].LikedByMe {
		t.Fatal("作者沒按過")
	}
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	third := f.newBuyer(t)
	if _, err := f.module.LikeComment(ctx, f.asBuyer(third), c.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("被隱藏的留言不能按讚：%v", err)
	}
	if ok := mustV[bool](t)(f.module.UnlikeComment(ctx, f.asBuyer(fan), c.ID)); !ok {
		t.Fatal("內容被隱藏之後仍然可以收回自己的讚")
	}
}

// SD18：瀏覽同一篇、同一個台北日、同一個觀看者只算一次，換日再算；匿名沒有 IP 不計；登入者用帳號；表裡沒有 IP 與 UA 的欄位；salt 每日換。
func TestSD18ViewsAreDeduplicatedPerTaipeiDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	views := func() int64 {
		return mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)).Stats.Views
	}
	count := func(v social.Viewer) bool { return mustV[bool](t)(f.module.RecordView(ctx, p.ID, v)) }

	anon := social.Viewer{IP: "203.0.113.7", UserAgent: "Mozilla/5.0"}
	if !count(anon) || count(anon) || views() != 1 {
		t.Fatalf("同日同 IP 與 UA 只計一次：%d", views())
	}
	if !count(social.Viewer{IP: "203.0.113.7", UserAgent: "curl/8"}) || !count(social.Viewer{IP: "203.0.113.8", UserAgent: "Mozilla/5.0"}) || views() != 3 {
		t.Fatalf("換 IP 或 UA 是不同的觀看者：%d", views())
	}
	if count(social.Viewer{UserAgent: "Mozilla/5.0"}) || views() != 3 {
		t.Fatal("沒有 IP 不計數")
	}
	acct := social.Viewer{AccountID: 4242, IP: "198.51.100.1"}
	if !count(acct) || count(acct) || count(social.Viewer{AccountID: 4242, IP: "198.51.100.99"}) || views() != 4 {
		t.Fatalf("登入者用帳號，與 IP 無關：%d", views())
	}
	// 換日（台北日 00:00 之後）再計。台北日以 +08:00 為界：UTC 15:59 與 16:00 是不同的台北日。
	f.mu.Lock()
	f.now = time.Date(2026, 10, 23, 15, 59, 0, 0, time.UTC)
	f.mu.Unlock()
	if count(anon) {
		t.Fatal("同一個台北日（10/23 23:59）已計過")
	}
	f.advance(2 * time.Minute)
	if !count(anon) || count(anon) || !count(acct) || views() != 6 {
		t.Fatalf("換日再計一次：%d", views())
	}
	if n := f.scalar(t, "SELECT count(DISTINCT salt) FROM social.view_salts"); n != 2 {
		t.Fatalf("每個台北日一把 salt：%d", n)
	}
	if n := f.scalar(t, "SELECT count(DISTINCT viewer_key) FROM social.post_views WHERE viewer_key IS NOT NULL"); n != 6 {
		t.Fatalf("換日後同一個觀看者的鍵不同（salt 輪替）：%d", n)
	}
	// 表裡沒有 IP 與 UA 的欄位，也沒有可回推帳號的值。
	var cols []string
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT column_name FROM information_schema.columns WHERE table_schema = 'social' AND table_name IN ('post_views', 'view_salts')")
	must(t, err)
	for rows.Next() {
		var c string
		must(t, rows.Scan(&c))
		cols = append(cols, c)
	}
	must(t, rows.Err())
	for _, c := range cols {
		switch c {
		case "post_id", "view_day", "viewer_key", "salt":
		default:
			t.Fatalf("瀏覽的表不該有欄位 %q", c)
		}
	}
	// 不公開的貼文不計；清理 job 只刪舊的去重列與 salt，不扣瀏覽數。
	draft := mustV[social.Post](t)(f.module.CreatePost(ctx, f.store(t, storeID), social.CreatePostInput{Body: "草稿"}))
	if mustV[bool](t)(f.module.RecordView(ctx, draft.ID, anon)) {
		t.Fatal("草稿不計瀏覽")
	}
	f.advance(48 * time.Hour)
	must(t, f.module.CleanupViews(ctx))
	if n := f.scalar(t, "SELECT count(*) FROM social.post_views"); n != 0 {
		t.Fatalf("兩天前的去重列已刪：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.view_salts"); n != 0 {
		t.Fatalf("前一日以前的 salt 已刪：%d", n)
	}
	if views() != 6 {
		t.Fatalf("清理不扣瀏覽數：%d", views())
	}
}

// 分享：同一位買家同一篇只算一次，只對公開可讀的貼文。
func TestSharesAreDeduplicatedPerBuyer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	a, b := f.newBuyer(t), f.newBuyer(t)
	if !mustV[bool](t)(f.module.RecordShare(ctx, a.BuyerID, p.ID)) || mustV[bool](t)(f.module.RecordShare(ctx, a.BuyerID, p.ID)) {
		t.Fatal("同一位買家只算一次")
	}
	mustV[bool](t)(f.module.RecordShare(ctx, b.BuyerID, p.ID))
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Shares != 2 {
		t.Fatalf("分享數 %d", got.Stats.Shares)
	}
	must(t, f.module.DeletePost(ctx, f.store(t, storeID), p.ID, 0))
	if _, err := f.module.RecordShare(ctx, a.BuyerID, p.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("刪除的貼文不能分享：%v", err)
	}
}

// 熱門（本週主題）：最近發布的 30 篇公開貼文中瀏覽數最高的 n 篇，同分以發布時間、ID 排；不公開的不算。
func TestTopPostsRanksTheLatestThirtyByViews(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	var posts []social.Post
	for range 4 {
		f.advance(time.Minute)
		posts = append(posts, f.publishedPost(t, storeID, "貼文"))
	}
	view := func(p social.Post, n int) {
		for i := range n {
			mustV[bool](t)(f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: int64(1000 + i)}))
		}
	}
	view(posts[0], 3)
	view(posts[2], 3)
	view(posts[1], 1)
	got := mustV[[]social.Post](t)(f.module.TopPosts(ctx, social.Caller{}, 3))
	if len(got) != 3 || got[0].ID != posts[2].ID || got[1].ID != posts[0].ID || got[2].ID != posts[1].ID {
		t.Fatalf("依瀏覽數、同分較新的在前：%v", ids(got))
	}
	must(t, f.module.DeletePost(ctx, f.store(t, storeID), posts[2].ID, 0))
	if got := mustV[[]social.Post](t)(f.module.TopPosts(ctx, social.Caller{}, 1)); got[0].ID != posts[0].ID {
		t.Fatalf("刪除的不算：%v", ids(got))
	}
	if got := mustV[[]social.Post](t)(f.module.TopPosts(ctx, social.Caller{}, 99)); len(got) != 3 {
		t.Fatalf("n 最多 30，只有 3 篇公開：%d", len(got))
	}
}

func ids(ps []social.Post) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID.String()[:8]
	}
	return out
}
