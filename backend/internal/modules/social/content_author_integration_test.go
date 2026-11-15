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
	a, x, y, z := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	store := f.asStore(t, storeID)
	cX := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(x), social.CommentInput{PostID: p.ID, Body: "X"}))
	rY := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(y), social.CommentInput{PostID: p.ID, Body: "Y 回 X", ParentID: &cX.ID}))
	cZ := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(z), social.CommentInput{PostID: p.ID, Body: "Z"}))
	cS := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家"}))

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

	t.Run("被封鎖者的頂層讀不到", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, cX.ID); got.Found {
			t.Fatalf("被封鎖者的頂層：%+v", got)
		}
	})
	t.Run("被封鎖的頂層底下別人的回覆也讀不到（父列條件）", func(t *testing.T) {
		if got := f.authorFor(t, f.asBuyer(a), social.TargetComment, rY.ID); got.Found {
			t.Fatalf("X 的頂層被排除，Y 的回覆也要讀不到：%+v", got)
		}
	})
	t.Run("被封鎖者看不到封鎖者的留言（雙向）", func(t *testing.T) {
		cA := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(a), social.CommentInput{PostID: p.ID, Body: "A"}))
		if got := f.authorFor(t, f.asBuyer(x), social.TargetComment, cA.ID); got.Found {
			t.Fatalf("雙向：%+v", got)
		}
	})
	t.Run("第三人與匿名讀得到", func(t *testing.T) {
		for name, c := range map[string]social.Caller{"第三人": f.asBuyer(z), "匿名": {}} {
			if got := f.authorFor(t, c, social.TargetComment, rY.ID); !got.Found || got.Author.ID != y.BuyerID {
				t.Fatalf("%s：%+v", name, got)
			}
		}
	})
	t.Run("店主讀自己的店，不套個人封鎖", func(t *testing.T) {
		got := f.authorFor(t, f.asStore(t, storeID), social.TargetComment, cX.ID)
		if !got.Found || got.Author.ID != x.BuyerID {
			t.Fatalf("店主：%+v", got)
		}
	})
	t.Run("買家封鎖的店的留言讀不到", func(t *testing.T) {
		b := f.newBuyer(t)
		f.blockStore(t, b, storeID)
		if got := f.authorFor(t, f.asBuyer(b), social.TargetComment, cZ.ID); got.Found {
			t.Fatalf("封鎖的店：%+v", got)
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
	a, x, y := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	eX := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(x), social.ForumInput{StoreID: storeID, Body: "X"}))
	eY := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(y), social.ForumInput{StoreID: storeID, Body: "Y 回 X", ParentID: &eX.ID}))
	f.blockBuyer(t, a, x)

	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eX.ID); got.Found {
		t.Fatalf("被封鎖者的頂層：%+v", got)
	}
	if got := f.authorFor(t, f.asBuyer(a), social.TargetForumEntry, eY.ID); got.Found {
		t.Fatalf("父列被排除，別人的回覆也讀不到：%+v", got)
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
	if got := f.authorFor(t, f.asBuyer(b), social.TargetForumEntry, eY.ID); got.Found {
		t.Fatalf("封鎖的店的討論區：%+v", got)
	}
}
