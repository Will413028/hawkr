"use client";

import {
	KYCField,
	KYCIssueCode,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { type FormEvent, useState } from "react";
import { Button, Dialog, StatusBadge } from "@/components/UI";
import type { ActionResult } from "@/lib/connect-action";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import {
	type DocumentChoice,
	ISSUE_MESSAGE_MAX,
	type IssueFieldOption,
	issueAssetForbidden,
	issueAssetRequired,
	issueView,
	KYC_ISSUE_CODE_OPTIONS,
	kycIssueCodeLabel,
	REJECT_TEXT_MAX,
	type ReviewDraft,
	validateIssueDraft,
	validateRejectDraft,
} from "./kyc-application";

/** 不能退回的橫幅在伺服器的句子之後的固定一句。 */
export const REJECT_BLOCKED_KEEP_INPUT_TEXT =
	"你輸入的內容還在，可以關閉視窗後確認申請的狀態。";

interface RejectKycDialogProps {
	/** 草稿（問題、總理由、內部備註）由頁面持有：送出失敗（含版本衝突）、或對話框重開時輸入都還在。 */
	draft: ReviewDraft;
	onDraftChange: (draft: ReviewDraft) => void;
	/** 這份申請可以指出問題的欄位。 */
	options: readonly IssueFieldOption[];
	/** 此刻可以指定的文件（顯示全文之後才有）。 */
	choices: readonly DocumentChoice[];
	/** 伺服器說此刻不能退回的原因（`reject_unavailabilities` 第一個原因的句子，本身是完整一句）；有值時保留輸入，但不能再送出。 */
	blockedReason: string | undefined;
	onClose: () => void;
	/** 送出退回；失敗時回傳錯誤文字，對話框留在原地顯示。 */
	onConfirm: () => Promise<ActionResult>;
}

let issueSeq = 0;

/**
 * 退回申請（RejectKYC）：逐項指出問題（欄位、原因碼、給賣家看的說明；文件類欄位在顯示全文之後可以指定某一張，缺件只標欄位），
 * 加上必填的總理由（賣家看得到）與選填的內部備註（只有後台看得到）。2b-0 契約：issues 至少一項。
 */
export default function RejectKycDialog({
	draft,
	onDraftChange,
	options,
	choices,
	blockedReason,
	onClose,
	onConfirm,
}: RejectKycDialogProps) {
	const blocked = blockedReason !== undefined;
	const [error, setError] = useState<string | null>(null);
	const [addError, setAddError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	// 「加入一項問題」的表單：加入之後清空。
	const [field, setField] = useState<KYCField>(KYCField.KYC_FIELD_UNSPECIFIED);
	const [assetId, setAssetId] = useState("");
	const [code, setCode] = useState<KYCIssueCode>(
		KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED,
	);
	const [message, setMessage] = useState("");

	const option = options.find((o) => o.field === field);
	const fieldChoices = choices.filter((c) => c.field === field);

	const addIssue = () => {
		const invalid = validateIssueDraft(
			{ field, documentAssetId: assetId, code, message },
			options,
			choices,
			draft.issues,
		);
		if (invalid) {
			setAddError(invalid);
			return;
		}
		issueSeq += 1;
		onDraftChange({
			...draft,
			issues: [
				...draft.issues,
				{
					key: `issue-${issueSeq}`,
					field,
					documentAssetId: assetId,
					documentLabel:
						choices.find((c) => c.assetId === assetId)?.label ?? "",
					code,
					message: message.trim(),
				},
			],
		});
		setField(KYCField.KYC_FIELD_UNSPECIFIED);
		setAssetId("");
		setCode(KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED);
		setMessage("");
		setAddError(null);
		setError(null);
	};

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting || blocked) return;
		const invalid = validateRejectDraft(draft);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		const result = await onConfirm();
		// 成功時頁面會關掉對話框；失敗留在原地，問題、理由與備註都還在。
		if (!result.ok) setError(result.text);
		setSubmitting(false);
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="退回申請"
			titleIcon="cancel"
			description="逐項指出有問題的欄位或文件；賣家會看到每一項的原因與說明，修改之後才能重新送審。"
			closeLabel="關閉退回申請視窗"
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						返回
					</Button>
					<Button
						variant="danger"
						type="submit"
						loading={submitting}
						disabled={blocked}
					>
						{submitting ? "正在退回" : "確認退回"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>已指出的問題（至少一項）</span>
				{draft.issues.length === 0 ? (
					<strong>還沒有</strong>
				) : (
					<ul className="kyc-issue-list" aria-label="已指出的問題">
						{draft.issues.map((i) => {
							const view = issueView(i);
							return (
								<li key={i.key}>
									<strong>{view.field}</strong>
									{view.document && <span>（{view.document}）</span>}
									<StatusBadge variant="danger">{view.code}</StatusBadge>
									<span className="kyc-issue-message">{view.message}</span>
									<Button
										variant="ghost"
										size="sm"
										disabled={submitting}
										onClick={() =>
											onDraftChange({
												...draft,
												issues: draft.issues.filter((x) => x.key !== i.key),
											})
										}
									>
										移除
									</Button>
								</li>
							);
						})}
					</ul>
				)}
			</div>

			<fieldset className="kyc-issue-form" disabled={submitting}>
				<legend>加入一項問題</legend>
				<div className="adm-operation-field">
					<label htmlFor="adm-reject-kyc-field">有問題的欄位</label>
					<select
						id="adm-reject-kyc-field"
						value={field}
						onChange={(event) => {
							setField(Number(event.target.value) as KYCField);
							setAssetId("");
							setAddError(null);
						}}
					>
						<option value={KYCField.KYC_FIELD_UNSPECIFIED}>請選擇欄位</option>
						{options.map((o) => (
							<option key={o.field} value={o.field}>
								{o.label}
							</option>
						))}
					</select>
				</div>
				{option?.document && !issueAssetForbidden(code) && (
					<div className="adm-operation-field">
						<label htmlFor="adm-reject-kyc-document">
							哪一張文件
							{issueAssetRequired(code) ? "（必選）" : "（選填）"}
						</label>
						<select
							id="adm-reject-kyc-document"
							value={assetId}
							onChange={(event) => {
								setAssetId(event.target.value);
								setAddError(null);
							}}
						>
							<option value="">
								{issueAssetRequired(code)
									? "請選擇哪一張文件"
									: "整個位置（不指定某一張）"}
							</option>
							{fieldChoices.map((c) => (
								<option key={c.assetId} value={c.assetId}>
									{c.label}
								</option>
							))}
						</select>
						{fieldChoices.length === 0 && (
							<span className="adm-operation-field-note">
								要指定某一張文件，請先在頁面上「顯示全文」。
							</span>
						)}
					</div>
				)}
				<div className="adm-operation-field">
					<label htmlFor="adm-reject-kyc-code">原因</label>
					<select
						id="adm-reject-kyc-code"
						value={code}
						onChange={(event) => {
							const next = Number(event.target.value) as KYCIssueCode;
							setCode(next);
							// 缺少文件指的是整個位置：不能同時指定某一張。
							if (issueAssetForbidden(next)) setAssetId("");
							setAddError(null);
						}}
					>
						<option value={KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED}>
							請選擇原因
						</option>
						{KYC_ISSUE_CODE_OPTIONS.map((c) => (
							<option key={c} value={c}>
								{kycIssueCodeLabel(c)}
							</option>
						))}
					</select>
				</div>
				{option?.document && issueAssetForbidden(code) && (
					<span className="adm-operation-field-note">
						缺少文件只標出欄位，不指定某一張。
					</span>
				)}
				<div className="adm-operation-field adm-operation-field--wide">
					<label htmlFor="adm-reject-kyc-message">給賣家看的說明（必填）</label>
					<textarea
						id="adm-reject-kyc-message"
						value={message}
						onChange={(event) => {
							setMessage(event.target.value);
							setAddError(null);
						}}
						rows={2}
						placeholder="例如：身分證已過期，請重新上傳有效證件"
					/>
					<span className="adm-operation-field-note">
						1 到 {ISSUE_MESSAGE_MAX} 字。
					</span>
				</div>
				{addError && (
					<GovernanceNoticeBanner notice={{ tone: "error", text: addError }} />
				)}
				<Button variant="outline" onClick={addIssue}>
					加入這項問題
				</Button>
			</fieldset>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-reject-kyc-reason">
					退回原因（必填，賣家看得到）
				</label>
				<textarea
					id="adm-reject-kyc-reason"
					value={draft.rejectReason}
					onChange={(event) => {
						onDraftChange({ ...draft, rejectReason: event.target.value });
						setError(null);
					}}
					rows={3}
					placeholder="整份申請的總說明，例如：證件需要重新上傳，其他資料請依各項說明修改"
					disabled={submitting}
				/>
				<span className="adm-operation-field-note">
					最多 {REJECT_TEXT_MAX} 字。
				</span>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-reject-kyc-note">
					內部備註（選填，賣家看不到）
				</label>
				<textarea
					id="adm-reject-kyc-note"
					value={draft.internalNote}
					onChange={(event) => {
						onDraftChange({ ...draft, internalNote: event.target.value });
						setError(null);
					}}
					rows={2}
					disabled={submitting}
				/>
				<span className="adm-operation-field-note">
					只記在審核紀錄給後台看，最多 {REJECT_TEXT_MAX} 字。
				</span>
			</div>

			{/* 伺服器的句子（完整一句、以「。」結尾）與固定的一句分開排，client 不拼接也不加標點。 */}
			<GovernanceNoticeBanner
				notice={
					blocked
						? {
								tone: "error",
								text: [blockedReason, REJECT_BLOCKED_KEEP_INPUT_TEXT],
							}
						: null
				}
			/>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
