"use client";

import StoryRail from "@/features/feed/components/StoryRail";

const placeholders = Array.from({ length: 8 }, (_, index) => ({
	id: `loading-${index}`,
	name: "\u00a0",
	avatar: "",
}));
const tabs = [
	{ key: "recommend", label: "推薦" },
	{ key: "follow", label: "追蹤中" },
	{ key: "nearby", label: "附近商家" },
];
const ignoreSelection = () => {};

/** Reuse the real rail's responsive geometry and saved collapse preference.
 * Inert buttons avoid both interaction and prefetching placeholder seller URLs. */
export default function RouteStorySkeleton({
	market = false,
}: {
	market?: boolean;
}) {
	return (
		<div className="route-story-skeleton" aria-hidden="true" inert>
			<StoryRail
				items={placeholders}
				tabs={tabs}
				activeTab="recommend"
				onSelect={ignoreSelection}
				hideTabsOnMobile={market}
				reserveRegion
			/>
		</div>
	);
}
