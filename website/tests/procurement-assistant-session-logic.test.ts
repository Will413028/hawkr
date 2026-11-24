import assert from "node:assert/strict";
import test from "node:test";
import {
	applyOptimisticSelection,
	deriveStageFromStatus,
	isTerminalCandidateStatus,
	prepareTurnSubmission,
	questionHasUnitOptions,
	shouldAutofocusHandoff,
	slotLabel,
} from "../features/procurement-assistant/session-logic";
import type { ProcurementCandidate } from "@/features/procurement-assistant/types";

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

test("applyOptimisticSelection flips only the targeted candidate and returns a new array", () => {
	const candidates: ProcurementCandidate[] = [
		{
			candidateId: "1",
			displayName: "A",
			matchedFacts: [],
			unverifiableFacts: [],
			factsAsOf: "2026-08-08T00:00:00Z",
			selected: false,
		},
		{
			candidateId: "2",
			displayName: "B",
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
			unitOptions: [{ optionId: "unit-box", label: "箱" }],
		}),
		true,
	);
});
