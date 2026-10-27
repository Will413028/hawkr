"use client";

import { useSyncExternalStore } from "react";

const QUERY = "(prefers-reduced-motion: reduce)";

/** SSR / 舊瀏覽器安全的一次性讀取；只在事件 handler 或 effect 內呼叫。 */
export function prefersReducedMotion(): boolean {
	if (
		typeof window === "undefined" ||
		typeof window.matchMedia !== "function"
	) {
		return false;
	}
	return window.matchMedia(QUERY).matches;
}

function subscribe(onChange: () => void): () => void {
	if (
		typeof window === "undefined" ||
		typeof window.matchMedia !== "function"
	) {
		return () => {};
	}
	const media = window.matchMedia(QUERY);
	media.addEventListener("change", onChange);
	return () => media.removeEventListener("change", onChange);
}

/**
 * 跟著 OS 設定即時更新的 hook。Server snapshot 固定 false，
 * 所以首次 client render 與 SSR 一致、不會 hydration mismatch；
 * 使用者在 session 中途切換設定也會反映。
 *
 * 純 CSS 的動畫請直接用 Tailwind 的 `motion-reduce:` 變體，
 * 這個 hook 留給 JS 驅動的動畫（GSAP、framer-motion、
 * scrollIntoView、View Transitions、setTimeout 排程）。
 */
export function usePrefersReducedMotion(): boolean {
	return useSyncExternalStore(subscribe, prefersReducedMotion, () => false);
}
