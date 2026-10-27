import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const source = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/InsightsClient.tsx",
	),
	"utf-8",
);
const historyChartSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/HistoricalPriceChart.tsx",
	),
	"utf-8",
);
const forecastChartSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/ForecastBandChart.tsx",
	),
	"utf-8",
);
const emptyWorkspaceSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/InsightsEmptyWorkspace.tsx",
	),
	"utf-8",
);
const animatedMetricSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/AnimatedMetricValue.tsx",
	),
	"utf-8",
);
const workspaceTabsSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/InsightsWorkspaceTabs.tsx",
	),
	"utf-8",
);
const observedTableSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/ObservedPriceTable.tsx",
	),
	"utf-8",
);
const sellerCenterCssSource = readFileSync(
	join(import.meta.dirname, "../styles/features/seller-center.css"),
	"utf-8",
);

describe("InsightsClient structural contract", () => {
	it("uses the inline CropCatalogSearch, not the old flat <select>", () => {
		assert.match(source, /<CropCatalogSearch/);
		assert.match(source, /selectedCategoryId=\{selectedCategoryId\}/);
		assert.doesNotMatch(source, /pricebook-crop-select/);
	});

	it("places the product classification first in the primary workspace instead of the page-header actions slot", () => {
		assert.match(source, /seller-price-lookup__scope-picker/);
		assert.match(source, /seller-price-lookup__scope-control/);
		assert.match(source, /商品分類/);
		assert.doesNotMatch(source, /actions=\{[\s\S]*?<CropCatalogSearch/);
		assert.ok(
			source.indexOf("<CropCatalogSearch") <
				source.indexOf("seller-price-lookup__scope-summary"),
		);
		assert.match(
			source,
			/<InsightsWorkspaceTabs[\s\S]*?<div[\s\S]*?id="insights-workspace-panel"/,
		);
		assert.match(source, /role="tabpanel"[\s\S]*?tabIndex=\{0\}/);
	});

	it("uses an accessible, state-preserving disclosure for the scope filters", () => {
		const start = source.indexOf("<details");
		const end = source.indexOf("<InsightsWorkspaceTabs", start);
		assert.notEqual(start, -1);
		assert.notEqual(end, -1);
		const scopePickerSource = source.slice(start, end);

		assert.match(scopePickerSource, /open=\{scopeExpanded\}/);
		assert.match(scopePickerSource, /onToggle=/);
		assert.match(scopePickerSource, /<summary/);
		assert.match(scopePickerSource, /收合/);
		assert.match(scopePickerSource, /展開/);
		assert.match(scopePickerSource, /collapsedScopeSummary/);
		assert.match(
			source,
			/setScopeExpanded\(true\)[\s\S]*?requestAnimationFrame[\s\S]*?catalogSelectorRef\.current\?\.focus\(\)/,
		);
	});

	it("keeps the filter summary focused on official scope context", () => {
		const start = source.indexOf("<details");
		const end = source.indexOf("<InsightsWorkspaceTabs", start);
		const scopePickerSource = source.slice(start, end);

		assert.match(scopePickerSource, /<dt>市場範圍<\/dt>/);
		assert.match(scopePickerSource, /<dt>官方最新交易日<\/dt>/);
		assert.doesNotMatch(scopePickerSource, /<dt>預測狀態<\/dt>/);
		assert.match(
			sellerCenterCssSource,
			/\.seller-price-lookup__scope-summary\s*\{[^}]*grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/s,
		);
		assert.doesNotMatch(
			sellerCenterCssSource,
			/scope-summary > div:nth-child\(3\)/,
		);
		assert.match(
			sellerCenterCssSource,
			/\.seller-price-lookup__scope-picker:not\(\[open\]\)[\s\S]*?> \.seller-price-lookup__scope-content\s*\{[\s\S]*?display:\s*none;/,
		);
	});

	it("gives every unselected workspace one direct way to open the product picker", () => {
		assert.match(emptyWorkspaceSource, /onRequestSearch/);
		assert.match(emptyWorkspaceSource, /<Button/);
		assert.match(emptyWorkspaceSource, /選擇商品分類/);
		assert.match(source, /onRequestSearch={focusCatalogSelector}/);
		assert.doesNotMatch(emptyWorkspaceSource, /<ul|選定後會顯示/);
	});

	it("hides the workspace scrollbar", () => {
		assert.match(
			sellerCenterCssSource,
			/\.seller-price-lookup__workspace-tabs\s*\{[^}]*overflow-y:\s*hidden;[^}]*scrollbar-width:\s*none;/s,
		);
		assert.match(
			sellerCenterCssSource,
			/\.seller-price-lookup__workspace-tabs::-webkit-scrollbar\s*\{[^}]*display:\s*none;/s,
		);
	});

	it("uses a flat five-column workbench tab contract on mobile and desktop", () => {
		assert.match(workspaceTabsSource, /aria-orientation="horizontal"/);
		const start = sellerCenterCssSource.indexOf(
			".seller-price-lookup__workspace-nav {",
		);
		const end = sellerCenterCssSource.indexOf(
			"#insights-workspace-panel",
			start,
		);
		assert.notEqual(start, -1);
		assert.notEqual(end, -1);
		const sharedTabsCss = sellerCenterCssSource.slice(start, end);

		assert.match(
			sharedTabsCss,
			/grid-template-columns:\s*repeat\(5, minmax\(0, 1fr\)\);/,
		);
		assert.match(sharedTabsCss, /overflow-x:\s*hidden;/);
		assert.match(
			sharedTabsCss,
			/min-height:\s*48px;[\s\S]*font-weight:\s*700;/,
		);
		assert.match(
			sharedTabsCss,
			/button\[aria-selected="true"\][\s\S]*button\[aria-selected="true"\]::after/s,
		);
		assert.match(
			sharedTabsCss,
			/button:focus-visible\s*\{[\s\S]*outline-offset:\s*-2px;/,
		);
		assert.doesNotMatch(
			sharedTabsCss,
			/backdrop-filter|border-radius:\s*26px|position:\s*fixed|data-hidden/,
		);
	});

	it("renders a semantic four-part行情摘要 ledger", () => {
		assert.match(source, /aria-label="行情摘要"/);
		assert.match(source, /<dl className="seller-price-lookup__metric-ledger">/);
		const matches =
			source.match(/className="seller-price-lookup__metric(?: |")/g) ?? [];
		assert.equal(matches.length, 4);
		assert.match(source, /最新官方平均價/);
		assert.match(source, /D\+1 模型中位價/);
		assert.match(source, /P10～P90，非官方上／下價/);
		assert.match(source, /近 30 日成交量/);
		assert.match(source, /const hasD1Median =/);
		assert.match(source, /D\+1 尚未回傳；未以 D\+2 代替/);
		assert.match(source, /P10～P90 尚未完整提供/);
		assert.doesNotMatch(source, /<StatCard/);
	});

	it("branches on a group/crop selection kind", () => {
		assert.match(source, /"group"/);
		assert.match(source, /"crop"/);
	});

	it("renders MarketComparisonPanel with the right entries source per selection kind", () => {
		assert.match(source, /<MarketComparisonPanel/);
	});

	it("renders SevenDayTable and the fixed decision-support disclosures", () => {
		assert.match(source, /<SevenDayTable/);
		assert.match(source, /行情只協助研究判斷，不代表供貨、成交或獲利保證/);
		assert.match(source, /成交量反映批發市場交易活動，不是賣家庫存/);
	});

	it("renders the historical and forecast charts as two separate cards, not the old merged PriceTrendChart", () => {
		assert.match(source, /<HistoricalPriceChart/);
		assert.match(source, /<ForecastBandChart/);
		assert.doesNotMatch(source, /<PriceTrendChart\b/);
	});

	it("uses plain-language section headers without decorative English eyebrows", () => {
		assert.match(source, /<InsightsSectionHeader/);
		assert.doesNotMatch(source, /<SectionEyebrow/);
		assert.doesNotMatch(source, /HISTORICAL SIGNAL|SEVEN-DAY FORECAST/);
	});

	it("restores the five-part Pricebook workspace and lazy-loads the map", () => {
		assert.match(source, /<InsightsWorkspaceTabs/);
		assert.match(source, /"overview"/);
		assert.match(source, /"forecast"/);
		assert.match(source, /"spatial"/);
		assert.match(source, /"markets"/);
		assert.match(source, /"quality"/);
		assert.match(
			source,
			/dynamic\(\(\) => import\("\.\/PriceTopographyPanel"\)/,
		);
		assert.match(source, /ssr: false/);
	});

	it("uses exact-value metric transitions without showcase motion libraries", () => {
		assert.match(source, /<AnimatedMetricValue/);
		assert.doesNotMatch(source, /import\("gsap"\)|gsap\.context/);
		assert.doesNotMatch(source, /import\("lenis"\)|new Lenis/);
		assert.doesNotMatch(source, /import\("vanta|VANTA/);
		assert.doesNotMatch(animatedMetricSource, /requestAnimationFrame|useState/);
		assert.match(animatedMetricSource, /aria-live="polite"/);
		assert.match(
			sellerCenterCssSource,
			/@media \(prefers-reduced-motion: reduce\)[\s\S]*#insights-workspace-panel/,
		);
	});

	it("uses pressed buttons, not tab semantics, for the historical range control", () => {
		assert.match(source, /<fieldset\s+aria-label="時間範圍"/);
		assert.match(source, /aria-pressed=\{historyRange === opt\.value\}/);
		assert.match(source, /<section\s+id="observed-price-history-panel"/);
		assert.doesNotMatch(source, /role="tab"\s+aria-selected=\{historyRange/);
	});

	it("consumes the fail-closed forecast display policy across price surfaces", () => {
		assert.match(
			source,
			/import \{ resolveForecastDisplayData \} from "\.\/forecastDisplayData"/,
		);
		assert.match(
			source,
			/const forecastDisplay = resolveForecastDisplayData\(forecast\.data\)/,
		);
		assert.match(source, /status, isPriceReady, points, marketComparison/);
		assert.match(source, /forecast\.error \|\| !hasD1Median/);
		assert.match(source, /forecast\.error \|\| !hasD1Band/);
		assert.match(source, /\) : !isPriceReady \? \(/);
		assert.match(source, /!forecast\.error &&\s+isPriceReady/);
		assert.match(
			source,
			/entries=\{\s*selection\.kind === "crop"\s*\?\s*cropMarketComparison\s*:\s*groupMarketComparison\s*\}/,
		);
	});

	it("keeps market as the fixed fourth field, defaulting to all markets", () => {
		assert.match(source, /<InsightsMarketFilter/);
		assert.match(
			source,
			/className="seller-price-lookup__scope-fields has-market-filter"/,
		);
		assert.match(
			source,
			/if \(urlState\.scope === "crop" && urlState\.crop\?\.trim\(\)\)/,
		);
		assert.match(
			source,
			/selection\?\.kind === "group"\s*\?\s*selection\.marketCode/,
		);
		assert.match(
			source,
			/selection == null\s*\? \(urlState\.market \?\? undefined\)/,
		);
		assert.match(source, /: radarMarketEntries/);
		assert.match(source, /if \(selection == null\)/);
		assert.match(
			source,
			/loading=\{\s*\(selection\?\.kind === "crop" \|\| selection\?\.kind === "group"\)\s*&&\s*forecast\.loading\s*\}/,
		);
		assert.match(source, /market:\s*nextSelection\.marketCode \?\? null/);
		assert.match(source, /focusMarket:\s*null/);
		assert.match(source, /market:\s*marketCode \?\? null/);
		assert.match(source, /全市場成交量加權彙整/);
		assert.doesNotMatch(
			source,
			/pendingCropMatch|selectCropWithMarket|market_required/,
		);
	});

	it("clears all previous price dimensions before resolving a new L3", () => {
		assert.match(source, /const beginCategorySelection = React\.useCallback/);
		for (const key of [
			"scope",
			"group",
			"crop",
			"market",
			"label",
			"focusMarket",
		]) {
			assert.match(source, new RegExp(`${key}: null`));
		}
		assert.match(source, /onCategoryChange=\{beginCategorySelection\}/);
	});

	it("separates official observed prices from conditional model forecasts", () => {
		assert.match(source, /官方批發實際行情/);
		assert.match(source, /農業部批發市場交易行情/);
		assert.match(source, /D\+1～D\+7 條件式預測/);
		assert.match(source, /P10～P90.*不是官方上、中、下價/s);
		assert.match(source, /<ObservedPriceTable/);
	});

	it("shows auditable date, model, market and unit context", () => {
		assert.match(source, /aria-label="目前研究範圍"/);
		assert.match(source, /官方最新交易日/);
		assert.match(source, /預測模型版本[\s\S]*預測狀態：\{forecastStatusText\}/);
		assert.match(source, /activeTab === "quality"[\s\S]*實際價來源/);
		assert.match(source, /最新交易日/);
		assert.match(source, /預測模型版本/);
		assert.match(source, /市場範圍/);
		assert.match(source, /<dt>價格單位<\/dt>/);
		assert.match(source, /formatCurrencyUnitLabel\(forecastDisplay\.unit\)/);
		assert.match(source, /selectionIdentity/);
		assert.match(source, /品項代碼.*市場代碼/s);
	});

	it("round-trips the workspace state through typed URL parameters", () => {
		assert.match(source, /useQueryStates/);
		for (const key of [
			"tab",
			"scope",
			"group",
			"crop",
			"category",
			"market",
			"day",
			"view",
			"focusMarket",
			"range",
		]) {
			assert.match(source, new RegExp(`${key}:`));
		}
		assert.match(source, /history: "replace"/);
		assert.match(source, /shallow: true/);
		assert.match(source, /focusMarket: null/);
	});

	it("uses responsive accessible Recharts and honors reduced motion", () => {
		for (const chartSource of [historyChartSource, forecastChartSource]) {
			assert.match(chartSource, /ResponsiveContainer/);
			assert.match(chartSource, /accessibilityLayer/);
			assert.match(chartSource, /<figure/);
			assert.match(chartSource, /<figcaption className="sr-only">/);
			assert.match(chartSource, /usePrefersReducedMotion/);
			assert.match(chartSource, /isAnimationActive=\{!reducedMotion\}/);
			assert.doesNotMatch(chartSource, /role="img"/);
		}
	});

	it("keeps the wide observed-price table keyboard scrollable on narrow screens", () => {
		assert.match(observedTableSource, /<section/);
		assert.match(observedTableSource, /aria-label="實際行情明細，可左右捲動"/);
		assert.match(observedTableSource, /tabIndex=\{0\}/);
	});
});
