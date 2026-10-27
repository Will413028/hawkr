import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
	type CheckoutDraft,
	readCheckoutDraft,
	writeCheckoutDraft,
} from "./draft";

/**
 * 草稿是購物車與結帳頁之間**唯一**的載體，而 `writeCheckoutDraft` 是把物件拆開
 * 重組後才落地的（PII 拆到 sessionStorage）。逐欄重組正是欄位靜默消失的那個形狀
 * ——`lib/checkout/repo.ts` 的 `invoice` 2026-09-01 就是這樣掉的：畫面正常、
 * 型別正常、測試全綠，只有 wire body 少一欄。
 */
function memoryStorage() {
	const store = new Map<string, string>();
	return {
		getItem: (k: string) => store.get(k) ?? null,
		setItem: (k: string, v: string) => {
			store.set(k, v);
		},
		removeItem: (k: string) => {
			store.delete(k);
		},
	};
}

const HOME_DRAFT: CheckoutDraft = {
	version: 2,
	createdAt: 1_757_000_000_000,
	validateAt: 1_757_000_000_000,
	seller: { key: "2685914682", name: "測試賣家" },
	selectedSkuIds: ["101"],
	shipping: {
		method: "3001",
		deliveryTemp: "3202",
		label: "黑貓宅急便",
		fee: 150,
	},
	recipient: {
		id: "843856384063471760",
		name: "王小明",
		phone: "0912345678",
		line: "臺北市中正區重慶南路一段1號",
	},
	note: "",
};

beforeEach(() => {
	const g = globalThis as Record<string, unknown>;
	g.window = {
		localStorage: memoryStorage(),
		sessionStorage: memoryStorage(),
	};
});

afterEach(() => {
	delete (globalThis as Record<string, unknown>).window;
});

describe("CheckoutDraft 的逐欄重組", () => {
	it("配送方式撐過 write → read", () => {
		writeCheckoutDraft(HOME_DRAFT);

		expect(readCheckoutDraft()?.shipping).toEqual(HOME_DRAFT.shipping);
	});

	it("收件地址 id 撐得過去", () => {
		writeCheckoutDraft(HOME_DRAFT);

		// `recipient` 走 sessionStorage 這條分支（PII 不進 localStorage），
		// 與 shipping 不同路 ⇒ 兩者要各自斷言。
		expect(String(readCheckoutDraft()?.recipient?.id)).toBe(
			"843856384063471760",
		);
	});
});
