"use client";

import {
	BuyerInvoiceSkeletonCards,
	BuyerInvoiceSkeletonRows,
} from "../components/BuyerInvoiceListSkeleton";

/**
 * B09｜買家發票中心（列表，58A §B09）。整頁只打端點 68 一支。
 *
 * 四件事值得先讀：
 *
 *   1. **「開立人」欄是硬需求，不是裝飾**。方向一（賣家→買家）顯示賣家名稱、
 *      方向三（平台→買家）顯示平台名稱。不得省略此欄、也不得用平台名稱統一
 *      填充——付款商、平台與發票開立人是三件事，混在一起正是這一頁要消滅的
 *      混淆（58A §B09）。
 *   2. **發票號碼完整顯示，不遮罩**（58 §8.1.3(b) O 類與裁決 1）。它是買家自己
 *      的憑證號，遮掉之後這一頁就沒有可以拿去對帳的東西了。
 *   3. **致命錯誤態判 `!result` 而不是 `error || !result`**：背景 refetch 失敗時
 *      保留最後一份好資料，只有真的沒東西可顯示才換成錯誤畫面（同 B05）。
 *   4. **前端不自行過濾**：端點 68 的授權述詞已經只回自己是買受人的文件，
 *      在這裡再加一層過濾只會漏資料，不會多擋掉任何東西。
 *
 * 🔴 **狀態用買家的折疊投影**（`foldInvoiceDocumentStatus`），與賣家面的 11 原值
 * 是兩張表。共用賣家那份會讓買家看到「開立失敗」，而法規要求 provider 異常時對
 * 買家只說「處理中」。
 *
 * 🔴 **篩選狀態放 URL query 不放 React state**：瀏覽器上一頁回得到原篩選，也讓
 * 這一頁可以被深連結（與 S06 同一個慣例）。
 *
 * ⚠️ **不能用 `seller-table` / `seller-orders__table-wrap` 那組 class**：
 * `styles/features/seller-center.css` 只在 `app/[locale]/(seller)` 與 `app/(delegated)` 的
 * layout 被 import，`/account/*` 底下根本沒載入它 ⇒ 那些 class 在這一頁是死的
 * （表格會變成沒有框線、沒有間距的裸 table，而且編譯、lint、測試全部沉默）。
 */

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Link from "@/components/shared/Link";
import Pagination from "@/components/shared/Pagination";
import { ScFilters, ScPill } from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Input } from "@/components/ui/Input";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { INVOICE_LIST_TEXT } from "@/features/invoices/i18n";
import {
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentKind,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useBuyerInvoiceDocuments } from "@/lib/hooks/useBuyerInvoiceDocuments";
import { usePathname } from "@/lib/intl/navigation";
import { BUYER_DOCUMENTS_PAGE_SIZE } from "@/lib/invoices/buyer-repo";
import { formatInvoiceAmount } from "@/lib/invoices/display";
import { foldInvoiceDocumentStatus } from "@/lib/invoices/documentStatus";
import type { BuyerInvoiceDocumentItem } from "@/lib/invoices/types";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

const T = INVOICE_LIST_TEXT;
const LIST_HREF = "/account/invoices";
const COLUMNS = 6;

/** `YYYY-MM-DD` 嚴格驗證。端點 68 對格式錯的日期回 400 ⇒ 不合法就**不發請求**。 */
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function isValidDate(raw: string): boolean {
	return raw === "" || DATE_RE.test(raw);
}

/** 三個 `InvoiceDocumentKind` 值 ＋ 全部。中文一律取自 CodeMeta（§0.4 ⑤）。 */
const KIND_FILTERS = [
	{ value: "", label: T.filterAll },
	{ value: InvoiceDocumentKind.B2C, label: "" },
	{ value: InvoiceDocumentKind.B2B, label: "" },
	{ value: InvoiceDocumentKind.RECEIPT, label: "" },
] as const;

/**
 * 買家投影與後端原值 1:1 的四個 `InvoiceDocumentStatus` ＋ 全部。中文一律取自
 * CodeMeta（§0.4 ⑤）。「處理中」與「已作廢」是折疊值，開不了 pill——判準與解封
 * 條件見 `features/invoices/i18n.ts` 的 `filterStatusLabel`。
 *
 * 🔴 **匯出是為了讓 `tests/buyer-invoice-status-filter-contract.test.ts` 守住
 * 「pill 標籤 ↔ 它篩出的集合」**：把任一顆換成折疊值（例如 `VOIDED`，其 label
 * 「已作廢」同時涵蓋 `SUPERSEDED`），lint / tsc / unit / architecture 四道全綠、
 * 零訊號，而買家點下去只會拿到其中一半的文件，另一半在畫面上寫著同樣四個字卻
 * 憑空消失。那道測試從 `foldInvoiceDocumentStatus` 推導 1:1 集合，不寫死碼值。
 */
export const STATUS_FILTERS = [
	{ value: "", label: T.filterAll },
	{ value: InvoiceDocumentStatus.ISSUED, label: "" },
	{ value: InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE, label: "" },
	{ value: InvoiceDocumentStatus.NOT_APPLICABLE, label: "" },
	{ value: InvoiceDocumentStatus.RESOLVED_EXTERNALLY, label: "" },
] as const;

/** 值域外的類型退回原字串而不是空白——買家截圖問客服時那串碼是唯一的線索。 */
function kindLabel(kind: string): string {
	if (!kind) return "—";
	return INVOICE_DOCUMENT_KIND_LABEL[kind] ?? kind;
}

/** 同 `kindLabel`：後端先上線一個前端還不知道的狀態時，退回原碼而不是空白 pill。 */
function statusLabel(status: string): string {
	if (!status) return "—";
	return INVOICE_DOCUMENT_STATUS_LABEL[status] ?? status;
}

function detailHref(documentId: string): string {
	return `${LIST_HREF}/${encodeURIComponent(documentId)}`;
}

export default function BuyerInvoiceListClient() {
	// 走 observed router 而不是裸 useRouter：`tests/navigation-route-contract.test.ts`
	// 要求所有程式化導航都經過這一層（導航看門狗靠它辨識 stuck navigation）。
	const router = useObservedRouter("buyer_invoice_documents");
	const pathname = usePathname() ?? LIST_HREF;
	const searchParams = useSearchParams();

	const dateFrom = searchParams?.get("date_from") ?? "";
	const dateTo = searchParams?.get("date_to") ?? "";
	const documentKind = searchParams?.get("document_kind") ?? "";
	const documentStatus = searchParams?.get("status") ?? "";
	const page = Math.max(1, Number(searchParams?.get("page") ?? "1") || 1);

	const datesValid = isValidDate(dateFrom) && isValidDate(dateTo);

	const setParams = React.useCallback(
		(patch: Record<string, string | null>) => {
			const next = new URLSearchParams(searchParams?.toString() ?? "");
			for (const [key, value] of Object.entries(patch)) {
				if (value === null || value === "") next.delete(key);
				else next.set(key, value);
			}
			// 改任一條件時 `page` 一律重設為 1，否則會停在一個不存在的頁碼。
			if (!("page" in patch)) next.delete("page");
			const qs = next.toString();
			router.replace(qs ? `${pathname}?${qs}` : pathname);
		},
		[pathname, router, searchParams],
	);

	const queryParams = React.useMemo(
		() => ({
			dateFrom,
			dateTo,
			documentKind,
			status: documentStatus,
			page,
			pageSize: BUYER_DOCUMENTS_PAGE_SIZE,
		}),
		[dateFrom, dateTo, documentKind, documentStatus, page],
	);

	const {
		page: result,
		loading,
		refresh,
	} = useBuyerInvoiceDocuments(queryParams, { enabled: datesValid });

	const records = result?.records ?? [];
	// 🔴 `total` 是總筆數不是頁數：直接餵給 Pagination 會得到一個長度等於筆數的頁碼列。
	const totalPages = result
		? Math.max(1, Math.ceil(result.total / BUYER_DOCUMENTS_PAGE_SIZE))
		: 1;
	const hasFilters = Boolean(
		dateFrom || dateTo || documentKind || documentStatus,
	);
	// 只有真的沒東西可顯示才換錯誤畫面；背景 refetch 失敗時保留最後一份好資料。
	const failed = datesValid && !loading && !result;

	return (
		<main>
			<SellerPageShell
				title={T.pageTitle}
				eyebrow={T.eyebrow}
				subtitle={T.subtitle(result?.total ?? 0)}
				toolbar={
					<div className="flex flex-col gap-[var(--hkui-space-3)]">
						<ScFilters aria-label={T.filterKindLabel}>
							{KIND_FILTERS.map((item) => (
								<ScPill
									key={item.value || "all-kind"}
									as="button"
									active={documentKind === item.value}
									aria-selected={documentKind === item.value}
									onClick={() =>
										setParams({ document_kind: item.value || null })
									}
								>
									{item.label || kindLabel(item.value)}
								</ScPill>
							))}
						</ScFilters>
						<ScFilters aria-label={T.filterStatusLabel}>
							{STATUS_FILTERS.map((item) => (
								<ScPill
									key={item.value || "all-status"}
									as="button"
									active={documentStatus === item.value}
									aria-selected={documentStatus === item.value}
									onClick={() => setParams({ status: item.value || null })}
								>
									{item.label || statusLabel(item.value)}
								</ScPill>
							))}
						</ScFilters>
						<div className="flex flex-wrap items-start gap-[var(--hkui-space-3)]">
							<Input
								className="max-w-[13rem]"
								size="lg"
								type="text"
								label={T.dateFrom}
								placeholder={T.datePlaceholder}
								value={dateFrom}
								isInvalid={!isValidDate(dateFrom)}
								errorMessage={isValidDate(dateFrom) ? undefined : T.dateInvalid}
								onChange={(event) =>
									setParams({ date_from: event.target.value || null })
								}
							/>
							<Input
								className="max-w-[13rem]"
								size="lg"
								type="text"
								label={T.dateTo}
								placeholder={T.datePlaceholder}
								value={dateTo}
								isInvalid={!isValidDate(dateTo)}
								errorMessage={isValidDate(dateTo) ? undefined : T.dateInvalid}
								onChange={(event) =>
									setParams({ date_to: event.target.value || null })
								}
							/>
						</div>
					</div>
				}
			>
				<section className="seller-card" aria-busy={loading}>
					{loading ? (
						<span className="sr-only" role="status">
							{T.loading}
						</span>
					) : null}
					{/*
					 * ≥768px 走表格、<768px 走卡片：同一份資料渲染兩份 DOM ＋ Tailwind
					 * 斷點 `hidden`。`display:none` 讓被隱藏那份不進 accessibility tree，
					 * 螢幕閱讀器只會讀到一組列表（§B09 Responsive）。
					 */}
					<div className="hidden overflow-x-auto md:block">
						<table className="w-full border-collapse text-sm">
							<caption className="sr-only">{T.tableCaption}</caption>
							<thead>
								<tr className="border-b border-divider text-left">
									<th className={TH_CLASS} scope="col">
										{T.colIssuedAt}
									</th>
									<th className={TH_CLASS} scope="col">
										{T.colInvoiceNo}
									</th>
									<th className={TH_CLASS} scope="col">
										{T.colIssuer}
									</th>
									<th className={TH_CLASS} scope="col">
										{T.colKind}
									</th>
									<th className={`${TH_CLASS} text-right`} scope="col">
										{T.colAmount}
									</th>
									<th className={TH_CLASS} scope="col">
										{T.colStatus}
									</th>
								</tr>
							</thead>
							<tbody>
								{!datesValid ? (
									<tr>
										<td colSpan={COLUMNS}>
											<p
												className="m-0 py-6 text-center text-sm text-danger-700"
												role="alert"
											>
												{T.dateInvalid}
											</p>
										</td>
									</tr>
								) : loading ? (
									<BuyerInvoiceSkeletonRows />
								) : failed ? (
									<tr>
										<td colSpan={COLUMNS}>
											<ErrorBody onRetry={() => void refresh()} />
										</td>
									</tr>
								) : records.length === 0 ? (
									<tr>
										<td colSpan={COLUMNS}>
											<EmptyBody
												hasFilters={hasFilters}
												onClear={() => router.replace(pathname)}
											/>
										</td>
									</tr>
								) : (
									records.map((item) => (
										<DocumentRow item={item} key={item.summary.documentId} />
									))
								)}
							</tbody>
						</table>
					</div>

					<ul className="m-0 flex list-none flex-col gap-[var(--hkui-space-3)] p-0 md:hidden">
						{!datesValid ? (
							<li className="p-4 text-sm text-danger-700" role="alert">
								{T.dateInvalid}
							</li>
						) : loading ? (
							<BuyerInvoiceSkeletonCards />
						) : failed ? (
							<li>
								<ErrorBody onRetry={() => void refresh()} />
							</li>
						) : records.length === 0 ? (
							<li>
								<EmptyBody
									hasFilters={hasFilters}
									onClear={() => router.replace(pathname)}
								/>
							</li>
						) : (
							records.map((item) => (
								<DocumentCard item={item} key={item.summary.documentId} />
							))
						)}
					</ul>
				</section>

				<Pagination
					current={page}
					onPageChange={(next) => setParams({ page: String(next) })}
					total={totalPages}
				/>
			</SellerPageShell>
		</main>
	);
}

const TH_CLASS =
	"px-3 py-2.5 text-xs font-bold tracking-wide text-foreground-soft whitespace-nowrap";
const TD_CLASS = "px-3 py-3 align-middle text-hk-body";

/** 空值一律 `—`，不猜也不留空白格（空白格看起來像版面壞了）。 */
function dash(value: string): string {
	return value || "—";
}

function ErrorBody({ onRetry }: { onRetry: () => void }) {
	return (
		<div
			className="flex flex-col items-center gap-3 py-8 text-center"
			role="alert"
		>
			<p className="m-0 text-sm text-foreground-soft">{T.errorBody}</p>
			<Button onClick={onRetry} size="lg" variant="bordered">
				{T.retry}
			</Button>
		</div>
	);
}

function EmptyBody({
	hasFilters,
	onClear,
}: {
	hasFilters: boolean;
	onClear: () => void;
}) {
	return (
		<div className="flex flex-col items-center gap-3 py-8 text-center">
			<AccountEmptyStateIcon variant="orders" />
			<strong className="text-base font-semibold">
				{hasFilters ? T.filteredEmptyTitle : T.emptyTitle}
			</strong>
			<p className="m-0 text-sm text-foreground-soft">
				{hasFilters ? T.filteredEmptyBody : T.emptyBody}
			</p>
			{hasFilters ? (
				<Button onClick={onClear} size="lg" variant="bordered">
					{T.clearFilters}
				</Button>
			) : null}
		</div>
	);
}

/**
 * 一列 = 一份文件。整列導向 B05 靠**發票號碼那一格的錨點**，不是 `<tr onClick>`：
 * 後者不是可聚焦元素、鍵盤與螢幕閱讀器都到不了，而這一頁唯一的動作就是「點進去」。
 * 發票號碼為空（尚未開立）時錨點文字換成「查看詳情」，連結本身不消失。
 */
function DocumentRow({ item }: { item: BuyerInvoiceDocumentItem }) {
	const { summary } = item;
	const status = foldInvoiceDocumentStatus(summary.status);
	return (
		<tr className="border-b border-divider last:border-b-0">
			<td className={`${TD_CLASS} tabular-nums whitespace-nowrap`}>
				{dash(formatTaipeiDate(summary.issuedAt))}
			</td>
			<td className={TD_CLASS}>
				{/* 🔴 完整顯示，不遮罩（58 §8.1.3(b) O 類與裁決 1）。 */}
				<Link
					className="font-semibold tabular-nums underline"
					href={detailHref(summary.documentId)}
				>
					{summary.invoiceNumber || T.viewDetail}
				</Link>
			</td>
			<td className={TD_CLASS}>{dash(item.issuerName)}</td>
			<td className={TD_CLASS}>
				<Chip size="sm" variant="flat">
					{kindLabel(summary.documentKind)}
				</Chip>
			</td>
			<td className={`${TD_CLASS} text-right font-semibold tabular-nums`}>
				{dash(formatInvoiceAmount(summary.totalAmount))}
			</td>
			<td className={TD_CLASS}>
				<Chip color={status.tone} size="sm" variant="flat">
					{status.label}
				</Chip>
			</td>
		</tr>
	);
}

/** <768px：開立日期＋狀態同一行、發票號碼為卡標題、開立人與金額各一行（§B09）。 */
function DocumentCard({ item }: { item: BuyerInvoiceDocumentItem }) {
	const { summary } = item;
	const status = foldInvoiceDocumentStatus(summary.status);
	return (
		<li className="border-b border-divider p-4 last:border-b-0">
			<div className="flex flex-col gap-2">
				<span className="flex items-center justify-between gap-2">
					<span className="text-xs text-foreground-soft tabular-nums">
						{dash(formatTaipeiDate(summary.issuedAt))}
					</span>
					<Chip color={status.tone} size="sm" variant="flat">
						{status.label}
					</Chip>
				</span>
				<Link
					className="text-base font-semibold tabular-nums underline"
					href={detailHref(summary.documentId)}
				>
					{summary.invoiceNumber || T.viewDetail}
				</Link>
				<span className="flex items-center justify-between gap-2 text-sm">
					<span className="text-hk-muted">{T.colIssuer}</span>
					<span className="min-w-0 truncate text-hk-body">
						{dash(item.issuerName)}
					</span>
				</span>
				<span className="flex items-center justify-between gap-2 text-sm">
					<Chip size="sm" variant="flat">
						{kindLabel(summary.documentKind)}
					</Chip>
					<span className="font-semibold text-hk-body tabular-nums">
						{dash(formatInvoiceAmount(summary.totalAmount))}
					</span>
				</span>
			</div>
		</li>
	);
}
