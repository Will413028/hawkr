import path from "node:path";
import type { Plugin as PostCssPlugin } from "postcss";
import { parser, parserOptions, traverse } from "storybook/internal/babel";
import type { Plugin as VitePlugin } from "vite";
import {
	rebaseCatalogAssetUrl,
	rebaseCatalogCssUrl,
} from "../scripts/storybook-paths.mjs";

/** Build transform only: production modules keep their original public asset URLs. */
export function rebaseCatalogJavaScript(code: string, basePath: string) {
	if (basePath === "/") return null;
	const changes: { start: number; end: number; text: string }[] = [];
	// babelParse uses Recast, which normalizes tabs and line endings before
	// parsing. Slice the original source only with offsets from the raw parser.
	const ast = parser.parse(code, {
		...parserOptions,
		plugins: [
			...(parserOptions.plugins ?? []),
			/^\s*\/\/\s*@flow/.test(code) ? "flow" : "typescript",
		],
	});
	traverse(ast, {
		StringLiteral(reference) {
			if (
				reference.parentPath.isImportDeclaration() ||
				reference.parentPath.isExportNamedDeclaration() ||
				reference.parentPath.isExportAllDeclaration()
			)
				return;
			const node = reference.node;
			const rebased = rebaseCatalogAssetUrl(node.value, basePath);
			if (rebased !== node.value && node.start != null && node.end != null) {
				changes.push({
					start: node.start,
					end: node.end,
					text: JSON.stringify(rebased),
				});
			}
		},
		TemplateLiteral(reference) {
			const first = reference.node.quasis[0];
			if (!first || first.start == null || first.end == null) return;
			const rebased = rebaseCatalogAssetUrl(first.value.raw, basePath);
			if (rebased !== first.value.raw) {
				changes.push({ start: first.start, end: first.end, text: rebased });
			}
		},
	});
	if (!changes.length) return null;
	let result = code;
	for (const change of changes.sort((a, b) => b.start - a.start)) {
		result =
			result.slice(0, change.start) + change.text + result.slice(change.end);
	}
	return result;
}

export function catalogAssetPlugin(
	webRoot: string,
	basePath: string,
): VitePlugin {
	return {
		name: "hawkr-catalog-static-asset-base",
		enforce: "pre",
		transform(code, id) {
			const filename = id.split("?", 1)[0];
			const relative = path
				.relative(webRoot, filename)
				.split(path.sep)
				.join("/");
			if (
				!/^(components|features|lib|stories)\//.test(relative) ||
				!/\.[cm]?[jt]sx?$/.test(filename)
			)
				return;
			const transformed = rebaseCatalogJavaScript(code, basePath);
			return transformed ? { code: transformed, map: null } : undefined;
		},
	};
}

/** Runs after Tailwind expands imports, before Vite serializes the stylesheet. */
export function catalogCssAssetPlugin(basePath: string): PostCssPlugin {
	return {
		postcssPlugin: "hawkr-catalog-css-asset-base",
		OnceExit(root) {
			root.walkDecls((declaration) => {
				declaration.value = declaration.value.replace(
					/url\(\s*(["']?)([^"')\s]+)\1\s*\)/g,
					(match, quote: string, url: string) => {
						const rebased = rebaseCatalogCssUrl(url, basePath);
						return rebased === url ? match : `url(${quote}${rebased}${quote})`;
					},
				);
			});
		},
	};
}
