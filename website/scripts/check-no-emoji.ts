#!/usr/bin/env node

/**
 * No-emoji gate: UI text must not contain emoji — icons go through
 * components/shared/AppIcon.tsx (icon registry). Emoji render per-OS (macOS / Windows / Android all differ) and
 * clash with the brand icon language; the 2026-06 sweep removed them all, and
 * this gate keeps the class from regrowing (a 2026-07 plan-doc verbatim
 * implementation reintroduced 📣/🌐 because the rule only lived in one
 * contributor's session memory).
 *
 * Matches: the colour-emoji planes (U+1F000–1FAFF), the U+FE0F emoji
 * variation selector, and the dingbat/symbol block (U+2600–27BF) MINUS the
 * specific monochrome characters the sweep deliberately kept (★ ✓ ✕ …) —
 * many dingbats (✅ ⭐ ⚡ ❗ ❌) have default EMOJI presentation, so the block
 * cannot be excluded wholesale.
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const failures: string[] = [];

const COLOUR_EMOJI = /[\u{1F000}-\u{1FAFF}]|\u{FE0F}/u;
// Dingbats（2600–27BF）+ Misc Symbols and Arrows（2B00–2BFF，⭐⬛⬅…）+ ‼⁉。
// 刻意不含 25AA–25FE 幾何形（▶◀▪ 是正當的單色文字 UI 符號）與 2300–23FF
// （誤殺風險大於價值）；漏網個案靠 review，主力威脅（照抄 plan/AI 產出的
// ✅⭐❌❗ 類）已在網內。
const DINGBAT = /[☀-➿⬀-⯿‼⁉]/u;
// 2026-06 清掃時保留的單色符號（★☆ 評分、✓ 勾選、✕✗ 關閉）——文字預設
// 呈現、跨平台一致。其餘 dingbat（✅⭐⚡❗❌…）預設 emoji 呈現，一律擋。
const KEPT_DINGBATS = new Set(["★", "☆", "✓", "✕", "✗"]);

function findEmoji(text: string): string | null {
	const colour = text.match(COLOUR_EMOJI);
	if (colour) return colour[0];
	for (const match of text.matchAll(new RegExp(DINGBAT, "gu"))) {
		if (!KEPT_DINGBATS.has(match[0])) return match[0];
	}
	return null;
}

/**
 * Strip comment content before matching: block comments plus lines whose
 * trimmed text starts with `//`. Comments legitimately reference emoji when
 * documenting this very rule or describing stripped user input. Deliberately
 * narrow — no `*`-prefixed-line filter (that would silently skip template
 * literal / JSX lines that merely start with `*`; block-comment bodies are
 * already removed by the regex pass).
 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

function walk(dir: string, extensions: RegExp): string[] {
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
		// A broken symlink / file deleted mid-scan must degrade to "skipped",
		// not crash the whole lint chain.
		let isDirectory: boolean;
		try {
			isDirectory = statSync(join(root, rel)).isDirectory();
		} catch {
			continue;
		}
		if (isDirectory) out.push(...walk(rel, extensions));
		else if (extensions.test(rel)) out.push(rel);
	}
	return out;
}

const files = [
	...["lib", "features", "components", "app"].flatMap((d) =>
		walk(d, /\.(ts|tsx)$/),
	),
	// emoji 也可能經 CSS `content: "…"` 進 UI——styles 一併掃。
	...walk("styles", /\.css$/),
];
for (const path of files) {
	const text = stripComments(readFileSync(join(root, path), "utf8"));
	const found = findEmoji(text);
	if (found) {
		const index = text.indexOf(found);
		const line = text.slice(0, index).split("\n").length;
		failures.push(
			`${path}:~${line}: emoji "${found}" in UI code — use AppIcon (SVG) instead.`,
		);
	}
}

// Self-check: the gate must fire on colour emoji AND emoji-presentation
// dingbats, stay quiet on the kept monochrome set and comment-only mentions.
if (
	findEmoji("<span>📣</span>") === null ||
	findEmoji("label: '🌐 官網'") === null ||
	findEmoji("<span>✅ 完成</span>") === null ||
	findEmoji("value ? '⭐' : ''") === null ||
	findEmoji("<span>★★★</span>") !== null ||
	findEmoji('aria-label="✕"') !== null ||
	findEmoji("checked && '✓'") !== null ||
	findEmoji(stripComments("// 📣 emoji is documented here\nconst a = 1;")) !==
		null
) {
	console.error("check-no-emoji self-test failed — the gate regex is broken.");
	process.exit(1);
}

if (failures.length > 0) {
	console.error(`No-emoji gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}

console.log(`No-emoji gate passed (${files.length} files scanned).`);
