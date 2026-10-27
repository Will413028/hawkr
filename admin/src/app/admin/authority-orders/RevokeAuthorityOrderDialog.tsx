"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type {
	AuthorityOrder,
	RevokeAuthorityOrderRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { AUTHORITY_FAILED_TEXT, buildRevoke } from "./authority-actions";
import { AUTHORITY_REVOKE_TEXT } from "./authority-text";

interface RevokeAuthorityOrderDialogProps {
	order: AuthorityOrder;
	onClose: () => void;
	/** 送出撤銷；reject 時對話框留在原地、保留輸入並就地顯示原因（版本衝突由呼叫端重讀並丟出衝突句）。 */
	onConfirm: (
		request: MessageInitShape<typeof RevokeAuthorityOrderRequestSchema>,
	) => Promise<void>;
}

/** 撤銷登記：登記錯誤的來函（對象或文號登記錯了）。原因必填，長度由伺服器判斷並回已核准的句子。 */
export default function RevokeAuthorityOrderDialog({
	order,
	onClose,
	onConfirm,
}: RevokeAuthorityOrderDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildRevoke(order, reason));
		} catch (caught) {
			setError(connectErrorText(caught, AUTHORITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title={AUTHORITY_REVOKE_TEXT.action}
			onClose={onClose}
			busy={submitting}
			describedBy="revoke-authority-note"
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : AUTHORITY_REVOKE_TEXT.action}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note" id="revoke-authority-note">
				{AUTHORITY_REVOKE_TEXT.description}
			</p>
			<label className="form-label" htmlFor="revoke-authority-reason">
				{AUTHORITY_REVOKE_TEXT.reasonLabel} <span className="required">*</span>
			</label>
			<textarea
				id="revoke-authority-reason"
				className="form-textarea"
				rows={3}
				value={reason}
				disabled={submitting}
				onChange={(e) => {
					setReason(e.target.value);
					setError(null);
				}}
			/>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
