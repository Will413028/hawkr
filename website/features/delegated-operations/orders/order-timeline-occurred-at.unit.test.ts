import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { formatOrderDateTime } from "./display";
import { createDelegatedOrderRepo } from "./repo";

/**
 * 訂單時間軸的最後一格 —— 「這筆訂單什麼時候完成／取消的」（backend#2471）。
 *
 * 後端曾對當下狀態一律送空字串，前端因此完全不渲染那一行；現在送達／完成／取消三個
 * 結案節點有自己的時間欄了。這一組守兩件互為對稱的事，兩件都是「畫面少一行、沒有任何
 * 錯誤訊息」的靜默失效：
 *   1. 後端送了時間，這一行要**畫得出來**（值一路走到渲染判準，中途沒被吃掉）
 *   2. 後端沒有權威時間戳時仍**不畫**，且不會被誤讀成「還在載入」
 *
 * ⚠️ vitest 的 environment 是 `node`，沒有 DOM 可以掛元件。所以第 1、2 點測的是元件
 * 真正用來決定畫不畫的那個判準（`formatOrderDateTime(entry.occurredAt)` 的真假），
 * 第 3 點則直接對原始碼斷言那個判準確實還接在渲染上 —— 否則這支測試會在元件改用別的
 * 判準之後繼續全綠。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const PANEL =
	"features/delegated-operations/orders/DelegatedOrderDetailPanel.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

function stubDetail(timeline: unknown) {
	return async () => ({ data: { orderNo: "O005001417", timeline } });
}

describe("代行訂單時間軸：結案節點的時間戳", () => {
	it("後端送的 occurredAt 原樣走到 DTO，不被解析吃掉", async () => {
		const repo = createDelegatedOrderRepo(
			"2685914682",
			stubDetail([
				{
					status: "0",
					statusLabel: "待付款",
					occurredAt: "2026-08-05T07:59:56Z",
				},
				{
					status: "80",
					statusLabel: "已取消（終態）",
					occurredAt: "2026-08-05T09:30:00Z",
				},
			]) as never,
		);
		const detail = await repo.detail("O005001417");
		expect(detail.timeline.map((e) => e.occurredAt)).toEqual([
			"2026-08-05T07:59:56Z",
			"2026-08-05T09:30:00Z",
		]);
	});

	it("有時間戳 → 判準為真 → 那一行畫得出來", () => {
		// 台北時間，實際字面交給 formatTaipeiDateTime；這裡只斷言它非空。
		expect(formatOrderDateTime("2026-08-05T09:30:00Z")).not.toBe("");
	});

	it("無時間戳 → 判準為假 → 那一行不畫（不是空白也不是 Invalid Date）", async () => {
		const repo = createDelegatedOrderRepo(
			"2685914682",
			stubDetail([
				{
					status: "0",
					statusLabel: "待付款",
					occurredAt: "2026-08-05T07:59:56Z",
				},
				{ status: "20", statusLabel: "待出貨", occurredAt: "" },
			]) as never,
		);
		const detail = await repo.detail("O005001417");
		const last = detail.timeline[detail.timeline.length - 1];
		expect(last.occurredAt).toBe("");
		expect(formatOrderDateTime(last.occurredAt)).toBe("");
	});

	it("那個判準確實還接在渲染上", () => {
		const src = read(PANEL);
		// 判準：算出來的 time 為真才渲染 __time；改成無條件渲染或換成別的來源即轉紅。
		expect(src).toMatch(
			/const time = formatOrderDateTime\(entry\.occurredAt\);/,
		);
		expect(src).toMatch(
			/time \? \(\s*<span className="adm-order-timeline__time">\{time\}<\/span>\s*\) : null/,
		);
	});

	/**
	 * 「沒有時間」與「還在載入」必須可分辨（#2471 的明確要求）。
	 *
	 * 兩者在畫面上都是「這裡少了東西」，靠的是它們**根本不會同時出現**：載入中整個面板
	 * 被一句話取代，時間軸連掛都沒掛上。所以要守的不變式是那個 early return 還在，且排
	 * 在時間軸之前 —— 一旦改成「面板照畫、只有內容區塊 loading」，少一行時間就會變成
	 * 兩種意思。
	 */
	it("載入中整個面板被取代，時間軸不會掛載", () => {
		const src = read(PANEL);
		const loadingAt = src.indexOf(
			'if (loading) return <p className="seller-empty">',
		);
		expect(loadingAt).toBeGreaterThan(-1);
		expect(src.indexOf("<Timeline order={order}")).toBeGreaterThan(loadingAt);
	});
});
