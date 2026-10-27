"use client";

import type * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Chip } from "@/components/ui";
import Button, { buttonClasses } from "@/components/ui/Button";
import { carrierRowKindOf } from "@/features/invoices/carrierDisplay";
import { DocumentTimeline } from "@/features/invoices/components/DocumentTimeline";
import {
	INVOICE_DOCUMENT_TEXT,
	INVOICE_ORDER_TEXT,
} from "@/features/invoices/i18n";
import {
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_LINE_TYPE_LABEL,
	InvoiceDocumentStatus,
	InvoiceTaxCategory,
} from "@/lib/codemeta/generated";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useInvoiceDocument,
	useResendInvoiceNotification,
} from "@/lib/hooks/useInvoiceDocument";
import Link from "@/lib/intl/native-link";
import { hasRecipientTaxId } from "@/lib/invoices/display";
import {
	canResendInvoiceNotification,
	foldInvoiceDocumentStatus,
} from "@/lib/invoices/documentStatus";
import { invoiceFileUnavailableHint } from "@/lib/invoices/orderInvoiceState";
import type { InvoiceLine } from "@/lib/invoices/types";

const T = INVOICE_DOCUMENT_TEXT;

const CARD_CLASS = "bg-content1 border border-divider rounded-hkui-lg p-5";
const DT_CLASS =
	"m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0";
const DD_CLASS = "m-0 break-words text-sm text-hk-body";
const DL_CLASS =
	"m-0 grid grid-cols-[96px_minmax(0,1fr)] gap-x-4 gap-y-2 max-[540px]:grid-cols-1 max-[540px]:gap-x-0 max-[540px]:gap-y-1";

function Card({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<section className={CARD_CLASS}>
			<h2 className="text-base font-semibold mb-3">{title}</h2>
			{children}
		</section>
	);
}

/** 中文一律取自 CodeMeta（§0.4 ⑤）；認不得的值退回原字串而不是空白。 */
function kindLabel(kind: string): string {
	return INVOICE_DOCUMENT_KIND_LABEL[kind] ?? kind;
}

function lineTypeLabel(lineType: string): string {
	return INVOICE_LINE_TYPE_LABEL[lineType] ?? lineType;
}

function money(raw: string): string {
	return `NT$ ${Number(raw || 0).toLocaleString()}`;
}

function stamp(raw: string): string {
	return formatTaipeiDateTime(raw) || raw;
}

/** `ZERO_RATED`／`TAX_EXEMPT` 時在稅額列旁補一行小字；應稅不補。 */
function taxCategoryNote(taxCategory: string): string {
	switch (taxCategory) {
		case InvoiceTaxCategory.ZERO_RATED:
			return T.zeroRatedNote;
		case InvoiceTaxCategory.TAX_EXEMPT:
			return T.taxExemptNote;
		default:
			return "";
	}
}

function LineRow({ line }: { line: InvoiceLine }) {
	return (
		<li className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-divider py-2 last:border-b-0">
			<span className="min-w-0 flex-1 break-words text-sm text-hk-body">
				{line.name || lineTypeLabel(line.lineType)}
			</span>
			<span className="text-xs text-hk-muted tabular-nums">
				{line.quantity} × {money(line.unitPrice)}
			</span>
			{/* DISCOUNT 的 netAmount 本來就是負數，照原樣顯示負號，不 Math.abs()
			    之後自己加減號——那會在後端某天改用正數表示折抵時靜默翻轉。 */}
			<span className="text-sm font-semibold text-hk-body tabular-nums">
				{money(line.netAmount)}
			</span>
		</li>
	);
}

/**
 * B05｜發票／收據文件詳情（58A §B05）。整頁只打端點 69 一次。
 *
 * 三件事值得先讀：
 *
 *   1. **路由是 document-scoped**（`/account/invoices/[documentId]`）。發票通知的
 *      深連結就是這個形狀，而平台→買家方向的發票根本不掛在任何訂單底下 ⇒ 頁面
 *      拿不到、也不需要 `orderNo`。「回上一頁」因此用 `BackLink`：從 B04 點進來
 *      會退回那張訂單，從通知點進來則落到訂單列表。
 *   2. **致命錯誤態判 `!detail` 而不是 `error || !detail`**：背景 refetch 失敗時
 *      保留最後一份好資料，只有真的沒東西可顯示才換頁（照 `AfterSaleDetailClient`）。
 *      文案與「不存在」一致，**不得寫「你沒有權限查看」**——端點 69 對越權與不
 *      存在回同一個 9301。
 *   3. **`invoiceFileKey` 不顯示也不使用**：它是 media objectKey，`secure/invoice/`
 *      前綴在公開 CDN 面 fail-closed 而 image-gateway 沒有簽章機制。
 *
 * ⚠️ **本頁沒有「下載 PDF」**。買家側的 PDF 二進位只有 order-scoped 一條子路由
 * （`/app/orders/{orderNo}/invoice_documents/{documentId}/file/content`），而本頁
 * 沒有 `orderNo`；`/app/invoices/documents/{id}/file` 回的是 objectKey 不是網址。
 * 下載入口目前只在 B04。`fileAvailable === false` 時仍照規格顯示原因句。
 *
 * ⚠️ **「重新寄送通知」（端點 71）已接上**（2026-09-01 dev 實測 200 ＋ 真的寄出；
 * `ResendBuyerInvoiceNotification` 在 `internal/grpc/notification_resend.go`）。
 * 兩個限制寫在這裡免得下次又被當成缺口：
 *   ① 買家側的回應**沒有 `nextResendAvailableAt`**（那一欄只在賣家的端點 31）⇒
 *      做不出規格描述的倒數，只能在拿到 `9352` 時說「剛剛才寄過」。
 *   ② 入口的閘門是 `canResendInvoiceNotification(summary.status)`，**不是**
 *      「有沒有發票號碼」——後端的可重寄三值含 `VOIDED`，而作廢的文件也有號碼。
 */
export default function InvoiceDocumentDetailClient({
	documentId,
}: {
	documentId: string;
}) {
	const { detail, loading, error } = useInvoiceDocument(documentId);

	if (loading && !detail) {
		return (
			<main>
				<SellerPageShell title={T.pageTitle} eyebrow={T.eyebrow}>
					<p className="text-foreground-soft text-sm">{T.loading}</p>
				</SellerPageShell>
			</main>
		);
	}

	if (!detail) {
		return (
			<main>
				<SellerPageShell
					title={T.notFoundTitle}
					actions={
						<BackLink
							fallbackHref="/account/orders"
							className={buttonClasses({
								variant: "bordered",
								color: "default",
								size: "sm",
							})}
						>
							{T.backToOrders}
						</BackLink>
					}
				>
					<p className="text-danger-700 text-sm font-semibold">
						{error || T.notFoundBody}
					</p>
				</SellerPageShell>
			</main>
		);
	}

	const { summary } = detail;
	const status = foldInvoiceDocumentStatus(summary.status);
	const voided =
		summary.status === InvoiceDocumentStatus.VOIDED ||
		summary.status === InvoiceDocumentStatus.SUPERSEDED;
	const kind = kindLabel(summary.documentKind);
	const taxNote = taxCategoryNote(detail.taxCategory);

	return (
		<main>
			<SellerPageShell
				title={T.pageTitle}
				eyebrow={T.eyebrow}
				subtitle={
					<>
						{kind}
						{" · "}
						<Chip
							variant={voided ? "solid" : "flat"}
							color={voided ? "danger" : status.tone}
							size="sm"
						>
							{status.label}
						</Chip>
					</>
				}
				actions={
					<BackLink
						fallbackHref="/account/orders"
						className={buttonClasses({
							variant: "bordered",
							color: "default",
							size: "sm",
						})}
					>
						{T.back}
					</BackLink>
				}
			>
				<div className="flex max-w-[52rem] flex-col gap-[18px]">
					{voided ? (
						// 作廢是已成事實不是待處理的錯誤 ⇒ `status` 而不是 `alert`。
						<section
							className="rounded-hkui-md border border-danger-200 bg-danger-50 p-4 text-sm text-danger-700"
							role="status"
						>
							<strong className="block text-base">{T.voidedTitle}</strong>
							<p className="m-0 mt-1 leading-relaxed">{T.voidedBody}</p>
						</section>
					) : null}

					<Card title={T.cardDocument}>
						<dl className={DL_CLASS}>
							<dt className={DT_CLASS}>{T.fieldKind}</dt>
							<dd className={DD_CLASS}>{kind}</dd>
							{summary.invoiceNumber ? (
								<>
									<dt className={DT_CLASS}>{T.fieldInvoiceNo}</dt>
									<dd className={`${DD_CLASS} tabular-nums font-semibold`}>
										{summary.invoiceNumber}
									</dd>
								</>
							) : null}
							{summary.issuedAt ? (
								<>
									<dt className={DT_CLASS}>{T.fieldIssuedAt}</dt>
									<dd className={`${DD_CLASS} tabular-nums`}>
										{stamp(summary.issuedAt)}
									</dd>
								</>
							) : null}
							{detail.issuerName ? (
								<>
									{/* label 是「開立人」不是「賣家」：PLATFORM_RECEIPT 模式下
									    開立人是平台，寫「賣家」會在收據情境下說謊。 */}
									<dt className={DT_CLASS}>{T.fieldIssuer}</dt>
									<dd className={DD_CLASS}>{detail.issuerName}</dd>
								</>
							) : null}
							{detail.issuerTaxId ? (
								<>
									<dt className={DT_CLASS}>
										{summary.documentKind === "RECEIPT"
											? T.fieldIssuerTaxStatus
											: T.fieldIssuerTaxId}
									</dt>
									<dd className={`${DD_CLASS} tabular-nums`}>
										{detail.issuerTaxId}
									</dd>
								</>
							) : null}
							{detail.recipientName ? (
								<>
									<dt className={DT_CLASS}>{T.fieldRecipient}</dt>
									<dd className={DD_CLASS}>{detail.recipientName}</dd>
								</>
							) : null}
							{/*
							 * 🔴 **不能用純真值判斷**：B2C 的 `recipientTaxId` 是 DB CHECK
							 * 強制的哨兵值 `"0000000000"`，它是非空字串 ⇒ truthy ⇒ 十個零
							 * 會被當成統編印給消費者看（#3012）。判準見 `hasRecipientTaxId`。
							 */}
							{hasRecipientTaxId(
								summary.documentKind,
								detail.recipientTaxId,
							) ? (
								<>
									<dt className={DT_CLASS}>{T.fieldRecipientTaxId}</dt>
									<dd className={`${DD_CLASS} tabular-nums`}>
										{detail.recipientTaxId}
									</dd>
								</>
							) : null}
							{/* backend#3084：carrierSummary 在沒有載具時裝的是**買受人統編**，
							    而這一頁上方已經有專屬的「買受人統編」列 ⇒ 那種情況不渲染，
							    否則同一頁會出現兩列同值（dev 實測：兩列都是 22223223）。 */}
							{carrierRowKindOf(summary.carrierSummary, summary.carrierType) !==
							"recipientTaxId" ? (
								<>
									<dt className={DT_CLASS}>{T.fieldCarrier}</dt>
									<dd className={DD_CLASS}>
										{summary.carrierSummary || INVOICE_ORDER_TEXT.noCarrier}
									</dd>
								</>
							) : null}
						</dl>
					</Card>

					<Card title={T.cardAmount}>
						<dl className={DL_CLASS}>
							<dt className={DT_CLASS}>
								{summary.documentKind === "B2B"
									? T.fieldSalesExTax
									: T.fieldSalesInclTax}
							</dt>
							<dd className={`${DD_CLASS} tabular-nums`}>
								{money(detail.salesAmount)}
							</dd>
							<dt className={DT_CLASS}>{T.fieldTax}</dt>
							<dd className={`${DD_CLASS} tabular-nums`}>
								{money(detail.taxAmount)}
								{taxNote ? (
									<span className="ml-2 text-xs text-hk-muted">{taxNote}</span>
								) : null}
							</dd>
							<dt className={`${DT_CLASS} font-semibold`}>{T.fieldTotal}</dt>
							<dd className={`${DD_CLASS} text-base font-bold tabular-nums`}>
								{money(detail.totalAmount)}
							</dd>
						</dl>

						{detail.lines.length > 0 ? (
							// `<ul>` 不是 `<table>`：repo 查無 Table primitive，為一頁新造一個
							// 表格語彙的代價比這裡的可讀性損失大。
							<ul className="mt-4 list-none border-t border-divider p-0">
								{detail.lines.map((line) => (
									<LineRow
										key={`${line.lineType}-${line.name}-${line.netAmount}`}
										line={line}
									/>
								))}
							</ul>
						) : null}

						<p
							className="m-0 mt-4 rounded-hkui-md border border-info-200 bg-info-50 p-3 text-xs leading-relaxed text-info-700"
							role="status"
						>
							{INVOICE_ORDER_TEXT.shippingFeeNotice}
						</p>

						{/*
						 * 只有三聯式需要這句：它的明細（含稅）與銷售額（未稅）加起來對不上，
						 * 而那個落差看起來就像「稅是另外加的」。見 i18n 的 taxInclusiveNotice。
						 */}
						{summary.documentKind === "B2B" ? (
							<p
								className="m-0 mt-2 rounded-hkui-md border border-info-200 bg-info-50 p-3 text-xs leading-relaxed text-info-700"
								role="status"
							>
								{INVOICE_ORDER_TEXT.taxInclusiveNotice}
							</p>
						) : null}
					</Card>

					<Card title={T.cardTimeline}>
						{detail.timeline.length > 0 ? (
							<DocumentTimeline items={detail.timeline} />
						) : (
							<p className="m-0 text-sm text-hk-muted">{T.emptyTimeline}</p>
						)}
						{summary.relatedDocumentId ? (
							<p className="m-0 mt-4 text-sm">
								<Link
									href={`/account/invoices/${encodeURIComponent(summary.relatedDocumentId)}`}
									className="underline underline-offset-2"
								>
									{T.relatedDocument}
								</Link>
							</p>
						) : null}
					</Card>

					{canResendInvoiceNotification(summary.status) ? (
						<ResendNotificationCard documentId={summary.documentId} />
					) : null}

					{!summary.fileAvailable ? (
						<p className="m-0 text-sm text-hk-muted">
							{invoiceFileUnavailableHint(summary.fileUnavailableReason)}
						</p>
					) : null}
				</div>
			</SellerPageShell>
		</main>
	);
}

/**
 * 端點 71｜重寄通知。
 *
 * 🔴 三個結果分開講：**已寄出**／**剛剛才寄過（9352）**／**失敗**。把 `9352` 併進
 * 失敗會讓買家以為系統壞了，然後去開一張「沒收到發票」的申訴——而重寄鍵存在的
 * 目的正是讓他不必開那張單。
 *
 * ⚠️ 送出中把鍵鎖住（`isLoading` 已含 `disabled`），避免連點把 10 分鐘的窗口
 * 用在一次操作上。
 */
function ResendNotificationCard({ documentId }: { documentId: string }) {
	const { resend, sending, result, throttled, error } =
		useResendInvoiceNotification(documentId);

	return (
		<Card title={T.cardResend}>
			<p className="m-0 text-sm text-hk-muted">{T.resendBody}</p>
			<div className="mt-3">
				<Button
					variant="bordered"
					color="default"
					size="sm"
					isLoading={sending}
					loadingLabel={T.resendSending}
					onClick={() => resend()}
				>
					{T.resendAction}
				</Button>
			</div>
			{result?.resent ? (
				<p role="status" className="m-0 mt-3 text-sm text-success-700">
					{result.toMasked
						? `已重新寄出到 ${result.toMasked}。`
						: T.resendSentFallback}
				</p>
			) : null}
			{throttled ? (
				<p role="status" className="m-0 mt-3 text-sm text-warning-700">
					{T.resendThrottled}
				</p>
			) : null}
			{error ? (
				<p role="alert" className="m-0 mt-3 text-sm text-danger-700">
					{T.resendFailed}
				</p>
			) : null}
		</Card>
	);
}
