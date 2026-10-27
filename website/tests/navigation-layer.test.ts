import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { findLiteralPathSinks } from "./helpers/literal-path-sinks";

const root = fileURLToPath(new URL("../", import.meta.url));

/**
 * The navigation layer (lib/intl/navigation.ts, ADR 2026-10-17 D1): application code writes locale-free paths
 * and the exits add the reader's language prefix once. A page that goes around an exit works in zh-TW and
 * silently drops an English reader into the zh-TW site, so the exits are fenced here, by source.
 */
const SOURCE_DIRS = ["app", "components", "features", "i18n", "lib", "stories"];

/** Files that may do what the rule forbids elsewhere, with the reason it is safe there. */
const ALLOWED = {
	nextLink: [
		"components/shared/view-transitions.tsx",
		/** `next/link` plus the prefix, for the links that keep its own prefetch and no View Transition. */
		"lib/intl/native-link.tsx",
	],
	useRouter: ["lib/intl/use-router.ts"],
	redirect: [] as string[],
	/** Analytics wants the real URL (prefix included) and never renders from it. */
	usePathname: ["lib/analytics/NavigationEvents.tsx"],
	documentNavigation: ["lib/navigation/document-location.ts"],
	handWrittenLocale: ["lib/intl/", "i18n/"],
};

function withoutComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.replace(/(^|[^:"'`\\])\/\/.*$/gm, "$1");
}

async function sourceFiles(dir: string): Promise<string[]> {
	const out: string[] = [];
	for (const entry of await readdir(path.join(root, dir), {
		withFileTypes: true,
	})) {
		const relative = `${dir}/${entry.name}`;
		if (entry.isDirectory()) {
			if (entry.name === "__tests__" || entry.name === "node_modules") continue;
			out.push(...(await sourceFiles(relative)));
		} else if (
			/\.(ts|tsx)$/.test(entry.name) &&
			!/\.(test|unit\.test|spec)\.(ts|tsx)$/.test(entry.name)
		) {
			out.push(relative);
		}
	}
	return out;
}

async function allSources(): Promise<Array<{ file: string; code: string }>> {
	const out: Array<{ file: string; code: string }> = [];
	for (const dir of SOURCE_DIRS) {
		for (const file of await sourceFiles(dir)) {
			out.push({
				file,
				code: withoutComments(await readFile(path.join(root, file), "utf8")),
			});
		}
	}
	return out;
}

const allowedIn = (list: readonly string[], file: string) =>
	list.some((entry) => file === entry || file.startsWith(entry));

/** Names imported (or re-exported) from `next/navigation`, as written, without `type` markers and aliases. */
function nextNavigationNames(code: string): Set<string> {
	const names = new Set<string>();
	const named =
		/(?:import|export)\s*(?:type\s*)?\{([^}]*)\}\s*from\s*["']next\/navigation["']/g;
	for (const match of code.matchAll(named)) {
		for (const part of match[1].split(",")) {
			const name = part
				.trim()
				.replace(/^type\s+/, "")
				.split(/\s+as\s+/)[0]
				.trim();
			if (name) names.add(name);
		}
	}
	const namespace =
		/import\s*\*\s*as\s+(\w+)\s+from\s*["']next\/navigation["']/.exec(code);
	if (namespace) {
		const used = new RegExp(`\\b${namespace[1]}\\.(\\w+)`, "g");
		for (const match of code.matchAll(used)) names.add(match[1]);
	}
	return names;
}

test("next/link is the view-transitions fork's business only: pages use components/shared/Link", async () => {
	const valueImport = /import\s+(?!type\b)[^;]*?\bfrom\s*["']next\/link["']/;
	const offenders = (await allSources())
		.filter(
			({ file, code }) =>
				!allowedIn(ALLOWED.nextLink, file) && valueImport.test(code),
		)
		.map(({ file }) => file);
	assert.deepEqual(
		offenders,
		[],
		'這些檔案從 "next/link" import 了 Link；改用 "@/components/shared/Link"（它加語系前綴、預取、導覽觀測）。',
	);
});

test("the router comes from lib/intl/use-router.ts, redirects from lib/intl/navigation.ts", async () => {
	const offenders: string[] = [];
	for (const { file, code } of await allSources()) {
		const names = nextNavigationNames(code);
		if (names.has("useRouter") && !allowedIn(ALLOWED.useRouter, file)) {
			offenders.push(`${file}: useRouter（改用 "@/lib/intl/use-router"）`);
		}
		for (const name of ["redirect", "permanentRedirect"]) {
			if (names.has(name) && !allowedIn(ALLOWED.redirect, file)) {
				offenders.push(
					`${file}: ${name}（改用 "@/lib/intl/navigation" 的版本，傳 { href, locale }；permanentRedirect 還沒匯出，要用就從 createNavigation 的結果多匯出一個）`,
				);
			}
		}
	}
	assert.deepEqual(offenders, []);
});

test("no component renders from Next's usePathname: the locale-free hook is the one to use", async () => {
	const offenders = (await allSources())
		.filter(
			({ file, code }) =>
				!allowedIn(ALLOWED.usePathname, file) &&
				nextNavigationNames(code).has("usePathname"),
		)
		.map(({ file }) => file);
	assert.deepEqual(
		offenders,
		[],
		'這些檔案從 "next/navigation" import usePathname；改用 "@/lib/intl/navigation"（它回不含語系前綴的路徑；' +
			"預設語系的頁面在 server 上渲染時 pathname 是改寫後的 /zh-TW/…，用 Next 的 hook 會和瀏覽器的 HTML 不一致）。",
	);
});

test("a hard navigation goes through commitDocumentNavigation, which adds the language prefix", async () => {
	const write =
		/\b(?:window\.|document\.)?location\.(?:assign|replace)\s*\(|\blocation\.href\s*=(?!=)/;
	const offenders = (await allSources())
		.filter(
			({ file, code }) =>
				!allowedIn(ALLOWED.documentNavigation, file) && write.test(code),
		)
		.map(({ file }) => file);
	assert.deepEqual(
		offenders,
		[],
		'這些檔案直接寫 window.location；改用 commitDocumentNavigation（"@/lib/navigation/document-location"）。',
	);
});

test("no plain anchor, form action, window.open or history write holds a literal root-relative page path", async () => {
	const offenders: string[] = [];
	for (const { file, code } of await allSources()) {
		// cheap filter before parsing the whole tree
		if (
			!/<a\b|<form\b|\bopen\s*\(|history\s*\.\s*(?:push|replace)State/.test(
				code,
			)
		) {
			continue;
		}
		for (const sink of findLiteralPathSinks(file, code)) {
			offenders.push(`${file}:${sink.line} ${sink.kind} → ${sink.path}`);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		"站內頁面路徑不要以字面值直接交給 <a>、<form action>、window.open、history：" +
			"連結改用 components/shared/Link（或 lib/intl/native-link），form action 與整頁導覽的 <a> 用 useLocalizedHref，" +
			"window.open／history 用 localizeHref（lib/intl/navigation）；否則英文讀者會落到 zh-TW 的頁面。",
	);
});

test("the literal-path scan catches every spelling it is meant to catch, and only page paths", () => {
	// `@{` stands for a template placeholder (a literal one in a plain string trips a lint rule)
	const scan = (code: string) =>
		findLiteralPathSinks("probe.tsx", code.replaceAll("@{", "${"));
	const caught = [
		'const x = <a href="/account">x</a>;',
		"const x = <a href='/account'>x</a>;",
		'const x = <a href={"/account"}>x</a>;',
		"const x = <a href={`/account`}>x</a>;",
		"const x = <a href={`/account/@{id}`}>x</a>;",
		"const x = <a href={`/@{slug}`}>x</a>;",
		'const x = <a href={ok ? "/account" : "/cart"}>x</a>;',
		'const x = <a href={href || "/account"}>x</a>;',
		'const x = <a href={"/account/" + id}>x</a>;',
		'const x = <form action="/search" />;',
		'const x = <form action={"/search"} />;',
		"const x = <form action={`/search`} />;",
		'window.open("/account");',
		'open("/account");',
		'window.history.pushState(null, "", "/account?tab=1");',
		'history.replaceState(null, "", `/account/@{id}`);',
	];
	for (const code of caught) {
		assert.equal(scan(code).length > 0, true, `should catch: ${code}`);
	}
	const allowed = [
		'const x = <a href="https://example.com/account">x</a>;',
		'const x = <a href="mailto:a@b.c">x</a>;',
		'const x = <a href="#top">x</a>;',
		'const x = <a href="//cdn.example/a">x</a>;',
		'const x = <a href="/api/v1/orders/export.csv" download>x</a>;',
		'const x = <a href="/templates/hawkr_import_template.csv">x</a>;',
		'const x = <a href="/placeholders/a.svg">x</a>;',
		"const x = <a href={href}>x</a>;",
		"const x = <a href={`@{origin}/account`}>x</a>;",
		"const x = <a href={localized}>x</a>;",
		'const x = <Link href="/account">x</Link>;',
		"const x = <form action={action} />;",
		'window.open(url, "_blank");',
		'window.history.pushState(null, "", url);',
	];
	for (const code of allowed) {
		assert.deepEqual(scan(code), [], `should allow: ${code}`);
	}
	assert.deepEqual(scan('\n\nconst x = <a href="/account">x</a>;')[0]?.line, 3);
});

test("no source writes a language prefix by hand", async () => {
	const handWritten = /["'`]\/(?:en|zh-TW)(?:\/|["'`?#])/;
	const offenders = (await allSources())
		.filter(
			({ file, code }) =>
				!allowedIn(ALLOWED.handWrittenLocale, file) && handWritten.test(code),
		)
		.map(({ file }) => file);
	assert.deepEqual(
		offenders,
		[],
		"這些檔案手寫了語系前綴；站內路徑一律不含語系，出口（Link、router、redirect、commitDocumentNavigation）加。",
	);
});
