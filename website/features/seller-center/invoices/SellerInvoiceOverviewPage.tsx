"use client";

/**
 * S01｜發票與稅籍總覽（58A §S01）。端點 16 是本頁唯一的資料源。
 *
 * 權限：`CapInvoiceManage`／停權 **BLOCK**（不進白名單）——`useSellerInvoiceOverview`
 * 在查詢發出之前就 gate（§B-2.3 (3)）。本頁**沒有任何 mutation**，所以沒有冪等鍵、
 * 沒有二次確認。
 *
 * ⚠️ S03（AMEGO 串接）已於 2026-09-01 接上，卡 2 的 CTA 因此打開。
 * S04（自行開票責任條款）同批落地。S05（收據代製授權）走端點 28／29
 * （`GetSellerReceiptAuthorization`／`ConfirmSellerReceiptAuthorization`），入口是
 * 連結；誰看得到它由 `showsReceiptAuth` 決定。
 */

import * as React from "react";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import {
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_FULFILLMENT_MODE_LABEL,
	INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
	InvoiceFulfillmentMode,
	SellerTaxReviewStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useSellerInvoiceOverview } from "@/lib/hooks/useSellerInvoices";
import { formatInvoiceAmount } from "@/lib/invoices/display";
import { hasIntakeLimit } from "@/lib/invoices/seller-types";
import { SellerEmptyState } from "../components/SellerEmptyState";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "../components/SellerMobileDisclosureList";
import { SellerInvoiceOverviewSkeleton } from "../components/SellerPageSkeletons";
import { TaxStatusCard } from "./components/TaxStatusCard";
import { invoiceDocumentDisplay } from "./invoiceDocumentDisplay";
import { invoiceTaxStatusBanner } from "./invoiceTaxStatusBanner";

const DOCUMENTS_HREF = "/seller-center/invoices/documents";
const AMEGO_HREF = "/seller-center/invoices/amego";
const RECEIPT_AUTH_HREF = "/seller-center/invoices/receipt-authorization";

/**
 * 端點 29 的 allowed-from，逐值抄自 invoice-service 的
 * `receiptAuthEligibleStatuses`（`internal/usecase/receipt_authorization.go`）。
 *
 * 🔴 兩邊漂移的後果不對稱、而且都難查：這裡多一個值 ⇒ 賣家看得到入口、點進去撞
 * gate；少一個值 ⇒ 那個狀態的賣家永遠找不到入口，且不會有任何錯誤指向這裡。
 */
const RECEIPT_AUTH_ENTRY_STATUSES = new Set<string>([
	SellerTaxReviewStatus.SUBMITTED,
	SellerTaxReviewStatus.UNDER_REVIEW,
	SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
	SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
]);

/**
 * S05 的入口要不要出現。
 *
 * 端點 16 沒有直接的 `eligible` 旗標，所以從兩個既有欄位推：稅務身分落在端點 29
 * 的 allowed-from（審核進行中的四個狀態 ＋ 已核定免用統一發票），或目前就走平台
 * 代製收據（`invoiceMode`）。
 *
 * 🔴 為什麼審核中就要開（DEC-20260905-015836 選 A）：核定完成到賣家實際簽授權之間
 * 的每一筆已付訂單，收據都會卡成待人工處理。讓他在等審核時就先簽，那個窗口才會
 * 消失——「等核定了再說」不是保守，是把成本轉嫁給客服。
 *
 * ⚠️ 這是**入口的可見性**，不是授權資格本身——真正的判準在端點 28 的 `eligible`，
 * S05 頁面自己會擋。所以這裡逐值對齊後端，不自己另外放寬或收緊。
 */
export function showsReceiptAuth(overview: {
	taxReviewStatus: string;
	invoiceMode: string;
}): boolean {
	return (
		RECEIPT_AUTH_ENTRY_STATUSES.has(overview.taxReviewStatus) ||
		overview.invoiceMode === InvoiceFulfillmentMode.PLATFORM_RECEIPT
	);
}
const APPEALS_HREF = "/seller-center/invoices/appeals";

interface RecentInvoiceDocument {
	documentId: string;
	documentKind: string;
	status: string;
	invoiceNumber: string;
	issuedAt: string;
	totalAmount: string;
}

function recentDocumentKey(document: RecentInvoiceDocument): string {
	return document.documentId;
}

export function SellerInvoiceOverviewPage() {
	const { isSuspended } = useSellerIdentity();
	const { overview, loading, error, refresh } = useSellerInvoiceOverview();

	const banner = React.useMemo(
		() =>
			invoiceTaxStatusBanner({
				reviewStatus: overview?.taxReviewStatus ?? "",
				selfIssueEffective: overview?.selfIssueEffective,
				withNavigationCta: true,
			}),
		[overview?.taxReviewStatus, overview?.selfIssueEffective],
	);

	return (
		<SellerPageShell
			title="發票與稅籍"
			subtitle="查看接單能力、稅務審核期限、目前開票方式與待處理申訴。"
			actions={
				<Button size="lg" variant="bordered" onClick={() => void refresh()}>
					重新整理
				</Button>
			}
		>
			{isSuspended ? (
				<SuspendedNotice />
			) : loading ? (
				<SellerInvoiceOverviewSkeleton />
			) : error || !overview ? (
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						發票資料載入失敗，請稍後再試。
					</p>
					<Button size="lg" variant="bordered" onClick={() => void refresh()}>
						重試
					</Button>
				</section>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-4)]">
					<TaxStatusCard
						banner={banner}
						deadlineAt={overview.deadlineAt}
						selfIssueEffective={overview.selfIssueEffective}
					/>

					{hasIntakeLimit(overview.intakeLimit) ? (
						<IntakeLimitBanner limit={overview.intakeLimit} />
					) : null}

					<div className="grid grid-cols-1 gap-[var(--hkui-space-4)] md:grid-cols-3">
						<StatusCard
							label="目前開票方式"
							value={
								INVOICE_FULFILLMENT_MODE_LABEL[overview.invoiceMode] ??
								overview.invoiceMode ??
								"尚未設定"
							}
							// S05 入口。審核進行中的賣家也看得到（判準見 `showsReceiptAuth`）：
							// 核定之前就先簽，核定之後才不會有一段沒授權的空窗。
							href={showsReceiptAuth(overview) ? RECEIPT_AUTH_HREF : undefined}
							hrefLabel={showsReceiptAuth(overview) ? "收據授權" : undefined}
						/>
						<StatusCard
							label="AMEGO 串接"
							value={
								// 上線當天全體賣家是 NOT_STARTED，文案必須是「尚未設定」而
								// 不是「設定錯誤」（§S01 的 DTO 欄位表）。
								INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL[
									overview.providerStatus
								] ?? "尚未設定"
							}
							// S03 已上線（端點 23–27 於 2026-09-01 重查已在 origin/dev）。
							// 🔴 這是整條發票鏈唯一的入口：賣家沒串 AMEGO ⇒ 訂單一張發票
							// 都開不出來，所以 CTA 一律給，不依 providerStatus 隱藏。
							href={AMEGO_HREF}
							hrefLabel="前往設定"
						/>
						<StatusCard
							label="買家申訴"
							value={`${overview.openComplaintCount || "0"} 件`}
							href={APPEALS_HREF}
							hrefLabel="查看申訴"
							note={
								Number(overview.overdueComplaintCount) > 0
									? `其中 ${overview.overdueComplaintCount} 件已逾期`
									: undefined
							}
							noteTone={
								Number(overview.overdueComplaintCount) > 0
									? "danger"
									: undefined
							}
						/>
					</div>

					<RecentDocumentsCard
						documents={overview.recentDocuments}
						pendingCount={overview.pendingDocumentCount}
					/>
				</div>
			)}
		</SellerPageShell>
	);
}

function SuspendedNotice() {
	return (
		<section role="status" className="seller-card flex flex-col gap-2 p-6">
			<strong className="text-base font-bold">
				帳號停權期間無法查看發票總覽。
			</strong>
			<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
				已開立的發票仍會正常寄送給買家。停權期間仍可前往
				<Link
					href="/seller-center/invoices/tax-status"
					className="mx-1 underline"
				>
					稅務身分
				</Link>
				補齊資料。
			</p>
		</section>
	);
}

function IntakeLimitBanner({
	limit,
}: {
	limit: {
		orderCountUsed: string;
		orderCountLimit: string;
		amountUsed: string;
		amountLimit: string;
		blocked: boolean;
	};
}) {
	return (
		<section
			role={limit.blocked ? "alert" : "status"}
			className="flex flex-col gap-1 rounded-hkui-lg border border-[var(--hkui-warning-200)] bg-warning-50 p-[var(--hkui-space-4)] text-warning-700"
		>
			<strong className="text-sm font-bold">
				{`稅務審核完成前，接單有上限：已接 ${limit.orderCountUsed}／${limit.orderCountLimit} 筆、${limit.amountUsed}／${limit.amountLimit} 元，先到者停止接受新訂單。`}
			</strong>
			{limit.blocked ? (
				<p className="m-0 text-[0.8125rem]">
					已達上限，目前無法接受新訂單。完成稅務身分審核後自動恢復。
				</p>
			) : null}
		</section>
	);
}

function StatusCard({
	label,
	value,
	note,
	noteTone,
	href,
	hrefLabel,
}: {
	label: string;
	value: string;
	note?: string;
	noteTone?: "danger";
	/** 有落地頁、且這位賣家真的用得到時才給連結。渲染一顆點下去撞 gate 或 404 的
	 *  連結，比維持唯讀狀態列更糟。 */
	href?: string;
	hrefLabel?: string;
}) {
	return (
		<section className="seller-card flex flex-col gap-1 p-[var(--hkui-space-4)]">
			<span className="text-[0.75rem] font-bold tracking-[0.06em] text-[var(--hkui-foreground-soft)]">
				{label}
			</span>
			<strong className="text-base font-bold text-[var(--hkui-foreground)]">
				{value}
			</strong>
			{note ? (
				<span
					className={
						noteTone === "danger"
							? "text-[0.75rem] text-danger-700"
							: "text-[0.75rem] text-[var(--hkui-foreground-soft)]"
					}
				>
					{note}
				</span>
			) : null}
			{href ? (
				<Link
					href={href}
					className="inline-flex min-h-11 items-center self-start text-sm underline"
				>
					{hrefLabel ?? "查看"}
				</Link>
			) : null}
		</section>
	);
}

function RecentDocumentsCard({
	documents,
	pendingCount,
}: {
	documents: readonly RecentInvoiceDocument[];
	pendingCount: string;
}) {
	return (
		<section className="seller-card">
			<header className="seller-card__head flex items-center justify-between gap-3">
				<h2>最近開立的發票與收據</h2>
				<span className="flex items-center gap-3">
					{Number(pendingCount) > 0 ? (
						<small className="text-[var(--hkui-foreground-soft)]">
							{pendingCount} 筆處理中
						</small>
					) : null}
					<Link
						href={DOCUMENTS_HREF}
						className="inline-flex min-h-11 items-center text-sm underline"
					>
						查看全部
					</Link>
				</span>
			</header>
			<div
				className={`seller-orders__table-wrap${documents.length > 0 ? " seller-mobile-disclosure-desktop" : ""}`}
			>
				<table className="seller-table" data-mobile-layout="cards">
					<caption className="sr-only">最近開立的發票與收據</caption>
					<thead>
						<tr>
							<th scope="col">開立時間</th>
							<th scope="col">發票號碼</th>
							<th scope="col">類型</th>
							<th scope="col">金額</th>
							<th scope="col">發票狀態</th>
							<th scope="col">操作</th>
						</tr>
					</thead>
					<tbody>
						{documents.length === 0 ? (
							<tr>
								<td colSpan={6}>
									<SellerEmptyState
										variant="invoices"
										seamless
										title="目前沒有待處理的發票或收據"
										description="訂單付款成功後，發票會自動開立並顯示在這裡。"
									/>
								</td>
							</tr>
						) : (
							documents.map((doc) => {
								const display = invoiceDocumentDisplay(doc.status);
								return (
									<tr key={doc.documentId}>
										<td data-label="開立時間">
											{formatTaipeiDateTime(doc.issuedAt) || "—"}
										</td>
										{/* 遮罩值用 <span> 而非 <code>，避免螢幕閱讀器逐字讀 `*`。 */}
										<td data-label="發票號碼">
											<span>{doc.invoiceNumber || "—"}</span>
										</td>
										<td data-label="類型">
											{INVOICE_DOCUMENT_KIND_LABEL[doc.documentKind] ??
												doc.documentKind}
										</td>
										<td className="seller-table__amount" data-label="金額">
											{formatInvoiceAmount(doc.totalAmount)}
										</td>
										<td data-label="發票狀態">
											<Chip variant="flat" color={display.color} size="sm">
												{display.label}
											</Chip>
										</td>
										<td data-label="操作">
											{/*
											 * `recentDocuments[]` 用的是 `InvoiceDocumentSummary`，
											 * **沒有 `orderNo`** ⇒ 只能導到 S06 並帶 `?document=`，
											 * 不得從 `documentId` 反推訂單編號（§S01 DTO 表）。
											 */}
											<Link
												href={`${DOCUMENTS_HREF}?document=${encodeURIComponent(doc.documentId)}`}
												className="text-sm underline"
											>
												查看文件
											</Link>
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>
			{documents.length > 0 ? (
				<div className="p-3">
					<SellerMobileDisclosureList
						items={documents}
						getKey={recentDocumentKey}
						ariaLabel="最近開立的發票與收據"
						getSummary={(document) => {
							const display = invoiceDocumentDisplay(document.status);
							return {
								label: document.invoiceNumber || "發票文件",
								leading: <SellerMobileDisclosureIcon name="document" />,
								title:
									document.invoiceNumber ||
									INVOICE_DOCUMENT_KIND_LABEL[document.documentKind] ||
									"發票文件",
								meta: formatInvoiceAmount(document.totalAmount),
								status: (
									<Chip variant="flat" color={display.color} size="sm">
										{display.label}
									</Chip>
								),
							};
						}}
						renderDetails={(document) => (
							<>
								<div className="seller-mobile-disclosure__body">
									<div className="seller-mobile-disclosure__detail-row">
										<span>開立時間</span>
										<strong>
											{formatTaipeiDateTime(document.issuedAt) || "—"}
										</strong>
									</div>
									<div className="seller-mobile-disclosure__detail-row">
										<span>類型</span>
										<strong>
											{INVOICE_DOCUMENT_KIND_LABEL[document.documentKind] ??
												document.documentKind}
										</strong>
									</div>
									<div className="seller-mobile-disclosure__detail-row">
										<span>金額</span>
										<strong>{formatInvoiceAmount(document.totalAmount)}</strong>
									</div>
								</div>
								<div className="seller-mobile-disclosure__actions">
									<Link
										href={`${DOCUMENTS_HREF}?document=${encodeURIComponent(document.documentId)}`}
										className="seller-orders__action"
										data-hkui-button
									>
										查看文件
									</Link>
								</div>
							</>
						)}
					/>
				</div>
			) : null}
		</section>
	);
}

export default SellerInvoiceOverviewPage;
