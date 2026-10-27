import { describe, expect, it } from "vitest";
import type { ImportItemDetail } from "../types";
import { importItemToDraft } from "./importItemAdapter";

describe("importItemToDraft certifications", () => {
	it("keeps the certificate number without inferring review approval", () => {
		const item = {
			brief: { name: "香蕉" },
			descriptionJson: "",
			skus: [],
			galleryAssets: [],
			descriptionAssets: [],
			errors: [],
			warnings: [],
			commitRevision: "0",
			origin: "",
			hashtags: [],
			certifications: [{ name: "3702", certNo: "58589605051" }],
			targetStatus: "DRAFT",
		} as unknown as ImportItemDetail;

		const draft = importItemToDraft(item);

		expect(draft.certifications).toEqual([
			{ code: "3702", certNo: "58589605051" },
		]);
		expect(draft.certifications?.[0]).not.toHaveProperty("verified");
		expect(draft.certifications?.[0]).not.toHaveProperty("approved");
	});
});
