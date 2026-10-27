// @vitest-environment jsdom

import { act, cleanup, renderHook } from "@testing-library/react";
import type { MotionValue } from "framer-motion";
import type { KeyboardEvent, PointerEvent } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGalleryCardMotion } from "./useGalleryCardMotion";

const { animateMock } = vi.hoisted(() => ({ animateMock: vi.fn() }));

// Keep real MotionValues and gesture logic. Only finish the animation clock
// immediately so these tests assert destinations without wall-clock timing.
vi.mock("framer-motion", async (importOriginal) => ({
	...(await importOriginal<typeof import("framer-motion")>()),
	animate: animateMock,
}));

let frame: HTMLElement;
let reducedMotion: boolean;
let motionQuery: MediaQueryList;
let captured: Set<number>;

function pointer(
	x: number,
	y = 100,
	overrides: Partial<PointerEvent<HTMLElement>> = {},
): PointerEvent<HTMLElement> {
	return {
		pointerId: 1,
		pointerType: "touch",
		isPrimary: true,
		button: 0,
		clientX: x,
		clientY: y,
		currentTarget: frame,
		preventDefault: vi.fn(),
		...overrides,
	} as PointerEvent<HTMLElement>;
}

function key(key: string): KeyboardEvent<HTMLElement> {
	return {
		key,
		preventDefault: vi.fn(),
	} as unknown as KeyboardEvent<HTMLElement>;
}

function setup(count = 4) {
	const ref = { current: frame };
	return renderHook(({ count }) => useGalleryCardMotion(count, ref), {
		initialProps: { count },
	});
}

beforeEach(() => {
	reducedMotion = false;
	captured = new Set();
	frame = document.createElement("section");
	// 388px card + 12px gap makes one card's travel exactly 400px.
	vi.spyOn(frame, "getBoundingClientRect").mockReturnValue(
		new DOMRect(0, 0, 388, 291),
	);
	frame.setPointerCapture = vi.fn((id) => captured.add(id));
	frame.hasPointerCapture = vi.fn((id) => captured.has(id));
	frame.releasePointerCapture = vi.fn((id) => captured.delete(id));
	motionQuery = Object.assign(new EventTarget(), {
		matches: reducedMotion,
		media: "(prefers-reduced-motion: reduce)",
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
	}) as MediaQueryList;
	Object.defineProperty(motionQuery, "matches", { get: () => reducedMotion });
	vi.stubGlobal(
		"matchMedia",
		vi.fn(() => motionQuery),
	);
	animateMock
		.mockReset()
		.mockImplementation((position: MotionValue<number>, target: number) => {
			position.set(target);
			return { stop: vi.fn() };
		});
});

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
});

describe("useGalleryCardMotion gestures", () => {
	it("follows horizontal movement and advances at the 48px release threshold", () => {
		const { result } = setup();
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(152)));
		expect(result.current.position.get()).toBeCloseTo(0.12);
		expect(result.current.dragging).toBe(true);
		expect(result.current.activeIndex).toBe(0);
		expect(frame.setPointerCapture).toHaveBeenCalledWith(1);
		act(() => result.current.handlers.onPointerUp(pointer(152)));
		expect(result.current.activeIndex).toBe(1);
		expect(result.current.position.get()).toBe(1);
		expect(result.current.dragging).toBe(false);
		expect(frame.releasePointerCapture).toHaveBeenCalledWith(1);
	});

	it("returns to the current card below the threshold and after dragging back", () => {
		const { result } = setup();
		act(() => result.current.select(1));
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(153)));
		expect(result.current.position.get()).toBeGreaterThan(1);
		act(() => result.current.handlers.onPointerUp(pointer(153)));
		expect(result.current.activeIndex).toBe(1);
		expect(result.current.position.get()).toBe(1);
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(100)));
		act(() => result.current.handlers.onPointerMove(pointer(198)));
		act(() => result.current.handlers.onPointerUp(pointer(198)));
		expect(result.current.position.get()).toBe(1);
		expect(result.current.activeIndex).toBe(1);
	});

	it("applies resistance at both edges and never selects outside the gallery", () => {
		const { result } = setup(3);
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(300)));
		expect(result.current.position.get()).toBeLessThan(0);
		expect(result.current.position.get()).toBeGreaterThan(-0.25);
		act(() => result.current.handlers.onPointerUp(pointer(300)));
		expect(result.current.activeIndex).toBe(0);
		act(() => result.current.select(2));
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(100)));
		expect(result.current.position.get()).toBeGreaterThan(2);
		expect(result.current.position.get()).toBeLessThan(2.25);
		act(() => result.current.handlers.onPointerUp(pointer(100)));
		expect(result.current.activeIndex).toBe(2);
	});

	it("ignores vertical scrolling and releases capture without blocking the browser", () => {
		const { result } = setup();
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		const vertical = pointer(190, 170);
		act(() => result.current.handlers.onPointerMove(vertical));
		expect(result.current.position.get()).toBe(0);
		expect(result.current.dragging).toBe(false);
		expect(vertical.preventDefault).not.toHaveBeenCalled();
		expect(frame.releasePointerCapture).toHaveBeenCalledWith(1);
		act(() => result.current.handlers.onPointerMove(pointer(80, 170)));
		act(() => result.current.handlers.onPointerUp(pointer(80, 170)));
		expect(result.current.activeIndex).toBe(0);
		expect(result.current.position.get()).toBe(0);
	});

	it("ignores secondary buttons and a second pointer during a primary gesture", () => {
		const { result } = setup();
		act(() =>
			result.current.handlers.onPointerDown(pointer(200, 100, { button: 2 })),
		);
		act(() =>
			result.current.handlers.onPointerDown(
				pointer(200, 100, { isPrimary: false }),
			),
		);
		expect(frame.setPointerCapture).not.toHaveBeenCalled();
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() =>
			result.current.handlers.onPointerDown(
				pointer(200, 100, { pointerId: 2 }),
			),
		);
		act(() =>
			result.current.handlers.onPointerMove(pointer(80, 100, { pointerId: 2 })),
		);
		act(() =>
			result.current.handlers.onPointerUp(pointer(80, 100, { pointerId: 2 })),
		);
		expect(result.current.position.get()).toBe(0);
		expect(result.current.activeIndex).toBe(0);
		expect(frame.releasePointerCapture).not.toHaveBeenCalled();
		act(() => result.current.handlers.onPointerMove(pointer(80)));
		act(() => result.current.handlers.onPointerUp(pointer(80)));
		expect(result.current.activeIndex).toBe(1);
	});

	it.each([
		"onPointerCancel",
		"onLostPointerCapture",
	] as const)("snaps back after %s and accepts the next gesture", (handler) => {
		const { result } = setup();
		act(() => result.current.select(1));
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(80)));
		expect(result.current.position.get()).toBeGreaterThan(1);
		act(() => result.current.handlers[handler](pointer(80)));
		expect(result.current.position.get()).toBe(1);
		expect(result.current.activeIndex).toBe(1);
		expect(result.current.dragging).toBe(false);
		act(() => result.current.handlers.onPointerUp(pointer(80)));
		expect(result.current.activeIndex).toBe(1);
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(80)));
		act(() => result.current.handlers.onPointerUp(pointer(80)));
		expect(result.current.activeIndex).toBe(2);
	});

	it("uses the latest selection for rapid keyboard and thumbnail commands before animations finish", () => {
		animateMock.mockImplementation(() => ({ stop: vi.fn() }));
		const { result } = setup(5);
		const stop = vi.spyOn(result.current.position, "stop");
		const right = key("ArrowRight");
		act(() => result.current.handlers.onKeyDown(right));
		act(() => result.current.handlers.onKeyDown(key("ArrowRight")));
		expect(result.current.activeIndex).toBe(2);
		act(() => result.current.select(4));
		act(() => result.current.handlers.onKeyDown(key("ArrowLeft")));
		expect(result.current.activeIndex).toBe(3);
		act(() => result.current.handlers.onKeyDown(key("Home")));
		expect(result.current.activeIndex).toBe(0);
		act(() => result.current.handlers.onKeyDown(key("End")));
		expect(result.current.activeIndex).toBe(4);
		expect(animateMock.mock.calls.map(([, target]) => target)).toEqual([
			1, 2, 4, 3, 0, 4,
		]);
		expect(stop).toHaveBeenCalledTimes(6);
		expect(right.preventDefault).toHaveBeenCalledOnce();
		const unrelated = key("Tab");
		act(() => result.current.handlers.onKeyDown(unrelated));
		expect(unrelated.preventDefault).not.toHaveBeenCalled();
		expect(result.current.activeIndex).toBe(4);
	});

	it("switches directly with reduced motion and avoids following the pointer", () => {
		reducedMotion = true;
		const { result } = setup();
		expect(result.current.reducedMotion).toBe(true);
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(80)));
		expect(result.current.position.get()).toBe(0);
		act(() => result.current.handlers.onPointerUp(pointer(80)));
		expect(result.current.activeIndex).toBe(1);
		expect(result.current.position.get()).toBe(1);
		act(() => result.current.handlers.onKeyDown(key("ArrowRight")));
		expect(result.current.position.get()).toBe(2);
		act(() => result.current.select(0));
		expect(result.current.position.get()).toBe(0);
		expect(animateMock).not.toHaveBeenCalled();
	});

	it("snaps to the selected card if reduced motion is enabled during a drag", () => {
		const { result } = setup();
		act(() => result.current.select(1));
		act(() => result.current.handlers.onPointerDown(pointer(200)));
		act(() => result.current.handlers.onPointerMove(pointer(80)));
		expect(result.current.position.get()).toBeGreaterThan(1);
		act(() => {
			reducedMotion = true;
			motionQuery.dispatchEvent(new Event("change"));
		});
		expect(result.current.position.get()).toBe(1);
		expect(result.current.dragging).toBe(false);
		act(() => result.current.handlers.onPointerUp(pointer(80)));
		expect(result.current.activeIndex).toBe(1);
	});

	it("resets removed selections and stops its MotionValue on unmount", () => {
		const { result, rerender, unmount } = setup();
		act(() => result.current.select(3));
		rerender({ count: 2 });
		expect(result.current.activeIndex).toBe(0);
		expect(result.current.position.get()).toBe(0);
		const stop = vi.spyOn(result.current.position, "stop");
		unmount();
		expect(stop).toHaveBeenCalledOnce();
	});
});
