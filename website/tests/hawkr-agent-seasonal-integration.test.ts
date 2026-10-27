import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

const nav = source("../features/home/components/HomeCompactCategoryNav.tsx");
const foodIcon = source("../components/shared/FoodCategoryIcon.tsx");

test("homepage renders exactly nine food-category chips with a registry glyph each", () => {
	const categoryIds = [...nav.matchAll(/categoryId: "([^"]+)"/g)].map(
		(match) => match[1],
	);
	assert.deepEqual(categoryIds, [
		"vegetables",
		"fruits",
		"seafood",
		"meat",
		"egg-dairy",
		"processed",
		"spices",
		"grains",
		"flowers",
	]);
	assert.match(nav, /<FoodCategoryIcon categoryId=\{item\.categoryId\} \/>/);
	assert.match(nav, /data-seasonal-category=\{item\.categoryId\}/);
	assert.doesNotMatch(nav, /season=|home-design\/mascots|\.json/);
	// Every category id has a glyph, and the box is decorative.
	for (const id of categoryIds) {
		assert.match(foodIcon, new RegExp(`"?${id}"?: "[a-z-]+"`));
	}
	assert.match(foodIcon, /aria-hidden="true"/);
	assert.doesNotMatch(foodIcon, /<img|AppImage|\.webp|\.png/);
});

test("promo chips use a registry glyph inside their accent frame", () => {
	assert.match(nav, /icon: "sparkle",\s*frameClass: "home-chip-frame--gold"/);
	assert.match(
		nav,
		/icon: "discount",\s*frameClass: "home-chip-frame--primary"/,
	);
	assert.match(nav, /<AppIcon name=\{item\.icon\} size=\{22\} \/>/);
});

test("food-only chip interaction reverses via CSS and reduced motion pins both layers", () => {
	const styles = source("../styles/features/home.css");
	assert.match(
		styles,
		/\.home-chip\[data-seasonal-category\] \.home-chip-icon \{[\s\S]*?transition: transform 180ms cubic-bezier\(0\.2, 0, 0, 1\);/,
	);
	assert.match(
		styles,
		/@media \(hover: hover\) and \(pointer: fine\) \{[\s\S]*?\.home-chip\[data-seasonal-category\]:hover \.home-chip-icon \{[\s\S]*?transform: translateY\(-1px\) scale\(1\.05\);/,
	);
	assert.match(
		styles,
		/\.home-chip\[data-seasonal-category\]:focus-visible \.home-chip-icon \{[\s\S]*?transform: translateY\(-1px\) scale\(1\.05\);/,
	);
	assert.match(
		styles,
		/\.home-chip\[data-seasonal-category\]:hover,[\s\S]*?\.home-chip\[data-seasonal-category\]:active \{[\s\S]*?translate: 0;/,
	);
	assert.match(
		styles,
		/\.home-chip\[data-seasonal-category\]:active \.home-chip-icon \{[\s\S]*?transform: scale\(0\.98\);/,
	);
	assert.match(
		styles,
		/@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.home-chip\[data-seasonal-category\],[\s\S]*?transform: none;[\s\S]*?translate: 0;[\s\S]*?transition: none;[\s\S]*?\.home-chip\[data-seasonal-category\] \.home-chip-icon,[\s\S]*?\.home-chip\[data-seasonal-category\]:hover \.home-chip-icon,[\s\S]*?animation: none;[\s\S]*?transform: none;[\s\S]*?transition: none;/,
	);
});
