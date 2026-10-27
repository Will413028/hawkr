package handler

import (
	"testing"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

const candidateMediaBase = "https://test-media.example.com"

// EXTERNAL lane：品牌圖出得來，平台身分欄位維持空。兩軸不得互相 fallback——
// 消費端若把 brandLogoUrl 讀成 logoUrl，站外業者在畫面上就變成平台商家。
func TestProcurementCandidate_ExternalCarriesBrandMediaOnly(t *testing.T) {
	got := procurementCandidateItemsFromProto([]*aipb.CandidateItem{{
		CandidateId: "1", SourceId: "7504399775467307008", Lane: "EXTERNAL",
		DisplayName:   "天御梅製所",
		BrandLogoKey:  "public/seller_prospect_logo/900000001/20260920/a.png",
		BrandCoverKey: "public/seller_prospect_cover/900000001/20260920/b.png",
	}}, candidateMediaBase)

	if len(got) != 1 {
		t.Fatalf("want 1 item, got %d", len(got))
	}
	c := got[0]

	wantLogo := candidateMediaBase + "/i/original/public/seller_prospect_logo/900000001/20260920/a.png"
	if c.BrandLogoURL != wantLogo {
		t.Errorf("brandLogoUrl = %q, want %q", c.BrandLogoURL, wantLogo)
	}
	wantCover := candidateMediaBase + "/i/original/public/seller_prospect_cover/900000001/20260920/b.png"
	if c.BrandCoverURL != wantCover {
		t.Errorf("brandCoverUrl = %q, want %q", c.BrandCoverURL, wantCover)
	}

	// 平台身分那一軸必須維持空，不得被品牌圖填滿。
	if c.LogoKey != "" || c.LogoURL != "" {
		t.Errorf("EXTERNAL 不得帶平台 logo：logoKey=%q logoUrl=%q", c.LogoKey, c.LogoURL)
	}
	if c.StoreID != "" || c.SellerName != "" || c.StoreCode != "" {
		t.Errorf("EXTERNAL 不得帶平台賣家身分：%+v", c)
	}
}

// PLATFORM lane 反過來：平台 logo 照舊，品牌欄位空。加了新欄位不得改動既有那一軸。
func TestProcurementCandidate_PlatformUnaffectedByBrandFields(t *testing.T) {
	got := procurementCandidateItemsFromProto([]*aipb.CandidateItem{{
		CandidateId: "2", SourceId: "88", Lane: "PLATFORM",
		DisplayName: "平台商家", StoreId: "1001", SellerName: "平台商家",
		StoreCode: "abc123", LogoKey: "public/seller_logo/1001/20260920/c.png",
	}}, candidateMediaBase)

	c := got[0]
	want := candidateMediaBase + "/i/original/public/seller_logo/1001/20260920/c.png"
	if c.LogoURL != want {
		t.Errorf("logoUrl = %q, want %q", c.LogoURL, want)
	}
	if c.BrandLogoURL != "" || c.BrandCoverURL != "" {
		t.Errorf("PLATFORM 不該有站外品牌素材：%q %q", c.BrandLogoURL, c.BrandCoverURL)
	}
}

// 沒有圖的那 186 家：key 空 ⇒ URL 也必須空，不能組出只有域名的半截網址。
func TestProcurementCandidate_EmptyBrandKeysYieldEmptyURLs(t *testing.T) {
	got := procurementCandidateItemsFromProto([]*aipb.CandidateItem{{
		CandidateId: "3", SourceId: "9", Lane: "EXTERNAL", DisplayName: "沒有圖的業者",
	}}, candidateMediaBase)

	if got[0].BrandLogoURL != "" || got[0].BrandCoverURL != "" {
		t.Fatalf("空 key 必須組出空 URL，得到 %q / %q", got[0].BrandLogoURL, got[0].BrandCoverURL)
	}
}
