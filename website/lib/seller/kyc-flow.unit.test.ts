import { create } from "@bufbuild/protobuf";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	KYCFlowSchema,
	KYCFlowStatus,
	PayUniState,
	SellerPayUniSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	kycFlowActionLabel,
	kycFlowBanner,
	kycFlowGate,
	kycFlowInReview,
	kycFlowStatusLabel,
	shownFlowStatus,
} from "@/lib/seller/kyc-flow";

const F = KYCFlowStatus;
const ut = uiTextFor("zh-TW");

const flowWith = (status: KYCFlowStatus) => create(KYCFlowSchema, { status });

/** 目錄 omit 的值：PayUni 階段的狀態由 `KYCFlow.payuni` 呈現，橫幅與閘門沒有文字。 */
const PAYUNI_STAGE = [
	F.KYC_FLOW_STATUS_BINDING,
	F.KYC_FLOW_STATUS_BIND_FAILED,
	F.KYC_FLOW_STATUS_FORWARDED,
	F.KYC_FLOW_STATUS_UNDER_REVIEW,
	F.KYC_FLOW_STATUS_PENDING_INFO,
	F.KYC_FLOW_STATUS_PAYUNI_REJECTED,
];

const KNOWN = Object.values(F).filter(
	(v): v is KYCFlowStatus =>
		typeof v === "number" && v !== F.KYC_FLOW_STATUS_UNSPECIFIED,
);

describe("shownFlowStatus", () => {
	it("已認證時一律是已通過，不管流程怎麼說", () => {
		expect(shownFlowStatus(true, undefined)).toBe(F.KYC_FLOW_STATUS_APPROVED);
		expect(shownFlowStatus(true, F.KYC_FLOW_STATUS_UNDER_REVIEW)).toBe(
			F.KYC_FLOW_STATUS_APPROVED,
		);
	});

	it("未認證時沿用流程狀態", () => {
		expect(shownFlowStatus(false, F.KYC_FLOW_STATUS_SUBMITTED)).toBe(
			F.KYC_FLOW_STATUS_SUBMITTED,
		);
		expect(shownFlowStatus(false, undefined)).toBeUndefined();
	});
});

describe("流程狀態的文字只有一張表", () => {
	it("每個已知狀態都有標籤；橫幅與閘門有文字，沒有文字的值正好是 PayUni 階段的六個", () => {
		const withoutBanner: KYCFlowStatus[] = [];
		const withoutGate: KYCFlowStatus[] = [];
		for (const status of KNOWN) {
			expect(kycFlowStatusLabel(ut, status), String(status)).not.toBe("未知");
			const banner = kycFlowBanner(ut, status);
			const gate = kycFlowGate(ut, flowWith(status)).copy;
			if (banner === null) withoutBanner.push(status);
			else {
				expect(banner.title, String(status)).not.toBe("未知");
				expect(banner.description, String(status)).not.toBe("未知");
			}
			// 目錄沒有閘門文字的狀態，閘門是「未知」（標題與內容都是）。
			if (gate.heading === "未知" && gate.message("開店") === "未知")
				withoutGate.push(status);
			else {
				expect(gate.heading, String(status)).not.toBe("未知");
				expect(gate.message("開店"), String(status)).not.toBe("未知");
			}
		}
		expect(withoutBanner.sort()).toEqual([...PAYUNI_STAGE].sort());
		expect(withoutGate.sort()).toEqual([...PAYUNI_STAGE].sort());
	});

	it("UNSPECIFIED 與沒有狀態不是任何已知狀態：標籤是共用文案目錄的「未知」，其餘同「尚未開始」", () => {
		for (const status of [F.KYC_FLOW_STATUS_UNSPECIFIED, undefined]) {
			expect(kycFlowStatusLabel(ut, status)).toBe("未知");
			expect(kycFlowBanner(ut, status)).toEqual(
				kycFlowBanner(ut, F.KYC_FLOW_STATUS_NOT_STARTED),
			);
			const gate = kycFlowGate(
				ut,
				status === undefined ? undefined : flowWith(status),
			);
			const base = kycFlowGate(ut, flowWith(F.KYC_FLOW_STATUS_NOT_STARTED));
			expect(gate.action).toBe(base.action);
			expect(gate.copy?.heading).toBe(base.copy?.heading);
			expect(gate.copy?.message("開店")).toBe(base.copy?.message("開店"));
			expect(kycFlowActionLabel(ut, status)).toBe("開始認證");
		}
	});

	it("平台審核中：不承諾天數，不宣稱 PayUni 已在審", () => {
		const banner = kycFlowBanner(ut, F.KYC_FLOW_STATUS_SUBMITTED);
		expect(banner?.title).toBe("平台審核中");
		expect(banner?.description).not.toMatch(/工作天/);
		expect(banner?.description).not.toMatch(/PayUni (審核中|正在審)/);
		expect(
			kycFlowGate(ut, flowWith(F.KYC_FLOW_STATUS_SUBMITTED)).copy?.message(
				"開店",
			),
		).not.toMatch(/工作天/);
	});

	it("PayUni 階段沒有流程橫幅；沒有 payuni 時閘門是「未知」（與 seller-app 一致）；有 PayUni 狀態時閘門是 PayUni 的標題與伺服器的說明，不帶 subject", () => {
		const payuni = create(SellerPayUniSchema, {
			state: PayUniState.FIXABLE,
			explanation: create(LocalizedMessageSchema, {
				locale: "zh-TW",
				message: "伺服器的獨特說明",
			}),
		});
		for (const status of PAYUNI_STAGE) {
			expect(kycFlowBanner(ut, status)).toBeNull();
			const bare = kycFlowGate(ut, flowWith(status)).copy;
			expect(bare.heading).toBe("未知");
			expect(bare.message("開店")).toBe("未知");
			const gate = kycFlowGate(ut, create(KYCFlowSchema, { status, payuni }));
			expect(gate.copy?.heading).toBe("PayUni 需要補件");
			expect(gate.copy?.message("開店")).toBe("伺服器的獨特說明");
		}
	});

	it("PayUni 狀態是 APPROVED 時閘門照目錄；不認得的狀態是「未知」加說明", () => {
		const approved = create(KYCFlowSchema, {
			status: F.KYC_FLOW_STATUS_NOT_STARTED,
			payuni: create(SellerPayUniSchema, { state: PayUniState.APPROVED }),
		});
		expect(kycFlowGate(ut, approved).copy?.heading).toBe("尚未通過商家認證");
		const unknown = create(KYCFlowSchema, {
			status: F.KYC_FLOW_STATUS_BINDING,
			payuni: create(SellerPayUniSchema, {
				state: PayUniState.UNSPECIFIED,
				explanation: create(LocalizedMessageSchema, { message: "說明" }),
			}),
		});
		const gate = kycFlowGate(ut, unknown).copy;
		expect(gate?.heading).toBe("未知");
		expect(gate?.message("開店")).toBe("說明");
	});

	it("已通過沒有下一步；補件與被拒是「前往補件」", () => {
		expect(kycFlowActionLabel(ut, F.KYC_FLOW_STATUS_APPROVED)).toBeUndefined();
		for (const status of [
			F.KYC_FLOW_STATUS_PENDING_INFO,
			F.KYC_FLOW_STATUS_REJECTED,
			F.KYC_FLOW_STATUS_PAYUNI_REJECTED,
		]) {
			expect(kycFlowActionLabel(ut, status)).toBe("前往補件");
			expect(kycFlowGate(ut, flowWith(status)).action).toBe("前往補件");
		}
	});

	it("被拒的閘門文字請賣家「修改」資料（不說補正）；填寫中的標籤是「填寫中」", () => {
		expect(
			kycFlowGate(ut, flowWith(F.KYC_FLOW_STATUS_REJECTED)).copy?.message(
				"開店",
			),
		).toBe("認證未通過，請修改資料後重新送審。");
		expect(kycFlowStatusLabel(ut, F.KYC_FLOW_STATUS_DRAFT)).toBe("填寫中");
	});

	it("橫幅的語氣與按鈕文字是版面：各階段各一組", () => {
		expect(kycFlowBanner(ut, F.KYC_FLOW_STATUS_NOT_STARTED)).toEqual({
			tone: "neutral",
			title: "尚未開始商家認證",
			description: "完成認證後才能正式收款、提領與配送設定。",
			ctaLabel: "開始認證",
		});
		expect(kycFlowBanner(ut, F.KYC_FLOW_STATUS_REJECTED)).toEqual({
			tone: "danger",
			title: "認證未通過",
			description: "請查看退件原因並編輯後重新送出。",
			ctaLabel: "查看原因",
		});
		expect(kycFlowBanner(ut, F.KYC_FLOW_STATUS_APPROVED)).toMatchObject({
			tone: "success",
			ctaLabel: "前往錢包",
		});
	});

	it("閘門文字不把認證當成上架商品或建立優惠券的前提", () => {
		for (const status of KNOWN) {
			const copy = kycFlowGate(ut, flowWith(status)).copy;
			expect(copy?.message("開店") ?? "").not.toMatch(/上架商品|新增商品|優惠/);
		}
	});
});

describe("kycFlowInReview", () => {
	it("只有已送出、還沒有結果的狀態算審核中", () => {
		const inReview = KNOWN.filter(kycFlowInReview);
		expect(inReview.sort()).toEqual(
			[
				F.KYC_FLOW_STATUS_SUBMITTED,
				F.KYC_FLOW_STATUS_BINDING,
				F.KYC_FLOW_STATUS_BIND_FAILED,
				F.KYC_FLOW_STATUS_FORWARDED,
				F.KYC_FLOW_STATUS_UNDER_REVIEW,
			].sort(),
		);
		expect(kycFlowInReview(undefined)).toBe(false);
	});
});
