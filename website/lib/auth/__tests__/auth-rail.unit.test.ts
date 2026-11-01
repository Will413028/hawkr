import { describe, expect, it } from "vitest";
import { type AuthRailInput, resolveAuthRailMode } from "../auth-rail";

const base: AuthRailInput = {
	status: "unknown",
	isLoggedIn: false,
	activeRole: null,
};

describe("resolveAuthRailMode", () => {
	it("unknown (the server's answer has not arrived) → skeleton, neither a 登入 button nor an avatar", () => {
		expect(resolveAuthRailMode({ ...base, status: "unknown" })).toBe(
			"skeleton",
		);
	});

	it("an unknown status outranks a stale identity", () => {
		expect(
			resolveAuthRailMode({
				...base,
				isLoggedIn: true,
				activeRole: "buyer",
			}),
		).toBe("skeleton");
	});

	it("settled guest → guest rail", () => {
		expect(resolveAuthRailMode({ ...base, status: "guest" })).toBe("guest");
	});

	it("settled buyer session → buyer rail", () => {
		expect(
			resolveAuthRailMode({
				status: "signedIn",
				isLoggedIn: true,
				activeRole: "buyer",
			}),
		).toBe("buyer");
	});

	it("settled seller session → seller rail", () => {
		expect(
			resolveAuthRailMode({
				status: "signedIn",
				isLoggedIn: true,
				activeRole: "seller",
			}),
		).toBe("seller");
	});

	it("logged-in but role unresolved → seller rail (preserves prior !isBuyer branch)", () => {
		expect(
			resolveAuthRailMode({
				status: "signedIn",
				isLoggedIn: true,
				activeRole: null,
			}),
		).toBe("seller");
	});

	it("unavailable (B-199 blip) → hold skeleton, never claim guest", () => {
		expect(resolveAuthRailMode({ ...base, status: "unavailable" })).toBe(
			"skeleton",
		);
	});

	it("an unavailable answer outranks a logged-in identity from before it", () => {
		expect(
			resolveAuthRailMode({
				status: "unavailable",
				isLoggedIn: true,
				activeRole: "buyer",
			}),
		).toBe("skeleton");
	});
});
