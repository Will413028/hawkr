import { countyByCode, townByCode } from "@hawkr/twaddr/lookup";
import { z } from "zod";
import { checkDisplayLength } from "@/lib/validation/display-length";

export const addressFormSchema = z
	.object({
		receiverName: z.string().trim().min(1, "請輸入收件人姓名"),
		receiverPhone: z
			.string()
			.trim()
			.min(1, "請輸入聯絡電話")
			.regex(/^09\d{8}$|^09\d{2}-\d{3}-\d{3}$/, "請輸入 09 開頭的台灣手機號碼"),
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
