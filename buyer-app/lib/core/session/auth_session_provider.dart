import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../features/auth/data/identity_gateway.dart';
import 'session_provider.dart';

export 'package:hawkr_auth/hawkr_auth.dart'
    show AuthSession, e2eAuthEnabled, kE2EAuthToken;

/// App 的登入狀態：保存在 Keychain 的 identity session token，登入時經 [identityGatewayProvider] 換發。
final authSessionProvider = FutureProvider<auth.AuthSession>(
  (ref) => auth.AuthSession.restore(
    store: ref.watch(auth.sessionTokenStoreProvider),
    gateway: ref.watch(identityGatewayProvider),
    config: ref.watch(auth.authConfigProvider),
    log: ref.watch(auth.authLoggerProvider),
  ),
);

Future<String?> currentSessionToken(Ref ref) => auth.currentSessionToken(
  ref,
  session: ref.read(authSessionProvider.future),
  accountOwner: () => ref.read(sessionProvider).value?.userId,
);
