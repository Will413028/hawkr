import assert from "node:assert/strict";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { Dropdown, resolveDropdownSelection } from "../components/ui/Dropdown";
import { Input, Select, Textarea } from "../components/ui/Input";
import {
	floatingLabelClasses,
	inputWrapperClasses,
} from "../components/ui/inputWrapperClasses";

test("Input SSR carries data-slot field/label/control/hint + data-variant", () => {
	const html = renderToStaticMarkup(
		React.createElement(Input, {
			variant: "bordered",
			label: "L",
			isRequired: true,
			description: "hint",
		}),
	);
	assert.match(html, /data-slot="field"/);
	assert.match(html, /data-slot="label"/);
	assert.match(html, /data-slot="control"/);
	assert.match(html, /data-slot="hint"/);
	assert.match(html, /data-variant="bordered"/);
	assert.match(html, /data-slot="input-wrapper"/);
});

test("Input floatingLabel SSR carries data-floating + floating-label slot", () => {
	const html = renderToStaticMarkup(
		React.createElement(Input, { label: "L", floatingLabel: true }),
	);
	assert.match(html, /data-floating=""/);
	assert.match(html, /data-slot="floating-label"/);
});

// 2026-09-10 產品負責人拍板：浮動標籤填入後上浮保持可見
// （取代舊的「填入後 opacity-0 淡出」行為）。以下三個測試釘住新契約：
// idle = 置中的 placeholder；active = 上浮到 top-2、縮到 12px 字級底線、
// 且**永遠不得**再帶 opacity-0；欄位本身要在控制項上留出對應的頂部內距。
test("floating label idle sits centred as a placeholder, active floats to the top and stays visible", () => {
	const idle = floatingLabelClasses({ active: false, size: "md" });
	const active = floatingLabelClasses({ active: true, size: "md" });

	// idle：置中、與值文字同字級、沒有位移
	assert.match(idle, /(?:^| )top-1\/2(?: |$)/);
	assert.match(idle, /(?:^| )-translate-y-1\/2(?: |$)/);
	assert.match(idle, /(?:^| )text-sm(?: |$)/);

	// active：上浮至 top-2、縮到 12px 底線、完全不透明
	assert.match(active, /(?:^| )top-2(?: |$)/);
	assert.match(active, /(?:^| )translate-y-0(?: |$)/);
	assert.match(active, /(?:^| )text-xs(?: |$)/);
	assert.doesNotMatch(active, /opacity-0/);
	assert.doesNotMatch(idle, /opacity-0/);

	// 位移／字級／顏色都會動，所以 transition 要列出這些屬性並尊重 reduced motion
	for (const cls of [idle, active]) {
		assert.match(cls, /transition-\[top,transform,font-size,color\]/);
		assert.match(cls, /motion-reduce:transition-none/);
	}

	// Small labels retain readable body ink in both themes; focused labels
	// follow the theme accent instead of dark-on-dark raw primary.
	assert.match(idle, /text-\[color:var\(--hkui-field-copy\)\]/);
	assert.match(active, /text-\[color:var\(--hkui-field-copy\)\]/);
	assert.match(
		floatingLabelClasses({ active: true, focused: true, size: "md" }),
		/text-\[color:var\(--theme-text-accent\)\]/,
	);
});

test("flat floating fields reserve the top strip on the control, non-floating ones are untouched", () => {
	const base = {
		variant: "flat",
		size: "md",
		focused: false,
		invalid: false,
		disabled: false,
	} as const;
	const floating = inputWrapperClasses({ ...base, floating: true });
	const plain = inputWrapperClasses({ ...base, floating: false });

	// 頂部留白加在「控制項」上（不是 wrapper），前後綴圖示才會維持對齊
	// 整個欄位的垂直中線；屬性選擇器 (0,2,0) 也穩定壓過 CONTROL_RESET 的 p-0。
	assert.match(floating, /\[&>\[data-slot=control\]\]:pt-5/);
	assert.match(floating, /\[&>\[data-slot=control\]\]:pb-1/);
	assert.match(floating, /\[&_\[data-slot=dropdown-value\]\]:pt-5/);
	// md 浮動欄位由 44 → 48px（12px 標籤 + 14px 值放不進 44）
	assert.match(floating, /(?:^| )min-h-12(?: |$)/);
	assert.match(plain, /(?:^| )min-h-11(?: |$)/);
	assert.doesNotMatch(plain, /data-slot=control\]\]:pt-/);

	// textarea：標籤釘在左上角的內距裡，第一行文字讓開 12px 標籤
	const textarea = inputWrapperClasses({
		...base,
		floating: true,
		textarea: true,
	});
	assert.match(textarea, /\[&>\[data-slot=textarea\]\]:pt-3/);
	assert.match(
		floatingLabelClasses({ active: true, textarea: true }),
		/(?:^| )top-2\.5(?: |$)/,
	);
});

test("Dropdown exposes the data-slot the floating layout targets", () => {
	const html = renderToStaticMarkup(
		React.createElement(Dropdown, {
			value: undefined,
			onChange: () => {},
			items: ["a", "b"],
			label: "L",
			floatingLabel: true,
		}),
	);
	// Tailwind 把 arbitrary variant 裡的 `_` 當成後代選擇器，所以 BEM 的
	// `.hkui-dropdown__value` 無法從 utility 定位——一律走 data-slot。
	assert.match(html, /data-slot="dropdown-value"/);
});

test("Dropdown whose value is the empty entry looks empty, one without the flag looks filled", () => {
	const render = (empty: boolean) =>
		renderToStaticMarkup(
			React.createElement(Dropdown, {
				value: "",
				onChange: () => {},
				items: [
					{ value: "", label: "尚未設定", empty },
					{ value: "1", label: "男性" },
				],
				label: "性別",
				placeholder: "請選擇性別",
			}),
		);
	const emptyEntry = render(true);
	assert.match(emptyEntry, /hkui-dropdown__value is-placeholder/);
	assert.match(emptyEntry, />請選擇性別</);
	assert.doesNotMatch(emptyEntry, />尚未設定</);

	const plainEntry = render(false);
	assert.doesNotMatch(plainEntry, /is-placeholder/);
	assert.match(plainEntry, />尚未設定</);
});

test("Dropdown menu still marks the empty entry as selected while the trigger shows nothing", () => {
	const items = [
		{ value: "", label: "尚未設定", empty: true },
		{ value: "1", label: "男性" },
	];
	const empty = resolveDropdownSelection(items, "");
	assert.equal(empty.selected?.value, "");
	assert.equal(empty.shown, undefined);

	const filled = resolveDropdownSelection(items, "1");
	assert.equal(filled.selected?.value, "1");
	assert.equal(filled.shown?.value, "1");

	const unknown = resolveDropdownSelection(items, "9");
	assert.equal(unknown.selected, undefined);
	assert.equal(unknown.shown, undefined);
});

test("Textarea SSR carries data-slot textarea", () => {
	const html = renderToStaticMarkup(
		React.createElement(Textarea, { label: "L" }),
	);
	assert.match(html, /data-slot="textarea"/);
});

test("Select SSR carries data-slot control", () => {
	const html = renderToStaticMarkup(
		React.createElement(Select, { label: "L" }),
	);
	assert.match(html, /data-slot="control"/);
});

test("Dropdown SSR carries field/input-wrapper/variant + floating", () => {
	const html = renderToStaticMarkup(
		React.createElement(Dropdown, {
			value: undefined,
			onChange: () => {},
			items: ["a", "b"],
			label: "L",
			floatingLabel: true,
		}),
	);
	assert.match(html, /data-slot="field"/);
	assert.match(html, /data-slot="input-wrapper"/);
	assert.match(html, /data-variant="bordered"/);
	assert.match(html, /data-floating=""/);
});
