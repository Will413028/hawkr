import assert from "node:assert/strict";
import { test } from "node:test";
import { inputWrapperData } from "../components/ui/inputWrapperClasses";

test("inputWrapperData: slot + variant always present", () => {
	const d = inputWrapperData({
		variant: "bordered",
		focused: false,
		floating: false,
	});
	assert.equal(d["data-slot"], "input-wrapper");
	assert.equal(d["data-variant"], "bordered");
	assert.equal(d["data-focus"], undefined);
	assert.equal(d["data-floating"], undefined);
});

test("inputWrapperData: focus + floating become present-when-true (empty string)", () => {
	const d = inputWrapperData({
		variant: "flat",
		focused: true,
		floating: true,
	});
	assert.equal(d["data-focus"], "");
	assert.equal(d["data-floating"], "");
	assert.equal(d["data-variant"], "flat");
});
