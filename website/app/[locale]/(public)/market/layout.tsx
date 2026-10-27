import type { ReactNode } from "react";
import "@/styles/features/market.css";
import "@/styles/features/market-products-list.css";
/* No feed-components.css here on purpose: the only thing /market/* used it
 * for was the "推薦商家" rail, and StoryRail is now pure Tailwind — so a
 * hard load of /market/* no longer needs feed.css to paint correctly. */

export default function MarketLayout({ children }: { children: ReactNode }) {
	return children;
}
