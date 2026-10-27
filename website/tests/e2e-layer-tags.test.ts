import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { hermeticEnv } from "./helpers/hermetic-env";

const repoRoot = fileURLToPath(new URL("../", import.meta.url));

/**
 * Every Playwright test in the main config carries two orthogonal labels:
 *
 *   INTENT (exactly one)   `@ui`       — the assertion is about our own
 *                                        document/CSS/JS: routing, hydration,
 *                                        geometry, pixels, link integrity.
 *                          `@contract` — the assertion is about the backend's
 *                                        answer: field names, resolvable ids,
 *                                        data-driven redirects.
 *
 *   DEPENDENCY (optional)  `@live`     — the outcome depends on the shared dev
 *                                        gateway answering (SSR fetch, browser
 *                                        fetch, or a fixture lookup). No `@live`
 *                                        = hermetic: mocked gateway, or a route
 *                                        that renders no backend data.
 *
 * The dependency label is what the CI ladder is cut on, because that is what
 * actually costs backend round-trips and inherits shared-environment drift.
 * The intent label is what says where the recorded-fixture work should aim:
 * `@ui ∩ @live` is exactly the set that pays for a backend it does not assert
 * anything about (hawkr-website#369).
 *
 * WHY THIS TEST EXISTS — the dev-push E2E tier used to be a hardcoded list of
 * four spec paths inside deploy.yml. A spec added after that list was written
 * belonged to no tier and nothing said so. Tags fix the expressiveness problem;
 * this test fixes the completeness problem. An untagged test is a test whose
 * ladder nobody chose.
 *
 * HOW TO RE-VERIFY THE HERMETIC SIDE (run it before trusting a new tag):
 *
 *   API_BASE_URL=http://127.0.0.1:9/api/v1 CI=1 PW_CHANNEL=chrome \
 *     pnpm run test:e2e:hermetic --project=chromium
 *
 * Pointing the gateway at a closed port is what makes "hermetic" a measured
 * property rather than an opinion — a test that secretly needs the backend
 * fails there instead of quietly passing on a warm ISR cache. The first run of
 * this (2026-08-01) moved 24 tests I had assumed were hermetic into `@live`.
 *
 * NOTE ON WHICH SIDE IS PROVEN — the hermetic set is verified by that run; the
 * `@live` set is a deliberate over-approximation. Mis-tagging live-as-hermetic
 * breaks the dev tier whenever the shared gateway hiccups; mis-tagging
 * hermetic-as-live only costs a test that runs less often than it could. Use
 * the command above to shrink `@live` when you want to reclaim one — do not
 * move a test into the hermetic set on reasoning alone.
 *
 * The Playwright JSON reporter strips the leading `@` from tag names, so the
 * expected values here are bare words while the source and `--grep` use `@`.
 */
const INTENT_TAGS = ["ui", "contract"];

interface ListedSpec {
	title: string;
	file: string;
	line: number;
	tags?: string[];
}

interface ListedSuite {
	file?: string;
	specs?: ListedSpec[];
	suites?: ListedSuite[];
}

function collectSpecs(suite: ListedSuite, file?: string): ListedSpec[] {
	const own = suite.file ?? file;
	const specs = (suite.specs ?? []).map((spec) => ({
		...spec,
		file: spec.file ?? own ?? "(unknown)",
	}));
	for (const child of suite.suites ?? []) {
		specs.push(...collectSpecs(child, own));
	}
	return specs;
}

function listMainConfigSpecs(): ListedSpec[] {
	// `--list` only resolves the test tree: it starts no web server and launches
	// no browser, so this stays runnable in the lint/unit job that never installs
	// Playwright browsers. Measured at ~0.7s.
	const stdout = execFileSync(
		"node",
		[
			"node_modules/@playwright/test/cli.js",
			"test",
			"--list",
			"--reporter=json",
		],
		{
			cwd: repoRoot,
			encoding: "utf8",
			env: hermeticEnv(),
			maxBuffer: 64 * 1024 * 1024,
		},
	);
	const report = JSON.parse(stdout) as { suites: ListedSuite[] };
	return report.suites.flatMap((suite) => collectSpecs(suite));
}

test("every Playwright test declares exactly one layer intent tag", () => {
	const specs = listMainConfigSpecs();
	assert.ok(specs.length > 0, "playwright --list returned no tests");

	const offenders = specs
		.map((spec) => ({
			spec,
			intents: (spec.tags ?? []).filter((tag) => INTENT_TAGS.includes(tag)),
		}))
		.filter(({ intents }) => intents.length !== 1)
		.map(
			({ spec, intents }) =>
				`${spec.file}:${spec.line} › ${spec.title} → [${intents.join(", ") || "none"}]`,
		);

	assert.deepEqual(
		[...new Set(offenders)],
		[],
		"每個 e2e test 必須帶且只帶一個意圖標籤（@ui 或 @contract），另可選擇性加 @live。\n" +
			"沒有標籤的測試 = 沒有人決定它該在哪個 CI 階梯上跑（這正是 deploy.yml 舊的四檔硬編碼清單的失效模式）。\n" +
			offenders.join("\n"),
	);
});

test("the hermetic and live layers partition the whole suite", () => {
	const specs = listMainConfigSpecs();
	const live = specs.filter((spec) => (spec.tags ?? []).includes("live"));
	const hermetic = specs.filter((spec) => !(spec.tags ?? []).includes("live"));

	assert.equal(
		live.length + hermetic.length,
		specs.length,
		"@live / hermetic must be a partition — `--grep @live` plus `--grep-invert @live` has to cover every test exactly once",
	);
	assert.ok(
		live.length > 0 && hermetic.length > 0,
		`both layers must be non-empty (live=${live.length}, hermetic=${hermetic.length}); ` +
			"an empty layer means a CI step is silently running nothing",
	);
});
