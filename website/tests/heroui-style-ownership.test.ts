import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const HEROUI_STYLESHEET = "styles/foundation/heroui.css";
const RUNTIME_ROOTS = ["app", "components", "features", "lib"] as const;

const STYLE_BY_RUNTIME_EXPORT = {
	ColorSwatch: "color-swatch",
	ColorSwatchPicker: "color-swatch-picker",
	RouterProvider: null,
	Slider: "slider",
	Switch: "switch",
	ToggleButton: "toggle-button",
	ToggleButtonGroup: "toggle-button-group",
	parseColor: null,
} as const;

const EXPECTED_RUNTIME_EXPORTS = Object.keys(STYLE_BY_RUNTIME_EXPORT).sort();
const EXPECTED_COMPONENT_STYLES = [
	"color-swatch",
	"color-swatch-picker",
	"slider",
	"switch",
	"toggle-button",
	"toggle-button-group",
].sort();

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

function walkRuntimeSources(directory: string): string[] {
	const files: string[] = [];
	for (const entry of readdirSync(directory)) {
		const absolute = join(directory, entry);
		if (statSync(absolute).isDirectory()) {
			if (entry !== "__tests__") files.push(...walkRuntimeSources(absolute));
			continue;
		}
		if (!/\.[jt]sx?$/.test(entry) || /\.(?:test|spec)\.[jt]sx?$/.test(entry)) {
			continue;
		}
		files.push(absolute);
	}
	return files;
}

function walkCssSources(directory: string): string[] {
	const files: string[] = [];
	for (const entry of readdirSync(directory)) {
		const absolute = join(directory, entry);
		if (statSync(absolute).isDirectory()) {
			files.push(...walkCssSources(absolute));
		} else if (entry.endsWith(".css")) {
			files.push(absolute);
		}
	}
	return files;
}

function runtimeHeroUIExports(): string[] {
	const imports: string[] = [];
	const unsupported: string[] = [];
	for (const root of RUNTIME_ROOTS) {
		for (const absolute of walkRuntimeSources(join(ROOT, root))) {
			const contents = readFileSync(absolute, "utf8");
			const relativePath = relative(ROOT, absolute);
			const coveredRanges: Array<[number, number]> = [];
			const importPattern =
				/import\s+([^;]*?)\s+from\s+["']@heroui\/react["']\s*;?/g;
			for (const match of contents.matchAll(importPattern)) {
				const start = match.index ?? -1;
				assert.notEqual(
					start,
					-1,
					`${relativePath}: import location is missing`,
				);
				coveredRanges.push([start, start + match[0].length]);
				const clause = match[1]?.trim() ?? "";
				if (clause.startsWith("type ")) continue;
				const named = clause.match(/^\{([\s\S]*)\}$/);
				assert.ok(
					named,
					`${relativePath}: @heroui/react runtime imports must remain named imports`,
				);
				for (const rawSpecifier of (named[1] ?? "").split(",")) {
					const specifier = rawSpecifier
						.replaceAll(/\/\*[\s\S]*?\*\//g, "")
						.trim();
					if (!specifier || specifier.startsWith("type ")) continue;
					const imported = specifier.match(
						/^([A-Za-z_$][\w$]*)(?:\s+as\s+[A-Za-z_$][\w$]*)?$/,
					)?.[1];
					assert.ok(
						imported,
						`${relativePath}: cannot determine HeroUI export from ${specifier}`,
					);
					imports.push(imported);
				}
			}

			for (const occurrence of contents.matchAll(/@heroui\/react/g)) {
				const index = occurrence.index ?? -1;
				if (
					!coveredRanges.some(([start, end]) => index >= start && index < end)
				) {
					unsupported.push(relativePath);
				}
			}
		}
	}
	assert.deepEqual(
		[...new Set(unsupported)].sort(),
		[],
		"dynamic, side-effect, or re-exported @heroui/react usage bypasses the CSS ownership map",
	);
	return [...new Set(imports)].sort();
}

test("HeroUI runtime exports and granular component styles stay exhaustive", () => {
	const runtimeExports = runtimeHeroUIExports();
	assert.deepEqual(
		runtimeExports,
		EXPECTED_RUNTIME_EXPORTS,
		"new HeroUI runtime exports need an explicit CSS ownership decision",
	);

	const unknown = runtimeExports.filter(
		(name) => !(name in STYLE_BY_RUNTIME_EXPORT),
	);
	assert.deepEqual(unknown, []);

	const requiredStyles = [
		...new Set(
			runtimeExports
				.map(
					(name) =>
						STYLE_BY_RUNTIME_EXPORT[
							name as keyof typeof STYLE_BY_RUNTIME_EXPORT
						],
				)
				.filter((name) => name !== null),
		),
	].sort();
	assert.deepEqual(requiredStyles, EXPECTED_COMPONENT_STYLES);

	const stylesheet = source(HEROUI_STYLESHEET);
	const loadedStyles = [
		...stylesheet.matchAll(
			/@import\s+["']@heroui\/styles\/components\/([^"']+)\.css["']\s+layer\(components\);/g,
		),
	]
		.map((match) => match[1] ?? "")
		.sort();
	assert.deepEqual(
		loadedStyles,
		requiredStyles,
		"HeroUI CSS must include every and only the component families rendered at runtime",
	);
});

test("HeroUI CSS uses only public granular exports in upstream layer order", () => {
	const stylesheet = source(HEROUI_STYLESHEET);
	const projectCssFiles = walkCssSources(join(ROOT, "styles"));
	assert.ok(projectCssFiles.length > 0, "the project CSS audit found no files");
	const allProjectCss = projectCssFiles
		.map((path) => readFileSync(path, "utf8"))
		.join("\n");

	assert.doesNotMatch(
		allProjectCss,
		/@import\s+["']@heroui\/styles(?:\/css)?["']/,
		"the full HeroUI stylesheet silently restores every component family",
	);
	assert.doesNotMatch(
		allProjectCss,
		/@import\s+["']@heroui\/styles\/components(?:\/index\.css)?["']/,
		"the component index silently restores every component family",
	);

	const orderedMarkers = [
		"@layer theme, base, components, utilities;",
		'@import "tw-animate-css";',
		'@import "@heroui/styles/base" layer(base);',
		'@import "@heroui/styles/components/toggle-button.css" layer(components);',
		'@import "@heroui/styles/components/toggle-button-group.css" layer(components);',
		'@import "@heroui/styles/components/color-swatch.css" layer(components);',
		'@import "@heroui/styles/components/color-swatch-picker.css" layer(components);',
		'@import "@heroui/styles/components/slider.css" layer(components);',
		'@import "@heroui/styles/components/switch.css" layer(components);',
		'@import "@heroui/styles/themes/default" layer(theme);',
		'@import "@heroui/styles/utilities";',
		'@import "@heroui/styles/variants";',
	];
	let previous = -1;
	for (const marker of orderedMarkers) {
		const index = stylesheet.indexOf(marker);
		assert.ok(index > previous, `${marker} is missing or out of order`);
		previous = index;
	}

	const publicImports = [
		...stylesheet.matchAll(/@import\s+["']([^"']+)["']/g),
	].map((match) => match[1] ?? "");
	for (const specifier of publicImports) {
		if (specifier === "tw-animate-css") continue;
		const resolved = import.meta.resolve(specifier);
		assert.ok(
			resolved.startsWith("file:") && existsSync(fileURLToPath(resolved)),
			`${specifier} must resolve to an existing public package export`,
		);
	}

	const twAnimatePackagePath = join(
		ROOT,
		"node_modules/tw-animate-css/package.json",
	);
	assert.ok(
		existsSync(twAnimatePackagePath),
		"tw-animate-css must be directly available to the root stylesheet",
	);
	const twAnimatePackage = JSON.parse(
		readFileSync(twAnimatePackagePath, "utf8"),
	) as {
		version?: string;
		exports?: { "."?: { style?: string } };
	};
	const twAnimateStyle = twAnimatePackage.exports?.["."]?.style;
	assert.ok(twAnimateStyle, "tw-animate-css must expose its root CSS export");
	assert.ok(
		existsSync(join(ROOT, "node_modules/tw-animate-css", twAnimateStyle)),
		"tw-animate-css root style export must point to an existing file",
	);
	const packageJson = JSON.parse(source("package.json")) as {
		dependencies?: Record<string, string>;
	};
	assert.equal(
		packageJson.dependencies?.["tw-animate-css"],
		twAnimatePackage.version,
		"tw-animate-css must be a pinned direct dependency for root CSS resolution",
	);

	const globals = source("styles/globals.css");
	assert.match(
		globals,
		/^@import "tailwindcss" source\(none\);\n@import "\.\/foundation\/heroui\.css";/,
		"Tailwind must load once before the granular HeroUI stylesheet",
	);
});
