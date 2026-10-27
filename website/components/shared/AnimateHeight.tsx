"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

/**
 * Smoothly animates its height to follow the content, so swapping children
 * (e.g. per-CTA form fields in a decision drawer) glides instead of snapping
 * and yanking everything below it.
 *
 * Why a measured pixel height and not the `grid-template-rows: 0fr → 1fr`
 * trick: that technique animates between COLLAPSED and OPEN. This component has
 * no open state — it animates between two *content* heights, and a `1fr` track
 * resolves to whatever the content is with nothing in between to interpolate.
 *
 * Two things the measured version has to get right, and now does:
 *   • the observer follows the node that is currently mounted (a callback ref),
 *     not the one that happened to be there on mount. React replaces the inner
 *     element when the children change shape, and the old observation went with
 *     it — the height then froze at the last size it happened to see.
 *   • the first measurement lands before paint. Committing `height: auto` and
 *     only switching to a pixel value on a later render meant the FIRST content
 *     change had to transition from `auto`, which no browser interpolates, so
 *     the first change of the session snapped while every later one glided.
 *
 * The inner measuring div carries 4px padding (cancelled by the wrapper's
 * negative margin) so input focus rings — 3px outset box-shadows — survive
 * the wrapper's `overflow-hidden`.
 */
export function AnimateHeight({
	children,
	className,
}: {
	children: React.ReactNode;
	className?: string;
}) {
	const [height, setHeight] = React.useState<number | null>(null);
	const observerRef = React.useRef<ResizeObserver | null>(null);

	const attachInner = React.useCallback((el: HTMLDivElement | null) => {
		observerRef.current?.disconnect();
		observerRef.current = null;
		if (!el || typeof ResizeObserver === "undefined") return;
		// Measure synchronously on attach — this runs during the commit, so the
		// very first painted frame already carries a pixel height.
		setHeight(el.offsetHeight);
		const observer = new ResizeObserver(() => setHeight(el.offsetHeight));
		observer.observe(el);
		observerRef.current = observer;
	}, []);

	React.useEffect(() => () => observerRef.current?.disconnect(), []);

	return (
		<div
			className={cn(
				"-m-1 overflow-hidden transition-[height] duration-200 ease-out motion-reduce:transition-none",
				className,
			)}
			style={{ height: height === null ? "auto" : height }}
		>
			<div ref={attachInner} className="p-1">
				{children}
			</div>
		</div>
	);
}

export default AnimateHeight;
