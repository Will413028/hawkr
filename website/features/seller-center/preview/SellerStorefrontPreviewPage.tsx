"use client";

import * as React from "react";
import { TruckMark } from "@/components/shared/TruckSpinner";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { storeIdOf } from "@/lib/seller/store";

/**
 * Resolve the authenticated seller's public account id before entering the
 * server-rendered preview route. The destination then reads the exact same
 * public seller/product/post/review sources as `/sellers/:id`.
 */
export default function SellerStorefrontPreviewPage() {
	const router = useObservedRouter("seller_storefront_preview");
	const { data: seller, loading, error } = useSellerSelf();
	const sellerId = storeIdOf(seller) ?? "";

	React.useEffect(() => {
		if (!sellerId) return;
		router.replace(`/seller-center/preview/${encodeURIComponent(sellerId)}`);
	}, [router, sellerId]);

	const message = loading || sellerId ? "正在開啟真實買家頁面…" : null;

	return (
		<div className="seller-profile">
			<div className="seller-card seller-profile__loading">
				{message ? (
					<div
						className="flex flex-col items-center justify-center gap-2"
						role="status"
						aria-live="polite"
					>
						<div
							className="relative h-[68px] w-[180px] overflow-hidden"
							aria-hidden="true"
						>
							<TruckMark
								size={60}
								className="absolute bottom-2 left-1/2 -translate-x-1/2"
							/>
							<div className="truck-loading__road-line absolute right-0 bottom-1 left-0 h-1 animate-truck-road opacity-60 motion-reduce:animate-none" />
						</div>
						<p className="m-0 text-sm font-medium">{message}</p>
					</div>
				) : (
					<p className="seller-profile__error" role="alert">
						{error?.message || "商店缺少公開識別碼，暫時無法預覽"}
					</p>
				)}
			</div>
		</div>
	);
}
