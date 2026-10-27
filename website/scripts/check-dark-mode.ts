#!/usr/bin/env node

/**
 * Dark-mode token-ladder guard for visitor-facing surfaces (home / about /
 * seller landing). Enforces the rules in
 * docs/three-page-split-plan.md §"禁止清單":
 *
 *   1. No arbitrary `var(--hkui-…)` / `var(--theme-…)` in className —
 *      those bypass `@theme inline` and won't dark-cascade.
 *   2. No `dark:(bg|text)-(white|black|zinc|slate|gray|neutral)-…` —
 *      that re-states a value the token cascade already provides, and
 *      drifts when the cascade is tuned.
 *
 * Exit code 1 on any violation so this can be wired into pre-commit
 * (Husky etc.) without rewriting it later.
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const SCAN_ROOTS = [
	"features/home",
	"features/about",
	"features/seller/landing",
];

const RULES: { name: string; pattern: RegExp }[] = [
	{
		name: "arbitrary --hkui- / --theme- var() in className",
		pattern:
			/(?:bg|text|border|fill|stroke|ring|shadow)-\[[^\]]*var\(--(?:hkui|theme)-[^\]]+\]/,
	},
	{
		name: "dark:* recolor of a plain colour scale (use cascade instead)",
		pattern:
			/dark:(?:bg|text|border|ring)-(?:white|black|zinc|slate|gray|neutral)-\d/,
	},
];

interface Violation {
	file: string;
	line: number;
	rule: string;
	snippet: string;
}

const violations: Violation[] = [];

function walk(dir: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(root, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (entry.startsWith(".") || entry === "node_modules") continue;
		const rel = join(dir, entry);
		const stat = statSync(join(root, rel));
		if (stat.isDirectory()) {
			out.push(...walk(rel));
		} else if (/\.(tsx?|jsx?)$/.test(entry)) {
			out.push(rel);
		}
	}
	return out;
}

const files = SCAN_ROOTS.flatMap((r) => walk(r));

for (const file of files) {
	const text = readFileSync(join(root, file), "utf8");
	const lines = text.split("\n");
	lines.forEach((line, idx) => {
		for (const rule of RULES) {
			if (rule.pattern.test(line)) {
				violations.push({
					file: relative(root, join(root, file)),
					line: idx + 1,
					rule: rule.name,
					snippet: line.trim().slice(0, 140),
				});
			}
		}
	});
}

if (violations.length === 0) {
	console.log(
		`✓ dark-mode check: ${files.length} files scanned, no violations.`,
	);
	process.exit(0);
}

console.error(`✗ dark-mode check: ${violations.length} violation(s)\n`);
for (const v of violations) {
	console.error(`  ${v.file}:${v.line}`);
	console.error(`    rule: ${v.rule}`);
	console.error(`    > ${v.snippet}\n`);
}
console.error(
	"See docs/three-page-split-plan.md §「禁止清單」 for the rationale.",
);
process.exit(1);
