import { describe, expect, it, vi } from "vitest";
import { createMembershipBatcher } from "./batch-membership";

describe("visible membership batching", () => {
	it("checks the mounted IDs once and keeps identities separate", async () => {
		const check = vi.fn(
			async (ids: string[]) => new Set(ids.filter((id) => id === "2")),
		);
		const batch = createMembershipBatcher(check);
		const results = await Promise.all([
			batch("buyer-1", "1"),
			batch("buyer-1", "2"),
			batch("buyer-1", "2"),
			batch("buyer-2", "3"),
		]);
		expect(results).toEqual([false, true, true, false]);
		expect(check).toHaveBeenCalledTimes(2);
		expect(check).toHaveBeenCalledWith(["1", "2"]);
		expect(check).toHaveBeenCalledWith(["3"]);
	});

	it("keeps each request within the 100-ID contract", async () => {
		const check = vi.fn(async (_ids: string[]) => new Set<string>());
		const batch = createMembershipBatcher(check);
		await Promise.all(
			Array.from({ length: 201 }, (_, id) => batch("buyer", String(id + 1))),
		);
		expect(check.mock.calls.map(([ids]) => ids.length)).toEqual([100, 100, 1]);
	});
});
