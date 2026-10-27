package characterization

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// scenarios 是舊的 social-service 與單體都要通過的 28 條情境（情境清單的出處：A7-0 的介面盤點 interface-inventory.md 的
// REST 清單，涵蓋貼文、留言、討論區、標籤；檢舉與封鎖在 A7-1d、A7-1b）：每個情境是 func(ctx context.Context, t ch.T, d Driver)，
// 資料庫可能已經有別的資料，所以只看自己建立的那幾筆（標籤與內容帶 ch.Nonce()）。
//
// 標 skipOnLegacy 的是計畫的 SD 修正造成的刻意差異：舊的一端跳過並附 SD 編號，單體的一端（A7-2）照新行為斷言。
// 不在這裡的 SD：SD3、SD10、SD12、SD14、SD15、SD21、SD23（A7-1b、A7-1d 的範圍）、SD8、SD9（事件與推播，沒有 REST 面，
// 由 integration 測試與 A7-3 的 wire 比對）、SD11、SD16、SD24（沒有 REST 面或需要 product-service）、SD17（並發，
// 由 social 的 integration 測試）、SD20、SD22、SD25（接受，舊碼退場）。
var scenarios = []ch.Scenario[Driver]{
	{Name: "post_create_is_public", Run: postCreateIsPublic},
	{Name: "post_create_authz_and_validation", Run: postCreateAuthzAndValidation},
	{Name: "post_update_replaces_content_and_tags", Run: postUpdateReplacesContentAndTags},
	{Name: "post_other_seller_cannot_modify", Run: postOtherSellerCannotModify},
	{Name: "post_unpublish_and_republish", Run: postUnpublishAndRepublish},
	{Name: "post_delete", Run: postDelete},
	{Name: "post_lists_and_paging", Run: postListsAndPaging},
	{Name: "post_admin_hide_and_restore", Run: postAdminHideAndRestore},
	{Name: "post_like_toggle", Run: postLikeToggle},
	{Name: "post_share", Run: postShare},
	{Name: "sd1_public_detail_requires_public_post", Run: sd1PublicDetailRequiresPublicPost},
	{Name: "sd1_sd18_view_counts_once_per_viewer_per_day", Run: sd1SD18ViewCountsOncePerViewerPerDay},
	{Name: "sd18_share_counts_once_per_buyer", Run: sd18ShareCountsOncePerBuyer},
	{Name: "sd6_edit_rejects_banned_words", Run: sd6EditRejectsBannedWords},
	{Name: "sd7_hidden_post_cannot_be_edited", Run: sd7HiddenPostCannotBeEdited},
	{Name: "comment_create_list_and_validation", Run: commentCreateListAndValidation},
	{Name: "comment_seller_reply_thread", Run: commentSellerReplyThread},
	{Name: "comment_edit_by_author_only", Run: commentEditByAuthorOnly},
	{Name: "comment_delete_rules", Run: commentDeleteRules},
	{Name: "comment_like_toggle", Run: commentLikeToggle},
	{Name: "sd2_comments_follow_post_visibility", Run: sd2CommentsFollowPostVisibility},
	{Name: "sd4_reply_to_must_be_a_visible_comment_of_the_same_post", Run: sd4ReplyToMustBeVisibleCommentOfSamePost},
	{Name: "sd5_deleting_a_comment_twice_counts_once", Run: sd5DeletingCommentTwiceCountsOnce},
	{Name: "a7_placeholder_for_unavailable_top_level_comment", Run: placeholderForUnavailableTopLevelComment},
	{Name: "forum_thread", Run: forumThread},
	{Name: "forum_edit_and_delete_by_author_only", Run: forumEditAndDeleteByAuthorOnly},
	{Name: "hashtag_suggest_and_trending", Run: hashtagSuggestAndTrending},
	{Name: "sd19_tag_search_treats_wildcards_literally", Run: sd19TagSearchTreatsWildcardsLiterally},
}

// imageSeq 讓每次建立的貼文有自己的圖片參照（全數字：舊的 social-service 把 asset ID 當整數解析）。
var imageSeq atomic.Int64

func init() { imageSeq.Store(time.Now().UnixNano() % 1_000_000_000_000) }

func image() string { return strconv.FormatInt(imageSeq.Add(1), 10) }

// skipOnLegacy 在舊的一端跳過情境：舊實作的行為與計畫的修正不同。回傳 true 時情境直接結束。
func skipOnLegacy(t ch.T, d Driver, why string) bool {
	if !d.Legacy() {
		return false
	}
	if s, ok := t.(interface{ Skipf(string, ...any) }); ok {
		s.Skipf("舊的 social-service 的行為不同：%s", why)
	}
	return true
}

func newPost(ctx context.Context, t ch.T, d Driver, c ch.Caller, tags ...string) Post {
	t.Helper()
	return ch.Must(t, ch.Ret(d.CreatePost(ctx, c, PostInput{Content: "貼文 " + ch.Nonce(), Tags: tags, Image: image()})))
}

func hasPost(posts []Post, id string) bool {
	for _, p := range posts {
		if p.ID == id {
			return true
		}
	}
	return false
}

func hasComment(list []Comment, id string) bool {
	for _, c := range list {
		if c.ID == id {
			return true
		}
	}
	return false
}

func findComment(list []Comment, id string) (Comment, bool) {
	for _, c := range list {
		if c.ID == id {
			return c, true
		}
	}
	return Comment{}, false
}

func hasEntry(list []ForumEntry, id string) bool {
	for _, e := range list {
		if e.ID == id {
			return true
		}
	}
	return false
}

func findTag(tags []TagCount, name string) (TagCount, bool) {
	for _, tg := range tags {
		if tg.Name == name {
			return tg, true
		}
	}
	return TagCount{}, false
}

func wantListed(t ch.T, ps []Post, id string, listed bool, where string) {
	t.Helper()
	if hasPost(ps, id) != listed {
		t.Errorf("%s：貼文 %s 在列表裡 = %v，預期 %v", where, id, !listed, listed)
	}
}

func wantNoError(t ch.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s：%v", what, err)
	}
}

func wantError(t ch.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s：預期失敗，卻成功了", what)
	}
}

func postCreateIsPublic(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	tagA, tagB := "烘焙"+n, "bake"+n
	p := ch.Must(t, ch.Ret(d.CreatePost(ctx, ch.Seller, PostInput{Content: "新貼文 " + n, Tags: []string{tagA, tagB}, Image: image()})))
	if p.ID == "" || p.SellerID == "" {
		t.Fatalf("建立的貼文缺 ID 或店家：%+v", p)
	}
	mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if mine.Status != "PUBLISHED" || mine.Content != "新貼文 "+n || mine.HiddenReason != "" {
		t.Errorf("店主讀到的貼文 = %+v", mine)
	}
	pub := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Anonymous, p.ID)))
	if pub.Content != "新貼文 "+n || pub.SellerID != p.SellerID || len(pub.Tags) != 2 {
		t.Errorf("公開讀到的貼文 = %+v", pub)
	}
	for _, tag := range []string{tagA, tagB} {
		wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, true, "標籤 "+tag+" 的動態牆")
	}
	page := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Anonymous, p.SellerID, 1, 50)))
	wantListed(t, page.Items, p.ID, true, "店家的貼文列表")
}

func postCreateAuthzAndValidation(ctx context.Context, t ch.T, d Driver) {
	in := PostInput{Content: "內容 " + ch.Nonce(), Image: image()}
	_, err := d.CreatePost(ctx, ch.Buyer, in)
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.CreatePost(ctx, ch.Anonymous, in)
	ch.WantKind(t, err, ch.ErrUnauthenticated)
	_, err = d.CreatePost(ctx, ch.Seller, PostInput{Content: "   ", Image: image()})
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreatePost(ctx, ch.Seller, PostInput{Content: "內容", Image: image(), Tags: []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "a10", "a11"}})
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreatePost(ctx, ch.Seller, PostInput{Content: "內容", Image: image(), Tags: []string{"含 空白"}})
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreatePost(ctx, ch.Seller, PostInput{Content: "你這個白痴 " + ch.Nonce(), Image: image()})
	ch.WantKind(t, err, ch.ErrInvalid)
}

func postUpdateReplacesContentAndTags(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	keep, drop, add := "keep"+n, "drop"+n, "add"+n
	p := newPost(ctx, t, d, ch.Seller, keep, drop)
	up := ch.Must(t, ch.Ret(d.UpdatePost(ctx, ch.Seller, p.ID, PostInput{Content: "更新後 " + n, Tags: []string{keep, add}})))
	if up.Content != "更新後 "+n {
		t.Errorf("更新後的內容 = %q", up.Content)
	}
	mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if mine.Content != "更新後 "+n || len(mine.Tags) != 2 {
		t.Errorf("店主讀到的貼文 = %+v", mine)
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, add))), p.ID, true, "新標籤的動態牆")
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, keep))), p.ID, true, "保留標籤的動態牆")
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, drop))), p.ID, false, "移除標籤的動態牆")
}

func postOtherSellerCannotModify(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	_, err := d.UpdatePost(ctx, ch.Seller2, p.ID, PostInput{Content: "別人改的"})
	ch.WantKind(t, err, ch.ErrPermission)
	ch.WantKind(t, d.UnpublishPost(ctx, ch.Seller2, p.ID), ch.ErrPermission)
	ch.WantKind(t, d.PublishPost(ctx, ch.Seller2, p.ID), ch.ErrPermission)
	ch.WantKind(t, d.DeletePost(ctx, ch.Seller2, p.ID), ch.ErrPermission)
	ch.WantKind(t, d.DeletePost(ctx, ch.Buyer, p.ID), ch.ErrPermission)
	mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if mine.Content != p.Content || mine.Status != "PUBLISHED" {
		t.Errorf("被拒絕的修改不該留下痕跡：%+v", mine)
	}
}

func postUnpublishAndRepublish(ctx context.Context, t ch.T, d Driver) {
	tag := "unpub" + ch.Nonce()
	p := newPost(ctx, t, d, ch.Seller, tag)
	wantNoError(t, d.UnpublishPost(ctx, ch.Seller, p.ID), "下架")
	mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if mine.Status != "UNPUBLISHED" {
		t.Errorf("下架後店主讀到的狀態 = %q", mine.Status)
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, false, "下架後的動態牆")
	page := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Anonymous, p.SellerID, 1, 50)))
	wantListed(t, page.Items, p.ID, false, "下架後的店家貼文列表")
	wantListed(t, ch.Must(t, ch.Ret(d.ListMyPosts(ctx, ch.Seller, "published"))), p.ID, false, "下架後店主的已發布列表")
	wantNoError(t, d.PublishPost(ctx, ch.Seller, p.ID), "重新發布")
	if mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID))); mine.Status != "PUBLISHED" {
		t.Errorf("重新發布後的狀態 = %q", mine.Status)
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, true, "重新發布後的動態牆")
}

func postDelete(ctx context.Context, t ch.T, d Driver) {
	tag := "del" + ch.Nonce()
	p := newPost(ctx, t, d, ch.Seller, tag)
	wantNoError(t, d.DeletePost(ctx, ch.Seller, p.ID), "刪除")
	_, err := d.GetMyPost(ctx, ch.Seller, p.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	_, err = d.GetPost(ctx, ch.Anonymous, p.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, false, "刪除後的動態牆")
	ch.WantKind(t, d.DeletePost(ctx, ch.Seller, p.ID), ch.ErrNotFound)
}

func postListsAndPaging(ctx context.Context, t ch.T, d Driver) {
	tag := "page" + ch.Nonce()
	first := newPost(ctx, t, d, ch.Seller, tag)
	second := newPost(ctx, t, d, ch.Seller, tag)
	third := newPost(ctx, t, d, ch.Seller, tag)
	feed := ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag)))
	if len(feed) != 3 || feed[0].ID != third.ID || feed[1].ID != second.ID || feed[2].ID != first.ID {
		t.Errorf("標籤的動態牆要由新到舊三篇：%+v", feed)
	}
	mine := ch.Must(t, ch.Ret(d.ListMyPosts(ctx, ch.Seller, "")))
	for _, p := range []Post{first, second, third} {
		wantListed(t, mine, p.ID, true, "店主的貼文列表")
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListMyPosts(ctx, ch.Seller2, ""))), first.ID, false, "別的店主的貼文列表")
	p1 := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Anonymous, first.SellerID, 1, 1)))
	p2 := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Anonymous, first.SellerID, 2, 1)))
	if len(p1.Items) != 1 || len(p2.Items) != 1 || p1.Items[0].ID == p2.Items[0].ID || p1.Total < 3 || p1.Total != p2.Total {
		t.Errorf("店家貼文列表的分頁：第一頁 %+v、第二頁 %+v", p1, p2)
	}
	if p1.Items[0].ID != third.ID {
		t.Errorf("店家貼文列表第一頁的第一篇 = %s，預期最新的 %s", p1.Items[0].ID, third.ID)
	}
}

func postAdminHideAndRestore(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	tag := "hide" + n
	p := newPost(ctx, t, d, ch.Seller, tag)
	ch.WantKind(t, d.ModeratePost(ctx, ch.Buyer, p.ID, true, "不是管理員"), ch.ErrPermission)
	wantNoError(t, d.ModeratePost(ctx, ch.Admin, p.ID, true, "測試隱藏 "+n), "管理員隱藏")
	mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if mine.HiddenReason != "測試隱藏 "+n {
		t.Errorf("店主讀到的隱藏理由 = %q", mine.HiddenReason)
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, false, "隱藏後的動態牆")
	page := ch.Must(t, ch.Ret(d.ListSellerPosts(ctx, ch.Anonymous, p.SellerID, 1, 50)))
	wantListed(t, page.Items, p.ID, false, "隱藏後的店家貼文列表")
	wantListed(t, ch.Must(t, ch.Ret(d.ListMyPosts(ctx, ch.Seller, "hidden"))), p.ID, true, "店主的隱藏列表")
	tags := ch.Must(t, ch.Ret(d.SuggestTags(ctx, ch.Anonymous, tag)))
	if _, ok := findTag(tags, tag); ok {
		t.Errorf("隱藏後的貼文不該貢獻標籤建議：%+v", tags)
	}
	wantNoError(t, d.ModeratePost(ctx, ch.Admin, p.ID, false, ""), "管理員還原")
	if mine := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID))); mine.HiddenReason != "" {
		t.Errorf("還原後的隱藏理由 = %q", mine.HiddenReason)
	}
	wantListed(t, ch.Must(t, ch.Ret(d.ListPosts(ctx, ch.Anonymous, tag))), p.ID, true, "還原後的動態牆")
}

func postLikeToggle(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	if liked := ch.Must(t, ch.Ret(d.ToggleLike(ctx, ch.Buyer, p.ID))); !liked {
		t.Errorf("第一次按讚 liked = false")
	}
	for caller, want := range map[ch.Caller]bool{ch.Buyer: true, ch.Buyer2: false, ch.Anonymous: false} {
		if got := ch.Must(t, ch.Ret(d.GetPost(ctx, caller, p.ID))); got.Liked != want {
			t.Errorf("呼叫者 %d 讀到的 isLiked = %v，預期 %v", caller, got.Liked, want)
		}
	}
	if liked := ch.Must(t, ch.Ret(d.ToggleLike(ctx, ch.Buyer, p.ID))); liked {
		t.Errorf("第二次切換 liked = true")
	}
	if got := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Buyer, p.ID))); got.Liked {
		t.Errorf("取消讚之後 isLiked 仍是 true")
	}
	_, err := d.ToggleLike(ctx, ch.Anonymous, p.ID)
	ch.WantKind(t, err, ch.ErrUnauthenticated)
	wantNoError(t, d.DeletePost(ctx, ch.Seller, p.ID), "刪除貼文")
	_, err = d.ToggleLike(ctx, ch.Buyer, p.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
}

func postShare(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	if n := ch.Must(t, ch.Ret(d.Share(ctx, ch.Buyer, p.ID))); n != 1 {
		t.Errorf("第一位買家分享後的分享數 = %d", n)
	}
	if n := ch.Must(t, ch.Ret(d.Share(ctx, ch.Buyer2, p.ID))); n != 2 {
		t.Errorf("第二位買家分享後的分享數 = %d", n)
	}
	if got := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID))); got.ShareCount != 2 {
		t.Errorf("店主讀到的分享數 = %d", got.ShareCount)
	}
	_, err := d.Share(ctx, ch.Anonymous, p.ID)
	ch.WantKind(t, err, ch.ErrUnauthenticated)
}

func sd1PublicDetailRequiresPublicPost(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD1：公開單篇不過濾下架與平台隱藏") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	wantNoError(t, d.UnpublishPost(ctx, ch.Seller, p.ID), "下架")
	_, err := d.GetPost(ctx, ch.Anonymous, p.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	wantNoError(t, d.PublishPost(ctx, ch.Seller, p.ID), "重新發布")
	wantNoError(t, d.ModeratePost(ctx, ch.Admin, p.ID, true, "SD1"), "隱藏")
	_, err = d.GetPost(ctx, ch.Anonymous, p.ID)
	ch.WantKind(t, err, ch.ErrNotFound)
	if got := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID))); got.HiddenReason != "SD1" {
		t.Errorf("店主仍要讀得到被隱藏的貼文與理由：%+v", got)
	}
}

func sd1SD18ViewCountsOncePerViewerPerDay(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD1、SD18：舊的讀取就計數，每次都加一") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	wantNoError(t, d.ViewPost(ctx, ch.Buyer, p.ID), "第一次瀏覽")
	wantNoError(t, d.ViewPost(ctx, ch.Buyer, p.ID), "同一天第二次瀏覽")
	for range 2 {
		if _, err := d.GetPost(ctx, ch.Buyer, p.ID); err != nil {
			t.Fatalf("讀取貼文：%v", err)
		}
	}
	if got := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Buyer, p.ID))); got.ViewCount != 1 {
		t.Errorf("同一位買家同一天瀏覽兩次、讀取三次之後的瀏覽數 = %d，預期 1", got.ViewCount)
	}
	wantNoError(t, d.ViewPost(ctx, ch.Buyer2, p.ID), "另一位買家瀏覽")
	if got := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Buyer, p.ID))); got.ViewCount != 2 {
		t.Errorf("兩位買家瀏覽之後的瀏覽數 = %d，預期 2", got.ViewCount)
	}
}

func sd18ShareCountsOncePerBuyer(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD18：舊的每次分享都加一") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	if n := ch.Must(t, ch.Ret(d.Share(ctx, ch.Buyer, p.ID))); n != 1 {
		t.Errorf("第一次分享後的分享數 = %d", n)
	}
	if n := ch.Must(t, ch.Ret(d.Share(ctx, ch.Buyer, p.ID))); n != 1 {
		t.Errorf("同一位買家第二次分享後的分享數 = %d，預期仍是 1", n)
	}
}

func sd6EditRejectsBannedWords(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD6：舊的只在建立時檢查髒字詞典，編輯會繞過") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	_, err := d.UpdatePost(ctx, ch.Seller, p.ID, PostInput{Content: "你這個白痴 " + ch.Nonce()})
	ch.WantKind(t, err, ch.ErrInvalid)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "正常留言 "+ch.Nonce())))
	_, err = d.UpdateComment(ctx, ch.Buyer, c.ID, "你這個白痴")
	ch.WantKind(t, err, ch.ErrInvalid)
}

func sd7HiddenPostCannotBeEdited(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD7：舊的被隱藏的貼文仍能編輯") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	wantNoError(t, d.ModeratePost(ctx, ch.Admin, p.ID, true, "SD7"), "隱藏")
	_, err := d.UpdatePost(ctx, ch.Seller, p.ID, PostInput{Content: "隱藏後的編輯"})
	wantError(t, err, "編輯被隱藏的貼文")
	got := ch.Must(t, ch.Ret(d.GetMyPost(ctx, ch.Seller, p.ID)))
	if got.Content != p.Content || got.HiddenReason != "SD7" {
		t.Errorf("作者讀得到被隱藏的內容與理由，內容不能被改：%+v", got)
	}
}

func commentCreateListAndValidation(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "留言 "+n)))
	if c.ID == "" || c.Role != "BUYER" || c.ParentID != "" || c.Content != "留言 "+n || !c.IsMine {
		t.Errorf("建立的留言 = %+v", c)
	}
	own := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Seller, p.ID, "店家留言 "+n)))
	if own.Role != "SELLER" {
		t.Errorf("店家在自己貼文的留言 role = %q", own.Role)
	}
	anon := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID)))
	got, ok := findComment(anon, c.ID)
	if !ok || got.UserName == "" || got.IsMine || got.Role != "BUYER" {
		t.Errorf("匿名讀到的留言列表：%+v", anon)
	}
	if !hasComment(anon, own.ID) {
		t.Errorf("匿名讀不到店家的留言：%+v", anon)
	}
	if mine, ok := findComment(ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Buyer, p.ID))), c.ID); !ok || !mine.IsMine {
		t.Errorf("作者讀自己的留言 isMine = %v", mine.IsMine)
	}
	_, err := d.CreateComment(ctx, ch.Anonymous, p.ID, "x")
	ch.WantKind(t, err, ch.ErrUnauthenticated)
	_, err = d.CreateComment(ctx, ch.Buyer, p.ID, "  ")
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreateComment(ctx, ch.Buyer, p.ID, "你這個白痴")
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreateComment(ctx, ch.Buyer, "999999999999", "留言")
	ch.WantKind(t, err, ch.ErrNotFound)
	wantNoError(t, d.DeletePost(ctx, ch.Seller, p.ID), "刪除貼文")
	_, err = d.CreateComment(ctx, ch.Buyer, p.ID, "已刪貼文的留言")
	ch.WantKind(t, err, ch.ErrNotFound)
}

func commentSellerReplyThread(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	p := newPost(ctx, t, d, ch.Seller)
	top := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "問題 "+n)))
	r1 := ch.Must(t, ch.Ret(d.ReplyComment(ctx, ch.Seller, p.ID, top.ID, "", "回覆一 "+n)))
	if r1.ParentID != top.ID || r1.Role != "SELLER" {
		t.Errorf("店家的回覆 = %+v", r1)
	}
	r2 := ch.Must(t, ch.Ret(d.ReplyComment(ctx, ch.Seller, p.ID, top.ID, r1.ID, "回覆二 "+n)))
	if r2.ParentID != top.ID || r2.ReplyToID != r1.ID {
		t.Errorf("回覆的回覆要掛在頂層留言底下並記住被回覆的那則：%+v", r2)
	}
	r3 := ch.Must(t, ch.Ret(d.ReplyComment(ctx, ch.Seller, p.ID, r1.ID, "", "對回覆再回 "+n)))
	if r3.ParentID != top.ID {
		t.Errorf("對回覆回覆時 parentId 要收斂成頂層留言：%+v", r3)
	}
	list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID)))
	got, ok := findComment(list, top.ID)
	if !ok || !hasComment(got.Replies, r1.ID) || !hasComment(got.Replies, r2.ID) {
		t.Errorf("頂層留言底下要有回覆：%+v", list)
	}
	replies := ch.Must(t, ch.Ret(d.ListReplies(ctx, ch.Anonymous, top.ID)))
	for _, r := range []Comment{r1, r2, r3} {
		if !hasComment(replies, r.ID) {
			t.Errorf("回覆列表缺 %s：%+v", r.ID, replies)
		}
	}
	_, err := d.ReplyComment(ctx, ch.Buyer, p.ID, top.ID, "", "買家回覆 "+n)
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.ReplyComment(ctx, ch.Seller2, p.ID, top.ID, "", "別家店回覆 "+n)
	ch.WantKind(t, err, ch.ErrPermission)
}

func commentEditByAuthorOnly(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "原本 "+n)))
	up := ch.Must(t, ch.Ret(d.UpdateComment(ctx, ch.Buyer, c.ID, "改過 "+n)))
	if up.Content != "改過 "+n {
		t.Errorf("編輯後的內容 = %q", up.Content)
	}
	if got, ok := findComment(ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))), c.ID); !ok || got.Content != "改過 "+n {
		t.Errorf("列表讀到的內容 = %+v", got)
	}
	_, err := d.UpdateComment(ctx, ch.Buyer2, c.ID, "別人改的")
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.UpdateComment(ctx, ch.Seller, c.ID, "店家改的")
	ch.WantKind(t, err, ch.ErrPermission)
	_, err = d.UpdateComment(ctx, ch.Buyer, c.ID, "  ")
	ch.WantKind(t, err, ch.ErrInvalid)
	wantNoError(t, d.DeleteComment(ctx, ch.Buyer, c.ID), "刪除")
	_, err = d.UpdateComment(ctx, ch.Buyer, c.ID, "刪除後編輯")
	ch.WantKind(t, err, ch.ErrNotFound)
}

func commentDeleteRules(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	mine := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "我的留言 "+ch.Nonce())))
	other := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "別人的留言 "+ch.Nonce())))
	ch.WantKind(t, d.DeleteComment(ctx, ch.Buyer2, mine.ID), ch.ErrPermission)
	ch.WantKind(t, d.DeleteComment(ctx, ch.Seller2, mine.ID), ch.ErrPermission)
	ch.WantKind(t, d.DeleteComment(ctx, ch.Anonymous, mine.ID), ch.ErrUnauthenticated)
	wantNoError(t, d.DeleteComment(ctx, ch.Buyer, mine.ID), "作者刪除自己的留言")
	wantNoError(t, d.DeleteComment(ctx, ch.Seller, other.ID), "貼文的店主刪除買家的留言")
	list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID)))
	if hasComment(list, mine.ID) || hasComment(list, other.ID) {
		t.Errorf("刪除之後留言還在列表裡：%+v", list)
	}
}

func commentLikeToggle(ctx context.Context, t ch.T, d Driver) {
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "值得讚的留言 "+ch.Nonce())))
	if liked := ch.Must(t, ch.Ret(d.ToggleCommentLike(ctx, ch.Buyer2, c.ID))); !liked {
		t.Errorf("第一次按讚 liked = false")
	}
	for caller, want := range map[ch.Caller]bool{ch.Buyer2: true, ch.Buyer: false, ch.Anonymous: false} {
		got, _ := findComment(ch.Must(t, ch.Ret(d.ListComments(ctx, caller, p.ID))), c.ID)
		if got.Liked != want {
			t.Errorf("呼叫者 %d 讀到的 isLiked = %v，預期 %v", caller, got.Liked, want)
		}
	}
	if liked := ch.Must(t, ch.Ret(d.ToggleCommentLike(ctx, ch.Buyer2, c.ID))); liked {
		t.Errorf("第二次切換 liked = true")
	}
	_, err := d.ToggleCommentLike(ctx, ch.Anonymous, c.ID)
	ch.WantKind(t, err, ch.ErrUnauthenticated)
}

func sd2CommentsFollowPostVisibility(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD2：舊的留言列表不看貼文狀態") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	c := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "留言 "+ch.Nonce())))
	wantNoError(t, d.UnpublishPost(ctx, ch.Seller, p.ID), "下架")
	unreadable := func(when string) {
		t.Helper()
		list, err := d.ListComments(ctx, ch.Anonymous, p.ID)
		if err != nil {
			ch.WantKind(t, err, ch.ErrNotFound)
			return
		}
		if hasComment(list, c.ID) {
			t.Errorf("%s：留言不該讀得到：%+v", when, list)
		}
	}
	unreadable("貼文下架")
	wantNoError(t, d.PublishPost(ctx, ch.Seller, p.ID), "重新發布")
	if !hasComment(ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))), c.ID) {
		t.Errorf("重新發布之後留言要讀得到")
	}
	wantNoError(t, d.ModeratePost(ctx, ch.Admin, p.ID, true, "SD2"), "隱藏")
	unreadable("貼文被隱藏")
}

func sd4ReplyToMustBeVisibleCommentOfSamePost(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD4：舊的 replyToId 不驗證，可以指向別篇或已刪除的留言") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	other := newPost(ctx, t, d, ch.Seller)
	top := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "頂層 "+ch.Nonce())))
	elsewhere := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, other.ID, "別篇 "+ch.Nonce())))
	gone := ch.Must(t, ch.Ret(d.ReplyComment(ctx, ch.Seller, p.ID, top.ID, "", "會被刪除的回覆 "+ch.Nonce())))
	wantNoError(t, d.DeleteComment(ctx, ch.Seller, gone.ID), "刪除回覆")
	for name, target := range map[string]string{"不存在": "999999999999", "別篇的留言": elsewhere.ID, "已刪除的回覆": gone.ID} {
		_, err := d.ReplyComment(ctx, ch.Seller, p.ID, top.ID, target, "回覆 "+ch.Nonce())
		wantError(t, err, "replyToId 指向"+name)
	}
}

func sd5DeletingCommentTwiceCountsOnce(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD5、SD17：舊的留言數由本機沒有的 RabbitMQ 非同步更新，重複刪除會重複扣") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	keep := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "留著 "+ch.Nonce())))
	gone := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer2, p.ID, "刪掉 "+ch.Nonce())))
	if got := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Anonymous, p.ID))); got.CommentCount != 2 {
		t.Errorf("兩則留言之後的留言數 = %d", got.CommentCount)
	}
	wantNoError(t, d.DeleteComment(ctx, ch.Buyer2, gone.ID), "第一次刪除")
	if err := d.DeleteComment(ctx, ch.Buyer2, gone.ID); err != nil {
		ch.WantKind(t, err, ch.ErrNotFound)
	}
	if got := ch.Must(t, ch.Ret(d.GetPost(ctx, ch.Anonymous, p.ID))); got.CommentCount != 1 {
		t.Errorf("連刪兩次只扣一次，留言數 = %d，預期 1", got.CommentCount)
	}
	if !hasComment(ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))), keep.ID) {
		t.Errorf("沒刪的留言要還在")
	}
}

func placeholderForUnavailableTopLevelComment(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "A7-1a 設計 §2.3（2026-10-23）：舊的頂層留言刪除時底下的回覆一起不見") {
		return
	}
	p := newPost(ctx, t, d, ch.Seller)
	top := ch.Must(t, ch.Ret(d.CreateComment(ctx, ch.Buyer, p.ID, "頂層 "+ch.Nonce())))
	reply := ch.Must(t, ch.Ret(d.ReplyComment(ctx, ch.Seller, p.ID, top.ID, "", "店家回覆 "+ch.Nonce())))
	wantNoError(t, d.DeleteComment(ctx, ch.Buyer, top.ID), "刪除頂層留言")
	list := ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID)))
	got, ok := findComment(list, top.ID)
	if !ok || got.State != "DELETED" || got.Content != "" || got.UserName != "" || !hasComment(got.Replies, reply.ID) {
		t.Errorf("有可見回覆的已刪除頂層留言要留佔位項目並保留回覆：%+v", list)
	}
	wantNoError(t, d.DeleteComment(ctx, ch.Seller, reply.ID), "刪除回覆")
	if hasComment(ch.Must(t, ch.Ret(d.ListComments(ctx, ch.Anonymous, p.ID))), top.ID) {
		t.Errorf("沒有可見回覆時，已刪除的頂層留言整則不出現")
	}
}

func forumThread(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	store := newPost(ctx, t, d, ch.Seller).SellerID
	e := ch.Must(t, ch.Ret(d.CreateForumEntry(ctx, ch.Buyer, store, "請問 "+n)))
	if e.ID == "" || e.SellerID != store || e.ParentID != "" || !e.IsMine {
		t.Errorf("建立的討論區留言 = %+v", e)
	}
	anon := ch.Must(t, ch.Ret(d.ListForum(ctx, ch.Anonymous, store)))
	if !hasEntry(anon, e.ID) {
		t.Fatalf("討論區列表缺剛建立的留言：%+v", anon)
	}
	for _, got := range anon {
		if got.ID == e.ID && (got.IsMine || got.UserName == "") {
			t.Errorf("匿名讀到的討論區留言 = %+v", got)
		}
	}
	owner := ch.Must(t, ch.Ret(d.ReplyForumEntry(ctx, ch.Seller, e.ID, "店家回覆 "+n)))
	other := ch.Must(t, ch.Ret(d.ReplyForumEntry(ctx, ch.Buyer2, e.ID, "路人回覆 "+n)))
	if owner.ParentID != e.ID || other.ParentID != e.ID {
		t.Errorf("回覆的 parentId 要是原留言：%+v、%+v", owner, other)
	}
	replies := ch.Must(t, ch.Ret(d.ListForumReplies(ctx, ch.Anonymous, e.ID)))
	if !hasEntry(replies, owner.ID) || !hasEntry(replies, other.ID) {
		t.Errorf("回覆列表：%+v", replies)
	}
	for _, got := range ch.Must(t, ch.Ret(d.ListForum(ctx, ch.Anonymous, store))) {
		if got.ID == e.ID && got.ReplyCount != 2 {
			t.Errorf("回覆數 = %d", got.ReplyCount)
		}
		if got.ParentID != "" {
			t.Errorf("頂層列表不該出現回覆：%+v", got)
		}
	}
	_, err := d.CreateForumEntry(ctx, ch.Buyer, store, "   ")
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreateForumEntry(ctx, ch.Buyer, store, "你這個白痴")
	ch.WantKind(t, err, ch.ErrInvalid)
	_, err = d.CreateForumEntry(ctx, ch.Anonymous, store, "x")
	ch.WantKind(t, err, ch.ErrUnauthenticated)
}

func forumEditAndDeleteByAuthorOnly(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	store := newPost(ctx, t, d, ch.Seller).SellerID
	e := ch.Must(t, ch.Ret(d.CreateForumEntry(ctx, ch.Buyer, store, "原本 "+n)))
	up := ch.Must(t, ch.Ret(d.UpdateForumEntry(ctx, ch.Buyer, e.ID, "改過 "+n)))
	if up.Content != "改過 "+n {
		t.Errorf("編輯後的內容 = %q", up.Content)
	}
	_, err := d.UpdateForumEntry(ctx, ch.Buyer2, e.ID, "別人改的")
	ch.WantKind(t, err, ch.ErrPermission)
	ch.WantKind(t, d.DeleteForumEntry(ctx, ch.Buyer2, e.ID), ch.ErrPermission)
	wantNoError(t, d.DeleteForumEntry(ctx, ch.Buyer, e.ID), "作者刪除")
	if hasEntry(ch.Must(t, ch.Ret(d.ListForum(ctx, ch.Anonymous, store))), e.ID) {
		t.Errorf("刪除之後留言還在討論區列表")
	}
}

func hashtagSuggestAndTrending(ctx context.Context, t ch.T, d Driver) {
	n := ch.Nonce()
	cjk, latin := "烘焙"+n, "bake"+n
	p := newPost(ctx, t, d, ch.Seller, cjk, latin)
	q := newPost(ctx, t, d, ch.Seller2, latin)
	got, ok := findTag(ch.Must(t, ch.Ret(d.SuggestTags(ctx, ch.Anonymous, "烘焙"+n[:4]))), cjk)
	if !ok || got.Count != 1 {
		t.Errorf("中文標籤的建議 = %+v（找到 %v）", got, ok)
	}
	got, ok = findTag(ch.Must(t, ch.Ret(d.SuggestTags(ctx, ch.Anonymous, "bake"+n[:4]))), latin)
	if !ok || got.Count != 2 {
		t.Errorf("兩篇貼文共用的標籤建議 = %+v（找到 %v）", got, ok)
	}
	if got, ok = findTag(ch.Must(t, ch.Ret(d.TrendingTags(ctx, ch.Anonymous))), latin); !ok || got.Count != 2 {
		t.Errorf("熱門標籤 = %+v（找到 %v）", got, ok)
	}
	wantNoError(t, d.UnpublishPost(ctx, ch.Seller2, q.ID), "下架其中一篇")
	if got, ok = findTag(ch.Must(t, ch.Ret(d.SuggestTags(ctx, ch.Anonymous, "bake"+n[:4]))), latin); !ok || got.Count != 1 {
		t.Errorf("下架一篇之後的標籤建議 = %+v（找到 %v）", got, ok)
	}
	wantNoError(t, d.DeletePost(ctx, ch.Seller, p.ID), "刪除另一篇")
	if got, ok = findTag(ch.Must(t, ch.Ret(d.SuggestTags(ctx, ch.Anonymous, "烘焙"+n[:4]))), cjk); ok {
		t.Errorf("貼文刪除之後不該再建議它的標籤：%+v", got)
	}
}

func sd19TagSearchTreatsWildcardsLiterally(ctx context.Context, t ch.T, d Driver) {
	if skipOnLegacy(t, d, "SD19：舊的標籤搜尋沒有轉義 LIKE 的萬用字元，% 與 _ 會比對到所有標籤") {
		return
	}
	newPost(ctx, t, d, ch.Seller, "wild"+ch.Nonce())
	for _, q := range []string{"%", "_"} {
		tags, err := d.SuggestTags(ctx, ch.Anonymous, q)
		if err != nil {
			ch.WantKind(t, err, ch.ErrInvalid)
			continue
		}
		if len(tags) != 0 {
			t.Errorf("查詢 %q 不該比對到任何標籤：%+v", q, tags)
		}
	}
}
