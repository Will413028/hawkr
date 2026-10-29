"use client";

import * as React from "react";
import StoryRail, {
	type StoryItem,
	type StoryTab,
} from "@/features/feed/components/StoryRail";
import { sortNearbyStories } from "@/features/feed/nearbyStories";
import { useAuth } from "@/lib/auth/context";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

export type StoryTabKey = "recommend" | "follow" | "nearby";

export const FEED_TABS: StoryTab[] = [
	{ key: "recommend", label: "推薦" },
	{ key: "follow", label: "追蹤中" },
	{ key: "nearby", label: "附近商家" },
];

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
			const sorted = sortNearbyStories(items, {
				lat: position.coords.latitude,
				lng: position.coords.longitude,
			});
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
