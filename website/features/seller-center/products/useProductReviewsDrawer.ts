"use client";

import * as React from "react";

/** The product whose buyer reviews are shown in the drawer. */
export interface ProductReviewsTarget {
	id: string;
	name: string;
	/** Main image for the drawer subheader; optional (list may omit it). */
	image?: string;
}

/**
 * 商品評論 drawer 的本地狀態 — 沿用 {@link useProductEditDrawer} 的設計：觸發點只
 * 在商品管理 / 庫存管理列表，兩路由互斥、無需跨頁共享，故用本地 state 而非全域
 * store（chat drawer 走全域是因為觸發點散落全站，這裡不需要）。
 *
 *   - `reviewsTarget` — 正在檢視評論的商品；`null` 表示關閉。
 */
export function useProductReviewsDrawer() {
	const [reviewsTarget, setReviewsTarget] =
		React.useState<ProductReviewsTarget | null>(null);

	const openReviews = React.useCallback((target: ProductReviewsTarget) => {
		setReviewsTarget(target);
	}, []);
	const closeReviews = React.useCallback(() => setReviewsTarget(null), []);

	return { reviewsTarget, openReviews, closeReviews };
}
