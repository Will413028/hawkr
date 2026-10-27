import { describe, expect, it } from "vitest";
import { unhandledRequests } from "@/tests/setup/unhandled-requests";

describe("MSW test setup", () => {
	it("serves shared handlers in Vitest", async () => {
		const response = await fetch("https://hawkr.test/health");

		await expect(response.json()).resolves.toEqual({ ok: true });
	});

	it("refuses an unhandled external request and records it, so the test that made it fails", async () => {
		// Not a synthetic 500 the code under test could swallow: the request is refused by the network
		// guard (tests/setup/hermetic.mjs), and tests/setup-msw.ts fails the test in afterEach.
		await expect(fetch("https://hawkr.test/unhandled")).rejects.toThrow(
			/hermetic test|fetch failed/,
		);
		// what afterEach would have thrown for this test; taken here so this test can pass
		expect(() => unhandledRequests.assertNone()).toThrow(
			/GET https:\/\/hawkr\.test\/unhandled/,
		);
		expect(unhandledRequests.seen).toEqual([]);
	});

	it("does not record a loopback request: it goes where it points", async () => {
		await fetch("http://127.0.0.1:9/never-listening").catch(() => undefined);
		expect(unhandledRequests.drain()).toEqual([]);
	});
});
