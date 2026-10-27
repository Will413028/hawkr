"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import {
	NOTIF_BELL_BADGE,
	NOTIF_BELL_INNER,
} from "@/components/shared/notifBell";
import { cn } from "@/lib/utils/cn";

/**
 * Shared base for the three navbar count bubbles (NotificationBell,
 * CartBubble, ChatBubble). They had three copies of the same six-line
 * hostClassName builder, three different event-prop shapes, no intrinsic size
 * (both hosts sized them at 40px) and no announcement when the count changed
 * while the page sat idle.
 *
 * Owned here:
 *  · `notif-bell` host classes + `notif-bell--has-unread` state hook
 *  · a 44×44 floor (`min-h-11 min-w-11`) that a host className can still
 *    override with its own explicit sizing
 *  · one event surface — onClick / onFocus / onMouseEnter / onPointerDown all
 *    pass through, and each also arms the intent-based route prefetch
 *  · a visually-hidden `aria-live="polite"` region that speaks only when the
 *    count CHANGES (the label already carries the count on first read)
 */

export interface NavBubbleLinkProps {
	href: string;
	className?: string;
	/** Base name; the count is folded into the accessible name when non-zero. */
	label: string;
	count: number;
	/** Renders the "（N 則未讀）"/"（N 件商品）" suffix and the live message. */
	countLabel: (display: string) => string;
	children?: React.ReactNode;
	onClick?: React.MouseEventHandler<HTMLAnchorElement>;
	onFocus?: React.FocusEventHandler<HTMLAnchorElement>;
	onMouseEnter?: React.MouseEventHandler<HTMLAnchorElement>;
	onPointerDown?: React.PointerEventHandler<HTMLAnchorElement>;
	/**
	 * Extra `data-*` attributes for the inner icon box / badge (the
	 * fly-to-cart landing pad and badge markers). Typed as a plain attribute
	 * bag: the keys come from runtime constants in lib/cart/flyToCart.ts, so a
	 * `data-${string}` template type would not match them.
	 */
	innerProps?: Record<string, string>;
	badgeProps?: Record<string, string>;
	/**
	 * Defer route prefetch until the user shows intent. Off by default: only
	 * the messages route is expensive enough to warrant the gate.
	 */
	prefetchOnIntent?: boolean;
}

export default function NavBubbleLink({
	href,
	className,
	label,
	count,
	countLabel,
	children,
	onClick,
	onFocus,
	onMouseEnter,
	onPointerDown,
	innerProps,
	badgeProps,
	prefetchOnIntent = false,
}: NavBubbleLinkProps) {
	const [routePrefetchEnabled, setRoutePrefetchEnabled] = React.useState(false);
	const display = count > 99 ? "99+" : String(count);
	const hasCount = count > 0;
	const accessibleName = hasCount
		? `${label}（${countLabel(display)}）`
		: label;

	// Announce only transitions. On first render the accessible name already
	// carries the count, so seeding the region would double-speak it.
	const [liveMessage, setLiveMessage] = React.useState("");
	const previousCount = React.useRef(count);
	React.useEffect(() => {
		if (previousCount.current === count) return;
		previousCount.current = count;
		setLiveMessage(count > 0 ? `${label}${countLabel(display)}` : "");
	}, [count, display, label, countLabel]);

	const armPrefetch = React.useCallback(() => {
		if (!prefetchOnIntent) return;
		setRoutePrefetchEnabled(true);
	}, [prefetchOnIntent]);

	return (
		<>
			<Link
				href={href}
				prefetch={
					prefetchOnIntent ? (routePrefetchEnabled ? null : false) : undefined
				}
				className={cn(
					"notif-bell relative min-h-11 min-w-11",
					hasCount && "notif-bell--has-unread",
					className,
				)}
				aria-label={accessibleName}
				data-has-unread={hasCount ? "true" : undefined}
				onClick={onClick}
				onFocus={(event) => {
					armPrefetch();
					onFocus?.(event);
				}}
				onMouseEnter={(event) => {
					armPrefetch();
					onMouseEnter?.(event);
				}}
				onPointerDown={(event) => {
					armPrefetch();
					onPointerDown?.(event);
				}}
			>
				<span {...innerProps} className={NOTIF_BELL_INNER}>
					{children}
				</span>
				{hasCount && (
					<span {...badgeProps} className={NOTIF_BELL_BADGE} aria-hidden="true">
						{display}
					</span>
				)}
			</Link>
			{/* 即時宣告放在連結外面當兄弟：導覽列的宿主規則 `body .site-nav-action span`
		    （navigation-final.css）會在桌面寬度把連結內所有 span 解開成可見，
		    sr-only 會被它蓋掉，未讀數文字就佔了 rail 的寬度（購物車 109→185px）。
		    絕對定位的兄弟不參與 flex 排版，不影響 gap。 */}
			<span className="sr-only" aria-live="polite" data-nav-bubble-live="">
				{liveMessage}
			</span>
		</>
	);
}
