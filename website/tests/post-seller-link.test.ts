import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { sellerPath } from "../lib/seo/urls";

/**
 * Post cards link the storefront from the post's `storeCode` alone.
 *
 * The gateway fills `storeCode` only when `GetSellerBatch` actually found the
 * seller, so an empty value means NOT FOUND: the card renders the seller without
 * a link. `sellerPath` takes no store_id fallback because `/sellers/{store_id}`
 * is not a storefront URL and answers 404.
 */

const componentPaths = {
	"HomeStageFeedCard.tsx": fileURLToPath(
		new URL(
			"../features/home/components/HomeStageFeedCard.tsx",
			import.meta.url,
		),
	),
	"PostDetailViewer.tsx": fileURLToPath(
		new URL(
			"../features/feed/components/PostDetailViewer.tsx",
			import.meta.url,
		),
	),
};

test("sellerPath gives a falsy value without a code", () => {
	assert.equal(sellerPath(""), "");
	assert.equal(sellerPath(null), "");
	assert.equal(sellerPath(undefined), "");
	// And it still builds the canonical path when a code IS present, so guarding
	// costs nothing in the normal case.
	assert.equal(sellerPath("2605576172"), "/sellers/2605576172");
});

test("post-facing callers pass storeCode alone", async () => {
	for (const [name, path] of Object.entries(componentPaths)) {
		const source = await readFile(path, "utf8");
		const calls = source.match(/sellerPath\([^)]*\)/g) ?? [];
		assert.ok(calls.length > 0, `${name} should still build a seller path`);
		for (const call of calls) {
			assert.doesNotMatch(
				call,
				/,/,
				`${name}: ${call} — a post's empty storeCode means the seller was not found, so a sellerId fallback produces a 404. Guard on the empty string instead.`,
			);
		}
	}
});

test("post-facing callers guard on the empty result", async () => {
	// Passing one argument is only half of it: the caller must also branch, or it
	// renders `href=""`, which resolves to the current page — a link that looks
	// alive and goes nowhere, which is worse than the 404 it replaced.
	for (const [name, path] of Object.entries(componentPaths)) {
		const source = await readFile(path, "utf8");
		assert.match(
			source,
			/(sellerHref \?|sellerPath\(post\.storeCode\) \?)/,
			`${name} must branch on the empty path rather than render href=""`,
		);
	}
});

test("dropping the anchor does not move anything on screen", async () => {
	// The two branches must lay out identically — same flex box, same gap — so a
	// seller whose link is withheld looks the same as one whose link is present,
	// minus the affordance. Only anchor-specific classes may differ: `no-underline`
	// (nothing to underline without an <a>) and `group` (it exists solely to drive
	// `group-hover:text-hk-primary`, and a non-link must not offer hover feedback).
	const home = await readFile(componentPaths["HomeStageFeedCard.tsx"], "utf8");
	// Scoped to the sellerHref ternary. Searching the whole file finds an earlier
	// `) : (` — the image placeholder — and silently compares the wrong element,
	// which is how this assertion first "failed" on a correct component.
	const ternary = home.slice(home.indexOf("{sellerHref ?"));
	const linked = ternary.match(
		/<Link\s+href=\{sellerHref\}[\s\S]*?className="([^"]+)"/,
	)?.[1];
	const plain = ternary.match(/\) : \(\s*<div className="([^"]+)"/)?.[1];
	assert.ok(linked && plain, "both branches must carry a className");
	const layoutOf = (cls: string) =>
		cls
			.split(/\s+/)
			.filter((c) => c !== "group" && c !== "no-underline")
			.sort()
			.join(" ");
	assert.equal(
		layoutOf(plain),
		layoutOf(linked),
		"the non-linked branch must keep the linked branch's layout classes",
	);
});

test("the seller's name and avatar are not withheld along with the link", async () => {
	// The snapshot social-service carries is still good; hiding the identity would
	// turn a missing link into a broken-looking card. Only the anchor goes away.
	const home = await readFile(componentPaths["HomeStageFeedCard.tsx"], "utf8");
	const branch = home.slice(home.indexOf("{sellerHref ?"));
	const elseArm = branch.slice(branch.indexOf(") : ("));
	assert.match(
		elseArm,
		/\{sellerIdentity\}/,
		"the non-linked branch must still render the seller identity",
	);
});
