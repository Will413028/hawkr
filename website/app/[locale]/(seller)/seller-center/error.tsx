"use client";

import ErrorState from "@/components/shared/ErrorState";

/**
 * Seller-center error boundary. Without one, a thrown render / data error in the
 * workspace bubbles to the root boundary and drops the seller out of their
 * management context. Keep the recovery inside the workspace: retry the segment,
 * or step back to the dashboard.
 */
export default function SellerCenterError({
	error,
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	return (
		<ErrorState
			error={error}
			reset={reset}
			title="賣家後台載入時出了點狀況"
			description="這個頁面沒有載入成功，你的商品與訂單資料都安全。再試一次，或回到後台首頁。"
			homeHref="/seller-center"
			homeLabel="回後台首頁"
		/>
	);
}
