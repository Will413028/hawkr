"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export type SellerShipmentOperationKind = "tcat-reserve";
export type SellerShipmentOperationPhase = "submitting" | "syncing";

export interface SellerShipmentOperation {
	kind: SellerShipmentOperationKind;
	phase: SellerShipmentOperationPhase;
	updatedAt: number;
}

type SellerShipmentOperationPersistedState = {
	operations: Record<string, SellerShipmentOperation>;
};

interface SellerShipmentOperationState
	extends SellerShipmentOperationPersistedState {
	startTcatReservation: (orderNo: string) => void;
	syncTcatReservation: (orderNo: string) => void;
	clearOperation: (orderNo: string) => void;
	pruneExpired: (now?: number) => void;
}

const defaultPersistedState: SellerShipmentOperationPersistedState = {
	operations: {},
};

/** A request should normally finish far sooner than this. The longer window
 * only protects against unlocking the order while an unusually slow request
 * is still in flight (including after the drawer was closed). */
export const SHIPMENT_SUBMIT_TIMEOUT_MS = 2 * 60_000;

/** The seller order endpoints can briefly return their pre-mutation snapshot
 * after a successful shipping call. Keep the action locked while polling for
 * the authoritative next state, but eventually allow recovery if sync stalls. */
export const SHIPMENT_SYNC_TIMEOUT_MS = 90_000;

/** One delayed safety refresh covers a missed realtime event without turning
 * the seller order page into a polling loop. */
export const SHIPMENT_FALLBACK_REFRESH_DELAY_MS = 8_000;

export function shipmentOperationExpiresAt(
	operation: SellerShipmentOperation,
): number {
	const timeout =
		operation.phase === "submitting"
			? SHIPMENT_SUBMIT_TIMEOUT_MS
			: SHIPMENT_SYNC_TIMEOUT_MS;
	return operation.updatedAt + timeout;
}

export function shipmentOperationLabel(
	operation: SellerShipmentOperation,
): string {
	return operation.phase === "submitting" ? "送出預約中…" : "預約處理中…";
}

export function isShipmentOperationExpired(
	operation: SellerShipmentOperation,
	now: number = Date.now(),
): boolean {
	return now >= shipmentOperationExpiresAt(operation);
}

function withoutExpiredOperations(
	operations: Record<string, SellerShipmentOperation>,
	now: number,
): Record<string, SellerShipmentOperation> {
	const next = Object.fromEntries(
		Object.entries(operations).filter(
			([, operation]) => !isShipmentOperationExpired(operation, now),
		),
	);
	return Object.keys(next).length === Object.keys(operations).length
		? operations
		: next;
}

export const useSellerShipmentOperationStore = createLocalPrivateStore<
	SellerShipmentOperationState,
	SellerShipmentOperationPersistedState
>(
	(set) => ({
		...defaultPersistedState,
		startTcatReservation: (orderNo) =>
			set(
				(state) => ({
					operations: {
						...withoutExpiredOperations(state.operations, Date.now()),
						[orderNo]: {
							kind: "tcat-reserve",
							phase: "submitting",
							updatedAt: Date.now(),
						},
					},
				}),
				false,
				"seller-shipment/start-tcat-reservation",
			),
		syncTcatReservation: (orderNo) =>
			set(
				(state) => ({
					operations: {
						...withoutExpiredOperations(state.operations, Date.now()),
						[orderNo]: {
							kind: "tcat-reserve",
							phase: "syncing",
							updatedAt: Date.now(),
						},
					},
				}),
				false,
				"seller-shipment/sync-tcat-reservation",
			),
		clearOperation: (orderNo) =>
			set(
				(state) => {
					if (!state.operations[orderNo]) return state;
					const operations = { ...state.operations };
					delete operations[orderNo];
					return { operations };
				},
				false,
				"seller-shipment/clear-operation",
			),
		pruneExpired: (now = Date.now()) =>
			set(
				(state) => {
					const operations = withoutExpiredOperations(state.operations, now);
					return operations === state.operations ? state : { operations };
				},
				false,
				"seller-shipment/prune-expired",
			),
	}),
	{
		keyName: "sellerShipmentOperations",
		devtoolsName: "seller-shipment-operation-store",
		defaults: defaultPersistedState,
		partialize: (state): SellerShipmentOperationPersistedState => ({
			operations: state.operations,
		}),
	},
);
