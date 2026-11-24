import { describe, expect, it } from "vitest";
import type { ProcurementSnapshot } from "./types";

describe("deriveSelectedCandidateIds", () => {
	it("只回傳 selected=true 的 candidateId", async () => {
		const { deriveSelectedCandidateIds } = await import("./session-logic");
		const ids = deriveSelectedCandidateIds([
			{ candidateId: "1", selected: true } as never,
			{ candidateId: "2", selected: false } as never,
			{ candidateId: "3", selected: true } as never,
		]);
		expect(ids).toEqual(["1", "3"]);
	});
});

const baseSnapshot: ProcurementSnapshot = {
	requestId: "req-1",
	revisionId: "rev-1",
	status: "CLARIFYING",
	slots: {},
	missingRequired: ["quantity"],
	runs: [],
	activeRunId: "",
	openConfirmation: null,
	publishedPublication: null,
};

/**
 * `pendingInterrupt`（`{interruptId, reason} | null`）有兩個來源：`applyTurnResult` 從 `turn_result` 的 interrupt 推導
 * （`useProcurementSession.interrupt.unit.test.ts`），以及 reload 時 `restoredStateFrom` 從 snapshot 的 `openConfirmation` 還原。
 * 這裡對 module-level 的純函式 `initialState`／`restoredStateFrom` 做直接單元測試。
 */
describe("pendingInterrupt", () => {
	it("initialState() 預設為 null（尚未有任何一輪停在 interrupt）", async () => {
		const { initialState } = await import("./useProcurementSession");
		expect(initialState().pendingInterrupt).toBeNull();
	});

	it("restoredStateFrom 從 snapshot.openConfirmation 恢復同一個 pendingInterrupt——reload 後原地復原", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, {
			...baseSnapshot,
			openConfirmation: {
				interruptId: "int-1",
				reason: "CLARIFICATION",
				question: null,
				processing: false,
			},
		});
		expect(restored.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
		const merged = { ...initialState(), ...restored };
		expect(merged.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});

	it("restoredStateFrom 在 snapshot 沒有 openConfirmation 時明確把 pendingInterrupt 歸零，不留呼叫前的殘值", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, baseSnapshot);
		expect(restored).toHaveProperty("pendingInterrupt", null);
		const merged = {
			...initialState(),
			pendingInterrupt: { interruptId: "stale", reason: "STALE" },
			...restored,
		};
		expect(merged.pendingInterrupt).toBeNull();
	});
});

describe("publishedPublication", () => {
	const base: ProcurementSnapshot = {
		...baseSnapshot,
		status: "CANDIDATES_READY",
		missingRequired: [],
	};
	const publication = {
		publicationId: "7504758025710731264",
		version: "1",
		publishedAt: "2026-09-13T04:29:04Z",
		recipientCount: 2,
		queuedCount: 2,
		sellerFacingHash: "a".repeat(64),
	};

	it("initialState() 預設為 null（這個 request 還沒發布過）", async () => {
		const { initialState } = await import("./useProcurementSession");
		expect(initialState().publishedPublication).toBeNull();
	});

	it("restoredStateFrom 從 snapshot.publishedPublication 還原發布事實——發布不改 request status，只有這個欄位證明得了", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, {
			...base,
			publishedPublication: publication,
		});
		expect(restored.publishedPublication).toEqual(publication);
		const merged = { ...initialState(), ...restored };
		expect(merged.publishedPublication).not.toBeNull();
	});

	it("restoredStateFrom 在 snapshot 沒有 publishedPublication 時把欄位明確寫成 null，不留上一個 request 的殘值", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, base);
		expect(restored).toHaveProperty("publishedPublication", null);
		const merged = {
			...initialState(),
			publishedPublication: { ...publication, publicationId: "stale" },
			...restored,
		};
		expect(merged.publishedPublication).toBeNull();
	});
});
