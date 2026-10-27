import { isLoopbackHost } from "./loopback.mjs";

/**
 * Requests no msw handler claimed (tests/setup-msw.ts). msw's own answer to an `onUnhandledRequest` that
 * throws is a synthetic 500 for the code under test, which can swallow it and let the test pass. So the
 * callback only records the request — the network guard (tests/setup/hermetic.mjs) refuses the real attempt —
 * and `assertNone()` in `afterEach` fails the test that made it.
 */
export const unhandledRequests = {
	seen: [] as string[],

	/** msw's `onUnhandledRequest`. Loopback requests are left to go where they point. */
	record(request: Request): void {
		if (isLoopbackHost(new URL(request.url).hostname)) return;
		this.seen.push(`${request.method} ${request.url}`);
	},

	/** Take the recorded requests (and forget them). */
	drain(): string[] {
		return this.seen.splice(0);
	},

	assertNone(): void {
		const requests = this.drain();
		if (requests.length === 0) return;
		throw new Error(
			`This test made request(s) no handler or stub answers; add one (tests/mocks/server.ts) or use a loopback address:\n${requests.join("\n")}`,
		);
	},
};
