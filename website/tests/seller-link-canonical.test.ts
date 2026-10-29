/**
 * #365 — every internal seller link must resolve to the CANONICAL storefront
 * segment (`store_code`) whenever the payload it renders from carries one.
 *
 * A store_id href is not broken (middleware 308s it), but it costs a second
 * navigation per tap and points crawlers away from the canonical URL — the
 * exact consolidation the redirect exists to achieve. These cases pin both
 * halves so the surfaces cannot silently drift back:
 *
 *   1. the data pipelines keep carrying store_code from the wire, and
 *   2. the components build hrefs through `sellerPath()` rather than
 *      interpolating `/sellers/${id}` themselves.
 *
 * Recommended sellers were in the second group until the gateway began
 * emitting their segment; they now read it like every other surface.
 *
 * backend#2235 收斂後**全部來源都是 camelCase `storeCode`** —— 這裡以前記載著
 * 「兩個推薦來源用 snake、商品內嵌賣家用 camel」，那個分歧已經不存在
 * （`recommended_sellers.go:40-42`、`marketplace.go:26`）。分歧消失並不代表
 * 這條測試沒用了：rows 是 loosely-typed `Record<string, unknown>`，讀錯名字
 * **不會**型別錯誤，只會恆為 undefined 而讓該表面悄悄退回 308。
 *
 * Posts joined this file in #2098 phase 3: the gateway now lifts store_code off
 * the GetSellerBatch call `enrichPostsWithSellerInfo` already makes for the
 * store name and logo, so `/public/posts` and the marketplace's `latest_posts`
 * both carry the segment at no extra RPC.
 *
 * Orders, coupons, browsing history and im conversations joined in #2137. Each
 * was assumed to need a new seller lookup; each turned out to already make one
 * (order-service and coupon-service via batchGetSellerMeta, the buyer module and
 * im-service via GetSellerBatch) and to be dropping store_code while copying
 * the store name and logo out of the very same response. Zero extra RPC, so
 * they cleared the "already fetched, merely dropped" bar.
 *
 * Those four pass ONLY the code to `sellerPath()` — no fallback id. A payload
 * that reaches them without a code has lost the seller block entirely (service
 * degraded), so the row renders as plain text rather than a link. The fallback
 * that remains in `sellerPath` serves the payloads that carry a seller key but
 * make no lookup at all (recommended-seller rows, legacy cart groups).
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	FavoriteSchema,
	FollowSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import ts from "typescript-compiler-api";
import { type CartGroupWire, flattenGroups } from "../lib/cart/types";
import { mapProduct } from "../lib/market/listFilters";
import { sellerCanonicalUrl, sellerPath, sellerUrl } from "../lib/seo/urls";

/**
 * `sellerPath` called the way every #2137 surface calls it: the canonical
 * segment alone, no fallback id. An absent code must yield "" (no link), never
 * a store_id path.
 */
function canonicalOnly(storeCode: string | undefined): string {
	return sellerPath(storeCode, undefined);
}

const STORE_ID = "2636840881";
const STORE_CODE = "2649605557";
function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

/** Files that render a seller link from a payload known to carry store_code. */
const CANONICAL_LINK_SOURCES = [
	"features/product/components/ProductSellerIntro.tsx",
	"features/market/components/MarketProductCard.tsx",
	"features/feed/components/StoryRail.tsx",
	"features/home/homeSnapshot.ts",
	// The two post-card surfaces, canonical since #2098 phase 3 taught
	// `/public/posts` to carry store_code.
	"features/home/components/HomeStageFeedCard.tsx",
	"features/feed/components/PostDetailViewer.tsx",
	// #2137: the four surfaces whose owning service already had the seller in
	// hand. These pass the code alone — a missing code means no link, not a
	// store_id link.
	"features/account/screens/overview/AccountOverviewClient.tsx",
	"features/account/screens/recent/RecentClient.tsx",
];

/**
 * The #2137 surfaces, with the field their normalizer must keep carrying.
 *
 * Each of these link sites reads ONLY the canonical segment, so a normalizer
 * that quietly stops copying it does not fall back to a store_id link — it
 * removes the link. That is a silent regression a source-level assertion on
 * the component cannot see, so the pipeline half is pinned separately below.
 */

/**
 * The StoryRail producers are enforced by DISCOVERY rather than by a list.
 *
 * `CANONICAL_LINK_SOURCES` above only covers files that build an href, so it
 * cannot reach a rail producer: those hand a `StoryItem` to StoryRail and never
 * import `sellerPath` themselves. And because `StoryItem.storeCode` is
 * optional, a producer that forgets it still type-checks — the failure is a
 * silent 308 per tap, the exact shape a hardcoded list keeps missing (three
 * separate producers were already shipped without it).
 *
 * So the set is derived from the tree: any file that constructs a StoryItem
 * must either carry the segment or be listed as an explicit exception below.
 */
const STORY_ITEM_DEFINITION = "features/feed/components/StoryRail.tsx";

/**
 * Producers whose upstream payload has no store_code at all, with the reason.
 *
 * Empty since #2098 phase 3: `/public/posts` now carries the segment (the
 * gateway lifts it off the GetSellerBatch response it already makes for the
 * store name and logo), so the last excused producer — FeedPage — reads it like
 * every other rail. Kept as a mechanism, not deleted: the discovery test below
 * fails a new producer that forgets the field, and an author needs somewhere
 * legitimate to record a genuine gap rather than deleting the assertion.
 */
const STORY_PRODUCERS_WITHOUT_SEGMENT: Record<string, string> = {};

/** Every file under the scanned roots that BUILDS a StoryItem. */
function discoverStoryItemProducers(): string[] {
	const roots = ["features", "app", "components"];
	const found: string[] = [];
	for (const root of roots) {
		let entries: string[];
		try {
			entries = readdirSync(new URL(`../${root}`, import.meta.url), {
				recursive: true,
			}) as string[];
		} catch {
			continue;
		}
		for (const entry of entries) {
			const relative = `${root}/${entry.split("\\").join("/")}`;
			if (!/\.tsx?$/.test(relative)) continue;
			if (relative === STORY_ITEM_DEFINITION) continue;
			let code: string;
			try {
				code = source(relative);
			} catch {
				continue;
			}
			// A consumer only types a `StoryItem[]` prop; a producer writes the
			// item's own fields, and `avatar:` is written by nothing else.
			if (code.includes("StoryItem") && /^\s*avatar:/m.test(code)) {
				found.push(relative);
			}
		}
	}
	return found.sort();
}

test("seller links on store_code-bearing surfaces are built through sellerPath()", () => {
	for (const path of CANONICAL_LINK_SOURCES) {
		const code = source(path);
		assert.match(
			code,
			/import \{ sellerPath \} from "@\/lib\/seo\/urls";/,
			`${path} must build its seller href through sellerPath()`,
		);
		assert.doesNotMatch(
			code,
			/`\/sellers\/\$\{/,
			`${path} must not interpolate a raw /sellers/{id} href`,
		);
	}
});

/** 參照入口（購物車、訂單、優惠券、聊天、追蹤清單）：進店連結與狀態來自 BatchGetStores 的店家卡片，不讀快照裡的店網址。 */
const CARD_REFERENCE_SOURCES = [
	"features/cart/CartClient.tsx",
	"features/account/screens/favorites/FavoritesClient.tsx",
	"features/messages/components/ChatContextPanel.tsx",
	"features/account/screens/orders/OrderDetailClient.tsx",
	"features/account/screens/orders/OrderQuickActionDrawer.tsx",
	"features/account/screens/coupons/CouponsClient.tsx",
	"features/account/screens/messages/ThreadClient.tsx",
];

test("reference entries take their store link from the store card, not the snapshot code", () => {
	for (const path of CARD_REFERENCE_SOURCES) {
		const code = source(path);
		assert.match(
			code,
			/useStoreCardViews|storeCardView\(/,
			`${path} must resolve its store link through the BatchGetStores card`,
		);
		assert.doesNotMatch(
			code,
			/sellerPath\(/,
			`${path} must not build a seller href from the snapshot's store code`,
		);
	}
});

test("sellerPath prefers the store_code and falls back to the key it was given", async () => {
	const { sellerPath } = await import("../lib/seo/urls");
	assert.equal(sellerPath(STORE_CODE, STORE_ID), `/sellers/${STORE_CODE}`);
	assert.equal(sellerPath(undefined, STORE_ID), `/sellers/${STORE_ID}`);
	assert.equal(sellerPath("", STORE_ID), `/sellers/${STORE_ID}`);
	assert.equal(sellerPath(" ", STORE_ID), `/sellers/${STORE_ID}`);
	// Numeric keys arrive off the wire as numbers on some payloads.
	assert.equal(sellerPath(null, 2636840881), `/sellers/${STORE_ID}`);
	// Neither key usable → empty, so a caller's own guard is what decides
	// whether a link renders at all (never a link to `/sellers/`).
	assert.equal(sellerPath(null, undefined), "");
	assert.equal(sellerPath(undefined, ""), "");
});

test("product list mapping carries the seller store_code to the card", () => {
	const item = mapProduct(
		{
			id: "p1",
			name: "Product",
			storeId: STORE_ID,
			seller: { name: "Store", storeId: STORE_ID, storeCode: STORE_CODE },
		},
		0,
	);
	assert.equal(item.sellerId, STORE_ID);
	assert.equal(item.sellerStoreCode, STORE_CODE);

	// A seller with no minted code stays on its store_id — middleware only
	// redirects a key it could resolve, so that path serves the storefront.
	const codeless = mapProduct(
		{ id: "p2", name: "Product", storeId: STORE_ID, seller: {} },
		1,
	);
	assert.equal(codeless.sellerStoreCode, undefined);
});

test("cart groups carry the seller store_code alongside storeId", () => {
	const group = {
		storeId: STORE_ID,
		storeCode: STORE_CODE,
		sellerName: "Store",
		items: [{ skuId: "sku-1", name: "Product", price: "100", quantity: "1" }],
	} satisfies CartGroupWire;

	const item = flattenGroups([group])[0];
	// Normalized `CartItem.sellerStoreId` — the 逛店 link's guard reads it, so
	// a rename must not blank it out.
	assert.equal(item.sellerStoreId, STORE_ID);
	assert.equal(item.sellerStoreCode, STORE_CODE);
});

test("home recommended-seller cards link the canonical store_code", async () => {
	const { buildHomeSnapshot } = await import("../features/home/homeSnapshot");
	const snapshot = buildHomeSnapshot({
		schemaVersion: "home.v2",
		hotProducts: [],
		products: { data: { records: [] } },
		recommendedSellers: [
			{
				id: STORE_ID,
				storeCode: STORE_CODE,
				name: "有短碼",
				logoUrl: "",
				coverUrl: "",
				townCode: "",
			},
		],
		posts: { page: 1, pageSize: 8, total: 0, records: [] },
		categoryGroups: [],
	});

	const coded = snapshot.mappedSellers.find((s) => s.name === "有短碼");
	assert.equal(coded?.href, `/sellers/${STORE_CODE}`);
	assert.notEqual(coded?.href, `/sellers/${STORE_ID}`);
});

test("every StoryRail producer carries the seller store_code", () => {
	const producers = discoverStoryItemProducers();
	// Without this the loop below would pass vacuously if discovery ever broke.
	assert.ok(
		producers.length >= 2,
		`expected to discover the known StoryItem producers, got ${producers.length}: ${producers.join(", ")}`,
	);

	for (const path of producers) {
		if (path in STORY_PRODUCERS_WITHOUT_SEGMENT) continue;
		const code = source(path);
		assert.match(
			code,
			/storeCode:/,
			`${path} builds a StoryItem without a storeCode, so its rail links the store_id and eats a 308 per tap. Carry the segment, or add it to STORY_PRODUCERS_WITHOUT_SEGMENT with a reason.`,
		);
		// 收斂後讀寫同名，所以「有沒有帶 storeCode」與「讀對名字沒有」不再是
		// 兩個不同的正則。改成反向釘：舊的 snake 名字讀不到值（rows 是
		// Record<string, unknown>，讀錯**不會**型別錯誤，只會恆為 undefined），
		// 所以它一旦復活就只能靠這條抓。
		assert.doesNotMatch(
			code,
			/\bstore_code\b/,
			`${path} 還在讀 snake_case store_code —— backend#2235 之後那個欄名不存在，值恆為 undefined，該 rail 會靜默退回 storeId 並每點一次吃一個 308`,
		);
	}

	// A stale exception is as bad as a missing one: it would silently excuse a
	// producer that no longer exists or that now has the field available.
	for (const excused of Object.keys(STORY_PRODUCERS_WITHOUT_SEGMENT)) {
		assert.ok(
			producers.includes(excused),
			`${excused} is excused from carrying store_code but is no longer a StoryItem producer — drop the entry`,
		);
	}
});

test("the posts pipeline carries store_code from the wire onto the feed rail", async () => {
	const { normalizePost } = await import("../lib/api/posts");
	// normalizePost is a spread, not a whitelist — but that is exactly the kind
	// of thing a later "tidy up" turns into an explicit field list, silently
	// dropping the segment again (the favourites normalizer did precisely that).
	const normalized = normalizePost({
		id: "post-1",
		sellerId: STORE_ID,
		storeCode: STORE_CODE,
		sellerName: "Store",
	} as never);
	assert.equal(normalized.storeCode, STORE_CODE);

	// FeedPage's buildStories is module-private, so assert the source instead:
	// it must read store_code, and must not fall back to the seller_id.
	const feed = source("features/feed/FeedPage.tsx");
	assert.match(feed, /storeCode: p\.storeCode \|\| undefined/);
});

test("the market 推薦商家 rail is built from the recommended stores' store_code", () => {
	const code = source("features/market/products/MarketProductsListPage.tsx");
	assert.match(code, /recommendedStoriesFromStores\(/);
});

test("followed-seller rows keep store_code alongside the seller_id the API keys on", () => {
	const row = create(FollowSchema, {
		store: { id: STORE_ID, storeCode: STORE_CODE, name: "Store" },
	});
	assert.equal(row.store?.id, STORE_ID);
	assert.equal(row.store?.storeCode, STORE_CODE);
});

/**
 * Favorites and member cards render the seller's identity (name + logo) but
 * carry only a store_id, so any seller link added to them would be a
 * non-canonical one — and once backend#2098 phase 3 narrows the storefront
 * resolver to store_code, a 404. Both normalizers are whitelists: a field the
 * gateway sends but they don't copy is silently dropped, which reads to the
 * next author as "the backend doesn't send it". These pin the pipeline half.
 */
test("favorite rows carry the seller store_code the wire now sends", async () => {
	const row = create(FavoriteSchema, {
		product: {
			id: "p1",
			store: { id: STORE_ID, name: "Store", storeCode: STORE_CODE },
		},
	});
	assert.equal(row.product?.store?.id, STORE_ID);
	assert.equal(row.product?.store?.storeCode, STORE_CODE);

	// seller 模組 unreachable → the backend blanks the whole seller block,
	// so the absent segment is indistinguishable from "no seller info at all".
	const degraded = create(FavoriteSchema, { product: { id: "p2" } });
	assert.equal(degraded.product?.store, undefined);
});

test("member cards carry the seller store_code the wire now sends", async () => {
	const { normalizeMemberCard } = await import("../lib/member-cards/repo");
	const card = normalizeMemberCard({
		sellerId: STORE_ID,
		sellerName: "Store",
		storeCode: STORE_CODE,
		totalSpending: "1420",
	});
	assert.equal(card.sellerId, STORE_ID);
	assert.equal(card.storeCode, STORE_CODE);

	// The in-site route is keyed on sellerId and is unaffected by the degrade.
	const degraded = normalizeMemberCard({
		sellerId: STORE_ID,
		totalSpending: "1420",
	});
	assert.equal(degraded.storeCode, "");
	assert.equal(degraded.sellerId, STORE_ID);
});

test("order rows carry the seller store_code and link only the canonical segment", async () => {
	const { normalizeOrder } = await import("../lib/orders/repo");
	const row = normalizeOrder({
		orderNo: "O1",
		storeId: STORE_ID,
		storeCode: STORE_CODE,
		sellerName: "Store",
	});
	assert.equal(row.storeId, STORE_ID);
	assert.equal(row.storeCode, STORE_CODE);
	assert.equal(canonicalOnly(row.storeCode), `/sellers/${STORE_CODE}`);

	// order-service blanks the whole seller block when the lookup degrades, so
	// the absent segment comes with an absent name — no link, not an id link.
	const degraded = normalizeOrder({ orderNo: "O2", storeId: STORE_ID });
	assert.equal(degraded.storeCode, undefined);
	assert.equal(canonicalOnly(degraded.storeCode), "");
});

test("coupon rows carry the seller store_code the wire now sends", async () => {
	const { normalizeCoupon } = await import("../lib/coupons/repo");
	const row = normalizeCoupon({
		id: "c1",
		applicableStoreId: STORE_ID,
		sellerName: "Store",
		storeCode: STORE_CODE,
	});
	assert.equal(row.applicableStoreId, STORE_ID);
	assert.equal(row.storeCode, STORE_CODE);

	// A platform-wide coupon has no seller at all.
	const platform = normalizeCoupon({ id: "c2" });
	assert.equal(platform.storeCode, undefined);
	assert.equal(canonicalOnly(platform.storeCode), "");
});

test("conversation rows carry the seller store_code the wire now sends", async () => {
	const { normalizeConversation } = await import("../lib/chat/repo");
	const row = normalizeConversation({
		targetUserId: STORE_ID,
		storeName: "Store",
		storeCode: STORE_CODE,
	});
	assert.equal(String(row.targetUserId), STORE_ID);
	assert.equal(row.storeCode, STORE_CODE);

	// Buyer peers have no storefront, so no segment and no link.
	const buyerPeer = normalizeConversation({
		targetUserId: "2623900520",
		targetNickname: "買家",
	});
	assert.equal(buyerPeer.storeCode, undefined);
	assert.equal(canonicalOnly(buyerPeer.storeCode), "");
});

/* ===================================================================== */
/* Canonical TAGS (not links): the seller routes' own <link rel=canonical> */
/* ===================================================================== */

/**
 * The routes build their canonical from the PAYLOAD, and the payload spells the
 * segment `storeCode`.
 *
 * This is the half a link-side test cannot see. `sellerPath` callers hand the
 * field in explicitly, so a misread shows up as a wrong href. A canonical is
 * built inside `generateMetadata` from `getCachedSeller`'s loosely-typed
 * result, where reading the retired `store_code` compiles, resolves to
 * undefined, and falls through to the request's own id segment — which is
 * ALREADY the store_code on every request middleware canonicalized. So the bug
 * is invisible exactly as long as the backend is healthy: middleware fails OPEN
 * on a rate limit / 5xx / timeout (`canonical-redirect.ts` → `{kind:"unknown"}`),
 * and on that path the page renders at the store_id URL and self-canonicalizes
 * to the WRONG one.
 *
 * Hence the fixture below uses a seller whose code differs from its id (dev's
 * `2636840881` → `2649605557`). A fixture where the two coincide passes with or
 * without the fix and would pin nothing.
 */
test("sellerCanonicalUrl reads the payload's storeCode, not the request's id", () => {
	assert.equal(
		sellerCanonicalUrl({ storeCode: STORE_CODE }, STORE_ID),
		sellerUrl(STORE_CODE),
	);
	assert.notEqual(
		sellerCanonicalUrl({ storeCode: STORE_CODE }, STORE_ID),
		sellerUrl(STORE_ID),
	);
	// Tabs stay SELF-canonical: the suffix rides along.
	assert.equal(
		sellerCanonicalUrl({ storeCode: STORE_CODE }, STORE_ID, "/reviews"),
		sellerUrl(STORE_CODE, "/reviews"),
	);
	// Numeric codes (some payloads emit the segment unquoted).
	assert.equal(
		sellerCanonicalUrl({ storeCode: Number(STORE_CODE) }, STORE_ID),
		sellerUrl(STORE_CODE),
	);
});

test("the retired store_code spelling is not honoured as a fallback", () => {
	// A payload carrying ONLY the pre-backend#2235 name must behave exactly like
	// a payload with no code at all — falling back to the request id. Accepting
	// it "just in case" would make the fix untestable: the assertion above would
	// pass while the routes still read the dead name.
	const retiredOnly = { store_code: STORE_CODE } as unknown as {
		storeCode?: unknown;
	};
	assert.equal(sellerCanonicalUrl(retiredOnly, STORE_ID), sellerUrl(STORE_ID));

	// No payload at all (a 404'd seller, or a cache miss during an outage) —
	// still a valid URL on the id the request arrived with, never `/sellers/`.
	assert.equal(sellerCanonicalUrl(null, STORE_ID), sellerUrl(STORE_ID));
	assert.equal(sellerCanonicalUrl(undefined, STORE_ID), sellerUrl(STORE_ID));
	assert.equal(sellerCanonicalUrl({}, STORE_ID), sellerUrl(STORE_ID));
	// Whitespace is not a code.
	assert.equal(
		sellerCanonicalUrl({ storeCode: "  " }, STORE_ID),
		sellerUrl(STORE_ID),
	);
});

/** Every seller route that emits a self-canonical. */
const SELLER_CANONICAL_ROUTES = [
	"features/sellers/storefront/metadata.ts",
	"features/sellers/posts/SellerPostsPage.tsx",
	"features/sellers/reviews/SellerReviewsPage.tsx",
	"features/sellers/messages/SellerMessagesPage.tsx",
];

test("the storefront route delegates metadata to its feature owner", () => {
	const route = source(
		"app/[locale]/(public)/sellers/[id]/(storefront)/page.tsx",
	);
	assert.match(
		route,
		/export \{ generateMetadata \} from "@\/features\/sellers\/storefront\/metadata"/,
	);
	assert.doesNotMatch(route, /function generateMetadata/);
});

test("every seller route derives its canonical through sellerCanonicalUrl", () => {
	for (const path of SELLER_CANONICAL_ROUTES) {
		const code = source(path);
		assert.match(
			code,
			/sellerCanonicalUrl\(/,
			`${path} must build its canonical through sellerCanonicalUrl() so the field is read in one place`,
		);
		// `sellerUrl(x || id)` is the shape that grew the defect: it puts the
		// field read back at the call site, where the id fallback hides it.
		assert.doesNotMatch(
			code,
			/sellerUrl\([^)]*\|\|/,
			`${path} must not re-derive the canonical segment inline — that is the shape that silently fell back to the request id`,
		);
	}
});

/* ===================================================================== */
/* §17.7 sweep: every name backend#2235 retired, everywhere at once       */
/* ===================================================================== */

/**
 * Wire names the 2026-08-05/06 camelCase convergence (backend#2235) retired,
 * each with the files still allowed to spell them and why.
 *
 * WHY A SWEEP AND NOT ONE ASSERTION PER SURFACE. Every one of these misreads
 * has the same signature and the same reason it survives review: the payload is
 * loosely typed, so the retired spelling COMPILES, resolves to `undefined`, and
 * lands on a fallback that happens to be right in the common case — the request
 * id already equals the store_code, the wallet prefetch is only a perf hint,
 * the optimistic cache write is only a head start on a refetch. Nothing throws,
 * nothing turns red, and the defect surfaces only when the surrounding
 * conditions stop coinciding. A per-surface assertion can only pin the surfaces
 * someone already thought of; the sweep pins the class.
 *
 * `staged_token` / `appeal_reason` / `appeal_evidence_asset_ids` are the write
 * side of the same class, and they do NOT degrade quietly: the gateway rejects
 * the body outright (verified against dev on 2026-08-06 — `400 staged_token is
 * required` on buyer-avatar finalize, `400 … 'AppealReason' failed on the
 * 'required' tag` on violation appeal). They are swept here anyway because the
 * cause is identical and the next one may well land on an optional field, where
 * it would go back to being silent.
 */
const RETIRED_WIRE_NAMES: Record<string, Record<string, string>> = {
	/**
	 * All three allowances are the CVS **pickup store** (超商門市代號) from the
	 * store-map callback and the order/shipment payloads — a different
	 * `store_code` from the seller storefront segment, on a wire this repo does
	 * not own. Each reads it beside the camelCase twin, so neither spelling is
	 * load-bearing alone.
	 */
	store_code: {
		"lib/checkout/store-pickups.ts":
			"CVS pickup-store wire (超商門市), dual-read",
		"lib/orders/repo.ts":
			"CVS pickup-store fields on order/shipment payloads, dual-read",
		"app/(commerce)/store-map/return/route.ts":
			"CVS store-map callback query params — third-party shape",
	},
	overall_certification_status: {},
	staged_token: {},
	appeal_reason: {},
	appeal_evidence_asset_ids: {},
};

/**
 * Identifier / string-literal occurrences of `name` in shipped source, found on
 * the TypeScript AST rather than by grep.
 *
 * Grep cannot do this one: the canonical code carries several paragraphs of
 * prose that legitimately spell `store_code` (it is what the segment IS called
 * on the backend and in every issue), and a comment-stripping regex would have
 * to get string literals containing `//` right to stay sound — a regex that is
 * wrong in the safe direction still produces false NEGATIVES, which is the one
 * failure mode a gate may not have. The AST has no comment nodes at all, so a
 * prose mention is structurally invisible and a real read is structurally
 * visible.
 */
function findRetiredNameReads(
	name: string,
): Array<{ file: string; line: number }> {
	const roots = ["app", "components", "features", "lib"];
	const hits: Array<{ file: string; line: number }> = [];
	for (const root of roots) {
		let entries: string[];
		try {
			entries = readdirSync(new URL(`../${root}`, import.meta.url), {
				recursive: true,
			}) as string[];
		} catch {
			continue;
		}
		for (const entry of entries) {
			const relative = `${root}/${entry.split("\\").join("/")}`;
			if (!/\.tsx?$/.test(relative)) continue;
			if (/\.(test|spec)\.tsx?$/.test(relative)) continue;
			if (/\.unit\.test\.tsx?$/.test(relative)) continue;
			let code: string;
			try {
				code = source(relative);
			} catch {
				continue;
			}
			if (!code.includes(name)) continue;
			const sf = ts.createSourceFile(
				relative,
				code,
				ts.ScriptTarget.Latest,
				true,
				relative.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
			);
			const visit = (node: ts.Node): void => {
				if (
					(ts.isIdentifier(node) || ts.isStringLiteral(node)) &&
					node.text === name
				) {
					hits.push({
						file: relative,
						line: sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1,
					});
				}
				node.forEachChild(visit);
			};
			visit(sf);
		}
	}
	return hits;
}

test("no shipped source reads a wire name backend#2235 retired", () => {
	// Proves the AST walk actually reaches source before any empty result below
	// is read as "clean". The CVS allowances are the only retired spelling that
	// legitimately survives, so they double as the scan's canary: an empty
	// result here means the walk broke, not that the tree is clean.
	const canary = findRetiredNameReads("store_code");
	assert.ok(
		canary.length > 0,
		"the AST scan found no store_code anywhere — it stopped reaching source, so every empty result below is meaningless",
	);

	for (const [name, allowed] of Object.entries(RETIRED_WIRE_NAMES)) {
		const offenders = findRetiredNameReads(name).filter(
			(h) => !(h.file in allowed),
		);
		assert.deepEqual(
			offenders,
			[],
			`\`${name}\` is not a name the wire sends any more (backend#2235). These read or write it, so the value is always undefined and the surface silently falls back:\n${offenders
				.map((h) => `  ${h.file}:${h.line}`)
				.join("\n")}`,
		);

		// A stale allowance is as bad as a missing one: it would silently excuse
		// a file that has since been fixed, and the next author would copy it.
		for (const file of Object.keys(allowed)) {
			assert.ok(
				canary.some((h) => h.file === file) ||
					findRetiredNameReads(name).some((h) => h.file === file),
				`${file} is allowed to name \`${name}\` but no longer does — drop the entry`,
			);
		}
	}
});
