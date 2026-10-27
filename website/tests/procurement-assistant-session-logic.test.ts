import assert from "node:assert/strict";
import test from "node:test";
import {
	applyOptimisticSelection,
	deriveStageFromStatus,
	hasPartialLaneFailure,
	isSucceededCandidateStatus,
	isTerminalCandidateStatus,
	mapSlots,
	normalizeBlocks,
	normalizeCandidateSelectionResult,
	normalizeProcurementTurnResult,
	prepareTurnSubmission,
	questionHasUnitOptions,
	shouldAutofocusHandoff,
	slotLabel,
} from "../features/procurement-assistant/session-logic";
import type { ProcurementCandidate } from "../lib/api/openapi-types";

test("exact retry 沿用 clientRequestId；只要使用者改過文字就建立新 key", () => {
	let sequence = 0;
	const createId = () => `cid-${++sequence}`;
	const first = prepareTurnSubmission(null, "  水果 50 箱  ", createId);
	assert.deepEqual(first, {
		clientRequestId: "cid-1",
		message: "水果 50 箱",
	});

	const exactRetry = prepareTurnSubmission(first, "水果 50 箱", createId);
	assert.equal(
		exactRetry,
		first,
		"相同 payload 應重用同一份 operation identity",
	);
	assert.equal(sequence, 1, "exact retry 不得產生第二個 key");

	const edited = prepareTurnSubmission(first, "水果 60 箱", createId);
	assert.deepEqual(edited, {
		clientRequestId: "cid-2",
		message: "水果 60 箱",
	});
});

test("handoff 只在桌面 autofocus，mobile/SSR 不主動叫出軟鍵盤", () => {
	assert.equal(shouldAutofocusHandoff(null), false);
	assert.equal(
		shouldAutofocusHandoff(() => ({ matches: false })),
		false,
	);
	assert.equal(
		shouldAutofocusHandoff(() => ({ matches: true })),
		true,
	);
});

test("mapSlots converts backend snake_case fields to camelCase", () => {
	assert.deepEqual(
		mapSlots({
			item: "紅蘿蔔",
			budget_per_unit: 800,
			delivery_city: "桃園市",
			delivery_date: "2026-08-20",
			exact_match_required: true,
		}),
		{
			item: "紅蘿蔔",
			quantity: undefined,
			unit: undefined,
			packaging: undefined,
			budgetPerUnit: 800,
			deliveryCity: "桃園市",
			deliveryDate: "2026-08-20",
			exactMatchRequired: true,
			itemResolution: undefined,
		},
	);
});

test("turn result normalizer accepts a valid public payload and rejects malformed slots/results", () => {
	const valid = {
		requestId: "request-1",
		revisionId: "revision-1",
		slots: {
			item: "紅蘿蔔",
			quantity: 20,
			budget_per_unit: 800,
			exact_match_required: true,
		},
		missingRequired: ["delivery_city"],
		rejected: [{ key: "delivery_date", reason: "日期已過" }],
		degraded: false,
		reply: "已記下目前的需求。",
		steps: [
			{
				code: "UNDERSTAND_INPUT",
				status: "SUCCEEDED",
				text: "TERMINAL_ONLY_STEP_SENTINEL",
			},
		],
	};
	const normalized = normalizeProcurementTurnResult(valid);
	assert.ok(normalized);
	assert.equal(normalized.requestId, valid.requestId);
	assert.equal(normalized.revisionId, valid.revisionId);
	assert.equal(normalized.reply, valid.reply);
	assert.equal(normalized.slots.item, "紅蘿蔔");
	assert.equal(normalized.slots.quantity, 20);
	assert.deepEqual(normalized.missingRequired, ["delivery_city"]);
	assert.deepEqual(normalized.rejected, valid.rejected);
	assert.equal(
		Object.hasOwn(normalized, "steps"),
		false,
		"終態 steps 不得穿過 normalizer 進入 session state",
	);
	assert.equal(normalizeProcurementTurnResult(null), null);
	assert.equal(normalizeProcurementTurnResult([]), null);
	assert.equal(normalizeProcurementTurnResult({ ...valid, slots: null }), null);
	assert.equal(
		normalizeProcurementTurnResult({
			...valid,
			slots: { quantity: Number.POSITIVE_INFINITY },
		}),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({
			...valid,
			slots: { delivery_city: 123 },
		}),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, reply: { private: true } }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, missingRequired: [null] }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, rejected: [{ key: "item" }] }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, conversationId: 123 }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, requestId: "   " }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, revisionId: " revision-1 " }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, conversationId: "   " }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...valid, degradedReason: false }),
		null,
	);
});

test("mapSlots is total for malformed snapshot input and never dereferences null", () => {
	assert.deepEqual(mapSlots(null), {
		item: undefined,
		quantity: undefined,
		unit: undefined,
		packaging: undefined,
		budgetPerUnit: undefined,
		deliveryCity: undefined,
		deliveryDate: undefined,
		exactMatchRequired: undefined,
		itemResolution: undefined,
	});
});

test("candidate selection response accepts only the exact candidate and a boolean server value", () => {
	assert.deepEqual(
		normalizeCandidateSelectionResult(
			{ candidateId: "candidate-1", selected: true },
			"candidate-1",
		),
		{ candidateId: "candidate-1", selected: true },
	);
	assert.equal(normalizeCandidateSelectionResult(null, "candidate-1"), null);
	assert.equal(
		normalizeCandidateSelectionResult(
			{ candidateId: "candidate-2", selected: true },
			"candidate-1",
		),
		null,
	);
	assert.equal(
		normalizeCandidateSelectionResult(
			{ candidateId: "candidate-1", selected: "true" },
			"candidate-1",
		),
		null,
	);
});

test("slotLabel maps known keys and falls back to the key itself", () => {
	assert.equal(slotLabel("deliveryCity"), "配送城市");
	assert.equal(slotLabel("budgetPerUnit"), "單價預算");
	assert.equal(slotLabel("unknownKey"), "unknownKey");
});

test("slotLabel also recognizes the raw snake_case wire spellings from missingRequired/rejected[].key", () => {
	assert.equal(slotLabel("budget_per_unit"), "單價預算");
	assert.equal(slotLabel("delivery_city"), "配送城市");
	assert.equal(slotLabel("delivery_date"), "到貨日期");
	assert.equal(slotLabel("exact_match_required"), "是否需完全相符");
});

test("deriveStageFromStatus maps CLARIFYING to clarifying and everything else to ready", () => {
	assert.equal(deriveStageFromStatus("CLARIFYING"), "clarifying");
	assert.equal(deriveStageFromStatus("REQUIREMENT_READY"), "ready");
	assert.equal(deriveStageFromStatus("MATCHING"), "ready");
	assert.equal(deriveStageFromStatus("CANDIDATES_READY"), "ready");
});

// 這支測試是「無限輪詢」這個故障模式的唯一自動化防線：isTerminalCandidateStatus 是
// useProcurementSession 每 1.5s 一次的輪詢的唯一終止條件，漏判一個終態值就是永遠停在
// 「正在搜尋候選」——而且 HTTP 一路 200 OK，不會進入任何錯誤狀態。
test("isTerminalCandidateStatus 認得契約凍結後的終態", () => {
	assert.equal(isTerminalCandidateStatus("QUEUED"), false);
	assert.equal(isTerminalCandidateStatus("RUNNING"), false);
	assert.equal(isTerminalCandidateStatus("SUCCEEDED"), true);
	assert.equal(isTerminalCandidateStatus("FAILED"), true);
});

// 舊值不得再被當成終態。少了這支，把 legacy 分支偷偷加回來不會有任何測試變紅——
// 而「兩組值並存」正是本波要收掉的過渡狀態。型別已不含這些值，故用 as 繞過型別層，
// 因為要驗的正是「執行期收到型別外的字串時會怎樣」。
test("isTerminalCandidateStatus 不再認改名前的舊值", () => {
	const legacy = ["COMPLETED", "PARTIAL"] as unknown as Array<
		Parameters<typeof isTerminalCandidateStatus>[0]
	>;
	for (const status of legacy) {
		assert.equal(isTerminalCandidateStatus(status), false);
	}
});

test("isSucceededCandidateStatus 區分成功終態與失敗終態", () => {
	assert.equal(isSucceededCandidateStatus("SUCCEEDED"), true);
	assert.equal(isSucceededCandidateStatus("FAILED"), false);
	assert.equal(isSucceededCandidateStatus("RUNNING"), false);
});

// 「部分來源未完成」的判準從「run 狀態是 PARTIAL」換成「run 成功但有 lane 失敗」。
// 這是本次改名唯一一處語意重寫（其餘都是換字面值），所以要正反兩面都釘住。
test("hasPartialLaneFailure 由 run 成功 + lane 失敗推導，而非讀 run 狀態", () => {
	assert.equal(
		hasPartialLaneFailure({
			status: "SUCCEEDED",
			platformLaneStatus: "FAILED",
			externalLaneStatus: "SUCCEEDED",
		}),
		true,
		"單一 lane 失敗必須提示部分來源未完成",
	);
	assert.equal(
		hasPartialLaneFailure({
			status: "SUCCEEDED",
			platformLaneStatus: "SUCCEEDED",
			externalLaneStatus: "SUCCEEDED",
		}),
		false,
		"兩條 lane 都成功時不得誤報",
	);
	assert.equal(
		hasPartialLaneFailure({
			status: "FAILED",
			platformLaneStatus: "FAILED",
			externalLaneStatus: "FAILED",
		}),
		false,
		"整個 run 失敗走的是錯誤畫面，不是部分成功提示",
	);
	assert.equal(
		hasPartialLaneFailure({
			status: "SUCCEEDED",
			platformLaneStatus: "PENDING",
			externalLaneStatus: "SUCCEEDED",
		}),
		false,
		"lane 還沒跑完不是失敗，不得提示部分來源未完成",
	);
});

test("applyOptimisticSelection flips only the targeted candidate and returns a new array", () => {
	const candidates: ProcurementCandidate[] = [
		{
			candidateId: "1",
			sourceId: "s1",
			lane: "PLATFORM",
			displayName: "A",
			score: 1,
			matchedFacts: [],
			unverifiableFacts: [],
			factsAsOf: "2026-08-08T00:00:00Z",
			selected: false,
		},
		{
			candidateId: "2",
			sourceId: "s2",
			lane: "PLATFORM",
			displayName: "B",
			score: 1,
			matchedFacts: [],
			unverifiableFacts: [],
			factsAsOf: "2026-08-08T00:00:00Z",
			selected: false,
		},
	];
	const next = applyOptimisticSelection(candidates, "1", true);
	assert.equal(next[0]?.selected, true);
	assert.equal(next[1]?.selected, false);
	assert.notEqual(next, candidates);
	assert.equal(candidates[0]?.selected, false);
});

test("questionHasUnitOptions 是『這題需不需要單位 chip』的單一權威來源——ClarifyQuestionCard 與 AssistantWorkspace 都要問同一個函式，不各自複製一份判斷式", () => {
	assert.equal(questionHasUnitOptions(null), false);
	assert.equal(questionHasUnitOptions(undefined), false);
	assert.equal(questionHasUnitOptions({ unitOptions: undefined }), false);
	assert.equal(questionHasUnitOptions({ unitOptions: [] }), false);
	assert.equal(
		questionHasUnitOptions({
			unitOptions: [{ optionId: "unit-box", label: "箱", value: "BOX" }],
		}),
		true,
	);
});

test("#2549 Phase 2：normalize 透傳 blocks/textFallback，缺席時保持 undefined", () => {
	const withBlocks = normalizeProcurementTurnResult({
		requestId: "1",
		revisionId: "1",
		slots: {},
		rejected: [],
		missingRequired: [],
		degraded: false,
		reply: "已收到你的訊息。",
		blocks: [{ kind: "UNDERSTOOD", text: "已收到你的訊息。" }],
		textFallback: "已收到你的訊息。",
	});
	assert.deepEqual(withBlocks?.blocks, [
		{ kind: "UNDERSTOOD", text: "已收到你的訊息。" },
	]);
	assert.equal(withBlocks?.textFallback, "已收到你的訊息。");

	const withoutBlocks = normalizeProcurementTurnResult({
		requestId: "1",
		revisionId: "1",
		slots: {},
		rejected: [],
		missingRequired: [],
		degraded: false,
		reply: "已收到你的訊息。",
	});
	assert.equal(withoutBlocks?.blocks, undefined);
	assert.equal(withoutBlocks?.textFallback, undefined);
});

test("normalizeBlocks 逐元素驗證 kind／text，壞元素直接丟棄而非讓整個 turn 報廢", () => {
	// 合法元素照樣通過。
	assert.deepEqual(normalizeBlocks([{ kind: "CONTEXT", text: "配送到台中" }]), [
		{ kind: "CONTEXT", text: "配送到台中" },
	]);

	// kind 不在 7 個 allowlisted 值域內——不管是亂字串還是繼承自
	// Object.prototype 的 "constructor"，一律丟棄（後者驗證的正是
	// narrativeIcon 對 Object.freeze 物件的 prototype-chain 查表風險）。
	assert.deepEqual(
		normalizeBlocks([
			{ kind: "NOT_A_KIND", text: "x" },
			{ kind: "constructor", text: "y" },
			{ kind: "CONTEXT", text: "配送到台中" },
		]),
		[{ kind: "CONTEXT", text: "配送到台中" }],
	);

	// text 缺席或型別不對——丟棄。
	assert.deepEqual(
		normalizeBlocks([
			{ kind: "CONTEXT" },
			{ kind: "CONTEXT", text: 123 },
			{ kind: "CONTEXT", text: "配送到台中" },
		]),
		[{ kind: "CONTEXT", text: "配送到台中" }],
	);

	// 元素本身不是 object（null／字串／陣列）——丟棄，不 throw。
	assert.deepEqual(
		normalizeBlocks([
			null,
			"not-an-object",
			["nested"],
			{ kind: "WARNING", text: "庫存不足" },
		]),
		[{ kind: "WARNING", text: "庫存不足" }],
	);

	// 全部無效、或本來就是空陣列——回 undefined 而非 []，讓
	// `turn.blocks ?? legacyReplyToBlocks(turn.text)` 的 fallback 邏輯成立。
	assert.equal(normalizeBlocks([{ kind: "NOT_A_KIND", text: "x" }]), undefined);
	assert.equal(normalizeBlocks([]), undefined);

	// 非陣列輸入一律 undefined。
	assert.equal(normalizeBlocks(undefined), undefined);
	assert.equal(normalizeBlocks(null), undefined);
	assert.equal(normalizeBlocks("blocks"), undefined);
});

test("normalizeProcurementTurnResult 透過 normalizeBlocks 驗證 blocks[] 每個元素——壞元素被濾掉，不會讓 turn 整批被拒或讓壞資料流進渲染層", () => {
	const normalized = normalizeProcurementTurnResult({
		requestId: "1",
		revisionId: "1",
		slots: {},
		rejected: [],
		missingRequired: [],
		degraded: false,
		reply: "已收到你的訊息。",
		blocks: [
			{ kind: "UNDERSTOOD", text: "已收到你的訊息。" },
			{ kind: "constructor", text: "不應通過" },
			{ kind: "CONTEXT", text: null },
		],
	});
	assert.deepEqual(normalized?.blocks, [
		{ kind: "UNDERSTOOD", text: "已收到你的訊息。" },
	]);

	// 全部元素都無效時，blocks 是 undefined 而非空陣列。
	const allInvalid = normalizeProcurementTurnResult({
		requestId: "1",
		revisionId: "1",
		slots: {},
		rejected: [],
		missingRequired: [],
		degraded: false,
		reply: "已收到你的訊息。",
		blocks: [{ kind: "NOT_A_KIND", text: "x" }],
	});
	assert.equal(allInvalid?.blocks, undefined);
});

test("normalizeProcurementTurnResult 解析 interruptId／interruptKind——unary SubmitTurn 與串流終態的 RUN_FINISHED.result 是同一份 TurnResult，兩個欄位成對出現才代表這一輪落在 WAITING_FOR_INPUT", () => {
	const BASE = {
		requestId: "1",
		revisionId: "1",
		slots: {},
		rejected: [],
		missingRequired: [],
		degraded: false,
		reply: "請選擇品項。",
	};

	// 缺席時兩個欄位都是 undefined——這是絕大多數正常落地的 turn。
	const withoutInterrupt = normalizeProcurementTurnResult(BASE);
	assert.equal(withoutInterrupt?.interruptId, undefined);
	assert.equal(withoutInterrupt?.interruptKind, undefined);

	// 成對出現時原樣透傳。
	const withInterrupt = normalizeProcurementTurnResult({
		...BASE,
		interruptId: "int-1",
		interruptKind: "CROP_DISAMBIGUATION",
	});
	assert.equal(withInterrupt?.interruptId, "int-1");
	assert.equal(withInterrupt?.interruptKind, "CROP_DISAMBIGUATION");

	// 型別錯誤——回 null，不讓半吊子資料流進 applyTurnResult。
	assert.equal(
		normalizeProcurementTurnResult({ ...BASE, interruptId: 1 }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({ ...BASE, interruptKind: 1 }),
		null,
	);

	// 只出現一半是不可能的中介狀態（兩者在 wire 上成對出現），視為壞掉的回應。
	assert.equal(
		normalizeProcurementTurnResult({ ...BASE, interruptId: "int-1" }),
		null,
	);
	assert.equal(
		normalizeProcurementTurnResult({
			...BASE,
			interruptKind: "CROP_DISAMBIGUATION",
		}),
		null,
	);
});
