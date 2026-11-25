// This is a generated file - do not edit.
//
// Generated from hawkr/payment/v1alpha1/payment.proto.

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
    as $0;

import 'payment.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'payment.pbenum.dart';

/// Payment 是買家對一張訂單的付款（order_no 是請求的那張訂單）。
class Payment extends $pb.GeneratedMessage {
  factory Payment({
    $core.String? orderNo,
    PaymentStatus? status,
    $fixnum.Int64? amount,
    $0.Timestamp? paidTime,
    PaymentSession? session,
  }) {
    final result = Payment._();
    if (orderNo != null) result.orderNo = orderNo;
    if (status != null) result.status = status;
    if (amount != null) result.amount = amount;
    if (paidTime != null) result.paidTime = paidTime;
    if (session != null) result.session = session;
    return result;
  }

  Payment._();

  factory Payment.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Payment()..mergeFromBuffer(data, registry);
  factory Payment.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Payment()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Payment',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: Payment.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'orderNo')
    ..aE<PaymentStatus>(2, _omitFieldNames ? '' : 'status',
        enumValues: PaymentStatus.values)
    ..aInt64(3, _omitFieldNames ? '' : 'amount')
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'paidTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<PaymentSession>(5, _omitFieldNames ? '' : 'session',
        subBuilder: PaymentSession.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Payment clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Payment copyWith(void Function(Payment) updates) =>
      super.copyWith((message) => updates(message as Payment)) as Payment;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Payment() / Payment.new instead')
  static Payment create() => Payment._();
  static $pb.GeneratedMessage $_createMessage() => Payment._();
  @$core.override
  Payment createEmptyInstance() => Payment._();
  @$core.pragma('dart2js:noInline')
  static Payment getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Payment>(Payment.$_createMessage);
  static Payment? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get orderNo => $_getSZ(0);
  @$pb.TagNumber(1)
  set orderNo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOrderNo() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrderNo() => $_clearField(1);

  @$pb.TagNumber(2)
  PaymentStatus get status => $_getN(1);
  @$pb.TagNumber(2)
  set status(PaymentStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  /// 付款金額（這筆付款的總額，不帶幣別；一次付一張訂單時就是訂單的金額）。
  @$pb.TagNumber(3)
  $fixnum.Int64 get amount => $_getI64(2);
  @$pb.TagNumber(3)
  set amount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAmount() => $_has(2);
  @$pb.TagNumber(3)
  void clearAmount() => $_clearField(3);

  /// 入帳時間；沒有入帳時不設。
  @$pb.TagNumber(4)
  $0.Timestamp get paidTime => $_getN(3);
  @$pb.TagNumber(4)
  set paidTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPaidTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearPaidTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensurePaidTime() => $_ensure(3);

  /// 完成付款所需的初始化資料；只有狀態 AWAITING_PAYMENT 且金流商還在等待付款時才有。
  @$pb.TagNumber(5)
  PaymentSession get session => $_getN(4);
  @$pb.TagNumber(5)
  set session(PaymentSession value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSession() => $_has(4);
  @$pb.TagNumber(5)
  void clearSession() => $_clearField(5);
  @$pb.TagNumber(5)
  PaymentSession ensureSession() => $_ensure(4);
}

enum PaymentSession_Provider { stripe, notSet }

/// PaymentSession 是完成付款的初始化資料，依金流商分支。
class PaymentSession extends $pb.GeneratedMessage {
  factory PaymentSession({
    StripePaymentSession? stripe,
  }) {
    final result = PaymentSession._();
    if (stripe != null) result.stripe = stripe;
    return result;
  }

  PaymentSession._();

  factory PaymentSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PaymentSession()..mergeFromBuffer(data, registry);
  factory PaymentSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PaymentSession()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, PaymentSession_Provider>
      _PaymentSession_ProviderByTag = {
    1: PaymentSession_Provider.stripe,
    0: PaymentSession_Provider.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PaymentSession',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: PaymentSession.$_createMessage)
    ..oo(0, [1])
    ..aOM<StripePaymentSession>(1, _omitFieldNames ? '' : 'stripe',
        subBuilder: StripePaymentSession.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentSession copyWith(void Function(PaymentSession) updates) =>
      super.copyWith((message) => updates(message as PaymentSession))
          as PaymentSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PaymentSession() / PaymentSession.new instead')
  static PaymentSession create() => PaymentSession._();
  static $pb.GeneratedMessage $_createMessage() => PaymentSession._();
  @$core.override
  PaymentSession createEmptyInstance() => PaymentSession._();
  @$core.pragma('dart2js:noInline')
  static PaymentSession getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PaymentSession>(
          PaymentSession.$_createMessage);
  static PaymentSession? _defaultInstance;

  @$pb.TagNumber(1)
  PaymentSession_Provider whichProvider() =>
      _PaymentSession_ProviderByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  void clearProvider() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  StripePaymentSession get stripe => $_getN(0);
  @$pb.TagNumber(1)
  set stripe(StripePaymentSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStripe() => $_has(0);
  @$pb.TagNumber(1)
  void clearStripe() => $_clearField(1);
  @$pb.TagNumber(1)
  StripePaymentSession ensureStripe() => $_ensure(0);
}

/// StripePaymentSession 是 Stripe Payment Element（web）與 PaymentSheet（App）的初始化資料。
class StripePaymentSession extends $pb.GeneratedMessage {
  factory StripePaymentSession({
    $core.String? clientSecret,
    $core.String? publishableKey,
  }) {
    final result = StripePaymentSession._();
    if (clientSecret != null) result.clientSecret = clientSecret;
    if (publishableKey != null) result.publishableKey = publishableKey;
    return result;
  }

  StripePaymentSession._();

  factory StripePaymentSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StripePaymentSession()..mergeFromBuffer(data, registry);
  factory StripePaymentSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StripePaymentSession()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StripePaymentSession',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: StripePaymentSession.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'clientSecret')
    ..aOS(2, _omitFieldNames ? '' : 'publishableKey')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StripePaymentSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StripePaymentSession copyWith(void Function(StripePaymentSession) updates) =>
      super.copyWith((message) => updates(message as StripePaymentSession))
          as StripePaymentSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use StripePaymentSession() / StripePaymentSession.new instead')
  static StripePaymentSession create() => StripePaymentSession._();
  static $pb.GeneratedMessage $_createMessage() => StripePaymentSession._();
  @$core.override
  StripePaymentSession createEmptyInstance() => StripePaymentSession._();
  @$core.pragma('dart2js:noInline')
  static StripePaymentSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StripePaymentSession>(
          StripePaymentSession.$_createMessage);
  static StripePaymentSession? _defaultInstance;

  /// PaymentIntent 的 client secret；只給這個買家的 client，不記錄、不快取。
  @$pb.TagNumber(1)
  $core.String get clientSecret => $_getSZ(0);
  @$pb.TagNumber(1)
  set clientSecret($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasClientSecret() => $_has(0);
  @$pb.TagNumber(1)
  void clearClientSecret() => $_clearField(1);

  /// 初始化 Stripe.js 與 PaymentSheet 用的公開金鑰（不是 secret）。
  @$pb.TagNumber(2)
  $core.String get publishableKey => $_getSZ(1);
  @$pb.TagNumber(2)
  set publishableKey($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublishableKey() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublishableKey() => $_clearField(2);
}

class CreatePaymentRequest extends $pb.GeneratedMessage {
  factory CreatePaymentRequest({
    $core.String? orderNo,
  }) {
    final result = CreatePaymentRequest._();
    if (orderNo != null) result.orderNo = orderNo;
    return result;
  }

  CreatePaymentRequest._();

  factory CreatePaymentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePaymentRequest()..mergeFromBuffer(data, registry);
  factory CreatePaymentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePaymentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePaymentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: CreatePaymentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'orderNo')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePaymentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePaymentRequest copyWith(void Function(CreatePaymentRequest) updates) =>
      super.copyWith((message) => updates(message as CreatePaymentRequest))
          as CreatePaymentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreatePaymentRequest() / CreatePaymentRequest.new instead')
  static CreatePaymentRequest create() => CreatePaymentRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreatePaymentRequest._();
  @$core.override
  CreatePaymentRequest createEmptyInstance() => CreatePaymentRequest._();
  @$core.pragma('dart2js:noInline')
  static CreatePaymentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreatePaymentRequest>(
          CreatePaymentRequest.$_createMessage);
  static CreatePaymentRequest? _defaultInstance;

  /// 要付款的訂單編號。
  @$pb.TagNumber(1)
  $core.String get orderNo => $_getSZ(0);
  @$pb.TagNumber(1)
  set orderNo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOrderNo() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrderNo() => $_clearField(1);
}

class CreatePaymentResponse extends $pb.GeneratedMessage {
  factory CreatePaymentResponse({
    Payment? payment,
  }) {
    final result = CreatePaymentResponse._();
    if (payment != null) result.payment = payment;
    return result;
  }

  CreatePaymentResponse._();

  factory CreatePaymentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePaymentResponse()..mergeFromBuffer(data, registry);
  factory CreatePaymentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePaymentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePaymentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: CreatePaymentResponse.$_createMessage)
    ..aOM<Payment>(1, _omitFieldNames ? '' : 'payment',
        subBuilder: Payment.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePaymentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePaymentResponse copyWith(
          void Function(CreatePaymentResponse) updates) =>
      super.copyWith((message) => updates(message as CreatePaymentResponse))
          as CreatePaymentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreatePaymentResponse() / CreatePaymentResponse.new instead')
  static CreatePaymentResponse create() => CreatePaymentResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreatePaymentResponse._();
  @$core.override
  CreatePaymentResponse createEmptyInstance() => CreatePaymentResponse._();
  @$core.pragma('dart2js:noInline')
  static CreatePaymentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreatePaymentResponse>(
          CreatePaymentResponse.$_createMessage);
  static CreatePaymentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Payment get payment => $_getN(0);
  @$pb.TagNumber(1)
  set payment(Payment value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPayment() => $_has(0);
  @$pb.TagNumber(1)
  void clearPayment() => $_clearField(1);
  @$pb.TagNumber(1)
  Payment ensurePayment() => $_ensure(0);
}

class GetPaymentRequest extends $pb.GeneratedMessage {
  factory GetPaymentRequest({
    $core.String? orderNo,
  }) {
    final result = GetPaymentRequest._();
    if (orderNo != null) result.orderNo = orderNo;
    return result;
  }

  GetPaymentRequest._();

  factory GetPaymentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPaymentRequest()..mergeFromBuffer(data, registry);
  factory GetPaymentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPaymentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPaymentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: GetPaymentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'orderNo')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPaymentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPaymentRequest copyWith(void Function(GetPaymentRequest) updates) =>
      super.copyWith((message) => updates(message as GetPaymentRequest))
          as GetPaymentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPaymentRequest() / GetPaymentRequest.new instead')
  static GetPaymentRequest create() => GetPaymentRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetPaymentRequest._();
  @$core.override
  GetPaymentRequest createEmptyInstance() => GetPaymentRequest._();
  @$core.pragma('dart2js:noInline')
  static GetPaymentRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetPaymentRequest>(
          GetPaymentRequest.$_createMessage);
  static GetPaymentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get orderNo => $_getSZ(0);
  @$pb.TagNumber(1)
  set orderNo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOrderNo() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrderNo() => $_clearField(1);
}

class GetPaymentResponse extends $pb.GeneratedMessage {
  factory GetPaymentResponse({
    Payment? payment,
  }) {
    final result = GetPaymentResponse._();
    if (payment != null) result.payment = payment;
    return result;
  }

  GetPaymentResponse._();

  factory GetPaymentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPaymentResponse()..mergeFromBuffer(data, registry);
  factory GetPaymentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPaymentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPaymentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: GetPaymentResponse.$_createMessage)
    ..aOM<Payment>(1, _omitFieldNames ? '' : 'payment',
        subBuilder: Payment.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPaymentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPaymentResponse copyWith(void Function(GetPaymentResponse) updates) =>
      super.copyWith((message) => updates(message as GetPaymentResponse))
          as GetPaymentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPaymentResponse() / GetPaymentResponse.new instead')
  static GetPaymentResponse create() => GetPaymentResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetPaymentResponse._();
  @$core.override
  GetPaymentResponse createEmptyInstance() => GetPaymentResponse._();
  @$core.pragma('dart2js:noInline')
  static GetPaymentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPaymentResponse>(
          GetPaymentResponse.$_createMessage);
  static GetPaymentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Payment get payment => $_getN(0);
  @$pb.TagNumber(1)
  set payment(Payment value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPayment() => $_has(0);
  @$pb.TagNumber(1)
  void clearPayment() => $_clearField(1);
  @$pb.TagNumber(1)
  Payment ensurePayment() => $_ensure(0);
}

/// EarningsEntry 是賣家入帳帳本的一列：一張訂單的一次轉入、收回或補差額。不帶金流商的物件 ID 與失敗原因。
class EarningsEntry extends $pb.GeneratedMessage {
  factory EarningsEntry({
    $core.String? id,
    EarningsEntryType? type,
    $core.String? orderNo,
    $fixnum.Int64? amount,
    EarningsEntryStatus? status,
    $0.Timestamp? createTime,
  }) {
    final result = EarningsEntry._();
    if (id != null) result.id = id;
    if (type != null) result.type = type;
    if (orderNo != null) result.orderNo = orderNo;
    if (amount != null) result.amount = amount;
    if (status != null) result.status = status;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  EarningsEntry._();

  factory EarningsEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EarningsEntry()..mergeFromBuffer(data, registry);
  factory EarningsEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EarningsEntry()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EarningsEntry',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.payment.v1alpha1'),
      createEmptyInstance: EarningsEntry.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<EarningsEntryType>(2, _omitFieldNames ? '' : 'type',
        enumValues: EarningsEntryType.values)
    ..aOS(3, _omitFieldNames ? '' : 'orderNo')
    ..aInt64(4, _omitFieldNames ? '' : 'amount')
    ..aE<EarningsEntryStatus>(5, _omitFieldNames ? '' : 'status',
        enumValues: EarningsEntryStatus.values)
    ..aOM<$0.Timestamp>(6, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EarningsEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EarningsEntry copyWith(void Function(EarningsEntry) updates) =>
      super.copyWith((message) => updates(message as EarningsEntry))
          as EarningsEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use EarningsEntry() / EarningsEntry.new instead')
  static EarningsEntry create() => EarningsEntry._();
  static $pb.GeneratedMessage $_createMessage() => EarningsEntry._();
  @$core.override
  EarningsEntry createEmptyInstance() => EarningsEntry._();
  @$core.pragma('dart2js:noInline')
  static EarningsEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<EarningsEntry>(
          EarningsEntry.$_createMessage);
  static EarningsEntry? _defaultInstance;

  /// 列的識別（轉帳、收回或補差額各自的紀錄 ID），不是訂單編號；補差額的 ID 是所屬退款的 ID。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  EarningsEntryType get type => $_getN(1);
  @$pb.TagNumber(2)
  set type(EarningsEntryType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get orderNo => $_getSZ(2);
  @$pb.TagNumber(3)
  set orderNo($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOrderNo() => $_has(2);
  @$pb.TagNumber(3)
  void clearOrderNo() => $_clearField(3);

  /// 新台幣元，帶正負：TRANSFER 與 TOP_UP 為正，REVERSAL 為負。
  @$pb.TagNumber(4)
  $fixnum.Int64 get amount => $_getI64(3);
  @$pb.TagNumber(4)
  set amount($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAmount() => $_has(3);
  @$pb.TagNumber(4)
  void clearAmount() => $_clearField(4);

  @$pb.TagNumber(5)
  EarningsEntryStatus get status => $_getN(4);
  @$pb.TagNumber(5)
  set status(EarningsEntryStatus value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStatus() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatus() => $_clearField(5);

  /// 這一列記錄的時間（補差額是所屬退款的記錄時間）。
  @$pb.TagNumber(6)
  $0.Timestamp get createTime => $_getN(5);
  @$pb.TagNumber(6)
  set createTime($0.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCreateTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreateTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $0.Timestamp ensureCreateTime() => $_ensure(5);
}

/// PaymentAppService 只有登入的顧客帳號（買家）能呼叫；只讀寫呼叫者自己的付款（他人的訂單回 not_found）。
class PaymentAppServiceApi {
  final $pb.RpcClient _client;

  PaymentAppServiceApi(this._client);

  /// CreatePayment 對買家自己的一張待付款訂單建立付款（冪等：同一張訂單只有一筆付款、同時只有一個有效的 PaymentIntent，重複呼叫回目前的付款）。
  /// 訂單不是呼叫者的回 not_found；訂單不可付（已取消、已付款以外的狀態）、零元訂單、金額或店家與既有付款不同回 failed_precondition；沒有設定付款 provider 回 failed_precondition。
  /// provider 暫時失敗或逾時時回狀態 CREATING 的付款（沒有 session），背景補做之後以 GetPayment 重讀；provider 永久拒絕時回狀態 CREATE_FAILED（不是錯誤），訂單仍可付時再呼叫一次會換新的嘗試。
  /// 狀態 AWAITING_PAYMENT 時帶 session。
  $async.Future<CreatePaymentResponse> createPayment(
          $pb.ClientContext? ctx, CreatePaymentRequest request) =>
      _client.invoke<CreatePaymentResponse>(ctx, 'PaymentAppService',
          'CreatePayment', request, CreatePaymentResponse());

  /// GetPayment 讀買家自己的付款（沒有付款或不是呼叫者的回 not_found）。狀態 AWAITING_PAYMENT 時帶 session（client secret 是每次向 provider 讀的，不存）；
  /// provider 那邊已往前（買家剛付款）時不帶 session、狀態仍是 AWAITING_PAYMENT，背景同步後再讀會是 PROCESSING 或 SUCCEEDED。回應含 client secret，不標 NO_SIDE_EFFECTS（那會允許 GET 與快取），所以一律 POST。
  $async.Future<GetPaymentResponse> getPayment(
          $pb.ClientContext? ctx, GetPaymentRequest request) =>
      _client.invoke<GetPaymentResponse>(ctx, 'PaymentAppService', 'GetPayment',
          request, GetPaymentResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
