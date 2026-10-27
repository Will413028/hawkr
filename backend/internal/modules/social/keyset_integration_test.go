//go:build integration

package social_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 第一頁用方向的極值游標（model.DescStart／AscStart），所以「沒給游標」與「給極值游標」必須是同一頁；逐頁走完沒有缺漏也沒有重複
// （含發布時間相同、靠 id 決勝的列：測試不推進時鐘）。
func TestFirstPageEqualsSentinelCursorAndDeepPagesHaveNoGapsOrDupes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	for i := 0; i < 7; i++ {
		f.publishedPost(t, storeID, "貼文")
		if i%3 == 2 {
			f.advance(1)
		}
	}
	post := f.publishedPost(t, storeID, "留言的貼文")
	for i := 0; i < 7; i++ {
		mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: post.ID, Body: "留言"}))
	}

	// 由大到小：店家頁。
	at, id := model.DescStart()
	first := mustV[social.PostPage](t)(f.module.ListStorePosts(ctx, social.Caller{}, storeID, nil, 50))
	sent := mustV[social.PostPage](t)(f.module.ListStorePosts(ctx, social.Caller{}, storeID, &social.PostCursor{PublishedAt: at, ID: id}, 50))
	if len(first.Posts) != 8 || len(sent.Posts) != 8 {
		t.Fatalf("第一頁筆數：%d／%d", len(first.Posts), len(sent.Posts))
	}
	for i := range first.Posts {
		if first.Posts[i].ID != sent.Posts[i].ID {
			t.Fatalf("第一頁與極值游標的一頁在第 %d 筆不同", i)
		}
	}
	var got []uuid.UUID
	var cur *social.PostCursor
	for pages := 0; pages < 10; pages++ {
		pg := mustV[social.PostPage](t)(f.module.ListStorePosts(ctx, social.Caller{}, storeID, cur, 3))
		for _, p := range pg.Posts {
			got = append(got, p.ID)
		}
		if pg.Next == nil {
			break
		}
		cur = pg.Next
	}
	if len(got) != len(first.Posts) {
		t.Fatalf("逐頁走完共 %d 筆，一次取 %d 筆", len(got), len(first.Posts))
	}
	seen := map[uuid.UUID]bool{}
	for i, g := range got {
		if seen[g] || g != first.Posts[i].ID {
			t.Fatalf("第 %d 筆重複或順序不同：%s", i, g)
		}
		seen[g] = true
	}

	// 由小到大：頂層留言。
	at, id = model.AscStart()
	cf := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, post.ID, nil, 50))
	cs := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, post.ID, &social.CommentCursor{CreatedAt: at, ID: id}, 50))
	if len(cf.Comments) != 7 || len(cs.Comments) != 7 {
		t.Fatalf("留言第一頁筆數：%d／%d", len(cf.Comments), len(cs.Comments))
	}
	var gotC []uuid.UUID
	var cc *social.CommentCursor
	for pages := 0; pages < 10; pages++ {
		pg := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, post.ID, cc, 2))
		for _, c := range pg.Comments {
			gotC = append(gotC, c.ID)
		}
		if pg.Next == nil {
			break
		}
		cc = pg.Next
	}
	if len(gotC) != 7 {
		t.Fatalf("留言逐頁共 %d 筆", len(gotC))
	}
	for i := range gotC {
		if gotC[i] != cf.Comments[i].ID || cf.Comments[i].ID != cs.Comments[i].ID {
			t.Fatalf("留言第 %d 筆順序不同", i)
		}
	}
}

// 熱門標籤的窗口最長 30 天：剛好 30 天可以，多一點就是 InvalidInput；開始不早於結束也是。
func TestTrendingHashtagsWindowIsCappedAtThirtyDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	to := f.clock()
	if _, err := f.module.TrendingHashtags(ctx, to.Add(-model.MaxTrendingWindow), to, 10); err != nil {
		t.Fatalf("剛好 30 天：%v", err)
	}
	for name, from := range map[string]time.Time{
		"30 天多 1 微秒": to.Add(-model.MaxTrendingWindow - time.Microsecond),
		"365 天":      to.AddDate(-1, 0, 0),
	} {
		if _, err := f.module.TrendingHashtags(ctx, from, to, 10); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if _, err := f.module.TrendingHashtags(ctx, to, to, 10); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("空窗口：%v", err)
	}
}
