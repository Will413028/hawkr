"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { Dialog } from "@/components/UI";
import { formatClockTime } from "@/features/delegated-operations/format";
import {
	DELEGATED_AUDIT_RESOURCE_TYPE_LABELS,
	DELEGATED_AUDIT_RESULT_LABELS,
	DELEGATED_AUDIT_RISK_LABELS,
	delegatedAuditResourceDisplay,
} from "@/features/delegated-operations/labels";
import type { DelegatedAuditLogEntry } from "../governance";
import CopyIconButton from "./CopyIconButton";

/**
 * 稽核內容對話框（mockup image11 §4.11.4）。
 *
 * ⚠️ 面板上不出現 `beforeSnapshot`／`afterSnapshot` —— 那兩欄沒有 API 出口：
 * 快照存的是投影後的原始值（狀態是數字碼、圖片是張數），渲染出來不可讀；
 * 完整比對走維運 SQL。這裡顯示的是後端已經整理好的 `changedFields` 三欄表。
 *
 * 這裡是 modal 而不是側欄：側欄版是 `.adm-audit-layout`（`flex-wrap: wrap`）裡固定
 * 340px 的一欄，視窗寬度不足時會**換行到表格下方**，按「查看前後差異」的人看到的是
 * 「按了沒反應」（2026-08-21 回報）。修法不是替它補捲動與移焦 —— 那只是讓症狀在某
 * 些寬度下不出現；modal 在任何寬度都在畫面正中，不存在「打開在畫面外」這個狀態。
 *
 * ⚠️ 「變更內容（系統紀錄）」排在 body 第一個。這一頁的讀者按下那顆鈕就是為了看它，
 * 其餘 14 列是佐證欄位。它原本排在第 12 個，開啟後還要捲過整頁定義列表才看得到。
 */

interface AuditDetailDialogProps {
	entry: DelegatedAuditLogEntry;
	/** 保留天數是設定值下發（單位天），不得把「2 年」寫死。 */
	auditRetentionDays: number | null;
	onClose: () => void;
}

function CopyableValue({ value, label }: { value: string; label: string }) {
	if (!value) return <span className="adm-audit-detail-empty">—</span>;
	return (
		<span className="adm-audit-copyable">
			<code>{value}</code>
			<CopyIconButton value={value} label={label} />
		</span>
	);
}

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

export default function AuditDetailDialog({
	entry,
	auditRetentionDays,
	onClose,
}: AuditDetailDialogProps) {
	return (
		<Dialog
			variant="operation"
			size="audit"
			title="稽核內容"
			titleIcon="find_in_page"
			description={entry.actionLabel}
			closeLabel="關閉稽核內容"
			onClose={onClose}
		>
			<div className="adm-audit-changed">
				<h3>變更內容（系統紀錄）</h3>
				{entry.changedFields.length === 0 ? (
					<p className="adm-audit-detail-placeholder">
						{/* 失敗的那一列是「寫入沒有發生」，不是「執行了但沒改到東西」。
									    這一頁的讀者正是在判斷代行者到底動了什麼，兩者不可共用一句。 */}
						{entry.result === "FAILED"
							? "這次操作未寫入，因此沒有欄位差異。"
							: entry.result === "PENDING"
								? "結果尚未確認，這一筆沒有已確認的欄位差異。"
								: "這次操作沒有產生欄位差異。"}
					</p>
				) : (
					<table className="adm-audit-changed-table">
						<thead>
							<tr>
								<th scope="col">欄位</th>
								<th scope="col">變更前</th>
								<th scope="col">變更後</th>
							</tr>
						</thead>
						<tbody>
							{entry.changedFields.map((field) => (
								<tr key={field.field}>
									{/* label 一律用後端下發的值。 */}
									<th scope="row">{field.label}</th>
									<td>{field.before || "—"}</td>
									<td>{field.after || "—"}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</div>

			<DetailRow label="操作">
				<strong>{entry.actionLabel}</strong>
				<span className="adm-audit-detail-sub">{entry.action}</span>
			</DetailRow>
			<DetailRow label="結果">
				<span
					className={`adm-audit-result adm-audit-result--${entry.result.toLowerCase()}`}
				>
					{DELEGATED_AUDIT_RESULT_LABELS[entry.result]}
				</span>
				{entry.result === "FAILED" && entry.bizCode !== 0 && (
					<>
						{/*
										說明一律用後端下發的 bizCodeLabel（同 changedFields 的
										label）。前端自建碼→文案對照表就是第二份 CodeMeta 鏡像。
										拿不到說明時只留原碼：顯示原碼還追得到，自己編一句話會
										蓋掉唯一的線索。
									*/}
						{entry.bizCodeLabel && (
							<span className="adm-audit-detail-sub">{entry.bizCodeLabel}</span>
						)}
						<span className="adm-audit-detail-sub">業務碼 {entry.bizCode}</span>
					</>
				)}
				{entry.result === "PENDING" && (
					<span className="adm-audit-detail-sub">
						尚未收到結果確認，這一筆不計入成功數。
					</span>
				)}
			</DetailRow>
			<DetailRow label="風險等級">
				<span className={`adm-audit-risk adm-audit-risk--${entry.riskLevel}`}>
					{DELEGATED_AUDIT_RISK_LABELS[entry.riskLevel]}
				</span>
			</DetailRow>
			<DetailRow label="操作者（Actor）">
				<strong>{entry.actor.displayName}（ADM）</strong>
				<span className="adm-audit-detail-sub">
					Admin ID · {entry.actor.adminId}
				</span>
			</DetailRow>
			<DetailRow label="目標店家（Target）">
				<strong>{entry.target.storeName}</strong>
				<span className="adm-audit-detail-sub">
					Store ID · {entry.target.storeId}
				</span>
				<span className="adm-audit-detail-sub">
					Account ID · {entry.target.sellerAccountId}
				</span>
			</DetailRow>
			<DetailRow label="目標資源">
				<strong>{delegatedAuditResourceDisplay(entry)}</strong>
				<span className="adm-audit-detail-sub">
					{DELEGATED_AUDIT_RESOURCE_TYPE_LABELS[entry.resourceType]}
					{entry.resourceId ? ` · ${entry.resourceId}` : ""}
				</span>
				{(entry.resourceVersionBefore || entry.resourceVersionAfter) && (
					<span className="adm-audit-detail-sub">
						版本 {entry.resourceVersionBefore || "—"} →{" "}
						{entry.resourceVersionAfter || "—"}
					</span>
				)}
			</DetailRow>
			<DetailRow label="操作能力（Capability）">
				<CopyableValue value={entry.capability} label="操作能力" />
			</DetailRow>
			<DetailRow label="代行原因">
				<span>{entry.operationReason || "—"}</span>
				{entry.caseReference && (
					<span className="adm-audit-detail-sub">
						案件編號 · {entry.caseReference}
					</span>
				)}
			</DetailRow>
			<DetailRow label="關聯 ID（Correlation ID）">
				<CopyableValue value={entry.correlationId} label="關聯 ID" />
			</DetailRow>
			<DetailRow label="冪等鍵（Idempotency Key）">
				<CopyableValue value={entry.idempotencyKey} label="冪等鍵" />
			</DetailRow>
			<DetailRow label="裝置">
				{/* userAgent 是後端存的原文，不解析成「Chrome 126 (macOS)」——那會是
							    第二份會隨 UA 慣例過時的衍生值。 */}
				<span className="adm-audit-detail-ua">{entry.userAgent || "—"}</span>
				<span className="adm-audit-detail-sub">來源 · {entry.origin}</span>
			</DetailRow>
			<DetailRow label="賣家通知狀態">
				{entry.notifiedAt ? (
					<>
						{/* 語意是「事件已發布至 MQ」，不是「賣家已讀」。 */}
						<span className="adm-audit-notify">
							<MaterialSymbol name="notifications_active" />
							已透過站內訊息通知賣家
						</span>
						<span className="adm-audit-detail-sub">
							發送時間 · {formatClockTime(entry.notifiedAt)}
						</span>
					</>
				) : (
					<span className="adm-audit-detail-empty">這次操作未發送賣家通知</span>
				)}
			</DetailRow>
			<DetailRow label="保留原則">
				<span>
					{auditRetentionDays === null
						? "—"
						: `依平台稽核政策保留 ${auditRetentionDays} 天`}
				</span>
			</DetailRow>
			<DetailRow label="紀錄時間">
				{/* 與左表同一個格式化函式：後端回 UTC，這一頁一律顯示本地時間。
							    兩邊各印各的，同一列會變成表格 08:03、面板 23:03Z。 */}
				<span>{formatClockTime(entry.createdAt)}</span>
				<span className="adm-audit-detail-sub">
					確認時間 ·{" "}
					{entry.confirmedAt ? formatClockTime(entry.confirmedAt) : "未確認"}
				</span>
				<span className="adm-audit-detail-sub">
					Audit Log ID · {entry.auditLogId}
				</span>
			</DetailRow>
		</Dialog>
	);
}
