import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { sellerPath } from "../lib/seo/urls";

/**
 * Why post cards must not fall back to `sellerId`.
 *
 * `sellerPath(code, fallbackId)` is right almost everywhere: a recommended-seller
 * row or a product's seller block can legitimately lack a minted `storeCode`,
 * and `/sellers/{store_id}` 308s to the canonical path — one redirect, not a dead
 * link.
 *
 * Posts invert that. The gateway fills `storeCode` only when `GetSellerBatch`
 * actually found the seller (`marketplace.go`: the assignment sits inside
 * `if info, ok := sellerMap[sellerID]; ok`), so an empty value means NOT FOUND —
 * and `/sellers/{store_id}` then 404s. Passing the fallback turns "do not link
 * this" into a link that is guaranteed to break. `post.go` says so in the field
 * comment; these tests are what stops the sentence from being only a comment.
 *
 * Observed 2026-08-04 in the scheduled contract run (30949696548): two post cards
 * rendered `/sellers/7487659509985814869` and `/sellers/7487329918184193303`,
 * both 404. Their name and avatar came through fine — social-service keeps a
 * denormalised snapshot — so only the link was ever wrong.
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

test("the fallback really does manufacture the dead link", () => {
	// Not a strawman: this is exactly what the two components used to do, and it
	// is why a comment alone was not enough. The function is behaving correctly —
	// the caller was asking it the wrong question.
	assert.equal(
		sellerPath("", "7487659509985814869"),
		"/sellers/7487659509985814869",
		"passing a post's sellerId as fallback yields the URL that 404s",
	);
	// One argument, and the caller gets a falsy value it can branch on.
	assert.equal(sellerPath(""), "");
	assert.equal(sellerPath(null), "");
	assert.equal(sellerPath(undefined), "");
	// And it still builds the canonical path when a code IS present, so guarding
	// costs nothing in the normal case.
	assert.equal(sellerPath("2605576172"), "/sellers/2605576172");
	// The fallback stays available for every other surface, where it is correct.
	assert.equal(sellerPath(undefined, "2605576172"), "/sellers/2605576172");
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
