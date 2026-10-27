import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import SellerFeatureOverview from "../features/seller/landing/components/SellerFeatureOverview";
import { SELLER_FEATURE_OVERVIEW } from "../features/seller/landing/sellerFeatureOverviewData";

const ICON_COMPONENT_PATH = new URL(
	"../features/seller/landing/components/SellerFeatureOverviewIcon.tsx",
	import.meta.url,
);

const ICON_BY_FEATURE = {
	storefront: "home",
	products: "bag",
	inventory: "stock",
	orders: "document",
	fulfillment: "location",
	customers: "users",
	content: "discount",
	messages: "message",
	operations: "chart",
} as const;

test("every seller feature heading maps to its own semantic icon", () => {
	assert.deepEqual(
		SELLER_FEATURE_OVERVIEW.map(({ icon }) => icon).sort(),
		Object.keys(ICON_BY_FEATURE).sort(),
	);
	assert.equal(new Set(Object.values(ICON_BY_FEATURE)).size, 9);

	const source = readFileSync(ICON_COMPONENT_PATH, "utf8");
	for (const [feature, icon] of Object.entries(ICON_BY_FEATURE)) {
		assert.match(source, new RegExp(`${feature}: "${icon}"`));
	}
});

test("the heading icon is a static decorative AppIcon", () => {
	const source = readFileSync(ICON_COMPONENT_PATH, "utf8");

	assert.match(source, /<AppIcon name=\{SELLER_FEATURE_ICONS\[icon\]\}/);
	assert.match(source, /aria-hidden="true"/);
	assert.doesNotMatch(source, /import\(|<svg|tabIndex|role="button"/);
});

test("server markup exposes nine decorative icons beside intact headings", () => {
	const html = renderToStaticMarkup(React.createElement(SellerFeatureOverview));

	assert.equal((html.match(/data-seller-feature-icon=/g) ?? []).length, 9);
	assert.equal((html.match(/class="hk-app-icon/g) ?? []).length, 9);
	assert.equal((html.match(/aria-hidden="true"/g) ?? []).length >= 9, true);
	for (const group of SELLER_FEATURE_OVERVIEW) {
		assert.match(
			html,
			new RegExp(`<h3 id="${group.id}-title">${group.title}</h3>`),
		);
	}
});
