"use client";

import { useEffect, useRef } from "react";

interface VantaEffect {
	destroy: () => void;
}

export default function SellerVantaAtmosphere() {
	const containerRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
		const precisePointer = window.matchMedia(
			"(hover: hover) and (pointer: fine)",
		);
		if (reduceMotion.matches || !precisePointer.matches) return;

		const canvas = document.createElement("canvas");
		const hasWebGl = Boolean(
			canvas.getContext("webgl") || canvas.getContext("experimental-webgl"),
		);
		if (!hasWebGl) return;

		let effect: VantaEffect | undefined;
		let cancelled = false;
		const destroyEffect = () => {
			effect?.destroy();
			effect = undefined;
		};
		const handlePreferenceChange = () => {
			if (reduceMotion.matches || !precisePointer.matches) destroyEffect();
		};
		reduceMotion.addEventListener("change", handlePreferenceChange);
		precisePointer.addEventListener("change", handlePreferenceChange);

		const observer = new IntersectionObserver(
			(entries) => {
				if (!entries.some((entry) => entry.isIntersecting)) return;
				observer.disconnect();

				void Promise.all([import("three"), import("vanta/dist/vanta.fog.min")])
					.then(([THREE, { default: FOG }]) => {
						if (cancelled) return;
						effect = FOG({
							el: container,
							THREE,
							mouseControls: true,
							touchControls: false,
							gyroControls: false,
							minHeight: 200,
							minWidth: 200,
							highlightColor: 0xf2a541,
							midtoneColor: 0xe4572e,
							lowlightColor: 0xb8441f,
							baseColor: 0x3f5a2f,
							blurFactor: 0.42,
							speed: 0.2,
							zoom: 0.72,
						}) as VantaEffect;
					})
					.catch(() => {
						// The original CSS gradient remains the complete fallback.
					});
			},
			{ rootMargin: "240px 0px" },
		);

		observer.observe(container);

		return () => {
			cancelled = true;
			observer.disconnect();
			reduceMotion.removeEventListener("change", handlePreferenceChange);
			precisePointer.removeEventListener("change", handlePreferenceChange);
			destroyEffect();
		};
	}, []);

	return (
		<div
			ref={containerRef}
			className="seller-vanta-atmosphere"
			aria-hidden="true"
		/>
	);
}
