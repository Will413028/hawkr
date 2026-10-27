import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const hook = readFileSync(
	path.join(root, "features/procurement-assistant/useProcurementSession.ts"),
	"utf8",
);

test("submitTurn re-fetches snapshot for the authoritative status instead of inferring from missingRequired", () => {
	assert.match(
		hook,
		/const snapshot = await fetchSnapshot\(data\.requestId\);/,
	);
	assert.doesNotMatch(hook, /missingRequired\.length === 0/);
});

test("submitTurn 的 HTTP/SSE 409 都 fail closed，不換 key 自動建立第二個 operation", () => {
	assert.match(hook, /err instanceof ApiClientError && err\.status === 409/);
	assert.match(hook, /err\.code === 409/);
	assert.match(hook, /error: TURN_IDEMPOTENCY_CONFLICT/);
	assert.doesNotMatch(
		hook,
		/await (?:attempt|execute)\(crypto\.randomUUID\(\)/,
	);
	assert.doesNotMatch(hook, /已自動用新的識別碼重試/);
});

test("exact retry 用 clientRequestId 當 optimistic turn id，不在 transcript 疊出同文泡泡", () => {
	assert.match(hook, /id: effectiveClientRequestId,/);
	assert.match(
		hook,
		/s\.turns\.some\(\s*\(turn\) => turn\.id === effectiveClientRequestId/,
	);
});

test("turn transport captures one immutable operation identity and never re-reads requestId for fallback", () => {
	const submitTurn = hook.slice(
		hook.indexOf("const submitTurn"),
		hook.indexOf("const submitAnswer"),
	);
	assert.match(submitTurn, /const operation = operationGuard\.begin\(\{/);
	assert.match(submitTurn, /requestId: current\.requestId,/);
	assert.doesNotMatch(submitTurn, /stateRef\.current\.requestId/);
	assert.match(submitTurn, /requestId: operation\.requestId \?\? undefined/);
	assert.match(submitTurn, /clientRequestId: operation\.clientRequestId/);
	assert.match(submitTurn, /signal: operation\.signal/);
	assert.match(
		submitTurn,
		/stateRef\.current = \{ \.\.\.current, pendingTurn: true \};/,
	);
	assert.match(
		submitTurn,
		/if \(!operationGuard\.isCurrent\(operation\)\) return;/,
	);
});

test("reset invalidates turn and session generations before synchronously clearing stateRef", () => {
	const reset = hook.slice(
		hook.indexOf("const reset"),
		hook.indexOf("const [pollRetryToken"),
	);
	assert.match(reset, /operationGuard\.invalidate\(\);/);
	assert.match(reset, /sessionGenerationGuard\.invalidate\(\);/);
	assert.match(reset, /stateRef\.current = nextState;/);
	assert.match(reset, /setRestoring\(false\);/);
});

test("candidate, poll, and quote async paths are scoped to session generation", () => {
	assert.match(
		hook,
		/const sessionIdentity = sessionGenerationGuard\.capture\(/,
	);
	assert.match(
		hook,
		/sessionGenerationGuard\.isCurrent\(sessionIdentity, s\.requestId\)/,
	);
	assert.match(hook, /s\.candidateSet\?\.candidateSetId === candidateSetId/);
	// legacy quote-batch 的建立與撤回端已下架（doc 64 §7），連同它們自己的
	// generation 守衛一起——這裡不再有第二組 quoteBatchId 判準。
	assert.doesNotMatch(hook, /quoteBatchId/);
});

test("applyTurnResult and snapshot completion reject stale or mismatched operation identity", () => {
	const applyTurnResult = hook.slice(
		hook.indexOf("const applyTurnResult"),
		hook.indexOf("const submitTurn"),
	);
	assert.match(
		applyTurnResult,
		/if \(!operationGuard\.isCurrent\(operation\)\) return;/,
	);
	assert.match(
		applyTurnResult,
		/operationGuard\.acceptsResult\(operation, data\.requestId\)/,
	);
	assert.match(applyTurnResult, /snapshot\.revisionId !== data\.revisionId/);
});

test("polling backs off through 1.5s/3s/6s and stops after exhausting retries", () => {
	assert.match(hook, /const POLL_BACKOFF_MS = \[1500, 3000, 6000\] as const;/);
	assert.match(hook, /failureCount >= POLL_BACKOFF_MS\.length\)/);
	assert.doesNotMatch(hook, /POLL_BACKOFF_MS\.length - 1/);
	const polling = hook.slice(
		hook.indexOf("const requestId = state.requestId;"),
		hook.indexOf("const retryPolling"),
	);
	assert.match(
		polling,
		/sessionGenerationGuard\.isCurrent\(sessionIdentity, s\.requestId\)[\s\S]*?pollError: true/,
	);
});

test("polling stops on terminal status and switches to the candidates stage", () => {
	assert.match(hook, /isTerminalCandidateStatus\(env\.data\.status\)/);
	assert.match(hook, /stage: "candidates",\s*candidateSet: env\.data/);
});

test("toggleSelection applies the flip optimistically before the request resolves", () => {
	// toggleSelection 已改為三個分支（樂觀套用／成功覆寫／失敗回滾）共用同一個
	// applySelectedValue helper，差別只在傳入的 selected 值。這裡驗證「樂觀套用」
	// 那次呼叫（傳 nextSelected）確實發生在送出 POST /selections 之前。
	const optimisticSetIndex = hook.indexOf(
		"applySelectedValue(s.candidateSet, nextSelected)",
	);
	const postIndex = hook.indexOf("/selections`,");
	assert.ok(optimisticSetIndex > -1 && postIndex > -1);
	assert.ok(
		optimisticSetIndex < postIndex,
		"optimistic update must run before the POST call",
	);
});

test("toggleSelection rolls back only the toggled candidate to its captured pre-toggle value on failure, using latest state (not a stale whole-snapshot capture)", () => {
	// 失敗回滾走 applySelectedValue(s.candidateSet, previousSelected)：只復原
	// 這一個 candidateId，且讀最新的 s.candidateSet（functional setState），不是
	// 呼叫前就地捕捉的整份快照（previousCandidateSet 已不存在於原始碼中）——
	// 否則使用者連續勾選兩個候選、其中一個先失敗時，會連帶蓋掉另一個已成功的選取。
	assert.match(
		hook,
		/applySelectedValue\(\s*s\.candidateSet,\s*previousSelected,?\s*\)/,
	);
	assert.doesNotMatch(hook, /previousCandidateSet/);
});

test("toggleSelection overwrites the optimistic value with the server response on success (does not trust the local flip blindly)", () => {
	assert.match(
		hook,
		/applySelectedValue\(\s*s\.candidateSet,\s*response\.selected,?\s*\)/,
	);
});

test("toggleSelection serializes one candidate and validates the untrusted response before applying it", () => {
	assert.match(hook, /selectionGuard\.begin\(/);
	assert.match(hook, /if \(!selectionIdentity\) return;/);
	assert.match(hook, /apiClient\.authed\.post<unknown>\(/);
	assert.match(hook, /normalizeCandidateSelectionResult\(/);
	assert.match(hook, /selectionGuard\.finish\(selectionIdentity\)/);
	assert.match(hook, /CANDIDATE_SELECTION_UNCONFIRMED/);
	assert.match(
		hook,
		/selectionUnconfirmed\s*=\s*invalidResponse \|\| !\(err instanceof ApiClientError\)/,
	);
});

test("every turn result passes the runtime normalizer before identity, slots, or transcript mutation", () => {
	const applyTurnResult = hook.slice(
		hook.indexOf("const applyTurnResult"),
		hook.indexOf("const submitTurn"),
	);
	const normalizeIndex = applyTurnResult.indexOf(
		"normalizeProcurementTurnResult(rawData)",
	);
	const identityIndex = applyTurnResult.indexOf(
		"operationGuard.acceptsResult(operation, data.requestId)",
	);
	const slotsIndex = applyTurnResult.indexOf("mapSlots(data.slots)");
	assert.ok(normalizeIndex > -1);
	assert.ok(identityIndex > normalizeIndex);
	assert.ok(slotsIndex > normalizeIndex);
	assert.match(hook, /post<unknown>\(/);
	assert.doesNotMatch(hook, /result as RawTurnResult/);
});

test("toggleSelection locks further toggles once CANDIDATE_FACTS_STALE is seen", () => {
	assert.match(hook, /candidatesStale: true, error: parsed/);
	assert.match(hook, /current\.candidatesStale\s*\)\s*return;/);
});

test("session restore builds a slots summary instead of fabricating turn history", () => {
	assert.match(hook, /restoredSummary:/);
	assert.doesNotMatch(hook, /turns: \[.*snapshot/);
});

test("session restore invalidates revision-derived artifacts before any remote fetch", () => {
	const restoreEffect = hook.slice(
		hook.indexOf("if (!initialRequestId && !initialConversationId) return;"),
		hook.indexOf("const applyTurnResult = React.useCallback("),
	);
	const generationInvalidationIndex = restoreEffect.indexOf(
		"sessionGenerationGuard.invalidate();",
	);
	const restoreCaptureIndex = restoreEffect.indexOf(
		"const restoreIdentity = sessionGenerationGuard.capture(",
	);
	const invalidateIndex = restoreEffect.indexOf(
		"invalidateCandidateSelections();",
	);
	const fetchIndex = restoreEffect.indexOf("await fetchTranscript(");
	assert.ok(generationInvalidationIndex > -1);
	assert.ok(restoreCaptureIndex > generationInvalidationIndex);
	assert.ok(invalidateIndex > -1);
	assert.ok(fetchIndex > invalidateIndex);
	assert.match(
		restoreEffect.slice(invalidateIndex, fetchIndex),
		/runId: null,[\s\S]*?candidateSet: null,/,
	);
});

test("candidate search keeps the previous candidate canvas until the replacement run is accepted", () => {
	const startSearch = hook.slice(
		hook.indexOf("const startSearch"),
		hook.indexOf("const requestId = state.requestId;"),
	);
	const postIndex = startSearch.indexOf("apiClient.authed.post<");
	const acceptedIndex = startSearch.indexOf(
		"if (!isCurrentSession() || env.data.requestId !== requestId) return;",
	);
	const invalidateIndex = startSearch.indexOf(
		"invalidateCandidateSelections();",
	);
	const clearCandidateIndex = startSearch.indexOf("candidateSet: null,");

	assert.ok(postIndex > -1);
	assert.ok(acceptedIndex > postIndex);
	assert.ok(invalidateIndex > acceptedIndex);
	assert.ok(clearCandidateIndex > invalidateIndex);
	assert.doesNotMatch(
		startSearch.slice(0, acceptedIndex),
		/candidateSet: null/,
	);
});

test("submitTurn uses the backend-composed natural-language reply instead of building its own template text", () => {
	assert.match(hook, /text: data\.reply,/);
	assert.doesNotMatch(hook, /summarizeTurnResult/);
});

test("原始模型推理（reasoning）已依 doc 55 §8.1 下架——hook 不得再讀取或轉傳", () => {
	// 哨兵：後端仍可能回傳 reasoning 欄位（過渡期），但前端不得消費。
	assert.doesNotMatch(hook, /\breasoning\b(?!.*不再消費)/);
});

test("turns／answers／rewinds 共用同一個 applyTurnResult（同構回應、同一 serializer）", () => {
	assert.match(hook, /const applyTurnResult = React\.useCallback/);
	const applyCalls = hook.match(/await applyTurnResult\(/g) ?? [];
	assert.ok(
		applyCalls.length >= 3,
		`expected turns/answers/rewinds to all route through applyTurnResult, saw ${applyCalls.length}`,
	);
});

test("第一輪拿到 conversationId 後用 history.replaceState 寫回網址（不是 router.replace）", () => {
	assert.match(hook, /window\.history\.replaceState/);
	assert.doesNotMatch(hook, /\brouter\.replace\(/);
});

test("submitAnswer 帶 expectedRevisionId，STALE_REVISION 時整段同步最新快照", () => {
	assert.match(hook, /expectedRevisionId: current\.revisionId,/);
	assert.match(hook, /STALE_REVISION/);
});

test("submitAnswer 接受 skip:true，request body 帶上 skip 欄位（與 optionId/freeText 同一個 json 物件）", () => {
	assert.match(hook, /skip\?: boolean;/);
	assert.match(hook, /skip: answer\.skip,/);
});

test("submitAnswer 的 skip:true 繞過『沒有文字就不送』的空字串 guard，樂觀泡泡顯示「略過」", () => {
	assert.match(hook, /answer\.skip\s*\?\s*"略過"/);
	// guard 仍在，只是永遠有值可判斷——不得為了 skip 另開一條不檢查 buyerText 的分支。
	assert.match(hook, /if \(!buyerText\) return;/);
});

test("executeRfqPublication 打 execute 端點而不是 preview，body 由契約層產生", () => {
	const start = hook.indexOf("const executeRfqPublication");
	assert.notEqual(start, -1);
	const body = hook.slice(start, start + 3000);

	assert.match(
		body,
		/rfq-publications`,\n\s*\{ json: publicationExecuteBody\(command\) \}/,
	);
	assert.doesNotMatch(body, /rfq-publications\/preview/);
});

test("executeRfqPublication 對回傳 fail closed，不把無法解析當成已發布", () => {
	const start = hook.indexOf("const executeRfqPublication");
	const body = hook.slice(start, start + 3000);

	assert.match(body, /rfqPublicationResultSchema\.safeParse\(env\.data\)/);
	assert.match(body, /INVALID_RESPONSE/);
	assert.match(body, /if \(!parsed\.success\) \{[\s\S]*?ok: false/);
});

test("executeRfqPublication 沿用 preview 的 STALE_CONTEXT 守衛", () => {
	const start = hook.indexOf("const executeRfqPublication");
	const body = hook.slice(start, start + 3000);

	assert.match(body, /current\.revisionId !== command\.expectedRevisionId/);
	assert.match(body, /STALE_CONTEXT/);
	assert.match(body, /sessionGenerationGuard\.capture\(requestId\)/);
});
