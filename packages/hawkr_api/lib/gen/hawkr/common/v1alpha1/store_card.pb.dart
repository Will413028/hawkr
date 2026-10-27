// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/store_card.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'image.pb.dart' as $0;
import 'store_card.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'store_card.pbenum.dart';

/// StoreCard 是各處（訂單、聊天、收藏、追蹤、會員卡、推薦排行、BatchGetStores）顯示一家店的同一張卡片
/// （ADR 2026-10-03-public-store-identity-and-visibility）。
///
/// 可見性：只有「曾經公開過」（曾經可以公開曝光）的店家有卡片；從沒公開過與不存在的店家沒有，持有卡片的訊息以 store 欄位不設
/// 表示「這家店不顯示」，client 不區分兩者（store_id 在過渡期連號、可列舉，沒公開過的店名不能被讀出）。
///
/// 評分（review_count、rating_average）由伺服器批次組裝，每個回應只讀一次評價來源；讀不到時兩欄都不設，不讓整張卡片或呼叫它的 RPC 失敗。
class StoreCard extends $pb.GeneratedMessage {
  factory StoreCard({
    $core.String? id,
    $core.String? storeCode,
    $core.String? name,
    $0.Image? logo,
    $0.Image? coverImage,
    StoreCardStatus? status,
    $core.String? townCode,
    $core.int? reviewCount,
    $core.double? ratingAverage,
  }) {
    final result = StoreCard._();
    if (id != null) result.id = id;
    if (storeCode != null) result.storeCode = storeCode;
    if (name != null) result.name = name;
    if (logo != null) result.logo = logo;
    if (coverImage != null) result.coverImage = coverImage;
    if (status != null) result.status = status;
    if (townCode != null) result.townCode = townCode;
    if (reviewCount != null) result.reviewCount = reviewCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    return result;
  }

  StoreCard._();

  factory StoreCard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreCard()..mergeFromBuffer(data, registry);
  factory StoreCard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreCard()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoreCard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.common.v1alpha1'),
      createEmptyInstance: StoreCard.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeCode')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOM<$0.Image>(4, _omitFieldNames ? '' : 'logo',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<$0.Image>(5, _omitFieldNames ? '' : 'coverImage',
        subBuilder: $0.Image.$_createMessage)
    ..aE<StoreCardStatus>(6, _omitFieldNames ? '' : 'status',
        enumValues: StoreCardStatus.values)
    ..aOS(7, _omitFieldNames ? '' : 'townCode')
    ..aI(8, _omitFieldNames ? '' : 'reviewCount')
    ..aD(9, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreCard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreCard copyWith(void Function(StoreCard) updates) =>
      super.copyWith((message) => updates(message as StoreCard)) as StoreCard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use StoreCard() / StoreCard.new instead')
  static StoreCard create() => StoreCard._();
  static $pb.GeneratedMessage $_createMessage() => StoreCard._();
  @$core.override
  StoreCard createEmptyInstance() => StoreCard._();
  @$core.pragma('dart2js:noInline')
  static StoreCard getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoreCard>(StoreCard.$_createMessage);
  static StoreCard? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 店家網址的路徑段；只有營業中（OPEN）的店家有公開的店面頁。
  @$pb.TagNumber(2)
  $core.String get storeCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set storeCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoreCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoreCode() => $_clearField(2);

  /// 個資清除之後（店家關閉滿保留期，6 個月）不設，client 顯示「已關閉的店家」。
  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  /// 沒有設定時不設；已關閉的店家沒有。
  @$pb.TagNumber(4)
  $0.Image get logo => $_getN(3);
  @$pb.TagNumber(4)
  set logo($0.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLogo() => $_has(3);
  @$pb.TagNumber(4)
  void clearLogo() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Image ensureLogo() => $_ensure(3);

  /// 沒有設定時不設；已關閉的店家沒有。
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
  StoreCardStatus get status => $_getN(5);
  @$pb.TagNumber(6)
  set status(StoreCardStatus value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatus() => $_clearField(6);

  /// 內政部的 8 碼鄉鎮市區代碼（縣市是前 5 碼）；空字串表示不設（沒有填，或個資清除之後）。
  @$pb.TagNumber(7)
  $core.String get townCode => $_getSZ(6);
  @$pb.TagNumber(7)
  set townCode($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasTownCode() => $_has(6);
  @$pb.TagNumber(7)
  void clearTownCode() => $_clearField(7);

  /// 評價數；0 是確實沒有評價，讀取失敗時不設。
  @$pb.TagNumber(8)
  $core.int get reviewCount => $_getIZ(7);
  @$pb.TagNumber(8)
  set reviewCount($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasReviewCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearReviewCount() => $_clearField(8);

  /// 平均評分，1 到 5；沒有評價或讀取失敗時不設。
  @$pb.TagNumber(9)
  $core.double get ratingAverage => $_getN(8);
  @$pb.TagNumber(9)
  set ratingAverage($core.double value) => $_setFloat(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRatingAverage() => $_has(8);
  @$pb.TagNumber(9)
  void clearRatingAverage() => $_clearField(9);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
