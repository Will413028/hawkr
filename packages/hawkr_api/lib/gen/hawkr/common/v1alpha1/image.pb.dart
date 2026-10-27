// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/image.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Image 是對外 API 裡的一張圖（ADR 2026-09-25-api-image-uri-template）。只出現在回應；
/// 換圖時在各資源的 *_staged_token 帶 media 上傳完成後拿到的 staged token。
class Image extends $pb.GeneratedMessage {
  factory Image({
    $core.String? uriTemplate,
  }) {
    final result = Image._();
    if (uriTemplate != null) result.uriTemplate = uriTemplate;
    return result;
  }

  Image._();

  factory Image.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Image()..mergeFromBuffer(data, registry);
  factory Image.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Image()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Image',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: Image.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'uriTemplate')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Image clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Image copyWith(void Function(Image) updates) =>
      super.copyWith((message) => updates(message as Image)) as Image;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Image() / Image.new instead')
  static Image create() => Image._();
  static $pb.GeneratedMessage $_createMessage() => Image._();
  @$core.override
  Image createEmptyInstance() => Image._();
  @$core.pragma('dart2js:noInline')
  static Image getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Image>(Image.$_createMessage);
  static Image? _defaultInstance;

  /// 取圖的 URI 樣板（RFC 6570 Level 1）：恰好一個變數 {preset}，或沒有變數（外部圖片）。
  /// client 把 {preset} 換成 preset 名稱（image gateway 的白名單）後直接使用，不解析、不改寫其他部分，
  /// 也不自己組網址。
  @$pb.TagNumber(1)
  $core.String get uriTemplate => $_getSZ(0);
  @$pb.TagNumber(1)
  set uriTemplate($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUriTemplate() => $_has(0);
  @$pb.TagNumber(1)
  void clearUriTemplate() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
