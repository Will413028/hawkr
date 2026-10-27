import {
	MeasureKind,
	type MeasureState,
	RiskGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { createTranslator } from "use-intl/core";
import { describe, expect, it } from "vitest";
import manifest from "../src/enum-manifest.gen.json";
import { createUiText, formatUiMessage, type Translate } from "../src/format";
import {
	commonUnknown,
	sellerKYCFlowStatusAction,
	sellerMeasureKind,
	sellerMeasureRejectionCooldownWarning,
	sellerMeasureState,
	sellerMeasureTitleRisk,
} from "../src/keys.gen";
import type { UiMessage } from "../src/message";
import en from "../src/messages/en.gen.json";
import zhTW from "../src/messages/zh-TW.gen.json";
import { formatStaff } from "../src/zh-tw.gen";

function translator(locale: "zh-TW" | "en"): Translate {
	const t = createTranslator({
		locale,
		messages: locale === "en" ? en : zhTW,
		onError: (error) => {
			throw error;
		},
	}) as unknown as Translate;
	return t;
}

const zh = createUiText(translator("zh-TW"));
/** 名詞一定有值的位置（測試裡已知的值）：沒有就是測試寫錯。 */
const zhOf = (m: UiMessage | undefined): string => {
	if (m === undefined) throw new Error("應該有文字");
	return zh(m);
};

describe("建構函式", () => {
	it("名詞：UNSPECIFIED 與不認得的值是 undefined，「未知」由呼叫端明確用 commonUnknown()", () => {
		expect(zh(sellerMeasureKind(MeasureKind.WARNING) ?? commonUnknown())).toBe(
			"警告",
		);
		expect(sellerMeasureKind(MeasureKind.UNSPECIFIED)).toBeUndefined();
		expect(sellerMeasureKind(999 as MeasureKind)).toBeUndefined();
		expect(
			sellerMeasureState(999 as MeasureState, { kind: 1 }),
		).toBeUndefined();
		expect(zh(commonUnknown())).toBe("未知");
	});

	it("select 參數不認得的值走 other", () => {
		expect(zhOf(sellerMeasureState(2, { kind: MeasureKind.TERMINATION }))).toBe(
			"合作已終止",
		);
		expect(zhOf(sellerMeasureState(2, { kind: MeasureKind.SUSPENSION }))).toBe(
			"停權中",
		);
		expect(zhOf(sellerMeasureState(2, { kind: 999 as MeasureKind }))).toBe(
			"停權中",
		);
		expect(zhOf(sellerMeasureState(2, { kind: MeasureKind.UNSPECIFIED }))).toBe(
			"停權中",
		);
	});

	it("名詞參數：巢狀解出，不認得整個回 undefined", () => {
		const msg = sellerMeasureTitleRisk({ ground: RiskGround.FRAUD });
		expect(msg).toEqual({
			key: "seller.Measure.title_risk",
			values: { ground: { key: "seller.RiskGround.RISK_GROUND_FRAUD" } },
		});
		expect(msg && zh(msg)).toBe("風險：詐欺");
		expect(
			sellerMeasureTitleRisk({ ground: RiskGround.UNSPECIFIED }),
		).toBeUndefined();
		expect(
			sellerMeasureTitleRisk({ ground: 999 as RiskGround }),
		).toBeUndefined();
	});

	it("變體：omit 的值回 undefined", () => {
		const approved = 10;
		expect(sellerKYCFlowStatusAction(approved)).toBeUndefined();
		expect(sellerKYCFlowStatusAction(0)).toBeUndefined();
		expect(sellerKYCFlowStatusAction(3)).toEqual({
			key: "seller.KYCFlowStatus.action.KYC_FLOW_STATUS_SUBMITTED",
		});
	});

	it("數字參數輸出 1234，不分組（use-intl 對簡單參數用 String(value)）", () => {
		const msg = sellerMeasureRejectionCooldownWarning({ days: 1234 });
		expect(zh(msg)).toContain("1234");
		expect(zh(msg)).not.toContain("1,234");
		expect(formatStaff(msg)).toContain("1234");
	});
});

describe("格式化", () => {
	it("名詞參數以同一語系先解出，再格式化外層", () => {
		const seen: string[] = [];
		const translate: Translate = (key, values) => {
			seen.push(key);
			return `${key}|${JSON.stringify(values ?? {})}`;
		};
		const out = formatUiMessage(translate, {
			key: "seller.Measure.title_risk",
			values: { ground: { key: "seller.RiskGround.RISK_GROUND_FRAUD" } },
		});
		expect(seen).toEqual([
			"seller.RiskGround.RISK_GROUND_FRAUD",
			"seller.Measure.title_risk",
		]);
		expect(out).toContain("seller.RiskGround.RISK_GROUND_FRAUD|{}");
	});

	it("en 缺字時有後備：en.gen.json 的 key 集合與 zh-TW 相同，每個值都是非空字串", () => {
		const flatten = (node: unknown, prefix = ""): Map<string, unknown> => {
			const out = new Map<string, unknown>();
			if (node !== null && typeof node === "object") {
				for (const [k, v] of Object.entries(node)) {
					for (const [kk, vv] of flatten(v, `${prefix}${k}.`)) out.set(kk, vv);
				}
			} else {
				out.set(prefix.slice(0, -1), node);
			}
			return out;
		};
		const zhKeys = flatten(zhTW);
		const enKeys = flatten(en);
		expect([...enKeys.keys()].sort()).toEqual([...zhKeys.keys()].sort());
		for (const [key, value] of enKeys) {
			expect(typeof value, key).toBe("string");
			expect((value as string).length, key).toBeGreaterThan(0);
		}
		// 還沒有英文的 key 用 zh-TW 的文字；之後補了英文，這一行仍成立（取得非空文字）。
		const eng = createUiText(translator("en"));
		expect(
			eng(sellerMeasureKind(MeasureKind.TERMINATION) ?? commonUnknown()),
		).not.toBe("");
	});

	it("messages 是預設受眾：沒有 staff 專有的文字，也沒有 staff 專有的 key", () => {
		const flow = (
			zhTW as unknown as { seller: { KYCFlowStatus: Record<string, string> } }
		).seller.KYCFlowStatus;
		expect(flow.KYC_FLOW_STATUS_SUBMITTED).toBe("平台審核中");
		expect(
			formatStaff({ key: "seller.KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED" }),
		).toBe("待審核");
		const text = JSON.stringify(zhTW);
		expect(text).not.toContain("待審核");
		expect(text).not.toContain("申覆待裁定");
	});

	it("formatStaff：staff 的名詞參數優先、staff 獨有的 key 可用", () => {
		expect(
			formatStaff({
				key: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_CLOSED",
				values: { kind: "MEASURE_REQUEST_KIND_REMEDIATION" },
			}),
		).toBe("解除請求已結案");
		expect(() => formatStaff({ key: "nope" as never })).toThrow();
	});
});

describe("manifest", () => {
	it("標示目錄與伺服器擁有的 enum", () => {
		const byName = new Map(manifest.enums.map((e) => [e.name, e]));
		expect(byName.get("hawkr.seller.v1alpha1.MeasureKind")).toMatchObject({
			tsName: "MeasureKind",
			tsModule: "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb",
			inCatalog: true,
			serverOwned: false,
		});
		expect(byName.get("hawkr.common.v1alpha1.ErrorReason")).toMatchObject({
			inCatalog: false,
			serverOwned: true,
		});
		expect(
			byName.get("hawkr.admin.seller.v1alpha1.MeasureActionUnavailableReason"),
		).toMatchObject({
			serverOwned: true,
		});
		expect(manifest.enums.filter((e) => e.inCatalog).length).toBe(16);
	});
});
