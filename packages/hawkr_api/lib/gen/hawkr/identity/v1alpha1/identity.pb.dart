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
  }) {
    final result = GetSessionResponse._();
    if (account != null) result.account = account;
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
}

class LinkAppleCredentialRequest extends $pb.GeneratedMessage {
  factory LinkAppleCredentialRequest({
    $core.String? idToken,
    $core.String? authorizationCode,
  }) {
    final result = LinkAppleCredentialRequest._();
    if (idToken != null) result.idToken = idToken;
    if (authorizationCode != null) result.authorizationCode = authorizationCode;
    return result;
  }

  LinkAppleCredentialRequest._();

  factory LinkAppleCredentialRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkAppleCredentialRequest()..mergeFromBuffer(data, registry);
  factory LinkAppleCredentialRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkAppleCredentialRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LinkAppleCredentialRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LinkAppleCredentialRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'idToken')
    ..aOS(2, _omitFieldNames ? '' : 'authorizationCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkAppleCredentialRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkAppleCredentialRequest copyWith(
          void Function(LinkAppleCredentialRequest) updates) =>
      super.copyWith(
              (message) => updates(message as LinkAppleCredentialRequest))
          as LinkAppleCredentialRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use LinkAppleCredentialRequest() / LinkAppleCredentialRequest.new instead')
  static LinkAppleCredentialRequest create() => LinkAppleCredentialRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      LinkAppleCredentialRequest._();
  @$core.override
  LinkAppleCredentialRequest createEmptyInstance() =>
      LinkAppleCredentialRequest._();
  @$core.pragma('dart2js:noInline')
  static LinkAppleCredentialRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LinkAppleCredentialRequest>(
          LinkAppleCredentialRequest.$_createMessage);
  static LinkAppleCredentialRequest? _defaultInstance;

  /// Sign in with Apple 這次給的 identity token：驗證它的簽章與 audience；伺服器之後確認 Apple 換 token 簽發的 ID token 是同一個身分。
  @$pb.TagNumber(1)
  $core.String get idToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set idToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasIdToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdToken() => $_clearField(1);

  /// 同一次給的 authorization code，伺服器在背景以它換 refresh token 保存。
  @$pb.TagNumber(2)
  $core.String get authorizationCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set authorizationCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuthorizationCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuthorizationCode() => $_clearField(2);
}

class LinkAppleCredentialResponse extends $pb.GeneratedMessage {
  factory LinkAppleCredentialResponse() => LinkAppleCredentialResponse._();

  LinkAppleCredentialResponse._();

  factory LinkAppleCredentialResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkAppleCredentialResponse()..mergeFromBuffer(data, registry);
  factory LinkAppleCredentialResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LinkAppleCredentialResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LinkAppleCredentialResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.identity.v1alpha1'),
      createEmptyInstance: LinkAppleCredentialResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkAppleCredentialResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LinkAppleCredentialResponse copyWith(
          void Function(LinkAppleCredentialResponse) updates) =>
      super.copyWith(
              (message) => updates(message as LinkAppleCredentialResponse))
          as LinkAppleCredentialResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use LinkAppleCredentialResponse() / LinkAppleCredentialResponse.new instead')
  static LinkAppleCredentialResponse create() =>
      LinkAppleCredentialResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      LinkAppleCredentialResponse._();
  @$core.override
  LinkAppleCredentialResponse createEmptyInstance() =>
      LinkAppleCredentialResponse._();
  @$core.pragma('dart2js:noInline')
  static LinkAppleCredentialResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LinkAppleCredentialResponse>(
          LinkAppleCredentialResponse.$_createMessage);
  static LinkAppleCredentialResponse? _defaultInstance;
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

/// IdentityAppService 是登入的買家、賣家的方法。
class IdentityAppServiceApi {
  final $pb.RpcClient _client;

  IdentityAppServiceApi(this._client);

  /// GetSession 回傳目前登入的帳號；web 以它判斷是否仍在登入。
  $async.Future<GetSessionResponse> getSession(
          $pb.ClientContext? ctx, GetSessionRequest request) =>
      _client.invoke<GetSessionResponse>(ctx, 'IdentityAppService',
          'GetSession', request, GetSessionResponse());

  /// LinkAppleCredential 交出 Sign in with Apple 的憑證（App Store 審核準則 5.1.1(v)）：App 每次以 Apple 原生登入成功後呼叫。
  /// 伺服器驗證 identity token（簽章、issuer、audience），之後在背景以 authorization code 換成 refresh token；Apple 換 token
  /// 簽發的 ID token 必須與這個 identity token 是同一個身分才加密保存（取代舊的），成功只表示已經排入。帳號最終刪除時
  /// 伺服器自動撤銷保存的 token。可以安全重送。identity token 無效時回 invalid_argument；Apple 沒有設定（本機開發）時回
  /// unavailable。
  $async.Future<LinkAppleCredentialResponse> linkAppleCredential(
          $pb.ClientContext? ctx, LinkAppleCredentialRequest request) =>
      _client.invoke<LinkAppleCredentialResponse>(ctx, 'IdentityAppService',
          'LinkAppleCredential', request, LinkAppleCredentialResponse());

  /// DeleteAccount 刪除呼叫者的帳號（App Store 審核準則 5.1.1(v)）。沒有未完成的訂單時立即刪除；有的話進入刪除流程，
  /// 最後一筆訂單完成後自動刪除，期間不能新增收藏與追蹤；流程中再呼叫回目前的狀態。帳號有店家時店家是
  /// 參與者：同一個 transaction 關閉買家、店家並刪除帳號，店家有未結訂單時與買家一起等待到兩側都結清。保存的 Apple 授權由伺服器在最後刪除時撤銷，client 不需要任何前置步驟。
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
