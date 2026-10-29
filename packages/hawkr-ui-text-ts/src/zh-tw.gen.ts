// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。
import type { DefaultErrorText, UiMessage, UiTextKey } from "./keys.gen";
import { SELLER_MEASURE_REQUEST_KIND_NAME, commonError, commonUnknown } from "./keys.gen";
import type { MeasureRequestKind, MeasureRequestState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

type Values = Readonly<Record<string, string | number>>;
type Formatter = (v: Values) => string;

const DEFAULT: Readonly<Partial<Record<UiTextKey, Formatter>>> = {
	"buyer.Gender.GENDER_MALE": () => "男性",
	"buyer.Gender.GENDER_FEMALE": () => "女性",
	"buyer.Gender.GENDER_OTHER": () => "其他",
	"buyer.Profile.gender_unset": () => "尚未設定",
	"common.PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY": () => "食品安全",
	"common.PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS": () => "違法或禁售商品",
	"common.PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE": () => "未經許可販售",
	"common.PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO": () => "不實或誤導資訊",
	"common.PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION": () => "假認證",
	"common.PolicyCategory.POLICY_CATEGORY_COUNTERFEIT": () => "仿冒商品",
	"common.PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT": () => "侵害智慧財產權",
	"common.PolicyCategory.POLICY_CATEGORY_FRAUD": () => "詐欺或操縱交易",
	"common.PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE": () => "繞過平台交易",
	"common.PolicyCategory.POLICY_CATEGORY_PRIVACY": () => "不當使用個人資料",
	"common.PolicyCategory.POLICY_CATEGORY_IMPERSONATION": () => "冒用或誤導交易主體",
	"common.PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE": () => "干擾或濫用平台",
	"common.PolicyCategory.POLICY_CATEGORY_OTHER": () => "其他",
	"common.PolicyCategory.POLICY_CATEGORY_SPAM": () => "垃圾訊息",
	"common.PolicyCategory.POLICY_CATEGORY_HARASSMENT": () => "騷擾或霸凌",
	"common.PolicyCategory.POLICY_CATEGORY_HATE": () => "仇恨或歧視言論",
	"common.error.ABORTED": () => "資源衝突",
	"common.error.ALREADY_EXISTS": () => "資源衝突",
	"common.error.DATA_LOSS": () => "系統錯誤",
	"common.error.DEADLINE_EXCEEDED": () => "服務暫時不可用",
	"common.error.FAILED_PRECONDITION": () => "資源衝突",
	"common.error.INTERNAL": () => "系統錯誤",
	"common.error.INVALID_ARGUMENT": () => "參數錯誤",
	"common.error.NOT_FOUND": () => "資源不存在",
	"common.error.OUT_OF_RANGE": () => "參數錯誤",
	"common.error.PERMISSION_DENIED": () => "無權操作",
	"common.error.RESOURCE_EXHAUSTED": () => "操作過於頻繁，請稍後再試",
	"common.error.UNAUTHENTICATED": () => "未登入",
	"common.error.UNAVAILABLE": () => "服務暫時不可用",
	"common.error.UNIMPLEMENTED": () => "功能暫不支援",
	"common.error.UNKNOWN": () => "系統錯誤",
	"common.unknown": () => "未知",
	"seller.ContentKind.CONTENT_KIND_POST": () => "貼文",
	"seller.ContentKind.CONTENT_KIND_COMMENT": () => "留言",
	"seller.ContentKind.CONTENT_KIND_FORUM_ENTRY": () => "討論區留言",
	"seller.KYCField.KYC_FIELD_REAL_NAME": () => "真實姓名",
	"seller.KYCField.KYC_FIELD_BIRTH_DATE": () => "出生日期",
	"seller.KYCField.KYC_FIELD_ID_CARD_ISSUE_DATE": () => "發證日期",
	"seller.KYCField.KYC_FIELD_ID_CARD_ISSUE_TYPE": () => "發證類型",
	"seller.KYCField.KYC_FIELD_ID_CARD_ISSUE_LOCATION": () => "發證地點",
	"seller.KYCField.KYC_FIELD_ENGLISH_NAME": () => "英文姓名",
	"seller.KYCField.KYC_FIELD_BANK_NAME": () => "銀行名稱",
	"seller.KYCField.KYC_FIELD_BANK_CODE": () => "銀行代碼",
	"seller.KYCField.KYC_FIELD_BANK_ACCOUNT": () => "銀行帳號",
	"seller.KYCField.KYC_FIELD_PRODUCT_TYPE": () => "主要商品類型",
	"seller.KYCField.KYC_FIELD_WEBSITE_URL": () => "官方網站",
	"seller.KYCField.KYC_FIELD_ID_CARD_FRONT": () => "身分證正面照",
	"seller.KYCField.KYC_FIELD_ID_CARD_BACK": () => "身分證反面照",
	"seller.KYCField.KYC_FIELD_BANK_BOOK": () => "存摺封面",
	"seller.KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO": () => "身分證字號",
	"seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME": () => "公司名稱",
	"seller.KYCField.KYC_FIELD_BUSINESS_TAX_ID": () => "統一編號",
	"seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE": () => "設立日期",
	"seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE": () => "公司電話",
	"seller.KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO": () => "負責人身分證字號",
	"seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS": () => "補充文件",
	"seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS": () => "公司地址",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED": () => "待認證",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_DRAFT": () => "填寫中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED": () => "平台審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_REJECTED": () => "未通過",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_BINDING": () => "PayUni 審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED": () => "PayUni 建檔失敗，處理中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED": () => "PayUni 審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW": () => "PayUni 審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO": () => "需補件",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_APPROVED": () => "已通過",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED": () => "PayUni 審核未通過",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_NOT_STARTED": () => "開始認證",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_DRAFT": () => "開始認證",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_SUBMITTED": () => "查看進度",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_REJECTED": () => "前往補件",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_BINDING": () => "查看進度",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_BIND_FAILED": () => "查看進度",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_FORWARDED": () => "查看進度",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_UNDER_REVIEW": () => "查看進度",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_PENDING_INFO": () => "前往補件",
	"seller.KYCFlowStatus.action.KYC_FLOW_STATUS_PAYUNI_REJECTED": () => "前往補件",
	"seller.KYCFlowStatus.banner_description.KYC_FLOW_STATUS_NOT_STARTED": () => "完成認證後才能正式收款、提領與配送設定。",
	"seller.KYCFlowStatus.banner_description.KYC_FLOW_STATUS_DRAFT": () => "完成認證後才能正式收款、提領與配送設定。",
	"seller.KYCFlowStatus.banner_description.KYC_FLOW_STATUS_SUBMITTED": () => "審核通過後將自動送出至 PayUni 進行金流審核。",
	"seller.KYCFlowStatus.banner_description.KYC_FLOW_STATUS_REJECTED": () => "請查看退件原因並編輯後重新送出。",
	"seller.KYCFlowStatus.banner_description.KYC_FLOW_STATUS_APPROVED": () => "可開始正式收款、提領與配送設定。",
	"seller.KYCFlowStatus.banner_title.KYC_FLOW_STATUS_NOT_STARTED": () => "尚未開始商家認證",
	"seller.KYCFlowStatus.banner_title.KYC_FLOW_STATUS_DRAFT": () => "尚未開始商家認證",
	"seller.KYCFlowStatus.banner_title.KYC_FLOW_STATUS_SUBMITTED": () => "平台審核中",
	"seller.KYCFlowStatus.banner_title.KYC_FLOW_STATUS_REJECTED": () => "認證未通過",
	"seller.KYCFlowStatus.banner_title.KYC_FLOW_STATUS_APPROVED": () => "商家認證已通過",
	"seller.KYCFlowStatus.gate_heading.KYC_FLOW_STATUS_NOT_STARTED": () => "尚未通過商家認證",
	"seller.KYCFlowStatus.gate_heading.KYC_FLOW_STATUS_DRAFT": () => "尚未通過商家認證",
	"seller.KYCFlowStatus.gate_heading.KYC_FLOW_STATUS_SUBMITTED": () => "商家認證審核中",
	"seller.KYCFlowStatus.gate_heading.KYC_FLOW_STATUS_REJECTED": () => "商家認證需要補件",
	"seller.KYCFlowStatus.gate_heading.KYC_FLOW_STATUS_APPROVED": () => "尚未通過商家認證",
	"seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_NOT_STARTED": (v) => "請先完成商家認證，才能" + String(v.subject) + "。",
	"seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_DRAFT": (v) => "請先完成商家認證，才能" + String(v.subject) + "。",
	"seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_SUBMITTED": (v) => "你的商家認證平台審核中，通過後將自動送出至 PayUni 進行金流審核，完成後就能" + String(v.subject) + "。",
	"seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_REJECTED": () => "認證未通過，請修改資料後重新送審。",
	"seller.KYCFlowStatus.gate_message.KYC_FLOW_STATUS_APPROVED": (v) => "請先完成商家認證，才能" + String(v.subject) + "。",
	"seller.Measure.appeal_deadline": (v) => "申覆期限：" + String(v.deadline),
	"seller.Measure.meta_kind": (v) => "措施：" + String(v.kind),
	"seller.Measure.meta_severity": (v) => "嚴重程度：" + String(v.severity),
	"seller.Measure.meta_termination_ground": (v) => "事由：" + String(v.ground),
	"seller.Measure.rejection_cooldown_warning": (v) => "若本次經駁回，將於 " + String(v.days) + " 日內不得再提出解除請求",
	"seller.Measure.suspension_active_open": () => "停權中（未定結束時間，平台定期複審）",
	"seller.Measure.suspension_active_until": (v) => "停權中（至 " + String(v.end) + "）",
	"seller.Measure.suspension_ended": (v) => "已於 " + String(v.end) + " 結束",
	"seller.Measure.termination_effective": (v) => "終止生效：" + String(v.start),
	"seller.Measure.termination_effective_ended": (v) => "終止生效：" + String(v.start) + "，合作已於 " + String(v.end) + " 結束",
	"seller.Measure.termination_notice": (v) => "平台已通知將於 " + String(v.start) + " 終止合作。在此之前店家照常營業；如有異議，可提出申覆。",
	"seller.Measure.title_fallback": () => "措施紀錄",
	"seller.Measure.title_risk": (v) => "風險：" + String(v.ground),
	"seller.Measure.title_risk_fallback": () => "風險措施",
	"seller.MeasureBasis.MEASURE_BASIS_VIOLATION": () => "違規認定",
	"seller.MeasureBasis.MEASURE_BASIS_RISK": () => "風險",
	"seller.MeasureBasis.MEASURE_BASIS_CONTRACT": () => "契約條款",
	"seller.MeasureKind.MEASURE_KIND_WARNING": () => "警告",
	"seller.MeasureKind.MEASURE_KIND_SUSPENSION": () => "停權",
	"seller.MeasureKind.MEASURE_KIND_TERMINATION": () => "終止合作",
	"seller.MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN": () => "內容下架",
	"seller.MeasureRequest.reply_note": (v) => "平台回覆：" + String(v.reason),
	"seller.MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL": () => "申覆",
	"seller.MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION": () => "解除請求",
	"seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_APPEAL": () => "已提交申覆，請等候平台複核。",
	"seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_REMEDIATION": () => "已提交解除請求，請等候平台複核。",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING": () => "審核中",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED": () => "已通過",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED": () => "已駁回",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED": () => "已結案",
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_PENDING": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆審核中";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求審核中";
default:
return "請求審核中";
}
})(),
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_ACCEPTED": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆已通過";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求已通過";
default:
return "請求已通過";
}
})(),
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_REJECTED": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆已駁回";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求已駁回";
default:
return "請求已駁回";
}
})(),
	"seller.MeasureState.MEASURE_STATE_ISSUED": () => "已發出",
	"seller.MeasureState.MEASURE_STATE_ACTIVE": (v) => (() => {
switch (v.kind) {
case "MEASURE_KIND_TERMINATION":
return "合作已終止";
case "MEASURE_KIND_CONTENT_TAKEDOWN":
return "內容已下架";
default:
return "停權中";
}
})(),
	"seller.MeasureState.MEASURE_STATE_ENDED": () => "已結束",
	"seller.MeasureState.MEASURE_STATE_REVOKED": () => "已撤銷",
	"seller.MeasureState.MEASURE_STATE_SCHEDULED": () => "預定生效",
	"seller.PayUniState.PAY_UNI_STATE_REVIEWING": () => "PayUni 審核中",
	"seller.PayUniState.PAY_UNI_STATE_PENDING_INFO": () => "PayUni 需要補件",
	"seller.PayUniState.PAY_UNI_STATE_FIXABLE": () => "PayUni 需要補件",
	"seller.PayUniState.PAY_UNI_STATE_FIX_VIA_SUPPORT": () => "PayUni 需要補件",
	"seller.PayUniState.PAY_UNI_STATE_CONTACT_SUPPORT": () => "PayUni 申請未通過",
	"seller.PayUniState.PAY_UNI_STATE_RETRY_LATER": () => "PayUni 暫時無法處理",
	"seller.PayUniState.PAY_UNI_STATE_TERMINATED": () => "PayUni 收款帳戶已終止",
	"seller.PayUniState.PAY_UNI_STATE_APPROVED": () => "PayUni 收款已啟用",
	"seller.RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED": () => "申覆成立",
	"seller.RevokeReason.REVOKE_REASON_ADMIN_CORRECTION": () => "平台更正",
	"seller.RevokeReason.REVOKE_REASON_EXTERNAL_DECISION": () => "主管機關或法院的決定",
	"seller.RevokeReason.REVOKE_REASON_OTHER": () => "其他",
	"seller.RevokeReason.REVOKE_REASON_FINDING_REVOKED": () => "依據的認定已撤銷",
	"seller.RevokeReason.REVOKE_REASON_SOURCE_REVOKED": () => "關聯的終止已撤銷",
	"seller.RiskGround.RISK_GROUND_TRANSACTION": () => "交易風險",
	"seller.RiskGround.RISK_GROUND_FOOD_SAFETY": () => "食品安全",
	"seller.RiskGround.RISK_GROUND_FRAUD": () => "詐欺",
	"seller.RiskGround.RISK_GROUND_PERSONAL_DATA": () => "個人資料",
	"seller.RiskGround.RISK_GROUND_INFRINGEMENT": () => "侵權",
	"seller.RiskGround.RISK_GROUND_SECURITY": () => "資安",
	"seller.RiskGround.RISK_GROUND_FULFILLMENT": () => "履約",
	"seller.RiskGround.RISK_GROUND_EXTERNAL_ORDER": () => "主管機關或司法機關的要求",
	"seller.SellerPayUni.fix_fields_heading": () => "請修正以下資料",
	"seller.SellerPayUni.reason_heading": () => "PayUni 的審核意見",
	"seller.SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE": () => "店家未關閉",
	"seller.SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED": () => "帳號未停權",
	"seller.SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED": () => "商家認證已通過",
	"seller.SellerRequirement.SELLER_REQUIREMENT_STORE_NAME": () => "店名",
	"seller.SellerRequirement.SELLER_REQUIREMENT_LOGO": () => "logo",
	"seller.SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE": () => "所在地",
	"seller.SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION": () => "店家未在刪除中",
	"seller.StoreField.STORE_FIELD_ADDRESS": () => "店家地址",
	"seller.StoreField.STORE_FIELD_CONTACT_NAME": () => "聯絡人姓名",
	"seller.StoreField.STORE_FIELD_CONTACT_MOBILE": () => "聯絡人手機",
	"seller.StoreField.STORE_FIELD_SUPPORT_EMAIL": () => "客服電子信箱",
	"seller.StoreField.STORE_FIELD_CONTACT_PHONE": () => "商店電話",
	"seller.TerminationGround.TERMINATION_GROUND_NOTICE": () => "預告終止（第 21.2 條）",
	"seller.TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH": () => "重大違約未改善",
	"seller.TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS": () => "停業",
	"seller.TerminationGround.TERMINATION_GROUND_DISSOLUTION": () => "解散",
	"seller.TerminationGround.TERMINATION_GROUND_BANKRUPTCY": () => "破產",
	"seller.TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED": () => "必要許可遭撤銷",
	"seller.TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT": () => "重大食安事件",
	"seller.TerminationGround.TERMINATION_GROUND_FRAUD": () => "詐欺事件",
	"seller.TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY": () => "危害平台安全",
	"seller.TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS": () => "其他嚴重影響履約的情形",
	"seller.TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT": () => "關聯帳號",
	"seller.ViolationSeverity.VIOLATION_SEVERITY_LOW": () => "低",
	"seller.ViolationSeverity.VIOLATION_SEVERITY_MEDIUM": () => "中",
	"seller.ViolationSeverity.VIOLATION_SEVERITY_HIGH": () => "高",
	"seller.ViolationSeverity.VIOLATION_SEVERITY_CRITICAL": () => "嚴重",
};

const STAFF: Readonly<Partial<Record<UiTextKey, Formatter>>> = {
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED": () => "尚未申請",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_DRAFT": () => "填寫中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED": () => "待審核",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_REJECTED": () => "已退回",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_BINDING": () => "PayUni 綁定中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED": () => "待重新綁定",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED": () => "PayUni 審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW": () => "PayUni 審核中",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO": () => "PayUni 待補件",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_APPROVED": () => "已核准",
	"seller.KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED": () => "PayUni 審核未通過",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING": () => "待裁定",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED": () => "成立",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED": () => "駁回",
	"seller.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED": () => "已結案",
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_PENDING": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆待裁定";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求待裁定";
default:
return "請求待裁定";
}
})(),
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_ACCEPTED": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆成立";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求成立";
default:
return "請求成立";
}
})(),
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_REJECTED": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆駁回";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求駁回";
default:
return "請求駁回";
}
})(),
	"seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_CLOSED": (v) => (() => {
switch (v.kind) {
case "MEASURE_REQUEST_KIND_APPEAL":
return "申覆已結案";
case "MEASURE_REQUEST_KIND_REMEDIATION":
return "解除請求已結案";
default:
return "請求已結案";
}
})(),
};

/**
 * 後台員工受眾的格式化（admin RPC 固定 zh-TW，ADR D1）：staff 有的 key 用 staff，沒有的用預設受眾；
 * 名詞參數也取 staff 優先。DEFAULT 只放預設受眾有文字的 key，staff 獨有的 key 在 STAFF。
 */
export function formatStaff(m: UiMessage): string {
	const values: Record<string, string | number> = {};
	for (const [name, value] of Object.entries(m.values ?? {})) {
		values[name] = typeof value === "object" ? formatStaff(value) : value;
	}
	const format = STAFF[m.key] ?? DEFAULT[m.key];
	if (format === undefined) {
		throw new Error("uitext: no zh-TW text for " + m.key);
	}
	return format(values);
}

/** Connect code 的預設文字（zh-TW）；Canceled 由呼叫端自己處理。 */
export const defaultErrorTextZhTW: DefaultErrorText = (code) =>
	formatStaff(commonError(code) ?? commonUnknown());

const SELLER_MEASURE_REQUEST_STATE_WITH_KIND_STAFF: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_PENDING",
	2: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_ACCEPTED",
	3: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_REJECTED",
	4: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_CLOSED",
};

/** seller.MeasureRequestState.with_kind；staff 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerMeasureRequestStateWithKindStaff(v: MeasureRequestState, a: { kind: MeasureRequestKind }): UiMessage | undefined {
	const key = SELLER_MEASURE_REQUEST_STATE_WITH_KIND_STAFF[v];
	if (key === undefined) return undefined;
	return { key, values: { kind: SELLER_MEASURE_REQUEST_KIND_NAME[a.kind] ?? "" } };
}

