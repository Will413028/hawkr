import assert from "node:assert/strict";
import { test } from "node:test";
import { STATUS_FILTERS } from "@/features/invoices/screens/BuyerInvoiceListClient";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/enums";
import { foldInvoiceDocumentStatus } from "@/lib/invoices/documentStatus";

/**
 * B09｜買家發票中心的狀態 pill 契約：**pill 上寫的標籤，必須等於它按下去真的篩得
 * 出來的那一組文件**。
 *
 * 為什麼需要這一支：`STATUS_FILTERS` 的每顆 pill 把自己的 `value` 原封不動當成
 * `status=` 送給端點 68 ⇒ 後端用**原值**過濾；而列表列上的狀態 Chip 走
 * `foldInvoiceDocumentStatus` 的**買家投影**。兩張表只要有一顆對不上，pill 就會說謊：
 * 點「已作廢」只回 `VOIDED` 那一半，另一半 `SUPERSEDED` 在畫面上同樣寫著「已作廢」
 * 卻憑空消失，全程 HTTP 200、零錯誤、四道 gate 全綠（實測：把 `NOT_APPLICABLE`
 * 換成 `VOIDED`，lint / tsc / test:unit / architecture:check 逐項與變異前相同）。
 *
 * 🔴 **1:1 集合一律「推導」不得「寫死」**：底下不出現任何一個狀態碼字面值。合法的
 * pill 集合＝對 CodeMeta 的**全值域**各跑一次 fold、依折疊後的 label 分桶、取桶大小
 * 為 1 者。寫死那四個碼等於把 `documentStatus.ts` 抄第二遍——後端把第 12 個狀態折進
 * 「處理中」時，抄本不會跟著重算，而這支測試存在的理由正是那一刻。
 */

/** CodeMeta 的完整值域（不是手打清單——新增狀態時這裡要自動看得到）。 */
const ALL_STATUSES: readonly string[] = Object.values(InvoiceDocumentStatus);

/** 非「全部」的 pill。空字串是 sentinel（不送 `status=`），不受本契約拘束。 */
const PILL_VALUES: readonly string[] = STATUS_FILTERS.map(
	(item) => item.value as string,
).filter((value) => value !== "");

/** 依「折疊後的 label」把全值域分桶：label → 折到該 label 的所有原值。 */
function bucketsByFoldedLabel(): Map<string, string[]> {
	const buckets = new Map<string, string[]>();
	for (const status of ALL_STATUSES) {
		const { label } = foldInvoiceDocumentStatus(status);
		const bucket = buckets.get(label);
		if (bucket) bucket.push(status);
		else buckets.set(label, [status]);
	}
	return buckets;
}

/** 桶大小 == 1 ⇒ 該 label 只由這一個原值產生 ⇒ 它是買家投影 1:1 的狀態。 */
function oneToOneStatuses(): Set<string> {
	const out = new Set<string>();
	for (const members of bucketsByFoldedLabel().values()) {
		if (members.length === 1) out.add(members[0]);
	}
	return out;
}

/* ── 測量本身要有鑑別力：任一邊退化成空集合，⊆ 會變成恆真 ────────────── */

test("B09 推導前提成立（值域、pill、1:1 集合都不是空的）", () => {
	assert.ok(
		ALL_STATUSES.length > 0,
		"CodeMeta 的 InvoiceDocumentStatus 值域是空的——推導失去意義",
	);
	assert.ok(
		PILL_VALUES.length > 0,
		"STATUS_FILTERS 沒有任何非空 pill——底下的 ⊆ 斷言會恆真",
	);
	assert.ok(
		oneToOneStatuses().size > 0,
		"推導出的 1:1 集合是空的——fold 把所有狀態都折在一起了？",
	);
});

/* ── 主契約 ──────────────────────────────────────────────────────────── */

test("B09 每顆狀態 pill 都是買家投影 1:1 的原值（折疊值不得開 pill）", () => {
	const buckets = bucketsByFoldedLabel();
	const oneToOne = oneToOneStatuses();

	for (const value of PILL_VALUES) {
		assert.ok(
			ALL_STATUSES.includes(value),
			`pill「${value}」不在 CodeMeta 的 InvoiceDocumentStatus 值域裡——` +
				"它送給端點 68 之後永遠篩不到任何東西",
		);

		const { label } = foldInvoiceDocumentStatus(value);
		const sameLabel = buckets.get(label) ?? [];
		assert.ok(
			oneToOne.has(value),
			`pill「${value}」的標籤「${label}」同時涵蓋 ${sameLabel.length} 個原值` +
				`（${sameLabel.join(" / ")}）——按下去只會篩出 ${value} 這一份，` +
				"畫面上其餘同標籤的文件會憑空消失（HTTP 200、零錯誤）。" +
				"折疊值不得開 pill；要開就得先讓後端投影變成 1:1。",
		);
	}
});

test("B09 pill 上的標籤＝該筆文件在列表列上顯示的標籤", () => {
	// pill 走 `statusLabel()`（CodeMeta 原 label），列表列走 fold 的投影 label。
	// 兩者只有在 1:1 時才會一致——不一致代表使用者按了 A、看到滿頁 B。
	for (const value of PILL_VALUES) {
		const pillLabel = INVOICE_DOCUMENT_STATUS_LABEL[value] ?? value;
		const rowLabel = foldInvoiceDocumentStatus(value).label;
		assert.equal(
			pillLabel,
			rowLabel,
			`pill「${value}」上寫「${pillLabel}」，但篩出來的每一列都寫「${rowLabel}」`,
		);
	}
});

test("B09 fold 表覆蓋 CodeMeta 全值域（新增狀態不得靜默落 default）", () => {
	// `foldInvoiceDocumentStatus` 的 default 分支是**執行期**的安全網（後端先上線、
	// CodeMeta 拷貝還沒同步時，顯示「處理中」比白畫面好）。但只要那個值已經進了
	// 本 repo 的 CodeMeta 拷貝，它就必須被顯式折過一次——否則新狀態會靜默混進
	// 「處理中」桶，而沒有任何人決定過它該長什麼樣。
	const unhandled = ALL_STATUSES.filter(
		(status) => foldInvoiceDocumentStatus(status).unknown,
	);
	assert.deepEqual(
		unhandled,
		[],
		`這些狀態已在 CodeMeta 值域內卻沒被 foldInvoiceDocumentStatus 顯式處理：` +
			`${unhandled.join(" / ")}。它們會落 default 靜默顯示「處理中」——` +
			"請在 lib/invoices/documentStatus.ts 補 case（順帶決定要不要開 pill）。",
	);
});
