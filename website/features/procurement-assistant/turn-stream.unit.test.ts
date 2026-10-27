import { afterEach, describe, expect, it, vi } from "vitest";
import type { AgentActivityEvent } from "./activity";
import { type AgentStep, stepLabel } from "./steps";
import {
	mergeLiveActivity,
	mergeLiveStep,
	resetStreamProbe,
	streamProcurementTurn,
	TurnStreamEventError,
	TurnStreamUnavailableError,
} from "./turn-stream";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			buildUrl: vi.fn(
				() => "https://gw.test/api/v1/app/buyers/me/procurement/turns/stream",
			),
		},
	};
});

const BODY = { message: "我要訂水果禮盒", clientRequestId: "cid-1" };

const RESULT = {
	requestId: "1234567890123456788",
	revisionId: "1234567890123456790",
	slots: { item: "水果禮盒" },
	missingRequired: ["delivery_city"],
	rejected: [],
	degraded: false,
	reply: "好的，記下水果禮盒了。",
};

function aguiFrame(event: Record<string, unknown>): string {
	return `data: ${JSON.stringify(event)}\n\n`;
}

function stepFrame(payload: Record<string, unknown>): string {
	return aguiFrame({ type: "CUSTOM", name: "step_detail", value: payload });
}

function doneFrame(result: unknown): string {
	return aguiFrame({
		type: "RUN_FINISHED",
		threadId: "thread-golden",
		runId: "r1",
		result,
	});
}

function errorFrame(code: number, message: string, reason?: string): string {
	return aguiFrame({
		type: "RUN_ERROR",
		message,
		code: String(code),
		...(reason === undefined ? {} : { reason }),
	});
}

/** RUN_FINISHED 帶 outcome.type="interrupt"（wire 是陣列，v1 一次 run 最多
 * 一個未結案 interrupt）。
 *
 * ⚠️ 2026-09-03 更正：舊註解寫「interruptId／interruptKind 已經直接內嵌在
 * result 上（unary 與串流同形狀）」——**那是錯的**。實測 dev 的 RUN_FINISHED
 * 頂層鍵是 {type,threadId,runId,result,outcome}，result 的 17 個鍵裡沒有
 * interruptId；interrupt 只存在於 outcome。unary SubmitTurn 才有那兩個欄位
 * （gateway procurement_response_mapping.go:206）。統一形狀是
 * streamProcurementTurn 的職責，見它回傳前的正規化。
 *
 * 所以呼叫這個 helper 時**不要**自己在 result 裡塞 interruptId——那會做出一份
 * production 不存在的形狀，測試就變成只驗透傳、驗不到推導。 */
function interruptFrame(
	result: unknown,
	interruptId: string,
	reason: string,
): string {
	return aguiFrame({
		type: "RUN_FINISHED",
		threadId: "thread-golden",
		runId: "r1",
		result,
		outcome: {
			type: "interrupt",
			interrupts: [{ id: interruptId, reason }],
		},
	});
}

/** 一個**真正分段**的 SSE response：每個 chunk 之間隔 gapMs，模擬伺服器
 * 邊執行邊推。用來證明消費端是邊收邊播，而不是收完才一次吐出來。 */
function chunkedSse(chunks: string[], gapMs: number): Response {
	const encoder = new TextEncoder();
	let i = 0;
	const stream = new ReadableStream<Uint8Array>({
		async pull(controller) {
			if (i >= chunks.length) {
				controller.close();
				return;
			}
			await new Promise((r) => setTimeout(r, gapMs));
			controller.enqueue(encoder.encode(chunks[i]));
			i += 1;
		},
	});
	return new Response(stream, {
		status: 200,
		headers: { "content-type": "text/event-stream" },
	});
}

afterEach(() => {
	vi.unstubAllGlobals();
	resetStreamProbe();
});

function activityFrame(payload: Record<string, unknown>): string {
	return aguiFrame({ type: "CUSTOM", name: "activity", value: payload });
}

describe("mergeLiveStep", () => {
	it("相同 status/duration 是語意 no-op，沿用原 array reference", () => {
		const steps: AgentStep[] = [
			{ code: "UNDERSTAND_INPUT", status: "RUNNING" },
		];
		const merged = mergeLiveStep(steps, {
			code: "UNDERSTAND_INPUT",
			status: "RUNNING",
		});

		expect(merged).toBe(steps);
	});

	it("相同 RUNNING status 補上 StepNarrator text 時會更新原列", () => {
		const steps: AgentStep[] = [
			{ code: "UNDERSTAND_INPUT", status: "RUNNING" },
		];
		const merged = mergeLiveStep(steps, {
			code: "UNDERSTAND_INPUT",
			status: "RUNNING",
			text: "已讀懂這一輪的珍珠芭樂需求",
		});

		expect(merged).not.toBe(steps);
		expect(stepLabel(merged[0])).toBe("已讀懂這一輪的珍珠芭樂需求");
	});

	it("SUCCEEDED 後的 RUNNING heartbeat 不退回進行中、不清完成內容，且是 no-op", () => {
		const steps: AgentStep[] = [
			{
				code: "UNDERSTAND_INPUT",
				status: "SUCCEEDED",
				durationMs: 125,
				text: "已完成需求理解",
			},
		];
		const merged = mergeLiveStep(steps, {
			code: "UNDERSTAND_INPUT",
			status: "RUNNING",
			text: "遲到的執行中文案",
		});

		expect(merged).toBe(steps);
		expect(merged[0]).toEqual({
			code: "UNDERSTAND_INPUT",
			status: "SUCCEEDED",
			durationMs: 125,
			text: "已完成需求理解",
		});
	});

	it("RUNNING → SUCCEEDED 會改變 UI，不能算 duplicate", () => {
		const steps: AgentStep[] = [
			{ code: "UNDERSTAND_INPUT", status: "RUNNING" },
		];
		const merged = mergeLiveStep(steps, {
			code: "UNDERSTAND_INPUT",
			status: "SUCCEEDED",
			durationMs: 125,
		});

		expect(merged).not.toBe(steps);
		expect(merged[0]).toEqual({
			code: "UNDERSTAND_INPUT",
			status: "SUCCEEDED",
			durationMs: 125,
		});
	});
});

describe("mergeLiveActivity", () => {
	it("entry 事件（新 rowId）append 一列", () => {
		const events = mergeLiveActivity([], {
			rowId: "1",
			code: "EXTRACT_SLOTS",
			label: "抓出品項、數量和交期",
			status: "RUNNING",
			text: "",
		});
		expect(events).toEqual([
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "RUNNING",
				text: "",
			},
		]);
	});

	it("completion 事件覆寫同 rowId 的 entry", () => {
		const events = mergeLiveActivity(
			[
				{
					rowId: "1",
					code: "EXTRACT_SLOTS",
					label: "抓出品項、數量和交期",
					status: "RUNNING",
					text: "",
				},
			],
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			},
		);
		expect(events).toEqual([
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			},
		]);
	});

	it("不同 rowId 各自獨立 append", () => {
		const events = mergeLiveActivity(
			[
				{
					rowId: "1",
					code: "EXTRACT_SLOTS",
					label: "抓出品項、數量和交期",
					status: "SUCCEEDED",
					text: "抓到你要的蘋果",
				},
			],
			{
				rowId: "2",
				code: "VALIDATE_SLOTS",
				label: "核對條件合不合理",
				status: "RUNNING",
				text: "",
			},
		);
		expect(events).toEqual([
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			},
			{
				rowId: "2",
				code: "VALIDATE_SLOTS",
				label: "核對條件合不合理",
				status: "RUNNING",
				text: "",
			},
		]);
	});

	it("重送的舊 entry 事件不得覆蓋已存在的 completion（防 SSE replay 倒退）", () => {
		const events = mergeLiveActivity(
			[
				{
					rowId: "1",
					code: "EXTRACT_SLOTS",
					label: "抓出品項、數量和交期",
					status: "SUCCEEDED",
					text: "抓到你要的蘋果",
				},
			],
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "RUNNING",
				text: "",
			},
		);
		expect(events).toEqual([
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			},
		]);
	});
});

describe("streamProcurementTurn", () => {
	it("邊收邊播：每個 step 在它抵達的當下就交給呼叫端，不是等 RUN_FINISHED 才一次全給", async () => {
		const chunks = [
			stepFrame({
				code: "UNDERSTAND_INPUT",
				status: "RUNNING",
				label: "讀你寫的需求",
			}),
			stepFrame({
				code: "UNDERSTAND_INPUT",
				status: "SUCCEEDED",
				label: "讀你寫的需求",
				durationMs: 120,
			}),
			stepFrame({
				code: "EXTRACT_SLOTS",
				status: "RUNNING",
				label: "抓出品項、數量和交期",
			}),
			stepFrame({
				code: "EXTRACT_SLOTS",
				status: "SUCCEEDED",
				label: "抓出品項、數量和交期",
				durationMs: 4200,
			}),
			doneFrame(RESULT),
		];
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(chunkedSse(chunks, 20)));

		const seen: Array<{ code: string; atMs: number }> = [];
		const t0 = Date.now();
		const result = (await streamProcurementTurn(BODY, {
			onStep: (s: AgentStep) =>
				seen.push({ code: s.code, atMs: Date.now() - t0 }),
		})) as typeof RESULT;

		expect(seen.map((s) => s.code)).toEqual([
			"UNDERSTAND_INPUT",
			"UNDERSTAND_INPUT",
			"EXTRACT_SLOTS",
			"EXTRACT_SLOTS",
		]);
		// 關鍵斷言：第一步遠早於最後一步抵達——若是收完才播，兩者會同時發生。
		expect(seen[0].atMs).toBeLessThan(seen[3].atMs);
		expect(result.reply).toBe(RESULT.reply);
	});

	it("只在第一個通過 allowlist 的 public step 通知開始，不把連線/RUN_FINISHED 冒充 ACK", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				chunkedSse(
					[
						stepFrame({ code: "RAW_PRIVATE_TOOL", status: "RUNNING" }),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "FUTURE_PRIVATE_STATUS",
						}),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
						}),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "SUCCEEDED",
						}),
						doneFrame(RESULT),
					],
					0,
				),
			),
		);
		const onStartObserved = vi.fn();
		const seen: string[] = [];
		await streamProcurementTurn(BODY, {
			onStartObserved,
			onStep: (step) => seen.push(step.code),
		});
		expect(onStartObserved).toHaveBeenCalledTimes(1);
		expect(onStartObserved).toHaveBeenCalledWith(expect.any(Number));
		expect(seen).toEqual(["UNDERSTAND_INPUT", "UNDERSTAND_INPUT"]);
	});

	it("只把 merge 語意 no-op 累加為 duplicate，terminal 才交付 aggregate", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				chunkedSse(
					[
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
						}),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
						}),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "SUCCEEDED",
							durationMs: 125,
						}),
						stepFrame({
							code: "UNDERSTAND_INPUT",
							status: "RUNNING",
						}),
						doneFrame(RESULT),
					],
					0,
				),
			),
		);
		const seenStatuses: AgentStep["status"][] = [];
		const onDuplicateStepSuppressed = vi.fn();
		const onTerminalObserved = vi.fn();

		await streamProcurementTurn(BODY, {
			onStep: (step) => seenStatuses.push(step.status),
			onDuplicateStepSuppressed,
			onTerminalObserved,
		});

		expect(seenStatuses).toEqual(["RUNNING", "SUCCEEDED"]);
		expect(onDuplicateStepSuppressed).toHaveBeenCalledTimes(2);
		expect(onTerminalObserved).toHaveBeenCalledTimes(1);
		expect(onTerminalObserved).toHaveBeenCalledWith({
			observedAt: expect.any(Number),
			duplicateStepSuppressedCount: 2,
		});
	});

	it("RUN_FINISHED.result 的 TurnResult 原樣回傳（大整數 ID 不被 JSON.parse 精度吃掉）", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(chunkedSse([doneFrame(RESULT)], 0)),
		);
		const result = (await streamProcurementTurn(BODY, {
			onStep: () => {},
		})) as typeof RESULT;
		expect(result.requestId).toBe("1234567890123456788");
		expect(result.revisionId).toBe("1234567890123456790");
	});

	it("404（gateway 未部署串流）→ 標記不可用並要求降級，之後不再重試探測", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(new Response("not found", { status: 404 }));
		vi.stubGlobal("fetch", fetchMock);

		await expect(
			streamProcurementTurn(BODY, { onStep: () => {} }),
		).rejects.toBeInstanceOf(TurnStreamUnavailableError);
		// 第二輪連 fetch 都不該發——probe-once。
		await expect(
			streamProcurementTurn(BODY, { onStep: () => {} }),
		).rejects.toBeInstanceOf(TurnStreamUnavailableError);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it("429（併發已滿）→ 要求降級，但不永久標記不可用", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(new Response("too many", { status: 429 }))
			.mockResolvedValueOnce(chunkedSse([doneFrame(RESULT)], 0));
		vi.stubGlobal("fetch", fetchMock);

		await expect(
			streamProcurementTurn(BODY, { onStep: () => {} }),
		).rejects.toBeInstanceOf(TurnStreamUnavailableError);
		// 下一輪照樣試串流。
		await expect(
			streamProcurementTurn(BODY, { onStep: () => {} }),
		).resolves.toMatchObject({ reply: RESULT.reply });
	});

	it("AG-UI RUN_ERROR → TurnStreamEventError（這一輪真的失敗，不該降級重送）", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(chunkedSse([errorFrame(500, "internal error")], 0)),
		);
		const onTerminalObserved = vi.fn();
		await expect(
			streamProcurementTurn(BODY, {
				onStep: () => {},
				onTerminalObserved,
			}),
		).rejects.toMatchObject({
			name: "TurnStreamEventError",
			code: 500,
			message: "internal error",
		});
		expect(onTerminalObserved).toHaveBeenCalledTimes(1);
		expect(onTerminalObserved).toHaveBeenCalledWith({
			observedAt: expect.any(Number),
			duplicateStepSuppressedCount: 0,
		});
	});

	it("in-band AG-UI RUN_ERROR 帶 reason 時，TurnStreamEventError.reason 是後端給的真值（不再永遠是空字串）", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					chunkedSse(
						[
							errorFrame(
								409,
								"request is parked on an open interrupt",
								"APPROVAL_PENDING",
							),
						],
						0,
					),
				),
		);
		const err = await streamProcurementTurn(BODY, {
			onStep: () => {},
		}).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TurnStreamEventError);
		expect((err as TurnStreamEventError).reason).toBe("APPROVAL_PENDING");
		expect((err as TurnStreamEventError).message).toBe(
			"request is parked on an open interrupt",
		);
	});

	it("in-band AG-UI RUN_ERROR 沒有 reason 時，TurnStreamEventError.reason 維持空字串", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(chunkedSse([errorFrame(500, "internal error")], 0)),
		);
		const err = await streamProcurementTurn(BODY, {
			onStep: () => {},
		}).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TurnStreamEventError);
		expect((err as TurnStreamEventError).reason).toBe("");
	});

	it("409（clientRequestId 衝突）的 RUN_ERROR 帶原始碼，讓呼叫端能顯示後端文案", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					chunkedSse([errorFrame(409, "clientRequestId 已用於其他需求")], 0),
				),
		);
		const err = await streamProcurementTurn(BODY, {
			onStep: () => {},
		}).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TurnStreamEventError);
		expect((err as TurnStreamEventError).code).toBe(409);
	});

	it("HTTP 409 直接成為 terminal turn error，不標記 stream unavailable", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(new Response("conflict", { status: 409 }));
		vi.stubGlobal("fetch", fetchMock);
		const onTerminalObserved = vi.fn();

		await expect(
			streamProcurementTurn(BODY, {
				onStep: () => {},
				onTerminalObserved,
			}),
		).rejects.toMatchObject({
			name: "TurnStreamEventError",
			code: 409,
		});
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(onTerminalObserved).toHaveBeenCalledTimes(1);
		expect(onTerminalObserved).toHaveBeenCalledWith({
			observedAt: expect.any(Number),
			duplicateStepSuppressedCount: 0,
		});
	});

	it("pre-stream HTTP 409 帶 reason 時，TurnStreamEventError 用後端真正的 message／reason，不再硬編一句只對身分衝突講得通的前端文案", async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(
				JSON.stringify({
					code: 409,
					reason: "APPROVAL_PENDING",
					message: "request is parked on an open interrupt",
				}),
				{ status: 409 },
			),
		);
		vi.stubGlobal("fetch", fetchMock);

		const err = await streamProcurementTurn(BODY, {
			onStep: () => {},
		}).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TurnStreamEventError);
		expect((err as TurnStreamEventError).code).toBe(409);
		expect((err as TurnStreamEventError).reason).toBe("APPROVAL_PENDING");
		expect((err as TurnStreamEventError).message).toBe(
			"request is parked on an open interrupt",
		);
	});

	it("pre-stream HTTP 409 沒有 reason 時，TurnStreamEventError.reason 是空字串", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(new Response("conflict", { status: 409 })),
		);
		const err = await streamProcurementTurn(BODY, {
			onStep: () => {},
		}).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TurnStreamEventError);
		expect((err as TurnStreamEventError).reason).toBe("");
	});

	// ⛔ 2026-09-03：真實 wire 的迴歸測試。上面那支測試自己在 result 裡塞了
	// interruptId／interruptKind，但 **production 從來不送那兩個欄位**——實測
	// dev 的 RUN_FINISHED 頂層鍵是 {type,threadId,runId,result,outcome}，而
	// result 的 17 個鍵裡沒有 interruptId，interrupt 只存在於 outcome.interrupts[0]。
	// 於是 useProcurementSession 的 `data.interruptId` 判斷永遠不成立、
	// pendingInterrupt 設不起來、前端退回打同步 /crop-disambiguation，被後端的
	// GuardNoOpenInterrupt 擋成 409 APPROVAL_PENDING。
	// 本測試刻意餵**不含** interrupt 欄位的 result（即 production 的形狀）。
	it("result 不含 interruptId 時（production 真實形狀），仍要從 outcome.interrupts 推導出 interruptId／interruptKind", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					chunkedSse([interruptFrame(RESULT, "int-9", "CLARIFICATION")], 0),
				),
		);
		const result = (await streamProcurementTurn(BODY, {
			onStep: () => {},
		})) as Record<string, unknown>;
		// session-logic.ts:249 的成對不變式：兩個欄位必須同時出現，否則
		// normalizeProcurementTurnResult 會整份回 null。
		expect(result.interruptId).toBe("int-9");
		expect(result.interruptKind).toBe("CLARIFICATION");
	});

	it("outcome.type=success 時不得憑空生出 interrupt 欄位", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				chunkedSse(
					[
						aguiFrame({
							type: "RUN_FINISHED",
							threadId: "thread-golden",
							runId: "r1",
							result: RESULT,
							outcome: { type: "success" },
						}),
					],
					0,
				),
			),
		);
		const result = (await streamProcurementTurn(BODY, {
			onStep: () => {},
		})) as Record<string, unknown>;
		expect(result.interruptId).toBeUndefined();
		expect(result.interruptKind).toBeUndefined();
	});

	// 保留這支：它驗的是「result 本來就帶 interrupt 欄位時要原樣透傳」（unary
	// 降級路徑會走到這個形狀）。它**不**證明串流能從 outcome 推導——那是上面
	// 那支新測試的職責。兩支合起來才涵蓋兩種來源。
	it("result 已帶 interrupt 欄位時原樣透傳，不把整份結果丟棄", async () => {
		const resultWithInterrupt = {
			...RESULT,
			interruptId: "int-1",
			interruptKind: "CLARIFICATION",
		};
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					chunkedSse(
						[interruptFrame(resultWithInterrupt, "int-1", "CLARIFICATION")],
						0,
					),
				),
		);
		const onTerminalObserved = vi.fn();
		const result = await streamProcurementTurn(BODY, {
			onStep: () => {},
			onTerminalObserved,
		});
		expect(result).toEqual(resultWithInterrupt);
		// 這是合法終態，跟其他 terminal 分支一樣要回報一次 onTerminalObserved。
		expect(onTerminalObserved).toHaveBeenCalledTimes(1);
	});

	it("outcome.type=interrupt 但 result 的 JSON round-trip 失敗 → 視為沒收到結果，一律走同步端點降級（不再靠 outcome 另開一條特殊路徑繞過這個形狀檢查）", async () => {
		// result 故意給一個「合法字串值，但內容長得像裸數字」的邊界案例：
		// stream-client 把它 JSON.stringify 成 rawData 交給 onDone，turn-stream
		// 再用 parseJsonSafe（會先跑 preserveBigInts 正規表示式保留 64-bit ID）
		// 解回來——但那支正規表示式不理解字串邊界，會把字串內容裡「看起來像
		// 16 位數字」的片段也加上引號，round-trip 後語法壞掉、拋錯。
		// landed.result 因此在 onDone 的 try/catch 裡被吃掉、維持 undefined——
		// 這份壞掉的 result 不論 outcome 是不是 interrupt 都一樣沒有可用內容，
		// 所以現在跟一般成功結果壞掉時同一個下場：降級走同步端點重新查詢。
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				chunkedSse(
					[
						aguiFrame({
							type: "RUN_FINISHED",
							threadId: "thread-golden",
							runId: "r1",
							result: ":1234567890123456,",
							outcome: {
								type: "interrupt",
								interrupts: [{ id: "int-2", reason: "CLARIFICATION" }],
							},
						}),
					],
					0,
				),
			),
		);
		await expect(
			streamProcurementTurn(BODY, { onStep: () => {} }),
		).rejects.toBeInstanceOf(TurnStreamUnavailableError);
	});

	it("串流開了但中途斷線（沒有 RUN_FINISHED）→ 要求降級，把結果從同步端點撈回來", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					chunkedSse(
						[stepFrame({ code: "UNDERSTAND_INPUT", status: "RUNNING" })],
						0,
					),
				),
		);
		const onTerminalObserved = vi.fn();
		await expect(
			streamProcurementTurn(BODY, {
				onStep: () => {},
				onTerminalObserved,
			}),
		).rejects.toBeInstanceOf(TurnStreamUnavailableError);
		expect(onTerminalObserved).not.toHaveBeenCalled();
	});

	it("送出的 body 就是同步版那一份契約（requestId 省略時不送 null）", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(chunkedSse([doneFrame(RESULT)], 0));
		vi.stubGlobal("fetch", fetchMock);
		await streamProcurementTurn(
			{ requestId: undefined, message: "x", clientRequestId: "cid-2" },
			{ onStep: () => {} },
		);
		const sent = JSON.parse(fetchMock.mock.calls[0][1].body as string);
		expect(sent).toEqual({ message: "x", clientRequestId: "cid-2" });
		expect(fetchMock.mock.calls[0][1].headers.get("accept")).toBe(
			"text/event-stream",
		);
	});

	it("activity CUSTOM 事件轉呼叫 onActivity，不誤觸 onStep", async () => {
		const chunks = [
			activityFrame({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "RUNNING",
				text: "",
			}),
			activityFrame({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			}),
			doneFrame(RESULT),
		];
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(chunkedSse(chunks, 20)));

		const seenActivity: AgentActivityEvent[] = [];
		const seenSteps: AgentStep[] = [];
		await streamProcurementTurn(BODY, {
			onStep: (s) => seenSteps.push(s),
			onActivity: (a) => seenActivity.push(a),
		});

		expect(seenSteps).toEqual([]);
		expect(seenActivity).toEqual([
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "RUNNING",
				parentRowId: undefined,
				count: undefined,
				text: "",
			},
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "SUCCEEDED",
				parentRowId: undefined,
				count: undefined,
				text: "抓到你要的蘋果",
			},
		]);
	});
});

// 回歸守門，不是新行為：mergeLiveActivity 的「entry 不得覆寫已完成的 text」那條
// 護欄是 T23 之前寫的，判準看 text。T23 的四列 text 恆空，所以護欄對它們**永遠
// 不觸發**——status 才推得動。有人把護欄「簡化」成看 status 就會靜默鎖死這四列。
describe("mergeLiveActivity 對沒有 narrator 的列（T23）", () => {
	it("text 恆空的列仍能依序推進 PENDING → RUNNING → SUCCEEDED", () => {
		const base = {
			rowId: "9002",
			code: "SEARCH_PLATFORM",
			label: "翻平台上的賣家",
			text: "",
		} as const;

		let rows = mergeLiveActivity([], { ...base, status: "PENDING" });
		rows = mergeLiveActivity(rows, { ...base, status: "RUNNING" });
		rows = mergeLiveActivity(rows, {
			...base,
			status: "SUCCEEDED",
			count: "12",
		});

		expect(rows).toHaveLength(1);
		expect(rows[0].status).toBe("SUCCEEDED");
		expect(rows[0].count).toBe("12");
	});

	it("已有敘述的列不被遲到的 entry 事件洗掉（既有護欄仍然成立）", () => {
		const rows = mergeLiveActivity(
			[
				{
					rowId: "1",
					code: "EXTRACT_SLOTS",
					label: "抓出品項、數量和交期",
					status: "SUCCEEDED",
					text: "抓到你要的蘋果",
				},
			],
			{
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "抓出品項、數量和交期",
				status: "RUNNING",
				text: "",
			},
		);

		expect(rows[0].text).toBe("抓到你要的蘋果");
		expect(rows[0].status).toBe("SUCCEEDED");
	});
});
