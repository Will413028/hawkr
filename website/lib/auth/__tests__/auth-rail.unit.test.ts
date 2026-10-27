import { describe, expect, it } from "vitest";
import { type AuthRailInput, resolveAuthRailMode } from "../auth-rail";

const base: AuthRailInput = {
	authReady: false,
	isLoggedIn: false,
	activeRole: null,
	sessionUnavailable: false,
};

describe("resolveAuthRailMode", () => {
	it("before hydration → guest rail (登入 clickable, the bug)", () => {
		expect(resolveAuthRailMode({ ...base, authReady: false })).toBe("guest");
	});

	it("stale cookie: probe says logged-out → guest", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				isLoggedIn: false,
			}),
		).toBe("guest");
	});

	it("settled buyer session → buyer rail", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				isLoggedIn: true,
				activeRole: "buyer",
			}),
		).toBe("buyer");
	});

	it("settled seller session → seller rail", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				isLoggedIn: true,
				activeRole: "seller",
			}),
		).toBe("seller");
	});

	it("logged-in but role unresolved → seller rail (preserves prior !isBuyer branch)", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				isLoggedIn: true,
				activeRole: null,
			}),
		).toBe("seller");
	});

	it("settled but unavailable (B-199 blip) → hold skeleton, never claim guest", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				sessionUnavailable: true,
			}),
		).toBe("skeleton");
	});

	it("an unavailable probe outranks a logged-in identity from before it", () => {
		expect(
			resolveAuthRailMode({
				...base,
				authReady: true,
				isLoggedIn: true,
				activeRole: "buyer",
				sessionUnavailable: true,
			}),
		).toBe("skeleton");
	});
});
