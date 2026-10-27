"use client";

import { useEffect, useRef, useState } from "react";

export type GalleryImagePreset = "card" | "detail" | "banner";

/** Current gateway widths; card also matches this gallery's 4:3 crop. */
export function galleryImagePreset(
	width: number,
	pixelRatio: number,
): GalleryImagePreset {
	const pixels =
		width * (Number.isFinite(pixelRatio) && pixelRatio > 0 ? pixelRatio : 1);
	return pixels <= 720 ? "card" : pixels <= 1280 ? "detail" : "banner";
}

export function useGalleryImagePreset() {
	const ref = useRef<HTMLElement>(null);
	// Render the reusable thumbnail first; never start a speculative large
	// image request before we know this particular gallery's rendered size.
	const [preset, setPreset] = useState<GalleryImagePreset>("card");
	useEffect(() => {
		const element = ref.current;
		if (!element) return;
		let width = element.getBoundingClientRect().width;
		let density: MediaQueryList | undefined;
		const update = () => {
			if (width > 0)
				setPreset(galleryImagePreset(width, window.devicePixelRatio));
		};
		const watchDensity = () => {
			density?.removeEventListener("change", watchDensity);
			update();
			density = window.matchMedia?.(
				`(resolution: ${window.devicePixelRatio}dppx)`,
			);
			density?.addEventListener("change", watchDensity);
		};
		const measure = () => {
			width = element.getBoundingClientRect().width;
			update();
		};
		const observer =
			typeof ResizeObserver === "undefined"
				? undefined
				: new ResizeObserver(([entry]) => {
						width = entry.contentRect.width;
						update();
					});
		observer?.observe(element);
		window.addEventListener("resize", measure);
		watchDensity();
		return () => {
			observer?.disconnect();
			density?.removeEventListener("change", watchDensity);
			window.removeEventListener("resize", measure);
		};
	}, []);
	return { ref, preset };
}
