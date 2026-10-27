import { describe, expect, it } from "vitest";
import {
	classifyDelegatedLoadFailure,
	delegatedBatchFailureText,
	delegatedFailureCode,
	delegatedProductLoadFailureCopy,
} from "./load-failure";

/**
 * 讀取失敗的分類（P1-4）。
 *
 * 這裡守的不是「有沒有分支」，是**兩個語意不同的失敗不得共用同一個畫面**：
 * - 9275（找不到／不屬於本店）重試一萬次都不會有結果 ⇒ 不得畫「重新載入」
 * - 上游暫時掛掉只要再按一次 ⇒ 必須畫「重新載入」，而且不得說「找不到這個商品」
 *
 * 以及一條與後端同批變更綁在一起的不變式：**判別只能看業務碼**。9280 正在從 502
 * 改成 424，任何以 HTTP status 為錨的判斷都會在那天靜默改變意義。
 */

/** `delegatedFetch` 丟出的形狀（`ApiClientError`：message ＋ status ＋ code）。 */
function apiError(code: number | string | undefined, status: number): Error {
	const error = new Error(
		code == null
			? `代行請求失敗（HTTP ${status}）`
			: `代行請求失敗（HTTP ${status}／代碼 ${code}）`,
	);
	return Object.assign(error, { status, code });
}

describe("classifyDelegatedLoadFailure", () => {
	it("9275 是「找不到／不屬於本店」，與暫時性失敗分開", () => {
		expect(classifyDelegatedLoadFailure(apiError(9275, 404))).toBe("not-owned");
	});

	it("9262 是授權不足，走共用的授權空狀態", () => {
		expect(classifyDelegatedLoadFailure(apiError(9262, 403))).toBe("denied");
	});

	it("上游不可用一律 temporary：9280 與 BFF 的字串碼 UPSTREAM_FAILURE", () => {
		expect(classifyDelegatedLoadFailure(apiError(9280, 502))).toBe("temporary");
		expect(
			classifyDelegatedLoadFailure(apiError("UPSTREAM_FAILURE", 502)),
		).toBe("temporary");
	});

	it("逾時／斷線／沒有碼的錯誤也是 temporary，不得被講成「找不到」", () => {
		expect(classifyDelegatedLoadFailure(new Error("network down"))).toBe(
			"temporary",
		);
		expect(classifyDelegatedLoadFailure(apiError(undefined, 504))).toBe(
			"temporary",
		);
		expect(classifyDelegatedLoadFailure(null)).toBe("temporary");
	});

	/**
	 * 後端同批正在把 9280 從 502 改成 424。分類必須完全不受影響 —— 這一條就是那個
	 * 變更的迴歸守衛：同一個碼配不同的 status，結論必須一致。
	 */
	it("同一個業務碼配不同 HTTP status，分類不變（status 不得參與判別）", () => {
		for (const status of [400, 403, 404, 424, 500, 502, 504]) {
			expect(classifyDelegatedLoadFailure(apiError(9280, status))).toBe(
				"temporary",
			);
			expect(classifyDelegatedLoadFailure(apiError(9275, status))).toBe(
				"not-owned",
			);
		}
	});

	it("delegatedFailureCode 讀得到數字碼與字串碼，其餘回 null", () => {
		expect(delegatedFailureCode(apiError(9275, 404))).toBe(9275);
		expect(delegatedFailureCode(apiError("UPSTREAM_FAILURE", 502))).toBe(
			"UPSTREAM_FAILURE",
		);
		expect(delegatedFailureCode(new Error("boom"))).toBe(null);
		expect(delegatedFailureCode(undefined)).toBe(null);
	});
});

describe("delegatedProductLoadFailureCopy", () => {
	it("找不到：沒有重試出口（重試必然再失敗，畫了就是騙人再按一次）", () => {
		const copy = delegatedProductLoadFailureCopy("not-owned");
		expect(copy.retryable).toBe(false);
		expect(copy.title).toBe("找不到這個商品");
	});

	it("暫時性：有重試出口，且標題不得宣稱「找不到」", () => {
		const copy = delegatedProductLoadFailureCopy("temporary");
		expect(copy.retryable).toBe(true);
		expect(copy.title).not.toContain("找不到");
		// 「資料沒有變動」是操作者此刻最需要知道的一件事。
		expect(copy.hint).toContain("沒有任何變動");
	});

	it("兩種文案都不得出現 HTTP 狀態碼或上游技術碼", () => {
		for (const kind of ["not-owned", "temporary"] as const) {
			const copy = delegatedProductLoadFailureCopy(kind);
			const text = `${copy.title}${copy.hint}`;
			expect(text).not.toMatch(/HTTP/u);
			expect(text).not.toMatch(/UPSTREAM_FAILURE/u);
			expect(text).not.toMatch(/代碼\s*\d/u);
			expect(text).not.toMatch(/\b(?:4\d\d|5\d\d|9\d{3})\b/u);
		}
	});
});

/**
 * 批次逐項失敗的碼 → 人話。
 *
 * dev 實測畫面上是「2 筆中 0 成功、2 失敗（876659813789762327：代碼 9260、
 * 872854281232616611：代碼 9260）」——`9260` 對操作者不是資訊。而三個最常見的碼
 * 各自對應完全不同的下一步（補欄位／重新載入／回 ADM），共用一個裸碼等於三種情況
 * 都答不出來。
 */
describe("delegatedBatchFailureText", () => {
	it("已登錄的碼一律換成人話，不留碼值", () => {
		for (const code of [9260, 9262, 9266, 9273, 9275, 9276, 9279, 9280]) {
			const text = delegatedBatchFailureText(code);
			expect(text).not.toMatch(/\d/u);
			expect(text.length).toBeGreaterThan(0);
		}
	});

	it("三個常見碼講出來的下一步互不相同", () => {
		const texts = [9260, 9266, 9262].map(delegatedBatchFailureText);
		expect(new Set(texts).size).toBe(3);
	});

	it("9266 要講得出「版本」，操作者才知道去重新載入而不是再按一次", () => {
		expect(delegatedBatchFailureText(9266)).toContain("版本");
	});

	/**
	 * 批次是唯一拿得到逐項碼的路徑。把陌生碼吞成「未知錯誤」會讓後端新增的碼在畫面上
	 * 完全無跡可循 —— 這裡刻意保留碼值當最後線索。
	 */
	it("未登錄的碼原樣帶出碼值，不吞成「未知錯誤」", () => {
		expect(delegatedBatchFailureText(9281)).toBe("代碼 9281");
		expect(delegatedBatchFailureText(1234)).toBe("代碼 1234");
	});
});
