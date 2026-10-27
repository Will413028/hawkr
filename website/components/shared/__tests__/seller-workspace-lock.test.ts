// @vitest-environment jsdom

import { cleanup, render as renderWithoutIntl } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	pathname: "/",
	replace: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	usePathname: () => mocks.pathname,
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "seller",
		hydrated: true,
	}),
}));
vi.mock("@/lib/auth/redirect", async (importActual) => {
	const actual = await importActual<typeof import("@/lib/auth/redirect")>();
	return {
		...actual,
		requiredAccessForPath: (pathname: string) =>
			pathname === "/account-level-page"
				? "account"
				: actual.requiredAccessForPath(pathname),
	};
});
vi.mock("@/lib/auth/storage", () => ({ getActiveRole: () => "seller" }));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ replace: mocks.replace }),
}));

import SellerWorkspaceLock from "@/components/shared/SellerWorkspaceLock";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

function visit(pathname: string) {
	mocks.pathname = pathname;
	render(React.createElement(SellerWorkspaceLock));
}

describe("SellerWorkspaceLock 與帳號層級的措施與申覆頁", () => {
	beforeEach(() => mocks.replace.mockReset());
	afterEach(cleanup);

	it.each([
		"/seller/measures",
		"/seller/measures/m1",
		"/seller/measures/m1/appeal",
		"/seller/measures/m1/remediation",
	])("seller 視角進 %s 不會被導回工作區", (pathname) => {
		visit(pathname);
		expect(mocks.replace).not.toHaveBeenCalled();
	});

	it("seller 視角進存取類別不屬於任何工作區的頁面，不看路徑前綴", () => {
		visit("/account-level-page");
		expect(mocks.replace).not.toHaveBeenCalled();
	});

	it("seller 視角進買家頁面仍被導回工作區", () => {
		visit("/account/orders");
		expect(mocks.replace).toHaveBeenCalledWith("/seller-center/dashboard");
	});
});
