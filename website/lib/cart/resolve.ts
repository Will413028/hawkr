import { apiClient } from "@/lib/api/client";
import type { CartInput, CartItem } from "@/lib/cart";

type ProductSkuRecord = {
	id?: string | number;
	skuName?: string;
	name?: string;
	price?: string | number;
	salePrice?: string | number;
	sale_price?: string | number;
	listPrice?: string | number;
	list_price?: string | number;
	originalPrice?: string | number;
	original_price?: string | number;
	stockQty?: string | number;
};

type ProductDetailRecord = {
	id?: string | number;
	name?: string;
	/** §231 flat-pair: `imageKey`/`imageKeys` (bare objectKeys) are the SSOT.
	 *  The `imageUrl` twin is `mediaurl.BuildURL(base, imageKey)` — derived from
	 *  the key we already read, so it is not declared and must not return as a
	 *  fallback. */
	imageKey?: string;
	imageKeys?: string[];
	status?: string;
	sellerName?: string;
	seller?: {
		name?: string;
		logoUrl?: string;
		storeId?: string | number;
	};
	storeId?: string | number;
	skus?: ProductSkuRecord[];
};

function toPositiveNumber(value: unknown): number {
	if (value == null || value === "") return 0;
	const n = typeof value === "number" ? value : Number(value);
	return Number.isFinite(n) && n > 0 ? n : 0;
}

function effectiveSkuPrice(sku: ProductSkuRecord): number {
	const regular = toPositiveNumber(sku.price);
	const sale = toPositiveNumber(sku.salePrice ?? sku.sale_price);
	return sale > 0 && sale < regular ? sale : regular;
}

async function fetchProductDetailForCart(
	productId: string | number,
): Promise<ProductDetailRecord | null> {
	// Anonymous read → omit. throwOn*:false mirrors the prior "any non-OK → null"
	// (a missing/unavailable product just means "no purchasable SKU" downstream).
	const { data } = await apiClient.public.get<ProductDetailRecord>(
		`/products/${encodeURIComponent(String(productId))}`,
		{
			credentials: "omit",
			throwOnHttpError: false,
			throwOnEnvelopeError: false,
		},
	);
	return data ?? null;
}

/**
 * Product cards only know the product id, while `/public/carts/items`
 * requires a real SKU id. Resolve the first purchasable SKU before writing
 * local cart state so the cart can immediately validate shipping.
 */
export async function resolveCartInputSku(
	input: CartInput,
): Promise<CartInput> {
	if (input.skuId && input.productId != null) return input;

	const productId = input.productId ?? input.id;
	const product = await fetchProductDetailForCart(productId);
	const sku = product?.skus?.find((candidate) => candidate.id != null);

	if (!product || !sku?.id) {
		throw new Error("此商品目前沒有可購買規格，請進入商品頁確認後再加入購物車");
	}

	const skuId = String(sku.id);
	const resolvedProductId = product.id ?? productId;
	const price = effectiveSkuPrice(sku);

	return {
		...input,
		id: skuId,
		skuId,
		productId: resolvedProductId,
		name: input.name || product.name || "商品",
		price: price > 0 ? String(price) : input.price,
		image: input.image || product.imageKey || product.imageKeys?.[0],
		unit: sku.skuName || sku.name || input.unit,
		status: product.status ?? input.status,
		sellerName: input.sellerName || product.seller?.name || product.sellerName,
		sellerStoreId:
			input.sellerStoreId ||
			(product.seller?.storeId != null
				? String(product.seller.storeId)
				: product.storeId != null
					? String(product.storeId)
					: undefined),
		sellerLogoUrl:
			input.sellerLogoUrl || product.seller?.logoUrl?.trim() || undefined,
		href: input.href || `/products/${resolvedProductId}`,
	};
}

export async function resolveLegacyCartItem(
	item: CartItem,
): Promise<CartInput | null> {
	try {
		return await resolveCartInputSku({
			id: item.productId != null ? String(item.productId) : item.id,
			skuId: item.productId != null ? item.skuId : undefined,
			productId: item.productId,
			name: item.name,
			price: item.price,
			image: item.image,
			unit: item.unit,
			href: item.href,
			status: item.status,
			sellerName: item.sellerName,
			sellerStoreId: item.sellerStoreId,
			sellerLogoUrl: item.sellerLogoUrl,
			quantity: item.quantity,
		});
	} catch {
		return null;
	}
}
