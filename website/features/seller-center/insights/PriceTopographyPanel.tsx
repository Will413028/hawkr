"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useSpatialSurface } from "@/lib/hooks/useSpatialSurface";
import {
	type PricebookSelection,
	spatialSelectionKey,
} from "@/lib/pricebook/repo";
import { InsightsSectionHeader } from "./InsightsSectionHeader";
import { formatPrice, formatVolume } from "./insightsFormat";
import { priceScalePresentation } from "./spatialMapData";
import {
	type PriceMapMode,
	type PriceMapUnavailableReason,
	TaiwanPriceMap,
	TaiwanPriceMapPreview,
	TaiwanPriceMapUnavailable,
} from "./TaiwanPriceMap";

const QUALITY_LABELS: Record<string, string> = {
	boundary_unavailable: "尚未取得核准且可追溯的臺灣陸域邊界。",
	insufficient_spatial_points: "可定位且有價格的開市市場不足，未生成曲面。",
	insufficient_spatial_coverage: "市場距離不足以支撐連續價格面。",
	unsupported_boundary_regions: "部分離島沒有足夠批發市場訊號。",
	unsupported_land_area: "可支撐的陸域比例偏低。",
	missing_market_geocodes: "部分市場缺少通過治理契約的座標。",
	outside_boundary_domain: "部分市場點落在核准邊界範圍外。",
	flat_price_field: "市場價格相同，沒有可辨識的等高線梯度。",
	missing_fixed_price_scale: "尚未取得此品項適用的價格色階規則。",
	price_scale_unavailable: "尚未取得此品項適用的價格色階規則。",
};

const DISCLOSURE_LABELS: Record<string, string> = {
	interpolated_surface_not_observed_between_markets:
		"市場之間的顏色與高度是空間插值，不是當地官方成交紀錄。",
	wholesale_market_location_not_production_origin:
		"地圖節點代表批發市場位置，不代表農產品產地。",
	closed_markets_excluded_from_surface:
		"休市市場不補零、不沿用舊價，也不參與當日價格面。",
	offshore_regions_without_market_support_remain_unpriced:
		"沒有足夠市場支撐的離島維持無價格，不跨海插值。",
	simplified_administrative_boundary_not_legal_boundary:
		"行政邊界經簡化供視覺化使用，不可作地籍或法律界線判定。",
};

function formatRatio(value?: number): string {
	if (value == null || !Number.isFinite(value)) return "—";
	return `${Math.round(value * 100)}%`;
}

/** 把 `unavailableDetail`（status:"unavailable" 的診斷欄位）轉成地圖空狀態
 *  的具體文案；沒有細節（一般網路/契約錯誤）時回傳 undefined，元件會退回
 *  既有的通用錯誤文案。 */
function spatialUnavailableReason(
	horizonDays: number,
	detail: {
		targetDate?: string;
		qualityReasonCodes: string[];
		closedMarketCount?: number;
		totalMarketCount?: number;
	} | null,
): PriceMapUnavailableReason | undefined {
	if (!detail) return undefined;
	const dateLabel = detail.targetDate ? `（${detail.targetDate}）` : "";
	const summary =
		detail.closedMarketCount != null && detail.totalMarketCount != null
			? `D+${horizonDays}${dateLabel} ${detail.totalMarketCount} 個市場中有 ${detail.closedMarketCount} 個休市，成交資料不足以計算空間分布，不是服務故障。`
			: `D+${horizonDays}${dateLabel} 目前沒有足夠的市場成交資料可計算空間分布，不是服務故障。`;
	return {
		summary,
		details: detail.qualityReasonCodes.map(
			(code) => QUALITY_LABELS[code] ?? code,
		),
	};
}

export function PriceTopographyPreview({
	onRequestSearch,
}: {
	onRequestSearch: () => void;
}) {
	return (
		<section className="seller-card seller-price-lookup__section seller-price-lookup__spatial-section">
			<InsightsSectionHeader
				title="全臺價格地圖"
				description="選擇品項後，即可查看各地市場價格。"
				action={
					<Button color="primary" size="lg" onClick={onRequestSearch}>
						選擇品項
					</Button>
				}
			/>
			<TaiwanPriceMapPreview />
		</section>
	);
}

export default function PriceTopographyPanel({
	selection,
	label,
	horizonDays,
	requestedMode,
	selectedMarketCode,
	onHorizonDaysChange,
	onModeChange,
	onMarketSelect,
}: {
	selection: PricebookSelection;
	label: string;
	horizonDays: number;
	requestedMode: "contour" | "height";
	selectedMarketCode: string | null;
	onHorizonDaysChange: (day: number) => void;
	onModeChange: (mode: "contour" | "height") => void;
	onMarketSelect: (marketCode: string) => void;
}) {
	const spatial = useSpatialSurface(selection, horizonDays);
	const marketListRef = React.useRef<HTMLDivElement>(null);
	const data =
		spatial.data?.status === "ready" || spatial.data?.status === "degraded"
			? spatial.data
			: undefined;
	const scalePresentation = data
		? priceScalePresentation(data.scale, data.statistics)
		: null;
	const hasPriceScale = Boolean(scalePresentation);
	const canContour = Boolean(
		data &&
			hasPriceScale &&
			scalePresentation?.kind !== "flat" &&
			data.displayMode !== "points_only" &&
			data.displayMode !== "table_only" &&
			data.contourLinesGeojson.features.length,
	);
	const canHeight = Boolean(
		data &&
			hasPriceScale &&
			data.displayMode !== "points_only" &&
			data.displayMode !== "table_only" &&
			data.surfaceCellsGeojson.features.length,
	);
	const effectiveMode: PriceMapMode =
		requestedMode === "contour" && canContour
			? "contour"
			: requestedMode === "height" && canHeight
				? "height"
				: canContour
					? "contour"
					: canHeight
						? "height"
						: "points";

	React.useEffect(() => {
		if (requestedMode === "contour" && !canContour && canHeight) {
			onModeChange("height");
		}
		if (requestedMode === "height" && !canHeight && canContour) {
			onModeChange("contour");
		}
	}, [canContour, canHeight, onModeChange, requestedMode]);

	const selectedMarket =
		data?.marketPrices.find(
			(market) => market.marketCode === selectedMarketCode,
		) ?? null;
	const effectiveSelectedMarketCode =
		selectedMarket?.marketCode ?? selectedMarketCode;

	React.useEffect(() => {
		const list = marketListRef.current;
		if (!list || !effectiveSelectedMarketCode) return;
		const target = Array.from(
			list.querySelectorAll<HTMLButtonElement>("button[data-market-code]"),
		).find(
			(button) => button.dataset.marketCode === effectiveSelectedMarketCode,
		);
		if (!target) return;
		const listRect = list.getBoundingClientRect();
		const targetRect = target.getBoundingClientRect();
		const delta =
			targetRect.top < listRect.top
				? targetRect.top - listRect.top
				: targetRect.bottom > listRect.bottom
					? targetRect.bottom - listRect.bottom
					: 0;
		if (!delta) return;
		list.scrollTo({
			top: list.scrollTop + delta,
			behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
				? "auto"
				: "smooth",
		});
	}, [effectiveSelectedMarketCode]);

	return (
		<div className="seller-price-lookup__spatial-workspace">
			<section className="seller-card seller-price-lookup__section seller-price-lookup__spatial-section">
				<InsightsSectionHeader
					title="全臺價格地圖"
					description={`查看${label}在各市場的預估價格分布。`}
				/>

				<div className="seller-price-lookup__spatial-controls">
					<fieldset>
						<legend>預測日期</legend>
						<div className="seller-price-lookup__day-switch">
							{Array.from({ length: 7 }, (_, index) => index + 1).map((day) => (
								<button
									key={day}
									type="button"
									aria-pressed={horizonDays === day}
									onClick={() => onHorizonDaysChange(day)}
								>
									D+{day}
								</button>
							))}
						</div>
					</fieldset>
					<fieldset>
						<legend>地圖視角</legend>
						<div className="seller-price-lookup__view-switch">
							<button
								type="button"
								aria-pressed={effectiveMode === "contour"}
								disabled={!canContour}
								onClick={() => onModeChange("contour")}
								title={canContour ? undefined : "這筆資料沒有可用等高線"}
							>
								等高線
							</button>
							<button
								type="button"
								aria-pressed={effectiveMode === "height"}
								disabled={!canHeight}
								onClick={() => onModeChange("height")}
								title={canHeight ? undefined : "這筆資料沒有可用立體價格面"}
							>
								立體高度
							</button>
						</div>
						{data && (!canContour || !canHeight) ? (
							<p className="seller-price-lookup__view-capability" role="status">
								{!canContour && !canHeight
									? "本期只支援市場點與行政區價格；等高線與立體高度未達品質門檻。"
									: !canContour
										? "本期未提供可用等高線。"
										: "本期未提供可用立體價格面。"}
							</p>
						) : null}
					</fieldset>
				</div>

				{spatial.loading ? (
					<div className="seller-price-lookup__map-loading" role="status">
						<div aria-hidden="true" />
						<strong>正在載入核准邊界與價格等高線…</strong>
						<span>歷史行情與預測資料不受影響。</span>
					</div>
				) : spatial.error || !data ? (
					<TaiwanPriceMapUnavailable
						onRetry={spatial.retry}
						reason={spatialUnavailableReason(
							horizonDays,
							spatial.unavailableDetail,
						)}
					/>
				) : data.displayMode === "table_only" ? (
					<div
						className="seller-price-lookup__spatial-unavailable"
						role="status"
					>
						<strong>目前只能顯示市場價格表</strong>
						<span>
							空間支撐或邊界品質未達門檻，因此不初始化地圖，也不補成零元。
						</span>
					</div>
				) : (
					<>
						<dl className="seller-price-lookup__spatial-metrics">
							<div>
								<dt>全市場 P50</dt>
								<dd>{formatPrice(data.statistics.p50)}</dd>
							</div>
							<div>
								<dt>有價格市場</dt>
								<dd>{data.statistics.pricedMarketCount} 個市場</dd>
							</div>
							<div>
								<dt>市場 P50 低～高</dt>
								<dd>
									{formatPrice(data.statistics.minimumP50)}～
									{formatPrice(data.statistics.maximumP50)}
								</dd>
							</div>
							<div>
								<dt>陸域支撐率</dt>
								<dd>{formatRatio(data.supportedLandAreaRatio)}</dd>
							</div>
						</dl>
						<div className="seller-price-lookup__map-layout">
							<TaiwanPriceMap
								key={`${spatialSelectionKey(selection)}:D+${data.horizonDays}`}
								surface={data}
								mode={effectiveMode}
								viewIdentity={`${spatialSelectionKey(selection)}:D+${data.horizonDays}`}
								selectedMarketCode={effectiveSelectedMarketCode}
								onMarketSelect={onMarketSelect}
							/>
							<aside
								className="seller-price-lookup__map-rail"
								aria-label="地圖市場清單"
							>
								<div className="seller-price-lookup__map-rail-head">
									<AppIcon name="location" size={18} />
									<div>
										<h3>批發市場</h3>
										<p>點選後同步地圖與下方明細</p>
									</div>
								</div>
								{selectedMarket ? (
									<dl className="seller-price-lookup__map-rail-summary">
										<div>
											<dt>目前選取</dt>
											<dd>{selectedMarket.marketName}</dd>
										</div>
										<div>
											<dt>D+{data.horizonDays} P50</dt>
											<dd>{formatPrice(selectedMarket.p50)}</dd>
										</div>
										<div>
											<dt>P10～P90</dt>
											<dd>
												{formatPrice(selectedMarket.p10)}～
												{formatPrice(selectedMarket.p90)}
											</dd>
										</div>
									</dl>
								) : null}
								<div
									ref={marketListRef}
									className="seller-price-lookup__map-market-list"
								>
									{data.marketPrices.map((market) => (
										<button
											key={`${market.categoryCode ?? "unknown"}:${market.marketCode}`}
											type="button"
											data-market-code={market.marketCode}
											aria-pressed={
												effectiveSelectedMarketCode === market.marketCode
											}
											onClick={() => onMarketSelect(market.marketCode)}
										>
											<span>
												<strong>{market.marketName}</strong>
												<small>
													{market.usedForSurface
														? "價格面採用"
														: market.marketStatus}
												</small>
											</span>
											<b>{formatPrice(market.p50)}</b>
										</button>
									))}
								</div>
							</aside>
						</div>
					</>
				)}
			</section>

			{data ? (
				<>
					<section
						id="spatial-market-detail"
						className="seller-card seller-price-lookup__section"
					>
						<InsightsSectionHeader
							title={`D+${data.horizonDays} 市場價格明細`}
							description="地圖之外保留全部精確值；休市、缺值與未採用市場不會顯示成零元。"
						/>
						<p className="seller-price-lookup__table-scroll-hint">
							左右滑動可查看
							P10、P50、P90、近期成交量與價格面狀態；市場名稱會固定在左側。
						</p>
						<div className="seller-price-lookup__table-scroll">
							<table className="seller-price-lookup__table seller-price-lookup__table--spatial">
								<caption className="sr-only">
									各批發市場 D+{data.horizonDays} 預測價格與空間運算狀態
								</caption>
								<thead>
									<tr>
										<th>批發市場</th>
										<th>狀態</th>
										<th>P10</th>
										<th>P50</th>
										<th>P90</th>
										<th>近期成交量</th>
										<th>價格面</th>
									</tr>
								</thead>
								<tbody>
									{data.marketPrices.map((market) => (
										<tr
											key={`${market.categoryCode ?? "unknown"}:${market.marketCode}`}
											className={
												effectiveSelectedMarketCode === market.marketCode
													? "is-selected"
													: undefined
											}
										>
											<th scope="row">
												<button
													type="button"
													className="seller-price-lookup__market-row-action"
													aria-pressed={
														effectiveSelectedMarketCode === market.marketCode
													}
													onClick={() => onMarketSelect(market.marketCode)}
												>
													<strong>{market.marketName}</strong>
													<small>
														{market.marketCode}
														{market.categoryCode
															? ` · ${market.categoryCode}`
															: ""}
													</small>
												</button>
											</th>
											<td>{market.marketStatus || market.predictionStatus}</td>
											<td>{formatPrice(market.p10)}</td>
											<td className="seller-price-lookup__table-emphasis">
												{formatPrice(market.p50)}
											</td>
											<td>{formatPrice(market.p90)}</td>
											<td>{formatVolume(market.recentVolumeKg)}</td>
											<td>{market.usedForSurface ? "已採用" : "未採用"}</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					</section>

					<aside className="seller-price-lookup__spatial-notes">
						<div>
							<h3>判讀方式</h3>
							<ol>
								<li>先看目前品項的價格帶，再用等高線文字核對每公斤價格。</li>
								<li>市場點是模型輸入位置，不是產地。</li>
								<li>空白區域表示支撐不足，不代表價格為零。</li>
							</ol>
						</div>
						<div>
							<h3>本圖品質</h3>
							{data.qualityReasonCodes.length ? (
								<ul>
									{data.qualityReasonCodes.map((code) => (
										<li key={code}>{QUALITY_LABELS[code] ?? code}</li>
									))}
								</ul>
							) : (
								<p>目前沒有額外品質警告。</p>
							)}
						</div>
						<div>
							<h3>資料系譜</h3>
							<dl>
								<div>
									<dt>模型</dt>
									<dd>{data.lineage.modelVersion || "未提供"}</dd>
								</div>
								<div>
									<dt>邊界</dt>
									<dd>{data.boundary.version || "未提供"}</dd>
								</div>
								<div>
									<dt>來源截止</dt>
									<dd>{data.lineage.sourceDataAsOf || "未提供"}</dd>
								</div>
							</dl>
						</div>
					</aside>

					{data.disclosures.length ? (
						<div className="seller-price-lookup__map-disclosure">
							<strong>價格面不是報價單</strong>
							<ul>
								{data.disclosures.map((code) => (
									<li key={code}>{DISCLOSURE_LABELS[code] ?? code}</li>
								))}
							</ul>
						</div>
					) : null}
				</>
			) : null}
		</div>
	);
}
