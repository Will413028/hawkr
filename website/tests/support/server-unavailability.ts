import { create } from "@bufbuild/protobuf";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	type AppealUnavailability,
	AppealUnavailabilitySchema,
	type AppealUnavailableReason,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/**
 * 單元測試用：伺服器回的「不能做某個動作」的一筆原因（`*Unavailability`）。`reason` 只給 client 分支，`message` 是伺服器組好的一句話
 * （LocalizedMessage，locale 是實際用的語系）；沒給 `message` 表示伺服器沒帶句子。句子取自設計
 * `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md` §7(a)，與 Go 的 seller 模組 `internal/text/zh-TW.json` 相同。
 */
export function unavailable<R extends number>(reason: R, message?: string) {
	return {
		reason,
		message: message === undefined ? undefined : { locale: "zh-TW", message },
	};
}

/** 同 `unavailable`，但回完整的 `AppealUnavailability` 訊息（要放進已建好的 `Measure` 時用）。 */
export function appealUnavailability(
	reason: AppealUnavailableReason,
	message?: string,
): AppealUnavailability {
	return create(AppealUnavailabilitySchema, {
		reason,
		message:
			message === undefined
				? undefined
				: create(LocalizedMessageSchema, { locale: "zh-TW", message }),
	});
}

export const APPEAL_SENTENCE = {
	REVOKED: "這筆措施已撤銷，不需要申覆",
	ALREADY_APPEALED: "這筆措施已提出過申覆，每筆措施只能申覆一次",
	APPEAL_WINDOW_ENDED: "終止合作的申覆期限已過",
	REQUEST_PENDING: "這筆措施有待處理的解除請求，審核完成後才能申覆",
} as const;

export const REMEDIATION_SENTENCE = {
	REMEDIATION_PENDING: "這筆措施的解除請求審核中，審核完成前不能再提出",
	APPEAL_PENDING: "這筆措施的申覆審核中，審核完成前不能請求解除",
	COOLDOWN: "解除請求目前在冷卻期內，2026/10/20 10:06 後可再提出",
	NOT_RISK: "只有依據風險類型的措施可以請求解除",
	NOT_ACTIVE_REVOKED: "這筆措施已撤銷",
	NOT_ACTIVE: "這筆措施目前不在生效中",
} as const;

export const OPENING_SENTENCE = {
	TERMINATED:
		"此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。",
	SUSPENDED: "帳號停權中，解除停權前不能開店",
	ACCOUNT_DELETING: "帳號正在刪除，不能申請開店",
	STORE_EXISTS: "你已經是賣家，不需要重複申請",
} as const;
