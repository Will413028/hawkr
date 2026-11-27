import { Code } from "@connectrpc/connect";
import { createTranslator } from "use-intl/core";
import { describe, expect, it } from "vitest";
import { createUiText, type Translate } from "../src/format";
import { commonError, commonUnknown } from "../src/keys.gen";
import en from "../src/messages/en.gen.json";
import zhTW from "../src/messages/zh-TW.gen.json";
import { defaultErrorTextZhTW, formatStaff } from "../src/zh-tw.gen";
import golden from "./golden.gen.json";
import { type GoldenCase, goldenMessage } from "./golden-calls.gen";

const cases = golden as GoldenCase[];

function translatorFor(locale: string): Translate {
	const messages = locale === "en" ? en : zhTW;
	const t = createTranslator({
		locale,
		messages,
		onError: (error) => {
			throw error;
		},
	});
	return (key, values) =>
		(t as (key: string, values?: Record<string, string | number>) => string)(
			key,
			values,
		);
}

describe("共用 golden", () => {
	const ui = {
		"zh-TW": createUiText(translatorFor("zh-TW")),
		en: createUiText(translatorFor("en")),
	} as const;

	it("每一筆都與參考格式器一致", () => {
		const failures: string[] = [];
		const staffKeys = new Set(
			cases.filter((c) => c.audience === "staff").map((c) => c.key),
		);
		for (const c of cases) {
			const msg = goldenMessage(c);
			let got: string | null;
			if (msg === undefined) {
				got = null;
			} else if (c.audience === "staff") {
				got = formatStaff(msg);
			} else {
				got = ui[c.locale as "zh-TW" | "en"](msg);
				if (c.locale === "zh-TW" && !staffKeys.has(c.key)) {
					const viaDefaultTable = formatStaff(msg);
					if (viaDefaultTable !== c.want) {
						failures.push(
							`${c.key} [default table] ${JSON.stringify(c.params)}: got ${JSON.stringify(viaDefaultTable)}, want ${JSON.stringify(c.want)}`,
						);
					}
				}
			}
			if (got !== c.want) {
				failures.push(
					`${c.key} [${c.audience} ${c.locale}] ${JSON.stringify(c.params)}: got ${JSON.stringify(got)}, want ${JSON.stringify(c.want)}`,
				);
			}
		}
		expect(failures).toEqual([]);
	});

	it("golden 涵蓋所有函式、受眾與語系，且含「沒有文字」的案例", () => {
		const seen = new Set(cases.map((c) => `${c.audience}/${c.locale}`));
		expect([...seen].sort()).toEqual([
			"default/en",
			"default/zh-TW",
			"staff/zh-TW",
		]);
		const fns = new Map<string, number>();
		for (const c of cases) {
			fns.set(c.fn, (fns.get(c.fn) ?? 0) + (c.want === null ? 0 : 1));
		}
		for (const [fn, n] of fns) {
			expect(n, `${fn} 沒有任何一筆有文字`).toBeGreaterThan(0);
		}
		expect(cases.some((c) => c.want === null)).toBe(true);
	});

	it("en 缺字時輸出 zh-TW 的文字", () => {
		const zh = cases.find(
			(c) =>
				c.key === "seller.MeasureKind.MEASURE_KIND_WARNING" &&
				c.locale === "zh-TW",
		);
		const eng = cases.find(
			(c) =>
				c.key === "seller.MeasureKind.MEASURE_KIND_WARNING" &&
				c.locale === "en",
		);
		expect(zh?.want).toBe("警告");
		expect(eng?.want).toBe("警告");
	});

	it("整數參數輸出十進位 ASCII、不分組", () => {
		const c = cases.find(
			(x) =>
				x.fn === "seller.Measure.rejection_cooldown_warning" &&
				x.params?.days === 1234 &&
				x.locale === "zh-TW",
		);
		expect(c?.want).toContain(" 1234 ");
	});
});

describe("Connect 預設文字", () => {
	it("defaultErrorTextZhTW 與目錄一致，Canceled 沒有文字", () => {
		expect(commonError(Code.Canceled)).toBeUndefined();
		for (const code of [
			Code.Unknown,
			Code.InvalidArgument,
			Code.DeadlineExceeded,
			Code.NotFound,
			Code.AlreadyExists,
			Code.PermissionDenied,
			Code.ResourceExhausted,
			Code.FailedPrecondition,
			Code.Aborted,
			Code.OutOfRange,
			Code.Unimplemented,
			Code.Internal,
			Code.Unavailable,
			Code.DataLoss,
			Code.Unauthenticated,
		] as const) {
			const msg = commonError(code);
			expect(msg, Code[code]).toBeDefined();
			expect(defaultErrorTextZhTW(code)).toBe(
				formatStaff(msg ?? commonUnknown()),
			);
		}
		expect(defaultErrorTextZhTW(Code.InvalidArgument)).toBe("參數錯誤");
		expect(defaultErrorTextZhTW(Code.Internal)).toBe("系統錯誤");
	});
});
