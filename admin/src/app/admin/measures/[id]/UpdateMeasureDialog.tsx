"use client";

import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import { STATEMENT_MAX_LENGTH } from "../../merchants/[id]/measure";
import {
	endTimeField,
	initialUpdateForm,
	rebaseForm,
	statementLockReasons,
	type UpdateForm,
	updatePaths,
	validateUpdate,
} from "../measure-actions";

interface UpdateMeasureDialogProps {
	/** 要修改的措施；版本衝突時呼叫端重讀，這裡的輸入照舊、之後的送出帶新的 etag。 */
	measure: Measure;
	onClose: () => void;
	/** 送出修改；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	/** form 是目前的輸入，baseline 是管理員開始編輯時（或上次重讀時沒動過的欄位）的值：只有與 baseline 不同的欄位才是管理員改的。 */
	onConfirm: (form: UpdateForm, baseline: UpdateForm) => Promise<void>;
}

/**
 * 修改措施：給賣家的理由與結束時間。理由只在 can_update_statement 時可以編輯；結束時間的上下限依 can_extend／can_shorten；
 * 不能改的欄位旁邊顯示伺服器給的原因。沒有改任何欄位時送出鍵停用。
 */
export default function UpdateMeasureDialog({
	measure,
	onClose,
	onConfirm,
}: UpdateMeasureDialogProps) {
	const [baseline, setBaseline] = useState<UpdateForm>(() =>
		initialUpdateForm(measure),
	);
	const [form, setForm] = useState<UpdateForm>(() =>
		initialUpdateForm(measure),
	);
	// 措施被重讀（版本衝突後 etag 變了）：沒動過的欄位換成新措施的值，管理員改過的欄位保留輸入。
	const [seenEtag, setSeenEtag] = useState(measure.etag);
	if (seenEtag !== measure.etag) {
		const next = rebaseForm(measure, baseline, form);
		setSeenEtag(measure.etag);
		setBaseline(next.baseline);
		setForm(next.form);
	}
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const changed = updatePaths(measure, form, baseline).length > 0;
	const statementReasons = statementLockReasons(measure);
	const endField = endTimeField(measure, new Date());

	const submit = async () => {
		if (submitting || !changed) return;
		const invalid = validateUpdate(measure, form, baseline, new Date());
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(form, baseline);
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"修改失敗，措施沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	const edit = (patch: Partial<UpdateForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	return (
		<Dialog
			title="修改措施"
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
						disabled={submitting || !changed}
					>
						{submitting ? "送出中…" : "確認修改"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="update-measure-statement">
				給賣家的理由
			</label>
			<textarea
				id="update-measure-statement"
				className="form-textarea"
				rows={4}
				value={form.statement}
				disabled={submitting || !measure.canUpdateStatement}
				aria-describedby={
					statementReasons.length > 0
						? "update-measure-statement-reasons"
						: undefined
				}
				onChange={(e) => edit({ statement: e.target.value })}
			/>
			{statementReasons.length > 0 ? (
				<ul
					id="update-measure-statement-reasons"
					className="adm-operation-field-note"
				>
					{statementReasons.map((text) => (
						<li key={text}>{text}</li>
					))}
				</ul>
			) : (
				<p className="adm-operation-field-note">
					最多 {STATEMENT_MAX_LENGTH} 字。
				</p>
			)}
			<label
				className="form-label"
				htmlFor="update-measure-end-time"
				style={{ marginTop: 16 }}
			>
				結束時間
			</label>
			<input
				id="update-measure-end-time"
				type="datetime-local"
				className="form-control"
				value={form.endTime}
				min={endField.min}
				max={endField.max}
				disabled={submitting || endField.disabled}
				aria-describedby={
					endField.reasons.length > 0
						? "update-measure-end-time-reasons"
						: undefined
				}
				onChange={(e) => edit({ endTime: e.target.value })}
			/>
			{endField.reasons.length > 0 && (
				<ul
					id="update-measure-end-time-reasons"
					className="adm-operation-field-note"
				>
					{endField.reasons.map((text) => (
						<li key={text}>{text}</li>
					))}
				</ul>
			)}
			<p className="adm-operation-field-note">
				賣家會看到新的理由與期間，並產生新的版本
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
