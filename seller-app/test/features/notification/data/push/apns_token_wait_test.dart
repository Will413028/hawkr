import 'package:hawkr_seller/features/notification/data/push/apns_token_wait.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('waitForApnsToken', () {
    test('returns immediately when the token is already available', () async {
      var calls = 0;
      final token = await waitForApnsToken(() async {
        calls++;
        return 'apns-token';
      });

      expect(token, 'apns-token');
      expect(calls, 1);
    });

    test('retries until the token becomes available', () async {
      var calls = 0;
      final delays = <Duration>[];
      final token = await waitForApnsToken(
        () async {
          calls++;
          return calls < 3 ? null : 'apns-token';
        },
        interval: const Duration(seconds: 1),
        delay: (d) async => delays.add(d),
      );

      expect(token, 'apns-token');
      expect(calls, 3);
      expect(delays, [const Duration(seconds: 1), const Duration(seconds: 1)]);
    });

    test('gives up after maxAttempts and returns null', () async {
      var calls = 0;
      final delays = <Duration>[];
      final token = await waitForApnsToken(
        () async {
          calls++;
          return null;
        },
        maxAttempts: 3,
        delay: (d) async => delays.add(d),
      );

      expect(token, isNull);
      expect(calls, 3);
      // 最後一次嘗試後不應再多等一次 delay。
      expect(delays, hasLength(2));
    });
  });
}
