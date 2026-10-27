"use client";

const OVERLAY_OPEN_ATTR = "data-hk-overlay-open";
let activeOverlayCount = 0;

export function retainOverlayOpenAttribute(): () => void {
	if (typeof document === "undefined") return () => {};

	activeOverlayCount += 1;
	if (activeOverlayCount === 1) {
		document.body.setAttribute(OVERLAY_OPEN_ATTR, "true");
	}

	let released = false;
	return () => {
		if (released) return;
		released = true;
		activeOverlayCount = Math.max(0, activeOverlayCount - 1);
		if (activeOverlayCount === 0) {
			document.body.removeAttribute(OVERLAY_OPEN_ATTR);
		}
	};
}
