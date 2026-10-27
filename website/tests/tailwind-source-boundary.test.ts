import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { isAbsolute, join, relative, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import tailwind from "@tailwindcss/postcss";
import postcss, { type Result, type Root, type Rule } from "postcss";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const GLOBALS = join(ROOT, "styles/globals.css");
const RUNTIME_ROOTS = ["app", "components", "features", "lib"] as const;

// If automatic repository scanning is accidentally restored, this test file is
// itself a source candidate and this otherwise-unused color will enter the CSS.
const TEST_ONLY_SENTINEL = "bg-[#010203]";

let compiledGlobals: Promise<Result> | undefined;

function compileGlobals(): Promise<Result> {
	compiledGlobals ??= readFile(GLOBALS, "utf8").then((css) =>
		postcss([tailwind({ base: ROOT, optimize: false })]).process(css, {
			from: GLOBALS,
		}),
	);
	return compiledGlobals;
}

function isWithin(parent: string, child: string): boolean {
	const path = relative(parent, child);
	return (
		path === "" ||
		(path !== ".." && !path.startsWith(`..${sep}`) && !isAbsolute(path))
	);
}

function projectPath(path: string): string {
	return relative(ROOT, path).split(sep).join("/");
}

function dependencyFiles(result: Result): string[] {
	const files = new Set<string>();
	for (const message of result.messages) {
		if (message.type !== "dependency" || !("file" in message)) continue;
		if (typeof message.file === "string") files.add(message.file);
	}
	return [...files].sort();
}

function sourceDirectories(result: Result): string[] {
	const directories = new Set<string>();
	for (const message of result.messages) {
		if (message.type !== "dir-dependency" || !("dir" in message)) continue;
		if (typeof message.dir === "string") directories.add(message.dir);
	}
	return [...directories].sort();
}

function rulesFor(root: Root, selector: string): Rule[] {
	const rules: Rule[] = [];
	root.walkRules((rule) => {
		if (rule.selectors?.includes(selector)) rules.push(rule);
	});
	return rules;
}

function hasDeclaration(
	root: Root,
	property: string,
	value: string,
	selectorFragment?: string,
): boolean {
	let found = false;
	root.walkRules((rule) => {
		if (
			found ||
			(selectorFragment && !rule.selector.includes(selectorFragment))
		) {
			return;
		}
		rule.walkDecls(property, (declaration) => {
			if (declaration.value === value) found = true;
		});
	});
	return found;
}

test("Tailwind scans only product runtime sources and imported styles", async () => {
	const result = await compileGlobals();
	const dependencies = dependencyFiles(result);
	const projectDependencies = dependencies.filter((path) =>
		isWithin(ROOT, path),
	);
	const allowedDependencyRoots = [
		...RUNTIME_ROOTS.map((root) => join(ROOT, root)),
		join(ROOT, "styles"),
		join(ROOT, "node_modules"),
	];
	const unexpected = projectDependencies
		.filter(
			(path) => !allowedDependencyRoots.some((root) => isWithin(root, path)),
		)
		.map(projectPath);

	assert.deepEqual(
		unexpected,
		[],
		"Tailwind registered a project dependency outside the approved runtime/CSS roots",
	);

	const scanDirectories = sourceDirectories(result);
	assert.ok(
		scanDirectories.length > 0,
		"Tailwind reported no source scan roots",
	);
	const unexpectedScanDirectories = scanDirectories
		.filter(
			(path) => !RUNTIME_ROOTS.some((root) => isWithin(join(ROOT, root), path)),
		)
		.map(projectPath);
	assert.deepEqual(
		unexpectedScanDirectories,
		[],
		"Tailwind registered a scanner/watch root outside product runtime code",
	);

	const runtimeDependencies = projectDependencies
		.filter((path) =>
			RUNTIME_ROOTS.some((root) => isWithin(join(ROOT, root), path)),
		)
		.map(projectPath);
	for (const expected of [
		"components/shared/RootDocument.tsx",
		"components/ui/Button.tsx",
		"features/home/components/homeChrome.styles.ts",
		"lib/utils/cn.ts",
	]) {
		assert.ok(
			runtimeDependencies.includes(expected),
			`${expected} is no longer registered as a Tailwind runtime source`,
		);
	}

	const nonRuntimeDependencies = runtimeDependencies.filter(
		(path) =>
			path.includes("/__tests__/") ||
			/\.(?:test|spec)\.[^/]+$/.test(path) ||
			/\.mdx?$/.test(path),
	);
	assert.deepEqual(
		nonRuntimeDependencies,
		[],
		"co-located tests or Markdown re-entered Tailwind's production scan",
	);

	assert.ok(TEST_ONLY_SENTINEL, "keep the test-only scan sentinel in source");
	assert.equal(
		hasDeclaration(result.root, "background-color", "#010203"),
		false,
		"the root tests directory was scanned despite the explicit source boundary",
	);
});

test("the restricted scan keeps runtime class maps and imported HeroUI apply output", async () => {
	const result = await compileGlobals();

	assert.doesNotMatch(
		result.css,
		/@apply\b/,
		"an imported HeroUI @apply directive escaped Tailwind compilation",
	);

	for (const selector of [
		".h-8",
		".h-10",
		".h-12",
		".rounded-none",
		".bg-hk-home-accent-500",
	]) {
		assert.ok(
			rulesFor(result.root, selector).length > 0,
			`${selector} from a runtime static class map is missing`,
		);
	}
	assert.ok(
		hasDeclaration(
			result.root,
			"background-color",
			"var(--hkui-primary)",
			"bg-",
		),
		"the Button primary arbitrary-value utility is missing",
	);
	assert.ok(
		hasDeclaration(
			result.root,
			"border-radius",
			"var(--hkui-radius-md)",
			"rounded-",
		),
		"the Button radius arbitrary-value utility is missing",
	);

	for (const [selector, property, value] of [
		[".toggle-button", "position", "relative"],
		[".color-swatch-picker", "display", "flex"],
		[".slider", "display", "grid"],
		[".switch", "display", "inline-flex"],
	] as const) {
		const rules = rulesFor(result.root, selector);
		assert.ok(rules.length > 0, `${selector} HeroUI component CSS is missing`);
		assert.ok(
			rules.some((rule) =>
				rule.nodes.some(
					(node) =>
						node.type === "decl" &&
						node.prop === property &&
						node.value === value,
				),
			),
			`${selector} did not receive ${property}: ${value} from @apply`,
		);
	}
});
