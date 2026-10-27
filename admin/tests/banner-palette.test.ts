import assert from "node:assert/strict";
import { test } from "node:test";
import {
	COLOR_SWATCHES,
	GRADIENT_PRESETS,
} from "../src/components/banner/bannerPalette";

const HEX = /^#[0-9a-fA-F]{6}$/;

test("banner 色盤的每個色值都是字面 #rrggbb（fabric 不解析 CSS var，會畫成黑色並存進 JSON）", () => {
	const swatches = COLOR_SWATCHES.flat();
	const gradients = GRADIENT_PRESETS.flatMap((g) => [g.c1, g.c2]);
	assert.ok(swatches.length > 0 && gradients.length > 0);
	for (const color of [...swatches, ...gradients]) {
		assert.match(color, HEX);
	}
});
