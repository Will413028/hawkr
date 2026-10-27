import { describe, expect, it } from "vitest";
import { OrderCancelByRole, OrderStatus } from "@/lib/codemeta/enums";
import {
	delegatedOrdersEmptyCopy,
	formatCancelNote,
	isTerminalOrderStatus,
} from "./display";
import { EMPTY_DELEGATED_ORDER_FILTERS } from "./filters";

/**
 * 終態訂單曾在列表每一列與詳情卡上被宣告「僅賣家可出貨／出貨、取號、叫件與確認寄出
 * 僅能由賣家帳號執行」。那句話對一筆已取消的訂單是雙重錯誤：CTA 不是因為權限才沒有，
 * 而且賣家本人也出不了貨 —— 正是 §8.1.1「不要在畫面上宣告這裡有出貨功能只是你沒權限」
 * 要防的形狀。判準抽在這裡，兩個呼叫端（列表列、詳情卡）共用同一份。
 */
describe("isTerminalOrderStatus", () => {
	it("CodeMeta 標了（終態）的三個為真", () => {
		expect(isTerminalOrderStatus(OrderStatus.COMPLETED)).toBe(true);
		expect(isTerminalOrderStatus(OrderStatus.FULLY_REFUNDED)).toBe(true);
		expect(isTerminalOrderStatus(OrderStatus.CANCELLED)).toBe(true);
	});

	it("還走得下去的狀態為假", () => {
		for (const status of [
			OrderStatus.PENDING_PAYMENT,
			OrderStatus.PAID,
			OrderStatus.PENDING_SHIPMENT,
			OrderStatus.IN_TRANSIT,
			OrderStatus.WAITING_PICKUP,
			OrderStatus.SHIPPING_ABNORMAL,
			OrderStatus.DELIVERED,
		]) {
			expect(isTerminalOrderStatus(status)).toBe(false);
		}
	});

	it("部分退款不是終態 —— 它的碼比已完成大，用大小比較會判錯", () => {
		expect(Number(OrderStatus.PARTIALLY_REFUNDED)).toBeGreaterThan(
			Number(OrderStatus.COMPLETED),
		);
		expect(isTerminalOrderStatus(OrderStatus.PARTIALLY_REFUNDED)).toBe(false);
	});
});

/**
 * 取消說明要與賣家中心詳情頁讀到的是同一句話：代行者與賣家對同一筆訂單溝通時，
 * 兩邊看到不同措辭就對不上。
 */
describe("formatCancelNote", () => {
	it("系統排程取消：角色 ＋ 原因", () => {
		expect(formatCancelNote(OrderCancelByRole.SYSTEM, "9101", "")).toBe(
			"系統自動取消訂單：付款逾時自動取消",
		);
	});

	it("賣家取消：角色 ＋ 原因 ＋ 備註", () => {
		expect(
			formatCancelNote(OrderCancelByRole.SELLER, "9102", "QA驗證-新碼9205"),
		).toBe("賣家取消訂單：出貨逾時自動取消：QA驗證-新碼9205");
	});

	it("買家自願取消：兩欄皆 NULL，仍要說得出是誰取消的", () => {
		expect(formatCancelNote(OrderCancelByRole.BUYER, "", "")).toBe(
			"買家取消訂單",
		);
	});

	it("角色缺漏時退回中性描述，不編造角色", () => {
		expect(formatCancelNote("", "", "")).toBe("訂單已取消");
	});

	it("未登錄的原因碼原樣顯示，不吃掉它", () => {
		expect(formatCancelNote(OrderCancelByRole.SELLER, "9999", "")).toBe(
			"賣家取消訂單：9999",
		);
	});

	it("備註與原因文字相同時不重複一次", () => {
		expect(formatCancelNote(OrderCancelByRole.SELLER, "9202", "商品缺貨")).toBe(
			"賣家取消訂單：商品缺貨",
		);
	});

	/**
	 * 後端系統取消把「角色＋原因」再抄一次進備註（`SellerCancelService:220`
	 * ＝ `"系統自動取消：" + label`），dev 實測整句是
	 * 「系統自動取消訂單：出貨逾時自動取消：系統自動取消：出貨逾時自動取消」。
	 * 「備註 === 原因」擋不住它 —— 備註是超集不是相等。
	 */
	it("備註是「角色：原因」的複述時整段丟掉", () => {
		expect(
			formatCancelNote(
				OrderCancelByRole.SYSTEM,
				"9102",
				"系統自動取消：出貨逾時自動取消",
			),
		).toBe("系統自動取消訂單：出貨逾時自動取消");
	});

	it("備註只複述原因、順序反過來也一樣丟掉", () => {
		expect(
			formatCancelNote(
				OrderCancelByRole.SYSTEM,
				"9101",
				"付款逾時自動取消；系統自動取消",
			),
		).toBe("系統自動取消訂單：付款逾時自動取消");
	});

	it("備註帶了新資訊時一個字都不吃掉", () => {
		expect(
			formatCancelNote(
				OrderCancelByRole.SELLER,
				"9102",
				"颱風停止出貨，已與買家溝通",
			),
		).toBe("賣家取消訂單：出貨逾時自動取消：颱風停止出貨，已與買家溝通");
	});

	it("備註混了複述與新資訊時只留新的那一段", () => {
		expect(
			formatCancelNote(
				OrderCancelByRole.SELLER,
				"9202",
				"賣家取消訂單：商品缺貨：買家同意改期",
			),
		).toBe("賣家取消訂單：商品缺貨：買家同意改期");
	});
});

/**
 * 零筆訂單有兩種完全不同的意思，不得共用一個畫面。
 *
 * dev 實測：這家店「全部訂單 81」，篩「部分退款」零命中，畫面卻是變體預設的
 * 「目前沒有訂單／買家下單後會自動出現在這裡」—— 對讀畫面的平台管理員來說，那是在
 * 宣稱這家店還沒開始賣，而正確的下一步（清掉篩選）也就想不到。
 */
describe("delegatedOrdersEmptyCopy", () => {
	const noFilter = EMPTY_DELEGATED_ORDER_FILTERS;
	const withStatus = { ...noFilter, status: OrderStatus.PARTIALLY_REFUNDED };

	it("沒有篩選條件時回 null（讓變體預設文案接手）", () => {
		expect(delegatedOrdersEmptyCopy(noFilter)).toBe(null);
	});

	it("有篩選條件時不得宣稱這家店沒有訂單", () => {
		const copy = delegatedOrdersEmptyCopy(withStatus);
		expect(copy).not.toBe(null);
		const text = `${copy?.title}${copy?.description}`;
		expect(text).not.toContain("目前沒有訂單");
		expect(text).not.toContain("買家下單後會自動出現在這裡");
	});

	it("要講出是哪一個狀態沒有命中，並指出清除篩選這條出路", () => {
		const copy = delegatedOrdersEmptyCopy(withStatus);
		expect(copy?.description).toContain("部分退款");
		expect(copy?.description).toContain("清除");
	});

	it("每一個 OrderStatus 都拿得到文案，且都不得退回「沒有訂單」", () => {
		for (const status of Object.values(OrderStatus)) {
			const copy = delegatedOrdersEmptyCopy({ ...noFilter, status });
			expect(copy?.title).toBe("目前的篩選條件沒有符合的訂單");
			expect(copy?.description).not.toContain("目前沒有訂單");
		}
	});

	/**
	 * `status` 是空的、但關鍵字或日期把清單篩成零筆 —— 只看 `status` 的舊判準會在這裡
	 * 退回「目前沒有訂單」，也就是替目標店家宣稱它還沒開始賣。這幾條把新的兩個外殼
	 * 一起釘住。
	 */
	it("只打了關鍵字而零筆時，講出是哪個關鍵字沒命中", () => {
		const copy = delegatedOrdersEmptyCopy({ ...noFilter, keyword: "番茄" });
		expect(copy?.description).toContain("番茄");
		expect(copy?.description).not.toContain("目前沒有訂單");
	});

	it("只有空白的關鍵字不算篩選（送出去會命中全部，與使用者的認知相反）", () => {
		expect(delegatedOrdersEmptyCopy({ ...noFilter, keyword: "   " })).toBe(
			null,
		);
	});

	it("日期起訖各自單獨成立時都有講法", () => {
		expect(
			delegatedOrdersEmptyCopy({ ...noFilter, dateFrom: "2026-08-01" })
				?.description,
		).toContain("2026-08-01 之後");
		expect(
			delegatedOrdersEmptyCopy({ ...noFilter, dateTo: "2026-08-31" })
				?.description,
		).toContain("2026-08-31 之前");
		expect(
			delegatedOrdersEmptyCopy({
				...noFilter,
				dateFrom: "2026-08-01",
				dateTo: "2026-08-31",
			})?.description,
		).toContain("2026-08-01 至 2026-08-31");
	});

	it("多個條件同時成立時全部列出來（只講一個會讓人只清掉一個）", () => {
		const copy = delegatedOrdersEmptyCopy({
			status: OrderStatus.PARTIALLY_REFUNDED,
			keyword: "番茄",
			dateFrom: "2026-08-01",
			dateTo: "2026-08-31",
		});
		expect(copy?.description).toContain("部分退款");
		expect(copy?.description).toContain("番茄");
		expect(copy?.description).toContain("2026-08-01 至 2026-08-31");
	});
});
