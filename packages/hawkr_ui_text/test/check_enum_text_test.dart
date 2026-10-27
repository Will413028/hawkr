import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

import '../tool/check_enum_text.dart';

const _mk = 'hawkr.seller.v1alpha1.MeasureKind';

void main() {
  final manifestUri = Directory.current.uri.resolve(
    'tool/enum_manifest.g.json',
  );
  final manifest = loadManifest(manifestUri);

  // import 與第一行程式寫在同一行，行號不位移。
  const sellerImport =
      "import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'; "
      "import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as pb; ";

  late Directory tmp;
  setUp(() => tmp = Directory.systemTemp.createTempSync('enum-text-'));
  tearDown(() => tmp.deleteSync(recursive: true));

  String importOf(ManifestEnum e) =>
      "import '${e.dartLibrary!.replaceFirst('.pbenum.dart', '.pb.dart')}'; ";

  // 回傳 `E:<form>:<enum>.<member>@<行>`。
  List<String> scan(
    String source, {
    String imports = sellerImport,
    ConstResolver? consts,
  }) => [
    for (final h in scanSource(
      'a.dart',
      '$imports$source',
      manifest,
      consts: consts,
    ))
      '${h.rule}:${h.form.label}:${h.enumName}.${h.member}@${h.line}',
  ];

  String hit(String form, String member, int line) =>
      'E:$form:$_mk.MEASURE_KIND_$member@$line';

  const warn = 'MeasureKind.MEASURE_KIND_WARNING';
  const susp = 'MeasureKind.MEASURE_KIND_SUSPENSION';
  const term = 'MeasureKind.MEASURE_KIND_TERMINATION';
  const unspec = 'MeasureKind.MEASURE_KIND_UNSPECIFIED';

  group('if／else', () {
    test('then：== 成立；else：!= 為假', () {
      expect(
        scan('''
String f(MeasureKind k) {
  var t = '';
  if (k == $warn) return '警告';
  if (k != $susp) {} else { t = '停權'; }
  return t;
}
'''),
        [hit('if', 'WARNING', 3), hit('else', 'SUSPENSION', 4)],
      );
    });

    test('if-case 與集合裡的 if', () {
      expect(
        scan('''
String f(Object o) {
  if (o case $warn) return '警告';
  return '';
}
List<Object> w(MeasureKind k) => [if (k == $susp) Text('停權')];
List<Object> v(Object o) => [if (o case $term) Text('終止')];
List<Object> u(MeasureKind k) => [if (k != $warn) Text('x') else Text('警告')];
'''),
        [
          hit('if', 'WARNING', 2),
          hit('if', 'SUSPENSION', 5),
          hit('if', 'TERMINATION', 6),
          hit('else', 'WARNING', 7),
        ],
      );
    });

    test('補集不算：== 的 else、&& 為假、|| 為真；沒有漢字與非 proto 不報', () {
      expect(
        scan('''
String f(MeasureKind k, bool ok, Other o) {
  var t = '';
  if (k == $warn) {} else { t = '其他'; }
  if (k == $susp && ok) {} else { t = '其他'; }
  if (k != $term || ok) { t = '其他'; }
  if (k != $warn && ok) {} else { t = '其他'; }
  if (k == $susp || ok) { t = '其他'; }
  if (k == $warn) t = 'warning';
  if (o == Other.x) t = '警告';
  return t;
}
'''),
        isEmpty,
      );
    });

    test('蘊含：|| 為假、!(!=) 為真才確定相等', () {
      expect(
        scan('''
String f(MeasureKind k, bool ok) {
  var t = '';
  if (k != $warn || ok) {} else { t = '警告'; }
  if (!(k != $susp)) t = '停權';
  return t;
}
'''),
        [hit('else', 'WARNING', 3), hit('if', 'SUSPENSION', 4)],
      );
    });
  });

  group('guard（提早離開）', () {
    test('then 一定離開（return 或 throw）、沒有 else：!= 之後確定相等', () {
      expect(
        scan('''
String f(MeasureKind k) {
  if (k != $warn) return '';
  return '警告';
}
String g(MeasureKind k) {
  if (k != $susp) throw StateError('x');
  return '停權';
}
'''),
        [hit('guard', 'WARNING', 2), hit('guard', 'SUSPENSION', 6)],
      );
    });

    test('== 的 guard 是補集；then 沒有離開也不報', () {
      expect(
        scan('''
String f(MeasureKind k) {
  if (k == $warn) return '';
  return '其他';
}
String g(MeasureKind k) {
  var t = '';
  if (k != $warn) { t = 'x'; }
  return '\${t}警告';
}
'''),
        isEmpty,
      );
    });
  });

  group('三元式與短路', () {
    test('三元式：== 的真支、!= 與反向比較的假支', () {
      expect(
        scan('''
String a(MeasureKind k) => k == $warn ? '警告' : '';
String b(MeasureKind k) => $susp != k ? '' : '停權';
'''),
        [hit('ternary', 'WARNING', 1), hit('ternary', 'SUSPENSION', 2)],
      );
    });

    test('補集那一支不報', () {
      expect(
        scan('''
String a(MeasureKind k) => k == $warn ? '' : '其他';
String b(MeasureKind k) => k != $warn ? '其他' : '';
'''),
        isEmpty,
      );
    });

    test('&& 與 ||；補集（== 的 ||、!= 的 &&）不報', () {
      expect(
        scan('''
Object a(MeasureKind k) => k == $warn && f('警告');
Object b(MeasureKind k) => k != $susp || f('停權');
Object c(MeasureKind k) => k == $warn || f('其他');
Object d(MeasureKind k) => k != $susp && f('其他');
'''),
        [hit('and', 'WARNING', 1), hit('or', 'SUSPENSION', 2)],
      );
    });

    test('集合成員檢查：contains 為真才算；!contains 不算', () {
      expect(
        scan('''
String a(MeasureKind k) => {$warn, $susp}.contains(k) ? '限制' : '';
String b(MeasureKind k) => [$term].contains(k) ? '終止' : '';
String c(MeasureKind k) => !{$warn}.contains(k) ? '限制' : '';
'''),
        [
          hit('ternary', 'WARNING', 1),
          hit('ternary', 'SUSPENSION', 1),
          hit('ternary', 'TERMINATION', 2),
        ],
      );
    });
  });

  group('具名的成員集合（K5 的作用域查找）', () {
    test('const／final 的 set、list 當接收者：contains 為真才算', () {
      expect(
        scan('''
const _s = {$warn, $susp};
final _l = [$term];
final _s2 = {...[$warn]};
String f(MeasureKind k) {
  final local = {$unspec, $term};
  return [_s.contains(k) ? '限制' : '', _l.contains(k) && '終止', !_s.contains(k) ? '其他' : '', local.contains(k) ? '終止' : ''].join();
}
'''),
        [
          hit('ternary', 'WARNING', 1),
          hit('ternary', 'SUSPENSION', 1),
          hit('and', 'TERMINATION', 2),
          hit('ternary', 'TERMINATION', 5),
        ],
      );
    });

    test('guard 不用成員檢查（選項驗證）；if 用', () {
      expect(
        scan('''
const _options = {$warn, $susp};
String f(MeasureKind k, String n) {
  if (!_options.contains(k)) return '請選擇種類。';
  if (n.isEmpty) return '請填寫名稱。';
  return '';
}
String g(MeasureKind k) {
  if (_options.contains(k)) { return '限制'; }
  return '';
}
'''),
        [hit('if', 'WARNING', 1), hit('if', 'SUSPENSION', 1)],
      );
    });

    test('參數或 var 遮住的名稱、內容不是 enum 成員的集合不算', () {
      expect(
        scan('''
const _s = {$warn};
const _t = {'a', 'b'};
String f(MeasureKind k, Set<MeasureKind> _s) {
  var l = [$susp];
  return [_s.contains(k) ? '限制' : '', _t.contains('a') ? '其他' : '', l.contains(k) ? '停權' : ''].join();
}
'''),
        isEmpty,
      );
    });
  });

  group('guard 在 switch case 裡', () {
    test('case 的敘述也是同一層：!= 之後確定相等；== 的 guard 是補集', () {
      expect(
        scan('''
String f(int n, MeasureKind k) {
  switch (n) {
    case 1:
      if (k != $warn) return '';
      return '警告';
    case 2:
      if (k == $susp) return '';
      return '其他';
  }
  return '';
}
'''),
        [hit('guard', 'WARNING', 4)],
      );
    });
  });

  group('switch 與 case', () {
    test('switch 運算式：case E.M 的分支有漢字要報錯；import 前綴、or 樣式都認得', () {
      expect(
        scan('''
String f(MeasureKind k) => switch (k) {
  $warn => '警告',
  _ => '',
};
String g(pb.MeasureKind k) => switch (k) {
  pb.$warn || $susp => '限制',
  _ => '',
};
'''),
        [
          hit('case', 'WARNING', 2),
          hit('case', 'WARNING', 6),
          hit('case', 'SUSPENSION', 6),
        ],
      );
    });

    test('switch 敘述：return、賦值、throw、widget；fall-through 的 case 一起算', () {
      expect(
        scan('''
String f(MeasureKind k) {
  var label = '';
  switch (k) {
    case $warn:
    case $susp:
      return '限制';
    case $term:
      label = '終止合作';
  }
  return label;
}
Object g(MeasureKind k) {
  switch (k) {
    case $warn:
      throw StateError('限制');
    case $susp:
      return Text('停權');
    case $term:
      break;
  }
  return 0;
}
'''),
        [
          hit('case', 'WARNING', 4),
          hit('case', 'SUSPENSION', 5),
          hit('case', 'TERMINATION', 7),
          hit('case', 'WARNING', 14),
          hit('case', 'SUSPENSION', 16),
        ],
      );
    });

    test('樣式：object、relational ==、when', () {
      expect(
        scan('''
String f(Measure m, MeasureKind k) => switch (m) {
  Measure(kind: $warn) => '警告',
  == $susp => '停權',
  _ when k == $term => '終止',
  _ => '',
};
'''),
        [
          hit('case', 'WARNING', 2),
          hit('case', 'SUSPENSION', 3),
          hit('case', 'TERMINATION', 4),
        ],
      );
    });

    test('default 與 _ 是補集不算；沒有漢字不報；fall-through 進 default 不算', () {
      expect(
        scan('''
String f(MeasureKind k) {
  switch (k) {
    case $warn:
      return '';
    default:
      return '其他';
  }
}
String g(MeasureKind k) => switch (k) { $warn => 'warning', _ => '其他' };
String h(MeasureKind k) {
  switch (k) {
    case $warn:
    default:
      return '其他';
  }
}
String i(MeasureKind k, Other o) => switch (o) { Other.x => '警告', _ => '' };
'''),
        isEmpty,
      );
    });
  });

  group('key 與 pair', () {
    test('map literal 的 key 是 E.M、值（含 record）有漢字要報錯', () {
      expect(
        scan('''
const labels = {
  $warn: '警告',
  $susp: (label: '停權', tone: 1),
  $term: 'x',
};
'''),
        [hit('key', 'WARNING', 2), hit('key', 'SUSPENSION', 3)],
      );
    });

    test('配對：record、具名引數、MapEntry、list、字串 key 的 map', () {
      expect(
        scan('''
final a = (value: $warn, label: '警告');
final b = Option(value: $susp, label: '停權');
final c = MapEntry($term, '終止');
final d = [$warn, '警告'];
final e = {'value': $susp, 'label': '停權'};
'''),
        [
          hit('pair', 'WARNING', 1),
          hit('pair', 'SUSPENSION', 2),
          hit('pair', 'TERMINATION', 3),
          hit('pair', 'WARNING', 4),
          hit('pair', 'SUSPENSION', 5),
        ],
      );
    });

    test('配對不進入巢狀函式', () {
      expect(
        scan('''
final a = f($warn, () => '警告');
final b = Option(value: $warn, onTap: () { final s = '警告'; });
'''),
        isEmpty,
      );
    });

    test('值號 0（UNSPECIFIED）不是選擇子', () {
      expect(
        scan('''
final a = f($unspec, '請選擇');
final d = f(pb.$unspec, '請選擇');
String b(MeasureKind k) => k == $unspec ? '請選擇' : '';
const c = {$unspec: '請選擇'};
'''),
        isEmpty,
      );
    });
  });

  group('別名與常數', () {
    test('final 別名是比較或 cond ? x : null（含 != null）；var 不代入', () {
      expect(
        scan('''
Object f(MeasureKind k, Object o) {
  final isW = k == $warn;
  final p = k == $susp ? o : null;
  final q = isW;
  return [isW ? '警告' : '', p != null && '停權', q && '警告'];
}
Object g(MeasureKind k) {
  var t = k == $warn;
  return t ? '警告' : '';
}
'''),
        [
          hit('ternary', 'WARNING', 2),
          hit('and', 'SUSPENSION', 3),
          hit('and', 'WARNING', 2),
        ],
      );
    });

    test('參數遮住同名的別名；非條件式的別名不當成有值', () {
      expect(
        scan('''
Object f(MeasureKind k) {
  final t = k == $warn;
  final b = k == $susp;
  return [((bool t) => t ? '警告' : '')(true), b != null && '其他'];
}
'''),
        isEmpty,
      );
    });

    test('頂層與函式內的 final／const 常數算有文字；不含漢字、var 不算', () {
      expect(
        scan('''
const t = '警告';
String f(MeasureKind k) {
  final u = '停權';
  return [k == $warn ? t : '', k == $susp ? u : ''].join();
}
const en = 'warning';
String g(MeasureKind k) {
  var v = '終止';
  return [k == $warn ? en : '', k == $susp ? v : ''].join();
}
'''),
        [hit('ternary', 'WARNING', 4), hit('ternary', 'SUSPENSION', 4)],
      );
    });

    test('同檔的物件常數：record 的欄位、map 的字串 key，值含漢字才算', () {
      expect(
        scan('''
const text = (term: '合作已終止', en: 'ended');
const labels = {'a': '警告', 'b': 'warning'};
List<String> f(MeasureKind k) => [
  k == $warn ? text.term : '',
  k == $susp ? labels['a']! : '',
  k == $term ? text.en : '',
  k == $warn ? labels['b']! : '',
];
'''),
        [hit('ternary', 'WARNING', 4), hit('ternary', 'SUSPENSION', 5)],
      );
    });

    test('跨檔匯入的常數（resolver 給的名稱；show／hide／前綴）', () {
      Set<String>? consts(String uri, String from) =>
          uri == 'text.dart' ? {'t', 'u'} : null;
      expect(
        scan(
          '''
String f(MeasureKind k) => k == $warn ? t : '';
String g(MeasureKind k) => k == $susp ? m : '';
String h(MeasureKind k) => k == $term ? x.u : '';
String i(MeasureKind k) => k == $susp ? u : '';
String j(MeasureKind k) => k == $warn ? x.t : '';
''',
          imports:
              "$sellerImport import 'text.dart' show t; import 'other.dart'; import 'text.dart' as x hide t; ",
          consts: consts,
        ),
        [hit('ternary', 'WARNING', 1), hit('ternary', 'TERMINATION', 3)],
      );
      // 沒有 resolver：只看同檔，匯入的不算。
      expect(
        scan(
          "String f(MeasureKind k) => k == $warn ? t : '';",
          imports: "$sellerImport import 'text.dart'; ",
        ),
        isEmpty,
      );
    });

    test('scanDir 解析相對路徑與本 App 的 package 路徑，只追樹內、含漢字的頂層常數', () {
      final lib = Directory('${tmp.path}/app/lib')..createSync(recursive: true);
      File('${tmp.path}/app/pubspec.yaml').writeAsStringSync('name: my_app\n');
      File('${lib.path}/text.dart').writeAsStringSync(
        "const t = '警告';\nconst en = 'warning';\nconst _hidden = '私有';\n",
      );
      File('${tmp.path}/outside.dart').writeAsStringSync("const t = '樹外';\n");
      File('${lib.path}/use.dart').writeAsStringSync(
        "$sellerImport import 'text.dart'; import 'package:my_app/text.dart' as p; import '../../outside.dart' as o;\n"
        "String a(MeasureKind k) => k == $warn ? t : '';\n"
        "String b(MeasureKind k) => k == $susp ? en : '';\n"
        "String c(MeasureKind k) => k == $term ? p.t : '';\n"
        "String d(MeasureKind k) => k == $warn ? o.t : '';\n",
      );
      final hits = [
        for (final h in scanDir(lib, manifest))
          if (h.file == 'use.dart') '${h.form.label}:${h.member}@${h.line}',
      ];
      expect(hits, [
        'ternary:MEASURE_KIND_WARNING@2',
        'ternary:MEASURE_KIND_TERMINATION@4',
      ]);
    });
  });

  group('manifest 與回報', () {
    test('單端的 enum：任何形式都不失敗；只有對應表（case、key、pair）進 report', () {
      final single = manifest.firstWhere(
        (e) => e.dartClass != null && !e.inCatalog && !e.serverOwned,
      );
      final owned = manifest.firstWhere(
        (e) => e.dartClass != null && e.serverOwned,
      );
      List<Hit> hits(ManifestEnum e, String body) =>
          scanSource('a.dart', '${importOf(e)}$body', manifest);
      final c = single.dartClass;
      final ternary = hits(single, "String f($c k) => k == $c.X ? '文字' : '';");
      expect(ternary, hasLength(1));
      expect(forbidden(ternary, manifest), isEmpty);
      expect(buildReport(ternary)['enums'], isEmpty);
      for (final body in [
        "const m = {$c.X: '文字'};",
        "final m = ($c.X, '文字');",
        "String f($c k) => switch (k) { $c.X => '文字', _ => '' };",
      ]) {
        final h = hits(single, body);
        expect(forbidden(h, manifest), isEmpty);
        expect(buildReport(h)['enums'], {
          single.name: ['a.dart:1'],
        });
      }
      // 目錄與伺服器擁有的 enum：條件形式也擋。
      final ownedHits = hits(
        owned,
        "String f(${owned.dartClass} k) => k == ${owned.dartClass}.X ? '文字' : '';",
      );
      expect(forbidden(ownedHits, manifest), hasLength(1));
      expect(
        forbidden(
          scanSource(
            'a.dart',
            "${sellerImport}String f(MeasureKind k) => k == $warn ? '警告' : '';",
            manifest,
          ),
          manifest,
        ),
        hasLength(1),
      );
    });

    test('只算從 api 生成檔引入的 enum：同名的非 proto 類別（CodeMeta）不報', () {
      final body = "const m = {$warn: '警告'};";
      // 同名類別來自 App 自己的 CodeMeta。
      expect(
        scan(
          body,
          imports: "import 'package:app/core/codemeta/code_meta.dart'; ",
        ),
        isEmpty,
      );
      // 沒有任何 import：不報。
      expect(scan(body, imports: ''), isEmpty);
      // 同一段程式從 api 的 .pb.dart 或 .pbenum.dart 引入就報。
      const pb = 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller';
      for (final lib in ['$pb.pb.dart', '$pb.pbenum.dart']) {
        expect(scan(body, imports: "import '$lib'; "), [
          hit('key', 'WARNING', 1),
        ]);
      }
      // show 沒列到或 hide 掉這個類別：不是這個 enum。
      expect(scan(body, imports: "import '$pb.pb.dart' show Other; "), isEmpty);
      expect(
        scan(body, imports: "import '$pb.pb.dart' hide MeasureKind; "),
        isEmpty,
      );
    });

    test('Dart 版 manifest 不含 hawkr.admin.*', () {
      expect(manifest.where((e) => e.name.startsWith('hawkr.admin.')), isEmpty);
      expect(manifest.where((e) => e.inCatalog), hasLength(16));
    });
  });

  group('命令列', () {
    test('掃目錄：略過生成檔、gen、測試；寫出 report；有禁止的表就非零結束', () {
      const bad =
          "$sellerImport const m = {MeasureKind.MEASURE_KIND_WARNING: '警告'};\n";
      for (final path in [
        'lib/labels.dart',
        'lib/labels.g.dart',
        'lib/labels.freezed.dart',
        'lib/gen/x.dart',
        'lib/core/codemeta/x.dart',
        'lib/labels_test.dart',
      ]) {
        File('${tmp.path}/$path')
          ..createSync(recursive: true)
          ..writeAsStringSync(bad);
      }
      final report = '${tmp.path}/report.json';
      final code = run([
        '--report',
        report,
        '${tmp.path}/lib',
      ], manifestUri: manifestUri);
      expect(code, 1);
      expect(jsonDecode(File(report).readAsStringSync()), {
        'enums': {
          _mk: ['labels.dart:1'],
        },
      });
    });

    test('乾淨的目錄回 0，缺參數回 2', () {
      File('${tmp.path}/ok.dart').writeAsStringSync("const x = '警告';\n");
      expect(run([tmp.path], manifestUri: manifestUri), 0);
      expect(run([], manifestUri: manifestUri), 2);
    });
  });
}
