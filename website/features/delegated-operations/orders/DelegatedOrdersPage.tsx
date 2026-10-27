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

	// 輸入框顯示的是本地草稿，停手 240ms 才寫進 URL（＝才發查詢）。逐字發查詢會對
	// 每一個中間狀態各打 6 支請求（清單 ＋ 五張 KPI 卡），而中間狀態沒有一個是使用者
	// 想看的結果。
	const [keywordDraft, setKeywordDraft] = React.useState(keyword);
	React.useEffect(() => {
		// ⚠️ **比對與寫入都用 trim 後的值**（判準見 `keywordUrlValue`）。少了它，
		// 只打幾個空白也會換出一組新的 react-query 鍵，白打 6 支內容完全一樣的請求，
		// 順帶重置 page 並留下 `?keyword=%20%20%20` 這種可分享網址。
		const next = keywordUrlValue(keywordDraft);
		if ((next ?? "") === keyword) return;
		const timer = window.setTimeout(() => {
			void setKeyword(next);
			void setPage(1);
		}, KEYWORD_DEBOUNCE_MS);
		return () => window.clearTimeout(timer);
	}, [keywordDraft, keyword, setKeyword, setPage]);

	// 三個條件併成一份，清單與 KPI 卡都吃它 —— 兩邊各自組會漂移，而漂移的結果是
	// 畫面上兩個互相矛盾的數字。
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
		// capability 不足時**不發任何請求**：打一支必然 403 的端點來探測，會在稽核表
		// 製造一批 FAILED 的讀取列，把「有人試圖跨店讀取」這個安全訊號淹掉。
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
						{/* mockup 右端還有一個「查看權限說明」外連。實查兩個前端 repo 都沒有
						    權限說明頁，設計書 §8.1.1 是內部章節不是使用者可見頁面 —— 連到一個
						    不存在的位置比沒有連結更糟，故不渲染。它要解釋的事情，情境列的
						    capability chips 與抽屜裡的履約鎖定卡已經各答了一次。 */}
						<div className="adm-orders__notice" role="note">
							<AppIcon name="lock" size={20} />
							{/* 產品需求 §6.7 的固定告知文案，逐字照抄。**「與處理平台異常」
							    這半句不得再刪掉**：它對應訂單抽屜底部那條「前往平台異常
							    處置」的出口，兩者是同一件事的宣告與落點。曾經因為那顆鈕
							    沒做而把後半句一起砍掉 —— 文案與功能一起消失是誠實的，但
							    結果是 ADM 發現訂單異常時畫面上沒有任何出口，只能結束代行、
							    換身分、重新找那筆單。 */}
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
							{/* 版型沿用賣家中心的 `.seller-orders__search`（同一顆放大鏡、同一個
							    輸入框），`adm/layout.tsx` 已經 import 了 seller-center.css。 */}
							<div className="seller-orders__search">
								<AppIcon name="search" />
								<input
									type="search"
									value={keywordDraft}
									onChange={(event) => setKeywordDraft(event.target.value)}
									/* placeholder 逐字對應後端的比對範圍，一個字都不能多：
									   收件人電話與完整地址**不在**比對範圍內（列表投影看不到
									   它們，拿來當條件等於造出一支不落稽核的存在性 oracle），
									   寫進來就是承諾一件做不到的事。賣家自助端的
									   「搜尋訂單編號或顧客…」正是這種：那一頁的比對其實碰不到
									   顧客電話，但文案讀起來像可以。 */
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
							{/* `max`／`min` 互鎖：反向的區間後端會回 9260／400，而使用者看到的
							    只是一次失敗的查詢，指不出是哪一格填錯了。交給瀏覽器原生擋，
							    不必自己畫一行錯誤訊息。 */}
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
									{/* 設計書 §6.7 的保留清單逐字列了「物流狀態」，而它與訂單狀態不是
									    同一件事：代行者要回答的是「賣家沒動，這筆卡在哪」，卡點在物流
									    這一側。列表 DTO 本來就帶著 `shippingStatusLabel`，先前只有明細
									    在用。 */}
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
											{/* 判準與文案在 `display.ts`（`delegatedOrdersEmptyCopy`）：
											    有篩選條件時不得說「目前沒有訂單」。`searchQuery` 是這支
											    元件切「查無符合」表現（收據放大鏡圖示）的既有開關，
											    文字另由 title／description 覆寫。**它必須非空**：留空
											    會退回「空清單」的動畫，於是「只選了日期而零筆」與
											    「只選了狀態而零筆」兩個同樣的處境長得不一樣。四個條件
											    至少有一個非空（`emptyCopy` 非 null 的前提就是它）。 */}
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
											{/* 搜尋比對 `order_items.product_name`／`sku_name`，而這一欄一度只印
											    件數 ⇒ 搜一個品名拿到四筆、四列全是「共 2 件」，操作者無從判斷
											    是哪一件命中。第二行的「等 N 件」只在 N > 1 出現：N = 1 時它對
											    畫面沒有新增任何資訊，而每一列都掛一行「等 1 件」會把真正多品項
											    的那幾列淹掉。沿用 `.seller-table__buyer` 的同一組 strong／small
											    兩行式宣告（seller-center.css 以逗號共用同一條規則），不新畫、
											    不新增 token。 */}
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
														{/* 第二行放**收件人**，不是 mockup 那行電話：列表 DTO 不帶任何
														    電話（上游 OrderSummaryView 沒有這個欄位，
														    `admin_delegated_order.go:51-53` 明令禁止為此反向擴充 proto）。
														    放收件人是因為搜尋會比對 `recipient_name` 而代訂很常見 ——
														    搜「周勾」跳出四筆買家全叫 Cora Chou，操作者的第一反應會是
														    「搜尋壞了」。結果正確但無法自我解釋，比沒有搜尋更難信任。
														    **永遠顯示**（空值走 —）：只在收件人≠買家時才出現，會讓同一張
														    表在不同資料下長不一樣，QA 取樣要涵蓋兩種、操作者也會困惑。
														    帶「收件人：」前綴：兩行都是人名，不標就分不出誰是誰。 */}
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
												{/* 還沒建立物流單的訂單（待付款、剛成立）這一欄是空的 ——
												    顯示 `—`，不編一個狀態出來。 */}
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
													{/* 層 1 的正面表達：不畫出貨 CTA，改畫「為什麼這裡沒有」。
													    ⚠️ 但終態訂單不適用 —— 對一筆已取消／已完成的訂單說
													    「僅賣家可出貨」，宣告了一個對賣家本人也不存在的動作，
													    正是 §8.1.1 要防的那種「這裡有功能只是你沒權限」。 */}
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
