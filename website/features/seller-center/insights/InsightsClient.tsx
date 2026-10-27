"use client";

import dynamic from "next/dynamic";
import { parseAsString, parseAsStringEnum, useQueryStates } from "nuqs";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { ScPill } from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useForecastCurve } from "@/lib/hooks/useForecastCurve";
import { useObservedPriceHistory } from "@/lib/hooks/useObservedPriceHistory";
import type {
	CatalogMarketOption,
	PricebookSelection,
} from "@/lib/pricebook/repo";
import { AnimatedMetricValue } from "./AnimatedMetricValue";
import { CategoryRollupSummary } from "./CategoryRollupSummary";
import {
	CropCatalogSearch,
	type CropCatalogSearchHandle,
} from "./CropCatalogSearch";
import { ForecastBandChart } from "./ForecastBandChart";
import { resolveForecastDisplayData } from "./forecastDisplayData";
import { HistoricalPriceChart } from "./HistoricalPriceChart";
import {
	InsightsInlineError,
	InsightsMetricSkeleton,
	InsightsPanelSkeleton,
	InsightsRefreshNotice,
} from "./InsightsDataState";
import { InsightsEmptyWorkspace } from "./InsightsEmptyWorkspace";
import { InsightsMarketFilter } from "./InsightsMarketFilter";
import { InsightsSectionHeader } from "./InsightsSectionHeader";
import {
	INSIGHTS_TAB_IDS,
	type InsightsTabId,
	InsightsWorkspaceTabs,
} from "./InsightsWorkspaceTabs";
import {
	formatCurrencyUnitLabel,
	formatPrice,
	formatPriceLayerLabel,
	formatVolume,
} from "./insightsFormat";
import { MarketComparisonPanel } from "./MarketComparisonPanel";
import { MarketRadarHome } from "./MarketRadarHome";
import { ObservedPriceTable } from "./ObservedPriceTable";
import type { PartialCategorySelection } from "./partialCategorySelection";
import type { HistoryRange } from "./priceTrendChartMath";
import { SevenDayTable } from "./SevenDayTable";
import { STATUS_LABELS } from "./statusLabels";

const PriceTopographyPanel = dynamic(() => import("./PriceTopographyPanel"), {
	ssr: false,
	loading: () => (
		<div className="seller-price-lookup__map-loading" role="status">
			<div aria-hidden="true" />
			<strong>正在載入價格地圖…</strong>
		</div>
	),
});

const PriceTopographyPreview = dynamic(
	() =>
		import("./PriceTopographyPanel").then(
			(module) => module.PriceTopographyPreview,
		),
	{
		ssr: false,
		loading: () => (
			<div className="seller-price-lookup__map-loading" role="status">
				<div aria-hidden="true" />
				<strong>正在載入價格地圖…</strong>
			</div>
		),
	},
);

const INSIGHTS_HORIZON_VALUES = ["1", "2", "3", "4", "5", "6", "7"] as const;
type InsightsHorizonValue = (typeof INSIGHTS_HORIZON_VALUES)[number];
const INSIGHTS_MAP_VIEW_VALUES = ["contour", "height"] as const;
type InsightsMapView = (typeof INSIGHTS_MAP_VIEW_VALUES)[number];
const INSIGHTS_HISTORY_RANGE_VALUES = ["7", "30", "all"] as const;
type InsightsHistoryRangeValue = (typeof INSIGHTS_HISTORY_RANGE_VALUES)[number];
const INSIGHTS_SCOPE_VALUES = ["crop", "group"] as const;
type InsightsScopeValue = (typeof INSIGHTS_SCOPE_VALUES)[number];
const INSIGHTS_URL_OPTIONS = {
	history: "replace",
	shallow: true,
	scroll: false,
} as const;

function IconCalendar() {
	return <AppIcon name="calendar" />;
}
function IconChart() {
	return <AppIcon name="chart" />;
}
function IconGraph() {
	return <AppIcon name="graph" />;
}
function IconBox() {
	return <AppIcon name="box" />;
}

// 歷史行情卡片的 7/30/全部範圍切換放在區塊標頭右側，避免墊高圖表。
const RANGE_OPTIONS: Array<{ value: HistoryRange; label: string }> = [
	{ value: 7, label: "7 日" },
	{ value: 30, label: "30 日" },
	{ value: "all", label: "全部" },
];

export default function InsightsClient() {
	const [urlState, setUrlState] = useQueryStates(
		{
			tab: parseAsStringEnum<InsightsTabId>([...INSIGHTS_TAB_IDS]).withDefault(
				"overview",
			),
			scope: parseAsStringEnum<InsightsScopeValue>([...INSIGHTS_SCOPE_VALUES]),
			group: parseAsString,
			crop: parseAsString,
			category: parseAsString,
			market: parseAsString,
			label: parseAsString,
			day: parseAsStringEnum<InsightsHorizonValue>([
				...INSIGHTS_HORIZON_VALUES,
			]).withDefault("1"),
			view: parseAsStringEnum<InsightsMapView>([
				...INSIGHTS_MAP_VIEW_VALUES,
			]).withDefault("contour"),
			focusMarket: parseAsString,
			range: parseAsStringEnum<InsightsHistoryRangeValue>([
				...INSIGHTS_HISTORY_RANGE_VALUES,
			]).withDefault("30"),
		},
		INSIGHTS_URL_OPTIONS,
	);
	const activeTab = urlState.tab;
	const selection = React.useMemo<PricebookSelection | null>(() => {
		if (urlState.scope === "group" && urlState.group?.trim()) {
			const marketCode = urlState.market?.trim();
			return {
				kind: "group",
				groupCode: urlState.group.trim(),
				...(marketCode ? { marketCode } : {}),
			};
		}
		if (urlState.scope === "crop" && urlState.crop?.trim()) {
			const marketCode = urlState.market?.trim();
			return {
				kind: "crop",
				cropCode: urlState.crop.trim(),
				...(marketCode ? { marketCode } : {}),
			};
		}
		return null;
	}, [urlState.crop, urlState.group, urlState.market, urlState.scope]);
	const label =
		urlState.label?.trim().slice(0, 80) ||
		(selection?.kind === "group"
			? selection.groupCode
			: selection?.kind === "crop"
				? selection.cropCode
				: "");
	const selectedCategoryId = React.useMemo(() => {
		if (!urlState.category?.trim()) return undefined;
		const value = Number(urlState.category);
		return Number.isInteger(value) && value > 0 ? value : undefined;
	}, [urlState.category]);
	const historyRange: HistoryRange =
		urlState.range === "all" ? "all" : Number(urlState.range) === 7 ? 7 : 30;
	const horizonDays = Number(urlState.day);
	const selectedMapMarketCode =
		urlState.focusMarket ??
		(selection?.kind === "crop" ? (selection.marketCode ?? null) : null);
	const setActiveTab = React.useCallback(
		(tab: InsightsTabId) => {
			void setUrlState({ tab });
		},
		[setUrlState],
	);
	const commitSelection = React.useCallback(
		(
			nextSelection: PricebookSelection,
			nextLabel: string,
			categoryId: number,
		) => {
			if (nextSelection.kind === "group") {
				void setUrlState({
					scope: "group",
					group: nextSelection.groupCode,
					crop: null,
					category: String(categoryId),
					market: nextSelection.marketCode ?? null,
					label: nextLabel,
					focusMarket: null,
				});
				return;
			}
			if (nextSelection.kind === "crop") {
				void setUrlState({
					scope: "crop",
					group: null,
					crop: nextSelection.cropCode,
					category: String(categoryId),
					market: nextSelection.marketCode ?? null,
					label: nextLabel,
					focusMarket: null,
				});
			}
		},
		[setUrlState],
	);
	const catalogSelectorRef = React.useRef<CropCatalogSearchHandle>(null);
	const [scopeExpanded, setScopeExpanded] = React.useState(true);
	const [partialCategory, setPartialCategory] =
		React.useState<PartialCategorySelection | null>(null);
	const focusCatalogSelector = React.useCallback(() => {
		setScopeExpanded(true);
		window.requestAnimationFrame(() => {
			catalogSelectorRef.current?.focus();
		});
	}, []);
	const setMapHorizonDays = React.useCallback(
		(day: number) => {
			void setUrlState({ day: String(day) as InsightsHorizonValue });
		},
		[setUrlState],
	);
	const setMapView = React.useCallback(
		(view: InsightsMapView) => {
			void setUrlState({ view });
		},
		[setUrlState],
	);
	const setMapMarketFocus = React.useCallback(
		(marketCode: string) => {
			void setUrlState({ focusMarket: marketCode });
		},
		[setUrlState],
	);
	const [radarMarketEntries, setRadarMarketEntries] = React.useState<
		CatalogMarketOption[]
	>([]);
	const beginCategorySelection = React.useCallback(
		(categoryId: number | undefined) => {
			void setUrlState({
				scope: null,
				group: null,
				crop: null,
				category: categoryId == null ? null : String(categoryId),
				market: null,
				label: null,
				focusMarket: null,
			});
		},
		[setUrlState],
	);

	const history = useObservedPriceHistory(selection);
	const forecast = useForecastCurve(selection);

	// 歷史行情卡片的時間範圍寫入 URL，重新整理或分享連結都保留研究上下文。
	const setHistoryRange = React.useCallback(
		(range: HistoryRange) => {
			void setUrlState({
				range: String(range) as InsightsHistoryRangeValue,
			});
		},
		[setUrlState],
	);
	const historyRangeToggle = (
		<fieldset
			aria-label="時間範圍"
			className="seller-price-lookup__chart-range m-0 flex min-w-0 flex-nowrap gap-[var(--hkui-space-2)] border-0 p-0"
		>
			{RANGE_OPTIONS.map((opt) => (
				<ScPill
					key={String(opt.value)}
					as="button"
					aria-pressed={historyRange === opt.value}
					aria-controls="observed-price-history-panel"
					active={historyRange === opt.value}
					onClick={() => setHistoryRange(opt.value)}
				>
					{opt.label}
				</ScPill>
			))}
		</fieldset>
	);

	const forecastDisplay = resolveForecastDisplayData(forecast.data);
	// 切換到一條沒有該能力的序列時，目前所在的分頁會整個消失——URL 仍指向它，
	// 畫面卻是一塊空白，看起來像壞了。把使用者帶回總覽，那是所有序列都有的。
	const capabilities = forecastDisplay.capabilities;
	React.useEffect(() => {
		if (!capabilities) return;
		if (activeTab === "spatial" && !capabilities.spatial)
			setActiveTab("overview");
		if (activeTab === "markets" && !capabilities.marketComparison)
			setActiveTab("overview");
	}, [activeTab, capabilities, setActiveTab]);
	const { status, isPriceReady, points, marketComparison, marketRoster } =
		forecastDisplay;
	// 市場選項只接受明確回報目前 cropCode 的預測 payload；換品項期間不能把
	// 上一個品項的市場清單帶進新的選填篩選。
	const cropMarketEntries =
		selection?.kind === "crop" &&
		forecast.data?.resolvedCropCode === selection.cropCode
			? marketRoster
			: [];
	const cropMarketComparison =
		selection?.kind === "crop" &&
		forecast.data?.resolvedCropCode === selection.cropCode
			? marketComparison
			: [];
	const groupMarketEntries =
		selection?.kind === "group" &&
		forecast.data?.resolvedCropGroupCode === selection.groupCode
			? marketRoster
			: [];
	const groupMarketComparison =
		selection?.kind === "group" &&
		forecast.data?.resolvedCropGroupCode === selection.groupCode
			? marketComparison
			: [];
	// D+1 指標及跨市場比較都只讀真正的 horizonDays=1，不能在休市時偷偷改用
	// D+2，否則畫面雖有數字，日期語意卻已經錯了。
	const horizon1 = points.find((p) => p.horizonDays === 1) ?? null;
	const hasD1Median =
		isPriceReady &&
		horizon1 != null &&
		typeof horizon1.p50 === "number" &&
		Number.isFinite(horizon1.p50);
	const uncertainty =
		horizon1?.p10 != null && horizon1?.p90 != null
			? horizon1.p90 - horizon1.p10
			: undefined;
	const hasD1Band =
		hasD1Median &&
		typeof uncertainty === "number" &&
		Number.isFinite(uncertainty) &&
		uncertainty >= 0;
	const latestActual = [...history.observations]
		.filter((observation) => observation.avgPrice != null)
		.sort((a, b) => b.tradeDate.localeCompare(a.tradeDate))[0];
	// 任一回傳列缺成交量時就不拼成一個看似完整的總量；缺值維持「—」。
	const hasCompleteVolume =
		history.observations.length > 0 &&
		history.observations.every((observation) => observation.volumeKg != null);
	const recentVolume = hasCompleteVolume
		? history.observations.reduce(
				(sum, observation) => sum + (observation.volumeKg ?? 0),
				0,
			)
		: undefined;

	const statusLabel = status ? (STATUS_LABELS[status] ?? status) : null;
	const forecastPriceStateLabel =
		forecastDisplay.issue === "invalid_points"
			? "預測資料格式無法驗證"
			: forecastDisplay.issue === "no_price_ready_point"
				? "目前沒有可顯示的預測價格"
				: statusLabel;
	const isAllMarketCrop = selection?.kind === "crop" && !selection.marketCode;
	const currentMarketName =
		selection?.kind === "crop"
			? selection.marketCode
				? (cropMarketEntries.find(
						(market) => market.marketCode === selection.marketCode,
					)?.marketName ?? selection.marketCode)
				: "全部市場"
			: selection?.kind === "group" && selection.marketCode
				? (groupMarketEntries.find(
						(market) => market.marketCode === selection.marketCode,
					)?.marketName ?? selection.marketCode)
				: "大類彙整參考";
	const latestActualDateText = history.loading
		? "讀取中"
		: history.error
			? "載入失敗"
			: (latestActual?.tradeDate ?? "尚無資料");
	const collapsedScopeSummary = selection
		? `${label} · ${currentMarketName}`
		: partialCategory
			? `${partialCategory.label} · 尚未選到細項`
			: "尚未選擇商品分類";
	const actualMetricContext = history.loading
		? "正在讀取官方行情"
		: history.error
			? "官方行情載入失敗"
			: history.refreshError
				? latestActual
					? `${latestActual.tradeDate} · 更新失敗，顯示上次資料`
					: "更新失敗，顯示上次資料"
				: latestActual
					? selection?.kind === "group"
						? selection.marketCode
							? `${latestActual.tradeDate} · ${currentMarketName}`
							: `${latestActual.tradeDate} · 大類與市場成交量加權`
						: isAllMarketCrop
							? `${latestActual.tradeDate} · 全市場成交量加權`
							: `${latestActual.tradeDate} · ${currentMarketName}`
					: "尚無可用實際價";
	const forecastMetricContext = forecast.loading
		? "正在讀取條件式預測"
		: forecast.error
			? "模型預測載入失敗"
			: forecast.refreshError
				? horizon1?.targetDate
					? `${horizon1.targetDate} · 更新失敗，顯示上次資料`
					: "更新失敗，顯示上次資料"
				: !isPriceReady
					? (forecastPriceStateLabel ?? "尚無可用預測")
					: !horizon1
						? "D+1 尚未回傳；未以 D+2 代替"
						: !hasD1Median
							? `D+1：${STATUS_LABELS[horizon1.status] ?? "尚無模型中位價"}`
							: horizon1.targetDate;
	const uncertaintyMetricContext = forecast.loading
		? "正在讀取 D+1 預測區間"
		: forecast.error
			? "模型預測載入失敗"
			: !isPriceReady
				? (forecastPriceStateLabel ?? "尚無可用預測")
				: !horizon1
					? "D+1 尚未回傳；未以 D+2 代替"
					: !hasD1Band
						? "P10～P90 尚未完整提供"
						: "P10～P90，非官方上／下價";
	const selectionIdentity =
		selection?.kind === "group"
			? `大類代碼 ${selection.groupCode}`
			: selection?.kind === "crop"
				? selection.marketCode
					? `品項代碼 ${selection.cropCode} · 市場代碼 ${selection.marketCode}`
					: `品項代碼 ${selection.cropCode} · 全市場成交量加權彙整`
				: "";
	const forecastStatusText = forecast.loading
		? "預測讀取中"
		: forecast.error
			? "預測載入失敗"
			: forecast.refreshError
				? "更新失敗，顯示上次資料"
				: forecast.refreshing
					? "預測更新中"
					: (forecastPriceStateLabel ?? "尚無預測狀態");
	const forecastNotice =
		forecastDisplay.issue === "invalid_points"
			? "預測回傳格式無法驗證，為避免顯示錯誤價格，本頁已暫停呈現預測數值。"
			: forecastDisplay.issue === "no_price_ready_point"
				? "目前回傳沒有任何可用的模型中位價，缺值維持「—」。"
				: status === "partial"
					? "部分日期或子品項缺少可用預測，缺值維持「—」。"
					: status === "stale"
						? "模型資料已過舊；目前資料契約未提供來源截止時間，無法判定更新到哪一天。"
						: null;
	// MarketComparisonPanel/SevenDayTable both read ONLY from forecast.data
	// (never history) — gate them on forecast's own loading/error, not the
	// page-level combined `loading`. Gating on the combined flag would mask
	// useForecastCurve's placeholderData: keepPreviousData (TanStack Query
	// v5: isLoading = isPending && isFetching, and isPending flips false once
	// placeholder data exists, so forecast.loading alone correctly stays
	// false during a market-switch refetch) behind history.loading, which
	// has no placeholderData and re-enters loading on every selection change
	// (its query key also includes marketCode) — reintroducing a "flash to
	// loading" on every market switch even though forecast already has
	// something to show.
	const forecastReady =
		selection != null && !forecast.loading && !forecast.error && isPriceReady;

	const selectMarket = (marketCode: string | undefined) => {
		if (selection == null) {
			void setUrlState({
				market: marketCode ?? null,
				focusMarket: null,
			});
			return;
		}
		void setUrlState({
			market: marketCode ?? null,
			focusMarket: marketCode ?? null,
		});
	};

	return (
		<SellerPageShell
			className="seller-price-lookup"
			title="市場行情工作台"
			subtitle="查看市場價格、七日預測與各地差異。"
		>
			<details
				className="seller-price-lookup__scope-picker"
				open={scopeExpanded}
				onToggle={(event) => setScopeExpanded(event.currentTarget.open)}
			>
				<summary className="seller-price-lookup__scope-toggle">
					<span className="seller-price-lookup__scope-toggle-copy">
						<strong>篩選條件</strong>
						<small className="seller-price-lookup__scope-collapsed-summary">
							{collapsedScopeSummary}
						</small>
					</span>
					<span className="seller-price-lookup__scope-toggle-action">
						<span className="seller-price-lookup__scope-toggle-open-label">
							收合
						</span>
						<span className="seller-price-lookup__scope-toggle-closed-label">
							展開
						</span>
						<AppIcon name="chevron-down" size={18} />
					</span>
				</summary>

				<div className="seller-price-lookup__scope-content">
					<div className="seller-price-lookup__scope-fields has-market-filter">
						<div className="seller-price-lookup__scope-control">
							<span className="seller-price-lookup__scope-label">商品分類</span>
							<CropCatalogSearch
								ref={catalogSelectorRef}
								selection={selection}
								selectedCategoryId={selectedCategoryId}
								selectedLabel={selection ? label : null}
								onResolve={commitSelection}
								onCategoryChange={beginCategorySelection}
								onPartialCategoryChange={setPartialCategory}
							/>
						</div>
						<InsightsMarketFilter
							marketCode={
								selection?.kind === "crop"
									? selection.marketCode
									: selection?.kind === "group"
										? selection.marketCode
										: selection == null
											? (urlState.market ?? undefined)
											: undefined
							}
							entries={
								selection?.kind === "crop"
									? cropMarketEntries
									: selection?.kind === "group"
										? groupMarketEntries
										: radarMarketEntries
							}
							loading={
								(selection?.kind === "crop" || selection?.kind === "group") &&
								forecast.loading
							}
							onChange={selectMarket}
						/>
					</div>
					{selection ? (
						<dl
							className="seller-price-lookup__scope-summary"
							aria-label="目前研究範圍"
						>
							<div>
								<dt>市場範圍</dt>
								<dd>{currentMarketName}</dd>
							</div>
							<div>
								<dt>官方最新交易日</dt>
								<dd>
									{latestActual ? (
										<time dateTime={latestActual.tradeDate}>
											{latestActualDateText}
										</time>
									) : (
										latestActualDateText
									)}
									{history.refreshError ? (
										<small className="seller-price-lookup__scope-meta-warning">
											更新失敗，顯示上次資料
										</small>
									) : null}
								</dd>
							</div>
						</dl>
					) : null}
					{selection?.kind === "group" ? (
						<p className="seller-price-lookup__scope-disclaimer">
							大類彙整只供參考，不代表單一品項或市場價格。
						</p>
					) : null}
					{selection && forecastNotice ? (
						<p className="seller-price-lookup__scope-notice" role="status">
							{forecastNotice}
						</p>
					) : null}
				</div>
			</details>

			<InsightsWorkspaceTabs
				activeTab={activeTab}
				onChange={setActiveTab}
				capabilities={forecastDisplay.capabilities}
			/>

			<div
				key={activeTab}
				id="insights-workspace-panel"
				role="tabpanel"
				aria-labelledby={`insights-tab-${activeTab}`}
				// biome-ignore lint/a11y/noNoninteractiveTabindex: the ARIA tab pattern needs a stable focus target when a panel has no interactive child
				tabIndex={0}
			>
				{selection == null ? (
					activeTab === "overview" ? (
						partialCategory ? (
							<CategoryRollupSummary
								categoryId={partialCategory.categoryId}
								level={partialCategory.level}
								label={partialCategory.label}
								historyRange={historyRange}
								onHistoryRangeChange={setHistoryRange}
							/>
						) : (
							<MarketRadarHome
								marketCode={urlState.market ?? undefined}
								onMarketOptionsChange={setRadarMarketEntries}
							/>
						)
					) : activeTab === "spatial" ? (
						<PriceTopographyPreview onRequestSearch={focusCatalogSelector} />
					) : (
						<InsightsEmptyWorkspace
							activeTab={activeTab}
							onRequestSearch={focusCatalogSelector}
						/>
					)
				) : (
					<div className="seller-price-lookup__panel-stack">
						{activeTab === "overview" ? (
							<section
								className="seller-price-lookup__kpis"
								aria-label="行情摘要"
								aria-busy={history.loading || forecast.loading}
							>
								<dl className="seller-price-lookup__metric-ledger">
									<div className="seller-price-lookup__metric seller-price-lookup__metric--primary">
										<dt>
											<span aria-hidden="true">
												<IconCalendar />
											</span>
											{selection.kind === "group"
												? selection.marketCode
													? "最新市場官方平均價"
													: "最新大類彙整價"
												: isAllMarketCrop
													? "最新實際彙整價"
													: "最新官方平均價"}
										</dt>
										<dd className="seller-price-lookup__metric-value">
											{history.loading ? (
												<InsightsMetricSkeleton label="官方實際行情" />
											) : history.error ? (
												"—"
											) : (
												<AnimatedMetricValue
													value={latestActual?.avgPrice}
													format={formatPrice}
												/>
											)}
										</dd>
										<dd className="seller-price-lookup__metric-context">
											{actualMetricContext}
										</dd>
									</div>

									<div className="seller-price-lookup__metric seller-price-lookup__metric--forecast">
										<dt>
											<span aria-hidden="true">
												<IconChart />
											</span>
											D+1 模型中位價
										</dt>
										<dd className="seller-price-lookup__metric-value">
											{forecast.loading ? (
												<InsightsMetricSkeleton label="D+1 模型中位價" />
											) : forecast.error || !hasD1Median ? (
												"—"
											) : (
												<AnimatedMetricValue
													value={horizon1?.p50}
													format={formatPrice}
												/>
											)}
										</dd>
										<dd className="seller-price-lookup__metric-context">
											{forecastMetricContext}
										</dd>
									</div>

									<div className="seller-price-lookup__metric">
										<dt>
											<span aria-hidden="true">
												<IconGraph />
											</span>
											D+1 預測區間寬度
										</dt>
										<dd className="seller-price-lookup__metric-value">
											{forecast.loading ? (
												<InsightsMetricSkeleton label="D+1 預測區間寬度" />
											) : forecast.error || !hasD1Band ? (
												"—"
											) : (
												<AnimatedMetricValue
													value={uncertainty}
													format={formatPrice}
												/>
											)}
										</dd>
										<dd className="seller-price-lookup__metric-context">
											{uncertaintyMetricContext}
										</dd>
									</div>

									<div className="seller-price-lookup__metric">
										<dt>
											<span aria-hidden="true">
												<IconBox />
											</span>
											近 30 日成交量
										</dt>
										<dd className="seller-price-lookup__metric-value">
											{selection.kind === "group" ? (
												"—"
											) : history.loading ? (
												<InsightsMetricSkeleton label="近 30 日成交量" />
											) : history.error ? (
												"—"
											) : (
												<AnimatedMetricValue
													value={recentVolume}
													format={formatVolume}
												/>
											)}
										</dd>
										<dd className="seller-price-lookup__metric-context">
											{selection.kind === "group"
												? "大類層級不提供"
												: history.loading
													? "正在讀取官方成交量"
													: history.error
														? "官方成交量載入失敗"
														: "市場交易活動，非賣家庫存"}
										</dd>
									</div>
								</dl>
							</section>
						) : null}

						{activeTab === "overview" || activeTab === "forecast" ? (
							<div
								className={
									activeTab === "forecast"
										? "seller-price-lookup__charts seller-price-lookup__charts--single"
										: "seller-price-lookup__charts"
								}
							>
								{activeTab === "overview" ? (
									<section
										className="seller-card seller-price-lookup__section"
										aria-label="官方批發實際行情"
									>
										<InsightsSectionHeader
											title="官方批發實際行情"
											description={
												selection.kind === "group"
													? selection.marketCode
														? "農業部批發市場每日實際平均價；只看所選市場，為該市場內子品項成交量加權彙整。"
														: "農業部批發市場每日實際平均價；大類由後端依子品項與市場成交量分段彙整，不混用官方上、中、下價。"
													: isAllMarketCrop
														? "農業部批發市場每日實際平均價；全市場由後端依成交量加權彙整，不把各市場上、中、下價混成全國值。"
														: "農業部批發市場每日實際價格；圖表顯示平均價，明細另列官方上、中、下價。"
											}
											action={historyRangeToggle}
										/>
										<section
											id="observed-price-history-panel"
											aria-label={`${historyRange === "all" ? "全部可用" : historyRange}日官方平均價`}
										>
											{history.loading && !history.observations.length ? (
												<InsightsPanelSkeleton
													variant="chart"
													label="正在載入官方實際行情圖表…"
												/>
											) : history.error ? (
												<InsightsInlineError
													title="官方實際行情載入失敗"
													description="這不影響右側的模型預測；可只重新載入官方行情。"
													retryLabel="重新載入官方行情"
													retrying={history.retrying}
													onRetry={history.retry}
												/>
											) : (
												<>
													{history.refreshError ? (
														<InsightsRefreshNotice
															label="官方實際行情"
															retrying={history.refreshing}
															onRetry={history.retry}
														/>
													) : null}
													<HistoricalPriceChart
														observations={history.observations}
														range={historyRange}
													/>
												</>
											)}
										</section>
									</section>
								) : null}

								<section
									className="seller-card seller-price-lookup__section"
									aria-label="七日條件式價格預測"
								>
									<InsightsSectionHeader
										title="D+1～D+7 條件式預測"
										description="P50 為模型中位價，色帶為 P10～P90；只描述市場有交易時的價格區間。"
									/>
									{forecast.loading ? (
										<InsightsPanelSkeleton
											variant="chart"
											chartKind="forecast"
											label="正在載入模型預測圖表…"
										/>
									) : forecast.error ? (
										<InsightsInlineError
											title="條件式預測載入失敗"
											description="官方實際行情仍可使用；可只重新載入模型預測。"
											retryLabel="重新載入模型預測"
											retrying={forecast.retrying}
											onRetry={forecast.retry}
										/>
									) : !isPriceReady ? (
										<p className="seller-price-lookup__estimate-unavailable">
											{forecastPriceStateLabel ?? "目前無法估價"}
										</p>
									) : (
										<>
											{forecast.refreshError ? (
												<InsightsRefreshNotice
													label="條件式預測"
													retrying={forecast.refreshing}
													onRetry={forecast.retry}
												/>
											) : null}
											<ForecastBandChart forecastPoints={points} />
										</>
									)}
								</section>
							</div>
						) : null}

						{activeTab === "spatial" ? (
							<PriceTopographyPanel
								selection={selection}
								label={label}
								horizonDays={horizonDays}
								requestedMode={urlState.view}
								selectedMarketCode={selectedMapMarketCode}
								onHorizonDaysChange={setMapHorizonDays}
								onModeChange={setMapView}
								onMarketSelect={setMapMarketFocus}
							/>
						) : null}

						{activeTab === "markets" ? (
							<>
								<section
									className="seller-card seller-price-lookup__section"
									aria-label="官方實際行情明細"
								>
									<InsightsSectionHeader
										title="實際行情明細"
										description="精確值替代表格；缺值顯示「—」，不以零或模型預測補齊。"
									/>
									{history.loading ? (
										<InsightsPanelSkeleton
											variant="table"
											label="正在載入官方實際行情明細…"
										/>
									) : history.error ? (
										<InsightsInlineError
											title="官方實際行情明細載入失敗"
											description="跨市場模型比較仍可使用；可只重新載入官方行情。"
											retryLabel="重新載入官方行情"
											retrying={history.retrying}
											onRetry={history.retry}
										/>
									) : (
										<>
											{history.refreshError ? (
												<InsightsRefreshNotice
													label="官方實際行情"
													retrying={history.refreshing}
													onRetry={history.retry}
												/>
											) : null}
											<ObservedPriceTable
												observations={history.observations}
												currentMarketCode={
													selection.kind === "crop"
														? selection.marketCode
														: undefined
												}
												currentMarketName={
													selection.kind === "crop"
														? currentMarketName
														: undefined
												}
												aggregateMarketLabel={
													selection.kind === "group"
														? selection.marketCode
															? currentMarketName
															: "大類彙整"
														: "全部市場彙整"
												}
											/>
										</>
									)}
								</section>

								{selection.kind === "crop" || selection.kind === "group" ? (
									<section
										className="seller-card seller-price-lookup__section"
										aria-label="跨市場比較"
									>
										<InsightsSectionHeader
											title={
												selection.kind === "group"
													? "跨市場比較"
													: "同品項跨市場比較"
											}
											description={
												selection.kind === "group"
													? selection.marketCode
														? "比較大類底下各品項在其他市場的 D+1 模型中位價平均；點選其他市場可切換完整行情。"
														: "上方總覽是大類與市場成交量加權彙整；點選任一市場後，只看該市場的完整行情。"
													: selection.marketCode
														? "只比較相同品項的 D+1 模型中位價；點選其他市場可切換完整行情。"
														: "上方總覽是全市場成交量加權彙整；點選任一市場後，只看該市場的完整行情。"
											}
										/>
										{selection.kind === "group" ? (
											<p
												className="seller-price-lookup__scope-notice"
												role="status"
											>
												不同品項預測分別平均，非嚴謹統計區間，僅供參考。
											</p>
										) : null}
										{forecastReady ? (
											<>
												{forecast.refreshError ? (
													<InsightsRefreshNotice
														label="跨市場模型比較"
														retrying={forecast.refreshing}
														onRetry={forecast.retry}
													/>
												) : null}
												<MarketComparisonPanel
													entries={
														selection.kind === "crop"
															? cropMarketComparison
															: groupMarketComparison
													}
													currentMarketCode={selection.marketCode}
													currentMarketName={
														selection.marketCode
															? currentMarketName
															: "全部市場（成交量加權彙整）"
													}
													currentP50={horizon1?.p50}
													onSelectMarket={selectMarket}
													unitLabel={
														selection.kind === "group"
															? `大類子品項、D+1 模型中位價平均（${formatCurrencyUnitLabel(
																	forecastDisplay.unit,
																)}）`
															: undefined
													}
												/>
											</>
										) : forecast.loading ? (
											<InsightsPanelSkeleton
												variant="comparison"
												label="正在載入跨市場比較…"
											/>
										) : forecast.error ? (
											<InsightsInlineError
												title="跨市場模型比較載入失敗"
												description="上方官方實際行情明細仍可使用。"
												retryLabel="重新載入模型預測"
												retrying={forecast.retrying}
												onRetry={forecast.retry}
											/>
										) : (
											<p className="seller-price-lookup__estimate-unavailable">
												目前無法載入跨市場比較。
											</p>
										)}
									</section>
								) : null}
							</>
						) : null}

						{activeTab === "forecast" ? (
							<section
								className="seller-card seller-price-lookup__section"
								aria-label="七日逐日狀態"
							>
								<InsightsSectionHeader
									title="七日預測明細"
									description="P10、P50、P90 都是模型預測分位數，不是官方上、中、下價。"
								/>
								{forecastReady ? (
									<SevenDayTable points={points} unit={forecastDisplay.unit} />
								) : forecast.loading ? (
									<InsightsPanelSkeleton
										variant="table"
										tableKind="forecast"
										label="正在載入七日預測明細…"
									/>
								) : forecast.error ? (
									<p className="seller-price-lookup__estimate-unavailable">
										模型預測載入失敗；請使用上方的重新載入操作。
									</p>
								) : (
									<p className="seller-price-lookup__estimate-unavailable">
										目前無法載入七日預測明細。
									</p>
								)}
							</section>
						) : null}

						{activeTab === "quality" ? (
							<>
								<section className="seller-card seller-price-lookup__section">
									<InsightsSectionHeader
										title="資料來源與可用性"
										description="把官方實際行情與模型預測分開標示；後端尚未提供的 freshness 與 lineage 不自行推算。"
									/>
									<dl className="seller-price-lookup__quality-grid">
										<div>
											<dt>實際價來源</dt>
											{/* 🔴 原本寫死「農業部批發市場交易行情」。這一頁正是賣家
											    用來核對資料可信度的地方，對雞蛋產地價或白米零售價寫死
											    批發市場就是雙重錯誤宣告——來源錯、層級也錯。 */}
											<dd>
												{formatPriceLayerLabel(forecastDisplay.priceLayer) ??
													"農業部批發市場交易行情"}
											</dd>
											<small>
												{history.error
													? "載入失敗"
													: history.loading
														? "讀取中"
														: history.refreshError
															? "更新失敗，顯示上次資料"
															: history.observations.length === 0
																? "本範圍無成交紀錄"
																: "可用"}
												{" · "}最新交易日{" "}
												{latestActual ? (
													<time dateTime={latestActual.tradeDate}>
														{latestActual.tradeDate}
													</time>
												) : (
													"—"
												)}
											</small>
										</div>
										<div>
											<dt>市場範圍</dt>
											<dd>{currentMarketName}</dd>
											<small>{selectionIdentity}</small>
										</div>
										<div>
											<dt>預測模型版本</dt>
											<dd>
												{forecast.error
													? "載入失敗"
													: forecast.data?.modelVersion || "未提供"}
											</dd>
											<small>
												預測狀態：{forecastStatusText}
												{forecast.refreshError
													? " · 更新失敗，顯示上次資料"
													: ""}
											</small>
										</div>
										<div>
											<dt>資料新鮮度</dt>
											<dd>尚未提供</dd>
											<small>不把未知狀態推定為 fresh</small>
										</div>
										<div>
											<dt>價格單位</dt>
											<dd>{formatCurrencyUnitLabel(forecastDisplay.unit)}</dd>
											<small>實際價與模型預測使用相同單位</small>
										</div>
										<div>
											<dt>缺值規則</dt>
											<dd>維持「—」</dd>
											<small>休市、缺值與失敗都不補零</small>
										</div>
									</dl>
								</section>
								<aside
									className="seller-price-lookup__semantics"
									aria-labelledby="insights-semantics-title"
								>
									<h2
										id="insights-semantics-title"
										className="seller-price-lookup__semantics-title"
									>
										使用行情前請留意
									</h2>
									<ul>
										<li>行情只協助研究判斷，不代表供貨、成交或獲利保證。</li>
										<li>
											官方實際價來自農業部批發市場交易行情；P10～P90
											為市場有交易條件下的模型預測，不是官方上、中、下價。
										</li>
										<li>
											成交量反映批發市場交易活動，不是賣家庫存；區域行情也不是指定地址的實際採購成本。
										</li>
										<li>實際成本仍受品規、包裝、損耗與物流等條件影響。</li>
									</ul>
								</aside>
							</>
						) : null}
					</div>
				)}
			</div>
		</SellerPageShell>
	);
}
