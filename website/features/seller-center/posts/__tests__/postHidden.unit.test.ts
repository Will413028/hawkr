import { describe, expect, it } from "vitest";
import { isPostHidden } from "../postHidden";

describe("isPostHidden", () => {
	it("true when hiddenAt > 0", () => {
		expect(isPostHidden({ hiddenAt: 1720000000 })).toBe(true);
	});
	it("false when 0 / undefined", () => {
		expect(isPostHidden({ hiddenAt: 0 })).toBe(false);
		expect(isPostHidden({ hiddenAt: undefined })).toBe(false);
		expect(isPostHidden({})).toBe(false);
	});
});
