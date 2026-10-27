"use client";

import * as React from "react";

/**
 * Persisted collapse state for the chat page's conversation sidebar.
 *
 * The chat-shell layout (`MessagesClient`, `ThreadClient`) consumes
 * `collapsed` to apply `data-sidebar-collapsed` on the grid wrapper —
 * CSS handles the actual layout flip (full 320px column ↔ 72px icon
 * rail). Persisting in localStorage keeps the user's preference
 * across both the list view and individual thread views without
 * having to lift the state into a context.
 *
 * SSR-safe: starts as `false` on the server and only reads
 * localStorage after mount, so there's no hydration warning even
 * when the user previously collapsed the sidebar.
 */
const STORAGE_KEY = "hkui:chat:sidebar-collapsed";

export function useChatSidebarCollapsed() {
	const [collapsed, setCollapsed] = React.useState(false);

	React.useEffect(() => {
		try {
			if (window.localStorage.getItem(STORAGE_KEY) === "1") {
				setCollapsed(true);
			}
		} catch {
			/* ignore */
		}
	}, []);

	const toggle = React.useCallback(() => {
		setCollapsed((prev) => {
			const next = !prev;
			try {
				window.localStorage.setItem(STORAGE_KEY, next ? "1" : "0");
			} catch {
				/* ignore */
			}
			return next;
		});
	}, []);

	return [collapsed, toggle] as const;
}
