import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/transport.dart';
import 'package:test/test.dart';

void main() {
  test('session unavailable codes match website', () {
    for (final code in Code.values) {
      expect(
        isSessionUnavailableCode(code),
        code != Code.unauthenticated && code != Code.permissionDenied,
        reason: code.name,
      );
    }
  });
}
