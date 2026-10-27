import DelegatedCapabilityGate from "@/features/delegated-operations/components/DelegatedCapabilityGate";
import PostComposeClient from "@/features/seller-center/posts/PostComposeClient";

/**
 * 代行工作區 — 新增貼文（mockup image9／§6.1.9）。
 *
 * ⚠️ `posts.write` 的閘門與 `products/new` 同一個判準（那一頁在缺 `products.write`
 * 時直接空狀態）。少了它，整頁照常渲染：編輯器、標籤、商品關聯、預覽、代行原因、
 * 通知賣家全都在，只有送出鍵是 `disabled` —— 一個從頭到尾都不可能成功的工作
 * 檯面。操作者會把內容打完才發現送不出去，而**一顆停用的按鈕仍在宣稱「這件事本來
 * 做得到」**（L3857 對「不渲染 vs disabled」的裁決，這裡是同一條）。
 */
export const metadata = { title: "新增貼文 — 代行工作區" };

export default function Page() {
	return (
		<DelegatedCapabilityGate
			capability="posts.write"
			reason="本次工作階段沒有核發動態編輯權限，無法替這家店建立動態。需要發布請返回 ADM 重新建立工作階段。"
			backSegment="posts"
			backLabel="返回動態管理"
		>
			<PostComposeClient />
		</DelegatedCapabilityGate>
	);
}
