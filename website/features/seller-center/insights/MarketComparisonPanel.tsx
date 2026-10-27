import type { MarketComparisonEntry } from "@/lib/pricebook/repo";
import { formatPrice } from "./insightsFormat";

interface MarketComparisonPanelProps {
	entries: MarketComparisonEntry[];
	currentMarketCode?: string;
	currentMarketName: string;
	currentP50: number | undefined;
	onSelectMarket: (marketCode: string) => void;
	/** 由呼叫端依實際單位組出來的說明字串。
	 *
	 * 🔴 刻意移除原本的預設值「同品項、D+1 模型中位價（新臺幣／公斤）」：
	 * 那個預設在呼叫端不傳時會替一條可能是台斤的序列宣稱它是公斤。缺席時
	 * 不顯示說明，比顯示一句可能錯的話好。 */
	unitLabel?: string;
}

/** 跨市場比較條狀圖——用 CSS 寬度畫條，不是第二個 SVG（見
 *  tests/seller-center-icon-policy.test.ts 的 svg 數量限制）。 */
export function MarketComparisonPanel({
	entries,
	currentMarketCode,
	currentMarketName,
	currentP50,
	onSelectMarket,
	unitLabel,
}: MarketComparisonPanelProps) {
	const priced = entries.filter(
		(e) =>
			e.p50 != null &&
			(!currentMarketCode || e.marketCode !== currentMarketCode),
	);
	const maximum = Math.max(
		currentP50 ?? 0,
		...priced.map((e) => e.p50 ?? 0),
		1,
	);

	return (
		<div className="seller-price-lookup__comparison">
			{unitLabel != null ? (
				<p className="seller-price-lookup__comparison-unit">{unitLabel}</p>
			) : null}
			<div className="seller-price-lookup__comparison-row seller-price-lookup__comparison-row--current">
				<span>{currentMarketName}</span>
				<i
					aria-hidden="true"
					style={{
						width: `${Math.min(((currentP50 ?? 0) / maximum) * 100, 100)}%`,
					}}
				/>
				<b>{formatPrice(currentP50)}</b>
			</div>
			{priced.map((entry) => (
				<button
					key={entry.marketCode}
					type="button"
					className="seller-price-lookup__comparison-row"
					onClick={() => onSelectMarket(entry.marketCode)}
					aria-label={`切換到${entry.marketName}，D+1 模型中位價${formatPrice(entry.p50)}`}
				>
					<span>{entry.marketName}</span>
					<i
						aria-hidden="true"
						style={{
							width: `${Math.min(((entry.p50 ?? 0) / maximum) * 100, 100)}%`,
						}}
					/>
					<b>{formatPrice(entry.p50)}</b>
				</button>
			))}
			{priced.length === 0 ? (
				<p className="seller-price-lookup__comparison-empty">
					{currentMarketCode
						? "目前沒有其他市場的 D+1 預測可比較。"
						: "目前沒有可個別比較的市場 D+1 預測。"}
				</p>
			) : null}
		</div>
	);
}
