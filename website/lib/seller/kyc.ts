/**
 * 商家認證（KYC）的 Connect 資料層與表單對照。
 *
 * 申請是店家的單例資源：沒有申請 ID，也沒有 Create。`GetMyKYC` 回 not_found 就是還沒有申請（或已取消）；
 * 沒有申請時不帶 etag 的 `SaveKYCDraft` 建立它，之後每次寫入帶讀到的 etag。表單是整份字串狀態，
 * 伺服器版本是 baseline：只有和 baseline 不同的欄位才列進 update_mask。平台只收市集審核的資料（統編與公司資料、
 * 補充文件）；身分與銀行資料由金流商收集，不在這個表單裡。還缺什麼由伺服器的 `missing_fields` 決定，client 不重寫完整性規則。
 */

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BusinessKYCSchema,
	type BusinessRegistration,
	type CancelKYCResponse,
	type KYCApplication,
	KYCApplicationSchema,
	KYCField,
	KYCStatus,
	type SaveKYCDraftRequest,
	SaveKYCDraftRequestSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { formatDate, parseDate } from "@hawkr/api/proto/date";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { countyCodeOfTownCode } from "@hawkr/twaddr/lookup";
import { sellerKYCField } from "@hawkr/ui-text/keys";
import { sellerAppClient } from "../connect/seller";
import { optionalText, type UiText } from "../i18n/ui-text";
import type { AgreementAcceptanceInput } from "./agreements";
import type { StatusBannerTone } from "./status-tone";

export type SellerType = "individual" | "business";

export type CertificationSectionId =
	| "company"
	| "other"
	| "documents"
	| "confirm";

export interface KycForm {
	sellerType: SellerType;
	productType: string;
	taxId: string;
	companyName: string;
	companyRegisterDate: string;
	companyCountyCode: string;
	companyTownCode: string;
	companyAddressDetail: string;
	companyPhone: string;
}

export const EMPTY_KYC_FORM: KycForm = {
	sellerType: "individual",
	productType: "",
	taxId: "",
	companyName: "",
	companyRegisterDate: "",
	companyCountyCode: "",
	companyTownCode: "",
	companyAddressDetail: "",
	companyPhone: "",
};

/** 一張補充文件圖片：已在申請上的（assetId）或剛上傳還沒存檔的（stagedToken），preview 是顯示用的網址。 */
export interface KycDoc {
	assetId: string;
	stagedToken: string;
	preview: string;
}

export interface KycDocs {
	companyDocuments: KycDoc[];
}

export const EMPTY_KYC_DOCS: KycDocs = {
	companyDocuments: [],
};

/* ===== 日期與地址 ===== */

/* ===== 申請 → 表單 ===== */

export function kycTypeOf(app: KYCApplication | null): SellerType | null {
	switch (app?.subject.case) {
		case "individual":
			return "individual";
		case "business":
			return "business";
		default:
			return null;
	}
}

export function kycFormOf(app: KYCApplication | null): KycForm {
	if (!app) return { ...EMPTY_KYC_FORM };
	const business = app.subject.case === "business" ? app.subject.value : null;
	const townCode = business?.companyAddress?.townCode ?? "";
	return {
		sellerType: kycTypeOf(app) ?? "individual",
		productType: app.productType,
		taxId: business?.taxId ?? "",
		companyName: business?.companyName ?? "",
		companyRegisterDate: formatDate(business?.companyRegisterDate),
		companyCountyCode: countyCodeOfTownCode(townCode),
		companyTownCode: townCode,
		companyAddressDetail: business?.companyAddress?.addressDetail ?? "",
		companyPhone: business?.companyPhone ?? "",
	};
}

function docOf(
	doc: { assetId: string; downloadUri: string } | undefined,
): KycDoc | null {
	if (!doc || !doc.assetId) return null;
	return { assetId: doc.assetId, stagedToken: "", preview: doc.downloadUri };
}

export function kycDocsOf(app: KYCApplication | null): KycDocs {
	if (!app) return { ...EMPTY_KYC_DOCS, companyDocuments: [] };
	const business = app.subject.case === "business" ? app.subject.value : null;
	return {
		companyDocuments: (business?.companyDocuments ?? [])
			.map(docOf)
			.filter((d): d is KycDoc => d !== null),
	};
}

/**
 * 建立草稿時別處已經建立了申請：原樣採用那一份。表單、補充文件與 baseline 都取自申請，店家的預設值（公司電話）
 * 只屬於全新的申請，不能當成使用者的修改保留下來而被自動存檔寫進既有的申請。
 */
export function adoptExistingApplication(app: KYCApplication): {
	form: KycForm;
	docs: KycDocs;
	baseline: KycForm;
	baselineDocs: KycDocs;
	etag: string;
} {
	const form = kycFormOf(app);
	const docs = kycDocsOf(app);
	return { form, docs, baseline: form, baselineDocs: docs, etag: app.etag };
}

/** 表單欄位對應的 update_mask 路徑（一個欄位可能與別的欄位同路徑，例如公司地址）。 */
const MASK_PATHS_OF_FORM_KEY: Partial<Record<keyof KycForm, string[]>> = {
	productType: ["product_type"],
	taxId: ["business.tax_id"],
	companyName: ["business.company_name"],
	companyRegisterDate: ["business.company_register_date"],
	companyTownCode: ["business.company_address"],
	companyCountyCode: ["business.company_address"],
	companyAddressDetail: ["business.company_address"],
	companyPhone: ["business.company_phone"],
};

/**
 * 存檔成功之後的 baseline：這次請求真的送出的欄位（`sentPaths`，取自請求的 update_mask）的 baseline 變成送出的值，沒送出的欄位
 * 維持原本的 baseline。「存檔之後有沒有改動」因此只看目前的輸入和送出的值，與伺服器怎麼正規化（去前後空白、電話去橫線）無關：
 * 不會重送，也不需要把伺服器的值寫回輸入中的表單；格式不合而沒送出的欄位仍與 baseline 不同，輸入原樣保留。
 */
export function baselineAfterSave(
	previous: KycForm,
	sent: KycForm,
	sentPaths: readonly string[],
): KycForm {
	const paths = new Set(sentPaths);
	const next = { ...previous, sellerType: sent.sellerType } as Record<
		string,
		unknown
	>;
	for (const key of Object.keys(MASK_PATHS_OF_FORM_KEY) as (keyof KycForm)[]) {
		if (MASK_PATHS_OF_FORM_KEY[key]?.some((path) => paths.has(path))) {
			next[key] = sent[key];
		}
	}
	return next as unknown as KycForm;
}

/* ===== 輸入驗證（只決定「這個值能不能送」，缺不缺由伺服器決定） ===== */

/* ===== 表單 → SaveKYCDraft ===== */

type ApplicationInit = MessageInitShape<typeof KYCApplicationSchema>;

function docInit(doc: KycDoc) {
	return doc.stagedToken
		? { stagedToken: doc.stagedToken }
		: { assetId: doc.assetId };
}

function docChanged(next: KycDoc | null, base: KycDoc | null): boolean {
	if (next === null || base === null) return next !== base;
	return next.stagedToken !== "" || next.assetId !== base.assetId;
}

function docListChanged(next: KycDoc[], base: KycDoc[]): boolean {
	if (next.length !== base.length) return true;
	return next.some((d, i) => docChanged(d, base[i] ?? null));
}

export interface KycDraftSource {
	form: KycForm;
	docs: KycDocs;
	baseline: KycForm;
	baselineDocs: KycDocs;
	/** 已經有申請時的 etag；沒有申請（要建立）時不帶。 */
	etag?: string;
}

/**
 * 表單相對於 baseline 的差異，組成 `SaveKYCDraft`。沒有申請（沒有 etag）時是建立：subject 一定設，所有非零值都送；
 * 已經有申請時只列有改的欄位，etag 必帶。沒有改動回 null。
 */
export function buildSaveKycDraftRequest(
	src: KycDraftSource,
): SaveKYCDraftRequest | null {
	const { form, docs, baseline, baselineDocs } = src;
	const creating = !src.etag;
	const business = form.sellerType === "business";
	const paths: string[] = [];
	const app: ApplicationInit = {};
	const biz: MessageInitShape<typeof BusinessKYCSchema> = {};
	let touchesBranch = false;

	// 每個欄位只用 localName 命名一次，proto 路徑從描述取（與 SaveKYCDraft 的 update_mask 一致）。
	const appPath = (key: keyof typeof KYCApplicationSchema.field) =>
		KYCApplicationSchema.field[key].name;
	const bizPath = (key: keyof typeof BusinessKYCSchema.field) =>
		`business.${BusinessKYCSchema.field[key].name}`;

	const text = (key: "productType") => {
		const value = form[key];
		if (value === baseline[key]) return;
		paths.push(appPath(key));
		app[key] = value;
	};

	text("productType");

	if (business) {
		const bizText = (key: "companyName" | "taxId" | "companyPhone") => {
			const value = form[key];
			if (value === baseline[key]) return;
			paths.push(bizPath(key));
			biz[key] = value;
			touchesBranch = true;
		};
		bizText("companyName");
		bizText("taxId");
		bizText("companyPhone");
		if (form.companyRegisterDate !== baseline.companyRegisterDate) {
			const date = parseDate(form.companyRegisterDate);
			if (form.companyRegisterDate === "" || date) {
				paths.push(bizPath("companyRegisterDate"));
				if (date) biz.companyRegisterDate = date;
				touchesBranch = true;
			}
		}
		const addressChanged =
			form.companyTownCode !== baseline.companyTownCode ||
			form.companyAddressDetail !== baseline.companyAddressDetail;
		if (addressChanged) {
			if (form.companyTownCode) {
				paths.push(bizPath("companyAddress"));
				biz.companyAddress = {
					townCode: form.companyTownCode,
					addressDetail: form.companyAddressDetail,
				};
				touchesBranch = true;
			} else if (form.companyAddressDetail === "") {
				paths.push(bizPath("companyAddress"));
				touchesBranch = true;
			}
		}
	}

	if (
		business &&
		docListChanged(docs.companyDocuments, baselineDocs.companyDocuments)
	) {
		paths.push(bizPath("companyDocuments"));
		biz.companyDocuments = docs.companyDocuments.map(docInit);
		touchesBranch = true;
	}

	// 個人沒有欄位，型態只由 subject 設定的那一支表示：已有申請從公司改成個人時，要在 update_mask 直接列 individual 並送出空的 individual。
	if (!creating && !business && baseline.sellerType === "business") {
		paths.push(appPath("individual"));
		touchesBranch = true;
	}

	if (!creating && paths.length === 0) return null;

	if (creating || touchesBranch) {
		app.subject = business
			? { case: "business", value: biz }
			: { case: "individual", value: {} };
	}
	app.etag = src.etag ?? "";
	return create(SaveKYCDraftRequestSchema, {
		application: app,
		updateMask: fieldMask(KYCApplicationSchema, paths),
	});
}

/* ===== 續填位置與缺項 ===== */

export const SECTION_LABELS: Record<CertificationSectionId, string> = {
	company: "公司資訊",
	other: "商家營運資訊",
	documents: "補充文件",
	confirm: "確認送出",
};

const SECTION_ORDER: readonly CertificationSectionId[] = [
	"company",
	"other",
	"documents",
	"confirm",
];

interface FieldMeta {
	section: CertificationSectionId;
}

/** 伺服器的欄位（缺項、退回要補的欄位）所在的區塊；名稱取自共用文案目錄（`kycFieldLabel`）。表單沒有的欄位不在這裡。 */
export const KYC_FIELDS: Partial<Record<KYCField, FieldMeta>> = {
	[KYCField.KYC_FIELD_PRODUCT_TYPE]: { section: "company" },
	[KYCField.KYC_FIELD_WEBSITE_URL]: { section: "other" },
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME]: { section: "company" },
	[KYCField.KYC_FIELD_BUSINESS_TAX_ID]: { section: "company" },
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE]: { section: "company" },
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE]: { section: "company" },
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS]: { section: "documents" },
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS]: { section: "company" },
};

export interface MissingItem {
	sectionId: CertificationSectionId;
	label: string;
}

function metaOf(field: KYCField): FieldMeta | undefined {
	return KYC_FIELDS[field];
}

/**
 * 欄位的呈現名稱（共用文案目錄）；UNSPECIFIED 與不認得的欄位回 undefined（不顯示一個空白或原始列舉名）。
 * 表單已經沒有的欄位（舊申請的退回問題可能指到它們）照樣有名稱，退回提示要說出是哪一項。
 */
export function kycFieldLabel(ut: UiText, field: KYCField): string | undefined {
	return optionalText(ut, sellerKYCField(field));
}

/**
 * 伺服器的 `missing_fields` 轉成畫面的缺項清單，依欄位順序；表單沒有的欄位（不應再出現）與沒有名稱的欄位略過。
 */
export function missingItemsOf(
	ut: UiText,
	missing: readonly KYCField[],
): MissingItem[] {
	return missing.flatMap((field) => {
		const section = metaOf(field)?.section;
		const label = kycFieldLabel(ut, field);
		return section === undefined || label === undefined
			? []
			: [{ sectionId: section, label }];
	});
}

/** 續填的位置：由 `missing_fields` 推導——第一個還有缺項的區塊；沒有缺項回 null。 */
export function resumeSectionOf(
	missing: readonly KYCField[],
): CertificationSectionId | null {
	const sections = new Set(
		missing.map((f) => metaOf(f)?.section).filter((s) => s !== undefined),
	);
	return SECTION_ORDER.find((s) => sections.has(s)) ?? null;
}

/* ===== 版本衝突：保留輸入、顯示最新 ===== */

/**
 * 衝突之後的表單：使用者改過的欄位（與舊 baseline 不同）保留輸入，沒改的欄位換成最新版本。
 */
export function mergeLatestForm(
	current: KycForm,
	oldBaseline: KycForm,
	latest: KycForm,
): KycForm {
	const next = { ...latest } as Record<string, unknown>;
	for (const key of Object.keys(current) as (keyof KycForm)[]) {
		if (current[key] !== oldBaseline[key]) next[key] = current[key];
	}
	return next as unknown as KycForm;
}

/** 補充文件的同一規則：使用者換過、移除過或新傳的清單保留，其餘換成最新版本。 */
export function mergeLatestDocs(
	current: KycDocs,
	oldBaseline: KycDocs,
	latest: KycDocs,
): KycDocs {
	return {
		companyDocuments: docListChanged(
			current.companyDocuments,
			oldBaseline.companyDocuments,
		)
			? current.companyDocuments
			: latest.companyDocuments,
	};
}

/**
 * 存檔回應之後的補充文件：剛傳的（staged）換成申請上的圖片，預覽沿用本機的（不必重新載入），
 * 使用者在存檔期間又動過的清單不碰。
 */
export function adoptSavedDocs(
	current: KycDocs,
	sent: KycDocs,
	saved: KycDocs,
): KycDocs {
	const adopt = (cur: KycDoc | null, now: KycDoc | null) =>
		cur && now ? { ...now, preview: cur.preview || now.preview } : now;
	return {
		companyDocuments:
			current.companyDocuments === sent.companyDocuments
				? sent.companyDocuments
						.map((doc, i) => adopt(doc, saved.companyDocuments[i] ?? null))
						.filter((d): d is KycDoc => d !== null)
				: current.companyDocuments,
	};
}

/* ===== 存檔失敗的欄位對照 ===== */

const FORM_KEY_OF_FIELD: Partial<Record<KYCField, keyof KycForm>> = {
	[KYCField.KYC_FIELD_PRODUCT_TYPE]: "productType",
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME]: "companyName",
	[KYCField.KYC_FIELD_BUSINESS_TAX_ID]: "taxId",
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE]: "companyRegisterDate",
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE]: "companyPhone",
	[KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS]: "companyAddressDetail",
};

/** SaveKYCDraft 的欄位錯誤（BadRequest 的 field 是 `application.` 開頭的路徑）對應回表單的欄位。 */
export function kycFormKeyOfDraftPath(path: string): keyof KycForm | undefined {
	const field = kycFieldOfPath(path.replace(/^application\./, ""));
	return field === undefined ? undefined : FORM_KEY_OF_FIELD[field];
}

/* ===== 退回的待處理問題 ===== */

/** 賣家還沒處理的退回問題（伺服器的 `outstanding_issues`），整理成表單上各處要顯示的說明。 */
export interface KycIssueMessages {
	/** 表單欄位的說明。 */
	form: Partial<Record<keyof KycForm, string>>;
	/** 指到某一張公司文件的說明，key 是 asset ID。 */
	companyDocumentByAsset: Record<string, string>;
	/** 公司文件沒有指到特定一張的說明。 */
	companyDocuments: string | null;
}

function joinMessages(current: string | undefined, next: string): string {
	return current ? `${current}；${next}` : next;
}

/**
 * 待處理的問題對照到畫面：哪些是已處理由伺服器判定（欄位的值被改過、指到的文件被移除或替換），client 不推導。
 * 同一處有多項問題時說明接在一起。
 */
export function kycIssueMessages(
	app: KYCApplication | null | undefined,
): KycIssueMessages {
	const out: KycIssueMessages = {
		form: {},
		companyDocumentByAsset: {},
		companyDocuments: null,
	};
	for (const issue of app?.outstandingIssues ?? []) {
		if (issue.field === KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS) {
			if (issue.documentAssetId) {
				out.companyDocumentByAsset[issue.documentAssetId] = joinMessages(
					out.companyDocumentByAsset[issue.documentAssetId],
					issue.message,
				);
			} else {
				out.companyDocuments = joinMessages(
					out.companyDocuments ?? undefined,
					issue.message,
				);
			}
		} else {
			const key = FORM_KEY_OF_FIELD[issue.field];
			if (key) out.form[key] = joinMessages(out.form[key], issue.message);
		}
	}
	return out;
}

/* ===== 流程狀態 ===== */

/** 表單可以編輯：沒有申請（要建立），或申請是草稿、被退回（送出審核中與已通過的申請不能改）。 */
export function kycEditable(app: KYCApplication | null): boolean {
	return (
		!app ||
		app.status === KYCStatus.KYC_STATUS_DRAFT ||
		app.status === KYCStatus.KYC_STATUS_REJECTED
	);
}

/** 申請狀態卡的色調；沒有色調（UNSPECIFIED、不認得的值）回 undefined。 */
export function kycStatusTone(status: KYCStatus): StatusBannerTone | undefined {
	switch (status) {
		case KYCStatus.KYC_STATUS_DRAFT:
			return "neutral";
		case KYCStatus.KYC_STATUS_SUBMITTED:
			return "info";
		case KYCStatus.KYC_STATUS_APPROVED:
			return "success";
		case KYCStatus.KYC_STATUS_REJECTED:
			return "danger";
		default:
			return undefined;
	}
}

/** 審核中的申請可以取消。 */
export function kycCancellable(status: KYCStatus | undefined): boolean {
	return status === KYCStatus.KYC_STATUS_SUBMITTED;
}

/** 草稿可以放棄。 */
export function kycDraftDiscardable(app: KYCApplication): boolean {
	return app.status === KYCStatus.KYC_STATUS_DRAFT;
}

/**
 * BadRequest 的欄位路徑（`business.tax_id`）對應回 `KYCField`；路徑指到訊息欄位底下（`business.company_address.town_code`）時，
 * 取最長的前綴（`business.company_address`）；重複欄位裡的單一元素（`business.company_documents[2]`）指到整個欄位
 * （表單不分文件標示，標在那個欄位上）；不認得的路徑回 undefined。
 */
export function kycFieldOfPath(path: string): KYCField | undefined {
	const segments = path.replace(/\[\d+\]/g, "").split(".");
	for (let length = segments.length; length > 0; length--) {
		const key = `KYC_FIELD_${segments.slice(0, length).join("_").toUpperCase()}`;
		const value = (KYCField as unknown as Record<string, unknown>)[key];
		if (typeof value === "number") return value as KYCField;
	}
	return undefined;
}

/** 補充文件欄位：值是上傳的圖片，不在表單的文字欄位裡（`FORM_KEY_OF_FIELD` 沒有它）。 */
export function isKycDocumentField(field: KYCField): boolean {
	return field === KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS;
}

/**
 * 補充文件欄位的錯誤在提示裡的文字：欄位名稱加上伺服器的訊息；單一元素（`business.company_documents[2]`）註明第幾張。
 * 不是補充文件欄位回 undefined。
 */
export function kycDocumentNotice(
	ut: UiText,
	path: string,
	field: KYCField,
	message: string,
): string | undefined {
	if (!isKycDocumentField(field)) return undefined;
	const label = kycFieldLabel(ut, field) ?? "補充文件";
	const index = /\[(\d+)\]/.exec(path)?.[1];
	const where =
		index === undefined ? label : `${label}（第 ${Number(index) + 1} 張）`;
	return `${where}：${message}`;
}

/* ===== RPC ===== */

type SellerClient = ReturnType<typeof sellerAppClient>;

/** 目前的申請；還沒有申請（或已取消）是 null，不是錯誤。 */
export async function fetchMyKyc(
	client: () => SellerClient = sellerAppClient,
): Promise<KYCApplication | null> {
	try {
		const res = await client().getMyKYC({});
		return res.application ?? null;
	} catch (e) {
		if (ConnectError.from(e).code === Code.NotFound) return null;
		throw e;
	}
}

export async function saveKycDraft(
	request: SaveKYCDraftRequest,
	client: () => SellerClient = sellerAppClient,
): Promise<KYCApplication> {
	const res = await client().saveKYCDraft(request);
	if (!res.application) throw new Error("SaveKYCDraft 的回應沒有申請");
	return res.application;
}

export async function submitKyc(
	etag: string,
	acceptedAgreements: readonly AgreementAcceptanceInput[],
	client: () => SellerClient = sellerAppClient,
): Promise<KYCApplication> {
	if (!etag) throw new Error("SubmitKYC 需要申請的 etag");
	const res = await client().submitKYC({
		etag,
		acceptedAgreements: acceptedAgreements.map((a) => ({
			kind: a.kind,
			version: a.version,
		})),
	});
	if (!res.application) throw new Error("SubmitKYC 的回應沒有申請");
	return res.application;
}

export async function cancelKyc(
	etag: string,
	client: () => SellerClient = sellerAppClient,
): Promise<CancelKYCResponse> {
	if (!etag) throw new Error("CancelKYC 需要申請的 etag");
	return client().cancelKYC({ etag });
}

/** 統編查到的登記資料，轉成表單用的值；查無這個統編是 null。 */
export interface BusinessLookup {
	name: string;
	registerDate: string;
	townCode: string;
	addressDetail: string;
}

export function businessLookupOf(
	reg: BusinessRegistration | undefined,
): BusinessLookup | null {
	if (!reg) return null;
	const postal = reg.postalAddress;
	return {
		name: reg.name,
		registerDate: formatDate(reg.setupDate),
		townCode: postal?.townCode ?? "",
		addressDetail: postal?.townCode ? postal.addressDetail : reg.address,
	};
}

export async function lookupBusinessRegistration(
	taxId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<BusinessLookup | null> {
	if (!/^\d{8}$/.test(taxId.trim())) return null;
	const res = await client().lookupBusinessRegistration({
		taxId: taxId.trim(),
	});
	return businessLookupOf(res.registration);
}
