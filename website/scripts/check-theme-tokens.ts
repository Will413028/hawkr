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

const MIXED_FAMILY = /^--text-/;
const TEXT_VAR_USE = /text-\[var\((--[a-zA-Z0-9_-]+)/g;

function ambiguousTextVars(line: string): string[] {
	const out: string[] = [];
	for (const m of line.matchAll(TEXT_VAR_USE)) {
		const name = m[1];
		if (MIXED_FAMILY.test(name) || varKind(name) === "length") out.push(name);
	}
	return out;
}

const ARBITRARY = /\[[^\]\n]*\]/g;
const VAR_REFERENCE = /var\(\s*(--[a-zA-Z0-9_-]+)\s*(,?)/g;
const INLINE_STYLE_DEF = /["'](--[a-zA-Z0-9_-]+)["']\s*:/g;
const ARBITRARY_PROP_DEF = /\[(--[a-zA-Z0-9_-]+)\s*:/g;

function locallyDefinedVars(source: string): Set<string> {
	const out = new Set<string>();
	for (const m of source.matchAll(INLINE_STYLE_DEF)) out.add(m[1]);
	for (const m of source.matchAll(ARBITRARY_PROP_DEF)) out.add(m[1]);
	return out;
}

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
		for (const name of ambiguousTextVars(line).filter(
			(n) => !missing.includes(n),
		)) {
			failures.push(
				`${path}:${i + 1}: text-[var(${name})] 無型別歧義寫法 — Tailwind 編成 color 規則，字級變數會沉默失效。字級請改 text-[length:var(${name})]，顏色請用 @theme token utility。`,
			);
		}
	});
}

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
const probe = (value: string) => `className="text-[${value}]"`;
const selfAmbiguous = ["var(--text-sm)", "var(--hkui-text-small)"].map(probe);
const selfTyped = [
	"length:var(--text-sm)",
	"color:var(--text-body)",
	"var(--ink-soft,#6b7a5a)",
	"var(--hkui-foreground)",
].map(probe);
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
