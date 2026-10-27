import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const readSource = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

const candidateBrowser = readSource(
	"features/procurement-assistant/CandidateBrowser.tsx",
);
const candidateCanvas = readSource(
	"features/procurement-assistant/CandidateCanvas.tsx",
);
const assistantWorkspace = readSource(
	"features/procurement-assistant/AssistantWorkspace.tsx",
);
const rfqDraft = readSource("features/procurement-assistant/RfqDraftFlow.tsx");
const sessionHook = readSource(
	"features/procurement-assistant/useProcurementSession.ts",
);
const priceContract = readSource(
	"features/procurement-assistant/rfq-price-contract.ts",
);
const procurementCss = readSource("styles/features/procurement-assistant.css");

test("selected candidates open the typed RFQ price confirmation inside the workspace", () => {
	assert.match(candidateBrowser, /建立詢價草稿/);
	assert.match(candidateBrowser, /onStartRfqDraft\(selectedCandidates\)/);
	assert.match(candidateCanvas, /<RfqDraftFlow/);
	assert.match(candidateCanvas, /onPreviewRfqPrice=\{onPreviewRfqPrice\}/);
	assert.match(
		candidateCanvas,
		/onConfirmRfqSnapshot=\{onConfirmRfqSnapshot\}/,
	);
	assert.doesNotMatch(candidateBrowser, /hawkr-agent-rfq-preview/);
});

test("the live RFQ flow uses the new preview and confirmation endpoints, never legacy publication", () => {
	assert.match(sessionHook, /rfq-price-previews`[\s\S]*?\{ json: command \}/);
	assert.match(sessionHook, /rfq-confirmations`[\s\S]*?\{ json: command \}/);
	assert.doesNotMatch(rfqDraft, /quote-batches|onPublish|onPublished/);
	assert.doesNotMatch(rfqDraft, /apiClient|\.authed\.|fetch\(/);
	assert.match(rfqDraft, /確認並準備發布/);
	assert.match(rfqDraft, /!publicationReconfirmed/);
	assert.match(
		rfqDraft,
		/aria-describedby="assistant-rfq-publication-meaning"/,
	);
	assert.doesNotMatch(rfqDraft, /發布詢價（尚未開放）/);
});

test("guided preview omits buyerRequestedRange until the buyer edits it, and seeds the inputs from the server's prefill", () => {
	assert.match(
		rfqDraft,
		/React\.useState<RfqPricingMode>\("PRICEBOOK_GUIDED"\)/,
	);
	assert.match(
		rfqDraft,
		/pricingMode === "BUYER_ENTERED_NO_REFERENCE" \|\|[\s\S]*?pricingMode === "PRICEBOOK_GUIDED" && hasDesiredRangeInput/,
	);
	assert.match(
		rfqDraft,
		/result\.data\.pricingMode === "PRICEBOOK_GUIDED"[\s\S]*?setDesiredPriceLower\([\s\S]*?result\.data\.buyerRequestedRange\.lowerBoundAmount/,
	);
	assert.doesNotMatch(rfqDraft, /basisQuantity:\s*(?:quantity|"1")/);
	assert.doesNotMatch(rfqDraft, /basisUnit:\s*(?:unit|"KG")/);
});

test("quantity, unit, city and delivery date are persisted-slot read-only fields", () => {
	for (const label of ["需求數量", "單位", "配送城市", "交貨日期"]) {
		const field = rfqDraft.match(
			new RegExp(`<span>${label}<\\/span>[\\s\\S]*?<input[\\s\\S]*?\\/>`),
		)?.[0];
		assert.ok(field, label);
		assert.match(field, /readOnly/, label);
		assert.doesNotMatch(field, /onChange/, label);
	}
	assert.match(rfqDraft, /回到對話產生新的需求版本/);
});

test("no-reference mode is explicit and remains fail closed without a canonical server basis", () => {
	assert.match(rfqDraft, /type="radio"/);
	assert.match(rfqDraft, /改用自行設定價格/);
	assert.match(rfqDraft, /MAPPING_MISSING/);
	assert.match(rfqDraft, /hasConfirmablePriceBasis\(preview,\s*unit\)/);
	assert.match(rfqDraft, /價格資料尚未完整，或與目前需求單位不一致/);
	assert.match(priceContract, /basisUnit === "KG"/);
	assert.match(priceContract, /basisUnit === "CATTY"/);
	assert.match(priceContract, /basisUnit === "GRAM"/);
});

test("preview and confirmation exact retries reuse their stored command identity", () => {
	assert.match(
		rfqDraft,
		/reuseExistingCommand && existing\?\.fingerprint === fingerprint[\s\S]*?existing\.command/,
	);
	assert.match(
		rfqDraft,
		/previewCommandRef\.current = \{ fingerprint, command \}/,
	);
	assert.match(
		rfqDraft,
		/confirmationCommandRef\.current = \{ fingerprint, command \}/,
	);
	assert.match(rfqDraft, /以原操作重試/);
	assert.match(rfqDraft, /IDEMPOTENCY_KEY_CONFLICT/);
	const previewRfqPriceBody = sessionHook.match(
		/\tconst previewRfqPrice = React\.useCallback\([\s\S]*?\n\t\);/,
	)?.[0];
	assert.ok(
		previewRfqPriceBody,
		"previewRfqPrice not found in useProcurementSession.ts",
	);
	assert.doesNotMatch(previewRfqPriceBody, /randomUUID/);
});

test("confirmation is fail-closed and locks navigation while the mutation is in flight", () => {
	assert.match(
		rfqDraft,
		/!approvalEnabled \|\|[\s\S]*?!snapshotReconfirmed \|\|[\s\S]*?confirmationState\.status === "success"/,
	);
	assert.match(rfqDraft, /confirmationState\.status === "idle"/);
	assert.match(
		rfqDraft,
		/onInteractionLockChange\?\.\(confirmationState\.status === "loading"\)/,
	);
	assert.match(candidateCanvas, /disabled=\{rfqInteractionLocked\}/);
	assert.match(
		candidateCanvas,
		/onInteractionLockChange=\{setRfqInteractionLocked\}/,
	);
	assert.match(
		assistantWorkspace,
		/const resultsVisible = hasResults && panelOpen && !isProcessing/,
	);
	assert.match(assistantWorkspace, /hidden=\{!resultsVisible\}/);
});

test("confirmation echoes preview identity and hash but keeps the confirmed hash independent", () => {
	assert.match(rfqDraft, /pricePreviewId: preview\.pricePreviewId/);
	assert.match(rfqDraft, /pricePreviewVersion: preview\.version/);
	assert.match(rfqDraft, /payloadHash: preview\.payloadHash/);
	assert.match(
		priceContract,
		/export const rfqConfirmationSchema = z\.object\(\{[\s\S]*?payloadHash: payloadHashSchema/,
	);
	assert.match(rfqDraft, /\? \{ status: "success", data: result\.data \}/);
	assert.doesNotMatch(
		rfqDraft,
		/confirmed\.payloadHash\s*===\s*preview\.payloadHash/,
	);
});

test("runtime parsing and stale-session guards fail closed before UI state mutation", () => {
	assert.match(sessionHook, /rfqPricePreviewSchema\.safeParse\(env\.data\)/);
	assert.match(sessionHook, /rfqConfirmationSchema\.safeParse\(env\.data\)/);
	assert.match(sessionHook, /code: "INVALID_RESPONSE"/);
	assert.match(sessionHook, /stateRef\.current\.revisionId === revisionId/);
	assert.match(sessionHook, /code: "STALE_CONTEXT"/);
	assert.match(sessionHook, /RESOURCE_NOT_FOUND/);
	assert.match(sessionHook, /error\.status === 408 \|\| error\.status === 429/);
});

test("quality context remains server-owned and confirmation success does not claim publication", () => {
	assert.doesNotMatch(
		rfqDraft,
		/BUYER_QUALITY_UNKNOWN|data-quality-state|品相狀態|品相未知狀態|以買家品相未知保存/,
	);
	assert.match(rfqDraft, /需求版本與價格範圍已固定，等待發布。/);
	assert.match(rfqDraft, /這次確認不包含候選名單、截止時間與通知設定/);
	assert.doesNotMatch(rfqDraft, /候選名單已固定|通知已排入處理/);
});

test("request or revision remounts price proof while candidate changes only return to the candidate view", () => {
	assert.match(candidateCanvas, /key=\{rfqDraftIdentity\}/);
	assert.match(
		candidateCanvas,
		/const rfqDraftIdentity = JSON\.stringify\(\[\s*requestId \?\? null,\s*revisionId \?\? null,\s*\]\)/,
	);
	assert.match(
		candidateCanvas,
		/const candidateSelectionIdentity = JSON\.stringify/,
	);
	assert.match(
		candidateCanvas,
		/selectedCandidates\.map\(\(candidate\) => candidate\.candidateId\)\.sort\(\)/,
	);
	assert.match(
		sessionHook,
		/revisionChanged[\s\S]*?runId: null[\s\S]*?candidateSet: null/,
	);
	assert.match(rfqDraft, /clearDerivedPriceState/);
});

test("RFQ capability flags separately gate entry and confirmation", () => {
	assert.match(
		assistantWorkspace,
		/capabilities\.WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1/,
	);
	assert.match(
		assistantWorkspace,
		/capabilities\.WEB_PROCUREMENT_RFQ_APPROVAL_V1/,
	);
	assert.match(candidateCanvas, /rfqDraftEnabled=\{rfqPriceRangeEnabled\}/);
	assert.match(candidateCanvas, /approvalEnabled=\{rfqApprovalEnabled\}/);
	assert.match(rfqDraft, /approvalEnabled &&/);
});

test("new loading, error, mode, confirmed and mobile states are styled", () => {
	for (const selector of [
		"assistant-rfq-guidance__modes",
		"assistant-rfq-guidance__loading",
		"assistant-rfq-guidance__error",
		"assistant-rfq-guidance__server-range",
		"assistant-rfq-snapshot__confirmed",
	]) {
		assert.match(procurementCss, new RegExp(`\\.${selector}`), selector);
	}
	assert.match(procurementCss, /@media \(max-width: 640px\)/);
});

test("v6 稿 03 的市場價格參考與總額區間都由伺服器供給，前端不自算也不補值", () => {
	assert.match(
		rfqDraft,
		/const basis = preview\.marketPriceBasis;\s*if \(!basis\) return null;/,
	);
	assert.match(
		rfqDraft,
		/const total = preview\.quoteTotalRange;\s*if \(!total\) return null;/,
	);

	assert.match(rfqDraft, /formatTwd\(total\.lowerBoundAmount\)/);
	assert.match(rfqDraft, /formatTwd\(total\.upperBoundAmount\)/);
	assert.doesNotMatch(rfqDraft, /total\.quantity\s*\*|Number\(total\./);

	assert.match(rfqDraft, /每 \$\{basisQuantity\} \$\{basisUnit\}/);
	assert.doesNotMatch(rfqDraft, /quantityUnit:\s*"|basisUnit\s*=\s*"/);

	assert.match(rfqDraft, /formatTaipeiDate\(basis\.sourceDataAsOf\)/);
	assert.match(rfqDraft, /行情來源未提供/);

	assert.match(
		priceContract,
		/marketPriceBasis: rfqMarketPriceBasisSchema\.nullable\(\)\.default\(null\)/,
	);
	assert.match(
		priceContract,
		/quoteTotalRange: rfqQuoteTotalRangeSchema\.nullable\(\)\.default\(null\)/,
	);
	assert.match(
		priceContract,
		/\(preview\.marketPriceBasis !== null\) !== hasReference/,
	);

	for (const selector of [
		"assistant-rfq-guidance__market-basis",
		"assistant-rfq-guidance__quote-total",
	]) {
		assert.match(procurementCss, new RegExp(`\\.${selector}`), selector);
	}
});

test("A13：execute CTA 從 workspace 一路接到 RfqDraftFlow", () => {
	assert.match(
		assistantWorkspace,
		/onExecuteRfqPublication=\{[a-zA-Z.]*executeRfqPublication\}/,
	);
	assert.match(
		candidateCanvas,
		/onExecuteRfqPublication=\{onExecuteRfqPublication\}/,
	);
	assert.match(rfqDraft, /確認並送出邀請/);
});

test("A13：execute CTA 只在取得可執行核准後才存在，不是常駐 disabled", () => {
	assert.match(
		rfqDraft,
		/publicationState\.status === "success" \? \([\s\S]{0,900}?確認並送出邀請/,
	);
});

test("A13：發布結果面板有樣式", () => {
	assert.match(rfqDraft, /assistant-rfq-reconfirm__published/);
	assert.match(procurementCss, /assistant-rfq-reconfirm__published/);
});

test("A13：受眾只剩指定候選，沒有候選時 fail closed", () => {
	assert.match(rfqDraft, /const hasRecipients = candidates\.length > 0;/);
	assert.doesNotMatch(rfqDraft, /mode: "WISH_POOL"/);
	assert.doesNotMatch(rfqDraft, /radiogroup/);
	assert.match(rfqDraft, /!hasRecipients \|\|/);
	assert.match(rfqDraft, /executionState\.data\.audienceMode === "WISH_POOL"/);
});

test("回覆期限的輸入卡有對應樣式", () => {
	assert.match(rfqDraft, /assistant-rfq-reconfirm__deadline/);
	assert.match(procurementCss, /\.assistant-rfq-reconfirm__deadline/);
});

test("買家的補充說明進 confirm 的 CAS 指紋，而且不是 slot", () => {
	assert.match(priceContract, /export const BUYER_NOTE_MAX_LENGTH = 200;/);
	assert.match(priceContract, /buyerNote: string;/);

	assert.match(rfqDraft, /maxLength=\{BUYER_NOTE_MAX_LENGTH\}/);
	assert.match(rfqDraft, /placeholder="例如：中大果，不要有壓傷"/);

	assert.match(rfqDraft, /buyerNote: buyerNote\.trim\(\),/);
	const runConfirmation = rfqDraft.slice(
		rfqDraft.indexOf("const runConfirmation = React.useCallback("),
	);
	const commandInput = runConfirmation.slice(
		runConfirmation.indexOf("const commandInput = {"),
		runConfirmation.indexOf(
			"const fingerprint = JSON.stringify(commandInput);",
		),
	);
	assert.match(
		commandInput,
		/buyerNote/,
		"補充說明必須在 commandInput 裡：fingerprint 由它算出來，" +
			"漏掉的話改了說明卻沿用同一個 clientRequestId，後端會把它當冪等重播而不寫入",
	);

	assert.match(rfqDraft, /\[\s*approvalEnabled,\s*buyerNote,/);
});

test("補充說明只從輸入框來，不從對話抽取", () => {
	assert.doesNotMatch(
		sessionHook,
		/buyerNote/,
		"session hook 不該認得 buyerNote——它不是 slot、不經 LLM 抽取。" +
			"2026-08-17 移除的 spec 就是走那條路（backend #2476／website #498）",
	);
});

test("補充說明有樣式，且沒填時不畫空框", () => {
	assert.match(procurementCss, /\.assistant-rfq-snapshot__note \{/);
	assert.match(procurementCss, /\.assistant-rfq-snapshot__note-count \{/);
});
