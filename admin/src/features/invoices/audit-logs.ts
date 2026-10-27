import { INVOICE_OPERATION_TYPE_LABEL, toOptions } from "@/lib/codemeta";

/**
 * A10 發票稽核紀錄的顯示規則。
 *
 * 形狀照 ADM 唯一一支完整的稽核台（`features/delegated-operations/components/
 * SessionAuditConsole.tsx`），但**判定邏輯抽成純函式**放這裡：那一支把 `ACTION_GROUPS`
 * 與 `changeSummaryOf` 寫在元件檔內，於是沒有任何測試守得住它們。
 */

/**
 * 操作類型下拉的分組。分組是**結構推導**（資源前綴），不是人工分類表——加一個新的
 * `ISSUE_*` 值會自動落進「開立」組，不需要有人回來改這裡。
 *
 * 🔴 **值域落差，Phase 1 照 58 §6 寫的做**：端點 61 的 `operation_type` 文件上寫
 * 「`InvoiceOperationType` 值」，而那個 enum 是**對 provider 的操作**六值
 * （ISSUE／QUERY／VOID／ALLOWANCE／REISSUE／VOID_ALLOWANCE）；本頁真正要查的卻是
 * **ADM 的動作**（稅務決策、停用連線、人工解鎖、手動開票…），那些不在六值裡——
 * `targetKind` 的五值（TAX_PROFILE／PROVIDER_ACCOUNT／JOB／DOCUMENT／COMPLAINT）
 * 正好說明本表涵蓋的遠不只 job。
 *
 * ⇒ 下拉用 `INVOICE_OPERATION_TYPE_LABEL`（照規格），而**列**的中文只讀後端下發的
 * 那個值 ⇒ 即使後端實際吐的是另一組字串，列也不會壞，只有下拉會少幾個選項。
 */
export const AUDIT_ACTION_GROUPS: {
	key: string;
	label: string;
	prefix: string;
}[] = [
	{ key: "issue", label: "開立", prefix: "ISSUE" },
	{ key: "void", label: "作廢", prefix: "VOID" },
	{ key: "allowance", label: "折讓", prefix: "ALLOWANCE" },
	{ key: "query", label: "查詢", prefix: "QUERY" },
	{ key: "other", label: "其他", prefix: "" },
];

/** 某個操作類型碼屬於哪一組（最長前綴優先；都不中就是「其他」）。 */
export function auditActionGroupOf(value: string): string {
	let best = "other";
	let bestLength = -1;
	for (const group of AUDIT_ACTION_GROUPS) {
		if (group.prefix === "") continue;
		if (value.startsWith(group.prefix) && group.prefix.length > bestLength) {
			best = group.key;
			bestLength = group.prefix.length;
		}
	}
	return best;
}

/** 下拉的分組結果：`[分組 label, [[value, label], …]]`，空組不渲染。 */
export function auditActionOptionGroups(): Array<
	[string, Array<[string, string]>]
> {
	const all = toOptions(INVOICE_OPERATION_TYPE_LABEL);
	return AUDIT_ACTION_GROUPS.map((group) => {
		const options = all.filter(
			([value]) => auditActionGroupOf(value) === group.key,
		);
		return [group.label, options] as [string, Array<[string, string]>];
	}).filter(([, options]) => options.length > 0);
}

/**
 * 「變更摘要」欄的文字。
 *
 * 🔴 **只有兩種「沒有差異」的說法，而且不可共用一句**：
 *   - `FAILED` → 「未寫入」：這次操作**根本沒有生效**
 *   - 其餘且前後內容皆空 → 「無欄位差異」：生效了，但什麼都沒改（例如 PII 揭露這種
 *     讀取類紀錄）
 * 對正在判斷「這個管理員到底動了什麼」的人，這兩件事的差別就是全部。
 *
 * 🔴 **不得補上第三態 `PENDING` → 「結果未確認」**。既有先例（代行稽核台）有那個分支，
 * 因為它的 DTO 真的產得出 `PENDING`；本 DTO 的 `result` 值域只有 `SUCCESS` / `FAILED`
 * （Gateway `InvoiceAuditLogItem.result`）⇒ 照抄過來會是一支永遠走不到的死碼，而且它會
 * 誤導維護者以為稽核列可能處於「不知道有沒有生效」的狀態。本案沒有這種狀態：稽核列是
 * 寫入路徑的結果，不是 provider 的回報。
 *
 * ⚠️ 有內容時**不解析 JSON**：`beforeJson` / `afterJson` 是 invoice-service 在寫入時
 * 就遮罩好的原文（稽核表保存 5 年，查詢時才遮罩等於 DB 裡存著五年份明文 PII）。
 * 解析它等於把前端契約綁死一個歷史結構，同 A06 的 `orderSnapshotJson`。
 */
export function auditChangeSummary(
	result: string,
	beforeJson: string,
	afterJson: string,
): string {
	if (result === "FAILED") return "未寫入";
	if (beforeJson.trim() === "" && afterJson.trim() === "") return "無欄位差異";
	return "有變更內容，展開查看";
}
