import { defineConfig, devices } from "@playwright/test";

/**
 * SEO invariant suite — runs the spec's "上線 QA Gate" against a build with the
 * indexability launch-gate OPEN (NEXT_PUBLIC_FLAVOR=prod + SEO_INDEXABLE=true),
 * public API base same-origin (/api/v1) and API_BASE_URL pinned to an unreachable loopback address (a spec run without env can
 * never leave the machine; pages render their empty state) while canonicals resolve to the prod origin (env.webOrigin). Kept in
 * a SEPARATE config + testDir so the gate-closed main e2e run never touches it.
 */

process.env.NEXT_PUBLIC_FLAVOR ??= "prod";
process.env.NEXT_PUBLIC_SEO_INDEXABLE ??= "true";
process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED ??= "false";
process.env.NEXT_PUBLIC_WEB_ORIGIN ??= "https://example.com";
process.env.NEXT_PUBLIC_ADM_ORIGIN ??= "https://admin.example.com";

export default defineConfig({
	testDir: "./tests/seo",
	testMatch: "**/*.spec.ts",
	fullyParallel: true,
	retries: process.env.CI ? 2 : 0,
	workers: 1,
	reporter: "list",
	use: {
		baseURL: "http://localhost:3000",
		trace: "on-first-retry",
		actionTimeout: 15_000,
		navigationTimeout: 15_000,
	},
	projects: [
		{
			name: "chromium",
			use: {
				...devices["Desktop Chrome"],
				...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
			},
		},
	],
	webServer: {
		command: "pnpm build:seo && pnpm start",
		url: "http://localhost:3000",
		reuseExistingServer: !process.env.CI,
		timeout: 300_000,
	},
});
