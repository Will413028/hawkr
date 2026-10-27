/**
 * 貼文編輯器「目前上架商品」挑選器的 wire → view 轉換。
 *
 * 來源：`GET /public/sellers/{id}/products`，wire 形狀由
 * `handler.ProductItem`（gateway/internal/handler/product.go:176-216）定義。
 * 實際欄位只有 `id` / `name` / `imageKey` / `imageUrl` / `imageKeys` /
 * `imageUrls` / `priceMin` / `priceMax` / `listPrice*` / `salePrice*` /
 * `skuNames` … 等。
 *
 * 這支以前讀 `skuId` / `skuName` / `productName` / `image` / `pic` /
 * `images[0]` / `unit` / `price` / `salePrice` —— **九個都不在 wire 上**。
 * 猜錯欄位不會報錯，只會安靜地退到 fallback：每個商品都被印成「（組）」，
 * 特價永遠不顯示（真正的欄位是 `salePriceMin`）。所以這裡不再有 `??` 鏈，
 * 只讀單一來源，並用契約測試釘住「舊 key 的 fixture 讀不到」。
 */

import {
	getProductPriceDisplay,
	type PriceFields,
	type ProductPriceDisplay,
} from "@/lib/products/pricing";

export interface ProductRow {
	id: string;
	name: string;
	imageUrl?: string;
	/** 售價顯示（含特價與刪除線）交給全站共用的 `getProductPriceDisplay`。 */
	price: ProductPriceDisplay;
}

export function mapProductRecord(
	p: Record<string, unknown>,
): ProductRow | null {
	const idRaw = p.id;
	if (idRaw === undefined || idRaw === null || idRaw === "") return null;
	const str = (v: unknown): string | undefined =>
		typeof v === "string" && v ? v : undefined;
	return {
		id: String(idRaw),
		name: str(p.name) ?? "未命名商品",
		// `imageUrl` 由 Gateway 用 `imageKey` 組好（49 §6.1 唯一入口），前端不自己
		// 組路徑；imageKey 為空時 Gateway 回的就是空字串。
		imageUrl: str(p.imageUrl),
		price: getProductPriceDisplay(p as PriceFields),
	};
}

export function mapProductRecords(
	records: Array<Record<string, unknown>>,
): ProductRow[] {
	const out: ProductRow[] = [];
	for (const rec of records) {
		const row = mapProductRecord(rec);
		if (row) out.push(row);
	}
	return out;
}
