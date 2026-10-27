import { describe, expect, it } from "vitest";
import {
	ProcurementSellerResponseType,
	ProcurementTaxInclusionBasis,
} from "@/lib/codemeta/generated";
import {
	checkDraftAmountBalance,
	emptyVendorResponseDraft,
	isRfc3339,
	normalizeVendorResponseDraft,
	rfc3339ToTaipeiInput,
	taipeiInputToRfc3339,
	type VendorResponseDraft,
} from "./draft";

/**
 * 草稿模型的契約（#2538）。這裡驗的三件事各自對應一種只會靜默出錯的失誤：
 * 欄位數對不上（後端會拒收整包）、缺欄變成 `undefined`（受控欄位炸掉）、
 * 以及把正式提交的金額驗證誤植到草稿上（業者存不了半成品）。
 */
const WIRE_FIELDS = [
	"responseType",
	"offeredQuality",
	"unitBasis",
	"offeredQuantity",
	"offeredQuantityUnit",
	"conversionBasis",
	"currency",
	"goodsAmount",
	"shippingAmount",
	"taxAmount",
	"taxInclusionBasis",
	"totalAmount",
	"leadTimeDays",
	"promisedDeliveryAt",
	"validUntil",
	"termsNote",
	"deviationDisclosure",
] as const;

describe("emptyVendorResponseDraft", () => {
	it("剛好 17 欄，不多不少", () => {
		expect(Object.keys(emptyVendorResponseDraft()).sort()).toEqual(
			[...WIRE_FIELDS].sort(),
		);
	});

	it("三個有預設值的欄位取自 CodeMeta 值域，不是手寫字面值", () => {
		const draft = emptyVendorResponseDraft();
		expect(draft.responseType).toBe(ProcurementSellerResponseType.MANUAL_QUOTE);
		expect(draft.taxInclusionBasis).toBe(
			ProcurementTaxInclusionBasis.TAX_EXCLUDED,
		);
		expect(draft.currency).toBe("TWD");
	});

	it("其餘欄位一律空字串——草稿的起點不得替業者發明任何金額或品相", () => {
		const draft = emptyVendorResponseDraft();
		expect(draft.offeredQuality).toEqual({ grade: "", inspectionNotes: "" });
		expect(draft.unitBasis).toEqual({ quantity: "", unit: "" });
		expect(draft.goodsAmount).toBe("");
		expect(draft.totalAmount).toBe("");
		expect(draft.validUntil).toBe("");
	});
});

describe("normalizeVendorResponseDraft", () => {
	it("完整回聲原樣保留", () => {
		const echoed: VendorResponseDraft = {
			responseType:
				ProcurementSellerResponseType.ACCEPT_NON_PRICE_TERMS_WITH_QUOTE,
			offeredQuality: { grade: "特級", inspectionNotes: "抽驗 3 箱" },
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "300",
			offeredQuantityUnit: "KG",
			conversionBasis: "1 箱 = 10 KG",
			currency: "TWD",
			goodsAmount: "30000",
			shippingAmount: "800",
			taxAmount: "1540",
			taxInclusionBasis: ProcurementTaxInclusionBasis.TAX_INCLUDED,
			totalAmount: "32340",
			leadTimeDays: "3",
			promisedDeliveryAt: "2026-09-20 09:00",
			validUntil: "2026-09-18 16:00",
			termsNote: "雨天順延一日",
			deviationDisclosure: "包裝改用塑膠籃",
		};

		expect(normalizeVendorResponseDraft(echoed)).toEqual(echoed);
	});

	it("缺欄補空字串，任何一欄都不得是 undefined", () => {
		const normalized = normalizeVendorResponseDraft({ goodsAmount: "100" });

		expect(normalized.goodsAmount).toBe("100");
		for (const key of WIRE_FIELDS) {
			expect(normalized[key]).toBeDefined();
		}
		// 受控欄位吃到 undefined 會在第一次輸入時從 uncontrolled 跳成 controlled。
		expect(normalized.termsNote).toBe("");
		expect(normalized.offeredQuality.grade).toBe("");
		expect(normalized.unitBasis.unit).toBe("");
	});

	it("列舉與幣別缺席時退回預設值，其餘欄位不猜", () => {
		const normalized = normalizeVendorResponseDraft({});
		expect(normalized.responseType).toBe(
			ProcurementSellerResponseType.MANUAL_QUOTE,
		);
		expect(normalized.taxInclusionBasis).toBe(
			ProcurementTaxInclusionBasis.TAX_EXCLUDED,
		);
		expect(normalized.currency).toBe("TWD");
	});

	it("沒見過的列舉值原樣保留，不悄悄改成預設值", () => {
		// 吞掉它等於替後端決定「這個事實不重要」，而業者存的是他自己填的東西。
		const normalized = normalizeVendorResponseDraft({
			responseType: "SOMETHING_NEW",
		});
		expect(normalized.responseType).toBe("SOMETHING_NEW");
	});

	it("收到 number 轉成字串保住金額，收到其他型別才落回空字串", () => {
		const normalized = normalizeVendorResponseDraft({
			goodsAmount: 30000,
			shippingAmount: null,
			taxAmount: { nested: true },
		});
		expect(normalized.goodsAmount).toBe("30000");
		expect(normalized.shippingAmount).toBe("");
		expect(normalized.taxAmount).toBe("");
	});

	it("整包不是物件時給一份乾淨的空草稿而不是丟例外", () => {
		expect(normalizeVendorResponseDraft(null)).toEqual(
			emptyVendorResponseDraft(),
		);
		expect(normalizeVendorResponseDraft("nope")).toEqual(
			emptyVendorResponseDraft(),
		);
	});
});

describe("checkDraftAmountBalance", () => {
	function withAmounts(
		goods: string,
		shipping: string,
		tax: string,
		total: string,
	): VendorResponseDraft {
		return {
			...emptyVendorResponseDraft(),
			goodsAmount: goods,
			shippingAmount: shipping,
			taxAmount: tax,
			totalAmount: total,
		};
	}

	it("相等時 balanced", () => {
		expect(
			checkDraftAmountBalance(withAmounts("30000", "800", "1540", "32340")),
		).toBe("balanced");
	});

	it("小數不得被 binary64 判成不相等", () => {
		// Number 版本會算出 0.30000000000000004，喊出一個根本不存在的不一致。
		expect(checkDraftAmountBalance(withAmounts("0.1", "0.2", "0", "0.3"))).toBe(
			"balanced",
		);
	});

	it("小數位數不同也要對齊比較", () => {
		expect(
			checkDraftAmountBalance(withAmounts("100.50", "9.5", "0", "110")),
		).toBe("balanced");
	});

	it("不相等時 unbalanced（只是提示，不擋儲存）", () => {
		expect(
			checkDraftAmountBalance(withAmounts("30000", "800", "1540", "30000")),
		).toBe("unbalanced");
	});

	it("有任何一欄還沒填就是 unknown，不替業者把空字串當 0", () => {
		expect(checkDraftAmountBalance(withAmounts("30000", "", "", "30000"))).toBe(
			"unknown",
		);
		expect(checkDraftAmountBalance(emptyVendorResponseDraft())).toBe("unknown");
	});

	it("不是合法 decimal 就是 unknown，不硬解析", () => {
		expect(
			checkDraftAmountBalance(withAmounts("三萬", "800", "1540", "32340")),
		).toBe("unknown");
		expect(
			checkDraftAmountBalance(withAmounts("30,000", "800", "1540", "32340")),
		).toBe("unknown");
	});
});

/**
 * 日期時間欄的格式必須與**平台賣家 App 同一條規則**，不是自己寫一套寬鬆版。
 *
 * 下面這條 regex 是從 `hawkr_seller`
 * `lib/features/procurement/presentation/seller_quote_validation.dart:91` 的
 * `isRfc3339()` 逐字抄過來的第二份副本，刻意不 import 本檔的實作——本檔要是把
 * 規則放寬（例如允許空白分隔），這裡才會轉紅。兩份一起改才算改對。
 */
const SELLER_APP_RFC3339 =
	/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

describe("isRfc3339", () => {
	it("Z 與 ±hh:mm 兩種位移都收——平台側就是兩種都收", () => {
		expect(isRfc3339("2026-09-18T08:00:00Z")).toBe(true);
		expect(isRfc3339("2026-09-18T16:00:00+08:00")).toBe(true);
		expect(isRfc3339("2026-09-18T16:00:00.123+08:00")).toBe(true);
		expect(isRfc3339("2026-09-18T04:00:00-04:00")).toBe(true);
	});

	it("沒有位移、用空白分隔、只有日期，一律不合法", () => {
		// 這三種正是原本自由文字欄位會產出的東西。
		expect(isRfc3339("2026-09-18T16:00:00")).toBe(false);
		expect(isRfc3339("2026-09-18 16:00")).toBe(false);
		expect(isRfc3339("2026-09-18")).toBe(false);
		expect(isRfc3339("9/20 早上")).toBe(false);
		expect(isRfc3339("")).toBe(false);
	});

	it("月份超出 01-12 擋掉——這一項比平台側嚴，是刻意的", () => {
		// 實測 Dart：`DateTime.tryParse("2026-13-45T00:00:00Z")` 會進位成
		// 2027-02-14 而不是回 null，所以賣家 App 那支其實放行它。這裡不跟。
		// 會出事的方向是前端比平台鬆（草稿存得進、提交被擋）；嚴一點只會對
		// 選擇器根本產不出來的值多提示一次。
		expect(isRfc3339("2026-13-45T00:00:00Z")).toBe(false);
	});

	it("日期進位（2026-02-31）兩邊都放行，不自己多發明一條規則", () => {
		// 實測 Dart 與 V8 都把它讀成 3/3。選擇器產不出這個值，這條只是把
		// 「為什麼不擋」釘住，免得日後有人順手加驗證造出新的分歧。
		expect(isRfc3339("2026-02-31T09:00:00+08:00")).toBe(true);
	});
});

describe("taipeiInputToRfc3339", () => {
	it("選擇器產出的字串必須通過平台賣家 App 那條 regex，位移固定台北", () => {
		const produced = taipeiInputToRfc3339("2026-09-20T09:00");
		expect(SELLER_APP_RFC3339.test(produced)).toBe(true);
		expect(isRfc3339(produced)).toBe(true);
		// 整串都是定值：位移不跟著 runtime 時區跑，所以這條在哪個時區都一樣。
		expect(produced).toBe("2026-09-20T09:00:00+08:00");
	});

	it("帶秒的選擇器值也保留秒", () => {
		expect(taipeiInputToRfc3339("2026-09-20T09:00:30")).toBe(
			"2026-09-20T09:00:30+08:00",
		);
	});

	it("被 isRfc3339 擋掉的形狀原樣回傳，不硬組一個字串出來", () => {
		// 月份 13 過不了 isRfc3339，就退回原字串讓業者看到自己填了什麼。
		expect(taipeiInputToRfc3339("2026-13-01T09:00")).toBe("2026-13-01T09:00");
	});

	it("空字串維持空字串，不生出一個「現在」", () => {
		expect(taipeiInputToRfc3339("")).toBe("");
		expect(taipeiInputToRfc3339("   ")).toBe("");
	});

	it("不是選擇器形狀的輸入原樣回傳，不猜", () => {
		// 猜錯會把業者填的東西改成別的時間，比留著錯字更糟。
		expect(taipeiInputToRfc3339("9/20 早上")).toBe("9/20 早上");
	});
});

describe("rfc3339ToTaipeiInput", () => {
	it("與 taipeiInputToRfc3339 對分鐘精度來回一致", () => {
		const roundTripped = rfc3339ToTaipeiInput(
			taipeiInputToRfc3339("2026-09-20T09:00"),
		);
		expect(roundTripped).toBe("2026-09-20T09:00");
	});

	it("UTC 寫法的同一個 instant 要讀成台北牆上時間，不是 UTC 的鐘點", () => {
		// 後端／平台側可能送 Z 那種寫法，選擇器要顯示業者當地看到的 16:00。
		expect(rfc3339ToTaipeiInput("2026-09-18T08:00:00Z")).toBe(
			"2026-09-18T16:00",
		);
	});

	it("不是 RFC3339 就回空字串，讓選擇器保持空白", () => {
		// ⛔ 回空字串**不代表**呼叫端可以把欄位清掉：舊值要留在草稿裡原樣顯示。
		expect(rfc3339ToTaipeiInput("9/20 早上")).toBe("");
		expect(rfc3339ToTaipeiInput("2026-09-18 16:00")).toBe("");
		expect(rfc3339ToTaipeiInput("")).toBe("");
	});
});
