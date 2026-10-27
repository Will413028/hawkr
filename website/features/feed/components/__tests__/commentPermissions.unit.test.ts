import { describe, expect, it } from "vitest";
import { canDeleteComment, canEditComment } from "../commentPermissions";

describe("canEditComment", () => {
	it("is true only for the viewer's own comment", () => {
		expect(canEditComment(true)).toBe(true);
		expect(canEditComment(false)).toBe(false);
	});
});

describe("canDeleteComment", () => {
	it("post owner (canModerate) may delete any comment, own or not", () => {
		expect(canDeleteComment(false, true)).toBe(true);
		expect(canDeleteComment(true, true)).toBe(true);
	});

	it("non-moderator may delete only their own comment", () => {
		expect(canDeleteComment(true, false)).toBe(true);
		expect(canDeleteComment(false, false)).toBe(false);
	});

	it("a post owner cannot rewrite someone else's comment", () => {
		expect(canDeleteComment(false, true)).toBe(true);
		expect(canEditComment(false)).toBe(false);
	});
});
