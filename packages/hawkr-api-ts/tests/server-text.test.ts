import { describe, expect, it } from "vitest";
import { goTrimmedRuneCount, trimGoSpace } from "../src/text/server-text";

const GO_SPACES = [
	"\u0009",
	"\u000A",
	"\u000B",
	"\u000C",
	"\u000D",
	"\u0020",
	"\u0085",
	"\u00A0",
	"\u1680",
	"\u2000",
	"\u2001",
	"\u2002",
	"\u2003",
	"\u2004",
	"\u2005",
	"\u2006",
	"\u2007",
	"\u2008",
	"\u2009",
	"\u200A",
	"\u2028",
	"\u2029",
	"\u202F",
	"\u205F",
	"\u3000",
];

describe("trimGoSpace", () => {
	it("修剪 Go unicode.IsSpace 的每一種空白", () => {
		for (const space of GO_SPACES) {
			expect(trimGoSpace(`${space}理由${space}`), JSON.stringify(space)).toBe(
				"理由",
			);
		}
	});

	it("不修剪 Go 不當成空白的字元（U+FEFF、U+200B、U+180E）", () => {
		for (const ch of ["\uFEFF", "\u200B", "\u180E"]) {
			expect(trimGoSpace(`${ch}理由${ch}`), JSON.stringify(ch)).toBe(
				`${ch}理由${ch}`,
			);
		}
	});

	it("只修剪兩端，中間的空白保留", () => {
		expect(trimGoSpace("\u0020\u0020a\u0020\u3000\u0020b\u0020\u0020")).toBe(
			"a\u0020\u3000\u0020b",
		);
	});

	it("全是空白得到空字串", () => {
		expect(trimGoSpace("\u0020\u3000\u000A")).toBe("");
		expect(trimGoSpace("")).toBe("");
	});

	it("不拆開 surrogate pair", () => {
		expect(trimGoSpace("\u0020\u{1F600}\u0020")).toBe("\u{1F600}");
	});
});

describe("goTrimmedRuneCount", () => {
	it("先修剪再以 code point 計，不是 UTF-16 code unit", () => {
		expect(goTrimmedRuneCount("abc")).toBe(3);
		expect(goTrimmedRuneCount("申訴")).toBe(2);
		expect(goTrimmedRuneCount("\u{1F600}")).toBe(1);
		expect("\u{1F600}".length).toBe(2);
		expect(goTrimmedRuneCount("a\u{1F600}b")).toBe(3);
		expect(goTrimmedRuneCount("\u0020\u3000\u{1F600}\u0020")).toBe(1);
		expect(goTrimmedRuneCount("")).toBe(0);
	});

	it("全是空白算 0；U+FEFF 與 U+200B 不是空白，各算 1", () => {
		for (const space of GO_SPACES) {
			expect(goTrimmedRuneCount(space.repeat(3)), JSON.stringify(space)).toBe(
				0,
			);
		}
		expect(goTrimmedRuneCount("\uFEFF")).toBe(1);
		expect(goTrimmedRuneCount("\u200B理\u200B")).toBe(3);
	});
});
