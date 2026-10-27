import { afterEach, describe, expect, it, vi } from "vitest";
import { SESSION_UNAVAILABLE, type SessionResult } from "../../api/auth";
import { resolveSession, SESSION_RETRY_BACKOFF_MS } from "../session-resolver";

afterEach(() => {
	vi.useRealTimers();
});

const identity = {
	accountId: "1",
	isSeller: false,
};

function callbacks() {
	return {
		onResolved: vi.fn(),
		onUnavailable: vi.fn(),
		onGaveUp: vi.fn(),
	};
}

describe("resolveSession", () => {
	// Pin the literal schedule: the ~10.5s total backoff window is a product
	// decision, and the timing tests below derive their expectations from this
	// constant, so they'd stay green if the values drifted. This is the guard.
	it("keeps the bounded-backoff schedule at [1500, 3000, 6000]", () => {
		expect(SESSION_RETRY_BACKOFF_MS).toEqual([1500, 3000, 6000]);
	});

	it("resolves the account the monolith returns", async () => {
		vi.useFakeTimers();
		const cb = callbacks();
		resolveSession(() => Promise.resolve(identity), cb);

		await vi.advanceTimersByTimeAsync(0);
		expect(cb.onResolved).toHaveBeenCalledWith(identity);
		expect(cb.onUnavailable).not.toHaveBeenCalled();
		expect(cb.onGaveUp).not.toHaveBeenCalled();
	});

	it("a refused session is a definitive guest", async () => {
		vi.useFakeTimers();
		const cb = callbacks();
		resolveSession(() => Promise.resolve(null), cb);

		await vi.advanceTimersByTimeAsync(0);
		expect(cb.onResolved).toHaveBeenCalledWith(null);
		expect(cb.onGaveUp).not.toHaveBeenCalled();
	});

	it("applies a slow answer whenever it lands", async () => {
		vi.useFakeTimers();
		let settle!: (value: SessionResult) => void;
		const cb = callbacks();
		resolveSession(
			() =>
				new Promise<SessionResult>((resolve) => {
					settle = resolve;
				}),
			cb,
		);

		await vi.advanceTimersByTimeAsync(30_000);
		expect(cb.onResolved).not.toHaveBeenCalled();

		settle(identity);
		await vi.advanceTimersByTimeAsync(0);
		expect(cb.onResolved).toHaveBeenCalledWith(identity);
	});

	it("retries SESSION_UNAVAILABLE on a bounded backoff, then gives up without calling it a guest", async () => {
		vi.useFakeTimers();
		const fetchSession = vi.fn(() => Promise.resolve(SESSION_UNAVAILABLE));
		const cb = callbacks();

		resolveSession(fetchSession, cb);

		// initial probe
		await vi.advanceTimersByTimeAsync(0);
		expect(fetchSession).toHaveBeenCalledTimes(1);
		expect(cb.onUnavailable).toHaveBeenCalledTimes(1);

		// three backoff retries at the scheduled delays
		for (let i = 0; i < SESSION_RETRY_BACKOFF_MS.length; i++) {
			await vi.advanceTimersByTimeAsync(SESSION_RETRY_BACKOFF_MS[i]);
			expect(fetchSession).toHaveBeenCalledTimes(i + 2);
		}
		expect(cb.onUnavailable).toHaveBeenCalledTimes(
			SESSION_RETRY_BACKOFF_MS.length + 1,
		);
		// Exhausted: stop the skeleton, but an outage is not an answer about the
		// session — nothing may end it on this.
		expect(cb.onGaveUp).toHaveBeenCalledTimes(1);
		expect(cb.onResolved).not.toHaveBeenCalled();

		// no further probes after exhaustion
		await vi.advanceTimersByTimeAsync(60_000);
		expect(fetchSession).toHaveBeenCalledTimes(
			SESSION_RETRY_BACKOFF_MS.length + 1,
		);
	});

	it("stops retrying the moment a probe recovers with an identity", async () => {
		vi.useFakeTimers();
		const fetchSession = vi
			.fn<() => Promise<SessionResult>>()
			.mockResolvedValueOnce(SESSION_UNAVAILABLE)
			.mockResolvedValue(identity);
		const cb = callbacks();

		resolveSession(fetchSession, cb);

		await vi.advanceTimersByTimeAsync(0);
		await vi.advanceTimersByTimeAsync(SESSION_RETRY_BACKOFF_MS[0]);
		expect(cb.onResolved).toHaveBeenCalledWith(identity);

		// recovered → no more retries fire
		await vi.advanceTimersByTimeAsync(60_000);
		expect(fetchSession).toHaveBeenCalledTimes(2);
	});

	it("cancel() drops a late answer", async () => {
		vi.useFakeTimers();
		let settle!: (value: SessionResult) => void;
		const cb = callbacks();
		const cancel = resolveSession(
			() =>
				new Promise<SessionResult>((resolve) => {
					settle = resolve;
				}),
			cb,
		);
		cancel();
		settle(identity);
		await vi.advanceTimersByTimeAsync(0);
		expect(cb.onResolved).not.toHaveBeenCalled();
	});

	it("cancel() stops a pending retry after SESSION_UNAVAILABLE", async () => {
		vi.useFakeTimers();
		const fetchSession = vi.fn(() => Promise.resolve(SESSION_UNAVAILABLE));

		const cancel = resolveSession(fetchSession, callbacks());

		await vi.advanceTimersByTimeAsync(0);
		expect(fetchSession).toHaveBeenCalledTimes(1);
		cancel();

		await vi.advanceTimersByTimeAsync(SESSION_RETRY_BACKOFF_MS[0] + 1000);
		expect(fetchSession).toHaveBeenCalledTimes(1);
	});
});
