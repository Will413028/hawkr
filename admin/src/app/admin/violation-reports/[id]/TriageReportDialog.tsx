"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type {
	Report,
	TriageReportRequestSchema,
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
	buildTriage,
	EMPTY_ASSESS_FORM,
	SEVERITY_OPTIONS,
	TRIAGE_FAILED_TEXT,
	validateAssess,
} from "../report-actions";

interface TriageReportDialogProps {
	report: Pick<Report, "id" | "category" | "etag" | "triageTargets">;
	onClose: () => void;
	/** 送出分案；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof TriageReportRequestSchema>,
	) => Promise<void>;
}

/**
 * 分案：選平台判定類別，通報歸到這家店那個類別的案件（沒有進行中的案件就開一個，要選嚴重度）。
 * 可選的類別與每個類別的去處（加入哪個案件或開新案件）都來自伺服器隨通報給的 triage_targets；送出時把選的去處原樣放進請求，
 * 讀取之後別人動過那個案件，伺服器回版本衝突。
 */
export default function TriageReportDialog({
	report,
	onClose,
	onConfirm,
}: TriageReportDialogProps) {
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
			await onConfirm(buildTriage(report, form));
		} catch (caught) {
			setError(connectErrorText(caught, TRIAGE_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="分案"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "確認分案"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				平台判定類別決定通報歸到這家店的哪個案件（同一家店、同一個類別一個案件）；不會改掉檢舉人選的類別。
			</p>
			<label className="form-label" htmlFor="triage-report-category">
				平台判定類別 <span className="required">*</span>
			</label>
			<select
				id="triage-report-category"
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
			<p className="adm-operation-field-note">
				檢舉人選的是 {policyCategoryText(report.category)}
			</p>
			{notice && <p className="adm-operation-field-note">{notice.text}</p>}
			{notice?.kind === "open" && (
				<>
					<label
						className="form-label"
						htmlFor="triage-report-severity"
						style={{ marginTop: 16 }}
					>
						嚴重度 <span className="required">*</span>
					</label>
					<select
						id="triage-report-severity"
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
