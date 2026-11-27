// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart'
    as $4;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import '../../common/v1alpha1/delegated_operation.pb.dart' as $3;
import '../../common/v1alpha1/image.pb.dart' as $0;
import '../../common/v1alpha1/store_card.pb.dart' as $1;
import 'catalog.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'catalog.pbenum.dart';

/// ProductFilter 是公開列表與搜尋共用的篩選，全部選填，之間是 AND。不含觀看者相關的篩選（收藏、追蹤）。
class ProductFilter extends $pb.GeneratedMessage {
  factory ProductFilter({
    $core.Iterable<$core.String>? categoryIds,
    $fixnum.Int64? priceMin,
    $fixnum.Int64? priceMax,
    $core.Iterable<$core.String>? hashtags,
    HashtagMatch? hashtagMatch,
    $core.Iterable<$core.String>? storeIds,
  }) {
    final result = ProductFilter._();
    if (categoryIds != null) result.categoryIds.addAll(categoryIds);
    if (priceMin != null) result.priceMin = priceMin;
    if (priceMax != null) result.priceMax = priceMax;
    if (hashtags != null) result.hashtags.addAll(hashtags);
    if (hashtagMatch != null) result.hashtagMatch = hashtagMatch;
    if (storeIds != null) result.storeIds.addAll(storeIds);
    return result;
  }

  ProductFilter._();

  factory ProductFilter.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductFilter()..mergeFromBuffer(data, registry);
  factory ProductFilter.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductFilter()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductFilter',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductFilter.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'categoryIds')
    ..aInt64(2, _omitFieldNames ? '' : 'priceMin')
    ..aInt64(3, _omitFieldNames ? '' : 'priceMax')
    ..pPS(4, _omitFieldNames ? '' : 'hashtags')
    ..aE<HashtagMatch>(5, _omitFieldNames ? '' : 'hashtagMatch',
        enumValues: HashtagMatch.values)
    ..pPS(6, _omitFieldNames ? '' : 'storeIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductFilter clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductFilter copyWith(void Function(ProductFilter) updates) =>
      super.copyWith((message) => updates(message as ProductFilter))
          as ProductFilter;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductFilter() / ProductFilter.new instead')
  static ProductFilter create() => ProductFilter._();
  static $pb.GeneratedMessage $_createMessage() => ProductFilter._();
  @$core.override
  ProductFilter createEmptyInstance() => ProductFilter._();
  @$core.pragma('dart2js:noInline')
  static ProductFilter getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductFilter>(
          ProductFilter.$_createMessage);
  static ProductFilter? _defaultInstance;

  /// 分類 ID，可以給頂層分類（包含它底下所有葉節點）或葉節點；多個之間是 OR。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get categoryIds => $_getList(0);

  /// 價格區間，新台幣元的整數，含邊界；比對商品的 [min_price, max_price] 與區間有交集。
  @$pb.TagNumber(2)
  $fixnum.Int64 get priceMin => $_getI64(1);
  @$pb.TagNumber(2)
  set priceMin($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPriceMin() => $_has(1);
  @$pb.TagNumber(2)
  void clearPriceMin() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get priceMax => $_getI64(2);
  @$pb.TagNumber(3)
  set priceMax($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPriceMax() => $_has(2);
  @$pb.TagNumber(3)
  void clearPriceMax() => $_clearField(3);

  /// 標籤（正規化後的名稱，或使用者輸入的樣子，規則同 ProductInput.hashtag_inputs）。
  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get hashtags => $_getList(3);

  @$pb.TagNumber(5)
  HashtagMatch get hashtagMatch => $_getN(4);
  @$pb.TagNumber(5)
  set hashtagMatch(HashtagMatch value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasHashtagMatch() => $_has(4);
  @$pb.TagNumber(5)
  void clearHashtagMatch() => $_clearField(5);

  /// 只看這些店家的商品；店家不可曝光的照樣排除。
  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get storeIds => $_getList(5);
}

class ListProductsRequest extends $pb.GeneratedMessage {
  factory ListProductsRequest({
    ProductFilter? filter,
    ProductSort? sort,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListProductsRequest._();
    if (filter != null) result.filter = filter;
    if (sort != null) result.sort = sort;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListProductsRequest._();

  factory ListProductsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductsRequest()..mergeFromBuffer(data, registry);
  factory ListProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListProductsRequest.$_createMessage)
    ..aOM<ProductFilter>(1, _omitFieldNames ? '' : 'filter',
        subBuilder: ProductFilter.$_createMessage)
    ..aE<ProductSort>(2, _omitFieldNames ? '' : 'sort',
        enumValues: ProductSort.values)
    ..aI(3, _omitFieldNames ? '' : 'pageSize')
    ..aOS(4, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductsRequest copyWith(void Function(ListProductsRequest) updates) =>
      super.copyWith((message) => updates(message as ListProductsRequest))
          as ListProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListProductsRequest() / ListProductsRequest.new instead')
  static ListProductsRequest create() => ListProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListProductsRequest._();
  @$core.override
  ListProductsRequest createEmptyInstance() => ListProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListProductsRequest>(
          ListProductsRequest.$_createMessage);
  static ListProductsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ProductFilter get filter => $_getN(0);
  @$pb.TagNumber(1)
  set filter(ProductFilter value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearFilter() => $_clearField(1);
  @$pb.TagNumber(1)
  ProductFilter ensureFilter() => $_ensure(0);

  /// 不指定是 LATEST；不接受 RELEVANCE。
  @$pb.TagNumber(2)
  ProductSort get sort => $_getN(1);
  @$pb.TagNumber(2)
  set sort(ProductSort value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSort() => $_has(1);
  @$pb.TagNumber(2)
  void clearSort() => $_clearField(2);

  /// 預設 20，上限 100；負數回 invalid_argument。
  @$pb.TagNumber(3)
  $core.int get pageSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set pageSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageSize() => $_clearField(3);

  /// page_token 與 filter、sort 綁定，其中任何一個與取得 token 時不同回 invalid_argument。
  @$pb.TagNumber(4)
  $core.String get pageToken => $_getSZ(3);
  @$pb.TagNumber(4)
  set pageToken($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPageToken() => $_has(3);
  @$pb.TagNumber(4)
  void clearPageToken() => $_clearField(4);
}

class ListProductsResponse extends $pb.GeneratedMessage {
  factory ListProductsResponse({
    $core.Iterable<ProductSummary>? products,
    $core.String? nextPageToken,
  }) {
    final result = ListProductsResponse._();
    if (products != null) result.products.addAll(products);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListProductsResponse._();

  factory ListProductsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductsResponse()..mergeFromBuffer(data, registry);
  factory ListProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListProductsResponse.$_createMessage)
    ..pPM<ProductSummary>(1, _omitFieldNames ? '' : 'products',
        subBuilder: ProductSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductsResponse copyWith(void Function(ListProductsResponse) updates) =>
      super.copyWith((message) => updates(message as ListProductsResponse))
          as ListProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListProductsResponse() / ListProductsResponse.new instead')
  static ListProductsResponse create() => ListProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListProductsResponse._();
  @$core.override
  ListProductsResponse createEmptyInstance() => ListProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListProductsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListProductsResponse>(
          ListProductsResponse.$_createMessage);
  static ListProductsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ProductSummary> get products => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class SearchProductsRequest extends $pb.GeneratedMessage {
  factory SearchProductsRequest({
    $core.String? query,
    ProductFilter? filter,
    ProductSort? sort,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = SearchProductsRequest._();
    if (query != null) result.query = query;
    if (filter != null) result.filter = filter;
    if (sort != null) result.sort = sort;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  SearchProductsRequest._();

  factory SearchProductsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchProductsRequest()..mergeFromBuffer(data, registry);
  factory SearchProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SearchProductsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOM<ProductFilter>(2, _omitFieldNames ? '' : 'filter',
        subBuilder: ProductFilter.$_createMessage)
    ..aE<ProductSort>(3, _omitFieldNames ? '' : 'sort',
        enumValues: ProductSort.values)
    ..aI(4, _omitFieldNames ? '' : 'pageSize')
    ..aOS(5, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProductsRequest copyWith(
          void Function(SearchProductsRequest) updates) =>
      super.copyWith((message) => updates(message as SearchProductsRequest))
          as SearchProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SearchProductsRequest() / SearchProductsRequest.new instead')
  static SearchProductsRequest create() => SearchProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() => SearchProductsRequest._();
  @$core.override
  SearchProductsRequest createEmptyInstance() => SearchProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static SearchProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchProductsRequest>(
          SearchProductsRequest.$_createMessage);
  static SearchProductsRequest? _defaultInstance;

  /// 關鍵字；去頭尾空白後不得為空。
  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  @$pb.TagNumber(2)
  ProductFilter get filter => $_getN(1);
  @$pb.TagNumber(2)
  set filter(ProductFilter value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFilter() => $_has(1);
  @$pb.TagNumber(2)
  void clearFilter() => $_clearField(2);
  @$pb.TagNumber(2)
  ProductFilter ensureFilter() => $_ensure(1);

  /// 不指定是 RELEVANCE。
  @$pb.TagNumber(3)
  ProductSort get sort => $_getN(2);
  @$pb.TagNumber(3)
  set sort(ProductSort value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSort() => $_has(2);
  @$pb.TagNumber(3)
  void clearSort() => $_clearField(3);

  /// 預設 20，上限 100。
  @$pb.TagNumber(4)
  $core.int get pageSize => $_getIZ(3);
  @$pb.TagNumber(4)
  set pageSize($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPageSize() => $_has(3);
  @$pb.TagNumber(4)
  void clearPageSize() => $_clearField(4);

  /// page_token 與 query、filter、sort 綁定。
  @$pb.TagNumber(5)
  $core.String get pageToken => $_getSZ(4);
  @$pb.TagNumber(5)
  set pageToken($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPageToken() => $_has(4);
  @$pb.TagNumber(5)
  void clearPageToken() => $_clearField(5);
}

class SearchProductsResponse extends $pb.GeneratedMessage {
  factory SearchProductsResponse({
    $core.Iterable<ProductSummary>? products,
    $core.String? nextPageToken,
  }) {
    final result = SearchProductsResponse._();
    if (products != null) result.products.addAll(products);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  SearchProductsResponse._();

  factory SearchProductsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchProductsResponse()..mergeFromBuffer(data, registry);
  factory SearchProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SearchProductsResponse.$_createMessage)
    ..pPM<ProductSummary>(1, _omitFieldNames ? '' : 'products',
        subBuilder: ProductSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProductsResponse copyWith(
          void Function(SearchProductsResponse) updates) =>
      super.copyWith((message) => updates(message as SearchProductsResponse))
          as SearchProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SearchProductsResponse() / SearchProductsResponse.new instead')
  static SearchProductsResponse create() => SearchProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() => SearchProductsResponse._();
  @$core.override
  SearchProductsResponse createEmptyInstance() => SearchProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static SearchProductsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchProductsResponse>(
          SearchProductsResponse.$_createMessage);
  static SearchProductsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ProductSummary> get products => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class SuggestProductNamesRequest extends $pb.GeneratedMessage {
  factory SuggestProductNamesRequest({
    $core.String? query,
    $core.int? maxResults,
  }) {
    final result = SuggestProductNamesRequest._();
    if (query != null) result.query = query;
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestProductNamesRequest._();

  factory SuggestProductNamesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestProductNamesRequest()..mergeFromBuffer(data, registry);
  factory SuggestProductNamesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestProductNamesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestProductNamesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestProductNamesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aI(2, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestProductNamesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestProductNamesRequest copyWith(
          void Function(SuggestProductNamesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestProductNamesRequest))
          as SuggestProductNamesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestProductNamesRequest() / SuggestProductNamesRequest.new instead')
  static SuggestProductNamesRequest create() => SuggestProductNamesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestProductNamesRequest._();
  @$core.override
  SuggestProductNamesRequest createEmptyInstance() =>
      SuggestProductNamesRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestProductNamesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestProductNamesRequest>(
          SuggestProductNamesRequest.$_createMessage);
  static SuggestProductNamesRequest? _defaultInstance;

  /// 輸入的前綴或片段；正規化後是空的回空清單。
  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  /// 最多回傳幾筆，1 到 20；0 表示預設值 10，超過 20 以 20 計，負數回 invalid_argument。
  @$pb.TagNumber(2)
  $core.int get maxResults => $_getIZ(1);
  @$pb.TagNumber(2)
  set maxResults($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMaxResults() => $_has(1);
  @$pb.TagNumber(2)
  void clearMaxResults() => $_clearField(2);
}

class SuggestProductNamesResponse extends $pb.GeneratedMessage {
  factory SuggestProductNamesResponse({
    $core.Iterable<$core.String>? names,
  }) {
    final result = SuggestProductNamesResponse._();
    if (names != null) result.names.addAll(names);
    return result;
  }

  SuggestProductNamesResponse._();

  factory SuggestProductNamesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestProductNamesResponse()..mergeFromBuffer(data, registry);
  factory SuggestProductNamesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestProductNamesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestProductNamesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestProductNamesResponse.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'names')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestProductNamesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestProductNamesResponse copyWith(
          void Function(SuggestProductNamesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestProductNamesResponse))
          as SuggestProductNamesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestProductNamesResponse() / SuggestProductNamesResponse.new instead')
  static SuggestProductNamesResponse create() =>
      SuggestProductNamesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestProductNamesResponse._();
  @$core.override
  SuggestProductNamesResponse createEmptyInstance() =>
      SuggestProductNamesResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestProductNamesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestProductNamesResponse>(
          SuggestProductNamesResponse.$_createMessage);
  static SuggestProductNamesResponse? _defaultInstance;

  /// 商品名稱，相關度高的在前，已去重。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get names => $_getList(0);
}

class GetProductRequest extends $pb.GeneratedMessage {
  factory GetProductRequest({
    $core.String? productId,
  }) {
    final result = GetProductRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  GetProductRequest._();

  factory GetProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductRequest()..mergeFromBuffer(data, registry);
  factory GetProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetProductRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductRequest copyWith(void Function(GetProductRequest) updates) =>
      super.copyWith((message) => updates(message as GetProductRequest))
          as GetProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetProductRequest() / GetProductRequest.new instead')
  static GetProductRequest create() => GetProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetProductRequest._();
  @$core.override
  GetProductRequest createEmptyInstance() => GetProductRequest._();
  @$core.pragma('dart2js:noInline')
  static GetProductRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetProductRequest>(
          GetProductRequest.$_createMessage);
  static GetProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class GetProductResponse extends $pb.GeneratedMessage {
  factory GetProductResponse({
    Product? product,
  }) {
    final result = GetProductResponse._();
    if (product != null) result.product = product;
    return result;
  }

  GetProductResponse._();

  factory GetProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductResponse()..mergeFromBuffer(data, registry);
  factory GetProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetProductResponse.$_createMessage)
    ..aOM<Product>(1, _omitFieldNames ? '' : 'product',
        subBuilder: Product.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductResponse copyWith(void Function(GetProductResponse) updates) =>
      super.copyWith((message) => updates(message as GetProductResponse))
          as GetProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetProductResponse() / GetProductResponse.new instead')
  static GetProductResponse create() => GetProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetProductResponse._();
  @$core.override
  GetProductResponse createEmptyInstance() => GetProductResponse._();
  @$core.pragma('dart2js:noInline')
  static GetProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProductResponse>(
          GetProductResponse.$_createMessage);
  static GetProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Product get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(Product value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  Product ensureProduct() => $_ensure(0);
}

class ListCategoriesRequest extends $pb.GeneratedMessage {
  factory ListCategoriesRequest() => ListCategoriesRequest._();

  ListCategoriesRequest._();

  factory ListCategoriesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCategoriesRequest()..mergeFromBuffer(data, registry);
  factory ListCategoriesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCategoriesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCategoriesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListCategoriesRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCategoriesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCategoriesRequest copyWith(
          void Function(ListCategoriesRequest) updates) =>
      super.copyWith((message) => updates(message as ListCategoriesRequest))
          as ListCategoriesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCategoriesRequest() / ListCategoriesRequest.new instead')
  static ListCategoriesRequest create() => ListCategoriesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCategoriesRequest._();
  @$core.override
  ListCategoriesRequest createEmptyInstance() => ListCategoriesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCategoriesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCategoriesRequest>(
          ListCategoriesRequest.$_createMessage);
  static ListCategoriesRequest? _defaultInstance;
}

class ListCategoriesResponse extends $pb.GeneratedMessage {
  factory ListCategoriesResponse({
    $core.Iterable<Category>? categories,
  }) {
    final result = ListCategoriesResponse._();
    if (categories != null) result.categories.addAll(categories);
    return result;
  }

  ListCategoriesResponse._();

  factory ListCategoriesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCategoriesResponse()..mergeFromBuffer(data, registry);
  factory ListCategoriesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCategoriesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCategoriesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListCategoriesResponse.$_createMessage)
    ..pPM<Category>(1, _omitFieldNames ? '' : 'categories',
        subBuilder: Category.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCategoriesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCategoriesResponse copyWith(
          void Function(ListCategoriesResponse) updates) =>
      super.copyWith((message) => updates(message as ListCategoriesResponse))
          as ListCategoriesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCategoriesResponse() / ListCategoriesResponse.new instead')
  static ListCategoriesResponse create() => ListCategoriesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCategoriesResponse._();
  @$core.override
  ListCategoriesResponse createEmptyInstance() => ListCategoriesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCategoriesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCategoriesResponse>(
          ListCategoriesResponse.$_createMessage);
  static ListCategoriesResponse? _defaultInstance;

  /// 頂層分類，每個帶底下的葉節點。
  @$pb.TagNumber(1)
  $pb.PbList<Category> get categories => $_getList(0);
}

/// Category 是平台分類樹的一個節點。商品只能掛在葉節點（沒有 children 的節點）。
/// 路徑（頂層 → 葉節點）由這棵樹推得，沒有另外的方法。Shopify 對照 ID 是內部資料，不對外。
class Category extends $pb.GeneratedMessage {
  factory Category({
    $core.String? id,
    $core.String? name,
    $core.Iterable<Category>? children,
  }) {
    final result = Category._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (children != null) result.children.addAll(children);
    return result;
  }

  Category._();

  factory Category.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Category()..mergeFromBuffer(data, registry);
  factory Category.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Category()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Category',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: Category.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..pPM<Category>(3, _omitFieldNames ? '' : 'children',
        subBuilder: Category.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Category clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Category copyWith(void Function(Category) updates) =>
      super.copyWith((message) => updates(message as Category)) as Category;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Category() / Category.new instead')
  static Category create() => Category._();
  static $pb.GeneratedMessage $_createMessage() => Category._();
  @$core.override
  Category createEmptyInstance() => Category._();
  @$core.pragma('dart2js:noInline')
  static Category getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Category>(Category.$_createMessage);
  static Category? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 依請求語系的名稱。
  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  /// 底下的葉節點；葉節點本身沒有。
  @$pb.TagNumber(3)
  $pb.PbList<Category> get children => $_getList(2);
}

class SuggestHashtagsRequest extends $pb.GeneratedMessage {
  factory SuggestHashtagsRequest({
    $core.String? query,
    $core.bool? contains,
    $core.int? maxResults,
  }) {
    final result = SuggestHashtagsRequest._();
    if (query != null) result.query = query;
    if (contains != null) result.contains = contains;
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestHashtagsRequest._();

  factory SuggestHashtagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestHashtagsRequest()..mergeFromBuffer(data, registry);
  factory SuggestHashtagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestHashtagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestHashtagsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestHashtagsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOB(2, _omitFieldNames ? '' : 'contains')
    ..aI(3, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestHashtagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestHashtagsRequest copyWith(
          void Function(SuggestHashtagsRequest) updates) =>
      super.copyWith((message) => updates(message as SuggestHashtagsRequest))
          as SuggestHashtagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestHashtagsRequest() / SuggestHashtagsRequest.new instead')
  static SuggestHashtagsRequest create() => SuggestHashtagsRequest._();
  static $pb.GeneratedMessage $_createMessage() => SuggestHashtagsRequest._();
  @$core.override
  SuggestHashtagsRequest createEmptyInstance() => SuggestHashtagsRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestHashtagsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestHashtagsRequest>(
          SuggestHashtagsRequest.$_createMessage);
  static SuggestHashtagsRequest? _defaultInstance;

  /// 搜尋字串，可以帶開頭的 #；正規化後是空的回空清單。
  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  /// true 是名稱包含 query，false 是以 query 開頭。
  @$pb.TagNumber(2)
  $core.bool get contains => $_getBF(1);
  @$pb.TagNumber(2)
  set contains($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContains() => $_has(1);
  @$pb.TagNumber(2)
  void clearContains() => $_clearField(2);

  /// 最多回傳幾筆，1 到 20；0 表示預設值 10，超過 20 以 20 計，負數回 invalid_argument。
  @$pb.TagNumber(3)
  $core.int get maxResults => $_getIZ(2);
  @$pb.TagNumber(3)
  set maxResults($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMaxResults() => $_has(2);
  @$pb.TagNumber(3)
  void clearMaxResults() => $_clearField(3);
}

class SuggestHashtagsResponse extends $pb.GeneratedMessage {
  factory SuggestHashtagsResponse({
    $core.Iterable<HashtagCount>? hashtags,
  }) {
    final result = SuggestHashtagsResponse._();
    if (hashtags != null) result.hashtags.addAll(hashtags);
    return result;
  }

  SuggestHashtagsResponse._();

  factory SuggestHashtagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestHashtagsResponse()..mergeFromBuffer(data, registry);
  factory SuggestHashtagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestHashtagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestHashtagsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestHashtagsResponse.$_createMessage)
    ..pPM<HashtagCount>(1, _omitFieldNames ? '' : 'hashtags',
        subBuilder: HashtagCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestHashtagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestHashtagsResponse copyWith(
          void Function(SuggestHashtagsResponse) updates) =>
      super.copyWith((message) => updates(message as SuggestHashtagsResponse))
          as SuggestHashtagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestHashtagsResponse() / SuggestHashtagsResponse.new instead')
  static SuggestHashtagsResponse create() => SuggestHashtagsResponse._();
  static $pb.GeneratedMessage $_createMessage() => SuggestHashtagsResponse._();
  @$core.override
  SuggestHashtagsResponse createEmptyInstance() => SuggestHashtagsResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestHashtagsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestHashtagsResponse>(
          SuggestHashtagsResponse.$_createMessage);
  static SuggestHashtagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<HashtagCount> get hashtags => $_getList(0);
}

class SuggestTrendingHashtagsRequest extends $pb.GeneratedMessage {
  factory SuggestTrendingHashtagsRequest({
    $core.int? maxResults,
  }) {
    final result = SuggestTrendingHashtagsRequest._();
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestTrendingHashtagsRequest._();

  factory SuggestTrendingHashtagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingHashtagsRequest()..mergeFromBuffer(data, registry);
  factory SuggestTrendingHashtagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingHashtagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingHashtagsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestTrendingHashtagsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingHashtagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingHashtagsRequest copyWith(
          void Function(SuggestTrendingHashtagsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingHashtagsRequest))
          as SuggestTrendingHashtagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingHashtagsRequest() / SuggestTrendingHashtagsRequest.new instead')
  static SuggestTrendingHashtagsRequest create() =>
      SuggestTrendingHashtagsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingHashtagsRequest._();
  @$core.override
  SuggestTrendingHashtagsRequest createEmptyInstance() =>
      SuggestTrendingHashtagsRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingHashtagsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingHashtagsRequest>(
          SuggestTrendingHashtagsRequest.$_createMessage);
  static SuggestTrendingHashtagsRequest? _defaultInstance;

  /// 最多回傳幾名，1 到 50；0 表示預設值 20，超過 50 以 50 計，負數回 invalid_argument。
  @$pb.TagNumber(1)
  $core.int get maxResults => $_getIZ(0);
  @$pb.TagNumber(1)
  set maxResults($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMaxResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearMaxResults() => $_clearField(1);
}

class SuggestTrendingHashtagsResponse extends $pb.GeneratedMessage {
  factory SuggestTrendingHashtagsResponse({
    $core.Iterable<HashtagCount>? hashtags,
  }) {
    final result = SuggestTrendingHashtagsResponse._();
    if (hashtags != null) result.hashtags.addAll(hashtags);
    return result;
  }

  SuggestTrendingHashtagsResponse._();

  factory SuggestTrendingHashtagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingHashtagsResponse()..mergeFromBuffer(data, registry);
  factory SuggestTrendingHashtagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingHashtagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingHashtagsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SuggestTrendingHashtagsResponse.$_createMessage)
    ..pPM<HashtagCount>(1, _omitFieldNames ? '' : 'hashtags',
        subBuilder: HashtagCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingHashtagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingHashtagsResponse copyWith(
          void Function(SuggestTrendingHashtagsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingHashtagsResponse))
          as SuggestTrendingHashtagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingHashtagsResponse() / SuggestTrendingHashtagsResponse.new instead')
  static SuggestTrendingHashtagsResponse create() =>
      SuggestTrendingHashtagsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingHashtagsResponse._();
  @$core.override
  SuggestTrendingHashtagsResponse createEmptyInstance() =>
      SuggestTrendingHashtagsResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingHashtagsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingHashtagsResponse>(
          SuggestTrendingHashtagsResponse.$_createMessage);
  static SuggestTrendingHashtagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<HashtagCount> get hashtags => $_getList(0);
}

/// HashtagCount 是一個商品標籤與它在範圍內的商品數。
class HashtagCount extends $pb.GeneratedMessage {
  factory HashtagCount({
    $core.String? name,
    $core.String? display,
    $fixnum.Int64? productCount,
  }) {
    final result = HashtagCount._();
    if (name != null) result.name = name;
    if (display != null) result.display = display;
    if (productCount != null) result.productCount = productCount;
    return result;
  }

  HashtagCount._();

  factory HashtagCount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HashtagCount()..mergeFromBuffer(data, registry);
  factory HashtagCount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HashtagCount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HashtagCount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: HashtagCount.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'display')
    ..aInt64(3, _omitFieldNames ? '' : 'productCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HashtagCount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HashtagCount copyWith(void Function(HashtagCount) updates) =>
      super.copyWith((message) => updates(message as HashtagCount))
          as HashtagCount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HashtagCount() / HashtagCount.new instead')
  static HashtagCount create() => HashtagCount._();
  static $pb.GeneratedMessage $_createMessage() => HashtagCount._();
  @$core.override
  HashtagCount createEmptyInstance() => HashtagCount._();
  @$core.pragma('dart2js:noInline')
  static HashtagCount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<HashtagCount>(
          HashtagCount.$_createMessage);
  static HashtagCount? _defaultInstance;

  /// 正規化後的名稱，用來比對與連結。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// 最多商品使用的輸入樣子，用來顯示。
  @$pb.TagNumber(2)
  $core.String get display => $_getSZ(1);
  @$pb.TagNumber(2)
  set display($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplay() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplay() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get productCount => $_getI64(2);
  @$pb.TagNumber(3)
  set productCount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProductCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearProductCount() => $_clearField(3);
}

/// MyProductFilter 是賣家清單與計數共用的篩選，全部選填，之間是 AND。
class MyProductFilter extends $pb.GeneratedMessage {
  factory MyProductFilter({
    ProductStatus? status,
    StockStatus? stockStatus,
    $core.String? query,
  }) {
    final result = MyProductFilter._();
    if (status != null) result.status = status;
    if (stockStatus != null) result.stockStatus = stockStatus;
    if (query != null) result.query = query;
    return result;
  }

  MyProductFilter._();

  factory MyProductFilter.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyProductFilter()..mergeFromBuffer(data, registry);
  factory MyProductFilter.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MyProductFilter()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MyProductFilter',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: MyProductFilter.$_createMessage)
    ..aE<ProductStatus>(1, _omitFieldNames ? '' : 'status',
        enumValues: ProductStatus.values)
    ..aE<StockStatus>(2, _omitFieldNames ? '' : 'stockStatus',
        enumValues: StockStatus.values)
    ..aOS(3, _omitFieldNames ? '' : 'query')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyProductFilter clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MyProductFilter copyWith(void Function(MyProductFilter) updates) =>
      super.copyWith((message) => updates(message as MyProductFilter))
          as MyProductFilter;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MyProductFilter() / MyProductFilter.new instead')
  static MyProductFilter create() => MyProductFilter._();
  static $pb.GeneratedMessage $_createMessage() => MyProductFilter._();
  @$core.override
  MyProductFilter createEmptyInstance() => MyProductFilter._();
  @$core.pragma('dart2js:noInline')
  static MyProductFilter getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MyProductFilter>(
          MyProductFilter.$_createMessage);
  static MyProductFilter? _defaultInstance;

  /// 只列這個狀態的商品；UNSPECIFIED 是全部。
  @$pb.TagNumber(1)
  ProductStatus get status => $_getN(0);
  @$pb.TagNumber(1)
  set status(ProductStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  /// 依庫存篩選；LOW 與 OUT_OF_STOCK 只比對 ACTIVE 的商品（與 status 同時給了非 ACTIVE 的狀態就沒有結果）。
  @$pb.TagNumber(2)
  StockStatus get stockStatus => $_getN(1);
  @$pb.TagNumber(2)
  set stockStatus(StockStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStockStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStockStatus() => $_clearField(2);

  /// 關鍵字，比對標題、描述、標籤、分類與選項（同公開搜尋的 searchable_text）；去頭尾空白後空的是不篩選。
  @$pb.TagNumber(3)
  $core.String get query => $_getSZ(2);
  @$pb.TagNumber(3)
  set query($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQuery() => $_has(2);
  @$pb.TagNumber(3)
  void clearQuery() => $_clearField(3);
}

class ListMyProductsRequest extends $pb.GeneratedMessage {
  factory ListMyProductsRequest({
    MyProductFilter? filter,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMyProductsRequest._();
    if (filter != null) result.filter = filter;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMyProductsRequest._();

  factory ListMyProductsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyProductsRequest()..mergeFromBuffer(data, registry);
  factory ListMyProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListMyProductsRequest.$_createMessage)
    ..aOM<MyProductFilter>(1, _omitFieldNames ? '' : 'filter',
        subBuilder: MyProductFilter.$_createMessage)
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProductsRequest copyWith(
          void Function(ListMyProductsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyProductsRequest))
          as ListMyProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyProductsRequest() / ListMyProductsRequest.new instead')
  static ListMyProductsRequest create() => ListMyProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyProductsRequest._();
  @$core.override
  ListMyProductsRequest createEmptyInstance() => ListMyProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyProductsRequest>(
          ListMyProductsRequest.$_createMessage);
  static ListMyProductsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  MyProductFilter get filter => $_getN(0);
  @$pb.TagNumber(1)
  set filter(MyProductFilter value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearFilter() => $_clearField(1);
  @$pb.TagNumber(1)
  MyProductFilter ensureFilter() => $_ensure(0);

  /// 預設 20，上限 100。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  /// page_token 與 filter 綁定。
  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);
}

class ListMyProductsResponse extends $pb.GeneratedMessage {
  factory ListMyProductsResponse({
    $core.Iterable<SellerProductSummary>? products,
    $core.String? nextPageToken,
  }) {
    final result = ListMyProductsResponse._();
    if (products != null) result.products.addAll(products);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyProductsResponse._();

  factory ListMyProductsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyProductsResponse()..mergeFromBuffer(data, registry);
  factory ListMyProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListMyProductsResponse.$_createMessage)
    ..pPM<SellerProductSummary>(1, _omitFieldNames ? '' : 'products',
        subBuilder: SellerProductSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProductsResponse copyWith(
          void Function(ListMyProductsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyProductsResponse))
          as ListMyProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyProductsResponse() / ListMyProductsResponse.new instead')
  static ListMyProductsResponse create() => ListMyProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyProductsResponse._();
  @$core.override
  ListMyProductsResponse createEmptyInstance() => ListMyProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyProductsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyProductsResponse>(
          ListMyProductsResponse.$_createMessage);
  static ListMyProductsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SellerProductSummary> get products => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetMyProductRequest extends $pb.GeneratedMessage {
  factory GetMyProductRequest({
    $core.String? productId,
  }) {
    final result = GetMyProductRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  GetMyProductRequest._();

  factory GetMyProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyProductRequest()..mergeFromBuffer(data, registry);
  factory GetMyProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetMyProductRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyProductRequest copyWith(void Function(GetMyProductRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyProductRequest))
          as GetMyProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetMyProductRequest() / GetMyProductRequest.new instead')
  static GetMyProductRequest create() => GetMyProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyProductRequest._();
  @$core.override
  GetMyProductRequest createEmptyInstance() => GetMyProductRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyProductRequest>(
          GetMyProductRequest.$_createMessage);
  static GetMyProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class GetMyProductResponse extends $pb.GeneratedMessage {
  factory GetMyProductResponse({
    SellerProduct? product,
  }) {
    final result = GetMyProductResponse._();
    if (product != null) result.product = product;
    return result;
  }

  GetMyProductResponse._();

  factory GetMyProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyProductResponse()..mergeFromBuffer(data, registry);
  factory GetMyProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetMyProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyProductResponse copyWith(void Function(GetMyProductResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyProductResponse))
          as GetMyProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyProductResponse() / GetMyProductResponse.new instead')
  static GetMyProductResponse create() => GetMyProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyProductResponse._();
  @$core.override
  GetMyProductResponse createEmptyInstance() => GetMyProductResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyProductResponse>(
          GetMyProductResponse.$_createMessage);
  static GetMyProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);
}

class CountMyProductsRequest extends $pb.GeneratedMessage {
  factory CountMyProductsRequest({
    MyProductFilter? filter,
  }) {
    final result = CountMyProductsRequest._();
    if (filter != null) result.filter = filter;
    return result;
  }

  CountMyProductsRequest._();

  factory CountMyProductsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CountMyProductsRequest()..mergeFromBuffer(data, registry);
  factory CountMyProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CountMyProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CountMyProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CountMyProductsRequest.$_createMessage)
    ..aOM<MyProductFilter>(1, _omitFieldNames ? '' : 'filter',
        subBuilder: MyProductFilter.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CountMyProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CountMyProductsRequest copyWith(
          void Function(CountMyProductsRequest) updates) =>
      super.copyWith((message) => updates(message as CountMyProductsRequest))
          as CountMyProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CountMyProductsRequest() / CountMyProductsRequest.new instead')
  static CountMyProductsRequest create() => CountMyProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() => CountMyProductsRequest._();
  @$core.override
  CountMyProductsRequest createEmptyInstance() => CountMyProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static CountMyProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CountMyProductsRequest>(
          CountMyProductsRequest.$_createMessage);
  static CountMyProductsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  MyProductFilter get filter => $_getN(0);
  @$pb.TagNumber(1)
  set filter(MyProductFilter value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearFilter() => $_clearField(1);
  @$pb.TagNumber(1)
  MyProductFilter ensureFilter() => $_ensure(0);
}

class CountMyProductsResponse extends $pb.GeneratedMessage {
  factory CountMyProductsResponse({
    $core.int? count,
    $core.int? defaultLowStockThreshold,
  }) {
    final result = CountMyProductsResponse._();
    if (count != null) result.count = count;
    if (defaultLowStockThreshold != null)
      result.defaultLowStockThreshold = defaultLowStockThreshold;
    return result;
  }

  CountMyProductsResponse._();

  factory CountMyProductsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CountMyProductsResponse()..mergeFromBuffer(data, registry);
  factory CountMyProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CountMyProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CountMyProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CountMyProductsResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'count')
    ..aI(2, _omitFieldNames ? '' : 'defaultLowStockThreshold')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CountMyProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CountMyProductsResponse copyWith(
          void Function(CountMyProductsResponse) updates) =>
      super.copyWith((message) => updates(message as CountMyProductsResponse))
          as CountMyProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CountMyProductsResponse() / CountMyProductsResponse.new instead')
  static CountMyProductsResponse create() => CountMyProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() => CountMyProductsResponse._();
  @$core.override
  CountMyProductsResponse createEmptyInstance() => CountMyProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static CountMyProductsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CountMyProductsResponse>(
          CountMyProductsResponse.$_createMessage);
  static CountMyProductsResponse? _defaultInstance;

  /// 符合篩選的商品數。
  @$pb.TagNumber(1)
  $core.int get count => $_getIZ(0);
  @$pb.TagNumber(1)
  set count($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearCount() => $_clearField(1);

  /// 商品沒有自己的低庫存門檻時用的平台預設值，client 顯示用，不自己寫死數字。
  @$pb.TagNumber(2)
  $core.int get defaultLowStockThreshold => $_getIZ(1);
  @$pb.TagNumber(2)
  set defaultLowStockThreshold($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDefaultLowStockThreshold() => $_has(1);
  @$pb.TagNumber(2)
  void clearDefaultLowStockThreshold() => $_clearField(2);
}

class CreateProductRequest extends $pb.GeneratedMessage {
  factory CreateProductRequest({
    ProductInput? product,
    $core.String? requestId,
  }) {
    final result = CreateProductRequest._();
    if (product != null) result.product = product;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  CreateProductRequest._();

  factory CreateProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateProductRequest()..mergeFromBuffer(data, registry);
  factory CreateProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CreateProductRequest.$_createMessage)
    ..aOM<ProductInput>(1, _omitFieldNames ? '' : 'product',
        subBuilder: ProductInput.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProductRequest copyWith(void Function(CreateProductRequest) updates) =>
      super.copyWith((message) => updates(message as CreateProductRequest))
          as CreateProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateProductRequest() / CreateProductRequest.new instead')
  static CreateProductRequest create() => CreateProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateProductRequest._();
  @$core.override
  CreateProductRequest createEmptyInstance() => CreateProductRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateProductRequest>(
          CreateProductRequest.$_createMessage);
  static CreateProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ProductInput get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(ProductInput value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  ProductInput ensureProduct() => $_ensure(0);

  /// UUIDv4（AIP-155）：範圍是（店家、方法），永久保存；同一家店以同一個 request_id 重送回第一次建立的商品目前的樣子，
  /// 兩個請求同時在途也只建立一個；內容不同回 invalid_argument。必填。
  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);
}

class CreateProductResponse extends $pb.GeneratedMessage {
  factory CreateProductResponse({
    SellerProduct? product,
  }) {
    final result = CreateProductResponse._();
    if (product != null) result.product = product;
    return result;
  }

  CreateProductResponse._();

  factory CreateProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateProductResponse()..mergeFromBuffer(data, registry);
  factory CreateProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CreateProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProductResponse copyWith(
          void Function(CreateProductResponse) updates) =>
      super.copyWith((message) => updates(message as CreateProductResponse))
          as CreateProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateProductResponse() / CreateProductResponse.new instead')
  static CreateProductResponse create() => CreateProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateProductResponse._();
  @$core.override
  CreateProductResponse createEmptyInstance() => CreateProductResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateProductResponse>(
          CreateProductResponse.$_createMessage);
  static CreateProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);
}

class UpdateProductRequest extends $pb.GeneratedMessage {
  factory UpdateProductRequest({
    $core.String? productId,
    $core.String? etag,
    ProductInput? product,
  }) {
    final result = UpdateProductRequest._();
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    if (product != null) result.product = product;
    return result;
  }

  UpdateProductRequest._();

  factory UpdateProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProductRequest()..mergeFromBuffer(data, registry);
  factory UpdateProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UpdateProductRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..aOM<ProductInput>(3, _omitFieldNames ? '' : 'product',
        subBuilder: ProductInput.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProductRequest copyWith(void Function(UpdateProductRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateProductRequest))
          as UpdateProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateProductRequest() / UpdateProductRequest.new instead')
  static UpdateProductRequest create() => UpdateProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateProductRequest._();
  @$core.override
  UpdateProductRequest createEmptyInstance() => UpdateProductRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProductRequest>(
          UpdateProductRequest.$_createMessage);
  static UpdateProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);

  /// 完整內容，整體取代；沒有 update_mask。
  @$pb.TagNumber(3)
  ProductInput get product => $_getN(2);
  @$pb.TagNumber(3)
  set product(ProductInput value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasProduct() => $_has(2);
  @$pb.TagNumber(3)
  void clearProduct() => $_clearField(3);
  @$pb.TagNumber(3)
  ProductInput ensureProduct() => $_ensure(2);
}

/// UpdateProductResponse 的商品是修改後的完整資源（含新的 etag）。
class UpdateProductResponse extends $pb.GeneratedMessage {
  factory UpdateProductResponse({
    SellerProduct? product,
  }) {
    final result = UpdateProductResponse._();
    if (product != null) result.product = product;
    return result;
  }

  UpdateProductResponse._();

  factory UpdateProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProductResponse()..mergeFromBuffer(data, registry);
  factory UpdateProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UpdateProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProductResponse copyWith(
          void Function(UpdateProductResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateProductResponse))
          as UpdateProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateProductResponse() / UpdateProductResponse.new instead')
  static UpdateProductResponse create() => UpdateProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateProductResponse._();
  @$core.override
  UpdateProductResponse createEmptyInstance() => UpdateProductResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProductResponse>(
          UpdateProductResponse.$_createMessage);
  static UpdateProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);
}

class SetProductStatusRequest extends $pb.GeneratedMessage {
  factory SetProductStatusRequest({
    $core.String? productId,
    ProductStatus? status,
    $core.String? etag,
  }) {
    final result = SetProductStatusRequest._();
    if (productId != null) result.productId = productId;
    if (status != null) result.status = status;
    if (etag != null) result.etag = etag;
    return result;
  }

  SetProductStatusRequest._();

  factory SetProductStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetProductStatusRequest()..mergeFromBuffer(data, registry);
  factory SetProductStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetProductStatusRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetProductStatusRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SetProductStatusRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aE<ProductStatus>(2, _omitFieldNames ? '' : 'status',
        enumValues: ProductStatus.values)
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetProductStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetProductStatusRequest copyWith(
          void Function(SetProductStatusRequest) updates) =>
      super.copyWith((message) => updates(message as SetProductStatusRequest))
          as SetProductStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetProductStatusRequest() / SetProductStatusRequest.new instead')
  static SetProductStatusRequest create() => SetProductStatusRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetProductStatusRequest._();
  @$core.override
  SetProductStatusRequest createEmptyInstance() => SetProductStatusRequest._();
  @$core.pragma('dart2js:noInline')
  static SetProductStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetProductStatusRequest>(
          SetProductStatusRequest.$_createMessage);
  static SetProductStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 目標狀態，不得是 UNSPECIFIED。
  @$pb.TagNumber(2)
  ProductStatus get status => $_getN(1);
  @$pb.TagNumber(2)
  set status(ProductStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class SetProductStatusResponse extends $pb.GeneratedMessage {
  factory SetProductStatusResponse({
    SellerProduct? product,
  }) {
    final result = SetProductStatusResponse._();
    if (product != null) result.product = product;
    return result;
  }

  SetProductStatusResponse._();

  factory SetProductStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetProductStatusResponse()..mergeFromBuffer(data, registry);
  factory SetProductStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetProductStatusResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetProductStatusResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SetProductStatusResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetProductStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetProductStatusResponse copyWith(
          void Function(SetProductStatusResponse) updates) =>
      super.copyWith((message) => updates(message as SetProductStatusResponse))
          as SetProductStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetProductStatusResponse() / SetProductStatusResponse.new instead')
  static SetProductStatusResponse create() => SetProductStatusResponse._();
  static $pb.GeneratedMessage $_createMessage() => SetProductStatusResponse._();
  @$core.override
  SetProductStatusResponse createEmptyInstance() =>
      SetProductStatusResponse._();
  @$core.pragma('dart2js:noInline')
  static SetProductStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetProductStatusResponse>(
          SetProductStatusResponse.$_createMessage);
  static SetProductStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);
}

class DeleteProductRequest extends $pb.GeneratedMessage {
  factory DeleteProductRequest({
    $core.String? productId,
    $core.String? etag,
  }) {
    final result = DeleteProductRequest._();
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    return result;
  }

  DeleteProductRequest._();

  factory DeleteProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteProductRequest()..mergeFromBuffer(data, registry);
  factory DeleteProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: DeleteProductRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProductRequest copyWith(void Function(DeleteProductRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteProductRequest))
          as DeleteProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteProductRequest() / DeleteProductRequest.new instead')
  static DeleteProductRequest create() => DeleteProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteProductRequest._();
  @$core.override
  DeleteProductRequest createEmptyInstance() => DeleteProductRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteProductRequest>(
          DeleteProductRequest.$_createMessage);
  static DeleteProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 選填；帶了就與目前版本比對。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class DeleteProductResponse extends $pb.GeneratedMessage {
  factory DeleteProductResponse() => DeleteProductResponse._();

  DeleteProductResponse._();

  factory DeleteProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteProductResponse()..mergeFromBuffer(data, registry);
  factory DeleteProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: DeleteProductResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProductResponse copyWith(
          void Function(DeleteProductResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteProductResponse))
          as DeleteProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteProductResponse() / DeleteProductResponse.new instead')
  static DeleteProductResponse create() => DeleteProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteProductResponse._();
  @$core.override
  DeleteProductResponse createEmptyInstance() => DeleteProductResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteProductResponse>(
          DeleteProductResponse.$_createMessage);
  static DeleteProductResponse? _defaultInstance;
}

class AdjustStockRequest extends $pb.GeneratedMessage {
  factory AdjustStockRequest({
    $core.String? skuId,
    $core.int? delta,
    $core.String? requestId,
  }) {
    final result = AdjustStockRequest._();
    if (skuId != null) result.skuId = skuId;
    if (delta != null) result.delta = delta;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  AdjustStockRequest._();

  factory AdjustStockRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AdjustStockRequest()..mergeFromBuffer(data, registry);
  factory AdjustStockRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AdjustStockRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AdjustStockRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: AdjustStockRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'skuId')
    ..aI(2, _omitFieldNames ? '' : 'delta')
    ..aOS(3, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AdjustStockRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AdjustStockRequest copyWith(void Function(AdjustStockRequest) updates) =>
      super.copyWith((message) => updates(message as AdjustStockRequest))
          as AdjustStockRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AdjustStockRequest() / AdjustStockRequest.new instead')
  static AdjustStockRequest create() => AdjustStockRequest._();
  static $pb.GeneratedMessage $_createMessage() => AdjustStockRequest._();
  @$core.override
  AdjustStockRequest createEmptyInstance() => AdjustStockRequest._();
  @$core.pragma('dart2js:noInline')
  static AdjustStockRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AdjustStockRequest>(
          AdjustStockRequest.$_createMessage);
  static AdjustStockRequest? _defaultInstance;

  /// 要調整的 SKU；不屬於自己店家或已刪除回 not_found。
  @$pb.TagNumber(1)
  $core.String get skuId => $_getSZ(0);
  @$pb.TagNumber(1)
  set skuId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSkuId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSkuId() => $_clearField(1);

  /// 有號變動量，不得為 0；調整後的庫存必須在 0 到 2147483647 之間。
  @$pb.TagNumber(2)
  $core.int get delta => $_getIZ(1);
  @$pb.TagNumber(2)
  set delta($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDelta() => $_has(1);
  @$pb.TagNumber(2)
  void clearDelta() => $_clearField(2);

  /// UUIDv4（AIP-155）：範圍是（店家、方法），永久保存；同一家店以同一個 request_id 重送且內容相同只生效一次，同時在途也只生效一次；內容不同回 invalid_argument。必填：變動量重送會重複計算。
  @$pb.TagNumber(3)
  $core.String get requestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set requestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRequestId() => $_clearField(3);
}

class AdjustStockResponse extends $pb.GeneratedMessage {
  factory AdjustStockResponse({
    Sku? sku,
  }) {
    final result = AdjustStockResponse._();
    if (sku != null) result.sku = sku;
    return result;
  }

  AdjustStockResponse._();

  factory AdjustStockResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AdjustStockResponse()..mergeFromBuffer(data, registry);
  factory AdjustStockResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AdjustStockResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AdjustStockResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: AdjustStockResponse.$_createMessage)
    ..aOM<Sku>(1, _omitFieldNames ? '' : 'sku', subBuilder: Sku.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AdjustStockResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AdjustStockResponse copyWith(void Function(AdjustStockResponse) updates) =>
      super.copyWith((message) => updates(message as AdjustStockResponse))
          as AdjustStockResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use AdjustStockResponse() / AdjustStockResponse.new instead')
  static AdjustStockResponse create() => AdjustStockResponse._();
  static $pb.GeneratedMessage $_createMessage() => AdjustStockResponse._();
  @$core.override
  AdjustStockResponse createEmptyInstance() => AdjustStockResponse._();
  @$core.pragma('dart2js:noInline')
  static AdjustStockResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AdjustStockResponse>(
          AdjustStockResponse.$_createMessage);
  static AdjustStockResponse? _defaultInstance;

  /// 調整後的 SKU（含新的庫存）。
  @$pb.TagNumber(1)
  Sku get sku => $_getN(0);
  @$pb.TagNumber(1)
  set sku(Sku value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSku() => $_has(0);
  @$pb.TagNumber(1)
  void clearSku() => $_clearField(1);
  @$pb.TagNumber(1)
  Sku ensureSku() => $_ensure(0);
}

class SetStockRequest extends $pb.GeneratedMessage {
  factory SetStockRequest({
    $core.String? skuId,
    $core.int? quantity,
    $core.int? expectedQuantity,
    $core.String? requestId,
  }) {
    final result = SetStockRequest._();
    if (skuId != null) result.skuId = skuId;
    if (quantity != null) result.quantity = quantity;
    if (expectedQuantity != null) result.expectedQuantity = expectedQuantity;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  SetStockRequest._();

  factory SetStockRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetStockRequest()..mergeFromBuffer(data, registry);
  factory SetStockRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetStockRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetStockRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SetStockRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'skuId')
    ..aI(2, _omitFieldNames ? '' : 'quantity')
    ..aI(3, _omitFieldNames ? '' : 'expectedQuantity')
    ..aOS(4, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetStockRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetStockRequest copyWith(void Function(SetStockRequest) updates) =>
      super.copyWith((message) => updates(message as SetStockRequest))
          as SetStockRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SetStockRequest() / SetStockRequest.new instead')
  static SetStockRequest create() => SetStockRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetStockRequest._();
  @$core.override
  SetStockRequest createEmptyInstance() => SetStockRequest._();
  @$core.pragma('dart2js:noInline')
  static SetStockRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SetStockRequest>(
          SetStockRequest.$_createMessage);
  static SetStockRequest? _defaultInstance;

  /// 要設定的 SKU；不屬於自己店家或已刪除回 not_found。
  @$pb.TagNumber(1)
  $core.String get skuId => $_getSZ(0);
  @$pb.TagNumber(1)
  set skuId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSkuId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSkuId() => $_clearField(1);

  /// 新的庫存，0 到 2147483647。
  @$pb.TagNumber(2)
  $core.int get quantity => $_getIZ(1);
  @$pb.TagNumber(2)
  set quantity($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasQuantity() => $_has(1);
  @$pb.TagNumber(2)
  void clearQuantity() => $_clearField(2);

  /// 呼叫者看到的現值；與目前的庫存不等回 failed_precondition。
  @$pb.TagNumber(3)
  $core.int get expectedQuantity => $_getIZ(2);
  @$pb.TagNumber(3)
  set expectedQuantity($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedQuantity() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedQuantity() => $_clearField(3);

  /// UUIDv4（AIP-155）：範圍與冪等語意同 AdjustStock。必填。
  @$pb.TagNumber(4)
  $core.String get requestId => $_getSZ(3);
  @$pb.TagNumber(4)
  set requestId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRequestId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRequestId() => $_clearField(4);
}

class SetStockResponse extends $pb.GeneratedMessage {
  factory SetStockResponse({
    Sku? sku,
  }) {
    final result = SetStockResponse._();
    if (sku != null) result.sku = sku;
    return result;
  }

  SetStockResponse._();

  factory SetStockResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetStockResponse()..mergeFromBuffer(data, registry);
  factory SetStockResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetStockResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetStockResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SetStockResponse.$_createMessage)
    ..aOM<Sku>(1, _omitFieldNames ? '' : 'sku', subBuilder: Sku.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetStockResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetStockResponse copyWith(void Function(SetStockResponse) updates) =>
      super.copyWith((message) => updates(message as SetStockResponse))
          as SetStockResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SetStockResponse() / SetStockResponse.new instead')
  static SetStockResponse create() => SetStockResponse._();
  static $pb.GeneratedMessage $_createMessage() => SetStockResponse._();
  @$core.override
  SetStockResponse createEmptyInstance() => SetStockResponse._();
  @$core.pragma('dart2js:noInline')
  static SetStockResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SetStockResponse>(
          SetStockResponse.$_createMessage);
  static SetStockResponse? _defaultInstance;

  /// 設定後的 SKU。
  @$pb.TagNumber(1)
  Sku get sku => $_getN(0);
  @$pb.TagNumber(1)
  set sku(Sku value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSku() => $_has(0);
  @$pb.TagNumber(1)
  void clearSku() => $_clearField(1);
  @$pb.TagNumber(1)
  Sku ensureSku() => $_ensure(0);
}

/// ProductImage 是商品的一張圖（圖庫或描述圖片）。
class ProductImage extends $pb.GeneratedMessage {
  factory ProductImage({
    $core.String? assetId,
    $0.Image? image,
    $core.String? alt,
  }) {
    final result = ProductImage._();
    if (assetId != null) result.assetId = assetId;
    if (image != null) result.image = image;
    if (alt != null) result.alt = alt;
    return result;
  }

  ProductImage._();

  factory ProductImage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductImage()..mergeFromBuffer(data, registry);
  factory ProductImage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductImage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductImage',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductImage.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'assetId')
    ..aOM<$0.Image>(2, _omitFieldNames ? '' : 'image',
        subBuilder: $0.Image.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'alt')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductImage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductImage copyWith(void Function(ProductImage) updates) =>
      super.copyWith((message) => updates(message as ProductImage))
          as ProductImage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductImage() / ProductImage.new instead')
  static ProductImage create() => ProductImage._();
  static $pb.GeneratedMessage $_createMessage() => ProductImage._();
  @$core.override
  ProductImage createEmptyInstance() => ProductImage._();
  @$core.pragma('dart2js:noInline')
  static ProductImage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductImage>(
          ProductImage.$_createMessage);
  static ProductImage? _defaultInstance;

  /// 圖片資產 ID；只有賣家讀自己的商品才設（寫入沿用這張圖時放進 ImageInput.asset_id），公開讀取不設。
  @$pb.TagNumber(1)
  $core.String get assetId => $_getSZ(0);
  @$pb.TagNumber(1)
  set assetId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAssetId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAssetId() => $_clearField(1);

  @$pb.TagNumber(2)
  $0.Image get image => $_getN(1);
  @$pb.TagNumber(2)
  set image($0.Image value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasImage() => $_has(1);
  @$pb.TagNumber(2)
  void clearImage() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Image ensureImage() => $_ensure(1);

  /// 替代文字，可空。
  @$pb.TagNumber(3)
  $core.String get alt => $_getSZ(2);
  @$pb.TagNumber(3)
  set alt($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAlt() => $_has(2);
  @$pb.TagNumber(3)
  void clearAlt() => $_clearField(3);
}

enum ProductDescriptionBlock_Block {
  paragraph,
  image,
  bulletList,
  table,
  notSet
}

/// ProductDescriptionBlock 是描述的一個區塊（輸出），剛好設一種；形狀對應寫入的 DescriptionBlock。
class ProductDescriptionBlock extends $pb.GeneratedMessage {
  factory ProductDescriptionBlock({
    $core.String? paragraph,
    ProductImage? image,
    BulletList? bulletList,
    Table? table,
  }) {
    final result = ProductDescriptionBlock._();
    if (paragraph != null) result.paragraph = paragraph;
    if (image != null) result.image = image;
    if (bulletList != null) result.bulletList = bulletList;
    if (table != null) result.table = table;
    return result;
  }

  ProductDescriptionBlock._();

  factory ProductDescriptionBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductDescriptionBlock()..mergeFromBuffer(data, registry);
  factory ProductDescriptionBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductDescriptionBlock()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ProductDescriptionBlock_Block>
      _ProductDescriptionBlock_BlockByTag = {
    1: ProductDescriptionBlock_Block.paragraph,
    2: ProductDescriptionBlock_Block.image,
    3: ProductDescriptionBlock_Block.bulletList,
    4: ProductDescriptionBlock_Block.table,
    0: ProductDescriptionBlock_Block.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductDescriptionBlock',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductDescriptionBlock.$_createMessage)
    ..oo(0, [1, 2, 3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'paragraph')
    ..aOM<ProductImage>(2, _omitFieldNames ? '' : 'image',
        subBuilder: ProductImage.$_createMessage)
    ..aOM<BulletList>(3, _omitFieldNames ? '' : 'bulletList',
        subBuilder: BulletList.$_createMessage)
    ..aOM<Table>(4, _omitFieldNames ? '' : 'table',
        subBuilder: Table.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductDescriptionBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductDescriptionBlock copyWith(
          void Function(ProductDescriptionBlock) updates) =>
      super.copyWith((message) => updates(message as ProductDescriptionBlock))
          as ProductDescriptionBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ProductDescriptionBlock() / ProductDescriptionBlock.new instead')
  static ProductDescriptionBlock create() => ProductDescriptionBlock._();
  static $pb.GeneratedMessage $_createMessage() => ProductDescriptionBlock._();
  @$core.override
  ProductDescriptionBlock createEmptyInstance() => ProductDescriptionBlock._();
  @$core.pragma('dart2js:noInline')
  static ProductDescriptionBlock getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProductDescriptionBlock>(
          ProductDescriptionBlock.$_createMessage);
  static ProductDescriptionBlock? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  ProductDescriptionBlock_Block whichBlock() =>
      _ProductDescriptionBlock_BlockByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearBlock() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get paragraph => $_getSZ(0);
  @$pb.TagNumber(1)
  set paragraph($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasParagraph() => $_has(0);
  @$pb.TagNumber(1)
  void clearParagraph() => $_clearField(1);

  @$pb.TagNumber(2)
  ProductImage get image => $_getN(1);
  @$pb.TagNumber(2)
  set image(ProductImage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasImage() => $_has(1);
  @$pb.TagNumber(2)
  void clearImage() => $_clearField(2);
  @$pb.TagNumber(2)
  ProductImage ensureImage() => $_ensure(1);

  @$pb.TagNumber(3)
  BulletList get bulletList => $_getN(2);
  @$pb.TagNumber(3)
  set bulletList(BulletList value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBulletList() => $_has(2);
  @$pb.TagNumber(3)
  void clearBulletList() => $_clearField(3);
  @$pb.TagNumber(3)
  BulletList ensureBulletList() => $_ensure(2);

  @$pb.TagNumber(4)
  Table get table => $_getN(3);
  @$pb.TagNumber(4)
  set table(Table value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasTable() => $_has(3);
  @$pb.TagNumber(4)
  void clearTable() => $_clearField(4);
  @$pb.TagNumber(4)
  Table ensureTable() => $_ensure(3);
}

/// OptionAxis 是一個選項軸（輸出）。
class OptionAxis extends $pb.GeneratedMessage {
  factory OptionAxis({
    $core.String? name,
    $core.Iterable<$core.String>? values,
  }) {
    final result = OptionAxis._();
    if (name != null) result.name = name;
    if (values != null) result.values.addAll(values);
    return result;
  }

  OptionAxis._();

  factory OptionAxis.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxis()..mergeFromBuffer(data, registry);
  factory OptionAxis.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxis()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OptionAxis',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: OptionAxis.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..pPS(2, _omitFieldNames ? '' : 'values')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxis clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxis copyWith(void Function(OptionAxis) updates) =>
      super.copyWith((message) => updates(message as OptionAxis)) as OptionAxis;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OptionAxis() / OptionAxis.new instead')
  static OptionAxis create() => OptionAxis._();
  static $pb.GeneratedMessage $_createMessage() => OptionAxis._();
  @$core.override
  OptionAxis createEmptyInstance() => OptionAxis._();
  @$core.pragma('dart2js:noInline')
  static OptionAxis getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OptionAxis>(OptionAxis.$_createMessage);
  static OptionAxis? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get values => $_getList(1);
}

/// Sku 是商品的一個 SKU。沒有該軸的位置，option 是空字串。
class Sku extends $pb.GeneratedMessage {
  factory Sku({
    $core.String? id,
    $core.String? option1,
    $core.String? option2,
    $fixnum.Int64? price,
    $fixnum.Int64? compareAtPrice,
    $core.int? stockQty,
    ProductAvailability? availability,
  }) {
    final result = Sku._();
    if (id != null) result.id = id;
    if (option1 != null) result.option1 = option1;
    if (option2 != null) result.option2 = option2;
    if (price != null) result.price = price;
    if (compareAtPrice != null) result.compareAtPrice = compareAtPrice;
    if (stockQty != null) result.stockQty = stockQty;
    if (availability != null) result.availability = availability;
    return result;
  }

  Sku._();

  factory Sku.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Sku()..mergeFromBuffer(data, registry);
  factory Sku.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Sku()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Sku',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: Sku.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'option1')
    ..aOS(3, _omitFieldNames ? '' : 'option2')
    ..aInt64(4, _omitFieldNames ? '' : 'price')
    ..aInt64(5, _omitFieldNames ? '' : 'compareAtPrice')
    ..aI(6, _omitFieldNames ? '' : 'stockQty')
    ..aE<ProductAvailability>(7, _omitFieldNames ? '' : 'availability',
        enumValues: ProductAvailability.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sku clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sku copyWith(void Function(Sku) updates) =>
      super.copyWith((message) => updates(message as Sku)) as Sku;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Sku() / Sku.new instead')
  static Sku create() => Sku._();
  static $pb.GeneratedMessage $_createMessage() => Sku._();
  @$core.override
  Sku createEmptyInstance() => Sku._();
  @$core.pragma('dart2js:noInline')
  static Sku getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Sku>(Sku.$_createMessage);
  static Sku? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get option1 => $_getSZ(1);
  @$pb.TagNumber(2)
  set option1($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOption1() => $_has(1);
  @$pb.TagNumber(2)
  void clearOption1() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get option2 => $_getSZ(2);
  @$pb.TagNumber(3)
  set option2($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOption2() => $_has(2);
  @$pb.TagNumber(3)
  void clearOption2() => $_clearField(3);

  /// 售價，新台幣元的整數。
  @$pb.TagNumber(4)
  $fixnum.Int64 get price => $_getI64(3);
  @$pb.TagNumber(4)
  set price($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPrice() => $_has(3);
  @$pb.TagNumber(4)
  void clearPrice() => $_clearField(4);

  /// 原價（劃線價）；沒有設不設。
  @$pb.TagNumber(5)
  $fixnum.Int64 get compareAtPrice => $_getI64(4);
  @$pb.TagNumber(5)
  set compareAtPrice($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCompareAtPrice() => $_has(4);
  @$pb.TagNumber(5)
  void clearCompareAtPrice() => $_clearField(5);

  /// 現在的庫存，不為負。
  @$pb.TagNumber(6)
  $core.int get stockQty => $_getIZ(5);
  @$pb.TagNumber(6)
  set stockQty($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasStockQty() => $_has(5);
  @$pb.TagNumber(6)
  void clearStockQty() => $_clearField(6);

  /// 這個 SKU 現在能不能買：商品上架中（ACTIVE）而且庫存大於 0。
  @$pb.TagNumber(7)
  ProductAvailability get availability => $_getN(6);
  @$pb.TagNumber(7)
  set availability(ProductAvailability value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasAvailability() => $_has(6);
  @$pb.TagNumber(7)
  void clearAvailability() => $_clearField(7);
}

/// ProductHashtag 是商品的一個標籤。
class ProductHashtag extends $pb.GeneratedMessage {
  factory ProductHashtag({
    $core.String? name,
    $core.String? display,
  }) {
    final result = ProductHashtag._();
    if (name != null) result.name = name;
    if (display != null) result.display = display;
    return result;
  }

  ProductHashtag._();

  factory ProductHashtag.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductHashtag()..mergeFromBuffer(data, registry);
  factory ProductHashtag.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductHashtag()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductHashtag',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductHashtag.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'display')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductHashtag clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductHashtag copyWith(void Function(ProductHashtag) updates) =>
      super.copyWith((message) => updates(message as ProductHashtag))
          as ProductHashtag;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductHashtag() / ProductHashtag.new instead')
  static ProductHashtag create() => ProductHashtag._();
  static $pb.GeneratedMessage $_createMessage() => ProductHashtag._();
  @$core.override
  ProductHashtag createEmptyInstance() => ProductHashtag._();
  @$core.pragma('dart2js:noInline')
  static ProductHashtag getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductHashtag>(
          ProductHashtag.$_createMessage);
  static ProductHashtag? _defaultInstance;

  /// 正規化後的名稱，用來比對與連結。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// 使用者輸入的樣子，用來顯示。
  @$pb.TagNumber(2)
  $core.String get display => $_getSZ(1);
  @$pb.TagNumber(2)
  set display($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplay() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplay() => $_clearField(2);
}

/// Product 是公開讀取的商品完整內容。沒有賣家的狀態 enum、etag、建立與修改時間：公開的只有 availability。
class Product extends $pb.GeneratedMessage {
  factory Product({
    $core.String? id,
    $1.StoreCard? store,
    $core.String? categoryId,
    $core.String? title,
    ProductAvailability? availability,
    $2.Timestamp? firstPublishTime,
    $fixnum.Int64? minPrice,
    $fixnum.Int64? maxPrice,
    $core.int? salesCount,
    $core.int? favoriteCount,
    $core.double? ratingAverage,
    $core.int? reviewCount,
    $core.Iterable<ProductImage>? gallery,
    $core.Iterable<ProductDescriptionBlock>? description,
    $core.Iterable<OptionAxis>? axes,
    $core.Iterable<Sku>? skus,
    $core.Iterable<ProductHashtag>? hashtags,
  }) {
    final result = Product._();
    if (id != null) result.id = id;
    if (store != null) result.store = store;
    if (categoryId != null) result.categoryId = categoryId;
    if (title != null) result.title = title;
    if (availability != null) result.availability = availability;
    if (firstPublishTime != null) result.firstPublishTime = firstPublishTime;
    if (minPrice != null) result.minPrice = minPrice;
    if (maxPrice != null) result.maxPrice = maxPrice;
    if (salesCount != null) result.salesCount = salesCount;
    if (favoriteCount != null) result.favoriteCount = favoriteCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    if (reviewCount != null) result.reviewCount = reviewCount;
    if (gallery != null) result.gallery.addAll(gallery);
    if (description != null) result.description.addAll(description);
    if (axes != null) result.axes.addAll(axes);
    if (skus != null) result.skus.addAll(skus);
    if (hashtags != null) result.hashtags.addAll(hashtags);
    return result;
  }

  Product._();

  factory Product.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Product()..mergeFromBuffer(data, registry);
  factory Product.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Product()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Product',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: Product.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$1.StoreCard>(2, _omitFieldNames ? '' : 'store',
        subBuilder: $1.StoreCard.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'categoryId')
    ..aOS(4, _omitFieldNames ? '' : 'title')
    ..aE<ProductAvailability>(5, _omitFieldNames ? '' : 'availability',
        enumValues: ProductAvailability.values)
    ..aOM<$2.Timestamp>(6, _omitFieldNames ? '' : 'firstPublishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aInt64(7, _omitFieldNames ? '' : 'minPrice')
    ..aInt64(8, _omitFieldNames ? '' : 'maxPrice')
    ..aI(9, _omitFieldNames ? '' : 'salesCount')
    ..aI(10, _omitFieldNames ? '' : 'favoriteCount')
    ..aD(11, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..aI(12, _omitFieldNames ? '' : 'reviewCount')
    ..pPM<ProductImage>(13, _omitFieldNames ? '' : 'gallery',
        subBuilder: ProductImage.$_createMessage)
    ..pPM<ProductDescriptionBlock>(14, _omitFieldNames ? '' : 'description',
        subBuilder: ProductDescriptionBlock.$_createMessage)
    ..pPM<OptionAxis>(15, _omitFieldNames ? '' : 'axes',
        subBuilder: OptionAxis.$_createMessage)
    ..pPM<Sku>(16, _omitFieldNames ? '' : 'skus',
        subBuilder: Sku.$_createMessage)
    ..pPM<ProductHashtag>(17, _omitFieldNames ? '' : 'hashtags',
        subBuilder: ProductHashtag.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Product clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Product copyWith(void Function(Product) updates) =>
      super.copyWith((message) => updates(message as Product)) as Product;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Product() / Product.new instead')
  static Product create() => Product._();
  static $pb.GeneratedMessage $_createMessage() => Product._();
  @$core.override
  Product createEmptyInstance() => Product._();
  @$core.pragma('dart2js:noInline')
  static Product getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Product>(Product.$_createMessage);
  static Product? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 賣這個商品的店家；店家卡片讀不到時不設。
  @$pb.TagNumber(2)
  $1.StoreCard get store => $_getN(1);
  @$pb.TagNumber(2)
  set store($1.StoreCard value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStore() => $_has(1);
  @$pb.TagNumber(2)
  void clearStore() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.StoreCard ensureStore() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get categoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set categoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get title => $_getSZ(3);
  @$pb.TagNumber(4)
  set title($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearTitle() => $_clearField(4);

  /// 商品現在能不能買：上架中而且至少一個 SKU 有庫存。已下架（ARCHIVED）的商品是不可購買。
  @$pb.TagNumber(5)
  ProductAvailability get availability => $_getN(4);
  @$pb.TagNumber(5)
  set availability(ProductAvailability value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasAvailability() => $_has(4);
  @$pb.TagNumber(5)
  void clearAvailability() => $_clearField(5);

  /// 第一次上架的時間。
  @$pb.TagNumber(6)
  $2.Timestamp get firstPublishTime => $_getN(5);
  @$pb.TagNumber(6)
  set firstPublishTime($2.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasFirstPublishTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearFirstPublishTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $2.Timestamp ensureFirstPublishTime() => $_ensure(5);

  /// 沒刪除 SKU 的最低與最高售價，新台幣元的整數。
  @$pb.TagNumber(7)
  $fixnum.Int64 get minPrice => $_getI64(6);
  @$pb.TagNumber(7)
  set minPrice($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMinPrice() => $_has(6);
  @$pb.TagNumber(7)
  void clearMinPrice() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get maxPrice => $_getI64(7);
  @$pb.TagNumber(8)
  set maxPrice($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasMaxPrice() => $_has(7);
  @$pb.TagNumber(8)
  void clearMaxPrice() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.int get salesCount => $_getIZ(8);
  @$pb.TagNumber(9)
  set salesCount($core.int value) => $_setSignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasSalesCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearSalesCount() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.int get favoriteCount => $_getIZ(9);
  @$pb.TagNumber(10)
  set favoriteCount($core.int value) => $_setSignedInt32(9, value);
  @$pb.TagNumber(10)
  $core.bool hasFavoriteCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearFavoriteCount() => $_clearField(10);

  /// 平均評分，1 到 5；沒有評價不設。
  @$pb.TagNumber(11)
  $core.double get ratingAverage => $_getN(10);
  @$pb.TagNumber(11)
  set ratingAverage($core.double value) => $_setFloat(10, value);
  @$pb.TagNumber(11)
  $core.bool hasRatingAverage() => $_has(10);
  @$pb.TagNumber(11)
  void clearRatingAverage() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.int get reviewCount => $_getIZ(11);
  @$pb.TagNumber(12)
  set reviewCount($core.int value) => $_setSignedInt32(11, value);
  @$pb.TagNumber(12)
  $core.bool hasReviewCount() => $_has(11);
  @$pb.TagNumber(12)
  void clearReviewCount() => $_clearField(12);

  @$pb.TagNumber(13)
  $pb.PbList<ProductImage> get gallery => $_getList(12);

  @$pb.TagNumber(14)
  $pb.PbList<ProductDescriptionBlock> get description => $_getList(13);

  @$pb.TagNumber(15)
  $pb.PbList<OptionAxis> get axes => $_getList(14);

  @$pb.TagNumber(16)
  $pb.PbList<Sku> get skus => $_getList(15);

  @$pb.TagNumber(17)
  $pb.PbList<ProductHashtag> get hashtags => $_getList(16);
}

/// ProductSummary 是公開列表的一列（商品卡片）。
class ProductSummary extends $pb.GeneratedMessage {
  factory ProductSummary({
    $core.String? id,
    $1.StoreCard? store,
    $core.String? categoryId,
    $core.String? title,
    $0.Image? coverImage,
    ProductAvailability? availability,
    $2.Timestamp? firstPublishTime,
    $fixnum.Int64? minPrice,
    $fixnum.Int64? maxPrice,
    $core.int? salesCount,
    $core.int? favoriteCount,
    $core.double? ratingAverage,
    $core.int? reviewCount,
  }) {
    final result = ProductSummary._();
    if (id != null) result.id = id;
    if (store != null) result.store = store;
    if (categoryId != null) result.categoryId = categoryId;
    if (title != null) result.title = title;
    if (coverImage != null) result.coverImage = coverImage;
    if (availability != null) result.availability = availability;
    if (firstPublishTime != null) result.firstPublishTime = firstPublishTime;
    if (minPrice != null) result.minPrice = minPrice;
    if (maxPrice != null) result.maxPrice = maxPrice;
    if (salesCount != null) result.salesCount = salesCount;
    if (favoriteCount != null) result.favoriteCount = favoriteCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    if (reviewCount != null) result.reviewCount = reviewCount;
    return result;
  }

  ProductSummary._();

  factory ProductSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductSummary()..mergeFromBuffer(data, registry);
  factory ProductSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductSummary',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$1.StoreCard>(2, _omitFieldNames ? '' : 'store',
        subBuilder: $1.StoreCard.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'categoryId')
    ..aOS(4, _omitFieldNames ? '' : 'title')
    ..aOM<$0.Image>(5, _omitFieldNames ? '' : 'coverImage',
        subBuilder: $0.Image.$_createMessage)
    ..aE<ProductAvailability>(6, _omitFieldNames ? '' : 'availability',
        enumValues: ProductAvailability.values)
    ..aOM<$2.Timestamp>(7, _omitFieldNames ? '' : 'firstPublishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aInt64(8, _omitFieldNames ? '' : 'minPrice')
    ..aInt64(9, _omitFieldNames ? '' : 'maxPrice')
    ..aI(10, _omitFieldNames ? '' : 'salesCount')
    ..aI(11, _omitFieldNames ? '' : 'favoriteCount')
    ..aD(12, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..aI(13, _omitFieldNames ? '' : 'reviewCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductSummary copyWith(void Function(ProductSummary) updates) =>
      super.copyWith((message) => updates(message as ProductSummary))
          as ProductSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductSummary() / ProductSummary.new instead')
  static ProductSummary create() => ProductSummary._();
  static $pb.GeneratedMessage $_createMessage() => ProductSummary._();
  @$core.override
  ProductSummary createEmptyInstance() => ProductSummary._();
  @$core.pragma('dart2js:noInline')
  static ProductSummary getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductSummary>(
          ProductSummary.$_createMessage);
  static ProductSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 店家卡片讀不到時不設。
  @$pb.TagNumber(2)
  $1.StoreCard get store => $_getN(1);
  @$pb.TagNumber(2)
  set store($1.StoreCard value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStore() => $_has(1);
  @$pb.TagNumber(2)
  void clearStore() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.StoreCard ensureStore() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get categoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set categoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get title => $_getSZ(3);
  @$pb.TagNumber(4)
  set title($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearTitle() => $_clearField(4);

  /// 主圖庫的第一張；沒有圖不設。
  @$pb.TagNumber(5)
  $0.Image get coverImage => $_getN(4);
  @$pb.TagNumber(5)
  set coverImage($0.Image value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCoverImage() => $_has(4);
  @$pb.TagNumber(5)
  void clearCoverImage() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Image ensureCoverImage() => $_ensure(4);

  @$pb.TagNumber(6)
  ProductAvailability get availability => $_getN(5);
  @$pb.TagNumber(6)
  set availability(ProductAvailability value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasAvailability() => $_has(5);
  @$pb.TagNumber(6)
  void clearAvailability() => $_clearField(6);

  @$pb.TagNumber(7)
  $2.Timestamp get firstPublishTime => $_getN(6);
  @$pb.TagNumber(7)
  set firstPublishTime($2.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasFirstPublishTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearFirstPublishTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $2.Timestamp ensureFirstPublishTime() => $_ensure(6);

  @$pb.TagNumber(8)
  $fixnum.Int64 get minPrice => $_getI64(7);
  @$pb.TagNumber(8)
  set minPrice($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasMinPrice() => $_has(7);
  @$pb.TagNumber(8)
  void clearMinPrice() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get maxPrice => $_getI64(8);
  @$pb.TagNumber(9)
  set maxPrice($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasMaxPrice() => $_has(8);
  @$pb.TagNumber(9)
  void clearMaxPrice() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.int get salesCount => $_getIZ(9);
  @$pb.TagNumber(10)
  set salesCount($core.int value) => $_setSignedInt32(9, value);
  @$pb.TagNumber(10)
  $core.bool hasSalesCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearSalesCount() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.int get favoriteCount => $_getIZ(10);
  @$pb.TagNumber(11)
  set favoriteCount($core.int value) => $_setSignedInt32(10, value);
  @$pb.TagNumber(11)
  $core.bool hasFavoriteCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearFavoriteCount() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.double get ratingAverage => $_getN(11);
  @$pb.TagNumber(12)
  set ratingAverage($core.double value) => $_setFloat(11, value);
  @$pb.TagNumber(12)
  $core.bool hasRatingAverage() => $_has(11);
  @$pb.TagNumber(12)
  void clearRatingAverage() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.int get reviewCount => $_getIZ(12);
  @$pb.TagNumber(13)
  set reviewCount($core.int value) => $_setSignedInt32(12, value);
  @$pb.TagNumber(13)
  $core.bool hasReviewCount() => $_has(12);
  @$pb.TagNumber(13)
  void clearReviewCount() => $_clearField(13);
}

/// SellerProduct 是賣家讀自己的商品完整內容。
class SellerProduct extends $pb.GeneratedMessage {
  factory SellerProduct({
    $core.String? id,
    $core.String? categoryId,
    $core.String? title,
    ProductStatus? status,
    $2.Timestamp? firstPublishTime,
    $fixnum.Int64? minPrice,
    $fixnum.Int64? maxPrice,
    $core.int? salesCount,
    $core.int? favoriteCount,
    $core.double? ratingAverage,
    $core.int? reviewCount,
    $core.Iterable<ProductImage>? gallery,
    $core.Iterable<ProductDescriptionBlock>? description,
    $core.Iterable<OptionAxis>? axes,
    $core.Iterable<Sku>? skus,
    $core.Iterable<ProductHashtag>? hashtags,
    $core.int? lowStockThreshold,
    $core.String? etag,
    $2.Timestamp? createTime,
    $2.Timestamp? updateTime,
  }) {
    final result = SellerProduct._();
    if (id != null) result.id = id;
    if (categoryId != null) result.categoryId = categoryId;
    if (title != null) result.title = title;
    if (status != null) result.status = status;
    if (firstPublishTime != null) result.firstPublishTime = firstPublishTime;
    if (minPrice != null) result.minPrice = minPrice;
    if (maxPrice != null) result.maxPrice = maxPrice;
    if (salesCount != null) result.salesCount = salesCount;
    if (favoriteCount != null) result.favoriteCount = favoriteCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    if (reviewCount != null) result.reviewCount = reviewCount;
    if (gallery != null) result.gallery.addAll(gallery);
    if (description != null) result.description.addAll(description);
    if (axes != null) result.axes.addAll(axes);
    if (skus != null) result.skus.addAll(skus);
    if (hashtags != null) result.hashtags.addAll(hashtags);
    if (lowStockThreshold != null) result.lowStockThreshold = lowStockThreshold;
    if (etag != null) result.etag = etag;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    return result;
  }

  SellerProduct._();

  factory SellerProduct.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerProduct()..mergeFromBuffer(data, registry);
  factory SellerProduct.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerProduct()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerProduct',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SellerProduct.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'categoryId')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aE<ProductStatus>(4, _omitFieldNames ? '' : 'status',
        enumValues: ProductStatus.values)
    ..aOM<$2.Timestamp>(5, _omitFieldNames ? '' : 'firstPublishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aInt64(6, _omitFieldNames ? '' : 'minPrice')
    ..aInt64(7, _omitFieldNames ? '' : 'maxPrice')
    ..aI(8, _omitFieldNames ? '' : 'salesCount')
    ..aI(9, _omitFieldNames ? '' : 'favoriteCount')
    ..aD(10, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..aI(11, _omitFieldNames ? '' : 'reviewCount')
    ..pPM<ProductImage>(12, _omitFieldNames ? '' : 'gallery',
        subBuilder: ProductImage.$_createMessage)
    ..pPM<ProductDescriptionBlock>(13, _omitFieldNames ? '' : 'description',
        subBuilder: ProductDescriptionBlock.$_createMessage)
    ..pPM<OptionAxis>(14, _omitFieldNames ? '' : 'axes',
        subBuilder: OptionAxis.$_createMessage)
    ..pPM<Sku>(15, _omitFieldNames ? '' : 'skus',
        subBuilder: Sku.$_createMessage)
    ..pPM<ProductHashtag>(16, _omitFieldNames ? '' : 'hashtags',
        subBuilder: ProductHashtag.$_createMessage)
    ..aI(17, _omitFieldNames ? '' : 'lowStockThreshold')
    ..aOS(18, _omitFieldNames ? '' : 'etag')
    ..aOM<$2.Timestamp>(19, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOM<$2.Timestamp>(20, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerProduct clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerProduct copyWith(void Function(SellerProduct) updates) =>
      super.copyWith((message) => updates(message as SellerProduct))
          as SellerProduct;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerProduct() / SellerProduct.new instead')
  static SellerProduct create() => SellerProduct._();
  static $pb.GeneratedMessage $_createMessage() => SellerProduct._();
  @$core.override
  SellerProduct createEmptyInstance() => SellerProduct._();
  @$core.pragma('dart2js:noInline')
  static SellerProduct getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerProduct>(
          SellerProduct.$_createMessage);
  static SellerProduct? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get categoryId => $_getSZ(1);
  @$pb.TagNumber(2)
  set categoryId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCategoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategoryId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  ProductStatus get status => $_getN(3);
  @$pb.TagNumber(4)
  set status(ProductStatus value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatus() => $_clearField(4);

  /// 第一次上架的時間；從沒上架過不設。
  @$pb.TagNumber(5)
  $2.Timestamp get firstPublishTime => $_getN(4);
  @$pb.TagNumber(5)
  set firstPublishTime($2.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasFirstPublishTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearFirstPublishTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $2.Timestamp ensureFirstPublishTime() => $_ensure(4);

  @$pb.TagNumber(6)
  $fixnum.Int64 get minPrice => $_getI64(5);
  @$pb.TagNumber(6)
  set minPrice($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasMinPrice() => $_has(5);
  @$pb.TagNumber(6)
  void clearMinPrice() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get maxPrice => $_getI64(6);
  @$pb.TagNumber(7)
  set maxPrice($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMaxPrice() => $_has(6);
  @$pb.TagNumber(7)
  void clearMaxPrice() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get salesCount => $_getIZ(7);
  @$pb.TagNumber(8)
  set salesCount($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSalesCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearSalesCount() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.int get favoriteCount => $_getIZ(8);
  @$pb.TagNumber(9)
  set favoriteCount($core.int value) => $_setSignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasFavoriteCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearFavoriteCount() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.double get ratingAverage => $_getN(9);
  @$pb.TagNumber(10)
  set ratingAverage($core.double value) => $_setFloat(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRatingAverage() => $_has(9);
  @$pb.TagNumber(10)
  void clearRatingAverage() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.int get reviewCount => $_getIZ(10);
  @$pb.TagNumber(11)
  set reviewCount($core.int value) => $_setSignedInt32(10, value);
  @$pb.TagNumber(11)
  $core.bool hasReviewCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearReviewCount() => $_clearField(11);

  @$pb.TagNumber(12)
  $pb.PbList<ProductImage> get gallery => $_getList(11);

  @$pb.TagNumber(13)
  $pb.PbList<ProductDescriptionBlock> get description => $_getList(12);

  @$pb.TagNumber(14)
  $pb.PbList<OptionAxis> get axes => $_getList(13);

  @$pb.TagNumber(15)
  $pb.PbList<Sku> get skus => $_getList(14);

  @$pb.TagNumber(16)
  $pb.PbList<ProductHashtag> get hashtags => $_getList(15);

  /// 賣家設的低庫存門檻；沒設不設（用平台預設值）。
  @$pb.TagNumber(17)
  $core.int get lowStockThreshold => $_getIZ(16);
  @$pb.TagNumber(17)
  set lowStockThreshold($core.int value) => $_setSignedInt32(16, value);
  @$pb.TagNumber(17)
  $core.bool hasLowStockThreshold() => $_has(16);
  @$pb.TagNumber(17)
  void clearLowStockThreshold() => $_clearField(17);

  /// 修改、上下架與刪除時帶回，client 視為不透明字串。範圍是賣家可編輯的內容與狀態；庫存、銷量、收藏數、評分、首次上架時間不影響它。
  @$pb.TagNumber(18)
  $core.String get etag => $_getSZ(17);
  @$pb.TagNumber(18)
  set etag($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasEtag() => $_has(17);
  @$pb.TagNumber(18)
  void clearEtag() => $_clearField(18);

  @$pb.TagNumber(19)
  $2.Timestamp get createTime => $_getN(18);
  @$pb.TagNumber(19)
  set createTime($2.Timestamp value) => $_setField(19, value);
  @$pb.TagNumber(19)
  $core.bool hasCreateTime() => $_has(18);
  @$pb.TagNumber(19)
  void clearCreateTime() => $_clearField(19);
  @$pb.TagNumber(19)
  $2.Timestamp ensureCreateTime() => $_ensure(18);

  @$pb.TagNumber(20)
  $2.Timestamp get updateTime => $_getN(19);
  @$pb.TagNumber(20)
  set updateTime($2.Timestamp value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasUpdateTime() => $_has(19);
  @$pb.TagNumber(20)
  void clearUpdateTime() => $_clearField(20);
  @$pb.TagNumber(20)
  $2.Timestamp ensureUpdateTime() => $_ensure(19);
}

/// SellerProductSummary 是賣家清單的一列。
class SellerProductSummary extends $pb.GeneratedMessage {
  factory SellerProductSummary({
    $core.String? id,
    $core.String? categoryId,
    $core.String? title,
    $0.Image? coverImage,
    ProductStatus? status,
    $2.Timestamp? firstPublishTime,
    $fixnum.Int64? minPrice,
    $fixnum.Int64? maxPrice,
    $core.int? salesCount,
    $core.int? favoriteCount,
    $core.double? ratingAverage,
    $core.int? reviewCount,
    $core.int? totalStock,
    $core.int? lowStockThreshold,
    $core.String? etag,
    $2.Timestamp? createTime,
  }) {
    final result = SellerProductSummary._();
    if (id != null) result.id = id;
    if (categoryId != null) result.categoryId = categoryId;
    if (title != null) result.title = title;
    if (coverImage != null) result.coverImage = coverImage;
    if (status != null) result.status = status;
    if (firstPublishTime != null) result.firstPublishTime = firstPublishTime;
    if (minPrice != null) result.minPrice = minPrice;
    if (maxPrice != null) result.maxPrice = maxPrice;
    if (salesCount != null) result.salesCount = salesCount;
    if (favoriteCount != null) result.favoriteCount = favoriteCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    if (reviewCount != null) result.reviewCount = reviewCount;
    if (totalStock != null) result.totalStock = totalStock;
    if (lowStockThreshold != null) result.lowStockThreshold = lowStockThreshold;
    if (etag != null) result.etag = etag;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  SellerProductSummary._();

  factory SellerProductSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerProductSummary()..mergeFromBuffer(data, registry);
  factory SellerProductSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerProductSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerProductSummary',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SellerProductSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'categoryId')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOM<$0.Image>(4, _omitFieldNames ? '' : 'coverImage',
        subBuilder: $0.Image.$_createMessage)
    ..aE<ProductStatus>(5, _omitFieldNames ? '' : 'status',
        enumValues: ProductStatus.values)
    ..aOM<$2.Timestamp>(6, _omitFieldNames ? '' : 'firstPublishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aInt64(7, _omitFieldNames ? '' : 'minPrice')
    ..aInt64(8, _omitFieldNames ? '' : 'maxPrice')
    ..aI(9, _omitFieldNames ? '' : 'salesCount')
    ..aI(10, _omitFieldNames ? '' : 'favoriteCount')
    ..aD(11, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..aI(12, _omitFieldNames ? '' : 'reviewCount')
    ..aI(13, _omitFieldNames ? '' : 'totalStock')
    ..aI(14, _omitFieldNames ? '' : 'lowStockThreshold')
    ..aOS(15, _omitFieldNames ? '' : 'etag')
    ..aOM<$2.Timestamp>(16, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerProductSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerProductSummary copyWith(void Function(SellerProductSummary) updates) =>
      super.copyWith((message) => updates(message as SellerProductSummary))
          as SellerProductSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerProductSummary() / SellerProductSummary.new instead')
  static SellerProductSummary create() => SellerProductSummary._();
  static $pb.GeneratedMessage $_createMessage() => SellerProductSummary._();
  @$core.override
  SellerProductSummary createEmptyInstance() => SellerProductSummary._();
  @$core.pragma('dart2js:noInline')
  static SellerProductSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerProductSummary>(
          SellerProductSummary.$_createMessage);
  static SellerProductSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get categoryId => $_getSZ(1);
  @$pb.TagNumber(2)
  set categoryId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCategoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategoryId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $0.Image get coverImage => $_getN(3);
  @$pb.TagNumber(4)
  set coverImage($0.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCoverImage() => $_has(3);
  @$pb.TagNumber(4)
  void clearCoverImage() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Image ensureCoverImage() => $_ensure(3);

  @$pb.TagNumber(5)
  ProductStatus get status => $_getN(4);
  @$pb.TagNumber(5)
  set status(ProductStatus value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStatus() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatus() => $_clearField(5);

  @$pb.TagNumber(6)
  $2.Timestamp get firstPublishTime => $_getN(5);
  @$pb.TagNumber(6)
  set firstPublishTime($2.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasFirstPublishTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearFirstPublishTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $2.Timestamp ensureFirstPublishTime() => $_ensure(5);

  @$pb.TagNumber(7)
  $fixnum.Int64 get minPrice => $_getI64(6);
  @$pb.TagNumber(7)
  set minPrice($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMinPrice() => $_has(6);
  @$pb.TagNumber(7)
  void clearMinPrice() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get maxPrice => $_getI64(7);
  @$pb.TagNumber(8)
  set maxPrice($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasMaxPrice() => $_has(7);
  @$pb.TagNumber(8)
  void clearMaxPrice() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.int get salesCount => $_getIZ(8);
  @$pb.TagNumber(9)
  set salesCount($core.int value) => $_setSignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasSalesCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearSalesCount() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.int get favoriteCount => $_getIZ(9);
  @$pb.TagNumber(10)
  set favoriteCount($core.int value) => $_setSignedInt32(9, value);
  @$pb.TagNumber(10)
  $core.bool hasFavoriteCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearFavoriteCount() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.double get ratingAverage => $_getN(10);
  @$pb.TagNumber(11)
  set ratingAverage($core.double value) => $_setFloat(10, value);
  @$pb.TagNumber(11)
  $core.bool hasRatingAverage() => $_has(10);
  @$pb.TagNumber(11)
  void clearRatingAverage() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.int get reviewCount => $_getIZ(11);
  @$pb.TagNumber(12)
  set reviewCount($core.int value) => $_setSignedInt32(11, value);
  @$pb.TagNumber(12)
  $core.bool hasReviewCount() => $_has(11);
  @$pb.TagNumber(12)
  void clearReviewCount() => $_clearField(12);

  /// 沒刪除 SKU 的庫存總和（庫存頁用，免得逐商品再讀 SKU）。
  @$pb.TagNumber(13)
  $core.int get totalStock => $_getIZ(12);
  @$pb.TagNumber(13)
  set totalStock($core.int value) => $_setSignedInt32(12, value);
  @$pb.TagNumber(13)
  $core.bool hasTotalStock() => $_has(12);
  @$pb.TagNumber(13)
  void clearTotalStock() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.int get lowStockThreshold => $_getIZ(13);
  @$pb.TagNumber(14)
  set lowStockThreshold($core.int value) => $_setSignedInt32(13, value);
  @$pb.TagNumber(14)
  $core.bool hasLowStockThreshold() => $_has(13);
  @$pb.TagNumber(14)
  void clearLowStockThreshold() => $_clearField(14);

  /// 列表上的上下架動作用它當版本。
  @$pb.TagNumber(15)
  $core.String get etag => $_getSZ(14);
  @$pb.TagNumber(15)
  set etag($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasEtag() => $_has(14);
  @$pb.TagNumber(15)
  void clearEtag() => $_clearField(15);

  @$pb.TagNumber(16)
  $2.Timestamp get createTime => $_getN(15);
  @$pb.TagNumber(16)
  set createTime($2.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasCreateTime() => $_has(15);
  @$pb.TagNumber(16)
  void clearCreateTime() => $_clearField(16);
  @$pb.TagNumber(16)
  $2.Timestamp ensureCreateTime() => $_ensure(15);
}

/// ProductInput 是商品的完整可寫內容。修改是整體取代：沒有帶的圖片、選項軸與 SKU 組合會被移除。
/// 狀態不在這裡，用切換狀態的操作改。
class ProductInput extends $pb.GeneratedMessage {
  factory ProductInput({
    $core.String? title,
    $core.String? categoryId,
    $core.Iterable<$core.String>? hashtagInputs,
    $core.Iterable<ImageInput>? gallery,
    $core.Iterable<DescriptionBlock>? description,
    $core.Iterable<OptionAxisInput>? axes,
    $core.Iterable<SkuInput>? skus,
    $core.int? lowStockThreshold,
  }) {
    final result = ProductInput._();
    if (title != null) result.title = title;
    if (categoryId != null) result.categoryId = categoryId;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    if (gallery != null) result.gallery.addAll(gallery);
    if (description != null) result.description.addAll(description);
    if (axes != null) result.axes.addAll(axes);
    if (skus != null) result.skus.addAll(skus);
    if (lowStockThreshold != null) result.lowStockThreshold = lowStockThreshold;
    return result;
  }

  ProductInput._();

  factory ProductInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductInput()..mergeFromBuffer(data, registry);
  factory ProductInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'title')
    ..aOS(2, _omitFieldNames ? '' : 'categoryId')
    ..pPS(3, _omitFieldNames ? '' : 'hashtagInputs')
    ..pPM<ImageInput>(4, _omitFieldNames ? '' : 'gallery',
        subBuilder: ImageInput.$_createMessage)
    ..pPM<DescriptionBlock>(5, _omitFieldNames ? '' : 'description',
        subBuilder: DescriptionBlock.$_createMessage)
    ..pPM<OptionAxisInput>(6, _omitFieldNames ? '' : 'axes',
        subBuilder: OptionAxisInput.$_createMessage)
    ..pPM<SkuInput>(7, _omitFieldNames ? '' : 'skus',
        subBuilder: SkuInput.$_createMessage)
    ..aI(8, _omitFieldNames ? '' : 'lowStockThreshold')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductInput copyWith(void Function(ProductInput) updates) =>
      super.copyWith((message) => updates(message as ProductInput))
          as ProductInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductInput() / ProductInput.new instead')
  static ProductInput create() => ProductInput._();
  static $pb.GeneratedMessage $_createMessage() => ProductInput._();
  @$core.override
  ProductInput createEmptyInstance() => ProductInput._();
  @$core.pragma('dart2js:noInline')
  static ProductInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductInput>(
          ProductInput.$_createMessage);
  static ProductInput? _defaultInstance;

  /// 商品標題，去頭尾空白後不得為空。
  @$pb.TagNumber(1)
  $core.String get title => $_getSZ(0);
  @$pb.TagNumber(1)
  set title($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTitle() => $_has(0);
  @$pb.TagNumber(1)
  void clearTitle() => $_clearField(1);

  /// 分類 ID，必須是葉節點。
  @$pb.TagNumber(2)
  $core.String get categoryId => $_getSZ(1);
  @$pb.TagNumber(2)
  set categoryId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCategoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategoryId() => $_clearField(2);

  /// 標籤的輸入（使用者輸入的樣子，可帶開頭的 #）；整批取代，空清單是沒有標籤。個數與格式的規則同 social 的標籤（platform/hashtag）。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(2);

  /// 主圖，依顯示順序；狀態為 ACTIVE 的商品至少要有一張。
  @$pb.TagNumber(4)
  $pb.PbList<ImageInput> get gallery => $_getList(3);

  /// 描述的區塊，依顯示順序。上限跨整份描述計算：區塊個數，帶圖片的區塊個數，以及段落、清單項目與表格儲存格的文字總量。
  @$pb.TagNumber(5)
  $pb.PbList<DescriptionBlock> get description => $_getList(4);

  /// 選項軸，依順序；沒有軸的商品只有一個預設 SKU。
  @$pb.TagNumber(6)
  $pb.PbList<OptionAxisInput> get axes => $_getList(5);

  /// SKU，以選項值的組合辨識；至少一個。
  @$pb.TagNumber(7)
  $pb.PbList<SkuInput> get skus => $_getList(6);

  /// 低庫存提醒門檻（每個 SKU 的庫存小於或等於它且大於 0 就算偏低）；不設是用平台預設值（CountMyProductsResponse.default_low_stock_threshold）。
  /// 0 到 9999；整體取代時不帶就是清掉，改回預設值。
  @$pb.TagNumber(8)
  $core.int get lowStockThreshold => $_getIZ(7);
  @$pb.TagNumber(8)
  set lowStockThreshold($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasLowStockThreshold() => $_has(7);
  @$pb.TagNumber(8)
  void clearLowStockThreshold() => $_clearField(8);
}

enum ImageInput_Source { stagedToken, assetId, notSet }

/// ImageInput 是寫入時的一張圖：新上傳的圖帶 staged token，商品現有的圖帶它的 asset_id，兩者剛好給一個。
class ImageInput extends $pb.GeneratedMessage {
  factory ImageInput({
    $core.String? stagedToken,
    $core.String? assetId,
    $core.String? alt,
  }) {
    final result = ImageInput._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    if (assetId != null) result.assetId = assetId;
    if (alt != null) result.alt = alt;
    return result;
  }

  ImageInput._();

  factory ImageInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageInput()..mergeFromBuffer(data, registry);
  factory ImageInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageInput()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ImageInput_Source> _ImageInput_SourceByTag =
      {
    1: ImageInput_Source.stagedToken,
    2: ImageInput_Source.assetId,
    0: ImageInput_Source.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImageInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ImageInput.$_createMessage)
    ..oo(0, [1, 2])
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..aOS(2, _omitFieldNames ? '' : 'assetId')
    ..aOS(3, _omitFieldNames ? '' : 'alt')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageInput copyWith(void Function(ImageInput) updates) =>
      super.copyWith((message) => updates(message as ImageInput)) as ImageInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ImageInput() / ImageInput.new instead')
  static ImageInput create() => ImageInput._();
  static $pb.GeneratedMessage $_createMessage() => ImageInput._();
  @$core.override
  ImageInput createEmptyInstance() => ImageInput._();
  @$core.pragma('dart2js:noInline')
  static ImageInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImageInput>(ImageInput.$_createMessage);
  static ImageInput? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  ImageInput_Source whichSource() => _ImageInput_SourceByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearSource() => $_clearField($_whichOneof(0));

  /// media 上傳完成後拿到的 staged token；伺服器在寫入的 transaction 內確認它。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);

  /// 商品現有的圖（讀取商品時回傳的資產 ID），沿用不重新上傳。
  @$pb.TagNumber(2)
  $core.String get assetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set assetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAssetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAssetId() => $_clearField(2);

  /// 圖片的替代文字，可空。
  @$pb.TagNumber(3)
  $core.String get alt => $_getSZ(2);
  @$pb.TagNumber(3)
  set alt($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAlt() => $_has(2);
  @$pb.TagNumber(3)
  void clearAlt() => $_clearField(3);
}

enum DescriptionBlock_Block { paragraph, image, bulletList, table, notSet }

/// DescriptionBlock 是描述的一個區塊，剛好設一種。
class DescriptionBlock extends $pb.GeneratedMessage {
  factory DescriptionBlock({
    $core.String? paragraph,
    ImageInput? image,
    BulletList? bulletList,
    Table? table,
  }) {
    final result = DescriptionBlock._();
    if (paragraph != null) result.paragraph = paragraph;
    if (image != null) result.image = image;
    if (bulletList != null) result.bulletList = bulletList;
    if (table != null) result.table = table;
    return result;
  }

  DescriptionBlock._();

  factory DescriptionBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DescriptionBlock()..mergeFromBuffer(data, registry);
  factory DescriptionBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DescriptionBlock()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, DescriptionBlock_Block>
      _DescriptionBlock_BlockByTag = {
    1: DescriptionBlock_Block.paragraph,
    2: DescriptionBlock_Block.image,
    3: DescriptionBlock_Block.bulletList,
    4: DescriptionBlock_Block.table,
    0: DescriptionBlock_Block.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DescriptionBlock',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: DescriptionBlock.$_createMessage)
    ..oo(0, [1, 2, 3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'paragraph')
    ..aOM<ImageInput>(2, _omitFieldNames ? '' : 'image',
        subBuilder: ImageInput.$_createMessage)
    ..aOM<BulletList>(3, _omitFieldNames ? '' : 'bulletList',
        subBuilder: BulletList.$_createMessage)
    ..aOM<Table>(4, _omitFieldNames ? '' : 'table',
        subBuilder: Table.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescriptionBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescriptionBlock copyWith(void Function(DescriptionBlock) updates) =>
      super.copyWith((message) => updates(message as DescriptionBlock))
          as DescriptionBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DescriptionBlock() / DescriptionBlock.new instead')
  static DescriptionBlock create() => DescriptionBlock._();
  static $pb.GeneratedMessage $_createMessage() => DescriptionBlock._();
  @$core.override
  DescriptionBlock createEmptyInstance() => DescriptionBlock._();
  @$core.pragma('dart2js:noInline')
  static DescriptionBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DescriptionBlock>(
          DescriptionBlock.$_createMessage);
  static DescriptionBlock? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  DescriptionBlock_Block whichBlock() =>
      _DescriptionBlock_BlockByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearBlock() => $_clearField($_whichOneof(0));

  /// 段落，純文字。
  @$pb.TagNumber(1)
  $core.String get paragraph => $_getSZ(0);
  @$pb.TagNumber(1)
  set paragraph($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasParagraph() => $_has(0);
  @$pb.TagNumber(1)
  void clearParagraph() => $_clearField(1);

  /// 圖片。
  @$pb.TagNumber(2)
  ImageInput get image => $_getN(1);
  @$pb.TagNumber(2)
  set image(ImageInput value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasImage() => $_has(1);
  @$pb.TagNumber(2)
  void clearImage() => $_clearField(2);
  @$pb.TagNumber(2)
  ImageInput ensureImage() => $_ensure(1);

  /// 項目清單。
  @$pb.TagNumber(3)
  BulletList get bulletList => $_getN(2);
  @$pb.TagNumber(3)
  set bulletList(BulletList value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBulletList() => $_has(2);
  @$pb.TagNumber(3)
  void clearBulletList() => $_clearField(3);
  @$pb.TagNumber(3)
  BulletList ensureBulletList() => $_ensure(2);

  /// 表格。
  @$pb.TagNumber(4)
  Table get table => $_getN(3);
  @$pb.TagNumber(4)
  set table(Table value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasTable() => $_has(3);
  @$pb.TagNumber(4)
  void clearTable() => $_clearField(4);
  @$pb.TagNumber(4)
  Table ensureTable() => $_ensure(3);
}

/// BulletList 是項目清單。
class BulletList extends $pb.GeneratedMessage {
  factory BulletList({
    $core.Iterable<$core.String>? items,
  }) {
    final result = BulletList._();
    if (items != null) result.items.addAll(items);
    return result;
  }

  BulletList._();

  factory BulletList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BulletList()..mergeFromBuffer(data, registry);
  factory BulletList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BulletList()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BulletList',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: BulletList.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'items')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulletList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulletList copyWith(void Function(BulletList) updates) =>
      super.copyWith((message) => updates(message as BulletList)) as BulletList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BulletList() / BulletList.new instead')
  static BulletList create() => BulletList._();
  static $pb.GeneratedMessage $_createMessage() => BulletList._();
  @$core.override
  BulletList createEmptyInstance() => BulletList._();
  @$core.pragma('dart2js:noInline')
  static BulletList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BulletList>(BulletList.$_createMessage);
  static BulletList? _defaultInstance;

  /// 清單項目，至少一項；每項的文字上限是 max_runes。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get items => $_getList(0);
}

/// Table 是表格，每列的儲存格數相同。
class Table extends $pb.GeneratedMessage {
  factory Table({
    $core.Iterable<TableRow>? rows,
  }) {
    final result = Table._();
    if (rows != null) result.rows.addAll(rows);
    return result;
  }

  Table._();

  factory Table.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Table()..mergeFromBuffer(data, registry);
  factory Table.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Table()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Table',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: Table.$_createMessage)
    ..pPM<TableRow>(1, _omitFieldNames ? '' : 'rows',
        subBuilder: TableRow.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Table clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Table copyWith(void Function(Table) updates) =>
      super.copyWith((message) => updates(message as Table)) as Table;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Table() / Table.new instead')
  static Table create() => Table._();
  static $pb.GeneratedMessage $_createMessage() => Table._();
  @$core.override
  Table createEmptyInstance() => Table._();
  @$core.pragma('dart2js:noInline')
  static Table getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Table>(Table.$_createMessage);
  static Table? _defaultInstance;

  /// 表格的列，至少一列。
  @$pb.TagNumber(1)
  $pb.PbList<TableRow> get rows => $_getList(0);
}

/// TableRow 是表格的一列。
class TableRow extends $pb.GeneratedMessage {
  factory TableRow({
    $core.Iterable<$core.String>? cells,
  }) {
    final result = TableRow._();
    if (cells != null) result.cells.addAll(cells);
    return result;
  }

  TableRow._();

  factory TableRow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TableRow()..mergeFromBuffer(data, registry);
  factory TableRow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TableRow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TableRow',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: TableRow.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'cells')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TableRow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TableRow copyWith(void Function(TableRow) updates) =>
      super.copyWith((message) => updates(message as TableRow)) as TableRow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TableRow() / TableRow.new instead')
  static TableRow create() => TableRow._();
  static $pb.GeneratedMessage $_createMessage() => TableRow._();
  @$core.override
  TableRow createEmptyInstance() => TableRow._();
  @$core.pragma('dart2js:noInline')
  static TableRow getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TableRow>(TableRow.$_createMessage);
  static TableRow? _defaultInstance;

  /// 這一列的儲存格，至少一格；每格的文字上限是 max_runes。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get cells => $_getList(0);
}

/// OptionAxisInput 是一個選項軸：名稱與有序的選項值。名稱與值都是賣家的字，不翻譯。
class OptionAxisInput extends $pb.GeneratedMessage {
  factory OptionAxisInput({
    $core.String? name,
    $core.Iterable<$core.String>? values,
  }) {
    final result = OptionAxisInput._();
    if (name != null) result.name = name;
    if (values != null) result.values.addAll(values);
    return result;
  }

  OptionAxisInput._();

  factory OptionAxisInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxisInput()..mergeFromBuffer(data, registry);
  factory OptionAxisInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxisInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OptionAxisInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: OptionAxisInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..pPS(2, _omitFieldNames ? '' : 'values')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxisInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxisInput copyWith(void Function(OptionAxisInput) updates) =>
      super.copyWith((message) => updates(message as OptionAxisInput))
          as OptionAxisInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OptionAxisInput() / OptionAxisInput.new instead')
  static OptionAxisInput create() => OptionAxisInput._();
  static $pb.GeneratedMessage $_createMessage() => OptionAxisInput._();
  @$core.override
  OptionAxisInput createEmptyInstance() => OptionAxisInput._();
  @$core.pragma('dart2js:noInline')
  static OptionAxisInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OptionAxisInput>(
          OptionAxisInput.$_createMessage);
  static OptionAxisInput? _defaultInstance;

  /// 軸名稱，例如「顏色」。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// 選項值，依顯示順序，至少一個；每個值的長度上限是 max_runes。
  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get values => $_getList(1);
}

/// SkuInput 是一個 SKU，以選項值的組合辨識：沒有軸的商品兩個選項都空；有一個軸只給 option1。
/// 組合已存在時更新價格；庫存只在新組合時作為初始值，既有 SKU 的庫存只由庫存調整改。
class SkuInput extends $pb.GeneratedMessage {
  factory SkuInput({
    $core.String? option1,
    $core.String? option2,
    $fixnum.Int64? price,
    $fixnum.Int64? compareAtPrice,
    $core.int? initialStock,
  }) {
    final result = SkuInput._();
    if (option1 != null) result.option1 = option1;
    if (option2 != null) result.option2 = option2;
    if (price != null) result.price = price;
    if (compareAtPrice != null) result.compareAtPrice = compareAtPrice;
    if (initialStock != null) result.initialStock = initialStock;
    return result;
  }

  SkuInput._();

  factory SkuInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SkuInput()..mergeFromBuffer(data, registry);
  factory SkuInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SkuInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SkuInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SkuInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'option1')
    ..aOS(2, _omitFieldNames ? '' : 'option2')
    ..aInt64(3, _omitFieldNames ? '' : 'price')
    ..aInt64(4, _omitFieldNames ? '' : 'compareAtPrice')
    ..aI(5, _omitFieldNames ? '' : 'initialStock')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SkuInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SkuInput copyWith(void Function(SkuInput) updates) =>
      super.copyWith((message) => updates(message as SkuInput)) as SkuInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SkuInput() / SkuInput.new instead')
  static SkuInput create() => SkuInput._();
  static $pb.GeneratedMessage $_createMessage() => SkuInput._();
  @$core.override
  SkuInput createEmptyInstance() => SkuInput._();
  @$core.pragma('dart2js:noInline')
  static SkuInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SkuInput>(SkuInput.$_createMessage);
  static SkuInput? _defaultInstance;

  /// 第一個軸的選項值，必須是 axes[0].values 之一。
  @$pb.TagNumber(1)
  $core.String get option1 => $_getSZ(0);
  @$pb.TagNumber(1)
  set option1($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOption1() => $_has(0);
  @$pb.TagNumber(1)
  void clearOption1() => $_clearField(1);

  /// 第二個軸的選項值，必須是 axes[1].values 之一。
  @$pb.TagNumber(2)
  $core.String get option2 => $_getSZ(1);
  @$pb.TagNumber(2)
  set option2($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOption2() => $_has(1);
  @$pb.TagNumber(2)
  void clearOption2() => $_clearField(2);

  /// 售價，新台幣元的整數，大於 0。
  @$pb.TagNumber(3)
  $fixnum.Int64 get price => $_getI64(2);
  @$pb.TagNumber(3)
  set price($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPrice() => $_has(2);
  @$pb.TagNumber(3)
  void clearPrice() => $_clearField(3);

  /// 原價（劃線價），新台幣元的整數；沒有設是沒有原價。
  @$pb.TagNumber(4)
  $fixnum.Int64 get compareAtPrice => $_getI64(3);
  @$pb.TagNumber(4)
  set compareAtPrice($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCompareAtPrice() => $_has(3);
  @$pb.TagNumber(4)
  void clearCompareAtPrice() => $_clearField(4);

  /// 新組合的初始庫存，不得為負。
  @$pb.TagNumber(5)
  $core.int get initialStock => $_getIZ(4);
  @$pb.TagNumber(5)
  set initialStock($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInitialStock() => $_has(4);
  @$pb.TagNumber(5)
  void clearInitialStock() => $_clearField(5);
}

class ListStoreProductsRequest extends $pb.GeneratedMessage {
  factory ListStoreProductsRequest({
    MyProductFilter? filter,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListStoreProductsRequest._();
    if (filter != null) result.filter = filter;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListStoreProductsRequest._();

  factory ListStoreProductsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreProductsRequest()..mergeFromBuffer(data, registry);
  factory ListStoreProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListStoreProductsRequest.$_createMessage)
    ..aOM<MyProductFilter>(1, _omitFieldNames ? '' : 'filter',
        subBuilder: MyProductFilter.$_createMessage)
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreProductsRequest copyWith(
          void Function(ListStoreProductsRequest) updates) =>
      super.copyWith((message) => updates(message as ListStoreProductsRequest))
          as ListStoreProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreProductsRequest() / ListStoreProductsRequest.new instead')
  static ListStoreProductsRequest create() => ListStoreProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListStoreProductsRequest._();
  @$core.override
  ListStoreProductsRequest createEmptyInstance() =>
      ListStoreProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStoreProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreProductsRequest>(
          ListStoreProductsRequest.$_createMessage);
  static ListStoreProductsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  MyProductFilter get filter => $_getN(0);
  @$pb.TagNumber(1)
  set filter(MyProductFilter value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearFilter() => $_clearField(1);
  @$pb.TagNumber(1)
  MyProductFilter ensureFilter() => $_ensure(0);

  /// 預設 20，上限 50。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  /// page_token 與 filter 綁定。
  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);
}

class ListStoreProductsResponse extends $pb.GeneratedMessage {
  factory ListStoreProductsResponse({
    $core.Iterable<SellerProductSummary>? products,
    $core.String? nextPageToken,
  }) {
    final result = ListStoreProductsResponse._();
    if (products != null) result.products.addAll(products);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStoreProductsResponse._();

  factory ListStoreProductsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreProductsResponse()..mergeFromBuffer(data, registry);
  factory ListStoreProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ListStoreProductsResponse.$_createMessage)
    ..pPM<SellerProductSummary>(1, _omitFieldNames ? '' : 'products',
        subBuilder: SellerProductSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreProductsResponse copyWith(
          void Function(ListStoreProductsResponse) updates) =>
      super.copyWith((message) => updates(message as ListStoreProductsResponse))
          as ListStoreProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreProductsResponse() / ListStoreProductsResponse.new instead')
  static ListStoreProductsResponse create() => ListStoreProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListStoreProductsResponse._();
  @$core.override
  ListStoreProductsResponse createEmptyInstance() =>
      ListStoreProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStoreProductsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreProductsResponse>(
          ListStoreProductsResponse.$_createMessage);
  static ListStoreProductsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SellerProductSummary> get products => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetStoreProductRequest extends $pb.GeneratedMessage {
  factory GetStoreProductRequest({
    $core.String? productId,
  }) {
    final result = GetStoreProductRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  GetStoreProductRequest._();

  factory GetStoreProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreProductRequest()..mergeFromBuffer(data, registry);
  factory GetStoreProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetStoreProductRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreProductRequest copyWith(
          void Function(GetStoreProductRequest) updates) =>
      super.copyWith((message) => updates(message as GetStoreProductRequest))
          as GetStoreProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreProductRequest() / GetStoreProductRequest.new instead')
  static GetStoreProductRequest create() => GetStoreProductRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetStoreProductRequest._();
  @$core.override
  GetStoreProductRequest createEmptyInstance() => GetStoreProductRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStoreProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreProductRequest>(
          GetStoreProductRequest.$_createMessage);
  static GetStoreProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class GetStoreProductResponse extends $pb.GeneratedMessage {
  factory GetStoreProductResponse({
    SellerProduct? product,
  }) {
    final result = GetStoreProductResponse._();
    if (product != null) result.product = product;
    return result;
  }

  GetStoreProductResponse._();

  factory GetStoreProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreProductResponse()..mergeFromBuffer(data, registry);
  factory GetStoreProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: GetStoreProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreProductResponse copyWith(
          void Function(GetStoreProductResponse) updates) =>
      super.copyWith((message) => updates(message as GetStoreProductResponse))
          as GetStoreProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreProductResponse() / GetStoreProductResponse.new instead')
  static GetStoreProductResponse create() => GetStoreProductResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetStoreProductResponse._();
  @$core.override
  GetStoreProductResponse createEmptyInstance() => GetStoreProductResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStoreProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreProductResponse>(
          GetStoreProductResponse.$_createMessage);
  static GetStoreProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);
}

class CreateStoreProductRequest extends $pb.GeneratedMessage {
  factory CreateStoreProductRequest({
    $3.DelegatedOperation? operation,
    ProductInput? product,
  }) {
    final result = CreateStoreProductRequest._();
    if (operation != null) result.operation = operation;
    if (product != null) result.product = product;
    return result;
  }

  CreateStoreProductRequest._();

  factory CreateStoreProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStoreProductRequest()..mergeFromBuffer(data, registry);
  factory CreateStoreProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStoreProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStoreProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CreateStoreProductRequest.$_createMessage)
    ..aOM<$3.DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: $3.DelegatedOperation.$_createMessage)
    ..aOM<ProductInput>(2, _omitFieldNames ? '' : 'product',
        subBuilder: ProductInput.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoreProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoreProductRequest copyWith(
          void Function(CreateStoreProductRequest) updates) =>
      super.copyWith((message) => updates(message as CreateStoreProductRequest))
          as CreateStoreProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateStoreProductRequest() / CreateStoreProductRequest.new instead')
  static CreateStoreProductRequest create() => CreateStoreProductRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CreateStoreProductRequest._();
  @$core.override
  CreateStoreProductRequest createEmptyInstance() =>
      CreateStoreProductRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateStoreProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStoreProductRequest>(
          CreateStoreProductRequest.$_createMessage);
  static CreateStoreProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation($3.DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.DelegatedOperation ensureOperation() => $_ensure(0);

  /// 完整內容；建立一律是草稿。
  @$pb.TagNumber(2)
  ProductInput get product => $_getN(1);
  @$pb.TagNumber(2)
  set product(ProductInput value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasProduct() => $_has(1);
  @$pb.TagNumber(2)
  void clearProduct() => $_clearField(2);
  @$pb.TagNumber(2)
  ProductInput ensureProduct() => $_ensure(1);
}

class CreateStoreProductResponse extends $pb.GeneratedMessage {
  factory CreateStoreProductResponse({
    SellerProduct? product,
    $core.String? auditLogId,
  }) {
    final result = CreateStoreProductResponse._();
    if (product != null) result.product = product;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  CreateStoreProductResponse._();

  factory CreateStoreProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStoreProductResponse()..mergeFromBuffer(data, registry);
  factory CreateStoreProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStoreProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStoreProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: CreateStoreProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoreProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoreProductResponse copyWith(
          void Function(CreateStoreProductResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CreateStoreProductResponse))
          as CreateStoreProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateStoreProductResponse() / CreateStoreProductResponse.new instead')
  static CreateStoreProductResponse create() => CreateStoreProductResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      CreateStoreProductResponse._();
  @$core.override
  CreateStoreProductResponse createEmptyInstance() =>
      CreateStoreProductResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateStoreProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStoreProductResponse>(
          CreateStoreProductResponse.$_createMessage);
  static CreateStoreProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);

  /// 這次寫入的稽核列 ID（重送時是第一次的）。
  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UpdateStoreProductRequest extends $pb.GeneratedMessage {
  factory UpdateStoreProductRequest({
    $3.DelegatedOperation? operation,
    $core.String? productId,
    $core.String? etag,
    ProductInput? product,
    $4.FieldMask? updateMask,
  }) {
    final result = UpdateStoreProductRequest._();
    if (operation != null) result.operation = operation;
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    if (product != null) result.product = product;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateStoreProductRequest._();

  factory UpdateStoreProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStoreProductRequest()..mergeFromBuffer(data, registry);
  factory UpdateStoreProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStoreProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateStoreProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UpdateStoreProductRequest.$_createMessage)
    ..aOM<$3.DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: $3.DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'productId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..aOM<ProductInput>(4, _omitFieldNames ? '' : 'product',
        subBuilder: ProductInput.$_createMessage)
    ..aOM<$4.FieldMask>(5, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $4.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStoreProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStoreProductRequest copyWith(
          void Function(UpdateStoreProductRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateStoreProductRequest))
          as UpdateStoreProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateStoreProductRequest() / UpdateStoreProductRequest.new instead')
  static UpdateStoreProductRequest create() => UpdateStoreProductRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpdateStoreProductRequest._();
  @$core.override
  UpdateStoreProductRequest createEmptyInstance() =>
      UpdateStoreProductRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateStoreProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateStoreProductRequest>(
          UpdateStoreProductRequest.$_createMessage);
  static UpdateStoreProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation($3.DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.DelegatedOperation ensureOperation() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get productId => $_getSZ(1);
  @$pb.TagNumber(2)
  set productId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProductId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProductId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);

  /// 要修改的欄位的新值；只讀 update_mask 列出的欄位。
  @$pb.TagNumber(4)
  ProductInput get product => $_getN(3);
  @$pb.TagNumber(4)
  set product(ProductInput value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasProduct() => $_has(3);
  @$pb.TagNumber(4)
  void clearProduct() => $_clearField(4);
  @$pb.TagNumber(4)
  ProductInput ensureProduct() => $_ensure(3);

  /// ProductInput 最上層欄位的子集（title、category_id、hashtag_inputs、gallery、description、axes、skus、low_stock_threshold），至少一個；沒列的欄位保留目前的值。
  /// axes 與 skus 要一起列（SKU 的選項值要對得上選項軸）。
  @$pb.TagNumber(5)
  $4.FieldMask get updateMask => $_getN(4);
  @$pb.TagNumber(5)
  set updateMask($4.FieldMask value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasUpdateMask() => $_has(4);
  @$pb.TagNumber(5)
  void clearUpdateMask() => $_clearField(5);
  @$pb.TagNumber(5)
  $4.FieldMask ensureUpdateMask() => $_ensure(4);
}

class UpdateStoreProductResponse extends $pb.GeneratedMessage {
  factory UpdateStoreProductResponse({
    SellerProduct? product,
    $core.String? auditLogId,
  }) {
    final result = UpdateStoreProductResponse._();
    if (product != null) result.product = product;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UpdateStoreProductResponse._();

  factory UpdateStoreProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStoreProductResponse()..mergeFromBuffer(data, registry);
  factory UpdateStoreProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStoreProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateStoreProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UpdateStoreProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStoreProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStoreProductResponse copyWith(
          void Function(UpdateStoreProductResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UpdateStoreProductResponse))
          as UpdateStoreProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateStoreProductResponse() / UpdateStoreProductResponse.new instead')
  static UpdateStoreProductResponse create() => UpdateStoreProductResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpdateStoreProductResponse._();
  @$core.override
  UpdateStoreProductResponse createEmptyInstance() =>
      UpdateStoreProductResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateStoreProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateStoreProductResponse>(
          UpdateStoreProductResponse.$_createMessage);
  static UpdateStoreProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class PublishStoreProductRequest extends $pb.GeneratedMessage {
  factory PublishStoreProductRequest({
    $3.DelegatedOperation? operation,
    $core.String? productId,
    $core.String? etag,
  }) {
    final result = PublishStoreProductRequest._();
    if (operation != null) result.operation = operation;
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    return result;
  }

  PublishStoreProductRequest._();

  factory PublishStoreProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStoreProductRequest()..mergeFromBuffer(data, registry);
  factory PublishStoreProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStoreProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishStoreProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: PublishStoreProductRequest.$_createMessage)
    ..aOM<$3.DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: $3.DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'productId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStoreProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStoreProductRequest copyWith(
          void Function(PublishStoreProductRequest) updates) =>
      super.copyWith(
              (message) => updates(message as PublishStoreProductRequest))
          as PublishStoreProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PublishStoreProductRequest() / PublishStoreProductRequest.new instead')
  static PublishStoreProductRequest create() => PublishStoreProductRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      PublishStoreProductRequest._();
  @$core.override
  PublishStoreProductRequest createEmptyInstance() =>
      PublishStoreProductRequest._();
  @$core.pragma('dart2js:noInline')
  static PublishStoreProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishStoreProductRequest>(
          PublishStoreProductRequest.$_createMessage);
  static PublishStoreProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation($3.DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.DelegatedOperation ensureOperation() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get productId => $_getSZ(1);
  @$pb.TagNumber(2)
  set productId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProductId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProductId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class PublishStoreProductResponse extends $pb.GeneratedMessage {
  factory PublishStoreProductResponse({
    SellerProduct? product,
    $core.String? auditLogId,
  }) {
    final result = PublishStoreProductResponse._();
    if (product != null) result.product = product;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  PublishStoreProductResponse._();

  factory PublishStoreProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStoreProductResponse()..mergeFromBuffer(data, registry);
  factory PublishStoreProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStoreProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishStoreProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: PublishStoreProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStoreProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStoreProductResponse copyWith(
          void Function(PublishStoreProductResponse) updates) =>
      super.copyWith(
              (message) => updates(message as PublishStoreProductResponse))
          as PublishStoreProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PublishStoreProductResponse() / PublishStoreProductResponse.new instead')
  static PublishStoreProductResponse create() =>
      PublishStoreProductResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      PublishStoreProductResponse._();
  @$core.override
  PublishStoreProductResponse createEmptyInstance() =>
      PublishStoreProductResponse._();
  @$core.pragma('dart2js:noInline')
  static PublishStoreProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishStoreProductResponse>(
          PublishStoreProductResponse.$_createMessage);
  static PublishStoreProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UnpublishStoreProductRequest extends $pb.GeneratedMessage {
  factory UnpublishStoreProductRequest({
    $3.DelegatedOperation? operation,
    $core.String? productId,
    $core.String? etag,
  }) {
    final result = UnpublishStoreProductRequest._();
    if (operation != null) result.operation = operation;
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    return result;
  }

  UnpublishStoreProductRequest._();

  factory UnpublishStoreProductRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStoreProductRequest()..mergeFromBuffer(data, registry);
  factory UnpublishStoreProductRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStoreProductRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishStoreProductRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UnpublishStoreProductRequest.$_createMessage)
    ..aOM<$3.DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: $3.DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'productId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStoreProductRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStoreProductRequest copyWith(
          void Function(UnpublishStoreProductRequest) updates) =>
      super.copyWith(
              (message) => updates(message as UnpublishStoreProductRequest))
          as UnpublishStoreProductRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishStoreProductRequest() / UnpublishStoreProductRequest.new instead')
  static UnpublishStoreProductRequest create() =>
      UnpublishStoreProductRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnpublishStoreProductRequest._();
  @$core.override
  UnpublishStoreProductRequest createEmptyInstance() =>
      UnpublishStoreProductRequest._();
  @$core.pragma('dart2js:noInline')
  static UnpublishStoreProductRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishStoreProductRequest>(
          UnpublishStoreProductRequest.$_createMessage);
  static UnpublishStoreProductRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation($3.DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.DelegatedOperation ensureOperation() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get productId => $_getSZ(1);
  @$pb.TagNumber(2)
  set productId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProductId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProductId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class UnpublishStoreProductResponse extends $pb.GeneratedMessage {
  factory UnpublishStoreProductResponse({
    SellerProduct? product,
    $core.String? auditLogId,
  }) {
    final result = UnpublishStoreProductResponse._();
    if (product != null) result.product = product;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UnpublishStoreProductResponse._();

  factory UnpublishStoreProductResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStoreProductResponse()..mergeFromBuffer(data, registry);
  factory UnpublishStoreProductResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStoreProductResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishStoreProductResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UnpublishStoreProductResponse.$_createMessage)
    ..aOM<SellerProduct>(1, _omitFieldNames ? '' : 'product',
        subBuilder: SellerProduct.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStoreProductResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStoreProductResponse copyWith(
          void Function(UnpublishStoreProductResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UnpublishStoreProductResponse))
          as UnpublishStoreProductResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishStoreProductResponse() / UnpublishStoreProductResponse.new instead')
  static UnpublishStoreProductResponse create() =>
      UnpublishStoreProductResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnpublishStoreProductResponse._();
  @$core.override
  UnpublishStoreProductResponse createEmptyInstance() =>
      UnpublishStoreProductResponse._();
  @$core.pragma('dart2js:noInline')
  static UnpublishStoreProductResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishStoreProductResponse>(
          UnpublishStoreProductResponse.$_createMessage);
  static UnpublishStoreProductResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerProduct get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(SellerProduct value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerProduct ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class BatchUnpublishStoreProductsRequest extends $pb.GeneratedMessage {
  factory BatchUnpublishStoreProductsRequest({
    $3.DelegatedOperation? operation,
    $core.Iterable<BatchUnpublishItem>? items,
  }) {
    final result = BatchUnpublishStoreProductsRequest._();
    if (operation != null) result.operation = operation;
    if (items != null) result.items.addAll(items);
    return result;
  }

  BatchUnpublishStoreProductsRequest._();

  factory BatchUnpublishStoreProductsRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishStoreProductsRequest()..mergeFromBuffer(data, registry);
  factory BatchUnpublishStoreProductsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishStoreProductsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchUnpublishStoreProductsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: BatchUnpublishStoreProductsRequest.$_createMessage)
    ..aOM<$3.DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: $3.DelegatedOperation.$_createMessage)
    ..pPM<BatchUnpublishItem>(2, _omitFieldNames ? '' : 'items',
        subBuilder: BatchUnpublishItem.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishStoreProductsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishStoreProductsRequest copyWith(
          void Function(BatchUnpublishStoreProductsRequest) updates) =>
      super.copyWith((message) =>
              updates(message as BatchUnpublishStoreProductsRequest))
          as BatchUnpublishStoreProductsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchUnpublishStoreProductsRequest() / BatchUnpublishStoreProductsRequest.new instead')
  static BatchUnpublishStoreProductsRequest create() =>
      BatchUnpublishStoreProductsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchUnpublishStoreProductsRequest._();
  @$core.override
  BatchUnpublishStoreProductsRequest createEmptyInstance() =>
      BatchUnpublishStoreProductsRequest._();
  @$core.pragma('dart2js:noInline')
  static BatchUnpublishStoreProductsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchUnpublishStoreProductsRequest>(
          BatchUnpublishStoreProductsRequest.$_createMessage);
  static BatchUnpublishStoreProductsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation($3.DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.DelegatedOperation ensureOperation() => $_ensure(0);

  /// 要下架的商品，1 到 50 項，商品 ID 不得重複。
  @$pb.TagNumber(2)
  $pb.PbList<BatchUnpublishItem> get items => $_getList(1);
}

/// BatchUnpublishItem 是批次下架的一項：商品與讀取時拿到的 etag。
class BatchUnpublishItem extends $pb.GeneratedMessage {
  factory BatchUnpublishItem({
    $core.String? productId,
    $core.String? etag,
  }) {
    final result = BatchUnpublishItem._();
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    return result;
  }

  BatchUnpublishItem._();

  factory BatchUnpublishItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishItem()..mergeFromBuffer(data, registry);
  factory BatchUnpublishItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishItem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchUnpublishItem',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: BatchUnpublishItem.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishItem copyWith(void Function(BatchUnpublishItem) updates) =>
      super.copyWith((message) => updates(message as BatchUnpublishItem))
          as BatchUnpublishItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BatchUnpublishItem() / BatchUnpublishItem.new instead')
  static BatchUnpublishItem create() => BatchUnpublishItem._();
  static $pb.GeneratedMessage $_createMessage() => BatchUnpublishItem._();
  @$core.override
  BatchUnpublishItem createEmptyInstance() => BatchUnpublishItem._();
  @$core.pragma('dart2js:noInline')
  static BatchUnpublishItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchUnpublishItem>(
          BatchUnpublishItem.$_createMessage);
  static BatchUnpublishItem? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class BatchUnpublishStoreProductsResponse extends $pb.GeneratedMessage {
  factory BatchUnpublishStoreProductsResponse({
    $core.Iterable<UnpublishedProduct>? products,
    $core.String? auditLogId,
  }) {
    final result = BatchUnpublishStoreProductsResponse._();
    if (products != null) result.products.addAll(products);
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  BatchUnpublishStoreProductsResponse._();

  factory BatchUnpublishStoreProductsResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishStoreProductsResponse()..mergeFromBuffer(data, registry);
  factory BatchUnpublishStoreProductsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchUnpublishStoreProductsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchUnpublishStoreProductsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: BatchUnpublishStoreProductsResponse.$_createMessage)
    ..pPM<UnpublishedProduct>(1, _omitFieldNames ? '' : 'products',
        subBuilder: UnpublishedProduct.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishStoreProductsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchUnpublishStoreProductsResponse copyWith(
          void Function(BatchUnpublishStoreProductsResponse) updates) =>
      super.copyWith((message) =>
              updates(message as BatchUnpublishStoreProductsResponse))
          as BatchUnpublishStoreProductsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchUnpublishStoreProductsResponse() / BatchUnpublishStoreProductsResponse.new instead')
  static BatchUnpublishStoreProductsResponse create() =>
      BatchUnpublishStoreProductsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchUnpublishStoreProductsResponse._();
  @$core.override
  BatchUnpublishStoreProductsResponse createEmptyInstance() =>
      BatchUnpublishStoreProductsResponse._();
  @$core.pragma('dart2js:noInline')
  static BatchUnpublishStoreProductsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
              BatchUnpublishStoreProductsResponse>(
          BatchUnpublishStoreProductsResponse.$_createMessage);
  static BatchUnpublishStoreProductsResponse? _defaultInstance;

  /// 下架後的商品（已經下架的項目也在其中），順序同請求的 items；重送時是它們目前的狀態。
  @$pb.TagNumber(1)
  $pb.PbList<UnpublishedProduct> get products => $_getList(0);

  /// 這一批唯一的稽核列 ID（重送時是第一次的）。
  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

/// UnpublishedProduct 是批次下架後一個商品的狀態：client 以它更新列表上的狀態與 etag，不必重讀整份。
class UnpublishedProduct extends $pb.GeneratedMessage {
  factory UnpublishedProduct({
    $core.String? productId,
    $core.String? etag,
    ProductStatus? status,
  }) {
    final result = UnpublishedProduct._();
    if (productId != null) result.productId = productId;
    if (etag != null) result.etag = etag;
    if (status != null) result.status = status;
    return result;
  }

  UnpublishedProduct._();

  factory UnpublishedProduct.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishedProduct()..mergeFromBuffer(data, registry);
  factory UnpublishedProduct.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishedProduct()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishedProduct',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: UnpublishedProduct.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..aE<ProductStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: ProductStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishedProduct clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishedProduct copyWith(void Function(UnpublishedProduct) updates) =>
      super.copyWith((message) => updates(message as UnpublishedProduct))
          as UnpublishedProduct;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnpublishedProduct() / UnpublishedProduct.new instead')
  static UnpublishedProduct create() => UnpublishedProduct._();
  static $pb.GeneratedMessage $_createMessage() => UnpublishedProduct._();
  @$core.override
  UnpublishedProduct createEmptyInstance() => UnpublishedProduct._();
  @$core.pragma('dart2js:noInline')
  static UnpublishedProduct getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishedProduct>(
          UnpublishedProduct.$_createMessage);
  static UnpublishedProduct? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 下架後的 etag（已經下架的項目是它目前的 etag，沒有改動）。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);

  @$pb.TagNumber(3)
  ProductStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(ProductStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);
}

/// CatalogPublicService 是任何人都能讀的商品、搜尋與分類，不需要登入，回應不依觀看者（可共享快取）。
/// 公開可讀的商品：沒刪除、店家此刻可曝光（seller.pub_exposable_stores，時間用請求的 clock）、狀態是 ACTIVE 或 ARCHIVED；
/// 只有 ACTIVE 的出現在列表與搜尋，ARCHIVED 的商品頁讀得到但 availability 是不可購買。其他一律與不存在相同（not_found）。
/// 公開訊息不帶賣家的狀態 enum，只帶 availability。
class CatalogPublicServiceApi {
  final $pb.RpcClient _client;

  CatalogPublicServiceApi(this._client);

  /// ListProducts 是買家瀏覽的商品列表（分類頁、市集、店家的商品頁）；不帶關鍵字，關鍵字搜尋用 SearchProducts。filter 的欄位全部選填，之間是 AND。
  /// 要看收藏的商品用 BuyerAppService.ListFavorites，追蹤的店家的商品用 BuyerAppService.ListFollowedStoreProducts。
  $async.Future<ListProductsResponse> listProducts(
          $pb.ClientContext? ctx, ListProductsRequest request) =>
      _client.invoke<ListProductsResponse>(ctx, 'CatalogPublicService',
          'ListProducts', request, ListProductsResponse());

  /// SearchProducts 依關鍵字找商品（標題、描述文字、標籤、分類名稱與選項的 searchable_text，pg_trgm），預設依相關度排序。
  $async.Future<SearchProductsResponse> searchProducts(
          $pb.ClientContext? ctx, SearchProductsRequest request) =>
      _client.invoke<SearchProductsResponse>(ctx, 'CatalogPublicService',
          'SearchProducts', request, SearchProductsResponse());

  /// SuggestProductNames 是搜尋框輸入時的建議：商品名稱符合的前幾筆，不分頁，最多 20 筆。
  $async.Future<SuggestProductNamesResponse> suggestProductNames(
          $pb.ClientContext? ctx, SuggestProductNamesRequest request) =>
      _client.invoke<SuggestProductNamesResponse>(ctx, 'CatalogPublicService',
          'SuggestProductNames', request, SuggestProductNamesResponse());

  /// GetProduct 讀一個公開可見的商品的完整內容（含 SKU 與庫存）。ACTIVE 與 ARCHIVED 讀得到（ARCHIVED 的 availability 是不可購買）；
  /// 不存在、草稿、已刪除、店家不可曝光回 not_found。不帶觀看者狀態（是否已收藏、是否訂閱補貨）：那些由 BuyerAppService 提供。
  $async.Future<GetProductResponse> getProduct(
          $pb.ClientContext? ctx, GetProductRequest request) =>
      _client.invoke<GetProductResponse>(ctx, 'CatalogPublicService',
          'GetProduct', request, GetProductResponse());

  /// ListCategories 回整棵平台分類樹（兩層，頂層在前，同層依平台排序）。分類只在 migration 變動，不分頁。
  /// 名稱依 Accept-Language 協商；沒有該語言時是 zh-TW。
  $async.Future<ListCategoriesResponse> listCategories(
          $pb.ClientContext? ctx, ListCategoriesRequest request) =>
      _client.invoke<ListCategoriesResponse>(ctx, 'CatalogPublicService',
          'ListCategories', request, ListCategoriesResponse());

  /// SuggestHashtags 是商品標籤輸入與搜尋的建議：名稱符合的標籤與其商品數（只算可曝光店家的 ACTIVE 商品），商品數多的在前。沒有分頁，最多 20 筆。
  $async.Future<SuggestHashtagsResponse> suggestHashtags(
          $pb.ClientContext? ctx, SuggestHashtagsRequest request) =>
      _client.invoke<SuggestHashtagsResponse>(ctx, 'CatalogPublicService',
          'SuggestHashtags', request, SuggestHashtagsResponse());

  /// SuggestTrendingHashtags 是熱門商品標籤：各標籤的 ACTIVE 商品數（只算可曝光店家），多的在前。只取前 max_results 名，不分頁。
  $async.Future<SuggestTrendingHashtagsResponse> suggestTrendingHashtags(
          $pb.ClientContext? ctx, SuggestTrendingHashtagsRequest request) =>
      _client.invoke<SuggestTrendingHashtagsResponse>(
          ctx,
          'CatalogPublicService',
          'SuggestTrendingHashtags',
          request,
          SuggestTrendingHashtagsResponse());
}

/// SellerCatalogAppService 是賣家管理自己店家的商品。店家由登入身分推導，請求不帶 store_id；
/// 商品不是這家店的、或已刪除，與不存在一樣回 not_found（規範 §二 防枚舉）。每個方法需要賣家能力 CAP_PRODUCT_MANAGE。
///
/// etag 只涵蓋賣家可編輯的內容與狀態：庫存、銷量、收藏數、評分、首次上架時間不改它，所以調整庫存或有人下單不會讓編輯中的表單衝突。
class SellerCatalogAppServiceApi {
  final $pb.RpcClient _client;

  SellerCatalogAppServiceApi(this._client);

  /// ListMyProducts 列出自己店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前。
  $async.Future<ListMyProductsResponse> listMyProducts(
          $pb.ClientContext? ctx, ListMyProductsRequest request) =>
      _client.invoke<ListMyProductsResponse>(ctx, 'SellerCatalogAppService',
          'ListMyProducts', request, ListMyProductsResponse());

  /// GetMyProduct 讀自己店家的商品完整內容（任何狀態，含 etag 與每個 SKU 的庫存）。
  $async.Future<GetMyProductResponse> getMyProduct(
          $pb.ClientContext? ctx, GetMyProductRequest request) =>
      _client.invoke<GetMyProductResponse>(ctx, 'SellerCatalogAppService',
          'GetMyProduct', request, GetMyProductResponse());

  /// CountMyProducts 回自己店家符合篩選的商品數（導覽列的低庫存徽章用 status = ACTIVE、stock_status = LOW）。
  $async.Future<CountMyProductsResponse> countMyProducts(
          $pb.ClientContext? ctx, CountMyProductsRequest request) =>
      _client.invoke<CountMyProductsResponse>(ctx, 'SellerCatalogAppService',
          'CountMyProducts', request, CountMyProductsResponse());

  /// CreateProduct 建立一個草稿商品（建立一律是草稿；要上架呼叫 SetProductStatus）。
  /// 同一家店以同一個 request_id 重送回第一次建立的商品目前的樣子，不重複建立；內容不同回 invalid_argument。
  $async.Future<CreateProductResponse> createProduct(
          $pb.ClientContext? ctx, CreateProductRequest request) =>
      _client.invoke<CreateProductResponse>(ctx, 'SellerCatalogAppService',
          'CreateProduct', request, CreateProductResponse());

  /// UpdateProduct 整體取代商品的內容（狀態不變；等同 AIP-134 的 update_mask = "*"，沒有 update_mask）：沒有帶的圖片、選項軸與 SKU 組合會被移除；既有 SKU 的庫存不在這裡改（用 AdjustStock、SetStock）。
  /// etag 必填（在請求上，因為寫入的 ProductInput 與讀取的 SellerProduct 是不同的型別），與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。商品目前是 ACTIVE 時，輸入至少要有一張主圖，否則回 failed_precondition。
  $async.Future<UpdateProductResponse> updateProduct(
          $pb.ClientContext? ctx, UpdateProductRequest request) =>
      _client.invoke<UpdateProductResponse>(ctx, 'SellerCatalogAppService',
          'UpdateProduct', request, UpdateProductResponse());

  /// SetProductStatus 切換商品狀態：草稿、上架、下架任兩個都能互切，切成目前的狀態也回成功（不改 etag）。
  /// 切成 ACTIVE 時商品至少要有一張主圖，否則回 failed_precondition。首次上架寫入 first_publish_time，之後不清除。
  /// etag 必填（上架是把看到的內容公開，內容在狀態不變時仍可能被別人改動，規範 §七）。
  $async.Future<SetProductStatusResponse> setProductStatus(
          $pb.ClientContext? ctx, SetProductStatusRequest request) =>
      _client.invoke<SetProductStatusResponse>(ctx, 'SellerCatalogAppService',
          'SetProductStatus', request, SetProductStatusResponse());

  /// DeleteProduct 刪除商品：從賣家清單消失、不可還原，列保留（訂單與評價仍引用）。已經刪除的也回成功。etag 選填，帶了就檢查。
  $async.Future<DeleteProductResponse> deleteProduct(
          $pb.ClientContext? ctx, DeleteProductRequest request) =>
      _client.invoke<DeleteProductResponse>(ctx, 'SellerCatalogAppService',
          'DeleteProduct', request, DeleteProductResponse());

  /// AdjustStock 以有號變動量調整自己店家某個 SKU 的庫存（進貨加、出貨減）。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。調整後不得小於 0，否則回 failed_precondition。不改商品的 etag。
  $async.Future<AdjustStockResponse> adjustStock(
          $pb.ClientContext? ctx, AdjustStockRequest request) =>
      _client.invoke<AdjustStockResponse>(ctx, 'SellerCatalogAppService',
          'AdjustStock', request, AdjustStockResponse());

  /// SetStock 把自己店家某個 SKU 的庫存設成絕對值（盤點、商品表單輸入庫存），以 expected_quantity 比對現值：
  /// 現值不等於預期（期間有訂單扣減或別處調整）回 failed_precondition，不寫入，client 重讀後再決定。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。不改商品的 etag。
  $async.Future<SetStockResponse> setStock(
          $pb.ClientContext? ctx, SetStockRequest request) =>
      _client.invoke<SetStockResponse>(ctx, 'SellerCatalogAppService',
          'SetStock', request, SetStockResponse());
}

/// CatalogDelegatedService 是代行營運中的平台管理員讀寫代行店家的商品：憑證是 X-HK-Operation-Session 標頭裡的代行 session（由 website 的 BFF 轉送，不放在請求訊息），
/// 不讀顧客或管理員的身分；目標店家是 session 建立當下綁定的那一家，請求不帶 store_id。回應一律不可快取（private, no-store）。
/// 商品、狀態與內容的規則與 SellerCatalogAppService 相同（同一組模組方法），回應也用同一組賣家訊息（SellerProduct、SellerProductSummary）。
///
/// 能力（products.read、products.write、products.publish）依方法宣告：憑證缺少或無效回 unauthenticated；session 已結束、到期或撤銷回 failed_precondition；
/// session 沒有授予對應的能力回 permission_denied；店家此刻不可代行（不存在、已關閉、等待刪除、停權）回 failed_precondition，只拒絕、不撤銷 session。
///
/// 稽核：成功的寫入在同一個 transaction 內留下一列 SUCCEEDED（寫入失敗時一併 rollback）；被拒絕的請求（含讀取）與寫入的業務失敗另開 transaction 留下一列 FAILED；
/// 成功的讀取不稽核。
class CatalogDelegatedServiceApi {
  final $pb.RpcClient _client;

  CatalogDelegatedServiceApi(this._client);

  /// ListStoreProducts 列出代行店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前（keyset 翻頁）。需要能力 products.read。
  $async.Future<ListStoreProductsResponse> listStoreProducts(
          $pb.ClientContext? ctx, ListStoreProductsRequest request) =>
      _client.invoke<ListStoreProductsResponse>(ctx, 'CatalogDelegatedService',
          'ListStoreProducts', request, ListStoreProductsResponse());

  /// GetStoreProduct 讀代行店家的商品完整內容（任何狀態，含 etag）；別家的商品與不存在一樣回 not_found。需要能力 products.read。
  $async.Future<GetStoreProductResponse> getStoreProduct(
          $pb.ClientContext? ctx, GetStoreProductRequest request) =>
      _client.invoke<GetStoreProductResponse>(ctx, 'CatalogDelegatedService',
          'GetStoreProduct', request, GetStoreProductResponse());

  /// CreateStoreProduct 以代行店家建立草稿商品（建立一律是草稿；要上架再呼叫 PublishStoreProduct）。同一個 session 以同一個 request_id 重送回第一次建立的商品目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 products.write。
  $async.Future<CreateStoreProductResponse> createStoreProduct(
          $pb.ClientContext? ctx, CreateStoreProductRequest request) =>
      _client.invoke<CreateStoreProductResponse>(ctx, 'CatalogDelegatedService',
          'CreateStoreProduct', request, CreateStoreProductResponse());

  /// UpdateStoreProduct 修改代行店家的商品：update_mask 列 ProductInput 最上層欄位的子集，沒列的欄位保留目前的值；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留成功的稽核。重送規則同 CreateStoreProduct。需要能力 products.write。
  $async.Future<UpdateStoreProductResponse> updateStoreProduct(
          $pb.ClientContext? ctx, UpdateStoreProductRequest request) =>
      _client.invoke<UpdateStoreProductResponse>(ctx, 'CatalogDelegatedService',
          'UpdateStoreProduct', request, UpdateStoreProductResponse());

  /// PublishStoreProduct 把商品上架（切成 ACTIVE；商品至少要有一張主圖，否則回 failed_precondition）；etag 必填。商品已經是上架時回成功、不改 etag。重送規則同 CreateStoreProduct。需要能力 products.publish。
  $async.Future<PublishStoreProductResponse> publishStoreProduct(
          $pb.ClientContext? ctx, PublishStoreProductRequest request) =>
      _client.invoke<PublishStoreProductResponse>(
          ctx,
          'CatalogDelegatedService',
          'PublishStoreProduct',
          request,
          PublishStoreProductResponse());

  /// UnpublishStoreProduct 把商品下架（切成 ARCHIVED）；etag 必填。商品已經是下架時回成功、不改 etag。重送規則同 CreateStoreProduct。需要能力 products.publish。
  $async.Future<UnpublishStoreProductResponse> unpublishStoreProduct(
          $pb.ClientContext? ctx, UnpublishStoreProductRequest request) =>
      _client.invoke<UnpublishStoreProductResponse>(
          ctx,
          'CatalogDelegatedService',
          'UnpublishStoreProduct',
          request,
          UnpublishStoreProductResponse());

  /// BatchUnpublishStoreProducts 原子地下架多個商品（AIP-233）：一個 request_id、一個 transaction，全部下架或全部不動；一筆稽核列（PRODUCT_BATCH_UNPUBLISH）記錄本批商品，
  /// 一則賣家通知。任一項的 etag 與目前版本不符，整批拒絕，回 aborted 與 VERSION_CONFLICT，ErrorInfo.metadata 的 conflictingProductIds 列出衝突的商品 ID
  /// （逗號連接、由小到大排列）；已經下架的項目視為成功（不改它的 etag）；任一項不是代行店家的商品回 not_found，整批不動。同一個 request_id 重送回第一次的結果。需要能力 products.publish。
  $async.Future<BatchUnpublishStoreProductsResponse>
      batchUnpublishStoreProducts($pb.ClientContext? ctx,
              BatchUnpublishStoreProductsRequest request) =>
          _client.invoke<BatchUnpublishStoreProductsResponse>(
              ctx,
              'CatalogDelegatedService',
              'BatchUnpublishStoreProducts',
              request,
              BatchUnpublishStoreProductsResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
