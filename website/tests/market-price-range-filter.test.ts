import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { deriveMarketListView } from "../lib/market/deriveListView";

const source = readFileSync(
	new URL(
		"../features/market/products/MarketProductsListLayout.tsx",
		import.meta.url,
	),
	"utf8",
);
const styles = readFileSync(
	new URL("../styles/features/market-products-list.css", import.meta.url),
	"utf8",
);

const emptyMarketData = {
	categoryList: [],
	trendingHashtags: [],
};

test("market price range derives numeric bounds and a Taiwan-dollar chip", () => {
	const view = deriveMarketListView(
		{ priceMin: "100", priceMax: "500", tab: "nearby" },
		emptyMarketData,
	);

	assert.equal(view.fetchParams.priceMin, 100);
	assert.equal(view.fetchParams.priceMax, 500);
	assert.equal(view.currentTab, "nearby");
	assert.deepEqual(
		view.appliedChips.find((chip) => chip.kind === "price"),
		{
			key: "price",
			kind: "price",
			label: "價格",
			value: "NT$ 100 – NT$ 500",
			href: "/market/products?tab=nearby",
			ariaLabel: "移除價格範圍",
		},
	);
});

test("market price controls keep two usable fields separate from the submit action", () => {
	assert.match(source, /<label htmlFor="price-min">最低價<\/label>/);
	assert.match(source, /<label htmlFor="price-max">最高價<\/label>/);
	assert.equal(
		source.match(/<span aria-hidden="true">NT\$<\/span>/g)?.length,
		2,
	);
	assert.equal(
		source.match(/data-slot="control"/g)?.length,
		2,
		"both inner number inputs must opt out of the global bare-input frame",
	);
	assert.match(source, /loadingLabel="套用價格範圍中…"[\s\S]*fullWidth/);
	assert.match(
		source,
		/currentTab !== "recommend"[\s\S]*name="tab" value=\{currentTab\}/,
	);
	assert.match(
		source,
		/onChange=\{\(event\) => \{[\s\S]*maximumRef\.current\?\.setCustomValidity\(""\);[\s\S]*setMinimum/,
	);

	assert.match(
		styles,
		/\.market-list-sidebar__price-fields \{[\s\S]*grid-template-columns: minmax\(0, 1fr\) auto minmax\(0, 1fr\)/,
	);
	assert.match(
		styles,
		/\.market-list-sidebar__price button \{[\s\S]*width: 100%;[\s\S]*min-height: 44px/,
	);
	assert.match(
		styles,
		/@container market-price-range \(max-width: 12\.5rem\) \{[\s\S]*grid-template-columns: minmax\(0, 1fr\)/,
	);
	assert.match(
		styles,
		/\.market-list-sidebar__price-input:focus-within \{[\s\S]*border-color: var\(--theme-action-primary\);[\s\S]*box-shadow: 0 0 0 2px var\(--theme-primary-border\)/,
	);
	assert.match(
		styles,
		/\.market-list-sidebar__price input \{[\s\S]*border: 0;[\s\S]*border-radius: 0;[\s\S]*box-shadow: none;[\s\S]*font-variant-numeric: tabular-nums/,
	);
	assert.match(
		styles,
		/\.market-list-sidebar__price-input:has\(input:invalid\):focus-within \{[\s\S]*box-shadow: 0 0 0 2px color-mix\(in srgb, var\(--hkui-danger\) 28%, transparent\)/,
	);
	assert.match(
		styles,
		/\.market-list-sidebar__price-input \{[\s\S]*background: var\(--theme-surface-elevated\);/,
	);
	assert.doesNotMatch(styles, /grid-template-columns: 1fr auto 1fr auto/);
});
