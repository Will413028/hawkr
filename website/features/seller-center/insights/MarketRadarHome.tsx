"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { useMarketOverview } from "@/lib/hooks/useMarketOverview";
import { useMarketWatchlist } from "@/lib/hooks/useMarketWatchlist";
import type {
	CatalogMarketOption,
	MarketOverviewData,
	MarketOverviewItem,
	MarketOverviewWindowDays,
} from "@/lib/pricebook/repo";
import {
	InsightsInlineError,
	InsightsRefreshNotice,
} from "./InsightsDataState";
import { formatPriceLayerLabel, formatUnitLabel } from "./insightsFormat";
import { ObservedPriceSparkline } from "./ObservedPriceSparkline";
import { usePrefersReducedMotion } from "./usePrefersReducedMotion";

const RADAR_PAGE_SIZE = 20;
const RADAR_SKELETON_KEYS = [
	"leaf-one",
	"leaf-two",
	"leaf-three",
	"leaf-four",
	"leaf-five",
	"leaf-six",
	"leaf-seven",
] as const;
const RADAR_WINDOW_OPTIONS: Array<{
	value: MarketOverviewWindowDays;
	label: string;
}> = [
	{ value: 7, label: "近 7 天" },
	{ value: 30, label: "近 30 天" },
];

/** 這一列的單位——優先取品項自己的，退回清單層級的。
 *
 * 多來源之後同一份清單可能同時含元／公斤與元／台斤，所以頂層單位可能是空的，
 * 品項層級才是權威。 */
function itemUnit(
	item: MarketOverviewItem,
	data: MarketOverviewData,
): string | undefined {
	return item.unit ?? data.unit;
}

/** 價格可否顯示：幣別是台幣、且**有**單位。
 *
 * 🔴 這裡原本問的是「單位是不是 kg」。那在只有一種來源時等價，多來源之後
 * 會把合法的台斤價擋成「單位待確認」——整排雞蛋、雞肉、鴨鵝顯示成破折號，
 * 看起來像系統壞了。契約是「必須說得出單位」，不是「單位必須是公斤」。 */
function hasPriceContract(
	item: MarketOverviewItem,
	data: MarketOverviewData,
): boolean {
	const unit = itemUnit(item, data);
	return (item.currency ?? data.currency) === "TWD" && unit != null;
}

function formatRadarPrice(
	value: number | undefined,
	item: MarketOverviewItem,
	data: MarketOverviewData,
) {
	if (value == null || !hasPriceContract(item, data)) return "—";
	return `NT$ ${value.toLocaleString("zh-TW", { maximumFractionDigits: 1 })}`;
}

function formatSignedValue(value: number, suffix = "") {
	const sign = value > 0 ? "+" : "";
	return `${sign}${value.toLocaleString("zh-TW", {
		maximumFractionDigits: 1,
	})}${suffix}`;
}

function changeLabel(item: MarketOverviewItem, data: MarketOverviewData) {
	const change = item.change;
	if (!change || (change.absolute == null && change.percent == null))
		return "—";
	const pieces: string[] = [];
	if (change.absolute != null && hasPriceContract(item, data)) {
		pieces.push(`${formatSignedValue(change.absolute)} 元`);
	}
	if (change.percent != null) {
		pieces.push(`${formatSignedValue(change.percent, "%")}`);
	}
	return pieces.join(" · ") || "—";
}

function directionCopy(item: MarketOverviewItem) {
	switch (item.change?.direction) {
		case "up":
			return { className: "is-up", icon: "arrow-up" as const, label: "上升" };
		case "down":
			return {
				className: "is-down",
				icon: "arrow-down" as const,
				label: "下降",
			};
		case "flat":
			return { className: "is-flat", icon: null, label: "持平" };
		default:
			return { className: "is-unknown", icon: null, label: "變動待確認" };
	}
}

function dataStateCopy(item: MarketOverviewItem) {
	if (
		item.status === "unavailable" ||
		item.qualityReasonCodes.includes("NO_OBSERVED_DATA") ||
		item.latestActual?.avgPrice == null
	) {
		return "暫無官方成交資料";
	}
	if (
		item.freshness === "unavailable" ||
		item.qualityReasonCodes.includes("SOURCE_DATA_TOO_OLD")
	) {
		return "來源資料過舊";
	}
	if (item.freshness === "stale" || item.status === "stale") return "資料較舊";
	if (item.status === "closed") return "市場休市";
	if (item.status === "partial") return "部分資料";
	if (item.qualityReasonCodes.length > 0) return "資料待確認";
	return null;
}

function itemHref(
	item: MarketOverviewItem,
	windowDays: MarketOverviewWindowDays,
) {
	const query = new URLSearchParams({
		tab: "overview",
		scope: "crop",
		crop: item.cropCode,
		label: item.variantName
			? `${item.cropName}-${item.variantName}`
			: item.cropName,
		range: String(windowDays),
	});
	if (item.marketCode) query.set("market", item.marketCode);
	return `/seller-center/insights?${query.toString()}`;
}

export function RadarRowsSkeleton({ count = 5 }: { count?: number }) {
	return (
		<div
			className="market-radar__rows-skeleton"
			role="status"
			aria-busy="true"
			aria-label="正在載入官方行情"
		>
			{RADAR_SKELETON_KEYS.slice(0, count).map((key) => (
				<div className="market-radar__row" key={key} aria-hidden="true">
					<span className="market-radar__watch-button">
						<span className="seller-skeleton__bone size-5" />
					</span>
					<div className="market-radar__row-link">
						<span className="market-radar__identity">
							<span className="seller-skeleton__bone h-3.5 w-28" />
							<span className="seller-skeleton__bone h-3 w-20" />
						</span>
						<span className="market-radar__latest">
							<span className="seller-skeleton__bone h-4 w-20" />
							<span className="seller-skeleton__bone h-3 w-9" />
						</span>
						<span className="market-radar__change">
							<span className="seller-skeleton__bone h-3.5 w-20" />
						</span>
						<span className="market-radar__spark-cell">
							<span className="seller-skeleton__bone h-8 w-full" />
						</span>
						<span className="market-radar__date">
							<span className="seller-skeleton__bone h-3 w-20" />
						</span>
						<span className="market-radar__open-icon">
							<span className="seller-skeleton__bone size-4" />
						</span>
					</div>
				</div>
			))}
		</div>
	);
}

function RadarColumnHeader({ windowDays }: { windowDays: number }) {
	return (
		<div className="market-radar__column-head" aria-hidden="true">
			<span />
			<div>
				<span>品相</span>
				<span>最新官方價</span>
				<span>前一交易日變動</span>
				<span>近 {windowDays} 天</span>
				<span>最新交易日</span>
				<span />
			</div>
		</div>
	);
}

function RadarRow({
	item,
	data,
	windowDays,
	mutationPending,
	pendingTarget,
	onToggle,
}: {
	item: MarketOverviewItem;
	data: MarketOverviewData;
	windowDays: MarketOverviewWindowDays;
	mutationPending: boolean;
	pendingTarget: Pick<MarketOverviewItem, "cropCode" | "marketCode"> | null;
	onToggle: (item: MarketOverviewItem) => void;
}) {
	const effectiveItem = React.useMemo(
		() => ({
			...item,
			marketCode:
				item.marketCode ?? (data.scope === "all" ? data.marketCode : undefined),
		}),
		[item, data.scope, data.marketCode],
	);
	const direction = directionCopy(item);
	const stateCopy = dataStateCopy(item);
	const isCurrentMutation =
		mutationPending &&
		pendingTarget?.cropCode === effectiveItem.cropCode &&
		(pendingTarget.marketCode ?? "") === (effectiveItem.marketCode ?? "");
	const cannotRemove = item.isWatched && !item.watchTargetId;
	const marketLabel =
		item.marketName ??
		(effectiveItem.marketCode
			? `市場 ${effectiveItem.marketCode}`
			: "全部市場");
	const watchTargetLabel = `${item.cropName}${item.variantName ? `・${item.variantName}` : ""}（${marketLabel}，${item.cropCode}）`;
	const sparklineLabel = `${watchTargetLabel}近 ${windowDays} 天內有成交資料的官方平均價走勢`;
	const accessiblePrice =
		item.latestActual?.avgPrice != null && hasPriceContract(item, data)
			? `${formatUnitLabel(itemUnit(item, data))} ${formatRadarPrice(item.latestActual.avgPrice, item, data)}`
			: "暫無可用官方成交價";
	const accessibleChange = changeLabel(item, data);
	const accessibleDate = item.latestActual?.tradeDate ?? "尚無交易日";

	return (
		<li className="market-radar__row">
			<button
				type="button"
				className="market-radar__watch-button"
				aria-label={
					item.isWatched
						? `停止追蹤${watchTargetLabel}`
						: `追蹤${watchTargetLabel}`
				}
				aria-pressed={item.isWatched}
				disabled={mutationPending || cannotRemove}
				aria-busy={isCurrentMutation || undefined}
				title={cannotRemove ? "追蹤識別碼缺失，請重新整理後再試" : undefined}
				onClick={() => onToggle(effectiveItem)}
			>
				<AppIcon
					name={isCurrentMutation ? "spinner" : "star"}
					fill={item.isWatched}
					spin={isCurrentMutation}
					size={20}
				/>
			</button>
			<Link
				href={itemHref(effectiveItem, windowDays)}
				className="market-radar__row-link"
				aria-label={`查看${watchTargetLabel}的行情明細；最新官方價${accessiblePrice}；較前一交易日${direction.label}${accessibleChange === "—" ? "，變動資料待確認" : `，${accessibleChange}`}；最新交易日${accessibleDate}${stateCopy ? `；資料狀態${stateCopy}` : ""}`}
			>
				<span className="market-radar__identity">
					<strong>{item.cropName}</strong>
					<small>
						{item.variantName ? `${item.variantName} · ` : ""}
						{marketLabel} · {item.cropCode}
					</small>
					{stateCopy ? <em>{stateCopy}</em> : null}
				</span>
				<span className="market-radar__latest">
					<strong>
						{formatRadarPrice(item.latestActual?.avgPrice, item, data)}
					</strong>
					<small>
						{formatUnitLabel(itemUnit(item, data))}
						{layerLabel(item) != null ? ` · ${layerLabel(item)}` : ""}
					</small>
				</span>
				<span className={`market-radar__change ${direction.className}`}>
					<span>
						{direction.icon ? (
							<AppIcon name={direction.icon} size={15} fill />
						) : null}
						<strong>{direction.label}</strong>
					</span>
					<small>{changeLabel(item, data)}</small>
				</span>
				<span className={`market-radar__spark-cell ${direction.className}`}>
					<ObservedPriceSparkline points={item.points} label={sparklineLabel} />
				</span>
				<span className="market-radar__date">
					{item.latestActual ? (
						<time dateTime={item.latestActual.tradeDate}>
							{item.latestActual.tradeDate}
						</time>
					) : (
						"—"
					)}
				</span>
				<span className="market-radar__open-icon" aria-hidden="true">
					<AppIcon name="chevron-right" size={18} />
				</span>
			</Link>
		</li>
	);
}

function ContractNotice({ count }: { count: number }) {
	if (count === 0) return null;
	return (
		<p className="market-radar__contract-notice" role="status">
			<AppIcon name="warning" size={17} />
			回傳資料有 {count} 個欄位驗證問題；已保留有效資料並隱藏不可靠數值。
		</p>
	);
}

export function MarketRadarHome({
	marketCode,
	onMarketOptionsChange,
}: {
	marketCode?: string;
	onMarketOptionsChange: (options: CatalogMarketOption[]) => void;
}) {
	const [windowDays, setWindowDays] =
		React.useState<MarketOverviewWindowDays>(7);
	const [searchInput, setSearchInput] = React.useState("");
	const [committedSearch, setCommittedSearch] = React.useState("");
	const [page, setPage] = React.useState(1);
	const discoveryInputRef = React.useRef<HTMLInputElement>(null);
	const prefersReducedMotion = usePrefersReducedMotion();
	React.useEffect(() => {
		const timer = window.setTimeout(() => {
			setCommittedSearch(searchInput.trim());
			setPage(1);
		}, 280);
		return () => window.clearTimeout(timer);
	}, [searchInput]);
	const watchlistRequest = React.useMemo(
		() => ({
			scope: "watchlist" as const,
			windowDays,
			page: 1,
			pageSize: RADAR_PAGE_SIZE,
		}),
		[windowDays],
	);
	const allRequest = React.useMemo(
		() => ({
			scope: "all" as const,
			windowDays,
			marketCode,
			query: committedSearch || undefined,
			page,
			pageSize: RADAR_PAGE_SIZE,
		}),
		[committedSearch, marketCode, page, windowDays],
	);
	const watchlist = useMarketOverview(watchlistRequest);
	const all = useMarketOverview(allRequest);
	const watchMutation = useMarketWatchlist();

	const marketOptions = React.useMemo(() => {
		const byCode = new Map<string, CatalogMarketOption>();
		for (const option of [
			...(watchlist.data?.marketOptions ?? []),
			...(all.data?.marketOptions ?? []),
		]) {
			if (!byCode.has(option.marketCode)) byCode.set(option.marketCode, option);
		}
		return [...byCode.values()];
	}, [all.data?.marketOptions, watchlist.data?.marketOptions]);
	React.useEffect(() => {
		onMarketOptionsChange(marketOptions);
	}, [marketOptions, onMarketOptionsChange]);

	const updateWindow = (nextWindow: MarketOverviewWindowDays) => {
		setWindowDays(nextWindow);
		setPage(1);
	};
	const updateSearch = (value: string) => {
		setSearchInput(value);
	};
	const toggleItem = (item: MarketOverviewItem) => {
		void watchMutation.toggle(item, {
			windowDays,
		});
	};
	const focusDiscovery = () => {
		discoveryInputRef.current?.scrollIntoView({
			behavior: prefersReducedMotion ? "auto" : "smooth",
			block: "center",
		});
		discoveryInputRef.current?.focus({ preventScroll: true });
	};

	return (
		<section className="market-radar" aria-labelledby="market-radar-title">
			<header className="market-radar__hero">
				<div>
					<h2 id="market-radar-title">市場雷達</h2>
					<p>
						用最新官方成交價、前一交易日變動與小型走勢，快速查看你關心的品相。
					</p>
				</div>
				<fieldset className="market-radar__window-switch">
					<legend>走勢範圍</legend>
					{RADAR_WINDOW_OPTIONS.map((option) => (
						<button
							type="button"
							key={option.value}
							aria-pressed={windowDays === option.value}
							onClick={() => updateWindow(option.value)}
						>
							{option.label}
						</button>
					))}
				</fieldset>
				<p
					className="market-radar__source-note"
					role="status"
					aria-live="polite"
				>
					<AppIcon name="shield" size={17} />
					官方批發市場成交資料；各品相的最新交易日見列內
					{watchlist.refreshing || all.refreshing ? "；正在更新" : ""}
				</p>
			</header>

			<section
				className="market-radar__ledger"
				aria-labelledby="watchlist-title"
			>
				<div className="market-radar__section-head">
					<div>
						<h3 id="watchlist-title">我的追蹤</h3>
						<p>一進行情頁就先看到自選品相，不需重新挑選。</p>
					</div>
					<span>
						{watchlist.data?.totalItems ?? watchlist.data?.items.length ?? 0}{" "}
						個品相
					</span>
				</div>
				{watchlist.refreshError ? (
					<InsightsRefreshNotice
						label="我的追蹤"
						retrying={watchlist.refreshing}
						onRetry={watchlist.retry}
					/>
				) : null}
				{watchlist.loading ? <RadarRowsSkeleton /> : null}
				{watchlist.error ? (
					<InsightsInlineError
						title="無法載入我的追蹤"
						description="其他行情仍可使用；請重新載入這個區塊。"
						retryLabel="重新載入"
						retrying={watchlist.retrying}
						onRetry={watchlist.retry}
					/>
				) : null}
				{watchlist.data ? (
					<ContractNotice count={watchlist.data.contractIssues.length} />
				) : null}
				{watchlist.data && watchlist.data.items.length === 0 ? (
					<div className="market-radar__watch-empty">
						<span aria-hidden="true">
							<AppIcon name="star" size={23} />
						</span>
						<div>
							<strong>還沒有追蹤品相</strong>
							<p>從下方官方行情按下星號，之後會固定顯示在這裡。</p>
						</div>
						<Button
							type="button"
							variant="bordered"
							color="primary"
							onClick={focusDiscovery}
						>
							瀏覽可追蹤品相
						</Button>
					</div>
				) : null}
				{watchlist.data && watchlist.data.items.length > 0 ? (
					<>
						<RadarColumnHeader windowDays={windowDays} />
						<ul className="market-radar__rows">
							{watchlist.data.items.map((item) => (
								<RadarRow
									key={
										item.watchTargetId ??
										`${item.cropCode}:${item.marketCode ?? "all"}`
									}
									item={item}
									data={watchlist.data as MarketOverviewData}
									windowDays={windowDays}
									mutationPending={watchMutation.pending}
									pendingTarget={watchMutation.pendingTarget}
									onToggle={toggleItem}
								/>
							))}
						</ul>
					</>
				) : null}
			</section>

			<section
				className="market-radar__ledger"
				aria-labelledby="discovery-title"
			>
				<div className="market-radar__section-head market-radar__section-head--discovery">
					<div>
						<h3 id="discovery-title">全部官方行情</h3>
						<p>搜尋品相並一鍵加入追蹤；點擊整列可進入完整行情分析。</p>
					</div>
					<label className="market-radar__search">
						<span className="sr-only">搜尋可追蹤品相</span>
						<AppIcon name="search" size={19} />
						<input
							ref={discoveryInputRef}
							type="search"
							value={searchInput}
							placeholder="搜尋品相名稱或代碼"
							onChange={(event) => updateSearch(event.currentTarget.value)}
						/>
					</label>
				</div>
				{all.refreshError ? (
					<InsightsRefreshNotice
						label="全部官方行情"
						retrying={all.refreshing}
						onRetry={all.retry}
					/>
				) : null}
				{all.loading ? <RadarRowsSkeleton count={7} /> : null}
				{all.error ? (
					<InsightsInlineError
						title="全部行情暫時無法使用"
						description="我的追蹤仍可獨立使用；請重新載入這個區塊。"
						retryLabel="重新載入"
						retrying={all.retrying}
						onRetry={all.retry}
					/>
				) : null}
				{all.data ? (
					<ContractNotice count={all.data.contractIssues.length} />
				) : null}
				{all.data && all.data.items.length === 0 ? (
					<div className="market-radar__discovery-empty" role="status">
						<strong>
							{committedSearch
								? "找不到符合的官方行情"
								: "目前沒有可顯示的官方行情"}
						</strong>
						<p>
							{committedSearch
								? "請改用其他品相名稱或官方代碼搜尋。"
								: "缺值不會補成 0；請稍後重新載入。"}
						</p>
					</div>
				) : null}
				{all.data && all.data.items.length > 0 ? (
					<>
						<RadarColumnHeader windowDays={windowDays} />
						<ul className="market-radar__rows">
							{all.data.items.map((item) => (
								<RadarRow
									key={`${item.cropCode}:${item.marketCode ?? marketCode ?? "all"}`}
									item={item}
									data={all.data as MarketOverviewData}
									windowDays={windowDays}
									mutationPending={watchMutation.pending}
									pendingTarget={watchMutation.pendingTarget}
									onToggle={toggleItem}
								/>
							))}
						</ul>
					</>
				) : null}
				{all.data && (page > 1 || all.data.hasNext) ? (
					<nav className="market-radar__pagination" aria-label="全部行情分頁">
						<Button
							type="button"
							variant="light"
							disabled={page <= 1 || all.loading}
							startContent={<AppIcon name="chevron-left" size={17} />}
							onClick={() => setPage((current) => Math.max(1, current - 1))}
						>
							上一頁
						</Button>
						<span aria-live="polite">
							第 {page} 頁
							{all.data.totalItems == null
								? ""
								: ` · 共 ${all.data.totalItems} 個品相`}
						</span>
						<Button
							type="button"
							variant="light"
							disabled={!all.data.hasNext || all.loading}
							endContent={<AppIcon name="chevron-right" size={17} />}
							onClick={() => setPage((current) => current + 1)}
						>
							下一頁
						</Button>
					</nav>
				) : null}
			</section>
		</section>
	);
}

/** 價格層級標籤。🔴 不是裝飾：毛豬回的是活體拍賣均價，不是豬肉分切零售價，
 * 兩者水平差數倍——顯示價格卻不說層級就是讓賣家誤讀。
 * 舊資料沒有這個欄位，回 null 時整塊不渲染。 */
function layerLabel(item: MarketOverviewItem): string | null {
	return formatPriceLayerLabel(item.priceLayer);
}
