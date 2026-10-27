import { describe, expect, it, vi } from "vitest";
import {
	createExclusiveMutationGuard,
	createSessionGenerationGuard,
	createSessionOperationGuard,
	type SessionOperation,
} from "./session-operation";

describe("exclusive mutation identity", () => {
	it("serializes the same entity while allowing sibling candidates", () => {
		const guard = createExclusiveMutationGuard();
		const first = guard.begin("set-a:candidate-1");
		if (!first) throw new Error("first lease missing");
		expect(guard.begin("set-a:candidate-1")).toBeNull();

		const sibling = guard.begin("set-a:candidate-2");
		if (!sibling) throw new Error("sibling lease missing");
		expect(guard.isCurrent(first)).toBe(true);
		expect(guard.isCurrent(sibling)).toBe(true);

		guard.finish(first);
		const retry = guard.begin("set-a:candidate-1");
		expect(retry).not.toBeNull();
		expect(retry).not.toBe(first);
	});

	it("cannot let a stale finish delete the replacement lease after invalidate", () => {
		const guard = createExclusiveMutationGuard();
		const oldLease = guard.begin("set-a:candidate-1");
		if (!oldLease) throw new Error("old lease missing");
		guard.invalidate();
		expect(guard.isCurrent(oldLease)).toBe(false);

		const replacement = guard.begin("set-a:candidate-1");
		if (!replacement) throw new Error("replacement lease missing");
		guard.finish(oldLease);
		expect(guard.isCurrent(replacement)).toBe(true);
	});
});

describe("session generation identity", () => {
	it("allows same-session siblings but rejects reset and request changes", () => {
		const guard = createSessionGenerationGuard();
		const poll = guard.capture("request-a");
		const quotePreview = guard.capture("request-a");

		expect(guard.isCurrent(poll, "request-a")).toBe(true);
		expect(guard.isCurrent(quotePreview, "request-a")).toBe(true);
		expect(guard.isCurrent(poll, "request-b")).toBe(false);
		expect(guard.isCurrent(poll, null)).toBe(false);

		guard.invalidate();
		expect(guard.isCurrent(poll, "request-a")).toBe(false);
		expect(guard.isCurrent(quotePreview, "request-a")).toBe(false);
		const nextSession = guard.capture("request-a");
		expect(nextSession.generation).toBeGreaterThan(poll.generation);
		expect(guard.isCurrent(nextSession, "request-a")).toBe(true);
	});
});

describe("session operation identity", () => {
	it("reset invalidates and aborts the old operation before abort listeners run", () => {
		const guard = createSessionOperationGuard();
		const oldOperation = guard.begin({
			clientRequestId: "client-old",
			requestId: "request-old",
		});
		const currentDuringAbort = vi.fn(() => guard.isCurrent(oldOperation));
		oldOperation.signal.addEventListener("abort", currentDuringAbort);

		expect(guard.isCurrent(oldOperation)).toBe(true);
		expect(() => guard.invalidate()).not.toThrow();
		expect(oldOperation.signal.aborted).toBe(true);
		expect(guard.isCurrent(oldOperation)).toBe(false);
		expect(currentDuringAbort).toHaveReturnedWith(false);
		// reset/unmount cleanup can safely repeat invalidation.
		expect(() => guard.invalidate()).not.toThrow();
	});

	it("a newer execution aborts the old one and exact retry keeps only the idempotency key", () => {
		const guard = createSessionOperationGuard();
		const oldOperation = guard.begin({
			clientRequestId: "same-client-key",
			requestId: "request-a",
		});
		const currentDuringAbort = vi.fn(() => guard.isCurrent(oldOperation));
		oldOperation.signal.addEventListener("abort", currentDuringAbort);

		const exactRetry = guard.begin({
			clientRequestId: "same-client-key",
			requestId: "request-a",
		});

		expect(oldOperation.signal.aborted).toBe(true);
		expect(currentDuringAbort).toHaveReturnedWith(false);
		expect(guard.isCurrent(oldOperation)).toBe(false);
		expect(guard.isCurrent(exactRetry)).toBe(true);
		expect(exactRetry.clientRequestId).toBe(oldOperation.clientRequestId);
		expect(exactRetry.generation).toBeGreaterThan(oldOperation.generation);
	});

	it("requires the immutable token and every captured identity field", () => {
		const guard = createSessionOperationGuard();
		const operation = guard.begin({
			clientRequestId: "client-a",
			requestId: "request-a",
		});
		const forge = (overrides: Partial<SessionOperation>): SessionOperation => ({
			...operation,
			...overrides,
		});

		expect(
			guard.isCurrent(forge({ generation: operation.generation + 1 })),
		).toBe(false);
		expect(guard.isCurrent(forge({ clientRequestId: "client-b" }))).toBe(false);
		expect(guard.isCurrent(forge({ requestId: "request-b" }))).toBe(false);
		// A value-for-value clone is not the frozen token captured by the submit.
		expect(guard.isCurrent(forge({}))).toBe(false);
		expect(Object.isFrozen(operation)).toBe(true);
	});

	it("accepts a first server request id but fails closed on mismatched existing requests", () => {
		const guard = createSessionOperationGuard();
		const firstTurn = guard.begin({
			clientRequestId: "client-first",
			requestId: null,
		});

		expect(guard.acceptsResult(firstTurn, "request-created")).toBe(true);
		expect(guard.acceptsResult(firstTurn, undefined)).toBe(false);
		expect(guard.acceptsResult(firstTurn, null)).toBe(false);
		expect(guard.acceptsResult(firstTurn, "")).toBe(false);

		const existing = guard.begin({
			clientRequestId: "client-existing",
			requestId: "request-a",
		});
		expect(guard.acceptsResult(existing, "request-a")).toBe(true);
		expect(guard.acceptsResult(existing, "request-b")).toBe(false);
		expect(guard.acceptsResult(firstTurn, "request-created")).toBe(false);
	});

	it("drops late step, result, and error mutations after reset plus a new submit", () => {
		const guard = createSessionOperationGuard();
		const oldOperation = guard.begin({
			clientRequestId: "client-old",
			requestId: "request-old",
		});
		guard.invalidate();
		const newOperation = guard.begin({
			clientRequestId: "client-new",
			requestId: null,
		});
		let state = {
			pendingClientRequestId: "client-new",
			requestId: null as string | null,
			turns: ["new buyer"],
			steps: [] as string[],
			error: null as string | null,
		};
		const mutateIfCurrent = (
			operation: SessionOperation,
			mutation: (current: typeof state) => typeof state,
		) => {
			if (guard.isCurrent(operation)) state = mutation(state);
		};

		// Simulate the old SSE step, done, and error all landing while the new
		// operation is pending. None may clear or overwrite the new session.
		mutateIfCurrent(oldOperation, (current) => ({
			...current,
			steps: [...current.steps, "old step"],
		}));
		mutateIfCurrent(oldOperation, (current) => ({
			...current,
			pendingClientRequestId: "",
			requestId: "request-old",
			turns: [...current.turns, "old reply"],
		}));
		mutateIfCurrent(oldOperation, (current) => ({
			...current,
			pendingClientRequestId: "",
			error: "old error",
		}));

		expect(state).toEqual({
			pendingClientRequestId: "client-new",
			requestId: null,
			turns: ["new buyer"],
			steps: [],
			error: null,
		});

		mutateIfCurrent(newOperation, (current) => ({
			...current,
			steps: ["new step"],
		}));
		expect(state.steps).toEqual(["new step"]);
	});
});
