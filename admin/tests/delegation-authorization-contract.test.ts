import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	DELEGATION_AUTHORIZATION_SOURCES,
	type DelegationCapabilityCatalog,
	delegationAuthorizationErrorMessage,
	delegationCapabilityDependencyClosure,
	delegationCapabilityRequiredBy,
	deselectDelegationCapability,
	isDelegationSourceReference,
	missingDelegationCapabilityDependencies,
	parseDelegationAuthorization,
	parseDelegationAuthorizationGrant,
	parseDelegationAuthorizationHistoryPage,
	parseDelegationAuthorizationLookup,
	parseDelegationAuthorizationRevocation,
	parseDelegationCapabilityCatalog,
	selectDelegationCapability,
	toDelegationEffectiveUntil,
	unknownDelegationCapabilities,
} from "../src/features/delegated-operations/authorization";

/**
 * 代行授權列（backend#2444）的契約與連動規則。
 *
 * 目錄 fixture 逐欄對應 gateway `internal/authz/admin_capability.go` 的
 * `adminCapabilityDependencies` 與 `adminCapabilityRisks`——它是契約證據，不是開發用
 * 假資料，而且**測試裡的這一份不是 production 的依賴來源**：production 只吃
 * `GET /admin/delegated-operations/capabilities` 當下回來的那一份。
 */
const catalogPayload = {
	capabilities: [
		{ value: "storefront.read", risk: "low", dependsOn: [] },
		{
			value: "storefront.write",
			risk: "medium",
			dependsOn: ["storefront.read"],
		},
		{ value: "products.read", risk: "low", dependsOn: [] },
		{ value: "products.write", risk: "medium", dependsOn: ["products.read"] },
		{
			value: "products.publish",
			risk: "high",
			dependsOn: ["products.read", "products.write"],
		},
		{ value: "posts.read", risk: "low", dependsOn: [] },
		{ value: "posts.write", risk: "medium", dependsOn: ["posts.read"] },
		{
			value: "posts.publish",
			risk: "high",
			dependsOn: ["posts.read", "posts.write"],
		},
		// 九值中唯一「唯讀卻是 high」的一項：每次揭露都落一列 PII 稽核。
		{ value: "orders.read", risk: "high", dependsOn: [] },
	],
};

const catalog = parseDelegationCapabilityCatalog(
	catalogPayload,
) as DelegationCapabilityCatalog;

const authorizationPayload = {
	id: "7495464541774413824",
	storeId: "2685914682",
	source: "contract",
	sourceReference: "CT-2026-0188",
	grantedCapabilities: ["products.read", "products.write"],
	effectiveFrom: "2026-08-19T00:00:00Z",
	effectiveUntil: null,
	approvedByAdminId: "9001",
	approvedByAdminName: "ADM 王小明",
	approvedAt: "2026-08-19T00:00:00Z",
	revokedAt: null,
	revokedByAdminId: null,
	revokedByAdminName: null,
	revokeReason: null,
	effective: true,
	createdAt: "2026-08-19T00:00:00Z",
	updatedAt: "2026-08-19T00:00:00Z",
};

test("能力目錄解析成功並保留後端順序與風險等級", () => {
	assert.ok(catalog);
	assert.equal(catalog.capabilities.length, 9);
	assert.deepEqual(
		catalog.capabilities.map((descriptor) => descriptor.value),
		[
			"storefront.read",
			"storefront.write",
			"products.read",
			"products.write",
			"products.publish",
			"posts.read",
			"posts.write",
			"posts.publish",
			"orders.read",
		],
	);
	assert.equal(
		catalog.capabilities.find((d) => d.value === "orders.read")?.risk,
		"high",
	);
});

test("目錄 fail-closed：空目錄、未知風險、指向目錄外的依賴一律不放行", () => {
	// 空目錄放行會畫出一個永遠送不出去的表單（至少要勾一項），而畫面上看起來像
	// 設定被清空了。
	assert.equal(parseDelegationCapabilityCatalog({ capabilities: [] }), null);
	assert.equal(
		parseDelegationCapabilityCatalog({
			capabilities: [{ value: "a.read", risk: "critical", dependsOn: [] }],
		}),
		null,
	);
	// 依賴指向目錄外 ⇒ 連動算不完整，會給出一個後端必然以 9287 拒絕的組合。
	assert.equal(
		parseDelegationCapabilityCatalog({
			capabilities: [
				{ value: "a.write", risk: "medium", dependsOn: ["a.read"] },
			],
		}),
		null,
	);
	assert.equal(parseDelegationCapabilityCatalog({ capabilities: null }), null);
});

test("勾選一項會連帶補上遞移依賴，且順序等同目錄順序", () => {
	assert.deepEqual(
		selectDelegationCapability(catalog, [], "products.publish"),
		["products.read", "products.write", "products.publish"],
	);
	// posts.publish 的依賴是兩層（publish → write → read），閉包必須遞移展開。
	assert.deepEqual(
		delegationCapabilityDependencyClosure(catalog, "posts.publish"),
		["posts.read", "posts.write"],
	);
	// 無依賴者不多補任何東西。
	assert.deepEqual(selectDelegationCapability(catalog, [], "orders.read"), [
		"orders.read",
	]);
	// 重複勾選是冪等的。
	assert.deepEqual(
		selectDelegationCapability(
			catalog,
			["products.read", "products.write", "products.publish"],
			"products.write",
		),
		["products.read", "products.write", "products.publish"],
	);
});

test("取消一項會連帶取消所有依賴它的項（遞移）", () => {
	const all = ["products.read", "products.write", "products.publish"];
	assert.deepEqual(
		deselectDelegationCapability(catalog, all, "products.read"),
		[],
	);
	assert.deepEqual(
		deselectDelegationCapability(catalog, all, "products.write"),
		["products.read"],
	);
	// 只取消最上層不動下層。
	assert.deepEqual(
		deselectDelegationCapability(catalog, all, "products.publish"),
		["products.read", "products.write"],
	);
	// 跨族群互不影響。
	assert.deepEqual(
		deselectDelegationCapability(
			catalog,
			["products.read", "posts.read", "posts.write"],
			"posts.read",
		),
		["products.read"],
	);
});

test("被連帶勾選的項要指得出是被誰要求的（勾選框據此停用）", () => {
	const required = delegationCapabilityRequiredBy(catalog, [
		"products.read",
		"products.write",
		"products.publish",
	]);
	assert.deepEqual(required.get("products.read"), [
		"products.write",
		"products.publish",
	]);
	assert.deepEqual(required.get("products.write"), ["products.publish"]);
	// 最上層沒有人要求它 ⇒ 可自由取消。
	assert.equal(required.has("products.publish"), false);
	// 單獨勾一個沒有依賴者的項，不該被標成連帶。
	assert.equal(
		delegationCapabilityRequiredBy(catalog, ["orders.read"]).size,
		0,
	);
});

test("送出前的閉包校驗擋下不完整組合，完整組合放行", () => {
	assert.deepEqual(
		missingDelegationCapabilityDependencies(catalog, ["products.publish"]),
		["products.read", "products.write"],
	);
	assert.deepEqual(
		missingDelegationCapabilityDependencies(catalog, [
			"products.read",
			"products.write",
			"products.publish",
		]),
		[],
	);
	assert.deepEqual(missingDelegationCapabilityDependencies(catalog, []), []);
});

test("目錄認不得的能力被標出來而不是靜默丟掉", () => {
	// 靜默丟掉會讓「調整授權範圍」在送出時把它一併撤掉，而畫面從未顯示過它。
	assert.deepEqual(
		unknownDelegationCapabilities(catalog, ["products.read", "finance.read"]),
		["finance.read"],
	);
	assert.deepEqual(
		unknownDelegationCapabilities(catalog, ["products.read"]),
		[],
	);
});

test("到期時間換算成 UTC 秒精度（帶毫秒會被 gateway 的正則擋掉）", () => {
	assert.equal(toDelegationEffectiveUntil(""), null);
	assert.equal(toDelegationEffectiveUntil("   "), null);
	assert.equal(toDelegationEffectiveUntil("not-a-date"), null);
	const value = toDelegationEffectiveUntil("2026-12-31T23:59");
	assert.ok(value);
	assert.match(value, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/u);
});

test("依據編號沿用 chk_sda_source_reference：4–200 字且至少含一個數字", () => {
	assert.equal(isDelegationSourceReference("CT-2026-0188"), true);
	assert.equal(isDelegationSourceReference("賣家說要開通"), false);
	assert.equal(isDelegationSourceReference("A1"), false);
	assert.equal(isDelegationSourceReference(`${"9".repeat(201)}`), false);
});

test("授權列解析：完整投影通過，缺一不可", () => {
	const parsed = parseDelegationAuthorization(authorizationPayload);
	assert.ok(parsed);
	assert.equal(parsed.storeId, "2685914682");
	assert.equal(parsed.effectiveUntil, null);
	assert.equal(parsed.effective, true);

	// 空範圍＝一列不授權任何東西的授權，畫面會顯示「已開通」卻沒有任何範圍。
	assert.equal(
		parseDelegationAuthorization({
			...authorizationPayload,
			grantedCapabilities: [],
		}),
		null,
	);
	// `effective` 是伺服端算的，缺席時不得由前端推導。
	assert.equal(
		parseDelegationAuthorization({
			...authorizationPayload,
			effective: undefined,
		}),
		null,
	);
	// source 是 DB CHECK 的封閉三值。
	assert.equal(
		parseDelegationAuthorization({ ...authorizationPayload, source: "verbal" }),
		null,
	);
	assert.deepEqual(
		[...DELEGATION_AUTHORIZATION_SOURCES],
		["contract", "ticket", "platform_policy"],
	);
});

test("尚未開通是 exists=false 的正常狀態，不是錯誤；自相矛盾的回應才是", () => {
	const empty = parseDelegationAuthorizationLookup({
		exists: false,
		authorization: null,
		activeSessionCount: "0",
	});
	assert.ok(empty);
	assert.equal(empty.exists, false);
	assert.equal(empty.authorization, null);
	assert.equal(empty.activeSessionCount, 0);

	// exists=false 卻帶著一列授權 ⇒ 畫面會出現「尚未開通」配上一份範圍。
	assert.equal(
		parseDelegationAuthorizationLookup({
			exists: false,
			authorization: authorizationPayload,
			activeSessionCount: "0",
		}),
		null,
	);
	// exists=true 卻沒有投影 ⇒ 一個沒有任何欄位的「已開通」。
	assert.equal(
		parseDelegationAuthorizationLookup({
			exists: true,
			authorization: null,
			activeSessionCount: "0",
		}),
		null,
	);
	// activeSessionCount 是撤回二次確認的唯一來源，空字串會變成 NaN。
	assert.equal(
		parseDelegationAuthorizationLookup({
			exists: false,
			authorization: null,
			activeSessionCount: "",
		}),
		null,
	);
	// 進行中筆數與 exists 正交：已撤回的店仍可能有殘留工作階段。
	const busy = parseDelegationAuthorizationLookup({
		exists: true,
		authorization: authorizationPayload,
		activeSessionCount: "3",
	});
	assert.equal(busy?.activeSessionCount, 3);
});

test("開通結果的 outcome 由伺服端回報，不由呼叫方指定", () => {
	const created = parseDelegationAuthorizationGrant({
		authorization: authorizationPayload,
		outcome: "created",
	});
	assert.equal(created?.outcome, "created");
	assert.equal(
		parseDelegationAuthorizationGrant({
			authorization: authorizationPayload,
			outcome: "upserted",
		}),
		null,
	);
});

test("撤回結果：已撤回是冪等成功，但不得同時宣稱中止了工作階段", () => {
	const revokedPayload = {
		...authorizationPayload,
		revokedAt: "2026-08-19T02:00:00Z",
		revokedByAdminId: "9001",
		revokedByAdminName: "ADM 王小明",
		revokeReason: "合約已終止",
		effective: false,
	};
	const fresh = parseDelegationAuthorizationRevocation({
		authorization: revokedPayload,
		alreadyRevoked: false,
		revokedSessionIds: ["7495464600540807168"],
	});
	assert.equal(fresh?.revokedSessionIds.length, 1);

	const idempotent = parseDelegationAuthorizationRevocation({
		authorization: revokedPayload,
		alreadyRevoked: true,
		revokedSessionIds: [],
	});
	assert.equal(idempotent?.alreadyRevoked, true);

	// 「什麼都沒動」卻回報中止了工作階段 ＝ 一份假的撤回證明。
	assert.equal(
		parseDelegationAuthorizationRevocation({
			authorization: revokedPayload,
			alreadyRevoked: true,
			revokedSessionIds: ["7495464600540807168"],
		}),
		null,
	);
	// 缺欄位一律 null，不補一個空陣列冒充成功。
	assert.equal(
		parseDelegationAuthorizationRevocation({ alreadyRevoked: false }),
		null,
	);
});

test("授權變更歷程：單列不合契約時整頁失敗，不跳過那一列", () => {
	const entry = {
		id: "7495464541774413825",
		authorizationId: "7495464541774413824",
		storeId: "2685914682",
		action: "SCOPE_UPDATE",
		actionLabel: "調整授權範圍",
		actorAdminId: "9001",
		actorDisplayName: "ADM 王小明",
		operationReason: "縮小為僅檢視",
		source: "contract",
		sourceReference: "CT-2026-0188",
		capabilitiesBefore: ["products.read", "products.write"],
		capabilitiesAfter: ["products.read"],
		effectiveUntilBefore: null,
		effectiveUntilAfter: null,
		createdAt: "2026-08-19T01:00:00Z",
	};
	const page = parseDelegationAuthorizationHistoryPage({
		records: [entry],
		total: "1",
		page: "1",
		pageSize: "10",
	});
	assert.equal(page?.records.length, 1);
	// 動作中文由後端逐列下發；缺它就整頁失敗，不退回機器名。
	assert.equal(
		parseDelegationAuthorizationHistoryPage({
			records: [entry, { ...entry, actionLabel: undefined }],
			total: "2",
			page: "1",
			pageSize: "10",
		}),
		null,
	);
	assert.equal(
		parseDelegationAuthorizationHistoryPage({
			records: [],
			total: "0",
			page: "1",
			pageSize: "51",
		}),
		null,
	);
});

test("錯誤分派以業務碼為準，未知碼退回呼叫端給的說法", () => {
	assert.match(delegationAuthorizationErrorMessage(9286, "x"), /尚未開通/u);
	assert.match(delegationAuthorizationErrorMessage(9285, "x"), /已有有效/u);
	assert.match(delegationAuthorizationErrorMessage(9287, "x"), /相依/u);
	// 401 是 envelope 的 code（BFF 的 unauthorized()／gateway JWT 中介層都送得出它）。
	// 漏了它 ⇒ 顯示英文 "Unauthorized" 或「服務暫時無法使用」，而正解是重新登入。
	assert.match(delegationAuthorizationErrorMessage(401, "x"), /重新登入/u);
	// 9261 是另一個碼（requireAdmin 過了但取不到 accountID 的窄路徑），有真實
	// emitter，不可被 401 取代 —— 兩者必須各自答得出話。
	assert.match(delegationAuthorizationErrorMessage(9261, "x"), /重新登入/u);
	// 9277（工作階段不存在）與 9280（上游不可用）的下一步相反，而 BFF 的
	// safeUpstreamError 對兩者送的 message 一字不差 ⇒ 這裡不分派的話畫面就分不出來。
	// 斷言「兩者各自答得出話」且「答的不是同一句」——後者才是真正的不變式：
	// 把 9277 複製成 9280 的文案一樣會讓人去等一個永遠不會好的服務。
	assert.match(delegationAuthorizationErrorMessage(9277, "x"), /找不到/u);
	assert.match(delegationAuthorizationErrorMessage(9280, "x"), /稍後再試/u);
	assert.notEqual(
		delegationAuthorizationErrorMessage(9277, "x"),
		delegationAuthorizationErrorMessage(9280, "x"),
	);
	assert.equal(
		delegationAuthorizationErrorMessage(9999, "fallback"),
		"fallback",
	);
	assert.equal(
		delegationAuthorizationErrorMessage("UPSTREAM_FAILURE", "fallback"),
		"fallback",
	);
	assert.equal(
		delegationAuthorizationErrorMessage(undefined, "fallback"),
		"fallback",
	);
});

// ==================== 機械保證（source-level） ====================

/** 授權面的新檔案。加檔案就要加到這裡，否則保證只覆蓋一半。 */
const AUTHORIZATION_FILES = [
	"src/features/delegated-operations/authorization.ts",
	"src/features/delegated-operations/server/authorization-route.ts",
	"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
	"src/features/delegated-operations/components/DelegationAuthorizationDialog.tsx",
	"src/features/delegated-operations/components/DelegationAuthorizationRevokeDialog.tsx",
	"src/app/api/admin/delegated-operations/capabilities/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/revoke/route.ts",
	"src/app/api/admin/delegated-operations/authorizations/[storeId]/history/route.ts",
];

/**
 * 註解不是程式碼。這些檔案的註解**刻意**寫著「不得引用 CAPABILITY_DEPENDENCIES」
 * 之類的句子，直接對原文 grep 會把那句警告本身判成違規——命中的是註解還是會被執行
 * 的東西，是這類 source-level gate 的第一個誤判來源。
 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//gu, "")
		.replace(/(^|[^:])\/\/.*$/gmu, "$1");
}

async function readAuthorizationFiles(): Promise<[string, string][]> {
	return Promise.all(
		AUTHORIZATION_FILES.map(
			async (file) =>
				[file, stripComments(await readFile(file, "utf8"))] as [string, string],
		),
	);
}

test("授權面不維護第二份能力依賴表：一律吃後端下發的目錄", async () => {
	for (const [file, source] of await readAuthorizationFiles()) {
		// `policy.ts` 的 CAPABILITY_DEPENDENCIES 是 preflight 的私有副本。引用它，
		// 「畫面允許送出什麼」就由一份沒有人在維護的鏡像決定，而漂移的兩種症狀
		//（擋掉合法組合／放行後被 9287 拒絕）都不會有東西轉紅。
		assert.doesNotMatch(source, /CAPABILITY_DEPENDENCIES/u, file);
		// 也不得自己再寫一份對照表。
		assert.doesNotMatch(source, /["']products\.publish["']\s*:\s*\[/u, file);
	}
	// 連動規則只有一個入口：目錄。四支純函式全部以它為第一個參數。
	const authorization = stripComments(
		await readFile(
			"src/features/delegated-operations/authorization.ts",
			"utf8",
		),
	);
	for (const fn of [
		"delegationCapabilityDependencyClosure",
		"selectDelegationCapability",
		"deselectDelegationCapability",
		"delegationCapabilityRequiredBy",
		"missingDelegationCapabilityDependencies",
	]) {
		assert.match(
			authorization,
			new RegExp(
				`function ${fn}\\(\\s*catalog: DelegationCapabilityCatalog`,
				"u",
			),
			fn,
		);
	}
	// `policy.ts` 只准借解析原語，不准借規則。
	assert.match(authorization, /from "\.\/policy"/u);
	assert.doesNotMatch(
		authorization,
		/import[\s\S]{0,200}CAPABILITY_DEPENDENCIES/u,
	);
});

test("授權面錯誤分派看業務碼，不看 HTTP status", async () => {
	for (const [file, source] of await readAuthorizationFiles()) {
		// 9286（找不到授權設定）就是 404，而 404 同時也是「這個環境沒有這支端點」的
		// 形狀。`MerchantOperationEntry` 那條 `status === 404 || 503` 是為 preflight
		// 寫的，授權面沿用會把「這家店還沒開通」講成「代行營運未啟用」。
		assert.doesNotMatch(source, /status\s*===\s*(404|403|409|503)/u, file);
		assert.doesNotMatch(source, /代行營運目前未啟用/u, file);
	}
});

test("授權面對話框走共用 Dialog（modal 語意與 focus trap 在 Dialog 內）", async () => {
	for (const file of [
		"src/features/delegated-operations/components/DelegationAuthorizationDialog.tsx",
		"src/features/delegated-operations/components/DelegationAuthorizationRevokeDialog.tsx",
	]) {
		const source = await readFile(file, "utf8");
		assert.match(source, /<Dialog\b/u, file);
		assert.doesNotMatch(source, /role="dialog"/u, file);
	}
	const dialog = await readFile("src/components/UI/Dialog.tsx", "utf8");
	assert.match(dialog, /role="dialog"/u);
	assert.match(dialog, /aria-modal="true"/u);
	assert.match(dialog, /useEscapeKey/u);
	// focus trap 掛 dialog、背景隔離掛 overlay：兩個 ref 必須是不同節點。
	assert.match(dialog, /useFocusTrap\(dialogRef, true, isTopmost\)/u);
	// 堆疊與背景隔離是同一個 hook，掛在 overlay。
	assert.match(dialog, /useModalStack\(overlayRef, true\)/u);
});

test("撤回對話框必須在按下之前講出連帶中止的工作階段數", async () => {
	const source = await readFile(
		"src/features/delegated-operations/components/DelegationAuthorizationRevokeDialog.tsx",
		"utf8",
	);
	assert.match(source, /activeSessionCount/u);
	assert.match(source, /個進行中的代行工作階段/u);
	// 實際中止筆數以回應的 revokedSessionIds 為準（中間可能有人自行結束）。
	assert.match(source, /result\.revokedSessionIds\.length/u);
	// 撤回失敗不換冪等鍵：撤回可能已經生效而回應沒送到。
	assert.match(source, /const \[idempotencyKey\] = useState/u);
});

test("首次開通不預設全選，且沒有一鍵全選的捷徑", async () => {
	const source = await readFile(
		"src/features/delegated-operations/components/DelegationAuthorizationDialog.tsx",
		"utf8",
	);
	// grant 模式一律從空集合開始（設計書「首批一律最小必要子集」）。
	assert.match(
		source,
		/useState<string\[\]>\(\s*isUpdate && current \? current\.grantedCapabilities : \[\],\s*\)/u,
	);
	// 全空時送出鍵停用，並說明「請直接使用撤回」。
	assert.match(source, /disabled=\{selected\.length === 0\}/u);
	assert.match(source, /請直接使用「撤回授權」/u);
	assert.doesNotMatch(
		source,
		/catalog\.capabilities\.map\(\(d\) => d\.value\)/u,
	);
});
