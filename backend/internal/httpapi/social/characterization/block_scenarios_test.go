package characterization

import (
	"context"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

func blockAuthorByCommentReferenceIsIdempotent(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	if changed := ch.Must(t, ch.Ret(d.BlockAuthor(ctx, ch.Buyer, c.ID))); !changed {
		t.Errorf("第一次封鎖 changed = false")
	}
	if changed := ch.Must(t, ch.Ret(d.BlockAuthor(ctx, ch.Buyer, c.ID))); changed {
		t.Errorf("重複封鎖 changed = true，封鎖要冪等")
	}
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Buyer, p.ID))); hasComment(list, c.ID) {
		t.Errorf("封鎖者讀得到被封鎖者的留言：%+v", list)
	}
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))); !hasComment(list, c.ID) {
		t.Errorf("匿名讀不到被封鎖者的留言：%+v", list)
	}
	mine := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "我的留言 "+ch.Nonce())))
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Buyer2, p.ID))); hasComment(list, mine.ID) {
		t.Errorf("被封鎖者讀得到封鎖者的留言：%+v", list)
	}
}

func blockByUnreadableCommentIsNotFound(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	wantNoError(t, d.DeleteComment(ctx, ch.Buyer2, c.ID), "刪除留言")
	_, err := d.BlockAuthor(ctx, ch.Buyer, c.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	_, err = d.SellerBlockAuthor(ctx, ch.Seller, c.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	_, err = d.BlockAuthor(ctx, ch.Anonymous, c.ID)
	ch.WantKind(t, err, ch.ErrUnauthenticated)
}

func d2BuyerBlockingAStoreHidesItsPostsOnlyForTheBlocker(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	if changed := ch.Must(t, ch.Ret(d.BlockStore(ctx, ch.Buyer, p.SellerID))); !changed {
		t.Errorf("封鎖店家 changed = false")
	}
	if posts := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Buyer, p.SellerID, 1, 20))); hasPost(posts.Items, p.ID) {
		t.Errorf("封鎖者讀得到被封鎖的店的貼文：%+v", posts.Items)
	}
	if posts := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Buyer2, p.SellerID, 1, 20))); !hasPost(posts.Items, p.ID) {
		t.Errorf("別的買家讀不到店的貼文：%+v", posts.Items)
	}
	if changed := ch.Must(t, ch.Ret(d.UnblockStore(ctx, ch.Buyer, p.SellerID))); !changed {
		t.Errorf("解除封鎖 changed = false")
	}
	if changed := ch.Must(t, ch.Ret(d.UnblockStore(ctx, ch.Buyer, p.SellerID))); changed {
		t.Errorf("重複解除 changed = true，解除要冪等")
	}
	if posts := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Buyer, p.SellerID, 1, 20))); !hasPost(posts.Items, p.ID) {
		t.Errorf("解除封鎖之後讀得到店的貼文：%+v", posts.Items)
	}
}

func sd3StoreBlockingABuyerStopsWritesButNotReads(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	ch.Must(t, ch.Ret(d.SellerBlockAuthor(ctx, ch.Seller, c.ID)))
	_, err := d.CreateComment(ctx, ch.Buyer2, p.ID, "再留言 "+ch.Nonce())
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.ToggleLike(ctx, ch.Buyer2, p.ID)
	ch.WantKind(t, err, ch.ErrPermission)
	if posts := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Buyer2, p.SellerID, 1, 20))); !hasPost(posts.Items, p.ID) {
		t.Errorf("被店家封鎖的買家仍要讀得到店的公開貼文：%+v", posts.Items)
	}
	other := newPost(ctx, t, d, ch.Seller2)
	ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, other.ID, "別家店 "+ch.Nonce())))
}

func storeBlockListAndUnblock(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	if changed := ch.Must(t, ch.Ret(d.SellerBlockAuthor(ctx, ch.Seller, c.ID))); !changed {
		t.Errorf("店家封鎖 changed = false")
	}
	if changed := ch.Must(t, ch.Ret(d.SellerBlockAuthor(ctx, ch.Seller, c.ID))); changed {
		t.Errorf("重複封鎖 changed = true，封鎖要冪等")
	}
	list := ch.Must(t, ch.Ret(d.SellerBlockedBuyers(ctx, ch.Seller)))
	if len(list) == 0 {
		t.Fatalf("封鎖名單是空的")
	}
	_, err := d.SellerBlockedBuyers(ctx, ch.Buyer)
	wantOwnerDenied(t, err)
	for _, b := range list {
		if !b.Deleted {
			ch.Must(t, ch.Ret(d.SellerUnblockBuyer(ctx, ch.Seller, b.BuyerID)))
		}
	}
	ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "解除後 "+ch.Nonce())))
}
