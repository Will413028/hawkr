import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'clerk.dart';
import 'logging.dart';

const kClerkPublishableKey = String.fromEnvironment('CLERK_PUBLISHABLE_KEY');
const kE2EAuthToken = String.fromEnvironment('E2E_AUTH_TOKEN');

bool get e2eAuthEnabled => kDebugMode && kE2EAuthToken.isNotEmpty;

final clerkSessionProvider = FutureProvider<ClerkSession>((ref) async {
  if (kClerkPublishableKey.isEmpty) return const ClerkSession.disabled();
  final log = ref.watch(authLoggerProvider);
  const storage = FlutterSecureStorage();
  final session = await ClerkSession.start(
    publishableKey: kClerkPublishableKey,
    persistor: KeyValuePersistor(
      read: (key) => storage.read(key: key),
      write: (key, value) => storage.write(key: key, value: value),
      delete: (key) => storage.delete(key: key),
      onError: (error, stack) =>
          log(AuthLogLevel.error, '[Clerk] 讀寫 Keychain 失敗', error, stack),
    ),
    onBackgroundError: (error, stack) =>
        log(AuthLogLevel.warning, '[Clerk] 背景換新 token 失敗', error, stack),
  );
  ref.onDispose(session.dispose);
  return session;
});

Future<String?> currentSessionToken(
  Ref ref, {
  String? Function()? accountOwner,
}) async {
  final owner = accountOwner?.call();
  if (accountOwner != null && owner == null) return null;
  if (e2eAuthEnabled) return kE2EAuthToken;
  final clerk = await ref.read(clerkSessionProvider.future);
  final token = await clerk.token();
  if (!ref.mounted || (accountOwner != null && accountOwner() != owner)) {
    return null;
  }
  return token;
}
