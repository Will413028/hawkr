// This is a generated file - do not edit.
//
// Generated from hawkr/identity/v1alpha1/identity.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

import 'identity.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'identity.pbenum.dart';

/// Account 是登入的帳號。
class Account extends $pb.GeneratedMessage {
  factory Account({
    $core.String? id,
    $core.Iterable<Role>? roles,
  }) {
    final result = Account._();
    if (id != null) result.id = id;
    if (roles != null) result.roles.addAll(roles);
    return result;
  }

  Account._();

  factory Account.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Account()..mergeFromBuffer(data, registry);
  factory Account.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Account()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Account',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: Account.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..pc<Role>(2, _omitFieldNames ? '' : 'roles', $pb.PbFieldType.KE,
        valueOf: Role.valueOf,
        enumValues: Role.values,
        defaultEnumValue: Role.ROLE_UNSPECIFIED)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Account clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Account copyWith(void Function(Account) updates) =>
      super.copyWith((message) => updates(message as Account)) as Account;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Account() / Account.new instead')
  static Account create() => Account._();
  static $pb.GeneratedMessage $_createMessage() => Account._();
  @$core.override
  Account createEmptyInstance() => Account._();
  @$core.pragma('dart2js:noInline')
  static Account getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Account>(Account.$_createMessage);
  static Account? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 帳號另外申請的角色（目前只有 ROLE_SELLER）；沒有時是空的。買家不列出：每個帳號都是買家。
  @$pb.TagNumber(2)
  $pb.PbList<Role> get roles => $_getList(1);
}

class IssueAppLoginNonceRequest extends $pb.GeneratedMessage {
  factory IssueAppLoginNonceRequest() => IssueAppLoginNonceRequest._();

  IssueAppLoginNonceRequest._();

  factory IssueAppLoginNonceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IssueAppLoginNonceRequest()..mergeFromBuffer(data, registry);
  factory IssueAppLoginNonceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IssueAppLoginNonceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IssueAppLoginNonceRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: IssueAppLoginNonceRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueAppLoginNonceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueAppLoginNonceRequest copyWith(
          void Function(IssueAppLoginNonceRequest) updates) =>
      super.copyWith((message) => updates(message as IssueAppLoginNonceRequest))
          as IssueAppLoginNonceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use IssueAppLoginNonceRequest() / IssueAppLoginNonceRequest.new instead')
  static IssueAppLoginNonceRequest create() => IssueAppLoginNonceRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      IssueAppLoginNonceRequest._();
  @$core.override
  IssueAppLoginNonceRequest createEmptyInstance() =>
      IssueAppLoginNonceRequest._();
  @$core.pragma('dart2js:noInline')
  static IssueAppLoginNonceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IssueAppLoginNonceRequest>(
          IssueAppLoginNonceRequest.$_createMessage);
  static IssueAppLoginNonceRequest? _defaultInstance;
}

class IssueAppLoginNonceResponse extends $pb.GeneratedMessage {
  factory IssueAppLoginNonceResponse({
    $core.String? nonce,
    $0.Timestamp? expireTime,
  }) {
    final result = IssueAppLoginNonceResponse._();
    if (nonce != null) result.nonce = nonce;
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  IssueAppLoginNonceResponse._();

  factory IssueAppLoginNonceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IssueAppLoginNonceResponse()..mergeFromBuffer(data, registry);
  factory IssueAppLoginNonceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      IssueAppLoginNonceResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IssueAppLoginNonceResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: IssueAppLoginNonceResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'nonce')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueAppLoginNonceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueAppLoginNonceResponse copyWith(
          void Function(IssueAppLoginNonceResponse) updates) =>
      super.copyWith(
              (message) => updates(message as IssueAppLoginNonceResponse))
          as IssueAppLoginNonceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use IssueAppLoginNonceResponse() / IssueAppLoginNonceResponse.new instead')
  static IssueAppLoginNonceResponse create() => IssueAppLoginNonceResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      IssueAppLoginNonceResponse._();
  @$core.override
  IssueAppLoginNonceResponse createEmptyInstance() =>
      IssueAppLoginNonceResponse._();
  @$core.pragma('dart2js:noInline')
  static IssueAppLoginNonceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IssueAppLoginNonceResponse>(
          IssueAppLoginNonceResponse.$_createMessage);
  static IssueAppLoginNonceResponse? _defaultInstance;

  /// 單次使用的 nonce：不透明的字串，client 不解析。
  @$pb.TagNumber(1)
  $core.String get nonce => $_getSZ(0);
  @$pb.TagNumber(1)
  set nonce($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNonce() => $_has(0);
  @$pb.TagNumber(1)
  void clearNonce() => $_clearField(1);

  /// nonce 的到期時間（發出後 5 分鐘）。
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

class ExchangeIdTokenRequest extends $pb.GeneratedMessage {
  factory ExchangeIdTokenRequest({
    LoginProvider? provider,
    $core.String? idToken,
    $core.String? nonce,
  }) {
    final result = ExchangeIdTokenRequest._();
    if (provider != null) result.provider = provider;
    if (idToken != null) result.idToken = idToken;
    if (nonce != null) result.nonce = nonce;
    return result;
  }

  ExchangeIdTokenRequest._();

  factory ExchangeIdTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeIdTokenRequest()..mergeFromBuffer(data, registry);
  factory ExchangeIdTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeIdTokenRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExchangeIdTokenRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ExchangeIdTokenRequest.$_createMessage)
    ..aE<LoginProvider>(1, _omitFieldNames ? '' : 'provider',
        enumValues: LoginProvider.values)
    ..aOS(2, _omitFieldNames ? '' : 'idToken')
    ..aOS(3, _omitFieldNames ? '' : 'nonce')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeIdTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeIdTokenRequest copyWith(
          void Function(ExchangeIdTokenRequest) updates) =>
      super.copyWith((message) => updates(message as ExchangeIdTokenRequest))
          as ExchangeIdTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExchangeIdTokenRequest() / ExchangeIdTokenRequest.new instead')
  static ExchangeIdTokenRequest create() => ExchangeIdTokenRequest._();
  static $pb.GeneratedMessage $_createMessage() => ExchangeIdTokenRequest._();
  @$core.override
  ExchangeIdTokenRequest createEmptyInstance() => ExchangeIdTokenRequest._();
  @$core.pragma('dart2js:noInline')
  static ExchangeIdTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExchangeIdTokenRequest>(
          ExchangeIdTokenRequest.$_createMessage);
  static ExchangeIdTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  LoginProvider get provider => $_getN(0);
  @$pb.TagNumber(1)
  set provider(LoginProvider value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProvider() => $_has(0);
  @$pb.TagNumber(1)
  void clearProvider() => $_clearField(1);

  /// 原生 SDK 簽發的 ID token。
  @$pb.TagNumber(2)
  $core.String get idToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set idToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIdToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdToken() => $_clearField(2);

  /// IssueAppLoginNonce 發的 nonce，必須等於 ID token 的 nonce claim。
  @$pb.TagNumber(3)
  $core.String get nonce => $_getSZ(2);
  @$pb.TagNumber(3)
  set nonce($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNonce() => $_has(2);
  @$pb.TagNumber(3)
  void clearNonce() => $_clearField(3);
}

class ExchangeIdTokenResponse extends $pb.GeneratedMessage {
  factory ExchangeIdTokenResponse({
    $core.String? sessionToken,
    $0.Timestamp? absoluteExpireTime,
    $1.Duration? idleTimeout,
  }) {
    final result = ExchangeIdTokenResponse._();
    if (sessionToken != null) result.sessionToken = sessionToken;
    if (absoluteExpireTime != null)
      result.absoluteExpireTime = absoluteExpireTime;
    if (idleTimeout != null) result.idleTimeout = idleTimeout;
    return result;
  }

  ExchangeIdTokenResponse._();

  factory ExchangeIdTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeIdTokenResponse()..mergeFromBuffer(data, registry);
  factory ExchangeIdTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExchangeIdTokenResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExchangeIdTokenResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ExchangeIdTokenResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionToken')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'absoluteExpireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$1.Duration>(3, _omitFieldNames ? '' : 'idleTimeout',
        subBuilder: $1.Duration.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeIdTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExchangeIdTokenResponse copyWith(
          void Function(ExchangeIdTokenResponse) updates) =>
      super.copyWith((message) => updates(message as ExchangeIdTokenResponse))
          as ExchangeIdTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExchangeIdTokenResponse() / ExchangeIdTokenResponse.new instead')
  static ExchangeIdTokenResponse create() => ExchangeIdTokenResponse._();
  static $pb.GeneratedMessage $_createMessage() => ExchangeIdTokenResponse._();
  @$core.override
  ExchangeIdTokenResponse createEmptyInstance() => ExchangeIdTokenResponse._();
  @$core.pragma('dart2js:noInline')
  static ExchangeIdTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExchangeIdTokenResponse>(
          ExchangeIdTokenResponse.$_createMessage);
  static ExchangeIdTokenResponse? _defaultInstance;

  /// 新 session 的 token，只在這裡出現一次：client 保存在安全儲存，之後以 Authorization: Bearer 帶。
  @$pb.TagNumber(1)
  $core.String get sessionToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionToken() => $_clearField(1);

  /// session 的絕對到期時間（建立後 90 天），到期一定要重新登入。
  @$pb.TagNumber(2)
  $0.Timestamp get absoluteExpireTime => $_getN(1);
  @$pb.TagNumber(2)
  set absoluteExpireTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAbsoluteExpireTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearAbsoluteExpireTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureAbsoluteExpireTime() => $_ensure(1);

  /// 閒置逾時（App 14 天）：這麼久沒有請求 session 就失效；使用中的 session 每天最多續期一次。
  @$pb.TagNumber(3)
  $1.Duration get idleTimeout => $_getN(2);
  @$pb.TagNumber(3)
  set idleTimeout($1.Duration value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasIdleTimeout() => $_has(2);
  @$pb.TagNumber(3)
  void clearIdleTimeout() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Duration ensureIdleTimeout() => $_ensure(2);
}

class SignOutRequest extends $pb.GeneratedMessage {
  factory SignOutRequest() => SignOutRequest._();

  SignOutRequest._();

  factory SignOutRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SignOutRequest()..mergeFromBuffer(data, registry);
  factory SignOutRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SignOutRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SignOutRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: SignOutRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SignOutRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SignOutRequest copyWith(void Function(SignOutRequest) updates) =>
      super.copyWith((message) => updates(message as SignOutRequest))
          as SignOutRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SignOutRequest() / SignOutRequest.new instead')
  static SignOutRequest create() => SignOutRequest._();
  static $pb.GeneratedMessage $_createMessage() => SignOutRequest._();
  @$core.override
  SignOutRequest createEmptyInstance() => SignOutRequest._();
  @$core.pragma('dart2js:noInline')
  static SignOutRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SignOutRequest>(
          SignOutRequest.$_createMessage);
  static SignOutRequest? _defaultInstance;
}

class SignOutResponse extends $pb.GeneratedMessage {
  factory SignOutResponse() => SignOutResponse._();

  SignOutResponse._();

  factory SignOutResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SignOutResponse()..mergeFromBuffer(data, registry);
  factory SignOutResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SignOutResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SignOutResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: SignOutResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SignOutResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SignOutResponse copyWith(void Function(SignOutResponse) updates) =>
      super.copyWith((message) => updates(message as SignOutResponse))
          as SignOutResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SignOutResponse() / SignOutResponse.new instead')
  static SignOutResponse create() => SignOutResponse._();
  static $pb.GeneratedMessage $_createMessage() => SignOutResponse._();
  @$core.override
  SignOutResponse createEmptyInstance() => SignOutResponse._();
  @$core.pragma('dart2js:noInline')
  static SignOutResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SignOutResponse>(
          SignOutResponse.$_createMessage);
  static SignOutResponse? _defaultInstance;
}

class LinkIdentityRequest extends $pb.GeneratedMessage {
  factory LinkIdentityRequest({
    LoginProvider? provider,
    $core.String? idToken,
    $core.String? nonce,
  }) {
    final result = LinkIdentityRequest._();
    if (provider != null) result.provider = provider;
    if (idToken != null) result.idToken = idToken;
    if (nonce != null) result.nonce = nonce;
    return result;
  }

  LinkIdentityRequest._();

  factory LinkIdentityRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkIdentityRequest()..mergeFromBuffer(data, registry);
  factory LinkIdentityRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkIdentityRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LinkIdentityRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LinkIdentityRequest.$_createMessage)
    ..aE<LoginProvider>(1, _omitFieldNames ? '' : 'provider',
        enumValues: LoginProvider.values)
    ..aOS(2, _omitFieldNames ? '' : 'idToken')
    ..aOS(3, _omitFieldNames ? '' : 'nonce')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkIdentityRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkIdentityRequest copyWith(void Function(LinkIdentityRequest) updates) =>
      super.copyWith((message) => updates(message as LinkIdentityRequest))
          as LinkIdentityRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use LinkIdentityRequest() / LinkIdentityRequest.new instead')
  static LinkIdentityRequest create() => LinkIdentityRequest._();
  static $pb.GeneratedMessage $_createMessage() => LinkIdentityRequest._();
  @$core.override
  LinkIdentityRequest createEmptyInstance() => LinkIdentityRequest._();
  @$core.pragma('dart2js:noInline')
  static LinkIdentityRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LinkIdentityRequest>(
          LinkIdentityRequest.$_createMessage);
  static LinkIdentityRequest? _defaultInstance;

  @$pb.TagNumber(1)
  LoginProvider get provider => $_getN(0);
  @$pb.TagNumber(1)
  set provider(LoginProvider value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProvider() => $_has(0);
  @$pb.TagNumber(1)
  void clearProvider() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get idToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set idToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIdToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdToken() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get nonce => $_getSZ(2);
  @$pb.TagNumber(3)
  set nonce($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNonce() => $_has(2);
  @$pb.TagNumber(3)
  void clearNonce() => $_clearField(3);
}

class LinkIdentityResponse extends $pb.GeneratedMessage {
  factory LinkIdentityResponse() => LinkIdentityResponse._();

  LinkIdentityResponse._();

  factory LinkIdentityResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkIdentityResponse()..mergeFromBuffer(data, registry);
  factory LinkIdentityResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkIdentityResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LinkIdentityResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LinkIdentityResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkIdentityResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkIdentityResponse copyWith(void Function(LinkIdentityResponse) updates) =>
      super.copyWith((message) => updates(message as LinkIdentityResponse))
          as LinkIdentityResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use LinkIdentityResponse() / LinkIdentityResponse.new instead')
  static LinkIdentityResponse create() => LinkIdentityResponse._();
  static $pb.GeneratedMessage $_createMessage() => LinkIdentityResponse._();
  @$core.override
  LinkIdentityResponse createEmptyInstance() => LinkIdentityResponse._();
  @$core.pragma('dart2js:noInline')
  static LinkIdentityResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LinkIdentityResponse>(
          LinkIdentityResponse.$_createMessage);
  static LinkIdentityResponse? _defaultInstance;
}

class ListSessionsRequest extends $pb.GeneratedMessage {
  factory ListSessionsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListSessionsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListSessionsRequest._();

  factory ListSessionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSessionsRequest()..mergeFromBuffer(data, registry);
  factory ListSessionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSessionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSessionsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ListSessionsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSessionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSessionsRequest copyWith(void Function(ListSessionsRequest) updates) =>
      super.copyWith((message) => updates(message as ListSessionsRequest))
          as ListSessionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListSessionsRequest() / ListSessionsRequest.new instead')
  static ListSessionsRequest create() => ListSessionsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListSessionsRequest._();
  @$core.override
  ListSessionsRequest createEmptyInstance() => ListSessionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListSessionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSessionsRequest>(
          ListSessionsRequest.$_createMessage);
  static ListSessionsRequest? _defaultInstance;

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

class ListSessionsResponse extends $pb.GeneratedMessage {
  factory ListSessionsResponse({
    $core.Iterable<LoginSession>? sessions,
    $core.String? nextPageToken,
  }) {
    final result = ListSessionsResponse._();
    if (sessions != null) result.sessions.addAll(sessions);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListSessionsResponse._();

  factory ListSessionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSessionsResponse()..mergeFromBuffer(data, registry);
  factory ListSessionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSessionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSessionsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ListSessionsResponse.$_createMessage)
    ..pPM<LoginSession>(1, _omitFieldNames ? '' : 'sessions',
        subBuilder: LoginSession.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSessionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSessionsResponse copyWith(void Function(ListSessionsResponse) updates) =>
      super.copyWith((message) => updates(message as ListSessionsResponse))
          as ListSessionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSessionsResponse() / ListSessionsResponse.new instead')
  static ListSessionsResponse create() => ListSessionsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListSessionsResponse._();
  @$core.override
  ListSessionsResponse createEmptyInstance() => ListSessionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListSessionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSessionsResponse>(
          ListSessionsResponse.$_createMessage);
  static ListSessionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<LoginSession> get sessions => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListLinkedIdentitiesRequest extends $pb.GeneratedMessage {
  factory ListLinkedIdentitiesRequest() => ListLinkedIdentitiesRequest._();

  ListLinkedIdentitiesRequest._();

  factory ListLinkedIdentitiesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLinkedIdentitiesRequest()..mergeFromBuffer(data, registry);
  factory ListLinkedIdentitiesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLinkedIdentitiesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLinkedIdentitiesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ListLinkedIdentitiesRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLinkedIdentitiesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLinkedIdentitiesRequest copyWith(
          void Function(ListLinkedIdentitiesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListLinkedIdentitiesRequest))
          as ListLinkedIdentitiesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListLinkedIdentitiesRequest() / ListLinkedIdentitiesRequest.new instead')
  static ListLinkedIdentitiesRequest create() =>
      ListLinkedIdentitiesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListLinkedIdentitiesRequest._();
  @$core.override
  ListLinkedIdentitiesRequest createEmptyInstance() =>
      ListLinkedIdentitiesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListLinkedIdentitiesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLinkedIdentitiesRequest>(
          ListLinkedIdentitiesRequest.$_createMessage);
  static ListLinkedIdentitiesRequest? _defaultInstance;
}

class ListLinkedIdentitiesResponse extends $pb.GeneratedMessage {
  factory ListLinkedIdentitiesResponse({
    $core.Iterable<LinkedIdentity>? identities,
  }) {
    final result = ListLinkedIdentitiesResponse._();
    if (identities != null) result.identities.addAll(identities);
    return result;
  }

  ListLinkedIdentitiesResponse._();

  factory ListLinkedIdentitiesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLinkedIdentitiesResponse()..mergeFromBuffer(data, registry);
  factory ListLinkedIdentitiesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLinkedIdentitiesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLinkedIdentitiesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: ListLinkedIdentitiesResponse.$_createMessage)
    ..pPM<LinkedIdentity>(1, _omitFieldNames ? '' : 'identities',
        subBuilder: LinkedIdentity.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLinkedIdentitiesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLinkedIdentitiesResponse copyWith(
          void Function(ListLinkedIdentitiesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListLinkedIdentitiesResponse))
          as ListLinkedIdentitiesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListLinkedIdentitiesResponse() / ListLinkedIdentitiesResponse.new instead')
  static ListLinkedIdentitiesResponse create() =>
      ListLinkedIdentitiesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListLinkedIdentitiesResponse._();
  @$core.override
  ListLinkedIdentitiesResponse createEmptyInstance() =>
      ListLinkedIdentitiesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListLinkedIdentitiesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLinkedIdentitiesResponse>(
          ListLinkedIdentitiesResponse.$_createMessage);
  static ListLinkedIdentitiesResponse? _defaultInstance;

  /// 連結時間舊到新；不分頁。
  @$pb.TagNumber(1)
  $pb.PbList<LinkedIdentity> get identities => $_getList(0);
}

/// LinkedIdentity 是帳號已連結的一個登入身分。
class LinkedIdentity extends $pb.GeneratedMessage {
  factory LinkedIdentity({
    LoginProvider? provider,
    $0.Timestamp? linkTime,
  }) {
    final result = LinkedIdentity._();
    if (provider != null) result.provider = provider;
    if (linkTime != null) result.linkTime = linkTime;
    return result;
  }

  LinkedIdentity._();

  factory LinkedIdentity.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkedIdentity()..mergeFromBuffer(data, registry);
  factory LinkedIdentity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkedIdentity()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LinkedIdentity',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LinkedIdentity.$_createMessage)
    ..aE<LoginProvider>(1, _omitFieldNames ? '' : 'provider',
        enumValues: LoginProvider.values)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'linkTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkedIdentity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkedIdentity copyWith(void Function(LinkedIdentity) updates) =>
      super.copyWith((message) => updates(message as LinkedIdentity))
          as LinkedIdentity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use LinkedIdentity() / LinkedIdentity.new instead')
  static LinkedIdentity create() => LinkedIdentity._();
  static $pb.GeneratedMessage $_createMessage() => LinkedIdentity._();
  @$core.override
  LinkedIdentity createEmptyInstance() => LinkedIdentity._();
  @$core.pragma('dart2js:noInline')
  static LinkedIdentity getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LinkedIdentity>(
          LinkedIdentity.$_createMessage);
  static LinkedIdentity? _defaultInstance;

  @$pb.TagNumber(1)
  LoginProvider get provider => $_getN(0);
  @$pb.TagNumber(1)
  set provider(LoginProvider value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProvider() => $_has(0);
  @$pb.TagNumber(1)
  void clearProvider() => $_clearField(1);

  @$pb.TagNumber(2)
  $0.Timestamp get linkTime => $_getN(1);
  @$pb.TagNumber(2)
  set linkTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLinkTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearLinkTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureLinkTime() => $_ensure(1);
}

class RevokeSessionRequest extends $pb.GeneratedMessage {
  factory RevokeSessionRequest({
    $core.String? sessionId,
  }) {
    final result = RevokeSessionRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  RevokeSessionRequest._();

  factory RevokeSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RevokeSessionRequest()..mergeFromBuffer(data, registry);
  factory RevokeSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RevokeSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeSessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: RevokeSessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSessionRequest copyWith(void Function(RevokeSessionRequest) updates) =>
      super.copyWith((message) => updates(message as RevokeSessionRequest))
          as RevokeSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RevokeSessionRequest() / RevokeSessionRequest.new instead')
  static RevokeSessionRequest create() => RevokeSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() => RevokeSessionRequest._();
  @$core.override
  RevokeSessionRequest createEmptyInstance() => RevokeSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static RevokeSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeSessionRequest>(
          RevokeSessionRequest.$_createMessage);
  static RevokeSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

class RevokeSessionResponse extends $pb.GeneratedMessage {
  factory RevokeSessionResponse() => RevokeSessionResponse._();

  RevokeSessionResponse._();

  factory RevokeSessionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RevokeSessionResponse()..mergeFromBuffer(data, registry);
  factory RevokeSessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RevokeSessionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeSessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: RevokeSessionResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSessionResponse copyWith(
          void Function(RevokeSessionResponse) updates) =>
      super.copyWith((message) => updates(message as RevokeSessionResponse))
          as RevokeSessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RevokeSessionResponse() / RevokeSessionResponse.new instead')
  static RevokeSessionResponse create() => RevokeSessionResponse._();
  static $pb.GeneratedMessage $_createMessage() => RevokeSessionResponse._();
  @$core.override
  RevokeSessionResponse createEmptyInstance() => RevokeSessionResponse._();
  @$core.pragma('dart2js:noInline')
  static RevokeSessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeSessionResponse>(
          RevokeSessionResponse.$_createMessage);
  static RevokeSessionResponse? _defaultInstance;
}

/// LoginSession 是帳號的一個登入中的 session。id 不是憑證：token 不會出現在任何回應（除了建立它的那一次）。
class LoginSession extends $pb.GeneratedMessage {
  factory LoginSession({
    $core.String? id,
    ClientKind? clientKind,
    $0.Timestamp? createTime,
    $0.Timestamp? lastUseTime,
    $core.bool? current,
  }) {
    final result = LoginSession._();
    if (id != null) result.id = id;
    if (clientKind != null) result.clientKind = clientKind;
    if (createTime != null) result.createTime = createTime;
    if (lastUseTime != null) result.lastUseTime = lastUseTime;
    if (current != null) result.current = current;
    return result;
  }

  LoginSession._();

  factory LoginSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LoginSession()..mergeFromBuffer(data, registry);
  factory LoginSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LoginSession()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LoginSession',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LoginSession.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<ClientKind>(2, _omitFieldNames ? '' : 'clientKind',
        enumValues: ClientKind.values)
    ..aOM<$0.Timestamp>(3, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'lastUseTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOB(5, _omitFieldNames ? '' : 'current')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginSession copyWith(void Function(LoginSession) updates) =>
      super.copyWith((message) => updates(message as LoginSession))
          as LoginSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use LoginSession() / LoginSession.new instead')
  static LoginSession create() => LoginSession._();
  static $pb.GeneratedMessage $_createMessage() => LoginSession._();
  @$core.override
  LoginSession createEmptyInstance() => LoginSession._();
  @$core.pragma('dart2js:noInline')
  static LoginSession getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LoginSession>(
          LoginSession.$_createMessage);
  static LoginSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  ClientKind get clientKind => $_getN(1);
  @$pb.TagNumber(2)
  set clientKind(ClientKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasClientKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientKind() => $_clearField(2);

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

  /// 最後一次使用的時間；使用中的 session 每天最多更新一次。
  @$pb.TagNumber(4)
  $0.Timestamp get lastUseTime => $_getN(3);
  @$pb.TagNumber(4)
  set lastUseTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLastUseTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastUseTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensureLastUseTime() => $_ensure(3);

  /// 這是不是送出這個請求的 session。
  @$pb.TagNumber(5)
  $core.bool get current => $_getBF(4);
  @$pb.TagNumber(5)
  set current($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCurrent() => $_has(4);
  @$pb.TagNumber(5)
  void clearCurrent() => $_clearField(5);
}

class GetSessionRequest extends $pb.GeneratedMessage {
  factory GetSessionRequest() => GetSessionRequest._();

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
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: GetSessionRequest.$_createMessage)
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
}

class GetSessionResponse extends $pb.GeneratedMessage {
  factory GetSessionResponse({
    Account? account,
    Suspension? suspension,
  }) {
    final result = GetSessionResponse._();
    if (account != null) result.account = account;
    if (suspension != null) result.suspension = suspension;
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
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: GetSessionResponse.$_createMessage)
    ..aOM<Account>(2, _omitFieldNames ? '' : 'account',
        subBuilder: Account.$_createMessage)
    ..aOM<Suspension>(3, _omitFieldNames ? '' : 'suspension',
        subBuilder: Suspension.$_createMessage)
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

  @$pb.TagNumber(2)
  Account get account => $_getN(0);
  @$pb.TagNumber(2)
  set account(Account value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAccount() => $_has(0);
  @$pb.TagNumber(2)
  void clearAccount() => $_clearField(2);
  @$pb.TagNumber(2)
  Account ensureAccount() => $_ensure(0);

  /// 帳號被停權時的狀態；沒有停權時不設。停權的帳號仍能登入、讀取與刪除帳號，寫入回 permission_denied 與
  /// hawkr.common.v1alpha1.ErrorReason.ACCOUNT_SUSPENDED。
  @$pb.TagNumber(3)
  Suspension get suspension => $_getN(1);
  @$pb.TagNumber(3)
  set suspension(Suspension value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSuspension() => $_has(1);
  @$pb.TagNumber(3)
  void clearSuspension() => $_clearField(3);
  @$pb.TagNumber(3)
  Suspension ensureSuspension() => $_ensure(1);
}

/// Suspension 是帳號的停權狀態。
class Suspension extends $pb.GeneratedMessage {
  factory Suspension({
    $0.Timestamp? suspendedAt,
  }) {
    final result = Suspension._();
    if (suspendedAt != null) result.suspendedAt = suspendedAt;
    return result;
  }

  Suspension._();

  factory Suspension.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Suspension()..mergeFromBuffer(data, registry);
  factory Suspension.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Suspension()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Suspension',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: Suspension.$_createMessage)
    ..aOM<$0.Timestamp>(1, _omitFieldNames ? '' : 'suspendedAt',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Suspension clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Suspension copyWith(void Function(Suspension) updates) =>
      super.copyWith((message) => updates(message as Suspension)) as Suspension;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Suspension() / Suspension.new instead')
  static Suspension create() => Suspension._();
  static $pb.GeneratedMessage $_createMessage() => Suspension._();
  @$core.override
  Suspension createEmptyInstance() => Suspension._();
  @$core.pragma('dart2js:noInline')
  static Suspension getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Suspension>(Suspension.$_createMessage);
  static Suspension? _defaultInstance;

  /// 停權的時間。
  @$pb.TagNumber(1)
  $0.Timestamp get suspendedAt => $_getN(0);
  @$pb.TagNumber(1)
  set suspendedAt($0.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSuspendedAt() => $_has(0);
  @$pb.TagNumber(1)
  void clearSuspendedAt() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Timestamp ensureSuspendedAt() => $_ensure(0);
}

class DeleteAccountRequest extends $pb.GeneratedMessage {
  factory DeleteAccountRequest() => DeleteAccountRequest._();

  DeleteAccountRequest._();

  factory DeleteAccountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAccountRequest()..mergeFromBuffer(data, registry);
  factory DeleteAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAccountRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAccountRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: DeleteAccountRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountRequest copyWith(void Function(DeleteAccountRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteAccountRequest))
          as DeleteAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteAccountRequest() / DeleteAccountRequest.new instead')
  static DeleteAccountRequest create() => DeleteAccountRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteAccountRequest._();
  @$core.override
  DeleteAccountRequest createEmptyInstance() => DeleteAccountRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAccountRequest>(
          DeleteAccountRequest.$_createMessage);
  static DeleteAccountRequest? _defaultInstance;
}

class DeleteAccountResponse extends $pb.GeneratedMessage {
  factory DeleteAccountResponse({
    DeleteAccountResponse_DeletionState? state,
  }) {
    final result = DeleteAccountResponse._();
    if (state != null) result.state = state;
    return result;
  }

  DeleteAccountResponse._();

  factory DeleteAccountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAccountResponse()..mergeFromBuffer(data, registry);
  factory DeleteAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteAccountResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAccountResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: DeleteAccountResponse.$_createMessage)
    ..aE<DeleteAccountResponse_DeletionState>(1, _omitFieldNames ? '' : 'state',
        enumValues: DeleteAccountResponse_DeletionState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountResponse copyWith(
          void Function(DeleteAccountResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteAccountResponse))
          as DeleteAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteAccountResponse() / DeleteAccountResponse.new instead')
  static DeleteAccountResponse create() => DeleteAccountResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteAccountResponse._();
  @$core.override
  DeleteAccountResponse createEmptyInstance() => DeleteAccountResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAccountResponse>(
          DeleteAccountResponse.$_createMessage);
  static DeleteAccountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  DeleteAccountResponse_DeletionState get state => $_getN(0);
  @$pb.TagNumber(1)
  set state(DeleteAccountResponse_DeletionState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasState() => $_has(0);
  @$pb.TagNumber(1)
  void clearState() => $_clearField(1);
}

/// IdentityPublicService 是登入之前的方法：App 把原生 SDK 取得的 ID token 換成 session。兩個方法都是 POST，回應一律不可快取。
/// 每個來源 IP 有額度，超過回 resource_exhausted（帶 RetryInfo）。
class IdentityPublicServiceApi {
  final $pb.RpcClient _client;

  IdentityPublicServiceApi(this._client);

  /// IssueAppLoginNonce 發一個單次使用、5 分鐘內有效的 nonce：App 把它交給原生 SDK（Google 的 nonce、LINE 的 nonce），
  /// SDK 簽發的 ID token 的 nonce claim 必須等於它，再交給 ExchangeIdToken 或 LinkIdentity。
  $async.Future<IssueAppLoginNonceResponse> issueAppLoginNonce(
          $pb.ClientContext? ctx, IssueAppLoginNonceRequest request) =>
      _client.invoke<IssueAppLoginNonceResponse>(ctx, 'IdentityPublicService',
          'IssueAppLoginNonce', request, IssueAppLoginNonceResponse());

  /// ExchangeIdToken 以原生 SDK 取得的 ID token 登入並換發 session（App 種類，閒置 14 天、絕對 90 天）：驗證簽章、issuer、audience、
  /// nonce（必須是 IssueAppLoginNonce 發的、尚未用過）；Google 另外要求 azp 在允許的 App client 清單。已有這個身分就登入它的帳號，
  /// 否則依規則自動連結或建立帳號（ADR 同上 D4）。請求已帶 Authorization: Bearer 的舊 session 時一併撤銷。
  /// ID token 或 nonce 無效回 unauthenticated；身分屬於已刪除的帳號也回 unauthenticated；這個 provider 沒有設定回 unavailable。
  $async.Future<ExchangeIdTokenResponse> exchangeIdToken(
          $pb.ClientContext? ctx, ExchangeIdTokenRequest request) =>
      _client.invoke<ExchangeIdTokenResponse>(ctx, 'IdentityPublicService',
          'ExchangeIdToken', request, ExchangeIdTokenResponse());
}

/// IdentityAppService 是登入的買家、賣家的方法。
class IdentityAppServiceApi {
  final $pb.RpcClient _client;

  IdentityAppServiceApi(this._client);

  /// GetSession 回傳目前登入的帳號；web 以它判斷是否仍在登入。
  $async.Future<GetSessionResponse> getSession(
          $pb.ClientContext? ctx, GetSessionRequest request) =>
      _client.invoke<GetSessionResponse>(ctx, 'IdentityAppService',
          'GetSession', request, GetSessionResponse());

  /// SignOut 撤銷目前的 session。web 的回應另以 Set-Cookie 清掉 `__Host-hk_session`。session 已經不存在時也成功。停權的帳號也能呼叫。
  $async.Future<SignOutResponse> signOut(
          $pb.ClientContext? ctx, SignOutRequest request) =>
      _client.invoke<SignOutResponse>(
          ctx, 'IdentityAppService', 'SignOut', request, SignOutResponse());

  /// LinkIdentity 把 App 以原生 SDK 取得的 Google 或 LINE 身分連結到呼叫者的帳號（web 走 GET /api/auth/{provider}/start?link=1）。
  /// ID token 與 nonce 的驗證同 IdentityPublicService.ExchangeIdToken，不換發 session。這個身分已連結到別的帳號（含已刪除帳號
  /// 留下的身分）回 already_exists 與 ErrorReason IDENTITY_ALREADY_LINKED，什麼都不改；已連結到呼叫者自己時成功。
  $async.Future<LinkIdentityResponse> linkIdentity(
          $pb.ClientContext? ctx, LinkIdentityRequest request) =>
      _client.invoke<LinkIdentityResponse>(ctx, 'IdentityAppService',
          'LinkIdentity', request, LinkIdentityResponse());

  /// ListSessions 列出呼叫者的有效 session（新到舊），標出目前這一個。停權的帳號也能呼叫。
  $async.Future<ListSessionsResponse> listSessions(
          $pb.ClientContext? ctx, ListSessionsRequest request) =>
      _client.invoke<ListSessionsResponse>(ctx, 'IdentityAppService',
          'ListSessions', request, ListSessionsResponse());

  /// ListLinkedIdentities 列出呼叫者的帳號已連結的登入身分。同一 provider 可能有多筆；client 依 provider 判斷是否已連結。
  /// 不回 provider 的使用者識別與 email。停權的帳號也能呼叫。
  $async.Future<ListLinkedIdentitiesResponse> listLinkedIdentities(
          $pb.ClientContext? ctx, ListLinkedIdentitiesRequest request) =>
      _client.invoke<ListLinkedIdentitiesResponse>(ctx, 'IdentityAppService',
          'ListLinkedIdentities', request, ListLinkedIdentitiesResponse());

  /// RevokeSession 終止呼叫者自己的一個 session（包含目前這個）。目前的 session 必須在 15 分鐘內以 provider 驗證過
  /// （重新走一次登入），否則回 failed_precondition 與 ErrorReason REAUTHENTICATION_REQUIRED，client 引導重新登入後再送。
  /// 沒有這個 session 或屬於別的帳號回 not_found。停權的帳號也能呼叫。
  $async.Future<RevokeSessionResponse> revokeSession(
          $pb.ClientContext? ctx, RevokeSessionRequest request) =>
      _client.invoke<RevokeSessionResponse>(ctx, 'IdentityAppService',
          'RevokeSession', request, RevokeSessionResponse());

  /// DeleteAccount 刪除呼叫者的帳號（App Store 審核準則 5.1.1(v)）。沒有未完成的訂單時立即刪除；有的話進入刪除流程，
  /// 最後一筆訂單完成後自動刪除，期間不能新增收藏與追蹤；流程中再呼叫回目前的狀態。帳號有店家時店家是
  /// 參與者：同一個 transaction 關閉買家、店家並刪除帳號，店家有未結訂單時與買家一起等待到兩側都結清。client 不需要任何前置步驟。
  /// 刪除之後 token 不再有效，所以 client 不自動重送：沒收到回應時以 GetSession 確認，unauthenticated 表示已刪除。
  $async.Future<DeleteAccountResponse> deleteAccount(
          $pb.ClientContext? ctx, DeleteAccountRequest request) =>
      _client.invoke<DeleteAccountResponse>(ctx, 'IdentityAppService',
          'DeleteAccount', request, DeleteAccountResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
