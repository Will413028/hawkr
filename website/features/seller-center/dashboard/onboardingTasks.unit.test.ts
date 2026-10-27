import { create } from "@bufbuild/protobuf";
import {
	CapabilityStatusSchema,
	FoodRegistrationState,
	GetMyStoreResponseSchema,
	KYCFlowStatus,
	SellerCapability,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { publicListingStatus } from "@/lib/seller/capability-fixtures";
import type { ShippingOnboarding } from "@/lib/seller/shipping";
import { buildOnboardingTasks, type OnboardingTask } from "./onboardingTasks";

const ut = uiTextFor("zh-TW");
const R = SellerRequirement;
const F = KYCFlowStatus;

function shippingStatus(
	publicMissing: SellerRequirement[],
	extra: { shippingMissing?: SellerRequirement[] },
) {
	const missing =
		extra.shippingMissing ??
		(publicMissing.includes(R.KYC_APPROVED) ? [R.KYC_APPROVED] : []);
	return create(CapabilityStatusSchema, {
		capability: SellerCapability.SHIPPING_SETUP,
		active: missing.length === 0,
		missing,
	});
}

function build(
	missing: SellerRequirement[] | undefined,
	extra: {
		flowStatus?: KYCFlowStatus;
		description?: string;
		shippingRows?: ShippingOnboarding[];
		productCount?: number | null;
		foodRegistrationNumber?: string;
		foodRequestState?: FoodRegistrationState;
		/** SHIPPING_SETUP 的缺項；沒給時照伺服器：PUBLIC_LISTING 缺 KYC_APPROVED 才缺它。 */
		shippingMissing?: SellerRequirement[];
	} = {},
): OnboardingTask[] {
	const my = create(GetMyStoreResponseSchema, {
		store: {
			id: "1",
			description: extra.description ?? "",
			approvedFoodRegistration: extra.foodRegistrationNumber
				? { registrationNumber: extra.foodRegistrationNumber }
				: undefined,
			pendingFoodRegistration:
				extra.foodRequestState === FoodRegistrationState.PENDING
					? { state: extra.foodRequestState, registrationNumber: "A-9" }
					: undefined,
			rejectedFoodRegistration:
				extra.foodRequestState === FoodRegistrationState.REJECTED
					? { state: extra.foodRequestState, registrationNumber: "A-9" }
					: undefined,
		},
		capabilities:
			missing === undefined
				? []
				: [publicListingStatus(missing), shippingStatus(missing, extra)],
	});
	return buildOnboardingTasks({
		ut,
		my,
		flowStatus: extra.flowStatus,
		shippingRows: extra.shippingRows ?? [],
		productCount: extra.productCount ?? 0,
	});
}

function task(tasks: OnboardingTask[], id: string): OnboardingTask {
	const found = tasks.find((t) => t.id === id);
	if (!found) throw new Error(`task ${id} not found`);
	return found;
}

const required = (tasks: OnboardingTask[]) =>
	tasks.filter((t) => t.group === "required");
const recommended = (tasks: OnboardingTask[]) =>
	tasks.filter((t) => t.group === "recommended");

describe("公開前必須 follows the PUBLIC_LISTING missing requirements", () => {
	it("nothing missing: every required task is complete", () => {
		const tasks = build([]);
		expect(required(tasks).map((t) => t.id)).toEqual(["profile", "cert"]);
		expect(required(tasks).every((t) => t.completed)).toBe(true);
	});

	it("each store-info requirement keeps the profile task open and names itself", () => {
		for (const [requirement, label] of [
			[R.STORE_NAME, "店名"],
			[R.LOGO, "logo"],
			[R.TOWN_CODE, "所在地"],
		] as const) {
			const profile = task(build([requirement]), "profile");
			expect(profile.completed, label).toBe(false);
			expect(profile.statusLabel, label).toContain(label);
			expect(profile.href).toBe("/seller-center/me");
		}
	});

	it("profile is complete only when none of name, logo and town are missing", () => {
		const tasks = build([R.KYC_APPROVED]);
		expect(task(tasks, "profile").completed).toBe(true);
		expect(task(tasks, "cert").completed).toBe(false);
	});

	it("KYC_APPROVED keeps the cert task open and shows the flow status as its text", () => {
		const cert = task(
			build([R.KYC_APPROVED], { flowStatus: F.KYC_FLOW_STATUS_UNDER_REVIEW }),
			"cert",
		);
		expect(cert.completed).toBe(false);
		expect(cert.statusLabel).toBe("PayUni 審核中");
		expect(cert.href).toBe("/seller-center/me/certifications");
	});

	it("the cert task is decided by the missing requirements, not by the flow status", () => {
		// 伺服器說沒有缺 KYC_APPROVED：就算流程狀態還顯示審核中，這一列也是完成。
		expect(
			task(build([], { flowStatus: F.KYC_FLOW_STATUS_UNDER_REVIEW }), "cert")
				.completed,
		).toBe(true);
		// 伺服器說還缺 KYC_APPROVED：就算流程狀態是 APPROVED，這一列也還沒完成。
		expect(
			task(
				build([R.KYC_APPROVED], { flowStatus: F.KYC_FLOW_STATUS_APPROVED }),
				"cert",
			).completed,
		).toBe(false);
	});

	it("PayUni is folded into the KYC row: there is no separate payment task", () => {
		const ids = build([R.KYC_APPROVED]).map((t) => t.id);
		expect(ids).not.toContain("payuni");
	});

	it("requirements the notice page handles have no row and do not fall into the fallback row", () => {
		for (const requirement of [
			R.NOT_SUSPENDED,
			R.STORE_ACTIVE,
			R.NOT_PENDING_DELETION,
		]) {
			expect(
				build([requirement]).map((t) => t.id),
				String(requirement),
			).toEqual(build([]).map((t) => t.id));
		}
	});

	it("a requirement the client does not know still shows up", () => {
		const tasks = build([99 as SellerRequirement]);
		expect(task(tasks, "other_requirement").completed).toBe(false);
	});

	it("an unspecified requirement is not a task", () => {
		const tasks = build([R.UNSPECIFIED]);
		expect(tasks.some((t) => t.id === "other_requirement")).toBe(false);
	});

	it("without a PUBLIC_LISTING status nothing is treated as satisfied", () => {
		const tasks = build(undefined);
		expect(required(tasks).every((t) => !t.completed)).toBe(true);
	});

	it("the required group does not depend on shipping, products or the store description", () => {
		const bare = required(build([R.LOGO]));
		const busy = required(
			build([R.LOGO], {
				description: "產地直送",
				productCount: 5,
				shippingRows: [{ payuniSubmitStatus: "SUCCEEDED" }],
			}),
		);
		expect(busy.map((t) => [t.id, t.completed])).toEqual(
			bare.map((t) => [t.id, t.completed]),
		);
	});
});

describe("建議完成", () => {
	it("holds shipping, the first product, food registration and the store description", () => {
		expect(recommended(build([])).map((t) => t.id)).toEqual([
			"shipping",
			"product",
			"food",
			"description",
		]);
	});

	it("shipping opens only when the store is certified, whatever the flow says", () => {
		expect(task(build([]), "shipping").locked).toBe(false);
		expect(
			task(
				build([R.KYC_APPROVED], { flowStatus: F.KYC_FLOW_STATUS_APPROVED }),
				"shipping",
			).locked,
		).toBe(true);
		expect(
			task(
				build([R.KYC_APPROVED], { flowStatus: F.KYC_FLOW_STATUS_FORWARDED }),
				"shipping",
			).locked,
		).toBe(true);
	});

	it("shipping is gated by SHIPPING_SETUP, not by the PUBLIC_LISTING list", () => {
		// PUBLIC_LISTING 說缺 KYC_APPROVED，但 SHIPPING_SETUP 啟用：不鎖。
		expect(
			task(build([R.KYC_APPROVED], { shippingMissing: [] }), "shipping").locked,
		).toBe(false);
		// 認證已核准（PUBLIC_LISTING 不缺 KYC），但 SHIPPING_SETUP 缺 NOT_SUSPENDED：鎖，不說需要認證。
		const suspended = task(
			build([R.NOT_SUSPENDED], { shippingMissing: [R.NOT_SUSPENDED] }),
			"shipping",
		);
		expect(suspended.locked).toBe(true);
		expect(suspended.statusLabel).not.toBe("需先通過認證");
		// 缺 KYC_APPROVED：鎖，文案是需先通過認證。
		const kyc = task(build([R.KYC_APPROVED]), "shipping");
		expect(kyc.locked).toBe(true);
		expect(kyc.statusLabel).toBe("需先通過認證");
	});

	it("suspended and KYC not approved: the logistics row reads 未營業, not 需先通過認證", () => {
		const row = task(
			build([R.KYC_APPROVED, R.NOT_SUSPENDED], {
				shippingMissing: [R.NOT_SUSPENDED, R.KYC_APPROVED],
			}),
			"shipping",
		);
		expect(row.locked).toBe(true);
		expect(row.statusLabel).toBe("未營業");
	});

	it("the first product is never locked behind certification", () => {
		expect(task(build([R.KYC_APPROVED]), "product").locked).toBeFalsy();
	});

	it("completion follows the store and product data", () => {
		const tasks = build([], {
			description: "產地直送",
			productCount: 2,
			foodRegistrationNumber: "A-123",
		});
		expect(task(tasks, "description").completed).toBe(true);
		expect(task(tasks, "product").completed).toBe(true);
		expect(task(tasks, "food").completed).toBe(true);
		expect(task(build([]), "description").completed).toBe(false);
		expect(task(build([]), "food").completed).toBe(false);
	});

	it("the food task always points to the wallet food row, whatever the KYC state", () => {
		const expected =
			"/seller-center/wallet?section=food-registration#food-registration";
		for (const flowStatus of [
			undefined,
			F.KYC_FLOW_STATUS_NOT_STARTED,
			F.KYC_FLOW_STATUS_UNDER_REVIEW,
			F.KYC_FLOW_STATUS_APPROVED,
		]) {
			const food = task(build([R.KYC_APPROVED], { flowStatus }), "food");
			expect(food.href).toBe(expected);
			expect(food.panelDescription).toBe(
				"若販售加工食品，請在錢包與認證送出食品業者登錄字號審核；一般生鮮商品可略過。",
			);
		}
	});

	it("the food task labels depend only on whether a registration is approved", () => {
		const none = task(build([]), "food");
		expect([none.cta, none.statusLabel]).toEqual(["填寫資料", "尚未登錄"]);
		const pending = task(
			build([], { foodRequestState: FoodRegistrationState.PENDING }),
			"food",
		);
		expect(pending.completed).toBe(false);
		expect([pending.cta, pending.statusLabel]).toEqual([
			"填寫資料",
			"尚未登錄",
		]);
		const approved = task(
			build([], { foodRegistrationNumber: "A-123" }),
			"food",
		);
		expect([approved.cta, approved.statusLabel]).toEqual([
			"查看或更新資料",
			"已登錄",
		]);
	});

	it("no task asks the seller to publish a post first", () => {
		const tasks = build([R.KYC_APPROVED, R.LOGO]);
		expect(tasks.some((t) => t.id === ("post" as OnboardingTask["id"]))).toBe(
			false,
		);
		for (const t of tasks) {
			expect(
				`${t.description}${t.panelDescription}${t.statusLabel}`,
			).not.toMatch(/發布動態|發布.*動態/);
		}
	});
});
