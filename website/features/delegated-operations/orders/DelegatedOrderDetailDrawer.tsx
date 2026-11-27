"use client";

import { Drawer } from "@/components/shared/Drawer";
import DelegatedOrderDetailPanel from "./DelegatedOrderDetailPanel";

/**
 * 訂單詳情右抽屜（mockup §4.8.7，寬 400）。
 *
 * 與賣家中心的 `SellerOrderDetailDrawer` 是兩支不同的檔案，因為它們裝的是兩份不同的
 * 內容：那一支裝 `OrderDetailClient`（含出貨、取號、取消、售後），這一支裝的是唯讀
 * 面板。共用外殼再用 prop 切內容，就等於把履約禁區變成一個 prop 的值。
 */
export default function DelegatedOrderDetailDrawer({
	orderNo,
	onClose,
}: {
	/** `null` 代表沒有選中任何一筆，抽屜關閉。 */
	orderNo: string | null;
	onClose: () => void;
}) {
	return (
		<Drawer
			open={orderNo !== null}
			onClose={onClose}
			side="right"
			title="訂單詳情"
			className="max-w-md"
		>
			{orderNo ? (
				<DelegatedOrderDetailPanel key={orderNo} orderNo={orderNo} />
			) : null}
		</Drawer>
	);
}
