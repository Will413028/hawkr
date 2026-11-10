import { create } from "@bufbuild/protobuf";
import {
	OnboardingSurface,
	type PaymentAccount,
	PaymentAccountSchema,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import type { OnboardingSession } from "./payment-account";
import {
	confirmPaymentReturn,
	embeddedClientSecret,
	handlePaymentParam,
	onboardingUrls,
	paymentParamOf,
	startOnboarding,
} from "./payment-onboarding";

const account = (status: PaymentAccountStatus): PaymentAccount =>
	create(PaymentAccountSchema, { status });

const hosted: OnboardingSession = {
	surface: "hosted",
	url: "https://connect.stripe.test/setup/abc",
};
const embedded: OnboardingSession = {
	surface: "embedded",
	clientSecret: "secret_123",
};

describe("onboardingUrls", () => {
	it("return 與 refresh 是同一個認證頁加上不同的 payment 參數，帶語系前綴與 origin", () => {
		expect(
			onboardingUrls("https://hawkr.test", "/seller-center/me/certifications"),
		).toEqual({
			returnUrl:
				"https://hawkr.test/seller-center/me/certifications?payment=return",
			refreshUrl:
				"https://hawkr.test/seller-center/me/certifications?payment=refresh",
		});
		expect(
			onboardingUrls(
				"https://hawkr.test",
				"/en/seller-center/me/certifications",
			).returnUrl,
		).toBe(
			"https://hawkr.test/en/seller-center/me/certifications?payment=return",
		);
	});
});

describe("paymentParamOf", () => {
	it("只認得 return 與 refresh", () => {
		expect(paymentParamOf("return")).toBe("return");
		expect(paymentParamOf("refresh")).toBe("refresh");
		expect(paymentParamOf("done")).toBeNull();
		expect(paymentParamOf("")).toBeNull();
		expect(paymentParamOf(null)).toBeNull();
	});
});

describe("startOnboarding", () => {
	it("HOSTED：建立 session，整頁導到連結", async () => {
		const createSession = vi.fn().mockResolvedValue(hosted);
		await expect(
			startOnboarding(OnboardingSurface.HOSTED, { createSession }),
		).resolves.toEqual({ kind: "redirect", url: hosted.url });
		expect(createSession).toHaveBeenCalledTimes(1);
	});

	it("EMBEDDED：不導頁、也不在這裡建立 session（內嵌元件自己取 client secret）", async () => {
		const createSession = vi.fn().mockResolvedValue(hosted);
		await expect(
			startOnboarding(OnboardingSurface.EMBEDDED, { createSession }),
		).resolves.toEqual({ kind: "embedded" });
		expect(createSession).not.toHaveBeenCalled();
	});

	it("沒有設定介面或不認得的介面：丟錯，不猜預設", async () => {
		const createSession = vi.fn();
		await expect(
			startOnboarding(OnboardingSurface.UNSPECIFIED, { createSession }),
		).rejects.toThrow();
		await expect(
			startOnboarding(99 as OnboardingSurface, { createSession }),
		).rejects.toThrow();
		expect(createSession).not.toHaveBeenCalled();
	});

	it("HOSTED 卻拿到內嵌的 session：契約被違反，丟錯", async () => {
		await expect(
			startOnboarding(OnboardingSurface.HOSTED, {
				createSession: async () => embedded,
			}),
		).rejects.toThrow();
	});
});

describe("embeddedClientSecret", () => {
	it("每次都建立新的 session 並回傳 client secret", async () => {
		const createSession = vi.fn().mockResolvedValue(embedded);
		await expect(embeddedClientSecret(createSession)).resolves.toBe(
			"secret_123",
		);
		await embeddedClientSecret(createSession);
		expect(createSession).toHaveBeenCalledTimes(2);
	});

	it("拿到託管連結：契約被違反，丟錯", async () => {
		await expect(embeddedClientSecret(async () => hosted)).rejects.toThrow();
	});
});

describe("confirmPaymentReturn", () => {
	it("先重讀一次再讀：順序是 refresh、read，各只有一次，不輪詢", async () => {
		const calls: string[] = [];
		const refreshAccount = vi.fn(async () => {
			calls.push("refresh");
		});
		// 讀到的還是待設定：舊的輪詢會因為「還沒同步」再讀，這裡必須只讀一次。
		const readAccount = vi.fn(async () => {
			calls.push("read");
			return account(PaymentAccountStatus.ACTION_REQUIRED);
		});
		const result = await confirmPaymentReturn({ refreshAccount, readAccount });
		expect(calls).toEqual(["refresh", "read"]);
		expect(refreshAccount).toHaveBeenCalledTimes(1);
		expect(readAccount).toHaveBeenCalledTimes(1);
		expect(result?.status).toBe(PaymentAccountStatus.ACTION_REQUIRED);
	});

	it("重讀失敗：錯誤往上丟，不讀帳戶", async () => {
		const refreshAccount = vi.fn().mockRejectedValue(new Error("refresh boom"));
		const readAccount = vi.fn(async () => null);
		await expect(
			confirmPaymentReturn({ refreshAccount, readAccount }),
		).rejects.toThrow("refresh boom");
		expect(readAccount).not.toHaveBeenCalled();
	});

	it("讀取失敗：錯誤往上丟，由畫面顯示一般的錯誤", async () => {
		const refreshAccount = vi.fn(async () => {});
		const readAccount = vi.fn().mockRejectedValue(new Error("boom"));
		await expect(
			confirmPaymentReturn({ refreshAccount, readAccount }),
		).rejects.toThrow("boom");
	});
});

describe("handlePaymentParam", () => {
	const deps = () => ({
		refreshAccount: vi.fn(async () => {}),
		readAccount: vi.fn(async () => account(PaymentAccountStatus.UNDER_REVIEW)),
		createSession: vi.fn().mockResolvedValue(hosted),
		surface: OnboardingSurface.HOSTED as OnboardingSurface | undefined,
	});

	it("?payment=return：先重讀再讀 GetMyPaymentAccount，不建立新的 session", async () => {
		const d = deps();
		const outcome = await handlePaymentParam("return", d);
		expect(d.refreshAccount).toHaveBeenCalledTimes(1);
		expect(d.readAccount).toHaveBeenCalledTimes(1);
		expect(d.refreshAccount.mock.invocationCallOrder[0]).toBeLessThan(
			d.readAccount.mock.invocationCallOrder[0] as number,
		);
		expect(d.createSession).not.toHaveBeenCalled();
		expect(outcome).toMatchObject({ kind: "confirmed" });
	});

	it("?payment=refresh（HOSTED）：重新取得連結再導，不重讀帳戶", async () => {
		const d = deps();
		const outcome = await handlePaymentParam("refresh", d);
		expect(d.createSession).toHaveBeenCalledTimes(1);
		expect(d.refreshAccount).not.toHaveBeenCalled();
		expect(d.readAccount).not.toHaveBeenCalled();
		expect(outcome).toEqual({ kind: "redirect", url: hosted.url });
	});

	it("?payment=refresh 但還不知道介面：丟錯", async () => {
		const d = { ...deps(), surface: undefined };
		await expect(handlePaymentParam("refresh", d)).rejects.toThrow();
	});
});
