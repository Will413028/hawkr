"use client";

import * as React from "react";
import { TruckMark } from "@/components/shared/TruckSpinner";
import AppDownloadBannerV2 from "@/features/seller/components/AppDownloadBannerV2";
import SellerPageShell from "@/features/seller/components/SellerPageShell";
import SellerProfileV2 from "@/features/seller/components/SellerProfileV2";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import {
	storeCanBePublic,
	storefrontViewOfMyStore,
} from "@/lib/seller/storefront-preview";
import { PreviewStoreProvider } from "./PreviewStoreContext";
import SellerStorefrontPreviewBanner from "./SellerStorefrontPreviewBanner";
import StorefrontPreviewBoundary from "./StorefrontPreviewBoundary";

export const PREVIEW_ROOT = "/seller-center/preview";

function PreviewStatus({
	loading,
	message,
}: {
	loading: boolean;
	message: string;
}) {
	return (
		<div className="seller-profile">
			<div className="seller-card seller-profile__loading">
				{loading ? (
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
						{message}
					</p>
				)}
			</div>
		</div>
	);
}

/**
 * 賣家的店面預覽外框：頂部資料與統計讀 `GetMyStore`（登入的店主本人，不經公開讀取與共用快取），
 * 各分頁從 context 取店家。
 */
export default function SellerStorefrontPreviewFrame({
	children,
}: {
	children: React.ReactNode;
}) {
	const { data, loading } = useSellerSelf();
	const view = React.useMemo(() => storefrontViewOfMyStore(data), [data]);
	const canBePublic = storeCanBePublic(data);
	const store = React.useMemo(
		() => (view ? { view, canBePublic } : null),
		[view, canBePublic],
	);

	if (!view || !store) {
		return (
			<PreviewStatus
				loading={loading}
				message={
					loading ? "正在載入店面預覽…" : "店面預覽暫時載入不到，請稍後再試。"
				}
			/>
		);
	}

	return (
		<PreviewStoreProvider value={store}>
			<SellerPageShell>
				<h1 className="sr-only">買家視角預覽</h1>
				<SellerStorefrontPreviewBanner canBePublic={canBePublic} />

				<div className="seller-tab-frame">
					<SellerProfileV2
						seller={view}
						sellerId={view.storeCode || view.storeId}
						previewMode
						tabBasePath={PREVIEW_ROOT}
					/>
					{view.storeCode ? (
						<AppDownloadBannerV2
							deepLink={`hawkr://seller/${view.storeCode}`}
						/>
					) : null}
					<StorefrontPreviewBoundary>{children}</StorefrontPreviewBoundary>
				</div>
			</SellerPageShell>
		</PreviewStoreProvider>
	);
}
