/**
 * S02 送審閘門的判準測試。
 *
 * 🔴 這一組真正要守的是「前端判準 ≡ 後端守衛」。兩邊各自正確但不等價時：
 *   - 前端比後端**寬** ⇒ 賣家按下去拿到 400／9325，而不是就地的說明
 *   - 前端比後端**緊** ⇒ 出現「後端可以送、前端不讓送」的鎖死態，賣家無法自救
 * 所以每一支的註解都指名它對應後端的哪一條守衛。
 */

import { describe, expect, it } from "vitest";
import type {
	SellerTaxIdentity,
	SellerTaxProfile,
} from "@/lib/invoices/seller-types";
import { isIdentityComplete, taxSubmitGate } from "./taxSubmitGate";

function identity(over: Partial<SellerTaxIdentity> = {}): SellerTaxIdentity {
	return {
		entityType: "COMPANY",
		taxId: "12345678",
		legalName: "範例生鮮股份有限公司",
		responsibleName: "王小明",
		addressCity: "台北市",
		addressDistrict: "信義區",
		addressDetail: "市府路 1 號",
		claimedStatus: "REQUIRED",
		...over,
	};
}

function profile(over: Partial<SellerTaxProfile> = {}): SellerTaxProfile {
	return {
		storeId: "2685914682",
		reviewStatus: "DRAFT",
		version: "3",
		submittedAt: "",
		deadlineAt: "",
		extensionCount: "0",
		canRequestExtension: false,
		reapplyCooldownUntil: "",
		taxIdentity: identity(),
		documents: [
			{
				documentId: "1",
				kind: "TAX_REGISTRATION",
				fileName: "稅籍登記證明.pdf",
				fileKey: "secure/certification/1/20260830/a.pdf",
				fileSize: "1024",
				fileAssetId: "a1",
				uploadedAt: "2026-08-30T00:00:00Z",
			},
		],
		admNotes: [],
		pendingExtensionRequest: null,
		selfIssueAttestation: {
			termsVersion: "",
			termsKey: "",
			acceptedAt: "",
			acceptedByAccountId: "",
			effective: false,
			currentTermsVersion: "2026-10-25",
		},
		intakeLimit: {
			orderCountUsed: "0",
			orderCountLimit: "0",
			amountUsed: "0",
			amountLimit: "0",
			blocked: false,
		},
		...over,
	};
}

const READY = {
	profile: profile(),
	notStarted: false,
	editable: true,
	cooldownActive: false,
};

describe("taxSubmitGate", () => {
	it("資料齊全 ＋ 有佐證 ⇒ 勾了條款就能送", () => {
		expect(taxSubmitGate(READY)).toEqual({ visible: true, blocker: null });
	});

	// 🔴 陰性：不可編輯的狀態整個區塊不出現。渲染一顆必定回 9321 的鍵比缺一顆鍵更糟。
	it("審核中／已核定 ⇒ 區塊不渲染", () => {
		expect(taxSubmitGate({ ...READY, editable: false })).toEqual({
			visible: false,
			blocker: "not-editable",
		});
	});

	// 新賣家的常態。區塊**要出現**——安靜消失的話他看不到任何入口，
	// 也就無從得知下一步是先建立草稿。
	it("尚未建立資料 ⇒ 區塊出現並說明要先建草稿", () => {
		expect(
			taxSubmitGate({ ...READY, notStarted: true, profile: null }),
		).toEqual({ visible: true, blocker: "not-started" });
	});

	// 對應後端 400 ErrTaxIdentityIncomplete。
	it("必填欄缺 ⇒ identity-incomplete", () => {
		expect(
			taxSubmitGate({
				...READY,
				profile: profile({ taxIdentity: identity({ legalName: "" }) }),
			}).blocker,
		).toBe("identity-incomplete");
	});

	// 對應後端 T02 守衛②（400「請至少上傳一份稅籍證明文件」），**不是** 9321。
	it("沒有任何佐證 ⇒ evidence-missing", () => {
		expect(
			taxSubmitGate({ ...READY, profile: profile({ documents: [] }) }).blocker,
		).toBe("evidence-missing");
	});

	// 冷卻優先於欄位檢查：冷卻期間資料再齊全也送不出去（後端 9324）。
	it("冷卻中 ⇒ cooldown 先於欄位檢查", () => {
		expect(
			taxSubmitGate({
				...READY,
				cooldownActive: true,
				profile: profile({
					documents: [],
					taxIdentity: identity({ legalName: "" }),
				}),
			}).blocker,
		).toBe("cooldown");
	});
});

describe("isIdentityComplete（逐字對應 usecase validateTaxIdentity）", () => {
	it("自述應開統一發票時，統編是必填", () => {
		expect(
			isIdentityComplete(
				profile({
					taxIdentity: identity({ claimedStatus: "REQUIRED", taxId: "" }),
				}),
			),
		).toBe(false);
	});

	// 免用統一發票**不需要**統編（DB 的 chk_stp_exempt_no_taxid 甚至禁止它）。
	// 前端若一併要求統編，自述免用的賣家會永遠送不出去。
	it("自述免用統一發票時，沒有統編仍算齊全", () => {
		expect(
			isIdentityComplete(
				profile({
					taxIdentity: identity({ claimedStatus: "EXEMPT", taxId: "" }),
				}),
			),
		).toBe(true);
	});

	// UNKNOWN ＝ 尚未填答（後端 normalizeObligation 把未知值一律折成它），
	// 送出去必定 400 ⇒ 前端要先擋並說「請選擇」。
	it("自述值未填（UNKNOWN／空字串）一律不齊", () => {
		for (const claimedStatus of ["UNKNOWN", ""]) {
			expect(
				isIdentityComplete(
					profile({ taxIdentity: identity({ claimedStatus }) }),
				),
			).toBe(false);
		}
	});

	// 🔴 地址三欄在後端折成單欄，回程只填得出 addressDetail ⇒ 判準必須是「任一非空」。
	// 只看 addressDetail 會讓剛存完草稿的賣家被誤判成沒填地址。
	it("地址三欄任一非空即算填了", () => {
		expect(
			isIdentityComplete(
				profile({
					taxIdentity: identity({
						addressCity: "",
						addressDistrict: "",
						addressDetail: "台北市信義區市府路 1 號",
					}),
				}),
			),
		).toBe(true);
		expect(
			isIdentityComplete(
				profile({
					taxIdentity: identity({
						addressCity: "",
						addressDistrict: "",
						addressDetail: "",
					}),
				}),
			),
		).toBe(false);
	});
});
