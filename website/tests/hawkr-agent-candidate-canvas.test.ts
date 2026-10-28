import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const browser = read("features/procurement-assistant/CandidateBrowser.tsx");
const canvas = read("features/procurement-assistant/CandidateCanvas.tsx");
const draftFlow = read("features/procurement-assistant/RfqDraftFlow.tsx");
const workspace = read("features/procurement-assistant/AssistantWorkspace.tsx");
const chat = read("features/procurement-assistant/ProcurementClarifyChat.tsx");
const panel = read("features/procurement-assistant/AgentSidePanel.tsx");
const sessionLogic = read("features/procurement-assistant/session-logic.ts");
const css = read("styles/features/procurement-assistant.css");

test("assistant workspace uses the signed-off 55/45 shell and collapses without horizontal overflow", () => {
	// 面板常駐後 clarifying 也吃這個比例，所以不能再用「為寬候選表格挑的」38/62
	// ——那會讓對話在它最主要的階段只剩 38%。
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
		/<table[\s\S]*?aria-label=\{`\$\{LANE_LABEL\[lane\]\}候選資料`\}/,
	);
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
	assert.match(browser, /PLATFORM: "平台賣家"/);
	assert.match(browser, /EXTERNAL: "外部業者"/);
	// 候選卡仍然只講 server 已提供的事實：供應量／配送條件從來沒有進過 DTO，
	// 不得由前端推導或用商品欄位冒充。
	assert.doesNotMatch(browser, /供應量|配送條件/);
	// storeId 是 store-stint surrogate，不是 sellerId（backend PR #2702 三層同名）。
	// 用錯名字會在 tsc 過關的情況下永遠讀到 undefined，分組靜默失效。
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
 * 資料判準。這同時涵蓋 backend 尚未部署的空窗期、部署窗口內新舊資料混雜、以及
 * EXTERNAL lane（站外業者沒有平台賣家）。
 */
test("seller rollup is data-driven, never a toggle that can be empty", () => {
	// 沒有模式切換器：舊的 previewAvailable/MODE_ITEMS 不得回來，也不得換個名字重生。
	assert.doesNotMatch(canvas, /previewAvailable|MODE_ITEMS|候選檢視模式/);
	assert.doesNotMatch(browser, /previewAvailable|MODE_ITEMS|候選檢視模式/);
	assert.doesNotMatch(browser, /groupingEnabled|showSellerGroups/);

	// 唯一入口是資料判準，而且收斂結果必須來自它。
	assert.match(browser, /canGroupBySeller\(visibleCandidates\)/);
	assert.match(
		browser,
		/\?\s*groupPlatformCandidates\(visibleCandidates\)\s*:\s*null/,
	);

	// 收斂不了時走的是原本那張候選平表，不是空狀態。
	assert.match(browser, /visibleCandidates\.map\(renderRow\)/);

	// 帶不出商家身分的候選必須照樣列出，不得被靜默丟掉。
	assert.match(browser, /grouped\.ungrouped\.map\(renderRow\)/);

	// storeCode 缺值不得退回 storeId 組連結（那條路徑正在被收掉）。
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
	// 不得從候選資料渲染出任何價格數字。判準是「價格欄位與標價用詞」，不是「報價」二字——
	// 既有的免責文案「正式報價才代表賣家願意承接」本身是正確的，而且必須留著。
	assert.doesNotMatch(browser, /標價|成交價/);
	assert.doesNotMatch(browser, /priceMin|priceMax|listed-price/);
	assert.match(browser, /正式報價才代表賣家願意承接/);
	// 商品縮圖同理：EXTERNAL lane 根本沒有商品，做了對主要 lane 也是空的。
	assert.doesNotMatch(browser, /imageUrl|imageKey|candidate-row__thumb/);

	// 商家列講的是「幾項符合的商品」（一個計數），不是把商品列出來。
	assert.match(browser, /項符合的商品/);

	// 勾選一家＝勾選它底下全部候選，且走既有的 per-candidate 契約，不新開批次端點。
	assert.match(browser, /candidatesToToggleForSeller\(group\)\.candidates/);
	assert.doesNotMatch(browser, /toggleSellerSelection|bulkSelection/);

	// 部分勾選要用 indeterminate 表達，不能顯示成完全未選。
	assert.match(browser, /indeterminate = selection === "partial"/);
	assert.match(
		css,
		/\.assistant-candidate-row__selection-mark\[data-partial\]/,
	);
});

test("canvas controls and candidate selection expose 44px focusable targets", () => {
	// 「回對話」歸面板管（hawkr-agent-panel__close），canvas 自己那顆已下架，
	// 連同它的 CSS 一起——留著就是指向不存在元素的死規則。
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

test("lane vocabulary has exactly one source, and the narrow card lays it out per the mockup", () => {
	// 候選詳情面板原本自己寫死一組同義字串（`平台來源`／`外部來源`），清單改了
	// 詳情不會跟著改——這支就是那個 drift 的守門。斷言的是「有沒有第二份」，
	// 不是「文案是什麼」：文案本身已由上一支測試釘在 CandidateBrowser。
	assert.match(
		workspace,
		/import \{ LANE_LABEL \} from "\.\/CandidateBrowser"/,
	);
	assert.match(workspace, /LANE_LABEL\[candidate\.lane\]/);
	assert.doesNotMatch(workspace, /"平台賣家"|"外部業者"|"平台來源"|"外部來源"/);

	// 窄容器（畫布可能是窄分割面板）把同一組語意列排成卡片。比對分數要自成一個
	// grid area 貼右欄，不能留在 meta 那串文字裡——否則卡片右上角是空的。
	assert.match(css, /@container \(max-width: 620px\)/);
	assert.match(css, /grid-template-areas:\s*\n\s*"name score"/);
	assert.match(
		css,
		/\.assistant-candidate-row__score \{[\s\S]*?grid-area: score;/,
	);

	// `__meta` 必須 display: contents，它的子元素才會成為 identity 的 grid item。
	// 少了這一條，上面那些 grid-area 全部落空、版面靜默退回舊樣式。
	assert.match(
		css,
		/\.assistant-candidate-row__meta \{\s*display: contents;\s*\}/,
	);

	// 寬版不得受影響：分隔點由 ::before 生成，窄版才關掉。斷言的是兩邊各自的
	// content 值——只斷言選擇器存在沒有鑑別力，因為窄版區塊裡有同名選擇器，
	// 拿掉寬版那條規則測試照樣會綠（本測試第一版就是這樣寫的）。
	assert.match(css, /__meta > \* \+ \*::before \{\s*content: "·";/);
	assert.match(css, /__meta > \* \+ \*::before \{\s*content: none;\s*\}/);
});

test("row actions live in their own cell so the narrow card can put them side by side", () => {
	// 查看詳情／查看商家必須是 row 層級的儲存格，不能留在 identity 裡——
	// 巢狀元素排不進祖先的 grid，留在裡面就併不到底部那一列。
	assert.match(browser, /<td className="assistant-candidate-row__actions">/);
	assert.match(browser, /<th scope="col">操作<\/th>/);
	// 逐格檢查而不是用一條跨檔 regex：`[\s\S]*?` 沒有邊界，會跨過不相干的區段
	// 湊出一個假的匹配（這支測試第一版就是這樣誤報的）。
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

	// 窄版底部：選取與操作並排。
	assert.match(css, /grid-template-areas:[\s\S]*?"select actions";/);

	// 選取控制維持真的 checkbox。換成 <button> 會丟掉 indeterminate（部分勾選）
	// 這個原生語意，而 SellerRow 正是靠它表達「這家商家只選了一部分候選」。
	assert.match(browser, /type="checkbox"/);
	assert.match(browser, /indeterminate/);

	// 狀態文字同一個 span 兩種可見度：寬版 visually-hidden、窄版現形當按鈕文字。
	// 窄版必須逐條抵銷寬版的隱藏宣告——少抵銷 clip-path 或 position 文字就不會出現。
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
	// 掛載、開啟鈕與可見性共用同一個 hasResults。這三處各自寫一份條件，就是
	// 「面板掛得起來、開啟鈕也在，但 hidden 永遠拿不掉」那個 bug 的成因，
	// 所以這裡釘的是「都走同一個值」，不是條件的字面寫法。
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
	// 而那個唯一判準必須吃得到已發布成果——只綁 stage === "candidates" 的話，
	// 重整後 deriveStageFromStatus 只會回 clarifying／ready，右欄永遠打不開。
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
test("候選卡用 lane 代號與逐列 glyph 承載資訊，不靠讀文字分辨", () => {
	// 兩條 lane 各自的代號圖示。缺了它，站外業者那一列左邊是空的，
	// 兩條 lane 的可信度差異在卡片上完全看不出來。
	assert.match(browser, /const LANE_AVATAR_ICON: Record<ProcurementLane/);
	assert.match(browser, /PLATFORM: "home"/);
	assert.match(browser, /EXTERNAL: "box"/);
	// 單一候選列與收斂後的商家列共用同一個身分區塊，不是只有商家列有頭像。
	assert.match(browser, /function CandidateIdentity\(/);
	assert.doesNotMatch(browser, /assistant-candidate-seller/);
	for (const call of ["lane={candidate.lane}", 'lane="PLATFORM"']) {
		assert.ok(
			browser.includes(call),
			`CandidateIdentity 必須由兩種列各自帶入 lane（缺 ${call}）`,
		);
	}

	// matchedFacts 逐列給字形：整欄同一個勾不帶任何資訊。key 的值域是後端
	// domain.SlotKey，值域外退回勾（B2 擴充 slot 時多出來的列仍要列得出來）。
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
	// 換掉的是 matchedFacts；unverifiableFacts 仍然一律是警示符號。
	assert.match(browser, /name="warning"/);
});

test("lane badge 與比對分數方塊在所有寬度都成立，不只窄容器", () => {
	// 這兩塊原本只寫在 @container 裡，寬版把 lane 印成沒有顏色的裸文字。
	// 判準是「規則在容器查詢之前就出現」——只斷言選擇器存在沒有鑑別力，
	// 窄版區塊裡有同名選擇器。
	// 錨在 at-rule 本身（前導換行＋大括號），不是它的字面字串——這個檔案的註解
	// 也提到同一句話，用字串會切在註解上，寬版那半段跟著整個消失。
	const narrowStart = css.indexOf("\n@container (max-width: 620px) {");
	assert.ok(narrowStart > 0);
	const wide = css.slice(0, narrowStart);
	assert.match(
		wide,
		/\.assistant-candidate-row__lane \{[\s\S]*?border-radius: var\(--radius-pill\);/,
	);
	assert.match(
		wide,
		/\.assistant-candidate-row__lane\[data-lane="EXTERNAL"\] \{[\s\S]*?--hkui-warning/,
	);
	// 顏色鍵在 badge 自己的 data-lane 上，不鍵在祖先的 lane 面板上——
	// 判準的來源是那一列候選本身。
	assert.doesNotMatch(
		css,
		/\.assistant-lane\[data-lane="EXTERNAL"\] \.assistant-candidate-row__lane/,
	);
	assert.match(
		browser,
		/className="assistant-candidate-row__lane"\s*\n?\s*data-lane=/,
	);

	// 分數方塊的「比對」與「94 分」要是具名子元素：留成裸文字節點時，窄版那個
	// grid 會把它們拆成各自佔一列的匿名 item，「94」與「分」上下分家。
	assert.match(browser, /function CandidateScore\(/);
	assert.match(browser, /assistant-candidate-row__score-label/);
	assert.match(browser, /assistant-candidate-row__score-value/);
	assert.doesNotMatch(browser, /比對 <b>/);
});

test("lane 分頁是實心膠囊，且沿用唯一那份 lane 詞彙", () => {
	assert.match(browser, /variant="solid"/);
	assert.doesNotMatch(browser, /variant="underlined"/);
	// 選中態要由主色實心承擔：solid variant 自己只換成 elevated 表面，
	// 在這個工作台裡跟未選中幾乎分不出來。
	assert.match(
		css,
		/\[role="tablist"\] button\[data-selected="true"\] \{[\s\S]*?background: var\(--theme-action-primary\);[\s\S]*?color: var\(--hkui-primary-foreground\);/,
	);
	// 分頁文字用 LANE_LABEL，不再另寫一組同義字。
	assert.match(
		browser,
		/\{LANE_LABEL\.PLATFORM\} <b>\{platform\.length\}<\/b>/,
	);
	assert.match(
		browser,
		/\{LANE_LABEL\.EXTERNAL\} <b>\{external\.length\}<\/b>/,
	);
	assert.doesNotMatch(browser, /平台來源|外部來源/);
});

test("底部行動列講「家」，跟畫面上被勾起來的列數對得起來", () => {
	// 同一家商家底下勾三筆在畫面上是一列、打出去是三次 POST .../selections。
	// 沿用候選筆數會報成 3，跟使用者看到的對不上。
	assert.match(browser, /countSelectedRecipients\(selectedCandidates\)/);
	assert.match(browser, /`已選 \$\{selectedRecipientCount\} 家`/);
	assert.doesNotMatch(browser, /筆候選已加入比較/);
	// 按鈕能不能按仍然看候選筆數（一筆都沒有就不能）。
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
	// 兩個 id 仍然只在檢視真的展開時才交出去：hook 拿到 null 就完全不發請求。
	// 「先載好」的背景預取正是這條斷言在擋的——而入口判準刻意不倚賴這支 hook
	// 的結果，否則「要先進得去才知道進不進得去」的唯一解法就是改成 eager。
	assert.match(
		canvas,
		/const showingQuoteInvites = workspaceView === "quote-invites";/,
	);
	assert.match(
		canvas,
		/useRfqRuntimeSnapshot\(\n\s*showingQuoteInvites \? conversationId : null,\n\s*showingQuoteInvites \? requestId : null,\n\s*\)/,
	);

	// 那句「這一格目前沒有入口」在入口接上之後就是假的了。註解會被讀成現況，
	// 留著比沒有更糟。
	assert.doesNotMatch(canvas, /這一格目前沒有入口/);

	// 三個檢視各自有名字：quote-invites 原本落在「候選資料工作台」那一支，
	// 走不到時看不出來，走得到就是螢幕閱讀器上的錯標。
	assert.match(canvas, /"quote-invites": "詢價狀態工作台"/);
	assert.match(canvas, /aria-label=\{CANVAS_LABEL\[workspaceView\]\}/);
});

test("external candidates render their own brand logo without borrowing platform seller identity", () => {
	// 單一候選那一列（EXTERNAL lane 走這條）要把站外業者自己的品牌圖傳進去，
	// 否則 446 家有 logo 的業者在候選卡上永遠只看得到 lane 代號圖示。
	assert.match(browser, /logoUrl=\{candidate\.brandLogoUrl\}/);

	// 但**不得** fallback 到 candidate.logoUrl：那一欄是平台商家的身分快照，
	// 混用會讓站外業者在畫面上看起來像已入駐的平台商家。
	assert.doesNotMatch(
		browser,
		/candidate\.brandLogoUrl\s*\?\?\s*candidate\.logoUrl/,
	);
	assert.doesNotMatch(
		browser,
		/candidate\.logoUrl\s*\?\?\s*candidate\.brandLogoUrl/,
	);

	// 分組仍只看 storeId——品牌圖不得變成分組或身分判準。
	assert.doesNotMatch(
		read("features/procurement-assistant/candidate-grouping.ts"),
		/brandLogo|brandCover/,
	);
});
