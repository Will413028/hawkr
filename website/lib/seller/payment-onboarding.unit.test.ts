import { describe, expect, it, vi } from "vitest";
import type { OnboardingSession } from "./payment-account";
import {
	confirmPaymentReturn,
	embeddedClientSecret,
	paymentParamOf,
	startOnboarding,
} from "./payment-onboarding";

const hosted: OnboardingSession = {
	kind: "hosted",
	url: "https://connect.stripe.test/setup/abc",
};
const embedded: OnboardingSession = {
	kind: "embedded",
	provider: "stripe",
	clientSecret: "secret_123",
	publishableKey: "pk_test_dummy",
};

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
	it("伺服器回託管連結：整頁導到連結", async () => {
		const createSession = vi.fn().mockResolvedValue(hosted);
		await expect(startOnboarding({ createSession })).resolves.toEqual({
			kind: "redirect",
			url: hosted.url,
		});
		expect(createSession).toHaveBeenCalledTimes(1);
	});

	it("伺服器回內嵌：取得金流商與公開金鑰；第一次 client secret 用這個 session，之後每次是新的", async () => {
		const createSession = vi.fn().mockResolvedValue(embedded);
		const started = await startOnboarding({ createSession });
		if (started.kind !== "embedded") throw new Error("不是內嵌");
		expect(started.provider).toBe("stripe");
		expect(started.publishableKey).toBe("pk_test_dummy");
		expect(createSession).toHaveBeenCalledTimes(1);
		await expect(started.fetchClientSecret()).resolves.toBe("secret_123");
		expect(createSession).toHaveBeenCalledTimes(1);
		await expect(started.fetchClientSecret()).resolves.toBe("secret_123");
		expect(createSession).toHaveBeenCalledTimes(2);
	});

	it("session 不是兩種已知的形狀（例如較新的伺服器回了這個版本不認得的分支）：丟錯，不猜", async () => {
		await expect(
			startOnboarding({
				createSession: async () => ({ kind: "other" }) as never,
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
		const readAccount = vi.fn(async () => {
			calls.push("read");
		});
		await confirmPaymentReturn({ refreshAccount, readAccount });
		expect(calls).toEqual(["refresh", "read"]);
		expect(refreshAccount).toHaveBeenCalledTimes(1);
		expect(readAccount).toHaveBeenCalledTimes(1);
	});

	it("重讀失敗：不丟出，仍讀一次（快取不留舊的）", async () => {
		const refreshAccount = vi.fn().mockRejectedValue(new Error("refresh boom"));
		const readAccount = vi.fn(async () => {});
		await expect(
			confirmPaymentReturn({ refreshAccount, readAccount }),
		).resolves.toBeUndefined();
		expect(readAccount).toHaveBeenCalledTimes(1);
	});

	it("讀取失敗：錯誤往上丟", async () => {
		const refreshAccount = vi.fn(async () => {});
		const readAccount = vi.fn().mockRejectedValue(new Error("boom"));
		await expect(
			confirmPaymentReturn({ refreshAccount, readAccount }),
		).rejects.toThrow("boom");
	});
});
