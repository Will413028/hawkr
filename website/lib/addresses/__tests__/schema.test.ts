import { describe, expect, it } from "vitest";
import { addressFormSchema } from "@/lib/addresses/schema";

describe("addressFormSchema", () => {
	it("accepts a valid Taiwan mobile and town pair", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0912-345-678",
			countyCode: "63000",
			townCode: "63000020",
			detail: "信義路一段 1 號",
			isDefault: true,
		});

		expect(result.success).toBe(true);
	});

	it("rejects a town that does not belong to the selected county", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0912345678",
			countyCode: "63000",
			townCode: "66000060",
			detail: "信義路一段 1 號",
			isDefault: false,
		});

		expect(result.success).toBe(false);
		expect(result.error?.flatten().fieldErrors.townCode).toContain(
			"「西屯區」不屬於「臺北市」",
		);
	});

	it("rejects an unknown county and an unknown town code", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0912345678",
			countyCode: "99999",
			townCode: "99999999",
			detail: "信義路一段 1 號",
			isDefault: false,
		});

		expect(result.success).toBe(false);
		const errors = result.error?.flatten().fieldErrors;
		expect(errors?.countyCode).toContain("縣市不在台灣行政區清單內");
		expect(errors?.townCode).toContain("鄉鎮市區不在台灣行政區清單內");
	});

	it("rejects an empty town code instead of sending it", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0912345678",
			countyCode: "63000",
			townCode: "",
			detail: "信義路一段 1 號",
			isDefault: false,
		});

		expect(result.success).toBe(false);
		expect(result.error?.flatten().fieldErrors.townCode).toContain(
			"請選擇鄉鎮市區",
		);
	});

	it("rejects a detail longer than 255", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0912345678",
			countyCode: "63000",
			townCode: "63000020",
			detail: "路".repeat(256),
			isDefault: false,
		});

		expect(result.success).toBe(false);
		expect(result.error?.flatten().fieldErrors.detail).toContain(
			"詳細地址最長 255 字",
		);
	});

	it("rejects non-mobile Taiwan phone numbers", () => {
		const result = addressFormSchema.safeParse({
			receiverName: "王小明",
			receiverPhone: "0212345678",
			countyCode: "63000",
			townCode: "63000020",
			detail: "信義路一段 1 號",
			isDefault: false,
		});

		expect(result.success).toBe(false);
		expect(result.error?.flatten().fieldErrors.receiverPhone).toContain(
			"請輸入 09 開頭的台灣手機號碼",
		);
	});

	it("accepts a one-character detail and rejects an empty one", () => {
		const base = {
			receiverName: "王小明",
			receiverPhone: "0912345678",
			countyCode: "63000",
			townCode: "63000020",
			isDefault: false,
		};
		expect(addressFormSchema.safeParse({ ...base, detail: "1" }).success).toBe(
			true,
		);
		for (const detail of ["", "   "]) {
			const result = addressFormSchema.safeParse({ ...base, detail });
			expect(result.success).toBe(false);
			expect(result.error?.flatten().fieldErrors.detail).toContain(
				"請輸入詳細地址",
			);
		}
	});
});
