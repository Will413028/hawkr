import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_session.dart';
import 'session_token_store.dart';

const kE2EAuthToken = String.fromEnvironment('E2E_AUTH_TOKEN');

bool get e2eAuthEnabled => kDebugMode && kE2EAuthToken.isNotEmpty;

/// session token 的保存處：Keychain。
final sessionTokenStoreProvider = Provider<SessionTokenStore>(
  (ref) => const SecureSessionTokenStore(),
);

final authConfigProvider = Provider<AuthConfig>(
  (ref) => const AuthConfig.fromEnvironment(),
);

/// 請求要帶的 session token（沒有登入時 null）。[session] 是 App 的 `AuthSession`（App 用自己的 gateway 建立）。
/// [accountOwner] 給的時候，帳號在取得 token 的過程中換了人就回 null，避免用前一個帳號的 token 送出新帳號的請求。
Future<String?> currentSessionToken(
  Ref ref, {
  required Future<AuthSession> session,
  String? Function()? accountOwner,
}) async {
  final owner = accountOwner?.call();
  if (accountOwner != null && owner == null) return null;
  if (e2eAuthEnabled) return kE2EAuthToken;
  final token = await (await session).token();
  if (!ref.mounted || (accountOwner != null && accountOwner() != owner)) {
    return null;
  }
  return token;
}
