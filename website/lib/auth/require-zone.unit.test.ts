import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	verifySession: vi.fn(),
	headers: vi.fn(),
	redirect: vi.fn(),
}));
vi.mock("server-only", () => ({}));
vi.mock("./dal", () => ({ verifySession: mocks.verifySession }));
vi.mock("next/headers", () => ({ headers: mocks.headers }));
vi.mock("@/lib/intl/navigation", () => ({ redirect: mocks.redirect }));

import { requireZone } from "./require-zone";
import { RETURN_PATH_HEADER } from "./return-path";

function requestAt(path: string | null) {
	mocks.headers.mockResolvedValue(
		new Headers(path === null ? {} : { [RETURN_PATH_HEADER]: path }),
	);
}

describe("requireZone", () => {
	beforeEach(() => {
		mocks.verifySession.mockReset();
		mocks.headers.mockReset();
		mocks.redirect.mockReset();
	});

	it("redirects a guest to login with the URL the middleware forwarded", async () => {
		mocks.verifySession.mockResolvedValue({ status: "guest" });
		requestAt("/account/orders?tab=paid");
		await requireZone("buyer");
		expect(mocks.redirect).toHaveBeenCalledWith({
			href: "/login?role=buyer&redirect=%2Faccount%2Forders%3Ftab%3Dpaid",
			locale: "zh-TW",
		});
	});

	it("redirects a signed-in non-seller away from the seller zone", async () => {
		mocks.verifySession.mockResolvedValue({
			status: "signedIn",
			identity: { accountId: "7", isSeller: false },
		});
		requestAt("/seller-center/dashboard");
		await requireZone("seller");
		expect(mocks.redirect).toHaveBeenCalledWith({
			href: "/",
			locale: "zh-TW",
		});
	});

	it("admits a seller without redirecting", async () => {
		mocks.verifySession.mockResolvedValue({
			status: "signedIn",
			identity: { accountId: "8", isSeller: true },
		});
		requestAt("/seller-center/dashboard");
		expect(await requireZone("seller")).toEqual({
			kind: "allow",
			identity: { accountId: "8", isSeller: true },
		});
		expect(mocks.redirect).not.toHaveBeenCalled();
	});

	it("reports an unavailable session without redirecting", async () => {
		mocks.verifySession.mockResolvedValue({ status: "unavailable" });
		requestAt("/account");
		expect(await requireZone("buyer")).toEqual({ kind: "unavailable" });
		expect(mocks.redirect).not.toHaveBeenCalled();
	});
});
