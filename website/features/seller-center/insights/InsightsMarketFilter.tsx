"use client";

import * as React from "react";
import { Dropdown, type DropdownItem } from "@/components/ui/Dropdown";
import type { CatalogMarketOption } from "@/lib/pricebook/repo";

export const ALL_MARKETS_VALUE = "__all_markets__";

export function marketFilterItems(
	entries: readonly CatalogMarketOption[],
	selectedMarketCode?: string,
): DropdownItem[] {
	const markets = new Map<string, string>();
	for (const entry of entries) {
		const code = entry.marketCode.trim();
		if (!code || markets.has(code)) continue;
		markets.set(code, entry.marketName.trim() || code);
	}
	if (selectedMarketCode && !markets.has(selectedMarketCode)) {
		markets.set(selectedMarketCode, selectedMarketCode);
	}

	return [
		{
			value: ALL_MARKETS_VALUE,
			label: "全部市場",
			description: "跨市場成交量加權彙整",
		},
		...[...markets]
			.sort((left, right) =>
				left[1].localeCompare(right[1], "zh-Hant", { numeric: true }),
			)
			.map(([value, label]) => ({ value, label })),
	];
}

interface InsightsMarketFilterProps {
	marketCode?: string;
	entries: readonly CatalogMarketOption[];
	loading: boolean;
	onChange: (marketCode: string | undefined) => void;
}

/** 固定第四欄的市場篩選；它不是商品分類的第四層。 */
export function InsightsMarketFilter({
	marketCode,
	entries,
	loading,
	onChange,
}: InsightsMarketFilterProps) {
	const items = React.useMemo(
		() => marketFilterItems(entries, marketCode),
		[entries, marketCode],
	);
	const description = loading
		? "已預設全部市場，正在載入可選市場…"
		: marketCode
			? "實際行情與預測只看指定市場；價格地圖仍保留全臺比較。"
			: "預設全部市場；也可改選特定市場。";
	const selectedLabel =
		items.find((item) => item.value === (marketCode ?? ALL_MARKETS_VALUE))
			?.label ?? "全部市場";

	return (
		<div className="seller-price-lookup__scope-control seller-price-lookup__market-filter">
			<span className="seller-price-lookup__scope-label">市場範圍</span>
			<Dropdown
				value={marketCode ?? ALL_MARKETS_VALUE}
				onChange={(value) =>
					onChange(value === ALL_MARKETS_VALUE ? undefined : value)
				}
				items={items}
				aria-label={`市場範圍：${selectedLabel}`}
				size="lg"
				searchable={items.length > 7}
				noResultsLabel="找不到可查詢的市場"
				description={description}
			/>
		</div>
	);
}
