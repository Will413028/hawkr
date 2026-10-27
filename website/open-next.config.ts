// open-next.config.ts
import { defineCloudflareConfig } from "@opennextjs/cloudflare";
import { purgeCache } from "@opennextjs/cloudflare/overrides/cache-purge/index";
import r2IncrementalCache from "@opennextjs/cloudflare/overrides/incremental-cache/r2-incremental-cache";
import { withRegionalCache } from "@opennextjs/cloudflare/overrides/incremental-cache/regional-cache";
import doQueue from "@opennextjs/cloudflare/overrides/queue/do-queue";
import queueCache from "@opennextjs/cloudflare/overrides/queue/queue-cache";
import doShardedTagCache from "@opennextjs/cloudflare/overrides/tag-cache/do-sharded-tag-cache";
import {
	softTagFilter,
	withFilter,
} from "@opennextjs/cloudflare/overrides/tag-cache/tag-cache-filter";

export default defineCloudflareConfig({
	// ISR / cacheComponents 的渲染結果存 R2（KV 不可用：eventual consistency 會讀到髒快取）。
	// regional cache：SEO 站讀多寫少，邊緣再快取一層省 R2 往返。
	// 需要 wrangler r2_buckets binding NEXT_INC_CACHE_R2_BUCKET（見 wrangler.jsonc）。
	//
	// Keep short-lived until a cross-PoP freshness probe proves purge delivery
	// under burst load. It bounds a regional entry to about one minute even if a
	// purge is delayed. Keep tag-cache checks enabled, but do not lazily re-read
	// R2 on every regional hit: OpenNext 1.20's background read has no timeout
	// and can leave an otherwise-complete navigation waiting on Worker I/O.
	// A future long-lived canary must compare freshness, R2 Class B operations
	// and latency together rather than assuming fewer reads.
	incrementalCache: withRegionalCache(r2IncrementalCache, {
		mode: "short-lived",
		shouldLazilyUpdateOnCacheHit: false,
		bypassTagCacheOnCacheHit: false,
	}),

	// Time-based cacheLife revalidation must be coordinated across isolates.
	// The DO queue deduplicates concurrent refreshes for the same stale entry.
	// Cache the accepted deduplication id for the full 40s canary probe window so
	// repeated STALE reads in one colo do not each pay a JS-RPC to the DO. Cache
	// only after the DO acknowledges the message: a transport failure must remain
	// retryable instead of being hidden behind a local suppression marker.
	// Requires NEXT_CACHE_DO_QUEUE → DOQueueHandler in every deployed env.
	queue: queueCache(doQueue, {
		regionalCacheTtlSec: 60,
		waitForQueueAck: true,
	}),

	// Explicit revalidateTag calls rely on this store (the app intentionally
	// forbids revalidatePath while softTagFilter is enabled).
	// sharded Durable Object（SQLite-backed，吃免費額度，閒置自動休眠）；regionalCache 邊緣再快取 tag 查詢。
	// 需要 wrangler durable_objects binding NEXT_TAG_CACHE_DO_SHARDED → DOShardedTagCache。
	// This app invalidates explicit content tags only. Dropping Next's implicit
	// `_N_T_` path tags avoids needless Durable Object reads/writes. The
	// architecture check forbids revalidatePath() while this filter is enabled.
	//
	// shardReplication + regionalReplication.defaultRegion: "apac"
	// ------------------------------------------------------------------
	// 沒有 regionalReplication 時，每個 shard 只有一個 DO 實例，落點由「第一個
	// 建立它的請求」決定 —— 實測 prod 三個 shard 落在 SIN 7ms / FRA 68ms / IAD 73ms，
	// 兩個錯位到歐美是因為歐洲的無 UA HEAD 探針先碰到它們。而真人 80~97% 在台灣，
	// 於是每次 render 的 tag 查詢都在付一趟跨洋 RTT。
	//
	// 開啟後（do-sharded-tag-cache.js:426,442）讀取固定走 defaultRegion，
	// 台灣使用者恆定命中 apac。代價在寫入側：:399-404 的 AVAILABLE_REGIONS.map
	// 讓每次 tag 失效從打 1 個 DO 變成打 6 個（enam/weur/apac/sam/afr/oc）。
	// 這個交換對本站成立 —— 讀是每請求一次，寫（內容失效）實測只有 0~3 次/天。
	//
	// ⚠️ defaultRegion 必填：省略時 :28 會 fallback 到 DEFAULT_REGION = "enam"，
	// 等於把落點釘在北美，比現況更糟。只加 shardReplication 而不加
	// regionalReplication 則 :27 的 Boolean() 為 false，整段完全不生效。
	// tests/open-next-cache-config.test.ts 有對應 gate 擋這兩種寫法。
	//
	// replicas 都取 1：本站要的是「落點正確」，不是「多副本分攤讀取壓力」。
	// 調高只會讓寫入放大再乘上倍數。
	tagCache: withFilter({
		tagCache: doShardedTagCache({
			baseShardSize: 4,
			regionalCache: true,
			shardReplication: {
				numberOfSoftReplicas: 1,
				numberOfHardReplicas: 1,
				regionalReplication: { defaultRegion: "apac" },
			},
		}),
		filterFn: softTagFilter,
	}),

	// Buffer and deduplicate CDN purge tags in a Durable Object before calling
	// Cloudflare's purge API. Inventory/publish bursts can otherwise exceed the
	// tag-purge rate limit. OpenNext 1.20 retries rate-limited (429) batches via
	// the DO alarm; other purge failures still rely on the short-lived regional
	// cache and cache-tag freshness boundary.
	// Requires NEXT_CACHE_DO_PURGE → BucketCachePurge and the two purge secrets.
	cachePurge: purgeCache({ type: "durableObject" }),

	// ISR / SSG 命中時直接從 incremental cache 回 HTML 與 RSC，不進 NextServer、
	// 不載入該頁的 server bundle（handler.mjs 約 22 MB，其中商品頁與賣家頁的
	// render 路徑正是 #338 定案的 Worker OOM 來源）。這是 OpenNext 對純 ISR 站
	// 最直接的冷啟動與記憶體減負，前提只有一個：不能是 PPR —— 本站在 #338 已
	// 定案全站純 ISR、cacheComponents 關閉，prerender-manifest 零 PARTIALLY_STATIC
	// 路由（tests/open-next-cache-config.test.ts 有 gate）。
	//
	// 攔截發生在 middleware 之後（@opennextjs/aws routingHandler.js:176），所以
	// 賣家 store_id → store_code 的 308、角色分區守衛都照常先跑；MISS / STALE
	// 仍回到 NextServer 走原本的 ISR 路徑，DO queue 的去重與 tag 失效不受影響。
	// next.config.ts `headers()` 的 Cache-Control 在攔截結果上一樣後套
	// （applyMiddlewareHeaders 最後執行），瀏覽器看到的政策與未攔截時一致。
	enableCacheInterception: true,
});
