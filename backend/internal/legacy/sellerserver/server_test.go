package sellerserver

import (
	"strings"
	"testing"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

func text(v string) *string { return &v }

func TestLifecycleStatus(t *testing.T) {
	ground, category := seller.RiskFraud, policy.Fraud
	admin := []seller.EffectiveMeasure{{Kind: seller.MeasureSuspension, Basis: seller.BasisRisk, RiskGround: &ground}}
	governance := []seller.EffectiveMeasure{{Kind: seller.MeasureSuspension, Basis: seller.BasisViolation, LeadCategory: &category}}
	for _, tc := range []struct {
		name string
		s    seller.Store
		want string
	}{
		{"營運中", seller.Store{Status: seller.Active, Sanctions: seller.NoSanctions()}, "active"},
		{"風險停權（舊的管理員停權）", seller.Store{Status: seller.Active, Sanctions: seller.SanctionedBy(admin...)}, "suspended"},
		{"認定停權（舊的違規停權）", seller.Store{Status: seller.Active, Sanctions: seller.SanctionedBy(governance...)}, "suspended_governance"},
		{"兩種並存：風險優先", seller.Store{Status: seller.Active, Sanctions: seller.SanctionedBy(append(governance[:1:1], admin...)...)}, "suspended"},
		{"等待刪除", seller.Store{Status: seller.PendingDeletion, Sanctions: seller.NoSanctions()}, "pending_deletion"},
		{"等待刪除又停權：停權優先", seller.Store{Status: seller.PendingDeletion, Sanctions: seller.SanctionedBy(admin...)}, "suspended"},
		{"已關閉（帳號的措施仍在）：已關閉優先", seller.Store{Status: seller.Closed, Sanctions: seller.SanctionedBy(admin...)}, "closed"},
		{"已關閉", seller.Store{Status: seller.Closed, Sanctions: seller.NoSanctions()}, "closed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lifecycleStatus(tc.s)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, bad := range []seller.Store{
		{}, {Status: "FROZEN"},
		{Status: seller.Active},
		{Status: seller.Active, Sanctions: seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, Basis: "WHATEVER"})},
	} {
		if got, err := lifecycleStatus(bad); err == nil {
			t.Errorf("%+v 不認得的狀態不得當成有效值：%q", bad, got)
		}
	}
}

func TestLocationDerivesFromTheTownCode(t *testing.T) {
	if city, district, postal := location(text("63000050")); city != "台北市" || district != "中正區" || postal != "100" {
		t.Errorf("%q %q %q", city, district, postal)
	}
	if city, district, postal := location(text("10014010")); city != "台東縣" || district == "" || postal == "" {
		t.Errorf("%q %q %q", city, district, postal)
	}
	for _, town := range []*string{nil, text(""), text("00000000")} {
		if city, district, postal := location(town); city != "" || district != "" || postal != "" {
			t.Errorf("沒有或未知的代碼不得有地址：%q %q %q", city, district, postal)
		}
	}
}

// 兩個能力欄位的真值：上架閘（can_manage_products）只看店家的營運狀態與停權，結帳閘（can_accept_orders）看可公開曝光。
func TestSellerMetaTruthTable(t *testing.T) {
	active := seller.Store{Status: seller.Active, Sanctions: seller.NoSanctions(), StoreID: 7, StoreCode: 2600000001, AccountID: 3, LogoKey: text("logo")}
	active.StoreName = "範例小農"
	active.TownCode = text("63000050")
	// seller 讀出的條件（Store.Conditions）；各列只改測試要的那一項。
	active.Conditions = seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true, KYCApproved: true, StoreName: true, Logo: true, TownCode: true}
	ground := seller.RiskFraud
	with := func(f func(s *seller.Store)) seller.Store { s := active; f(&s); return s }
	for _, tc := range []struct {
		name           string
		store          seller.Store
		exposable      bool
		manage, accept bool
	}{
		{"可曝光", active, true, true, true},
		{"停權", with(func(s *seller.Store) {
			s.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, Basis: seller.BasisRisk, RiskGround: &ground})
		}), false, false, false},
		{"等待刪除", with(func(s *seller.Store) { s.Status, s.Conditions.NotPendingDeletion = seller.PendingDeletion, false }), false, false, false},
		{"已關閉", with(func(s *seller.Store) { s.Status, s.Conditions.Active = seller.Closed, false; s.LogoKey = nil }), false, false, false},
		{"KYC 或 PayUni 未核准：可以先上架、不能結帳", with(func(s *seller.Store) { s.Conditions.KYCApproved = false }), false, true, false},
		{"缺 logo：可以先上架、不能結帳", with(func(s *seller.Store) { s.LogoKey, s.Conditions.Logo = nil, false }), false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := sellerMeta(seller.StoreExposure{Store: tc.store, Exposable: tc.exposable})
			if m.GetCanManageProducts() != tc.manage || m.GetCanAcceptOrders() != tc.accept {
				t.Fatalf("manage=%v accept=%v, want %v %v", m.GetCanManageProducts(), m.GetCanAcceptOrders(), tc.manage, tc.accept)
			}
		})
	}
	for _, c := range []struct {
		exposable, hasCard bool
	}{{true, true}, {false, true}, {false, false}} {
		if got := sellerMeta(seller.StoreExposure{Store: active, Exposable: c.exposable, HasPublicCard: c.hasCard}).GetHasPublicCard(); got != c.hasCard {
			t.Errorf("exposable=%v: has_public_card=%v, want %v", c.exposable, got, c.hasCard)
		}
	}
	m := sellerMeta(seller.StoreExposure{Store: active, Exposable: true})
	want := &pb.SellerMeta{
		AccountId: 3, SellerName: "範例小農", LogoKey: "logo", ZipCode: "100", PayuniMerId: "HAWK2600000001", CanAcceptOrders: true,
		CanManageProducts: true, City: "台北市", District: "中正區", StoreId: "7", StoreCode: "2600000001",
	}
	if m.GetAccountId() != want.AccountId || m.GetSellerName() != want.SellerName || m.GetLogoKey() != want.LogoKey || m.GetZipCode() != want.ZipCode ||
		m.GetPayuniMerId() != want.PayuniMerId || m.GetCity() != want.City || m.GetDistrict() != want.District || m.GetStoreId() != want.StoreId || m.GetStoreCode() != want.StoreCode {
		t.Fatalf("%v, want %v", m, want)
	}
}

func TestSellerInfoRoundsTheRatingAndFillsTheLocation(t *testing.T) {
	s := seller.Store{Status: seller.Active, Sanctions: seller.NoSanctions(), StoreID: 7, StoreCode: 11, AccountID: 3, Version: 4, LogoKey: text("l"), CoverKey: text("c")}
	s.StoreName, s.TownCode, s.AddressDetail = "店", text("63000050"), text("路一號")
	info, err := sellerInfo(s, 4.7727272727)
	if err != nil {
		t.Fatal(err)
	}
	if info.GetRating() != 4.77 || info.GetCity() != "台北市" || info.GetPostalCode() != "100" || info.GetAddressDetail() != "路一號" || info.GetVersion() != "4" ||
		info.GetNickname() != "店" || info.GetLifecycleStatus() != "active" || info.GetCoverKey() != "c" {
		t.Fatalf("%v", info)
	}
}

// 舊契約的 22 縣市：ISO 代碼與正規名稱照舊 pkg/twaddr 的表（逐筆列出，不由程式推導）；縣市代碼必須是 platform/twaddr 現行的縣市而且
// 名稱相同（臺視為台）；順序是固定的顯示順序。
func TestCitiesMatchTheLegacyTableAndTheCountyCodes(t *testing.T) {
	legacy := []struct{ name, iso string }{
		{"基隆市", "TW-KEE"}, {"台北市", "TW-TPE"}, {"新北市", "TW-NWT"}, {"桃園市", "TW-TAO"}, {"新竹市", "TW-HSZ"}, {"新竹縣", "TW-HSQ"},
		{"苗栗縣", "TW-MIA"}, {"台中市", "TW-TXG"}, {"彰化縣", "TW-CHA"}, {"南投縣", "TW-NAN"}, {"雲林縣", "TW-YUN"}, {"嘉義市", "TW-CYI"},
		{"嘉義縣", "TW-CYQ"}, {"台南市", "TW-TNN"}, {"高雄市", "TW-KHH"}, {"屏東縣", "TW-PIF"}, {"宜蘭縣", "TW-ILA"}, {"花蓮縣", "TW-HUA"},
		{"台東縣", "TW-TTT"}, {"澎湖縣", "TW-PEN"}, {"金門縣", "TW-KIN"}, {"連江縣", "TW-LIE"},
	}
	if len(cities) != len(legacy) {
		t.Fatalf("%d 個縣市，舊契約是 %d 個", len(cities), len(legacy))
	}
	counties := map[string]string{}
	for _, c := range twaddr.Counties() {
		counties[c.Code] = strings.ReplaceAll(c.Name, "臺", "台")
	}
	seen := map[string]bool{}
	for i, c := range cities {
		if c.Name != legacy[i].name || c.ISO != legacy[i].iso {
			t.Errorf("第 %d 個：%+v，舊契約 %+v", i, c, legacy[i])
		}
		if counties[c.CountyCode] != c.Name {
			t.Errorf("%s 的縣市代碼 %s 在 twaddr 是 %q", c.Name, c.CountyCode, counties[c.CountyCode])
		}
		if seen[c.CountyCode] {
			t.Errorf("縣市代碼重複：%s", c.CountyCode)
		}
		seen[c.CountyCode] = true
	}
	if len(seen) != len(counties) {
		t.Errorf("twaddr 有 %d 個縣市，對照表涵蓋 %d 個", len(counties), len(seen))
	}
}

func TestCityOfAcceptsNamesAliasesAndISOCodes(t *testing.T) {
	if c, ok := cityOf(" 臺南市 "); !ok || c.ISO != "TW-TNN" {
		t.Errorf("前後空白要修剪：%+v %v", c, ok)
	}
	for input, want := range map[string]string{
		"台北市": "TW-TPE", "臺北市": "TW-TPE", "臺南市": "TW-TNN", "tw-khh": "TW-KHH", "TW-TTT": "TW-TTT", "臺東縣": "TW-TTT", "連江縣": "TW-LIE",
	} {
		if c, ok := cityOf(input); !ok || c.ISO != want {
			t.Errorf("%q: %+v %v", input, c, ok)
		}
	}
	for _, bad := range []string{"", "火星", "TW", "TW-XXX", "台北"} {
		if c, ok := cityOf(bad); ok {
			t.Errorf("%q 不該有結果：%+v", bad, c)
		}
	}
}
