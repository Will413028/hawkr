// This is a generated file - do not edit.
//
// Generated from hawkr/buyer/v1alpha1/buyer.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// BuyerStatus 是買家的狀態。
class BuyerStatus extends $pb.ProtobufEnum {
  static const BuyerStatus BUYER_STATUS_UNSPECIFIED =
      BuyerStatus._(0, _omitEnumNames ? '' : 'BUYER_STATUS_UNSPECIFIED');
  static const BuyerStatus BUYER_STATUS_ACTIVE =
      BuyerStatus._(1, _omitEnumNames ? '' : 'BUYER_STATUS_ACTIVE');

  /// 申請刪除帳號，等待未完成的訂單結束。
  static const BuyerStatus BUYER_STATUS_PENDING_DELETION =
      BuyerStatus._(2, _omitEnumNames ? '' : 'BUYER_STATUS_PENDING_DELETION');

  static const $core.List<BuyerStatus> values = <BuyerStatus>[
    BUYER_STATUS_UNSPECIFIED,
    BUYER_STATUS_ACTIVE,
    BUYER_STATUS_PENDING_DELETION,
  ];

  static final $core.List<BuyerStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static BuyerStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const BuyerStatus._(super.value, super.name);
}

/// Gender 是買家自填的性別。
class Gender extends $pb.ProtobufEnum {
  static const Gender GENDER_UNSPECIFIED =
      Gender._(0, _omitEnumNames ? '' : 'GENDER_UNSPECIFIED');
  static const Gender GENDER_MALE =
      Gender._(1, _omitEnumNames ? '' : 'GENDER_MALE');
  static const Gender GENDER_FEMALE =
      Gender._(2, _omitEnumNames ? '' : 'GENDER_FEMALE');
  static const Gender GENDER_OTHER =
      Gender._(3, _omitEnumNames ? '' : 'GENDER_OTHER');

  static const $core.List<Gender> values = <Gender>[
    GENDER_UNSPECIFIED,
    GENDER_MALE,
    GENDER_FEMALE,
    GENDER_OTHER,
  ];

  static final $core.List<Gender?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static Gender? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const Gender._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
