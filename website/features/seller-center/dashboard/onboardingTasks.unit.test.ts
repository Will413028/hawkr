import { create } from "@bufbuild/protobuf";
import {
	CapabilityStatusSchema,
	GetMyStoreResponseSchema,
	KYCStatus,
	PaymentAccountSchema,
	PaymentAccountStatus,
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

function shippingStatus(
	publicMissing: SellerRequirement[],
	extra: { shippingMissing?: SellerRequirement[] },
) {
	const missing =
		extra.shippingMissing ??
		(publicMissing.includes(R.ONBOARDING_COMPLETE)
			? [R.ONBOARDING_COMPLETE]
			: []);
	return create(CapabilityStatusSchema, {
		capability: SellerCapability.SHIPPING_SETUP,
		active: missing.length === 0,
		missing,
	});
}

function build(
	missing: SellerRequirement[] | undefined,
	extra: {
		kycStatus?: KYCStatus;
		paymentStatus?: PaymentAccountStatus;
		payoutsActive?: boolean;
		description?: string;
		shippingRows?: ShippingOnboarding[];
		productCount?: number | null;
		/** SHIPPING_SETUP 的缺項；沒給時照伺服器：PUBLIC_LISTING 缺 ONBOARDING_COMPLETE 才缺它。 */
		shippingMissing?: SellerRequirement[];
	} = {},
): OnboardingTask[] {
	const my = create(GetMyStoreResponseSchema, {
		store: {
			id: "1",
			description: extra.description ?? "",
		},
		capabilities:
			missing === undefined
				? []
				: [publicListingStatus(missing), shippingStatus(missing, extra)],
	});
	return buildOnboardingTasks({
		ut,
		my,
		kycStatus: extra.kycStatus,
		paymentAccount:
			extra.paymentStatus === undefined
				? null
				: create(PaymentAccountSchema, {
						status: extra.paymentStatus,
						payoutsActive: extra.payoutsActive ?? false,
					}),
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
		expect(required(tasks).map((t) => t.id)).toEqual([
			"profile",
			"platform_review",
			"payment_account",
		]);
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
		const tasks = build([R.ONBOARDING_COMPLETE]);
		expect(task(tasks, "profile").completed).toBe(true);
		expect(task(tasks, "platform_review").completed).toBe(false);
		expect(task(tasks, "payment_account").completed).toBe(false);
	});

	it("before the review is approved: 平台審核 shows the KYC status and 收款帳戶 is greyed", () => {
		const submitted = build([R.ONBOARDING_COMPLETE], {
			kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
		});
		const review = task(submitted, "platform_review");
		expect(review.completed).toBe(false);
		expect(review.statusLabel).toBe("審核中");
		expect(review.href).toBe("/seller-center/me/certifications");
		const payment = task(submitted, "payment_account");
		expect(payment.locked).toBe(true);
		expect(payment.statusLabel).toBe("平台審核通過後開放設定");
		expect(payment.cta).toBeUndefined();
		// 沒有申請：未送出。
		expect(
			task(build([R.ONBOARDING_COMPLETE]), "platform_review").statusLabel,
		).toBe("未送出");
	});

	it("approved review, payment not enabled: 收款帳戶 shows the catalog badge and action", () => {
		const tasks = build([R.ONBOARDING_COMPLETE], {
			kycStatus: KYCStatus.KYC_STATUS_APPROVED,
			paymentStatus: PaymentAccountStatus.ACTION_REQUIRED,
		});
		expect(task(tasks, "platform_review").completed).toBe(true);
		const payment = task(tasks, "payment_account");
		expect(payment.completed).toBe(false);
		expect(payment.locked).toBe(false);
		expect(payment.statusLabel).toBe("待設定");
		expect(payment.cta).toBe("前往設定收款帳戶");
		expect(payment.statusLabel).not.toMatch(/逾期|停用/);
	});

	it("approved review without an account yet reads as 準備中", () => {
		const payment = task(
			build([R.ONBOARDING_COMPLETE], {
				kycStatus: KYCStatus.KYC_STATUS_APPROVED,
			}),
			"payment_account",
		);
		expect(payment.statusLabel).toBe("準備中");
	});

	it("both rows are decided by the missing requirements, not by the statuses", () => {
		// 伺服器說沒有缺 ONBOARDING_COMPLETE：就算狀態還顯示審核中，兩列都是完成。
		const done = build([], {
			kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
			paymentStatus: PaymentAccountStatus.UNDER_REVIEW,
		});
		expect(task(done, "platform_review").completed).toBe(true);
		expect(task(done, "payment_account").completed).toBe(true);
		expect(task(done, "platform_review").cta).toBeUndefined();
		expect(task(done, "payment_account").cta).toBeUndefined();
		// 伺服器說還缺 ONBOARDING_COMPLETE：就算收款帳戶 ENABLED，收款那一列也還沒完成。
		expect(
			task(
				build([R.ONBOARDING_COMPLETE], {
					kycStatus: KYCStatus.KYC_STATUS_APPROVED,
					paymentStatus: PaymentAccountStatus.ENABLED,
				}),
				"payment_account",
			).completed,
		).toBe(false);
	});

	it("both titles are client text and the PayUni row stays folded away", () => {
		const tasks = build([R.ONBOARDING_COMPLETE]);
		expect(task(tasks, "platform_review").title).toBe("平台審核");
		expect(task(tasks, "payment_account").title).toBe("收款帳戶");
		expect(tasks.map((t) => t.id)).not.toContain("payuni");
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
	it("holds shipping, the first product and the store description", () => {
		expect(recommended(build([])).map((t) => t.id)).toEqual([
			"shipping",
			"product",
			"description",
		]);
	});

	it("shipping opens only when the store is certified, whatever the statuses say", () => {
		expect(task(build([]), "shipping").locked).toBe(false);
		expect(
			task(
				build([R.ONBOARDING_COMPLETE], {
					kycStatus: KYCStatus.KYC_STATUS_APPROVED,
					paymentStatus: PaymentAccountStatus.UNDER_REVIEW,
				}),
				"shipping",
			).locked,
		).toBe(true);
	});

	it("shipping is gated by SHIPPING_SETUP, not by the PUBLIC_LISTING list", () => {
		// PUBLIC_LISTING 說缺 ONBOARDING_COMPLETE，但 SHIPPING_SETUP 啟用：不鎖。
		expect(
			task(build([R.ONBOARDING_COMPLETE], { shippingMissing: [] }), "shipping")
				.locked,
		).toBe(false);
		// 認證已核准（PUBLIC_LISTING 不缺 KYC），但 SHIPPING_SETUP 缺 NOT_SUSPENDED：鎖，不說需要認證。
		const suspended = task(
			build([R.NOT_SUSPENDED], { shippingMissing: [R.NOT_SUSPENDED] }),
			"shipping",
		);
		expect(suspended.locked).toBe(true);
		expect(suspended.statusLabel).not.toBe("需先通過認證");
		// 缺 ONBOARDING_COMPLETE：鎖，文案是需先通過認證。
		const kyc = task(build([R.ONBOARDING_COMPLETE]), "shipping");
		expect(kyc.locked).toBe(true);
		expect(kyc.statusLabel).toBe("需先通過認證");
	});

	it("suspended and KYC not approved: the logistics row reads 未營業, not 需先通過認證", () => {
		const row = task(
			build([R.ONBOARDING_COMPLETE, R.NOT_SUSPENDED], {
				shippingMissing: [R.NOT_SUSPENDED, R.ONBOARDING_COMPLETE],
			}),
			"shipping",
		);
		expect(row.locked).toBe(true);
		expect(row.statusLabel).toBe("未營業");
	});

	it("the first product is never locked behind certification", () => {
		expect(task(build([R.ONBOARDING_COMPLETE]), "product").locked).toBeFalsy();
	});

	it("completion follows the store and product data", () => {
		const tasks = build([], {
			description: "產地直送",
			productCount: 2,
		});
		expect(task(tasks, "description").completed).toBe(true);
		expect(task(tasks, "product").completed).toBe(true);
		expect(task(build([]), "description").completed).toBe(false);
	});

	it("no task asks the seller to publish a post first", () => {
		const tasks = build([R.ONBOARDING_COMPLETE, R.LOGO]);
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
