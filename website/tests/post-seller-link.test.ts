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
	const home = await readFile(componentPaths["HomeStageFeedCard.tsx"], "utf8");
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
	const home = await readFile(componentPaths["HomeStageFeedCard.tsx"], "utf8");
	const branch = home.slice(home.indexOf("{sellerHref ?"));
	const elseArm = branch.slice(branch.indexOf(") : ("));
	assert.match(
		elseArm,
		/\{sellerIdentity\}/,
		"the non-linked branch must still render the seller identity",
	);
});
