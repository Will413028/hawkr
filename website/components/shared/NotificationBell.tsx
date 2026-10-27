"use client";

import type * as React from "react";
import NavBubbleLink from "@/components/shared/NavBubbleLink";
import { useNotificationUnread } from "@/lib/notifications/unread-context";

interface NotificationBellProps {
	href: string;
	className?: string;
	"aria-label"?: string;
	children?: React.ReactNode;
	onFocus?: React.FocusEventHandler<HTMLAnchorElement>;
	onMouseEnter?: React.MouseEventHandler<HTMLAnchorElement>;
}

/**
 * Wraps an icon link with a live unread-count badge sourced from the active
 * session's notification feed. Renders nothing extra when count is 0 — keeps
 * the navbar quiet for empty inboxes.
 *
 * The host chrome (classes, 44px floor, event surface, count live region)
 * lives in NavBubbleLink, shared with CartBubble and ChatBubble.
 */
const unreadLabel = (display: string) => `${display} 則未讀`;

export default function NotificationBell({
	href,
	className,
	"aria-label": ariaLabel = "通知",
	children,
	onFocus,
	onMouseEnter,
}: NotificationBellProps) {
	const { count } = useNotificationUnread();

	return (
		<NavBubbleLink
			href={href}
			className={className}
			label={ariaLabel}
			count={count}
			countLabel={unreadLabel}
			onFocus={onFocus}
			onMouseEnter={onMouseEnter}
		>
			{children}
		</NavBubbleLink>
	);
}
