import { describe, expect, it } from "vitest";
import { errorMessage } from "./queryError";

describe("errorMessage", () => {
	it("returns null when there is no error", () => {
		expect(errorMessage(null, "載入失敗")).toBeNull();
		expect(errorMessage(undefined, "載入失敗")).toBeNull();
	});

	it("returns the Error's message when error is an Error instance", () => {
		expect(errorMessage(new Error("網路逾時"), "載入失敗")).toBe("網路逾時");
	});

	it("returns the fallback when error is truthy but not an Error instance", () => {
		expect(errorMessage("some string error", "載入失敗")).toBe("載入失敗");
		expect(errorMessage({ code: 500 }, "載入失敗")).toBe("載入失敗");
	});
});
