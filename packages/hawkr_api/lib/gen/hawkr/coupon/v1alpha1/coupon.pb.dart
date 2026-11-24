// This is a generated file - do not edit.
//
// Generated from hawkr/coupon/v1alpha1/coupon.proto.

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
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/store_card.pb.dart' as $0;
import 'coupon.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'coupon.pbenum.dart';

/// CashDiscount 是現金折抵。
class CashDiscount extends $pb.GeneratedMessage {
  factory CashDiscount({
    $fixnum.Int64? amount,
  }) {
    final result = CashDiscount._();
    if (amount != null) result.amount = amount;
    return result;
  }

  CashDiscount._();

  factory CashDiscount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CashDiscount()..mergeFromBuffer(data, registry);
  factory CashDiscount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CashDiscount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CashDiscount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: CashDiscount.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'amount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CashDiscount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CashDiscount copyWith(void Function(CashDiscount) updates) =>
      super.copyWith((message) => updates(message as CashDiscount))
          as CashDiscount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CashDiscount() / CashDiscount.new instead')
  static CashDiscount create() => CashDiscount._();
  static $pb.GeneratedMessage $_createMessage() => CashDiscount._();
  @$core.override
  CashDiscount createEmptyInstance() => CashDiscount._();
  @$core.pragma('dart2js:noInline')
  static CashDiscount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CashDiscount>(
          CashDiscount.$_createMessage);
  static CashDiscount? _defaultInstance;

  /// 折抵金額，至少 1。
  @$pb.TagNumber(1)
  $fixnum.Int64 get amount => $_getI64(0);
  @$pb.TagNumber(1)
  set amount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAmount() => $_has(0);
  @$pb.TagNumber(1)
  void clearAmount() => $_clearField(1);
}

/// PercentDiscount 是百分比折抵：訂單金額乘以 percent 除以 100（整數截斷），不超過 max_amount。
class PercentDiscount extends $pb.GeneratedMessage {
  factory PercentDiscount({
    $core.int? percent,
    $fixnum.Int64? maxAmount,
  }) {
    final result = PercentDiscount._();
    if (percent != null) result.percent = percent;
    if (maxAmount != null) result.maxAmount = maxAmount;
    return result;
  }

  PercentDiscount._();

  factory PercentDiscount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PercentDiscount()..mergeFromBuffer(data, registry);
  factory PercentDiscount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PercentDiscount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PercentDiscount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: PercentDiscount.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'percent')
    ..aInt64(2, _omitFieldNames ? '' : 'maxAmount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PercentDiscount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PercentDiscount copyWith(void Function(PercentDiscount) updates) =>
      super.copyWith((message) => updates(message as PercentDiscount))
          as PercentDiscount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PercentDiscount() / PercentDiscount.new instead')
  static PercentDiscount create() => PercentDiscount._();
  static $pb.GeneratedMessage $_createMessage() => PercentDiscount._();
  @$core.override
  PercentDiscount createEmptyInstance() => PercentDiscount._();
  @$core.pragma('dart2js:noInline')
  static PercentDiscount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PercentDiscount>(
          PercentDiscount.$_createMessage);
  static PercentDiscount? _defaultInstance;

  /// 1 到 99。
  @$pb.TagNumber(1)
  $core.int get percent => $_getIZ(0);
  @$pb.TagNumber(1)
  set percent($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPercent() => $_has(0);
  @$pb.TagNumber(1)
  void clearPercent() => $_clearField(1);

  /// 折抵上限，至少 1。
  @$pb.TagNumber(2)
  $fixnum.Int64 get maxAmount => $_getI64(1);
  @$pb.TagNumber(2)
  set maxAmount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMaxAmount() => $_has(1);
  @$pb.TagNumber(2)
  void clearMaxAmount() => $_clearField(2);
}

enum Campaign_Discount { cash, percent, notSet }

/// Campaign 是活動的公開部分：買家與店家都看得到的規則。
class Campaign extends $pb.GeneratedMessage {
  factory Campaign({
    $core.String? id,
    $core.String? title,
    CashDiscount? cash,
    PercentDiscount? percent,
    $fixnum.Int64? thresholdAmount,
    TargetType? targetType,
    $0.StoreCard? store,
    $core.String? storeId,
    $1.Timestamp? startTime,
    $1.Timestamp? endTime,
  }) {
    final result = Campaign._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (cash != null) result.cash = cash;
    if (percent != null) result.percent = percent;
    if (thresholdAmount != null) result.thresholdAmount = thresholdAmount;
    if (targetType != null) result.targetType = targetType;
    if (store != null) result.store = store;
    if (storeId != null) result.storeId = storeId;
    if (startTime != null) result.startTime = startTime;
    if (endTime != null) result.endTime = endTime;
    return result;
  }

  Campaign._();

  factory Campaign.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Campaign()..mergeFromBuffer(data, registry);
  factory Campaign.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Campaign()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, Campaign_Discount> _Campaign_DiscountByTag =
      {
    3: Campaign_Discount.cash,
    4: Campaign_Discount.percent,
    0: Campaign_Discount.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Campaign',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: Campaign.$_createMessage)
    ..oo(0, [3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOM<CashDiscount>(3, _omitFieldNames ? '' : 'cash',
        subBuilder: CashDiscount.$_createMessage)
    ..aOM<PercentDiscount>(4, _omitFieldNames ? '' : 'percent',
        subBuilder: PercentDiscount.$_createMessage)
    ..aInt64(5, _omitFieldNames ? '' : 'thresholdAmount')
    ..aE<TargetType>(6, _omitFieldNames ? '' : 'targetType',
        enumValues: TargetType.values)
    ..aOM<$0.StoreCard>(7, _omitFieldNames ? '' : 'store',
        subBuilder: $0.StoreCard.$_createMessage)
    ..aOS(8, _omitFieldNames ? '' : 'storeId')
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'startTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'endTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Campaign clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Campaign copyWith(void Function(Campaign) updates) =>
      super.copyWith((message) => updates(message as Campaign)) as Campaign;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Campaign() / Campaign.new instead')
  static Campaign create() => Campaign._();
  static $pb.GeneratedMessage $_createMessage() => Campaign._();
  @$core.override
  Campaign createEmptyInstance() => Campaign._();
  @$core.pragma('dart2js:noInline')
  static Campaign getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Campaign>(Campaign.$_createMessage);
  static Campaign? _defaultInstance;

  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  Campaign_Discount whichDiscount() =>
      _Campaign_DiscountByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearDiscount() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  CashDiscount get cash => $_getN(2);
  @$pb.TagNumber(3)
  set cash(CashDiscount value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCash() => $_has(2);
  @$pb.TagNumber(3)
  void clearCash() => $_clearField(3);
  @$pb.TagNumber(3)
  CashDiscount ensureCash() => $_ensure(2);

  @$pb.TagNumber(4)
  PercentDiscount get percent => $_getN(3);
  @$pb.TagNumber(4)
  set percent(PercentDiscount value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPercent() => $_has(3);
  @$pb.TagNumber(4)
  void clearPercent() => $_clearField(4);
  @$pb.TagNumber(4)
  PercentDiscount ensurePercent() => $_ensure(3);

  /// 訂單金額的門檻；0 是沒有門檻。
  @$pb.TagNumber(5)
  $fixnum.Int64 get thresholdAmount => $_getI64(4);
  @$pb.TagNumber(5)
  set thresholdAmount($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasThresholdAmount() => $_has(4);
  @$pb.TagNumber(5)
  void clearThresholdAmount() => $_clearField(5);

  @$pb.TagNumber(6)
  TargetType get targetType => $_getN(5);
  @$pb.TagNumber(6)
  set targetType(TargetType value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasTargetType() => $_has(5);
  @$pb.TagNumber(6)
  void clearTargetType() => $_clearField(6);

  /// 適用的店家；不設是全平台。這家店不顯示（沒公開過）時也不設，client 不區分兩者，以 store_id 判斷適用範圍。
  @$pb.TagNumber(7)
  $0.StoreCard get store => $_getN(6);
  @$pb.TagNumber(7)
  set store($0.StoreCard value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasStore() => $_has(6);
  @$pb.TagNumber(7)
  void clearStore() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.StoreCard ensureStore() => $_ensure(6);

  /// 適用的店家 ID；不設是全平台。
  @$pb.TagNumber(8)
  $core.String get storeId => $_getSZ(7);
  @$pb.TagNumber(8)
  set storeId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasStoreId() => $_has(7);
  @$pb.TagNumber(8)
  void clearStoreId() => $_clearField(8);

  /// 生效時間窗。
  @$pb.TagNumber(9)
  $1.Timestamp get startTime => $_getN(8);
  @$pb.TagNumber(9)
  set startTime($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasStartTime() => $_has(8);
  @$pb.TagNumber(9)
  void clearStartTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureStartTime() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get endTime => $_getN(9);
  @$pb.TagNumber(10)
  set endTime($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasEndTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearEndTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureEndTime() => $_ensure(9);
}

/// Voucher 是買家持有的一張券。
class Voucher extends $pb.GeneratedMessage {
  factory Voucher({
    $core.String? id,
    Campaign? campaign,
    VoucherStatus? status,
    VoucherOrigin? origin,
    $core.bool? expired,
    $1.Timestamp? createTime,
    $fixnum.Int64? estimatedDiscount,
  }) {
    final result = Voucher._();
    if (id != null) result.id = id;
    if (campaign != null) result.campaign = campaign;
    if (status != null) result.status = status;
    if (origin != null) result.origin = origin;
    if (expired != null) result.expired = expired;
    if (createTime != null) result.createTime = createTime;
    if (estimatedDiscount != null) result.estimatedDiscount = estimatedDiscount;
    return result;
  }

  Voucher._();

  factory Voucher.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Voucher()..mergeFromBuffer(data, registry);
  factory Voucher.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Voucher()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Voucher',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: Voucher.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<Campaign>(2, _omitFieldNames ? '' : 'campaign',
        subBuilder: Campaign.$_createMessage)
    ..aE<VoucherStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: VoucherStatus.values)
    ..aE<VoucherOrigin>(4, _omitFieldNames ? '' : 'origin',
        enumValues: VoucherOrigin.values)
    ..aOB(5, _omitFieldNames ? '' : 'expired')
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aInt64(7, _omitFieldNames ? '' : 'estimatedDiscount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Voucher clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Voucher copyWith(void Function(Voucher) updates) =>
      super.copyWith((message) => updates(message as Voucher)) as Voucher;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Voucher() / Voucher.new instead')
  static Voucher create() => Voucher._();
  static $pb.GeneratedMessage $_createMessage() => Voucher._();
  @$core.override
  Voucher createEmptyInstance() => Voucher._();
  @$core.pragma('dart2js:noInline')
  static Voucher getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Voucher>(Voucher.$_createMessage);
  static Voucher? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  Campaign get campaign => $_getN(1);
  @$pb.TagNumber(2)
  set campaign(Campaign value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCampaign() => $_has(1);
  @$pb.TagNumber(2)
  void clearCampaign() => $_clearField(2);
  @$pb.TagNumber(2)
  Campaign ensureCampaign() => $_ensure(1);

  @$pb.TagNumber(3)
  VoucherStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(VoucherStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  VoucherOrigin get origin => $_getN(3);
  @$pb.TagNumber(4)
  set origin(VoucherOrigin value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasOrigin() => $_has(3);
  @$pb.TagNumber(4)
  void clearOrigin() => $_clearField(4);

  /// 活動的時間窗已過（只對 AVAILABLE 有意義；到期的券不能使用）。
  @$pb.TagNumber(5)
  $core.bool get expired => $_getBF(4);
  @$pb.TagNumber(5)
  set expired($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpired() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpired() => $_clearField(5);

  @$pb.TagNumber(6)
  $1.Timestamp get createTime => $_getN(5);
  @$pb.TagNumber(6)
  set createTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCreateTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreateTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureCreateTime() => $_ensure(5);

  /// 預估折抵金額，新台幣元；只有帶 checkout 的 ListMyVouchers 才設。
  @$pb.TagNumber(7)
  $fixnum.Int64 get estimatedDiscount => $_getI64(6);
  @$pb.TagNumber(7)
  set estimatedDiscount($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasEstimatedDiscount() => $_has(6);
  @$pb.TagNumber(7)
  void clearEstimatedDiscount() => $_clearField(7);
}

/// Checkout 是結帳情境：哪一家店、訂單金額多少。
class Checkout extends $pb.GeneratedMessage {
  factory Checkout({
    $core.String? storeId,
    $fixnum.Int64? orderAmount,
  }) {
    final result = Checkout._();
    if (storeId != null) result.storeId = storeId;
    if (orderAmount != null) result.orderAmount = orderAmount;
    return result;
  }

  Checkout._();

  factory Checkout.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Checkout()..mergeFromBuffer(data, registry);
  factory Checkout.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Checkout()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Checkout',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: Checkout.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aInt64(2, _omitFieldNames ? '' : 'orderAmount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Checkout clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Checkout copyWith(void Function(Checkout) updates) =>
      super.copyWith((message) => updates(message as Checkout)) as Checkout;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Checkout() / Checkout.new instead')
  static Checkout create() => Checkout._();
  static $pb.GeneratedMessage $_createMessage() => Checkout._();
  @$core.override
  Checkout createEmptyInstance() => Checkout._();
  @$core.pragma('dart2js:noInline')
  static Checkout getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Checkout>(Checkout.$_createMessage);
  static Checkout? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 新台幣元，至少 1。
  @$pb.TagNumber(2)
  $fixnum.Int64 get orderAmount => $_getI64(1);
  @$pb.TagNumber(2)
  set orderAmount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOrderAmount() => $_has(1);
  @$pb.TagNumber(2)
  void clearOrderAmount() => $_clearField(2);
}

class ListMyVouchersRequest extends $pb.GeneratedMessage {
  factory ListMyVouchersRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.Iterable<VoucherStatus>? statuses,
    Checkout? checkout,
  }) {
    final result = ListMyVouchersRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (statuses != null) result.statuses.addAll(statuses);
    if (checkout != null) result.checkout = checkout;
    return result;
  }

  ListMyVouchersRequest._();

  factory ListMyVouchersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyVouchersRequest()..mergeFromBuffer(data, registry);
  factory ListMyVouchersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyVouchersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyVouchersRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListMyVouchersRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..pc<VoucherStatus>(
        3, _omitFieldNames ? '' : 'statuses', $pb.PbFieldType.KE,
        valueOf: VoucherStatus.valueOf,
        enumValues: VoucherStatus.values,
        defaultEnumValue: VoucherStatus.VOUCHER_STATUS_UNSPECIFIED)
    ..aOM<Checkout>(4, _omitFieldNames ? '' : 'checkout',
        subBuilder: Checkout.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyVouchersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyVouchersRequest copyWith(
          void Function(ListMyVouchersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyVouchersRequest))
          as ListMyVouchersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyVouchersRequest() / ListMyVouchersRequest.new instead')
  static ListMyVouchersRequest create() => ListMyVouchersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyVouchersRequest._();
  @$core.override
  ListMyVouchersRequest createEmptyInstance() => ListMyVouchersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyVouchersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyVouchersRequest>(
          ListMyVouchersRequest.$_createMessage);
  static ListMyVouchersRequest? _defaultInstance;

  /// 預設與上限見 page_size 的說明：0 用預設 20，超過 100 以 100 計，負數回 invalid_argument。
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

  @$pb.TagNumber(3)
  $pb.PbList<VoucherStatus> get statuses => $_getList(2);

  @$pb.TagNumber(4)
  Checkout get checkout => $_getN(3);
  @$pb.TagNumber(4)
  set checkout(Checkout value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCheckout() => $_has(3);
  @$pb.TagNumber(4)
  void clearCheckout() => $_clearField(4);
  @$pb.TagNumber(4)
  Checkout ensureCheckout() => $_ensure(3);
}

class ListMyVouchersResponse extends $pb.GeneratedMessage {
  factory ListMyVouchersResponse({
    $core.Iterable<Voucher>? vouchers,
    $core.String? nextPageToken,
  }) {
    final result = ListMyVouchersResponse._();
    if (vouchers != null) result.vouchers.addAll(vouchers);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyVouchersResponse._();

  factory ListMyVouchersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyVouchersResponse()..mergeFromBuffer(data, registry);
  factory ListMyVouchersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyVouchersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyVouchersResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListMyVouchersResponse.$_createMessage)
    ..pPM<Voucher>(1, _omitFieldNames ? '' : 'vouchers',
        subBuilder: Voucher.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyVouchersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyVouchersResponse copyWith(
          void Function(ListMyVouchersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyVouchersResponse))
          as ListMyVouchersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyVouchersResponse() / ListMyVouchersResponse.new instead')
  static ListMyVouchersResponse create() => ListMyVouchersResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyVouchersResponse._();
  @$core.override
  ListMyVouchersResponse createEmptyInstance() => ListMyVouchersResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyVouchersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyVouchersResponse>(
          ListMyVouchersResponse.$_createMessage);
  static ListMyVouchersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Voucher> get vouchers => $_getList(0);

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

class ListClaimableCampaignsRequest extends $pb.GeneratedMessage {
  factory ListClaimableCampaignsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? storeId,
  }) {
    final result = ListClaimableCampaignsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  ListClaimableCampaignsRequest._();

  factory ListClaimableCampaignsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListClaimableCampaignsRequest()..mergeFromBuffer(data, registry);
  factory ListClaimableCampaignsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListClaimableCampaignsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListClaimableCampaignsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListClaimableCampaignsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aOS(3, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListClaimableCampaignsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListClaimableCampaignsRequest copyWith(
          void Function(ListClaimableCampaignsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListClaimableCampaignsRequest))
          as ListClaimableCampaignsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListClaimableCampaignsRequest() / ListClaimableCampaignsRequest.new instead')
  static ListClaimableCampaignsRequest create() =>
      ListClaimableCampaignsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListClaimableCampaignsRequest._();
  @$core.override
  ListClaimableCampaignsRequest createEmptyInstance() =>
      ListClaimableCampaignsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListClaimableCampaignsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListClaimableCampaignsRequest>(
          ListClaimableCampaignsRequest.$_createMessage);
  static ListClaimableCampaignsRequest? _defaultInstance;

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

  @$pb.TagNumber(3)
  $core.String get storeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreId() => $_clearField(3);
}

class ListClaimableCampaignsResponse extends $pb.GeneratedMessage {
  factory ListClaimableCampaignsResponse({
    $core.Iterable<Campaign>? campaigns,
    $core.String? nextPageToken,
  }) {
    final result = ListClaimableCampaignsResponse._();
    if (campaigns != null) result.campaigns.addAll(campaigns);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListClaimableCampaignsResponse._();

  factory ListClaimableCampaignsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListClaimableCampaignsResponse()..mergeFromBuffer(data, registry);
  factory ListClaimableCampaignsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListClaimableCampaignsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListClaimableCampaignsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListClaimableCampaignsResponse.$_createMessage)
    ..pPM<Campaign>(1, _omitFieldNames ? '' : 'campaigns',
        subBuilder: Campaign.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListClaimableCampaignsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListClaimableCampaignsResponse copyWith(
          void Function(ListClaimableCampaignsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListClaimableCampaignsResponse))
          as ListClaimableCampaignsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListClaimableCampaignsResponse() / ListClaimableCampaignsResponse.new instead')
  static ListClaimableCampaignsResponse create() =>
      ListClaimableCampaignsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListClaimableCampaignsResponse._();
  @$core.override
  ListClaimableCampaignsResponse createEmptyInstance() =>
      ListClaimableCampaignsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListClaimableCampaignsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListClaimableCampaignsResponse>(
          ListClaimableCampaignsResponse.$_createMessage);
  static ListClaimableCampaignsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Campaign> get campaigns => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ClaimCouponRequest extends $pb.GeneratedMessage {
  factory ClaimCouponRequest({
    $core.String? campaignId,
  }) {
    final result = ClaimCouponRequest._();
    if (campaignId != null) result.campaignId = campaignId;
    return result;
  }

  ClaimCouponRequest._();

  factory ClaimCouponRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClaimCouponRequest()..mergeFromBuffer(data, registry);
  factory ClaimCouponRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClaimCouponRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClaimCouponRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ClaimCouponRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'campaignId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClaimCouponRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClaimCouponRequest copyWith(void Function(ClaimCouponRequest) updates) =>
      super.copyWith((message) => updates(message as ClaimCouponRequest))
          as ClaimCouponRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ClaimCouponRequest() / ClaimCouponRequest.new instead')
  static ClaimCouponRequest create() => ClaimCouponRequest._();
  static $pb.GeneratedMessage $_createMessage() => ClaimCouponRequest._();
  @$core.override
  ClaimCouponRequest createEmptyInstance() => ClaimCouponRequest._();
  @$core.pragma('dart2js:noInline')
  static ClaimCouponRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClaimCouponRequest>(
          ClaimCouponRequest.$_createMessage);
  static ClaimCouponRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get campaignId => $_getSZ(0);
  @$pb.TagNumber(1)
  set campaignId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaignId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaignId() => $_clearField(1);
}

class ClaimCouponResponse extends $pb.GeneratedMessage {
  factory ClaimCouponResponse({
    Voucher? voucher,
  }) {
    final result = ClaimCouponResponse._();
    if (voucher != null) result.voucher = voucher;
    return result;
  }

  ClaimCouponResponse._();

  factory ClaimCouponResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClaimCouponResponse()..mergeFromBuffer(data, registry);
  factory ClaimCouponResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClaimCouponResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClaimCouponResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ClaimCouponResponse.$_createMessage)
    ..aOM<Voucher>(1, _omitFieldNames ? '' : 'voucher',
        subBuilder: Voucher.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClaimCouponResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClaimCouponResponse copyWith(void Function(ClaimCouponResponse) updates) =>
      super.copyWith((message) => updates(message as ClaimCouponResponse))
          as ClaimCouponResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ClaimCouponResponse() / ClaimCouponResponse.new instead')
  static ClaimCouponResponse create() => ClaimCouponResponse._();
  static $pb.GeneratedMessage $_createMessage() => ClaimCouponResponse._();
  @$core.override
  ClaimCouponResponse createEmptyInstance() => ClaimCouponResponse._();
  @$core.pragma('dart2js:noInline')
  static ClaimCouponResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClaimCouponResponse>(
          ClaimCouponResponse.$_createMessage);
  static ClaimCouponResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Voucher get voucher => $_getN(0);
  @$pb.TagNumber(1)
  set voucher(Voucher value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoucher() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoucher() => $_clearField(1);
  @$pb.TagNumber(1)
  Voucher ensureVoucher() => $_ensure(0);
}

/// SellerCampaign 是店家看到的活動：公開規則加上狀態與發行數。
class SellerCampaign extends $pb.GeneratedMessage {
  factory SellerCampaign({
    Campaign? campaign,
    CampaignStatus? status,
    $core.int? totalIssueLimit,
    $core.int? issuedCount,
    $core.bool? expired,
    $1.Timestamp? createTime,
    $1.Timestamp? endedTime,
  }) {
    final result = SellerCampaign._();
    if (campaign != null) result.campaign = campaign;
    if (status != null) result.status = status;
    if (totalIssueLimit != null) result.totalIssueLimit = totalIssueLimit;
    if (issuedCount != null) result.issuedCount = issuedCount;
    if (expired != null) result.expired = expired;
    if (createTime != null) result.createTime = createTime;
    if (endedTime != null) result.endedTime = endedTime;
    return result;
  }

  SellerCampaign._();

  factory SellerCampaign.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerCampaign()..mergeFromBuffer(data, registry);
  factory SellerCampaign.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerCampaign()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerCampaign',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: SellerCampaign.$_createMessage)
    ..aOM<Campaign>(1, _omitFieldNames ? '' : 'campaign',
        subBuilder: Campaign.$_createMessage)
    ..aE<CampaignStatus>(2, _omitFieldNames ? '' : 'status',
        enumValues: CampaignStatus.values)
    ..aI(3, _omitFieldNames ? '' : 'totalIssueLimit')
    ..aI(4, _omitFieldNames ? '' : 'issuedCount')
    ..aOB(5, _omitFieldNames ? '' : 'expired')
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'endedTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerCampaign clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerCampaign copyWith(void Function(SellerCampaign) updates) =>
      super.copyWith((message) => updates(message as SellerCampaign))
          as SellerCampaign;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerCampaign() / SellerCampaign.new instead')
  static SellerCampaign create() => SellerCampaign._();
  static $pb.GeneratedMessage $_createMessage() => SellerCampaign._();
  @$core.override
  SellerCampaign createEmptyInstance() => SellerCampaign._();
  @$core.pragma('dart2js:noInline')
  static SellerCampaign getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerCampaign>(
          SellerCampaign.$_createMessage);
  static SellerCampaign? _defaultInstance;

  @$pb.TagNumber(1)
  Campaign get campaign => $_getN(0);
  @$pb.TagNumber(1)
  set campaign(Campaign value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaign() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaign() => $_clearField(1);
  @$pb.TagNumber(1)
  Campaign ensureCampaign() => $_ensure(0);

  @$pb.TagNumber(2)
  CampaignStatus get status => $_getN(1);
  @$pb.TagNumber(2)
  set status(CampaignStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  /// 總發行量；不設是無上限（與 0 不同：上限至少 1）。
  @$pb.TagNumber(3)
  $core.int get totalIssueLimit => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalIssueLimit($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalIssueLimit() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalIssueLimit() => $_clearField(3);

  /// 已發行（已被領取或發放）的張數。
  @$pb.TagNumber(4)
  $core.int get issuedCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set issuedCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIssuedCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearIssuedCount() => $_clearField(4);

  /// 活動的時間窗已過。
  @$pb.TagNumber(5)
  $core.bool get expired => $_getBF(4);
  @$pb.TagNumber(5)
  set expired($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpired() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpired() => $_clearField(5);

  @$pb.TagNumber(6)
  $1.Timestamp get createTime => $_getN(5);
  @$pb.TagNumber(6)
  set createTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCreateTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreateTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureCreateTime() => $_ensure(5);

  /// 提前結束的時間；沒有結束時不設。
  @$pb.TagNumber(7)
  $1.Timestamp get endedTime => $_getN(6);
  @$pb.TagNumber(7)
  set endedTime($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasEndedTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearEndedTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureEndedTime() => $_ensure(6);
}

enum CampaignInput_Discount { cash, percent, notSet }

/// CampaignInput 是建立活動的內容。
class CampaignInput extends $pb.GeneratedMessage {
  factory CampaignInput({
    $core.String? title,
    CashDiscount? cash,
    PercentDiscount? percent,
    $fixnum.Int64? thresholdAmount,
    TargetType? targetType,
    $1.Timestamp? startTime,
    $1.Timestamp? endTime,
    $core.int? totalIssueLimit,
    $core.Iterable<$core.String>? designatedBuyerIds,
  }) {
    final result = CampaignInput._();
    if (title != null) result.title = title;
    if (cash != null) result.cash = cash;
    if (percent != null) result.percent = percent;
    if (thresholdAmount != null) result.thresholdAmount = thresholdAmount;
    if (targetType != null) result.targetType = targetType;
    if (startTime != null) result.startTime = startTime;
    if (endTime != null) result.endTime = endTime;
    if (totalIssueLimit != null) result.totalIssueLimit = totalIssueLimit;
    if (designatedBuyerIds != null)
      result.designatedBuyerIds.addAll(designatedBuyerIds);
    return result;
  }

  CampaignInput._();

  factory CampaignInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CampaignInput()..mergeFromBuffer(data, registry);
  factory CampaignInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CampaignInput()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, CampaignInput_Discount>
      _CampaignInput_DiscountByTag = {
    2: CampaignInput_Discount.cash,
    3: CampaignInput_Discount.percent,
    0: CampaignInput_Discount.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CampaignInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: CampaignInput.$_createMessage)
    ..oo(0, [2, 3])
    ..aOS(1, _omitFieldNames ? '' : 'title')
    ..aOM<CashDiscount>(2, _omitFieldNames ? '' : 'cash',
        subBuilder: CashDiscount.$_createMessage)
    ..aOM<PercentDiscount>(3, _omitFieldNames ? '' : 'percent',
        subBuilder: PercentDiscount.$_createMessage)
    ..aInt64(4, _omitFieldNames ? '' : 'thresholdAmount')
    ..aE<TargetType>(5, _omitFieldNames ? '' : 'targetType',
        enumValues: TargetType.values)
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'startTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'endTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aI(8, _omitFieldNames ? '' : 'totalIssueLimit')
    ..pPS(9, _omitFieldNames ? '' : 'designatedBuyerIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CampaignInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CampaignInput copyWith(void Function(CampaignInput) updates) =>
      super.copyWith((message) => updates(message as CampaignInput))
          as CampaignInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CampaignInput() / CampaignInput.new instead')
  static CampaignInput create() => CampaignInput._();
  static $pb.GeneratedMessage $_createMessage() => CampaignInput._();
  @$core.override
  CampaignInput createEmptyInstance() => CampaignInput._();
  @$core.pragma('dart2js:noInline')
  static CampaignInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CampaignInput>(
          CampaignInput.$_createMessage);
  static CampaignInput? _defaultInstance;

  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  CampaignInput_Discount whichDiscount() =>
      _CampaignInput_DiscountByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  void clearDiscount() => $_clearField($_whichOneof(0));

  /// 1 到 100 個字元（去掉前後空白後）。
  @$pb.TagNumber(1)
  $core.String get title => $_getSZ(0);
  @$pb.TagNumber(1)
  set title($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTitle() => $_has(0);
  @$pb.TagNumber(1)
  void clearTitle() => $_clearField(1);

  @$pb.TagNumber(2)
  CashDiscount get cash => $_getN(1);
  @$pb.TagNumber(2)
  set cash(CashDiscount value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCash() => $_has(1);
  @$pb.TagNumber(2)
  void clearCash() => $_clearField(2);
  @$pb.TagNumber(2)
  CashDiscount ensureCash() => $_ensure(1);

  @$pb.TagNumber(3)
  PercentDiscount get percent => $_getN(2);
  @$pb.TagNumber(3)
  set percent(PercentDiscount value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPercent() => $_has(2);
  @$pb.TagNumber(3)
  void clearPercent() => $_clearField(3);
  @$pb.TagNumber(3)
  PercentDiscount ensurePercent() => $_ensure(2);

  /// 門檻，至少 0；現金折抵的金額不得超過門檻（門檻 0 不套用）。
  @$pb.TagNumber(4)
  $fixnum.Int64 get thresholdAmount => $_getI64(3);
  @$pb.TagNumber(4)
  set thresholdAmount($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasThresholdAmount() => $_has(3);
  @$pb.TagNumber(4)
  void clearThresholdAmount() => $_clearField(4);

  /// 店家只能建 ALL、MEMBER、DESIGNATED。
  @$pb.TagNumber(5)
  TargetType get targetType => $_getN(4);
  @$pb.TagNumber(5)
  set targetType(TargetType value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasTargetType() => $_has(4);
  @$pb.TagNumber(5)
  void clearTargetType() => $_clearField(5);

  @$pb.TagNumber(6)
  $1.Timestamp get startTime => $_getN(5);
  @$pb.TagNumber(6)
  set startTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStartTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearStartTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureStartTime() => $_ensure(5);

  /// 必須晚於現在與 start_time。
  @$pb.TagNumber(7)
  $1.Timestamp get endTime => $_getN(6);
  @$pb.TagNumber(7)
  set endTime($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasEndTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearEndTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureEndTime() => $_ensure(6);

  /// 總發行量；不設是無上限，設了至少 1（0 回 invalid_argument，不當成無上限）。
  @$pb.TagNumber(8)
  $core.int get totalIssueLimit => $_getIZ(7);
  @$pb.TagNumber(8)
  set totalIssueLimit($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasTotalIssueLimit() => $_has(7);
  @$pb.TagNumber(8)
  void clearTotalIssueLimit() => $_clearField(8);

  /// DESIGNATED 的名單（買家 ID，例如 CRM 客戶的 buyer_id），最多 500 位；其他資格不得帶。
  @$pb.TagNumber(9)
  $pb.PbList<$core.String> get designatedBuyerIds => $_getList(8);
}

class CreateCampaignRequest extends $pb.GeneratedMessage {
  factory CreateCampaignRequest({
    CampaignInput? campaign,
  }) {
    final result = CreateCampaignRequest._();
    if (campaign != null) result.campaign = campaign;
    return result;
  }

  CreateCampaignRequest._();

  factory CreateCampaignRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCampaignRequest()..mergeFromBuffer(data, registry);
  factory CreateCampaignRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCampaignRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCampaignRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: CreateCampaignRequest.$_createMessage)
    ..aOM<CampaignInput>(1, _omitFieldNames ? '' : 'campaign',
        subBuilder: CampaignInput.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCampaignRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCampaignRequest copyWith(
          void Function(CreateCampaignRequest) updates) =>
      super.copyWith((message) => updates(message as CreateCampaignRequest))
          as CreateCampaignRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateCampaignRequest() / CreateCampaignRequest.new instead')
  static CreateCampaignRequest create() => CreateCampaignRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateCampaignRequest._();
  @$core.override
  CreateCampaignRequest createEmptyInstance() => CreateCampaignRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateCampaignRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCampaignRequest>(
          CreateCampaignRequest.$_createMessage);
  static CreateCampaignRequest? _defaultInstance;

  @$pb.TagNumber(1)
  CampaignInput get campaign => $_getN(0);
  @$pb.TagNumber(1)
  set campaign(CampaignInput value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaign() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaign() => $_clearField(1);
  @$pb.TagNumber(1)
  CampaignInput ensureCampaign() => $_ensure(0);
}

class CreateCampaignResponse extends $pb.GeneratedMessage {
  factory CreateCampaignResponse({
    SellerCampaign? campaign,
  }) {
    final result = CreateCampaignResponse._();
    if (campaign != null) result.campaign = campaign;
    return result;
  }

  CreateCampaignResponse._();

  factory CreateCampaignResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCampaignResponse()..mergeFromBuffer(data, registry);
  factory CreateCampaignResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCampaignResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCampaignResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: CreateCampaignResponse.$_createMessage)
    ..aOM<SellerCampaign>(1, _omitFieldNames ? '' : 'campaign',
        subBuilder: SellerCampaign.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCampaignResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCampaignResponse copyWith(
          void Function(CreateCampaignResponse) updates) =>
      super.copyWith((message) => updates(message as CreateCampaignResponse))
          as CreateCampaignResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateCampaignResponse() / CreateCampaignResponse.new instead')
  static CreateCampaignResponse create() => CreateCampaignResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateCampaignResponse._();
  @$core.override
  CreateCampaignResponse createEmptyInstance() => CreateCampaignResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateCampaignResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCampaignResponse>(
          CreateCampaignResponse.$_createMessage);
  static CreateCampaignResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerCampaign get campaign => $_getN(0);
  @$pb.TagNumber(1)
  set campaign(SellerCampaign value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaign() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaign() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerCampaign ensureCampaign() => $_ensure(0);
}

class ListCampaignsRequest extends $pb.GeneratedMessage {
  factory ListCampaignsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    CampaignStatus? status,
  }) {
    final result = ListCampaignsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (status != null) result.status = status;
    return result;
  }

  ListCampaignsRequest._();

  factory ListCampaignsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCampaignsRequest()..mergeFromBuffer(data, registry);
  factory ListCampaignsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCampaignsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCampaignsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListCampaignsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aE<CampaignStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: CampaignStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCampaignsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCampaignsRequest copyWith(void Function(ListCampaignsRequest) updates) =>
      super.copyWith((message) => updates(message as ListCampaignsRequest))
          as ListCampaignsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCampaignsRequest() / ListCampaignsRequest.new instead')
  static ListCampaignsRequest create() => ListCampaignsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCampaignsRequest._();
  @$core.override
  ListCampaignsRequest createEmptyInstance() => ListCampaignsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCampaignsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCampaignsRequest>(
          ListCampaignsRequest.$_createMessage);
  static ListCampaignsRequest? _defaultInstance;

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

  /// 只列這個狀態；不設是全部。明確送出 CAMPAIGN_STATUS_UNSPECIFIED 回 invalid_argument。
  @$pb.TagNumber(3)
  CampaignStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(CampaignStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);
}

class ListCampaignsResponse extends $pb.GeneratedMessage {
  factory ListCampaignsResponse({
    $core.Iterable<SellerCampaign>? campaigns,
    $core.String? nextPageToken,
    $core.int? maxActiveCampaigns,
    $core.int? activeCampaigns,
  }) {
    final result = ListCampaignsResponse._();
    if (campaigns != null) result.campaigns.addAll(campaigns);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (maxActiveCampaigns != null)
      result.maxActiveCampaigns = maxActiveCampaigns;
    if (activeCampaigns != null) result.activeCampaigns = activeCampaigns;
    return result;
  }

  ListCampaignsResponse._();

  factory ListCampaignsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCampaignsResponse()..mergeFromBuffer(data, registry);
  factory ListCampaignsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCampaignsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCampaignsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: ListCampaignsResponse.$_createMessage)
    ..pPM<SellerCampaign>(1, _omitFieldNames ? '' : 'campaigns',
        subBuilder: SellerCampaign.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aI(3, _omitFieldNames ? '' : 'maxActiveCampaigns')
    ..aI(4, _omitFieldNames ? '' : 'activeCampaigns')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCampaignsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCampaignsResponse copyWith(
          void Function(ListCampaignsResponse) updates) =>
      super.copyWith((message) => updates(message as ListCampaignsResponse))
          as ListCampaignsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCampaignsResponse() / ListCampaignsResponse.new instead')
  static ListCampaignsResponse create() => ListCampaignsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCampaignsResponse._();
  @$core.override
  ListCampaignsResponse createEmptyInstance() => ListCampaignsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCampaignsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCampaignsResponse>(
          ListCampaignsResponse.$_createMessage);
  static ListCampaignsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SellerCampaign> get campaigns => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 店家同時進行中的活動數上限。
  @$pb.TagNumber(3)
  $core.int get maxActiveCampaigns => $_getIZ(2);
  @$pb.TagNumber(3)
  set maxActiveCampaigns($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMaxActiveCampaigns() => $_has(2);
  @$pb.TagNumber(3)
  void clearMaxActiveCampaigns() => $_clearField(3);

  /// 目前同時進行中的活動數（進行中、時間窗沒過、不含補償）。
  @$pb.TagNumber(4)
  $core.int get activeCampaigns => $_getIZ(3);
  @$pb.TagNumber(4)
  set activeCampaigns($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasActiveCampaigns() => $_has(3);
  @$pb.TagNumber(4)
  void clearActiveCampaigns() => $_clearField(4);
}

class GetCampaignRequest extends $pb.GeneratedMessage {
  factory GetCampaignRequest({
    $core.String? campaignId,
  }) {
    final result = GetCampaignRequest._();
    if (campaignId != null) result.campaignId = campaignId;
    return result;
  }

  GetCampaignRequest._();

  factory GetCampaignRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCampaignRequest()..mergeFromBuffer(data, registry);
  factory GetCampaignRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCampaignRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCampaignRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: GetCampaignRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'campaignId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCampaignRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCampaignRequest copyWith(void Function(GetCampaignRequest) updates) =>
      super.copyWith((message) => updates(message as GetCampaignRequest))
          as GetCampaignRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetCampaignRequest() / GetCampaignRequest.new instead')
  static GetCampaignRequest create() => GetCampaignRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCampaignRequest._();
  @$core.override
  GetCampaignRequest createEmptyInstance() => GetCampaignRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCampaignRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCampaignRequest>(
          GetCampaignRequest.$_createMessage);
  static GetCampaignRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get campaignId => $_getSZ(0);
  @$pb.TagNumber(1)
  set campaignId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaignId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaignId() => $_clearField(1);
}

/// DesignatedBuyer 是 DESIGNATED 名單裡的一位買家；暱稱由 buyer 補（買家已刪除時不設）。
class DesignatedBuyer extends $pb.GeneratedMessage {
  factory DesignatedBuyer({
    $core.String? buyerId,
    $core.String? name,
  }) {
    final result = DesignatedBuyer._();
    if (buyerId != null) result.buyerId = buyerId;
    if (name != null) result.name = name;
    return result;
  }

  DesignatedBuyer._();

  factory DesignatedBuyer.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DesignatedBuyer()..mergeFromBuffer(data, registry);
  factory DesignatedBuyer.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DesignatedBuyer()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DesignatedBuyer',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: DesignatedBuyer.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'buyerId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DesignatedBuyer clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DesignatedBuyer copyWith(void Function(DesignatedBuyer) updates) =>
      super.copyWith((message) => updates(message as DesignatedBuyer))
          as DesignatedBuyer;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DesignatedBuyer() / DesignatedBuyer.new instead')
  static DesignatedBuyer create() => DesignatedBuyer._();
  static $pb.GeneratedMessage $_createMessage() => DesignatedBuyer._();
  @$core.override
  DesignatedBuyer createEmptyInstance() => DesignatedBuyer._();
  @$core.pragma('dart2js:noInline')
  static DesignatedBuyer getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DesignatedBuyer>(
          DesignatedBuyer.$_createMessage);
  static DesignatedBuyer? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get buyerId => $_getSZ(0);
  @$pb.TagNumber(1)
  set buyerId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBuyerId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBuyerId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class GetCampaignResponse extends $pb.GeneratedMessage {
  factory GetCampaignResponse({
    SellerCampaign? campaign,
    $core.Iterable<DesignatedBuyer>? designatedBuyers,
    $core.int? availableCount,
    $core.int? lockedCount,
    $core.int? redeemedCount,
    $core.int? voidedCount,
  }) {
    final result = GetCampaignResponse._();
    if (campaign != null) result.campaign = campaign;
    if (designatedBuyers != null)
      result.designatedBuyers.addAll(designatedBuyers);
    if (availableCount != null) result.availableCount = availableCount;
    if (lockedCount != null) result.lockedCount = lockedCount;
    if (redeemedCount != null) result.redeemedCount = redeemedCount;
    if (voidedCount != null) result.voidedCount = voidedCount;
    return result;
  }

  GetCampaignResponse._();

  factory GetCampaignResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCampaignResponse()..mergeFromBuffer(data, registry);
  factory GetCampaignResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCampaignResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCampaignResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: GetCampaignResponse.$_createMessage)
    ..aOM<SellerCampaign>(1, _omitFieldNames ? '' : 'campaign',
        subBuilder: SellerCampaign.$_createMessage)
    ..pPM<DesignatedBuyer>(2, _omitFieldNames ? '' : 'designatedBuyers',
        subBuilder: DesignatedBuyer.$_createMessage)
    ..aI(3, _omitFieldNames ? '' : 'availableCount')
    ..aI(4, _omitFieldNames ? '' : 'lockedCount')
    ..aI(5, _omitFieldNames ? '' : 'redeemedCount')
    ..aI(6, _omitFieldNames ? '' : 'voidedCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCampaignResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCampaignResponse copyWith(void Function(GetCampaignResponse) updates) =>
      super.copyWith((message) => updates(message as GetCampaignResponse))
          as GetCampaignResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetCampaignResponse() / GetCampaignResponse.new instead')
  static GetCampaignResponse create() => GetCampaignResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetCampaignResponse._();
  @$core.override
  GetCampaignResponse createEmptyInstance() => GetCampaignResponse._();
  @$core.pragma('dart2js:noInline')
  static GetCampaignResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCampaignResponse>(
          GetCampaignResponse.$_createMessage);
  static GetCampaignResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerCampaign get campaign => $_getN(0);
  @$pb.TagNumber(1)
  set campaign(SellerCampaign value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaign() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaign() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerCampaign ensureCampaign() => $_ensure(0);

  @$pb.TagNumber(2)
  $pb.PbList<DesignatedBuyer> get designatedBuyers => $_getList(1);

  /// 各狀態的券數。
  @$pb.TagNumber(3)
  $core.int get availableCount => $_getIZ(2);
  @$pb.TagNumber(3)
  set availableCount($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAvailableCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearAvailableCount() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get lockedCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set lockedCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasLockedCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearLockedCount() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get redeemedCount => $_getIZ(4);
  @$pb.TagNumber(5)
  set redeemedCount($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRedeemedCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearRedeemedCount() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.int get voidedCount => $_getIZ(5);
  @$pb.TagNumber(6)
  set voidedCount($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVoidedCount() => $_has(5);
  @$pb.TagNumber(6)
  void clearVoidedCount() => $_clearField(6);
}

class EndCampaignRequest extends $pb.GeneratedMessage {
  factory EndCampaignRequest({
    $core.String? campaignId,
  }) {
    final result = EndCampaignRequest._();
    if (campaignId != null) result.campaignId = campaignId;
    return result;
  }

  EndCampaignRequest._();

  factory EndCampaignRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EndCampaignRequest()..mergeFromBuffer(data, registry);
  factory EndCampaignRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EndCampaignRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCampaignRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: EndCampaignRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'campaignId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCampaignRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCampaignRequest copyWith(void Function(EndCampaignRequest) updates) =>
      super.copyWith((message) => updates(message as EndCampaignRequest))
          as EndCampaignRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use EndCampaignRequest() / EndCampaignRequest.new instead')
  static EndCampaignRequest create() => EndCampaignRequest._();
  static $pb.GeneratedMessage $_createMessage() => EndCampaignRequest._();
  @$core.override
  EndCampaignRequest createEmptyInstance() => EndCampaignRequest._();
  @$core.pragma('dart2js:noInline')
  static EndCampaignRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCampaignRequest>(
          EndCampaignRequest.$_createMessage);
  static EndCampaignRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get campaignId => $_getSZ(0);
  @$pb.TagNumber(1)
  set campaignId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaignId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaignId() => $_clearField(1);
}

class EndCampaignResponse extends $pb.GeneratedMessage {
  factory EndCampaignResponse({
    SellerCampaign? campaign,
    $core.int? voidedVouchers,
  }) {
    final result = EndCampaignResponse._();
    if (campaign != null) result.campaign = campaign;
    if (voidedVouchers != null) result.voidedVouchers = voidedVouchers;
    return result;
  }

  EndCampaignResponse._();

  factory EndCampaignResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EndCampaignResponse()..mergeFromBuffer(data, registry);
  factory EndCampaignResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EndCampaignResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCampaignResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.coupon.v1alpha1'),
      createEmptyInstance: EndCampaignResponse.$_createMessage)
    ..aOM<SellerCampaign>(1, _omitFieldNames ? '' : 'campaign',
        subBuilder: SellerCampaign.$_createMessage)
    ..aI(2, _omitFieldNames ? '' : 'voidedVouchers')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCampaignResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCampaignResponse copyWith(void Function(EndCampaignResponse) updates) =>
      super.copyWith((message) => updates(message as EndCampaignResponse))
          as EndCampaignResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use EndCampaignResponse() / EndCampaignResponse.new instead')
  static EndCampaignResponse create() => EndCampaignResponse._();
  static $pb.GeneratedMessage $_createMessage() => EndCampaignResponse._();
  @$core.override
  EndCampaignResponse createEmptyInstance() => EndCampaignResponse._();
  @$core.pragma('dart2js:noInline')
  static EndCampaignResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCampaignResponse>(
          EndCampaignResponse.$_createMessage);
  static EndCampaignResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerCampaign get campaign => $_getN(0);
  @$pb.TagNumber(1)
  set campaign(SellerCampaign value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCampaign() => $_has(0);
  @$pb.TagNumber(1)
  void clearCampaign() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerCampaign ensureCampaign() => $_ensure(0);

  /// 這次作廢的券數。
  @$pb.TagNumber(2)
  $core.int get voidedVouchers => $_getIZ(1);
  @$pb.TagNumber(2)
  set voidedVouchers($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoidedVouchers() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoidedVouchers() => $_clearField(2);
}

/// CouponAppService 是登入帳號的優惠券。
class CouponAppServiceApi {
  final $pb.RpcClient _client;

  CouponAppServiceApi(this._client);

  /// ListMyVouchers 列出呼叫者（買家）持有的券，新到舊。statuses 為空是全部狀態；到期的券仍是 AVAILABLE，以 expired 呈現。
  /// 帶 checkout 時是結帳情境：statuses 必須為空或只含 AVAILABLE，只回這筆訂單現在用得上的券（與鎖券同一組檢查：狀態、時間窗、店家停權、資格、店家範圍、門檻、折抵大於 0），
  /// 每張附 estimated_discount。page_token 只能用在同一組 statuses 與 checkout。
  $async.Future<ListMyVouchersResponse> listMyVouchers(
          $pb.ClientContext? ctx, ListMyVouchersRequest request) =>
      _client.invoke<ListMyVouchersResponse>(ctx, 'CouponAppService',
          'ListMyVouchers', request, ListMyVouchersResponse());

  /// ListClaimableCampaigns 列出呼叫者現在領得到的活動，新到舊：進行中、在時間窗內、還有名額、呼叫者符合資格、而且還沒有這個活動的券（與 ClaimCoupon 同一個資格判定）。
  /// store_id 有值時只列該店與全平台的活動。page_token 只能用在同一個 store_id。
  $async.Future<ListClaimableCampaignsResponse> listClaimableCampaigns(
          $pb.ClientContext? ctx, ListClaimableCampaignsRequest request) =>
      _client.invoke<ListClaimableCampaignsResponse>(ctx, 'CouponAppService',
          'ListClaimableCampaigns', request, ListClaimableCampaignsResponse());

  /// ClaimCoupon 領取活動的券，回傳領到的券。已經領過回既有的那張（冪等，不再計數）。活動不存在回 not_found；不是進行中、不在時間窗、發行店家停權、
  /// 不符合資格回 failed_precondition（reason 見 ErrorReason 的 COUPON_*）；領完了回 failed_precondition 與 COUPON_ISSUE_LIMIT_REACHED；
  /// 曾經領過但券已被作廢回 failed_precondition。
  $async.Future<ClaimCouponResponse> claimCoupon(
          $pb.ClientContext? ctx, ClaimCouponRequest request) =>
      _client.invoke<ClaimCouponResponse>(ctx, 'CouponAppService',
          'ClaimCoupon', request, ClaimCouponResponse());

  /// CreateCampaign 以呼叫者此刻的店建立活動，直接生效（ACTIVE）。需要賣家能力 CAP_MARKETING。每家店同時進行的活動數有上限（預設 5），超過回
  /// failed_precondition 與 COUPON_QUOTA_EXCEEDED。DESIGNATED 的名單不得為空、不得含呼叫者自己的買家身分。欄位不合法回 invalid_argument 並指出欄位。
  $async.Future<CreateCampaignResponse> createCampaign(
          $pb.ClientContext? ctx, CreateCampaignRequest request) =>
      _client.invoke<CreateCampaignResponse>(ctx, 'CouponAppService',
          'CreateCampaign', request, CreateCampaignResponse());

  /// ListCampaigns 列出呼叫者此刻的店自己發的活動，新到舊（不含系統建立的補償活動）。需要賣家能力 CAP_MARKETING（唯讀也要）。
  /// 回應附店家的活動配額與同時進行中的活動數。page_token 只能用在同一個 status。
  $async.Future<ListCampaignsResponse> listCampaigns(
          $pb.ClientContext? ctx, ListCampaignsRequest request) =>
      _client.invoke<ListCampaignsResponse>(ctx, 'CouponAppService',
          'ListCampaigns', request, ListCampaignsResponse());

  /// GetCampaign 讀店家自己的活動：名單（DESIGNATED）與各狀態的券數。需要賣家能力 CAP_MARKETING。不存在與不是這家店發的一律 not_found。
  $async.Future<GetCampaignResponse> getCampaign(
          $pb.ClientContext? ctx, GetCampaignRequest request) =>
      _client.invoke<GetCampaignResponse>(ctx, 'CouponAppService',
          'GetCampaign', request, GetCampaignResponse());

  /// EndCampaign 提前結束店家自己的活動：擋掉新的領取與鎖定，作廢還沒被使用的券（AVAILABLE），已被訂單鎖定的券照常核銷或釋放。收縮型動作，
  /// 需要賣家能力 CAP_CONTENT_CONTRACT（停權的店家仍可結束自己的活動）。已經結束的重送成功。不存在與不是這家店發的一律 not_found。
  $async.Future<EndCampaignResponse> endCampaign(
          $pb.ClientContext? ctx, EndCampaignRequest request) =>
      _client.invoke<EndCampaignResponse>(ctx, 'CouponAppService',
          'EndCampaign', request, EndCampaignResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
