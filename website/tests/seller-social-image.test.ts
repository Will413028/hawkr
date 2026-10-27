import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript-compiler-api";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { sellerSocialImage, socialImages } from "../lib/seo/metadata";

/**
 * One seller, four URLs, one share picture.
 *
 * `/sellers/{id}` and its three tabs (`/reviews`, `/posts`, `/messages`) each
 * used to decide for themselves which backend field represents a store. The
 * index had the full cascade (cover, else logo); `/reviews` carried a copy of
 * it; `/posts` and `/messages` carried a shorter one that knew only about the
 * cover. A seller with a logo and no cover therefore shared as an avatar card
 * from two of its own tabs and as the generic site card from the other two —
 * nothing looked broken on any of the four pages, because a share picture is
 * only ever seen in someone else's chat window (#436).
 *
 * Two halves, and both are needed:
 *   1. the cascade itself, asserted on the pure function
 *   2. a source gate proving no route re-implements it — the failure mode here
 *      is a fifth copy drifting in, which behaviour tests on the current four
 *      routes cannot see
 */

const ROUTES = [
	"features/sellers/storefront/metadata.ts",
	"features/sellers/reviews/SellerReviewsPage.tsx",
	"features/sellers/posts/SellerPostsPage.tsx",
	"features/sellers/messages/SellerMessagesPage.tsx",
] as const;

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

function withMediaGateway<T>(run: () => T): T {
	__setFlavorForTest("dev");
	__setEnvForTest(null);
	__setEnvForTest({
		...getEnv(),
		mediaBaseUrl: "https://test-media.example.com",
	});
	try {
		return run();
	} finally {
		__setEnvForTest(null);
		__setFlavorForTest(null);
	}
}

test("a seller's share picture is cover → banner, else logo → avatar", () => {
	withMediaGateway(() => {
		assert.equal(
			sellerSocialImage({
				coverImage: "public/seller_cover/1/c.webp",
				logo_url: "public/user_avatar/1/l.png",
			}),
			"https://test-media.example.com/i/banner/public/seller_cover/1/c.webp",
			"a cover wins, and it is served at the wide banner preset",
		);

		assert.equal(
			sellerSocialImage({
				coverImage: "",
				logo_url: "public/user_avatar/1/l.png",
			}),
			"https://test-media.example.com/i/avatar/public/user_avatar/1/l.png",
			"no cover falls back to the logo at the square avatar preset",
		);

		// A whitespace-only cover is the same thing as no cover. Without the trim
		// this returns the banner preset applied to "   ", i.e. a broken image.
		assert.equal(
			sellerSocialImage({
				coverImage: "   ",
				logo_url: "public/user_avatar/1/l.png",
			}),
			"https://test-media.example.com/i/avatar/public/user_avatar/1/l.png",
		);
	});
});

test("a seller with neither picture falls through to the site card", () => {
	withMediaGateway(() => {
		const resolved = sellerSocialImage({ coverImage: null, logo_url: null });
		assert.equal(resolved, "", "nothing to build a URL from");
		// The empty string is load-bearing: socialImages() turns it into the site
		// card rather than an empty images array (see tests/seo-social-image.test.ts).
		assert.deepEqual(socialImages(resolved), [{ url: "/opengraph-image" }]);
	});
});

test("every seller route takes its share picture from the shared resolver", () => {
	for (const route of ROUTES) {
		const text = source(route);
		assert.match(
			text,
			/sellerSocialImage\(s\)/,
			`${route} must resolve its share picture through sellerSocialImage`,
		);
		// The whole point is that the cascade exists once. A route that spells
		// buildImageUrl(...) itself has started a fifth copy.
		assert.doesNotMatch(
			text,
			/buildImageUrl\(\s*s\.(?:coverImage|logo_url|bannerUrl)/,
			`${route} must not re-implement the cover/logo cascade`,
		);
	}
});

/**
 * The four routes agree because they call the same function — proven by walking
 * each `generateMetadata` and reading the expression that actually reaches
 * `openGraph.images`. Grep cannot tell "the value passed to socialImages" from
 * "the string sellerSocialImage appears somewhere in this file".
 */
test("openGraph.images is fed by sellerSocialImage on all four routes", () => {
	for (const route of ROUTES) {
		const file = ts.createSourceFile(
			route,
			source(route),
			ts.ScriptTarget.Latest,
			true,
			ts.ScriptKind.TSX,
		);

		// `const socialImage = sellerSocialImage(s)` — the binding each route's
		// openGraph block hands to socialImages().
		let socialImageIsShared = false;
		let imagesArgument: string | null = null;

		const visit = (node: ts.Node): void => {
			if (
				ts.isVariableDeclaration(node) &&
				ts.isIdentifier(node.name) &&
				node.name.text === "socialImage" &&
				node.initializer &&
				ts.isCallExpression(node.initializer) &&
				ts.isIdentifier(node.initializer.expression) &&
				node.initializer.expression.text === "sellerSocialImage"
			) {
				socialImageIsShared = true;
			}
			if (
				ts.isPropertyAssignment(node) &&
				ts.isIdentifier(node.name) &&
				node.name.text === "images" &&
				ts.isCallExpression(node.initializer) &&
				ts.isIdentifier(node.initializer.expression) &&
				node.initializer.expression.text === "socialImages"
			) {
				imagesArgument = node.initializer.arguments[0]?.getText() ?? "";
			}
			ts.forEachChild(node, visit);
		};
		visit(file);

		assert.ok(
			socialImageIsShared,
			`${route}: socialImage must be assigned from sellerSocialImage(...)`,
		);
		assert.equal(
			imagesArgument,
			"socialImage",
			`${route}: openGraph.images must be socialImages(socialImage)`,
		);
	}
});
