import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { playwright } from "@vitest/browser-playwright";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

function serveMswWorker(): Plugin {
	const worker = fileURLToPath(
		new URL("./tests/browser/mockServiceWorker.js", import.meta.url),
	);
	return {
		name: "hawkr:serve-msw-worker",
		configureServer(server) {
			server.middlewares.use("/mockServiceWorker.js", (_request, response) => {
				response.setHeader("Content-Type", "text/javascript");
				response.setHeader("Cache-Control", "no-store");
				response.end(readFileSync(worker));
			});
		},
	};
}

export default defineConfig({
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
					env: {
						API_BASE_URL: "http://127.0.0.1:9/api/v1",
						NEXT_PUBLIC_API_BASE_URL: "http://127.0.0.1:9/api/v1",
					},
					setupFiles: ["./tests/setup/hermetic.mjs", "./tests/setup-msw.ts"],
					server: { deps: { inline: ["next-intl"] } },
				},
			},
			{
				extends: true,
				plugins: [serveMswWorker()],
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
		],
	},
});
