import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	type Measure,
	MeasureActionUnavailabilitySchema,
	MeasureActionUnavailableReason,
	MeasureSchema,
	ViolationSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasurePeriodSchema,
	MeasureState,
	RiskGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/**
 * 商家詳情的停權狀態卡實際渲染出來的 HTML（ActiveMeasureCards，GetStore.active_measures）。
 *
 * 守的是：結束按鈕只由措施自己的 `can_end` 決定；違規依據的措施沒有結束按鈕，
 * 只連到它依據的違規（可能不只一筆）；風險依據的措施顯示風險類型、給賣家的理由與複審期限，沒有結束時間時說明由平台定期複審。
 * 清單中的每一筆都要畫出來。寫法與 delegated-operation-entry-gating.test.ts 相同（renderToStaticMarkup），不是拿 regex 掃源碼。
 */

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

interface ReactLike {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
}

const { createElement } = requireFromTest("react") as ReactLike;
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const cardModule = requireFromTest(
	"../src/app/admin/merchants/[id]/ActiveMeasuresCard",
) as { default: unknown; ActiveMeasureCards: unknown };

const END = "結束措施";
const VIOLATION_TEXT =
	"因違規處分停權；違規認定只供查閱，撤銷要對這個措施進行。";

function renderCards(
	active: Measure[],
	onEnd: (m: Measure) => void = () => {},
): string {
	return renderToStaticMarkup(
		createElement(cardModule.ActiveMeasureCards, {
			active,
			endPending: false,
			onEnd,
		}),
	);
}

/** 狀態區每一張卡的 HTML（依 DOM 順序）。 */
function cardsOf(html: string): string[] {
	return html.split('class="audit-notice rejected suspension-banner"').slice(1);
}

function countOf(html: string, needle: string): number {
	return html.split(needle).length - 1;
}

function risk(patch: MessageInitShape<typeof MeasureSchema> = {}): Measure {
	return create(MeasureSchema, {
		id: "r1",
		storeId: "100",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.RISK,
		riskGround: RiskGround.FRAUD,
		statement: "疑似詐欺，先行停權",
		state: MeasureState.ACTIVE,
		createdByAdminId: "9",
		createTime: timestampFromDate(new Date("2026-10-01T00:00:00Z")),
		reviewBy: timestampFromDate(new Date("2026-10-08T00:00:00Z")),
		etag: "v1",
		canEnd: true,
		...patch,
	});
}

function violation(id: string, findingIds: string[]): Measure {
	return create(MeasureSchema, {
		id,
		storeId: "100",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.VIOLATION,
		state: MeasureState.ACTIVE,
		canEnd: false,
		// 句子由伺服器給（model.ActionUnavailableText，VIOLATION_BASIS）；client 不寫。
		endUnavailabilities: [
			create(MeasureActionUnavailabilitySchema, {
				reason: MeasureActionUnavailableReason.VIOLATION_BASIS,
				message: { locale: "zh-TW", message: VIOLATION_TEXT },
			}),
		],
		findings: findingIds.map((f) => create(ViolationSchema, { id: f })),
	});
}

test("並存：風險措施與兩筆違規措施都畫出來；結束按鈕只在 can_end 的風險措施那張，整個狀態區只有一顆", () => {
	const html = renderCards([
		violation("v1", ["88"]),
		risk(),
		violation("v2", ["89"]),
	]);
	const cards = cardsOf(html);
	assert.equal(cards.length, 3, "並存時應該是三張卡");
	assert.match(cards[1] ?? "", /結束措施/u, "風險措施那張有結束按鈕");
	assert.match(cards[1] ?? "", /風險：詐欺/u, "風險類型照對照表顯示");
	assert.match(cards[1] ?? "", /疑似詐欺，先行停權/u, "給賣家的理由");
	assert.match(cards[1] ?? "", /操作者 ID：9/u);
	assert.doesNotMatch(
		cards[1] ?? "",
		/\/admin\/violations\//u,
		"風險措施不連到違規紀錄",
	);
	for (const card of [cards[0] ?? "", cards[2] ?? ""]) {
		assert.match(card, new RegExp(VIOLATION_TEXT, "u"));
		assert.doesNotMatch(card, new RegExp(END, "u"), "違規措施不得有結束按鈕");
	}
	assert.equal(countOf(html, END), 1, "整個狀態區只能有一顆結束按鈕");
});

test("結束按鈕只看措施自己的 can_end：店家是否關閉不參與（措施以帳號為範圍）", () => {
	assert.equal(countOf(renderCards([risk()]), END), 1);
	assert.equal(
		countOf(renderCards([risk({ canEnd: false })]), END),
		0,
		"措施的 can_end 為假（例如沒有生效）不得有結束",
	);
});

test("違規措施不連到舊的違規頁（舊頁用整數 ID、打舊 gateway，撤銷不了這個措施），也不叫操作者從違規紀錄撤銷", () => {
	const html = renderCards([violation("v1", ["88", "89"])]);
	assert.equal(countOf(html, "<a href="), 0);
	assert.doesNotMatch(html, /\/admin\/violations/u);
	assert.doesNotMatch(html, /從違規紀錄撤銷/u);
});

test("沒有生效中的措施：狀態區不畫任何東西", () => {
	assert.equal(renderCards([]), "");
});

test("風險措施沒有結束時間：說明由平台定期複審，並顯示複審期限；有結束時間時不顯示這句", () => {
	const open = renderCards([risk()]);
	assert.match(open, /未定（由平台定期複審）/u);
	assert.match(open, /複審期限：/u);
	const fixed = renderCards([
		risk({ endTime: timestampFromDate(new Date("2026-10-05T00:00:00Z")) }),
	]);
	assert.doesNotMatch(fixed, /未定（由平台定期複審）/u);
});

test("每一張卡都連到措施詳情頁 /admin/measures/[id]，違規依據的卡不再說撤銷操作尚未提供", () => {
	const html = renderCards([violation("v1", ["88"]), risk({ id: "r9" })]);
	const cards = cardsOf(html);
	assert.equal(cards.length, 2);
	assert.match(cards[0] ?? "", /href="\/admin\/measures\/v1"/u);
	assert.match(cards[1] ?? "", /href="\/admin\/measures\/r9"/u);
	assert.doesNotMatch(html, /尚未提供/u);
});

test("卡片標題取自目錄、依措施的 state 與 kind：生效中的停權是「停權中」，生效中的終止合作是「合作已終止」", () => {
	const suspension = cardsOf(renderCards([risk()]))[0] ?? "";
	assert.match(suspension, /停權中/u);
	assert.doesNotMatch(suspension, /賣家停權中/u);
	const termination =
		cardsOf(
			renderCards([
				risk({
					kind: MeasureKind.TERMINATION,
					appealDeadlineTime: timestampFromDate(
						new Date("2026-10-19T02:00:00Z"),
					),
				}),
			]),
		)[0] ?? "";
	assert.match(termination, /合作已終止/u);
	assert.match(termination, /申覆期限：2026\/10\/19 10:00/u);
	// 標題照 state，不寫死 ACTIVE：預定生效的終止合作不是「合作已終止」。
	const scheduled =
		cardsOf(
			renderCards([
				risk({ kind: MeasureKind.TERMINATION, state: MeasureState.SCHEDULED }),
			]),
		)[0] ?? "";
	assert.doesNotMatch(scheduled, /合作已終止/u);
	assert.match(scheduled, /預定生效/u);
});

test("時間列依契約欄位有沒有值：生效時間、結束時間、複審期限、申覆期限；沒有的不顯示，也沒有「申覆期限：-」", () => {
	const none =
		cardsOf(
			renderCards([
				risk({ reviewBy: undefined, kind: MeasureKind.TERMINATION }),
			]),
		)[0] ?? "";
	assert.doesNotMatch(none, /生效時間|結束時間|複審期限|申覆期限/u);
	const all =
		cardsOf(
			renderCards([
				risk({
					period: create(MeasurePeriodSchema, {
						startTime: timestampFromDate(new Date("2026-10-02T00:00:00Z")),
					}),
					endTime: timestampFromDate(new Date("2026-10-05T00:00:00Z")),
				}),
			]),
		)[0] ?? "";
	assert.match(all, /生效時間：2026\/10\/02 08:00/u);
	assert.match(all, /結束時間：2026\/10\/05 08:00/u);
	assert.match(all, /複審期限：/u);
	assert.doesNotMatch(all, /申覆期限/u);
});

test("違規依據的停權和其他措施一樣列依據、理由與時間，不能結束的原因是伺服器給的第一個；client 的原始碼沒有這句", async () => {
	const card = cardsOf(renderCards([violation("v1", ["88"])]))[0] ?? "";
	assert.match(card, /依據：/u);
	assert.match(card, /給賣家的理由：/u);
	assert.match(card, new RegExp(VIOLATION_TEXT, "u"));
	// 沒有給原因時仍相信 can_end=false，顯示通用文字，不自己寫句子。
	const bare =
		cardsOf(
			renderCards([risk({ canEnd: false, endUnavailabilities: [] })]),
		)[0] ?? "";
	assert.match(bare, /目前不能操作/u);
	assert.doesNotMatch(bare, new RegExp(VIOLATION_TEXT, "u"));
	// 可以結束時沒有原因句。
	assert.doesNotMatch(cardsOf(renderCards([risk()]))[0] ?? "", /目前不能操作/u);
	for (const path of [
		"src/app/admin/merchants/[id]/ActiveMeasuresCard.tsx",
		"src/app/admin/merchants/[id]/store-detail.ts",
	]) {
		const { readFile } = await import("node:fs/promises");
		assert.doesNotMatch(await readFile(path, "utf8"), /因違規處分停權/u, path);
	}
});

test("結束時間列與詳情頁同一個規則：沒有結束時間而有複審期限寫「未定」，兩者都沒有就沒有這一列", () => {
	const open = cardsOf(renderCards([risk()]))[0] ?? "";
	assert.match(open, /結束時間：未定（由平台定期複審）/u);
	const none =
		cardsOf(
			renderCards([risk({ reviewBy: undefined, endTime: undefined })]),
		)[0] ?? "";
	assert.doesNotMatch(none, /結束時間/u);
	const fixed =
		cardsOf(
			renderCards([
				risk({
					reviewBy: undefined,
					endTime: timestampFromDate(new Date("2026-10-05T00:00:00Z")),
				}),
			]),
		)[0] ?? "";
	assert.match(fixed, /結束時間：2026\/10\/05 08:00/u);
	assert.doesNotMatch(
		fixed,
		/複審期限/u,
		"沒有複審期限就沒有「· 複審期限：-」",
	);
});
