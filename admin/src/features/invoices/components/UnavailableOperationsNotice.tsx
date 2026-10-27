import { MaterialSymbol } from "@hawkr/icons/material-symbol";
/**
 * 「這一頁的某些動作本波不渲染」的統一告示。
 *
 * ## 現在只剩一個使用者：A11（手動開立發票）
 *
 * 2026-09-02 之前，A03／A06／A08／A13 四頁也掛著它，理由寫的是「invoice-service 那一側
 * 整批沒有實作」。**那個理由是錯的**：九支 ADM 寫入 RPC 全部都在，當時的判斷來自拿
 * 猜的路徑名去打 dev（`complaints/{id}/decisions` 真名是 `.../actions`）。四頁已接上，
 * 告示一併移除。詳見 `features/invoices/admin-actions.ts` 的檔頭。
 *
 * ⇒ **這則告示的門檻因此要拉高**：掛上去之前，先用**路由註冊裡的路徑名**（gateway
 * `internal/router/constants.go`）實打一次。404 只證明你打錯路徑，不證明後端沒做。
 *
 * A11 剩下的兩項是真的缺口，而且缺在契約不是實作：
 *   - 「沿用訂單快照」的來源需要 `buyer_id`，本服務拿不到（拿 ADM 手填的頂替會讓這一頁
 *     變成 IDOR 入口）
 *   - 載具三欄在 proto 與 DTO 上都有，但 `AdminIssuance.buildDocument` 一個都沒設
 *     ⇒ 送過去會被靜默丟掉，開出來是一張無載具的發票
 *
 * ## 為什麼是告示而不是靜默省略
 *
 * 靜默省略的話，管理員會以為自己權限不足或畫面壞了，然後去問人。一句話講清楚
 * 「這些動作還沒開放」比一片空白便宜。
 *
 * ## 什麼時候刪掉它
 *
 * 對應缺口補上之後，把該頁的動作補上、從這裡的清單移除；清單空了就整個元件刪掉。
 * 它不是永久設施。
 */
export default function UnavailableOperationsNotice({
	operations,
}: {
	/** 本波不渲染的動作名稱（使用者看得懂的說法，不是端點編號）。 */
	operations: string[];
}) {
	if (operations.length === 0) return null;
	return (
		<section className="invoice-unavailable-notice" role="status">
			<MaterialSymbol name="info" />
			<div>
				<p className="invoice-unavailable-notice__title">這一頁目前只能檢視</p>
				<p className="invoice-unavailable-notice__text">
					下列動作的後端端點尚未上線，因此本頁不提供對應按鈕：
					{operations.join("、")}。
				</p>
			</div>
		</section>
	);
}
