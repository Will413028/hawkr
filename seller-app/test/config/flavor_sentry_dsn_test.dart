import 'package:hawkr_seller/config/flavor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Flavor.sentryDsn', () {
    test(
      'every flavor has no DSN unless SENTRY_DSN is defined at build time',
      () {
        // 測試不帶 --dart-define=SENTRY_DSN：Sentry 停用（SentryFlutter.init 不送事件）。
        for (final flavor in Flavor.values) {
          expect(flavor.sentryDsn, '', reason: flavor.name);
        }
      },
    );
  });
}
