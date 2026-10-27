import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	buildDelegatedAllAuditLogSearch,
	delegatedAllAuditLogQueryKey,
	parseDelegatedAuditLogPage,
} from "../src/features/delegated-operations/governance";

/**
 * 跨工作階段代行紀錄（56 §11.3 P1 的第二支端點，backend#2740 才接上前端）。
 *
 * 這支端點在 backend PR #2723 就交付並部署，但**零前端消費者**：ADM 沒有 BFF route、
 * 沒有 service、沒有頁面，管理員只能一場一場點進去看。兩支專為它建的索引
 * （`idx_aoal_target_created`／`idx_aoal_actor_created`）在那段期間一直付寫入成本
 * 而沒有任何查詢在用。
 *
 * 本檔守四件事，每一件都要能單獨被打紅：
 *
 *   ① 兩個 id 篩選**只在非空時送出**（空字串會變成「比對空字串」＝永遠零筆）
 *   ② query key 跟著篩選變（否則換條件時畫面顯示上一組條件的快取，且看起來完全正常）
 *   ③ 三種空狀態在畫面上分得開（載入中／篩選無結果／真的沒有任何代行動作）
 *   ④ **有人按得到它**——零 caller ＝ 未交付
 */

// 後端契約（gateway `DelegatedAuditLogEntry`）。數字一律字串。
const ENTRY = {
	auditLogId: "7494008076136087553",
	createdAt: "2026-08-14T12:33:10Z",
	confirmedAt: "2026-08-14T12:33:11Z",
	actor: { adminId: "2661027599", displayName: "alexischen" },
	target: {
		storeId: "2685914682",
		sellerAccountId: "2685914682",
		storeName: "梨山盛花園",
	},
	action: "PRODUCT_UPDATE",
	actionLabel: "修改商品",
	riskLevel: "low",
	capability: "products.write",
	resourceType: "product",
	resourceId: "7401234567890123456",
	resourceName: "高山高麗菜",
	resourceImageKey: "products/abc.jpg",
	operationReason: "賣家來電要求更正產地",
	caseReference: "CS-20260814-001",
	result: "SUCCEEDED",
	bizCode: 0,
	bizCodeLabel: "",
	resourceVersionBefore: "3",
	resourceVersionAfter: "4",
	changedFields: [],
	correlationId: "c-1",
	requestId: "r-1",
	idempotencyKey: "i-1",
	origin: "ADM",
	userAgent: "Mozilla/5.0",
	notifiedAt: null,
};

const PAGE = { records: [ENTRY], total: "1", page: "1", pageSize: "20" };

// ---------------------------------------------------------------------------
// ① 兩個 id 篩選只在非空時送出
// ---------------------------------------------------------------------------

test("① actorAdminId／storeId 空字串時不得出現在 query string", () => {
	const search = buildDelegatedAllAuditLogSearch({
		page: 1,
		pageSize: 20,
		actorAdminId: "",
		storeId: "",
	});
	// 送 `actor_admin_id=` 會讓後端把它當成一個要精確比對的值 ⇒ 永遠零筆，而畫面上
	// 與「這位管理員真的沒做過任何代行」長得一模一樣。
	assert.equal(search.has("actor_admin_id"), false);
	assert.equal(search.has("store_id"), false);
});

test("① 非空時逐字送出，且沿用 per-session 版的其餘欄位", () => {
	const search = buildDelegatedAllAuditLogSearch({
		page: 2,
		pageSize: 20,
		actorAdminId: "2661027599",
		storeId: "2685914682",
		resourceType: "product",
		result: "FAILED",
		dateFrom: "2026-08-01T00:00:00Z",
		dateTo: "2026-08-31T23:59:59Z",
	});
	assert.equal(search.get("actor_admin_id"), "2661027599");
	assert.equal(search.get("store_id"), "2685914682");
	// 這幾個必須與 per-session 版逐字相同，否則同一組條件在兩個畫面上查出不同的列。
	assert.equal(search.get("page"), "2");
	assert.equal(search.get("resource_type"), "product");
	assert.equal(search.get("result"), "FAILED");
	assert.equal(search.get("date_from"), "2026-08-01T00:00:00Z");
	assert.equal(search.get("date_to"), "2026-08-31T23:59:59Z");
});

// ---------------------------------------------------------------------------
// ② query key 跟著篩選變
// ---------------------------------------------------------------------------

test("② 換篩選就換 key：否則畫面拿上一組條件的快取當新結果", () => {
	const base = { page: 1, pageSize: 20, actorAdminId: "", storeId: "" };
	const a = delegatedAllAuditLogQueryKey(base);
	const b = delegatedAllAuditLogQueryKey({ ...base, storeId: "2685914682" });
	const c = delegatedAllAuditLogQueryKey({
		...base,
		actorAdminId: "2661027599",
	});
	assert.notDeepEqual(a, b);
	assert.notDeepEqual(a, c);
	assert.notDeepEqual(b, c);
	// 與工作階段列表的 key 必須分得開，否則兩頁互相汙染彼此的快取。
	assert.equal(a[1], "all-audit-logs");
});

// ---------------------------------------------------------------------------
// parser：與 per-session 共用同一份，跨階段的列 actor／target 互不相同
// ---------------------------------------------------------------------------

test("跨工作階段的列共用同一份 parser，且每列的 actor／target 可以不同", () => {
	const parsed = parseDelegatedAuditLogPage({
		...PAGE,
		records: [
			ENTRY,
			{
				...ENTRY,
				auditLogId: "7494008076136087554",
				actor: { adminId: "2701338845", displayName: "willwu" },
				target: {
					storeId: "2632049831",
					sellerAccountId: "2632049831",
					storeName: "另一家店",
				},
			},
		],
		total: "2",
	});
	assert.ok(parsed);
	assert.equal(parsed.records.length, 2);
	// 這正是 proto `AuditLogEntry` 的註解預告的：「§11.3 P1 的跨 session 稽核端點沿用
	// 本 message，那支端點的每一列 actor／target 必然互不相同」。
	assert.notEqual(
		parsed.records[0].actor.adminId,
		parsed.records[1].actor.adminId,
	);
	assert.notEqual(
		parsed.records[0].target.storeId,
		parsed.records[1].target.storeId,
	);
});

test("真的空集合是合法回應，不得與解析失敗混為一談", () => {
	// 「這家店從來沒被代行過」正是治理最想確認的結論。它必須解析成功且 records 為空，
	// 而不是被當成壞掉的回應吞掉。
	const empty = parseDelegatedAuditLogPage({
		records: [],
		total: "0",
		page: "1",
		pageSize: "20",
	});
	assert.ok(empty);
	assert.equal(empty.records.length, 0);
	assert.equal(empty.total, 0);
});

// ---------------------------------------------------------------------------
// ③ 三種空狀態分得開　④ 有人按得到
// ---------------------------------------------------------------------------

test("③ 載入中／篩選無結果／真的沒有紀錄，三句話互不相同", async () => {
	const src = await readFile(
		"src/features/delegated-operations/components/AllAuditLogConsole.tsx",
		"utf8",
	);
	const loading = "正在載入代行紀錄…";
	const filtered = "目前的篩選條件沒有符合的代行紀錄。";
	const truly = "平台上還沒有任何代行操作紀錄。";
	for (const s of [loading, filtered, truly]) {
		assert.ok(src.includes(s), `缺少空狀態文案：${s}`);
	}
	assert.equal(new Set([loading, filtered, truly]).size, 3);
	// 上游錯誤是第四態，且必須與空狀態走不同分支（錯誤時不得渲染成一張空表 ——
	// 那會把「讀取失敗」講成「沒有紀錄」，而後者是治理會據以下結論的答案）。
	assert.match(src, /auditQuery\.isError/u);
	assert.match(src, /role="alert"/u);
});

test("④ 有人按得到：零 caller ＝ 未交付", async () => {
	// 這支端點在 backend PR #2723 就交付了，缺的一直是這顆按鈕。判準是 §2.0 的
	// 「一個真實使用者，從哪個畫面、按哪顆按鈕」。
	const entry = await readFile(
		"src/features/delegated-operations/components/SessionListConsole.tsx",
		"utf8",
	);
	assert.match(entry, /href="\/admin\/delegated-operations\/audit-logs"/u);

	const page = await readFile(
		"src/app/admin/delegated-operations/audit-logs/page.tsx",
		"utf8",
	);
	assert.match(page, /<AllAuditLogConsole/u);

	// BFF route 存在且走的是專用的 upstream 路徑常數，不是泛用代理。
	const route = await readFile(
		"src/app/api/admin/delegated-operations/audit-logs/route.ts",
		"utf8",
	);
	assert.match(route, /DELEGATED_OPERATION_UPSTREAM\.allAuditLogs/u);
	// 身分是 ADM session；這支不吃也不需要 operation credential——讓憑證持有者列得出
	// 全平台的代行活動，等於把治理視角交給被治理的人。
	assert.match(route, /adminSession\.token\(request\)/u);
	assert.doesNotMatch(route, /credential/iu);
});

test("BFF 對打錯的 id 篩選回參數錯，不得靜默降級成不篩選", async () => {
	const route = await readFile(
		"src/app/api/admin/delegated-operations/audit-logs/route.ts",
		"utf8",
	);
	// 靜默降級會把「你打錯了店家 id」變成「這是全平台的稽核紀錄」，而畫面上兩者
	// 只差在筆數多寡。
	assert.match(route, /isDelegatedNumericId/u);
	assert.match(route, /INVALID_AUDIT_QUERY/u);
});
