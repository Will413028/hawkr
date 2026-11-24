// This is a generated file - do not edit.
//
// Generated from hawkr/coupon/v1alpha1/coupon.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// DiscountType 是折抵方式。
class DiscountType extends $pb.ProtobufEnum {
  static const DiscountType DISCOUNT_TYPE_UNSPECIFIED =
      DiscountType._(0, _omitEnumNames ? '' : 'DISCOUNT_TYPE_UNSPECIFIED');

  /// 現金折抵固定金額。
  static const DiscountType DISCOUNT_TYPE_CASH =
      DiscountType._(1, _omitEnumNames ? '' : 'DISCOUNT_TYPE_CASH');

  /// 百分比折抵，有折抵上限。
  static const DiscountType DISCOUNT_TYPE_PERCENT =
      DiscountType._(2, _omitEnumNames ? '' : 'DISCOUNT_TYPE_PERCENT');

  static const $core.List<DiscountType> values = <DiscountType>[
    DISCOUNT_TYPE_UNSPECIFIED,
    DISCOUNT_TYPE_CASH,
    DISCOUNT_TYPE_PERCENT,
  ];

  static final $core.List<DiscountType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static DiscountType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const DiscountType._(super.value, super.name);
}

/// TargetType 是活動的資格。
class TargetType extends $pb.ProtobufEnum {
  static const TargetType TARGET_TYPE_UNSPECIFIED =
      TargetType._(0, _omitEnumNames ? '' : 'TARGET_TYPE_UNSPECIFIED');

  /// 每一位買家。
  static const TargetType TARGET_TYPE_ALL =
      TargetType._(1, _omitEnumNames ? '' : 'TARGET_TYPE_ALL');

  /// 在該店有消費紀錄的買家。
  static const TargetType TARGET_TYPE_MEMBER =
      TargetType._(2, _omitEnumNames ? '' : 'TARGET_TYPE_MEMBER');

  /// 名單內的買家。
  static const TargetType TARGET_TYPE_DESIGNATED =
      TargetType._(3, _omitEnumNames ? '' : 'TARGET_TYPE_DESIGNATED');

  /// 售後或營運補償：只有收補償的那位買家，由系統建立。
  static const TargetType TARGET_TYPE_COMPENSATION =
      TargetType._(4, _omitEnumNames ? '' : 'TARGET_TYPE_COMPENSATION');

  static const $core.List<TargetType> values = <TargetType>[
    TARGET_TYPE_UNSPECIFIED,
    TARGET_TYPE_ALL,
    TARGET_TYPE_MEMBER,
    TARGET_TYPE_DESIGNATED,
    TARGET_TYPE_COMPENSATION,
  ];

  static final $core.List<TargetType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static TargetType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const TargetType._(super.value, super.name);
}

/// CampaignStatus 是活動的狀態。到期不是狀態：看 Campaign 的 end_time。
class CampaignStatus extends $pb.ProtobufEnum {
  static const CampaignStatus CAMPAIGN_STATUS_UNSPECIFIED =
      CampaignStatus._(0, _omitEnumNames ? '' : 'CAMPAIGN_STATUS_UNSPECIFIED');
  static const CampaignStatus CAMPAIGN_STATUS_DRAFT =
      CampaignStatus._(1, _omitEnumNames ? '' : 'CAMPAIGN_STATUS_DRAFT');
  static const CampaignStatus CAMPAIGN_STATUS_ACTIVE =
      CampaignStatus._(2, _omitEnumNames ? '' : 'CAMPAIGN_STATUS_ACTIVE');

  /// 提前結束。
  static const CampaignStatus CAMPAIGN_STATUS_ENDED =
      CampaignStatus._(3, _omitEnumNames ? '' : 'CAMPAIGN_STATUS_ENDED');

  static const $core.List<CampaignStatus> values = <CampaignStatus>[
    CAMPAIGN_STATUS_UNSPECIFIED,
    CAMPAIGN_STATUS_DRAFT,
    CAMPAIGN_STATUS_ACTIVE,
    CAMPAIGN_STATUS_ENDED,
  ];

  static final $core.List<CampaignStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static CampaignStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CampaignStatus._(super.value, super.name);
}

/// VoucherStatus 是券的狀態。到期的券仍是 AVAILABLE，以 Voucher.expired 呈現。
class VoucherStatus extends $pb.ProtobufEnum {
  static const VoucherStatus VOUCHER_STATUS_UNSPECIFIED =
      VoucherStatus._(0, _omitEnumNames ? '' : 'VOUCHER_STATUS_UNSPECIFIED');

  /// 可使用。
  static const VoucherStatus VOUCHER_STATUS_AVAILABLE =
      VoucherStatus._(1, _omitEnumNames ? '' : 'VOUCHER_STATUS_AVAILABLE');

  /// 已被一筆訂單鎖定。
  static const VoucherStatus VOUCHER_STATUS_LOCKED =
      VoucherStatus._(2, _omitEnumNames ? '' : 'VOUCHER_STATUS_LOCKED');

  /// 已使用（訂單核銷）。
  static const VoucherStatus VOUCHER_STATUS_REDEEMED =
      VoucherStatus._(3, _omitEnumNames ? '' : 'VOUCHER_STATUS_REDEEMED');

  /// 已作廢（活動被結束或營運作廢）。
  static const VoucherStatus VOUCHER_STATUS_VOIDED =
      VoucherStatus._(4, _omitEnumNames ? '' : 'VOUCHER_STATUS_VOIDED');

  static const $core.List<VoucherStatus> values = <VoucherStatus>[
    VOUCHER_STATUS_UNSPECIFIED,
    VOUCHER_STATUS_AVAILABLE,
    VOUCHER_STATUS_LOCKED,
    VOUCHER_STATUS_REDEEMED,
    VOUCHER_STATUS_VOIDED,
  ];

  static final $core.List<VoucherStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static VoucherStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VoucherStatus._(super.value, super.name);
}

/// VoucherOrigin 是券的來源。
class VoucherOrigin extends $pb.ProtobufEnum {
  static const VoucherOrigin VOUCHER_ORIGIN_UNSPECIFIED =
      VoucherOrigin._(0, _omitEnumNames ? '' : 'VOUCHER_ORIGIN_UNSPECIFIED');

  /// 買家自己領取。
  static const VoucherOrigin VOUCHER_ORIGIN_CLAIM =
      VoucherOrigin._(1, _omitEnumNames ? '' : 'VOUCHER_ORIGIN_CLAIM');

  /// 指定發放。
  static const VoucherOrigin VOUCHER_ORIGIN_GRANT =
      VoucherOrigin._(2, _omitEnumNames ? '' : 'VOUCHER_ORIGIN_GRANT');

  /// 補償。
  static const VoucherOrigin VOUCHER_ORIGIN_COMPENSATION =
      VoucherOrigin._(3, _omitEnumNames ? '' : 'VOUCHER_ORIGIN_COMPENSATION');

  static const $core.List<VoucherOrigin> values = <VoucherOrigin>[
    VOUCHER_ORIGIN_UNSPECIFIED,
    VOUCHER_ORIGIN_CLAIM,
    VOUCHER_ORIGIN_GRANT,
    VOUCHER_ORIGIN_COMPENSATION,
  ];

  static final $core.List<VoucherOrigin?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static VoucherOrigin? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VoucherOrigin._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
