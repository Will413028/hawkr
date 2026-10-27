"use client";

import { type RefObject, useEffect } from "react";

const SPOTLIGHT_SELECTOR = "[data-product-cert-spotlight]";

/**
 * Progressive pointer spotlight for buyer-facing certification cards.
 *
 * Adapted from React Bits' SpotlightCard interaction, using the same guarded
 * event-delegation pattern already proven on Hawkr seller pricing. The
 * certification content remains complete without JavaScript; this hook only
 * enhances fine-pointer desktop use.
 *
 * Source and license: third_party/react-bits/NOTICE.md
 */
export function useProductCertificationSpotlight(
	rootRef: RefObject<HTMLUListElement | null>,
	cardCount: number,
) {
	useEffect(() => {
		if (cardCount === 0) return;
		const root = rootRef.current;
		if (!root) return;

		const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
		const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
		const forcedColors = window.matchMedia("(forced-colors: active)");

		let activeCard: HTMLElement | null = null;
		let frameId: number | null = null;
		let pointerX = 0;
		let pointerY = 0;

		const stopFrame = () => {
			if (frameId === null) return;
			cancelAnimationFrame(frameId);
			frameId = null;
		};

		const hideCard = (card: HTMLElement | null) => {
			if (!card) return;
			card.style.setProperty("--product-cert-spotlight-opacity", "0");
		};

		const clearActiveCard = () => {
			stopFrame();
			hideCard(activeCard);
			activeCard = null;
		};

		const configureMode = () => {
			const enabled =
				!reducedMotion.matches && finePointer.matches && !forcedColors.matches;
			root.dataset.productCertSpotlightMode = enabled ? "enhanced" : "static";
			if (!enabled) clearActiveCard();
		};

		const cardFromTarget = (target: EventTarget | null) => {
			if (!(target instanceof Element)) return null;
			const card = target.closest<HTMLElement>(SPOTLIGHT_SELECTOR);
			return card && root.contains(card) ? card : null;
		};

		const onPointerMove = (event: PointerEvent) => {
			if (
				reducedMotion.matches ||
				!finePointer.matches ||
				forcedColors.matches ||
				event.pointerType !== "mouse"
			)
				return;

			const card = cardFromTarget(event.target);
			if (!card) return;

			if (activeCard !== card) {
				hideCard(activeCard);
				activeCard = card;
			}

			pointerX = event.clientX;
			pointerY = event.clientY;
			if (frameId !== null) return;

			frameId = requestAnimationFrame(() => {
				frameId = null;
				if (!activeCard) return;
				// Read current viewport-relative bounds in the pointer frame so the
				// glow stays aligned after scrolling or an upstream layout shift.
				const bounds = activeCard.getBoundingClientRect();
				activeCard.style.setProperty(
					"--product-cert-spotlight-x",
					`${pointerX - bounds.left}px`,
				);
				activeCard.style.setProperty(
					"--product-cert-spotlight-y",
					`${pointerY - bounds.top}px`,
				);
				activeCard.style.setProperty("--product-cert-spotlight-opacity", "1");
			});
		};

		const onPointerOut = (event: PointerEvent) => {
			if (!activeCard) return;
			const nextTarget = event.relatedTarget;
			if (nextTarget instanceof Node && activeCard.contains(nextTarget)) return;
			clearActiveCard();
		};

		const onFocusIn = (event: FocusEvent) => {
			if (reducedMotion.matches || !finePointer.matches || forcedColors.matches)
				return;
			const card = cardFromTarget(event.target);
			if (!card || !(event.target instanceof HTMLElement)) return;

			const cardBounds = card.getBoundingClientRect();
			const focusBounds = event.target.getBoundingClientRect();
			card.style.setProperty(
				"--product-cert-spotlight-x",
				`${focusBounds.left + focusBounds.width / 2 - cardBounds.left}px`,
			);
			card.style.setProperty(
				"--product-cert-spotlight-y",
				`${focusBounds.top + focusBounds.height / 2 - cardBounds.top}px`,
			);
		};

		const onVisibilityChange = () => {
			if (document.hidden) clearActiveCard();
		};

		configureMode();
		root.addEventListener("pointermove", onPointerMove, { passive: true });
		root.addEventListener("pointerout", onPointerOut, { passive: true });
		root.addEventListener("pointercancel", clearActiveCard, { passive: true });
		root.addEventListener("focusin", onFocusIn);
		window.addEventListener("blur", clearActiveCard);
		document.addEventListener("visibilitychange", onVisibilityChange);
		reducedMotion.addEventListener("change", configureMode);
		finePointer.addEventListener("change", configureMode);
		forcedColors.addEventListener("change", configureMode);

		return () => {
			clearActiveCard();
			delete root.dataset.productCertSpotlightMode;
			root.removeEventListener("pointermove", onPointerMove);
			root.removeEventListener("pointerout", onPointerOut);
			root.removeEventListener("pointercancel", clearActiveCard);
			root.removeEventListener("focusin", onFocusIn);
			window.removeEventListener("blur", clearActiveCard);
			document.removeEventListener("visibilitychange", onVisibilityChange);
			reducedMotion.removeEventListener("change", configureMode);
			finePointer.removeEventListener("change", configureMode);
			forcedColors.removeEventListener("change", configureMode);
		};
	}, [rootRef, cardCount]);
}
