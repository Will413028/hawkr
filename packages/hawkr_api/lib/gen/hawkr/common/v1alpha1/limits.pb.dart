// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/limits.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Limits 是一個欄位的內容上限。max_items 用在 repeated 欄位（元素個數），max_runes 用在 string 欄位（Unicode code point 數，
/// 不是位元組也不是 UTF-16 單位）。兩者擇一，必須大於 0。
class Limits extends $pb.GeneratedMessage {
  factory Limits({
    $core.int? maxItems,
    $core.int? maxRunes,
  }) {
    final result = Limits._();
    if (maxItems != null) result.maxItems = maxItems;
    if (maxRunes != null) result.maxRunes = maxRunes;
    return result;
  }

  Limits._();

  factory Limits.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Limits()..mergeFromBuffer(data, registry);
  factory Limits.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Limits()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Limits',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: Limits.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'maxItems')
    ..aI(2, _omitFieldNames ? '' : 'maxRunes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Limits clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Limits copyWith(void Function(Limits) updates) =>
      super.copyWith((message) => updates(message as Limits)) as Limits;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Limits() / Limits.new instead')
  static Limits create() => Limits._();
  static $pb.GeneratedMessage $_createMessage() => Limits._();
  @$core.override
  Limits createEmptyInstance() => Limits._();
  @$core.pragma('dart2js:noInline')
  static Limits getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Limits>(Limits.$_createMessage);
  static Limits? _defaultInstance;

  /// 元素個數上限。
  @$pb.TagNumber(1)
  $core.int get maxItems => $_getIZ(0);
  @$pb.TagNumber(1)
  set maxItems($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMaxItems() => $_has(0);
  @$pb.TagNumber(1)
  void clearMaxItems() => $_clearField(1);

  /// code point 數上限。
  @$pb.TagNumber(2)
  $core.int get maxRunes => $_getIZ(1);
  @$pb.TagNumber(2)
  set maxRunes($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMaxRunes() => $_has(1);
  @$pb.TagNumber(2)
  void clearMaxRunes() => $_clearField(2);
}

class LimitsExt {
  static final limits = $pb.Extension<Limits>(
      _omitMessageNames ? '' : 'google.protobuf.FieldOptions',
      _omitFieldNames ? '' : 'limits',
      50002,
      $pb.PbFieldType.OM,
      defaultOrMaker: Limits.getDefault,
      subBuilder: Limits.$_createMessage);
  static void registerAllExtensions($pb.ExtensionRegistry registry) {
    registry.add(limits);
  }
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
