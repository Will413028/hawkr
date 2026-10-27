"use client";

import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import {
	canSubmitDecision,
	DECIDE_REASON_LABEL,
	DECIDE_REJECT_LABEL,
	DECIDE_REVOKE_FINDINGS_LABEL,
	DECISION_REASON_MAX_LENGTH,
	type DecideForm,
	type DecideKind,
	decideAcceptLabel,
	decideTitle,
	REMEDIATION_REJECT_HINT,
	validateDecision,
} from "../measure-actions";

interface DecideMeasureRequestDialogProps {
	kind: DecideKind;
	measure: Measure;
	onClose: () => void;
	/** 送出裁定；reject 時對話框留在原地、保留輸入並就地顯示原因（版本衝突時呼叫端已重讀措施）。 */
	onConfirm: (form: DecideForm) => Promise<void>;
}

/** 裁定申覆或解除請求：結果（接受／駁回）、理由（必填，賣家看得到）。 */
export default function DecideMeasureRequestDialog({
	kind,
	measure,
	onClose,
	onConfirm,
}: DecideMeasureRequestDialogProps) {
	const [accept, setAccept] = useState<boolean | null>(null);
	const [reason, setReason] = useState("");
	const [revokeFindings, setRevokeFindings] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const form: DecideForm = { accept, reason, revokeFindings };
	const edited = () => setError(null);

	const submit = async () => {
		if (submitting || !canSubmitDecision(form)) return;
		const invalid = validateDecision(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(form);
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"裁定失敗，措施沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	const showRevokeFindings =
		kind === "appeal" && accept === true && measure.canRevokeFindings;
	// 理由太長時按鈕停用，原因就地顯示（不是無聲地停用）；空的理由由必填標記與停用的按鈕表達。
	const reasonError = reason.trim() === "" ? undefined : validateDecision(form);
	const shownError = error ?? reasonError ?? null;

	return (
		<Dialog
			title={decideTitle(kind)}
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button
						variant="primary"
						onClick={submit}
						disabled={submitting || !canSubmitDecision(form)}
					>
						{submitting ? "送出中…" : "確認裁定"}
					</Button>
				</>
			}
		>
			<div role="radiogroup" aria-label={decideTitle(kind)}>
				{[
					{ value: true, label: decideAcceptLabel(kind) },
					{ value: false, label: DECIDE_REJECT_LABEL },
				].map((option) => (
					<label
						key={String(option.value)}
						style={{
							display: "flex",
							gap: 8,
							alignItems: "center",
							minHeight: 44,
						}}
					>
						<input
							type="radio"
							name="decide-measure-request"
							checked={accept === option.value}
							disabled={submitting}
							onChange={() => {
								setAccept(option.value);
								edited();
							}}
						/>
						<span>{option.label}</span>
					</label>
				))}
			</div>
			{kind === "remediation" && accept === false && (
				<p className="adm-operation-field-note" role="status">
					{REMEDIATION_REJECT_HINT}
				</p>
			)}
			<label
				className="form-label"
				htmlFor="decide-measure-request-reason"
				style={{ marginTop: 16 }}
			>
				{DECIDE_REASON_LABEL} <span className="required">*</span>
			</label>
			<textarea
				id="decide-measure-request-reason"
				className="form-textarea"
				rows={4}
				value={reason}
				disabled={submitting}
				onChange={(e) => {
					setReason(e.target.value);
					edited();
				}}
			/>
			<p className="adm-operation-field-note">
				最多 {DECISION_REASON_MAX_LENGTH} 字。
			</p>
			{showRevokeFindings && (
				<label
					style={{
						display: "flex",
						gap: 8,
						alignItems: "center",
						minHeight: 44,
					}}
				>
					<input
						type="checkbox"
						checked={revokeFindings}
						disabled={submitting}
						onChange={(e) => {
							setRevokeFindings(e.target.checked);
							edited();
						}}
					/>
					<span>{DECIDE_REVOKE_FINDINGS_LABEL}</span>
				</label>
			)}
			<GovernanceNoticeBanner
				notice={shownError ? { tone: "error", text: shownError } : null}
			/>
		</Dialog>
	);
}
