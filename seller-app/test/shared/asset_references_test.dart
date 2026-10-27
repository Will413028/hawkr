import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

final _assetLiteral = RegExp(r'''(['"])(assets/[^'"]*)\1''');
final _dynamicAsset = RegExp(
  r'''(['"])assets/[^'"]*\$|(['"])assets/[^'"]*\2\s*\+|\+\s*(['"])assets/''',
);

List<File> _dartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'))
    .toList();

List<String> _declaredAssets() {
  final lines = File('pubspec.yaml').readAsLinesSync();
  final start = lines.indexWhere((line) => line == '  assets:');
  expect(start, isNonNegative, reason: 'pubspec.yaml has no flutter assets');
  final declared = <String>[];
  for (final line in lines.skip(start + 1)) {
    final trimmed = line.trim();
    if (trimmed.isEmpty || trimmed.startsWith('#')) continue;
    if (!trimmed.startsWith('- ')) break;
    declared.add(trimmed.substring(2).trim());
  }
  return declared;
}

bool _covered(String asset, List<String> declared) => declared.any(
  (entry) => entry == asset || (entry.endsWith('/') && asset.startsWith(entry)),
);

void main() {
  final files = _dartFiles();

  test('lib has Dart files to scan', () {
    expect(files, isNotEmpty);
  });

  test('every asset literal in lib exists and is declared in pubspec.yaml', () {
    final declared = _declaredAssets();
    final problems = <String>[];
    for (final file in files) {
      final source = file.readAsStringSync();
      for (final match in _assetLiteral.allMatches(source)) {
        final asset = match.group(2)!;
        if (!File(asset).existsSync()) {
          problems.add('${file.path}: $asset is not on disk');
        } else if (!_covered(asset, declared)) {
          problems.add('${file.path}: $asset is not declared in pubspec.yaml');
        }
      }
    }
    expect(problems, isEmpty);
  });

  test('asset paths are never built dynamically', () {
    final dynamicPaths = <String>[
      for (final file in files)
        for (final match in _dynamicAsset.allMatches(file.readAsStringSync()))
          '${file.path}: ${match.group(0)}',
    ];
    expect(dynamicPaths, isEmpty);
  });
}
