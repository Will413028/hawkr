package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAvatarDTOs_OutwardJSONKeyIsFlatAvatarKey 釘住 G1 的收斂結果：每一個對外吐
// 買家/使用者頭像的 DTO，wire 上都必須是扁平的 `avatarKey`／`avatarUrl`，不得帶
// 主體前綴（49 §6.6 preamble：禁止對外透傳 proto 前綴）。
//
// 為什麼要逐 struct 釘：這些欄位散在 7 個檔案、由 5 個不同的 proto 家族餵養，
// 「改了一處其他應該也對」正是 #1226 翻車的形狀。wire key 完全由 json tag 決定，
// 所以 json.Marshal 一個樣本值就足以抓到任何一處靜默回退。
//
// 每個樣本值都刻意把兩個欄位都填非空——這些 DTO 多數帶 `,omitempty`，留空會讓
// 欄位整個消失，斷言就會因為錯誤的理由而通過。
func TestAvatarDTOs_OutwardJSONKeyIsFlatAvatarKey(t *testing.T) {
	const key = "public/buyer_avatar/2623900520/a.png"
	const url = "https://cdn.test/i/original/public/buyer_avatar/2623900520/a.png"

	cases := []struct {
		name string
		v    any
		// 同物件內共存的其他媒體欄（若有）——順帶證明扁平化沒有跟它們撞名。
		alsoWant []string
	}{
		{"OrderDetailResponse", OrderDetailResponse{
			BuyerAvatarKey: key, BuyerAvatarUrl: url,
			SellerLogoKey: "public/user_avatar/1/logo.png", SellerLogoUrl: url,
		}, []string{`"logoKey"`, `"logoUrl"`}},
		{"OrderSummaryResponse", OrderSummaryResponse{
			BuyerAvatarKey: key, BuyerAvatarUrl: url,
			SellerLogoKey: "public/user_avatar/1/logo.png", SellerLogoUrl: url,
		}, []string{`"logoKey"`, `"logoUrl"`}},
		{"AfterSaleCaseSummaryResponse", AfterSaleCaseSummaryResponse{
			BuyerAvatarKey: key, BuyerAvatarUrl: url,
		}, nil},
		{"AppealSummaryResponse", AppealSummaryResponse{
			BuyerAvatarKey: key, BuyerAvatarUrl: url,
			SellerAvatarKey: "public/user_avatar/1/logo.png", SellerAvatarUrl: url,
		}, []string{`"logoKey"`, `"logoUrl"`}},
		{"AdminContextResponse", AdminContextResponse{
			BuyerAvatarKey: key, BuyerAvatarUrl: url,
			SellerAvatarKey: "public/user_avatar/1/logo.png", SellerAvatarUrl: url,
		}, []string{`"logoKey"`, `"logoUrl"`}},
		{"ReviewListItem", ReviewListItem{
			UserAvatarKey: key, UserAvatar: url,
			ImageKeys: []string{"public/review/1.png"}, Images: []string{url},
		}, []string{`"imageKeys"`, `"imageUrls"`}},
		{"ReviewDetail", ReviewDetail{
			UserAvatarKey: key, UserAvatar: url,
		}, nil},
		{"PublicSellerReviewItem", PublicSellerReviewItem{
			UserAvatarKey: key, UserAvatar: url,
			ItemImageKey: "public/product/1.png", ItemImageURL: url,
		}, []string{`"productImageKey"`, `"productImageUrl"`}},
		{"OrderReviewRecord", OrderReviewRecord{
			BuyerAvatarKey: key, BuyerAvatar: url,
			ItemImageKey: "public/product/1.png", ItemImageURL: url,
		}, []string{`"productImageKey"`, `"productImageUrl"`}},
		{"ForumRecord", ForumRecord{
			UserAvatarKey: key, UserAvatar: url,
		}, nil},
		{"commentData", commentData{
			UserAvatarKey: key, UserAvatar: url,
		}, nil},
		// seller_crm 收斂前就已經是 avatarKey/avatarUrl；一併納入
		// 才能證明「全平台同一個名字」，而不是只證明「我改過的那幾個」。
		{"BuyerLookupResponse", BuyerLookupResponse{
			AvatarKey: key, AvatarURL: url,
		}, nil},
		{"crmCustomerListItem", crmCustomerListItem{
			AvatarKey: key, AvatarURL: url,
		}, nil},
		{"crmCustomerDetail", crmCustomerDetail{
			AvatarKey: key, AvatarURL: url,
		}, nil},
		{"MatchedCustomerResp", MatchedCustomerResp{
			AvatarKey: key, AvatarURL: url,
		}, nil},
	}

	// 收斂掉的舊名一個都不准回來。
	banned := []string{
		`"buyerAvatarKey"`, `"buyerAvatarUrl"`,
		`"userAvatarKey"`, `"userAvatarUrl"`,
		`"targetAvatarKey"`, `"targetAvatarUrl"`,
		`"sellerAvatarKey"`, `"sellerAvatarUrl"`,
		`"senderAvatar"`, `"userAvatar"`, `"avatar"`,
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			got := string(b)

			for _, want := range append([]string{`"avatarKey"`, `"avatarUrl"`}, tc.alsoWant...) {
				if !strings.Contains(got, want) {
					t.Errorf("%s 缺少 %s；got %s", tc.name, want, got)
				}
			}
			for _, bad := range banned {
				if strings.Contains(got, bad) {
					t.Errorf("%s 回退到舊欄位 %s（49 §6.6：頭像欄不帶主體前綴）；got %s", tc.name, bad, got)
				}
			}
		})
	}
}
