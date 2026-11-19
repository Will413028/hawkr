"use client";

import SellerMessageTab from "@/features/sellers/components/SellerMessageTab";
import { PREVIEW_MESSAGES_UNAVAILABLE } from "@/lib/seller/storefront-preview";
import { PreviewPublicOnly, usePreviewStore } from "./PreviewStoreContext";

function PublicMessages() {
	const { view } = usePreviewStore();
	return (
		<section
			className="max-w-[720px] mx-auto py-[var(--hkui-space-6,24px)] px-[var(--hkui-space-5,20px)] pb-[var(--hkui-space-12,48px)]"
			data-hk-page-content-ready="seller-storefront"
		>
			<SellerMessageTab
				sellerId={view.storeId}
				routeId={view.storeCode || view.storeId}
			/>
		</section>
	);
}

/** 迴響分頁：只在店家能公開時才掛載，讀買家看得到的公開討論區。 */
export default function PreviewMessagesTab() {
	return (
		<PreviewPublicOnly unavailable={PREVIEW_MESSAGES_UNAVAILABLE}>
			<PublicMessages />
		</PreviewPublicOnly>
	);
}
