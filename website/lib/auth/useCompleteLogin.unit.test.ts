// @vitest-environment jsdom

import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	commitIdentity: vi.fn(),
	switchRole: vi.fn(),
	navigateDocument: vi.fn(),
}));

vi.mock("./context", () => ({
	useAuth: () => ({
		commitIdentity: mocks.commitIdentity,
		switchRole: mocks.switchRole,
	}),
}));
vi.mock("@/lib/navigation/observability", () => ({
	navigateDocument: mocks.navigateDocument,
}));

import { useCompleteLogin } from "./useCompleteLogin";

const identity = {
	accountId: "account-1",
	isSeller: true,
};

beforeEach(() => {
	mocks.switchRole.mockReset().mockReturnValue(true);
	mocks.commitIdentity.mockReset();
	mocks.navigateDocument.mockReset().mockReturnValue(true);
});

afterEach(cleanup);

describe("login document-navigation ownership", () => {
	it("preserves a product destination and its query/hash after applying the role", () => {
		const { result } = renderHook(useCompleteLogin);
		const destination = "/products/demo?source=login#details";

		act(() => result.current.completeLogin(destination, "buyer", identity));

		expect(mocks.switchRole).toHaveBeenCalledExactlyOnceWith("buyer");
		expect(mocks.navigateDocument).toHaveBeenCalledExactlyOnceWith(
			destination,
			expect.objectContaining({
				componentId: "login",
				actionId: "login_completed",
			}),
		);
		expect(mocks.switchRole.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.navigateDocument.mock.invocationCallOrder[0],
		);
	});

	it("accepts one completion when two completions arrive in the same tick", () => {
		const { result, rerender } = renderHook(useCompleteLogin);

		act(() => {
			result.current.completeLogin("/products/first", "buyer", identity);
			result.current.completeLogin(
				"/seller-center/dashboard",
				"seller",
				identity,
			);
		});
		rerender();
		act(() =>
			result.current.completeLogin("/products/later", "buyer", identity),
		);

		expect(mocks.switchRole).toHaveBeenCalledExactlyOnceWith("buyer");
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(1);
		expect(mocks.navigateDocument.mock.calls[0][0]).toBe("/products/first");
	});

	it("keeps the existing role boundary when a seller receives a buyer-only redirect", () => {
		const { result } = renderHook(useCompleteLogin);

		act(() =>
			result.current.completeLogin("/account?tab=orders", "seller", identity),
		);

		expect(mocks.switchRole).toHaveBeenCalledExactlyOnceWith("seller");
		expect(mocks.navigateDocument.mock.calls[0][0]).toBe(
			"/seller-center/dashboard",
		);
	});

	it("does not switch roles for a public destination with no selected role", () => {
		const { result } = renderHook(useCompleteLogin);

		act(() => result.current.completeLogin("/products/demo", null, identity));

		expect(mocks.switchRole).not.toHaveBeenCalled();
		expect(mocks.navigateDocument.mock.calls[0][0]).toBe("/products/demo");
	});

	it("releases the leaving guard if document navigation is refused, allowing retry", () => {
		mocks.navigateDocument.mockReturnValueOnce(false);
		const { result } = renderHook(useCompleteLogin);

		act(() =>
			result.current.completeLogin("/products/demo", "buyer", identity),
		);
		act(() =>
			result.current.completeLogin("/products/demo", "buyer", identity),
		);
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(2);

		act(() =>
			result.current.completeLogin("/products/demo", "buyer", identity),
		);
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(2);
	});
});
