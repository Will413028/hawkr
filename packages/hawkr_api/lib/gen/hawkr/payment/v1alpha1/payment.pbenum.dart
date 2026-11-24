// This is a generated file - do not edit.
//
// Generated from hawkr/payment/v1alpha1/payment.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// PaymentStatus 是付款的狀態，只由金流商回報的 PaymentIntent 狀態推導。
class PaymentStatus extends $pb.ProtobufEnum {
  static const PaymentStatus PAYMENT_STATUS_UNSPECIFIED =
      PaymentStatus._(0, _omitEnumNames ? '' : 'PAYMENT_STATUS_UNSPECIFIED');

  /// 付款已記錄，金流商那邊的付款物件還沒確認建立（含呼叫過、結果不明）；背景補做，client 稍後重讀。
  static const PaymentStatus PAYMENT_STATUS_CREATING =
      PaymentStatus._(1, _omitEnumNames ? '' : 'PAYMENT_STATUS_CREATING');

  /// 已建立，等買家付款（含需要 3DS 的額外驗證）。
  static const PaymentStatus PAYMENT_STATUS_AWAITING_PAYMENT = PaymentStatus._(
      2, _omitEnumNames ? '' : 'PAYMENT_STATUS_AWAITING_PAYMENT');

  /// 買家已送出付款，金流商處理中。
  static const PaymentStatus PAYMENT_STATUS_PROCESSING =
      PaymentStatus._(3, _omitEnumNames ? '' : 'PAYMENT_STATUS_PROCESSING');

  /// 已入帳（終局）。
  static const PaymentStatus PAYMENT_STATUS_SUCCEEDED =
      PaymentStatus._(4, _omitEnumNames ? '' : 'PAYMENT_STATUS_SUCCEEDED');

  /// 已取消（之後的入帳仍勝出）；訂單仍可付時 CreatePayment 會換新的嘗試。
  static const PaymentStatus PAYMENT_STATUS_CANCELED =
      PaymentStatus._(5, _omitEnumNames ? '' : 'PAYMENT_STATUS_CANCELED');

  /// 建立被金流商永久拒絕，或查回證明金流商沒有這筆。
  static const PaymentStatus PAYMENT_STATUS_CREATE_FAILED =
      PaymentStatus._(6, _omitEnumNames ? '' : 'PAYMENT_STATUS_CREATE_FAILED');

  /// 建立的結果與金流商對不起來：待營運人工處理，買家不能重試。
  static const PaymentStatus PAYMENT_STATUS_ANOMALY =
      PaymentStatus._(7, _omitEnumNames ? '' : 'PAYMENT_STATUS_ANOMALY');

  static const $core.List<PaymentStatus> values = <PaymentStatus>[
    PAYMENT_STATUS_UNSPECIFIED,
    PAYMENT_STATUS_CREATING,
    PAYMENT_STATUS_AWAITING_PAYMENT,
    PAYMENT_STATUS_PROCESSING,
    PAYMENT_STATUS_SUCCEEDED,
    PAYMENT_STATUS_CANCELED,
    PAYMENT_STATUS_CREATE_FAILED,
    PAYMENT_STATUS_ANOMALY,
  ];

  static final $core.List<PaymentStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static PaymentStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentStatus._(super.value, super.name);
}

/// PaymentAccountStatus 是收款帳戶的中立狀態，由金流商的轉帳能力推導（不是金流商自己的狀態名）。
class PaymentAccountStatus extends $pb.ProtobufEnum {
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_UNSPECIFIED =
      PaymentAccountStatus._(
          0, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_UNSPECIFIED');

  /// 平台已核准，外部帳戶還在建立（背景程序處理，失敗時進後台待辦）。
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_NOT_CREATED =
      PaymentAccountStatus._(
          1, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_NOT_CREATED');

  /// 轉帳能力還沒啟用，等金流商審核，賣家沒有要做的事。
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW =
      PaymentAccountStatus._(
          2, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW');

  /// 賣家要補資料（包含新帳戶尚未開通）：重新取得開通 session 完成。
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED =
      PaymentAccountStatus._(
          3, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED');

  /// 轉帳能力已啟用；平台審核同時核准時，店家通過開通閘門（SELLER_REQUIREMENT_ONBOARDING_COMPLETE）。
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_ENABLED =
      PaymentAccountStatus._(
          4, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_ENABLED');

  /// 被金流商拒絕、不支援或限制，賣家補件解不開，要由平台或金流商處理。
  static const PaymentAccountStatus PAYMENT_ACCOUNT_STATUS_DISABLED =
      PaymentAccountStatus._(
          5, _omitEnumNames ? '' : 'PAYMENT_ACCOUNT_STATUS_DISABLED');

  static const $core.List<PaymentAccountStatus> values = <PaymentAccountStatus>[
    PAYMENT_ACCOUNT_STATUS_UNSPECIFIED,
    PAYMENT_ACCOUNT_STATUS_NOT_CREATED,
    PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
    PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED,
    PAYMENT_ACCOUNT_STATUS_ENABLED,
    PAYMENT_ACCOUNT_STATUS_DISABLED,
  ];

  static final $core.List<PaymentAccountStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static PaymentAccountStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentAccountStatus._(super.value, super.name);
}

/// RequirementCollection 是誰向賣家收身分與銀行資料。
class RequirementCollection extends $pb.ProtobufEnum {
  static const RequirementCollection REQUIREMENT_COLLECTION_UNSPECIFIED =
      RequirementCollection._(
          0, _omitEnumNames ? '' : 'REQUIREMENT_COLLECTION_UNSPECIFIED');

  /// 金流商收件（託管頁面為預設）：平台不經手身分與銀行資料。
  static const RequirementCollection REQUIREMENT_COLLECTION_PROVIDER =
      RequirementCollection._(
          1, _omitEnumNames ? '' : 'REQUIREMENT_COLLECTION_PROVIDER');

  /// 平台收件（金流商的內嵌元件）：資料在網頁上由元件直接送給金流商。
  static const RequirementCollection REQUIREMENT_COLLECTION_PLATFORM =
      RequirementCollection._(
          2, _omitEnumNames ? '' : 'REQUIREMENT_COLLECTION_PLATFORM');

  static const $core.List<RequirementCollection> values =
      <RequirementCollection>[
    REQUIREMENT_COLLECTION_UNSPECIFIED,
    REQUIREMENT_COLLECTION_PROVIDER,
    REQUIREMENT_COLLECTION_PLATFORM,
  ];

  static final $core.List<RequirementCollection?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static RequirementCollection? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RequirementCollection._(super.value, super.name);
}

/// EmbeddedOnboardingRuntime 是呼叫端能承載的內嵌開通執行環境。
class EmbeddedOnboardingRuntime extends $pb.ProtobufEnum {
  static const EmbeddedOnboardingRuntime
      EMBEDDED_ONBOARDING_RUNTIME_UNSPECIFIED = EmbeddedOnboardingRuntime._(
          0, _omitEnumNames ? '' : 'EMBEDDED_ONBOARDING_RUNTIME_UNSPECIFIED');

  /// Stripe Connect.js（瀏覽器）。
  static const EmbeddedOnboardingRuntime
      EMBEDDED_ONBOARDING_RUNTIME_STRIPE_CONNECT_JS =
      EmbeddedOnboardingRuntime._(
          1,
          _omitEnumNames
              ? ''
              : 'EMBEDDED_ONBOARDING_RUNTIME_STRIPE_CONNECT_JS');

  static const $core.List<EmbeddedOnboardingRuntime> values =
      <EmbeddedOnboardingRuntime>[
    EMBEDDED_ONBOARDING_RUNTIME_UNSPECIFIED,
    EMBEDDED_ONBOARDING_RUNTIME_STRIPE_CONNECT_JS,
  ];

  static final $core.List<EmbeddedOnboardingRuntime?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static EmbeddedOnboardingRuntime? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EmbeddedOnboardingRuntime._(super.value, super.name);
}

/// PaymentRefundStatus 是退款的狀態（只給後台）。
class PaymentRefundStatus extends $pb.ProtobufEnum {
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_UNSPECIFIED =
      PaymentRefundStatus._(
          0, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_UNSPECIFIED');

  /// 已記錄，還沒呼叫金流商。
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_PENDING =
      PaymentRefundStatus._(
          1, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_PENDING');

  /// 已呼叫金流商，等結果。
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_REFUNDING =
      PaymentRefundStatus._(
          2, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_REFUNDING');

  /// 退款成功。
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_SUCCEEDED =
      PaymentRefundStatus._(
          3, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_SUCCEEDED');

  /// 退款失敗，不佔累計額度。
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_FAILED =
      PaymentRefundStatus._(
          4, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_FAILED');

  /// 與金流商對不起來：仍佔額度，待營運人工結案。
  static const PaymentRefundStatus PAYMENT_REFUND_STATUS_ANOMALY =
      PaymentRefundStatus._(
          5, _omitEnumNames ? '' : 'PAYMENT_REFUND_STATUS_ANOMALY');

  static const $core.List<PaymentRefundStatus> values = <PaymentRefundStatus>[
    PAYMENT_REFUND_STATUS_UNSPECIFIED,
    PAYMENT_REFUND_STATUS_PENDING,
    PAYMENT_REFUND_STATUS_REFUNDING,
    PAYMENT_REFUND_STATUS_SUCCEEDED,
    PAYMENT_REFUND_STATUS_FAILED,
    PAYMENT_REFUND_STATUS_ANOMALY,
  ];

  static final $core.List<PaymentRefundStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static PaymentRefundStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentRefundStatus._(super.value, super.name);
}

/// PaymentTransferStatus 是付款轉給賣家的轉帳的狀態（只給後台）。
class PaymentTransferStatus extends $pb.ProtobufEnum {
  static const PaymentTransferStatus PAYMENT_TRANSFER_STATUS_UNSPECIFIED =
      PaymentTransferStatus._(
          0, _omitEnumNames ? '' : 'PAYMENT_TRANSFER_STATUS_UNSPECIFIED');

  /// 已記錄，等送出或等結果。
  static const PaymentTransferStatus PAYMENT_TRANSFER_STATUS_PENDING =
      PaymentTransferStatus._(
          1, _omitEnumNames ? '' : 'PAYMENT_TRANSFER_STATUS_PENDING');

  /// 轉帳成功。
  static const PaymentTransferStatus PAYMENT_TRANSFER_STATUS_SUCCEEDED =
      PaymentTransferStatus._(
          2, _omitEnumNames ? '' : 'PAYMENT_TRANSFER_STATUS_SUCCEEDED');

  /// 轉帳失敗（終局）。
  static const PaymentTransferStatus PAYMENT_TRANSFER_STATUS_FAILED =
      PaymentTransferStatus._(
          3, _omitEnumNames ? '' : 'PAYMENT_TRANSFER_STATUS_FAILED');

  /// 與金流商對不起來：佔住這筆付款的轉帳，待營運人工結案。
  static const PaymentTransferStatus PAYMENT_TRANSFER_STATUS_ANOMALY =
      PaymentTransferStatus._(
          4, _omitEnumNames ? '' : 'PAYMENT_TRANSFER_STATUS_ANOMALY');

  static const $core.List<PaymentTransferStatus> values =
      <PaymentTransferStatus>[
    PAYMENT_TRANSFER_STATUS_UNSPECIFIED,
    PAYMENT_TRANSFER_STATUS_PENDING,
    PAYMENT_TRANSFER_STATUS_SUCCEEDED,
    PAYMENT_TRANSFER_STATUS_FAILED,
    PAYMENT_TRANSFER_STATUS_ANOMALY,
  ];

  static final $core.List<PaymentTransferStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static PaymentTransferStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentTransferStatus._(super.value, super.name);
}

/// PaymentReversalStatus 是退款向賣家收回款項（撤回轉帳）的狀態，與退款狀態並行（只給後台）。
class PaymentReversalStatus extends $pb.ProtobufEnum {
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_UNSPECIFIED =
      PaymentReversalStatus._(
          0, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_UNSPECIFIED');

  /// 沒有要向賣家收回的款項。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_NONE =
      PaymentReversalStatus._(
          1, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_NONE');

  /// 等退款被金流商接受之後才開始收回。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_WAITING =
      PaymentReversalStatus._(
          2, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_WAITING');

  /// 收回進行中。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_REVERSING =
      PaymentReversalStatus._(
          3, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_REVERSING');

  /// 已向賣家收回。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_REVERSED =
      PaymentReversalStatus._(
          4, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_REVERSED');

  /// 不再收回（退款失敗或轉帳沒有可收回的額度）。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_RELEASED =
      PaymentReversalStatus._(
          5, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_RELEASED');

  /// 與金流商對不起來：待營運人工結案。
  static const PaymentReversalStatus PAYMENT_REVERSAL_STATUS_ANOMALY =
      PaymentReversalStatus._(
          6, _omitEnumNames ? '' : 'PAYMENT_REVERSAL_STATUS_ANOMALY');

  static const $core.List<PaymentReversalStatus> values =
      <PaymentReversalStatus>[
    PAYMENT_REVERSAL_STATUS_UNSPECIFIED,
    PAYMENT_REVERSAL_STATUS_NONE,
    PAYMENT_REVERSAL_STATUS_WAITING,
    PAYMENT_REVERSAL_STATUS_REVERSING,
    PAYMENT_REVERSAL_STATUS_REVERSED,
    PAYMENT_REVERSAL_STATUS_RELEASED,
    PAYMENT_REVERSAL_STATUS_ANOMALY,
  ];

  static final $core.List<PaymentReversalStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static PaymentReversalStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentReversalStatus._(super.value, super.name);
}

/// PaymentTopupStatus 是已從轉帳金額扣掉的退款之後失敗，補差額轉帳給賣家的狀態（只給後台）。
class PaymentTopupStatus extends $pb.ProtobufEnum {
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_UNSPECIFIED =
      PaymentTopupStatus._(
          0, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_UNSPECIFIED');

  /// 沒有要補的差額。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_NONE =
      PaymentTopupStatus._(
          1, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_NONE');

  /// 等補差額轉帳。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_WAITING =
      PaymentTopupStatus._(
          2, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_WAITING');

  /// 補差額轉帳送出中。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_SENDING =
      PaymentTopupStatus._(
          3, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_SENDING');

  /// 已補給賣家。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_DONE =
      PaymentTopupStatus._(
          4, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_DONE');

  /// 不再補（差額已無需補）。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_RELEASED =
      PaymentTopupStatus._(
          5, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_RELEASED');

  /// 與金流商對不起來：待營運人工結案。
  static const PaymentTopupStatus PAYMENT_TOPUP_STATUS_ANOMALY =
      PaymentTopupStatus._(
          6, _omitEnumNames ? '' : 'PAYMENT_TOPUP_STATUS_ANOMALY');

  static const $core.List<PaymentTopupStatus> values = <PaymentTopupStatus>[
    PAYMENT_TOPUP_STATUS_UNSPECIFIED,
    PAYMENT_TOPUP_STATUS_NONE,
    PAYMENT_TOPUP_STATUS_WAITING,
    PAYMENT_TOPUP_STATUS_SENDING,
    PAYMENT_TOPUP_STATUS_DONE,
    PAYMENT_TOPUP_STATUS_RELEASED,
    PAYMENT_TOPUP_STATUS_ANOMALY,
  ];

  static final $core.List<PaymentTopupStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static PaymentTopupStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentTopupStatus._(super.value, super.name);
}

/// PaymentAnomalyKind 是進入 ANOMALY 的物件種類（只給後台）。
class PaymentAnomalyKind extends $pb.ProtobufEnum {
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_UNSPECIFIED =
      PaymentAnomalyKind._(
          0, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_UNSPECIFIED');

  /// 付款：建立結果與金流商對不起來。
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_PAYMENT =
      PaymentAnomalyKind._(
          1, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_PAYMENT');

  /// 轉帳。
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_TRANSFER =
      PaymentAnomalyKind._(
          2, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_TRANSFER');

  /// 退款。
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_REFUND =
      PaymentAnomalyKind._(
          3, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_REFUND');

  /// 退款的收回（撤回轉帳）：id 是撤回列 ID。
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_REVERSAL =
      PaymentAnomalyKind._(
          4, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_REVERSAL');

  /// 退款的補差額轉帳：id 是退款 ID。
  static const PaymentAnomalyKind PAYMENT_ANOMALY_KIND_TOPUP =
      PaymentAnomalyKind._(
          5, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_KIND_TOPUP');

  static const $core.List<PaymentAnomalyKind> values = <PaymentAnomalyKind>[
    PAYMENT_ANOMALY_KIND_UNSPECIFIED,
    PAYMENT_ANOMALY_KIND_PAYMENT,
    PAYMENT_ANOMALY_KIND_TRANSFER,
    PAYMENT_ANOMALY_KIND_REFUND,
    PAYMENT_ANOMALY_KIND_REVERSAL,
    PAYMENT_ANOMALY_KIND_TOPUP,
  ];

  static final $core.List<PaymentAnomalyKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static PaymentAnomalyKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentAnomalyKind._(super.value, super.name);
}

/// PaymentAnomalyOutcome 是人工結案的結果（只給後台）。
class PaymentAnomalyOutcome extends $pb.ProtobufEnum {
  static const PaymentAnomalyOutcome PAYMENT_ANOMALY_OUTCOME_UNSPECIFIED =
      PaymentAnomalyOutcome._(
          0, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_OUTCOME_UNSPECIFIED');

  /// 核對金流商之後確認它確實執行了（要附金流商物件 ID 為證據）。
  static const PaymentAnomalyOutcome PAYMENT_ANOMALY_OUTCOME_SUCCEEDED =
      PaymentAnomalyOutcome._(
          1, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_OUTCOME_SUCCEEDED');

  /// 核對金流商之後確認它沒有執行。
  static const PaymentAnomalyOutcome PAYMENT_ANOMALY_OUTCOME_FAILED =
      PaymentAnomalyOutcome._(
          2, _omitEnumNames ? '' : 'PAYMENT_ANOMALY_OUTCOME_FAILED');

  static const $core.List<PaymentAnomalyOutcome> values =
      <PaymentAnomalyOutcome>[
    PAYMENT_ANOMALY_OUTCOME_UNSPECIFIED,
    PAYMENT_ANOMALY_OUTCOME_SUCCEEDED,
    PAYMENT_ANOMALY_OUTCOME_FAILED,
  ];

  static final $core.List<PaymentAnomalyOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static PaymentAnomalyOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PaymentAnomalyOutcome._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
