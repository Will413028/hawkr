import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const zhMessages = JSON.parse(read("messages/zh-TW.json")) as {
	procurementAssistant: Record<string, string>;
};

const browser = read("features/procurement-assistant/CandidateBrowser.tsx");
const canvas = read("features/procurement-assistant/CandidateCanvas.tsx");
const draftFlow = read("features/procurement-assistant/RfqDraftFlow.tsx");
const workspace = read("features/procurement-assistant/AssistantWorkspace.tsx");
const chat = read("features/procurement-assistant/ProcurementClarifyChat.tsx");
const panel = read("features/procurement-assistant/AgentSidePanel.tsx");
const sessionLogic = read("features/procurement-assistant/session-logic.ts");
const css = read("styles/features/procurement-assistant.css");

test("assistant workspace uses the signed-off 55/45 shell and collapses without horizontal overflow", () => {
	assert.match(
		css,
		/grid-template-columns:\s*minmax\(0, 55fr\) minmax\(320px, 45fr\)/,
	);
	assert.match(
		css,
		/@media \(max-width: 1199px\)[\s\S]*?\.thread-page[\s\S]*?display: none/,
	);
	assert.match(css, /container-type:\s*inline-size/);
	assert.match(css, /@container \(max-width: 620px\)/);
	assert.match(css, /grid-template-areas:[\s\S]*?"status status"/);
	assert.match(css, /env\(safe-area-inset-bottom, 0px\)/);
	assert.match(
		css,
		/\.assistant-candidate-row__facts span,[\s\S]*?\.assistant-candidate-row__pending span,[\s\S]*?\.assistant-candidate-row__date time\s*\{[\s\S]*?min-width:\s*0;[\s\S]*?overflow-wrap:\s*anywhere;/,
	);
});

test("candidate workspace renders a semantic fact table from only current DTO fields", () => {
	assert.match(
		browser,
		/<table[\s\S]*?aria-label=\{t\("candidateTableLabel"\)\}/,
	);
	assert.equal(zhMessages.procurementAssistant.candidateTableLabel, "候選資料");
	for (const heading of ["候選", "符合條件", "待確認", "資料時間", "狀態"]) {
		assert.match(browser, new RegExp(`<th scope="col">${heading}</th>`));
	}
	assert.match(browser, /candidate\.matchedFacts/);
	assert.match(browser, /candidate\.unverifiableFacts/);
	assert.doesNotMatch(
		browser,
		/candidate\.(?:matched|unverifiable)Facts\.slice/,
	);
	assert.match(browser, /candidate\.factsAsOf/);
	assert.match(browser, /待進一步查核/);
	assert.doesNotMatch(browser, /待商家確認/);
	assert.doesNotMatch(browser, /外部業者|EXTERNAL|ProcurementLane/);
	assert.doesNotMatch(browser, /供應量|配送條件/);
	assert.doesNotMatch(browser, /candidate\.sellerId|\bsellerId\b/);
});

/**
 * 這一則取代了原本的 `doesNotMatch(browser, /sellerId|sellerName/)`。
 *
 * 舊斷言的用意是「backend 還沒給資料，前端就不准假裝有商家」——那個前提已由
 * backend #2377（PR #2702）解除。但它守的那個約定（**不做「看起來有、點進去空」的
 * 東西**）仍然成立，只是判準要從「不准出現這些欄位」換成「收斂必須由資料決定」。
 *
 * 所以現在釘的是：候選收斂成商家沒有任何模式開關，唯一入口是 canGroupBySeller 的
 * 資料判準。這同時涵蓋 backend 尚未部署的空窗期與部署窗口內新舊資料混雜。
 */
test("seller rollup is data-driven, never a toggle that can be empty", () => {
	assert.doesNotMatch(canvas, /previewAvailable|MODE_ITEMS|候選檢視模式/);
	assert.doesNotMatch(browser, /previewAvailable|MODE_ITEMS|候選檢視模式/);
	assert.doesNotMatch(browser, /groupingEnabled|showSellerGroups/);

	assert.match(browser, /canGroupBySeller\(visibleCandidates\)/);
	assert.match(
		browser,
		/\?\s*groupPlatformCandidates\(visibleCandidates\)\s*:\s*null/,
	);

	assert.match(browser, /visibleCandidates\.map\(renderRow\)/);

	assert.match(browser, /grouped\.ungrouped\.map\(renderRow\)/);

	assert.match(browser, /group\.storeCode \? \(/);
	assert.doesNotMatch(browser, /href=\{`\/sellers\/\$\{group\.storeId\}`\}/);
});

/**
 * 候選的單位是**收件人**不是品項：收斂成商家之後不得再顯示商品層的東西。
 *
 * 這一則是為了擋住「順手把商品名／縮圖／標價加回來」——那些欄位後端刻意沒有交付
 * （backend PR #2702 的說明），而價格尤其不能從這裡長出來：買家該看到的是自己設定的
 * 希望詢價區間（#2403），不是某個平台商品的標價。
 */
test("seller rows carry recipient identity only, never product-level price", () => {
	assert.doesNotMatch(browser, /標價|成交價/);
	assert.doesNotMatch(browser, /priceMin|priceMax|listed-price/);
	assert.match(browser, /正式報價才代表賣家願意承接/);
	assert.doesNotMatch(browser, /imageUrl|imageKey|candidate-row__thumb/);

	assert.match(browser, /項符合的商品/);

	assert.match(browser, /candidatesToToggleForSeller\(group\)\.candidates/);
	assert.doesNotMatch(browser, /toggleSellerSelection|bulkSelection/);

	assert.match(browser, /indeterminate = selection === "partial"/);
	assert.match(
		css,
		/\.assistant-candidate-row__selection-mark\[data-partial\]/,
	);
});

test("canvas controls and candidate selection expose 44px focusable targets", () => {
	assert.doesNotMatch(css, /\.hawkr-agent-canvas__close/);
	assert.doesNotMatch(canvas, /hawkr-agent-canvas__close/);
	assert.match(css, /\.hawkr-agent-panel__close \{[\s\S]*?min-height: 44px;/);
	for (const selector of [
		".hawkr-agent-canvas__back",
		".assistant-candidate-row__select-control",
	]) {
		const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
		assert.match(
			css,
			new RegExp(
				`${escaped}[\\s\\S]*?width:\\s*44px;[\\s\\S]*?height:\\s*44px;`,
			),
		);
	}
	assert.match(
		css,
		/select-control:has\(input:focus-visible\)[\s\S]*?focus-ring/,
	);
});

test("one in-flight candidate mutation disables only that candidate checkbox", () => {
	assert.match(browser, /pendingSelectionCandidateIds:\s*ReadonlySet<string>/);
	assert.match(
		browser,
		/pending=\{pendingSelectionCandidateIds\.has\([\s\S]*?candidate\.candidateId/,
	);
	assert.match(browser, /disabled=\{disabled \|\| pending\}/);
	assert.match(browser, /aria-busy=\{pending \|\| undefined\}/);
	assert.match(
		browser,
		/selectedCount === 0 \|\|[\s\S]*?candidatesStale \|\|[\s\S]*?pendingSelectionCandidateIds\.size > 0/,
	);
	assert.match(workspace, /session\.pendingSelectionCandidateIds/);
});

test("成果才開側欄，收起與改需求都有焦點返回", () => {
	assert.match(panel, /id="hawkr-agent-side-panel"/);
	assert.match(workspace, /if \(!hasResults\) \{\s*setPanelOpen\(false\)/);
	assert.match(workspace, /addEventListener\("change", openOnDesktop\)/);
	assert.match(workspace, /!panel\.hidden/);
	assert.match(workspace, /!heading\.closest\("\[hidden\]"\)/);
	assert.match(workspace, /ref=\{panelTriggerRef\}/);
	assert.match(workspace, /aria-expanded=\{resultsVisible\}/);
	assert.match(workspace, /panelTriggerRef\.current\?\.focus/);
	assert.match(workspace, /onReturnToConversation=\{returnToConversation\}/);
	assert.match(workspace, /composerInputRef\.current\?\.focus/);
	assert.match(workspace, /hidden=\{!resultsVisible\}/);
});

test("candidate rows have no lane vocabulary, and the narrow card lays out name and match count", () => {
	assert.doesNotMatch(workspace, /LANE_LABEL|candidate\.lane/);
	assert.doesNotMatch(workspace, /"平台賣家"|"外部業者"|"平台來源"|"外部來源"/);

	assert.match(css, /@container \(max-width: 620px\)/);
	assert.match(css, /grid-template-areas:\s*\n\s*"name"\s*\n\s*"matches"/);

	assert.match(
		css,
		/\.assistant-candidate-row__meta \{\s*display: contents;\s*\}/,
	);

	assert.match(css, /__meta > \* \+ \*::before \{\s*content: "·";/);
	assert.match(css, /__meta > \* \+ \*::before \{\s*content: none;\s*\}/);
});

test("row actions live in their own cell so the narrow card can put them side by side", () => {
	assert.match(browser, /<td className="assistant-candidate-row__actions">/);
	assert.match(browser, /<th scope="col">操作<\/th>/);
	for (const cell of browser.split(
		'<td className="assistant-candidate-row__',
	)) {
		if (!cell.startsWith("identity")) continue;
		const body = cell.slice(0, cell.indexOf("</td>"));
		assert.ok(
			!body.includes("__details"),
			"查看詳情不得留在 identity 儲存格內——巢狀元素排不進祖先的 grid",
		);
	}

	assert.match(css, /grid-template-areas:[\s\S]*?"select actions";/);

	assert.match(browser, /type="checkbox"/);
	assert.match(browser, /indeterminate/);

	assert.match(browser, /className="assistant-candidate-row__select-text"/);
	assert.doesNotMatch(browser, /<span className="sr-only">/);
	for (const prop of [
		"position: static",
		"clip: auto",
		"clip-path: none",
		"width: auto",
		"height: auto",
	]) {
		assert.match(
			css,
			new RegExp(
				`__select-text \\{[\\s\\S]*?${prop.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")};`,
			),
		);
	}
});

test("右欄只含成果，思考固定留在主對話", () => {
	assert.match(
		workspace,
		/const hasResults = hasProcurementResults\(session\);/,
	);
	assert.match(workspace, /const resultsVisible = hasResults && panelOpen/);
	assert.match(workspace, /\{hasResults \? \(\s*<AgentSidePanel/);
	assert.match(
		workspace,
		/\{hasResults \? \(\s*<button\s+ref=\{panelTriggerRef\}/,
	);
	assert.match(
		sessionLogic,
		/export function hasProcurementResults\([\s\S]*?\(state\.stage === "candidates" && state\.candidateSet\) \|\|\s*state\.publishedPublication/,
	);
	assert.doesNotMatch(workspace, /activeTab=|tasks=|results=|setActiveTab/);
	assert.doesNotMatch(panel, /AgentTaskPanel|role="tab"/);
	assert.match(workspace, /processing=\{\s*<AgentTaskPanel/);
	assert.match(chat, /hawkr-agent-processing-row/);
});

/**
 * v6 稿 02 的候選卡視覺（A2）。
 *
 * 這一組釘的是「兩條 lane 在卡片上分得出來」與「命中的條件看得出是哪一種」——
 * 都是視覺承載的資訊，沒有任何一條是靠文案講出來的，所以退化了不會有其他測試轉紅。
 */
test("候選卡用店面圖示與逐列 glyph 承載資訊，不靠讀文字分辨", () => {
	assert.match(browser, /function CandidateIdentity\(/);
	assert.match(browser, /<AppIcon name="home" size=\{19\} \/>/);
	assert.doesNotMatch(browser, /assistant-candidate-seller/);

	assert.match(
		browser,
		/const FACT_GLYPH_BY_SLOT: Record<string, AppIconName>/,
	);
	for (const slot of [
		"item",
		"quantity",
		"unit",
		"packaging",
		"budget_per_unit",
		"delivery_city",
		"delivery_date",
		"exact_match_required",
	]) {
		assert.match(browser, new RegExp(`\\b${slot}: "`));
	}
	assert.match(browser, /FACT_GLYPH_BY_SLOT\[key\] \?\? "check"/);
	assert.match(browser, /name=\{factGlyph\(fact\.key\)\}/);
	assert.match(browser, /name="warning"/);
});

test("候選卡沒有 lane 標籤與比對分數", () => {
	assert.doesNotMatch(
		browser,
		/data-lane|CandidateScore|assistant-candidate-row__lane|assistant-candidate-row__score/,
	);
	assert.doesNotMatch(
		css,
		/assistant-candidate-row__lane|assistant-candidate-row__score|data-lane/,
	);
});

test("候選名單是單一面板，沒有來源分頁", () => {
	assert.doesNotMatch(browser, /<Tabs|variant="solid"|laneQueries|activeLane/);
	assert.doesNotMatch(css, /assistant-tab-label/);
	assert.doesNotMatch(browser, /平台來源|外部來源/);
});

test("底部行動列講「家」，跟畫面上被勾起來的列數對得起來", () => {
	assert.match(browser, /countSelectedRecipients\(selectedCandidates\)/);
	assert.match(browser, /`已選 \$\{selectedRecipientCount\} 家`/);
	assert.doesNotMatch(browser, /筆候選已加入比較/);
	assert.match(browser, /selectedCount === 0 \|\|/);
});

test("詢價已建立那一格的入口判準是發布事實，不是 legacy 批次", () => {
	assert.doesNotMatch(canvas, /quoteBatch|QuoteBatch(?!Summary)/);

	assert.match(draftFlow, /if \(outcome\.ok\) onPublicationExecuted\?\.\(\);/);
	assert.match(
		canvas,
		/onPublicationExecuted=\{\(\) => setPublicationExecuted\(true\)\}/,
	);

	assert.match(
		canvas,
		/const published =\s*publicationExecuted \|\| publishedPublication !== null;/,
	);

	assert.match(
		canvas,
		/if \(!publicationExecuted\) return;\n\s*setWorkspaceView\("quote-invites"\);\n\s*\}, \[publicationExecuted\]\);/,
	);

	assert.match(
		canvas,
		/if \(workspaceView !== "quote-invites" \|\| published\) return;\n\s*setWorkspaceView\("candidates"\);/,
	);
	assert.doesNotMatch(canvas, /runtime\.publication \?|!runtime\.publication/);

	assert.match(
		canvas,
		/\{published \? \(\n\s*<div className="hawkr-agent-canvas__resume">/,
	);
	assert.match(canvas, /查看詢價狀態/);
	assert.match(css, /\.hawkr-agent-canvas__resume \{/);

	assert.doesNotMatch(canvas, /撤回|撤銷/);
	assert.doesNotMatch(workspace, /撤回報價單|withdrawQuoteBatch/);
});

test("重整後沒有候選也回得去：畫布不再要求 candidateSet", () => {
	assert.match(
		canvas,
		/candidateSet:\s*\|\s*React\.ComponentProps<typeof CandidateBrowser>\["candidateSet"\]\s*\|\s*null;/,
	);
	assert.match(
		canvas,
		/React\.useState<WorkspaceView>\(\s*candidateSet \? "candidates" : "quote-invites",\s*\);/,
	);
	assert.match(canvas, /\{candidateSet \? \(\n\s*<CandidateBrowser/);
	assert.match(canvas, /\{rfqPriceRangeEnabled && candidateSet \? \(/);
	assert.match(
		canvas,
		/onViewSnapshot=\{\n\s*rfqPriceRangeEnabled && candidateSet/,
	);
});

test("接上入口沒有動到 runtime-snapshot 的惰性", () => {
	assert.match(
		canvas,
		/const showingQuoteInvites = workspaceView === "quote-invites";/,
	);
	assert.match(
		canvas,
		/useRfqRuntimeSnapshot\(\n\s*showingQuoteInvites \? conversationId : null,\n\s*showingQuoteInvites \? requestId : null,\n\s*\)/,
	);

	assert.doesNotMatch(canvas, /這一格目前沒有入口/);

	assert.match(canvas, /"quote-invites": "詢價狀態工作台"/);
	assert.match(canvas, /aria-label=\{CANVAS_LABEL\[workspaceView\]\}/);
});
