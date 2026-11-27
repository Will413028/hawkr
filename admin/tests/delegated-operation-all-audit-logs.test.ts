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


test("① actorAdminId／storeId 空字串時不得出現在 query string", () => {
	const search = buildDelegatedAllAuditLogSearch({
		page: 1,
		pageSize: 20,
		actorAdminId: "",
		storeId: "",
	});
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
	assert.equal(search.get("page"), "2");
	assert.equal(search.get("resource_type"), "product");
	assert.equal(search.get("result"), "FAILED");
	assert.equal(search.get("date_from"), "2026-08-01T00:00:00Z");
	assert.equal(search.get("date_to"), "2026-08-31T23:59:59Z");
});


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
	assert.equal(a[1], "all-audit-logs");
});


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
	assert.notEqual(
		parsed.records[0].actor.adminId,
		parsed.records[1].actor.adminId,
	);
	assert.notEqual(
		parsed.records[0].target.storeId,
		parsed.records[1].target.storeId,
	);
});

test("被拒絕的上傳列（MEDIA_UPLOAD／media）可以解析，不會讓整頁 fail-closed", () => {
	const parsed = parseDelegatedAuditLogPage({
		...PAGE,
		records: [
			{
				...ENTRY,
				action: "MEDIA_UPLOAD",
				actionLabel: "上傳檔案（被拒絕）",
				riskLevel: "low",
				capability: "media.upload",
				resourceType: "media",
				resourceId: "",
				resourceName: "",
				result: "FAILED",
			},
		],
	});
	assert.ok(parsed);
	assert.equal(parsed.records[0].action, "MEDIA_UPLOAD");
	assert.equal(parsed.records[0].resourceType, "media");
});

test("真的空集合是合法回應，不得與解析失敗混為一談", () => {
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
	assert.match(src, /auditQuery\.isError/u);
	assert.match(src, /role="alert"/u);
});

test("④ 有人按得到：零 caller ＝ 未交付", async () => {
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

	const route = await readFile(
		"src/app/api/admin/delegated-operations/audit-logs/route.ts",
		"utf8",
	);
	assert.match(route, /DELEGATED_OPERATION_UPSTREAM\.allAuditLogs/u);
	assert.match(route, /adminSession\.token\(request\)/u);
	assert.doesNotMatch(route, /credential/iu);
});

test("BFF 對打錯的 id 篩選回參數錯，不得靜默降級成不篩選", async () => {
	const route = await readFile(
		"src/app/api/admin/delegated-operations/audit-logs/route.ts",
		"utf8",
	);
	assert.match(route, /isDelegatedNumericId/u);
	assert.match(route, /INVALID_AUDIT_QUERY/u);
});
