import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { Role } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import {
	KYCStatus,
	PayUniBindStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	ACCOUNT_LOOKUP_PAGE_SIZE,
	accountLookupRequest,
	collectAccountIds,
	FILTER_TABS,
	type FilterTab,
	filterFields,
	listStoresRequest,
	mergeStoreLists,
	parseFilterTab,
	planStoreSearch,
	textSearchRequest,
} from "../src/lib/store-search";

/**
 * 商家清單的篩選分頁 → ListStores 請求欄位（決策簡報 D4 第 1 點、K2、K4）與登入信箱的兩段查詢（K5）。
 * 這裡釘的是「哪個分頁送哪個欄位」；契約本身（欄位語意）由 proto 與後端測試守。
 */

test("篩選分頁對應的請求欄位（D4 第 1 點）", () => {
	const expected: Record<FilterTab, object> = {
		all: {},
		approved: { kycStatus: KYCStatus.KYC_STATUS_APPROVED },
		pending: { kycAwaitingReview: true },
		rejected: { kycStatus: KYCStatus.KYC_STATUS_REJECTED },
		rebind: { payuniBindStatus: PayUniBindStatus.FAILED },
		suspended: { suspended: true },
		closed: { status: StoreStatus.CLOSED },
	};
	for (const tab of FILTER_TABS) {
		assert.deepEqual(filterFields(tab), expected[tab], tab);
	}
});

test("「待審核」送 kycAwaitingReview（待辦的成員），不再送 kycStatus=SUBMITTED（伺服器拒絕它）", () => {
	const request = listStoresRequest({ tab: "pending", pageSize: 10 });
	assert.equal(request.kycAwaitingReview, true);
	assert.equal(request.kycStatus, undefined);
});

test("「全部」不送任何篩選欄位，也不送 UNSPECIFIED 的值", () => {
	const request = listStoresRequest({ tab: "all", pageSize: 10 });
	assert.deepEqual(request, { pageSize: 10 });
	assert.deepEqual(
		Object.keys(request).sort(),
		["pageSize"],
		"all 多送了欄位 ⇒ 清單被偷偷篩選",
	);
});

test("沒有值的 query、accountId、pageToken 不放進請求", () => {
	assert.deepEqual(
		listStoresRequest({
			tab: "pending",
			query: "",
			accountId: "",
			pageToken: "",
			pageSize: 20,
		}),
		{ pageSize: 20, kycAwaitingReview: true },
	);
	assert.deepEqual(
		listStoresRequest({
			tab: "closed",
			query: "甜心",
			accountId: "a1",
			pageToken: "tok",
			pageSize: 20,
		}),
		{
			pageSize: 20,
			status: StoreStatus.CLOSED,
			query: "甜心",
			accountId: "a1",
			pageToken: "tok",
		},
	);
});

test("網址的 filter：不認得的值（含舊版的 active、forwarded）回到全部", () => {
	assert.equal(parseFilterTab("suspended"), "suspended");
	assert.equal(parseFilterTab("pending"), "pending");
	for (const legacy of [
		"active",
		"forwarded",
		"forward_failed",
		"monthly",
		"",
	]) {
		assert.equal(parseFilterTab(legacy), "all", legacy);
	}
	assert.equal(parseFilterTab(null), "all");
});

test("搜尋字串分流：含 @ 的走信箱兩段查詢，其他走 ListStores 的 query", () => {
	assert.deepEqual(planStoreSearch("   "), { kind: "none" });
	assert.deepEqual(planStoreSearch(" 甜心 "), { kind: "text", query: "甜心" });
	assert.deepEqual(planStoreSearch("0912345678"), {
		kind: "text",
		query: "0912345678",
	});
	assert.deepEqual(planStoreSearch(" a@b.com "), {
		kind: "email",
		email: "a@b.com",
	});
});

test("信箱找帳號（K5）：完全相符的 email 加賣家角色", () => {
	assert.deepEqual(accountLookupRequest("a@b.com", ""), {
		email: "a@b.com",
		role: Role.SELLER,
		pageSize: ACCOUNT_LOOKUP_PAGE_SIZE,
	});
	assert.equal(accountLookupRequest("a@b.com", "t2").pageToken, "t2");
});

test("信箱找帳號：走完所有頁、去重複；沒有符合就是空", async () => {
	const pages: Record<
		string,
		{ accounts: { id: string }[]; nextPageToken: string }
	> = {
		"": { accounts: [{ id: "1" }, { id: "2" }], nextPageToken: "t2" },
		t2: { accounts: [{ id: "2" }, { id: "3" }], nextPageToken: "" },
	};
	const seen: string[] = [];
	const ids = await collectAccountIds("a@b.com", async (request) => {
		seen.push(request.pageToken ?? "");
		return (
			pages[request.pageToken ?? ""] ?? { accounts: [], nextPageToken: "" }
		);
	});
	assert.deepEqual(ids, ["1", "2", "3"]);
	assert.deepEqual(seen, ["", "t2"], "第二頁必須帶上一頁的 token");
	assert.deepEqual(
		await collectAccountIds("x@y.z", async () => ({
			accounts: [],
			nextPageToken: "",
		})),
		[],
	);
});

test("信箱找帳號：token 一直不結束就報錯，不靜默截斷", async () => {
	await assert.rejects(
		collectAccountIds("a@b.com", async () => ({
			accounts: [{ id: "1" }],
			nextPageToken: "more",
		})),
	);
});

test("合併多個帳號的店家：保持順序、同一家店只留第一次", () => {
	const a = { store: { id: "10" }, tag: "a-first" };
	const b = { store: { id: "11" }, tag: "b" };
	const dup = { store: { id: "10" }, tag: "a-again" };
	const c = { store: { id: "12" }, tag: "c" };
	assert.deepEqual(mergeStoreLists([[a, b], [dup, c], []]), [a, b, c]);
	assert.deepEqual(mergeStoreLists([]), []);
});

test("文字搜尋與沒有關鍵字：直接查店家，不碰帳號", () => {
	const input = { tab: "all" as const, pageSize: 10, pageToken: "tok" };
	assert.deepEqual(textSearchRequest({ kind: "text", query: "甜心" }, input), {
		pageSize: 10,
		pageToken: "tok",
		query: "甜心",
	});
	assert.deepEqual(textSearchRequest({ kind: "none" }, input), {
		pageSize: 10,
		pageToken: "tok",
	});
});

test("兩張「待審核」統計卡讀 kycAwaitingReviewStores（與待審核分頁同一個成員），不讀已刪除的 kycSubmittedStores", async () => {
	for (const file of [
		"src/app/admin/page.tsx",
		"src/app/admin/merchants/page.tsx",
	]) {
		const source = await readFile(file, "utf8");
		assert.match(source, /\.kycAwaitingReviewStores/u, file);
		assert.doesNotMatch(source, /kycSubmittedStores/u, file);
	}
});
