import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const homePagePath = new URL("../features/home/HomePage.tsx", import.meta.url);
const mobileBottomNavPath = new URL(
	"../components/shared/MobileBottomNav.tsx",
	import.meta.url,
);

test("homepage data cannot reopen a server-streaming blank state", async () => {
	const source = await readFile(homePagePath, "utf8");

	assert.match(source, /^"use client";/);
	assert.match(source, /queryKey: queryKeys\.home\.snapshot\(\)/);
	assert.match(source, /enabled: isBrowser/);
	assert.match(source, /staleTime: HOME_SNAPSHOT_STALE_MS/);
	assert.match(source, /gcTime: HOME_SNAPSHOT_GC_MS/);
	// SEO restore (#338): the server builds the snapshot and hands it down as a
	// prop; HomeContent seeds it as react-query initialData so the crawlable HTML
	// carries content and the browser query only reconciles afterwards. The
	// browser refetch reshapes its payload with the same pure builder.
	assert.match(
		source,
		/export default function HomePage\(\{(?=[^}]*\bsnapshot\b)[^}]*\}/,
	);
	assert.match(source, /initialData: initialSnapshot/);
	assert.match(
		source,
		/<HomeContent\b(?=[^>]*\binitialSnapshot=\{snapshot\})[^>]*\/>/,
	);
	assert.match(source, /return buildHomeSnapshot\(bootstrap\)/);
	// Pending renders null; only the isError terminal branch may render.
	assert.match(
		source,
		/if \(!homeQuery\.data\) \{[\s\S]*?if \(homeQuery\.isError\) \{[\s\S]*?return null;\s*\}/,
	);
	assert.doesNotMatch(source, /from "next\/server"/);
	assert.doesNotMatch(source, /from "@\/lib\/api\/cached"/);
	assert.doesNotMatch(source, /await connection\(/);
	assert.doesNotMatch(source, /<Suspense/);
	assert.doesNotMatch(source, /getCachedSellerStats/);
	assert.doesNotMatch(source, /getCachedSeller\b/);
	assert.doesNotMatch(source, /mappedSellers\.map\(async/);
	assert.match(source, /<HomeCompactMerchants sellers=\{mappedSellers\} \/>/);
});

test("homepage leaves pending content to the single initial loader", async () => {
	const homeSource = await readFile(homePagePath, "utf8");

	// The non-error pending branch must stay null — ErrorState is an error
	// terminal, not a pending surface, so it must be isError-gated.
	assert.match(
		homeSource,
		/if \(!homeQuery\.data\) \{[\s\S]*?if \(homeQuery\.isError\) \{[\s\S]*?return null;\s*\}/,
	);
	assert.doesNotMatch(homeSource, /HomeContentFallback/);
	assert.doesNotMatch(homeSource, /HomeShelfSkeleton/);
	assert.doesNotMatch(homeSource, /aria-label="首頁內容載入中"/);
});

test("mobile market tab bypasses the legacy redirect", async () => {
	const source = await readFile(mobileBottomNavPath, "utf8");

	assert.match(
		source,
		/\{ label: "市集", href: "\/market\/products", icon: "market" \}/,
	);
	assert.doesNotMatch(
		source,
		/\{ label: "市集", href: "\/market", icon: "market" \}/,
	);
});
