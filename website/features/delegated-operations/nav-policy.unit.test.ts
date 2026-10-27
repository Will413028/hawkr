import { describe, expect, it } from "vitest";
import {
	SELLER_FOOTER_ITEMS,
	SELLER_NAV_GROUPS,
} from "@/features/seller-center/components/sellerNav";
import { DELEGATED_HIDDEN_HREFS, isDelegatedHiddenHref } from "./nav-policy";

/**
 * 賣家側欄與 ADM 代行側欄**共用同一份選單資料**（`sellerNav.tsx` 檔頭自陳「新增
 * 模組時代行側欄自動跟上」）⇒ 新增一個賣家選單項而不決定代行可見性，就是**默認
 * 對代行開放**，而且不編譯錯、不測試紅（58A §0.6 C08 / §B-2.0 T02）。
 *
 * 在本檔之前，`DELEGATED_HIDDEN_HREFS` 沒有任何測試覆蓋——這一條的「機制」只存在
 * 於規範文字裡。本檔把它變成 gate：每個賣家選單項都必須在下面兩份具名清單之一，
 * 新增一項而不做決定會直接紅。
 */

/** 明確判定「代行可以看到」的選單項。加進來＝做過決定。 */
const DELEGATED_VISIBLE_HREFS = new Set([
	"/seller-center/dashboard",
	"/seller-center/orders",
	"/seller-center/products",
	"/seller-center/inventory",
	"/seller-center/posts",
	"/seller-center/coupons",
	"/seller-center/customers",
	"/seller-center/insights",
	"/seller-center/me",
	"/seller-center/messages",
]);

const ALL_NAV_HREFS = [
	...SELLER_NAV_GROUPS.flatMap((group) => group.items.map((item) => item.href)),
	...SELLER_FOOTER_ITEMS.map((item) => item.href),
];

describe("delegated nav policy", () => {
	it("forces an explicit hidden/visible decision for every seller nav item", () => {
		const undecided = ALL_NAV_HREFS.filter(
			(href) =>
				!DELEGATED_HIDDEN_HREFS.has(href) && !DELEGATED_VISIBLE_HREFS.has(href),
		);
		expect(
			undecided,
			`新增賣家選單項時必須決定代行可見性（58A C08）。未決定就是默認對代行開放：\n${undecided.join("\n")}`,
		).toEqual([]);
	});

	it("hides the invoice module from the delegated workspace", () => {
		// 這一條不是取捨：`gateway/internal/router/seller_route_assert.go` 的
		// `delegatedPathForbiddenWords` 含 "invoice" ⇒ 代行 namespace 底下永遠掛不了
		// 發票路由，Gateway 會在開機時 panic。渲染那個選單項只會得到一個點進去
		// 必定空白的入口。
		expect(isDelegatedHiddenHref("/seller-center/invoices")).toBe(true);
		expect(isDelegatedHiddenHref("/seller-center/invoices/documents")).toBe(
			false,
		);
	});

	it("hides the seller-owned notification inbox from delegated operators", () => {
		expect(isDelegatedHiddenHref("/seller-center/notifications")).toBe(true);
		expect(
			isDelegatedHiddenHref("/seller-center/notifications/preferences"),
		).toBe(false);
	});

	it("keeps the pre-existing hidden entries", () => {
		expect(isDelegatedHiddenHref("/seller-center/wallet")).toBe(true);
		expect(isDelegatedHiddenHref("/seller-center/me/shipping")).toBe(true);
		expect(isDelegatedHiddenHref("/seller-center/preview")).toBe(true);
	});
});
