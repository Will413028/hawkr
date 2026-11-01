// @vitest-environment jsdom

import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({
	status: "unknown" as string,
	isLoggedIn: false,
	activeRole: null as "buyer" | "seller" | null,
}));

vi.mock("../context", () => ({ useAuth: () => auth }));

import { useAuthGate } from "../useAuthGate";

beforeEach(() => {
	auth.status = "unknown";
	auth.isLoggedIn = false;
	auth.activeRole = null;
});
afterEach(cleanup);

describe("useAuthGate", () => {
	it.each([
		"unknown",
		"unavailable",
	])("%s: parks the action, no guest flow", (status) => {
		auth.status = status;
		const onGuest = vi.fn();
		const action = vi.fn();
		const { result } = renderHook(() => useAuthGate(onGuest));
		act(() => result.current.authGate(action));
		expect(result.current.authWaiting).toBe(true);
		expect(onGuest).not.toHaveBeenCalled();
		expect(action).not.toHaveBeenCalled();
	});

	it("a parked action replays when the session becomes signedIn", () => {
		auth.status = "unavailable";
		const onGuest = vi.fn();
		const action = vi.fn();
		const { result, rerender } = renderHook(() => useAuthGate(onGuest));
		act(() => result.current.authGate(action));
		auth.status = "signedIn";
		auth.isLoggedIn = true;
		auth.activeRole = "buyer";
		rerender();
		expect(action).toHaveBeenCalledWith({ activeRole: "buyer" });
		expect(onGuest).not.toHaveBeenCalled();
	});

	it("a parked action opens the guest flow only once the session is guest", () => {
		auth.status = "unavailable";
		const onGuest = vi.fn();
		const { result, rerender } = renderHook(() => useAuthGate(onGuest));
		act(() => result.current.authGate(vi.fn()));
		auth.status = "guest";
		rerender();
		expect(onGuest).toHaveBeenCalledTimes(1);
	});

	it("guest: onGuest immediately", () => {
		auth.status = "guest";
		const onGuest = vi.fn();
		const { result } = renderHook(() => useAuthGate(onGuest));
		act(() => result.current.authGate(vi.fn()));
		expect(onGuest).toHaveBeenCalledTimes(1);
	});
});
