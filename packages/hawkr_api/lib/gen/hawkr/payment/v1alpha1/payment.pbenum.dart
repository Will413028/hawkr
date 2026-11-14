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

/// OnboardingSurface 是開通用的介面。
class OnboardingSurface extends $pb.ProtobufEnum {
  static const OnboardingSurface ONBOARDING_SURFACE_UNSPECIFIED =
      OnboardingSurface._(
          0, _omitEnumNames ? '' : 'ONBOARDING_SURFACE_UNSPECIFIED');

  /// 金流商託管的頁面（連結）。
  static const OnboardingSurface ONBOARDING_SURFACE_HOSTED =
      OnboardingSurface._(1, _omitEnumNames ? '' : 'ONBOARDING_SURFACE_HOSTED');

  /// 嵌在網頁上的元件（client secret）。
  static const OnboardingSurface ONBOARDING_SURFACE_EMBEDDED =
      OnboardingSurface._(
          2, _omitEnumNames ? '' : 'ONBOARDING_SURFACE_EMBEDDED');

  static const $core.List<OnboardingSurface> values = <OnboardingSurface>[
    ONBOARDING_SURFACE_UNSPECIFIED,
    ONBOARDING_SURFACE_HOSTED,
    ONBOARDING_SURFACE_EMBEDDED,
  ];

  static final $core.List<OnboardingSurface?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static OnboardingSurface? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const OnboardingSurface._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
