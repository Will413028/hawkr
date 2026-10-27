import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { StorybookConfig } from "@storybook/nextjs-vite";
import tailwindcss from "@tailwindcss/postcss";
import { normalizeStorybookBasePath } from "../scripts/storybook-paths.mjs";
import { catalogAssetPlugin, catalogCssAssetPlugin } from "./catalog-assets";

const basePath = normalizeStorybookBasePath(
	process.env.STORYBOOK_BASE_PATH || "/",
);
const webRoot = fileURLToPath(new URL("..", import.meta.url));

const config: StorybookConfig = {
	framework: "@storybook/nextjs-vite",
	stories: ["../stories/**/*.mdx", "../stories/**/*.stories.tsx"],
	staticDirs: [
		"../public",
		"./public",
		{ from: "../assets/fonts", to: "/assets/fonts" },
		...(existsSync(new URL("../output/flutter", import.meta.url))
			? [{ from: "../output/flutter", to: "/flutter" }]
			: []),
	],
	addons: [
		"@storybook/addon-docs",
		"@storybook/addon-a11y",
		"@storybook/addon-vitest",
		"@storybook/addon-mcp",
	],
	features: { componentsManifest: true },
	core: { disableTelemetry: true },
	async viteFinal(config) {
		// Explicit plugin preserves Tailwind v4 import/source semantics in Vite.
		config.base = basePath;
		config.define = {
			...config.define,
			__STORYBOOK_BASE_PATH__: JSON.stringify(basePath),
		};
		config.plugins = [
			...(config.plugins || []),
			catalogAssetPlugin(webRoot, basePath),
		];
		// Clerk 需要 provider、網路與 application key；預覽一律用未登入的替身（clerk-mock.tsx）。
		config.resolve = {
			...config.resolve,
			alias: [
				...(Array.isArray(config.resolve?.alias)
					? config.resolve.alias
					: Object.entries(config.resolve?.alias ?? {}).map(
							([find, replacement]) => ({ find, replacement }),
						)),
				{
					find: /^@clerk\/nextjs$/,
					replacement: fileURLToPath(
						new URL("./clerk-mock.tsx", import.meta.url),
					),
				},
			],
		};
		config.css = {
			...config.css,
			postcss: { plugins: [tailwindcss(), catalogCssAssetPlugin(basePath)] },
		};
		return config;
	},
};

export default config;
