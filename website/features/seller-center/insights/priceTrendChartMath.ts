export interface ChartPoint {
	date: string; // YYYY-MM-DD
	value: number;
}

export interface ChartBounds {
	width: number;
	height: number;
	minValue: number;
	maxValue: number;
	minDate: string;
	maxDate: string;
}

export interface BandPoint {
	date: string;
	low: number;
	high: number;
}

/** 把日期字串（YYYY-MM-DD）換算成 x 座標（0~width）。 */
export function xForDate(
	date: string,
	minDate: string,
	maxDate: string,
	width: number,
): number {
	const t = new Date(date).getTime();
	const min = new Date(minDate).getTime();
	const max = new Date(maxDate).getTime();
	if (max === min) return width / 2;
	return ((t - min) / (max - min)) * width;
}

/** 把價格數值換算成 y 座標（0~height）。SVG y 軸朝下，所以數值愈大 y 愈小。 */
export function yForValue(
	value: number,
	minValue: number,
	maxValue: number,
	height: number,
): number {
	if (maxValue === minValue) return height / 2;
	return height - ((value - minValue) / (maxValue - minValue)) * height;
}

/** 把一組 {date, value} 點轉成 SVG <path> 的 "M x y L x y..." d 屬性字串。
 *  空陣列回傳空字串——呼叫端據此決定要不要渲染 <path>。 */
export function pointsToLinePath(
	points: ChartPoint[],
	bounds: ChartBounds,
): string {
	if (points.length === 0) return "";
	return points
		.map((p, i) => {
			const x = xForDate(p.date, bounds.minDate, bounds.maxDate, bounds.width);
			const y = yForValue(
				p.value,
				bounds.minValue,
				bounds.maxValue,
				bounds.height,
			);
			return `${i === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`;
		})
		.join(" ");
}

/** 把一組 {date, low, high} 點轉成合理區間帶的封閉 SVG <path>（沿上緣走，
 *  再沿下緣走回來）。空陣列回傳空字串。 */
export function pointsToBandPath(
	points: BandPoint[],
	bounds: ChartBounds,
): string {
	if (points.length === 0) return "";
	const top = points.map((p) => ({
		x: xForDate(p.date, bounds.minDate, bounds.maxDate, bounds.width),
		y: yForValue(p.high, bounds.minValue, bounds.maxValue, bounds.height),
	}));
	const bottom = points
		.map((p) => ({
			x: xForDate(p.date, bounds.minDate, bounds.maxDate, bounds.width),
			y: yForValue(p.low, bounds.minValue, bounds.maxValue, bounds.height),
		}))
		.reverse();
	const path = [...top, ...bottom]
		.map(
			(pt, i) => `${i === 0 ? "M" : "L"}${pt.x.toFixed(2)},${pt.y.toFixed(2)}`,
		)
		.join(" ");
	return `${path} Z`;
}

/** 從歷史觀測值＋預測曲線（含 P10/P90）一起算出 y 軸的 min/max，讓兩段資料
 *  能畫在同一張圖、同一個尺度上。上下各留 10% padding 避免線貼著邊緣；單一
 *  重複數值時仍給 padding，避免 min===max 讓所有點疊成一條線。空輸入回傳
 *  null。 */
export function computeValueRange(
	values: number[],
): { min: number; max: number } | null {
	if (values.length === 0) return null;
	const min = Math.min(...values);
	const max = Math.max(...values);
	if (min === max) {
		const padding = min === 0 ? 1 : Math.max(min * 0.05, 0.5);
		return { min: Math.max(0, min - padding), max: max + padding };
	}
	const padding = (max - min) * 0.1;
	return { min: Math.max(0, min - padding), max: max + padding };
}

/** 產生 X 軸日期刻度：頭尾兩個日期一定包含，中間依 targetCount 平均分佈。
 *  minDate == maxDate 時只回一個刻度，避免除以零。 */
export function generateDateTicks(
	minDate: string,
	maxDate: string,
	targetCount: number,
): string[] {
	if (minDate === maxDate) return [minDate];
	const min = new Date(minDate).getTime();
	const max = new Date(maxDate).getTime();
	const count = Math.max(2, targetCount);
	const ticks: string[] = [];
	for (let i = 0; i < count; i++) {
		const t = min + ((max - min) * i) / (count - 1);
		ticks.push(new Date(t).toISOString().slice(0, 10));
	}
	return ticks;
}

/** 產生 Y 軸價格刻度：頭尾兩個值一定包含，中間平均分佈。minValue ==
 *  maxValue 時只回一個刻度。 */
export function generatePriceTicks(
	minValue: number,
	maxValue: number,
	targetCount: number,
): number[] {
	if (minValue === maxValue) return [minValue];
	const count = Math.max(2, targetCount);
	const ticks: number[] = [];
	for (let i = 0; i < count; i++) {
		ticks.push(minValue + ((maxValue - minValue) * i) / (count - 1));
	}
	return ticks;
}

/** 找出離 targetDate 最近的點的 index，供 hover tooltip 用滑鼠 x 座標反查
 *  資料點。空陣列回傳 -1。同距離時取較早的索引（Array.prototype.reduce
 *  的第一次比較天然滿足這個 tie-break，不需要額外程式碼）。 */
export function nearestPointIndex(
	points: ChartPoint[],
	targetDate: string,
): number {
	if (points.length === 0) return -1;
	const target = new Date(targetDate).getTime();
	let bestIndex = 0;
	let bestDiff = Math.abs(new Date(points[0].date).getTime() - target);
	for (let i = 1; i < points.length; i++) {
		const diff = Math.abs(new Date(points[i].date).getTime() - target);
		if (diff < bestDiff) {
			bestDiff = diff;
			bestIndex = i;
		}
	}
	return bestIndex;
}

/** 7 日／30 日／全部 的時間範圍。全部＝目前已抓回的整個資料集，不代表後端有
 *  更長的歷史——`observed-price?history=true` 目前固定回傳
 *  `OBSERVED_WINDOW_DAYS`（預設 30 天）整個 window，是否有更多天數不由前端
 *  決定。 */
export type HistoryRange = 7 | 30 | "all";

/** 依 `range` 從 `today`（含）往回篩選 `points`。"all" 原樣回傳。空輸入回傳
 *  空陣列。`today` 是資料集裡最後一個實際日期（呼叫端傳入），不是裝置當下
 *  時間——避免時區/資料邊界不一致的問題，也讓這個函式保持純粹、可測。 */
export function filterPointsByRange(
	points: ChartPoint[],
	range: HistoryRange,
	today: string,
): ChartPoint[] {
	if (range === "all") return points;
	const cutoff = new Date(today).getTime() - (range - 1) * 24 * 60 * 60 * 1000;
	return points.filter((p) => new Date(p.date).getTime() >= cutoff);
}
