// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { useProcurementSession } from "./useProcurementSession";

/**
 * Task — interrupt/resume UI 串接的五個修復點，本檔涵蓋跟 `useProcurementSession`
 * 這個 hook 的 `useCallback` closure 有關的四個（Fix 1 的 applyTurnResult 部分、
 * Fix 2、Fix 3、Fix 4）；Fix 1 的 `normalizeProcurementTurnResult` 部分是純函式，
 * 已在 `tests/procurement-assistant-session-logic.test.ts` 覆蓋，Fix 5 的
 * `AssistantWorkspace.tsx` wiring 是另一個檔案（render-based），不在這裡測。
 *
 * 沿用 `useProcurementSession.justCompleted.unit.test.ts`／
 * `useProcurementSession.resume.unit.test.ts` 已建立的 renderHook 慣例：
 * mock `@/lib/api/client`（post／get）、`./turn-stream`（streamProcurementTurn）、
 * `@/lib/auth/context`。
 */

const { postMock, getMock, streamProcurementTurnMock, toastErrorMock } =
	vi.hoisted(() => ({
		postMock: vi.fn(),
		getMock: vi.fn(),
		streamProcurementTurnMock: vi.fn(),
		toastErrorMock: vi.fn(),
	}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "buyer",
		identity: { accountId: "1001" },
	}),
}));

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			authed: { ...actual.apiClient.authed, post: postMock, get: getMock },
		},
	};
});

vi.mock("./turn-stream", async (importOriginal) => {
	const actual = await importOriginal<typeof import("./turn-stream")>();
	return { ...actual, streamProcurementTurn: streamProcurementTurnMock };
});

vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: toastErrorMock, success: vi.fn() },
}));

afterEach(() => {
	postMock.mockReset();
	getMock.mockReset();
	streamProcurementTurnMock.mockReset();
	toastErrorMock.mockReset();
});

function renderProcurementSession(initialRequestId: string | null = null) {
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	// `.unit.test.ts`（vitest.config.ts 的 include 只認這個副檔名）不能用
	// JSX，跟既有檔案一樣改用 createElement。
	function wrapper({ children }: { children: ReactNode }) {
		return createElement(
			QueryClientProvider,
			{ client: queryClient },
			children,
		);
	}
	return renderHook(() => useProcurementSession(initialRequestId, null), {
		wrapper,
	});
}

const SNAPSHOT_WITH_OPEN_CONFIRMATION = {
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
};

const SNAPSHOT_WITHOUT_OPEN_CONFIRMATION = {
	requestId: "req-1",
	revisionId: "rev-2",
	status: "REQUIREMENT_READY",
	slots: {},
	missingRequired: [],
};

describe("Fix 1（sync-fallback 路徑）：applyTurnResult 從 interruptId／interruptKind 推導 pendingInterrupt", () => {
	it("串流不可用、降級打同步 /turns 端點，回應帶 interruptId／interruptKind 時寫入 pendingInterrupt，不再被無條件 null 吞掉", async () => {
		const { TurnStreamUnavailableError } = await import("./turn-stream");
		streamProcurementTurnMock.mockRejectedValueOnce(
			new TurnStreamUnavailableError("NOT_DEPLOYED", "no stream endpoint"),
		);
		postMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				reply: "請選擇品項。",
				degraded: false,
				slots: {},
				missingRequired: [],
				rejected: [],
				interruptId: "int-1",
				interruptKind: "CROP_DISAMBIGUATION",
			},
		});
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

		const { result } = renderProcurementSession();
		expect(result.current.pendingInterrupt).toBeNull();

		await act(async () => {
			await result.current.submitTurn("我要訂水果", "client-req-1");
		});

		await waitFor(() => {
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CROP_DISAMBIGUATION",
			});
		});
		// 降級路徑真的走了同步端點，不是串流成功。
		expect(postMock).toHaveBeenCalledTimes(1);
	});

	it("同步 /turns 回應沒有 interruptId 時 pendingInterrupt 維持 null（正常落地，不誤判成 interrupt）", async () => {
		const { TurnStreamUnavailableError } = await import("./turn-stream");
		streamProcurementTurnMock.mockRejectedValueOnce(
			new TurnStreamUnavailableError("NOT_DEPLOYED", "no stream endpoint"),
		);
		postMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				reply: "好的，記下了。",
				degraded: false,
				slots: {},
				missingRequired: ["quantity"],
				rejected: [],
			},
		});
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				status: "CLARIFYING",
				slots: {},
				missingRequired: ["quantity"],
			},
		});

		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("我要訂水果", "client-req-1");
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.pendingInterrupt).toBeNull();
	});
});

describe("Fix 2（live-stream 路徑）：interrupt outcome 不再整份丟棄，走跟成功一樣的 applyTurnResult", () => {
	it("RUN_FINISHED 帶 interruptId／interruptKind 時，cropDisambiguation／turns／pendingInterrupt 一起更新，不退化打同步端點", async () => {
		streamProcurementTurnMock.mockResolvedValueOnce({
			requestId: "req-1",
			revisionId: "rev-1",
			reply: "請選擇品項。",
			degraded: false,
			slots: {},
			missingRequired: [],
			rejected: [],
			interruptId: "int-1",
			interruptKind: "CROP_DISAMBIGUATION",
			cropDisambiguation: {
				prompt: "請選擇品項",
				options: [{ optionId: "opt-a", label: "紅蘿蔔（進口）" }],
			},
		});
		getMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-1",
				status: "CLARIFYING",
				slots: {},
				missingRequired: [],
			},
		});

		const { result } = renderProcurementSession();

		await act(async () => {
			await result.current.submitTurn("紅蘿蔔 50 公斤", "client-req-1");
		});

		await waitFor(() => {
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CROP_DISAMBIGUATION",
			});
		});
		expect(result.current.cropDisambiguation).toEqual({
			prompt: "請選擇品項",
			options: [
				{
					optionId: "opt-a",
					label: "紅蘿蔔（進口）",
					cropCode: null,
					groupCode: null,
				},
			],
		});
		expect(
			result.current.turns.find((turn) => turn.id === "client-req-1-assistant"),
		).toMatchObject({ role: "assistant", text: "請選擇品項。" });
		expect(result.current.pendingTurn).toBe(false);
		// 串流已經拿到完整結果，不該再退化打同步 /turns。
		expect(postMock).not.toHaveBeenCalled();
	});
});

describe("Fix 3：submitTurn 的兩個 409 handler 對 reason 敏感", () => {
	describe("同步 fallback 路徑（ApiClientError）", () => {
		it("reason=APPROVAL_PENDING：改用後端訊息，不顯示 IDEMPOTENCY 誤導文案，並呼叫 refreshCurrentSnapshot 自我校正", async () => {
			const { TurnStreamUnavailableError } = await import("./turn-stream");
			getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
			const { result } = renderProcurementSession("req-1");
			await waitFor(() =>
				expect(result.current.pendingInterrupt).toEqual({
					interruptId: "int-1",
					reason: "CLARIFICATION",
				}),
			);

			streamProcurementTurnMock.mockRejectedValueOnce(
				new TurnStreamUnavailableError("NOT_DEPLOYED", "no stream endpoint"),
			);
			postMock.mockRejectedValueOnce(
				new ApiClientError(
					"request is parked on an open interrupt",
					409,
					undefined,
					undefined,
					"APPROVAL_PENDING",
				),
			);
			// refreshCurrentSnapshot 校正用的第二次 GET。
			getMock.mockResolvedValueOnce({
				data: SNAPSHOT_WITH_OPEN_CONFIRMATION,
			});

			await act(async () => {
				await result.current.submitTurn("再幫我確認一次", "client-req-2");
			});

			expect(result.current.error).toEqual({
				code: "APPROVAL_PENDING",
				detail: "request is parked on an open interrupt",
			});
			expect(getMock).toHaveBeenCalledTimes(2);
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			});
		});

		it("其他 409 reason（含空字串）維持既有 TURN_IDEMPOTENCY_CONFLICT 行為，不觸發 refreshCurrentSnapshot", async () => {
			const { TurnStreamUnavailableError } = await import("./turn-stream");
			getMock.mockResolvedValueOnce({
				data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
			});
			const { result } = renderProcurementSession("req-1");
			await waitFor(() => expect(result.current.restoring).toBe(false));

			streamProcurementTurnMock.mockRejectedValueOnce(
				new TurnStreamUnavailableError("NOT_DEPLOYED", "no stream endpoint"),
			);
			postMock.mockRejectedValueOnce(
				new ApiClientError("conflict", 409, undefined, undefined, ""),
			);

			await act(async () => {
				await result.current.submitTurn("重送同一句", "client-req-2");
			});

			expect(result.current.error).toEqual({
				code: "IDEMPOTENCY_CONFLICT",
				detail:
					"這則訊息的識別碼已綁定其他內容，系統沒有建立新的操作。請修改文字後再送出。",
			});
			// 只有 mount 當下那一次 GET，沒有額外的 refreshCurrentSnapshot 校正。
			expect(getMock).toHaveBeenCalledTimes(1);
		});
	});

	describe("即時串流路徑（TurnStreamEventError）", () => {
		it("reason=APPROVAL_PENDING：改用後端訊息並呼叫 refreshCurrentSnapshot", async () => {
			const { TurnStreamEventError } = await import("./turn-stream");
			getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
			const { result } = renderProcurementSession("req-1");
			await waitFor(() =>
				expect(result.current.pendingInterrupt).toEqual({
					interruptId: "int-1",
					reason: "CLARIFICATION",
				}),
			);

			streamProcurementTurnMock.mockRejectedValueOnce(
				new TurnStreamEventError(
					409,
					"request is parked on an open interrupt",
					"APPROVAL_PENDING",
				),
			);
			getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

			await act(async () => {
				await result.current.submitTurn("再幫我確認一次", "client-req-2");
			});

			expect(result.current.error).toEqual({
				code: "APPROVAL_PENDING",
				detail: "request is parked on an open interrupt",
			});
			expect(getMock).toHaveBeenCalledTimes(2);
		});

		it("沒有 reason（既有 RUN_ERROR／HTTP 409 皆不帶業務 reason 時）維持既有 TURN_IDEMPOTENCY_CONFLICT 行為", async () => {
			const { TurnStreamEventError } = await import("./turn-stream");
			getMock.mockResolvedValueOnce({
				data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
			});
			const { result } = renderProcurementSession("req-1");
			await waitFor(() => expect(result.current.restoring).toBe(false));

			streamProcurementTurnMock.mockRejectedValueOnce(
				new TurnStreamEventError(409, "這則訊息的識別碼已綁定其他內容"),
			);

			await act(async () => {
				await result.current.submitTurn("重送同一句", "client-req-2");
			});

			expect(result.current.error).toEqual({
				code: "IDEMPOTENCY_CONFLICT",
				detail:
					"這則訊息的識別碼已綁定其他內容，系統沒有建立新的操作。請修改文字後再送出。",
			});
			expect(getMock).toHaveBeenCalledTimes(1);
		});
	});
});

describe("Fix 4：chooseCropDisambiguation 不再樂觀清空 pendingInterrupt", () => {
	it("送出 /crop-disambiguation 期間 pendingInterrupt 維持既有值，不在請求送達前就消失", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			}),
		);

		let resolvePost: (value: unknown) => void = () => {};
		postMock.mockImplementationOnce(
			() =>
				new Promise((resolve) => {
					resolvePost = resolve;
				}),
		);

		act(() => {
			void result.current.chooseCropDisambiguation({
				optionId: "opt-a",
				label: "紅蘿蔔（進口）",
				cropCode: "CARROT_IMPORT",
				groupCode: null,
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(true));
		// 樂觀 setState 已經送出，但這通請求本身不解掉 interrupt——維持原值。
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});

		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});
		await act(async () => {
			resolvePost({
				data: {
					requestId: "req-1",
					revisionId: "rev-2",
					reply: "好的，已記下紅蘿蔔。",
					degraded: false,
					slots: {},
					missingRequired: [],
					rejected: [],
				},
			});
			await Promise.resolve();
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		// applyTurnResult 落地後這一輪不是 interrupt，才由它自然清空。
		expect(result.current.pendingInterrupt).toBeNull();
	});

	it("送出後端 409 APPROVAL_PENDING 失敗：呼叫 refreshCurrentSnapshot 校正，而不是留著一個誤導的舊值", async () => {
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });
		const { result } = renderProcurementSession("req-1");
		await waitFor(() =>
			expect(result.current.pendingInterrupt).toEqual({
				interruptId: "int-1",
				reason: "CLARIFICATION",
			}),
		);

		postMock.mockRejectedValueOnce(
			new ApiClientError(
				"request is parked on an open interrupt",
				409,
				undefined,
				undefined,
				"APPROVAL_PENDING",
			),
		);
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

		await act(async () => {
			await result.current.chooseCropDisambiguation({
				optionId: "opt-a",
				label: "紅蘿蔔（進口）",
				cropCode: "CARROT_IMPORT",
				groupCode: null,
			});
		});

		expect(getMock).toHaveBeenCalledTimes(2);
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});
});

/**
 * 2026-09-03 dev 實測補上的路由缺口：品項消歧問句（`question.slotKey ===
 * "crop_disambiguation"`，dialogueActCode ITEM_NEEDS_DISAMBIGUATION）**必然**
 * 伴隨一個 open interrupt——串流終態 `outcome.interrupts[0]` 一定有值，對
 * `/answers` 送答案一定被後端 GuardNoOpenInterrupt 擋成 409 APPROVAL_PENDING。
 *
 * 問題卡的五個答題入口（選項／自由輸入／crop-other 子表單／略過／選單位）
 * 全部共用 submitAnswer，所以路由判斷放在 submitAnswer，不在 AssistantWorkspace
 * 的個別 handler 上做特判。
 */

const CROP_QUESTION = {
	slotKey: "crop_disambiguation",
	prompt: "找到幾個相符的品項，麻煩選一下你要的是哪一個：",
	allowFreeText: false,
	canSkip: false,
	options: [
		{
			optionId: "group-moa-group:f102791d",
			label: "甘藍（甘藍）",
			value: "甘藍（甘藍）",
		},
		{ optionId: "crop-other", label: "其他（自己輸入）", value: "OTHER" },
	],
};

/** 送出一輪並落在「品項消歧待選」的 interrupt 上，回傳已就緒的 hook result。 */
async function arriveAtOpenCropInterrupt(question = CROP_QUESTION) {
	streamProcurementTurnMock.mockResolvedValueOnce({
		requestId: "req-1",
		revisionId: "rev-1",
		reply: CROP_QUESTION.prompt,
		degraded: false,
		slots: { item: "甘藍" },
		missingRequired: ["delivery_city", "unit"],
		rejected: [],
		question,
		// turn-stream.ts 從 RUN_FINISHED.outcome 推導出來的扁平形狀；production
		// 的 result 本身沒有這兩個欄位（dev 實測 result.interruptId === ""）。
		interruptId: "int-1",
		interruptKind: "CLARIFICATION",
	});
	getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

	const { result } = renderProcurementSession();
	await act(async () => {
		await result.current.submitTurn("甘藍", "client-req-1");
	});
	await waitFor(() => {
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});
	return result;
}

describe("普通選填問句的 snapshot 重試", () => {
	const packagingQuestion = {
		slotKey: "packaging",
		prompt: "這批芭樂需要分裝嗎？",
		options: [{ optionId: "pack-box", label: "分箱包裝", value: "分箱包裝" }],
		allowFreeText: true,
		canSkip: true,
		source: "FIXED",
	};
	const sameRevisionSnapshot = {
		...SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		revisionId: "rev-1",
		slots: {
			item: "珍珠芭樂",
			quantity: 80,
			unit: "KG",
			delivery_city: "桃園市",
		},
	};
	const legacyCrop = {
		prompt: "請重新選擇品項",
		options: [{ optionId: "legacy-crop", label: "珍珠芭樂", cropCode: "P1" }],
	};
	async function arriveAtOptionalQuestion(question = packagingQuestion) {
		streamProcurementTurnMock.mockResolvedValueOnce({
			...sameRevisionSnapshot,
			reply: "已把數量更新為 80 KG。",
			degraded: false,
			rejected: [],
			question,
		});
		// Turn 已成功，但後續 status 校正失敗；重試只應校正缺少的狀態。
		getMock.mockRejectedValueOnce(new Error("snapshot unavailable"));
		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("數量改成 80 KG", "optional-turn");
		});
		expect(result.current.question).toMatchObject(question);
		expect(result.current.pendingInterrupt).toBeNull();
		return result;
	}

	it("相同 request/revision 的普通選填問題保留，仍可經 /answers 作答", async () => {
		const result = await arriveAtOptionalQuestion();
		getMock.mockResolvedValueOnce({ data: sameRevisionSnapshot });
		await act(async () => {
			expect(await result.current.refreshCurrentSnapshot()).toBe(true);
		});
		expect(result.current.stage).toBe("ready");
		expect(result.current.question).toMatchObject(packagingQuestion);
		expect(result.current.pendingInterrupt).toBeNull();
		expect(result.current.cropDisambiguation).toBeNull();

		postMock.mockResolvedValueOnce({
			data: {
				...sameRevisionSnapshot,
				revisionId: "rev-2",
				reply: "已記下分箱包裝。",
				degraded: false,
				rejected: [],
				question: null,
			},
		});
		getMock.mockResolvedValueOnce({
			data: { ...sameRevisionSnapshot, revisionId: "rev-2" },
		});
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "packaging",
				optionId: "pack-box",
				optionLabel: "分箱包裝",
			});
		});
		expect(postMock).toHaveBeenCalledWith(
			"/buyers/me/procurement/requests/req-1/answers",
			expect.objectContaining({
				json: expect.objectContaining({
					slotKey: "packaging",
					optionId: "pack-box",
					expectedRevisionId: "rev-1",
				}),
			}),
		);
		expect(result.current.question).toBeNull();
	});

	it.each([
		["新版 revision", { revisionId: "rev-2" }],
		["已在搜尋候選", { status: "MATCHING" }],
		["候選已就緒", { status: "CANDIDATES_READY" }],
		[
			"新中斷未提供問題",
			{ openConfirmation: SNAPSHOT_WITH_OPEN_CONFIRMATION.openConfirmation },
		],
		["明確無效的中斷問題", { openConfirmation: { question: null } }],
		["分類舊資料", { cropDisambiguation: legacyCrop }],
	])("%s 不沿用普通選填問題", async (_case, overrides) => {
		const result = await arriveAtOptionalQuestion();
		getMock.mockResolvedValueOnce({
			data: { ...sameRevisionSnapshot, ...overrides },
		});
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toBeNull();
		if ("cropDisambiguation" in overrides) {
			expect(result.current.cropDisambiguation).toMatchObject(legacyCrop);
		}
	});

	it("新權威問題取代普通選填問題及 legacy 分類卡", async () => {
		const result = await arriveAtOptionalQuestion();
		getMock.mockResolvedValueOnce({
			data: {
				...sameRevisionSnapshot,
				cropDisambiguation: legacyCrop,
				openConfirmation: {
					...SNAPSHOT_WITH_OPEN_CONFIRMATION.openConfirmation,
					question: CROP_QUESTION,
				},
			},
		});
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toMatchObject(CROP_QUESTION);
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-1");
		expect(result.current.cropDisambiguation).toBeNull();
	});

	it("已解掉的中斷即使 revision 未變也不保留問題", async () => {
		const result = await arriveAtOpenCropInterrupt();
		getMock.mockResolvedValueOnce({ data: sameRevisionSnapshot });
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toBeNull();
		expect(result.current.pendingInterrupt).toBeNull();
	});

	it("沒有 interrupt 的舊分類問句不能被當成普通問題保留", async () => {
		const result = await arriveAtOptionalQuestion({
			...packagingQuestion,
			slotKey: "crop_disambiguation",
		});
		getMock.mockResolvedValueOnce({ data: sameRevisionSnapshot });
		await act(async () => {
			await result.current.refreshCurrentSnapshot();
		});
		expect(result.current.question).toBeNull();
	});

	it("不同 request 的 snapshot 不套用到目前問題", async () => {
		const result = await arriveAtOptionalQuestion();
		getMock.mockResolvedValueOnce({
			data: { ...sameRevisionSnapshot, requestId: "req-other" },
		});
		await act(async () => {
			expect(await result.current.refreshCurrentSnapshot()).toBe(false);
		});
		expect(result.current.requestId).toBe("req-1");
		expect(result.current.question).toMatchObject(packagingQuestion);
	});
});

describe("分類中斷的權威問句還原", () => {
	const categoryQuestion = {
		...CROP_QUESTION,
		prompt: "請選擇豬肉品項，或補充名稱：",
		allowFreeText: true,
		options: [
			{ optionId: "category-102", label: "豬五花", value: "豬五花" },
			{ optionId: "crop-other", label: "自行填寫品項與單價", value: "OTHER" },
		],
	};
	const legacyCard = {
		prompt: "舊分類卡",
		options: [{ optionId: "legacy-crop", label: "舊選項", cropCode: "LA1" }],
	};
	const snapshotWithQuestion = (question: unknown) => ({
		...SNAPSHOT_WITH_OPEN_CONFIRMATION,
		cropDisambiguation: legacyCard,
		openConfirmation: {
			...SNAPSHOT_WITH_OPEN_CONFIRMATION.openConfirmation,
			question,
		},
	});

	it.each([
		[
			"大類",
			{
				...categoryQuestion,
				options: [
					{ optionId: "category-101", label: "豬肉", value: "豬肉" },
					categoryQuestion.options[1],
				],
			},
		],
		["小類", categoryQuestion],
		[
			"自填價",
			{
				...categoryQuestion,
				allowFreeText: false,
				options: [categoryQuestion.options[1]],
			},
		],
	])("reload 還原%s問題、保留 opaque optionId 並抑制舊分類卡", async (_stage, question) => {
		getMock.mockResolvedValueOnce({ data: snapshotWithQuestion(question) });
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		expect(result.current.question).toMatchObject(question);
		expect(result.current.cropDisambiguation).toBeNull();
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-1");
	});

	it("resume 失敗後以最新 snapshot 還原下一層問題及 interrupt", async () => {
		getMock.mockResolvedValueOnce({
			data: snapshotWithQuestion(CROP_QUESTION),
		});
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		postMock.mockRejectedValueOnce(
			new ApiClientError("已更新，請繼續確認", 409),
		);
		const latest = snapshotWithQuestion(categoryQuestion);
		getMock.mockResolvedValueOnce({
			data: {
				...latest,
				openConfirmation: { ...latest.openConfirmation, interruptId: "int-2" },
			},
		});
		await act(async () => {
			await result.current.resume({ freeText: "豬五花" });
		});
		expect(result.current.question).toMatchObject(categoryQuestion);
		expect(result.current.pendingInterrupt?.interruptId).toBe("int-2");
		expect(result.current.cropDisambiguation).toBeNull();
	});

	it("舊版或不完整問句保留 legacy 分類卡，不從 categoryId 猜選項", async () => {
		getMock.mockResolvedValueOnce({
			data: snapshotWithQuestion({
				slotKey: "crop_disambiguation",
				prompt: 42,
			}),
		});
		const { result } = renderProcurementSession("req-1");
		await waitFor(() => expect(result.current.restoring).toBe(false));
		expect(result.current.question).toBeNull();
		expect(result.current.cropDisambiguation).toMatchObject(legacyCard);
	});
});

describe("問題卡答題的傳輸路由：有未回覆的 interrupt 時走 resume，不打 /answers", () => {
	it("選項送出打的是 runs/{interruptId}/resume（payload 帶 optionId），完全不碰 /answers", async () => {
		const result = await arriveAtOpenCropInterrupt();
		postMock.mockResolvedValueOnce({
			data: { runId: "run-9", runStatus: "SUCCEEDED" },
		});
		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		expect(postMock).toHaveBeenCalledTimes(1);
		const [path, init] = postMock.mock.calls[0] as [
			string,
			{ json: Record<string, unknown> },
		];
		expect(path).toBe("/buyers/me/procurement/runs/int-1/resume");
		expect(init.json).toMatchObject({
			interruptId: "int-1",
			payload: { optionId: "group-moa-group:f102791d" },
		});
		expect(
			postMock.mock.calls.some(([p]) => String(p).endsWith("/answers")),
		).toBe(false);
	});

	it("crop-other 子表單的自填描述與價格一起帶進 resume payload（後端 cropDisambiguationResponseV1 的三個欄位）", async () => {
		const result = await arriveAtOpenCropInterrupt();
		postMock.mockResolvedValueOnce({
			data: { runId: "run-9", runStatus: "SUCCEEDED" },
		});
		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "crop-other",
				optionLabel: "其他（自己輸入）",
				freeText: "有機甘藍",
				selfEnteredPriceAmount: "120",
			});
		});

		const [, init] = postMock.mock.calls[0] as [
			string,
			{ json: Record<string, unknown> },
		];
		expect(init.json).toMatchObject({
			payload: {
				optionId: "crop-other",
				freeText: "有機甘藍",
				selfEnteredPriceAmount: "120",
			},
		});
	});

	it("resume 成功後收掉已回覆的問題卡，並讓權威 snapshot 校正 pendingInterrupt", async () => {
		const result = await arriveAtOpenCropInterrupt();
		expect(result.current.question).not.toBeNull();

		postMock.mockResolvedValueOnce({
			data: { runId: "run-9", runStatus: "SUCCEEDED" },
		});
		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.question).toBeNull();
		expect(result.current.pendingInterrupt).toBeNull();
		// 買家選的選項留在對話裡（他確實做了這個選擇）。
		expect(result.current.turns.some((t) => t.text === "甘藍（甘藍）")).toBe(
			true,
		);
	});

	it("resume 失敗時撤掉樂觀買家泡泡，pendingInterrupt 交給 snapshot 校正後仍在", async () => {
		const result = await arriveAtOpenCropInterrupt();
		postMock.mockRejectedValueOnce(
			new ApiClientError(
				"resume failed",
				500,
				undefined,
				undefined,
				"INTERNAL",
			),
		);
		getMock.mockResolvedValueOnce({ data: SNAPSHOT_WITH_OPEN_CONFIRMATION });

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.turns.some((t) => t.text === "甘藍（甘藍）")).toBe(
			false,
		);
		expect(result.current.question).not.toBeNull();
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});

	it("沒有未回覆的 interrupt 時維持既有 /answers 路徑（一般引導式問句不受影響）", async () => {
		streamProcurementTurnMock.mockResolvedValueOnce({
			requestId: "req-1",
			revisionId: "rev-1",
			reply: "這次想採購多少？",
			degraded: false,
			slots: { item: "甘藍" },
			missingRequired: ["quantity"],
			rejected: [],
			question: {
				slotKey: "quantity",
				prompt: "這次想採購多少？",
				allowFreeText: true,
				canSkip: false,
				options: [],
			},
		});
		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});

		const { result } = renderProcurementSession();
		await act(async () => {
			await result.current.submitTurn("甘藍", "client-req-1");
		});
		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.pendingInterrupt).toBeNull();

		postMock.mockResolvedValueOnce({
			data: {
				requestId: "req-1",
				revisionId: "rev-3",
				reply: "好的，記下了。",
				degraded: false,
				slots: { item: "甘藍", quantity: 20 },
				missingRequired: [],
				rejected: [],
			},
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "quantity",
				freeText: "20",
			});
		});

		const [path] = postMock.mock.calls[0] as [string];
		expect(path).toBe("/buyers/me/procurement/requests/req-1/answers");
	});
});

describe("submitAnswer 不樂觀清空 pendingInterrupt", () => {
	it("resume 失敗且快照校正也失敗時，pendingInterrupt 仍在——否則下一次作答會回頭打 /answers 再撞一次 409", async () => {
		const result = await arriveAtOpenCropInterrupt();
		postMock.mockRejectedValueOnce(
			new ApiClientError(
				"resume failed",
				500,
				undefined,
				undefined,
				"INTERNAL",
			),
		);
		// refreshCurrentSnapshot 自己也讀不到（斷線／後端暫時不可用）：這時本地
		// 就是唯一還記得「有一個未回覆的中斷」的地方，提早清掉就再也回不來。
		getMock.mockRejectedValueOnce(new Error("network down"));

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});
});

describe("resume 帶回 TurnResult 時走 applyTurnResult（backend-payuni 2026-09-03）", () => {
	const NEXT_TURN_RESULT = {
		requestId: "req-1",
		revisionId: "rev-2",
		reply: "好的，主要資訊都到齊了。 希望什麼時候送達？",
		degraded: false,
		slots: { item: "甘藍", delivery_city: "台中市" },
		missingRequired: [],
		rejected: [],
		question: {
			slotKey: "delivery_date",
			prompt: "希望什麼時候送達？",
			allowFreeText: true,
			canSkip: false,
			options: [],
		},
	};

	it("分類自由描述只送 freeText，接收同 slotKey 的下一層問題與新 interrupt", async () => {
		const result = await arriveAtOpenCropInterrupt({
			...CROP_QUESTION,
			allowFreeText: true,
		});
		const nextQuestion = {
			...CROP_QUESTION,
			prompt: "請選擇豬肉品項，或補充規格：",
			allowFreeText: true,
			options: [
				{ optionId: "fixture-pork-belly", label: "豬五花", value: "豬五花" },
				{ optionId: "crop-other", label: "自行填寫品項與單價", value: "OTHER" },
			],
		};
		postMock.mockResolvedValueOnce({
			data: {
				runId: "run-9",
				runStatus: "WAITING_FOR_INPUT",
				result: {
					...NEXT_TURN_RESULT,
					question: nextQuestion,
					interruptId: "int-2",
					interruptKind: "CLARIFICATION",
				},
			},
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				freeText: "臺灣豬五花",
			});
		});

		const [path, init] = postMock.mock.calls[0] as [
			string,
			{ json: { payload: unknown } },
		];
		expect(path).toBe("/buyers/me/procurement/runs/int-1/resume");
		expect(JSON.parse(JSON.stringify(init.json.payload))).toEqual({
			freeText: "臺灣豬五花",
		});
		expect(result.current.question).toMatchObject(nextQuestion);
		expect(result.current.pendingInterrupt).toEqual({
			interruptId: "int-2",
			reason: "CLARIFICATION",
		});

		postMock.mockResolvedValueOnce({
			data: {
				runId: "run-10",
				runStatus: "SUCCEEDED",
				result: NEXT_TURN_RESULT,
			},
		});
		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "fixture-pork-belly",
				optionLabel: "豬五花",
			});
		});
		expect(postMock.mock.calls[1][0]).toBe(
			"/buyers/me/procurement/runs/int-2/resume",
		);
		expect(
			postMock.mock.calls.some(([candidatePath]) =>
				String(candidatePath).endsWith("/answers"),
			),
		).toBe(false);
	});

	it("助理回覆與下一題一次落地，不必再回讀 snapshot", async () => {
		const result = await arriveAtOpenCropInterrupt();
		postMock.mockResolvedValueOnce({
			data: {
				runId: "run-9",
				runStatus: "SUCCEEDED",
				result: NEXT_TURN_RESULT,
			},
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.question).toMatchObject({ slotKey: "delivery_date" });
		expect(
			result.current.turns.some((t) => t.text === NEXT_TURN_RESULT.reply),
		).toBe(true);
		expect(result.current.pendingInterrupt).toBeNull();
		expect(result.current.revisionId).toBe("rev-2");
	});

	it("resume 沒帶 result（冪等重播／後端尚未部署）時退回收卡＋回讀 snapshot", async () => {
		const result = await arriveAtOpenCropInterrupt();
		// setup 那一輪已經放了一則助理訊息（消歧提示），所以判準是「沒有新增」
		// 而不是「一則都沒有」。
		const assistantTurnsBefore = result.current.turns.filter(
			(t) => t.role === "assistant",
		).length;
		postMock.mockResolvedValueOnce({
			data: { runId: "run-9", runStatus: "SUCCEEDED" },
		});
		getMock.mockResolvedValueOnce({
			data: SNAPSHOT_WITHOUT_OPEN_CONFIRMATION,
		});

		await act(async () => {
			await result.current.submitAnswer({
				slotKey: "crop_disambiguation",
				optionId: "group-moa-group:f102791d",
				optionLabel: "甘藍（甘藍）",
			});
		});

		await waitFor(() => expect(result.current.pendingTurn).toBe(false));
		expect(result.current.question).toBeNull();
		expect(result.current.pendingInterrupt).toBeNull();
		// 沒有新的助理回覆可放——這正是後端補上 result 之前的行為。
		expect(
			result.current.turns.filter((t) => t.role === "assistant").length,
		).toBe(assistantTurnsBefore);
	});
});
