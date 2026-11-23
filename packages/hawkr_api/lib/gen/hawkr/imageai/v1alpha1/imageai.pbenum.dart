// This is a generated file - do not edit.
//
// Generated from hawkr/imageai/v1alpha1/imageai.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// AiStyleCategory 是風格適用的用途。
class AiStyleCategory extends $pb.ProtobufEnum {
  static const AiStyleCategory AI_STYLE_CATEGORY_UNSPECIFIED =
      AiStyleCategory._(
          0, _omitEnumNames ? '' : 'AI_STYLE_CATEGORY_UNSPECIFIED');

  /// 商品圖。
  static const AiStyleCategory AI_STYLE_CATEGORY_PRODUCT =
      AiStyleCategory._(1, _omitEnumNames ? '' : 'AI_STYLE_CATEGORY_PRODUCT');

  /// 貼文圖。
  static const AiStyleCategory AI_STYLE_CATEGORY_POST =
      AiStyleCategory._(2, _omitEnumNames ? '' : 'AI_STYLE_CATEGORY_POST');

  static const $core.List<AiStyleCategory> values = <AiStyleCategory>[
    AI_STYLE_CATEGORY_UNSPECIFIED,
    AI_STYLE_CATEGORY_PRODUCT,
    AI_STYLE_CATEGORY_POST,
  ];

  static final $core.List<AiStyleCategory?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static AiStyleCategory? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AiStyleCategory._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
