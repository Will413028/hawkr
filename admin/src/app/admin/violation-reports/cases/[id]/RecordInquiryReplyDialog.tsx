"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CaseInquiry } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import type { RecordCaseInquiryReplyRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useRef, useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { toLocalInputSeconds } from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { uploadImageToMedia } from "@/services/media-upload";
import {
	buildInquiryReply,
	emptyReplyForm,
	INQUIRY_FAILED_TEXT,
	type InquiryReplyForm,
	inquiryEvidenceOverText,
} from "../../inquiry-actions";
import {
	INQUIRY_CHANNEL_OPTIONS,
	inquiryChannelText,
} from "../../inquiry-text";

/** 附件一律是機敏檔（B27）：經 gateway 到 media-service 的機敏上傳，回 staged token。 */
const uploadSecure = (file: File) =>
	uploadImageToMedia(file, { isSecure: true });

interface RecordInquiryReplyDialogProps {
	inquiry: CaseInquiry;
	onClose: () => void;
	/** 送出；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof RecordCaseInquiryReplyRequestSchema>,
	) => Promise<void>;
	/** 上傳一份附件並回 staged token；預設是機敏上傳，測試可換掉。 */
	upload?: (file: File) => Promise<string>;
}

/**
 * 記下賣家的回覆（J7–J10）：在系統外收到的回覆，記下之後這筆詢問就結束。附件在送出時才上傳成機敏檔；
 * 上傳過的檔記住它的 token，送出失敗後再按一次不會重傳。內容與時間的範圍由伺服器判斷並回已核准的句子。
 */
export default function RecordInquiryReplyDialog({
	inquiry,
	onClose,
	onConfirm,
	upload = uploadSecure,
}: RecordInquiryReplyDialogProps) {
	const [form, setForm] = useState<InquiryReplyForm>(() =>
		emptyReplyForm(toLocalInputSeconds(new Date())),
	);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const tokens = useRef(new Map<File, string>());

	const edit = (patch: Partial<InquiryReplyForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		// 份數上限是伺服器的（reply_evidence_max）；超過就在上傳之前擋下，不浪費機敏上傳。
		if (form.files.length > inquiry.replyEvidenceMax) {
			setError(inquiryEvidenceOverText(inquiry.replyEvidenceMax));
			return;
		}
		setError(null);
		setSubmitting(true);
		const staged: string[] = [];
		try {
			// 只有上傳步驟的失敗用固定的失敗句（J15）：上傳的錯誤是內部字串（缺 token、逾時、Failed to fetch），不給使用者看。
			for (const file of form.files) {
				let token = tokens.current.get(file);
				if (token === undefined) {
					token = await upload(file);
					tokens.current.set(file, token);
				}
				staged.push(token);
			}
		} catch {
			setError(INQUIRY_FAILED_TEXT);
			setSubmitting(false);
			return;
		}
		try {
			await onConfirm(buildInquiryReply(inquiry.id, form, staged));
		} catch (caught) {
			setError(connectErrorText(caught, INQUIRY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="記下賣家的回覆"
			onClose={onClose}
			busy={submitting}
			describedBy="record-inquiry-reply-note"
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
			<p className="adm-operation-field-note" id="record-inquiry-reply-note">
				記下在系統外收到的回覆。記下之後這筆詢問就結束；要再問，請新增一筆詢問。
			</p>
			<label className="form-label" htmlFor="record-inquiry-reply-channel">
				管道 <span className="required">*</span>
			</label>
			<select
				id="record-inquiry-reply-channel"
				className="form-select"
				value={form.channel}
				disabled={submitting}
				onChange={(e) =>
					edit({
						channel: Number(e.target.value) as InquiryReplyForm["channel"],
					})
				}
			>
				<option value={0}>請選擇</option>
				{INQUIRY_CHANNEL_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{inquiryChannelText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="record-inquiry-reply-time"
				style={{ marginTop: 16 }}
			>
				收到的時間 <span className="required">*</span>
			</label>
			<input
				id="record-inquiry-reply-time"
				type="datetime-local"
				step="1"
				className="form-control"
				value={form.receiveTime}
				disabled={submitting}
				onChange={(e) => edit({ receiveTime: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="record-inquiry-reply-body"
				style={{ marginTop: 16 }}
			>
				回覆內容 <span className="required">*</span>
			</label>
			<textarea
				id="record-inquiry-reply-body"
				className="form-textarea"
				rows={4}
				value={form.body}
				disabled={submitting}
				onChange={(e) => edit({ body: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="record-inquiry-reply-files"
				style={{ marginTop: 16 }}
			>
				附件（選填，最多 {inquiry.replyEvidenceMax} 份）
			</label>
			<input
				id="record-inquiry-reply-files"
				type="file"
				multiple
				accept="image/*,.pdf"
				disabled={submitting}
				onChange={(e) => edit({ files: [...(e.target.files ?? [])] })}
			/>
			{form.files.length > 0 && (
				<ul style={{ margin: "8px 0 0", paddingLeft: 20 }}>
					{form.files.map((file) => (
						<li key={`${file.name}-${file.size}-${file.lastModified}`}>
							{file.name}
						</li>
					))}
				</ul>
			)}
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
