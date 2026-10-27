// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/postal_address.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// PostalAddress 是台灣的地址：內政部的 8 碼鄉鎮市區代碼加上其後的明細（ADR 2026-10-04-address-town-code）。
/// 縣市是代碼的前 5 碼，郵遞區號與中文名稱由 client 以同一份代碼表推導，API 不回傳。兩個欄位一起修改，所以 update_mask 的
/// 路徑是持有它的欄位（例如 address），整個換掉。
class PostalAddress extends $pb.GeneratedMessage {
  factory PostalAddress({
    $core.String? townCode,
    $core.String? addressDetail,
  }) {
    final result = PostalAddress._();
    if (townCode != null) result.townCode = townCode;
    if (addressDetail != null) result.addressDetail = addressDetail;
    return result;
  }

  PostalAddress._();

  factory PostalAddress.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostalAddress()..mergeFromBuffer(data, registry);
  factory PostalAddress.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostalAddress()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PostalAddress',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: PostalAddress.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'townCode')
    ..aOS(2, _omitFieldNames ? '' : 'addressDetail')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostalAddress clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostalAddress copyWith(void Function(PostalAddress) updates) =>
      super.copyWith((message) => updates(message as PostalAddress))
          as PostalAddress;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PostalAddress() / PostalAddress.new instead')
  static PostalAddress create() => PostalAddress._();
  static $pb.GeneratedMessage $_createMessage() => PostalAddress._();
  @$core.override
  PostalAddress createEmptyInstance() => PostalAddress._();
  @$core.pragma('dart2js:noInline')
  static PostalAddress getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PostalAddress>(
          PostalAddress.$_createMessage);
  static PostalAddress? _defaultInstance;

  /// 內政部現行的 8 碼鄉鎮市區代碼；不是現行的代碼回 invalid_argument。
  @$pb.TagNumber(1)
  $core.String get townCode => $_getSZ(0);
  @$pb.TagNumber(1)
  set townCode($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTownCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearTownCode() => $_clearField(1);

  /// 縣市與鄉鎮市區之後的地址，最多 255 字。收貨地址一定要有明細（至少 1 字）；店家可以只填鄉鎮市區，沒有明細時是空字串。
  @$pb.TagNumber(2)
  $core.String get addressDetail => $_getSZ(1);
  @$pb.TagNumber(2)
  set addressDetail($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAddressDetail() => $_has(1);
  @$pb.TagNumber(2)
  void clearAddressDetail() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
