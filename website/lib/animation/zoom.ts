import type { CSSProperties } from "react";


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

export function zoomOriginCenter(rect: ZoomRect): string {
	return `${rect.left + rect.width / 2}px ${rect.top + rect.height / 2}px`;
}

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
