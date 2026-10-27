import type { PriceObservation } from "@/lib/pricebook/repo";
import { formatPrice, formatVolume } from "./insightsFormat";

interface ObservedPriceTableProps {
	observations: PriceObservation[];
	currentMarketCode?: string;
	currentMarketName?: string;
	aggregateMarketLabel?: string;
}

/** 官方批發實際行情的精確值替代表格，也供手機與鍵盤使用者核對圖表。 */
export function ObservedPriceTable({
	observations,
	currentMarketCode,
	currentMarketName,
	aggregateMarketLabel = "全部市場彙整",
}: ObservedPriceTableProps) {
	const rows = [...observations].sort((a, b) =>
		b.tradeDate.localeCompare(a.tradeDate),
	);

	if (rows.length === 0) {
		return (
			<p className="seller-price-lookup__chart-empty">
				目前沒有可顯示的官方實際行情。
			</p>
		);
	}

	return (
		<details className="seller-price-lookup__details" open>
			<summary>核對 {rows.length} 筆實際行情明細</summary>
			<section
				className="seller-price-lookup__table-scroll seller-price-lookup__table-scroll--observed"
				aria-label="實際行情明細，可左右捲動"
				// biome-ignore lint/a11y/noNoninteractiveTabindex: keyboard users need a focus target to pan the wide table
				tabIndex={0}
			>
				<table className="seller-price-lookup__table seller-price-lookup__table--observed">
					<caption className="visually-hidden">
						農業部批發市場實際行情；價格單位為新臺幣每公斤，成交量單位為公斤
					</caption>
					<thead>
						<tr>
							<th scope="col">交易日</th>
							<th scope="col">市場</th>
							<th scope="col">平均價</th>
							<th scope="col">官方上價</th>
							<th scope="col">官方中價</th>
							<th scope="col">官方下價</th>
							<th scope="col">成交量</th>
						</tr>
					</thead>
					<tbody>
						{rows.map((row) => {
							const marketLabel = !row.marketCode
								? aggregateMarketLabel
								: currentMarketName && row.marketCode === currentMarketCode
									? currentMarketName
									: row.marketCode;
							return (
								<tr key={`${row.marketCode}-${row.cropCode}-${row.tradeDate}`}>
									<td>
										<time dateTime={row.tradeDate}>{row.tradeDate}</time>
									</td>
									<td>{marketLabel}</td>
									<td className="seller-price-lookup__table-emphasis">
										{formatPrice(row.avgPrice)}
									</td>
									<td>{formatPrice(row.upperPrice)}</td>
									<td>{formatPrice(row.middlePrice)}</td>
									<td>{formatPrice(row.lowerPrice)}</td>
									<td>{formatVolume(row.volumeKg)}</td>
								</tr>
							);
						})}
					</tbody>
				</table>
			</section>
		</details>
	);
}
