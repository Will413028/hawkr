import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	PRE_ACK_MOTION_STATES,
	SPRITE_MOTION,
} from "../features/procurement-assistant/agent-motion";
import {
	AGENT_CAPABILITIES_OFF,
	parseAgentCapabilities,
	WEB_AGENT_FLAGS,
} from "../features/procurement-assistant/capabilities";
import {
	ENTRY_HEADING,
	flaggedCapabilityChips,
	SLICE_A_CAPABILITY_CHIPS,
} from "../features/procurement-assistant/entry-content";
import {
	prepareTurnSubmission,
	shouldShowAgentEntry,
} from "../features/procurement-assistant/session-logic";

/**
 * Hawkr Agent 新對話入口與狀態記號的**契約**測試（設計書 v6 §V.10.10、§V.11.0、
 * §V.13）。
 *
 * 這裡斷言的是設計書逐字寫下的規則，不是實作細節。每一條紅燈都代表「畫面已經
 * 與已簽核的 Screen Contract 00 脫節」，而不是「有人改了 class 名」。
 *
 * fileURLToPath 而不是 .pathname：後者會把路徑裡的空白留成 %20，於是在
 * 「Hawkr Web」這種含空白的 checkout 路徑下讀不到檔案。
 */
const root = fileURLToPath(new URL("..", import.meta.url));

function read(rel: string): string {
	return readFileSync(path.join(root, rel), "utf8");
}

/**
 * 註解要先剝掉再比對「這段程式碼裡不准出現 X」。
 *
 * 這幾支檔案的檔頭**刻意**寫出 `process.env`、`NEXT_PUBLIC_` 這些名字來解釋
 * 為什麼不用它們——那是說明不是程式碼。剝法與 `scripts/check-no-emoji.ts`
 * 同一支（區塊註解 ＋ 整行雙斜線）。
 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

const entryPanel = read("features/procurement-assistant/AgentEntryPanel.tsx");
const workspace = read("features/procurement-assistant/AssistantWorkspace.tsx");

test("§V.13：四支 Web capability flag 預設全部 OFF", () => {
	assert.equal(WEB_AGENT_FLAGS.length, 4);
	for (const flag of WEB_AGENT_FLAGS) {
		assert.equal(AGENT_CAPABILITIES_OFF[flag], false, `${flag} 預設必須是 OFF`);
	}
});

test("§V.13：只有字串 true 算開，未知來源一律補 false", () => {
	const parsed = parseAgentCapabilities({
		WEB_PROCUREMENT_AGENT_UI_V1: "true",
		WEB_PROCUREMENT_AGENT_RUNTIME_V1: "1",
		WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1: "yes",
	});
	assert.equal(parsed.WEB_PROCUREMENT_AGENT_UI_V1, true);
	// "1" 與 "yes" 都不算開——旗標的開關是 runbook 動作，寬鬆解析只會讓
	// 「我明明設了」與「它其實沒開」變成考古。
	assert.equal(parsed.WEB_PROCUREMENT_AGENT_RUNTIME_V1, false);
	assert.equal(parsed.WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1, false);
	// 完全沒出現在來源裡的旗標必須是 false，不是 undefined。
	assert.equal(parsed.WEB_PROCUREMENT_RFQ_APPROVAL_V1, false);
});

test("§V.13 規則 6：capability 不從 NEXT_PUBLIC_* 讀，client 模組不碰 process.env", () => {
	const capabilities = stripComments(
		read("features/procurement-assistant/capabilities.ts"),
	);
	assert.doesNotMatch(capabilities, /process\.env/);
	assert.doesNotMatch(capabilities, /NEXT_PUBLIC_/);
	// server 端才准讀 env，而且必須是非 NEXT_PUBLIC_ 的名字（不進 client bundle）。
	const server = read("features/procurement-assistant/capabilities.server.ts");
	assert.match(server, /import "server-only"/);
	assert.doesNotMatch(stripComments(server), /NEXT_PUBLIC_/);
});

test("§V.10.10-2：Entry IDLE 與 request 送出後、ACK 之前都不得循環", () => {
	assert.equal(SPRITE_MOTION.IDLE.loop, false);
	assert.deepEqual(PRE_ACK_MOTION_STATES, ["IDLE", "SUBMITTING"]);
	for (const state of PRE_ACK_MOTION_STATES) {
		assert.equal(
			SPRITE_MOTION[state].loop,
			false,
			`${state} 在拿到 RUN_ACCEPTED 前不得 loop`,
		);
	}
	// terminal state 也是單次播放，不得無限慶祝（§V.10.10-5）。
	assert.equal(SPRITE_MOTION.SUCCESS.loop, false);
	assert.equal(SPRITE_MOTION.RECOVERABLE_ERROR.loop, false);
	assert.equal(SPRITE_MOTION.WAITING_FOR_INPUT.loop, false);
	// RECONNECTING 不得畫成載入動態——那會讓斷線看起來像還在跑（§V.10.10-2）。
	assert.equal(SPRITE_MOTION.RECONNECTING.loop, false);
});

test("§V.10.10-5：每個 motion state 都有固定的 role=img 標籤，狀態不靠動畫傳達", () => {
	const labels = Object.values(SPRITE_MOTION).map((motion) => motion.label);
	for (const label of labels) {
		assert.match(label, /^Hawkr Agent /);
	}
	assert.equal(new Set(labels).size, labels.length, "標籤不得重複");
});

test("兩個記號元件只畫 Hawkr 標誌，不載入任何動畫資產", () => {
	for (const file of ["HawkrAgentSprite", "HawkrAgentMark"]) {
		const source = read(`features/procurement-assistant/${file}.tsx`);
		assert.match(source, /<BrandLogo variant="mark" size="fill" \/>/);
		assert.doesNotMatch(source, /@\/assets\//, `${file} 不得載入動畫資產`);
	}
});

test("§V.10.10-12：識別記號永遠中性，只有 Sprite 依 motion state 換呈現", () => {
	const mark = read("features/procurement-assistant/HawkrAgentMark.tsx");
	// Mark 不接 motion state——設計書禁止「讓目前 run 的 active／success 狀態
	// 全域套到其他歷史 conversation」，而 Mark 掛在側欄、對話列與歷史訊息上。
	assert.doesNotMatch(mark, /AgentMotionState|spriteMotion|SPRITE_MOTION/);
	assert.doesNotMatch(mark, /BrandLogoLoader/);
	// 依 state 切換靜態或載入動態的路徑只存在於 Sprite。
	const sprite = read("features/procurement-assistant/HawkrAgentSprite.tsx");
	assert.match(sprite, /spriteMotion\(state\)/);
	assert.match(sprite, /motion\.loop \? \(\s*<BrandLogoLoader size="fit" \/>/);
	assert.match(sprite, /data-motion=\{state\}/);
});

test("§V.11.0：Slice A 只顯示三個 capability chip，未啟用的能力完全不出現", () => {
	assert.deepEqual(
		SLICE_A_CAPABILITY_CHIPS.map((chip) => chip.label),
		["找供應商", "整理採購需求", "比較候選"],
	);
	// 旗標全關時不得多出任何 chip——「未啟用時完全不出現」，不是 disabled。
	assert.deepEqual(flaggedCapabilityChips(AGENT_CAPABILITIES_OFF), []);
	assert.deepEqual(
		SLICE_A_CAPABILITY_CHIPS.map((chip) => chip.icon),
		["find-suppliers", "organise-requirements", "compare-candidates"],
	);
	assert.ok(
		SLICE_A_CAPABILITY_CHIPS.every((chip) => chip.description === undefined),
		"入口能力按鈕不顯示第二行工具說明",
	);
});

test("入口只有一排帶入範本的 CTA——「從範例開始」那排重複的起手卡已移除", () => {
	// §V.11.0 原本把 chip 與起手卡分成兩排，但兩排的三個項目是同一組能力
	//（找供應商／整理採購需求／比較候選），等於同一件事給兩個入口。依產品端
	// 決定移除下面那排，範本併到 chip 上——留半截開頭反而讓使用者接不下去。
	for (const chip of SLICE_A_CAPABILITY_CHIPS) {
		assert.ok(chip.template.length > 12, `${chip.label} 應帶完整可編輯範本`);
	}
	// 剝註解再比對：這支元件的檔頭刻意寫出「從範例開始」來說明為什麼把它拿掉。
	const panelCode = stripComments(entryPanel);
	assert.doesNotMatch(panelCode, /從範例開始/);
	assert.doesNotMatch(panelCode, /hawkr-agent-entry__starter/);
});

test("§V.11.0：h1 逐字凍結，且入口只有一個 h1", () => {
	assert.equal(ENTRY_HEADING, "今天想請 Hawkr 幫你完成什麼？");
	const headings = entryPanel.match(/<h1\b/g) ?? [];
	assert.equal(headings.length, 1);
});

test("assistant route 的側欄標題降為 h2，entry／thread 主內容各自保留唯一 h1", () => {
	const sidebar = read("features/messages/components/ConversationSidebar.tsx");
	assert.match(sidebar, /onAssistantRoute \? <h2>\{title\}<\/h2> : <h1>/);
	assert.match(
		workspace,
		/function LoadingShell\(\)[\s\S]*?<h1>Hawkr Agent<\/h1>/,
	);
	assert.match(workspace, /const thread = \([\s\S]*?<h1>Hawkr Agent<\/h1>/);
});

test("入口只剩標誌、標題、chips 與 composer——三個區塊依產品端決定移除", () => {
	const panelCode = stripComments(entryPanel);
	// 起手卡：與 chips 重複。
	assert.doesNotMatch(panelCode, /從範例開始/);
	// 信任提示：⚠️ §V.11.0 列為固定顯示、§V.16.5-8 列入驗收，移除是產品端決定，
	// 不是漏做。這條斷言存在是為了讓「有人照設計書把它加回來」變成一次明確的
	// 決策，而不是靜悄悄地改動使用者看到的承諾。
	assert.doesNotMatch(panelCode, /送出需求不會直接下單/);
	// 最近的任務：完整清單本來就在左側 Messages 導覽 sidebar。
	assert.doesNotMatch(panelCode, /最近的任務/);
	assert.doesNotMatch(panelCode, /useAgentConversations/);
});

test("§V.11.0：chips 是 button，只把字帶進 composer，不觸發任何 side effect", () => {
	assert.match(entryPanel, /onClick=\{\(\) => onApply\(chip\)\}/);
	assert.match(entryPanel, /onApply\(chip\)/);
	assert.match(
		entryPanel,
		/apply\(\{ id: target\.id, text: target\.template \}\)/,
	);
	// 整支元件不得出現送出、建案件或派工的呼叫——點 chip 之後使用者仍要自己
	// 檢視並按送出。
	assert.doesNotMatch(entryPanel, /submitTurn|startSearch|submitAnswer/);
});

test("三個能力圖示是靜態 AppIcon glyph，裝飾性、不載入動畫", () => {
	const icon = read(
		"features/procurement-assistant/HawkrAgentCapabilityIcon.tsx",
	);
	assert.doesNotMatch(icon, /@\/assets\/|\.json/);
	assert.match(
		icon,
		/<AppIcon name=\{CAPABILITY_ICONS\[icon\]\} size=\{20\} \/>/,
	);
	assert.match(icon, /aria-hidden="true"/);
	for (const id of [
		"find-suppliers",
		"organise-requirements",
		"compare-candidates",
	]) {
		assert.match(icon, new RegExp(`"${id}":`));
	}
	assert.doesNotMatch(entryPanel, /onMouseEnter|replayCapabilityIcon/);
});

test("Entry hero 是靜態 Hawkr 標誌，不取代 authoritative Sprite 狀態", () => {
	const styles = read("styles/features/procurement-assistant.css");

	assert.match(
		entryPanel,
		/<span\s+className="hawkr-agent-entry__sprite hawkr-agent-sprite"\s+aria-hidden="true"\s*>\s*<BrandLogo variant="mark" size="fill" \/>/,
	);
	assert.doesNotMatch(entryPanel, /<HawkrAgentSprite|season/);
	assert.doesNotMatch(styles, /gaze|data-hoverable/);
	// Product states remain separately mapped and continue to express pre-ACK,
	// working, waiting, terminal, and error truth outside the decorative entry.
	assert.equal(SPRITE_MOTION.IDLE.loop, false);
	assert.ok(PRE_ACK_MOTION_STATES.includes("IDLE"));
});

test("§V.11.0：只有一個 composer，草稿與 clientRequestId 由 workspace 持有", () => {
	// entry 不自己畫輸入框，只擺 workspace 傳進來的那一個。
	assert.doesNotMatch(entryPanel, /<textarea/);
	assert.match(entryPanel, /composer: React\.ReactNode/);
	// workspace 只建立一次 ProcurementClarifyComposer 元素，entry 與 thread 共用它。
	const composerUses = workspace.match(/<ProcurementClarifyComposer\b/g) ?? [];
	assert.equal(composerUses.length, 1);
	// ACK 前不得換 key：同一份 trimmed payload 重送時沿用完整 identity；使用者
	// 改內容才建立新 operation。直接測 helper，不鎖 workspace 的 ref 寫法。
	let sequence = 0;
	const createId = () => `request-${++sequence}`;
	const first = prepareTurnSubmission(null, "  水果禮盒 50 箱  ", createId);
	const retry = prepareTurnSubmission(first, "水果禮盒 50 箱", createId);
	assert.strictEqual(retry, first);
	assert.equal(sequence, 1);
	const edited = prepareTurnSubmission(first, "水果禮盒 60 箱", createId);
	assert.equal(edited.clientRequestId, "request-2");
	assert.match(workspace, /prepareTurnSubmission\(previous, message/);
	// pending 或 error 時不得提前清掉 key；否則 retry 會變成另一個 operation。
	assert.match(
		workspace,
		/if \(session\.pendingTurn \|\| session\.error\) return;/,
	);
});

test("Entry composer 是單一主表面：沒有上方分隔線，送出圖示永遠有靜態 AppIcon", () => {
	const styles = read("styles/features/procurement-assistant.css");
	const messagesStyles = read("styles/features/messages.css");
	const sendButton = read("features/messages/components/SendButton.tsx");
	const entryComposeRule =
		styles.match(
			/\.hawkr-agent-entry__composer \.thread-page__compose \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.match(entryComposeRule, /border-top: 0/);
	assert.match(entryComposeRule, /box-shadow: none/);
	assert.match(sendButton, /<AppIcon name="send"/);
	assert.doesNotMatch(sendButton, /send-icon\.json/);
	assert.match(
		messagesStyles,
		/\.thread-page__send \{[\s\S]*?color: var\(--color-neutral-white\);/,
	);
	assert.match(
		messagesStyles,
		/\[data-theme="dark"\] \.thread-page__send \{[\s\S]*?color: var\(--color-neutral-white\);/,
	);
	assert.match(
		messagesStyles,
		/\[data-theme="dark"\] \.thread-page__send:not\(:disabled\):hover \{[\s\S]*?color: var\(--color-neutral-white\);/,
	);
	assert.match(
		messagesStyles,
		/\.thread-page__field \{[\s\S]*?border-radius: var\(--hkui-radius-2xl\)/,
	);
});

test("§V.11.0：既有 conversation、deep link 與還原都不重播入口頁", () => {
	const blankEntry = {
		conversationId: null,
		initialRequestId: null,
		initialConversationId: null,
		sessionConversationId: null,
		restoredSummary: null,
		turnCount: 0,
		stage: "clarifying" as const,
	};
	assert.equal(shouldShowAgentEntry(blankEntry), true);
	for (const occupied of [
		{ conversationId: "demo" },
		{ initialRequestId: "request-1" },
		{ initialConversationId: "conversation-deep-link" },
		{ sessionConversationId: "conversation-live" },
		{ restoredSummary: "已還原摘要" },
		{ turnCount: 1 },
		{ stage: "ready" as const },
	]) {
		assert.equal(shouldShowAgentEntry({ ...blankEntry, ...occupied }), false);
	}
	assert.match(workspace, /const showEntry = shouldShowAgentEntry\(\{/);
});

test("§V.10.12-4：唯一正式新對話入口，legacy 路徑在 HTTP 層退場", () => {
	const conversationRoute = read(
		"app/[locale]/(account)/account/messages/assistant/[conversationId]/page.tsx",
	);
	// /assistant/new 不得再掛 legacy workspace。比對 import 與 JSX 而不是
	// 光禿禿的名字——這支路由的註解**刻意**寫出 HawkrAgentChatWorkspace 來說明
	// 為什麼不再掛它，那是說明不是程式碼。
	assert.doesNotMatch(conversationRoute, /import HawkrAgentChatWorkspace/);
	assert.doesNotMatch(conversationRoute, /<HawkrAgentChatWorkspace/);

	// 退場必須在 next.config.ts 的 redirects()，不是 server component 的
	// redirect()：cacheComponents 下 PPR 會先把 shell 串出去，redirect() 只能
	// 降級成 client-side 導向，網址列停在舊路徑直到 RSC payload 抵達。實測
	// `/account/find-sellers` 當時回的是 200＋x-nextjs-prerender: 1。
	const config = read("next.config.ts");
	for (const legacy of [
		"/account/messages/assistant/new",
		"/account/find-sellers",
	]) {
		assert.match(
			config,
			new RegExp(
				`source: "${legacy}",\\s*destination: "/account/messages/assistant/procurement",\\s*permanent: false,`,
			),
			`${legacy} 必須在 next.config.ts 以 307 轉向 procurement`,
		);
	}
});
