"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type {
	AuthorityOrder,
	ExtendAuthorityOrderHoldRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	AUTHORITY_FAILED_TEXT,
	buildExtend,
	EMPTY_EXTEND_FORM,
	type ExtendForm,
} from "./authority-actions";

interface ExtendHoldDialogProps {
	order: AuthorityOrder;
	onClose: () => void;
	/** 送出延長；reject 時對話框留在原地、保留輸入並就地顯示原因（版本衝突由呼叫端重讀並丟出衝突句）。 */
	onConfirm: (
		request: MessageInitShape<typeof ExtendAuthorityOrderHoldRequestSchema>,
	) => Promise<void>;
}

/** 延長保存（L19）：只能延長；每次延長留下前後的期限與原因。新的期限必須晚於目前的，由伺服器判斷。 */
export default function ExtendHoldDialog({
	order,
	onClose,
	onConfirm,
}: ExtendHoldDialogProps) {
	const [form, setForm] = useState<ExtendForm>(EMPTY_EXTEND_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<ExtendForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildExtend(order, form));
		} catch (caught) {
			setError(connectErrorText(caught, AUTHORITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="延長保存"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "延長保存"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="extend-hold-until">
				保存到 <span className="required">*</span>
			</label>
			<input
				id="extend-hold-until"
				type="datetime-local"
				className="form-control"
				value={form.holdUntil}
				disabled={submitting}
				onChange={(e) => edit({ holdUntil: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="extend-hold-reason"
				style={{ marginTop: 16 }}
			>
				原因 <span className="required">*</span>
			</label>
			<textarea
				id="extend-hold-reason"
				className="form-textarea"
				rows={3}
				value={form.reason}
				disabled={submitting}
				onChange={(e) => edit({ reason: e.target.value })}
			/>
			<p className="adm-operation-field-note">
				只能延長。例如進入訴訟時，延長到裁判確定後 3
				個月。每次延長會留下前後的期限與原因。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
