"use client";

import type * as React from "react";
import NavBubbleLink from "@/components/shared/NavBubbleLink";
import { useChatUnread } from "@/lib/chat/unread-context";

/**
 * Navbar chat bubble — same skin as NotificationBell but pulls the
 * unread *messages* count rather than notifications.
 *
 * `prefetchOnIntent`: the messages route is heavy, so it is only prefetched
 * once the user shows intent (focus / hover / pointer-down). The gate itself
 * lives in NavBubbleLink so all three bubbles share one event surface.
 */
const unreadLabel = (display: string) => `${display} 則未讀`;

export default function ChatBubble({
	href,
	className,
	"aria-label": ariaLabel = "訊息",
	children,
	onClick,
	onFocus,
	onMouseEnter,
	onPointerDown,
}: {
	href: string;
	className?: string;
	"aria-label"?: string;
	children?: React.ReactNode;
	onClick?: React.MouseEventHandler<HTMLAnchorElement>;
	onFocus?: React.FocusEventHandler<HTMLAnchorElement>;
	onMouseEnter?: React.MouseEventHandler<HTMLAnchorElement>;
	onPointerDown?: React.PointerEventHandler<HTMLAnchorElement>;
}) {
	const { count } = useChatUnread();

	return (
		<NavBubbleLink
			href={href}
			className={className}
			label={ariaLabel}
			count={count}
			countLabel={unreadLabel}
			prefetchOnIntent
			onClick={onClick}
			onFocus={onFocus}
			onMouseEnter={onMouseEnter}
			onPointerDown={onPointerDown}
		>
			{children}
		</NavBubbleLink>
	);
}
