import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../../tool/presentation_io_guard.dart';

void main() {
  test('current presentation I/O inventory is empty', () {
    final result = evaluatePresentationIoGuard(
      sourceRoot: Directory('lib/features'),
      policy: readPresentationIoPolicy(defaultPresentationPolicyPath),
    );

    expect(result.failures, isEmpty);
    expect(result.violations, isEmpty);
  });

  test('a new direct presentation dependency is rejected', () {
    final root = Directory.systemTemp.createTempSync(
      'seller-presentation-guard-',
    );
    addTearDown(() => root.deleteSync(recursive: true));
    final source = File('${root.path}/customer/presentation/new_screen.dart')
      ..createSync(recursive: true);
    source.writeAsStringSync(
      "import 'package:shared_preferences/shared_preferences.dart';\n",
    );

    final result = evaluatePresentationIoGuard(
      sourceRoot: root,
      policy: const <String, dynamic>{
        'schemaVersion': 1,
        'exceptions': <dynamic>[],
      },
    );

    expect(
      result.failures,
      contains(
        contains('New presentation I/O: ${source.path.replaceAll(r'\', '/')}'),
      ),
    );
  });

  test('legacy exceptions require owner rationale and removal condition', () {
    final root = Directory.systemTemp.createTempSync(
      'seller-presentation-metadata-',
    );
    addTearDown(() => root.deleteSync(recursive: true));
    final source = File('${root.path}/customer/presentation/legacy_screen.dart')
      ..createSync(recursive: true);
    source.writeAsStringSync('final client = Dio();\n');

    final result = evaluatePresentationIoGuard(
      sourceRoot: root,
      policy: <String, dynamic>{
        'schemaVersion': 1,
        'exceptions': <dynamic>[
          <String, dynamic>{
            'path': source.path.replaceAll(r'\', '/'),
            'rule': 'rawDioConstruction',
            'allowedOccurrences': 1,
          },
        ],
      },
    );

    expect(result.failures, contains(contains('missing owner')));
    expect(result.failures, contains(contains('missing rationale')));
    expect(result.failures, contains(contains('missing removalCondition')));
  });
}
