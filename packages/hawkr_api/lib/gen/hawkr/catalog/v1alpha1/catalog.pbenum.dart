// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ProductStatus 是商品的狀態（賣家與後台看得到，公開訊息不帶）。
class ProductStatus extends $pb.ProtobufEnum {
  static const ProductStatus PRODUCT_STATUS_UNSPECIFIED =
      ProductStatus._(0, _omitEnumNames ? '' : 'PRODUCT_STATUS_UNSPECIFIED');

  /// 草稿：隱藏，只有賣家看得到。
  static const ProductStatus PRODUCT_STATUS_DRAFT =
      ProductStatus._(1, _omitEnumNames ? '' : 'PRODUCT_STATUS_DRAFT');

  /// 上架：公開可見（店家可曝光時），可購買（有庫存時）。
  static const ProductStatus PRODUCT_STATUS_ACTIVE =
      ProductStatus._(2, _omitEnumNames ? '' : 'PRODUCT_STATUS_ACTIVE');

  /// 下架：賣家自己停售，可還原；商品頁保留（收藏與分享連結仍可讀），但不可購買，不出現在列表與搜尋。要完全隱藏請改回草稿。
  static const ProductStatus PRODUCT_STATUS_ARCHIVED =
      ProductStatus._(3, _omitEnumNames ? '' : 'PRODUCT_STATUS_ARCHIVED');

  static const $core.List<ProductStatus> values = <ProductStatus>[
    PRODUCT_STATUS_UNSPECIFIED,
    PRODUCT_STATUS_DRAFT,
    PRODUCT_STATUS_ACTIVE,
    PRODUCT_STATUS_ARCHIVED,
  ];

  static final $core.List<ProductStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ProductStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ProductStatus._(super.value, super.name);
}

/// ProductAvailability 是公開訊息裡商品或 SKU 現在能不能買。公開訊息只帶它，不帶賣家的狀態。
class ProductAvailability extends $pb.ProtobufEnum {
  static const ProductAvailability PRODUCT_AVAILABILITY_UNSPECIFIED =
      ProductAvailability._(
          0, _omitEnumNames ? '' : 'PRODUCT_AVAILABILITY_UNSPECIFIED');

  /// 可購買：上架中（ACTIVE）而且有庫存。
  static const ProductAvailability PRODUCT_AVAILABILITY_AVAILABLE =
      ProductAvailability._(
          1, _omitEnumNames ? '' : 'PRODUCT_AVAILABILITY_AVAILABLE');

  /// 不可購買：已下架（ARCHIVED）或沒有庫存。client 以它決定能不能加入購物車，不自己由庫存推導。
  static const ProductAvailability PRODUCT_AVAILABILITY_UNAVAILABLE =
      ProductAvailability._(
          2, _omitEnumNames ? '' : 'PRODUCT_AVAILABILITY_UNAVAILABLE');

  static const $core.List<ProductAvailability> values = <ProductAvailability>[
    PRODUCT_AVAILABILITY_UNSPECIFIED,
    PRODUCT_AVAILABILITY_AVAILABLE,
    PRODUCT_AVAILABILITY_UNAVAILABLE,
  ];

  static final $core.List<ProductAvailability?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ProductAvailability? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ProductAvailability._(super.value, super.name);
}

/// StockStatus 是賣家清單與計數依庫存的篩選。兩個值都只比對 ACTIVE 的商品（草稿與下架不在賣，提醒是噪音）。
class StockStatus extends $pb.ProtobufEnum {
  static const StockStatus STOCK_STATUS_UNSPECIFIED =
      StockStatus._(0, _omitEnumNames ? '' : 'STOCK_STATUS_UNSPECIFIED');

  /// 庫存偏低：至少有一個沒刪除的 SKU 庫存大於 0 且小於或等於商品的低庫存門檻（沒設用平台預設值）。
  static const StockStatus STOCK_STATUS_LOW =
      StockStatus._(1, _omitEnumNames ? '' : 'STOCK_STATUS_LOW');

  /// 售完：至少有一個沒刪除的 SKU 庫存是 0。與 LOW 各自獨立計算。
  static const StockStatus STOCK_STATUS_OUT_OF_STOCK =
      StockStatus._(2, _omitEnumNames ? '' : 'STOCK_STATUS_OUT_OF_STOCK');

  static const $core.List<StockStatus> values = <StockStatus>[
    STOCK_STATUS_UNSPECIFIED,
    STOCK_STATUS_LOW,
    STOCK_STATUS_OUT_OF_STOCK,
  ];

  static final $core.List<StockStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static StockStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StockStatus._(super.value, super.name);
}

/// ProductSort 是商品列表與搜尋的排序。
class ProductSort extends $pb.ProtobufEnum {
  /// 不指定：ListProducts 是 LATEST，SearchProducts 是 RELEVANCE。
  static const ProductSort PRODUCT_SORT_UNSPECIFIED =
      ProductSort._(0, _omitEnumNames ? '' : 'PRODUCT_SORT_UNSPECIFIED');

  /// 最新上架（first_publish_time 遞減）。
  static const ProductSort PRODUCT_SORT_LATEST =
      ProductSort._(1, _omitEnumNames ? '' : 'PRODUCT_SORT_LATEST');

  /// 價格低到高（依 min_price）。
  static const ProductSort PRODUCT_SORT_PRICE_ASC =
      ProductSort._(2, _omitEnumNames ? '' : 'PRODUCT_SORT_PRICE_ASC');

  /// 價格高到低（依 max_price）。
  static const ProductSort PRODUCT_SORT_PRICE_DESC =
      ProductSort._(3, _omitEnumNames ? '' : 'PRODUCT_SORT_PRICE_DESC');

  /// 銷量多到少。
  static const ProductSort PRODUCT_SORT_SALES =
      ProductSort._(4, _omitEnumNames ? '' : 'PRODUCT_SORT_SALES');

  /// 評分高到低。
  static const ProductSort PRODUCT_SORT_RATING =
      ProductSort._(5, _omitEnumNames ? '' : 'PRODUCT_SORT_RATING');

  /// 相關度（只有 SearchProducts 接受；ListProducts 收到回 invalid_argument）。
  static const ProductSort PRODUCT_SORT_RELEVANCE =
      ProductSort._(6, _omitEnumNames ? '' : 'PRODUCT_SORT_RELEVANCE');

  static const $core.List<ProductSort> values = <ProductSort>[
    PRODUCT_SORT_UNSPECIFIED,
    PRODUCT_SORT_LATEST,
    PRODUCT_SORT_PRICE_ASC,
    PRODUCT_SORT_PRICE_DESC,
    PRODUCT_SORT_SALES,
    PRODUCT_SORT_RATING,
    PRODUCT_SORT_RELEVANCE,
  ];

  static final $core.List<ProductSort?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static ProductSort? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ProductSort._(super.value, super.name);
}

/// HashtagMatch 是多個 hashtag 的比對方式。
class HashtagMatch extends $pb.ProtobufEnum {
  /// 不指定：ANY。
  static const HashtagMatch HASHTAG_MATCH_UNSPECIFIED =
      HashtagMatch._(0, _omitEnumNames ? '' : 'HASHTAG_MATCH_UNSPECIFIED');

  /// 商品帶有其中任何一個。
  static const HashtagMatch HASHTAG_MATCH_ANY =
      HashtagMatch._(1, _omitEnumNames ? '' : 'HASHTAG_MATCH_ANY');

  /// 商品帶有全部。
  static const HashtagMatch HASHTAG_MATCH_ALL =
      HashtagMatch._(2, _omitEnumNames ? '' : 'HASHTAG_MATCH_ALL');

  static const $core.List<HashtagMatch> values = <HashtagMatch>[
    HASHTAG_MATCH_UNSPECIFIED,
    HASHTAG_MATCH_ANY,
    HASHTAG_MATCH_ALL,
  ];

  static final $core.List<HashtagMatch?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static HashtagMatch? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const HashtagMatch._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
