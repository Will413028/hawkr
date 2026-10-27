import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss, { type ChildNode, type Root } from "postcss";
import ts from "typescript-compiler-api";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const CARD_STYLES = "styles/components/cards-shop.css";
const RUNTIME_ROOTS = ["app", "components", "features", "lib"] as const;

const RETIRED_CLASS_NAMES = [
	"store-card",
	"store-card__cover",
	"store-card__body",
	"store-card__avatar",
	"store-card__name",
	"store-card__meta",
	"store-card__location",
	"store-card__rating",
	"store-card__desc",
	"store-card__footer",
	"store-card__subscribers",
	"store-card__enter-btn",
	"seller-hero__avatar",
	"seller-hero__profile",
	"seller-hero__name",
	"seller-hero__cover",
	"seller-hero__stats",
	"seller-hero__stat",
	"seller-hero__tabs",
	"seller-hero__tab",
	"seller-hero__tab--active",
	"seller-hero__stat-divider",
] as const;

const LIVE_SELECTOR_ORDER = [
	".glass-card",
	'[data-theme="dark"] .glass-card',
	'[data-theme="dark"] .cart-shell-page',
	'[data-theme="dark"] .cart-shell-page::before',
	".glass-card--sm",
	".glass-card--sm::before",
	".glass-card--lg",
	".glass-card--solid",
	'[data-theme="dark"] .glass-card--solid',
] as const;

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

function runtimeSourcePaths(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		if (entry.name === "__tests__") return [];
		const absolute = join(directory, entry.name);
		if (entry.isDirectory()) return runtimeSourcePaths(absolute);
		if (!entry.isFile() || !/\.[jt]sx?$/.test(entry.name)) return [];
		if (
			/\.(?:test|spec)\.[jt]sx?$/.test(entry.name) ||
			entry.name.endsWith(".d.ts")
		) {
			return [];
		}
		return [absolute];
	});
}

function scriptKind(path: string): ts.ScriptKind {
	switch (extname(path)) {
		case ".tsx":
			return ts.ScriptKind.TSX;
		case ".jsx":
			return ts.ScriptKind.JSX;
		case ".js":
			return ts.ScriptKind.JS;
		default:
			return ts.ScriptKind.TS;
	}
}

function exactTokenPattern(token: string): RegExp {
	const escaped = token.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	return new RegExp(`(?<![A-Za-z0-9_-])${escaped}(?![A-Za-z0-9_-])`);
}

const RETIRED_LITERAL_PATTERNS = [
	...RETIRED_CLASS_NAMES.map((name) => ({
		name,
		pattern: exactTokenPattern(name),
	})),
	// Catch template/concatenation families such as `store-card__${variant}`.
	// The left boundary deliberately excludes active names like
	// `prd-store-card__media`.
	{
		name: "store-card__*",
		pattern: /(?<![A-Za-z0-9_-])store-card__/,
	},
	{
		name: "seller-hero__*",
		pattern: /(?<![A-Za-z0-9_-])seller-hero__/,
	},
	{ name: "bounceUp", pattern: exactTokenPattern("bounceUp") },
];

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
	return createHash("sha256")
		.update(JSON.stringify(canonicalNode(postcss.parse(css))))
		.digest("hex");
}

test("retired card hooks do not return in production runtime literals", () => {
	const offenders: string[] = [];

	for (const absolute of RUNTIME_ROOTS.flatMap((directory) =>
		runtimeSourcePaths(join(ROOT, directory)),
	)) {
		const relativePath = relative(ROOT, absolute);
		const contents = readFileSync(absolute, "utf8");
		const sourceFile = ts.createSourceFile(
			relativePath,
			contents,
			ts.ScriptTarget.Latest,
			true,
			scriptKind(absolute),
		);

		const inspectLiteral = (value: string, node: ts.Node) => {
			for (const { name, pattern } of RETIRED_LITERAL_PATTERNS) {
				if (!pattern.test(value)) continue;
				const { line } = sourceFile.getLineAndCharacterOfPosition(
					node.getStart(sourceFile),
				);
				offenders.push(`${relativePath}:${line + 1} ${name}`);
			}
		};

		const visit = (node: ts.Node) => {
			if (
				ts.isStringLiteral(node) ||
				ts.isNoSubstitutionTemplateLiteral(node)
			) {
				inspectLiteral(node.text, node);
			} else if (ts.isTemplateExpression(node)) {
				inspectLiteral(node.head.text, node.head);
				for (const span of node.templateSpans) {
					inspectLiteral(span.literal.text, span.literal);
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(sourceFile);
	}

	assert.deepEqual(offenders, []);
});

test("cards-shop CSS contains no retired selectors or animation", () => {
	const root = postcss.parse(source(CARD_STYLES));
	const retiredSelectors: string[] = [];
	root.walkRules((rule) => {
		for (const name of RETIRED_CLASS_NAMES) {
			if (exactTokenPattern(name).test(rule.selector)) {
				retiredSelectors.push(rule.selector);
			}
		}
	});

	assert.deepEqual(retiredSelectors, []);
	root.walkAtRules((atRule) => {
		assert.notEqual(atRule.params, "bounceUp");
	});
	root.walkDecls((declaration) => {
		assert.doesNotMatch(declaration.value, exactTokenPattern("bounceUp"));
	});
});

test("live glass and cart rules preserve their complete semantic contract", () => {
	const css = source(CARD_STYLES);
	const root = postcss.parse(css);
	const selectors: string[] = [];
	let declarations = 0;
	let media = 0;
	let keyframes = 0;

	root.walkRules((rule) => {
		if (
			rule.parent?.type === "atrule" &&
			/(?:^|-)keyframes$/i.test(rule.parent.name)
		) {
			return;
		}
		selectors.push(...rule.selectors);
	});
	root.walkDecls(() => {
		declarations += 1;
	});
	root.walkAtRules((atRule) => {
		if (atRule.name === "media") media += 1;
		if (/(?:^|-)keyframes$/i.test(atRule.name)) keyframes += 1;
	});

	assert.deepEqual(selectors, LIVE_SELECTOR_ORDER);
	assert.deepEqual(
		{
			styleRules: selectors.length,
			selectors: selectors.length,
			declarations,
			media,
			keyframes,
		},
		{ styleRules: 9, selectors: 9, declarations: 17, media: 0, keyframes: 0 },
	);
	assert.equal(
		semanticDigest(css),
		// Baseline live rules before the retirement. Intentional restyling must
		// review the whole ordered rule contract before updating this digest.
		"69e7d42ef8589cadd0afbc104e8cc8602597323522888cc4f9bb04a9d79c45a9",
	);
});
