"use client";

import * as React from "react";

/**
 * Mouse drag-to-scroll with IG-style momentum for horizontal rails.
 * Touch is intentionally skipped — native `overflow-x: auto` already
 * gives buttery inertia on touch devices, and intercepting it makes
 * things worse.
 *
 * Mouse path:
 *   1. pointerdown — capture pointer, kill any in-flight inertia
 *   2. pointermove — drive `scrollLeft` directly + sample (t, x) into a
 *      ring buffer used to estimate release velocity
 *   3. pointerup   — compute px/ms over the last ~100ms and start a RAF
 *      loop that decays the velocity each frame (friction ≈ 0.94),
 *      bailing once it drops below 0.02 px/ms
 *
 * A drag past 5px also suppresses the trailing `click` (capture phase)
 * so the user doesn't accidentally activate a child link mid-drag.
 *
 * Native browser drag (the URL ghost from `<a>`/`<img>`) is suppressed
 * via `dragstart` preventDefault — pair with `user-drag: none` in CSS
 * for browsers where dragstart races differently.
 */
export function useDragScroll(ref: React.RefObject<HTMLElement | null>) {
	React.useEffect(() => {
		const el = ref.current;
		if (!el) return;

		let dragging = false;
		let captured = false;
		let committedDrag = false;
		let startX = 0;
		let startScrollLeft = 0;
		let moved = 0;
		let lastPointerType: string | null = null;
		let activePointerId: number | null = null;
		let rafId: number | null = null;
		const samples: Array<{ t: number; x: number }> = [];
		const DRAG_COMMIT_PX = 6;

		const cancelInertia = () => {
			if (rafId !== null) {
				cancelAnimationFrame(rafId);
				rafId = null;
			}
		};

		const startInertia = (velocityPxPerMs: number) => {
			let v = velocityPxPerMs;
			let last = performance.now();
			const step = (now: number) => {
				const dt = now - last;
				last = now;
				el.scrollLeft -= v * dt;
				v *= 0.94 ** (dt / 16.67);
				if (Math.abs(v) < 0.02) {
					rafId = null;
					return;
				}
				rafId = requestAnimationFrame(step);
			};
			rafId = requestAnimationFrame(step);
		};

		const pushSample = (x: number) => {
			const now = performance.now();
			samples.push({ t: now, x });
			const cutoff = now - 100;
			while (samples.length > 1 && samples[0].t < cutoff) samples.shift();
		};

		const onPointerDown = (e: PointerEvent) => {
			lastPointerType = e.pointerType;
			if (e.pointerType !== "mouse" || e.button !== 0) return;
			cancelInertia();
			dragging = true;
			captured = false;
			committedDrag = false;
			moved = 0;
			startX = e.clientX;
			startScrollLeft = el.scrollLeft;
			activePointerId = e.pointerId;
			samples.length = 0;
			pushSample(e.clientX);
		};

		const onPointerMove = (e: PointerEvent) => {
			if (!dragging || e.pointerId !== activePointerId) return;
			const dx = e.clientX - startX;
			if (Math.abs(dx) > moved) moved = Math.abs(dx);
			if (!captured && moved > DRAG_COMMIT_PX) {
				try {
					el.setPointerCapture(e.pointerId);
					captured = true;
				} catch {}
				committedDrag = true;
				el.style.cursor = "grabbing";
			}
			if (captured) {
				el.scrollLeft = startScrollLeft - dx;
			}
			pushSample(e.clientX);
		};

		const stop = (e: PointerEvent) => {
			if (!dragging || e.pointerId !== activePointerId) return;
			dragging = false;
			activePointerId = null;
			el.style.cursor = "";
			if (captured) {
				try {
					el.releasePointerCapture(e.pointerId);
				} catch {}
				captured = false;
			}
			if (samples.length >= 2) {
				const first = samples[0];
				const last = samples[samples.length - 1];
				const dt = last.t - first.t;
				if (dt > 0) {
					const velocity = (last.x - first.x) / dt;
					if (Math.abs(velocity) > 0.05) startInertia(velocity);
				}
			}
		};

		const onClickCapture = (e: MouseEvent) => {
			if (e.detail === 0) return;
			if (lastPointerType !== "mouse") return;
			const scrollDelta = Math.abs(el.scrollLeft - startScrollLeft);
			const dragged = committedDrag || scrollDelta > 4 || moved > 12;
			if (dragged) {
				e.stopPropagation();
				e.preventDefault();
			}
			moved = 0;
		};

		const onWheel = () => {
			cancelInertia();
		};

		const onDragStart = (e: DragEvent) => {
			e.preventDefault();
		};

		el.addEventListener("pointerdown", onPointerDown);
		el.addEventListener("pointermove", onPointerMove);
		el.addEventListener("pointerup", stop);
		el.addEventListener("pointercancel", stop);
		el.addEventListener("click", onClickCapture, true);
		el.addEventListener("wheel", onWheel, { passive: true });
		el.addEventListener("dragstart", onDragStart);

		return () => {
			cancelInertia();
			el.removeEventListener("pointerdown", onPointerDown);
			el.removeEventListener("pointermove", onPointerMove);
			el.removeEventListener("pointerup", stop);
			el.removeEventListener("pointercancel", stop);
			el.removeEventListener("click", onClickCapture, true);
			el.removeEventListener("wheel", onWheel);
			el.removeEventListener("dragstart", onDragStart);
		};
	}, [ref]);
}
