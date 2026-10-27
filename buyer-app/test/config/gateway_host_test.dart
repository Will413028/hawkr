import 'package:hawkr_buyer/config/flavor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('resolveGatewayHost', () {
    test('a provided define wins for every flavor', () {
      for (final flavor in Flavor.values) {
        expect(
          resolveGatewayHost(flavor, 'https://api.example.test'),
          'https://api.example.test',
          reason: flavor.name,
        );
      }
    });

    test('dev falls back to the local monolith', () {
      expect(resolveGatewayHost(Flavor.dev, ''), 'http://localhost:8080');
    });

    test('stage and prod fail fast naming GATEWAY_HOST', () {
      for (final flavor in [Flavor.stage, Flavor.prod]) {
        expect(
          () => resolveGatewayHost(flavor, ''),
          throwsA(
            isA<StateError>().having(
              (e) => e.message,
              'message',
              contains('GATEWAY_HOST'),
            ),
          ),
          reason: flavor.name,
        );
      }
    });
  });

  group('Flavor core config without a GATEWAY_HOST define', () {
    test('dev keeps a local absolute API base', () {
      Flavor.dev.requireCoreConfig();
      expect(Flavor.dev.apiBaseUrl, 'http://localhost:8080/api/v1');
    });

    test('stage and prod never produce a relative API base', () {
      for (final flavor in [Flavor.stage, Flavor.prod]) {
        expect(flavor.requireCoreConfig, throwsStateError, reason: flavor.name);
        expect(() => flavor.apiBaseUrl, throwsStateError, reason: flavor.name);
      }
    });
  });
}
