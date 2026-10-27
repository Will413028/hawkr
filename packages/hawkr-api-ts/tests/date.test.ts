import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { DateSchema } from "../src/gen/google/type/date_pb";
import { formatDate, parseDate } from "../src/proto/date";

describe("parseDate", () => {
	it("合法日期", () => {
		expect(parseDate("1990-05-07")).toEqual({ year: 1990, month: 5, day: 7 });
	});

	it("閏日只在閏年存在", () => {
		expect(parseDate("2024-02-29")).toBeDefined();
		expect(parseDate("2023-02-29")).toBeUndefined();
	});

	it("不存在的月與日", () => {
		expect(parseDate("2023-13-01")).toBeUndefined();
		expect(parseDate("2023-04-31")).toBeUndefined();
		expect(parseDate("2023-00-10")).toBeUndefined();
		expect(parseDate("2023-01-00")).toBeUndefined();
	});

	it("1 到 99 年不被 Date 當成 19xx 年", () => {
		expect(parseDate("0050-05-07")).toEqual({ year: 50, month: 5, day: 7 });
	});

	it("年份 0000 是 google.type.Date 的「沒有年份」，拒絕", () => {
		expect(parseDate("0000-01-01")).toBeUndefined();
		expect(parseDate("0001-01-01")).toEqual({ year: 1, month: 1, day: 1 });
		expect(parseDate("9999-12-31")).toEqual({ year: 9999, month: 12, day: 31 });
	});

	it.each([
		"",
		"1990-5-7",
		"1990/05/07",
		"19900507",
		" 1990-05-07",
		"1990-05-07 ",
		"1990-05-07\n",
		"abcd-ef-gh",
	])("格式不符一律 undefined：%j", (value) => {
		expect(parseDate(value)).toBeUndefined();
	});
});

describe("formatDate", () => {
	it("補零成 YYYY-MM-DD", () => {
		expect(formatDate({ year: 990, month: 5, day: 7 })).toBe("0990-05-07");
		expect(formatDate({ year: 2023, month: 12, day: 31 })).toBe("2023-12-31");
	});

	it("undefined 與任何欄位為 0 回空字串", () => {
		expect(formatDate(undefined)).toBe("");
		expect(formatDate(create(DateSchema))).toBe("");
		expect(formatDate({ year: 0, month: 5, day: 7 })).toBe("");
		expect(formatDate({ year: 1990, month: 0, day: 7 })).toBe("");
		expect(formatDate({ year: 1990, month: 5, day: 0 })).toBe("");
	});

	it("與 parseDate 往返", () => {
		expect(formatDate(parseDate("2024-02-29"))).toBe("2024-02-29");
	});

	it("接受生成的 google.type.Date", () => {
		expect(
			formatDate(create(DateSchema, { year: 2024, month: 2, day: 29 })),
		).toBe("2024-02-29");
	});
});
