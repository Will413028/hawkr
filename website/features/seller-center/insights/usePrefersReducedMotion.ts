"use client";

import * as React from "react";

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

function subscribe(onStoreChange: () => void): () => void {
	const media = window.matchMedia(REDUCED_MOTION_QUERY);
	media.addEventListener("change", onStoreChange);
	return () => media.removeEventListener("change", onStoreChange);
}

function getSnapshot(): boolean {
	return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

export function usePrefersReducedMotion(): boolean {
	return React.useSyncExternalStore(subscribe, getSnapshot, () => true);
}
