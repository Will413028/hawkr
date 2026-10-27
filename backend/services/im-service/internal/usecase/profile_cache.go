package usecase

import (
	"sync"
	"time"
)

const profileCacheTTL = 5 * time.Minute

type cachedBuyerProfile struct {
	fetchedAt time.Time
	data      BuyerProfileInfo
}

type cachedSellerProfile struct {
	fetchedAt time.Time
	data      SellerProfileInfo
}

// ProfileCache 提供 in-memory TTL cache 給 buyer/seller profile。
// 兩張表的 key 都是「已解析的 room 身分」——buyers 用 buyer_id、sellers 用
// store_id，與 room id（b{buyerID}_s{storeID}）兩側一致，都不是 account_id。
type ProfileCache struct {
	buyers  sync.Map // buyer_id -> cachedBuyerProfile
	sellers sync.Map // store_id -> cachedSellerProfile
}

func NewProfileCache() *ProfileCache {
	return &ProfileCache{}
}

func (c *ProfileCache) GetBuyer(buyerID int64) (BuyerProfileInfo, bool) {
	v, ok := c.buyers.Load(buyerID)
	if !ok {
		return BuyerProfileInfo{}, false
	}
	entry := v.(cachedBuyerProfile)
	if time.Since(entry.fetchedAt) > profileCacheTTL {
		c.buyers.Delete(buyerID)
		return BuyerProfileInfo{}, false
	}
	return entry.data, true
}

func (c *ProfileCache) SetBuyer(buyerID int64, p BuyerProfileInfo) {
	c.buyers.Store(buyerID, cachedBuyerProfile{data: p, fetchedAt: time.Now()})
}

func (c *ProfileCache) GetSeller(storeID int64) (SellerProfileInfo, bool) {
	v, ok := c.sellers.Load(storeID)
	if !ok {
		return SellerProfileInfo{}, false
	}
	entry := v.(cachedSellerProfile)
	if time.Since(entry.fetchedAt) > profileCacheTTL {
		c.sellers.Delete(storeID)
		return SellerProfileInfo{}, false
	}
	return entry.data, true
}

func (c *ProfileCache) SetSeller(storeID int64, p SellerProfileInfo) {
	c.sellers.Store(storeID, cachedSellerProfile{data: p, fetchedAt: time.Now()})
}
