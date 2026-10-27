import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

/**
 * A4 人工解鎖的取消證據檔上傳（backend #3108）走的是**機敏檔**路徑。
 *
 * 🔴 這支測試存在的理由是一個實測到的 100% 失敗：
 * media-service 的 `UploadSessionService` 逐字寫著「僅公開上傳提供簽名 URL」
 * ⇒ `isSecure: true` 建出來的會話**不回 `uploadUrl`**，機敏檔要 PUT 到
 * `/app/media/secure_upload/{sessionId}`，而且那一支**直接回 stagedToken**（沒有 complete）。
 *
 * 而 `uploadImageToMedia` 原本無條件要求 `uploadUrl` ⇒ 唯一 `isSecure: true` 的
 * 呼叫端（發票取消證據）必定拋「建立上傳會話失敗：缺少 sessionId 或 uploadUrl」。
 * dev 實測（2026-09-05）：改走 secure_upload 之後，端點 54 換發出 objectKey
 * （129 字元、非 JWT），覆核端下載回來的位元組與原檔完全相同。
 *
 * 範圍誠實標註：本 repo 的 tests/ 全是 node:test 純函式，沒有 DOM／fetch harness
 * ⇒ 這裡驗的是**原始碼契約**（哪條路徑、用哪個 API），不是執行期行為。
 * 執行期那一半由上面那段 dev 實測負責。
 */

const SRC = readFileSync(
	join(import.meta.dirname, "..", "src", "services", "media-upload.ts"),
	"utf8",
);

test("機敏檔走 secure_upload，不依賴 uploadUrl", () => {
	// 對照組：先證明我讀到的是對的檔案。
	assert.match(SRC, /export async function uploadImageToMedia/);

	assert.match(
		SRC,
		/media\/secure_upload\/\$\{sessionId\}/,
		"isSecure 分支必須 PUT 到 /app/media/secure_upload/{sessionId} —— " +
			"公開上傳那條的 presigned uploadUrl 對機敏會話恆為空",
	);
	// 缺 uploadUrl 的檢查不得在 isSecure 分支**之前**擋下來。
	const secureIdx = SRC.indexOf("options.isSecure");
	const uploadUrlGuard = SRC.indexOf("缺少 uploadUrl");
	assert.ok(secureIdx > 0, "找不到 isSecure 分支");
	assert.ok(uploadUrlGuard > 0, "找不到 uploadUrl 守衛");
	assert.ok(
		secureIdx < uploadUrlGuard,
		"isSecure 分支必須排在 uploadUrl 守衛之前，否則機敏檔會先被那個守衛擋掉",
	);
});

test("secure 分支不得用 api.put（它會 JSON.stringify 掉二進位）", () => {
	assert.doesNotMatch(
		SRC,
		/api\.put</,
		"api.put 對 body 做 JSON.stringify，ArrayBuffer 會變成 {} —— 要用 request()",
	);
	assert.match(SRC, /body: await file\.arrayBuffer\(\)/);
});
