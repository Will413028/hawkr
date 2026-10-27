import 'package:hawkr_seller/shared/utils/secret_redaction.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const sentinel = 'KNOWN-SECRET-SENTINEL';

  test('recursively redacts token-shaped keys without losing structure', () {
    final redacted = redactSensitiveData({
      'token': sentinel,
      'nested': {
        'refreshToken': sentinel,
        'device_token': sentinel,
        'status': 503,
      },
      'items': [
        {'password': sentinel, 'name': 'safe'},
      ],
    }).toString();

    expect(redacted, isNot(contains(sentinel)));
    expect(redacted, contains('status: 503'));
    expect(redacted, contains('name: safe'));
  });

  test('redacts rendered Bearer, JWT, and token assignments', () {
    const jwt = 'eyJhbGciOiJIUzI1NiJ9.cGF5bG9hZA.signature';
    final redacted = redactSensitiveText(
      'Authorization: Bearer $sentinel token=$sentinel jwt_token=$jwt',
    );

    expect(redacted, isNot(contains(sentinel)));
    expect(redacted, isNot(contains(jwt)));
    expect(redacted, contains(redactedSecret));
  });

  test('redacts secret query parameters in URIs', () {
    final uri = redactSensitiveUri(
      Uri.parse('https://example.test/resource?token=$sentinel&account=42'),
    ).toString();

    expect(uri, isNot(contains(sentinel)));
    expect(uri, contains('account=42'));
  });
}
