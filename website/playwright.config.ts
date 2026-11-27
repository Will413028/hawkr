import pkg from "@next/env";
import { defineConfig, devices } from "@playwright/test";

const { loadEnvConfig } = pkg;
loadEnvConfig(process.cwd());

const PORT = process.env.PLAYWRIGHT_PORT || "3000";
const SESSION_UPSTREAM_PORT = String(Number(PORT) + 1000);

export default defineConfig({
	testDir: "./tests",
	testMatch: "**/*.spec.ts",
	testIgnore: "**/seo/**",
	fullyParallel: true,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 2 : 1,
	reporter: [["list"], ["html", { open: "never" }]],
	expect: {
		timeout: 15_000,
	},
	use: {
		baseURL: `http://localhost:${PORT}`,
		trace: "retain-on-failure",
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
		{
			name: "mobile",
			use: {
				...devices["Pixel 5"],
				...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
			},
		},
	],
	webServer: [
		{
			command: "pnpm exec tsx tests/e2e/helpers/fake-session-upstream.main.ts",
			url: `http://127.0.0.1:${SESSION_UPSTREAM_PORT}/healthz`,
			reuseExistingServer: !process.env.CI,
			env: { FAKE_SESSION_UPSTREAM_PORT: SESSION_UPSTREAM_PORT },
			timeout: 30_000,
		},
		{
			command: process.env.CI ? "pnpm start" : "pnpm exec next dev",
			url: `http://localhost:${PORT}`,
			reuseExistingServer: !process.env.CI,
			env: {
				PORT,
				NEXT_PUBLIC_API_BASE_URL: `http://localhost:${PORT}/api/v1`,
				API_BASE_URL: `http://127.0.0.1:${SESSION_UPSTREAM_PORT}/api/v1`,
				PLAYWRIGHT_E2E_API_PROXY: "1",
				NEXT_PUBLIC_DEV_OVERLAYS: "off",
				NEXT_PUBLIC_GOOGLE_CLIENT_ID:
					"e2e-dummy-client-id.apps.googleusercontent.com",
				WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1: "true",
				WEB_PROCUREMENT_RFQ_APPROVAL_V1: "true",
			},
			timeout: 120_000,
		},
	],
});
