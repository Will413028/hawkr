import { describe, expect, it } from "vitest";
import {
	PAYUNI_LOCK_ON_SUBMIT_NOTICE,
	PAYUNI_LOCKED_NOTICE,
	payuniLockedNoticeFor,
	stripPayuniLockedFields,
} from "./payuni-locked-fields";

describe("payuniLockedNoticeFor", () => {
	// 鎖定由伺服器推導（flow.identity_locked：註冊已有 MerID 且綁定沒有失敗），client 只照它顯示。
	it("identity locked 時回「找 PayUni」那一句", () => {
		expect(payuniLockedNoticeFor(true)).toBe(PAYUNI_LOCKED_NOTICE);
	});

	it("沒鎖、或還沒讀到流程狀態時可以改", () => {
		expect(payuniLockedNoticeFor(false)).toBeUndefined();
		expect(payuniLockedNoticeFor(undefined)).toBeUndefined();
	});

	// 文案與 seller App 的常數逐字對齊，這裡釘住字面值——兩邊任一邊改動而另一邊沒跟上時，這格會紅。
	it("文案與 seller App 逐字一致", () => {
		expect(PAYUNI_LOCKED_NOTICE).toBe(
			"資料已送至 PayUni，店名與商店介紹無法修改，如需變更請聯繫 PayUni",
		);
		expect(PAYUNI_LOCK_ON_SUBMIT_NOTICE).toBe(
			"送出後商店名稱與商店介紹將無法修改，請先確認無誤",
		);
	});
});

describe("stripPayuniLockedFields", () => {
	it("鎖定時把兩個欄位從變更拿掉，其餘不動", () => {
		const changes = {
			storeName: "改過的店名",
			description: "改過的介紹",
			contactPhone: "0912345678",
			address: { townCode: "63000010", addressDetail: "信義路 1 號" },
		};
		expect(stripPayuniLockedFields(changes, true)).toEqual({
			contactPhone: "0912345678",
			address: { townCode: "63000010", addressDetail: "信義路 1 號" },
		});
	});

	it("未鎖定時原樣回傳", () => {
		const changes = { storeName: "改過的店名", prize: "金牌" };
		expect(stripPayuniLockedFields(changes, false)).toEqual(changes);
	});

	// 只改了鎖定欄位時要回空物件，呼叫端才有辦法判斷「沒有東西要送」而不是送一個必定被擋掉的請求。
	it("只有鎖定欄位變動時回空物件", () => {
		expect(stripPayuniLockedFields({ storeName: "改過的店名" }, true)).toEqual(
			{},
		);
	});

	it("不就地改寫呼叫端的物件", () => {
		const changes = { storeName: "改過的店名", prize: "金牌" };
		stripPayuniLockedFields(changes, true);
		expect(changes.storeName).toBe("改過的店名");
	});
});
