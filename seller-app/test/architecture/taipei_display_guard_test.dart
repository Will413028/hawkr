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
    final dir = Directory('test/architecture/.taipei_probe')
      ..createSync(recursive: true);
    addTearDown(() => dir.deleteSync(recursive: true));
    final probe = File('${dir.path}/probe.dart')
      ..writeAsStringSync('''
import 'package:hawkr_api/taipei_time.dart';

void probe(DateTime d) {
  d.taipei;
  d.taipeiWall;
  formatTaipeiDate(d);
  formatTaipeiDateTime(d);
}
''');
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
  }, timeout: const Timeout(Duration(minutes: 3)));
}
