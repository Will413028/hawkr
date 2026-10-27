"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type {
	Case,
	ChangeCaseSeverityRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	buildChangeSeverity,
	EMPTY_SEVERITY_FORM,
	SEVERITY_FAILED_TEXT,
	SEVERITY_OPTIONS,
	type SeverityForm,
	validateSeverity,
} from "../../report-actions";

interface ChangeCaseSeverityDialogProps {
	kase: Pick<Case, "id" | "severity" | "etag">;
	onClose: () => void;
	/** 送出修改；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof ChangeCaseSeverityRequestSchema>,
	) => Promise<void>;
}

/** 改嚴重度：新的嚴重度與原因（寫成案件的一則內部註記，註記標頭記下改前與改後）。 */
export default function ChangeCaseSeverityDialog({
	kase,
	onClose,
	onConfirm,
}: ChangeCaseSeverityDialogProps) {
	const [form, setForm] = useState<SeverityForm>(EMPTY_SEVERITY_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<SeverityForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		const invalid = validateSeverity(form, kase.severity);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildChangeSeverity(kase, form));
		} catch (caught) {
			setError(connectErrorText(caught, SEVERITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="改嚴重度"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "確認修改"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="change-case-severity-value">
				嚴重度 <span className="required">*</span>
			</label>
			<select
				id="change-case-severity-value"
				className="form-select"
				value={form.severity}
				disabled={submitting}
				onChange={(e) =>
					edit({ severity: Number(e.target.value) as ViolationSeverity })
				}
			>
				<option value={ViolationSeverity.UNSPECIFIED}>請選擇</option>
				{SEVERITY_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{violationSeverityText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="change-case-severity-note"
				style={{ marginTop: 16 }}
			>
				原因（內部註記） <span className="required">*</span>
			</label>
			<textarea
				id="change-case-severity-note"
				className="form-textarea"
				rows={3}
				placeholder="寫下為什麼改嚴重度（最多 2000 字）"
				value={form.note}
				disabled={submitting}
				onChange={(e) => edit({ note: e.target.value })}
			/>
			<p className="adm-operation-field-note">
				原因會寫成一則內部註記，只有後台看得到。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
