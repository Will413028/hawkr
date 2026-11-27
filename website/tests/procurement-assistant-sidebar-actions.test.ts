import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const menu = readFileSync(
	path.join(root, "features/procurement-assistant/ConversationActionsMenu.tsx"),
	"utf8",
);
const sidebar = readFileSync(
	path.join(root, "features/messages/components/ConversationSidebar.tsx"),
	"utf8",
);
const css = readFileSync(
	path.join(root, "styles/features/procurement-assistant.css"),
	"utf8",
);
const messagesCss = readFileSync(
	path.join(root, "styles/features/messages.css"),
	"utf8",
);
const hawkrAgentMark = readFileSync(
	path.join(root, "features/procurement-assistant/HawkrAgentMark.tsx"),
	"utf8",
);
const layout = readFileSync(
	path.join(root, "app/[locale]/(account)/account/messages/layout.tsx"),
	"utf8",
);

test("訊息 workspace 延續網站紙張底色，不再另外鋪一層純白面板", () => {
	const shellRule = messagesCss.match(/^\.chat-shell \{([^}]*)\}/m)?.[1] ?? "";
	assert.match(shellRule, /background: var\(--surface-page\)/);
	assert.doesNotMatch(shellRule, /background: var\(--hkui-content1\)/);
});

test("「⋯」是就地彈出選單，不是從右側滑出的 Drawer", () => {
	assert.match(menu, /role="menu"/);
	assert.match(menu, /role="menuitem"/);
	assert.match(menu, /aria-haspopup="menu"/);
	assert.match(menu, /aria-expanded=\{open\}/);
	assert.doesNotMatch(menu, /^import .*Drawer/m);
	assert.equal(
		existsSync(
			path.join(
				root,
				"features/procurement-assistant/ConversationActionsDrawer.tsx",
			),
		),
		false,
		"ConversationActionsDrawer 應已刪除",
	);
	assert.doesNotMatch(layout, /ConversationActionsDrawer/);
});

test("選單關閉語意：點外面、Esc 都關；Esc 關閉後焦點回觸發鈕", () => {
	assert.match(menu, /pointerdown/);
	assert.match(menu, /event\.key === "Escape"/);
	assert.match(menu, /triggerRef\.current\?\.focus\(\)/);
	assert.match(menu, /firstItemRef\.current\?\.focus\(\)/);
});

test("選單用 portal＋fixed 逃出 overflow：absolute 會被側欄裁掉（實機上第二個項目被切掉）", () => {
	assert.match(css, /\.chat-sidebar__agent-menu-list \{\n\tposition: fixed;/);
	assert.doesNotMatch(
		css,
		/\.chat-sidebar__agent-menu-list \{\n\tposition: absolute;/,
	);
	assert.match(menu, /createPortal\(/);
	assert.match(menu, /document\.body/);
	assert.match(menu, /menuRef\.current\?\.contains\(target\)/);
	assert.match(
		menu,
		/left: Math\.max\(VIEWPORT_EDGE, rect\.right - MENU_WIDTH\)/,
	);
	assert.doesNotMatch(menu, /right:/);
	assert.match(menu, /menu\.offsetHeight/);
	assert.match(menu, /window\.addEventListener\("scroll"/);
	assert.match(menu, /if \(!reposition\(\)\) close\(false\);/);
});

test("刪除有兩條路：hover 的垃圾桶與選單項，都走同一個樂觀刪除＋復原 toast", () => {
	assert.match(sidebar, /chat-sidebar__agent-action--delete/);
	assert.match(sidebar, /aria-label=\{`刪除對話「\$\{label\}」`\}/);
	assert.match(menu, /刪除對話/);
	assert.match(sidebar, /onClick=\{\(\) => onAgentRowDelete\(row\)\}/);
	assert.match(sidebar, /onDelete=\{\(\) => onAgentRowDelete\(row\)\}/);
});

test("刪掉正在看的對話會離開該深連結（不留在已刪對話的 404 頁）", () => {
	assert.match(layout, /row\.id === activeAgentConversationId/);
	assert.match(layout, /router\.replace\(ASSISTANT_PROCUREMENT_PATH\)/);
});

test("改名是就地編輯：編輯中該列不是連結（輸入框巢在 <a> 裡點擊會被導航吃掉）", () => {
	assert.match(sidebar, /renamingId === row\.id && onAgentRowRename \? \(/);
	assert.match(
		sidebar,
		/<div className="chat-sidebar__item" data-kind="agent">/,
	);
	assert.match(sidebar, /<RenameField/);
	assert.match(sidebar, /renamingId !== row\.id &&/);
});

test("改名的標題規則與後端契約一致：trim 後 1–40 runes、禁換行", () => {
	assert.match(sidebar, /const MAX_TITLE_RUNES = 40;/);
	assert.match(sidebar, /replaceAll\(\/\[\\n\\r\]\/g, " "\)\.trim\(\)/);
	assert.match(sidebar, /runes < 1 \|\| tooLong \|\| trimmed === initial/);
	assert.match(sidebar, /const tooLong = runes > MAX_TITLE_RUNES/);
	assert.match(sidebar, /最多 \$\{MAX_TITLE_RUNES\} 字，目前 \$\{runes\} 字/);
	assert.match(sidebar, /aria-invalid=\{tooLong \|\| undefined\}/);
});

test("改名鍵盤語意：Enter 儲存、Esc 取消、失焦儲存；輸入法組字中不誤送", () => {
	assert.match(sidebar, /event\.key === "Escape"/);
	assert.match(sidebar, /onBlur=\{commit\}/);
	assert.match(sidebar, /event\.nativeEvent\.isComposing/);
});

test("沒有標題時用最後一則訊息當列標，且不重複印一次預覽", () => {
	assert.match(
		sidebar,
		/const label = row\.title \|\| row\.preview \|\| "未命名對話";/,
	);
	assert.match(
		sidebar,
		/const showPreview = Boolean\(row\.title\) && Boolean\(row\.preview\);/,
	);
});

test("icon rail 仍以真實 viewport 判定；歷史列改為永遠可見的靜態分類彈珠", () => {
	assert.match(
		sidebar,
		/const ICON_RAIL_MEDIA_QUERY = "\(min-width: 960px\) and \(max-width: 1199px\)";/,
	);
	assert.match(
		sidebar,
		/const \[matches, setMatches\] = React\.useState\(false\);/,
	);
	assert.match(sidebar, /window\.matchMedia\(ICON_RAIL_MEDIA_QUERY\)/);
	assert.match(sidebar, /media\.addEventListener\("change", sync\);/);
	assert.match(
		sidebar,
		/return \(\) => media\.removeEventListener\("change", sync\);/,
	);
	assert.match(
		sidebar,
		/const compactSectionTabs = hasAgentSection && collapsed && iconRailViewport;/,
	);

	assert.doesNotMatch(
		sidebar,
		/HawkrAgentConversationMarble|primaryCategoryId/,
	);
	assert.doesNotMatch(sidebar, /resolveHawkrAgentSeason|season=/);
	assert.doesNotMatch(sidebar, /showAgentRowMarks/);
	assert.doesNotMatch(sidebar, /chat-sidebar__agent-mark/);
});

test("動作鈕浮出時時間戳讓位——不改版面，只換誰可見", () => {
	assert.match(
		css,
		/\.chat-sidebar__agent-row:hover \.chat-sidebar__item-head small/,
	);
	assert.match(css, /opacity: 0;/);
	assert.match(css, /@media \(hover: none\)/);
});

test("agent 對話用分頁承載，不再是釘在人類列表上方的無上限區塊", () => {
	assert.doesNotMatch(css, /\.chat-sidebar__agent \{[^}]*flex-shrink: 0/);
	const agentRule = css.match(/\.chat-sidebar__agent \{([^}]*)\}/)?.[1] ?? "";
	assert.match(agentRule, /flex: 1 1 0/);
	assert.match(agentRule, /min-height: 0/);

	assert.match(sidebar, /type SectionKey = "human" \| "agent"/);
	assert.match(sidebar, /const hasAgentSection = Boolean\(assistantEntry\)/);
	assert.doesNotMatch(sidebar, /hasAgentSection = agentRows\.length > 0/);
	assert.match(layout, /assistantEntry=\{\s*HAWKR_AGENT_ENABLED/);
	assert.doesNotMatch(
		readFileSync(
			path.join(
				root,
				"app/[locale]/(seller)/seller-center/messages/layout.tsx",
			),
			"utf8",
		),
		/assistantEntry/,
	);
	assert.match(
		sidebar,
		/const showAgentList = hasAgentSection && section === "agent"/,
	);
	assert.match(sidebar, /hasAgentSection \? \(/);
	const agentTabStart = sidebar.indexOf('id="chat-tab-agent"');
	const agentTab = sidebar.slice(
		agentTabStart,
		sidebar.indexOf("</button>", agentTabStart),
	);
	assert.match(agentTab, /Hawkr Agent/);
	assert.match(agentTab, /<HawkrAgentMark/);
	assert.doesNotMatch(agentTab, /chat-sidebar__filter-count/);
	assert.match(
		sidebar,
		/\{showAgentList \? null : loading && items\.length === 0/,
	);
	assert.doesNotMatch(sidebar, /chat-sidebar__section-label/);
});

test("重複的 Agent hero 已移除；雙分頁直接承載賣家與 Agent identity", () => {
	assert.doesNotMatch(sidebar, /chat-sidebar__assistant/);
	assert.doesNotMatch(css, /chat-sidebar__assistant/);
	assert.doesNotMatch(sidebar, /assistantEntry\?\.(?:href|label|description)/);
	const humanTabStart = sidebar.indexOf('id="chat-tab-human"');
	const humanTab = sidebar.slice(
		humanTabStart,
		sidebar.indexOf("</button>", humanTabStart),
	);
	assert.match(humanTab, /name="work"/);
	assert.match(humanTab, /chat-sidebar__filter-icon/);
	assert.match(humanTab, /aria-hidden="true"/);

	const agentTabStart = sidebar.indexOf('id="chat-tab-agent"');
	const agentTab = sidebar.slice(
		agentTabStart,
		sidebar.indexOf("</button>", agentTabStart),
	);
	assert.match(agentTab, /chat-sidebar__filter-agent-mark/);
	assert.match(agentTab, /hideBadge/);
});

test("Agent 分頁的記號是靜態 Hawkr 標誌，不掛重播 handle", () => {
	assert.match(hawkrAgentMark, /<BrandLogo variant="mark" size="fill" \/>/);
	assert.doesNotMatch(hawkrAgentMark, /forwardRef/);
	assert.doesNotMatch(sidebar, /playAgentTabMark|agentTabMarkRef/);
});

test("收合側欄保留兩顆直向 identity tabs，不因 hero 移除而失去 Agent 入口", () => {
	assert.match(
		sidebar,
		/data-kind=\{hasAgentSection \? "sections" : "filters"\}/,
	);
	assert.match(
		sidebar,
		/aria-orientation=\{compactSectionTabs \? "vertical" : "horizontal"\}/,
	);
	assert.match(sidebar, /"ArrowUp"/);
	assert.match(sidebar, /"ArrowDown"/);
	assert.match(
		sidebar,
		/const orientationKeys = compactSectionTabs[\s\S]*?\["ArrowUp", "ArrowDown"\][\s\S]*?\["ArrowLeft", "ArrowRight"\]/,
	);
	assert.match(
		messagesCss,
		/chat-sidebar__filters\[data-kind="sections"\][\s\S]*?grid-template-columns: 44px;[\s\S]*?grid-auto-rows: 44px;/,
	);
	assert.match(
		messagesCss,
		/chat-sidebar__filters\[data-kind="sections"\][\s\S]*?button \{[\s\S]*?width: 44px;[\s\S]*?height: 44px;/,
	);
	assert.match(
		messagesCss,
		/chat-sidebar__filters\[data-kind="sections"\][\s\S]*?button::before \{[\s\S]*?inset: 0;/,
	);
});

test("沒有標題也沒有訊息的空對話不進側欄，但正在看的那一個保留", () => {
	const filterBlock =
		sidebar.match(/const agentRows = React\.useMemo\(([\s\S]*?)\);\n/)?.[1] ??
		"";
	assert.match(
		filterBlock,
		/row\.active \|\| Boolean\(row\.title\) \|\| Boolean\(row\.preview\)/,
	);
	assert.equal(
		existsSync(
			path.join(root, "docs/backend-issues/B-247-empty-agent-conversations.md"),
		),
		true,
	);
});

test("agent 分頁沒東西時有自己的空狀態，且區分「還沒有」與「搜不到」", () => {
	const start = sidebar.indexOf(
		"showAgentList && visibleAgentRows.length === 0",
	);
	const block = start === -1 ? "" : sidebar.slice(start, start + 1400);
	assert.match(block, /query \? "找不到對話" : "還沒有 Agent 對話"/);
	assert.match(block, /Hawkr Agent 會整理成可報價的需求/);
});

test("空狀態沒有虛線框、有圖示、且自己帶主要動作", () => {
	const messagesCss = readFileSync(
		path.join(root, "styles/features/messages.css"),
		"utf8",
	);
	const emptyRule =
		messagesCss.match(/\.chat-sidebar__empty \{([^}]*)\}/g)?.join("") ?? "";
	assert.doesNotMatch(emptyRule, /border:\s*1px dashed/);
	assert.match(emptyRule, /align-items: center/);
	assert.match(emptyRule, /text-align: center/);

	assert.match(sidebar, /className="chat-sidebar__empty-icon"/);
	assert.match(sidebar, /className="chat-sidebar__empty-action"/);
	assert.match(
		messagesCss,
		/\.chat-sidebar__empty-action\.chat-sidebar__empty-action--quiet \{/,
	);
	assert.match(sidebar, /onClick=\{onNewAgentChat\}/);
	assert.doesNotMatch(sidebar, /開始新對話/);
});

test("分頁選中態是低對比的被選篩選，不是搶過主要動作的實心按鈕；0 不印", () => {
	const messagesCss = readFileSync(
		path.join(root, "styles/features/messages.css"),
		"utf8",
	);
	const activeRule =
		messagesCss.match(
			/\.chat-sidebar__filters button\[data-active="true"\] \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.match(activeRule, /background: var\(--theme-primary-subtle\)/);
	assert.match(activeRule, /border-color: var\(--theme-primary-border\)/);
	assert.match(activeRule, /color: var\(--theme-text-accent-on-tint\)/);
	assert.doesNotMatch(activeRule, /background: var\(--hkui-primary\);/);

	assert.equal(
		(
			sidebar.match(
				/unreadCount > 0 \? \(\s*<span className="chat-sidebar__filter-count">/g,
			) ?? []
		).length,
		2,
	);
	assert.doesNotMatch(sidebar, /chat-sidebar__filter-dot/);
});

test("agent 分頁把載入／降級／空／搜不到當成四種狀態，不共用一句 placeholder", () => {
	const start = sidebar.indexOf(
		"showAgentList && visibleAgentRows.length === 0",
	);
	const block = start === -1 ? "" : sidebar.slice(start, start + 2600);
	assert.match(block, /agentLoading \? \(/);
	assert.match(block, /<ConversationListLoading \/>/);
	assert.match(sidebar, /function ConversationListLoading\(\)/);
	assert.match(sidebar, /<span className="sr-only">載入對話中…<\/span>/);
	assert.match(block, /!agentAvailable \? \(/);
	assert.match(block, /Hawkr Agent 對話暫時無法載入/);
	assert.match(block, /你的對話沒有消失/);
	assert.match(block, /還沒有 Agent 對話/);

	assert.match(
		layout,
		/agentLoading=\{HAWKR_AGENT_ENABLED && agent\.loading\}/,
	);
	assert.match(
		layout,
		/agentAvailable=\{HAWKR_AGENT_ENABLED && agent\.available\}/,
	);
	assert.match(
		layout,
		/agentHasMore=\{HAWKR_AGENT_ENABLED && agent\.hasMore\}/,
	);
});

test("錯誤與重新整理不是死碼：兩個 layout 都要傳 onRefresh", () => {
	const sellerLayout = readFileSync(
		path.join(root, "app/[locale]/(seller)/seller-center/messages/layout.tsx"),
		"utf8",
	);
	assert.match(layout, /onRefresh=\{\(\) => \{/);
	assert.match(sellerLayout, /onRefresh=\{\(\) => void refresh\(\)\}/);
	assert.doesNotMatch(sidebar, /↻/);
	assert.match(sidebar, /<AppIcon name="refresh" size=\{18\} \/>/);
	assert.doesNotMatch(sidebar, /name="reorder"/);
});

test("觸控上不常駐刪除鈕，且兩顆鈕的命中區不重疊", () => {
	const touchBlock =
		css.match(/@media \(hover: none\) \{([\s\S]*?)\n\}/)?.[1] ?? "";
	assert.match(
		touchBlock,
		/\.chat-sidebar__agent-action--delete \{\s*display: none;/,
	);
	assert.match(touchBlock, /padding-right: 44px/);

	const actionsRule =
		css.match(/\.chat-sidebar__agent-actions \{([^}]*)\}/)?.[1] ?? "";
	const gap = Number(actionsRule.match(/gap:\s*(\d+)px/)?.[1] ?? "0");
	assert.ok(gap >= 10, `agent 動作鈕 gap 需 ≥ 10px 才不重疊，實際 ${gap}px`);
});

test("tablist 有真的鍵盤行為與 aria 連結，不是只有外殼", () => {
	assert.match(sidebar, /role="tablist"/);
	assert.match(sidebar, /onKeyDown=\{\(event\) => \{/);
	for (const key of ["ArrowLeft", "ArrowRight", "Home", "End"]) {
		assert.match(sidebar, new RegExp(`"${key}"`));
	}
	assert.match(sidebar, /tabIndex=\{section === "human" \? 0 : -1\}/);
	assert.match(sidebar, /aria-controls="chat-panel-human"/);
	assert.match(sidebar, /aria-controls="chat-panel-agent"/);
	assert.match(
		sidebar,
		/id=\{showAgentList \? "chat-panel-agent" : "chat-panel-human"\}/,
	);
	assert.match(sidebar, /role="tabpanel"/);
	assert.match(
		sidebar,
		/aria-busy=\{showAgentList \? agentLoading : loading\}/,
	);
	const panelRule =
		readFileSync(path.join(root, "styles/features/messages.css"), "utf8").match(
			/\.chat-sidebar__panel \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.match(panelRule, /display: flex/);
	assert.match(panelRule, /flex: 1 1 0/);
	assert.match(panelRule, /min-height: 0/);
	assert.match(panelRule, /overflow: hidden/);
});

test("冷載入保留列表幾何；active row 以外框取代左側指示條", () => {
	const messagesCss = readFileSync(
		path.join(root, "styles/features/messages.css"),
		"utf8",
	);
	assert.match(sidebar, /function ConversationListLoading\(\)/);
	assert.match(
		sidebar,
		/chat-sidebar__loading" role="status" aria-live="polite"/,
	);
	assert.match(sidebar, /\[0, 1, 2\]\.map/);
	assert.match(messagesCss, /\.chat-sidebar__loading-row \{/);
	assert.doesNotMatch(messagesCss, /\.chat-sidebar__item::before \{/);
	assert.doesNotMatch(
		messagesCss,
		/\.chat-sidebar__item\[aria-current="page"\]::before \{/,
	);
	assert.match(sidebar, /aria-current=\{active \? "page" : undefined\}/);
	assert.match(
		messagesCss,
		/\.chat-sidebar__item\[aria-current="page"\] \{[\s\S]*?outline: 1px solid var\(--theme-primary-border\);[\s\S]*?outline-offset: -1px;/,
	);
	assert.match(
		messagesCss,
		/\.chat-sidebar__item:focus-visible \{[\s\S]*?outline: 2px solid var\(--theme-action-primary\);[\s\S]*?outline-offset: 2px;/,
	);
	assert.match(
		messagesCss,
		/\.chat-shell\[data-sidebar-collapsed="true"\] \.chat-sidebar__item:focus-visible \{[\s\S]*?outline-offset: -2px;/,
	);
});

test("列表 chrome 使用搜尋／撰寫圖示，主要控制維持 44px 命中區", () => {
	const messagesCss = readFileSync(
		path.join(root, "styles/features/messages.css"),
		"utf8",
	);
	assert.match(sidebar, /name="search"[\s\S]{0,160}?chat-sidebar__search-icon/);
	assert.match(sidebar, /name="edit"[\s\S]{0,120}?開新對話/);
	assert.doesNotMatch(sidebar, /name="plus"/);
	const controls =
		messagesCss.match(
			/\.chat-sidebar__refresh,\s*\.chat-sidebar__collapse \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.match(controls, /width: 44px/);
	assert.match(controls, /height: 44px/);
});

test("清單可能被 50 筆上限截斷時，搜尋文案不得斬釘截鐵說找不到", () => {
	assert.match(sidebar, /const humanMaybeTruncated = items\.length >= 50/);
	assert.match(sidebar, /activeListMaybeTruncated/);
	assert.match(sidebar, /目前只載入最近的對話，更早的還沒抓下來。/);
	assert.match(sidebar, /crossTabHint/);
	assert.match(sidebar, /筆符合/);
});

test("header 不再有重複的「＋」，加號方框圖示也不出現在主要動作上", () => {
	assert.doesNotMatch(sidebar, /aria-label="開新的 Hawkr Agent 對話"/);

	assert.match(
		sidebar,
		/\{showAgentList && onNewAgentChat \? \(\s*<button[\s\S]{0,240}?chat-sidebar__new-chat/,
		"「開新對話」必須常駐於 Agent 分頁，而不是只在空狀態出現",
	);
	assert.match(sidebar, /aria-label="開新對話"/);
	assert.match(sidebar, /title=\{collapsed \? "開新對話" : undefined\}/);
	assert.match(sidebar, /chat-sidebar__new-chat-label/);
	assert.match(
		css,
		/data-sidebar-collapsed="true"\] \.chat-sidebar__new-chat \{[\s\S]*?width: 44px;[\s\S]*?height: 44px;[\s\S]*?min-height: 44px;[\s\S]*?justify-content: center;/,
	);
	assert.match(
		css,
		/data-sidebar-collapsed="true"\] \.chat-sidebar__new-chat-label \{[\s\S]*?display: none;/,
	);
	assert.match(
		messagesCss,
		/data-sidebar-collapsed="true"\] \.chat-sidebar__loading-row \{[\s\S]*?width: 44px;[\s\S]*?grid-template-columns: 36px;/,
	);
	assert.match(
		messagesCss,
		/data-sidebar-collapsed="true"\] \.chat-sidebar__loading-copy \{[\s\S]*?display: none;/,
	);
	assert.doesNotMatch(sidebar, /開始新對話/);
	assert.doesNotMatch(sidebar, /name="plus"/);
});
