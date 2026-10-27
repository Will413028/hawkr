import { beforeEach, describe, expect, it, vi } from "vitest";

// `unstable_cache` is a pass-through here on purpose: what is under test is the
// layer ABOVE it — how many upstream reads one key opens when several requests
// want it at the same moment. The cache entry's own behaviour (tags, revalidate)
// is Next's, and mocking it out is what makes the coalescing observable at all.
vi.mock("next/cache", () => ({
	unstable_cache: <T>(fn: T) => fn,
}));
vi.mock("@sentry/nextjs", () => ({
	startSpan: <T>(_options: unknown, fn: () => T) => fn(),
}));

const { cachedRead, coalesceRead, __resetInFlightReadsForTest } = await import(
	"./cached"
);

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

describe("cross-request in-flight coalescing (backend#2264)", () => {
	beforeEach(() => {
		__resetInFlightReadsForTest();
	});

	it("collapses concurrent reads of one key into a single upstream call", async () => {
		const gate = deferred<string>();
		const upstream = vi.fn(() => gate.promise);

		const waiters = [
			cachedRead(upstream, ["product", "1"], { tags: [], revalidate: 1 }),
			cachedRead(upstream, ["product", "1"], { tags: [], revalidate: 1 }),
			cachedRead(upstream, ["product", "1"], { tags: [], revalidate: 1 }),
		];
		gate.resolve("payload");

		expect(await Promise.all(waiters)).toEqual([
			"payload",
			"payload",
			"payload",
		]);
		expect(upstream).toHaveBeenCalledTimes(1);
	});

	it("keeps different keys on different flights", async () => {
		const upstream = vi.fn(async (id: string) => id);
		await Promise.all([
			cachedRead(() => upstream("1"), ["product", "1"], {
				tags: [],
				revalidate: 1,
			}),
			cachedRead(() => upstream("2"), ["product", "2"], {
				tags: [],
				revalidate: 1,
			}),
		]);
		expect(upstream).toHaveBeenCalledTimes(2);
	});

	// The discriminating case: key parts are joined, so two DIFFERENT lists must
	// not be able to produce the same joined string and share one answer.
	it.each([
		[
			["seller", "1:2"],
			["seller:1", "2"],
		],
		[
			["seller", "1 2"],
			["seller 1", "2"],
		],
	])("does not let %j and %j collide into one flight", async (left, right) => {
		const upstream = vi.fn(async (label: string) => label);
		const [a, b] = await Promise.all([
			cachedRead(() => upstream("a"), left, { tags: [], revalidate: 1 }),
			cachedRead(() => upstream("b"), right, { tags: [], revalidate: 1 }),
		]);
		expect([a, b]).toEqual(["a", "b"]);
		expect(upstream).toHaveBeenCalledTimes(2);
	});

	it("releases the key once settled so a later read is not served the old flight", async () => {
		let answer = "first";
		const upstream = vi.fn(async () => answer);

		expect(await cachedRead(upstream, ["k"], { tags: [], revalidate: 1 })).toBe(
			"first",
		);
		answer = "second";
		expect(await cachedRead(upstream, ["k"], { tags: [], revalidate: 1 })).toBe(
			"second",
		);
		expect(upstream).toHaveBeenCalledTimes(2);
	});

	// A joiner must not inherit the originator's failure: on workerd the
	// originator's request context can be torn down while its fetch is pending
	// (a raced deadline), which is not a statement about the upstream.
	it("lets a joiner retry on its own when the flight it joined fails", async () => {
		const gate = deferred<string>();
		let call = 0;
		const upstream = vi.fn(() => {
			call += 1;
			return call === 1 ? gate.promise : Promise.resolve("own read");
		});

		const originator = cachedRead(upstream, ["k"], { tags: [], revalidate: 1 });
		const joiner = cachedRead(upstream, ["k"], { tags: [], revalidate: 1 });
		gate.reject(new Error("context torn down"));

		await expect(originator).rejects.toThrow("context torn down");
		await expect(joiner).resolves.toBe("own read");
		expect(upstream).toHaveBeenCalledTimes(2);
	});

	it("coalesceRead exposes the same behaviour for callers owning their cache scope", async () => {
		const gate = deferred<string>();
		const upstream = vi.fn(() => gate.promise);
		const waiters = [
			coalesceRead(["home-live-bootstrap"], upstream),
			coalesceRead(["home-live-bootstrap"], upstream),
		];
		gate.resolve("home");
		expect(await Promise.all(waiters)).toEqual(["home", "home"]);
		expect(upstream).toHaveBeenCalledTimes(1);
	});
});
