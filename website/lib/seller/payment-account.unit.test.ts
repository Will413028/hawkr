import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	OnboardingSurface,
	OnboardingTaskState,
	PaymentAccountSchema,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { requiredFixture } from "./onboarding-fixtures";
import {
	createPaymentOnboardingSession,
	fetchMyPaymentAccount,
	paymentAccountCanOnboard,
	paymentAccountCopy,
	paymentAccountTone,
	paymentBannerCopy,
} from "./payment-account";

const ut = uiTextFor("zh-TW");
const account = (
	status: PaymentAccountStatus,
	extra: { surface?: OnboardingSurface; payoutsActive?: boolean } = {},
) =>
	create(PaymentAccountSchema, {
		status,
		onboardingSurface: extra.surface ?? OnboardingSurface.HOSTED,
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
	const urls = {
		returnUrl: "https://h.test/r",
		refreshUrl: "https://h.test/f",
	};
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

	it("把 return_url 與 refresh_url 交給伺服器", async () => {
		const c = client({
			surface: OnboardingSurface.HOSTED,
			url: "https://x.test",
		});
		await createPaymentOnboardingSession(urls, c.factory);
		expect(c.rpc).toHaveBeenCalledWith({
			returnUrl: urls.returnUrl,
			refreshUrl: urls.refreshUrl,
		});
	});

	it("HOSTED 回連結，EMBEDDED 回 client secret", async () => {
		await expect(
			createPaymentOnboardingSession(
				urls,
				client({ surface: OnboardingSurface.HOSTED, url: "https://x.test" })
					.factory,
			),
		).resolves.toEqual({ surface: "hosted", url: "https://x.test" });
		await expect(
			createPaymentOnboardingSession(
				urls,
				client({ surface: OnboardingSurface.EMBEDDED, clientSecret: "cs_1" })
					.factory,
			),
		).resolves.toEqual({ surface: "embedded", clientSecret: "cs_1" });
	});

	it("缺對應欄位或介面未知：丟錯，不當成有效的 session", async () => {
		for (const res of [
			{ surface: OnboardingSurface.HOSTED, url: "" },
			{ surface: OnboardingSurface.EMBEDDED, clientSecret: "" },
			{ surface: OnboardingSurface.UNSPECIFIED, url: "https://x.test" },
		]) {
			await expect(
				createPaymentOnboardingSession(urls, client(res).factory),
			).rejects.toThrow();
		}
	});
});

describe("paymentAccountCopy：文字只來自共用文案目錄", () => {
	it("每個狀態的徽章、說明與動作", () => {
		const table: [PaymentAccountStatus, string, string, string | undefined][] =
			[
				[
					PaymentAccountStatus.NOT_CREATED,
					"準備中",
					"正在建立收款帳戶，通常幾分鐘內完成。",
					"重新整理",
				],
				[
					PaymentAccountStatus.ACTION_REQUIRED,
					"待設定",
					"提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。",
					"前往設定收款帳戶",
				],
				[
					PaymentAccountStatus.UNDER_REVIEW,
					"審核中",
					"金流商正在審核你的資料，不需要額外操作。",
					"重新整理",
				],
				[PaymentAccountStatus.ENABLED, "已啟用", "收款已啟用。", undefined],
				[
					PaymentAccountStatus.DISABLED,
					"無法啟用",
					"金流商目前無法啟用你的收款帳戶，請聯絡客服。",
					"聯絡客服",
				],
			];
		for (const [status, badge, description, action] of table) {
			const copy = paymentAccountCopy(ut, account(status));
			expect(copy.badge, String(status)).toBe(badge);
			expect(copy.description, String(status)).toBe(description);
			expect(copy.action, String(status)).toBe(action);
		}
	});

	it("ACTION_REQUIRED 的文案不說逾期或停用（D13）", () => {
		const copy = paymentAccountCopy(
			ut,
			account(PaymentAccountStatus.ACTION_REQUIRED),
		);
		expect(`${copy.badge}${copy.description}${copy.action}`).not.toMatch(
			/逾期|停用|過期/,
		);
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
			expect(copy.description).toBeUndefined();
			expect(copy.action).toBeUndefined();
			expect(paymentAccountTone(status)).toBeUndefined();
			expect(paymentAccountCanOnboard(account(status))).toBe(false);
		}
	});
});

describe("paymentAccountTone 與能否開通", () => {
	it("色調", () => {
		expect(paymentAccountTone(PaymentAccountStatus.NOT_CREATED)).toBe("info");
		expect(paymentAccountTone(PaymentAccountStatus.UNDER_REVIEW)).toBe("info");
		expect(paymentAccountTone(PaymentAccountStatus.ACTION_REQUIRED)).toBe(
			"warning",
		);
		expect(paymentAccountTone(PaymentAccountStatus.ENABLED)).toBe("success");
		expect(paymentAccountTone(PaymentAccountStatus.DISABLED)).toBe("danger");
	});

	it("只有 ACTION_REQUIRED 而且伺服器告知介面時能開始開通", () => {
		expect(
			paymentAccountCanOnboard(account(PaymentAccountStatus.ACTION_REQUIRED)),
		).toBe(true);
		expect(
			paymentAccountCanOnboard(
				account(PaymentAccountStatus.ACTION_REQUIRED, {
					surface: OnboardingSurface.EMBEDDED,
				}),
			),
		).toBe(true);
		expect(
			paymentAccountCanOnboard(
				account(PaymentAccountStatus.ACTION_REQUIRED, {
					surface: OnboardingSurface.UNSPECIFIED,
				}),
			),
		).toBe(false);
		for (const status of [
			PaymentAccountStatus.NOT_CREATED,
			PaymentAccountStatus.UNDER_REVIEW,
			PaymentAccountStatus.ENABLED,
			PaymentAccountStatus.DISABLED,
		]) {
			expect(paymentAccountCanOnboard(account(status))).toBe(false);
		}
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
