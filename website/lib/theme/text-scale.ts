"use client";

import * as React from "react";

/**
 * Site-wide text size (字體大小：小 / 中 / 大).
 *
 * Mirrors `lib/theme/context.tsx` deliberately — same storage-key namespace,
 * same "boot script paints `<html>` before hydration" contract, same
 * hydration-safety gate — so the two personalisation settings behave
 * identically and can sit side by side in the navbar / footer / settings.
 *
 * One source of truth: the `data-text-scale` attribute on `<html>`.
 * - `md` is the default and is represented by the attribute being ABSENT, so
 *   the overwhelmingly common case adds no attribute and no CSS work.
 * - `styles/foundation/02-reset-layout.css` turns the attribute into a root
 *   `font-size` percentage (87.5% / 112.5%), which is why every `rem` in the
 *   product scales together.
 * - `styles/globals.css` divides Tailwind's `--spacing` by the same factor, so
 *   padding / gap / size utilities stay pinned to their design px values while
 *   only type scales. See the comment on that declaration.
 *
 * Storage is localStorage only (no cookie, no account field) — the setting is
 * per-device by design, like the site-background pause switch.
 */

export type TextScale = "sm" | "md" | "lg";

export const TEXT_SCALES: readonly TextScale[] = ["sm", "md", "lg"];

export const TEXT_SCALE_STORAGE_KEY = "hk:text-scale";

/** Attribute on `<html>`; absent means `md`. */
export const TEXT_SCALE_ATTRIBUTE = "data-text-scale";

/**
 * Same-document change notification. `storage` only fires in OTHER tabs, so a
 * custom event is what keeps the toggle in the navbar and the segmented
 * control in 設定 in sync within one page.
 */
export const TEXT_SCALE_EVENT = "hk:text-scale-change";

export const TEXT_SCALE_LABELS: Record<TextScale, string> = {
	sm: "小",
	md: "中",
	lg: "大",
};

function isTextScale(value: unknown): value is TextScale {
	return value === "sm" || value === "md" || value === "lg";
}

/**
 * Inline script string for the bootstrap `<script>` in `components/shared/RootDocument.tsx`.
 *
 * Reads localStorage synchronously and writes `data-text-scale` to `<html>`
 * before React hydrates, so a 小/大 reader never sees a frame of 中 type and
 * the page never reflows on hydration.
 */
export const TEXT_SCALE_BOOT_SCRIPT = `try {
  var v = localStorage.getItem(${JSON.stringify(TEXT_SCALE_STORAGE_KEY)});
  if (v === 'sm' || v === 'lg') {
    document.documentElement.setAttribute(${JSON.stringify(TEXT_SCALE_ATTRIBUTE)}, v);
  } else {
    document.documentElement.removeAttribute(${JSON.stringify(TEXT_SCALE_ATTRIBUTE)});
  }
} catch (e) {}`;

/** Current scale from the DOM. SSR-safe: returns `md` with no window. */
export function getTextScale(): TextScale {
	if (typeof document === "undefined") return "md";
	const value = document.documentElement.getAttribute(TEXT_SCALE_ATTRIBUTE);
	return isTextScale(value) ? value : "md";
}

/** The value the server rendered, always. Kept as its own function so
 * `useSyncExternalStore` gets a stable server snapshot reference. */
function getServerTextScale(): TextScale {
	return "md";
}

/** Write the attribute + localStorage and wake every subscriber. */
export function setTextScale(scale: TextScale): void {
	if (typeof document === "undefined") return;
	const root = document.documentElement;
	if (scale === "md") {
		root.removeAttribute(TEXT_SCALE_ATTRIBUTE);
	} else {
		root.setAttribute(TEXT_SCALE_ATTRIBUTE, scale);
	}
	try {
		localStorage.setItem(TEXT_SCALE_STORAGE_KEY, scale);
	} catch {
		// Private mode / storage disabled: the attribute still applies for this
		// page view, it just will not survive a reload.
	}
	window.dispatchEvent(new CustomEvent(TEXT_SCALE_EVENT, { detail: scale }));
}

/** 中 → 大 → 小 → 中. */
export function nextTextScale(scale: TextScale): TextScale {
	if (scale === "md") return "lg";
	if (scale === "lg") return "sm";
	return "md";
}

function subscribe(onStoreChange: () => void): () => void {
	window.addEventListener(TEXT_SCALE_EVENT, onStoreChange);
	// Cross-tab: another tab wrote the key, so re-read (and re-apply) here.
	window.addEventListener("storage", onStoreChange);
	return () => {
		window.removeEventListener(TEXT_SCALE_EVENT, onStoreChange);
		window.removeEventListener("storage", onStoreChange);
	};
}

export interface TextScaleControls {
	scale: TextScale;
	setScale: (scale: TextScale) => void;
	cycle: () => void;
}

/**
 * The boot script mutates `<html>` before React runs, so the very first client
 * render of a 小/大 reader would otherwise disagree with the server HTML (which
 * is always 中) and trip a hydration mismatch — the same problem, and the same
 * fix, as `ThemeToggle`'s mounted gate. `useSyncExternalStore` already swaps in
 * `getServerSnapshot` while hydrating, and the mounted flag additionally covers
 * subtrees that hydrate LATE (the navbar renders inside `<Suspense>`), where the
 * store snapshot is read on a normal client render rather than a hydration one.
 */
export function useTextScale(): TextScaleControls {
	const stored = React.useSyncExternalStore(
		subscribe,
		getTextScale,
		getServerTextScale,
	);
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => setMounted(true), []);
	const scale = mounted ? stored : "md";

	const setScale = React.useCallback((next: TextScale) => {
		setTextScale(next);
	}, []);

	const cycle = React.useCallback(() => {
		setTextScale(nextTextScale(getTextScale()));
	}, []);

	return { scale, setScale, cycle };
}
