"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type Case,
	CaseAction,
	CaseDecision,
	type DecideCaseRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { policyCategoryText } from "@/lib/policy-category";
import {
	buildDecide,
	DECIDE_FAILED_TEXT,
	type DecideForm,
	EMPTY_DECIDE_FORM,
	REASON_MAX_LENGTH,
	SEVERITY_OPTIONS,
	validateDecide,
} from "../../case-decide";
import {
	actionsFor,
	caseActionText,
	caseDecisionText,
	categoriesFor,
} from "../../moderation-text";

interface Props {
	kase: Case;
	onClose: () => void;
	/** 送出決定；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof DecideCaseRequestSchema>,
	) => Promise<void>;
}

/** 決定案件：決定即結案、不能重開。違規要選處置、類別並寫給作者的理由；下架店家的內容還要填內容所屬的店家與嚴重度。 */
export default function DecideModerationCaseDialog({
	kase,
	onClose,
	onConfirm,
}: Props) {
	const [form, setForm] = useState<DecideForm>(EMPTY_DECIDE_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const edit = (patch: Partial<DecideForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};
	const violation = form.decision === CaseDecision.VIOLATION;
	const actions = actionsFor(kase.target?.kind ?? 0);
	const takedown = violation && form.action === CaseAction.TAKEDOWN;

	const submit = async () => {
		if (submitting) return;
		const invalid = validateDecide(kase, form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildDecide(kase, form));
		} catch (caught) {
			setError(connectErrorText(caught, DECIDE_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="決定案件"
			closeLabel="關閉"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "確認決定"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">決定即結案，不能重開。</p>
			<label className="form-label" htmlFor="moderation-decision">
				決定 <span className="required">*</span>
			</label>
			<select
				id="moderation-decision"
				className="form-select"
				value={form.decision}
				disabled={submitting}
				onChange={(e) =>
					edit({
						...EMPTY_DECIDE_FORM,
						decision: Number(e.target.value) as CaseDecision,
					})
				}
			>
				<option value={CaseDecision.UNSPECIFIED}>請選擇</option>
				{[CaseDecision.VIOLATION, CaseDecision.NO_VIOLATION].map((option) => (
					<option key={option} value={option}>
						{caseDecisionText(option)}
					</option>
				))}
			</select>
			{violation && (
				<>
					<label
						className="form-label"
						htmlFor="moderation-action"
						style={{ marginTop: 16 }}
					>
						處置 <span className="required">*</span>
					</label>
					<select
						id="moderation-action"
						className="form-select"
						value={form.action}
						disabled={submitting}
						onChange={(e) =>
							edit({
								action: Number(e.target.value) as CaseAction,
								category: PolicyCategory.UNSPECIFIED,
							})
						}
					>
						<option value={CaseAction.UNSPECIFIED}>請選擇</option>
						{actions.map((option) => (
							<option key={option} value={option}>
								{caseActionText(option)}
							</option>
						))}
					</select>
					<label
						className="form-label"
						htmlFor="moderation-category"
						style={{ marginTop: 16 }}
					>
						類別 <span className="required">*</span>
					</label>
					<select
						id="moderation-category"
						className="form-select"
						value={form.category}
						disabled={submitting || form.action === CaseAction.UNSPECIFIED}
						onChange={(e) =>
							edit({ category: Number(e.target.value) as PolicyCategory })
						}
					>
						<option value={PolicyCategory.UNSPECIFIED}>請選擇</option>
						{categoriesFor(form.action).map((option) => (
							<option key={option} value={option}>
								{policyCategoryText(option)}
							</option>
						))}
					</select>
					{takedown && (
						<>
							<div className="form-label" style={{ marginTop: 16 }}>
								內容所屬的店家 ID
							</div>
							<div id="moderation-store-id">{kase.storeId ?? "-"}</div>
							<p className="adm-operation-field-note">
								取自案件的內容快照；伺服器會向社群核對內容確實屬於這家店。
							</p>
							<label
								className="form-label"
								htmlFor="moderation-severity"
								style={{ marginTop: 16 }}
							>
								嚴重度 <span className="required">*</span>
							</label>
							<select
								id="moderation-severity"
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
					<label
						className="form-label"
						htmlFor="moderation-reason"
						style={{ marginTop: 16 }}
					>
						給作者的理由 <span className="required">*</span>
					</label>
					<textarea
						id="moderation-reason"
						className="form-textarea"
						rows={3}
						value={form.reason}
						disabled={submitting}
						onChange={(e) => edit({ reason: e.target.value })}
					/>
					<p className="adm-operation-field-note">
						作者看得到；最多 {REASON_MAX_LENGTH} 字。
					</p>
				</>
			)}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
