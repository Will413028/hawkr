#!/usr/bin/env node

/**
 * 硬寫中文的 ratchet（多語系 ADR D4 與 2026-10-22 修訂；計畫 I-CI）。
 *
 * 會發佈 en 的 website 程式裡，含漢字的字面值（字串、樣板字串、JSX 文字；註解不算）只能減少：
 * baseline（`hardcoded-han-baseline.json`）逐筆記「檔案 → 字面值內容 → 次數」，不記行號。
 * - 檢查：某檔某字面值的次數超過 baseline（含 baseline 沒有的新檔、新字面值）就失敗；
 *   次數少於 baseline 也失敗，要求執行 `--update` 收緊，所以 baseline 只會跟著變少（betterer 的做法）。
 * - `--update`：重寫 baseline，只在「總筆數不增加且每個字面值的全域總次數不增加」時允許（容許檔案改名或搬移）。
 * 用 TypeScript AST，不用 regex。
 *
 * 用法：tsx scripts/check-hardcoded-han.ts [--update]
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
	if (/\.(test|spec|unit\.test)\.[cm]?tsx?$/.test(name)) return true;
	if (/\.(gen|g)\.[cm]?tsx?$/.test(name)) return true;
	return false;
}

const HAN = /[㐀-䶿一-鿿豈-﫿]/;

/** 一個檔裡所有含漢字的字面值（保留重複）。樣板字串的各段合併成一筆，`${}` 以 `${}` 佔位。 */
export function collectHanLiterals(fileName: string, text: string): string[] {
	const sf = ts.createSourceFile(
		fileName,
		text,
		ts.ScriptTarget.Latest,
		true,
		fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
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

/** `--update` 的放行條件：總筆數不增加，且每個字面值的全域總次數不增加（搬移、改名放行）。 */
export function canUpdate(
	current: Counts,
	baseline: Counts,
): { ok: boolean; increases: { literal: string; from: number; to: number }[] } {
	const cur = globalTotals(current);
	const base = globalTotals(baseline);
	const increases: { literal: string; from: number; to: number }[] = [];
	for (const [literal, to] of [...cur].sort((a, b) => cmp(a[0], b[0]))) {
		const from = base.get(literal) ?? 0;
		if (to > from) increases.push({ literal, from, to });
	}
	const sum = (m: Map<string, number>): number =>
		[...m.values()].reduce((a, b) => a + b, 0);
	return { ok: increases.length === 0 && sum(cur) <= sum(base), increases };
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
		} else if (/\.tsx?$/.test(e.name) && !isExcluded(r)) out.push(r);
	}
}

export function listSources(root: string): string[] {
	const out: string[] = [];
	for (const r of SCAN_ROOTS) {
		if (r.endsWith(".ts")) {
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

/** 純函式：不碰檔案系統，方便測試。 */
export function run(
	current: Counts,
	baseline: Counts,
	update: boolean,
): RunResult {
	if (update) {
		const { ok, increases } = canUpdate(current, baseline);
		if (!ok) {
			return {
				code: 1,
				lines: [
					"拒絕 --update：硬寫中文的總筆數或某個字面值的總次數增加了。",
					...increases.map(
						(i) => `  ${JSON.stringify(i.literal)}（${i.from} → ${i.to}）`,
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
	const update = argv.includes("--update");
	const current = scanRoot(WEBSITE_ROOT);
	// baseline 檔不存在時 `--update` 是建立第一份（刪掉 baseline 會在 diff 裡看到）。
	const baseline: Counts = existsSync(BASELINE)
		? (JSON.parse(readFileSync(BASELINE, "utf8")) as Counts)
		: update
			? current
			: {};
	const r = run(current, baseline, update);
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
