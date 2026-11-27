"use client";

import {
	CustomerSort,
	CustomerType,
} from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { useTranslations } from "next-intl";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { ScrollableTableRegion } from "@/components/shared/ScrollableTableRegion";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	SellerCustomersSkeletonRows,
	SellerEmptyState,
} from "@/features/seller-center";
import { SellerMobileDisclosureList } from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDate } from "@/lib/date/taipei";
import {
	useInvalidateSellerCustomers,
	useSellerCustomers,
} from "@/lib/hooks/useSellerCustomers";
import { useSellerTags } from "@/lib/hooks/useSellerTags";
import { useUiText } from "@/lib/i18n/ui-text";
import { fetchOrders } from "@/lib/orders/repo";
import {
	customerSortText,
	customerTypeClass,
	customerTypeText,
	normalizeCustomerSort,
	type SellerCustomer,
} from "@/lib/seller/crm";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import CustomerDetailDrawer from "./CustomerDetailDrawer";
import { useSellerCustomersUiStore } from "./customersUiStore";

function formatTime(ts?: number): string {
	return formatTaipeiDate(ts) || "—";
}

const TYPE_TABS: Array<{ value: CustomerType; label: string }> = [
	{ value: CustomerType.UNSPECIFIED, label: "全部" },
	{ value: CustomerType.VIP, label: "VIP" },
];

const SORT_OPTIONS: CustomerSort[] = [
	CustomerSort.TOTAL_SPENT,
	CustomerSort.ORDER_COUNT,
	CustomerSort.FIRST_ORDER_TIME,
];

function customerRowKey(customer: SellerCustomer): string {
	return customer.id;
}

export default function CustomersClient() {
	const ut = useUiText();
	const t = useTranslations("sellerCenter.customers");
	const openChat = useChatDrawerStore((state) => state.openChat);
	const customerType = useSellerCustomersUiStore((state) => state.customerType);
	const setCustomerType = useSellerCustomersUiStore(
		(state) => state.setCustomerType,
	);
	const tagId = useSellerCustomersUiStore((state) => state.tagId);
	const setTagId = useSellerCustomersUiStore((state) => state.setTagId);
	const sort = useSellerCustomersUiStore((state) => state.sort);
	const setSort = useSellerCustomersUiStore((state) => state.setSort);
	const { tags: tagPool, error: tagPoolError } = useSellerTags();
	const [lastPurchaseByCustomerId, setLastPurchaseByCustomerId] =
		React.useState<Record<string, number>>({});
	const requestedPurchaseIds = React.useRef(new Set<string>());
	const activeCustomerId = useSellerCustomersUiStore(
		(state) => state.activeCustomerId,
	);
	const setActiveCustomerId = useSellerCustomersUiStore(
		(state) => state.setActiveCustomerId,
	);

	const params = React.useMemo(
		() => ({
			type:
				customerType === CustomerType.UNSPECIFIED ? undefined : customerType,
			tagId: tagId ?? undefined,
			sort,
			descending: true,
		}),
		[customerType, tagId, sort],
	);
	const {
		data: items,
		loading,
		error,
		hasMore,
		loadingMore,
		loadMore,
	} = useSellerCustomers(params);
	const invalidateCustomers = useInvalidateSellerCustomers();
	const rows = React.useMemo(() => items ?? [], [items]);

	React.useEffect(() => {
		const fresh = rows.filter((c) => !requestedPurchaseIds.current.has(c.id));
		if (fresh.length === 0) return;
		for (const c of fresh) requestedPurchaseIds.current.add(c.id);
		Promise.allSettled(
			fresh.map(async (c) => {
				const res = await fetchOrders({
					view: "seller",
					buyerId: String(c.id),
					pageSize: 50,
				});
				const latest = res.records.reduce(
					(max, order) => Math.max(max, order.createdAt || 0),
					0,
				);
				return [c.id, latest] as const;
			}),
		).then((results) => {
			const next: Record<string, number> = {};
			for (const [i, result] of results.entries()) {
				if (result.status !== "fulfilled") {
					requestedPurchaseIds.current.delete(fresh[i].id);
					continue;
				}
				const [id, latest] = result.value;
				if (latest > 0) next[id] = latest;
			}
			setLastPurchaseByCustomerId((prev) => ({ ...prev, ...next }));
		});
	}, [rows]);

	const closeDrawer = React.useCallback(() => {
		setActiveCustomerId(null);
	}, [setActiveCustomerId]);

	const activeCustomer =
		activeCustomerId === null
			? null
			: (rows.find((customer) => customer.id === activeCustomerId) ?? null);

	return (
		<SellerPageShell
			className="seller-customers"
			title="客戶管理"
			subtitle={
				hasMore
					? t("loaded", { count: rows.length })
					: `總共 ${rows.length} 位客戶`
			}
		>
			<section
				className="seller-customers__stats seller-summary-rail"
				aria-label="客戶摘要"
				// biome-ignore lint/a11y/noNoninteractiveTabindex: This horizontally scrollable summary needs a keyboard focus target.
				tabIndex={0}
			>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-primary">
						<AppIcon name="users" />
					</span>
					<strong>
						{rows.length.toLocaleString()}
						{hasMore ? "+" : ""}
					</strong>
					<small>累積客戶</small>
				</div>
			</section>

			<div className="seller-customers__filters">
				<div
					className="seller-customers__tabs"
					role="tablist"
					aria-label="客戶分類"
				>
					{TYPE_TABS.map((tab) => (
						<button
							key={tab.value || "all"}
							type="button"
							role="tab"
							aria-selected={customerType === tab.value}
							className={`seller-customers__tab${
								customerType === tab.value ? " is-active" : ""
							}`}
							onClick={() => setCustomerType(tab.value)}
						>
							{tab.label}
						</button>
					))}
				</div>

				<label className="seller-customers__sort">
					<span className="visually-hidden">排序方式</span>
					<select
						value={normalizeCustomerSort(sort)}
						onChange={(e) => setSort(Number(e.target.value) as CustomerSort)}
					>
						{SORT_OPTIONS.map((opt) => (
							<option key={opt} value={opt}>
								{customerSortText(ut, opt)}
							</option>
						))}
					</select>
				</label>
			</div>

			{tagPool.length > 0 && (
				<fieldset className="seller-customers__tag-filter">
					<legend className="visually-hidden">依標籤篩選</legend>
					<button
						type="button"
						className={`seller-customers__tag-chip${tagId === null ? " is-active" : ""}`}
						onClick={() => setTagId(null)}
					>
						全部標籤
					</button>
					{tagPool.map((tag) => (
						<button
							key={tag.id}
							type="button"
							className={`seller-customers__tag-chip${tagId === tag.id ? " is-active" : ""}`}
							onClick={() => setTagId(tagId === tag.id ? null : tag.id)}
						>
							{tag.name}
						</button>
					))}
				</fieldset>
			)}

			{tagPoolError && (
				<p className="seller-customers__error" role="alert">
					{tagPoolError}
				</p>
			)}

			{error && items !== null && (
				<p className="seller-customers__error" role="alert">
					{error}
				</p>
			)}

			<div
				className={[
					"seller-card seller-customers__card",
					rows.length > 0 ? "seller-mobile-disclosure-desktop" : "",
				]
					.filter(Boolean)
					.join(" ")}
			>
				<ScrollableTableRegion label="顧客表格">
					<table
						className="seller-customers__table seller-customers__table--list"
						data-mobile-layout="cards"
					>
						<thead>
							<tr>
								<th>客戶</th>
								<th>標籤</th>
								<th className="is-center">訂單</th>
								<th className="is-right">累計消費</th>
								<th className="is-center">最近購買</th>
								<th className="is-center">操作</th>
							</tr>
						</thead>
						<tbody>
							{loading && rows.length === 0 ? (
								<SellerCustomersSkeletonRows rows={5} />
							) : error && items === null ? (
								<tr>
									<td colSpan={6} className="seller-orders__empty">
										{error}
									</td>
								</tr>
							) : rows.length === 0 ? (
								<tr>
									<td colSpan={6} className="seller-orders__empty">
										<SellerEmptyState variant="customers" seamless />
									</td>
								</tr>
							) : (
								rows.map((c) => (
									<tr key={c.id}>
										<td data-label="客戶">
											<button
												type="button"
												onClick={() => setActiveCustomerId(c.id)}
												className="seller-customers__name seller-customers__name-link seller-customers__name-button"
											>
												<Avatar
													src={c.avatar}
													name={c.name}
													alt={c.name}
													entity="buyer"
													fallbackClassName="seller-customers__avatar-fallback"
													loading="eager"
												/>
												<div>
													<strong>
														{c.name || "未命名客戶"}
														{c.customerType === CustomerType.VIP && (
															<span
																className={`seller-customers__badge ${customerTypeClass(c.customerType)}`}
															>
																{customerTypeText(ut, c.customerType)}
															</span>
														)}
													</strong>
												</div>
											</button>
										</td>
										<td
											className="seller-customers__tags-cell"
											data-label="標籤"
										>
											{c.tags.length > 0 ? (
												<div className="seller-customers__tags seller-customers__tags--table">
													{c.tags.slice(0, 4).map((t) => (
														<span key={t.id} className="seller-customers__tag">
															{t.name}
														</span>
													))}
													{c.tags.length > 4 ? (
														<span className="seller-customers__tag">
															+{c.tags.length - 4}
														</span>
													) : null}
												</div>
											) : (
												<span className="seller-customers__tags-empty">
													尚無標籤
												</span>
											)}
										</td>
										<td className="is-center" data-label="訂單">
											{c.orderCount} 筆
										</td>
										<td
											className="is-right seller-orders__amount"
											data-label="累計消費"
										>
											NT$ {c.totalSpent.toLocaleString()}
										</td>
										<td
											className="is-center seller-orders__shipping"
											data-label="最近購買"
										>
											{formatTime(lastPurchaseByCustomerId[c.id])}
										</td>
										<td className="is-center" data-label="操作">
											<button
												type="button"
												onClick={() =>
													openChat(c.id, { name: c.name, avatar: c.avatar })
												}
												className="seller-orders__action"
											>
												發訊息
											</button>
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</ScrollableTableRegion>
			</div>

			{rows.length > 0 ? (
				<SellerMobileDisclosureList
					items={rows}
					getKey={customerRowKey}
					ariaLabel="客戶清單"
					resetKey={JSON.stringify([customerType, tagId, sort])}
					getSummary={(customer) => ({
						label: `客戶：${customer.name || "未命名客戶"}`,
						leading: (
							<Avatar
								src={customer.avatar}
								name={customer.name}
								alt=""
								entity="buyer"
								fallbackClassName="seller-customers__avatar-fallback"
								fallback="initial"
								loading="eager"
							/>
						),
						title: customer.name || "未命名客戶",
						meta: `${customer.orderCount.toLocaleString()} 筆`,
						status: customerTypeText(ut, customer.customerType),
					})}
					renderDetails={(customer) => (
						<>
							<div className="seller-mobile-disclosure__body">
								{customer.tags.length > 0 ? (
									<div className="seller-mobile-disclosure__facts">
										{customer.tags.map((tag) => (
											<span
												key={tag.id}
												className="seller-mobile-disclosure__fact"
											>
												{tag.name}
											</span>
										))}
									</div>
								) : (
									<div className="seller-mobile-disclosure__detail-row">
										<span>標籤</span>
										<strong>尚無標籤</strong>
									</div>
								)}
								<div className="seller-mobile-disclosure__detail-row">
									<span>累計消費</span>
									<strong>NT$ {customer.totalSpent.toLocaleString()}</strong>
								</div>
								<div className="seller-mobile-disclosure__detail-row">
									<span>最近購買</span>
									<strong>
										{formatTime(lastPurchaseByCustomerId[customer.id])}
									</strong>
								</div>
							</div>
							<div className="seller-mobile-disclosure__actions">
								<button
									type="button"
									className="seller-orders__action"
									onClick={() => setActiveCustomerId(customer.id)}
								>
									客戶資料
								</button>
								<button
									type="button"
									className="seller-orders__action seller-orders__action--secondary"
									onClick={() =>
										openChat(customer.id, {
											name: customer.name,
											avatar: customer.avatar,
										})
									}
								>
									發訊息
								</button>
							</div>
						</>
					)}
				/>
			) : null}

			{hasMore ? (
				<Button
					type="button"
					variant="bordered"
					color="default"
					onClick={() => void loadMore()}
					isLoading={loadingMore}
					loadingLabel={t("loading")}
				>
					{t("loadMore")}
				</Button>
			) : null}

			<CustomerDetailDrawer
				open={activeCustomer !== null}
				customer={activeCustomer}
				onClose={closeDrawer}
				onMutated={invalidateCustomers}
			/>
		</SellerPageShell>
	);
}
