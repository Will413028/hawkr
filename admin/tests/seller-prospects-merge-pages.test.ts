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
	ContactKind,
	ConvertProspectRequestSchema,
	MarkProspectsDistinctRequestSchema,
	MergeProspectsRequestSchema,
	OutreachStatus,
	ProspectMatchReason,
	ProspectSchema,
	ProspectStatus,
	SellerProspectAdminService,
	VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { KYCStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	bodyText,
	buttonByText,
	buttonsByText,
	byId,
	click,
	clickElement,
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
 * 站外業者的疑似重複、合併、統編對帳，與修改時的合併出口實際掛進 DOM 後的行為（router transport 回真的 protobuf 訊息）。
 */

const h = React.createElement;
const doc = () => dom.window.document;

const DuplicatesPage = requireFromTest(
	"../src/app/admin/seller-prospects/duplicates/page",
).default as unknown;
const ReconciliationPage = requireFromTest(
	"../src/app/admin/seller-prospects/reconciliation/page",
).default as unknown;
const DetailPage = requireFromTest(
	"../src/app/admin/seller-prospects/[id]/page",
).default as unknown;
const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};

type Init = Parameters<typeof create<typeof ProspectSchema>>[1];
function prospect(init: Init = {}) {
	return create(ProspectSchema, {
		id: "7",
		etag: "e1",
		displayName: "阿里山農場",
		vendorType: VendorType.BUSINESS,
		status: ProspectStatus.ACTIVE,
		outreachStatus: OutreachStatus.NOT_CONTACTED,
		region: "嘉義",
		contacts: [{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true }],
		...init,
	});
}

function failure(
	code: Code,
	message: string,
	reason?: string,
	metadata: Record<string, string> = {},
): ConnectError {
	return new ConnectError("raw", code, undefined, [
		...(reason
			? [
					{
						desc: ErrorInfoSchema,
						value: { reason, domain: "hawkr", metadata },
					},
				]
			: []),
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message },
		},
	] as never);
}
const matchFound = (ids: string) =>
	failure(Code.FailedPrecondition, "找到相符的業者", "PROSPECT_MATCH_FOUND", {
		prospectIds: ids,
	});
const taxIdHeld = (id: string) =>
	failure(
		Code.FailedPrecondition,
		`統一編號已由業者 ${id} 使用，該做的是合併`,
		"PROSPECT_TAX_ID_HELD",
		{ prospectIds: id },
	);

type Handlers = Partial<
	Record<
		keyof typeof SellerProspectAdminService.method,
		(req: never) => unknown
	>
>;
function transportWith(handlers: Handlers): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerProspectAdminService, handlers as never);
	});
}
/** 指定的方法以網路層失敗回應（像 fetch 斷線：connect-es 把 TypeError 包成 ConnectError），其他交給原本的 transport。 */
function noResponseOn(inner: Transport, methodName: string): Transport {
	return {
		...inner,
		unary: ((...args: Parameters<Transport["unary"]>) =>
			args[0].name === methodName
				? Promise.reject(ConnectError.from(new TypeError("Failed to fetch")))
				: inner.unary(...args)) as Transport["unary"],
		stream: inner.stream.bind(inner),
	};
}
function open(element: unknown, transport: Transport) {
	return mount(h(ToastProvider, null, element), transport);
}

async function type(el: Element, value: string) {
	await React.act(async () => {
		setValue(el, value);
	});
}

/** 兩邊各自的版本：重讀會拿到新的 etag。 */
function versionedProspects(
	byId_: Record<string, Init>,
	versions: Record<string, number>,
	reads: string[],
) {
	return (req: { id: string }) => {
		reads.push(req.id);
		return {
			prospect: prospect({
				id: req.id,
				etag: `v${versions[req.id] ?? 1}-${req.id}`,
				...byId_[req.id],
			}),
		};
	};
}

const pair = (leftId: string, rightId: string) => ({
	left: prospect({ id: leftId, displayName: `業者${leftId}`, contacts: [] }),
	right: prospect({ id: rightId, displayName: `業者${rightId}`, contacts: [] }),
	matchedOn: [ProspectMatchReason.EMAIL, ProspectMatchReason.NAME],
});

const mergeCheckbox = () => byId<HTMLInputElement>("adm-merge-ack");
const mergeConfirm = () => buttonByText("確認合併");

test("疑似重複：副標與兩個操作、命中原因；翻頁帶 page_token；沒有截斷橫幅與聯絡方式", async () => {
	const tokens: string[] = [];
	const transport = transportWith({
		listProspectDuplicates: (req: { pageToken: string }) => {
			tokens.push(req.pageToken);
			return req.pageToken === ""
				? { duplicates: [pair("7", "9")], nextPageToken: "t2" }
				: { duplicates: [pair("12", "13")], nextPageToken: "" };
		},
	});
	const m = open(h(DuplicatesPage), transport);
	await m.render();
	assert.deepEqual(tokens, [""]);
	assert.match(bodyText(), /進行中的業者之間名稱相同，或聯絡方式/u);
	assert.match(bodyText(), /電子郵件相同、名稱相同/u);
	assert.equal(buttonsByText("比對並合併").length, 1);
	assert.equal(buttonsByText("不是同一家").length, 1);
	assert.doesNotMatch(bodyText(), /500|未留聯絡方式/u);
	assert.match(bodyText(), /統編 · 未填寫/u);
	await click("下一頁");
	await until(() => tokens.length === 2, "第二頁");
	assert.deepEqual(tokens, ["", "t2"]);
	await until(() => bodyText().includes("業者12"), "第二頁的列");
	assert.match(bodyText(), /第 2 頁/u);
});

test("標記不是同一家：確認框的文案、送出的兩個 ID 等於該列、成功提示後清單重讀", async () => {
	const marks: { first: string; second: string }[] = [];
	let lists = 0;
	const transport = transportWith({
		listProspectDuplicates: () => {
			lists += 1;
			return {
				duplicates: lists === 1 ? [pair("7", "9")] : [],
				nextPageToken: "",
			};
		},
		markProspectsDistinct: (req: unknown) => {
			const r = create(MarkProspectsDistinctRequestSchema, req as never);
			marks.push({ first: r.firstProspectId, second: r.secondProspectId });
			return {};
		},
	});
	const m = open(h(DuplicatesPage), transport);
	await m.render();
	await click("不是同一家");
	assert.match(bodyText(), /確定這兩筆不是同一家嗎？/u);
	assert.match(bodyText(), /標記無法在畫面上取消，也不會再提示這兩筆可以合併/u);
	await click("確認不是同一家");
	await until(() => marks.length === 1, "送出");
	assert.deepEqual(marks, [{ first: "7", second: "9" }]);
	await until(() => bodyText().includes("已標記為不是同一家"), "成功提示");
	await until(() => lists === 2, "清單重讀");
	await until(
		() => bodyText().includes("目前沒有疑似重複的業者"),
		"重讀後該列消失",
	);
	assert.doesNotMatch(bodyText(), /確定這兩筆不是同一家嗎？/u);
});

test("標記不是同一家失敗：框內顯示伺服器句子，清單也重讀", async () => {
	let lists = 0;
	const transport = transportWith({
		listProspectDuplicates: () => {
			lists += 1;
			return { duplicates: [pair("7", "9")], nextPageToken: "" };
		},
		markProspectsDistinct: () => {
			throw failure(Code.FailedPrecondition, "業者「業者9」目前不是進行中");
		},
	});
	const m = open(h(DuplicatesPage), transport);
	await m.render();
	await click("不是同一家");
	await click("確認不是同一家");
	await until(() => bodyText().includes("目前不是進行中"), "伺服器句子");
	await until(() => lists === 2, "清單重讀");
	assert.match(bodyText(), /確定這兩筆不是同一家嗎？/u);
});

async function openMerge(transport: Transport) {
	const m = open(h(DuplicatesPage), transport);
	await m.render();
	await click("比對並合併");
	await until(() => maybeById("adm-merge-ack") !== null, "合併框讀完兩邊");
	return m;
}

test("合併：開啟時兩邊各讀一次；送出帶讀到的 etag，切換勝方後兩者對調；成功後提示、關框、清單重讀", async () => {
	const reads: string[] = [];
	const merges: {
		winnerId: string;
		winnerEtag: string;
		loserId: string;
		loserEtag: string;
	}[] = [];
	let lists = 0;
	const transport = transportWith({
		listProspectDuplicates: () => {
			lists += 1;
			return { duplicates: [pair("7", "9")], nextPageToken: "" };
		},
		getProspect: versionedProspects(
			{
				"7": { displayName: "甲農場" },
				"9": { displayName: "乙農場", taxId: "12345678", category: "蔬果" },
			},
			{},
			reads,
		),
		mergeProspects: (req: unknown) => {
			const r = create(MergeProspectsRequestSchema, req as never);
			merges.push({
				winnerId: r.winnerId,
				winnerEtag: r.winnerEtag,
				loserId: r.loserId,
				loserEtag: r.loserEtag,
			});
			return {
				prospect: prospect({
					id: r.winnerId,
					displayName: "乙農場",
					etag: "m1",
				}),
			};
		},
	});
	const m = await openMerge(transport);
	assert.deepEqual([...reads].sort(), ["7", "9"], "兩邊各讀一次");
	assert.match(bodyText(), /比對依據/u);
	assert.match(bodyText(), /合併沒有復原功能/u);
	assert.match(bodyText(), /寄給「甲農場」的詢價連結改由「乙農場」承接/u);
	assert.match(
		bodyText(),
		/「甲農場」的 logo 與封面會刪除，不會搬給「乙農場」/u,
	);
	assert.match(bodyText(), /兩家之間的標記會撤銷/u);
	assert.match(bodyText(), /logo 沒有、封面沒有/u);
	assert.ok(mergeConfirm()?.disabled, "沒勾選不能送出");

	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(() => merges.length === 1, "送出");
	assert.deepEqual(merges[0], {
		winnerId: "9",
		winnerEtag: "v1-9",
		loserId: "7",
		loserEtag: "v1-7",
	});
	await until(() => bodyText().includes("已合併到「乙農場」"), "成功提示");
	assert.doesNotMatch(bodyText(), /確定要合併這兩筆站外業者嗎？/u);
	await until(() => lists >= 2, "清單重讀");
	await m.unmount();
});

test("合併：切換勝方後兩個 etag 對調，並清掉勾選", async () => {
	const merges: { winnerId: string; winnerEtag: string; loserEtag: string }[] =
		[];
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: versionedProspects({ "9": { taxId: "12345678" } }, {}, []),
		mergeProspects: (req: unknown) => {
			const r = create(MergeProspectsRequestSchema, req as never);
			merges.push({
				winnerId: r.winnerId,
				winnerEtag: r.winnerEtag,
				loserEtag: r.loserEtag,
			});
			return { prospect: prospect({ id: r.winnerId }) };
		},
	});
	await openMerge(transport);
	await clickElement(mergeCheckbox());
	assert.equal(mergeCheckbox().checked, true);
	await click("設為勝方");
	assert.equal(mergeCheckbox().checked, false, "換方向清掉勾選");
	assert.ok(mergeConfirm()?.disabled);
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(() => merges.length === 1, "送出");
	assert.deepEqual(merges[0], {
		winnerId: "7",
		winnerEtag: "v1-7",
		loserEtag: "v1-9",
	});
});

test("合併 VERSION_CONFLICT：兩邊重讀、勾選清掉、顯示衝突句；再送出帶新的 etag", async () => {
	const reads: string[] = [];
	const versions: Record<string, number> = { "7": 1, "9": 1 };
	const merges: { winnerEtag: string; loserEtag: string }[] = [];
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: versionedProspects(
			{ "9": { taxId: "12345678" } },
			versions,
			reads,
		),
		mergeProspects: (req: unknown) => {
			const r = create(MergeProspectsRequestSchema, req as never);
			merges.push({ winnerEtag: r.winnerEtag, loserEtag: r.loserEtag });
			if (merges.length === 1) {
				versions["7"] = 2;
				versions["9"] = 2;
				throw versionConflict();
			}
			return { prospect: prospect({ id: r.winnerId }) };
		},
	});
	await openMerge(transport);
	assert.equal(reads.length, 2);
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(
		() =>
			bodyText().includes(
				"其中一筆已被其他操作更新。畫面已重新讀取，請重新核對後再合併。",
			),
		"衝突句",
	);
	assert.equal(reads.length, 4, "兩邊各重讀一次");
	assert.equal(mergeCheckbox().checked, false, "勾選清掉");
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(() => merges.length === 2, "再送出");
	assert.deepEqual(merges[1], { winnerEtag: "v2-9", loserEtag: "v2-7" });
});

test("合併沒收到回應：兩邊各重讀一次，顯示無法確認是否完成", async () => {
	const reads: string[] = [];
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: versionedProspects({}, {}, reads),
	});
	await openMerge(noResponseOn(transport, "MergeProspects"));
	assert.equal(reads.length, 2);
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(
		() => bodyText().includes("無法確認合併是否完成。畫面已重新讀取"),
		"新句子",
	);
	assert.equal(reads.length, 4, "兩邊各重讀一次");
	assert.doesNotMatch(bodyText(), /都沒有變動/u);
});

test("合併沒收到回應：清單類查詢也被重新抓取；重讀後有一邊已不是進行中就停用送出", async () => {
	const reads: string[] = [];
	let lists = 0;
	let merged = false;
	const transport = transportWith({
		listProspectDuplicates: () => {
			lists += 1;
			return { duplicates: [pair("7", "9")], nextPageToken: "" };
		},
		getProspect: (req: { id: string }) => {
			reads.push(req.id);
			return {
				prospect: prospect({
					id: req.id,
					displayName: `業者${req.id}`,
					status:
						merged && req.id === "9"
							? ProspectStatus.ARCHIVED
							: ProspectStatus.ACTIVE,
				}),
			};
		},
		mergeProspects: () => {
			merged = true;
			return { prospect: prospect({ id: "7" }) };
		},
	});
	await openMerge(noResponseOn(transport, "MergeProspects"));
	const listsBefore = lists;
	merged = true;
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(
		() => bodyText().includes("無法確認合併是否完成。畫面已重新讀取"),
		"新句子",
	);
	await until(() => lists > listsBefore, "清單類查詢被重新抓取");
	assert.ok(mergeConfirm()?.disabled, "有一邊已不是進行中，停用送出");
});

test("合併其他失敗：顯示伺服器句子並重讀", async () => {
	const reads: string[] = [];
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: versionedProspects({}, {}, reads),
		mergeProspects: () => {
			throw failure(Code.FailedPrecondition, "兩筆業者都必須是進行中");
		},
	});
	await openMerge(transport);
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(
		() => bodyText().includes("兩筆業者都必須是進行中"),
		"伺服器句子",
	);
	assert.equal(reads.length, 4);
});

test("合併讀到有一邊不是進行中：停用送出並說明；讀取失敗可重試", async () => {
	let reads = 0;
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: (req: { id: string }) => {
			reads += 1;
			if (reads === 1) throw failure(Code.Internal, "boom");
			return {
				prospect: prospect({
					id: req.id,
					displayName: `業者${req.id}`,
					status:
						req.id === "9" ? ProspectStatus.CONVERTED : ProspectStatus.ACTIVE,
				}),
			};
		},
	});
	const m = open(h(DuplicatesPage), transport);
	await m.render();
	await click("比對並合併");
	await until(() => bodyText().includes("無法讀取業者資料。"), "讀取失敗");
	await click("重試");
	await until(() => maybeById("adm-merge-ack") !== null, "重試後讀到");
	assert.match(bodyText(), /「業者9」目前不是進行中，不能合併。/u);
	assert.ok(mergeConfirm()?.disabled);
	await clickElement(mergeCheckbox());
	assert.ok(mergeConfirm()?.disabled, "勾選也不能送出");
});

test("合併：快取已有資料但重讀失敗 → 顯示讀取失敗與重試，確認停用", async () => {
	let failReads = false;
	const transport = transportWith({
		listProspectDuplicates: () => ({
			duplicates: [pair("7", "9")],
			nextPageToken: "",
		}),
		getProspect: (req: { id: string }) => {
			if (failReads) throw failure(Code.Internal, "boom");
			return {
				prospect: prospect({ id: req.id, displayName: `業者${req.id}` }),
			};
		},
		mergeProspects: () => {
			throw failure(Code.FailedPrecondition, "兩筆業者都必須是進行中");
		},
	});
	await openMerge(transport);
	assert.ok(maybeById("adm-merge-ack"), "先讀到資料");
	failReads = true;
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(
		() => bodyText().includes("無法讀取業者資料。"),
		"重讀失敗後顯示讀取失敗",
	);
	assert.ok(buttonByText("重試"), "有重試");
	assert.ok(mergeConfirm()?.disabled, "確認停用");
});

async function openDetail(transport: Transport) {
	navigation.params = { id: "7" };
	const m = open(h(DetailPage), transport);
	await m.render();
	return m;
}

const rowNames = () =>
	[...doc().querySelectorAll("li a")].map((a) => a.textContent);

function editTransport(overrides: Handlers, others: Record<string, Init> = {}) {
	return transportWith({
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id, ...(others[req.id] ?? {}) }),
		}),
		previewProspect: () => ({ matches: [] }),
		...overrides,
	});
}

test("修改撞到相符業者：進行中的列有「與這家合併」、已入駐的沒有並說明；按下後取消重送、合併框以兩個 ID 開啟並提醒草稿", async () => {
	const updates: number[] = [];
	const reads: string[] = [];
	const transport = transportWith({
		getProspect: (req: { id: string }) => {
			reads.push(req.id);
			return {
				prospect: prospect({
					id: req.id,
					displayName: `業者${req.id}`,
					status:
						req.id === "12" ? ProspectStatus.CONVERTED : ProspectStatus.ACTIVE,
					region: req.id === "11" ? "嘉義" : "",
					contacts: [],
				}),
			};
		},
		previewProspect: () => ({ matches: [] }),
		updateProspect: () => {
			updates.push(1);
			throw matchFound("12,11");
		},
	});
	await openDetail(transport);
	await type(byId("prospect-edit-display-name"), "新名稱");
	await click("儲存變更");
	await until(() => rowNames().length === 2, "相符業者");
	assert.match(
		bodyText(),
		/是同一家的話：進行中的可以與它合併，已入駐的請封存這筆/u,
	);
	assert.equal(buttonsByText("與這家合併").length, 1, "只有進行中的有合併鍵");
	assert.match(bodyText(), /已入駐，不能合併/u);

	reads.length = 0;
	await click("與這家合併");
	await until(() => maybeById("adm-merge-ack") !== null, "合併框");
	assert.doesNotMatch(bodyText(), /可能是同一家業者/u, "相符對話框已取消");
	assert.deepEqual([...reads].sort(), ["11", "7"]);
	assert.match(bodyText(), /這筆業者尚未儲存的修改不會保留。/u);
	assert.equal(updates.length, 1, "只送過一次，合併前沒有重送");
	const winnerCard = doc().querySelector(
		".adm-operation-identity-card--target",
	)?.textContent;
	assert.match(
		winnerCard ?? "",
		/業者11/u,
		"預設勝方依完整度（完整度高的是別筆、ID 也較大）",
	);
});

test("修改撞到統編：持有者進行中 → 提示有合併按鈕，開出的合併框預設勝方是持有者", async () => {
	const merges: { winnerId: string; loserId: string }[] = [];
	const transport = editTransport(
		{
			updateProspect: () => {
				throw taxIdHeld("11");
			},
			mergeProspects: (req: unknown) => {
				const r = create(MergeProspectsRequestSchema, req as never);
				merges.push({ winnerId: r.winnerId, loserId: r.loserId });
				return { prospect: prospect({ id: r.winnerId }) };
			},
		},
		{ "11": { displayName: "持有者農場", contacts: [], region: "" } },
	);
	await openDetail(transport);
	await type(byId("prospect-edit-tax-id"), "12345678");
	await click("儲存變更");
	await until(
		() => bodyText().includes("統一編號已由業者 11 使用，該做的是合併"),
		"伺服器句子",
	);
	await until(
		() => buttonByText("與「持有者農場」合併") !== undefined,
		"統編出口",
	);
	await click("與「持有者農場」合併");
	await until(() => maybeById("adm-merge-ack") !== null, "合併框");
	assert.match(bodyText(), /這筆業者尚未儲存的修改不會保留。/u);
	assert.match(
		doc().querySelector(".adm-operation-identity-card--target")?.textContent ??
			"",
		/持有者農場/u,
		"勝方是持有者，不是完整度較高的目前這筆",
	);
	await until(() => !mergeCheckbox().disabled, "兩邊讀完");
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(() => merges.length === 1, "送出");
	assert.deepEqual(merges[0], { winnerId: "11", loserId: "7" });
});

test("修改撞到統編：持有者已入駐、沒有 reason 的 failed_precondition、別的 reason 帶同樣的 metadata，都只有句子沒有合併按鈕", async () => {
	const failures = [
		taxIdHeld("11"),
		failure(Code.FailedPrecondition, "只有進行中的業者可以修改"),
		failure(Code.FailedPrecondition, "賣家已停權", "SELLER_SUSPENDED", {
			prospectIds: "11",
		}),
	];
	const texts = [
		"統一編號已由業者 11 使用",
		"只有進行中的業者可以修改",
		"賣家已停權",
	];
	for (const [index, error] of failures.entries()) {
		const holderReads = { n: 0 };
		const transport = editTransport(
			{
				getProspect: (req: { id: string }) => {
					if (req.id === "11") holderReads.n += 1;
					return {
						prospect: prospect({
							id: req.id,
							...(req.id === "11"
								? {
										displayName: "持有者農場",
										status:
											index === 0
												? ProspectStatus.CONVERTED
												: ProspectStatus.ACTIVE,
									}
								: {}),
						}),
					};
				},
				updateProspect: () => {
					throw error;
				},
			},
			{
				"11": {
					displayName: "持有者農場",
					status:
						index === 0 ? ProspectStatus.CONVERTED : ProspectStatus.ACTIVE,
				},
			},
		);
		const m = await openDetail(transport);
		await type(byId("prospect-edit-tax-id"), "12345678");
		await click("儲存變更");
		await until(() => bodyText().includes(texts[index]), `句子 ${index}`);
		if (index === 0) {
			// 只有第一個情境會去讀持有者；等到讀取發生，再讓畫面吃下結果（以輪詢的次數，不以時間）
			await until(() => holderReads.n > 0, "持有者的 GetProspect 被呼叫");
			let ticks = 0;
			await until(() => ++ticks > 5, "持有者的結果渲染");
		}
		assert.equal(
			[...doc().querySelectorAll("button")].some((b) =>
				(b.textContent ?? "").includes("合併"),
			),
			false,
			`情境 ${index} 沒有合併按鈕`,
		);
		await m.unmount();
	}
});

test("相符對話框的重送撞到 PROSPECT_TAX_ID_HELD：對話框關閉，編輯頁出現統編出口", async () => {
	let updates = 0;
	const transport = editTransport(
		{
			updateProspect: () => {
				updates += 1;
				throw updates === 1 ? matchFound("11") : taxIdHeld("12");
			},
		},
		{
			"11": { displayName: "甲農場" },
			"12": { displayName: "持有者農場" },
		},
	);
	await openDetail(transport);
	await type(byId("prospect-edit-display-name"), "新名稱");
	await click("儲存變更");
	await until(() => rowNames().length === 1, "相符業者");
	await clickElement(
		doc().querySelector('input[type="checkbox"]') as HTMLInputElement,
	);
	await click("確認不是同一家並儲存");
	await until(() => updates === 2, "重送");
	await until(
		() => buttonByText("與「持有者農場」合併") !== undefined,
		"統編出口",
	);
	assert.doesNotMatch(bodyText(), /可能是同一家業者/u);
	assert.match(bodyText(), /統一編號已由業者 12 使用/u);
});

test("從編輯頁合併成功：目前這筆是敗方 → 導到勝方詳情；是勝方 → 表單以新版本重設，不出現衝突提示", async () => {
	const merged = { id: "" };
	const detailReads = { n: 0 };
	const transport = editTransport(
		{
			getProspect: (req: { id: string }) => {
				if (req.id === "7") detailReads.n += 1;
				return {
					prospect: prospect({
						id: req.id,
						...(req.id === "11"
							? { displayName: "甲農場", contacts: [], region: "" }
							: {}),
					}),
				};
			},
			updateProspect: () => {
				throw matchFound("11");
			},
			mergeProspects: (req: unknown) => {
				const r = create(MergeProspectsRequestSchema, req as never);
				merged.id = r.winnerId;
				return {
					prospect: prospect({
						id: r.winnerId,
						etag: "m9",
						displayName: "合併後的名稱",
					}),
				};
			},
		},
		{ "11": { displayName: "甲農場", contacts: [], region: "" } },
	);

	// 目前這筆（7）是敗方
	let m = await openDetail(transport);
	await type(byId("prospect-edit-display-name"), "新名稱");
	await click("儲存變更");
	await until(() => rowNames().length === 1, "相符業者");
	await click("與這家合併");
	await until(() => maybeById("adm-merge-ack") !== null, "合併框");
	await click("設為勝方");
	await clickElement(mergeCheckbox());
	const readsBeforeMerge = detailReads.n;
	await click("確認合併");
	await until(() => navigation.pushed.length === 1, "導到勝方");
	assert.equal(merged.id, "11");
	await until(() => detailReads.n > readsBeforeMerge, "敗方的詳情查詢失效重讀");
	assert.deepEqual(navigation.pushed, ["/admin/seller-prospects/11"]);
	await m.unmount();
	navigation.pushed = [];

	// 目前這筆（7）是勝方
	m = await openDetail(transport);
	await type(byId("prospect-edit-display-name"), "新名稱");
	await click("儲存變更");
	await until(() => rowNames().length === 1, "相符業者");
	await click("與這家合併");
	await until(() => maybeById("adm-merge-ack") !== null, "合併框");
	await clickElement(mergeCheckbox());
	await click("確認合併");
	await until(() => merged.id === "7", "送出");
	await until(
		() =>
			(byId<HTMLInputElement>("prospect-edit-display-name").value ?? "") ===
			"合併後的名稱",
		"表單以回應重設",
	);
	await until(
		() => buttonByText("儲存變更")?.disabled === true,
		"草稿已被新版本取代",
	);
	assert.doesNotMatch(bodyText(), /你輸入的內容還在/u);
	assert.deepEqual(navigation.pushed, []);
	await m.unmount();
});

function taxIdMatch(storeId = "s1") {
	return {
		prospect: prospect({
			id: "7",
			displayName: "阿里山農場",
			taxId: "12345678",
			contacts: [],
		}),
		storeId,
		storeName: "阿里山商店",
		companyName: "阿里山有限公司",
		kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
	};
}

test("統編對帳：認證狀態轉文字、翻頁帶 page_token；開確認框時讀該業者，讀到前不能確認，讀到後顯示聯絡方式", async () => {
	const tokens: string[] = [];
	const reads: string[] = [];
	const transport = transportWith({
		listProspectTaxIDMatches: (req: { pageToken: string }) => {
			tokens.push(req.pageToken);
			return req.pageToken === ""
				? { matches: [taxIdMatch()], nextPageToken: "t2" }
				: { matches: [], nextPageToken: "" };
		},
		getProspect: (req: { id: string }) => {
			reads.push(req.id);
			return { prospect: prospect({ id: req.id, etag: "e1" }) };
		},
	});
	const m = open(h(ReconciliationPage), transport);
	await m.render();
	assert.match(bodyText(), /待審核/u);
	assert.doesNotMatch(bodyText(), /已處理/u);
	assert.deepEqual(reads, [], "沒開框不讀");
	await click("比對並綁定");
	await until(() => bodyText().includes("a@x.test"), "讀到聯絡方式");
	assert.deepEqual(reads, ["7"]);
	assert.match(bodyText(), /認證狀態 · 待審核/u);
	await click("下一頁");
	await until(() => tokens.length === 2, "第二頁");
	assert.deepEqual(tokens, ["", "t2"]);
});

test("統編對帳：already_exists → 關框、頁面顯示伺服器句子、清單重讀", async () => {
	let lists = 0;
	const converts: string[] = [];
	const transport = transportWith({
		listProspectTaxIDMatches: () => {
			lists += 1;
			return { matches: [taxIdMatch()], nextPageToken: "" };
		},
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id }),
		}),
		convertProspect: (req: unknown) => {
			converts.push(create(ConvertProspectRequestSchema, req as never).etag);
			throw failure(Code.AlreadyExists, "這家店已被其他業者認領");
		},
	});
	const m = open(h(ReconciliationPage), transport);
	await m.render();
	await click("比對並綁定");
	await until(() => bodyText().includes("a@x.test"), "讀到業者");
	await clickElement(byId("adm-convert-ack"));
	await click("確認歸因綁定");
	await until(() => converts.length === 1, "送出");
	await until(() => lists === 2, "清單重讀");
	assert.doesNotMatch(bodyText(), /確定要將這筆站外業者歸因綁定/u, "框關閉");
	assert.match(bodyText(), /這家店已被其他業者認領/u);
});

test("統編對帳 VERSION_CONFLICT：留在框內、重讀那一筆，重送帶的是新的 etag", async () => {
	let version = 1;
	let reads = 0;
	const converts: { etag: string; storeId: string; id: string }[] = [];
	const transport = transportWith({
		listProspectTaxIDMatches: () => ({
			matches: [taxIdMatch()],
			nextPageToken: "",
		}),
		getProspect: (req: { id: string }) => {
			reads += 1;
			return { prospect: prospect({ id: req.id, etag: `e${version}` }) };
		},
		convertProspect: (req: unknown) => {
			const r = create(ConvertProspectRequestSchema, req as never);
			converts.push({ etag: r.etag, storeId: r.storeId, id: r.id });
			if (converts.length === 1) {
				version = 2;
				throw versionConflict();
			}
			return {
				prospect: prospect({ id: r.id, status: ProspectStatus.CONVERTED }),
			};
		},
	});
	const m = open(h(ReconciliationPage), transport);
	await m.render();
	await click("比對並綁定");
	await until(() => bodyText().includes("a@x.test"), "讀到業者");
	await clickElement(byId("adm-convert-ack"));
	await click("確認歸因綁定");
	await until(
		() =>
			bodyText().includes(
				"資料已被其他操作更新。畫面已重新讀取，請確認最新狀態後再操作。",
			),
		"衝突句",
	);
	assert.match(bodyText(), /確定要將這筆站外業者歸因綁定/u, "留在框內");
	assert.equal(reads, 2, "重讀那一筆");
	await until(() => buttonByText("確認歸因綁定")?.disabled === false, "可再送");
	await click("確認歸因綁定");
	await until(() => converts.length === 2, "重送");
	assert.deepEqual(converts[0], { etag: "e1", storeId: "s1", id: "7" });
	assert.deepEqual(converts[1], { etag: "e2", storeId: "s1", id: "7" });
	await until(() => bodyText().includes("已轉成店家"), "成功提示");
});

test("統編對帳沒收到回應：留在框內，顯示後備句，可再送", async () => {
	const transport = transportWith({
		listProspectTaxIDMatches: () => ({
			matches: [taxIdMatch()],
			nextPageToken: "",
		}),
		getProspect: (req: { id: string }) => ({
			prospect: prospect({ id: req.id }),
		}),
	});
	const m = open(
		h(ReconciliationPage),
		noResponseOn(transport, "ConvertProspect"),
	);
	await m.render();
	await click("比對並綁定");
	await until(() => bodyText().includes("a@x.test"), "讀到業者");
	await clickElement(byId("adm-convert-ack"));
	await click("確認歸因綁定");
	await until(
		() => bodyText().includes("歸因綁定失敗，請確認網路狀態後再試一次。"),
		"後備句",
	);
	assert.match(bodyText(), /確定要將這筆站外業者歸因綁定/u);
	assert.equal(buttonByText("確認歸因綁定")?.disabled, false);
});

test("統編對帳：讀不到業者時顯示讀取失敗與重試，不能確認", async () => {
	let reads = 0;
	const transport = transportWith({
		listProspectTaxIDMatches: () => ({
			matches: [taxIdMatch()],
			nextPageToken: "",
		}),
		getProspect: (req: { id: string }) => {
			reads += 1;
			if (reads === 1) throw failure(Code.Internal, "boom");
			return { prospect: prospect({ id: req.id }) };
		},
	});
	const m = open(h(ReconciliationPage), transport);
	await m.render();
	await click("比對並綁定");
	await until(() => bodyText().includes("無法讀取業者資料。"), "讀取失敗");
	await clickElement(byId("adm-convert-ack"));
	assert.ok(buttonByText("確認歸因綁定")?.disabled);
	await click("重試");
	await until(() => bodyText().includes("a@x.test"), "重試後讀到");
	assert.equal(buttonByText("確認歸因綁定")?.disabled, false);
});
