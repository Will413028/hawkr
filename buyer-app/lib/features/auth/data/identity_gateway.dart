import 'package:hawkr_api/errors.dart'
    show DefaultErrorText, ErrorFacts, errorText;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart'
    as identity;
import 'package:hawkr_api/transport.dart' show ConnectException;
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';

/// 登入時換發 session 的呼叫：identity 的 `IdentityPublicService`（`IssueAppLoginNonce`、`ExchangeIdToken`），
/// 登入前沒有 session，所以不帶身分。
final identityGatewayProvider = Provider<auth.IdentityGateway>(
  (ref) => IdentityLoginGateway(
    IdentityPublicServiceClient(ref.watch(connectTransportProvider)),
    defaults: ref.watch(defaultErrorTextProvider),
  ),
);

/// 失敗轉成 [AppException]（`toString()` 是給使用者看的文字）：沒收到回應是 [NetworkException]，其他是
/// [ServerException]，文字取自伺服器的 LocalizedMessage，沒有時用該 code 的預設文字。
class IdentityLoginGateway implements auth.IdentityGateway {
  IdentityLoginGateway(this._client, {required DefaultErrorText defaults})
    : _defaults = defaults;

  final IdentityPublicServiceClient _client;
  final DefaultErrorText _defaults;

  @override
  Future<String> issueNonce() async {
    final response = await _call(
      'issueAppLoginNonce',
      () => _client.issueAppLoginNonce(identity.IssueAppLoginNonceRequest()),
    );
    return response.nonce;
  }

  @override
  Future<String> exchange({
    required auth.OAuthProvider provider,
    required String idToken,
    required String nonce,
  }) async {
    final response = await _call(
      'exchangeIdToken',
      () => _client.exchangeIdToken(
        identity.ExchangeIdTokenRequest(
          provider: switch (provider) {
            auth.OAuthProvider.google =>
              identity.LoginProvider.LOGIN_PROVIDER_GOOGLE,
            auth.OAuthProvider.line =>
              identity.LoginProvider.LOGIN_PROVIDER_LINE,
          },
          idToken: idToken,
          nonce: nonce,
        ),
      ),
    );
    if (response.sessionToken.isEmpty) {
      throw const MalformedLoginResponse(
        'ExchangeIdToken without session token',
      );
    }
    return response.sessionToken;
  }

  Future<T> _call<T>(String tag, Future<T> Function() call) async {
    try {
      return await call();
    } catch (raw) {
      final error = ConnectException.from(raw);
      HKLog.logE('[IdentityGateway] $tag failed: ${error.code.name}', error);
      if (ErrorFacts.fromException(error).noResponse) {
        throw const NetworkException();
      }
      throw ServerException(errorText(error, defaults: _defaults));
    }
  }
}

/// 伺服器的回應不符契約時拋出。**不要在 UI 層 catch 後吞掉**：這代表 backend spec drift。
class MalformedLoginResponse implements Exception {
  const MalformedLoginResponse(this.detail);
  final String detail;

  @override
  String toString() => 'MalformedLoginResponse: $detail';
}
