// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/seller.proto.

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
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as $6;
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart'
    as $4;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../../google/rpc/error_details.pb.dart' as $3;
import '../../../google/type/date.pb.dart' as $5;
import '../../common/v1alpha1/image.pb.dart' as $0;
import '../../common/v1alpha1/policy_category.pbenum.dart' as $8;
import '../../common/v1alpha1/postal_address.pb.dart' as $2;
import '../../common/v1alpha1/store_card.pb.dart' as $7;
import 'seller.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'seller.pbenum.dart';

/// AgreementAcceptance 是賣家對一份文件的同意：client 送回它顯示給賣家的版本，伺服器核對後記錄。
class AgreementAcceptance extends $pb.GeneratedMessage {
  factory AgreementAcceptance({
    AgreementKind? kind,
    $core.String? version,
  }) {
    final result = AgreementAcceptance._();
    if (kind != null) result.kind = kind;
    if (version != null) result.version = version;
    return result;
  }

  AgreementAcceptance._();

  factory AgreementAcceptance.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgreementAcceptance()..mergeFromBuffer(data, registry);
  factory AgreementAcceptance.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgreementAcceptance()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgreementAcceptance',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: AgreementAcceptance.$_createMessage)
    ..aE<AgreementKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: AgreementKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'version')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgreementAcceptance clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgreementAcceptance copyWith(void Function(AgreementAcceptance) updates) =>
      super.copyWith((message) => updates(message as AgreementAcceptance))
          as AgreementAcceptance;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use AgreementAcceptance() / AgreementAcceptance.new instead')
  static AgreementAcceptance create() => AgreementAcceptance._();
  static $pb.GeneratedMessage $_createMessage() => AgreementAcceptance._();
  @$core.override
  AgreementAcceptance createEmptyInstance() => AgreementAcceptance._();
  @$core.pragma('dart2js:noInline')
  static AgreementAcceptance getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AgreementAcceptance>(
          AgreementAcceptance.$_createMessage);
  static AgreementAcceptance? _defaultInstance;

  /// 同意的文件種類，不能是 UNSPECIFIED。
  @$pb.TagNumber(1)
  AgreementKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(AgreementKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  /// client 顯示給賣家的版本（取自 GetSellerAgreements 的 SellerAgreement.version），與現行版本不同時回
  /// failed_precondition，ErrorInfo.reason 是 AGREEMENT_OUTDATED：client 重讀 GetSellerAgreements，請賣家重新勾選。
  @$pb.TagNumber(2)
  $core.String get version => $_getSZ(1);
  @$pb.TagNumber(2)
  set version($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearVersion() => $_clearField(2);
}

/// SellerAgreement 是一份入駐文件的現行版本。
class SellerAgreement extends $pb.GeneratedMessage {
  factory SellerAgreement({
    AgreementKind? kind,
    $core.String? version,
    $core.String? url,
  }) {
    final result = SellerAgreement._();
    if (kind != null) result.kind = kind;
    if (version != null) result.version = version;
    if (url != null) result.url = url;
    return result;
  }

  SellerAgreement._();

  factory SellerAgreement.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerAgreement()..mergeFromBuffer(data, registry);
  factory SellerAgreement.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerAgreement()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerAgreement',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SellerAgreement.$_createMessage)
    ..aE<AgreementKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: AgreementKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'version')
    ..aOS(3, _omitFieldNames ? '' : 'url')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerAgreement clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerAgreement copyWith(void Function(SellerAgreement) updates) =>
      super.copyWith((message) => updates(message as SellerAgreement))
          as SellerAgreement;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerAgreement() / SellerAgreement.new instead')
  static SellerAgreement create() => SellerAgreement._();
  static $pb.GeneratedMessage $_createMessage() => SellerAgreement._();
  @$core.override
  SellerAgreement createEmptyInstance() => SellerAgreement._();
  @$core.pragma('dart2js:noInline')
  static SellerAgreement getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerAgreement>(
          SellerAgreement.$_createMessage);
  static SellerAgreement? _defaultInstance;

  @$pb.TagNumber(1)
  AgreementKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(AgreementKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  /// 現行版本，值是生效日（YYYY-MM-DD）；各版本的全文以 repo 裡的正本保存。
  @$pb.TagNumber(2)
  $core.String get version => $_getSZ(1);
  @$pb.TagNumber(2)
  set version($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearVersion() => $_clearField(2);

  /// 文件的位置，一律是絕對網址（站內文件由伺服器接上網站網域）。
  @$pb.TagNumber(3)
  $core.String get url => $_getSZ(2);
  @$pb.TagNumber(3)
  set url($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearUrl() => $_clearField(3);
}

/// Store 是賣家的店家資料。
class Store extends $pb.GeneratedMessage {
  factory Store({
    $core.String? id,
    $core.String? storeCode,
    $core.String? storeName,
    $core.String? description,
    $core.String? contactPhone,
    $core.String? prize,
    $core.String? websiteUrl,
    $core.bool? onboardingCompleted,
    $0.Image? logo,
    $0.Image? coverImage,
    StoreStatus? status,
    $1.Timestamp? createTime,
    $1.Timestamp? updateTime,
    $core.String? etag,
    $core.String? logoStagedToken,
    $core.String? coverStagedToken,
    $2.PostalAddress? address,
    $core.String? contactName,
    $core.String? contactMobile,
    $core.String? supportEmail,
  }) {
    final result = Store._();
    if (id != null) result.id = id;
    if (storeCode != null) result.storeCode = storeCode;
    if (storeName != null) result.storeName = storeName;
    if (description != null) result.description = description;
    if (contactPhone != null) result.contactPhone = contactPhone;
    if (prize != null) result.prize = prize;
    if (websiteUrl != null) result.websiteUrl = websiteUrl;
    if (onboardingCompleted != null)
      result.onboardingCompleted = onboardingCompleted;
    if (logo != null) result.logo = logo;
    if (coverImage != null) result.coverImage = coverImage;
    if (status != null) result.status = status;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (etag != null) result.etag = etag;
    if (logoStagedToken != null) result.logoStagedToken = logoStagedToken;
    if (coverStagedToken != null) result.coverStagedToken = coverStagedToken;
    if (address != null) result.address = address;
    if (contactName != null) result.contactName = contactName;
    if (contactMobile != null) result.contactMobile = contactMobile;
    if (supportEmail != null) result.supportEmail = supportEmail;
    return result;
  }

  Store._();

  factory Store.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Store()..mergeFromBuffer(data, registry);
  factory Store.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Store()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Store',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: Store.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeCode')
    ..aOS(3, _omitFieldNames ? '' : 'storeName')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'contactPhone')
    ..aOS(8, _omitFieldNames ? '' : 'prize')
    ..aOS(9, _omitFieldNames ? '' : 'websiteUrl')
    ..aOB(12, _omitFieldNames ? '' : 'onboardingCompleted')
    ..aOM<$0.Image>(13, _omitFieldNames ? '' : 'logo',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<$0.Image>(14, _omitFieldNames ? '' : 'coverImage',
        subBuilder: $0.Image.$_createMessage)
    ..aE<StoreStatus>(15, _omitFieldNames ? '' : 'status',
        enumValues: StoreStatus.values)
    ..aOM<$1.Timestamp>(16, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(17, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(18, _omitFieldNames ? '' : 'etag')
    ..aOS(19, _omitFieldNames ? '' : 'logoStagedToken')
    ..aOS(20, _omitFieldNames ? '' : 'coverStagedToken')
    ..aOM<$2.PostalAddress>(21, _omitFieldNames ? '' : 'address',
        subBuilder: $2.PostalAddress.$_createMessage)
    ..aOS(23, _omitFieldNames ? '' : 'contactName')
    ..aOS(24, _omitFieldNames ? '' : 'contactMobile')
    ..aOS(25, _omitFieldNames ? '' : 'supportEmail')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Store clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Store copyWith(void Function(Store) updates) =>
      super.copyWith((message) => updates(message as Store)) as Store;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Store() / Store.new instead')
  static Store create() => Store._();
  static $pb.GeneratedMessage $_createMessage() => Store._();
  @$core.override
  Store createEmptyInstance() => Store._();
  @$core.pragma('dart2js:noInline')
  static Store getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Store>(Store.$_createMessage);
  static Store? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 店家網址的路徑段。
  @$pb.TagNumber(2)
  $core.String get storeCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set storeCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoreCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoreCode() => $_clearField(2);

  /// 店名，1–100 字。
  @$pb.TagNumber(3)
  $core.String get storeName => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreName() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreName() => $_clearField(3);

  /// 店家介紹，最多 2000 字；空字串表示沒有填。
  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  /// 商店電話，台灣的手機或市話（PayUni 的 MerTel）；空字串表示沒有填。
  @$pb.TagNumber(5)
  $core.String get contactPhone => $_getSZ(4);
  @$pb.TagNumber(5)
  set contactPhone($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasContactPhone() => $_has(4);
  @$pb.TagNumber(5)
  void clearContactPhone() => $_clearField(5);

  /// 得獎紀錄，最多 255 字。
  @$pb.TagNumber(8)
  $core.String get prize => $_getSZ(5);
  @$pb.TagNumber(8)
  set prize($core.String value) => $_setString(5, value);
  @$pb.TagNumber(8)
  $core.bool hasPrize() => $_has(5);
  @$pb.TagNumber(8)
  void clearPrize() => $_clearField(8);

  /// 官網，http 或 https 網址。
  @$pb.TagNumber(9)
  $core.String get websiteUrl => $_getSZ(6);
  @$pb.TagNumber(9)
  set websiteUrl($core.String value) => $_setString(6, value);
  @$pb.TagNumber(9)
  $core.bool hasWebsiteUrl() => $_has(6);
  @$pb.TagNumber(9)
  void clearWebsiteUrl() => $_clearField(9);

  /// 已完成新手引導；由 CompleteMyStoreOnboarding 設定，完成之後不能改回未完成。
  @$pb.TagNumber(12)
  $core.bool get onboardingCompleted => $_getBF(7);
  @$pb.TagNumber(12)
  set onboardingCompleted($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(12)
  $core.bool hasOnboardingCompleted() => $_has(7);
  @$pb.TagNumber(12)
  void clearOnboardingCompleted() => $_clearField(12);

  /// 沒有設定時不設。
  @$pb.TagNumber(13)
  $0.Image get logo => $_getN(8);
  @$pb.TagNumber(13)
  set logo($0.Image value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasLogo() => $_has(8);
  @$pb.TagNumber(13)
  void clearLogo() => $_clearField(13);
  @$pb.TagNumber(13)
  $0.Image ensureLogo() => $_ensure(8);

  /// 沒有設定時不設。
  @$pb.TagNumber(14)
  $0.Image get coverImage => $_getN(9);
  @$pb.TagNumber(14)
  set coverImage($0.Image value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasCoverImage() => $_has(9);
  @$pb.TagNumber(14)
  void clearCoverImage() => $_clearField(14);
  @$pb.TagNumber(14)
  $0.Image ensureCoverImage() => $_ensure(9);

  @$pb.TagNumber(15)
  StoreStatus get status => $_getN(10);
  @$pb.TagNumber(15)
  set status(StoreStatus value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasStatus() => $_has(10);
  @$pb.TagNumber(15)
  void clearStatus() => $_clearField(15);

  @$pb.TagNumber(16)
  $1.Timestamp get createTime => $_getN(11);
  @$pb.TagNumber(16)
  set createTime($1.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasCreateTime() => $_has(11);
  @$pb.TagNumber(16)
  void clearCreateTime() => $_clearField(16);
  @$pb.TagNumber(16)
  $1.Timestamp ensureCreateTime() => $_ensure(11);

  /// 店家資料最後一次被編輯的時間。
  @$pb.TagNumber(17)
  $1.Timestamp get updateTime => $_getN(12);
  @$pb.TagNumber(17)
  set updateTime($1.Timestamp value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasUpdateTime() => $_has(12);
  @$pb.TagNumber(17)
  void clearUpdateTime() => $_clearField(17);
  @$pb.TagNumber(17)
  $1.Timestamp ensureUpdateTime() => $_ensure(12);

  @$pb.TagNumber(18)
  $core.String get etag => $_getSZ(13);
  @$pb.TagNumber(18)
  set etag($core.String value) => $_setString(13, value);
  @$pb.TagNumber(18)
  $core.bool hasEtag() => $_has(13);
  @$pb.TagNumber(18)
  void clearEtag() => $_clearField(18);

  /// 上傳完成後拿到的 staged token；換 logo 時列進 update_mask。
  @$pb.TagNumber(19)
  $core.String get logoStagedToken => $_getSZ(14);
  @$pb.TagNumber(19)
  set logoStagedToken($core.String value) => $_setString(14, value);
  @$pb.TagNumber(19)
  $core.bool hasLogoStagedToken() => $_has(14);
  @$pb.TagNumber(19)
  void clearLogoStagedToken() => $_clearField(19);

  /// 上傳完成後拿到的 staged token；換封面時列進 update_mask。
  @$pb.TagNumber(20)
  $core.String get coverStagedToken => $_getSZ(15);
  @$pb.TagNumber(20)
  set coverStagedToken($core.String value) => $_setString(15, value);
  @$pb.TagNumber(20)
  $core.bool hasCoverStagedToken() => $_has(15);
  @$pb.TagNumber(20)
  void clearCoverStagedToken() => $_clearField(20);

  /// 店家地址；不設表示沒有填。可以只填鄉鎮市區（address_detail 是空字串），有明細就一定有鄉鎮市區代碼。
  /// update_mask 列 address 時整個換成請求的地址，請求沒有 address 就是清掉。
  @$pb.TagNumber(21)
  $2.PostalAddress get address => $_getN(16);
  @$pb.TagNumber(21)
  set address($2.PostalAddress value) => $_setField(21, value);
  @$pb.TagNumber(21)
  $core.bool hasAddress() => $_has(16);
  @$pb.TagNumber(21)
  void clearAddress() => $_clearField(21);
  @$pb.TagNumber(21)
  $2.PostalAddress ensureAddress() => $_ensure(16);

  /// 聯絡人姓名，最多 20 字（PayUni 的 ContactName）；空字串表示沒有填。送出 KYC 之前要填；店家有 PayUni 註冊之後不能清掉。
  @$pb.TagNumber(23)
  $core.String get contactName => $_getSZ(17);
  @$pb.TagNumber(23)
  set contactName($core.String value) => $_setString(17, value);
  @$pb.TagNumber(23)
  $core.bool hasContactName() => $_has(17);
  @$pb.TagNumber(23)
  void clearContactName() => $_clearField(23);

  /// 聯絡人手機，09 開頭的 10 碼（PayUni 的 ContactMobile）；空字串表示沒有填。送出 KYC 之前要填；店家有 PayUni 註冊之後不能清掉。
  @$pb.TagNumber(24)
  $core.String get contactMobile => $_getSZ(18);
  @$pb.TagNumber(24)
  set contactMobile($core.String value) => $_setString(18, value);
  @$pb.TagNumber(24)
  $core.bool hasContactMobile() => $_has(18);
  @$pb.TagNumber(24)
  void clearContactMobile() => $_clearField(24);

  /// 客服電子信箱，最多 50 字（PayUni 的 MerEmail；綁定時同時是會員的 Email，綁定之後 PayUni 不能改它）；空字串表示沒有填。
  /// 送出 KYC 之前要填；店家有 PayUni 註冊之後不能清掉。
  @$pb.TagNumber(25)
  $core.String get supportEmail => $_getSZ(19);
  @$pb.TagNumber(25)
  set supportEmail($core.String value) => $_setString(19, value);
  @$pb.TagNumber(25)
  $core.bool hasSupportEmail() => $_has(19);
  @$pb.TagNumber(25)
  void clearSupportEmail() => $_clearField(25);
}

/// CapabilityStatus 是一個能力此刻的狀態：啟用，或沒有啟用與還缺的條件。
class CapabilityStatus extends $pb.GeneratedMessage {
  factory CapabilityStatus({
    SellerCapability? capability,
    $core.bool? active,
    $core.Iterable<SellerRequirement>? missing,
  }) {
    final result = CapabilityStatus._();
    if (capability != null) result.capability = capability;
    if (active != null) result.active = active;
    if (missing != null) result.missing.addAll(missing);
    return result;
  }

  CapabilityStatus._();

  factory CapabilityStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CapabilityStatus()..mergeFromBuffer(data, registry);
  factory CapabilityStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CapabilityStatus()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CapabilityStatus',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CapabilityStatus.$_createMessage)
    ..aE<SellerCapability>(1, _omitFieldNames ? '' : 'capability',
        enumValues: SellerCapability.values)
    ..aOB(2, _omitFieldNames ? '' : 'active')
    ..pc<SellerRequirement>(
        3, _omitFieldNames ? '' : 'missing', $pb.PbFieldType.KE,
        valueOf: SellerRequirement.valueOf,
        enumValues: SellerRequirement.values,
        defaultEnumValue: SellerRequirement.SELLER_REQUIREMENT_UNSPECIFIED)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CapabilityStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CapabilityStatus copyWith(void Function(CapabilityStatus) updates) =>
      super.copyWith((message) => updates(message as CapabilityStatus))
          as CapabilityStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CapabilityStatus() / CapabilityStatus.new instead')
  static CapabilityStatus create() => CapabilityStatus._();
  static $pb.GeneratedMessage $_createMessage() => CapabilityStatus._();
  @$core.override
  CapabilityStatus createEmptyInstance() => CapabilityStatus._();
  @$core.pragma('dart2js:noInline')
  static CapabilityStatus getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CapabilityStatus>(
          CapabilityStatus.$_createMessage);
  static CapabilityStatus? _defaultInstance;

  @$pb.TagNumber(1)
  SellerCapability get capability => $_getN(0);
  @$pb.TagNumber(1)
  set capability(SellerCapability value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCapability() => $_has(0);
  @$pb.TagNumber(1)
  void clearCapability() => $_clearField(1);

  /// 能力是否啟用；啟用時 missing 是空的。
  @$pb.TagNumber(2)
  $core.bool get active => $_getBF(1);
  @$pb.TagNumber(2)
  set active($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasActive() => $_has(1);
  @$pb.TagNumber(2)
  void clearActive() => $_clearField(2);

  /// 還沒滿足的條件，依 SellerRequirement 的數字順序；沒有啟用的能力至少有一項。
  @$pb.TagNumber(3)
  $pb.PbList<SellerRequirement> get missing => $_getList(2);
}

class ApplyStoreRequest extends $pb.GeneratedMessage {
  factory ApplyStoreRequest({
    $core.String? storeName,
    $core.String? description,
    $core.String? contactPhone,
    $core.String? prize,
    $2.PostalAddress? address,
    $core.Iterable<AgreementAcceptance>? acceptedAgreements,
  }) {
    final result = ApplyStoreRequest._();
    if (storeName != null) result.storeName = storeName;
    if (description != null) result.description = description;
    if (contactPhone != null) result.contactPhone = contactPhone;
    if (prize != null) result.prize = prize;
    if (address != null) result.address = address;
    if (acceptedAgreements != null)
      result.acceptedAgreements.addAll(acceptedAgreements);
    return result;
  }

  ApplyStoreRequest._();

  factory ApplyStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApplyStoreRequest()..mergeFromBuffer(data, registry);
  factory ApplyStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApplyStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ApplyStoreRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeName')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'contactPhone')
    ..aOS(6, _omitFieldNames ? '' : 'prize')
    ..aOM<$2.PostalAddress>(7, _omitFieldNames ? '' : 'address',
        subBuilder: $2.PostalAddress.$_createMessage)
    ..pPM<AgreementAcceptance>(8, _omitFieldNames ? '' : 'acceptedAgreements',
        subBuilder: AgreementAcceptance.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyStoreRequest copyWith(void Function(ApplyStoreRequest) updates) =>
      super.copyWith((message) => updates(message as ApplyStoreRequest))
          as ApplyStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApplyStoreRequest() / ApplyStoreRequest.new instead')
  static ApplyStoreRequest create() => ApplyStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => ApplyStoreRequest._();
  @$core.override
  ApplyStoreRequest createEmptyInstance() => ApplyStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static ApplyStoreRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ApplyStoreRequest>(
          ApplyStoreRequest.$_createMessage);
  static ApplyStoreRequest? _defaultInstance;

  /// 店名，1–100 字。
  @$pb.TagNumber(1)
  $core.String get storeName => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreName() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreName() => $_clearField(1);

  /// 店家介紹，最多 2000 字。
  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  /// 聯絡電話，台灣的手機或市話。
  @$pb.TagNumber(3)
  $core.String get contactPhone => $_getSZ(2);
  @$pb.TagNumber(3)
  set contactPhone($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasContactPhone() => $_has(2);
  @$pb.TagNumber(3)
  void clearContactPhone() => $_clearField(3);

  /// 得獎紀錄，最多 255 字。
  @$pb.TagNumber(6)
  $core.String get prize => $_getSZ(3);
  @$pb.TagNumber(6)
  set prize($core.String value) => $_setString(3, value);
  @$pb.TagNumber(6)
  $core.bool hasPrize() => $_has(3);
  @$pb.TagNumber(6)
  void clearPrize() => $_clearField(6);

  /// 店家地址；不設表示沒有填。可以只填鄉鎮市區（address_detail 是空字串）。
  @$pb.TagNumber(7)
  $2.PostalAddress get address => $_getN(4);
  @$pb.TagNumber(7)
  set address($2.PostalAddress value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasAddress() => $_has(4);
  @$pb.TagNumber(7)
  void clearAddress() => $_clearField(7);
  @$pb.TagNumber(7)
  $2.PostalAddress ensureAddress() => $_ensure(4);

  /// 賣家同意的文件，剛好是 SELLER_TERMS 一份。缺種類、多出不屬於這個操作的種類、同一種重複或 UNSPECIFIED 回
  /// invalid_argument（field violation 指向 accepted_agreements）；版本不是現行版本回 failed_precondition，
  /// ErrorInfo.reason 是 AGREEMENT_OUTDATED，client 重讀 GetSellerAgreements，請賣家重新勾選。同意與開店同一個
  /// transaction，連同時間與來源 IP、User-Agent 記錄。
  @$pb.TagNumber(8)
  $pb.PbList<AgreementAcceptance> get acceptedAgreements => $_getList(5);
}

class ApplyStoreResponse extends $pb.GeneratedMessage {
  factory ApplyStoreResponse({
    Store? store,
  }) {
    final result = ApplyStoreResponse._();
    if (store != null) result.store = store;
    return result;
  }

  ApplyStoreResponse._();

  factory ApplyStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApplyStoreResponse()..mergeFromBuffer(data, registry);
  factory ApplyStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApplyStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ApplyStoreResponse.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyStoreResponse copyWith(void Function(ApplyStoreResponse) updates) =>
      super.copyWith((message) => updates(message as ApplyStoreResponse))
          as ApplyStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApplyStoreResponse() / ApplyStoreResponse.new instead')
  static ApplyStoreResponse create() => ApplyStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => ApplyStoreResponse._();
  @$core.override
  ApplyStoreResponse createEmptyInstance() => ApplyStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static ApplyStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyStoreResponse>(
          ApplyStoreResponse.$_createMessage);
  static ApplyStoreResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);
}

class GetMyStoreOpeningRequest extends $pb.GeneratedMessage {
  factory GetMyStoreOpeningRequest() => GetMyStoreOpeningRequest._();

  GetMyStoreOpeningRequest._();

  factory GetMyStoreOpeningRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreOpeningRequest()..mergeFromBuffer(data, registry);
  factory GetMyStoreOpeningRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreOpeningRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyStoreOpeningRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyStoreOpeningRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreOpeningRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreOpeningRequest copyWith(
          void Function(GetMyStoreOpeningRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyStoreOpeningRequest))
          as GetMyStoreOpeningRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyStoreOpeningRequest() / GetMyStoreOpeningRequest.new instead')
  static GetMyStoreOpeningRequest create() => GetMyStoreOpeningRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyStoreOpeningRequest._();
  @$core.override
  GetMyStoreOpeningRequest createEmptyInstance() =>
      GetMyStoreOpeningRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyStoreOpeningRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyStoreOpeningRequest>(
          GetMyStoreOpeningRequest.$_createMessage);
  static GetMyStoreOpeningRequest? _defaultInstance;
}

class GetMyStoreOpeningResponse extends $pb.GeneratedMessage {
  factory GetMyStoreOpeningResponse({
    $core.bool? canApplyStore,
    $core.Iterable<StoreOpeningUnavailability>? unavailabilities,
  }) {
    final result = GetMyStoreOpeningResponse._();
    if (canApplyStore != null) result.canApplyStore = canApplyStore;
    if (unavailabilities != null)
      result.unavailabilities.addAll(unavailabilities);
    return result;
  }

  GetMyStoreOpeningResponse._();

  factory GetMyStoreOpeningResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreOpeningResponse()..mergeFromBuffer(data, registry);
  factory GetMyStoreOpeningResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreOpeningResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyStoreOpeningResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyStoreOpeningResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'canApplyStore')
    ..pPM<StoreOpeningUnavailability>(
        3, _omitFieldNames ? '' : 'unavailabilities',
        subBuilder: StoreOpeningUnavailability.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreOpeningResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreOpeningResponse copyWith(
          void Function(GetMyStoreOpeningResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyStoreOpeningResponse))
          as GetMyStoreOpeningResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyStoreOpeningResponse() / GetMyStoreOpeningResponse.new instead')
  static GetMyStoreOpeningResponse create() => GetMyStoreOpeningResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetMyStoreOpeningResponse._();
  @$core.override
  GetMyStoreOpeningResponse createEmptyInstance() =>
      GetMyStoreOpeningResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyStoreOpeningResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyStoreOpeningResponse>(
          GetMyStoreOpeningResponse.$_createMessage);
  static GetMyStoreOpeningResponse? _defaultInstance;

  /// 此刻能不能申請開店；等於 unavailabilities 為空。client 只呈現，不從 GetMyStore 的狀態或措施推導。
  @$pb.TagNumber(1)
  $core.bool get canApplyStore => $_getBF(0);
  @$pb.TagNumber(1)
  set canApplyStore($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCanApplyStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearCanApplyStore() => $_clearField(1);

  /// 不能申請的原因，依優先序（結構性的在前，第一個是主要原因，終止合作先於停權：終止撤銷之前解除停權也開不了店）；can_apply_store 為 true 時為空。
  @$pb.TagNumber(3)
  $pb.PbList<StoreOpeningUnavailability> get unavailabilities => $_getList(1);
}

/// StoreOpeningUnavailability 是帳號此刻不能申請開店的一個原因。與 ApplyStore 的前置檢查是同一個規則，message 與 ApplyStore 被擋下時錯誤的
/// LocalizedMessage 是同一份文字（同一個 Go 函式）。
class StoreOpeningUnavailability extends $pb.GeneratedMessage {
  factory StoreOpeningUnavailability({
    StoreOpeningUnavailableReason? reason,
    $3.LocalizedMessage? message,
  }) {
    final result = StoreOpeningUnavailability._();
    if (reason != null) result.reason = reason;
    if (message != null) result.message = message;
    return result;
  }

  StoreOpeningUnavailability._();

  factory StoreOpeningUnavailability.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreOpeningUnavailability()..mergeFromBuffer(data, registry);
  factory StoreOpeningUnavailability.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreOpeningUnavailability()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoreOpeningUnavailability',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: StoreOpeningUnavailability.$_createMessage)
    ..aE<StoreOpeningUnavailableReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: StoreOpeningUnavailableReason.values)
    ..aOM<$3.LocalizedMessage>(2, _omitFieldNames ? '' : 'message',
        subBuilder: $3.LocalizedMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreOpeningUnavailability clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreOpeningUnavailability copyWith(
          void Function(StoreOpeningUnavailability) updates) =>
      super.copyWith(
              (message) => updates(message as StoreOpeningUnavailability))
          as StoreOpeningUnavailability;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use StoreOpeningUnavailability() / StoreOpeningUnavailability.new instead')
  static StoreOpeningUnavailability create() => StoreOpeningUnavailability._();
  static $pb.GeneratedMessage $_createMessage() =>
      StoreOpeningUnavailability._();
  @$core.override
  StoreOpeningUnavailability createEmptyInstance() =>
      StoreOpeningUnavailability._();
  @$core.pragma('dart2js:noInline')
  static StoreOpeningUnavailability getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoreOpeningUnavailability>(
          StoreOpeningUnavailability.$_createMessage);
  static StoreOpeningUnavailability? _defaultInstance;

  /// 原因，給 client 依原因分支用（例如導向措施與申覆）。伺服器一律設，不會是 UNSPECIFIED。
  @$pb.TagNumber(1)
  StoreOpeningUnavailableReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason(StoreOpeningUnavailableReason value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  /// 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；client 直接顯示，不另建原因對照表，沒有 message 時才顯示通用文字。
  @$pb.TagNumber(2)
  $3.LocalizedMessage get message => $_getN(1);
  @$pb.TagNumber(2)
  set message($3.LocalizedMessage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.LocalizedMessage ensureMessage() => $_ensure(1);
}

class GetMyStoreRequest extends $pb.GeneratedMessage {
  factory GetMyStoreRequest() => GetMyStoreRequest._();

  GetMyStoreRequest._();

  factory GetMyStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreRequest()..mergeFromBuffer(data, registry);
  factory GetMyStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyStoreRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreRequest copyWith(void Function(GetMyStoreRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyStoreRequest))
          as GetMyStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyStoreRequest() / GetMyStoreRequest.new instead')
  static GetMyStoreRequest create() => GetMyStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyStoreRequest._();
  @$core.override
  GetMyStoreRequest createEmptyInstance() => GetMyStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyStoreRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetMyStoreRequest>(
          GetMyStoreRequest.$_createMessage);
  static GetMyStoreRequest? _defaultInstance;
}

class GetMyStoreResponse extends $pb.GeneratedMessage {
  factory GetMyStoreResponse({
    Store? store,
    $core.Iterable<CapabilityStatus>? capabilities,
    StoreStats? stats,
    $core.bool? suspended,
  }) {
    final result = GetMyStoreResponse._();
    if (store != null) result.store = store;
    if (capabilities != null) result.capabilities.addAll(capabilities);
    if (stats != null) result.stats = stats;
    if (suspended != null) result.suspended = suspended;
    return result;
  }

  GetMyStoreResponse._();

  factory GetMyStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreResponse()..mergeFromBuffer(data, registry);
  factory GetMyStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyStoreResponse.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..pPM<CapabilityStatus>(3, _omitFieldNames ? '' : 'capabilities',
        subBuilder: CapabilityStatus.$_createMessage)
    ..aOM<StoreStats>(5, _omitFieldNames ? '' : 'stats',
        subBuilder: StoreStats.$_createMessage)
    ..aOB(6, _omitFieldNames ? '' : 'suspended')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyStoreResponse copyWith(void Function(GetMyStoreResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyStoreResponse))
          as GetMyStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyStoreResponse() / GetMyStoreResponse.new instead')
  static GetMyStoreResponse create() => GetMyStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyStoreResponse._();
  @$core.override
  GetMyStoreResponse createEmptyInstance() => GetMyStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyStoreResponse>(
          GetMyStoreResponse.$_createMessage);
  static GetMyStoreResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);

  /// 每個能力（含 PUBLIC_LISTING，不含 UNSPECIFIED）此刻的狀態，依 SellerCapability 的數字順序，所以沒有出現的能力
  /// 不代表沒有啟用。
  @$pb.TagNumber(3)
  $pb.PbList<CapabilityStatus> get capabilities => $_getList(1);

  /// 店家自己看的統計，與前台店面頁的 StoreStats 同一份組裝。粉絲與評價各自讀，讀不到的欄位不設；預覽自己的店，不論是否公開都讀得到。
  @$pb.TagNumber(5)
  StoreStats get stats => $_getN(2);
  @$pb.TagNumber(5)
  set stats(StoreStats value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStats() => $_has(2);
  @$pb.TagNumber(5)
  void clearStats() => $_clearField(5);
  @$pb.TagNumber(5)
  StoreStats ensureStats() => $_ensure(2);

  /// 帳號此刻是否停權（任何一個店家的生效中措施都算）。與 capabilities 裡的 NOT_SUSPENDED 缺項來自同一次推導；
  /// 停權的內容讀 ListMyMeasures(state=ACTIVE)。
  @$pb.TagNumber(6)
  $core.bool get suspended => $_getBF(3);
  @$pb.TagNumber(6)
  set suspended($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(6)
  $core.bool hasSuspended() => $_has(3);
  @$pb.TagNumber(6)
  void clearSuspended() => $_clearField(6);
}

class UpdateMyStoreRequest extends $pb.GeneratedMessage {
  factory UpdateMyStoreRequest({
    Store? store,
    $4.FieldMask? updateMask,
  }) {
    final result = UpdateMyStoreRequest._();
    if (store != null) result.store = store;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateMyStoreRequest._();

  factory UpdateMyStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateMyStoreRequest()..mergeFromBuffer(data, registry);
  factory UpdateMyStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateMyStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateMyStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: UpdateMyStoreRequest.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..aOM<$4.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $4.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateMyStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateMyStoreRequest copyWith(void Function(UpdateMyStoreRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateMyStoreRequest))
          as UpdateMyStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateMyStoreRequest() / UpdateMyStoreRequest.new instead')
  static UpdateMyStoreRequest create() => UpdateMyStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateMyStoreRequest._();
  @$core.override
  UpdateMyStoreRequest createEmptyInstance() => UpdateMyStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateMyStoreRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateMyStoreRequest>(
          UpdateMyStoreRequest.$_createMessage);
  static UpdateMyStoreRequest? _defaultInstance;

  /// etag 必填。
  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);

  @$pb.TagNumber(2)
  $4.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($4.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $4.FieldMask ensureUpdateMask() => $_ensure(1);
}

class UpdateMyStoreResponse extends $pb.GeneratedMessage {
  factory UpdateMyStoreResponse({
    Store? store,
  }) {
    final result = UpdateMyStoreResponse._();
    if (store != null) result.store = store;
    return result;
  }

  UpdateMyStoreResponse._();

  factory UpdateMyStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateMyStoreResponse()..mergeFromBuffer(data, registry);
  factory UpdateMyStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateMyStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateMyStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: UpdateMyStoreResponse.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateMyStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateMyStoreResponse copyWith(
          void Function(UpdateMyStoreResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateMyStoreResponse))
          as UpdateMyStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateMyStoreResponse() / UpdateMyStoreResponse.new instead')
  static UpdateMyStoreResponse create() => UpdateMyStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateMyStoreResponse._();
  @$core.override
  UpdateMyStoreResponse createEmptyInstance() => UpdateMyStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateMyStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateMyStoreResponse>(
          UpdateMyStoreResponse.$_createMessage);
  static UpdateMyStoreResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);
}

class CompleteMyStoreOnboardingRequest extends $pb.GeneratedMessage {
  factory CompleteMyStoreOnboardingRequest({
    $core.String? etag,
  }) {
    final result = CompleteMyStoreOnboardingRequest._();
    if (etag != null) result.etag = etag;
    return result;
  }

  CompleteMyStoreOnboardingRequest._();

  factory CompleteMyStoreOnboardingRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteMyStoreOnboardingRequest()..mergeFromBuffer(data, registry);
  factory CompleteMyStoreOnboardingRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteMyStoreOnboardingRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteMyStoreOnboardingRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CompleteMyStoreOnboardingRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMyStoreOnboardingRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMyStoreOnboardingRequest copyWith(
          void Function(CompleteMyStoreOnboardingRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CompleteMyStoreOnboardingRequest))
          as CompleteMyStoreOnboardingRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CompleteMyStoreOnboardingRequest() / CompleteMyStoreOnboardingRequest.new instead')
  static CompleteMyStoreOnboardingRequest create() =>
      CompleteMyStoreOnboardingRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CompleteMyStoreOnboardingRequest._();
  @$core.override
  CompleteMyStoreOnboardingRequest createEmptyInstance() =>
      CompleteMyStoreOnboardingRequest._();
  @$core.pragma('dart2js:noInline')
  static CompleteMyStoreOnboardingRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteMyStoreOnboardingRequest>(
          CompleteMyStoreOnboardingRequest.$_createMessage);
  static CompleteMyStoreOnboardingRequest? _defaultInstance;

  /// 讀到的店家的 etag。
  @$pb.TagNumber(1)
  $core.String get etag => $_getSZ(0);
  @$pb.TagNumber(1)
  set etag($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEtag() => $_has(0);
  @$pb.TagNumber(1)
  void clearEtag() => $_clearField(1);
}

class CompleteMyStoreOnboardingResponse extends $pb.GeneratedMessage {
  factory CompleteMyStoreOnboardingResponse({
    Store? store,
  }) {
    final result = CompleteMyStoreOnboardingResponse._();
    if (store != null) result.store = store;
    return result;
  }

  CompleteMyStoreOnboardingResponse._();

  factory CompleteMyStoreOnboardingResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteMyStoreOnboardingResponse()..mergeFromBuffer(data, registry);
  factory CompleteMyStoreOnboardingResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteMyStoreOnboardingResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteMyStoreOnboardingResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CompleteMyStoreOnboardingResponse.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMyStoreOnboardingResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMyStoreOnboardingResponse copyWith(
          void Function(CompleteMyStoreOnboardingResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CompleteMyStoreOnboardingResponse))
          as CompleteMyStoreOnboardingResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CompleteMyStoreOnboardingResponse() / CompleteMyStoreOnboardingResponse.new instead')
  static CompleteMyStoreOnboardingResponse create() =>
      CompleteMyStoreOnboardingResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      CompleteMyStoreOnboardingResponse._();
  @$core.override
  CompleteMyStoreOnboardingResponse createEmptyInstance() =>
      CompleteMyStoreOnboardingResponse._();
  @$core.pragma('dart2js:noInline')
  static CompleteMyStoreOnboardingResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteMyStoreOnboardingResponse>(
          CompleteMyStoreOnboardingResponse.$_createMessage);
  static CompleteMyStoreOnboardingResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);
}

class ListMyFollowersRequest extends $pb.GeneratedMessage {
  factory ListMyFollowersRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMyFollowersRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMyFollowersRequest._();

  factory ListMyFollowersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyFollowersRequest()..mergeFromBuffer(data, registry);
  factory ListMyFollowersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyFollowersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyFollowersRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListMyFollowersRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyFollowersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyFollowersRequest copyWith(
          void Function(ListMyFollowersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyFollowersRequest))
          as ListMyFollowersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyFollowersRequest() / ListMyFollowersRequest.new instead')
  static ListMyFollowersRequest create() => ListMyFollowersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyFollowersRequest._();
  @$core.override
  ListMyFollowersRequest createEmptyInstance() => ListMyFollowersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyFollowersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyFollowersRequest>(
          ListMyFollowersRequest.$_createMessage);
  static ListMyFollowersRequest? _defaultInstance;

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

class ListMyFollowersResponse extends $pb.GeneratedMessage {
  factory ListMyFollowersResponse({
    $core.Iterable<Follower>? followers,
    $core.String? nextPageToken,
  }) {
    final result = ListMyFollowersResponse._();
    if (followers != null) result.followers.addAll(followers);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyFollowersResponse._();

  factory ListMyFollowersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyFollowersResponse()..mergeFromBuffer(data, registry);
  factory ListMyFollowersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyFollowersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyFollowersResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListMyFollowersResponse.$_createMessage)
    ..pPM<Follower>(1, _omitFieldNames ? '' : 'followers',
        subBuilder: Follower.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyFollowersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyFollowersResponse copyWith(
          void Function(ListMyFollowersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyFollowersResponse))
          as ListMyFollowersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyFollowersResponse() / ListMyFollowersResponse.new instead')
  static ListMyFollowersResponse create() => ListMyFollowersResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyFollowersResponse._();
  @$core.override
  ListMyFollowersResponse createEmptyInstance() => ListMyFollowersResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyFollowersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyFollowersResponse>(
          ListMyFollowersResponse.$_createMessage);
  static ListMyFollowersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Follower> get followers => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

/// BuyerCard 是賣家看到的買家卡片。
class BuyerCard extends $pb.GeneratedMessage {
  factory BuyerCard({
    $core.String? id,
    $core.bool? deleted,
    $core.String? displayName,
    $0.Image? avatar,
  }) {
    final result = BuyerCard._();
    if (id != null) result.id = id;
    if (deleted != null) result.deleted = deleted;
    if (displayName != null) result.displayName = displayName;
    if (avatar != null) result.avatar = avatar;
    return result;
  }

  BuyerCard._();

  factory BuyerCard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerCard()..mergeFromBuffer(data, registry);
  factory BuyerCard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerCard()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BuyerCard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: BuyerCard.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOB(2, _omitFieldNames ? '' : 'deleted')
    ..aOS(3, _omitFieldNames ? '' : 'displayName')
    ..aOM<$0.Image>(4, _omitFieldNames ? '' : 'avatar',
        subBuilder: $0.Image.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerCard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerCard copyWith(void Function(BuyerCard) updates) =>
      super.copyWith((message) => updates(message as BuyerCard)) as BuyerCard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BuyerCard() / BuyerCard.new instead')
  static BuyerCard create() => BuyerCard._();
  static $pb.GeneratedMessage $_createMessage() => BuyerCard._();
  @$core.override
  BuyerCard createEmptyInstance() => BuyerCard._();
  @$core.pragma('dart2js:noInline')
  static BuyerCard getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BuyerCard>(BuyerCard.$_createMessage);
  static BuyerCard? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 買家已刪除帳號：只有 id，其他欄位不設，client 顯示「已刪除的帳號」。
  @$pb.TagNumber(2)
  $core.bool get deleted => $_getBF(1);
  @$pb.TagNumber(2)
  set deleted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeleted() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeleted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get displayName => $_getSZ(2);
  @$pb.TagNumber(3)
  set displayName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDisplayName() => $_has(2);
  @$pb.TagNumber(3)
  void clearDisplayName() => $_clearField(3);

  @$pb.TagNumber(4)
  $0.Image get avatar => $_getN(3);
  @$pb.TagNumber(4)
  set avatar($0.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasAvatar() => $_has(3);
  @$pb.TagNumber(4)
  void clearAvatar() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Image ensureAvatar() => $_ensure(3);
}

/// Follower 是追蹤店家的買家。
class Follower extends $pb.GeneratedMessage {
  factory Follower({
    BuyerCard? buyer,
    $1.Timestamp? createTime,
  }) {
    final result = Follower._();
    if (buyer != null) result.buyer = buyer;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  Follower._();

  factory Follower.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Follower()..mergeFromBuffer(data, registry);
  factory Follower.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Follower()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Follower',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: Follower.$_createMessage)
    ..aOM<BuyerCard>(1, _omitFieldNames ? '' : 'buyer',
        subBuilder: BuyerCard.$_createMessage)
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Follower clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Follower copyWith(void Function(Follower) updates) =>
      super.copyWith((message) => updates(message as Follower)) as Follower;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Follower() / Follower.new instead')
  static Follower create() => Follower._();
  static $pb.GeneratedMessage $_createMessage() => Follower._();
  @$core.override
  Follower createEmptyInstance() => Follower._();
  @$core.pragma('dart2js:noInline')
  static Follower getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Follower>(Follower.$_createMessage);
  static Follower? _defaultInstance;

  @$pb.TagNumber(1)
  BuyerCard get buyer => $_getN(0);
  @$pb.TagNumber(1)
  set buyer(BuyerCard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBuyer() => $_has(0);
  @$pb.TagNumber(1)
  void clearBuyer() => $_clearField(1);
  @$pb.TagNumber(1)
  BuyerCard ensureBuyer() => $_ensure(0);

  @$pb.TagNumber(2)
  $1.Timestamp get createTime => $_getN(1);
  @$pb.TagNumber(2)
  set createTime($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureCreateTime() => $_ensure(1);
}

class GetMyDashboardRequest extends $pb.GeneratedMessage {
  factory GetMyDashboardRequest() => GetMyDashboardRequest._();

  GetMyDashboardRequest._();

  factory GetMyDashboardRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyDashboardRequest()..mergeFromBuffer(data, registry);
  factory GetMyDashboardRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyDashboardRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyDashboardRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyDashboardRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyDashboardRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyDashboardRequest copyWith(
          void Function(GetMyDashboardRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyDashboardRequest))
          as GetMyDashboardRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyDashboardRequest() / GetMyDashboardRequest.new instead')
  static GetMyDashboardRequest create() => GetMyDashboardRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyDashboardRequest._();
  @$core.override
  GetMyDashboardRequest createEmptyInstance() => GetMyDashboardRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyDashboardRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyDashboardRequest>(
          GetMyDashboardRequest.$_createMessage);
  static GetMyDashboardRequest? _defaultInstance;
}

class GetMyDashboardResponse extends $pb.GeneratedMessage {
  factory GetMyDashboardResponse({
    Dashboard? dashboard,
  }) {
    final result = GetMyDashboardResponse._();
    if (dashboard != null) result.dashboard = dashboard;
    return result;
  }

  GetMyDashboardResponse._();

  factory GetMyDashboardResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyDashboardResponse()..mergeFromBuffer(data, registry);
  factory GetMyDashboardResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyDashboardResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyDashboardResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyDashboardResponse.$_createMessage)
    ..aOM<Dashboard>(1, _omitFieldNames ? '' : 'dashboard',
        subBuilder: Dashboard.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyDashboardResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyDashboardResponse copyWith(
          void Function(GetMyDashboardResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyDashboardResponse))
          as GetMyDashboardResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyDashboardResponse() / GetMyDashboardResponse.new instead')
  static GetMyDashboardResponse create() => GetMyDashboardResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyDashboardResponse._();
  @$core.override
  GetMyDashboardResponse createEmptyInstance() => GetMyDashboardResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyDashboardResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyDashboardResponse>(
          GetMyDashboardResponse.$_createMessage);
  static GetMyDashboardResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Dashboard get dashboard => $_getN(0);
  @$pb.TagNumber(1)
  set dashboard(Dashboard value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasDashboard() => $_has(0);
  @$pb.TagNumber(1)
  void clearDashboard() => $_clearField(1);
  @$pb.TagNumber(1)
  Dashboard ensureDashboard() => $_ensure(0);
}

/// Dashboard 是賣家首頁儀表板。兩個期間由伺服器固定（不接受參數）：current_period 是最近 7 個完整的台北日曆日、不含
/// 今天，previous_period 是它前面緊鄰的 7 日。四個區塊各自來自不同的模組，讀不到時（失敗、逾時、數值超出範圍）該
/// 區塊不設，不是 0。
class Dashboard extends $pb.GeneratedMessage {
  factory Dashboard({
    DashboardPeriod? currentPeriod,
    DashboardPeriod? previousPeriod,
    DashboardSales? sales,
    DashboardFollowers? followers,
    PeriodCount? productViews,
    OrderStatusCounts? orderStatusCounts,
  }) {
    final result = Dashboard._();
    if (currentPeriod != null) result.currentPeriod = currentPeriod;
    if (previousPeriod != null) result.previousPeriod = previousPeriod;
    if (sales != null) result.sales = sales;
    if (followers != null) result.followers = followers;
    if (productViews != null) result.productViews = productViews;
    if (orderStatusCounts != null) result.orderStatusCounts = orderStatusCounts;
    return result;
  }

  Dashboard._();

  factory Dashboard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Dashboard()..mergeFromBuffer(data, registry);
  factory Dashboard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Dashboard()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Dashboard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: Dashboard.$_createMessage)
    ..aOM<DashboardPeriod>(1, _omitFieldNames ? '' : 'currentPeriod',
        subBuilder: DashboardPeriod.$_createMessage)
    ..aOM<DashboardPeriod>(2, _omitFieldNames ? '' : 'previousPeriod',
        subBuilder: DashboardPeriod.$_createMessage)
    ..aOM<DashboardSales>(3, _omitFieldNames ? '' : 'sales',
        subBuilder: DashboardSales.$_createMessage)
    ..aOM<DashboardFollowers>(4, _omitFieldNames ? '' : 'followers',
        subBuilder: DashboardFollowers.$_createMessage)
    ..aOM<PeriodCount>(5, _omitFieldNames ? '' : 'productViews',
        subBuilder: PeriodCount.$_createMessage)
    ..aOM<OrderStatusCounts>(6, _omitFieldNames ? '' : 'orderStatusCounts',
        subBuilder: OrderStatusCounts.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Dashboard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Dashboard copyWith(void Function(Dashboard) updates) =>
      super.copyWith((message) => updates(message as Dashboard)) as Dashboard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Dashboard() / Dashboard.new instead')
  static Dashboard create() => Dashboard._();
  static $pb.GeneratedMessage $_createMessage() => Dashboard._();
  @$core.override
  Dashboard createEmptyInstance() => Dashboard._();
  @$core.pragma('dart2js:noInline')
  static Dashboard getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Dashboard>(Dashboard.$_createMessage);
  static Dashboard? _defaultInstance;

  @$pb.TagNumber(1)
  DashboardPeriod get currentPeriod => $_getN(0);
  @$pb.TagNumber(1)
  set currentPeriod(DashboardPeriod value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCurrentPeriod() => $_has(0);
  @$pb.TagNumber(1)
  void clearCurrentPeriod() => $_clearField(1);
  @$pb.TagNumber(1)
  DashboardPeriod ensureCurrentPeriod() => $_ensure(0);

  @$pb.TagNumber(2)
  DashboardPeriod get previousPeriod => $_getN(1);
  @$pb.TagNumber(2)
  set previousPeriod(DashboardPeriod value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPreviousPeriod() => $_has(1);
  @$pb.TagNumber(2)
  void clearPreviousPeriod() => $_clearField(2);
  @$pb.TagNumber(2)
  DashboardPeriod ensurePreviousPeriod() => $_ensure(1);

  /// 銷售；讀不到時不設。
  @$pb.TagNumber(3)
  DashboardSales get sales => $_getN(2);
  @$pb.TagNumber(3)
  set sales(DashboardSales value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSales() => $_has(2);
  @$pb.TagNumber(3)
  void clearSales() => $_clearField(3);
  @$pb.TagNumber(3)
  DashboardSales ensureSales() => $_ensure(2);

  /// 粉絲；讀不到時不設。
  @$pb.TagNumber(4)
  DashboardFollowers get followers => $_getN(3);
  @$pb.TagNumber(4)
  set followers(DashboardFollowers value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasFollowers() => $_has(3);
  @$pb.TagNumber(4)
  void clearFollowers() => $_clearField(4);
  @$pb.TagNumber(4)
  DashboardFollowers ensureFollowers() => $_ensure(3);

  /// 商品瀏覽數：店家全部商品在期間內的總次數、不去重（匿名訪客與登入買家合計）。來自每日彙總，彙總每 5 分鐘更新一次，
  /// 所以最近一次的瀏覽可能還沒計入，期間的最後一天也一樣；讀不到時不設。
  @$pb.TagNumber(5)
  PeriodCount get productViews => $_getN(4);
  @$pb.TagNumber(5)
  set productViews(PeriodCount value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasProductViews() => $_has(4);
  @$pb.TagNumber(5)
  void clearProductViews() => $_clearField(5);
  @$pb.TagNumber(5)
  PeriodCount ensureProductViews() => $_ensure(4);

  /// 訂單狀態的目前快照，不屬於任何期間；讀不到時不設。
  @$pb.TagNumber(6)
  OrderStatusCounts get orderStatusCounts => $_getN(5);
  @$pb.TagNumber(6)
  set orderStatusCounts(OrderStatusCounts value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasOrderStatusCounts() => $_has(5);
  @$pb.TagNumber(6)
  void clearOrderStatusCounts() => $_clearField(6);
  @$pb.TagNumber(6)
  OrderStatusCounts ensureOrderStatusCounts() => $_ensure(5);
}

/// DashboardPeriod 是一個期間，以台北日曆日表示，含頭含尾（共 7 日）。
class DashboardPeriod extends $pb.GeneratedMessage {
  factory DashboardPeriod({
    $5.Date? firstDate,
    $5.Date? lastDate,
  }) {
    final result = DashboardPeriod._();
    if (firstDate != null) result.firstDate = firstDate;
    if (lastDate != null) result.lastDate = lastDate;
    return result;
  }

  DashboardPeriod._();

  factory DashboardPeriod.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardPeriod()..mergeFromBuffer(data, registry);
  factory DashboardPeriod.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardPeriod()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DashboardPeriod',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: DashboardPeriod.$_createMessage)
    ..aOM<$5.Date>(1, _omitFieldNames ? '' : 'firstDate',
        subBuilder: $5.Date.$_createMessage)
    ..aOM<$5.Date>(2, _omitFieldNames ? '' : 'lastDate',
        subBuilder: $5.Date.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardPeriod clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardPeriod copyWith(void Function(DashboardPeriod) updates) =>
      super.copyWith((message) => updates(message as DashboardPeriod))
          as DashboardPeriod;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DashboardPeriod() / DashboardPeriod.new instead')
  static DashboardPeriod create() => DashboardPeriod._();
  static $pb.GeneratedMessage $_createMessage() => DashboardPeriod._();
  @$core.override
  DashboardPeriod createEmptyInstance() => DashboardPeriod._();
  @$core.pragma('dart2js:noInline')
  static DashboardPeriod getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DashboardPeriod>(
          DashboardPeriod.$_createMessage);
  static DashboardPeriod? _defaultInstance;

  @$pb.TagNumber(1)
  $5.Date get firstDate => $_getN(0);
  @$pb.TagNumber(1)
  set firstDate($5.Date value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFirstDate() => $_has(0);
  @$pb.TagNumber(1)
  void clearFirstDate() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.Date ensureFirstDate() => $_ensure(0);

  @$pb.TagNumber(2)
  $5.Date get lastDate => $_getN(1);
  @$pb.TagNumber(2)
  set lastDate($5.Date value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLastDate() => $_has(1);
  @$pb.TagNumber(2)
  void clearLastDate() => $_clearField(2);
  @$pb.TagNumber(2)
  $5.Date ensureLastDate() => $_ensure(1);
}

/// PeriodAmount 是兩個期間的金額，新台幣元。
class PeriodAmount extends $pb.GeneratedMessage {
  factory PeriodAmount({
    $fixnum.Int64? current,
    $fixnum.Int64? previous,
  }) {
    final result = PeriodAmount._();
    if (current != null) result.current = current;
    if (previous != null) result.previous = previous;
    return result;
  }

  PeriodAmount._();

  factory PeriodAmount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PeriodAmount()..mergeFromBuffer(data, registry);
  factory PeriodAmount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PeriodAmount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PeriodAmount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: PeriodAmount.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'current')
    ..aInt64(2, _omitFieldNames ? '' : 'previous')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PeriodAmount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PeriodAmount copyWith(void Function(PeriodAmount) updates) =>
      super.copyWith((message) => updates(message as PeriodAmount))
          as PeriodAmount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PeriodAmount() / PeriodAmount.new instead')
  static PeriodAmount create() => PeriodAmount._();
  static $pb.GeneratedMessage $_createMessage() => PeriodAmount._();
  @$core.override
  PeriodAmount createEmptyInstance() => PeriodAmount._();
  @$core.pragma('dart2js:noInline')
  static PeriodAmount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PeriodAmount>(
          PeriodAmount.$_createMessage);
  static PeriodAmount? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get current => $_getI64(0);
  @$pb.TagNumber(1)
  set current($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCurrent() => $_has(0);
  @$pb.TagNumber(1)
  void clearCurrent() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get previous => $_getI64(1);
  @$pb.TagNumber(2)
  set previous($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPrevious() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrevious() => $_clearField(2);
}

/// PeriodCount 是兩個期間的次數。
class PeriodCount extends $pb.GeneratedMessage {
  factory PeriodCount({
    $core.int? current,
    $core.int? previous,
  }) {
    final result = PeriodCount._();
    if (current != null) result.current = current;
    if (previous != null) result.previous = previous;
    return result;
  }

  PeriodCount._();

  factory PeriodCount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PeriodCount()..mergeFromBuffer(data, registry);
  factory PeriodCount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PeriodCount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PeriodCount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: PeriodCount.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'current')
    ..aI(2, _omitFieldNames ? '' : 'previous')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PeriodCount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PeriodCount copyWith(void Function(PeriodCount) updates) =>
      super.copyWith((message) => updates(message as PeriodCount))
          as PeriodCount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PeriodCount() / PeriodCount.new instead')
  static PeriodCount create() => PeriodCount._();
  static $pb.GeneratedMessage $_createMessage() => PeriodCount._();
  @$core.override
  PeriodCount createEmptyInstance() => PeriodCount._();
  @$core.pragma('dart2js:noInline')
  static PeriodCount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PeriodCount>(
          PeriodCount.$_createMessage);
  static PeriodCount? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get current => $_getIZ(0);
  @$pb.TagNumber(1)
  set current($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCurrent() => $_has(0);
  @$pb.TagNumber(1)
  void clearCurrent() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get previous => $_getIZ(1);
  @$pb.TagNumber(2)
  set previous($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPrevious() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrevious() => $_clearField(2);
}

/// DashboardSales 的三個數字都來自成立訂單：已付款、未取消、未全額退款的訂單，以付款時間落在期間內計（不是下單時間）。
/// 店家自己買自己的訂單照樣計入。退款發生在訂單付款的期間之後時，那個過去的期間會變小，所以同一個期間在不同時間
/// 讀到的值可能不同。
class DashboardSales extends $pb.GeneratedMessage {
  factory DashboardSales({
    PeriodAmount? revenue,
    PeriodCount? orders,
    PeriodCount? customers,
  }) {
    final result = DashboardSales._();
    if (revenue != null) result.revenue = revenue;
    if (orders != null) result.orders = orders;
    if (customers != null) result.customers = customers;
    return result;
  }

  DashboardSales._();

  factory DashboardSales.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardSales()..mergeFromBuffer(data, registry);
  factory DashboardSales.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardSales()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DashboardSales',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: DashboardSales.$_createMessage)
    ..aOM<PeriodAmount>(1, _omitFieldNames ? '' : 'revenue',
        subBuilder: PeriodAmount.$_createMessage)
    ..aOM<PeriodCount>(2, _omitFieldNames ? '' : 'orders',
        subBuilder: PeriodCount.$_createMessage)
    ..aOM<PeriodCount>(3, _omitFieldNames ? '' : 'customers',
        subBuilder: PeriodCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardSales clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardSales copyWith(void Function(DashboardSales) updates) =>
      super.copyWith((message) => updates(message as DashboardSales))
          as DashboardSales;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DashboardSales() / DashboardSales.new instead')
  static DashboardSales create() => DashboardSales._();
  static $pb.GeneratedMessage $_createMessage() => DashboardSales._();
  @$core.override
  DashboardSales createEmptyInstance() => DashboardSales._();
  @$core.pragma('dart2js:noInline')
  static DashboardSales getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DashboardSales>(
          DashboardSales.$_createMessage);
  static DashboardSales? _defaultInstance;

  /// 營收：成立訂單的實付金額，扣掉已完成的退款金額。
  @$pb.TagNumber(1)
  PeriodAmount get revenue => $_getN(0);
  @$pb.TagNumber(1)
  set revenue(PeriodAmount value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRevenue() => $_has(0);
  @$pb.TagNumber(1)
  void clearRevenue() => $_clearField(1);
  @$pb.TagNumber(1)
  PeriodAmount ensureRevenue() => $_ensure(0);

  /// 成立訂單數。
  @$pb.TagNumber(2)
  PeriodCount get orders => $_getN(1);
  @$pb.TagNumber(2)
  set orders(PeriodCount value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOrders() => $_has(1);
  @$pb.TagNumber(2)
  void clearOrders() => $_clearField(2);
  @$pb.TagNumber(2)
  PeriodCount ensureOrders() => $_ensure(1);

  /// 顧客數：期間內有成立訂單的不重複買家數；同一個買家在兩個期間都買過，兩邊都計一次。
  @$pb.TagNumber(3)
  PeriodCount get customers => $_getN(2);
  @$pb.TagNumber(3)
  set customers(PeriodCount value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCustomers() => $_has(2);
  @$pb.TagNumber(3)
  void clearCustomers() => $_clearField(3);
  @$pb.TagNumber(3)
  PeriodCount ensureCustomers() => $_ensure(2);
}

/// DashboardFollowers 是店家的粉絲。
class DashboardFollowers extends $pb.GeneratedMessage {
  factory DashboardFollowers({
    $core.int? total,
    PeriodCount? newFollowers,
  }) {
    final result = DashboardFollowers._();
    if (total != null) result.total = total;
    if (newFollowers != null) result.newFollowers = newFollowers;
    return result;
  }

  DashboardFollowers._();

  factory DashboardFollowers.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardFollowers()..mergeFromBuffer(data, registry);
  factory DashboardFollowers.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DashboardFollowers()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DashboardFollowers',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: DashboardFollowers.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'total')
    ..aOM<PeriodCount>(2, _omitFieldNames ? '' : 'newFollowers',
        subBuilder: PeriodCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardFollowers clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DashboardFollowers copyWith(void Function(DashboardFollowers) updates) =>
      super.copyWith((message) => updates(message as DashboardFollowers))
          as DashboardFollowers;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DashboardFollowers() / DashboardFollowers.new instead')
  static DashboardFollowers create() => DashboardFollowers._();
  static $pb.GeneratedMessage $_createMessage() => DashboardFollowers._();
  @$core.override
  DashboardFollowers createEmptyInstance() => DashboardFollowers._();
  @$core.pragma('dart2js:noInline')
  static DashboardFollowers getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DashboardFollowers>(
          DashboardFollowers.$_createMessage);
  static DashboardFollowers? _defaultInstance;

  /// 目前的粉絲總數，不屬於任何期間。
  @$pb.TagNumber(1)
  $core.int get total => $_getIZ(0);
  @$pb.TagNumber(1)
  set total($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotal() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotal() => $_clearField(1);

  /// 期間內新增的追蹤次數（以追蹤時間計）。
  @$pb.TagNumber(2)
  PeriodCount get newFollowers => $_getN(1);
  @$pb.TagNumber(2)
  set newFollowers(PeriodCount value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasNewFollowers() => $_has(1);
  @$pb.TagNumber(2)
  void clearNewFollowers() => $_clearField(2);
  @$pb.TagNumber(2)
  PeriodCount ensureNewFollowers() => $_ensure(1);
}

/// OrderStatusCounts 是店家訂單狀態的目前快照，共同的規則：店家自己買的訂單不計，待付款的訂單不計；有售後處理中的訂單
/// 不在前三項，只算在 aftersales_count。前三項互斥。
class OrderStatusCounts extends $pb.GeneratedMessage {
  factory OrderStatusCounts({
    $core.int? pendingShipmentCount,
    $core.int? inProgressCount,
    $core.int? closedCount,
    $core.int? aftersalesCount,
  }) {
    final result = OrderStatusCounts._();
    if (pendingShipmentCount != null)
      result.pendingShipmentCount = pendingShipmentCount;
    if (inProgressCount != null) result.inProgressCount = inProgressCount;
    if (closedCount != null) result.closedCount = closedCount;
    if (aftersalesCount != null) result.aftersalesCount = aftersalesCount;
    return result;
  }

  OrderStatusCounts._();

  factory OrderStatusCounts.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OrderStatusCounts()..mergeFromBuffer(data, registry);
  factory OrderStatusCounts.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OrderStatusCounts()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OrderStatusCounts',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: OrderStatusCounts.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pendingShipmentCount')
    ..aI(2, _omitFieldNames ? '' : 'inProgressCount')
    ..aI(3, _omitFieldNames ? '' : 'closedCount')
    ..aI(4, _omitFieldNames ? '' : 'aftersalesCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OrderStatusCounts clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OrderStatusCounts copyWith(void Function(OrderStatusCounts) updates) =>
      super.copyWith((message) => updates(message as OrderStatusCounts))
          as OrderStatusCounts;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OrderStatusCounts() / OrderStatusCounts.new instead')
  static OrderStatusCounts create() => OrderStatusCounts._();
  static $pb.GeneratedMessage $_createMessage() => OrderStatusCounts._();
  @$core.override
  OrderStatusCounts createEmptyInstance() => OrderStatusCounts._();
  @$core.pragma('dart2js:noInline')
  static OrderStatusCounts getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OrderStatusCounts>(
          OrderStatusCounts.$_createMessage);
  static OrderStatusCounts? _defaultInstance;

  /// 待出貨：已付款與待出貨的訂單。
  @$pb.TagNumber(1)
  $core.int get pendingShipmentCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set pendingShipmentCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPendingShipmentCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearPendingShipmentCount() => $_clearField(1);

  /// 進行中：配送中、待取貨、物流異常與已送達（買家還沒確認收貨）的訂單。
  @$pb.TagNumber(2)
  $core.int get inProgressCount => $_getIZ(1);
  @$pb.TagNumber(2)
  set inProgressCount($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInProgressCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearInProgressCount() => $_clearField(2);

  /// 已結束：賣家不需要再處理的訂單，包含已完成、部分退款、全額退款與已取消，以及售後已經結案、訂單狀態卻還沒進入終態的
  /// 訂單。因為含已取消與全額退款，所以不是成交數。
  @$pb.TagNumber(3)
  $core.int get closedCount => $_getIZ(2);
  @$pb.TagNumber(3)
  set closedCount($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClosedCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearClosedCount() => $_clearField(3);

  /// 售後處理中：進行中的售後案件數。
  @$pb.TagNumber(4)
  $core.int get aftersalesCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set aftersalesCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAftersalesCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearAftersalesCount() => $_clearField(4);
}

class CloseMyStoreRequest extends $pb.GeneratedMessage {
  factory CloseMyStoreRequest() => CloseMyStoreRequest._();

  CloseMyStoreRequest._();

  factory CloseMyStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseMyStoreRequest()..mergeFromBuffer(data, registry);
  factory CloseMyStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseMyStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CloseMyStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CloseMyStoreRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseMyStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseMyStoreRequest copyWith(void Function(CloseMyStoreRequest) updates) =>
      super.copyWith((message) => updates(message as CloseMyStoreRequest))
          as CloseMyStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CloseMyStoreRequest() / CloseMyStoreRequest.new instead')
  static CloseMyStoreRequest create() => CloseMyStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => CloseMyStoreRequest._();
  @$core.override
  CloseMyStoreRequest createEmptyInstance() => CloseMyStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static CloseMyStoreRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CloseMyStoreRequest>(
          CloseMyStoreRequest.$_createMessage);
  static CloseMyStoreRequest? _defaultInstance;
}

class CloseMyStoreResponse extends $pb.GeneratedMessage {
  factory CloseMyStoreResponse({
    Store? store,
  }) {
    final result = CloseMyStoreResponse._();
    if (store != null) result.store = store;
    return result;
  }

  CloseMyStoreResponse._();

  factory CloseMyStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseMyStoreResponse()..mergeFromBuffer(data, registry);
  factory CloseMyStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseMyStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CloseMyStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CloseMyStoreResponse.$_createMessage)
    ..aOM<Store>(1, _omitFieldNames ? '' : 'store',
        subBuilder: Store.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseMyStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseMyStoreResponse copyWith(void Function(CloseMyStoreResponse) updates) =>
      super.copyWith((message) => updates(message as CloseMyStoreResponse))
          as CloseMyStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CloseMyStoreResponse() / CloseMyStoreResponse.new instead')
  static CloseMyStoreResponse create() => CloseMyStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => CloseMyStoreResponse._();
  @$core.override
  CloseMyStoreResponse createEmptyInstance() => CloseMyStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static CloseMyStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CloseMyStoreResponse>(
          CloseMyStoreResponse.$_createMessage);
  static CloseMyStoreResponse? _defaultInstance;

  /// 關閉後的店家：status 是 CLOSED，logo 與封面已清掉。
  @$pb.TagNumber(1)
  Store get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(Store value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  Store ensureStore() => $_ensure(0);
}

/// KYCIssue 是退回時指出的一個問題：哪個欄位（文件類的欄位可以再指到哪一張）、什麼原因、給賣家看的說明。不含欄位的值。
class KYCIssue extends $pb.GeneratedMessage {
  factory KYCIssue({
    KYCField? field_1,
    $core.String? documentAssetId,
    KYCIssueCode? code,
    $core.String? message,
  }) {
    final result = KYCIssue._();
    if (field_1 != null) result.field_1 = field_1;
    if (documentAssetId != null) result.documentAssetId = documentAssetId;
    if (code != null) result.code = code;
    if (message != null) result.message = message;
    return result;
  }

  KYCIssue._();

  factory KYCIssue.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCIssue()..mergeFromBuffer(data, registry);
  factory KYCIssue.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCIssue()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCIssue',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCIssue.$_createMessage)
    ..aE<KYCField>(1, _omitFieldNames ? '' : 'field',
        enumValues: KYCField.values)
    ..aOS(2, _omitFieldNames ? '' : 'documentAssetId')
    ..aE<KYCIssueCode>(3, _omitFieldNames ? '' : 'code',
        enumValues: KYCIssueCode.values)
    ..aOS(4, _omitFieldNames ? '' : 'message')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCIssue clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCIssue copyWith(void Function(KYCIssue) updates) =>
      super.copyWith((message) => updates(message as KYCIssue)) as KYCIssue;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use KYCIssue() / KYCIssue.new instead')
  static KYCIssue create() => KYCIssue._();
  static $pb.GeneratedMessage $_createMessage() => KYCIssue._();
  @$core.override
  KYCIssue createEmptyInstance() => KYCIssue._();
  @$core.pragma('dart2js:noInline')
  static KYCIssue getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<KYCIssue>(KYCIssue.$_createMessage);
  static KYCIssue? _defaultInstance;

  /// 有問題的欄位。
  @$pb.TagNumber(1)
  KYCField get field_1 => $_getN(0);
  @$pb.TagNumber(1)
  set field_1(KYCField value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasField_1() => $_has(0);
  @$pb.TagNumber(1)
  void clearField_1() => $_clearField(1);

  /// field 是文件類（id_card_front、id_card_back、bank_book、business.company_documents）時，指出的那一張文件的 asset_id
  /// （KYCDocument.asset_id），公司文件有多張時用它指到其中一張，順序改變也不會指錯。非文件類的欄位必須是空字串。
  /// 文件類欄位的規則依原因碼：UNREADABLE、EXPIRED、MISMATCH、INVALID 必須指定是哪一張文件（這些問題出在某一張文件上）；
  /// MISSING 不可以指定（缺的是整個位置，沒有那一張）；OTHER 兩者皆可。違反時 RejectKYC 回 invalid_argument。
  @$pb.TagNumber(2)
  $core.String get documentAssetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set documentAssetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDocumentAssetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearDocumentAssetId() => $_clearField(2);

  @$pb.TagNumber(3)
  KYCIssueCode get code => $_getN(2);
  @$pb.TagNumber(3)
  set code(KYCIssueCode value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCode() => $_has(2);
  @$pb.TagNumber(3)
  void clearCode() => $_clearField(3);

  /// 給賣家看的說明，1 到 500 字。
  @$pb.TagNumber(4)
  $core.String get message => $_getSZ(3);
  @$pb.TagNumber(4)
  set message($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMessage() => $_has(3);
  @$pb.TagNumber(4)
  void clearMessage() => $_clearField(4);
}

/// KYCDocument 是申請上的一張證件圖片。證件含個資，下載要經有授權的 download_uri（只有申請的擁有者能下載），不是公開的
/// 圖片網址。
class KYCDocument extends $pb.GeneratedMessage {
  factory KYCDocument({
    $core.String? assetId,
    $core.String? downloadUri,
    $core.String? stagedToken,
  }) {
    final result = KYCDocument._();
    if (assetId != null) result.assetId = assetId;
    if (downloadUri != null) result.downloadUri = downloadUri;
    if (stagedToken != null) result.stagedToken = stagedToken;
    return result;
  }

  KYCDocument._();

  factory KYCDocument.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCDocument()..mergeFromBuffer(data, registry);
  factory KYCDocument.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCDocument()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCDocument',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCDocument.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'assetId')
    ..aOS(2, _omitFieldNames ? '' : 'downloadUri')
    ..aOS(3, _omitFieldNames ? '' : 'stagedToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCDocument clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCDocument copyWith(void Function(KYCDocument) updates) =>
      super.copyWith((message) => updates(message as KYCDocument))
          as KYCDocument;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use KYCDocument() / KYCDocument.new instead')
  static KYCDocument create() => KYCDocument._();
  static $pb.GeneratedMessage $_createMessage() => KYCDocument._();
  @$core.override
  KYCDocument createEmptyInstance() => KYCDocument._();
  @$core.pragma('dart2js:noInline')
  static KYCDocument getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<KYCDocument>(
          KYCDocument.$_createMessage);
  static KYCDocument? _defaultInstance;

  /// 這份申請目前的這張圖片；寫入時指定它表示保留這一張（單張的位置不指定就是沿用，公司文件清單裡的現有圖片要列出來才保留）。
  @$pb.TagNumber(1)
  $core.String get assetId => $_getSZ(0);
  @$pb.TagNumber(1)
  set assetId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAssetId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAssetId() => $_clearField(1);

  /// 下載網址（含 asset_id）；需要登入，只有申請的擁有者能下載。
  @$pb.TagNumber(2)
  $core.String get downloadUri => $_getSZ(1);
  @$pb.TagNumber(2)
  set downloadUri($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDownloadUri() => $_has(1);
  @$pb.TagNumber(2)
  void clearDownloadUri() => $_clearField(2);

  /// 新上傳的圖片的 staged token；與 asset_id 擇一。
  @$pb.TagNumber(3)
  $core.String get stagedToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set stagedToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStagedToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearStagedToken() => $_clearField(3);
}

/// KYCRejection 是最近一次退回的內容。
class KYCRejection extends $pb.GeneratedMessage {
  factory KYCRejection({
    $core.String? reason,
    $1.Timestamp? decideTime,
    $core.Iterable<KYCIssue>? issues,
  }) {
    final result = KYCRejection._();
    if (reason != null) result.reason = reason;
    if (decideTime != null) result.decideTime = decideTime;
    if (issues != null) result.issues.addAll(issues);
    return result;
  }

  KYCRejection._();

  factory KYCRejection.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCRejection()..mergeFromBuffer(data, registry);
  factory KYCRejection.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCRejection()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCRejection',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCRejection.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reason')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'decideTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..pPM<KYCIssue>(4, _omitFieldNames ? '' : 'issues',
        subBuilder: KYCIssue.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCRejection clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCRejection copyWith(void Function(KYCRejection) updates) =>
      super.copyWith((message) => updates(message as KYCRejection))
          as KYCRejection;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use KYCRejection() / KYCRejection.new instead')
  static KYCRejection create() => KYCRejection._();
  static $pb.GeneratedMessage $_createMessage() => KYCRejection._();
  @$core.override
  KYCRejection createEmptyInstance() => KYCRejection._();
  @$core.pragma('dart2js:noInline')
  static KYCRejection getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<KYCRejection>(
          KYCRejection.$_createMessage);
  static KYCRejection? _defaultInstance;

  /// 退回的總理由。
  @$pb.TagNumber(1)
  $core.String get reason => $_getSZ(0);
  @$pb.TagNumber(1)
  set reason($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  @$pb.TagNumber(3)
  $1.Timestamp get decideTime => $_getN(1);
  @$pb.TagNumber(3)
  set decideTime($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasDecideTime() => $_has(1);
  @$pb.TagNumber(3)
  void clearDecideTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureDecideTime() => $_ensure(1);

  /// 退回時逐項指出的問題（不管賣家後來有沒有處理）；賣家還沒處理的是 KYCApplication.outstanding_issues。
  @$pb.TagNumber(4)
  $pb.PbList<KYCIssue> get issues => $_getList(2);
}

/// PayUniProblem 是後台看的 PayUni 問題的事實：code 給員工與客服排查；fields、store_fields 是碼指出的欄位（未依身分鎖過濾）；
/// reason 是 PayUni 的審核意見原文（去掉前後空白，沒有時是空字串）。賣家看的是 KYCFlow.payuni。
class PayUniProblem extends $pb.GeneratedMessage {
  factory PayUniProblem({
    $core.String? code,
    $core.Iterable<KYCField>? fields,
    $core.String? reason,
    $core.Iterable<StoreField>? storeFields,
  }) {
    final result = PayUniProblem._();
    if (code != null) result.code = code;
    if (fields != null) result.fields.addAll(fields);
    if (reason != null) result.reason = reason;
    if (storeFields != null) result.storeFields.addAll(storeFields);
    return result;
  }

  PayUniProblem._();

  factory PayUniProblem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PayUniProblem()..mergeFromBuffer(data, registry);
  factory PayUniProblem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PayUniProblem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PayUniProblem',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: PayUniProblem.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..pc<KYCField>(3, _omitFieldNames ? '' : 'fields', $pb.PbFieldType.KE,
        valueOf: KYCField.valueOf,
        enumValues: KYCField.values,
        defaultEnumValue: KYCField.KYC_FIELD_UNSPECIFIED)
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..pc<StoreField>(
        5, _omitFieldNames ? '' : 'storeFields', $pb.PbFieldType.KE,
        valueOf: StoreField.valueOf,
        enumValues: StoreField.values,
        defaultEnumValue: StoreField.STORE_FIELD_UNSPECIFIED)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PayUniProblem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PayUniProblem copyWith(void Function(PayUniProblem) updates) =>
      super.copyWith((message) => updates(message as PayUniProblem))
          as PayUniProblem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PayUniProblem() / PayUniProblem.new instead')
  static PayUniProblem create() => PayUniProblem._();
  static $pb.GeneratedMessage $_createMessage() => PayUniProblem._();
  @$core.override
  PayUniProblem createEmptyInstance() => PayUniProblem._();
  @$core.pragma('dart2js:noInline')
  static PayUniProblem getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PayUniProblem>(
          PayUniProblem.$_createMessage);
  static PayUniProblem? _defaultInstance;

  /// 錯誤碼：我們自己的碼（INCOMPLETE_DATA、PAYUNI_NOT_CONFIGURED、PAYUNI_UNAVAILABLE、MERCHANT_TERMINAL）或 PayUni
  /// 的碼（AGENT 開頭，MER_ 與 MEMBER_ 開頭的是審核狀態）。開放集合。
  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  /// 碼指出的申請欄位（與 KYCIssue.field 同一套）；問題不在申請欄位上時是空的。
  @$pb.TagNumber(3)
  $pb.PbList<KYCField> get fields => $_getList(1);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  /// 碼指出的店家欄位（UpdateMyStore）；問題不在店家欄位上時是空的。
  @$pb.TagNumber(5)
  $pb.PbList<StoreField> get storeFields => $_getList(3);
}

/// SellerPayUni 是賣家看到的 PayUni 狀態與要處理的事。
class SellerPayUni extends $pb.GeneratedMessage {
  factory SellerPayUni({
    PayUniState? state,
    $3.LocalizedMessage? explanation,
    $core.String? reason,
    $core.Iterable<KYCField>? fixFields,
    $core.Iterable<StoreField>? fixStoreFields,
  }) {
    final result = SellerPayUni._();
    if (state != null) result.state = state;
    if (explanation != null) result.explanation = explanation;
    if (reason != null) result.reason = reason;
    if (fixFields != null) result.fixFields.addAll(fixFields);
    if (fixStoreFields != null) result.fixStoreFields.addAll(fixStoreFields);
    return result;
  }

  SellerPayUni._();

  factory SellerPayUni.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPayUni()..mergeFromBuffer(data, registry);
  factory SellerPayUni.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPayUni()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerPayUni',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SellerPayUni.$_createMessage)
    ..aE<PayUniState>(1, _omitFieldNames ? '' : 'state',
        enumValues: PayUniState.values)
    ..aOM<$3.LocalizedMessage>(2, _omitFieldNames ? '' : 'explanation',
        subBuilder: $3.LocalizedMessage.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..pc<KYCField>(4, _omitFieldNames ? '' : 'fixFields', $pb.PbFieldType.KE,
        valueOf: KYCField.valueOf,
        enumValues: KYCField.values,
        defaultEnumValue: KYCField.KYC_FIELD_UNSPECIFIED)
    ..pc<StoreField>(
        5, _omitFieldNames ? '' : 'fixStoreFields', $pb.PbFieldType.KE,
        valueOf: StoreField.valueOf,
        enumValues: StoreField.values,
        defaultEnumValue: StoreField.STORE_FIELD_UNSPECIFIED)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPayUni clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPayUni copyWith(void Function(SellerPayUni) updates) =>
      super.copyWith((message) => updates(message as SellerPayUni))
          as SellerPayUni;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerPayUni() / SellerPayUni.new instead')
  static SellerPayUni create() => SellerPayUni._();
  static $pb.GeneratedMessage $_createMessage() => SellerPayUni._();
  @$core.override
  SellerPayUni createEmptyInstance() => SellerPayUni._();
  @$core.pragma('dart2js:noInline')
  static SellerPayUni getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerPayUni>(
          SellerPayUni.$_createMessage);
  static SellerPayUni? _defaultInstance;

  @$pb.TagNumber(1)
  PayUniState get state => $_getN(0);
  @$pb.TagNumber(1)
  set state(PayUniState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasState() => $_has(0);
  @$pb.TagNumber(1)
  void clearState() => $_clearField(1);

  /// 給賣家看的狀態說明（locale 是實際用的語系），只由 state 決定；state 是 APPROVED 時不設。
  @$pb.TagNumber(2)
  $3.LocalizedMessage get explanation => $_getN(1);
  @$pb.TagNumber(2)
  set explanation($3.LocalizedMessage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasExplanation() => $_has(1);
  @$pb.TagNumber(2)
  void clearExplanation() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.LocalizedMessage ensureExplanation() => $_ensure(1);

  /// PayUni 針對這家店寫的審核意見原文（去掉前後空白；沒有時是空字串）。第三方原文、不翻譯，client 另外顯示並標 zh-TW。
  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  /// 賣家此刻能在申請上修正的欄位（已依賣家型態與身分鎖過濾）；沒有具體欄位時是空的，client 不顯示清單。
  @$pb.TagNumber(4)
  $pb.PbList<KYCField> get fixFields => $_getList(3);

  /// 賣家此刻能在店家設定（UpdateMyStore）修正的欄位；沒有時是空的。
  @$pb.TagNumber(5)
  $pb.PbList<StoreField> get fixStoreFields => $_getList(4);
}

/// KYCResubmitUnavailability 是 SaveKYCDraft 與 SubmitKYC 此刻不能做的一個原因。與兩個 RPC 的前置檢查是同一個規則，message 與
/// 被擋下時錯誤的訊息是同一份文字（同一個 Go 函式）。
class KYCResubmitUnavailability extends $pb.GeneratedMessage {
  factory KYCResubmitUnavailability({
    KYCResubmitUnavailableReason? reason,
    $3.LocalizedMessage? message,
  }) {
    final result = KYCResubmitUnavailability._();
    if (reason != null) result.reason = reason;
    if (message != null) result.message = message;
    return result;
  }

  KYCResubmitUnavailability._();

  factory KYCResubmitUnavailability.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCResubmitUnavailability()..mergeFromBuffer(data, registry);
  factory KYCResubmitUnavailability.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCResubmitUnavailability()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCResubmitUnavailability',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCResubmitUnavailability.$_createMessage)
    ..aE<KYCResubmitUnavailableReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: KYCResubmitUnavailableReason.values)
    ..aOM<$3.LocalizedMessage>(2, _omitFieldNames ? '' : 'message',
        subBuilder: $3.LocalizedMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCResubmitUnavailability clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCResubmitUnavailability copyWith(
          void Function(KYCResubmitUnavailability) updates) =>
      super.copyWith((message) => updates(message as KYCResubmitUnavailability))
          as KYCResubmitUnavailability;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use KYCResubmitUnavailability() / KYCResubmitUnavailability.new instead')
  static KYCResubmitUnavailability create() => KYCResubmitUnavailability._();
  static $pb.GeneratedMessage $_createMessage() =>
      KYCResubmitUnavailability._();
  @$core.override
  KYCResubmitUnavailability createEmptyInstance() =>
      KYCResubmitUnavailability._();
  @$core.pragma('dart2js:noInline')
  static KYCResubmitUnavailability getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<KYCResubmitUnavailability>(
          KYCResubmitUnavailability.$_createMessage);
  static KYCResubmitUnavailability? _defaultInstance;

  /// 原因。伺服器一律設，不會是 UNSPECIFIED。
  @$pb.TagNumber(1)
  KYCResubmitUnavailableReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason(KYCResubmitUnavailableReason value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  /// 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；client 直接顯示。
  @$pb.TagNumber(2)
  $3.LocalizedMessage get message => $_getN(1);
  @$pb.TagNumber(2)
  set message($3.LocalizedMessage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.LocalizedMessage ensureMessage() => $_ensure(1);
}

/// KYCFlow 是申請的流程狀態與衍生旗標：KYC 與 PayUni 兩條軸合成的結果。
class KYCFlow extends $pb.GeneratedMessage {
  factory KYCFlow({
    KYCFlowStatus? status,
    $core.bool? canResubmit,
    $core.bool? identityLocked,
    $core.Iterable<KYCResubmitUnavailability>? resubmitUnavailabilities,
    SellerPayUni? payuni,
  }) {
    final result = KYCFlow._();
    if (status != null) result.status = status;
    if (canResubmit != null) result.canResubmit = canResubmit;
    if (identityLocked != null) result.identityLocked = identityLocked;
    if (resubmitUnavailabilities != null)
      result.resubmitUnavailabilities.addAll(resubmitUnavailabilities);
    if (payuni != null) result.payuni = payuni;
    return result;
  }

  KYCFlow._();

  factory KYCFlow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCFlow()..mergeFromBuffer(data, registry);
  factory KYCFlow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCFlow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCFlow',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCFlow.$_createMessage)
    ..aE<KYCFlowStatus>(1, _omitFieldNames ? '' : 'status',
        enumValues: KYCFlowStatus.values)
    ..aOB(2, _omitFieldNames ? '' : 'canResubmit')
    ..aOB(3, _omitFieldNames ? '' : 'identityLocked')
    ..pPM<KYCResubmitUnavailability>(
        4, _omitFieldNames ? '' : 'resubmitUnavailabilities',
        subBuilder: KYCResubmitUnavailability.$_createMessage)
    ..aOM<SellerPayUni>(5, _omitFieldNames ? '' : 'payuni',
        subBuilder: SellerPayUni.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCFlow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCFlow copyWith(void Function(KYCFlow) updates) =>
      super.copyWith((message) => updates(message as KYCFlow)) as KYCFlow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use KYCFlow() / KYCFlow.new instead')
  static KYCFlow create() => KYCFlow._();
  static $pb.GeneratedMessage $_createMessage() => KYCFlow._();
  @$core.override
  KYCFlow createEmptyInstance() => KYCFlow._();
  @$core.pragma('dart2js:noInline')
  static KYCFlow getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<KYCFlow>(KYCFlow.$_createMessage);
  static KYCFlow? _defaultInstance;

  @$pb.TagNumber(1)
  KYCFlowStatus get status => $_getN(0);
  @$pb.TagNumber(1)
  set status(KYCFlowStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  /// 此刻 SaveKYCDraft 與 SubmitKYC 會被接受（草稿、被退回、PayUni 拒絕或綁定失敗而且賣家能自己修正）。
  @$pb.TagNumber(2)
  $core.bool get canResubmit => $_getBF(1);
  @$pb.TagNumber(2)
  set canResubmit($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCanResubmit() => $_has(1);
  @$pb.TagNumber(2)
  void clearCanResubmit() => $_clearField(2);

  /// 註冊已有 MerID：身分欄位（店名、介紹、姓名或公司名稱、證號、統編、英文名稱、官網、商品類型、賣家型態）鎖住，只能
  /// 更正聯絡欄位與地址。
  @$pb.TagNumber(3)
  $core.bool get identityLocked => $_getBF(2);
  @$pb.TagNumber(3)
  set identityLocked($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIdentityLocked() => $_has(2);
  @$pb.TagNumber(3)
  void clearIdentityLocked() => $_clearField(3);

  /// can_resubmit 為假時的原因，依先後，至少一個；為真時是空的。SaveKYCDraft、SubmitKYC 不能做時回的
  /// failed_precondition 是第一個的 message。只在賣家的回應設，admin 的 KYCApplication.flow 不設。
  @$pb.TagNumber(4)
  $pb.PbList<KYCResubmitUnavailability> get resubmitUnavailabilities =>
      $_getList(3);

  /// 申請在 PayUni 階段（已核准），或賣家正在修正 PayUni 拒絕、綁定失敗的那份申請（草稿）時才設。只在賣家的回應設，
  /// admin 的 KYCApplication.flow 不設（後台的 PayUni 呈現在 PayUniRegistration）。
  @$pb.TagNumber(5)
  SellerPayUni get payuni => $_getN(4);
  @$pb.TagNumber(5)
  set payuni(SellerPayUni value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPayuni() => $_has(4);
  @$pb.TagNumber(5)
  void clearPayuni() => $_clearField(5);
  @$pb.TagNumber(5)
  SellerPayUni ensurePayuni() => $_ensure(4);
}

/// IndividualKYC 是個人賣家專屬的欄位：身分證號是賣家本人的。
class IndividualKYC extends $pb.GeneratedMessage {
  factory IndividualKYC({
    $core.String? idCardNo,
    $core.String? idCardNoMasked,
  }) {
    final result = IndividualKYC._();
    if (idCardNo != null) result.idCardNo = idCardNo;
    if (idCardNoMasked != null) result.idCardNoMasked = idCardNoMasked;
    return result;
  }

  IndividualKYC._();

  factory IndividualKYC.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IndividualKYC()..mergeFromBuffer(data, registry);
  factory IndividualKYC.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IndividualKYC()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IndividualKYC',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: IndividualKYC.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'idCardNo')
    ..aOS(2, _omitFieldNames ? '' : 'idCardNoMasked')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IndividualKYC clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IndividualKYC copyWith(void Function(IndividualKYC) updates) =>
      super.copyWith((message) => updates(message as IndividualKYC))
          as IndividualKYC;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use IndividualKYC() / IndividualKYC.new instead')
  static IndividualKYC create() => IndividualKYC._();
  static $pb.GeneratedMessage $_createMessage() => IndividualKYC._();
  @$core.override
  IndividualKYC createEmptyInstance() => IndividualKYC._();
  @$core.pragma('dart2js:noInline')
  static IndividualKYC getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<IndividualKYC>(
          IndividualKYC.$_createMessage);
  static IndividualKYC? _defaultInstance;

  /// 身分證字號（寫入用）。
  @$pb.TagNumber(1)
  $core.String get idCardNo => $_getSZ(0);
  @$pb.TagNumber(1)
  set idCardNo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasIdCardNo() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdCardNo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get idCardNoMasked => $_getSZ(1);
  @$pb.TagNumber(2)
  set idCardNoMasked($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIdCardNoMasked() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdCardNoMasked() => $_clearField(2);
}

/// BusinessKYC 是企業賣家專屬的欄位：公司資料與負責人的身分證號。
class BusinessKYC extends $pb.GeneratedMessage {
  factory BusinessKYC({
    $core.String? companyName,
    $core.String? taxId,
    $5.Date? companyRegisterDate,
    $core.String? companyPhone,
    $core.String? repIdCardNo,
    $core.String? repIdCardNoMasked,
    $core.Iterable<KYCDocument>? companyDocuments,
    $2.PostalAddress? companyAddress,
  }) {
    final result = BusinessKYC._();
    if (companyName != null) result.companyName = companyName;
    if (taxId != null) result.taxId = taxId;
    if (companyRegisterDate != null)
      result.companyRegisterDate = companyRegisterDate;
    if (companyPhone != null) result.companyPhone = companyPhone;
    if (repIdCardNo != null) result.repIdCardNo = repIdCardNo;
    if (repIdCardNoMasked != null) result.repIdCardNoMasked = repIdCardNoMasked;
    if (companyDocuments != null)
      result.companyDocuments.addAll(companyDocuments);
    if (companyAddress != null) result.companyAddress = companyAddress;
    return result;
  }

  BusinessKYC._();

  factory BusinessKYC.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BusinessKYC()..mergeFromBuffer(data, registry);
  factory BusinessKYC.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BusinessKYC()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BusinessKYC',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: BusinessKYC.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'companyName')
    ..aOS(3, _omitFieldNames ? '' : 'taxId')
    ..aOM<$5.Date>(4, _omitFieldNames ? '' : 'companyRegisterDate',
        subBuilder: $5.Date.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'companyPhone')
    ..aOS(8, _omitFieldNames ? '' : 'repIdCardNo')
    ..aOS(9, _omitFieldNames ? '' : 'repIdCardNoMasked')
    ..pPM<KYCDocument>(10, _omitFieldNames ? '' : 'companyDocuments',
        subBuilder: KYCDocument.$_createMessage)
    ..aOM<$2.PostalAddress>(11, _omitFieldNames ? '' : 'companyAddress',
        subBuilder: $2.PostalAddress.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BusinessKYC clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BusinessKYC copyWith(void Function(BusinessKYC) updates) =>
      super.copyWith((message) => updates(message as BusinessKYC))
          as BusinessKYC;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BusinessKYC() / BusinessKYC.new instead')
  static BusinessKYC create() => BusinessKYC._();
  static $pb.GeneratedMessage $_createMessage() => BusinessKYC._();
  @$core.override
  BusinessKYC createEmptyInstance() => BusinessKYC._();
  @$core.pragma('dart2js:noInline')
  static BusinessKYC getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BusinessKYC>(
          BusinessKYC.$_createMessage);
  static BusinessKYC? _defaultInstance;

  @$pb.TagNumber(2)
  $core.String get companyName => $_getSZ(0);
  @$pb.TagNumber(2)
  set companyName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(2)
  $core.bool hasCompanyName() => $_has(0);
  @$pb.TagNumber(2)
  void clearCompanyName() => $_clearField(2);

  /// 8 碼統一編號，含檢查碼（2023 年起的規則）。
  @$pb.TagNumber(3)
  $core.String get taxId => $_getSZ(1);
  @$pb.TagNumber(3)
  set taxId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(3)
  $core.bool hasTaxId() => $_has(1);
  @$pb.TagNumber(3)
  void clearTaxId() => $_clearField(3);

  @$pb.TagNumber(4)
  $5.Date get companyRegisterDate => $_getN(2);
  @$pb.TagNumber(4)
  set companyRegisterDate($5.Date value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCompanyRegisterDate() => $_has(2);
  @$pb.TagNumber(4)
  void clearCompanyRegisterDate() => $_clearField(4);
  @$pb.TagNumber(4)
  $5.Date ensureCompanyRegisterDate() => $_ensure(2);

  @$pb.TagNumber(7)
  $core.String get companyPhone => $_getSZ(3);
  @$pb.TagNumber(7)
  set companyPhone($core.String value) => $_setString(3, value);
  @$pb.TagNumber(7)
  $core.bool hasCompanyPhone() => $_has(3);
  @$pb.TagNumber(7)
  void clearCompanyPhone() => $_clearField(7);

  /// 負責人的身分證字號（寫入用）。
  @$pb.TagNumber(8)
  $core.String get repIdCardNo => $_getSZ(4);
  @$pb.TagNumber(8)
  set repIdCardNo($core.String value) => $_setString(4, value);
  @$pb.TagNumber(8)
  $core.bool hasRepIdCardNo() => $_has(4);
  @$pb.TagNumber(8)
  void clearRepIdCardNo() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get repIdCardNoMasked => $_getSZ(5);
  @$pb.TagNumber(9)
  set repIdCardNoMasked($core.String value) => $_setString(5, value);
  @$pb.TagNumber(9)
  $core.bool hasRepIdCardNoMasked() => $_has(5);
  @$pb.TagNumber(9)
  void clearRepIdCardNoMasked() => $_clearField(9);

  /// 公司文件，最多 10 張，順序就是位置；列進 update_mask 時整個換成請求的清單。
  @$pb.TagNumber(10)
  $pb.PbList<KYCDocument> get companyDocuments => $_getList(6);

  /// 公司地址；列進 update_mask 時整個換成請求的地址，請求沒有 address 就是清掉。可以只填鄉鎮市區。
  @$pb.TagNumber(11)
  $2.PostalAddress get companyAddress => $_getN(7);
  @$pb.TagNumber(11)
  set companyAddress($2.PostalAddress value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCompanyAddress() => $_has(7);
  @$pb.TagNumber(11)
  void clearCompanyAddress() => $_clearField(11);
  @$pb.TagNumber(11)
  $2.PostalAddress ensureCompanyAddress() => $_ensure(7);
}

enum KYCApplication_Subject { individual, business, notSet }

/// KYCApplication 是店家的身分認證申請，每個店家同時最多一筆（以呼叫者為範圍，沒有申請 ID 的定址）。賣家型態是 subject 設定
/// 的是哪一支（個人 individual 或企業 business）；個人與企業共用的欄位（姓名、銀行、聯絡方式、證件）在最上層，各自專屬的
/// 欄位在那一支裡面。update_mask 的路徑可以下到那一支（individual.id_card_no、business.tax_id），列 individual 或 business
/// 本身是那一支全部可寫的欄位，"*" 是所有最上層欄位加上設定的那一支。建立時 subject 必須設定其中一支；後來設定另一支並且
/// update_mask 列到它的欄位，就是切換型態，原本那一支專屬的欄位（個人的身分證號、企業的公司資料與公司文件）一併清掉。
/// 身分證號、負責人身分證號與銀行帳號寫入時用 id_card_no、rep_id_card_no、bank_account，讀出時只有遮蔽後的值。日期是日曆日。
/// 證件圖片是狀態：單張的位置（id_card_front、id_card_back、bank_book）列進 update_mask 時整個換成請求的那一張（不帶值
/// 是清掉），company_documents 整個換成請求的清單，每個元素是現有的圖片（asset_id，保留）或新上傳的圖片（staged_token），
/// 同一張圖片不能放進兩個位置。
class KYCApplication extends $pb.GeneratedMessage {
  factory KYCApplication({
    $core.String? id,
    KYCStatus? status,
    $core.String? etag,
    $core.String? realName,
    $5.Date? birthDate,
    $5.Date? idCardIssueDate,
    $core.String? idCardIssueType,
    $core.String? idCardIssueLocation,
    $core.String? englishName,
    $core.String? bankName,
    $core.String? bankCode,
    $core.String? bankAccount,
    $core.String? bankAccountMasked,
    $core.String? productType,
    $core.String? websiteUrl,
    KYCDocument? idCardFront,
    KYCDocument? idCardBack,
    KYCDocument? bankBook,
    IndividualKYC? individual,
    BusinessKYC? business,
    KYCFlow? flow,
    KYCRejection? lastRejection,
    $core.Iterable<KYCField>? missingFields,
    $1.Timestamp? submitTime,
    $1.Timestamp? decideTime,
    $1.Timestamp? createTime,
    $1.Timestamp? updateTime,
    $core.Iterable<StoreField>? missingStoreFields,
    $core.Iterable<KYCIssue>? outstandingIssues,
  }) {
    final result = KYCApplication._();
    if (id != null) result.id = id;
    if (status != null) result.status = status;
    if (etag != null) result.etag = etag;
    if (realName != null) result.realName = realName;
    if (birthDate != null) result.birthDate = birthDate;
    if (idCardIssueDate != null) result.idCardIssueDate = idCardIssueDate;
    if (idCardIssueType != null) result.idCardIssueType = idCardIssueType;
    if (idCardIssueLocation != null)
      result.idCardIssueLocation = idCardIssueLocation;
    if (englishName != null) result.englishName = englishName;
    if (bankName != null) result.bankName = bankName;
    if (bankCode != null) result.bankCode = bankCode;
    if (bankAccount != null) result.bankAccount = bankAccount;
    if (bankAccountMasked != null) result.bankAccountMasked = bankAccountMasked;
    if (productType != null) result.productType = productType;
    if (websiteUrl != null) result.websiteUrl = websiteUrl;
    if (idCardFront != null) result.idCardFront = idCardFront;
    if (idCardBack != null) result.idCardBack = idCardBack;
    if (bankBook != null) result.bankBook = bankBook;
    if (individual != null) result.individual = individual;
    if (business != null) result.business = business;
    if (flow != null) result.flow = flow;
    if (lastRejection != null) result.lastRejection = lastRejection;
    if (missingFields != null) result.missingFields.addAll(missingFields);
    if (submitTime != null) result.submitTime = submitTime;
    if (decideTime != null) result.decideTime = decideTime;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (missingStoreFields != null)
      result.missingStoreFields.addAll(missingStoreFields);
    if (outstandingIssues != null)
      result.outstandingIssues.addAll(outstandingIssues);
    return result;
  }

  KYCApplication._();

  factory KYCApplication.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCApplication()..mergeFromBuffer(data, registry);
  factory KYCApplication.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      KYCApplication()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, KYCApplication_Subject>
      _KYCApplication_SubjectByTag = {
    22: KYCApplication_Subject.individual,
    23: KYCApplication_Subject.business,
    0: KYCApplication_Subject.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KYCApplication',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: KYCApplication.$_createMessage)
    ..oo(0, [22, 23])
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<KYCStatus>(2, _omitFieldNames ? '' : 'status',
        enumValues: KYCStatus.values)
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..aOS(4, _omitFieldNames ? '' : 'realName')
    ..aOM<$5.Date>(5, _omitFieldNames ? '' : 'birthDate',
        subBuilder: $5.Date.$_createMessage)
    ..aOM<$5.Date>(6, _omitFieldNames ? '' : 'idCardIssueDate',
        subBuilder: $5.Date.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'idCardIssueType')
    ..aOS(8, _omitFieldNames ? '' : 'idCardIssueLocation')
    ..aOS(9, _omitFieldNames ? '' : 'englishName')
    ..aOS(10, _omitFieldNames ? '' : 'bankName')
    ..aOS(11, _omitFieldNames ? '' : 'bankCode')
    ..aOS(12, _omitFieldNames ? '' : 'bankAccount')
    ..aOS(13, _omitFieldNames ? '' : 'bankAccountMasked')
    ..aOS(14, _omitFieldNames ? '' : 'productType')
    ..aOS(16, _omitFieldNames ? '' : 'websiteUrl')
    ..aOM<KYCDocument>(19, _omitFieldNames ? '' : 'idCardFront',
        subBuilder: KYCDocument.$_createMessage)
    ..aOM<KYCDocument>(20, _omitFieldNames ? '' : 'idCardBack',
        subBuilder: KYCDocument.$_createMessage)
    ..aOM<KYCDocument>(21, _omitFieldNames ? '' : 'bankBook',
        subBuilder: KYCDocument.$_createMessage)
    ..aOM<IndividualKYC>(22, _omitFieldNames ? '' : 'individual',
        subBuilder: IndividualKYC.$_createMessage)
    ..aOM<BusinessKYC>(23, _omitFieldNames ? '' : 'business',
        subBuilder: BusinessKYC.$_createMessage)
    ..aOM<KYCFlow>(24, _omitFieldNames ? '' : 'flow',
        subBuilder: KYCFlow.$_createMessage)
    ..aOM<KYCRejection>(25, _omitFieldNames ? '' : 'lastRejection',
        subBuilder: KYCRejection.$_createMessage)
    ..pc<KYCField>(
        26, _omitFieldNames ? '' : 'missingFields', $pb.PbFieldType.KE,
        valueOf: KYCField.valueOf,
        enumValues: KYCField.values,
        defaultEnumValue: KYCField.KYC_FIELD_UNSPECIFIED)
    ..aOM<$1.Timestamp>(27, _omitFieldNames ? '' : 'submitTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(28, _omitFieldNames ? '' : 'decideTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(29, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(30, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..pc<StoreField>(
        31, _omitFieldNames ? '' : 'missingStoreFields', $pb.PbFieldType.KE,
        valueOf: StoreField.valueOf,
        enumValues: StoreField.values,
        defaultEnumValue: StoreField.STORE_FIELD_UNSPECIFIED)
    ..pPM<KYCIssue>(32, _omitFieldNames ? '' : 'outstandingIssues',
        subBuilder: KYCIssue.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCApplication clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KYCApplication copyWith(void Function(KYCApplication) updates) =>
      super.copyWith((message) => updates(message as KYCApplication))
          as KYCApplication;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use KYCApplication() / KYCApplication.new instead')
  static KYCApplication create() => KYCApplication._();
  static $pb.GeneratedMessage $_createMessage() => KYCApplication._();
  @$core.override
  KYCApplication createEmptyInstance() => KYCApplication._();
  @$core.pragma('dart2js:noInline')
  static KYCApplication getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<KYCApplication>(
          KYCApplication.$_createMessage);
  static KYCApplication? _defaultInstance;

  @$pb.TagNumber(22)
  @$pb.TagNumber(23)
  KYCApplication_Subject whichSubject() =>
      _KYCApplication_SubjectByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(22)
  @$pb.TagNumber(23)
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
  KYCStatus get status => $_getN(1);
  @$pb.TagNumber(2)
  set status(KYCStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  /// 建立時不帶；修改必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get realName => $_getSZ(3);
  @$pb.TagNumber(4)
  set realName($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRealName() => $_has(3);
  @$pb.TagNumber(4)
  void clearRealName() => $_clearField(4);

  @$pb.TagNumber(5)
  $5.Date get birthDate => $_getN(4);
  @$pb.TagNumber(5)
  set birthDate($5.Date value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasBirthDate() => $_has(4);
  @$pb.TagNumber(5)
  void clearBirthDate() => $_clearField(5);
  @$pb.TagNumber(5)
  $5.Date ensureBirthDate() => $_ensure(4);

  @$pb.TagNumber(6)
  $5.Date get idCardIssueDate => $_getN(5);
  @$pb.TagNumber(6)
  set idCardIssueDate($5.Date value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasIdCardIssueDate() => $_has(5);
  @$pb.TagNumber(6)
  void clearIdCardIssueDate() => $_clearField(6);
  @$pb.TagNumber(6)
  $5.Date ensureIdCardIssueDate() => $_ensure(5);

  @$pb.TagNumber(7)
  $core.String get idCardIssueType => $_getSZ(6);
  @$pb.TagNumber(7)
  set idCardIssueType($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasIdCardIssueType() => $_has(6);
  @$pb.TagNumber(7)
  void clearIdCardIssueType() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get idCardIssueLocation => $_getSZ(7);
  @$pb.TagNumber(8)
  set idCardIssueLocation($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasIdCardIssueLocation() => $_has(7);
  @$pb.TagNumber(8)
  void clearIdCardIssueLocation() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get englishName => $_getSZ(8);
  @$pb.TagNumber(9)
  set englishName($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasEnglishName() => $_has(8);
  @$pb.TagNumber(9)
  void clearEnglishName() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get bankName => $_getSZ(9);
  @$pb.TagNumber(10)
  set bankName($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasBankName() => $_has(9);
  @$pb.TagNumber(10)
  void clearBankName() => $_clearField(10);

  /// 3 碼銀行代碼，或 3 碼加 4 碼分行代碼。
  @$pb.TagNumber(11)
  $core.String get bankCode => $_getSZ(10);
  @$pb.TagNumber(11)
  set bankCode($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasBankCode() => $_has(10);
  @$pb.TagNumber(11)
  void clearBankCode() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get bankAccount => $_getSZ(11);
  @$pb.TagNumber(12)
  set bankAccount($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasBankAccount() => $_has(11);
  @$pb.TagNumber(12)
  void clearBankAccount() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get bankAccountMasked => $_getSZ(12);
  @$pb.TagNumber(13)
  set bankAccountMasked($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasBankAccountMasked() => $_has(12);
  @$pb.TagNumber(13)
  void clearBankAccountMasked() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get productType => $_getSZ(13);
  @$pb.TagNumber(14)
  set productType($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasProductType() => $_has(13);
  @$pb.TagNumber(14)
  void clearProductType() => $_clearField(14);

  @$pb.TagNumber(16)
  $core.String get websiteUrl => $_getSZ(14);
  @$pb.TagNumber(16)
  set websiteUrl($core.String value) => $_setString(14, value);
  @$pb.TagNumber(16)
  $core.bool hasWebsiteUrl() => $_has(14);
  @$pb.TagNumber(16)
  void clearWebsiteUrl() => $_clearField(16);

  @$pb.TagNumber(19)
  KYCDocument get idCardFront => $_getN(15);
  @$pb.TagNumber(19)
  set idCardFront(KYCDocument value) => $_setField(19, value);
  @$pb.TagNumber(19)
  $core.bool hasIdCardFront() => $_has(15);
  @$pb.TagNumber(19)
  void clearIdCardFront() => $_clearField(19);
  @$pb.TagNumber(19)
  KYCDocument ensureIdCardFront() => $_ensure(15);

  @$pb.TagNumber(20)
  KYCDocument get idCardBack => $_getN(16);
  @$pb.TagNumber(20)
  set idCardBack(KYCDocument value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasIdCardBack() => $_has(16);
  @$pb.TagNumber(20)
  void clearIdCardBack() => $_clearField(20);
  @$pb.TagNumber(20)
  KYCDocument ensureIdCardBack() => $_ensure(16);

  @$pb.TagNumber(21)
  KYCDocument get bankBook => $_getN(17);
  @$pb.TagNumber(21)
  set bankBook(KYCDocument value) => $_setField(21, value);
  @$pb.TagNumber(21)
  $core.bool hasBankBook() => $_has(17);
  @$pb.TagNumber(21)
  void clearBankBook() => $_clearField(21);
  @$pb.TagNumber(21)
  KYCDocument ensureBankBook() => $_ensure(17);

  @$pb.TagNumber(22)
  IndividualKYC get individual => $_getN(18);
  @$pb.TagNumber(22)
  set individual(IndividualKYC value) => $_setField(22, value);
  @$pb.TagNumber(22)
  $core.bool hasIndividual() => $_has(18);
  @$pb.TagNumber(22)
  void clearIndividual() => $_clearField(22);
  @$pb.TagNumber(22)
  IndividualKYC ensureIndividual() => $_ensure(18);

  @$pb.TagNumber(23)
  BusinessKYC get business => $_getN(19);
  @$pb.TagNumber(23)
  set business(BusinessKYC value) => $_setField(23, value);
  @$pb.TagNumber(23)
  $core.bool hasBusiness() => $_has(19);
  @$pb.TagNumber(23)
  void clearBusiness() => $_clearField(23);
  @$pb.TagNumber(23)
  BusinessKYC ensureBusiness() => $_ensure(19);

  @$pb.TagNumber(24)
  KYCFlow get flow => $_getN(20);
  @$pb.TagNumber(24)
  set flow(KYCFlow value) => $_setField(24, value);
  @$pb.TagNumber(24)
  $core.bool hasFlow() => $_has(20);
  @$pb.TagNumber(24)
  void clearFlow() => $_clearField(24);
  @$pb.TagNumber(24)
  KYCFlow ensureFlow() => $_ensure(20);

  /// 最近一次退回，只在「最近一次審核紀錄是退回、賣家還沒重新送出（核准、取消之後也算）」時設，其餘時候不設；client 直接看有沒有
  /// 這個欄位決定要不要顯示總理由與「需重新送出」的提示，不必自己推導。賣家把每一項問題都處理完、還沒重新送出時它仍在。
  @$pb.TagNumber(25)
  KYCRejection get lastRejection => $_getN(21);
  @$pb.TagNumber(25)
  set lastRejection(KYCRejection value) => $_setField(25, value);
  @$pb.TagNumber(25)
  $core.bool hasLastRejection() => $_has(21);
  @$pb.TagNumber(25)
  void clearLastRejection() => $_clearField(25);
  @$pb.TagNumber(25)
  KYCRejection ensureLastRejection() => $_ensure(21);

  /// 送出之前還缺的欄位（與 SubmitKYC 檢查的是同一份規則），依 KYCField 的數字順序；齊全時是空的。client 以它決定續填的
  /// 位置與哪些欄位標紅，不必自己重寫完整性規則。
  @$pb.TagNumber(26)
  $pb.PbList<KYCField> get missingFields => $_getList(22);

  @$pb.TagNumber(27)
  $1.Timestamp get submitTime => $_getN(23);
  @$pb.TagNumber(27)
  set submitTime($1.Timestamp value) => $_setField(27, value);
  @$pb.TagNumber(27)
  $core.bool hasSubmitTime() => $_has(23);
  @$pb.TagNumber(27)
  void clearSubmitTime() => $_clearField(27);
  @$pb.TagNumber(27)
  $1.Timestamp ensureSubmitTime() => $_ensure(23);

  @$pb.TagNumber(28)
  $1.Timestamp get decideTime => $_getN(24);
  @$pb.TagNumber(28)
  set decideTime($1.Timestamp value) => $_setField(28, value);
  @$pb.TagNumber(28)
  $core.bool hasDecideTime() => $_has(24);
  @$pb.TagNumber(28)
  void clearDecideTime() => $_clearField(28);
  @$pb.TagNumber(28)
  $1.Timestamp ensureDecideTime() => $_ensure(24);

  @$pb.TagNumber(29)
  $1.Timestamp get createTime => $_getN(25);
  @$pb.TagNumber(29)
  set createTime($1.Timestamp value) => $_setField(29, value);
  @$pb.TagNumber(29)
  $core.bool hasCreateTime() => $_has(25);
  @$pb.TagNumber(29)
  void clearCreateTime() => $_clearField(29);
  @$pb.TagNumber(29)
  $1.Timestamp ensureCreateTime() => $_ensure(25);

  @$pb.TagNumber(30)
  $1.Timestamp get updateTime => $_getN(26);
  @$pb.TagNumber(30)
  set updateTime($1.Timestamp value) => $_setField(30, value);
  @$pb.TagNumber(30)
  $core.bool hasUpdateTime() => $_has(26);
  @$pb.TagNumber(30)
  void clearUpdateTime() => $_clearField(30);
  @$pb.TagNumber(30)
  $1.Timestamp ensureUpdateTime() => $_ensure(26);

  /// 送出之前還缺的店家欄位（PayUni 綁定需要的店家資料，與 SubmitKYC 檢查的是同一份規則），依 StoreField 的數字順序；齊全時是空的。
  /// 這些欄位用 UpdateMyStore 寫，不在申請上。
  @$pb.TagNumber(31)
  $pb.PbList<StoreField> get missingStoreFields => $_getList(27);

  /// 最近一次退回指出的問題裡賣家還沒處理的（已處理由伺服器判定：該欄位的值在退回之後被改過，或指到的文件已被移除或替換）；
  /// 重新送出、核准或取消之後是空的。client 以它標出要補的欄位與文件，不必自己推導退回是否仍然有效。
  @$pb.TagNumber(32)
  $pb.PbList<KYCIssue> get outstandingIssues => $_getList(28);
}

class SaveKYCDraftRequest extends $pb.GeneratedMessage {
  factory SaveKYCDraftRequest({
    KYCApplication? application,
    $4.FieldMask? updateMask,
  }) {
    final result = SaveKYCDraftRequest._();
    if (application != null) result.application = application;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  SaveKYCDraftRequest._();

  factory SaveKYCDraftRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SaveKYCDraftRequest()..mergeFromBuffer(data, registry);
  factory SaveKYCDraftRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SaveKYCDraftRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SaveKYCDraftRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SaveKYCDraftRequest.$_createMessage)
    ..aOM<KYCApplication>(1, _omitFieldNames ? '' : 'application',
        subBuilder: KYCApplication.$_createMessage)
    ..aOM<$4.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $4.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SaveKYCDraftRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SaveKYCDraftRequest copyWith(void Function(SaveKYCDraftRequest) updates) =>
      super.copyWith((message) => updates(message as SaveKYCDraftRequest))
          as SaveKYCDraftRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SaveKYCDraftRequest() / SaveKYCDraftRequest.new instead')
  static SaveKYCDraftRequest create() => SaveKYCDraftRequest._();
  static $pb.GeneratedMessage $_createMessage() => SaveKYCDraftRequest._();
  @$core.override
  SaveKYCDraftRequest createEmptyInstance() => SaveKYCDraftRequest._();
  @$core.pragma('dart2js:noInline')
  static SaveKYCDraftRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SaveKYCDraftRequest>(
          SaveKYCDraftRequest.$_createMessage);
  static SaveKYCDraftRequest? _defaultInstance;

  /// 草稿；已經有申請時 etag 必填，建立時（不帶 etag）subject 必須設定其中一支。
  @$pb.TagNumber(1)
  KYCApplication get application => $_getN(0);
  @$pb.TagNumber(1)
  set application(KYCApplication value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasApplication() => $_has(0);
  @$pb.TagNumber(1)
  void clearApplication() => $_clearField(1);
  @$pb.TagNumber(1)
  KYCApplication ensureApplication() => $_ensure(0);

  /// 只改列出的欄位；沒帶時視為列出所有非零值的欄位。
  @$pb.TagNumber(2)
  $4.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($4.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $4.FieldMask ensureUpdateMask() => $_ensure(1);
}

class SaveKYCDraftResponse extends $pb.GeneratedMessage {
  factory SaveKYCDraftResponse({
    KYCApplication? application,
  }) {
    final result = SaveKYCDraftResponse._();
    if (application != null) result.application = application;
    return result;
  }

  SaveKYCDraftResponse._();

  factory SaveKYCDraftResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SaveKYCDraftResponse()..mergeFromBuffer(data, registry);
  factory SaveKYCDraftResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SaveKYCDraftResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SaveKYCDraftResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SaveKYCDraftResponse.$_createMessage)
    ..aOM<KYCApplication>(1, _omitFieldNames ? '' : 'application',
        subBuilder: KYCApplication.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SaveKYCDraftResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SaveKYCDraftResponse copyWith(void Function(SaveKYCDraftResponse) updates) =>
      super.copyWith((message) => updates(message as SaveKYCDraftResponse))
          as SaveKYCDraftResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SaveKYCDraftResponse() / SaveKYCDraftResponse.new instead')
  static SaveKYCDraftResponse create() => SaveKYCDraftResponse._();
  static $pb.GeneratedMessage $_createMessage() => SaveKYCDraftResponse._();
  @$core.override
  SaveKYCDraftResponse createEmptyInstance() => SaveKYCDraftResponse._();
  @$core.pragma('dart2js:noInline')
  static SaveKYCDraftResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SaveKYCDraftResponse>(
          SaveKYCDraftResponse.$_createMessage);
  static SaveKYCDraftResponse? _defaultInstance;

  @$pb.TagNumber(1)
  KYCApplication get application => $_getN(0);
  @$pb.TagNumber(1)
  set application(KYCApplication value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasApplication() => $_has(0);
  @$pb.TagNumber(1)
  void clearApplication() => $_clearField(1);
  @$pb.TagNumber(1)
  KYCApplication ensureApplication() => $_ensure(0);
}

class GetMyKYCRequest extends $pb.GeneratedMessage {
  factory GetMyKYCRequest() => GetMyKYCRequest._();

  GetMyKYCRequest._();

  factory GetMyKYCRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyKYCRequest()..mergeFromBuffer(data, registry);
  factory GetMyKYCRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyKYCRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyKYCRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyKYCRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyKYCRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyKYCRequest copyWith(void Function(GetMyKYCRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyKYCRequest))
          as GetMyKYCRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyKYCRequest() / GetMyKYCRequest.new instead')
  static GetMyKYCRequest create() => GetMyKYCRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyKYCRequest._();
  @$core.override
  GetMyKYCRequest createEmptyInstance() => GetMyKYCRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyKYCRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetMyKYCRequest>(
          GetMyKYCRequest.$_createMessage);
  static GetMyKYCRequest? _defaultInstance;
}

class GetMyKYCResponse extends $pb.GeneratedMessage {
  factory GetMyKYCResponse({
    KYCApplication? application,
  }) {
    final result = GetMyKYCResponse._();
    if (application != null) result.application = application;
    return result;
  }

  GetMyKYCResponse._();

  factory GetMyKYCResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyKYCResponse()..mergeFromBuffer(data, registry);
  factory GetMyKYCResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyKYCResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyKYCResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyKYCResponse.$_createMessage)
    ..aOM<KYCApplication>(1, _omitFieldNames ? '' : 'application',
        subBuilder: KYCApplication.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyKYCResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyKYCResponse copyWith(void Function(GetMyKYCResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyKYCResponse))
          as GetMyKYCResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyKYCResponse() / GetMyKYCResponse.new instead')
  static GetMyKYCResponse create() => GetMyKYCResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyKYCResponse._();
  @$core.override
  GetMyKYCResponse createEmptyInstance() => GetMyKYCResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyKYCResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetMyKYCResponse>(
          GetMyKYCResponse.$_createMessage);
  static GetMyKYCResponse? _defaultInstance;

  @$pb.TagNumber(1)
  KYCApplication get application => $_getN(0);
  @$pb.TagNumber(1)
  set application(KYCApplication value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasApplication() => $_has(0);
  @$pb.TagNumber(1)
  void clearApplication() => $_clearField(1);
  @$pb.TagNumber(1)
  KYCApplication ensureApplication() => $_ensure(0);
}

class SubmitKYCRequest extends $pb.GeneratedMessage {
  factory SubmitKYCRequest({
    $core.String? etag,
    $core.Iterable<AgreementAcceptance>? acceptedAgreements,
  }) {
    final result = SubmitKYCRequest._();
    if (etag != null) result.etag = etag;
    if (acceptedAgreements != null)
      result.acceptedAgreements.addAll(acceptedAgreements);
    return result;
  }

  SubmitKYCRequest._();

  factory SubmitKYCRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitKYCRequest()..mergeFromBuffer(data, registry);
  factory SubmitKYCRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitKYCRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitKYCRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitKYCRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'etag')
    ..pPM<AgreementAcceptance>(2, _omitFieldNames ? '' : 'acceptedAgreements',
        subBuilder: AgreementAcceptance.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitKYCRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitKYCRequest copyWith(void Function(SubmitKYCRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitKYCRequest))
          as SubmitKYCRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SubmitKYCRequest() / SubmitKYCRequest.new instead')
  static SubmitKYCRequest create() => SubmitKYCRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitKYCRequest._();
  @$core.override
  SubmitKYCRequest createEmptyInstance() => SubmitKYCRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitKYCRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SubmitKYCRequest>(
          SubmitKYCRequest.$_createMessage);
  static SubmitKYCRequest? _defaultInstance;

  /// 讀到的申請的 etag。
  @$pb.TagNumber(1)
  $core.String get etag => $_getSZ(0);
  @$pb.TagNumber(1)
  set etag($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEtag() => $_has(0);
  @$pb.TagNumber(1)
  void clearEtag() => $_clearField(1);

  /// 賣家確認已閱讀的 KYC_PRIVACY_NOTICE 與同意的 PAYUNI_TERMS，剛好這兩份。缺種類、多出不屬於這個操作的種類、同一種重複或
  /// UNSPECIFIED 回 invalid_argument（field violation 指向 accepted_agreements）；版本不是現行版本回
  /// failed_precondition，ErrorInfo.reason 是 AGREEMENT_OUTDATED，client 重讀 GetSellerAgreements，請賣家重新勾選。
  /// 同意與送出同一個 transaction；每次送出各記一組同意，綁定那次送出的審核紀錄（被退回後再送出，也各自留下一組）。
  @$pb.TagNumber(2)
  $pb.PbList<AgreementAcceptance> get acceptedAgreements => $_getList(1);
}

class SubmitKYCResponse extends $pb.GeneratedMessage {
  factory SubmitKYCResponse({
    KYCApplication? application,
  }) {
    final result = SubmitKYCResponse._();
    if (application != null) result.application = application;
    return result;
  }

  SubmitKYCResponse._();

  factory SubmitKYCResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitKYCResponse()..mergeFromBuffer(data, registry);
  factory SubmitKYCResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitKYCResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitKYCResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitKYCResponse.$_createMessage)
    ..aOM<KYCApplication>(1, _omitFieldNames ? '' : 'application',
        subBuilder: KYCApplication.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitKYCResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitKYCResponse copyWith(void Function(SubmitKYCResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitKYCResponse))
          as SubmitKYCResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SubmitKYCResponse() / SubmitKYCResponse.new instead')
  static SubmitKYCResponse create() => SubmitKYCResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubmitKYCResponse._();
  @$core.override
  SubmitKYCResponse createEmptyInstance() => SubmitKYCResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitKYCResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SubmitKYCResponse>(
          SubmitKYCResponse.$_createMessage);
  static SubmitKYCResponse? _defaultInstance;

  @$pb.TagNumber(1)
  KYCApplication get application => $_getN(0);
  @$pb.TagNumber(1)
  set application(KYCApplication value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasApplication() => $_has(0);
  @$pb.TagNumber(1)
  void clearApplication() => $_clearField(1);
  @$pb.TagNumber(1)
  KYCApplication ensureApplication() => $_ensure(0);
}

class CancelKYCRequest extends $pb.GeneratedMessage {
  factory CancelKYCRequest({
    $core.String? etag,
    $core.String? reason,
  }) {
    final result = CancelKYCRequest._();
    if (etag != null) result.etag = etag;
    if (reason != null) result.reason = reason;
    return result;
  }

  CancelKYCRequest._();

  factory CancelKYCRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelKYCRequest()..mergeFromBuffer(data, registry);
  factory CancelKYCRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelKYCRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelKYCRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CancelKYCRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'etag')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelKYCRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelKYCRequest copyWith(void Function(CancelKYCRequest) updates) =>
      super.copyWith((message) => updates(message as CancelKYCRequest))
          as CancelKYCRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelKYCRequest() / CancelKYCRequest.new instead')
  static CancelKYCRequest create() => CancelKYCRequest._();
  static $pb.GeneratedMessage $_createMessage() => CancelKYCRequest._();
  @$core.override
  CancelKYCRequest createEmptyInstance() => CancelKYCRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelKYCRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelKYCRequest>(
          CancelKYCRequest.$_createMessage);
  static CancelKYCRequest? _defaultInstance;

  /// 讀到的申請的 etag。
  @$pb.TagNumber(1)
  $core.String get etag => $_getSZ(0);
  @$pb.TagNumber(1)
  set etag($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEtag() => $_has(0);
  @$pb.TagNumber(1)
  void clearEtag() => $_clearField(1);

  /// 取消的原因，可以不填。
  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class CancelKYCResponse extends $pb.GeneratedMessage {
  factory CancelKYCResponse({
    KYCApplication? application,
  }) {
    final result = CancelKYCResponse._();
    if (application != null) result.application = application;
    return result;
  }

  CancelKYCResponse._();

  factory CancelKYCResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelKYCResponse()..mergeFromBuffer(data, registry);
  factory CancelKYCResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelKYCResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelKYCResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: CancelKYCResponse.$_createMessage)
    ..aOM<KYCApplication>(1, _omitFieldNames ? '' : 'application',
        subBuilder: KYCApplication.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelKYCResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelKYCResponse copyWith(void Function(CancelKYCResponse) updates) =>
      super.copyWith((message) => updates(message as CancelKYCResponse))
          as CancelKYCResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelKYCResponse() / CancelKYCResponse.new instead')
  static CancelKYCResponse create() => CancelKYCResponse._();
  static $pb.GeneratedMessage $_createMessage() => CancelKYCResponse._();
  @$core.override
  CancelKYCResponse createEmptyInstance() => CancelKYCResponse._();
  @$core.pragma('dart2js:noInline')
  static CancelKYCResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelKYCResponse>(
          CancelKYCResponse.$_createMessage);
  static CancelKYCResponse? _defaultInstance;

  @$pb.TagNumber(1)
  KYCApplication get application => $_getN(0);
  @$pb.TagNumber(1)
  set application(KYCApplication value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasApplication() => $_has(0);
  @$pb.TagNumber(1)
  void clearApplication() => $_clearField(1);
  @$pb.TagNumber(1)
  KYCApplication ensureApplication() => $_ensure(0);
}

/// BusinessRegistration 是經濟部商業司的一筆登記資料。
class BusinessRegistration extends $pb.GeneratedMessage {
  factory BusinessRegistration({
    $core.String? taxId,
    $core.String? name,
    $core.String? address,
    $5.Date? setupDate,
    $core.String? organizationType,
    $2.PostalAddress? postalAddress,
  }) {
    final result = BusinessRegistration._();
    if (taxId != null) result.taxId = taxId;
    if (name != null) result.name = name;
    if (address != null) result.address = address;
    if (setupDate != null) result.setupDate = setupDate;
    if (organizationType != null) result.organizationType = organizationType;
    if (postalAddress != null) result.postalAddress = postalAddress;
    return result;
  }

  BusinessRegistration._();

  factory BusinessRegistration.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BusinessRegistration()..mergeFromBuffer(data, registry);
  factory BusinessRegistration.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BusinessRegistration()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BusinessRegistration',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: BusinessRegistration.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'taxId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'address')
    ..aOM<$5.Date>(6, _omitFieldNames ? '' : 'setupDate',
        subBuilder: $5.Date.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'organizationType')
    ..aOM<$2.PostalAddress>(8, _omitFieldNames ? '' : 'postalAddress',
        subBuilder: $2.PostalAddress.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BusinessRegistration clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BusinessRegistration copyWith(void Function(BusinessRegistration) updates) =>
      super.copyWith((message) => updates(message as BusinessRegistration))
          as BusinessRegistration;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BusinessRegistration() / BusinessRegistration.new instead')
  static BusinessRegistration create() => BusinessRegistration._();
  static $pb.GeneratedMessage $_createMessage() => BusinessRegistration._();
  @$core.override
  BusinessRegistration createEmptyInstance() => BusinessRegistration._();
  @$core.pragma('dart2js:noInline')
  static BusinessRegistration getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BusinessRegistration>(
          BusinessRegistration.$_createMessage);
  static BusinessRegistration? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get taxId => $_getSZ(0);
  @$pb.TagNumber(1)
  set taxId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTaxId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTaxId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  /// 登記的地址，原樣的文字。
  @$pb.TagNumber(3)
  $core.String get address => $_getSZ(2);
  @$pb.TagNumber(3)
  set address($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAddress() => $_has(2);
  @$pb.TagNumber(3)
  void clearAddress() => $_clearField(3);

  /// 設立日期；來源缺漏時不設。
  @$pb.TagNumber(6)
  $5.Date get setupDate => $_getN(3);
  @$pb.TagNumber(6)
  set setupDate($5.Date value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasSetupDate() => $_has(3);
  @$pb.TagNumber(6)
  void clearSetupDate() => $_clearField(6);
  @$pb.TagNumber(6)
  $5.Date ensureSetupDate() => $_ensure(3);

  /// 組織型態（股份有限公司、有限公司、獨資、合夥……）；判斷不出時是空字串。
  @$pb.TagNumber(7)
  $core.String get organizationType => $_getSZ(4);
  @$pb.TagNumber(7)
  set organizationType($core.String value) => $_setString(4, value);
  @$pb.TagNumber(7)
  $core.bool hasOrganizationType() => $_has(4);
  @$pb.TagNumber(7)
  void clearOrganizationType() => $_clearField(7);

  /// 登記的地址開頭是現行的縣市與鄉鎮市區時，拆出來的鄉鎮市區代碼與其後的地址；拆不出來時不設，client 讓使用者自己選。
  @$pb.TagNumber(8)
  $2.PostalAddress get postalAddress => $_getN(5);
  @$pb.TagNumber(8)
  set postalAddress($2.PostalAddress value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasPostalAddress() => $_has(5);
  @$pb.TagNumber(8)
  void clearPostalAddress() => $_clearField(8);
  @$pb.TagNumber(8)
  $2.PostalAddress ensurePostalAddress() => $_ensure(5);
}

class LookupBusinessRegistrationRequest extends $pb.GeneratedMessage {
  factory LookupBusinessRegistrationRequest({
    $core.String? taxId,
  }) {
    final result = LookupBusinessRegistrationRequest._();
    if (taxId != null) result.taxId = taxId;
    return result;
  }

  LookupBusinessRegistrationRequest._();

  factory LookupBusinessRegistrationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LookupBusinessRegistrationRequest()..mergeFromBuffer(data, registry);
  factory LookupBusinessRegistrationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LookupBusinessRegistrationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupBusinessRegistrationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: LookupBusinessRegistrationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'taxId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupBusinessRegistrationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupBusinessRegistrationRequest copyWith(
          void Function(LookupBusinessRegistrationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as LookupBusinessRegistrationRequest))
          as LookupBusinessRegistrationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use LookupBusinessRegistrationRequest() / LookupBusinessRegistrationRequest.new instead')
  static LookupBusinessRegistrationRequest create() =>
      LookupBusinessRegistrationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      LookupBusinessRegistrationRequest._();
  @$core.override
  LookupBusinessRegistrationRequest createEmptyInstance() =>
      LookupBusinessRegistrationRequest._();
  @$core.pragma('dart2js:noInline')
  static LookupBusinessRegistrationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LookupBusinessRegistrationRequest>(
          LookupBusinessRegistrationRequest.$_createMessage);
  static LookupBusinessRegistrationRequest? _defaultInstance;

  /// 8 碼統一編號。
  @$pb.TagNumber(1)
  $core.String get taxId => $_getSZ(0);
  @$pb.TagNumber(1)
  set taxId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTaxId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTaxId() => $_clearField(1);
}

class LookupBusinessRegistrationResponse extends $pb.GeneratedMessage {
  factory LookupBusinessRegistrationResponse({
    BusinessRegistration? registration,
  }) {
    final result = LookupBusinessRegistrationResponse._();
    if (registration != null) result.registration = registration;
    return result;
  }

  LookupBusinessRegistrationResponse._();

  factory LookupBusinessRegistrationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LookupBusinessRegistrationResponse()..mergeFromBuffer(data, registry);
  factory LookupBusinessRegistrationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LookupBusinessRegistrationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupBusinessRegistrationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: LookupBusinessRegistrationResponse.$_createMessage)
    ..aOM<BusinessRegistration>(1, _omitFieldNames ? '' : 'registration',
        subBuilder: BusinessRegistration.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupBusinessRegistrationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupBusinessRegistrationResponse copyWith(
          void Function(LookupBusinessRegistrationResponse) updates) =>
      super.copyWith((message) =>
              updates(message as LookupBusinessRegistrationResponse))
          as LookupBusinessRegistrationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use LookupBusinessRegistrationResponse() / LookupBusinessRegistrationResponse.new instead')
  static LookupBusinessRegistrationResponse create() =>
      LookupBusinessRegistrationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      LookupBusinessRegistrationResponse._();
  @$core.override
  LookupBusinessRegistrationResponse createEmptyInstance() =>
      LookupBusinessRegistrationResponse._();
  @$core.pragma('dart2js:noInline')
  static LookupBusinessRegistrationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LookupBusinessRegistrationResponse>(
          LookupBusinessRegistrationResponse.$_createMessage);
  static LookupBusinessRegistrationResponse? _defaultInstance;

  /// 查無這個統編時不設。
  @$pb.TagNumber(1)
  BusinessRegistration get registration => $_getN(0);
  @$pb.TagNumber(1)
  set registration(BusinessRegistration value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRegistration() => $_has(0);
  @$pb.TagNumber(1)
  void clearRegistration() => $_clearField(1);
  @$pb.TagNumber(1)
  BusinessRegistration ensureRegistration() => $_ensure(0);
}

/// ContentRef 是內容下架措施的一個對象：種類加上該內容的 ID（不透明的字串）。
class ContentRef extends $pb.GeneratedMessage {
  factory ContentRef({
    ContentKind? kind,
    $core.String? id,
  }) {
    final result = ContentRef._();
    if (kind != null) result.kind = kind;
    if (id != null) result.id = id;
    return result;
  }

  ContentRef._();

  factory ContentRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ContentRef()..mergeFromBuffer(data, registry);
  factory ContentRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ContentRef()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ContentRef',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ContentRef.$_createMessage)
    ..aE<ContentKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: ContentKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContentRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContentRef copyWith(void Function(ContentRef) updates) =>
      super.copyWith((message) => updates(message as ContentRef)) as ContentRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ContentRef() / ContentRef.new instead')
  static ContentRef create() => ContentRef._();
  static $pb.GeneratedMessage $_createMessage() => ContentRef._();
  @$core.override
  ContentRef createEmptyInstance() => ContentRef._();
  @$core.pragma('dart2js:noInline')
  static ContentRef getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ContentRef>(ContentRef.$_createMessage);
  static ContentRef? _defaultInstance;

  @$pb.TagNumber(1)
  ContentKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(ContentKind value) => $_setField(1, value);
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

/// RemediationUnavailability 是賣家此刻不能對措施請求解除的一個原因。與 RequestRemediation 的前置檢查是同一個規則，message 與該方法被拒絕時錯誤的
/// LocalizedMessage 是同一份文字（同一個 Go 函式）。
class RemediationUnavailability extends $pb.GeneratedMessage {
  factory RemediationUnavailability({
    RemediationUnavailableReason? reason,
    $3.LocalizedMessage? message,
  }) {
    final result = RemediationUnavailability._();
    if (reason != null) result.reason = reason;
    if (message != null) result.message = message;
    return result;
  }

  RemediationUnavailability._();

  factory RemediationUnavailability.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemediationUnavailability()..mergeFromBuffer(data, registry);
  factory RemediationUnavailability.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemediationUnavailability()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemediationUnavailability',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: RemediationUnavailability.$_createMessage)
    ..aE<RemediationUnavailableReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: RemediationUnavailableReason.values)
    ..aOM<$3.LocalizedMessage>(2, _omitFieldNames ? '' : 'message',
        subBuilder: $3.LocalizedMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemediationUnavailability clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemediationUnavailability copyWith(
          void Function(RemediationUnavailability) updates) =>
      super.copyWith((message) => updates(message as RemediationUnavailability))
          as RemediationUnavailability;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RemediationUnavailability() / RemediationUnavailability.new instead')
  static RemediationUnavailability create() => RemediationUnavailability._();
  static $pb.GeneratedMessage $_createMessage() =>
      RemediationUnavailability._();
  @$core.override
  RemediationUnavailability createEmptyInstance() =>
      RemediationUnavailability._();
  @$core.pragma('dart2js:noInline')
  static RemediationUnavailability getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemediationUnavailability>(
          RemediationUnavailability.$_createMessage);
  static RemediationUnavailability? _defaultInstance;

  /// 原因，給 client 依原因分支用（例如沒有解除請求入口時不顯示）。伺服器一律設，不會是 UNSPECIFIED。
  @$pb.TagNumber(1)
  RemediationUnavailableReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason(RemediationUnavailableReason value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  /// 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；client 直接顯示，不另建原因對照表，沒有 message 時才顯示通用文字。
  @$pb.TagNumber(2)
  $3.LocalizedMessage get message => $_getN(1);
  @$pb.TagNumber(2)
  set message($3.LocalizedMessage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.LocalizedMessage ensureMessage() => $_ensure(1);
}

/// AppealUnavailability 是賣家此刻不能對措施提出申覆的一個原因。與 SubmitAppeal 的前置檢查是同一個規則，message 與該方法被拒絕時錯誤的
/// LocalizedMessage 是同一份文字（同一個 Go 函式）。
class AppealUnavailability extends $pb.GeneratedMessage {
  factory AppealUnavailability({
    AppealUnavailableReason? reason,
    $3.LocalizedMessage? message,
  }) {
    final result = AppealUnavailability._();
    if (reason != null) result.reason = reason;
    if (message != null) result.message = message;
    return result;
  }

  AppealUnavailability._();

  factory AppealUnavailability.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AppealUnavailability()..mergeFromBuffer(data, registry);
  factory AppealUnavailability.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AppealUnavailability()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AppealUnavailability',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: AppealUnavailability.$_createMessage)
    ..aE<AppealUnavailableReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: AppealUnavailableReason.values)
    ..aOM<$3.LocalizedMessage>(2, _omitFieldNames ? '' : 'message',
        subBuilder: $3.LocalizedMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AppealUnavailability clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AppealUnavailability copyWith(void Function(AppealUnavailability) updates) =>
      super.copyWith((message) => updates(message as AppealUnavailability))
          as AppealUnavailability;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AppealUnavailability() / AppealUnavailability.new instead')
  static AppealUnavailability create() => AppealUnavailability._();
  static $pb.GeneratedMessage $_createMessage() => AppealUnavailability._();
  @$core.override
  AppealUnavailability createEmptyInstance() => AppealUnavailability._();
  @$core.pragma('dart2js:noInline')
  static AppealUnavailability getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AppealUnavailability>(
          AppealUnavailability.$_createMessage);
  static AppealUnavailability? _defaultInstance;

  /// 原因，給 client 依原因分支用。伺服器一律設，不會是 UNSPECIFIED。
  @$pb.TagNumber(1)
  AppealUnavailableReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason(AppealUnavailableReason value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  /// 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；client 直接顯示，不另建原因對照表，沒有 message 時才顯示通用文字。
  @$pb.TagNumber(2)
  $3.LocalizedMessage get message => $_getN(1);
  @$pb.TagNumber(2)
  set message($3.LocalizedMessage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.LocalizedMessage ensureMessage() => $_ensure(1);
}

/// MeasureFinding 是措施依據的認定（違規）。
class MeasureFinding extends $pb.GeneratedMessage {
  factory MeasureFinding({
    $core.String? id,
    $8.PolicyCategory? category,
    ViolationSeverity? severity,
    $1.Timestamp? createTime,
  }) {
    final result = MeasureFinding._();
    if (id != null) result.id = id;
    if (category != null) result.category = category;
    if (severity != null) result.severity = severity;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  MeasureFinding._();

  factory MeasureFinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasureFinding()..mergeFromBuffer(data, registry);
  factory MeasureFinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasureFinding()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MeasureFinding',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: MeasureFinding.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<$8.PolicyCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: $8.PolicyCategory.values)
    ..aE<ViolationSeverity>(3, _omitFieldNames ? '' : 'severity',
        enumValues: ViolationSeverity.values)
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasureFinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasureFinding copyWith(void Function(MeasureFinding) updates) =>
      super.copyWith((message) => updates(message as MeasureFinding))
          as MeasureFinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MeasureFinding() / MeasureFinding.new instead')
  static MeasureFinding create() => MeasureFinding._();
  static $pb.GeneratedMessage $_createMessage() => MeasureFinding._();
  @$core.override
  MeasureFinding createEmptyInstance() => MeasureFinding._();
  @$core.pragma('dart2js:noInline')
  static MeasureFinding getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MeasureFinding>(
          MeasureFinding.$_createMessage);
  static MeasureFinding? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 認定的類別：政策分類中平台判定與認定可用的值（不含 OTHER）。
  @$pb.TagNumber(2)
  $8.PolicyCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category($8.PolicyCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  @$pb.TagNumber(3)
  ViolationSeverity get severity => $_getN(2);
  @$pb.TagNumber(3)
  set severity(ViolationSeverity value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSeverity() => $_has(2);
  @$pb.TagNumber(3)
  void clearSeverity() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get createTime => $_getN(3);
  @$pb.TagNumber(4)
  set createTime($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCreateTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreateTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureCreateTime() => $_ensure(3);
}

/// MeasurePeriod 是措施的期間。時間到了不改變這些時間欄位（也不改變措施的 etag），此刻是否仍生效看 Measure.state。
class MeasurePeriod extends $pb.GeneratedMessage {
  factory MeasurePeriod({
    $1.Timestamp? startTime,
    $1.Timestamp? endTime,
  }) {
    final result = MeasurePeriod._();
    if (startTime != null) result.startTime = startTime;
    if (endTime != null) result.endTime = endTime;
    return result;
  }

  MeasurePeriod._();

  factory MeasurePeriod.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasurePeriod()..mergeFromBuffer(data, registry);
  factory MeasurePeriod.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasurePeriod()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MeasurePeriod',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: MeasurePeriod.$_createMessage)
    ..aOM<$1.Timestamp>(1, _omitFieldNames ? '' : 'startTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'endTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasurePeriod clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasurePeriod copyWith(void Function(MeasurePeriod) updates) =>
      super.copyWith((message) => updates(message as MeasurePeriod))
          as MeasurePeriod;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MeasurePeriod() / MeasurePeriod.new instead')
  static MeasurePeriod create() => MeasurePeriod._();
  static $pb.GeneratedMessage $_createMessage() => MeasurePeriod._();
  @$core.override
  MeasurePeriod createEmptyInstance() => MeasurePeriod._();
  @$core.pragma('dart2js:noInline')
  static MeasurePeriod getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MeasurePeriod>(
          MeasurePeriod.$_createMessage);
  static MeasurePeriod? _defaultInstance;

  /// 措施開始的時間：停權與立即生效的終止是建立的時間，預告終止（NOTICE）是建立後 30 日，不可變；警告不設。
  @$pb.TagNumber(1)
  $1.Timestamp get startTime => $_getN(0);
  @$pb.TagNumber(1)
  set startTime($1.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStartTime() => $_has(0);
  @$pb.TagNumber(1)
  void clearStartTime() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Timestamp ensureStartTime() => $_ensure(0);

  /// 預定結束的時間；沒有固定期間（由平台定期複審）時不設。管理員可以調整它。終止合作沒有預定結束時間：只在預告終止（NOTICE）的店家關閉時，
  /// 由系統寫入關店的時間（此時 end_time 可能早於 start_time，state 是 ENDED）。
  @$pb.TagNumber(2)
  $1.Timestamp get endTime => $_getN(1);
  @$pb.TagNumber(2)
  set endTime($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasEndTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearEndTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureEndTime() => $_ensure(1);
}

/// MeasureRequest 是措施上賣家的請求（申覆或解除請求，看 kind）。全部欄位由伺服器推導：措施已不在生效中而仍待處理的解除請求，
/// state 是 CLOSED、close_cause 是 ENDED、decide_time 是措施的結束時間，與系統結案之後的值相同。
class MeasureRequest extends $pb.GeneratedMessage {
  factory MeasureRequest({
    $core.String? id,
    MeasureRequestState? state,
    $core.String? reason,
    $1.Timestamp? submitTime,
    $core.String? decisionReason,
    $1.Timestamp? decideTime,
    MeasureRequestCloseCause? closeCause,
    MeasureRequestKind? kind,
  }) {
    final result = MeasureRequest._();
    if (id != null) result.id = id;
    if (state != null) result.state = state;
    if (reason != null) result.reason = reason;
    if (submitTime != null) result.submitTime = submitTime;
    if (decisionReason != null) result.decisionReason = decisionReason;
    if (decideTime != null) result.decideTime = decideTime;
    if (closeCause != null) result.closeCause = closeCause;
    if (kind != null) result.kind = kind;
    return result;
  }

  MeasureRequest._();

  factory MeasureRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasureRequest()..mergeFromBuffer(data, registry);
  factory MeasureRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MeasureRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MeasureRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: MeasureRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<MeasureRequestState>(2, _omitFieldNames ? '' : 'state',
        enumValues: MeasureRequestState.values)
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'submitTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(5, _omitFieldNames ? '' : 'decisionReason')
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'decideTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aE<MeasureRequestCloseCause>(7, _omitFieldNames ? '' : 'closeCause',
        enumValues: MeasureRequestCloseCause.values)
    ..aE<MeasureRequestKind>(8, _omitFieldNames ? '' : 'kind',
        enumValues: MeasureRequestKind.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasureRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MeasureRequest copyWith(void Function(MeasureRequest) updates) =>
      super.copyWith((message) => updates(message as MeasureRequest))
          as MeasureRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MeasureRequest() / MeasureRequest.new instead')
  static MeasureRequest create() => MeasureRequest._();
  static $pb.GeneratedMessage $_createMessage() => MeasureRequest._();
  @$core.override
  MeasureRequest createEmptyInstance() => MeasureRequest._();
  @$core.pragma('dart2js:noInline')
  static MeasureRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MeasureRequest>(
          MeasureRequest.$_createMessage);
  static MeasureRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  MeasureRequestState get state => $_getN(1);
  @$pb.TagNumber(2)
  set state(MeasureRequestState value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get submitTime => $_getN(3);
  @$pb.TagNumber(4)
  set submitTime($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSubmitTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubmitTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureSubmitTime() => $_ensure(3);

  /// 裁定的理由（接受與駁回都有）；待處理或結案時是空字串。
  @$pb.TagNumber(5)
  $core.String get decisionReason => $_getSZ(4);
  @$pb.TagNumber(5)
  set decisionReason($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDecisionReason() => $_has(4);
  @$pb.TagNumber(5)
  void clearDecisionReason() => $_clearField(5);

  /// 裁定或結案的時間；待處理時不設。
  @$pb.TagNumber(6)
  $1.Timestamp get decideTime => $_getN(5);
  @$pb.TagNumber(6)
  set decideTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasDecideTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearDecideTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureDecideTime() => $_ensure(5);

  /// 結案的原因；只有 state 是 CLOSED 時設，其他是 UNSPECIFIED。
  @$pb.TagNumber(7)
  MeasureRequestCloseCause get closeCause => $_getN(6);
  @$pb.TagNumber(7)
  set closeCause(MeasureRequestCloseCause value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCloseCause() => $_has(6);
  @$pb.TagNumber(7)
  void clearCloseCause() => $_clearField(7);

  @$pb.TagNumber(8)
  MeasureRequestKind get kind => $_getN(7);
  @$pb.TagNumber(8)
  set kind(MeasureRequestKind value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasKind() => $_has(7);
  @$pb.TagNumber(8)
  void clearKind() => $_clearField(8);
}

/// Measure 是賣家看到的措施。平台內部的註記不在這裡，賣家只看 statement。
class Measure extends $pb.GeneratedMessage {
  factory Measure({
    $core.String? id,
    MeasureKind? kind,
    MeasureBasis? basis,
    $core.Iterable<MeasureFinding>? findings,
    RiskGround? riskGround,
    $core.String? statement,
    MeasureState? state,
    MeasurePeriod? period,
    $1.Timestamp? createTime,
    $core.String? etag,
    $core.bool? canAppeal,
    RevokeReason? revokeReason,
    $core.bool? canRequestRemediation,
    $1.Timestamp? nextRemediationTime,
    $6.Duration? rejectionCooldown,
    $core.Iterable<MeasureRequest>? requests,
    TerminationGround? terminationGround,
    $1.Timestamp? appealDeadlineTime,
    $core.Iterable<RemediationUnavailability>? remediationUnavailabilities,
    $core.Iterable<AppealUnavailability>? appealUnavailabilities,
    $core.Iterable<ContentRef>? contents,
  }) {
    final result = Measure._();
    if (id != null) result.id = id;
    if (kind != null) result.kind = kind;
    if (basis != null) result.basis = basis;
    if (findings != null) result.findings.addAll(findings);
    if (riskGround != null) result.riskGround = riskGround;
    if (statement != null) result.statement = statement;
    if (state != null) result.state = state;
    if (period != null) result.period = period;
    if (createTime != null) result.createTime = createTime;
    if (etag != null) result.etag = etag;
    if (canAppeal != null) result.canAppeal = canAppeal;
    if (revokeReason != null) result.revokeReason = revokeReason;
    if (canRequestRemediation != null)
      result.canRequestRemediation = canRequestRemediation;
    if (nextRemediationTime != null)
      result.nextRemediationTime = nextRemediationTime;
    if (rejectionCooldown != null) result.rejectionCooldown = rejectionCooldown;
    if (requests != null) result.requests.addAll(requests);
    if (terminationGround != null) result.terminationGround = terminationGround;
    if (appealDeadlineTime != null)
      result.appealDeadlineTime = appealDeadlineTime;
    if (remediationUnavailabilities != null)
      result.remediationUnavailabilities.addAll(remediationUnavailabilities);
    if (appealUnavailabilities != null)
      result.appealUnavailabilities.addAll(appealUnavailabilities);
    if (contents != null) result.contents.addAll(contents);
    return result;
  }

  Measure._();

  factory Measure.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Measure()..mergeFromBuffer(data, registry);
  factory Measure.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Measure()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Measure',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: Measure.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<MeasureKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: MeasureKind.values)
    ..aE<MeasureBasis>(3, _omitFieldNames ? '' : 'basis',
        enumValues: MeasureBasis.values)
    ..pPM<MeasureFinding>(4, _omitFieldNames ? '' : 'findings',
        subBuilder: MeasureFinding.$_createMessage)
    ..aE<RiskGround>(5, _omitFieldNames ? '' : 'riskGround',
        enumValues: RiskGround.values)
    ..aOS(6, _omitFieldNames ? '' : 'statement')
    ..aE<MeasureState>(7, _omitFieldNames ? '' : 'state',
        enumValues: MeasureState.values)
    ..aOM<MeasurePeriod>(8, _omitFieldNames ? '' : 'period',
        subBuilder: MeasurePeriod.$_createMessage)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(11, _omitFieldNames ? '' : 'etag')
    ..aOB(12, _omitFieldNames ? '' : 'canAppeal')
    ..aE<RevokeReason>(13, _omitFieldNames ? '' : 'revokeReason',
        enumValues: RevokeReason.values)
    ..aOB(15, _omitFieldNames ? '' : 'canRequestRemediation')
    ..aOM<$1.Timestamp>(16, _omitFieldNames ? '' : 'nextRemediationTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$6.Duration>(17, _omitFieldNames ? '' : 'rejectionCooldown',
        subBuilder: $6.Duration.$_createMessage)
    ..pPM<MeasureRequest>(18, _omitFieldNames ? '' : 'requests',
        subBuilder: MeasureRequest.$_createMessage)
    ..aE<TerminationGround>(20, _omitFieldNames ? '' : 'terminationGround',
        enumValues: TerminationGround.values)
    ..aOM<$1.Timestamp>(21, _omitFieldNames ? '' : 'appealDeadlineTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..pPM<RemediationUnavailability>(
        23, _omitFieldNames ? '' : 'remediationUnavailabilities',
        subBuilder: RemediationUnavailability.$_createMessage)
    ..pPM<AppealUnavailability>(
        24, _omitFieldNames ? '' : 'appealUnavailabilities',
        subBuilder: AppealUnavailability.$_createMessage)
    ..pPM<ContentRef>(25, _omitFieldNames ? '' : 'contents',
        subBuilder: ContentRef.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Measure clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Measure copyWith(void Function(Measure) updates) =>
      super.copyWith((message) => updates(message as Measure)) as Measure;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Measure() / Measure.new instead')
  static Measure create() => Measure._();
  static $pb.GeneratedMessage $_createMessage() => Measure._();
  @$core.override
  Measure createEmptyInstance() => Measure._();
  @$core.pragma('dart2js:noInline')
  static Measure getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Measure>(Measure.$_createMessage);
  static Measure? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  MeasureKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(MeasureKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  @$pb.TagNumber(3)
  MeasureBasis get basis => $_getN(2);
  @$pb.TagNumber(3)
  set basis(MeasureBasis value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBasis() => $_has(2);
  @$pb.TagNumber(3)
  void clearBasis() => $_clearField(3);

  /// VIOLATION 依據的認定；第一筆是主要依據（建立時由規則決定：有新的認定是新的認定，否則是最嚴重、同嚴重度最新建立的引用認定），其餘依建立時間。
  /// RISK 依據沒有。
  @$pb.TagNumber(4)
  $pb.PbList<MeasureFinding> get findings => $_getList(3);

  /// RISK 依據的風險類型；VIOLATION 依據是 UNSPECIFIED。
  @$pb.TagNumber(5)
  RiskGround get riskGround => $_getN(4);
  @$pb.TagNumber(5)
  set riskGround(RiskGround value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasRiskGround() => $_has(4);
  @$pb.TagNumber(5)
  void clearRiskGround() => $_clearField(5);

  /// 平台給賣家的理由（目前那一版）。
  @$pb.TagNumber(6)
  $core.String get statement => $_getSZ(5);
  @$pb.TagNumber(6)
  set statement($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasStatement() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatement() => $_clearField(6);

  @$pb.TagNumber(7)
  MeasureState get state => $_getN(6);
  @$pb.TagNumber(7)
  set state(MeasureState value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasState() => $_has(6);
  @$pb.TagNumber(7)
  void clearState() => $_clearField(7);

  /// 警告沒有期間，不設。
  @$pb.TagNumber(8)
  MeasurePeriod get period => $_getN(7);
  @$pb.TagNumber(8)
  set period(MeasurePeriod value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasPeriod() => $_has(7);
  @$pb.TagNumber(8)
  void clearPeriod() => $_clearField(8);
  @$pb.TagNumber(8)
  MeasurePeriod ensurePeriod() => $_ensure(7);

  @$pb.TagNumber(10)
  $1.Timestamp get createTime => $_getN(8);
  @$pb.TagNumber(10)
  set createTime($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreateTime() => $_has(8);
  @$pb.TagNumber(10)
  void clearCreateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCreateTime() => $_ensure(8);

  /// 提出申覆時帶的版本；由賣家看得到的版本產生，平台寫內部註記、調整複審期限都不改變它。
  @$pb.TagNumber(11)
  $core.String get etag => $_getSZ(9);
  @$pb.TagNumber(11)
  set etag($core.String value) => $_setString(9, value);
  @$pb.TagNumber(11)
  $core.bool hasEtag() => $_has(9);
  @$pb.TagNumber(11)
  void clearEtag() => $_clearField(11);

  /// 賣家此刻能不能對這筆措施提出申覆：措施未撤銷、還沒申覆過、沒有待處理的請求（措施已結束而仍待處理的解除請求不算，它的 state 已是
  /// CLOSED）、終止合作還在申覆期限內。由伺服器判斷，與 SubmitAppeal 的前置檢查是同一個規則（appeal_unavailabilities 為空），client 不用自己從
  /// state 與 requests 推導。已結束的措施也可以申覆；預定生效（SCHEDULED）的措施可以申覆。
  @$pb.TagNumber(12)
  $core.bool get canAppeal => $_getBF(10);
  @$pb.TagNumber(12)
  set canAppeal($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(12)
  $core.bool hasCanAppeal() => $_has(10);
  @$pb.TagNumber(12)
  void clearCanAppeal() => $_clearField(12);

  /// 已撤銷時的原因；FINDING_REVOKED 表示依據的認定被撤銷。沒有撤銷時是 UNSPECIFIED。
  @$pb.TagNumber(13)
  RevokeReason get revokeReason => $_getN(11);
  @$pb.TagNumber(13)
  set revokeReason(RevokeReason value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasRevokeReason() => $_has(11);
  @$pb.TagNumber(13)
  void clearRevokeReason() => $_clearField(13);

  /// 賣家此刻能不能請求解除：RISK 依據、措施生效中、沒有待處理的請求（申覆或解除請求）、不在冷卻期。由伺服器判斷，與
  /// RequestRemediation 的前置檢查是同一個規則，client 不用自己推導；不能用時的原因看 remediation_unavailabilities（可以用時
  /// 是空的）。
  @$pb.TagNumber(15)
  $core.bool get canRequestRemediation => $_getBF(12);
  @$pb.TagNumber(15)
  set canRequestRemediation($core.bool value) => $_setBool(12, value);
  @$pb.TagNumber(15)
  $core.bool hasCanRequestRemediation() => $_has(12);
  @$pb.TagNumber(15)
  void clearCanRequestRemediation() => $_clearField(15);

  /// 冷卻期的結束時間；只在 RISK 依據、措施生效中而且在冷卻期內設（此刻起不得提出解除請求，到這個時間可以再提出），其餘不設。
  @$pb.TagNumber(16)
  $1.Timestamp get nextRemediationTime => $_getN(13);
  @$pb.TagNumber(16)
  set nextRemediationTime($1.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasNextRemediationTime() => $_has(13);
  @$pb.TagNumber(16)
  void clearNextRemediationTime() => $_clearField(16);
  @$pb.TagNumber(16)
  $1.Timestamp ensureNextRemediationTime() => $_ensure(13);

  /// 這筆措施的下一個請求若被駁回，會開始的冷卻期長度；只在 RISK 依據、措施生效中時才可能設，不設表示不會有冷卻。給送出前的警告用（「若本次經駁回，將於這段期間內不得
  /// 再提出」），client 不用自己從駁回次數推導。冷卻規則由伺服器決定（條款第 17.4 條）。
  @$pb.TagNumber(17)
  $6.Duration get rejectionCooldown => $_getN(14);
  @$pb.TagNumber(17)
  set rejectionCooldown($6.Duration value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasRejectionCooldown() => $_has(14);
  @$pb.TagNumber(17)
  void clearRejectionCooldown() => $_clearField(17);
  @$pb.TagNumber(17)
  $6.Duration ensureRejectionCooldown() => $_ensure(14);

  /// 措施上賣家的請求（申覆與解除請求，看 MeasureRequest.kind），新到舊；沒有請求過時是空的。每一筆的 state 由伺服器推導（見
  /// MeasureRequest）；同時至多一筆 PENDING，所以「目前的請求」是第一筆不是 CLOSED 的。
  @$pb.TagNumber(18)
  $pb.PbList<MeasureRequest> get requests => $_getList(15);

  /// 終止合作（kind 是 TERMINATION）的事由；其他種類是 UNSPECIFIED。
  @$pb.TagNumber(20)
  TerminationGround get terminationGround => $_getN(16);
  @$pb.TagNumber(20)
  set terminationGround(TerminationGround value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasTerminationGround() => $_has(16);
  @$pb.TagNumber(20)
  void clearTerminationGround() => $_clearField(20);

  /// 終止合作的申覆期限：生效（period.start_time）後 184 天（任何連續六個曆月的最大長度，不短於條款的「六個月」）；期限內（含預告期間）可以申覆，
  /// 之後 can_appeal 是 false、appeal_unavailabilities 含 APPEAL_WINDOW_ENDED，已在期限內提出的申覆照常裁定。其他種類不設。
  @$pb.TagNumber(21)
  $1.Timestamp get appealDeadlineTime => $_getN(17);
  @$pb.TagNumber(21)
  set appealDeadlineTime($1.Timestamp value) => $_setField(21, value);
  @$pb.TagNumber(21)
  $core.bool hasAppealDeadlineTime() => $_has(17);
  @$pb.TagNumber(21)
  void clearAppealDeadlineTime() => $_clearField(21);
  @$pb.TagNumber(21)
  $1.Timestamp ensureAppealDeadlineTime() => $_ensure(17);

  /// 不能請求解除的原因，依優先序排列（依據不是 RISK、不在生效中、待處理的請求、冷卻期）；結構性的原因在前，第一個的 reason 是 NOT_RISK 或 NOT_ACTIVE 表示這筆措施沒有解除請求入口（client 不顯示入口）；其餘的原因 client 顯示第一個的 message。
  /// can_request_remediation 是 true 時是空的，反之不為空，兩者來自同一個規則。冷卻期結束的時間另見 next_remediation_time，message 已帶好格式化的時間。
  @$pb.TagNumber(23)
  $pb.PbList<RemediationUnavailability> get remediationUnavailabilities =>
      $_getList(18);

  /// 不能申覆的原因，依優先序排列（已撤銷、已申覆過、申覆期限已過、有審核中的請求）；can_appeal 是 true 時是空的，反之不為空，兩者來自同一個
  /// 規則（與 SubmitAppeal 的前置檢查相同），client 顯示第一個的 message，不自己推導。
  @$pb.TagNumber(24)
  $pb.PbList<AppealUnavailability> get appealUnavailabilities => $_getList(19);

  /// 內容下架（kind 是 CONTENT_TAKEDOWN）的對象，依種類、id 排序，賣家據此知道被下架的是哪些內容、申覆什麼；其他種類沒有。建立後不能改。
  @$pb.TagNumber(25)
  $pb.PbList<ContentRef> get contents => $_getList(20);
}

class ListMyMeasuresRequest extends $pb.GeneratedMessage {
  factory ListMyMeasuresRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    MeasureState? state,
  }) {
    final result = ListMyMeasuresRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (state != null) result.state = state;
    return result;
  }

  ListMyMeasuresRequest._();

  factory ListMyMeasuresRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyMeasuresRequest()..mergeFromBuffer(data, registry);
  factory ListMyMeasuresRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyMeasuresRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyMeasuresRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListMyMeasuresRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aE<MeasureState>(3, _omitFieldNames ? '' : 'state',
        enumValues: MeasureState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyMeasuresRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyMeasuresRequest copyWith(
          void Function(ListMyMeasuresRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyMeasuresRequest))
          as ListMyMeasuresRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyMeasuresRequest() / ListMyMeasuresRequest.new instead')
  static ListMyMeasuresRequest create() => ListMyMeasuresRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyMeasuresRequest._();
  @$core.override
  ListMyMeasuresRequest createEmptyInstance() => ListMyMeasuresRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyMeasuresRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyMeasuresRequest>(
          ListMyMeasuresRequest.$_createMessage);
  static ListMyMeasuresRequest? _defaultInstance;

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

  /// 只列此刻是這個狀態的措施；UNSPECIFIED 不篩。
  @$pb.TagNumber(3)
  MeasureState get state => $_getN(2);
  @$pb.TagNumber(3)
  set state(MeasureState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);
}

class ListMyMeasuresResponse extends $pb.GeneratedMessage {
  factory ListMyMeasuresResponse({
    $core.Iterable<Measure>? measures,
    $core.String? nextPageToken,
  }) {
    final result = ListMyMeasuresResponse._();
    if (measures != null) result.measures.addAll(measures);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyMeasuresResponse._();

  factory ListMyMeasuresResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyMeasuresResponse()..mergeFromBuffer(data, registry);
  factory ListMyMeasuresResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyMeasuresResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyMeasuresResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListMyMeasuresResponse.$_createMessage)
    ..pPM<Measure>(1, _omitFieldNames ? '' : 'measures',
        subBuilder: Measure.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyMeasuresResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyMeasuresResponse copyWith(
          void Function(ListMyMeasuresResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyMeasuresResponse))
          as ListMyMeasuresResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyMeasuresResponse() / ListMyMeasuresResponse.new instead')
  static ListMyMeasuresResponse create() => ListMyMeasuresResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyMeasuresResponse._();
  @$core.override
  ListMyMeasuresResponse createEmptyInstance() => ListMyMeasuresResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyMeasuresResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyMeasuresResponse>(
          ListMyMeasuresResponse.$_createMessage);
  static ListMyMeasuresResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Measure> get measures => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetMyMeasureRequest extends $pb.GeneratedMessage {
  factory GetMyMeasureRequest({
    $core.String? measureId,
  }) {
    final result = GetMyMeasureRequest._();
    if (measureId != null) result.measureId = measureId;
    return result;
  }

  GetMyMeasureRequest._();

  factory GetMyMeasureRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyMeasureRequest()..mergeFromBuffer(data, registry);
  factory GetMyMeasureRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyMeasureRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyMeasureRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyMeasureRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'measureId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyMeasureRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyMeasureRequest copyWith(void Function(GetMyMeasureRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyMeasureRequest))
          as GetMyMeasureRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetMyMeasureRequest() / GetMyMeasureRequest.new instead')
  static GetMyMeasureRequest create() => GetMyMeasureRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyMeasureRequest._();
  @$core.override
  GetMyMeasureRequest createEmptyInstance() => GetMyMeasureRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyMeasureRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyMeasureRequest>(
          GetMyMeasureRequest.$_createMessage);
  static GetMyMeasureRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get measureId => $_getSZ(0);
  @$pb.TagNumber(1)
  set measureId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasureId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasureId() => $_clearField(1);
}

class GetMyMeasureResponse extends $pb.GeneratedMessage {
  factory GetMyMeasureResponse({
    Measure? measure,
  }) {
    final result = GetMyMeasureResponse._();
    if (measure != null) result.measure = measure;
    return result;
  }

  GetMyMeasureResponse._();

  factory GetMyMeasureResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyMeasureResponse()..mergeFromBuffer(data, registry);
  factory GetMyMeasureResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyMeasureResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyMeasureResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetMyMeasureResponse.$_createMessage)
    ..aOM<Measure>(1, _omitFieldNames ? '' : 'measure',
        subBuilder: Measure.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyMeasureResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyMeasureResponse copyWith(void Function(GetMyMeasureResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyMeasureResponse))
          as GetMyMeasureResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetMyMeasureResponse() / GetMyMeasureResponse.new instead')
  static GetMyMeasureResponse create() => GetMyMeasureResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyMeasureResponse._();
  @$core.override
  GetMyMeasureResponse createEmptyInstance() => GetMyMeasureResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyMeasureResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyMeasureResponse>(
          GetMyMeasureResponse.$_createMessage);
  static GetMyMeasureResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Measure get measure => $_getN(0);
  @$pb.TagNumber(1)
  set measure(Measure value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasure() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasure() => $_clearField(1);
  @$pb.TagNumber(1)
  Measure ensureMeasure() => $_ensure(0);
}

class SubmitAppealRequest extends $pb.GeneratedMessage {
  factory SubmitAppealRequest({
    $core.String? measureId,
    $core.String? reason,
    $core.Iterable<$core.String>? evidenceStagedTokens,
    $core.String? etag,
  }) {
    final result = SubmitAppealRequest._();
    if (measureId != null) result.measureId = measureId;
    if (reason != null) result.reason = reason;
    if (evidenceStagedTokens != null)
      result.evidenceStagedTokens.addAll(evidenceStagedTokens);
    if (etag != null) result.etag = etag;
    return result;
  }

  SubmitAppealRequest._();

  factory SubmitAppealRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAppealRequest()..mergeFromBuffer(data, registry);
  factory SubmitAppealRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAppealRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAppealRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitAppealRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'measureId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..pPS(3, _omitFieldNames ? '' : 'evidenceStagedTokens')
    ..aOS(4, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealRequest copyWith(void Function(SubmitAppealRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitAppealRequest))
          as SubmitAppealRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SubmitAppealRequest() / SubmitAppealRequest.new instead')
  static SubmitAppealRequest create() => SubmitAppealRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitAppealRequest._();
  @$core.override
  SubmitAppealRequest createEmptyInstance() => SubmitAppealRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitAppealRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAppealRequest>(
          SubmitAppealRequest.$_createMessage);
  static SubmitAppealRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get measureId => $_getSZ(0);
  @$pb.TagNumber(1)
  set measureId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasureId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasureId() => $_clearField(1);

  /// 申覆理由，最多 2000 字。
  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);

  /// 佐證圖片的 staged token，最多 5 張。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get evidenceStagedTokens => $_getList(2);

  /// 讀到的措施的 etag。
  @$pb.TagNumber(4)
  $core.String get etag => $_getSZ(3);
  @$pb.TagNumber(4)
  set etag($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEtag() => $_has(3);
  @$pb.TagNumber(4)
  void clearEtag() => $_clearField(4);
}

class SubmitAppealResponse extends $pb.GeneratedMessage {
  factory SubmitAppealResponse({
    Measure? measure,
  }) {
    final result = SubmitAppealResponse._();
    if (measure != null) result.measure = measure;
    return result;
  }

  SubmitAppealResponse._();

  factory SubmitAppealResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAppealResponse()..mergeFromBuffer(data, registry);
  factory SubmitAppealResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAppealResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAppealResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: SubmitAppealResponse.$_createMessage)
    ..aOM<Measure>(1, _omitFieldNames ? '' : 'measure',
        subBuilder: Measure.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealResponse copyWith(void Function(SubmitAppealResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitAppealResponse))
          as SubmitAppealResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitAppealResponse() / SubmitAppealResponse.new instead')
  static SubmitAppealResponse create() => SubmitAppealResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubmitAppealResponse._();
  @$core.override
  SubmitAppealResponse createEmptyInstance() => SubmitAppealResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitAppealResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAppealResponse>(
          SubmitAppealResponse.$_createMessage);
  static SubmitAppealResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Measure get measure => $_getN(0);
  @$pb.TagNumber(1)
  set measure(Measure value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasure() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasure() => $_clearField(1);
  @$pb.TagNumber(1)
  Measure ensureMeasure() => $_ensure(0);
}

class RequestRemediationRequest extends $pb.GeneratedMessage {
  factory RequestRemediationRequest({
    $core.String? measureId,
    $core.String? reason,
    $core.Iterable<$core.String>? evidenceStagedTokens,
    $core.String? etag,
  }) {
    final result = RequestRemediationRequest._();
    if (measureId != null) result.measureId = measureId;
    if (reason != null) result.reason = reason;
    if (evidenceStagedTokens != null)
      result.evidenceStagedTokens.addAll(evidenceStagedTokens);
    if (etag != null) result.etag = etag;
    return result;
  }

  RequestRemediationRequest._();

  factory RequestRemediationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RequestRemediationRequest()..mergeFromBuffer(data, registry);
  factory RequestRemediationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RequestRemediationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RequestRemediationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: RequestRemediationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'measureId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..pPS(3, _omitFieldNames ? '' : 'evidenceStagedTokens')
    ..aOS(4, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestRemediationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestRemediationRequest copyWith(
          void Function(RequestRemediationRequest) updates) =>
      super.copyWith((message) => updates(message as RequestRemediationRequest))
          as RequestRemediationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RequestRemediationRequest() / RequestRemediationRequest.new instead')
  static RequestRemediationRequest create() => RequestRemediationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      RequestRemediationRequest._();
  @$core.override
  RequestRemediationRequest createEmptyInstance() =>
      RequestRemediationRequest._();
  @$core.pragma('dart2js:noInline')
  static RequestRemediationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RequestRemediationRequest>(
          RequestRemediationRequest.$_createMessage);
  static RequestRemediationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get measureId => $_getSZ(0);
  @$pb.TagNumber(1)
  set measureId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasureId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasureId() => $_clearField(1);

  /// 解除請求的說明，最多 2000 字。
  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);

  /// 佐證圖片的 staged token，最多 5 張。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get evidenceStagedTokens => $_getList(2);

  /// 讀到的措施的 etag。
  @$pb.TagNumber(4)
  $core.String get etag => $_getSZ(3);
  @$pb.TagNumber(4)
  set etag($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEtag() => $_has(3);
  @$pb.TagNumber(4)
  void clearEtag() => $_clearField(4);
}

class RequestRemediationResponse extends $pb.GeneratedMessage {
  factory RequestRemediationResponse({
    Measure? measure,
  }) {
    final result = RequestRemediationResponse._();
    if (measure != null) result.measure = measure;
    return result;
  }

  RequestRemediationResponse._();

  factory RequestRemediationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RequestRemediationResponse()..mergeFromBuffer(data, registry);
  factory RequestRemediationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RequestRemediationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RequestRemediationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: RequestRemediationResponse.$_createMessage)
    ..aOM<Measure>(1, _omitFieldNames ? '' : 'measure',
        subBuilder: Measure.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestRemediationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestRemediationResponse copyWith(
          void Function(RequestRemediationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RequestRemediationResponse))
          as RequestRemediationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RequestRemediationResponse() / RequestRemediationResponse.new instead')
  static RequestRemediationResponse create() => RequestRemediationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      RequestRemediationResponse._();
  @$core.override
  RequestRemediationResponse createEmptyInstance() =>
      RequestRemediationResponse._();
  @$core.pragma('dart2js:noInline')
  static RequestRemediationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RequestRemediationResponse>(
          RequestRemediationResponse.$_createMessage);
  static RequestRemediationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Measure get measure => $_getN(0);
  @$pb.TagNumber(1)
  set measure(Measure value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMeasure() => $_has(0);
  @$pb.TagNumber(1)
  void clearMeasure() => $_clearField(1);
  @$pb.TagNumber(1)
  Measure ensureMeasure() => $_ensure(0);
}

/// PublicStore 是店面頁上的店家資料。逐欄列出前台需要的欄位：沒有聯絡電話、詳細地址與任何版本欄位。
class PublicStore extends $pb.GeneratedMessage {
  factory PublicStore({
    $core.String? id,
    $core.String? storeCode,
    $core.String? storeName,
    $core.String? description,
    $core.String? townCode,
    $core.String? prize,
    $core.String? websiteUrl,
    $0.Image? logo,
    $0.Image? coverImage,
    $1.Timestamp? updateTime,
  }) {
    final result = PublicStore._();
    if (id != null) result.id = id;
    if (storeCode != null) result.storeCode = storeCode;
    if (storeName != null) result.storeName = storeName;
    if (description != null) result.description = description;
    if (townCode != null) result.townCode = townCode;
    if (prize != null) result.prize = prize;
    if (websiteUrl != null) result.websiteUrl = websiteUrl;
    if (logo != null) result.logo = logo;
    if (coverImage != null) result.coverImage = coverImage;
    if (updateTime != null) result.updateTime = updateTime;
    return result;
  }

  PublicStore._();

  factory PublicStore.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublicStore()..mergeFromBuffer(data, registry);
  factory PublicStore.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublicStore()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublicStore',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: PublicStore.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeCode')
    ..aOS(3, _omitFieldNames ? '' : 'storeName')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'townCode')
    ..aOS(6, _omitFieldNames ? '' : 'prize')
    ..aOS(7, _omitFieldNames ? '' : 'websiteUrl')
    ..aOM<$0.Image>(8, _omitFieldNames ? '' : 'logo',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<$0.Image>(9, _omitFieldNames ? '' : 'coverImage',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<$1.Timestamp>(13, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublicStore clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublicStore copyWith(void Function(PublicStore) updates) =>
      super.copyWith((message) => updates(message as PublicStore))
          as PublicStore;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PublicStore() / PublicStore.new instead')
  static PublicStore create() => PublicStore._();
  static $pb.GeneratedMessage $_createMessage() => PublicStore._();
  @$core.override
  PublicStore createEmptyInstance() => PublicStore._();
  @$core.pragma('dart2js:noInline')
  static PublicStore getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PublicStore>(
          PublicStore.$_createMessage);
  static PublicStore? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 店家網址的路徑段。
  @$pb.TagNumber(2)
  $core.String get storeCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set storeCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoreCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoreCode() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get storeName => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreName() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreName() => $_clearField(3);

  /// 空字串表示沒有填。
  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  /// 內政部的 8 碼鄉鎮市區代碼（縣市是前 5 碼）；空字串表示沒有填。
  @$pb.TagNumber(5)
  $core.String get townCode => $_getSZ(4);
  @$pb.TagNumber(5)
  set townCode($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTownCode() => $_has(4);
  @$pb.TagNumber(5)
  void clearTownCode() => $_clearField(5);

  /// 得獎紀錄；空字串表示沒有填。
  @$pb.TagNumber(6)
  $core.String get prize => $_getSZ(5);
  @$pb.TagNumber(6)
  set prize($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasPrize() => $_has(5);
  @$pb.TagNumber(6)
  void clearPrize() => $_clearField(6);

  /// 官網；空字串表示沒有填。
  @$pb.TagNumber(7)
  $core.String get websiteUrl => $_getSZ(6);
  @$pb.TagNumber(7)
  set websiteUrl($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasWebsiteUrl() => $_has(6);
  @$pb.TagNumber(7)
  void clearWebsiteUrl() => $_clearField(7);

  /// 沒有設定時不設。
  @$pb.TagNumber(8)
  $0.Image get logo => $_getN(7);
  @$pb.TagNumber(8)
  set logo($0.Image value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasLogo() => $_has(7);
  @$pb.TagNumber(8)
  void clearLogo() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Image ensureLogo() => $_ensure(7);

  /// 沒有設定時不設。
  @$pb.TagNumber(9)
  $0.Image get coverImage => $_getN(8);
  @$pb.TagNumber(9)
  set coverImage($0.Image value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCoverImage() => $_has(8);
  @$pb.TagNumber(9)
  void clearCoverImage() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Image ensureCoverImage() => $_ensure(8);

  /// 這家店的公開資料最後更新的時間，給 sitemap 的 lastmod：取店家資料的最後修改時間、第一次可公開曝光的時間與最後一次可曝光狀態翻轉
  /// 的時間中最晚的一個，所以不會早於上架日，停權結束後重新曝光也會前進，
  /// 全量重新公告重發同一個狀態則不會。店家的非公開欄位（例如聯絡資料）改變也會讓它前進，因此偶爾比公開內容的實際變更晚。
  @$pb.TagNumber(13)
  $1.Timestamp get updateTime => $_getN(9);
  @$pb.TagNumber(13)
  set updateTime($1.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasUpdateTime() => $_has(9);
  @$pb.TagNumber(13)
  void clearUpdateTime() => $_clearField(13);
  @$pb.TagNumber(13)
  $1.Timestamp ensureUpdateTime() => $_ensure(9);
}

/// StoreStats 是店面頁的統計。每個欄位都只在讀得到時才設：粉絲與評價各自來自別的模組，讀取失敗不是零。
class StoreStats extends $pb.GeneratedMessage {
  factory StoreStats({
    $core.int? followerCount,
    $core.int? reviewCount,
    $core.double? ratingAverage,
  }) {
    final result = StoreStats._();
    if (followerCount != null) result.followerCount = followerCount;
    if (reviewCount != null) result.reviewCount = reviewCount;
    if (ratingAverage != null) result.ratingAverage = ratingAverage;
    return result;
  }

  StoreStats._();

  factory StoreStats.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreStats()..mergeFromBuffer(data, registry);
  factory StoreStats.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StoreStats()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoreStats',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: StoreStats.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'followerCount')
    ..aI(2, _omitFieldNames ? '' : 'reviewCount')
    ..aD(3, _omitFieldNames ? '' : 'ratingAverage',
        fieldType: $pb.PbFieldType.OF)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreStats clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoreStats copyWith(void Function(StoreStats) updates) =>
      super.copyWith((message) => updates(message as StoreStats)) as StoreStats;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use StoreStats() / StoreStats.new instead')
  static StoreStats create() => StoreStats._();
  static $pb.GeneratedMessage $_createMessage() => StoreStats._();
  @$core.override
  StoreStats createEmptyInstance() => StoreStats._();
  @$core.pragma('dart2js:noInline')
  static StoreStats getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoreStats>(StoreStats.$_createMessage);
  static StoreStats? _defaultInstance;

  /// 粉絲數；0 是確實沒有粉絲，讀取失敗時不設。
  @$pb.TagNumber(1)
  $core.int get followerCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set followerCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFollowerCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearFollowerCount() => $_clearField(1);

  /// 評價數；0 是確實沒有評價，讀取失敗時不設。
  @$pb.TagNumber(2)
  $core.int get reviewCount => $_getIZ(1);
  @$pb.TagNumber(2)
  set reviewCount($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReviewCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearReviewCount() => $_clearField(2);

  /// 平均評分，1 到 5；沒有評價或讀取失敗時不設。
  @$pb.TagNumber(3)
  $core.double get ratingAverage => $_getN(2);
  @$pb.TagNumber(3)
  set ratingAverage($core.double value) => $_setFloat(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRatingAverage() => $_has(2);
  @$pb.TagNumber(3)
  void clearRatingAverage() => $_clearField(3);
}

class GetStoreRequest extends $pb.GeneratedMessage {
  factory GetStoreRequest({
    $core.String? storeCode,
  }) {
    final result = GetStoreRequest._();
    if (storeCode != null) result.storeCode = storeCode;
    return result;
  }

  GetStoreRequest._();

  factory GetStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreRequest()..mergeFromBuffer(data, registry);
  factory GetStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetStoreRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreRequest copyWith(void Function(GetStoreRequest) updates) =>
      super.copyWith((message) => updates(message as GetStoreRequest))
          as GetStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetStoreRequest() / GetStoreRequest.new instead')
  static GetStoreRequest create() => GetStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetStoreRequest._();
  @$core.override
  GetStoreRequest createEmptyInstance() => GetStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStoreRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetStoreRequest>(
          GetStoreRequest.$_createMessage);
  static GetStoreRequest? _defaultInstance;

  /// 店家的 store_code，不是 id。
  @$pb.TagNumber(1)
  $core.String get storeCode => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeCode($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreCode() => $_clearField(1);
}

class GetStoreResponse extends $pb.GeneratedMessage {
  factory GetStoreResponse({
    PublicStore? store,
    StoreStats? stats,
  }) {
    final result = GetStoreResponse._();
    if (store != null) result.store = store;
    if (stats != null) result.stats = stats;
    return result;
  }

  GetStoreResponse._();

  factory GetStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreResponse()..mergeFromBuffer(data, registry);
  factory GetStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetStoreResponse.$_createMessage)
    ..aOM<PublicStore>(1, _omitFieldNames ? '' : 'store',
        subBuilder: PublicStore.$_createMessage)
    ..aOM<StoreStats>(2, _omitFieldNames ? '' : 'stats',
        subBuilder: StoreStats.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreResponse copyWith(void Function(GetStoreResponse) updates) =>
      super.copyWith((message) => updates(message as GetStoreResponse))
          as GetStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetStoreResponse() / GetStoreResponse.new instead')
  static GetStoreResponse create() => GetStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetStoreResponse._();
  @$core.override
  GetStoreResponse createEmptyInstance() => GetStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStoreResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetStoreResponse>(
          GetStoreResponse.$_createMessage);
  static GetStoreResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PublicStore get store => $_getN(0);
  @$pb.TagNumber(1)
  set store(PublicStore value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStore() => $_has(0);
  @$pb.TagNumber(1)
  void clearStore() => $_clearField(1);
  @$pb.TagNumber(1)
  PublicStore ensureStore() => $_ensure(0);

  @$pb.TagNumber(2)
  StoreStats get stats => $_getN(1);
  @$pb.TagNumber(2)
  set stats(StoreStats value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStats() => $_has(1);
  @$pb.TagNumber(2)
  void clearStats() => $_clearField(2);
  @$pb.TagNumber(2)
  StoreStats ensureStats() => $_ensure(1);
}

class ListStoresRequest extends $pb.GeneratedMessage {
  factory ListStoresRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListStoresRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListStoresRequest._();

  factory ListStoresRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoresRequest()..mergeFromBuffer(data, registry);
  factory ListStoresRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoresRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoresRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListStoresRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoresRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoresRequest copyWith(void Function(ListStoresRequest) updates) =>
      super.copyWith((message) => updates(message as ListStoresRequest))
          as ListStoresRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListStoresRequest() / ListStoresRequest.new instead')
  static ListStoresRequest create() => ListStoresRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListStoresRequest._();
  @$core.override
  ListStoresRequest createEmptyInstance() => ListStoresRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStoresRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListStoresRequest>(
          ListStoresRequest.$_createMessage);
  static ListStoresRequest? _defaultInstance;

  /// 每頁幾家，預設 100，上限 1000；超過以上限計，負數回 invalid_argument。
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

class ListStoresResponse extends $pb.GeneratedMessage {
  factory ListStoresResponse({
    $core.Iterable<PublicStore>? stores,
    $core.String? nextPageToken,
  }) {
    final result = ListStoresResponse._();
    if (stores != null) result.stores.addAll(stores);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStoresResponse._();

  factory ListStoresResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoresResponse()..mergeFromBuffer(data, registry);
  factory ListStoresResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoresResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoresResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: ListStoresResponse.$_createMessage)
    ..pPM<PublicStore>(1, _omitFieldNames ? '' : 'stores',
        subBuilder: PublicStore.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoresResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoresResponse copyWith(void Function(ListStoresResponse) updates) =>
      super.copyWith((message) => updates(message as ListStoresResponse))
          as ListStoresResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListStoresResponse() / ListStoresResponse.new instead')
  static ListStoresResponse create() => ListStoresResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListStoresResponse._();
  @$core.override
  ListStoresResponse createEmptyInstance() => ListStoresResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStoresResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoresResponse>(
          ListStoresResponse.$_createMessage);
  static ListStoresResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PublicStore> get stores => $_getList(0);

  /// 空字串表示最後一頁。
  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class BatchGetStoresRequest extends $pb.GeneratedMessage {
  factory BatchGetStoresRequest({
    $core.Iterable<$core.String>? ids,
  }) {
    final result = BatchGetStoresRequest._();
    if (ids != null) result.ids.addAll(ids);
    return result;
  }

  BatchGetStoresRequest._();

  factory BatchGetStoresRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchGetStoresRequest()..mergeFromBuffer(data, registry);
  factory BatchGetStoresRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchGetStoresRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchGetStoresRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: BatchGetStoresRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'ids')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchGetStoresRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchGetStoresRequest copyWith(
          void Function(BatchGetStoresRequest) updates) =>
      super.copyWith((message) => updates(message as BatchGetStoresRequest))
          as BatchGetStoresRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchGetStoresRequest() / BatchGetStoresRequest.new instead')
  static BatchGetStoresRequest create() => BatchGetStoresRequest._();
  static $pb.GeneratedMessage $_createMessage() => BatchGetStoresRequest._();
  @$core.override
  BatchGetStoresRequest createEmptyInstance() => BatchGetStoresRequest._();
  @$core.pragma('dart2js:noInline')
  static BatchGetStoresRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchGetStoresRequest>(
          BatchGetStoresRequest.$_createMessage);
  static BatchGetStoresRequest? _defaultInstance;

  /// 店家 id，1 到 100 個。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get ids => $_getList(0);
}

class BatchGetStoresResponse extends $pb.GeneratedMessage {
  factory BatchGetStoresResponse({
    $core.Iterable<$7.StoreCard>? stores,
  }) {
    final result = BatchGetStoresResponse._();
    if (stores != null) result.stores.addAll(stores);
    return result;
  }

  BatchGetStoresResponse._();

  factory BatchGetStoresResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchGetStoresResponse()..mergeFromBuffer(data, registry);
  factory BatchGetStoresResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchGetStoresResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchGetStoresResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: BatchGetStoresResponse.$_createMessage)
    ..pPM<$7.StoreCard>(1, _omitFieldNames ? '' : 'stores',
        subBuilder: $7.StoreCard.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchGetStoresResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchGetStoresResponse copyWith(
          void Function(BatchGetStoresResponse) updates) =>
      super.copyWith((message) => updates(message as BatchGetStoresResponse))
          as BatchGetStoresResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchGetStoresResponse() / BatchGetStoresResponse.new instead')
  static BatchGetStoresResponse create() => BatchGetStoresResponse._();
  static $pb.GeneratedMessage $_createMessage() => BatchGetStoresResponse._();
  @$core.override
  BatchGetStoresResponse createEmptyInstance() => BatchGetStoresResponse._();
  @$core.pragma('dart2js:noInline')
  static BatchGetStoresResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchGetStoresResponse>(
          BatchGetStoresResponse.$_createMessage);
  static BatchGetStoresResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$7.StoreCard> get stores => $_getList(0);
}

class GetSellerAgreementsRequest extends $pb.GeneratedMessage {
  factory GetSellerAgreementsRequest() => GetSellerAgreementsRequest._();

  GetSellerAgreementsRequest._();

  factory GetSellerAgreementsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerAgreementsRequest()..mergeFromBuffer(data, registry);
  factory GetSellerAgreementsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerAgreementsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSellerAgreementsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetSellerAgreementsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerAgreementsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerAgreementsRequest copyWith(
          void Function(GetSellerAgreementsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetSellerAgreementsRequest))
          as GetSellerAgreementsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetSellerAgreementsRequest() / GetSellerAgreementsRequest.new instead')
  static GetSellerAgreementsRequest create() => GetSellerAgreementsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetSellerAgreementsRequest._();
  @$core.override
  GetSellerAgreementsRequest createEmptyInstance() =>
      GetSellerAgreementsRequest._();
  @$core.pragma('dart2js:noInline')
  static GetSellerAgreementsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSellerAgreementsRequest>(
          GetSellerAgreementsRequest.$_createMessage);
  static GetSellerAgreementsRequest? _defaultInstance;
}

class GetSellerAgreementsResponse extends $pb.GeneratedMessage {
  factory GetSellerAgreementsResponse({
    $core.Iterable<SellerAgreement>? agreements,
  }) {
    final result = GetSellerAgreementsResponse._();
    if (agreements != null) result.agreements.addAll(agreements);
    return result;
  }

  GetSellerAgreementsResponse._();

  factory GetSellerAgreementsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerAgreementsResponse()..mergeFromBuffer(data, registry);
  factory GetSellerAgreementsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerAgreementsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSellerAgreementsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: GetSellerAgreementsResponse.$_createMessage)
    ..pPM<SellerAgreement>(1, _omitFieldNames ? '' : 'agreements',
        subBuilder: SellerAgreement.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerAgreementsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerAgreementsResponse copyWith(
          void Function(GetSellerAgreementsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetSellerAgreementsResponse))
          as GetSellerAgreementsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetSellerAgreementsResponse() / GetSellerAgreementsResponse.new instead')
  static GetSellerAgreementsResponse create() =>
      GetSellerAgreementsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetSellerAgreementsResponse._();
  @$core.override
  GetSellerAgreementsResponse createEmptyInstance() =>
      GetSellerAgreementsResponse._();
  @$core.pragma('dart2js:noInline')
  static GetSellerAgreementsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSellerAgreementsResponse>(
          GetSellerAgreementsResponse.$_createMessage);
  static GetSellerAgreementsResponse? _defaultInstance;

  /// 全部文件的現行版本，依 AgreementKind 的數字順序。
  @$pb.TagNumber(1)
  $pb.PbList<SellerAgreement> get agreements => $_getList(0);
}

class RecordProductViewRequest extends $pb.GeneratedMessage {
  factory RecordProductViewRequest({
    $core.String? productId,
  }) {
    final result = RecordProductViewRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  RecordProductViewRequest._();

  factory RecordProductViewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordProductViewRequest()..mergeFromBuffer(data, registry);
  factory RecordProductViewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordProductViewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordProductViewRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: RecordProductViewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordProductViewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordProductViewRequest copyWith(
          void Function(RecordProductViewRequest) updates) =>
      super.copyWith((message) => updates(message as RecordProductViewRequest))
          as RecordProductViewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RecordProductViewRequest() / RecordProductViewRequest.new instead')
  static RecordProductViewRequest create() => RecordProductViewRequest._();
  static $pb.GeneratedMessage $_createMessage() => RecordProductViewRequest._();
  @$core.override
  RecordProductViewRequest createEmptyInstance() =>
      RecordProductViewRequest._();
  @$core.pragma('dart2js:noInline')
  static RecordProductViewRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordProductViewRequest>(
          RecordProductViewRequest.$_createMessage);
  static RecordProductViewRequest? _defaultInstance;

  /// 被瀏覽的商品 id。
  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class RecordProductViewResponse extends $pb.GeneratedMessage {
  factory RecordProductViewResponse() => RecordProductViewResponse._();

  RecordProductViewResponse._();

  factory RecordProductViewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordProductViewResponse()..mergeFromBuffer(data, registry);
  factory RecordProductViewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordProductViewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordProductViewResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.seller.v1alpha1'),
      createEmptyInstance: RecordProductViewResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordProductViewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordProductViewResponse copyWith(
          void Function(RecordProductViewResponse) updates) =>
      super.copyWith((message) => updates(message as RecordProductViewResponse))
          as RecordProductViewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RecordProductViewResponse() / RecordProductViewResponse.new instead')
  static RecordProductViewResponse create() => RecordProductViewResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      RecordProductViewResponse._();
  @$core.override
  RecordProductViewResponse createEmptyInstance() =>
      RecordProductViewResponse._();
  @$core.pragma('dart2js:noInline')
  static RecordProductViewResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordProductViewResponse>(
          RecordProductViewResponse.$_createMessage);
  static RecordProductViewResponse? _defaultInstance;
}

/// SellerAppService 是登入的帳號的方法。除了 ApplyStore 與措施、申覆的方法之外，都以呼叫者目前的店家操作，店家是
/// 呼叫者的現任店家，不接受指定；沒有店家時回 not_found。措施與申覆（ListMyMeasures、GetMyMeasure、SubmitAppeal）
/// 以帳號為範圍：措施的效果以帳號為單位，所有店家都已關閉的帳號照樣受處分、必須還能申覆，所以這三個方法不要求賣家角色
/// 或店家。每個方法需要的賣家能力（SellerCapability）宣告在 internal/httpapi/seller，賣家停權時不在白名單內的方法回
/// permission_denied，ErrorInfo.reason 是 SELLER_SUSPENDED；其他前置條件沒有滿足（例如身分認證未核准、店家正在隨帳號刪除）
/// 回 failed_precondition，ErrorInfo.reason 是 SELLER_REQUIREMENT_MISSING，metadata 帶缺的條件。
class SellerAppServiceApi {
  final $pb.RpcClient _client;

  SellerAppServiceApi(this._client);

  /// ApplyStore 申請開店：建立店家並授予賣家角色。前置條件依優先序檢查，第一個不符的決定回應：有生效中的終止合作回
  /// failed_precondition 與 STORE_OPENING_TERMINATED（解除停權也開不了店）；其次帳號停權中回 failed_precondition 與
  /// STORE_OPENING_SUSPENDED；其次帳號正在刪除回 failed_precondition（沒有 reason）；最後帳號已經有店家才回 already_exists。
  /// 所以被終止、停權或正在刪除的帳號即使還有店家，回的也不是 already_exists；任何一個欄位不合法都不會建立店家。官網、logo 與封面開店後以 UpdateMyStore 設定。
  $async.Future<ApplyStoreResponse> applyStore(
          $pb.ClientContext? ctx, ApplyStoreRequest request) =>
      _client.invoke<ApplyStoreResponse>(
          ctx, 'SellerAppService', 'ApplyStore', request, ApplyStoreResponse());

  /// GetMyStoreOpening 回答呼叫者的帳號此刻能不能申請開店，以及不能的原因（依優先序）。開店入口在送出表單之前讀它，不必等 ApplyStore 失敗才知道。
  /// 與 ApplyStore 的前置條件是同一份規則（伺服器推導）：can_apply_store 為 true 時 ApplyStore 不會被前置條件擋下；ApplyStore 的 ErrorReason
  /// （STORE_OPENING_SUSPENDED、STORE_OPENING_TERMINATED）仍是兩次呼叫之間狀態改變時的後備。不要求賣家角色，帳號還沒有店家也可以呼叫。
  $async.Future<GetMyStoreOpeningResponse> getMyStoreOpening(
          $pb.ClientContext? ctx, GetMyStoreOpeningRequest request) =>
      _client.invoke<GetMyStoreOpeningResponse>(ctx, 'SellerAppService',
          'GetMyStoreOpening', request, GetMyStoreOpeningResponse());

  /// GetMyStore 回傳呼叫者的店家，連同停權與每個能力的狀態（含缺的條件）。停權與等待刪除的店家照樣回傳。
  $async.Future<GetMyStoreResponse> getMyStore(
          $pb.ClientContext? ctx, GetMyStoreRequest request) =>
      _client.invoke<GetMyStoreResponse>(
          ctx, 'SellerAppService', 'GetMyStore', request, GetMyStoreResponse());

  /// UpdateMyStore 修改店家資料的欄位（update_mask），含 logo 與封面（logo_staged_token、
  /// cover_staged_token）。換圖與文字欄位在同一個版本條件下一起寫入，任何一項失敗都不會寫入；沒有任何欄位改變時
  /// 不寫入，etag 不變。
  $async.Future<UpdateMyStoreResponse> updateMyStore(
          $pb.ClientContext? ctx, UpdateMyStoreRequest request) =>
      _client.invoke<UpdateMyStoreResponse>(ctx, 'SellerAppService',
          'UpdateMyStore', request, UpdateMyStoreResponse());

  /// CompleteMyStoreOnboarding 標記新手引導完成；已經完成時不改變任何東西（etag 仍要相符）。
  $async.Future<CompleteMyStoreOnboardingResponse> completeMyStoreOnboarding(
          $pb.ClientContext? ctx, CompleteMyStoreOnboardingRequest request) =>
      _client.invoke<CompleteMyStoreOnboardingResponse>(
          ctx,
          'SellerAppService',
          'CompleteMyStoreOnboarding',
          request,
          CompleteMyStoreOnboardingResponse());

  /// ListMyFollowers 列出追蹤呼叫者店家的買家，新到舊。
  $async.Future<ListMyFollowersResponse> listMyFollowers(
          $pb.ClientContext? ctx, ListMyFollowersRequest request) =>
      _client.invoke<ListMyFollowersResponse>(ctx, 'SellerAppService',
          'ListMyFollowers', request, ListMyFollowersResponse());

  /// GetMyDashboard 回傳呼叫者店家的首頁儀表板：最近 7 個完整的台北日曆日（不含今天）與前一個 7 日的銷售、粉絲、商品
  /// 瀏覽，加上訂單狀態的目前快照。數字都是原始值，不計算成長率（client 自己比較 current 與 previous）。四個區塊來自
  /// 不同的模組，各自讀取、各自逾時：讀不到的區塊不設，不會是 0，其他區塊照樣回傳，方法本身不因為單一區塊失敗而失敗。
  /// 需要行銷經營的能力（CAP_MARKETING），所以停權或等待刪除的店家回 permission_denied（SELLER_SUSPENDED）或
  /// failed_precondition。
  $async.Future<GetMyDashboardResponse> getMyDashboard(
          $pb.ClientContext? ctx, GetMyDashboardRequest request) =>
      _client.invoke<GetMyDashboardResponse>(ctx, 'SellerAppService',
          'GetMyDashboard', request, GetMyDashboardResponse());

  /// CloseMyStore 關閉店家（帳號與買家身分留著，賣家角色移除）。店家有未結訂單時回 failed_precondition；
  /// 停權中也可以關店；正在隨帳號刪除的店家由刪除流程關閉，回 failed_precondition。
  $async.Future<CloseMyStoreResponse> closeMyStore(
          $pb.ClientContext? ctx, CloseMyStoreRequest request) =>
      _client.invoke<CloseMyStoreResponse>(ctx, 'SellerAppService',
          'CloseMyStore', request, CloseMyStoreResponse());

  /// SaveKYCDraft 部分更新呼叫者店家的 KYC 草稿（update_mask）。申請是店家的單例資源（沒有 Create，也沒有申請 ID）：
  /// 店家還沒有申請時，不帶 etag 的呼叫建立它（subject 必須設定其中一支）；已經有申請時 etag 必填，沒帶 etag 回
  /// already_exists、與目前的版本不符回 aborted，兩者的 ErrorInfo.reason 都是 VERSION_CONFLICT（client 的處理相同：
  /// 保留輸入、顯示最新版本）。被退回的申請在這裡回到草稿重新編輯。全部欄位先驗證才寫入，任何一欄不合法都不會寫入。
  /// 送出（SubmitKYC）才檢查必填欄位與證件是否齊全。流程狀態 can_resubmit 為 false 時回 failed_precondition，訊息是
  /// KYCFlow.resubmit_unavailabilities 的第一句。
  $async.Future<SaveKYCDraftResponse> saveKYCDraft(
          $pb.ClientContext? ctx, SaveKYCDraftRequest request) =>
      _client.invoke<SaveKYCDraftResponse>(ctx, 'SellerAppService',
          'SaveKYCDraft', request, SaveKYCDraftResponse());

  /// GetMyKYC 回傳呼叫者店家目前的 KYC 申請（身分證號與銀行帳號只有遮蔽後的值）與流程狀態。還沒有申請（或申請已取消）
  /// 時回 not_found。
  $async.Future<GetMyKYCResponse> getMyKYC(
          $pb.ClientContext? ctx, GetMyKYCRequest request) =>
      _client.invoke<GetMyKYCResponse>(
          ctx, 'SellerAppService', 'GetMyKYC', request, GetMyKYCResponse());

  /// SubmitKYC 送出申請給管理員審核。草稿，以及流程狀態 can_resubmit 為 true 的申請（被退回的、PayUni 拒絕或綁定失敗而且賣家能自己修正的）都能直接送出，
  /// 不必先儲存；can_resubmit 為 false 的狀態回 failed_precondition，訊息是 KYCFlow.resubmit_unavailabilities 的第一句。必填欄位與證件齊全、身分沒有命中別的帳號生效中的處分。缺少的欄位回
  /// invalid_argument，BadRequest 的每個 field 是 KYCApplication 的欄位路徑（real_name、id_card_front、business.tax_id……，與 KYCField 一對一）。申請本身齊全之後，
  /// PayUni 綁定需要的店家資料（地址、聯絡人姓名與手機、客服電子信箱）還有缺漏時回 failed_precondition（店家的狀態不在請求裡），
  /// client 讀 GetMyKYC 的 missing_store_fields 取得缺哪些，以 UpdateMyStore 補齊。 身分比對沒有通過時回
  /// failed_precondition，訊息固定，不透露是哪個欄位或哪個帳號。每個帳號每小時最多送出 5 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  $async.Future<SubmitKYCResponse> submitKYC(
          $pb.ClientContext? ctx, SubmitKYCRequest request) =>
      _client.invoke<SubmitKYCResponse>(
          ctx, 'SellerAppService', 'SubmitKYC', request, SubmitKYCResponse());

  /// CancelKYC 取消草稿或審核中的申請；取消之後可以重新建立。
  $async.Future<CancelKYCResponse> cancelKYC(
          $pb.ClientContext? ctx, CancelKYCRequest request) =>
      _client.invoke<CancelKYCResponse>(
          ctx, 'SellerAppService', 'CancelKYC', request, CancelKYCResponse());

  /// LookupBusinessRegistration 以統一編號查經濟部商業司的公開登記資料，給 KYC 表單帶入公司資料。查無這個統編不是錯誤：
  /// 回應沒有 registration；資料來源暫時無法使用回 unavailable，畫面退回手動填寫。每個帳號每分鐘最多查 10 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  $async.Future<LookupBusinessRegistrationResponse> lookupBusinessRegistration(
          $pb.ClientContext? ctx, LookupBusinessRegistrationRequest request) =>
      _client.invoke<LookupBusinessRegistrationResponse>(
          ctx,
          'SellerAppService',
          'LookupBusinessRegistration',
          request,
          LookupBusinessRegistrationResponse());

  /// ListMyMeasures 列出呼叫者帳號所有店家上的措施（警告與停權），新到舊。以帳號為範圍，不要求賣家角色。state 篩選措施此刻的
  /// 狀態（例如 ACTIVE 是生效中的停權，給停權公告用）；UNSPECIFIED 不篩。
  $async.Future<ListMyMeasuresResponse> listMyMeasures(
          $pb.ClientContext? ctx, ListMyMeasuresRequest request) =>
      _client.invoke<ListMyMeasuresResponse>(ctx, 'SellerAppService',
          'ListMyMeasures', request, ListMyMeasuresResponse());

  /// GetMyMeasure 回傳呼叫者帳號的一筆措施。不屬於這個帳號的措施與不存在的措施都回 not_found。
  $async.Future<GetMyMeasureResponse> getMyMeasure(
          $pb.ClientContext? ctx, GetMyMeasureRequest request) =>
      _client.invoke<GetMyMeasureResponse>(ctx, 'SellerAppService',
          'GetMyMeasure', request, GetMyMeasureResponse());

  /// SubmitAppeal 對自己的措施提出申覆，每筆措施一次。etag 必填（措施的內容在申覆之前被管理員改過時，賣家要看到新的內容再決定）。
  /// etag 不符（管理員在讀取之後改過內容）回 aborted（VERSION_CONFLICT），不寫入。提出申覆不改變 etag，所以申覆已經生效後
  /// 再送一次（包含重送同一個請求）回 failed_precondition，不寫入；措施已經撤銷時也回 failed_precondition。已結束的措施也
  /// 可以申覆；措施已結束而仍有待處理的解除請求時，解除請求在同一個 transaction 以 CLOSED（ENDED）結案後再提出申覆。終止合作到 Measure.appeal_deadline_time
  /// 為止可以申覆（含預告期間，措施還是 SCHEDULED 時），期限已過回 failed_precondition（Measure.appeal_unavailabilities 含
  /// APPEAL_WINDOW_ENDED）。不要求賣家角色。
  $async.Future<SubmitAppealResponse> submitAppeal(
          $pb.ClientContext? ctx, SubmitAppealRequest request) =>
      _client.invoke<SubmitAppealResponse>(ctx, 'SellerAppService',
          'SubmitAppeal', request, SubmitAppealResponse());

  /// RequestRemediation 對依據風險類型（RISK，含條款第 17.5 條的外部要求）的措施，在風險消除或外部要求解除後檢具事證請求解除
  /// （條款第 17.4 條）。可以重複提出，但同一措施同時只能有一筆待處理的請求（申覆與解除請求合計），累計被駁回的請求多了會有冷卻期
  /// （Measure.next_remediation_time）；VIOLATION 依據的措施沒有這個入口。etag 必填，規則同 SubmitAppeal：不符回 aborted
  /// （VERSION_CONFLICT）；提出請求不改變 etag，所以請求已經生效後再送一次（包含重送同一個請求）回 failed_precondition，不寫入。
  /// 措施不是生效中、不是 RISK 依據、在冷卻期內、已有待處理的請求，或已撤銷時回 failed_precondition。不要求賣家角色。
  $async.Future<RequestRemediationResponse> requestRemediation(
          $pb.ClientContext? ctx, RequestRemediationRequest request) =>
      _client.invoke<RequestRemediationResponse>(ctx, 'SellerAppService',
          'RequestRemediation', request, RequestRemediationResponse());
}

/// SellerPublicService 是任何人都能讀的店家資料，不需要登入（ADR 2026-10-03-public-store-identity-and-visibility）。
/// 公開的店家以 store_code 識別、以 id 參照：店面頁網址用 store_code（亂數、不可列舉），各處的店家卡片用 id；
/// 兩個方法都只接受自己的鍵，不接受另一個。
class SellerPublicServiceApi {
  final $pb.RpcClient _client;

  SellerPublicServiceApi(this._client);

  /// GetStore 以 store_code 讀取一家店的店面，連同粉絲數與評價統計。只回可以公開曝光的店家（營運中、沒有停權、身分認證
  /// 通過、店名與 logo 與所在地齊全）；店家不存在與不公開都回 not_found，兩者的回應相同。賣家預覽自己還沒公開的店面讀
  /// SellerAppService.GetMyStore。粉絲數與評價統計來自別的模組：讀取失敗時 stats 的對應欄位不設，不會是 0（見 StoreStats），
  /// 店面本身照樣回傳。
  $async.Future<GetStoreResponse> getStore(
          $pb.ClientContext? ctx, GetStoreRequest request) =>
      _client.invoke<GetStoreResponse>(
          ctx, 'SellerPublicService', 'GetStore', request, GetStoreResponse());

  /// ListStores 列出可以公開曝光的店家，依 id 由小到大，給 sitemap 與之後的店家目錄使用。清單依最後一次公告的可曝光狀態
  /// 產生，每一頁再以 GetStore 的規則複核：店家剛變得可曝光，要等公告之後才出現；公告落後現況而此刻不可曝光的店不會列出。
  /// 一頁可以少於 page_size（甚至是空的），結束只看 next_page_token 是不是空字串，不可用「少於 page_size」判斷；翻頁途中
  /// 新增或失去可曝光的店不會讓已讀過的店重複出現。沒有可曝光的店時回空清單。
  $async.Future<ListStoresResponse> listStores(
          $pb.ClientContext? ctx, ListStoresRequest request) =>
      _client.invoke<ListStoresResponse>(ctx, 'SellerPublicService',
          'ListStores', request, ListStoresResponse());

  /// BatchGetStores 以 id 批次讀取店家卡片，給收藏、追蹤、聊天、訂單、貼文這類只需要店名與 logo 的地方。曾經公開過的
  /// 店家都回傳，包括停權與已關閉的（status 區分，個資清除之後 name 不設）；不存在與從沒公開過的店家不在回應裡。
  /// 回應依請求的順序，重複的 id 只回一次。一次最多 100 個 id。這是刻意的部分結果（不是 AIP-231 的整批失敗）：找不到與
  /// 從沒公開過的 id 直接略過，不回錯誤，client 以回應裡有沒有該 id 判斷（ADR 2026-10-03-public-store-identity-and-visibility）。
  $async.Future<BatchGetStoresResponse> batchGetStores(
          $pb.ClientContext? ctx, BatchGetStoresRequest request) =>
      _client.invoke<BatchGetStoresResponse>(ctx, 'SellerPublicService',
          'BatchGetStores', request, BatchGetStoresResponse());

  /// GetSellerAgreements 讀取賣家入駐時要同意的文件的現行版本與網址，全部種類都回，依 AgreementKind 的數字順序。內容是公開的，
  /// 開店前就要能讀，所以不需要登入。client 把這裡的版本顯示給賣家勾選，再原樣帶回 ApplyStore 與 SubmitKYC 的
  /// accepted_agreements。
  $async.Future<GetSellerAgreementsResponse> getSellerAgreements(
          $pb.ClientContext? ctx, GetSellerAgreementsRequest request) =>
      _client.invoke<GetSellerAgreementsResponse>(ctx, 'SellerPublicService',
          'GetSellerAgreements', request, GetSellerAgreementsResponse());

  /// RecordProductView 記錄一次商品頁的瀏覽，只計訪客：登入的買家由買家的瀏覽紀錄計入，登入的呼叫者收到成功的回應但
  /// 不計數（不會重複計算，也不必讓 client 判斷登入狀態）。商品所屬的店家由背景程序解析，不存在的商品不回錯。
  /// 每個來源 IP 每分鐘最多 120 次，超過回 resource_exhausted（帶 RetryInfo）。
  $async.Future<RecordProductViewResponse> recordProductView(
          $pb.ClientContext? ctx, RecordProductViewRequest request) =>
      _client.invoke<RecordProductViewResponse>(ctx, 'SellerPublicService',
          'RecordProductView', request, RecordProductViewResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
