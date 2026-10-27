import { describe, expect, it } from "vitest";
import { buildCsvContent } from "./export";

describe("buildCsvContent", () => {
	it("joins rows with commas and newlines", () => {
		expect(
			buildCsvContent([
				["a", "b"],
				["1", "2"],
			]),
		).toBe('"a","b"\n"1","2"');
	});

	it("escapes embedded double quotes by doubling them", () => {
		expect(buildCsvContent([['he said "hi"']])).toBe('"he said ""hi"""');
	});

	it("returns an empty string for no rows", () => {
		expect(buildCsvContent([])).toBe("");
	});
});
