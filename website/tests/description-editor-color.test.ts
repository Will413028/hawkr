import assert from "node:assert/strict";
import test from "node:test";
import {
	descriptionColorPoint,
	descriptionHexToHsv,
	descriptionHexToRgb,
	descriptionHsvToHex,
	descriptionRgbToHex,
	normalizeDescriptionHex,
	parseDescriptionRgb,
} from "../lib/products/descriptionEditorColor";

test("HEX input accepts shorthand/case/hash variants but rejects alpha and invalid characters", () => {
	assert.equal(normalizeDescriptionHex(" #AbC "), "#aabbcc");
	assert.equal(normalizeDescriptionHex("334C0F"), "#334c0f");
	for (const input of [
		"",
		"#12",
		"#12345",
		"#12345678",
		"rgba(0,0,0,1)",
		"#xx0000",
		"12 3456",
	])
		assert.equal(normalizeDescriptionHex(input), null);
});

test("RGB values require all three integer channels within the wire range", () => {
	assert.deepEqual(parseDescriptionRgb(["0", "128", "255"]), [0, 128, 255]);
	assert.equal(descriptionRgbToHex([0, 128, 255]), "#0080ff");
	assert.deepEqual(descriptionHexToRgb("#334c0f"), [51, 76, 15]);
	for (const values of [
		["", "0", "0"],
		["256", "0", "0"],
		["-1", "0", "0"],
		["1.5", "0", "0"],
		["0", "0"],
		["1e2", "0", "0"],
	])
		assert.equal(parseDescriptionRgb(values), null);
});

test("HSV round-trips black, white, grey, primaries and arbitrary source colors exactly", () => {
	for (const hex of [
		"#000000",
		"#ffffff",
		"#808080",
		"#ff0000",
		"#00ff00",
		"#0000ff",
		"#334c0f",
		"#e78f39",
		"#6194d0",
		"#123456",
		"#ffff00",
		"#ff00ff",
		"#00ffff",
	])
		assert.equal(descriptionHsvToHex(descriptionHexToHsv(hex)), hex);
	assert.equal(descriptionHsvToHex({ h: 360, s: 100, v: 100 }), "#ff0000");
	assert.equal(descriptionHsvToHex({ h: -120, s: 100, v: 100 }), "#0000ff");
	assert.equal(descriptionHexToHsv("#777777", 230).h, 230);
});

test("captured pointer movement clamps outside the saturation/brightness plane", () => {
	const box = { left: 10, top: 20, width: 200, height: 100 };
	assert.deepEqual(descriptionColorPoint(110, 70, box), { s: 50, v: 50 });
	assert.deepEqual(descriptionColorPoint(-10, -20, box), { s: 0, v: 100 });
	assert.deepEqual(descriptionColorPoint(999, 999, box), { s: 100, v: 0 });
	assert.deepEqual(
		descriptionColorPoint(0, 0, { left: 0, top: 0, width: 0, height: 0 }),
		{ s: 0, v: 100 },
	);
});
