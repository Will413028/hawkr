import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const read = (path: string) =>
	readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

const rootLayout = read("components/shared/RootDocument.tsx");
const accountMessagesLayout = read(
	"app/[locale]/(account)/account/messages/layout.tsx",
);
const sellerMessagesLayout = read(
	"app/[locale]/(seller)/seller-center/messages/layout.tsx",
);
const assistantMessagesLayout = read(
	"app/[locale]/(account)/account/messages/assistant/layout.tsx",
);
const threadClient = read("features/account/screens/messages/ThreadClient.tsx");
const drawerThread = read("features/messages/components/ChatDrawerThread.tsx");
const hawkrAgentMark = read(
	"features/procurement-assistant/HawkrAgentMark.tsx",
);
const searchPage = read("features/search/SearchPage.tsx");

const messagesCss = read("styles/features/messages.css");
const procurementCss = read("styles/features/procurement-assistant.css");
const drawerShellCss = read("styles/components/chat-drawer-shell.css");
const markCss = read("styles/components/hawkr-agent-mark.css");
const sidebarCss = read("styles/features/procurement-assistant-sidebar.css");
const searchAgentCss = read("styles/features/search-agent-entry.css");

function normalizeCss(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.replace(/\s+/g, "")
		.trim();
}

function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function cssBlock(source: string, prelude: string, occurrence = 0): string {
	const flexiblePrelude = escapeRegExp(prelude).replace(/\s+/g, "\\s+");
	const pattern = new RegExp(`(?:^|\\n)\\s*${flexiblePrelude}\\s*\\{`, "g");
	let match: RegExpExecArray | null = null;
	for (let index = 0; index <= occurrence; index += 1) {
		match = pattern.exec(source);
		assert.ok(match, `Missing CSS block: ${prelude} #${occurrence}`);
	}

	const open = source.indexOf("{", match.index);
	let depth = 1;
	let cursor = open + 1;
	while (cursor < source.length && depth > 0) {
		if (source[cursor] === "{") depth += 1;
		if (source[cursor] === "}") depth -= 1;
		cursor += 1;
	}
	assert.equal(depth, 0, `Unclosed CSS block: ${prelude}`);
	return source.slice(open + 1, cursor - 1);
}

function assertSameRule(
	actualSource: string,
	canonicalSource: string,
	prelude: string,
) {
	assert.equal(
		normalizeCss(cssBlock(actualSource, prelude)),
		normalizeCss(cssBlock(canonicalSource, prelude)),
		`${prelude} must stay identical to messages.css`,
	);
}

test("message and Agent styles are feature-owned instead of root-eager", () => {
	assert.doesNotMatch(rootLayout, /styles\/features\/messages\.css/);
	assert.doesNotMatch(
		rootLayout,
		/styles\/features\/procurement-assistant\.css/,
	);

	assert.match(accountMessagesLayout, /styles\/features\/messages\.css/);
	assert.match(
		accountMessagesLayout,
		/styles\/features\/procurement-assistant-sidebar\.css/,
	);
	assert.ok(
		accountMessagesLayout.indexOf("messages.css") <
			accountMessagesLayout.indexOf("procurement-assistant-sidebar.css"),
		"Messages base styles must precede Agent sidebar extensions",
	);

	assert.match(sellerMessagesLayout, /styles\/features\/messages\.css/);
	assert.match(threadClient, /styles\/features\/messages\.css/);
	assert.match(drawerThread, /styles\/components\/chat-drawer-shell\.css/);

	assert.match(assistantMessagesLayout, /styles\/features\/messages\.css/);
	assert.match(
		assistantMessagesLayout,
		/styles\/features\/procurement-assistant\.css/,
	);
	assert.ok(
		assistantMessagesLayout.indexOf("messages.css") <
			assistantMessagesLayout.indexOf("procurement-assistant.css"),
		"Hawkr Agent refinements must follow the shared thread primitives",
	);

	assert.match(hawkrAgentMark, /styles\/components\/hawkr-agent-mark\.css/);
	assert.match(searchPage, /styles\/features\/search-agent-entry\.css/);
});

test("the eager drawer boundary mirrors final thread geometry and room paint", () => {
	for (const selector of [
		".thread-page",
		".thread-page::before,\n.chat-empty-panel::before",
		'[data-theme="dark"] .thread-page::before,\n[data-theme="dark"] .chat-empty-panel::before',
		".thread-page__head",
		".thread-page__head-meta",
		".thread-page__head-meta :is(strong, h1)",
		".thread-page__head-status",
		".thread-page__head-status::before",
		'.thread-page__head-status[data-state="offline"]::before',
		".thread-page__body",
		".thread-page__compose",
		".seller-message-chat-sheet",
		".seller-message-chat-sheet > .overflow-auto",
		".seller-message-chat-sheet .thread-page",
		".seller-message-chat-sheet .chat-context",
		".seller-message-chat-sheet .thread-page__head",
		".thread-page--embedded .thread-page__back",
	]) {
		assertSameRule(drawerShellCss, messagesCss, selector);
	}

	// Keep the bounded conversation texture, but no second ambient loop
	// competing with the site's persistent, user-pausable background.
	assert.doesNotMatch(messagesCss, /chat-wallpaper-drift/);
	assert.doesNotMatch(drawerShellCss, /chat-wallpaper-drift/);

	const drawer960 = cssBlock(drawerShellCss, "@media (max-width: 960px)");
	const messages960 = cssBlock(messagesCss, "@media (max-width: 960px)");
	assertSameRule(drawer960, messages960, ".seller-message-chat-sheet");

	const drawer959 = cssBlock(drawerShellCss, "@media (max-width: 959px)");
	const messages959 = cssBlock(messagesCss, "@media (max-width: 959px)");
	for (const selector of [
		".thread-page__head",
		".thread-page__body",
		".thread-page__compose",
	]) {
		assertSameRule(drawer959, messages959, selector);
	}
});

test("small cross-route Agent styles stay identical to their compatibility source", () => {
	const mainSidebarStart = procurementCss.indexOf(".chat-sidebar__agent {");
	const mainMarkComment = procurementCss.indexOf(
		"/* Hawkr Agent 識別記號",
		mainSidebarStart,
	);
	const sidebarMediaPrelude =
		"@media (min-width: 960px) and (max-width: 1199px)";
	const firstCopyMedia = sidebarCss.indexOf(sidebarMediaPrelude);
	const secondCopyMedia = sidebarCss.indexOf(
		sidebarMediaPrelude,
		firstCopyMedia + sidebarMediaPrelude.length,
	);
	assert.ok(mainSidebarStart >= 0 && mainMarkComment > mainSidebarStart);
	assert.ok(firstCopyMedia >= 0 && secondCopyMedia > firstCopyMedia);
	assert.equal(
		normalizeCss(
			sidebarCss.slice(
				sidebarCss.indexOf(".chat-sidebar__agent {"),
				secondCopyMedia,
			),
		),
		normalizeCss(procurementCss.slice(mainSidebarStart, mainMarkComment)),
	);
	assert.equal(
		normalizeCss(cssBlock(sidebarCss, sidebarMediaPrelude, 1)),
		normalizeCss(cssBlock(procurementCss, sidebarMediaPrelude, 1)),
	);

	const mainMarkStart = procurementCss.indexOf(".assistant-mark {");
	const mainMarkEnd = procurementCss.indexOf(
		sidebarMediaPrelude,
		mainMarkStart,
	);
	assert.equal(
		normalizeCss(markCss.slice(markCss.indexOf(".assistant-mark {"))),
		normalizeCss(procurementCss.slice(mainMarkStart, mainMarkEnd)),
	);

	const searchSelector =
		".search-page:has([data-search-empty] [data-hawkr-agent-entry])";
	const mainSearchStart = procurementCss.indexOf(searchSelector);
	const mainSearchEnd = procurementCss.indexOf(
		"/* Workspace shell",
		mainSearchStart,
	);
	assert.ok(mainSearchStart >= 0 && mainSearchEnd > mainSearchStart);
	assert.equal(
		normalizeCss(searchAgentCss.slice(searchAgentCss.indexOf(searchSelector))),
		normalizeCss(procurementCss.slice(mainSearchStart, mainSearchEnd)),
	);
});
