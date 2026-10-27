import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join, relative, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";

const ROOT = fileURLToPath(new URL("../", import.meta.url));
const APP_ROOT = join(ROOT, "app");
const SOURCE_ROOTS = ["app", "components", "features", "lib"];
const SOURCE_EXTENSIONS = new Set([".ts", ".tsx"]);
const STATIC_ASSET_PREFIXES = [
	"/_next/",
	"/api/",
	"/placeholders/",
	"/assets/",
];
const OBSERVED_NAVIGATION_FILES = [
	"components/shared/Link.tsx",
	"instrumentation-client.ts",
	"lib/analytics/NavigationEvents.tsx",
	"lib/loading/navigation-watchdog.ts",
] as const;
const DIRECT_ROUTER_INFRA_ALLOWLIST = new Set([
	"components/shared/Link.tsx",
	"components/shared/view-transitions.tsx",
	"lib/intl/use-router.ts",
	"lib/navigation/use-observed-router.ts",
]);

interface NavigationTarget {
	path: string;
	source: string;
	line: number;
}

interface NavigationInventory {
	targets: NavigationTarget[];
	unsafeTargets: NavigationTarget[];
	nativeInternalAnchors: NavigationTarget[];
}

test("shared navigation lifecycle is wired to observability", () => {
	const sources = Object.fromEntries(
		OBSERVED_NAVIGATION_FILES.map((file) => [
			file,
			readFileSync(join(ROOT, file), "utf8"),
		]),
	);
	assert.match(
		sources["components/shared/Link.tsx"],
		/beginNavigationIntent\(/,
	);
	assert.match(
		sources["instrumentation-client.ts"],
		/markRouterTransitionStarted\(/,
	);
	assert.match(
		sources["lib/analytics/NavigationEvents.tsx"],
		/markNavigationCommitted\(/,
	);
	assert.match(
		sources["lib/loading/navigation-watchdog.ts"],
		/reportNavigationWatchdogRecovery\(/,
	);
	assert.match(
		sources["instrumentation-client.ts"],
		/runNavigationHookSafely\(\(\) =>/,
	);
	assert.match(
		sources["lib/loading/navigation-watchdog.ts"],
		/try \{\s*reportNavigationWatchdogRecovery\(targetHref\);\s*\} finally \{/,
	);
});

test("programmatic navigation uses the semantic observed-router layer", () => {
	const violations: string[] = [];
	for (const sourceRoot of SOURCE_ROOTS) {
		for (const file of walk(join(ROOT, sourceRoot))) {
			if (!SOURCE_EXTENSIONS.has(extname(file))) continue;
			// Co-located tests exercise the router layer; they are not navigation call sites.
			if (/\.(?:test|spec)\.[jt]sx?$/.test(file)) continue;
			const source = relative(ROOT, file);
			if (DIRECT_ROUTER_INFRA_ALLOWLIST.has(source)) continue;
			const text = readFileSync(file, "utf8");
			if (
				/\buseRouter\s*\(/.test(text) ||
				/import\s*\{[^}]*\buseRouter\b[^}]*\}\s*from\s*["']next\/navigation["']/.test(
					text,
				)
			) {
				violations.push(source);
			}
		}
	}
	assert.deepEqual(
		violations,
		[],
		`Direct useRouter bypasses component/action ownership:\n${violations.join("\n")}`,
	);
});

test("observed router keeps navigation live and arms only commit-arrival motion", () => {
	const source = readFileSync(
		join(ROOT, "lib/navigation/use-observed-router.ts"),
		"utf8",
	);
	assert.match(source, /armProgrammaticRouteMotion\(args\[0\]\)/);
	assert.match(source, /cancelProgrammaticRouteMotion\(motionToken\)/);
	assert.doesNotMatch(source, /useTransitionRouter/);
	assert.doesNotMatch(source, /startViewTransition/);
});

test("hardNavigation stays scoped to authentication boundaries", () => {
	const offenders: string[] = [];
	for (const sourceRoot of SOURCE_ROOTS) {
		for (const file of walk(join(ROOT, sourceRoot))) {
			if (!SOURCE_EXTENSIONS.has(extname(file))) continue;
			// Co-located tests may mock Link props; they are not application
			// navigation call sites and must not enter this production inventory.
			if (/\.(?:test|spec)\.[jt]sx?$/.test(file)) continue;
			const source = relative(ROOT, file);
			if (source === "components/shared/view-transitions.tsx") continue;
			if (source === "components/shared/Link.tsx") continue;
			if (source === "components/shared/PageTransition.tsx") continue;
			const text = readFileSync(file, "utf8");
			if (!/\bhardNavigation\b/.test(text)) continue;
			// A full document load is the auth/recovery escape hatch, not a way to
			// paper over an in-app navigation that misbehaves. Every ordinary route
			// must stay a soft navigation.
			if (!/["'`]\/login/.test(text)) offenders.push(source);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		`hardNavigation outside an auth boundary:\n${offenders.join("\n")}`,
	);
});

test("leaving a product quick-view is a normal top-of-page navigation", () => {
	const modal = readFileSync(
		join(ROOT, "features/product/components/ProductModal.tsx"),
		"utf8",
	);
	const transition = readFileSync(
		join(ROOT, "components/shared/PageTransition.tsx"),
		"utf8",
	);
	// The pinned scroll belongs to the opener page: released without restore,
	// and never re-pinned, when the user navigated onwards instead of back.
	assert.match(modal, /isQuickViewOpener\(window\.location\.href\)/);
	assert.match(modal, /restoreScrollPosition:\s*returnedToOpener/);
	assert.match(modal, /if \(!returnedToOpener\) return;/);
	// The instant no-scroll swap is reserved for opening the overlay and for
	// closing it back onto its opener.
	assert.match(transition, /isQuickViewOpener\(window\.location\.href\)/);
});

function walk(directory: string): string[] {
	const files: string[] = [];
	for (const entry of readdirSync(directory, { withFileTypes: true })) {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) files.push(...walk(path));
		else files.push(path);
	}
	return files;
}

function routeSegment(segment: string): string | null {
	// `[locale]` is the first URL segment of every localized page, and zh-TW (the
	// default locale) has no prefix: the routes below are the zh-TW URLs.
	if (segment === "[locale]") return null;
	if (segment.startsWith("@")) return null;
	if (/^\([^)]+\)$/.test(segment)) return null;
	const intercepted = segment.match(/^\(\.{1,3}\)(.+)$/);
	return intercepted?.[1] ?? segment;
}

function routeForFile(file: string): string {
	const directory = relative(APP_ROOT, file).split(sep).slice(0, -1);
	const segments = directory
		.map(routeSegment)
		.filter((segment): segment is string => Boolean(segment));
	return `/${segments.join("/")}`.replace(/\/+$/, "") || "/";
}

function escapeRegex(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function routeRegex(route: string): RegExp {
	if (route === "/") return /^\/$/;
	const segments = route.slice(1).split("/");
	const pattern = segments
		.map((segment) => {
			if (/^\[\[\.\.\.[^\]]+\]\]$/.test(segment)) return "(?:.+)?";
			if (/^\[\.\.\.[^\]]+\]$/.test(segment)) return ".+";
			if (/^\[[^\]]+\]$/.test(segment)) return "[^/]+";
			return escapeRegex(segment);
		})
		.join("/");
	return new RegExp(`^/${pattern}/?$`);
}

function navigationText(node: ts.Node | undefined): string | null {
	if (
		node &&
		(ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
	) {
		return node.text;
	}
	if (node && ts.isTemplateExpression(node)) {
		let value = node.head.text;
		for (const span of node.templateSpans) {
			// A substitution directly appended to a literal pathname (for example
			// `/login${condition ? ...}`) cannot be modelled as one route segment.
			// Query substitutions and slash-delimited dynamic segments can.
			if (
				!/[?#]/.test(value) &&
				!value.endsWith("/") &&
				!span.literal.text.startsWith("/") &&
				!span.literal.text.startsWith("?") &&
				!span.literal.text.startsWith("#")
			) {
				return null;
			}
			value += `route-contract-dynamic${span.literal.text}`;
		}
		return value;
	}
	return null;
}

function jsxLiteral(attribute: ts.JsxAttribute): string | null {
	if (!attribute.initializer) return null;
	if (ts.isStringLiteral(attribute.initializer)) {
		return attribute.initializer.text;
	}
	if (ts.isJsxExpression(attribute.initializer)) {
		return navigationText(attribute.initializer.expression);
	}
	return null;
}

function propertyName(node: ts.PropertyName): string | null {
	if (ts.isIdentifier(node) || ts.isStringLiteral(node)) return node.text;
	return null;
}

function callName(expression: ts.LeftHandSideExpression): string {
	if (ts.isIdentifier(expression)) return expression.text;
	if (ts.isPropertyAccessExpression(expression)) {
		return `${callName(expression.expression)}.${expression.name.text}`;
	}
	return "";
}

function isNavigationCall(name: string): boolean {
	return (
		name === "redirect" ||
		name === "permanentRedirect" ||
		name === "router.push" ||
		name === "router.replace" ||
		name === "window.location.assign" ||
		name === "window.location.replace" ||
		// the navigation layer's own exits (lib/intl, lib/navigation): they take the locale-free app path
		name === "commitDocumentNavigation" ||
		name === "useLocalizedHref" ||
		name === "localizeHref"
	);
}

function normalizeTarget(value: string): string | null {
	const candidate = value.trim();
	if (!candidate.startsWith("/") || candidate.startsWith("//")) return null;
	const path = candidate.split(/[?#]/, 1)[0] || "/";
	if (STATIC_ASSET_PREFIXES.some((prefix) => path.startsWith(prefix))) {
		return null;
	}
	if (extname(path)) return null;
	return path.replace(/\/+$/, "") || "/";
}

function collectNavigationInventory(): NavigationInventory {
	const targets: NavigationTarget[] = [];
	const unsafeTargets: NavigationTarget[] = [];
	const nativeInternalAnchors: NavigationTarget[] = [];
	for (const sourceRoot of SOURCE_ROOTS) {
		for (const file of walk(join(ROOT, sourceRoot))) {
			if (!SOURCE_EXTENSIONS.has(extname(file))) continue;
			// Co-located tests hand components made-up targets (`/en/faq`, `/x`); they are not
			// application navigation call sites.
			if (/\.(?:test|spec)\.[jt]sx?$/.test(file)) continue;
			const text = readFileSync(file, "utf8");
			const sourceFile = ts.createSourceFile(
				file,
				text,
				ts.ScriptTarget.Latest,
				true,
				file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
			);
			const reference = (value: string, node: ts.Node): NavigationTarget => ({
				path: value,
				source: relative(ROOT, file),
				line:
					sourceFile.getLineAndCharacterOfPosition(node.getStart()).line + 1,
			});
			const record = (
				value: string | null,
				node: ts.Node,
				options?: { nativeAnchor?: boolean },
			) => {
				if (!value) return;
				const candidate = value.trim();
				const hasControlCharacter = [...candidate].some((character) => {
					const code = character.charCodeAt(0);
					return code <= 0x1f || code === 0x7f;
				});
				const unsafeScheme = /^(?:javascript|data|vbscript):/i.test(candidate);
				if (
					hasControlCharacter ||
					unsafeScheme ||
					/^\/[\\/]/.test(candidate) ||
					candidate.includes("\\")
				) {
					unsafeTargets.push(reference(candidate, node));
					return;
				}
				const path = normalizeTarget(candidate);
				if (!path) return;
				const target = reference(path, node);
				targets.push(target);
				if (options?.nativeAnchor) nativeInternalAnchors.push(target);
			};
			const visit = (node: ts.Node) => {
				if (ts.isJsxAttribute(node)) {
					const name = node.name.getText(sourceFile);
					if (name === "href" || name === "action") {
						const parent = node.parent;
						const nativeAnchor =
							name === "href" &&
							(ts.isJsxOpeningElement(parent) ||
								ts.isJsxSelfClosingElement(parent)) &&
							parent.tagName.getText(sourceFile) === "a";
						record(jsxLiteral(node), node, { nativeAnchor });
					}
				} else if (ts.isPropertyAssignment(node)) {
					const name = propertyName(node.name);
					if (name === "href" || name === "redirect") {
						record(navigationText(node.initializer), node);
					}
				} else if (ts.isCallExpression(node)) {
					const name = callName(node.expression);
					if (isNavigationCall(name)) {
						record(navigationText(node.arguments[0]), node);
					}
				}
				ts.forEachChild(node, visit);
			};
			visit(sourceFile);
		}
	}
	return { targets, unsafeTargets, nativeInternalAnchors };
}

function routePatterns(): RegExp[] {
	const routeFiles = walk(APP_ROOT).filter((file) =>
		/(?:^|\/)(?:page|route)\.(?:ts|tsx)$/.test(file),
	);
	const routes = new Set(routeFiles.map(routeForFile));

	// HTTP-layer aliases are valid navigation targets too.
	const nextConfig = readFileSync(join(ROOT, "next.config.ts"), "utf8");
	for (const match of nextConfig.matchAll(/source:\s*"([^"]+)"/g)) {
		const source = normalizeTarget(match[1]);
		if (source && !source.includes(":")) routes.add(source);
	}
	return [...routes].map(routeRegex);
}

test("every static internal navigation target resolves to an app route", () => {
	const patterns = routePatterns();
	const invalid = collectNavigationInventory().targets.filter(
		(target) => !patterns.some((pattern) => pattern.test(target.path)),
	);
	assert.deepEqual(
		invalid,
		[],
		invalid
			.map(
				(target) =>
					`${target.source}:${target.line} points to missing route ${target.path}`,
			)
			.join("\n"),
	);
});

test("navigation literals cannot use protocol-relative or backslash URLs", () => {
	assert.deepEqual(collectNavigationInventory().unsafeTargets, []);
});

test("internal page links use the shared Link navigation layer", () => {
	assert.deepEqual(collectNavigationInventory().nativeInternalAnchors, []);
});

test("every HTTP redirect destination resolves to an app route", () => {
	const patterns = routePatterns();
	const nextConfig = readFileSync(join(ROOT, "next.config.ts"), "utf8");
	const invalid = [...nextConfig.matchAll(/destination:\s*"([^"]+)"/g)]
		.map((match) => normalizeTarget(match[1]))
		.filter((path): path is string => Boolean(path))
		.filter((path) => !patterns.some((pattern) => pattern.test(path)));
	assert.deepEqual(invalid, []);
});
