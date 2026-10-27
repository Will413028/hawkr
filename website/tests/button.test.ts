import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import Button, { buttonClasses } from "../components/ui/Button";
import { withAppRouter } from "./mocks/app-router";

test("buttonClasses: solid primary md defaults — faithful to hkui-btn", () => {
	const c = buttonClasses({});
	assert.ok(c.includes("inline-flex"));
	assert.ok(c.includes("h-11"), "md height (44px touch target)");
	assert.ok(c.includes("px-4"), "md padding");
	assert.ok(c.includes("gap-2"), "md gap");
	assert.ok(c.includes("text-sm"), "md font");
	assert.ok(c.includes("active:scale-[0.97]"), "hkui active scale");
	assert.ok(c.includes("[font-family:var(--hkui-font-sans)]"), "hkui font");
	assert.ok(c.includes("bg-[var(--hkui-primary)]"), "solid primary bg");
	assert.ok(!/\bmin-w-/.test(c), "hkui-btn has no min-width");
});

test("buttonClasses: bordered has 2px border (hkui --bordered)", () => {
	const c = buttonClasses({ variant: "bordered", color: "primary" });
	assert.ok(c.includes("border-2"), "bordered is 2px");
	assert.ok(c.includes("border-[var(--hkui-primary)]"));
});

test("buttonClasses: bordered default carries dark override", () => {
	// The three literals moved to --page-button-bordered-* in
	// styles/foundation/00-color-config.css (same values, now tokenised).
	const c = buttonClasses({ variant: "bordered", color: "default" });
	assert.ok(c.includes("dark:border-[var(--page-button-bordered-border)]"));
	assert.ok(c.includes("dark:text-[var(--page-button-bordered-fg)]"));
	assert.ok(c.includes("dark:hover:bg-[var(--page-button-bordered-hover-bg)]"));
});

test("buttonClasses: sizes map to hkui control ladder", () => {
	assert.ok(buttonClasses({ size: "sm" }).includes("h-8"));
	assert.ok(buttonClasses({ size: "lg" }).includes("h-12"));
});

test("buttonClasses: radius full + fullWidth + iconOnly", () => {
	assert.ok(buttonClasses({ radius: "full" }).includes("rounded-full"));
	assert.ok(buttonClasses({ fullWidth: true }).includes("w-full"));
	assert.ok(buttonClasses({ isIconOnly: true, size: "md" }).includes("w-11"));
});

test("Button renders <button> by default", () => {
	const html = renderToStaticMarkup(
		React.createElement(Button, { color: "primary" }, "Go"),
	);
	assert.ok(html.startsWith("<button"));
});

test("Button with href renders <a> (Next Link → <a> in SSR)", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(Button, { href: "/x", color: "primary" }, "Go"),
		),
	);
	assert.ok(html.startsWith("<a"));
	assert.ok(html.includes('href="/x"'));
});

test("Button disabled href is aria-disabled, not <button disabled>", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(Button, { href: "/x", disabled: true }, "Go"),
		),
	);
	assert.ok(html.includes('aria-disabled="true"'));
});

test("Button loading uses the branded fixed-width pending contract", () => {
	const html = renderToStaticMarkup(
		React.createElement(
			Button,
			{ isLoading: true, loadingLabel: "送出合作申請中…", type: "submit" },
			"送出合作申請",
		),
	);
	assert.ok(html.includes('disabled=""'));
	assert.ok(html.includes('aria-busy="true"'));
	assert.ok(html.includes('aria-hidden="true"'));
	assert.ok(html.includes("opacity-0"), "original label keeps button geometry");
	assert.ok(html.includes("w-max shrink-0"), "pending width uses max-content");
	assert.ok(
		html.includes("absolute inset-0"),
		"visible pending content does not participate in intrinsic sizing",
	);
	assert.ok(
		html.includes("il-logo"),
		"brand LOGO replaces the generic spinner",
	);
	assert.ok(html.includes("送出合作申請中…"));
	assert.equal(
		(html.match(/role="status"/g) ?? []).length,
		1,
		"only the visible pending content announces",
	);
	assert.ok(html.includes("bg-[var(--hkui-default-300)]!"));
	assert.ok(html.includes("disabled:opacity-100!"));
	assert.ok(html.includes("motion-reduce:transition-none"));
	assert.ok(!html.includes("animate-spin"));
	assert.equal(
		(html.match(/送出合作申請中…/g) ?? []).length,
		2,
		"one inert reservation fixes width while one active status reports progress",
	);
});

test("Button reserves explicit pending content before loading starts", () => {
	const html = renderToStaticMarkup(
		React.createElement(Button, { loadingLabel: "送出售後申請中…" }, "送出"),
	);
	assert.ok(html.includes("grid"));
	assert.ok(html.includes("送出售後申請中…"));
	assert.ok(html.includes("opacity-0"));
	assert.ok(html.includes("w-max shrink-0"));
	assert.ok(html.includes("whitespace-nowrap"));
	assert.ok(html.includes('aria-hidden="true"'));
	assert.ok(
		!html.includes("il-logo"),
		"idle width reservation must not mount an invisible animation",
	);
	assert.ok(!html.includes('aria-busy="true"'));
	assert.ok(!html.includes('role="status"'));
});

test("Button reserves the widest toggle label while announcing the exact action", () => {
	const html = renderToStaticMarkup(
		React.createElement(
			Button,
			{
				isLoading: true,
				loadingLabel: "追蹤商家中…",
				loadingReservationLabel: "取消追蹤商家中…",
			},
			"追蹤",
		),
	);

	assert.ok(html.includes("取消追蹤商家中…"));
	assert.ok(html.includes("追蹤商家中…"));
	assert.equal((html.match(/role="status"/g) ?? []).length, 1);
	assert.equal((html.match(/取消追蹤商家中…/g) ?? []).length, 1);
});

test("Button loading link exposes both disabled and busy semantics", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(
				Button,
				{ href: "/x", isLoading: true, loadingLabel: "前往頁面中…" },
				"前往頁面",
			),
		),
	);
	assert.ok(html.includes('aria-disabled="true"'));
	assert.ok(html.includes('aria-busy="true"'));
	assert.ok(html.includes("前往頁面中…"));
});

test("Button loading preserves caller aria-busy when not using isLoading", () => {
	const html = renderToStaticMarkup(
		React.createElement(Button, { "aria-busy": true }, "預約出貨"),
	);
	assert.ok(html.includes('aria-busy="true"'));
});

test("Button loading accepts a semantic indicator without changing its contract", () => {
	const html = renderToStaticMarkup(
		React.createElement(
			Button,
			{
				isLoading: true,
				loadingLabel: "上傳商品中…",
				loadingIndicator: React.createElement("span", {
					"data-upload-indicator": "true",
				}),
			},
			"上傳商品",
		),
	);
	assert.ok(html.includes('data-upload-indicator="true"'));
	assert.ok(html.includes("上傳商品中…"));
	assert.ok(html.includes('disabled=""'));
	assert.ok(html.includes('aria-busy="true"'));
	assert.ok(!html.includes("il-logo"));
	assert.equal(
		(html.match(/data-upload-indicator="true"/g) ?? []).length,
		1,
		"the semantic animation only mounts in the active pending layer",
	);
});

test("Brand button loader has a static reduced-motion state", () => {
	const css = readFileSync(
		new URL("../styles/components/decorations-overlays.css", import.meta.url),
		"utf8",
	);
	assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
	assert.match(
		css,
		/\.il-blade\s*\{\s*animation:\s*none;\s*opacity:\s*1;\s*transform:\s*none;/,
	);
});

test("buttonClasses: warning text uses -700 shade (hkui contrast)", () => {
	// hkui-btn uses var(--hkui-warning-700) for warning text on bordered/light
	// (transparent bg needs the darker shade); solid keeps the -foreground token.
	assert.ok(
		buttonClasses({ variant: "bordered", color: "warning" }).includes(
			"text-[var(--hkui-warning-700)]",
		),
	);
	assert.ok(
		buttonClasses({ variant: "light", color: "warning" }).includes(
			"text-[var(--hkui-warning-700)]",
		),
	);
});
