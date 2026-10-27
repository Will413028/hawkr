import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	buildPageMetadata,
	buildRootMetadata,
	SOCIAL_IMAGE_PATH,
	socialImages,
} from "../lib/seo/metadata";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

/**
 * Source-level guard for the rule that decides whether a page has a share
 * picture at all: a page that declares `openGraph` must name its `images`, and
 * must name them through `socialImages()`.
 *
 * Next merges metadata segment by segment. A segment that declares `openGraph`
 * REPLACES the parent's resolved one wholesale — it does not merge into it —
 * and the `app/opengraph-image.tsx` file convention is folded in only at the
 * segment whose own directory holds the file, i.e. `app/`. The root LAYOUTS
 * (`app/[locale]/layout.tsx`, `app/(delegated)/layout.tsx`) are one segment
 * below it, so they declare `openGraph` too and have to name the card
 * themselves (`buildRootMetadata`): a root layout that does not leaves every
 * page that declares no `openGraph` of its own — the account, checkout, search,
 * login, seller-center and vendor pages and the route-level 404s — without
 * og:image, its four `og:image:*` description tags and the twitter mirror of them.
 * And every page that declares an `openGraph` (which is every page wanting a
 * page-specific og:url) drops the picture on the way past unless it names it.
 * Writing `images: []`, or the `x ? [{url:x}] : []` shape that collapses to it,
 * is the same thing: the key exists, so the parent's images are gone and
 * nothing takes their place.
 *
 * Measured on a built server before the fix: 11 of 96 metadata-bearing routes
 * served no og:image and no twitter:image — `/`, `/about`, `/contact`, `/faq`,
 * `/feed`, `/market/products`, `/seller`, `/seller-guide`, the seller messages
 * tab, and the two product templates whenever the product had no photo. The
 * home page, the most-shared URL on the site, was one of them.
 *
 * Nothing about that is visible from the page: the tab title, the description
 * and every other tag render perfectly. It only shows up in someone else's chat
 * window, which is why this is a test and not a review checklist item.
 *
 * Parsed with the TypeScript AST rather than grepped, for the same reason as
 * tests/seo-title-brand-suffix.test.ts: `images` is a common property name and
 * only the one nested directly under `openGraph` is the one Next reads.
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const SCAN_DIRS = ["app", "features"];
const SOURCE_FILE = /\.[jt]sx?$/;
/** Test files talk ABOUT metadata; they do not declare any. */
const TEST_FILE = /\.(test|spec|unit\.test)\.[jt]sx?$/;

interface FoundOpenGraph {
	file: string;
	line: number;
	/** The `socialImages(...)` callee name, or a description of what was found. */
	images: string;
}

async function* walkFiles(dir: string): AsyncGenerator<string> {
	for (const entry of await readdir(dir, { withFileTypes: true })) {
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			if (entry.name === "__tests__") continue;
			yield* walkFiles(full);
		} else if (SOURCE_FILE.test(entry.name) && !TEST_FILE.test(entry.name)) {
			yield full;
		}
	}
}

function propertyKey(name: ts.PropertyName): string | null {
	if (ts.isIdentifier(name) || ts.isStringLiteral(name)) return name.text;
	return null;
}

/**
 * How this `openGraph` block spells its images, as a short label:
 *   "socialImages()"  — the only accepted answer
 *   "<missing>"       — no `images` property; the site card is silently lost
 *   anything else     — an array/conditional written by hand, which can be empty
 */
function describeImages(og: ts.ObjectLiteralExpression): string {
	for (const prop of og.properties) {
		if (ts.isPropertyAssignment(prop) && propertyKey(prop.name) === "images") {
			const init = prop.initializer;
			if (
				ts.isCallExpression(init) &&
				ts.isIdentifier(init.expression) &&
				init.expression.text === "socialImages"
			) {
				return "socialImages()";
			}
			return init.getText().replace(/\s+/g, " ").slice(0, 60);
		}
		// A spread hides whether `images` is present, so it cannot be accepted.
		if (ts.isSpreadAssignment(prop)) return `...${prop.expression.getText()}`;
	}
	return "<missing>";
}

async function collectOpenGraphBlocks(): Promise<FoundOpenGraph[]> {
	const found: FoundOpenGraph[] = [];
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
					propertyKey(node.name) === "openGraph" &&
					ts.isObjectLiteralExpression(node.initializer)
				) {
					found.push({
						file: path.relative(ROOT, file),
						line: sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1,
						images: describeImages(node.initializer),
					});
				}
				ts.forEachChild(node, visit);
			};
			ts.forEachChild(sf, visit);
		}
	}
	return found.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line);
}

const blocks = await collectOpenGraphBlocks();

test("every page-declared openGraph names its images via socialImages()", () => {
	const offenders = blocks.filter((b) => b.images !== "socialImages()");
	assert.deepEqual(
		offenders.map((b) => `${b.file}:${b.line} → images: ${b.images}`),
		[],
		`An openGraph block that does not route its images through socialImages() ` +
			`ships a page with no share picture the moment its own image is absent ` +
			`— and declaring the block is what threw away the site card. Write ` +
			`images: socialImages(maybeUrl); the helper falls back for you.`,
	);
});

test("the scan reaches the openGraph blocks it is meant to guard", () => {
	// A scanner that silently matched nothing would pass the assertion above.
	// Six blocks live under app/ and features/ today (the two product templates,
	// the storefront index, and the three storefront tabs); every other page
	// goes through buildPageMetadata, which is covered below.
	assert.ok(
		blocks.length >= 6,
		`expected the openGraph scan to reach the hand-written blocks under ` +
			`${SCAN_DIRS.join("/, ")}/, found ${blocks.length}`,
	);
});

test("socialImages falls back to the site card, never to an empty list", () => {
	assert.deepEqual(socialImages(), [{ url: SOCIAL_IMAGE_PATH }]);
	assert.deepEqual(socialImages(null), [{ url: SOCIAL_IMAGE_PATH }]);
	assert.deepEqual(socialImages(undefined), [{ url: SOCIAL_IMAGE_PATH }]);
	// buildImageUrl() returns "" for a record with no picture — the exact input
	// the old `x ? [{url:x}] : []` shape turned into an empty array.
	assert.deepEqual(socialImages(""), [{ url: SOCIAL_IMAGE_PATH }]);
	assert.deepEqual(socialImages("   "), [{ url: SOCIAL_IMAGE_PATH }]);
	assert.deepEqual(socialImages("https://img.example/a.jpg"), [
		{ url: "https://img.example/a.jpg" },
	]);
});

test("buildPageMetadata gives a page a share image without being asked", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const og = buildPageMetadata({
		title: "常見問題",
		description: "d",
		path: "/faq",
		locale: "zh-TW",
	}).openGraph as { images: unknown };
	assert.deepEqual(og.images, [{ url: SOCIAL_IMAGE_PATH }]);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

/**
 * What the share-card route publishes about itself: the `alt`, `size` and
 * `contentType` exports of app/opengraph-image.tsx, read from the source so the
 * root layouts cannot drift away from the picture they describe.
 */
function opengraphImageRoute(): {
	alt: string;
	width: number;
	height: number;
	contentType: string;
} {
	const file = path.join(ROOT, "app/opengraph-image.tsx");
	const sf = ts.createSourceFile(
		file,
		readFileSync(file, "utf8"),
		ts.ScriptTarget.Latest,
		true,
		ts.ScriptKind.TSX,
	);
	const exportsByName = new Map<string, ts.Expression>();
	for (const statement of sf.statements) {
		if (!ts.isVariableStatement(statement)) continue;
		for (const decl of statement.declarationList.declarations) {
			if (ts.isIdentifier(decl.name) && decl.initializer) {
				exportsByName.set(decl.name.text, decl.initializer);
			}
		}
	}
	const literal = (name: string) => {
		const init = exportsByName.get(name);
		assert.ok(init, `app/opengraph-image.tsx no longer exports ${name}`);
		return init;
	};
	const size = literal("size");
	assert.ok(ts.isObjectLiteralExpression(size));
	const dimension = (key: string) => {
		const prop = size.properties.find(
			(p): p is ts.PropertyAssignment =>
				ts.isPropertyAssignment(p) && propertyKey(p.name) === key,
		);
		assert.ok(prop && ts.isNumericLiteral(prop.initializer), `size.${key}`);
		return Number(prop.initializer.text);
	};
	const text = (name: string) => {
		const init = literal(name);
		assert.ok(ts.isStringLiteralLike(init), name);
		return init.text;
	};
	return {
		alt: text("alt"),
		width: dimension("width"),
		height: dimension("height"),
		contentType: text("contentType"),
	};
}

test("the root names the site card itself: the file convention sits one segment above its layouts", () => {
	// buildRootMetadata is called by app/[locale]/layout.tsx and
	// app/(delegated)/layout.tsx, below the directory that holds
	// app/opengraph-image.tsx. Their `openGraph` replaces the parent's resolved
	// one, so without `images` here the pages that inherit it ship no picture.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const route = opengraphImageRoute();
	const og = buildRootMetadata("zh-TW").openGraph as { images?: unknown };
	assert.deepEqual(og.images, [
		{
			url: SOCIAL_IMAGE_PATH,
			width: route.width,
			height: route.height,
			type: route.contentType,
			alt: route.alt,
		},
	]);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("the root's picture is described in the page's language", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const alt = (locale: "zh-TW" | "en") =>
		(
			buildRootMetadata(locale).openGraph as {
				images: Array<{ alt: string }>;
			}
		).images[0].alt;
	assert.equal(alt("zh-TW"), opengraphImageRoute().alt);
	assert.equal(alt("en"), "Hawkr — A marketplace for local artisans");
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("the root names no twitter images: a card follows the og:image of its own page", () => {
	// Next fills `twitter.images` from `openGraph.images` only while the card has
	// none of its own (postProcessMetadata). A root `twitter.images` would be
	// inherited by every page that declares no `twitter` — a product page with
	// its own photo among them — and pin its twitter:image to the site card while
	// its og:image is the photo.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const twitter = buildRootMetadata("zh-TW").twitter as Record<string, unknown>;
	assert.ok(
		!("images" in twitter),
		"root twitter must not declare images — they follow openGraph.images",
	);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("the one root in the file's own segment leaves the picture to the file convention", () => {
	// app/global-not-found.tsx sits next to app/opengraph-image.tsx: Next folds
	// the file in there (content-hashed URL) only while the metadata declares no
	// `images` key at all.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const og = buildRootMetadata("zh-TW", { shareImage: false })
		.openGraph as Record<string, unknown>;
	assert.ok(!("images" in og));
	const source = readFileSync(
		path.join(ROOT, "app/global-not-found.tsx"),
		"utf8",
	);
	assert.match(
		source,
		/buildRootMetadata\(defaultLocale, \{ shareImage: false \}\)/,
	);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});
