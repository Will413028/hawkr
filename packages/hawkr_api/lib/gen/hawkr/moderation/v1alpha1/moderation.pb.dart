// This is a generated file - do not edit.
//
// Generated from hawkr/moderation/v1alpha1/moderation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

import '../../common/v1alpha1/policy_category.pbenum.dart' as $1;
import 'moderation.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'moderation.pbenum.dart';

/// ReportTarget 是被檢舉的對象。
class ReportTarget extends $pb.GeneratedMessage {
  factory ReportTarget({
    ReportTargetKind? kind,
    $core.String? id,
  }) {
    final result = ReportTarget._();
    if (kind != null) result.kind = kind;
    if (id != null) result.id = id;
    return result;
  }

  ReportTarget._();

  factory ReportTarget.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportTarget()..mergeFromBuffer(data, registry);
  factory ReportTarget.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportTarget()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportTarget',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ReportTarget.$_createMessage)
    ..aE<ReportTargetKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: ReportTargetKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportTarget clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportTarget copyWith(void Function(ReportTarget) updates) =>
      super.copyWith((message) => updates(message as ReportTarget))
          as ReportTarget;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReportTarget() / ReportTarget.new instead')
  static ReportTarget create() => ReportTarget._();
  static $pb.GeneratedMessage $_createMessage() => ReportTarget._();
  @$core.override
  ReportTarget createEmptyInstance() => ReportTarget._();
  @$core.pragma('dart2js:noInline')
  static ReportTarget getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReportTarget>(
          ReportTarget.$_createMessage);
  static ReportTarget? _defaultInstance;

  @$pb.TagNumber(1)
  ReportTargetKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(ReportTargetKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get id => $_getSZ(1);
  @$pb.TagNumber(2)
  set id($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasId() => $_has(1);
  @$pb.TagNumber(2)
  void clearId() => $_clearField(2);
}

class SubmitReportRequest extends $pb.GeneratedMessage {
  factory SubmitReportRequest({
    ReportTarget? target,
    $1.PolicyCategory? category,
    $core.String? description,
    $core.String? requestId,
  }) {
    final result = SubmitReportRequest._();
    if (target != null) result.target = target;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  SubmitReportRequest._();

  factory SubmitReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitReportRequest()..mergeFromBuffer(data, registry);
  factory SubmitReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitReportRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitReportRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: SubmitReportRequest.$_createMessage)
    ..aOM<ReportTarget>(1, _omitFieldNames ? '' : 'target',
        subBuilder: ReportTarget.$_createMessage)
    ..aE<$1.PolicyCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: $1.PolicyCategory.values)
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitReportRequest copyWith(void Function(SubmitReportRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitReportRequest))
          as SubmitReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SubmitReportRequest() / SubmitReportRequest.new instead')
  static SubmitReportRequest create() => SubmitReportRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitReportRequest._();
  @$core.override
  SubmitReportRequest createEmptyInstance() => SubmitReportRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitReportRequest>(
          SubmitReportRequest.$_createMessage);
  static SubmitReportRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ReportTarget get target => $_getN(0);
  @$pb.TagNumber(1)
  set target(ReportTarget value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTarget() => $_has(0);
  @$pb.TagNumber(1)
  void clearTarget() => $_clearField(1);
  @$pb.TagNumber(1)
  ReportTarget ensureTarget() => $_ensure(0);

  /// 檢舉原因：SPAM、HARASSMENT、HATE、SEXUAL_CONTENT、MISLEADING_INFO、FOOD_SAFETY、FRAUD、OTHER 其中之一。
  @$pb.TagNumber(2)
  $1.PolicyCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category($1.PolicyCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  /// 選填的說明，去除首尾空白後最多 500 個字元（以 Unicode 字元計）。
  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  /// 選填，UUIDv4（AIP-155）：同一位檢舉人以同一個 request_id 重送回第一次的結果；內容不同回 invalid_argument。
  @$pb.TagNumber(4)
  $core.String get requestId => $_getSZ(3);
  @$pb.TagNumber(4)
  set requestId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRequestId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRequestId() => $_clearField(4);
}

class SubmitReportResponse extends $pb.GeneratedMessage {
  factory SubmitReportResponse({
    $core.String? reportId,
    $0.Timestamp? createTime,
  }) {
    final result = SubmitReportResponse._();
    if (reportId != null) result.reportId = reportId;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  SubmitReportResponse._();

  factory SubmitReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitReportResponse()..mergeFromBuffer(data, registry);
  factory SubmitReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitReportResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitReportResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: SubmitReportResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitReportResponse copyWith(void Function(SubmitReportResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitReportResponse))
          as SubmitReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitReportResponse() / SubmitReportResponse.new instead')
  static SubmitReportResponse create() => SubmitReportResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubmitReportResponse._();
  @$core.override
  SubmitReportResponse createEmptyInstance() => SubmitReportResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitReportResponse>(
          SubmitReportResponse.$_createMessage);
  static SubmitReportResponse? _defaultInstance;

  /// 檢舉 ID。
  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);

  /// 收件時間；重送回第一次的時間。
  @$pb.TagNumber(2)
  $0.Timestamp get createTime => $_getN(1);
  @$pb.TagNumber(2)
  set createTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureCreateTime() => $_ensure(1);
}

class ListMyReportsRequest extends $pb.GeneratedMessage {
  factory ListMyReportsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMyReportsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMyReportsRequest._();

  factory ListMyReportsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReportsRequest()..mergeFromBuffer(data, registry);
  factory ListMyReportsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReportsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyReportsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ListMyReportsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReportsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReportsRequest copyWith(void Function(ListMyReportsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyReportsRequest))
          as ListMyReportsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyReportsRequest() / ListMyReportsRequest.new instead')
  static ListMyReportsRequest create() => ListMyReportsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyReportsRequest._();
  @$core.override
  ListMyReportsRequest createEmptyInstance() => ListMyReportsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyReportsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyReportsRequest>(
          ListMyReportsRequest.$_createMessage);
  static ListMyReportsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListMyReportsResponse extends $pb.GeneratedMessage {
  factory ListMyReportsResponse({
    $core.Iterable<ReportRecord>? reports,
    $core.String? nextPageToken,
  }) {
    final result = ListMyReportsResponse._();
    if (reports != null) result.reports.addAll(reports);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyReportsResponse._();

  factory ListMyReportsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReportsResponse()..mergeFromBuffer(data, registry);
  factory ListMyReportsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReportsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyReportsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ListMyReportsResponse.$_createMessage)
    ..pPM<ReportRecord>(1, _omitFieldNames ? '' : 'reports',
        subBuilder: ReportRecord.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReportsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReportsResponse copyWith(
          void Function(ListMyReportsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyReportsResponse))
          as ListMyReportsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyReportsResponse() / ListMyReportsResponse.new instead')
  static ListMyReportsResponse create() => ListMyReportsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyReportsResponse._();
  @$core.override
  ListMyReportsResponse createEmptyInstance() => ListMyReportsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyReportsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyReportsResponse>(
          ListMyReportsResponse.$_createMessage);
  static ListMyReportsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ReportRecord> get reports => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

/// ReportRecord 是呼叫者送出的一筆檢舉。
class ReportRecord extends $pb.GeneratedMessage {
  factory ReportRecord({
    $core.String? id,
    ReportTarget? target,
    $1.PolicyCategory? category,
    $core.String? description,
    ReportOutcome? outcome,
    $0.Timestamp? createTime,
    $0.Timestamp? resolveTime,
  }) {
    final result = ReportRecord._();
    if (id != null) result.id = id;
    if (target != null) result.target = target;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (outcome != null) result.outcome = outcome;
    if (createTime != null) result.createTime = createTime;
    if (resolveTime != null) result.resolveTime = resolveTime;
    return result;
  }

  ReportRecord._();

  factory ReportRecord.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportRecord()..mergeFromBuffer(data, registry);
  factory ReportRecord.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportRecord()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportRecord',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ReportRecord.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<ReportTarget>(2, _omitFieldNames ? '' : 'target',
        subBuilder: ReportTarget.$_createMessage)
    ..aE<$1.PolicyCategory>(3, _omitFieldNames ? '' : 'category',
        enumValues: $1.PolicyCategory.values)
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aE<ReportOutcome>(5, _omitFieldNames ? '' : 'outcome',
        enumValues: ReportOutcome.values)
    ..aOM<$0.Timestamp>(6, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(7, _omitFieldNames ? '' : 'resolveTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportRecord clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportRecord copyWith(void Function(ReportRecord) updates) =>
      super.copyWith((message) => updates(message as ReportRecord))
          as ReportRecord;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReportRecord() / ReportRecord.new instead')
  static ReportRecord create() => ReportRecord._();
  static $pb.GeneratedMessage $_createMessage() => ReportRecord._();
  @$core.override
  ReportRecord createEmptyInstance() => ReportRecord._();
  @$core.pragma('dart2js:noInline')
  static ReportRecord getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReportRecord>(
          ReportRecord.$_createMessage);
  static ReportRecord? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  ReportTarget get target => $_getN(1);
  @$pb.TagNumber(2)
  set target(ReportTarget value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTarget() => $_has(1);
  @$pb.TagNumber(2)
  void clearTarget() => $_clearField(2);
  @$pb.TagNumber(2)
  ReportTarget ensureTarget() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.PolicyCategory get category => $_getN(2);
  @$pb.TagNumber(3)
  set category($1.PolicyCategory value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCategory() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategory() => $_clearField(3);

  /// 送出時的說明；沒有或已超過保留期被清除時是空字串。
  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  ReportOutcome get outcome => $_getN(4);
  @$pb.TagNumber(5)
  set outcome(ReportOutcome value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOutcome() => $_has(4);
  @$pb.TagNumber(5)
  void clearOutcome() => $_clearField(5);

  @$pb.TagNumber(6)
  $0.Timestamp get createTime => $_getN(5);
  @$pb.TagNumber(6)
  set createTime($0.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCreateTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreateTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $0.Timestamp ensureCreateTime() => $_ensure(5);

  /// 案件結案的時間；處理中不設。
  @$pb.TagNumber(7)
  $0.Timestamp get resolveTime => $_getN(6);
  @$pb.TagNumber(7)
  set resolveTime($0.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasResolveTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearResolveTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Timestamp ensureResolveTime() => $_ensure(6);
}

/// ModerationAppService 是登入的買家檢舉與讀自己的檢舉；店家身分不能呼叫（permission_denied）。
class ModerationAppServiceApi {
  final $pb.RpcClient _client;

  ModerationAppServiceApi(this._client);

  /// SubmitReport 送出一筆檢舉。對象不存在、已刪除或不是公開可讀回 not_found；檢舉自己的內容或自己回 invalid_argument；每位檢舉人每個台北日最多新增 20 筆，
  /// 超過回 resource_exhausted。同一位檢舉人對同一個對象在案件進行中時再送，回原本那一筆（不吃額度）；案件結案後再送是新的檢舉。
  /// 帶 request_id 時，同一個 request_id 重送一律回第一次的結果，不看案件現在的狀態；內容不同回 invalid_argument。
  $async.Future<SubmitReportResponse> submitReport(
          $pb.ClientContext? ctx, SubmitReportRequest request) =>
      _client.invoke<SubmitReportResponse>(ctx, 'ModerationAppService',
          'SubmitReport', request, SubmitReportResponse());

  /// ListMyReports 列出呼叫者自己送出的檢舉，新到舊。每筆帶對象、原因、說明與結果（處理中、違規、不違規）。停權的帳號也可以呼叫。
  $async.Future<ListMyReportsResponse> listMyReports(
          $pb.ClientContext? ctx, ListMyReportsRequest request) =>
      _client.invoke<ListMyReportsResponse>(ctx, 'ModerationAppService',
          'ListMyReports', request, ListMyReportsResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
