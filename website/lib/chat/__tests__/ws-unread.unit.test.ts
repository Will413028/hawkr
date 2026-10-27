import { describe, expect, it } from "vitest";
import { parseUnreadCount } from "../ws";

describe("parseUnreadCount", () => {
	it("number", () => expect(parseUnreadCount({ unreadCount: 7 })).toBe(7));
	it('coerces "7"', () =>
		expect(parseUnreadCount({ unreadCount: "7" })).toBe(7));
	it("missing -> 0", () => expect(parseUnreadCount({})).toBe(0));
	it("garbage -> 0", () =>
		expect(parseUnreadCount({ unreadCount: "abc" })).toBe(0));
	it("clamps negative to 0", () =>
		expect(parseUnreadCount({ unreadCount: -3 })).toBe(0));
	it("keeps 0 (mark-read)", () =>
		expect(parseUnreadCount({ unreadCount: 0 })).toBe(0));
});
