/**
 * PDP layout props derive helper（pure function）。
 *
 * Server PDP（`ProductDetailPage`）與賣家批次匯入預覽 PDP（`PreviewDetailPage`）
 * 共用同一條 render path：兩者拿到 product / seller / reviews 之後，全部走本
 * helper 展平成 `ProductDetailLayout` 接得到的 props，避免 adapter 自己又翻譯
 * 一次資料形狀（會塞 placeholder 字串 / 假 origin / 假 unit）。
 *
 * 設計重點：
 *   - 純函式：無 async、無 fetch、無 React。
 *   - 缺欄位就是空 — 不寫死 placeholder（如「我的賣場」「預覽」「件」「—」）。
 *   - 與正式 PDP 取不到 dims 時行為一致：descriptionImageDims 空 Map 時
 *     layout 對 image block 走 fallback `<img>`，不爆。
 */

import type { PurchaseBoxSku } from "@/features/product/components/ProductPurchaseBox";
import { formatUnitLabel, ShippingMethod } from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import type { ProductReview } from "./ProductDetailLayout";

export type { ProductReview };

export interface DeriveInput {
	product: Record<string, unknown>;
	seller: unknown;
	sellerStats: unknown;
	reviewsResult: { reviews: unknown[]; total: number } | null;
	/**
	 * pair 內第一個元素必須與 product.description.blocks 內的 image block 同
	 * reference（descriptionImageDims Map key 用 object identity 比對）。
	 */
	descriptionImageDimEntries: ReadonlyArray<
		readonly [Record<string, unknown>, { width: number; height: number } | null]
	>;
}

export interface DerivedLayoutProps {
	product: Record<string, unknown>;
	seller: unknown;
	sellerStats: unknown;
	productSeller: unknown;
	reviews: ProductReview[];
	reviewTotal: number;
	purchasable: boolean;
	descriptionImageDims: Map<
		Record<string, unknown>,
		{ width: number; height: number } | null
	>;
	thumbnails: string[];
	skus: PurchaseBoxSku[];
	enabledCarriers: ReadonlyArray<{
		method: string;
		label: string;
	}>;
	shippingNote: string;
	categoryName: string;
	categoryTrail: string[];
	sellerStoreId: string;
	purchaseUnit: string;
	descriptionBlocks: Array<Record<string, unknown>>;
}

/* ============================================================
 * 小工具（與 ProductDetailLayout 內部 helpers 同邏輯）
 * ============================================================ */

export function textOrEmpty(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

export function textOrNumber(value: unknown): string {
	if (typeof value === "string") return value.trim();
	if (typeof value === "number" && Number.isFinite(value)) return String(value);
	return "";
}

export function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

export function descriptionImageRaw(block: Record<string, unknown>): string {
	// description.blocks[] 的圖片欄是 imageKey/imageUrl（gateway
	// seller_product.go:200 與 product.go:434，backend#2235 起與 gallery 同名）。
	return textOrEmpty(block.imageUrl) || textOrEmpty(block.imageKey);
}

export function getProductSellerStoreId(product: unknown): string {
	const productRecord = asRecord(product);
	const sellerRecord = asRecord(productRecord.seller);
	return textOrEmpty(
		sellerRecord.storeId ||
			productRecord.sellerAccountId ||
			productRecord.storeId ||
			productRecord.createdBy,
	);
}

function getProductCategoryName(product: Record<string, unknown>): string {
	return (
		textOrEmpty(product.categoryLv3Name) ||
		textOrEmpty(product.categoryLv2Name) ||
		textOrEmpty(product.categoryLv1Name) ||
		textOrEmpty(product.categoryName) ||
		"未分類"
	);
}

function getProductCategoryTrail(product: Record<string, unknown>): string[] {
	const trail = [
		textOrEmpty(product.categoryLv1Name),
		textOrEmpty(product.categoryLv2Name),
		textOrEmpty(product.categoryLv3Name),
	].filter(Boolean);
	if (trail.length > 0) return trail;
	const flat = textOrEmpty(product.categoryName);
	return flat ? [flat] : [];
}

/**
 * Carrier mini-spec：對應到的 ShippingMethod enum 值 + 短 label。
 * layout 只負責 render。
 */
export const CARRIER_META: ReadonlyArray<{
	method: string;
	label: string;
}> = [
	{
		method: ShippingMethod.TAQBIN,
		label: "宅配",
	},
	{
		method: ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
		label: "店到店",
	},
];

const KNOWN_METHOD_CODES = new Set<string>([
	ShippingMethod.TAQBIN,
	ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
]);

function pickMethodCode(rec: Record<string, unknown>): string {
	const candidates = [
		rec.shippingMethod,
		rec.method,
		rec.methodCode,
		rec.shippingMethodCode,
		rec.code,
		rec.id,
		rec.type,
	];
	for (const v of candidates) {
		const s = textOrEmpty(v);
		if (s && KNOWN_METHOD_CODES.has(s)) return s;
	}
	return "";
}

function extractEnabledMethods(
	product: Record<string, unknown>,
	seller: unknown,
): Set<string> {
	const sellerRec = asRecord(seller);
	const arrayCandidates: unknown[] = [
		product.shippingMethods,
		product.shippingTypes,
		product.allowedShippingMethods,
		product.deliveryMethods,
		sellerRec.shippingMethods,
		sellerRec.shippingOnboardings,
		sellerRec.enabledShippingMethods,
	];

	const out = new Set<string>();
	for (const arr of arrayCandidates) {
		if (!Array.isArray(arr)) continue;
		for (const item of arr) {
			if (typeof item === "string" || typeof item === "number") {
				const s = String(item);
				if (KNOWN_METHOD_CODES.has(s)) out.add(s);
				continue;
			}
			const code = pickMethodCode(asRecord(item));
			if (code) out.add(code);
		}
	}
	return out;
}

function shippingNoteFromMethods(methods: unknown): string {
	if (!Array.isArray(methods) || methods.length === 0) return "";
	const records = methods.map((m) => asRecord(m));
	const allSellerPaid = records.every((m) => m.feePayer === "seller");
	if (allSellerPaid) return "賣家負擔運費";
	const thresholds = records
		.filter((m) => m.feePayer !== "seller")
		.map((m) => Number(m.freeThreshold))
		.filter((n) => Number.isFinite(n) && n > 0);
	if (thresholds.length === 0) return "";
	const min = Math.min(...thresholds);
	return `滿 NT$ ${min.toLocaleString()} 免運`;
}

/**
 * Normalise the live reviews endpoint payload (`getCachedProductReviews`) into
 * the `ProductReview` shape the layout renders. Shared by the synchronous derive
 * (preview path) and the streamed `ReviewsAsync` server component so the two
 * never drift in how they read username/avatar/createdAt.
 */
export function mapLiveReviews(
	reviewsResult: { reviews: unknown[]; total: number } | null,
): { reviews: ProductReview[]; total: number } {
	const total = reviewsResult?.total ?? 0;
	const reviews: ProductReview[] = (reviewsResult?.reviews ?? []).map(
		(rvRaw) => {
			const rv = asRecord(rvRaw);
			const createdRaw = rv.createdAt as number | undefined;
			return {
				id: rv.id as number | string | undefined,
				author: textOrEmpty(rv.username),
				// `reviewsResult` is already normalized by lib/reviews/repo.ts
				// (fetchProductReviews → normalizeReview). Since backend#2235 the
				// domain type mirrors the wire field-for-field, so these names are
				// the same on both sides of that boundary.
				avatar: textOrEmpty(rv.avatarKey),
				rating: parseRating(rv.rating) ?? 0,
				text: textOrEmpty(rv.content),
				// Photo list is `imageKeys` (bare objectKeys) on both the wire and
				// the normalized shape; downstream renders via buildImageUrl.
				photos: Array.isArray(rv.imageKeys) ? (rv.imageKeys as unknown[]) : [],
				created_at: createdRaw,
				// #374：這個字串在 server component（ReviewsAsync）算完才序列化下傳，
				// 因此不會有 hydration mismatch，但 Workers runtime 是 UTC ——
				// 用 `toLocaleDateString` 等於讓全部使用者看 UTC 日期，台北使用者在
				// 16:00Z 之後的跨日窗口會看到差一天。固定台北偏移才是正確日期。
				date: formatTaipeiDate(createdRaw),
			};
		},
	);
	return { reviews, total };
}

/* ============================================================
 * Entry
 * ============================================================ */

export function deriveDetailLayoutProps(
	input: DeriveInput,
): DerivedLayoutProps {
	const { product, seller, sellerStats, reviewsResult } = input;

	const sellerStoreId = getProductSellerStoreId(product);

	// description blocks：原物件 reference 保留，layout 端的 descriptionImageDims
	// Map 必須用同一份物件 lookup（identity key）。
	const rawDescriptionBlocks: unknown[] = Array.isArray(
		(product.description as Record<string, unknown> | undefined)?.blocks,
	)
		? ((product.description as Record<string, unknown>).blocks as unknown[])
		: [];
	const descriptionBlocks = rawDescriptionBlocks.map(asRecord);

	const descriptionImageDims = new Map(input.descriptionImageDimEntries);

	// 縮圖：seller 沒上傳多角度 → 退回單張 `imageKey`；都無 → 走下面的 gallery。
	// §231：`imageKeys`/`imageKey`（裸 objectKey）是 SSOT，成對的
	// `imageUrls`/`imageUrl` 是 `mediaurl.BuildURL(base, imageKey…)`
	// （gateway product.go:1119-1123）——由同一把 key 推導出來的，不可能補上 key
	// 沒有的圖，所以不讀；舊單欄 `images` 後端也從來沒回過。
	const asStringArray = (v: unknown): string[] =>
		Array.isArray(v)
			? (v as unknown[]).filter(
					(s: unknown): s is string => typeof s === "string" && s.length > 0,
				)
			: [];
	const thumbnails: string[] = [...asStringArray(product.imageKeys)];
	if (thumbnails.length === 0 && typeof product.imageKey === "string") {
		if (product.imageKey) thumbnails.push(product.imageKey);
	}
	// gallery 條目。**這條 derive 同時服務兩個來源**，兩者的圖片欄名不同，
	// 而且不同是刻意的（backend#2235）：
	//   - 正式商品（handler.GalleryItem / PublicGalleryItem）→ `imageKey`，
	//     是平台 objectKey，下游可以套 preset。
	//   - 匯入預覽（handler.GalleryItemResponse）→ `previewUrl`，是 presigned
	//     或原站 URL，沒有對應 objectKey（圖還在 tmp/product-import/ 前綴），
	//     套 preset 會 404。
	// 讀兩個名字在這裡不是 fallback，是兩個端點；`buildImageUrl` 對 http 開頭
	// 的值 passthrough，所以兩者都渲染得出來。
	if (thumbnails.length === 0 && Array.isArray(product.gallery)) {
		for (const g of product.gallery as unknown[]) {
			const rec = asRecord(g);
			const src = textOrEmpty(rec.imageKey) || textOrEmpty(rec.previewUrl);
			if (src) thumbnails.push(src);
		}
	}
	// Intentionally left empty when the product has no imagery — ProductGallery
	// renders the design-system ImageFallback (folder-image) for an empty
	// gallery instead of a branded stand-in.

	// SKU 組合（handler.SkuItem 形狀）。
	const toPositiveNumber = (value: unknown): number => {
		if (value == null || value === "") return 0;
		const n = typeof value === "number" ? value : Number(value);
		return Number.isFinite(n) && n > 0 ? n : 0;
	};

	const rawSkus: unknown[] = Array.isArray(product.skus)
		? (product.skus as unknown[])
		: [];
	const skus: PurchaseBoxSku[] = rawSkus.map((rawSku, i) => {
		const s = asRecord(rawSku);
		const regularPrice = toPositiveNumber(s.price);
		const salePrice = toPositiveNumber(s.salePrice);
		// 商品詳情是 `listPrice`（handler.SkuItem），賣家草稿轉出的是
		// `originalPrice`（productFormAdapter）——兩個真實來源，不是新舊退路。
		const originalPrice = toPositiveNumber(s.listPrice ?? s.originalPrice);

		const onSale = salePrice > 0 && salePrice < regularPrice;
		const effectivePrice = onSale ? salePrice : regularPrice;
		const list = onSale
			? Math.max(regularPrice, originalPrice)
			: originalPrice > regularPrice
				? originalPrice
				: 0;

		// 兩個來源（商品詳情 handler.SkuItem、匯入預覽 ImportItemSkuResponse）
		// 都是 `skuName` / `stockQty`（seller_product_import.go:291,294）。
		// 這裡曾寫著「preview_data 走 snake_case」——該前提不成立，snake 分支
		// 從來沒有被走到過。
		const stockNum = Number.parseInt(textOrEmpty(s.stockQty), 10);
		const nameStr = typeof s.skuName === "string" ? s.skuName.trim() : "";
		const fallbackName = rawSkus.length === 1 ? "預設" : `規格 ${i + 1}`;

		return {
			id: textOrNumber(s.id) || String(i),
			name: nameStr || fallbackName,
			price: effectivePrice,
			listPrice: list > effectivePrice ? list : undefined,
			unit: formatUnitLabel(textOrEmpty(s.unit)) || undefined,
			stock: Number.isFinite(stockNum) && stockNum >= 0 ? stockNum : undefined,
		};
	});

	const { reviews: liveReviews, total: liveReviewTotal } =
		mapLiveReviews(reviewsResult);
	const rawProductReviews: unknown[] = Array.isArray(product.reviews)
		? (product.reviews as unknown[])
		: [];
	const productReviews: ProductReview[] = rawProductReviews.map(
		(review) => asRecord(review) as ProductReview,
	);
	const reviews: ProductReview[] =
		liveReviews.length > 0 ? liveReviews : productReviews;
	// 標題評論數優先 product.reviewCount（#1156 後端聚合已可靠）：SSR 首屏 reviewsResult
	// 尚未載入（liveReviewTotal=0）時，過去退回內嵌 reviews.length（detail 內嵌 reviews 為空 → 0），
	// 現改用權威的 product.reviewCount，避免「★4.8 (0則評價)」不一致。
	const reviewTotal = Object.hasOwn(product, "reviewCount")
		? parseReviewCount(product.reviewCount)
		: liveReviewTotal > 0
			? liveReviewTotal
			: reviews.length;
	const purchasable = product.purchasable !== false;

	const firstSkuRecord = asRecord(rawSkus[0]);
	const firstSkuUnit = formatUnitLabel(
		textOrEmpty(firstSkuRecord.unit) || undefined,
	);
	// 與正式 PDP 行為一致：缺 SKU unit 時 fallback「件」（ProductPurchaseBox
	// 預設值對齊）。注意 — 這是「商品詳情頁購買盒」的 unit fallback，與「商城
	// 列表卡 origin/unit」的寫死 placeholder 是兩回事；後者預覽不該注入。
	const purchaseUnit =
		typeof firstSkuUnit === "string" && firstSkuUnit ? firstSkuUnit : "件";
	const categoryName = getProductCategoryName(product);
	const categoryTrail = getProductCategoryTrail(product);
	const shippingNote = shippingNoteFromMethods(product.shippingMethods);
	const enabledCarrierMethods = extractEnabledMethods(product, seller);
	const enabledCarriers = CARRIER_META.filter((c) =>
		enabledCarrierMethods.has(c.method),
	);

	return {
		product,
		seller,
		sellerStats,
		productSeller: product.seller,
		reviews,
		reviewTotal,
		purchasable,
		descriptionImageDims,
		thumbnails,
		skus,
		enabledCarriers,
		shippingNote,
		categoryName,
		categoryTrail,
		sellerStoreId,
		purchaseUnit,
		descriptionBlocks,
	};
}
