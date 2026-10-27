"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useRef, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { actionErrorText } from "@/lib/connect-action";
import {
	ASSESSABLE_CATEGORIES,
	policyCategoryText,
} from "@/lib/policy-category";
import CitedFindingsField, {
	type CitedFindingsAccount,
} from "./CitedFindingsField";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import {
	isResend,
	MEASURE_CONFLICT_TEXT,
	measureKindLabel,
	NOTE_MAX_LENGTH,
	type RequestIdSlot,
	requestIdFor,
	STATEMENT_MAX_LENGTH,
	sourceAuthorityOrderNote,
	violationSeverityText,
} from "./measure";
import SourceReportsField from "./SourceReportsField";
import { SUSPEND_WARNING_TEXT } from "./store-detail";
import { useStoreDetail } from "./use-store-detail";
import {
	buildCreateViolationMeasure,
	type CreateMeasureInitial,
	VIOLATION_MEASURE_OPTIONS,
	VIOLATION_SEVERITY_OPTIONS,
	type ViolationMeasureChoice,
	validateViolationMeasure,
	violationMeasureRequestKey,
} from "./violation-measure";

interface CreateMeasureDialogProps {
	/** 要建立措施的店家（CreateMeasureRequest.store_id）；來源通報與可引用的認定都以它（及它的帳號）為範圍。 */
	storeId: string;
	/** 從案件建立時的案件 id（CreateMeasureRequest.source_case_id）：措施記為這個案件導出的措施，頂端說明。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函（CreateMeasureRequest.source_authority_order_id）：措施記為這份來函導出的措施，頂端說明（M2）。 */
	sourceAuthorityOrder?: { id: string; documentNo: string };
	/**
	 * 開啟時的初始值（發票申訴的停權建議帶種類、結束時間與理由；通報詳情帶預先勾選的來源通報）。只是起點：每一欄都可以改，
	 * 必填的檢查照常，沒有預填的類別仍要選。
	 */
	initial?: CreateMeasureInitial;
	/** 來源通報欄下面的說明（例如預先勾選的通報不只一頁、只勾了最早的 20 筆）。 */
	sourceReportsNote?: string;
	onClose: () => void;
	/** 送出建立；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => Promise<void>;
}

/**
 * 建立措施的確認框（違規依據的警告與停權）。掛在商家詳情與發票申訴頁（之後也掛在通報詳情）。骨架沿用 CreateRiskMeasureDialog：
 * request_id（AIP-155）在對話框開啟時才產生，同一份內容重送沿用同一個（網路失敗後再按一次不會建立第二個措施），內容改過換新的，
 * 關閉再開啟是新的對話框。新認定（類別、嚴重度、來源通報）一定有；一併引用的既有認定選填。
 */
export default function CreateMeasureDialog({
	storeId,
	sourceCaseId,
	sourceAuthorityOrder,
	initial,
	sourceReportsNote,
	onClose,
	onConfirm,
}: CreateMeasureDialogProps) {
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	// 可引用的認定屬於店家的帳號，帳號從店家詳情讀；讀取中或失敗時「依據的認定」欄顯示那個狀態（失敗可重試），不是整欄消失。
	const store = useStoreDetail(storeId);
	const accountId = store.data?.store?.accountId ?? "";
	const account: CitedFindingsAccount =
		accountId !== ""
			? { status: "ready", accountId }
			: store.isLoading
				? { status: "loading" }
				: {
						status: "failed",
						error: store.error ?? undefined,
						retry: () => void store.refetch(),
					};
	const [measureCase, setMeasureCase] = useState<
		ViolationMeasureChoice["case"]
	>(initial?.measure?.case);
	const [category, setCategory] = useState<PolicyCategory>(
		initial?.category ?? PolicyCategory.UNSPECIFIED,
	);
	const [severity, setSeverity] = useState<ViolationSeverity>(
		ViolationSeverity.UNSPECIFIED,
	);
	const [sourceReportIds, setSourceReportIds] = useState<string[]>([
		...(initial?.sourceReportIds ?? []),
	]);
	const [citedFindingIds, setCitedFindingIds] = useState<string[]>([]);
	const [statement, setStatement] = useState(initial?.statement ?? "");
	const [endTime, setEndTime] = useState(
		initial?.measure?.case === "suspension"
			? (initial.measure.endTime ?? "")
			: "",
	);
	const [note, setNote] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		const measure: ViolationMeasureChoice =
			measureCase === "suspension"
				? { case: "suspension", endTime }
				: measureCase === "warning"
					? { case: "warning" }
					: { case: undefined };
		const form = {
			measure,
			category,
			severity,
			sourceReportIds,
			citedFindingIds,
			statement,
			note,
			sourceCaseId,
			sourceAuthorityOrderId: sourceAuthorityOrder?.id,
			sourceAuthorityOrderIds: initial?.sourceAuthorityOrderIds ?? [],
		};
		const key = violationMeasureRequestKey(storeId, form);
		const invalid = validateViolationMeasure(
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
			await onConfirm(buildCreateViolationMeasure(slot.id, storeId, form));
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
	const suspension = measureCase === "suspension";

	return (
		<Dialog
			title="建立措施"
			closeLabel="關閉"
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
					id="create-violation-measure-source-case"
				>
					這個措施會記為案件 {sourceCaseId} 導出的措施。
				</p>
			)}
			{sourceAuthorityOrder && (
				<p
					className="adm-operation-field-note"
					id="create-violation-measure-source-authority-order"
				>
					{sourceAuthorityOrderNote(sourceAuthorityOrder.documentNo)}
				</p>
			)}
			<label className="form-label" htmlFor="create-violation-measure-kind">
				種類 <span className="required">*</span>
			</label>
			<select
				id="create-violation-measure-kind"
				className="form-select"
				value={measureCase ?? ""}
				disabled={submitting}
				onChange={(e) => {
					setMeasureCase(
						VIOLATION_MEASURE_OPTIONS.find((o) => o.case === e.target.value)
							?.case,
					);
					edited();
				}}
			>
				<option value="">請選擇</option>
				{VIOLATION_MEASURE_OPTIONS.map((option) => (
					<option key={option.case} value={option.case}>
						{measureKindLabel(option.kind)}
					</option>
				))}
			</select>
			{suspension && (
				<div
					className="audit-notice rejected"
					style={{ marginTop: 16, marginBottom: 16 }}
				>
					<div className="audit-notice-icon">
						<MaterialSymbol name="error" fill />
					</div>
					<div className="audit-notice-content">
						<div className="audit-notice-text">{SUSPEND_WARNING_TEXT}</div>
					</div>
				</div>
			)}
			<label
				className="form-label"
				htmlFor="create-violation-measure-category"
				style={{ marginTop: 16 }}
			>
				類別 <span className="required">*</span>
			</label>
			<select
				id="create-violation-measure-category"
				className="form-select"
				value={category}
				disabled={submitting}
				onChange={(e) => {
					setCategory(Number(e.target.value) as PolicyCategory);
					edited();
				}}
			>
				<option value={PolicyCategory.UNSPECIFIED}>請選擇</option>
				{ASSESSABLE_CATEGORIES.map((option) => (
					<option key={option} value={option}>
						{policyCategoryText(option)}
					</option>
				))}
			</select>
			<label
				className="form-label"
				htmlFor="create-violation-measure-severity"
				style={{ marginTop: 16 }}
			>
				嚴重度 <span className="required">*</span>
			</label>
			<select
				id="create-violation-measure-severity"
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
			<SourceReportsField
				storeId={storeId}
				selected={sourceReportIds}
				disabled={submitting}
				onChange={(ids) => {
					setSourceReportIds(ids);
					edited();
				}}
			/>
			{sourceReportsNote && (
				<p className="adm-operation-field-note">{sourceReportsNote}</p>
			)}
			<CitedFindingsField
				account={account}
				selected={citedFindingIds}
				disabled={submitting}
				required={false}
				emptyText="這個帳號沒有生效中的認定。"
				noteText="選填；引用的認定必須未撤銷。"
				onChange={(ids) => {
					setCitedFindingIds(ids);
					edited();
				}}
			/>
			<label
				className="form-label"
				htmlFor="create-violation-measure-statement"
				style={{ marginTop: 16 }}
			>
				給賣家的理由 <span className="required">*</span>
			</label>
			<textarea
				id="create-violation-measure-statement"
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
			{suspension && (
				<>
					<label
						className="form-label"
						htmlFor="create-violation-measure-end-time"
						style={{ marginTop: 16 }}
					>
						結束時間（台北時間） <span className="required">*</span>
					</label>
					<input
						id="create-violation-measure-end-time"
						type="datetime-local"
						className="form-control"
						value={endTime}
						disabled={submitting}
						onChange={(e) => {
							setEndTime(e.target.value);
							edited();
						}}
					/>
				</>
			)}
			<label
				className="form-label"
				htmlFor="create-violation-measure-note"
				style={{ marginTop: 16 }}
			>
				內部註記 <span className="required">*</span>
			</label>
			<textarea
				id="create-violation-measure-note"
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
