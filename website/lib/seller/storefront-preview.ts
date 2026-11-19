import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { ProductRecord } from "@/lib/market/listFilters";
import {
	PRODUCT_STATUS,
	type SellerProductDetail,
	type SellerProductListItem,
} from "@/lib/products/seller-repo";
import { type PublicStoreView, storeViewOf } from "./public-store";
import { hasCapability, type MyStore } from "./store";

/** 店家此刻能不能公開曝光：`GetMyStore.capabilities` 的 `PUBLIC_LISTING` 啟用。沒有回報算不能。 */
export function storeCanBePublic(my: MyStore | null | undefined): boolean {
	return hasCapability(my, SellerCapability.PUBLIC_LISTING);
}

/** 店主端的店家與統計 → 店面頁頂部元件用的形狀（與公開店面同一個型別）。沒有店家回 null。 */
export function storefrontViewOfMyStore(
	my: MyStore | null | undefined,
): PublicStoreView | null {
	const store = my?.store;
	if (!store?.id) return null;
	return storeViewOf(
		{
			id: store.id,
			storeCode: store.storeCode,
			storeName: store.storeName,
			description: store.description,
			prize: store.prize,
			websiteUrl: store.websiteUrl,
			logo: store.logo,
			coverImage: store.coverImage,
			townCode: store.address?.townCode ?? "",
		},
		my?.stats,
	);
}

const PREVIEW_NOTICE_PUBLIC =
	"使用正式公開商店資料；預覽期間不會送出訂閱、聊天、收藏、按讚、留言、分享或購買操作。";
const PREVIEW_NOTICE_NOT_PUBLIC =
	"你的店家還沒公開，買家目前看不到這個店面。以下是公開後買家會看到的樣子。";

/** 預覽橫幅的說明：店家還不能公開時改說明買家目前看不到。 */
export function previewNoticeText(canBePublic: boolean): string {
	return canBePublic ? PREVIEW_NOTICE_PUBLIC : PREVIEW_NOTICE_NOT_PUBLIC;
}

export const PREVIEW_REVIEWS_UNAVAILABLE =
	"店家公開後，買家的評價會顯示在這裡。";
export const PREVIEW_MESSAGES_UNAVAILABLE =
	"店家公開後，買家的迴響會顯示在這裡。";

/** 預覽只列上架中的商品：狀態碼是 CodeMeta 字串，也接受數字形式。 */
export function isOnSaleProduct(item: SellerProductListItem): boolean {
	return String(item.status ?? "") === PRODUCT_STATUS.ON;
}

/** 店主端商品列表項 → 市集卡片吃的 `ProductRecord`。圖片是 objectKey，賣家資訊取自店面。 */
export function previewProductRecord(
	item: SellerProductListItem,
	store: PublicStoreView,
): ProductRecord {
	return {
		id: item.id,
		name: item.name,
		imageKey: item.image_url ?? item.main_image,
		priceMin: item.price_min,
		priceMax: item.price_max,
		listPriceMin: item.list_price_min,
		listPriceMax: item.list_price_max,
		salePriceMin: item.sale_price_min,
		salePriceMax: item.sale_price_max,
		price: item.price,
		origin: item.origin,
		categoryName: item.category_name,
		categoryId: item.category_id,
		hashtags: item.hashtags as ProductRecord["hashtags"],
		rating: item.rating,
		stock: item.stock,
		salesCount: item.sales_count,
		favoriteCount: item.favorite_count,
		createdAt: item.created_at,
		storeId: store.storeId,
		seller: {
			name: store.name,
			storeId: store.storeId,
			storeCode: store.storeCode,
			logoUrl: store.logo_url,
		},
	};
}

/** 商品詳情的賣家卡讀的統計：評價數與評分、粉絲數；店面統計讀不到就不設。 */
export function previewStoreStats(store: PublicStoreView): {
	sellerStats: { avgRating: number; totalCount: number } | null;
	followerStats: { followers: number } | null;
} {
	return {
		sellerStats:
			store.ratingAverage !== undefined && store.reviewCount !== undefined
				? { avgRating: store.ratingAverage, totalCount: store.reviewCount }
				: null,
		followerStats:
			store.followerCount !== undefined
				? { followers: store.followerCount }
				: null,
	};
}

interface WireShippingMethod {
	type?: unknown;
	feePayer?: unknown;
	freeThreshold?: unknown;
}

/**
 * 店主端商品詳情（`GET /app/products/{id}`）→ 商品頁 layout 讀的商品。欄名與買家商品頁相同
 * （`gallery`、`skus[].skuName`／`stockQty`、`description.blocks`），不另做改名；商品層的配送方式
 * 由各 SKU 的配送方式聯集而來，賣家卡的資料取自店面（不在商品詳情裡）。
 */
export function previewDetailProduct(
	detail: SellerProductDetail,
	store: PublicStoreView,
): Record<string, unknown> {
	const record = detail as unknown as Record<string, unknown>;
	const skus = Array.isArray(record.skus)
		? (record.skus as Array<{ shippingMethods?: WireShippingMethod[] }>)
		: [];
	const seen = new Set<string>();
	const shippingMethods: WireShippingMethod[] = [];
	for (const sku of skus) {
		for (const method of sku.shippingMethods ?? []) {
			const key = [method.type, method.feePayer, method.freeThreshold].join(
				"|",
			);
			if (seen.has(key)) continue;
			seen.add(key);
			shippingMethods.push(method);
		}
	}
	return {
		...record,
		shippingMethods: Array.isArray(record.shippingMethods)
			? record.shippingMethods
			: shippingMethods,
		storeId: store.storeId,
		seller: {
			storeId: store.storeId,
			storeCode: store.storeCode,
			name: store.name,
			logoUrl: store.logo_url,
			city: store.city,
			district: store.district,
		},
	};
}
