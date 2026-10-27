import { describe, expect, it } from "vitest";
import { categoryRollupPointsToObservations } from "./categoryRollupObservations";

describe("categoryRollupPointsToObservations", () => {
	it("maps rollup points to PriceObservation shape with placeholder identity fields", () => {
		const result = categoryRollupPointsToObservations([
			{ tradeDate: "2026-08-29", avgPrice: 44 },
			{ tradeDate: "2026-08-30", avgPrice: 45.2 },
		]);

		expect(result).toEqual([
			{ marketCode: "", cropCode: "", tradeDate: "2026-08-29", avgPrice: 44 },
			{
				marketCode: "",
				cropCode: "",
				tradeDate: "2026-08-30",
				avgPrice: 45.2,
			},
		]);
	});

	it("omits avgPrice for a point with no value rather than defaulting to 0", () => {
		const result = categoryRollupPointsToObservations([
			{ tradeDate: "2026-08-29" },
		]);

		expect(result).toEqual([
			{ marketCode: "", cropCode: "", tradeDate: "2026-08-29" },
		]);
	});
});
