// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/deadline.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Deadline 是一個有期限的義務：從 start_time 起算，到 due_time 到期。兩者都由伺服器給：依審閱政策算出，
/// 或是存下的期限（例如管理員建立詢問時指定的）。client 只拿現在和它們比較（剩多少、是否逾期、
/// 色階），不重建起算規則。
///
/// 資源以 `<義務>_deadline` 欄位輸出它，只在義務還在時設；沒有義務時不設，client 不顯示期限。
class Deadline extends $pb.GeneratedMessage {
  factory Deadline({
    $0.Timestamp? startTime,
    $0.Timestamp? dueTime,
  }) {
    final result = Deadline._();
    if (startTime != null) result.startTime = startTime;
    if (dueTime != null) result.dueTime = dueTime;
    return result;
  }

  Deadline._();

  factory Deadline.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Deadline()..mergeFromBuffer(data, registry);
  factory Deadline.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Deadline()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Deadline',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: Deadline.$_createMessage)
    ..aOM<$0.Timestamp>(1, _omitFieldNames ? '' : 'startTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'dueTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Deadline clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Deadline copyWith(void Function(Deadline) updates) =>
      super.copyWith((message) => updates(message as Deadline)) as Deadline;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Deadline() / Deadline.new instead')
  static Deadline create() => Deadline._();
  static $pb.GeneratedMessage $_createMessage() => Deadline._();
  @$core.override
  Deadline createEmptyInstance() => Deadline._();
  @$core.pragma('dart2js:noInline')
  static Deadline getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Deadline>(Deadline.$_createMessage);
  static Deadline? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Timestamp get startTime => $_getN(0);
  @$pb.TagNumber(1)
  set startTime($0.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStartTime() => $_has(0);
  @$pb.TagNumber(1)
  void clearStartTime() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Timestamp ensureStartTime() => $_ensure(0);

  @$pb.TagNumber(2)
  $0.Timestamp get dueTime => $_getN(1);
  @$pb.TagNumber(2)
  set dueTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDueTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearDueTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureDueTime() => $_ensure(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
