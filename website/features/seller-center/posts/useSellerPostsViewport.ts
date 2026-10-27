"use client";

import * as React from "react";

export const SELLER_POSTS_MOBILE_QUERY = "(max-width: 767px)";

export type SellerPostsViewport = "mobile" | "desktop" | null;

function getViewportSnapshot(): SellerPostsViewport {
	if (
		typeof window === "undefined" ||
		typeof window.matchMedia !== "function"
	) {
		return null;
	}

	return window.matchMedia(SELLER_POSTS_MOBILE_QUERY).matches
		? "mobile"
		: "desktop";
}

function getServerSnapshot(): null {
	return null;
}

function subscribe(onStoreChange: () => void): () => void {
	if (
		typeof window === "undefined" ||
		typeof window.matchMedia !== "function"
	) {
		return () => undefined;
	}

	const mediaQuery = window.matchMedia(SELLER_POSTS_MOBILE_QUERY);
	mediaQuery.addEventListener("change", onStoreChange);
	return () => mediaQuery.removeEventListener("change", onStoreChange);
}

/**
 * The seller post page keeps separate desktop and phone layouts in the DOM.
 * Resolve the active breakpoint after hydration so only the visible layout is
 * allowed to mount remote media; a server snapshot of null prevents an eager
 * image request from the wrong layout during hydration.
 */
export function useSellerPostsViewport(): SellerPostsViewport {
	return React.useSyncExternalStore(
		subscribe,
		getViewportSnapshot,
		getServerSnapshot,
	);
}
