import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("shared switch uses HeroUI accessible switch behavior and brand tokens", () => {
	const sharedSwitch = source("../components/ui/Switch.tsx");

	assert.match(sharedSwitch, /Switch as HeroSwitch/);
	assert.match(sharedSwitch, /isSelected=\{checked\}/);
	assert.match(sharedSwitch, /onChange=\{onCheckedChange\}/);
	assert.match(sharedSwitch, /isDisabled=\{disabled \|\| busy\}/);
	assert.match(sharedSwitch, /aria-label=\{ariaLabel\}/);
	assert.match(sharedSwitch, /hk-heroui-brand-scope/);
	assert.match(sharedSwitch, /<HeroSwitch\.Control>/);
	assert.match(sharedSwitch, /<HeroSwitch\.Thumb/);
});

test("shipping rows use the shared switch and lock during updates", () => {
	const shipping = source(
		"../features/seller-center/profile/shipping/ShippingClient.tsx",
	);
	const styles = source("../styles/features/seller-center.css");

	assert.match(shipping, /import Switch from "@\/components\/ui\/Switch"/);
	assert.match(shipping, /const \[toggling, setToggling\]/);
	assert.match(
		shipping,
		/<Switch[\s\S]*checked=\{reference\.enabled === "true"\}/,
	);
	assert.match(shipping, /disabled=\{toggling\}/);
	assert.match(shipping, /busy=\{toggling\}/);
	assert.match(shipping, /toggling \? "更新中…"/);
	assert.doesNotMatch(shipping, /className=.*shipping-toggle/);
	assert.doesNotMatch(styles, /\.shipping-toggle/);
});
