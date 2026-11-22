"use client";

import Button from "@/components/ui/Button";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { PRODUCT_REVIEW_TITLE_CLASS } from "./productReviewsLayout";

/**
 * PDP 評價讀取失敗時的區塊：失敗不能顯示成「還沒有評價」，也不能整段消失。
 * 重試重新跑伺服器端的評價讀取（`router.refresh()`）。
 */
export default function ProductReviewsUnavailable() {
	const router = useObservedRouter("product_reviews_unavailable");
	return (
		<div className="mb-8" id="product-reviews">
			<h2 className={`${PRODUCT_REVIEW_TITLE_CLASS} mb-4`}>商品評價</h2>
			<p className="text-danger text-sm m-0 mb-3" role="alert">
				評價暫時無法載入，請稍後再試。
			</p>
			<Button type="button" variant="bordered" onClick={() => router.refresh()}>
				重新載入
			</Button>
		</div>
	);
}
