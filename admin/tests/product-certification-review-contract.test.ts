// 時區必須在任何 Date 運算之前釘死。`toUtcDayStart` / `toUtcDayEnd` 的整個重點就是
// 「本地那一天 → UTC 瞬間」，跟著跑測試那台機器的時區走就等於什麼都沒斷言。
process.env.TZ = "Asia/Taipei";

import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";
import {
	factoryScopeHint,
	isFactoryScopedCertification,
	parsePaged,
	REVERIFY_HINT,
	reverifyResultNoticeText,
	reviewIdempotencyKey,
	reviewResultNoticeText,
	toUtcDayEnd,
	toUtcDayStart,
} from "../src/lib/product-certifications";

/**
 * 商品認證審核（ADM）的契約 gate。
 *
 * 對應 backend-payuni 19 號設計書 §9.2（Admin 端點）／§9.3（重新查驗）／§8.5（3703 CAS
 * 是工廠級編號）／§14.2（ADM 前端清單）。
 *
 * ## 這支在守什麼
 *
 * 這一整條流程的失效方式**全部是靜默的**：
 *
 * - Sidebar 漏登記 ⇒ 頁面存在、build 綠、測試綠，但沒有人找得到它，所有待審申請永遠
 *   停在佇列裡。**沒有任何既有 gate 擋得住這件事**，所以下面第一條測試就是它。
 * - `govVerifyPayload` 少渲染幾個欄位 ⇒ 畫面看起來完全正常，只是審核者變成在無依據
 *   核准。**這也不會有任何東西報錯**。
 * - 分頁三欄照數字接 ⇒ TypeScript 不會抱怨（JSON 沒有型別檢查），`Math.ceil(NaN)`
 *   一路沉默到執行期。
 */

const LIST_PAGE = "src/app/admin/product-certifications/page.tsx";
const DETAIL_PAGE = "src/app/admin/product-certifications/[id]/page.tsx";
const SERVICE = "src/services/product-certifications.ts";
const SIDEBAR = "src/components/Layout/Sidebar.tsx";
const LIB = "src/lib/product-certifications.ts";

const read = (path: string) => readFile(path, "utf8");

test("兩個路由檔都存在（頁面本身）", async () => {
	await Promise.all([access(LIST_PAGE), access(DETAIL_PAGE), access(SERVICE)]);
});

test("Sidebar 有「商品認證審核」入口，且掛在「賣家 / 商品」群組", async () => {
	const sidebar = await read(SIDEBAR);

	// 先鎖定「賣家 / 商品」那一段：光是「檔案裡有這個字串」不夠——掛錯群組的入口
	// 使用者一樣找不到（他會去「賣家 / 商品」底下找，因為審的是商品的認證）。
	const sectionStart = sidebar.indexOf('section: "賣家 / 商品"');
	assert.ok(
		sectionStart > 0,
		"找不到「賣家 / 商品」群組——群組改名就要同步改這支測試",
	);
	const nextSection = sidebar.indexOf("section:", sectionStart + 1);
	const group = sidebar.slice(
		sectionStart,
		nextSection > 0 ? nextSection : undefined,
	);

	assert.match(
		group,
		/href: "\/admin\/product-certifications"/u,
		"「賣家 / 商品」群組裡沒有商品認證審核的入口。漏加不會有任何 gate 報錯——" +
			"頁面會存在但沒有人找得到，所有待審申請永遠停在佇列裡。",
	);
	assert.match(group, /label: "商品認證審核"/u);
	// 緊鄰商品管理：兩者的對象都是商品，分開放會讓人以為是兩個不相干的模組。
	assert.ok(
		group.indexOf('href: "/admin/products"') <
			group.indexOf('href: "/admin/product-certifications"'),
		"認證審核應該排在「商品管理」之後、與它相鄰",
	);
});

test("service 的每一支端點都掛在 admin 認證申請的路徑前綴下", async () => {
	const service = await read(SERVICE);

	assert.match(
		service,
		/const BASE = "\/api\/v1\/admin\/product_certification_applications"/u,
		"路徑前綴改了？gateway 掛的是 /admin/product_certification_applications",
	);

	// 🔴 /admin/certifications 是商家認證（KYC，18 號設計書），完全不同的東西。
	// 兩者都叫「認證」，打錯不會 404（那個端點存在），只會拿到別人的資料。
	assert.doesNotMatch(
		service,
		/["'`]\/api\/v1\/admin\/certifications/u,
		"打到了商家 KYC 的端點，不是商品認證",
	);

	// 端點清單（19 §9.2 / §9.3 共七支，approve/reject/revoke 共用同一條路由樣板）。
	// 用 RegExp 而不是字串比對：biome 的 noTemplateCurlyInString 會把普通字串裡的
	// 插值語法當成寫壞的樣板字串。
	const endpoints: Array<[string, RegExp]> = [
		["佇列列表", /`\$\{BASE\}\?\$\{buildQueueParams\(q\)\}`/u],
		["單筆詳情", /`\$\{BASE\}\/\$\{encodeURIComponent\(id\)\}`/u],
		["審核事件流水", /\/audit_logs\?\$\{params\}`/u],
		["核准／駁回／撤銷", /\/\$\{action\}`/u],
		["重新查驗", /\/reverify`/u],
	];
	for (const [name, pattern] of endpoints) {
		assert.match(service, pattern, `service 少了端點：${name}`);
	}
});

test("有副作用的四支一律走 POST（reverify 尤其不能做成 GET）", async () => {
	const service = await read(SERVICE);

	// 取出 reverify 函式體，確認它用的是 api.post。
	const reverify = service.slice(
		service.indexOf("export async function reverifyCertificationApplication"),
	);
	assert.match(
		reverify.slice(0, 400),
		/api\.post</u,
		"reverify 必須是 POST：它會打政府開放資料平台（消耗每日配額）並寫 DB，" +
			"GET 會被瀏覽器、代理與 prefetch 重放（19 §9.3）。",
	);

	const review = service.slice(
		service.indexOf("export async function reviewCertificationApplication"),
	);
	assert.match(review.slice(0, 400), /api\.post</u);

	// 反向：不得出現以 revalidate / verify 當 query flag 的 GET 變體。
	assert.doesNotMatch(service, /revalidate=true/u);
});

test("詳情頁把 govVerifyPayload 的每一個判斷欄位都渲染出來", async () => {
	const page = await read(DETAIL_PAGE);

	// 這張清單就是「ADM 憑什麼核准」。少一項，畫面不會有異狀——它只是變成在無依據
	// 核准，而那正是 19 §14.2 明訂「必須顯示 govVerifyPayload」的理由。
	const required = [
		"payload.holderName", // 業者名 —— 比對證書是不是這個賣家的
		"payload.holderAddress",
		"payload.issuerName", // 驗證機構
		"payload.productSummary", // 品項 —— 比對證書涵不涵蓋這個商品
		"payload.statusRaw", // 政府原始狀態（3701 的 通過/結束/終止/暫終）
		"payload.expiresOn", // 效期
		"payload.queriedAt", // 查驗時間 —— 太舊的結果不能當依據
		"payload.matchedCount", // 命中列數 —— 3703 判斷整廠情境的關鍵
		"payload.validity",
		"payload.found",
		"payload.certNo", // 編號自我校驗（19 §8.4 防線 2）
	];
	const missing = required.filter((field) => !page.includes(field));
	assert.deepEqual(
		missing,
		[],
		`詳情頁沒有渲染這些政府查驗欄位:\n${missing.join("\n")}`,
	);

	// payload 為 null 時要有明說的空狀態，不能整塊消失——「看不到查驗結果」與
	// 「查驗結果是空的」對審核者是完全不同的兩件事。
	assert.match(page, /尚無政府查驗結果/u);

	// 🔴 3702 的 ParentTraceCode 內含未跳脫的 HTML（19 §8.4 附帶陷阱）。
	// rawExcerpt 一律走 React 的文字節點自動 escape。
	//
	// ⚠️ 比對的是 JSX prop（後面跟著 `=`）而不是裸字串：這個檔案的註解本來就會提到
	// 這個 API 名字，用裸字串比對會被自己的註解打到——那種命中什麼都沒證明。
	assert.doesNotMatch(
		page,
		/dangerouslySetInnerHTML\s*=/u,
		"政府原始回應含未跳脫 HTML，不得以 innerHTML 渲染",
	);
	// 正向：確認它真的是走 JSON.stringify 的文字節點，而不是整塊沒渲染。
	assert.match(page, /JSON\.stringify\(payload\.rawExcerpt/u);
});

test("詳情頁有「重新查驗」按鈕並打 reverify 端點", async () => {
	const page = await read(DETAIL_PAGE);

	// 它是 migration 產生的 legacy 申請**唯一**的查驗觸發方式：那些列沒有 MQ 事件，
	// 而 19 §8.3 明令禁止用排程去掃 PENDING。沒有這顆按鈕它們永遠停在待查驗。
	assert.match(page, /重新查驗/u);
	assert.match(page, /reverifyCertificationApplication\(/u);
});

test("3703 CAS 的整廠提示：判定與文案", () => {
	// 🔴 Emblem_ID 是工廠標誌編號，實測一個編號對到 173 個產品（19 §8.5）。
	assert.equal(isFactoryScopedCertification("3703"), true);
	// 反向：3701 有機是一對一的商品級證書，不該掛上整廠警語（狼來了會讓真的警語失效）。
	assert.equal(isFactoryScopedCertification("3701"), false);
	assert.equal(isFactoryScopedCertification("3702"), false);
	assert.equal(isFactoryScopedCertification(""), false);

	const hint = factoryScopeHint("173");
	assert.match(hint, /CAS/u);
	assert.match(
		hint,
		/173/u,
		"命中列數要出現在提示裡，那是審核者判斷整廠情境的依據",
	);
	// 沒有 matchedCount（背景查驗還沒回來）時仍要出警語，只是不報數字。
	assert.match(factoryScopeHint(null), /CAS/u);
	assert.doesNotMatch(factoryScopeHint(null), /0 項/u);
});

test("詳情頁真的呼叫了整廠提示，而不是只在 lib 裡放著", async () => {
	const page = await read(DETAIL_PAGE);
	assert.match(page, /factoryScopeHint\(/u);
	assert.match(page, /isFactoryScopedCertification\(/u);
});

test("分頁三欄在 wire 上是字串，parsePaged 在邊界轉成數字", () => {
	// gateway pkg/envelope 的 Page(c, records, total, page, pageSize string)
	// 三個參數都是 string ⇒ 吐出來就是 "1" 而不是 1。
	const parsed = parsePaged<{ id: string }>({
		records: [{ id: "843856384063471761" }],
		total: "137",
		page: "3",
		pageSize: "20",
	});
	assert.equal(parsed.total, 137);
	assert.equal(parsed.page, 3);
	assert.equal(parsed.pageSize, 20);
	assert.equal(parsed.records[0].id, "843856384063471761");

	// records 為 null（proto repeated 空集合）是正常狀態，不是錯誤。
	assert.deepEqual(
		parsePaged({ records: null, total: "0", page: "1", pageSize: "20" })
			.records,
		[],
	);

	// 壞值不得靜默變成看起來正常的數字：parseInt("12abc") 會回 12，Number 才會回 NaN。
	const broken = parsePaged({
		records: [],
		total: "12abc",
		page: "",
		pageSize: "x",
	} as never);
	assert.equal(broken.total, 0);
	assert.equal(broken.page, 1);
	assert.equal(broken.pageSize, 20);
});

test("送件日期區間是「本地那一天」轉成 UTC，不是直接補 Z", () => {
	// TZ 已在檔頭釘成 Asia/Taipei（UTC+8）。直接把 "2026-08-01" 接上 "T00:00:00Z"
	// 會讓區間整整平移 8 小時 —— 台灣早上 8 點前送的件全部掉出「今天」。
	assert.equal(toUtcDayStart("2026-08-01"), "2026-07-31T16:00:00Z");
	assert.equal(toUtcDayEnd("2026-08-01"), "2026-08-01T15:59:59Z");

	// 契約是 YYYY-MM-DDTHH:mm:ssZ，毫秒要砍掉。
	for (const value of [
		toUtcDayStart("2026-08-01"),
		toUtcDayEnd("2026-08-01"),
	]) {
		assert.match(value, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/u);
	}

	// 未選日期 → 空字串，呼叫端據此不帶該 query 參數（帶空字串等於送出一個空條件）。
	assert.equal(toUtcDayStart(""), "");
	assert.equal(toUtcDayEnd(""), "");
	assert.equal(toUtcDayStart("not-a-date"), "");
});

test("裁決的冪等鍵由送出內容決定，且塞得進 VARCHAR(64)", () => {
	const base = {
		applicationId: "843856384063471761",
		action: "reject",
		expectedVersion: "2",
		reasonCode: "CERT_NOT_FOUND",
		publicReason: "政府資料庫查無此編號",
		internalNote: "已致電賣家確認",
	};

	// 同一個決定 → 同一把鑰匙：重送安全（連點兩次只會成立一次）。
	assert.equal(reviewIdempotencyKey(base), reviewIdempotencyKey({ ...base }));

	// 任何一個會送出的欄位改變 → 換一把鑰匙。後端 IdempotencyService 會比對
	// request hash，同鑰匙配不同 payload 是**直接拒絕**，不是靜默覆蓋 ——
	// 固定一把鑰匙的話，管理員改完原因再送就會撞牆。
	const variants: Array<Partial<typeof base>> = [
		{ action: "approve" },
		{ expectedVersion: "3" },
		{ reasonCode: "CERT_EXPIRED" },
		{ publicReason: "換一句" },
		{ internalNote: "換一句" },
		{ applicationId: "843856384063471762" },
	];
	for (const patch of variants) {
		assert.notEqual(
			reviewIdempotencyKey({ ...base, ...patch }),
			reviewIdempotencyKey(base),
			`改了 ${Object.keys(patch)[0]} 卻沿用同一把鑰匙`,
		);
	}

	// 分隔符要能區分欄位邊界：["a b", ""] 與 ["a", "b"] 是兩個不同的決定。
	assert.notEqual(
		reviewIdempotencyKey({ ...base, publicReason: "a b", internalNote: "" }),
		reviewIdempotencyKey({ ...base, publicReason: "a", internalNote: "b" }),
	);

	// idempotency_request.key 是 VARCHAR(64)（product-service V1 migration）。
	// 超過就是寫入失敗，而失敗的時機是「管理員按下確認」的那一刻。
	for (const patch of [
		{},
		{ action: "approve" },
		{ expectedVersion: "99999" },
	]) {
		const key = reviewIdempotencyKey({ ...base, ...patch });
		assert.ok(key.length <= 64, `冪等鍵 ${key.length} 字元，超過 VARCHAR(64)`);
	}
});

test("重新查驗的冪等鍵每次都是新的（不是「這輩子只能查一次」）", async () => {
	const service = await read(SERVICE);
	const detail = await read(DETAIL_PAGE);

	// 證書可能在申請**之後**才登記上政府資料庫，同一筆申請本來就會被查很多次。
	// 若這裡改用內容決定的鍵，第二次以後全部會拿到第一次的快取結果，
	// 而畫面看起來完全正常——只是永遠不會更新。
	assert.match(detail, /newIdempotencyKey\("cert-reverify"\)/u);
	assert.match(service, /export function newIdempotencyKey/u);
	assert.doesNotMatch(
		detail,
		/reviewIdempotencyKey\([^)]*reverify/u,
		"重新查驗不得共用裁決那把由內容決定的鑰匙",
	);
});

test("id 系列一律宣告成 string（TSID > Number.MAX_SAFE_INTEGER）", async () => {
	const lib = await read(LIB);

	// 命名以 Id / ID 結尾的欄位不得宣告成 number：JSON.parse 當下就失精，
	// 之後拿著錯的 id 去核准另一筆申請。
	const numericIds = [
		...lib.matchAll(/^\t(\w*(?:[Ii]d|ID))\??:\s*number/gmu),
	].map((m) => m[1]);
	assert.deepEqual(
		numericIds,
		[],
		`這些 id 欄位宣告成了 number:\n${numericIds}`,
	);

	// 正向：抽幾個關鍵欄位確認真的是 string（避免上面那條在欄位改名後空跑）。
	for (const field of [
		"\tid: string;",
		"\tproductId: string;",
		"\tstoreId: string;",
		"\tsubmittedByAccountId: string | null;",
		"\tversion: string;",
	]) {
		assert.ok(lib.includes(field), `lib 少了欄位宣告：${field.trim()}`);
	}
});

test("裁決一律帶樂觀鎖版本，且駁回／撤銷會擋住沒填原因代碼", async () => {
	const page = await read(DETAIL_PAGE);
	const service = await read(SERVICE);

	// expectedVersion 是「這筆在我審核期間有沒有被別人改過」的唯一保護。
	assert.match(service, /expectedVersion: string;/u);
	assert.match(page, /expectedVersion: application\.version/u);

	// reject / revoke 缺 reasonCode 後端回 3028；前端先擋是為了讓人當場知道要填什麼，
	// 不是取代後端規則。
	assert.match(page, /const REASON_REQUIRED/u);
	assert.match(page, /reasonRequired && !reasonCode/u);
});

/**
 * 裁決成功提示的文案 gate。
 *
 * 這一條是 dev 實測抓到的：原本寫 `` `已${codeLabel(STATUS_LABEL, status)}` ``，
 * 而碼表標籤本身就是完成式 ⇒ 核准吐「已已通過」、撤銷吐「已已撤銷」、
 * 駁回吐「已未通過」。三個裡面**只有駁回那個不重字**，所以它最容易通過肉眼檢查，
 * 也正是它先被使用者看到。
 *
 * ⚠️ 這支測試刻意**逐字**釘死三種輸出，而不是只斷言「不含『已已』」：
 * 後者對「已未通過」是綠的 —— 那個 bug 會原樣通過。
 */
const REVIEW_OUTCOMES: Array<[string, string, string]> = [
	["核准", "APPROVED", "審核結果已更新為「已通過」，並已寫入審核紀錄。"],
	["駁回", "REJECTED", "審核結果已更新為「未通過」，並已寫入審核紀錄。"],
	["撤銷", "REVOKED", "審核結果已更新為「已撤銷」，並已寫入審核紀錄。"],
];

test("三種裁決的成功文案都是通順中文（不是「已已通過」「已未通過」）", () => {
	for (const [action, status, expected] of REVIEW_OUTCOMES) {
		assert.equal(
			reviewResultNoticeText(status),
			expected,
			`${action}（${status}）的成功文案不對`,
		);
	}
});

test("成功文案不得在碼表標籤前面自己補字（三個動作逐一檢查）", () => {
	for (const [action, status] of REVIEW_OUTCOMES) {
		const text = reviewResultNoticeText(status);
		// 疊字：核准／撤銷會現形
		assert.doesNotMatch(text, /已已/u, `${action} 出現疊字：${text}`);
		// 🔴 駁回那個不重字，只能靠「『已』不得緊貼在標籤前」來抓。
		// 標籤是 CodeMeta 生成物，前端不得假設它的字面 ⇒ 句型必須讓標籤
		// 整個包在「」裡當名詞用，前一個字元一定是開引號。
		const label = text.match(/「(.+?)」/u);
		assert.ok(label, `${action} 的狀態標籤沒有被引號包起來：${text}`);
		assert.ok(
			!text.includes(`已${label[1]}`) || label[1].startsWith("已"),
			`${action} 在標籤前自己補了「已」：${text}`,
		);
	}
});

test("狀態缺席時退成佔位符，不吐出 undefined／空引號", () => {
	for (const bad of [null, undefined, ""]) {
		const text = reviewResultNoticeText(bad);
		assert.doesNotMatch(text, /undefined|null/u);
		assert.match(text, /「—」/u);
	}
});

test("詳情頁真的呼叫 helper，沒有留下裸插值的成功文案", async () => {
	const page = await read(DETAIL_PAGE);

	// 正向：頁面走的是同一個 helper（helper 修得再對，頁面不呼叫就照不到）。
	assert.match(page, /reviewResultNoticeText\(/u);
	assert.match(page, /reverifyResultNoticeText\(/u);

	// 反向：`已${...}` 這個形狀不得再出現在這一頁。它才是 bug 本體 ——
	// 只測 helper 的話，有人把頁面改回內嵌插值時測試照樣全綠。
	assert.doesNotMatch(
		page,
		/已\$\{/u,
		"又出現了在插值前面硬接「已」的寫法；碼表標籤自己就是完成式",
	);
});

/**
 * §9.3 reverify 的 UI 契約。
 *
 * 後端的 reverify 刻意**繞過結果快取**（快取 TTL 對 VALID 是 7 天，讀快取會讓
 * 「賣家事後補登證書」這個本來就是這顆按鈕存在理由的情境永遠拿不到新答案）。
 * 代價是每按一次都真的消耗農業部的每日配額 ⇒ 按之前就要看得到這件事。
 */
test("重新查驗有事前說明，講明會重新向農業部查詢", async () => {
	const page = await read(DETAIL_PAGE);

	assert.match(REVERIFY_HINT, /農業部/u);
	assert.match(REVERIFY_HINT, /快取/u, "要講明不吃先前的查驗快取");
	assert.match(REVERIFY_HINT, /配額/u, "要講明會消耗每日配額");

	// 常駐在畫面上，不是藏在 hover（Button 元件根本不吃 title）。
	assert.match(page, /\{REVERIFY_HINT\}/u);
});

test("重新查驗的成功文案：講清楚打了外部、且沒動審核狀態", () => {
	const text = reverifyResultNoticeText("VERIFIED", "SUBMITTED");
	assert.match(text, /農業部/u);
	assert.match(text, /快取/u);
	assert.match(text, /查驗通過|VERIFIED/u, "查驗狀態要走碼表標籤而不是吞掉");
	assert.match(
		text,
		/不會改變審核狀態/u,
		"reverify 只回寫查驗欄位、不改 status（19 §9.3）",
	);
	assert.match(text, /「審核中」/u, "目前的審核狀態要顯示成標籤而非原始碼值");

	// 同樣不得在標籤前硬接「已」。
	assert.doesNotMatch(reverifyResultNoticeText("EXPIRED", "APPROVED"), /已已/u);
	assert.doesNotMatch(
		reverifyResultNoticeText("NOT_FOUND", "REJECTED"),
		/已未通過/u,
	);
});

/**
 * 冷卻護欄（backend-payuni 19 §9.3）在 ADM 端的可見性。
 *
 * 後端 `3030` 的 message 自己就帶了「幾秒前查的／還要等多久／為什麼有這個限制」，
 * 前端唯一要做的事是**別把它蓋掉**。改成寫死的文案，管理員就只會看到
 * 「重新查驗失敗」——那跟靜默無反應沒有差別，他會繼續盲目重按。
 */
test("重新查驗失敗時原樣呈現後端訊息，不得用寫死文案蓋掉（冷卻 3030 靠它才看得見原因）", async () => {
	const page = await read(DETAIL_PAGE);

	assert.match(
		page,
		/governanceErrorText\(caught, "重新查驗失敗，請稍後再試"\)/u,
		"錯誤文案必須走 governanceErrorText（它會回傳後端 message ＋ 代碼），" +
			"寫死字串會把 3030 的冷卻說明整段吃掉",
	);
	// 反向：不得出現「只印 fallback、丟掉 caught」的形狀。
	assert.doesNotMatch(
		page,
		/tone: "error",\s*text: "重新查驗/u,
		"錯誤文案被寫死了，後端的冷卻原因傳不到管理員眼前",
	);
});
