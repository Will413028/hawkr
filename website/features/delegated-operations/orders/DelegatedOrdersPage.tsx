"use client";

import "@/styles/features/delegated-orders.css";

import { parseAsInteger, parseAsString, useQueryState } from "nuqs";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { SellerEmptyState } from "@/features/seller-center/components/SellerEmptyState";
import { SummaryStatCard } from "@/features/seller-center/components/SummaryStatCard";
import DelegatedAreaUnauthorized from "../components/DelegatedAreaUnauthorized";
import DelegatedOrderDetailDrawer from "./DelegatedOrderDetailDrawer";
import {
	buyerInitial,
	DELEGATED_ORDER_STAT_CARDS,
	DELEGATED_ORDER_STATUS_OPTIONS,
	delegatedOrdersEmptyCopy,
	formatOrderAmount,
	formatOrderDateTime,
	hasMoreThanOneItem,
	isTerminalOrderStatus,
	orderStatusText,
	orderStatusTone,
	shippingStatusText,
} from "./display";
import { hasDelegatedOrderFilter, keywordUrlValue } from "./filters";
import {
	DELEGATED_ORDER_DEFAULT_PAGE_SIZE,
	DELEGATED_ORDER_PAGE_SIZES,
	useDelegatedOrderList,
	useDelegatedOrderStatusTotals,
	useDelegatedOrdersReadable,
} from "./useDelegatedOrders";

/**
 * 代行工作區・訂單管理（mockup image8、§6.1.10）。
 *
 * ⚠️ **這一頁刻意不是 `features/seller-center/orders/SellerOrdersPage.tsx` 的一個
 * 分支。** 那支 2,700 行的頁面把出貨深深織進了自己：`nextShipmentAction`、
 * `ACTION_LABEL`／`ACTION_TONE` 批次動作表、`BulkKind`、`useSellerShipmentOperationStore`、
 * `useSellerShipmentAnomalies`、`confirmShipment`／`callTcat`／`generateHomeLabel`。
 * 設計書 §8.1.1 要求代行訂單頁**不渲染** row checkbox、批次列與任何出貨 CTA
 * （不是 `disabled`）—— 用一個 `if` 去關掉它們，等於把履約禁區的第一層降級成一個
 * 布林值：那個布林值算錯、或有人日後在別的分支多畫一顆按鈕，畫面就會出現一個
 * 打不到端點的出貨鍵，而沒有任何測試會轉紅。
 *
 * 這裡的機械保證是**整條 import 鏈上不存在出貨模組**：本目錄不 import
 * `lib/seller/shipment.ts`、不 import 任何 `useSellerShipment*`，由
 * `fulfilment-exclusion.unit.test.ts` 雙向斷言（代行側零命中 ＋ 賣家側必須全部命中，
 * 否則那支測試就是空過的）。賣家自己的 `/seller-center/orders` 一個字都沒動。
 *
 * ⚠️ **與 mockup 的兩處資料面差異**（都是「沒有來源就不畫」，不是漏做）：
 * 1. 表頭的「更新時間」欄與「訂單資訊」第三行的付款方式**不渲染** —— 列表 DTO 沒有
 *    `updatedAt`／`paymentMethod`（Gateway 明說：與其填一個看起來像資料的空字串，
 *    不如不宣告）。付款方式在**明細**有，那裡照渲染。
 * 2. 「商品」欄沒有縮圖 —— 列表 DTO 只帶 `firstProductName`（第一件品項名）與 `itemCount`，
 *    不含品項陣列，縮圖在明細裡。品名一度也不在列表 DTO，於是搜一個品名拿到四筆、四列
 *    全印「共 2 件」，畫面說不出是哪一件命中（#2547）。
 *
 * ⚠️ **篩選條件一律送到伺服器，不做 `.filter()`。** 這一頁是伺服端分頁，前端過濾只會
 * 篩掉「當前這 10 筆」裡不符的那幾列 —— 使用者搜一個真的存在但在第 4 頁的訂單編號，
 * 得到的是「查無資料」。那比沒有搜尋更糟：沒有搜尋只是麻煩，會騙人的搜尋會讓代行者
 * 據此回覆賣家「你這筆單不存在」。
 *
 * ⚠️ **KPI 卡與清單吃同一組條件**（`filters`）。少餵一個，卡片會停在「全部 81 筆」而
 * 清單只剩 3 筆，兩個數字在同一個畫面上互相矛盾。型別上是必填，漏接會編譯失敗。
 */

const PAGE_QUERY_OPTIONS = {
	history: "replace",
	shallow: true,
	scroll: false,
} as const;

const statusParser = parseAsString
	.withOptions(PAGE_QUERY_OPTIONS)
	.withDefault("");
const pageParser = parseAsInteger
	.withOptions(PAGE_QUERY_OPTIONS)
	.withDefault(1);
const sizeParser = parseAsInteger
	.withOptions(PAGE_QUERY_OPTIONS)
	.withDefault(DELEGATED_ORDER_DEFAULT_PAGE_SIZE);
const orderParser = parseAsString.withOptions(PAGE_QUERY_OPTIONS);
const keywordParser = parseAsString
	.withOptions(PAGE_QUERY_OPTIONS)
	.withDefault("");
const dateParser = parseAsString
	.withOptions(PAGE_QUERY_OPTIONS)
	.withDefault("");

/** KPI 卡固定五個 status，抽成模組常數讓陣列 identity 跨 render 穩定。 */
const STAT_STATUSES = DELEGATED_ORDER_STAT_CARDS.map((card) => card.status);

/** 打字停下來多久才送出。與 `CouponBuyerPicker` 的既有搜尋同一個值。 */
const KEYWORD_DEBOUNCE_MS = 240;

export default function DelegatedOrdersPage() {
	const readable = useDelegatedOrdersReadable();
	const [status, setStatus] = useQueryState("status", statusParser);
	const [keyword, setKeyword] = useQueryState("keyword", keywordParser);
	const [dateFrom, setDateFrom] = useQueryState("date_from", dateParser);
	const [dateTo, setDateTo] = useQueryState("date_to", dateParser);
	const [page, setPage] = useQueryState("page", pageParser);
	const [pageSize, setPageSize] = useQueryState("size", sizeParser);
	const [activeOrderNo, setActiveOrderNo] = useQueryState("order", orderParser);

	const [keywordDraft, setKeywordDraft] = React.useState(keyword);
	React.useEffect(() => {
		const next = keywordUrlValue(keywordDraft);
		if ((next ?? "") === keyword) return;
		const timer = window.setTimeout(() => {
			void setKeyword(next);
			void setPage(1);
		}, KEYWORD_DEBOUNCE_MS);
		return () => window.clearTimeout(timer);
	}, [keywordDraft, keyword, setKeyword, setPage]);

	const filters = React.useMemo(
		() => ({ status, keyword, dateFrom, dateTo }),
		[status, keyword, dateFrom, dateTo],
	);

	const { data, loading, refreshing, error, refresh } = useDelegatedOrderList({
		page,
		pageSize,
		...filters,
	});
	const { totals } = useDelegatedOrderStatusTotals(STAT_STATUSES, filters);

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const emptyCopy = delegatedOrdersEmptyCopy(filters);
	const firstRow = total === 0 ? 0 : (page - 1) * pageSize + 1;
	const lastRow = Math.min(page * pageSize, total);
	const hasNext = page * pageSize < total;

	const selectStatus = React.useCallback(
		(next: string) => {
			void setStatus(next || null);
			void setPage(1);
		},
		[setPage, setStatus],
	);

	const selectDate = React.useCallback(
		(set: (next: string | null) => unknown, next: string) => {
			void set(next || null);
			void setPage(1);
		},
		[setPage],
	);

	const clearFilters = React.useCallback(() => {
		setKeywordDraft("");
		void setStatus(null);
		void setKeyword(null);
		void setDateFrom(null);
		void setDateTo(null);
		void setPage(1);
	}, [setDateFrom, setDateTo, setKeyword, setPage, setStatus]);

	if (!readable) {
		return (
			<DelegatedAreaUnauthorized reason="本次工作階段沒有核發訂單讀取權限。需要查看訂單請返回 ADM 重新建立工作階段。" />
		);
	}

	return (
		<>
			<SellerPageShell
				title="訂單管理"
				subtitle="沿用賣家工作區的查詢體驗，但履約仍由賣家本人負責"
				toolbar={
					<>
						<div className="adm-orders__notice" role="note">
							<AppIcon name="lock" size={20} />

							<span>此區由賣家負責履約；ADM 僅可查看訂單與處理平台異常。</span>
						</div>
						<div className="adm-orders__stats" role="tablist">
							{DELEGATED_ORDER_STAT_CARDS.map((card) => (
								<SummaryStatCard
									key={card.status || "all"}
									label={
										"label" in card
											? card.label
											: orderStatusText(card.status, "")
									}
									value={String(totals[card.status] ?? 0)}
									tint={card.tint}
									icon={<AppIcon name={card.icon} size={20} />}
									active={status === card.status}
									onClick={() =>
										selectStatus(status === card.status ? "" : card.status)
									}
								/>
							))}
						</div>
						<div className="adm-orders__filters">
							<div className="seller-orders__search">
								<AppIcon name="search" />
								<input
									type="search"
									value={keywordDraft}
									onChange={(event) => setKeywordDraft(event.target.value)}
									placeholder="搜尋訂單編號、收件人或商品名稱…"
									aria-label="搜尋訂單"
								/>
							</div>
							<label className="adm-orders__filter-label" htmlFor="adm-status">
								訂單狀態
							</label>
							<select
								id="adm-status"
								className="adm-orders__select"
								value={status}
								onChange={(event) => selectStatus(event.target.value)}
							>
								{DELEGATED_ORDER_STATUS_OPTIONS.map((option) => (
									<option key={option.value} value={option.value}>
										{option.label}
									</option>
								))}
							</select>
							<label
								className="adm-orders__filter-label"
								htmlFor="adm-date-from"
							>
								建立日期
							</label>

							<input
								id="adm-date-from"
								type="date"
								className="adm-orders__select"
								value={dateFrom}
								max={dateTo || undefined}
								onChange={(event) =>
									selectDate(setDateFrom, event.target.value)
								}
								aria-label="建立日期起"
							/>
							<span className="adm-orders__filter-label">至</span>
							<input
								id="adm-date-to"
								type="date"
								className="adm-orders__select"
								value={dateTo}
								min={dateFrom || undefined}
								onChange={(event) => selectDate(setDateTo, event.target.value)}
								aria-label="建立日期迄"
							/>
							{hasDelegatedOrderFilter(filters) ? (
								<button
									type="button"
									className="adm-orders__clear"
									onClick={clearFilters}
								>
									清除篩選
								</button>
							) : null}
							<button
								type="button"
								className="adm-orders__refresh"
								onClick={() => void refresh()}
								disabled={refreshing}
								aria-label="重新整理訂單列表"
							>
								<AppIcon name="refresh" size={18} />
							</button>
						</div>
					</>
				}
			>
				<div className="seller-card">
					<div className="seller-table-wrap">
						<table className="seller-table">
							<thead>
								<tr>
									<th>訂單資訊</th>
									<th>商品</th>
									<th>買家</th>
									<th>金額</th>
									<th>訂單狀態</th>

									<th>物流狀態</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								{loading ? (
									<tr>
										<td colSpan={7}>
											<p className="seller-empty">正在載入訂單…</p>
										</td>
									</tr>
								) : error ? (
									<tr>
										<td colSpan={7}>
											<p className="seller-empty">
												訂單列表載入失敗，請按重新整理再試一次。
											</p>
										</td>
									</tr>
								) : records.length === 0 ? (
									<tr>
										<td colSpan={7}>
											{emptyCopy ? (
												<SellerEmptyState
													variant="orders"
													seamless
													searchQuery={
														keyword ||
														orderStatusText(status, "") ||
														dateFrom ||
														dateTo
													}
													title={emptyCopy.title}
													description={emptyCopy.description}
												/>
											) : (
												<SellerEmptyState variant="orders" seamless />
											)}
										</td>
									</tr>
								) : (
									records.map((order) => (
										<tr
											key={order.orderNo}
											className={
												activeOrderNo === order.orderNo
													? "adm-orders__row--active"
													: undefined
											}
										>
											<td>
												<span className="adm-orders__no">#{order.orderNo}</span>
												<span className="seller-table__id adm-orders__time">
													{formatOrderDateTime(order.createdAt)}
												</span>
											</td>

											<td className="seller-table__product">
												<strong>{order.firstProductName || "—"}</strong>
												{hasMoreThanOneItem(order.itemCount) ? (
													<small>等 {order.itemCount} 件</small>
												) : null}
											</td>
											<td className="seller-table__buyer">
												<span className="adm-orders__buyer">
													{buyerInitial(order.buyer.displayName) ? (
														<span
															className="adm-orders__buyer-avatar"
															aria-hidden="true"
														>
															{buyerInitial(order.buyer.displayName)}
														</span>
													) : null}
													<span>
														<strong>{order.buyer.displayName || "—"}</strong>

														<small>收件人：{order.recipient.name || "—"}</small>
													</span>
												</span>
											</td>
											<td className="seller-table__amount">
												{formatOrderAmount(order.totalAmount)}
											</td>
											<td>
												<span
													className={`seller-status seller-status--${orderStatusTone(order.status)}`}
												>
													{orderStatusText(order.status, order.statusLabel)}
												</span>
											</td>
											<td>
												{shippingStatusText(
													order.shippingStatus,
													order.shippingStatusLabel,
												) || "—"}
											</td>
											<td>
												<span className="adm-orders__actions">
													<button
														type="button"
														className="seller-table__action"
														onClick={() => void setActiveOrderNo(order.orderNo)}
													>
														查看詳情
													</button>

													{isTerminalOrderStatus(order.status) ? (
														<span className="adm-orders__locked-hint">
															已結案
														</span>
													) : (
														<span className="adm-orders__locked-hint">
															<AppIcon name="lock" size={12} />
															僅賣家可出貨
														</span>
													)}
												</span>
											</td>
										</tr>
									))
								)}
							</tbody>
						</table>
					</div>
					<div className="adm-orders__pagination">
						<label className="adm-orders__filter-label" htmlFor="adm-page-size">
							顯示
						</label>
						<select
							id="adm-page-size"
							className="adm-orders__select"
							value={pageSize}
							onChange={(event) => {
								void setPageSize(Number(event.target.value));
								void setPage(1);
							}}
						>
							{DELEGATED_ORDER_PAGE_SIZES.map((size) => (
								<option key={size} value={size}>
									{size}
								</option>
							))}
						</select>
						<span className="adm-orders__filter-label">筆</span>
						<span className="adm-orders__pagination-range">
							第 {firstRow}–{lastRow} 筆，共 {total} 筆
						</span>
						<button
							type="button"
							className="adm-orders__page-button"
							onClick={() => void setPage(Math.max(1, page - 1))}
							disabled={page <= 1}
							aria-label="上一頁"
						>
							<AppIcon name="chevron-left" size={16} />
						</button>
						<span className="adm-orders__page-current">{page}</span>
						<button
							type="button"
							className="adm-orders__page-button"
							onClick={() => void setPage(page + 1)}
							disabled={!hasNext}
							aria-label="下一頁"
						>
							<AppIcon name="chevron-right" size={16} />
						</button>
					</div>
				</div>
			</SellerPageShell>
			<DelegatedOrderDetailDrawer
				orderNo={activeOrderNo}
				onClose={() => void setActiveOrderNo(null)}
			/>
		</>
	);
}
