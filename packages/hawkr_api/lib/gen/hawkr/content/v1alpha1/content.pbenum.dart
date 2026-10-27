// This is a generated file - do not edit.
//
// Generated from hawkr/content/v1alpha1/content.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// AppType 是 App 的類型。
class AppType extends $pb.ProtobufEnum {
  static const AppType APP_TYPE_UNSPECIFIED =
      AppType._(0, _omitEnumNames ? '' : 'APP_TYPE_UNSPECIFIED');

  /// 買家 App。
  static const AppType APP_TYPE_BUYER =
      AppType._(1, _omitEnumNames ? '' : 'APP_TYPE_BUYER');

  /// 賣家 App。
  static const AppType APP_TYPE_SELLER =
      AppType._(2, _omitEnumNames ? '' : 'APP_TYPE_SELLER');

  static const $core.List<AppType> values = <AppType>[
    APP_TYPE_UNSPECIFIED,
    APP_TYPE_BUYER,
    APP_TYPE_SELLER,
  ];

  static final $core.List<AppType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static AppType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AppType._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
