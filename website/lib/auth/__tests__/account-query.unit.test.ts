// @vitest-environment jsdom

import {
	QueryClient,
	QueryClientProvider,
	useQuery,
} from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import { accountQueryFn, accountQueryPending } from "../account-query";
import type { AuthStatus } from "../session-state";

function setup(status: AuthStatus) {
	const fn = vi.fn(async () => "data");
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
	const hook = renderHook(
		() =>
			useQuery({
				queryKey: ["account", status],
				queryFn: accountQueryFn(status, fn),
			}),
		{ wrapper },
	);
	return { fn, hook };
}

describe("accountQueryFn", () => {
	it.each([
		"unknown",
		"guest",
		"unavailable",
	] as const)("%s: no request, and the query stays pending (never an empty success)", async (status) => {
		const { fn, hook } = setup(status);
		await new Promise((done) => setTimeout(done, 20));
		expect(fn).not.toHaveBeenCalled();
		expect(hook.result.current.isPending).toBe(true);
		expect(hook.result.current.data).toBeUndefined();
	});

	it("signedIn: fetches", async () => {
		const { fn, hook } = setup("signedIn");
		await waitFor(() => expect(hook.result.current.data).toBe("data"));
		expect(fn).toHaveBeenCalledTimes(1);
	});
});

describe("accountQueryPending", () => {
	it("unknown waits; guest and unavailable have nothing to load; signedIn follows the query", () => {
		expect(accountQueryPending("unknown", false)).toBe(true);
		expect(accountQueryPending("guest", true)).toBe(false);
		expect(accountQueryPending("unavailable", true)).toBe(false);
		expect(accountQueryPending("signedIn", true)).toBe(true);
		expect(accountQueryPending("signedIn", false)).toBe(false);
	});
});
