"use client";

import { Dialog } from "@/components/UI";
import CopyIconButton from "@/features/delegated-operations/components/CopyIconButton";
import { auditChangeSummary } from "@/features/invoices/audit-logs";
import type { InvoiceAuditLogItem } from "@/features/invoices/types";
import { codeLabel, INVOICE_OPERATION_TYPE_LABEL } from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";

/**
 * A10 的「查看前後差異」明細（58A §A10）。
 *
 * ## 為什麼是 modal 而不是側欄
 *
 * 逐字沿用 `AuditDetailDialog.tsx` 的判斷：側欄版是 `flex-wrap: wrap` 裡固定寬度的一欄，
 * 視窗不夠寬時會**換行到表格下方**，按鈕按下去的人看到的是「按了沒反應」（2026-08-21
 * 回報）。modal 在任何寬度都在畫面正中。
 *
 * ## 為什麼沒有「欄位｜變更前｜變更後」三欄表
 *
 * 58A §A10 寫「明細 dialog：變更前／後三欄表，**只顯示後端整理好的差異**」——那句話的
 * 前提是後端有一個 `changedFields`（代行稽核台的 DTO 就有）。**端點 61 沒有**：Gateway
 * 的 `InvoiceAuditLogItem` 只有 `beforeJson` / `afterJson` 兩個**字串**。
 * 自己把 JSON 解析成三欄表就是 58A 同一節明令禁止的事（「必須是 `<pre>` 收合區塊、
 * 且不做任何解析後的重新呈現」）——解析等於把前端契約綁死一個歷史結構，而稽核表保存
 * 5 年，那個結構一定會變。⇒ 兩塊 `<pre>`，原樣呈現。
 *
 * ## 遮罩
 *
 * `beforeJson` / `afterJson` 是 invoice-service **寫入時**就遮好的（查詢時才遮罩等於
 * DB 裡躺著五年份明文 PII）。前端零遮罩邏輯、零「顯示完整」切換——沒有完整值可顯示。
 * ⚠️ 若在這裡看到未遮罩的個資，那是**後端的洩漏**：回報，不要在前端補遮（前端補遮會
 * 讓那個洩漏永遠沒有人發現，而 DB 裡的明文一樣留著）。
 */
function DetailRow({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div className="adm-audit-detail-row">
			<span className="adm-audit-detail-label">{label}</span>
			<div className="adm-audit-detail-value">{children}</div>
		</div>
	);
}

export default function InvoiceAuditDetailDialog({
	entry,
	onClose,
}: {
	entry: InvoiceAuditLogItem;
	onClose: () => void;
}) {
	return (
		<Dialog
			variant="operation"
			size="audit"
			title="稽核內容"
			titleIcon="find_in_page"
			description={codeLabel(INVOICE_OPERATION_TYPE_LABEL, entry.operationType)}
			closeLabel="關閉稽核內容"
			onClose={onClose}
		>
			<div className="adm-audit-changed">
				<h3>變更前後內容（系統紀錄，已於寫入時遮罩）</h3>
				<p className="adm-audit-detail-placeholder">
					{auditChangeSummary(entry.result, entry.beforeJson, entry.afterJson)}
				</p>
				<details className="invoice-details">
					<summary>展開變更前內容</summary>
					<pre className="invoice-pre">{entry.beforeJson || "（無）"}</pre>
				</details>
				<details className="invoice-details">
					<summary>展開變更後內容</summary>
					<pre className="invoice-pre">{entry.afterJson || "（無）"}</pre>
				</details>
			</div>

			<DetailRow label="操作">
				<strong>
					{codeLabel(INVOICE_OPERATION_TYPE_LABEL, entry.operationType)}
				</strong>
				<span className="adm-audit-detail-sub">{entry.operationType}</span>
			</DetailRow>
			<DetailRow label="結果">
				<span
					className={`adm-audit-result adm-audit-result--${entry.result === "FAILED" ? "failed" : "succeeded"}`}
				>
					{entry.result || "—"}
				</span>
			</DetailRow>
			<DetailRow label="目標資源">
				<strong>{entry.targetId || "—"}</strong>
				{/* targetKind 的五值不是 CodeMeta enum（沒有 SSOT）⇒ 原樣顯示碼值，
							    不自己寫一張五筆的對照表。同 A03 對 TaxDocumentInfo.kind 的處置。 */}
				<span className="adm-audit-detail-sub">{entry.targetKind || "—"}</span>
			</DetailRow>
			<DetailRow label="操作者">
				<strong>{entry.actorDisplayName || "—"}</strong>
				<span className="adm-audit-detail-sub">
					account_id · {entry.actorAccountId || "—"}
				</span>
			</DetailRow>
			<DetailRow label="理由">
				<span>{entry.reason || "—"}</span>
			</DetailRow>
			<DetailRow label="請求編號">
				{entry.requestId ? (
					<span className="adm-audit-copyable">
						<code>{entry.requestId}</code>
						<CopyIconButton value={entry.requestId} label="請求編號" />
					</span>
				) : (
					<span className="adm-audit-detail-empty">—</span>
				)}
			</DetailRow>
			{/* 只有條款相關的操作才有值；空的時候說「不涉及」而不是「—」，
						    因為這一欄的空與「查不到」不是同一件事。 */}
			<DetailRow label="條款版本">
				<span>{entry.termsVersion || "此操作不涉及條款"}</span>
			</DetailRow>
			<DetailRow label="紀錄時間">
				<span>{formatDateTime(entry.occurredAt)}</span>
				<span className="adm-audit-detail-sub">
					Audit Log ID · {entry.auditLogId || "—"}
				</span>
			</DetailRow>
		</Dialog>
	);
}
