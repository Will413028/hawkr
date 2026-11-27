"use client";

import dynamic from "next/dynamic";
import * as React from "react";
import { FOOTER_ICON_LINK } from "@/components/shared/footerChrome";
import { cn } from "@/lib/utils/cn";

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
