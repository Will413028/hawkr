"use client";

import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import DelegatedAreaUnauthorized from "../components/DelegatedAreaUnauthorized";
import DelegatedOrderDetailPanel from "./DelegatedOrderDetailPanel";
import { useDelegatedOrdersReadable } from "./useDelegatedOrders";

/**
 * 訂單詳情獨立頁的外殼。內容與抽屜共用 `DelegatedOrderDetailPanel`。
 *
 * capability 不足時**一個請求都不發**，直接顯示與 `[...area]` 空狀態同一句文案 ——
 * 打一支必然 403 的端點來探測，會在稽核表製造一批 FAILED 的讀取列，把「有人試圖跨店
 * 讀取」這個安全訊號淹掉。
 */
export default function DelegatedOrderDetailRoute({
	storeId,
	orderNo,
}: {
	storeId: string;
	orderNo: string;
}) {
	const readable = useDelegatedOrdersReadable();

	if (!readable) {
		return (
			<DelegatedAreaUnauthorized
				reason="本次工作階段沒有核發訂單讀取權限。需要查看訂單請返回 ADM 重新建立工作階段。"
				backHref={`/adm/${encodeURIComponent(storeId)}/orders`}
				backLabel="返回訂單列表"
			/>
		);
	}

	return (
		<SellerPageShell
			title="訂單詳情"
			subtitle="沿用賣家工作區的查詢體驗，但履約仍由賣家本人負責"
			actions={
				<Link
					className="seller-table__action"
					href={`/adm/${encodeURIComponent(storeId)}/orders`}
				>
					返回訂單列表
				</Link>
			}
		>
			<div className="seller-card">
				<DelegatedOrderDetailPanel key={orderNo} orderNo={orderNo} />
			</div>
		</SellerPageShell>
	);
}
