"use client";

import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Card } from "@/components/ui/Card";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { useSellerMeasure } from "@/lib/hooks/useSellerMeasures";
import MeasureCard from "./MeasureCard";
import { MEASURES_PATH } from "./measureDisplay";

/**
 * 單筆措施（`/seller/measures/[measureId]`）。停權公告頁的每張卡連到這裡；內容與列表的卡片相同，申覆與解除請求入口也相同
 * （各連到自己的獨立頁）。措施與申覆以帳號為範圍，不在賣家中心外框裡。
 */
export default function SellerMeasureDetailPage({
	measureId,
}: {
	measureId: string;
}) {
	const { measure, loading, error } = useSellerMeasure(measureId);

	return (
		<SellerPageShell
			title="措施與申覆"
			actions={
				<Link
					href={MEASURES_PATH}
					className="text-sm font-bold text-[var(--hkui-primary)]"
				>
					查看全部措施
				</Link>
			}
		>
			{loading ? (
				<SellerCardGridSkeleton
					variant="panel"
					count={1}
					className="flex flex-col gap-[var(--hkui-space-4)]"
				/>
			) : error ? (
				<Card className="p-6 text-center text-sm text-[var(--hkui-foreground-soft)]">
					{error}
				</Card>
			) : measure ? (
				<MeasureCard measure={measure} />
			) : null}
		</SellerPageShell>
	);
}
