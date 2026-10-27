// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/report.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ReportOutcome 是一筆檢舉的結果（step10e-reports-brief R2）。還沒有結果時是 UNSPECIFIED（與 resolve_time 不設同時成立）。
class ReportOutcome extends $pb.ProtobufEnum {
  static const ReportOutcome REPORT_OUTCOME_UNSPECIFIED =
      ReportOutcome._(0, _omitEnumNames ? '' : 'REPORT_OUTCOME_UNSPECIFIED');

  /// 平台採取了措施（只會是案件裡的檢舉）。
  static const ReportOutcome REPORT_OUTCOME_ACTION_TAKEN =
      ReportOutcome._(1, _omitEnumNames ? '' : 'REPORT_OUTCOME_ACTION_TAKEN');

  /// 查過之後沒有違規。
  static const ReportOutcome REPORT_OUTCOME_NO_VIOLATION =
      ReportOutcome._(2, _omitEnumNames ? '' : 'REPORT_OUTCOME_NO_VIOLATION');

  /// 內容不足，無法判斷。
  static const ReportOutcome REPORT_OUTCOME_INSUFFICIENT =
      ReportOutcome._(3, _omitEnumNames ? '' : 'REPORT_OUTCOME_INSUFFICIENT');

  /// 與另一筆檢舉重複。
  static const ReportOutcome REPORT_OUTCOME_DUPLICATE =
      ReportOutcome._(4, _omitEnumNames ? '' : 'REPORT_OUTCOME_DUPLICATE');

  /// 濫用檢舉。
  static const ReportOutcome REPORT_OUTCOME_ABUSIVE =
      ReportOutcome._(5, _omitEnumNames ? '' : 'REPORT_OUTCOME_ABUSIVE');

  static const $core.List<ReportOutcome> values = <ReportOutcome>[
    REPORT_OUTCOME_UNSPECIFIED,
    REPORT_OUTCOME_ACTION_TAKEN,
    REPORT_OUTCOME_NO_VIOLATION,
    REPORT_OUTCOME_INSUFFICIENT,
    REPORT_OUTCOME_DUPLICATE,
    REPORT_OUTCOME_ABUSIVE,
  ];

  static final $core.List<ReportOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static ReportOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReportOutcome._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
