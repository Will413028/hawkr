import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import "@/styles/features/feed-components.css";
import "@/styles/features/market.css";
import "@/styles/features/storefront-product-preview.css";
import AppDownloadBannerV2 from "@/features/seller/components/AppDownloadBannerV2";
import SellerPageShell from "@/features/seller/components/SellerPageShell";
import SellerProfileV2 from "@/features/seller/components/SellerProfileV2";
import SellerStorefrontPreviewBanner from "@/features/seller-center/preview/SellerStorefrontPreviewBanner";
import StorefrontPreviewBoundary from "@/features/seller-center/preview/StorefrontPreviewBoundary";
import { getCachedSeller } from "@/lib/api/cached";

export default async function SellerStorefrontPreviewLayout({
	children,
	params,
}: {
	children: ReactNode;
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	if (!/^\d+$/.test(id)) notFound();

	const seller = await getCachedSeller(id);
	if (!seller) notFound();

	const previewRoot = `/seller-center/preview/${id}`;

	return (
		<SellerPageShell>
			<h1 className="sr-only">買家視角預覽</h1>
			<SellerStorefrontPreviewBanner />

			<div className="seller-tab-frame">
				<SellerProfileV2
					seller={seller}
					sellerId={id}
					previewMode
					tabBasePath={previewRoot}
				/>
				<AppDownloadBannerV2 deepLink={`hawkr://seller/${id}`} />
				<StorefrontPreviewBoundary sellerId={id}>
					{children}
				</StorefrontPreviewBoundary>
			</div>
		</SellerPageShell>
	);
}
