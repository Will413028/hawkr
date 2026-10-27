import assert from "node:assert/strict";
import test from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

/**
 * next.config.ts decides its path rules ONCE, in its module body, from the routed locales of the
 * flavor it is loaded under. Each case gets its own module instance (a distinct import specifier)
 * with the flavor injected BEFORE that instance is evaluated — the same trick as seo-robots.test.ts.
 *
 * What these rules are for: `app/[locale]` pages are served at `/<locale>/…` too, and a path rule
 * that only knows the unprefixed path silently stops applying there (`/en/products` would answer 404
 * instead of 308).
 */

type Redirect = {
	source: string;
	destination: string;
	permanent: boolean;
	has?: unknown;
};
type HeaderRule = { source: string; headers: { key: string }[] };

async function loadConfig(
	flavor: "dev" | "prod",
	tag: string,
	overrides: { hawkrAgentEnabled?: boolean } = {},
) {
	__setFlavorForTest(flavor);
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), ...overrides });
	try {
		const config = (await import(`../next.config.ts?${tag}`)).default;
		return {
			redirects: (await config.redirects()) as Redirect[],
			headers: (await config.headers()) as HeaderRule[],
		};
	} finally {
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
}

test("dev routes /en, so every page redirect exists under /en too, with the prefix kept on both sides", async () => {
	const { redirects } = await loadConfig("dev", "dev-redirects");
	const find = (source: string) => redirects.find((r) => r.source === source);

	assert.deepEqual(find("/products"), {
		source: "/products",
		destination: "/market/products",
		permanent: true,
	});
	assert.deepEqual(find("/en/products"), {
		source: "/en/products",
		destination: "/en/market/products",
		permanent: true,
	});
	assert.equal(find("/en/explore")?.destination, "/en/feed");

	// every unprefixed page rule has its /en twin (the 307 Agent-legacy rules included)
	const unprefixed = redirects.filter(
		(r) => !r.source.startsWith("/en/") && !r.source.startsWith("/adm/"),
	);
	assert.ok(unprefixed.length >= 12, `${unprefixed.length} page redirects`);
	for (const rule of unprefixed) {
		const twin = find(`/en${rule.source}`);
		assert.ok(twin, `${rule.source} has no /en twin`);
		assert.equal(twin.permanent, rule.permanent, rule.source);
		assert.equal(
			twin.destination,
			rule.destination === "/" ? "/en" : `/en${rule.destination}`,
			rule.source,
		);
	}
});

test("the staff workspace redirect has no locale twin: /adm is not localized", async () => {
	const { redirects } = await loadConfig("dev", "dev-adm");
	assert.ok(redirects.some((r) => r.source === "/adm/:storeId(\\d+)"));
	assert.equal(
		redirects.some(
			(r) => r.source.includes("/adm/") && r.source.startsWith("/en"),
		),
		false,
	);
});

test("the Agent gate rules (production default) are localized too, and a bare '/' destination becomes '/en'", async () => {
	const { redirects } = await loadConfig("dev", "dev-agent-off", {
		hawkrAgentEnabled: false,
	});
	const preview = redirects.find(
		(r) => r.source === "/en/hawkr-agent-rfq-preview",
	);
	assert.deepEqual(preview, {
		source: "/en/hawkr-agent-rfq-preview",
		destination: "/en",
		permanent: false,
	});
	const assistant = redirects.find(
		(r) => r.source === "/en/account/messages/assistant/:path*" && r.has,
	);
	assert.equal(assistant?.destination, "/en/account/messages?requestId=");
});

test("prod routes no /en: its redirect and header rules are exactly the unprefixed ones, none added", async () => {
	const prod = await loadConfig("prod", "prod-rules");
	assert.equal(
		prod.redirects.some((r) => r.source.startsWith("/en")),
		false,
	);
	assert.equal(
		prod.headers.some((rule) => rule.source.startsWith("/en")),
		false,
	);
	// same rule list as dev minus the /en twins
	const dev = await loadConfig("dev", "dev-for-prod-compare", {
		hawkrAgentEnabled: false, // prod's default: the Agent gate rules exist there
	});
	assert.deepEqual(
		dev.redirects.filter((r) => !r.source.startsWith("/en/")),
		prod.redirects,
	);
});

test("dev adds exactly one private-route header rule per routed non-default locale, scoped to that prefix", async () => {
	const { headers } = await loadConfig("dev", "dev-headers");
	const localized = headers.filter((rule) => rule.source.startsWith("/en/"));
	assert.equal(localized.length, 1);
	assert.deepEqual(
		localized[0].headers.map((h) => h.key),
		["Cache-Control"],
	);
	// the staff and infrastructure prefixes have no locale, so they are not in the /en rule
	for (const prefix of ["adm", "api", "sentinel"]) {
		assert.equal(
			localized[0].source.includes(`|${prefix}|`) ||
				localized[0].source.includes(`(${prefix}|`) ||
				localized[0].source.includes(`|${prefix})`),
			false,
			prefix,
		);
	}
});
