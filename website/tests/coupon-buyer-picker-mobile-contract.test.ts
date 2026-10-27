import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const picker = readFileSync(
	path.join(root, "features/seller-center/coupons/CouponBuyerPicker.tsx"),
	"utf8",
);

test("coupon buyer picker is a safe-area-aware full-height phone surface", () => {
	assert.match(picker, /place-items-stretch p-0 sm:place-items-center sm:p-4/);
	assert.match(picker, /h-\[100dvh\] max-h-\[100dvh\]/);
	assert.match(picker, /pb-\[env\(safe-area-inset-bottom,0px\)\]/);
	assert.match(picker, /pt-\[env\(safe-area-inset-top,0px\)\]/);
	assert.match(picker, /rounded-none[\s\S]*sm:rounded-2xl/);
	assert.match(picker, /grid h-11 w-11 shrink-0 place-items-center/);
	assert.match(picker, /className="h-11 w-full[\s\S]*sm:h-10"/);
});

test("customer type, CRM tags and bulk actions stay on horizontal phone rails", () => {
	for (const rail of ["customer-type", "crm-tag", "bulk-actions"]) {
		const marker = `data-mobile-rail="${rail}"`;
		const markerAt = picker.indexOf(marker);
		assert.notEqual(markerAt, -1, `missing ${rail} rail`);
		const railSource = picker.slice(markerAt, markerAt + 420);
		assert.match(railSource, /flex-nowrap/);
		assert.match(railSource, /overflow-x-auto/);
		assert.match(railSource, /sm:flex-wrap/);
	}

	assert.match(picker, /h-11 shrink-0 whitespace-nowrap[\s\S]*sm:h-8/);
	assert.match(picker, /清除已選/);
	assert.match(
		picker,
		/className="h-11 min-h-11 shrink-0 whitespace-nowrap sm:hidden"/,
	);
});

test("phone footer only keeps count, cancel and compact confirm actions", () => {
	const footerAt = picker.indexOf('data-mobile-footer="compact-actions"');
	assert.notEqual(footerAt, -1);
	const footerEnd = picker.indexOf("</footer>", footerAt);
	assert.ok(footerEnd > footerAt);
	const footer = picker.slice(footerAt, footerEnd);

	assert.match(footer, /grid-cols-\[minmax\(0,1fr\)_auto_auto\][\s\S]*sm:flex/);
	assert.match(footer, /className="hidden sm:inline-flex"/);
	assert.match(footer, /<span className="sm:hidden">確認<\/span>/);
	assert.match(footer, /取消/);
	assert.match(footer, /h-11 min-h-11/);
});
