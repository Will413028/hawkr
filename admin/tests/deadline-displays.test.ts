import assert from "node:assert/strict";
import test, { mock } from "node:test";
import { create } from "@bufbuild/protobuf";
import { createRouterTransport } from "@connectrpc/connect";
import {
	CaseState,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MeasureSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	FoodRegistrationSchema,
	FoodRegistrationState,
	KYCSummarySchema,
	SellerAdminService,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { DeadlineSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import {
	MeasureRequestKind,
	MeasureRequestState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	bodyText,
	click,
	dom,
	mount,
	navigation,
	React,
	requireFromTest,
	until,
} from "./fixtures/dom-harness";
import { kase, ts } from "./fixtures/report-fixtures";

/**
 * 既有頁面的期限顯示（設計 step10e-3d-design §4.2、§4.3，文案 #122–128）：每個位置都有
 * 「未逾期」「逾期（danger）」「沒有期限（-）」三種。現在固定在 2026-10-20 12:00（台北），測試不讀時鐘。
 */

const NOW = new Date("2026-10-20T04:00:00Z");
const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;
const h = React.createElement;
const idle = () => createRouterTransport(() => {});

/** 剩 2 天：2026/10/22 12:00 到期，整段 3 天，剩下的時間大於四分之一，沒有色階。 */
const upcoming = () =>
	create(DeadlineSchema, {
		startTime: ts(new Date(NOW.getTime() - DAY)),
		dueTime: ts(new Date(NOW.getTime() + 2 * DAY)),
	});
/** 已逾期 3 小時：2026/10/20 09:00 到期。 */
const overdue = () =>
	create(DeadlineSchema, {
		startTime: ts(new Date(NOW.getTime() - 5 * DAY)),
		dueTime: ts(new Date(NOW.getTime() - 3 * HOUR)),
	});

const UPCOMING_TEXT = "2026/10/22 12:00（剩 2 天）";
const OVERDUE_TEXT = "2026/10/20 09:00（已逾期 3 小時）";

async function withNow<T>(run: () => Promise<T>): Promise<T> {
	mock.timers.enable({ apis: ["Date"], now: NOW });
	try {
		return await run();
	} finally {
		mock.timers.reset();
	}
}

const dangerCount = () =>
	dom.window.document.querySelectorAll(".adm-sla--danger").length;

const { DeadlineLine } = requireFromTest("../src/components/UI") as {
	DeadlineLine: unknown;
};
const { CaseSummarySection } = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/CaseSections",
) as { CaseSummarySection: unknown };
const { PendingRequestSection } = requireFromTest(
	"../src/app/admin/measures/[id]/MeasureSections",
) as { PendingRequestSection: unknown };
const MeasureInbox = requireFromTest("../src/app/admin/measures/page")
	.default as unknown;
const FoodDetail = requireFromTest(
	"../src/app/admin/food-registrations/[id]/page",
).default as unknown;
const MerchantDetail = requireFromTest("../src/app/admin/merchants/[id]/page")
	.default as unknown;
const FoodInbox = requireFromTest("../src/app/admin/food-registrations/page")
	.default as unknown;

test("DeadlineLine：「{欄名}：{日期時間}（剩／已逾期 …）」；逾期是 danger；沒有期限是「{欄名}：-」", async () => {
	await withNow(async () => {
		for (const [label, deadline, want, danger] of [
			["裁定期限", upcoming(), `裁定期限：${UPCOMING_TEXT}`, 0],
			["審核期限", overdue(), `審核期限：${OVERDUE_TEXT}`, 1],
			["複審期限", undefined, "複審期限：-", 0],
		] as const) {
			const m = mount(h(DeadlineLine, { label, deadline, now: NOW }), idle());
			await m.render({ settle: false });
			assert.equal(dom.window.document.body.textContent, want);
			assert.equal(dangerCount(), danger, want);
			await m.unmount();
		}
	});
});

test("#127 案件詳情：決定期限與轉報期限放在案件區；有值才顯示，已結案的不出現", async () => {
	await withNow(async () => {
		const withBoth = kase({
			decideDeadline: upcoming(),
			fdaReferralDeadline: overdue(),
		});
		const a = mount(
			h(CaseSummarySection, { kase: withBoth, now: NOW }),
			idle(),
		);
		await a.render({ settle: false });
		assert.match(bodyText(), new RegExp(`決定期限：${UPCOMING_TEXT}`, "u"));
		assert.match(bodyText(), new RegExp(`轉報期限：${OVERDUE_TEXT}`, "u"));
		assert.equal(dangerCount(), 1, "只有轉報期限逾期");
		await a.unmount();

		const inProgress = mount(
			h(CaseSummarySection, {
				kase: kase({ decideDeadline: upcoming() }),
				now: NOW,
			}),
			idle(),
		);
		await inProgress.render({ settle: false });
		assert.match(bodyText(), new RegExp(`決定期限：${UPCOMING_TEXT}`, "u"));
		assert.doesNotMatch(bodyText(), /轉報期限/u, "沒有轉報期限就不顯示這一行");
		await inProgress.unmount();

		const closed = mount(
			h(CaseSummarySection, {
				kase: kase({ state: CaseState.CLOSED }),
				now: NOW,
			}),
			idle(),
		);
		await closed.render({ settle: false });
		assert.doesNotMatch(bodyText(), /決定期限/u, "已結案的案件沒有決定期限");
		assert.doesNotMatch(bodyText(), /轉報期限/u, "已結案的案件沒有轉報期限");
		assert.equal(dangerCount(), 0);
		await closed.unmount();
	});
});

test("#123 措施詳情的待處理請求：裁定期限；沒有期限是「-」", async () => {
	const measureWith = (decideDeadline?: ReturnType<typeof upcoming>) =>
		create(MeasureSchema, {
			id: "m-1",
			requests: [
				{
					id: "r-1",
					kind: MeasureRequestKind.APPEAL,
					state: MeasureRequestState.PENDING,
					submitTime: ts(new Date(NOW.getTime() - DAY)),
					decideDeadline,
				},
			],
		});
	await withNow(async () => {
		for (const [deadline, pattern, danger] of [
			[upcoming(), `裁定期限：${UPCOMING_TEXT}`, 0],
			[overdue(), `裁定期限：${OVERDUE_TEXT}`, 1],
			[undefined, "裁定期限：-", 0],
		] as const) {
			const m = mount(
				h(PendingRequestSection, { measure: measureWith(deadline), now: NOW }),
				idle(),
			);
			await m.render({ settle: false });
			assert.match(bodyText(), new RegExp(pattern, "u"));
			assert.equal(dangerCount(), danger, pattern);
			await m.unmount();
		}
	});
});

test("#122／#128 措施清單：複審到期分頁每列「複審期限：…」；沒有店名顯示「店家 {id}」，不退回措施 ID", async () => {
	const measure = (
		id: string,
		patch: Parameters<typeof create<typeof MeasureSchema>>[1],
	) => create(MeasureSchema, { id, storeId: "77", ...patch });
	await withNow(async () => {
		const m = mount(
			h(MeasureInbox),
			createRouterTransport(({ service }) => {
				service(SellerGovernanceAdminService, {
					listMeasures: () => ({
						measures: [
							measure("measure-id-1", {
								storeName: "阿里山農場",
								reviewDeadline: upcoming(),
							}),
							measure("measure-id-2", { reviewDeadline: overdue() }),
							measure("measure-id-3", { storeName: "" }),
						],
					}),
				} as never);
			}),
		);
		await m.render();
		await click("複審到期");
		await until(
			() => dom.window.document.querySelectorAll("tbody tr").length === 3,
			"複審到期分頁的三列",
		);
		const rows = [...dom.window.document.querySelectorAll("tbody tr")].map(
			(tr) => tr.querySelector("td")?.textContent ?? "",
		);
		assert.deepEqual(rows, [
			`阿里山農場複審期限：${UPCOMING_TEXT}`,
			`店家 77複審期限：${OVERDUE_TEXT}`,
			"店家 77複審期限：-",
		]);
		assert.equal(dangerCount(), 1);
		for (const id of ["measure-id-1", "measure-id-2", "measure-id-3"]) {
			assert.ok(
				!rows.some((row) => row.includes(id)),
				`店家欄不會出現措施 ID ${id}`,
			);
		}
		await m.unmount();
	});
});

test("#124 食品業者登錄清單：新欄「期限」；待審核照伺服器的順序，等商家認證顯示「-」；換分頁回第一頁", async () => {
	const registration = (
		id: string,
		patch: Parameters<typeof create<typeof FoodRegistrationSchema>>[1],
	) =>
		create(FoodRegistrationSchema, {
			id,
			state: FoodRegistrationState.PENDING,
			registrationNumber: `字號-${id}`,
			...patch,
		});
	const requests: { pageToken: string; inbox: number }[] = [];
	await withNow(async () => {
		const m = mount(
			h(FoodInbox),
			createRouterTransport(({ service }) => {
				service(SellerAdminService, {
					listFoodRegistrations: (req: {
						pageToken: string;
						inbox: number;
					}) => {
						requests.push({ pageToken: req.pageToken, inbox: req.inbox });
						return req.inbox === 1
							? {
									registrations: [
										registration("a", { decideDeadline: overdue() }),
										registration("b", { decideDeadline: upcoming() }),
									],
									nextPageToken: "next",
								}
							: { registrations: [registration("c", {})] };
					},
				} as never);
			}),
		);
		await m.render();
		await until(
			() => dom.window.document.querySelectorAll("tbody tr").length === 2,
			"待審核的兩列",
		);
		const headers = [...dom.window.document.querySelectorAll("thead th")].map(
			(th) => th.textContent,
		);
		assert.ok(headers.includes("期限"), `欄位 ${headers.join("｜")}`);
		const column = headers.indexOf("期限");
		const cells = () =>
			[...dom.window.document.querySelectorAll("tbody tr")].map(
				(tr) => tr.querySelectorAll("td")[column]?.textContent,
			);
		assert.deepEqual(cells(), ["已逾期 3 小時", "剩 2 天"], "照伺服器的順序");
		await click("下一頁 ");
		await until(
			() => requests.at(-1)?.pageToken === "next",
			"第二頁的請求帶上一頁給的 token",
		);
		await click("等商家認證");
		await until(() => cells().join() === "-", "等商家認證的一列");
		assert.deepEqual(cells(), ["-"]);
		assert.equal(requests.at(-1)?.pageToken, "", "換分頁回第一頁");
		await m.unmount();
	});
});

test("#125 食品業者登錄詳情：裁定期限；沒有期限是「-」", async () => {
	await withNow(async () => {
		for (const [deadline, pattern, danger] of [
			[upcoming(), `裁定期限：${UPCOMING_TEXT}`, 0],
			[overdue(), `裁定期限：${OVERDUE_TEXT}`, 1],
			[undefined, "裁定期限：-", 0],
		] as const) {
			navigation.params = { id: "f-1" };
			const m = mount(
				h(FoodDetail),
				createRouterTransport(({ service }) => {
					service(SellerAdminService, {
						getFoodRegistration: () => ({
							registration: create(FoodRegistrationSchema, {
								id: "f-1",
								state: FoodRegistrationState.PENDING,
								registrationNumber: "字號-f-1",
								decideDeadline: deadline,
							}),
						}),
					} as never);
				}),
			);
			await m.render();
			await until(() => bodyText().includes("裁定期限："), "裁定期限那一行");
			assert.match(bodyText(), new RegExp(pattern, "u"));
			assert.equal(dangerCount(), danger, pattern);
			await m.unmount();
		}
	});
});

test("#126 商家詳情的 KYC：審核期限；沒有期限是「-」；沒有送出時間時整段不顯示", async () => {
	const storeWith = (
		kyc: Parameters<typeof create<typeof KYCSummarySchema>>[1] | undefined,
	) =>
		({
			store: create(StoreSchema, { id: "9", storeName: "阿里山農場" }),
			kyc: kyc ? create(KYCSummarySchema, kyc) : undefined,
		}) as never;
	await withNow(async () => {
		for (const [kyc, pattern, danger] of [
			[
				{
					submitTime: ts(new Date(NOW.getTime() - DAY)),
					decideDeadline: upcoming(),
				},
				`審核期限：${UPCOMING_TEXT}`,
				0,
			],
			[
				{
					submitTime: ts(new Date(NOW.getTime() - 8 * DAY)),
					decideDeadline: overdue(),
				},
				`審核期限：${OVERDUE_TEXT}`,
				1,
			],
			[{ submitTime: ts(new Date(NOW.getTime() - DAY)) }, "審核期限：-", 0],
			[{ decideDeadline: upcoming() }, "", 0],
			[undefined, "", 0],
		] as const) {
			navigation.params = { id: "9" };
			const m = mount(
				h(MerchantDetail),
				createRouterTransport(({ service }) => {
					service(SellerAdminService, {
						getStore: () => storeWith(kyc),
					} as never);
				}),
			);
			await m.render({ settle: false });
			await until(() => bodyText().includes("阿里山農場"), "商家詳情的店名");
			if (pattern) {
				assert.match(bodyText(), new RegExp(pattern, "u"));
			} else {
				assert.doesNotMatch(
					bodyText(),
					/審核期限/u,
					"沒有送出時間就沒有這一行",
				);
			}
			assert.equal(dangerCount(), danger, pattern);
			await m.unmount();
		}
	});
});
