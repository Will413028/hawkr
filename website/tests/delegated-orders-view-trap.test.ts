import assert from "node:assert/strict";
import test from "node:test";
import { createOrderRepo } from "@/lib/orders/repo";

/**
 * 代行的訂單 BFF 只收 `page` / `page_size` / `status` / `keyword` /
 * `date_from` / `date_to`
 * （app/(delegated)/adm/api/orders/[storeId]/route.ts 的 LIST_QUERY），
 * 不在白名單的 key 會被 `buildQuery` 判 INVALID_QUERY 回 400、一列都不轉送。
 *
 * 而代行綁的正是 `createOrderRepo`（lib/seller-data/delegated-repos.ts 的 `orders:`），
 * 它照樣會送 `view`。今天不會炸，只因為代行工作區的頁面都到不了 `scope.repos.orders`
 * —— 那是「目前不可達」，不是「已經處理好」。`lib/hooks/useOrders.ts` 有三處把 view
 * 直送 scope.repos.orders 且沒有 mode 守門。
 *
 * 這支測試把那個落差釘住：它斷言的是**現況為真**（view 會被送出），所以哪天有人
 * 讓代行實作真的忽略 view、或把 view 加進 BFF 白名單，這支就會轉紅並提醒改註解。
 * 反過來，若有人在代行頁面接上 useOrders 的那三處之一，這支測試的存在會讓
 * code review 立刻看到「送出去會 400」。
 */
test("createOrderRepo 會把 view 送進 query —— 代行 BFF 白名單不收它", async () => {
	const seen: string[] = [];
	const fetcher = async (url: string) => {
		seen.push(url);
		return { data: { records: [], total: "0", page: "1", pageSize: "20" } };
	};

	const repo = createOrderRepo(fetcher as never, {
		root: "/orders/2685914682",
	});
	await repo.fetchOrders({ view: "seller" });

	assert.equal(seen.length, 1, "應該只打一次");
	const qs = new URL(seen[0], "http://x.local").searchParams;

	assert.equal(
		qs.get("view"),
		"seller",
		"createOrderRepo 不再送 view —— 若這是刻意的，請同步改掉 lib/orders/repo.ts " +
			"的 OrderRepo 介面註解（它現在寫著「代行實作並沒有忽略 view」）",
	);

	// BFF 白名單（LIST_QUERY）逐字對照。多一個 key 就是 400。
	const allowed = new Set([
		"page",
		"page_size",
		"status",
		"keyword",
		"date_from",
		"date_to",
	]);
	const rejected = [...qs.keys()].filter((k) => !allowed.has(k));
	assert.deepEqual(
		rejected,
		["view"],
		"送出的 query 中不被代行 BFF 接受的 key 變了 —— 白名單或 repo 有一邊改過，" +
			"兩邊要一起看：app/(delegated)/adm/api/orders/[storeId]/route.ts 的 LIST_QUERY",
	);
});
