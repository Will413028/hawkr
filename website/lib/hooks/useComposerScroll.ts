"use client";

import * as React from "react";

/**
 * Own the scroll position of a fixed-height composer.
 *
 * Two behaviours, one owner — they write the same scroll offset, so splitting
 * them across two hooks would let them fight:
 *
 *   focused  → keep the caret in view while typing
 *   idle     → walk through the overflowing draft so a message too big for the
 *              box can still be read back
 *
 * The composers are one line tall and stay that way — a product decision, not
 * an oversight, so this hook never touches the element's height. What it fixes
 * is that in a controlled React textarea you could not see what you were
 * typing at all: every keystroke reassigns `.value`, which resets the scroll
 * offset to 0, so the view snapped back to the start while the caret moved on.
 * Measured on a 390px viewport before the fix: `scrollTop` sat at 0 against a
 * `maxScroll` of 45.
 *
 * Programmatic scrolling is the only lever available. Chrome does not scroll a
 * textarea in response to `setSelectionRange` (verified: re-applying, and even
 * changing-then-restoring, the selection leaves the offset untouched), so the
 * caret's position is measured here and the offset written directly.
 *
 * The measurement uses a hidden mirror because a textarea exposes no caret
 * geometry. Jumping to the far end would be a one-line alternative but is only
 * right while the caret sits at the end of the text — it throws the view to the
 * end when someone clicks back into the middle of a draft.
 */

/**
 * Which way the composer overflows.
 *
 * `block`  — the textarea wraps, so overflow is vertical and the reveal walks
 *            down a line at a time.
 * `inline` — the textarea is `white-space: nowrap`, so the draft is one long
 *            line and both the caret-follow and the reveal run horizontally.
 */
export type ComposerAxis = "block" | "inline";

interface AxisOps {
	/** Current scroll offset along the axis. */
	get(el: HTMLTextAreaElement): number;
	set(el: HTMLTextAreaElement, next: number): void;
	/** Smooth-scroll, used by the idle reveal. */
	scrollTo(el: HTMLTextAreaElement, next: number): void;
	/** Visible extent along the axis. */
	viewport(el: HTMLTextAreaElement): number;
	/** Total content extent along the axis. */
	content(el: HTMLTextAreaElement): number;
	/** How far the reveal advances per hold. */
	stride(el: HTMLTextAreaElement, style: CSSStyleDeclaration): number;
	/**
	 * Space the caret needs beyond its own position before it counts as
	 * visible: a whole line vertically, or a little breathing room ahead of it
	 * horizontally so it never sits flush against the edge.
	 */
	caretExtent(style: CSSStyleDeclaration): number;
	/** Caret offset along the axis, measured via the mirror. */
	caret(mirrorMarker: HTMLSpanElement, style: CSSStyleDeclaration): number;
	/** Mirror geometry that reproduces this axis's wrapping. */
	mirrorCss(el: HTMLTextAreaElement, style: CSSStyleDeclaration): string;
}

const CARET_LOOKAHEAD_PX = 24;

const AXES: Record<ComposerAxis, AxisOps> = {
	block: {
		get: (el) => el.scrollTop,
		set: (el, next) => {
			el.scrollTop = next;
		},
		scrollTo: (el, next) => el.scrollTo({ top: next, behavior: "smooth" }),
		viewport: (el) => el.clientHeight,
		content: (el) => el.scrollHeight,
		stride: (_el, style) => Number.parseFloat(style.lineHeight) || 20,
		caretExtent: (style) => Number.parseFloat(style.lineHeight) || 20,
		caret: (m, style) =>
			(Number.parseFloat(style.paddingTop) || 0) + m.offsetTop,
		mirrorCss: (el, style) => {
			const padLeft = Number.parseFloat(style.paddingLeft) || 0;
			const padRight = Number.parseFloat(style.paddingRight) || 0;
			return (
				"white-space:pre-wrap;overflow-wrap:break-word;" +
				`width:${el.clientWidth - padLeft - padRight}px;`
			);
		},
	},
	inline: {
		get: (el) => el.scrollLeft,
		set: (el, next) => {
			el.scrollLeft = next;
		},
		scrollTo: (el, next) => el.scrollTo({ left: next, behavior: "smooth" }),
		viewport: (el) => el.clientWidth,
		content: (el) => el.scrollWidth,
		// Advance by most of a screenful, keeping a sliver of overlap so the eye
		// can carry the sentence across the jump.
		stride: (el) => Math.max(48, el.clientWidth * 0.6),
		caretExtent: () => CARET_LOOKAHEAD_PX,
		caret: (m, style) =>
			(Number.parseFloat(style.paddingLeft) || 0) + m.offsetLeft,
		mirrorCss: () => "white-space:pre;width:max-content;",
	},
};

/* ── Idle reveal ────────────────────────────────────────────────────────── */

/**
 * A one-line box cannot show an overflowing draft, so when the composer is
 * left alone it walks through it instead — a stride at a time, with long
 * holds, so the text is read rather than watched.
 *
 * Deliberately conservative about when it runs. Moving text inside an editable
 * field is hostile if it can start while someone is working, and WCAG 2.2.2
 * requires a way to stop motion that runs past five seconds: focus, hover,
 * pointer contact, or any edit cancels it outright, and it only ever starts
 * after the composer has been idle. Reduced-motion users never see it.
 */
const IDLE_START_MS = 3_000;
const LINE_HOLD_MS = 2_400;
const END_HOLD_MS = 3_600;

function useIdleReveal(
	ref: React.RefObject<HTMLTextAreaElement | null>,
	value: string,
	axis: ComposerAxis,
): void {
	// biome-ignore lint/correctness/useExhaustiveDependencies: value re-arms the cycle rather than being read inside — a new draft is new content to reveal, and the idle countdown has to restart from the last edit.
	React.useEffect(() => {
		const el = ref.current;
		if (!el) return;
		const ops = AXES[axis];
		if (
			window.matchMedia("(prefers-reduced-motion: reduce)").matches ||
			ops.content(el) <= ops.viewport(el)
		) {
			return;
		}

		const stride = ops.stride(el, window.getComputedStyle(el));
		let timer: number | undefined;
		let stopped = false;

		const step = () => {
			if (stopped || document.activeElement === el) return;
			const maxScroll = ops.content(el) - ops.viewport(el);
			// One stride further, or back to the start once the end has been held.
			const atEnd = ops.get(el) >= maxScroll - 1;
			const next = atEnd ? 0 : Math.min(ops.get(el) + stride, maxScroll);
			ops.scrollTo(el, next);
			timer = window.setTimeout(
				step,
				// The end earns the longer pause — it carries whatever the buyer
				// typed most recently.
				next >= maxScroll - 1 ? END_HOLD_MS : LINE_HOLD_MS,
			);
		};

		const stop = () => {
			stopped = true;
			if (timer !== undefined) window.clearTimeout(timer);
		};
		const restart = () => {
			stop();
			stopped = false;
			timer = window.setTimeout(step, IDLE_START_MS);
		};

		// Any sign of attention cancels the motion and hands the scroll back.
		el.addEventListener("focus", stop);
		el.addEventListener("pointerenter", stop);
		el.addEventListener("pointerdown", stop);
		el.addEventListener("blur", restart);

		if (document.activeElement !== el) restart();

		return () => {
			stop();
			el.removeEventListener("focus", stop);
			el.removeEventListener("pointerenter", stop);
			el.removeEventListener("pointerdown", stop);
			el.removeEventListener("blur", restart);
		};
	}, [ref, value, axis]);
}

/** One mirror for the whole app: it is written and read synchronously, never
 *  concurrently, and creating one per composer would leak nodes into <body>. */
let mirror: HTMLDivElement | null = null;
let marker: HTMLSpanElement | null = null;

function measureCaret(
	el: HTMLTextAreaElement,
	style: CSSStyleDeclaration,
	axis: ComposerAxis,
): number {
	if (!mirror || !marker) {
		mirror = document.createElement("div");
		mirror.setAttribute("aria-hidden", "true");
		marker = document.createElement("span");
		// Zero-width space: occupies a caret position without adding width.
		marker.textContent = "​";
		document.body.appendChild(mirror);
	}

	// No padding and no border on the mirror, so the offsets come back purely
	// content-relative; the textarea's own padding is added back by the axis.
	// Copying the padding onto the mirror instead made `offsetTop` include it
	// and the caret landed ~11px off.
	mirror.style.cssText =
		"position:absolute;top:-9999px;left:0;visibility:hidden;pointer-events:none;" +
		"margin:0;padding:0;border:0;box-sizing:content-box;" +
		`font:${style.font};letter-spacing:${style.letterSpacing};` +
		`text-transform:${style.textTransform};` +
		AXES[axis].mirrorCss(el, style);
	// Everything up to the caret, then the marker: the marker sits exactly
	// where the caret does.
	mirror.textContent = el.value.slice(0, el.selectionEnd);
	mirror.appendChild(marker);

	return AXES[axis].caret(marker, style);
}

export function useComposerScroll(
	ref: React.RefObject<HTMLTextAreaElement | null>,
	/** Re-run on every draft change, including the reset to "" after a send. */
	value: string,
	/** Must match the textarea's `white-space`. See {@link ComposerAxis}. */
	axis: ComposerAxis = "block",
): void {
	useIdleReveal(ref, value, axis);

	// While an IME is composing, the committed value and the caret disagree
	// (注音 / 拼音 candidates are still provisional). Scrolling on those
	// intermediate states fights the input method, so hold until it commits.
	const composingRef = React.useRef(false);

	React.useEffect(() => {
		const el = ref.current;
		if (!el) return;
		const start = () => {
			composingRef.current = true;
		};
		const end = () => {
			composingRef.current = false;
		};
		el.addEventListener("compositionstart", start);
		el.addEventListener("compositionend", end);
		return () => {
			el.removeEventListener("compositionstart", start);
			el.removeEventListener("compositionend", end);
		};
	}, [ref]);

	// Layout effect: the scroll lands in the same frame as the character that
	// caused it, so the caret never flashes out of view.
	// biome-ignore lint/correctness/useExhaustiveDependencies: value is the re-run trigger, not a value read inside — the text itself lives in the DOM node. It must stay: React's write to `.value` is what resets the scroll offset, and this is what undoes it.
	React.useLayoutEffect(() => {
		const el = ref.current;
		// No caret to follow when the composer isn't focused, and nothing to
		// measure when it has no layout box.
		if (!el || composingRef.current) return;
		if (document.activeElement !== el || el.clientWidth === 0) return;

		const ops = AXES[axis];
		const style = window.getComputedStyle(el);
		const extent = ops.caretExtent(style);
		if (!Number.isFinite(extent)) return;

		const caret = measureCaret(el, style, axis);
		const offset = ops.get(el);
		if (caret < offset) {
			ops.set(el, caret);
		} else if (caret + extent > offset + ops.viewport(el)) {
			ops.set(el, caret + extent - ops.viewport(el));
		}
	}, [ref, value, axis]);
}
