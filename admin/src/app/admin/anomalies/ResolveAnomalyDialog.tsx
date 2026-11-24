"use client";

import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import type { ActionResult } from "@/lib/connect-action";
import { GovernanceNoticeBanner } from "../merchants/[id]/GovernanceNotice";
import {
	EMPTY_RESOLVE_DRAFT,
	RESOLVE_INVALID_TEXT,
	type ResolveDraft,
	type ResolveOutcomeKey,
	type ResolveTarget,
	TARGET_NOUN,
	validateResolveDraft,
} from "./resolve-anomaly";

interface ResolveAnomalyDialogProps {
	target: ResolveTarget;
	/** 物件在列上的顯示編號（付款、轉帳、退款的 ID）。 */
	displayId: string;
	onClose: () => void;
	/** 送出結案；失敗時回傳錯誤文字，對話框留在原地顯示並保留輸入。 */
	onConfirm: (draft: ResolveDraft) => Promise<ActionResult>;
}

const OUTCOMES = [
	{
		value: "succeeded",
		title: "金流商已執行",
		note: "結案成功，要附金流商的物件 ID",
	},
	{
		value: "failed",
		title: "金流商沒有執行",
		note: "結案失敗，不需要物件 ID",
	},
] as const satisfies readonly {
	value: Exclude<ResolveOutcomeKey, "">;
	title: string;
	note: string;
}[];

/**
 * 人工結案（ResolvePaymentAnomaly）：管理員先到金流商核對，再選結果；成功結案要填核對到的金流商物件 ID。
 * 契約只有結果與證據兩個欄位，沒有理由欄；操作者與時間由伺服器與領域變更一起記下。
 */
export default function ResolveAnomalyDialog({
	target,
	displayId,
	onClose,
	onConfirm,
}: ResolveAnomalyDialogProps) {
	const [draft, setDraft] = useState<ResolveDraft>(EMPTY_RESOLVE_DRAFT);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const noun = TARGET_NOUN[target.slot];

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = validateResolveDraft(draft);
		if (invalid) {
			setError(RESOLVE_INVALID_TEXT[invalid]);
			return;
		}
		setError(null);
		setSubmitting(true);
		const result = await onConfirm(draft);
		// 成功時呼叫端會關掉對話框；失敗留在原地，輸入還在。
		if (!result.ok) setError(result.text);
		setSubmitting(false);
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title={`結案${noun}異常`}
			titleIcon="paid"
			description={`請先到金流商核對這筆${noun}實際有沒有執行，再選擇結果。結案之後平台會依結果處理，並留下操作者與時間的紀錄，無法撤回。`}
			closeLabel="關閉結案視窗"
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
					<Button variant="danger" type="submit" loading={submitting}>
						{submitting ? "正在結案" : "確認結案"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>
					{noun} · {displayId}
				</span>
			</div>

			<fieldset className="adm-operation-reasons">
				<legend>
					金流商那邊的結果？ <span className="required">*</span>
				</legend>
				<div className="adm-operation-reason-grid">
					{OUTCOMES.map((o) => (
						<label
							className={`adm-operation-reason-card${draft.outcome === o.value ? " is-selected" : ""}`}
							key={o.value}
						>
							<input
								type="radio"
								name="resolve-outcome"
								value={o.value}
								checked={draft.outcome === o.value}
								disabled={submitting}
								onChange={() => {
									setDraft((d) => ({ ...d, outcome: o.value }));
									setError(null);
								}}
							/>
							<span>
								<strong>{o.title}</strong>
								<small>{o.note}</small>
							</span>
						</label>
					))}
				</div>
			</fieldset>

			{draft.outcome === "succeeded" && (
				<>
					<label
						className="form-label"
						htmlFor="resolve-evidence"
						style={{ marginTop: 16 }}
					>
						金流商物件 ID <span className="required">*</span>
					</label>
					<input
						id="resolve-evidence"
						className="form-input"
						type="text"
						autoComplete="off"
						value={draft.evidence}
						disabled={submitting}
						onChange={(e) => {
							setDraft((d) => ({ ...d, evidence: e.target.value }));
							setError(null);
						}}
					/>
					<p className="adm-operation-field-note">
						填金流商那邊核對到的物件 ID，例如 Stripe 的轉帳或退款 ID。
					</p>
				</>
			)}

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
