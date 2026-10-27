"use client";

import type { CaseInquiry } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { INQUIRY_FAILED_TEXT } from "../../inquiry-actions";

interface WithdrawInquiryDialogProps {
	inquiry: CaseInquiry;
	onClose: () => void;
	/** 撤回；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (inquiryId: string) => Promise<void>;
}

/** 撤回詢問（J12、J13）：撤回之後賣家不必再回覆；賣家端的畫面上線前，要另外告知賣家。 */
export default function WithdrawInquiryDialog({
	inquiry,
	onClose,
	onConfirm,
}: WithdrawInquiryDialogProps) {
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(inquiry.id);
		} catch (caught) {
			setError(connectErrorText(caught, INQUIRY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="撤回詢問"
			onClose={onClose}
			busy={submitting}
			describedBy="withdraw-inquiry-note"
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="danger" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "撤回"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note" id="withdraw-inquiry-note">
				撤回之後賣家不必再回覆。賣家端的畫面上線前，請另外告知賣家。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
