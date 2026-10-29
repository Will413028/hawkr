import type { StoryItem } from "@/features/feed/components/StoryRail";

export const TAIWAN_CITY_CENTERS: Record<string, { lat: number; lng: number }> =
	{
		台北市: { lat: 25.0375, lng: 121.5637 },
		新北市: { lat: 25.012, lng: 121.4657 },
		桃園市: { lat: 24.9937, lng: 121.3009 },
		新竹縣: { lat: 24.8387, lng: 121.0177 },
		新竹市: { lat: 24.8138, lng: 120.9675 },
		苗栗縣: { lat: 24.5602, lng: 120.8214 },
		台中市: { lat: 24.1477, lng: 120.6736 },
		彰化縣: { lat: 24.0685, lng: 120.5575 },
		南投縣: { lat: 23.9609, lng: 120.9719 },
		雲林縣: { lat: 23.7092, lng: 120.4313 },
		嘉義縣: { lat: 23.4518, lng: 120.2555 },
		嘉義市: { lat: 23.4801, lng: 120.4491 },
		台南市: { lat: 22.9998, lng: 120.227 },
		高雄市: { lat: 22.6273, lng: 120.3014 },
		屏東縣: { lat: 22.5519, lng: 120.5488 },
		宜蘭縣: { lat: 24.7021, lng: 121.7378 },
		花蓮縣: { lat: 23.9872, lng: 121.6015 },
		台東縣: { lat: 22.7972, lng: 121.0714 },
		基隆市: { lat: 25.1276, lng: 121.7392 },
		連江縣: { lat: 26.1608, lng: 119.9517 },
		金門縣: { lat: 24.4321, lng: 118.3171 },
		澎湖縣: { lat: 23.5711, lng: 119.5793 },
	};

/** 縣市名稱對上 TAIWAN_CITY_CENTERS 的鍵：內政部寫法「臺」換成「台」。 */
export function normalizeCity(value?: string): string {
	return (value ?? "").trim().replaceAll("臺", "台");
}

export function distanceKm(
	from: { lat: number; lng: number },
	to: { lat: number; lng: number },
): number {
	const radiusKm = 6371;
	const dLat = ((to.lat - from.lat) * Math.PI) / 180;
	const dLng = ((to.lng - from.lng) * Math.PI) / 180;
	const lat1 = (from.lat * Math.PI) / 180;
	const lat2 = (to.lat * Math.PI) / 180;
	const a =
		Math.sin(dLat / 2) ** 2 +
		Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2;
	return radiusKm * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

function storyPoint(item: StoryItem) {
	const city = normalizeCity(item.city);
	return city ? TAIWAN_CITY_CENTERS[city] : undefined;
}

/** 依與目前位置的距離由近到遠排；沒有縣市或認不得縣市的排在最後，彼此維持原順序。 */
export function sortNearbyStories(
	items: StoryItem[],
	current: { lat: number; lng: number },
): StoryItem[] {
	return [...items].sort((a, b) => {
		const aPoint = storyPoint(a);
		const bPoint = storyPoint(b);
		const aDistance = aPoint
			? distanceKm(current, aPoint)
			: Number.POSITIVE_INFINITY;
		const bDistance = bPoint
			? distanceKm(current, bPoint)
			: Number.POSITIVE_INFINITY;
		if (aDistance === bDistance) return 0;
		return aDistance - bDistance;
	});
}
