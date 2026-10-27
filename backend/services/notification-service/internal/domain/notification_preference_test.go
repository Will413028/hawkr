package domain

import "testing"

func TestCategoriesForUserType_Buyer(t *testing.T) {
	cats := CategoriesForUserType("buyer")
	if len(cats) != len(BuyerCategories) {
		t.Fatalf("len = %d, want %d", len(cats), len(BuyerCategories))
	}

	has := make(map[string]bool)
	for _, c := range cats {
		has[c] = true
	}

	if !has[CategoryCoupon] {
		t.Error("buyer categories missing coupon")
	}
	if !has[CategoryMessage] {
		t.Error("buyer categories missing message")
	}
	if has[CategoryFollower] {
		t.Error("buyer categories should not contain follower")
	}
}

func TestCategoriesForUserType_Merchant(t *testing.T) {
	cats := CategoriesForUserType("seller")
	if len(cats) != len(SellerCategories) {
		t.Fatalf("len = %d, want %d", len(cats), len(SellerCategories))
	}

	has := make(map[string]bool)
	for _, c := range cats {
		has[c] = true
	}

	if !has[CategoryFollower] {
		t.Error("seller categories missing follower")
	}
	if has[CategoryCoupon] {
		t.Error("seller categories should not contain coupon")
	}
}

func TestCategoriesForUserType_UnknownFallsToMerchant(t *testing.T) {
	cats := CategoriesForUserType("unknown")
	if len(cats) != len(SellerCategories) {
		t.Errorf("unknown user type: len = %d, want %d (seller fallback)", len(cats), len(SellerCategories))
	}
}

func TestIsValidCategory(t *testing.T) {
	for _, cat := range AllCategories {
		if !IsValidCategory(cat) {
			t.Errorf("IsValidCategory(%q) = false, want true", cat)
		}
	}
	if IsValidCategory("nonexistent") {
		t.Error("IsValidCategory(nonexistent) = true, want false")
	}
}

func TestAllCategoriesContainsBuyerAndMerchant(t *testing.T) {
	all := make(map[string]bool)
	for _, c := range AllCategories {
		all[c] = true
	}

	for _, c := range BuyerCategories {
		if !all[c] {
			t.Errorf("AllCategories missing buyer category %q", c)
		}
	}
	for _, c := range SellerCategories {
		if !all[c] {
			t.Errorf("AllCategories missing seller category %q", c)
		}
	}
}
