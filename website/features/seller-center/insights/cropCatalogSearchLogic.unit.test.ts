import { describe, expect, it } from "vitest";
import {
	computeDebounceDelay,
	resolveCropCatalogKeyAction,
} from "./cropCatalogSearchLogic";

describe("computeDebounceDelay", () => {
	it("returns 0 for an empty query (no debounce needed — the empty-query request is cheap and cached)", () => {
		expect(computeDebounceDelay("")).toBe(0);
	});

	it("returns 250ms for a non-empty query, matching HomeCompactSearch's existing debounce", () => {
		expect(computeDebounceDelay("小黃瓜")).toBe(250);
	});
});

describe("resolveCropCatalogKeyAction", () => {
	it("starts at the first option with ArrowDown and wraps past the last", () => {
		expect(resolveCropCatalogKeyAction("ArrowDown", null, 3)).toEqual({
			kind: "move",
			index: 0,
		});
		expect(resolveCropCatalogKeyAction("ArrowDown", 2, 3)).toEqual({
			kind: "move",
			index: 0,
		});
	});

	it("starts at the last option with ArrowUp and wraps before the first", () => {
		expect(resolveCropCatalogKeyAction("ArrowUp", null, 3)).toEqual({
			kind: "move",
			index: 2,
		});
		expect(resolveCropCatalogKeyAction("ArrowUp", 0, 3)).toEqual({
			kind: "move",
			index: 2,
		});
	});

	it("selects only a valid active option on Enter", () => {
		expect(resolveCropCatalogKeyAction("Enter", 1, 3)).toEqual({
			kind: "select",
			index: 1,
		});
		expect(resolveCropCatalogKeyAction("Enter", null, 3)).toEqual({
			kind: "none",
		});
		expect(resolveCropCatalogKeyAction("Enter", 3, 3)).toEqual({
			kind: "none",
		});
	});

	it("closes on Escape and ignores navigation when the list is empty", () => {
		expect(resolveCropCatalogKeyAction("Escape", 0, 3)).toEqual({
			kind: "close",
		});
		expect(resolveCropCatalogKeyAction("ArrowDown", null, 0)).toEqual({
			kind: "none",
		});
		expect(resolveCropCatalogKeyAction("Tab", 0, 3)).toEqual({
			kind: "none",
		});
	});
});
