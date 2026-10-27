import { defineConfig, devices } from "@playwright/test";

/**
 * SEO invariant suite — runs the spec's "上線 QA Gate" against a build with the
 * indexability launch-gate OPEN (NEXT_PUBLIC_FLAVOR=prod + SEO_INDEXABLE=true),
 * API base pinned to an unreachable loopback address (a spec run without env can
 * never leave the machine; pages render their empty state) while canonicals resolve to the prod origin (env.webOrigin). Kept in
 * a SEPARATE config + testDir so the gate-closed main e2e run never touches it.
 */
// Build env: single source is package.json `build:seo` (the command below runs it).

// The served bundle is baked with the prod flavor, so its canonicals resolve to
// the origin named by NEXT_PUBLIC_WEB_ORIGIN (https://example.com). Specs that import lib/seo/urls
// recompute absoluteUrl() inside THIS runner process, so it must read the same
// flavor and origin — otherwise it falls back to "dev" (the local dev server)
// and every canonical assertion mismatches. Mirror the build env onto the runner env.
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
	// Invariants (H1/canonical/JSON-LD/meta) are viewport-independent → desktop
	// only. Mobile coverage for CWV is Lighthouse's job (lighthouserc.mobile.cjs).
	//
	// PW_CHANNEL 未設 → 走 Playwright 自帶的 chrome-headless-shell，正是 CI 的
	// `playwright install --force --only-shell` 裝的那顆，CI 不必多裝瀏覽器。
	// 本機沒有那顆（且此機到 Playwright CDN 實質不通，實測 30 分鐘只下載 1.5 MB），
	// 所以本機跑要 `PW_CHANNEL=chrome` 用系統 Chrome。沒有這個開關的話，「本地驗證
	// 全綠才 push」對 Playwright 這一層就做不到，只能推上去看 CI —— 那正是紅線 5
	// 明令禁止的「拿 CI 當 debug 工具」，而且是環境逼出來的，不是有人偷懶。
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
		// Always build with the gate open (NEXT_PUBLIC_* are baked at build), then
		// serve. Locally reuse a server you already started; in CI build fresh.
		//
		// 曾經是 `node .next/standalone/server.js`（理由：`next start` 不支援
		// `output: standalone`）。那個前提已經消失——`output: "standalone"` 於
		// 2026-07-02 隨「拔除 VM 部署路徑，收斂到 Cloudflare Workers」(ae9c3fd2)
		// 一併刪除，`pnpm build` 不再產出 `.next/standalone/`，於是這行從那天起
		// 必定 `Cannot find module …/.next/standalone/server.js`，整個 SEO 套件
		// 連 server 都起不來。沒人發現，是因為 CI 從來沒呼叫過 `test:seo`。
		command: "pnpm build:seo && pnpm start",
		url: "http://localhost:3000",
		reuseExistingServer: !process.env.CI,
		timeout: 300_000,
	},
});
