"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { WaiveFDAReferralRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { buildWaive, REFERRAL_V2_FAILED_TEXT } from "../../report-actions";

interface WaiveFDAReferralDialogProps {
	caseId: string;
	onClose: () => void;
	/** 送出；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof WaiveFDAReferralRequestSchema>,
	) => Promise<void>;
}

/** 不轉報（K7、K8）：必須附原因；標記之後不能再記錄轉報。原因的長度由伺服器判斷。 */
export default function WaiveFDAReferralDialog({
	caseId,
	onClose,
	onConfirm,
}: WaiveFDAReferralDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildWaive(caseId, reason));
		} catch (caught) {
			setError(connectErrorText(caught, REFERRAL_V2_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="不轉報"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "標記不轉報"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="waive-fda-referral-reason">
				原因 <span className="required">*</span>
			</label>
			<textarea
				id="waive-fda-referral-reason"
				className="form-textarea"
				rows={3}
				value={reason}
				disabled={submitting}
				onChange={(e) => {
					setReason(e.target.value);
					setError(null);
				}}
			/>
			<p className="adm-operation-field-note">
				例如主管機關已經介入。標記之後不能再記錄轉報。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
