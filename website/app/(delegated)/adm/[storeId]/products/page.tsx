import DelegatedCapabilityGate from "@/features/delegated-operations/components/DelegatedCapabilityGate";
import { ProductsPage } from "@/features/seller-center/products";

/**
 * 代行工作區 — 商品管理（mockup image7／§6.1.8）。`/adm/enter` 兌換成功後的落地頁。
 *
 * ⚠️ `products.read` 的閘門必須在這一層。少了它，`ProductsPage` 照常掛載並無條件送出
 * 五支請求（列表一支 ＋ 四張 KPI 卡各一支）—— 它是唯一一支完全不看 `scope.canRead()`
 * 的賣家中心元件。五支全被拒之後畫面落到列表的空狀態「還沒有任何商品／去上架第一個
 * 商品開始營業吧。」，副標同時寫著「本工作階段僅可檢視商品」。那是對平台管理員
 * **謊報目標店家沒有商品**，而讀畫面的人正是要據此判斷這家店有沒有在經營。
 *
 * 與動態列表的「共 0 則動態」是同一個形狀，差別只在那一面連請求都不發、這一面是發了
 * 五支必然被拒的請求再把拒絕畫成「沒有」。
 */
export const metadata = { title: "商品管理 — 代行工作區" };

export default function Page() {
	return (
		<DelegatedCapabilityGate
			capability="products.read"
			reason="本次工作階段沒有核發商品讀取權限，因此這裡看不到這家店的商品——這不代表這家店沒有商品。需要查看請返回 ADM 重新建立工作階段。"
		>
			<ProductsPage />
		</DelegatedCapabilityGate>
	);
}
