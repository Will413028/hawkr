import { describe, expect, it } from "vitest";
import {
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	ProductCertificationStatus,
} from "@/lib/codemeta/enums";
import { certStatusView, resolveCertTypeView } from "../certification-status";
import type { ProductCertificationApplication } from "../seller-repo";

/**
 * 認證狀態判讀（19 號設計書 §5 / §9.1）。
 *
 * 這一支守的是「三個輸入面對同一筆認證不能有三個答案」，以及本次要修掉的兩個具體
 * 謊言：① 草稿宣稱被標成已通過 ② 沒有登入態的面憑空生出審核狀態。
 */

function app(
	over: Partial<ProductCertificationApplication> = {},
): ProductCertificationApplication {
	return {
		id: "1",
		productId: "p1",
		certificationType: "3701",
		certificateNo: "1-009-300022",
		status: ProductCertificationStatus.SUBMITTED,
		govVerifyState: "VERIFIED",
		govVerifiedAt: null,
		submittedAt: "2026-08-26T04:12:05Z",
		reviewedAt: null,
		reasonCode: null,
		publicReason: null,
		version: "1",
		...over,
	};
}

describe("certStatusView", () => {
	it("徽章文字一律取自 CodeMeta 生成表，不是本檔自己寫的對照", () => {
		for (const status of Object.values(ProductCertificationStatus)) {
			expect(certStatusView(status)?.label).toBe(
				PRODUCT_CERTIFICATION_STATUS_LABEL[status],
			);
		}
	});

	it("駁回優先顯示 ADM 填的 publicReason，其次才是原因碼", () => {
		expect(
			certStatusView(ProductCertificationStatus.REJECTED, {
				publicReason: "政府資料庫查無此編號，請確認後重新提交",
				reasonCode: "CERT_NOT_FOUND",
			})?.description,
		).toBe("政府資料庫查無此編號，請確認後重新提交");

		expect(
			certStatusView(ProductCertificationStatus.REJECTED, {
				reasonCode: "CERT_NOT_FOUND",
			})?.description,
		).toBe("查無此證書");
	});

	it("認不得的碼值回 null，不畫一個沒有文字的空徽章", () => {
		expect(certStatusView("NOT_A_STATUS")).toBeNull();
		expect(certStatusView(undefined)).toBeNull();
	});
});

describe("resolveCertTypeView — draft（商品建立頁 / CSV 匯入逐筆編輯）", () => {
	it("草稿宣稱一律沒有徽章：那一筆還沒送審，任何狀態都是編造的", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "draft",
			entry: { certNo: "1-009-300022" },
		});
		expect(view.checked).toBe(true);
		expect(view.certNo).toBe("1-009-300022");
		expect(view.badge).toBeNull();
	});

	it("即使硬塞一份已核准的申請進來也不顯示：draft 面拿不到這種資料", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "draft",
			entry: { certNo: "1-009-300022" },
			applications: [app({ status: ProductCertificationStatus.APPROVED })],
		});
		expect(view.badge).toBeNull();
		expect(view.withdrawable).toEqual([]);
	});
});

describe("resolveCertTypeView — readonly（代營運工作區，沒有賣家登入態）", () => {
	it("只從 approved-only 投影推導，掛著就是已通過", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "readonly",
			entry: { certNo: "1-009-300022" },
		});
		expect(view.badge?.status).toBe(ProductCertificationStatus.APPROVED);
		expect(view.badge?.label).toBe("已通過");
	});

	it("鑑別力：傳進申請清單也一律忽略，且不提供任何可撤回的對象", () => {
		// 這一條就是「不做即時驗證」的機械保證。拿掉 readonly 分支的話，下面這筆
		// REJECTED 會蓋掉投影推導出來的「已通過」，本條立刻轉紅。
		const view = resolveCertTypeView({
			code: "3701",
			mode: "readonly",
			entry: { certNo: "1-009-300022" },
			applications: [
				app({
					status: ProductCertificationStatus.REJECTED,
					publicReason: "查無此編號",
				}),
			],
		});
		expect(view.badge?.status).toBe(ProductCertificationStatus.APPROVED);
		expect(view.withdrawable).toEqual([]);
		expect(view.pending).toBeNull();
	});

	it("沒掛任何認證就沒有徽章", () => {
		expect(
			resolveCertTypeView({ code: "3701", mode: "readonly" }).badge,
		).toBeNull();
	});
});

describe("resolveCertTypeView — review（商品編輯頁）", () => {
	it("待審申請顯示「審核中」，而它根本不在 approved-only 投影裡", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "review",
			entry: undefined, // 投影只含已核准，剛送出的那一筆看不到
			applications: [app({ status: ProductCertificationStatus.SUBMITTED })],
		});
		expect(view.checked).toBe(true);
		expect(view.badge?.label).toBe("審核中");
		expect(view.certNo).toBe("1-009-300022");
	});

	it("換號情境：待審號碼優先顯示，但公開的仍是舊那張", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "review",
			entry: { certNo: "OLD-001" },
			applications: [
				app({
					id: "old",
					status: ProductCertificationStatus.APPROVED,
					certificateNo: "OLD-001",
					submittedAt: "2026-08-20T00:00:00Z",
				}),
				app({
					id: "new",
					status: ProductCertificationStatus.SUBMITTED,
					certificateNo: "NEW-002",
					submittedAt: "2026-08-26T00:00:00Z",
				}),
			],
		});
		expect(view.badge?.label).toBe("審核中");
		expect(view.certNo).toBe("NEW-002");
		expect(view.publicCertNo).toBe("OLD-001");
		// 取消勾選＝停止宣稱整個類型 ⇒ 兩筆都要撤，待審的先撤。
		expect(view.withdrawable.map((a) => a.id)).toEqual(["new", "old"]);
	});

	it("被駁回的那一筆也要顯示：它既不在投影也不在待審，留白等於賣家不知道發生過什麼", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "review",
			applications: [
				app({
					status: ProductCertificationStatus.REJECTED,
					publicReason: "證書持有人與賣場名稱不符",
					reviewedAt: "2026-08-26T05:00:00Z",
				}),
			],
		});
		expect(view.badge?.label).toBe("未通過");
		expect(view.badge?.description).toBe("證書持有人與賣場名稱不符");
		// 終態不再轉出 ⇒ 這個類型現在沒有宣稱，勾選框不該是勾的。
		expect(view.checked).toBe(false);
		expect(view.withdrawable).toEqual([]);
	});

	it("只認自己那個類型的申請，別的類型不得外溢", () => {
		const view = resolveCertTypeView({
			code: "3702",
			mode: "review",
			applications: [
				app({
					certificationType: "3701",
					status: ProductCertificationStatus.SUBMITTED,
				}),
			],
		});
		expect(view.badge).toBeNull();
		expect(view.checked).toBe(false);
	});

	it("申請清單拿不到時（端點尚未部署）退回投影，仍答得出已通過", () => {
		const view = resolveCertTypeView({
			code: "3701",
			mode: "review",
			entry: { certNo: "1-009-300022" },
			applications: [],
		});
		expect(view.badge?.status).toBe(ProductCertificationStatus.APPROVED);
		expect(view.checked).toBe(true);
	});
});
