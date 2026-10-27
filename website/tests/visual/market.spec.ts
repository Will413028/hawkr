import { test } from "@playwright/test";
import { gotoThemed, snap, type Theme } from "./_helpers";

/**
 * Visual baseline for the `styles/features/market.css` → Tailwind migration.
 *
 * These are historical, full-page migration captures backed by a mutable
 * shared dev dataset. Product count/content and seller id `1` are encoded in
 * the PNGs, so this must never run as a general full-suite quality gate:
 * ordinary backend edits would create large false diffs (or a legitimate 404)
 * without any CSS regression.
 *
 * Opt in only while actively migrating market.css, after restoring the matching
 * visual fixture dataset. General regression coverage belongs to site-stability
 * and the semantic nav/layout specs.
 */
test.skip(
	process.env.RUN_LIVE_MARKET_VISUAL_MIGRATION !== "1",
	"historical live-data migration baseline; opt in with RUN_LIVE_MARKET_VISUAL_MIGRATION=1 after restoring its fixture dataset",
);

const THEMES: Theme[] = ["light", "dark"];

const ROUTES: Array<{ name: string; url: string }> = [
	// /market permanent-redirects to /market/products.
	{ name: "market-products", url: "/market/products" },
	{ name: "seller-detail", url: "/sellers/1" },
];

test.describe("market.css visual baseline", { tag: ["@ui", "@live"] }, () => {
	for (const route of ROUTES) {
		for (const theme of THEMES) {
			test(`${route.name} (${theme})`, async ({ page }) => {
				await gotoThemed(page, route.url, theme);
				await snap(page, `${route.name}-${theme}`);
			});
		}
	}
});
