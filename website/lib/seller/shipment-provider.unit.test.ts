import { describe, expect, it } from "vitest";
import {
	formatShippingMethodLabel,
	normalizeShippingMethod,
	SHIPPING_METHOD_LABEL,
	ShippingMethod,
} from "@/lib/codemeta/enums";
import {
	isRunnableShipmentAction,
	LGS_TYPE,
	lgsTypeForShippingMethod,
	nextShipmentAction,
	progressPercentFor,
	type ShipmentLike,
} from "@/lib/seller/shipment";

/**
 * 回歸測試：物流 provider 的二元 fallback（65A §1.10 ／ §12.1 類型 A）。
 *
 * 舊行為是「不是黑貓就當 7-11」——`isTaqbin ? … : …`。它對 `3003`（已刪除的 Uber）、
 * `3004` 7-11 大宗寄倉、`3005` 商家自送回的都是**合法值**：型別對、不拋例外、
 * 當時全部測試都是綠的，畫面卻把別人家的物流畫成 7-11，而且按鈕按下去會真的
 * 打 `POST /app/shipments/{no}/print_cvs_label`。
 *
 * 🔴 因此本檔的斷言一律是「**不得**等於已知 provider 的答案」，而不是「等於某個
 * 新文案」——後者換個文案就綠了，擋不住 fallback 回歸。
 *
 * 值域取自 CodeMeta 生成檔（`lib/codemeta/generated.ts`），不在本檔手寫碼值。
 */

/** 前端沒有實作的三個通路（Uber 已刪除，值域仍在 CodeMeta）。 */
const NEW_PROVIDERS = [
	ShippingMethod.UBER_INSTANT,
	ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE,
	ShippingMethod.MERCHANT_DELIVERY,
] as const;

/** 連 CodeMeta 都沒有的值：後端未來新增、或髒資料。 */
const UNKNOWN_VALUES = ["", "3999", "SOME_NEW_CARRIER", "宅急便"] as const;

function shipping(method: string, extra: Partial<ShipmentLike> = {}) {
	return { method, ...extra } satisfies ShipmentLike;
}

describe("normalizeShippingMethod", () => {
	it("CodeMeta 的五個 value 全部認得（含本波新增的 3004 / 3005）", () => {
		for (const value of Object.values(ShippingMethod)) {
			expect(normalizeShippingMethod(value)).toBe(value);
		}
	});

	it("歷史別名正規化到對應的 CodeMeta value", () => {
		expect(normalizeShippingMethod("TAQBIN")).toBe(ShippingMethod.TAQBIN);
		expect(normalizeShippingMethod("home")).toBe(ShippingMethod.TAQBIN);
		expect(normalizeShippingMethod("宅配")).toBe(ShippingMethod.TAQBIN);
		expect(normalizeShippingMethod("CVS_7_11")).toBe(
			ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
		);
	});

	it("[禁 fallback] 認不得就回 undefined，不得落到任何既有 provider", () => {
		for (const raw of UNKNOWN_VALUES) {
			expect(normalizeShippingMethod(raw)).toBeUndefined();
		}
	});
});

describe("formatShippingMethodLabel", () => {
	it("[禁 fallback] 空值不得憑空捏造一個物流商（舊版回「宅配」）", () => {
		for (const empty of ["", undefined, null, "   "]) {
			const label = formatShippingMethodLabel(empty);
			expect(label).toBe("—");
			expect(Object.values(SHIPPING_METHOD_LABEL)).not.toContain(label);
		}
	});

	it("認得的碼查 CodeMeta 標籤；3004 不得顯示成 7-11 店到店", () => {
		expect(formatShippingMethodLabel(ShippingMethod.TAQBIN)).toBe(
			SHIPPING_METHOD_LABEL[ShippingMethod.TAQBIN],
		);
		expect(
			formatShippingMethodLabel(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE),
		).toBe(SHIPPING_METHOD_LABEL[ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE]);
		expect(
			formatShippingMethodLabel(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE),
		).not.toBe(SHIPPING_METHOD_LABEL[ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP]);
	});

	it("認不得的碼照顯原碼，不猜也不隱藏", () => {
		expect(formatShippingMethodLabel("3999")).toBe("3999");
		expect(formatShippingMethodLabel("SOME_NEW_CARRIER")).toBe(
			"SOME_NEW_CARRIER",
		);
	});
});

describe("lgsTypeForShippingMethod", () => {
	it("已知兩軸各自對到自己的 PayUni 軸", () => {
		expect(lgsTypeForShippingMethod(ShippingMethod.TAQBIN)).toBe(LGS_TYPE.HOME);
		expect(
			lgsTypeForShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP),
		).toBe(LGS_TYPE.C2C);
	});

	it("[禁 fallback] 新通路與未知值一律回 null，不得回 C2C", () => {
		for (const method of [...NEW_PROVIDERS, ...UNKNOWN_VALUES, undefined]) {
			expect(lgsTypeForShippingMethod(method)).toBeNull();
		}
	});
});

describe("progressPercentFor", () => {
	// PayUni 原始 shipStatus：11 在超商表是 80%（到店），在宅配表不存在。
	const code = 11;

	it("兩條已知軸各自有自己的百分比", () => {
		expect(progressPercentFor(code, LGS_TYPE.C2C)).toBe(80);
		expect(progressPercentFor(code, LGS_TYPE.HOME)).toBe(50);
	});

	it("[禁 fallback] B2C / C2B / 空字串 / 未知軸不得沿用 C2C 的百分比", () => {
		for (const axis of [LGS_TYPE.B2C, LGS_TYPE.C2B, "", "WHATEVER"]) {
			const percent = progressPercentFor(code, axis);
			expect(percent).toBeNull();
			expect(percent).not.toBe(progressPercentFor(code, LGS_TYPE.C2C));
		}
	});
});

describe("nextShipmentAction", () => {
	it("黑貓（含別名）走黑貓分支", () => {
		for (const method of [ShippingMethod.TAQBIN, "TAQBIN", "home", "宅配"]) {
			const action = nextShipmentAction("O1", shipping(method), "preparing");
			expect(action.kind).toBe("tcat-reserve");
		}
	});

	it("7-11 店到店走超商分支", () => {
		expect(
			nextShipmentAction(
				"O1",
				shipping(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP),
				"preparing",
			).kind,
		).toBe("cvs-print");
		expect(
			nextShipmentAction(
				"O1",
				shipping(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, { odno: "A1" }),
				"preparing",
			).kind,
		).toBe("cvs-confirm");
	});

	it("[禁 fallback] 3003 / 3004 / 3005 不得跑出任何超商或黑貓的動作", () => {
		for (const method of NEW_PROVIDERS) {
			const action = nextShipmentAction("O1", shipping(method), "preparing");
			expect(action.kind).toBe("unsupported");
			expect(action.kind).not.toMatch(/^cvs-/);
			expect(action.kind).not.toMatch(/^tcat-/);
			if (action.kind === "unsupported") expect(action.method).toBe(method);
		}
	});

	it("[禁 fallback] 已取號的 odno 也不能讓未知 provider 冒出「出貨確認」", () => {
		// 這是舊行為最危險的一格：odno 是超商交貨便代碼，未知 provider 若帶了
		// 任何 odno（例如後端沿用同一欄位），舊版會直接畫出「出貨確認」按鈕。
		const action = nextShipmentAction(
			"O1",
			shipping(ShippingMethod.MERCHANT_DELIVERY, {
				odno: "A1",
				validationNo: "1234",
			}),
			"preparing",
		);
		expect(action.kind).toBe("unsupported");
	});

	it("[禁 fallback] 未知配送方式不是可執行動作（不得進批次列、不得算成待處理）", () => {
		for (const method of NEW_PROVIDERS) {
			const action = nextShipmentAction("O1", shipping(method), "preparing");
			expect(isRunnableShipmentAction(action)).toBe(false);
		}
		expect(
			isRunnableShipmentAction(
				nextShipmentAction(
					"O1",
					shipping(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP),
					"preparing",
				),
			),
		).toBe(true);
	});

	it("unsupported 與 none 是兩件事：沒單子時仍是 none", () => {
		expect(nextShipmentAction("O1", undefined, "preparing").kind).toBe("none");
		expect(
			nextShipmentAction("O1", shipping(ShippingMethod.TAQBIN), "shipping")
				.kind,
		).toBe("none");
	});
});
