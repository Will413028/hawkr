import { forwardDelegated } from "../../_forward";

/**
 * `POST /adm/api/media/upload_sessions` →
 * `POST /admin/delegated-operations/media/upload_sessions`（§6.1.13 步 ①）。
 *
 * 三步上傳：① 這裡建 session ② `PUT <uploadUrl>` **瀏覽器直送 R2，不經任何 BFF**
 * ③ `.../{sessionId}/complete` 換 `stagedToken`。
 *
 * ⚠️ **不得**退回既有的 `/api/v1/app/media/*`：www 上的瀏覽器沒有 ADM session
 * （`__Host-` cookie 是 host-only），那條路一律 401 —— 症狀看起來像權限設定錯誤，
 * 而不是「這個環境根本沒有代行上傳端點」。
 *
 * ⚠️ 逾時走 **15s 檔不是 60s**：大圖耗時全落在步 ②，而步 ② 一次 gateway 都不打；
 * 經過這裡的只有 JSON metadata，位元組數與檔案大小無關。反向也不成立 —— 不得歸
 * 10s 檔（那一檔的作用域是 session 端點，這兩支會在 media 的 `upload_session` 表
 * 寫列）。
 *
 * ⚠️ 這兩支**不掛 capability**（上傳不碰 target store 業務狀態，真正要授權的是「把圖
 * 掛上去」，由 `storefront.write`／`products.write`／`posts.write` 各自守住），但
 * **一律算活動** —— 換一批商品圖正是最容易停留超過一個閒置窗口的動作。
 */

export const dynamic = "force-dynamic";

export async function POST(request: Request): Promise<Response> {
	return forwardDelegated(request, {
		method: "POST",
		path: "/admin/delegated-operations/media/upload_sessions",
		timeout: "mutation",
	});
}
