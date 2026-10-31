import 'package:hawkr_api/errors.dart' show DefaultErrorText;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart'
    as identity;
import 'package:hawkr_api/transport.dart' show ConnectException;
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import 'auth_repository.dart';

/// 登入時換發 session 的呼叫：identity 的 `IdentityPublicService`（`IssueAppLoginNonce`、`ExchangeIdToken`），
/// 登入前沒有 session，所以不帶身分。
final identityGatewayProvider = Provider<auth.IdentityGateway>(
  (ref) => IdentityLoginGateway(
    IdentityPublicServiceClient(ref.watch(connectTransportProvider)),
    defaults: ref.watch(defaultErrorTextProvider),
  ),
);

/// 失敗一律轉成 [AuthRequestException]（`toString()` 是給使用者看的文字）。
class IdentityLoginGateway implements auth.IdentityGateway {
  IdentityLoginGateway(this._client, {required DefaultErrorText defaults})
    : _defaults = defaults;

  final IdentityPublicServiceClient _client;
  final DefaultErrorText _defaults;

  @override
  Future<String> issueNonce() async {
    final response = await _call(
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
      throw const MalformedAuthResponse(
        'ExchangeIdToken: missing session token',
      );
    }
    return response.sessionToken;
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
