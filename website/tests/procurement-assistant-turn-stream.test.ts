import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { procurementErrorFromStatus } from "../features/procurement-assistant/errors";
import {
	type AgentStep,
	STEP_ICON,
	stepLabel,
} from "../features/procurement-assistant/steps";
import { mergeLiveStep } from "../features/procurement-assistant/turn-stream";

const root = fileURLToPath(new URL("..", import.meta.url));
const stream = readFileSync(
	path.join(root, "features/procurement-assistant/turn-stream.ts"),
	"utf8",
);
const session = readFileSync(
	path.join(root, "features/procurement-assistant/useProcurementSession.ts"),
	"utf8",
);
const ticker = readFileSync(
	path.join(root, "features/procurement-assistant/AgentActivityRail.tsx"),
	"utf8",
);
const chat = readFileSync(
	path.join(root, "features/procurement-assistant/ProcurementClarifyChat.tsx"),
	"utf8",
);
const taskPanel = readFileSync(
	path.join(root, "features/procurement-assistant/AgentTaskPanel.tsx"),
	"utf8",
);

const step = (
	code: string,
	status: AgentStep["status"],
	durationMs?: number,
): AgentStep => ({ code, status, durationMs });

test("每階段的 RUNNING → SUCCEEDED 收斂成同一步，不長出兩行", () => {
	let steps: AgentStep[] = [];
	steps = mergeLiveStep(steps, step("UNDERSTAND_INPUT", "RUNNING"));
	steps = mergeLiveStep(steps, step("UNDERSTAND_INPUT", "SUCCEEDED", 120));
	assert.equal(steps.length, 1);
	assert.equal(steps[0].status, "SUCCEEDED");
	assert.equal(steps[0].durationMs, 120);
});

test("Compose 的 heartbeat（重發同 code 的 RUNNING）不得把已完成的那步打回進行中，也不得清掉耗時", () => {
	// 後端 composeWithHeartbeat 每隔一段時間重發同一個 code 的 RUNNING，
	// 用來證明連線還活著——那不是「這步又重新開始了」。
	let steps = mergeLiveStep([], step("PICK_NEXT_QUESTION", "RUNNING"));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", "SUCCEEDED", 8400));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", "RUNNING"));
	assert.equal(steps.length, 1);
	assert.equal(steps[0].status, "SUCCEEDED");
	assert.equal(steps[0].durationMs, 8400);
});

test("不同階段依抵達順序 append——這就是跑條換行的順序", () => {
	let steps: AgentStep[] = [];
	for (const code of [
		"UNDERSTAND_INPUT",
		"EXTRACT_SLOTS",
		"VALIDATE_SLOTS",
		"PICK_NEXT_QUESTION",
	]) {
		steps = mergeLiveStep(steps, step(code, "RUNNING"));
		steps = mergeLiveStep(steps, step(code, "SUCCEEDED", 10));
	}
	assert.deepEqual(
		steps.map((s) => s.code),
		[
			"UNDERSTAND_INPUT",
			"EXTRACT_SLOTS",
			"VALIDATE_SLOTS",
			"PICK_NEXT_QUESTION",
		],
	);
});

test("逐欄位 CAPTURE_* 是抽取階段的細粒度事件：一碼一欄位，各自成行", () => {
	// 前端先擬好的逐欄位進度碼，對應 backend-payuni 未合併分支
	// `feat/procurement-field-steps`（PR #2373，開出即關閉，未上線）。同 code
	// 會被 mergeLiveStep 併成一行，所以一個 slot key 一碼才能逐欄位前進。不含
	// CAPTURE_SPEC——對應的 spec slot 已隨 #2476／#498 永久移除。
	const codes = [
		"CAPTURE_ITEM",
		"CAPTURE_QUANTITY",
		"CAPTURE_UNIT",
		"CAPTURE_PACKAGING",
		"CAPTURE_BUDGET",
		"CAPTURE_CITY",
		"CAPTURE_DATE",
		"CAPTURE_EXACT_MATCH",
	];
	let steps: AgentStep[] = [];
	// 串流時 CAPTURE 只發成功終態（durationMs 為 0，真實逐欄位耗時留在終態）。
	for (const code of codes)
		steps = mergeLiveStep(steps, step(code, "SUCCEEDED", 0));
	assert.equal(steps.length, codes.length, "每個欄位都要自己一行");
	for (const code of codes) {
		assert.ok(code.startsWith("CAPTURE_"), `${code} 命名要守封閉約定`);
		assert.notEqual(
			STEP_ICON[code],
			undefined,
			`${code} 沒有 icon 會落到 clock 降級，8 行長得一模一樣`,
		);
	}
});

test("CAPTURE_* 的 icon 彼此不重複——重複就等於沒有拆粒度", () => {
	const captureIcons = Object.entries(STEP_ICON)
		.filter(([code]) => code.startsWith("CAPTURE_"))
		.map(([, icon]) => icon);
	assert.equal(
		new Set(captureIcons).size,
		captureIcons.length,
		`CAPTURE_* icon 撞號：${captureIcons.join("／")}`,
	);
});

test("CAPTURE_* 備援 label 與後端固定字串表逐字一致（兩邊不得漂移）", () => {
	// 後端尚未上線這組事件（backend-payuni 分支 feat/procurement-field-steps
	// 未合併，見 steps.ts 開頭註解），這裡先跟該分支的草稿字串對齊。
	const backend: Record<string, string> = {
		CAPTURE_ITEM: "抓到你要的品項",
		CAPTURE_QUANTITY: "抓到數量",
		CAPTURE_UNIT: "抓到單位",
		CAPTURE_PACKAGING: "抓到包裝方式",
		CAPTURE_BUDGET: "抓到你的預算",
		CAPTURE_CITY: "抓到送達縣市",
		CAPTURE_DATE: "抓到到貨日",
		CAPTURE_EXACT_MATCH: "抓到要不要完全相符",
	};
	for (const [code, label] of Object.entries(backend)) {
		assert.equal(stepLabel({ code }), label, `${code} 的備援文案漂移了`);
	}
});

test("候選 public steps 不把公開資料說成庫存承諾、最佳排名或 Agent 建議", () => {
	const labels = [
		stepLabel({ code: "CHECK_STOCK" }),
		stepLabel({ code: "RANK_CANDIDATES" }),
		stepLabel({ code: "SUMMARIZE" }),
	];
	assert.deepEqual(labels, [
		"標出仍需商家確認的供貨資訊",
		"整理候選資料順序",
		"整理這一輪已提供的事實",
	]);
	assert.doesNotMatch(
		labels.join(" "),
		/供應量夠不夠|最合適|最佳|最推薦|給你的建議/,
	);
});

test("公開處理列只在當前 turn 或搜尋 run 呈現，終態不寫回對話或快照", () => {
	assert.doesNotMatch(chat, /AgentStepSummary/);
	assert.doesNotMatch(chat, /replay/i);
	assert.doesNotMatch(chat, /turn\.steps/);
	assert.doesNotMatch(session, /normalizeSteps\(data\.steps\)/);
	// 主區的處理 slot 由 AgentTaskPanel 呈現 SSE 推來的即時列，與正式回覆分層。
	assert.doesNotMatch(chat, /AgentActivityRail/);
	assert.match(
		taskPanel,
		/<AgentActivityRail[\s\S]*?steps=\{liveSteps\}[\s\S]*?activity=\{liveActivity\}[\s\S]*?motion=\{motion\}[\s\S]*?\/>/,
	);
	assert.match(
		taskPanel,
		/const hasLiveRows =\s*liveSteps\.length > 0 \|\| liveActivity\.length > 0;/,
	);
	// 完成後立即移除，不因 conversationId 補號或換 run 而保存舊處理列。
	assert.match(
		taskPanel,
		/if \(stage !== "searching" && !pending\) return null;/,
	);
	assert.doesNotMatch(taskPanel, /RailSnapshot|snapshotRail|setSnapshot/);
	assert.match(taskPanel, /activity=\{searchActivity\}/);
	// graph 路徑的列身分鍵是後端指定的 rowId，不是（clientRequestId, code）。
	// T23 起分成 top-level 列與 lane 子列兩層，兩層都必須用後端的 rowId 當 key。
	assert.match(ticker, /key=\{row\.rowId\}/);
	assert.match(ticker, /key=\{child\.rowId\}/);
});

test("request 不在了（對話已軟刪／深連結指向別人的需求）給中文引導，不端出後端英文字串", () => {
	// Gateway 對非 BusinessError 的錯誤（含這句英文 "request not found"）送出
	// 空 reason（gateway/internal/handler/procurement_errors.go 的 `reason := ""`
	// fallback）。
	assert.equal(
		procurementErrorFromStatus(404, "", "request not found").code,
		"CONVERSATION_GONE",
	);
	assert.match(
		procurementErrorFromStatus(404, "", "request not found").detail,
		/新對話/,
	);
	// 後端也可能用非 404 帶同一句話。
	assert.equal(
		procurementErrorFromStatus(500, "", "request not found").code,
		"CONVERSATION_GONE",
	);
	// 其他錯誤讀 {code,reason,message} 三個獨立欄位，不再切字串。
	assert.equal(
		procurementErrorFromStatus(422, "INPUT_TOO_LONG", "單則訊息上限 2000 字")
			.code,
		"INPUT_TOO_LONG",
	);
});

test("SSE transport 的 AG-UI 404 RUN_ERROR 同樣翻成中文引導", () => {
	assert.match(session, /err\.code === 404\s*\n?\s*\? CONVERSATION_GONE/);
});

test("step 事件走 normalizeSteps 同一套收斂，不長出第二套驗證", () => {
	assert.match(stream, /normalizeSteps\(\[parsed\]\)\[0\] \?\? null/);
});

test("串流不可用（404/405 未部署、429 併發已滿）降級同步端點，且沿用同一個 clientRequestId", () => {
	assert.match(stream, /err\.status === 404 \|\| err\.status === 405/);
	assert.match(stream, /err\.status === 429/);
	// probe-once：端點不存在就記住，別每輪再浪費一個 round trip。
	assert.match(stream, /streamEndpointAvailable = false/);
	// 降級用同一個 clientRequestId → 後端冪等 replay，不會多一筆 revision。
	const execute = session.slice(session.indexOf("const execute = async"));
	const syncPost = execute.indexOf('"/buyers/me/procurement/turns"');
	assert.ok(syncPost > 0, "降級路徑（同步 /turns）不見了");
	assert.match(
		execute.slice(0, syncPost),
		/runTurnStream\(/,
		"必須先試串流才降級",
	);
	assert.doesNotMatch(
		execute.slice(0, syncPost),
		/crypto\.randomUUID\(\)/,
		"降級時不得換新的 clientRequestId——那會在後端多開一輪",
	);
});

test("SSE transport 的 AG-UI RUN_ERROR 不降級重送——那一輪真的失敗了", () => {
	assert.match(
		stream,
		/if \(eventError\) \{[\s\S]*?reportTerminal\([\s\S]*?throw eventError;/,
	);
	assert.match(
		session,
		/if \(err instanceof TurnStreamEventError\) throw err;/,
	);
	assert.match(session, /err instanceof TurnStreamEventError/);
});

test("stream 開場 HTTP 409 也收斂成 terminal turn error，不降級再 POST 一次", () => {
	assert.match(stream, /if \(err\.status === 409\)/);
	assert.match(stream, /throw new TurnStreamEventError\(409,/);
});

test("RUN_FINISHED.result 不合法（連線中途斷）也走降級，把結果從同步端點撈回來", () => {
	assert.match(
		stream,
		/!sawAnyEvent \|\| !landed\.result \|\| typeof landed\.result !== "object"/,
	);
});

test("遲到的 step 不得在已收尾／已重置的畫面上冒出進度", () => {
	assert.match(
		session,
		/isCurrent\(\) && s\.pendingTurn[\s\S]*?liveSteps: mergeLiveStep/,
	);
});

test("終態結果會清掉兩條 ephemeral rail，且不投影 data.steps", () => {
	const applyTurnResult = session.slice(
		session.indexOf("const applyTurnResult"),
		session.indexOf("const submitTurn"),
	);
	assert.match(applyTurnResult, /pendingTurn: false,[\s\S]*?liveSteps: \[\],/);
	assert.match(applyTurnResult, /liveActivity: \[\],/);
	assert.doesNotMatch(applyTurnResult, /normalizeSteps\(data\.steps\)/);
});

test("狀態軌的節奏完全來自伺服器——沒有計時器、沒有回放收尾回呼", () => {
	assert.doesNotMatch(ticker, /setTimeout/);
	assert.doesNotMatch(ticker, /onComplete/);
	assert.doesNotMatch(ticker, /holdMs/);
	// 設計書 v6 §V.10.6-1：收到幾個 step 就顯示幾列，前一列在下一步抵達後
	// 仍然保留（step-level 完成只完成那一列）。
	assert.match(ticker, /steps\.map\(\(step\) =>/);
	assert.doesNotMatch(ticker, /\.at\(-1\)/);
});

// 契約凍結後舊值 DONE 不再是成功終態：normalizeSteps 的 deny-by-default 白名單會直接
// 丟掉它，而 mergeLiveStep 收到它也不得當成已完成。沒有這一則，把 legacy 分支偷偷加回來
// 不會有任何測試變紅。型別已不含 DONE，故用 as 繞過——要驗的正是執行期收到型別外字串的行為。
test("改名前的 DONE 不再被視為已完成", () => {
	const legacyDone = "DONE" as unknown as AgentStep["status"];
	let steps = mergeLiveStep([], step("PICK_NEXT_QUESTION", "RUNNING"));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", legacyDone, 8400));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", "RUNNING"));
	assert.equal(steps.length, 1);
	assert.equal(
		steps[0].status,
		"RUNNING",
		"DONE 不是成功終態，後到的 RUNNING heartbeat 應照常覆寫",
	);
});

// 正向的對照：真正的成功終態不得被後到的 heartbeat 打回。沒有這一則，
// 把 isStepSucceeded 寫成 `return false` 上面那支也會綠，而線上症狀是已完成的步驟
// 在跑條上反覆跳回進行中、耗時被清掉。
test("SUCCEEDED 不得被後到的 RUNNING heartbeat 打回", () => {
	let steps = mergeLiveStep([], step("PICK_NEXT_QUESTION", "RUNNING"));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", "SUCCEEDED", 8400));
	steps = mergeLiveStep(steps, step("PICK_NEXT_QUESTION", "RUNNING"));
	assert.equal(steps.length, 1);
	assert.equal(steps[0].status, "SUCCEEDED");
	assert.equal(steps[0].durationMs, 8400);
});
