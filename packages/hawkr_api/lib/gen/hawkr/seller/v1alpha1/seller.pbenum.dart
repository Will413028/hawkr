// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/seller.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// AgreementKind 是賣家入駐時要同意或確認已閱讀的文件種類。每個操作要求的種類固定：ApplyStore 要 SELLER_TERMS；SubmitKYC 要
/// KYC_PRIVACY_NOTICE 與 PAYUNI_TERMS；SubmitFoodRegistration 要 FOOD_REGISTRATION_NOTICE。
class AgreementKind extends $pb.ProtobufEnum {
  static const AgreementKind AGREEMENT_KIND_UNSPECIFIED =
      AgreementKind._(0, _omitEnumNames ? '' : 'AGREEMENT_KIND_UNSPECIFIED');

  /// 賣家契約，開店（ApplyStore）時同意。
  static const AgreementKind AGREEMENT_KIND_SELLER_TERMS =
      AgreementKind._(1, _omitEnumNames ? '' : 'AGREEMENT_KIND_SELLER_TERMS');

  /// 身分認證的個資蒐集告知，送出 KYC（SubmitKYC）時確認已閱讀（告知不是同意）。
  static const AgreementKind AGREEMENT_KIND_KYC_PRIVACY_NOTICE =
      AgreementKind._(
          2, _omitEnumNames ? '' : 'AGREEMENT_KIND_KYC_PRIVACY_NOTICE');

  /// PayUni 的服務條款，送出 KYC（SubmitKYC）時同意。
  static const AgreementKind AGREEMENT_KIND_PAYUNI_TERMS =
      AgreementKind._(3, _omitEnumNames ? '' : 'AGREEMENT_KIND_PAYUNI_TERMS');

  /// 食品業者登錄的個資蒐集告知，送出食品業者登錄（SubmitFoodRegistration）時確認已閱讀（告知不是同意）。
  static const AgreementKind AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE =
      AgreementKind._(
          4, _omitEnumNames ? '' : 'AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE');

  static const $core.List<AgreementKind> values = <AgreementKind>[
    AGREEMENT_KIND_UNSPECIFIED,
    AGREEMENT_KIND_SELLER_TERMS,
    AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
    AGREEMENT_KIND_PAYUNI_TERMS,
    AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
  ];

  static final $core.List<AgreementKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static AgreementKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AgreementKind._(super.value, super.name);
}

/// StoreStatus 是店家的生命週期。停權是另一個維度（GetMyStoreResponse.suspended）。
class StoreStatus extends $pb.ProtobufEnum {
  static const StoreStatus STORE_STATUS_UNSPECIFIED =
      StoreStatus._(0, _omitEnumNames ? '' : 'STORE_STATUS_UNSPECIFIED');

  /// 營業中。
  static const StoreStatus STORE_STATUS_ACTIVE =
      StoreStatus._(1, _omitEnumNames ? '' : 'STORE_STATUS_ACTIVE');

  /// 隨帳號刪除，等待未結訂單結束；不能做經營類的事。
  static const StoreStatus STORE_STATUS_PENDING_DELETION =
      StoreStatus._(2, _omitEnumNames ? '' : 'STORE_STATUS_PENDING_DELETION');

  /// 已關閉；只出現在 CloseMyStore 的回應。
  static const StoreStatus STORE_STATUS_CLOSED =
      StoreStatus._(3, _omitEnumNames ? '' : 'STORE_STATUS_CLOSED');

  static const $core.List<StoreStatus> values = <StoreStatus>[
    STORE_STATUS_UNSPECIFIED,
    STORE_STATUS_ACTIVE,
    STORE_STATUS_PENDING_DELETION,
    STORE_STATUS_CLOSED,
  ];

  static final $core.List<StoreStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static StoreStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoreStatus._(super.value, super.name);
}

/// SellerCapability 是賣家能做的一類事，也是 GetMyStore 回報狀態的單位（CapabilityStatus）。能力由店家的生命週期、
/// 停權與前置條件（例如身分認證）一起決定：停權時只剩白名單，等待刪除時沒有經營類與關店，關閉時沒有能力，
/// 前置條件沒有滿足的能力不啟用；沒有啟用的能力連同缺的條件（SellerRequirement）一起回報。client 以它決定畫面上哪些
/// 操作可用、閘門顯示什麼，伺服器仍會逐方法檢查。
class SellerCapability extends $pb.ProtobufEnum {
  static const SellerCapability SELLER_CAPABILITY_UNSPECIFIED =
      SellerCapability._(
          0, _omitEnumNames ? '' : 'SELLER_CAPABILITY_UNSPECIFIED');

  /// 看自己的店家資料與停權狀態。
  static const SellerCapability SELLER_CAPABILITY_ACCOUNT_SELF_READ =
      SellerCapability._(
          1, _omitEnumNames ? '' : 'SELLER_CAPABILITY_ACCOUNT_SELF_READ');

  /// 店面設定：資料、logo、封面、新手引導。
  static const SellerCapability SELLER_CAPABILITY_STOREFRONT_WRITE =
      SellerCapability._(
          2, _omitEnumNames ? '' : 'SELLER_CAPABILITY_STOREFRONT_WRITE');

  /// 商品的建立、編輯、上下架、價格。
  static const SellerCapability SELLER_CAPABILITY_PRODUCT_MANAGE =
      SellerCapability._(
          3, _omitEnumNames ? '' : 'SELLER_CAPABILITY_PRODUCT_MANAGE');

  /// 出貨、售後、訂單查詢、PayUni 狀態。
  static const SellerCapability SELLER_CAPABILITY_ORDER_FULFILL =
      SellerCapability._(
          4, _omitEnumNames ? '' : 'SELLER_CAPABILITY_ORDER_FULFILL');

  /// 收縮型的訂單動作（取消出貨與退款受理）。
  static const SellerCapability SELLER_CAPABILITY_ORDER_CONTRACT =
      SellerCapability._(
          5, _omitEnumNames ? '' : 'SELLER_CAPABILITY_ORDER_CONTRACT');

  /// 行銷經營：優惠券、CRM、貼文發佈、AI 助理、dashboard 與報表。
  static const SellerCapability SELLER_CAPABILITY_MARKETING =
      SellerCapability._(
          6, _omitEnumNames ? '' : 'SELLER_CAPABILITY_MARKETING');

  /// 收縮型的內容動作（刪除、下架自己的貼文，撤回自己的券）。
  static const SellerCapability SELLER_CAPABILITY_CONTENT_CONTRACT =
      SellerCapability._(
          7, _omitEnumNames ? '' : 'SELLER_CAPABILITY_CONTENT_CONTRACT');

  /// 送出訊息。
  static const SellerCapability SELLER_CAPABILITY_CHAT_SEND =
      SellerCapability._(
          9, _omitEnumNames ? '' : 'SELLER_CAPABILITY_CHAT_SEND');

  /// 讀取訊息。
  static const SellerCapability SELLER_CAPABILITY_CHAT_READ =
      SellerCapability._(
          10, _omitEnumNames ? '' : 'SELLER_CAPABILITY_CHAT_READ');

  /// 通知收件箱、偏好與裝置。
  static const SellerCapability SELLER_CAPABILITY_NOTIFICATION =
      SellerCapability._(
          11, _omitEnumNames ? '' : 'SELLER_CAPABILITY_NOTIFICATION');

  /// 關店。
  static const SellerCapability SELLER_CAPABILITY_ACCOUNT_CLOSE =
      SellerCapability._(
          12, _omitEnumNames ? '' : 'SELLER_CAPABILITY_ACCOUNT_CLOSE');

  /// 發票與稅務憑證的查詢與設定。
  static const SellerCapability SELLER_CAPABILITY_INVOICE_MANAGE =
      SellerCapability._(
          13, _omitEnumNames ? '' : 'SELLER_CAPABILITY_INVOICE_MANAGE');

  /// 稅務身分的申請、補件與延長。
  static const SellerCapability SELLER_CAPABILITY_INVOICE_TAX_REMEDY =
      SellerCapability._(
          14, _omitEnumNames ? '' : 'SELLER_CAPABILITY_INVOICE_TAX_REMEDY');

  /// 物流建店、啟用與關閉配送方式。前置條件是商家認證（含 PayUni 審核）核准：物流商以 PayUni 已成立的子商店建店。
  static const SellerCapability SELLER_CAPABILITY_SHIPPING_SETUP =
      SellerCapability._(
          15, _omitEnumNames ? '' : 'SELLER_CAPABILITY_SHIPPING_SETUP');

  /// 店家的狀態：買家看得到店面、可以結帳。缺的條件是公開曝光的全部條件（SellerRequirement）。這是狀態而不是
  /// 賣家能做的動作，只出現在 GetMyStore 的 capabilities，沒有任何方法需要它；「至少一個上架商品」由商品的曝光端檢查，不在其中。
  static const SellerCapability SELLER_CAPABILITY_PUBLIC_LISTING =
      SellerCapability._(
          16, _omitEnumNames ? '' : 'SELLER_CAPABILITY_PUBLIC_LISTING');

  static const $core.List<SellerCapability> values = <SellerCapability>[
    SELLER_CAPABILITY_UNSPECIFIED,
    SELLER_CAPABILITY_ACCOUNT_SELF_READ,
    SELLER_CAPABILITY_STOREFRONT_WRITE,
    SELLER_CAPABILITY_PRODUCT_MANAGE,
    SELLER_CAPABILITY_ORDER_FULFILL,
    SELLER_CAPABILITY_ORDER_CONTRACT,
    SELLER_CAPABILITY_MARKETING,
    SELLER_CAPABILITY_CONTENT_CONTRACT,
    SELLER_CAPABILITY_CHAT_SEND,
    SELLER_CAPABILITY_CHAT_READ,
    SELLER_CAPABILITY_NOTIFICATION,
    SELLER_CAPABILITY_ACCOUNT_CLOSE,
    SELLER_CAPABILITY_INVOICE_MANAGE,
    SELLER_CAPABILITY_INVOICE_TAX_REMEDY,
    SELLER_CAPABILITY_SHIPPING_SETUP,
    SELLER_CAPABILITY_PUBLIC_LISTING,
  ];

  static final $core.List<SellerCapability?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 16);
  static SellerCapability? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SellerCapability._(super.value, super.name);
}

/// SellerRequirement 是能力啟用或店家公開的一項條件。CapabilityStatus.missing 依這個數字順序列出還沒滿足的條件。
class SellerRequirement extends $pb.ProtobufEnum {
  static const SellerRequirement SELLER_REQUIREMENT_UNSPECIFIED =
      SellerRequirement._(
          0, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_UNSPECIFIED');

  /// 店家沒有關閉。
  static const SellerRequirement SELLER_REQUIREMENT_STORE_ACTIVE =
      SellerRequirement._(
          1, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_STORE_ACTIVE');

  /// 帳號沒有生效中的停權。
  static const SellerRequirement SELLER_REQUIREMENT_NOT_SUSPENDED =
      SellerRequirement._(
          2, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_NOT_SUSPENDED');

  /// 商家認證（含 PayUni 審核）已核准。
  static const SellerRequirement SELLER_REQUIREMENT_KYC_APPROVED =
      SellerRequirement._(
          3, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_KYC_APPROVED');

  /// 有店名。
  static const SellerRequirement SELLER_REQUIREMENT_STORE_NAME =
      SellerRequirement._(
          4, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_STORE_NAME');

  /// 有 logo。
  static const SellerRequirement SELLER_REQUIREMENT_LOGO =
      SellerRequirement._(5, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_LOGO');

  /// 有鄉鎮市區。
  static const SellerRequirement SELLER_REQUIREMENT_TOWN_CODE =
      SellerRequirement._(
          6, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_TOWN_CODE');

  /// 店家沒有正在隨帳號刪除。
  static const SellerRequirement SELLER_REQUIREMENT_NOT_PENDING_DELETION =
      SellerRequirement._(
          7, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_NOT_PENDING_DELETION');

  static const $core.List<SellerRequirement> values = <SellerRequirement>[
    SELLER_REQUIREMENT_UNSPECIFIED,
    SELLER_REQUIREMENT_STORE_ACTIVE,
    SELLER_REQUIREMENT_NOT_SUSPENDED,
    SELLER_REQUIREMENT_KYC_APPROVED,
    SELLER_REQUIREMENT_STORE_NAME,
    SELLER_REQUIREMENT_LOGO,
    SELLER_REQUIREMENT_TOWN_CODE,
    SELLER_REQUIREMENT_NOT_PENDING_DELETION,
  ];

  static final $core.List<SellerRequirement?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static SellerRequirement? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SellerRequirement._(super.value, super.name);
}

/// StoreField 是店家上的一個欄位，值與 Store 的欄位一對一。送出 KYC 之前還缺的店家欄位
/// （KYCApplication.missing_store_fields）與 PayUni 問題要賣家補的店家欄位（PayUniProblem.store_fields）都用這一套。
class StoreField extends $pb.ProtobufEnum {
  static const StoreField STORE_FIELD_UNSPECIFIED =
      StoreField._(0, _omitEnumNames ? '' : 'STORE_FIELD_UNSPECIFIED');

  /// 店家地址（鄉鎮市區與地址明細）。
  static const StoreField STORE_FIELD_ADDRESS =
      StoreField._(1, _omitEnumNames ? '' : 'STORE_FIELD_ADDRESS');
  static const StoreField STORE_FIELD_CONTACT_NAME =
      StoreField._(2, _omitEnumNames ? '' : 'STORE_FIELD_CONTACT_NAME');
  static const StoreField STORE_FIELD_CONTACT_MOBILE =
      StoreField._(3, _omitEnumNames ? '' : 'STORE_FIELD_CONTACT_MOBILE');
  static const StoreField STORE_FIELD_SUPPORT_EMAIL =
      StoreField._(4, _omitEnumNames ? '' : 'STORE_FIELD_SUPPORT_EMAIL');

  /// 商店電話。
  static const StoreField STORE_FIELD_CONTACT_PHONE =
      StoreField._(5, _omitEnumNames ? '' : 'STORE_FIELD_CONTACT_PHONE');

  static const $core.List<StoreField> values = <StoreField>[
    STORE_FIELD_UNSPECIFIED,
    STORE_FIELD_ADDRESS,
    STORE_FIELD_CONTACT_NAME,
    STORE_FIELD_CONTACT_MOBILE,
    STORE_FIELD_SUPPORT_EMAIL,
    STORE_FIELD_CONTACT_PHONE,
  ];

  static final $core.List<StoreField?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static StoreField? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoreField._(super.value, super.name);
}

/// FoodRegistrationState 是賣家看得到的登錄狀態：撤回與被新的取代的登錄賣家看不到，核准後因賣家清除或被新核准的取代而結束的也不顯示，
/// 所以只有被平台撤銷的才會以 ENDED 出現（Store.revoked_food_registration）。
class FoodRegistrationState extends $pb.ProtobufEnum {
  static const FoodRegistrationState FOOD_REGISTRATION_STATE_UNSPECIFIED =
      FoodRegistrationState._(
          0, _omitEnumNames ? '' : 'FOOD_REGISTRATION_STATE_UNSPECIFIED');

  /// 等平台核對。
  static const FoodRegistrationState FOOD_REGISTRATION_STATE_PENDING =
      FoodRegistrationState._(
          1, _omitEnumNames ? '' : 'FOOD_REGISTRATION_STATE_PENDING');

  /// 核准中：店面與商品頁公開的字號只讀這一筆。
  static const FoodRegistrationState FOOD_REGISTRATION_STATE_APPROVED =
      FoodRegistrationState._(
          2, _omitEnumNames ? '' : 'FOOD_REGISTRATION_STATE_APPROVED');

  /// 平台退回；賣家確認字號與食藥署的登錄資料一致後可以重新送出。
  static const FoodRegistrationState FOOD_REGISTRATION_STATE_REJECTED =
      FoodRegistrationState._(
          3, _omitEnumNames ? '' : 'FOOD_REGISTRATION_STATE_REJECTED');

  /// 核准後結束；賣家看得到的只有被平台撤銷的（end_time 與 end_reason 有值）。
  static const FoodRegistrationState FOOD_REGISTRATION_STATE_ENDED =
      FoodRegistrationState._(
          4, _omitEnumNames ? '' : 'FOOD_REGISTRATION_STATE_ENDED');

  static const $core.List<FoodRegistrationState> values =
      <FoodRegistrationState>[
    FOOD_REGISTRATION_STATE_UNSPECIFIED,
    FOOD_REGISTRATION_STATE_PENDING,
    FOOD_REGISTRATION_STATE_APPROVED,
    FOOD_REGISTRATION_STATE_REJECTED,
    FOOD_REGISTRATION_STATE_ENDED,
  ];

  static final $core.List<FoodRegistrationState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static FoodRegistrationState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FoodRegistrationState._(super.value, super.name);
}

/// StoreOpeningUnavailableReason 是帳號此刻不能申請開店的原因（StoreOpeningUnavailability.reason）。client 用它分支，不用它組顯示文字。
class StoreOpeningUnavailableReason extends $pb.ProtobufEnum {
  static const StoreOpeningUnavailableReason
      STORE_OPENING_UNAVAILABLE_REASON_UNSPECIFIED =
      StoreOpeningUnavailableReason._(0,
          _omitEnumNames ? '' : 'STORE_OPENING_UNAVAILABLE_REASON_UNSPECIFIED');

  /// 帳號有生效中的終止合作，終止撤銷之前不能再開店。client 導向措施與申覆。對應 ErrorReason.STORE_OPENING_TERMINATED。
  static const StoreOpeningUnavailableReason
      STORE_OPENING_UNAVAILABLE_REASON_TERMINATED =
      StoreOpeningUnavailableReason._(1,
          _omitEnumNames ? '' : 'STORE_OPENING_UNAVAILABLE_REASON_TERMINATED');

  /// 帳號有生效中的停權。client 導向措施與申覆。對應 ErrorReason.STORE_OPENING_SUSPENDED。
  static const StoreOpeningUnavailableReason
      STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED =
      StoreOpeningUnavailableReason._(2,
          _omitEnumNames ? '' : 'STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED');

  /// 帳號已受理刪除申請、等待未結訂單結束。
  static const StoreOpeningUnavailableReason
      STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING =
      StoreOpeningUnavailableReason._(
          3,
          _omitEnumNames
              ? ''
              : 'STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING');

  /// 帳號已經有目前的店家；client 走更新店家的流程，不呼叫 ApplyStore。
  static const StoreOpeningUnavailableReason
      STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS =
      StoreOpeningUnavailableReason._(
          4,
          _omitEnumNames
              ? ''
              : 'STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS');

  static const $core.List<StoreOpeningUnavailableReason> values =
      <StoreOpeningUnavailableReason>[
    STORE_OPENING_UNAVAILABLE_REASON_UNSPECIFIED,
    STORE_OPENING_UNAVAILABLE_REASON_TERMINATED,
    STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED,
    STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING,
    STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS,
  ];

  static final $core.List<StoreOpeningUnavailableReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static StoreOpeningUnavailableReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoreOpeningUnavailableReason._(super.value, super.name);
}

/// KYCStatus 是申請的人工決定；PayUni 的綁定結果不在這條軸上（見 KYCFlow）。
class KYCStatus extends $pb.ProtobufEnum {
  static const KYCStatus KYC_STATUS_UNSPECIFIED =
      KYCStatus._(0, _omitEnumNames ? '' : 'KYC_STATUS_UNSPECIFIED');

  /// 填寫中；被退回的申請編輯時也回到這裡。
  static const KYCStatus KYC_STATUS_DRAFT =
      KYCStatus._(1, _omitEnumNames ? '' : 'KYC_STATUS_DRAFT');

  /// 已送出，等待管理員審核。
  static const KYCStatus KYC_STATUS_SUBMITTED =
      KYCStatus._(2, _omitEnumNames ? '' : 'KYC_STATUS_SUBMITTED');

  /// 管理員核准。
  static const KYCStatus KYC_STATUS_APPROVED =
      KYCStatus._(3, _omitEnumNames ? '' : 'KYC_STATUS_APPROVED');

  /// 管理員退回，可以修改後重送。
  static const KYCStatus KYC_STATUS_REJECTED =
      KYCStatus._(4, _omitEnumNames ? '' : 'KYC_STATUS_REJECTED');

  /// 賣家取消；只出現在 CancelKYC 的回應。
  static const KYCStatus KYC_STATUS_CANCELLED =
      KYCStatus._(5, _omitEnumNames ? '' : 'KYC_STATUS_CANCELLED');

  static const $core.List<KYCStatus> values = <KYCStatus>[
    KYC_STATUS_UNSPECIFIED,
    KYC_STATUS_DRAFT,
    KYC_STATUS_SUBMITTED,
    KYC_STATUS_APPROVED,
    KYC_STATUS_REJECTED,
    KYC_STATUS_CANCELLED,
  ];

  static final $core.List<KYCStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static KYCStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const KYCStatus._(super.value, super.name);
}

/// KYCField 是申請上一個欄位，值與 KYCApplication 的欄位路徑一對一（名稱同樣是 snake_case，巢狀路徑的 . 換成 _：
/// KYC_FIELD_BUSINESS_TAX_ID 是 business.tax_id）。退回時指出有問題的欄位（KYCIssue.field）、PayUni 問題要補的
/// 欄位（PayUniProblem.fields）、還缺的欄位（KYCApplication.missing_fields）與送出時 BadRequest 的 field 都用這一套。
class KYCField extends $pb.ProtobufEnum {
  static const KYCField KYC_FIELD_UNSPECIFIED =
      KYCField._(0, _omitEnumNames ? '' : 'KYC_FIELD_UNSPECIFIED');
  static const KYCField KYC_FIELD_REAL_NAME =
      KYCField._(1, _omitEnumNames ? '' : 'KYC_FIELD_REAL_NAME');
  static const KYCField KYC_FIELD_BIRTH_DATE =
      KYCField._(2, _omitEnumNames ? '' : 'KYC_FIELD_BIRTH_DATE');
  static const KYCField KYC_FIELD_ID_CARD_ISSUE_DATE =
      KYCField._(3, _omitEnumNames ? '' : 'KYC_FIELD_ID_CARD_ISSUE_DATE');
  static const KYCField KYC_FIELD_ID_CARD_ISSUE_TYPE =
      KYCField._(4, _omitEnumNames ? '' : 'KYC_FIELD_ID_CARD_ISSUE_TYPE');
  static const KYCField KYC_FIELD_ID_CARD_ISSUE_LOCATION =
      KYCField._(5, _omitEnumNames ? '' : 'KYC_FIELD_ID_CARD_ISSUE_LOCATION');
  static const KYCField KYC_FIELD_ENGLISH_NAME =
      KYCField._(6, _omitEnumNames ? '' : 'KYC_FIELD_ENGLISH_NAME');
  static const KYCField KYC_FIELD_BANK_NAME =
      KYCField._(7, _omitEnumNames ? '' : 'KYC_FIELD_BANK_NAME');
  static const KYCField KYC_FIELD_BANK_CODE =
      KYCField._(8, _omitEnumNames ? '' : 'KYC_FIELD_BANK_CODE');
  static const KYCField KYC_FIELD_BANK_ACCOUNT =
      KYCField._(9, _omitEnumNames ? '' : 'KYC_FIELD_BANK_ACCOUNT');
  static const KYCField KYC_FIELD_PRODUCT_TYPE =
      KYCField._(10, _omitEnumNames ? '' : 'KYC_FIELD_PRODUCT_TYPE');
  static const KYCField KYC_FIELD_WEBSITE_URL =
      KYCField._(12, _omitEnumNames ? '' : 'KYC_FIELD_WEBSITE_URL');
  static const KYCField KYC_FIELD_ID_CARD_FRONT =
      KYCField._(15, _omitEnumNames ? '' : 'KYC_FIELD_ID_CARD_FRONT');
  static const KYCField KYC_FIELD_ID_CARD_BACK =
      KYCField._(16, _omitEnumNames ? '' : 'KYC_FIELD_ID_CARD_BACK');
  static const KYCField KYC_FIELD_BANK_BOOK =
      KYCField._(17, _omitEnumNames ? '' : 'KYC_FIELD_BANK_BOOK');
  static const KYCField KYC_FIELD_INDIVIDUAL_ID_CARD_NO =
      KYCField._(18, _omitEnumNames ? '' : 'KYC_FIELD_INDIVIDUAL_ID_CARD_NO');
  static const KYCField KYC_FIELD_BUSINESS_COMPANY_NAME =
      KYCField._(20, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_COMPANY_NAME');
  static const KYCField KYC_FIELD_BUSINESS_TAX_ID =
      KYCField._(21, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_TAX_ID');
  static const KYCField KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE = KYCField._(
      22, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE');
  static const KYCField KYC_FIELD_BUSINESS_COMPANY_PHONE =
      KYCField._(25, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_COMPANY_PHONE');
  static const KYCField KYC_FIELD_BUSINESS_REP_ID_CARD_NO =
      KYCField._(26, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_REP_ID_CARD_NO');
  static const KYCField KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS = KYCField._(
      27, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS');
  static const KYCField KYC_FIELD_BUSINESS_COMPANY_ADDRESS = KYCField._(
      28, _omitEnumNames ? '' : 'KYC_FIELD_BUSINESS_COMPANY_ADDRESS');

  static const $core.List<KYCField> values = <KYCField>[
    KYC_FIELD_UNSPECIFIED,
    KYC_FIELD_REAL_NAME,
    KYC_FIELD_BIRTH_DATE,
    KYC_FIELD_ID_CARD_ISSUE_DATE,
    KYC_FIELD_ID_CARD_ISSUE_TYPE,
    KYC_FIELD_ID_CARD_ISSUE_LOCATION,
    KYC_FIELD_ENGLISH_NAME,
    KYC_FIELD_BANK_NAME,
    KYC_FIELD_BANK_CODE,
    KYC_FIELD_BANK_ACCOUNT,
    KYC_FIELD_PRODUCT_TYPE,
    KYC_FIELD_WEBSITE_URL,
    KYC_FIELD_ID_CARD_FRONT,
    KYC_FIELD_ID_CARD_BACK,
    KYC_FIELD_BANK_BOOK,
    KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
    KYC_FIELD_BUSINESS_COMPANY_NAME,
    KYC_FIELD_BUSINESS_TAX_ID,
    KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE,
    KYC_FIELD_BUSINESS_COMPANY_PHONE,
    KYC_FIELD_BUSINESS_REP_ID_CARD_NO,
    KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
    KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
  ];

  static final $core.List<KYCField?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 28);
  static KYCField? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const KYCField._(super.value, super.name);
}

/// KYCIssueCode 是退回時一個問題的原因類別：封閉集合，client 可以依它分支；具體的說明在 KYCIssue.message。
class KYCIssueCode extends $pb.ProtobufEnum {
  static const KYCIssueCode KYC_ISSUE_CODE_UNSPECIFIED =
      KYCIssueCode._(0, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_UNSPECIFIED');

  /// 看不清楚（圖片模糊、反光、被遮住）。
  static const KYCIssueCode KYC_ISSUE_CODE_UNREADABLE =
      KYCIssueCode._(1, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_UNREADABLE');

  /// 與其他資料或登記不一致。
  static const KYCIssueCode KYC_ISSUE_CODE_MISMATCH =
      KYCIssueCode._(2, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_MISMATCH');

  /// 已過期。
  static const KYCIssueCode KYC_ISSUE_CODE_EXPIRED =
      KYCIssueCode._(3, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_EXPIRED');

  /// 缺少。
  static const KYCIssueCode KYC_ISSUE_CODE_MISSING =
      KYCIssueCode._(4, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_MISSING');

  /// 格式或內容不符規定。
  static const KYCIssueCode KYC_ISSUE_CODE_INVALID =
      KYCIssueCode._(5, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_INVALID');

  /// 其他，靠 message 說明。
  static const KYCIssueCode KYC_ISSUE_CODE_OTHER =
      KYCIssueCode._(6, _omitEnumNames ? '' : 'KYC_ISSUE_CODE_OTHER');

  static const $core.List<KYCIssueCode> values = <KYCIssueCode>[
    KYC_ISSUE_CODE_UNSPECIFIED,
    KYC_ISSUE_CODE_UNREADABLE,
    KYC_ISSUE_CODE_MISMATCH,
    KYC_ISSUE_CODE_EXPIRED,
    KYC_ISSUE_CODE_MISSING,
    KYC_ISSUE_CODE_INVALID,
    KYC_ISSUE_CODE_OTHER,
  ];

  static final $core.List<KYCIssueCode?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static KYCIssueCode? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const KYCIssueCode._(super.value, super.name);
}

/// KYCFlowStatus 是 KYC 與 PayUni 兩條軸合成的流程狀態，給畫面顯示。
class KYCFlowStatus extends $pb.ProtobufEnum {
  static const KYCFlowStatus KYC_FLOW_STATUS_UNSPECIFIED =
      KYCFlowStatus._(0, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_UNSPECIFIED');

  /// 沒有申請，或申請已取消。
  static const KYCFlowStatus KYC_FLOW_STATUS_NOT_STARTED =
      KYCFlowStatus._(1, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_NOT_STARTED');
  static const KYCFlowStatus KYC_FLOW_STATUS_DRAFT =
      KYCFlowStatus._(2, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_DRAFT');
  static const KYCFlowStatus KYC_FLOW_STATUS_SUBMITTED =
      KYCFlowStatus._(3, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_SUBMITTED');
  static const KYCFlowStatus KYC_FLOW_STATUS_REJECTED =
      KYCFlowStatus._(4, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_REJECTED');

  /// 管理員核准，正在向 PayUni 註冊。
  static const KYCFlowStatus KYC_FLOW_STATUS_BINDING =
      KYCFlowStatus._(5, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_BINDING');

  /// 向 PayUni 註冊失敗，等管理員處理。
  static const KYCFlowStatus KYC_FLOW_STATUS_BIND_FAILED =
      KYCFlowStatus._(6, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_BIND_FAILED');

  /// 已向 PayUni 註冊，還沒有審核結果。
  static const KYCFlowStatus KYC_FLOW_STATUS_FORWARDED =
      KYCFlowStatus._(7, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_FORWARDED');
  static const KYCFlowStatus KYC_FLOW_STATUS_UNDER_REVIEW =
      KYCFlowStatus._(8, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_UNDER_REVIEW');
  static const KYCFlowStatus KYC_FLOW_STATUS_PENDING_INFO =
      KYCFlowStatus._(9, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_PENDING_INFO');

  /// PayUni 核准，可以收款。
  static const KYCFlowStatus KYC_FLOW_STATUS_APPROVED =
      KYCFlowStatus._(10, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_APPROVED');
  static const KYCFlowStatus KYC_FLOW_STATUS_PAYUNI_REJECTED = KYCFlowStatus._(
      11, _omitEnumNames ? '' : 'KYC_FLOW_STATUS_PAYUNI_REJECTED');

  static const $core.List<KYCFlowStatus> values = <KYCFlowStatus>[
    KYC_FLOW_STATUS_UNSPECIFIED,
    KYC_FLOW_STATUS_NOT_STARTED,
    KYC_FLOW_STATUS_DRAFT,
    KYC_FLOW_STATUS_SUBMITTED,
    KYC_FLOW_STATUS_REJECTED,
    KYC_FLOW_STATUS_BINDING,
    KYC_FLOW_STATUS_BIND_FAILED,
    KYC_FLOW_STATUS_FORWARDED,
    KYC_FLOW_STATUS_UNDER_REVIEW,
    KYC_FLOW_STATUS_PENDING_INFO,
    KYC_FLOW_STATUS_APPROVED,
    KYC_FLOW_STATUS_PAYUNI_REJECTED,
  ];

  static final $core.List<KYCFlowStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 11);
  static KYCFlowStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const KYCFlowStatus._(super.value, super.name);
}

/// PayUniBindStatus 是 PayUni 子商店的綁定狀態。
class PayUniBindStatus extends $pb.ProtobufEnum {
  static const PayUniBindStatus PAY_UNI_BIND_STATUS_UNSPECIFIED =
      PayUniBindStatus._(
          0, _omitEnumNames ? '' : 'PAY_UNI_BIND_STATUS_UNSPECIFIED');

  /// 意圖已寫入，背景程序還沒有送出或正在重試。
  static const PayUniBindStatus PAY_UNI_BIND_STATUS_PENDING =
      PayUniBindStatus._(
          1, _omitEnumNames ? '' : 'PAY_UNI_BIND_STATUS_PENDING');
  static const PayUniBindStatus PAY_UNI_BIND_STATUS_BOUND =
      PayUniBindStatus._(2, _omitEnumNames ? '' : 'PAY_UNI_BIND_STATUS_BOUND');

  /// 綁定失敗，等管理員重新綁定。
  static const PayUniBindStatus PAY_UNI_BIND_STATUS_FAILED =
      PayUniBindStatus._(3, _omitEnumNames ? '' : 'PAY_UNI_BIND_STATUS_FAILED');

  static const $core.List<PayUniBindStatus> values = <PayUniBindStatus>[
    PAY_UNI_BIND_STATUS_UNSPECIFIED,
    PAY_UNI_BIND_STATUS_PENDING,
    PAY_UNI_BIND_STATUS_BOUND,
    PAY_UNI_BIND_STATUS_FAILED,
  ];

  static final $core.List<PayUniBindStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static PayUniBindStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PayUniBindStatus._(super.value, super.name);
}

/// PayUniState 是賣家看到的 PayUni 收款狀態，一個值對應賣家的一種動作。伺服器由申請與 PayUni 註冊推導。
class PayUniState extends $pb.ProtobufEnum {
  static const PayUniState PAY_UNI_STATE_UNSPECIFIED =
      PayUniState._(0, _omitEnumNames ? '' : 'PAY_UNI_STATE_UNSPECIFIED');

  /// 綁定中或 PayUni 審核中，賣家等結果。
  static const PayUniState PAY_UNI_STATE_REVIEWING =
      PayUniState._(1, _omitEnumNames ? '' : 'PAY_UNI_STATE_REVIEWING');

  /// PayUni 要求補件，賣家依 PayUni 的通知信處理。
  static const PayUniState PAY_UNI_STATE_PENDING_INFO =
      PayUniState._(2, _omitEnumNames ? '' : 'PAY_UNI_STATE_PENDING_INFO');

  /// 申請的資料有問題，賣家修正申請後自己重新送出。
  static const PayUniState PAY_UNI_STATE_FIXABLE =
      PayUniState._(3, _omitEnumNames ? '' : 'PAY_UNI_STATE_FIXABLE');

  /// 店家設定的資料有問題，賣家修正後由平台重新處理，不能自己重新送出。
  static const PayUniState PAY_UNI_STATE_FIX_VIA_SUPPORT =
      PayUniState._(4, _omitEnumNames ? '' : 'PAY_UNI_STATE_FIX_VIA_SUPPORT');

  /// 沒有賣家能修正的資料，賣家要聯絡客服，不能自己重新送出。
  static const PayUniState PAY_UNI_STATE_CONTACT_SUPPORT =
      PayUniState._(5, _omitEnumNames ? '' : 'PAY_UNI_STATE_CONTACT_SUPPORT');

  /// PayUni 暫時無法處理，等平台重新送出。
  static const PayUniState PAY_UNI_STATE_RETRY_LATER =
      PayUniState._(6, _omitEnumNames ? '' : 'PAY_UNI_STATE_RETRY_LATER');

  /// PayUni 端的商店已是終止狀態，同一個店家無法再綁定。
  static const PayUniState PAY_UNI_STATE_TERMINATED =
      PayUniState._(7, _omitEnumNames ? '' : 'PAY_UNI_STATE_TERMINATED');

  /// 可以收款。
  static const PayUniState PAY_UNI_STATE_APPROVED =
      PayUniState._(8, _omitEnumNames ? '' : 'PAY_UNI_STATE_APPROVED');

  static const $core.List<PayUniState> values = <PayUniState>[
    PAY_UNI_STATE_UNSPECIFIED,
    PAY_UNI_STATE_REVIEWING,
    PAY_UNI_STATE_PENDING_INFO,
    PAY_UNI_STATE_FIXABLE,
    PAY_UNI_STATE_FIX_VIA_SUPPORT,
    PAY_UNI_STATE_CONTACT_SUPPORT,
    PAY_UNI_STATE_RETRY_LATER,
    PAY_UNI_STATE_TERMINATED,
    PAY_UNI_STATE_APPROVED,
  ];

  static final $core.List<PayUniState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 8);
  static PayUniState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PayUniState._(super.value, super.name);
}

/// KYCResubmitUnavailableReason 是 SaveKYCDraft 與 SubmitKYC 此刻不能做的原因（KYCResubmitUnavailability.reason）。
class KYCResubmitUnavailableReason extends $pb.ProtobufEnum {
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_UNSPECIFIED =
      KYCResubmitUnavailableReason._(0,
          _omitEnumNames ? '' : 'KYC_RESUBMIT_UNAVAILABLE_REASON_UNSPECIFIED');

  /// 申請送審中，可以先取消再修改。
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW_CANCELABLE =
      KYCResubmitUnavailableReason._(
          1,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW_CANCELABLE');

  /// 申請送審中，PayUni 已收下店家，不能取消。
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW =
      KYCResubmitUnavailableReason._(2,
          _omitEnumNames ? '' : 'KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW');

  /// 以下對應 PayUniState 裡不能重送的值。
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_REVIEWING =
      KYCResubmitUnavailableReason._(
          3,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_REVIEWING');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_PENDING_INFO =
      KYCResubmitUnavailableReason._(
          4,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_PENDING_INFO');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_FIX_VIA_SUPPORT =
      KYCResubmitUnavailableReason._(
          5,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_FIX_VIA_SUPPORT');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_CONTACT_SUPPORT =
      KYCResubmitUnavailableReason._(
          6,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_CONTACT_SUPPORT');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_RETRY_LATER =
      KYCResubmitUnavailableReason._(
          7,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_RETRY_LATER');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_TERMINATED =
      KYCResubmitUnavailableReason._(
          8,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_TERMINATED');
  static const KYCResubmitUnavailableReason
      KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_APPROVED =
      KYCResubmitUnavailableReason._(
          9,
          _omitEnumNames
              ? ''
              : 'KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_APPROVED');

  static const $core.List<KYCResubmitUnavailableReason> values =
      <KYCResubmitUnavailableReason>[
    KYC_RESUBMIT_UNAVAILABLE_REASON_UNSPECIFIED,
    KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW_CANCELABLE,
    KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_REVIEWING,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_PENDING_INFO,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_FIX_VIA_SUPPORT,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_CONTACT_SUPPORT,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_RETRY_LATER,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_TERMINATED,
    KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_APPROVED,
  ];

  static final $core.List<KYCResubmitUnavailableReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 9);
  static KYCResubmitUnavailableReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const KYCResubmitUnavailableReason._(super.value, super.name);
}

/// ViolationSeverity 是認定套用的嚴重度，也是案件的嚴重度（開案時決定，之後只經 ChangeCaseSeverity 修改）。
class ViolationSeverity extends $pb.ProtobufEnum {
  static const ViolationSeverity VIOLATION_SEVERITY_UNSPECIFIED =
      ViolationSeverity._(
          0, _omitEnumNames ? '' : 'VIOLATION_SEVERITY_UNSPECIFIED');
  static const ViolationSeverity VIOLATION_SEVERITY_LOW =
      ViolationSeverity._(1, _omitEnumNames ? '' : 'VIOLATION_SEVERITY_LOW');
  static const ViolationSeverity VIOLATION_SEVERITY_MEDIUM =
      ViolationSeverity._(2, _omitEnumNames ? '' : 'VIOLATION_SEVERITY_MEDIUM');
  static const ViolationSeverity VIOLATION_SEVERITY_HIGH =
      ViolationSeverity._(3, _omitEnumNames ? '' : 'VIOLATION_SEVERITY_HIGH');
  static const ViolationSeverity VIOLATION_SEVERITY_CRITICAL =
      ViolationSeverity._(
          4, _omitEnumNames ? '' : 'VIOLATION_SEVERITY_CRITICAL');

  static const $core.List<ViolationSeverity> values = <ViolationSeverity>[
    VIOLATION_SEVERITY_UNSPECIFIED,
    VIOLATION_SEVERITY_LOW,
    VIOLATION_SEVERITY_MEDIUM,
    VIOLATION_SEVERITY_HIGH,
    VIOLATION_SEVERITY_CRITICAL,
  ];

  static final $core.List<ViolationSeverity?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static ViolationSeverity? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ViolationSeverity._(super.value, super.name);
}

/// RevokeReason 是撤銷措施或認定的原因。
class RevokeReason extends $pb.ProtobufEnum {
  static const RevokeReason REVOKE_REASON_UNSPECIFIED =
      RevokeReason._(0, _omitEnumNames ? '' : 'REVOKE_REASON_UNSPECIFIED');

  /// 接受申覆；只由裁定申覆寫入。
  static const RevokeReason REVOKE_REASON_APPEAL_ACCEPTED =
      RevokeReason._(1, _omitEnumNames ? '' : 'REVOKE_REASON_APPEAL_ACCEPTED');

  /// 平台更正誤判。
  static const RevokeReason REVOKE_REASON_ADMIN_CORRECTION =
      RevokeReason._(2, _omitEnumNames ? '' : 'REVOKE_REASON_ADMIN_CORRECTION');

  /// 主管機關或法院的決定。
  static const RevokeReason REVOKE_REASON_EXTERNAL_DECISION = RevokeReason._(
      3, _omitEnumNames ? '' : 'REVOKE_REASON_EXTERNAL_DECISION');

  /// 其他原因。
  static const RevokeReason REVOKE_REASON_OTHER =
      RevokeReason._(4, _omitEnumNames ? '' : 'REVOKE_REASON_OTHER');

  /// 措施引用的認定被撤銷，措施連帶撤銷；只由連帶撤銷寫入。
  static const RevokeReason REVOKE_REASON_FINDING_REVOKED =
      RevokeReason._(5, _omitEnumNames ? '' : 'REVOKE_REASON_FINDING_REVOKED');

  /// 關聯帳號的終止（TerminationGround.LINKED_ACCOUNT）所依據的來源終止被撤銷，連帶撤銷；只由連帶撤銷寫入。
  static const RevokeReason REVOKE_REASON_SOURCE_REVOKED =
      RevokeReason._(6, _omitEnumNames ? '' : 'REVOKE_REASON_SOURCE_REVOKED');

  static const $core.List<RevokeReason> values = <RevokeReason>[
    REVOKE_REASON_UNSPECIFIED,
    REVOKE_REASON_APPEAL_ACCEPTED,
    REVOKE_REASON_ADMIN_CORRECTION,
    REVOKE_REASON_EXTERNAL_DECISION,
    REVOKE_REASON_OTHER,
    REVOKE_REASON_FINDING_REVOKED,
    REVOKE_REASON_SOURCE_REVOKED,
  ];

  static final $core.List<RevokeReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static RevokeReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RevokeReason._(super.value, super.name);
}

/// ViolationStatus 是認定（違規）的狀態。
class ViolationStatus extends $pb.ProtobufEnum {
  static const ViolationStatus VIOLATION_STATUS_UNSPECIFIED = ViolationStatus._(
      0, _omitEnumNames ? '' : 'VIOLATION_STATUS_UNSPECIFIED');
  static const ViolationStatus VIOLATION_STATUS_ENFORCED =
      ViolationStatus._(1, _omitEnumNames ? '' : 'VIOLATION_STATUS_ENFORCED');

  /// 已撤銷（平台更正、接受申覆或主管機關的決定）。
  static const ViolationStatus VIOLATION_STATUS_REVOKED =
      ViolationStatus._(2, _omitEnumNames ? '' : 'VIOLATION_STATUS_REVOKED');

  static const $core.List<ViolationStatus> values = <ViolationStatus>[
    VIOLATION_STATUS_UNSPECIFIED,
    VIOLATION_STATUS_ENFORCED,
    VIOLATION_STATUS_REVOKED,
  ];

  static final $core.List<ViolationStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ViolationStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ViolationStatus._(super.value, super.name);
}

/// MeasureKind 是措施的種類（條款第十七條）。
class MeasureKind extends $pb.ProtobufEnum {
  static const MeasureKind MEASURE_KIND_UNSPECIFIED =
      MeasureKind._(0, _omitEnumNames ? '' : 'MEASURE_KIND_UNSPECIFIED');

  /// 沒有限制，也沒有期間。
  static const MeasureKind MEASURE_KIND_WARNING =
      MeasureKind._(1, _omitEnumNames ? '' : 'MEASURE_KIND_WARNING');

  /// 帳號在期間內被限制。
  static const MeasureKind MEASURE_KIND_SUSPENSION =
      MeasureKind._(2, _omitEnumNames ? '' : 'MEASURE_KIND_SUSPENSION');

  /// 終止賣家合作條款（第二十一條）。生效時（period.start_time）店家暫停營業，未結訂單完成後店家關閉；沒有固定結束時間，
  /// 也沒有解除請求、複審或結束，只能由撤銷、申覆成立解除。事由看 Measure.termination_ground；申覆期間到 appeal_deadline_time 為止。
  static const MeasureKind MEASURE_KIND_TERMINATION =
      MeasureKind._(3, _omitEnumNames ? '' : 'MEASURE_KIND_TERMINATION');

  static const $core.List<MeasureKind> values = <MeasureKind>[
    MEASURE_KIND_UNSPECIFIED,
    MEASURE_KIND_WARNING,
    MEASURE_KIND_SUSPENSION,
    MEASURE_KIND_TERMINATION,
  ];

  static final $core.List<MeasureKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static MeasureKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureKind._(super.value, super.name);
}

/// MeasureBasis 是措施的依據：VIOLATION 依據違規的認定（有固定期間的處分），RISK 依據條款第 17.1 條的風險類型或第 17.5 條
/// 的外部要求（沒有固定期間時由平台定期複審），CONTRACT 依據契約條款（只配終止合作，無過錯或與認定無關）。
class MeasureBasis extends $pb.ProtobufEnum {
  static const MeasureBasis MEASURE_BASIS_UNSPECIFIED =
      MeasureBasis._(0, _omitEnumNames ? '' : 'MEASURE_BASIS_UNSPECIFIED');
  static const MeasureBasis MEASURE_BASIS_VIOLATION =
      MeasureBasis._(1, _omitEnumNames ? '' : 'MEASURE_BASIS_VIOLATION');
  static const MeasureBasis MEASURE_BASIS_RISK =
      MeasureBasis._(2, _omitEnumNames ? '' : 'MEASURE_BASIS_RISK');

  /// 終止合作依據賣家合作條款第 21 條，沒有認定也沒有風險類型。
  static const MeasureBasis MEASURE_BASIS_CONTRACT =
      MeasureBasis._(3, _omitEnumNames ? '' : 'MEASURE_BASIS_CONTRACT');

  static const $core.List<MeasureBasis> values = <MeasureBasis>[
    MEASURE_BASIS_UNSPECIFIED,
    MEASURE_BASIS_VIOLATION,
    MEASURE_BASIS_RISK,
    MEASURE_BASIS_CONTRACT,
  ];

  static final $core.List<MeasureBasis?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static MeasureBasis? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureBasis._(super.value, super.name);
}

/// RiskGround 是 RISK 措施的風險類型（條款第 17.1 條）與外部要求（第 17.5 條）。
class RiskGround extends $pb.ProtobufEnum {
  static const RiskGround RISK_GROUND_UNSPECIFIED =
      RiskGround._(0, _omitEnumNames ? '' : 'RISK_GROUND_UNSPECIFIED');

  /// 交易風險。
  static const RiskGround RISK_GROUND_TRANSACTION =
      RiskGround._(1, _omitEnumNames ? '' : 'RISK_GROUND_TRANSACTION');

  /// 食品安全風險。
  static const RiskGround RISK_GROUND_FOOD_SAFETY =
      RiskGround._(2, _omitEnumNames ? '' : 'RISK_GROUND_FOOD_SAFETY');

  /// 詐欺風險。
  static const RiskGround RISK_GROUND_FRAUD =
      RiskGround._(3, _omitEnumNames ? '' : 'RISK_GROUND_FRAUD');

  /// 個人資料風險。
  static const RiskGround RISK_GROUND_PERSONAL_DATA =
      RiskGround._(4, _omitEnumNames ? '' : 'RISK_GROUND_PERSONAL_DATA');

  /// 侵權風險。
  static const RiskGround RISK_GROUND_INFRINGEMENT =
      RiskGround._(5, _omitEnumNames ? '' : 'RISK_GROUND_INFRINGEMENT');

  /// 資安風險。
  static const RiskGround RISK_GROUND_SECURITY =
      RiskGround._(6, _omitEnumNames ? '' : 'RISK_GROUND_SECURITY');

  /// 履約風險。
  static const RiskGround RISK_GROUND_FULFILLMENT =
      RiskGround._(7, _omitEnumNames ? '' : 'RISK_GROUND_FULFILLMENT');

  /// 主管機關或司法機關的要求（第 17.5 條）。
  static const RiskGround RISK_GROUND_EXTERNAL_ORDER =
      RiskGround._(8, _omitEnumNames ? '' : 'RISK_GROUND_EXTERNAL_ORDER');

  static const $core.List<RiskGround> values = <RiskGround>[
    RISK_GROUND_UNSPECIFIED,
    RISK_GROUND_TRANSACTION,
    RISK_GROUND_FOOD_SAFETY,
    RISK_GROUND_FRAUD,
    RISK_GROUND_PERSONAL_DATA,
    RISK_GROUND_INFRINGEMENT,
    RISK_GROUND_SECURITY,
    RISK_GROUND_FULFILLMENT,
    RISK_GROUND_EXTERNAL_ORDER,
  ];

  static final $core.List<RiskGround?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 8);
  static RiskGround? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RiskGround._(super.value, super.name);
}

/// MeasureState 是措施此刻的狀態，由伺服器在讀取時推導，不儲存：時間到了不改變 etag，只改變 state。
class MeasureState extends $pb.ProtobufEnum {
  static const MeasureState MEASURE_STATE_UNSPECIFIED =
      MeasureState._(0, _omitEnumNames ? '' : 'MEASURE_STATE_UNSPECIFIED');

  /// 警告，未撤銷（不是「生效中」，沒有限制）。
  static const MeasureState MEASURE_STATE_ISSUED =
      MeasureState._(1, _omitEnumNames ? '' : 'MEASURE_STATE_ISSUED');

  /// 停權生效中。終止合作生效後（含店家已關閉）也是 ACTIVE。
  static const MeasureState MEASURE_STATE_ACTIVE =
      MeasureState._(2, _omitEnumNames ? '' : 'MEASURE_STATE_ACTIVE');

  /// 停權已結束（到期或平台結束）。
  static const MeasureState MEASURE_STATE_ENDED =
      MeasureState._(3, _omitEnumNames ? '' : 'MEASURE_STATE_ENDED');

  /// 措施已撤銷。
  static const MeasureState MEASURE_STATE_REVOKED =
      MeasureState._(4, _omitEnumNames ? '' : 'MEASURE_STATE_REVOKED');

  /// 已建立、還沒到開始時間（period.start_time 在未來）；目前只有預告終止（TerminationGround.NOTICE）。此時帳號不受限制。
  /// 預告期內店家關閉，措施直接變 ENDED，不會變 ACTIVE。
  static const MeasureState MEASURE_STATE_SCHEDULED =
      MeasureState._(5, _omitEnumNames ? '' : 'MEASURE_STATE_SCHEDULED');

  static const $core.List<MeasureState> values = <MeasureState>[
    MEASURE_STATE_UNSPECIFIED,
    MEASURE_STATE_ISSUED,
    MEASURE_STATE_ACTIVE,
    MEASURE_STATE_ENDED,
    MEASURE_STATE_REVOKED,
    MEASURE_STATE_SCHEDULED,
  ];

  static final $core.List<MeasureState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static MeasureState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureState._(super.value, super.name);
}

/// MeasureRequestKind 是措施上請求的種類。
class MeasureRequestKind extends $pb.ProtobufEnum {
  static const MeasureRequestKind MEASURE_REQUEST_KIND_UNSPECIFIED =
      MeasureRequestKind._(
          0, _omitEnumNames ? '' : 'MEASURE_REQUEST_KIND_UNSPECIFIED');

  /// 對措施本身的決定提出異議；接受時措施撤銷、效果解除。
  static const MeasureRequestKind MEASURE_REQUEST_KIND_APPEAL =
      MeasureRequestKind._(
          1, _omitEnumNames ? '' : 'MEASURE_REQUEST_KIND_APPEAL');

  /// 完成補救後請求解除（只有 RISK 依據的措施）；接受時措施結束（state 變 ENDED、期間結束在接受的當下），不是撤銷，
  /// 措施的依據與認定都不變，仍可申覆。
  static const MeasureRequestKind MEASURE_REQUEST_KIND_REMEDIATION =
      MeasureRequestKind._(
          2, _omitEnumNames ? '' : 'MEASURE_REQUEST_KIND_REMEDIATION');

  static const $core.List<MeasureRequestKind> values = <MeasureRequestKind>[
    MEASURE_REQUEST_KIND_UNSPECIFIED,
    MEASURE_REQUEST_KIND_APPEAL,
    MEASURE_REQUEST_KIND_REMEDIATION,
  ];

  static final $core.List<MeasureRequestKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static MeasureRequestKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureRequestKind._(super.value, super.name);
}

/// MeasureRequestState 是措施上請求（申覆與解除請求）的狀態；由伺服器推導（措施已不在生效中而仍待處理的解除請求是 CLOSED，見
/// MeasureRequestCloseCause）。ACCEPTED 的效果看 MeasureRequest.kind。
class MeasureRequestState extends $pb.ProtobufEnum {
  static const MeasureRequestState MEASURE_REQUEST_STATE_UNSPECIFIED =
      MeasureRequestState._(
          0, _omitEnumNames ? '' : 'MEASURE_REQUEST_STATE_UNSPECIFIED');
  static const MeasureRequestState MEASURE_REQUEST_STATE_PENDING =
      MeasureRequestState._(
          1, _omitEnumNames ? '' : 'MEASURE_REQUEST_STATE_PENDING');
  static const MeasureRequestState MEASURE_REQUEST_STATE_ACCEPTED =
      MeasureRequestState._(
          2, _omitEnumNames ? '' : 'MEASURE_REQUEST_STATE_ACCEPTED');
  static const MeasureRequestState MEASURE_REQUEST_STATE_REJECTED =
      MeasureRequestState._(
          3, _omitEnumNames ? '' : 'MEASURE_REQUEST_STATE_REJECTED');

  /// 請求沒有經裁定就結案，原因看 MeasureRequest.close_cause。
  static const MeasureRequestState MEASURE_REQUEST_STATE_CLOSED =
      MeasureRequestState._(
          4, _omitEnumNames ? '' : 'MEASURE_REQUEST_STATE_CLOSED');

  static const $core.List<MeasureRequestState> values = <MeasureRequestState>[
    MEASURE_REQUEST_STATE_UNSPECIFIED,
    MEASURE_REQUEST_STATE_PENDING,
    MEASURE_REQUEST_STATE_ACCEPTED,
    MEASURE_REQUEST_STATE_REJECTED,
    MEASURE_REQUEST_STATE_CLOSED,
  ];

  static final $core.List<MeasureRequestState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static MeasureRequestState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureRequestState._(super.value, super.name);
}

/// RemediationUnavailableReason 是賣家此刻不能請求解除的原因（RemediationUnavailability.reason）。client 用它分支，不用它組顯示文字。
class RemediationUnavailableReason extends $pb.ProtobufEnum {
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_UNSPECIFIED =
      RemediationUnavailableReason._(0,
          _omitEnumNames ? '' : 'REMEDIATION_UNAVAILABLE_REASON_UNSPECIFIED');

  /// 有待處理的解除請求。
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING =
      RemediationUnavailableReason._(
          1,
          _omitEnumNames
              ? ''
              : 'REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING');

  /// 有待處理的申覆。
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING =
      RemediationUnavailableReason._(
          2,
          _omitEnumNames
              ? ''
              : 'REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING');

  /// 在解除請求的冷卻期內，到 Measure.next_remediation_time 可以再提出。
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_COOLDOWN = RemediationUnavailableReason._(
          3, _omitEnumNames ? '' : 'REMEDIATION_UNAVAILABLE_REASON_COOLDOWN');

  /// 措施的依據不是風險類型（VIOLATION 依據沒有解除請求入口）。
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_NOT_RISK = RemediationUnavailableReason._(
          4, _omitEnumNames ? '' : 'REMEDIATION_UNAVAILABLE_REASON_NOT_RISK');

  /// 措施不在生效中（已撤銷、已結束或是警告）。
  static const RemediationUnavailableReason
      REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE =
      RemediationUnavailableReason._(
          5, _omitEnumNames ? '' : 'REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE');

  static const $core.List<RemediationUnavailableReason> values =
      <RemediationUnavailableReason>[
    REMEDIATION_UNAVAILABLE_REASON_UNSPECIFIED,
    REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING,
    REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING,
    REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
    REMEDIATION_UNAVAILABLE_REASON_NOT_RISK,
    REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE,
  ];

  static final $core.List<RemediationUnavailableReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static RemediationUnavailableReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RemediationUnavailableReason._(super.value, super.name);
}

/// TerminationGround 是終止合作（MeasureKind.TERMINATION）的事由，對應賣家合作條款第 21.2、21.3 條。
class TerminationGround extends $pb.ProtobufEnum {
  static const TerminationGround TERMINATION_GROUND_UNSPECIFIED =
      TerminationGround._(
          0, _omitEnumNames ? '' : 'TERMINATION_GROUND_UNSPECIFIED');

  /// 甲方預告終止，建立後 30 日生效（period.start_time）；預告期間店家照常營業，賣家可申覆。無過錯，
  /// 依據是 CONTRACT；不禁止賣家日後重新申請開店，不取身分快照。
  static const TerminationGround TERMINATION_GROUND_NOTICE =
      TerminationGround._(1, _omitEnumNames ? '' : 'TERMINATION_GROUND_NOTICE');

  /// 第 21.3 條第 1 款，立即生效。依據是 VIOLATION，至少引用一筆已存在 7 日以上、未撤銷的認定。
  static const TerminationGround TERMINATION_GROUND_MATERIAL_BREACH =
      TerminationGround._(
          2, _omitEnumNames ? '' : 'TERMINATION_GROUND_MATERIAL_BREACH');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_CEASED_BUSINESS =
      TerminationGround._(
          3, _omitEnumNames ? '' : 'TERMINATION_GROUND_CEASED_BUSINESS');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_DISSOLUTION =
      TerminationGround._(
          4, _omitEnumNames ? '' : 'TERMINATION_GROUND_DISSOLUTION');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_BANKRUPTCY =
      TerminationGround._(
          5, _omitEnumNames ? '' : 'TERMINATION_GROUND_BANKRUPTCY');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_LICENSE_REVOKED =
      TerminationGround._(
          6, _omitEnumNames ? '' : 'TERMINATION_GROUND_LICENSE_REVOKED');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_FOOD_SAFETY_INCIDENT =
      TerminationGround._(
          7, _omitEnumNames ? '' : 'TERMINATION_GROUND_FOOD_SAFETY_INCIDENT');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_FRAUD =
      TerminationGround._(8, _omitEnumNames ? '' : 'TERMINATION_GROUND_FRAUD');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_PLATFORM_SECURITY =
      TerminationGround._(
          9, _omitEnumNames ? '' : 'TERMINATION_GROUND_PLATFORM_SECURITY');

  /// 第 21.3 條，立即生效。
  static const TerminationGround TERMINATION_GROUND_OTHER_SERIOUS =
      TerminationGround._(
          10, _omitEnumNames ? '' : 'TERMINATION_GROUND_OTHER_SERIOUS');

  /// 與另一個帳號的終止（來源措施）使用同一個身分證明文件或統一編號，立即生效；來源措施撤銷時一併撤銷
  /// （RevokeReason.SOURCE_REVOKED）。賣家可對這筆單獨申覆。
  static const TerminationGround TERMINATION_GROUND_LINKED_ACCOUNT =
      TerminationGround._(
          11, _omitEnumNames ? '' : 'TERMINATION_GROUND_LINKED_ACCOUNT');

  static const $core.List<TerminationGround> values = <TerminationGround>[
    TERMINATION_GROUND_UNSPECIFIED,
    TERMINATION_GROUND_NOTICE,
    TERMINATION_GROUND_MATERIAL_BREACH,
    TERMINATION_GROUND_CEASED_BUSINESS,
    TERMINATION_GROUND_DISSOLUTION,
    TERMINATION_GROUND_BANKRUPTCY,
    TERMINATION_GROUND_LICENSE_REVOKED,
    TERMINATION_GROUND_FOOD_SAFETY_INCIDENT,
    TERMINATION_GROUND_FRAUD,
    TERMINATION_GROUND_PLATFORM_SECURITY,
    TERMINATION_GROUND_OTHER_SERIOUS,
    TERMINATION_GROUND_LINKED_ACCOUNT,
  ];

  static final $core.List<TerminationGround?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 11);
  static TerminationGround? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const TerminationGround._(super.value, super.name);
}

/// AppealUnavailableReason 是賣家此刻不能對措施提出申覆的原因（AppealUnavailability.reason），依優先序排列；空表示可以申覆。
/// client 用它分支，不用它組顯示文字。
class AppealUnavailableReason extends $pb.ProtobufEnum {
  static const AppealUnavailableReason APPEAL_UNAVAILABLE_REASON_UNSPECIFIED =
      AppealUnavailableReason._(
          0, _omitEnumNames ? '' : 'APPEAL_UNAVAILABLE_REASON_UNSPECIFIED');

  /// 措施已撤銷。
  static const AppealUnavailableReason APPEAL_UNAVAILABLE_REASON_REVOKED =
      AppealUnavailableReason._(
          1, _omitEnumNames ? '' : 'APPEAL_UNAVAILABLE_REASON_REVOKED');

  /// 同一措施只能申覆一次。
  static const AppealUnavailableReason
      APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED = AppealUnavailableReason._(2,
          _omitEnumNames ? '' : 'APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED');

  /// 終止合作的申覆期間已結束（Measure.appeal_deadline_time）。
  static const AppealUnavailableReason
      APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED = AppealUnavailableReason._(
          3,
          _omitEnumNames
              ? ''
              : 'APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED');

  /// 措施上有待處理的請求（申覆或解除請求）。
  static const AppealUnavailableReason
      APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING = AppealUnavailableReason._(
          4, _omitEnumNames ? '' : 'APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING');

  static const $core.List<AppealUnavailableReason> values =
      <AppealUnavailableReason>[
    APPEAL_UNAVAILABLE_REASON_UNSPECIFIED,
    APPEAL_UNAVAILABLE_REASON_REVOKED,
    APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED,
    APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED,
    APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING,
  ];

  static final $core.List<AppealUnavailableReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static AppealUnavailableReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AppealUnavailableReason._(super.value, super.name);
}

/// MeasureRequestCloseCause 是請求以 CLOSED 結案的原因。
class MeasureRequestCloseCause extends $pb.ProtobufEnum {
  static const MeasureRequestCloseCause
      MEASURE_REQUEST_CLOSE_CAUSE_UNSPECIFIED = MeasureRequestCloseCause._(
          0, _omitEnumNames ? '' : 'MEASURE_REQUEST_CLOSE_CAUSE_UNSPECIFIED');

  /// 措施被撤銷，待處理的請求由平台結案。
  static const MeasureRequestCloseCause MEASURE_REQUEST_CLOSE_CAUSE_REVOKED =
      MeasureRequestCloseCause._(
          1, _omitEnumNames ? '' : 'MEASURE_REQUEST_CLOSE_CAUSE_REVOKED');

  /// 措施已結束（到期或縮短到期），待處理的解除請求失去標的，由系統結案。
  static const MeasureRequestCloseCause MEASURE_REQUEST_CLOSE_CAUSE_ENDED =
      MeasureRequestCloseCause._(
          2, _omitEnumNames ? '' : 'MEASURE_REQUEST_CLOSE_CAUSE_ENDED');

  static const $core.List<MeasureRequestCloseCause> values =
      <MeasureRequestCloseCause>[
    MEASURE_REQUEST_CLOSE_CAUSE_UNSPECIFIED,
    MEASURE_REQUEST_CLOSE_CAUSE_REVOKED,
    MEASURE_REQUEST_CLOSE_CAUSE_ENDED,
  ];

  static final $core.List<MeasureRequestCloseCause?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static MeasureRequestCloseCause? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MeasureRequestCloseCause._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
