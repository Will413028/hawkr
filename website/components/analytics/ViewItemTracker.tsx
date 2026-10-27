"use client";

import { useEffect } from "react";
import { trackViewItem } from "@/lib/analytics/events";
import type { GA4Item } from "@/lib/analytics/types";

/** Fires view_item once on mount with a server-built GA4 item. */
export default function ViewItemTracker({ item }: { item: GA4Item }) {
	useEffect(() => {
		trackViewItem(item);
	}, [item]);
	return null;
}
