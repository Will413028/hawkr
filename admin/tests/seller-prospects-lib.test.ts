import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	ContactKind,
	OutreachStatus,
	ProspectMatchReason,
	ProspectMatchTier,
	ProspectSchema,
	ProspectStatus,
	VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import {
	addContact,
	buildContacts,
	CONTACT_KIND_OPTIONS,
	type ContactRow,
	canReuseEtag,
	changeContactKind,
	compareIds,
	contactKindText,
	defaultWinnerId,
	EDITABLE_OUTREACH_STATUSES,
	emptyProspectForm,
	filterParam,
	hasActiveFilter,
	listProspectsFields,
	MAX_CONTACTS,
	matchReasonsText,
	matchReasonText,
	matchTierText,
	mergeDraftOntoIncoming,
	normalizePrimaries,
	outreachStatusText,
	type ProspectForm,
	parsePriorityStars,
	previewRequest,
	profileCompleteness,
	prospectFormError,
	prospectInit,
	prospectStatusText,
	prospectToForm,
	readProspectFilters,
	removeContact,
	setPrimaryContact,
	sortIdsNumerically,
	updateMaskPaths,
	vendorTypeText,
} from "../src/lib/seller-prospects";
import { unknownText } from "../src/lib/ui-text";

const row = (
	key: string,
	kind: ContactKind,
	value: string,
	primary = false,
): ContactRow => ({ key, kind, value, primary });

const primaries = (rows: ContactRow[]) =>
	rows.filter((r) => r.primary).map((r) => r.key);

test("聯絡方式：新增的列預設是電子郵件，同類型只有一列時它就是主要", () => {
	const rows = addContact([], "a");
	assert.equal(rows[0].kind, ContactKind.EMAIL);
	assert.equal(rows[0].primary, true);
	const two = addContact(rows, "b");
	assert.deepEqual(primaries(two), ["a"]);
});

test("聯絡方式：每個類型恰一個主要（沒有時第一列遞補、多個時留第一個）", () => {
	const rows = normalizePrimaries([
		row("a", ContactKind.EMAIL, "a@x.test"),
		row("b", ContactKind.EMAIL, "b@x.test"),
		row("c", ContactKind.PHONE, "02-1", true),
		row("d", ContactKind.PHONE, "02-2", true),
	]);
	assert.deepEqual(primaries(rows), ["a", "c"]);
});

test("聯絡方式：設成主要時同類型其他列取消，不影響別的類型", () => {
	const rows = setPrimaryContact(
		[
			row("a", ContactKind.EMAIL, "a@x.test", true),
			row("b", ContactKind.EMAIL, "b@x.test"),
			row("c", ContactKind.PHONE, "02-1", true),
		],
		"b",
	);
	assert.deepEqual(primaries(rows), ["b", "c"]);
});

test("聯絡方式：刪掉主要的那一列，同類型的第一列遞補", () => {
	const rows = removeContact(
		[
			row("a", ContactKind.EMAIL, "a@x.test"),
			row("b", ContactKind.EMAIL, "b@x.test"),
			row("c", ContactKind.EMAIL, "c@x.test", true),
		],
		"c",
	);
	assert.deepEqual(primaries(rows), ["a"]);
	assert.deepEqual(primaries(removeContact(rows, "a")), ["b"]);
	assert.deepEqual(primaries(removeContact(removeContact(rows, "a"), "b")), []);
});

test("聯絡方式：改類型時兩個類型的主要都重算", () => {
	const start = [
		row("a", ContactKind.EMAIL, "a@x.test", true),
		row("b", ContactKind.EMAIL, "b@x.test"),
		row("p", ContactKind.PHONE, "02-1", true),
		row("q", ContactKind.PHONE, "02-2"),
	];
	// 主要的 a 改成電話：原類型（電子郵件）由 b 遞補；電話已有主要 p，a 不是主要。
	const moved = changeContactKind(start, "a", ContactKind.PHONE);
	assert.deepEqual(primaries(moved), ["b", "p"]);
	assert.equal(moved.find((r) => r.key === "a")?.primary, false);
	// 新類型沒有其他列時，改過去的那一列成為主要；原類型沒有列了就沒有主要。
	const alone = changeContactKind(
		[row("a", ContactKind.EMAIL, "a@x.test", true)],
		"a",
		ContactKind.WEBSITE,
	);
	assert.deepEqual(primaries(alone), ["a"]);
	// 非主要的列改類型：原類型的主要不變。
	const nonPrimary = changeContactKind(start, "b", ContactKind.WEBSITE);
	assert.deepEqual(primaries(nonPrimary), ["a", "b", "p"]);
});

test("聯絡方式：去空白、依類型檢查長度（以字數）、最多 50 筆", () => {
	const ok = buildContacts([row("a", ContactKind.EMAIL, "  a@x.test　", true)]);
	assert.deepEqual(ok, {
		ok: true,
		contacts: [{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true }],
	});
	const limits: [ContactKind, number][] = [
		[ContactKind.EMAIL, 255],
		[ContactKind.PHONE, 50],
		[ContactKind.WEBSITE, 500],
		[ContactKind.SOCIAL, 500],
	];
	for (const [kind, max] of limits) {
		assert.equal(
			buildContacts([row("a", kind, "字".repeat(max), true)]).ok,
			true,
		);
		const tooLong = buildContacts([row("a", kind, "字".repeat(max + 1), true)]);
		assert.deepEqual(tooLong, {
			ok: false,
			error: `聯絡方式的值必須是 1 到 ${max} 字`,
		});
	}
	assert.equal(
		buildContacts([row("a", ContactKind.EMAIL, "  ", true)]).ok,
		false,
	);
	const many = Array.from({ length: MAX_CONTACTS + 1 }, (_, i) =>
		row(`k${i}`, ContactKind.EMAIL, `u${i}@x.test`, i === 0),
	);
	assert.deepEqual(buildContacts(many), {
		ok: false,
		error: "聯絡方式最多 50 個",
	});
	assert.equal(buildContacts(many.slice(0, MAX_CONTACTS)).ok, true);
});

const prospect = (
	init: Parameters<typeof create<typeof ProspectSchema>>[1] = {},
) =>
	create(ProspectSchema, {
		id: "7",
		etag: "e1",
		displayName: "阿里山農場",
		vendorType: VendorType.BUSINESS,
		status: ProspectStatus.ACTIVE,
		outreachStatus: OutreachStatus.INVITED,
		taxId: "12345678",
		contacts: [
			{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true },
			{ kind: ContactKind.PHONE, value: "02-1", primary: true },
		],
		categoryIds: ["c1"],
		category: "水果",
		region: "嘉義",
		priorityStars: 3,
		...init,
	});

test("update mask：沒有改動回空陣列，呼叫端不送出", () => {
	const base = prospectToForm(prospect());
	assert.deepEqual(updateMaskPaths(base, structuredClone(base)), []);
	assert.deepEqual(
		updateMaskPaths(base, { ...base, displayName: "  阿里山農場 " }),
		[],
	);
});

test('update mask：只列改過的欄位，不含 "*" 與 category_ids', () => {
	const base = prospectToForm(prospect());
	const edited: ProspectForm = {
		...base,
		displayName: "阿里山有機農場",
		outreachStatus: OutreachStatus.REPLIED,
		region: "",
		priorityStars: "5",
		vendorType: VendorType.INDIVIDUAL,
		categoryIds: ["c1", "c2"],
	};
	const paths = updateMaskPaths(base, edited);
	assert.deepEqual(paths.slice().sort(), [
		"display_name",
		"outreach_status",
		"priority_stars",
		"region",
		"vendor_type",
	]);
	assert.equal(paths.includes("*" as never), false);
	assert.equal(paths.includes("category_ids" as never), false);
});

test("update mask：清空文字欄位會列出該欄位；聯絡方式整批列 contacts、與順序無關", () => {
	const base = prospectToForm(prospect());
	assert.deepEqual(updateMaskPaths(base, { ...base, category: "" }), [
		"category",
	]);
	assert.deepEqual(
		updateMaskPaths(base, {
			...base,
			contacts: [...base.contacts].reverse(),
		}),
		[],
	);
	assert.deepEqual(
		updateMaskPaths(base, {
			...base,
			contacts: setPrimaryContact(
				[...base.contacts, row("n", ContactKind.EMAIL, "n@x.test")],
				"n",
			),
		}),
		["contacts"],
	);
	assert.deepEqual(updateMaskPaths(base, { ...base, priorityStars: "" }), [
		"priority_stars",
	]);
});

test("E5 沿用 etag：圖片以外全部可寫欄位都等於基準才沿用", () => {
	const base = prospectToForm(prospect());
	const withImages = prospectToForm(
		prospect({
			etag: "e2",
			logoImage: { uriTemplate: "https://m.test/i/{preset}/x.png" },
			logoSourceUrl: "https://src.test/",
		}),
	);
	assert.equal(canReuseEtag(base, withImages), true);
	const changes: Parameters<typeof prospect>[0][] = [
		{ displayName: "別人改的名稱" },
		{ vendorType: VendorType.INDIVIDUAL },
		{ outreachStatus: OutreachStatus.DECLINED },
		{ priorityStars: 4 },
		{ taxId: "87654321" },
		{ productSummary: "新摘要" },
		{
			contacts: [{ kind: ContactKind.EMAIL, value: "z@x.test", primary: true }],
		},
		{ categoryIds: ["c1", "c9"] },
	];
	for (const change of changes) {
		assert.equal(
			canReuseEtag(base, prospectToForm(prospect({ etag: "e2", ...change }))),
			false,
			JSON.stringify(change),
		);
	}
});

test("衝突後的草稿：管理員改過的欄位保留，沒碰的欄位換成新版本，之後的 mask 只含自己改的", () => {
	const base = prospectToForm(prospect());
	const draft: ProspectForm = { ...base, displayName: "我改的名稱" };
	const incoming = prospectToForm(
		prospect({ etag: "e2", region: "花蓮", displayName: "別人改的名稱" }),
	);
	const merged = mergeDraftOntoIncoming(base, draft, incoming);
	assert.equal(merged.displayName, "我改的名稱");
	assert.equal(merged.region, "花蓮");
	assert.deepEqual(updateMaskPaths(incoming, merged), ["display_name"]);
});

test("表單：新增表單預設型態是個人、招商進度不送；星等只收 0 以上的整數", () => {
	const form = emptyProspectForm();
	assert.equal(form.vendorType, VendorType.INDIVIDUAL);
	assert.equal(prospectFormError(form), "請填寫業者名稱。");
	const init = prospectInit(
		{ ...form, displayName: " 店 " },
		{ includeOutreach: false },
	);
	assert.equal(init.outreachStatus, undefined);
	assert.equal(init.displayName, "店");
	assert.equal(init.priorityStars, 0);
	assert.equal(parsePriorityStars(""), 0);
	assert.equal(parsePriorityStars(" 5 "), 5);
	for (const bad of ["-1", "1.5", "abc", "5星", "9999999999"]) {
		assert.equal(parsePriorityStars(bad), null, bad);
	}
	assert.equal(
		prospectFormError({ ...form, displayName: "店", taxId: "123" }),
		"統一編號須為 8 碼數字",
	);
});

test("即時查重的內容：名稱沒填不查；格式不對的統編與空的聯絡方式不送", () => {
	const form = emptyProspectForm();
	assert.equal(previewRequest(form), null);
	assert.deepEqual(
		previewRequest({
			...form,
			displayName: " 店 ",
			taxId: "12",
			contacts: [
				row("a", ContactKind.EMAIL, "a@x.test", true),
				row("b", ContactKind.EMAIL, "  ", false),
			],
		}),
		{
			displayName: "店",
			taxId: "",
			contacts: [{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true }],
		},
	);
});

function everyValue<E extends number>(
	enumObject: Record<string, string | E>,
): E[] {
	return Object.values(enumObject).filter((v): v is E => typeof v === "number");
}

test("enum 表：每個非 UNSPECIFIED 的值都有文字，UNSPECIFIED 與不認得的值是「未知」", () => {
	const cases: [string, number[], (v: number) => string][] = [
		["ProspectStatus", everyValue(ProspectStatus), prospectStatusText],
		["VendorType", everyValue(VendorType), vendorTypeText],
		["OutreachStatus", everyValue(OutreachStatus), outreachStatusText],
		["ContactKind", everyValue(ContactKind), contactKindText],
		["ProspectMatchTier", everyValue(ProspectMatchTier), matchTierText],
		["ProspectMatchReason", everyValue(ProspectMatchReason), matchReasonText],
	];
	for (const [name, values, text] of cases) {
		assert.ok(values.length > 1, name);
		for (const value of values) {
			if (value === 0) {
				assert.equal(text(value), unknownText(), `${name} UNSPECIFIED`);
			} else {
				assert.notEqual(text(value), unknownText(), `${name} ${value}`);
			}
		}
		assert.equal(text(999), unknownText(), `${name} 不認得的值`);
	}
	assert.equal(
		matchReasonsText([ProspectMatchReason.EMAIL, ProspectMatchReason.NAME]),
		"電子郵件相同、名稱相同",
	);
	assert.equal(CONTACT_KIND_OPTIONS.length, everyValue(ContactKind).length - 1);
});

test("招商進度的選項不含 UNSPECIFIED 與 CONVERTED", () => {
	assert.deepEqual(
		[...EDITABLE_OUTREACH_STATUSES].sort(),
		everyValue(OutreachStatus)
			.filter(
				(v) =>
					v !== OutreachStatus.UNSPECIFIED && v !== OutreachStatus.CONVERTED,
			)
			.sort(),
	);
});

const params = (query: string) => new URLSearchParams(query);

test("URL 篩選：只認白名單的小寫字串，不認得的回預設，UNSPECIFIED 不會出現", () => {
	assert.deepEqual(readProspectFilters(params("")), {
		status: ProspectStatus.ACTIVE,
		outreachStatus: undefined,
		vendorType: undefined,
		category: "",
		region: "",
		keyword: "",
	});
	const parsed = readProspectFilters(
		params(
			"status=archived&outreachStatus=do_not_contact&vendorType=business&category=水果&region=嘉義&keyword=茶",
		),
	);
	assert.equal(parsed.status, ProspectStatus.ARCHIVED);
	assert.equal(parsed.outreachStatus, OutreachStatus.DO_NOT_CONTACT);
	assert.equal(parsed.vendorType, VendorType.BUSINESS);
	assert.equal(
		readProspectFilters(params("status=converted")).status,
		ProspectStatus.CONVERTED,
	);
	for (const bad of ["", "1", "2", "ACTIVE", "unspecified", "deleted", "%00"]) {
		const f = readProspectFilters(
			params(`status=${bad}&outreachStatus=${bad}&vendorType=${bad}`),
		);
		assert.equal(f.outreachStatus, undefined, bad);
		assert.equal(f.vendorType, undefined, bad);
		assert.equal(f.status, ProspectStatus.ACTIVE, `status=${bad} 應回預設`);
	}
	// 招商進度的 CONVERTED 不在選項裡。
	assert.equal(
		readProspectFilters(params("outreachStatus=converted")).outreachStatus,
		undefined,
	);
	assert.equal(filterParam(ProspectStatus, ProspectStatus.ACTIVE), "active");
	assert.equal(
		filterParam(OutreachStatus, OutreachStatus.NOT_CONTACTED),
		"not_contacted",
	);
});

test("清單請求：狀態送明確的值，沒選的招商進度與型態不送", () => {
	const none = listProspectsFields(readProspectFilters(params("")));
	assert.equal(none.status, ProspectStatus.ACTIVE);
	assert.equal("outreachStatus" in none, false);
	assert.equal("vendorType" in none, false);
	const all = listProspectsFields(
		readProspectFilters(
			params(
				"status=archived&outreachStatus=invited&vendorType=individual&keyword=k",
			),
		),
	);
	assert.equal(all.status, ProspectStatus.ARCHIVED);
	assert.equal(all.outreachStatus, OutreachStatus.INVITED);
	assert.equal(all.vendorType, VendorType.INDIVIDUAL);
	assert.equal(all.keyword, "k");
	assert.equal(hasActiveFilter(readProspectFilters(params(""))), false);
	assert.equal(
		hasActiveFilter(readProspectFilters(params("status=archived"))),
		true,
	);
});

const mergeSide = (init: Parameters<typeof create<typeof ProspectSchema>>[1]) =>
	create(ProspectSchema, init);

test("合併的完整度：聯絡方式以類型數計入，同類型多筆只算一項；0 星等不算填寫", () => {
	assert.equal(
		profileCompleteness(
			mergeSide({
				id: "1",
				contacts: [
					{ kind: ContactKind.EMAIL, value: "a@x.test", primary: true },
					{ kind: ContactKind.EMAIL, value: "b@x.test" },
					{ kind: ContactKind.PHONE, value: "0212345678", primary: true },
				],
				region: "嘉義",
				priorityStars: 0,
			}),
		),
		3,
	);
	assert.equal(
		profileCompleteness(
			mergeSide({ id: "1", taxId: "12345678", priorityStars: 3 }),
		),
		2,
	);
});

test("合併的預設勝方：指定的優先；否則完整度高的；平手取 ID 數值小的", () => {
	const rich = mergeSide({ id: "20", taxId: "12345678", region: "嘉義" });
	const poor = mergeSide({ id: "3" });
	assert.equal(defaultWinnerId(poor, rich), "20", "完整度高的一邊");
	assert.equal(defaultWinnerId(rich, poor), "20");
	assert.equal(defaultWinnerId(poor, rich, "3"), "3", "指定的勝方優先");
	assert.equal(
		defaultWinnerId(poor, rich, "99"),
		"20",
		"指定的不是兩邊之一就忽略",
	);
	const a = mergeSide({ id: "9", region: "A" });
	const b = mergeSide({ id: "10", region: "B" });
	assert.equal(defaultWinnerId(b, a), "9", "平手取數值小的，不是字串較小的");
	assert.equal(defaultWinnerId(a, b), "9");
});

test("ID 以數值比較與排序，超過 Number.MAX_SAFE_INTEGER 也不失精", () => {
	assert.ok(compareIds("9", "10") < 0);
	assert.equal(compareIds("7", "7"), 0);
	assert.ok(
		compareIds("9007199254740993", "9007199254740992") > 0,
		"相差 1 的兩個大數",
	);
	assert.deepEqual(sortIdsNumerically("10", "9"), ["9", "10"]);
	assert.deepEqual(sortIdsNumerically("9", "10"), ["9", "10"]);
});
