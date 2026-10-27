package usecase

import (
	"testing"
	"time"
)

func TestProfileCache_BuyerHitAndMiss(t *testing.T) {
	cache := NewProfileCache()

	// Miss
	if _, ok := cache.GetBuyer(42); ok {
		t.Error("cache should miss on empty")
	}

	// Set + Hit
	cache.SetBuyer(42, BuyerProfileInfo{Nickname: "Alice", Avatar: "a.jpg"})
	got, ok := cache.GetBuyer(42)
	if !ok {
		t.Fatal("cache should hit after Set")
	}
	if got.Nickname != "Alice" {
		t.Errorf("Nickname = %q, want Alice", got.Nickname)
	}
}

func TestProfileCache_SellerHitAndMiss(t *testing.T) {
	cache := NewProfileCache()

	cache.SetSeller(100, SellerProfileInfo{StoreName: "好賣家", LogoKey: "logo.jpg"})
	got, ok := cache.GetSeller(100)
	if !ok {
		t.Fatal("cache should hit after Set")
	}
	if got.StoreName != "好賣家" {
		t.Errorf("StoreName = %q, want 好賣家", got.StoreName)
	}
}

func TestProfileCache_TTLExpiry(t *testing.T) {
	cache := NewProfileCache()

	// 手動插入一筆過期的 entry
	cache.buyers.Store(int64(42), cachedBuyerProfile{
		data:      BuyerProfileInfo{Nickname: "old"},
		fetchedAt: time.Now().Add(-6 * time.Minute), // 6 分鐘前（超過 5 分鐘 TTL）
	})

	if _, ok := cache.GetBuyer(42); ok {
		t.Error("cache should miss for expired entry")
	}

	// 確認過期 entry 被刪除
	if _, ok := cache.buyers.Load(int64(42)); ok {
		t.Error("expired entry should be deleted from underlying map")
	}
}
