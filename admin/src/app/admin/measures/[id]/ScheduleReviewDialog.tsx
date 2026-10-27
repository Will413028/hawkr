"use client";

import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import {
	defaultReviewBy,
	NOTE_MAX_LENGTH,
	toLocalInput,
} from "../../merchants/[id]/measure";
import { type ScheduleForm, validateSchedule } from "../measure-actions";

interface ScheduleReviewDialogProps {
	onClose: () => void;
	/** 送出延後；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (form: ScheduleForm) => Promise<void>;
}

/** 延後複審：新的複審期限（現在之後、30 天內）與為什麼延後（內部註記，必填）。 */
export default function ScheduleReviewDialog({
	onClose,
	onConfirm,
}: ScheduleReviewDialogProps) {
	const [form, setForm] = useState<ScheduleForm>(() => ({
		reviewBy: toLocalInput(defaultReviewBy(new Date())),
		note: "",
	}));
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<ScheduleForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		const invalid = validateSchedule(form, new Date());
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
					"延後失敗，措施沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="延後複審"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "確認延後"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="schedule-review-by">
				新的複審期限（30 天內） <span className="required">*</span>
			</label>
			<input
				id="schedule-review-by"
				type="datetime-local"
				className="form-control"
				value={form.reviewBy}
				disabled={submitting}
				onChange={(e) => edit({ reviewBy: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="schedule-review-note"
				style={{ marginTop: 16 }}
			>
				為什麼延後（內部註記，必填） <span className="required">*</span>
			</label>
			<textarea
				id="schedule-review-note"
				className="form-textarea"
				rows={3}
				value={form.note}
				disabled={submitting}
				onChange={(e) => edit({ note: e.target.value })}
			/>
			<p className="adm-operation-field-note">
				賣家看不到；最多 {NOTE_MAX_LENGTH} 字。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
