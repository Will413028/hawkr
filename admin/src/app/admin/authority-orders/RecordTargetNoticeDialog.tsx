"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type AuthorityOrder,
	AuthorityTargetNoticeKind,
	type RecordAuthorityTargetNoticeRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	AUTHORITY_FAILED_TEXT,
	buildRecordNotice,
	EMPTY_NOTICE_FORM,
	type NoticeForm,
	TARGET_NOTICE_REQUIRED_TEXT,
} from "./authority-actions";
import { targetLabel } from "./authority-text";
import TargetNoticeFields from "./TargetNoticeFields";

/** 暫不告知的後續只能改成已告知，或寫新的確認日期（契約）。 */
const FOLLOW_UP_KINDS = [
	AuthorityTargetNoticeKind.NOTIFIED,
	AuthorityTargetNoticeKind.WITHHELD,
];

interface RecordTargetNoticeDialogProps {
	order: AuthorityOrder;
	/** 對象在 order.targets 的位置。 */
	position: number;
	onClose: () => void;
	/** 送出；reject 時對話框留在原地、保留輸入並就地顯示原因（版本衝突由呼叫端重讀並丟出衝突句）。 */
	onConfirm: (
		request: MessageInitShape<typeof RecordAuthorityTargetNoticeRequestSchema>,
	) => Promise<void>;
}

/** 記下告知（M8）：機關要求暫不告知的對象，到了確認日期之後改成已告知，或寫新的確認日期。每次改變伺服器都留歷史。 */
export default function RecordTargetNoticeDialog({
	order,
	position,
	onClose,
	onConfirm,
}: RecordTargetNoticeDialogProps) {
	const [form, setForm] = useState<NoticeForm>(EMPTY_NOTICE_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const target = order.targets[position];

	const submit = async () => {
		if (submitting) return;
		if (form.kind === AuthorityTargetNoticeKind.UNSPECIFIED) {
			setError(TARGET_NOTICE_REQUIRED_TEXT);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(buildRecordNotice(order, position, form));
		} catch (caught) {
			setError(connectErrorText(caught, AUTHORITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="記下告知"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "記下告知"}
					</Button>
				</>
			}
		>
			{target && <p>{targetLabel(target)}</p>}
			<TargetNoticeFields
				idPrefix="record-authority-notice"
				kinds={FOLLOW_UP_KINDS}
				value={form}
				disabled={submitting}
				onChange={(patch) => {
					setForm((f) => ({ ...f, ...patch }));
					setError(null);
				}}
			/>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
