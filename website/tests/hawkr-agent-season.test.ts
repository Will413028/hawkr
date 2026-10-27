import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
	HAWKR_AGENT_CATEGORY_IDS,
	HAWKR_AGENT_CATEGORY_METADATA,
} from "../lib/animation/hawkr-agent-season";

describe("Hawkr Agent category metadata", () => {
	it("keeps category metadata complete and ordered", () => {
		assert.deepEqual(HAWKR_AGENT_CATEGORY_IDS, [
			"vegetables",
			"fruits",
			"seafood",
			"meat",
			"egg-dairy",
			"processed",
			"spices",
			"grains",
			"flowers",
		]);
		assert.deepEqual(
			Object.values(HAWKR_AGENT_CATEGORY_METADATA).map(({ order }) => order),
			[1, 2, 3, 4, 5, 6, 7, 8, 9],
		);
	});
});
