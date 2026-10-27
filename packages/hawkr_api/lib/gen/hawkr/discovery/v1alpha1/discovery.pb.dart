// This is a generated file - do not edit.
//
// Generated from hawkr/discovery/v1alpha1/discovery.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import '../../common/v1alpha1/store_card.pb.dart' as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class SuggestRecommendedSellersRequest extends $pb.GeneratedMessage {
  factory SuggestRecommendedSellersRequest({
    $core.int? maxResults,
  }) {
    final result = SuggestRecommendedSellersRequest._();
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestRecommendedSellersRequest._();

  factory SuggestRecommendedSellersRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestRecommendedSellersRequest()..mergeFromBuffer(data, registry);
  factory SuggestRecommendedSellersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestRecommendedSellersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestRecommendedSellersRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.discovery.v1alpha1'),
      createEmptyInstance: SuggestRecommendedSellersRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestRecommendedSellersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestRecommendedSellersRequest copyWith(
          void Function(SuggestRecommendedSellersRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestRecommendedSellersRequest))
          as SuggestRecommendedSellersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestRecommendedSellersRequest() / SuggestRecommendedSellersRequest.new instead')
  static SuggestRecommendedSellersRequest create() =>
      SuggestRecommendedSellersRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestRecommendedSellersRequest._();
  @$core.override
  SuggestRecommendedSellersRequest createEmptyInstance() =>
      SuggestRecommendedSellersRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestRecommendedSellersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestRecommendedSellersRequest>(
          SuggestRecommendedSellersRequest.$_createMessage);
  static SuggestRecommendedSellersRequest? _defaultInstance;

  /// 最多回傳幾家，1 到 50；0 表示預設值 10，超過 50 以 50 計（與分頁的 page_size 相同），負數回 invalid_argument。
  @$pb.TagNumber(1)
  $core.int get maxResults => $_getIZ(0);
  @$pb.TagNumber(1)
  set maxResults($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMaxResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearMaxResults() => $_clearField(1);
}

class SuggestRecommendedSellersResponse extends $pb.GeneratedMessage {
  factory SuggestRecommendedSellersResponse({
    $core.Iterable<RecommendedSeller>? sellers,
  }) {
    final result = SuggestRecommendedSellersResponse._();
    if (sellers != null) result.sellers.addAll(sellers);
    return result;
  }

  SuggestRecommendedSellersResponse._();

  factory SuggestRecommendedSellersResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestRecommendedSellersResponse()..mergeFromBuffer(data, registry);
  factory SuggestRecommendedSellersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestRecommendedSellersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestRecommendedSellersResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.discovery.v1alpha1'),
      createEmptyInstance: SuggestRecommendedSellersResponse.$_createMessage)
    ..pPM<RecommendedSeller>(1, _omitFieldNames ? '' : 'sellers',
        subBuilder: RecommendedSeller.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestRecommendedSellersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestRecommendedSellersResponse copyWith(
          void Function(SuggestRecommendedSellersResponse) updates) =>
      super.copyWith((message) =>
              updates(message as SuggestRecommendedSellersResponse))
          as SuggestRecommendedSellersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestRecommendedSellersResponse() / SuggestRecommendedSellersResponse.new instead')
  static SuggestRecommendedSellersResponse create() =>
      SuggestRecommendedSellersResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestRecommendedSellersResponse._();
  @$core.override
  SuggestRecommendedSellersResponse createEmptyInstance() =>
      SuggestRecommendedSellersResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestRecommendedSellersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestRecommendedSellersResponse>(
          SuggestRecommendedSellersResponse.$_createMessage);
  static SuggestRecommendedSellersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<RecommendedSeller> get sellers => $_getList(0);
}

/// RecommendedSeller 是排行上的一家店。
class RecommendedSeller extends $pb.GeneratedMessage {
  factory RecommendedSeller({
    $0.StoreCard? store,
    $core.int? followerCount,
  }) {
    final result = RecommendedSeller._();
    if (store != null) result.store = store;
    if (followerCount != null) result.followerCount = followerCount;
    return result;
  }

  RecommendedSeller._();

  factory RecommendedSeller.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecommendedSeller()..mergeFromBuffer(data, registry);
  factory RecommendedSeller.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecommendedSeller()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecommendedSeller',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.discovery.v1alpha1'),
      createEmptyInstance: RecommendedSeller.$_createMessage)
    ..aOM<$0.StoreCard>(1, _omitFieldNames ? '' : 'store',
        subBuilder: $0.StoreCard.$_createMessage)
    ..aI(4, _omitFieldNames ? '' : 'followerCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecommendedSeller clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecommendedSeller copyWith(void Function(RecommendedSeller) updates) =>
      super.copyWith((message) => updates(message as RecommendedSeller))
          as RecommendedSeller;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RecommendedSeller() / RecommendedSeller.new instead')
  static RecommendedSeller create() => RecommendedSeller._();
  static $pb.GeneratedMessage $_createMessage() => RecommendedSeller._();
  @$core.override
  RecommendedSeller createEmptyInstance() => RecommendedSeller._();
  @$core.pragma('dart2js:noInline')
  static RecommendedSeller getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RecommendedSeller>(
          RecommendedSeller.$_createMessage);
  static RecommendedSeller? _defaultInstance;

  /// 店家卡片；清單上的每家店都有營業中（OPEN）的卡片。
  @$pb.TagNumber(1)
  $0.StoreCard get store => $_getN(0);
  @$pb.TagNumber(1)
  set store($0.StoreCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.StoreCard ensureStore() => $_ensure(0);

  /// 粉絲數；0 是確實沒有粉絲，讀取失敗時不設。
  @$pb.TagNumber(4)
  $core.int get followerCount => $_getIZ(1);
  @$pb.TagNumber(4)
  set followerCount($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(4)
  $core.bool hasFollowerCount() => $_has(1);
  @$pb.TagNumber(4)
  void clearFollowerCount() => $_clearField(4);
}

/// DiscoveryPublicService 是任何人都能讀的探索資料，不需要登入。
class DiscoveryPublicServiceApi {
  final $pb.RpcClient _client;

  DiscoveryPublicServiceApi(this._client);

  /// SuggestRecommendedSellers 依推薦分數由高到低回傳前幾家可以公開曝光的店家：沒有分頁（排行只取前面幾筆），數量由
  /// max_results 決定。分數是排程每 15 分鐘重算的快照；曝光資格是 seller 的「可公開曝光」規則（營運中、沒有停權、身分認證與
  /// PayUni 審核通過、店名與 logo 與所在地齊全），每次讀取即時檢查，停權立刻生效。店家卡片上的評價統計（review_count、
  /// rating_average）與其他清單上同一家店的卡片一致，是讀取時批次查詢評價的結果，不是排名用的快照；評價讀取失敗時卡片不設
  /// 評價欄位。沒有任何合格的店家時回空清單；排行上的店家若此刻沒有營業中的卡片（資料不一致，或剛好在兩次讀取之間不再可
  /// 曝光）會被略過，所以回傳的家數可能少於 max_results。粉絲數來自別的模組：讀取失敗時該欄位不設，不會是 0。
  $async.Future<SuggestRecommendedSellersResponse> suggestRecommendedSellers(
          $pb.ClientContext? ctx, SuggestRecommendedSellersRequest request) =>
      _client.invoke<SuggestRecommendedSellersResponse>(
          ctx,
          'DiscoveryPublicService',
          'SuggestRecommendedSellers',
          request,
          SuggestRecommendedSellersResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
