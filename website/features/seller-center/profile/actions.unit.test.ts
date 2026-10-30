import { Code, ConnectError } from "@connectrpc/connect";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	getSession: vi.fn(),
	revalidateTag: vi.fn(),
	authedTransport: vi.fn((_opts: unknown) => ({})),
	sessionCookieHeader: vi.fn(),
}));
vi.mock("@/lib/auth/session-cookie.server", () => ({
	sessionCookieHeader: mocks.sessionCookieHeader,
}));
vi.mock("@connectrpc/connect", async (importOriginal) => ({
	...(await importOriginal<typeof import("@connectrpc/connect")>()),
	createClient: () => ({ getSession: mocks.getSession }),
}));
vi.mock("@/lib/connect/transport", () => ({
	authedTransport: mocks.authedTransport,
}));
vi.mock("next/cache", () => ({ revalidateTag: mocks.revalidateTag }));

import { revalidateOwnSellerProfile } from "./actions";

beforeEach(() => {
	mocks.getSession.mockReset();
	mocks.revalidateTag.mockReset();
	mocks.authedTransport.mockClear();
	mocks.sessionCookieHeader.mockReset();
	mocks.sessionCookieHeader.mockResolvedValue("__Host-hk_session=opaque");
});

describe("revalidateOwnSellerProfile asks the monolith who the visitor is (read-only GetSession)", () => {
	it("expires both storefront tags for a live session", async () => {
		mocks.getSession.mockResolvedValue({ account: { id: "9", roles: [] } });
		await revalidateOwnSellerProfile({ accountId: "9", storeCode: "12" });
		expect(mocks.revalidateTag).toHaveBeenCalledTimes(2);
		// the visitor's cookie goes to the monolith through the transport (GET only there)
		expect(mocks.authedTransport).toHaveBeenCalledWith({
			locale: "zh-TW",
			cookie: "__Host-hk_session=opaque",
		});
	});

	it("a visitor with no session cookie is refused without asking the monolith", async () => {
		mocks.sessionCookieHeader.mockResolvedValue(null);
		await expect(
			revalidateOwnSellerProfile({ accountId: "9" }),
		).rejects.toThrow("登入狀態已失效");
		expect(mocks.getSession).not.toHaveBeenCalled();
		expect(mocks.revalidateTag).not.toHaveBeenCalled();
	});

	it("refuses an unauthenticated visitor and revalidates nothing", async () => {
		mocks.getSession.mockRejectedValue(
			new ConnectError("no session", Code.Unauthenticated),
		);
		await expect(
			revalidateOwnSellerProfile({ accountId: "9" }),
		).rejects.toThrow("登入狀態已失效");
		expect(mocks.revalidateTag).not.toHaveBeenCalled();
	});

	it("does not treat a monolith outage or an empty answer as a live session", async () => {
		mocks.getSession.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		await expect(
			revalidateOwnSellerProfile({ accountId: "9" }),
		).rejects.toThrow("登入狀態已失效");
		mocks.getSession.mockResolvedValue({});
		await expect(
			revalidateOwnSellerProfile({ accountId: "9" }),
		).rejects.toThrow("登入狀態已失效");
		expect(mocks.revalidateTag).not.toHaveBeenCalled();
	});

	it("still validates the ids once the session is live", async () => {
		mocks.getSession.mockResolvedValue({ account: { id: "9", roles: [] } });
		await expect(
			revalidateOwnSellerProfile({ accountId: "not-a-number" }),
		).rejects.toThrow("缺少可更新的商店識別碼");
	});
});
