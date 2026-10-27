"use client";

import { useEffect, useRef } from "react";
import { apiClient } from "@/lib/api/client";
import { useRecordHistory } from "@/lib/hooks/useRecordHistory";

export function ProductViewTracker({ productId }: { productId: string }) {
	const { enabled, record } = useRecordHistory();
	const recordedFor = useRef<string | null>(null);

	useEffect(() => {
		// Fire-and-forget view beacon, routed through apiClient.public for its
		// timeout policy and the environment's API base.
		apiClient.public.post(`/products/${productId}/view`).catch(() => {});
	}, [productId]);

	useEffect(() => {
		// Logged-in viewers also get this visit written to their personal browse
		// history (the beacon above only feeds aggregate counts). The mutation
		// handles the request + history-cache invalidation; we just gate on the
		// buyer scope and fire once per product mount — guests keep just the beacon.
		if (!enabled) return;
		if (recordedFor.current === productId) return;
		recordedFor.current = productId;
		record(productId);
	}, [productId, enabled, record]);

	return null;
}
