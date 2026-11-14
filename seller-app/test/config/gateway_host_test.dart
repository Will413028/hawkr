import 'dart:io';

import 'package:hawkr_seller/config/flavor.dart';
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

  group('Makefile require-gateway', () {
    // stage 與 prod 的建置要擋缺少的 GATEWAY_HOST：不然缺值的建置通過、啟動時才丟 StateError。
    Future<ProcessResult> run(Map<String, String> vars) => Process.run('make', [
      '-s',
      'require-gateway',
      'DEFINES_FILE=/nonexistent/dart-defines.json',
      for (final entry in vars.entries) '${entry.key}=${entry.value}',
    ]);

    test('missing GATEWAY_HOST fails and names it', () async {
      final result = await run({});
      expect(result.exitCode, isNot(0));
      expect(result.stderr, contains('GATEWAY_HOST'));
    });

    test('provided GATEWAY_HOST passes', () async {
      final result = await run({'GATEWAY_HOST': 'https://api.example.test'});
      expect(result.exitCode, 0);
    });
  });
}
