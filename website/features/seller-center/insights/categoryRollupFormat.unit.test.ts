import { describe, expect, it } from "vitest";
import { computeRollupChange } from "./categoryRollupFormat";

describe("computeRollupChange", () => {
	it("returns null when either price is missing — no fabricated change", () => {
		expect(computeRollupChange(undefined, 40)).toBeNull();
		expect(computeRollupChange(45, undefined)).toBeNull();
		expect(computeRollupChange(undefined, undefined)).toBeNull();
	});

	it("computes an upward change", () => {
		expect(computeRollupChange(45, 40)).toEqual({
			absolute: 5,
			percent: 12.5,
			direction: "up",
		});
	});

	it("computes a downward change", () => {
		expect(computeRollupChange(38, 40)).toEqual({
			absolute: -2,
			percent: -5,
			direction: "down",
		});
	});

	it("reports flat when the price didn't move", () => {
		expect(computeRollupChange(40, 40)).toEqual({
			absolute: 0,
			percent: 0,
			direction: "flat",
		});
	});

	it("does not divide by zero when the previous price was zero", () => {
		expect(computeRollupChange(5, 0)).toEqual({
			absolute: 5,
			percent: 0,
			direction: "up",
		});
	});
});
