/**
 * PDP layout props derive helper（pure function）。
 *
 * Server PDP（`ProductDetailPage`）與代行預覽 PDP
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
import type { Review } from "@/lib/reviews/repo";
import type { ProductReview } from "./ProductDetailLayout";

export type { ProductReview };

export interface DeriveInput {
	product: Record<string, unknown>;
	seller: unknown;
	sellerStats: unknown;
	reviewsResult: { reviews: Review[]; total: number } | null;
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
	reviewsResult: { reviews: Review[]; total: number } | null,
): { reviews: ProductReview[]; total: number } {
	const total = reviewsResult?.total ?? 0;
	const reviews: ProductReview[] = (reviewsResult?.reviews ?? []).map((rv) => ({
		id: rv.id,
		author: rv.username,
		isMine: rv.isMine,
		avatar: rv.avatarUrl ?? "",
		rating: rv.rating,
		text: rv.content,
		photos: rv.imageUrls,
		created_at: rv.createdAt,
		date: formatTaipeiDate(rv.createdAt),
	}));
	return { reviews, total };
}

export function deriveDetailLayoutProps(
	input: DeriveInput,
): DerivedLayoutProps {
	const { product, seller, sellerStats, reviewsResult } = input;

	const sellerStoreId = getProductSellerStoreId(product);

	const rawDescriptionBlocks: unknown[] = Array.isArray(
		(product.description as Record<string, unknown> | undefined)?.blocks,
	)
		? ((product.description as Record<string, unknown>).blocks as unknown[])
		: [];
	const descriptionBlocks = rawDescriptionBlocks.map(asRecord);

	const descriptionImageDims = new Map(input.descriptionImageDimEntries);

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
	if (thumbnails.length === 0 && Array.isArray(product.gallery)) {
		for (const g of product.gallery as unknown[]) {
			const rec = asRecord(g);
			const src = textOrEmpty(rec.imageKey);
			if (src) thumbnails.push(src);
		}
	}

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
		const originalPrice = toPositiveNumber(s.listPrice ?? s.originalPrice);

		const onSale = salePrice > 0 && salePrice < regularPrice;
		const effectivePrice = onSale ? salePrice : regularPrice;
		const list = onSale
			? Math.max(regularPrice, originalPrice)
			: originalPrice > regularPrice
				? originalPrice
				: 0;

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
