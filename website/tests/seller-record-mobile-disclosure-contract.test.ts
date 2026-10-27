import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const messages = source(
	"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
);
const customers = source(
	"features/seller-center/customers/CustomersClient.tsx",
);
const coupons = source(
	"features/seller-center/coupons/SellerCouponsClient.tsx",
);

test("message, customer and coupon records share the phone disclosure seam", () => {
	for (const page of [messages, customers, coupons]) {
		assert.match(page, /SellerMobileDisclosureList/);
		assert.match(page, /seller-mobile-disclosure-desktop/);
		assert.match(page, /seller-mobile-disclosure__body/);
		assert.match(page, /seller-mobile-disclosure__actions/);
	}
});

test("message summaries stay scannable and keep both workbench actions", () => {
	assert.match(messages, /getKey=\{messageRowKey\}/);
	assert.match(
		messages,
		/row\.unreadCount > 0[\s\S]*formatDateTime\(row\.lastMessageAt\)/,
	);
	assert.match(messages, /<span>最新訊息<\/span>/);
	assert.match(messages, /電話 \{row\.phone\}/);
	assert.match(messages, /<span>下單時間<\/span>/);
	assert.match(messages, /<span>購買紀錄<\/span>/);
	assert.match(messages, /<span>上次回覆<\/span>/);
	assert.match(messages, />\s*開啟聊天室\s*<\/button>/);
	assert.match(messages, />\s*客戶資料\s*<\/button>/);
});

test("customer summaries expose order count and classification before details", () => {
	assert.match(customers, /getKey=\{customerRowKey\}/);
	assert.match(customers, /meta: `\$\{\(customer\.orderCount \?\? 0\).*筆`/);
	assert.match(
		customers,
		/status: customerTypeLabel\(customer\.customerType\)/,
	);
	assert.match(customers, /customer\.tags\?\.map/);
	assert.match(customers, /<span>累計消費<\/span>/);
	assert.match(customers, /<span>最近購買<\/span>/);
	assert.match(customers, />\s*客戶資料\s*<\/button>/);
	assert.match(customers, />\s*發訊息\s*<\/button>/);
});

test("coupon summaries keep discount and state while dates and actions disclose", () => {
	assert.match(coupons, /getKey=\{couponKey\}/);
	assert.match(
		coupons,
		/leading: <SellerMobileDisclosureIcon name="discount" \/>/,
	);
	assert.match(coupons, /meta: couponValueLabel\(coupon\)/);
	assert.match(coupons, /seller-coupons__state-pill/);
	assert.match(coupons, /<span>活動日期<\/span>/);
	assert.match(coupons, /<span>已領取<\/span>/);
	assert.match(coupons, />\s*檢視\s*<\/Button>/);
	assert.match(coupons, />\s*收回\s*<\/Button>/);
});
