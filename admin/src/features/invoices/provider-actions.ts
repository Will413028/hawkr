/**
 * A04（AMEGO 串接監控）四個寫入動作的共用常數與判定。
 *
 * 端點 44（重新測試）／45（停用）／66（恢復）／67（撤銷）。三支 dialog 與詳情頁
 * 共用這一份，避免同一組業務碼在四個地方各寫一次。
 *
 * ## 為什麼這裡只放「判定」不放「送出」
 *
 * 送出要帶 `Idempotency-Key` header（gateway 的 `RequireIdempotencyKey` 掛在整個
 * `/admin/invoices` 群組上），而 `api.post` 不收自訂 header ⇒ 呼叫點直接用
 * `services/api.ts` 的 `request()`。那幾行留在頁面上，是為了讓「哪一顆鍵打哪一支端點」
 * 在頁面本身讀得到。
 */

import { InvoiceProviderAccountStatus } from "@/lib/codemeta";

/** 端點 42／43／44／45／46／66／67 的共同前綴。 */
export const PROVIDER_ACCOUNTS_ENDPOINT =
	"/api/v1/admin/invoices/provider_accounts";

/**
 * 端點 44 與 66 的前端逾時（毫秒）。
 *
 * 這兩支在請求路徑上同步等 AMEGO，`api.ts` 預設的 10 秒一定不夠（58A 陷阱 T10）。
 * 130 秒 ＝ gateway 那一側 2 分鐘 deadline 再加一點緩衝：前端要**比後端晚放棄**，
 * 反過來的話畫面會顯示逾時而後端仍在跑、結果照樣落庫。
 */
export const PROVIDER_SYNC_TIMEOUT_MS = 130000;

/**
 * 這一次操作的冪等鍵。
 *
 * 格式要通過 gateway 的 `^[A-Za-z0-9._:-]{8,128}$`，而且會落進
 * `invoice_admin_audit_logs.request_id`（varchar(64)）。
 *
 * 🔴 **送出失敗時不換鍵**（58A 陷阱 T28）：操作可能已經生效而回應沒送到，換一把新的
 * 會被後端當成第二次操作。反過來，**收到回應之後**（不論 code 是 0 還是業務碼）下一次
 * 按鍵是新的操作，必須換一把——`uq_invadmaudit_client_request` 是
 * `(actor_account_id, action, request_id)` 的唯一索引，而插入走 `ON CONFLICT DO NOTHING`
 * ⇒ 沿用同一把鍵的第二次操作**不會報錯，只會靜靜地不寫稽核列**。
 */
export function createProviderIdempotencyKey(): string {
	const uuid =
		typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
			? crypto.randomUUID()
			: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
	return `inv-provider-${uuid}`;
}

/**
 * 「這個動作在目前狀態下不成立」那一類的業務碼。
 *
 * 共同處置：**關 dialog ＋ refetch ＋ 在頁面上說一句自己的話**。留在 dialog 裡沒有意義
 * ——使用者能做的只有看新的狀態再決定按哪一顆。
 *
 * ⚠️ **一律不透傳後端 message**，三個碼的定版文案在這一頁上都是錯的：
 *   - 9336 `INVOICE_COMPLAINT_ACTION_NOT_ALLOWED`「這件**申訴**目前的狀態…」（申訴語境）
 *   - 9333 `INVOICE_PROVIDER_STATE_INVALID`「憑證尚未驗證通過，無法**啟用或輪替**」
 *     （撤銷與停用既不是啟用也不是輪替）
 *   - 9322 `SELLER_TAX_PROFILE_VERSION_CONFLICT`「**稅務資格資料**已被更新」（不是稅務資料）
 *
 * ⚠️ 58A 只列了 9322／9336，**實作回的是 9333**：`usecase.adminLifecycle` 的狀態 CAS
 * rows=0 一律回 9333（見 `provider_account.go` 的 `RevokeForAdmin` 防護②），
 * `ReactivateForAdmin` 的 `RequireStatus` 不符也是 9333。兩個都收才不會漏。
 */
export const PROVIDER_STATE_CONFLICT_CODES = new Set([9322, 9333, 9336]);

/** 帳戶不存在（或越權）。詳情頁本身會切成「尚未建立 AMEGO 串接」。 */
export const PROVIDER_NOT_FOUND_CODE = 9330;

/** 憑證驗證失敗（HTTP 恆 200，preserveData）。賣家自己修得掉。 */
export const PROVIDER_CREDENTIAL_INVALID_CODE = 9331;

/** AMEGO 暫時不可用（HTTP 409）。**不是這個賣家的設定問題**。 */
export const PROVIDER_UNAVAILABLE_CODE = 9332;

/** 版本衝突與狀態不合法的頁面級文案（逐字，58A §A04「業務碼」）。 */
export function providerConflictText(code: number): string {
	if (code === 9322) {
		return "這條連線剛剛有新的變更，請重新確認後再操作一次。";
	}
	return "這條連線目前的狀態無法進行這個動作，畫面已更新。";
}

/** 檢查／重新驗證的結果落點。三種，前端只渲染不判斷（58A §A04）。 */
export type ProviderVerifyTone = "success" | "warning" | "danger";

export interface ProviderVerifyResult {
	tone: ProviderVerifyTone;
	text: string;
}

/**
 * 由「回來的 `status`」推出結果橫幅。
 *
 * 🔴 **文案必須讓人分得出「憑證錯」與「AMEGO 連不上」**（58A §A04 逐字），
 * 兩者都寫成「連線失敗」的話，ADM 會在 AMEGO 出事時跑去跟賣家要 AppKey。
 *
 * ⚠️ 為什麼看 `status` 而不是只看業務碼：端點 44 的 gateway handler
 * （`invoice_admin.go` 的 `HealthCheckProviderAccountForAdmin`）**沒有檢查
 * `resp.GetCode()`**，一律走 `response.OK` ⇒ 9331 是 preserveData 碼、interceptor
 * 不轉 error ⇒ 前端收到的 envelope `code` 是 0，唯一的線索就是 data 裡的 `status`。
 * （這是後端的缺口，已在交付回報登記；前端這一側兩條線索都吃，端點修好之後這裡
 * 也不用改。）
 */
export function providerVerifyResultOf(status: string): ProviderVerifyResult {
	switch (status) {
		case InvoiceProviderAccountStatus.INVALID_CREDENTIAL:
			return {
				tone: "danger",
				text: "憑證驗證沒有通過：這組統一編號與 AppKey 在 AMEGO 不被接受。請賣家回「AMEGO 串接設定」重新輸入 AppKey。",
			};
		case InvoiceProviderAccountStatus.PROVIDER_REJECTED:
			return {
				tone: "danger",
				text: "AMEGO 拒絕了這組憑證。請賣家回「AMEGO 串接設定」重新輸入 AppKey，或先向 AMEGO 確認這個商戶是否已開通。",
			};
		case InvoiceProviderAccountStatus.PROVIDER_UNAVAILABLE:
			return {
				tone: "warning",
				text: "AMEGO 目前無法連線，這不是該賣家的設定問題。請稍後再測一次，不要請賣家更換 AppKey。",
			};
		case InvoiceProviderAccountStatus.CREDENTIAL_VERIFYING:
			return {
				tone: "warning",
				// ⚠️ 這一句會被端點 66 前綴「已送出重新驗證。」再接一次 ⇒ 自己不要再說一遍。
				text: "AMEGO 尚未回覆驗證結果，稍後用「手動更新」看最新狀態。",
			};
		case InvoiceProviderAccountStatus.ACTIVE:
			return { tone: "success", text: "連線正常，憑證通過驗證。" };
		default:
			// 後端新增狀態值時不要白畫面，也不要謊稱成功。
			return {
				tone: "warning",
				text: "已完成檢查，目前的連線狀態顯示在下方。",
			};
	}
}

// ---------------------------------------------------------------------------
// 五顆 CTA 的渲染條件（58A §A04「CTA 五顆，依 status 分成互斥的兩組」）
// ---------------------------------------------------------------------------
//
// 🔴 條件不成立時**不渲染**（不是 disabled）——58A §A04「四態．disabled」逐字。
// 一顆按不下去的按鈕會讓管理員一直找「為什麼不能按」，而正確的訊息是「這個狀態
// 根本沒有這個動作」。
//
// ⚠️ 這四組住在這裡而不是頁面裡，是為了讓 `tests/invoice-adm-contract.test.ts`
// 逐一比對得到——寫在頁面裡的話，把「恢復」放寬成 `ACTIVE` 也能顯示這種事，
// 沒有任何東西會轉紅。

/** 端點 44（重新測試）不渲染的狀態：除這三個外恆在。 */
export const HEALTH_CHECK_HIDDEN_STATUSES = new Set<string>([
	InvoiceProviderAccountStatus.NOT_STARTED,
	InvoiceProviderAccountStatus.NOT_CONFIGURED,
	InvoiceProviderAccountStatus.REVOKED,
]);

/** 端點 45（停用）的來源狀態。 */
export const DEACTIVATABLE_STATUSES = new Set<string>([
	InvoiceProviderAccountStatus.ACTIVE,
	InvoiceProviderAccountStatus.DEGRADED,
]);

/**
 * 端點 66（恢復）的來源狀態：**只有 `SUSPENDED`**。
 *
 * 這一組放寬會直接製造一顆必死的按鈕：`usecase.ReactivateForAdmin` 的
 * `RequireStatus` 寫死 `SUSPENDED`，對別的狀態一律回 9333。
 */
export const REACTIVATABLE_STATUSES = new Set<string>([
	InvoiceProviderAccountStatus.SUSPENDED,
]);

/** 端點 67（撤銷）的來源狀態（P16 的 allowed-from，寫在 `RevokeProviderAccount` 的 WHERE）。 */
export const REVOCABLE_STATUSES = new Set<string>([
	InvoiceProviderAccountStatus.ACTIVE,
	InvoiceProviderAccountStatus.DEGRADED,
	InvoiceProviderAccountStatus.SUSPENDED,
]);
