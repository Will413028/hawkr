import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	RunStatus,
	type WatchRunResponse,
	WatchRunResponseSchema,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	MAX_ATTEMPTS,
	type WatchOpen,
	type WatchOutcome,
	type WatchRunRequest,
	watchRun,
} from "./watch-run";

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

function textEvent(cursor: string): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: { case: "text", value: { text: cursor } },
	});
}

function finishedEvent(cursor: string): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: { case: "finished", value: { status: RunStatus.SUCCEEDED } },
	});
}

function cursorError(reason: string) {
	return new ConnectError("x", Code.FailedPrecondition, undefined, [
		{
			desc: ErrorInfoSchema,
			value: create(ErrorInfoSchema, { domain: "hawkr", reason }),
		},
	]);
}

type Step =
	| { at: number; event: WatchRunResponse }
	| { at: number; end: true }
	| { at: number; fail: ConnectError };

/** 一條連線的腳本：`at` 是連線開啟後的毫秒；到了 `end` 就正常結束。 */
function connection(steps: Step[]) {
	return async function* (
		signal: AbortSignal,
	): AsyncGenerator<WatchRunResponse> {
		let elapsed = 0;
		for (const step of steps) {
			await sleep(step.at - elapsed);
			elapsed = step.at;
			if (signal.aborted) return;
			if ("event" in step) yield step.event;
			else if ("fail" in step) throw step.fail;
			else return;
		}
	};
}

function scripted(
	connections: Array<(signal: AbortSignal) => AsyncGenerator<WatchRunResponse>>,
) {
	const requests: WatchRunRequest[] = [];
	const open: WatchOpen = (request, signal) => {
		requests.push({ ...request });
		const make = connections[requests.length - 1];
		if (!make) throw new Error("unexpected connection");
		return make(signal);
	};
	return { open, requests };
}

async function run(
	open: WatchOpen,
	advanceMs: number,
	delivered: WatchRunResponse[] = [],
): Promise<{ outcome: WatchOutcome; delivered: WatchRunResponse[] }> {
	const controller = new AbortController();
	const promise = watchRun({
		open,
		runId: "run-1",
		signal: controller.signal,
		onEvent: (e) => delivered.push(e),
		deps: { random: () => 0.999 },
	});
	await vi.advanceTimersByTimeAsync(advanceMs);
	return { outcome: await promise, delivered };
}

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

describe("watchRun", () => {
	it("150 秒的一輪被 100 秒的串流切成兩段：第二條帶最後的 cursor，事件各收一次、依序，finished 一次", async () => {
		const first = connection([
			{ at: 20_000, event: textEvent("c1") },
			{ at: 40_000, event: textEvent("c2") },
			{ at: 60_000, event: textEvent("c3") },
			{ at: 80_000, event: textEvent("c4") },
			{ at: 100_000, end: true },
		]);
		const second = connection([
			{ at: 20_000, event: textEvent("c5") },
			{ at: 40_000, event: textEvent("c6") },
			{ at: 50_000, event: finishedEvent("c7") },
		]);
		const { open, requests } = scripted([first, second]);
		const { outcome, delivered } = await run(open, 150_000);

		expect(outcome).toEqual({ kind: "finished" });
		expect(requests).toEqual([
			{ runId: "run-1", cursor: "" },
			{ runId: "run-1", cursor: "c4" },
		]);
		expect(delivered.map((e) => e.cursor)).toEqual([
			"c1",
			"c2",
			"c3",
			"c4",
			"c5",
			"c6",
			"c7",
		]);
		expect(delivered.filter((e) => e.event.case === "finished")).toHaveLength(
			1,
		);
	});

	it("重連後伺服器重播已送達的事件：以 cursor 去重，只交一次", async () => {
		const first = connection([
			{ at: 10_000, event: textEvent("c1") },
			{ at: 20_000, event: textEvent("c2") },
			{ at: 100_000, end: true },
		]);
		const second = connection([
			{ at: 1_000, event: textEvent("c2") },
			{ at: 2_000, event: textEvent("c1") },
			{ at: 3_000, event: textEvent("c3") },
			{ at: 4_000, event: finishedEvent("c4") },
		]);
		const { open } = scripted([first, second]);
		const { outcome, delivered } = await run(open, 110_000);

		expect(outcome).toEqual({ kind: "finished" });
		expect(delivered.map((e) => e.cursor)).toEqual(["c1", "c2", "c3", "c4"]);
	});

	it("沒有 finished 就結束的串流是被截斷：以最後的 cursor 立刻重連", async () => {
		const first = connection([
			{ at: 1_000, event: textEvent("c1") },
			{ at: 2_000, end: true },
		]);
		const second = connection([
			{ at: 1_000, event: textEvent("c2") },
			{ at: 2_000, event: finishedEvent("c3") },
		]);
		const { open, requests } = scripted([first, second]);
		const { outcome } = await run(open, 10_000);

		expect(outcome).toEqual({ kind: "finished" });
		expect(requests.map((r) => r.cursor)).toEqual(["", "c1"]);
	});

	it("暫時性錯誤（unavailable）backoff 後以同一個 cursor 重連", async () => {
		const first = connection([
			{ at: 1_000, event: textEvent("c1") },
			{ at: 2_000, fail: new ConnectError("down", Code.Unavailable) },
		]);
		const second = connection([{ at: 1_000, event: finishedEvent("c2") }]);
		const { open, requests } = scripted([first, second]);
		const { outcome, delivered } = await run(open, 30_000);

		expect(outcome).toEqual({ kind: "finished" });
		expect(requests.map((r) => r.cursor)).toEqual(["", "c1"]);
		expect(delivered.map((e) => e.cursor)).toEqual(["c1", "c2"]);
	});

	it("REPLAY_CURSOR_EXPIRED 與 REPLAY_CURSOR_AHEAD 回 resync，不重連", async () => {
		for (const reason of ["REPLAY_CURSOR_EXPIRED", "REPLAY_CURSOR_AHEAD"]) {
			const first = connection([
				{ at: 1_000, event: textEvent("c1") },
				{ at: 2_000, end: true },
			]);
			const second = connection([{ at: 0, fail: cursorError(reason) }]);
			const { open, requests } = scripted([first, second]);
			const { outcome } = await run(open, 10_000);

			expect(outcome).toEqual({ kind: "resync", reason });
			expect(requests).toHaveLength(2);
		}
	});

	it("unauthenticated、not_found、permission_denied 回 failed，不重連", async () => {
		for (const code of [
			Code.Unauthenticated,
			Code.NotFound,
			Code.PermissionDenied,
		]) {
			const { open, requests } = scripted([
				connection([{ at: 0, fail: new ConnectError("no", code) }]),
			]);
			const { outcome } = await run(open, 1_000);

			expect(outcome.kind).toBe("failed");
			expect(requests).toHaveLength(1);
		}
	});

	it("一直沒有進展的失敗在 MAX_ATTEMPTS 次後回 exhausted", async () => {
		const failing = () =>
			connection([{ at: 0, fail: new ConnectError("down", Code.Unavailable) }]);
		const { open, requests } = scripted(
			Array.from({ length: MAX_ATTEMPTS }, failing),
		);
		const { outcome } = await run(open, 120_000);

		expect(outcome.kind).toBe("exhausted");
		expect(requests).toHaveLength(MAX_ATTEMPTS);
	});

	it("中止時回 aborted，之後不再開連線", async () => {
		const controller = new AbortController();
		const { open, requests } = scripted([
			connection([{ at: 50_000, event: textEvent("c1") }]),
		]);
		const promise = watchRun({
			open,
			runId: "run-1",
			signal: controller.signal,
			onEvent: () => {},
		});
		await vi.advanceTimersByTimeAsync(1_000);
		controller.abort();
		await vi.advanceTimersByTimeAsync(60_000);

		expect(await promise).toEqual({ kind: "aborted" });
		expect(requests).toHaveLength(1);
	});
});
