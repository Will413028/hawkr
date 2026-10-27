"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	createDelegatedOperationIdempotencyKey,
	DelegatedOperationClientError,
	revokeDelegatedSession,
} from "@/services/delegated-operations";
import type { DelegatedSessionReport } from "../governance";

/**
 * 撤銷是治理動作，不是「結束」：它留下 `revoked_by_admin_id` 與理由，而工作區那
 * 邊的結束不留這些。已到期的工作階段仍可撤銷，正是為了留下這道痕跡。
 *
 * ⚠️ 撤銷失敗時**不換新的冪等鍵**：撤銷可能已經生效而回應沒送到，換一把鍵會讓
 * 後端把重試當成第二次撤銷。
 */

interface SessionRevokeDialogProps {
	report: DelegatedSessionReport;
	onClose: () => void;
	onRevoked: () => void;
}

export default function SessionRevokeDialog({
	report,
	onClose,
	onRevoked,
}: SessionRevokeDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [idempotencyKey] = useState(() => {
		try {
			return createDelegatedOperationIdempotencyKey();
		} catch {
			return "";
		}
	});

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		const trimmed = reason.trim();
		if (trimmed.length === 0) {
			setError("請填寫撤銷理由，它會寫入治理紀錄。");
			return;
		}
		if (!idempotencyKey) {
			setError("瀏覽器無法建立安全的操作識別碼，請更新瀏覽器後再試。");
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await revokeDelegatedSession({
				sessionId: report.sessionId,
				revokeReason: trimmed,
				idempotencyKey,
			});
			onRevoked();
		} catch (caught) {
			setError(
				caught instanceof DelegatedOperationClientError
					? `${caught.message}${caught.traceId ? `（追蹤編號 ${caught.traceId}）` : ""}`
					: "撤銷失敗，請重試；重試會沿用同一次請求。",
			);
			setSubmitting(false);
		}
	};

	// `noValidate`（Dialog 固定開啟）：同 `OperationPreflightDialog` —— `required` 的原生氣泡會在
	// submit 之前攔下，`handleSubmit` 裡的「請填寫撤銷理由，它會寫入治理紀錄。」
	// 因此永遠不會出現，而那句話說明的正是「為什麼要填」。
	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="撤銷這個代行工作階段？"
			titleIcon="gpp_bad"
			description="撤銷即時生效：工作區的下一次操作與查詢都會被拒絕。已結束的工作階段仍可撤銷，用來留下治理痕跡。"
			closeLabel="關閉撤銷視窗"
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						返回
					</Button>
					<Button type="submit" loading={submitting}>
						{submitting ? "正在撤銷" : "撤銷工作階段"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>工作階段</span>
				<strong>{report.displayCode}</strong>
				<span>
					{report.target.storeName}（{report.target.storeId}）
				</span>
				<span>操作者 · {report.actor.displayName}</span>
			</div>
			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-revoke-reason">撤銷理由（必填）</label>
				<textarea
					id="adm-revoke-reason"
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					maxLength={200}
					rows={3}
					placeholder="例如：操作者回報誤選店家，立即中止"
					required
				/>
				<span className="adm-operation-field-note">
					理由會寫入治理紀錄，最多 200 字。
				</span>
			</div>
			{error && (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{error}</div>
				</div>
			)}
		</Dialog>
	);
}
