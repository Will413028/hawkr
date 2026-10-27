/**
 * 商品認證申請的顯示狀態判讀（19 號設計書 §5 狀態機 / §9.1 賣家端點）。
 *
 * 三個認證輸入面（商品建立與編輯、CSV 匯入逐筆編輯、代營運工作區）共用這一層，
 * 所以「一筆認證現在算什麼狀態」只有一個答案。
 *
 * ⚠️ 兩個資料來源的語意不同，混用就會說謊：
 *
 *  - `product.certifications`（本檔的 `entry`）自 19 號起是 **approved-only 公開投影**
 *    —— 出現在裡面即代表平台已核准。但這只在「讀一個已存在的商品」時成立：CSV 匯入
 *    項目的 `certifications` 是尚未送審的原始宣稱，用同一套判讀會把它標成「已通過」，
 *    正是本次要移除的那個 bug（無條件 `verified: true`）。
 *  - `certification_applications`（本檔的 `applications`）才帶審核狀態，而它**需要賣家
 *    登入態**。代行工作區的瀏覽器在 www 上沒有登入態（同一條約束見
 *    `seller-repo.ts` 的 `fetchPublicCategoryTree`），拿不到這份清單。
 *
 * ⇒ 呼叫端一律宣告自己是哪一種面（{@link CertificationSurfaceMode}），本模組據此決定
 *   可以讀哪個來源、以及不准推導什麼。少了這個宣告，「代行看到的狀態是猜的」這種錯誤
 *   不會有任何東西轉紅。
 */

import {
	PRODUCT_CERTIFICATION_REJECT_REASON_LABEL,
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	ProductCertificationStatus,
	type ProductCertificationStatusValue,
} from "@/lib/codemeta/enums";
import type {
	ProductCertificationApplication,
	ProductCertificationEntry,
} from "./seller-repo";

/**
 * 這個輸入面的能力與資料語意。三種面各自對應一組真實限制，不是樣式開關：
 *
 * - `"review"`   商品編輯頁。商品已存在 ⇒ 有 productId、有賣家登入態，
 *                `entry` 是 approved-only 投影、`applications` 可用，可送審與撤回。
 * - `"draft"`    商品建立頁與 CSV 匯入逐筆編輯。商品還不存在 ⇒ 沒有申請端點可打，
 *                `entry` 是**尚未送審的草稿宣稱**，因此一律不得顯示任何審核狀態。
 * - `"readonly"` 代營運工作區。**沒有賣家登入態 ⇒ 不做即時驗證、不呼叫任何認證端點**，
 *                只能從 approved-only 投影推導，且不提供送審與撤回。
 */
export type CertificationSurfaceMode = "review" | "draft" | "readonly";

/** 徽章色調。與 `components/ui/Chip` 的 `ChipColor` 同名，由元件層直接對映。 */
export type CertStatusTone =
	| "default"
	| "primary"
	| "success"
	| "warning"
	| "danger";

export interface CertStatusView {
	status: ProductCertificationStatusValue;
	tone: CertStatusTone;
	/** 徽章文字。**一律取自 CodeMeta 生成表**，本檔不得自己寫碼值到中文的對照。 */
	label: string;
	/** 徽章下方的一句說明；駁回與撤銷會優先用 ADM 填的 `publicReason`。 */
	description?: string;
}

const TONE_BY_STATUS: Record<ProductCertificationStatusValue, CertStatusTone> =
	{
		// 琥珀＝等待中，與 ADM 佇列的「審核中」同一個色語意。
		// 不可與 APPROVED 的 success 同色系：賣家掃一眼要分得出「已生效」與「還在等」，
		// 兩者都是綠色時只剩文字可讀（見 certification-status-tone.unit.test.ts）。
		[ProductCertificationStatus.SUBMITTED]: "warning",
		[ProductCertificationStatus.APPROVED]: "success",
		[ProductCertificationStatus.REJECTED]: "danger",
		[ProductCertificationStatus.WITHDRAWN]: "default",
		[ProductCertificationStatus.SUPERSEDED]: "default",
		[ProductCertificationStatus.REVOKED]: "danger",
	};

/**
 * 沒有 `publicReason` 時的預設說明。
 *
 * 每一句都在回答「所以買家現在看不看得到這個標章」——那是賣家在這一格唯一真正關心的
 * 事，只印狀態名（「已被新號碼取代」）並不會讓人知道商品頁上少了什麼。
 */
const DESCRIPTION_BY_STATUS: Record<ProductCertificationStatusValue, string> = {
	[ProductCertificationStatus.SUBMITTED]:
		"平台審核中，通過後才會顯示在商品頁。",
	[ProductCertificationStatus.APPROVED]: "買家在商品頁看得到這個標章。",
	[ProductCertificationStatus.REJECTED]: "未通過，修正後可重新送出。",
	[ProductCertificationStatus.WITHDRAWN]: "已撤回，商品頁不再顯示這個標章。",
	[ProductCertificationStatus.SUPERSEDED]:
		"已由同類型的新證書編號取代，商品頁顯示新的那一張。",
	[ProductCertificationStatus.REVOKED]: "平台已撤銷，商品頁不再顯示這個標章。",
};

/** 四個終態（19 §5.1）：不再轉出，也就不會有「還在處理中」的期待。 */
const TERMINAL_STATUSES: ReadonlySet<string> = new Set<string>([
	ProductCertificationStatus.REJECTED,
	ProductCertificationStatus.WITHDRAWN,
	ProductCertificationStatus.SUPERSEDED,
	ProductCertificationStatus.REVOKED,
]);

export function isTerminalCertStatus(status: string | undefined): boolean {
	return TERMINAL_STATUSES.has((status ?? "").trim());
}

/**
 * 一筆申請 → 徽章。
 *
 * 認不得的碼值回 `null`（**不畫空徽章**）：一個沒有文字的徽章比沒有徽章更糟，它讓人
 * 以為自己看到了狀態。
 */
export function certStatusView(
	status: string | undefined,
	options: { publicReason?: string | null; reasonCode?: string | null } = {},
): CertStatusView | null {
	const code = (status ?? "").trim() as ProductCertificationStatusValue;
	const label = PRODUCT_CERTIFICATION_STATUS_LABEL[code];
	if (!label) return null;

	const reason = (options.publicReason ?? "").trim();
	const reasonCode = (options.reasonCode ?? "").trim();
	const reasonCodeLabel = reasonCode
		? PRODUCT_CERTIFICATION_REJECT_REASON_LABEL[reasonCode]
		: undefined;

	return {
		status: code,
		tone: TONE_BY_STATUS[code] ?? "default",
		label,
		description: reason || reasonCodeLabel || DESCRIPTION_BY_STATUS[code],
	};
}

export interface CertTypeView {
	/** 這個類型現在有沒有被宣稱（決定勾選框勾不勾）。 */
	checked: boolean;
	/** 顯示在證書編號欄的號碼。 */
	certNo: string;
	/** 徽章。**沒有可信來源時為 `null`——不猜**。 */
	badge: CertStatusView | null;
	/** 進行中的申請（`SUBMITTED`）。只有 `review` 模式可能有值。 */
	pending: ProductCertificationApplication | null;
	/** 目前生效的已核准申請。只有 `review` 模式可能有值。 */
	approved: ProductCertificationApplication | null;
	/**
	 * 換號情境（19 §6.4）：新號碼待審時舊標章仍然公開，這裡是「買家現在看到的那一個
	 * 號碼」。待審號碼與公開號碼相同時為 `null`。
	 */
	publicCertNo: string | null;
	/** 取消勾選要撤回的申請，依序處理（待審的先撤，再撤已核准的）。 */
	withdrawable: ProductCertificationApplication[];
}

const EMPTY_VIEW: CertTypeView = {
	checked: false,
	certNo: "",
	badge: null,
	pending: null,
	approved: null,
	publicCertNo: null,
	withdrawable: [],
};

/** 較新的排前面。`submittedAt` 是 ISO 8601 UTC，字典序即時序。 */
function byRecencyDesc(
	a: ProductCertificationApplication,
	b: ProductCertificationApplication,
): number {
	const at = a.submittedAt ?? a.reviewedAt ?? "";
	const bt = b.submittedAt ?? b.reviewedAt ?? "";
	if (at === bt) return 0;
	return at < bt ? 1 : -1;
}

/**
 * 一個認證類型在這個面上該長什麼樣。
 *
 * `review` 模式的優先序是「進行中 > 已核准 > 最近一個終態」：
 * 賣家最需要看到的是「我送的那一筆現在怎麼了」，其次才是「目前公開的是哪一張」。
 * 終態也要顯示（而不是留白），因為被駁回或撤銷的那一筆**既不在投影也不在待審清單**
 * ——留白就等於賣家不知道自己的申請發生過什麼事。
 */
export function resolveCertTypeView(input: {
	code: string;
	mode: CertificationSurfaceMode;
	/**
	 * 只讀 `certNo` 與「在不在」，刻意不要求完整的 {@link ProductCertificationEntry}：
	 * 代營運工作區的清單可能帶著認不得的歷史碼值，強制它先轉型別只會逼出一個 cast。
	 */
	entry?: Pick<ProductCertificationEntry, "certNo"> | undefined;
	applications?: readonly ProductCertificationApplication[] | undefined;
}): CertTypeView {
	const { code, mode, entry } = input;
	const entryCertNo = (entry?.certNo ?? "").trim();

	if (mode === "draft") {
		// 草稿宣稱沒有審核狀態，**任何徽章都是編造的**。
		if (!entry) return EMPTY_VIEW;
		return { ...EMPTY_VIEW, checked: true, certNo: entryCertNo };
	}

	if (mode === "readonly") {
		// 沒有登入態就拿不到申請清單。能誠實說出口的只有投影本身的語意：
		// 出現在 approved-only 投影裡 ⇒ 已核准。刻意忽略傳進來的 applications，
		// 免得日後有人在這個面上接了一份不該取得的資料而沒有任何東西轉紅。
		if (!entry) return EMPTY_VIEW;
		return {
			...EMPTY_VIEW,
			checked: true,
			certNo: entryCertNo,
			badge: certStatusView(ProductCertificationStatus.APPROVED),
		};
	}

	const mine = (input.applications ?? [])
		.filter((app) => (app.certificationType ?? "").trim() === code)
		.slice()
		.sort(byRecencyDesc);

	const pending =
		mine.find((app) => app.status === ProductCertificationStatus.SUBMITTED) ??
		null;
	const approved =
		mine.find((app) => app.status === ProductCertificationStatus.APPROVED) ??
		null;
	const latestTerminal = mine.find((app) => isTerminalCertStatus(app.status));

	const governing = pending ?? approved ?? latestTerminal ?? null;

	// 投影是權威的「買家現在看得到什麼」。申請清單拿不到時（端點還沒部署、或這次沒帶）
	// 仍要靠投影答出已核准，否則編輯頁會退化成一片空白。
	const badge = governing
		? certStatusView(governing.status, {
				publicReason: governing.publicReason,
				reasonCode: governing.reasonCode,
			})
		: entry
			? certStatusView(ProductCertificationStatus.APPROVED)
			: null;

	const approvedCertNo = (approved?.certificateNo ?? entryCertNo ?? "").trim();
	const certNo = (
		pending?.certificateNo ||
		approvedCertNo ||
		latestTerminal?.certificateNo ||
		""
	).trim();

	return {
		checked: Boolean(pending || approved || entry),
		certNo,
		badge,
		pending,
		approved,
		publicCertNo:
			approvedCertNo && approvedCertNo !== certNo ? approvedCertNo : null,
		withdrawable: [pending, approved].filter(
			(app): app is ProductCertificationApplication => app !== null,
		),
	};
}
