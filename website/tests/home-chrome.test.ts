import assert from "node:assert/strict";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import HomeButton from "../features/home/components/HomeButton";
import HomeEyebrow from "../features/home/components/HomeEyebrow";
import HomeSectionHeader from "../features/home/components/HomeSectionHeader";
import {
	buttonClasses,
	eyebrowClasses,
	eyebrowDotClasses,
	sectionDescriptionClasses,
	sectionHeaderClasses,
	sectionHeadingClasses,
	sectionMoreClasses,
	sectionMoreFontStyle,
} from "../features/home/components/homeChrome.styles";
import { withAppRouter } from "./mocks/app-router";

test("eyebrowClasses: light tone (default)", () => {
	const c = eyebrowClasses();
	assert.ok(c.includes("inline-flex"));
	assert.ok(c.includes("rounded-full"));
	assert.ok(c.includes("tracking-[0.26em]"));
	assert.ok(c.includes("bg-white/70"));
	assert.ok(c.includes("text-hk-home-accent-700"));
});

test("eyebrowClasses: on-dark tone", () => {
	const c = eyebrowClasses(true);
	assert.ok(c.includes("bg-white/12"));
	assert.ok(c.includes("text-hk-home-paper"));
	assert.ok(!c.includes("bg-white/70"));
});

test("eyebrowDotClasses: gold dot", () => {
	assert.ok(eyebrowDotClasses.includes("bg-hk-brand-secondary"));
	assert.ok(eyebrowDotClasses.includes("size-1.5"));
});

test("buttonClasses: primary md (defaults)", () => {
	const c = buttonClasses({});
	assert.ok(c.includes("rounded-full"));
	assert.ok(c.includes("hover:-translate-y-px"));
	assert.ok(c.includes("py-3.5 px-6"));
	// 2026-09-10 px→rem for the text-scale setting
	assert.ok(c.includes("text-[0.9375rem]"));
	assert.ok(c.includes("bg-hk-home-accent-500"));
	assert.ok(c.includes("hover:bg-hk-home-primary-deep"));
});

test("buttonClasses: ghost light", () => {
	const c = buttonClasses({ variant: "ghost" });
	assert.ok(c.includes("border-[1.5px]"));
	assert.ok(c.includes("border-hk-home-ink/22"));
	assert.ok(c.includes("text-hk-home-ink"));
	assert.ok(!c.includes("bg-hk-home-accent-500"));
});

test("buttonClasses: ghost on-dark lg", () => {
	const c = buttonClasses({ variant: "ghost", size: "lg", onDark: true });
	assert.ok(c.includes("py-[18px] px-[30px]"));
	assert.ok(c.includes("text-base"));
	assert.ok(c.includes("bg-hk-home-paper/94"));
	assert.ok(c.includes("text-hk-home-primary-deep"));
	assert.ok(c.includes("hover:text-hk-home-deep-ink"));
});

test("buttonClasses: onDark is a no-op for primary", () => {
	assert.equal(buttonClasses({ onDark: true }), buttonClasses({}));
});

test("sectionHeaderClasses: between (default)", () => {
	const c = sectionHeaderClasses();
	assert.ok(c.includes("flex justify-between"));
	assert.ok(c.includes("max-w-[1280px]"));
	assert.ok(c.includes("px-[6vw]"));
	assert.ok(c.includes("max-[640px]:flex-col"));
});

test("sectionHeaderClasses: center", () => {
	const c = sectionHeaderClasses("center");
	assert.ok(c.includes("text-center"));
	assert.ok(c.includes("max-w-[1180px]"));
	assert.ok(!c.includes("justify-between"));
	assert.ok(!c.includes("px-[6vw]"));
});

test("sectionHeadingClasses: mb-3 only when description present", () => {
	assert.ok(sectionHeadingClasses(true).includes("mb-3"));
	assert.ok(!sectionHeadingClasses(false).includes("mb-3"));
	assert.ok(sectionHeadingClasses(false).includes("mt-4"));
	assert.ok(
		sectionHeadingClasses(false).includes(
			"text-[clamp(2.125rem,4.4vw,3.75rem)]",
		),
	);
});

test("section description + more constants", () => {
	assert.ok(sectionDescriptionClasses.includes("max-w-[36ch]"));
	assert.ok(sectionMoreClasses.includes("border-b-[1.5px]"));
	assert.ok(
		sectionMoreClasses.includes(
			"hover:text-[color:var(--color-brand-primary-deep)]",
		),
	);
	assert.equal(sectionMoreFontStyle.fontFamily, '"Roboto", var(--font-body)');
});

test("HomeEyebrow: renders dot + children, light tone", () => {
	const html = renderToStaticMarkup(
		React.createElement(HomeEyebrow, null, "今日市集"),
	);
	assert.ok(html.includes("bg-white/70"));
	assert.ok(html.includes("bg-hk-brand-secondary")); // the dot
	assert.ok(html.includes("今日市集"));
});

test("HomeEyebrow: onDark tone", () => {
	const html = renderToStaticMarkup(
		React.createElement(HomeEyebrow, { onDark: true }, "SELLER"),
	);
	assert.ok(html.includes("bg-white/12"));
	assert.ok(html.includes("text-hk-home-paper"));
});

test("HomeButton: primary renders an anchor with href", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(HomeButton, { href: "/market/products" }, "逛市集"),
		),
	);
	assert.ok(html.startsWith("<a"));
	assert.ok(html.includes('href="/market/products"'));
	assert.ok(html.includes("bg-hk-home-accent-500"));
	assert.ok(html.includes("逛市集"));
});

test("HomeButton: ghost on-dark lg", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(
				HomeButton,
				{ href: "/seller", variant: "ghost", size: "lg", onDark: true },
				"了解合作方式",
			),
		),
	);
	assert.ok(html.includes("bg-hk-home-paper/94"));
	assert.ok(html.includes("py-[18px]"));
});

test("HomeSectionHeader: between with description + more link", () => {
	const html = renderToStaticMarkup(
		withAppRouter(
			React.createElement(
				HomeSectionHeader,
				{
					eyebrow: "今日分類 · CATEGORIES",
					description: "從蔬菜水果到地方熟食。",
					moreHref: "/market/products",
					moreLabel: "全部分類 →",
				},
				"今天想買什麼？",
			),
		),
	);
	assert.ok(html.includes("justify-between"));
	assert.ok(html.includes("<h2"));
	assert.ok(html.includes("今天想買什麼？"));
	assert.ok(html.includes("<p")); // description present
	assert.ok(html.includes('href="/market/products"'));
	assert.ok(html.includes("border-b-[1.5px]")); // the more link
	assert.ok(html.includes("mb-3")); // h2 spacing with description
});

test("HomeSectionHeader: center, no description, no more", () => {
	const html = renderToStaticMarkup(
		React.createElement(
			HomeSectionHeader,
			{ align: "center", eyebrow: "安心購買 · WHY HAWKR" },
			"為什麼在 Hawkr 買？",
		),
	);
	assert.ok(html.includes("text-center"));
	assert.ok(html.includes("<h2"));
	assert.ok(!html.includes("<p")); // no description
	assert.ok(!html.includes("border-b-[1.5px]")); // no more link
	assert.ok(!html.includes("mb-3")); // h2 has no bottom margin without description
});

test("HomeSectionHeader: between with description but no more link (DEMO case)", () => {
	const html = renderToStaticMarkup(
		React.createElement(
			HomeSectionHeader,
			{
				eyebrow: "產品 DEMO · HOW IT WORKS",
				description: "從想吃什麼到送到家，只要幾步。",
			},
			"從想吃什麼，到送到家",
		),
	);
	assert.ok(html.includes("justify-between")); // between layout
	assert.ok(html.includes("<p")); // description present
	assert.ok(html.includes("mb-3")); // h2 spacing with description
	assert.ok(!html.includes("border-b-[1.5px]")); // no more link rendered
	assert.ok(!html.includes("href=")); // no link at all
});
