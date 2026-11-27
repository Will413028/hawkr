import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const pagePath = new URL(
	"../features/market/products/MarketProductsListPage.tsx",
	import.meta.url,
);
const sectionPath = new URL(
	"../features/market/components/MarketProductsSection.tsx",
	import.meta.url,
);
const layoutPath = new URL(
	"../features/market/products/MarketProductsListLayout.tsx",
	import.meta.url,
);
const clientPath = new URL(
	"../features/market/products/MarketProductsListClient.tsx",
	import.meta.url,
);

test("market crosses the client layout with data, never a server product node", async () => {
	const [page, section, layout, client] = await Promise.all([
		readFile(pagePath, "utf8"),
		readFile(sectionPath, "utf8"),
		readFile(layoutPath, "utf8"),
		readFile(clientPath, "utf8"),
	]);

	assert.match(page, /await loadMarketProductsSection\(\{/);
	assert.match(
		page,
		/defaultProducts=\{initialProducts\}[\s\S]*defaultInitialDone=\{initialDone\}/,
	);
	assert.doesNotMatch(page, /productGrid=\{/);
	assert.match(
		client,
		/<MarketProductsListLayout[\s\S]*products=\{products\}[\s\S]*hasActiveFilters=\{view\.hasActiveFilters\}/,
	);
	assert.match(
		layout,
		/<MarketProductsList[\s\S]*products=\{products\}[\s\S]*initialDone=\{initialDone\}/,
	);
	assert.doesNotMatch(section, /export default function MarketProductsSection/);
});
