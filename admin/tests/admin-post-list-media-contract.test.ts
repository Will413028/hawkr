import assert from "node:assert/strict";
import test from "node:test";
import {
	type AdminPostListItem,
	selectPostThumbKey,
} from "../src/lib/posts/adminList";

/**
 * `GET /api/v1/admin/posts` 的欄名與媒體欄位契約。
 *
 * RECORDS 的資料值來自 dev gateway 的實打回應（2026-08-05 `curl` 原文），**欄名**
 * 則對齊 backend-payuni PR #2235 的收斂結果（`gateway/internal/handler/post.go`
 * `AdminPostListItemData` / `toAdminPostListItems`）：
 *   - JSON body 一律 camelCase（`post_id` → `postId` …）
 *   - 貼文圖只有 `imageKeys` / `imageUrls`，且**沒有 omitempty**：
 *     `emptyIfNil` / `buildURLsAlways` 保證無圖時是 `[]`，不是 undefined
 *
 * 手寫 wire shape 正是上一個 bug 的成因：貼文管理頁自己宣告了 `images: string[]`，
 * gateway 從來沒送過這個欄位，`post.images?.length > 0` 恆為 false，
 * 111 則貼文的縮圖一張都沒渲染，TypeScript 全程無異議
 * （hawkr-website#430 B1）。
 *
 * 因此本檔的斷言分兩種，缺一不可：
 *   正向 — 選取器對實打回應取得到 objectKey
 *   反向 — 舊欄位名（`images` / 單數 imageKey / snake_case 全組）在 wire 上
 *          **確實不存在**，且只餵舊欄名的 payload 取不到任何東西（否則
 *          「兩個名字都讀」的相容層照樣會通過）
 */

// 依序：帶圖上架貼文 / 帶圖已下架貼文 / 無圖且有檢舉的貼文 / 無圖已刪除貼文。
const RECORDS = [
	{
		sellerName: "陳記",
		content: "C6 契約驗證貼文（帶圖，模擬器 UI 驗證用）",
		status: "published",
		imageKeys: [
			"public/post/2685914682/20260804/8bddff6a-0c90-4cfb-ace0-8c414175ecd2.png",
		],
		imageUrls: [
			"https://test-media.example.com/i/original/public/post/2685914682/20260804/8bddff6a-0c90-4cfb-ace0-8c414175ecd2.png",
		],
		postId: "7490317712170336958",
		storeId: "2685914682",
		createdAt: 1785830906,
		reportCount: 0,
	},
	{
		sellerName: "測試",
		content: "高麗菜",
		status: "hidden",
		hiddenReason: "D",
		imageKeys: [
			"public/post/2644305813/20260803/22d252b3-ee0f-4de6-8dd7-0aee296ab2ad.jpg",
		],
		imageUrls: [
			"https://test-media.example.com/i/original/public/post/2644305813/20260803/22d252b3-ee0f-4de6-8dd7-0aee296ab2ad.jpg",
		],
		postId: "7489863941948315224",
		storeId: "7489859548420286732",
		createdAt: 1785722719,
		reportCount: 0,
	},
	{
		sellerName: "verify-A",
		content: "verify post",
		status: "hidden",
		hiddenReason: "D",
		latestReportReason: "SPAM",
		imageKeys: [],
		imageUrls: [],
		postId: "7489343561842560048",
		storeId: "7489343558508297313",
		createdAt: 1785598650,
		latestReportAt: 1785598651,
		reportCount: 1,
	},
	{
		sellerName: "測試",
		content: "postdeploy-flow40-reverify",
		status: "deleted",
		imageKeys: [],
		imageUrls: [],
		postId: "7490480635440241198",
		storeId: "7487768729113814280",
		createdAt: 1785869749,
		reportCount: 0,
	},
] as unknown as AdminPostListItem[];

const WITH_IMAGE = RECORDS.slice(0, 2);
const WITHOUT_IMAGE = RECORDS.slice(2);

/** 收斂前的舊欄名，一個都不該再出現在 wire 上。 */
const RETIRED_KEYS = [
	"images",
	"image_keys",
	"imageKey",
	"imageUrl",
	"post_id",
	"store_id",
	"seller_name",
	"report_count",
	"latest_report_reason",
	"latest_report_at",
	"hidden_reason",
	"created_at",
];

test("wire 的貼文圖只有 imageKeys/imageUrls —— 舊欄名一個都不存在", () => {
	for (const post of RECORDS) {
		const raw = post as unknown as Record<string, unknown>;
		for (const k of RETIRED_KEYS) {
			assert.equal(
				k in raw,
				false,
				`wire 竟然有 ${k} —— 舊欄位復活了（${post.postId}）`,
			);
		}
		assert.ok(Array.isArray(post.imageKeys), "imageKeys 必為陣列（無圖為 []）");
		assert.ok(Array.isArray(post.imageUrls), "imageUrls 必為陣列（無圖為 []）");
	}
});

test("只給舊欄名的 payload 取不到任何 key（fallback 復活會在這裡轉紅）", () => {
	const legacyOnly = {
		post_id: "7490317712170336958",
		store_id: "2685914682",
		seller_name: "陳記",
		content: "legacy",
		status: "published",
		report_count: 0,
		created_at: 1785830906,
		// 收斂前的單數欄 + 舊命名的複數欄，兩種都不該被讀到
		imageKey: "public/post/legacy/single.png",
		imageUrl: "https://test-media.example.com/i/original/legacy.png",
		images: ["public/post/legacy/a.png"],
		image_keys: ["public/post/legacy/b.png"],
	} as unknown as AdminPostListItem;
	assert.equal(
		selectPostThumbKey(legacyOnly),
		"",
		"讀到了舊欄位 —— 相容層復活了",
	);
});

test("帶圖貼文取得到 objectKey（修前這裡全是空字串，整頁一張縮圖都沒有）", () => {
	for (const post of WITH_IMAGE) {
		const key = selectPostThumbKey(post);
		assert.equal(typeof key, "string");
		assert.notEqual(key, "", "空 key 會讓縮圖整個不渲染（修前的症狀）");
		assert.equal(key, post.imageKeys?.[0]);
		assert.ok(
			key.startsWith("public/post/"),
			`貼文圖 objectKey 前綴應為 public/post/，實得 ${key}`,
		);
		assert.equal(
			key.startsWith("http"),
			false,
			"取到的必須是 objectKey，不是顯示 URL",
		);
	}
});

test('無圖貼文回空字串，不會渲染成 src="" 的破圖框', () => {
	for (const post of WITHOUT_IMAGE) {
		// 收斂後無圖是 []，不再是「欄位不存在」—— 前端不得再用 undefined 判斷有無圖
		assert.deepEqual(post.imageKeys, []);
		assert.deepEqual(post.imageUrls, []);
		assert.equal(selectPostThumbKey(post), "");
	}
});

test("imageKeys 為空陣列 / 全空字串時同樣回空字串", () => {
	const base = WITH_IMAGE[0];
	assert.equal(selectPostThumbKey({ ...base, imageKeys: [] }), "");
	assert.equal(selectPostThumbKey({ ...base, imageKeys: ["", "  "] }), "");
	// 前面有空值時跳過，取第一張真的有 key 的圖。
	assert.equal(
		selectPostThumbKey({ ...base, imageKeys: ["", base.imageKeys?.[0] ?? ""] }),
		base.imageKeys?.[0],
	);
});

test("imageUrls 是固定 /i/original/ 的原圖 —— 縮圖不能讀它", () => {
	for (const post of WITH_IMAGE) {
		const url = post.imageUrls?.[0] ?? "";
		assert.ok(url.includes(`/i/original/${post.imageKeys?.[0]}`));
		assert.notEqual(
			selectPostThumbKey(post),
			url,
			"選取器回的必須是 key，不是 original URL",
		);
	}
});

test("postId / storeId 是字串（TSID 超過 2^53，轉 number 會失精打錯貼文）", () => {
	for (const post of RECORDS) {
		assert.equal(typeof post.postId, "string");
		assert.equal(typeof post.storeId, "string");
	}
	const tsid = "7490317712170336958";
	assert.notEqual(String(Number(tsid)), tsid, "這正是不能轉 number 的原因");
});

test("createdAt / latestReportAt 是 Unix 秒，不是毫秒", () => {
	for (const post of RECORDS) {
		// 2020-01-01 ~ 2100-01-01 的秒級區間；毫秒值會遠大於上界。
		assert.ok(
			post.createdAt > 1_577_836_800 && post.createdAt < 4_102_444_800,
			`createdAt ${post.createdAt} 不在秒級區間（頁面會 ×1000 顯示）`,
		);
	}
	const reported = RECORDS.find((p) => p.latestReportAt);
	assert.ok(reported, "fixture 需要一筆有檢舉的貼文");
	assert.ok(
		(reported.latestReportAt ?? 0) > 1_577_836_800 &&
			(reported.latestReportAt ?? 0) < 4_102_444_800,
	);
});
