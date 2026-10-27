"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateCaseInquiryRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useRef, useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import {
	type RequestIdSlot,
	requestIdFor,
} from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	buildCreateInquiry,
	EMPTY_INQUIRY_FORM,
	INQUIRY_FAILED_TEXT,
	type InquiryForm,
	inquiryRequestKey,
	validateInquiry,
} from "../../inquiry-actions";
import { INQUIRY_KIND_OPTIONS, inquiryKindText } from "../../inquiry-text";

interface CreateCaseInquiryDialogProps {
	caseId: string;
	onClose: () => void;
	/** 送出詢問；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof CreateCaseInquiryRequestSchema>,
	) => Promise<void>;
}

/**
 * 詢問賣家（J1–J6）：只記下詢問，系統不會通知賣家。request_id（AIP-155）在對話框開啟時才產生，同一份內容重送沿用同一個
 * （網路失敗後再按一次不會建立第二筆詢問），內容改過換新的。
 */
export default function CreateCaseInquiryDialog({
	caseId,
	onClose,
	onConfirm,
}: CreateCaseInquiryDialogProps) {
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const [form, setForm] = useState<InquiryForm>(EMPTY_INQUIRY_FORM);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<InquiryForm>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};

	const submit = async () => {
		if (submitting) return;
		const invalid = validateInquiry(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		const slot = requestIdFor(
			requestIdRef.current,
			inquiryRequestKey(caseId, form),
			() => crypto.randomUUID(),
		);
		requestIdRef.current = slot;
		try {
			await onConfirm(buildCreateInquiry(slot.id, caseId, form));
		} catch (caught) {
			setError(connectErrorText(caught, INQUIRY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="詢問賣家"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "送出中…" : "記下詢問"}
					</Button>
				</>
			}
		>
			<label className="form-label" htmlFor="create-case-inquiry-kind">
				種類 <span className="required">*</span>
			</label>
			<select
				id="create-case-inquiry-kind"
				className="form-select"
				value={form.kind}
				disabled={submitting}
				onChange={(e) =>
					edit({ kind: Number(e.target.value) as InquiryForm["kind"] })
				}
			>
				<option value={0}>請選擇</option>
				{INQUIRY_KIND_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{inquiryKindText(option)}
					</option>
				))}
			</select>
			<p className="adm-operation-field-note">
				提供紀錄：商品的來源、批次、效期、進出貨及流向（依賣家合作條款）。請說明：請賣家說明事實或處理情形。
			</p>
			<label
				className="form-label"
				htmlFor="create-case-inquiry-message"
				style={{ marginTop: 16 }}
			>
				給賣家的說明 <span className="required">*</span>
			</label>
			<textarea
				id="create-case-inquiry-message"
				className="form-textarea"
				rows={4}
				value={form.message}
				disabled={submitting}
				onChange={(e) => edit({ message: e.target.value })}
			/>
			<label
				className="form-label"
				htmlFor="create-case-inquiry-due"
				style={{ marginTop: 16 }}
			>
				期限
			</label>
			<input
				id="create-case-inquiry-due"
				type="datetime-local"
				className="form-control"
				value={form.dueTime}
				disabled={submitting}
				onChange={(e) => edit({ dueTime: e.target.value })}
			/>
			<p className="adm-operation-field-note">
				預設 3 天後；可以設在 1 天到 14 天之間。
			</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
