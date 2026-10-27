"use client";

import * as React from "react";

/**
 * Pin a full-height layout above the on-screen keyboard.
 *
 * The chat shell sizes itself to `calc(100dvh - … - var(--keyboard-inset))`.
 * On Android Chrome the `interactive-widget=resizes-content` viewport meta
 * already shrinks `dvh` when the soft keyboard opens, but iOS Safari ignores
 * that meta entirely: `dvh`/`vh`/`innerHeight` stay at the full layout
 * viewport, the keyboard just overlays the bottom, and Safari scrolls the
 * document up to reveal the focused field — exposing the footer in the gap
 * between the compose bar and the keyboard.
 *
 * This hook closes that gap by measuring the keyboard from `visualViewport`
 * and publishing it as a CSS variable the layout can subtract:
 *
 *   inset = innerHeight − visualViewport.height − visualViewport.offsetTop
 *
 * Because the iOS dynamic URL bar moves `innerHeight` and `visualViewport`
 * together, toolbar collapse/expand cancels out and only the keyboard shows
 * up as a non-zero inset. On Android the layout viewport shrinks with the
 * keyboard too, so the inset stays ~0 and there's no double subtraction.
 *
 * Side effects (on `document.documentElement`, reset on unmount):
 *  · `--keyboard-inset` — keyboard height in px (`0px` when closed)
 *  · `data-keyboard-open` — present while the keyboard is open, so floors
 *    like the shell's `min-height` can be released to let it shrink.
 *
 * Robustness: the inset is only meaningful while a text field holds focus, so
 * we gate the measured value on `activeElement` being editable. That turns the
 * iOS "stuck inset" failure mode — Safari sometimes leaves a shrunken
 * `visualViewport` / non-zero `offsetTop` behind after the keyboard dismisses,
 * or simply stops firing `resize`/`scroll` — into a guaranteed reset: no field
 * focused → inset 0. A short cascade of re-checks after `focusout` also rides
 * out the ~250ms iOS close animation in case no final viewport event arrives.
 */
function isEditableTarget(): boolean {
	const el = document.activeElement;
	if (!el) return false;
	const tag = el.tagName;
	return (
		tag === "TEXTAREA" ||
		tag === "INPUT" ||
		(el as HTMLElement).isContentEditable
	);
}

export function useKeyboardInset(): void {
	React.useEffect(() => {
		const vv =
			typeof window !== "undefined" ? window.visualViewport : undefined;
		// No VisualViewport (older browsers / SSR): degrade to today's
		// behaviour — the var stays unset (defaults to 0) so the layout is
		// unchanged.
		if (!vv) return;

		const root = document.documentElement;
		// Keyboards are >200px; a small floor keeps sub-pixel rounding and the
		// dynamic URL bar from flipping the open/closed flag.
		const OPEN_THRESHOLD = 80;
		let raf = 0;
		let wasOpen = false;
		let settleTimers: ReturnType<typeof setTimeout>[] = [];

		const apply = () => {
			raf = 0;
			const measured = Math.max(
				0,
				window.innerHeight - vv.height - vv.offsetTop,
			);
			// Only honour the keyboard height while a field is focused; otherwise
			// force 0 so a lingering viewport offset can never strand the shell in
			// its collapsed state.
			const inset = isEditableTarget() ? measured : 0;
			const open = inset >= OPEN_THRESHOLD;
			root.style.setProperty("--keyboard-inset", open ? `${inset}px` : "0px");
			if (open) {
				root.setAttribute("data-keyboard-open", "");
				// iOS may have scrolled the document up to reveal the focused field
				// before the open-state scroll lock engaged; pull it back to the top
				// once per open so the chat (not the page below it) fills the space
				// above the keyboard.
				if (!wasOpen) window.scrollTo(0, 0);
			} else {
				root.removeAttribute("data-keyboard-open");
			}
			wasOpen = open;
		};

		const schedule = () => {
			if (raf) return;
			raf = window.requestAnimationFrame(apply);
		};

		// iOS dismiss animates over ~250ms and can emit no closing viewport
		// event — recheck a few times after focus leaves so the inset settles
		// back to 0 even when the viewport stays silent.
		const settle = () => {
			settleTimers.forEach(clearTimeout);
			settleTimers = [120, 300, 600].map((d) => setTimeout(schedule, d));
		};

		apply();
		vv.addEventListener("resize", schedule);
		vv.addEventListener("scroll", schedule);
		document.addEventListener("focusin", schedule);
		document.addEventListener("focusout", settle);
		// Returning from the bfcache / app switch can restore a stale inset.
		window.addEventListener("pageshow", schedule);
		window.addEventListener("orientationchange", settle);
		return () => {
			if (raf) cancelAnimationFrame(raf);
			settleTimers.forEach(clearTimeout);
			vv.removeEventListener("resize", schedule);
			vv.removeEventListener("scroll", schedule);
			document.removeEventListener("focusin", schedule);
			document.removeEventListener("focusout", settle);
			window.removeEventListener("pageshow", schedule);
			window.removeEventListener("orientationchange", settle);
			root.style.removeProperty("--keyboard-inset");
			root.removeAttribute("data-keyboard-open");
		};
	}, []);
}
