import type { CSSProperties } from "react";

// Shared FLIP-style zoom math: a full-viewport surface shrinks down onto (or
// grows out of) a captured card box, anchored at the card's centre. Uniform
// (width-based) scale keeps the content from distorting; opacity fades it out
// as it shrinks.
//
// Used by components/shared/PageTransition.tsx (card ↔ product-detail zoom,
// bounds 0.05–0.85) and features/feed/components/PostDetailViewer.tsx (feed
// thumbnail ↔ viewer, bounds 0.08–0.72).

export interface ZoomRect {
	top: number;
	left: number;
	width: number;
	height: number;
}

export interface ZoomScaleBounds {
	min: number;
	max: number;
}

// `transform-origin` value at the centre of the captured card box.
export function zoomOriginCenter(rect: ZoomRect): string {
	return `${rect.left + rect.width / 2}px ${rect.top + rect.height / 2}px`;
}

// Transform that shrinks the full-viewport surface onto the card box. The
// width-based scale is clamped to `bounds` so extreme card/viewport ratios
// can't collapse the surface to nothing or barely move it.
export function zoomShrinkStyle(
	rect: ZoomRect,
	bounds: ZoomScaleBounds,
): CSSProperties {
	const vw = typeof window !== "undefined" ? window.innerWidth || 1 : 1;
	const scale = Math.max(bounds.min, Math.min(bounds.max, rect.width / vw));
	return {
		transformOrigin: zoomOriginCenter(rect),
		transform: `scale(${scale})`,
		opacity: 0,
	};
}
