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

import '../../common/v1alpha1/policy_category.pbenum.dart' as $2;
import '../../seller/v1alpha1/report.pb.dart' as $1;
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
    $2.PolicyCategory? category,
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
    ..aE<$2.PolicyCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: $2.PolicyCategory.values)
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

  /// 檢舉原因：只接受 accepted_policy_categories 列出的值，其他值（含 UNSPECIFIED）回 invalid_argument。
  @$pb.TagNumber(2)
  $2.PolicyCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category($2.PolicyCategory value) => $_setField(2, value);
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
    $core.Iterable<MyReportItem>? reports,
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
    ..pPM<MyReportItem>(1, _omitFieldNames ? '' : 'reports',
        subBuilder: MyReportItem.$_createMessage)
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
  $pb.PbList<MyReportItem> get reports => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListMyStoreReportsRequest extends $pb.GeneratedMessage {
  factory ListMyStoreReportsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMyStoreReportsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMyStoreReportsRequest._();

  factory ListMyStoreReportsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyStoreReportsRequest()..mergeFromBuffer(data, registry);
  factory ListMyStoreReportsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyStoreReportsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyStoreReportsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ListMyStoreReportsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyStoreReportsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyStoreReportsRequest copyWith(
          void Function(ListMyStoreReportsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyStoreReportsRequest))
          as ListMyStoreReportsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyStoreReportsRequest() / ListMyStoreReportsRequest.new instead')
  static ListMyStoreReportsRequest create() => ListMyStoreReportsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListMyStoreReportsRequest._();
  @$core.override
  ListMyStoreReportsRequest createEmptyInstance() =>
      ListMyStoreReportsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyStoreReportsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyStoreReportsRequest>(
          ListMyStoreReportsRequest.$_createMessage);
  static ListMyStoreReportsRequest? _defaultInstance;

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

class ListMyStoreReportsResponse extends $pb.GeneratedMessage {
  factory ListMyStoreReportsResponse({
    $core.Iterable<MyReportItem>? reports,
    $core.String? nextPageToken,
  }) {
    final result = ListMyStoreReportsResponse._();
    if (reports != null) result.reports.addAll(reports);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyStoreReportsResponse._();

  factory ListMyStoreReportsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyStoreReportsResponse()..mergeFromBuffer(data, registry);
  factory ListMyStoreReportsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyStoreReportsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyStoreReportsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: ListMyStoreReportsResponse.$_createMessage)
    ..pPM<MyReportItem>(1, _omitFieldNames ? '' : 'reports',
        subBuilder: MyReportItem.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyStoreReportsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyStoreReportsResponse copyWith(
          void Function(ListMyStoreReportsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListMyStoreReportsResponse))
          as ListMyStoreReportsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyStoreReportsResponse() / ListMyStoreReportsResponse.new instead')
  static ListMyStoreReportsResponse create() => ListMyStoreReportsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListMyStoreReportsResponse._();
  @$core.override
  ListMyStoreReportsResponse createEmptyInstance() =>
      ListMyStoreReportsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyStoreReportsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyStoreReportsResponse>(
          ListMyStoreReportsResponse.$_createMessage);
  static ListMyStoreReportsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<MyReportItem> get reports => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

enum MyReportItem_Subject { community, store, notSet }

/// MyReportItem 是呼叫者送出的一筆檢舉：共用欄位加依種類的 subject。
class MyReportItem extends $pb.GeneratedMessage {
  factory MyReportItem({
    $core.String? id,
    $2.PolicyCategory? category,
    $core.String? description,
    $0.Timestamp? createTime,
    $0.Timestamp? resolveTime,
    CommunityReport? community,
    $1.StoreReport? store,
  }) {
    final result = MyReportItem._();
    if (id != null) result.id = id;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (createTime != null) result.createTime = createTime;
    if (resolveTime != null) result.resolveTime = resolveTime;
    if (community != null) result.community = community;
    if (store != null) result.store = store;
    return result;
  }

  MyReportItem._();

  factory MyReportItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyReportItem()..mergeFromBuffer(data, registry);
  factory MyReportItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyReportItem()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MyReportItem_Subject>
      _MyReportItem_SubjectByTag = {
    6: MyReportItem_Subject.community,
    7: MyReportItem_Subject.store,
    0: MyReportItem_Subject.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MyReportItem',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: MyReportItem.$_createMessage)
    ..oo(0, [6, 7])
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<$2.PolicyCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: $2.PolicyCategory.values)
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(5, _omitFieldNames ? '' : 'resolveTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<CommunityReport>(6, _omitFieldNames ? '' : 'community',
        subBuilder: CommunityReport.$_createMessage)
    ..aOM<$1.StoreReport>(7, _omitFieldNames ? '' : 'store',
        subBuilder: $1.StoreReport.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyReportItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyReportItem copyWith(void Function(MyReportItem) updates) =>
      super.copyWith((message) => updates(message as MyReportItem))
          as MyReportItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MyReportItem() / MyReportItem.new instead')
  static MyReportItem create() => MyReportItem._();
  static $pb.GeneratedMessage $_createMessage() => MyReportItem._();
  @$core.override
  MyReportItem createEmptyInstance() => MyReportItem._();
  @$core.pragma('dart2js:noInline')
  static MyReportItem getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MyReportItem>(
          MyReportItem.$_createMessage);
  static MyReportItem? _defaultInstance;

  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  MyReportItem_Subject whichSubject() =>
      _MyReportItem_SubjectByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  void clearSubject() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.PolicyCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category($2.PolicyCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  /// 送出時的說明；沒有或已超過保留期被清除時是空字串。
  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $0.Timestamp get createTime => $_getN(3);
  @$pb.TagNumber(4)
  set createTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCreateTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreateTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensureCreateTime() => $_ensure(3);

  /// 結果設定或案件結案的時間；處理中不設。
  @$pb.TagNumber(5)
  $0.Timestamp get resolveTime => $_getN(4);
  @$pb.TagNumber(5)
  set resolveTime($0.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasResolveTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearResolveTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Timestamp ensureResolveTime() => $_ensure(4);

  @$pb.TagNumber(6)
  CommunityReport get community => $_getN(5);
  @$pb.TagNumber(6)
  set community(CommunityReport value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCommunity() => $_has(5);
  @$pb.TagNumber(6)
  void clearCommunity() => $_clearField(6);
  @$pb.TagNumber(6)
  CommunityReport ensureCommunity() => $_ensure(5);

  @$pb.TagNumber(7)
  $1.StoreReport get store => $_getN(6);
  @$pb.TagNumber(7)
  set store($1.StoreReport value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasStore() => $_has(6);
  @$pb.TagNumber(7)
  void clearStore() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.StoreReport ensureStore() => $_ensure(6);
}

/// CommunityReport 是社群內容或買家的檢舉：對象與由案件的決定推導的結果。
class CommunityReport extends $pb.GeneratedMessage {
  factory CommunityReport({
    ReportTarget? target,
    ReportOutcome? outcome,
  }) {
    final result = CommunityReport._();
    if (target != null) result.target = target;
    if (outcome != null) result.outcome = outcome;
    return result;
  }

  CommunityReport._();

  factory CommunityReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CommunityReport()..mergeFromBuffer(data, registry);
  factory CommunityReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CommunityReport()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CommunityReport',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.moderation.v1alpha1'),
      createEmptyInstance: CommunityReport.$_createMessage)
    ..aOM<ReportTarget>(1, _omitFieldNames ? '' : 'target',
        subBuilder: ReportTarget.$_createMessage)
    ..aE<ReportOutcome>(2, _omitFieldNames ? '' : 'outcome',
        enumValues: ReportOutcome.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommunityReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommunityReport copyWith(void Function(CommunityReport) updates) =>
      super.copyWith((message) => updates(message as CommunityReport))
          as CommunityReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CommunityReport() / CommunityReport.new instead')
  static CommunityReport create() => CommunityReport._();
  static $pb.GeneratedMessage $_createMessage() => CommunityReport._();
  @$core.override
  CommunityReport createEmptyInstance() => CommunityReport._();
  @$core.pragma('dart2js:noInline')
  static CommunityReport getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CommunityReport>(
          CommunityReport.$_createMessage);
  static CommunityReport? _defaultInstance;

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

  @$pb.TagNumber(2)
  ReportOutcome get outcome => $_getN(1);
  @$pb.TagNumber(2)
  set outcome(ReportOutcome value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOutcome() => $_has(1);
  @$pb.TagNumber(2)
  void clearOutcome() => $_clearField(2);
}

/// ModerationAppService 是登入的買家檢舉與讀自己的檢舉；對聊天訊息的檢舉，檢舉人是買家或店家：由訊息所屬的對話推導呼叫者是哪一方（店家由呼叫者此刻的店推導，請求不帶身分）。
/// 管理員等非顧客帳號的身分不能呼叫（permission_denied）。
class ModerationAppServiceApi {
  final $pb.RpcClient _client;

  ModerationAppServiceApi(this._client);

  /// SubmitReport 送出一筆檢舉。檢舉人是買家；對象是聊天訊息時，呼叫者是該訊息所屬對話的任一方（買家，或呼叫者此刻的店）都可以送，不是對話的一方回 not_found，與訊息不存在相同。
  /// 對象不存在、已刪除或不是公開可讀回 not_found；檢舉自己的內容或自己回 invalid_argument；每位檢舉人每個台北日最多新增 20 筆，
  /// 超過回 resource_exhausted。同一位檢舉人對同一個對象在案件進行中時再送，回原本那一筆（不吃額度）；案件結案後再送是新的檢舉。
  /// 帶 request_id 時，同一個 request_id 重送一律回第一次的結果，不看案件現在的狀態；內容不同回 invalid_argument。
  $async.Future<SubmitReportResponse> submitReport(
          $pb.ClientContext? ctx, SubmitReportRequest request) =>
      _client.invoke<SubmitReportResponse>(ctx, 'ModerationAppService',
          'SubmitReport', request, SubmitReportResponse());

  /// ListMyReports 列出呼叫者自己送出的全部檢舉（社群內容與買家的，以及店家的），新到舊（建立時間，相同時社群在前，再依 id 新到舊）。
  /// 每筆帶共用欄位與依種類的 subject：社群檢舉帶對象與結果（處理中、違規、不違規），店家檢舉帶 SellerReportAppService 的 StoreReport
  /// （含結果與結果說明；補充與佐證由 GetMyReport 讀）。page_token 不透明，換了查詢就不能沿用。停權的帳號也可以呼叫。
  $async.Future<ListMyReportsResponse> listMyReports(
          $pb.ClientContext? ctx, ListMyReportsRequest request) =>
      _client.invoke<ListMyReportsResponse>(ctx, 'ModerationAppService',
          'ListMyReports', request, ListMyReportsResponse());

  /// ListMyStoreReports 列出呼叫者此刻的店以店家身分送出的檢舉（對象只有聊天訊息），新到舊（建立時間，再依 id 新到舊）；請求不帶 store_id。
  /// 每筆的 subject 是 community（對象與結果）。page_token 不透明，換了查詢就不能沿用。停權的帳號也可以呼叫；沒有店回 not_found，店家退場後沒有此刻的店，所以讀不到。
  $async.Future<ListMyStoreReportsResponse> listMyStoreReports(
          $pb.ClientContext? ctx, ListMyStoreReportsRequest request) =>
      _client.invoke<ListMyStoreReportsResponse>(ctx, 'ModerationAppService',
          'ListMyStoreReports', request, ListMyStoreReportsResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
