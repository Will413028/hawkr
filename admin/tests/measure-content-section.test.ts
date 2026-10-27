import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { MeasureSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasurePeriodSchema,
	MeasureState,
	RiskGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/**
 * 措施詳情的「措施內容」：生效時間、結束時間、複審期限、申覆期限這四列各自依契約欄位有沒有值顯示，
 * 不依措施種類（終止合作才有申覆期限、停權才有結束時間是伺服器的資料，不是畫面的條件）。
 */

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};
const { createElement } = requireFromTest("react") as {
	createElement: (type: unknown, props?: unknown) => unknown;
};
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const { MeasureContentSection } = requireFromTest(
	"../src/app/admin/measures/[id]/MeasureSections",
) as { MeasureContentSection: unknown };

const at = (iso: string) => timestampFromDate(new Date(iso));

function render(patch: Parameters<typeof create<typeof MeasureSchema>>[1]) {
	return renderToStaticMarkup(
		createElement(MeasureContentSection, {
			measure: create(MeasureSchema, {
				id: "m1",
				storeId: "100",
				kind: MeasureKind.SUSPENSION,
				basis: MeasureBasis.RISK,
				riskGround: RiskGround.FRAUD,
				state: MeasureState.ACTIVE,
				statement: "理由",
				createTime: at("2026-10-01T00:00:00Z"),
				...patch,
			}),
		}),
	);
}

test("停權：有生效時間（建立時間）與結束時間；沒有結束時間時不顯示「結束時間：-」", () => {
	const fixed = render({
		period: create(MeasurePeriodSchema, {
			startTime: at("2026-10-01T00:00:00Z"),
			endTime: at("2026-10-05T00:00:00Z"),
		}),
		endTime: at("2026-10-05T00:00:00Z"),
	});
	assert.match(fixed, /生效時間/u);
	assert.match(fixed, /結束時間/u);
	const open = render({
		period: create(MeasurePeriodSchema, {
			startTime: at("2026-10-01T00:00:00Z"),
		}),
	});
	assert.match(open, /生效時間/u);
	assert.doesNotMatch(open, /結束時間/u);
});

test("終止合作：有申覆期限；已結束的預告終止有結束時間（可能早於生效時間）；沒有期限就沒有那一列", () => {
	const ended = render({
		kind: MeasureKind.TERMINATION,
		state: MeasureState.ENDED,
		period: create(MeasurePeriodSchema, {
			startTime: at("2026-11-01T00:00:00Z"),
			endTime: at("2026-10-20T00:00:00Z"),
		}),
		endTime: at("2026-10-20T00:00:00Z"),
		appealDeadlineTime: at("2026-11-08T00:00:00Z"),
	});
	assert.match(ended, /生效時間/u);
	assert.match(ended, /結束時間/u);
	assert.match(ended, /申覆期限/u);
	const noDeadline = render({
		kind: MeasureKind.TERMINATION,
		period: create(MeasurePeriodSchema, {
			startTime: at("2026-11-01T00:00:00Z"),
		}),
	});
	assert.doesNotMatch(noDeadline, /申覆期限/u);
});

test("沒有結束時間而有複審期限（開放式的風險停權）：結束時間寫「未定」，與狀態卡同一個規則", () => {
	const open = render({
		period: create(MeasurePeriodSchema, {
			startTime: at("2026-10-01T00:00:00Z"),
		}),
		reviewBy: at("2026-10-08T00:00:00Z"),
	});
	assert.match(open, /結束時間/u);
	assert.match(open, /未定（由平台定期複審）/u);
	assert.match(open, /複審期限/u);
});

test("沒有任何時間欄位：四列都不顯示；複審期限有值才顯示", () => {
	const none = render({});
	assert.doesNotMatch(none, /生效時間|結束時間|複審期限|申覆期限/u);
	assert.match(render({ reviewBy: at("2026-10-08T00:00:00Z") }), /複審期限/u);
});
