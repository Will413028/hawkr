"use client";

/**
 * Import 場景下的 SKU 編輯卡片。
 *
 * 因為 `importItemAdapter` 已經把 `ImportItemSku` 攤平成正式 `ProductSku`
 * 形狀（含 `shipping_methods: SkuShippingMethod[]` 卡式陣列），運費試算 +
 * 尺寸 / 重量驗證 / SKU 卡片佈局 / 原價 + 售價 + 區間 picker 與正式 SKU
 * 編輯器完全相同 — 直接 delegate 到 `ProductFormSkuEditor`，避免維護兩份近
 * 乎一樣的長 component。
 *
 * 設計上保留獨立 wrapper 是為了將來 import 端要新增 import-only 元件（例如
 * 「警告：此 SKU 觸發 SHIPPING_ALL_DOWNGRADED」浮層）時有地方擴充，不需要回
 * 頭改正式 form 元件。
 */

import ProductFormSkuEditor from "@/features/seller-center/products/components/ProductFormSkuEditor";
import type {
	ProductSku,
	SellerProductDetail,
} from "@/lib/products/seller-repo";

interface Props {
	skus: SellerProductDetail["skus"];
	onUpdate: (index: number, patch: Partial<ProductSku>) => void;
	onAdd: () => void;
	onRemove: (index: number) => void;
}

export default function ImportItemSkuEditor({
	skus,
	onUpdate,
	onAdd,
	onRemove,
}: Props) {
	return (
		<ProductFormSkuEditor
			skus={skus}
			onUpdate={onUpdate}
			onAdd={onAdd}
			onRemove={onRemove}
		/>
	);
}
