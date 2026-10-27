"use client";

import { type BadgeVariant, StatusBadge } from "@/components/UI";
import {
	codeLabel,
	PRODUCT_CERTIFICATION_GOV_VERIFY_STATE_LABEL,
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	PRODUCT_CERTIFICATION_VALIDITY_LABEL,
	ProductCertificationGovVerifyState,
	ProductCertificationStatus,
	ProductCertificationValidity,
} from "@/lib/codemeta";

/**
 * 認證審核佇列與詳情共用的狀態徽章。
 *
 * 顏色留在前端（後端不下發顏色），**文字一律走 `codeLabel(生成物)`**——這兩張表的鍵
 * 直接引用生成的 enum 物件，值域長出新成員時這裡會型別錯，而不是靜靜落進 fallback。
 */

/**
 * 申請狀態 → 徽章顏色。
 *
 * 四個終態刻意不共用同一個顏色：`REJECTED` / `REVOKED` 是平台判定不成立（紅），
 * `WITHDRAWN`（賣家自己停止宣稱）與 `SUPERSEDED`（換號碼退場）是中性收尾（灰）。
 * 全部塗成灰會讓「被駁回」在佇列裡看起來像「自己撤掉的」。
 */
const STATUS_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[ProductCertificationStatus.SUBMITTED]: "warning",
	[ProductCertificationStatus.APPROVED]: "success",
	[ProductCertificationStatus.REJECTED]: "danger",
	[ProductCertificationStatus.WITHDRAWN]: "gray",
	[ProductCertificationStatus.SUPERSEDED]: "gray",
	[ProductCertificationStatus.REVOKED]: "danger",
};

/**
 * 政府查驗結果 → 徽章顏色。
 *
 * 🔴 `UNAVAILABLE`（政府 API 掛了／金鑰失效）與 `NOT_FOUND`（真的查無此編號）**不同色**：
 * 前者是我方或上游的故障、不是查驗結論，塗成紅色會讓審核者把系統故障讀成「賣家造假」
 * 而直接駁回。19 §8.4 防線 1 就是為了讓這兩者不再混為一談。
 * `PENDING`（背景查驗還沒回來）同理是「還不知道」，不是「不通過」。
 */
const GOV_STATE_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[ProductCertificationGovVerifyState.PENDING]: "gray",
	[ProductCertificationGovVerifyState.VERIFIED]: "success",
	[ProductCertificationGovVerifyState.NOT_FOUND]: "danger",
	[ProductCertificationGovVerifyState.EXPIRED]: "danger",
	[ProductCertificationGovVerifyState.UNSUPPORTED]: "gray",
	[ProductCertificationGovVerifyState.UNAVAILABLE]: "warning",
};

export function CertificationStatusPill({ status }: { status: string }) {
	return (
		<StatusBadge variant={STATUS_BADGE_VARIANT[status] ?? "gray"}>
			{codeLabel(PRODUCT_CERTIFICATION_STATUS_LABEL, status)}
		</StatusBadge>
	);
}

export function GovVerifyStatePill({ state }: { state: string }) {
	return (
		<StatusBadge variant={GOV_STATE_BADGE_VARIANT[state] ?? "gray"}>
			{codeLabel(PRODUCT_CERTIFICATION_GOV_VERIFY_STATE_LABEL, state)}
		</StatusBadge>
	);
}

/**
 * 政府資料判讀出的**證書效力** → 徽章顏色。
 *
 * 這一欄才是「這張證書到底能不能核准」的主要依據；`govVerifyState` 是它塌縮後的
 * 有損版本（`EXPIRED`／`TERMINATED`／`SUSPENDED` 三值一律塌成 `EXPIRED`）。
 *
 * 🔴 `UNKNOWN` 是全表唯一的 `warning`，刻意不與任何「不通過」的值同色：它的語意是
 * **我們沒能判讀出這張證書的效力**（政府回應缺狀態欄，或狀態值不在已知值域），
 * 那是故障、不是查驗結論。塗成紅色會讓審核者把「讀不到」讀成「查證不通過」而直接
 * 駁回一張可能完全正常的證書——與 `UNAVAILABLE` 被當成 `NOT_FOUND` 那次
 * （19 §8.4 防線 1）是同一個坑。正確處置是重新查驗，詳情頁另有一則提示明說。
 *
 * 反過來，`TERMINATED`（已終止，不可逆）與 `SUSPENDED`（暫時終止，日後可能恢復）
 * 同為 `danger`：顏色只分得了「現在能不能核准」這一刀，分不了可逆性。那個差別由
 * **標籤文字**承擔——而那正是把原始碼值換成 CodeMeta 標籤所解決的問題：碼值時期
 * 審核者看到的是 `TERMINATED` / `SUSPENDED` 兩個英文字，處置卻完全不同
 * （前者要求賣家換一張新證書，後者不必）。
 */
const VALIDITY_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[ProductCertificationValidity.VALID]: "success",
	[ProductCertificationValidity.EXPIRED]: "danger",
	[ProductCertificationValidity.TERMINATED]: "danger",
	[ProductCertificationValidity.SUSPENDED]: "danger",
	[ProductCertificationValidity.NOT_FOUND]: "danger",
	[ProductCertificationValidity.UNKNOWN]: "warning",
};

export function CertificationValidityPill({ validity }: { validity: string }) {
	return (
		<StatusBadge variant={VALIDITY_BADGE_VARIANT[validity] ?? "gray"}>
			{codeLabel(PRODUCT_CERTIFICATION_VALIDITY_LABEL, validity)}
		</StatusBadge>
	);
}
