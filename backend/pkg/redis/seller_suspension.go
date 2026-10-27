package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// seller:suspended:<accountId> — admin manual suspension state propagation
// (32 §13 / 46 §4.2). Mirrors the account:blocked:<id> blacklist pattern.
// Value = the suspension reason code, so guards can surface it without a round
// trip back to seller-service.
//
// 🔴 **鍵一律是 account_id，不是 store_id。** 兩者在首任賣家上相等（grandfather），
// 所以寫錯的那一側在測試與首任賣家身上完全看不出來——而 dev 上有 195 個現任賣家
// store_id ≠ account_id，對他們而言那是一把**永遠不可能存在的 key**（#3007 順帶發現，
// gateway sellerSuspensionOnly 曾經就是這樣讀的 ⇒ 那些賣家在那條 guard 上永久 fail-open）。
//
// ⚠️ 以下兩句原本寫在這裡，2026-09-02 實查**都不成立**，已刪除：
//
//  1. 「WITHOUT a TTL: admin suspension never auto-expires」——沒有 TTL 不等於受保護。
//     這顆 Redis 是 maxmemory 512mb ＋ **allkeys-lru**，無 TTL 的 key 一樣是逐出候選；
//     今天沒被逐出只是因為用量低（2.88M/512M），不是因為它有豁免。
//     而逐出的後果不是「稍後重建」，是**永久破口**：讀取端把 miss 當成 active。
//  2. 「and the seller.suspended/seller.unsuspended MQ consumer as a self-healing
//     second writer」——**那個 consumer 不存在**（services/seller-service/internal/mq/
//     底下只有 account_deleted / agent_notify / product_view / review_rating）。
//     寫入者只有一個：seller_status_recompute 的 post-commit SyncRedis。
//     那句話讓人以為有自癒機制，於是不會去想 miss 該怎麼辦——而那正是缺口所在。

const sellerSuspendedPrefix = "seller:suspended:"

func sellerSuspendedKey(accountID int64) string {
	return fmt.Sprintf("%s%d", sellerSuspendedPrefix, accountID)
}

// SetSellerSuspended writes the suspension marker for accountID, no TTL.
func SetSellerSuspended(ctx context.Context, rdb *redis.Client, accountID int64, reasonCode string) error {
	return rdb.Set(ctx, sellerSuspendedKey(accountID), reasonCode, 0).Err()
}

// ClearSellerSuspended removes the suspension marker for accountID.
func ClearSellerSuspended(ctx context.Context, rdb *redis.Client, accountID int64) error {
	return rdb.Del(ctx, sellerSuspendedKey(accountID)).Err()
}

// ---------------------------------------------------------------------------
// 三態查詢 ＋ 負快取（#3007 裁決 A，2026-09-02）
//
// 🔴 **兩個 key、兩個擁有者，刻意不共用**：
//
//	seller:suspended:<accountId>   seller-service 擁有。無 TTL。存在 ⟺ 已停權。
//	                              gateway **只讀不寫** —— 它是 lift 時要被 DEL 的那一把，
//	                              gateway 若也寫它（尤其帶 TTL）就會有兩個真實來源。
//	seller:suspcache:<accountId>   gateway 擁有。帶 TTL。回源查完的結果快取，正負面都存。
//
// 為什麼需要負快取：沒有它，每一個**沒被停權**的賣家的每一次 BLOCK-capability 請求
// 都要打一次 seller-service —— 那是把一個熱路徑綁在跨服務往返上。
//
// 為什麼 TTL 不能省：`allkeys-lru` 下無 TTL 的 key 一樣是逐出候選，而逐出後
// 「miss ⇒ 回源」是有界的一次多餘 RPC，不再是永久破口。TTL 同時是停權／解除停權
// 生效延遲的上界。
// ---------------------------------------------------------------------------

// SuspensionState 是三態。**不要把它壓回布林** —— 那正是舊簽章的問題：
// 它把「這位賣家沒被停權」與「我不知道」壓成同一個 false，呼叫端沒有第三條路可走。
type SuspensionState int

const (
	// SuspensionUnknown ＝ 兩個 key 都沒命中，或 Redis 出錯。呼叫端**必須回源**。
	SuspensionUnknown SuspensionState = iota
	// SuspensionActive ＝ 負快取命中，確定沒被停權。
	SuspensionActive
	// SuspensionSuspended ＝ 確定被停權（reason 可能為空字串：手改 DB 造出來的列沒有理由碼）。
	SuspensionSuspended
)

const sellerSuspCachePrefix = "seller:suspcache:"

func sellerSuspCacheKey(accountID int64) string {
	return fmt.Sprintf("%s%d", sellerSuspCachePrefix, accountID)
}

// LookupSellerSuspension 三態查詢：先讀權威 marker，再讀負快取，都沒有就回 Unknown。
//
// 讀取順序不可調換：權威 marker 在前，才不會讓一份還沒過期的負快取蓋掉
// seller-service 剛寫進來的停權事實。
func LookupSellerSuspension(ctx context.Context, rdb *redis.Client, accountID int64) (SuspensionState, string) {
	if rdb == nil {
		return SuspensionUnknown, ""
	}
	if val, err := rdb.Get(ctx, sellerSuspendedKey(accountID)).Result(); err == nil {
		return SuspensionSuspended, val
	} else if !errors.Is(err, redis.Nil) {
		// Redis 故障：**不得**當成 Active。回 Unknown 讓呼叫端自己決定要不要回源。
		return SuspensionUnknown, ""
	}
	val, err := rdb.Get(ctx, sellerSuspCacheKey(accountID)).Result()
	if err != nil {
		return SuspensionUnknown, ""
	}
	if val == "0" {
		return SuspensionActive, ""
	}
	if reason, ok := strings.CutPrefix(val, "1:"); ok {
		return SuspensionSuspended, reason
	}
	// 值域外的內容視為不可信 ⇒ 回源重建，而不是猜。
	return SuspensionUnknown, ""
}

// CacheSellerSuspension 把回源結果寫進 gateway 自己的快取鍵。
//
// **失敗不回報**：寫不進去的後果只是下一次再回源一次，而把它做成錯誤會讓一次
// Redis 抖動變成請求失敗。
func CacheSellerSuspension(ctx context.Context, rdb *redis.Client, accountID int64,
	suspended bool, reasonCode string, ttl time.Duration) {
	if rdb == nil || ttl <= 0 {
		return
	}
	val := "0"
	if suspended {
		val = "1:" + reasonCode
	}
	_ = rdb.Set(ctx, sellerSuspCacheKey(accountID), val, ttl).Err()
}

// InvalidateSellerSuspensionCache 清掉 gateway 的快取鍵。
//
// 供 seller-service 之外的路徑在確知狀態已變時使用（例如 ADM 停權後想立即生效）。
// 權威 marker 不在此清 —— 那是 seller-service 的。
func InvalidateSellerSuspensionCache(ctx context.Context, rdb *redis.Client, accountID int64) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(ctx, sellerSuspCacheKey(accountID)).Err()
}

// GetSellerSuspension returns (reasonCode, true) if accountID is currently
// marked suspended. Returns ("", false) on cache miss AND on Redis failure —
// callers on the BLOCK-if-suspended path fail-open per 46 §4.3 (the
// CheckoutValidate / product ACTIVE transition backstops are the fail-closed
// correctness net, not this cache read).
//
// ⚠️ **這個簽章在型別上就強制呼叫端 fail-open**：它把「這位賣家沒被停權」與
// 「我不知道」壓成同一個 false，呼叫端沒有第三條路可走。46 §4.3 是這樣定的
// （可用性優先），而 #3007 指出它的補償控制對唯讀類 capability（CapInvoiceManage）
// 不成立——那一類沒有 CheckoutValidate 這種收斂點兜底。要不要改成 miss 回源查
// seller-service，是推翻設計書既有決定的架構取捨，不在本次修復範圍。
func GetSellerSuspension(ctx context.Context, rdb *redis.Client, accountID int64) (string, bool) {
	val, err := rdb.Get(ctx, sellerSuspendedKey(accountID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return "", false // fail-open on Redis error
		}
		return "", false // cache miss
	}
	return val, true
}
