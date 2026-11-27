import 'package:hawkr_buyer/config/flavor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Flavor.sentryDsn', () {
    test(
      'every flavor has no DSN unless SENTRY_DSN is defined at build time',
      () {
        for (final flavor in Flavor.values) {
          expect(flavor.sentryDsn, '', reason: flavor.name);
        }
      },
    );
  });
}
