import type {
	CategoryPriceRollupPoint,
	PriceObservation,
} from "@/lib/pricebook/repo";

/** `HistoricalPriceChart` 是為 crop/group 的官方行情打造的，需要
 *  `PriceObservation`（含必填的 marketCode/cropCode）；分類彙整走勢沒有單一
 *  市場或品項可以填，元件本身完全不讀這兩個欄位（只用 tradeDate/avgPrice/
 *  upperPrice/middlePrice/lowerPrice/volumeKg），所以用空字串佔位，不影響
 *  圖表渲染。 */
export function categoryRollupPointsToObservations(
	points: CategoryPriceRollupPoint[],
): PriceObservation[] {
	return points.map((point) => ({
		marketCode: "",
		cropCode: "",
		tradeDate: point.tradeDate,
		...(point.avgPrice == null ? {} : { avgPrice: point.avgPrice }),
	}));
}
