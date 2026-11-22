"use client";

import { useTranslations } from "next-intl";
import Button from "@/components/ui/Button";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { PRODUCT_REVIEW_TITLE_CLASS } from "./productReviewsLayout";

/**
 * PDP 評價讀取失敗時的區塊：失敗不能顯示成「還沒有評價」，也不能整段消失。
 * 重試重新跑伺服器端的評價讀取（`router.refresh()`）。
 */
export default function ProductReviewsUnavailable() {
	const t = useTranslations("product.reviewsUnavailable");
	const router = useObservedRouter("product_reviews_unavailable");
	return (
		<div className="mb-8" id="product-reviews">
			<h2 className={`${PRODUCT_REVIEW_TITLE_CLASS} mb-4`}>{t("title")}</h2>
			<p className="text-danger text-sm m-0 mb-3" role="alert">
				{t("body")}
			</p>
			<Button type="button" variant="bordered" onClick={() => router.refresh()}>
				{t("reload")}
			</Button>
		</div>
	);
}
