import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// CI 的 `scripts/check-taipei-display.py`：lib/ 不准 `toLocal(`，`formatTaipei*` 只在
/// `shared/time/taipei_display.dart` 呼叫。這裡驗腳本本身會擋（造一個違規的 lib），並驗目前的 lib 是乾淨的。
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

void main() {
  test(
    'the app lib has no toLocal and no formatTaipei outside the helper',
    () async {
      final result = await _run('lib');
      expect(result.exitCode, 0, reason: '${result.stdout}${result.stderr}');
    },
  );

  test('the script fails on toLocal(', () async {
    final result = await _runOn({'a.dart': 'final x = d.toLocal();\n'});
    expect(result.exitCode, 1);
    expect(result.stdout, contains('toLocal('));
  });

  test('the script fails on formatTaipei calls outside the helper', () async {
    final result = await _runOn({'a.dart': 'final x = formatTaipeiDate(d);\n'});
    expect(result.exitCode, 1);
    expect(result.stdout, contains('formatTaipei'));
  });

  test('the script accepts formatTaipei calls inside the helper', () async {
    final result = await _runOn({
      'shared/time/taipei_display.dart': 'final x = formatTaipeiDate(d);\n',
    });
    expect(result.exitCode, 0, reason: '${result.stdout}');
  });
}
