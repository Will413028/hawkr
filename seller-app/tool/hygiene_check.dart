import 'dart:convert';
import 'dart:io';

void main() {
  final failures = <String>[];
  final policy =
      jsonDecode(File('tool/hygiene_policy.json').readAsStringSync())
          as Map<String, dynamic>;
  final pubspec = File('pubspec.yaml').readAsStringSync();

  _checkBoundedDependencies(pubspec, failures);
  _checkUniqueAssetDeclarations(pubspec, failures);
  _checkPolicyMetadata(policy, failures);
  _checkDesignPolicyRatchet(policy, failures);

  for (final dependency
      in (policy['forbiddenDependencies'] as List<dynamic>).cast<String>()) {
    if (RegExp(
      '^  ${RegExp.escape(dependency)}:',
      multiLine: true,
    ).hasMatch(pubspec)) {
      failures.add('Forbidden/discontinued dependency returned: $dependency');
    }
  }

  final assetFiles = Directory(
    'assets',
  ).listSync(recursive: true).whereType<File>().toList();
  final totalAssetBytes = assetFiles.fold<int>(
    0,
    (total, file) => total + file.lengthSync(),
  );
  final maxSourceAssetsBytes = policy['maxSourceAssetsBytes'] as int;
  if (totalAssetBytes > maxSourceAssetsBytes) {
    failures.add(
      'Source assets grew to $totalAssetBytes bytes; '
      'ratchet is $maxSourceAssetsBytes bytes.',
    );
  }

  final largeThreshold = policy['largeAssetThresholdBytes'] as int;
  final exceptionList =
      policy['largeAssetExceptions'] as List<dynamic>? ?? const [];
  final exceptions = <String, Map<String, dynamic>>{
    for (final value in exceptionList)
      (value as Map<String, dynamic>)['path'] as String: value,
  };
  for (final file in assetFiles.where(
    (candidate) => candidate.lengthSync() > largeThreshold,
  )) {
    final path = file.path;
    final exception = exceptions[path];
    if (exception == null) {
      failures.add(
        '$path is ${file.lengthSync()} bytes and has no named exception.',
      );
      continue;
    }
    final maxBytes = exception['maxBytes'] as int;
    if (file.lengthSync() > maxBytes) {
      failures.add('$path grew beyond its $maxBytes-byte exception budget.');
    }
  }
  for (final path in exceptions.keys) {
    if (!File(path).existsSync()) {
      failures.add('Large-asset exception points to missing file: $path');
    }
  }

  for (final path in (policy['removedPaths'] as List<dynamic>).cast<String>()) {
    if (FileSystemEntity.typeSync(path) != FileSystemEntityType.notFound) {
      failures.add('Resolved dead-code/asset candidate returned: $path');
    }
  }

  final readme = File('README.md').readAsStringSync();
  for (final token
      in (policy['requiredDocumentationTokens'] as List<dynamic>)
          .cast<String>()) {
    if (!readme.contains(token)) {
      failures.add('README toolchain token is missing: $token');
    }
  }

  if (failures.isNotEmpty) {
    stderr.writeln('Hygiene policy failed:');
    for (final failure in failures) {
      stderr.writeln(' - $failure');
    }
    exitCode = 1;
    return;
  }
  stdout.writeln(
    'Hygiene policy passed: ${assetFiles.length} assets, '
    '$totalAssetBytes source bytes.',
  );
}

void _checkBoundedDependencies(String pubspec, List<String> failures) {
  var section = '';
  for (final line in const LineSplitter().convert(pubspec)) {
    final topLevel = RegExp(r'^([a-zA-Z_][\w]*):\s*$').firstMatch(line);
    if (topLevel != null) section = topLevel.group(1)!;
    if (section != 'dependencies' && section != 'dev_dependencies') continue;
    final unbounded = RegExp(
      r'^\s{2}([\w]+):\s*any(?:\s*#.*)?$',
    ).firstMatch(line);
    if (unbounded != null) {
      failures.add('Unbounded dependency: ${unbounded.group(1)}');
    }
  }
}

void _checkUniqueAssetDeclarations(String pubspec, List<String> failures) {
  final declarations = RegExp(
    r'^\s+-\s+(assets/[^\s#]+(?: [^\s#]+)*/?)\s*$',
    multiLine: true,
  ).allMatches(pubspec).map((match) => match.group(1)!).toList();
  final seen = <String>{};
  final duplicates = declarations.where((path) => !seen.add(path)).toSet();
  if (duplicates.isNotEmpty) {
    failures.add('Duplicate asset declarations: ${duplicates.join(', ')}');
  }
}

void _checkPolicyMetadata(Map<String, dynamic> policy, List<String> failures) {
  for (final key in [
    'largeAssetExceptions',
    'unusedCodeExceptions',
    'policyExceptions',
  ]) {
    final entries = policy[key] as List<dynamic>? ?? const [];
    for (final value in entries) {
      final entry = value as Map<String, dynamic>;
      for (final field in ['owner', 'rationale', 'removalCondition']) {
        if ((entry[field]?.toString().trim() ?? '').isEmpty) {
          failures.add('$key entry is missing $field: $entry');
        }
      }
    }
  }
}

void _checkDesignPolicyRatchet(
  Map<String, dynamic> policy,
  List<String> failures,
) {
  final exception = (policy['policyExceptions'] as List<dynamic>)
      .cast<Map<String, dynamic>>()
      .where((entry) => entry['policy'] == 'screenUtilFontSize')
      .single;
  final pattern = RegExp(r'fontSize\s*:\s*\d+(?:\.\d+)?\.sp\b');
  var occurrences = 0;
  for (final file
      in Directory('lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((file) => file.path.endsWith('.dart'))) {
    occurrences += pattern.allMatches(file.readAsStringSync()).length;
  }
  final maximum = exception['maxOccurrences'] as int;
  if (occurrences > maximum) {
    failures.add(
      'ScreenUtil fontSize usage grew to $occurrences; '
      'incremental ratchet is $maximum.',
    );
  }
}
