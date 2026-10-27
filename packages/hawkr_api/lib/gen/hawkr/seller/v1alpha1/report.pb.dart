// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/report.proto.

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

import '../../../google/rpc/error_details.pb.dart' as $1;
import '../../common/v1alpha1/policy_category.pbenum.dart' as $2;
import 'report.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'report.pbenum.dart';

/// MyReport 是檢舉人看到的自己的檢舉。平台內部的分案（案件、嚴重度、處理的管理員）與轉報主管機關的紀錄不在這裡；
/// 店名與 logo 由 client 以 store_id 呼叫 SellerPublicService.BatchGetStores。
class MyReport extends $pb.GeneratedMessage {
  factory MyReport({
    $core.String? id,
    $core.String? storeId,
    $core.String? productId,
    $core.String? orderNo,
    $2.PolicyCategory? category,
    $core.String? description,
    $core.String? resolutionNote,
    $0.Timestamp? createTime,
    $0.Timestamp? resolveTime,
    ReportOutcome? outcome,
  }) {
    final result = MyReport._();
    if (id != null) result.id = id;
    if (storeId != null) result.storeId = storeId;
    if (productId != null) result.productId = productId;
    if (orderNo != null) result.orderNo = orderNo;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (resolutionNote != null) result.resolutionNote = resolutionNote;
    if (createTime != null) result.createTime = createTime;
    if (resolveTime != null) result.resolveTime = resolveTime;
    if (outcome != null) result.outcome = outcome;
    return result;
  }

  MyReport._();

  factory MyReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyReport()..mergeFromBuffer(data, registry);
  factory MyReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyReport()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MyReport',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: MyReport.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeId')
    ..aOS(3, _omitFieldNames ? '' : 'productId')
    ..aOS(4, _omitFieldNames ? '' : 'orderNo')
    ..aE<$2.PolicyCategory>(5, _omitFieldNames ? '' : 'category',
        enumValues: $2.PolicyCategory.values)
    ..aOS(6, _omitFieldNames ? '' : 'description')
    ..aOS(8, _omitFieldNames ? '' : 'resolutionNote')
    ..aOM<$0.Timestamp>(9, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(10, _omitFieldNames ? '' : 'resolveTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aE<ReportOutcome>(11, _omitFieldNames ? '' : 'outcome',
        enumValues: ReportOutcome.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyReport copyWith(void Function(MyReport) updates) =>
      super.copyWith((message) => updates(message as MyReport)) as MyReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MyReport() / MyReport.new instead')
  static MyReport create() => MyReport._();
  static $pb.GeneratedMessage $_createMessage() => MyReport._();
  @$core.override
  MyReport createEmptyInstance() => MyReport._();
  @$core.pragma('dart2js:noInline')
  static MyReport getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MyReport>(MyReport.$_createMessage);
  static MyReport? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get storeId => $_getSZ(1);
  @$pb.TagNumber(2)
  set storeId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoreId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoreId() => $_clearField(2);

  /// 空字串表示沒有提供。
  @$pb.TagNumber(3)
  $core.String get productId => $_getSZ(2);
  @$pb.TagNumber(3)
  set productId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProductId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProductId() => $_clearField(3);

  /// 空字串表示沒有提供。
  @$pb.TagNumber(4)
  $core.String get orderNo => $_getSZ(3);
  @$pb.TagNumber(4)
  set orderNo($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOrderNo() => $_has(3);
  @$pb.TagNumber(4)
  void clearOrderNo() => $_clearField(4);

  /// 檢舉人選的類別。
  @$pb.TagNumber(5)
  $2.PolicyCategory get category => $_getN(4);
  @$pb.TagNumber(5)
  set category($2.PolicyCategory value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCategory() => $_has(4);
  @$pb.TagNumber(5)
  void clearCategory() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get description => $_getSZ(5);
  @$pb.TagNumber(6)
  set description($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDescription() => $_has(5);
  @$pb.TagNumber(6)
  void clearDescription() => $_clearField(6);

  /// 平台給檢舉人的結果說明；還沒有結果或結果沒有說明時是空字串。
  @$pb.TagNumber(8)
  $core.String get resolutionNote => $_getSZ(6);
  @$pb.TagNumber(8)
  set resolutionNote($core.String value) => $_setString(6, value);
  @$pb.TagNumber(8)
  $core.bool hasResolutionNote() => $_has(6);
  @$pb.TagNumber(8)
  void clearResolutionNote() => $_clearField(8);

  @$pb.TagNumber(9)
  $0.Timestamp get createTime => $_getN(7);
  @$pb.TagNumber(9)
  set createTime($0.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCreateTime() => $_has(7);
  @$pb.TagNumber(9)
  void clearCreateTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Timestamp ensureCreateTime() => $_ensure(7);

  /// 結果設定的時間；還沒有結果時不設。
  @$pb.TagNumber(10)
  $0.Timestamp get resolveTime => $_getN(8);
  @$pb.TagNumber(10)
  set resolveTime($0.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasResolveTime() => $_has(8);
  @$pb.TagNumber(10)
  void clearResolveTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $0.Timestamp ensureResolveTime() => $_ensure(8);

  /// 結果；還沒有結果時是 UNSPECIFIED（處理中）。
  @$pb.TagNumber(11)
  ReportOutcome get outcome => $_getN(9);
  @$pb.TagNumber(11)
  set outcome(ReportOutcome value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasOutcome() => $_has(9);
  @$pb.TagNumber(11)
  void clearOutcome() => $_clearField(11);
}

class CreateReportRequest extends $pb.GeneratedMessage {
  factory CreateReportRequest({
    $core.String? storeId,
    $2.PolicyCategory? category,
    $core.String? description,
    $core.String? productId,
    $core.String? orderNo,
    $core.Iterable<$core.String>? evidenceStagedTokens,
  }) {
    final result = CreateReportRequest._();
    if (storeId != null) result.storeId = storeId;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (productId != null) result.productId = productId;
    if (orderNo != null) result.orderNo = orderNo;
    if (evidenceStagedTokens != null)
      result.evidenceStagedTokens.addAll(evidenceStagedTokens);
    return result;
  }

  CreateReportRequest._();

  factory CreateReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReportRequest()..mergeFromBuffer(data, registry);
  factory CreateReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReportRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReportRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CreateReportRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aE<$2.PolicyCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: $2.PolicyCategory.values)
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'productId')
    ..aOS(5, _omitFieldNames ? '' : 'orderNo')
    ..pPS(6, _omitFieldNames ? '' : 'evidenceStagedTokens')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportRequest copyWith(void Function(CreateReportRequest) updates) =>
      super.copyWith((message) => updates(message as CreateReportRequest))
          as CreateReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CreateReportRequest() / CreateReportRequest.new instead')
  static CreateReportRequest create() => CreateReportRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateReportRequest._();
  @$core.override
  CreateReportRequest createEmptyInstance() => CreateReportRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReportRequest>(
          CreateReportRequest.$_createMessage);
  static CreateReportRequest? _defaultInstance;

  /// 被檢舉的店家 id（店家卡片的 id，不是 store_code）。
  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 檢舉的類別：政策分類中檢舉人可選的值，除了 PLATFORM_ABUSE（只給平台判定與認定）；UNSPECIFIED 與 PLATFORM_ABUSE 回 invalid_argument。
  @$pb.TagNumber(2)
  $2.PolicyCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category($2.PolicyCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  /// 檢舉內容，最多 2000 字。
  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  /// 相關的商品 id，最多 64 字；可以不填。
  @$pb.TagNumber(4)
  $core.String get productId => $_getSZ(3);
  @$pb.TagNumber(4)
  set productId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProductId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProductId() => $_clearField(4);

  /// 相關的訂單編號，最多 64 字；可以不填。
  @$pb.TagNumber(5)
  $core.String get orderNo => $_getSZ(4);
  @$pb.TagNumber(5)
  set orderNo($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOrderNo() => $_has(4);
  @$pb.TagNumber(5)
  void clearOrderNo() => $_clearField(5);

  /// 佐證圖片的 staged token，最多 5 張。
  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get evidenceStagedTokens => $_getList(5);
}

class CreateReportResponse extends $pb.GeneratedMessage {
  factory CreateReportResponse({
    MyReport? report,
    $core.bool? supplemented,
    $1.LocalizedMessage? notice,
  }) {
    final result = CreateReportResponse._();
    if (report != null) result.report = report;
    if (supplemented != null) result.supplemented = supplemented;
    if (notice != null) result.notice = notice;
    return result;
  }

  CreateReportResponse._();

  factory CreateReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReportResponse()..mergeFromBuffer(data, registry);
  factory CreateReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReportResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReportResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CreateReportResponse.$_createMessage)
    ..aOM<MyReport>(1, _omitFieldNames ? '' : 'report',
        subBuilder: MyReport.$_createMessage)
    ..aOB(2, _omitFieldNames ? '' : 'supplemented')
    ..aOM<$1.LocalizedMessage>(3, _omitFieldNames ? '' : 'notice',
        subBuilder: $1.LocalizedMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportResponse copyWith(void Function(CreateReportResponse) updates) =>
      super.copyWith((message) => updates(message as CreateReportResponse))
          as CreateReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateReportResponse() / CreateReportResponse.new instead')
  static CreateReportResponse create() => CreateReportResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateReportResponse._();
  @$core.override
  CreateReportResponse createEmptyInstance() => CreateReportResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReportResponse>(
          CreateReportResponse.$_createMessage);
  static CreateReportResponse? _defaultInstance;

  /// 新的檢舉；supplemented 時是原本那一筆（去重的鍵含買家，所以一定是呼叫者自己的）。
  @$pb.TagNumber(1)
  MyReport get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(MyReport value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  MyReport ensureReport() => $_ensure(0);

  /// 這次的內容補充到同一個台北日、同一家店、同一個類別、還沒有結果的那一筆。
  @$pb.TagNumber(2)
  $core.bool get supplemented => $_getBF(1);
  @$pb.TagNumber(2)
  set supplemented($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSupplemented() => $_has(1);
  @$pb.TagNumber(2)
  void clearSupplemented() => $_clearField(2);

  /// supplemented 時給檢舉人看的那一句（伺服器的句子：它說的是伺服器的去重規則）；沒有補充時不設。
  @$pb.TagNumber(3)
  $1.LocalizedMessage get notice => $_getN(2);
  @$pb.TagNumber(3)
  set notice($1.LocalizedMessage value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasNotice() => $_has(2);
  @$pb.TagNumber(3)
  void clearNotice() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.LocalizedMessage ensureNotice() => $_ensure(2);
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
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
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

  /// 預設 20，上限 100。
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
    $core.Iterable<MyReport>? reports,
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
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListMyReportsResponse.$_createMessage)
    ..pPM<MyReport>(1, _omitFieldNames ? '' : 'reports',
        subBuilder: MyReport.$_createMessage)
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
  $pb.PbList<MyReport> get reports => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class SubmitFoodSafetyReportRequest extends $pb.GeneratedMessage {
  factory SubmitFoodSafetyReportRequest({
    $core.String? storeId,
    $core.String? description,
    $core.String? productId,
    $core.String? orderNo,
    $core.String? phone,
    $core.String? email,
    $core.String? name,
  }) {
    final result = SubmitFoodSafetyReportRequest._();
    if (storeId != null) result.storeId = storeId;
    if (description != null) result.description = description;
    if (productId != null) result.productId = productId;
    if (orderNo != null) result.orderNo = orderNo;
    if (phone != null) result.phone = phone;
    if (email != null) result.email = email;
    if (name != null) result.name = name;
    return result;
  }

  SubmitFoodSafetyReportRequest._();

  factory SubmitFoodSafetyReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitFoodSafetyReportRequest()..mergeFromBuffer(data, registry);
  factory SubmitFoodSafetyReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitFoodSafetyReportRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitFoodSafetyReportRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitFoodSafetyReportRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'productId')
    ..aOS(4, _omitFieldNames ? '' : 'orderNo')
    ..aOS(5, _omitFieldNames ? '' : 'phone')
    ..aOS(6, _omitFieldNames ? '' : 'email')
    ..aOS(7, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitFoodSafetyReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitFoodSafetyReportRequest copyWith(
          void Function(SubmitFoodSafetyReportRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SubmitFoodSafetyReportRequest))
          as SubmitFoodSafetyReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitFoodSafetyReportRequest() / SubmitFoodSafetyReportRequest.new instead')
  static SubmitFoodSafetyReportRequest create() =>
      SubmitFoodSafetyReportRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SubmitFoodSafetyReportRequest._();
  @$core.override
  SubmitFoodSafetyReportRequest createEmptyInstance() =>
      SubmitFoodSafetyReportRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitFoodSafetyReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitFoodSafetyReportRequest>(
          SubmitFoodSafetyReportRequest.$_createMessage);
  static SubmitFoodSafetyReportRequest? _defaultInstance;

  /// 被檢舉的店家 id（店家卡片的 id，不是 store_code）。
  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 事件的說明，最多 2000 字。
  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  /// 相關的商品 id，最多 64 字；可以不填。
  @$pb.TagNumber(3)
  $core.String get productId => $_getSZ(2);
  @$pb.TagNumber(3)
  set productId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProductId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProductId() => $_clearField(3);

  /// 相關的訂單編號，最多 64 字；可以不填。
  @$pb.TagNumber(4)
  $core.String get orderNo => $_getSZ(3);
  @$pb.TagNumber(4)
  set orderNo($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOrderNo() => $_has(3);
  @$pb.TagNumber(4)
  void clearOrderNo() => $_clearField(4);

  /// 台灣的電話號碼；phone 與 email 至少填一個。
  @$pb.TagNumber(5)
  $core.String get phone => $_getSZ(4);
  @$pb.TagNumber(5)
  set phone($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPhone() => $_has(4);
  @$pb.TagNumber(5)
  void clearPhone() => $_clearField(5);

  /// phone 與 email 至少填一個，最多 254 字。
  @$pb.TagNumber(6)
  $core.String get email => $_getSZ(5);
  @$pb.TagNumber(6)
  set email($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasEmail() => $_has(5);
  @$pb.TagNumber(6)
  void clearEmail() => $_clearField(6);

  /// 檢舉人的稱呼，最多 100 字；可以不填。
  @$pb.TagNumber(7)
  $core.String get name => $_getSZ(6);
  @$pb.TagNumber(7)
  set name($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasName() => $_has(6);
  @$pb.TagNumber(7)
  void clearName() => $_clearField(7);
}

/// SubmitFoodSafetyReportResponse 沒有內容：新的檢舉、補充與超過上限而沒有存下的送出回應相同。
class SubmitFoodSafetyReportResponse extends $pb.GeneratedMessage {
  factory SubmitFoodSafetyReportResponse() =>
      SubmitFoodSafetyReportResponse._();

  SubmitFoodSafetyReportResponse._();

  factory SubmitFoodSafetyReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitFoodSafetyReportResponse()..mergeFromBuffer(data, registry);
  factory SubmitFoodSafetyReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitFoodSafetyReportResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitFoodSafetyReportResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitFoodSafetyReportResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitFoodSafetyReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitFoodSafetyReportResponse copyWith(
          void Function(SubmitFoodSafetyReportResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SubmitFoodSafetyReportResponse))
          as SubmitFoodSafetyReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitFoodSafetyReportResponse() / SubmitFoodSafetyReportResponse.new instead')
  static SubmitFoodSafetyReportResponse create() =>
      SubmitFoodSafetyReportResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SubmitFoodSafetyReportResponse._();
  @$core.override
  SubmitFoodSafetyReportResponse createEmptyInstance() =>
      SubmitFoodSafetyReportResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitFoodSafetyReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitFoodSafetyReportResponse>(
          SubmitFoodSafetyReportResponse.$_createMessage);
  static SubmitFoodSafetyReportResponse? _defaultInstance;
}

/// SellerReportAppService 是登入的買家的檢舉。
class SellerReportAppServiceApi {
  final $pb.RpcClient _client;

  SellerReportAppServiceApi(this._client);

  /// CreateReport 以呼叫者的買家身分檢舉一家店。店家必須是曾經公開過的店（BatchGetStores 回得到的；已關閉的也可以），否則回
  /// not_found；店家屬於呼叫者的帳號時回 invalid_argument（store_id）。同一個買家對同一家店、同一個類別，一個台北日曆日內、
  /// 還沒有結果的那一筆存在時，這次的內容補充到它（supplemented 為 true，report 是原本那一筆，notice 說明發生了什麼）；原本那一筆
  /// 已經有結果時，這次是新的檢舉。原本那一筆已有 10 則補充時回 failed_precondition。每個帳號 10 分鐘最多 5 次、1 小時最多 10 次，
  /// 超過回 resource_exhausted（帶 RetryInfo）。
  $async.Future<CreateReportResponse> createReport(
          $pb.ClientContext? ctx, CreateReportRequest request) =>
      _client.invoke<CreateReportResponse>(ctx, 'SellerReportAppService',
          'CreateReport', request, CreateReportResponse());

  /// ListMyReports 列出呼叫者的檢舉，新到舊。
  $async.Future<ListMyReportsResponse> listMyReports(
          $pb.ClientContext? ctx, ListMyReportsRequest request) =>
      _client.invoke<ListMyReportsResponse>(ctx, 'SellerReportAppService',
          'ListMyReports', request, ListMyReportsResponse());
}

/// SellerReportPublicService 是不需要登入的檢舉入口。
class SellerReportPublicServiceApi {
  final $pb.RpcClient _client;

  SellerReportPublicServiceApi(this._client);

  /// SubmitFoodSafetyReport 匿名檢舉食品安全事件，必須留下電話或 email（主管機關可能需要聯絡檢舉人），不收佐證圖片（要附圖請
  /// 登入後以 CreateReport 檢舉）。店家的規則與 CreateReport 相同；呼叫者登入而且店家屬於他的帳號時回 invalid_argument（store_id）。
  /// 同一支電話或同一個 email 對同一家店，一個台北 ISO 週內、還沒有結果的那一筆存在時，這次的內容補充到它（每筆最多 10 則，超過時
  /// 內容不存）；新的檢舉、補充與超過上限的送出回應完全相同（回應沒有內容），不會透露這支電話或 email 用過。每個來源 IP
  /// （登入的呼叫者以帳號計）10 分鐘最多 5 次、1 小時最多 10 次，被拒絕的送出不計；超過回 resource_exhausted（帶 RetryInfo）。
  /// 聯絡方式在「結果設定後 30 天、且不早於送出後 90 天」清除；還沒有結果的檢舉不清除。
  $async.Future<SubmitFoodSafetyReportResponse> submitFoodSafetyReport(
          $pb.ClientContext? ctx, SubmitFoodSafetyReportRequest request) =>
      _client.invoke<SubmitFoodSafetyReportResponse>(
          ctx,
          'SellerReportPublicService',
          'SubmitFoodSafetyReport',
          request,
          SubmitFoodSafetyReportResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
