"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { RecordFDAReferralRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	buildReferral,
	EMPTY_REFERRAL_FORM,
	REFERRAL_FAILED_TEXT,
	type ReferralForm,
	validateReferral,
} from "../../report-actions";
import { FDA_CHANNEL_OPTIONS, fdaChannelText } from "../../report-text";

interface RecordFDAReferralDialogProps {
	caseId: string;
	onClose: () => void;
	/** 送出記錄；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof RecordFDAReferralRequestSchema>,
	) => Promise<void>;
}

/**
 * 記錄已轉報主管機關：食安案件的轉報本身在系統外完成，這裡只記管道與主管機關給的案號（選填），記一次，記過之後不能修改。
 * 記過的案件再送，伺服器回原本的紀錄（頁面說明這次輸入沒有寫入）。
 */
export default function RecordFDAReferralDialog({
	caseId,
	onClose,
	onConfirm,
}: RecordFDAReferralDialogProps) {
	const [form, setForm] = useState<ReferralForm>(EMPTY_REFERRAL_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<ReferralForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		const invalid = validateReferral(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildReferral(caseId, form));
		} catch (caught) {
			setError(connectErrorText(caught, REFERRAL_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="記錄已轉報主管機關"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "記錄轉報"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				轉報本身在系統外完成；這裡只記一次，記過之後不能修改。
			</p>
			<label className="form-label" htmlFor="record-fda-referral-channel">
				管道 <span className="required">*</span>
			</label>
			<select
				id="record-fda-referral-channel"
				className="form-select"
				value={form.channel ?? ""}
				disabled={submitting}
				onChange={(e) =>
					edit({
						channel:
							e.target.value === ""
								? undefined
								: (Number(e.target.value) as ReferralForm["channel"]),
					})
				}
			>
				<option value="">請選擇</option>
				{FDA_CHANNEL_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{fdaChannelText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="record-fda-referral-reference-no"
				style={{ marginTop: 16 }}
			>
				主管機關的案號（選填）
			</label>
			<input
				id="record-fda-referral-reference-no"
				type="text"
				className="form-control"
				value={form.referenceNo}
				disabled={submitting}
				onChange={(e) => edit({ referenceNo: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="record-fda-referral-follow-up"
				style={{ marginTop: 16 }}
			>
				追蹤日期（預設 30 天後）
			</label>
			<input
				id="record-fda-referral-follow-up"
				type="date"
				className="form-control"
				value={form.followUp}
				disabled={submitting}
				onChange={(e) => edit({ followUp: e.target.value })}
			/>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
