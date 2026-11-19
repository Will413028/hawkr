import type { ReactNode } from "react";
import "@/styles/features/feed-components.css";
import "@/styles/features/market.css";
import "@/styles/features/storefront-product-preview.css";
import SellerStorefrontPreviewFrame from "@/features/seller-center/preview/SellerStorefrontPreviewFrame";

export default function SellerStorefrontPreviewLayout({
	children,
}: {
	children: ReactNode;
}) {
	return (
		<SellerStorefrontPreviewFrame>{children}</SellerStorefrontPreviewFrame>
	);
}
