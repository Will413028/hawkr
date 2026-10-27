"use client";

/**
 * S12｜進項分頁（58A §S12）。端點 72 `GET /received_documents` 是唯一的資料源。
 *
 * 進項 ＝ **平台開給這個賣家**的發票（服務費之類），與銷項（端點 30，我開給買家的）
 * 是不同的資料集、不同的法律角色：進項是報稅憑證、**唯讀**，賣家沒有權限也沒有法律
 * 地位去作廢它 ⇒ 本面一律不出現任何作廢／折讓／重寄入口。
 *
 * 🔴 **停權時 BLOCK**。58A §S12 寫「進項 tab 停權時仍可看」，但它自己指定的
 * capability 是 `CapInvoiceManage`，而 `gateway/internal/authz/policy.go` 的
 * `suspendedWhitelist` **刻意不含**它（該檔逐字：「後者是擴張型／唯讀查看」）⇒
 * 停權賣家打端點 72 必回 `9240`。前端若照 58A 放行，做出來的是一頁必定 9240 的畫面
 * ＋ 一次全域停權廣播（§B-2.0 T03 的「加太多」那一半）。已回報衝突，未自行改規格。
 *
 * ⚠️ **Phase 1 這一頁很可能長期是空的**（平台服務費的自動線是 Phase 2）⇒ 空態文案
 * 必須讓賣家知道「這是正常的」而不是「壞了」。
 *
 * 匯出走既有的 `lib/csv/export.ts`（不另造第二份 CSV 組裝）。⚠️ 匯出的是**目前這一頁**
 * ——端點 72 沒有「全部匯出」的形態，前端自行翻頁累加會在賣家不知情下打出 N 次請求
 * 而共用限流額度（§0.4 ④）。按鈕文案因此逐字寫明範圍。
 */

import * as React from "react";
import Link from "@/components/shared/Link";
import Pagination from "@/components/shared/Pagination";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { downloadCsv } from "@/lib/csv/export";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useSellerReceivedInvoices } from "@/lib/hooks/useSellerInvoices";
import { formatInvoiceAmount } from "@/lib/invoices/display";
import type { SellerReceivedInvoiceItem } from "@/lib/invoices/seller-types";
import { SellerEmptyState } from "../../components/SellerEmptyState";
import { invoiceDocumentDisplay } from "../invoiceDocumentDisplay";

const PAGE_SIZE = 20;
const COLUMNS = 6;

const CSV_HEADER = [
	"開立日期",
	"發票號碼",
	"開立人",
	"用途說明",
	"金額",
	"狀態",
];

function dash(value: string): string {
	return value || "—";
}

function detailHref(documentId: string): string {
	return `/seller-center/invoices/${encodeURIComponent(documentId)}?kind=received`;
}

export function SellerReceivedInvoicesPanel({
	page,
	onPageChange,
	suspended,
}: {
	page: number;
	onPageChange: (next: number) => void;
	suspended: boolean;
}) {
	const queryParams = React.useMemo(
		() => ({ page, pageSize: PAGE_SIZE }),
		[page],
	);
	const {
		page: result,
		loading,
		error,
		refresh,
	} = useSellerReceivedInvoices(queryParams);

	const records = result?.records ?? [];
	// 🔴 `total` 是總筆數不是頁數（§B-2.0 T25）。
	const totalPages = result
		? Math.max(1, Math.ceil(result.total / PAGE_SIZE))
		: 1;

	if (suspended) {
		return (
			<section role="status" className="seller-card flex flex-col gap-2 p-6">
				<strong className="text-base font-bold">
					帳號停權期間無法查看進項發票。
				</strong>
				<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
					平台開立給你的發票仍會正常寄送到你的電子信箱。
				</p>
			</section>
		);
	}

	return (
		<>
			<section className="seller-card">
				<div className="flex flex-wrap items-center justify-between gap-2 p-[var(--hkui-space-4)] pb-0">
					<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
						平台向你收取費用時開立的發票，供你報稅使用。這裡的資料唯讀。
					</p>
					<Button
						size="lg"
						variant="bordered"
						disabled={records.length === 0}
						onClick={() =>
							downloadCsv(`received-invoices-page-${page}.csv`, [
								CSV_HEADER,
								...records.map((item) => [
									formatTaipeiDate(item.summary.issuedAt),
									item.summary.invoiceNumber,
									item.issuerName,
									item.recipientNote,
									item.summary.totalAmount,
									invoiceDocumentDisplay(item.summary.status).label,
								]),
							])
						}
					>
						匯出本頁 CSV
					</Button>
				</div>
				<div className="seller-orders__table-wrap">
					{/*
					 * ≥768px 走表格、<768px 走卡片：同一份資料渲染兩份 DOM ＋ Tailwind
					 * 斷點 `hidden`。`display:none` 讓被隱藏那份不進 accessibility tree。
					 */}
					<table className="hidden seller-table md:table">
						<caption className="sr-only">進項發票</caption>
						<thead>
							<tr>
								<th scope="col">開立日期</th>
								<th scope="col">發票號碼</th>
								<th scope="col">開立人</th>
								<th scope="col">用途說明</th>
								<th scope="col">金額</th>
								<th scope="col">狀態</th>
							</tr>
						</thead>
						<tbody>
							{loading ? (
								<tr>
									<td colSpan={COLUMNS}>
										<p className="m-0 py-6 text-center text-sm text-[var(--hkui-foreground-soft)]">
											載入中…
										</p>
									</td>
								</tr>
							) : error ? (
								<tr>
									<td colSpan={COLUMNS}>
										<div
											role="alert"
											className="flex flex-col items-center gap-3 py-8 text-center"
										>
											<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
												進項發票載入失敗，請稍後再試。
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
										<ReceivedEmpty />
									</td>
								</tr>
							) : (
								records.map((item) => (
									<ReceivedRow key={item.summary.documentId} item={item} />
								))
							)}
						</tbody>
					</table>
				</div>

				<ul className="m-0 flex list-none flex-col gap-[var(--hkui-space-3)] p-0 md:hidden">
					{loading ? (
						<li className="p-4 text-sm text-[var(--hkui-foreground-soft)]">
							載入中…
						</li>
					) : error ? (
						<li role="alert" className="flex flex-col gap-3 p-4 text-sm">
							<span className="text-[var(--hkui-foreground-soft)]">
								進項發票載入失敗，請稍後再試。
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
							<ReceivedEmpty />
						</li>
					) : null}
				</ul>
				{!loading && !error && records.length > 0 ? (
					<SellerMobileDisclosureList
						items={records}
						getKey={receivedInvoiceKey}
						resetKey={String(page)}
						ariaLabel="手機版進項發票"
						getSummary={(item) => {
							const display = invoiceDocumentDisplay(item.summary.status);
							return {
								label: `進項發票 ${item.summary.invoiceNumber || item.summary.documentId}`,
								leading: <SellerMobileDisclosureIcon name="document" />,
								title: dash(item.summary.invoiceNumber),
								meta: formatInvoiceAmount(item.summary.totalAmount) || "—",
								status: display.label,
							};
						}}
						renderDetails={(item) => <ReceivedMobileDetails item={item} />}
					/>
				) : null}
			</section>

			<Pagination
				current={page}
				total={totalPages}
				onPageChange={onPageChange}
			/>
		</>
	);
}

function receivedInvoiceKey(item: SellerReceivedInvoiceItem) {
	return item.summary.documentId;
}

/** 空態文案必須讓賣家知道「這是正常的」——Phase 1 這一頁很可能長期是空的。 */
function ReceivedEmpty() {
	return (
		<SellerEmptyState
			variant="invoices"
			seamless
			title="目前沒有收到任何發票"
			description="平台向你收取費用時，發票會顯示在這裡。"
		/>
	);
}

function ReceivedRow({ item }: { item: SellerReceivedInvoiceItem }) {
	const display = invoiceDocumentDisplay(item.summary.status);
	return (
		<tr>
			<td>{dash(formatTaipeiDate(item.summary.issuedAt))}</td>
			<td className="tabular-nums">
				{/* 進項的發票號碼**完整不遮**（58 §8.1.3(b) O 類與裁決 1）：它是報帳與
				    對獎的唯一憑據。 */}
				<Link href={detailHref(item.summary.documentId)} className="underline">
					{dash(item.summary.invoiceNumber)}
				</Link>
			</td>
			<td>{dash(item.issuerName)}</td>
			<td>{dash(item.recipientNote)}</td>
			<td className="seller-table__amount">
				{formatInvoiceAmount(item.summary.totalAmount) || "—"}
			</td>
			<td>
				<Chip variant="flat" color={display.color} size="sm">
					{display.label}
				</Chip>
			</td>
		</tr>
	);
}

function ReceivedMobileDetails({ item }: { item: SellerReceivedInvoiceItem }) {
	const display = invoiceDocumentDisplay(item.summary.status);
	return (
		<>
			<div className="seller-mobile-disclosure__body">
				<div className="seller-mobile-disclosure__detail-row">
					<span>開立日期</span>
					<span>{dash(formatTaipeiDate(item.summary.issuedAt))}</span>
				</div>
				<div className="seller-mobile-disclosure__detail-row">
					<span>開立人</span>
					<strong>{dash(item.issuerName)}</strong>
				</div>
				<div className="seller-mobile-disclosure__detail-row">
					<span>用途說明</span>
					<span>{dash(item.recipientNote)}</span>
				</div>
				<div className="seller-mobile-disclosure__facts">
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
				</div>
			</div>
			<div className="seller-mobile-disclosure__actions">
				<Link
					href={detailHref(item.summary.documentId)}
					className="text-sm underline"
				>
					查看發票詳情
				</Link>
			</div>
		</>
	);
}

export default SellerReceivedInvoicesPanel;
