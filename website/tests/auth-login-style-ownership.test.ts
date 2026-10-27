import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss, { type ChildNode, type Root } from "postcss";
import ts from "typescript-compiler-api";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const UI_KIT = "styles/components/ui-kit.css";
const AUTH_LOGIN = "styles/features/auth-login.css";
const RUNTIME_ROOTS = ["app", "components", "features", "lib"] as const;
const ALLOWED_ROOT_LOGIN_SELECTORS = [
	".hkui-login__hint",
	".hkui-login__hint--error",
] as const;

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
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
	return createHash("sha256")
		.update(JSON.stringify(canonicalNode(postcss.parse(css))))
		.digest("hex");
}

function inventory(css: string) {
	const root = postcss.parse(css);
	let styleRules = 0;
	let selectors = 0;
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

function stylesheetImporters(stylesheet: string): string[] {
	const importStatement = `import "@/${stylesheet}";`;
	return RUNTIME_ROOTS.flatMap((directory) =>
		runtimeSourcePaths(join(ROOT, directory)),
	)
		.filter((path) => readFileSync(path, "utf8").includes(importStatement))
		.map((path) => relative(ROOT, path))
		.sort();
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

const AUTH_CLASS_PATTERN =
	/(?<![A-Za-z0-9_-])hkui-(?:login(?:__[A-Za-z0-9_-]*)?|phone-field(?:__[A-Za-z0-9_-]*)?)(?![A-Za-z0-9_-])/g;

function authClassTokens(path: string): string[] {
	const relativePath = relative(ROOT, path);
	const file = ts.createSourceFile(
		relativePath,
		readFileSync(path, "utf8"),
		ts.ScriptTarget.Latest,
		true,
		scriptKind(path),
	);
	const tokens = new Set<string>();
	const inspect = (value: string) => {
		for (const match of value.matchAll(AUTH_CLASS_PATTERN))
			tokens.add(match[0]);
	};
	const visit = (node: ts.Node) => {
		if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
			inspect(node.text);
		} else if (ts.isTemplateExpression(node)) {
			inspect(node.head.text);
			for (const span of node.templateSpans) inspect(span.literal.text);
		}
		ts.forEachChild(node, visit);
	};
	visit(file);
	return [...tokens].sort();
}

test("auth login owner preserves the exact unlayered semantic CSS", () => {
	const uiKit = source(UI_KIT);
	const authLogin = source(AUTH_LOGIN);

	// Reviewed update: the phone-verification, account-link and GSI-overlay
	// rules were removed with those screens (Clerk sign-in); the login panel,
	// OAuth buttons, role tabs and cascade are intact.
	assert.equal(
		semanticDigest(authLogin),
		"4470ca3983d9da7899e7c2a01ddd4237c7781037cc5208cb29dc3a3ff5af796b",
	);
	assert.deepEqual(inventory(authLogin), {
		styleRules: 33,
		selectors: 34,
		declarations: 124,
		media: 1,
		keyframes: 0,
	});
	// 2026-09-10 元件健檢（reviewed）：dropdown / datepicker / calendar 觸控尺寸
	// 與 reduced-motion、下拉色值 token 化，並移除 AppDownloadModal 改建到 Dialog 後
	// 已無人使用的 .hkui-app-modal* 規則。
	// 2026-09-10 px→rem for the text-scale setting
	// 2026-09-10 下拉夜間色改由 --page-dropdown-* token 提供（seller-center.css 的
	// `[data-theme="dark"] .hkui-dropdown__*` patch 已刪除）；light 值逐條沿用原本
	// 畫出來的顏色，所以日間渲染不變。
	// 2026-09-12 元件庫對比檢查（reviewed）：新增 light/dark danger-text 與
	// field-copy 共 3 個宣告；4 個 dropdown 輔助文字色改走 field-copy。
	// Selector、media、keyframes 與 auth/seller stylesheet ownership 不變。
	// App input 對齊（reviewed）：新增 2 個 root input token、1 個 dark surface
	// 宣告，以及 bordered dropdown trigger 的 5 個宣告；既有 padding selector
	// 排除 bordered。合計 +1 rule、+1 selector、+8 declarations；auth CSS、
	// stylesheet owner、media 與 keyframes 均未改動。
	assert.equal(
		semanticDigest(uiKit),
		"77e17f286989ff8552cf479c122766cc9df88770a19892212e7159f8d1605af2",
	);
	assert.deepEqual(inventory(uiKit), {
		// 2026-10-27 刪除產地地圖（scope-cut 步驟 1）：整段 LOCATION PICKER MODAL
		// 與 origin-seller-preview 規則一併移除，-42 rules、-42 selectors、
		// -201 declarations、-1 media；其餘 owner 不變。
		styleRules: 242,
		selectors: 263,
		declarations: 1067,
		media: 16,
		keyframes: 5,
	});

	const authRoot = postcss.parse(authLogin);
	const layers: string[] = [];
	authRoot.walkAtRules("layer", (atRule) => layers.push(atRule.params));
	assert.deepEqual(
		layers,
		[],
		"auth overrides must stay unlayered so Button utilities and HeroUI OTP styles keep the established cascade",
	);

	const uiLoginSelectors: string[] = [];
	postcss.parse(uiKit).walkRules((rule) => {
		for (const selector of rule.selectors) {
			if (/\.hkui-(?:login|phone-field)(?:\b|__)/.test(selector)) {
				uiLoginSelectors.push(selector);
			}
		}
	});
	assert.deepEqual(uiLoginSelectors, ALLOWED_ROOT_LOGIN_SELECTORS);

	const authSelectors: string[] = [];
	authRoot.walkRules((rule) => authSelectors.push(...rule.selectors));
	assert.equal(authSelectors[0], ".hkui-login");
	assert.equal(authSelectors.includes(".hkui-login__hint"), false);
	assert.equal(authSelectors.includes(".hkui-login__hint--error"), false);
});

test("LoginShell is the only synchronous auth stylesheet owner", () => {
	assert.deepEqual(stylesheetImporters(AUTH_LOGIN), [
		"features/auth/components/login/LoginShell.tsx",
	]);

	const loginShell = source("features/auth/components/login/LoginShell.tsx");
	assert.match(loginShell, /^import "@\/styles\/features\/auth-login\.css";/);
	assert.match(loginShell, /<main className="hkui-login">/);

	for (const screen of ["features/auth/screens/login/LoginPage.tsx"]) {
		const contents = source(screen);
		assert.match(
			contents,
			/import LoginShell from "@\/features\/auth\/components\/login\/LoginShell";/,
			screen,
		);
		assert.match(contents, /<LoginShell\b/, screen);
	}

	assert.match(
		source("features/auth/screens/login/LoginPage.tsx"),
		/import OAuthLoginButton from "@\/features\/auth\/components\/login\/OAuthLoginButton";/,
	);

	const rootCssOwners = `${source("components/shared/RootDocument.tsx")}\n${source("styles/globals.css")}`;
	assert.match(rootCssOwners, /styles\/components\/ui-kit\.css/);
	assert.doesNotMatch(rootCssOwners, /styles\/features\/auth-login\.css/);
});

test("shared hints and the parent layout hook remain available outside auth", () => {
	const uiRoot = postcss.parse(source(UI_KIT));
	const hintRules = (uiRoot.nodes ?? []).filter(
		(node): node is ChildNode =>
			node.type === "rule" &&
			ALLOWED_ROOT_LOGIN_SELECTORS.includes(
				node.selector as (typeof ALLOWED_ROOT_LOGIN_SELECTORS)[number],
			),
	);
	assert.equal(
		createHash("sha256")
			.update(
				JSON.stringify([
					"root",
					...hintRules.map((rule) => canonicalNode(rule)),
				]),
			)
			.digest("hex"),
		"8c84bb059f052253b51fcb6592702eb2b23ace229662d9a85e9a4a0e7168fb79",
	);
	assert.match(
		source("styles/foundation/02-reset-layout.css"),
		/\.page-content:has\(\.hkui-login\)\s*\{\s*padding-top:\s*0;\s*\}/,
	);

	const nonAuthConsumers: string[] = [];
	const offenders: string[] = [];
	for (const absolute of RUNTIME_ROOTS.flatMap((directory) =>
		runtimeSourcePaths(join(ROOT, directory)),
	)) {
		const relativePath = relative(ROOT, absolute);
		if (relativePath.startsWith("features/auth/")) continue;
		const tokens = authClassTokens(absolute);
		if (tokens.length === 0) continue;
		nonAuthConsumers.push(relativePath);
		for (const token of tokens) {
			if (
				token !== "hkui-login__hint" &&
				!token.startsWith("hkui-login__hint--")
			) {
				offenders.push(`${relativePath}: ${token}`);
			}
		}
	}

	assert.deepEqual(offenders, []);
	assert.deepEqual(nonAuthConsumers.sort(), [
		"features/account/screens/addresses/AddressesClient.tsx",
		"features/cart/CartClient.tsx",
		"features/cart/components/StorePickupPickerSheet.tsx",
		"features/checkout/screens/CheckoutClient.tsx",
	]);
});
