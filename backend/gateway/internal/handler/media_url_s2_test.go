package handler

import (
	"testing"

	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
)

// 49 §6.1：social/review 顯示面必須成對回 {語意}Key（裸 objectKey）＋ {語意}Url（Gateway 以
// MEDIA_BASE_URL 組的顯示 URL）。此組測試釘住「URL 值不變」＝現行 CDN 域名拼接；空 key → 空 Url。

const s2Base = "https://test-media.example.com"

// s2WantURL 組出 Image Gateway 原圖顯示 URL（BuildURL 契約 base + "/i/original/" + key，49 §6.1）
func s2WantURL(key string) string { return s2Base + "/i/original/" + key }

func TestToPostListItems_ImageKeysAndURLsPaired(t *testing.T) {
	const key = "post/7114/20260717/abc.jpg"
	items := toPostListItems(s2Base, []*socialpb.PostListItem{
		{Id: "1", ImageKey: key},
		{Id: "2", ImageKey: ""}, // 空 key
	})
	if got := items[0].ImageKeys; len(got) != 1 || got[0] != key {
		t.Errorf("ImageKeys = %v, want [%q]", got, key)
	}
	if want := s2WantURL(key); len(items[0].Images) != 1 || items[0].Images[0] != want {
		t.Errorf("Images = %v, want [%q]", items[0].Images, want)
	}
	// 無圖：必須是空陣列而非 nil——欄位沒有 omitempty，nil 會序列化成 null，
	// 前端就得為 null / [] 各寫一次判斷。
	if items[1].ImageKeys == nil || len(items[1].ImageKeys) != 0 {
		t.Errorf("empty key must yield [] not nil/non-empty, got %#v", items[1].ImageKeys)
	}
	if items[1].Images == nil || len(items[1].Images) != 0 {
		t.Errorf("empty key must yield [] not nil/non-empty, got %#v", items[1].Images)
	}
}

func TestToCommentData_AvatarKeyAndURLPaired(t *testing.T) {
	const key = "seller_avatars/123.jpg"
	// 內部 objectKey → 組 URL
	cd := toCommentData(s2Base, &socialpb.CommentInfo{Id: "1", UserAvatarKey: key})
	if cd.UserAvatarKey != key {
		t.Errorf("UserAvatarKey = %q, want %q", cd.UserAvatarKey, key)
	}
	if want := s2WantURL(key); cd.UserAvatar != want {
		t.Errorf("UserAvatar = %q, want %q", cd.UserAvatar, want)
	}
	// Google 外部頭像（http 開頭）→ passthrough，不重複拼域名
	const ext = "https://lh3.googleusercontent.com/a/x.jpg"
	cd2 := toCommentData(s2Base, &socialpb.CommentInfo{Id: "2", UserAvatarKey: ext})
	if cd2.UserAvatar != ext {
		t.Errorf("external avatar must passthrough, got %q", cd2.UserAvatar)
	}
	// 空頭像 → 空
	cd3 := toCommentData(s2Base, &socialpb.CommentInfo{Id: "3", UserAvatarKey: ""})
	if cd3.UserAvatar != "" || cd3.UserAvatarKey != "" {
		t.Errorf("empty avatar must yield empty, got key=%q url=%q", cd3.UserAvatarKey, cd3.UserAvatar)
	}
}

func TestToOrderReviewRecords_ImageKeysAndURLsPaired(t *testing.T) {
	const itemKey = "products/abc.jpg"
	const img0 = "reviews/r1.jpg"
	recs := toOrderReviewRecords(s2Base, []*reviewpb.OrderReviewInfo{
		{Id: "1", ItemImageKey: itemKey, ImageKeys: []string{img0}},
		{Id: "2", ItemImageKey: "", ImageKeys: nil}, // 空
	})
	r0 := recs[0]
	if r0.ItemImageKey != itemKey {
		t.Errorf("ItemImageKey = %q, want %q", r0.ItemImageKey, itemKey)
	}
	if want := s2WantURL(itemKey); r0.ItemImageURL != want {
		t.Errorf("ItemImageURL = %q, want %q", r0.ItemImageURL, want)
	}
	if len(r0.ImageKeys) != 1 || r0.ImageKeys[0] != img0 {
		t.Errorf("ImageKeys = %v, want [%q]", r0.ImageKeys, img0)
	}
	if want := s2WantURL(img0); len(r0.ImageURLs) != 1 || r0.ImageURLs[0] != want {
		t.Errorf("ImageURLs = %v, want [%q]", r0.ImageURLs, want)
	}
	r1 := recs[1]
	if r1.ItemImageKey != "" || r1.ItemImageURL != "" {
		t.Errorf("empty item image must yield empty, got key=%q url=%q", r1.ItemImageKey, r1.ItemImageURL)
	}
	if r1.ImageURLs != nil {
		t.Errorf("empty image keys must yield nil imageUrls, got %v", r1.ImageURLs)
	}
}

func TestToReviewListItem_ImagesKeyAndURLPaired(t *testing.T) {
	const img = "reviews/x.jpg"
	li := toReviewListItem(s2Base, &reviewpb.ReviewInfo{Id: "1", ImageKeys: []string{img}})
	if len(li.ImageKeys) != 1 || li.ImageKeys[0] != img {
		t.Errorf("ImageKeys = %v, want [%q]", li.ImageKeys, img)
	}
	if want := s2WantURL(img); len(li.Images) != 1 || li.Images[0] != want {
		t.Errorf("Images = %v, want [%q]", li.Images, want)
	}
}
