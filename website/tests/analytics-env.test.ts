import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

beforeEach(() => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
});

afterEach(() => {
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID;
});

test("gaId reads NEXT_PUBLIC_GA_MEASUREMENT_ID", () => {
	process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID = "G-TEST123";
	assert.equal(getEnv().gaId, "G-TEST123");
});

test("gaId defaults to empty string when unset", () => {
	delete process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID;
	__setEnvForTest(null);
	assert.equal(getEnv().gaId, "");
});
