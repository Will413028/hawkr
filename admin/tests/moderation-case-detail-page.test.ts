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
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ReportOutcome,
	ReportTargetKind,
} from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
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
		reporterId: "b-2",
		category: PolicyCategory.SPAM,
		description: "新的檢舉",
		outcome: ReportOutcome.PENDING,
		createTime: timestampFromMs(T0 + 2000),
	}),
	create(CaseReportSchema, {
		id: "r-old",
		reporterId: "b-1",
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
	});
	return { decides, transport, reads: () => reads };
}

async function open(transport: Transport) {
	navigation.params = { id: "case-1" };
	const m = mount(h(Page), transport);
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
	assert.equal(req?.storeId, "");
});

test("下架要店家與嚴重度，請求帶進去", async () => {
	const { transport, decides } = setup(() => kase());
	await open(transport);
	await click("決定");
	await pick("moderation-decision", CaseDecision.VIOLATION);
	await pick("moderation-action", CaseAction.TAKEDOWN);
	await pick("moderation-category", PolicyCategory.HARASSMENT);
	await pick("moderation-reason", "騷擾");
	await click("確認決定");
	assert.equal(decides.length, 0);
	await pick("moderation-store-id", "store-9");
	await pick("moderation-severity", ViolationSeverity.MEDIUM);
	await click("確認決定");
	await until(() => decides.length === 1, "DecideCase 的請求");
	assert.equal(decides[0]?.storeId, "store-9");
	assert.equal(decides[0]?.severity, ViolationSeverity.MEDIUM);
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
