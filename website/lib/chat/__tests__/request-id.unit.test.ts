import { afterEach, describe, expect, it, vi } from "vitest";
import { newRequestId } from "../request-id";

const UUID4 =
	/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

afterEach(() => vi.unstubAllGlobals());

describe("newRequestId", () => {
	it("is a UUIDv4 and differs per call", () => {
		const a = newRequestId();
		expect(a).toMatch(UUID4);
		expect(newRequestId()).not.toBe(a);
	});

	it("still produces a valid UUIDv4 where crypto.randomUUID is unavailable (plain-http dev origin)", () => {
		vi.stubGlobal("crypto", {
			getRandomValues: (a: Uint8Array) => a.fill(255),
		});
		expect(newRequestId()).toMatch(UUID4);
	});
});
