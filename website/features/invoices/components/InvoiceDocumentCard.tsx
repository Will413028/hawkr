"use client";

import * as React from "react";
import { Chip } from "@/components/ui";
import { buttonClasses } from "@/components/ui/Button";
import { carrierRowKindOf } from "@/features/invoices/carrierDisplay";
import { INVOICE_ORDER_TEXT } from "@/features/invoices/i18n";
import {
	INVOICE_DOCUMENT_KIND_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/generated";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import Link from "@/lib/intl/native-link";
import { invoiceFileUrl } from "@/lib/invoices/buyer-repo";
import { foldInvoiceDocumentStatus } from "@/lib/invoices/documentStatus";
import { invoiceFileUnavailableHint } from "@/lib/invoices/orderInvoiceState";
import type { InvoiceDocumentSummary } from "@/lib/invoices/types";

const T = INVOICE_ORDER_TEXT;

const COPY_RESET_MS = 1500;

const DT_CLASS =
	"m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0";
const DD_CLASS = "m-0 break-words text-sm text-hk-body";

/**
 * 文件類型的中文一律取自 CodeMeta（§0.4 ⑤）。認不得的值退回原字串而不是空白：
 * 卡片標題空掉會讓整張卡看起來壞了。
 */
function kindLabel(kind: string): string {
	return INVOICE_DOCUMENT_KIND_LABEL[kind] ?? kind;
}

interface InvoiceDocumentCardProps {
	orderNo: string;
	document: InvoiceDocumentSummary;
	/**
	 * 同一張訂單裡的其他文件，只用來把 `relatedDocumentId` 換成看得懂的發票號碼。
	 * 查不到就只給連結，不猜。
	 */
	siblings: InvoiceDocumentSummary[];
	/**
	 * 是否渲染「下載 PDF」與通知信箱列。
	 *
	 * 只有**當前有效的那一份**（最後一筆 `ISSUED`）給——作廢鏈上會同時有三份文件，
	 * 每份都掛一顆下載鍵等於請買家自己分辨哪一張才算數；而通知信箱是訂單層級的
	 * 單一事實，逐卡重複只是噪音。
	 */
	isCurrent: boolean;
	/** 端點 8 的 `notifiedEmailMasked`。為空代表通知還沒寄成功 ⇒ 整列不渲染。 */
	notifiedEmailMasked: string;
}

/**
 * 一份發票／收據文件的卡片（58A §B04）。
 *
 * `status` 的折疊表在 `lib/invoices/documentStatus.ts` 一處，與 B05 共用。
 * 「已作廢」用實心 danger Chip，**不加刪除線**（影響可讀性且 repo 無先例）。
 */
export function InvoiceDocumentCard({
	orderNo,
	document,
	siblings,
	isCurrent,
	notifiedEmailMasked,
}: InvoiceDocumentCardProps) {
	const [copied, setCopied] = React.useState(false);
	const status = foldInvoiceDocumentStatus(document.status);
	const kind = kindLabel(document.documentKind);
	const voided =
		document.status === InvoiceDocumentStatus.VOIDED ||
		document.status === InvoiceDocumentStatus.SUPERSEDED;
	const related = document.relatedDocumentId
		? (siblings.find(
				(sibling) => sibling.documentId === document.relatedDocumentId,
			) ?? null)
		: null;

	const copy = async () => {
		try {
			await navigator.clipboard.writeText(document.invoiceNumber);
			setCopied(true);
			window.setTimeout(() => setCopied(false), COPY_RESET_MS);
		} catch {
			/* clipboard may be denied — silently no-op */
		}
	};

	return (
		<li className="rounded-hkui-md border border-divider bg-content2 p-4">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<h3 className="m-0 text-sm font-semibold text-hk-heading">{kind}</h3>
				{/* 作廢／被重開取代用實心 danger 大標籤（不加刪除線）。這是**強調**
				    不是第二張碼表——中文仍然來自折疊表；而 tone 相同的
				    RESOLVED_EXTERNALLY 不該被強調成失敗，所以判的是 status 本身。 */}
				<Chip
					variant={voided ? "solid" : "flat"}
					color={voided ? "danger" : status.tone}
					size="sm"
				>
					{status.label}
				</Chip>
			</div>

			<dl className="m-0 mt-3 grid grid-cols-[88px_minmax(0,1fr)] gap-x-4 gap-y-2 max-[540px]:grid-cols-1 max-[540px]:gap-x-0 max-[540px]:gap-y-1">
				{document.invoiceNumber ? (
					<>
						<dt className={DT_CLASS}>{T.fieldInvoiceNo}</dt>
						<dd className={`${DD_CLASS} flex flex-wrap items-center gap-2`}>
							<span className="tabular-nums font-semibold">
								{document.invoiceNumber}
							</span>
							<button
								type="button"
								className="shrink-0 rounded-full border border-divider bg-transparent px-3 py-1.5 text-xs font-bold text-hk-body"
								onClick={() => void copy()}
								aria-label={T.copyInvoiceNo}
							>
								{copied ? T.copied : "複製"}
							</button>
							{copied ? (
								<span role="status" className="sr-only">
									{T.copied}
								</span>
							) : null}
						</dd>
					</>
				) : null}
				{document.issuedAt ? (
					<>
						<dt className={DT_CLASS}>{T.fieldIssuedAt}</dt>
						<dd className={`${DD_CLASS} tabular-nums`}>
							{formatTaipeiDateTime(document.issuedAt) || document.issuedAt}
						</dd>
					</>
				) : null}
				<dt className={DT_CLASS}>{T.fieldAmount}</dt>
				<dd className={`${DD_CLASS} tabular-nums`}>
					NT$ {Number(document.totalAmount || 0).toLocaleString()}
				</dd>
				{/* backend#3084：carrierSummary 在沒有載具時裝的是**買受人統編**。
				    這張卡沒有專屬的統編列，所以改標籤而不是不渲染 ——
				    標成「載具」會在沒有載具的 B2B 上說謊。 */}
				<dt className={DT_CLASS}>
					{carrierRowKindOf(document.carrierSummary, document.carrierType) ===
					"recipientTaxId"
						? T.fieldRecipientTaxId
						: T.fieldCarrier}
				</dt>
				<dd className={DD_CLASS}>{document.carrierSummary || T.noCarrier}</dd>
				{isCurrent && notifiedEmailMasked ? (
					<>
						<dt className={DT_CLASS}>{T.fieldNotifiedEmail}</dt>
						<dd className={DD_CLASS}>{notifiedEmailMasked}</dd>
					</>
				) : null}
			</dl>

			{document.relatedDocumentId ? (
				<p className="m-0 mt-3 text-xs text-hk-muted">
					{related?.invoiceNumber ? (
						<>
							{T.relatedWithNumber(related.invoiceNumber)}
							{" · "}
						</>
					) : null}
					<Link
						href={`/account/invoices/${encodeURIComponent(document.relatedDocumentId)}`}
						className="underline underline-offset-2"
					>
						{T.relatedDocument}
					</Link>
				</p>
			) : null}

			{!document.fileAvailable ? (
				<p className="m-0 mt-3 text-xs text-hk-muted">
					{invoiceFileUnavailableHint(document.fileUnavailableReason)}
				</p>
			) : null}

			<div className="mt-4 flex flex-wrap justify-end gap-2">
				<Link
					href={`/account/invoices/${encodeURIComponent(document.documentId)}`}
					className={buttonClasses({
						variant: "bordered",
						color: "default",
						size: "lg",
					})}
				>
					{T.viewDocument}
				</Link>
				{isCurrent && document.fileAvailable ? (
					// 預取好的 `<a>` 而不是「點擊 → await → window.open」：`await` 之後
					// 的 `window.open` 會被 Safari 的彈窗攔截器擋掉。
					<a
						href={invoiceFileUrl(orderNo, document.documentId)}
						target="_blank"
						rel="noreferrer noopener"
						className={buttonClasses({ size: "lg" })}
						aria-label={`下載 ${kind} PDF，訂單 ${orderNo}`}
					>
						{T.downloadPdf}
					</a>
				) : null}
			</div>
		</li>
	);
}
