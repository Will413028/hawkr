import { countyByCode, townByCode } from "@hawkr/twaddr/lookup";
import { z } from "zod";
import { checkDisplayLength } from "@/lib/validation/display-length";

export const addressFormSchema = z
	.object({
		// 地址本收件人姓名：顯示長度 [4, 30]（中文 1 字算 2、英文 1 字算 1），
		// 對齊後端 PayUni 物流欄位約束。顯示長度範圍在物件層 superRefine 內
		// 驗（見下方），欄位本身維持純 string 以免破壞型別推導。
		receiverName: z.string().trim().min(1, "請輸入收件人姓名"),
		receiverPhone: z
			.string()
			.trim()
			.min(1, "請輸入聯絡電話")
			.regex(/^09\d{8}$|^09\d{2}-\d{3}-\d{3}$/, "請輸入 09 開頭的台灣手機號碼"),
		// 縣市只用來帶出鄉鎮市區選單；送出的是 townCode（內政部 8 碼，前 5 碼是縣市）。
		countyCode: z.string().trim().min(1, "請選擇縣市"),
		townCode: z.string().trim().min(1, "請選擇鄉鎮市區"),
		detail: z
			.string()
			.trim()
			.min(1, "請輸入詳細地址")
			.max(255, "詳細地址最長 255 字"),
		isDefault: z.boolean(),
	})
	.superRefine((values, ctx) => {
		const nameError = checkDisplayLength(values.receiverName, {
			label: "收件人姓名",
			min: 4,
			max: 30,
		});
		if (nameError) {
			ctx.addIssue({
				code: "custom",
				path: ["receiverName"],
				message: nameError,
			});
		}

		if (values.countyCode && !countyByCode(values.countyCode)) {
			ctx.addIssue({
				code: "custom",
				path: ["countyCode"],
				message: "縣市不在台灣行政區清單內",
			});
		}

		if (values.townCode) {
			const town = townByCode(values.townCode);
			if (!town) {
				ctx.addIssue({
					code: "custom",
					path: ["townCode"],
					message: "鄉鎮市區不在台灣行政區清單內",
				});
			} else if (values.countyCode && town.countyCode !== values.countyCode) {
				ctx.addIssue({
					code: "custom",
					path: ["townCode"],
					message: `「${town.name}」不屬於「${countyByCode(values.countyCode)?.name ?? values.countyCode}」`,
				});
			}
		}
	});

export type AddressFormValues = z.infer<typeof addressFormSchema>;
