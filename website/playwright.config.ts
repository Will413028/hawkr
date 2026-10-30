import path from "node:path";
import pkg from "@next/env";
import { defineConfig, devices } from "@playwright/test";

// Load .env.local so any required secrets are available in the test process.
// This mirrors the environment the Next.js dev server runs in.
const { loadEnvConfig } = pkg;
loadEnvConfig(process.cwd());

// CI runs many refs concurrently on the same self-hosted runner pool (the
// web-deploy-${{ github.ref }} concurrency group only serializes same-ref
// runs). A fixed port meant two unrelated runs landing on the same runner
// would collide ("address already in use"). deploy.yml derives a unique
// PLAYWRIGHT_PORT per run (run id + attempt) and exports it before this
// config loads; local dev has no such env var and stays on 3000.
const PORT = process.env.PLAYWRIGHT_PORT || "3000";

export default defineConfig({
	testDir: "./tests",
	testMatch: "**/*.spec.ts",
	// `tests/seo/**` 由 playwright.seo.config.ts 承接，不是「被排除掉沒人跑」——
	// 執行入口是 deploy.yml 的「🔍 SEO 不變式閘門（tests/seo）」step（`pnpm run
	// test:seo`，與本 config 的全量 E2E 同一階梯：main push / PR 進 main）。
	//
	// 這一行是**必要的**，不能為了「讓 SEO 測試也跑到」而拿掉：那批斷言的前提是
	// indexability gate 打開的建置物（NEXT_PUBLIC_FLAVOR=prod +
	// NEXT_PUBLIC_SEO_INDEXABLE=true），而本 config 的 webServer 服務的是這個 job
	// 前面用 dev flavor build 出來的產物，gate 是關的。實測（2026-07-31）：拿掉這
	// 行後 `tests/seo/sitemap-robots.seo.spec.ts` 立刻兩紅——robots.txt 是
	// `Disallow: /`（`app/robots.ts` 的 `buildRobots(false)`，dev flavor 的正確行為），
	// 於是 "robots policy (gate open)" 兩支必然失敗。拿掉 = 在主 E2E 注入兩個
	// 恆紅的假失敗，不是補上覆蓋。
	testIgnore: "**/seo/**",
	fullyParallel: true,
	// One retry-on-CI absorbs the residual first-hit nondeterminism a loaded
	// shared runner still produces even against a prebuilt server.
	retries: process.env.CI ? 2 : 0,
	// CI 的 job 是 4 vCPU，跑一個 next server 加兩個 Chromium；測試彼此獨立（見 website-e2e.yml 的 shard 說明）。
	// 本機維持 1，避免和其他 session 搶資源。
	workers: process.env.CI ? 2 : 1,
	// "list" prints to the CI log; "html" bundles retained failing traces in
	// playwright-report/ for local or runner-side diagnosis. The deploy artifact
	// policy deliberately keeps reports off GitHub Actions storage. open:"never"
	// prevents CI from trying to open the report in a browser.
	reporter: [["list"], ["html", { open: "never" }]],
	expect: {
		timeout: 15_000,
		toHaveScreenshot: {
			// CSS → Tailwind migration must be pixel-faithful. Use an ABSOLUTE
			// pixel budget, not a ratio: full-page mobile shots are very tall at
			// DPR 2.75, so a ratio dilutes localized regressions (a whole tabs bar
			// changing width stayed under 0.5%). An absolute budget catches a
			// localized change regardless of page height while tolerating
			// sub-pixel AA noise on text-heavy pages.
			maxDiffPixels: 120,
			animations: "disabled",
			caret: "hide",
			// Hides the dev-only Next.js indicator so it never counts as a diff.
			stylePath: path.join(process.cwd(), "tests/visual/screenshot.css"),
		},
	},
	use: {
		baseURL: `http://localhost:${PORT}`,
		// Keep the original failing attempt, not only a retry that may pass after
		// the timing window has disappeared. Streaming/hydration races need the
		// first DOM/network timeline to remain diagnosable.
		trace: "retain-on-failure",
		actionTimeout: 15_000,
		navigationTimeout: 15_000,
	},
	// PW_CHANNEL 未設 → 走 Playwright 自帶的 chrome-headless-shell，正是 CI 的
	// `playwright install --force --only-shell` 裝的那顆，CI 不必多裝瀏覽器。
	// 本機沒有那顆（且此機到 Playwright CDN 實質不通，實測 30 分鐘只下載 1.5 MB），
	// 所以本機跑要 `PW_CHANNEL=chrome` 用系統 Chrome。沒有這個開關的話，「本地驗證
	// 全綠才 push」對 Playwright 這一層就做不到，只能推上去看 CI —— 那正是紅線 5
	// 明令禁止的「拿 CI 當 debug 工具」，而且是環境逼出來的，不是有人偷懶。
	// 2026-07-31 之前每次要在本機跑都得手動改這一段再改回去，改一次漏一次。
	projects: [
		{
			name: "chromium",
			use: {
				...devices["Desktop Chrome"],
				...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
			},
		},
		{
			name: "mobile",
			use: {
				...devices["Pixel 5"],
				...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
			},
		},
	],
	webServer: {
		// In CI, test against a PRODUCTION build: `next dev` compiles routes
		// on-demand, so the first navigation to a cold route (e.g. "/", "/login")
		// can blow the 15s navigationTimeout on a loaded runner — the source of
		// the flaky page.goto timeouts. `next build` precompiles every route, so
		// `next start` serves instantly and deterministically.
		//
		// The CI job already runs `pnpm run build` as a separate "cacheComponents
		// gate" step (.github/workflows/deploy.yml) BEFORE the E2E step, so we only
		// `pnpm start` here — reusing that artifact. Folding the build into the
		// webServer command would (a) build twice and (b) bury a build failure
		// inside Playwright's webServer boot instead of failing at the gate step.
		// Locally keep the dev server for fast iteration.
		// `next dev` on an explicit PORT (`pnpm dev` is the same command on the
		// default port 3000) so the local server matches `url` below.
		command: process.env.CI ? "pnpm start" : "pnpm exec next dev",
		url: `http://localhost:${PORT}`,
		reuseExistingServer: !process.env.CI,
		// E2E browser base must be SAME-ORIGIN with this server so the browser
		// exercises the first-party API paths (/api/connect, /api/v1) the way a
		// deployment does. SSR inherits the branch-correct gateway
		// loaded from .env.local: main exercises the formal API, while dev/local
		// defaults to the test API.
		// (CI's pnpm-start build bakes the matching PORT-scoped value via
		// .env.local in deploy.yml — NEXT_PUBLIC_* is inlined at build time, so
		// setting it here only affects local `next dev`, not the CI prebuilt run.)
		env: {
			PORT,
			NEXT_PUBLIC_API_BASE_URL: `http://localhost:${PORT}/api/v1`,
			API_BASE_URL: process.env.API_BASE_URL ?? "http://127.0.0.1:9/api/v1",
			PLAYWRIGHT_E2E_API_PROXY: "1",
			// dev-only 浮動工具（角色切換器 / React Query Devtools）在本機 `next dev`
			// 下會 render，production build 不會 —— 於是本機量到的 DOM 與 CI 不是同
			// 一份。實測（web#425）：兩顆都是右下角 viewport-fixed，整個蓋在手機底部
			// 導覽列第 5 格上，mobile project 的點擊被攔截、3/3 紅，而 CI 上同一支
			// mobile 測試一直是綠的。關掉它們，本機測的才是 CI 會測到的那份 DOM。
			//
			// 只對本機 `next dev` 生效，這是對的：CI 的 `pnpm start` 服務的是先前
			// build 好的產物，NEXT_PUBLIC_* 在 build 期就被字面替換掉了（同上面
			// NEXT_PUBLIC_API_BASE_URL 那段註解），而那份產物本來就是 production，
			// 兩顆都不存在。
			NEXT_PUBLIC_DEV_OVERLAYS: "off",
			// 廠商入口二次驗證（GIS）的 Google client id：測試一律用這個假值，**不吃 .env.local**。
			//
			// 🔴 不設的話行為會隨環境漂：有 .env.local 的機器上 GSI 元件會渲染，
			// 乾淨的 worktree（或 CI 的某些 job）上元件會走「尚未設定」分支而整個不畫
			// 按鈕——同一支測試在兩台機器上測到的是不同的畫面。真正的 Google 網路請求
			// 由 tests/e2e/helpers/mock-gsi.ts 的 window.google stub 擋掉，所以這裡
			// 放假值是安全的。
			NEXT_PUBLIC_GOOGLE_CLIENT_ID:
				"e2e-dummy-client-id.apps.googleusercontent.com",
			// RFQ specs exercise the enabled cohort explicitly. Production/default
			// remains fail-safe OFF in capabilities.ts; the browser fixture turns on
			// only the two Web presentation gates. The tests exercise price
			// preview and immutable confirmation; formal publication remains a
			// separate, visibly disabled capability in the UI.
			WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1: "true",
			WEB_PROCUREMENT_RFQ_APPROVAL_V1: "true",
		},
		// `next start` boot only — the build already ran at the gate step.
		timeout: 120_000,
	},
});
