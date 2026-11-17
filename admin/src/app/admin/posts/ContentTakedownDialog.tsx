"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { AdminPost } from "@hawkr/api/gen/hawkr/admin/social/v1alpha1/social_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useRef, useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import {
	MEASURE_CONFLICT_TEXT,
	NOTE_MAX_LENGTH,
	type RequestIdSlot,
	requestIdFor,
	STATEMENT_MAX_LENGTH,
	violationSeverityText,
} from "@/app/admin/merchants/[id]/measure";
import { VIOLATION_SEVERITY_OPTIONS } from "@/app/admin/merchants/[id]/violation-measure";
import { Button, Dialog } from "@/components/UI";
import { actionErrorText } from "@/lib/connect-action";
import {
	CONTENT_TAKEDOWN_CATEGORIES,
	policyCategoryText,
} from "@/lib/policy-category";
import {
	buildTakedown,
	takedownRequestKey,
	validateTakedown,
} from "./post-actions";

interface Props {
	post: AdminPost;
	onClose: () => void;
	/** 送出建立措施；reject 時對話框留在原地、保留輸入並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => Promise<void>;
}

/**
 * 下架貼文：建立以這則貼文為對象的內容下架措施（違規認定、給賣家的理由與內部註記）。
 * 沒有結束時間，也不影響店家的能力；解除只走申覆或管理員撤銷。
 * 不用 CreateMeasureDialog：它的種類是警告與停權，並帶來源通報與引用認定的欄位，內容下架都不適用。
 */
export default function ContentTakedownDialog({
	post,
	onClose,
	onConfirm,
}: Props) {
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const [category, setCategory] = useState<PolicyCategory>(
		PolicyCategory.UNSPECIFIED,
	);
	const [severity, setSeverity] = useState<ViolationSeverity>(
		ViolationSeverity.UNSPECIFIED,
	);
	const [statement, setStatement] = useState("");
	const [note, setNote] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const edited = () => setError(null);

	const submit = async () => {
		if (submitting) return;
		const form = { category, severity, statement, note };
		const invalid = validateTakedown(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		const key = takedownRequestKey(post, form);
		const slot = requestIdFor(requestIdRef.current, key, () =>
			crypto.randomUUID(),
		);
		requestIdRef.current = slot;
		try {
			await onConfirm(buildTakedown(slot.id, post, form));
		} catch (caught) {
			setError(
				actionErrorText(
					caught,
					"下架失敗，貼文狀態沒有變動；請確認網路狀態後再試一次。",
					MEASURE_CONFLICT_TEXT,
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="下架貼文"
			closeLabel="關閉"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="danger" onClick={submit} disabled={submitting}>
						{submitting ? "下架中…" : "確認下架"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				貼文 {post.id}（店家 {post.storeId}）會以內容下架措施下架，不再公開。
			</p>
			<label className="form-label" htmlFor="takedown-category">
				類別 <span className="required">*</span>
			</label>
			<select
				id="takedown-category"
				className="form-select"
				value={category}
				disabled={submitting}
				onChange={(e) => {
					setCategory(Number(e.target.value) as PolicyCategory);
					edited();
				}}
			>
				<option value={PolicyCategory.UNSPECIFIED}>請選擇</option>
				{CONTENT_TAKEDOWN_CATEGORIES.map((option) => (
					<option key={option} value={option}>
						{policyCategoryText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="takedown-severity"
				style={{ marginTop: 16 }}
			>
				嚴重度 <span className="required">*</span>
			</label>
			<select
				id="takedown-severity"
				className="form-select"
				value={severity}
				disabled={submitting}
				onChange={(e) => {
					setSeverity(Number(e.target.value) as ViolationSeverity);
					edited();
				}}
			>
				<option value={ViolationSeverity.UNSPECIFIED}>請選擇</option>
				{VIOLATION_SEVERITY_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{violationSeverityText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="takedown-statement"
				style={{ marginTop: 16 }}
			>
				給賣家的理由 <span className="required">*</span>
			</label>
			<textarea
				id="takedown-statement"
				className="form-textarea"
				rows={3}
				value={statement}
				disabled={submitting}
				onChange={(e) => {
					setStatement(e.target.value);
					edited();
				}}
			/>
			<p className="adm-operation-field-note">
				賣家看得到；最多 {STATEMENT_MAX_LENGTH} 字。
			</p>
			<label
				className="form-label"
				htmlFor="takedown-note"
				style={{ marginTop: 16 }}
			>
				內部註記 <span className="required">*</span>
			</label>
			<textarea
				id="takedown-note"
				className="form-textarea"
				rows={3}
				value={note}
				disabled={submitting}
				onChange={(e) => {
					setNote(e.target.value);
					edited();
				}}
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
