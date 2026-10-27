import { afterEach, describe, expect, it, vi } from "vitest";
import { AUTH_EXPIRED_EVENT } from "@/lib/api/client";
import {
	AiStreamHttpError,
	AiStreamTimeoutError,
	streamAiChat,
} from "./stream-client";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			buildUrl: vi.fn(() => "https://gw.test/api/v1/app/test/endpoint"),
		},
	};
});

function sseResponse(rawChunks: string[]): Response {
	const encoder = new TextEncoder();
	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			for (const chunk of rawChunks) controller.enqueue(encoder.encode(chunk));
			controller.close();
		},
	});
	return new Response(stream, { status: 200 });
}

function aguiFrame(event: Record<string, unknown>): string {
	return `data: ${JSON.stringify(event)}\n\n`;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("streamAiChat", () => {
	it("carries the stored workspace role on the cookie-authenticated POST", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(
				sseResponse([
					aguiFrame({ type: "RUN_FINISHED", threadId: "t1", runId: "r1" }),
				]),
			);
		vi.stubGlobal("window", {
			Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
		});
		vi.stubGlobal("localStorage", {
			getItem: vi.fn(() => "buyer"),
		});
		vi.stubGlobal("fetch", fetchMock);

		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: { message: "hi" } },
			{
				onText: vi.fn(),
				onNamedEvent: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		);

		const headers = new Headers(fetchMock.mock.calls[0][1].headers);
		expect(headers.get("X-Active-Role")).toBe("BUYER");
		expect(headers.has("X-Requested-With")).toBe(false);
		expect(fetchMock.mock.calls[0][1].credentials).toBe("include");
	});

	it("broadcasts auth expiry with endpoint context when the stream POST gets 401", async () => {
		const dispatchEvent = vi.fn();
		class TestCustomEvent<T> {
			readonly type: string;
			readonly detail: T;
			constructor(type: string, init: { detail: T }) {
				this.type = type;
				this.detail = init.detail;
			}
		}
		vi.stubGlobal("window", {
			dispatchEvent,
			Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
		});
		vi.stubGlobal("CustomEvent", TestCustomEvent);
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				new Response(
					JSON.stringify({
						code: 401,
						message: "session expired",
					}),
					{ status: 401 },
				),
			),
		);

		await expect(
			streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{
					onText: vi.fn(),
					onNamedEvent: vi.fn(),
					onDone: vi.fn(),
					onError: vi.fn(),
				},
			),
		).rejects.toThrow("session expired");

		expect(dispatchEvent).toHaveBeenCalledTimes(1);
		const event = dispatchEvent.mock.calls[0][0] as TestCustomEvent<{
			path: string;
			message: string;
		}>;
		expect(event.type).toBe(AUTH_EXPIRED_EVENT);
		expect(event.detail).toEqual({
			path: "/app/test/endpoint",
			message: "session expired",
		});
	});

	it("dispatches TEXT_MESSAGE_CONTENT deltas via onText", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "TEXT_MESSAGE_CONTENT",
						messageId: "m1",
						delta: "hello",
					}),
				]),
			),
		);
		const onText = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: { message: "hi" } },
			{ onText, onNamedEvent: vi.fn(), onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onText).toHaveBeenCalledWith("hello");
	});

	it("帶 event: 行的 frame 仍走 AG-UI 分派，不被 legacy 分支吞掉", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([
						'event: TEXT_MESSAGE_CONTENT\ndata: {"type":"TEXT_MESSAGE_CONTENT","messageId":"msg-1","delta":"嗨"}\n\n',
					]),
				),
		);
		const onText = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText, onNamedEvent: vi.fn(), onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onText).toHaveBeenCalledWith("嗨");
	});

	it("routes a CUSTOM event through onNamedEvent keyed by its name, value re-serialized to a string", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "CUSTOM",
						name: "step_detail",
						value: { code: "EXTRACT_SLOTS", status: "RUNNING" },
					}),
				]),
			),
		);
		const onNamedEvent = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onNamedEvent).toHaveBeenCalledWith(
			"step_detail",
			JSON.stringify({ code: "EXTRACT_SLOTS", status: "RUNNING" }),
		);
	});

	it("routes TOOL_CALL_RESULT through onNamedEvent keyed by the toolCallName seen on the matching TOOL_CALL_START", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "TOOL_CALL_START",
						toolCallId: "call-1",
						toolCallName: "tool_result",
					}),
					aguiFrame({ type: "TOOL_CALL_END", toolCallId: "call-1" }),
					aguiFrame({
						type: "TOOL_CALL_RESULT",
						messageId: "m1",
						toolCallId: "call-1",
						content: '{"type":"seller_candidates","candidates":[]}',
					}),
				]),
			),
		);
		const onNamedEvent = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onNamedEvent).toHaveBeenCalledWith(
			"tool_result",
			'{"type":"seller_candidates","candidates":[]}',
		);
	});

	it("falls back to a generic tool_result name when TOOL_CALL_RESULT arrives without a matching TOOL_CALL_START", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "TOOL_CALL_RESULT",
						messageId: "m1",
						toolCallId: "unknown-call",
						content: "{}",
					}),
				]),
			),
		);
		const onNamedEvent = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onNamedEvent).toHaveBeenCalledWith("tool_result", "{}");
	});

	it("does not leak toolCallName tracking across separate streamAiChat calls", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValueOnce(
					sseResponse([
						aguiFrame({
							type: "TOOL_CALL_START",
							toolCallId: "call-1",
							toolCallName: "step",
						}),
					]),
				)
				.mockResolvedValueOnce(
					sseResponse([
						aguiFrame({
							type: "TOOL_CALL_RESULT",
							messageId: "m1",
							toolCallId: "call-1",
							content: "{}",
						}),
					]),
				),
		);
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{
				onText: vi.fn(),
				onNamedEvent: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		);
		const onNamedEvent = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent, onDone: vi.fn(), onError: vi.fn() },
		);
		// A fresh call must not remember the previous call's toolCallId→name
		// mapping — falls back to the generic name, not "step" from call 1.
		expect(onNamedEvent).toHaveBeenCalledWith("tool_result", "{}");
	});

	it("passes RUN_FINISHED's result through onDone, re-serialized to a string", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "t1",
						runId: "r1",
						result: { sessionId: "s1" },
					}),
				]),
			),
		);
		const onDone = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone, onError: vi.fn() },
		);
		expect(onDone).toHaveBeenCalledWith('{"sessionId":"s1"}', {
			type: "success",
		});
	});

	it("passes an empty object through onDone when RUN_FINISHED carries no result", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([
						aguiFrame({ type: "RUN_FINISHED", threadId: "t1", runId: "r1" }),
					]),
				),
		);
		const onDone = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone, onError: vi.fn() },
		);
		// 沒有 outcome 欄位＝這個 feature 上線前的舊 frame，向後相容視為 legacy
		// success，不當成待處理 interrupt。
		expect(onDone).toHaveBeenCalledWith("{}", { type: "success" });
	});

	it("RUN_FINISHED.outcome.type=success 原樣收斂成 success 傳給 onDone", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "t1",
						runId: "r1",
						result: {},
						outcome: { type: "success" },
					}),
				]),
			),
		);
		const onDone = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone, onError: vi.fn() },
		);
		expect(onDone).toHaveBeenCalledWith("{}", { type: "success" });
	});

	// 官方 @ag-ui/core schema：outcome.type==="interrupt" 的 interrupts 是陣列
	// （v1 一次 run 最多一個未結案 interrupt，但 wire 形狀仍是陣列）。這裡驗證
	// stream-client 把它收斂成呼叫端要的扁平 { interruptId, reason }。
	it("RUN_FINISHED.outcome.type=interrupt 收斂成扁平 interruptId／reason 傳給 onDone", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "t1",
						runId: "r1",
						result: {},
						outcome: {
							type: "interrupt",
							interrupts: [{ id: "int-1", reason: "CLARIFICATION" }],
						},
					}),
				]),
			),
		);
		const onDone = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone, onError: vi.fn() },
		);
		expect(onDone).toHaveBeenCalledWith("{}", {
			type: "interrupt",
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});

	// 官方 schema 對 interrupts 有 min(1) 限制——一個「type=interrupt 卻沒有任何
	// interrupt」的 frame 不合法，會在 EventSchemas.safeParse 這一關就被整個
	// frame 拒絕（conformance anchor），走 onError，根本到不了 RUN_FINISHED 的
	// switch case。這個 case 因此不是靠 toRunOutcome 的向後相容 fallback
	// 擋下，而是更早的一層（schema 本身）已經擋掉——這裡直接驗證那個事實，
	// 不假裝 onDone 曾經看過這個 frame。
	it("outcome.interrupts 為空陣列不合 schema minimum，整個 frame 被 conformance 檢查擋下，走 onError 不進 onDone", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "t1",
						runId: "r1",
						result: {},
						outcome: { type: "interrupt", interrupts: [] },
					}),
				]),
			),
		);
		const onDone = vi.fn();
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone, onError },
		);
		expect(onDone).not.toHaveBeenCalled();
		expect(onError).toHaveBeenCalled();
	});

	it("dispatches RUN_ERROR via onError", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([aguiFrame({ type: "RUN_ERROR", message: "boom" })]),
				),
		);
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onError).toHaveBeenCalledWith("boom", undefined, undefined);
	});

	it("forwards RUN_ERROR's code as a number, converted from the wire's stringified form", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([
						aguiFrame({ type: "RUN_ERROR", message: "conflict", code: "409" }),
					]),
				),
		);
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onError).toHaveBeenCalledWith("conflict", 409, undefined);
	});

	// 官方 @ag-ui/core@0.0.58 的 RunErrorEventSchema 只宣告 type/message/code/
	// usage，沒有 reason——zod 的預設 .object() 行為會把 safeParse 後的
	// record 上任何 schema 沒宣告的欄位剝掉。這裡驗證：即使 reason 因此在
	// EventSchemas.safeParse 後消失，它仍然出現在 raw wire JSON 上，dispatcher
	// 必須從那份原始 JSON（而不是驗證後的 record）另外把它讀出來，轉成
	// onError 的第三個參數。
	it("戴著 schema 沒宣告的 reason 的 RUN_ERROR frame 仍通過官方 schema 驗證，且 reason 原樣交給 onError 第三個參數", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_ERROR",
						message: "request is parked on an open interrupt",
						code: "409",
						reason: "APPROVAL_PENDING",
					}),
				]),
			),
		);
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onError).toHaveBeenCalledWith(
			"request is parked on an open interrupt",
			409,
			"APPROVAL_PENDING",
		);
	});

	it("RUN_ERROR frame 沒有 reason 時，onError 第三個參數是 undefined", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([aguiFrame({ type: "RUN_ERROR", message: "boom" })]),
				),
		);
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onError).toHaveBeenCalledWith("boom", undefined, undefined);
	});

	// AG-UI schema 要求 RUN_ERROR 一定帶 message 欄位（可以是空字串），所以
	// 「完全沒有 message」的 frame 現在會被 EventSchemas 擋在 switch 之前，走
	// 上面那支 conformance 測試的路徑，不會進到這裡的 fallback；這支改成測
	// message 為空字串時、switch case 內仍會 fallback 成通用訊息。
	it("falls back to a generic message when RUN_ERROR's message is empty", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([aguiFrame({ type: "RUN_ERROR", message: "" })]),
				),
		);
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText: vi.fn(), onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onError).toHaveBeenCalledWith("AI 助手出錯", undefined, undefined);
	});

	it("不符合 AG-UI schema 的 frame 走 onError，不靜默丟棄", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					// type 合法但缺 schema 要求的 delta 欄位
					'data: {"type":"TEXT_MESSAGE_CONTENT"}\n\n',
				]),
			),
		);
		const onText = vi.fn();
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText, onNamedEvent: vi.fn(), onDone: vi.fn(), onError },
		);
		expect(onText).not.toHaveBeenCalled();
		expect(onError).toHaveBeenCalled();
	});

	it("ignores event types it has no handler for (e.g. RUN_STARTED, STATE_SNAPSHOT)", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([
						aguiFrame({ type: "RUN_STARTED", threadId: "t1", runId: "r1" }),
						aguiFrame({ type: "STATE_SNAPSHOT", snapshot: { foo: "bar" } }),
						aguiFrame({ type: "TEXT_MESSAGE_START", messageId: "m1" }),
						aguiFrame({ type: "TEXT_MESSAGE_END", messageId: "m1" }),
					]),
				),
		);
		const onText = vi.fn();
		const onNamedEvent = vi.fn();
		const onDone = vi.fn();
		const onError = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText, onNamedEvent, onDone, onError },
		);
		expect(onText).not.toHaveBeenCalled();
		expect(onNamedEvent).not.toHaveBeenCalled();
		expect(onDone).not.toHaveBeenCalled();
		expect(onError).not.toHaveBeenCalled();
	});

	it("handles a multi-chunk stream split mid-frame", async () => {
		const wholeFrame = aguiFrame({
			type: "TEXT_MESSAGE_CONTENT",
			messageId: "m1",
			delta: "partial",
		});
		const splitAt = wholeFrame.indexOf('"partial"');
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse([
						wholeFrame.slice(0, splitAt),
						wholeFrame.slice(splitAt),
					]),
				),
		);
		const onText = vi.fn();
		await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{ onText, onNamedEvent: vi.fn(), onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onText).toHaveBeenCalledWith("partial");
	});

	it("rejects on non-2xx response", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(new Response("server error", { status: 500 })),
		);
		await expect(
			streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{
					onText: vi.fn(),
					onNamedEvent: vi.fn(),
					onDone: vi.fn(),
					onError: vi.fn(),
				},
			),
		).rejects.toThrow("server error");
	});

	// I2: non-2xx bodies are gateway Envelope JSON ({"code":...,"message":...,
	// "traceId":...}), not plain text — the thrown Error must surface a human
	// message, not the raw JSON blob, or it renders verbatim in the chat UI.
	it("extracts messageZh from a gateway error envelope instead of raw JSON", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				new Response(
					JSON.stringify({
						code: 404,
						message: "session not found",
						messageZh: "找不到這個對話",
						traceId: "abc123",
					}),
					{ status: 404 },
				),
			),
		);
		await expect(
			streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{
					onText: vi.fn(),
					onNamedEvent: vi.fn(),
					onDone: vi.fn(),
					onError: vi.fn(),
				},
			),
		).rejects.toThrow("找不到這個對話");
	});

	// procurement 等端點的業務錯誤是 {code,reason,message} 三個獨立欄位
	// （見 lib/api/client.ts 的 ApiClientError.reason）；串流開場前的 409
	// 一樣可能帶這個 envelope，AiStreamHttpError 必須把它透傳出來，不只留
	// message，否則 turn-stream.ts 沒有真值可以判斷 APPROVAL_PENDING。
	it("populates AiStreamHttpError.reason from a non-2xx error envelope's reason field", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				new Response(
					JSON.stringify({
						code: 409,
						reason: "APPROVAL_PENDING",
						message: "request is parked on an open interrupt",
					}),
					{ status: 409 },
				),
			),
		);
		const err = await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{
				onText: vi.fn(),
				onNamedEvent: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AiStreamHttpError);
		expect((err as AiStreamHttpError).reason).toBe("APPROVAL_PENDING");
		expect((err as AiStreamHttpError).message).toBe(
			"request is parked on an open interrupt",
		);
	});

	it("defaults AiStreamHttpError.reason to an empty string when the error envelope has no reason field", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				new Response(JSON.stringify({ code: 409, message: "conflict" }), {
					status: 409,
				}),
			),
		);
		const err = await streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{
				onText: vi.fn(),
				onNamedEvent: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AiStreamHttpError);
		expect((err as AiStreamHttpError).reason).toBe("");
	});

	it("falls back to message when an error envelope has no messageZh", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					new Response(
						JSON.stringify({ code: 401, message: "session expired" }),
						{ status: 401 },
					),
				),
		);
		await expect(
			streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{
					onText: vi.fn(),
					onNamedEvent: vi.fn(),
					onDone: vi.fn(),
					onError: vi.fn(),
				},
			),
		).rejects.toThrow("session expired");
	});

	it("falls back to the raw text when the non-2xx body isn't JSON", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(new Response("upstream timeout", { status: 504 })),
		);
		await expect(
			streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{
					onText: vi.fn(),
					onNamedEvent: vi.fn(),
					onDone: vi.fn(),
					onError: vi.fn(),
				},
			),
		).rejects.toThrow("upstream timeout");
	});

	// I3: a one-shot POST chat stream has no heartbeat/reconnect (unlike
	// lib/sse/client.ts) — a stall must still resolve into a clear timeout
	// error instead of hanging the awaiting caller forever.
	it("rejects with a timeout error when the stream stalls past the stall window", async () => {
		vi.useFakeTimers();
		try {
			const encoder = new TextEncoder();
			const stream = new ReadableStream<Uint8Array>({
				start(controller) {
					// One chunk, then silence forever — no close(), no further enqueue.
					controller.enqueue(
						encoder.encode(
							aguiFrame({
								type: "TEXT_MESSAGE_CONTENT",
								messageId: "m1",
								delta: "hi",
							}),
						),
					);
				},
			});
			vi.stubGlobal(
				"fetch",
				vi.fn().mockResolvedValue(new Response(stream, { status: 200 })),
			);

			const onText = vi.fn();
			const promise = streamAiChat(
				{ endpoint: "/app/test/endpoint", body: {} },
				{ onText, onNamedEvent: vi.fn(), onDone: vi.fn(), onError: vi.fn() },
			);
			// Attach the rejection assertion BEFORE advancing the fake clock —
			// matching lib/auth/__tests__/gsi.unit.test.ts's pattern — so the
			// handler is wired up before the timeout actually fires; attaching
			// it afterward makes Node briefly see `promise` as an unhandled
			// rejection (PromiseRejectionHandledWarning, a fake-timer-only
			// false positive, not a real bug in the implementation).
			const rejection = expect(promise).rejects.toThrow(AiStreamTimeoutError);

			// Let the fetch mock + first (already-buffered) chunk resolve so the
			// second reader.read() call — the one that stalls — actually starts
			// and registers its timeout before we fast-forward the clock.
			await vi.advanceTimersByTimeAsync(0);
			expect(onText).toHaveBeenCalledWith("hi");

			await vi.advanceTimersByTimeAsync(30_000);
			await rejection;
		} finally {
			vi.useRealTimers();
		}
	});
});

it.each([
	false,
	true,
])("revoked Clerk session uses stream HTTP 401 (refresh=%s)", async (refresh) => {
	const source = vi.fn().mockRejectedValue({ status: 401 });
	if (refresh) source.mockResolvedValueOnce("stale");
	const dispatchEvent = vi.fn();
	vi.stubGlobal("window", {
		dispatchEvent,
		Clerk: { loaded: true, session: { getToken: source } },
	});
	const authorization: (string | null)[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (_url, init) => {
			authorization.push(new Headers(init.headers).get("Authorization"));
			return new Response('{"message":"expired"}', { status: 401 });
		}),
	);
	await expect(
		streamAiChat(
			{ endpoint: "/app/test/endpoint", body: {} },
			{
				onText: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
				onNamedEvent: vi.fn(),
			},
		),
	).rejects.toMatchObject({ name: "AiStreamHttpError", status: 401 });
	expect(authorization).toEqual(refresh ? ["Bearer stale", null] : [null]);
	expect(dispatchEvent).toHaveBeenCalledOnce();
});
