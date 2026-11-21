import assert from "node:assert/strict";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { createRouterTransport, type Transport } from "@connectrpc/connect";
import {
	CaseAction,
	CaseDecision,
	CaseReportSchema,
	CaseSchema,
	CaseState,
	type DecideCaseRequest,
	ModerationAdminService,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MeasureSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ReportOutcome,
	ReportTargetKind,
} from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	MeasureKind,
	MeasureState,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import { versionConflict } from "./fixtures/report-fixtures";

/**
 * 社群檢舉案件詳情（/admin/moderation/cases/[id]）：案件與全部檢舉（舊到新）、進行中才有決定、
 * 決定帶讀到的 etag、版本衝突時重讀；已結案是終態，沒有決定的操作。
 */

const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};
const Page = requireFromTest("../src/app/admin/moderation/cases/[id]/page")
	.default as unknown;
const h = React.createElement;
const T0 = Date.UTC(2026, 10, 1);

function kase(patch: MessageInitShape<typeof CaseSchema> = {}) {
	const init = {
		id: "case-1",
		target: { kind: ReportTargetKind.COMMENT, id: "comment-1" },
		state: CaseState.OPEN,
		createTime: timestampFromMs(T0),
		etag: "etag-7",
		...patch,
	};
	return create(CaseSchema, init as MessageInitShape<typeof CaseSchema>);
}

const reports = [
	create(CaseReportSchema, {
		id: "r-new",
		reporter: { case: "reporterBuyerId", value: "b-2" },
		category: PolicyCategory.SPAM,
		description: "新的檢舉",
		outcome: ReportOutcome.PENDING,
		createTime: timestampFromMs(T0 + 2000),
	}),
	create(CaseReportSchema, {
		id: "r-old",
		reporter: { case: "reporterStoreId", value: "7" },
		category: PolicyCategory.HATE,
		description: "舊的檢舉",
		outcome: ReportOutcome.PENDING,
		createTime: timestampFromMs(T0 + 1000),
	}),
];

function setup(
	current: () => ReturnType<typeof kase>,
	onDecide: (req: DecideCaseRequest) => unknown = () => ({}),
) {
	const decides: DecideCaseRequest[] = [];
	const measureReads: string[] = [];
	let reads = 0;
	const transport: Transport = createRouterTransport(({ service }) => {
		service(ModerationAdminService, {
			getCase: () => {
				reads++;
				return { case: current(), reports };
			},
			decideCase: (req: DecideCaseRequest) => {
				decides.push(req);
				return onDecide(req);
			},
		} as never);
		service(SellerGovernanceAdminService, {
			getMeasure: (req: { id: string }) => {
				measureReads.push(req.id);
				return {
					measure: create(MeasureSchema, {
						id: req.id,
						state: MeasureState.ACTIVE,
						kind: MeasureKind.CONTENT_TAKEDOWN,
					}),
				};
			},
		} as never);
	});
	return { decides, measureReads, transport, reads: () => reads };
}

async function open(transport: Transport) {
	navigation.params = { id: "case-1" };
	const m = mount(h(ToastProvider, null, h(Page)), transport);
	await m.render();
	return m;
}

const pick = (id: string, value: number | string) =>
	React.act(async () => {
		setValue(byId(id), String(value));
	});

test("案件與全部檢舉，舊到新", async () => {
	const { transport } = setup(() => kase());
	await open(transport);
	const text = bodyText();
	assert.ok(text.includes("留言") && text.includes("comment-1"));
	assert.ok(text.indexOf("舊的檢舉") < text.indexOf("新的檢舉"));
	assert.ok(
		text.includes("檢舉人（買家）b-2") && text.includes("檢舉人（店家）7"),
	);
});

test("進行中的案件：決定不違規帶讀到的 etag，不帶處置欄位", async () => {
	const { transport, decides } = setup(() => kase());
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.NO_VIOLATION);
	await click("確認決定");
	await until(() => decides.length === 1, "DecideCase 的請求");
	const req = decides[0];
	assert.equal(req?.id, "case-1");
	assert.equal(req?.etag, "etag-7");
	assert.equal(req?.decision, CaseDecision.NO_VIOLATION);
	assert.equal(req?.action, CaseAction.UNSPECIFIED);
	assert.equal(req?.reason, "");
});

test("違規：留言可選處置，隱藏帶類別與理由；沒填理由不送出", async () => {
	const { transport, decides } = setup(() => kase());
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.VIOLATION);
	await pick("moderation-action", CaseAction.HIDE);
	await pick("moderation-category", PolicyCategory.SPAM);
	await click("確認決定");
	assert.equal(decides.length, 0);
	assert.ok(bodyText().includes("請填寫給作者的理由"));
	await pick("moderation-reason", "  廣告洗版  ");
	await click("確認決定");
	await until(() => decides.length === 1, "DecideCase 的請求");
	const req = decides[0];
	assert.equal(req?.etag, "etag-7");
	assert.equal(req?.action, CaseAction.HIDE);
	assert.equal(req?.category, PolicyCategory.SPAM);
	assert.equal(req?.reason, "廣告洗版");
});

test("下架：店家唯讀取自快照，只填嚴重度，請求不帶店家", async () => {
	const { transport, decides } = setup(() => kase({ storeId: "store-9" }));
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.VIOLATION);
	await pick("moderation-action", CaseAction.TAKEDOWN);
	await pick("moderation-category", PolicyCategory.HARASSMENT);
	await pick("moderation-reason", "騷擾");
	assert.equal(byId("moderation-store-id").textContent, "store-9");
	assert.equal(byId("moderation-store-id").tagName, "DIV");
	await click("確認決定");
	assert.equal(decides.length, 0);
	assert.ok(bodyText().includes("請選擇嚴重度"));
	await pick("moderation-severity", ViolationSeverity.MEDIUM);
	await click("確認決定");
	await until(() => decides.length === 1, "DecideCase 的請求");
	assert.equal(decides[0]?.severity, ViolationSeverity.MEDIUM);
	assert.ok(!("storeId" in (decides[0] ?? {})));
});

test("下架：案件沒有快照店家時不送出", async () => {
	const { transport, decides } = setup(() => kase());
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.VIOLATION);
	await pick("moderation-action", CaseAction.TAKEDOWN);
	await pick("moderation-category", PolicyCategory.HARASSMENT);
	await pick("moderation-reason", "騷擾");
	await pick("moderation-severity", ViolationSeverity.MEDIUM);
	await click("確認決定");
	assert.equal(decides.length, 0);
	assert.ok(bodyText().includes("沒有內容快照的店家"));
});

test("案件有 measure_id：以它讀措施，顯示目前狀態與措施連結；沒有就不讀", async () => {
	const withMeasure = setup(() =>
		kase({
			state: CaseState.CLOSED,
			decision: CaseDecision.VIOLATION,
			action: CaseAction.TAKEDOWN,
			measureId: "measure-5",
			reportCount: BigInt(3),
		}),
	);
	await open(withMeasure.transport);
	await until(() => withMeasure.measureReads.length > 0, "GetMeasure 的請求");
	assert.deepEqual(withMeasure.measureReads, ["measure-5"]);
	const link = [...dom.window.document.querySelectorAll("a")].find(
		(a) => a.getAttribute("href") === "/admin/measures/measure-5",
	);
	assert.ok(link);
	await until(() => bodyText().includes("目前狀態"), "措施區塊");
	assert.ok(bodyText().includes("檢舉數"));
});

test("案件沒有 measure_id：不呼叫 GetMeasure", async () => {
	const none = setup(() => kase());
	await open(none.transport);
	assert.deepEqual(none.measureReads, []);
	assert.ok(!bodyText().includes("下架措施"));
});

test("已結案是終態：沒有決定的操作，顯示決定結果", async () => {
	const { transport } = setup(() =>
		kase({
			state: CaseState.CLOSED,
			decision: CaseDecision.VIOLATION,
			category: PolicyCategory.SPAM,
			action: CaseAction.HIDE,
			decidedBy: "admin-1",
			resolveTime: timestampFromMs(T0 + 5000),
		}),
	);
	await open(transport);
	assert.equal(buttonByText("決定"), undefined);
	assert.equal(maybeById("moderation-decision"), null);
	assert.ok(bodyText().includes("隱藏內容"));
});

test("版本衝突：重讀案件，對話框留著並說明", async () => {
	let version = "etag-7";
	const { transport, decides, reads } = setup(
		() => kase({ etag: version }),
		() => {
			version = "etag-8";
			throw versionConflict("case");
		},
	);
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.NO_VIOLATION);
	await click("確認決定");
	await until(() => decides.length === 1 && reads() >= 2, "衝突後重讀");
	await until(() => bodyText().includes("頁面已重新載入"), "衝突說明");
	await click("確認決定");
	await until(() => decides.length === 2, "用新版本重送");
	assert.equal(decides[1]?.etag, "etag-8");
});

test("版本衝突後重讀發現案件已結案：對話框消失，衝突說明改用 toast", async () => {
	let closed = false;
	const { transport, decides } = setup(
		() =>
			closed
				? kase({ state: CaseState.CLOSED, decision: CaseDecision.NO_VIOLATION })
				: kase(),
		() => {
			closed = true;
			throw versionConflict("case");
		},
	);
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.NO_VIOLATION);
	await click("確認決定");
	await until(() => decides.length === 1, "DecideCase 的請求");
	await until(() => maybeById("moderation-decision") === null, "對話框關閉");
	await until(
		() => bodyText().includes("頁面已重新載入"),
		"衝突說明留在頁面上",
	);
	assert.equal(buttonByText("決定"), undefined);
});
