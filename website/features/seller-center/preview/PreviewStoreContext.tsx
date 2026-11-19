"use client";

import * as React from "react";
import type { PublicStoreView } from "@/lib/seller/public-store";

interface PreviewStoreValue {
	/** `GetMyStore` 整理成的店面頂部資料；`storeId` 是其餘讀取的參照鍵。 */
	view: PublicStoreView;
	/** 店家此刻能不能公開曝光（`PUBLIC_LISTING` 啟用）。 */
	canBePublic: boolean;
}

const PreviewStoreContext = React.createContext<PreviewStoreValue | null>(null);

export const PreviewStoreProvider = PreviewStoreContext.Provider;

export function usePreviewStore(): PreviewStoreValue {
	const value = React.useContext(PreviewStoreContext);
	if (!value) throw new Error("usePreviewStore outside the storefront preview");
	return value;
}

/**
 * 只讀公開來源的分頁（評價、迴響）的閘門：店家不能公開時只顯示說明，children 不會掛載，
 * 所以它的公開讀取一個都不會發出。
 */
export function PreviewPublicOnly({
	unavailable,
	children,
}: {
	unavailable: string;
	children: React.ReactNode;
}) {
	const { canBePublic } = usePreviewStore();
	if (canBePublic) return <>{children}</>;
	return (
		<div
			className="glass-card glass-card--lg"
			style={{ marginTop: 24, textAlign: "center", color: "var(--ink-body)" }}
			data-storefront-preview-unavailable="true"
		>
			<p>{unavailable}</p>
		</div>
	);
}
