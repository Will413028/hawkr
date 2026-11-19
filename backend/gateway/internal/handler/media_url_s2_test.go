package handler

import (
	"testing"

	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
)

// 49 §6.1：review 顯示面必須成對回 {語意}Key（裸 objectKey）＋ {語意}Url（Gateway 以
// MEDIA_BASE_URL 組的顯示 URL）。此組測試釘住「URL 值不變」＝現行 CDN 域名拼接；空 key → 空 Url。

const s2Base = "https://test-media.example.com"

// s2WantURL 組出 Image Gateway 原圖顯示 URL（BuildURL 契約 base + "/i/original/" + key，49 §6.1）
func s2WantURL(key string) string { return s2Base + "/i/original/" + key }

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
