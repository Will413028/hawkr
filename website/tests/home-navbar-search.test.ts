import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const navbarPath = new URL("../components/shared/Navbar.tsx", import.meta.url);
const homePagePath = new URL("../features/home/HomePage.tsx", import.meta.url);
const memberSearchPath = new URL(
	"../features/home/components/HomeMemberMobileSearch.tsx",
	import.meta.url,
);
const searchPath = new URL(
	"../features/home/components/HomeCompactSearch.tsx",
	import.meta.url,
);
const searchExperiencePath = new URL(
	"../features/home/components/HomeCompactSearchExperience.tsx",
	import.meta.url,
);
const assistantEntryPath = new URL(
	"../features/procurement-assistant/AssistantEntryLink.tsx",
	import.meta.url,
);
const assistantEntryLayoutPath = new URL(
	"../features/procurement-assistant/AssistantEntryLayout.tsx",
	import.meta.url,
);

test("home keeps member search in the desktop navbar and moves only its mobile copy below", async () => {
	const [navbarSource, homeSource, memberSearchSource] = await Promise.all([
		readFile(navbarPath, "utf8"),
		readFile(homePagePath, "utf8"),
		readFile(memberSearchPath, "utf8"),
	]);

	assert.match(navbarSource, /const isHome = pathname === "\/"/);
	assert.match(
		navbarSource,
		/const showHomeSearch =\s*isHome && \(railMode === "guest" \|\| railMode === "buyer"\)/,
	);
	assert.match(
		navbarSource,
		/const moveHomeSearchBelowNavbar = isHome && railMode === "buyer"/,
	);
	assert.match(navbarSource, /showHomeSearch \? \(\s*<HomeCompactSearch/);
	assert.doesNotMatch(navbarSource, /min-width:841px/);
	assert.match(
		navbarSource,
		/moveHomeSearchBelowNavbar &&\s*"\[@media\(max-width:840px\)\]:hidden"/,
	);
	assert.match(
		navbarSource,
		/showHomeSearch\s*\? navItems\.filter\(\(item\) => item\.href !== "\/search"\)/,
	);
	assert.match(
		homeSource,
		/<HomeMemberMobileSearch\b[^>]*\/>\s*<HomeContent\b(?=[^>]*\binitialSnapshot=\{snapshot\})[^>]*\/>/,
	);
	assert.doesNotMatch(homeSource, /<Suspense/);
	assert.match(
		memberSearchSource,
		/const \{ hydrated, isLoggedIn \} = useAuth\(\)/,
	);
	assert.match(
		memberSearchSource,
		/if \(!hydrated \|\| !isLoggedIn\) return null/,
	);
	assert.match(memberSearchSource, /data-home-member-search/);
	assert.match(
		memberSearchSource,
		/className="home-fixed-section relative z-\[3\]/,
	);
	assert.match(
		memberSearchSource,
		/hidden \[@media\(max-width:840px\)\]:block/,
	);
	assert.match(
		memberSearchSource,
		/<HomeCompactSearch className="w-full" recoveryScope="member-mobile" \/>/,
	);
});

test("home navbar search follows the CTA height and spacing contract", async () => {
	const [navbarSource, searchSource] = await Promise.all([
		readFile(navbarPath, "utf8"),
		readFile(searchPath, "utf8"),
	]);

	assert.match(searchSource, /data-home-navbar-search/);
	assert.match(searchSource, /data-home-search-backdrop/);
	assert.match(searchSource, /\[@media\(max-width:840px\)\]:block/);
	assert.match(searchSource, /touch-none/);
	assert.match(
		searchSource,
		/target\.closest\("\[data-home-search-backdrop\]"\)/,
	);
	assert.match(
		searchSource,
		/onPointerDown=\{\(event\) => event\.stopPropagation\(\)\}/,
	);
	assert.match(
		searchSource,
		/onMouseDown=\{\(event\) => event\.stopPropagation\(\)\}/,
	);
	assert.match(
		searchSource,
		/onClick=\{\(event\) => \{[\s\S]*?dismiss\(\{ blurInput: true \}\)/,
	);
	assert.match(
		searchSource,
		/fixed z-\[60\].*\[@media\(max-width:840px\)\]:z-\[210\]/,
	);
	assert.match(searchSource, /\[@media\(max-width:840px\)\]:overflow-y-auto/);
	assert.match(searchSource, /className="flex h-10 w-full min-w-0 max-w-full/);
	assert.match(searchSource, /pr-1 pl-3/);
	assert.match(searchSource, /type="submit"/);
	assert.match(searchSource, /aria-label="送出搜尋"/);
	assert.match(searchSource, /<SearchGlyph stroke=\{2\.2\} size=\{17\}/);
	assert.match(searchSource, /data-slot="control"/);
	assert.match(
		searchSource,
		/h-full w-full min-w-0 border-0 bg-transparent p-0/,
	);
	assert.match(searchSource, /\[@media\(max-width:840px\)\]:text-base/);
	assert.match(searchSource, /inputMode="search"/);
	assert.match(searchSource, /enterKeyHint="search"/);
	assert.match(searchSource, /window\.visualViewport/);
	assert.match(
		searchSource,
		/viewport\?\.addEventListener\("resize", updateCoords\)/,
	);
	assert.match(searchSource, /\[@media\(max-width:840px\)\]:hidden/);
	assert.match(searchSource, />\s*搜尋商品\s*<\/span>/);
	assert.match(navbarSource, /showHomeSearch && "mr-0"/);
	assert.match(navbarSource, /mx-\[10px\] w-auto min-w-\[180px\] flex-1/);
	assert.match(navbarSource, /\[@media\(max-width:1280px\)\]:mx-\[2px\]/);
	assert.match(navbarSource, /\[@media\(max-width:840px\)\]:mx-\[8px\]/);
	assert.match(navbarSource, /\[@media\(max-width:840px\)\]:min-w-0/);
	assert.match(navbarSource, /showHomeSearch && "h-10!"/);
});

test("home search exposes the Hawkr Agent entry without replacing product search", async () => {
	const [
		searchSource,
		searchExperienceSource,
		assistantEntrySource,
		assistantEntryLayoutSource,
	] = await Promise.all([
		readFile(searchPath, "utf8"),
		readFile(searchExperiencePath, "utf8"),
		readFile(assistantEntryPath, "utf8"),
		readFile(assistantEntryLayoutPath, "utf8"),
	]);

	assert.match(searchExperienceSource, /<AssistantEntryLink/);
	assert.match(searchExperienceSource, /ref=\{agentEntryRef\}/);
	assert.match(searchSource, /aria-haspopup="dialog"/);
	assert.match(searchSource, /event\.key !== "ArrowDown"/);
	assert.match(searchSource, /const firstPanelControl =/);
	assert.match(searchSource, /panelRef\.current\?\.querySelector<HTMLElement>/);
	assert.match(searchSource, /firstPanelControl\.focus\(\)/);
	assert.match(searchSource, /type="submit"/);
	assert.match(searchSource, /router\.push\(`\/search\?q=/);
	assert.match(assistantEntrySource, /data-hawkr-agent-entry/);
	assert.match(
		assistantEntrySource,
		/className=\{cn\([\s\S]*?ASSISTANT_ENTRY_LAYOUT,/,
	);
	assert.match(assistantEntryLayoutSource, /min-h-11/);
	assert.match(assistantEntrySource, /redirect: ASSISTANT_PROCUREMENT_PATH/);
	assert.match(assistantEntrySource, /:\s*ASSISTANT_PROCUREMENT_PATH/);
	assert.match(assistantEntrySource, /href=\{href\}/);
	assert.match(assistantEntrySource, /<AssistantEntryContent\b/);
	assert.match(assistantEntryLayoutSource, /Hawkr Agent 幫你找/);
	assert.match(searchSource, /query=\{query\}/);
	assert.match(searchExperienceSource, /source="home-search"/);
});
