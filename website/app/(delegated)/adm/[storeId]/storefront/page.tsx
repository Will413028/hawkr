import DelegatedCapabilityGate from "@/features/delegated-operations/components/DelegatedCapabilityGate";
import SellerProfileClient from "@/features/seller-center/profile/SellerProfileClient";

/**
 * 代行工作區 — 商店設定（mockup image4／§6.1.7）。
 *
 * 與 `/seller-center/me` 是同一個元件：代行只換資料層（`DelegatedSellerDataProvider`
 * 注入的 scope），不換畫面。**不得**為了傳 storeId 而改元件簽章 —— 目標店家由
 * operation credential 對應的工作階段決定，路由上的 `{storeId}` 只用於顯示與 URL
 * 可讀性；照著路由參數再比對一次，等於讓前端對授權有第二個意見。
 *
 * ⚠️ `storefront.read` 的閘門必須在這一層。少了它，`SellerProfileClient` 的
 * `scope.canRead("storefront")` 為 false ⇒ 查詢不發 ⇒ `data` 恆為 null ⇒ 畫面落到它
 * 的 `!data` 分支「無法載入商店資料」。那是**技術故障文案**：它叫操作者去重試、去找
 * 工程，而重試一萬次也不會有資料。同一個工作階段的訂單頁在缺 `orders.read` 時給的是
 * 授權文案 —— 三條路徑三套標準，這一條對齊過來。
 */
export const metadata = { title: "商店設定 — 代行工作區" };

export default function Page() {
	return (
		<DelegatedCapabilityGate
			capability="storefront.read"
			reason="本次工作階段沒有核發店面讀取權限，因此看不到也改不了這家店的商店設定。需要調整請返回 ADM 重新建立工作階段。"
		>
			<SellerProfileClient />
		</DelegatedCapabilityGate>
	);
}
