"use client";

import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Tabs } from "@/components/ui/Tabs";
import PendingReviewsList from "./PendingReviewsList";
import ReviewedList from "./ReviewedList";

/**
 * "我的評價" — two tabs: 待評價 (completed orders awaiting a review) and
 * 已評價 (reviews the buyer has written, GET /app/me/reviews). Tabs unmount
 * the inactive panel, so the 已評價 list only fetches once opened.
 */
export default function ReviewsClient() {
	return (
		<main className="bg-transparent">
			<SellerPageShell
				title="我的評價"
				subtitle="撰寫待評價訂單，或回顧寫過的評價"
			>
				<Tabs
					fullWidth
					items={[
						{ key: "todo", label: "待評價", content: <PendingReviewsList /> },
						{ key: "done", label: "已評價", content: <ReviewedList /> },
					]}
				/>
			</SellerPageShell>
		</main>
	);
}
