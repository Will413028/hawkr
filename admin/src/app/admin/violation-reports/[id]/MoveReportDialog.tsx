"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type {
	MoveReportRequestSchema,
	Report,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { policyCategoryText } from "@/lib/policy-category";
import {
	type AssessForm,
	assessNotice,
	buildMove,
	EMPTY_ASSESS_FORM,
	MOVE_FAILED_TEXT,
	SEVERITY_OPTIONS,
	validateAssess,
} from "../report-actions";

interface MoveReportDialogProps {
	report: Pick<Report, "id" | "etag" | "triageTargets">;
	onClose: () => void;
	/** 送出移動；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof MoveReportRequestSchema>,
	) => Promise<void>;
}

/**
 * 移到其他案件：改這筆通報的平台判定類別，通報移到這家店那個類別的案件（沒有進行中的案件就開一個，要選嚴重度）。
 * 可選的類別（不含目前的類別）與每個類別的去處都來自伺服器隨通報給的 triage_targets；送出時把選的去處原樣放進請求。
 */
export default function MoveReportDialog({
	report,
	onClose,
	onConfirm,
}: MoveReportDialogProps) {
	const [form, setForm] = useState<AssessForm>(EMPTY_ASSESS_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const notice = assessNotice(form.category, report.triageTargets);
	const edit = (patch: Partial<AssessForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		const invalid = validateAssess(form, report.triageTargets);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildMove(report, form));
		} catch (caught) {
			setError(connectErrorText(caught, MOVE_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="移到其他案件"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "確認移動"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				改這筆通報的平台判定類別，通報會移到這家店那個類別的案件。原案件的通報都移走時，原案件會自動關閉，不算一次決定。
			</p>
			<label className="form-label" htmlFor="move-report-category">
				新的平台判定類別 <span className="required">*</span>
			</label>
			<select
				id="move-report-category"
				className="form-select"
				value={form.category}
				disabled={submitting}
				onChange={(e) =>
					edit({ category: Number(e.target.value) as PolicyCategory })
				}
			>
				<option value={PolicyCategory.UNSPECIFIED}>請選擇</option>
				{report.triageTargets.map(({ category: option }) => (
					<option key={option} value={option}>
						{policyCategoryText(option)}
					</option>
				))}
			</select>
			{notice && <p className="adm-operation-field-note">{notice.text}</p>}
			{notice?.kind === "open" && (
				<>
					<label
						className="form-label"
						htmlFor="move-report-severity"
						style={{ marginTop: 16 }}
					>
						嚴重度 <span className="required">*</span>
					</label>
					<select
						id="move-report-severity"
						className="form-select"
						value={form.severity}
						disabled={submitting}
						onChange={(e) =>
							edit({
								severity: Number(e.target.value) as ViolationSeverity,
							})
						}
					>
						<option value={ViolationSeverity.UNSPECIFIED}>請選擇</option>
						{SEVERITY_OPTIONS.map((option) => (
							<option key={option} value={option}>
								{violationSeverityText(option)}
							</option>
						))}
					</select>
				</>
			)}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
