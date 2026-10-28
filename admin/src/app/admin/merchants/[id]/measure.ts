import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	AdminRevokeReason,
	type CreateMeasureRequestSchema,
	type TerminationInputSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	LinkedIdentity,
	type Measure,
	type TerminationOption,
	TerminationViolationRequirement,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasureState,
	RevokeReason,
	RiskGround,
	type TerminationGround,
	type ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import {
	sellerMeasureAppealDeadline,
	sellerMeasureBasis,
	sellerMeasureKind,
	sellerMeasureState,
	sellerRevokeReason,
	sellerRiskGround,
	sellerTerminationGround,
	sellerViolationSeverity,
} from "@hawkr/ui-text/keys";
import { formatStaff } from "@hawkr/ui-text/zh-TW";
import { formatDateTimeShort } from "@/lib/format";
import { staffTextOrUnknown, unknownText } from "@/lib/ui-text";

// 商家詳情的措施（SellerGovernanceAdminService：CreateMeasure、EndMeasure、ListMeasures；GetStore.active_measures）的顯示判斷、
// 輸入驗證與請求組裝（設計 step10e-2c-design §3.3、§3.6、§4「admin」）。畫面只接結果。時間一律由呼叫端傳入（now），這裡不讀時鐘。

// ---------------------------------------------------------------------------
// 風險類型

/** 風險類型下拉的選項順序（不含 UNSPECIFIED；UNSPECIFIED 不是有效的送出值）。 */
export const RISK_GROUND_OPTIONS: readonly RiskGround[] = [
	RiskGround.TRANSACTION,
	RiskGround.FOOD_SAFETY,
	RiskGround.FRAUD,
	RiskGround.PERSONAL_DATA,
	RiskGround.INFRINGEMENT,
	RiskGround.SECURITY,
	RiskGround.FULFILLMENT,
	RiskGround.EXTERNAL_ORDER,
];

/** 風險類型（條款第 17.1 條）與外部要求（第 17.5 條）的顯示文字，取自共用文案目錄；UNSPECIFIED 或不認得的值是「未知」。 */
export function riskGroundLabel(ground: RiskGround): string {
	return staffTextOrUnknown(sellerRiskGround(ground));
}

// ---------------------------------------------------------------------------
// 措施的狀態與依據

const STATE_VARIANT: Readonly<
	Partial<Record<MeasureState, "danger" | "warning" | "gray">>
> = {
	[MeasureState.ISSUED]: "warning",
	[MeasureState.SCHEDULED]: "warning",
	[MeasureState.ACTIVE]: "danger",
	[MeasureState.ENDED]: "gray",
	[MeasureState.REVOKED]: "gray",
};

/**
 * 措施此刻的狀態（伺服器在讀取時推導的 `state`）：文字取自共用文案目錄（生效中的終止合作是「合作已終止」），
 * 顏色是 admin 的呈現。UNSPECIFIED 與不認得的值是灰色的「未知」。
 */
export function measureStateBadge(
	state: MeasureState,
	kind: MeasureKind,
): { label: string; variant: "danger" | "warning" | "gray" } {
	return {
		label: staffTextOrUnknown(sellerMeasureState(state, { kind })),
		variant: STATE_VARIANT[state] ?? "gray",
	};
}

/** 措施依據的顯示文字，取自共用文案目錄；UNSPECIFIED 與不認得的值是「未知」。 */
export function measureBasisLabel(basis: MeasureBasis): string {
	return staffTextOrUnknown(sellerMeasureBasis(basis));
}

/**
 * 措施的種類與依據：風險依據帶風險類型，違規依據指向違規紀錄，契約依據（終止合作）帶終止事由；
 * 終止合作的違規依據（重大違約）同樣帶終止事由。
 */
export function measureBasisText(
	measure: Pick<Measure, "kind" | "basis" | "riskGround" | "terminationGround">,
): string {
	const kindMessage = sellerMeasureKind(measure.kind);
	const basisMessage = sellerMeasureBasis(measure.basis);
	// 種類或依據是目錄不認得的值（含 UNSPECIFIED）時整句是「未知」，不組出「未知・…」的半句。
	if (kindMessage === undefined || basisMessage === undefined) {
		return unknownText();
	}
	const kind = formatStaff(kindMessage);
	const basis = formatStaff(basisMessage);
	if (measure.kind === MeasureKind.TERMINATION) {
		return `${kind}・${basis}：${terminationGroundLabel(measure.terminationGround)}`;
	}
	switch (measure.basis) {
		case MeasureBasis.RISK:
			return `${kind}・${basis}：${riskGroundLabel(measure.riskGround)}`;
		case MeasureBasis.VIOLATION:
			return `${kind}・${basis}`;
		default:
			// CONTRACT 只配終止合作；其他種類配 CONTRACT 不是有效資料。
			return unknownText();
	}
}

/** 認定嚴重度與措施種類的顯示文字，取自共用文案目錄；UNSPECIFIED 與不認得的值是「未知」。類別的文字是 `@/lib/policy-category` 的 `policyCategoryText`。 */
export function violationSeverityText(severity: ViolationSeverity): string {
	return staffTextOrUnknown(sellerViolationSeverity(severity));
}

export function measureKindLabel(kind: MeasureKind): string {
	return staffTextOrUnknown(sellerMeasureKind(kind));
}

// ---------------------------------------------------------------------------
// 終止合作（條款第 21 條）

/**
 * 伺服器此刻允許建立的終止合作（GetStoreResponse.available_terminations）對應的對話框選項：順序與內容都照伺服器給的（伺服器依 TerminationGround 的
 * 數字順序、不含 LINKED_ACCOUNT），只丟掉沒有顯示文字的事由（UNSPECIFIED、不認得的值）。每個選項帶生效時機與條件的說明句，以及依據的需求。
 */
export function availableTerminationOptions(
	available: readonly TerminationOption[],
): TerminationOption[] {
	return available.filter(
		(option) => sellerTerminationGround(option.ground) !== undefined,
	);
}

/** 「終止合作」入口與下拉的可用性，全部來自伺服器（GetStoreResponse）；沒有可選事由時入口不顯示（不留按了沒有選項的控制）。 */
export interface TerminationAvailability {
	canCreate: boolean;
	/** 伺服器給的可建立的終止合作（每個事由一筆，生效時機、說明句與依據的需求）。 */
	options: TerminationOption[];
}

export function terminationAvailability(
	detail:
		| {
				canCreateTermination: boolean;
				availableTerminations: readonly TerminationOption[];
		  }
		| undefined,
): TerminationAvailability {
	const options = detail
		? availableTerminationOptions(detail.availableTerminations)
		: [];
	return {
		canCreate: detail?.canCreateTermination === true && options.length > 0,
		options,
	};
}

/** 終止事由的顯示文字，取自共用文案目錄；UNSPECIFIED 或不認得的值是「未知」。 */
export function terminationGroundLabel(ground: TerminationGround): string {
	return staffTextOrUnknown(sellerTerminationGround(ground));
}

/** 兩個帳號相同的身分類別（admin measure.proto LinkedIdentity 各值註解的標籤）。UNSPECIFIED 不是有效值。 */
export const LINKED_IDENTITY_LABEL: Record<
	Exclude<LinkedIdentity, LinkedIdentity.UNSPECIFIED>,
	string
> = {
	[LinkedIdentity.NATIONAL_ID]: "身分證號",
	[LinkedIdentity.UBN]: "統編",
	[LinkedIdentity.RESPONSIBLE_PERSON_ID]: "負責人身分證號",
};

/** 命中的身分類別的顯示文字；UNSPECIFIED 或不認得的值是「未知」。 */
export function linkedIdentityLabel(identity: LinkedIdentity): string {
	return (
		(LINKED_IDENTITY_LABEL as Partial<Record<LinkedIdentity, string>>)[
			identity
		] ?? unknownText()
	);
}

/** 申覆期限（Measure.appeal_deadline_time，伺服器給；只有終止合作有）；沒有時回 undefined（畫面不顯示）。 */
export function appealDeadlineText(
	measure: Pick<Measure, "appealDeadlineTime">,
): string | undefined {
	if (!measure.appealDeadlineTime) return undefined;
	return formatStaff(
		sellerMeasureAppealDeadline({
			deadline: formatDateTimeShort(timestampDate(measure.appealDeadlineTime)),
		}),
	);
}

/**
 * 「結束時間」列的值，卡片與詳情頁共用：有 end_time 顯示時間；沒有 end_time 而有 review_by（開放式的風險停權，由平台定期複審）
 * 顯示「未定」；兩者都沒有（警告、終止合作）回 undefined，畫面不顯示這一列。
 */
export function endTimeText(
	measure: Pick<Measure, "endTime" | "reviewBy">,
): string | undefined {
	if (measure.endTime)
		return formatDateTimeShort(timestampDate(measure.endTime));
	return measure.reviewBy ? "未定（由平台定期複審）" : undefined;
}

/** 撤銷原因的顯示文字，取自共用文案目錄；UNSPECIFIED 與不認得的值是「未知」。 */
export function revokeReasonLabel(reason: RevokeReason): string {
	return staffTextOrUnknown(sellerRevokeReason(reason));
}

/** 管理員撤銷時可以選的原因（AdminRevokeReason）對應到共用的 RevokeReason，文字取自目錄。UNSPECIFIED 不是有效值。 */
const ADMIN_REVOKE_REASON_TO_REVOKE_REASON: Record<
	Exclude<AdminRevokeReason, AdminRevokeReason.UNSPECIFIED>,
	RevokeReason
> = {
	[AdminRevokeReason.ADMIN_CORRECTION]: RevokeReason.ADMIN_CORRECTION,
	[AdminRevokeReason.EXTERNAL_DECISION]: RevokeReason.EXTERNAL_DECISION,
	[AdminRevokeReason.OTHER]: RevokeReason.OTHER,
};

export function adminRevokeReasonLabel(reason: AdminRevokeReason): string {
	const mapped = (
		ADMIN_REVOKE_REASON_TO_REVOKE_REASON as Partial<
			Record<AdminRevokeReason, RevokeReason>
		>
	)[reason];
	return mapped === undefined ? unknownText() : revokeReasonLabel(mapped);
}

/** 已撤銷措施的撤銷時間與原因（「時間・原因」）；沒有撤銷時回 undefined。 */
export function revokeDetailText(
	measure: Pick<Measure, "state" | "revokeTime" | "revokeReason">,
): string | undefined {
	if (measure.state !== MeasureState.REVOKED) return undefined;
	const time = measure.revokeTime
		? formatDateTimeShort(timestampDate(measure.revokeTime))
		: "-";
	return `${time}・${revokeReasonLabel(measure.revokeReason)}`;
}

// ---------------------------------------------------------------------------
// 建立風險措施：輸入、驗證、請求

export const STATEMENT_MAX_LENGTH = 2000;
export const NOTE_MAX_LENGTH = 2000;
/** 複審期限的預設與上限（設計書 32 §1.1、CreateMeasureRequest.review_by：晚於現在、不超過 30 天）。 */
export const REVIEW_DEFAULT_DAYS = 7;
export const REVIEW_MAX_DAYS = 30;

/** 一天的毫秒數（台北沒有日光節約，一天固定是 24 小時）。 */
export const DAY_MS = 24 * 60 * 60 * 1000;

/** 對話框開啟時複審期限的預設值：現在起 7 天。 */
export function defaultReviewBy(now: Date): Date {
	return new Date(now.getTime() + REVIEW_DEFAULT_DAYS * DAY_MS);
}

/** `datetime-local` 輸入框的值：台北時間（ADM 一律台北釘死，不跟著瀏覽器時區），到分鐘。 */
export function toLocalInput(date: Date): string {
	return formatDateTimeShort(date).replace(/\//gu, "-").replace(" ", "T");
}

/**
 * 帶秒的 `datetime-local` 值（台北時間，`step="1"` 的輸入框用）。預設「現在」要精確到秒：整分的預設值在同一分鐘內
 * 會早於剛建立的紀錄，被「收到的時間不能早於詢問建立」擋下。
 */
export function toLocalInputSeconds(date: Date): string {
	const seconds = Math.floor((date.getTime() % 60000) / 1000);
	return `${toLocalInput(date)}:${String(seconds).padStart(2, "0")}`;
}

/**
 * 解析 `datetime-local` 的值（台北時間）；空字串或格式不對回 undefined。秒是選填（`step="1"` 的輸入框才有）：
 * 沒有秒就是整分。
 */
export function fromLocalInput(value: string): Date | undefined {
	const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})(:\d{2})?$/u.exec(value);
	if (!match) return undefined;
	const ms = Date.parse(`${match[1]}${match[2] ?? ":00"}+08:00`);
	return Number.isNaN(ms) ? undefined : new Date(ms);
}

/**
 * id 集合（來源通報、引用的認定）的正規形式：依字串遞增排序的複本。這些清單是集合，沒有順序：請求與 request_id 的鍵都用它，
 * 取消勾選再勾回、換勾選順序不會變成另一個請求（伺服器的 request hash 同樣先排序，沒有例外，measure.Create）。
 * 措施的主要依據（lead_finding_id）由伺服器的規則決定，不看清單的位置：有新的認定用新的認定，否則取引用的認定中最嚴重的，
 * 同嚴重度取最新建立的。
 */
export function sortedIds(ids: readonly string[]): string[] {
	return [...ids].sort();
}

/**
 * 從案件建立措施時帶的來源案件（CreateMeasureRequest.source_case_id）。沒有案件（空字串或沒給）時請求不帶這個欄位；
 * 帶了就是 request hash 的一部分：同一個 request_id 換案件是不同的請求。
 */
export function sourceCaseField(sourceCaseId: string | undefined): {
	sourceCaseId?: string;
} {
	return sourceCaseId ? { sourceCaseId } : {};
}

/**
 * 從主管機關來函建立措施時帶的來源來函（CreateMeasureRequest.source_authority_order_id）。沒有來函時請求不帶這個欄位；
 * 帶了就是 request hash 的一部分：同一個 request_id 換來函是不同的請求。
 */
export function sourceAuthorityOrderField(
	sourceAuthorityOrderId: string | undefined,
): { sourceAuthorityOrderId?: string } {
	return sourceAuthorityOrderId ? { sourceAuthorityOrderId } : {};
}

/** 從來函建立措施時三個對話框頂端的說明（M2）。 */
export function sourceAuthorityOrderNote(documentNo: string): string {
	return `這個措施會記為主管機關來函 ${documentNo} 導出的措施。`;
}

/** 對話框的輸入。時間欄位是 `datetime-local` 的字串（空字串＝沒填）。 */
export interface RiskMeasureForm {
	ground: RiskGround;
	/** 給賣家的理由。 */
	statement: string;
	/** 結束時間（選填；不填是沒有固定期間，由平台定期複審）。 */
	endTime: string;
	/** 複審期限（必填）。 */
	reviewBy: string;
	/** 第一則內部註記（賣家看不到）。 */
	note: string;
	/** 從案件建立時的案件 id；沒有或空字串是不連案件。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函 id；沒有或空字串是不連來函。 */
	sourceAuthorityOrderId?: string;
}

/**
 * 複審期限（`datetime-local` 的值，台北時間）的檢查，建立風險措施與延後複審共用，規則同伺服器：必填、格式正確、晚於現在、
 * 不超過現在起 30 天。`resend` 的意義同 validateRiskMeasure（重送時不再用現在的時間檢查）。沒有錯誤回 undefined。
 */
export function reviewByError(
	value: string,
	now: Date,
	resend = false,
): string | undefined {
	if (value === "") return "請填寫複審期限。";
	const review = fromLocalInput(value);
	if (!review) return "複審期限的格式不正確。";
	if (!resend && review.getTime() <= now.getTime()) {
		return "複審期限必須晚於現在。";
	}
	if (!resend && review.getTime() > now.getTime() + REVIEW_MAX_DAYS * DAY_MS) {
		return `複審期限最多為現在起 ${REVIEW_MAX_DAYS} 天。`;
	}
	return undefined;
}

/**
 * 送出前的檢查，與伺服器的規則一致（CreateMeasureRequest）：風險類型必須是選項裡的值；理由與註記去掉前後空白後 1–2000 字；
 * 結束時間若填必須晚於現在；複審期限必填、晚於現在、不超過現在起 30 天。回傳第一個錯誤，沒有錯誤回 undefined。
 * `resend` 是同一份內容的重送（沿用上一次的 request_id）：上一次可能已經建立成功，伺服器依 request_id 回傳原本的措施，
 * 與現在的時間無關，所以不再用現在的時間檢查結束時間與複審期限；格式與必填照常檢查。
 */
export function validateRiskMeasure(
	form: RiskMeasureForm,
	now: Date,
	resend = false,
): string | undefined {
	if (!RISK_GROUND_OPTIONS.includes(form.ground)) return "請選擇風險類型。";
	const statement = trimGoSpace(form.statement);
	if (statement.length === 0) return "請填寫給賣家的理由。";
	if (goTrimmedRuneCount(statement) > STATEMENT_MAX_LENGTH) {
		return `給賣家的理由最多 ${STATEMENT_MAX_LENGTH} 字。`;
	}
	if (form.endTime !== "") {
		const end = fromLocalInput(form.endTime);
		if (!end) return "結束時間的格式不正確。";
		if (!resend && end.getTime() <= now.getTime()) {
			return "結束時間必須晚於現在。";
		}
	}
	const reviewError = reviewByError(form.reviewBy, now, resend);
	if (reviewError) return reviewError;
	const note = trimGoSpace(form.note);
	if (note.length === 0) return "請填寫內部註記。";
	if (goTrimmedRuneCount(note) > NOTE_MAX_LENGTH) {
		return `內部註記最多 ${NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

/**
 * CreateMeasure 的請求：風險依據的停權措施。理由與註記送去掉前後空白的內容；結束時間沒填就不送（開放式）。
 * 呼叫前須先通過 validateRiskMeasure（時間欄位解析不出來時丟例外，不送出缺值的請求）。
 */
export function buildCreateRiskMeasure(
	requestId: string,
	storeId: string,
	form: RiskMeasureForm,
): MessageInitShape<typeof CreateMeasureRequestSchema> {
	const review = fromLocalInput(form.reviewBy);
	if (!review) throw new Error("複審期限不是有效的時間");
	const end = form.endTime === "" ? undefined : fromLocalInput(form.endTime);
	if (form.endTime !== "" && !end) throw new Error("結束時間不是有效的時間");
	return {
		requestId,
		storeId,
		statement: trimGoSpace(form.statement),
		note: trimGoSpace(form.note),
		...sourceCaseField(form.sourceCaseId),
		...sourceAuthorityOrderField(form.sourceAuthorityOrderId),
		measure: {
			case: "suspension",
			value: {
				basis: { case: "risk", value: { ground: form.ground } },
				endTime: end ? timestampFromDate(end) : undefined,
				reviewBy: timestampFromDate(review),
			},
		},
	};
}

// ---------------------------------------------------------------------------
// 建立終止合作：輸入、驗證、請求、說明文字（條款第 21.2、21.3 條；W9 的關聯帳號終止也走這裡）

/**
 * 選到的終止合作，記住選項從哪裡來：`available` 是店家詳情的 available_terminations（一般的事由），依據的認定依選項的
 * violationRequirement 收；`linked` 是關聯帳號的 termination_option（W9），帶來源措施 id，沒有依據。
 * 請求的事由是 option.ground，來源措施只在 linked 存在（型別保證，不需要比對事由）。
 */
export type TerminationChoice =
	| {
			from: "available";
			option: TerminationOption;
			/** 依據的認定 id（集合，送出時排序，見 sortedIds）；只在選項的需求不是 FORBIDDEN 時送出。 */
			citedFindingIds: readonly string[];
	  }
	| { from: "linked"; option: TerminationOption; sourceMeasureId: string };

/** 終止合作對話框的輸入。 */
export interface TerminationForm {
	/** 選到的終止合作；還沒選是 undefined。 */
	choice: TerminationChoice | undefined;
	/** 給賣家的理由。 */
	statement: string;
	/** 第一則內部註記（賣家看不到）。 */
	note: string;
	/** 從案件建立時的案件 id；沒有或空字串是不連案件。 */
	sourceCaseId?: string;
	/** 從主管機關來函建立時的來函 id；沒有或空字串是不連來函。 */
	sourceAuthorityOrderId?: string;
}

/** 已選到終止合作的輸入（通過 validateTermination 之後）。 */
export type ReadyTerminationForm = Omit<TerminationForm, "choice"> & {
	choice: TerminationChoice;
};

/** 請求要送的依據的認定 id：選項的需求是 FORBIDDEN（或不認得）與來自關聯帳號時不送；OPTIONAL 沒選也不送。 */
function citedFindingIdsToSend(choice: TerminationChoice): readonly string[] {
	if (choice.from !== "available") return [];
	switch (choice.option.violationRequirement) {
		case TerminationViolationRequirement.REQUIRED:
		case TerminationViolationRequirement.OPTIONAL:
			return choice.citedFindingIds;
		default:
			return [];
	}
}

/** 「依據的認定」欄位的呈現：需求是 REQUIRED 或 OPTIONAL 才顯示，FORBIDDEN 與不認得的值不顯示；條件由伺服器的說明句呈現，這裡不寫規則。 */
export function citedFindingsFieldFor(
	option: TerminationOption,
): { required: boolean; emptyText: string; noteText: string } | undefined {
	switch (option.violationRequirement) {
		case TerminationViolationRequirement.REQUIRED:
			return {
				required: true,
				emptyText: "這個帳號沒有生效中的認定，無法以這個事由終止。",
				noteText: "至少選一筆。",
			};
		case TerminationViolationRequirement.OPTIONAL:
			return {
				required: false,
				emptyText: "這個帳號沒有生效中的認定。",
				noteText: "選填。",
			};
		default:
			return undefined;
	}
}

/** validateTermination 的結果：有錯誤回第一個錯誤；通過回已選到終止合作的輸入（可直接送出）。 */
export type TerminationValidation =
	| { error: string; form?: undefined }
	| { error?: undefined; form: ReadyTerminationForm };

/**
 * 送出前的檢查，與伺服器的規則一致（CreateMeasureRequest、TerminationInput）：要選到一個終止合作；需求是 REQUIRED 時至少選一筆
 * 依據的認定（引用的認定夠不夠久、有沒有撤銷由伺服器檢查）；關聯帳號的來源措施由型別保證；理由與註記去掉前後空白後 1–2000 字。
 * 終止沒有結束時間與複審期限。回傳第一個錯誤，或通過的輸入（choice 一定有值）。
 */
export function validateTermination(
	form: TerminationForm,
): TerminationValidation {
	const { choice } = form;
	if (!choice) return { error: "請選擇終止事由。" };
	if (
		choice.option.violationRequirement ===
			TerminationViolationRequirement.REQUIRED &&
		citedFindingIdsToSend(choice).length === 0
	) {
		return { error: "請選擇依據的認定。" };
	}
	const statement = trimGoSpace(form.statement);
	if (statement.length === 0) return { error: "請填寫給賣家的理由。" };
	if (goTrimmedRuneCount(statement) > STATEMENT_MAX_LENGTH) {
		return { error: `給賣家的理由最多 ${STATEMENT_MAX_LENGTH} 字。` };
	}
	const note = trimGoSpace(form.note);
	if (note.length === 0) return { error: "請填寫內部註記。" };
	if (goTrimmedRuneCount(note) > NOTE_MAX_LENGTH) {
		return { error: `內部註記最多 ${NOTE_MAX_LENGTH} 字。` };
	}
	return { form: { ...form, choice } };
}

/**
 * CreateMeasure 的請求：終止合作。事由是選項的 ground；依據只送選項所需的欄位：有選依據的認定（需求不是 FORBIDDEN）帶引用的認定，
 * 關聯帳號帶來源措施（伺服器對多餘的欄位回 invalid_argument）。呼叫前須先通過 validateTermination（choice 一定有值）。
 */
export function buildCreateTermination(
	requestId: string,
	storeId: string,
	form: ReadyTerminationForm,
): MessageInitShape<typeof CreateMeasureRequestSchema> {
	const { choice } = form;
	const termination: MessageInitShape<typeof TerminationInputSchema> = {
		ground: choice.option.ground,
	};
	const cited = citedFindingIdsToSend(choice);
	if (cited.length > 0) {
		termination.violation = { citedFindingIds: sortedIds(cited) };
	}
	if (choice.from === "linked") {
		termination.sourceMeasureId = choice.sourceMeasureId;
	}
	return {
		requestId,
		storeId,
		statement: trimGoSpace(form.statement),
		note: trimGoSpace(form.note),
		...sourceCaseField(form.sourceCaseId),
		...sourceAuthorityOrderField(form.sourceAuthorityOrderId),
		measure: { case: "termination", value: termination },
	};
}

/** 終止合作的 request_id 鍵：與送出的請求一一對應（只含會送出的欄位；認定 id 是集合，用排序後的清單）。 */
export function terminationRequestKey(
	storeId: string,
	form: ReadyTerminationForm,
): string {
	const { choice } = form;
	return JSON.stringify([
		storeId,
		choice.option.ground,
		trimGoSpace(form.statement),
		trimGoSpace(form.note),
		sortedIds(citedFindingIdsToSend(choice)),
		choice.from === "linked" ? choice.sourceMeasureId : "",
		form.sourceCaseId ?? "",
		form.sourceAuthorityOrderId ?? "",
	]);
}

/** 終止合作對話框的共同說明（T7）。 */
export const TERMINATION_COMMON_TEXT =
	"未結訂單完成後店家會關閉。終止生效後 6 個月內撤銷或申覆成立，可恢復同一家店；之後只能撤銷紀錄。";

/**
 * 終止合作的說明（T7）：第一則是伺服器給的生效時機與這個事由的建立條件（TerminationOption.explanation，天數、生效時間與條件
 * 由伺服器依自己的規則寫好，這裡不推算也不補規則），再加共同說明。還沒選（option 是 undefined）沒有說明（回空陣列）。
 */
export function terminationExplanation(
	option: TerminationOption | undefined,
): string[] {
	if (!option) return [];
	const lead = option.explanation?.message;
	return [...(lead ? [lead] : []), TERMINATION_COMMON_TEXT];
}

/**
 * 商家詳情「預定終止：{日期時間}」：帳號範圍、state=SCHEDULED 的終止措施（ListMeasures）中，生效時間（period.start_time）最早的一筆。
 * 沒有預定生效的終止、或那一筆沒有生效時間（不是有效資料）時回 undefined，畫面不顯示。
 */
export function scheduledTerminationText(
	measures: readonly Pick<Measure, "kind" | "state" | "period">[],
): string | undefined {
	let earliest: Date | undefined;
	for (const m of measures) {
		if (
			m.kind !== MeasureKind.TERMINATION ||
			m.state !== MeasureState.SCHEDULED ||
			!m.period?.startTime
		) {
			continue;
		}
		const start = timestampDate(m.period.startTime);
		if (!earliest || start.getTime() < earliest.getTime()) earliest = start;
	}
	return earliest ? `預定終止：${formatDateTimeShort(earliest)}` : undefined;
}

// ---------------------------------------------------------------------------
// request_id（AIP-155）：同一份內容的重送沿用同一個，內容變了換新的

/** 目前這個 request_id 是為哪一份內容產生的。 */
export interface RequestIdSlot {
	key: string;
	id: string;
}

/** 輸入內容的鍵：去掉前後空白後的內容，與送出的請求一一對應。 */
export function requestKey(storeId: string, form: RiskMeasureForm): string {
	return JSON.stringify([
		storeId,
		form.ground,
		trimGoSpace(form.statement),
		form.endTime,
		form.reviewBy,
		trimGoSpace(form.note),
		form.sourceCaseId ?? "",
		form.sourceAuthorityOrderId ?? "",
	]);
}

/** 這份內容是不是上一次送出過的（同一個 request_id 的重送）。 */
export function isResend(slot: RequestIdSlot | null, key: string): boolean {
	return slot !== null && slot.key === key;
}

/**
 * 這一份內容要用的 request_id。同一份內容重送（上一次失敗或沒有回應）沿用同一個，伺服器才認得是同一個請求而回傳原本的措施；
 * 內容改過就換新的，否則伺服器會因為「同 id、內容不同」回 invalid_argument。對話框每次開啟都從沒有開始，所以重開是新的。
 */
export function requestIdFor(
	slot: RequestIdSlot | null,
	key: string,
	generate: () => string,
): RequestIdSlot {
	if (slot && isResend(slot, key)) return slot;
	return { key, id: generate() };
}

// ---------------------------------------------------------------------------
// 措施操作的失敗處理

export const MEASURE_CONFLICT_TEXT =
	"這個措施剛被更新，頁面已重新載入；請確認最新狀態後再操作。";

// ---------------------------------------------------------------------------
// 措施詳情：關聯帳號（W9）

/** 建立終止合作成功後的頁首提示；生效時間用伺服器回的 period.start_time（不自己推算）。 */
export function terminationCreatedText(
	measure: Pick<Measure, "period"> | undefined,
): string {
	const start = measure?.period?.startTime;
	return start
		? `已建立終止合作，生效時間：${formatDateTimeShort(timestampDate(start))}。`
		: "已建立終止合作。";
}
