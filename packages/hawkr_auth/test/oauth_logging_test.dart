import 'package:hawkr_auth/logging.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  for (final channel in [
    'flutter_web_auth_2',
    'com.aboutyou.dart_packages.sign_in_with_apple',
  ]) {
    test('原生登入錯誤交給注入 logger：$channel', () async {
      final events = <(AuthLogLevel, Object)>[];
      final container = ProviderContainer(
        overrides: [
          authLoggerProvider.overrideWithValue((level, message, error, stack) {
            events.add((level, error));
          }),
        ],
      );
      addTearDown(container.dispose);
      final messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      messenger.setMockMethodCallHandler(MethodChannel(channel), (_) async {
        throw PlatformException(code: 'FAILED', message: 'native failed');
      });
      addTearDown(
        () => messenger.setMockMethodCallHandler(MethodChannel(channel), null),
      );
      final picker = container.read(oauthPickerProvider);
      await expectLater(
        channel == 'flutter_web_auth_2'
            ? picker.authenticateInBrowser(
                Uri.parse('https://clerk.example/oauth'),
                'app',
              )
            : picker.pickApple(),
        throwsException,
      );
      expect(events, isNotEmpty);
      expect(events.single.$1, AuthLogLevel.error);
    });
  }
}
