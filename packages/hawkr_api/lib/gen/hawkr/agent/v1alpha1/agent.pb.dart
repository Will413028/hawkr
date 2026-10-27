// This is a generated file - do not edit.
//
// Generated from hawkr/agent/v1alpha1/agent.proto.

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

import 'agent.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'agent.pbenum.dart';

/// SessionVerification 是 session 的驗證紀錄。驗證的信箱就是邀請寄出的信箱，這裡不重複回傳。
class SessionVerification extends $pb.GeneratedMessage {
  factory SessionVerification({
    SessionVerificationMethod? method,
    $0.Timestamp? verifyTime,
  }) {
    final result = SessionVerification._();
    if (method != null) result.method = method;
    if (verifyTime != null) result.verifyTime = verifyTime;
    return result;
  }

  SessionVerification._();

  factory SessionVerification.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SessionVerification()..mergeFromBuffer(data, registry);
  factory SessionVerification.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SessionVerification()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SessionVerification',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: SessionVerification.$_createMessage)
    ..aE<SessionVerificationMethod>(1, _omitFieldNames ? '' : 'method',
        enumValues: SessionVerificationMethod.values)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'verifyTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionVerification clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SessionVerification copyWith(void Function(SessionVerification) updates) =>
      super.copyWith((message) => updates(message as SessionVerification))
          as SessionVerification;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SessionVerification() / SessionVerification.new instead')
  static SessionVerification create() => SessionVerification._();
  static $pb.GeneratedMessage $_createMessage() => SessionVerification._();
  @$core.override
  SessionVerification createEmptyInstance() => SessionVerification._();
  @$core.pragma('dart2js:noInline')
  static SessionVerification getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SessionVerification>(
          SessionVerification.$_createMessage);
  static SessionVerification? _defaultInstance;

  /// 驗證的方式。
  @$pb.TagNumber(1)
  SessionVerificationMethod get method => $_getN(0);
  @$pb.TagNumber(1)
  set method(SessionVerificationMethod value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMethod() => $_has(0);
  @$pb.TagNumber(1)
  void clearMethod() => $_clearField(1);

  /// 驗證完成的時間。
  @$pb.TagNumber(2)
  $0.Timestamp get verifyTime => $_getN(1);
  @$pb.TagNumber(2)
  set verifyTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasVerifyTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearVerifyTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureVerifyTime() => $_ensure(1);
}

class PreviewInviteRequest extends $pb.GeneratedMessage {
  factory PreviewInviteRequest({
    $core.String? secret,
  }) {
    final result = PreviewInviteRequest._();
    if (secret != null) result.secret = secret;
    return result;
  }

  PreviewInviteRequest._();

  factory PreviewInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewInviteRequest()..mergeFromBuffer(data, registry);
  factory PreviewInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewInviteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewInviteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: PreviewInviteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'secret')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewInviteRequest copyWith(void Function(PreviewInviteRequest) updates) =>
      super.copyWith((message) => updates(message as PreviewInviteRequest))
          as PreviewInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewInviteRequest() / PreviewInviteRequest.new instead')
  static PreviewInviteRequest create() => PreviewInviteRequest._();
  static $pb.GeneratedMessage $_createMessage() => PreviewInviteRequest._();
  @$core.override
  PreviewInviteRequest createEmptyInstance() => PreviewInviteRequest._();
  @$core.pragma('dart2js:noInline')
  static PreviewInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewInviteRequest>(
          PreviewInviteRequest.$_createMessage);
  static PreviewInviteRequest? _defaultInstance;

  /// 邀請信的連結帶的 secret：不透明的字串，client 不解析。
  @$pb.TagNumber(1)
  $core.String get secret => $_getSZ(0);
  @$pb.TagNumber(1)
  set secret($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSecret() => $_has(0);
  @$pb.TagNumber(1)
  void clearSecret() => $_clearField(1);
}

class PreviewInviteResponse extends $pb.GeneratedMessage {
  factory PreviewInviteResponse({
    $core.String? maskedEmail,
    $0.Timestamp? expireTime,
  }) {
    final result = PreviewInviteResponse._();
    if (maskedEmail != null) result.maskedEmail = maskedEmail;
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  PreviewInviteResponse._();

  factory PreviewInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewInviteResponse()..mergeFromBuffer(data, registry);
  factory PreviewInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewInviteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewInviteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: PreviewInviteResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'maskedEmail')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewInviteResponse copyWith(
          void Function(PreviewInviteResponse) updates) =>
      super.copyWith((message) => updates(message as PreviewInviteResponse))
          as PreviewInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewInviteResponse() / PreviewInviteResponse.new instead')
  static PreviewInviteResponse create() => PreviewInviteResponse._();
  static $pb.GeneratedMessage $_createMessage() => PreviewInviteResponse._();
  @$core.override
  PreviewInviteResponse createEmptyInstance() => PreviewInviteResponse._();
  @$core.pragma('dart2js:noInline')
  static PreviewInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewInviteResponse>(
          PreviewInviteResponse.$_createMessage);
  static PreviewInviteResponse? _defaultInstance;

  /// 遮罩後的 email（例如 v***@example.com，只留第一個字元與網域）。顯示的文字（例如品牌名稱）由 client 決定，不在這裡。
  @$pb.TagNumber(1)
  $core.String get maskedEmail => $_getSZ(0);
  @$pb.TagNumber(1)
  set maskedEmail($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMaskedEmail() => $_has(0);
  @$pb.TagNumber(1)
  void clearMaskedEmail() => $_clearField(1);

  /// 邀請的到期時間。
  @$pb.TagNumber(2)
  $0.Timestamp get expireTime => $_getN(1);
  @$pb.TagNumber(2)
  set expireTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasExpireTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpireTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureExpireTime() => $_ensure(1);
}

class ExchangeInviteRequest extends $pb.GeneratedMessage {
  factory ExchangeInviteRequest({
    $core.String? secret,
  }) {
    final result = ExchangeInviteRequest._();
    if (secret != null) result.secret = secret;
    return result;
  }

  ExchangeInviteRequest._();

  factory ExchangeInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeInviteRequest()..mergeFromBuffer(data, registry);
  factory ExchangeInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeInviteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExchangeInviteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: ExchangeInviteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'secret')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeInviteRequest copyWith(
          void Function(ExchangeInviteRequest) updates) =>
      super.copyWith((message) => updates(message as ExchangeInviteRequest))
          as ExchangeInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExchangeInviteRequest() / ExchangeInviteRequest.new instead')
  static ExchangeInviteRequest create() => ExchangeInviteRequest._();
  static $pb.GeneratedMessage $_createMessage() => ExchangeInviteRequest._();
  @$core.override
  ExchangeInviteRequest createEmptyInstance() => ExchangeInviteRequest._();
  @$core.pragma('dart2js:noInline')
  static ExchangeInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExchangeInviteRequest>(
          ExchangeInviteRequest.$_createMessage);
  static ExchangeInviteRequest? _defaultInstance;

  /// 邀請信的連結帶的 secret。
  @$pb.TagNumber(1)
  $core.String get secret => $_getSZ(0);
  @$pb.TagNumber(1)
  set secret($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSecret() => $_has(0);
  @$pb.TagNumber(1)
  void clearSecret() => $_clearField(1);
}

class ExchangeInviteResponse extends $pb.GeneratedMessage {
  factory ExchangeInviteResponse({
    $core.String? rfqRecipientId,
    SessionVerification? verification,
    $core.String? googleNonce,
  }) {
    final result = ExchangeInviteResponse._();
    if (rfqRecipientId != null) result.rfqRecipientId = rfqRecipientId;
    if (verification != null) result.verification = verification;
    if (googleNonce != null) result.googleNonce = googleNonce;
    return result;
  }

  ExchangeInviteResponse._();

  factory ExchangeInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeInviteResponse()..mergeFromBuffer(data, registry);
  factory ExchangeInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeInviteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExchangeInviteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: ExchangeInviteResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'rfqRecipientId')
    ..aOM<SessionVerification>(2, _omitFieldNames ? '' : 'verification',
        subBuilder: SessionVerification.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'googleNonce')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeInviteResponse copyWith(
          void Function(ExchangeInviteResponse) updates) =>
      super.copyWith((message) => updates(message as ExchangeInviteResponse))
          as ExchangeInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExchangeInviteResponse() / ExchangeInviteResponse.new instead')
  static ExchangeInviteResponse create() => ExchangeInviteResponse._();
  static $pb.GeneratedMessage $_createMessage() => ExchangeInviteResponse._();
  @$core.override
  ExchangeInviteResponse createEmptyInstance() => ExchangeInviteResponse._();
  @$core.pragma('dart2js:noInline')
  static ExchangeInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExchangeInviteResponse>(
          ExchangeInviteResponse.$_createMessage);
  static ExchangeInviteResponse? _defaultInstance;

  /// 這張邀請對應的 RFQ 收件人 ID（十進位整數字串，不透明）。session 的 token 只在 Set-Cookie，不在回應內容。
  @$pb.TagNumber(1)
  $core.String get rfqRecipientId => $_getSZ(0);
  @$pb.TagNumber(1)
  set rfqRecipientId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRfqRecipientId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRfqRecipientId() => $_clearField(1);

  /// 這個 session 的驗證紀錄；沿用既有 session 時是它目前的狀態，沒有（新發的 session 或還沒驗證）時不設。
  @$pb.TagNumber(2)
  SessionVerification get verification => $_getN(1);
  @$pb.TagNumber(2)
  set verification(SessionVerification value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasVerification() => $_has(1);
  @$pb.TagNumber(2)
  void clearVerification() => $_clearField(2);
  @$pb.TagNumber(2)
  SessionVerification ensureVerification() => $_ensure(1);

  /// 交給 Google 登入的 nonce（GSI 的 initialize 的 nonce）：VerifySession 收到的 ID token 必須帶它。client 不解析。
  @$pb.TagNumber(3)
  $core.String get googleNonce => $_getSZ(2);
  @$pb.TagNumber(3)
  set googleNonce($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGoogleNonce() => $_has(2);
  @$pb.TagNumber(3)
  void clearGoogleNonce() => $_clearField(3);
}

class GetSessionRequest extends $pb.GeneratedMessage {
  factory GetSessionRequest({
    $core.String? rfqRecipientId,
  }) {
    final result = GetSessionRequest._();
    if (rfqRecipientId != null) result.rfqRecipientId = rfqRecipientId;
    return result;
  }

  GetSessionRequest._();

  factory GetSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSessionRequest()..mergeFromBuffer(data, registry);
  factory GetSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: GetSessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'rfqRecipientId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSessionRequest copyWith(void Function(GetSessionRequest) updates) =>
      super.copyWith((message) => updates(message as GetSessionRequest))
          as GetSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetSessionRequest() / GetSessionRequest.new instead')
  static GetSessionRequest create() => GetSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetSessionRequest._();
  @$core.override
  GetSessionRequest createEmptyInstance() => GetSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static GetSessionRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetSessionRequest>(
          GetSessionRequest.$_createMessage);
  static GetSessionRequest? _defaultInstance;

  /// 要讀哪一張邀請的 session：RFQ 收件人 ID（十進位整數字串，不透明）。
  @$pb.TagNumber(1)
  $core.String get rfqRecipientId => $_getSZ(0);
  @$pb.TagNumber(1)
  set rfqRecipientId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRfqRecipientId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRfqRecipientId() => $_clearField(1);
}

class GetSessionResponse extends $pb.GeneratedMessage {
  factory GetSessionResponse({
    SessionVerification? verification,
    $core.String? googleNonce,
  }) {
    final result = GetSessionResponse._();
    if (verification != null) result.verification = verification;
    if (googleNonce != null) result.googleNonce = googleNonce;
    return result;
  }

  GetSessionResponse._();

  factory GetSessionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSessionResponse()..mergeFromBuffer(data, registry);
  factory GetSessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSessionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: GetSessionResponse.$_createMessage)
    ..aOM<SessionVerification>(1, _omitFieldNames ? '' : 'verification',
        subBuilder: SessionVerification.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'googleNonce')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSessionResponse copyWith(void Function(GetSessionResponse) updates) =>
      super.copyWith((message) => updates(message as GetSessionResponse))
          as GetSessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetSessionResponse() / GetSessionResponse.new instead')
  static GetSessionResponse create() => GetSessionResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetSessionResponse._();
  @$core.override
  GetSessionResponse createEmptyInstance() => GetSessionResponse._();
  @$core.pragma('dart2js:noInline')
  static GetSessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSessionResponse>(
          GetSessionResponse.$_createMessage);
  static GetSessionResponse? _defaultInstance;

  /// 這個 session 的驗證紀錄；還沒驗證時不設。
  @$pb.TagNumber(1)
  SessionVerification get verification => $_getN(0);
  @$pb.TagNumber(1)
  set verification(SessionVerification value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVerification() => $_has(0);
  @$pb.TagNumber(1)
  void clearVerification() => $_clearField(1);
  @$pb.TagNumber(1)
  SessionVerification ensureVerification() => $_ensure(0);

  /// 交給 Google 登入的 nonce，同 ExchangeInviteResponse.google_nonce。
  @$pb.TagNumber(2)
  $core.String get googleNonce => $_getSZ(1);
  @$pb.TagNumber(2)
  set googleNonce($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGoogleNonce() => $_has(1);
  @$pb.TagNumber(2)
  void clearGoogleNonce() => $_clearField(2);
}

class VerifySessionRequest extends $pb.GeneratedMessage {
  factory VerifySessionRequest({
    $core.String? rfqRecipientId,
    $core.String? googleIdToken,
  }) {
    final result = VerifySessionRequest._();
    if (rfqRecipientId != null) result.rfqRecipientId = rfqRecipientId;
    if (googleIdToken != null) result.googleIdToken = googleIdToken;
    return result;
  }

  VerifySessionRequest._();

  factory VerifySessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      VerifySessionRequest()..mergeFromBuffer(data, registry);
  factory VerifySessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      VerifySessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerifySessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: VerifySessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'rfqRecipientId')
    ..aOS(2, _omitFieldNames ? '' : 'googleIdToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifySessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifySessionRequest copyWith(void Function(VerifySessionRequest) updates) =>
      super.copyWith((message) => updates(message as VerifySessionRequest))
          as VerifySessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use VerifySessionRequest() / VerifySessionRequest.new instead')
  static VerifySessionRequest create() => VerifySessionRequest._();
  static $pb.GeneratedMessage $_createMessage() => VerifySessionRequest._();
  @$core.override
  VerifySessionRequest createEmptyInstance() => VerifySessionRequest._();
  @$core.pragma('dart2js:noInline')
  static VerifySessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerifySessionRequest>(
          VerifySessionRequest.$_createMessage);
  static VerifySessionRequest? _defaultInstance;

  /// 要驗證哪一張邀請的 session：RFQ 收件人 ID（十進位整數字串，不透明）。
  @$pb.TagNumber(1)
  $core.String get rfqRecipientId => $_getSZ(0);
  @$pb.TagNumber(1)
  set rfqRecipientId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRfqRecipientId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRfqRecipientId() => $_clearField(1);

  /// Google 登入取得的 ID token（JWT）。
  @$pb.TagNumber(2)
  $core.String get googleIdToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set googleIdToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGoogleIdToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearGoogleIdToken() => $_clearField(2);
}

class VerifySessionResponse extends $pb.GeneratedMessage {
  factory VerifySessionResponse({
    $core.String? rfqRecipientId,
    SessionVerification? verification,
  }) {
    final result = VerifySessionResponse._();
    if (rfqRecipientId != null) result.rfqRecipientId = rfqRecipientId;
    if (verification != null) result.verification = verification;
    return result;
  }

  VerifySessionResponse._();

  factory VerifySessionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      VerifySessionResponse()..mergeFromBuffer(data, registry);
  factory VerifySessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      VerifySessionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerifySessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.agent.v1alpha1'),
      createEmptyInstance: VerifySessionResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'rfqRecipientId')
    ..aOM<SessionVerification>(2, _omitFieldNames ? '' : 'verification',
        subBuilder: SessionVerification.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifySessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifySessionResponse copyWith(
          void Function(VerifySessionResponse) updates) =>
      super.copyWith((message) => updates(message as VerifySessionResponse))
          as VerifySessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use VerifySessionResponse() / VerifySessionResponse.new instead')
  static VerifySessionResponse create() => VerifySessionResponse._();
  static $pb.GeneratedMessage $_createMessage() => VerifySessionResponse._();
  @$core.override
  VerifySessionResponse createEmptyInstance() => VerifySessionResponse._();
  @$core.pragma('dart2js:noInline')
  static VerifySessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerifySessionResponse>(
          VerifySessionResponse.$_createMessage);
  static VerifySessionResponse? _defaultInstance;

  /// 這個 session 對應的 RFQ 收件人 ID（十進位整數字串，不透明）。
  @$pb.TagNumber(1)
  $core.String get rfqRecipientId => $_getSZ(0);
  @$pb.TagNumber(1)
  set rfqRecipientId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRfqRecipientId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRfqRecipientId() => $_clearField(1);

  /// 這個 session 的驗證紀錄。
  @$pb.TagNumber(2)
  SessionVerification get verification => $_getN(1);
  @$pb.TagNumber(2)
  set verification(SessionVerification value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasVerification() => $_has(1);
  @$pb.TagNumber(2)
  void clearVerification() => $_clearField(2);
  @$pb.TagNumber(2)
  SessionVerification ensureVerification() => $_ensure(1);
}

/// VendorPortalPublicService 是持有邀請的供應商入口，不需要登入帳號。兩個方法都是 POST：憑證（邀請 secret）放在請求訊息，
/// 不進 URL（API 設計規範（Connect）§三），所以回應一律不可快取。兌換之後的方法在 VendorPortalInviteeService。
class VendorPortalPublicServiceApi {
  final $pb.RpcClient _client;

  VendorPortalPublicServiceApi(this._client);

  /// PreviewInvite 回傳遮罩後的 email 與邀請的到期時間，讓供應商在兌換之前確認這是寄給自己的邀請；沒有副作用，也不發 session。
  /// 邀請不可用（不存在、secret 格式不對、已到期、已撤銷）一律回同一個 not_found，猜 secret 的人分不出原因。
  /// 與 ExchangeInvite 共用同一份額度：每個來源 5 分鐘最多 20 次，超過回 resource_exhausted（帶 RetryInfo）。
  $async.Future<PreviewInviteResponse> previewInvite(
          $pb.ClientContext? ctx, PreviewInviteRequest request) =>
      _client.invoke<PreviewInviteResponse>(ctx, 'VendorPortalPublicService',
          'PreviewInvite', request, PreviewInviteResponse());

  /// ExchangeInvite 以邀請 secret 取得這張邀請的 session，並在回應的 Set-Cookie 設上 session 的 cookie（名稱 __Host-hk_vp_<
  /// rfq_recipient_id>；HttpOnly、Secure、SameSite=Lax；Max-Age 是 session 到絕對期限的剩餘秒數，不超過邀請的到期時間）。
  /// 請求已帶這張邀請的有效 session 的 cookie 時沿用它：不發新的 session，回應是那個 session 目前的驗證狀態與 google_nonce；
  /// 沒有 cookie 或 cookie 無效時發新的 session（新的 Set-Cookie 蓋掉舊的）。邀請到期或撤銷之前可以重複兌換；同一張邀請同時
  /// 最多 5 個有效 session，超過時撤銷最舊的未驗證 session，全部都已驗證才撤銷最舊的，本次新發的不在撤銷之列。請求帶的邀請
  /// cookie 超過 20 個時，回應以 Max-Age=0 清掉多出的。錯誤與額度同 PreviewInvite。
  $async.Future<ExchangeInviteResponse> exchangeInvite(
          $pb.ClientContext? ctx, ExchangeInviteRequest request) =>
      _client.invoke<ExchangeInviteResponse>(ctx, 'VendorPortalPublicService',
          'ExchangeInvite', request, ExchangeInviteResponse());
}

/// VendorPortalInviteeService 是已經兌換邀請的供應商（受邀供應商）：憑證是 ExchangeInvite 設的 cookie，沒有登入帳號。瀏覽器
/// 可以同時持有多張邀請的 cookie，每個方法以 rfq_recipient_id 指名要用哪一張（cookie 名稱含它）；沒有那張邀請的 cookie、
/// session 無效、已過期、已撤銷時回 unauthenticated。回應一律不可快取（private, no-store），讀取方法可以用 GET。
class VendorPortalInviteeServiceApi {
  final $pb.RpcClient _client;

  VendorPortalInviteeServiceApi(this._client);

  /// GetSession 回傳這張邀請的 session 目前的驗證狀態，以及要交給 Google 登入的 google_nonce。client 重新整理之後以它還原狀態。
  /// 每個來源每分鐘最多 300 次，超過回 resource_exhausted（帶 RetryInfo）。
  $async.Future<GetSessionResponse> getSession(
          $pb.ClientContext? ctx, GetSessionRequest request) =>
      _client.invoke<GetSessionResponse>(ctx, 'VendorPortalInviteeService',
          'GetSession', request, GetSessionResponse());

  /// VerifySession 以 Google 登入證明供應商持有邀請的信箱（step-up）：google_id_token 由伺服器驗證，信箱以 Google 驗證過的
  /// 為準，不接受 client 自報；Google 帳號的信箱必須已經驗證，而且與邀請寄出的信箱相同（去前後空白、不分大小寫）。token 的
  /// nonce 必須是這個 session 的 google_nonce，iat 不得早於 10 分鐘前。驗證紀錄綁在 rfq_recipient_id 指名的 session。同一個
  /// session 以相同的信箱重送是冪等的成功。
  ///
  /// Google token 無效、nonce 不符、token 太舊、信箱未驗證、信箱不符、session 已用別的信箱驗證一律回同一個 permission_denied
  /// （同一段訊息），分不出原因。Google 的公鑰暫時問不到時回 unavailable，可以稍後重試。每個來源 10 分鐘最多 10 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  $async.Future<VerifySessionResponse> verifySession(
          $pb.ClientContext? ctx, VerifySessionRequest request) =>
      _client.invoke<VerifySessionResponse>(ctx, 'VendorPortalInviteeService',
          'VerifySession', request, VerifySessionResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
