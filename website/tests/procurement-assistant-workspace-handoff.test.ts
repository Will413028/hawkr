import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const source = readFileSync(
	path.join(root, "features/procurement-assistant/AssistantWorkspace.tsx"),
	"utf8",
);
const taskCard = readFileSync(
	path.join(root, "features/procurement-assistant/AgentTaskCard.tsx"),
	"utf8",
);

test("a fresh session (no initialRequestId) consumes the hawkr-agent handoff draft once and passes it to the composer", () => {
	assert.match(
		source,
		/import \{ consumeHawkrAgentHandoff \} from "\.\/handoff";/,
	);
	assert.match(
		source,
		/if \(initialRequestId \|\| initialConversationId \|\| consumedHandoffRef\.current\)/,
	);
	assert.match(source, /consumeHawkrAgentHandoff\(\)/);
	// 草稿現在由 workspace 持有（設計書 v6 §V.11.0「不生成第二套 composer」），
	// 所以 handoff 直接寫進那一份 state，而不是再往下傳一個 initialDraft prop
	// 讓 composer 自己複製一份。只有桌面可 autofocus；mobile 不主動叫出鍵盤。
	assert.match(source, /setDraft\(handoff\.query\);/);
	assert.match(
		source,
		/shouldAutofocusHandoff\(window\.matchMedia\?\.bind\(window\)\)/,
	);
	assert.match(source, /composerInputRef\.current\?\.focus\(\)/);
});

test("prompt 在首個 public step／成功結果前不清除；錯誤保留原文與 exact-retry key", () => {
	const submitStart = source.indexOf("const submitTurnFromComposer");
	const submitEnd = source.indexOf("const handleDraftChange", submitStart);
	const submit = source.slice(submitStart, submitEnd);
	assert.doesNotMatch(submit, /setDraft\(""\)/);
	assert.match(source, /session\.pendingTurn \|\| !session\.turnStartObserved/);
	assert.match(source, /if \(!session\.error\) return;/);
	assert.match(
		source,
		/pendingRequestKeyRef\.current = submission\.clientRequestId/,
	);
	assert.match(source, /prepareTurnSubmission\(previous, message/);
});

test("liveSteps 的 code/status/duration 參與 transcript auto-scroll trigger", () => {
	assert.match(source, /const liveStepVersion = session\.liveSteps/);
	assert.match(source, /step\.code.*step\.status.*step\.durationMs/s);
	assert.match(source, /const transcriptVersion = .*\$\{liveStepVersion\}/);
});

test("demo conversation 有 conversationId，因此永遠不進新對話 Entry", () => {
	assert.match(source, /const showEntry = shouldShowAgentEntry\(\{/);
	assert.match(source, /conversationId,/);
});

test("a restored session (initialRequestId set) never consumes the handoff draft", () => {
	assert.match(
		source,
		/if \(initialRequestId \|\| initialConversationId \|\| consumedHandoffRef\.current\)/,
	);
});

test("跳過按鈕的可見性交給後端 wire 欄位 canSkip——取代前端自己複製 QuoteRequiredOptionalSlots 規則的舊 canSkipSlot", () => {
	assert.match(source, /onSkipOption=\{\s*session\.question\?\.canSkip/);
	assert.match(source, /void session\.submitAnswer\(payload\)/);

	// canSkip 現在完全由後端算好回傳（backend-payuni#2442 §6：
	// !IsRequiredSlot(key) && !QuoteRequiredOptionalSlots[key]），前端不得再
	// 維護一份會漂移的複製規則。
	const logic = readFileSync(
		path.join(root, "features/procurement-assistant/session-logic.ts"),
		"utf8",
	);
	assert.doesNotMatch(logic, /QUOTE_REQUIRED_OPTIONAL_SLOTS/);
	assert.doesNotMatch(logic, /canSkipSlot/);
});

test("onPickUnit 交給 session.submitAnswer——跟 onPickOption/onSkipOption 同一條回呼路徑", () => {
	assert.match(
		source,
		/onPickUnit=\{\(payload\) => void session\.submitAnswer\(payload\)\}/,
	);
});

test("複合問題卡活躍時底部 composer 不走 submitAnswer——缺單位會 422，改走一般 /turns 讓 LLM 從自由句子抽取", () => {
	assert.match(
		source,
		/import \{[\s\S]*questionHasUnitOptions,[\s\S]*\} from "\.\/session-logic";/,
	);
	assert.match(
		source,
		/const questionNeedsUnitPick = questionHasUnitOptions\(session\.question\);/,
	);
	assert.match(
		source,
		/composerAnswerMode && session\.question\?\.allowFreeText/,
	);
});

test("REQUIREMENT_READY 以真實 candidate-search 任務卡交接，不再只丟一顆孤立按鈕", () => {
	assert.match(
		source,
		/<AgentTaskCard[\s\S]*?onAssign=\{session\.startSearch\}[\s\S]*?\/>/,
	);
	// 問題還沒答完不派工：任務卡與問題卡不並存（否則等於在問題未答完時就
	// 誘導使用者跳過剩下的補充）。
	assert.match(
		source,
		/session\.stage === "ready" &&\s*!session\.question &&\s*!isProcessing &&\s*!session\.pendingInterrupt \? \(\s*<AgentTaskCard/,
	);
	assert.match(taskCard, /搜尋候選供應商/);
	assert.match(taskCard, /平台賣家/);
	assert.match(taskCard, /外部業者/);
	assert.match(taskCard, /只整理候選，不會替你下單/);
	assert.match(taskCard, /isLoading=\{assigning\}/);
});

test("active question 讓底部 composer 進入 200 字 answer mode，文案直接接續後端 prompt——但 quantity 複合問題卡活躍時不進入這個 mode", () => {
	// composerAnswerMode 只定義一次，maxLength/placeholder/validate/onSubmit
	// 四處都讀同一個旗標——不各自複製一份「session.question && !questionNeedsUnitPick」。
	assert.match(
		source,
		/const composerAnswerMode =\s*canClarify &&\s*Boolean\(session\.question\) &&\s*!questionNeedsUnitPick;/,
	);
	assert.match(source, /maxLength=\{composerAnswerMode \? 200 : undefined\}/);
	assert.match(source, /`直接回答：\$\{session\.question\.prompt\}/);
	assert.match(source, /validateFreeTextAnswer/);
});
