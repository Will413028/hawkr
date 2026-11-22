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

/// StoreReport 是檢舉人看到的自己的店家檢舉。平台內部的分案（案件、嚴重度、處理的管理員）與轉報主管機關的紀錄不在這裡；
/// 店名與 logo 由 client 以 store_id 呼叫 SellerPublicService.BatchGetStores。
class StoreReport extends $pb.GeneratedMessage {
  factory StoreReport({
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
    final result = StoreReport._();
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

  StoreReport._();

  factory StoreReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreReport()..mergeFromBuffer(data, registry);
  factory StoreReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreReport()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoreReport',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: StoreReport.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeId')
    ..aOS(3, _omitFieldNames ? '' : 'productId')
    ..aOS(4, _omitFieldNames ? '' : 'orderNo')
    ..aE<$2.PolicyCategory>(5, _omitFieldNames ? '' : 'category',
        enumValues: $2.PolicyCategory.values)
    ..aOS(6, _omitFieldNames ? '' : 'description')
    ..aOS(7, _omitFieldNames ? '' : 'resolutionNote')
    ..aOM<$0.Timestamp>(8, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(9, _omitFieldNames ? '' : 'resolveTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aE<ReportOutcome>(10, _omitFieldNames ? '' : 'outcome',
        enumValues: ReportOutcome.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreReport copyWith(void Function(StoreReport) updates) =>
      super.copyWith((message) => updates(message as StoreReport))
          as StoreReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use StoreReport() / StoreReport.new instead')
  static StoreReport create() => StoreReport._();
  static $pb.GeneratedMessage $_createMessage() => StoreReport._();
  @$core.override
  StoreReport createEmptyInstance() => StoreReport._();
  @$core.pragma('dart2js:noInline')
  static StoreReport getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StoreReport>(
          StoreReport.$_createMessage);
  static StoreReport? _defaultInstance;

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
  @$pb.TagNumber(7)
  $core.String get resolutionNote => $_getSZ(6);
  @$pb.TagNumber(7)
  set resolutionNote($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasResolutionNote() => $_has(6);
  @$pb.TagNumber(7)
  void clearResolutionNote() => $_clearField(7);

  @$pb.TagNumber(8)
  $0.Timestamp get createTime => $_getN(7);
  @$pb.TagNumber(8)
  set createTime($0.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCreateTime() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreateTime() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Timestamp ensureCreateTime() => $_ensure(7);

  /// 結果設定的時間；還沒有結果時不設。
  @$pb.TagNumber(9)
  $0.Timestamp get resolveTime => $_getN(8);
  @$pb.TagNumber(9)
  set resolveTime($0.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasResolveTime() => $_has(8);
  @$pb.TagNumber(9)
  void clearResolveTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Timestamp ensureResolveTime() => $_ensure(8);

  /// 結果；還沒有結果時不設（與 resolve_time 不設同時成立）。
  @$pb.TagNumber(10)
  ReportOutcome get outcome => $_getN(9);
  @$pb.TagNumber(10)
  set outcome(ReportOutcome value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasOutcome() => $_has(9);
  @$pb.TagNumber(10)
  void clearOutcome() => $_clearField(10);
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

  /// 檢舉的類別：只接受 accepted_policy_categories 列出的值，其他值（含 UNSPECIFIED）回 invalid_argument。
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
    StoreReport? report,
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
    ..aOM<StoreReport>(1, _omitFieldNames ? '' : 'report',
        subBuilder: StoreReport.$_createMessage)
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
  StoreReport get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(StoreReport value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  StoreReport ensureReport() => $_ensure(0);

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

class GetMyReportRequest extends $pb.GeneratedMessage {
  factory GetMyReportRequest({
    $core.String? reportId,
  }) {
    final result = GetMyReportRequest._();
    if (reportId != null) result.reportId = reportId;
    return result;
  }

  GetMyReportRequest._();

  factory GetMyReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyReportRequest()..mergeFromBuffer(data, registry);
  factory GetMyReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyReportRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyReportRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyReportRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyReportRequest copyWith(void Function(GetMyReportRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyReportRequest))
          as GetMyReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyReportRequest() / GetMyReportRequest.new instead')
  static GetMyReportRequest create() => GetMyReportRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyReportRequest._();
  @$core.override
  GetMyReportRequest createEmptyInstance() => GetMyReportRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyReportRequest>(
          GetMyReportRequest.$_createMessage);
  static GetMyReportRequest? _defaultInstance;

  /// 檢舉的 id。
  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);
}

class GetMyReportResponse extends $pb.GeneratedMessage {
  factory GetMyReportResponse({
    StoreReport? report,
    $core.Iterable<ReportSupplement>? supplements,
    $core.Iterable<ReportEvidence>? evidence,
    $core.bool? canSupplement,
    $core.int? supplementMax,
  }) {
    final result = GetMyReportResponse._();
    if (report != null) result.report = report;
    if (supplements != null) result.supplements.addAll(supplements);
    if (evidence != null) result.evidence.addAll(evidence);
    if (canSupplement != null) result.canSupplement = canSupplement;
    if (supplementMax != null) result.supplementMax = supplementMax;
    return result;
  }

  GetMyReportResponse._();

  factory GetMyReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyReportResponse()..mergeFromBuffer(data, registry);
  factory GetMyReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyReportResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyReportResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyReportResponse.$_createMessage)
    ..aOM<StoreReport>(1, _omitFieldNames ? '' : 'report',
        subBuilder: StoreReport.$_createMessage)
    ..pPM<ReportSupplement>(2, _omitFieldNames ? '' : 'supplements',
        subBuilder: ReportSupplement.$_createMessage)
    ..pPM<ReportEvidence>(3, _omitFieldNames ? '' : 'evidence',
        subBuilder: ReportEvidence.$_createMessage)
    ..aOB(4, _omitFieldNames ? '' : 'canSupplement')
    ..aI(5, _omitFieldNames ? '' : 'supplementMax')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyReportResponse copyWith(void Function(GetMyReportResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyReportResponse))
          as GetMyReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetMyReportResponse() / GetMyReportResponse.new instead')
  static GetMyReportResponse create() => GetMyReportResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyReportResponse._();
  @$core.override
  GetMyReportResponse createEmptyInstance() => GetMyReportResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyReportResponse>(
          GetMyReportResponse.$_createMessage);
  static GetMyReportResponse? _defaultInstance;

  @$pb.TagNumber(1)
  StoreReport get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(StoreReport value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  StoreReport ensureReport() => $_ensure(0);

  /// 補充（去重併入的與檢舉人主動補充的合在一起），舊到新。
  @$pb.TagNumber(2)
  $pb.PbList<ReportSupplement> get supplements => $_getList(1);

  /// 原檢舉與全部補充的佐證，依送出的順序。
  @$pb.TagNumber(3)
  $pb.PbList<ReportEvidence> get evidence => $_getList(2);

  /// 檢舉人此刻能不能補充（還沒有結果，且補充數未達 supplement_max）；由伺服器判斷，與 AddReportSupplement 的前置檢查是同一個規則，client 不自己從結果與補充數推導。
  @$pb.TagNumber(4)
  $core.bool get canSupplement => $_getBF(3);
  @$pb.TagNumber(4)
  set canSupplement($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCanSupplement() => $_has(3);
  @$pb.TagNumber(4)
  void clearCanSupplement() => $_clearField(4);

  /// 每筆檢舉最多的補充數（伺服器的上限，達到後 AddReportSupplement 回 failed_precondition）；client 用它顯示「n/上限」。
  @$pb.TagNumber(5)
  $core.int get supplementMax => $_getIZ(4);
  @$pb.TagNumber(5)
  set supplementMax($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSupplementMax() => $_has(4);
  @$pb.TagNumber(5)
  void clearSupplementMax() => $_clearField(5);
}

/// ReportSupplement 是檢舉的一則補充；只新增，每筆檢舉最多 10 則。
class ReportSupplement extends $pb.GeneratedMessage {
  factory ReportSupplement({
    $core.String? id,
    $core.String? description,
    $0.Timestamp? createTime,
    $core.Iterable<$core.int>? evidencePositions,
  }) {
    final result = ReportSupplement._();
    if (id != null) result.id = id;
    if (description != null) result.description = description;
    if (createTime != null) result.createTime = createTime;
    if (evidencePositions != null)
      result.evidencePositions.addAll(evidencePositions);
    return result;
  }

  ReportSupplement._();

  factory ReportSupplement.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportSupplement()..mergeFromBuffer(data, registry);
  factory ReportSupplement.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportSupplement()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportSupplement',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ReportSupplement.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOM<$0.Timestamp>(3, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..p<$core.int>(
        4, _omitFieldNames ? '' : 'evidencePositions', $pb.PbFieldType.K3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportSupplement clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportSupplement copyWith(void Function(ReportSupplement) updates) =>
      super.copyWith((message) => updates(message as ReportSupplement))
          as ReportSupplement;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReportSupplement() / ReportSupplement.new instead')
  static ReportSupplement create() => ReportSupplement._();
  static $pb.GeneratedMessage $_createMessage() => ReportSupplement._();
  @$core.override
  ReportSupplement createEmptyInstance() => ReportSupplement._();
  @$core.pragma('dart2js:noInline')
  static ReportSupplement getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReportSupplement>(
          ReportSupplement.$_createMessage);
  static ReportSupplement? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  @$pb.TagNumber(3)
  $0.Timestamp get createTime => $_getN(2);
  @$pb.TagNumber(3)
  set createTime($0.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCreateTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreateTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Timestamp ensureCreateTime() => $_ensure(2);

  /// 這則補充帶來的佐證在 GetMyReportResponse.evidence 的 position。
  @$pb.TagNumber(4)
  $pb.PbList<$core.int> get evidencePositions => $_getList(3);
}

/// ReportEvidence 是檢舉的一張佐證。佐證是機敏檔：回應只帶它在這筆檢舉裡的順序，不帶檔案的網址。
class ReportEvidence extends $pb.GeneratedMessage {
  factory ReportEvidence({
    $core.int? position,
  }) {
    final result = ReportEvidence._();
    if (position != null) result.position = position;
    return result;
  }

  ReportEvidence._();

  factory ReportEvidence.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportEvidence()..mergeFromBuffer(data, registry);
  factory ReportEvidence.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReportEvidence()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportEvidence',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ReportEvidence.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'position')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportEvidence clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportEvidence copyWith(void Function(ReportEvidence) updates) =>
      super.copyWith((message) => updates(message as ReportEvidence))
          as ReportEvidence;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReportEvidence() / ReportEvidence.new instead')
  static ReportEvidence create() => ReportEvidence._();
  static $pb.GeneratedMessage $_createMessage() => ReportEvidence._();
  @$core.override
  ReportEvidence createEmptyInstance() => ReportEvidence._();
  @$core.pragma('dart2js:noInline')
  static ReportEvidence getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReportEvidence>(
          ReportEvidence.$_createMessage);
  static ReportEvidence? _defaultInstance;

  /// 佐證在這筆檢舉裡的順序（從 0 起）。
  @$pb.TagNumber(1)
  $core.int get position => $_getIZ(0);
  @$pb.TagNumber(1)
  set position($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPosition() => $_has(0);
  @$pb.TagNumber(1)
  void clearPosition() => $_clearField(1);
}

class AddReportSupplementRequest extends $pb.GeneratedMessage {
  factory AddReportSupplementRequest({
    $core.String? reportId,
    $core.String? description,
    $core.Iterable<$core.String>? evidenceStagedTokens,
  }) {
    final result = AddReportSupplementRequest._();
    if (reportId != null) result.reportId = reportId;
    if (description != null) result.description = description;
    if (evidenceStagedTokens != null)
      result.evidenceStagedTokens.addAll(evidenceStagedTokens);
    return result;
  }

  AddReportSupplementRequest._();

  factory AddReportSupplementRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddReportSupplementRequest()..mergeFromBuffer(data, registry);
  factory AddReportSupplementRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddReportSupplementRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddReportSupplementRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: AddReportSupplementRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..pPS(3, _omitFieldNames ? '' : 'evidenceStagedTokens')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReportSupplementRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReportSupplementRequest copyWith(
          void Function(AddReportSupplementRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AddReportSupplementRequest))
          as AddReportSupplementRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AddReportSupplementRequest() / AddReportSupplementRequest.new instead')
  static AddReportSupplementRequest create() => AddReportSupplementRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      AddReportSupplementRequest._();
  @$core.override
  AddReportSupplementRequest createEmptyInstance() =>
      AddReportSupplementRequest._();
  @$core.pragma('dart2js:noInline')
  static AddReportSupplementRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddReportSupplementRequest>(
          AddReportSupplementRequest.$_createMessage);
  static AddReportSupplementRequest? _defaultInstance;

  /// 要補充的檢舉 id。
  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);

  /// 補充內容，最多 2000 字（規則同 CreateReport 的 description）。
  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  /// 佐證圖片的 staged token，最多 5 張（同 CreateReport）。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get evidenceStagedTokens => $_getList(2);
}

class AddReportSupplementResponse extends $pb.GeneratedMessage {
  factory AddReportSupplementResponse({
    StoreReport? report,
    $core.Iterable<ReportSupplement>? supplements,
    $core.Iterable<ReportEvidence>? evidence,
    $core.bool? canSupplement,
    $core.int? supplementMax,
  }) {
    final result = AddReportSupplementResponse._();
    if (report != null) result.report = report;
    if (supplements != null) result.supplements.addAll(supplements);
    if (evidence != null) result.evidence.addAll(evidence);
    if (canSupplement != null) result.canSupplement = canSupplement;
    if (supplementMax != null) result.supplementMax = supplementMax;
    return result;
  }

  AddReportSupplementResponse._();

  factory AddReportSupplementResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddReportSupplementResponse()..mergeFromBuffer(data, registry);
  factory AddReportSupplementResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddReportSupplementResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddReportSupplementResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: AddReportSupplementResponse.$_createMessage)
    ..aOM<StoreReport>(1, _omitFieldNames ? '' : 'report',
        subBuilder: StoreReport.$_createMessage)
    ..pPM<ReportSupplement>(2, _omitFieldNames ? '' : 'supplements',
        subBuilder: ReportSupplement.$_createMessage)
    ..pPM<ReportEvidence>(3, _omitFieldNames ? '' : 'evidence',
        subBuilder: ReportEvidence.$_createMessage)
    ..aOB(4, _omitFieldNames ? '' : 'canSupplement')
    ..aI(5, _omitFieldNames ? '' : 'supplementMax')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReportSupplementResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReportSupplementResponse copyWith(
          void Function(AddReportSupplementResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AddReportSupplementResponse))
          as AddReportSupplementResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AddReportSupplementResponse() / AddReportSupplementResponse.new instead')
  static AddReportSupplementResponse create() =>
      AddReportSupplementResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      AddReportSupplementResponse._();
  @$core.override
  AddReportSupplementResponse createEmptyInstance() =>
      AddReportSupplementResponse._();
  @$core.pragma('dart2js:noInline')
  static AddReportSupplementResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddReportSupplementResponse>(
          AddReportSupplementResponse.$_createMessage);
  static AddReportSupplementResponse? _defaultInstance;

  /// 補充之後的檢舉、全部補充與佐證（同 GetMyReportResponse）。
  @$pb.TagNumber(1)
  StoreReport get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(StoreReport value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  StoreReport ensureReport() => $_ensure(0);

  @$pb.TagNumber(2)
  $pb.PbList<ReportSupplement> get supplements => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<ReportEvidence> get evidence => $_getList(2);

  /// 檢舉人此刻能不能補充（還沒有結果，且補充數未達 supplement_max）；由伺服器判斷，與 AddReportSupplement 的前置檢查是同一個規則，client 不自己從結果與補充數推導。
  @$pb.TagNumber(4)
  $core.bool get canSupplement => $_getBF(3);
  @$pb.TagNumber(4)
  set canSupplement($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCanSupplement() => $_has(3);
  @$pb.TagNumber(4)
  void clearCanSupplement() => $_clearField(4);

  /// 每筆檢舉最多的補充數（伺服器的上限，達到後 AddReportSupplement 回 failed_precondition）；client 用它顯示「n/上限」。
  @$pb.TagNumber(5)
  $core.int get supplementMax => $_getIZ(4);
  @$pb.TagNumber(5)
  set supplementMax($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSupplementMax() => $_has(4);
  @$pb.TagNumber(5)
  void clearSupplementMax() => $_clearField(5);
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

  /// GetMyReport 讀呼叫者自己的一筆店家檢舉：檢舉本身、補充與佐證。別人的或不存在的 report_id 一律回 not_found（不洩漏存在與否）。
  /// 呼叫者的全部檢舉（社群與店家）的清單是 hawkr.moderation.v1alpha1.ModerationAppService.ListMyReports。
  $async.Future<GetMyReportResponse> getMyReport(
          $pb.ClientContext? ctx, GetMyReportRequest request) =>
      _client.invoke<GetMyReportResponse>(ctx, 'SellerReportAppService',
          'GetMyReport', request, GetMyReportResponse());

  /// AddReportSupplement 為呼叫者自己、還沒有結果的一筆店家檢舉補充內容。與 CreateReport 去重時併入的補充是同一串、同一個上限
  /// （每筆檢舉合計最多 10 則），admin 看到的是同一串補充。別人的或不存在的 report_id 回 not_found；檢舉已經有結果，或補充已
  /// 達 10 則時回 failed_precondition。每個帳號 10 分鐘最多 5 次、1 小時最多 10 次（與 CreateReport 同樣的額度，各自計數），超過回
  /// resource_exhausted（帶 RetryInfo）。
  $async.Future<AddReportSupplementResponse> addReportSupplement(
          $pb.ClientContext? ctx, AddReportSupplementRequest request) =>
      _client.invoke<AddReportSupplementResponse>(ctx, 'SellerReportAppService',
          'AddReportSupplement', request, AddReportSupplementResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
