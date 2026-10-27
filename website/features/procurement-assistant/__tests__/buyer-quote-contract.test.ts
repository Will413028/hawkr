import { describe, expect, it } from "vitest";
import {
	type BuyerQuoteAcceptanceCommand,
	buyerFacingSellerResponseListSchema,
	buyerFacingSellerResponseSchema,
	buyerQuoteAcceptanceBody,
	buyerQuoteAcceptanceResponseSchema,
	buyerQuoteFailure,
	formatTwdAmount,
	hasComparableUnitBasis,
	isAcceptableQuote,
	parseCanonicalTwdAmount,
	QUOTE_ACCEPTABILITY_VALUES,
	QUOTE_ACCEPTED_HEADLINE,
	quoteAcceptabilityLabel,
} from "../buyer-quote-contract";

function quote(overrides: Record<string, unknown> = {}) {
	return {
		responseType: "MANUAL_QUOTE",
		offeredQuality: { grade: "特級", inspectionNotes: "抽驗三箱" },
		unitBasis: { quantity: "1", unit: "KG" },
		offeredQuantity: "500",
		offeredQuantityUnit: "KG",
		conversionBasis: "1 台斤 = 0.6 公斤",
		currency: "TWD",
		goodsAmount: "18000",
		shippingAmount: "0",
		taxAmount: "900",
		taxInclusionBasis: "TAX_EXCLUDED",
		totalAmount: "18900",
		leadTimeDays: "3",
		promisedDeliveryAt: "2026-09-20T02:00:00Z",
		validUntil: "2026-09-18T02:00:00Z",
		termsNote: "需自備棧板",
		deviationDisclosure: "數量以實際採收為準",
		...overrides,
	};
}

function row(overrides: Record<string, unknown> = {}) {
	return {
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		status: "SUBMITTED",
		payloadHash: "a".repeat(64),
		submittedAt: "2026-09-13T02:00:00Z",
		seller: {
			displayName: "範例鮮果合作社",
			partyType: "PLATFORM_SELLER",
			storeCode: "greenfield",
		},
		quote: quote(),
		outsideBuyerRequestedRange: false,
		shortlisted: false,
		acceptability: "ACCEPTABLE",
		...overrides,
	};
}

describe("BuyerFacingSellerResponse", () => {
	it("解析十七欄的報價 payload", () => {
		const parsed = buyerFacingSellerResponseSchema.parse(row());
		expect(Object.keys(parsed.quote).sort()).toEqual(
			[
				"conversionBasis",
				"currency",
				"deviationDisclosure",
				"goodsAmount",
				"leadTimeDays",
				"offeredQuality",
				"offeredQuantity",
				"offeredQuantityUnit",
				"promisedDeliveryAt",
				"responseType",
				"shippingAmount",
				"taxAmount",
				"taxInclusionBasis",
				"termsNote",
				"totalAmount",
				"unitBasis",
				"validUntil",
			].sort(),
		);
		expect(Object.keys(parsed.quote)).toHaveLength(17);
	});

	it("賣家身分只留三欄，站外業者的聯絡資料進不了瀏覽器 state", () => {
		const parsed = buyerFacingSellerResponseSchema.parse(
			row({
				seller: {
					displayName: "豐田產地直送",
					partyType: "EXTERNAL_VENDOR",
					storeCode: "",
					// 這三個永遠不該出現在買家面投影；就算出現也必須被剝掉。
					email: "vendor@example.com",
					kycStatus: "PENDING",
					prospectContact: "0912-000-000",
				},
			}),
		);
		expect(parsed.seller).toEqual({
			displayName: "豐田產地直送",
			partyType: "EXTERNAL_VENDOR",
			storeCode: "",
		});
		expect(JSON.stringify(parsed)).not.toContain("vendor@example.com");
		expect(JSON.stringify(parsed)).not.toContain("PENDING");
	});

	it("金額不是 canonical 十進位字串就 fail closed", () => {
		expect(
			buyerFacingSellerResponseSchema.safeParse(
				row({ quote: quote({ totalAmount: "18,900" }) }),
			).success,
		).toBe(false);
		expect(
			buyerFacingSellerResponseSchema.safeParse(
				row({ quote: quote({ totalAmount: 18900 }) }),
			).success,
		).toBe(false);
	});

	it("運費與稅額允許 0，價格區間那支不允許的規則不套用到這裡", () => {
		const parsed = buyerFacingSellerResponseSchema.parse(
			row({ quote: quote({ shippingAmount: "0", taxAmount: "0" }) }),
		);
		expect(parsed.quote.shippingAmount).toBe("0");
		expect(parsed.quote.taxAmount).toBe("0");
	});

	it("交期兩種 wire 形狀都收，並一律正規化成字串", () => {
		expect(
			buyerFacingSellerResponseSchema.parse(
				row({ quote: quote({ leadTimeDays: 5 }) }),
			).quote.leadTimeDays,
		).toBe("5");
		expect(
			buyerFacingSellerResponseSchema.parse(
				row({ quote: quote({ leadTimeDays: "5" }) }),
			).quote.leadTimeDays,
		).toBe("5");
	});

	it("清單接受裸陣列與包一層具名欄位兩種外層", () => {
		expect(buyerFacingSellerResponseListSchema.parse([row()])).toHaveLength(1);
		expect(
			buyerFacingSellerResponseListSchema.parse({ sellerResponses: [row()] }),
		).toHaveLength(1);
		expect(
			buyerFacingSellerResponseListSchema.parse({ items: [row()] }),
		).toHaveLength(1);
	});

	it("清單裡有一筆壞掉就整份 fail closed，不只顯示解析得出來的那幾筆", () => {
		const result = buyerFacingSellerResponseListSchema.safeParse([
			row(),
			row({ payloadHash: "not-a-hash" }),
		]);
		expect(result.success).toBe(false);
	});
});

describe("acceptability 是伺服器判定", () => {
	it("值域五個值都認得，只有 ACCEPTABLE 可接受", () => {
		// 順序跟著 CodeMeta `ProcurementQuoteAcceptability` 的宣告順序（值域來源已
		// 由本檔的本地清單換成生成檔）；仍是逐項全等，不是子集比對。
		expect([...QUOTE_ACCEPTABILITY_VALUES]).toEqual([
			"ACCEPTABLE",
			"SUPERSEDED",
			"WITHDRAWN",
			"EXPIRED",
			"INELIGIBLE",
		]);
		for (const value of QUOTE_ACCEPTABILITY_VALUES) {
			expect(isAcceptableQuote(value)).toBe(value === "ACCEPTABLE");
			expect(quoteAcceptabilityLabel(value)).not.toBe("");
		}
	});

	it("沒見過的值 fail closed，而且不編一個原因出來", () => {
		expect(isAcceptableQuote("SOMETHING_NEW")).toBe(false);
		expect(quoteAcceptabilityLabel("SOMETHING_NEW")).toBe(
			"目前無法接受這個版本",
		);
	});

	it("不可接受的版本仍然解析得出來，留在清單裡供稽核", () => {
		const parsed = buyerFacingSellerResponseSchema.parse(
			row({ acceptability: "SUPERSEDED" }),
		);
		expect(parsed.acceptability).toBe("SUPERSEDED");
		expect(isAcceptableQuote(parsed.acceptability)).toBe(false);
	});
});

describe("acceptance 請求", () => {
	const command: BuyerQuoteAcceptanceCommand = {
		idempotencyKey: "bqa-abc",
		sellerResponseId: "1234567890123456788",
		expectedSellerResponseVersion: "1234567890123456791",
		expectedRfqSnapshotHash: "b".repeat(64),
	};

	it("body 恰好三個 CAS 欄位，不帶冪等鍵也不帶任何金額", () => {
		const body = buyerQuoteAcceptanceBody(command);
		expect(Object.keys(body).sort()).toEqual([
			"expectedRfqSnapshotHash",
			"expectedSellerResponseVersion",
			"sellerResponseId",
		]);
		const serialized = JSON.stringify(body);
		expect(serialized).not.toContain("idempotencyKey");
		expect(serialized).not.toContain("Amount");
		expect(serialized).not.toContain("acceptedDealPrice");
		expect(serialized).not.toContain("currency");
	});
});

describe("BuyerQuoteAcceptanceResponse", () => {
	function ack(overrides: Record<string, unknown> = {}) {
		return {
			buyerAcceptanceId: "1234567890123456805",
			sellerResponseId: "1234567890123456788",
			sellerResponseVersion: "1234567890123456791",
			acceptedDealPrice: "18900",
			currency: "TWD",
			acceptedAt: "2026-09-13T03:00:00Z",
			orderProvisioningState: "CREATING_ORDER",
			orderNo: "",
			paymentDeadline: "",
			replayed: false,
			...overrides,
		};
	}

	it("CREATING_ORDER 的 ACK 不帶訂單編號與付款期限", () => {
		const parsed = buyerQuoteAcceptanceResponseSchema.parse(ack());
		expect(parsed.orderNo).toBe("");
		expect(parsed.paymentDeadline).toBe("");
	});

	it("有訂單編號卻不是 ORDER_CREATED 一律拒收", () => {
		expect(
			buyerQuoteAcceptanceResponseSchema.safeParse(
				ack({ orderNo: "HK20260913001" }),
			).success,
		).toBe(false);
	});

	it("ORDER_CREATED 卻沒有訂單編號一律拒收", () => {
		expect(
			buyerQuoteAcceptanceResponseSchema.safeParse(
				ack({ orderProvisioningState: "ORDER_CREATED" }),
			).success,
		).toBe(false);
	});

	it("ORDER_CREATED 帶單號與付款期限時解析成功", () => {
		const parsed = buyerQuoteAcceptanceResponseSchema.parse(
			ack({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260913001",
				paymentDeadline: "2026-09-14T03:00:00Z",
			}),
		);
		expect(parsed.orderNo).toBe("HK20260913001");
	});

	it("ORDER_FAILED 是合法狀態，不是解析失敗", () => {
		expect(
			buyerQuoteAcceptanceResponseSchema.parse(
				ack({ orderProvisioningState: "ORDER_FAILED" }),
			).orderProvisioningState,
		).toBe("ORDER_FAILED");
	});

	it("成交固定文案逐字凍結", () => {
		expect(QUOTE_ACCEPTED_HEADLINE).toBe(
			"已接受這個報價版本，成交價格已凍結，尚未付款",
		);
	});
});

describe("錯誤碼", () => {
	it("STALE_SELLER_RESPONSE 明講報價已變動，而且不承諾自動接受新版", () => {
		const failure = buyerQuoteFailure(409, "STALE_SELLER_RESPONSE");
		expect(failure.detail).toContain("新的報價版本");
		expect(failure.detail).toContain("不會自動");
		expect(failure.retryable).toBe(false);
	});

	it("已在結帳與已成交結案是兩句不同的話", () => {
		const active = buyerQuoteFailure(409, "RFQ_CHECKOUT_ALREADY_ACTIVE");
		const awarded = buyerQuoteFailure(409, "RFQ_ALREADY_AWARDED");
		expect(active.detail).not.toBe(awarded.detail);
		expect(active.detail).toContain("付款");
		expect(awarded.detail).toContain("成交結案");
	});

	it("結帳中會把伺服器給的既有訂單編號帶出來，其他錯誤碼不會", () => {
		expect(
			buyerQuoteFailure(409, "RFQ_CHECKOUT_ALREADY_ACTIVE", "HK20260913001")
				.orderNo,
		).toBe("HK20260913001");
		expect(
			buyerQuoteFailure(409, "RFQ_ALREADY_AWARDED", "HK20260913001").orderNo,
		).toBeUndefined();
	});

	it("九個業務碼各有自己的文案，沒有落到未知錯誤", () => {
		const codes = [
			"STALE_RFQ_SNAPSHOT",
			"STALE_PUBLICATION",
			"STALE_SELLER_RESPONSE",
			"RFQ_RESPONSE_EXPIRED",
			"RFQ_RECIPIENT_CLOSED",
			"RFQ_CHECKOUT_ALREADY_ACTIVE",
			"RFQ_ALREADY_AWARDED",
			"QUOTE_AMOUNT_INCONSISTENT",
			"QUOTE_QUALITY_REQUIRED",
		];
		const details = new Set<string>();
		for (const code of codes) {
			const failure = buyerQuoteFailure(
				code.startsWith("QUOTE_") ? 422 : 409,
				code,
			);
			expect(failure.code).toBe(code);
			details.add(failure.detail);
		}
		expect(details.size).toBe(codes.length);
	});

	it("沿用的兩個既有碼也認得", () => {
		expect(buyerQuoteFailure(409, "IDEMPOTENCY_KEY_CONFLICT").code).toBe(
			"IDEMPOTENCY_KEY_CONFLICT",
		);
		expect(buyerQuoteFailure(404, "RESOURCE_NOT_FOUND").code).toBe(
			"RESOURCE_NOT_FOUND",
		);
	});

	it("認不得的 reason 落回依 HTTP 狀態分類的傳輸層錯誤", () => {
		expect(buyerQuoteFailure(503, "SOMETHING_ELSE").code).toBe("SERVER_ERROR");
		expect(buyerQuoteFailure(0, "").code).toBe("NETWORK_ERROR");
		expect(buyerQuoteFailure(401, "").code).toBe("UNAUTHORIZED");
		expect(buyerQuoteFailure(404, "").code).toBe("RESOURCE_NOT_FOUND");
	});
});

describe("比較輔助", () => {
	it("計價基準不同時回報不可直接相比", () => {
		const kilogram = buyerFacingSellerResponseSchema.parse(row());
		const catty = buyerFacingSellerResponseSchema.parse(
			row({
				sellerResponseId: "1234567890123456789",
				quote: quote({
					unitBasis: { quantity: "1", unit: "CATTY" },
					offeredQuantityUnit: "CATTY",
				}),
			}),
		);
		expect(hasComparableUnitBasis([kilogram])).toBe(true);
		expect(hasComparableUnitBasis([kilogram, kilogram])).toBe(true);
		expect(hasComparableUnitBasis([kilogram, catty])).toBe(false);
	});

	it("金額顯示逐字保留伺服器給的小數位", () => {
		expect(formatTwdAmount("18900")).toBe("NT$18,900");
		expect(formatTwdAmount("1200.5")).toBe("NT$1,200.5");
		expect(formatTwdAmount("0")).toBe("NT$0");
	});

	it("canonical 金額解析允許 0、拒絕分隔符與科學記號", () => {
		expect(parseCanonicalTwdAmount("0")).toBe(BigInt(0));
		expect(parseCanonicalTwdAmount("1.5")).toBe(BigInt(15_000));
		expect(parseCanonicalTwdAmount("1e3")).toBeNull();
		expect(parseCanonicalTwdAmount("1,000")).toBeNull();
		expect(parseCanonicalTwdAmount("-1")).toBeNull();
	});
});
