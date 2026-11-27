"use client";

import { useEffect, useRef } from "react";
import { sellerBeaconClient } from "@/lib/connect/seller-public";
import { useRecordHistory } from "@/lib/hooks/useRecordHistory";

export function ProductViewTracker({ productId }: { productId: string }) {
	const { canRecord, record } = useRecordHistory();
	const recordedFor = useRef<string | null>(null);

	useEffect(() => {
		sellerBeaconClient()
			.recordProductView({ productId })
			.catch(() => {});
	}, [productId]);

	useEffect(() => {
		if (!canRecord) return;
		if (recordedFor.current === productId) return;
		recordedFor.current = productId;
		record(productId);
	}, [productId, canRecord, record]);

	return null;
}
