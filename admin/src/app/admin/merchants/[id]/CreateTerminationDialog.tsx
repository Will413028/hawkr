"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { TerminationOption } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { TerminationGround } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useRef, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { actionErrorText } from "@/lib/connect-action";
import CitedFindingsField from "./CitedFindingsField";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import {
	buildCreateTermination,
	citedFindingsFieldFor,
	MEASURE_CONFLICT_TEXT,
	NOTE_MAX_LENGTH,
	type RequestIdSlot,
	requestIdFor,
	STATEMENT_MAX_LENGTH,
	sourceAuthorityOrderNote,
	type TerminationChoice,
	terminationExplanation,
	terminationGroundLabel,
	terminationRequestKey,
	validateTermination,
} from "./measure";

interface CreateTerminationDialogProps {
	/** 被終止的店家（CreateMeasureRequest.store_id）；關聯帳號的終止是那個帳號的店家，不是目前頁面的店家。 */
	storeId: string;
	/** 被終止的帳號：重大違約從這個帳號的認定中選依據。 */
	accountId: string;
	/**
	 * 關聯帳號的終止（W9）：帶來源措施 id 時事由固定為「關聯帳號」、來源措施不可改；沒帶是一般的終止合作。
	 */
	sourceMeasureId?: string;
	/** 從案件建立時的案件 id（CreateMeasureRequest.source_case_id）：措施記為這個案件導出的措施，頂端說明。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函（CreateMeasureRequest.source_authority_order_id）：措施記為這份來函導出的措施，頂端說明（M2）。 */
	sourceAuthorityOrder?: { id: string; documentNo: string };
	/**
	 * 伺服器此刻允許建立的終止合作（GetStoreResponse.available_terminations 經 terminationAvailability；關聯帳號的終止是
	 * [LinkedAccount.termination_option]）：下拉只列這些，每筆帶生效時機與條件的說明句（直接顯示）以及依據的需求（決定「依據的認定」欄位）。
	 */
	options?: readonly TerminationOption[];
	onClose: () => void;
	/** 送出建立；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => Promise<void>;
}

/**
 * 終止合作確認框（條款第 21.2、21.3 條）。骨架沿用 CreateRiskMeasureDialog：request_id 在對話框開啟時才產生，
 * 同一份內容重送沿用同一個（網路失敗後再按一次不會建立第二個措施），內容改過換新的，關閉再開啟是新的對話框。
 * 終止沒有結束時間與複審期限；說明文字依事由（T7）。
 */
export default function CreateTerminationDialog({
	storeId,
	accountId,
	sourceMeasureId = "",
	sourceCaseId,
	sourceAuthorityOrder,
	options = [],
	onClose,
	onConfirm,
}: CreateTerminationDialogProps) {
	const linked = sourceMeasureId !== "";
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const [ground, setGround] = useState<TerminationGround>(
		// 關聯帳號的終止只有一個選項（LinkedAccount.termination_option），事由固定、不可改。
		linked
			? (options[0]?.ground ?? TerminationGround.UNSPECIFIED)
			: TerminationGround.UNSPECIFIED,
	);
	const [statement, setStatement] = useState("");
	const [note, setNote] = useState("");
	const [citedFindingIds, setCitedFindingIds] = useState<string[]>([]);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	// 選到的終止合作：來自關聯帳號的選項帶來源措施，其他帶依據的認定；請求的事由與欄位都由選項決定，這裡不比較事由。
	const option = options.find((o) => o.ground === ground);
	const choice: TerminationChoice | undefined = !option
		? undefined
		: linked
			? { from: "linked", option, sourceMeasureId }
			: { from: "available", option, citedFindingIds };
	const citedField = option ? citedFindingsFieldFor(option) : undefined;

	const submit = async () => {
		if (submitting) return;
		const form = {
			choice,
			statement,
			note,
			sourceCaseId,
			sourceAuthorityOrderId: sourceAuthorityOrder?.id,
		};
		const checked = validateTermination(form);
		if (checked.error !== undefined) {
			setError(checked.error);
			return;
		}
		const sendable = checked.form;
		setError(null);
		setSubmitting(true);
		const key = terminationRequestKey(storeId, sendable);
		const slot = requestIdFor(requestIdRef.current, key, () =>
			crypto.randomUUID(),
		);
		requestIdRef.current = slot;
		try {
			await onConfirm(buildCreateTermination(slot.id, storeId, sendable));
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
	const explanation = terminationExplanation(option);

	return (
		<Dialog
			title="終止合作"
			closeLabel="關閉"
			describedBy="create-termination-modal-explanation"
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
				<p
					className="adm-operation-field-note"
					id="create-termination-source-case"
				>
					這個措施會記為案件 {sourceCaseId} 導出的措施。
				</p>
			)}
			{sourceAuthorityOrder && (
				<p
					className="adm-operation-field-note"
					id="create-termination-source-authority-order"
				>
					{sourceAuthorityOrderNote(sourceAuthorityOrder.documentNo)}
				</p>
			)}
			<div className="audit-notice rejected" style={{ marginBottom: 16 }}>
				<div className="audit-notice-icon">
					<MaterialSymbol name="error" fill />
				</div>
				<div className="audit-notice-content">
					<div
						className="audit-notice-text"
						id="create-termination-modal-explanation"
					>
						{explanation.length === 0
							? "請先選擇終止事由。"
							: explanation.map((line) => <p key={line}>{line}</p>)}
					</div>
				</div>
			</div>
			<label className="form-label" htmlFor="create-termination-ground">
				終止事由 <span className="required">*</span>
			</label>
			<select
				id="create-termination-ground"
				className="form-select"
				value={ground}
				disabled={submitting || linked}
				onChange={(e) => {
					setGround(Number(e.target.value) as TerminationGround);
					edited();
				}}
			>
				{!linked && (
					<option value={TerminationGround.UNSPECIFIED}>請選擇</option>
				)}
				{options.map((o) => (
					<option key={o.ground} value={o.ground}>
						{terminationGroundLabel(o.ground)}
					</option>
				))}
			</select>
			{linked && (
				<p className="adm-operation-field-note">來源措施：{sourceMeasureId}</p>
			)}
			{!linked && citedField && (
				<CitedFindingsField
					account={{ status: "ready", accountId }}
					selected={citedFindingIds}
					disabled={submitting}
					required={citedField.required}
					emptyText={citedField.emptyText}
					noteText={citedField.noteText}
					onChange={(ids) => {
						setCitedFindingIds(ids);
						edited();
					}}
				/>
			)}
			<label
				className="form-label"
				htmlFor="create-termination-statement"
				style={{ marginTop: 16 }}
			>
				給賣家的理由 <span className="required">*</span>
			</label>
			<textarea
				id="create-termination-statement"
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
				htmlFor="create-termination-note"
				style={{ marginTop: 16 }}
			>
				內部註記 <span className="required">*</span>
			</label>
			<textarea
				id="create-termination-note"
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
