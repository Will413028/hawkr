import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { shouldTeardownPrivateCache, teardownPrivateCache } from "./teardown";

const HOME_SNAPSHOT_KEY = ["home", "snapshot"] as const;

describe("shouldTeardownPrivateCache — identity transition table", () => {
	it("does NOT tear down on the first identity landing (guest/none → account) [#281]", () => {
		expect(shouldTeardownPrivateCache(undefined, "123")).toBe(false);
	});

	it("does NOT tear down on the same-account backfill event (account → same account) [#259]", () => {
		expect(shouldTeardownPrivateCache("123", "123")).toBe(false);
	});

	it("tears down on a real account switch (A → B)", () => {
		expect(shouldTeardownPrivateCache("123", "456")).toBe(true);
	});

	it("tears down on logout (account → guest)", () => {
		expect(shouldTeardownPrivateCache("123", undefined)).toBe(true);
	});

	it("regression guard: the pre-#281 bare inequality wrongly fired on first landing", () => {
		// The old context.tsx used `next?.accountId !== accountIdRef.current` with
		// accountIdRef starting `undefined` — so a cold-loaded logged-in session's
		// first landing computed `123 !== undefined === true` and purged the
		// in-flight home.snapshot, locking the content skeleton.
		const preFixDecision = 123 !== undefined;
		expect(preFixDecision).toBe(true); // the bug
		expect(shouldTeardownPrivateCache(undefined, "123")).toBe(false); // the fix
	});
});

describe("teardownPrivateCache — effect on the public home.snapshot query", () => {
	function seedHomeSnapshot(qc: QueryClient) {
		qc.setQueryData(HOME_SNAPSHOT_KEY, { editorRecommendedProducts: [1] });
	}

	it("first landing keeps the in-flight home.snapshot — no skeleton lock [#281]", async () => {
		const qc = new QueryClient();
		seedHomeSnapshot(qc);
		// Cold-load logged-in: guest/none → account. The decision must be false, so
		// the public snapshot survives and the homepage renders instead of the
		// skeleton locking forever.
		if (shouldTeardownPrivateCache(undefined, "123")) {
			await teardownPrivateCache(qc);
		}
		expect(qc.getQueryData(HOME_SNAPSHOT_KEY)).toEqual({
			editorRecommendedProducts: [1],
		});
	});

	it("real account switch drops the previous viewer's home.snapshot — leak prevention intact", async () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
		const qc = new QueryClient();
		seedHomeSnapshot(qc);
		if (shouldTeardownPrivateCache("123", "456")) {
			await teardownPrivateCache(qc);
		}
		expect(qc.getQueryData(HOME_SNAPSHOT_KEY)).toBeUndefined();
		expect(warn).toHaveBeenCalledWith(
			"[auth] private cache purged on identity change",
			expect.objectContaining({ removedNamespaces: ["home"] }),
		);
		warn.mockRestore();
	});

	it("keeps the identity-free products cache across a teardown", async () => {
		vi.spyOn(console, "warn").mockImplementation(() => {});
		const qc = new QueryClient();
		qc.setQueryData(["products", "list"], { records: [1] });
		qc.setQueryData(HOME_SNAPSHOT_KEY, { x: 1 });
		await teardownPrivateCache(qc);
		expect(qc.getQueryData(["products", "list"])).toEqual({ records: [1] });
		expect(qc.getQueryData(HOME_SNAPSHOT_KEY)).toBeUndefined();
		vi.restoreAllMocks();
	});
});
