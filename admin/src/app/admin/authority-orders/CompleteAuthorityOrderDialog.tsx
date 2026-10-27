"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type AuthorityOrder,
	AuthorityOrderKind,
	type CompleteAuthorityOrderRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	AUTHORITY_FAILED_TEXT,
	buildComplete,
	type CompleteForm,
	emptyCompleteForm,
	type NoticeForm,
	validateComplete,
} from "./authority-actions";
import {
	DATA_PROVIDE_NOTE_TEXT,
	NOTICE_KIND_OPTIONS,
	targetLabel,
} from "./authority-text";
import TargetNoticeFields from "./TargetNoticeFields";

interface CompleteAuthorityOrderDialogProps {
	order: AuthorityOrder;
	onClose: () => void;
	/** 送出完成；reject 時對話框留在原地、保留輸入並就地顯示原因（版本衝突由呼叫端重讀並丟出衝突句）。 */
	onConfirm: (
		request: MessageInitShape<typeof CompleteAuthorityOrderRequestSchema>,
	) => Promise<void>;
}

/**
 * 完成來函（L18、M6、M7）：處理摘要，加上每個對象的告知方式；調取資料的來函可以填提供資料的時間（保存期限會延到提供後 6 個月）。
 * 完成之後保存期限不變。欄位的長度、範圍與告知的組合由伺服器判斷並回已核准的句子。
 */
export default function CompleteAuthorityOrderDialog({
	order,
	onClose,
	onConfirm,
}: CompleteAuthorityOrderDialogProps) {
	const [form, setForm] = useState<CompleteForm>(() =>
		emptyCompleteForm(order.targets.length),
	);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<CompleteForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};
	const editNotice = (index: number, patch: Partial<NoticeForm>) =>
		edit({
			notices: form.notices.map((n, i) =>
				i === index ? { ...n, ...patch } : n,
			),
		});

	const submit = async () => {
		if (submitting) return;
		const invalid = validateComplete(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildComplete(order, form));
		} catch (caught) {
			setError(connectErrorText(caught, AUTHORITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="完成來函"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "完成"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="complete-authority-summary">
				處理摘要 <span className="required">*</span>
			</label>
			<textarea
				id="complete-authority-summary"
				className="form-textarea"
				rows={4}
				value={form.resolutionSummary}
				disabled={submitting}
				onChange={(e) => edit({ resolutionSummary: e.target.value })}
			/>
			<p className="adm-operation-field-note">
				寫下做了什麼、何時回覆機關。完成之後保存期限不變。
			</p>
			{order.kind === AuthorityOrderKind.DATA_REQUEST && (
				<>
					<label
						className="form-label"
						htmlFor="complete-authority-provide-time"
						style={{ marginTop: 16 }}
					>
						提供資料的時間
					</label>
					<input
						id="complete-authority-provide-time"
						type="datetime-local"
						className="form-control"
						value={form.provideTime}
						disabled={submitting}
						onChange={(e) => edit({ provideTime: e.target.value })}
					/>
					<p className="adm-operation-field-note">{DATA_PROVIDE_NOTE_TEXT}</p>
				</>
			)}
			{order.targets.map((target, index) => (
				<fieldset
					key={index}
					style={{
						border: "1px solid var(--admin-border)",
						borderRadius: 8,
						margin: "16px 0 0",
						padding: 12,
						minWidth: 0,
					}}
				>
					<legend>{targetLabel(target)}</legend>
					<TargetNoticeFields
						idPrefix={`complete-authority-notice-${index}`}
						kinds={NOTICE_KIND_OPTIONS}
						value={form.notices[index]}
						disabled={submitting}
						onChange={(patch) => editNotice(index, patch)}
					/>
				</fieldset>
			))}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
