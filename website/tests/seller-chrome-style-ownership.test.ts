import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss, { type ChildNode, type Root } from "postcss";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const UI_KIT = "styles/components/ui-kit.css";
const AUTH_LOGIN = "styles/features/auth-login.css";
const SELLER_PROFILE = "styles/features/seller-profile.css";
const SELLER_SHARE_CARD = "styles/features/seller-share-card.css";
const AUTH_NODES_BEFORE_SHARED_HINTS = 35;

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

function meaningfulNodes(root: Root): ChildNode[] {
	return (root.nodes ?? []).filter(
		(node): node is ChildNode => node.type !== "comment",
	);
}

function digest(value: unknown): string {
	return createHash("sha256").update(JSON.stringify(value)).digest("hex");
}

function semanticDigest(css: string): string {
	return digest(canonicalNode(postcss.parse(css)));
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
	const paths: string[] = [];
	for (const entry of readdirSync(directory, { withFileTypes: true })) {
		if (entry.name === "__tests__") continue;
		const absolute = join(directory, entry.name);
		if (entry.isDirectory()) {
			paths.push(...runtimeSourcePaths(absolute));
			continue;
		}
		if (!/\.(?:ts|tsx|js|jsx)$/.test(entry.name)) continue;
		if (/\.(?:test|spec)\./.test(entry.name)) continue;
		paths.push(absolute);
	}
	return paths;
}

function stylesheetImporters(stylesheet: string): string[] {
	const importStatement = `import "@/${stylesheet}";`;
	return ["app", "components", "features", "lib"]
		.flatMap((directory) => runtimeSourcePaths(join(ROOT, directory)))
		.filter((path) => readFileSync(path, "utf8").includes(importStatement))
		.map((path) => relative(ROOT, path))
		.sort();
}

test("seller and auth owner CSS preserve the exact reviewed semantic AST", () => {
	const uiKit = source(UI_KIT);
	const authLogin = source(AUTH_LOGIN);
	const profile = source(SELLER_PROFILE);
	const share = source(SELLER_SHARE_CARD);

	// Phone verification, account link and the GSI overlay left with Clerk
	// sign-in; tests/auth-login-style-ownership.test.ts carries the inventory.
	assert.equal(
		semanticDigest(authLogin),
		"4470ca3983d9da7899e7c2a01ddd4237c7781037cc5208cb29dc3a3ff5af796b",
	);
	// 2026-09-10 px→rem for the text-scale setting
	// 2026-09-19 店家簡介可展開：夾行從 -webkit-line-clamp 改為 max-height +
	// 浮動佔位塊，並新增 __desc-wrap / __desc-text / __desc-toggle 三條規則。
	assert.equal(
		semanticDigest(profile),
		"52911b95450e30ec9f9d54d1dc49423005bb02dc9a7fd983928c35c621e4636b",
	);
	assert.equal(
		semanticDigest(share),
		"42faf1c871d1b2f7bdae97031e0200a455c90d808a7b94f6b56c60a6e4739579",
	);
	assert.deepEqual(inventory(uiKit), {
		// 2026-09-10 元件健檢（reviewed）：見 auth-login-style-ownership 同日註記。
		// 2026-09-12 對比文字 token 新增 3 宣告；owner 順序與 selector 不變。
		// App input 對齊（reviewed）：3 個 input token 宣告，以及 bordered
		// dropdown trigger 的 1 個 rule / selector、5 個宣告；owner 順序不變。
		// 2026-10-27 刪除產地地圖（scope-cut 步驟 1）：整段 LOCATION PICKER MODAL
		// 與 origin-seller-preview 規則一併移除，-42 rules、-42 selectors、
		// -201 declarations、-1 media；其餘 owner 不變。
		styleRules: 242,
		selectors: 263,
		declarations: 1067,
		media: 16,
		keyframes: 5,
	});
	assert.deepEqual(inventory(authLogin), {
		styleRules: 33,
		selectors: 34,
		declarations: 124,
		media: 1,
		keyframes: 0,
	});
	assert.deepEqual(inventory(profile), {
		// 2026-09-19 店家簡介可展開（reviewed）：+10 rule／selector、+34 宣告，
		// 全部屬於新的 __desc-wrap／__desc-text／__desc-toggle 三個 owner，
		// 其中 2 條落在既有的手機 media query 內（media 數不變）。
		styleRules: 95,
		selectors: 95,
		declarations: 386,
		media: 2,
		keyframes: 0,
	});
	assert.deepEqual(inventory(share), {
		styleRules: 53,
		selectors: 53,
		declarations: 233,
		media: 0,
		keyframes: 0,
	});

	const uiRoot = postcss.parse(uiKit);
	const uiNodes = meaningfulNodes(uiRoot);
	const authNodes = meaningfulNodes(postcss.parse(authLogin));
	const sharedHintIndex = uiNodes.findIndex(
		(node) => node.type === "rule" && node.selector === ".hkui-login__hint",
	);
	assert.ok(sharedHintIndex >= 0, "shared auth hint must remain in ui-kit.css");
	const sharedHint = uiNodes[sharedHintIndex];
	const sharedErrorHint = uiNodes[sharedHintIndex + 1];
	assert.ok(sharedHint, "shared auth hint node must exist");
	assert.ok(sharedErrorHint, "shared auth error hint node must exist");
	assert.equal(
		sharedErrorHint.type === "rule" ? sharedErrorHint.selector : null,
		".hkui-login__hint--error",
		"the shared error hint must remain adjacent to its base rule",
	);

	const uiNodesWithAuth = [
		...uiNodes.slice(0, sharedHintIndex),
		...authNodes.slice(0, AUTH_NODES_BEFORE_SHARED_HINTS),
		sharedHint,
		sharedErrorHint,
		...authNodes.slice(AUTH_NODES_BEFORE_SHARED_HINTS),
		...uiNodes.slice(sharedHintIndex + 2),
	];
	const overlayIndex = uiNodesWithAuth.findIndex(
		(node) =>
			node.type === "rule" &&
			node.selector ===
				'body[data-hk-overlay-open="true"] .tsqd-open-btn-container',
	);
	const appBannerIndex = uiNodesWithAuth.findIndex(
		(node) => node.type === "rule" && node.selector === ".hkui-app-banner",
	);
	assert.ok(overlayIndex >= 0, "shared overlay anchor left ui-kit.css");
	assert.ok(
		appBannerIndex > overlayIndex,
		"app banner must remain after the shared overlay block",
	);

	const reconstructed = [
		"root",
		...uiNodesWithAuth.slice(0, overlayIndex).map(canonicalNode),
		...meaningfulNodes(postcss.parse(profile)).map(canonicalNode),
		...uiNodesWithAuth.slice(overlayIndex, appBannerIndex).map(canonicalNode),
		...meaningfulNodes(postcss.parse(share)).map(canonicalNode),
		...uiNodesWithAuth.slice(appBannerIndex).map(canonicalNode),
	];
	// 2026-09-10 元件健檢（reviewed）：ui-kit 的 dropdown / datepicker / calendar 觸控與
	// reduced-motion 調整、下拉色值 token 化、移除已無人用的 .hkui-app-modal* 規則。
	// Keep the reconstructed owner order and every foreground declaration exact.
	// 2026-09-10 px→rem for the text-scale setting
	// 2026-09-10 下拉色值 token 化（見 auth-login-style-ownership 同日註記）。
	// 2026-09-12 同步已檢視的 danger-text / field-copy 與 4 個 dropdown 文字色。
	// App input 的 3 個 token、bordered trigger 與 padding selector 排除條件
	// 同步進入重組基線；auth-login / seller-profile / seller-share-card AST 未變。
	// 2026-09-19 seller-profile 的店家簡介可展開：新增的 10 條規則落在原本
	// __desc 的位置，owner 順序與 ui-kit／auth-login／share-card 皆未變。
	// auth-login 移除手機驗證、帳號連結與 GSI 覆蓋層的規則（改用 Clerk 登入）；
	// ui-kit／seller-profile／share-card 與 owner 順序未變。
	// 2026-10-27 ui-kit 刪除 LOCATION PICKER MODAL 整段（產地地圖）；其餘 owner 與順序未變。
	assert.equal(
		digest(reconstructed),
		"67283011d8c4cc0b90124a301529ad7eece70f706dc0be2b18a62d07edfba957",
		"recombining every owner file at its original position must reproduce the reviewed ui-kit AST",
	);
	assert.deepEqual(
		{
			styleRules:
				inventory(uiKit).styleRules +
				inventory(authLogin).styleRules +
				inventory(profile).styleRules +
				inventory(share).styleRules,
			selectors:
				inventory(uiKit).selectors +
				inventory(authLogin).selectors +
				inventory(profile).selectors +
				inventory(share).selectors,
			declarations:
				inventory(uiKit).declarations +
				inventory(authLogin).declarations +
				inventory(profile).declarations +
				inventory(share).declarations,
			media:
				inventory(uiKit).media +
				inventory(authLogin).media +
				inventory(profile).media +
				inventory(share).media,
			keyframes:
				inventory(uiKit).keyframes +
				inventory(authLogin).keyframes +
				inventory(profile).keyframes +
				inventory(share).keyframes,
		},
		{
			// 2026-09-19 四檔合計跟著 seller-profile 走：+10 rule／selector、
			// +34 宣告，與上面單檔的差額相同（唯一的來源就是店家簡介那一刀）。
			// auth-login 的刪除：-89 rule、-93 selector、-318 宣告，同上面單檔的差額。
			// 2026-10-27 產地地圖刪除：同上面 ui-kit 單檔的差額。
			styleRules: 423,
			selectors: 445,
			declarations: 1810,
			media: 19,
			keyframes: 5,
		},
	);
});

test("only the rendering owners load seller chrome styles", () => {
	assert.deepEqual(stylesheetImporters(SELLER_PROFILE), [
		"features/seller/components/SellerProfileV2.tsx",
		"features/seller/components/SellerShareButton.tsx",
	]);
	assert.deepEqual(stylesheetImporters(SELLER_SHARE_CARD), [
		"features/seller/components/SellerBusinessCard.tsx",
		"features/seller/components/SellerShareButton.tsx",
	]);

	const rootLayout = source("components/shared/RootDocument.tsx");
	const globals = source("styles/globals.css");
	assert.match(rootLayout, /import "@\/styles\/components\/ui-kit\.css";/);
	assert.doesNotMatch(
		`${rootLayout}\n${globals}`,
		/styles\/features\/seller-(?:profile|share-card)\.css/,
	);

	const uiKit = source(UI_KIT);
	assert.doesNotMatch(uiKit, /\.hkui-seller-profile(?:\b|__)/);
	assert.doesNotMatch(uiKit, /\.seller-(?:share|business-card)(?:\b|__|-)/);
	assert.match(uiKit, /:root\s*\{[\s\S]*--hkui-space-0:/);
	assert.match(
		uiKit,
		/body\[data-hk-overlay-open="true"\] \.tsqd-open-btn-container/,
	);
	assert.match(uiKit, /\[data-hk-overlay-root\]\s*\{/);
	assert.match(uiKit, /\.hk-heroui-brand-scope\s*\{/);
	assert.doesNotMatch(
		`${source(SELLER_PROFILE)}\n${source(SELLER_SHARE_CARD)}`,
		/\[data-hk-overlay-root\]|\.hk-heroui-brand-scope/,
	);
});

test("public storefront and seller preview keep the server profile owner", () => {
	const profileOwner = source("features/seller/components/SellerProfileV2.tsx");
	assert.doesNotMatch(profileOwner, /^"use client";/m);
	assert.match(
		profileOwner,
		/import "@\/styles\/features\/seller-profile\.css";/,
	);

	for (const layoutPath of [
		"app/[locale]/(public)/sellers/[id]/(storefront)/layout.tsx",
		"app/[locale]/(seller)/seller-center/preview/[id]/layout.tsx",
	]) {
		assert.equal(statSync(join(ROOT, layoutPath)).isFile(), true);
		const layout = source(layoutPath);
		assert.doesNotMatch(layout, /^"use client";/m, layoutPath);
		assert.match(
			layout,
			/import SellerProfileV2 from "@\/features\/seller\/components\/SellerProfileV2";/,
			layoutPath,
		);
		assert.match(layout, /<SellerProfileV2\b/, layoutPath);
	}
});
