import type { ForecastCurvePoint } from "@/lib/pricebook/repo";
import { formatCurrencyUnitLabel, formatPrice } from "./insightsFormat";
import { STATUS_LABELS } from "./statusLabels";

export function SevenDayTable({
	points,
	unit,
}: {
	points: ForecastCurvePoint[];
	/** 後端送來的單位代碼。缺席時 caption 顯示「單位待確認」而不是假設公斤
	 * ——多來源之後這張表也可能承載元／台斤的序列。 */
	unit?: string;
}) {
	if (points.length === 0) {
		return (
			<p className="seller-price-lookup__chart-empty">
				目前沒有可顯示的七日預測明細。
			</p>
		);
	}

	return (
		<div className="seller-price-lookup__forecast-detail">
			<section
				className="seller-price-lookup__table-scroll seller-price-lookup__table-scroll--forecast"
				aria-label="七日預測明細，可左右捲動"
			>
				<table className="seller-price-lookup__table seller-price-lookup__table--forecast">
					<caption className="visually-hidden">
						D+1 至 D+7 條件式價格預測；價格單位為{formatCurrencyUnitLabel(unit)}
					</caption>
					<thead>
						<tr>
							<th scope="col">目標日</th>
							<th scope="col">資料狀態</th>
							<th scope="col">預測下界 P10</th>
							<th scope="col">模型中位 P50</th>
							<th scope="col">預測上界 P90</th>
						</tr>
					</thead>
					<tbody>
						{points.map((point) => (
							<tr key={`${point.targetDate}-${point.horizonDays}`}>
								<td>
									<time dateTime={point.targetDate}>{point.targetDate}</time>
									<small>D+{point.horizonDays}</small>
								</td>
								<td>{STATUS_LABELS[point.status] ?? point.status}</td>
								<td>{formatPrice(point.p10)}</td>
								<td className="seller-price-lookup__table-emphasis">
									{formatPrice(point.p50)}
								</td>
								<td>{formatPrice(point.p90)}</td>
							</tr>
						))}
					</tbody>
				</table>
			</section>

			<ol
				className="seller-price-lookup__forecast-list"
				aria-label="七日預測明細"
			>
				{points.map((point) => (
					<li key={`${point.targetDate}-${point.horizonDays}`}>
						<header>
							<span>
								<time dateTime={point.targetDate}>{point.targetDate}</time>
								<small>D+{point.horizonDays}</small>
							</span>
							<strong>{STATUS_LABELS[point.status] ?? point.status}</strong>
						</header>
						<dl>
							<div className="seller-price-lookup__forecast-median">
								<dt>模型中位 P50</dt>
								<dd>{formatPrice(point.p50)}</dd>
							</div>
							<div>
								<dt>預測區間 P10～P90</dt>
								<dd>
									{formatPrice(point.p10)}～{formatPrice(point.p90)}
								</dd>
							</div>
						</dl>
					</li>
				))}
			</ol>
		</div>
	);
}
