package model

import (
	"slices"
	"strings"
	"testing"

	notificationv1 "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
)

// D4：交易必要的收件匣與 email 不可關；推播可關，只有店家的 SHIPMENT_ANOMALY 推播不可關；互動與行銷全部可關。
// 兩個受眾各自驗證：BUYER 沒有 SHIPMENT_ANOMALY，所以那個分類對 BUYER 一律不可關（不存在）。
func TestDisableableFollowsD4(t *testing.T) {
	for _, a := range Audiences() {
		for _, c := range CategoriesFor(a) {
			class, _ := ClassOf(c)
			for _, ch := range Channels() {
				var want bool
				switch class {
				case ClassTransactional, ClassSystem:
					want = ch == ChannelPush && (a != AudienceStore || c != CategoryShipmentAnomaly)
				case ClassInteraction, ClassMarketing:
					want = true
				}
				if got := Disableable(a, c, ch); got != want {
					t.Errorf("Disableable(%s, %s, %s) = %v，D4 要求 %v", a, c, ch, got, want)
				}
			}
		}
	}
}

func TestUnknownCategoryChannelOrAudienceIsNeverDisableable(t *testing.T) {
	if Disableable(AudienceBuyer, "NOT_A_CATEGORY", ChannelPush) || Disableable(AudienceBuyer, CategoryChat, "SMS") || Disableable("SELLER", CategoryChat, ChannelPush) {
		t.Error("清單外的分類、渠道或受眾不可關")
	}
}

// 受眾的分類集合：每個 D4 分類至少屬於一個受眾；買家沒有 SHIPMENT_ANOMALY，店家沒有 COUPON，所以對方的分類不可關也不算有效。
func TestCategoriesAreScopedPerAudience(t *testing.T) {
	covered := map[PrefCategory]bool{}
	for _, a := range Audiences() {
		for _, c := range CategoriesFor(a) {
			covered[c] = true
			if !ValidCategoryFor(a, c) {
				t.Errorf("%s 列出的 %s 卻不算有效", a, c)
			}
		}
	}
	for _, c := range Categories() {
		if !covered[c] {
			t.Errorf("分類 %s 不屬於任何受眾", c)
		}
	}
	if ValidCategoryFor(AudienceBuyer, CategoryShipmentAnomaly) || Disableable(AudienceBuyer, CategoryShipmentAnomaly, ChannelPush) {
		t.Error("SHIPMENT_ANOMALY 不屬於 BUYER，不能設定它的推播")
	}
	if !ValidCategoryFor(AudienceStore, CategoryShipmentAnomaly) || Disableable(AudienceStore, CategoryShipmentAnomaly, ChannelPush) {
		t.Error("STORE 的 SHIPMENT_ANOMALY 推播不可關")
	}
	if ValidCategoryFor(AudienceStore, CategoryCoupon) || !ValidCategoryFor(AudienceBuyer, CategoryCoupon) {
		t.Error("COUPON 只屬於 BUYER")
	}
	if ValidCategoryFor("SELLER", CategoryChat) || len(CategoriesFor("SELLER")) != 0 {
		t.Error("不認得的受眾沒有分類")
	}
}

// 每個分類都有大類，大類只有 D4 的三種；預設全開。
func TestEveryCategoryHasAClassAndDefaultsOn(t *testing.T) {
	seen := map[PrefCategory]bool{}
	for _, c := range Categories() {
		if seen[c] {
			t.Errorf("分類 %s 重複", c)
		}
		seen[c] = true
		class, ok := ClassOf(c)
		if !ok || !slices.Contains([]CategoryClass{ClassTransactional, ClassInteraction, ClassMarketing, ClassSystem}, class) {
			t.Errorf("分類 %s 的大類 %q 不在 D4", c, class)
		}
		for _, ch := range Channels() {
			if !DefaultEnabled(c, ch) {
				t.Errorf("(%s, %s) 預設要開", c, ch)
			}
		}
	}
	if len(Categories()) != 15 {
		t.Errorf("清單是 D4 的 14 個加 D18 的 ANNOUNCEMENT，共 15 個，現在 %d", len(Categories()))
	}
}

// D18：公告兩個受眾都有；收件匣與 email 不可關，推播可關。
func TestAnnouncementCategoryFollowsD18(t *testing.T) {
	for _, a := range Audiences() {
		if !ValidCategoryFor(a, CategoryAnnouncement) {
			t.Errorf("%s 要有 ANNOUNCEMENT", a)
		}
		if Disableable(a, CategoryAnnouncement, ChannelInbox) || !Disableable(a, CategoryAnnouncement, ChannelPush) {
			t.Errorf("%s 的公告：收件匣不可關、推播可關", a)
		}
	}
}

func TestPresentationsAreTheTenOfD6(t *testing.T) {
	if got := len(Presentations()); got != 10 {
		t.Errorf("D6 有 10 個呈現分類，現在 %d", got)
	}
}

// 契約的分類 enum 與註冊表的分類一一對應：多一個或少一個都會讓偏好 API 與收件匣篩選不一致。
func TestCategoryEnumMatchesRegistry(t *testing.T) {
	fromProto := map[PrefCategory]bool{}
	for num, name := range notificationv1.NotificationCategory_name {
		if num == int32(notificationv1.NotificationCategory_NOTIFICATION_CATEGORY_UNSPECIFIED) {
			continue
		}
		fromProto[PrefCategory(strings.TrimPrefix(name, "NOTIFICATION_CATEGORY_"))] = true
	}
	for _, c := range Categories() {
		if !fromProto[c] {
			t.Errorf("註冊表的分類 %s 不在 proto 的 NotificationCategory", c)
		}
		delete(fromProto, c)
	}
	for c := range fromProto {
		t.Errorf("proto 的分類 %s 不在註冊表", c)
	}
}
