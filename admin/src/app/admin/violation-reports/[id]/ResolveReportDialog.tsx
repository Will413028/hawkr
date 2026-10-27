"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type Report,
	ReportState,
	type ResolveReportRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	buildResolve,
	EMPTY_RESOLVE_FORM,
	livingSourceMeasures,
	NEGATING_OUTCOME_TEXT,
	negatesAsSource,
	RESOLVE_FAILED_TEXT,
	type ResolveForm,
	sourceMeasureNoticeText,
	validateResolve,
} from "../report-actions";
import { reportOutcomeText } from "../report-text";
import { useAllMeasures } from "../use-all-measures";

interface ResolveReportDialogProps {
	report: Pick<Report, "id" | "etag" | "state" | "availableOutcomes">;
	onClose: () => void;
	/** 送出結果；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof ResolveReportRequestSchema>,
	) => Promise<void>;
}

/**
 * 設定結果：選項只列伺服器說此刻可以設定的（available_outcomes）；結果設定後不能修改。
 * 開啟時讀這筆通報當來源的措施，選了會讓它不能再當來源的結果時並排提醒：設定結果不會撤銷已經建立的措施。
 * 這是並排呈現伺服器的關聯資料，不是推導規則；讀取失敗只提示，仍可送出。
 */
export default function ResolveReportDialog({
	report,
	onClose,
	onConfirm,
}: ResolveReportDialogProps) {
	const [form, setForm] = useState<ResolveForm>(EMPTY_RESOLVE_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const measures = useAllMeasures(
		{ sourceReportId: report.id },
		{ fresh: true },
	);
	const measuresLoading = measures.loading;
	const measuresFailed = measures.failed;
	const sourceMeasureIds = measuresLoading
		? []
		: livingSourceMeasures(measures.measures);

	const edit = (patch: Partial<ResolveForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting || measuresLoading) return;
		const invalid = validateResolve(form, report.availableOutcomes);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildResolve(report, form));
		} catch (caught) {
			setError(connectErrorText(caught, RESOLVE_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	const negating = negatesAsSource(report.availableOutcomes, form.outcome);

	return (
		<Dialog
			title="設定結果"
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
						disabled={submitting || measuresLoading}
					>
						{submitting ? "送出中…" : "確認設定"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				結果設定後不能修改。還沒分案的通報只能直接設為未違規、內容不足、重複或濫報。
			</p>
			{report.state === ReportState.IN_CASE && (
				<p className="adm-operation-field-note">
					沒有個別設定結果的通報，會在案件結案時帶入案件的決定。
				</p>
			)}
			<label className="form-label" htmlFor="resolve-report-outcome">
				結果 <span className="required">*</span>
			</label>
			<select
				id="resolve-report-outcome"
				className="form-select"
				value={form.outcome}
				disabled={submitting}
				onChange={(e) =>
					edit({ outcome: Number(e.target.value) as ReportOutcome })
				}
			>
				<option value={ReportOutcome.UNSPECIFIED}>請選擇</option>
				{report.availableOutcomes.map(({ outcome }) => (
					<option key={outcome} value={outcome}>
						{reportOutcomeText(outcome)}
					</option>
				))}
			</select>
			{negating && (
				<p className="adm-operation-field-note">{NEGATING_OUTCOME_TEXT}</p>
			)}
			{negating &&
				sourceMeasureIds.map((id) => (
					<p key={id} className="adm-operation-field-note">
						{sourceMeasureNoticeText(id, form.outcome)}
					</p>
				))}
			{measuresLoading ? (
				<p role="status">載入中...</p>
			) : measuresFailed ? (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>措施讀取失敗，無法確認這筆通報是不是措施的來源。</div>
					<Button variant="outline" onClick={() => measures.retry()}>
						重試
					</Button>
				</div>
			) : null}
			<label
				className="form-label"
				htmlFor="resolve-report-note"
				style={{ marginTop: 16 }}
			>
				給檢舉人的結果說明（選填）
			</label>
			<textarea
				id="resolve-report-note"
				className="form-textarea"
				rows={3}
				value={form.note}
				disabled={submitting}
				onChange={(e) => edit({ note: e.target.value })}
			/>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
