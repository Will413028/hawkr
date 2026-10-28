import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { buildRootMetadata, SITE_NAME } from "../lib/seo/metadata";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

/**
 * Source-level guard for the one rule the root title template makes impossible
 * to see locally: `title.template = "%s | Hawkr"` is applied to every plain
 * string title a segment returns, so a page that spells the brand into its own
 * title string serves it twice. prod served `找不到商品 | Hawkr | Hawkr`
 * (#432) and, before that, `測試 | Hawkr | Hawkr` (#412) — two tickets,
 * one cause, because nothing enforced the rule that lib/seo/metadata.ts has
 * documented in a comment since the first fix.
 *
 * Reading the file a page lives in never reveals this: the second suffix is
 * added by the root layout. So this walks every Metadata `title` in app/ and
 * features/, renders it the way Next will, and asserts the brand survives
 * exactly once.
 *
 * Parsed with the TypeScript AST rather than grepped on purpose. The page title
 * and the og:title next to it are frequently byte-identical strings a few lines
 * apart, but only the page title flows through the template — og:title is a
 * plain value and MUST keep its brand. A line-based check cannot tell the two
 * apart; the AST can, because it knows which one is nested under `openGraph`.
 *
 * The last test pair guards a second way a page ends up with the wrong title:
 * a component rendering a JSX `<title>`, which React 19 hoists into <head> to
 * sit BESIDE the Metadata one rather than replace it. Same AST, different node
 * kind — and the same reason grep will not do, since the accessible `<title>`
 * inside an `<svg>` is legitimate and far more common.
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const SCAN_DIRS = ["app", "features"];
/** Widened for the JSX check: shared components render `<title>` too. */
const JSX_SCAN_DIRS = ["app", "features", "components"];

/** Stands in for `${...}` in a template literal — runtime data, not our text. */
const INTERPOLATION = "\u0000";

interface FoundTitle {
	file: string;
	line: number;
	/** `page` flows through the root template; `og` is emitted verbatim. */
	kind: "page" | "og";
	/** Literal text as authored, `${...}` collapsed to INTERPOLATION. */
	value: string;
	absolute: boolean;
}

/**
 * Every source file a route or component can live in. `.js`/`.jsx` are included
 * even though the tree currently holds none: the parser is handed TSX either
 * way, so covering them costs nothing, and a `.jsx` component rendering a
 * document `<title>` would otherwise slip past this guard in silence — the exact
 * failure mode the scan-reaches-its-targets assertions exist to prevent.
 */
const SOURCE_FILE = /\.[jt]sx?$/;

async function* walkFiles(dir: string): AsyncGenerator<string> {
	for (const entry of await readdir(dir, { withFileTypes: true })) {
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			yield* walkFiles(full);
		} else if (SOURCE_FILE.test(entry.name)) {
			yield full;
		}
	}
}

function propertyKey(name: ts.PropertyName): string | null {
	if (ts.isIdentifier(name) || ts.isStringLiteral(name)) return name.text;
	return null;
}

/**
 * True when this `title` property is the one Next feeds through the root
 * template: it sits under `export const metadata`, inside `generateMetadata`,
 * or in a `buildPageMetadata({...})` argument — and is NOT nested under
 * `openGraph`/`twitter`, whose titles are emitted verbatim.
 */
function titleKind(node: ts.Node): "page" | "og" | null {
	let social = false;
	let cur: ts.Node | undefined = node.parent;
	while (cur && !ts.isSourceFile(cur)) {
		if (ts.isPropertyAssignment(cur)) {
			const key = propertyKey(cur.name);
			if (key === "openGraph" || key === "twitter") social = true;
		}
		if (
			ts.isCallExpression(cur) &&
			ts.isIdentifier(cur.expression) &&
			cur.expression.text === "buildPageMetadata"
		) {
			return social ? "og" : "page";
		}
		if (
			ts.isVariableDeclaration(cur) &&
			ts.isIdentifier(cur.name) &&
			(cur.name.text === "metadata" || cur.name.text === "generateMetadata")
		) {
			return social ? "og" : "page";
		}
		if (
			ts.isFunctionDeclaration(cur) &&
			cur.name?.text === "generateMetadata"
		) {
			return social ? "og" : "page";
		}
		cur = cur.parent;
	}
	return null;
}

/** Literal text of a string/template expression, or null when not static. */
function staticText(expr: ts.Expression): string | null {
	if (ts.isStringLiteral(expr) || ts.isNoSubstitutionTemplateLiteral(expr)) {
		return expr.text;
	}
	if (ts.isTemplateExpression(expr)) {
		let out = expr.head.text;
		for (const span of expr.templateSpans)
			out += INTERPOLATION + span.literal.text;
		return out;
	}
	return null;
}

function readTitle(
	expr: ts.Expression,
): { value: string; absolute: boolean } | null {
	if (ts.isObjectLiteralExpression(expr)) {
		for (const prop of expr.properties) {
			if (!ts.isPropertyAssignment(prop)) continue;
			if (propertyKey(prop.name) !== "absolute") continue;
			const value = staticText(prop.initializer);
			return value === null ? null : { value, absolute: true };
		}
		return null;
	}
	const value = staticText(expr);
	return value === null ? null : { value, absolute: false };
}

async function collectTitles(): Promise<FoundTitle[]> {
	const found: FoundTitle[] = [];
	for (const dir of SCAN_DIRS) {
		for await (const file of walkFiles(path.join(ROOT, dir))) {
			const source = await readFile(file, "utf8");
			const sf = ts.createSourceFile(
				file,
				source,
				ts.ScriptTarget.Latest,
				true,
				ts.ScriptKind.TSX,
			);
			const visit = (node: ts.Node) => {
				if (
					ts.isPropertyAssignment(node) &&
					propertyKey(node.name) === "title"
				) {
					const kind = titleKind(node);
					const title = kind && readTitle(node.initializer);
					if (kind && title) {
						found.push({
							file: path.relative(ROOT, file),
							line:
								sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1,
							kind,
							...title,
						});
					}
				}
				ts.forEachChild(node, visit);
			};
			ts.forEachChild(sf, visit);
		}
	}
	return found;
}

/** JSX tag name as authored, e.g. `title` or `svg`. */
function jsxTagName(node: ts.JsxOpeningElement | ts.JsxSelfClosingElement) {
	return node.tagName.getText();
}

/**
 * True when this JSX element is nested inside an `<svg>`. SVG's own `<title>`
 * is the accessible name of a graphic and belongs where it is; only a `<title>`
 * in document markup competes with the Metadata title.
 */
function insideSvg(node: ts.Node): boolean {
	for (let cur = node.parent; cur; cur = cur.parent) {
		if (ts.isJsxElement(cur) && jsxTagName(cur.openingElement) === "svg") {
			return true;
		}
	}
	return false;
}

interface FoundJsxTitle {
	file: string;
	line: number;
	svg: boolean;
}

/**
 * Every `<title>` element authored in JSX, flagged by whether an `<svg>`
 * encloses it.
 */
async function collectJsxTitles(): Promise<FoundJsxTitle[]> {
	const found: FoundJsxTitle[] = [];
	for (const dir of JSX_SCAN_DIRS) {
		for await (const file of walkFiles(path.join(ROOT, dir))) {
			const source = await readFile(file, "utf8");
			const sf = ts.createSourceFile(
				file,
				source,
				ts.ScriptTarget.Latest,
				true,
				ts.ScriptKind.TSX,
			);
			const visit = (node: ts.Node) => {
				if (
					(ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
					jsxTagName(node) === "title"
				) {
					found.push({
						file: path.relative(ROOT, file),
						line: sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1,
						svg: insideSvg(node),
					});
				}
				ts.forEachChild(node, visit);
			};
			ts.forEachChild(sf, visit);
		}
	}
	return found;
}

function rootTemplate(): string {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const template = (buildRootMetadata("zh-TW").title as { template: string })
		.template;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	return template;
}

function occurrences(haystack: string, needle: string): number {
	return haystack.split(needle).length - 1;
}

const found = await collectTitles();
const titles = found.filter((t) => t.kind === "page");
const socialTitles = found.filter((t) => t.kind === "og");

test("no page title renders the brand twice", () => {
	const template = rootTemplate();
	const offenders = titles
		.map((t) => ({
			...t,
			rendered: t.absolute ? t.value : template.replace("%s", t.value),
		}))
		.filter((t) => occurrences(t.rendered, SITE_NAME) > 1);

	assert.deepEqual(
		offenders.map((t) => `${t.file}:${t.line} → ${t.rendered}`),
		[],
		`A page title spells the brand into its own string while the root template ` +
			`("${template}") appends it again. Drop the brand from the string and let ` +
			`the template add it — do not reach for { absolute }, which is reserved ` +
			`for titles that own the whole line (home, seller storefront).`,
	);
});

test("the scan reaches the titles it is meant to guard", () => {
	// A scanner that silently matches nothing would pass the assertion above.
	assert.ok(
		titles.length >= 60,
		`expected the metadata title scan to reach the ~90 page titles under ` +
			`${SCAN_DIRS.join("/, ")}/, found ${titles.length}`,
	);

	const inProductPage = titles.filter(
		(t) => t.file === "app/[locale]/(public)/products/[id]/page.tsx",
	);
	// Exactly the not-found title. The success title is `{ absolute:
	// productSeoTitle(...) }` — runtime data the scan cannot read, so it is
	// covered by tests/product-seo-title.test.ts instead. Seeing a second value
	// here would mean someone put a template literal back and re-entered the
	// root template (the double-suffix path this file guards).
	assert.deepEqual(
		inProductPage.map((t) => t.value),
		["找不到商品"],
	);
});

test("the home title still opts out of the template, and takes its locale's default title", async () => {
	// The title is the locale's own default title (`defaultTitleFor`), which the scan cannot read as
	// text — so assert on the source: it is `{ absolute }`, never a plain string.
	const home = await readFile(
		path.join(ROOT, "app/[locale]/(public)/page.tsx"),
		"utf8",
	);
	assert.match(
		home,
		/title:\s*\{\s*absolute:\s*defaultTitleFor\(locale\)\s*\}/,
	);
	assert.equal(
		titles.filter((t) => t.file === "app/[locale]/(public)/page.tsx").length,
		0,
		"the home title must not be spelled out as a literal the root template would suffix",
	);
});

test("the hand-written og:titles keep the brand the template never gives them, in their locale's spelling", () => {
	// og:title is not run through title.template, so these three carry the
	// brand themselves. They sit within a few lines of a page title that reads
	// identically, which is exactly how a whole-file search-and-replace strips
	// the brand from a share card while every page title still looks right.
	// The brand is the page locale's site name (`siteNameFor`), so in the scan it is an
	// interpolation after the separator, not literal text.
	// (The two product routes now share one composed string between page and
	// og:title — see productSeoTitle — so no literal of theirs is scanned here.)
	const branded = socialTitles
		.filter((t) => t.value.endsWith(` | ${INTERPOLATION}`))
		.map((t) => t.file)
		.sort();

	assert.deepEqual(branded, [
		"features/sellers/messages/SellerMessagesPage.tsx",
		"features/sellers/posts/SellerPostsPage.tsx",
		"features/sellers/reviews/SellerReviewsPage.tsx",
	]);
	// ...and none of them spells the zh-TW brand out, which would be wrong in English
	assert.deepEqual(
		socialTitles.filter((t) => t.value.includes(SITE_NAME)).map((t) => t.file),
		[],
	);
});

const jsxTitles = await collectJsxTitles();

test("no component renders a document <title> of its own", () => {
	// React 19 hoists a `<title>` rendered anywhere in the tree into <head>,
	// where it does NOT replace the one Metadata produced — it joins it, and the
	// document ships two. /faq and /contact each served two that way, and the
	// Metadata scan above cannot see it: these are JSX elements, not `title:`
	// properties. Titles belong to Metadata; a component that wants a different
	// one is asking for its route's buildPageMetadata to change.
	assert.deepEqual(
		jsxTitles.filter((t) => !t.svg).map((t) => `${t.file}:${t.line}`),
		[],
		`A component renders a <title> element. React hoists it into <head> ` +
			`alongside the Metadata title, so the page serves two. Delete it and set ` +
			`the title in the route's Metadata (buildPageMetadata) instead.`,
	);
});

test("the JSX scan reaches the <title> elements it is meant to guard", () => {
	// Same discriminating-power check as above: without this, a scan that
	// matched nothing at all (wrong dirs, tag name read the wrong way) would
	// sail through the assertion above forever.
	assert.ok(
		jsxTitles.some((t) => t.svg),
		`expected the JSX scan to find the accessible <title> elements inside ` +
			`<svg> under ${JSX_SCAN_DIRS.join("/, ")}/, found none — the scan is ` +
			`not reaching JSX at all`,
	);
});

test("Hawkr-derived feature names keep their brand prefix in the page title, opting out of the template", () => {
	const expected: Record<string, string> = {
		"app/[locale]/(public)/feed/page.tsx": "Hawkr 動態",
		"app/[locale]/(public)/market/products/page.tsx": "Hawkr 市集",
		"app/[locale]/(account)/account/messages/assistant/[conversationId]/page.tsx":
			"Hawkr Agent",
		"app/[locale]/(account)/account/messages/assistant/procurement/page.tsx":
			"Hawkr Agent",
		"app/[locale]/hawkr-agent-rfq-demo/page.tsx": "Hawkr Agent",
	};
	for (const [file, prefix] of Object.entries(expected)) {
		const own = titles.filter((t) => t.file === file);
		assert.equal(own.length, 1, `${file}: expected exactly one page title`);
		assert.equal(own[0].absolute, true, `${file}: title must be { absolute }`);
		assert.ok(
			own[0].value.startsWith(prefix),
			`${file}: title "${own[0].value}" must start with "${prefix}"`,
		);
	}
});
