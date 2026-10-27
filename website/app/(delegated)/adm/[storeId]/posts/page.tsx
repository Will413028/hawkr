import DelegatedCapabilityGate from "@/features/delegated-operations/components/DelegatedCapabilityGate";
import SellerPostsClient from "@/features/seller-center/posts/SellerPostsClient";

/**
 * 代行工作區 — 動態管理（§6.1.9）。
 *
 * ⚠️ `posts.read` 的閘門必須在這一層。少了它，`SellerPostsClient` 會照常渲染：
 * `scope.canRead("posts")` 為 false ⇒ 一個請求都不發 ⇒ `items` 是空陣列 ⇒ 標題寫
 * 「共 0 則動態」、卡片裡是「發布第一篇」CTA。dev 實測同一家店在有 `posts.read` 的
 * 工作階段下是 11 則、DB 未刪 —— 也就是說那個畫面對平台管理員**謊報了目標店家的
 * 經營狀態**，而且沒有任何東西會報錯。
 */
export const metadata = { title: "動態管理 — 代行工作區" };

export default function Page() {
	return (
		<DelegatedCapabilityGate
			capability="posts.read"
			reason="本次工作階段沒有核發動態讀取權限，因此這裡看不到這家店的動態內容——這不代表這家店沒有動態。需要查看請返回 ADM 重新建立工作階段。"
		>
			<SellerPostsClient />
		</DelegatedCapabilityGate>
	);
}
