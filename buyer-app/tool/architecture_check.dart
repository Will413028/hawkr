import 'dart:convert';
import 'dart:io';

const _policyPath = 'tool/presentation_io_policy.json';
const _permittedLegacyExceptionKeys = <String>{
  'presentation-direct-dio:lib/features/cart/presentation/widgets/checkout_payment_sheet.dart',
  'presentation-direct-dio:lib/features/category/presentation/category_landing_screen.dart',
  'presentation-direct-dio:lib/features/home/presentation/home_screen.dart',
  'presentation-direct-dio:lib/features/product/presentation/search_screen.dart',
  'presentation-direct-dio:lib/features/seller/presentation/merchant_screen.dart',
  'presentation-direct-dio:lib/features/seller/presentation/widgets/merchant_chat_tab.dart',
  'presentation-direct-dio:lib/features/seller/presentation/widgets/merchant_home_tab.dart',
  'presentation-direct-dio:lib/features/seller/presentation/widgets/merchant_shop_tab.dart',
};

enum PresentationIoRule {
  dio(
    id: 'presentation-direct-dio',
    description: 'Presentation code must not import or obtain Dio.',
  ),
  sharedPreferences(
    id: 'presentation-direct-shared-preferences',
    description: 'Presentation code must not import or open SharedPreferences.',
  ),
  connect(
    id: 'presentation-direct-connect',
    description:
        'Presentation code must not import or obtain a Connect client or '
        'transport.',
  );

  const PresentationIoRule({required this.id, required this.description});

  final String id;
  final String description;

  bool matches(String source) {
    return switch (this) {
      PresentationIoRule.dio =>
        RegExp(
              r'''import\s+['"]package:dio/dio\.dart['"]''',
            ).hasMatch(source) ||
            RegExp(
              r'''import\s+['"][^'"]*core/networking/dio_provider\.dart['"]''',
            ).hasMatch(source) ||
            RegExp(
              r'\b(?:ref|widget\.ref)\.(?:read|watch)\(dioProvider',
            ).hasMatch(source),
      PresentationIoRule.sharedPreferences =>
        RegExp(
              r'''import\s+['"]package:shared_preferences/shared_preferences\.dart['"]''',
            ).hasMatch(source) ||
            RegExp(r'\bSharedPreferences\.getInstance\s*\(').hasMatch(source),
      // proto 的訊息型別（*.pb.dart）是資料，可以用；client 與 transport 是 I/O。
      PresentationIoRule.connect =>
        RegExp(
              r'''import\s+['"][^'"]*core/networking/connect_provider\.dart['"]''',
            ).hasMatch(source) ||
            RegExp(
              r'''import\s+['"]package:hawkr_api/(?:transport\.dart|gen/[^'"]*\.connect\.client\.dart)['"]''',
            ).hasMatch(source),
    };
  }
}

final class PresentationIoException {
  const PresentationIoException({
    required this.rule,
    required this.path,
    required this.owner,
    required this.rationale,
    required this.removalCondition,
  });

  factory PresentationIoException.fromJson(Map<String, dynamic> json) {
    String readRequired(String key) {
      final value = json[key]?.toString().trim() ?? '';
      if (value.isEmpty) {
        throw FormatException('Architecture exception is missing "$key".');
      }
      return value;
    }

    final ruleId = readRequired('rule');
    final rule = PresentationIoRule.values.where((item) {
      return item.id == ruleId;
    }).firstOrNull;
    if (rule == null) {
      throw FormatException('Unknown presentation I/O rule "$ruleId".');
    }

    return PresentationIoException(
      rule: rule,
      path: readRequired('path'),
      owner: readRequired('owner'),
      rationale: readRequired('rationale'),
      removalCondition: readRequired('removalCondition'),
    );
  }

  final PresentationIoRule rule;
  final String path;
  final String owner;
  final String rationale;
  final String removalCondition;

  String get key => '${rule.id}:$path';
}

final class ArchitectureCheckResult {
  const ArchitectureCheckResult({
    required this.checkedFiles,
    required this.exceptions,
    required this.errors,
  });

  final int checkedFiles;
  final int exceptions;
  final List<String> errors;

  bool get isSuccess => errors.isEmpty;
}

Future<ArchitectureCheckResult> checkPresentationIo({
  Directory? repositoryRoot,
}) async {
  final root = repositoryRoot ?? Directory.current;
  final errors = <String>[];
  final exceptions = <String, PresentationIoException>{};

  final policyFile = File('${root.path}/$_policyPath');
  if (!policyFile.existsSync()) {
    return ArchitectureCheckResult(
      checkedFiles: 0,
      exceptions: 0,
      errors: ['Missing architecture policy: $_policyPath'],
    );
  }

  try {
    final decoded = jsonDecode(await policyFile.readAsString());
    if (decoded is! Map<String, dynamic>) {
      throw const FormatException('Policy root must be a JSON object.');
    }
    final rawExceptions = decoded['exceptions'];
    if (rawExceptions is! List) {
      throw const FormatException('Policy "exceptions" must be a list.');
    }
    for (final raw in rawExceptions) {
      if (raw is! Map) {
        throw const FormatException('Every exception must be a JSON object.');
      }
      final exception = PresentationIoException.fromJson(
        Map<String, dynamic>.from(raw),
      );
      if (!exception.path.startsWith('lib/') ||
          !exception.path.contains('/presentation/') ||
          !exception.path.endsWith('.dart')) {
        errors.add(
          '${exception.key}: exception path must target a presentation Dart file.',
        );
        continue;
      }
      if (!_permittedLegacyExceptionKeys.contains(exception.key)) {
        errors.add(
          '${exception.key}: new architecture exceptions are not permitted.',
        );
        continue;
      }
      if (exceptions.containsKey(exception.key)) {
        errors.add('${exception.key}: duplicate exception.');
        continue;
      }
      exceptions[exception.key] = exception;
    }
  } on Object catch (error) {
    return ArchitectureCheckResult(
      checkedFiles: 0,
      exceptions: 0,
      errors: ['Invalid architecture policy: $error'],
    );
  }

  final presentationFiles =
      root
          .listSync(recursive: true, followLinks: false)
          .whereType<File>()
          .where((file) {
            final relative = _relativePath(root, file);
            return relative.startsWith('lib/') &&
                relative.contains('/presentation/') &&
                relative.endsWith('.dart') &&
                !relative.endsWith('.g.dart') &&
                !relative.endsWith('.freezed.dart');
          })
          .toList()
        ..sort((a, b) => a.path.compareTo(b.path));

  final usedExceptions = <String>{};
  for (final file in presentationFiles) {
    final relative = _relativePath(root, file);
    final source = await file.readAsString();
    for (final rule in PresentationIoRule.values) {
      if (!rule.matches(source)) continue;
      final key = '${rule.id}:$relative';
      if (exceptions.containsKey(key)) {
        usedExceptions.add(key);
      } else {
        errors.add(
          '$relative: ${rule.description} '
          'Move I/O behind service -> repository -> Riverpod controller.',
        );
      }
    }
  }

  for (final exception in exceptions.values) {
    final file = File('${root.path}/${exception.path}');
    if (!file.existsSync()) {
      errors.add('${exception.key}: exception target does not exist.');
      continue;
    }
    if (!usedExceptions.contains(exception.key)) {
      errors.add(
        '${exception.key}: stale exception; remove it from $_policyPath.',
      );
    }
  }

  return ArchitectureCheckResult(
    checkedFiles: presentationFiles.length,
    exceptions: exceptions.length,
    errors: List.unmodifiable(errors),
  );
}

String _relativePath(Directory root, File file) {
  final prefix = '${root.absolute.path}${Platform.pathSeparator}';
  return file.absolute.path.replaceFirst(prefix, '').replaceAll(r'\', '/');
}

Future<void> main() async {
  final result = await checkPresentationIo();
  if (!result.isSuccess) {
    stderr.writeln('Presentation I/O architecture gate failed:');
    for (final error in result.errors) {
      stderr.writeln('- $error');
    }
    exitCode = 1;
    return;
  }
  stdout.writeln(
    'Presentation I/O architecture gate passed '
    '(${result.checkedFiles} files, ${result.exceptions} ratcheted exceptions).',
  );
}
