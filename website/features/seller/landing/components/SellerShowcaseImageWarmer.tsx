"use client";

import { useEffect, useRef } from "react";

const IMAGE_SOURCES = [
	"/placeholders/web-screen.svg",
	"/placeholders/app-screen.svg",
] as const;

type SellerShowcaseWindow = Window & {
	__hkSellerShowcaseImageWarmers?: HTMLImageElement[];
};

/**
 * Warm the static showcase images from a client boundary at the top of the
 * seller page. The carousel itself sits several viewports below the hero, so
 * React/Chromium may defer hydrating and fetching its clipped slides until the
 * user scrolls near it even when their <img> tags use loading="eager".
 *
 * This intentionally uses Image objects instead of <link rel="preload">. The
 * requests still start during initial page hydration, while visitors who never
 * reach the carousel do not receive "preloaded but not used" console warnings.
 */
export default function SellerShowcaseImageWarmer() {
	const imageWarmers = useRef<HTMLImageElement[]>([]);

	useEffect(() => {
		const target = window as SellerShowcaseWindow;
		const existing = target.__hkSellerShowcaseImageWarmers;
		if (existing?.length === IMAGE_SOURCES.length) {
			imageWarmers.current = existing;
		} else {
			imageWarmers.current = IMAGE_SOURCES.map((src) => {
				const image = new window.Image();
				image.decoding = "async";
				image.fetchPriority = "low";
				image.src = src;
				return image;
			});
			target.__hkSellerShowcaseImageWarmers = imageWarmers.current;
		}

		return () => {
			imageWarmers.current = [];
			delete target.__hkSellerShowcaseImageWarmers;
		};
	}, []);

	// This boundary sits above the carousel, so its effect runs with initial
	// hydration instead of waiting for the below-fold slides. Rendering a script
	// tag from a client component makes React warn during hydration; Image
	// objects provide the same cache warming without altering the DOM.
	return null;
}
