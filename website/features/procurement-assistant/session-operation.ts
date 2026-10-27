export interface SessionOperationIdentity {
	readonly generation: number;
	readonly clientRequestId: string;
	readonly requestId: string | null;
}

export interface SessionOperation extends SessionOperationIdentity {
	readonly signal: AbortSignal;
}

export interface SessionOperationGuard {
	begin(input: {
		clientRequestId: string;
		requestId: string | null;
	}): SessionOperation;
	invalidate(): void;
	isCurrent(operation: SessionOperationIdentity): boolean;
	acceptsResult(
		operation: SessionOperationIdentity,
		resultRequestId: unknown,
	): boolean;
}

export interface SessionMutationIdentity {
	readonly generation: number;
	readonly requestId: string | null;
}

export interface SessionGenerationGuard {
	capture(requestId: string | null): SessionMutationIdentity;
	invalidate(): void;
	isCurrent(
		identity: SessionMutationIdentity,
		currentRequestId: string | null,
	): boolean;
}

export interface ExclusiveMutationIdentity {
	readonly key: string;
	readonly generation: number;
}

export interface ExclusiveMutationGuard {
	/** 同一個 key 已有 request 在途時回 null；不同 key 可並行。 */
	begin(key: string): ExclusiveMutationIdentity | null;
	invalidate(): void;
	isCurrent(identity: ExclusiveMutationIdentity): boolean;
	finish(identity: ExclusiveMutationIdentity): void;
}

/**
 * Serialises server-owned toggle mutations per entity without blocking siblings.
 *
 * A checkbox toggle is not a commutative write: if two requests for the same
 * candidate overlap, server processing order can differ from response order and
 * leave the browser showing a state the server no longer owns. The immutable
 * lease makes a second same-key toggle a no-op until the first settles, while
 * still allowing two different candidates to update concurrently.
 */
export function createExclusiveMutationGuard(): ExclusiveMutationGuard {
	let generation = 0;
	const active = new Map<string, ExclusiveMutationIdentity>();

	return {
		begin(key) {
			if (active.has(key)) return null;
			const identity = Object.freeze({ key, generation });
			active.set(key, identity);
			return identity;
		},
		invalidate() {
			generation += 1;
			active.clear();
		},
		isCurrent(identity) {
			return (
				identity.generation === generation &&
				active.get(identity.key) === identity
			);
		},
		finish(identity) {
			if (active.get(identity.key) === identity) active.delete(identity.key);
		},
	};
}

/**
 * Session-wide epoch for async work that is not a turn command (candidate
 * search/polling, selection, quote preview/send/withdraw and restore reads).
 * These operations may legitimately overlap, so unlike SessionOperationGuard
 * a new capture does not invalidate siblings; reset/unmount or a new
 * requirement-changing turn advances the epoch. Each mutation must additionally
 * still belong to the captured request.
 */
export function createSessionGenerationGuard(): SessionGenerationGuard {
	let generation = 0;
	return {
		capture(requestId) {
			return Object.freeze({ generation, requestId });
		},
		invalidate() {
			generation += 1;
		},
		isCurrent(identity, currentRequestId) {
			return (
				identity.generation === generation &&
				identity.requestId === currentRequestId
			);
		},
	};
}

/**
 * Correlates every async procurement command with the session generation that
 * started it. A reset or newer command aborts the previous transport and makes
 * all of its late callbacks fail the same `isCurrent` check.
 *
 * `clientRequestId` remains the backend idempotency key. Generation is a
 * browser-only execution identity: an exact retry starts a new generation but
 * deliberately keeps the same clientRequestId.
 */
export function createSessionOperationGuard(): SessionOperationGuard {
	let generation = 0;
	let current: {
		operation: SessionOperation;
		controller: AbortController;
	} | null = null;

	const invalidate = () => {
		generation += 1;
		const previous = current;
		current = null;
		// Clear the identity before aborting: abort listeners run synchronously and
		// must already see the previous operation as stale.
		previous?.controller.abort();
	};
	const isCurrent = (operation: SessionOperationIdentity) =>
		Boolean(
			current &&
				current.operation === operation &&
				!current.operation.signal.aborted &&
				current.operation.generation === operation.generation &&
				current.operation.clientRequestId === operation.clientRequestId &&
				current.operation.requestId === operation.requestId,
		);

	return {
		begin({ clientRequestId, requestId }) {
			const previous = current;
			generation += 1;
			const controller = new AbortController();
			const operation = Object.freeze({
				generation,
				clientRequestId,
				requestId,
				signal: controller.signal,
			});
			current = { operation, controller };
			// Publish the replacement identity first. If an abort listener starts an
			// even newer operation synchronously, that newer generation remains current.
			previous?.controller.abort();
			return operation;
		},
		invalidate,
		isCurrent,
		acceptsResult(operation, resultRequestId) {
			return (
				isCurrent(operation) &&
				typeof resultRequestId === "string" &&
				resultRequestId.length > 0 &&
				(operation.requestId === null ||
					operation.requestId === resultRequestId)
			);
		},
	};
}
