// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/delegated_operation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class DelegatedOperation extends $pb.GeneratedMessage {
  factory DelegatedOperation({
    $core.String? requestId,
    $core.String? operationReason,
    $core.bool? notifySeller,
  }) {
    final result = DelegatedOperation._();
    if (requestId != null) result.requestId = requestId;
    if (operationReason != null) result.operationReason = operationReason;
    if (notifySeller != null) result.notifySeller = notifySeller;
    return result;
  }

  DelegatedOperation._();

  factory DelegatedOperation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromBuffer(data, registry);
  factory DelegatedOperation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DelegatedOperation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: DelegatedOperation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'operationReason')
    ..aOB(3, _omitFieldNames ? '' : 'notifySeller')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation copyWith(void Function(DelegatedOperation) updates) =>
      super.copyWith((message) => updates(message as DelegatedOperation))
          as DelegatedOperation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DelegatedOperation() / DelegatedOperation.new instead')
  static DelegatedOperation create() => DelegatedOperation._();
  static $pb.GeneratedMessage $_createMessage() => DelegatedOperation._();
  @$core.override
  DelegatedOperation createEmptyInstance() => DelegatedOperation._();
  @$core.pragma('dart2js:noInline')
  static DelegatedOperation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DelegatedOperation>(
          DelegatedOperation.$_createMessage);
  static DelegatedOperation? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationReason => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationReason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationReason() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get notifySeller => $_getBF(2);
  @$pb.TagNumber(3)
  set notifySeller($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNotifySeller() => $_has(2);
  @$pb.TagNumber(3)
  void clearNotifySeller() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
