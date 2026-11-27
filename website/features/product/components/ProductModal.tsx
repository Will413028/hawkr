"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { usePathname } from "@/lib/intl/navigation";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	hideProductQuickViewPending,
	isQuickViewOpener,
	readQuickViewOrigin,
	setQuickViewOpener,
} from "@/lib/product/quickViewOrigin";
import { retainOverlayOpenAttribute } from "@/lib/state/overlay-open-attribute";

const isProductPath = (p: string) => /^\/products\/[^/]+/.test(p);
const PRODUCT_MODAL_TRANSITION_MS = 280;
const FOCUSABLE_SELECTOR =
	"a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex='-1'])";

function focusableElements(root: HTMLElement): HTMLElement[] {
	return Array.from(
		root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
	).filter(
		(element) =>
			!element.closest("[inert], [hidden], [aria-hidden='true']") &&
			element.getClientRects().length > 0,
	);
}

/**
 * Quick-view shell for the intercepted product route. The list/page that opened
 * it stays mounted underneath (parallel `@modal` slot), so this is a pure
 * overlay: open = scale up out of the tapped card, close = scale back into it,
 * then `router.back()` reveals the page exactly where it was.
 *
 * Visibility is driven by the URL (`usePathname`), NOT just by being mounted:
 * Next doesn't reliably reset the `@modal` slot to `default` on back, so the
 * slot can keep this component mounted after the URL has returned to the list.
 * Gating on the pathname makes close 100% reliable — when the URL is no longer a
 * product the modal renders null and releases its scroll-lock. Closing is via
 * the product page's own 返回 (intercepted here so it plays the shrink), the
 * backdrop, or Escape.
 */
export default function ProductModal({
	children,
}: {
	children: React.ReactNode;
}) {
	const router = useObservedRouter("product_modal");
	const pathname = usePathname() ?? "";
	const shouldShow = isProductPath(pathname);
	const [open, setOpen] = React.useState(false);
	const [handoff, setHandoff] = React.useState(false);
	const modalRef = React.useRef<HTMLDivElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const closingRef = React.useRef(false);
	const handedOffPendingRef = React.useRef(false);

	const close = React.useCallback(() => {
		if (closingRef.current) return;
		closingRef.current = true;
		setOpen(false);
		window.setTimeout(() => router.back(), PRODUCT_MODAL_TRANSITION_MS);
	}, [router]);

	React.useLayoutEffect(() => {
		if (!shouldShow) {
			handedOffPendingRef.current = false;
			setHandoff(false);
			return;
		}
		const origin = readQuickViewOrigin();
		const panel = panelRef.current;
		if (panel && origin) {
			panel.style.transformOrigin = `${origin.cx}px ${origin.cy}px`;
		}
		if (hideProductQuickViewPending()) {
			handedOffPendingRef.current = true;
			setHandoff(true);
			setOpen(true);
		}
	}, [shouldShow]);

	React.useEffect(() => {
		if (!shouldShow) return;
		const origin = readQuickViewOrigin();
		const scrollY = origin?.scrollY ?? window.scrollY;
		setQuickViewOpener(origin?.openerUrl ?? null);
		const releaseScrollLock = acquireBodyScrollLock({
			preserveScrollPosition: true,
			scrollY,
		});
		return () => {
			const returnedToOpener = isQuickViewOpener(window.location.href);
			releaseScrollLock({ restoreScrollPosition: returnedToOpener });
			if (!returnedToOpener) return;
			let frames = 0;
			const repin = () => {
				window.scrollTo(0, scrollY);
				if (++frames < 18) requestAnimationFrame(repin);
			};
			requestAnimationFrame(repin);
		};
	}, [shouldShow]);

	React.useEffect(() => {
		if (!shouldShow) return;
		return retainOverlayOpenAttribute();
	}, [shouldShow]);

	React.useEffect(() => {
		if (!shouldShow) return;
		closingRef.current = false;
		const modal = modalRef.current;
		const previouslyFocused =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		const inertSiblings = modal
			? Array.from(document.body.children)
					.filter(
						(element): element is HTMLElement =>
							element instanceof HTMLElement && element !== modal,
					)
					.map((element) => ({ element, inert: element.inert }))
			: [];
		for (const { element } of inertSiblings) element.inert = true;
		let raf: number | null = null;
		if (handedOffPendingRef.current) {
			setOpen(true);
			raf = requestAnimationFrame(() => setHandoff(false));
		} else {
			setHandoff(false);
			setOpen(false);
			raf = requestAnimationFrame(() => setOpen(true));
		}
		const focusFrame = requestAnimationFrame(() => {
			modal?.focus({ preventScroll: true });
		});
		const onKey = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				event.preventDefault();
				close();
				return;
			}
			if (event.key !== "Tab" || !modal) return;
			const focusables = focusableElements(modal);
			if (!focusables.length) {
				event.preventDefault();
				modal.focus({ preventScroll: true });
				return;
			}
			const first = focusables[0];
			const last = focusables[focusables.length - 1];
			const active = document.activeElement;
			if (
				event.shiftKey &&
				(active === first || active === modal || !modal.contains(active))
			) {
				event.preventDefault();
				last.focus({ preventScroll: true });
			} else if (
				!event.shiftKey &&
				(active === last || !modal.contains(active))
			) {
				event.preventDefault();
				first.focus({ preventScroll: true });
			}
		};
		const onClickCapture = (event: MouseEvent) => {
			if (event.button !== 0 || !(event.target instanceof Element)) return;
			if (event.target.closest("[data-zoom-back]")) {
				event.preventDefault();
				event.stopPropagation();
				close();
			}
		};
		document.addEventListener("keydown", onKey);
		document.addEventListener("click", onClickCapture, true);
		return () => {
			if (raf !== null) cancelAnimationFrame(raf);
			cancelAnimationFrame(focusFrame);
			document.removeEventListener("keydown", onKey);
			document.removeEventListener("click", onClickCapture, true);
			for (const { element, inert } of inertSiblings) element.inert = inert;
			if (
				isQuickViewOpener(window.location.href) &&
				previouslyFocused?.isConnected
			) {
				previouslyFocused.focus({ preventScroll: true });
			}
		};
	}, [shouldShow, close]);

	if (!shouldShow || typeof document === "undefined") return null;

	return createPortal(
		<div
			ref={modalRef}
			className="product-modal outline-none"
			data-handoff={handoff ? "true" : undefined}
			data-open={open}
			role="dialog"
			aria-modal="true"
			aria-label="商品詳情"
			tabIndex={-1}
		>
			<button
				type="button"
				className="product-modal__backdrop"
				aria-label="關閉"
				onClick={close}
				tabIndex={-1}
			/>
			<div className="product-modal__panel" ref={panelRef}>
				<div className="product-modal__scroll">{children}</div>
			</div>
		</div>,
		document.body,
	);
}
