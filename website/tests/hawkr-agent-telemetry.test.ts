import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	buildFirstStepLatencyParams,
	buildTerminalRenderedParams,
	EMPTY_RAIL_TELEMETRY_STATE,
	HAWKR_AGENT_METRIC_MAPPING,
	type HawkrAgentTelemetryDispatch,
	observeRailCommit,
	scheduleTerminalRailMeasurement,
	trackHawkrAgentEntryPromptSubmit,
	trackHawkrAgentEntryView,
	trackHawkrAgentFirstStepLatency,
} from "../features/procurement-assistant/telemetry";

const root = fileURLToPath(new URL("..", import.meta.url));
const workspace = readFileSync(
	path.join(root, "features/procurement-assistant/AssistantWorkspace.tsx"),
	"utf8",
);

test("entry_prompt_submit_rate 由兩個 event count 下游計算，不發送假 rate", () => {
	assert.deepEqual(HAWKR_AGENT_METRIC_MAPPING.entry_prompt_submit_rate, {
		kind: "derived-rate",
		numeratorEvent: "hawkr_agent_entry_prompt_submit",
		denominatorEvent: "hawkr_agent_entry_view",
	});

	const calls: Array<{
		eventName: string;
		params: Readonly<Record<string, number | boolean>>;
	}> = [];
	const dispatch: HawkrAgentTelemetryDispatch = (eventName, params) => {
		calls.push({ eventName, params });
	};
	trackHawkrAgentEntryView(dispatch);
	trackHawkrAgentEntryPromptSubmit(dispatch);

	assert.deepEqual(calls, [
		{ eventName: "hawkr_agent_entry_view", params: { sample_count: 1 } },
		{
			eventName: "hawkr_agent_entry_prompt_submit",
			params: { sample_count: 1 },
		},
	]);
	assert.ok(calls.every(({ eventName }) => !eventName.endsWith("_rate")));
});

test("同一個 Entry episode 最多只記一次 view 與 submit", () => {
	assert.match(
		workspace,
		/if \(entryViewTrackedRef\.current\) return;\s*entryViewTrackedRef\.current = true;\s*trackHawkrAgentEntryView\(\);/,
	);
	assert.match(
		workspace,
		/if \(showEntry && !entrySubmitTrackedRef\.current\) \{\s*entrySubmitTrackedRef\.current = true;\s*trackHawkrAgentEntryPromptSubmit\(\);\s*\}/,
	);
	assert.match(
		workspace,
		/if \(!entryVisible\) \{\s*entryViewTrackedRef\.current = false;\s*entrySubmitTrackedRef\.current = false;/,
	);
});

test("analytics transport 失敗不得阻斷 Entry submit", () => {
	const unavailable: HawkrAgentTelemetryDispatch = () => {
		throw new Error("analytics unavailable");
	};
	assert.doesNotThrow(() => trackHawkrAgentEntryView(unavailable));
	assert.doesNotThrow(() => trackHawkrAgentEntryPromptSubmit(unavailable));
});

test("first_step_latency 從 command invocation 到第一個 allowlisted public step，只送數值", () => {
	assert.deepEqual(HAWKR_AGENT_METRIC_MAPPING.first_step_latency, {
		kind: "event-parameter",
		event: "hawkr_agent_first_public_step_observed",
		parameter: "first_step_latency_ms",
		definition:
			"command-invocation-to-first-allowlisted-public-step-arrival-ms",
	});
	assert.deepEqual(
		buildFirstStepLatencyParams({
			commandInvokedAt: 10,
			firstStepObservedAt: 42.5,
		}),
		{ first_step_latency_ms: 32.5, sample_count: 1 },
	);

	const calls: Array<{
		eventName: string;
		params: Readonly<Record<string, number | boolean>>;
	}> = [];
	trackHawkrAgentFirstStepLatency(
		{ commandInvokedAt: 10, firstStepObservedAt: 42.5 },
		(eventName, params) => calls.push({ eventName, params }),
	);
	assert.deepEqual(calls, [
		{
			eventName: "hawkr_agent_first_public_step_observed",
			params: { first_step_latency_ms: 32.5, sample_count: 1 },
		},
	]);
	assert.ok(
		Object.values(calls[0].params).every(
			(value) => typeof value === "number" || typeof value === "boolean",
		),
	);
	assert.ok(
		Object.keys(calls[0].params).every(
			(key) => !/(?:prompt|code|label|raw|payload|(?:^|_)id(?:_|$))/i.test(key),
		),
	);

	for (const sample of [
		{ commandInvokedAt: 20, firstStepObservedAt: 10 },
		{ commandInvokedAt: -1, firstStepObservedAt: 10 },
		{ commandInvokedAt: Number.NaN, firstStepObservedAt: 10 },
	]) {
		assert.equal(buildFirstStepLatencyParams(sample), null);
	}
	const unavailable: HawkrAgentTelemetryDispatch = () => {
		throw new Error("analytics unavailable");
	};
	assert.doesNotThrow(() =>
		trackHawkrAgentFirstStepLatency(
			{ commandInvokedAt: 10, firstStepObservedAt: 11 },
			unavailable,
		),
	);
});

test("rail reducer 只接受 authoritative terminal，以 operation epoch 隔離 exact retry，reset 不冒充 terminal", () => {
	const first = observeRailCommit(EMPTY_RAIL_TELEMETRY_STATE, {
		pending: true,
		visibleStepCount: 1,
		terminalObservedAt: null,
		terminalOperationGeneration: null,
		duplicateStepSuppressedCount: 0,
	});
	const second = observeRailCommit(first.state, {
		pending: true,
		visibleStepCount: 3,
		terminalObservedAt: null,
		terminalOperationGeneration: null,
		duplicateStepSuppressedCount: 0,
	});
	const terminal = observeRailCommit(second.state, {
		pending: false,
		visibleStepCount: 0,
		terminalObservedAt: 100,
		terminalOperationGeneration: 1,
		duplicateStepSuppressedCount: 2,
	});
	assert.deepEqual(terminal.terminal, {
		visibleStepCount: 3,
		terminalObservedAt: 100,
		duplicateStepSuppressedCount: 2,
	});
	const repeatedCommit = observeRailCommit(terminal.state, {
		pending: false,
		visibleStepCount: 0,
		terminalObservedAt: 100,
		terminalOperationGeneration: 1,
		duplicateStepSuppressedCount: 2,
	});
	assert.equal(repeatedCommit.terminal, null);

	const exactRetryPending = observeRailCommit(terminal.state, {
		pending: true,
		visibleStepCount: 2,
		terminalObservedAt: null,
		terminalOperationGeneration: null,
		duplicateStepSuppressedCount: 0,
	});
	const exactRetryTerminal = observeRailCommit(exactRetryPending.state, {
		pending: false,
		visibleStepCount: 0,
		// 即使測試時鐘解析度讓 timestamp 相同，新 operation 也要獨立發送。
		terminalObservedAt: 100,
		terminalOperationGeneration: 2,
		duplicateStepSuppressedCount: 0,
	});
	assert.deepEqual(exactRetryTerminal.terminal, {
		visibleStepCount: 2,
		terminalObservedAt: 100,
		duplicateStepSuppressedCount: 0,
	});

	const reset = observeRailCommit(second.state, {
		pending: false,
		visibleStepCount: 0,
		terminalObservedAt: null,
		terminalOperationGeneration: null,
		duplicateStepSuppressedCount: 0,
	});
	assert.equal(reset.terminal, null);

	const failed = observeRailCommit(first.state, {
		pending: false,
		visibleStepCount: 0,
		terminalObservedAt: 200,
		terminalOperationGeneration: 3,
		duplicateStepSuppressedCount: 0,
	});
	assert.deepEqual(failed.terminal, {
		visibleStepCount: 1,
		terminalObservedAt: 200,
		duplicateStepSuppressedCount: 0,
	});

	const timestampWithoutOperation = observeRailCommit(first.state, {
		pending: false,
		visibleStepCount: 0,
		terminalObservedAt: 200,
		terminalOperationGeneration: null,
		duplicateStepSuppressedCount: 0,
	});
	assert.equal(timestampWithoutOperation.terminal, null);
});

test("terminal rail 等到 next frame 才送出 measured latency，cleanup 可取消", () => {
	let frameCallback: (() => void) | null = null;
	let cancelledFrame: number | null = null;
	const calls: Array<{
		eventName: string;
		params: Readonly<Record<string, number | boolean>>;
	}> = [];

	const cleanup = scheduleTerminalRailMeasurement({
		visibleStepCount: 4,
		duplicateStepSuppressedCount: 2,
		terminalObservedAt: 100,
		pageVisibleAtCommit: true,
		now: () => 117.25,
		isPageVisible: () => true,
		requestFrame: (callback) => {
			frameCallback = callback;
			return 9;
		},
		cancelFrame: (frame) => {
			cancelledFrame = frame;
		},
		dispatch: (eventName, params) => calls.push({ eventName, params }),
	});

	assert.equal(calls.length, 0, "commit 當下不得送出假的 0ms");
	assert.ok(frameCallback);
	(frameCallback as () => void)();
	assert.deepEqual(calls, [
		{
			eventName: "hawkr_agent_terminal_rendered",
			params: {
				visible_step_count: 4,
				duplicate_step_suppressed_count: 2,
				rail_was_visible: true,
				page_visible_at_commit: true,
				page_visible_at_measurement: true,
				terminal_rail_clear_latency_ms: 17.25,
			},
		},
	]);
	cleanup();
	assert.equal(cancelledFrame, 9);
});

test("無 rail 或背景頁不捏造 clear latency，payload 永遠只有數值與布林", () => {
	for (const params of [
		buildTerminalRenderedParams({
			visibleStepCount: 0,
			duplicateStepSuppressedCount: 0,
			terminalObservedAt: 100,
			terminalRenderedAt: 112,
			pageVisibleAtCommit: true,
			pageVisibleAtMeasurement: true,
		}),
		buildTerminalRenderedParams({
			visibleStepCount: 2,
			duplicateStepSuppressedCount: 3,
			terminalObservedAt: 100,
			terminalRenderedAt: 112,
			pageVisibleAtCommit: false,
			pageVisibleAtMeasurement: false,
		}),
		buildTerminalRenderedParams({
			visibleStepCount: 2,
			duplicateStepSuppressedCount: 3,
			terminalObservedAt: 100,
			terminalRenderedAt: 112,
			pageVisibleAtCommit: true,
			pageVisibleAtMeasurement: false,
		}),
	]) {
		assert.equal("terminal_rail_clear_latency_ms" in params, false);
		assert.ok(
			Object.values(params).every(
				(value) => typeof value === "number" || typeof value === "boolean",
			),
		);
		assert.ok(
			Object.keys(params).every(
				(key) =>
					!/(?:prompt|code|label|raw|payload|(?:^|_)id(?:_|$))/i.test(key),
			),
		);
	}
});

test("背景頁 terminal 立即送 duplicate aggregate，不排 rAF、不送 latency", () => {
	const calls: Array<{
		eventName: string;
		params: Readonly<Record<string, number | boolean>>;
	}> = [];
	let requestedFrame = false;

	const cleanup = scheduleTerminalRailMeasurement({
		visibleStepCount: 2,
		duplicateStepSuppressedCount: 4,
		terminalObservedAt: 100,
		pageVisibleAtCommit: false,
		now: () => 120,
		requestFrame: () => {
			requestedFrame = true;
			return 1;
		},
		dispatch: (eventName, params) => calls.push({ eventName, params }),
	});

	assert.equal(requestedFrame, false);
	assert.deepEqual(calls, [
		{
			eventName: "hawkr_agent_terminal_rendered",
			params: {
				visible_step_count: 2,
				duplicate_step_suppressed_count: 4,
				rail_was_visible: true,
				page_visible_at_commit: false,
				page_visible_at_measurement: false,
			},
		},
	]);
	assert.doesNotThrow(cleanup);
});

test("終態 commit 時前景、rAF 時轉背景，仍送 aggregate 但不送 latency", () => {
	let frameCallback: (() => void) | null = null;
	const calls: Array<{
		eventName: string;
		params: Readonly<Record<string, number | boolean>>;
	}> = [];
	scheduleTerminalRailMeasurement({
		visibleStepCount: 1,
		duplicateStepSuppressedCount: 1,
		terminalObservedAt: 100,
		pageVisibleAtCommit: true,
		now: () => 120,
		isPageVisible: () => false,
		requestFrame: (callback) => {
			frameCallback = callback;
			return 1;
		},
		dispatch: (eventName, params) => calls.push({ eventName, params }),
	});
	assert.ok(frameCallback);
	(frameCallback as () => void)();
	assert.equal("terminal_rail_clear_latency_ms" in calls[0].params, false);
	assert.equal(calls[0].params.duplicate_step_suppressed_count, 1);
});

// Source pin，不是行為測試。graph 路徑的接線都寫在 AssistantWorkspace.tsx
// 的 render body 裡，沒有可獨立呼叫的純函式，所以單元測試碰不到它們的行為。
// 這些 regex 能做到的只有一
// 件事：擋住有人把它們改回「只讀 liveSteps／只讀 latestStep」。真正的行為驗
// 證要靠跑一輪 graph turn 的 e2e。
test("[source pin] 遙測與小精靈 motion 都看得到 graph 路徑的 Activity", () => {
	// graph turn 填 liveActivity、legacy turn 填 liveSteps，同輪互斥；只數
	// liveSteps 會讓 graph turn 的 terminal_rendered 永遠回報 0 列。
	// 對話現在跨 stage 常駐，候選階段提交修改也會顯示 pending turn 的 rail，
	// 不能再限於 clarifying／ready。仍須 pendingTurn 才計數，終態不可算舊列；
	// 搜尋 run 的 searchActivity 不混入 turn terminal 的遙測分母。
	assert.match(
		workspace,
		/const visibleStepCount =\s*session\.pendingTurn\s*\?\s*session\.liveActivity\.length\s*\|\|\s*session\.liveSteps\.length\s*:\s*0;/,
	);
	// graph 與 legacy 都只在權威 turn-start 後取狀態；優先進行中列，沒有時
	// 回退這輪最後一列，不能因活動清單含 PENDING 子列就遺失目前 motion。
	assert.match(
		workspace,
		/const latestActivity = session\.turnStartObserved\s*\?\s*\(lastRunningActivity\(session\.liveActivity\)\s*\?\?\s*session\.liveActivity\.at\(-1\)\s*\?\?\s*null\)\s*:\s*null;/,
	);
	assert.match(
		workspace,
		/const latestStep = session\.turnStartObserved\s*\?\s*\(session\.liveSteps\.findLast\(\(step\) => step\.status === "RUNNING"\)\s*\?\?\s*session\.liveSteps\.at\(-1\)\s*\?\?\s*null\)\s*:\s*null;/,
	);
	// 只讀 latestStep?.code 會讓 graph turn 整輪停在 submitting 的表情。
	assert.match(
		workspace,
		/latestStepCode: latestActivity\?\.code \?\? latestStep\?\.code \?\? null,/,
	);
});
