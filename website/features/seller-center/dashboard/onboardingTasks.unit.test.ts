import { create } from "@bufbuild/protobuf";
import {
	CapabilityStatusSchema,
	GetMyStoreResponseSchema,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
	SellerCapability,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { publicListingStatus } from "@/lib/seller/capability-fixtures";
import {
	onboardingFixture,
	requiredFixture,
} from "@/lib/seller/onboarding-fixtures";
import type { ShippingOnboarding } from "@/lib/seller/shipping";
import { buildOnboardingTasks, type OnboardingTask } from "./onboardingTasks";

const ut = uiTextFor("zh-TW");
const R = SellerRequirement;
const S = OnboardingTaskState;
const A = OnboardingAction;
const ID = OnboardingTaskId;

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

const NOT_DONE = { state: S.ACTION_REQUIRED, action: A.NONE } as const;

/** 建議任務由 client 組，只看店家的能力與資料；必要任務由伺服器的摘要給（預設三項都還沒完成）。 */
function build(
	missing: SellerRequirement[] | undefined,
	extra: {
		onboarding?: Parameters<typeof buildOnboardingTasks>[0]["onboarding"];
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
		onboarding:
			extra.onboarding === undefined
				? requiredFixture(NOT_DONE, NOT_DONE, NOT_DONE)
				: extra.onboarding,
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

describe("公開前必須 comes from the server's GetMyOnboarding", () => {
	it("keeps the server order and shows its title and status sentence verbatim", () => {
		const onboarding = onboardingFixture([
			{
				id: ID.PROFILE,
				state: S.ACTION_REQUIRED,
				title: "伺服器標題X",
				status: "伺服器狀態X",
			},
			{
				id: ID.PLATFORM_REVIEW,
				state: S.IN_PROGRESS,
				title: "伺服器標題Y",
				status: "伺服器狀態Y",
			},
			{
				id: ID.PAYMENT_ACCOUNT,
				state: S.LOCKED,
				title: "伺服器標題Z",
				status: "伺服器狀態Z",
			},
		]);
		const tasks = required(build([], { onboarding }));
		expect(tasks.map((t) => [t.id, t.title, t.statusLabel])).toEqual([
			["profile", "伺服器標題X", "伺服器狀態X"],
			["platform_review", "伺服器標題Y", "伺服器狀態Y"],
			["payment_account", "伺服器標題Z", "伺服器狀態Z"],
		]);
	});

	it("completed is state COMPLETED and locked is state LOCKED; nothing else", () => {
		for (const [state, completed, locked] of [
			[S.COMPLETED, true, false],
			[S.LOCKED, false, true],
			[S.ACTION_REQUIRED, false, false],
			[S.IN_PROGRESS, false, false],
			[S.UNKNOWN, false, false],
			[S.UNSPECIFIED, false, false],
			[99 as OnboardingTaskState, false, false],
		] as const) {
			const onboarding = onboardingFixture([{ id: ID.PLATFORM_REVIEW, state }]);
			const t = task(build([], { onboarding }), "platform_review");
			expect([t.completed, t.locked], String(state)).toEqual([
				completed,
				locked,
			]);
		}
	});

	it("UNKNOWN shows the server's status sentence and offers no button", () => {
		const onboarding = onboardingFixture([
			{
				id: ID.PAYMENT_ACCOUNT,
				state: S.UNKNOWN,
				action: A.NONE,
				status: "伺服器說讀不到",
			},
		]);
		const t = task(build([], { onboarding }), "payment_account");
		expect(t.statusLabel).toBe("伺服器說讀不到");
		expect(t.completed).toBe(false);
		expect(t.locked).toBe(false);
		expect(t.next).toBeUndefined();
	});

	it("state comes from the server, not from the store's capabilities or KYC", () => {
		// 店家的能力說 ONBOARDING_COMPLETE 沒缺，摘要說兩項都還沒完成：跟摘要。
		const stillOpen = build([], {
			onboarding: requiredFixture(
				{ state: S.COMPLETED, action: A.NONE },
				{ state: S.ACTION_REQUIRED, action: A.SUBMIT_CERTIFICATION },
				{ state: S.LOCKED, action: A.NONE },
			),
		});
		expect(task(stillOpen, "platform_review").completed).toBe(false);
		expect(task(stillOpen, "payment_account").completed).toBe(false);
		// 店家的能力說還缺 ONBOARDING_COMPLETE，缺店名，摘要說都完成：跟摘要。
		const done = build([R.ONBOARDING_COMPLETE, R.STORE_NAME], {
			onboarding: requiredFixture(
				{ state: S.COMPLETED, action: A.NONE },
				{ state: S.COMPLETED, action: A.NONE },
				{ state: S.COMPLETED, action: A.NONE },
			),
		});
		expect(required(done).every((t) => t.completed)).toBe(true);
	});

	it("button label comes from the catalog per action; NONE has no button", () => {
		const labels = new Map<OnboardingAction, string>([
			[A.EDIT_PROFILE, "去設定"],
			[A.SUBMIT_CERTIFICATION, "前往商家認證"],
			[A.START_PAYMENT_ONBOARDING, "前往設定收款帳戶"],
			[A.REFRESH_PAYMENT_ACCOUNT, "重新整理"],
			[A.CONTACT_SUPPORT, "聯絡客服"],
		]);
		for (const [action, label] of labels) {
			const onboarding = onboardingFixture([
				{ id: ID.PROFILE, state: S.ACTION_REQUIRED, action },
			]);
			expect(
				task(build([], { onboarding }), "profile").next?.label,
				String(action),
			).toBe(label);
		}
		for (const action of [A.NONE, A.UNSPECIFIED, 99 as OnboardingAction]) {
			const onboarding = onboardingFixture([
				{ id: ID.PROFILE, state: S.ACTION_REQUIRED, action },
			]);
			expect(
				task(build([], { onboarding }), "profile").next,
				String(action),
			).toBeUndefined();
		}
	});

	it("action maps to the web destination; refresh stays in place", () => {
		const next = (action: OnboardingAction) =>
			task(
				build([], {
					onboarding: onboardingFixture([
						{ id: ID.PAYMENT_ACCOUNT, state: S.ACTION_REQUIRED, action },
					]),
				}),
				"payment_account",
			).next;
		expect(next(A.EDIT_PROFILE)).toMatchObject({
			kind: "link",
			href: "/seller-center/me",
		});
		expect(next(A.SUBMIT_CERTIFICATION)).toMatchObject({
			kind: "link",
			href: "/seller-center/me/certifications",
		});
		expect(next(A.START_PAYMENT_ONBOARDING)).toMatchObject({
			kind: "link",
			href: "/seller-center/me/certifications",
		});
		expect(next(A.CONTACT_SUPPORT)).toMatchObject({
			kind: "link",
			href: "/contact",
		});
		expect(next(A.REFRESH_PAYMENT_ACCOUNT)?.kind).toBe("refresh");
	});

	it("presentation fields depend on the task id only, not on the state", () => {
		const present = (state: OnboardingTaskState) => {
			const t = task(
				build([], {
					onboarding: onboardingFixture([{ id: ID.PLATFORM_REVIEW, state }]),
				}),
				"platform_review",
			);
			return [t.icon, t.panelTitle, t.panelDescription, t.description];
		};
		expect(present(S.COMPLETED)).toEqual(present(S.LOCKED));
		expect(present(S.COMPLETED)).toEqual(present(S.UNKNOWN));
	});

	it("description and panel texts come from the shared catalog per task id", () => {
		const tasks = build([], {
			onboarding: onboardingFixture([
				{ id: ID.PROFILE, state: S.COMPLETED },
				{ id: ID.PLATFORM_REVIEW, state: S.COMPLETED },
				{ id: ID.PAYMENT_ACCOUNT, state: S.COMPLETED },
			]),
		});
		expect(
			tasks
				.slice(0, 3)
				.map((t) => [t.description, t.panelTitle, t.panelDescription]),
		).toEqual([
			[
				"店名、logo 與所在地。",
				"完成商店資料",
				"買家要看得到你的店，需要有店名、logo 與所在地。",
			],
			[
				"商家資料與統編的平台審核。",
				"完成平台審核",
				"平台審核通過後，店面才能公開，並開放設定收款帳戶。",
			],
			[
				"向金流商設定身分與銀行資料。",
				"設定收款帳戶",
				"身分與銀行資料由金流商收集，平台不經手；收款帳戶啟用後才能正式收款。",
			],
		]);
	});

	it("no summary (not loaded or failed): no required tasks, recommended stay", () => {
		const tasks = build([], { onboarding: null });
		expect(required(tasks)).toEqual([]);
		expect(recommended(tasks).map((t) => t.id)).toEqual([
			"shipping",
			"product",
			"description",
		]);
	});

	it("UNSPECIFIED id is skipped; an id the web has no presentation for still renders with the server's title, status and action", () => {
		const onboarding = onboardingFixture([
			{ id: ID.UNSPECIFIED, state: S.ACTION_REQUIRED },
			{ id: ID.PROFILE, state: S.COMPLETED },
			{
				id: 77 as OnboardingTaskId,
				state: S.ACTION_REQUIRED,
				action: A.CONTACT_SUPPORT,
				title: "未來任務標題",
				status: "未來任務狀態",
			},
		]);
		const tasks = required(build([], { onboarding }));
		expect(tasks.map((t) => t.id)).toEqual(["profile", "required_77"]);
		const future = task(tasks, "required_77");
		expect(future.title).toBe("未來任務標題");
		expect(future.statusLabel).toBe("未來任務狀態");
		expect(future.completed).toBe(false);
		expect(future.icon).toBe("activity");
		expect(future.next).toMatchObject({ kind: "link", href: "/contact" });
	});

	it("the required group does not depend on shipping, products or the store description", () => {
		const onboarding = requiredFixture(
			{ state: S.COMPLETED, action: A.NONE },
			{ state: S.ACTION_REQUIRED, action: A.SUBMIT_CERTIFICATION },
			{ state: S.LOCKED, action: A.NONE },
		);
		const bare = required(build([R.LOGO], { onboarding }));
		const busy = required(
			build([R.LOGO], {
				onboarding,
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
		expect(task(build([R.ONBOARDING_COMPLETE], {}), "shipping").locked).toBe(
			true,
		);
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
