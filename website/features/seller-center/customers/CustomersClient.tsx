"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
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
import { fetchOrders } from "@/lib/orders/repo";
import {
	fetchCustomerTags,
	type SellerCustomer,
	type SellerCustomerType,
} from "@/lib/seller/repo";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import CustomerDetailDrawer from "./CustomerDetailDrawer";
import {
	type SellerCustomersSort,
	useSellerCustomersUiStore,
} from "./customersUiStore";

function formatTime(ts?: number): string {
	return formatTaipeiDate(ts) || "—";
}

const TYPE_TABS: Array<{ value: "" | SellerCustomerType; label: string }> = [
	{ value: "", label: "全部" },
	{ value: "vip", label: "VIP" },
	{ value: "blocklist", label: "黑名單" },
];

const SORT_OPTIONS: Array<{
	value: "totalSpent" | "orderCount" | "last_order_at";
	label: string;
}> = [
	{ value: "totalSpent", label: "依消費金額" },
	{ value: "orderCount", label: "依訂單數" },
	{ value: "last_order_at", label: "依最近購買" },
];

const TYPE_BADGE: Partial<Record<SellerCustomerType, string>> = {
	vip: "VIP",
	blocklist: "黑名單",
};

function customerRowKey(customer: SellerCustomer): string {
	return customer.id;
}

function customerTypeLabel(customerType?: SellerCustomerType): string {
	return (customerType && TYPE_BADGE[customerType]) || "一般";
}

export default function CustomersClient() {
	const searchParams = useSearchParams();
	const openChat = useChatDrawerStore((state) => state.openChat);
	// Seeded from `?q=` so deep-links (e.g. a buyer name clicked in the orders
	// table) land pre-filtered on that customer.
	const initialQuery = searchParams?.get("q")?.trim() ?? "";
	const search = useSellerCustomersUiStore((state) => state.search);
	const setSearch = useSellerCustomersUiStore((state) => state.setSearch);
	// Server-driven segmentation (matches the app): classification tab, tag
	// filter, and sort key. Changing any of these refetches.
	const customerType = useSellerCustomersUiStore((state) => state.customerType);
	const setCustomerType = useSellerCustomersUiStore(
		(state) => state.setCustomerType,
	);
	const tagId = useSellerCustomersUiStore((state) => state.tagId);
	const setTagId = useSellerCustomersUiStore((state) => state.setTagId);
	const sortBy = useSellerCustomersUiStore((state) => state.sortBy);
	const setSortBy = useSellerCustomersUiStore((state) => state.setSortBy);
	// Tag pool powers the filter chips; shared with the tag editor's suggestions.
	const tagPool = useSellerTags();
	const [tagsByCustomerId, setTagsByCustomerId] = React.useState<
		Record<string, string[]>
	>({});
	const [lastPurchaseByCustomerId, setLastPurchaseByCustomerId] =
		React.useState<Record<string, number>>({});
	// Drawer state — clicking a row opens the detail in a right-side Sheet so
	// the seller stays on the list and can flick between customers. The full
	// `/customers/[id]` route remains the deep-link / refresh fallback.
	const activeCustomerId = useSellerCustomersUiStore(
		(state) => state.activeCustomerId,
	);
	const setActiveCustomerId = useSellerCustomersUiStore(
		(state) => state.setActiveCustomerId,
	);

	// Server-driven segmentation lives in the queryKey; changing any param
	// refetches the list (its own cache entry — other consumers use other sizes).
	const params = React.useMemo(
		() => ({
			pageSize: 50,
			keyword: initialQuery || undefined,
			customerType: customerType || undefined,
			tagId: tagId ?? undefined,
			sortBy: sortBy === "last_order_at" ? undefined : sortBy,
		}),
		[initialQuery, customerType, tagId, sortBy],
	);
	const { data: baseCustomers, loading, error } = useSellerCustomers(params);
	const invalidateCustomers = useInvalidateSellerCustomers();

	React.useEffect(() => {
		if (initialQuery) setSearch(initialQuery);
	}, [initialQuery, setSearch]);

	// Two N-row enrichment fan-outs patch gaps the CRM list endpoint leaves: the
	// authoritative per-buyer tags and the last-purchase date. Both key off the
	// query result reference, so an intentional refetch (mount / filter change /
	// post-mutation invalidate) re-runs them. ponytail: structural sharing keeps
	// the reference stable across content-identical refetches, so this fires at
	// most once per real list change — not per render.
	React.useEffect(() => {
		const rows = baseCustomers ?? [];
		if (rows.length === 0) {
			setTagsByCustomerId({});
			return;
		}
		let alive = true;
		Promise.allSettled(
			rows.map(async (c) => {
				const tags = await fetchCustomerTags(c.id);
				return [c.id, tags.map((tag) => tag.name)] as const;
			}),
		).then((results) => {
			if (!alive) return;
			const next: Record<string, string[]> = {};
			for (const result of results) {
				if (result.status !== "fulfilled") continue;
				const [id, tags] = result.value;
				next[id] = tags;
			}
			setTagsByCustomerId(next);
		});
		return () => {
			alive = false;
		};
	}, [baseCustomers]);

	React.useEffect(() => {
		const rows = baseCustomers ?? [];
		if (rows.length === 0) {
			setLastPurchaseByCustomerId({});
			return;
		}
		let alive = true;
		Promise.allSettled(
			rows.map(async (c) => {
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
			if (!alive) return;
			const next: Record<string, number> = {};
			for (const result of results) {
				if (result.status !== "fulfilled") continue;
				const [id, latest] = result.value;
				if (latest > 0) next[id] = latest;
			}
			setLastPurchaseByCustomerId(next);
		});
		return () => {
			alive = false;
		};
	}, [baseCustomers]);

	// Base list + the per-row tag fan-out, merged for render. Derived (not state)
	// so a fresh query result never flashes an empty table before enrichment.
	const items = React.useMemo(() => {
		const rows = baseCustomers ?? [];
		return rows.map((c) => {
			const tags = tagsByCustomerId[c.id];
			return tags ? { ...c, tags } : c;
		});
	}, [baseCustomers, tagsByCustomerId]);

	const closeDrawer = React.useCallback(() => {
		setActiveCustomerId(null);
	}, [setActiveCustomerId]);

	const visible = React.useMemo(() => {
		const q = search.trim().toLowerCase();
		const filtered = !q
			? items
			: items.filter(
					(c) =>
						c.name?.toLowerCase().includes(q) ||
						c.phone?.toLowerCase().includes(q),
				);
		if (sortBy !== "last_order_at") return filtered;
		return [...filtered].sort(
			(a, b) =>
				(lastPurchaseByCustomerId[b.id] ?? 0) -
				(lastPurchaseByCustomerId[a.id] ?? 0),
		);
	}, [items, lastPurchaseByCustomerId, search, sortBy]);

	const totalSpend = items.reduce(
		(sum, c) => sum + Number(c.totalSpent ?? 0),
		0,
	);
	const repeatBuyers = items.filter((c) => (c.orderCount ?? 0) >= 2).length;
	const activeCustomer =
		activeCustomerId === null
			? null
			: (items.find((customer) => customer.id === activeCustomerId) ?? null);

	return (
		<SellerPageShell
			className="seller-customers"
			title="客戶管理"
			subtitle={`總共 ${items.length} 位客戶`}
			actions={
				<div className="seller-orders__search">
					<AppIcon name="search" />
					<input
						type="search"
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						placeholder="搜尋客戶姓名或電話…"
						aria-label="搜尋客戶"
					/>
				</div>
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
					<strong>{items.length.toLocaleString()}</strong>
					<small>累積客戶</small>
				</div>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-secondary">
						<AppIcon name="check" />
					</span>
					<strong>{repeatBuyers.toLocaleString()}</strong>
					<small>回購客戶（≥ 2 筆）</small>
				</div>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-info">
						<AppIcon name="coin" />
					</span>
					<strong>NT$ {Math.round(totalSpend).toLocaleString()}</strong>
					<small>客戶累積消費</small>
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
						value={sortBy}
						onChange={(e) => setSortBy(e.target.value as SellerCustomersSort)}
					>
						{SORT_OPTIONS.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
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

			{error && (
				<p className="seller-customers__error" role="alert">
					{error}
				</p>
			)}

			<div
				className={[
					"seller-card seller-customers__card",
					visible.length > 0 ? "seller-mobile-disclosure-desktop" : "",
				]
					.filter(Boolean)
					.join(" ")}
			>
				<div className="seller-orders__table-wrap">
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
							{loading && items.length === 0 ? (
								<SellerCustomersSkeletonRows rows={5} />
							) : visible.length === 0 ? (
								<tr>
									<td colSpan={6} className="seller-orders__empty">
										<SellerEmptyState
											variant="customers"
											searchQuery={search}
											seamless
										/>
									</td>
								</tr>
							) : (
								visible.map((c) => (
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
														{c.customerType && TYPE_BADGE[c.customerType] && (
															<span
																className={`seller-customers__badge is-${c.customerType}`}
															>
																{TYPE_BADGE[c.customerType]}
															</span>
														)}
													</strong>
													{c.phone && <small>{c.phone}</small>}
												</div>
											</button>
										</td>
										<td
											className="seller-customers__tags-cell"
											data-label="標籤"
										>
											{c.tags && c.tags.length > 0 ? (
												<div className="seller-customers__tags seller-customers__tags--table">
													{c.tags.slice(0, 4).map((t) => (
														<span key={t} className="seller-customers__tag">
															{t}
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
											{c.orderCount ?? 0} 筆
										</td>
										<td
											className="is-right seller-orders__amount"
											data-label="累計消費"
										>
											NT$ {Number(c.totalSpent ?? 0).toLocaleString()}
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
				</div>
			</div>

			{visible.length > 0 ? (
				<SellerMobileDisclosureList
					items={visible}
					getKey={customerRowKey}
					ariaLabel="客戶清單"
					resetKey={JSON.stringify([search, customerType, tagId, sortBy])}
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
						meta: `${(customer.orderCount ?? 0).toLocaleString()} 筆`,
						status: customerTypeLabel(customer.customerType),
					})}
					renderDetails={(customer) => (
						<>
							<div className="seller-mobile-disclosure__body">
								{customer.phone || (customer.tags?.length ?? 0) > 0 ? (
									<div className="seller-mobile-disclosure__facts">
										{customer.phone ? (
											<span className="seller-mobile-disclosure__fact">
												電話 {customer.phone}
											</span>
										) : null}
										{customer.tags?.map((tag) => (
											<span
												key={tag}
												className="seller-mobile-disclosure__fact"
											>
												{tag}
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
									<strong>
										NT$ {Number(customer.totalSpent ?? 0).toLocaleString()}
									</strong>
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

			<CustomerDetailDrawer
				open={activeCustomer !== null}
				customer={activeCustomer}
				onClose={closeDrawer}
				// 分類 / 標籤變更當下即時 invalidate 清單（含 badge / chip），不再等關閉。
				onMutated={invalidateCustomers}
			/>
		</SellerPageShell>
	);
}
