import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('critical seller flows retain an identifiable test matrix', () {
    const requiredFiles = {
      'auth/session restore': 'test/features/auth/login_flow_test.dart',
      'shipping mutation failure':
          'test/features/settings/data/cvs_shipping_mutation_test.dart',
      'FCM registration':
          'test/features/notification/data/push/device_registration_test.dart',
      'route restoration': 'test/core/router/route_payload_test.dart',
      'order/logistics contract':
          'test/features/order/data/payuni_logistics_service_test.dart',
      'production router smoke':
          'test/integration/hermetic_app_router_smoke_test.dart',
    };

    for (final entry in requiredFiles.entries) {
      expect(
        File(entry.value).existsSync(),
        isTrue,
        reason: '${entry.key} must remain represented by ${entry.value}',
      );
    }
  });

  test('skipped tests require complete, time-bounded quarantine metadata', () {
    final quarantineFile = File('tool/test_quarantine.json');
    final decoded = jsonDecode(quarantineFile.readAsStringSync());
    expect(decoded, isA<Map<String, dynamic>>());
    final document = decoded as Map<String, dynamic>;
    expect(document['schemaVersion'], 1);
    final rawEntries = document['entries'];
    expect(rawEntries, isA<List>());
    final entries = (rawEntries as List)
        .whereType<Map>()
        .map((entry) => entry.cast<String, dynamic>())
        .toList();
    expect(entries, hasLength(rawEntries.length));

    final quarantinedFiles = <String>{};
    for (final entry in entries) {
      for (final field in const [
        'file',
        'testName',
        'owner',
        'trackingIssue',
        'reproductionEnvironment',
        'removalCondition',
        'removeBy',
      ]) {
        expect(
          entry[field]?.toString().trim(),
          isNotEmpty,
          reason: 'quarantine entry is missing $field: $entry',
        );
      }
      expect(
        DateTime.tryParse(entry['removeBy'].toString()),
        isNotNull,
        reason: 'removeBy must be an ISO date: $entry',
      );
      quarantinedFiles.add(entry['file'].toString());
    }

    final skipAnnotation =
        '@'
        'Skip';
    final skippedFiles = <String>{};
    for (final root in const ['test', 'integration_test']) {
      for (final entity in Directory(root).listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final source = entity.readAsStringSync();
        if (RegExp(r'\bskip\s*:').hasMatch(source) ||
            source.contains(skipAnnotation)) {
          skippedFiles.add(entity.path);
        }
      }
    }
    expect(
      skippedFiles,
      quarantinedFiles,
      reason:
          'Every skip must have owner, issue, reproduction, removal '
          'condition, and deadline metadata.',
    );
  });
}
