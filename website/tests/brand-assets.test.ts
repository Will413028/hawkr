import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	HAWKR_APP_ICON_SVG,
	HAWKR_LOCKUP_DARK_SVG,
} from "../lib/brand/hawkr-svg";

function read(relativePath: string) {
	return readFileSync(
		new URL(`../../${relativePath}`, import.meta.url),
		"utf8",
	);
}

const COPIES: [string, string][] = [
	["website/public/brand/logo-horizontal.svg", "brand/hawkr/hawkr-lockup.svg"],
	[
		"website/public/brand/logo-horizontal-light.svg",
		"brand/hawkr/hawkr-lockup-dark.svg",
	],
	["website/public/brand/logo-mark.svg", "brand/hawkr/hawkr-mark.svg"],
	["website/public/brand/wordmark.svg", "brand/hawkr/hawkr-wordmark.svg"],
	["website/public/brand/app-icon.svg", "brand/hawkr/hawkr-app-icon.svg"],
	["website/app/icon.svg", "brand/hawkr/hawkr-favicon.svg"],
	["admin/public/brand/logo-horizontal.svg", "brand/hawkr/hawkr-lockup.svg"],
	[
		"admin/public/brand/logo-horizontal-light.svg",
		"brand/hawkr/hawkr-lockup-dark.svg",
	],
	["admin/public/brand/logo-mark.svg", "brand/hawkr/hawkr-mark.svg"],
];

test("logo files in website and admin are copies of brand/hawkr", () => {
	for (const [copy, source] of COPIES) {
		assert.equal(read(copy), read(source), `${copy} drifted from ${source}`);
	}
});

test("the inline SVG strings for next/og match brand/hawkr", () => {
	assert.equal(HAWKR_APP_ICON_SVG, read("brand/hawkr/hawkr-app-icon.svg"));
	assert.equal(
		HAWKR_LOCKUP_DARK_SVG,
		read("brand/hawkr/hawkr-lockup-dark.svg"),
	);
});
