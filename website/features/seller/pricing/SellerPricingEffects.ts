"use client";

import * as React from "react";

const REVEAL_SELECTOR = "[data-sp-reveal]";
const SPOTLIGHT_SELECTOR = "[data-sp-spotlight]";

/**
 * Page-scoped progressive enhancement for the seller pricing surface.
 *
 * The reveal and spotlight patterns are adapted from React Bits. The page
 * keeps content visible during SSR and without JavaScript, then enhances
 * only after the first client layout is ready.
 *
 * Sources: https://github.com/DavidHDev/react-bits
 * License: MIT + Commons Clause
 */
export function useSellerPricingEffects(
	rootRef: React.RefObject<HTMLDivElement | null>,
) {
	React.useLayoutEffect(() => {
		const root = rootRef.current;
		if (!root) return;

		const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
		const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
		const revealElements = Array.from(
			root.querySelectorAll<HTMLElement>(REVEAL_SELECTOR),
		);
		const spotlightElements = Array.from(
			root.querySelectorAll<HTMLElement>(SPOTLIGHT_SELECTOR),
		);

		let revealObserver: IntersectionObserver | null = null;
		let activeSurface: HTMLElement | null = null;
		let activeBounds: DOMRect | null = null;
		let frameId: number | null = null;
		let pointerX = 0;
		let pointerY = 0;

		const stopPointerFrame = () => {
			if (frameId === null) return;
			cancelAnimationFrame(frameId);
			frameId = null;
		};

		const hideSpotlight = () => {
			stopPointerFrame();
			if (activeSurface) {
				activeSurface.style.setProperty("--sp-spotlight-opacity", "0");
			}
			activeSurface = null;
			activeBounds = null;
		};

		const cancelDecorativeAnimations = () => {
			for (const animation of root.getAnimations({ subtree: true })) {
				animation.cancel();
			}
		};

		const showAllReveals = () => {
			for (const element of revealElements) {
				element.dataset.spRevealState = "visible";
			}
		};

		const configureReveals = () => {
			revealObserver?.disconnect();
			revealObserver = null;

			if (
				reducedMotion.matches ||
				typeof IntersectionObserver === "undefined"
			) {
				root.dataset.spEffects = "static";
				showAllReveals();
				return;
			}

			const initialViewportBottom = window.innerHeight * 0.96;
			for (const element of revealElements) {
				const bounds = element.getBoundingClientRect();
				element.dataset.spRevealState =
					bounds.top < initialViewportBottom && bounds.bottom > 0
						? "visible"
						: "pending";
			}
			root.dataset.spEffects = "enhanced";

			revealObserver = new IntersectionObserver(
				(entries) => {
					for (const entry of entries) {
						if (!entry.isIntersecting) continue;
						const element = entry.target as HTMLElement;
						element.dataset.spRevealState = "visible";
						revealObserver?.unobserve(element);
					}
				},
				{ rootMargin: "0px 0px -8% 0px", threshold: 0.14 },
			);

			for (const element of revealElements) {
				if (element.dataset.spRevealState === "pending") {
					revealObserver.observe(element);
				}
			}
		};

		const onPointerMove = (event: PointerEvent) => {
			if (
				reducedMotion.matches ||
				!finePointer.matches ||
				(event.pointerType !== "mouse" && event.pointerType !== "pen")
			)
				return;

			const target = event.target;
			if (!(target instanceof Element)) return;
			const surface = target.closest<HTMLElement>(SPOTLIGHT_SELECTOR);
			if (!surface || !root.contains(surface)) return;

			if (activeSurface !== surface) {
				hideSpotlight();
				activeSurface = surface;
				activeBounds = surface.getBoundingClientRect();
			}
			pointerX = event.clientX;
			pointerY = event.clientY;
			if (frameId !== null) return;

			frameId = requestAnimationFrame(() => {
				frameId = null;
				if (!activeSurface) return;
				const bounds = activeBounds ?? activeSurface.getBoundingClientRect();
				activeSurface.style.setProperty(
					"--sp-spotlight-x",
					`${pointerX - bounds.left}px`,
				);
				activeSurface.style.setProperty(
					"--sp-spotlight-y",
					`${pointerY - bounds.top}px`,
				);
				activeSurface.style.setProperty("--sp-spotlight-opacity", "1");
			});
		};

		const onPointerOut = (event: PointerEvent) => {
			if (!activeSurface) return;
			const nextTarget = event.relatedTarget;
			if (nextTarget instanceof Node && activeSurface.contains(nextTarget)) {
				return;
			}
			hideSpotlight();
		};

		const onMotionPreferenceChange = () => {
			hideSpotlight();
			configureReveals();
			if (reducedMotion.matches) cancelDecorativeAnimations();
		};
		const onPointerCapabilityChange = () => {
			if (!finePointer.matches) hideSpotlight();
		};

		const resizeObserver =
			typeof ResizeObserver === "undefined"
				? null
				: new ResizeObserver(() => {
						activeBounds = null;
					});
		for (const element of spotlightElements) resizeObserver?.observe(element);

		configureReveals();
		root.addEventListener("pointermove", onPointerMove, { passive: true });
		root.addEventListener("pointerout", onPointerOut, { passive: true });
		root.addEventListener("pointercancel", hideSpotlight, { passive: true });
		reducedMotion.addEventListener("change", onMotionPreferenceChange);
		finePointer.addEventListener("change", onPointerCapabilityChange);

		return () => {
			revealObserver?.disconnect();
			resizeObserver?.disconnect();
			hideSpotlight();
			root.removeEventListener("pointermove", onPointerMove);
			root.removeEventListener("pointerout", onPointerOut);
			root.removeEventListener("pointercancel", hideSpotlight);
			reducedMotion.removeEventListener("change", onMotionPreferenceChange);
			finePointer.removeEventListener("change", onPointerCapabilityChange);
		};
	}, [rootRef]);
}
