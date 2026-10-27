import { describe, expect, it } from "vitest";
import {
	computeValueRange,
	filterPointsByRange,
	generateDateTicks,
	generatePriceTicks,
	nearestPointIndex,
	pointsToBandPath,
	pointsToLinePath,
	xForDate,
	yForValue,
} from "./priceTrendChartMath";

describe("xForDate", () => {
	it("maps the min date to x=0 and max date to x=width", () => {
		expect(xForDate("2026-07-01", "2026-07-01", "2026-07-31", 300)).toBe(0);
		expect(xForDate("2026-07-31", "2026-07-01", "2026-07-31", 300)).toBe(300);
	});

	it("maps a midpoint date to roughly half the width", () => {
		const x = xForDate("2026-07-16", "2026-07-01", "2026-07-31", 300);
		expect(x).toBeGreaterThan(140);
		expect(x).toBeLessThan(160);
	});

	it("returns width/2 when min and max dates are identical", () => {
		expect(xForDate("2026-07-01", "2026-07-01", "2026-07-01", 300)).toBe(150);
	});
});

describe("yForValue", () => {
	it("maps minValue to height (SVG y-down: low value = bottom)", () => {
		expect(yForValue(10, 10, 20, 100)).toBe(100);
	});

	it("maps maxValue to y=0 (top)", () => {
		expect(yForValue(20, 10, 20, 100)).toBe(0);
	});

	it("returns height/2 when minValue and maxValue are identical", () => {
		expect(yForValue(15, 15, 15, 100)).toBe(50);
	});
});

const BOUNDS = {
	width: 100,
	height: 100,
	minValue: 10,
	maxValue: 20,
	minDate: "2026-07-01",
	maxDate: "2026-07-02",
};

describe("pointsToLinePath", () => {
	it("returns an empty string for zero points", () => {
		expect(pointsToLinePath([], BOUNDS)).toBe("");
	});

	it("starts with M and continues with L for each subsequent point", () => {
		const path = pointsToLinePath(
			[
				{ date: "2026-07-01", value: 10 },
				{ date: "2026-07-02", value: 20 },
			],
			BOUNDS,
		);
		expect(path.startsWith("M")).toBe(true);
		expect(path).toContain("L");
		expect(path.split(" ")).toHaveLength(2);
	});
});

describe("pointsToBandPath", () => {
	it("returns an empty string for zero points", () => {
		expect(pointsToBandPath([], BOUNDS)).toBe("");
	});

	it("closes the path with Z and includes both the high (forward) and low (reversed) edges", () => {
		const path = pointsToBandPath(
			[
				{ date: "2026-07-01", low: 8, high: 12 },
				{ date: "2026-07-02", low: 9, high: 13 },
			],
			{ ...BOUNDS, minValue: 8, maxValue: 13 },
		);
		expect(path.endsWith("Z")).toBe(true);
		// 上緣 2 點 + 下緣（反向）2 點 = 4 個座標指令 + 結尾 "Z"
		expect(path.split(" ")).toHaveLength(5);
	});
});

describe("computeValueRange", () => {
	it("returns null for empty input", () => {
		expect(computeValueRange([])).toBeNull();
	});

	it("adds padding above and below the min/max", () => {
		const range = computeValueRange([10, 20]);
		expect(range).not.toBeNull();
		expect(range?.min).toBeLessThan(10);
		expect(range?.max).toBeGreaterThan(20);
	});

	it("pads a single repeated value so min !== max", () => {
		const range = computeValueRange([15, 15, 15]);
		expect(range).not.toBeNull();
		expect(range?.min).toBeLessThan(15);
		expect(range?.max).toBeGreaterThan(15);
	});

	it("never creates a negative price axis and expands an all-zero range", () => {
		expect(computeValueRange([0, 0])).toEqual({ min: 0, max: 1 });
		expect(computeValueRange([0.1, 0.2])?.min).toBeGreaterThanOrEqual(0);
	});
});

describe("generateDateTicks", () => {
	it("returns evenly-spaced dates including both endpoints", () => {
		const ticks = generateDateTicks("2026-08-01", "2026-08-08", 4);
		expect(ticks[0]).toBe("2026-08-01");
		expect(ticks.at(-1)).toBe("2026-08-08");
		expect(ticks.length).toBeLessThanOrEqual(4);
	});

	it("returns a single tick when minDate equals maxDate", () => {
		expect(generateDateTicks("2026-08-01", "2026-08-01", 4)).toEqual([
			"2026-08-01",
		]);
	});
});

describe("generatePriceTicks", () => {
	it("returns evenly-spaced values including both bounds", () => {
		const ticks = generatePriceTicks(0, 100, 5);
		expect(ticks[0]).toBe(0);
		expect(ticks.at(-1)).toBe(100);
		expect(ticks.length).toBe(5);
	});

	it("returns a single tick when minValue equals maxValue", () => {
		expect(generatePriceTicks(50, 50, 5)).toEqual([50]);
	});
});

describe("nearestPointIndex", () => {
	const points = [
		{ date: "2026-08-01", value: 10 },
		{ date: "2026-08-03", value: 20 },
		{ date: "2026-08-05", value: 30 },
	];

	it("returns the index of the exact date when present", () => {
		expect(nearestPointIndex(points, "2026-08-03")).toBe(1);
	});

	it("returns the nearest index when the date falls between points", () => {
		expect(nearestPointIndex(points, "2026-08-04")).toBe(1); // closer to 08-03 than 08-05? tie-break to earlier — see implementation note
	});

	it("returns -1 for an empty points array", () => {
		expect(nearestPointIndex([], "2026-08-01")).toBe(-1);
	});
});

describe("filterPointsByRange", () => {
	const points = [
		{ date: "2026-07-01", value: 10 },
		{ date: "2026-07-20", value: 20 },
		{ date: "2026-07-25", value: 25 },
		{ date: "2026-07-30", value: 30 },
	];

	it('returns all points unchanged for range="all"', () => {
		expect(filterPointsByRange(points, "all", "2026-07-30")).toEqual(points);
	});

	it("keeps only points within the trailing N days for a numeric range", () => {
		const result = filterPointsByRange(points, 7, "2026-07-30");
		expect(result).toEqual([
			{ date: "2026-07-25", value: 25 },
			{ date: "2026-07-30", value: 30 },
		]);
	});

	it("includes every point when the numeric range covers the full span", () => {
		const result = filterPointsByRange(points, 30, "2026-07-30");
		expect(result).toEqual(points);
	});

	it("returns an empty array when given an empty input", () => {
		expect(filterPointsByRange([], 7, "2026-07-30")).toEqual([]);
	});
});
