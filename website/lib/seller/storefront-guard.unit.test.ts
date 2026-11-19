import { describe, expect, it, vi } from "vitest";
import { resolveStorefrontRequest } from "./storefront-guard";

const present = { kind: "present", storeId: "9" } as const;

describe("resolveStorefrontRequest", () => {
	it("passes a path that is not a storefront without asking", async () => {
		const lookup = vi.fn();
		await expect(resolveStorefrontRequest("/about", lookup)).resolves.toEqual({
			kind: "pass",
		});
		expect(lookup).not.toHaveBeenCalled();
	});

	it("passes a present store, on every tab", async () => {
		const lookup = vi.fn().mockResolvedValue(present);
		for (const tail of ["", "/posts", "/reviews", "/messages", "/card"]) {
			await expect(
				resolveStorefrontRequest(`/sellers/2662192214${tail}`, lookup),
			).resolves.toEqual({ kind: "pass" });
		}
		expect(lookup).toHaveBeenCalledWith("2662192214");
	});

	it("answers 404 when GetStore says the store is absent", async () => {
		const lookup = vi.fn().mockResolvedValue({ kind: "absent" });
		await expect(
			resolveStorefrontRequest("/sellers/2662192214/posts", lookup),
		).resolves.toEqual({ kind: "not-found" });
	});

	it("fails open when there is no answer", async () => {
		const lookup = vi.fn().mockResolvedValue({ kind: "unknown" });
		await expect(
			resolveStorefrontRequest("/sellers/2662192214", lookup),
		).resolves.toEqual({ kind: "pass" });
	});

	it("answers 404 for a segment outside stores_store_code_check without asking", async () => {
		const lookup = vi.fn();
		for (const seg of ["0", "10000000000", "abc", "7487768729113814280"]) {
			await expect(
				resolveStorefrontRequest(`/sellers/${seg}`, lookup),
			).resolves.toEqual({ kind: "not-found" });
		}
		expect(lookup).not.toHaveBeenCalled();
	});

	it("does not redirect a store_id to a store_code", async () => {
		const lookup = vi.fn().mockResolvedValue({ kind: "absent" });
		await expect(
			resolveStorefrontRequest("/sellers/2636840881", lookup),
		).resolves.toEqual({ kind: "not-found" });
	});
});
