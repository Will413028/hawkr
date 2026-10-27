import type { Metadata } from "next";
import { VendorRfqLanding } from "@/features/vendor-portal/VendorRfqLanding";
import { noindexMetadata } from "@/lib/seo/metadata";

// 站外業者從一次性邀請信連結落地的頁面（#2539 E1）。免登入、無平台 session，
// 憑證是 URL fragment 裡的邀請 secret。
//
// 刻意不放在 HAWKR_AGENT_ENABLED 後面：存取憑證就是那個 secret，沒有它只會看到
// 「連結無效」，頁面本身沒有暴露面；而 gate 的實際風險是 prod 少設一個變數，
// 業者點開邀請信又看到 404——那正是這一頁被建出來要修的病。
//
// noindex：被索引的邀請 URL 等於把一次性連結洩出去或浪費掉。
export const metadata: Metadata = {
	title: "外部供應商報價入口",
	...noindexMetadata(),
};

export default function Page() {
	return <VendorRfqLanding />;
}
