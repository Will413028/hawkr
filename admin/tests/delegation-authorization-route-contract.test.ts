import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { isDelegatedOperationNamespace } from "../src/features/delegated-operations/server/bff-boundary";
import { DELEGATED_OPERATION_UPSTREAM } from "../src/features/delegated-operations/server/upstream";

/**
 * 授權列七支的 same-origin route 契約（backend#2444）。
 *
 * 這組是 source-level 契約，不是假裝成瀏覽器 E2E：實際的 cookie 轉送、CSRF 與
 * gateway 回應仍要 dev 實打。
 */

const ROUTE_FILES = [
	"src/app/api/admin/delegated-operations/capabilities/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/revoke/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/history/route.ts",
];

const CLIENT_FILES = [
	"src/services/delegated-operations.ts",
	"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
	"src/features/delegated-operations/components/DelegationAuthorizationDialog.tsx",
	"src/features/delegated-operations/components/DelegationAuthorizationRevokeDialog.tsx",
];

test("上游路徑一律含 delegated-operations 字面值，catch-all BFF 才拒得掉", () => {
	const paths = [
		DELEGATED_OPERATION_UPSTREAM.capabilities,
		DELEGATED_OPERATION_UPSTREAM.authorizations,
		DELEGATED_OPERATION_UPSTREAM.authorization("2685914682"),
		DELEGATED_OPERATION_UPSTREAM.authorizationRevoke("2685914682"),
		DELEGATED_OPERATION_UPSTREAM.authorizationHistory("2685914682"),
	];
	for (const path of paths) {
		assert.match(path, /^\/api\/v1\/admin\/delegated-operations\//u, path);
		// `bff-boundary.ts` 只認 `delegated-operation`／`delegated-operations` 這兩個
		// 字面值來拒絕代理。換成 `delegation-authorizations` 之類的名字，前端就能用
		// 泛用 api.get 繞過整套 parser／safeUpstreamError／no-store 保護。
		assert.equal(
			isDelegatedOperationNamespace(path.replace(/^\//u, "").split("/")),
			true,
			path,
		);
	}
	assert.equal(
		DELEGATED_OPERATION_UPSTREAM.authorization("2685914682"),
		"/api/v1/admin/delegated-operations/authorizations/2685914682",
	);
	assert.equal(
		DELEGATED_OPERATION_UPSTREAM.authorizationRevoke("2685914682"),
		"/api/v1/admin/delegated-operations/authorizations/2685914682/revoke",
	);
	assert.equal(
		DELEGATED_OPERATION_UPSTREAM.authorizationHistory("2685914682"),
		"/api/v1/admin/delegated-operations/authorizations/2685914682/history",
	);
});

test("瀏覽器只打專用 same-origin route，不碰 catch-all BFF 或 gateway 直址", async () => {
	for (const file of CLIENT_FILES) {
		const source = await readFile(file, "utf8");
		assert.doesNotMatch(source, /\/api\/bff/u, file);
		assert.doesNotMatch(source, /\/api\/v1\/admin/u, file);
		assert.doesNotMatch(source, /localStorage|sessionStorage/u, file);
	}
	const service = await readFile(
		"src/services/delegated-operations.ts",
		"utf8",
	);
	assert.match(
		service,
		/const AUTHORIZATION_PREFIX = "\/api\/admin\/delegated-operations\/authorizations"/u,
	);
	assert.match(service, /"\/api\/admin\/delegated-operations\/capabilities"/u);
});

test("每支 route 先驗身分再讀 body，且回應一律 no-store", async () => {
	for (const file of ROUTE_FILES) {
		const source = await readFile(file, "utf8");
		assert.match(source, /export const runtime = "nodejs"/u, file);
		assert.match(source, /export const dynamic = "force-dynamic"/u, file);
		assert.match(source, /adminSession\.token\(request\)/u, file);
		// `ok()` 統一補 Cache-Control: no-store（governance-route.ts）。
		assert.match(source, /return ok\(/u, file);
		// 上游 message 一律不透傳：未知服務可能在自由文字錯誤裡回吐憑證或內部拓撲。
		assert.doesNotMatch(source, /payload\.message/u, file);
	}
});

test("寫入型 route 三道防線齊備：同源、XHR、冪等鍵", async () => {
	for (const file of ROUTE_FILES.filter(
		(path) => !path.endsWith("capabilities/route.ts"),
	)) {
		const source = await readFile(file, "utf8");
		if (!/export async function (POST|PUT)/u.test(source)) continue;
		assert.match(source, /isSameOriginSubmission\(request\.headers\)/u, file);
		assert.match(source, /x-requested-with/u, file);
		assert.match(source, /isValidAuthorizationIdempotencyKey/u, file);
		assert.match(source, /isRequestTooLarge\(request\)/u, file);
	}
});

test("回應必須證明是同一個對象的答案，否則判契約錯", async () => {
	for (const file of ROUTE_FILES.filter((path) => path.includes("[storeId]"))) {
		const source = await readFile(file, "utf8");
		// 回的是另一家店的授權列 ⇒ 畫面會把它當成「這家店」而顯示一份不屬於它的範圍，
		// 管理員接著會對著它按下撤回。
		assert.match(source, /storeId !== storeId/u, file);
		assert.match(source, /contractViolation\(/u, file);
	}
	const grant = await readFile(
		"src/app/api/admin/delegated-operations/authorizations/route.ts",
		"utf8",
	);
	assert.match(grant, /parsed\.authorization\.storeId !== storeId/u);
	const revoke = await readFile(
		"src/app/api/admin/delegated-operations/authorizations/[storeId]/revoke/route.ts",
		"utf8",
	);
	// 宣稱成功卻沒有回出「這一列已撤回」＝一份假的撤回證明。
	assert.match(revoke, /parsed\.authorization\.revokedAt === null/u);
});

test("BFF 不重寫第三份依賴閉包規則", async () => {
	const shared = await readFile(
		"src/features/delegated-operations/server/authorization-route.ts",
		"utf8",
	);
	assert.doesNotMatch(shared, /dependsOn/u);
	assert.doesNotMatch(shared, /CAPABILITY_DEPENDENCIES/u);
	// 形狀仍要驗：值域、長度、必填。
	assert.match(shared, /DELEGATION_AUTHORIZATION_SOURCES/u);
	assert.match(shared, /INVALID_SOURCE_REFERENCE/u);
	assert.match(shared, /OPERATION_REASON_REQUIRED/u);
});

test("商家詳情頁以 store_id surrogate 掛載代行授權分頁", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /activeTab === "delegation"/u);
	assert.match(page, /代行授權/u);
	// 授權列以 sellers.store_id 為鍵：傳店家的 id（與 URL 上的 params.id 相同），不是 account_id。
	assert.match(
		page,
		/<MerchantDelegationAuthorizationPanel\s+storeId=\{store\.id\}/u,
	);
	assert.doesNotMatch(
		page,
		/<MerchantDelegationAuthorizationPanel[\s\S]{0,200}accountId/u,
	);
});
