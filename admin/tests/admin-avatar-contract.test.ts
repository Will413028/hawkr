import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import { buildImageUrl } from "../src/utils/media";

/**
 * 買家頭像欄位在 admin 用到的兩支列表端點上的 wire 契約：
 *   `GET /api/v1/admin/after_sales/appeals`（爭議列表 → disputes/page.tsx）
 *   `GET /api/v1/admin/orders`（商家詳情的訂單分頁 → merchants/[id]/page.tsx）
 *
 * 49 §6.6 把同一張買家頭像的九個對外名字收斂成扁平成對的 `avatarKey`／`avatarUrl`
 * （admin 這邊原本叫 `buyerAvatarKey`／`buyerAvatarUrl`）。名字改了而前端沒跟＝
 * 取值恆為 undefined、頭像變成佔位字母，而 TypeScript 不會有任何意見——介面是
 * 我們自己宣告的。所以這裡的斷言同樣分兩種：
 *   正向 — 新名取得到值，且頁面的取值運算（buildImageUrl(key, "avatar")）產出正確 URL
 *   反向 — 舊名在 wire 上**確實不存在**（否則「兩個名字都讀」的相容層照樣會通過）
 *
 * 兩份 fixture 是實打回應原文（2026-08-05，
 * `curl -H "Authorization: Bearer <admin JWT>" http://127.0.0.1:19900/...`）：
 * 本機建置的 gateway（含收斂 commit）經 SSH 通道接 dev cluster 的
 * aftersales-service / order-service，因為該 commit 當時尚未部署到 test-api。
 * 為了讓檔案能讀，records 只保留前兩筆、訂單的 items 已裁掉（本檔不驗那些欄位）。
 *
 * 範圍誠實標註：本檔驗的是 wire 形狀與取值運算，**不是**畫面渲染——本 repo 沒有
 * React component 測試 harness（tests/ 全是 node:test 純函式）。
 */

const MEDIA_BASE = "https://test-media.example.com";
const originalGate = process.env.NEXT_PUBLIC_MEDIA_BASE_URL;

afterEach(() => {
	if (originalGate) process.env.NEXT_PUBLIC_MEDIA_BASE_URL = originalGate;
	else delete process.env.NEXT_PUBLIC_MEDIA_BASE_URL;
});

// GET /api/v1/admin/after_sales/appeals?appeal_state=all&page=1&page_size=3
const APPEAL_LIST_BODY = {
	data: {
		records: [
			{
				caseNo: "A674611149",
				orderNo: "O869122935",
				buyerId: "7489877415879899167",
				storeId: "7487768729113814280",
				appealState: "10",
				appealSubmittedAt: "2026-08-04T08:04:50.200162Z",
				lastRejectedAt: "2026-08-04T08:04:30.30531Z",
				refundableAmount: "225",
				buyerName: "測試買家",
				avatarKey: "public/defaults/buyer_avatar.png",
				avatarUrl:
					"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
				sellerName: "測試",
				logoKey: "public/defaults/seller_logo.png",
				logoUrl:
					"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
			},
			{
				caseNo: "A884827652",
				orderNo: "O465780801",
				buyerId: "7489760880179860732",
				storeId: "7487768729113814280",
				appealState: "30",
				appealSubmittedAt: "2026-08-03T02:57:54.505698Z",
				lastRejectedAt: "2026-08-03T02:57:54.334447Z",
				refundableAmount: "225",
				buyerName: "測試買家",
				avatarKey: "public/defaults/buyer_avatar.png",
				avatarUrl:
					"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
				sellerName: "測試",
				logoKey: "public/defaults/seller_logo.png",
				logoUrl:
					"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
			},
		],
		total: "31",
		page: "1",
		pageSize: "3",
	},
	message: "success",
	code: 0,
} as const;

// GET /api/v1/admin/orders?page=1&page_size=2（items 已裁掉，本檔不驗）
const ADMIN_ORDERS_BODY = {
	data: {
		records: [
			{
				buyerNickname: "WEI-HAO WU",
				status: "10",
				totalActualAmount: "427",
				itemCount: "1",
				storeId: "7489859548420286732",
				sellerName: "測試",
				logoKey: "public/defaults/seller_logo.png",
				logoUrl:
					"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
				shippingMethod: "3001",
				deliveryTemp: "3201",
				buyerId: "7490573739669774333",
				statusSummary: "已完成付款，等待商家備貨出貨。",
				createTime: "2026-08-05T01:09:34.946039Z",
				avatarKey: "public/defaults/buyer_avatar.png",
				avatarUrl:
					"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
				orderNo: "O312601501",
			},
			{
				buyerNickname: "flow-verify-test",
				status: "50",
				totalActualAmount: "225",
				itemCount: "1",
				storeId: "7487768729113814280",
				sellerName: "測試",
				logoKey: "public/defaults/seller_logo.png",
				logoUrl:
					"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
				shippingMethod: "3001",
				deliveryTemp: "3201",
				buyerId: "7489534863566774901",
				statusSummary: "訂單已完成，感謝您的購買！",
				createTime: "2026-08-04T22:48:45.267637Z",
				avatarKey: "public/defaults/buyer_avatar.png",
				avatarUrl:
					"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
				orderNo: "O832117102",
			},
		],
		total: "204",
	},
	message: "success",
	code: 0,
} as const;

// 收斂掉的舊名，一個都不准回到 wire 上。與 backend
// gateway/internal/handler/avatar_jsonkey_test.go 的 banned 清單同一份意圖。
const BANNED_AVATAR_KEYS = [
	"buyerAvatarKey",
	"buyerAvatarUrl",
	"userAvatarKey",
	"userAvatarUrl",
	"targetAvatarKey",
	"targetAvatarUrl",
	"senderAvatar",
	"userAvatar",
	"avatar",
] as const;

function assertFlatAvatarPair(
	row: Record<string, unknown>,
	label: string,
): void {
	assert.ok("avatarKey" in row, `${label} 缺 avatarKey（SSOT objectKey）`);
	assert.ok("avatarUrl" in row, `${label} 缺 avatarUrl`);
	for (const banned of BANNED_AVATAR_KEYS) {
		assert.equal(
			banned in row,
			false,
			`${label} 出現舊欄位 ${banned} —— 收斂被回退了`,
		);
	}
}

test("爭議列表每筆的買家頭像是扁平成對 avatarKey/avatarUrl", () => {
	const records = APPEAL_LIST_BODY.data.records;
	assert.ok(records.length > 0, "fixture 為空的話下面整段都不會執行");
	for (const r of records) {
		assertFlatAvatarPair(r, `appeal ${r.caseNo}`);
		assert.ok(r.avatarKey.length > 0);
		assert.ok(r.avatarUrl.includes(`/i/original/${r.avatarKey}`));
		// 同一筆裡賣家 logo 維持 logoKey/logoUrl —— 拉平不該讓兩個主體撞名。
		assert.notEqual(r.avatarKey, r.logoKey);
	}
});

test("商家詳情的訂單列表每筆的買家頭像是扁平成對 avatarKey/avatarUrl", () => {
	const records = ADMIN_ORDERS_BODY.data.records;
	assert.ok(records.length > 0);
	for (const o of records) {
		assertFlatAvatarPair(o, `order ${o.orderNo}`);
		assert.ok(o.avatarKey.length > 0);
		assert.notEqual(o.avatarKey, o.logoKey);
	}
});

test("merchants 頁的取值運算：avatarKey 過 buildImageUrl 拿到 avatar preset URL", () => {
	process.env.NEXT_PUBLIC_MEDIA_BASE_URL = MEDIA_BASE;
	const o = ADMIN_ORDERS_BODY.data.records[0];
	assert.equal(
		buildImageUrl(o.avatarKey, "avatar"),
		`${MEDIA_BASE}/i/avatar/${o.avatarKey}`,
	);
	// 讀 avatarUrl 會拿到 /i/original/ 原圖再被剝回 key，繞一圈才回到同一張圖；
	// 這條釘住「該讀 Key 不該讀 Url」的理由，而不只是宣告它。
	assert.equal(
		buildImageUrl(o.avatarUrl, "avatar"),
		`${MEDIA_BASE}/i/avatar/${o.avatarKey}`,
	);
	assert.ok(o.avatarUrl.startsWith(`${MEDIA_BASE}/i/original/`));
});

test("讀錯 key 拿到的是 undefined，buildImageUrl 回空字串而非破圖 src", () => {
	process.env.NEXT_PUBLIC_MEDIA_BASE_URL = MEDIA_BASE;
	const o = ADMIN_ORDERS_BODY.data.records[0] as Record<string, string>;
	assert.equal(o.buyerAvatarUrl, undefined);
	assert.equal(buildImageUrl(o.buyerAvatarUrl, "avatar"), "");
});
