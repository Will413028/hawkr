import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

final _assetLiteral = RegExp(r"""(['"])(assets/[^'"\n]*)\1""");
final _concatenated = RegExp(
  r"""(['"])assets/[^'"\n]*\1\s*\+|\+\s*(['"])assets/""",
);

List<File> _libDartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .toList();

List<String> _declaredAssets() {
  final declared = <String>[];
  var inAssets = false;
  for (final line in File('pubspec.yaml').readAsLinesSync()) {
    if (RegExp(r'^  assets:\s*$').hasMatch(line)) {
      inAssets = true;
      continue;
    }
    if (!inAssets) continue;
    final entry = RegExp(r'^    - (\S+)\s*$').firstMatch(line);
    if (entry != null) {
      declared.add(entry.group(1)!);
    } else if (line.trim().isNotEmpty && !line.trimLeft().startsWith('#')) {
      break;
    }
  }
  return declared;
}

bool _covered(String path, List<String> declared) {
  if (declared.contains(path)) return true;
  final slash = path.lastIndexOf('/');
  final dir = path.endsWith('/') ? path : path.substring(0, slash + 1);
  return declared.contains(dir);
}

void main() {
  final files = _libDartFiles();

  test('every assets/ literal in lib exists and is declared in pubspec', () {
    final declared = _declaredAssets();
    expect(declared, isNotEmpty);
    final problems = <String>[];
    var seen = 0;
    for (final file in files) {
      for (final m in _assetLiteral.allMatches(file.readAsStringSync())) {
        final path = m.group(2)!;
        if (path.contains(r'$')) continue;
        seen++;
        final exists = path.endsWith('/')
            ? Directory(path).existsSync()
            : File(path).existsSync();
        if (!exists) problems.add('${file.path}: $path 不存在');
        if (!_covered(path, declared)) {
          problems.add('${file.path}: $path 沒有被 pubspec.yaml 宣告');
        }
      }
    }
    expect(seen, greaterThan(0));
    expect(problems, isEmpty);
  });

  test('lib builds no assets/ path dynamically', () {
    final dynamicPaths = <String>[];
    for (final file in files) {
      final source = file.readAsStringSync();
      for (final m in _assetLiteral.allMatches(source)) {
        if (m.group(2)!.contains(r'$')) {
          dynamicPaths.add('${file.path}: ${m.group(0)}');
        }
      }
      for (final m in _concatenated.allMatches(source)) {
        dynamicPaths.add('${file.path}: ${m.group(0)}');
      }
    }
    expect(dynamicPaths, isEmpty);
  });

  test('pubspec declares no empty assets directory', () {
    final emptyDeclared = <String>[
      for (final entry in _declaredAssets())
        if (entry.endsWith('/') &&
            (!Directory(entry).existsSync() ||
                !Directory(entry).listSync().any((e) => e is File)))
          entry,
    ];
    expect(emptyDeclared, isEmpty);
  });
}
