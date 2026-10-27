/**
 * 站外業者名單的畫面邏輯：enum 的顯示文字、清單篩選與網址的對應、聯絡方式的主要規則、表單與 update mask、合併的預設勝方。
 */

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	ContactKind,
	type ListProspectsRequestSchema,
	OutreachStatus,
	type PreviewProspectRequestSchema,
	type Prospect,
	ProspectMatchReason,
	ProspectMatchTier,
	type ProspectSchema,
	ProspectStatus,
	VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { unknownText } from "./ui-text";

// ---------------------------------------------------------------------------
// enum 的顯示文字（6 個 enum 都不在共用文案目錄，admin 單端自己寫；UNSPECIFIED 與不認得的值顯示「未知」）

function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

const PROSPECT_STATUS_TEXT: Record<
	Exclude<ProspectStatus, ProspectStatus.UNSPECIFIED>,
	string
> = {
	[ProspectStatus.ACTIVE]: "進行中",
	[ProspectStatus.ARCHIVED]: "已封存",
	[ProspectStatus.CONVERTED]: "已入駐",
};

const VENDOR_TYPE_TEXT: Record<
	Exclude<VendorType, VendorType.UNSPECIFIED>,
	string
> = {
	[VendorType.INDIVIDUAL]: "個人",
	[VendorType.BUSINESS]: "公司行號",
};

const OUTREACH_STATUS_TEXT: Record<
	Exclude<OutreachStatus, OutreachStatus.UNSPECIFIED>,
	string
> = {
	[OutreachStatus.NOT_CONTACTED]: "未聯絡",
	[OutreachStatus.INVITED]: "已寄邀請",
	[OutreachStatus.REPLIED]: "已回覆",
	[OutreachStatus.DECLINED]: "已婉拒",
	[OutreachStatus.DO_NOT_CONTACT]: "勿再聯絡",
	[OutreachStatus.CONVERTED]: "已入駐",
};

const CONTACT_KIND_TEXT: Record<
	Exclude<ContactKind, ContactKind.UNSPECIFIED>,
	string
> = {
	[ContactKind.EMAIL]: "電子郵件",
	[ContactKind.PHONE]: "電話",
	[ContactKind.WEBSITE]: "網站",
	[ContactKind.SOCIAL]: "社群帳號",
};

const MATCH_TIER_TEXT: Record<
	Exclude<ProspectMatchTier, ProspectMatchTier.UNSPECIFIED>,
	string
> = {
	[ProspectMatchTier.T1_TAX_ID]: "T1・統編唯一鍵",
	[ProspectMatchTier.T2_CONTACT]: "T2・聯絡方式相符",
	[ProspectMatchTier.T3_NAME]: "T3・名稱相同",
};

const MATCH_REASON_TEXT: Record<
	Exclude<ProspectMatchReason, ProspectMatchReason.UNSPECIFIED>,
	string
> = {
	[ProspectMatchReason.TAX_ID]: "統一編號相同",
	[ProspectMatchReason.NAME]: "名稱相同",
	[ProspectMatchReason.EMAIL]: "電子郵件相同",
	[ProspectMatchReason.PHONE]: "電話相同",
	[ProspectMatchReason.DOMAIN]: "網站網域相同",
	[ProspectMatchReason.SOCIAL]: "社群帳號相同",
};

export const prospectStatusText = (value: ProspectStatus) =>
	textOf(PROSPECT_STATUS_TEXT, value);
export const vendorTypeText = (value: VendorType) =>
	textOf(VENDOR_TYPE_TEXT, value);
export const outreachStatusText = (value: OutreachStatus) =>
	textOf(OUTREACH_STATUS_TEXT, value);
export const contactKindText = (value: ContactKind) =>
	textOf(CONTACT_KIND_TEXT, value);
export const matchTierText = (value: ProspectMatchTier) =>
	textOf(MATCH_TIER_TEXT, value);
export const matchReasonText = (value: ProspectMatchReason) =>
	textOf(MATCH_REASON_TEXT, value);

/** 命中原因（可能多個）以「、」連接。 */
export function matchReasonsText(
	reasons: readonly ProspectMatchReason[],
): string {
	return reasons.map(matchReasonText).join("、");
}

/** 可以建立的型態（下拉的選項）。 */
export const VENDOR_TYPE_OPTIONS: readonly VendorType[] = [
	VendorType.INDIVIDUAL,
	VendorType.BUSINESS,
];

/** 管理員可以設定的招商進度：不含 CONVERTED（它只由轉成店家寫入，契約不允許手動設定）。 */
export const EDITABLE_OUTREACH_STATUSES: readonly OutreachStatus[] = [
	OutreachStatus.NOT_CONTACTED,
	OutreachStatus.INVITED,
	OutreachStatus.REPLIED,
	OutreachStatus.DECLINED,
	OutreachStatus.DO_NOT_CONTACT,
];

/** 聯絡方式的類型選項（編輯器的下拉）。 */
export const CONTACT_KIND_OPTIONS: readonly ContactKind[] = [
	ContactKind.EMAIL,
	ContactKind.PHONE,
	ContactKind.WEBSITE,
	ContactKind.SOCIAL,
];

/** 清單的狀態篩選：沒有「全部」，預設進行中。 */
export const STATUS_FILTER_OPTIONS: readonly ProspectStatus[] = [
	ProspectStatus.ACTIVE,
	ProspectStatus.ARCHIVED,
	ProspectStatus.CONVERTED,
];
export const DEFAULT_STATUS_FILTER = ProspectStatus.ACTIVE;

// ---------------------------------------------------------------------------
// 清單的篩選與網址

/** enum 值在網址的寫法：enum 名稱的小寫（`active`、`not_contacted`）。 */
export function filterParam(
	enumObject: Record<number, string>,
	value: number,
): string {
	return (enumObject[value] ?? "").toLowerCase();
}

function parseFilter<E extends number>(
	raw: string | null | undefined,
	allowed: readonly E[],
	enumObject: Record<number, string>,
): E | undefined {
	if (!raw) return undefined;
	return allowed.find((value) => filterParam(enumObject, value) === raw);
}

/** 清單的篩選條件；狀態一定有值（預設進行中），其他沒選時是 undefined（不送）。 */
export interface ProspectFilters {
	status: ProspectStatus;
	outreachStatus?: EditableOutreachStatus;
	vendorType?: VendorType;
	category: string;
	region: string;
	keyword: string;
}

type EditableOutreachStatus = (typeof EDITABLE_OUTREACH_STATUSES)[number];

/** `useSearchParams()` 的回傳值只用得到 `.get`。 */
interface SearchParamsLike {
	get(name: string): string | null;
}

/** 從網址還原篩選：enum 以白名單轉換，不認得的值回預設（狀態）或不篩選。 */
export function readProspectFilters(
	searchParams: SearchParamsLike,
): ProspectFilters {
	return {
		status:
			parseFilter(
				searchParams.get("status"),
				STATUS_FILTER_OPTIONS,
				ProspectStatus,
			) ?? DEFAULT_STATUS_FILTER,
		outreachStatus: parseFilter(
			searchParams.get("outreachStatus"),
			EDITABLE_OUTREACH_STATUSES,
			OutreachStatus,
		),
		vendorType: parseFilter(
			searchParams.get("vendorType"),
			VENDOR_TYPE_OPTIONS,
			VendorType,
		),
		category: searchParams.get("category") ?? "",
		region: searchParams.get("region") ?? "",
		keyword: searchParams.get("keyword") ?? "",
	};
}

export function hasActiveFilter(filters: ProspectFilters): boolean {
	return (
		filters.status !== DEFAULT_STATUS_FILTER ||
		filters.outreachStatus !== undefined ||
		filters.vendorType !== undefined ||
		filters.category !== "" ||
		filters.region !== "" ||
		filters.keyword !== ""
	);
}

type ListProspectsInit = MessageInitShape<typeof ListProspectsRequestSchema>;

/** 篩選對應的請求欄位：沒選的不送；狀態送明確的值，不靠 UNSPECIFIED 的預設。 */
export function listProspectsFields(
	filters: ProspectFilters,
): Pick<
	ListProspectsInit,
	"status" | "outreachStatus" | "vendorType" | "category" | "region" | "keyword"
> {
	return {
		status: filters.status,
		...(filters.outreachStatus !== undefined && {
			outreachStatus: filters.outreachStatus,
		}),
		...(filters.vendorType !== undefined && { vendorType: filters.vendorType }),
		category: filters.category,
		region: filters.region,
		keyword: filters.keyword,
	};
}

// ---------------------------------------------------------------------------
// 聯絡方式：每個類型恰一個主要

export const MAX_CONTACTS = 50;
const CONTACT_VALUE_MAX: Record<
	Exclude<ContactKind, ContactKind.UNSPECIFIED>,
	number
> = {
	[ContactKind.EMAIL]: 255,
	[ContactKind.PHONE]: 50,
	[ContactKind.WEBSITE]: 500,
	[ContactKind.SOCIAL]: 500,
};

/** 編輯器的一列。`key` 只給畫面辨識這一列（不送出）。 */
export interface ContactRow {
	key: string;
	kind: ContactKind;
	value: string;
	primary: boolean;
}

let contactKeySeq = 0;
export function newContactKey(): string {
	contactKeySeq += 1;
	return `contact-${contactKeySeq}`;
}

/** 每個有列的類型恰好一個主要：沒有時第一列遞補，多個時留第一個。 */
export function normalizePrimaries(rows: readonly ContactRow[]): ContactRow[] {
	const chosen = new Map<ContactKind, string>();
	for (const row of rows) {
		if (row.primary && !chosen.has(row.kind)) chosen.set(row.kind, row.key);
	}
	for (const row of rows) {
		if (!chosen.has(row.kind)) chosen.set(row.kind, row.key);
	}
	return rows.map((row) => ({
		...row,
		primary: chosen.get(row.kind) === row.key,
	}));
}

/** 新增的列預設是電子郵件；同類型沒有其他列時它就是主要。 */
export function addContact(
	rows: readonly ContactRow[],
	key: string,
): ContactRow[] {
	return normalizePrimaries([
		...rows,
		{ key, kind: ContactKind.EMAIL, value: "", primary: false },
	]);
}

/** 刪掉主要的那一列時，同類型的第一列遞補。 */
export function removeContact(
	rows: readonly ContactRow[],
	key: string,
): ContactRow[] {
	return normalizePrimaries(rows.filter((row) => row.key !== key));
}

/** 改類型：原類型沒有主要時第一列遞補；新類型沒有其他列時它成為主要，否則不是主要。 */
export function changeContactKind(
	rows: readonly ContactRow[],
	key: string,
	kind: ContactKind,
): ContactRow[] {
	return normalizePrimaries(
		rows.map((row) =>
			row.key === key ? { ...row, kind, primary: false } : row,
		),
	);
}

/** 設成這個類型的主要，同類型其他列取消。 */
export function setPrimaryContact(
	rows: readonly ContactRow[],
	key: string,
): ContactRow[] {
	const target = rows.find((row) => row.key === key);
	if (!target) return [...rows];
	return rows.map((row) =>
		row.kind === target.kind ? { ...row, primary: row.key === key } : row,
	);
}

export function editContactValue(
	rows: readonly ContactRow[],
	key: string,
	value: string,
): ContactRow[] {
	return rows.map((row) => (row.key === key ? { ...row, value } : row));
}

export function contactRowsFromProspect(prospect: Prospect): ContactRow[] {
	return prospect.contacts.map((contact) => ({
		key: newContactKey(),
		kind: contact.kind,
		value: contact.value,
		primary: contact.primary,
	}));
}

export type ContactsResult =
	| {
			ok: true;
			contacts: { kind: ContactKind; value: string; primary: boolean }[];
	  }
	| { ok: false; error: string };

/**
 * 送出的聯絡方式：去前後空白（同伺服器的 Go TrimSpace）、依類型檢查長度、最多 50 筆。錯誤句與伺服器的句子相同
 * （`NormalizeContacts`）。
 */
export function buildContacts(rows: readonly ContactRow[]): ContactsResult {
	if (rows.length > MAX_CONTACTS) {
		return { ok: false, error: `聯絡方式最多 ${MAX_CONTACTS} 個` };
	}
	const contacts = [];
	for (const row of rows) {
		const max = CONTACT_VALUE_MAX[row.kind as keyof typeof CONTACT_VALUE_MAX];
		const length = goTrimmedRuneCount(row.value);
		if (max === undefined || length === 0 || length > max) {
			return {
				ok: false,
				error: `聯絡方式的值必須是 1 到 ${max ?? 500} 字`,
			};
		}
		contacts.push({
			kind: row.kind,
			value: trimGoSpace(row.value),
			primary: row.primary,
		});
	}
	return { ok: true, contacts };
}

/** 某個類型的主要聯絡方式的值；沒有時是空字串。 */
export function primaryContactValue(
	contacts: readonly { kind: ContactKind; value: string; primary: boolean }[],
	kind: ContactKind,
): string {
	return contacts.find((c) => c.kind === kind && c.primary)?.value ?? "";
}

// ---------------------------------------------------------------------------
// 表單、request 與 update mask

export interface ProspectForm {
	displayName: string;
	vendorType: VendorType;
	outreachStatus: OutreachStatus;
	taxId: string;
	contacts: ContactRow[];
	category: string;
	region: string;
	productSummary: string;
	positioning: string;
	researchSummary: string;
	priorityLetter: string;
	/** 輸入框的文字；空字串是 0。 */
	priorityStars: string;
	/** 沒有編輯畫面：帶著它只為了判斷別人有沒有改過（E5）。 */
	categoryIds: readonly string[];
}

/** 新增表單的初始值：型態預設個人（不用 UNSPECIFIED），招商進度不送（伺服器預設未聯絡）。 */
export function emptyProspectForm(): ProspectForm {
	return {
		displayName: "",
		vendorType: VendorType.INDIVIDUAL,
		outreachStatus: OutreachStatus.NOT_CONTACTED,
		taxId: "",
		contacts: [],
		category: "",
		region: "",
		productSummary: "",
		positioning: "",
		researchSummary: "",
		priorityLetter: "",
		priorityStars: "",
		categoryIds: [],
	};
}

export function prospectToForm(prospect: Prospect): ProspectForm {
	return {
		displayName: prospect.displayName,
		vendorType: prospect.vendorType,
		outreachStatus: prospect.outreachStatus,
		taxId: prospect.taxId,
		contacts: contactRowsFromProspect(prospect),
		category: prospect.category,
		region: prospect.region,
		productSummary: prospect.productSummary,
		positioning: prospect.positioning,
		researchSummary: prospect.researchSummary,
		priorityLetter: prospect.priorityLetter,
		priorityStars: prospect.priorityStars ? String(prospect.priorityStars) : "",
		categoryIds: prospect.categoryIds,
	};
}

/** 星等輸入框的值：空字串是 0；0 以上的整數；其他回 null（不是有效的星等）。 */
export function parsePriorityStars(text: string): number | null {
	const trimmed = trimGoSpace(text);
	if (trimmed === "") return 0;
	if (!/^\d{1,9}$/u.test(trimmed)) return null;
	return Number(trimmed);
}

type TextFieldName =
	| "displayName"
	| "taxId"
	| "category"
	| "region"
	| "productSummary"
	| "positioning"
	| "researchSummary"
	| "priorityLetter";

const TEXT_FIELD_PATHS: readonly [TextFieldName, string][] = [
	["displayName", "display_name"],
	["taxId", "tax_id"],
	["category", "category"],
	["region", "region"],
	["productSummary", "product_summary"],
	["positioning", "positioning"],
	["researchSummary", "research_summary"],
	["priorityLetter", "priority_letter"],
];

function contactsKey(rows: readonly ContactRow[]): string {
	return JSON.stringify(
		rows
			.map((r) => [r.kind, trimGoSpace(r.value), r.primary] as const)
			.sort((a, b) => a[0] - b[0] || a[1].localeCompare(b[1])),
	);
}

type WritableFieldPath =
	| "display_name"
	| "vendor_type"
	| "outreach_status"
	| "tax_id"
	| "contacts"
	| "category"
	| "region"
	| "product_summary"
	| "positioning"
	| "research_summary"
	| "priority_letter"
	| "priority_stars";

/**
 * update_mask 的路徑：只列和基準不同的欄位（文字比的是去空白後的值）；`contacts` 整批列；不送 `"*"`（它會要求兩張圖），
 * `category_ids` 沒有編輯畫面，永不列。沒有改動回空陣列，呼叫端不送出。
 */
export function updateMaskPaths(
	base: ProspectForm,
	draft: ProspectForm,
): WritableFieldPath[] {
	const paths: WritableFieldPath[] = [];
	for (const [field, path] of TEXT_FIELD_PATHS) {
		if (trimGoSpace(base[field]) !== trimGoSpace(draft[field])) {
			paths.push(path as WritableFieldPath);
		}
	}
	if (base.vendorType !== draft.vendorType) paths.push("vendor_type");
	if (base.outreachStatus !== draft.outreachStatus) {
		paths.push("outreach_status");
	}
	if (contactsKey(base.contacts) !== contactsKey(draft.contacts)) {
		paths.push("contacts");
	}
	if (
		(parsePriorityStars(base.priorityStars) ?? base.priorityStars) !==
		(parsePriorityStars(draft.priorityStars) ?? draft.priorityStars)
	) {
		paths.push("priority_stars");
	}
	return paths;
}

/**
 * 別人（或換圖）寫入之後，新版本的 etag 還能不能沿用：圖片以外全部可寫欄位（文字、vendor_type、outreach_status、
 * priority_stars、contacts、category_ids）都等於表單的基準才行；有任何一個不同，就是別人改過，要當成衝突。
 */
export function canReuseEtag(
	base: ProspectForm,
	incoming: ProspectForm,
): boolean {
	return (
		updateMaskPaths(base, incoming).length === 0 &&
		JSON.stringify(base.categoryIds) === JSON.stringify(incoming.categoryIds)
	);
}

/**
 * 衝突之後的草稿：管理員改過的欄位（和舊基準不同）保留；沒碰的欄位換成新版本的值，這樣之後送出的 mask 只含管理員
 * 自己改過的欄位，不會把別人的修改蓋回舊值。
 */
export function mergeDraftOntoIncoming(
	base: ProspectForm,
	draft: ProspectForm,
	incoming: ProspectForm,
): ProspectForm {
	const edited = new Set<string>(updateMaskPaths(base, draft));
	const pick = <K extends keyof ProspectForm>(
		path: WritableFieldPath,
		field: K,
	): ProspectForm[K] => (edited.has(path) ? draft[field] : incoming[field]);
	return {
		...incoming,
		displayName: pick("display_name", "displayName"),
		vendorType: pick("vendor_type", "vendorType"),
		outreachStatus: pick("outreach_status", "outreachStatus"),
		taxId: pick("tax_id", "taxId"),
		contacts: pick("contacts", "contacts"),
		category: pick("category", "category"),
		region: pick("region", "region"),
		productSummary: pick("product_summary", "productSummary"),
		positioning: pick("positioning", "positioning"),
		researchSummary: pick("research_summary", "researchSummary"),
		priorityLetter: pick("priority_letter", "priorityLetter"),
		priorityStars: pick("priority_stars", "priorityStars"),
	};
}

/** 只擋「非 8 碼數字」的明顯格式錯誤；統編的檢查碼由伺服器判斷。 */
export function taxIdFormatError(taxId: string): string | null {
	const trimmed = trimGoSpace(taxId);
	if (trimmed === "" || /^\d{8}$/.test(trimmed)) return null;
	return "統一編號須為 8 碼數字";
}

/** 送出前的檢查，回第一個要告訴管理員的錯誤；星等不是有效的整數由欄位自己標記（`parsePriorityStars`）。 */
export function prospectFormError(form: ProspectForm): string | null {
	if (trimGoSpace(form.displayName) === "") return "請填寫業者名稱。";
	const taxError = taxIdFormatError(form.taxId);
	if (taxError) return taxError;
	const contacts = buildContacts(form.contacts);
	return contacts.ok ? null : contacts.error;
}

/**
 * 表單轉成 Prospect 的請求內容。呼叫前要先通過 `prospectFormError` 與 `parsePriorityStars`；新增時不帶招商進度
 * （伺服器預設未聯絡），修改帶表單的值（只有 mask 選到的才會被採用）。
 */
export function prospectInit(
	form: ProspectForm,
	options: { includeOutreach: boolean },
): MessageInitShape<typeof ProspectSchema> {
	const contacts = buildContacts(form.contacts);
	if (!contacts.ok) throw new Error(contacts.error);
	const stars = parsePriorityStars(form.priorityStars);
	if (stars === null) throw new Error("priority_stars 不是有效的整數");
	return {
		displayName: trimGoSpace(form.displayName),
		vendorType: form.vendorType,
		...(options.includeOutreach && { outreachStatus: form.outreachStatus }),
		taxId: trimGoSpace(form.taxId),
		contacts: contacts.contacts,
		category: trimGoSpace(form.category),
		region: trimGoSpace(form.region),
		productSummary: trimGoSpace(form.productSummary),
		positioning: trimGoSpace(form.positioning),
		researchSummary: trimGoSpace(form.researchSummary),
		priorityLetter: trimGoSpace(form.priorityLetter),
		priorityStars: stars,
	};
}

/**
 * 即時查重送的內容：名稱、格式正確的統編，與已填值的聯絡方式（空列與過長的值留到送出時檢查）。名稱沒填時回 null。
 */
export function previewRequest(
	form: ProspectForm,
): MessageInitShape<typeof PreviewProspectRequestSchema> | null {
	const displayName = trimGoSpace(form.displayName);
	if (displayName === "") return null;
	const filled = form.contacts.filter((row) => trimGoSpace(row.value) !== "");
	const contacts = buildContacts(filled);
	return {
		displayName,
		taxId: taxIdFormatError(form.taxId) ? "" : trimGoSpace(form.taxId),
		contacts: contacts.ok ? contacts.contacts : [],
	};
}

// ---------------------------------------------------------------------------
// 合併

/** 十進位字串的 ID 以數值比較（字串比大小會讓 "9" 排在 "10" 後面）；回傳負數表示 a 較小。 */
export function compareIds(a: string, b: string): number {
	const x = BigInt(a);
	const y = BigInt(b);
	return x < y ? -1 : x > y ? 1 : 0;
}

/** 兩個 ID 依數值由小到大排成 [較小, 較大]。 */
export function sortIdsNumerically(a: string, b: string): [string, string] {
	return compareIds(a, b) <= 0 ? [a, b] : [b, a];
}

/**
 * 業務欄位填了幾項：統編、各類型的聯絡方式（每種類型算一項）、分類、地區、三段文字、優先字母與星等。
 * 不看狀態與稽核欄位，那些不是「這筆資料填得多完整」的訊號。
 */
export function profileCompleteness(prospect: Prospect): number {
	const contactKinds = new Set(prospect.contacts.map((c) => c.kind));
	const filled = [
		prospect.taxId,
		prospect.category,
		prospect.region,
		prospect.productSummary,
		prospect.positioning,
		prospect.researchSummary,
		prospect.priorityLetter,
		prospect.priorityStars,
	].filter(Boolean).length;
	return filled + contactKinds.size;
}

/**
 * 合併的預設勝方：`preferredId` 是兩邊之一時用它（統編出口以統編持有者為準）；否則完整度高的一邊，
 * 平手取 ID 數值小的。
 */
export function defaultWinnerId(
	left: Prospect,
	right: Prospect,
	preferredId?: string,
): string {
	if (preferredId === left.id || preferredId === right.id) return preferredId;
	const diff = profileCompleteness(right) - profileCompleteness(left);
	if (diff !== 0) return diff > 0 ? right.id : left.id;
	return compareIds(left.id, right.id) <= 0 ? left.id : right.id;
}
