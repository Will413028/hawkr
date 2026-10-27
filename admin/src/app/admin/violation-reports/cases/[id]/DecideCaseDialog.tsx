"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import {
	CaseDecision,
	type DecideCaseRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { CLOSE_WITH_OPEN_INQUIRY_TEXT } from "../../inquiry-actions";
import {
	bringInText,
	buildDecide,
	DECIDE_FAILED_TEXT,
	type DecideForm,
	EMPTY_DECIDE_FORM,
	unseenSupplementsText,
	validateDecide,
} from "../../report-actions";
import { caseDecisionText } from "../../report-text";

interface DecideCaseDialogProps {
	caseId: string;
	onClose: () => void;
	/** 送出決定；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof DecideCaseRequestSchema>,
	) => Promise<void>;
}

/**
 * 結案：決定即結案，不能重開。開啟時重讀案件，用讀到的版本與選項（只列伺服器說此刻可以選的 decision_options），
 * 讀完之前不能送出；結果說明會帶入還沒有結果的通報，帶入的結果名稱由伺服器隨選項給。
 * 案件裡有通報的補充還沒被看過時提醒結案前先看過，不擋送出（補充不改案件的版本）。
 */
export default function DecideCaseDialog({
	caseId,
	onClose,
	onConfirm,
}: DecideCaseDialogProps) {
	const [form, setForm] = useState<DecideForm>(EMPTY_DECIDE_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const reread = useQuery(
		SellerGovernanceAdminService.method.getCase,
		{ id: caseId },
		{ staleTime: 0, refetchOnMount: "always" },
	);
	const loading = reread.isFetching;
	const kase = loading ? undefined : reread.data?.case;
	const failed = !loading && (reread.isError || !kase);

	const edit = (patch: Partial<DecideForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const options = kase?.decisionOptions ?? [];
	const chosen = options.find((option) => option.decision === form.decision);

	const submit = async () => {
		if (submitting || !kase) return;
		const invalid = validateDecide(form, options);
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
			title="結案"
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
						disabled={submitting || loading || !kase}
					>
						{submitting ? "送出中…" : "確認結案"}
					</Button>
				</>
			}
		>
			{loading ? (
				<p role="status">載入中...</p>
			) : failed ? (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{connectErrorText(reread.error, "案件讀取失敗")}</div>
					<Button variant="outline" onClick={() => void reread.refetch()}>
						重試
					</Button>
				</div>
			) : (
				<>
					{kase && kase.unseenSupplementReportCount > 0 && (
						<p className="adm-operation-field-note">
							{unseenSupplementsText(kase.unseenSupplementReportCount)}
						</p>
					)}
					{kase?.awaitingSeller && (
						<p className="adm-operation-field-note">
							{CLOSE_WITH_OPEN_INQUIRY_TEXT}
						</p>
					)}
					<p className="adm-operation-field-note">
						採取措施：要有從這個案件建立的措施（已撤銷的也算），或已有通報的結果是已採取措施。不採取措施：從這個案件建立的措施都已撤銷，而且沒有通報的結果是已採取措施。
					</p>
					<label className="form-label" htmlFor="decide-case-decision">
						結案結果 <span className="required">*</span>
					</label>
					<select
						id="decide-case-decision"
						className="form-select"
						value={form.decision}
						disabled={submitting}
						onChange={(e) =>
							edit({ decision: Number(e.target.value) as CaseDecision })
						}
					>
						<option value={CaseDecision.UNSPECIFIED}>請選擇</option>
						{options.map((option) => (
							<option key={option.decision} value={option.decision}>
								{caseDecisionText(option.decision)}
							</option>
						))}
					</select>
					<p className="adm-operation-field-note">
						{bringInText(
							kase?.unresolvedReportCount ?? 0,
							chosen?.defaultOutcome,
						)}
					</p>
					<label
						className="form-label"
						htmlFor="decide-case-note"
						style={{ marginTop: 16 }}
					>
						給檢舉人的結果說明（選填）
					</label>
					<textarea
						id="decide-case-note"
						className="form-textarea"
						rows={3}
						value={form.note}
						disabled={submitting}
						onChange={(e) => edit({ note: e.target.value })}
					/>
					<p className="adm-operation-field-note">
						會帶入還沒有結果的通報；已經有結果的通報不受影響。
					</p>
				</>
			)}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
