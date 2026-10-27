/**
 * Product reviews — buyer write + public list.
 *
 * Endpoints:
 *   POST   /app/reviews                     submit an order-item review (see
 *                                           submitOrderItemReview) — the real
 *                                           backend route; photos ride along as
 *                                           `stagedTokens`.
 *   GET    /public/products/{id}/reviews    ?page&size&rating
 *   POST   /app/sellers/reviews/{id}/reply  (seller reply; body { content })
 */

import { apiClient, authedFetch } from "../api/client";

/** Seller's official reply to a buyer review. Backend returns it as a `reply`
 *  object on each list item (`{ content, repliedAt }`); absent when unanswered. */
export interface ReviewReply {
	content: string;
	/** ISO 8601 UTC, e.g. `2026-06-28T15:59:32Z`. */
	repliedAt: string;
}

/**
 * 一則公開商品評論 —— `gateway/internal/handler/review.go` 的 `ReviewListItem`。
 * wire 欄名（全 camelCase，backend#2235）：
 * `reviewId / buyerId / productId / username / avatarKey / avatarUrl /
 *  rating / content / createdAt / imageKeys / imageUrls / reply`。
 *
 * 這個端點**不帶** likeCount / replyCount（只有 `/app/reviews/{id}` 的
 * `ReviewDetail` 有），所以這裡不宣告，免得有人照著讀到 undefined。
 */
export interface Review {
	id: string;
	productId: string;
	buyerId: string;
	username: string;
	/** Buyer avatar objectKey (wire `avatarKey`, 49 §6.1 SSOT). */
	avatarKey?: string;
	rating: number;
	content: string;
	/** 評論附圖 objectKey 陣列（wire `imageKeys`）。 */
	imageKeys: string[];
	/** Seller reply; `undefined` = not yet replied (drives the 已回/未回 state). */
	reply?: ReviewReply;
	createdAt: number;
}

/** `data.stats` —— review.go:164-171 `ReviewStatsBlock`。零值不再被 omitempty
 *  吃掉（backend#2235），所以每個欄位都一定在 wire 上，`0` 就是真的 0。 */
export interface ReviewStats {
	totalCount: number;
	averageRating: number;
	rating5Count: number;
	rating4Count: number;
	rating3Count: number;
	rating2Count: number;
	rating1Count: number;
	withImagesCount: number;
}

export interface ReviewListResult {
	reviews: Review[];
	stats: ReviewStats;
	page: number;
	pageSize: number;
	total: number;
}

/**
 * Submit-review payload — mirrors the APP's `ReviewRepository.addReview`
 * (`POST /app/reviews`). Photos are uploaded first via `uploadImageFile` and
 * passed here as `stagedTokens` (max 4); the backend finalizes them.
 */
export interface SubmitOrderItemReviewBody {
	orderNo: string;
	orderItemId: number | string;
	productId?: number | string;
	/** Product/order-item snapshot fields accepted by the review backend. */
	skuName?: string;
	/** 被評論商品的封面顯示 URL（wire `productImageUrl`）。這裡刻意保留
	 *  `product` 前綴 —— 同一個物件上的 `imageKeys` 是買家上傳的評論附圖，
	 *  收斂成 `imageUrl` 會讓單數與複數指涉兩件不同的事。 */
	productImageUrl?: string;
	price?: number | string;
	rating: number;
	content?: string;
	/** Up to 4 stagedTokens from `uploadImageFile`. */
	stagedTokens?: string[];
}

/** Max review photos the backend accepts. */
export const REVIEW_PHOTO_LIMIT = 4;

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object" && !Array.isArray(value)
		? (value as Record<string, unknown>)
		: {};
}

function pickString(
	source: Record<string, unknown>,
	keys: readonly string[],
): string | undefined {
	for (const key of keys) {
		const value = source[key];
		if (typeof value === "string" && value.trim()) return value;
		if (typeof value === "number") return String(value);
	}
	return undefined;
}

// pickNestedString 隨欄名收斂一併移除：它存在的理由是「同一個值可能掛在
// 好幾層不同的父鍵下」，而 backend#2235 之後每個欄位只有一個位置一個名字。
function normalizeTimestamp(value: unknown): number {
	if (typeof value === "number") return value;
	if (typeof value === "string") {
		const numeric = Number(value);
		if (Number.isFinite(numeric)) return numeric;
		const parsed = Date.parse(value);
		if (Number.isFinite(parsed)) return parsed;
	}
	return 0;
}

function normalizeReply(value: unknown): ReviewReply | undefined {
	const raw = asRecord(value);
	const content = pickString(raw, ["content"]);
	if (!content) return undefined;
	return { content, repliedAt: pickString(raw, ["repliedAt"]) ?? "" };
}

function normalizeReview(value: unknown): Review {
	const raw = asRecord(value);
	// §231: read the bare objectKey (SSOT) only — the wire's paired `avatarUrl`
	// is composed FROM it, so falling back to the URL could only ever mask a
	// contract break. Avatar/buildImageUrl resolve the key.
	const avatarKey = pickString(raw, ["avatarKey"]);
	// §231: `imageKeys` (bare keys) is the SSOT and the only photo array on the
	// wire — `/public/products/{id}/reviews` emits `imageKeys` + the derived
	// `imageUrls` twin and never an `images` field.
	const imageSource = Array.isArray(raw.imageKeys) ? raw.imageKeys : [];
	return {
		// Public product reviews key it `reviewId` (the seller aggregate at
		// `/app/sellers/me/reviews` uses `id`); read both so the id is never empty
		// — an empty id collides React keys and breaks per-review actions.
		id: pickString(raw, ["id", "reviewId"]) ?? "",
		productId: pickString(raw, ["productId"]) ?? "",
		buyerId: pickString(raw, ["buyerId"]) ?? "",
		username: pickString(raw, ["username"]) ?? "匿名",
		avatarKey,
		rating: Number(raw.rating ?? 0),
		content: String(raw.content ?? ""),
		imageKeys: imageSource.filter(
			(img): img is string => typeof img === "string",
		),
		reply: normalizeReply(raw.reply),
		createdAt: normalizeTimestamp(raw.createdAt),
	};
}

function normalizeStats(value: unknown): ReviewStats {
	const raw = asRecord(value);
	const n = (key: string) => Number(raw[key] ?? 0) || 0;
	return {
		totalCount: n("totalCount"),
		averageRating: n("averageRating"),
		rating5Count: n("rating5Count"),
		rating4Count: n("rating4Count"),
		rating3Count: n("rating3Count"),
		rating2Count: n("rating2Count"),
		rating1Count: n("rating1Count"),
		withImagesCount: n("withImagesCount"),
	};
}

export async function fetchProductReviews(
	productId: number | string,
	{
		page = 1,
		pageSize = 10,
		rating,
	}: {
		page?: number;
		pageSize?: number;
		/** Exact star filter (1–5), not a minimum — the gateway matches on equality. */
		rating?: number;
	} = {},
): Promise<ReviewListResult> {
	// Gateway reads `size` / `rating` (review.go GetProductReviews). A query name
	// it doesn't recognise is dropped silently — no 400, just the default page
	// size and an inert star filter — so check the handler before renaming these.
	const query: Record<string, string> = {
		page: String(page),
		size: String(pageSize),
	};
	if (rating) query.rating = String(rating);
	// Public endpoint, anonymous → omit. softFailNotFound: a 404 product yields a
	// cacheable empty list, but a 5xx / network error / unparseable body THROWS so
	// the sole caller getCachedProductReviews keeps its prior stale page instead of
	// caching an outage-time empty list (ReviewsAsync's .catch(prerenderSafe) still
	// guards the render). throwOnEnvelopeError off preserves the prior "read data
	// regardless of envelope code" behaviour.
	//
	// Envelope（backend#2211 C8）：records / total / page / pageSize / stats 全部
	// 在 `data` 之下，三個數值欄是 string。
	const envelope = await apiClient.public.get<unknown>(
		`/products/${encodeURIComponent(String(productId))}/reviews`,
		{
			query,
			credentials: "omit",
			throwOnEnvelopeError: false,
			softFailNotFound: true,
		},
	);
	const nested = asRecord(envelope.data);
	// Number() 包裝不能拿掉：total / page / pageSize 是 string。
	const reviewsRaw: unknown[] = Array.isArray(nested.records)
		? (nested.records as unknown[])
		: [];
	return {
		reviews: reviewsRaw.map(normalizeReview),
		stats: normalizeStats(nested.stats),
		page: Number(nested.page ?? page),
		pageSize: Number(nested.pageSize ?? pageSize),
		total: Number(nested.total ?? 0),
	};
}

/** One review the buyer has written (GET /app/me/reviews — per order item).
 *  Wire shape = `gateway/internal/handler/order_review.go` `OrderReviewRecord`. */
export interface MyReview {
	id: string;
	/** Empty when backend omits it → card renders non-clickable. */
	orderNo: string;
	orderItemId: string;
	productId: string;
	rating: number;
	content: string;
	/** 買家上傳的評論附圖 objectKey 陣列（wire `imageKeys`）。 */
	imageKeys: string[];
	skuName: string;
	/** 被評論商品的封面 objectKey（wire `productImageKey`）。`product` 前綴刻意
	 *  保留：同物件上的 `imageKeys` 是評論附圖，兩者不是同一件事。 */
	productImageKey: string;
	/** Unit price in 元, as a string (matches backend FormatInt). */
	price: string;
	createdAt: number;
}

export interface MyReviewListResult {
	records: MyReview[];
	page: number;
	pageSize: number;
	total: number;
}

function normalizeMyReview(value: unknown): MyReview {
	const raw = asRecord(value);
	// §231: `imageKeys` is the objectKey SSOT. The paired `imageUrls` is
	// `mediaurl.BuildURL(base, imageKeys…)` (order_review.go:188-191) — composed
	// from these very keys, so it can never carry a photo the keys lack.
	const rawImages = raw.imageKeys;
	return {
		id: pickString(raw, ["id"]) ?? "",
		orderNo: pickString(raw, ["orderNo"]) ?? "",
		orderItemId: pickString(raw, ["orderItemId"]) ?? "",
		productId: pickString(raw, ["productId"]) ?? "",
		rating: Number(raw.rating ?? 0),
		content: String(raw.content ?? ""),
		imageKeys: Array.isArray(rawImages)
			? rawImages.filter((img): img is string => typeof img === "string")
			: [],
		skuName: pickString(raw, ["skuName"]) ?? "",
		// §231: the order-item snapshot is a flat `productImageKey` (objectKey
		// SSOT) + `productImageUrl` pair (order_review.go:182-183); the caller
		// feeds the key into buildImageUrl.
		productImageKey: pickString(raw, ["productImageKey"]) ?? "",
		price: pickString(raw, ["price"]) ?? "",
		createdAt: normalizeTimestamp(raw.createdAt),
	};
}

/** GET /app/me/reviews — reviews the logged-in buyer has written (paged). */
export async function fetchMyReviews({
	page = 1,
	pageSize = 20,
}: {
	page?: number;
	pageSize?: number;
} = {}): Promise<MyReviewListResult> {
	const { data } = await apiClient.authed.get<{
		records?: unknown[];
		page?: number | string;
		pageSize?: number | string;
		total?: number | string;
	}>("/me/reviews", {
		query: { page: String(page), size: String(pageSize) },
	});
	const d = data ?? {};
	return {
		records: (d.records ?? []).map(normalizeMyReview),
		page: Number(d.page ?? page),
		pageSize: Number(d.pageSize ?? pageSize),
		total: Number(d.total ?? 0),
	};
}

export async function submitOrderItemReview(
	body: SubmitOrderItemReviewBody,
): Promise<void> {
	const price =
		body.price != null && body.price !== "" ? Number(body.price) : undefined;
	await authedFetch("/app/reviews", {
		method: "POST",
		json: {
			orderNo: body.orderNo,
			orderItemId: String(body.orderItemId),
			productId:
				body.productId != null && body.productId !== ""
					? String(body.productId)
					: undefined,
			skuName: body.skuName?.trim() || undefined,
			productImageUrl: body.productImageUrl?.trim() || undefined,
			price: Number.isFinite(price) ? price : undefined,
			rating: body.rating,
			content: body.content,
			stagedTokens: body.stagedTokens?.length ? body.stagedTokens : undefined,
		},
	});
}

export async function replyReview(
	reviewId: number | string,
	content: string,
): Promise<void> {
	await authedFetch(
		`/app/sellers/reviews/${encodeURIComponent(String(reviewId))}/reply`,
		{ method: "POST", json: { content } },
	);
}
