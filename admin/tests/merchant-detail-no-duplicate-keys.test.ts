import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 商家詳情頁：同一個父層下的兄弟元件不得用同一個 key（React 會警告
 * "Encountered two children with the same key"，並可能丟掉或重複其中一個）。
 * 換一家店的狀態重置由外層 `<MerchantDetail key={params.id}>` 負責，
 * 內層元件不需要再用 store.accountId 當 key。
 */
const PAGE = "src/app/admin/merchants/[id]/page.tsx";

test("商家詳情頁不把 store.accountId 當成兄弟元件的 key", async () => {
	const src = await readFile(PAGE, "utf8");
	assert.doesNotMatch(src, /key=\{store\.accountId\}/u);
});

test("換店重置仍由外層 MerchantDetail 的 key 負責", async () => {
	const src = await readFile(PAGE, "utf8");
	assert.match(src, /<MerchantDetail key=\{params\.id\}/u);
});
