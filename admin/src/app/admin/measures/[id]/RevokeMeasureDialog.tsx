"use client";

import { AdminRevokeReason } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import { adminRevokeReasonLabel } from "../../merchants/[id]/measure";
import {
	DECIDE_REVOKE_FINDINGS_LABEL,
	isRevokeOption,
	REVOKE_REASON_OPTIONS,
} from "../measure-actions";

interface RevokeMeasureDialogProps {
	measure: Measure;
	onClose: () => void;
	/** 送出撤銷；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		reason: AdminRevokeReason,
		revokeFindings: boolean,
	) => Promise<void>;
}

/** 撤銷措施：撤銷原因，以及（伺服器說可以，can_revoke_findings 時）是否一併撤銷認定。 */
export default function RevokeMeasureDialog({
	measure,
	onClose,
	onConfirm,
}: RevokeMeasureDialogProps) {
	const [reason, setReason] = useState<AdminRevokeReason>(
		AdminRevokeReason.UNSPECIFIED,
	);
	const [revokeFindings, setRevokeFindings] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const valid = isRevokeOption(reason);

	const submit = async () => {
		if (submitting || !valid) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(reason, revokeFindings);
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"撤銷失敗，措施沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="撤銷措施"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button
						variant="danger"
						onClick={submit}
						disabled={submitting || !valid}
					>
						{submitting ? "撤銷中…" : "確認撤銷"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="revoke-measure-reason">
				撤銷原因 <span className="required">*</span>
			</label>
			<select
				id="revoke-measure-reason"
				className="form-select"
				value={reason}
				disabled={submitting}
				onChange={(e) => {
					setReason(Number(e.target.value) as AdminRevokeReason);
					setError(null);
				}}
			>
				<option value={AdminRevokeReason.UNSPECIFIED}>請選擇</option>
				{REVOKE_REASON_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{adminRevokeReasonLabel(option)}
					</option>
				))}
			</select>
			{measure.canRevokeFindings && (
				<label
					style={{
						display: "flex",
						gap: 8,
						alignItems: "center",
						minHeight: 44,
						marginTop: 16,
					}}
				>
					<input
						type="checkbox"
						checked={revokeFindings}
						disabled={submitting}
						onChange={(e) => {
							setRevokeFindings(e.target.checked);
							setError(null);
						}}
					/>
					<span>{DECIDE_REVOKE_FINDINGS_LABEL}</span>
				</label>
			)}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
