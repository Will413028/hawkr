import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const source = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/CropCatalogSearch.tsx",
	),
	"utf-8",
);
const publicTreeHookSource = readFileSync(
	join(import.meta.dirname, "../lib/hooks/usePublicCategoryTree.ts"),
	"utf-8",
);
const resolutionHookSource = readFileSync(
	join(import.meta.dirname, "../lib/hooks/useCategoryResolution.ts"),
	"utf-8",
);
const categorySelectorSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller/components/CategorySelector.tsx",
	),
	"utf-8",
);
const outcomeSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/categoryResolutionOutcome.ts",
	),
	"utf-8",
);
const pricebookRepoSource = readFileSync(
	join(import.meta.dirname, "../lib/pricebook/repo.ts"),
	"utf-8",
);

describe("CropCatalogSearch inline structural contract", () => {
	it("keeps the taxonomy visible and removes the dialog/sheet launcher", () => {
		assert.match(source, /className="seller-price-lookup__inline-catalog"/);
		assert.match(source, /<fieldset/);
		assert.match(source, /<legend className="sr-only">商品分類<\/legend>/);
		assert.doesNotMatch(source, /createPortal|<Sheet|role="dialog"/);
		assert.doesNotMatch(source, /aria-haspopup="dialog"/);
	});

	it("reuses the read-only public three-level CategorySelector", () => {
		assert.match(source, /<CategorySelector/);
		assert.match(source, /allowCreate=\{false\}/);
		assert.match(source, /treeSource=\{categoryTreeSource\}/);
		assert.match(source, /usePublicCategoryTree\(\)/);
		assert.doesNotMatch(source, /useCategoryTree/);
		assert.match(publicTreeHookSource, /fetchPublicCategoryTree/);
		assert.match(categorySelectorSource, /localPath\[0\] === l1Id/);
		assert.match(categorySelectorSource, /\[localPath, tree, value\]/);
	});

	it("resolves the selected L3 category id against the backend, never a name search", () => {
		assert.match(
			source,
			/categoryId != null \? String\(categoryId\) : undefined/,
		);
		assert.match(source, /enabled: categoryContext != null/);
		assert.match(resolutionHookSource, /fetchCategoryResolution/);
		assert.match(pricebookRepoSource, /query: \{ category_id: categoryId \}/);
		assert.match(resolutionHookSource, /options: \{ enabled\?: boolean \}/);
		assert.match(resolutionHookSource, /options\.enabled \?\? true/);
		assert.doesNotMatch(source, /crop-catalog\/search/);
	});

	it("finishes the flow at L3 and applies only a resolved authoritative match", () => {
		assert.doesNotMatch(source, /label="行情品相"/);
		assert.doesNotMatch(source, /<Dropdown/);
		assert.match(source, /resolveCategoryResolutionOutcome/);
		assert.match(source, /outcome\.kind !== "resolved"/);
		assert.match(source, /applyOutcome\(outcome\)/);
		assert.match(source, /activeUserCategoryRef/);
		assert.match(source, /activeUserCategory !== categoryId/);
		assert.match(
			source,
			/onResolve\(resolved\.selection, resolved\.label, categoryId\)/,
		);
		assert.doesNotMatch(source, /matches\[0\]/);
	});

	it("never renders a secondary catalog-identity confirmation field", () => {
		assert.doesNotMatch(source, /確認行情對應|找到多筆對應/);
		assert.doesNotMatch(source, /catalog-ambiguity/);
		assert.doesNotMatch(source, /visibleMatches\.map/);
		assert.match(source, /系統已停止套用以避免顯示錯誤價格/);
	});

	it("clears the previous selection as soon as any category level changes", () => {
		assert.match(
			source,
			/onCategoryChange: \(categoryId: number \| undefined\)/,
		);
		assert.match(source, /onCategoryChange\(nextCategoryId\)/);
	});

	it("never derives groupCode from a category id or display name", () => {
		assert.doesNotMatch(
			source,
			/groupCode:\s*categoryId|groupCode:\s*categoryContext/,
		);
		// The selection is built purely from the backend response field
		// (groupCode), never from the categoryId or a category name.
		assert.match(outcomeSource, /groupCode: data\.groupCode/);
	});

	it("focuses the first inline listbox when an empty-state CTA points here", () => {
		assert.match(source, /scrollIntoView/);
		assert.match(source, /querySelector<HTMLButtonElement>/);
		assert.match(source, /\[aria-haspopup="listbox"\]/);
		assert.match(source, /focus\(\{ preventScroll: true \}\)/);
	});

	it("keeps catalog errors local and renders no raw SVG", () => {
		assert.match(source, /seller-price-lookup__catalog-error/);
		assert.match(source, /role="alert"/);
		assert.match(source, /onClick=\{resolution\.retry\}/);
		assert.doesNotMatch(source, /<svg\b/);
	});
});
