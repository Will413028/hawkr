package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

func TestGetPreferences_MerchantDefaults(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{})

	prefs, err := uc.GetPreferences(context.Background(), 1, "seller")
	if err != nil {
		t.Fatalf("GetPreferences error: %v", err)
	}
	if len(prefs) != len(domain.SellerCategories) {
		t.Fatalf("len(prefs) = %d, want %d", len(prefs), len(domain.SellerCategories))
	}
	for i, p := range prefs {
		if p.Category != domain.SellerCategories[i] {
			t.Errorf("prefs[%d].Category = %q, want %q", i, p.Category, domain.SellerCategories[i])
		}
		if !p.Enabled {
			t.Errorf("prefs[%d].Enabled = false, want true (default)", i)
		}
		if p.Title == "" {
			t.Errorf("prefs[%d].Title is empty", i)
		}
	}
}

func TestGetPreferences_BuyerDefaults(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{})

	prefs, err := uc.GetPreferences(context.Background(), 1, "buyer")
	if err != nil {
		t.Fatalf("GetPreferences error: %v", err)
	}
	if len(prefs) != len(domain.BuyerCategories) {
		t.Fatalf("len(prefs) = %d, want %d", len(prefs), len(domain.BuyerCategories))
	}
	for i, p := range prefs {
		if p.Category != domain.BuyerCategories[i] {
			t.Errorf("prefs[%d].Category = %q, want %q", i, p.Category, domain.BuyerCategories[i])
		}
		if !p.Enabled {
			t.Errorf("prefs[%d].Enabled = false, want true (default)", i)
		}
	}
}

func TestGetPreferences_BuyerHasCouponAndMessage(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{})

	prefs, err := uc.GetPreferences(context.Background(), 1, "buyer")
	if err != nil {
		t.Fatalf("GetPreferences error: %v", err)
	}

	categories := make(map[string]bool)
	for _, p := range prefs {
		categories[p.Category] = true
	}

	if !categories["coupon"] {
		t.Error("buyer prefs missing 'coupon' category")
	}
	if !categories["message"] {
		t.Error("buyer prefs missing 'message' category")
	}
	if categories["follower"] {
		t.Error("buyer prefs should not contain 'follower' category")
	}
}

func TestGetPreferences_MerchantHasFollowerNotCoupon(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{})

	prefs, err := uc.GetPreferences(context.Background(), 1, "seller")
	if err != nil {
		t.Fatalf("GetPreferences error: %v", err)
	}

	categories := make(map[string]bool)
	for _, p := range prefs {
		categories[p.Category] = true
	}

	if !categories["follower"] {
		t.Error("seller prefs missing 'follower' category")
	}
	if categories["coupon"] {
		t.Error("seller prefs should not contain 'coupon' category")
	}
	if categories["message"] {
		t.Error("seller prefs should not contain 'message' category")
	}
}

func TestGetPreferences_WithExistingRecords(t *testing.T) {
	repo := &mockPrefRepo{
		prefs: []*domain.NotificationPreference{
			{UserID: 1, UserType: "buyer", Category: "coupon", Enabled: false},
			{UserID: 1, UserType: "buyer", Category: "chat", Enabled: false},
		},
	}
	uc := NewNotificationPreferenceUseCase(repo)

	prefs, err := uc.GetPreferences(context.Background(), 1, "buyer")
	if err != nil {
		t.Fatalf("GetPreferences error: %v", err)
	}

	for _, p := range prefs {
		switch p.Category {
		case "coupon", "chat":
			if p.Enabled {
				t.Errorf("%s should be disabled", p.Category)
			}
		default:
			if !p.Enabled {
				t.Errorf("%s should be enabled (default)", p.Category)
			}
		}
	}
}

func TestGetPreferences_BuyerTitlesDifferFromMerchant(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{})

	buyerPrefs, _ := uc.GetPreferences(context.Background(), 1, "buyer")
	sellerPrefs, _ := uc.GetPreferences(context.Background(), 1, "seller")

	buyerDesc := make(map[string]string)
	for _, p := range buyerPrefs {
		buyerDesc[p.Category] = p.Description
	}
	sellerDesc := make(map[string]string)
	for _, p := range sellerPrefs {
		sellerDesc[p.Category] = p.Description
	}

	// order_update should have different descriptions
	if buyerDesc["order_update"] == sellerDesc["order_update"] {
		t.Error("buyer and seller order_update descriptions should differ")
	}
}

func TestCheckCategoryEnabled_NoRecord_DefaultsToTrue(t *testing.T) {
	uc := NewNotificationPreferenceUseCase(&mockPrefRepo{prefByCat: nil})
	enabled, err := uc.CheckCategoryEnabled(context.Background(), 1, "seller", "follower")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true when no preference record exists")
	}
}

func TestCheckCategoryEnabled_EnabledRecord_ReturnsTrue(t *testing.T) {
	repo := &mockPrefRepo{
		prefByCat: &domain.NotificationPreference{Category: "follower", Enabled: true},
	}
	uc := NewNotificationPreferenceUseCase(repo)
	enabled, err := uc.CheckCategoryEnabled(context.Background(), 1, "seller", "follower")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true")
	}
}

func TestCheckCategoryEnabled_DisabledRecord_ReturnsFalse(t *testing.T) {
	repo := &mockPrefRepo{
		prefByCat: &domain.NotificationPreference{Category: "follower", Enabled: false},
	}
	uc := NewNotificationPreferenceUseCase(repo)
	enabled, err := uc.CheckCategoryEnabled(context.Background(), 1, "seller", "follower")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false when preference is disabled")
	}
}

func TestCheckCategoryEnabled_DBError_FailOpen(t *testing.T) {
	repo := &mockPrefRepo{getCatErr: errors.New("db timeout")}
	uc := NewNotificationPreferenceUseCase(repo)
	enabled, err := uc.CheckCategoryEnabled(context.Background(), 1, "seller", "follower")
	if err == nil {
		t.Fatal("expected error to be returned")
	}
	if !enabled {
		t.Error("expected fail-open (enabled=true) on DB error")
	}
}
