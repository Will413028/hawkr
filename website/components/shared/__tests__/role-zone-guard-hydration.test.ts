import { describe, expect, it } from "vitest";
import {
	roleZoneClientSnapshot,
	roleZoneFallbackPath,
	roleZoneServerSnapshot,
	shouldAdmitRoleZone,
} from "@/components/shared/RoleZoneGuard";

const resolvedMatchingSession = {
	hydrated: true,
	sessionSettled: true,
	isLoggedIn: true,
	ownsRequiredRole: true,
	ready: false,
	roleMatches: true,
};

describe("RoleZoneGuard hydration admission", () => {
	it("returns mismatched single-role sessions to their own workspace", () => {
		expect(roleZoneFallbackPath("seller", "seller")).toBe(
			"/seller-center/dashboard",
		);
		expect(roleZoneFallbackPath("buyer", "buyer")).toBe("/");
		expect(roleZoneFallbackPath(null, null)).toBe("/");
	});

	it("keeps SSR and the first hydration render null even if auth already settled", () => {
		expect(roleZoneServerSnapshot()).toBe(false);

		for (const phase of ["SSR", "first hydration render"]) {
			expect(
				shouldAdmitRoleZone({
					...resolvedMatchingSession,
					hydrationSafeClientRender: roleZoneServerSnapshot(),
				}),
				phase,
			).toBe(false);
		}
	});

	it("admits immediately after hydration and on a client-only mount", () => {
		expect(roleZoneClientSnapshot()).toBe(true);

		for (const phase of ["post-hydration render", "client-only mount"]) {
			expect(
				shouldAdmitRoleZone({
					...resolvedMatchingSession,
					hydrationSafeClientRender: roleZoneClientSnapshot(),
				}),
				phase,
			).toBe(true);
		}
	});

	it("still rejects a client render without a complete matching session", () => {
		expect(
			shouldAdmitRoleZone({
				...resolvedMatchingSession,
				hydrationSafeClientRender: roleZoneClientSnapshot(),
				sessionSettled: false,
			}),
		).toBe(false);
	});
});
