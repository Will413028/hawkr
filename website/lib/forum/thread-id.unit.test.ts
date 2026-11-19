import { describe, expect, it } from "vitest";
import { canonicalThreadId } from "./thread-id";

describe("canonicalThreadId", () => {
	it("lowercases a UUID", () => {
		expect(canonicalThreadId("0A1B2C3D-0000-4000-8000-00000000ABCD")).toBe(
			"0a1b2c3d-0000-4000-8000-00000000abcd",
		);
	});
	it.each(["", "t1", "123", "0a1b2c3d00004000800000000000abcd"])(
		"rejects %j",
		(raw) => {
			expect(canonicalThreadId(raw)).toBeNull();
		},
	);
});
