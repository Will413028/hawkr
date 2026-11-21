// This is a generated file - do not edit.
//
// Generated from hawkr/moderation/v1alpha1/moderation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ReportTargetKind 是檢舉對象的種類。
class ReportTargetKind extends $pb.ProtobufEnum {
  static const ReportTargetKind REPORT_TARGET_KIND_UNSPECIFIED =
      ReportTargetKind._(
          0, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_UNSPECIFIED');
  static const ReportTargetKind REPORT_TARGET_KIND_POST =
      ReportTargetKind._(1, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_POST');
  static const ReportTargetKind REPORT_TARGET_KIND_COMMENT =
      ReportTargetKind._(2, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_COMMENT');
  static const ReportTargetKind REPORT_TARGET_KIND_FORUM_ENTRY =
      ReportTargetKind._(
          3, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_FORUM_ENTRY');

  /// 買家；id 是買家的 ID。
  static const ReportTargetKind REPORT_TARGET_KIND_BUYER =
      ReportTargetKind._(4, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_BUYER');

  /// 買家寫的評價；id 是評價 ID。
  static const ReportTargetKind REPORT_TARGET_KIND_REVIEW =
      ReportTargetKind._(5, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_REVIEW');

  /// 買家與店家之間的一則聊天訊息；id 是訊息 ID。
  static const ReportTargetKind REPORT_TARGET_KIND_CHAT_MESSAGE =
      ReportTargetKind._(
          6, _omitEnumNames ? '' : 'REPORT_TARGET_KIND_CHAT_MESSAGE');

  static const $core.List<ReportTargetKind> values = <ReportTargetKind>[
    REPORT_TARGET_KIND_UNSPECIFIED,
    REPORT_TARGET_KIND_POST,
    REPORT_TARGET_KIND_COMMENT,
    REPORT_TARGET_KIND_FORUM_ENTRY,
    REPORT_TARGET_KIND_BUYER,
    REPORT_TARGET_KIND_REVIEW,
    REPORT_TARGET_KIND_CHAT_MESSAGE,
  ];

  static final $core.List<ReportTargetKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static ReportTargetKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReportTargetKind._(super.value, super.name);
}

/// ReportOutcome 是一筆檢舉的結果，由所屬案件的決定推導。
class ReportOutcome extends $pb.ProtobufEnum {
  static const ReportOutcome REPORT_OUTCOME_UNSPECIFIED =
      ReportOutcome._(0, _omitEnumNames ? '' : 'REPORT_OUTCOME_UNSPECIFIED');

  /// 案件還在處理中。
  static const ReportOutcome REPORT_OUTCOME_PENDING =
      ReportOutcome._(1, _omitEnumNames ? '' : 'REPORT_OUTCOME_PENDING');

  /// 案件認定違規。
  static const ReportOutcome REPORT_OUTCOME_VIOLATION =
      ReportOutcome._(2, _omitEnumNames ? '' : 'REPORT_OUTCOME_VIOLATION');

  /// 案件認定不違規。
  static const ReportOutcome REPORT_OUTCOME_NO_VIOLATION =
      ReportOutcome._(3, _omitEnumNames ? '' : 'REPORT_OUTCOME_NO_VIOLATION');

  static const $core.List<ReportOutcome> values = <ReportOutcome>[
    REPORT_OUTCOME_UNSPECIFIED,
    REPORT_OUTCOME_PENDING,
    REPORT_OUTCOME_VIOLATION,
    REPORT_OUTCOME_NO_VIOLATION,
  ];

  static final $core.List<ReportOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ReportOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReportOutcome._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
