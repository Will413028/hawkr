// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const self = vi.hoisted(() => ({
	value: {} as Record<string, unknown>,
}));
vi.mock("./useSellerSelf", () => ({ useSellerSelf: () => self.value }));

import { useCrmStoreId } from "./useCrmStoreId";

describe("useCrmStoreId", () => {
	it("exposes a store read failure instead of dropping it", () => {
		const error = new ConnectError("x", Code.Unavailable);
		self.value = { data: null, loading: false, error };
		const { result } = renderHook(() => useCrmStoreId());
		expect(result.current.error).toBe(error);
		expect(result.current.storeId).toBeUndefined();
	});

	it("treats a seller with no store yet (not_found) as no error", () => {
		self.value = {
			data: null,
			loading: false,
			error: new ConnectError("x", Code.NotFound),
		};
		const { result } = renderHook(() => useCrmStoreId());
		expect(result.current.error).toBeNull();
	});
});
