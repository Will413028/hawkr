import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const products = source("features/seller-center/products/ProductsPage.tsx");
const inventory = source("features/seller-center/inventory/InventoryPage.tsx");

test("商品與庫存保留桌面表格，手機改用單一展開 disclosure 清單", () => {
	for (const page of [products, inventory]) {
		assert.match(page, /seller-mobile-disclosure-desktop/);
		assert.match(page, /<SellerMobileDisclosureList/);
		assert.match(page, /getKey=\{[a-zA-Z]+DisclosureKey\}/);
		assert.match(page, /getSummary=\{[a-zA-Z]+DisclosureSummary\}/);
		assert.match(page, /renderDetails=\{renderMobile[A-Za-z]+Details\}/);
		assert.match(page, /resetKey=\{`\$\{/);
	}
});

test("商品摘要只放正式圖片、名稱、價格與唯讀狀態，操作留在展開區", () => {
	assert.match(
		products,
		/function productDisclosureSummary[\s\S]*leading:[\s\S]*ossImageUrl\([\s\S]*title: name,[\s\S]*meta: priceSummaryText\(product\),[\s\S]*status:[\s\S]*<ShelfStatusControl/s,
	);
	assert.match(products, /className="seller-mobile-disclosure__actions"/);
	assert.match(products, /className="seller-mobile-disclosure__detail-row"/);

	const detailsStart = products.indexOf("const renderMobileProductDetails");
	const detailsEnd = products.indexOf("\n\treturn (", detailsStart);
	const details = products.slice(detailsStart, detailsEnd);
	assert.match(details, /delegated && canPublish/);
	assert.match(details, /aria-label=\{`選取 \$\{productName\}`\}/);
	assert.match(details, /checked=\{selectedIds\.includes\(id\)\}/);
	assert.doesNotMatch(
		products.slice(
			products.indexOf("function productDisclosureSummary"),
			products.indexOf("function ProductNameCell"),
		),
		/type="checkbox"/,
		"批次勾選不可嵌入整列 summary button",
	);
});

test("庫存摘要只放商品、當前庫存與既有預警判定，展開顯示預測與調整入口", () => {
	assert.match(
		inventory,
		/function inventoryDisclosureSummary[\s\S]*title: name,[\s\S]*meta: `\$\{item\.row\.stock\} 件`[\s\S]*item\.row\.isLow[\s\S]*庫存不足/s,
	);
	assert.match(inventory, /建議安全庫存/);
	assert.match(inventory, /<Sparkline values=\{item\.weekly\} \/>/);
	assert.match(inventory, /item\.eta\.label/);
	assert.match(
		inventory,
		/Math\.round\(item\.row\.value\)\.toLocaleString\(\)/,
	);
	assert.match(inventory, />\s*調整庫存\s*<\/Link>/);
});

test("庫存手機工具列與商品清單同層，保留數量、搜尋與狀態篩選", () => {
	assert.match(
		inventory,
		/className="seller-inventory__mobile-toolbar-title"[\s\S]*顯示 \{showSkeleton \? "—" : visible\.length\.toLocaleString\(\)\} 件/s,
	);
	assert.match(
		inventory,
		/className="seller-orders__search seller-inventory__mobile-search"[\s\S]*aria-label="搜尋庫存"/s,
	);
	assert.match(inventory, /aria-label="篩選庫存狀態"/);
	assert.match(
		inventory,
		/className="seller-inventory__card seller-mobile-disclosure-desktop"/,
	);
	assert.doesNotMatch(
		inventory,
		/SellerMobileDisclosureList[\s\S]*className="p-2\.5"/s,
	);
});
