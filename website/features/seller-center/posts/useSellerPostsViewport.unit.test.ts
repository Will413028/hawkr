// @vitest-environment jsdom

import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	SELLER_POSTS_MOBILE_QUERY,
	useSellerPostsViewport,
} from "./useSellerPostsViewport";

function installMatchMedia(initialMatches: boolean) {
	let matches = initialMatches;
	const listeners = new Set<EventListener>();
	const mediaQuery = {
		get matches() {
			return matches;
		},
		media: SELLER_POSTS_MOBILE_QUERY,
		addEventListener: vi.fn((_type: string, listener: EventListener) => {
			listeners.add(listener);
		}),
		removeEventListener: vi.fn((_type: string, listener: EventListener) => {
			listeners.delete(listener);
		}),
	} as unknown as MediaQueryList;

	vi.stubGlobal(
		"matchMedia",
		vi.fn(() => mediaQuery),
	);

	return {
		setMatches(nextMatches: boolean) {
			matches = nextMatches;
			for (const listener of listeners) listener(new Event("change"));
		},
	};
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe("useSellerPostsViewport", () => {
	it("tracks the CSS breakpoint used by the seller post layouts", () => {
		const media = installMatchMedia(false);
		const { result } = renderHook(() => useSellerPostsViewport());

		expect(result.current).toBe("desktop");

		act(() => media.setMatches(true));

		expect(result.current).toBe("mobile");
		expect(window.matchMedia).toHaveBeenCalledWith(SELLER_POSTS_MOBILE_QUERY);
	});
});
