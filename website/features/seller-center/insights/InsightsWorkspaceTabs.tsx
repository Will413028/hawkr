"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";

export const INSIGHTS_TAB_IDS = [
	"overview",
	"forecast",
	"spatial",
	"markets",
	"quality",
] as const;

export type InsightsTabId = (typeof INSIGHTS_TAB_IDS)[number];

const TABS: Array<{
	id: InsightsTabId;
	label: string;
	description: string;
	icon: AppIconName;
}> = [
	{
		id: "overview",
		label: "趨勢總覽",
		description: "官方實際行情與關鍵指標",
		icon: "graph",
	},
	{
		id: "forecast",
		label: "七日預測",
		description: "D+1 到 D+7 條件式價格區間",
		icon: "chart",
	},
	{
		id: "spatial",
		label: "價格地圖",
		description: "查看全臺市場價格分布",
		icon: "location",
	},
	{
		id: "markets",
		label: "市場比較",
		description: "同品項跨批發市場比較",
		icon: "reorder",
	},
	{
		id: "quality",
		label: "資料品質",
		description: "來源、模型版本與缺值規則",
		icon: "shield",
	},
];

/** 依序列能力過濾分頁。
 *
 * 🔴 分頁原本是靜態常數，五個永遠都在——那是「所有 series 都有市場與地圖」
 * 這個假設的具體形狀。多來源之後不成立：雞蛋的來源一天只回一列、沒有市場
 * 維度；毛豬有 19 個市場但 registry 沒有座標。
 *
 * 能力由後端宣告（SeriesCapabilities），這裡只負責照做。**不在前端從
 * 「有沒有資料」推導**——那會把「這種序列本來就沒有」與「這次剛好查無」
 * 混為一談，而兩者要對使用者說的話完全相反。
 *
 * 未提供 capabilities（舊快取、或尚未選定品項）時全部顯示，維持既有行為。 */
function visibleTabs(capabilities?: InsightsSeriesCapabilities) {
	if (!capabilities) return TABS;
	return TABS.filter((tab) => {
		if (tab.id === "spatial") return capabilities.spatial;
		if (tab.id === "markets") return capabilities.marketComparison;
		return true;
	});
}

export interface InsightsSeriesCapabilities {
	marketComparison: boolean;
	spatial: boolean;
	reasonCodes: string[];
}

export function InsightsWorkspaceTabs({
	activeTab,
	onChange,
	capabilities,
}: {
	activeTab: InsightsTabId;
	onChange: (tab: InsightsTabId) => void;
	capabilities?: InsightsSeriesCapabilities;
}) {
	const tabRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
	const tabs = visibleTabs(capabilities);

	const moveFocus = (index: number) => {
		const next = (index + tabs.length) % tabs.length;
		const nextTab = tabRefs.current[next];
		nextTab?.focus();
		onChange(tabs[next].id);
	};

	return (
		<nav
			className="seller-price-lookup__workspace-nav"
			aria-label="行情分析分頁"
		>
			<div
				className="seller-price-lookup__workspace-tabs"
				role="tablist"
				aria-label="行情分析"
				aria-orientation="horizontal"
			>
				{tabs.map((tab, index) => (
					<button
						key={tab.id}
						ref={(node) => {
							tabRefs.current[index] = node;
						}}
						type="button"
						role="tab"
						id={`insights-tab-${tab.id}`}
						aria-selected={activeTab === tab.id}
						aria-controls="insights-workspace-panel"
						tabIndex={activeTab === tab.id ? 0 : -1}
						onClick={() => onChange(tab.id)}
						onKeyDown={(event) => {
							if (event.key === "ArrowRight") {
								event.preventDefault();
								moveFocus(index + 1);
							} else if (event.key === "ArrowLeft") {
								event.preventDefault();
								moveFocus(index - 1);
							} else if (event.key === "Home") {
								event.preventDefault();
								moveFocus(0);
							} else if (event.key === "End") {
								event.preventDefault();
								moveFocus(TABS.length - 1);
							}
						}}
						title={tab.description}
					>
						<AppIcon name={tab.icon} size={17} />
						<span>{tab.label}</span>
					</button>
				))}
			</div>
		</nav>
	);
}
