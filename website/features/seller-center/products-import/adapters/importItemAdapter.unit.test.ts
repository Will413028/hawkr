import { describe, expect, it } from "vitest";
import { draftSkuToImportSku, importSkuToDraft } from "./importItemAdapter";

describe("import SKU shipping methods", () => {
	it("sends no temperature field for any carrier", () => {
		const out = draftSkuToImportSku({
			id: "",
			sku_name: "規格",
			price: "100",
			stock: 1,
			unit: "3301",
			shipping_methods: [
				{ type: "3001", fee_payer: "3101", free_threshold: "0" },
				{ type: "3002", fee_payer: "3101", free_threshold: "0" },
			],
		});

		expect(out.shippingMethods).toEqual([
			{ method: "3001" },
			{ method: "3002" },
		]);
	});

	it("collapses per-temperature rows from the server into one row per carrier", () => {
		const draft = importSkuToDraft({
			skuId: "1",
			shippingMethods: [
				{ method: "3001", temperature: "3201" },
				{ method: "3001", temperature: "3202" },
				{ method: "3002", temperature: "3201" },
			],
		} as never);

		expect(draft.shipping_methods?.map((m) => m.type)).toEqual([
			"3001",
			"3002",
		]);
	});
});
