"use client";

import { type FormEvent, useState } from "react";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import {
	type IssuanceDraft,
	type LineTotals,
	SOURCE_KIND_LABEL,
	SOURCE_KIND_MANUAL_LINES,
	summariseLines,
} from "@/features/invoices/issuance";
import {
	codeLabel,
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_ISSUER_TYPE_LABEL,
	INVOICE_RECIPIENT_TYPE_LABEL,
} from "@/lib/codemeta";
import { formatAmount } from "@/lib/format";

/**
 * A11 的二次確認框（58A §A11、§B-3.1.5）。
 *
 * ## 為什麼手動開票要有它
 *
 * 端點 57 開出的是**一張真實的稅務憑證**：送出即進國稅局，之後只能作廢或折讓，
 * 沒有撤回。ADM 全站沒有 type-to-confirm primitive，所以最強的保護就是這三件：
 * 必填理由（在表單上收）＋ 明文 checkbox ＋ 送出中 Escape 失效。
 *
 * ## 這裡顯示的金額不是發票金額
 *
 * 端點 57 **沒有不落庫的 preview**（Phase 1 沒有那支端點）⇒ 這裡能顯示的只有
 * 「即將送出的內容摘要」與輸入值的合計。表頭三欄（應稅／零稅率／免稅銷售額）與
 * 稅額一律由後端依 58 §5A.6 計算，前端連欄位都不畫——畫了就是第二個真相。
 *
 * ## 冪等鍵
 *
 * 送出失敗時**不換鍵**：這一把落進稽核列的 `request_id`，換掉就等於同一個操作在
 * 稽核上斷成兩截。（去重本身走的是內容雜湊，不是這把鍵——見 `issuance.ts` 的
 * `createInvoiceIdempotencyKey` 註解。）
 */

/** 摘要列。純顯示，沒有互動。 */
function SummaryRow({ label, value }: { label: string; value: string }) {
	return (
		<div className="invoice-confirm-row">
			<dt>{label}</dt>
			<dd>{value}</dd>
		</div>
	);
}

export default function ManualIssuanceConfirmDialog({
	draft,
	totals,
	onClose,
	onSubmit,
}: {
	draft: IssuanceDraft;
	totals: LineTotals;
	onClose: () => void;
	/** 送出端點 57；reject 時對話框留在原地並就地顯示原因（9347 由呼叫端導頁）。 */
	onSubmit: () => Promise<void>;
}) {
	const [acknowledged, setAcknowledged] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const lines = draft.lines;
	const shownLines = lines.slice(0, 3);
	const restCount = Math.max(0, lines.length - shownLines.length);
	const isManualLines = draft.sourceKind === SOURCE_KIND_MANUAL_LINES;
	const shownTotals = isManualLines ? totals : summariseLines([]);

	const recipientTarget =
		draft.recipientBuyerId ||
		draft.recipientStoreId ||
		draft.recipientName ||
		"（未填）";

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!acknowledged) {
			setError("請先勾選確認，這一步是送出前的最後一道保護。");
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onSubmit();
		} catch (caught) {
			setError(
				governanceErrorText(
					caught,
					"開立失敗，沒有任何發票被開出；請確認欄位與網路狀態後再送一次。",
				),
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="確定要開立這張發票嗎？"
			titleIcon="receipt_long"
			description="送出後會立即產生一張真實的稅務憑證，開出後只能作廢或折讓，無法撤回。"
			closeLabel="關閉手動開立發票確認視窗"
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
						返回修改
					</Button>
					<Button
						variant="danger"
						type="submit"
						loading={submitting}
						disabled={!acknowledged}
					>
						{submitting ? "正在開立" : "確認開立發票"}
					</Button>
				</>
			}
		>
			<dl className="invoice-confirm-summary">
				<SummaryRow
					label="開票人"
					value={`${codeLabel(INVOICE_ISSUER_TYPE_LABEL, draft.issuerKind)}${
						draft.issuerStoreId ? `（store_id ${draft.issuerStoreId}）` : ""
					}`}
				/>
				<SummaryRow
					label="受票人"
					value={`${codeLabel(
						INVOICE_RECIPIENT_TYPE_LABEL,
						draft.recipientKind,
					)} · ${recipientTarget}`}
				/>
				<SummaryRow
					label="文件類型"
					value={codeLabel(INVOICE_DOCUMENT_KIND_LABEL, draft.documentKind)}
				/>
				<SummaryRow
					label="品項來源"
					value={SOURCE_KIND_LABEL[draft.sourceKind] ?? draft.sourceKind}
				/>
				{isManualLines ? null : (
					<SummaryRow label="訂單編號" value={draft.orderNo || "（未填）"} />
				)}
				<SummaryRow label="理由" value={draft.reason} />
			</dl>

			{isManualLines ? (
				<div className="invoice-confirm-lines">
					<p className="invoice-confirm-lines__caption">
						品項（共 {lines.length} 行）
					</p>
					<ul>
						{shownLines.map((line) => (
							<li key={line.key}>
								{line.name || "（未填品名）"} × {line.quantity || "?"} ＠{" "}
								{line.unitPrice || "?"}
							</li>
						))}
						{restCount > 0 ? <li>另 {restCount} 行</li> : null}
					</ul>
					<p className="invoice-confirm-total">
						輸入值合計：{formatAmount(String(shownTotals.total))}
					</p>
				</div>
			) : null}

			<p className="adm-operation-field-note">
				以上為即將送出的內容與輸入值合計。實際發票金額、稅額與銷售額拆分以送出後的系統計算為準。
			</p>

			<div className="adm-operation-field adm-operation-field--wide">
				<label
					className="invoice-confirm-check"
					htmlFor="adm-manual-issuance-ack"
				>
					<input
						id="adm-manual-issuance-ack"
						type="checkbox"
						checked={acknowledged}
						onChange={(event) => {
							setAcknowledged(event.target.checked);
							setError(null);
						}}
						disabled={submitting}
					/>
					<span>我已確認上述內容，並了解發票開出後只能作廢或折讓</span>
				</label>
				<span className="adm-operation-field-note">
					理由與這次操作會寫入發票稽核紀錄。
				</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
