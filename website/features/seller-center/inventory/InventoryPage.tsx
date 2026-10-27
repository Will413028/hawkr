"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { downloadCsv } from "@/lib/csv/export";
import { useInvalidateSellerProducts } from "@/lib/hooks/useSellerProducts";
import { ossImageUrl } from "@/lib/media/cdn";
import { fetchOrders } from "@/lib/orders/repo";
import {
	fetchProductSkus,
	fetchSellerProducts,
	type SellerProductListItem,
} from "@/lib/products/seller-repo";
import { SellerEmptyState } from "../components/SellerEmptyState";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
	type SellerMobileDisclosureSummary,
} from "../components/SellerMobileDisclosureList";
import { SellerInventorySkeletonRows } from "../components/SellerPageSkeletons";
import { SummaryStatCard } from "../components/SummaryStatCard";
import ProductEditDrawer, {
	openDrawerOnPlainClick,
} from "../products/ProductEditDrawer";
import { useProductEditDrawer } from "../products/useProductEditDrawer";
import {
	type InventoryFilter,
	useSellerInventoryUiStore,
} from "./inventoryUiStore";

/**
 * Inventory dashboard ported from 260509 SellerDashboardPage's
 * `activeTab === "inventory"` block (mock lines 1052–1224).
 *
 * The product LIST endpoint (/app/products) does NOT emit per-product stock
 * (B-192), so real stock is aggregated from each product's SKUs via
 * `GET /app/products/{id}/skus`: 當前庫存 = Σ sku.stock, 庫存價值 =
 * Σ (sku.stock × effective price). A per-product "safe stock" threshold isn't
 * on the list response either — the form persists `low_stock_threshold` but
 * the list omits it, so we fall back to a global default of 10 for the
 * "庫存不足 / 正常" pills until the backend surfaces it.
 */

const SAFE_STOCK_DEFAULT = 10;
const PAGE_SIZE = 50;

/** Trailing window used for the weekly-sales sparkline + depletion forecast.
 *  Sales velocity is derived from the seller's recent orders (the product /
 *  SKU endpoints carry no sales history), so the forecast is only as deep as
 *  the orders we sample below. */
const FORECAST_WEEKS = 8;
const WEEK_MS = 7 * 24 * 60 * 60 * 1000;
/** How many recent seller orders to sample for the velocity estimate. */
const ORDERS_SAMPLE = 200;

/** Weeks of average demand we recommend keeping on hand as the safety
 *  buffer (the suggested 安全庫存 = 週均銷量 × this). */
const SAFE_COVER_WEEKS = 2;
/** Floor for the suggestion so slow movers still keep a small buffer. */
const SAFE_STOCK_MIN = 5;

/**
 * Suggested safety-stock level from the trailing weekly sales: hold roughly
 * `SAFE_COVER_WEEKS` weeks of average demand, with a small floor. Falls back
 * to the global default when the product has no recent sales signal.
 */
function suggestedSafeStock(weekly: number[] | undefined): number {
	const avg = (weekly ?? []).reduce((a, b) => a + b, 0) / FORECAST_WEEKS;
	if (avg <= 0) return SAFE_STOCK_DEFAULT;
	return Math.max(SAFE_STOCK_MIN, Math.ceil(avg * SAFE_COVER_WEEKS));
}

interface RowMeta {
	stock: number;
	safe: number;
	isLow: boolean;
	value: number;
}

interface InventoryDisclosureItem {
	product: SellerProductListItem;
	row: RowMeta;
	weekly: number[] | undefined;
	eta: ReturnType<typeof stockoutForecast>;
}

type EtaTone = "danger" | "warn" | "ok" | "none";

/**
 * Estimate when a product runs out from its trailing weekly sales.
 * `weekly` holds `FORECAST_WEEKS` buckets (oldest → newest); the average
 * weekly burn divides into current stock to project weeks-to-empty.
 */
function stockoutForecast(
	stock: number,
	weekly: number[] | undefined,
): { label: string; tone: EtaTone; weeks: number } {
	const sum = (weekly ?? []).reduce((a, b) => a + b, 0);
	const avg = sum / FORECAST_WEEKS;
	if (stock <= 0) return { label: "已售罄", tone: "danger", weeks: 0 };
	if (avg <= 0) return { label: "—", tone: "none", weeks: Infinity };
	const weeks = stock / avg;
	if (weeks < 1) {
		return {
			label: `約 ${Math.max(1, Math.round(weeks * 7))} 天`,
			tone: "danger",
			weeks,
		};
	}
	if (weeks >= 52) return { label: "1 年以上", tone: "ok", weeks };
	return {
		label: `約 ${Math.round(weeks)} 週`,
		tone: weeks < 4 ? "warn" : "ok",
		weeks,
	};
}

/** Minimal inline sparkline of the weekly-sales buckets. Renders a dash when
 *  there's no movement in the window. */
function Sparkline({ values }: { values: number[] | undefined }) {
	const data = values ?? [];
	if (data.length === 0 || data.every((v) => v === 0)) {
		return <span className="seller-inventory__muted">—</span>;
	}
	const w = 72;
	const h = 24;
	const max = Math.max(...data, 1);
	const step = data.length > 1 ? w / (data.length - 1) : 0;
	const pts = data
		.map((v, i) => {
			const x = i * step;
			const y = h - 3 - (v / max) * (h - 6);
			return `${x.toFixed(1)},${y.toFixed(1)}`;
		})
		.join(" ");
	const lastV = data[data.length - 1];
	const lastX = (data.length - 1) * step;
	const lastY = h - 3 - (lastV / max) * (h - 6);
	return (
		<svg
			className="seller-inventory__spark"
			width={w}
			height={h}
			viewBox={`0 0 ${w} ${h}`}
			role="img"
			aria-label={`近 ${data.length} 週銷量趨勢，共售出 ${data.reduce((a, b) => a + b, 0)} 件`}
		>
			<polyline
				points={pts}
				fill="none"
				stroke="currentColor"
				strokeWidth="1.5"
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<circle
				cx={lastX.toFixed(1)}
				cy={lastY.toFixed(1)}
				r="2.2"
				fill="currentColor"
			/>
		</svg>
	);
}

/** Aggregate per-product units sold per week from recent seller orders. */
async function buildWeeklySales(): Promise<Map<string, number[]>> {
	const res = await fetchOrders({ view: "seller", pageSize: ORDERS_SAMPLE });
	const now = Date.now();
	const map = new Map<string, number[]>();
	for (const order of res.records) {
		const raw = Number(order.createdAt ?? 0);
		if (!raw) continue;
		const ts = raw < 1e12 ? raw * 1000 : raw;
		const weeksAgo = Math.floor((now - ts) / WEEK_MS);
		if (weeksAgo < 0 || weeksAgo >= FORECAST_WEEKS) continue;
		const idx = FORECAST_WEEKS - 1 - weeksAgo;
		for (const it of order.items ?? []) {
			const pid = String(it.productId ?? "");
			if (!pid) continue;
			const arr = map.get(pid) ?? new Array<number>(FORECAST_WEEKS).fill(0);
			arr[idx] += Number(it.quantity ?? 0) || 0;
			map.set(pid, arr);
		}
	}
	return map;
}

export default function InventoryPage() {
	const [items, setItems] = React.useState<SellerProductListItem[]>([]);
	const [stockMap, setStockMap] = React.useState<
		Map<string, { stock: number; value: number }>
	>(new Map());
	const [salesMap, setSalesMap] = React.useState<Map<string, number[]>>(
		new Map(),
	);
	const [loading, setLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const filter = useSellerInventoryUiStore((state) => state.filter);
	const setFilter = useSellerInventoryUiStore((state) => state.setFilter);
	const search = useSellerInventoryUiStore((state) => state.search);
	const setSearch = useSellerInventoryUiStore((state) => state.setSearch);
	const {
		editProductId,
		expanded: editExpanded,
		openEdit,
		closeEdit,
		toggleExpanded: toggleEditExpanded,
	} = useProductEditDrawer();

	const refresh = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			// Products + sales velocity load concurrently. A failed orders fetch
			// degrades gracefully to "no forecast" rather than breaking the page.
			const [r, sales] = await Promise.all([
				fetchSellerProducts({ pageSize: PAGE_SIZE }),
				buildWeeklySales().catch(() => new Map<string, number[]>()),
			]);
			// Fast path: once the list endpoint ships aggregated `stock`
			// (+ `inventory_value`) per B-192, we can build the whole table from
			// the single list call — no per-product `/skus` fetch at all.
			const listHasStock = r.records.every((p) => p.stock != null);
			let entries: ReadonlyArray<
				readonly [string, { stock: number; value: number }]
			>;
			if (listHasStock) {
				entries = r.records.map(
					(p) =>
						[
							String(p.id),
							{
								stock: p.stock ?? 0,
								// Prefer the server aggregate; fall back to stock × 最低價
								// (approximate for multi-SKU/mixed-price products).
								value: p.inventory_value ?? (p.stock ?? 0) * priceOf(p),
							},
						] as const,
				);
			} else {
				// Fallback (today): stock lives on the SKUs, not the list row, so
				// pull each product's SKUs and aggregate. A failed fetch leaves
				// that product at 0 rather than failing the whole page.
				entries = await Promise.all(
					r.records.map(async (p) => {
						try {
							const skus = await fetchProductSkus(p.id);
							let stock = 0;
							let value = 0;
							for (const s of skus) {
								const qty = Number(s.stock ?? 0);
								const price = Number(s.sale_price ?? s.price ?? 0);
								stock += qty;
								value += qty * price;
							}
							return [String(p.id), { stock, value }] as const;
						} catch {
							return [String(p.id), { stock: 0, value: 0 }] as const;
						}
					}),
				);
			}
			// Commit list + aggregated stock in the same tick so React batches
			// them into a single render — the table never flashes fake "0 件 /
			// 庫存不足 / NT$ 0" rows between the list load and the SKU loads.
			setSalesMap(sales);
			setStockMap(new Map(entries));
			setItems(r.records);
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入庫存失敗");
		} finally {
			setLoading(false);
		}
	}, []);

	// After a product edit, refetch this page's list and invalidate the shared
	// seller-products family (nav badge + dashboard count). Bare `refresh` on
	// mount/manual-refresh deliberately skips the invalidation.
	const invalidateSellerProducts = useInvalidateSellerProducts();
	const handleMutated = React.useCallback(async () => {
		await refresh();
		void invalidateSellerProducts();
	}, [refresh, invalidateSellerProducts]);

	React.useEffect(() => {
		void refresh();
	}, [refresh]);

	const meta = React.useMemo(() => {
		const map = new Map<string, RowMeta>();
		for (const p of items) {
			const agg = stockMap.get(String(p.id));
			const stock = agg?.stock ?? 0;
			// Safety stock is a recommendation derived from recent sales velocity,
			// not a flat constant — so "庫存不足" reflects each product's real burn.
			const safe = suggestedSafeStock(salesMap.get(String(p.id)));
			map.set(String(p.id), {
				stock,
				safe,
				isLow: stock < safe,
				value: agg?.value ?? 0,
			});
		}
		return map;
	}, [items, stockMap, salesMap]);

	const totalCount = items.length;
	const lowCount = React.useMemo(
		() => Array.from(meta.values()).filter((m) => m.isLow).length,
		[meta],
	);
	const okCount = totalCount - lowCount;
	const totalValue = React.useMemo(
		() => Array.from(meta.values()).reduce((sum, m) => sum + m.value, 0),
		[meta],
	);

	// First load only: keep cards + table on skeleton/dash until the aggregated
	// data is ready, so we never paint fake zeros. On a manual refresh we keep
	// the existing (real) rows visible instead.
	const showSkeleton = loading && items.length === 0;

	const visible = React.useMemo(() => {
		const q = search.trim().toLowerCase();
		return items.filter((p) => {
			const m = meta.get(String(p.id));
			if (filter === "low" && !m?.isLow) return false;
			if (filter === "ok" && m?.isLow) return false;
			if (q && !p.name?.toLowerCase().includes(q)) return false;
			return true;
		});
	}, [items, meta, filter, search]);

	const mobileItems = React.useMemo<InventoryDisclosureItem[]>(
		() =>
			visible.flatMap((product) => {
				const row = meta.get(String(product.id));
				if (!row) return [];
				const weekly = salesMap.get(String(product.id));
				return [
					{
						product,
						row,
						weekly,
						eta: stockoutForecast(row.stock, weekly),
					},
				];
			}),
		[meta, salesMap, visible],
	);

	const renderMobileInventoryDetails = React.useCallback(
		(item: InventoryDisclosureItem) => (
			<>
				<div className="seller-mobile-disclosure__body">
					<div
						className="seller-mobile-disclosure__detail-row"
						title={`依近 ${FORECAST_WEEKS} 週平均週銷量 × ${SAFE_COVER_WEEKS} 週估算的建議安全庫存`}
					>
						<span>建議安全庫存</span>
						<strong>{item.row.safe} 件</strong>
					</div>
					<div className="seller-mobile-disclosure__detail-row">
						<span>週銷趨勢</span>
						<Sparkline values={item.weekly} />
					</div>
					<div className="seller-mobile-disclosure__detail-row">
						<span>預估售罄</span>
						<strong
							className={`seller-inventory__eta seller-inventory__eta--${item.eta.tone}`}
						>
							{item.eta.label}
						</strong>
					</div>
					<div className="seller-mobile-disclosure__detail-row">
						<span>庫存價值</span>
						<strong className="seller-inventory__value">
							NT$ {Math.round(item.row.value).toLocaleString()}
						</strong>
					</div>
				</div>
				<div className="seller-mobile-disclosure__actions">
					<Link
						href={`/seller-center/products/${item.product.id}/edit`}
						data-hkui-button=""
						className="seller-orders__action"
						onClick={(event) =>
							openDrawerOnPlainClick(event, () =>
								openEdit(String(item.product.id)),
							)
						}
					>
						調整庫存
					</Link>
				</div>
			</>
		),
		[openEdit],
	);

	const exportCsv = () => {
		const rows = [
			[
				"商品名稱",
				"當前庫存",
				"建議安全庫存",
				"週均銷量",
				"預估售罄",
				"狀態",
				"庫存價值",
			],
			...visible.map((p) => {
				const m = meta.get(String(p.id));
				const weekly = salesMap.get(String(p.id));
				const weeklyAvg =
					(weekly ?? []).reduce((a, b) => a + b, 0) / FORECAST_WEEKS;
				const eta = stockoutForecast(m?.stock ?? 0, weekly);
				return [
					p.name ?? "",
					String(m?.stock ?? 0),
					String(m?.safe ?? SAFE_STOCK_DEFAULT),
					weeklyAvg.toFixed(1),
					eta.label,
					m?.isLow ? "庫存不足" : "正常",
					String(m?.value ?? 0),
				];
			}),
		];
		downloadCsv(`inventory-${new Date().toISOString().slice(0, 10)}.csv`, rows);
	};

	return (
		<SellerPageShell
			className="seller-inventory"
			title="庫存管理"
			subtitle="監控商品庫存，設定安全預警"
			actions={
				<>
					<div className="seller-orders__search seller-inventory__header-search">
						<IconSearch />
						<input
							type="search"
							value={search}
							onChange={(e) => setSearch(e.target.value)}
							placeholder="搜尋商品名稱…"
							aria-label="搜尋庫存"
						/>
					</div>
					<Button
						type="button"
						className="seller-orders__btn"
						onClick={() => void refresh()}
						disabled={loading}
						isLoading={loading}
						loadingLabel="重新整理中…"
					>
						<AppIcon name="refresh" size={14} />
						重新整理
					</Button>
					<button
						type="button"
						className="seller-orders__btn seller-orders__btn--primary"
						onClick={exportCsv}
						disabled={visible.length === 0}
					>
						<IconDownload /> 匯出
					</button>
				</>
			}
		>
			<section
				className="seller-inventory__stats seller-summary-rail seller-summary-rail--compact-phone"
				aria-label="庫存摘要"
				// biome-ignore lint/a11y/noNoninteractiveTabindex: This horizontally scrollable summary needs a keyboard focus target.
				tabIndex={0}
			>
				<SummaryStatCard
					label="總商品數"
					value={showSkeleton ? "—" : totalCount.toLocaleString()}
					tint="primary"
					icon={<IconBoxes />}
				/>
				<SummaryStatCard
					label="庫存預警"
					value={showSkeleton ? "—" : lowCount.toLocaleString()}
					tint="accent"
					icon={<IconAlert />}
				/>
				<SummaryStatCard
					label="充足庫存"
					value={showSkeleton ? "—" : okCount.toLocaleString()}
					tint="info"
					icon={<IconCheck />}
				/>
				<SummaryStatCard
					label="總庫存值"
					value={showSkeleton ? "—" : `NT$ ${formatShort(totalValue)}`}
					tint="secondary"
					icon={<IconDollar />}
				/>
			</section>

			<section className="seller-inventory__panel">
				<div className="seller-inventory__mobile-toolbar">
					<div
						className="seller-inventory__mobile-toolbar-title"
						aria-live="polite"
					>
						顯示 {showSkeleton ? "—" : visible.length.toLocaleString()} 件
					</div>
					<div className="seller-orders__search seller-inventory__mobile-search">
						<IconSearch />
						<input
							type="search"
							value={search}
							onChange={(e) => setSearch(e.target.value)}
							placeholder="搜尋商品名稱…"
							aria-label="搜尋庫存"
						/>
					</div>
					<div className="seller-inventory__toolbar-actions">
						<select
							className="seller-inventory__select"
							value={filter}
							onChange={(e) => setFilter(e.target.value as InventoryFilter)}
							aria-label="篩選庫存狀態"
						>
							<option value="all">全部</option>
							<option value="low">不足</option>
							<option value="ok">充足</option>
						</select>
					</div>
				</div>

				{error && (
					<p
						className="seller-inventory__error seller-inventory__error--mobile"
						role="alert"
					>
						{error}
					</p>
				)}

				<div className="seller-inventory__card seller-mobile-disclosure-desktop">
					<div className="seller-inventory__toolbar">
						<h2 className="seller-inventory__toolbar-title">商品庫存</h2>
						<div className="seller-inventory__toolbar-actions">
							<select
								className="seller-inventory__select"
								value={filter}
								onChange={(e) => setFilter(e.target.value as InventoryFilter)}
								aria-label="篩選庫存狀態"
							>
								<option value="all">全部</option>
								<option value="low">僅庫存不足</option>
								<option value="ok">僅充足</option>
							</select>
						</div>
					</div>

					{error && (
						<p className="seller-inventory__error" role="alert">
							{error}
						</p>
					)}

					<div className="seller-orders__table-wrap">
						<table
							className="seller-inventory__table"
							data-mobile-layout="cards"
						>
							<thead>
								<tr>
									<th>商品名稱</th>
									<th className="is-center">當前庫存</th>
									<th className="is-center">建議安全庫存</th>
									<th className="is-center">週銷趨勢</th>
									<th className="is-center">預估售罄</th>
									<th className="is-center">狀態</th>
									<th className="is-right">庫存價值</th>
									<th className="is-center">操作</th>
								</tr>
							</thead>
							<tbody>
								{showSkeleton ? (
									<SellerInventorySkeletonRows rows={6} />
								) : visible.length === 0 ? (
									<tr>
										<td colSpan={8} className="seller-orders__empty">
											<SellerEmptyState
												variant="inventory"
												searchQuery={search}
												seamless
											/>
										</td>
									</tr>
								) : (
									visible.map((p) => {
										const m = meta.get(String(p.id));
										if (!m) return null;
										const weekly = salesMap.get(String(p.id));
										const eta = stockoutForecast(m.stock, weekly);
										return (
											<tr key={String(p.id)}>
												<td data-label="商品名稱">
													<strong>{p.name || "未命名商品"}</strong>
												</td>
												<td className="is-center" data-label="當前庫存">
													<span
														className={
															m.isLow
																? "seller-inventory__stock seller-inventory__stock--low"
																: "seller-inventory__stock"
														}
													>
														{m.stock}
													</span>
													<small> 件</small>
												</td>
												<td
													className="is-center seller-inventory__muted"
													data-label="建議安全庫存"
													title={`依近 ${FORECAST_WEEKS} 週平均週銷量 × ${SAFE_COVER_WEEKS} 週估算的建議安全庫存`}
												>
													{m.safe} 件
												</td>
												<td className="is-center" data-label="週銷趨勢">
													<Sparkline values={weekly} />
												</td>
												<td className="is-center" data-label="預估售罄">
													<span
														className={`seller-inventory__eta seller-inventory__eta--${eta.tone}`}
													>
														{eta.label}
													</span>
												</td>
												<td className="is-center" data-label="狀態">
													<span
														className={
															m.isLow
																? "seller-orders__status seller-orders__status--pending"
																: "seller-orders__status seller-orders__status--complete"
														}
													>
														{m.isLow ? "庫存不足" : "正常"}
													</span>
												</td>
												<td
													className="is-right seller-inventory__value"
													data-label="庫存價值"
												>
													NT$ {Math.round(m.value).toLocaleString()}
												</td>
												<td className="is-center" data-label="操作">
													<Link
														href={`/seller-center/products/${p.id}/edit`}
														// `.seller-orders__action` 的白字（0-1-0）會被 dark 的
														// `body a:not([data-hkui-button])`（0-2-2）換成 `--color-ink-200`。
														data-hkui-button=""
														className="seller-orders__action"
														onClick={(e) =>
															openDrawerOnPlainClick(e, () =>
																openEdit(String(p.id)),
															)
														}
													>
														調整
													</Link>
												</td>
											</tr>
										);
									})
								)}
							</tbody>
						</table>
					</div>
				</div>

				{showSkeleton ? (
					<div className="seller-mobile-disclosure-list">
						<div className="seller-orders__mobile-loading" role="status">
							載入庫存中…
						</div>
					</div>
				) : mobileItems.length === 0 ? (
					<div className="seller-mobile-disclosure-list">
						<div className="seller-orders__mobile-empty">
							<SellerEmptyState
								variant="inventory"
								searchQuery={search}
								seamless
							/>
						</div>
					</div>
				) : (
					<SellerMobileDisclosureList
						items={mobileItems}
						getKey={inventoryDisclosureKey}
						getSummary={inventoryDisclosureSummary}
						renderDetails={renderMobileInventoryDetails}
						ariaLabel="商品庫存清單"
						resetKey={`${filter}:${search}`}
					/>
				)}
			</section>

			<ProductEditDrawer
				key={editProductId ?? "closed"}
				productId={editProductId}
				expanded={editExpanded}
				onClose={closeEdit}
				onToggleExpanded={toggleEditExpanded}
				onMutated={handleMutated}
			/>
		</SellerPageShell>
	);
}

function inventoryDisclosureKey(item: InventoryDisclosureItem): string {
	return String(item.product.id);
}

function inventoryDisclosureSummary(
	item: InventoryDisclosureItem,
): SellerMobileDisclosureSummary {
	const name = item.product.name || "未命名商品";
	return {
		label: `庫存商品「${name}」`,
		leading: item.product.main_image ? (
			// biome-ignore lint/performance/noImgElement: Seller product images may be remote API URLs.
			<img
				src={ossImageUrl(item.product.main_image, "productThumb")}
				alt=""
				loading="lazy"
			/>
		) : (
			<SellerMobileDisclosureIcon name="category" />
		),
		title: name,
		meta: `${item.row.stock} 件`,
		status: (
			<span
				title={item.row.isLow ? "庫存不足" : "正常"}
				className={
					item.row.isLow
						? "seller-orders__status seller-orders__status--pending"
						: "seller-orders__status seller-orders__status--complete"
				}
			>
				{item.row.isLow ? "不足" : "正常"}
			</span>
		),
	};
}

/** Best available unit price for a list row — used only to approximate
 *  庫存價值 when the backend hasn't shipped `inventory_value` yet. */
function priceOf(p: SellerProductListItem): number {
	return Number(p.price_min ?? p.price ?? p.price_max ?? 0) || 0;
}

function formatShort(n: number): string {
	if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
	if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
	return Math.round(n).toLocaleString();
}

function IconBoxes() {
	return <AppIcon name="category" />;
}
function IconAlert() {
	return <AppIcon name="danger" />;
}
function IconCheck() {
	return <AppIcon name="check" />;
}
function IconDollar() {
	return <AppIcon name="wallet" />;
}
function IconDownload() {
	return <AppIcon name="download" />;
}
function IconSearch() {
	return <AppIcon name="search" />;
}
