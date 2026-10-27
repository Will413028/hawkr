import { afterEach, describe, expect, it, vi } from "vitest";

const postMock = vi.fn();

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			authed: { ...actual.apiClient.authed, post: postMock },
		},
	};
});

afterEach(() => {
	postMock.mockReset();
});

describe("deriveSelectedCandidateIds", () => {
	it("只回傳 selected=true 的 candidateId，platform/external 都算", async () => {
		const { deriveSelectedCandidateIds } = await import("./session-logic");
		const ids = deriveSelectedCandidateIds({
			platform: [
				{ candidateId: "1", selected: true } as never,
				{ candidateId: "2", selected: false } as never,
			],
			external: [{ candidateId: "3", selected: true } as never],
		});
		expect(ids).toEqual(["1", "3"]);
	});
});

/**
 * `pendingInterrupt`——`{interruptId, reason} | null`，兩個來源共用同一個
 * 欄位：(a) `applyTurnResult` 從落地結果的 `interruptId`／`interruptKind`
 * 推導（unary SubmitTurn 與串流終態的 `RUN_FINISHED.result` 同形狀，live-
 * stream 與 sync-fallback 兩個呼叫端共用同一份推導邏輯，見 Fix 1/2）；
 * (b) reload 時 `restoredStateFrom` 從 snapshot 的 `openConfirmation` 還原
 * （Task 17，本 describe 涵蓋的部分）。
 *
 * 範圍說明：跟 `useProcurementSession.justCompleted.unit.test.ts` 記錄過的
 * 同一個已知落差——`applyTurnResult`／`submitTurn` 是 hook 內部 `useCallback`
 * closure，這裡只對 module-level 的純函式 `initialState`／`restoredStateFrom`
 * （含它私下呼叫的 `normalizeOpenConfirmation`）做直接單元測試；
 * `applyTurnResult` 推導 pendingInterrupt 的 renderHook 驗證在
 * `useProcurementSession.interrupt.unit.test.ts`，resume() 呼叫的實際串接
 * 行為在 `useProcurementSession.resume.unit.test.ts`。
 */
describe("pendingInterrupt", () => {
	it("initialState() 預設為 null（尚未有任何一輪停在 interrupt）", async () => {
		const { initialState } = await import("./useProcurementSession");
		expect(initialState().pendingInterrupt).toBeNull();
	});

	it("restoredStateFrom 從 snapshot.openConfirmation 恢復同一個 pendingInterrupt——interrupt 是後端 run ledger 的持久狀態，reload 後應該原地復原，不是只有串流當下才看得到", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, {
			requestId: "req-1",
			revisionId: "rev-1",
			status: "CLARIFYING",
			slots: {},
			missingRequired: ["quantity"],
			openConfirmation: {
				interruptId: "int-1",
				confirmationKind: "CLARIFICATION",
				expiresAt: "2026-09-01T00:00:00Z",
			},
		});
		// expiresAt 刻意不透傳（後端目前一律回 ""，跟本欄位無關的既有設計）；
		// confirmationKind 映射到既有的 reason 欄位名，不是另一個 shape。
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
		const restored = restoredStateFrom(null, {
			requestId: "req-1",
			revisionId: "rev-1",
			status: "CLARIFYING",
			slots: {},
			missingRequired: ["quantity"],
		});
		// 跟 justCompletedTurnId 那組測試刻意不同：這裡欄位本身要出現在回傳
		// 物件上（值是 null），不能只是「沒提到」——沒有 openConfirmation
		// 代表這個 interrupt 已經解掉了，是一個要覆寫舊值的明確結果，不是
		// 「這個 restore 不管這個欄位」。
		expect(restored).toHaveProperty("pendingInterrupt", null);
		const merged = {
			...initialState(),
			pendingInterrupt: { interruptId: "stale", reason: "STALE" },
			...restored,
		};
		expect(merged.pendingInterrupt).toBeNull();
	});

	it.each([
		["非物件（字串）", "int-1"],
		["null", null],
		["缺 interruptId", { confirmationKind: "CLARIFICATION" }],
		["缺 confirmationKind", { interruptId: "int-1" }],
		[
			"interruptId 型別錯誤",
			{ interruptId: 1, confirmationKind: "CLARIFICATION" },
		],
	])("restoredStateFrom 對格式錯誤的 openConfirmation（%s）防禦性回 null，不讓半吊子資料寫進 pendingInterrupt", async (_label, malformed) => {
		const { restoredStateFrom } = await import("./useProcurementSession");
		const restored = restoredStateFrom(null, {
			requestId: "req-1",
			revisionId: "rev-1",
			status: "CLARIFYING",
			slots: {},
			missingRequired: ["quantity"],
			openConfirmation: malformed,
		});
		expect(restored.pendingInterrupt).toBeNull();
	});
});

describe("publishedPublication", () => {
	const base = {
		requestId: "req-1",
		revisionId: "rev-1",
		status: "CANDIDATES_READY" as const,
		slots: {},
		missingRequired: [],
	};

	it("initialState() 預設為 null（這個 request 還沒發布過）", async () => {
		const { initialState } = await import("./useProcurementSession");
		expect(initialState().publishedPublication).toBeNull();
	});

	it("restoredStateFrom 從 snapshot.publishedPublication 還原發布事實——發布不改 request status（發布後仍是 CANDIDATES_READY），所以 status 推不出這件事，只有這個欄位證明得了", async () => {
		const { initialState, restoredStateFrom } = await import(
			"./useProcurementSession"
		);
		const restored = restoredStateFrom(null, {
			...base,
			publishedPublication: {
				publicationId: "7504758025710731264",
				version: "1",
				publishedAt: "2026-09-13T04:29:04Z",
			},
		});
		expect(restored.publishedPublication).toEqual({
			publicationId: "7504758025710731264",
			version: "1",
			publishedAt: "2026-09-13T04:29:04Z",
		});
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
			publishedPublication: {
				publicationId: "stale",
				version: "1",
				publishedAt: "",
			},
			...restored,
		};
		expect(merged.publishedPublication).toBeNull();
	});

	it.each([
		["非物件（字串）", "7504758025710731264"],
		["null", null],
		["缺 publicationId", { version: "1", publishedAt: "2026-09-13T04:29:04Z" }],
		["publicationId 型別錯誤（number）", { publicationId: 12345 }],
		["publicationId 空字串", { publicationId: "" }],
	])("restoredStateFrom 對格式錯誤的 publishedPublication（%s）回 null——寧可少一個入口，也不要畫一個點進去是空的入口", async (_label, malformed) => {
		const { restoredStateFrom } = await import("./useProcurementSession");
		const restored = restoredStateFrom(null, {
			...base,
			publishedPublication: malformed,
		});
		expect(restored.publishedPublication).toBeNull();
	});

	it("version 與 publishedAt 缺席時仍認得發布事實，只把缺的欄位補成空字串——publicationId 才是那個事實本身", async () => {
		const { restoredStateFrom } = await import("./useProcurementSession");
		const restored = restoredStateFrom(null, {
			...base,
			publishedPublication: { publicationId: "7504758025710731264" },
		});
		expect(restored.publishedPublication).toEqual({
			publicationId: "7504758025710731264",
			version: "",
			publishedAt: "",
		});
	});
});
