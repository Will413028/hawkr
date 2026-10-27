import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../tool/coverage_ratchet.dart';

void main() {
  test('LCOV parser ignores generated sources and de-duplicates lines', () {
    final snapshot = parseLcov('''
SF:/workspace/lib/a.dart
DA:1,1
DA:2,0
end_of_record
SF:/workspace/lib/a.dart
DA:1,3
end_of_record
SF:/workspace/lib/a.g.dart
DA:1,1
end_of_record
''', workspacePath: '/workspace');

    expect(snapshot.coveredLines, 1);
    expect(snapshot.instrumentedLines, 2);
    expect(snapshot.instrumentedFiles, {'lib/a.dart'});
  });

  test('LCOV parser never counts package-cache sources', () {
    final snapshot = parseLcov('''
SF:/root/.pub-cache/hosted/pub.dev/dependency/lib/source.dart
DA:1,1
end_of_record
SF:/workspace/lib/a.dart
DA:1,1
end_of_record
''', workspacePath: '/workspace');

    expect(snapshot.instrumentedFiles, {'lib/a.dart'});
  });

  test('ratchet rejects a lower exact line ratio', () {
    const current = CoverageSnapshot(
      coveredLines: 7,
      instrumentedLines: 10,
      instrumentedFiles: {'lib/a.dart'},
    );
    const baseline = CoverageBaseline(
      coveredLines: 3,
      instrumentedLines: 4,
      instrumentedFiles: {'lib/a.dart'},
      legacyUninstrumentedFiles: {},
    );

    expect(
      evaluateCoverage(
        current: current,
        baseline: baseline,
        currentSources: {'lib/a.dart'},
      ),
      contains(contains('Coverage fell below')),
    );
  });

  test('ratchet rejects a new production file absent from LCOV', () {
    const current = CoverageSnapshot(
      coveredLines: 3,
      instrumentedLines: 4,
      instrumentedFiles: {'lib/a.dart'},
    );
    const baseline = CoverageBaseline(
      coveredLines: 3,
      instrumentedLines: 4,
      instrumentedFiles: {'lib/a.dart'},
      legacyUninstrumentedFiles: {},
    );

    expect(
      evaluateCoverage(
        current: current,
        baseline: baseline,
        currentSources: {'lib/a.dart', 'lib/new_feature.dart'},
      ),
      contains(
        'New production source is not exercised by any test: '
        'lib/new_feature.dart',
      ),
    );
  });

  test('base-branch floor rejects a newly grandfathered source', () {
    const floor = CoverageBaseline(
      coveredLines: 3,
      instrumentedLines: 4,
      instrumentedFiles: {'lib/a.dart'},
      legacyUninstrumentedFiles: {'lib/legacy.dart'},
    );
    const candidate = CoverageBaseline(
      coveredLines: 3,
      instrumentedLines: 4,
      instrumentedFiles: {'lib/a.dart'},
      legacyUninstrumentedFiles: {'lib/legacy.dart', 'lib/new_feature.dart'},
    );

    expect(
      evaluateBaselineFloor(
        candidate: candidate,
        floor: floor,
        currentSources: {
          'lib/a.dart',
          'lib/legacy.dart',
          'lib/new_feature.dart',
        },
      ),
      contains(
        'Checked-in baseline cannot add a new uninstrumented exception: '
        'lib/new_feature.dart',
      ),
    );
  });

  test('baseline metadata must match the reproducible command', () {
    expect(
      () => CoverageBaseline.fromJson({
        'schemaVersion': 1,
        'sourceCommand': 'flutter test --coverage',
        'excludedSuffixes': generatedCoverageSuffixes,
        'coveredLines': 1,
        'instrumentedLines': 1,
        'instrumentedFiles': ['lib/a.dart'],
        'legacyUninstrumentedFiles': <String>[],
      }),
      throwsFormatException,
    );
  });

  test('source inventory excludes generated Dart files', () {
    final root = Directory.systemTemp.createTempSync('coverage-ratchet-');
    addTearDown(() => root.deleteSync(recursive: true));
    Directory('${root.path}/lib').createSync();
    File('${root.path}/lib/a.dart').writeAsStringSync('void a() {}');
    File('${root.path}/lib/a.g.dart').writeAsStringSync('void generated() {}');

    expect(listProductionSources(root), {'lib/a.dart'});
  });
}
