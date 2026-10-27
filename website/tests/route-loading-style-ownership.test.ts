import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss, { type ChildNode, type Root } from "postcss";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const GLOBAL_TRANSITION_STYLESHEET = "styles/components/page-transition.css";
const ROUTE_LOADING_STYLESHEET = "styles/components/route-loading-skeleton.css";

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

function cssSources(directory: string): string[] {
	const paths: string[] = [];
	for (const entry of readdirSync(directory)) {
		const absolute = join(directory, entry);
		if (statSync(absolute).isDirectory()) {
			paths.push(...cssSources(absolute));
		} else if (entry.endsWith(".css")) {
			paths.push(absolute);
		}
	}
	return paths;
}

function canonicalNode(node: Root | ChildNode): unknown {
	if (node.type === "root") {
		return [
			"root",
			...(node.nodes ?? [])
				.filter((child) => child.type !== "comment")
				.map(canonicalNode),
		];
	}
	if (node.type === "rule") {
		return [
			"rule",
			node.selector,
			...(node.nodes ?? [])
				.filter((child) => child.type !== "comment")
				.map(canonicalNode),
		];
	}
	if (node.type === "atrule") {
		return [
			"atrule",
			node.name,
			node.params,
			...(node.nodes ?? [])
				.filter((child) => child.type !== "comment")
				.map(canonicalNode),
		];
	}
	if (node.type === "decl") {
		return ["decl", node.prop, node.value, Boolean(node.important)];
	}
	assert.fail(`Unsupported CSS AST node: ${node.type}`);
}

function semanticDigest(css: string): string {
	const canonical = JSON.stringify(canonicalNode(postcss.parse(css)));
	return createHash("sha256").update(canonical).digest("hex");
}

function inventory(css: string) {
	const root = postcss.parse(css);
	let styleRules = 0;
	let selectors = 0;
	let declarations = 0;
	let media = 0;
	let keyframes = 0;

	root.walkRules((rule) => {
		const parent = rule.parent;
		if (parent?.type === "atrule" && /(?:^|-)keyframes$/i.test(parent.name)) {
			return;
		}
		styleRules += 1;
		selectors += rule.selectors.length;
	});
	root.walkDecls(() => {
		declarations += 1;
	});
	root.walkAtRules((atRule) => {
		if (atRule.name === "media") media += 1;
		if (/(?:^|-)keyframes$/i.test(atRule.name)) keyframes += 1;
	});

	return { styleRules, selectors, declarations, media, keyframes };
}

test("route loading styles preserve the reviewed semantic CSS inventory", () => {
	const globalTransitionCss = source(GLOBAL_TRANSITION_STYLESHEET);
	const routeLoadingCss = source(ROUTE_LOADING_STYLESHEET);

	assert.deepEqual(inventory(globalTransitionCss), {
		styleRules: 10,
		selectors: 17,
		declarations: 18,
		media: 1,
		keyframes: 5,
	});
	// Includes the reviewed home composition and data-only product grids, with
	// feed/market breakpoints matching their current destination layouts.
	assert.deepEqual(inventory(routeLoadingCss), {
		styleRules: 221,
		selectors: 374,
		declarations: 622,
		media: 12,
		keyframes: 3,
	});

	const recombinedCss = `${globalTransitionCss}\n${routeLoadingCss}`;
	assert.deepEqual(inventory(recombinedCss), {
		styleRules: 231,
		selectors: 391,
		declarations: 640,
		media: 13,
		keyframes: 8,
	});
	// 2026-09-10 px→rem for the text-scale setting
	assert.equal(
		semanticDigest(recombinedCss),
		"0b6cbd17cf0f96836bd13b0bd8d426064b6e7c92cafb7086a24778ce7837c8d7",
		"the split must not drop, duplicate, reorder, or rewrite the reviewed CSS AST",
	);
});

test("only rendering owners import the route loading stylesheet", () => {
	const globals = source("styles/globals.css");
	const pageTransition = source("components/shared/PageTransition.tsx");
	const routeLoadingSkeleton = source(
		"components/shared/RouteLoadingSkeleton.tsx",
	);
	const authBrandLoading = source("components/shared/AuthBrandLoading.tsx");

	assert.match(globals, /@import "\.\/components\/page-transition\.css";/);
	assert.doesNotMatch(globals, /route-loading-skeleton\.css/);
	assert.match(
		routeLoadingSkeleton,
		/import "@\/styles\/components\/route-loading-skeleton\.css";/,
	);
	assert.match(
		authBrandLoading,
		/import "@\/styles\/components\/route-loading-skeleton\.css";/,
	);

	assert.match(
		pageTransition,
		/import type \{ RouteLoadingVariant \} from "\.\/RouteLoadingSkeleton";/,
		"PageTransition must keep a type-only dependency on the skeleton component",
	);
	assert.doesNotMatch(pageTransition, /route-loading-skeleton\.css/);
});

test("skeleton tokens stay local while shared brand primitives remain global", () => {
	const tokenOwners = cssSources(join(ROOT, "styles"))
		.filter((path) => readFileSync(path, "utf8").includes("--skeleton-sheen"))
		.map((path) => relative(ROOT, path));
	assert.deepEqual(tokenOwners, [
		"styles/components/route-loading-layout.css",
		ROUTE_LOADING_STYLESHEET,
	]);
	const tokenDefinitions = cssSources(join(ROOT, "styles"))
		.filter((path) => /--skeleton-sheen\s*:/.test(readFileSync(path, "utf8")))
		.map((path) => relative(ROOT, path));
	assert.deepEqual(tokenDefinitions, [ROUTE_LOADING_STYLESHEET]);
	assert.doesNotMatch(
		source("styles/globals.css"),
		/route-loading-(?:layout|variants)\.css/,
	);

	const globalTransitionCss = source(GLOBAL_TRANSITION_STYLESHEET);
	const routeLoadingCss = source(ROUTE_LOADING_STYLESHEET);
	const globals = source("styles/globals.css");
	const sharedLoaderCss = source("styles/components/decorations-overlays.css");

	assert.doesNotMatch(
		globalTransitionCss,
		/route-loading-skeleton|auth-brand-loading|--skeleton-sheen/,
	);
	assert.match(routeLoadingCss, /\.route-loading-skeleton\s*\{/);
	assert.match(routeLoadingCss, /\.auth-brand-loading\s*\{/);
	assert.match(routeLoadingCss, /--skeleton-sheen:/);

	assert.match(globals, /@import "\.\/components\/decorations-overlays\.css";/);
	for (const sharedSelector of [".il-logo", ".il-word", ".il-blade"]) {
		assert.match(sharedLoaderCss, new RegExp(`\\${sharedSelector}\\b`));
	}
	assert.doesNotMatch(routeLoadingCss, /^\.il-(?:logo|word|blade)\b/m);
});
