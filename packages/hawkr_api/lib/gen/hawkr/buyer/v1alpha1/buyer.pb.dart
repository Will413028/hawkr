// This is a generated file - do not edit.
//
// Generated from hawkr/buyer/v1alpha1/buyer.proto.

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
    as $5;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import '../../../google/type/date.pb.dart' as $0;
import '../../common/v1alpha1/image.pb.dart' as $1;
import '../../common/v1alpha1/postal_address.pb.dart' as $3;
import '../../common/v1alpha1/store_card.pb.dart' as $4;
import 'buyer.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'buyer.pbenum.dart';

/// Profile 是買家的個人資料。
class Profile extends $pb.GeneratedMessage {
  factory Profile({
    $core.String? id,
    $core.String? displayName,
    $core.String? phone,
    Gender? gender,
    $0.Date? birthDate,
    $core.String? city,
    $core.String? job,
    $1.Image? avatar,
    BuyerStatus? status,
    $2.Timestamp? createTime,
    $2.Timestamp? updateTime,
    $core.String? etag,
    $core.String? avatarStagedToken,
  }) {
    final result = Profile._();
    if (id != null) result.id = id;
    if (displayName != null) result.displayName = displayName;
    if (phone != null) result.phone = phone;
    if (gender != null) result.gender = gender;
    if (birthDate != null) result.birthDate = birthDate;
    if (city != null) result.city = city;
    if (job != null) result.job = job;
    if (avatar != null) result.avatar = avatar;
    if (status != null) result.status = status;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (etag != null) result.etag = etag;
    if (avatarStagedToken != null) result.avatarStagedToken = avatarStagedToken;
    return result;
  }

  Profile._();

  factory Profile.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Profile()..mergeFromBuffer(data, registry);
  factory Profile.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Profile()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Profile',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: Profile.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'displayName')
    ..aOS(3, _omitFieldNames ? '' : 'phone')
    ..aE<Gender>(4, _omitFieldNames ? '' : 'gender', enumValues: Gender.values)
    ..aOM<$0.Date>(5, _omitFieldNames ? '' : 'birthDate',
        subBuilder: $0.Date.$_createMessage)
    ..aOS(6, _omitFieldNames ? '' : 'city')
    ..aOS(7, _omitFieldNames ? '' : 'job')
    ..aOM<$1.Image>(8, _omitFieldNames ? '' : 'avatar',
        subBuilder: $1.Image.$_createMessage)
    ..aE<BuyerStatus>(9, _omitFieldNames ? '' : 'status',
        enumValues: BuyerStatus.values)
    ..aOM<$2.Timestamp>(10, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOM<$2.Timestamp>(11, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOS(12, _omitFieldNames ? '' : 'etag')
    ..aOS(13, _omitFieldNames ? '' : 'avatarStagedToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Profile clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Profile copyWith(void Function(Profile) updates) =>
      super.copyWith((message) => updates(message as Profile)) as Profile;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Profile() / Profile.new instead')
  static Profile create() => Profile._();
  static $pb.GeneratedMessage $_createMessage() => Profile._();
  @$core.override
  Profile createEmptyInstance() => Profile._();
  @$core.pragma('dart2js:noInline')
  static Profile getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Profile>(Profile.$_createMessage);
  static Profile? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 暱稱，1–100 字。
  @$pb.TagNumber(2)
  $core.String get displayName => $_getSZ(1);
  @$pb.TagNumber(2)
  set displayName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplayName() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplayName() => $_clearField(2);

  /// 聯絡電話，台灣的手機或市話；空字串表示沒有填。
  @$pb.TagNumber(3)
  $core.String get phone => $_getSZ(2);
  @$pb.TagNumber(3)
  set phone($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPhone() => $_has(2);
  @$pb.TagNumber(3)
  void clearPhone() => $_clearField(3);

  /// 買家自填的性別；沒有填時不設。UpdateProfile 把 gender 列進 update_mask 而沒有設值時清除；不接受 GENDER_UNSPECIFIED。
  @$pb.TagNumber(4)
  Gender get gender => $_getN(3);
  @$pb.TagNumber(4)
  set gender(Gender value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasGender() => $_has(3);
  @$pb.TagNumber(4)
  void clearGender() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.Date get birthDate => $_getN(4);
  @$pb.TagNumber(5)
  set birthDate($0.Date value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasBirthDate() => $_has(4);
  @$pb.TagNumber(5)
  void clearBirthDate() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Date ensureBirthDate() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get city => $_getSZ(5);
  @$pb.TagNumber(6)
  set city($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCity() => $_has(5);
  @$pb.TagNumber(6)
  void clearCity() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get job => $_getSZ(6);
  @$pb.TagNumber(7)
  set job($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasJob() => $_has(6);
  @$pb.TagNumber(7)
  void clearJob() => $_clearField(7);

  /// 沒有設定時不設，client 顯示預設頭像。
  @$pb.TagNumber(8)
  $1.Image get avatar => $_getN(7);
  @$pb.TagNumber(8)
  set avatar($1.Image value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasAvatar() => $_has(7);
  @$pb.TagNumber(8)
  void clearAvatar() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Image ensureAvatar() => $_ensure(7);

  @$pb.TagNumber(9)
  BuyerStatus get status => $_getN(8);
  @$pb.TagNumber(9)
  set status(BuyerStatus value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearStatus() => $_clearField(9);

  @$pb.TagNumber(10)
  $2.Timestamp get createTime => $_getN(9);
  @$pb.TagNumber(10)
  set createTime($2.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreateTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearCreateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $2.Timestamp ensureCreateTime() => $_ensure(9);

  @$pb.TagNumber(11)
  $2.Timestamp get updateTime => $_getN(10);
  @$pb.TagNumber(11)
  set updateTime($2.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasUpdateTime() => $_has(10);
  @$pb.TagNumber(11)
  void clearUpdateTime() => $_clearField(11);
  @$pb.TagNumber(11)
  $2.Timestamp ensureUpdateTime() => $_ensure(10);

  @$pb.TagNumber(12)
  $core.String get etag => $_getSZ(11);
  @$pb.TagNumber(12)
  set etag($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasEtag() => $_has(11);
  @$pb.TagNumber(12)
  void clearEtag() => $_clearField(12);

  /// 上傳完成後拿到的 staged token；換頭像時列進 update_mask，沒有列進去就保留目前的頭像。沒有清掉頭像的操作。
  @$pb.TagNumber(13)
  $core.String get avatarStagedToken => $_getSZ(12);
  @$pb.TagNumber(13)
  set avatarStagedToken($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasAvatarStagedToken() => $_has(12);
  @$pb.TagNumber(13)
  void clearAvatarStagedToken() => $_clearField(13);
}

/// Address 是收貨地址。
class Address extends $pb.GeneratedMessage {
  factory Address({
    $core.String? id,
    $core.String? receiverName,
    $core.String? receiverPhone,
    $core.bool? defaultAddress,
    $2.Timestamp? createTime,
    $2.Timestamp? updateTime,
    $core.String? etag,
    $3.PostalAddress? postalAddress,
  }) {
    final result = Address._();
    if (id != null) result.id = id;
    if (receiverName != null) result.receiverName = receiverName;
    if (receiverPhone != null) result.receiverPhone = receiverPhone;
    if (defaultAddress != null) result.defaultAddress = defaultAddress;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (etag != null) result.etag = etag;
    if (postalAddress != null) result.postalAddress = postalAddress;
    return result;
  }

  Address._();

  factory Address.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Address()..mergeFromBuffer(data, registry);
  factory Address.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Address()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Address',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: Address.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(3, _omitFieldNames ? '' : 'receiverName')
    ..aOS(4, _omitFieldNames ? '' : 'receiverPhone')
    ..aOB(8, _omitFieldNames ? '' : 'defaultAddress')
    ..aOM<$2.Timestamp>(9, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOM<$2.Timestamp>(10, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOS(11, _omitFieldNames ? '' : 'etag')
    ..aOM<$3.PostalAddress>(12, _omitFieldNames ? '' : 'postalAddress',
        subBuilder: $3.PostalAddress.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Address clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Address copyWith(void Function(Address) updates) =>
      super.copyWith((message) => updates(message as Address)) as Address;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Address() / Address.new instead')
  static Address create() => Address._();
  static $pb.GeneratedMessage $_createMessage() => Address._();
  @$core.override
  Address createEmptyInstance() => Address._();
  @$core.pragma('dart2js:noInline')
  static Address getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Address>(Address.$_createMessage);
  static Address? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(3)
  $core.String get receiverName => $_getSZ(1);
  @$pb.TagNumber(3)
  set receiverName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(3)
  $core.bool hasReceiverName() => $_has(1);
  @$pb.TagNumber(3)
  void clearReceiverName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get receiverPhone => $_getSZ(2);
  @$pb.TagNumber(4)
  set receiverPhone($core.String value) => $_setString(2, value);
  @$pb.TagNumber(4)
  $core.bool hasReceiverPhone() => $_has(2);
  @$pb.TagNumber(4)
  void clearReceiverPhone() => $_clearField(4);

  /// 是否為預設地址。
  @$pb.TagNumber(8)
  $core.bool get defaultAddress => $_getBF(3);
  @$pb.TagNumber(8)
  set defaultAddress($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(8)
  $core.bool hasDefaultAddress() => $_has(3);
  @$pb.TagNumber(8)
  void clearDefaultAddress() => $_clearField(8);

  @$pb.TagNumber(9)
  $2.Timestamp get createTime => $_getN(4);
  @$pb.TagNumber(9)
  set createTime($2.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCreateTime() => $_has(4);
  @$pb.TagNumber(9)
  void clearCreateTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $2.Timestamp ensureCreateTime() => $_ensure(4);

  @$pb.TagNumber(10)
  $2.Timestamp get updateTime => $_getN(5);
  @$pb.TagNumber(10)
  set updateTime($2.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasUpdateTime() => $_has(5);
  @$pb.TagNumber(10)
  void clearUpdateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $2.Timestamp ensureUpdateTime() => $_ensure(5);

  @$pb.TagNumber(11)
  $core.String get etag => $_getSZ(6);
  @$pb.TagNumber(11)
  set etag($core.String value) => $_setString(6, value);
  @$pb.TagNumber(11)
  $core.bool hasEtag() => $_has(6);
  @$pb.TagNumber(11)
  void clearEtag() => $_clearField(11);

  /// 收貨的地點；明細必填（1–255 字）。update_mask 列 postal_address 時整個換成請求的地址。
  @$pb.TagNumber(12)
  $3.PostalAddress get postalAddress => $_getN(7);
  @$pb.TagNumber(12)
  set postalAddress($3.PostalAddress value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasPostalAddress() => $_has(7);
  @$pb.TagNumber(12)
  void clearPostalAddress() => $_clearField(12);
  @$pb.TagNumber(12)
  $3.PostalAddress ensurePostalAddress() => $_ensure(7);
}

/// ProductCard 是清單上的商品卡片。
class ProductCard extends $pb.GeneratedMessage {
  factory ProductCard({
    $core.String? id,
    $core.bool? deleted,
    $core.String? name,
    $1.Image? image,
    $fixnum.Int64? minPrice,
    $fixnum.Int64? maxPrice,
    $core.double? rating,
    $core.bool? available,
    $core.bool? restockSubscribed,
    $4.StoreCard? store,
    $core.String? storeId,
  }) {
    final result = ProductCard._();
    if (id != null) result.id = id;
    if (deleted != null) result.deleted = deleted;
    if (name != null) result.name = name;
    if (image != null) result.image = image;
    if (minPrice != null) result.minPrice = minPrice;
    if (maxPrice != null) result.maxPrice = maxPrice;
    if (rating != null) result.rating = rating;
    if (available != null) result.available = available;
    if (restockSubscribed != null) result.restockSubscribed = restockSubscribed;
    if (store != null) result.store = store;
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  ProductCard._();

  factory ProductCard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductCard()..mergeFromBuffer(data, registry);
  factory ProductCard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductCard()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductCard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ProductCard.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOB(2, _omitFieldNames ? '' : 'deleted')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOM<$1.Image>(4, _omitFieldNames ? '' : 'image',
        subBuilder: $1.Image.$_createMessage)
    ..aInt64(5, _omitFieldNames ? '' : 'minPrice')
    ..aInt64(6, _omitFieldNames ? '' : 'maxPrice')
    ..aD(7, _omitFieldNames ? '' : 'rating', fieldType: $pb.PbFieldType.OF)
    ..aOB(8, _omitFieldNames ? '' : 'available')
    ..aOB(9, _omitFieldNames ? '' : 'restockSubscribed')
    ..aOM<$4.StoreCard>(10, _omitFieldNames ? '' : 'store',
        subBuilder: $4.StoreCard.$_createMessage)
    ..aOS(11, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductCard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductCard copyWith(void Function(ProductCard) updates) =>
      super.copyWith((message) => updates(message as ProductCard))
          as ProductCard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductCard() / ProductCard.new instead')
  static ProductCard create() => ProductCard._();
  static $pb.GeneratedMessage $_createMessage() => ProductCard._();
  @$core.override
  ProductCard createEmptyInstance() => ProductCard._();
  @$core.pragma('dart2js:noInline')
  static ProductCard getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductCard>(
          ProductCard.$_createMessage);
  static ProductCard? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 商品已經刪除：只有 id，其他欄位不設，client 顯示「商品已不存在」。
  @$pb.TagNumber(2)
  $core.bool get deleted => $_getBF(1);
  @$pb.TagNumber(2)
  set deleted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeleted() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeleted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Image get image => $_getN(3);
  @$pb.TagNumber(4)
  set image($1.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasImage() => $_has(3);
  @$pb.TagNumber(4)
  void clearImage() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Image ensureImage() => $_ensure(3);

  /// 規格中最低與最高的價格，新台幣元。
  @$pb.TagNumber(5)
  $fixnum.Int64 get minPrice => $_getI64(4);
  @$pb.TagNumber(5)
  set minPrice($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMinPrice() => $_has(4);
  @$pb.TagNumber(5)
  void clearMinPrice() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get maxPrice => $_getI64(5);
  @$pb.TagNumber(6)
  set maxPrice($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasMaxPrice() => $_has(5);
  @$pb.TagNumber(6)
  void clearMaxPrice() => $_clearField(6);

  /// 平均評分 0–5；沒有評價時不設。
  @$pb.TagNumber(7)
  $core.double get rating => $_getN(6);
  @$pb.TagNumber(7)
  set rating($core.double value) => $_setFloat(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRating() => $_has(6);
  @$pb.TagNumber(7)
  void clearRating() => $_clearField(7);

  /// 現在可以購買；下架或售完時是 false。
  @$pb.TagNumber(8)
  $core.bool get available => $_getBF(7);
  @$pb.TagNumber(8)
  set available($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAvailable() => $_has(7);
  @$pb.TagNumber(8)
  void clearAvailable() => $_clearField(8);

  /// 呼叫者已訂閱這個商品的補貨通知。
  @$pb.TagNumber(9)
  $core.bool get restockSubscribed => $_getBF(8);
  @$pb.TagNumber(9)
  set restockSubscribed($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRestockSubscribed() => $_has(8);
  @$pb.TagNumber(9)
  void clearRestockSubscribed() => $_clearField(9);

  /// 商品所屬店家的卡片；不設表示這家店不顯示（從沒公開過或不存在）。
  @$pb.TagNumber(10)
  $4.StoreCard get store => $_getN(9);
  @$pb.TagNumber(10)
  set store($4.StoreCard value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasStore() => $_has(9);
  @$pb.TagNumber(10)
  void clearStore() => $_clearField(10);
  @$pb.TagNumber(10)
  $4.StoreCard ensureStore() => $_ensure(9);

  @$pb.TagNumber(11)
  $core.String get storeId => $_getSZ(10);
  @$pb.TagNumber(11)
  set storeId($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasStoreId() => $_has(10);
  @$pb.TagNumber(11)
  void clearStoreId() => $_clearField(11);
}

/// Favorite 是收藏的商品。
class Favorite extends $pb.GeneratedMessage {
  factory Favorite({
    ProductCard? product,
    $2.Timestamp? createTime,
  }) {
    final result = Favorite._();
    if (product != null) result.product = product;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  Favorite._();

  factory Favorite.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Favorite()..mergeFromBuffer(data, registry);
  factory Favorite.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Favorite()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Favorite',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: Favorite.$_createMessage)
    ..aOM<ProductCard>(1, _omitFieldNames ? '' : 'product',
        subBuilder: ProductCard.$_createMessage)
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Favorite clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Favorite copyWith(void Function(Favorite) updates) =>
      super.copyWith((message) => updates(message as Favorite)) as Favorite;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Favorite() / Favorite.new instead')
  static Favorite create() => Favorite._();
  static $pb.GeneratedMessage $_createMessage() => Favorite._();
  @$core.override
  Favorite createEmptyInstance() => Favorite._();
  @$core.pragma('dart2js:noInline')
  static Favorite getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Favorite>(Favorite.$_createMessage);
  static Favorite? _defaultInstance;

  @$pb.TagNumber(1)
  ProductCard get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(ProductCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  ProductCard ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.Timestamp get createTime => $_getN(1);
  @$pb.TagNumber(2)
  set createTime($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureCreateTime() => $_ensure(1);
}

/// Follow 是追蹤的店家。
class Follow extends $pb.GeneratedMessage {
  factory Follow({
    $4.StoreCard? store,
    $2.Timestamp? createTime,
    $core.String? storeId,
  }) {
    final result = Follow._();
    if (store != null) result.store = store;
    if (createTime != null) result.createTime = createTime;
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  Follow._();

  factory Follow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Follow()..mergeFromBuffer(data, registry);
  factory Follow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Follow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Follow',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: Follow.$_createMessage)
    ..aOM<$4.StoreCard>(1, _omitFieldNames ? '' : 'store',
        subBuilder: $4.StoreCard.$_createMessage)
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Follow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Follow copyWith(void Function(Follow) updates) =>
      super.copyWith((message) => updates(message as Follow)) as Follow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Follow() / Follow.new instead')
  static Follow create() => Follow._();
  static $pb.GeneratedMessage $_createMessage() => Follow._();
  @$core.override
  Follow createEmptyInstance() => Follow._();
  @$core.pragma('dart2js:noInline')
  static Follow getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Follow>(Follow.$_createMessage);
  static Follow? _defaultInstance;

  /// 不設表示這家店不顯示（從沒公開過或不存在）。
  @$pb.TagNumber(1)
  $4.StoreCard get store => $_getN(0);
  @$pb.TagNumber(1)
  set store($4.StoreCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.StoreCard ensureStore() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.Timestamp get createTime => $_getN(1);
  @$pb.TagNumber(2)
  set createTime($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureCreateTime() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get storeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreId() => $_clearField(3);
}

/// Visit 是瀏覽過的商品。
class Visit extends $pb.GeneratedMessage {
  factory Visit({
    ProductCard? product,
    $2.Timestamp? visitTime,
  }) {
    final result = Visit._();
    if (product != null) result.product = product;
    if (visitTime != null) result.visitTime = visitTime;
    return result;
  }

  Visit._();

  factory Visit.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Visit()..mergeFromBuffer(data, registry);
  factory Visit.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Visit()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Visit',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: Visit.$_createMessage)
    ..aOM<ProductCard>(1, _omitFieldNames ? '' : 'product',
        subBuilder: ProductCard.$_createMessage)
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'visitTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Visit clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Visit copyWith(void Function(Visit) updates) =>
      super.copyWith((message) => updates(message as Visit)) as Visit;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Visit() / Visit.new instead')
  static Visit create() => Visit._();
  static $pb.GeneratedMessage $_createMessage() => Visit._();
  @$core.override
  Visit createEmptyInstance() => Visit._();
  @$core.pragma('dart2js:noInline')
  static Visit getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Visit>(Visit.$_createMessage);
  static Visit? _defaultInstance;

  @$pb.TagNumber(1)
  ProductCard get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(ProductCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  ProductCard ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.Timestamp get visitTime => $_getN(1);
  @$pb.TagNumber(2)
  set visitTime($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasVisitTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearVisitTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureVisitTime() => $_ensure(1);
}

/// RestockSubscription 是等待補貨通知的商品。
class RestockSubscription extends $pb.GeneratedMessage {
  factory RestockSubscription({
    ProductCard? product,
    $2.Timestamp? createTime,
  }) {
    final result = RestockSubscription._();
    if (product != null) result.product = product;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  RestockSubscription._();

  factory RestockSubscription.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RestockSubscription()..mergeFromBuffer(data, registry);
  factory RestockSubscription.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RestockSubscription()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RestockSubscription',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: RestockSubscription.$_createMessage)
    ..aOM<ProductCard>(1, _omitFieldNames ? '' : 'product',
        subBuilder: ProductCard.$_createMessage)
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'createTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestockSubscription clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestockSubscription copyWith(void Function(RestockSubscription) updates) =>
      super.copyWith((message) => updates(message as RestockSubscription))
          as RestockSubscription;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use RestockSubscription() / RestockSubscription.new instead')
  static RestockSubscription create() => RestockSubscription._();
  static $pb.GeneratedMessage $_createMessage() => RestockSubscription._();
  @$core.override
  RestockSubscription createEmptyInstance() => RestockSubscription._();
  @$core.pragma('dart2js:noInline')
  static RestockSubscription getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RestockSubscription>(
          RestockSubscription.$_createMessage);
  static RestockSubscription? _defaultInstance;

  @$pb.TagNumber(1)
  ProductCard get product => $_getN(0);
  @$pb.TagNumber(1)
  set product(ProductCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProduct() => $_has(0);
  @$pb.TagNumber(1)
  void clearProduct() => $_clearField(1);
  @$pb.TagNumber(1)
  ProductCard ensureProduct() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.Timestamp get createTime => $_getN(1);
  @$pb.TagNumber(2)
  set createTime($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureCreateTime() => $_ensure(1);
}

/// MemberCard 是買家在一個店家的會員卡。
class MemberCard extends $pb.GeneratedMessage {
  factory MemberCard({
    $4.StoreCard? store,
    $fixnum.Int64? totalSpent,
    $core.bool? vip,
    $core.String? storeId,
  }) {
    final result = MemberCard._();
    if (store != null) result.store = store;
    if (totalSpent != null) result.totalSpent = totalSpent;
    if (vip != null) result.vip = vip;
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  MemberCard._();

  factory MemberCard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MemberCard()..mergeFromBuffer(data, registry);
  factory MemberCard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MemberCard()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberCard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: MemberCard.$_createMessage)
    ..aOM<$4.StoreCard>(1, _omitFieldNames ? '' : 'store',
        subBuilder: $4.StoreCard.$_createMessage)
    ..aInt64(2, _omitFieldNames ? '' : 'totalSpent')
    ..aOB(3, _omitFieldNames ? '' : 'vip')
    ..aOS(4, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberCard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberCard copyWith(void Function(MemberCard) updates) =>
      super.copyWith((message) => updates(message as MemberCard)) as MemberCard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MemberCard() / MemberCard.new instead')
  static MemberCard create() => MemberCard._();
  static $pb.GeneratedMessage $_createMessage() => MemberCard._();
  @$core.override
  MemberCard createEmptyInstance() => MemberCard._();
  @$core.pragma('dart2js:noInline')
  static MemberCard getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberCard>(MemberCard.$_createMessage);
  static MemberCard? _defaultInstance;

  /// 不設表示這家店不顯示（從沒公開過或不存在）。
  @$pb.TagNumber(1)
  $4.StoreCard get store => $_getN(0);
  @$pb.TagNumber(1)
  set store($4.StoreCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.StoreCard ensureStore() => $_ensure(0);

  /// 累計消費額（扣除退款），新台幣元。
  @$pb.TagNumber(2)
  $fixnum.Int64 get totalSpent => $_getI64(1);
  @$pb.TagNumber(2)
  set totalSpent($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalSpent() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalSpent() => $_clearField(2);

  /// 買家是這個店家的 VIP。
  @$pb.TagNumber(3)
  $core.bool get vip => $_getBF(2);
  @$pb.TagNumber(3)
  set vip($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVip() => $_has(2);
  @$pb.TagNumber(3)
  void clearVip() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get storeId => $_getSZ(3);
  @$pb.TagNumber(4)
  set storeId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStoreId() => $_has(3);
  @$pb.TagNumber(4)
  void clearStoreId() => $_clearField(4);
}

/// SpendingRecord 是一筆訂單的消費紀錄。
class SpendingRecord extends $pb.GeneratedMessage {
  factory SpendingRecord({
    $core.String? orderNumber,
    $fixnum.Int64? amount,
    $fixnum.Int64? refundedAmount,
    $core.bool? couponApplied,
    $2.Timestamp? orderTime,
  }) {
    final result = SpendingRecord._();
    if (orderNumber != null) result.orderNumber = orderNumber;
    if (amount != null) result.amount = amount;
    if (refundedAmount != null) result.refundedAmount = refundedAmount;
    if (couponApplied != null) result.couponApplied = couponApplied;
    if (orderTime != null) result.orderTime = orderTime;
    return result;
  }

  SpendingRecord._();

  factory SpendingRecord.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SpendingRecord()..mergeFromBuffer(data, registry);
  factory SpendingRecord.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SpendingRecord()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpendingRecord',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: SpendingRecord.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'orderNumber')
    ..aInt64(2, _omitFieldNames ? '' : 'amount')
    ..aInt64(3, _omitFieldNames ? '' : 'refundedAmount')
    ..aOB(4, _omitFieldNames ? '' : 'couponApplied')
    ..aOM<$2.Timestamp>(5, _omitFieldNames ? '' : 'orderTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpendingRecord clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpendingRecord copyWith(void Function(SpendingRecord) updates) =>
      super.copyWith((message) => updates(message as SpendingRecord))
          as SpendingRecord;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SpendingRecord() / SpendingRecord.new instead')
  static SpendingRecord create() => SpendingRecord._();
  static $pb.GeneratedMessage $_createMessage() => SpendingRecord._();
  @$core.override
  SpendingRecord createEmptyInstance() => SpendingRecord._();
  @$core.pragma('dart2js:noInline')
  static SpendingRecord getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SpendingRecord>(
          SpendingRecord.$_createMessage);
  static SpendingRecord? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get orderNumber => $_getSZ(0);
  @$pb.TagNumber(1)
  set orderNumber($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOrderNumber() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrderNumber() => $_clearField(1);

  /// 訂單金額，新台幣元。
  @$pb.TagNumber(2)
  $fixnum.Int64 get amount => $_getI64(1);
  @$pb.TagNumber(2)
  set amount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAmount() => $_has(1);
  @$pb.TagNumber(2)
  void clearAmount() => $_clearField(2);

  /// 已退款的金額，新台幣元。
  @$pb.TagNumber(3)
  $fixnum.Int64 get refundedAmount => $_getI64(2);
  @$pb.TagNumber(3)
  set refundedAmount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRefundedAmount() => $_has(2);
  @$pb.TagNumber(3)
  void clearRefundedAmount() => $_clearField(3);

  /// 訂單使用了優惠券。
  @$pb.TagNumber(4)
  $core.bool get couponApplied => $_getBF(3);
  @$pb.TagNumber(4)
  set couponApplied($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCouponApplied() => $_has(3);
  @$pb.TagNumber(4)
  void clearCouponApplied() => $_clearField(4);

  @$pb.TagNumber(5)
  $2.Timestamp get orderTime => $_getN(4);
  @$pb.TagNumber(5)
  set orderTime($2.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOrderTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearOrderTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $2.Timestamp ensureOrderTime() => $_ensure(4);
}

class GetProfileRequest extends $pb.GeneratedMessage {
  factory GetProfileRequest() => GetProfileRequest._();

  GetProfileRequest._();

  factory GetProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProfileRequest()..mergeFromBuffer(data, registry);
  factory GetProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProfileRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: GetProfileRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileRequest copyWith(void Function(GetProfileRequest) updates) =>
      super.copyWith((message) => updates(message as GetProfileRequest))
          as GetProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetProfileRequest() / GetProfileRequest.new instead')
  static GetProfileRequest create() => GetProfileRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetProfileRequest._();
  @$core.override
  GetProfileRequest createEmptyInstance() => GetProfileRequest._();
  @$core.pragma('dart2js:noInline')
  static GetProfileRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetProfileRequest>(
          GetProfileRequest.$_createMessage);
  static GetProfileRequest? _defaultInstance;
}

class GetProfileResponse extends $pb.GeneratedMessage {
  factory GetProfileResponse({
    Profile? profile,
  }) {
    final result = GetProfileResponse._();
    if (profile != null) result.profile = profile;
    return result;
  }

  GetProfileResponse._();

  factory GetProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProfileResponse()..mergeFromBuffer(data, registry);
  factory GetProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProfileResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: GetProfileResponse.$_createMessage)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileResponse copyWith(void Function(GetProfileResponse) updates) =>
      super.copyWith((message) => updates(message as GetProfileResponse))
          as GetProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetProfileResponse() / GetProfileResponse.new instead')
  static GetProfileResponse create() => GetProfileResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetProfileResponse._();
  @$core.override
  GetProfileResponse createEmptyInstance() => GetProfileResponse._();
  @$core.pragma('dart2js:noInline')
  static GetProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfileResponse>(
          GetProfileResponse.$_createMessage);
  static GetProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class UpdateProfileRequest extends $pb.GeneratedMessage {
  factory UpdateProfileRequest({
    Profile? profile,
    $5.FieldMask? updateMask,
  }) {
    final result = UpdateProfileRequest._();
    if (profile != null) result.profile = profile;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateProfileRequest._();

  factory UpdateProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProfileRequest()..mergeFromBuffer(data, registry);
  factory UpdateProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProfileRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProfileRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UpdateProfileRequest.$_createMessage)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.$_createMessage)
    ..aOM<$5.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $5.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileRequest copyWith(void Function(UpdateProfileRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateProfileRequest))
          as UpdateProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateProfileRequest() / UpdateProfileRequest.new instead')
  static UpdateProfileRequest create() => UpdateProfileRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateProfileRequest._();
  @$core.override
  UpdateProfileRequest createEmptyInstance() => UpdateProfileRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProfileRequest>(
          UpdateProfileRequest.$_createMessage);
  static UpdateProfileRequest? _defaultInstance;

  /// etag 必填。換頭像時把 avatar_staged_token 設成上傳完成後拿到的 token，並列進 update_mask。
  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);

  @$pb.TagNumber(2)
  $5.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($5.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $5.FieldMask ensureUpdateMask() => $_ensure(1);
}

class UpdateProfileResponse extends $pb.GeneratedMessage {
  factory UpdateProfileResponse({
    Profile? profile,
  }) {
    final result = UpdateProfileResponse._();
    if (profile != null) result.profile = profile;
    return result;
  }

  UpdateProfileResponse._();

  factory UpdateProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProfileResponse()..mergeFromBuffer(data, registry);
  factory UpdateProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateProfileResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProfileResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UpdateProfileResponse.$_createMessage)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileResponse copyWith(
          void Function(UpdateProfileResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateProfileResponse))
          as UpdateProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateProfileResponse() / UpdateProfileResponse.new instead')
  static UpdateProfileResponse create() => UpdateProfileResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateProfileResponse._();
  @$core.override
  UpdateProfileResponse createEmptyInstance() => UpdateProfileResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProfileResponse>(
          UpdateProfileResponse.$_createMessage);
  static UpdateProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class ListAddressesRequest extends $pb.GeneratedMessage {
  factory ListAddressesRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListAddressesRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListAddressesRequest._();

  factory ListAddressesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAddressesRequest()..mergeFromBuffer(data, registry);
  factory ListAddressesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAddressesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAddressesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListAddressesRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAddressesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAddressesRequest copyWith(void Function(ListAddressesRequest) updates) =>
      super.copyWith((message) => updates(message as ListAddressesRequest))
          as ListAddressesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAddressesRequest() / ListAddressesRequest.new instead')
  static ListAddressesRequest create() => ListAddressesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListAddressesRequest._();
  @$core.override
  ListAddressesRequest createEmptyInstance() => ListAddressesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListAddressesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAddressesRequest>(
          ListAddressesRequest.$_createMessage);
  static ListAddressesRequest? _defaultInstance;

  /// 預設 20（等於每個買家的地址上限），上限 100。
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

class ListAddressesResponse extends $pb.GeneratedMessage {
  factory ListAddressesResponse({
    $core.Iterable<Address>? addresses,
    $core.String? nextPageToken,
  }) {
    final result = ListAddressesResponse._();
    if (addresses != null) result.addresses.addAll(addresses);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListAddressesResponse._();

  factory ListAddressesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAddressesResponse()..mergeFromBuffer(data, registry);
  factory ListAddressesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAddressesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAddressesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListAddressesResponse.$_createMessage)
    ..pPM<Address>(1, _omitFieldNames ? '' : 'addresses',
        subBuilder: Address.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAddressesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAddressesResponse copyWith(
          void Function(ListAddressesResponse) updates) =>
      super.copyWith((message) => updates(message as ListAddressesResponse))
          as ListAddressesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAddressesResponse() / ListAddressesResponse.new instead')
  static ListAddressesResponse create() => ListAddressesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListAddressesResponse._();
  @$core.override
  ListAddressesResponse createEmptyInstance() => ListAddressesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListAddressesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAddressesResponse>(
          ListAddressesResponse.$_createMessage);
  static ListAddressesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Address> get addresses => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetAddressRequest extends $pb.GeneratedMessage {
  factory GetAddressRequest({
    $core.String? id,
  }) {
    final result = GetAddressRequest._();
    if (id != null) result.id = id;
    return result;
  }

  GetAddressRequest._();

  factory GetAddressRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAddressRequest()..mergeFromBuffer(data, registry);
  factory GetAddressRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAddressRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAddressRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: GetAddressRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAddressRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAddressRequest copyWith(void Function(GetAddressRequest) updates) =>
      super.copyWith((message) => updates(message as GetAddressRequest))
          as GetAddressRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetAddressRequest() / GetAddressRequest.new instead')
  static GetAddressRequest create() => GetAddressRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetAddressRequest._();
  @$core.override
  GetAddressRequest createEmptyInstance() => GetAddressRequest._();
  @$core.pragma('dart2js:noInline')
  static GetAddressRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetAddressRequest>(
          GetAddressRequest.$_createMessage);
  static GetAddressRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);
}

class GetAddressResponse extends $pb.GeneratedMessage {
  factory GetAddressResponse({
    Address? address,
  }) {
    final result = GetAddressResponse._();
    if (address != null) result.address = address;
    return result;
  }

  GetAddressResponse._();

  factory GetAddressResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAddressResponse()..mergeFromBuffer(data, registry);
  factory GetAddressResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAddressResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAddressResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: GetAddressResponse.$_createMessage)
    ..aOM<Address>(1, _omitFieldNames ? '' : 'address',
        subBuilder: Address.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAddressResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAddressResponse copyWith(void Function(GetAddressResponse) updates) =>
      super.copyWith((message) => updates(message as GetAddressResponse))
          as GetAddressResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetAddressResponse() / GetAddressResponse.new instead')
  static GetAddressResponse create() => GetAddressResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetAddressResponse._();
  @$core.override
  GetAddressResponse createEmptyInstance() => GetAddressResponse._();
  @$core.pragma('dart2js:noInline')
  static GetAddressResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAddressResponse>(
          GetAddressResponse.$_createMessage);
  static GetAddressResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Address get address => $_getN(0);
  @$pb.TagNumber(1)
  set address(Address value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAddress() => $_has(0);
  @$pb.TagNumber(1)
  void clearAddress() => $_clearField(1);
  @$pb.TagNumber(1)
  Address ensureAddress() => $_ensure(0);
}

class CreateAddressRequest extends $pb.GeneratedMessage {
  factory CreateAddressRequest({
    Address? address,
  }) {
    final result = CreateAddressRequest._();
    if (address != null) result.address = address;
    return result;
  }

  CreateAddressRequest._();

  factory CreateAddressRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateAddressRequest()..mergeFromBuffer(data, registry);
  factory CreateAddressRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateAddressRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateAddressRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CreateAddressRequest.$_createMessage)
    ..aOM<Address>(1, _omitFieldNames ? '' : 'address',
        subBuilder: Address.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAddressRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAddressRequest copyWith(void Function(CreateAddressRequest) updates) =>
      super.copyWith((message) => updates(message as CreateAddressRequest))
          as CreateAddressRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateAddressRequest() / CreateAddressRequest.new instead')
  static CreateAddressRequest create() => CreateAddressRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateAddressRequest._();
  @$core.override
  CreateAddressRequest createEmptyInstance() => CreateAddressRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateAddressRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateAddressRequest>(
          CreateAddressRequest.$_createMessage);
  static CreateAddressRequest? _defaultInstance;

  @$pb.TagNumber(1)
  Address get address => $_getN(0);
  @$pb.TagNumber(1)
  set address(Address value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAddress() => $_has(0);
  @$pb.TagNumber(1)
  void clearAddress() => $_clearField(1);
  @$pb.TagNumber(1)
  Address ensureAddress() => $_ensure(0);
}

class CreateAddressResponse extends $pb.GeneratedMessage {
  factory CreateAddressResponse({
    Address? address,
  }) {
    final result = CreateAddressResponse._();
    if (address != null) result.address = address;
    return result;
  }

  CreateAddressResponse._();

  factory CreateAddressResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateAddressResponse()..mergeFromBuffer(data, registry);
  factory CreateAddressResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateAddressResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateAddressResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CreateAddressResponse.$_createMessage)
    ..aOM<Address>(1, _omitFieldNames ? '' : 'address',
        subBuilder: Address.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAddressResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAddressResponse copyWith(
          void Function(CreateAddressResponse) updates) =>
      super.copyWith((message) => updates(message as CreateAddressResponse))
          as CreateAddressResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateAddressResponse() / CreateAddressResponse.new instead')
  static CreateAddressResponse create() => CreateAddressResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateAddressResponse._();
  @$core.override
  CreateAddressResponse createEmptyInstance() => CreateAddressResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateAddressResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateAddressResponse>(
          CreateAddressResponse.$_createMessage);
  static CreateAddressResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Address get address => $_getN(0);
  @$pb.TagNumber(1)
  set address(Address value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAddress() => $_has(0);
  @$pb.TagNumber(1)
  void clearAddress() => $_clearField(1);
  @$pb.TagNumber(1)
  Address ensureAddress() => $_ensure(0);
}

class UpdateAddressRequest extends $pb.GeneratedMessage {
  factory UpdateAddressRequest({
    Address? address,
    $5.FieldMask? updateMask,
  }) {
    final result = UpdateAddressRequest._();
    if (address != null) result.address = address;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateAddressRequest._();

  factory UpdateAddressRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateAddressRequest()..mergeFromBuffer(data, registry);
  factory UpdateAddressRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateAddressRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateAddressRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UpdateAddressRequest.$_createMessage)
    ..aOM<Address>(1, _omitFieldNames ? '' : 'address',
        subBuilder: Address.$_createMessage)
    ..aOM<$5.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $5.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateAddressRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateAddressRequest copyWith(void Function(UpdateAddressRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateAddressRequest))
          as UpdateAddressRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateAddressRequest() / UpdateAddressRequest.new instead')
  static UpdateAddressRequest create() => UpdateAddressRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateAddressRequest._();
  @$core.override
  UpdateAddressRequest createEmptyInstance() => UpdateAddressRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateAddressRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateAddressRequest>(
          UpdateAddressRequest.$_createMessage);
  static UpdateAddressRequest? _defaultInstance;

  /// id 與 etag 必填。
  @$pb.TagNumber(1)
  Address get address => $_getN(0);
  @$pb.TagNumber(1)
  set address(Address value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAddress() => $_has(0);
  @$pb.TagNumber(1)
  void clearAddress() => $_clearField(1);
  @$pb.TagNumber(1)
  Address ensureAddress() => $_ensure(0);

  @$pb.TagNumber(2)
  $5.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($5.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $5.FieldMask ensureUpdateMask() => $_ensure(1);
}

class UpdateAddressResponse extends $pb.GeneratedMessage {
  factory UpdateAddressResponse({
    Address? address,
  }) {
    final result = UpdateAddressResponse._();
    if (address != null) result.address = address;
    return result;
  }

  UpdateAddressResponse._();

  factory UpdateAddressResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateAddressResponse()..mergeFromBuffer(data, registry);
  factory UpdateAddressResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateAddressResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateAddressResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UpdateAddressResponse.$_createMessage)
    ..aOM<Address>(1, _omitFieldNames ? '' : 'address',
        subBuilder: Address.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateAddressResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateAddressResponse copyWith(
          void Function(UpdateAddressResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateAddressResponse))
          as UpdateAddressResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateAddressResponse() / UpdateAddressResponse.new instead')
  static UpdateAddressResponse create() => UpdateAddressResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateAddressResponse._();
  @$core.override
  UpdateAddressResponse createEmptyInstance() => UpdateAddressResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateAddressResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateAddressResponse>(
          UpdateAddressResponse.$_createMessage);
  static UpdateAddressResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Address get address => $_getN(0);
  @$pb.TagNumber(1)
  set address(Address value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAddress() => $_has(0);
  @$pb.TagNumber(1)
  void clearAddress() => $_clearField(1);
  @$pb.TagNumber(1)
  Address ensureAddress() => $_ensure(0);
}

class DeleteAddressRequest extends $pb.GeneratedMessage {
  factory DeleteAddressRequest({
    $core.String? id,
    $core.String? etag,
  }) {
    final result = DeleteAddressRequest._();
    if (id != null) result.id = id;
    if (etag != null) result.etag = etag;
    return result;
  }

  DeleteAddressRequest._();

  factory DeleteAddressRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAddressRequest()..mergeFromBuffer(data, registry);
  factory DeleteAddressRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAddressRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAddressRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: DeleteAddressRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAddressRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAddressRequest copyWith(void Function(DeleteAddressRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteAddressRequest))
          as DeleteAddressRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteAddressRequest() / DeleteAddressRequest.new instead')
  static DeleteAddressRequest create() => DeleteAddressRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteAddressRequest._();
  @$core.override
  DeleteAddressRequest createEmptyInstance() => DeleteAddressRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteAddressRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAddressRequest>(
          DeleteAddressRequest.$_createMessage);
  static DeleteAddressRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 帶了就檢查版本。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class DeleteAddressResponse extends $pb.GeneratedMessage {
  factory DeleteAddressResponse() => DeleteAddressResponse._();

  DeleteAddressResponse._();

  factory DeleteAddressResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAddressResponse()..mergeFromBuffer(data, registry);
  factory DeleteAddressResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAddressResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAddressResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: DeleteAddressResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAddressResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAddressResponse copyWith(
          void Function(DeleteAddressResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteAddressResponse))
          as DeleteAddressResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteAddressResponse() / DeleteAddressResponse.new instead')
  static DeleteAddressResponse create() => DeleteAddressResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteAddressResponse._();
  @$core.override
  DeleteAddressResponse createEmptyInstance() => DeleteAddressResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteAddressResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAddressResponse>(
          DeleteAddressResponse.$_createMessage);
  static DeleteAddressResponse? _defaultInstance;
}

class ListFavoritesRequest extends $pb.GeneratedMessage {
  factory ListFavoritesRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListFavoritesRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListFavoritesRequest._();

  factory ListFavoritesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFavoritesRequest()..mergeFromBuffer(data, registry);
  factory ListFavoritesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFavoritesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFavoritesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListFavoritesRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesRequest copyWith(void Function(ListFavoritesRequest) updates) =>
      super.copyWith((message) => updates(message as ListFavoritesRequest))
          as ListFavoritesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListFavoritesRequest() / ListFavoritesRequest.new instead')
  static ListFavoritesRequest create() => ListFavoritesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListFavoritesRequest._();
  @$core.override
  ListFavoritesRequest createEmptyInstance() => ListFavoritesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListFavoritesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFavoritesRequest>(
          ListFavoritesRequest.$_createMessage);
  static ListFavoritesRequest? _defaultInstance;

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

class ListFavoritesResponse extends $pb.GeneratedMessage {
  factory ListFavoritesResponse({
    $core.Iterable<Favorite>? favorites,
    $core.String? nextPageToken,
    $core.int? totalSize,
  }) {
    final result = ListFavoritesResponse._();
    if (favorites != null) result.favorites.addAll(favorites);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (totalSize != null) result.totalSize = totalSize;
    return result;
  }

  ListFavoritesResponse._();

  factory ListFavoritesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFavoritesResponse()..mergeFromBuffer(data, registry);
  factory ListFavoritesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFavoritesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFavoritesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListFavoritesResponse.$_createMessage)
    ..pPM<Favorite>(1, _omitFieldNames ? '' : 'favorites',
        subBuilder: Favorite.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aI(3, _omitFieldNames ? '' : 'totalSize')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesResponse copyWith(
          void Function(ListFavoritesResponse) updates) =>
      super.copyWith((message) => updates(message as ListFavoritesResponse))
          as ListFavoritesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListFavoritesResponse() / ListFavoritesResponse.new instead')
  static ListFavoritesResponse create() => ListFavoritesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListFavoritesResponse._();
  @$core.override
  ListFavoritesResponse createEmptyInstance() => ListFavoritesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListFavoritesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFavoritesResponse>(
          ListFavoritesResponse.$_createMessage);
  static ListFavoritesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Favorite> get favorites => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 收藏的總數（收藏頁的分頁標籤）。
  @$pb.TagNumber(3)
  $core.int get totalSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalSize() => $_clearField(3);
}

class CheckFavoriteRequest extends $pb.GeneratedMessage {
  factory CheckFavoriteRequest({
    $core.String? productId,
  }) {
    final result = CheckFavoriteRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  CheckFavoriteRequest._();

  factory CheckFavoriteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFavoriteRequest()..mergeFromBuffer(data, registry);
  factory CheckFavoriteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFavoriteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckFavoriteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CheckFavoriteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFavoriteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFavoriteRequest copyWith(void Function(CheckFavoriteRequest) updates) =>
      super.copyWith((message) => updates(message as CheckFavoriteRequest))
          as CheckFavoriteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CheckFavoriteRequest() / CheckFavoriteRequest.new instead')
  static CheckFavoriteRequest create() => CheckFavoriteRequest._();
  static $pb.GeneratedMessage $_createMessage() => CheckFavoriteRequest._();
  @$core.override
  CheckFavoriteRequest createEmptyInstance() => CheckFavoriteRequest._();
  @$core.pragma('dart2js:noInline')
  static CheckFavoriteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckFavoriteRequest>(
          CheckFavoriteRequest.$_createMessage);
  static CheckFavoriteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class CheckFavoriteResponse extends $pb.GeneratedMessage {
  factory CheckFavoriteResponse({
    $core.bool? favorited,
  }) {
    final result = CheckFavoriteResponse._();
    if (favorited != null) result.favorited = favorited;
    return result;
  }

  CheckFavoriteResponse._();

  factory CheckFavoriteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFavoriteResponse()..mergeFromBuffer(data, registry);
  factory CheckFavoriteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFavoriteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckFavoriteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CheckFavoriteResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'favorited')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFavoriteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFavoriteResponse copyWith(
          void Function(CheckFavoriteResponse) updates) =>
      super.copyWith((message) => updates(message as CheckFavoriteResponse))
          as CheckFavoriteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CheckFavoriteResponse() / CheckFavoriteResponse.new instead')
  static CheckFavoriteResponse create() => CheckFavoriteResponse._();
  static $pb.GeneratedMessage $_createMessage() => CheckFavoriteResponse._();
  @$core.override
  CheckFavoriteResponse createEmptyInstance() => CheckFavoriteResponse._();
  @$core.pragma('dart2js:noInline')
  static CheckFavoriteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckFavoriteResponse>(
          CheckFavoriteResponse.$_createMessage);
  static CheckFavoriteResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get favorited => $_getBF(0);
  @$pb.TagNumber(1)
  set favorited($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFavorited() => $_has(0);
  @$pb.TagNumber(1)
  void clearFavorited() => $_clearField(1);
}

class BatchCheckFavoritesRequest extends $pb.GeneratedMessage {
  factory BatchCheckFavoritesRequest({
    $core.Iterable<$core.String>? productIds,
  }) {
    final result = BatchCheckFavoritesRequest._();
    if (productIds != null) result.productIds.addAll(productIds);
    return result;
  }

  BatchCheckFavoritesRequest._();

  factory BatchCheckFavoritesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFavoritesRequest()..mergeFromBuffer(data, registry);
  factory BatchCheckFavoritesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFavoritesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchCheckFavoritesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: BatchCheckFavoritesRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'productIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFavoritesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFavoritesRequest copyWith(
          void Function(BatchCheckFavoritesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as BatchCheckFavoritesRequest))
          as BatchCheckFavoritesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchCheckFavoritesRequest() / BatchCheckFavoritesRequest.new instead')
  static BatchCheckFavoritesRequest create() => BatchCheckFavoritesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchCheckFavoritesRequest._();
  @$core.override
  BatchCheckFavoritesRequest createEmptyInstance() =>
      BatchCheckFavoritesRequest._();
  @$core.pragma('dart2js:noInline')
  static BatchCheckFavoritesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchCheckFavoritesRequest>(
          BatchCheckFavoritesRequest.$_createMessage);
  static BatchCheckFavoritesRequest? _defaultInstance;

  /// 空清單回傳空結果；最多 100 個 ID。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get productIds => $_getList(0);
}

class BatchCheckFavoritesResponse extends $pb.GeneratedMessage {
  factory BatchCheckFavoritesResponse({
    $core.Iterable<$core.String>? favoritedProductIds,
  }) {
    final result = BatchCheckFavoritesResponse._();
    if (favoritedProductIds != null)
      result.favoritedProductIds.addAll(favoritedProductIds);
    return result;
  }

  BatchCheckFavoritesResponse._();

  factory BatchCheckFavoritesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFavoritesResponse()..mergeFromBuffer(data, registry);
  factory BatchCheckFavoritesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFavoritesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchCheckFavoritesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: BatchCheckFavoritesResponse.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'favoritedProductIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFavoritesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFavoritesResponse copyWith(
          void Function(BatchCheckFavoritesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as BatchCheckFavoritesResponse))
          as BatchCheckFavoritesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchCheckFavoritesResponse() / BatchCheckFavoritesResponse.new instead')
  static BatchCheckFavoritesResponse create() =>
      BatchCheckFavoritesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchCheckFavoritesResponse._();
  @$core.override
  BatchCheckFavoritesResponse createEmptyInstance() =>
      BatchCheckFavoritesResponse._();
  @$core.pragma('dart2js:noInline')
  static BatchCheckFavoritesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchCheckFavoritesResponse>(
          BatchCheckFavoritesResponse.$_createMessage);
  static BatchCheckFavoritesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get favoritedProductIds => $_getList(0);
}

class AddFavoriteRequest extends $pb.GeneratedMessage {
  factory AddFavoriteRequest({
    $core.String? productId,
  }) {
    final result = AddFavoriteRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  AddFavoriteRequest._();

  factory AddFavoriteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddFavoriteRequest()..mergeFromBuffer(data, registry);
  factory AddFavoriteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddFavoriteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddFavoriteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: AddFavoriteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddFavoriteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddFavoriteRequest copyWith(void Function(AddFavoriteRequest) updates) =>
      super.copyWith((message) => updates(message as AddFavoriteRequest))
          as AddFavoriteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AddFavoriteRequest() / AddFavoriteRequest.new instead')
  static AddFavoriteRequest create() => AddFavoriteRequest._();
  static $pb.GeneratedMessage $_createMessage() => AddFavoriteRequest._();
  @$core.override
  AddFavoriteRequest createEmptyInstance() => AddFavoriteRequest._();
  @$core.pragma('dart2js:noInline')
  static AddFavoriteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddFavoriteRequest>(
          AddFavoriteRequest.$_createMessage);
  static AddFavoriteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class AddFavoriteResponse extends $pb.GeneratedMessage {
  factory AddFavoriteResponse() => AddFavoriteResponse._();

  AddFavoriteResponse._();

  factory AddFavoriteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddFavoriteResponse()..mergeFromBuffer(data, registry);
  factory AddFavoriteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddFavoriteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddFavoriteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: AddFavoriteResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddFavoriteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddFavoriteResponse copyWith(void Function(AddFavoriteResponse) updates) =>
      super.copyWith((message) => updates(message as AddFavoriteResponse))
          as AddFavoriteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use AddFavoriteResponse() / AddFavoriteResponse.new instead')
  static AddFavoriteResponse create() => AddFavoriteResponse._();
  static $pb.GeneratedMessage $_createMessage() => AddFavoriteResponse._();
  @$core.override
  AddFavoriteResponse createEmptyInstance() => AddFavoriteResponse._();
  @$core.pragma('dart2js:noInline')
  static AddFavoriteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddFavoriteResponse>(
          AddFavoriteResponse.$_createMessage);
  static AddFavoriteResponse? _defaultInstance;
}

class RemoveFavoriteRequest extends $pb.GeneratedMessage {
  factory RemoveFavoriteRequest({
    $core.String? productId,
  }) {
    final result = RemoveFavoriteRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  RemoveFavoriteRequest._();

  factory RemoveFavoriteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveFavoriteRequest()..mergeFromBuffer(data, registry);
  factory RemoveFavoriteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveFavoriteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFavoriteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: RemoveFavoriteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFavoriteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFavoriteRequest copyWith(
          void Function(RemoveFavoriteRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveFavoriteRequest))
          as RemoveFavoriteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RemoveFavoriteRequest() / RemoveFavoriteRequest.new instead')
  static RemoveFavoriteRequest create() => RemoveFavoriteRequest._();
  static $pb.GeneratedMessage $_createMessage() => RemoveFavoriteRequest._();
  @$core.override
  RemoveFavoriteRequest createEmptyInstance() => RemoveFavoriteRequest._();
  @$core.pragma('dart2js:noInline')
  static RemoveFavoriteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFavoriteRequest>(
          RemoveFavoriteRequest.$_createMessage);
  static RemoveFavoriteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class RemoveFavoriteResponse extends $pb.GeneratedMessage {
  factory RemoveFavoriteResponse() => RemoveFavoriteResponse._();

  RemoveFavoriteResponse._();

  factory RemoveFavoriteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveFavoriteResponse()..mergeFromBuffer(data, registry);
  factory RemoveFavoriteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveFavoriteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFavoriteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: RemoveFavoriteResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFavoriteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFavoriteResponse copyWith(
          void Function(RemoveFavoriteResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveFavoriteResponse))
          as RemoveFavoriteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RemoveFavoriteResponse() / RemoveFavoriteResponse.new instead')
  static RemoveFavoriteResponse create() => RemoveFavoriteResponse._();
  static $pb.GeneratedMessage $_createMessage() => RemoveFavoriteResponse._();
  @$core.override
  RemoveFavoriteResponse createEmptyInstance() => RemoveFavoriteResponse._();
  @$core.pragma('dart2js:noInline')
  static RemoveFavoriteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFavoriteResponse>(
          RemoveFavoriteResponse.$_createMessage);
  static RemoveFavoriteResponse? _defaultInstance;
}

class ListFollowsRequest extends $pb.GeneratedMessage {
  factory ListFollowsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListFollowsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListFollowsRequest._();

  factory ListFollowsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFollowsRequest()..mergeFromBuffer(data, registry);
  factory ListFollowsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFollowsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFollowsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListFollowsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFollowsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFollowsRequest copyWith(void Function(ListFollowsRequest) updates) =>
      super.copyWith((message) => updates(message as ListFollowsRequest))
          as ListFollowsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListFollowsRequest() / ListFollowsRequest.new instead')
  static ListFollowsRequest create() => ListFollowsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListFollowsRequest._();
  @$core.override
  ListFollowsRequest createEmptyInstance() => ListFollowsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListFollowsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFollowsRequest>(
          ListFollowsRequest.$_createMessage);
  static ListFollowsRequest? _defaultInstance;

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

class ListFollowsResponse extends $pb.GeneratedMessage {
  factory ListFollowsResponse({
    $core.Iterable<Follow>? follows,
    $core.String? nextPageToken,
    $core.int? totalSize,
  }) {
    final result = ListFollowsResponse._();
    if (follows != null) result.follows.addAll(follows);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (totalSize != null) result.totalSize = totalSize;
    return result;
  }

  ListFollowsResponse._();

  factory ListFollowsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFollowsResponse()..mergeFromBuffer(data, registry);
  factory ListFollowsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListFollowsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFollowsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListFollowsResponse.$_createMessage)
    ..pPM<Follow>(1, _omitFieldNames ? '' : 'follows',
        subBuilder: Follow.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aI(3, _omitFieldNames ? '' : 'totalSize')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFollowsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFollowsResponse copyWith(void Function(ListFollowsResponse) updates) =>
      super.copyWith((message) => updates(message as ListFollowsResponse))
          as ListFollowsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListFollowsResponse() / ListFollowsResponse.new instead')
  static ListFollowsResponse create() => ListFollowsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListFollowsResponse._();
  @$core.override
  ListFollowsResponse createEmptyInstance() => ListFollowsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListFollowsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFollowsResponse>(
          ListFollowsResponse.$_createMessage);
  static ListFollowsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Follow> get follows => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 追蹤的總數（收藏頁的分頁標籤）。
  @$pb.TagNumber(3)
  $core.int get totalSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalSize() => $_clearField(3);
}

class CheckFollowRequest extends $pb.GeneratedMessage {
  factory CheckFollowRequest({
    $core.String? storeId,
  }) {
    final result = CheckFollowRequest._();
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  CheckFollowRequest._();

  factory CheckFollowRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFollowRequest()..mergeFromBuffer(data, registry);
  factory CheckFollowRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFollowRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckFollowRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CheckFollowRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFollowRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFollowRequest copyWith(void Function(CheckFollowRequest) updates) =>
      super.copyWith((message) => updates(message as CheckFollowRequest))
          as CheckFollowRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CheckFollowRequest() / CheckFollowRequest.new instead')
  static CheckFollowRequest create() => CheckFollowRequest._();
  static $pb.GeneratedMessage $_createMessage() => CheckFollowRequest._();
  @$core.override
  CheckFollowRequest createEmptyInstance() => CheckFollowRequest._();
  @$core.pragma('dart2js:noInline')
  static CheckFollowRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckFollowRequest>(
          CheckFollowRequest.$_createMessage);
  static CheckFollowRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);
}

class CheckFollowResponse extends $pb.GeneratedMessage {
  factory CheckFollowResponse({
    $core.bool? following,
  }) {
    final result = CheckFollowResponse._();
    if (following != null) result.following = following;
    return result;
  }

  CheckFollowResponse._();

  factory CheckFollowResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFollowResponse()..mergeFromBuffer(data, registry);
  factory CheckFollowResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckFollowResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckFollowResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: CheckFollowResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'following')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFollowResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckFollowResponse copyWith(void Function(CheckFollowResponse) updates) =>
      super.copyWith((message) => updates(message as CheckFollowResponse))
          as CheckFollowResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CheckFollowResponse() / CheckFollowResponse.new instead')
  static CheckFollowResponse create() => CheckFollowResponse._();
  static $pb.GeneratedMessage $_createMessage() => CheckFollowResponse._();
  @$core.override
  CheckFollowResponse createEmptyInstance() => CheckFollowResponse._();
  @$core.pragma('dart2js:noInline')
  static CheckFollowResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckFollowResponse>(
          CheckFollowResponse.$_createMessage);
  static CheckFollowResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get following => $_getBF(0);
  @$pb.TagNumber(1)
  set following($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFollowing() => $_has(0);
  @$pb.TagNumber(1)
  void clearFollowing() => $_clearField(1);
}

class BatchCheckFollowsRequest extends $pb.GeneratedMessage {
  factory BatchCheckFollowsRequest({
    $core.Iterable<$core.String>? storeIds,
  }) {
    final result = BatchCheckFollowsRequest._();
    if (storeIds != null) result.storeIds.addAll(storeIds);
    return result;
  }

  BatchCheckFollowsRequest._();

  factory BatchCheckFollowsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFollowsRequest()..mergeFromBuffer(data, registry);
  factory BatchCheckFollowsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFollowsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchCheckFollowsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: BatchCheckFollowsRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'storeIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFollowsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFollowsRequest copyWith(
          void Function(BatchCheckFollowsRequest) updates) =>
      super.copyWith((message) => updates(message as BatchCheckFollowsRequest))
          as BatchCheckFollowsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchCheckFollowsRequest() / BatchCheckFollowsRequest.new instead')
  static BatchCheckFollowsRequest create() => BatchCheckFollowsRequest._();
  static $pb.GeneratedMessage $_createMessage() => BatchCheckFollowsRequest._();
  @$core.override
  BatchCheckFollowsRequest createEmptyInstance() =>
      BatchCheckFollowsRequest._();
  @$core.pragma('dart2js:noInline')
  static BatchCheckFollowsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchCheckFollowsRequest>(
          BatchCheckFollowsRequest.$_createMessage);
  static BatchCheckFollowsRequest? _defaultInstance;

  /// 空清單回傳空結果；最多 100 個 ID。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get storeIds => $_getList(0);
}

class BatchCheckFollowsResponse extends $pb.GeneratedMessage {
  factory BatchCheckFollowsResponse({
    $core.Iterable<$core.String>? followedStoreIds,
  }) {
    final result = BatchCheckFollowsResponse._();
    if (followedStoreIds != null)
      result.followedStoreIds.addAll(followedStoreIds);
    return result;
  }

  BatchCheckFollowsResponse._();

  factory BatchCheckFollowsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFollowsResponse()..mergeFromBuffer(data, registry);
  factory BatchCheckFollowsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchCheckFollowsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchCheckFollowsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: BatchCheckFollowsResponse.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'followedStoreIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFollowsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchCheckFollowsResponse copyWith(
          void Function(BatchCheckFollowsResponse) updates) =>
      super.copyWith((message) => updates(message as BatchCheckFollowsResponse))
          as BatchCheckFollowsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchCheckFollowsResponse() / BatchCheckFollowsResponse.new instead')
  static BatchCheckFollowsResponse create() => BatchCheckFollowsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchCheckFollowsResponse._();
  @$core.override
  BatchCheckFollowsResponse createEmptyInstance() =>
      BatchCheckFollowsResponse._();
  @$core.pragma('dart2js:noInline')
  static BatchCheckFollowsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchCheckFollowsResponse>(
          BatchCheckFollowsResponse.$_createMessage);
  static BatchCheckFollowsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get followedStoreIds => $_getList(0);
}

class FollowStoreRequest extends $pb.GeneratedMessage {
  factory FollowStoreRequest({
    $core.String? storeId,
  }) {
    final result = FollowStoreRequest._();
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  FollowStoreRequest._();

  factory FollowStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FollowStoreRequest()..mergeFromBuffer(data, registry);
  factory FollowStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FollowStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FollowStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: FollowStoreRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FollowStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FollowStoreRequest copyWith(void Function(FollowStoreRequest) updates) =>
      super.copyWith((message) => updates(message as FollowStoreRequest))
          as FollowStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use FollowStoreRequest() / FollowStoreRequest.new instead')
  static FollowStoreRequest create() => FollowStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => FollowStoreRequest._();
  @$core.override
  FollowStoreRequest createEmptyInstance() => FollowStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static FollowStoreRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FollowStoreRequest>(
          FollowStoreRequest.$_createMessage);
  static FollowStoreRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);
}

class FollowStoreResponse extends $pb.GeneratedMessage {
  factory FollowStoreResponse() => FollowStoreResponse._();

  FollowStoreResponse._();

  factory FollowStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FollowStoreResponse()..mergeFromBuffer(data, registry);
  factory FollowStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FollowStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FollowStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: FollowStoreResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FollowStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FollowStoreResponse copyWith(void Function(FollowStoreResponse) updates) =>
      super.copyWith((message) => updates(message as FollowStoreResponse))
          as FollowStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use FollowStoreResponse() / FollowStoreResponse.new instead')
  static FollowStoreResponse create() => FollowStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => FollowStoreResponse._();
  @$core.override
  FollowStoreResponse createEmptyInstance() => FollowStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static FollowStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FollowStoreResponse>(
          FollowStoreResponse.$_createMessage);
  static FollowStoreResponse? _defaultInstance;
}

class UnfollowStoreRequest extends $pb.GeneratedMessage {
  factory UnfollowStoreRequest({
    $core.String? storeId,
  }) {
    final result = UnfollowStoreRequest._();
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  UnfollowStoreRequest._();

  factory UnfollowStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnfollowStoreRequest()..mergeFromBuffer(data, registry);
  factory UnfollowStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnfollowStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnfollowStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UnfollowStoreRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnfollowStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnfollowStoreRequest copyWith(void Function(UnfollowStoreRequest) updates) =>
      super.copyWith((message) => updates(message as UnfollowStoreRequest))
          as UnfollowStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnfollowStoreRequest() / UnfollowStoreRequest.new instead')
  static UnfollowStoreRequest create() => UnfollowStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => UnfollowStoreRequest._();
  @$core.override
  UnfollowStoreRequest createEmptyInstance() => UnfollowStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static UnfollowStoreRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnfollowStoreRequest>(
          UnfollowStoreRequest.$_createMessage);
  static UnfollowStoreRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);
}

class UnfollowStoreResponse extends $pb.GeneratedMessage {
  factory UnfollowStoreResponse() => UnfollowStoreResponse._();

  UnfollowStoreResponse._();

  factory UnfollowStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnfollowStoreResponse()..mergeFromBuffer(data, registry);
  factory UnfollowStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnfollowStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnfollowStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UnfollowStoreResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnfollowStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnfollowStoreResponse copyWith(
          void Function(UnfollowStoreResponse) updates) =>
      super.copyWith((message) => updates(message as UnfollowStoreResponse))
          as UnfollowStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnfollowStoreResponse() / UnfollowStoreResponse.new instead')
  static UnfollowStoreResponse create() => UnfollowStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnfollowStoreResponse._();
  @$core.override
  UnfollowStoreResponse createEmptyInstance() => UnfollowStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static UnfollowStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnfollowStoreResponse>(
          UnfollowStoreResponse.$_createMessage);
  static UnfollowStoreResponse? _defaultInstance;
}

class ListMemberCardsRequest extends $pb.GeneratedMessage {
  factory ListMemberCardsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMemberCardsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMemberCardsRequest._();

  factory ListMemberCardsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMemberCardsRequest()..mergeFromBuffer(data, registry);
  factory ListMemberCardsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMemberCardsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMemberCardsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListMemberCardsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMemberCardsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMemberCardsRequest copyWith(
          void Function(ListMemberCardsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMemberCardsRequest))
          as ListMemberCardsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMemberCardsRequest() / ListMemberCardsRequest.new instead')
  static ListMemberCardsRequest create() => ListMemberCardsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMemberCardsRequest._();
  @$core.override
  ListMemberCardsRequest createEmptyInstance() => ListMemberCardsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMemberCardsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMemberCardsRequest>(
          ListMemberCardsRequest.$_createMessage);
  static ListMemberCardsRequest? _defaultInstance;

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

class ListMemberCardsResponse extends $pb.GeneratedMessage {
  factory ListMemberCardsResponse({
    $core.Iterable<MemberCard>? memberCards,
    $core.String? nextPageToken,
  }) {
    final result = ListMemberCardsResponse._();
    if (memberCards != null) result.memberCards.addAll(memberCards);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMemberCardsResponse._();

  factory ListMemberCardsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMemberCardsResponse()..mergeFromBuffer(data, registry);
  factory ListMemberCardsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMemberCardsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMemberCardsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListMemberCardsResponse.$_createMessage)
    ..pPM<MemberCard>(1, _omitFieldNames ? '' : 'memberCards',
        subBuilder: MemberCard.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMemberCardsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMemberCardsResponse copyWith(
          void Function(ListMemberCardsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMemberCardsResponse))
          as ListMemberCardsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMemberCardsResponse() / ListMemberCardsResponse.new instead')
  static ListMemberCardsResponse create() => ListMemberCardsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMemberCardsResponse._();
  @$core.override
  ListMemberCardsResponse createEmptyInstance() => ListMemberCardsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMemberCardsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMemberCardsResponse>(
          ListMemberCardsResponse.$_createMessage);
  static ListMemberCardsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<MemberCard> get memberCards => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListSpendingRecordsRequest extends $pb.GeneratedMessage {
  factory ListSpendingRecordsRequest({
    $core.String? storeId,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListSpendingRecordsRequest._();
    if (storeId != null) result.storeId = storeId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListSpendingRecordsRequest._();

  factory ListSpendingRecordsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSpendingRecordsRequest()..mergeFromBuffer(data, registry);
  factory ListSpendingRecordsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSpendingRecordsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSpendingRecordsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListSpendingRecordsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpendingRecordsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpendingRecordsRequest copyWith(
          void Function(ListSpendingRecordsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListSpendingRecordsRequest))
          as ListSpendingRecordsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSpendingRecordsRequest() / ListSpendingRecordsRequest.new instead')
  static ListSpendingRecordsRequest create() => ListSpendingRecordsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSpendingRecordsRequest._();
  @$core.override
  ListSpendingRecordsRequest createEmptyInstance() =>
      ListSpendingRecordsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListSpendingRecordsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSpendingRecordsRequest>(
          ListSpendingRecordsRequest.$_createMessage);
  static ListSpendingRecordsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 預設 20，上限 100。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);
}

class ListSpendingRecordsResponse extends $pb.GeneratedMessage {
  factory ListSpendingRecordsResponse({
    $core.Iterable<SpendingRecord>? records,
    $core.String? nextPageToken,
    $fixnum.Int64? totalSpent,
    $fixnum.Int64? totalRefunded,
  }) {
    final result = ListSpendingRecordsResponse._();
    if (records != null) result.records.addAll(records);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (totalSpent != null) result.totalSpent = totalSpent;
    if (totalRefunded != null) result.totalRefunded = totalRefunded;
    return result;
  }

  ListSpendingRecordsResponse._();

  factory ListSpendingRecordsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSpendingRecordsResponse()..mergeFromBuffer(data, registry);
  factory ListSpendingRecordsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSpendingRecordsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSpendingRecordsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListSpendingRecordsResponse.$_createMessage)
    ..pPM<SpendingRecord>(1, _omitFieldNames ? '' : 'records',
        subBuilder: SpendingRecord.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aInt64(3, _omitFieldNames ? '' : 'totalSpent')
    ..aInt64(4, _omitFieldNames ? '' : 'totalRefunded')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpendingRecordsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpendingRecordsResponse copyWith(
          void Function(ListSpendingRecordsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListSpendingRecordsResponse))
          as ListSpendingRecordsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSpendingRecordsResponse() / ListSpendingRecordsResponse.new instead')
  static ListSpendingRecordsResponse create() =>
      ListSpendingRecordsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSpendingRecordsResponse._();
  @$core.override
  ListSpendingRecordsResponse createEmptyInstance() =>
      ListSpendingRecordsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListSpendingRecordsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSpendingRecordsResponse>(
          ListSpendingRecordsResponse.$_createMessage);
  static ListSpendingRecordsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SpendingRecord> get records => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 在這個店家的累計消費額與退款額，新台幣元。
  @$pb.TagNumber(3)
  $fixnum.Int64 get totalSpent => $_getI64(2);
  @$pb.TagNumber(3)
  set totalSpent($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalSpent() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalSpent() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get totalRefunded => $_getI64(3);
  @$pb.TagNumber(4)
  set totalRefunded($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTotalRefunded() => $_has(3);
  @$pb.TagNumber(4)
  void clearTotalRefunded() => $_clearField(4);
}

class ListVisitsRequest extends $pb.GeneratedMessage {
  factory ListVisitsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListVisitsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListVisitsRequest._();

  factory ListVisitsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListVisitsRequest()..mergeFromBuffer(data, registry);
  factory ListVisitsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListVisitsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListVisitsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListVisitsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListVisitsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListVisitsRequest copyWith(void Function(ListVisitsRequest) updates) =>
      super.copyWith((message) => updates(message as ListVisitsRequest))
          as ListVisitsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListVisitsRequest() / ListVisitsRequest.new instead')
  static ListVisitsRequest create() => ListVisitsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListVisitsRequest._();
  @$core.override
  ListVisitsRequest createEmptyInstance() => ListVisitsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListVisitsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListVisitsRequest>(
          ListVisitsRequest.$_createMessage);
  static ListVisitsRequest? _defaultInstance;

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

class ListVisitsResponse extends $pb.GeneratedMessage {
  factory ListVisitsResponse({
    $core.Iterable<Visit>? visits,
    $core.String? nextPageToken,
  }) {
    final result = ListVisitsResponse._();
    if (visits != null) result.visits.addAll(visits);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListVisitsResponse._();

  factory ListVisitsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListVisitsResponse()..mergeFromBuffer(data, registry);
  factory ListVisitsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListVisitsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListVisitsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListVisitsResponse.$_createMessage)
    ..pPM<Visit>(1, _omitFieldNames ? '' : 'visits',
        subBuilder: Visit.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListVisitsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListVisitsResponse copyWith(void Function(ListVisitsResponse) updates) =>
      super.copyWith((message) => updates(message as ListVisitsResponse))
          as ListVisitsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListVisitsResponse() / ListVisitsResponse.new instead')
  static ListVisitsResponse create() => ListVisitsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListVisitsResponse._();
  @$core.override
  ListVisitsResponse createEmptyInstance() => ListVisitsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListVisitsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListVisitsResponse>(
          ListVisitsResponse.$_createMessage);
  static ListVisitsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Visit> get visits => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class RecordVisitRequest extends $pb.GeneratedMessage {
  factory RecordVisitRequest({
    $core.String? productId,
  }) {
    final result = RecordVisitRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  RecordVisitRequest._();

  factory RecordVisitRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordVisitRequest()..mergeFromBuffer(data, registry);
  factory RecordVisitRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordVisitRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordVisitRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: RecordVisitRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordVisitRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordVisitRequest copyWith(void Function(RecordVisitRequest) updates) =>
      super.copyWith((message) => updates(message as RecordVisitRequest))
          as RecordVisitRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RecordVisitRequest() / RecordVisitRequest.new instead')
  static RecordVisitRequest create() => RecordVisitRequest._();
  static $pb.GeneratedMessage $_createMessage() => RecordVisitRequest._();
  @$core.override
  RecordVisitRequest createEmptyInstance() => RecordVisitRequest._();
  @$core.pragma('dart2js:noInline')
  static RecordVisitRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordVisitRequest>(
          RecordVisitRequest.$_createMessage);
  static RecordVisitRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class RecordVisitResponse extends $pb.GeneratedMessage {
  factory RecordVisitResponse() => RecordVisitResponse._();

  RecordVisitResponse._();

  factory RecordVisitResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordVisitResponse()..mergeFromBuffer(data, registry);
  factory RecordVisitResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordVisitResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordVisitResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: RecordVisitResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordVisitResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordVisitResponse copyWith(void Function(RecordVisitResponse) updates) =>
      super.copyWith((message) => updates(message as RecordVisitResponse))
          as RecordVisitResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use RecordVisitResponse() / RecordVisitResponse.new instead')
  static RecordVisitResponse create() => RecordVisitResponse._();
  static $pb.GeneratedMessage $_createMessage() => RecordVisitResponse._();
  @$core.override
  RecordVisitResponse createEmptyInstance() => RecordVisitResponse._();
  @$core.pragma('dart2js:noInline')
  static RecordVisitResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordVisitResponse>(
          RecordVisitResponse.$_createMessage);
  static RecordVisitResponse? _defaultInstance;
}

class ClearVisitsRequest extends $pb.GeneratedMessage {
  factory ClearVisitsRequest() => ClearVisitsRequest._();

  ClearVisitsRequest._();

  factory ClearVisitsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearVisitsRequest()..mergeFromBuffer(data, registry);
  factory ClearVisitsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearVisitsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearVisitsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ClearVisitsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVisitsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVisitsRequest copyWith(void Function(ClearVisitsRequest) updates) =>
      super.copyWith((message) => updates(message as ClearVisitsRequest))
          as ClearVisitsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ClearVisitsRequest() / ClearVisitsRequest.new instead')
  static ClearVisitsRequest create() => ClearVisitsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ClearVisitsRequest._();
  @$core.override
  ClearVisitsRequest createEmptyInstance() => ClearVisitsRequest._();
  @$core.pragma('dart2js:noInline')
  static ClearVisitsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearVisitsRequest>(
          ClearVisitsRequest.$_createMessage);
  static ClearVisitsRequest? _defaultInstance;
}

class ClearVisitsResponse extends $pb.GeneratedMessage {
  factory ClearVisitsResponse() => ClearVisitsResponse._();

  ClearVisitsResponse._();

  factory ClearVisitsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearVisitsResponse()..mergeFromBuffer(data, registry);
  factory ClearVisitsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearVisitsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearVisitsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ClearVisitsResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVisitsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVisitsResponse copyWith(void Function(ClearVisitsResponse) updates) =>
      super.copyWith((message) => updates(message as ClearVisitsResponse))
          as ClearVisitsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ClearVisitsResponse() / ClearVisitsResponse.new instead')
  static ClearVisitsResponse create() => ClearVisitsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ClearVisitsResponse._();
  @$core.override
  ClearVisitsResponse createEmptyInstance() => ClearVisitsResponse._();
  @$core.pragma('dart2js:noInline')
  static ClearVisitsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearVisitsResponse>(
          ClearVisitsResponse.$_createMessage);
  static ClearVisitsResponse? _defaultInstance;
}

class ListRestockSubscriptionsRequest extends $pb.GeneratedMessage {
  factory ListRestockSubscriptionsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListRestockSubscriptionsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListRestockSubscriptionsRequest._();

  factory ListRestockSubscriptionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRestockSubscriptionsRequest()..mergeFromBuffer(data, registry);
  factory ListRestockSubscriptionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRestockSubscriptionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRestockSubscriptionsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListRestockSubscriptionsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRestockSubscriptionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRestockSubscriptionsRequest copyWith(
          void Function(ListRestockSubscriptionsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListRestockSubscriptionsRequest))
          as ListRestockSubscriptionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListRestockSubscriptionsRequest() / ListRestockSubscriptionsRequest.new instead')
  static ListRestockSubscriptionsRequest create() =>
      ListRestockSubscriptionsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListRestockSubscriptionsRequest._();
  @$core.override
  ListRestockSubscriptionsRequest createEmptyInstance() =>
      ListRestockSubscriptionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListRestockSubscriptionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListRestockSubscriptionsRequest>(
          ListRestockSubscriptionsRequest.$_createMessage);
  static ListRestockSubscriptionsRequest? _defaultInstance;

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

class ListRestockSubscriptionsResponse extends $pb.GeneratedMessage {
  factory ListRestockSubscriptionsResponse({
    $core.Iterable<RestockSubscription>? restockSubscriptions,
    $core.String? nextPageToken,
  }) {
    final result = ListRestockSubscriptionsResponse._();
    if (restockSubscriptions != null)
      result.restockSubscriptions.addAll(restockSubscriptions);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListRestockSubscriptionsResponse._();

  factory ListRestockSubscriptionsResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRestockSubscriptionsResponse()..mergeFromBuffer(data, registry);
  factory ListRestockSubscriptionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRestockSubscriptionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRestockSubscriptionsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: ListRestockSubscriptionsResponse.$_createMessage)
    ..pPM<RestockSubscription>(1, _omitFieldNames ? '' : 'restockSubscriptions',
        subBuilder: RestockSubscription.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRestockSubscriptionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRestockSubscriptionsResponse copyWith(
          void Function(ListRestockSubscriptionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListRestockSubscriptionsResponse))
          as ListRestockSubscriptionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListRestockSubscriptionsResponse() / ListRestockSubscriptionsResponse.new instead')
  static ListRestockSubscriptionsResponse create() =>
      ListRestockSubscriptionsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListRestockSubscriptionsResponse._();
  @$core.override
  ListRestockSubscriptionsResponse createEmptyInstance() =>
      ListRestockSubscriptionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListRestockSubscriptionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListRestockSubscriptionsResponse>(
          ListRestockSubscriptionsResponse.$_createMessage);
  static ListRestockSubscriptionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<RestockSubscription> get restockSubscriptions => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class SubscribeRestockRequest extends $pb.GeneratedMessage {
  factory SubscribeRestockRequest({
    $core.String? productId,
  }) {
    final result = SubscribeRestockRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  SubscribeRestockRequest._();

  factory SubscribeRestockRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubscribeRestockRequest()..mergeFromBuffer(data, registry);
  factory SubscribeRestockRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubscribeRestockRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubscribeRestockRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: SubscribeRestockRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRestockRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRestockRequest copyWith(
          void Function(SubscribeRestockRequest) updates) =>
      super.copyWith((message) => updates(message as SubscribeRestockRequest))
          as SubscribeRestockRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubscribeRestockRequest() / SubscribeRestockRequest.new instead')
  static SubscribeRestockRequest create() => SubscribeRestockRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubscribeRestockRequest._();
  @$core.override
  SubscribeRestockRequest createEmptyInstance() => SubscribeRestockRequest._();
  @$core.pragma('dart2js:noInline')
  static SubscribeRestockRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubscribeRestockRequest>(
          SubscribeRestockRequest.$_createMessage);
  static SubscribeRestockRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class SubscribeRestockResponse extends $pb.GeneratedMessage {
  factory SubscribeRestockResponse() => SubscribeRestockResponse._();

  SubscribeRestockResponse._();

  factory SubscribeRestockResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubscribeRestockResponse()..mergeFromBuffer(data, registry);
  factory SubscribeRestockResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubscribeRestockResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubscribeRestockResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: SubscribeRestockResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRestockResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRestockResponse copyWith(
          void Function(SubscribeRestockResponse) updates) =>
      super.copyWith((message) => updates(message as SubscribeRestockResponse))
          as SubscribeRestockResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubscribeRestockResponse() / SubscribeRestockResponse.new instead')
  static SubscribeRestockResponse create() => SubscribeRestockResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubscribeRestockResponse._();
  @$core.override
  SubscribeRestockResponse createEmptyInstance() =>
      SubscribeRestockResponse._();
  @$core.pragma('dart2js:noInline')
  static SubscribeRestockResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubscribeRestockResponse>(
          SubscribeRestockResponse.$_createMessage);
  static SubscribeRestockResponse? _defaultInstance;
}

class UnsubscribeRestockRequest extends $pb.GeneratedMessage {
  factory UnsubscribeRestockRequest({
    $core.String? productId,
  }) {
    final result = UnsubscribeRestockRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  UnsubscribeRestockRequest._();

  factory UnsubscribeRestockRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnsubscribeRestockRequest()..mergeFromBuffer(data, registry);
  factory UnsubscribeRestockRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnsubscribeRestockRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnsubscribeRestockRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UnsubscribeRestockRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnsubscribeRestockRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnsubscribeRestockRequest copyWith(
          void Function(UnsubscribeRestockRequest) updates) =>
      super.copyWith((message) => updates(message as UnsubscribeRestockRequest))
          as UnsubscribeRestockRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnsubscribeRestockRequest() / UnsubscribeRestockRequest.new instead')
  static UnsubscribeRestockRequest create() => UnsubscribeRestockRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnsubscribeRestockRequest._();
  @$core.override
  UnsubscribeRestockRequest createEmptyInstance() =>
      UnsubscribeRestockRequest._();
  @$core.pragma('dart2js:noInline')
  static UnsubscribeRestockRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnsubscribeRestockRequest>(
          UnsubscribeRestockRequest.$_createMessage);
  static UnsubscribeRestockRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class UnsubscribeRestockResponse extends $pb.GeneratedMessage {
  factory UnsubscribeRestockResponse() => UnsubscribeRestockResponse._();

  UnsubscribeRestockResponse._();

  factory UnsubscribeRestockResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnsubscribeRestockResponse()..mergeFromBuffer(data, registry);
  factory UnsubscribeRestockResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnsubscribeRestockResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnsubscribeRestockResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.buyer.v1alpha1'),
      createEmptyInstance: UnsubscribeRestockResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnsubscribeRestockResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnsubscribeRestockResponse copyWith(
          void Function(UnsubscribeRestockResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UnsubscribeRestockResponse))
          as UnsubscribeRestockResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnsubscribeRestockResponse() / UnsubscribeRestockResponse.new instead')
  static UnsubscribeRestockResponse create() => UnsubscribeRestockResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnsubscribeRestockResponse._();
  @$core.override
  UnsubscribeRestockResponse createEmptyInstance() =>
      UnsubscribeRestockResponse._();
  @$core.pragma('dart2js:noInline')
  static UnsubscribeRestockResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnsubscribeRestockResponse>(
          UnsubscribeRestockResponse.$_createMessage);
  static UnsubscribeRestockResponse? _defaultInstance;
}

/// BuyerAppService 是登入的帳號的方法。以呼叫者的買家身分操作，帳號沒有買家角色時回 permission_denied。
class BuyerAppServiceApi {
  final $pb.RpcClient _client;

  BuyerAppServiceApi(this._client);

  /// GetProfile 回傳呼叫者的買家 profile。
  $async.Future<GetProfileResponse> getProfile(
          $pb.ClientContext? ctx, GetProfileRequest request) =>
      _client.invoke<GetProfileResponse>(
          ctx, 'BuyerAppService', 'GetProfile', request, GetProfileResponse());

  /// UpdateProfile 修改 profile 的欄位（update_mask），含頭像（avatar_staged_token）。etag 必填；換頭像與文字欄位在
  /// 同一個版本條件下一起寫入，任何一項失敗都不會寫入；沒有任何欄位改變時不寫入，etag 不變。
  $async.Future<UpdateProfileResponse> updateProfile(
          $pb.ClientContext? ctx, UpdateProfileRequest request) =>
      _client.invoke<UpdateProfileResponse>(ctx, 'BuyerAppService',
          'UpdateProfile', request, UpdateProfileResponse());

  /// ListAddresses 列出收貨地址，預設地址在最前，其餘依修改時間新到舊。每個買家最多 20 筆（見 CreateAddress），
  /// 預設頁大小等於上限，所以通常一頁就是完整清單；client 仍要照 next_page_token 讀到底，上限調整時不會被截斷。
  $async.Future<ListAddressesResponse> listAddresses(
          $pb.ClientContext? ctx, ListAddressesRequest request) =>
      _client.invoke<ListAddressesResponse>(ctx, 'BuyerAppService',
          'ListAddresses', request, ListAddressesResponse());

  /// GetAddress 回傳一筆收貨地址。
  $async.Future<GetAddressResponse> getAddress(
          $pb.ClientContext? ctx, GetAddressRequest request) =>
      _client.invoke<GetAddressResponse>(
          ctx, 'BuyerAppService', 'GetAddress', request, GetAddressResponse());

  /// CreateAddress 新增收貨地址。第一筆地址自動成為預設地址。每個買家最多 20 筆：已有 20 筆時回
  /// failed_precondition（帶 LocalizedMessage），要先刪掉一筆。
  $async.Future<CreateAddressResponse> createAddress(
          $pb.ClientContext? ctx, CreateAddressRequest request) =>
      _client.invoke<CreateAddressResponse>(ctx, 'BuyerAppService',
          'CreateAddress', request, CreateAddressResponse());

  /// UpdateAddress 修改收貨地址。default_address 設為 true 時取代原本的預設地址；有地址時一定有一筆預設，
  /// 所以不能把預設地址改成 false，要改選另一筆。
  $async.Future<UpdateAddressResponse> updateAddress(
          $pb.ClientContext? ctx, UpdateAddressRequest request) =>
      _client.invoke<UpdateAddressResponse>(ctx, 'BuyerAppService',
          'UpdateAddress', request, UpdateAddressResponse());

  /// DeleteAddress 刪除收貨地址。刪掉的是預設地址時，最近修改的另一筆成為預設。
  $async.Future<DeleteAddressResponse> deleteAddress(
          $pb.ClientContext? ctx, DeleteAddressRequest request) =>
      _client.invoke<DeleteAddressResponse>(ctx, 'BuyerAppService',
          'DeleteAddress', request, DeleteAddressResponse());

  /// ListFavorites 列出收藏的商品，新到舊。
  $async.Future<ListFavoritesResponse> listFavorites(
          $pb.ClientContext? ctx, ListFavoritesRequest request) =>
      _client.invoke<ListFavoritesResponse>(ctx, 'BuyerAppService',
          'ListFavorites', request, ListFavoritesResponse());

  /// CheckFavorite 回傳商品是否已收藏。
  $async.Future<CheckFavoriteResponse> checkFavorite(
          $pb.ClientContext? ctx, CheckFavoriteRequest request) =>
      _client.invoke<CheckFavoriteResponse>(ctx, 'BuyerAppService',
          'CheckFavorite', request, CheckFavoriteResponse());

  /// BatchCheckFavorites 回傳畫面上已收藏的商品 ID；一次最多 100 個。
  $async.Future<BatchCheckFavoritesResponse> batchCheckFavorites(
          $pb.ClientContext? ctx, BatchCheckFavoritesRequest request) =>
      _client.invoke<BatchCheckFavoritesResponse>(ctx, 'BuyerAppService',
          'BatchCheckFavorites', request, BatchCheckFavoritesResponse());

  /// AddFavorite 收藏商品；已收藏時也成功。帳號在刪除流程中（等待訂單完成）時回 failed_precondition。
  $async.Future<AddFavoriteResponse> addFavorite(
          $pb.ClientContext? ctx, AddFavoriteRequest request) =>
      _client.invoke<AddFavoriteResponse>(ctx, 'BuyerAppService', 'AddFavorite',
          request, AddFavoriteResponse());

  /// RemoveFavorite 取消收藏；沒有收藏時也成功。
  $async.Future<RemoveFavoriteResponse> removeFavorite(
          $pb.ClientContext? ctx, RemoveFavoriteRequest request) =>
      _client.invoke<RemoveFavoriteResponse>(ctx, 'BuyerAppService',
          'RemoveFavorite', request, RemoveFavoriteResponse());

  /// ListFollows 列出追蹤的店家，新到舊。
  $async.Future<ListFollowsResponse> listFollows(
          $pb.ClientContext? ctx, ListFollowsRequest request) =>
      _client.invoke<ListFollowsResponse>(ctx, 'BuyerAppService', 'ListFollows',
          request, ListFollowsResponse());

  /// CheckFollow 回傳是否已追蹤店家。
  $async.Future<CheckFollowResponse> checkFollow(
          $pb.ClientContext? ctx, CheckFollowRequest request) =>
      _client.invoke<CheckFollowResponse>(ctx, 'BuyerAppService', 'CheckFollow',
          request, CheckFollowResponse());

  /// BatchCheckFollows 回傳畫面上已追蹤的店家 ID；一次最多 100 個。
  $async.Future<BatchCheckFollowsResponse> batchCheckFollows(
          $pb.ClientContext? ctx, BatchCheckFollowsRequest request) =>
      _client.invoke<BatchCheckFollowsResponse>(ctx, 'BuyerAppService',
          'BatchCheckFollows', request, BatchCheckFollowsResponse());

  /// FollowStore 追蹤店家；已追蹤時也成功。店家不存在或不公開時回 not_found，追蹤自己的店回 invalid_argument，
  /// 帳號在刪除流程中時回 failed_precondition。
  $async.Future<FollowStoreResponse> followStore(
          $pb.ClientContext? ctx, FollowStoreRequest request) =>
      _client.invoke<FollowStoreResponse>(ctx, 'BuyerAppService', 'FollowStore',
          request, FollowStoreResponse());

  /// UnfollowStore 取消追蹤；沒有追蹤時也成功。
  $async.Future<UnfollowStoreResponse> unfollowStore(
          $pb.ClientContext? ctx, UnfollowStoreRequest request) =>
      _client.invoke<UnfollowStoreResponse>(ctx, 'BuyerAppService',
          'UnfollowStore', request, UnfollowStoreResponse());

  /// ListMemberCards 列出買家消費過的店家與累計消費額（會員卡），消費額高到低。
  $async.Future<ListMemberCardsResponse> listMemberCards(
          $pb.ClientContext? ctx, ListMemberCardsRequest request) =>
      _client.invoke<ListMemberCardsResponse>(ctx, 'BuyerAppService',
          'ListMemberCards', request, ListMemberCardsResponse());

  /// ListSpendingRecords 列出買家在一個店家的消費紀錄，新到舊。沒有紀錄時回空的清單。
  $async.Future<ListSpendingRecordsResponse> listSpendingRecords(
          $pb.ClientContext? ctx, ListSpendingRecordsRequest request) =>
      _client.invoke<ListSpendingRecordsResponse>(ctx, 'BuyerAppService',
          'ListSpendingRecords', request, ListSpendingRecordsResponse());

  /// ListVisits 列出瀏覽過的商品，依最後瀏覽時間新到舊。
  $async.Future<ListVisitsResponse> listVisits(
          $pb.ClientContext? ctx, ListVisitsRequest request) =>
      _client.invoke<ListVisitsResponse>(
          ctx, 'BuyerAppService', 'ListVisits', request, ListVisitsResponse());

  /// RecordVisit 記錄一次商品瀏覽；每次呼叫都算一次瀏覽，client 不自動重送。
  $async.Future<RecordVisitResponse> recordVisit(
          $pb.ClientContext? ctx, RecordVisitRequest request) =>
      _client.invoke<RecordVisitResponse>(ctx, 'BuyerAppService', 'RecordVisit',
          request, RecordVisitResponse());

  /// ClearVisits 清除全部瀏覽紀錄。
  $async.Future<ClearVisitsResponse> clearVisits(
          $pb.ClientContext? ctx, ClearVisitsRequest request) =>
      _client.invoke<ClearVisitsResponse>(ctx, 'BuyerAppService', 'ClearVisits',
          request, ClearVisitsResponse());

  /// ListRestockSubscriptions 列出等待補貨通知的商品，新到舊。通知送出或取消之後就不在清單上。
  $async.Future<ListRestockSubscriptionsResponse> listRestockSubscriptions(
          $pb.ClientContext? ctx, ListRestockSubscriptionsRequest request) =>
      _client.invoke<ListRestockSubscriptionsResponse>(
          ctx,
          'BuyerAppService',
          'ListRestockSubscriptions',
          request,
          ListRestockSubscriptionsResponse());

  /// SubscribeRestock 訂閱商品的補貨通知；已訂閱時也成功。
  $async.Future<SubscribeRestockResponse> subscribeRestock(
          $pb.ClientContext? ctx, SubscribeRestockRequest request) =>
      _client.invoke<SubscribeRestockResponse>(ctx, 'BuyerAppService',
          'SubscribeRestock', request, SubscribeRestockResponse());

  /// UnsubscribeRestock 取消補貨通知；沒有訂閱時也成功。
  $async.Future<UnsubscribeRestockResponse> unsubscribeRestock(
          $pb.ClientContext? ctx, UnsubscribeRestockRequest request) =>
      _client.invoke<UnsubscribeRestockResponse>(ctx, 'BuyerAppService',
          'UnsubscribeRestock', request, UnsubscribeRestockResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
