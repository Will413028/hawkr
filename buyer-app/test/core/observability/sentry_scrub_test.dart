import 'dart:convert';

import 'package:hawkr_buyer/core/observability/sentry_scrub.dart';
import 'package:hawkr_buyer/shared/utils/secret_redaction.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

void main() {
  test('strips sensitive request headers and cookies', () {
    final event = SentryEvent(
      request: SentryRequest(
        headers: const {
          'authorization': 'Bearer secret',
          'cookie': 'session=abc',
          'accept': 'application/json',
        },
        cookies: 'session=abc',
        data: const {'password': 'hunter2', 'items': 2},
      ),
      user: SentryUser(email: 'user@example.com', ipAddress: '1.2.3.4'),
    );

    final scrubbed = scrubSentryEvent(event, Hint());

    expect(scrubbed.request?.headers.containsKey('authorization'), isFalse);
    expect(scrubbed.request?.headers.containsKey('cookie'), isFalse);
    expect(scrubbed.request?.headers['accept'], 'application/json');
    expect(scrubbed.request?.cookies, isNull);
    expect((scrubbed.request?.data as Map)['password'], redactedSecret);
    expect((scrubbed.request?.data as Map)['items'], 2);
    expect(scrubbed.user?.email, isNull);
    expect(scrubbed.user?.ipAddress, isNull);
  });

  test('passes through an event with no request/user untouched', () {
    final event = SentryEvent(message: SentryMessage('plain event'));
    final scrubbed = scrubSentryEvent(event, Hint());
    expect(scrubbed.message?.formatted, 'plain event');
  });

  test(
    'preserves environment/release/serverName when request/user are scrubbed',
    () {
      final event = SentryEvent(
        environment: 'prod',
        release: 'hawkr_buyer@1.0.0',
        serverName: 'buyer-app',
        request: SentryRequest(
          headers: const {
            'authorization': 'Bearer secret',
            'cookie': 'session=abc',
            'accept': 'application/json',
          },
          cookies: 'session=abc',
          data: const {'password': 'hunter2', 'items': 2},
        ),
        user: SentryUser(email: 'user@example.com', ipAddress: '1.2.3.4'),
      );

      final scrubbed = scrubSentryEvent(event, Hint());

      // The scrub must not silently drop unrelated event fields when it
      // rebuilds/replaces the request or user.
      expect(scrubbed.environment, 'prod');
      expect(scrubbed.release, 'hawkr_buyer@1.0.0');
      expect(scrubbed.serverName, 'buyer-app');

      // Sensitive data should still be scrubbed.
      expect(scrubbed.request?.headers.containsKey('authorization'), isFalse);
      expect(scrubbed.request?.cookies, isNull);
      expect(scrubbed.user?.email, isNull);
      expect(scrubbed.user?.ipAddress, isNull);
    },
  );

  test('known secret sentinel is absent from every scrubbed sink field', () {
    const sentinel = 'KNOWN-SECRET-SENTINEL';
    final event = SentryEvent(
      message: SentryMessage('token=$sentinel'),
      request: SentryRequest(
        url: 'https://example.test/path?fcm_token=$sentinel&safe=1',
        queryString: 'fcm_token=$sentinel&safe=1',
        headers: const {'X-Device-Token': sentinel},
        data: const {
          'deviceToken': sentinel,
          'nested': {'refresh_token': sentinel},
        },
      ),
      breadcrumbs: [
        Breadcrumb(
          message: 'Authorization: Bearer $sentinel',
          data: const {'token': sentinel},
        ),
      ],
      exceptions: [
        SentryException(type: 'Example', value: 'refresh_token=$sentinel'),
      ],
      // ignore: deprecated_member_use
      extra: const {'access_token': sentinel},
    );

    final scrubbed = scrubSentryEvent(event, Hint());
    final serialized = jsonEncode(scrubbed.toJson());

    expect(serialized, isNot(contains(sentinel)));
    expect(serialized, contains(redactedSecret));
    expect(scrubbed.request!.queryString, contains('safe=1'));
  });
}
