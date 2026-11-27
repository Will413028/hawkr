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

	it("審核中／已核定 ⇒ 區塊不渲染", () => {
		expect(taxSubmitGate({ ...READY, editable: false })).toEqual({
			visible: false,
			blocker: "not-editable",
		});
	});

	it("尚未建立資料 ⇒ 區塊出現並說明要先建草稿", () => {
		expect(
			taxSubmitGate({ ...READY, notStarted: true, profile: null }),
		).toEqual({ visible: true, blocker: "not-started" });
	});

	it("必填欄缺 ⇒ identity-incomplete", () => {
		expect(
			taxSubmitGate({
				...READY,
				profile: profile({ taxIdentity: identity({ legalName: "" }) }),
			}).blocker,
		).toBe("identity-incomplete");
	});

	it("沒有任何佐證 ⇒ evidence-missing", () => {
		expect(
			taxSubmitGate({ ...READY, profile: profile({ documents: [] }) }).blocker,
		).toBe("evidence-missing");
	});

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

	it("自述免用統一發票時，沒有統編仍算齊全", () => {
		expect(
			isIdentityComplete(
				profile({
					taxIdentity: identity({ claimedStatus: "EXEMPT", taxId: "" }),
				}),
			),
		).toBe(true);
	});

	it("自述值未填（UNKNOWN／空字串）一律不齊", () => {
		for (const claimedStatus of ["UNKNOWN", ""]) {
			expect(
				isIdentityComplete(
					profile({ taxIdentity: identity({ claimedStatus }) }),
				),
			).toBe(false);
		}
	});

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
