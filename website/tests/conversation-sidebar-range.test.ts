import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 側欄清單底部的計數（設計稿 01／03／04 都畫著「顯示 1 - 6 / 6」）。
 *
 * 數字本身的規則釘在 `lib/chat/__tests__/list-range.unit.test.ts`；這一支守的
 * 是接線：數字取自**當前分頁**、這一列不跟著清單捲走、收合成 icon rail 時要
 * 讓位。
 */
const root = fileURLToPath(new URL("..", import.meta.url));

function read(rel: string): string {
	return readFileSync(path.join(root, rel), "utf8");
}

const sidebar = read("features/messages/components/ConversationSidebar.tsx");
const css = read("styles/features/messages.css");

test("計數取當前分頁的數字，不把兩份清單加總", () => {
	// 使用者一次只看得到其中一份；加總會給出一個在畫面上數不出來的數字。
	assert.match(
		sidebar,
		/visible: showAgentList \? visibleAgentRows\.length : visibleItems\.length/,
	);
	assert.match(
		sidebar,
		/loaded: showAgentList \? agentRows\.length : items\.length/,
	);
});

test("分母用的是雜訊過濾後的 agentRows，不是原始的 agentConversations", () => {
	// 沒有標題也沒有訊息的殘骸列不會進清單，把它們算進分母就會出現
	// 「顯示 1 - 3 / 5」卻怎麼數都只有 3 列的情況。
	assert.doesNotMatch(sidebar, /loaded:[^\n]*agentConversations/);
});

test("清單可能沒抓完時沿用既有的截斷判定，不另外造一條", () => {
	assert.match(sidebar, /maybeTruncated: activeListMaybeTruncated/);
});

test("這一列住在可捲動面板外面，不跟著清單捲走", () => {
	// `.chat-sidebar__panel` 是 flex:1 且自己捲動；放進去的話，這句「共有幾筆」
	// 只有捲到底時才看得到，等於沒有。
	const panelClose = sidebar.indexOf('className="chat-sidebar__panel"');
	const rangeAt = sidebar.indexOf('className="chat-sidebar__range"');
	const asideClose = sidebar.indexOf("</aside>");
	assert.ok(panelClose > -1 && rangeAt > -1 && asideClose > -1);
	assert.ok(
		rangeAt > panelClose && rangeAt < asideClose,
		"計數列要落在 panel 之後、</aside> 之前",
	);
});

test("一列都看不到時不渲染這一行", () => {
	// helper 回 null，版位跟著整個消失——空狀態已經在講話，再疊一句
	// 「顯示 1 - 0 / 6」既不是句子也和它打架。
	assert.match(sidebar, /\{rangeLabel \?[\s\S]*?: null\}/);
});

test("收合成 icon rail 時整列隱藏", () => {
	// 72px 的 rail 塞不下一段中文計數。沿用既有那一組 collapsed 隱藏規則，
	// 不新開一條 media query。
	assert.match(
		css,
		/\.chat-shell\[data-sidebar-collapsed="true"\] \.chat-sidebar__range \{\s*\n?\s*display: none;/,
	);
});

test("計數列的樣式走 design token，不硬編色值", () => {
	const rule = css.match(/\.chat-sidebar__range \{[\s\S]*?\n\}/)?.[0];
	assert.ok(rule);
	assert.match(rule, /var\(--hkui-foreground-muted\)/);
	assert.match(rule, /var\(--hkui-divider\)/);
	// 數字會逐筆變動，等寬數字讓它不左右抖動。
	assert.match(rule, /font-variant-numeric: tabular-nums;/);
});
