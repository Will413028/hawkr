"use client";

/**
 * S06｜發票與收據紀錄 ＋ S12｜賣家發票中心（58A §S06／§S12）。
 *
 * 本路由是**兩個分頁**：`?tab=issued`（銷項，端點 30，預設）／`?tab=received`
 * （進項，端點 72）。不新開路由——同一個心智概念分兩個資料集，用 tab 比兩條路由
 * 好找；為什麼不是「一個列表加篩選」見 `./invoiceDocumentsTabs.tsx` 的檔頭。
 * 進項分頁的實作在 `./SellerReceivedInvoicesPanel.tsx`。
 *
 * 以下說明只適用**銷項**分頁：端點 30 是它唯一的資料源，**沒有任何 mutation**。
 *
 * 權限：`CapInvoiceManage`／停權 **BLOCK**（不在白名單——「純唯讀查看也鎖」是
 * 58 §8.1.6 的明文裁決）。
 *
 * 🔴 **篩選狀態放 URL query 不放 React state**：讓 S01 與儀表板可深連結、瀏覽器上一頁
 * 回得到原篩選，也與 `SellerOrdersPage` 的 `?tab=` / `?focus=` 既有慣例一致。
 *
 * 🔴 **第 6、7 欄是兩個彼此獨立的狀態維度，必須是兩顆 Chip、不可合併**：後端明令
 * 禁止合成「綜合狀態」，合成之後前端就無法履行「出貨不受影響」這句話。
 *
 * ✅ **`paidAt` 與 `shipmentStatus` 已由後端填值**（backend#3019，2026-09-02）：
 * `paidAt` 取 `invoice_documents.paid_at`（開票當下落下的收款時點）；
 * `shipmentStatus` 由 invoice-service 逐頁向 shipping-service 批次查。
 *
 * 🔴 **`shipmentStatus` 的空字串只有在 `shipmentStatusDegraded === false` 時
 * 才等於「還沒出貨」。** 兩者塌在一起正是這一欄原本只敢畫 `—` 的理由：降級時每一列
 * 都會是空字串，而空白會被讀成「還沒出貨」——一句我們無從得知的話。
 * ⇒ 判定一律走 `shipmentStatusDisplay()`（`lib/invoices/display.ts`，有單元測試守著），
 * 不要在這個檔案裡自己寫 `if`。
 *
 * ⚠️ 值域是 `ShippingStatus` 的 value（數字字串）⇒ 直接畫上去賣家看到的是「210」。
 *
 * ⚠️ 2026-09-02 之前建立的文件 `paid_at` 是 NULL ⇒ 那些列的 `paidAt` 仍是 `—`。
 */

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Link from "@/components/shared/Link";
import Pagination from "@/components/shared/Pagination";
import { ScFilters, ScPill } from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Input } from "@/components/ui/Input";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	INVOICE_FULFILLMENT_MODE_LABEL,
	INVOICE_RECIPIENT_TYPE_LABEL,
	InvoiceDocumentStatus,
	InvoiceFulfillmentMode,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useSellerInvoiceDocuments,
	useSellerInvoiceOverview,
} from "@/lib/hooks/useSellerInvoices";
import { usePathname } from "@/lib/intl/navigation";
import {
	formatInvoiceAmount,
	shipmentStatusDisplay,
} from "@/lib/invoices/display";
import type { SellerInvoiceDocumentItem } from "@/lib/invoices/seller-types";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { SellerEmptyState } from "../../components/SellerEmptyState";
import { SellerInvoiceDocumentsSkeletonRows } from "../../components/SellerPageSkeletons";
import { invoiceDocumentDisplay } from "../invoiceDocumentDisplay";
import {
	documentsPageLabel,
	invoiceSectionLabel,
	isReceiptOnlySeller,
} from "../receiptFraming";
import {
	InvoiceDocumentsTabs,
	normalizeInvoiceDocumentsTab,
} from "./invoiceDocumentsTabs";
import { SellerReceivedInvoicesPanel } from "./SellerReceivedInvoicesPanel";

const OVERVIEW_HREF = "/seller-center/invoices";
const PAGE_SIZE = 20;
const COLUMNS = 8;

/** `YYYY-MM-DD` 嚴格驗證。後端對帶斜線／空字串的日期會回全部歷史 ⇒ 格式驗證要在
 *  最上游一次做對，不合法就**不發請求**。 */
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function isValidDate(raw: string): boolean {
	return raw === "" || DATE_RE.test(raw);
}

const MODE_FILTERS = [
	{ value: "", label: "全部" },
	{ value: InvoiceFulfillmentMode.AMEGO_AUTO, label: "" },
	{ value: InvoiceFulfillmentMode.SELLER_SELF_ISSUE, label: "" },
	{ value: InvoiceFulfillmentMode.PLATFORM_RECEIPT, label: "" },
	{ value: InvoiceFulfillmentMode.LEGACY_MANUAL, label: "" },
] as const;

const STATUS_FILTERS = [
	{ value: "", label: "全部" },
	{ value: InvoiceDocumentStatus.ISSUED, label: "" },
	{ value: InvoiceDocumentStatus.ISSUING, label: "" },
	{ value: InvoiceDocumentStatus.ISSUE_FAILED, label: "" },
	{ value: InvoiceDocumentStatus.AWAITING_MANUAL, label: "" },
	{ value: InvoiceDocumentStatus.VOIDED, label: "" },
] as const;

export function SellerInvoiceDocumentsPage() {
	// backend#3066 裁決 B：免用統一發票的賣家在這一頁**只會有收據**
	// ⇒ 叫它「發票與收據紀錄」是在承諾一個他永遠看不到的東西。
	// 端點 16 走 TanStack 快取（S01／側欄已查過同一把 key），不多打 API。
	const { overview: framingOverview } = useSellerInvoiceOverview();
	const receiptOnly = isReceiptOnlySeller(framingOverview);
	const recordsLabel = documentsPageLabel(receiptOnly);
	// 走 observed router 而不是裸 useRouter：`tests/navigation-route-contract.test.ts`
	// 要求所有程式化導航都經過這一層（導航看門狗靠它辨識 stuck navigation）。
	const router = useObservedRouter("seller_invoice_documents");
	const pathname = usePathname() ?? OVERVIEW_HREF;
	const searchParams = useSearchParams();
	const { isSuspended } = useSellerIdentity();

	const dateFrom = searchParams?.get("date_from") ?? "";
	const dateTo = searchParams?.get("date_to") ?? "";
	const mode = searchParams?.get("mode") ?? "";
	const status = searchParams?.get("status") ?? "";
	const abnormalOnly = searchParams?.get("abnormal_only") === "true";
	const orderNo = searchParams?.get("order_no") ?? "";
	const page = Math.max(1, Number(searchParams?.get("page") ?? "1") || 1);
	const tab = normalizeInvoiceDocumentsTab(searchParams?.get("tab") ?? "");
	/** 前端專用：捲到該列 ＋ 高亮。端點 30 沒有 `document_id` 精確查詢，
	 *  **不得自行拼一個後端不認的 query 參數**。 */
	const focusDocumentId = searchParams?.get("document") ?? "";

	const datesValid = isValidDate(dateFrom) && isValidDate(dateTo);

	const [orderNoDraft, setOrderNoDraft] = React.useState(orderNo);
	React.useEffect(() => setOrderNoDraft(orderNo), [orderNo]);

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
			mode,
			status,
			abnormalOnly,
			orderNo,
			page,
			pageSize: PAGE_SIZE,
		}),
		[dateFrom, dateTo, mode, status, abnormalOnly, orderNo, page],
	);

	const {
		page: result,
		loading,
		error,
		refresh,
		// 進項分頁時整支不發：兩個分頁是兩個資料集，切過去還打端點 30 只是白燒一次
		// 共用限流額度（§0.4 ④）。
	} = useSellerInvoiceDocuments(queryParams, {
		enabled: datesValid && tab === "issued",
	});

	const records = result?.records ?? [];
	const shipmentDegraded = result?.shipmentStatusDegraded ?? false;
	// 🔴 `total` 是總筆數不是頁數（§B-2.0 T25）：直接餵給 Pagination 會得到一個
	// 長度等於筆數的頁碼列。
	const totalPages = result
		? Math.max(1, Math.ceil(result.total / PAGE_SIZE))
		: 1;
	const hasFilters = Boolean(
		dateFrom || dateTo || mode || status || abnormalOnly || orderNo,
	);

	return (
		<SellerPageShell
			title={recordsLabel}
			eyebrow={invoiceSectionLabel(receiptOnly ? "RECEIPT" : "")}
			subtitle={
				tab === "received"
					? "平台開立給你的發票憑證，唯讀。"
					: "每筆訂單的發票或收據狀態；出貨狀態與發票狀態是分開的兩件事。"
			}
			actions={
				tab === "received" ? null : (
					<>
						<Input
							size="lg"
							aria-label="搜尋訂單編號"
							placeholder="輸入訂單編號"
							value={orderNoDraft}
							onChange={(event) => setOrderNoDraft(event.target.value)}
							onKeyDown={(event) => {
								if (event.key === "Enter") {
									setParams({ order_no: orderNoDraft.trim() || null });
								}
							}}
						/>
						<Button size="lg" variant="bordered" onClick={() => void refresh()}>
							重新整理
						</Button>
					</>
				)
			}
			toolbar={
				<div className="flex flex-col gap-[var(--hkui-space-3)]">
					<InvoiceDocumentsTabs
						tab={tab}
						onChange={(next) =>
							setParams({ tab: next === "issued" ? null : next })
						}
					/>
					{tab === "received" ? null : (
						<>
							<ScFilters aria-label="開票方式">
								{MODE_FILTERS.map((item) => (
									<ScPill
										key={item.value || "all-mode"}
										as="button"
										active={mode === item.value}
										aria-selected={mode === item.value}
										onClick={() => setParams({ mode: item.value || null })}
									>
										{item.label ||
											INVOICE_FULFILLMENT_MODE_LABEL[item.value] ||
											item.value}
									</ScPill>
								))}
							</ScFilters>
							<ScFilters aria-label="文件狀態">
								{STATUS_FILTERS.map((item) => (
									<ScPill
										key={item.value || "all-status"}
										as="button"
										active={status === item.value}
										aria-selected={status === item.value}
										onClick={() => setParams({ status: item.value || null })}
									>
										{item.label ||
											INVOICE_DOCUMENT_STATUS_LABEL[item.value] ||
											item.value}
									</ScPill>
								))}
								<ScPill
									as="button"
									active={abnormalOnly}
									aria-selected={abnormalOnly}
									onClick={() =>
										setParams({ abnormal_only: abnormalOnly ? null : "true" })
									}
								>
									只看異常
								</ScPill>
							</ScFilters>
							<div className="flex flex-wrap items-start gap-[var(--hkui-space-3)]">
								<Input
									size="lg"
									type="text"
									label="起始日期"
									placeholder="YYYY-MM-DD"
									value={dateFrom}
									isInvalid={!isValidDate(dateFrom)}
									errorMessage={
										isValidDate(dateFrom)
											? undefined
											: "請輸入 YYYY-MM-DD 格式的日期"
									}
									onChange={(event) =>
										setParams({ date_from: event.target.value || null })
									}
								/>
								<Input
									size="lg"
									type="text"
									label="結束日期"
									placeholder="YYYY-MM-DD"
									value={dateTo}
									isInvalid={!isValidDate(dateTo)}
									errorMessage={
										isValidDate(dateTo)
											? undefined
											: "請輸入 YYYY-MM-DD 格式的日期"
									}
									onChange={(event) =>
										setParams({ date_to: event.target.value || null })
									}
								/>
							</div>
						</>
					)}
				</div>
			}
		>
			<BackLinkRow />

			{tab === "received" ? (
				<SellerReceivedInvoicesPanel
					page={page}
					onPageChange={(next) => setParams({ page: String(next) })}
					suspended={isSuspended}
				/>
			) : isSuspended ? (
				<section role="status" className="seller-card flex flex-col gap-2 p-6">
					<strong className="text-base font-bold">
						帳號停權期間無法查看發票紀錄。
					</strong>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						已開立的發票仍會正常寄送給買家。
					</p>
				</section>
			) : (
				<>
					<section className="seller-card">
						<div className="seller-orders__table-wrap">
							{/*
							 * ≥768px 走表格、<768px 走卡片：同一份資料渲染兩份 DOM ＋ Tailwind
							 * 斷點 `hidden`。`display:none` 讓被隱藏那份不進 accessibility tree，
							 * 螢幕閱讀器只會讀到一組列表（§S06 Responsive）。
							 */}
							<table className="hidden seller-table md:table">
								<caption className="sr-only">{recordsLabel}</caption>
								<thead>
									<tr>
										<th scope="col">訂單編號</th>
										<th scope="col">付款時間</th>
										<th scope="col">買家類型</th>
										<th scope="col">開票方式</th>
										<th scope="col">金額</th>
										<th scope="col">發票狀態</th>
										<th scope="col">出貨狀態</th>
										<th scope="col">更新時間</th>
									</tr>
								</thead>
								<tbody>
									{!datesValid ? (
										<tr>
											<td colSpan={COLUMNS}>
												<p
													role="alert"
													className="m-0 py-6 text-center text-sm text-danger-700"
												>
													請輸入 YYYY-MM-DD 格式的日期
												</p>
											</td>
										</tr>
									) : loading ? (
										<SellerInvoiceDocumentsSkeletonRows rows={8} />
									) : error ? (
										<tr>
											<td colSpan={COLUMNS}>
												<div
													role="alert"
													className="flex flex-col items-center gap-3 py-8 text-center"
												>
													<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
														發票紀錄載入失敗，請稍後再試。
													</p>
													<Button
														size="lg"
														variant="bordered"
														onClick={() => void refresh()}
													>
														重試
													</Button>
												</div>
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
											<DocumentRow
												key={item.summary.documentId}
												item={item}
												highlight={focusDocumentId === item.summary.documentId}
												shipmentDegraded={shipmentDegraded}
											/>
										))
									)}
								</tbody>
							</table>
						</div>

						<ul className="m-0 flex list-none flex-col gap-[var(--hkui-space-3)] p-0 md:hidden">
							{!datesValid ? (
								<li role="alert" className="p-4 text-sm text-danger-700">
									請輸入 YYYY-MM-DD 格式的日期
								</li>
							) : loading ? (
								<li className="p-4 text-sm text-[var(--hkui-foreground-soft)]">
									載入中…
								</li>
							) : error ? (
								<li role="alert" className="flex flex-col gap-3 p-4 text-sm">
									<span className="text-[var(--hkui-foreground-soft)]">
										發票紀錄載入失敗，請稍後再試。
									</span>
									<Button
										size="lg"
										variant="bordered"
										onClick={() => void refresh()}
									>
										重試
									</Button>
								</li>
							) : records.length === 0 ? (
								<li>
									<EmptyBody
										hasFilters={hasFilters}
										onClear={() => router.replace(pathname)}
									/>
								</li>
							) : null}
						</ul>
						{datesValid && !loading && !error && records.length > 0 ? (
							<SellerMobileDisclosureList
								items={records}
								getKey={invoiceDocumentKey}
								resetKey={`${page}:${mode}:${status}:${abnormalOnly}:${orderNo}:${dateFrom}:${dateTo}`}
								ariaLabel={`手機版${recordsLabel}`}
								getSummary={(item) => {
									const display = invoiceDocumentDisplay(item.summary.status);
									return {
										label: `發票紀錄 ${item.orderNo || item.summary.documentId}`,
										leading: <SellerMobileDisclosureIcon name="document" />,
										title: item.orderNo || "未提供訂單編號",
										meta: formatInvoiceAmount(item.summary.totalAmount),
										status: display.label,
									};
								}}
								renderDetails={(item) => (
									<DocumentMobileDetails
										item={item}
										shipmentDegraded={shipmentDegraded}
									/>
								)}
							/>
						) : null}
					</section>

					<Pagination
						current={page}
						total={totalPages}
						onPageChange={(next) => setParams({ page: String(next) })}
					/>
				</>
			)}
		</SellerPageShell>
	);
}

function invoiceDocumentKey(item: SellerInvoiceDocumentItem) {
	return item.summary.documentId;
}

function BackLinkRow() {
	return (
		<Link href={OVERVIEW_HREF} className="text-sm underline">
			返回發票與稅籍
		</Link>
	);
}

function EmptyBody({
	hasFilters,
	onClear,
}: {
	hasFilters: boolean;
	onClear: () => void;
}) {
	if (!hasFilters) {
		return (
			<SellerEmptyState
				variant="invoices"
				seamless
				title="還沒有發票或收據紀錄"
				description="訂單付款成功後，發票會自動開立並顯示在這裡。"
			/>
		);
	}
	return (
		<div className="flex flex-col items-center gap-[var(--hkui-space-3)] py-8 text-center">
			<SellerEmptyState
				variant="invoices"
				seamless
				title="沒有符合條件的紀錄"
				description="試著調整日期或狀態條件。"
			/>
			<Button size="lg" variant="bordered" onClick={onClear}>
				清除篩選
			</Button>
		</div>
	);
}

/** 出貨狀態與付款時間目前後端恆為空字串（見檔頭）；空值一律 `—`，不猜。 */
// 出貨狀態的三態 Chip。判定在 lib/invoices/display.ts（有單元測試），這裡只做對應。
//
// 🔴 「降級」與「還沒出貨」必須是**看得出差別的兩種畫面**：兩者都畫成 `—`
// 就是把「我們不知道」說成「還沒出貨」，而那正是這一欄原本不敢畫的理由。
function ShipmentStatusChip({
	code,
	degraded,
}: {
	code: string;
	degraded: boolean;
}) {
	const view = shipmentStatusDisplay(code, degraded);
	if (view.kind === "degraded") {
		return (
			<Chip variant="flat" color="warning" size="sm">
				暫時無法取得
			</Chip>
		);
	}
	if (view.kind === "none") {
		return (
			<Chip variant="flat" size="sm">
				—
			</Chip>
		);
	}
	return (
		<Chip variant="flat" size="sm">
			{view.label}
		</Chip>
	);
}

function dash(value: string): string {
	return value || "—";
}

function DocumentRow({
	item,
	highlight,
	shipmentDegraded,
}: {
	item: SellerInvoiceDocumentItem;
	highlight: boolean;
	shipmentDegraded: boolean;
}) {
	const display = invoiceDocumentDisplay(item.summary.status);
	return (
		<tr className={highlight ? "bg-warning-50" : ""}>
			<td>
				{item.orderNo ? (
					<Link
						href={`/seller-center/orders/${encodeURIComponent(item.orderNo)}`}
						className="underline"
					>
						<code>{item.orderNo}</code>
					</Link>
				) : (
					"—"
				)}
			</td>
			<td>{dash(formatTaipeiDateTime(item.paidAt))}</td>
			<td>
				{INVOICE_RECIPIENT_TYPE_LABEL[item.buyerKind] ?? dash(item.buyerKind)}
			</td>
			<td>
				<Chip variant="flat" size="sm">
					{INVOICE_FULFILLMENT_MODE_LABEL[item.invoiceMode] ??
						dash(item.invoiceMode)}
				</Chip>
			</td>
			<td className="seller-table__amount">
				{formatInvoiceAmount(item.summary.totalAmount)}
			</td>
			<td>
				<Chip variant="flat" color={display.color} size="sm">
					{display.label}
				</Chip>
			</td>
			<td>
				{/* 獨立的第二顆 Chip——與發票狀態是兩個維度，不可合併（見檔頭）。 */}
				<ShipmentStatusChip
					code={item.shipmentStatus}
					degraded={shipmentDegraded}
				/>
			</td>
			<td>
				<span className="flex flex-col gap-1">
					<small className="text-[var(--hkui-foreground-soft)]">
						{dash(formatTaipeiDateTime(item.updatedAt))}
					</small>
					{item.orderNo ? (
						<Link
							href={`/seller-center/orders/${encodeURIComponent(item.orderNo)}`}
							className="text-sm underline"
						>
							查看訂單
						</Link>
					) : null}
					{/*
					 * 🔴 `latestJobId` 為空字串時**整顆不渲染**，不要渲染 disabled 鍵：
					 * 「這份文件沒有任何 operation」與「你沒有權限」是兩件事。也不得拿
					 * `documentId` 去組 `/jobs/{jobId}`——打進去會 404＋9340，而那個 404
					 * 與「不是你的」長得完全一樣（Tier A），沒有人查得出原因（§B-2.0 T06）。
					 */}
					{item.latestJobId ? (
						<Link
							href={`/seller-center/invoices/jobs/${encodeURIComponent(item.latestJobId)}`}
							className="text-sm underline"
						>
							查看異常
						</Link>
					) : null}
					{/* S11：document-scoped 詳情。銷項列不帶 `?kind=`（預設就是銷項）。 */}
					<Link
						href={`/seller-center/invoices/${encodeURIComponent(item.summary.documentId)}`}
						className="text-sm underline"
					>
						查看發票詳情
					</Link>
				</span>
			</td>
		</tr>
	);
}

function DocumentMobileDetails({
	item,
	shipmentDegraded,
}: {
	item: SellerInvoiceDocumentItem;
	shipmentDegraded: boolean;
}) {
	const display = invoiceDocumentDisplay(item.summary.status);
	return (
		<>
			<div className="seller-mobile-disclosure__body">
				<div className="seller-mobile-disclosure__detail-row">
					<span>買家類型</span>
					<strong>
						{INVOICE_RECIPIENT_TYPE_LABEL[item.buyerKind] ??
							dash(item.buyerKind)}
					</strong>
				</div>
				<div className="seller-mobile-disclosure__detail-row">
					<span>付款時間</span>
					<span>{dash(formatTaipeiDateTime(item.paidAt))}</span>
				</div>
				<div className="seller-mobile-disclosure__facts">
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
					<ShipmentStatusChip
						code={item.shipmentStatus}
						degraded={shipmentDegraded}
					/>
					<Chip variant="flat" size="sm">
						{INVOICE_FULFILLMENT_MODE_LABEL[item.invoiceMode] ??
							dash(item.invoiceMode)}
					</Chip>
				</div>
				<div className="seller-mobile-disclosure__detail-row">
					<span>更新時間</span>
					<span>{dash(formatTaipeiDateTime(item.updatedAt))}</span>
				</div>
			</div>
			<div className="seller-mobile-disclosure__actions">
				{item.orderNo ? (
					<Link
						href={`/seller-center/orders/${encodeURIComponent(item.orderNo)}`}
						className="text-sm underline"
					>
						查看訂單
					</Link>
				) : null}
				{item.latestJobId ? (
					<Link
						href={`/seller-center/invoices/jobs/${encodeURIComponent(item.latestJobId)}`}
						className="text-sm underline"
					>
						查看異常
					</Link>
				) : null}
				<Link
					href={`/seller-center/invoices/${encodeURIComponent(item.summary.documentId)}`}
					className="text-sm underline"
				>
					查看發票詳情
				</Link>
			</div>
		</>
	);
}

export default SellerInvoiceDocumentsPage;
