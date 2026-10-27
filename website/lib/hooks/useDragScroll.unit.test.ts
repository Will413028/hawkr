/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDragScroll } from "./useDragScroll";

function Rail({ onActivate }: { onActivate: () => void }) {
	const ref = React.useRef<HTMLDivElement>(null);
	useDragScroll(ref);
	return React.createElement(
		"div",
		{ ref },
		React.createElement(
			"button",
			{ type: "button", onClick: onActivate },
			"分類",
		),
	);
}

function pointer(el: Element, type: string, x: number, pointerType = "mouse") {
	const event = new MouseEvent(type, { bubbles: true, clientX: x, button: 0 });
	Object.defineProperties(event, {
		pointerType: { value: pointerType },
		pointerId: { value: 1 },
	});
	el.dispatchEvent(event);
}

function setup() {
	const onActivate = vi.fn();
	const { container, unmount } = render(
		React.createElement(Rail, { onActivate }),
	);
	const rail = container.firstElementChild as HTMLDivElement;
	const button = rail.querySelector("button") as HTMLButtonElement;
	rail.setPointerCapture = vi.fn();
	rail.releasePointerCapture = vi.fn();
	const click = (detail: number) => {
		const event = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			detail,
		});
		button.dispatchEvent(event);
		return event;
	};
	const drag = () => {
		pointer(button, "pointerdown", 100);
		pointer(button, "pointermove", 60);
		pointer(button, "pointerup", 60);
	};
	return { rail, button, onActivate, click, drag, unmount };
}

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe("useDragScroll activation ownership", () => {
	it("allows keyboard and assistive zero-detail clicks after a mouse drag", () => {
		const { drag, click, onActivate } = setup();
		drag();
		expect(click(1).defaultPrevented).toBe(true);
		expect(onActivate).not.toHaveBeenCalled();
		expect(click(0).defaultPrevented).toBe(false);
		expect(click(0).defaultPrevented).toBe(false);
		expect(onActivate).toHaveBeenCalledTimes(2);
	});

	it("still suppresses a trailing mouse click and allows the next deliberate click", () => {
		const { drag, click, button, onActivate } = setup();
		drag();
		expect(click(1).defaultPrevented).toBe(true);
		pointer(button, "pointerdown", 60);
		pointer(button, "pointerup", 60);
		expect(click(1).defaultPrevented).toBe(false);
		expect(onActivate).toHaveBeenCalledTimes(1);
	});

	it("does not suppress touch activation after native scrolling or a previous mouse drag", () => {
		const { drag, click, button, rail, onActivate } = setup();
		drag();
		rail.scrollLeft = 250;
		pointer(button, "pointerdown", 20, "touch");
		pointer(button, "pointerup", 20, "touch");
		expect(click(1).defaultPrevented).toBe(false);
		expect(onActivate).toHaveBeenCalledTimes(1);
	});
});
