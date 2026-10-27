import assert from "node:assert/strict";
import { mock, test } from "node:test";

/**
 * Page-trap envelope 的解析（backend#2211 C8 之後的唯一形狀）。
 *
 * C8 把七支端點的 `records` / `total` / `page` / `pageSize` 收進 `data` 之下，
 * 三個數值欄一併字串化。**這是一次切，沒有相容層** —— 前端與後端同波上線，
 * 不保留舊形狀的讀取路徑。
 *
 * 這支測試釘的是兩件事：
 *   1. 讀的是 `data.records`（不是 `data.reviews`、不是 `data.data`、不是頂層）
 *   2. `total` / `page` / `pageSize` 是 string，出口必須是 number
 *
 * 第 2 點特別容易在重構時掉：型別上看起來已經是 number（回傳型別宣告如此），
 * 但拿掉 `Number()` 之後 TypeScript 不會抱怨（`unknown` 進、`number` 出全靠斷言），
 * 執行期就變成字串在跑，`total > 0` 之類的比較會靜默給出錯誤答案。
 *
 * 踩過的實例（2026-08-05）：`lib/reviews/repo.ts` 原本讀 `nested.reviews`、
 * `lib/visits/repo.ts` 原本讀 `env.data.data`，C8 上 dev 之後兩頁當場變空 ——
 * HTTP 200、無 console error，跟 #338 白畫面同一個形狀。
 */

const REVIEWS_ENVELOPE = {
	code: 0,
	message: "success",
	data: {
		records: [{ id: "r1" }],
		total: "7",
		page: "2",
		pageSize: "10",
		stats: {},
	},
};

async function withFetchStub<T>(
	payload: unknown,
	run: () => Promise<T>,
): Promise<T> {
	mock.module("@/lib/api/client", {
		namedExports: {
			authedFetch: async () => payload,
			// apiClient 是巢狀物件（apiClient.public.get），不是函式
			apiClient: { public: { get: async () => payload } },
		},
	});
	try {
		return await run();
	} finally {
		mock.reset();
	}
}

test("商品評論讀 data.records，且 string 數值欄出口為 number", async () => {
	const res = await withFetchStub(REVIEWS_ENVELOPE, async () => {
		const mod = await import(`../lib/reviews/repo?bust=${Math.random()}`);
		return mod.fetchProductReviews("871950679303556436", {
			page: 2,
			pageSize: 10,
		});
	});
	assert.equal(
		res.reviews.length,
		1,
		`應讀到 1 筆，實際 ${res.reviews.length} 筆`,
	);

	// 型別與值都要斷言：只斷言值的話，字串 "7" 會通過 `== 7`；只斷言型別的話，
	// 讀錯欄位拿到 undefined 再 Number() 會變 NaN 而 typeof 仍是 number。
	assert.equal(typeof res.total, "number", "total 應為 number");
	assert.equal(res.total, 7, "total 應為 7");
	assert.equal(typeof res.page, "number", "page 應為 number");
	assert.equal(res.page, 2, "page 應為 2");
	assert.equal(typeof res.pageSize, "number", "pageSize 應為 number");
	assert.equal(res.pageSize, 10, "pageSize 應為 10");
	assert.ok(Number.isFinite(res.total), "total 不得為 NaN");
});
