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
