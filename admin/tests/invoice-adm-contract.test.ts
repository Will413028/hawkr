import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";
import {
	ACTION_CONFLICT_CODES,
	COMPLAINT_ACTION_ALLOWED_FROM,
	COMPLAINT_ACTION_CONSEQUENCE,
	COMPLAINT_ACTION_DECIDE_RESOLVED,
	COMPLAINT_ACTION_DECIDE_VIOLATION,
	COMPLAINT_ACTION_LABEL,
	COMPLAINT_ACTION_ORDER,
	COMPLAINT_ACTION_RECOMMEND_SUSPENSION,
	COMPLAINT_ACTION_REMIND_SELLER,
	COMPLAINT_ACTION_REQUEST_REWORK,
	RECEIPT_AUTH_SIGNABLE_STATUSES,
	TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE,
	TAX_DECISION_ALLOWED_FROM,
	TAX_DECISION_ORDER,
	TAX_EXTENSION_APPROVE,
	TAX_EXTENSION_DEFAULT_DAYS,
	TAX_EXTENSION_MAX_DAYS,
	TAX_EXTENSION_REJECT,
	taxDecisionBlockedByReceiptAuth,
} from "../src/features/invoices/admin-actions";
import {
	INVOICE_OPERATION_STATUS_BADGE_VARIANT,
	TAX_REVIEW_STATUS_BADGE_VARIANT,
} from "../src/features/invoices/badge-variants";
import {
	KPI_META,
	KPI_ORDER,
	kpiDisplayValue,
	kpiMetaOf,
} from "../src/features/invoices/overview";
import {
	deadlineDisplay,
	PENDING_REVIEW_STATUSES,
	TAX_REVIEW_FILTER_VALUES,
} from "../src/features/invoices/tax-reviews";
import {
	InvoiceOperationStatus,
	SellerTaxReviewDecision,
	SellerTaxReviewStatus,
} from "../src/lib/codemeta";

/**
 * ADM 發票模組 A01／A02 的契約 gate。
 *
 * 這裡守的四件事全部有一個共同點：**做錯不會報錯**。
 *   - KPI key 打錯 → 那張卡直接消失（後端回的 key 對不上）
 *   - filter 值打錯 → 後端當成不篩，回全部；畫面看起來完全正常
 *   - 顏色表少一個值 → 那個狀態靜靜落 `gray`
 *   - 顯示對照沒有 default 分支 → 後端上線新值那天整頁白畫面，而測試用的是舊值域
 */

/** 58 §6.1.11a 的 8 個 KPI key（封閉列舉、順序固定）。 */
const EXPECTED_KPI_KEYS = [
	"pending_tax_reviews",
	"near_deadline",
	"provider_errors",
	"pending_jobs",
	"needs_manual",
	"open_complaints",
	"overdue_complaints",
	"suspended_sellers",
];

/** 端點 38 的 filter 值域 ＋ 空字串（不篩）。 */
const EXPECTED_FILTERS = [
	"",
	"NEW",
	"SUPPLEMENT_REQUIRED",
	"UNDER_REVIEW",
	"NEAR_DEADLINE",
	"EXTENDED",
	"EXPIRED",
];

test("A01：8 個 KPI key 逐字相同且順序固定", () => {
	assert.deepEqual(KPI_ORDER, EXPECTED_KPI_KEYS);
});

test("A01：四張『集合口徑』的卡不得帶單值篩選", () => {
	// KPI 的口徑是一組狀態的集合，而列表端點的篩選只吃一個值 ⇒ 帶了會少算，
	// 而兩個數字對不上這件事沒有任何訊號。
	for (const key of [
		"pending_tax_reviews",
		"provider_errors",
		"pending_jobs",
		"open_complaints",
	]) {
		assert.equal(
			KPI_META[key].href?.includes("?"),
			false,
			`${key} 的導向不得帶 query`,
		);
	}
	// 反向：兩張橫切視圖的卡本來就該帶。
	assert.match(KPI_META.near_deadline.href ?? "", /\?filter=NEAR_DEADLINE$/u);
	assert.match(KPI_META.overdue_complaints.href ?? "", /\?tab=OVERDUE$/u);
});

test("A01：停權卡導向商家清單的停權篩選，說明指向商家詳情的措施歷史（沒有違規頁）", () => {
	assert.equal(
		KPI_META.suspended_sellers.href,
		"/admin/merchants?filter=suspended",
	);
	assert.equal(
		KPI_META.suspended_sellers.note,
		"點擊後看到的是全部停權中的賣家；個別賣家的措施請到商家詳情的「措施歷史」查看。",
	);
});

test("A01：未知 KPI key 落 default 分支（不是白畫面、不是靜默消失）", () => {
	const meta = kpiMetaOf("brand_new_kpi_from_backend");
	assert.equal(meta.label, "brand_new_kpi_from_backend");
	assert.equal(meta.variant, "default");
	assert.equal(meta.href, undefined, "未知 KPI 不得有導向（導去哪都是猜的）");
});

test("A01：KPI 空字串是『取不到』不是 0", () => {
	// suspended_sellers 的權威在 seller-service，取不到時後端回空字串。
	// 印成 0 會看起來像「今天沒有人被停權」。
	assert.equal(kpiDisplayValue("", false), "取不到");
	assert.equal(kpiDisplayValue("0", false), "0");
	assert.equal(kpiDisplayValue("12", true), "--");
});

test("A02：六個 filter 值逐字相同（打錯不報錯，只會回全部）", () => {
	assert.deepEqual([...TAX_REVIEW_FILTER_VALUES], EXPECTED_FILTERS);
});

test("A02：期限分級與文字（顏色不得是唯一訊號）", () => {
	assert.deepEqual(deadlineDisplay("42"), { text: "剩 42 天", tone: "" });
	assert.deepEqual(deadlineDisplay("7"), { text: "剩 7 天", tone: "warning" });
	assert.deepEqual(deadlineDisplay("0"), { text: "剩 0 天", tone: "warning" });
	assert.deepEqual(deadlineDisplay("-2"), {
		text: "已逾期 2 天",
		tone: "danger",
	});
	// 後端算不出來時明說，不得退化成日曆天。
	assert.deepEqual(deadlineDisplay(""), {
		text: "處理期限計算中",
		tone: "",
	});
});

test("A02：佇列顯示收據代製授權，而且未授權要看得出來", async () => {
	// 🔴 端點 38 的 DTO 在 2026-09-05 之前沒有 receiptAuthorized ⇒ ADM 要逐一點進
	// A03 才知道哪些賣家已簽，而 T08「核定免用統一發票」的前置條件正是它
	// ——佇列的用途就是不必逐一點進去。本測試守後端補完之後前端真的用了它。
	const src = await readFile(
		new URL("../src/app/admin/invoices/tax-reviews/page.tsx", import.meta.url),
		"utf8",
	);
	assert.match(src, /item\.receiptAuthorized/u, "A02 沒有讀 receiptAuthorized");
	assert.match(src, /尚未授權/u, "未授權沒有文字，只有顏色不是可及的訊號");
	assert.match(src, /已授權/u);
	// 表頭與 colSpan 必須一起長：漏掉 colSpan 的症狀是空狀態列少一格、版面歪掉，
	// 而 tsc 與 lint 都不會叫。
	const headers = [...src.matchAll(/<th>/gu)].length;
	const colSpan = src.match(/colSpan=\{(\d+)\}/u);
	assert.ok(colSpan, "找不到空狀態列的 colSpan");
	assert.equal(
		Number(colSpan[1]),
		headers,
		`colSpan=${colSpan[1]} 與表頭 ${headers} 欄對不上`,
	);
});

test("A02：待審狀態集合等於 A01 pending_tax_reviews 的口徑", () => {
	assert.deepEqual([...PENDING_REVIEW_STATUSES].sort(), [
		"SELF_ISSUE_PENDING",
		"SUBMITTED",
		"SUPPLEMENT_REQUIRED",
		"UNDER_REVIEW",
	]);
});

test("顏色表：job 狀態 20 值與 CodeMeta 鍵集合相等", () => {
	assert.deepEqual(
		Object.keys(INVOICE_OPERATION_STATUS_BADGE_VARIANT).sort(),
		Object.keys(InvoiceOperationStatus).sort(),
	);
});

test("顏色表：稅務審核狀態 ＝ CodeMeta 的全部值（含 NOT_STARTED）", () => {
	// 2026-08-30：CodeMeta 已補上 NOT_STARTED（backend `業務狀態碼.md`），所以本條
	// 依原註解的指示改成單純相等 —— 顏色表不再需要自己補一個合成值。
	// 它仍然不進 DB（不在 chk_stp_review_status 的 10 值裡），那件事寫在 .md 的
	// 「DB 儲存欄位」那一行，不由本測試守。
	assert.deepEqual(
		Object.keys(TAX_REVIEW_STATUS_BADGE_VARIANT).sort(),
		Object.keys(SellerTaxReviewStatus).sort(),
	);
});

test("A01／A02 不得自動輪詢（限流桶與該管理員的其他請求共用）", async () => {
	const sources = await Promise.all([
		readFile("src/app/admin/invoices/page.tsx", "utf8"),
		readFile("src/app/admin/invoices/tax-reviews/page.tsx", "utf8"),
	]);
	for (const source of sources) {
		// 比對的是「真的呼叫／設定」的形狀，不是字面出現：兩頁的檔頭註解本來就會
		// 提到 refetchInterval（在說明為什麼不用它）。抓字面會讓這支測試逼人刪掉
		// 那段說明，而說明正是防止下一個人加回去的東西。
		assert.doesNotMatch(source, /refetchInterval\s*:/u);
		assert.doesNotMatch(source, /setInterval\(/u);
	}
});

test("Sidebar 有登記發票分組，且總覽帶 exact（沒有 gate 守這件事）", async () => {
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	assert.match(sidebar, /section: "發票 \/ 稅務"/u);
	for (const href of [
		"/admin/invoices",
		"/admin/invoices/tax-reviews",
		"/admin/invoices/provider-accounts",
		"/admin/invoices/jobs",
		"/admin/invoices/complaints",
		"/admin/invoices/documents",
		"/admin/invoices/audit-logs",
	]) {
		assert.ok(sidebar.includes(`href: "${href}"`), `Sidebar 缺 ${href}`);
	}
	// exact 少了的症狀：進任何子頁時「發票營運總覽」也會亮。
	const overviewEntry = sidebar.slice(
		sidebar.indexOf('href: "/admin/invoices",'),
	);
	assert.match(
		overviewEntry.slice(0, 200),
		/exact: true/u,
		"/admin/invoices 必須帶 exact: true",
	);
});

test("StateRow 三態齊全：error 不得被吞成『暫無資料』", async () => {
	const source = await readFile("src/components/UI/StateRow.tsx", "utf8");
	assert.match(source, /error: unknown/u, "error 必須是必填參數");
	assert.match(source, /role="alert"/u);
	assert.match(source, /onRetry/u);
	// 兩個佇列頁都要真的把 useQuery 的錯誤交給它，而不是只解構 data / isLoading。
	for (const path of [
		"src/app/admin/invoices/page.tsx",
		"src/app/admin/invoices/tax-reviews/page.tsx",
	]) {
		const page = await readFile(path, "utf8");
		assert.match(page, /isError/u, `${path} 沒有解構 isError`);
		assert.match(page, /error=\{isError \? error : null\}/u);
	}
});

test("ApiClientError 帶得動 envelope 的 data（preserveData 家族的價值全在裡面）", async () => {
	const [errorSource, apiSource] = await Promise.all([
		readFile("src/services/api-error.ts", "utf8"),
		readFile("src/services/api.ts", "utf8"),
	]);
	assert.match(errorSource, /readonly data\?: unknown/u);
	// code !== 0 的那個 throw 一定要把 data 帶上，否則按鈕點了沒反應。
	const bizThrow = apiSource.slice(apiSource.indexOf("if (data.code !== 0)"));
	assert.match(bizThrow.slice(0, 500), /data\.traceId,\s*\n\s*data\.data,/u);
});

// ===========================================================================
// A03–A08（第二波）
// ===========================================================================

import {
	COMPLAINT_STATUS_BADGE_VARIANT,
	PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT,
} from "../src/features/invoices/badge-variants";
import {
	COMPLAINT_SORT_VALUES,
	COMPLAINT_TAB_VALUES,
} from "../src/features/invoices/complaints";
import { JOB_TAB_VALUES, JOB_TABS } from "../src/features/invoices/jobs";
import {
	DEACTIVATABLE_STATUSES,
	HEALTH_CHECK_HIDDEN_STATUSES,
	PROVIDER_SYNC_TIMEOUT_MS,
	REACTIVATABLE_STATUSES,
	REVOCABLE_STATUSES,
} from "../src/features/invoices/provider-actions";
import {
	InvoiceComplaintStatus,
	InvoiceProviderAccountStatus,
} from "../src/lib/codemeta";

const INVOICE_PAGES = {
	a03: "src/app/admin/invoices/tax-reviews/[storeId]/page.tsx",
	a04List: "src/app/admin/invoices/provider-accounts/page.tsx",
	a04Detail: "src/app/admin/invoices/provider-accounts/[storeId]/page.tsx",
	a05: "src/app/admin/invoices/jobs/page.tsx",
	a06: "src/app/admin/invoices/jobs/[jobId]/page.tsx",
	a07: "src/app/admin/invoices/complaints/page.tsx",
	a08: "src/app/admin/invoices/complaints/[complaintId]/page.tsx",
	a13: "src/app/admin/invoices/documents/[documentId]/page.tsx",
} as const;

/** A04 的三支治理 dialog（58A 檔案總表 #8／#8b／#8c）。 */
const A04_DIALOGS = [
	"src/app/admin/invoices/provider-accounts/[storeId]/ProviderDeactivateDialog.tsx",
	"src/app/admin/invoices/provider-accounts/[storeId]/ProviderReactivateDialog.tsx",
	"src/app/admin/invoices/provider-accounts/[storeId]/ProviderRevokeDialog.tsx",
];

/**
 * 把註解拿掉之後再比對。
 *
 * 沒有這一步的話，下面幾條「這個欄名不得出現」的斷言會被自己的說明文字命中——而那
 * 會逼人為了讓測試變綠去刪掉解釋，正好刪掉最該留下的東西。反過來說，拿掉註解之後
 * 斷言才真的在守 code：把 `job.totalAmount` 加進列渲染就會轉紅。
 */
function stripComments(source: string): string {
	// 🔴 **不能用正則**。原本的寫法是「先 `replace(/\/\*[\s\S]*?\*\//g)` 再刪掉開頭是
	// `//` 的行」，而那對「**行註解裡出現 `/*`**」完全失守：A06 頁首有一行
	// `// …（`internal/grpc/*.proto` 的 rpc 清單）…`，那個 `/*` 開了一個假的區塊註解，
	// 一路吃到第 301 行才遇到下一個 `*/` ⇒ **整頁 12,671 字裡有 7,643 字（60%）
	// 對這個檔案的每一條斷言都是隱形的**，包含「不得呼叫未實作端點」那一條。
	// 實測：在 A06 加一行 `const DEAD = { method: "POST" };` 或 `api.post(...)`，
	// 兩者都不會轉紅。
	//
	// ⇒ 改成逐字掃描，狀態機認得字串／樣板字串／兩種註解。
	// ⚠️ 已知限制：regex literal 當成一般 code（`/\*/` 這種寫法會誤判）。頁面檔裡
	// 沒有這種寫法，而要正確處理它得先做 JS 的除法／regex 消歧義——代價遠大於收益。
	let out = "";
	let state: "code" | "line" | "block" | "'" | '"' | "`" = "code";
	for (let i = 0; i < source.length; i++) {
		const c = source[i];
		const next = source[i + 1];
		if (state === "code") {
			if (c === "/" && next === "/") {
				state = "line";
				i++;
			} else if (c === "/" && next === "*") {
				state = "block";
				i++;
			} else {
				if (c === "'" || c === '"' || c === "`") state = c;
				out += c;
			}
			continue;
		}
		if (state === "line") {
			// 換行本身保留，斷言的錯誤訊息才對得上行數。
			if (c === "\n") {
				state = "code";
				out += c;
			}
			continue;
		}
		if (state === "block") {
			if (c === "*" && next === "/") {
				state = "code";
				i++;
			} else if (c === "\n") {
				out += c;
			}
			continue;
		}
		// 字串內：原樣保留，跳過跳脫字元。
		out += c;
		if (c === "\\") {
			out += next ?? "";
			i++;
		} else if (c === state) {
			state = "code";
		}
	}
	return out;
}

const readCode = async (path: string) =>
	stripComments(await readFile(path, "utf8"));

/**
 * 把一段程式碼壓成正規形式：空白壓成單一空格、括號與大括號旁邊的空白去掉、括號前與結尾的逗號去掉。
 * 兩邊都過這個函式再比對，比對的是「寫了什麼」，不依賴 biome 怎麼換行或縮排。
 */
function squash(code: string): string {
	return code
		.replace(/\s+/gu, " ")
		.replace(/\s*([()[\]{}])\s*/gu, "$1")
		.trim()
		.replace(/,(?=[)\]}]|$)/gu, "");
}

/**
 * 原始碼裡每一次呼叫 `name(…)` 的引數文字，每次呼叫一筆（括號配對，字串與樣板字串裡的括號不算），過 `squash`。
 * 前面接著 `.`、字母或數字的不算（`obj.name(`、`resetname(`）。呼叫前先用 `readCode` 拿掉註解。
 */
function callArguments(source: string, name: string): string[] {
	const calls: string[] = [];
	const start = new RegExp(`(?<![\\w$.])${name}\\(`, "gu");
	for (const match of source.matchAll(start)) {
		const from = match.index + match[0].length;
		let depth = 1;
		let quote: string | null = null;
		let at = from;
		for (; at < source.length && depth > 0; at++) {
			const c = source[at];
			if (quote) {
				if (c === "\\") at++;
				else if (c === quote) quote = null;
			} else if (c === "'" || c === '"' || c === "`") quote = c;
			else if (c === "(") depth++;
			else if (c === ")") depth--;
		}
		calls.push(squash(source.slice(from, at - 1)));
	}
	return calls;
}

test("callArguments 取出呼叫的引數：不受換行、結尾逗號、字串裡的括號影響；不把別的名字算進來", () => {
	const source = [
		"setTarget({ id: 1 });",
		"setTarget(",
		"  { id: pick(a, b), label: 'x)' },",
		");",
		"obj.setTarget(9);",
		"resetTarget(8);",
		"setTarget(null);",
	].join("\n");
	assert.deepEqual(
		callArguments(source, "setTarget"),
		["{ id: 1 }", "{ id: pick(a, b), label: 'x)' }", "null"].map(squash),
	);
});

/**
 * `stripComments` 自己的 gate。
 *
 * 它是本檔**每一條**「這段 code 不得存在」斷言的前置處理 ⇒ 它多吃掉的每一個字元，
 * 都是那些斷言看不到的盲區，而盲區是完全靜默的（測試照樣全綠）。
 *
 * 第一條就是踩過的那一個：行註解裡的 `` `*.proto` `` 讓舊實作吃掉 A06 六成的原始碼。
 */
test("stripComments 只剝註解，不會多吃真正的 code", () => {
	const source = [
		"// 說明裡提到 `internal/grpc/*.proto` 的 rpc 清單",
		'const live = { method: "POST" };',
		"/* 這是真的區塊註解 */",
		'const url = "https://example.com/x";',
		'const literal = "/* 這在字串裡，不是註解 */";',
		"const tail = 1;",
	].join("\n");

	const out = stripComments(source);

	// 行註解裡的 `/*` 不得開啟區塊註解 —— 開了的話下面整段都會消失。
	assert.match(out, /method: "POST"/u, "行註解裡的 /* 把後面的 code 吃掉了");
	assert.match(out, /const tail = 1;/u);
	// 真的註解要剝掉。
	assert.doesNotMatch(out, /說明裡提到/u);
	assert.doesNotMatch(out, /這是真的區塊註解/u);
	// 字串裡的 `//` 與 `/* */` 都不是註解。
	assert.match(out, /https:\/\/example\.com/u);
	assert.match(out, /這在字串裡/u);
});

// --- A05：六個分頁的分割 -----------------------------------------------------

test("A05：六個分頁的聯集 ＝ InvoiceOperationStatus 的 20 值", () => {
	const union = JOB_TABS.flatMap((tab) => tab.statuses);
	assert.deepEqual(
		[...union].sort(),
		Object.values(InvoiceOperationStatus).sort(),
		"漏一個值 ⇒ 那個狀態的 job 在六頁裡都看不到（不是報錯，是消失）",
	);
});

test("A05：六個分頁兩兩交集為空，且分割數為 4+2+3+7+2+2", () => {
	const seen = new Map<string, string>();
	for (const tab of JOB_TABS) {
		for (const status of tab.statuses) {
			const previous = seen.get(status);
			assert.equal(
				previous,
				undefined,
				`${status} 同時落在 ${previous} 與 ${tab.value} ⇒ 同一筆會出現在兩頁，ADM 會處理兩次`,
			);
			seen.set(status, tab.value);
		}
	}
	assert.deepEqual(
		JOB_TABS.map((tab) => tab.statuses.length),
		[4, 2, 3, 7, 2, 2],
	);
});

test("A05：三個容易寫反的歸類", () => {
	const tabOf = (status: string) =>
		JOB_TABS.find((tab) => tab.statuses.includes(status))?.value;

	// 「送出去了但不知道結果」不是「正在重試」——歸錯會讓 ADM 以為系統在自己處理。
	assert.equal(
		tabOf(InvoiceOperationStatus.WAITING_CONFIRMATION),
		"NEEDS_MANUAL",
	);
	// 要動手的是賣家（去財政部申請字軌），不是平台。
	assert.equal(
		tabOf(InvoiceOperationStatus.SELLER_ACTION_REQUIRED),
		"NEEDS_ATTENTION",
	);
	// 取消還沒完成，而且可能卡在「provider 結果未明」。
	assert.equal(tabOf(InvoiceOperationStatus.CANCEL_REQUESTED), "NEEDS_MANUAL");
});

test("A05：SELLER_ACTION_REQUIRED 的 badge 必須與 FAILED_PERMANENT 不同色", () => {
	// NEEDS_ATTENTION 同時裝「ADM 要動手」與「賣家要動手」兩類，唯一的視覺區分就是
	// badge 顏色 ＋ 中文。兩者同色的話，管理員會把賣家該處理的案子攬過來查。
	assert.notEqual(
		INVOICE_OPERATION_STATUS_BADGE_VARIANT[
			InvoiceOperationStatus.SELLER_ACTION_REQUIRED
		],
		INVOICE_OPERATION_STATUS_BADGE_VARIANT[
			InvoiceOperationStatus.FAILED_PERMANENT
		],
	);
});

test("A05：分頁名只當 tab query 用，不拿去比對 job 狀態", async () => {
	const source = await readCode(INVOICE_PAGES.a05);
	// 分頁名不是 chk_invop_status 的值 ⇒ `status === "COMPLETED"` 這種比對恆為 false，
	// 而恆為 false 看起來就像「今天沒有這類案件」。
	for (const name of JOB_TAB_VALUES) {
		assert.doesNotMatch(
			source,
			new RegExp(`(?:status|Status)\\s*===\\s*"${name}"`, "u"),
			`${name} 是分頁名不是狀態值`,
		);
	}
});

test("A05：五個不在列表 DTO 上的欄名一律不得出現", async () => {
	const source = await readCode(INVOICE_PAGES.a05);
	// 端點 50 的 InvoiceJobInfo 沒有這五欄。憑語意造欄名的症狀是渲染 undefined，
	// TS 不擋、runtime 不擋。鑑別力：把 `job.totalAmount` 加進列渲染 → 本條轉紅。
	for (const field of [
		"paidAt",
		"invoiceMode",
		"fulfillmentMode",
		"totalAmount",
		"shipmentStatus",
	]) {
		assert.doesNotMatch(
			source,
			new RegExp(`\\b${field}\\b`, "u"),
			`${field} 不在端點 50 的回應上`,
		);
	}
});

// --- A06：不得出現的字面值 ---------------------------------------------------

test("A06：不得把分頁名／結果簡稱當成 job 狀態字面值", async () => {
	const source = await readCode(INVOICE_PAGES.a06);
	// 正則帶引號界定：`CANCELLED_BY_ADMIN` / `CANCELLED_BEFORE_ISSUE` 是合法值，
	// 裸 grep CANCELLED 會失去鑑別力。
	assert.doesNotMatch(
		source,
		/'(SELF_ISSUED_REPORTED|COMPLETED|CANCELLED)'|"(SELF_ISSUED_REPORTED|COMPLETED|CANCELLED)"/u,
	);
});

// --- A04／A07：顏色表的值域 --------------------------------------------------

test("A04：provider 顏色表 ＝ CodeMeta 的 13 值，且不含 SUSPENDING", () => {
	assert.deepEqual(
		Object.keys(PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT).sort(),
		Object.values(InvoiceProviderAccountStatus).sort(),
	);
	// DRAIN 整條分支已刪除。補一格 SUSPENDING 就是一個永遠不會命中的鍵，
	// 而下一個維護者會以為那個狀態存在。
	assert.equal(PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT.SUSPENDING, undefined);
	// PROVIDER_UNAVAILABLE 是「AMEGO 連不上」不是「這個賣家設定錯了」——
	// 畫成 danger 會讓 ADM 去跟賣家要 AppKey。
	assert.equal(
		PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[
			InvoiceProviderAccountStatus.PROVIDER_UNAVAILABLE
		],
		"warning",
	);
	// 三個「還沒設定好」的值必須是三種不同的呈現，不得被壓成同一態。
	assert.notEqual(
		PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[
			InvoiceProviderAccountStatus.NOT_STARTED
		],
		PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[
			InvoiceProviderAccountStatus.CONFIG_ERROR
		],
	);
});

test("A07：申訴顏色表 ＝ CodeMeta 的 7 值，不多收兩個不存在的狀態", () => {
	assert.deepEqual(
		Object.keys(COMPLAINT_STATUS_BADGE_VARIANT).sort(),
		Object.values(InvoiceComplaintStatus).sort(),
	);
	for (const ghost of ["PLATFORM_ASSIST_REQUESTED", "REOPENED"]) {
		assert.equal(
			COMPLAINT_STATUS_BADGE_VARIANT[ghost],
			undefined,
			`${ghost} 不在 chk_invcomp_status 的值域裡`,
		);
	}
});

test("A07：五個分頁與兩個排序值逐字相同", () => {
	assert.deepEqual(COMPLAINT_TAB_VALUES, [
		"AWAITING_SELLER",
		"SELLER_RESPONDED",
		"ADM_REVIEW",
		"OVERDUE",
		"CLOSED",
	]);
	assert.deepEqual(COMPLAINT_SORT_VALUES, ["due_at_asc", "created_at_desc"]);
});

test("A07：逾期橫幅的筆數來自獨立查詢，不是數當前分頁的 records", async () => {
	const source = await readCode(INVOICE_PAGES.a07);
	// 人不會先切到「已逾期」才知道有逾期 ⇒ 橫幅必須在任何分頁都看得到，
	// 所以它是同一支端點的獨立查詢（只讀 total）。
	assert.match(source, /tab=OVERDUE&page=1&page_size=1/u);
	assert.match(source, /"overdueCount"/u);
	// 五個分頁互相包含 ⇒ 不得顯示合計、不得用減法算任何一頁。
	assert.doesNotMatch(source, /合計/u);
});

// --- 顯示對照一律留 default 分支 ---------------------------------------------

test("四張顏色表都不是 exhaustive：未知值查得到 undefined，由頁面的 ?? 接住", () => {
	// 後端先上線一個前端還不知道的值時，這四張表回 undefined，頁面的 `?? "gray"`
	// 接住它 ⇒ badge 灰色但畫面還在。少了 default 分支就是整頁白畫面，而測試用的是
	// 舊值域所以全綠。
	for (const map of [
		INVOICE_OPERATION_STATUS_BADGE_VARIANT,
		TAX_REVIEW_STATUS_BADGE_VARIANT,
		PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT,
		COMPLAINT_STATUS_BADGE_VARIANT,
	]) {
		assert.equal(map.__A_VALUE_THE_FRONTEND_DOES_NOT_KNOW__, undefined);
	}
});

test("每一頁的 badge 查表都有 ?? 兜底", async () => {
	for (const [name, path] of Object.entries(INVOICE_PAGES)) {
		const source = await readCode(path);
		const lookups = source.match(/_BADGE_VARIANT\[[^\]]+\]/gu) ?? [];
		if (lookups.length === 0) continue;
		assert.match(
			source,
			/_BADGE_VARIANT\[[\s\S]{0,120}?\]\s*\?\?/u,
			`${name}（${path}）的顏色查表沒有 ?? 兜底`,
		);
	}
});

// --- 每一頁只准打自己那幾支端點 ---------------------------------------------

/**
 * 全部 ADM 發票寫入端點的路徑尾段（逐字取自
 * `gateway/internal/router/constants.go` 的 `AdminInvoice*` 常數）。
 *
 * 🔴 **這份清單是本節每一條斷言的觀測窗**：不在清單裡的端點，下面的比對看不見它。
 * 所以新增任何 ADM 發票寫入端點時要先加進這裡——漏加的症狀是那支端點被靜默放行。
 * （gateway 那一側新增路由時同步加，兩邊都在同一個 PR 內。）
 */
const ADM_INVOICE_WRITE_SEGMENTS = [
	"actions", // 端點 60　AdminInvoiceComplaintActions
	"approvals", // 端點 55　AdminInvoiceJobUnlockApproval
	"cancellations", // 端點 53　AdminInvoiceJobCancellations
	"deactivations", // 端點 45　AdminInvoiceProviderDeactivate
	"decisions", // 端點 40　AdminInvoiceTaxDecisions
	"extensions", // 端點 41　AdminInvoiceTaxExtensions
	"health_checks", // 端點 44　AdminInvoiceProviderHealth
	"issuances", // 端點 57　AdminInvoiceIssuances
	"manual_results", // 端點 56　AdminInvoiceJobManualResults
	"manual_unlocks", // 端點 54　AdminInvoiceJobManualUnlocks
	"notification_resends", // 端點 49　AdminInvoiceDocumentResend
	"reactivations", // 端點 66　AdminInvoiceProviderReactivations
	"retries", // 端點 52　AdminInvoiceJobRetries
	"revocations", // 端點 67　AdminInvoiceProviderRevocations
];

/**
 * 每一頁允許打的寫入端點。**沒有 key ＝ 一顆寫入鍵都不准有。**
 *
 * 進這份表的門檻是「四件全部實查過」：invoice-service 的 receiver 方法、Gateway
 * handler、`routes_invoice.go` 的掛載、以及該頁真的只打這幾支。移進來的那個 PR 要在
 * 說明裡列出證據。
 *
 * ⚠️ **2026-09-02 的教訓寫在這裡，因為它就是這份表存在的理由**：
 * 在那之前 a03／a06／a08／a13 四頁被判定為「後端沒實作」而整組不渲染，判定的依據是
 * 拿**猜的路徑名**去打 dev 拿到 404 —— `complaints/{id}/decisions`（真名 `.../actions`）、
 * `jobs/{id}/notification_resends`（它掛在 document 上）、
 * `jobs/{id}/external_issuances`（真名 `.../manual_results`）。九支 RPC 一直都在。
 * ⇒ **識別字取自路由註冊，不得取自記憶。** 404 只證明路徑打錯。
 */
const PAGE_WRITE_TARGETS: Partial<
	Record<keyof typeof INVOICE_PAGES, string[]>
> = {
	// 端點 44／45／66／67（`internal/grpc/provider_account.go`）。
	a04Detail: ["deactivations", "health_checks", "reactivations", "revocations"],
	// 端點 40／41（`internal/grpc/tax_profile.go`）。
	a03: ["decisions", "extensions"],
	// 端點 52／53／54／55／56（`internal/grpc/admin_write.go`＋`manual_unlock.go`）。
	a06: [
		"approvals",
		"cancellations",
		"manual_results",
		"manual_unlocks",
		"retries",
	],
	// 端點 60（`internal/grpc/complaint.go`）。
	a08: ["actions"],
	// 端點 49（`internal/grpc/notification_resend.go`）。它掛在 document 上不是 job 上，
	// 所以 A13 是唯一組得出這條路徑的地方（端點 51 的回應沒有 documentId）。
	a13: ["notification_resends"],
};

/**
 * 「這一頁有沒有寫入呼叫」。
 *
 * 🔴 **不能只掃 `api.post`**：本 repo 需要自訂 header 的寫入（發票全部的 POST 都要帶
 * `Idempotency-Key`，而 `api.post` 不收 header）走的是 `services/api.ts` 的 `request()`
 * ＋ `method: "POST"` ⇒ 只掃 `api.post` 的話，**這個 repo 實際使用的寫入路徑完全在
 * 偵測範圍外**，gate 一路綠燈。A11 手動開票就是這個形狀。
 */
const WRITE_CALL =
	/\bapi\.(?:post|put|patch|delete)\s*[<(]|method:\s*"(?:POST|PUT|PATCH|DELETE)"/u;

/**
 * 這一頁在原始碼裡出現過哪幾個寫入端點尾段。
 *
 * ⚠️ **兩種寫法都要認**：路徑可能直接寫在樣板字串裡（`` `.../notification_resends` ``），
 * 也可能經過頁面自己的 `actionPath("retries")` helper（A04／A06 都是後者）。
 * 只認前者的話，整頁的寫入目標會被讀成空集合而斷言仍然「通過」——那正是
 * fail-open：看不到就當作沒有。
 */
function writeSegmentsIn(source: string): string[] {
	return ADM_INVOICE_WRITE_SEGMENTS.filter((seg) =>
		new RegExp(`(?:/|["'])${seg}\\b`, "u").test(source),
	);
}

test("沒有登記寫入目標的發票頁，一顆寫入鍵都不准有", async () => {
	for (const [name, path] of Object.entries(INVOICE_PAGES)) {
		if (PAGE_WRITE_TARGETS[name as keyof typeof INVOICE_PAGES]) continue;
		const source = await readCode(path);
		assert.doesNotMatch(
			source,
			WRITE_CALL,
			`${name}（${path}）有寫入呼叫卻沒有登記在 PAGE_WRITE_TARGETS——` +
				"要嘛是打到不該打的端點，要嘛是登記漏了。兩種都要人看過。",
		);
	}
});

test("有寫入的發票頁，打到的端點集合逐一等於登記值", async () => {
	for (const [name, expected] of Object.entries(PAGE_WRITE_TARGETS)) {
		const path = INVOICE_PAGES[name as keyof typeof INVOICE_PAGES];
		const source = await readCode(path);

		assert.deepEqual(
			writeSegmentsIn(source),
			[...(expected ?? [])].sort(),
			`${name}（${path}）打到的發票寫入端點與登記值不符——` +
				"多出來的那一支是沒有人看過的，少掉的那一支是按鈕不見了。",
		);

		// `api.post` 帶不了 Idempotency-Key header，gateway 的
		// `RequireIdempotencyKey` 掛在整個 /admin/invoices 群組上 ⇒ 一律 400。
		assert.doesNotMatch(
			source,
			/\bapi\.(?:post|put|patch|delete)\s*[<(]/u,
			`${name}（${path}）用了 api.post——它帶不了 Idempotency-Key，gateway 會回 400`,
		);
	}
});

/**
 * `expectedVersion` 必須來自畫面上讀到的那筆資料。
 *
 * 它是樂觀鎖：兩位管理員同時開著同一頁時，後送出的那一位必須拿到版本衝突而不是
 * 覆蓋掉前一位的決策。寫死一個值（`"0"` 是最容易順手寫的）會讓那道保護整個消失，
 * 而**畫面上兩邊都會顯示成功**——沒有任何東西會轉紅。
 */
test("樂觀鎖的 expectedVersion 不得是字面值", async () => {
	for (const name of Object.keys(PAGE_WRITE_TARGETS)) {
		const path = INVOICE_PAGES[name as keyof typeof INVOICE_PAGES];
		const source = await readCode(path);
		if (!/expectedVersion/u.test(source)) continue;
		assert.doesNotMatch(
			source,
			/expectedVersion:\s*(?:"[^"]*"|\d+|`[^`]*`)/u,
			`${name}（${path}）把 expectedVersion 寫成字面值——樂觀鎖失效，而且不會有任何東西轉紅`,
		);
	}
});

/**
 * A03 的「決策 × 允許的來源狀態」逐列等於後端的 `taxDecisionAllowedFrom`。
 *
 * 🔴 這張表在**兩個方向**都是靜默的：
 *   - 多一個值 ⇒ 畫面上多一顆按鈕，按下去後端 CAS rows=0 回 9348，而 ADM 只看到
 *     一句「決策與狀態不相容」，不知道是自己按了一顆本來就不該存在的鍵。
 *   - 少一個值 ⇒ 合法的決策按鈕**不見了**。沒有錯誤、沒有紅字，只是那個決策做不到。
 *
 * 期望值逐字抄自 `services/invoice-service/internal/usecase/tax_profile.go` 的
 * `taxDecisionAllowedFrom`（＝ 58 §7.3.3 每一列轉移的來源狀態欄）。
 * ⚠️ `SUPPLEMENT_REQUIRED` 不在任何一列（補齊後由 T06 先推回 UNDER_REVIEW）；
 * ⚠️ `EXTEND` 是 `SellerTaxReviewDecision` 的第 8 值但不是端點 40 的決策。
 */
test("A03：七個決策的來源狀態逐列對上後端的 allowed-from", () => {
	assert.deepEqual(
		Object.fromEntries(
			Object.entries(TAX_DECISION_ALLOWED_FROM).map(([k, v]) => [
				k,
				[...v].sort(),
			]),
		),
		{
			APPROVE_INVOICE_REQUIRED: ["SELF_ISSUE_PENDING", "UNDER_REVIEW"],
			APPROVE_INVOICE_EXEMPT: ["SELF_ISSUE_PENDING", "UNDER_REVIEW"],
			REJECT: ["SELF_ISSUE_PENDING", "UNDER_REVIEW"],
			REQUEST_SUPPLEMENT: ["UNDER_REVIEW"],
			KEEP_UNDER_REVIEW: ["UNDER_REVIEW"],
			REVOKE: ["APPROVED_INVOICE_EXEMPT", "APPROVED_INVOICE_REQUIRED"],
			REOPEN_REVIEW: [
				"APPROVED_INVOICE_EXEMPT",
				"APPROVED_INVOICE_REQUIRED",
				"EXPIRED",
				"REVOKED",
			],
		},
		"與 tax_profile.go 的 taxDecisionAllowedFrom 漂移了——兩個方向都是靜默的",
	);

	// 顯示順序要蓋滿整張表，否則某個決策在畫面上永遠不會出現。
	assert.deepEqual(
		[...TAX_DECISION_ORDER].sort(),
		Object.keys(TAX_DECISION_ALLOWED_FROM).sort(),
		"TAX_DECISION_ORDER 沒有蓋滿 allowed-from：漏掉的那個決策永遠不會被渲染",
	);

	// 每個 key 都必須是 CodeMeta 的值，不能是手打的字串。
	for (const decision of Object.keys(TAX_DECISION_ALLOWED_FROM)) {
		assert.ok(
			Object.values(SellerTaxReviewDecision).includes(
				decision as (typeof SellerTaxReviewDecision)[keyof typeof SellerTaxReviewDecision],
			),
			`${decision} 不在 CodeMeta SellerTaxReviewDecision 裡`,
		);
	}
	// 端點 41 專用的 EXTEND 不得混進端點 40 的決策表。
	assert.ok(
		!("EXTEND" in TAX_DECISION_ALLOWED_FROM),
		"EXTEND 是端點 41 的稽核 reason_code，不是端點 40 的決策——放進來會多一顆打錯端點的鍵",
	);
});

/**
 * A03 的「收據代製授權」前置條件（2026-09-05，`DEC-20260905-015836`）。
 *
 * 三件事各自靜默，所以三件都要有斷言：
 *   - 哪些狀態下賣家簽得了（端點 29 的 allowed-from）——少一個 ⇒ 那個狀態下 ADM
 *     看不到授權狀態，而他正要按下核定。
 *   - T08 只拘束 `APPROVE_INVOICE_EXEMPT` ——多拘束一個 ⇒ 那個裁決永遠按不下去，
 *     沒有錯誤、沒有紅字。
 *   - 9354 仍然要有分流 ——前端擋住不代表它不會回來（賣家可以在這一頁載入之後才
 *     取消授權）。
 */
test("A03：收據代製授權簽得了的狀態逐列對上端點 29 的 allowed-from", () => {
	assert.deepEqual(
		[...RECEIPT_AUTH_SIGNABLE_STATUSES].sort(),
		[
			"APPROVED_INVOICE_EXEMPT",
			"SELF_ISSUE_PENDING",
			"SUBMITTED",
			"SUPPLEMENT_REQUIRED",
			"UNDER_REVIEW",
		],
		"與 invoice-service 端點 29 的 allowed-from 漂移了——" +
			"少一個狀態 ⇒ ADM 在那個狀態下看不到授權欄，而核定的前置條件正是它",
	);
	for (const status of RECEIPT_AUTH_SIGNABLE_STATUSES) {
		assert.ok(
			Object.values(SellerTaxReviewStatus).includes(
				status as (typeof SellerTaxReviewStatus)[keyof typeof SellerTaxReviewStatus],
			),
			`${status} 不在 CodeMeta SellerTaxReviewStatus 裡`,
		);
	}
});

test("A03：未授權只擋得住「核定免用統一發票」一顆", () => {
	assert.equal(
		taxDecisionBlockedByReceiptAuth(
			SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
			false,
		),
		true,
		"未授權時「核定免用統一發票」必須被擋——否則 ADM 按下去才吃到 9354，而他自己解決不了",
	);
	assert.equal(
		taxDecisionBlockedByReceiptAuth(
			SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
			true,
		),
		false,
		"已授權還擋著 ⇒ 一個免用發票賣家都核定不了",
	);
	for (const decision of TAX_DECISION_ORDER) {
		if (decision === SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT) continue;
		for (const authorized of [true, false]) {
			assert.equal(
				taxDecisionBlockedByReceiptAuth(decision, authorized),
				false,
				`${decision} 不受 T08 拘束（後端只對 APPROVE_INVOICE_EXEMPT 回 9354）——` +
					"擋住它不會有任何東西轉紅，只是那個裁決永遠按不下去",
			);
		}
	}
});

test("A03：授權欄與 9354 分流都真的接在 code 上", async () => {
	const source = await readCode(INVOICE_PAGES.a03);

	assert.match(
		source,
		/receiptAuthRelevant\(reviewStatus\)/u,
		"授權欄的顯示條件不是 receiptAuthRelevant——" +
			"寫死成單一狀態的話，決策按鈕按得到的那幾個狀態下它不會出現",
	);
	assert.match(
		source,
		/disabled=\{[^}]*taxDecisionBlockedByReceiptAuth\(/u,
		"「核定免用統一發票」沒有接上 T08 前置條件 ⇒ ADM 按下去才會知道被擋",
	);
	assert.match(
		source,
		/code === TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE/u,
		"9354 沒有分流——前端擋住不代表它不會回來（賣家可在載入後取消授權），" +
			"沒有分流就會退化成一句沒有下一步的紅字",
	);
	assert.ok(
		!ACTION_CONFLICT_CODES.has(TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE),
		"9354 混進 ACTION_CONFLICT_CODES ⇒ 會給出一句叫人重新整理的話，" +
			"而 ADM 重整幾次都沒用，下一步是通知賣家去簽",
	);
});

/**
 * A08 三個會轉狀態的動作只能從 `ESCALATED_TO_ADM` 出發。
 *
 * 逐字等於 `invoice_complaints.sql` 兩支 CAS 的 `AND status = 'ESCALATED_TO_ADM'`
 * （`DecideEscalatedInvoiceComplaint` 與 `ReturnInvoiceComplaintToSeller`），
 * rows=0 一律回 9336。放寬 ⇒ 多一顆按下去必死的鍵。
 *
 * 另兩個動作（提醒賣家／建議暫停接單）在後端沒有狀態守衛，那兩列是本 repo 自訂的
 * 渲染條件——所以這裡只斷言**它們不是空的**，不斷言內容（內容不是契約鏡像）。
 */
test("A08：三個會轉狀態的動作只准從 ESCALATED_TO_ADM 出發", () => {
	for (const action of [
		COMPLAINT_ACTION_REQUEST_REWORK,
		COMPLAINT_ACTION_DECIDE_RESOLVED,
		COMPLAINT_ACTION_DECIDE_VIOLATION,
	]) {
		assert.deepEqual(
			COMPLAINT_ACTION_ALLOWED_FROM[action],
			["ESCALATED_TO_ADM"],
			`${action} 的來源狀態與 invoice_complaints.sql 的 CAS 述詞漂移了`,
		);
	}
	for (const action of [
		COMPLAINT_ACTION_REMIND_SELLER,
		COMPLAINT_ACTION_RECOMMEND_SUSPENSION,
	]) {
		assert.ok(
			(COMPLAINT_ACTION_ALLOWED_FROM[action] ?? []).length > 0,
			`${action} 的渲染條件是空集合——那顆鍵永遠不會出現`,
		);
	}
	assert.deepEqual(
		[...COMPLAINT_ACTION_ORDER].sort(),
		Object.keys(COMPLAINT_ACTION_ALLOWED_FROM).sort(),
		"COMPLAINT_ACTION_ORDER 沒有蓋滿 allowed-from",
	);
	// 五個動作都要有中文與後果說明，否則畫面上會出現一顆沒有標籤的鍵。
	for (const action of COMPLAINT_ACTION_ORDER) {
		assert.ok(COMPLAINT_ACTION_LABEL[action], `${action} 沒有顯示文字`);
		assert.ok(COMPLAINT_ACTION_CONSEQUENCE[action], `${action} 沒有後果說明`);
	}
});

/**
 * 端點 41 的 decision 不得借用 `SellerTaxReviewDecision` 的值。
 *
 * 58 §6.1.12 逐字「端點 41 自己的 Decision 欄（APPROVE／REJECT）是另一組值域」。
 * 送 `EXTEND` 過去會拿到 `decision is outside the defined value set`——一顆 100% 失敗
 * 的按鈕，而失敗訊息看起來像是使用者做錯了什麼。
 */
test("A03：延長裁決的兩個值不是 SellerTaxReviewDecision 的成員", () => {
	assert.equal(TAX_EXTENSION_APPROVE, "APPROVE");
	assert.equal(TAX_EXTENSION_REJECT, "REJECT");
	// ⚠️ 這裡**不能**斷言「兩個值都不在 SellerTaxReviewDecision 裡」：`REJECT` 兩邊
	// 恰好同字（`SellerTaxReviewDecision.REJECT` 也是 "REJECT"），但那是巧合不是關係
	// ——一個是端點 40 的「駁回這筆申請」，一個是端點 41 的「駁回這次延長」。
	// 真正的不變式是這一條：`APPROVE` 只存在於端點 41，而 `EXTEND`（端點 41 的稽核
	// reason_code）絕不可被當成端點 41 的 decision 送出去。
	assert.ok(
		!Object.values(SellerTaxReviewDecision).includes(
			TAX_EXTENSION_APPROVE as (typeof SellerTaxReviewDecision)[keyof typeof SellerTaxReviewDecision],
		),
		"APPROVE 撞到了 SellerTaxReviewDecision——端點 41 的值域被污染了",
	);
	assert.notEqual(
		TAX_EXTENSION_APPROVE,
		SellerTaxReviewDecision.EXTEND,
		"端點 41 送 EXTEND 會拿到 decision is outside the defined value set（100% 失敗的按鈕）",
	);
	// 上界與預設值：APPROVE 時 1–365 之外一律 400。
	assert.equal(TAX_EXTENSION_DEFAULT_DAYS, 30);
	assert.equal(TAX_EXTENSION_MAX_DAYS, 365);
});

/**
 * A04 詳情頁的寫入**只准打那四支已實作的端點**。
 *
 * 上面那條 gate 對這一頁已經不適用（它是 WRITE_ENABLED），所以要有一條反向的：
 * 把它移出「禁止寫入」清單的同時，如果不補這一條，這一頁就變成整個發票模組裡
 * 唯一沒有人看管寫入目標的頁面——加一顆打端點 49（未實作）的按鈕不會有任何東西轉紅。
 */
test("A04 詳情頁的寫入只打 44／45／66／67 四支", async () => {
	const source = await readCode(INVOICE_PAGES.a04Detail);

	// 錨在頁面自己的路徑組裝 helper 上：改名就要同步改這支測試，而不是靜靜地不驗。
	assert.match(
		source,
		/const actionPath = \(action: string\) =>/u,
		"找不到 actionPath——路徑組裝改寫了就要同步改這支測試，不是刪掉它",
	);

	const targets = [...source.matchAll(/actionPath\("([a-z_]+)"\)/gu)].map(
		(m) => m[1],
	);
	assert.deepEqual(
		[...new Set(targets)].sort(),
		["deactivations", "health_checks", "reactivations", "revocations"],
		`A04 詳情頁打到了非預期的端點：${targets.join(", ")}`,
	);

	// 每一支寫入都要走 actionPath。數量對不上 ＝ 有人繞過去自己拼了一條路徑。
	const posts = source.match(/method:\s*"POST"/gu) ?? [];
	assert.equal(
		posts.length,
		targets.length,
		`POST 有 ${posts.length} 處但 actionPath 只有 ${targets.length} 處——有寫入沒有走 actionPath`,
	);
	assert.doesNotMatch(
		source,
		/\bapi\.(?:post|put|patch|delete)\s*[<(]/u,
		"api.post 帶不了 Idempotency-Key header，gateway 會回 400",
	);
});

/**
 * 端點 44／66 必須帶 130 秒逾時（58A 陷阱 T10）。
 *
 * 兩支都在請求路徑上同步等 AMEGO。用 `api.ts` 預設的 10 秒，症狀不是「失敗」而是
 * **「按了說逾時，重新整理卻看到結果已經變了」**——後端仍在跑而且結果會落庫。
 */
test("A04：端點 44／66 帶 130 秒逾時，45／67 不帶", async () => {
	assert.equal(
		PROVIDER_SYNC_TIMEOUT_MS,
		130000,
		"同步等 AMEGO 的逾時被改了——gateway 那一側的 deadline 是 2 分鐘，前端要比它晚放棄",
	);

	const source = await readCode(INVOICE_PAGES.a04Detail);
	const SYNC_ACTIONS = new Set(["health_checks", "reactivations"]);
	const segments = source.split(/actionPath\("/u).slice(1);
	assert.equal(segments.length, 4, "actionPath 的呼叫點數量變了");

	for (const segment of segments) {
		const action = segment.slice(0, segment.indexOf('"'));
		// 只看緊接其後的 request options 物件，不要讓 onSuccess／onError 的內容混進來。
		const options = segment.slice(0, 400);
		assert.equal(
			/timeoutMs: PROVIDER_SYNC_TIMEOUT_MS/u.test(options),
			SYNC_ACTIONS.has(action),
			SYNC_ACTIONS.has(action)
				? `${action} 同步等 AMEGO 卻沒有帶 timeoutMs——每次都會在 10 秒被前端 abort`
				: `${action} 不等第三方，不該帶 130 秒逾時`,
		);
	}
});

/**
 * `api.ts` 真的會用 `timeoutMs`。
 *
 * 沒有這一條，上面那支只驗到「頁面寫了這個字」——`request()` 把它當成無效參數丟掉的話，
 * 兩支端點照樣在 10 秒被 abort，而所有測試全綠。
 */
test("api.ts 的 request() 真的會套用 timeoutMs", async () => {
	const api = await readFile("src/services/api.ts", "utf8");
	assert.match(api, /timeoutMs\?:\s*number/u, "RequestOptions 沒有 timeoutMs");
	assert.match(
		api,
		/setTimeout\(\(\) => controller\.abort\(\), timeoutMs\)/u,
		"AbortController 的逾時沒有讀 timeoutMs（還是寫死的常數）",
	);
});

/**
 * 五顆 CTA 的渲染條件逐一對上 58A §A04 的表。
 *
 * 放寬任何一組就是造出一顆按了必死的按鈕（例：對 `ACTIVE` 顯示「恢復連線」⇒
 * `ReactivateForAdmin` 的 `RequireStatus` 不符 ⇒ 9333），而放寬這件事本身不會讓
 * 任何既有測試轉紅。
 */
test("A04：五顆 CTA 的來源狀態集合逐一對上 58A", () => {
	assert.deepEqual([...HEALTH_CHECK_HIDDEN_STATUSES].sort(), [
		"NOT_CONFIGURED",
		"NOT_STARTED",
		"REVOKED",
	]);
	assert.deepEqual([...DEACTIVATABLE_STATUSES].sort(), ["ACTIVE", "DEGRADED"]);
	// 「只有 SUSPENDED」——多一個值就是一顆必死的按鈕。
	assert.deepEqual([...REACTIVATABLE_STATUSES], ["SUSPENDED"]);
	assert.deepEqual([...REVOCABLE_STATUSES].sort(), [
		"ACTIVE",
		"DEGRADED",
		"SUSPENDED",
	]);
	// 四組都必須是 CodeMeta 的合法值：打錯字的話那顆 CTA 會永遠不出現（靜默）。
	const legal = new Set<string>(Object.values(InvoiceProviderAccountStatus));
	for (const set of [
		HEALTH_CHECK_HIDDEN_STATUSES,
		DEACTIVATABLE_STATUSES,
		REACTIVATABLE_STATUSES,
		REVOCABLE_STATUSES,
	]) {
		for (const value of set) {
			assert.ok(
				legal.has(value),
				`${value} 不是 InvoiceProviderAccountStatus 的值`,
			);
		}
	}
});

/**
 * A04 全頁（含三支 dialog）不得有任何顯示完整 AppKey 或憑證指紋的路徑（58A §A04 驗收）。
 *
 * `credentialFingerprint` 不在 58 §6 的兩個 provider DTO 上——外露 HMAC 指紋是一個
 * 沒有被裁決過的暴露面，畫面規格不能單方面決定。完整 AppKey 則是**根本不存在**
 * （proto 的 reserved 90 保證它加不回去），所以這裡要擋的是「前端自己造一個欄位去讀」。
 */
test("A04：全頁沒有顯示完整 AppKey 或憑證指紋的路徑", async () => {
	for (const path of [
		INVOICE_PAGES.a04List,
		INVOICE_PAGES.a04Detail,
		...A04_DIALOGS,
	]) {
		const source = await readCode(path);
		assert.doesNotMatch(
			source,
			/fingerprint/iu,
			`${path} 出現 fingerprint——58A 定版「本波一律不顯示」（含只印前 8 碼）`,
		);
		assert.doesNotMatch(
			source,
			/\.appKey\b/u,
			`${path} 讀了 appKey——沒有任何 RPC 回傳明文，這個欄位讀出來只會是 undefined`,
		);
	}
});

test("A08：只有「依建議建立措施」帶預填（停權、結束時間、建議理由），常駐的「建立措施」不帶；建議卡的欄標題是「建議理由」", async () => {
	const source = await readCode(INVOICE_PAGES.a08);
	assert.match(source, /<Field label="建議理由" full>/u);
	assert.doesNotMatch(source, /可直接複製到措施的理由/u);
	// 預填只經 measureInitialFromSuggestion，在點下去的當下以現在的時間算，只有建議卡的鈕用它。
	assert.equal(source.match(/measureInitialFromSuggestion\(/gu)?.length, 1);
	// 對話框的目標只在兩處設定（關閉時的 null 不算），逐一比對傳進去的物件，而不是掃整頁有沒有某個字：
	// 建議卡的鈕帶店家與建議的預填，常駐的「建立措施」只帶店家，所以兩者都沒有 category；
	// 類別不預填：發票沒有對應的政策分類，由管理員選。
	const targets = callArguments(source, "setMeasureTarget").filter(
		(argument) => argument !== "null",
	);
	assert.deepEqual(
		targets.toSorted(),
		[
			"{ storeId: complaint.storeId }",
			"{ storeId: recommendation.storeId || complaint.storeId, initial: measureInitialFromSuggestion(recommendation, new Date()) }",
		]
			.map(squash)
			.toSorted(),
	);
	// 對話框收到的初始值就是這個狀態的 initial。
	assert.match(source, /initial=\{measureTarget\.initial\}/u);
});

test("A08：措施只經建立措施對話框的 CreateMeasure 建立，不呼叫發票模組或舊的停權端點", async () => {
	const source = await readCode(INVOICE_PAGES.a08);
	// 停權狀態的權威是賣家治理模組。發票模組另開一套 ⇒ 兩份狀態、兩個時鐘，
	// ADM 在一邊解除、另一邊還鎖著：本頁不呼叫舊的停權端點。
	assert.doesNotMatch(source, /api\/v1\/admin\/violations/u);
	assert.doesNotMatch(source, /suspensions/u);
	// 舊的違規建立頁已刪除：不得再導過去。
	assert.doesNotMatch(source, /\/admin\/violations/u);
	// 本頁用到的賣家治理方法只有 CreateMeasure（沒有結束、撤銷、註記這類的寫入）。
	const governanceMethods = new Set(
		[...source.matchAll(/SellerGovernanceAdminService\.method\.(\w+)/gu)].map(
			(m) => m[1],
		),
	);
	assert.deepEqual([...governanceMethods], ["createMeasure"]);
	// 它只在 handleCreateMeasure 裡送出，而 handleCreateMeasure 只交給建立措施對話框的 onConfirm。
	assert.equal(
		source.match(/createMeasureMutation\.mutateAsync\(/gu)?.length,
		1,
		"CreateMeasure 只能有一個送出點",
	);
	assert.match(
		source,
		/const handleCreateMeasure = async \([^)]*\) => \{\s*await createMeasureMutation\.mutateAsync\(request\)/u,
	);
	assert.equal(source.match(/handleCreateMeasure/gu)?.length, 2);
	assert.match(source, /onConfirm=\{handleCreateMeasure\}/u);
	assert.match(
		source,
		/from "@\/app\/admin\/merchants\/\[id\]\/CreateMeasureDialog"/u,
	);
	// category 不開給發票模組預填：invoice_not_issued 不在認定類別的封閉值域。
	assert.doesNotMatch(source, /invoice_not_issued/u);
});

// --- Sidebar 登記（C13：ADM 沒有路由清冊 gate，這裡補一個）-------------------

/**
 * 刻意不進 Sidebar 的非動態發票路由。
 *
 * 目前只有 A11：58A §A11 逐字「**Sidebar 不放這一項**：它是低頻的例外出口，放進選單
 * 會讓人以為手動開票是常規動作，而固定警示逐字寫著『請勿以手動開票取代系統工作』」。
 *
 * ⚠️ 豁免不等於免驗：C13 要防的是「頁面存在但沒有人找得到」，而 Sidebar 只是入口的
 * 一種。下面第二支測試改驗這些路由**至少被發票模組內的某個檔案連到**——不然這份
 * 豁免清單就變成一個把新頁面靜靜藏起來的後門。
 */
const SIDEBAR_EXEMPT_ROUTES = ["/admin/invoices/issuances/new"];

async function invoiceStaticRoutes(): Promise<string[]> {
	const roots = await readdir("src/app/admin/invoices", {
		withFileTypes: true,
		recursive: true,
	});
	return roots
		.filter((e) => e.isFile() && e.name === "page.tsx")
		.map((e) =>
			(e.parentPath ?? "").replace(/^src\/app/u, "").replaceAll("\\", "/"),
		)
		.filter((route) => !route.includes("["));
}

test("每個非動態的發票頁都登記在 Sidebar（豁免清單除外）", async () => {
	// 58A §0.6 C13：ADM 新增頁面漏了 Sidebar 登記**沒有任何 gate** ⇒ 頁面存在、
	// 路由通、但沒有人找得到它。這一條就是那個 gate。
	//
	// 反方向（Sidebar 有、頁面還沒建）刻意不驗：58A §B-3.1.1 要求 7 項一次登記完，
	// 而 A10／A12 的頁面在後續波次。
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	const routes = await invoiceStaticRoutes();

	assert.ok(routes.length >= 5, `掃到的發票路由太少：${routes.join(", ")}`);
	// 豁免清單裡的路由必須真的存在：留一個已刪頁面的豁免，等於替下一個同名路由
	// 預先開好一張免死金牌。
	for (const exempt of SIDEBAR_EXEMPT_ROUTES) {
		assert.ok(
			routes.includes(exempt),
			`${exempt} 在豁免清單裡但頁面不存在——過期的豁免要刪掉`,
		);
	}
	for (const route of routes) {
		if (SIDEBAR_EXEMPT_ROUTES.includes(route)) continue;
		assert.match(
			sidebar,
			new RegExp(`href:\\s*"${route}"`, "u"),
			`${route} 沒有登記進 Sidebar 的 navGroups`,
		);
	}
});

test("Sidebar 豁免的發票頁仍然有入口（不得只靠直接輸入網址）", async () => {
	// 沒有這一條，`SIDEBAR_EXEMPT_ROUTES` 就是 C13 的後門：把新頁面加進豁免清單
	// 之後它照樣「存在、路由通、沒有人找得到」，而上面那支測試會是綠的。
	const sources: string[] = [];
	for (const root of ["src/app/admin/invoices", "src/features/invoices"]) {
		const entries = await readdir(root, {
			withFileTypes: true,
			recursive: true,
		}).catch(() => []);
		for (const entry of entries) {
			if (!entry.isFile() || !/\.tsx?$/u.test(entry.name)) continue;
			const path = `${entry.parentPath ?? root}/${entry.name}`;
			// 走 readCode（去註解）：頁面自己的說明文字裡會出現自己的路由，
			// 拿原文比對的話「只有註解提到它」也會過關。
			sources.push(await readCode(path));
		}
	}
	for (const exempt of SIDEBAR_EXEMPT_ROUTES) {
		assert.ok(
			sources.some((source) => source.includes(`"${exempt}"`)),
			`${exempt} 沒有任何發票頁連得過去 ⇒ 它沒有入口`,
		);
	}
});

// ===========================================================================
// A10／A12／A13（第三波）
// ===========================================================================

import {
	auditActionGroupOf,
	auditActionOptionGroups,
	auditChangeSummary,
} from "../src/features/invoices/audit-logs";
import {
	DOCUMENT_STATUS_BADGE_VARIANT,
	FULFILLMENT_MODE_BADGE_VARIANT,
} from "../src/features/invoices/badge-variants";
import {
	checkDateRange,
	isWireDate,
} from "../src/features/invoices/date-range";
import {
	documentTimelineLabel,
	fileUnavailableText,
	shipmentStatusDisplay,
} from "../src/features/invoices/documents";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	INVOICE_OPERATION_TYPE_LABEL,
	InvoiceDocumentStatus,
	InvoiceFulfillmentMode,
} from "../src/lib/codemeta";

const WAVE3_PAGES = {
	a10: "src/app/admin/invoices/audit-logs/page.tsx",
	a10Console: "src/features/invoices/components/InvoiceAuditConsole.tsx",
	a10Dialog: "src/features/invoices/components/InvoiceAuditDetailDialog.tsx",
	a12: "src/app/admin/invoices/documents/page.tsx",
	a13: "src/app/admin/invoices/documents/[documentId]/page.tsx",
} as const;

// --- A12／A13：文件狀態顏色表 ------------------------------------------------

test("A12：文件狀態顏色表 ＝ CodeMeta 的 11 值（VOIDING 與 RESOLVED_EXTERNALLY 不得漏）", () => {
	assert.deepEqual(
		Object.keys(DOCUMENT_STATUS_BADGE_VARIANT).sort(),
		Object.values(InvoiceDocumentStatus).sort(),
	);
	// 這兩個是 58A 驗收逐字點名的：漏了會落 `?? "gray"`，而「作廢中」看起來就跟
	// 「已作廢」一樣，「賣家自行開立」看起來就跟「不由平台開立」一樣。
	assert.equal(DOCUMENT_STATUS_BADGE_VARIANT.VOIDING, "info");
	assert.equal(DOCUMENT_STATUS_BADGE_VARIANT.RESOLVED_EXTERNALLY, "success");
	// 三個值域內零命中的鬼值不得混進來（同 A07 的 PLATFORM_ASSIST_REQUESTED）。
	for (const ghost of [
		"SELF_ISSUED_REPORTED",
		"DELEGATED_TO_ISSUER",
		"CANCELLED",
	]) {
		assert.equal(DOCUMENT_STATUS_BADGE_VARIANT[ghost], undefined);
	}
	// default 分支：後端上線新值時回 undefined，由頁面的 ?? 接住，不是白畫面。
	assert.equal(
		DOCUMENT_STATUS_BADGE_VARIANT.__A_VALUE_THE_FRONTEND_DOES_NOT_KNOW__,
		undefined,
	);
});

test("A12：開票模式顏色表 ＝ CodeMeta 的 4 值，LEGACY_MANUAL 不得是 danger", () => {
	assert.deepEqual(
		Object.keys(FULFILLMENT_MODE_BADGE_VARIANT).sort(),
		Object.values(InvoiceFulfillmentMode).sort(),
	);
	// 切點之前付款的訂單本來就走人工，那是設計不是故障。畫成紅色會讓 ADM 去追
	// 一批根本沒有問題的舊訂單。
	assert.notEqual(FULFILLMENT_MODE_BADGE_VARIANT.LEGACY_MANUAL, "danger");
});

// --- A13：時間軸中文的來源 ---------------------------------------------------

test("A13：timeline 的中文查文件狀態表（契約給的是 to_status，不是 timeline event）", () => {
	// invoice-service 的 toDocumentDetail 把 StatusHistoryEntry.ToStatus 填進 event，
	// 值域是 11 個文件狀態 —— 不是 58A 寫的 InvoiceDocumentTimelineEvent。
	assert.equal(
		documentTimelineLabel("AWAITING_MANUAL"),
		INVOICE_DOCUMENT_STATUS_LABEL.AWAITING_MANUAL,
	);
	assert.equal(documentTimelineLabel("VOIDING"), "作廢中");
	// timeline event 的值域仍然接得住（兩張表都是 CodeMeta 生成物，零手抄中文）。
	assert.equal(typeof documentTimelineLabel("ALLOWANCE_APPLIED"), "string");
	// 🔴 default 分支（C11）：後端加一個雙方都沒有的值時原樣顯示碼值，不是白畫面。
	assert.equal(
		documentTimelineLabel("A_STATUS_THE_FRONTEND_DOES_NOT_KNOW"),
		"A_STATUS_THE_FRONTEND_DOES_NOT_KNOW",
	);
	assert.equal(documentTimelineLabel(""), "—");
});

test("A12：fileUnavailableReason 不寫死中文對照（那個值域沒有 SSOT）", () => {
	// CARRIER_NOT_WON / EXPIRED / NOT_APPLICABLE 只出現在 proto 註解，不是 CodeMeta enum。
	// 手寫三筆對照表 ＝ 一份沒有來源的第二碼表 ⇒ 原碼必須留在畫面上。
	assert.ok(fileUnavailableText("CARRIER_NOT_WON").includes("CARRIER_NOT_WON"));
	assert.equal(fileUnavailableText(""), "沒有可下載的檔案");
});

// --- A10：變更摘要只有兩態 ---------------------------------------------------

test("A10：變更摘要沒有 PENDING 分支，且『未寫入』與『無欄位差異』不共用一句", () => {
	// FAILED ＝ 這次操作根本沒有生效；SUCCESS 且無內容 ＝ 生效了但什麼都沒改。
	// 對正在判斷「這個管理員動了什麼」的人，兩者的差別就是全部。
	assert.equal(auditChangeSummary("FAILED", "", ""), "未寫入");
	assert.equal(auditChangeSummary("FAILED", '{"a":1}', '{"a":2}'), "未寫入");
	assert.equal(auditChangeSummary("SUCCESS", "", ""), "無欄位差異");
	assert.notEqual(
		auditChangeSummary("SUCCESS", '{"a":1}', '{"a":2}'),
		"無欄位差異",
	);
	// 本 DTO 的 result 值域只有 SUCCESS / FAILED（Gateway InvoiceAuditLogItem）⇒
	// 照抄代行稽核台的第三態會是一支永遠走不到的死碼。
	assert.doesNotMatch(auditChangeSummary("PENDING", "", ""), /結果未確認/u);
});

test("A10：全頁沒有保留天數那一列（端點 61 沒有 retention 欄位）", async () => {
	// 讀不到就寫死「5 年」＝ 把法遵文字繞過版本控管；退而寫「長期保存」更糟。
	// 三個檔案都掃：wrapper／本體／明細 dialog。
	for (const path of [
		WAVE3_PAGES.a10,
		WAVE3_PAGES.a10Console,
		WAVE3_PAGES.a10Dialog,
	]) {
		const source = await readCode(path);
		assert.doesNotMatch(source, /保留/u, `${path} 出現了保留天數相關文字`);
		assert.doesNotMatch(source, /adm-audit-retention/u);
	}
});

test("A10：操作類型下拉分組涵蓋 CodeMeta 的六個值，且沒有空組", () => {
	const groups = auditActionOptionGroups();
	const flattened = groups.flatMap(([, options]) => options.map(([v]) => v));
	assert.deepEqual(
		flattened.sort(),
		Object.keys(INVOICE_OPERATION_TYPE_LABEL).sort(),
		"分組漏掉／重複某個值 ⇒ 那個操作類型在下拉裡消失或出現兩次",
	);
	for (const [, options] of groups) assert.ok(options.length > 0);
	// 最長前綴優先：VOID_ALLOWANCE 屬於「作廢」而不是「折讓」（它作廢的是折讓單，
	// 動作是作廢）。前綴比對寫成「第一個命中就算」會把它歸錯。
	assert.equal(auditActionGroupOf("VOID_ALLOWANCE"), "void");
	assert.equal(auditActionGroupOf("ALLOWANCE"), "allowance");
	assert.equal(auditActionGroupOf("SOMETHING_NEW_FROM_BACKEND"), "other");
});

// --- 日期區間：前端是第一道 ---------------------------------------------------

test("端點 47／61 的日期是 YYYY-MM-DD，且 from > to 前端就擋（後端不會報錯）", () => {
	assert.ok(isWireDate("2026-08-30"));
	assert.equal(isWireDate("2026-08-30T14:00"), false);
	assert.equal(isWireDate("2026/08/30"), false);

	assert.equal(checkDateRange("", "").ok, true);
	assert.equal(checkDateRange("2026-08-01", "2026-08-30").ok, true);
	// 同一天是合法區間，不得誤擋。
	assert.equal(checkDateRange("2026-08-30", "2026-08-30").ok, true);
	// 🔴 from > to 後端**不會回錯**，它會老實地回一個空集合 —— 而空集合看起來就像
	// 「這段期間沒有紀錄」。這是本函式最重要的一條。
	const reversed = checkDateRange("2026-08-30", "2026-08-01");
	assert.equal(reversed.ok, false);
	assert.match(reversed.message, /不能晚於/u);
	// 格式錯是另一條（後端回 400）。
	assert.equal(checkDateRange("2026-08-30T14:00", "").ok, false);
});

test('A10／A12 的日期輸入是 type="date" 不是 datetime-local', async () => {
	// 端點 47／61 的契約是 YYYY-MM-DD。給時間選擇器卻把時間那半丟掉，是一個不會
	// 發出任何訊號的靜默失效：管理員挑了 14:00，實際查的是一整天。
	// （這一項刻意偏離 58A §A10 的「四件套」樣板，理由寫在 features/invoices/date-range.ts。）
	for (const path of [WAVE3_PAGES.a10Console, WAVE3_PAGES.a12]) {
		const source = await readCode(path);
		assert.doesNotMatch(
			source,
			/datetime-local/u,
			`${path} 用了 datetime-local`,
		);
		assert.match(source, /type="date"/u, `${path} 沒有日期輸入`);
	}
});

// --- 三頁都是唯讀 -------------------------------------------------------------

test("A10／A12 一律唯讀；A13 的寫入只有登記過的那兩條", async () => {
	// A10／A12 依 58A 本來就完全唯讀。
	//
	// 🔴 **A13 已經不是唯讀了**，而原本這支測試看不到那件事：它掃的是 `api.post`，
	// 而兩條寫入都走 `services/api.ts` 的 `request()`（`api.post` 不收自訂 header，
	// 而 `/admin/invoices` 整組掛了 `RequireIdempotencyKey`）⇒ 觀測窗在寫入實際
	// 落地的那條路徑之外。掃不到不等於沒有。
	//
	// 改成正面白名單：A13 的 `method: "POST"` 呼叫必須恰好是這兩條，多一條就轉紅。
	//   - 端點 49 `notification_resends`（重寄通知，2026-09-02 接上）
	//   - 端點 73 `manual_resolutions`（送回自動線，2026-09-05 接上）
	//
	// 端點 73 是「停在人工出口且零 operation」的文件**唯一**的入口：自動線落
	// AWAITING_MANUAL 時不建 operation，而 A06 的三個動作全部以 job_id 為鍵
	// ⇒ 那些文件在 A06 組不出請求、在 A05 工作台上根本看不到（工作台是 job-first）。
	// 它不在下一支測試禁的四個 job 層操作之內。
	for (const [name, path] of Object.entries(WAVE3_PAGES)) {
		const source = await readCode(path);
		for (const verb of ["post", "put", "patch", "delete"]) {
			assert.doesNotMatch(
				source,
				new RegExp(`api\\.${verb}\\s*[<(]`, "u"),
				`${name}（${path}）呼叫了 api.${verb}`,
			);
		}
	}
	const a10 = await readCode(WAVE3_PAGES.a10);
	const a12 = await readCode(WAVE3_PAGES.a12);
	for (const [name, source] of [
		["A10", a10],
		["A12", a12],
	] as const) {
		assert.ok(
			!/method:\s*"(POST|PUT|PATCH|DELETE)"/u.test(source),
			`${name} 出現了寫入呼叫，但它依 58A 是唯讀頁`,
		);
	}
	const a13 = await readCode(WAVE3_PAGES.a13);
	const writes = [...a13.matchAll(/method:\s*"POST"/gu)].length;
	assert.equal(
		writes,
		2,
		`A13 的 POST 呼叫應恰好 2 條（端點 49 重寄通知、端點 73 送回自動線），實得 ${writes} 條`,
	);
	assert.match(a13, /notification_resends/u, "A13 少了端點 49");
	assert.match(a13, /manual_resolutions/u, "A13 少了端點 73");
});

test("A13 不畫作廢／折讓／重開／編輯，連 disabled 的都沒有", async () => {
	const source = await readCode(WAVE3_PAGES.a13);
	for (const word of ["作廢這張", "開立折讓", "重新開立", "編輯發票"]) {
		assert.ok(!source.includes(word), `A13 出現了 ${word}`);
	}
	// 那些是 job 層的操作，入口在 A06（各有雙人與樂觀鎖保護）。
	assert.match(source, /查看工作/u, "A13 → A06 的雙向連結不得消失");
});

test("A12／A13 不自行拼接 PDF 下載路徑（陷阱 T22）", async () => {
	// 只有端點 48 的回應**應該**帶 downloadPath，而實際契約連它都沒有；
	// `invoiceFileKey` 是 objectKey，58A 明令不顯示。拼出來的是一條死鍵。
	for (const path of [WAVE3_PAGES.a12, WAVE3_PAGES.a13]) {
		const source = await readCode(path);
		assert.doesNotMatch(source, /file\/content/u, `${path} 拼接了下載路徑`);
		assert.doesNotMatch(source, /invoiceFileKey/u, `${path} 顯示了 objectKey`);
	}
});

test("A12 不做金額加總、不做輪詢", async () => {
	const source = await readCode(WAVE3_PAGES.a12);
	// 加總是對帳語意，而月結匯出已移出 Phase 1 ⇒ 給一個沒有匯出、也沒有對帳定義的
	// 總和只會被當成帳用。
	assert.doesNotMatch(source, /總計|合計/u);
	assert.doesNotMatch(source, /refetchInterval\s*:/u);
	assert.doesNotMatch(source, /setInterval\(/u);
});

test("A10 不做輪詢，也不畫匯出鍵", async () => {
	const source = await readCode(WAVE3_PAGES.a10Console);
	assert.doesNotMatch(source, /refetchInterval\s*:/u);
	assert.doesNotMatch(source, /setInterval\(/u);
	// ADM 既有的 merchants/page.tsx 就是一顆 alert("匯出功能開發中...") 的假按鈕，
	// 存在很久、每個看到的人都會按一次，而它從來沒做過任何事。不要再造第二顆。
	assert.doesNotMatch(source, /匯出|下載/u);
});

// --- 契約上不存在的欄名一律不得出現 -------------------------------------------

test("A12／A13 不得出現契約上不存在的欄名（渲染 undefined 沒有東西會轉紅）", async () => {
	const a12 = await readCode(WAVE3_PAGES.a12);
	// 端點 47 回的是 []SellerInvoiceDocumentItem，沒有 storeName。
	//
	// ⚠️ paidAt / shipmentStatus 曾經也在這份禁列裡（後端從不填 ⇒ 恆為空字串），
	// backend#3019 之後兩者都有值了，禁列因此縮回只剩 storeName。
	// 「shipmentStatus 的空字串不得被當成沒出貨」改由本檔最後一支測試守——
	// 那才是欄位有值之後真正的風險。
	for (const field of ["storeName"]) {
		assert.doesNotMatch(
			a12,
			new RegExp(`\\.${field}\\b`, "u"),
			`A12 讀了 ${field}`,
		);
	}
	const a13 = await readCode(WAVE3_PAGES.a13);
	// 詳情 DTO 沒有 fulfillmentMode；idempotencyKey 與 createdBy 從不被填；
	// downloadPath 根本不存在。
	for (const field of [
		"fulfillmentMode",
		"downloadPath",
		"idempotencyKey",
		"createdBy",
	]) {
		assert.doesNotMatch(
			a13,
			new RegExp(`\\.${field}\\b`, "u"),
			`A13 讀了 ${field}`,
		);
	}
});

test("A12／A13 顯示完整發票號碼，且不得自己補遮罩", async () => {
	// proto 逐字：「完整值，**不遮罩**。§8.1.3 (b) 把它判為 O 類」；invoice-service
	// 更註明「§8.1.3(c) 裁定 1 逐字推翻 img-05 的遮罩畫法」。58A 兩處寫的
	// `invoiceNoMasked` 在契約上不存在。前端補遮會蓋掉對獎與報帳的唯一憑據。
	for (const path of [WAVE3_PAGES.a12, WAVE3_PAGES.a13]) {
		const source = await readCode(path);
		assert.match(source, /invoiceNumber/u, `${path} 沒有顯示發票號碼`);
		assert.doesNotMatch(source, /invoiceNoMasked/u);
		assert.doesNotMatch(source, /\*\*\*\*/u, `${path} 自己加了遮罩`);
	}
});

test("A13 副標宣告存取留痕，而 A06 不得宣告（陷阱 T30）", async () => {
	const a13 = await readCode(WAVE3_PAGES.a13);
	// 端點 48 fail-closed 落一列 INVOICE_PII_REVEAL（invoice-service
	// GetInvoiceDocumentForAdmin），所以這句話是真的。
	assert.match(a13, /記錄一筆存取紀錄/u);
	// A06 的主資料是端點 51，不落 PII_REVEAL ⇒ 宣告與事實不符會把嚇阻變成假保證。
	const a06 = await readCode("src/app/admin/invoices/jobs/[jobId]/page.tsx");
	assert.doesNotMatch(a06, /記錄一筆存取紀錄/u);
});

test("A13 的 ?from=duplicate 是 info 不是 error", async () => {
	const source = await readCode(WAVE3_PAGES.a13);
	assert.match(source, /from=== "duplicate"|from"\) === "duplicate"/u);
	// 9347 的語意是冪等鍵命中、發票已經開好了。畫成錯誤會讓管理員再送一次，
	// 而那正是它要防的事。
	const banner = source.slice(source.indexOf("fromDuplicate ?"));
	assert.match(banner.slice(0, 400), /role="status"/u);
	assert.doesNotMatch(banner.slice(0, 400), /role="alert"/u);
});

// --- 表格 a11y：本波刻意補強 --------------------------------------------------

test("A10／A12 的表格有 sr-only caption，且 .sr-only 真的有宣告", async () => {
	for (const path of [WAVE3_PAGES.a10Console, WAVE3_PAGES.a12]) {
		const source = await readCode(path);
		assert.match(
			source,
			/<caption className="sr-only">/u,
			`${path} 缺 caption`,
		);
		assert.match(source, /scope="col"/u, `${path} 的 th 缺 scope`);
	}
	// 🔴 ADM 全站原本沒有 .sr-only ⇒ 沒宣告的話 caption 會**照常顯示在表格上方**，
	// 而沒有任何測試會轉紅（同 §0.6 C07 的形狀）。
	const css = await readFile("src/styles/pages/Invoice.css", "utf8");
	assert.match(css, /^\.sr-only \{/mu, "Invoice.css 沒有宣告 .sr-only");
});

/**
 * 用了住在別張樣式表的 class，就必須 import 那張表（58A §0.6 陷阱 C07）。
 *
 * 這是一個**完全靜默**的失效：`.adm-operation-boundary` 之類定義在
 * `DelegatedOperations.css`（頁面級樣式表），頁面沒有 import 它的話 class 名照樣寫得上、
 * （對話框骨架 `.adm-operation-dialog-*` 已搬進 Dialog 自己的樣式表，不再有這個問題。）
 * TS 不擋、測試全綠，而畫面上的對話框會變成「沒有遮罩、沒有置中、直接接在頁尾的一段
 * 純文字」。實際踩到：A04 詳情頁補上三支治理 dialog 時只 import 了 `Invoice.css`，
 * 27 條行為斷言全過（文字都在），是**看截圖**才發現對話框根本沒有樣式。
 *
 * 判準不寫死 class 清單：兩張表各自宣告了什麼由檔案本身決定，
 * 「用到只有 DelegatedOperations.css 有的 class」才要求 import。
 */
test("發票頁用到 DelegatedOperations.css 專有的 class 時必須 import 它", async () => {
	const classesIn = async (file: string) => {
		const css = await readFile(file, "utf8");
		return new Set([...css.matchAll(/\.(adm-[a-z0-9-]+)/gu)].map((m) => m[1]));
	};
	const delegated = await classesIn("src/styles/pages/DelegatedOperations.css");
	const invoice = await classesIn("src/styles/pages/Invoice.css");
	const exclusive = new Set([...delegated].filter((c) => !invoice.has(c)));
	assert.ok(
		exclusive.has("adm-operation-boundary"),
		"DelegatedOperations.css 不再宣告 adm-operation-boundary——樣式搬家了就要同步改這支測試",
	);

	const entries = await readdir("src/app/admin/invoices", {
		withFileTypes: true,
		recursive: true,
	});
	const pages = entries
		.filter((e) => e.isFile() && e.name === "page.tsx")
		.map((e) => `${e.parentPath}/page.tsx`);
	assert.ok(pages.length >= 5, "掃到的發票頁太少");

	for (const page of pages) {
		const dir = page.slice(0, page.lastIndexOf("/"));
		const source = await readFile(page, "utf8");
		// 頁面本身 ＋ 它同目錄下的元件（dialog 都住在這裡）。
		const bundle = [source];
		for (const m of source.matchAll(/from\s+"(\.\/[^"]+)"/gu)) {
			bundle.push(
				await readFile(`${dir}/${m[1].slice(2)}.tsx`, "utf8").catch(() => ""),
			);
		}
		const used = new Set<string>();
		for (const text of bundle) {
			for (const m of text.matchAll(/className="([^"]*)"/gu)) {
				for (const cls of m[1].split(/\s+/u))
					if (exclusive.has(cls)) used.add(cls);
			}
		}
		if (used.size === 0) continue;
		assert.ok(
			source.includes('import "@/styles/pages/DelegatedOperations.css"'),
			`${page} 用了 ${[...used].join("／")}（只有 DelegatedOperations.css 有）卻沒有 import 它` +
				"——畫面會變成零樣式，而且不會有任何東西轉紅",
		);
	}
});

test("A10 的明細 dialog 用共用 Dialog（portal 是 Dialog 唯一的行為，表格內開 modal 不會被困住）", async () => {
	const source = await readCode(WAVE3_PAGES.a10Dialog);
	// sticky 儲存格會建立 stacking context 並困住 modal（2026-08-20 的線上問題）——
	// 症狀是「按了沒反應」。
	assert.match(source, /<Dialog\b/u);
	assert.doesNotMatch(source, /<Dialog\b[^>]*\bportal\b/u);
	assert.match(
		source,
		/import \{[^}]*\bDialog\b[^}]*\} from "@\/components\/UI"/u,
	);
});

// --- 發票模組內部連結必須有目的地 --------------------------------------------

/**
 * ADM **沒有路由清冊 gate**（58A §B-3.1.1），所以「按鈕指向一個不存在的頁面」在這個
 * repo 裡是完全靜默的：不編譯錯、不測試紅，使用者按下去得到一個 404。
 *
 * 這一條是那個 gate。它掃發票模組內所有指向 `/admin/invoices/**` 的連結字面值
 * （`router.push` 與 `KPI_META.href` 都涵蓋），把 `${…}` 當成動態段，再回檔案系統
 * 確認真的有對應的 `page.tsx`。
 *
 * ⚠️ 範圍只到 `/admin/invoices/**`：模組外的連結
 * 由那些模組自己負責，跨模組驗證會把這支 gate 變成全站路由清冊，那是另一件事。
 */
/**
 * 把 `${…}` 插值折成一個不含括號的佔位符，再交給下面的連結掃描。
 *
 * 🔴 **沒有這一步，這支 gate 對「動態段裡有函式呼叫」的連結完全失明。**
 * `LINK` 的字元類把 `)` 當終止符（它要在 `router.push(...)` 的右括號停下來），
 * 於是 `` `/admin/invoices/provider-accounts/${encodeURIComponent(id)}/cutovers` ``
 * 只會被擷取到 `…/${encodeURIComponent(id` 為止 —— **路徑尾巴整段被切掉**，
 * 而切完剩下的 `/admin/invoices/provider-accounts/[x]` 剛好存在 ⇒ 一律綠燈。
 * 實測：把 `/cutovers` 打成 `/cutover` 不會轉紅。
 *
 * 折成 `${X}` 之後括號消失，尾巴保得住；對本來就沒有括號的插值（`${jobId}`）
 * 這一步是等價變換，既有斷言不受影響。
 */
function normaliseInterpolations(source: string): string {
	// 佔位符**就是要**長成插值的樣子：下面的路徑正規化以 `seg.startsWith("$" + "{")`
	// 判定動態段，換成別的形狀就對不上了。
	// biome-ignore lint/suspicious/noTemplateCurlyInString: 見上一行
	return source.replace(/\$\{[^{}]*\}/gu, "${X}");
}

test("發票模組內所有指向 /admin/invoices 的連結都有對應的頁面", async () => {
	const roots = ["src/app/admin/invoices", "src/features/invoices"];
	const sources: string[] = [];
	for (const root of roots) {
		const entries = await readdir(root, {
			withFileTypes: true,
			recursive: true,
		}).catch(() => []);
		for (const entry of entries) {
			if (!entry.isFile() || !entry.name.endsWith(".ts")) {
				if (!entry.isFile() || !entry.name.endsWith(".tsx")) continue;
			}
			// 走 readCode（去註解）而不是 readFile：說明文字裡本來就會出現路由字面值
			// （例如「這顆鍵原本指向 /admin/invoices/issuances/new」），拿原文比對會逼人
			// 為了讓測試變綠去刪掉解釋 —— 而那段解釋正是防止下一個人把死鍵加回來的東西。
			sources.push(
				normaliseInterpolations(
					await readCode(`${entry.parentPath ?? root}/${entry.name}`),
				),
			);
		}
	}
	assert.ok(sources.length > 0, "一個發票檔案都沒掃到");

	// 檔案系統上真正存在的路由（動態段保留成 `[x]` 的形狀）。
	const pageEntries = await readdir("src/app/admin/invoices", {
		withFileTypes: true,
		recursive: true,
	});
	const existing = new Set(
		pageEntries
			.filter((e) => e.isFile() && e.name === "page.tsx")
			.map((e) =>
				(e.parentPath ?? "")
					.replaceAll("\\", "/")
					.replace(/^src\/app/u, "")
					.split("/")
					.map((seg) => (seg.startsWith("[") ? "[x]" : seg))
					.join("/"),
			),
	);
	assert.ok(
		existing.size >= 5,
		`掃到的發票路由太少：${[...existing].join(", ")}`,
	);

	// 負向 lookbehind 排除後端路徑：`/api/v1/admin/invoices/overview`（端點 37）沒有底線，
	// 光靠下面那條 `_` 過濾攔不住，會被誤判成一條不存在的前端路由。
	const LINK = /(?<!\/api\/v1)\/admin\/invoices(?:\/[^"'`\s?)]*)?/gu;
	const missing = new Set<string>();
	for (const source of sources) {
		for (const raw of source.match(LINK) ?? []) {
			// 前端路由 kebab、後端 API snake（58A §B-3.0 刻意兩套）⇒ 帶底線的一律是
			// API 路徑（`/api/v1/admin/invoices/audit_logs`）或註解裡的端點說明
			// （`{job_id}` 這種 placeholder），不是這支 gate 的對象。
			// 🔴 `{` 的判斷要區分兩種：`${…}` 是 JS 插值（**動態路由段，必須驗**），
			// 而裸 `{job_id}` 是註解裡的 API placeholder（不驗）。
			// 一開始寫成 `raw.includes("{")` ⇒ **所有帶動態段的連結整批被跳過**，
			// gate 只剩對靜態字串有效；變異測試（把 documents 打成 document）沒轉紅才發現。
			if (raw.includes("_") || /(?<!\$)\{/u.test(raw)) continue;
			const normalised = raw
				.split("?")[0]
				.replace(/\/$/u, "")
				.split("/")
				// 整段是插值 ＝ 動態路由段（`${jobId}` → `[jobId]`）；
				// 只是**接在段尾**的插值是 query 串接（`tax-reviews${query}`），砍掉即可。
				.map((seg) =>
					seg.startsWith("${") ? "[x]" : seg.replace(/\$\{[^}]*\}$/u, ""),
				)
				.join("/");
			if (!existing.has(normalised)) missing.add(raw);
		}
	}
	assert.deepEqual(
		[...missing].sort(),
		[],
		"這些連結沒有對應的 page.tsx ⇒ 按下去是 404，而 ADM 沒有任何 gate 會告訴你",
	);
});

// --- #3019：出貨狀態欄的降級語意 -----------------------------------------------

// 🔴 這一欄的風險不在「有沒有畫」，在**空字串代表什麼**。
//
// shipping-service 抖動時後端整批回空字串並把 shipmentStatusDegraded 設成 true；
// 只看 code 的實作會讓每一列都畫成「還沒出貨」——一句我們無從得知的話。那正是這一欄
// 在 backend#3019 之前不敢畫的理由，修完之後它會換一個形式重演。
//
// ⚠️ 這一支刻意驗**行為**不驗字串。第一版是對 page.tsx 做 source grep
// （assert 檔案裡有 "shipmentStatusDegraded"、有 "暫時無法取得"），實測把
// `if (degraded)` 整段刪掉之後**四條斷言全部照樣通過**——因為那些字串還活在
// queryFn 與表格上方的提示裡。零鑑別力的斷言比沒有斷言更糟，它會讓人以為驗過了。
test("A12：降級與「還沒出貨」必須是兩種顯示，不得塌成同一格", () => {
	assert.deepEqual(shipmentStatusDisplay("", true), { kind: "degraded" });
	assert.deepEqual(shipmentStatusDisplay("", false), { kind: "none" });
	// 有值時走 CodeMeta 查表（前端一個中文字都不手寫）。
	assert.deepEqual(shipmentStatusDisplay("210", false), {
		kind: "status",
		label: "已叫件",
	});
	// 判定順序：degraded 必須先判。降級時就算後端漏回了某一列的碼值，
	// 也不該拿它去說「這一列出貨了」——整批都是不可信的。
	assert.deepEqual(shipmentStatusDisplay("210", true), { kind: "degraded" });
});

// 58A §A12：出貨狀態「獨立一欄、獨立 badge，禁止與發票狀態合成一個綜合狀態」。
test("A12：出貨狀態是獨立一欄，且判定走 shipmentStatusDisplay", async () => {
	const a12 = await readCode(WAVE3_PAGES.a12);
	assert.match(
		a12,
		/<th scope="col">出貨狀態<\/th>/u,
		"出貨狀態必須是獨立一欄",
	);
	assert.match(
		a12,
		/shipmentStatusDisplay\(/u,
		"判定必須走有測試守著的純函式，不得在 .tsx 內自己寫 if",
	);
});
