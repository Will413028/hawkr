import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { unstable_getResponseFromNextConfig } from "next/dist/experimental/testing/server/config-testing-utils.js";
import { __setEnvForTest, resolveHawkrAgentEnabled } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const root = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

test("Hawkr Agent top-level gate defaults on in DEV and off in production", () => {
	assert.equal(resolveHawkrAgentEnabled("dev", undefined), true);
	assert.equal(resolveHawkrAgentEnabled("prod", undefined), false);
	assert.equal(resolveHawkrAgentEnabled("dev", "false"), false);
	assert.equal(resolveHawkrAgentEnabled("prod", "true"), true);
	assert.equal(resolveHawkrAgentEnabled("prod", "TRUE"), false);
	assert.equal(resolveHawkrAgentEnabled("prod", "1"), false);
});

test("all user-facing Agent entries consume the same top-level gate", () => {
	const menu = read("features/account/menu.ts");
	const entry = read("features/procurement-assistant/AssistantEntryLink.tsx");
	const homeSearch = read("features/home/components/HomeCompactSearch.tsx");
	const searchForm = read("features/search/components/SearchForm.tsx");
	const searchResults = read(
		"features/search/components/SearchResultsClient.tsx",
	);
	const messagesLayout = read(
		"app/[locale]/(account)/account/messages/layout.tsx",
	);
	const conversations = read(
		"features/procurement-assistant/useAgentConversations.ts",
	);

	assert.match(
		menu,
		/\.\.\.\(HAWKR_AGENT_ENABLED \? \[HAWKR_AGENT_MENU_ITEM\] : \[\]\)/,
	);
	assert.match(entry, /if \(!HAWKR_AGENT_ENABLED\) return null/);
	for (const source of [homeSearch, searchForm, searchResults]) {
		assert.match(source, /HAWKR_AGENT_ENABLED \? \(/);
	}
	assert.match(messagesLayout, /useAgentConversations\(HAWKR_AGENT_ENABLED\)/);
	assert.match(messagesLayout, /assistantEntry=\{\s*HAWKR_AGENT_ENABLED/);
	assert.match(
		conversations,
		/const shouldFetch = enabled && hydrated && isLoggedIn/,
	);
});

test("search keeps Agent beside the form on wide screens and stacked when narrow", () => {
	const searchForm = read("features/search/components/SearchForm.tsx");

	assert.match(searchForm, /max-w-\[880px\]/);
	assert.match(searchForm, /min-\[1280px\]:max-w-none/);
	assert.match(
		searchForm,
		/min-\[1280px\]:grid-cols-\[minmax\(0,760px\)_minmax\(320px,1fr\)\]/,
	);
	assert.match(searchForm, /min-\[1280px\]:items-start/);
	assert.match(searchForm, /min-\[1280px\]:gap-4/);
	assert.match(
		searchForm,
		/<form[\s\S]*<AssistantEntryLink[\s\S]*className="search-page__agent-entry"/,
	);
});

test("production HTTP redirects and page-level guards block every Agent route", () => {
	const config = read("next.config.ts");
	for (const source of [
		"/account/messages/assistant/:path*",
		"/account/find-sellers",
		"/hawkr-agent-rfq-preview",
		"/hawkr-agent-rfq-preview/:path*",
	]) {
		assert.match(
			config,
			new RegExp(`source: "${source.replaceAll("*", "\\*")}"`),
		);
	}

	for (const route of [
		"app/[locale]/(account)/account/messages/assistant/procurement/page.tsx",
		"app/[locale]/(account)/account/messages/assistant/[conversationId]/page.tsx",
	]) {
		assert.match(
			read(route),
			/if \(!HAWKR_AGENT_ENABLED\) \{\s*redirect\(\{\s*href: "\/account\/messages",\s*locale: localeFromParam\(/,
		);
	}

	for (const route of [
		"app/[locale]/hawkr-agent-rfq-preview/page.tsx",
		"app/[locale]/hawkr-agent-rfq-preview/[screen]/page.tsx",
		"app/[locale]/hawkr-agent-rfq-preview/external-vendor/[step]/page.tsx",
	]) {
		assert.match(read(route), /if \(!HAWKR_AGENT_ENABLED\) notFound\(\)/);
	}

	const analyticsPrivacy = read("lib/analytics/page-view-privacy.ts");
	assert.match(analyticsPrivacy, /CLICK_ATTRIBUTION_QUERY_KEYS/);
	assert.match(analyticsPrivacy, /UTM_QUERY_KEYS\.has\(key\)/);
});

test("the first production redirect blanks requestId before landing on messages", async () => {
	const originalFlag = process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED;
	process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED = "false";
	__setFlavorForTest("prod");
	__setEnvForTest(null);

	try {
		const { default: nextConfig } = await import("../next.config.ts");
		const response = await unstable_getResponseFromNextConfig({
			url: "https://example.com/account/messages/assistant/procurement?requestId=req-secret-123&utm_source=internal",
			nextConfig,
		});

		assert.equal(response.status, 307);
		const location = new URL(response.headers.get("location") as string);
		assert.equal(location.pathname, "/account/messages");
		assert.equal(location.searchParams.get("requestId"), "");
		assert.equal(location.searchParams.get("utm_source"), "internal");
		assert.doesNotMatch(location.href, /req-secret-123/);
	} finally {
		if (originalFlag === undefined) {
			delete process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED;
		} else {
			process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED = originalFlag;
		}
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
});
