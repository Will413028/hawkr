#!/usr/bin/env node

/**
 * Theme-token gate: hk-/hkui- Tailwind utilities must reference tokens that
 * actually exist in the styles/globals.css @theme blocks. Tailwind v4 emits
 * NO CSS for an unknown token utility — the class silently does nothing
 * (`bg-hk-brand` shipped a transparent reload button; `rounded-hkui-xl`
 * shipped square corners), and nothing at build or lint time says so.
 *
 * Companion rule enforced here: `text-[ var(--…) ]` without a type hint is
 * ambiguous — Tailwind resolves it as COLOR, so a font-size var written that
 * way emits `color: 0.9375rem` (invalid → the element's colour silently
 * falls back to inherited). Font-size vars must use
 * `text-[ length:var(--…) ]`.
 *
 * That companion rule is decided per-variable by RESOLVED CSS VALUE, not by
 * name: every custom property under styles/ is parsed (following `var()`
 * chains and fallbacks), and only the ones that resolve to a LENGTH are
 * rejected: `--hkui-foreground` is a colour and its bare form stays legal,
 * while `--hkui-text-small` is 0.875rem and its bare form is a silent bug.
 * Matching on a name prefix cannot do this — `--hkui-text-*` reads like the
 * colour family `--hk-text-*` but is the font-size scale, and that blind spot
 * is exactly how 46 dead font-size utilities shipped.
 *
 * (Variables are named bare above, never spelled as a full utility: Tailwind
 * scans scripts/ too, so a complete class written anywhere in this file mints
 * real CSS for a class no component uses.)
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const failures: string[] = [];

const theme = readFileSync(join(root, "styles/globals.css"), "utf8");
const definedTokens = new Set(
	[...theme.matchAll(/--([a-z]+(?:-[a-z0-9]+)+)\s*:/g)].map((m) => m[1]),
);

const COLOR_UTILS =
	"bg|text|border|ring|fill|stroke|from|via|to|outline|decoration|divide|accent|caret|placeholder";
const UTILITY = new RegExp(
	`\\b(${COLOR_UTILS}|shadow|rounded)-((?:hk|hkui)-[a-z0-9]+(?:-[a-z0-9]+)*)`,
	"g",
);
// --- 歧義 text-[ var(--…) ]：靠「變數解析後的實際值」判定，不靠名字前綴 ---

// styles/ 底下所有自訂屬性的第一份宣告（同名重宣告只換值不換型別，例如深色
// 模式覆寫顏色，因此取第一份即可判型）。
//
// 必須以「宣告起點」錨定並讓值吃到下一個分號為止，不能逐行比對：本 repo 有 33
// 處宣告是跨行的（ui-kit.css 的 --hkui-focus-ring、01-tokens-theme.css 的
// --font-primary、home.css 的 --font-body …），逐行版本會完全看不到它們，於是
// 把「定義好好的變數」判成未定義。
const CSS_DECLARATION = /(?:^|[;{])\s*(--[a-zA-Z0-9_-]+)\s*:\s*([^;]*)/gm;
const cssVarValue = new Map<string, string>();
for (const file of walk("styles", /\.css$/)) {
	const css = readFileSync(join(root, file), "utf8").replace(
		/\/\*[\s\S]*?\*\//g,
		" ",
	);
	for (const m of css.matchAll(CSS_DECLARATION)) {
		if (!cssVarValue.has(m[1]))
			cssVarValue.set(m[1], m[2].trim().replace(/\s+/g, " "));
	}
}

const LENGTH_VALUE =
	/^-?(?:\d+\.?\d*|\.\d+)(?:rem|px|em|ch|vw|vh|vmin|vmax|pt|pc|cm|mm|in|ex)\b/;
const COLOR_VALUE =
	/^(?:#[0-9a-fA-F]{3,8}\b|(?:rgba?|hsla?|oklch|oklab|lab|lch|color|color-mix)\(|transparent\b|currentColor\b)/;

type VarKind = "length" | "color" | "unknown";

function valueKind(raw: string, seen: Set<string>): VarKind {
	const value = raw.trim();
	if (LENGTH_VALUE.test(value)) return "length";
	if (COLOR_VALUE.test(value)) return "color";
	if (/^(?:calc|clamp|min|max)\(/.test(value)) {
		return /\d(?:rem|px|em|ch|vw|vh)/.test(value) ? "length" : "unknown";
	}
	const ref = value.match(/^var\(\s*(--[a-zA-Z0-9_-]+)\s*(?:,([\s\S]*))?\)$/);
	if (ref) {
		const viaRef = varKind(ref[1], seen);
		if (viaRef !== "unknown") return viaRef;
		if (ref[2]) return valueKind(ref[2], seen);
	}
	return "unknown";
}

function varKind(name: string, seen = new Set<string>()): VarKind {
	if (seen.has(name)) return "unknown";
	seen.add(name);
	const raw = cssVarValue.get(name);
	return raw === undefined ? "unknown" : valueKind(raw, seen);
}

// --text-* 家族是 Tailwind 內建字級刻度（不在 styles/ 內、解析不到），且本 repo
// 的同名家族「顏色與字級混居」（--text-body 是色、--text-sm 是級）——整族一律
// 要求寫明型別，不走值解析。
const MIXED_FAMILY = /^--text-/;
const TEXT_VAR_USE = /text-\[var\((--[a-zA-Z0-9_-]+)/g;

// 回傳這一行裡「歧義且會壞掉」的變數名；顏色變數的歧義形式行為確定（=color），
// 不在此攔截。
function ambiguousTextVars(line: string): string[] {
	const out: string[] = [];
	for (const m of line.matchAll(TEXT_VAR_USE)) {
		const name = m[1];
		if (MIXED_FAMILY.test(name) || varKind(name) === "length") out.push(name);
	}
	return out;
}

// --- 未定義的 custom property ---
//
// 「解析不到定義」和「解析到了但判不出型別」是兩件事：後者維持放行（varKind →
// unknown），前者可以確定是錯的——`var(--x)` 指向不存在的屬性時整條宣告無效被
// 丟掉，class 一樣是沉默失效，只是換了成因。`--hkui-text-xsmall` 就是這樣：寫法
// 完全正確（有 length:）卻從沒被定義過，字級照樣沒有套上。
//
// 有 fallback 的（`var(--x, 14px)`）不算壞——fallback 就是為此存在。
const ARBITRARY = /\[[^\]\n]*\]/g;
const VAR_REFERENCE = /var\(\s*(--[a-zA-Z0-9_-]+)\s*(,?)/g;
// 同檔的行內定義：style={{ "--x": … }} 與 Tailwind 的 arbitrary property [--x:…]。
// 兩者都是合法定義處，且都只在該檔生效，所以逐檔收集、不做全域池（全域池會讓
// A 檔的定義替 B 檔的錯字背書）。
const INLINE_STYLE_DEF = /["'](--[a-zA-Z0-9_-]+)["']\s*:/g;
const ARBITRARY_PROP_DEF = /\[(--[a-zA-Z0-9_-]+)\s*:/g;

function locallyDefinedVars(source: string): Set<string> {
	const out = new Set<string>();
	for (const m of source.matchAll(INLINE_STYLE_DEF)) out.add(m[1]);
	for (const m of source.matchAll(ARBITRARY_PROP_DEF)) out.add(m[1]);
	return out;
}

// 回傳這一行裡「引用了但哪裡都找不到定義、又沒給 fallback」的變數名。
function undefinedVars(line: string, localVars: Set<string>): string[] {
	const out: string[] = [];
	for (const bracket of line.matchAll(ARBITRARY)) {
		for (const ref of bracket[0].matchAll(VAR_REFERENCE)) {
			const [, name, comma] = ref;
			if (comma === ",") continue;
			if (cssVarValue.has(name) || localVars.has(name)) continue;
			out.push(name);
		}
	}
	return out;
}

function requiredTokens(util: string, token: string): string[] {
	if (util === "shadow") return [`shadow-${token}`, `color-${token}`];
	if (util === "rounded") return [`radius-${token}`];
	if (util === "text") return [`color-${token}`, `text-${token}`];
	return [`color-${token}`];
}

// 註解要「留白」而不是「刪掉」：刪行會讓後面每一筆 failure 的行號位移，開發者
// 依報告跳過去會看到不相干的程式碼。
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
		.split("\n")
		.map((line) => (line.trimStart().startsWith("//") ? "" : line))
		.join("\n");
}

function walk(dir: string, match: RegExp): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(root, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (
			entry === "node_modules" ||
			entry === ".next" ||
			entry === ".git" ||
			entry === ".worktrees"
		) {
			continue;
		}
		const rel = relative(root, join(root, dir, entry));
		let isDirectory: boolean;
		try {
			isDirectory = statSync(join(root, rel)).isDirectory();
		} catch {
			continue;
		}
		if (isDirectory) out.push(...walk(rel, match));
		else if (match.test(rel)) out.push(rel);
	}
	return out;
}

const files = ["lib", "features", "components", "app"].flatMap((d) =>
	walk(d, /\.tsx?$/),
);
for (const path of files) {
	const text = stripComments(readFileSync(join(root, path), "utf8"));
	const localVars = locallyDefinedVars(text);
	const lines = text.split("\n");
	lines.forEach((line, i) => {
		for (const m of line.matchAll(UTILITY)) {
			const [, util, token] = m;
			if (!requiredTokens(util, token).some((t) => definedTokens.has(t))) {
				failures.push(
					`${path}:${i + 1}: ${m[0]} — token 不存在於 @theme（Tailwind 不會產生任何 CSS，class 沉默失效）。查 styles/globals.css 的 hk/hkui 對照表選存在的變體。`,
				);
			}
		}
		const missing = undefinedVars(line, localVars);
		for (const name of missing) {
			failures.push(
				`${path}:${i + 1}: var(${name}) 未定義 — styles/ 與本檔都找不到這個 custom property，整條宣告會被丟掉（class 沉默失效）。請改用既有 token，或替它補上定義。`,
			);
		}
		// 未定義是更根本的問題，同一個變數不必再抱怨一次型別歧義。
		for (const name of ambiguousTextVars(line).filter(
			(n) => !missing.includes(n),
		)) {
			failures.push(
				`${path}:${i + 1}: text-[var(${name})] 無型別歧義寫法 — Tailwind 編成 color 規則，字級變數會沉默失效。字級請改 text-[length:var(${name})]，顏色請用 @theme token utility。`,
			);
		}
	});
}

// Self-check: known-dead forms must fire; known-good forms must not.
const selfDead: Array<[string, string]> = [
	["bg", "hk-surface"],
	["rounded", "hkui-xl"],
	["text", "hkui-warning"],
];
const selfAlive: Array<[string, string]> = [
	["bg", "hk-surface-card"],
	["border", "hk-border-soft"],
	["text", "hk-primary"],
];
// 歧義規則的自檢：必攔（混居家族＋解析為長度）／必放（已寫型別＋解析為顏色）。
// 後兩組同時是「CSS 值解析真的有在跑」的證據——styles/ 讀不到或 var() 鏈斷掉
// 時 varKind 會退成 unknown，字級案例就攔不住，這裡直接判 gate 壞掉。
//
// 這些樣本要「拼」出來、不能整串寫死：Tailwind 的來源掃描包含 scripts/，看到
// 完整 class 字面值就會當成真實用法產生 CSS。實測（2026-07-31）產物裡確實躺著
// 一條 --text-sm 歧義形式的規則（編出來是 color:var(--text-sm)），全 repo 沒有
// 任何元件在用，來源就是本檔舊版的自檢字串。拼接後掃描器抽不到候選字，樣本也
// 就不再污染 bundle。
const probe = (value: string) => `className="text-[${value}]"`;
const selfAmbiguous = ["var(--text-sm)", "var(--hkui-text-small)"].map(probe);
const selfTyped = [
	"length:var(--text-sm)",
	"color:var(--text-body)",
	"var(--ink-soft,#6b7a5a)",
	"var(--hkui-foreground)",
].map(probe);
// 未定義規則的自檢。第一組（既有 token 不可誤報）同時證明 styles/ 真的讀進來
// 了：cssVarValue 空掉的話它會整批誤報，gate 立刻判自己壞掉而不是靜靜放行。
const NO_SUCH_TOKEN = "--definitely-not-a-real-token";
const selfDefined = ["var(--hkui-danger-50)", "var(--hkui-text-tiny)"].map(
	probe,
);
const selfUndefined = [probe(`var(${NO_SUCH_TOKEN})`)];
const selfFallback = [probe(`var(${NO_SUCH_TOKEN},#fff)`)];
const noLocals = new Set<string>();
if (
	selfDead.some(([u, t]) =>
		requiredTokens(u, t).some((x) => definedTokens.has(x)),
	) ||
	selfAlive.some(
		([u, t]) => !requiredTokens(u, t).some((x) => definedTokens.has(x)),
	) ||
	selfAmbiguous.some((line) => ambiguousTextVars(line).length === 0) ||
	selfTyped.some((line) => ambiguousTextVars(line).length > 0) ||
	selfDefined.some((line) => undefinedVars(line, noLocals).length > 0) ||
	selfUndefined.some((line) => undefinedVars(line, noLocals).length === 0) ||
	selfFallback.some((line) => undefinedVars(line, noLocals).length > 0) ||
	undefinedVars(probe(`var(${NO_SUCH_TOKEN})`), new Set([NO_SUCH_TOKEN]))
		.length > 0 ||
	varKind("--hkui-text-small") !== "length" ||
	varKind("--hkui-foreground") !== "color"
) {
	console.error("check-theme-tokens self-test failed — the gate is broken.");
	process.exit(1);
}

if (failures.length > 0) {
	console.error(`Theme-token gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}

console.log(`Theme-token gate passed (${files.length} files scanned).`);
