import assert from "node:assert/strict";
import { test } from "node:test";
import {
	draftToSubmitBody,
	emptyDraft,
} from "../features/seller-center/products/adapters/productFormAdapter";
import { AMBIENT_TEMPERATURE_CODE } from "../lib/seller/ambient-temperature";

test("product submit body keeps stock on create and omits it on edit", () => {
	const draft = {
		...emptyDraft(),
		name: "Test product",
		skus: [
			{
				id: "123",
				sku_name: "Default",
				price: "100",
				stock: 7,
				unit: "pcs",
			},
		],
	};

	// The body is serialised to the gateway's camelCase CreateSkuInput shape
	// (skuName / stockQty / price as strings), so stock surfaces as `stockQty`.
	const createBody = draftToSubmitBody(draft);
	assert.equal(createBody.skus?.[0].stockQty, "7");
	assert.equal(createBody.skus?.[0].id, "123");

	const updateBody = draftToSubmitBody(draft, undefined, {
		includeStock: false,
	});
	assert.equal(updateBody.skus?.[0].id, "123");
	assert.equal("stockQty" in (updateBody.skus?.[0] ?? {}), false);
});

test("product submit body serialises categoryId and shipping methods camelCase", () => {
	const draft = {
		...emptyDraft(),
		name: "Test product",
		category_id: 4567,
		skus: [
			{
				id: "",
				sku_name: "1kg",
				price: "100",
				stock: 3,
				unit: "件",
				length_cm: "20",
				width_cm: "15",
				height_cm: "10",
				weight: "1.5",
				weight_unit: "3301",
				shipping_methods: [
					{
						type: "3001",
						fee_payer: "3102",
						free_threshold: "500",
					},
				],
			},
		],
	};

	const body = draftToSubmitBody(draft);
	assert.equal(body.categoryId, "4567");
	const sku = body.skus?.[0];
	assert.equal(sku?.lengthCm, "20");
	assert.equal(sku?.widthCm, "15");
	assert.equal(sku?.heightCm, "10");
	assert.deepEqual(sku?.shippingMethods, [
		{
			type: "3001",
			temperature: AMBIENT_TEMPERATURE_CODE,
			feePayer: "3102",
			freeThreshold: "500",
		},
	]);
});

test("product submit body clears a threshold when the buyer pays shipping", () => {
	const draft = {
		...emptyDraft(),
		name: "Test product",
		skus: [
			{
				id: "",
				sku_name: "Default",
				price: "100",
				stock: 1,
				unit: "件",
				shipping_methods: [
					{
						type: "3001",
						fee_payer: "3101",
						free_threshold: "3000",
					},
				],
			},
		],
	};

	assert.deepEqual(draftToSubmitBody(draft).skus?.[0].shippingMethods, [
		{
			type: "3001",
			temperature: AMBIENT_TEMPERATURE_CODE,
			feePayer: "3101",
			freeThreshold: "0",
		},
	]);
});

test("seller product submission always sends the ambient temperature code", () => {
	assert.equal(AMBIENT_TEMPERATURE_CODE, "3201");
	const draft = {
		...emptyDraft(),
		name: "Test product",
		skus: [
			{
				id: "",
				sku_name: "Default",
				price: "100",
				stock: 1,
				unit: "件",
				shipping_methods: [
					{ type: "3001", fee_payer: "3101", free_threshold: "0" },
					{ type: "3002", fee_payer: "3102", free_threshold: "0" },
				],
			},
		],
	};

	const methods = draftToSubmitBody(draft).skus?.[0].shippingMethods ?? [];
	assert.equal(methods.length, 2);
	for (const method of methods) {
		assert.equal(method.temperature, AMBIENT_TEMPERATURE_CODE);
	}
});
