import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/pricebook/repo", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/pricebook/repo")>();
	return { ...actual, fetchCategoryResolution: vi.fn() };
});
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "seller",
		identity: { accountId: 1 },
	}),
}));

// This hook wraps useQuery; without React Testing Library this repo has no
// way to mount it, so this test only proves the gating logic by calling the
// hook's exported pure helper — see useCropCatalogSearch.unit.test.ts (now
// removed) for the established precedent.
import { isCategoryResolutionEnabled } from "./useCategoryResolution";

describe("isCategoryResolutionEnabled", () => {
	it("is enabled for a logged-in seller", () => {
		expect(
			isCategoryResolutionEnabled({ isLoggedIn: true, activeRole: "seller" }),
		).toBe(true);
	});

	it("is disabled when not logged in", () => {
		expect(
			isCategoryResolutionEnabled({ isLoggedIn: false, activeRole: "seller" }),
		).toBe(false);
	});

	it("is disabled for a non-seller role", () => {
		expect(
			isCategoryResolutionEnabled({ isLoggedIn: true, activeRole: "buyer" }),
		).toBe(false);
	});
});
