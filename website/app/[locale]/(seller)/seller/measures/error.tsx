"use client";

import ErrorState from "@/components/shared/ErrorState";

/**
 * 措施與申覆頁的 error boundary：不在賣家中心外框裡，錯誤停在這一段，重試或回措施列表。
 */
export default function SellerMeasuresError({
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
			title="措施與申覆載入時出了點狀況"
			description="這個頁面沒有載入成功，你的帳號資料都安全。再試一次，或回到措施與申覆。"
			homeHref="/seller/measures"
			homeLabel="回措施與申覆"
		/>
	);
}
