import http from "node:http";
import { describe, expect, it } from "vitest";

/**
 * The same promises as tests/hermetic-guard.test.ts, inside a Vitest worker: tests/setup/hermetic.mjs is loaded there
 * through `setupFiles` (what tests/setup-msw.ts adds is in lib/testing/__tests__/msw.test.ts).
 */
describe("the test guard in a Vitest worker", () => {
	it("pins the API base to a loopback address", () => {
		expect(process.env.API_BASE_URL).toBe("http://127.0.0.1:9/api/v1");
		expect(process.env.NEXT_PUBLIC_API_BASE_URL).toBe(
			"http://127.0.0.1:9/api/v1",
		);
	});

	it("refuses node:http to a host that is not loopback", () => {
		// (WebSocket is covered by tests/hermetic-guard.test.ts: msw wraps the constructor here and
		// hands the refusal back asynchronously.)
		expect(() => http.request("http://api.hawkr.example/x")).toThrow(
			/hermetic test: refusing http\.request/,
		);
	});
});
