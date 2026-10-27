//go:build integration

package social_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

func TestSmoke(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "新鮮的蔬菜", "蔬菜")
	got, err := f.module.PublicPost(ctx, social.Caller{}, p.ID)
	must(t, err)
	if got.Body != "新鮮的蔬菜" || len(got.Hashtags) != 1 || got.Status != social.StatusPublished {
		t.Fatalf("%+v", got)
	}
	b := f.newBuyer(t)
	c, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "好吃"})
	must(t, err)
	page, err := f.module.ListComments(ctx, f.asBuyer(b), p.ID, nil, 10)
	must(t, err)
	if len(page.Comments) != 1 || page.Comments[0].ID != c.ID {
		t.Fatalf("%+v", page)
	}
	got, _ = f.module.PublicPost(ctx, social.Caller{}, p.ID)
	if got.Stats.Comments != 1 {
		t.Fatalf("comment count %d", got.Stats.Comments)
	}
}
