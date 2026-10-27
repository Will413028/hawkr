package model

import (
	"slices"
	"testing"
)

func TestContactMatchKey(t *testing.T) {
	tests := []struct {
		name     string
		contacts []ContactPoint
		want     []string
	}{
		{
			// 匯入來源實測：email 有 541 筆、電話有 48 筆是「待確認」。不排除會讓這幾百筆共用同一個鍵，被比對成同一家業者。
			name:     "佔位值一律視為沒有這個鍵",
			contacts: []ContactPoint{{Kind: ContactEmail, Value: "待確認"}, {Kind: ContactPhone, Value: "待確認"}, {Kind: ContactWebsite, Value: "N/A"}},
			want:     []string{},
		},
		{
			name:     "email 去空白轉小寫",
			contacts: []ContactPoint{{Kind: ContactEmail, Value: "  Marketing@EXAMPLE.org "}},
			want:     []string{"EMAIL:marketing@example.org"},
		},
		{
			name:     "電話只留數字",
			contacts: []ContactPoint{{Kind: ContactPhone, Value: "07-610-7551"}},
			want:     []string{"PHONE:076107551"},
		},
		{
			name:     "電話格式差異得到相同鍵",
			contacts: []ContactPoint{{Kind: ContactPhone, Value: "07-6107551"}},
			want:     []string{"PHONE:076107551"},
		},
		{
			name:     "國碼 886 正規化成 0 開頭",
			contacts: []ContactPoint{{Kind: ContactPhone, Value: "+886-2-1234-5678"}},
			want:     []string{"PHONE:0212345678"},
		},
		{
			name:     "一般官網取可註冊網域，去掉 www 與路徑",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.taiwan9.com.tw/products?a=1"}},
			want:     []string{"DOMAIN:taiwan9.com.tw"},
		},
		{
			// 匯入來源 CRM1 的 454 筆連結全落在社群網域（threads.com 347、facebook.com 107）。用網域當鍵會把 347 家併成一家。
			name:     "社群網域絕不可成為 match_domain，改抽 handle",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.threads.com/@tree_egg_"}},
			want:     []string{"SOCIAL:threads:tree_egg_"},
		},
		{
			name:     "facebook 頁面同樣抽 handle",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.facebook.com/hawkr.farm/"}},
			want:     []string{"SOCIAL:facebook:hawkr.farm"},
		},
		{
			name:     "裸社群網域無 handle 時不產生任何鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://threads.com/"}},
			want:     []string{},
		},
		{
			name:     "短網址無法還原身分，一律丟棄",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://lihi.cc/abc123"}},
			want:     []string{},
		},
		{
			name:     "social 欄位獨立提供時也走 handle 抽取",
			contacts: []ContactPoint{{Kind: ContactSocial, Value: "instagram.com/i_honey.tw"}},
			want:     []string{"SOCIAL:instagram:i_honey.tw"},
		},
		{
			name:     "website 是一般網域、social 是社群時兩個鍵並存",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://berryberry.com.tw/about/"}, {Kind: ContactSocial, Value: "https://www.facebook.com/berryberry"}},
			want:     []string{"DOMAIN:berryberry.com.tw", "SOCIAL:facebook:berryberry"},
		},
		{
			name:     "facebook.com/profile.php?id=... 特例：身分在 query 參數",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.facebook.com/profile.php?id=100012345678901"}},
			want:     []string{"SOCIAL:facebook:100012345678901"},
		},
		{
			name:     "facebook.com/profile.php 無 id 參數時不產生鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://facebook.com/profile.php"}},
			want:     []string{},
		},
		{
			name:     "facebook.com/pages/SomeCompanyA/123456 深層路徑不是帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.facebook.com/pages/SomeCompanyA/123456"}},
			want:     []string{},
		},
		{
			name:     "facebook.com/pages/SomeCompanyB/987654 深層路徑不是帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://facebook.com/pages/SomeCompanyB/987654"}},
			want:     []string{},
		},
		{
			name:     "instagram.com/p/CxYZ123abc/ 貼文路由不是帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.instagram.com/p/CxYZ123abc/"}},
			want:     []string{},
		},
		{
			name:     "instagram.com/reel/CxAAA111/ reel 路由不是帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://instagram.com/reel/CxAAA111/"}},
			want:     []string{},
		},
		{
			name:     "threads.com/t/CxPost001 貼文路由不是帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.threads.com/t/CxPost001"}},
			want:     []string{},
		},
		{
			name:     "threads.net/t/CxPost002 threads.net 黑名單 + 貼文路由",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://threads.net/t/CxPost002"}},
			want:     []string{},
		},
		{
			name:     "threads.net 黑名單：有效帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.threads.net/@valid_handle"}},
			want:     []string{"SOCIAL:threads:valid_handle"},
		},
		{
			name:     "m.facebook.com 黑名單：有效帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://m.facebook.com/someuser"}},
			want:     []string{"SOCIAL:facebook:someuser"},
		},
		{
			name:     "fb.com 黑名單：有效帳號",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.fb.com/company_page"}},
			want:     []string{"SOCIAL:facebook:company_page"},
		},
		{
			name:     "facebook.com/watch 單一段路由詞被黑名單拒絕",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.facebook.com/watch"}},
			want:     []string{},
		},
		{
			name:     "instagram.com/explore 單一段路由詞被黑名單拒絕",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.instagram.com/explore"}},
			want:     []string{},
		},
		{
			name:     "threads.com/t 單一段路由詞被黑名單拒絕",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.threads.com/t"}},
			want:     []string{},
		},
		{
			name:     "facebook.com/Watch 黑名單大小寫無關",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.facebook.com/Watch"}},
			want:     []string{},
		},
		{
			name:     "instagram.com/@explore @ 前綴被移除後黑名單檢查",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.instagram.com/@explore"}},
			want:     []string{},
		},
		{
			name:     "instagram.com/watchmen_farm 合法帳號包含路由詞但不相等",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.instagram.com/watchmen_farm"}},
			want:     []string{"SOCIAL:instagram:watchmen_farm"},
		},
		{
			name:     "共用平台 linktr.ee 的網域不是識別力",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://linktr.ee/farm_a"}},
			want:     []string{},
		},
		{
			name:     "共用平台 linktr.ee 的另一個頁面也沒有鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "linktr.ee/farm_b"}},
			want:     []string{},
		},
		{
			name:     "共用平台 pinkoi.com 的店面頁沒有鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://www.pinkoi.com/store/farm"}},
			want:     []string{},
		},
		{
			name:     "共用平台的子網域同樣沒有鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://farm.linktr.ee"}},
			want:     []string{},
		},
		{
			name:     "名稱裡含共用平台字樣的一般網域仍然有鍵",
			contacts: []ContactPoint{{Kind: ContactWebsite, Value: "https://notlinktr.ee.example.com/x"}},
			want:     []string{"DOMAIN:notlinktr.ee.example.com"},
		},
		{
			name:     "社群欄位的值不是社群網址時沒有鍵",
			contacts: []ContactPoint{{Kind: ContactSocial, Value: "https://berryberry.com.tw"}},
			want:     []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContactMatchKeys(tt.contacts)
			if !slices.Equal(got, slices.Sorted(slices.Values(tt.want))) {
				t.Errorf("ContactMatchKeys(%+v)\n got = %q\nwant = %q", tt.contacts, got, tt.want)
			}
		})
	}
}
