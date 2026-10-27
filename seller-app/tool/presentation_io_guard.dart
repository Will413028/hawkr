import 'dart:convert';
import 'dart:io';

const String defaultPresentationPolicyPath = 'tool/presentation_io_policy.json';

final List<PresentationIoRule> presentationIoRules = [
  PresentationIoRule(
    id: 'dioPackageImport',
    description: 'Presentation imports Dio directly',
    pattern: RegExp(
      r'''^\s*import\s+['"]package:dio/dio\.dart['"];\s*$''',
      multiLine: true,
    ),
  ),
  PresentationIoRule(
    id: 'dioProviderAccess',
    description: 'Presentation reads the central Dio client directly',
    pattern: RegExp(
      r'\b(?:ref|container)\s*\.\s*(?:read|watch)\s*'
      r'\(\s*dioProvider\s*\)',
    ),
  ),
  PresentationIoRule(
    id: 'apiRequestHelperImport',
    description: 'Presentation imports the low-level API request helper',
    pattern: RegExp(
      r'''^\s*import\s+['"][^'"]*core/networking/'''
      r'''api_request_helper\.dart['"];\s*$''',
      multiLine: true,
    ),
  ),
  PresentationIoRule(
    id: 'sharedPreferencesImport',
    description: 'Presentation imports SharedPreferences directly',
    pattern: RegExp(
      r'''^\s*import\s+['"]package:shared_preferences/'''
      r'''shared_preferences\.dart['"];\s*$''',
      multiLine: true,
    ),
  ),
  PresentationIoRule(
    id: 'sharedPreferencesAccess',
    description: 'Presentation opens SharedPreferences directly',
    pattern: RegExp(r'\bSharedPreferences\s*\.\s*getInstance\s*\('),
  ),
  PresentationIoRule(
    id: 'directApiHelperCall',
    description: 'Presentation calls a repository low-level API helper',
    pattern: RegExp(
      r'\.\s*api(?:Get|Post|Put|Patch|Delete)'
      r'(?:OrThrow|Result)?\s*\(',
    ),
  ),
  PresentationIoRule(
    id: 'rawDioConstruction',
    description: 'Presentation constructs a raw Dio client',
    pattern: RegExp(r'\bDio\s*\('),
  ),
  PresentationIoRule(
    id: 'dartHttpClientConstruction',
    description: 'Presentation constructs a dart:io HTTP client',
    pattern: RegExp(r'\bHttpClient\s*\('),
  ),
];

final class PresentationIoRule {
  const PresentationIoRule({
    required this.id,
    required this.description,
    required this.pattern,
  });

  final String id;
  final String description;
  final RegExp pattern;
}

final class PresentationIoViolation {
  const PresentationIoViolation({
    required this.path,
    required this.rule,
    required this.line,
    required this.snippet,
  });

  final String path;
  final PresentationIoRule rule;
  final int line;
  final String snippet;
}

final class PresentationIoGuardResult {
  const PresentationIoGuardResult({
    required this.failures,
    required this.violations,
  });

  final List<String> failures;
  final List<PresentationIoViolation> violations;

  bool get passed => failures.isEmpty;
}

PresentationIoGuardResult evaluatePresentationIoGuard({
  required Directory sourceRoot,
  required Map<String, dynamic> policy,
}) {
  final failures = <String>[];
  final violations = scanPresentationIo(sourceRoot);
  final rulesById = {for (final rule in presentationIoRules) rule.id: rule};
  final exceptions = <String, Map<String, dynamic>>{};

  if (policy['schemaVersion'] != 1) {
    failures.add('Policy schemaVersion must be 1.');
  }
  final rawExceptions = policy['exceptions'];
  if (rawExceptions is! List) {
    failures.add('Policy exceptions must be a list.');
  } else {
    for (final rawEntry in rawExceptions) {
      if (rawEntry is! Map) {
        failures.add('Policy exception must be an object: $rawEntry');
        continue;
      }
      final entry = Map<String, dynamic>.from(rawEntry);
      final path = entry['path']?.toString().trim() ?? '';
      final ruleId = entry['rule']?.toString().trim() ?? '';
      final key = '$path::$ruleId';
      if (path.isEmpty || !path.contains('/presentation/')) {
        failures.add('Policy exception has invalid presentation path: $entry');
      }
      if (!rulesById.containsKey(ruleId)) {
        failures.add('Policy exception has unknown rule "$ruleId": $entry');
      }
      final allowed = entry['allowedOccurrences'];
      if (allowed is! int || allowed <= 0) {
        failures.add(
          'Policy exception must have positive allowedOccurrences: $entry',
        );
      }
      for (final field in ['owner', 'rationale', 'removalCondition']) {
        if ((entry[field]?.toString().trim() ?? '').isEmpty) {
          failures.add('Policy exception is missing $field: $entry');
        }
      }
      if (exceptions.containsKey(key)) {
        failures.add('Duplicate policy exception: $key');
      } else {
        exceptions[key] = entry;
      }
    }
  }

  final grouped = <String, List<PresentationIoViolation>>{};
  for (final violation in violations) {
    final key = '${violation.path}::${violation.rule.id}';
    grouped.putIfAbsent(key, () => []).add(violation);
  }

  for (final entry in grouped.entries) {
    final exception = exceptions[entry.key];
    if (exception == null) {
      for (final violation in entry.value) {
        failures.add(
          'New presentation I/O: ${violation.path}:${violation.line} '
          '[${violation.rule.id}] ${violation.snippet}',
        );
      }
      continue;
    }
    final allowed = exception['allowedOccurrences'] as int;
    if (entry.value.length != allowed) {
      failures.add(
        '${entry.key} has ${entry.value.length} occurrence(s); '
        'policy records $allowed. Ratchet the policy in the same change.',
      );
    }
  }

  for (final entry in exceptions.entries) {
    if (!grouped.containsKey(entry.key)) {
      failures.add(
        'Stale presentation I/O exception ${entry.key}; remove it from policy.',
      );
    }
  }

  return PresentationIoGuardResult(
    failures: List<String>.unmodifiable(failures),
    violations: List<PresentationIoViolation>.unmodifiable(violations),
  );
}

List<PresentationIoViolation> scanPresentationIo(Directory sourceRoot) {
  if (!sourceRoot.existsSync()) return const [];
  final violations = <PresentationIoViolation>[];
  final files =
      sourceRoot
          .listSync(recursive: true)
          .whereType<File>()
          .where((file) => _isPresentationSource(file.path))
          .toList()
        ..sort((a, b) => a.path.compareTo(b.path));

  for (final file in files) {
    final content = file.readAsStringSync();
    final path = file.path.replaceAll(r'\', '/');
    for (final rule in presentationIoRules) {
      for (final match in rule.pattern.allMatches(content)) {
        final line =
            '\n'.allMatches(content.substring(0, match.start)).length + 1;
        final lineEnd = content.indexOf('\n', match.start);
        final snippet = content
            .substring(match.start, lineEnd == -1 ? content.length : lineEnd)
            .trim()
            .replaceAll(RegExp(r'\s+'), ' ');
        violations.add(
          PresentationIoViolation(
            path: path,
            rule: rule,
            line: line,
            snippet: snippet,
          ),
        );
      }
    }
  }
  return violations;
}

bool _isPresentationSource(String rawPath) {
  final path = rawPath.replaceAll(r'\', '/');
  return path.contains('/presentation/') &&
      path.endsWith('.dart') &&
      !path.endsWith('.g.dart') &&
      !path.endsWith('.freezed.dart');
}

Map<String, dynamic> readPresentationIoPolicy(String path) {
  return Map<String, dynamic>.from(
    jsonDecode(File(path).readAsStringSync()) as Map,
  );
}

void main(List<String> args) {
  final policyPath = args.isEmpty ? defaultPresentationPolicyPath : args.first;
  final result = evaluatePresentationIoGuard(
    sourceRoot: Directory('lib/features'),
    policy: readPresentationIoPolicy(policyPath),
  );

  if (!result.passed) {
    stderr.writeln('Presentation I/O architecture guard failed:');
    for (final failure in result.failures) {
      stderr.writeln(' - $failure');
    }
    exitCode = 1;
    return;
  }

  stdout.writeln(
    'Presentation I/O architecture guard passed '
    'with ${result.violations.length} named legacy occurrence(s).',
  );
}
