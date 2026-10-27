"use client";

import * as React from "react";
import { usePrefersReducedMotion } from "@/lib/a11y/reduced-motion";

export interface CarouselAutoplayOptions {
	/** Number of slides. Fewer than two never starts a timer. */
	count: number;
	/** Tick interval in ms. `0` (or less) disables autoplay entirely. */
	intervalMs: number;
	/** Master switch — pass false to keep the carousel manual. */
	enabled?: boolean;
	/** Extra hold reason owned by the caller (e.g. an in-flight drag). */
	paused?: boolean;
	/** Advance callback. Latest version is always the one invoked. */
	onTick: () => void;
}

export interface CarouselAutoplay<T extends HTMLElement = HTMLElement> {
	/**
	 * Attach to the element that owns the carousel. Hover and focus anywhere
	 * inside it hold the timer, which is what makes WCAG 2.2.2's "pause"
	 * requirement reachable by both pointer and keyboard users.
	 */
	containerRef: React.RefCallback<T>;
	/**
	 * True only while the timer is actually ticking. Callers use it to flip
	 * the slide region's `aria-live` on: an auto-rotating region that also
	 * announces would talk over the user, whereas a *stopped* carousel is
	 * exactly when a screen-reader user wants each manual change announced.
	 */
	running: boolean;
}

/**
 * Shared autoplay clock for the banner and megamenu carousels.
 *
 * Both surfaces previously ran their own `setInterval` with only
 * pause-on-hover (the megamenu had not even that), which fails WCAG 2.2.2
 * for keyboard users and ignores `prefers-reduced-motion`. This hook is the
 * single implementation: it holds on hover, on focus-within, while the tab is
 * hidden, and is fully disabled under reduced motion.
 */
export function useCarouselAutoplay<T extends HTMLElement = HTMLElement>({
	count,
	intervalMs,
	enabled = true,
	paused = false,
	onTick,
}: CarouselAutoplayOptions): CarouselAutoplay<T> {
	const [node, setNode] = React.useState<T | null>(null);
	const [held, setHeld] = React.useState(false);
	const [documentHidden, setDocumentHidden] = React.useState(false);
	const reducedMotion = usePrefersReducedMotion();

	const onTickRef = React.useRef(onTick);
	React.useLayoutEffect(() => {
		onTickRef.current = onTick;
	}, [onTick]);

	// Pointer AND keyboard hold. `focusin` / `focusout` bubble (unlike
	// focus/blur), so one pair of listeners on the container covers every
	// control inside it.
	React.useEffect(() => {
		if (!node) return;
		const hold = () => setHeld(true);
		const release = () => setHeld(false);
		node.addEventListener("pointerenter", hold);
		node.addEventListener("pointerleave", release);
		node.addEventListener("focusin", hold);
		node.addEventListener("focusout", release);
		return () => {
			node.removeEventListener("pointerenter", hold);
			node.removeEventListener("pointerleave", release);
			node.removeEventListener("focusin", hold);
			node.removeEventListener("focusout", release);
		};
	}, [node]);

	// A background tab must not burn ticks (and must not have silently
	// advanced twenty slides by the time the user comes back).
	React.useEffect(() => {
		const sync = () => setDocumentHidden(document.hidden);
		sync();
		document.addEventListener("visibilitychange", sync);
		return () => document.removeEventListener("visibilitychange", sync);
	}, []);

	const running =
		enabled &&
		!paused &&
		!held &&
		!documentHidden &&
		!reducedMotion &&
		count > 1 &&
		intervalMs > 0;

	React.useEffect(() => {
		if (!running) return;
		const id = window.setInterval(() => onTickRef.current(), intervalMs);
		return () => window.clearInterval(id);
	}, [running, intervalMs]);

	return { containerRef: setNode, running };
}
