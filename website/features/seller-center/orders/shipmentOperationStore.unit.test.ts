import { beforeEach, describe, expect, it } from "vitest";
import {
	isShipmentOperationExpired,
	SHIPMENT_SUBMIT_TIMEOUT_MS,
	SHIPMENT_SYNC_TIMEOUT_MS,
	shipmentOperationLabel,
	useSellerShipmentOperationStore,
} from "./shipmentOperationStore";

describe("seller shipment operation state", () => {
	beforeEach(() => {
		useSellerShipmentOperationStore.setState({ operations: {} });
	});

	it("keeps a reservation locked while it moves from submit to backend sync", () => {
		const state = useSellerShipmentOperationStore.getState();

		state.startTcatReservation("ORDER-1");
		expect(
			useSellerShipmentOperationStore.getState().operations["ORDER-1"]?.phase,
		).toBe("submitting");

		useSellerShipmentOperationStore.getState().syncTcatReservation("ORDER-1");
		expect(
			useSellerShipmentOperationStore.getState().operations["ORDER-1"]?.phase,
		).toBe("syncing");

		useSellerShipmentOperationStore.getState().clearOperation("ORDER-1");
		expect(
			useSellerShipmentOperationStore.getState().operations["ORDER-1"],
		).toBeUndefined();
	});

	it("uses a longer safety window for an in-flight request than read-model sync", () => {
		const now = 1_000_000;

		expect(
			isShipmentOperationExpired(
				{
					kind: "tcat-reserve",
					phase: "submitting",
					updatedAt: now - SHIPMENT_SUBMIT_TIMEOUT_MS + 1,
				},
				now,
			),
		).toBe(false);
		expect(
			isShipmentOperationExpired(
				{
					kind: "tcat-reserve",
					phase: "submitting",
					updatedAt: now - SHIPMENT_SUBMIT_TIMEOUT_MS,
				},
				now,
			),
		).toBe(true);
		expect(
			isShipmentOperationExpired(
				{
					kind: "tcat-reserve",
					phase: "syncing",
					updatedAt: now - SHIPMENT_SYNC_TIMEOUT_MS,
				},
				now,
			),
		).toBe(true);
	});

	it("does not describe an unacknowledged request as backend processing", () => {
		expect(
			shipmentOperationLabel({
				kind: "tcat-reserve",
				phase: "submitting",
				updatedAt: 1,
			}),
		).toBe("送出預約中…");
		expect(
			shipmentOperationLabel({
				kind: "tcat-reserve",
				phase: "syncing",
				updatedAt: 1,
			}),
		).toBe("預約處理中…");
	});
});
