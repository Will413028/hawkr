"use client";

import { useId, useSyncExternalStore } from "react";
import { FOOTER_ICON_LINK } from "@/components/shared/footerChrome";
import {
	getServerSiteBackgroundState,
	getSiteBackgroundState,
	subscribeSiteBackground,
	toggleSiteBackgroundPaused,
} from "@/lib/animation/site-background-state";
import { cn } from "@/lib/utils/cn";

const UNAVAILABLE_HINT = "此裝置不支援動態背景";

/** In the footer, never floating over a product action or mobile navigation. */
export default function SiteBackgroundToggle({
	variant = "footer",
}: {
	variant?: "footer" | "page" | "icon";
}) {
	const { available, paused } = useSyncExternalStore(
		subscribeSiteBackground,
		getSiteBackgroundState,
		getServerSiteBackgroundState,
	);
	const hintId = `${useId()}-site-background-hint`;
	const label = !available ? "靜態背景" : paused ? "播放背景" : "暫停背景";
	return (
		<div
			className={cn(
				variant === "icon" ? "contents" : "site-background-control",
				variant === "page" && "site-background-control--page",
			)}
		>
			<button
				type="button"
				// Geometry is also declared in site-grainient.css, but a component
				// must be able to guarantee its own 44px target without depending on
				// a stylesheet it does not own.
				className={
					variant === "icon"
						? FOOTER_ICON_LINK
						: "site-background-control__button min-h-11 px-3"
				}
				disabled={!available}
				// A permanently disabled control needs a reason. Without one the
				// footer just carries a dead 「靜態背景」 button.
				aria-describedby={available ? undefined : hintId}
				title={
					available
						? variant === "icon"
							? label
							: undefined
						: UNAVAILABLE_HINT
				}
				// Toggle semantics: the visible label changes with state, but the
				// control's nature (a two-state switch) was never exposed to AT.
				aria-pressed={available ? paused : undefined}
				onClick={toggleSiteBackgroundPaused}
			>
				{variant === "icon" && (
					<svg
						width="20"
						height="20"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="1.8"
						strokeLinecap="round"
						strokeLinejoin="round"
						aria-hidden="true"
					>
						{paused || !available ? (
							<path d="m9 5 10 7-10 7V5Z" />
						) : (
							<path d="M9 5v14M15 5v14" />
						)}
					</svg>
				)}
				<span className={variant === "icon" ? "sr-only" : undefined}>
					{label}
				</span>
			</button>
			{available ? null : (
				<span id={hintId} className="sr-only">
					{UNAVAILABLE_HINT}
				</span>
			)}
		</div>
	);
}
