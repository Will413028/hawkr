import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import * as codemeta from "../src/lib/codemeta";
import {
	ProductStatus,
	SHIPMENT_ANOMALY_SEVERITY_LABEL,
	SHIPMENT_ANOMALY_SEVERITY_ORDER,
	toOptions,
} from "../src/lib/codemeta";
import { getStatusMeta, UNKNOWN_STATUS_META } from "../src/lib/product-status";

/**
 * 「第二份碼表」偵測器。
 *
 * ## 這支在防什麼
 *
 * `src/lib/codemeta/generated.ts` 是 backend CodeMeta 的生成物，但**光是有一份正確的
 * 碼表，擋不住有人在別處再寫一份**。ADM 過去就是這樣壞掉的，而且壞了兩輪：
 *
 * 1. 各頁自定義 enum（transactions 頁假想 status 0-6 / payType 1-4，全數錯位）
 * 2. 收斂成單一 `src/lib/codemeta.ts` 之後，那份**本身仍是手抄的**，於是又漂了 5 處
 *
 * 收斂成生成物解決了第 2 輪。這支測試防的是第 1 輪 —— 有人（或某個 AI）圖方便，
 * 在某個頁面裡直接寫 `{ "0": "待付款", "10": "已付款" }`。
 *
 * ## 判準為什麼是「數字碼鍵 + 中文值」
 *
 * 這是手寫 value→label 表**唯一穩定的簽名**。試過的其他判準都不行：
 *
 * - 「掃中文標籤字面值」→ 大量誤報。「買家」「賣家」「物流」「異常訂單」這些既是
 *   CodeMeta 標籤、也是再正常不過的頁面標題與導覽文字，掃出來 9 個命中裡 7 個是誤報。
 *   會誤報的 gate 最後一定會被加一長串 allowlist，然後沒人看得懂它在守什麼。
 * - 「掃 `Record<string, string>`」→ 抓不到，因為顏色表、icon 表也是這個型別，
 *   而那些是正當的呈現層對照（後端不下發顏色）。
 *
 * 「數字碼當鍵、中文當值」則幾乎不可能是別的東西。
 *
 * ## 它擋不住什麼（誠實標註）
 *
 * - 字串碼值的手抄表（`{ HIGH: "高" }`）—— 鍵不是數字，本規則看不到。那類目前靠
 *   review。
 * - 把中文寫在 JSX 而非物件字面值裡（`{status === "0" ? "待付款" : …}`）。
 *
 * 抓不到不代表可以寫。規則是「碼值→顯示文字的對照一律走 codeLabel(生成物)」，
 * 這支只是其中一種形態的機械保證。
 */

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const SRC = join(projectRoot, "src");

/** 生成物自己當然滿是這個形狀，要排除。 */
const EXCLUDED_DIRS = [join(SRC, "lib", "codemeta")];

/** `"0": "待付款"` / `"3001": "黑貓宅急便"` —— 數字碼鍵配含中文的值。 */
const SECOND_COPY = /"(\d{1,4})"\s*:\s*"([^"\\\n]*[一-鿿][^"\\\n]*)"/g;

function collectSourceFiles(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const full = join(dir, entry.name);
		if (EXCLUDED_DIRS.some((d) => full.startsWith(d))) continue;
		if (entry.isDirectory()) {
			out.push(...collectSourceFiles(full));
		} else if (/\.tsx?$/.test(entry.name)) {
			out.push(full);
		}
	}
	return out;
}

test("src/ 沒有任何手寫的「數字碼 → 中文標籤」對照表", () => {
	const offenders: string[] = [];
	for (const file of collectSourceFiles(SRC)) {
		const source = readFileSync(file, "utf8");
		for (const m of source.matchAll(SECOND_COPY)) {
			const line = source.slice(0, m.index).split("\n").length;
			offenders.push(
				`${relative(projectRoot, file)}:${line}  "${m[1]}": "${m[2]}"`,
			);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		[
			"發現手寫的碼值→標籤對照：",
			...offenders,
			"",
			"標籤一律從 @/lib/codemeta 的生成物查：codeLabel(XXX_LABEL, code)。",
			"若該碼表還沒生給 ADM，是把 enum 名加進 backend `generate.sh` 的 ADMIN_ENUMS",
			"再重新生成 + 覆蓋，不是在這裡手寫一份。",
			"（顏色 / icon 這類後端不下發的呈現層對照不受此限——它們的值不是中文標籤。）",
		].join("\n"),
	);
});

test("偵測式本身有鑑別力（能抓到一個刻意造出來的第二份碼表）", () => {
	// 沒有這一條，SECOND_COPY 正則哪天被改壞成永不命中，上面那支會靜默恆綠。
	const planted = 'const X = {\n\t"0": "待付款",\n\t"10": "已付款",\n};';
	const hits = [...planted.matchAll(SECOND_COPY)];
	assert.equal(hits.length, 2, "偵測式抓不到已知的違規樣本");
	assert.equal(hits[0][2], "待付款");

	// 反向：顏色 / icon 表不該被誤判。
	const legit =
		'const S = {\n\t"0": { variant: "pending" },\n\t"10": "approved",\n};';
	assert.deepEqual([...legit.matchAll(SECOND_COPY)], [], "誤報了呈現層對照表");
});

test("toOptions 的顯示順序保證成立（嚴重度下拉是「高／中／低」）", () => {
	// `toOptions(map, order)` 對 order 內找不到的 key 是**靜默忽略**。若哪天 SSOT 把
	// ShipmentAnomalySeverity 的 Value 換成別的形式，SHIPMENT_ANOMALY_SEVERITY_ORDER
	// 會整組落空、下拉靜默翻轉成 SSOT 列序（低／中／高），而且沒有任何東西會報錯。
	// digest 測試只會告訴你「值變了」，不會告訴你「下拉順序反了」。
	assert.deepEqual(
		toOptions(
			SHIPMENT_ANOMALY_SEVERITY_LABEL,
			SHIPMENT_ANOMALY_SEVERITY_ORDER,
		).map(([, label]) => label),
		["高", "中", "低"],
	);
	// order 必須真的全部命中 map，沒有一個落到 rest（落空是上面那個靜默失效的前兆）。
	const missing = SHIPMENT_ANOMALY_SEVERITY_ORDER.filter(
		(k) => !(k in SHIPMENT_ANOMALY_SEVERITY_LABEL),
	);
	assert.deepEqual(missing, [], `order 有 key 不在碼表裡: ${missing.join()}`);
});

test("toOptions 不會漏掉 order 沒列到的值域（新增碼值不會從下拉靜默消失）", () => {
	const map = { A: "甲", B: "乙", C: "丙" };
	assert.deepEqual(toOptions(map, ["C", "A"]), [
		["C", "丙"],
		["A", "甲"],
		["B", "乙"],
	]);
});

// ── 第二份碼表的另一種形態：值不是中文，所以上面那條看不到它 ─────────────────────

/**
 * 「殘缺的碼表」偵測器。
 *
 * ## 這條在補哪個洞
 *
 * 上面的 `SECOND_COPY` 要求「冒號右邊直接是中文字串」。`src/lib/product-status.ts`
 * 曾經長這樣：
 *
 * ```
 * "4001": { label: "草稿",   className: "pending",  icon: "…" },
 * "4003": { label: "已下架", className: "disabled", icon: "…" },
 * "4004": { label: "已上架", className: "approved", icon: "…" },
 * ```
 *
 * 值是**巢狀物件**，於是 `SECOND_COPY` 結構性失明——中文躲在 `label:` 後面，不在
 * 數字鍵的正右邊。它就這樣帶著一個缺口活了下來：`ProductStatus` 有四個值，這張表只有
 * 三個，審核中（4002）的商品在三個頁面顯示「未知狀態(4002)」並套上退回用的紅色徽章。
 *
 * ## 判準：碼值當鍵，就得把那個值域收齊
 *
 * 對每個檔案 × 每個生成 enum，算出「該檔用了這個 enum 的哪幾個值當物件鍵」。
 * 命中 ≥3 個就視為「這個檔案在對這個值域做逐值對照」，那它就必須**收齊**。
 * 例外：只要該檔的數字鍵集合把**某一個** enum 完整覆蓋，就當它已經有主人（多個 enum
 * 值域重疊時避免誤報，例如 `{0,10,20,30,…,80}` 同時是 OrderStatus 的全集與
 * AfterSaleAppealState 的超集）。
 *
 * 為什麼不用「不准把碼值當鍵」這種更強的規則：呈現層對照（顏色 / icon，後端不下發）
 * 本來就得以碼值為鍵，那是正當用法。會出事的從來不是「有這張表」，是「這張表缺一個值」。
 *
 * ## 它擋不住什麼（誠實標註）
 *
 * - 少於 3 個命中的表（例如只列兩種狀態的 tab 清單）——那種通常是刻意的子集。
 * - 鍵不是數字的碼表（`{ HIGH: "高" }`）。
 * - 把碼值寫在 `switch` / 三元運算而不是物件鍵裡。
 *
 * `product-status.ts` 本身現在已經改用 `[ProductStatus.DRAFT]` 這種計算鍵，本規則
 * 看不到它（原始碼裡沒有數字字面值）——它的完整性改由下面那支行為測試守。
 */

/** 生成的 enum：PascalCase 命名（`PRODUCT_STATUS_LABEL` 這種標籤表有底線，排除）。 */
const GENERATED_ENUMS: Array<{ name: string; values: Set<string> }> =
	Object.entries(codemeta)
		.filter(
			([name, value]) =>
				/^[A-Z][A-Za-z]+$/.test(name) &&
				value !== null &&
				typeof value === "object",
		)
		.map(([name, value]) => ({
			name,
			values: new Set(Object.values(value as Record<string, string>)),
		}));

/** `"4001":` —— 數字碼當物件鍵。不管值是什麼型別。 */
const NUMERIC_KEY = /"(\d{1,5})"\s*:/g;

interface PartialTable {
	enumName: string;
	present: string[];
	missing: string[];
}

/** 回傳「命中 ≥3 個值但沒收齊」的 enum；已有某個 enum 被完整覆蓋時回空陣列。 */
function findPartialTables(source: string): PartialTable[] {
	const keys = new Set([...source.matchAll(NUMERIC_KEY)].map((m) => m[1]));
	if (keys.size === 0) return [];

	const scored = GENERATED_ENUMS.map((e) => ({
		enumName: e.name,
		present: [...e.values].filter((v) => keys.has(v)),
		missing: [...e.values].filter((v) => !keys.has(v)),
	}));
	// 有 enum 被完整覆蓋 ⇒ 這些鍵已經有主人，其他 enum 的部分命中是值域重疊造成的。
	if (scored.some((s) => s.present.length > 0 && s.missing.length === 0)) {
		return [];
	}
	return scored.filter((s) => s.present.length >= 3);
}

test("src/ 沒有任何「以碼值為鍵但漏掉值域成員」的對照表", () => {
	const offenders: string[] = [];
	for (const file of collectSourceFiles(SRC)) {
		const source = readFileSync(file, "utf8");
		for (const hit of findPartialTables(source)) {
			offenders.push(
				`${relative(projectRoot, file)}  ${hit.enumName} 少了 ${hit.missing.join(", ")}（已列 ${hit.present.join(", ")}）`,
			);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		[
			"以下檔案用碼值當鍵、卻沒有把值域收齊：",
			...offenders,
			"",
			"漏掉的那個值會靜靜落進 fallback —— 不會編譯錯、不會測試紅，只在那種資料",
			"真的出現時才在畫面上現形（`product-status.ts` 漏 4002 就是這樣過了很久）。",
			"把缺的值補上；若某個值在這一頁確實不可能出現，也要明確列出來並寫清楚它的呈現，",
			"不要留給 fallback 決定。",
		].join("\n"),
	);
});

test("殘缺碼表偵測式本身有鑑別力", () => {
	// 沒有這一條，findPartialTables 哪天被改壞成永不命中，上面那支會靜默恆綠。
	const planted = [
		"const M = {",
		'\t"4001": { className: "pending" },',
		'\t"4003": { className: "disabled" },',
		'\t"4004": { className: "approved" },',
		"};",
	].join("\n");
	const hits = findPartialTables(planted);
	assert.ok(
		hits.some(
			(h) => h.enumName === "ProductStatus" && h.missing.includes("4002"),
		),
		`抓不到已知的殘缺樣本，實際命中：${JSON.stringify(hits)}`,
	);

	// 正例一：收齊了就不該被判違規。
	assert.deepEqual(
		findPartialTables(
			planted.replace('\t"4003"', '\t"4002": { className: "info" },\n\t"4003"'),
		),
		[],
		"完整的呈現層對照被誤判成違規",
	);
	// 正例二：不足 3 個命中的子集（刻意只列兩種狀態的 tab）不該被判違規。
	assert.deepEqual(
		findPartialTables('const T = {\n\t"4001": "a",\n\t"4004": "b",\n};'),
		[],
		"兩個鍵的子集被誤判成違規",
	);
});

// ── 商品狀態的呈現 token：值域要收齊，而且彼此要看得出差別 ──────────────────────

test("每個 ProductStatus 值都有自己的呈現 token，且四種狀態彼此可分辨", () => {
	const codes = Object.values(ProductStatus);

	// ① 收齊：沒有任何一個值落進 fallback。漏一個的後果不是「少了顏色」，是畫面上
	//    出現一個看起來像「被退回」的紅色徽章 —— 那正是 4002 出過的事。
	const fellBack = codes.filter(
		(code) => getStatusMeta(code) === UNKNOWN_STATUS_META,
	);
	assert.deepEqual(
		fellBack,
		[],
		`這些 ProductStatus 值沒有呈現 token，會用未知碼的紅底問號渲染：${fellBack.join(", ")}`,
	);

	// ② 可分辨：className / badgeVariant 兩兩相異。光是「有補」不夠 —— 把 4002 也給
	//    `pending`，草稿與審核中在同一張表裡就是同一顆徽章，等於沒補。
	for (const field of ["className", "badgeVariant"] as const) {
		const used = codes.map((code) => getStatusMeta(code)[field]);
		assert.equal(
			new Set(used).size,
			codes.length,
			`${field} 有重複（${used.join(", ")}）—— 兩個狀態長得一模一樣，讀者分不出來`,
		);
	}
});

test("每個狀態的呈現 token 都對得到真的樣式（不是一個對不到 CSS 的字串）", () => {
	// className / badgeVariant 都是字串，打錯字不會編譯錯：`.status-badge` 只剩形狀沒有
	// 顏色、`<StatusBadge>` 靜靜退回預設 variant。兩者都不會有任何錯誤訊息。
	const globals = readFileSync(join(SRC, "app", "globals.css"), "utf8");
	const badgeCss = readFileSync(
		join(SRC, "components", "UI", "StatusBadge.css"),
		"utf8",
	);

	const metas = [
		...Object.values(ProductStatus).map((code) => getStatusMeta(code)),
		UNKNOWN_STATUS_META,
	];
	const missing: string[] = [];
	for (const meta of metas) {
		if (!globals.includes(`.status-badge.${meta.className} {`)) {
			missing.push(`globals.css 沒有 .status-badge.${meta.className}`);
		}
		if (!badgeCss.includes(`.hk-badge--${meta.badgeVariant} {`)) {
			missing.push(`StatusBadge.css 沒有 .hk-badge--${meta.badgeVariant}`);
		}
		const glyph = renderToStaticMarkup(
			createElement(MaterialSymbol, { name: meta.icon }),
		);
		if (!/<path d="[^"]+"/.test(glyph)) {
			missing.push(
				`icon 不是 @hawkr/icons 有圖形的 Material Symbol：${meta.icon}`,
			);
		}
	}
	assert.deepEqual(missing, [], missing.join("\n"));
});

// ── 以下與 CodeMeta 無關，是同一類「機械保證」：守住 modal 必須 portal ──────────

test("對話框一律 portal 到 body：共用 Dialog 沒有 portal 開關，也沒有人傳", async () => {
	// 2026-08-20：/admin/merchants 列表最後一欄是 `position: sticky; z-index: 1`
	// （`7d2a6e5` 為修 1440px 下按鈕被裁掉而加），而 MerchantOperationEntry 就掛在那格裡。
	// sticky + z-index 會建立 stacking context ⇒ 對話框的 `position: fixed; z-index: 1000`
	// 被壓到那一層 ⇒ 其他列的 sticky 儲存格蓋在遮罩上、對話框被表格的 overflow 裁掉，
	// 底部「建立工作階段」按鈕捲不到，整條代行流程無法完成。
	//
	// 調高 z-index 治不了（問題是被困在錯的 stacking context），唯一治本是 portal 出去。
	// 原本靠「每個代行對話框都要記得傳 portal」守，那是容易漏的約定；現在 portal 是 Dialog 唯一的行為，
	// 這支測試守兩件事：Dialog 真的 createPortal 到 body，而且沒有人再傳 `portal`（那個 prop 已不存在）。
	const dialog = readFileSync(
		join(SRC, "components", "UI", "Dialog.tsx"),
		"utf8",
	);
	assert.match(dialog, /createPortal\(scoped, document\.body\)/u);
	assert.doesNotMatch(dialog, /\bportal\??:/u, "Dialog 不該有 portal prop");
	const offenders: string[] = [];
	const walk = (dir: string) => {
		for (const entry of readdirSync(dir, { withFileTypes: true })) {
			const full = join(dir, entry.name);
			if (entry.isDirectory()) walk(full);
			else if (
				entry.name.endsWith(".tsx") &&
				/<Dialog\b[^>]*\bportal\b/u.test(readFileSync(full, "utf8"))
			)
				offenders.push(full);
		}
	};
	walk(SRC);
	assert.deepEqual(offenders, [], "這些檔案還在傳 portal，prop 已移除");
});

test("偵測式本身抓得到「還在傳 portal」", () => {
	// 同上，防止判定條件哪天被改壞成永不命中。
	const detect = (source: string) => /<Dialog\b[^>]*\bportal\b/u.test(source);
	assert.ok(
		detect('<Dialog variant="operation" portal title="x">'),
		"正例應被判為違規",
	);
	assert.ok(
		!detect('<Dialog variant="operation" title="x">'),
		"反例應被判為合格",
	);
});

/**
 * 「查看前後差異」開出來的稽核內容：必須是 modal，而且「變更內容」在最上面。
 *
 * ## 為什麼不是掃原始碼字串
 *
 * 這裡原本兩支是 `assert.match(源碼, /scrollIntoView/)` 與 `/\.focus\(/` —— 守的是
 * 側欄版的補救措施（面板換行到表格下方時把它捲進視野）。那種形態沒有鑑別力：
 * 字串出現在註解裡會過、呼叫在錯的元素上會過、`useEffect` 根本沒觸發也會過。
 * 側欄拆掉之後它們連語意都沒了。
 *
 * 改成把元件真的渲染出來驗 markup。這是本 repo 現行環境的上限：`node --test` ＋ tsx，
 * 沒有 jsdom，`document.activeElement` 這種行為斷言寫不出來（`Dialog` 在無
 * `document` 時就地渲染，正是為了讓這條路走得通）。
 */
const requireFromTest = createRequire(import.meta.url);
requireFromTest.extensions[".css"] = () => {};

const { createElement } = requireFromTest("react") as {
	createElement: (type: unknown, props?: unknown) => unknown;
};
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const AuditDetailDialog = requireFromTest(
	"../src/features/delegated-operations/components/AuditDetailDialog",
).default as unknown;

/** 欄位齊全的一筆稽核紀錄；`changedFields` 非空才驗得到三欄變更表。 */
const AUDIT_ENTRY = {
	auditLogId: "7495464541774413824",
	createdAt: "2026-08-18T06:12:00Z",
	confirmedAt: "2026-08-18T06:12:03Z",
	actor: { adminId: "1001", displayName: "王小明" },
	target: {
		storeId: "2685914682",
		storeName: "綠野有機農場",
		sellerAccountId: "2685914682",
	},
	action: "PRODUCT_UPDATE",
	actionLabel: "編輯商品",
	riskLevel: "normal",
	capability: "PRODUCT_EDIT",
	resourceType: "product",
	resourceId: "7480000000000000000",
	resourceName: "有機小松菜 200g",
	resourceImageKey: "",
	operationReason: "賣家來電要求更正產地",
	caseReference: "CS-20260818-001",
	result: "SUCCEEDED",
	bizCode: 0,
	bizCodeLabel: "",
	resourceVersionBefore: "18",
	resourceVersionAfter: "19",
	changedFields: [
		{ field: "certifications", label: "認證標章", before: "", after: "3701" },
	],
	correlationId: "corr-abc",
	requestId: "req-abc",
	idempotencyKey: "idem-abc",
	origin: "adm",
	userAgent: "Mozilla/5.0",
	notifiedAt: "2026-08-18T06:12:05Z",
};

const renderAuditDialog = (patch: Record<string, unknown> = {}): string =>
	renderToStaticMarkup(
		createElement(AuditDetailDialog, {
			entry: { ...AUDIT_ENTRY, ...patch },
			auditRetentionDays: 730,
			onClose: () => {},
		}),
	);

test("稽核內容是 modal，不是會被視窗寬度推到畫面外的側欄", () => {
	// 側欄版在 `.adm-audit-layout`（flex-wrap）裡是固定 340px 的一欄，視窗不夠寬就
	// 換行到表格下方 ⇒ 按「查看前後差異」的人看到的是「按了沒反應」（2026-08-21 回報）。
	// modal 在任何寬度都在畫面正中，不存在「打開在畫面外」這個狀態。
	const markup = renderAuditDialog();
	assert.match(markup, /role="dialog"/u, "稽核內容沒有 dialog 語意");
	assert.match(
		markup,
		/aria-modal="true"/u,
		"少了 aria-modal，輔助科技仍會把背景整頁讀出來",
	);
	assert.match(
		markup,
		/class="adm-operation-dialog-overlay"/u,
		"沒有遮罩層 ⇒ 背景隔離（useModalIsolation）無處可掛",
	);
});

test("「變更內容（系統紀錄）」是稽核內容的第一個區塊", () => {
	// 按下那顆鈕的人要看的就是它。它原本排在 15 個區塊的第 12 個，開啟後還要捲過
	// 整頁定義列表；那正是「查看前後差異」這個承諾沒有兌現的地方。
	const markup = renderAuditDialog();
	const body = '<div class="adm-operation-dialog-body">';
	const bodyAt = markup.indexOf(body);
	assert.notEqual(
		bodyAt,
		-1,
		"找不到對話框 body —— 版面已改，這支要跟著改不是刪掉",
	);

	// 第一個子元素就是變更內容區塊（相鄰，中間不得插入任何東西）。
	assert.ok(
		markup
			.slice(bodyAt + body.length)
			.startsWith('<div class="adm-audit-changed">'),
		`body 的第一個子區塊不是「變更內容」，而是 ${markup.slice(bodyAt + body.length, bodyAt + body.length + 60)}`,
	);
	// 再從另一個方向釘一次：它必須早於所有定義列（`DetailRow`）。
	const changedAt = markup.indexOf("變更內容（系統紀錄）");
	const firstRowAt = markup.indexOf('class="adm-audit-detail-row"');
	assert.notEqual(changedAt, -1);
	assert.notEqual(firstRowAt, -1);
	assert.ok(
		changedAt < firstRowAt,
		`「變更內容」出現在第一個定義列之後（${changedAt} > ${firstRowAt}）—— 它被推回欄位堆裡了`,
	);
	// 內容真的渲染出來了，不是驗到一個空殼。
	assert.match(markup, /認證標章/u, "三欄變更表沒有渲染出後端下發的 label");
});

test("失敗列印出後端下發的業務碼說明；拿不到說明才退回只顯示原碼", () => {
	// 「業務碼 9266」對操作者與客服等於沒有資訊，而說明只有後端查得到（稽核列存的是
	// 碼，不是當時的訊息）。前端補一張自己的對照表就是第二份 CodeMeta 鏡像 —— 這正是
	// 本檔整支測試在防的東西。
	const withLabel = renderAuditDialog({
		result: "FAILED",
		bizCode: 9266,
		bizCodeLabel: "資料已被更新，請重新載入後再試",
		changedFields: [],
	});
	assert.match(
		withLabel,
		/資料已被更新，請重新載入後再試/u,
		"失敗列沒有印出 bizCodeLabel —— 面板又退回只有裸業務碼",
	);
	// 原碼仍要留著：說明給人看，碼給客服／工單追。
	assert.match(withLabel, /業務碼 9266/u, "說明蓋掉了原碼");

	// 反向：後端拿不出說明（未登錄的碼、或 gateway 尚未 promote）時只顯示原碼。
	// 少了這一筆，「前端自己補一句『操作失敗』」也會讓上面那段綠。
	const withoutLabel = renderAuditDialog({
		result: "FAILED",
		bizCode: 9266,
		bizCodeLabel: "",
		changedFields: [],
	});
	assert.match(withoutLabel, /業務碼 9266/u, "連原碼都不見了");
	assert.doesNotMatch(
		withoutLabel,
		/未知錯誤|操作失敗/u,
		"前端自己編了一句說明 —— 那會蓋掉唯一追得到的線索",
	);
});

test("稽核內容對話框走共用 Dialog（a11y 三件套在 Dialog 內），且只在選中某一列之後才掛載", () => {
	const dialog = readFileSync(
		join(SRC, "features/delegated-operations/components/AuditDetailDialog.tsx"),
		"utf8",
	);
	assert.match(dialog, /import \{ Dialog \} from "@\/components\/UI"/u);
	assert.match(dialog, /<Dialog\b/u);

	// `Dialog` 的 portal 在無 `document` 時就地渲染，前提是「對話框都是互動後才掛載」
	// （見 Dialog.tsx）。首屏就渲染它＝server 就地、client portal ⇒
	// hydration mismatch，而 production build 下那個警告是靜默的。
	const console_ = readFileSync(
		join(
			SRC,
			"features/delegated-operations/components/SessionAuditConsole.tsx",
		),
		"utf8",
	);
	assert.match(
		console_,
		/\{selected && \(\s*<AuditDetailDialog/u,
		"稽核內容對話框不是條件渲染 —— 它會在首屏就掛載，踩破 Dialog portal 的 SSR 前提",
	);
});

// ── 第三種形態：碼值是字串，所以上面兩條都看不到它 ────────────────────────────────

/**
 * 「字串碼值的殘缺對照表」偵測器。
 *
 * ## 這條在補哪個洞
 *
 * 上面兩條都以**數字鍵**為錨（`"4001":`）。ADM 的檢舉頁把碼表手抄了七份，而那些碼值是
 * `PENDING` / `RESOLVED_HIDDEN` 這種字串 —— 前兩條規則的檔頭自己就寫著抓不到
 * （「字串碼值的手抄表（`{ HIGH: "高" }`）—— 鍵不是數字，本規則看不到」）。它就這樣帶著
 * 一個缺口活了下來：`ReportStatus` 有六個值，頁面那張 `STATUS_LABELS` 只有五個，漏掉
 * `APPEALED`（申訴重開）。後果不只是列表顯示原始碼字串 —— 狀態篩選下拉是照那張表
 * 手寫的，於是**申訴中的檢舉根本篩不出來**。
 *
 * ## 判準：以物件字面值為單位，不是以檔案為單位
 *
 * 沿用上面 `findPartialTables` 的邏輯（命中 ≥3 個值 ⇒ 這是在做逐值對照 ⇒ 必須收齊），
 * 但**範圍縮到單一個 `{ … }` 區塊**。數字鍵那條可以用整個檔案當範圍，因為它靠「某個 enum
 * 被完整覆蓋」來消除值域重疊的誤報；字串鍵不行 —— 同一個檔案裡常常同時有「收齊的表」與
 * 「殘缺的表」（檢舉頁就是：`ACTION_DESCRIPTIONS` 四個裁決動作收得好好的，`STATUS_LABELS`
 * 卻少一個），用檔案當範圍會讓前者替後者擋掉告警。
 *
 * 逐區塊還有第二個好處：`ACTION_DESCRIPTIONS` 這種**正當的**碼值鍵表（操作說明，後端
 * 不下發，不是碼→標籤）在自己的區塊裡是完整的，於是自動合格，不需要 allowlist。
 *
 * ## 它擋不住什麼（誠實標註）
 *
 * - 計算鍵（`[ReportStatus.PENDING]: "warning"`）—— 原始碼裡沒有字面值，看不到。那正是
 *   修好之後該長的樣子，完整性改由型別與 review 守（同 `product-status.ts` 的處境）。
 * - 收齊的手抄表（六個值一個不漏地重打一次）。那種不會有「某個值顯示成原碼」的症狀，
 *   代價是漂移；靠 review。
 * - 🔴 **只有 3 個成員的值域：這道 gate 對它不存在任何「會被抓到」的狀態**——是結構性的，
 *   不是漏網。`incompleteDomains` 的判定是 `present.length >= 3 && missing.length > 0`，
 *   而這兩個條件對 3 成員值域互斥：手抄完整表 ⇒ `missing = 0` 不報；漏掉一個 ⇒
 *   `present = 2 < 3` 也不報。同一個 `>= 3` 門檻由三支共用（數字鍵 `findPartialTables`、
 *   字串鍵 `findPartialStringTables`、`<option>` 下拉），所以三支對它同時失明。
 *
 *   為什麼不把門檻降到 2：那三個標籤（系統／賣家／管理員）都是通用詞，降門檻會讓大量
 *   無關頁面誤報——實測 `Sidebar.tsx` 的導覽區塊標題 `section: "系統"` 與
 *   `disputes/[id]/page.tsx` 的 `partyLabel="賣家"` 都會中。而會誤報的 gate 最後一定會被
 *   加上一長串 allowlist，然後沒人看得懂它在守什麼（與本檔開頭否決「掃中文標籤字面值」
 *   是同一個理由）。這是本規則的固有取捨，不是待修的 bug。
 * - 少於 3 個命中的子集（例如只列兩種狀態的 tab 清單）—— 那種通常是刻意的子集。
 */

/** `PENDING:` —— UPPER_SNAKE 識別字當物件鍵。`case "PENDING":` 這種帶引號的不算。 */
const IDENT_KEY = /(?:^|[{,;\s])([A-Z][A-Z0-9_]{2,})\s*:/gm;

/** 把原始碼切成一個個 `{ … }` 區塊（含巢狀，內外都會各自出現一次）。 */
function braceBlocks(source: string): { body: string; open: number }[] {
	const out: { body: string; open: number }[] = [];
	const opens: number[] = [];
	for (let i = 0; i < source.length; i++) {
		if (source[i] === "{") {
			opens.push(i);
		} else if (source[i] === "}" && opens.length > 0) {
			const open = opens.pop() as number;
			out.push({ body: source.slice(open + 1, i), open });
		}
	}
	return out;
}

/**
 * 這個 `{` 是不是一張「型別已保證窮盡」的表的開頭？
 *
 * 判準是往回看該宣告有沒有 `Record<T, …>` 而 `T` 是具名型別（不是 `string`）。
 *
 * 為什麼跳過它零損失——這是結構性的，不是信任：`Record<T, …>` 帶具名 `T` 時
 * TypeScript **不允許**漏成員，漏了編不過。所以「命中 ≥3 個值卻少了幾個」這個
 * 形狀在這種區塊上定義上不可能是真的違規，只可能是鍵名與別的值域撞名造成的誤報。
 *
 * ⛔ `Record<string, …>` 不在豁免內：那是任何物件都能戴的帽子，放行等於開後門，
 * 手抄的碼→標籤表只要加個 `: Record<string, string>` 就能規避本 gate。
 *
 * 2026-09-13：backend #2538 把 17 個 Procurement enum 接進 ADMIN_ENUMS 之後，
 * `DELEGATED_AUDIT_RESULT_LABELS`（代行營運稽核結果，三個鍵 SUCCEEDED/FAILED/PENDING）
 * 同時被四個採購 enum 判成「漏了成員」。那張表與採購線毫無關係，只是鍵名撞名——
 * 而 SUCCEEDED/FAILED/PENDING 是最通用的三個狀態字，不修就會一再重演。
 */
function isExhaustivelyTypedRecord(source: string, open: number): boolean {
	const before = source.slice(Math.max(0, open - 200), open);
	return /Record<\s*([A-Za-z_$][\w$]*)\s*,[^<>]*>\s*=\s*$/.test(before)
		? !/Record<\s*string\s*,/.test(before.slice(before.lastIndexOf("Record<")))
		: false;
}

/** 對一組 key 算出「命中 ≥3 個值但沒收齊」的 enum。 */
function incompleteDomains(keys: Set<string>): PartialTable[] {
	if (keys.size === 0) return [];
	return GENERATED_ENUMS.map((e) => ({
		enumName: e.name,
		present: [...e.values].filter((v) => keys.has(v)),
		missing: [...e.values].filter((v) => !keys.has(v)),
	})).filter((s) => s.present.length >= 3 && s.missing.length > 0);
}

function findPartialStringTables(source: string): PartialTable[] {
	const out: PartialTable[] = [];
	const seen = new Set<string>();
	for (const block of braceBlocks(source)) {
		// 型別已保證窮盡的表不可能真的漏成員，見 isExhaustivelyTypedRecord。
		if (isExhaustivelyTypedRecord(source, block.open)) continue;
		const keys = new Set([...block.body.matchAll(IDENT_KEY)].map((m) => m[1]));
		for (const hit of incompleteDomains(keys)) {
			// 巢狀區塊會讓同一張表被算到很多次，同 enum 只報一次。
			if (seen.has(hit.enumName)) continue;
			seen.add(hit.enumName);
			out.push(hit);
		}
	}
	return out;
}

test("src/ 沒有任何「以字串碼值為鍵但漏掉值域成員」的對照表", () => {
	const offenders: string[] = [];
	for (const file of collectSourceFiles(SRC)) {
		for (const hit of findPartialStringTables(readFileSync(file, "utf8"))) {
			offenders.push(
				`${relative(projectRoot, file)}  ${hit.enumName} 少了 ${hit.missing.join(", ")}（已列 ${hit.present.join(", ")}）`,
			);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		[
			"以下物件用字串碼值當鍵、卻沒有把值域收齊：",
			...offenders,
			"",
			"標籤一律 codeLabel(XXX_LABEL, code)；呈現 token（顏色 / icon）這類後端不下發的",
			"對照表可以留在前端，但鍵要寫成 [XxxEnum.MEMBER] 的計算鍵形式，值域改了才會跟著壞。",
		].join("\n"),
	);
});

test("型別已保證窮盡的表不報，但 Record<string, …> 不是後門", () => {
	// 沒有這一條，isExhaustivelyTypedRecord 哪天被改寬成「看到 Record< 就跳過」，
	// 上面那支會靜默恆綠——那正是 2026-09-13 這個誤報最容易被修壞的方向。
	// 必須挑「值本身是 UPPER_SNAKE 識別字」的值域：IDENT_KEY 只認那種形狀，
	// 挑到數字碼值域（"0"／"4001"）的話兩條斷言都會失去鑑別力而假性通過。
	const domain = GENERATED_ENUMS.find(
		(e) =>
			[...e.values].length >= 5 &&
			[...e.values].every((v) => /^[A-Z][A-Z0-9_]{2,}$/.test(v)),
	);
	assert.ok(domain, "需要一個 ≥5 個 UPPER_SNAKE 值的值域來造樣本");
	const threeKeys = [...domain.values].slice(0, 3);

	const typed = [
		`const LABELS: Record<SomeLocalUnion, string> = {`,
		...threeKeys.map((k) => `\t${k}: "x",`),
		"};",
	].join("\n");
	assert.deepEqual(
		findPartialStringTables(typed),
		[],
		"帶具名型別註記的表：TypeScript 已保證窮盡，漏成員在結構上不可能，不得回報",
	);

	const loophole = [
		`const LABELS: Record<string, string> = {`,
		...threeKeys.map((k) => `\t${k}: "x",`),
		"};",
	].join("\n");
	assert.ok(
		findPartialStringTables(loophole).some((h) => h.enumName === domain.name),
		"Record<string, …> 是任何物件都能戴的帽子，放行它等於讓手抄表加一行註記就規避 gate",
	);
});

test("字串碼表偵測式本身有鑑別力", () => {
	// 沒有這一條，findPartialStringTables 哪天被改壞成永不命中，上面那支會靜默恆綠。
	const planted = [
		"const STATUS_LABELS = {",
		'\tPENDING: "待處理",',
		'\tRESOLVED_HIDDEN: "已隱藏",',
		'\tRESOLVED_WARNED: "已警告",',
		'\tRESOLVED_BANNED: "已停權",',
		'\tDISMISSED: "已駁回",',
		"};",
	].join("\n");
	assert.ok(
		findPartialStringTables(planted).some(
			(h) => h.enumName === "ReportStatus" && h.missing.includes("APPEALED"),
		),
		`抓不到檢舉頁真正出過的那張殘缺表，實際命中：${JSON.stringify(findPartialStringTables(planted))}`,
	);

	// 正例一：收齊了就不該被判違規（`ACTION_DESCRIPTIONS` 的形狀）。
	assert.deepEqual(
		findPartialStringTables(
			[
				"const D = {",
				'\tHIDE_TARGET: "將被檢舉內容下架",',
				'\tWARN_USER: "記錄警告",',
				'\tBAN_USER: "停權帳號",',
				'\tDISMISS: "駁回檢舉，內容保留",',
				"};",
			].join("\n"),
		),
		[],
		"收齊的操作說明表被誤判成違規",
	);
	// 正例二：同一個檔案裡有一張收齊的表，不該替另一張殘缺的表擋掉告警
	//（這正是「逐區塊而不是逐檔案」存在的理由）。
	assert.ok(
		findPartialStringTables(
			[
				"const D = {",
				'\tHIDE_TARGET: "a",',
				'\tWARN_USER: "b",',
				'\tBAN_USER: "c",',
				'\tDISMISS: "d",',
				"};",
				planted,
			].join("\n"),
		).some((h) => h.enumName === "ReportStatus"),
		"收齊的表把同檔另一張殘缺表的告警吃掉了",
	);
	// 正例三：帶引號的 `case "PENDING":` 不是物件鍵，不該被當成對照表。
	assert.deepEqual(
		findPartialStringTables(
			'switch (s) {\ncase "PENDING":\ncase "DISMISSED":\ncase "RESOLVED_HIDDEN":\nbreak;\n}',
		),
		[],
		"switch 的 case 標籤被誤判成對照表",
	);
});

// ── 手寫的 <option>：碼值當選項時，值域一樣要收齊 ─────────────────────────────────

/**
 * 篩選下拉裡手寫的 `<option value="PENDING">` 是碼表的另一種寄生形態 —— 它連物件字面值
 * 都不是，所以上面每一條都看不到。檢舉頁的狀態篩選就是這樣寫死五個選項的，`APPEALED`
 * 因此**篩不出來**：管理員沒有任何辦法在 ADM 找出申訴中的檢舉。
 *
 * 這種缺口不會編譯錯、不會測試紅，也不會在畫面上留下痕跡 —— 少一個選項看起來就只是
 * 「這個系統只有五種狀態」。
 *
 * 呈現層可以刻意只列子集（≥3 命中才判定），但一旦在列舉某個值域就得列完；
 * 正解是 `toOptions(XXX_LABEL).map(…)`，SSOT 長出新成員時下拉自動跟上。
 */
/** `<option value="PENDING"` —— 值是字面值才算；`value={k}` 與 `value=""` 都不是。 */
const OPTION_VALUE = /<option\s+value="([^"]+)"/g;

test("src/ 沒有任何「手寫 <option> 卻漏掉值域成員」的下拉", () => {
	const offenders: string[] = [];
	for (const file of collectSourceFiles(SRC)) {
		const source = readFileSync(file, "utf8");
		const values = new Set([...source.matchAll(OPTION_VALUE)].map((m) => m[1]));
		for (const hit of incompleteDomains(values)) {
			offenders.push(
				`${relative(projectRoot, file)}  ${hit.enumName} 少了 ${hit.missing.join(", ")}（已列 ${hit.present.join(", ")}）`,
			);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		[
			"以下下拉手寫了碼值選項卻沒收齊值域：",
			...offenders,
			"",
			"漏掉的那個值不只是「顯示不出來」——那筆資料在這一頁**篩不出來**，",
			"等於功能缺一塊而畫面上毫無痕跡。改成 toOptions(XXX_LABEL).map(…)。",
		].join("\n"),
	);
});

test("下拉偵測式本身有鑑別力", () => {
	// 沒有這一條，OPTION_VALUE 哪天被改壞成永不命中，上面那支會靜默恆綠。
	const planted = [
		'<option value="">全部狀態</option>',
		'<option value="PENDING">待處理</option>',
		'<option value="RESOLVED_HIDDEN">已隱藏</option>',
		'<option value="RESOLVED_WARNED">已警告</option>',
		'<option value="RESOLVED_BANNED">已停權</option>',
		'<option value="DISMISSED">已駁回</option>',
	].join("\n");
	const values = new Set([...planted.matchAll(OPTION_VALUE)].map((m) => m[1]));
	assert.ok(
		incompleteDomains(values).some(
			(h) => h.enumName === "ReportStatus" && h.missing.includes("APPEALED"),
		),
		"抓不到檢舉頁真正出過的那組殘缺下拉",
	);
	// 正例：值域外的自訂選項（violations 那幾頁的小寫代碼）不該被誤判。
	assert.deepEqual(
		incompleteDomains(
			new Set(
				[
					...'<option value="low">低</option>\n<option value="high">高</option>\n<option value="critical">危急</option>'.matchAll(
						OPTION_VALUE,
					),
				].map((m) => m[1]),
			),
		),
		[],
		"非 CodeMeta 的自訂選項被誤判成殘缺值域",
	);
});

// ── 檢舉頁：三個篩選下拉真的把值域列完（行為層，不是掃字串） ──────────────────────

/**
 * 上面那條 `<option>` 規則守的是「手寫時不准漏」，但它對「改成 `.filter(…)` 濾掉一個值」
 * 這種寫法沒有意見。這一支把頁面真的渲染出來，直接數選項 —— 這是本票的核心修復
 * （申訴中的檢舉要篩得出來）唯一有行為層鑑別力的守門。
 */
const ReportsAdminPage = requireFromTest("../src/app/admin/reports/page")
	.default as unknown;
const { QueryClient, QueryClientProvider } = requireFromTest(
	"@tanstack/react-query",
) as {
	QueryClient: new (config?: unknown) => unknown;
	QueryClientProvider: unknown;
};

test("檢舉頁的三個篩選下拉各自列出 CodeMeta 值域的每一個值", () => {
	const markup = renderToStaticMarkup(
		createElement(QueryClientProvider, {
			client: new QueryClient({
				defaultOptions: { queries: { retry: false } },
			}),
			children: createElement(ReportsAdminPage),
		}),
	);
	const rendered = new Set(
		[...markup.matchAll(/<option value="([^"]*)"/g)].map((m) => m[1]),
	);
	// 空字串是「全部」選項，三個下拉各一個。
	assert.ok(rendered.has(""), "連下拉都沒渲染出來 —— 這支測到的是空殼");

	for (const [name, values] of [
		["ReportStatus", Object.values(codemeta.ReportStatus)],
		["ReportTargetType", Object.values(codemeta.ReportTargetType)],
		["ReportReasonCode", Object.values(codemeta.ReportReasonCode)],
	] as Array<[string, string[]]>) {
		const missing = values.filter((v) => !rendered.has(v));
		assert.deepEqual(
			missing,
			[],
			`${name} 的這些值篩不出來：${missing.join(", ")}（APPEALED 就是這樣掉了：列表顯示原始碼字串，下拉裡根本沒有那個選項）`,
		);
	}

	// 標籤也要是 SSOT 的，不是頁面自己想的。
	assert.match(markup, /<option value="APPEALED">申訴中<\/option>/u);
});
