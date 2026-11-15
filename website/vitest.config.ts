import { fileURLToPath } from "node:url";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig } from "vitest/config";

export default defineConfig({
	// 對齊 tsconfig paths `@/*` → 專案根，否則 unit test 透過模組圖載入 @/ 別名會解析失敗。
	resolve: {
		alias: {
			"@": fileURLToPath(new URL(".", import.meta.url)),
		},
	},
	test: {
		projects: [
			{
				extends: true,
				test: {
					name: "unit",
					include: ["**/__tests__/**/*.test.ts", "**/*.unit.test.ts"],
					exclude: [
						"node_modules",
						".next",
						"output/**",
						"tests/**",
						"**/.worktrees/**",
						".worktrees/**",
					],
					environment: "node",
					// 單元測試不得連到真的主機：dev flavor 的 API base 預設是舊公司的 gateway，
					// 這裡釘到 loopback 的丟棄埠。tests/setup/hermetic.mjs 再讓每一條出站連線（fetch、
					// http(s)、WebSocket、TCP）拒絕非 loopback 的主機，tests/setup-msw.ts 讓沒有 handler
					// 的請求使該測試失敗。
					env: {
						API_BASE_URL: "http://127.0.0.1:9/api/v1",
						NEXT_PUBLIC_API_BASE_URL: "http://127.0.0.1:9/api/v1",
					},
					setupFiles: ["./tests/setup/hermetic.mjs", "./tests/setup-msw.ts"],
					// middleware.ts imports next-intl/middleware, whose ESM build imports
					// `next/server` without an extension. Node's own resolver rejects that for an
					// externalized package; inlining it lets Vite resolve it like the app does.
					server: { deps: { inline: ["next-intl"] } },
				},
			},
			// 元件 browser 測試：只由 `pnpm test:browser` 跑（`--project browser`），不進 `pnpm test`。
			{
				extends: true,
				// `next/image` 是 CJS，預先打包後 default 匯入成物件；解析時就換成 tests/browser/next-image.tsx，
				// 並排除在預先打包之外。只比對完整的 `next/image`，`next/image-*` 等其他子路徑不動。
				resolve: {
					alias: [
						{
							find: /^next\/image$/,
							replacement: fileURLToPath(
								new URL("./tests/browser/next-image.tsx", import.meta.url),
							),
						},
					],
				},
				optimizeDeps: { include: ["framer-motion"], exclude: ["next/image"] },
				// 瀏覽器沒有 Node 的 process.env：照 Next 的 client 編譯在建置時替換。API base
				// 釘到 loopback 的丟棄埠，沒有 mock 的請求不會連到真的主機；其餘未設的
				// NEXT_PUBLIC_* 由 tests/browser/setup.ts 的 process 墊片讀成 undefined。
				define: {
					"process.env.NODE_ENV": JSON.stringify("test"),
					"process.env.NEXT_PUBLIC_API_BASE_URL": JSON.stringify(
						"http://127.0.0.1:9/api/v1",
					),
				},
				test: {
					name: "browser",
					include: ["**/*.browser.test.tsx"],
					exclude: [
						"node_modules",
						".next",
						"output/**",
						"**/.worktrees/**",
						".worktrees/**",
					],
					setupFiles: ["./tests/browser/setup.ts"],
					browser: {
						enabled: true,
						headless: true,
						provider: playwright(),
						instances: [{ browser: "chromium" }],
					},
				},
			},
			// The Next.js Storybook plugin loads app env/Next image settings at
			// config time. Keep it out of the hermetic unit-test process.
			...(process.env.VITEST_STORYBOOK === "true"
				? ["./vitest.storybook.config.ts"]
				: []),
		],
	},
});
