import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

/**
 * `<html lang>` follows the locale (ADR 2026-10-17 D1: the tag is `zh-TW` / `en`, not `zh-Hant`).
 * It is written in exactly one place — the document every root layout renders — and that place
 * takes it from the locale it is given, so a new root layout cannot forget it and a locale cannot
 * be hardcoded back.
 */
const read = (path: string) =>
	readFile(new URL(`../${path}`, import.meta.url), "utf8");

/** Source without comments, so prose that mentions `<html lang>` does not count as markup. */
const code = async (path: string) =>
	(await read(path))
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.replace(/^\s*\/\/.*$/gm, "");

test("the shared document sets <html lang> from its locale prop", async () => {
	const document = await read("components/shared/RootDocument.tsx");
	assert.match(document, /<html\s+lang=\{locale\}/);
	assert.doesNotMatch(document, /lang="zh-Hant"/);
});

test("every root layout renders the shared document with a locale", async () => {
	const rootLayouts = [
		"app/[locale]/layout.tsx",
		"app/(delegated)/layout.tsx",
		"app/global-not-found.tsx",
	];
	for (const file of rootLayouts) {
		const source = await code(file);
		assert.match(source, /<RootDocument locale=\{/, file);
		assert.doesNotMatch(
			source,
			/<html/,
			`${file} must not write its own <html>`,
		);
	}
});

test("the last-resort error boundary (no locale available) is zh-TW, not zh-Hant", async () => {
	const globalError = await read("app/global-error.tsx");
	assert.match(globalError, /<html lang="zh-TW">/);
});

test("there is no root app/layout.tsx: the document lives in the two root layouts", async () => {
	await assert.rejects(read("app/layout.tsx"), { code: "ENOENT" });
});
