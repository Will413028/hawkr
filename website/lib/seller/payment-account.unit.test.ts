import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	EmbeddedOnboardingRuntime,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	OnboardingReturnDestination,
	OnboardingTaskId,
	OnboardingTaskState,
	PaymentAccountSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { onboardingFixture, requiredFixture } from "./onboarding-fixtures";
import {
	createPaymentOnboardingSession,
	fetchMyPaymentAccount,
	paymentAccountCopy,
	paymentAccountTone,
	paymentBannerCopy,
} from "./payment-account";

const ut = uiTextFor("zh-TW");
const account = (
	status: PaymentAccountStatus,
	extra: { payoutsActive?: boolean } = {},
) =>
	create(PaymentAccountSchema, {
		status,
		payoutsActive: extra.payoutsActive ?? false,
	});

describe("fetchMyPaymentAccount", () => {
	it("not_found 是還沒有收款帳戶，其他錯誤照丟", async () => {
		const getMyPaymentAccount = vi
			.fn()
			.mockRejectedValue(new ConnectError("x", Code.NotFound));
		await expect(
			fetchMyPaymentAccount(() => ({ getMyPaymentAccount }) as never),
		).resolves.toBeNull();
		getMyPaymentAccount.mockRejectedValue(
			new ConnectError("x", Code.Unavailable),
		);
		await expect(
			fetchMyPaymentAccount(() => ({ getMyPaymentAccount }) as never),
		).rejects.toBeInstanceOf(ConnectError);
	});

	it("回傳伺服器的帳戶", async () => {
		const found = account(PaymentAccountStatus.ENABLED);
		const getMyPaymentAccount = vi.fn().mockResolvedValue({ account: found });
		await expect(
			fetchMyPaymentAccount(() => ({ getMyPaymentAccount }) as never),
		).resolves.toBe(found);
	});
});

describe("createPaymentOnboardingSession", () => {
	const client = (res: object) => {
		const createPaymentOnboardingSessionRpc = vi.fn().mockResolvedValue(res);
		return {
			rpc: createPaymentOnboardingSessionRpc,
			factory: () =>
				({
					createPaymentOnboardingSession: createPaymentOnboardingSessionRpc,
				}) as never,
		};
	};
	const hosted = {
		session: { case: "hosted", value: { url: "https://x.test" } },
	};
	const stripe = (value: object) => ({
		session: {
			case: "embedded",
			value: { provider: { case: "stripe", value } },
		},
	});

	it("只送回跳目的地與宣告能承載的內嵌執行環境 STRIPE_CONNECT_JS，網址與語系由伺服器處理", async () => {
		const c = client(hosted);
		await createPaymentOnboardingSession(c.factory);
		expect(c.rpc).toHaveBeenCalledWith({
			destination: OnboardingReturnDestination.WEBSITE_CERTIFICATION,
			embeddedRuntimes: [EmbeddedOnboardingRuntime.STRIPE_CONNECT_JS],
		});
	});

	it("hosted 回連結，embedded.stripe 回 client secret 與公開金鑰", async () => {
		await expect(
			createPaymentOnboardingSession(client(hosted).factory),
		).resolves.toEqual({ kind: "hosted", url: "https://x.test" });
		await expect(
			createPaymentOnboardingSession(
				client(stripe({ clientSecret: "cs_1", publishableKey: "pk_1" }))
					.factory,
			),
		).resolves.toEqual({
			kind: "embedded",
			provider: "stripe",
			clientSecret: "cs_1",
			publishableKey: "pk_1",
		});
	});

	it("兩支都沒設、embedded 的金流商不認得或缺欄位：丟錯，不當成有效的 session", async () => {
		for (const res of [
			{ session: { case: undefined } },
			{ session: { case: "hosted", value: { url: "" } } },
			{
				session: { case: "embedded", value: { provider: { case: undefined } } },
			},
			stripe({ clientSecret: "", publishableKey: "pk_1" }),
			stripe({ clientSecret: "cs_1", publishableKey: "" }),
		]) {
			await expect(
				createPaymentOnboardingSession(client(res).factory),
			).rejects.toThrow();
		}
	});
});

describe("paymentAccountCopy：徽章只來自共用文案目錄", () => {
	it("每個狀態的徽章", () => {
		const table: [PaymentAccountStatus, string][] = [
			[PaymentAccountStatus.NOT_CREATED, "準備中"],
			[PaymentAccountStatus.ACTION_REQUIRED, "待設定"],
			[PaymentAccountStatus.UNDER_REVIEW, "審核中"],
			[PaymentAccountStatus.ENABLED, "已啟用"],
			[PaymentAccountStatus.DISABLED, "無法啟用"],
		];
		for (const [status, badge] of table) {
			expect(
				paymentAccountCopy(ut, account(status)).badge,
				String(status),
			).toBe(badge);
		}
	});

	it("已啟用而撥款尚未啟用：多一行說明；撥款已啟用或不是已啟用：沒有", () => {
		expect(
			paymentAccountCopy(ut, account(PaymentAccountStatus.ENABLED)).payoutsNote,
		).toBe("撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。");
		expect(
			paymentAccountCopy(
				ut,
				account(PaymentAccountStatus.ENABLED, { payoutsActive: true }),
			).payoutsNote,
		).toBeUndefined();
		expect(
			paymentAccountCopy(ut, account(PaymentAccountStatus.UNDER_REVIEW))
				.payoutsNote,
		).toBeUndefined();
	});

	it("UNSPECIFIED 與不認得的狀態沒有文字，不被當成有效狀態", () => {
		for (const status of [
			PaymentAccountStatus.UNSPECIFIED,
			99 as PaymentAccountStatus,
		]) {
			const copy = paymentAccountCopy(ut, account(status));
			expect(copy.badge).toBeUndefined();
			expect(paymentAccountTone(status)).toBeUndefined();
		}
	});
});

describe("paymentAccountTone", () => {
	it("色調", () => {
		expect(paymentAccountTone(PaymentAccountStatus.NOT_CREATED)).toBe("info");
		expect(paymentAccountTone(PaymentAccountStatus.UNDER_REVIEW)).toBe("info");
		expect(paymentAccountTone(PaymentAccountStatus.ACTION_REQUIRED)).toBe(
			"warning",
		);
		expect(paymentAccountTone(PaymentAccountStatus.ENABLED)).toBe("success");
		expect(paymentAccountTone(PaymentAccountStatus.DISABLED)).toBe("danger");
	});
});

describe("paymentBannerCopy（儀表板的收款帳戶提示）", () => {
	const task = (state: OnboardingTaskState) =>
		requiredFixture(
			{ state: OnboardingTaskState.COMPLETED },
			{ state: OnboardingTaskState.COMPLETED },
			{ state },
		);

	it("摘要說收款帳戶任務未完成：即使帳戶 status 是 ENABLED 也顯示", () => {
		const copy = paymentBannerCopy(
			ut,
			account(PaymentAccountStatus.ENABLED),
			task(OnboardingTaskState.ACTION_REQUIRED),
		);
		expect(copy).not.toBeNull();
	});

	it("徽章讀帳戶狀態，說明句讀任務（兩者不一致時說明句仍是任務的）", () => {
		const copy = paymentBannerCopy(
			ut,
			account(PaymentAccountStatus.ACTION_REQUIRED),
			onboardingFixture([
				{
					id: OnboardingTaskId.PAYMENT_ACCOUNT,
					state: OnboardingTaskState.IN_PROGRESS,
					status: "伺服器的收款句",
				},
			]),
		);
		expect(copy?.badge).toBe("待設定");
		expect(copy?.description).toBe("伺服器的收款句");
	});

	it("摘要說完成：即使帳戶 status 是 UNDER_REVIEW 也不顯示", () => {
		expect(
			paymentBannerCopy(
				ut,
				account(PaymentAccountStatus.UNDER_REVIEW),
				task(OnboardingTaskState.COMPLETED),
			),
		).toBeNull();
	});

	it("沒讀到摘要或沒有帳戶：不顯示", () => {
		expect(
			paymentBannerCopy(ut, account(PaymentAccountStatus.UNDER_REVIEW), null),
		).toBeNull();
		expect(
			paymentBannerCopy(ut, null, task(OnboardingTaskState.ACTION_REQUIRED)),
		).toBeNull();
	});
});
