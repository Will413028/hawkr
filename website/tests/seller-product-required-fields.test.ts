import assert from "node:assert/strict";
import test from "node:test";
import {
	emptyDraft,
	validateDraft,
} from "../features/seller-center/products/adapters/productFormAdapter";
import type {
	ProductSku,
	SellerProductDetail,
} from "../lib/products/seller-repo";

/**
 * The form's required-field list, pinned rule by rule against the backend that
 * actually enforces each one.
 *
 * Before #435 the form checked the product name and "at least one SKU" and let
 * everything else through, so the seller discovered the rest of the contract
 * one 400 at a time — the backend validates in order and RETURNS on the first
 * failure, so a new product with the 可配送方案 section left collapsed produced
 * 「商品重量 必須大於 0」→ 「包裝長度…」→ 「包裝寬度…」→ 「包裝高度…」on four
 * separate submits.
 *
 * Each case below names where the backend rejection lives. Line numbers drift;
 * the message strings are the durable half and were observed live against the
 * dev gateway (POST /app/products, 2026-08-05) rather than read off the source.
 *
 * The failure this test exists to catch is the silent one: the backend gains a
 * required field, the form does not, and nothing anywhere goes red — sellers
 * just start hitting 400s again.
 */

/** A draft the backend accepts: every rule below satisfied. */
function completeDraft(): SellerProductDetail {
	const base = emptyDraft();
	return {
		...base,
		name: "測試商品",
		category_id: 527,
		skus: [
			{
				...base.skus[0],
				sku_name: "預設規格",
				price: "100",
				stock: 5,
				weight: "1",
				length_cm: "10",
				width_cm: "10",
				height_cm: "10",
			},
		],
	};
}

function withSku(patch: Partial<ProductSku>): SellerProductDetail {
	const draft = completeDraft();
	return { ...draft, skus: [{ ...draft.skus[0], ...patch }] };
}

interface Rule {
	/** What the seller did wrong. */
	readonly what: string;
	/** Where the backend rejects it, and with what message. */
	readonly backend: string;
	readonly draft: SellerProductDetail;
	/** The message the form must produce instead of that round trip. */
	readonly expect: string;
}

const RULES: readonly Rule[] = [
	{
		what: "商品名稱留白",
		backend:
			'gateway seller_product.go CreateProductRequest.Name binding:"required"',
		draft: { ...completeDraft(), name: "  " },
		expect: "商品名稱必填",
	},
	{
		what: "沒選商品分類",
		backend:
			'gateway seller_product.go CreateProductRequest.CategoryID binding:"required"',
		draft: { ...completeDraft(), category_id: undefined },
		expect: "請選擇商品分類（需選到最後一層）",
	},
	{
		what: "原價 0",
		backend:
			"product-service ProductService.createProduct「商品價格 必須大於 0」",
		draft: withSku({ price: "0" }),
		expect: "原價需大於 0",
	},
	{
		what: "售價 0（有填但不合法）",
		backend: "product-service ProductService.createProduct「特價 必須大於 0」",
		draft: withSku({ sale_price: "-5" }),
		expect: "售價需大於 0",
	},
	{
		what: "售價不低於原價",
		backend:
			"product-service ProductService.createProduct「特價 必須低於原價」",
		draft: withSku({ sale_price: "100" }),
		expect: "售價需小於原價",
	},
	{
		what: "庫存為負",
		backend:
			"product-service ProductService.createProduct「庫存數量 不可為負數」",
		draft: withSku({ stock: -1 }),
		expect: "庫存不可為負",
	},
	{
		what: "沒填重量",
		backend:
			"product-service ProductService.createProduct「商品重量 必須大於 0」",
		draft: withSku({ weight: "" }),
		expect: "重量需大於 0",
	},
	{
		what: "沒填包裝長度",
		backend:
			"product-service ProductService.createProduct「包裝長度 必須大於 0」",
		draft: withSku({ length_cm: "" }),
		expect: "包裝長度需大於 0",
	},
	{
		what: "沒填包裝寬度",
		backend:
			"product-service ProductService.createProduct「包裝寬度 必須大於 0」",
		draft: withSku({ width_cm: "" }),
		expect: "包裝寬度需大於 0",
	},
	{
		what: "沒填包裝高度",
		backend:
			"product-service ProductService.createProduct「包裝高度 必須大於 0」",
		draft: withSku({ height_cm: "" }),
		expect: "包裝高度需大於 0",
	},
];

test("a draft that satisfies every backend rule reports nothing", () => {
	assert.deepEqual(validateDraft(completeDraft()), []);
});

test("every backend rejection has a matching client-side rule", () => {
	for (const rule of RULES) {
		const problems = validateDraft(rule.draft);
		assert.ok(
			problems.includes(rule.expect),
			`${rule.what} → the form must say「${rule.expect}」before the request; ` +
				`the backend otherwise answers it at ${rule.backend}. Got: ${JSON.stringify(problems)}`,
		);
	}
});

test("each rule fires only for its own violation", () => {
	// Otherwise a rule could "pass" the check above by firing on everything —
	// e.g. a validator that always appends every message.
	const clean = completeDraft();
	for (const rule of RULES) {
		assert.ok(
			!validateDraft(clean).includes(rule.expect),
			`${rule.what}: 「${rule.expect}」must not fire on a complete draft`,
		);
	}
});

test("a bare new product reports all of its missing fields in one go", () => {
	// The state the seller is actually in: the create form as it mounts, with a
	// name typed in. Nothing else has been filled, and this is exactly the draft
	// that used to need five submits to get through.
	const bare: SellerProductDetail = { ...emptyDraft(), name: "測試商品" };
	const problems = validateDraft(bare);

	assert.deepEqual(problems, [
		"請選擇商品分類（需選到最後一層）",
		"原價需大於 0",
		"重量需大於 0",
		"包裝長度需大於 0",
		"包裝寬度需大於 0",
		"包裝高度需大於 0",
	]);
});

test("with several SKUs every message says which one it is about", () => {
	const draft = completeDraft();
	const problems = validateDraft({
		...draft,
		skus: [
			{ ...draft.skus[0], sku_name: "大包" },
			{ ...draft.skus[0], sku_name: "小包", weight: "" },
			{ ...draft.skus[0], sku_name: "  ", height_cm: "" },
		],
	});

	assert.deepEqual(problems, [
		"規格「小包」的重量需大於 0",
		"第 3 個規格需要名稱",
		"規格「第 3 個」的包裝高度需大於 0",
	]);
});

test("an empty SKU list is the only problem worth reporting", () => {
	// Per-SKU rules have nothing to run against, so listing them would be noise.
	assert.deepEqual(validateDraft({ ...completeDraft(), skus: [] }), [
		"至少要一個規格 (SKU)",
	]);
});
