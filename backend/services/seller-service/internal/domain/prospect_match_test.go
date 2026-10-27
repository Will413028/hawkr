package domain

import "testing"

func TestBuildProspectMatchKeys(t *testing.T) {
	tests := []struct {
		name string
		in   ProspectContactInput
		want ProspectMatchKeys
	}{
		{
			// 匯入來源實測：email 有 541 筆、電話有 48 筆是「待確認」。
			// 不排除會讓這幾百筆共用同一個鍵、在 T2 比對下塌成同一家業者。
			name: "佔位值一律視為沒有這個鍵",
			in:   ProspectContactInput{Email: "待確認", Phone: "待確認", Website: "N/A"},
			want: ProspectMatchKeys{},
		},
		{
			name: "email 去空白轉小寫",
			in:   ProspectContactInput{Email: "  Marketing@EXAMPLE.org "},
			want: ProspectMatchKeys{Email: "marketing@example.org"},
		},
		{
			name: "電話只留數字",
			in:   ProspectContactInput{Phone: "07-610-7551"},
			want: ProspectMatchKeys{Phone: "076107551"},
		},
		{
			name: "電話格式差異得到相同鍵",
			in:   ProspectContactInput{Phone: "07-6107551"},
			want: ProspectMatchKeys{Phone: "076107551"},
		},
		{
			name: "國碼 886 正規化成 0 開頭",
			in:   ProspectContactInput{Phone: "+886-2-1234-5678"},
			want: ProspectMatchKeys{Phone: "0212345678"},
		},
		{
			name: "一般官網取可註冊網域，去掉 www 與路徑",
			in:   ProspectContactInput{Website: "https://www.taiwan9.com.tw/products?a=1"},
			want: ProspectMatchKeys{Domain: "taiwan9.com.tw"},
		},
		{
			// 匯入來源 CRM1 的 454 筆連結全落在社群網域（threads.com 347、
			// facebook.com 107）。用網域當鍵會把 347 家併成一家。
			name: "社群網域絕不可成為 match_domain，改抽 handle",
			in:   ProspectContactInput{Website: "https://www.threads.com/@tree_egg_"},
			want: ProspectMatchKeys{Social: "threads:tree_egg_"},
		},
		{
			name: "facebook 頁面同樣抽 handle",
			in:   ProspectContactInput{Website: "https://www.facebook.com/hawkr.farm/"},
			want: ProspectMatchKeys{Social: "facebook:hawkr.farm"},
		},
		{
			name: "裸社群網域無 handle 時不產生任何鍵",
			in:   ProspectContactInput{Website: "https://threads.com/"},
			want: ProspectMatchKeys{},
		},
		{
			name: "短網址無法還原身分，一律丟棄",
			in:   ProspectContactInput{Website: "https://lihi.cc/abc123"},
			want: ProspectMatchKeys{},
		},
		{
			name: "social 欄位獨立提供時也走 handle 抽取",
			in:   ProspectContactInput{Social: "instagram.com/i_honey.tw"},
			want: ProspectMatchKeys{Social: "instagram:i_honey.tw"},
		},
		{
			name: "website 是一般網域、social 是社群時兩個鍵並存",
			in: ProspectContactInput{
				Website: "https://berryberry.com.tw/about/",
				Social:  "https://www.facebook.com/berryberry",
			},
			want: ProspectMatchKeys{Domain: "berryberry.com.tw", Social: "facebook:berryberry"},
		},
		{
			name: "統編保留原樣（格式由 DB CHECK 把關）",
			in:   ProspectContactInput{TaxID: "12345678"},
			want: ProspectMatchKeys{TaxID: "12345678"},
		},
		// 深層路徑與平台路由拒絕（Critical: 避免災難級 collapse）
		{
			name: "facebook.com/profile.php?id=... 特例：身分在 query 參數",
			in:   ProspectContactInput{Website: "https://www.facebook.com/profile.php?id=100012345678901"},
			want: ProspectMatchKeys{Social: "facebook:100012345678901"},
		},
		{
			name: "facebook.com/profile.php 無 id 參數時不產生鍵",
			in:   ProspectContactInput{Website: "https://facebook.com/profile.php"},
			want: ProspectMatchKeys{},
		},
		{
			name: "facebook.com/pages/SomeCompanyA/123456 深層路徑不是帳號",
			in:   ProspectContactInput{Website: "https://www.facebook.com/pages/SomeCompanyA/123456"},
			want: ProspectMatchKeys{},
		},
		{
			name: "facebook.com/pages/SomeCompanyB/987654 深層路徑不是帳號",
			in:   ProspectContactInput{Website: "https://facebook.com/pages/SomeCompanyB/987654"},
			want: ProspectMatchKeys{},
		},
		{
			name: "instagram.com/p/CxYZ123abc/ 貼文路由不是帳號",
			in:   ProspectContactInput{Website: "https://www.instagram.com/p/CxYZ123abc/"},
			want: ProspectMatchKeys{},
		},
		{
			name: "instagram.com/reel/CxAAA111/ reel 路由不是帳號",
			in:   ProspectContactInput{Website: "https://instagram.com/reel/CxAAA111/"},
			want: ProspectMatchKeys{},
		},
		{
			name: "threads.com/t/CxPost001 貼文路由不是帳號",
			in:   ProspectContactInput{Website: "https://www.threads.com/t/CxPost001"},
			want: ProspectMatchKeys{},
		},
		{
			name: "threads.net/t/CxPost002 threads.net 黑名單 + 貼文路由",
			in:   ProspectContactInput{Website: "https://threads.net/t/CxPost002"},
			want: ProspectMatchKeys{},
		},
		// 黑名單項目涵蓋測試
		{
			name: "threads.net 黑名單：有效帳號",
			in:   ProspectContactInput{Website: "https://www.threads.net/@valid_handle"},
			want: ProspectMatchKeys{Social: "threads:valid_handle"},
		},
		{
			name: "m.facebook.com 黑名單：有效帳號",
			in:   ProspectContactInput{Website: "https://m.facebook.com/someuser"},
			want: ProspectMatchKeys{Social: "facebook:someuser"},
		},
		{
			name: "fb.com 黑名單：有效帳號",
			in:   ProspectContactInput{Website: "https://www.fb.com/company_page"},
			want: ProspectMatchKeys{Social: "facebook:company_page"},
		},
		// 單一段路由詞測試（rule 2 的路由名拒絕）
		{
			name: "facebook.com/watch 單一段路由詞被黑名單拒絕",
			in:   ProspectContactInput{Website: "https://www.facebook.com/watch"},
			want: ProspectMatchKeys{},
		},
		{
			name: "instagram.com/explore 單一段路由詞被黑名單拒絕",
			in:   ProspectContactInput{Website: "https://www.instagram.com/explore"},
			want: ProspectMatchKeys{},
		},
		{
			name: "threads.com/t 單一段路由詞被黑名單拒絕",
			in:   ProspectContactInput{Website: "https://www.threads.com/t"},
			want: ProspectMatchKeys{},
		},
		{
			name: "facebook.com/Watch 黑名單大小寫無關",
			in:   ProspectContactInput{Website: "https://www.facebook.com/Watch"},
			want: ProspectMatchKeys{},
		},
		{
			name: "instagram.com/@explore @ 前綴被移除後黑名單檢查",
			in:   ProspectContactInput{Website: "https://www.instagram.com/@explore"},
			want: ProspectMatchKeys{},
		},
		{
			name: "instagram.com/watchmen_farm 合法帳號包含路由詞但不相等",
			in:   ProspectContactInput{Website: "https://www.instagram.com/watchmen_farm"},
			want: ProspectMatchKeys{Social: "instagram:watchmen_farm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildProspectMatchKeys(tt.in)
			if got != tt.want {
				t.Errorf("BuildProspectMatchKeys(%+v)\n got = %+v\nwant = %+v", tt.in, got, tt.want)
			}
		})
	}
}
