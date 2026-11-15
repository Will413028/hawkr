package characterization

import (
	"context"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// 封鎖的情境（A7-2e）：單體的語意見 A7-1b 與 block-brief.md。舊的 social-service 的封鎖是雙向、存在 Redis、不帶 kind 的 userId，
// 所以每一條在舊的一端以 skipOnLegacy 跳過並附 SD 編號（SD3 封鎖不擋寫入、SD15 封鎖過濾在分頁之後、D2 店家封鎖只擋寫入）。

func blockAuthorByCommentReferenceIsIdempotent(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD3、SD15：舊的封鎖是雙向、以 Redis 儲存、userId 不帶 kind，沒有從留言參照封鎖的入口") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	if changed := ch.Must(t, ch.Ret(d.BlockAuthor(ctx, ch.Buyer, c.ID))); !changed {
		t.Errorf("第一次封鎖 changed = false")
	}
	if changed := ch.Must(t, ch.Ret(d.BlockAuthor(ctx, ch.Buyer, c.ID))); changed {
		t.Errorf("重複封鎖 changed = true，封鎖要冪等")
	}
	// 封鎖者之後讀不到對方的頂層留言（依觀看者過濾，不留佔位）；第三人與匿名照常讀得到。
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Buyer, p.ID))); hasComment(list, c.ID) {
		t.Errorf("封鎖者讀得到被封鎖者的留言：%+v", list)
	}
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))); !hasComment(list, c.ID) {
		t.Errorf("匿名讀不到被封鎖者的留言：%+v", list)
	}
	// 作者看不到封鎖者的留言（買家之間雙向互不可見）。
	mine := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "我的留言 "+ch.Nonce())))
	if list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Buyer2, p.ID))); hasComment(list, mine.ID) {
		t.Errorf("被封鎖者讀得到封鎖者的留言：%+v", list)
	}
}

func blockByUnreadableCommentIsNotFound(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD3：舊的沒有從留言參照封鎖的入口") {
		return
	}
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
	if skipOnLegacy(t, d, "D2、SD15：舊的封鎖過濾在分頁之後才做，而且不看 kind") {
		return
	}
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
	if skipOnLegacy(t, d, "SD3：舊的封鎖不擋寫入") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "留言 "+ch.Nonce())))
	ch.Must(t, ch.Ret(d.SellerBlockAuthor(ctx, ch.Seller, c.ID)))
	// 擋寫入：被封鎖的買家在這家店留言與按讚被拒；不擋讀取。
	_, err := d.CreateComment(ctx, ch.Buyer2, p.ID, "再留言 "+ch.Nonce())
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.ToggleLike(ctx, ch.Buyer2, p.ID)
	ch.WantKind(t, err, ch.ErrPermission)
	if posts := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Buyer2, p.SellerID, 1, 20))); !hasPost(posts.Items, p.ID) {
		t.Errorf("被店家封鎖的買家仍要讀得到店的公開貼文：%+v", posts.Items)
	}
	// 其他店的貼文不受影響。
	other := newPost(ctx, t, d, ch.Seller2)
	ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, other.ID, "別家店 "+ch.Nonce())))
}

func storeBlockListAndUnblock(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD3：舊的沒有店家封鎖名單") {
		return
	}
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
	wantOwnerDenied(t, d, err)
	// 解除之後買家可以再留言。
	for _, b := range list {
		if !b.Deleted {
			ch.Must(t, ch.Ret(d.SellerUnblockBuyer(ctx, ch.Seller, b.BuyerID)))
		}
	}
	ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "解除後 "+ch.Nonce())))
}
