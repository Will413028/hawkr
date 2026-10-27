package domain

import (
	"testing"
	"time"
)

func TestMerchant_IsClosed(t *testing.T) {
	m := &Seller{Status: SellerStatusActive}
	if m.IsClosed() {
		t.Error("IsClosed() = true for active seller, want false")
	}

	m.Status = SellerStatusClosed
	if !m.IsClosed() {
		t.Error("IsClosed() = false for closed seller, want true")
	}
}

func TestMerchant_MarkClosed(t *testing.T) {
	now := time.Now()
	m := &Seller{}

	m.MarkClosed(now)

	if m.DeletedAt == nil || !m.DeletedAt.Equal(now) {
		t.Errorf("DeletedAt = %v, want %v", m.DeletedAt, now)
	}
	if !m.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", m.UpdatedAt, now)
	}
}

func TestMerchant_UpdateProfile(t *testing.T) {
	now := time.Now()
	m := &Seller{}

	m.UpdateProfile("Store", "Desc", "logo.png", "cover.png", "0912345678", "台北市", "中山區", "中山路1號", "104", "1990-01-01", "male", "Prize", "https://example.com", now)

	if m.StoreName != "Store" {
		t.Errorf("StoreName = %q, want %q", m.StoreName, "Store")
	}
	if m.Description != "Desc" {
		t.Errorf("Description = %q, want %q", m.Description, "Desc")
	}
	if m.Logo != "logo.png" {
		t.Errorf("Logo = %q, want %q", m.Logo, "logo.png")
	}
	if m.CoverKey != "cover.png" {
		t.Errorf("CoverKey = %q, want %q", m.CoverKey, "cover.png")
	}
	if m.ContactPhone != "0912345678" {
		t.Errorf("ContactPhone = %q, want %q", m.ContactPhone, "0912345678")
	}
	if m.City != "台北市" {
		t.Errorf("City = %q, want %q", m.City, "台北市")
	}
	if m.District != "中山區" {
		t.Errorf("District = %q, want %q", m.District, "中山區")
	}
	if m.AddressDetail != "中山路1號" {
		t.Errorf("AddressDetail = %q, want %q", m.AddressDetail, "中山路1號")
	}
	if m.PostalCode != "104" {
		t.Errorf("PostalCode = %q, want %q", m.PostalCode, "104")
	}
	if m.Prize != "Prize" {
		t.Errorf("Prize = %q, want %q", m.Prize, "Prize")
	}
	if m.WebsiteURL != "https://example.com" {
		t.Errorf("WebsiteURL = %q, want %q", m.WebsiteURL, "https://example.com")
	}
	if !m.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", m.UpdatedAt, now)
	}
}

func TestMerchant_FullAddress(t *testing.T) {
	m := &Seller{City: "台北市", District: "中山區", AddressDetail: "中山路1號"}
	want := "台北市中山區中山路1號"
	if got := m.FullAddress(); got != want {
		t.Errorf("FullAddress() = %q, want %q", got, want)
	}
}

func TestRoundRating(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"raw float average leaks into API", 4.7727272727272725, 4.77},
		{"already rounded", 4.77, 4.77},
		{"rounds half away from zero at 2-decimal boundary", 4.775, 4.78},
		{"integer", 5, 5.0},
		{"zero (no reviews)", 0, 0.0},
		{"truncates third decimal", 4.123, 4.12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RoundRating(c.in)
			if got != c.want {
				t.Errorf("RoundRating(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestCanAcceptOrders(t *testing.T) {
	cases := []struct {
		name         string
		payuniStatus string
		sellerStatus SellerStatus
		want         bool
	}{
		{"payuni active + seller active", PayuniStatusActive.String(), SellerStatusActive, true},
		{"payuni active + closed", PayuniStatusActive.String(), SellerStatusClosed, false},
		{"payuni active + suspended_governance", PayuniStatusActive.String(), SellerStatusSuspendedGovernance, false},
		{"payuni active + unknown future status fails safe", PayuniStatusActive.String(), SellerStatus("frozen"), false},
		{"payuni pending (gate passed, PayUni 未核准) + active", PayuniStatusPending.String(), SellerStatusActive, false},
		{"payuni failed + active", PayuniStatusFailed.String(), SellerStatusActive, false},
		{"payuni unregistered + active", PayuniStatusUnregistered.String(), SellerStatusActive, false},
		{"empty payuni status + active", "", SellerStatusActive, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanAcceptOrders(tc.payuniStatus, tc.sellerStatus); got != tc.want {
				t.Errorf("CanAcceptOrders(%q, %q) = %v, want %v", tc.payuniStatus, tc.sellerStatus, got, tc.want)
			}
		})
	}
}

func TestCanManageProducts(t *testing.T) {
	cases := []struct {
		name         string
		sellerStatus SellerStatus
		want         bool
	}{
		{"active", SellerStatusActive, true},
		{"suspended", SellerStatusSuspended, false},
		{"suspended_governance", SellerStatusSuspendedGovernance, false},
		{"closed", SellerStatusClosed, false},
		{"unknown future status fails safe", SellerStatus("frozen"), false},
		{"empty", SellerStatus(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanManageProducts(tc.sellerStatus); got != tc.want {
				t.Errorf("CanManageProducts(%q) = %v, want %v", tc.sellerStatus, got, tc.want)
			}
		})
	}
}

func TestMeetsCertificationPrecondition(t *testing.T) {
	cases := []struct {
		name        string
		hasPost     bool
		description string
		websiteURL  string
		want        bool
	}{
		{"has post, no description, no website", true, "", "", true},
		{"has post overrides everything else", true, "   ", "", true},
		{"no post, description+website both present", false, "自我介紹", "https://my-shop.example.com", true},
		{"no post, description only", false, "自我介紹", "", false},
		{"no post, website only", false, "", "https://my-shop.example.com", false},
		{"no post, whitespace-only description, has website", false, "   ", "https://my-shop.example.com", false},
		{"no post, has description, whitespace-only website", false, "自我介紹", "   ", false},
		{"no post, neither field set", false, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MeetsCertificationPrecondition(tc.hasPost, tc.description, tc.websiteURL); got != tc.want {
				t.Errorf("MeetsCertificationPrecondition(%v, %q, %q) = %v, want %v", tc.hasPost, tc.description, tc.websiteURL, got, tc.want)
			}
		})
	}
}
