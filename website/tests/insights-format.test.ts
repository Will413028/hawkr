import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
	formatPrice,
	formatVolume,
} from "../features/seller-center/insights/insightsFormat";

describe("insights formatters", () => {
	it("keeps one meaningful decimal instead of rounding price evidence away", () => {
		assert.equal(formatPrice(63.2), "NT$ 63.2");
		assert.equal(formatPrice(1200), "NT$ 1,200");
	});

	it("keeps missing values distinct from explicit zero", () => {
		assert.equal(formatPrice(undefined), "—");
		assert.equal(formatVolume(undefined), "—");
		assert.equal(formatVolume(0), "0 kg");
	});

	it("renders exact observed volume with unit", () => {
		assert.equal(formatVolume(50700), "50,700 kg");
	});
});
