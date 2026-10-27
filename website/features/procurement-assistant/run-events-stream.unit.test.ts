import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AgentActivityEvent } from "./activity";
import {
	openRunActivityStream,
	type RunArtifactRef,
} from "./run-events-stream";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			buildUrl: vi.fn(
				(_mode: string, path: string) => `https://gw.test/api/v1/app${path}`,
			),
		},
	};
});

const FIXTURE = readFileSync(
	join(
		fileURLToPath(new URL("../../", import.meta.url)),
		"lib/ai-chat/__tests__/agui-frames/replay-candidate-search-rail.sse.txt",
	),
	"utf8",
);

function sseResponse(body: string, status = 200): Response {
	const encoder = new TextEncoder();
	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			controller.enqueue(encoder.encode(body));
			controller.close();
		},
	});
	return new Response(stream, { status });
}

/** 讓 SseClient 的 fetch → readStream → dispatch 這串 microtask 跑完。 */
async function flush() {
	for (let i = 0; i < 20; i += 1) await Promise.resolve();
}

afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

describe("openRunActivityStream", () => {
	it("業務失敗的 RUN_ERROR 走 onRunFailed，不走 onUnavailable", async () => {
		// 後端把 RUN_FAILED 投影成原生 RUN_ERROR，機器碼放 reason（code 被本 repo
		// 保留給 HTTP 數字碼）。這是 run 自己失敗了，不是串流壞了——重連只會 replay
		// 到同一個終態，所以要收線，但呈現的是「搜尋失敗」而不是「連線中斷」。
		const body =
			'id: 5\ndata: {"type":"RUN_ERROR","message":"全部通道失敗","reason":"LANE_ALL_FAILED"}\n\n';
		const fetchMock = vi.fn().mockResolvedValue(sseResponse(body));
		vi.stubGlobal("fetch", fetchMock);
		const failures: { reason: string; message: string }[] = [];
		const unavailable: string[] = [];

		const stream = openRunActivityStream("1234567890123456800", {
			onActivity: () => {},
			onRunFailed: (f) => failures.push(f),
			onTerminal: () => {},
			onUnavailable: (reason) => unavailable.push(reason),
		});
		await flush();
		stream.close();

		expect(failures).toEqual([
			{ reason: "LANE_ALL_FAILED", message: "全部通道失敗" },
		]);
		expect(unavailable).toEqual([]);
	});

	it("傳輸層的 RUN_ERROR（帶數字 code）仍走 onUnavailable", async () => {
		// gateway 對 410 游標過期／503 等傳輸失敗送的也是 RUN_ERROR，但 code 帶的是
		// HTTP 字串碼。這種是「串流壞了」，run 本身可能還在跑——判別靠有沒有 code。
		const body =
			'id: 6\ndata: {"type":"RUN_ERROR","message":"游標已過期","code":"410","reason":"REPLAY_CURSOR_EXPIRED"}\n\n';
		const fetchMock = vi.fn().mockResolvedValue(sseResponse(body));
		vi.stubGlobal("fetch", fetchMock);
		const failures: { reason: string; message: string }[] = [];
		const unavailable: string[] = [];

		const stream = openRunActivityStream("1234567890123456800", {
			onActivity: () => {},
			onRunFailed: (f) => failures.push(f),
			onTerminal: () => {},
			onUnavailable: (reason) => unavailable.push(reason),
		});
		await flush();
		stream.close();

		expect(failures).toEqual([]);
		expect(unavailable).toHaveLength(1);
	});

	it("把 CUSTOM artifact frame 交成 onArtifact 的三個參照", async () => {
		// 後端 ARTIFACT_UPSERTED 的 public payload 就是這三欄（repository/run.go 的
		// artifactUpsertedPayload），gateway 原樣投影成 CUSTOM name="artifact"。
		// ledger 事件是通知不是內容——內容之後由 artifact 讀取端取，這裡只認參照。
		const body =
			'id: 7\ndata: {"type":"CUSTOM","name":"artifact","value":{"artifactId":"7502316611857022976","type":"CANDIDATE_RESULTS","version":1}}\n\n';
		const fetchMock = vi.fn().mockResolvedValue(sseResponse(body));
		vi.stubGlobal("fetch", fetchMock);
		const refs: RunArtifactRef[] = [];

		const stream = openRunActivityStream("1234567890123456800", {
			onActivity: () => {},
			onArtifact: (ref) => refs.push(ref),
			onTerminal: () => {},
			onUnavailable: () => {},
		});
		await flush();
		stream.close();

		expect(refs).toEqual([
			{
				artifactId: "7502316611857022976",
				type: "CANDIDATE_RESULTS",
				version: 1,
			},
		]);
	});

	it("artifact frame 形狀不合契約時只丟這個 frame，不終結訂閱", async () => {
		// fail-closed 同 activity：缺 artifactId 就不是一個可用的參照，寧可不交出
		// 也不要讓消費端拿到一個指不到東西的 id。而且丟掉一個 frame 不等於整條
		// replay 壞了——真正該停止訂閱的只有 in-band RUN_ERROR。
		const body =
			'id: 7\ndata: {"type":"CUSTOM","name":"artifact","value":{"type":"CANDIDATE_RESULTS","version":1}}\n\n' +
			'id: 8\ndata: {"type":"CUSTOM","name":"artifact","value":{"artifactId":"900","type":"ERROR_RECOVERY","version":2}}\n\n';
		const fetchMock = vi.fn().mockResolvedValue(sseResponse(body));
		vi.stubGlobal("fetch", fetchMock);
		const refs: RunArtifactRef[] = [];
		const unavailable: string[] = [];

		const stream = openRunActivityStream("1234567890123456800", {
			onActivity: () => {},
			onArtifact: (ref) => refs.push(ref),
			onTerminal: () => {},
			onUnavailable: (reason) => unavailable.push(reason),
		});
		await flush();
		stream.close();

		expect(refs).toEqual([
			{ artifactId: "900", type: "ERROR_RECOVERY", version: 2 },
		]);
		expect(unavailable).toEqual([]);
	});

	it("打的是 GET /runs/{runId}/events，並把三個 activity frame 依序交出來", async () => {
		const fetchMock = vi.fn().mockResolvedValue(sseResponse(FIXTURE));
		vi.stubGlobal("fetch", fetchMock);
		const rows: AgentActivityEvent[] = [];

		const stream = openRunActivityStream("1234567890123456800", {
			onActivity: (event) => rows.push(event),
			onTerminal: () => {},
			onUnavailable: () => {},
		});
		await flush();
		stream.close();

		const [url, init] = fetchMock.mock.calls[0];
		expect(url).toBe(
			"https://gw.test/api/v1/app/buyers/me/procurement/runs/1234567890123456800/events",
		);
		expect(init.method).toBe("GET");
		expect(rows.map((r) => r.code)).toEqual([
			"SEARCH_CANDIDATES",
			"SEARCH_PLATFORM",
			"SUMMARIZE",
		]);
		expect(rows[1].parentRowId).toBe("9001");
		expect(rows[1].count).toBe("12");
		expect(rows[2].status).toBe("PENDING");
	});

	it("keepalive 的註解 frame（`: ping`）不是事件，不得終止訂閱", async () => {
		// gateway 的 ai_run_events_stream.go 每 20 秒無條件寫一次 `: ping`，
		// 所以任何超過 20 秒的搜尋都會遇到它——也就是這個功能存在的理由。
		// 註解 frame 沒有 data: 行，依 SSE 規格本來就不該分派成事件；分派成
		// 空字串會讓 EventSchemas.safeParse("") 失敗，訂閱在 20 秒就死掉。
		// fixture 的 ping 夾在第二、三個 activity frame 中間，所以「第三列還
		// 收得到」就是這條線活著的證據。
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(sseResponse(FIXTURE)));
		const rows: AgentActivityEvent[] = [];
		const onUnavailable = vi.fn();

		const stream = openRunActivityStream("run1", {
			onActivity: (event) => rows.push(event),
			onTerminal: () => {},
			onUnavailable,
		});
		await flush();
		stream.close();

		expect(onUnavailable).not.toHaveBeenCalled();
		expect(rows.map((r) => r.code)).toEqual([
			"SEARCH_CANDIDATES",
			"SEARCH_PLATFORM",
			"SUMMARIZE",
		]);
	});

	it("合乎 schema 的 RUN_FINISHED 走 onTerminal——伺服器終態後不會自己關，連線得由前端收掉", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse(
						'id: 9\ndata: {"type":"RUN_FINISHED","threadId":"t1","runId":"r1"}\n\n',
					),
				),
		);
		const onTerminal = vi.fn();

		const stream = openRunActivityStream("run1", {
			onActivity: () => {},
			onTerminal,
			onUnavailable: () => {},
		});
		await flush();
		stream.close();

		expect(onTerminal).toHaveBeenCalledTimes(1);
	});

	it("schema 擋下來的 frame 只丟自己，訂閱不死——後面的 activity 仍要送達", async () => {
		// 這裡用的就是伺服器**今天實際送的** RUN_FINISHED：gateway 的 replay
		// frame 是 `{...data_json, type, runId}`，而候選搜尋 run 的
		// PublicPayloadJSON 是字面的 `{}`，所以 wire 上沒有 threadId，官方
		// @ag-ui/core schema 必填 → 整個 frame 被 conformance anchor 擋下。
		// 後端補 threadId 之前（設計文件 §8 的 backend follow-up），onTerminal
		// 在線上是走不到的；重點是這個被擋下的 frame 不得殺掉整條訂閱。
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse(
						[
							'id: 9\ndata: {"type":"RUN_FINISHED","runId":"r1"}\n\n',
							'id: 10\ndata: {"type":"CUSTOM","name":"activity","value":{"rowId":"9004","code":"SUMMARIZE","label":"整理成給你的建議","status":"SUCCEEDED","text":""}}\n\n',
						].join(""),
					),
				),
		);
		const rows: AgentActivityEvent[] = [];
		const onTerminal = vi.fn();
		const onUnavailable = vi.fn();

		const stream = openRunActivityStream("run1", {
			onActivity: (event) => rows.push(event),
			onTerminal,
			onUnavailable,
		});
		await flush();
		stream.close();

		expect(onUnavailable).not.toHaveBeenCalled();
		expect(onTerminal).not.toHaveBeenCalled();
		expect(rows.map((r) => r.code)).toEqual(["SUMMARIZE"]);
	});

	it("in-band RUN_ERROR（410 游標過期）走 onUnavailable 並收掉訂閱", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse(
						'data: {"type":"RUN_ERROR","message":"游標已過保留視窗","code":"410","reason":"REPLAY_CURSOR_EXPIRED"}\n\n',
					),
				),
		);
		const onUnavailable = vi.fn();

		openRunActivityStream("run1", {
			onActivity: () => {},
			onTerminal: () => {},
			onUnavailable,
		});
		await flush();

		expect(onUnavailable).toHaveBeenCalledTimes(1);
		expect(onUnavailable.mock.calls[0][0]).toContain("REPLAY_CURSOR_EXPIRED");
	});

	it("503 CAPABILITY_UNAVAILABLE 只打一次就放棄，不重試轟 gateway", async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response('{"code":503,"reason":"CAPABILITY_UNAVAILABLE"}', {
				status: 503,
			}),
		);
		vi.stubGlobal("fetch", fetchMock);
		const onUnavailable = vi.fn();

		openRunActivityStream("run1", {
			onActivity: () => {},
			onTerminal: () => {},
			onUnavailable,
		});
		await flush();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(onUnavailable).toHaveBeenCalledTimes(1);
	});

	it("close() 之後遲到的事件不再往外送", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(sseResponse(FIXTURE)));
		const onActivity = vi.fn();

		const stream = openRunActivityStream("run1", {
			onActivity,
			onTerminal: () => {},
			onUnavailable: () => {},
		});
		stream.close();
		await flush();

		expect(onActivity).not.toHaveBeenCalled();
	});
});
