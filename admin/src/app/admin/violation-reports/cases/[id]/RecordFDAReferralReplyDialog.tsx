"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { RecordFDAReferralReplyRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useRef, useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import {
	type RequestIdSlot,
	requestIdFor,
	toLocalInput,
} from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	buildReferralReply,
	EMPTY_REFERRAL_REPLY_FORM,
	REFERRAL_V2_FAILED_TEXT,
	type ReferralReplyForm,
	referralReplyRequestKey,
} from "../../report-actions";

interface RecordFDAReferralReplyDialogProps {
	caseId: string;
	onClose: () => void;
	/** 送出；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof RecordFDAReferralReplyRequestSchema>,
	) => Promise<void>;
}

/**
 * 記下主管機關的回覆（K9）：收到的日期（預設今天，台北）、內容、下一個追蹤日期（選填）或不再追蹤。回覆只新增；
 * 日期的範圍與兩者同時設定的衝突由伺服器判斷並回已核准的句子。
 */
export default function RecordFDAReferralReplyDialog({
	caseId,
	onClose,
	onConfirm,
}: RecordFDAReferralReplyDialogProps) {
	const [form, setForm] = useState<ReferralReplyForm>(() => ({
		...EMPTY_REFERRAL_REPLY_FORM,
		receiveDate: toLocalInput(new Date()).slice(0, 10),
	}));
	// request_id（AIP-155）：同一份內容重送沿用同一個，內容改了換新的；對話框每次開啟都從沒有開始。
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<ReferralReplyForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			const slot = requestIdFor(
				requestIdRef.current,
				referralReplyRequestKey(caseId, form),
				() => crypto.randomUUID(),
			);
			requestIdRef.current = slot;
			await onConfirm(buildReferralReply(caseId, form, slot.id));
		} catch (caught) {
			setError(connectErrorText(caught, REFERRAL_V2_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="記下主管機關的回覆"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "記下回覆"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="fda-referral-reply-date">
				收到的日期 <span className="required">*</span>
			</label>
			<input
				id="fda-referral-reply-date"
				type="date"
				className="form-control"
				value={form.receiveDate}
				disabled={submitting}
				onChange={(e) => edit({ receiveDate: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="fda-referral-reply-body"
				style={{ marginTop: 16 }}
			>
				內容 <span className="required">*</span>
			</label>
			<textarea
				id="fda-referral-reply-body"
				className="form-textarea"
				rows={4}
				value={form.body}
				disabled={submitting}
				onChange={(e) => edit({ body: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="fda-referral-reply-next"
				style={{ marginTop: 16 }}
			>
				下一個追蹤日期（選填）
			</label>
			<input
				id="fda-referral-reply-next"
				type="date"
				className="form-control"
				value={form.nextFollowUp}
				disabled={submitting || form.clearFollowUp}
				onChange={(e) => edit({ nextFollowUp: e.target.value })}
			/>
			<label
				htmlFor="fda-referral-reply-clear"
				style={{
					display: "flex",
					alignItems: "center",
					gap: 8,
					marginTop: 12,
					minHeight: 44,
				}}
			>
				<input
					id="fda-referral-reply-clear"
					type="checkbox"
					checked={form.clearFollowUp}
					disabled={submitting}
					onChange={(e) =>
						edit({
							clearFollowUp: e.target.checked,
							nextFollowUp: e.target.checked ? "" : form.nextFollowUp,
						})
					}
				/>
				不再追蹤
			</label>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
