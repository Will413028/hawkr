import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	emptyDraft,
	validateDraft,
} from "../features/seller-center/products/adapters/productFormAdapter";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("seller SKU editors label list price as 原價 and discounted price as 售價", () => {
	const skuEditor = source(
		"features/seller-center/products/components/ProductFormSkuEditor.tsx",
	);
	const quickEditor = source(
		"features/seller-center/products/ProductEditDrawer.tsx",
	);

	assert.match(skuEditor, /label="原價 \(NT\$\)"/);
	assert.match(skuEditor, /`售價 \(NT\$\) - 省 \$\{discount\}%`/);
	assert.match(skuEditor, /未填售價時會以原價販售；填寫時需低於原價/);
	assert.doesNotMatch(skuEditor, /特價/);
	assert.match(quickEditor, /label="原價 \(NT\$\)"/);
});

test("price validation and import field labels use the same seller terminology", () => {
	const negativeOriginal = {
		...emptyDraft(),
		name: "測試商品",
		skus: [{ ...emptyDraft().skus[0], price: "-1" }],
	};
	const invalidSale = {
		...emptyDraft(),
		name: "測試商品",
		skus: [
			{
				...emptyDraft().skus[0],
				sku_name: "預設規格",
				price: "100",
				sale_price: "100",
			},
		],
	};

	assert.ok(
		validateDraft(negativeOriginal).includes("原價需大於 0"),
		"a non-positive list price is reported in 原價 terms",
	);
	assert.ok(
		validateDraft(invalidSale).includes("售價需小於原價"),
		"a discounted price at or above the list price is reported in 售價 terms",
	);
});
