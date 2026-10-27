import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript-compiler-api";

const root = fileURLToPath(new URL("..", import.meta.url));
const sourceRoots = ["app", "components", "features"];

function tsxFiles(relativeDirectory: string): string[] {
	const directory = path.join(root, relativeDirectory);
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const relativePath = path.join(relativeDirectory, entry.name);
		if (entry.isDirectory()) return tsxFiles(relativePath);
		return entry.isFile() && entry.name.endsWith(".tsx") ? [relativePath] : [];
	});
}

function jsxAttributeNames(
	node: ts.JsxOpeningElement | ts.JsxSelfClosingElement,
	sourceFile: ts.SourceFile,
): string[] {
	return node.attributes.properties
		.filter(ts.isJsxAttribute)
		.map((attribute) => attribute.name.getText(sourceFile));
}

test("every shared Button loading state has an explicit action label", () => {
	const missing: string[] = [];
	let loadingButtons = 0;

	for (const relativePath of sourceRoots.flatMap(tsxFiles)) {
		const source = readFileSync(path.join(root, relativePath), "utf8");
		const sourceFile = ts.createSourceFile(
			relativePath,
			source,
			ts.ScriptTarget.Latest,
			true,
			ts.ScriptKind.TSX,
		);

		const visit = (node: ts.Node) => {
			const opening = ts.isJsxElement(node)
				? node.openingElement
				: ts.isJsxSelfClosingElement(node)
					? node
					: null;
			if (opening?.tagName.getText(sourceFile) === "Button") {
				const attributes = jsxAttributeNames(opening, sourceFile);
				if (attributes.includes("isLoading")) {
					loadingButtons += 1;
					if (!attributes.includes("loadingLabel")) {
						const { line } = sourceFile.getLineAndCharacterOfPosition(
							opening.getStart(sourceFile),
						);
						missing.push(`${relativePath}:${line + 1}`);
					}
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(sourceFile);
	}

	assert.ok(loadingButtons > 0, "expected shared pending buttons to be found");
	assert.deepEqual(missing, []);
});

test("native pending button text is reserved for the semantic upload tile", () => {
	const remaining: string[] = [];
	const pendingText =
		/載入中|儲存中|送出中|處理中|查詢中|建立中|開啟中|套用中|美化中|驗證中|登入中|上傳中|更新中|刪除中|傳送中|付款處理中|重新傳送中|比對中/;

	for (const relativePath of sourceRoots.flatMap(tsxFiles)) {
		if (relativePath === "components/ui/ImageUploader.tsx") continue;
		const source = readFileSync(path.join(root, relativePath), "utf8");
		const sourceFile = ts.createSourceFile(
			relativePath,
			source,
			ts.ScriptTarget.Latest,
			true,
			ts.ScriptKind.TSX,
		);

		const visit = (node: ts.Node) => {
			if (
				ts.isJsxElement(node) &&
				node.openingElement.tagName.getText(sourceFile) === "button"
			) {
				const markup = node.getText(sourceFile);
				if (
					pendingText.test(markup) &&
					(markup.includes("?") || /disabled\s*=/.test(markup))
				) {
					const { line } = sourceFile.getLineAndCharacterOfPosition(
						node.getStart(sourceFile),
					);
					remaining.push(`${relativePath}:${line + 1}`);
				}
			}
			ts.forEachChild(node, visit);
		};
		visit(sourceFile);
	}

	assert.deepEqual(remaining, []);
});

test("brand, Truck, and Upload indicators all stop under reduced motion", () => {
	const button = readFileSync(
		path.join(root, "components/ui/Button.tsx"),
		"utf8",
	);
	const brandCss = readFileSync(
		path.join(root, "styles/components/decorations-overlays.css"),
		"utf8",
	);
	const truck = readFileSync(
		path.join(root, "components/shared/TruckSpinner.tsx"),
		"utf8",
	);
	const sellerCenterCss = readFileSync(
		path.join(root, "styles/features/seller-center.css"),
		"utf8",
	);
	const upload = readFileSync(
		path.join(root, "components/shared/UploadLoadingIndicator.tsx"),
		"utf8",
	);

	assert.match(button, /BrandLogoLoader/);
	assert.doesNotMatch(button, /animate-spin/);
	assert.match(brandCss, /@media \(prefers-reduced-motion: reduce\)/);
	assert.match(brandCss, /\.il-blade\s*\{\s*animation:\s*none;/);
	assert.match(truck, /motion-reduce:animate-none/);
	assert.match(
		sellerCenterCss,
		/\.seller-orders__action--primary,\s*\.seller-orders__action--processing\s*\{[^}]*width:\s*7\.5rem;[^}]*flex:\s*0 0 7\.5rem;/,
	);
	assert.match(
		sellerCenterCss,
		/\.seller-orders__action--processing,\s*\.seller-orders__action--processing:hover\s*\{[^}]*background:\s*var\(--hkui-default-300\);/,
	);
	assert.match(upload, /name="upload"/);
	assert.match(upload, /motion-reduce:animate-none/);
});

test("seller order action column contains the stable shipment action group", () => {
	const sellerCenterCss = readFileSync(
		path.join(root, "styles/features/seller-center.css"),
		"utf8",
	);

	assert.match(
		sellerCenterCss,
		/\.seller-orders__table--orders\s*\{[^}]*min-width:\s*1036px;/,
	);
	assert.match(
		sellerCenterCss,
		/\.seller-orders__table--orders thead th:nth-child\(9\)\s*\{[^}]*width:\s*196px;/,
	);
});

test("restock reports branded pending state without a pre-request animation delay", () => {
	const restock = readFileSync(
		path.join(root, "components/shared/RestockNotifyButton.tsx"),
		"utf8",
	);

	assert.match(restock, /setPhase\("subscribing"\);/);
	assert.match(restock, /isLoading=\{loading\}/);
	assert.doesNotMatch(restock, /RING_MS|phase === "ringing"/);
});

test("product-import mutations keep the initiating button identity", () => {
	const jobDetail = readFileSync(
		path.join(
			root,
			"features/seller-center/products-import/pages/JobDetailPage.tsx",
		),
		"utf8",
	);
	const itemForm = readFileSync(
		path.join(
			root,
			"features/seller-center/products-import/components/ImportItemEditForm.tsx",
		),
		"utf8",
	);

	assert.match(
		jobDetail,
		/pendingAction\s*===\s*`retry-item:\$\{it\.importItemId\}`/,
	);
	assert.match(
		jobDetail,
		/pendingAction\s*===\s*`restore-item:\$\{it\.importItemId\}`/,
	);
	assert.match(
		jobDetail,
		/pendingAction\s*===\s*`remove-item:\$\{it\.importItemId\}`/,
	);
	assert.match(jobDetail, /pendingAction === "restore-selected"/);
	assert.match(jobDetail, /pendingActionRef\.current/);
	assert.match(itemForm, /isLoading=\{removing\}/);
	assert.match(itemForm, /loadingLabel="移除匯入商品中…"/);
});

test("navbar logout keeps its initiating menu action visible until settlement", () => {
	const navbar = readFileSync(
		path.join(root, "components/shared/Navbar.tsx"),
		"utf8",
	);
	const roleAwareNavbar = readFileSync(
		path.join(root, "components/shared/RoleAwareNavbar.tsx"),
		"utf8",
	);
	const sellerNavbar = readFileSync(
		path.join(root, "components/shared/SellerNavbar.tsx"),
		"utf8",
	);
	const sellerNavbarLoader = readFileSync(
		path.join(root, "components/shared/SellerNavbarLoader.tsx"),
		"utf8",
	);
	const combined = `${navbar}\n${roleAwareNavbar}\n${sellerNavbar}\n${sellerNavbarLoader}`;

	assert.equal(combined.match(/loadingLabel="登出中…"/g)?.length, 5);
	assert.equal(combined.match(/isLoading=\{isSigningOut\}/g)?.length, 5);
	assert.equal(
		combined.match(/signOutPendingRef\.current = true;/g)?.length,
		5,
	);
	assert.equal(combined.match(/disabled=\{isSigningOut\}/g)?.length, 5);
	assert.match(combined, /aria-disabled=\{isSigningOut \|\| undefined\}/);
	assert.equal(
		combined.match(
			/finally \{\s*signOutPendingRef\.current = false;\s*setIsSigningOut\(false\);\s*\}/g,
		)?.length,
		5,
	);
	assert.doesNotMatch(
		combined,
		/setOpen\(false\);\s*(?:try\s*\{\s*)?await (?:onSignOut|signOut)\(/,
	);
});

test("optimistic toggle hooks reject same-frame duplicate requests", () => {
	const social = readFileSync(
		path.join(root, "lib/hooks/useSocial.ts"),
		"utf8",
	);
	const subscription = readFileSync(
		path.join(root, "lib/hooks/useSellerSubscription.ts"),
		"utf8",
	);
	const restock = readFileSync(
		path.join(root, "components/shared/RestockNotifyButton.tsx"),
		"utf8",
	);

	assert.match(social, /const togglePendingRef = React\.useRef\(false\)/);
	assert.match(social, /if \(!postId \|\| togglePendingRef\.current\)/);
	assert.match(subscription, /if \(togglePendingRef\.current\)/);
	assert.match(restock, /if \(pendingRef\.current\) return;/);
});
