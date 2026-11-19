"use client";

import * as React from "react";
import StorefrontProductGrid from "@/features/sellers/storefront/StorefrontProductGrid";
import { useSellerProducts } from "@/lib/hooks/useSellerProducts";
import { useUrlPageParam } from "@/lib/hooks/useUrlPageParam";
import { mapProduct } from "@/lib/market/listFilters";
import { PRODUCT_STATUS } from "@/lib/products/seller-repo";
import {
	isOnSaleProduct,
	previewProductRecord,
} from "@/lib/seller/storefront-preview";
import { usePreviewStore } from "./PreviewStoreContext";

export const PREVIEW_PRODUCTS_PAGE_SIZE = 12;

/** 商品分頁：讀店主端商品列表，只列上架中的商品，轉成市集卡片用的形狀。 */
export default function PreviewProductsTab() {
	const { view } = usePreviewStore();
	const [page, setPage] = useUrlPageParam();
	const { data, loading, error } = useSellerProducts({
		page,
		pageSize: PREVIEW_PRODUCTS_PAGE_SIZE,
		status: PRODUCT_STATUS.ON,
	});
	const items = React.useMemo(
		() =>
			(data?.records ?? [])
				.filter(isOnSaleProduct)
				.map((item, index) =>
					mapProduct(previewProductRecord(item, view), index),
				),
		[data, view],
	);

	if (!data) {
		return error ? (
			<p className="seller-profile__error" role="alert">
				商品暫時載入不到，請稍後再試。
			</p>
		) : (
			<p role="status" aria-busy={loading}>
				正在載入商品…
			</p>
		);
	}

	return (
		<>
			<span data-hk-page-content-ready="seller-storefront" hidden />
			<StorefrontProductGrid
				items={items}
				currentPage={page}
				totalPages={Math.ceil(data.total / PREVIEW_PRODUCTS_PAGE_SIZE)}
				onPageChange={setPage}
				cardPreview
			/>
		</>
	);
}
