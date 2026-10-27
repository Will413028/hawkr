"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { RiskGround } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useRef, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { actionErrorText } from "@/lib/connect-action";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import {
	buildCreateRiskMeasure,
	defaultReviewBy,
	isResend,
	MEASURE_CONFLICT_TEXT,
	NOTE_MAX_LENGTH,
	REVIEW_MAX_DAYS,
	type RequestIdSlot,
	RISK_GROUND_OPTIONS,
	requestIdFor,
	requestKey,
	riskGroundLabel,
	STATEMENT_MAX_LENGTH,
	sourceAuthorityOrderNote,
	toLocalInput,
	validateRiskMeasure,
} from "./measure";
import { SUSPEND_WARNING_TEXT } from "./store-detail";

interface CreateRiskMeasureDialogProps {
	storeId: string;
	/** 從案件建立時的案件 id（CreateMeasureRequest.source_case_id）：措施記為這個案件導出的措施，頂端說明。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函（CreateMeasureRequest.source_authority_order_id）：措施記為這份來函導出的措施，頂端說明（M2）。 */
	sourceAuthorityOrder?: { id: string; documentNo: string };
	onClose: () => void;
	/** 送出建立；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => Promise<void>;
}

/**
 * 建立風險措施的確認框（風險依據的停權，條款第 17.1、17.5 條）。版面沿用商家詳情既有的 modal 寫法。
 *
 * request_id（AIP-155）：對話框開啟時才產生，同一份內容重送沿用同一個（網路失敗後再按一次不會建立第二個措施）；
 * 內容改過換新的；關閉再開啟是新的對話框，從頭產生。
 */
export default function CreateRiskMeasureDialog({
	storeId,
	sourceCaseId,
	sourceAuthorityOrder,
	onClose,
	onConfirm,
}: CreateRiskMeasureDialogProps) {
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const [ground, setGround] = useState<RiskGround>(RiskGround.UNSPECIFIED);
	const [statement, setStatement] = useState("");
	const [endTime, setEndTime] = useState("");
	const [reviewBy, setReviewBy] = useState(() =>
		toLocalInput(defaultReviewBy(new Date())),
	);
	const [note, setNote] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		const form = {
			ground,
			statement,
			endTime,
			reviewBy,
			note,
			sourceCaseId,
			sourceAuthorityOrderId: sourceAuthorityOrder?.id,
		};
		const key = requestKey(storeId, form);
		const invalid = validateRiskMeasure(
			form,
			new Date(),
			isResend(requestIdRef.current, key),
		);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		const slot = requestIdFor(requestIdRef.current, key, () =>
			crypto.randomUUID(),
		);
		requestIdRef.current = slot;
		try {
			await onConfirm(buildCreateRiskMeasure(slot.id, storeId, form));
		} catch (caught) {
			setError(
				actionErrorText(
					caught,
					"建立失敗，賣家狀態沒有變動；請確認網路狀態後再試一次。",
					MEASURE_CONFLICT_TEXT,
				),
			);
			setSubmitting(false);
		}
	};

	const edited = () => setError(null);

	return (
		<Dialog
			title="建立風險措施"
			closeLabel="關閉"
			describedBy="create-measure-modal-warning"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="danger" onClick={submit} disabled={submitting}>
						{submitting ? "建立中…" : "確認建立"}
					</Button>
				</>
			}
		>
			{sourceCaseId && (
				<p className="adm-operation-field-note" id="create-measure-source-case">
					這個措施會記為案件 {sourceCaseId} 導出的措施。
				</p>
			)}
			{sourceAuthorityOrder && (
				<p
					className="adm-operation-field-note"
					id="create-measure-source-authority-order"
				>
					{sourceAuthorityOrderNote(sourceAuthorityOrder.documentNo)}
				</p>
			)}
			<div className="audit-notice rejected" style={{ marginBottom: 16 }}>
				<div className="audit-notice-icon">
					<MaterialSymbol name="error" fill />
				</div>
				<div className="audit-notice-content">
					<div className="audit-notice-text" id="create-measure-modal-warning">
						{SUSPEND_WARNING_TEXT}
					</div>
				</div>
			</div>
			<label className="form-label" htmlFor="create-measure-ground">
				風險類型 <span className="required">*</span>
			</label>
			<select
				id="create-measure-ground"
				className="form-select"
				value={ground}
				disabled={submitting}
				onChange={(e) => {
					setGround(Number(e.target.value) as RiskGround);
					edited();
				}}
			>
				<option value={RiskGround.UNSPECIFIED}>請選擇</option>
				{RISK_GROUND_OPTIONS.map((option) => (
					<option key={option} value={option}>
						{riskGroundLabel(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="create-measure-statement"
				style={{ marginTop: 16 }}
			>
				給賣家的理由 <span className="required">*</span>
			</label>
			<textarea
				id="create-measure-statement"
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
				htmlFor="create-measure-review-by"
				style={{ marginTop: 16 }}
			>
				複審期限（台北時間） <span className="required">*</span>
			</label>
			<input
				id="create-measure-review-by"
				type="datetime-local"
				className="form-control"
				value={reviewBy}
				disabled={submitting}
				onChange={(e) => {
					setReviewBy(e.target.value);
					edited();
				}}
			/>
			<p className="adm-operation-field-note">
				預設 7 天後，最多 {REVIEW_MAX_DAYS}{" "}
				天；到期未處理的措施會出現在複審清單。
			</p>
			<label
				className="form-label"
				htmlFor="create-measure-end-time"
				style={{ marginTop: 16 }}
			>
				結束時間（台北時間）
			</label>
			<input
				id="create-measure-end-time"
				type="datetime-local"
				className="form-control"
				value={endTime}
				disabled={submitting}
				onChange={(e) => {
					setEndTime(e.target.value);
					edited();
				}}
			/>
			<p className="adm-operation-field-note">
				選填；不填是沒有固定期間，由平台定期複審，風險排除時再結束措施。
			</p>
			<label
				className="form-label"
				htmlFor="create-measure-note"
				style={{ marginTop: 16 }}
			>
				內部註記 <span className="required">*</span>
			</label>
			<textarea
				id="create-measure-note"
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
