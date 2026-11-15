//go:build integration

package social_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// ContentAuthorFor（A7-2e）：從留言、討論區封鎖作者前的解析，套用與列表相同的觀看者規則。

func (f *fixture) authorFor(t *testing.T, c social.Caller, kind social.TargetKind, id [16]byte) social.ContentAuthor {
	t.Helper()
	got, err := f.module.ContentAuthorFor(context.Background(), c, social.Target{Kind: kind, ID: id})
	if err != nil {
		t.Fatalf("ContentAuthorFor：%v", err)
	}
	return got
}

func TestContentAuthorForFollowsTheViewersReadRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, x, z, w := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	store := f.asStore(t, storeID)
	cX := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(x), social.CommentInput{PostID: p.ID, Body: "X"}))
	rX := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家回 X", ParentID: &cX.ID}))
	cZ := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(z), social.CommentInput{PostID: p.ID, Body: "Z"}))
	cS := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家"}))
	cW := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(w), social.CommentInput{PostID: p.ID, Body: "W"}))
	rW := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家回 W", ParentID: &cW.ID}))

	t.Run("讀得到的留言回作者與店家", func(t *testing.T) {
		got := f.authorFor(t, f.asBuyer(a), social.TargetComment, cZ.ID)
		if !got.Found || got.Author != (social.Actor{Kind: social.ActorBuyer, ID: z.BuyerID}) || got.StoreID != storeID {
			t.Fatalf("作者：%+v", got)
		}
		got = f.authorFor(t, f.asBuyer(a), social.TargetComment, cS.ID)
		if !got.Found || got.Author.Kind != social.ActorStore || got.Author.ID != storeID {
			t.Fatalf("店家的留言：%+v", got)
		}
	})
	t.Run("不存在的 ID", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, [16]byte{1}); got.Found {
			t.Fatalf("不存在：%+v", got)
		}
	})
	t.Run("貼文不是可解析的對象", func(t *testing.T) {
		if _, err := f.module.ContentAuthorFor(ctx, f.asBuyer(a), social.Target{Kind: social.TargetPost, ID: p.ID}); err == nil {
			t.Fatal("貼文要被拒絕")
		}
	})

	f.blockBuyer(t, a, x) // A 封鎖 X
	f.blockBuyer(t, w, a) // W 封鎖 A

	t.Run("自己封鎖的買家的頂層仍讀得到（重複封鎖要能解析）", func(t *testing.T) {
		got := f.authorFor(t, f.asBuyer(a), social.TargetComment, cX.ID)
		if !got.Found || got.Author.ID != x.BuyerID {
			t.Fatalf("A 封鎖 X 之後：%+v", got)
		}
	})
	t.Run("自己封鎖的頂層底下店家的回覆也讀得到（父列條件同樣略過自己的封鎖）", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, rX.ID); !got.Found || got.Author != (social.Actor{Kind: social.ActorStore, ID: storeID}) {
			t.Fatalf("A 封鎖 X，店家的回覆：%+v", got)
		}
	})
	t.Run("對方封鎖自己的買家的頂層讀不到（不洩漏被封鎖）", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, cW.ID); got.Found {
			t.Fatalf("W 封鎖 A：%+v", got)
		}
	})
	t.Run("對方封鎖自己的頂層底下店家的回覆也讀不到（父列條件）", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, rW.ID); got.Found {
			t.Fatalf("W 的頂層被排除，店家的回覆也要讀不到：%+v", got)
		}
	})
	t.Run("被封鎖者看不到封鎖者的留言（雙向）", func(t *testing.T) {
		cA := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(a), social.CommentInput{PostID: p.ID, Body: "A"}))
		if got := f.authorFor(t, f.asBuyer(x), social.TargetComment, cA.ID); got.Found {
			t.Fatalf("雙向：%+v", got)
		}
	})
	t.Run("第三人讀得到", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(z), social.TargetComment, rX.ID); !got.Found || got.Author != (social.Actor{Kind: social.ActorStore, ID: storeID}) {
			t.Fatalf("第三人：%+v", got)
		}
	})
	t.Run("沒有身分的讀取者讀不到（只有買家或店家身分才解析，正式流程不會匿名呼叫）", func(t *testing.T) {
		if got := f.authorFor(t, social.Caller{}, social.TargetComment, rX.ID); got.Found {
			t.Fatalf("匿名：%+v", got)
		}
	})
	t.Run("店主讀自己的店，不套個人封鎖", func(t *testing.T) {
		got := f.authorFor(t, f.asStore(t, storeID), social.TargetComment, cX.ID)
		if !got.Found || got.Author.ID != x.BuyerID {
			t.Fatalf("店主：%+v", got)
		}
	})
	t.Run("自己封鎖的店的留言仍讀得到（重複封鎖店要能解析）", func(t *testing.T) {
		b := f.newBuyer(t)
		f.blockStore(t, b, storeID)
		got := f.authorFor(t, f.asBuyer(b), social.TargetComment, cS.ID)
		if !got.Found || got.Author != (social.Actor{Kind: social.ActorStore, ID: storeID}) {
			t.Fatalf("封鎖的店的留言：%+v", got)
		}
	})
	t.Run("只有店家身分的讀取者讀不到別家店的留言", func(t *testing.T) {
		otherID, _ := f.exposableStore(t)
		if got := f.authorFor(t, f.asStore(t, otherID), social.TargetComment, cZ.ID); got.Found {
			t.Fatalf("別家店：%+v", got)
		}
	})
	t.Run("已刪除的留言讀不到", func(t *testing.T) {
		if err := f.module.DeleteComment(ctx, f.asBuyer(z), cZ.ID, cZ.Version); err != nil {
			t.Fatal(err)
		}
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, cZ.ID); got.Found {
			t.Fatalf("已刪除：%+v", got)
		}
	})
}

func TestContentAuthorForForumEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, x, y, w := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	eX := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(x), social.ForumInput{StoreID: storeID, Body: "X"}))
	eY := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(y), social.ForumInput{StoreID: storeID, Body: "Y 回 X", ParentID: &eX.ID}))
	eW := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(w), social.ForumInput{StoreID: storeID, Body: "W"}))
	eYW := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(y), social.ForumInput{StoreID: storeID, Body: "Y 回 W", ParentID: &eW.ID}))
	f.blockBuyer(t, a, x)
	f.blockBuyer(t, w, a)

	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eX.ID); !got.Found {
		t.Fatalf("自己封鎖的買家的頂層仍讀得到：%+v", got)
	}
	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eY.ID); !got.Found {
		t.Fatalf("自己封鎖的頂層底下別人的回覆讀得到：%+v", got)
	}
	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eW.ID); got.Found {
		t.Fatalf("對方封鎖自己：%+v", got)
	}
	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eYW.ID); got.Found {
		t.Fatalf("父列被對方的封鎖排除，別人的回覆也讀不到：%+v", got)
	}
	got := f.authorFor(t, f.asBuyer(y), social.TargetForumEntry, eY.ID)
	if !got.Found || got.Author != (social.Actor{Kind: social.ActorBuyer, ID: y.BuyerID}) || got.StoreID != storeID {
		t.Fatalf("自己的回覆：%+v", got)
	}
	if got := f.authorFor(t, f.asStore(t, storeID), social.TargetForumEntry, eX.ID); !got.Found {
		t.Fatalf("店主不套個人封鎖：%+v", got)
	}
	b := f.newBuyer(t)
	f.blockStore(t, b, storeID)
	if got := f.authorFor(t, f.asBuyer(b), social.TargetForumEntry, eY.ID); !got.Found {
		t.Fatalf("自己封鎖的店的討論區仍讀得到：%+v", got)
	}
}
