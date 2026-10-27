import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const pagePath = new URL("../app/[locale]/(public)/page.tsx", import.meta.url);
const snapshotPath = new URL(
	"../features/home/homeSnapshot.ts",
	import.meta.url,
);
const bootstrapServerPath = new URL(
	"../lib/api/home-bootstrap.server.ts",
	import.meta.url,
);

// #338 SEO restore: the homepage main content (product / seller / post cards)
// is server-rendered into the initial HTML for crawlers, not fetched client-side
// into an empty shell. These assertions lock the wiring that makes that true.

test("home page is a server component that renders the snapshot into HTML", async () => {
	const source = await readFile(pagePath, "utf8");

	// A client directive here would move the whole tree back off the server.
	assert.doesNotMatch(source, /^"use client";/);
	// Async server component that awaits the strict bootstrap and reshapes it with
	// the shared pure builder before handing it to the client view as a prop.
	assert.match(
		source,
		/export default async function Page\(\{\s*params,?\s*\}/,
	);
	assert.match(source, /await loadHomeRenderBootstrap\(\)/);
	assert.match(source, /buildHomeSnapshot\(bootstrap\)/);
	assert.match(source, /<HomePage snapshot=\{snapshot\} \/>/);

	// Traditional ISR: prerender the HTML, regenerate on the live-commerce window,
	// and keep the client route-cache tree for instant BFCache revisits.
	assert.match(source, /export const revalidate = 60/);
	assert.match(source, /export const unstable_dynamicStaleTime = 300/);

	// ItemList JSON-LD for the products the HTML lists (same contract as
	// /market/products), skipped when the fallback snapshot is empty.
	assert.match(source, /itemListSchema\(productUrls\)/);
	assert.match(
		source,
		/homeSnapshotProductIds\(snapshot\)\.map\(\(id\) =>\s*productUrl\(id, locale\)/,
	);
	assert.match(source, /productUrls\.length > 0 && <JsonLd/);
});

test("home page keeps the strict-throw / prerenderSafe freshness contract", async () => {
	const [page, bootstrap] = await Promise.all([
		readFile(pagePath, "utf8"),
		readFile(bootstrapServerPath, "utf8"),
	]);

	// Build phase softens (no prior prerender to keep); runtime re-throws so an
	// empty homepage is never cached as fresh — ISR keeps the last good page.
	assert.match(page, /prerenderSafe\(EMPTY_HOME_BOOTSTRAP\)/);

	// The server bootstrap uses the strict cached helpers with the exact same args
	// as the browser route, and must NOT swallow errors into an empty payload.
	assert.match(bootstrap, /import "server-only"/);
	assert.match(bootstrap, /getCachedMarketplace\(8, 6, 8\)/);
	assert.match(bootstrap, /getCachedProductsFirstPage\(100\)/);
	assert.match(bootstrap, /getCachedRecommendedSellers\(24\)/);
	assert.match(bootstrap, /getCachedPublicPosts\(1, 8\)/);
	assert.doesNotMatch(bootstrap, /\.catch\(/);
});

test("home snapshot builder stays a pure, client-safe module", async () => {
	const source = await readFile(snapshotPath, "utf8");

	// Shared by the server page and the client refetch, so it must not be a client
	// module and must not drag the server-only data cache into the client graph.
	assert.doesNotMatch(source, /^"use client";/);
	assert.doesNotMatch(source, /from "@\/lib\/api\/cached"/);
	assert.doesNotMatch(source, /from "next\/cache"/);
	assert.doesNotMatch(source, /\bfetch\(|window\./);
	assert.match(source, /export function buildHomeSnapshot\(/);
	assert.match(source, /export function homeSnapshotProductIds\(/);
	// The canonical seller-logo transform moved here with the builder.
	assert.match(source, /logo_url: recommendedSellerLogoUrl\(seller\)/);
});
