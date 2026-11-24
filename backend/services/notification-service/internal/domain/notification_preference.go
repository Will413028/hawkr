package domain

import "context"

const (
	CategoryOrderUpdate  = "order_update"
	CategoryRefundUpdate = "refund_update"
	CategorySystem       = "system"
	CategoryChat         = "chat"
	CategoryFollower     = "follower"
	CategoryPost         = "post"
	CategoryCoupon       = "coupon"
	CategoryMessage      = "message"
	CategoryRestock      = "restock" // 02 §5.13b / 12 §3.1 / #942 補貨通知（買家可關）
)

var AllCategories = []string{
	CategoryOrderUpdate,
	CategoryRefundUpdate,
	CategorySystem,
	CategoryChat,
	CategoryFollower,
	CategoryPost,
	CategoryCoupon,
	CategoryMessage,
	CategoryRestock,
}

var SellerCategories = []string{
	CategoryOrderUpdate,
	CategoryRefundUpdate,
	CategorySystem,
	CategoryChat,
	CategoryFollower,
}

var BuyerCategories = []string{
	CategoryOrderUpdate,
	CategoryRefundUpdate,
	CategorySystem,
	CategoryChat,
	CategoryCoupon,
	CategoryPost,
	CategoryMessage,
	CategoryRestock, // #942：補貨通知（買家可關；fall-open default）
}

// forcedCategoriesByUserType records categories the user is NOT allowed to disable.
// 目前沒有任何分類強制 on；機制保留給 gRPC 的 forced 欄位。
var forcedCategoriesByUserType = map[string][]string{
	"seller": {},
	"buyer":  {},
}

func CategoriesForUserType(userType string) []string {
	if userType == "buyer" {
		return BuyerCategories
	}
	return SellerCategories
}

// ForcedCategoriesFor returns categories the given user type is forced to keep enabled.
// Unknown user type falls back to seller (mirroring CategoriesForUserType).
func ForcedCategoriesFor(userType string) []string {
	if forced, ok := forcedCategoriesByUserType[userType]; ok {
		return forced
	}
	return forcedCategoriesByUserType["seller"]
}

// IsForcedCategory reports whether the (userType, category) pair is force-on
// (i.e. the user cannot disable it via the preference API).
func IsForcedCategory(userType, category string) bool {
	for _, c := range ForcedCategoriesFor(userType) {
		if c == category {
			return true
		}
	}
	return false
}

func IsValidCategory(cat string) bool {
	for _, c := range AllCategories {
		if c == cat {
			return true
		}
	}
	return false
}

type NotificationPreference struct {
	UserType string
	Category string
	UserID   int64
	Enabled  bool
}

type NotificationPreferenceRepository interface {
	Upsert(ctx context.Context, userID int64, userType, category string, enabled bool) error
	GetByUser(ctx context.Context, userID int64, userType string) ([]*NotificationPreference, error)
	GetByCategory(ctx context.Context, userID int64, userType, category string) (*NotificationPreference, error)
}
