"use client";

import * as React from "react";
import StoryRail, {
	type StoryItem,
	type StoryTab,
} from "@/features/feed/components/StoryRail";
import { useAuth } from "@/lib/auth/context";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

export type StoryTabKey = "recommend" | "follow" | "nearby";

export const FEED_TABS: StoryTab[] = [
	{ key: "recommend", label: "推薦" },
	{ key: "follow", label: "追蹤中" },
	{ key: "nearby", label: "附近商家" },
];

const TAIWAN_CITY_CENTERS: Record<string, { lat: number; lng: number }> = {
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

interface FeedStoriesProps {
	items: StoryItem[];
	activeTab: StoryTabKey;
	onTabChange: (key: StoryTabKey) => void;
	/**
	 * Preview mode for seller-side import preview shell. Tabs render
	 * with the same layout but click → no-op (no navigation, no buyer
	 * session gating). Rail stays interactive-looking but `onTabChange`
	 * is never invoked.
	 */
	previewMode?: boolean;
	hideTabsOnMobile?: boolean;
	/**
	 * Keep the rail's region sub-line height reserved from first paint. Set by
	 * the feed shell when it will enrich the rail with region-bearing sellers
	 * client-side, so the server→client story swap does not grow the rail and
	 * shift the post grid. See StoryRail's `reserveRegion`.
	 */
	reserveRegion?: boolean;
}

function normalizeCity(value?: string) {
	return (value ?? "")
		.trim()
		.replace(/^臺北市/, "台北市")
		.replace(/^臺中市/, "台中市")
		.replace(/^臺南市/, "台南市")
		.replace(/^臺東縣/, "台東縣");
}

function requestBrowserLocation(): Promise<GeolocationPosition> {
	return new Promise((resolve, reject) => {
		if (!navigator.geolocation) {
			reject(new Error("GeolocationUnsupported"));
			return;
		}
		navigator.geolocation.getCurrentPosition(resolve, reject, {
			enableHighAccuracy: false,
			maximumAge: 1000 * 60 * 10,
			timeout: 8000,
		});
	});
}

function distanceKm(
	from: { lat: number; lng: number },
	to: { lat: number; lng: number },
) {
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
	if (
		Number.isFinite(item.latitude) &&
		Number.isFinite(item.longitude) &&
		item.latitude !== undefined &&
		item.longitude !== undefined
	) {
		return { lat: item.latitude, lng: item.longitude };
	}
	const city = normalizeCity(item.city);
	return city ? TAIWAN_CITY_CENTERS[city] : undefined;
}

function sortNearbyStories(
	items: StoryItem[],
	position: GeolocationPosition,
): StoryItem[] {
	const current = {
		lat: position.coords.latitude,
		lng: position.coords.longitude,
	};
	return [...items].sort((a, b) => {
		const aPoint = storyPoint(a);
		const bPoint = storyPoint(b);
		const aDistance = aPoint
			? distanceKm(current, aPoint)
			: Number.POSITIVE_INFINITY;
		const bDistance = bPoint
			? distanceKm(current, bPoint)
			: Number.POSITIVE_INFINITY;
		return aDistance - bDistance;
	});
}

/**
 * Top story rail + tab strip on the feed surface. The rail items stay
 * the SAME for 推薦 and 追蹤中 — the 追蹤中 tab no longer swaps the
 * rail to "your follows" because the visible behaviour of that tab is
 * filtering the post grid below (see `FeedClientShell` /
 * `InfiniteFeed`). 附近商家 still sorts the rail by browser location.
 *
 * Tab state is owned by the parent shell so the post grid can react;
 * this component is purely the rail + tabs UI plus location handling.
 */
export default function FeedStories({
	items,
	activeTab,
	onTabChange,
	previewMode = false,
	hideTabsOnMobile = false,
	reserveRegion = false,
}: FeedStoriesProps) {
	const { hydrated, activeRole, isLoggedIn } = useAuth();
	const isBuyer = activeRole === "buyer";
	const [nearbyItems, setNearbyItems] = React.useState<StoryItem[] | null>(
		null,
	);
	const [loadingNearby, setLoadingNearby] = React.useState(false);
	const [emptyMessage, setEmptyMessage] = React.useState<React.ReactNode>(null);
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可查看個人化商家",
			description:
				"登入 Hawkr 後，可以查看追蹤中的商家，並依你的所在位置整理附近商家。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);

	const ensureBuyerSession = (): boolean => {
		if (!hydrated) {
			setEmptyMessage("正在確認登入狀態...");
			return false;
		}
		if (!isLoggedIn) {
			openLoginRequired();
			return false;
		}
		if (!isBuyer) {
			setEmptyMessage("請切換到買家帳號後再使用追蹤與附近商家。");
			return false;
		}
		return true;
	};

	const loadNearby = React.useCallback(async () => {
		if (nearbyItems) return;
		setLoadingNearby(true);
		setEmptyMessage(null);
		try {
			const position = await requestBrowserLocation();
			const sorted = sortNearbyStories(items, position);
			setNearbyItems(sorted);
			setEmptyMessage(
				sorted.length
					? null
					: "附近商家需要定位與商家所在地資料，目前暫時沒有可排序的商家。",
			);
		} catch {
			setNearbyItems([]);
			setEmptyMessage("無法取得定位權限，因此暫時不能排序附近商家。");
		} finally {
			setLoadingNearby(false);
		}
	}, [items, nearbyItems]);

	const handleTabChange = (key: string) => {
		if (previewMode) return;
		const nextKey = key as StoryTabKey;
		if (nextKey === "recommend") {
			onTabChange("recommend");
			setEmptyMessage(null);
			return;
		}
		if (!ensureBuyerSession()) return;
		onTabChange(nextKey);
		if (nextKey === "follow") {
			// Rail stays as-is; the post grid below does the actual filtering.
			// Empty-state copy is owned by InfiniteFeed when the filter yields zero.
			setEmptyMessage(null);
			return;
		}
		if (nextKey === "nearby") {
			void loadNearby();
		}
	};

	const visibleItems = activeTab === "nearby" ? (nearbyItems ?? items) : items;

	return (
		<StoryRail
			items={visibleItems}
			tabs={FEED_TABS}
			activeTab={activeTab}
			onTabChange={handleTabChange}
			isLoading={activeTab === "nearby" && loadingNearby}
			emptyMessage={emptyMessage}
			hideTabsOnMobile={hideTabsOnMobile}
			reserveRegion={reserveRegion}
		/>
	);
}
