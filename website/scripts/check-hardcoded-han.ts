#!/usr/bin/env node

/**
 * 硬寫中文的 ratchet（多語系 ADR D4 與 2026-10-22 修訂；計畫 I-CI）。
 *
 * 會發佈 en 的 website 程式裡，含漢字的字面值（字串、樣板字串、JSX 文字；註解不算）只能減少：
 * baseline（`hardcoded-han-baseline.json`）逐筆記「檔案 → 字面值內容 → 次數」，不記行號。
 * - 檢查：某檔某字面值的次數超過 baseline（含 baseline 沒有的新檔、新字面值）就失敗；
 *   次數少於 baseline 也失敗，要求執行 `--update` 收緊，所以 baseline 只會跟著變少（betterer 的做法）。
 * - `--update`：重寫 baseline，只在「總筆數不增加、每個字面值的全域總次數不增加，且新檔是已消失舊檔的改名或搬移」時允許。
 * 用 TypeScript AST，不用 regex。
 *
 * - `--init`：第一次建立 baseline；baseline 已存在時拒絕。`--update` 與檢查在 baseline 不存在時失敗。
 *
 * 用法：tsx scripts/check-hardcoded-han.ts [--init | --update]
 */

import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";

/** 掃描的頂層目錄與檔案（相對 website 根）。 */
export const SCAN_ROOTS = [
	"app",
	"components",
	"features",
	"lib",
	"i18n",
	"middleware.ts",
] as const;

/**
 * 排除規則集中在這裡：代行工作區 `app/(delegated)` 只有 zh-TW（不發佈 en）；測試不是使用者面文字；
 * 生成檔（`*.gen.*`、`*.g.*`）不得手改，由各自的 drift 檢查守。`messages/*.json` 不是 ts，本來就不掃。
 */
export function isExcluded(rel: string): boolean {
	const path = rel.split("\\").join("/");
	if (path.startsWith("app/(delegated)/")) return true;
	if (path.split("/").some((s) => s === "tests" || s === "__tests__"))
		return true;
	const name = path.slice(path.lastIndexOf("/") + 1);
	if (DECLARATION.test(name)) return true;
	if (/\.(test|spec|unit\.test)\.[cm]?[jt]sx?$/.test(name)) return true;
	if (/\.(gen|g)\.[cm]?[jt]sx?$/.test(name)) return true;
	return false;
}

/** 只算漢字（Script=Han，含擴充區）：計畫只管漢字，標點（、。）與注音符號不算硬寫中文。 */
const HAN = /\p{Script=Han}/u;

/** 掃描的副檔名；`.d.ts` 系列只有型別，不掃。 */
const SOURCE_EXT = /\.(?:[cm]?[jt]s|[jt]sx)$/;
const DECLARATION = /\.d\.[cm]?ts$/;

function scriptKind(fileName: string): ts.ScriptKind {
	if (fileName.endsWith(".tsx")) return ts.ScriptKind.TSX;
	if (fileName.endsWith(".jsx")) return ts.ScriptKind.JSX;
	if (/\.[cm]?js$/.test(fileName)) return ts.ScriptKind.JS;
	return ts.ScriptKind.TS;
}

/** 一個檔裡所有含漢字的字面值（保留重複）。樣板字串的各段合併成一筆，`${}` 以 `${}` 佔位。 */
export function collectHanLiterals(fileName: string, text: string): string[] {
	const sf = ts.createSourceFile(
		fileName,
		text,
		ts.ScriptTarget.Latest,
		true,
		scriptKind(fileName),
	);
	const out: string[] = [];
	const add = (s: string): void => {
		if (HAN.test(s)) out.push(s);
	};
	const visit = (n: ts.Node): void => {
		if (ts.isStringLiteral(n) || ts.isNoSubstitutionTemplateLiteral(n)) {
			add(n.text);
		} else if (ts.isTemplateExpression(n)) {
			add(
				n.head.text +
					n.templateSpans.map((s) => `\${}${s.literal.text}`).join(""),
			);
		} else if (ts.isJsxText(n)) {
			add(n.text.replace(/\s+/g, " ").trim());
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return out;
}

/** 檔案相對路徑 → 字面值內容 → 次數。 */
export type Counts = Record<string, Record<string, number>>;

function cmp(a: string, b: string): number {
	return a < b ? -1 : a > b ? 1 : 0;
}

/** 依來源檔建出 Counts：沒有命中的檔不出現，鍵排序，輸出穩定。 */
export function buildCounts(sources: ReadonlyMap<string, string>): Counts {
	const out: Counts = {};
	for (const file of [...sources.keys()].sort(cmp)) {
		if (isExcluded(file)) continue;
		const lits = collectHanLiterals(file, sources.get(file) ?? "");
		if (lits.length === 0) continue;
		const counts: Record<string, number> = {};
		for (const l of lits.sort(cmp)) counts[l] = (counts[l] ?? 0) + 1;
		out[file] = counts;
	}
	return out;
}

export interface Delta {
	file: string;
	literal: string;
	current: number;
	baseline: number;
}

/** 超過 baseline 的（含新檔、新字面值）與少於 baseline 的。 */
export function compare(
	current: Counts,
	baseline: Counts,
): { over: Delta[]; under: Delta[] } {
	const over: Delta[] = [];
	const under: Delta[] = [];
	const files = new Set([...Object.keys(current), ...Object.keys(baseline)]);
	for (const file of [...files].sort(cmp)) {
		const c = current[file] ?? {};
		const b = baseline[file] ?? {};
		for (const literal of [
			...new Set([...Object.keys(c), ...Object.keys(b)]),
		].sort(cmp)) {
			const cur = c[literal] ?? 0;
			const base = b[literal] ?? 0;
			if (cur > base)
				over.push({ file, literal, current: cur, baseline: base });
			else if (cur < base)
				under.push({ file, literal, current: cur, baseline: base });
		}
	}
	return { over, under };
}

function globalTotals(c: Counts): Map<string, number> {
	const m = new Map<string, number>();
	for (const lits of Object.values(c))
		for (const [l, n] of Object.entries(lits)) m.set(l, (m.get(l) ?? 0) + n);
	return m;
}

/**
 * `--update` 的放行條件：
 * 1. 總筆數不增加，且每個字面值的全域總次數不增加；
 * 2. baseline 沒有的檔（新檔）只有在「某個 baseline 有、現在已不存在的舊檔，其字面值多重集合包含新檔的全部字面值」時才放行
 *    （改名或搬移）；每個舊檔只能被一個新檔對應。
 */
export function canUpdate(
	current: Counts,
	baseline: Counts,
): {
	ok: boolean;
	increases: { literal: string; from: number; to: number }[];
	unmatchedNewFiles: string[];
} {
	const cur = globalTotals(current);
	const base = globalTotals(baseline);
	const increases: { literal: string; from: number; to: number }[] = [];
	for (const [literal, to] of [...cur].sort((a, b) => cmp(a[0], b[0]))) {
		const from = base.get(literal) ?? 0;
		if (to > from) increases.push({ literal, from, to });
	}
	const sum = (m: Map<string, number>): number =>
		[...m.values()].reduce((a, b) => a + b, 0);
	const goneFiles = Object.keys(baseline)
		.filter((f) => !(f in current))
		.sort(cmp);
	const used = new Set<string>();
	const unmatchedNewFiles: string[] = [];
	for (const file of Object.keys(current)
		.filter((f) => !(f in baseline))
		.sort(cmp)) {
		const lits = Object.entries(current[file] ?? {});
		const from = goneFiles.find(
			(g) =>
				!used.has(g) && lits.every(([l, n]) => (baseline[g]?.[l] ?? 0) >= n),
		);
		if (from === undefined) unmatchedNewFiles.push(file);
		else used.add(from);
	}
	return {
		ok:
			increases.length === 0 &&
			sum(cur) <= sum(base) &&
			unmatchedNewFiles.length === 0,
		increases,
		unmatchedNewFiles,
	};
}

export function serializeBaseline(c: Counts): string {
	return `${JSON.stringify(c, null, "\t")}\n`;
}

export function countTotal(c: Counts): number {
	return Object.values(c).reduce(
		(a, lits) => a + Object.values(lits).reduce((x, y) => x + y, 0),
		0,
	);
}

function walk(root: string, rel: string, out: string[]): void {
	const full = join(root, rel);
	let entries: import("node:fs").Dirent[];
	try {
		entries = readdirSync(full, { withFileTypes: true });
	} catch {
		return;
	}
	for (const e of entries) {
		const r = `${rel}/${e.name}`;
		if (e.isDirectory()) {
			if (e.name === "node_modules" || e.name === ".next") continue;
			walk(root, r, out);
		} else if (SOURCE_EXT.test(e.name) && !isExcluded(r)) out.push(r);
	}
}

export function listSources(root: string): string[] {
	const out: string[] = [];
	for (const r of SCAN_ROOTS) {
		if (SOURCE_EXT.test(r)) {
			if (existsSync(join(root, r))) out.push(r);
		} else walk(root, r, out);
	}
	return out.sort(cmp);
}

export function scanRoot(root: string): Counts {
	const sources = new Map<string, string>();
	for (const f of listSources(root))
		sources.set(f, readFileSync(join(root, f), "utf8"));
	return buildCounts(sources);
}

const fmt = (d: Delta): string =>
	`  ${d.file}: ${JSON.stringify(d.literal)}（${d.baseline} → ${d.current}）`;

export interface RunResult {
	code: number;
	lines: string[];
	write?: string;
}

export type Mode = "check" | "update" | "init";

/** 純函式：不碰檔案系統，方便測試。baseline 為 null 表示 baseline 檔不存在。 */
export function run(
	current: Counts,
	baseline: Counts | null,
	mode: Mode,
): RunResult {
	if (mode === "init") {
		if (baseline !== null) {
			return {
				code: 1,
				lines: ["拒絕 --init：baseline 已存在；要收緊請用 --update。"],
			};
		}
		return {
			code: 0,
			lines: [
				`已建立 baseline：${Object.keys(current).length} 個檔、${countTotal(current)} 筆。`,
			],
			write: serializeBaseline(current),
		};
	}
	if (baseline === null) {
		return {
			code: 1,
			lines: [
				`找不到 baseline；第一次建立請執行 \`pnpm exec tsx scripts/check-hardcoded-han.ts --init\`${mode === "update" ? "（--update 不建立 baseline）" : ""}。`,
			],
		};
	}
	if (mode === "update") {
		const { ok, increases, unmatchedNewFiles } = canUpdate(current, baseline);
		if (!ok) {
			return {
				code: 1,
				lines: [
					"拒絕 --update：硬寫中文的總筆數、某個字面值的總次數增加了，或出現不是改名或搬移的新檔。",
					...increases.map(
						(i) => `  ${JSON.stringify(i.literal)}（${i.from} → ${i.to}）`,
					),
					...unmatchedNewFiles.map(
						(f) => `  新檔 ${f}：沒有字面值完全涵蓋它的已消失舊檔`,
					),
					"新文字放進 messages（next-intl），不要新增硬寫中文。",
				],
			};
		}
		return {
			code: 0,
			lines: [
				`已更新 baseline：${Object.keys(current).length} 個檔、${countTotal(current)} 筆。`,
			],
			write: serializeBaseline(current),
		};
	}
	const { over, under } = compare(current, baseline);
	if (over.length > 0) {
		return {
			code: 1,
			lines: [
				"新增了 baseline 沒有的硬寫中文（新文字放進 messages，用 next-intl 的 t(…)）：",
				...over.map(fmt),
			],
		};
	}
	if (under.length > 0) {
		return {
			code: 1,
			lines: [
				"硬寫中文減少了，baseline 要收緊（只會變少）。執行 `pnpm exec tsx scripts/check-hardcoded-han.ts --update` 並提交 baseline：",
				...under.map(fmt),
			],
		};
	}
	return {
		code: 0,
		lines: [
			`硬寫中文 ratchet 通過：${Object.keys(current).length} 個檔、${countTotal(current)} 筆。`,
		],
	};
}

const WEBSITE_ROOT = fileURLToPath(new URL("..", import.meta.url));
const BASELINE = join(WEBSITE_ROOT, "scripts/hardcoded-han-baseline.json");

export function main(argv: readonly string[]): number {
	const mode: Mode = argv.includes("--init")
		? "init"
		: argv.includes("--update")
			? "update"
			: "check";
	const current = scanRoot(WEBSITE_ROOT);
	const baseline: Counts | null = existsSync(BASELINE)
		? (JSON.parse(readFileSync(BASELINE, "utf8")) as Counts)
		: null;
	const r = run(current, baseline, mode);
	if (r.write !== undefined) writeFileSync(BASELINE, r.write);
	for (const l of r.lines) console[r.code === 0 ? "log" : "error"](l);
	return r.code;
}

if (
	process.argv[1] &&
	relative(process.argv[1], fileURLToPath(import.meta.url)) === ""
) {
	process.exit(main(process.argv.slice(2)));
}
