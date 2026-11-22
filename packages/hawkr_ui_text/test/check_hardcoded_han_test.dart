import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

import '../tool/check_hardcoded_han.dart';

void main() {
  late Directory tmp;
  late Directory lib;
  late File baseline;

  setUp(() {
    tmp = Directory.systemTemp.createTempSync('han-ratchet-');
    lib = Directory('${tmp.path}/app/lib')..createSync(recursive: true);
    Directory('${tmp.path}/app/tool').createSync(recursive: true);
    baseline = File('${tmp.path}/app/tool/han_baseline.json');
  });
  tearDown(() => tmp.deleteSync(recursive: true));

  void write(String rel, String source) {
    File('${lib.path}/$rel')
      ..createSync(recursive: true)
      ..writeAsStringSync(source);
  }

  int runWith(List<String> extra) =>
      run(['--baseline', baseline.path, ...extra, lib.path]);

  group('scanHanLiterals', () {
    test('只算含漢字的字串字面值；註解、識別字與純 ASCII 字串不算', () {
      final found = scanHanLiterals('a.dart', '''
// 註解不算
const a = '你好';
const b = 'hello';
const c = "再見";
const d = 'x' '中文' 'y';
String e(int n) => '共 \$n 件，\${n + 1} 筆';
''');
      expect(found, ['你好', '再見', 'x中文y', '共 {} 件，{} 筆']);
    });

    test('插值運算式裡的字面值另算一筆', () {
      expect(
        scanHanLiterals(
          'a.dart',
          r"String f(int n) => 'a${n > 0 ? '正' : '負'}';",
        ),
        ['正', '負'],
      );
    });

    test('三引號與 raw 字串也算', () {
      expect(
        scanHanLiterals('a.dart', "const a = r'路徑';\nconst b = '''多行\n文字''';"),
        ['路徑', '多行\n文字'],
      );
    });
  });

  group('scanDirHan', () {
    test('路徑相對 lib 的上一層；排除 .g.dart、.freezed.dart 與 l10n/gen', () {
      write('a.dart', "const a = '中';");
      write('sub/b.dart', "const a = '中'; const b = '中';");
      write('x.g.dart', "const a = '生成';");
      write('y.freezed.dart', "const a = '生成';");
      write('l10n/gen/app_localizations_zh.dart', "const a = '生成';");
      write('l10n/app_localizations_x.dart', "const a = '手寫';");
      expect(scanDirHan(lib), {
        'lib/a.dart': {'中': 1},
        'lib/sub/b.dart': {'中': 2},
        'lib/l10n/app_localizations_x.dart': {'手寫': 1},
      });
    });
  });

  group('命令列', () {
    test('--init 建立 baseline；現況通過', () {
      write('a.dart', "const a = '中';");
      expect(runWith(['--init']), 0);
      expect(runWith([]), 0);
      expect(jsonDecode(baseline.readAsStringSync()), {
        'version': 1,
        'entries': {
          'lib/a.dart': {'中': 1},
        },
      });
    });

    test('同檔次數增加失敗（刪一句加一句不同的也失敗）', () {
      write('a.dart', "const a = '中'; const b = '舊';");
      runWith(['--init']);
      write('a.dart', "const a = '中'; const b = '中';");
      expect(runWith([]), 1);
      write('a.dart', "const a = '中'; const b = '新';");
      expect(runWith([]), 1);
    });

    test('同檔只增加不減少失敗', () {
      write('a.dart', "const a = '中';");
      runWith(['--init']);
      write('a.dart', "const a = '中'; const b = '中';");
      expect(runWith([]), 1);
    });

    test('新檔含漢字失敗；新檔沒有漢字通過', () {
      write('a.dart', "const a = '中';");
      runWith(['--init']);
      write('b.dart', "const b = 'ascii';");
      expect(runWith([]), 0);
      write('b.dart', "const b = '中';");
      expect(runWith([]), 1);
    });

    test('次數減少要求 --update；--update 後通過', () {
      write('a.dart', "const a = '中'; const b = '中';");
      runWith(['--init']);
      write('a.dart', "const a = '中';");
      expect(runWith([]), 1);
      expect(runWith(['--update']), 0);
      expect(runWith([]), 0);
      expect((jsonDecode(baseline.readAsStringSync()) as Map)['entries'], {
        'lib/a.dart': {'中': 1},
      });
    });

    test('--update 拒絕全域總次數增加，且不寫檔', () {
      write('a.dart', "const a = '中';");
      runWith(['--init']);
      final before = baseline.readAsStringSync();
      write('a.dart', "const a = '中'; const b = '中';");
      expect(runWith(['--update']), 1);
      write('a.dart', "const a = '中'; const c = '新';");
      expect(runWith(['--update']), 1);
      expect(baseline.readAsStringSync(), before);
    });

    test('改檔名：檢查失敗，--update 允許（全域總次數不變）', () {
      write('a.dart', "const a = '中';");
      runWith(['--init']);
      File('${lib.path}/a.dart').renameSync('${lib.path}/b.dart');
      expect(runWith([]), 1);
      expect(runWith(['--update']), 0);
      expect(runWith([]), 0);
    });

    test('把句子搬到別的檔（總次數不變）可 --update，搬成兩份不行', () {
      write('a.dart', "const a = '中';");
      write('b.dart', "const b = 'x';");
      runWith(['--init']);
      write('a.dart', 'const a = 1;');
      write('b.dart', "const b = '中';");
      expect(runWith(['--update']), 0);
      write('c.dart', "const c = '中';");
      expect(runWith(['--update']), 1);
    });

    test('用法錯誤與缺 baseline 回 2', () {
      expect(run([]), 2);
      expect(run(['--baseline', baseline.path]), 2);
      expect(runWith([]), 2);
    });

    test('--init 不覆蓋既有 baseline', () {
      write('a.dart', "const a = '中';");
      expect(runWith(['--init']), 0);
      expect(runWith(['--init']), 1);
    });
  });

  group('--check-baseline-against', () {
    late File old;
    setUp(() => old = File('${tmp.path}/old.json'));

    void put(File f, Map<String, Map<String, int>> entries) =>
        f.writeAsStringSync(jsonEncode({'version': 1, 'entries': entries}));

    int check() => run([
      '--baseline',
      baseline.path,
      '--check-baseline-against',
      old.path,
    ]);

    test('同樣或減少、檔案間搬移通過', () {
      put(old, {
        'lib/a.dart': {'你好': 2},
      });
      put(baseline, {
        'lib/a.dart': {'你好': 1},
        'lib/b.dart': {'你好': 1},
      });
      expect(check(), 0);
    });

    test('任一字面值全域總次數變大失敗，新字面值也算', () {
      put(old, {
        'lib/a.dart': {'你好': 1},
      });
      put(baseline, {
        'lib/a.dart': {'你好': 2},
      });
      expect(check(), 1);
      put(baseline, {
        'lib/a.dart': {'你好': 1, '再見': 1},
      });
      expect(check(), 1);
    });

    test('缺檔與未知參數回 2', () {
      put(baseline, {});
      expect(check(), 2);
      expect(run(['--baseline', baseline.path, '--bogus', lib.path]), 2);
    });
  });
}
