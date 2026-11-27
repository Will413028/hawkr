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

test("home page is a server component that renders the snapshot into HTML", async () => {
	const source = await readFile(pagePath, "utf8");

	assert.doesNotMatch(source, /^"use client";/);
	assert.match(
		source,
		/export default async function Page\(\{\s*params,?\s*\}/,
	);
	assert.match(source, /await loadHomeRenderBootstrap\(\)/);
	assert.match(source, /buildHomeSnapshot\(bootstrap\)/);
	assert.match(source, /<HomePage snapshot=\{snapshot\} \/>/);

	assert.doesNotMatch(source, /export const revalidate\b/);
	assert.match(source, /export const unstable_dynamicStaleTime = 300/);

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

	assert.match(page, /prerenderSafe\(EMPTY_HOME_BOOTSTRAP\)/);

	assert.match(bootstrap, /import "server-only"/);
	assert.match(bootstrap, /getCachedHotProducts\(8\)/);
	assert.match(bootstrap, /getCachedProductsFirstPage\(100\)/);
	assert.match(bootstrap, /getCachedRecommendedSellers\(24\)/);
	assert.match(bootstrap, /getCachedPublicPosts\(8\)/);
	assert.doesNotMatch(bootstrap, /\.catch\(/);
});

test("home snapshot builder stays a pure, client-safe module", async () => {
	const source = await readFile(snapshotPath, "utf8");

	assert.doesNotMatch(source, /^"use client";/);
	assert.doesNotMatch(source, /from "@\/lib\/api\/cached"/);
	assert.doesNotMatch(source, /from "next\/cache"/);
	assert.doesNotMatch(source, /\bfetch\(|window\./);
	assert.match(source, /export function buildHomeSnapshot\(/);
	assert.match(source, /export function homeSnapshotProductIds\(/);
	assert.match(source, /logo_url: store\.logoUrl/);
});
