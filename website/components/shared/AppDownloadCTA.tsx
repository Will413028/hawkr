"use client";

import dynamic from "next/dynamic";
import * as React from "react";
import { FOOTER_ICON_LINK } from "@/components/shared/footerChrome";
import { cn } from "@/lib/utils/cn";

// The modal (QR renderer) only exists after a click, but a
// static import put it in the navbar's AND the footer's module graph, i.e.
// every page's first load. `ssr: false` is correct here: the modal is a
// portal that renders nothing until `open`.
const AppDownloadModal = dynamic(
	() => import("@/components/shared/AppDownloadModal"),
	{ ssr: false },
);

interface AppDownloadCTAProps {
	/**
	 * "pill"   — branded pill button (footer hero use).
	 * "compact"— small inline button (navbar utility area).
	 * "icon"   — circular footer action with a screen-reader label.
	 */
	variant?: "pill" | "compact" | "icon";
	className?: string;
	/** Override the default "下載 App" label. */
	children?: React.ReactNode;
}

const PhoneIcon = ({ size = 18 }: { size?: number }) => (
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<rect x="5" y="2" width="14" height="20" rx="3" />
		<path d="M11 18h2" />
	</svg>
);

export default function AppDownloadCTA({
	variant = "compact",
	className,
	children = "下載 App",
}: AppDownloadCTAProps) {
	const [open, setOpen] = React.useState(false);
	// The modal is only mounted once it has been asked for: an unopened CTA
	// should not pull the chunk down at all.
	const [everOpened, setEverOpened] = React.useState(false);

	const classes =
		variant === "icon"
			? FOOTER_ICON_LINK
			: variant === "pill"
				? "hkui-app-cta hkui-app-cta--pill"
				: "hkui-app-cta hkui-app-cta--compact";

	return (
		<>
			<button
				type="button"
				// `min-h-11` is the touch floor. The compact variant's CSS height is
				// 36px and the navbar additionally forced it to 40px; with the floor
				// declared here that override is no longer doing anything useful.
				className={cn(classes, "min-h-11", className)}
				onClick={() => {
					setEverOpened(true);
					setOpen(true);
				}}
				aria-haspopup="dialog"
				aria-expanded={open}
				title={variant === "icon" ? "下載 App" : undefined}
			>
				<PhoneIcon
					size={variant === "icon" ? 20 : variant === "pill" ? 18 : 16}
				/>
				<span className={variant === "icon" ? "sr-only" : undefined}>
					{children}
				</span>
			</button>
			{everOpened ? (
				<AppDownloadModal open={open} onClose={() => setOpen(false)} />
			) : null}
		</>
	);
}
