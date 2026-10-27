/**
 * 從 `@material-symbols/svg-400` 取出 `src/names.ts` 列出的圖示，寫成
 * `src/material-symbols.gen.ts`（名稱 → outlined 與 filled 的 path data）。
 * 輸出只由名稱清單與鎖定的上游版本決定，重跑沒有 diff。
 *
 * 用法（在 repo 根目錄）：
 *   pnpm --filter @hawkr/icons gen
 * tests/drift.test.ts 在記憶體重跑 `render()`，與 checked-in 的生成檔逐字比對。
 */
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { MATERIAL_SYMBOL_NAMES } from "../src/names";

const UPSTREAM_PACKAGE = "@material-symbols/svg-400";
const UPSTREAM_VERSION = "0.48.1";
const VIEW_BOX = "0 -960 960 960";

const upstreamRoot = new URL(
	`../node_modules/${UPSTREAM_PACKAGE}/`,
	import.meta.url,
);
const OUT_PATH = fileURLToPath(
	new URL("../src/material-symbols.gen.ts", import.meta.url),
);

function readPath(file: string): string {
	const svg = readFileSync(new URL(`outlined/${file}`, upstreamRoot), "utf8");
	if (!svg.includes(`viewBox="${VIEW_BOX}"`)) {
		throw new Error(`${file}: viewBox 不是 ${VIEW_BOX}`);
	}
	const paths = [...svg.matchAll(/<path d="([^"]+)"\/>/g)];
	if (paths.length !== 1) {
		throw new Error(`${file}: 預期剛好一個 <path>，實際 ${paths.length}`);
	}
	return paths[0][1];
}

export function render(): string {
	const version = (
		JSON.parse(readFileSync(new URL("package.json", upstreamRoot), "utf8")) as {
			version: string;
		}
	).version;
	if (version !== UPSTREAM_VERSION) {
		throw new Error(
			`${UPSTREAM_PACKAGE} 版本是 ${version}，生成器鎖定 ${UPSTREAM_VERSION}`,
		);
	}
	const names = [...MATERIAL_SYMBOL_NAMES].sort();
	if (new Set(names).size !== names.length) {
		throw new Error("names.ts 有重複名稱");
	}
	const entries = names.map((name) => {
		const outlined = readPath(`${name}.svg`);
		const filled = readPath(`${name}-fill.svg`);
		return `\t${name}: {\n\t\toutlined: ${JSON.stringify(outlined)},\n\t\tfilled: ${JSON.stringify(filled)},\n\t},`;
	});
	return `// 生成檔，不得手改。由 tools/gen.ts 依 src/names.ts 生成。
// 來源：${UPSTREAM_PACKAGE} ${UPSTREAM_VERSION}（Material Symbols，Apache-2.0，見 ../LICENSE）。
// 修改：只取用到的 path。

export const MATERIAL_SYMBOL_VIEW_BOX = "${VIEW_BOX}";

export const MATERIAL_SYMBOL_PATHS = {
${entries.join("\n")}
} as const;
`;
}

if (
	process.argv[1] &&
	import.meta.url === pathToFileURL(process.argv[1]).href
) {
	writeFileSync(OUT_PATH, render());
}
