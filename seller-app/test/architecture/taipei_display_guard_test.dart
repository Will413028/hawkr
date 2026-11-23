import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// 臺北時間顯示的兩道守門：
/// - `toLocal(` 編譯器看不到，由 CI 的 `scripts/check-taipei-display.py` 擋；這裡驗腳本會擋、目前的 lib 是乾淨的。
/// - 牆上時間的欄位（`.taipei`、`.taipeiWall`、`formatTaipei*`）是 `hawkr_api` 函式庫私有的，App 讀了就編譯不過；
///   這裡用分析器實際編譯一個探針檔，確認這些寫法仍然是錯誤。
const _script = '../scripts/check-taipei-display.py';

Future<ProcessResult> _run(String lib) =>
    Process.run('python3', [_script, lib]);

Future<ProcessResult> _runOn(Map<String, String> files) async {
  final dir = Directory.systemTemp.createTempSync('taipei_guard_');
  addTearDown(() => dir.deleteSync(recursive: true));
  final lib = Directory('${dir.path}/lib')..createSync();
  files.forEach((name, text) {
    final file = File('${lib.path}/$name')..createSync(recursive: true);
    file.writeAsStringSync(text);
  });
  return _run(lib.path);
}

String get _dart {
  final root = Platform.environment['FLUTTER_ROOT'];
  return root == null ? 'dart' : '$root/bin/dart';
}

/// 探針在系統暫存目錄，要自己的 package_config：把 App 的相對路徑換成絕對路徑，才找得到 `hawkr_api`。
void _writeAbsolutePackageConfig(Directory dir) {
  final source = File('.dart_tool/package_config.json');
  final base = source.parent.absolute.uri;
  final config = jsonDecode(source.readAsStringSync()) as Map<String, dynamic>;
  for (final package in config['packages'] as List<dynamic>) {
    final entry = package as Map<String, dynamic>;
    entry['rootUri'] = base.resolve(entry['rootUri'] as String).toString();
  }
  File('${dir.path}/.dart_tool/package_config.json')
    ..createSync(recursive: true)
    ..writeAsStringSync(jsonEncode(config));
}

void main() {
  test('the app lib has no toLocal', () async {
    final result = await _run('lib');
    expect(result.exitCode, 0, reason: '${result.stdout}${result.stderr}');
  });

  test('the script fails on toLocal(', () async {
    final result = await _runOn({'a.dart': 'final x = d.toLocal();\n'});
    expect(result.exitCode, 1);
    expect(result.stdout, contains('toLocal('));
  });

  test('wall-clock accessors of hawkr_api do not compile in an app', () async {
    final dir = Directory.systemTemp.createTempSync('taipei_probe_');
    addTearDown(() => dir.deleteSync(recursive: true));
    final probe = File('${dir.path}/probe.dart')
      ..writeAsStringSync('''
import 'package:hawkr_api/taipei_time.dart';

void probe(DateTime d, TaipeiPickerInitial p) {
  d.taipei;
  d.taipeiWall;
  formatTaipeiDate(d);
  formatTaipeiDateTime(d);
  p.year;
  p.hour;
  p.minute;
  p._wall;
  TaipeiPickerInitial._(d);
  taipeiInstantOfPicked(2026, 1, 1, 0, 0);
}
''');
    _writeAbsolutePackageConfig(dir);
    final result = await Process.run(_dart, ['analyze', probe.path]);
    final output = '${result.stdout}${result.stderr}';
    expect(result.exitCode, isNot(0), reason: output);
    expect(output, contains("The getter 'taipei' isn't defined"));
    expect(output, contains("The getter 'taipeiWall' isn't defined"));
    expect(output, contains("The function 'formatTaipeiDate' isn't defined"));
    expect(
      output,
      contains("The function 'formatTaipeiDateTime' isn't defined"),
    );
    expect(output, isNot(contains("Target of URI")));
    expect(output, isNot(contains('Undefined class')));
    expect(output, contains("The getter 'year' isn't defined"));
    expect(output, contains("The getter 'hour' isn't defined"));
    expect(output, contains("The getter 'minute' isn't defined"));
    expect(output, contains("The getter '_wall' isn't defined"));
    expect(output, contains("taipeiInstantOfPicked' isn't defined"));
  }, timeout: const Timeout(Duration(minutes: 3)));
}
