import { describe, expect, it } from "vitest";
import { delegatedCapabilityChipLabels } from "./labels";

describe("delegatedCapabilityChipLabels", () => {
	it("shows media.upload as its own chip, outside every group", () => {
		expect(delegatedCapabilityChipLabels(["media.upload"])).toEqual([
			"上傳檔案",
		]);
		expect(
			delegatedCapabilityChipLabels(["products.read", "media.upload"]),
		).toContain("上傳檔案");
	});
});
