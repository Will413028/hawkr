import 'package:hawkr_api/errors.dart'
    show DefaultErrorText, ErrorFacts, isSessionUnavailableCode, errorText;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';

part 'auth_repository.g.dart';

/// 登入的帳號、交付 Apple 憑證與刪除帳號：identity 的 `IdentityAppService`（Connect）。登入與登出由 Clerk
/// 處理（`clerkSessionProvider`）。
///
/// [currentAccountId] 失敗一律拋 [AuthRequestException]（給使用者看的文字在 `message`），回應不符
/// 契約時拋 [MalformedAuthResponse]；[deleteAccount] 與 [accountExists] 拋出原本的例外，由刪除流程自己分類。
class AuthRepository {
  AuthRepository(
    this._client, {
    IdentityAppServiceClient? sessionClient,
    required DefaultErrorText defaults,
  }) : _sessionClient = sessionClient ?? _client,
       _defaults = defaults;

  final IdentityAppServiceClient _client;
  final IdentityAppServiceClient _sessionClient;
  final DefaultErrorText _defaults;

  /// 目前登入的帳號 ID（GetSession）。Clerk 登入之後呼叫：第一次登入的使用者，伺服器在這時建立帳號。
  Future<String> currentAccountId() async {
    final response = await _call(
      () => _sessionClient.getSession(GetSessionRequest()),
    );
    if (response.account.id.isEmpty) {
      throw const MalformedAuthResponse('GetSession: missing account id');
    }
    return response.account.id;
  }

  /// 刪除帳號（DeleteAccount）。失敗拋出原本的例外，由呼叫端依 [ErrorFacts] 分類：沒收到回應時帳號可能已刪除，
  /// 不能自動重送，要以 [accountExists] 確認。
  Future<DeleteAccountResponse_DeletionState> deleteAccount() async {
    final response = await _client.deleteAccount(DeleteAccountRequest());
    return response.state;
  }

  /// 以 GetSession 確認帳號還在：有回應是 true，unauthenticated（token 已失效）是 false。其他失敗拋出原本的例外。
  Future<bool> accountExists() async {
    try {
      await _sessionClient.getSession(GetSessionRequest());
      return true;
    } catch (raw) {
      if (ConnectException.from(raw).code == Code.unauthenticated) return false;
      rethrow;
    }
  }

  Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } catch (raw) {
      throw AuthRequestException.from(
        ConnectException.from(raw),
        defaults: _defaults,
      );
    }
  }
}

@riverpod
AuthRepository authRepository(Ref ref) {
  return AuthRepository(
    ref.watch(identityClientProvider),
    sessionClient: ref.watch(sessionIdentityClientProvider),
    defaults: ref.watch(defaultErrorTextProvider),
  );
}

/// 伺服器的回應不符契約時拋出（例如成功卻沒有帳號）。
/// **不要在 UI 層 catch 後吞掉** — 這代表契約不一致，必須冒泡讓人看到。
class MalformedAuthResponse implements Exception {
  const MalformedAuthResponse(this.detail);
  final String detail;

  @override
  String toString() => 'MalformedAuthResponse: $detail';
}

/// identity 的呼叫失敗。[message] 是給使用者看的文字：伺服器的 LocalizedMessage，沒有時是 code 的預設
/// 文字；沒收到回應時是網路錯誤的文字。`toString()` 就是 [message]，畫面直接顯示它。
class AuthRequestException implements Exception {
  factory AuthRequestException.from(
    ConnectException error, {
    required DefaultErrorText defaults,
  }) {
    final facts = ErrorFacts.fromException(error);
    return AuthRequestException._(
      code: error.code,
      noResponse: facts.noResponse,
      message: facts.noResponse
          ? '網路連線失敗，請檢查網路設定'
          : errorText(error, defaults: defaults),
    );
  }

  const AuthRequestException._({
    required this.code,
    required this.noResponse,
    required this.message,
  });

  bool get isTransient => noResponse || isSessionUnavailableCode(code);

  final Code code;

  /// 沒有收到伺服器的回應（連線失敗、斷線）。
  final bool noResponse;

  final String message;

  @override
  String toString() => message;
}
