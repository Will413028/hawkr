import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'session_provider.dart';

export 'package:hawkr_auth/hawkr_auth.dart'
    show
        ClerkSession,
        clerkSessionProvider,
        e2eAuthEnabled,
        kClerkPublishableKey,
        kE2EAuthToken;

final kOAuthRedirect = Uri.parse('hawkrbuyer://oauth-callback');

Future<String?> currentSessionToken(Ref ref) => auth.currentSessionToken(
  ref,
  accountOwner: () => ref.read(sessionProvider).value?.userId,
);
