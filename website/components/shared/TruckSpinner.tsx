"use client";

import { useEffect, useRef } from "react";
import AppIcon from "@/components/shared/AppIcon";
import { setInertOutside } from "@/lib/a11y/inert";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { cn } from "@/lib/utils/cn";

/**
 * Two loading affordances ported from the seller APP:
 *
 *   - `TruckSpinner`   — inline 24-32px truck glyph that gently
 *                        bobs in place (pure-CSS bob over a static SVG
 *                        so table rows can render many at once cheaply).
 *   - `TruckOverlay`   — page/dialog scrim with the truck icon bobbing over
 *                        a dashed road. Used for full-page loading or
 *                        bulk-operation modals.
 *
 * Runtime color/mask helpers live in `styles/globals.css`.
 */

interface TruckSpinnerProps {
	size?: number;
	label?: string;
	className?: string;
}

interface TruckMarkProps {
	size?: number;
	className?: string;
}

/**
 * The delivery-truck icon without overlay chrome. This is useful when the
 * caller already owns the loading surface and only needs the bobbing mark.
 */
export function TruckMark({ size = 60, className }: TruckMarkProps) {
	return (
		<AppIcon
			name="truck"
			size={size}
			className={cn("animate-truck-bob motion-reduce:animate-none", className)}
			style={{
				color:
					"var(--truck-loading-vehicle-color, var(--color-brand-primary-strong))",
			}}
		/>
	);
}

export function TruckSpinner({
	size = 28,
	label = "載入中",
	className,
}: TruckSpinnerProps) {
	return (
		<span
			className={[
				"truck-loading__inline inline-flex items-center justify-center",
				className,
			]
				.filter(Boolean)
				.join(" ")}
			role="status"
			aria-label={label}
			style={{ width: size, height: size }}
		>
			<AppIcon
				name="truck"
				size={size}
				className="animate-truck-bob motion-reduce:animate-none"
			/>
		</span>
	);
}

interface TruckOverlayProps {
	/** Shown under the truck. Optional. */
	message?: string;
	/** Render as a fixed full-screen overlay (default true). When false
	 * renders inline so callers can park it inside a modal body. */
	fullscreen?: boolean;
}

export function TruckOverlay({
	message = "處理中…",
	fullscreen = true,
}: TruckOverlayProps) {
	const hostRef = useRef<HTMLDivElement | null>(null);

	// Lock background scroll while the fullscreen scrim is mounted — `position:
	// fixed` stops clicks but not wheel scrolling, so the page would otherwise
	// scroll behind the overlay. Inline mode (fullscreen=false) lives inside a
	// Dialog body that already owns the lock, so leave the body untouched.
	//
	// The scrim also has to stop KEYBOARD reach: it covers the page but nothing
	// stopped Tab walking into the controls behind it, so a bulk operation could
	// be fired twice from a screen the user could not see. Same isolation the
	// Dialog uses.
	useEffect(() => {
		if (!fullscreen) return;
		const releaseScroll = acquireBodyScrollLock();
		const releaseInert = setInertOutside(hostRef.current);
		return () => {
			releaseInert();
			releaseScroll();
		};
	}, [fullscreen]);

	return (
		<div
			ref={hostRef}
			className={[
				"flex flex-col items-center justify-center gap-3 p-6 text-[var(--hkui-foreground)]",
				fullscreen
					? "fixed inset-0 z-[calc(var(--z-modal)+10)] bg-[var(--page-truck-scrim)] backdrop-blur-md"
					: "",
			]
				.filter(Boolean)
				.join(" ")}
			role="status"
			aria-live="polite"
		>
			<div className="relative w-[200px] h-20 bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[14px] overflow-hidden dark:bg-[var(--page-truck-panel-on-dark)] dark:border-[var(--page-truck-panel-border-on-dark)]">
				{/* The truck colour maps to the shared light/dark truck tokens. */}
				<span
					className="absolute bottom-2.5 left-1/2 -translate-x-1/2"
					aria-hidden="true"
				>
					<TruckMark />
				</span>
				<div
					className="truck-loading__road-line absolute right-0 bottom-1.5 left-0 h-1 animate-truck-road opacity-60 motion-reduce:animate-none"
					aria-hidden="true"
				/>
			</div>
			{message && (
				<p
					className={[
						"m-0 text-sm font-semibold rounded-full py-1.5 px-3.5 border",
						fullscreen
							? "text-[var(--page-truck-message-fg)] bg-[var(--page-truck-message-bg)] border-[var(--page-truck-message-border)]"
							: "text-[var(--hkui-foreground)] bg-[var(--hkui-content1)] border-[var(--hkui-divider)]",
					].join(" ")}
				>
					{message}
				</p>
			)}
		</div>
	);
}
