"use client";

import * as React from "react";
import AppDownloadModal from "@/components/shared/AppDownloadModal";
import { brandAssets } from "@/lib/media/public-assets";

interface AppDownloadBannerV2Props {
	/** Retained for call-site compatibility; the CTA now opens the download modal. */
	deepLink?: string;
	className?: string;
}

export default function AppDownloadBannerV2({
	className = "",
}: AppDownloadBannerV2Props) {
	const [dismissed, setDismissed] = React.useState(false);
	const [open, setOpen] = React.useState(false);
	if (dismissed) return null;

	return (
		<aside
			className={["hkui-app-banner", className].filter(Boolean).join(" ")}
			aria-label="下載 App"
		>
			<div className="hkui-app-banner__icon" aria-hidden="true">
				{/* biome-ignore lint/performance/noImgElement: brand mark SVG kept at intrinsic ratio. */}
				<img
					src={brandAssets.logoMark}
					alt=""
					width={20}
					height={20}
					decoding="async"
					draggable={false}
				/>
			</div>
			<div className="hkui-app-banner__copy">
				<strong>用 App 更順</strong>
				<span>即時通知、結帳更快</span>
			</div>
			<div className="hkui-app-banner__actions">
				<button
					type="button"
					className="hkui-app-banner__cta"
					onClick={() => setOpen(true)}
					aria-haspopup="dialog"
				>
					下載 App
				</button>
				<button
					type="button"
					className="hkui-app-banner__dismiss"
					onClick={() => setDismissed(true)}
					aria-label="關閉"
				>
					✕
				</button>
			</div>
			<AppDownloadModal open={open} onClose={() => setOpen(false)} />
		</aside>
	);
}
