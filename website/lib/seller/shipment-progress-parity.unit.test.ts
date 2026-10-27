import { describe, expect, it } from "vitest";
import { LGS_TYPE, progressPercentFor } from "./shipment";

/**
 * 出貨進度百分比的**跨 repo 對拍**。
 *
 * 🔴為什麼需要它：同一張 (PayUni 貨態碼 → 進度百分比) 對照表在本 repo 與
 * `hawkr_seller` 各有一份逐值相同的實作（website 的 `progressPercentFor`／
 * App 的 `LogisticsStatusCode.getProgressPercent`），而**沒有任何東西保證它們同步**。
 *
 * 兩份不會同時錯，只會**先後**錯：有人改了一邊，另一邊照舊，
 * 於是同一筆訂單在網頁與 App 上顯示不同的進度——而兩邊的測試都是綠的。
 *
 * ⚠️本 gate 擋不住什麼（誠實標明，別把它當成牆）：
 * digest 在兩個 repo 各硬寫一份，**它不會自動比對另一個 repo**。
 * 它能做到的是：改了邏輯而沒改 digest ⇒ 這一邊轉紅 ⇒ 那個人會看到下面這段註解，
 * 知道另一邊也要改。把「悄悄漂移」變成「明著改兩個 digest」。
 *
 * 🔴**改動這張表時，兩個 repo 的 digest 必須一起更新。**
 * 對應檔案：`hawkr_seller/test/features/order/progress_parity_test.dart`
 *
 * 判定紀錄（2026-09-10）：這兩份吃的是 `providerStatus`（PayUni 原生碼），
 * **不吃** normalized status ⇒ 物流履約 Foundation 新增的 7 個 normalized 值
 * 不會讓它們漂移 ⇒ 不是 Foundation correctness risk。本 gate 防的是未來的人為漂移。
 */

/** 涵蓋兩張表的每一個 case ＋ default 區間。 */
const CODES = [...Array.from({ length: 41 }, (_, i) => i), 99];

/** FNV-1a 32-bit，與 CodeMeta 同步契約測試用的同一支演算法。 */
function fnv1a32(input: string): string {
	let h = 0x811c9dc5;
	for (let i = 0; i < input.length; i++) {
		h ^= input.charCodeAt(i);
		h = Math.imul(h, 0x01000193) >>> 0;
	}
	return h.toString(16).padStart(8, "0");
}

function canonicalise(): string {
	const parts: string[] = [];
	for (const [label, axis] of [
		["HOME", LGS_TYPE.HOME],
		["C2C", LGS_TYPE.C2C],
	] as const) {
		for (const code of CODES) {
			parts.push(`${label}:${code}=${progressPercentFor(code, axis)}`);
		}
	}
	return parts.join(";");
}

describe("出貨進度百分比：跨 repo 對拍", () => {
	it("digest 與 seller App 的同一張表一致", () => {
		expect(
			fnv1a32(canonicalise()),
			"進度對照表改動了。hawkr_seller 的 " +
				"test/features/order/progress_parity_test.dart 必須同批更新，" +
				"否則同一筆訂單在網頁與 App 上會顯示不同的進度，而兩邊的測試都是綠的。",
		).toBe("0480fbc6");
	});

	it("未知配送軸回 null，不借用任何一張表的百分比", () => {
		for (const code of CODES) {
			expect(progressPercentFor(code, "B2C")).toBeNull();
			expect(progressPercentFor(code, "")).toBeNull();
		}
	});

	/** 測試自身的鑑別力：兩張表若產出完全相同的結果，本 gate 對「軸接錯」是盲的。 */
	it("兩張表必須真的不同（否則本 gate 對軸接錯完全無感）", () => {
		const home = CODES.map((c) => progressPercentFor(c, LGS_TYPE.HOME)).join(
			",",
		);
		const c2c = CODES.map((c) => progressPercentFor(c, LGS_TYPE.C2C)).join(",");
		expect(home).not.toBe(c2c);
	});
});
