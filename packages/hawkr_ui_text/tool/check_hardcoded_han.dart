/// 硬寫中文的 ratchet（多語系 ADR 2026-10-17-multi-locale-architecture D4 與 2026-10-22 修訂；計畫 I-CI 的 App 部分）。
///
/// 用 `package:analyzer` 的 `parseString`（只做語法分析，不用 regex 掃原始碼）：掃 `lib/` 底下每個 Dart 檔的字串字面值，
/// 含漢字的記成一筆「檔案 → 字面值內容 → 次數」，不記行號（改行不影響）。字面值內容是字串的文字本身；字串插值把每個
/// `$x`／`${…}` 寫成 `{}`，相鄰字串（`'a' 'b'`）合成一筆。註解不算。
///
/// - baseline（`tool/han_baseline.json`）只能減少：任何 （檔案，字面值） 的次數超過 baseline 就失敗；baseline 沒有的檔案
///   （新檔）一律視為 0，所以新檔含任何漢字字面值都失敗。新文字放 ARB（`lib/l10n`）。
/// - 次數減少（含刪檔、改檔名後舊路徑消失）時失敗，要求跑 `--update` 把 baseline 收緊：不收緊，之後同一檔加回去的
///   同一句會被舊的額度吞掉，分不出新舊。
/// - `--update` 只在每個字面值的「全域總次數」都沒有增加時寫入（容許改檔名與搬檔：檔案變了、總次數不變）；有任何字面值的
///   全域總次數增加就拒絕（exit 1）、不寫檔。baseline 因此只會變少，初次建立（`--init`）是唯一允許從無到有的路徑。
/// - 範圍是傳入的目錄（`buyer-app/lib`、`seller-app/lib`），排除生成檔：`*.g.dart`、`*.freezed.dart`、`l10n/gen/`。
///   路徑記成相對該目錄的上一層（`lib/features/x.dart`）。
///
/// 用法（在 packages/hawkr_ui_text）：
///   `dart run tool/check_hardcoded_han.dart --baseline <file> <dir>`
///   `dart run tool/check_hardcoded_han.dart --baseline <file> --update <dir>`
///   `dart run tool/check_hardcoded_han.dart --baseline <file> --init <dir>`（baseline 檔不存在時建立）
/// 結束碼：0 通過、1 違規（超過、新檔、需要 `--update`、`--update` 被拒）、2 用法錯誤。
library;

import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/utilities.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';

final _han = RegExp('[㐀-䶿一-鿿豈-﫿]');

/// 檔案 → 字面值內容 → 次數。
typedef HanCounts = Map<String, Map<String, int>>;

/// 一檔裡的漢字字串字面值內容（每出現一次一筆）。
List<String> scanHanLiterals(String path, String source) {
  final result = parseString(
    content: source,
    path: path,
    throwIfDiagnostics: false,
  );
  final visitor = _LiteralVisitor();
  result.unit.accept(visitor);
  return visitor.found;
}

/// 字面值的內容：插值運算式換成 `{}`。
String _contentOf(StringLiteral node) {
  if (node is SimpleStringLiteral) return node.value;
  if (node is AdjacentStrings) {
    return node.strings.map(_contentOf).join();
  }
  if (node is StringInterpolation) {
    final b = StringBuffer();
    for (final e in node.elements) {
      if (e is InterpolationString) {
        b.write(e.value);
      } else {
        b.write('{}');
      }
    }
    return b.toString();
  }
  return '';
}

class _LiteralVisitor extends RecursiveAstVisitor<void> {
  final found = <String>[];

  void _record(StringLiteral node) {
    final content = _contentOf(node);
    if (_han.hasMatch(content)) found.add(content);
  }

  @override
  void visitSimpleStringLiteral(SimpleStringLiteral node) => _record(node);

  @override
  void visitAdjacentStrings(AdjacentStrings node) {
    _record(node);
    // 插值裡的運算式可能還有字面值；字串片段本身已經算進整筆，不重複算。
    for (final s in node.strings) {
      if (s is StringInterpolation) _visitInterpolated(s);
    }
  }

  @override
  void visitStringInterpolation(StringInterpolation node) {
    _record(node);
    _visitInterpolated(node);
  }

  void _visitInterpolated(StringInterpolation node) {
    for (final e in node.elements) {
      if (e is InterpolationExpression) e.expression.accept(this);
    }
  }
}

bool _isGenerated(String relPath) =>
    relPath.endsWith('.g.dart') ||
    relPath.endsWith('.freezed.dart') ||
    relPath.contains('/l10n/gen/');

/// [dir]（例如 `buyer-app/lib`）底下的全部計數；路徑相對 [dir] 的上一層。
HanCounts scanDirHan(Directory dir) {
  final base = dir.parent.absolute.path;
  final counts = <String, Map<String, int>>{};
  final files = [
    for (final e in dir.listSync(recursive: true, followLinks: false))
      if (e is File && e.path.endsWith('.dart')) e,
  ]..sort((a, b) => a.path.compareTo(b.path));
  for (final f in files) {
    final rel = f.absolute.path.substring(base.length + 1);
    if (_isGenerated(rel)) continue;
    final literals = scanHanLiterals(rel, f.readAsStringSync());
    if (literals.isEmpty) continue;
    final m = counts[rel] = <String, int>{};
    for (final l in literals) {
      m[l] = (m[l] ?? 0) + 1;
    }
  }
  return counts;
}

/// 檢查的結果：[violations] 讓檢查失敗；[stale] 是 baseline 比現況多（要 `--update`）。
class Comparison {
  Comparison(this.violations, this.stale);

  final List<String> violations;
  final List<String> stale;

  bool get ok => violations.isEmpty && stale.isEmpty;
}

Comparison compareCounts(HanCounts baseline, HanCounts current) {
  final violations = <String>[];
  final stale = <String>[];
  for (final file in current.keys.toList()..sort()) {
    final base = baseline[file];
    final isNew = base == null;
    for (final lit in current[file]!.keys.toList()..sort()) {
      final cur = current[file]![lit]!;
      final allowed = base?[lit] ?? 0;
      if (cur > allowed) {
        violations.add(
          isNew
              ? '$file: 新檔含漢字字面值 ${jsonEncode(lit)} ×$cur（新檔必須 0；文字放 ARB）'
              : '$file: ${jsonEncode(lit)} 次數 $cur 超過 baseline $allowed',
        );
      }
    }
  }
  for (final file in baseline.keys.toList()..sort()) {
    for (final lit in baseline[file]!.keys.toList()..sort()) {
      final cur = current[file]?[lit] ?? 0;
      if (cur < baseline[file]![lit]!) {
        stale.add(
          '$file: ${jsonEncode(lit)} 次數 $cur 少於 baseline ${baseline[file]![lit]}',
        );
      }
    }
  }
  return Comparison(violations, stale);
}

Map<String, int> _globalTotals(HanCounts counts) {
  final totals = <String, int>{};
  for (final m in counts.values) {
    m.forEach((lit, n) => totals[lit] = (totals[lit] ?? 0) + n);
  }
  return totals;
}

/// `--update` 的守門：每個字面值的全域總次數都沒有增加。回傳增加的字面值（空＝允許）。
List<String> globalIncreases(HanCounts baseline, HanCounts current) {
  final before = _globalTotals(baseline);
  final after = _globalTotals(current);
  return [
    for (final lit in after.keys.toList()..sort())
      if (after[lit]! > (before[lit] ?? 0))
        '${jsonEncode(lit)}：全域 ${before[lit] ?? 0} → ${after[lit]}',
  ];
}

HanCounts loadBaseline(File file) {
  final json = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
  final entries = json['entries']! as Map<String, Object?>;
  return {
    for (final e in entries.entries)
      e.key: {
        for (final l in (e.value! as Map<String, Object?>).entries)
          l.key: l.value! as int,
      },
  };
}

String encodeBaseline(HanCounts counts) {
  final files = counts.keys.toList()..sort();
  final entries = <String, Object?>{
    for (final f in files)
      f: {for (final l in counts[f]!.keys.toList()..sort()) l: counts[f]![l]},
  };
  return '${const JsonEncoder.withIndent('  ').convert({'version': 1, 'entries': entries})}\n';
}

const _usage =
    '用法：check_hardcoded_han --baseline <file> [--update | --init] <dir>';

int run(List<String> argv) {
  final args = [...argv];
  String? baselinePath;
  var update = false;
  var init = false;
  final dirs = <String>[];
  for (var i = 0; i < args.length; i++) {
    switch (args[i]) {
      case '--baseline':
        if (i + 1 >= args.length) {
          stderr.writeln(_usage);
          return 2;
        }
        baselinePath = args[++i];
      case '--update':
        update = true;
      case '--init':
        init = true;
      default:
        dirs.add(args[i]);
    }
  }
  if (baselinePath == null || dirs.length != 1 || (update && init)) {
    stderr.writeln(_usage);
    return 2;
  }
  final dir = Directory(dirs.single);
  if (!dir.existsSync()) {
    stderr.writeln('check_hardcoded_han：找不到目錄 ${dirs.single}');
    return 2;
  }
  final current = scanDirHan(dir);
  final baselineFile = File(baselinePath);

  if (init) {
    if (baselineFile.existsSync()) {
      stderr.writeln('check_hardcoded_han：$baselinePath 已存在，--init 只用在建立');
      return 1;
    }
    baselineFile.writeAsStringSync(encodeBaseline(current));
    stdout.writeln('check_hardcoded_han：已建立 $baselinePath');
    return 0;
  }
  if (!baselineFile.existsSync()) {
    stderr.writeln('check_hardcoded_han：找不到 baseline $baselinePath');
    return 2;
  }
  final baseline = loadBaseline(baselineFile);

  if (update) {
    final increased = globalIncreases(baseline, current);
    if (increased.isNotEmpty) {
      stderr.writeln('check_hardcoded_han：--update 拒絕，baseline 只能減少：');
      for (final l in increased) {
        stderr.writeln('  $l');
      }
      return 1;
    }
    baselineFile.writeAsStringSync(encodeBaseline(current));
    stdout.writeln('check_hardcoded_han：已更新 $baselinePath');
    return 0;
  }

  final cmp = compareCounts(baseline, current);
  for (final v in cmp.violations) {
    stderr.writeln(v);
  }
  if (cmp.violations.isNotEmpty) {
    stderr.writeln(
      '\ncheck_hardcoded_han：${cmp.violations.length} 筆超過 baseline。新文字放 ARB（lib/l10n），不要硬寫中文。',
    );
    return 1;
  }
  if (cmp.stale.isNotEmpty) {
    for (final s in cmp.stale) {
      stderr.writeln(s);
    }
    stderr.writeln(
      '\ncheck_hardcoded_han：${cmp.stale.length} 筆比 baseline 少，請執行 --update 收緊 baseline。',
    );
    return 1;
  }
  stdout.writeln('check_hardcoded_han：通過');
  return 0;
}

void main(List<String> args) => exit(run(args));
