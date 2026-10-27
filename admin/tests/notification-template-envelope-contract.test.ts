import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { readBroadcastOutcome } from "../src/lib/broadcast-result";

/**
 * 通知模板兩支 admin 端點的 envelope 契約（backend#2198 envelope Tier-2 收斂）。
 *
 * 這兩支原本把欄位掛在 envelope **頂層**（`{code, reloadedCount}`、
 * `{code, renderedTitle, ...}`），是全站 envelope 規範（10 檔 §2）的偏離點。
 * #2198 之後一律收進 `data`，且 `reloadedCount` 依 §4 字串化。
 *
 * 失敗形態值得記住：頂層欄位搬進 `data` 之後，舊的讀法拿到的是 `undefined`
 * 而**不是**錯誤 —— reload 會顯示「已重載 0 筆模板」（看起來像後端沒模板），
 * preview 會整塊空白（看起來像渲染失敗）。兩者都不會有任何 console error。
 *
 * fixture 為 2026-08-05 對本機 gateway（含 backend#2198）的**實打回應**，
 * 值未改寫（中文已從 \u 逸出還原）：
 *   POST /api/v1/admin/notification-templates/reload
 *   POST /api/v1/admin/notification-templates/1/preview
 */

const RELOAD_RESPONSE = {
	data: { reloadedCount: "76" },
	message: "success",
	code: 0,
} as const;

const PREVIEW_RESPONSE = {
	data: {
		missingFields: null,
		renderedBody: "訂單成立！感謝您的訂購，我們正快馬加鞭為您處理中。",
		renderedData: {
			orderId: "O041282666",
			refId: "O041282666",
			refType: "ORDER",
			type: "ORDER_PAID",
			url: "hawkr://orders/{order_id}",
		},
		renderedReferenceId: "O041282666",
		renderedSchemaUrl: "hawkr://orders/{order_id}",
		renderedTitle: "訂單成立通知",
	},
	message: "success",
	code: 0,
} as const;

const PREVIEW_FIELDS = [
	"renderedTitle",
	"renderedBody",
	"renderedData",
	"renderedSchemaUrl",
	"renderedReferenceId",
	"missingFields",
] as const;

test("reload：reloadedCount 在 data 之下，不在 envelope 頂層", () => {
	assert.equal(
		"reloadedCount" in RELOAD_RESPONSE,
		false,
		"頂層不該再有 reloadedCount",
	);
	assert.equal(RELOAD_RESPONSE.data.reloadedCount, "76");
});

test("reload：reloadedCount 是字串（10 檔 §4），不是數字", () => {
	// 型別搞錯不會炸，只會讓 `?? 0` 這種預設值寫法看起來仍然正常運作，
	// 直到有人拿它做算術或比較。
	assert.equal(typeof RELOAD_RESPONSE.data.reloadedCount, "string");
	assert.notEqual(
		typeof (RELOAD_RESPONSE.data.reloadedCount as unknown),
		"number",
	);
});

test("reload：envelope 只有 code/message/data 三個鍵", () => {
	assert.deepEqual(Object.keys(RELOAD_RESPONSE).sort(), [
		"code",
		"data",
		"message",
	]);
	assert.equal("msg" in RELOAD_RESPONSE, false, "msg 欄名已從 wire 移除");
});

test("preview：六個欄位全部在 data 之下，頂層一個都沒有", () => {
	for (const f of PREVIEW_FIELDS) {
		assert.ok(f in PREVIEW_RESPONSE.data, `data 應含 ${f}`);
		assert.equal(f in PREVIEW_RESPONSE, false, `envelope 頂層不該再有 ${f}`);
	}
});

test("preview：missingFields 可能是 null，讀取端必須容得下", () => {
	// 實打回應就是 null（不是 []）—— `missingFields.join()` 直接叫會爆，
	// 頁面用 `(missingFields ?? []).length` 才安全。
	assert.equal(PREVIEW_RESPONSE.data.missingFields, null);
	assert.equal((PREVIEW_RESPONSE.data.missingFields ?? []).length, 0);
});

/**
 * ── 系統公告廣播的回應（backend 2026-08-22 廢除 OKMsg 的同一批）─────────────
 *
 * 原本回的是 `{code, message}`，人數包在 message 的中文句子裡（「已推播給 N
 * 位使用者」），頁面只能整句丟進 toast —— 管理員看得到「成功」，看不到**打到
 * 幾個人**。收件人枚舉整條壞掉時（查不到任何裝置 token、targetUserType 對不上）
 * 後端一樣回成功，畫面一樣顯示成功，沒有任何地方會變紅。
 *
 * 現在 `broadcastId` / `recipientCount` 走 `data` 的具名欄位，`recipientCount`
 * 依 10 檔 §4 是**字串**。
 *
 * ⚠️ 誠實標註：下面這組 fixture 不是實打回應，是照 gateway handler
 * `AdminBroadcastAnnouncement`（`response.OK(c, gin.H{"broadcastId":…,
 * "recipientCount":…}`）與 `BroadcastAnnouncementResponse` 的 proto 欄位型別
 * （`string broadcast_id = 3` / `string recipient_count = 4`）寫出來的。
 */
const BROADCAST_RESPONSE = {
	data: { broadcastId: "7495464541774413824", recipientCount: "128" },
	message: "success",
	code: 0,
} as const;

test("broadcast：broadcastId / recipientCount 在 data 之下，不在頂層", () => {
	for (const f of ["broadcastId", "recipientCount"] as const) {
		assert.ok(f in BROADCAST_RESPONSE.data, `data 應含 ${f}`);
		assert.equal(f in BROADCAST_RESPONSE, false, `envelope 頂層不該有 ${f}`);
	}
	// message 恆為 "success"：語意全部搬進 data，不再靠剖析中文字串取值
	assert.equal(BROADCAST_RESPONSE.message, "success");
});

test("broadcast：recipientCount 是字串（10 檔 §4），不是數字", () => {
	assert.equal(typeof BROADCAST_RESPONSE.data.recipientCount, "string");
});

test("readBroadcastOutcome：有人收到 → delivered，帶得出人數與公告 ID", () => {
	assert.deepEqual(readBroadcastOutcome(BROADCAST_RESPONSE.data), {
		kind: "delivered",
		count: 128,
		broadcastId: "7495464541774413824",
	});
});

test('readBroadcastOutcome："0" 是 empty，不能跟 delivered 同類', () => {
	// 這是本次要救的那個場景：後端成功、沒有例外、但一個人都沒收到。
	const outcome = readBroadcastOutcome({
		broadcastId: "749546454177441382",
		recipientCount: "0",
	});
	assert.equal(outcome.kind, "empty");
	assert.notEqual(outcome.kind, "delivered");
});

test("readBroadcastOutcome：鍵不存在 → unreported，不得壓成 0", () => {
	// `Number(undefined) || 0` 會把「後端沒回報」變成一個假的空推播警報，
	// 讓管理員去追一個不存在的故障。兩者必須分開。
	const outcome = readBroadcastOutcome({ broadcastId: "749546454177441382" });
	assert.equal(outcome.kind, "unreported");
	assert.notEqual(outcome.kind, "empty");
});

test("readBroadcastOutcome：整包 envelope 誤傳進來時不會假裝讀到 0", () => {
	// 傳錯層（給 envelope 而不是 data）是這頁踩過的原型失敗，見本檔開頭。
	assert.equal(readBroadcastOutcome(BROADCAST_RESPONSE).kind, "unreported");
	assert.equal(readBroadcastOutcome(null).kind, "unreported");
	assert.equal(readBroadcastOutcome(undefined).kind, "unreported");
});

/**
 * ── 頁面實際打的端點 ───────────────────────────────────────────────────────
 *
 * `POST /admin/notification-templates` 與 `PUT /admin/notification-templates/{id}`
 * 兩支 route 已於 issue #1176 從 Gateway 移除（模板內容的唯一寫入路徑是 Atlas
 * migration），`routes_admin_notification.go` 留有註解。頁面上的「新增模板」與
 * 編輯抽屜的「儲存」按下去就是 404。
 *
 * 這裡斷言**方法＋路徑的整組集合**而不是逐條 grep：新增用的 POST 與列表用的
 * GET 路徑一模一樣，只比對路徑字串完全分不出來，`fcmTopic` 那種單一字面值的
 * grep 也擋不住換個寫法重新加回來。
 */
const PAGE_PATH = "src/app/admin/notification-templates/page.tsx";

function extractApiCalls(source: string): string[] {
	const calls = new Set<string>();
	const re = /\bapi\.(get|post|put|patch|delete)\b/g;
	let m: RegExpExecArray | null = re.exec(source);
	while (m !== null) {
		// 泛型參數可能跨行，所以往後找第一個字串字面值當作 URL
		const rest = source.slice(m.index + m[0].length);
		const open = rest.search(/["'`]/);
		const delim = rest[open];
		const close = rest.indexOf(delim, open + 1);
		const url = rest
			.slice(open + 1, close)
			.split("?")[0]
			.replace(/\$\{[^}]*\}/g, "{}");
		calls.add(`${m[1].toUpperCase()} ${url}`);
		m = re.exec(source);
	}
	return [...calls].sort();
}

test("頁面不再呼叫已移除的 create / update 兩支 route", () => {
	const src = readFileSync(join(process.cwd(), PAGE_PATH), "utf8");
	assert.deepEqual(extractApiCalls(src), [
		"GET /api/v1/admin/notification-templates",
		"GET /api/v1/admin/notification-templates/{}",
		"PATCH /api/v1/admin/notification-templates/{}/enabled",
		"POST /api/v1/admin/notification-templates/reload",
		"POST /api/v1/admin/notification-templates/{}/preview",
		"POST /api/v1/admin/notification-templates/{}/test-send",
		"POST /api/v1/admin/notifications/broadcast",
	]);
});

test("公告結果由 helper 判讀，頁面不自己剖析 recipientCount", () => {
	// 上面那組單元測試只守住 helper 自己。頁面若繞過它（直接讀 res.data，或
	// 退回顯示 res.message），helper 測試照樣全綠而畫面照樣看不出人數。
	const src = readFileSync(join(process.cwd(), PAGE_PATH), "utf8");
	assert.ok(
		src.includes("readBroadcastOutcome(res?.data)"),
		"公告回應必須交給 readBroadcastOutcome 判讀，且傳入的是 data 不是整包 envelope",
	);
	assert.equal(
		/showToast\(\s*res\?\.message/.test(src),
		false,
		"不得退回把 message 整句丟進 toast 的舊寫法（人數藏在中文字串裡）",
	);
});

test("頁面不再有 fcmTopic 欄位（DB column 已 DROP）", () => {
	// `notification_templates.fcm_topic` 已由 Go 側 migration 移除，proto 欄位
	// 15 也 reserved 掉；後端不會再回這個鍵，表單填了也無處可去。
	const src = readFileSync(join(process.cwd(), PAGE_PATH), "utf8");
	assert.equal(/fcmTopic|fcm_topic/.test(src), false);
});
