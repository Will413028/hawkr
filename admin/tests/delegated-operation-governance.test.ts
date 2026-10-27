import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	buildDelegatedAuditLogSearch,
	isDelegatedRevokeReason,
	isDelegatedSessionId,
	parseDelegatedAuditLogPage,
	parseDelegatedSessionReport,
	parseDelegatedSessionRevocation,
} from "../src/features/delegated-operations/governance";

// 後端契約（56 §6.1.11）。這是契約證據，不是開發用假資料。
const auditEntry = {
	auditLogId: "7494008268352651264",
	createdAt: "2026-08-14T12:33:23Z",
	confirmedAt: "2026-08-14T12:33:23Z",
	actor: { adminId: "9000000000000000001", displayName: "Cora Chen" },
	target: {
		storeId: "9200000000000000009",
		sellerAccountId: "9200000000000000009",
		storeName: "梨山盛花園",
	},
	action: "STOREFRONT_UPDATE",
	actionLabel: "更新店面資料",
	riskLevel: "medium",
	capability: "storefront.write",
	resourceType: "storefront",
	resourceId: "9200000000000000009",
	resourceName: "梨山盛花園",
	resourceImageKey: "",
	operationReason: "賣家回報聯絡資訊有誤",
	caseReference: "HK-2026-0814",
	result: "SUCCEEDED",
	bizCode: 0,
	bizCodeLabel: "",
	resourceVersionBefore: "1",
	resourceVersionAfter: "2",
	changedFields: [
		{
			field: "contactPhone",
			label: "聯絡電話",
			before: "—",
			after: "0912000111",
		},
	],
	correlationId: "256cfcc806af713b80f0b8b836d2283d",
	requestId: "",
	idempotencyKey: "seed-storefront-2",
	origin: "interactive",
	userAgent: "Mozilla/5.0 (Macintosh) Chrome/126.0.0.0",
	notifiedAt: null,
};

const sessionReport = {
	sessionId: "7494008076136087552",
	displayCode: "OPS-1KXP05J5SRQPS",
	target: auditEntry.target,
	actor: auditEntry.actor,
	issuedAt: "2026-08-14T12:32:37Z",
	endedAt: null,
	durationSeconds: "327",
	status: "active",
	operationCount: "2",
	riskOperationCount: "0",
	pendingAuditCount: "1",
	notifiedSeller: true,
	fulfillmentActionCount: "0",
	financeActionCount: "0",
	auditRetentionDays: "730",
};

test("稽核列 parser 收下全部治理欄位，並保留後端下發的 label", () => {
	const page = parseDelegatedAuditLogPage({
		records: [auditEntry],
		total: "1",
		page: "1",
		pageSize: "20",
	});
	assert.ok(page);
	assert.equal(page.total, 1);
	const [entry] = page.records;
	assert.equal(entry.actionLabel, "更新店面資料");
	assert.equal(entry.changedFields[0].label, "聯絡電話");
	assert.equal(entry.notifiedAt, null);
	// beforeSnapshot／afterSnapshot 沒有 API 出口，DTO 也不得長出這兩欄。
	assert.equal("beforeSnapshot" in entry, false);
	assert.equal("afterSnapshot" in entry, false);
});

test("PENDING 是合法終態：confirmedAt 為 null、changedFields 為空仍解析得出", () => {
	const page = parseDelegatedAuditLogPage({
		records: [
			{
				...auditEntry,
				result: "PENDING",
				confirmedAt: null,
				changedFields: [],
			},
		],
		total: "1",
		page: "1",
		pageSize: "20",
	});
	assert.ok(page);
	assert.equal(page.records[0].result, "PENDING");
	assert.equal(page.records[0].confirmedAt, null);
});

test("失敗列帶著後端下發的業務碼說明，前端不自建碼→文案對照表", () => {
	const page = parseDelegatedAuditLogPage({
		records: [
			{
				...auditEntry,
				result: "FAILED",
				bizCode: 9266,
				bizCodeLabel: "資料已被更新，請重新載入後再試",
				changedFields: [],
			},
		],
		total: "1",
		page: "1",
		pageSize: "20",
	});
	assert.ok(page);
	assert.equal(page.records[0].bizCodeLabel, "資料已被更新，請重新載入後再試");
});

test("bizCodeLabel 缺席退回空字串（gateway 尚未 promote 的窗口），型別錯仍整頁失敗", () => {
	// 缺欄整頁報錯的代價不對等：為了一行補充說明擋掉整份稽核紀錄。畫面退回只顯示原碼。
	const { bizCodeLabel: _omitted, ...withoutLabel } = auditEntry;
	const page = parseDelegatedAuditLogPage({
		records: [withoutLabel],
		total: "1",
		page: "1",
		pageSize: "20",
	});
	assert.ok(page);
	assert.equal(page.records[0].bizCodeLabel, "");

	assert.equal(
		parseDelegatedAuditLogPage({
			records: [{ ...auditEntry, bizCodeLabel: 9266 }],
			total: "1",
			page: "1",
			pageSize: "20",
		}),
		null,
		"型別錯是契約違反，不得被當成「沒有說明」",
	);
});

test("單列不合契約時整頁失敗，不是靜默跳過那一列", () => {
	for (const broken of [
		{ ...auditEntry, result: "UNKNOWN" },
		{ ...auditEntry, riskLevel: "critical" },
		{ ...auditEntry, resourceType: "shipment" },
		{ ...auditEntry, action: "SHIPMENT_CREATE" },
		{ ...auditEntry, createdAt: "2026-08-14 12:33:23" },
		{ ...auditEntry, auditLogId: 749400826 },
	]) {
		assert.equal(
			parseDelegatedAuditLogPage({
				records: [auditEntry, broken],
				total: "2",
				page: "1",
				pageSize: "20",
			}),
			null,
			`should fail closed: ${JSON.stringify(broken.result ?? broken.action)}`,
		);
	}
});

test("分頁數字缺席或超上限一律失敗，空字串不得被當成 0 筆", () => {
	for (const patch of [
		{ total: "" },
		{ total: undefined },
		{ page: "0" },
		{ pageSize: "51" },
		{ total: "-1" },
	]) {
		assert.equal(
			parseDelegatedAuditLogPage({
				records: [],
				total: "0",
				page: "1",
				pageSize: "20",
				...patch,
			}),
			null,
		);
	}
	assert.ok(
		parseDelegatedAuditLogPage({
			records: [],
			total: "0",
			page: "1",
			pageSize: "20",
		}),
	);
});

test("摘要 parser 是冪等的：同一份資料會被 BFF 與瀏覽器各解析一次", () => {
	const first = parseDelegatedSessionReport(sessionReport);
	assert.ok(first);
	assert.equal(first.pendingAuditCount, 1);
	assert.equal(first.auditRetentionDays, 730);
	// 第二次看到的是第一次的產物（計數已是 number）。不冪等的症狀是治理頁永遠
	// 顯示「摘要格式不完整」，而兩邊各自單看都正確。
	const second = parseDelegatedSessionReport(first);
	assert.deepEqual(second, first);
});

test("摘要 parser 對缺欄、非封閉狀態與非法保留天數 fail closed", () => {
	for (const patch of [
		{ status: "ended" },
		{ sessionId: "not-a-tsid" },
		{ displayCode: "" },
		{ notifiedSeller: "true" },
		{ auditRetentionDays: "0" },
		{ pendingAuditCount: undefined },
		{ issuedAt: "2026-08-14T12:32:37+08:00" },
	]) {
		assert.equal(
			parseDelegatedSessionReport({ ...sessionReport, ...patch }),
			null,
			`should fail closed: ${JSON.stringify(patch)}`,
		);
	}
});

test("撤銷結果必須帶 revokedAt 與非負 revision", () => {
	const parsed = parseDelegatedSessionRevocation({
		sessionId: "7494008076136087552",
		status: "revoked",
		revision: 1,
		revokedAt: "2026-08-14T12:39:29Z",
	});
	assert.ok(parsed);
	assert.equal(parsed.status, "revoked");
	for (const patch of [
		{ revokedAt: undefined },
		{ revision: -1 },
		{ revision: "1" },
		{ status: "ended" },
	]) {
		assert.equal(
			parseDelegatedSessionRevocation({
				sessionId: "7494008076136087552",
				status: "revoked",
				revision: 1,
				revokedAt: "2026-08-14T12:39:29Z",
				...patch,
			}),
			null,
		);
	}
});

test("篩選為空時不送出該參數——送空值會被值域檢查判為 9260", () => {
	const empty = buildDelegatedAuditLogSearch({ page: 1, pageSize: 20 });
	assert.equal(empty.get("page"), "1");
	assert.equal(empty.get("page_size"), "20");
	for (const key of [
		"action",
		"resource_type",
		"result",
		"date_from",
		"date_to",
	]) {
		assert.equal(empty.has(key), false);
	}

	const filtered = buildDelegatedAuditLogSearch({
		page: 2,
		pageSize: 999,
		action: "PRODUCT_PUBLISH",
		resourceType: "product",
		result: "PENDING",
		dateFrom: "2026-08-14T00:00:00Z",
	});
	assert.equal(filtered.get("action"), "PRODUCT_PUBLISH");
	assert.equal(filtered.get("resource_type"), "product");
	assert.equal(filtered.get("result"), "PENDING");
	assert.equal(filtered.get("date_from"), "2026-08-14T00:00:00Z");
	// 超上限夾到 50：後端對超上限回 9260，前端不該用一個必被拒絕的值去問。
	assert.equal(filtered.get("page_size"), "50");
});

test("sessionId 與撤銷理由的值域與 gateway 一致", () => {
	assert.equal(isDelegatedSessionId("7494008076136087552"), true);
	for (const bad of ["", "abc", "-1", "1.5", "12345678901234567890"]) {
		assert.equal(isDelegatedSessionId(bad), false);
	}
	assert.equal(isDelegatedRevokeReason("誤選店家"), true);
	assert.equal(isDelegatedRevokeReason("   "), false);
	assert.equal(isDelegatedRevokeReason("a".repeat(201)), false);
	assert.equal(isDelegatedRevokeReason("含控制字元"), false);
});

/**
 * P1-b 回歸守門：preflight 頂層的 `caseReferenceRequired` 是「任一選項需要」的 OR，
 * 不是「這一次要填」。把它拿來當必填判準，會讓每一個「案件編號可選填」的原因都在
 * 送出時被擋在 CASE_REFERENCE_REQUIRED。
 */
test("案件編號是否必填只由被選中的原因決定，不看 preflight 的 OR 欄位", async () => {
	const [route, dialog] = await Promise.all([
		readFile("src/app/api/admin/delegated-operations/route.ts", "utf8"),
		readFile(
			"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
			"utf8",
		),
	]);
	assert.match(route, /selectedReason\.caseReferenceRequired === true/u);
	assert.match(dialog, /selectedReason\?\.caseReferenceRequired === true/u);
	const requiredExpressions = (source: string) =>
		source
			.split("\n")
			.filter(
				(line) =>
					/caseReferenceRequired/u.test(line) && !line.trim().startsWith("//"),
			);
	for (const source of [route, dialog]) {
		for (const line of requiredExpressions(source)) {
			assert.doesNotMatch(
				line,
				/preflight[?.]*\.caseReferenceRequired/u,
				`preflight 的 OR 欄位不得參與必填判定：${line.trim()}`,
			);
		}
	}
});

test("三支治理端點都經專用 BFF，並打到 requireAdmin 的 gateway 路徑", async () => {
	const [upstream, auditRoute, summaryRoute, revokeRoute, service] =
		await Promise.all([
			readFile("src/features/delegated-operations/server/upstream.ts", "utf8"),
			readFile(
				"src/app/api/admin/delegated-operations/[sessionId]/audit-logs/route.ts",
				"utf8",
			),
			readFile(
				"src/app/api/admin/delegated-operations/[sessionId]/summary/route.ts",
				"utf8",
			),
			readFile(
				"src/app/api/admin/delegated-operations/[sessionId]/revoke/route.ts",
				"utf8",
			),
			readFile("src/services/delegated-operations.ts", "utf8"),
		]);
	assert.match(upstream, /\/audit-logs/u);
	assert.match(upstream, /\/summary/u);
	assert.match(upstream, /\/revoke/u);
	for (const route of [auditRoute, summaryRoute, revokeRoute]) {
		// 治理三支的身分是 ADM session；帶 operation credential 就會把跨工作階段的
		// 讀取面交給憑證持有者。
		assert.doesNotMatch(route, /operationSession/u);
		assert.match(route, /adminSession\.token\(request\)/u);
	}
	// 撤銷是 mutation：同源 ＋ XHR ＋ 冪等鍵，三者缺一不可。
	assert.match(revokeRoute, /isSameOriginSubmission/u);
	assert.match(revokeRoute, /x-requested-with/u);
	assert.match(revokeRoute, /idempotencyKey/u);
	assert.match(service, /\/api\/admin\/delegated-operations/u);
});
