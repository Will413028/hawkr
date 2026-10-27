import assert from "node:assert/strict";
import { test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { withApiBase, withoutApiBasePins } from "./helpers/default-api-base";

// dev/prod connection parity: both env hit the gateway directly (no dev-only
// same-origin proxy). The browser path is apiBaseUrl in every flavor; if anyone
// reintroduces a dev proxy divergence, this fails.
for (const flavor of ["dev", "prod"] as const) {
	test(`${flavor} resolves the gateway directly`, () => {
		// dev's default gateway is under test, so build its env without the loopback pins that
		// tests/setup/hermetic.mjs puts in the API base variables (nothing here makes a request).
		// prod has no default: its gateway is the origin of the configured API base.
		const restore =
			flavor === "dev"
				? withoutApiBasePins()
				: withApiBase("https://api.example.com/api/v1");
		try {
			__setEnvForTest(null);
			__setFlavorForTest(flavor);
			const env = getEnv();
			assert.equal(env.apiBaseUrl, `${env.gatewayHost}/api/v1`);
			assert.equal(env.serverApiBase, `${env.gatewayHost}/api/v1`);
		} finally {
			restore();
			__setEnvForTest(null);
			__setFlavorForTest(null);
		}
	});
}

test("prod without an API base fails with a message naming the variables", () => {
	const restore = withoutApiBasePins();
	try {
		__setEnvForTest(null);
		__setFlavorForTest("prod");
		assert.throws(
			() => getEnv(),
			/API base is not configured for the production flavor.*API_BASE_URL.*NEXT_PUBLIC_API_BASE_URL/,
		);
	} finally {
		restore();
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
});

test("prod needs an absolute server-side API base even when the public one is relative", () => {
	const restore = withoutApiBasePins();
	const previous = process.env.NEXT_PUBLIC_API_BASE_URL;
	try {
		process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";
		__setEnvForTest(null);
		__setFlavorForTest("prod");
		assert.throws(() => getEnv(), /API_BASE_URL/);
	} finally {
		if (previous === undefined) delete process.env.NEXT_PUBLIC_API_BASE_URL;
		else process.env.NEXT_PUBLIC_API_BASE_URL = previous;
		restore();
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
});

test("the browser bundle of a prod build may use a same-origin API base", () => {
	const restore = withoutApiBasePins();
	const previous = process.env.NEXT_PUBLIC_API_BASE_URL;
	const globals = globalThis as { window?: unknown };
	try {
		process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";
		globals.window = {};
		__setEnvForTest(null);
		__setFlavorForTest("prod");
		assert.equal(getEnv().apiBaseUrl, "/api/v1");
	} finally {
		delete globals.window;
		if (previous === undefined) delete process.env.NEXT_PUBLIC_API_BASE_URL;
		else process.env.NEXT_PUBLIC_API_BASE_URL = previous;
		restore();
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
});

test("prod with only API_BASE_URL set fails: the browser-inlined NEXT_PUBLIC_API_BASE_URL is required too", () => {
	const restore = withoutApiBasePins();
	try {
		process.env.API_BASE_URL = "https://api.example.com/api/v1";
		__setEnvForTest(null);
		__setFlavorForTest("prod");
		assert.throws(() => getEnv(), /NEXT_PUBLIC_API_BASE_URL/);
	} finally {
		restore();
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
});
