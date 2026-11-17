import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	AdminRevokeReason,
	type CreateMeasureRequest,
	type RevokeMeasureRequest,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MeasureSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	AdminPostSchema,
	type ListPostsRequest,
	SocialAdminService,
} from "@hawkr/api/gen/hawkr/admin/social/v1alpha1/social_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ContentKind,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { PostStatus } from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import {
	bodyText,
	byId,
	click,
	clickElement,
	dom,
	mount,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";

/**
 * 貼文管理（/admin/posts）：keyset 的上一頁與下一頁；「下架」建立以該貼文為對象的內容下架措施；
 * 「恢復」撤銷下架它的措施（帶 takedown_measure_id、手動撤銷原因與讀到的 etag），沒有第二個入口。
 */

const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};
const Page = requireFromTest("../src/app/admin/posts/page").default as unknown;
const h = React.createElement;

const normal = create(AdminPostSchema, {
	id: "post-1",
	storeId: "store-1",
	body: "正常貼文",
	status: PostStatus.PUBLISHED,
});
const takenDown = create(AdminPostSchema, {
	id: "post-2",
	storeId: "store-2",
	body: "被下架的貼文",
	status: PostStatus.PUBLISHED,
	takedownMeasureId: "measure-9",
});
const takenDownOther = create(AdminPostSchema, {
	id: "post-3",
	storeId: "store-3",
	body: "另一則被下架的貼文",
	status: PostStatus.PUBLISHED,
	takedownMeasureId: "measure-10",
});

function setup(rejectCreate?: ConnectError) {
	const creates: CreateMeasureRequest[] = [];
	const revokes: RevokeMeasureRequest[] = [];
	const lists: ListPostsRequest[] = [];
	const gets: string[] = [];
	const transport: Transport = createRouterTransport(({ service }) => {
		service(SocialAdminService, {
			listPosts: (req: ListPostsRequest) => {
				lists.push(req);
				return req.pageToken === "t2"
					? { posts: [normal], nextPageToken: "" }
					: { posts: [normal, takenDown, takenDownOther], nextPageToken: "t2" };
			},
		} as never);
		service(SellerGovernanceAdminService, {
			createMeasure: (req: CreateMeasureRequest) => {
				creates.push(req);
				if (rejectCreate) throw rejectCreate;
				return {};
			},
			getMeasure: (req: { id: string }) => {
				gets.push(req.id);
				return {
					measure: create(MeasureSchema, {
						id: req.id,
						etag: `etag-of-${req.id}`,
						canRevoke: true,
						contents: [
							{ kind: ContentKind.POST, id: "post-2" },
							{ kind: ContentKind.COMMENT, id: "comment-77" },
						],
					}),
				};
			},
			revokeMeasure: (req: RevokeMeasureRequest) => {
				revokes.push(req);
				return {};
			},
		} as never);
	});
	return { creates, revokes, lists, gets, transport };
}

async function open(transport: Transport) {
	const m = mount(h(ToastProvider, null, h(Page)), transport);
	await m.render();
	return m;
}

const pick = (id: string, value: number | string) =>
	React.act(async () => {
		setValue(byId(id), String(value));
	});

const rowButton = (rowText: string, label: string) => {
	const row = [...dom.window.document.querySelectorAll("tbody tr")].find((tr) =>
		tr.textContent?.includes(rowText),
	);
	return [...(row?.querySelectorAll("button") ?? [])].find(
		(b) => b.textContent === label,
	);
};

const pager = (label: string) =>
	[...dom.window.document.querySelectorAll("button")].find((b) =>
		b.textContent?.includes(label),
	) as HTMLButtonElement;

test("下架：建立內容下架措施，對象是這則貼文，認定可選社群類別", async () => {
	const { transport, creates } = setup();
	await open(transport);
	const button = rowButton("正常貼文", "下架");
	assert.ok(button);
	assert.equal(rowButton("被下架的貼文", "下架"), undefined);
	await clickElement(button);
	await pick("takedown-category", PolicyCategory.SPAM);
	await pick("takedown-severity", ViolationSeverity.LOW);
	await pick("takedown-statement", " 廣告洗版 ");
	await pick("takedown-note", "內部說明");
	await click("確認下架");
	await until(() => creates.length === 1, "CreateMeasure 的請求");
	const req = creates[0];
	assert.equal(req?.storeId, "store-1");
	assert.equal(req?.statement, "廣告洗版");
	assert.equal(req?.measure.case, "contentTakedown");
	if (req?.measure.case !== "contentTakedown") return;
	const { contents, violation } = req.measure.value;
	assert.equal(contents.length, 1);
	assert.equal(contents[0]?.kind, ContentKind.POST);
	assert.equal(contents[0]?.id, "post-1");
	assert.equal(violation?.finding?.category, PolicyCategory.SPAM);
	assert.equal(violation?.finding?.severity, ViolationSeverity.LOW);
	assert.ok(req.requestId.length > 0);
});

test("下架欄位沒填完不送出", async () => {
	const { transport, creates } = setup();
	await open(transport);
	const button = rowButton("正常貼文", "下架");
	assert.ok(button);
	await clickElement(button);
	await click("確認下架");
	assert.equal(creates.length, 0);
	assert.ok(bodyText().includes("請選擇類別"));
});

test("恢復：撤銷 takedown_measure_id 的措施，帶手動原因與 etag", async () => {
	const { transport, revokes } = setup();
	await open(transport);
	assert.equal(rowButton("正常貼文", "恢復"), undefined);
	const button = rowButton("被下架的貼文", "恢復");
	assert.ok(button);
	await clickElement(button);
	await until(
		() => document_has("revoke-measure-reason"),
		"撤銷對話框載入措施",
	);
	await pick("revoke-measure-reason", AdminRevokeReason.ADMIN_CORRECTION);
	await click("確認撤銷");
	await until(() => revokes.length === 1, "RevokeMeasure 的請求");
	assert.equal(revokes[0]?.id, "measure-9");
	assert.equal(revokes[0]?.reason, AdminRevokeReason.ADMIN_CORRECTION);
	assert.equal(revokes[0]?.etag, "etag-of-measure-9");
});

test("恢復：每則貼文撤銷自己的 takedown_measure_id，不是別的措施", async () => {
	const { transport, revokes, gets } = setup();
	await open(transport);
	const button = rowButton("另一則被下架的貼文", "恢復");
	assert.ok(button);
	await clickElement(button);
	await until(
		() => document_has("revoke-measure-reason"),
		"撤銷對話框載入措施",
	);
	assert.deepEqual(gets, ["measure-10"]);
	await pick("revoke-measure-reason", AdminRevokeReason.ADMIN_CORRECTION);
	await click("確認撤銷");
	await until(() => revokes.length === 1, "RevokeMeasure 的請求");
	assert.equal(revokes[0]?.id, "measure-10");
	assert.equal(revokes[0]?.etag, "etag-of-measure-10");
});

test("恢復：對話框列出措施涵蓋的全部內容並說明會全部恢復", async () => {
	const { transport } = setup();
	await open(transport);
	const button = rowButton("被下架的貼文", "恢復");
	assert.ok(button);
	await clickElement(button);
	await until(
		() => document_has("revoke-measure-reason"),
		"撤銷對話框載入措施",
	);
	const text = bodyText();
	assert.ok(text.includes("全部 2 則內容"));
	assert.ok(text.includes("post-2") && text.includes("comment-77"));
});

function document_has(id: string) {
	return dom.window.document.getElementById(id) !== null;
}

test("翻頁：下一頁帶回傳的 token，上一頁退回；換篩選回第一頁", async () => {
	const { transport, lists } = setup();
	await open(transport);
	assert.equal(lists[0]?.pageToken, "");
	assert.equal(lists[0]?.takenDownOnly, false);
	await clickElement(pager("下一頁"));
	await until(() => lists.at(-1)?.pageToken === "t2", "第二頁的請求");
	await until(() => pager("上一頁") !== undefined, "第二頁載入");
	await clickElement(pager("上一頁"));
	await until(
		() => lists.length >= 2 && lists.at(-1)?.pageToken === "",
		"退回第一頁的請求",
	);
	await clickElement(pager("下一頁"));
	await until(() => lists.at(-1)?.pageToken === "t2", "再到第二頁");
	await click("已下架");
	await until(() => lists.at(-1)?.takenDownOnly === true, "只看已下架");
	assert.equal(lists.at(-1)?.pageToken, "");
});

test("下架：貼文已有生效中的下架措施時顯示伺服器的句子並重新讀取列表", async () => {
	const message = "這則內容已經有生效中的下架措施";
	const { transport, creates, lists } = setup(
		new ConnectError("active", Code.FailedPrecondition, undefined, [
			{
				desc: ErrorInfoSchema,
				value: {
					reason: "CONTENT_TAKEDOWN_ACTIVE",
					domain: "hawkr",
				},
			},
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message },
			},
		]),
	);
	await open(transport);
	const button = rowButton("正常貼文", "下架");
	assert.ok(button);
	await clickElement(button);
	await pick("takedown-category", PolicyCategory.SPAM);
	await pick("takedown-severity", ViolationSeverity.LOW);
	await pick("takedown-statement", "廣告洗版");
	await pick("takedown-note", "內部說明");
	const listsBefore = lists.length;
	await click("確認下架");
	await until(() => creates.length === 1, "CreateMeasure 的請求");
	await until(() => bodyText().includes(message), "顯示伺服器的句子");
	await until(() => lists.length > listsBefore, "重新讀取列表");
	assert.ok(bodyText().includes("確認下架"));
});
