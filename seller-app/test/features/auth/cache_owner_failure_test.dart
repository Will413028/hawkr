import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/shared/utils/hk_log.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_auth_session.dart';
import 'support/auth_test_fakes.dart';
import 'support/login_harness.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  for (final method in ['getAll', 'setString']) {
    for (final restore in [true, false]) {
      test(
        'cache owner $method failure preserves session and cache: restore=$restore',
        () async {
          SharedPreferences.resetStatic();
          final stored = <String, Object>{
            'flutter.auth_cache_owner': 'acc-1',
            'flutter.seller_info_json': '{"name":"cached"}',
            'flutter.image_editor_ai_history': '[]',
          };
          final messages = <String?>[];
          final calls = <String>[];
          final sink = HKLog.sentryBreadcrumbSink;
          HKLog.sentryBreadcrumbSink = (entry) => messages.add(entry.message);
          addTearDown(() => HKLog.sentryBreadcrumbSink = sink);
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
                const MethodChannel('plugins.flutter.io/shared_preferences'),
                (call) async {
                  calls.add(call.method);
                  if (call.method == method) {
                    throw PlatformException(code: 'storage_unavailable');
                  }
                  if (call.method == 'getAll') {
                    return Map<String, Object>.of(stored);
                  }
                  final args = call.arguments as Map;
                  if (call.method == 'remove') stored.remove(args['key']);
                  if (call.method == 'setString') {
                    stored[args['key'] as String] = args['value'] as Object;
                  }
                  return true;
                },
              );
          final repo = FakeAuthRepository();
          when(() => repo.currentAccountId()).thenAnswer((_) async => 'acc-1');
          final c = makeLoginContainer(
            repo: repo,
            auth: FakeAuthSession(signedIn: restore),
          );
          final session = c.read(sessionProvider.notifier);
          if (restore) {
            await session.initialize();
          } else {
            await session.login(accountId: 'acc-1');
          }
          expect(c.read(sessionProvider).isLoggedIn, isTrue);
          expect(c.read(sessionProvider).accountId, 'acc-1');
          expect(stored['flutter.seller_info_json'], '{"name":"cached"}');
          expect(stored['flutter.image_editor_ai_history'], '[]');
          expect(messages, contains(contains('cache owner')));
          expect(calls, contains(method));
        },
      );
    }
  }
}
