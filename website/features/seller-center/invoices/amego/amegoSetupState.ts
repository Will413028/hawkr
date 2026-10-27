/**
 * S03｜`InvoiceProviderAccountStatus`（11 個 DB 值 ＋ 2 個投影值）→ AMEGO 串接精靈的
 * 步驟、狀態 pill 與主按鈕可用性（58A §S03 的值域對照表）。
 *
 * 🔴 **步驟 4 與 5 的分界是 `automationCutoverAt` 有沒有值，不是 `status`。**
 * 後端沒有「測試成功待啟用」這個狀態——`CREDENTIAL_VERIFYING` 三項驗證全過會
 * **直接進 `ACTIVE`**。照 `status` 分界會讓賣家在設定切點之前就看到「已啟用」，
 * 然後以為不用做了，而那正是訂單開不出發票的成因。
 *
 * 🔴 **必須有 default 分支**（§0.4 ⑤／C11）：後端先上線一個前端還不知道的新值時，
 * 落 `狀態更新中` 而不是白畫面。`unknown` 旗標存在的唯一理由是讓測試驗得出
 * 「已知值真的有進 map」——沒有它，一個把所有值都吃掉的 default 會讓測試全綠。
 */

import { InvoiceProviderAccountStatus } from "@/lib/codemeta/enums";

/** 照 `ShippingClient` 的 `shipping-pill--*` 四態。 */
export type AmegoPillTone = "idle" | "pending" | "ok" | "alert";

export interface AmegoSetupState {
	/** 1–5，對應錨點導覽與進度 meter。 */
	step: number;
	pillTone: AmegoPillTone;
	pillLabel: string;
	/** ⑧ 主按鈕「確認啟用」可否按。 */
	canActivate: boolean;
	/** ④ AppKey 欄位是否回到可輸入（憑證被拒的兩種狀態）。 */
	credentialEditable: boolean;
	/** ⑤「測試連線」可否按。 */
	canVerify: boolean;
	/** 已設定切點 ⇒ 主按鈕降級成次要的「更新啟用時間」。 */
	activated: boolean;
	/** 平台端停用／作廢，賣家自己做不了任何事。 */
	terminal: boolean;
	/** 落到 default 分支。 */
	unknown: boolean;
}

const S = InvoiceProviderAccountStatus;

/**
 * @param status 端點 23 的 `status`（可能是值域外的字串）
 * @param hasCutover `automationCutoverAt` 是否有值
 */
export function amegoSetupState(
	status: string,
	hasCutover: boolean,
): AmegoSetupState {
	const base: AmegoSetupState = {
		step: 2,
		pillTone: "idle",
		pillLabel: "尚未串接",
		canActivate: false,
		credentialEditable: true,
		canVerify: false,
		activated: false,
		terminal: false,
		unknown: false,
	};

	switch (status) {
		case S.NOT_STARTED:
			return base;
		case S.NOT_CONFIGURED:
			return { ...base, pillLabel: "尚未設定憑證" };

		case S.CREDENTIAL_SUBMITTED:
			return {
				...base,
				step: 3,
				pillTone: "pending",
				pillLabel: "待測試",
				credentialEditable: false,
				canVerify: true,
			};
		// 🔴 文案是「待測試」不是「測試中」（2026-09-05 第 6 輪稽核）。
		//
		// 端點 24（送出 AppKey）**不會觸發驗證**：它把 P01＋P02 合併，直接落
		// CREDENTIAL_VERIFYING 就返回，真正打 AMEGO 的是端點 25（賣家按「測試連線」）。
		// 所以這個狀態在絕大多數時間的意思是「憑證已收下、等你按那顆按鈕」，
		// 而「測試中」讀起來是「系統正在跑，你等就好」——賣家因此不會去按。
		// dev 實測：4 個帳號停在這個狀態 1~3 天，歷程上有 SELLER_PROVIDER_CREDENTIAL_SUBMITTED
		// 卻沒有任何一筆 PROVIDER_VERIFICATION_STARTED，其中一個連按三次送出。
		//
		// ⚠️ 驗證**真的**在跑的那幾秒也落在同一個 status（端點 25 的三段式：開始→打
		// AMEGO→落地，全在一個請求內）。那一段不靠 pill 表達——按鈕自己有
		// loadingLabel「測試連線中…」，而它只有在使用者剛按下時才出現。
		// 兩個讀法擇一時取**持續得久的那個**：停在這裡好幾天的是前者。
		case S.CREDENTIAL_VERIFYING:
			return {
				...base,
				step: 3,
				pillTone: "pending",
				pillLabel: "待測試",
				credentialEditable: false,
				canVerify: true,
			};

		case S.INVALID_CREDENTIAL:
			return {
				...base,
				pillTone: "alert",
				pillLabel: "憑證錯誤",
				canVerify: true,
			};
		case S.PROVIDER_REJECTED:
			return {
				...base,
				pillTone: "alert",
				pillLabel: "服務商拒絕",
				canVerify: true,
			};
		case S.PROVIDER_UNAVAILABLE:
			return {
				...base,
				step: 3,
				pillTone: "alert",
				pillLabel: "服務商無法連線",
				credentialEditable: false,
				canVerify: true,
			};

		// 🔴 ACTIVE／DEGRADED 的分界看切點，不看 status——見檔頭。
		case S.ACTIVE:
			return activeLike(base, hasCutover, "測試成功", "已啟用", "ok");
		case S.DEGRADED:
			return activeLike(base, hasCutover, "降級中", "降級中", "pending");

		case S.ROTATION_PENDING:
			return {
				...base,
				step: 5,
				pillTone: "pending",
				pillLabel: "憑證更新中",
				credentialEditable: false,
				activated: hasCutover,
			};

		case S.SUSPENDED:
			return terminalState(base, "已停用");
		case S.REVOKED:
			return terminalState(base, "已作廢");

		case S.CONFIG_ERROR:
			return {
				...base,
				pillTone: "alert",
				pillLabel: "配置錯誤",
				credentialEditable: false,
			};

		default:
			// 值域外：維持在最保守的一格，什麼都不讓按，但**不擋畫面**。
			return {
				...base,
				pillLabel: "狀態更新中",
				credentialEditable: false,
				unknown: true,
			};
	}
}

function activeLike(
	base: AmegoSetupState,
	hasCutover: boolean,
	labelBeforeCutover: string,
	labelAfterCutover: string,
	tone: AmegoPillTone,
): AmegoSetupState {
	return {
		...base,
		step: hasCutover ? 5 : 4,
		pillTone: tone,
		pillLabel: hasCutover ? labelAfterCutover : labelBeforeCutover,
		canActivate: true,
		credentialEditable: false,
		canVerify: true,
		activated: hasCutover,
	};
}

function terminalState(base: AmegoSetupState, label: string): AmegoSetupState {
	return {
		...base,
		step: 5,
		pillTone: "alert",
		pillLabel: label,
		credentialEditable: false,
		terminal: true,
	};
}

/** 進度 meter 的百分比。步驟 5 ＝ 100%。 */
export function amegoProgressPercent(step: number): number {
	const clamped = Math.min(Math.max(step, 1), 5);
	return Math.round(((clamped - 1) / 4) * 100);
}
