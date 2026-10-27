import 'dart:convert';
import 'dart:io';

const coverageSourceCommand =
    'flutter test --no-pub --exclude-tags hermetic-integration '
    '--coverage --coverage-path coverage/lcov.info';

const generatedCoverageSuffixes = ['.g.dart', '.freezed.dart', '.mocks.dart'];

class CoverageSnapshot {
  const CoverageSnapshot({
    required this.coveredLines,
    required this.instrumentedLines,
    required this.instrumentedFiles,
  });

  final int coveredLines;
  final int instrumentedLines;
  final Set<String> instrumentedFiles;

  double get ratio =>
      instrumentedLines == 0 ? 0 : coveredLines / instrumentedLines;
}

class CoverageBaseline {
  const CoverageBaseline({
    required this.coveredLines,
    required this.instrumentedLines,
    required this.instrumentedFiles,
    required this.legacyUninstrumentedFiles,
  });

  factory CoverageBaseline.fromJson(Map<String, dynamic> json) {
    if (json['schemaVersion'] != 1) {
      throw const FormatException('Unsupported coverage baseline schema');
    }
    final covered = json['coveredLines'];
    final instrumented = json['instrumentedLines'];
    if (covered is! int ||
        instrumented is! int ||
        covered < 0 ||
        instrumented <= 0 ||
        covered > instrumented) {
      throw const FormatException('Invalid coverage baseline line counts');
    }
    return CoverageBaseline(
      coveredLines: covered,
      instrumentedLines: instrumented,
      instrumentedFiles: _stringSet(json['instrumentedFiles']),
      legacyUninstrumentedFiles: _stringSet(json['legacyUninstrumentedFiles']),
    );
  }

  final int coveredLines;
  final int instrumentedLines;
  final Set<String> instrumentedFiles;
  final Set<String> legacyUninstrumentedFiles;

  Set<String> get sourceFiles => {
    ...instrumentedFiles,
    ...legacyUninstrumentedFiles,
  };

  double get ratio => coveredLines / instrumentedLines;
}

CoverageSnapshot parseLcov(String contents, {String? workspacePath}) {
  final root = _normalize(
    Directory(workspacePath ?? Directory.current.path).absolute.path,
  );
  final instrumentedByFile = <String, Set<int>>{};
  final coveredByFile = <String, Set<int>>{};
  String? currentFile;

  for (final line in const LineSplitter().convert(contents)) {
    if (line.startsWith('SF:')) {
      currentFile = normalizeCoveragePath(line.substring(3), root);
      if (currentFile != null && isEligibleProductionSource(currentFile)) {
        instrumentedByFile.putIfAbsent(currentFile, () => <int>{});
        coveredByFile.putIfAbsent(currentFile, () => <int>{});
      } else {
        currentFile = null;
      }
      continue;
    }
    if (currentFile == null || !line.startsWith('DA:')) continue;
    final values = line.substring(3).split(',');
    if (values.length < 2) continue;
    final lineNumber = int.tryParse(values[0]);
    final hits = int.tryParse(values[1]);
    if (lineNumber == null || hits == null) continue;
    instrumentedByFile[currentFile]!.add(lineNumber);
    if (hits > 0) coveredByFile[currentFile]!.add(lineNumber);
  }

  return CoverageSnapshot(
    coveredLines: coveredByFile.values.fold(
      0,
      (sum, lines) => sum + lines.length,
    ),
    instrumentedLines: instrumentedByFile.values.fold(
      0,
      (sum, lines) => sum + lines.length,
    ),
    instrumentedFiles: instrumentedByFile.keys.toSet(),
  );
}

String? normalizeCoveragePath(String rawPath, String workspacePath) {
  var path = rawPath.trim();
  if (path.startsWith('file:')) path = Uri.parse(path).toFilePath();
  path = _normalize(path);
  final root = workspacePath.endsWith('/')
      ? workspacePath.substring(0, workspacePath.length - 1)
      : workspacePath;
  if (path.startsWith('$root/')) {
    path = path.substring(root.length + 1);
  } else if (path.startsWith('/')) {
    // Never count Flutter SDK or pub-cache sources that contain `/lib/`.
    return null;
  }
  while (path.startsWith('./')) {
    path = path.substring(2);
  }
  return path.startsWith('lib/') ? path : null;
}

bool isEligibleProductionSource(String path) {
  if (!path.startsWith('lib/') || !path.endsWith('.dart')) return false;
  return !generatedCoverageSuffixes.any(path.endsWith);
}

Set<String> listProductionSources(Directory workspace) {
  final lib = Directory('${workspace.path}/lib');
  if (!lib.existsSync()) return {};
  final root = _normalize(workspace.absolute.path);
  return lib
      .listSync(recursive: true, followLinks: false)
      .whereType<File>()
      .map((file) => _normalize(file.absolute.path))
      .where((path) => path.startsWith('$root/'))
      .map((path) => path.substring(root.length + 1))
      .where(isEligibleProductionSource)
      .toSet();
}

List<String> evaluateCoverage({
  required CoverageSnapshot current,
  required CoverageBaseline baseline,
  required Set<String> currentSources,
  String baselineLabel = 'checked-in baseline',
}) {
  final failures = <String>[];
  if (current.instrumentedLines == 0) {
    return ['LCOV contains no eligible production lines.'];
  }

  // Compare exact fractions; rounded display percentages never decide a gate.
  if (current.coveredLines * baseline.instrumentedLines <
      baseline.coveredLines * current.instrumentedLines) {
    failures.add(
      'Coverage fell below $baselineLabel: '
      '${formatPercent(current.ratio)} '
      '(${current.coveredLines}/${current.instrumentedLines}) < '
      '${formatPercent(baseline.ratio)} '
      '(${baseline.coveredLines}/${baseline.instrumentedLines}).',
    );
  }

  final dropped =
      baseline.instrumentedFiles
          .intersection(currentSources)
          .difference(current.instrumentedFiles)
          .toList()
        ..sort();
  for (final path in dropped) {
    failures.add('Previously instrumented source disappeared from LCOV: $path');
  }

  final newSources = currentSources.difference(baseline.sourceFiles).toList()
    ..sort();
  for (final path in newSources) {
    if (!current.instrumentedFiles.contains(path)) {
      failures.add('New production source is not exercised by any test: $path');
    }
  }
  return failures;
}

List<String> evaluateBaselineFloor({
  required CoverageBaseline candidate,
  required CoverageBaseline floor,
  required Set<String> currentSources,
}) {
  final failures = <String>[];
  if (candidate.coveredLines * floor.instrumentedLines <
      floor.coveredLines * candidate.instrumentedLines) {
    failures.add(
      'Checked-in baseline cannot be lowered: '
      '${formatPercent(candidate.ratio)} < ${formatPercent(floor.ratio)}.',
    );
  }

  final dropped =
      floor.instrumentedFiles
          .intersection(currentSources)
          .difference(candidate.instrumentedFiles)
          .toList()
        ..sort();
  for (final path in dropped) {
    failures.add(
      'Checked-in baseline removed an existing instrumented source: $path',
    );
  }

  final newExceptions =
      candidate.legacyUninstrumentedFiles
          .difference(floor.legacyUninstrumentedFiles)
          .intersection(currentSources)
          .toList()
        ..sort();
  for (final path in newExceptions) {
    failures.add(
      'Checked-in baseline cannot add an uninstrumented exception: $path',
    );
  }
  return failures;
}

Map<String, dynamic> baselineJson({
  required CoverageSnapshot snapshot,
  required Set<String> sourceFiles,
}) {
  final instrumented = snapshot.instrumentedFiles.toList()..sort();
  final legacyUninstrumented =
      sourceFiles.difference(snapshot.instrumentedFiles).toList()..sort();
  return {
    'schemaVersion': 1,
    'sourceCommand': coverageSourceCommand,
    'excludedSuffixes': generatedCoverageSuffixes,
    'coveredLines': snapshot.coveredLines,
    'instrumentedLines': snapshot.instrumentedLines,
    'instrumentedFiles': instrumented,
    'legacyUninstrumentedFiles': legacyUninstrumented,
  };
}

String formatPercent(double ratio) => '${(ratio * 100).toStringAsFixed(2)}%';

void main(List<String> arguments) {
  try {
    final options = _parseArguments(arguments);
    final workspace = Directory(options['workspace'] ?? Directory.current.path);
    final lcovFile = File(
      options['lcov'] ?? '${workspace.path}/coverage/lcov.info',
    );
    final baselineFile = File(
      options['baseline'] ?? '${workspace.path}/tool/coverage_baseline.json',
    );
    final summaryFile = File(
      options['summary'] ?? '${workspace.path}/coverage/coverage-summary.json',
    );

    if (!lcovFile.existsSync()) {
      throw FileSystemException('LCOV file does not exist', lcovFile.path);
    }
    final snapshot = parseLcov(
      lcovFile.readAsStringSync(),
      workspacePath: workspace.path,
    );
    final currentSources = listProductionSources(workspace);

    if (options.containsKey('print-baseline')) {
      stdout.writeln(
        const JsonEncoder.withIndent('  ').convert(
          baselineJson(snapshot: snapshot, sourceFiles: currentSources),
        ),
      );
      return;
    }

    final baseline = _readBaseline(baselineFile);
    final failures = evaluateCoverage(
      current: snapshot,
      baseline: baseline,
      currentSources: currentSources,
    );

    CoverageBaseline? floor;
    final floorPath = options['floor-baseline'];
    if (floorPath != null) {
      floor = _readBaseline(File(floorPath));
      failures.addAll(
        evaluateCoverage(
          current: snapshot,
          baseline: floor,
          currentSources: currentSources,
          baselineLabel: 'base-branch floor',
        ),
      );
      failures.addAll(
        evaluateBaselineFloor(
          candidate: baseline,
          floor: floor,
          currentSources: currentSources,
        ),
      );
    }

    final summary = {
      'schemaVersion': 1,
      'status': failures.isEmpty ? 'passed' : 'failed',
      'current': {
        'coveredLines': snapshot.coveredLines,
        'instrumentedLines': snapshot.instrumentedLines,
        'percent': double.parse((snapshot.ratio * 100).toStringAsFixed(4)),
        'instrumentedFiles': snapshot.instrumentedFiles.length,
      },
      'baseline': {
        'coveredLines': baseline.coveredLines,
        'instrumentedLines': baseline.instrumentedLines,
        'percent': double.parse((baseline.ratio * 100).toStringAsFixed(4)),
      },
      if (floor != null)
        'baseBranchFloor': {
          'coveredLines': floor.coveredLines,
          'instrumentedLines': floor.instrumentedLines,
          'percent': double.parse((floor.ratio * 100).toStringAsFixed(4)),
        },
      'failures': failures,
    };
    summaryFile.parent.createSync(recursive: true);
    summaryFile.writeAsStringSync(
      '${const JsonEncoder.withIndent('  ').convert(summary)}\n',
    );

    if (failures.isNotEmpty) {
      stderr.writeln('Coverage ratchet failed:');
      for (final failure in failures) {
        stderr.writeln(' - $failure');
      }
      exitCode = 1;
      return;
    }
    stdout.writeln(
      'Coverage ratchet passed: ${formatPercent(snapshot.ratio)} '
      '(${snapshot.coveredLines}/${snapshot.instrumentedLines}), '
      '${snapshot.instrumentedFiles.length} instrumented files.',
    );
  } on Object catch (error) {
    stderr.writeln('Coverage ratchet could not run: $error');
    exitCode = 2;
  }
}

CoverageBaseline _readBaseline(File file) {
  if (!file.existsSync()) {
    throw FileSystemException('Coverage baseline does not exist', file.path);
  }
  final json = jsonDecode(file.readAsStringSync());
  if (json is! Map<String, dynamic>) {
    throw const FormatException('Coverage baseline must be a JSON object');
  }
  return CoverageBaseline.fromJson(json);
}

Map<String, String?> _parseArguments(List<String> arguments) {
  final result = <String, String?>{};
  for (var index = 0; index < arguments.length; index++) {
    final argument = arguments[index];
    if (!argument.startsWith('--')) {
      throw FormatException('Unexpected argument: $argument');
    }
    final equals = argument.indexOf('=');
    if (equals >= 0) {
      result[argument.substring(2, equals)] = argument.substring(equals + 1);
      continue;
    }
    final key = argument.substring(2);
    if (key == 'print-baseline') {
      result[key] = null;
      continue;
    }
    if (index + 1 >= arguments.length ||
        arguments[index + 1].startsWith('--')) {
      throw FormatException('Missing value for --$key');
    }
    result[key] = arguments[++index];
  }
  return result;
}

Set<String> _stringSet(Object? value) {
  if (value is! List) {
    throw const FormatException('Expected a JSON string list');
  }
  final strings = value.whereType<String>().toSet();
  if (strings.length != value.length) {
    throw const FormatException('Expected a JSON string list');
  }
  return strings;
}

String _normalize(String path) => path.replaceAll(r'\', '/');
