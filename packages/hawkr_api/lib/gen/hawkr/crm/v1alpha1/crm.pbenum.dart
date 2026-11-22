// This is a generated file - do not edit.
//
// Generated from hawkr/crm/v1alpha1/crm.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// CustomerType 是店家對買家的分類。
class CustomerType extends $pb.ProtobufEnum {
  static const CustomerType CUSTOMER_TYPE_UNSPECIFIED =
      CustomerType._(0, _omitEnumNames ? '' : 'CUSTOMER_TYPE_UNSPECIFIED');
  static const CustomerType CUSTOMER_TYPE_NORMAL =
      CustomerType._(1, _omitEnumNames ? '' : 'CUSTOMER_TYPE_NORMAL');
  static const CustomerType CUSTOMER_TYPE_VIP =
      CustomerType._(2, _omitEnumNames ? '' : 'CUSTOMER_TYPE_VIP');

  static const $core.List<CustomerType> values = <CustomerType>[
    CUSTOMER_TYPE_UNSPECIFIED,
    CUSTOMER_TYPE_NORMAL,
    CUSTOMER_TYPE_VIP,
  ];

  static final $core.List<CustomerType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static CustomerType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CustomerType._(super.value, super.name);
}

/// CustomerSort 是客戶列表的排序欄位。
class CustomerSort extends $pb.ProtobufEnum {
  /// 同 CUSTOMER_SORT_FIRST_ORDER_TIME。
  static const CustomerSort CUSTOMER_SORT_UNSPECIFIED =
      CustomerSort._(0, _omitEnumNames ? '' : 'CUSTOMER_SORT_UNSPECIFIED');

  /// 首筆完成訂單的時間。
  static const CustomerSort CUSTOMER_SORT_FIRST_ORDER_TIME =
      CustomerSort._(1, _omitEnumNames ? '' : 'CUSTOMER_SORT_FIRST_ORDER_TIME');
  static const CustomerSort CUSTOMER_SORT_TOTAL_SPENT =
      CustomerSort._(2, _omitEnumNames ? '' : 'CUSTOMER_SORT_TOTAL_SPENT');
  static const CustomerSort CUSTOMER_SORT_ORDER_COUNT =
      CustomerSort._(3, _omitEnumNames ? '' : 'CUSTOMER_SORT_ORDER_COUNT');

  static const $core.List<CustomerSort> values = <CustomerSort>[
    CUSTOMER_SORT_UNSPECIFIED,
    CUSTOMER_SORT_FIRST_ORDER_TIME,
    CUSTOMER_SORT_TOTAL_SPENT,
    CUSTOMER_SORT_ORDER_COUNT,
  ];

  static final $core.List<CustomerSort?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static CustomerSort? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CustomerSort._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
