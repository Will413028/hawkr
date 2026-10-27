/// 守門：目錄 enum 的使用者面文字只有一個來源（設計 enum-text-guard-design.md §1；L′ §5.2）。
///
/// 用 `package:analyzer` 的 `parseString`（只做語法分析），名稱比對靠 `tool/enum_manifest.g.json`。
///
/// 一條規則 E：使用者面文字出現在「由受管 enum 的某個成員確定成立」的區域裡，就是命中。
/// 受管 enum＝manifest 的 `inCatalog || serverOwned`（共用文案目錄的 enum 與伺服器擁有的 enum），
/// 受管 enum 的任何命中都失敗。其他 enum（單端）的條件形式不失敗、也不回報；對照表形式寫進 `--report`（見下）。
/// 沒有範圍參數、沒有例外清單。
///
/// - 選擇子：`E.M`／`p.E.M`，E 是 manifest 的 Dart 類別名，且該檔從 `package:hawkr_api` 的
///   `.pb.dart`／`.pbenum.dart` 引入它（同名的非 proto 類別不算）。值號 0 的成員（名稱以 UNSPECIFIED 結尾）
///   不是選擇子：目錄不給值號 0 文字。
/// - 「選出」只算正向：進入區域的每一條路徑都已確定「x 等於成員 M」才算。補集（三元式的另一支、
///   `if (x == M) return;` 之後、`default:`、`_ =>`）不是任何成員的說法，不算。`x == M` 為真、`x != M` 為假、
///   `{E.A, …}.contains(x)` 為真（接收者可以是字面值，或指到這種初始值的 `final`／`const`）、`a && b` 為真、
///   `a || b` 為假才確定相等；`!` 反轉。成員檢查不用在 `guard`：`if (!options.contains(x)) return …;` 之後的敘述
///   是集合裡任何一個值共用的（選項驗證），不是某個值的說法。
/// - 區域與 form：`if`／`else`（含 if-case 與集合裡的 `if`）、`ternary`、`and`／`or`、`guard`（沒有 else、then 一定離開時，
///   同區塊之後的敘述）、`case`（switch 敘述與運算式；樣式裡任何位置的 `E.M` 或 `== E.M`，加 `when`）、`key`（map 的 key
///   對應的值）、`pair`（同一個 record、引數列、list、字串 key 的 map 裡與 `E.M` 並列的其他成員；不進入巢狀函式）。
///   條件裡的識別字若是用到之前宣告的 `final`／`const`，初始值是比較或 `cond ? x : null`，就代入（別名，一層一層）。
/// - 產生文字：區域裡有含漢字的字串，或引用作用域可見的 `final`／`const`，其初始值是含漢字的字串：同檔的（頂層或函式內）
///   或 import 自同一個 client 目錄樹內另一個檔、該檔頂層的；同檔 record 或 map 常數的成員（`text.key`、`text['key']`）
///   值含漢字也算。不分是 return、賦值、widget 參數還是 `throw`。參數、`var` 會遮住同名的常數。
///
/// `--report <file>` 寫出此 client 有「對應表」（`case`、`key`、`pair`）的 enum 全名，給跨端檢查交集
/// （scripts/check-enum-text-cross.sh）；條件形式（if、三元式…）是行為上的分流，只用在失敗判斷，不進 report。
///
/// 不涵蓋（只做語法分析、沒有型別與資料流的極限）：包在函式、getter 裡的條件；`var` 與沒有 `final` 的變數；先把 enum 對應成
/// 本地的值再以本地值選文字；以 enum 值當索引的 list 或 map；class 靜態欄位、跨檔的物件常數與 re-export 的常數；
/// 第二個 client 對只有一個 client 用的 enum，以條件形式寫出值的說法（跨端檢查只比對對照表，看不到條件形式）。
/// 這些由 code review 把關；會發佈 en 的介面另有字面值 ratchet（I-CI）。
/// 觸發：dot shorthand（`.M`）要 Dart 3.10 才能寫，任一 Dart 專案的 SDK 下限提到 3.10 以上時，以成員名
/// （buf 強制 `ENUM_VALUE_PREFIX`，成員名對得回唯一的 enum）支援；「產生文字」要加進 ARB 查詢（計畫 I-app-0），
/// 與 I-app-0 同一刀。
///
/// 用法（在 packages/hawkr_ui_text）：`dart run tool/check_enum_text.dart [--report <file>] <dir>...`
library;

import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/utilities.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/token.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/source/line_info.dart';

final _han = RegExp('[㐀-䶿一-鿿豈-﫿]');
final _unspecified = RegExp(r'UNSPECIFIED$');

const _excludedDirs = {
  '.dart_tool',
  '.git',
  'build',
  'codemeta',
  'gen',
  'test',
  'integration_test',
};

/// manifest 裡的一個 proto enum。
class ManifestEnum {
  ManifestEnum({
    required this.name,
    required this.dartClass,
    required this.dartLibrary,
    required this.inCatalog,
    required this.serverOwned,
  });

  factory ManifestEnum.fromJson(Map<String, Object?> json) => ManifestEnum(
    name: json['name']! as String,
    dartClass: json['dartClass'] as String?,
    dartLibrary: json['dartLibrary'] as String?,
    inCatalog: json['inCatalog']! as bool,
    serverOwned: json['serverOwned']! as bool,
  );

  final String name;
  final String? dartClass;

  /// 列舉所在的生成檔（`package:hawkr_api/…/x.pbenum.dart`）。
  final String? dartLibrary;
  final bool inCatalog;
  final bool serverOwned;
}

/// 選擇構造的種類（設計 §1.2）。[label] 是回報與測試用的名稱，與設計的 form 名稱一致。
enum Form {
  ifThen('if'),
  elseBranch('else'),
  ternary('ternary'),
  and('and'),
  or('or'),
  guard('guard'),
  caseBranch('case'),
  key('key'),
  pair('pair');

  const Form(this.label);

  final String label;
}

/// 進 `--report` 的形式：值到文字的對應表。其餘是行為上的分流，只用在失敗判斷。
const reportForms = {Form.caseBranch, Form.key, Form.pair};

/// 命中的一處：區域由 [enumName].[member] 確定成立，而且區域裡有使用者面文字。
class Hit {
  const Hit(this.file, this.line, this.form, this.enumName, this.member);

  final String file;

  /// 選擇子（`E.M`）所在的行。
  final int line;
  final Form form;
  final String enumName;
  final String member;

  String get rule => 'E';

  @override
  String toString() => '$file:$line:$rule:${form.label}:$enumName.$member';
}

/// 一個 import 的檔裡，頂層 `final`／`const` 且初始值是含漢字字串的名稱；解析不到（不在同一個 client 目錄樹內、
/// 不是相對路徑或本 App 的 package 路徑）回 null。scanSource 不碰檔案系統，由呼叫端（scanDir）提供。
typedef ConstResolver = Set<String>? Function(String uri, String fromPath);

List<ManifestEnum> loadManifest([Uri? uri]) {
  final file = File.fromUri(
    uri ?? Platform.script.resolve('enum_manifest.g.json'),
  );
  final json = jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
  return [
    for (final e in json['enums']! as List<Object?>)
      ManifestEnum.fromJson(e! as Map<String, Object?>),
  ];
}

bool _hasHanLiteral(AstNode node) {
  final visitor = _PlainHanVisitor();
  node.accept(visitor);
  return visitor.found;
}

class _PlainHanVisitor extends RecursiveAstVisitor<void> {
  bool found = false;

  @override
  void visitSimpleStringLiteral(SimpleStringLiteral node) {
    if (_han.hasMatch(node.value)) found = true;
  }

  @override
  void visitInterpolationString(InterpolationString node) {
    if (_han.hasMatch(node.value)) found = true;
  }
}

Expression _strip(Expression e) {
  var cur = e;
  while (cur is ParenthesizedExpression) {
    cur = cur.expression;
  }
  return cur;
}

/// 初始值是含漢字的字串（可包在括號裡）。
bool _isHanStringInit(Expression? e) {
  if (e == null) return false;
  final cur = _strip(e);
  return cur is StringLiteral && _hasHanLiteral(cur);
}

/// 一個檔裡頂層 `final`／`const` 且初始值是含漢字字串的公開名稱（跨檔常數，K4）。
Set<String> exportedHanConsts(String path, String source) {
  final unit = parseString(
    content: source,
    path: path,
    throwIfDiagnostics: false,
  ).unit;
  final names = <String>{};
  for (final d in unit.declarations) {
    if (d is! TopLevelVariableDeclaration) continue;
    final list = d.variables;
    if (!list.isFinal && !list.isConst) continue;
    for (final v in list.variables) {
      final name = v.name.lexeme;
      if (!name.startsWith('_') && _isHanStringInit(v.initializer)) {
        names.add(name);
      }
    }
  }
  return names;
}

/// `…/x.pb.dart` 與 `…/x.pbenum.dart` 同屬一組生成檔，取共同的 `…/x`；不是 api 生成檔回 null。
String? _protoBase(String uri) {
  if (!uri.startsWith('package:hawkr_api/')) return null;
  for (final suffix in const ['.pbenum.dart', '.pb.dart']) {
    if (uri.endsWith(suffix)) {
      return uri.substring(0, uri.length - suffix.length);
    }
  }
  return null;
}

/// import 的 show／hide 篩過之後可見的名稱。
Set<String> _visibleNames(Set<String> names, NamespaceDirective directive) {
  var visible = names;
  for (final c in directive.combinators) {
    if (c is ShowCombinator) {
      final shown = {for (final n in c.shownNames) n.name};
      visible = visible.intersection(shown);
    } else if (c is HideCombinator) {
      final hidden = {for (final n in c.hiddenNames) n.name};
      visible = visible.difference(hidden);
    }
  }
  return visible;
}

/// 這個檔從 api 生成檔（`package:hawkr_api` 的 `.pb.dart`、`.pbenum.dart`）引入的 enum 類別。
/// 同名但來自別處的類別（例如 App 的 CodeMeta `ReportStatus`）不算 proto enum。
Map<String, ManifestEnum> _importedProtoEnums(
  CompilationUnit unit,
  List<ManifestEnum> manifest,
) {
  final result = <String, ManifestEnum>{};
  for (final directive in unit.directives) {
    if (directive is! NamespaceDirective) continue;
    final uri = directive.uri.stringValue;
    final base = uri == null ? null : _protoBase(uri);
    if (base == null) continue;
    var show = <String>{};
    var hide = <String>{};
    var hasShow = false;
    for (final c in directive.combinators) {
      if (c is ShowCombinator) {
        hasShow = true;
        show = {for (final n in c.shownNames) n.name};
      } else if (c is HideCombinator) {
        hide = {for (final n in c.hiddenNames) n.name};
      }
    }
    for (final e in manifest) {
      final cls = e.dartClass;
      final lib = e.dartLibrary;
      if (cls == null || lib == null || _protoBase(lib) != base) continue;
      if (hasShow && !show.contains(cls)) continue;
      if (hide.contains(cls)) continue;
      result.putIfAbsent(cls, () => e);
    }
  }
  return result;
}

/// 掃一段原始碼，回「由 enum 成員確定成立的區域裡產生使用者面文字」的位置。
/// [consts] 解析 import 進來的常數（K4）；沒給就只看同檔。
List<Hit> scanSource(
  String path,
  String source,
  List<ManifestEnum> manifest, {
  ConstResolver? consts,
}) {
  final result = parseString(
    content: source,
    path: path,
    throwIfDiagnostics: false,
  );
  final byClass = _importedProtoEnums(result.unit, manifest);
  if (byClass.isEmpty) return const [];
  final importedConsts = <String>{};
  final prefixedConsts = <String, Set<String>>{};
  if (consts != null) {
    for (final directive in result.unit.directives) {
      if (directive is! ImportDirective) continue;
      final uri = directive.uri.stringValue;
      if (uri == null || _protoBase(uri) != null) continue;
      final exported = consts(uri, path);
      if (exported == null) continue;
      final visible = _visibleNames(exported, directive);
      final prefix = directive.prefix?.name;
      if (prefix == null) {
        importedConsts.addAll(visible);
      } else {
        (prefixedConsts[prefix] ??= {}).addAll(visible);
      }
    }
  }
  final visitor = _ScanVisitor(
    path,
    result.lineInfo,
    byClass,
    importedConsts,
    prefixedConsts,
  );
  result.unit.accept(visitor);
  return visitor.hits;
}

/// 一個選擇子：`E.M`。
class _Sel {
  const _Sel(this.e, this.member, this.node);

  final ManifestEnum e;
  final String member;
  final AstNode node;
}

/// 變數宣告查到的結果：[shadow] 是名稱被別的東西遮住（參數、`var`），不是可代入的常數。
class _Var {
  const _Var(this.init, {this.shadow = false});

  final Expression? init;
  final bool shadow;
}

class _TextFinder extends RecursiveAstVisitor<void> {
  _TextFinder(this.scan, {required this.shallow});

  final _ScanVisitor scan;
  final bool shallow;
  bool found = false;

  @override
  void visitSimpleStringLiteral(SimpleStringLiteral node) {
    if (_han.hasMatch(node.value)) found = true;
  }

  @override
  void visitInterpolationString(InterpolationString node) {
    if (_han.hasMatch(node.value)) found = true;
  }

  @override
  void visitSimpleIdentifier(SimpleIdentifier node) {
    if (scan.isReference(node) && scan.isHanConst(node)) found = true;
  }

  @override
  void visitIndexExpression(IndexExpression node) {
    final target = node.target;
    final index = node.index;
    if (target is SimpleIdentifier &&
        index is SimpleStringLiteral &&
        scan.isObjectConstText(target, index.value)) {
      found = true;
      return;
    }
    super.visitIndexExpression(node);
  }

  @override
  void visitPrefixedIdentifier(PrefixedIdentifier node) {
    if (scan.isObjectConstText(node.prefix, node.identifier.name)) {
      found = true;
      return;
    }
    final names = scan.prefixedConsts[node.prefix.name];
    if (names != null && names.contains(node.identifier.name)) {
      found = true;
      return;
    }
    super.visitPrefixedIdentifier(node);
  }

  @override
  void visitFunctionExpression(FunctionExpression node) {
    if (!shallow) super.visitFunctionExpression(node);
  }
}

class _ScanVisitor extends RecursiveAstVisitor<void> {
  _ScanVisitor(
    this.path,
    this.lineInfo,
    this.byClass,
    this.importedConsts,
    this.prefixedConsts,
  );

  final String path;
  final LineInfo lineInfo;
  final Map<String, ManifestEnum> byClass;
  final Set<String> importedConsts;
  final Map<String, Set<String>> prefixedConsts;
  final hits = <Hit>[];
  final _seen = <String>{};

  /// `E.M`（含 import 前綴 `p.E.M`，可包在括號裡）；不是 manifest 裡的 enum、或是值號 0，回 null。
  _Sel? _select(Expression? expr) {
    if (expr == null) return null;
    final e = _strip(expr);
    if (e is PrefixedIdentifier) {
      final en = byClass[e.prefix.name];
      if (en != null && !_unspecified.hasMatch(e.identifier.name)) {
        return _Sel(en, e.identifier.name, e);
      }
      // p.E 本身（後面還有成員）由 PropertyAccess 處理。
      return null;
    }
    if (e is PropertyAccess) {
      final target = e.target;
      if (target is PrefixedIdentifier) {
        final en = byClass[target.identifier.name];
        if (en != null && !_unspecified.hasMatch(e.propertyName.name)) {
          return _Sel(en, e.propertyName.name, e);
        }
      }
    }
    return null;
  }

  // 作用域（K4、K5 共用）：從使用處往上找函式參數／Block／switch 成員／CompilationUnit；區塊裡只看使用處之前的宣告，
  // 頂層看全部。參數、`var` 遮住同名的 final／const。
  _Var? _lookup(SimpleIdentifier id) {
    final name = id.name;
    final pos = id.offset;
    for (AstNode? n = id.parent; n != null; n = n.parent) {
      final params = switch (n) {
        FunctionExpression() => n.parameters,
        MethodDeclaration() => n.parameters,
        ConstructorDeclaration() => n.parameters,
        _ => null,
      };
      if (params != null &&
          params.parameters.any((p) => p.name?.lexeme == name)) {
        return const _Var(null, shadow: true);
      }
      final statements = switch (n) {
        Block() => n.statements,
        SwitchMember() => n.statements,
        _ => null,
      };
      if (statements != null) {
        for (final st in statements) {
          if (st is! VariableDeclarationStatement || st.end > pos) continue;
          final list = st.variables;
          for (final v in list.variables) {
            if (v.name.lexeme != name) continue;
            return list.isFinal || list.isConst
                ? _Var(v.initializer)
                : const _Var(null, shadow: true);
          }
        }
      }
      if (n is CompilationUnit) {
        for (final d in n.declarations) {
          if (d is! TopLevelVariableDeclaration) continue;
          final list = d.variables;
          for (final v in list.variables) {
            if (v.name.lexeme != name) continue;
            return list.isFinal || list.isConst
                ? _Var(v.initializer)
                : const _Var(null, shadow: true);
          }
        }
      }
    }
    return null;
  }

  /// 識別字是「引用」，不是屬性名、方法名或具名引數的標籤。
  bool isReference(SimpleIdentifier id) {
    final p = id.parent;
    if (p is PrefixedIdentifier && p.identifier == id) return false;
    if (p is PropertyAccess && p.propertyName == id) return false;
    if (p is MethodInvocation && p.methodName == id) return false;
    if (p is Label) return false;
    return true;
  }

  /// 識別字是含漢字字串的 final／const：同檔（作用域可見）或 import 進來的（K4）。
  bool isHanConst(SimpleIdentifier id) {
    final v = _lookup(id);
    if (v != null) return !v.shadow && _isHanStringInit(v.init);
    return importedConsts.contains(id.name);
  }

  /// `TEXT.key`（record 常數的欄位）、`TEXT['key']`（map 常數的字串 key）：同檔的物件常數，
  /// 該成員的值含漢字字串（K4）。
  bool isObjectConstText(SimpleIdentifier obj, String key) {
    final v = _lookup(obj);
    if (v == null || v.shadow || v.init == null) return false;
    final init = _strip(v.init!);
    if (init is RecordLiteral) {
      return init.fields.any(
        (f) =>
            f is NamedExpression &&
            f.name.label.name == key &&
            _hasHanLiteral(f.expression),
      );
    }
    if (init is SetOrMapLiteral) {
      return init.elements.any(
        (e) =>
            e is MapLiteralEntry &&
            e.key is SimpleStringLiteral &&
            (e.key as SimpleStringLiteral).value == key &&
            _hasHanLiteral(e.value),
      );
    }
    return false;
  }

  /// 區域裡產生文字（K4）。shallow：不進入巢狀函式本體（pair 用）。
  bool _hasText(AstNode? node, {bool shallow = false}) {
    if (node == null) return false;
    final finder = _TextFinder(this, shallow: shallow);
    node.accept(finder);
    return finder.found;
  }

  bool _isNullish(Expression e) => _strip(e) is NullLiteral;

  /// 樣式裡的 `E.M`（ConstantPattern）與 `== E.M`（RelationalPattern），任何位置都算
  /// （or、and、record、object、cast、null-check…）。
  List<_Sel> _patternSels(AstNode? pattern) {
    final out = <_Sel>[];
    pattern?.accept(_PatternVisitor(this, out));
    return out;
  }

  /// 成員檢查的接收者：list／set 字面值，或指到這種初始值的 final／const（K5 的作用域查找）。
  NodeList<CollectionElement>? _collectionOf(Expression? e, [int depth = 0]) {
    if (e == null) return null;
    final x = _strip(e);
    if (x is ListLiteral) return x.elements;
    if (x is SetOrMapLiteral) return x.elements;
    if (x is SimpleIdentifier && depth < 4) {
      final v = _lookup(x);
      if (v != null && !v.shadow) return _collectionOf(v.init, depth + 1);
    }
    return null;
  }

  /// expr 取 pol（真或假）時確定「相等」的成員（K3）。
  List<_Sel> _implied(
    Expression? expr,
    bool pol, [
    int depth = 0,
    bool membership = true,
  ]) {
    if (expr == null || depth > 8) return const [];
    var c = _strip(expr);
    if (c is PrefixExpression && c.operator.type == TokenType.BANG) {
      return _implied(c.operand, !pol, depth + 1, membership);
    }
    if (c is BinaryExpression) {
      final t = c.operator.type;
      if (t == TokenType.AMPERSAND_AMPERSAND) {
        return pol
            ? [
                ..._implied(c.leftOperand, true, depth + 1, membership),
                ..._implied(c.rightOperand, true, depth + 1, membership),
              ]
            : const [];
      }
      if (t == TokenType.BAR_BAR) {
        return pol
            ? const []
            : [
                ..._implied(c.leftOperand, false, depth + 1, membership),
                ..._implied(c.rightOperand, false, depth + 1, membership),
              ];
      }
      if (t == TokenType.EQ_EQ || t == TokenType.BANG_EQ) {
        // `alias != null`（或 `alias == null` 為假）：別名 `cond ? x : null` 有值，等於它的 cond 為真。
        final other = _isNullish(c.rightOperand)
            ? _strip(c.leftOperand)
            : _isNullish(c.leftOperand)
            ? _strip(c.rightOperand)
            : null;
        if (other != null) {
          final v = other is SimpleIdentifier ? _lookup(other) : null;
          final init = v == null || v.shadow || v.init == null
              ? null
              : _strip(v.init!);
          return (t == TokenType.BANG_EQ) == pol &&
                  init is ConditionalExpression
              ? _implied(other, true, depth + 1, membership)
              : const [];
        }
        if ((t == TokenType.EQ_EQ) != pol) return const [];
        final s = _select(c.leftOperand) ?? _select(c.rightOperand);
        return s == null ? const [] : [s];
      }
      return const [];
    }
    if (c is MethodInvocation && c.methodName.name == 'contains') {
      if (!pol || !membership) return const [];
      final elements = _collectionOf(c.target);
      if (elements == null) return const [];
      return [
        for (final el in elements)
          if (el is Expression) ?_select(el),
      ];
    }
    if (c is SimpleIdentifier) {
      final v = _lookup(c);
      final init = v == null || v.shadow ? null : v.init;
      if (init == null) return const [];
      final i = _strip(init);
      if (i is ConditionalExpression) {
        if (!pol) return const [];
        if (_isNullish(i.elseExpression)) {
          return _implied(i.condition, true, depth + 1, membership);
        }
        if (_isNullish(i.thenExpression)) {
          return _implied(i.condition, false, depth + 1, membership);
        }
        return const [];
      }
      return _implied(i, pol, depth + 1, membership);
    }
    return const [];
  }

  /// [sels] 在 [regions] 裡有文字就記一筆。
  void _emit(
    Iterable<_Sel> sels,
    Iterable<AstNode?> regions,
    Form form, {
    bool shallow = false,
  }) {
    if (sels.isEmpty) return;
    if (!regions.any((r) => _hasText(r, shallow: shallow))) return;
    for (final s in sels) {
      final line = lineInfo.getLocation(s.node.offset).lineNumber;
      final key = '$line:${form.label}:${s.e.name}.${s.member}';
      if (!_seen.add(key)) continue;
      hits.add(Hit(path, line, form, s.e.name, s.member));
    }
  }

  List<_Sel> _caseSels(GuardedPattern g) => [
    ..._patternSels(g.pattern),
    ..._implied(g.whenClause?.expression, true),
  ];

  bool _exits(Statement s) {
    final last = s is Block
        ? (s.statements.isEmpty ? null : s.statements.last)
        : s;
    return last is ReturnStatement ||
        last is BreakStatement ||
        last is ContinueStatement ||
        (last is ExpressionStatement && last.expression is ThrowExpression);
  }

  @override
  void visitIfStatement(IfStatement node) {
    final caseClause = node.caseClause;
    if (caseClause == null) {
      _emit(_implied(node.expression, true), [node.thenStatement], Form.ifThen);
      _emit(_implied(node.expression, false), [
        node.elseStatement,
      ], Form.elseBranch);
      final parent = node.parent;
      final siblings = switch (parent) {
        Block() => parent.statements,
        SwitchMember() => parent.statements,
        _ => null,
      };
      if (node.elseStatement == null &&
          siblings != null &&
          _exits(node.thenStatement)) {
        final st = siblings;
        _emit(
          _implied(node.expression, false, 0, false),
          st.sublist(st.indexOf(node) + 1),
          Form.guard,
        );
      }
    } else {
      // if-case：樣式與 when 成立時才進 then；else 是補集，不算。
      _emit(
        [
          ..._caseSels(caseClause.guardedPattern),
          ..._implied(node.expression, true),
        ],
        [node.thenStatement],
        Form.ifThen,
      );
    }
    super.visitIfStatement(node);
  }

  @override
  void visitIfElement(IfElement node) {
    final caseClause = node.caseClause;
    if (caseClause == null) {
      _emit(_implied(node.expression, true), [node.thenElement], Form.ifThen);
      _emit(_implied(node.expression, false), [
        node.elseElement,
      ], Form.elseBranch);
    } else {
      _emit(
        [
          ..._caseSels(caseClause.guardedPattern),
          ..._implied(node.expression, true),
        ],
        [node.thenElement],
        Form.ifThen,
      );
    }
    super.visitIfElement(node);
  }

  @override
  void visitConditionalExpression(ConditionalExpression node) {
    _emit(_implied(node.condition, true), [node.thenExpression], Form.ternary);
    _emit(_implied(node.condition, false), [node.elseExpression], Form.ternary);
    super.visitConditionalExpression(node);
  }

  @override
  void visitBinaryExpression(BinaryExpression node) {
    final t = node.operator.type;
    if (t == TokenType.AMPERSAND_AMPERSAND) {
      _emit(_implied(node.leftOperand, true), [node.rightOperand], Form.and);
    } else if (t == TokenType.BAR_BAR) {
      _emit(_implied(node.leftOperand, false), [node.rightOperand], Form.or);
    }
    super.visitBinaryExpression(node);
  }

  @override
  void visitSwitchStatement(SwitchStatement node) {
    var pending = <_Sel>[];
    for (final member in node.members) {
      if (member is SwitchDefault) {
        // default 可由任何值到達：不是確定相等；它的敘述也不屬於前面 fall-through 進來的成員。
        pending = [];
        continue;
      }
      if (member is SwitchPatternCase) {
        pending.addAll(_caseSels(member.guardedPattern));
      } else if (member is SwitchCase) {
        final s = _select(member.expression);
        if (s != null) pending.add(s);
      }
      if (member.statements.isEmpty) continue;
      _emit(pending, member.statements, Form.caseBranch);
      pending = [];
    }
    super.visitSwitchStatement(node);
  }

  @override
  void visitSwitchExpression(SwitchExpression node) {
    for (final c in node.cases) {
      _emit(_caseSels(c.guardedPattern), [c.expression], Form.caseBranch);
    }
    super.visitSwitchExpression(node);
  }

  @override
  void visitSetOrMapLiteral(SetOrMapLiteral node) {
    final entries = node.elements.whereType<MapLiteralEntry>().toList();
    for (final e in entries) {
      final s = _select(e.key);
      if (s != null) _emit([s], [e.value], Form.key);
    }
    // `{'value': E.M, 'label': '警告'}`：字串 key 對應 enum 值，其他的成員有文字。
    final pairs = entries
        .where((e) => e.key is StringLiteral && _select(e.value) != null)
        .toList();
    if (pairs.isNotEmpty) {
      _emit(
        [for (final p in pairs) _select(p.value)!],
        [
          for (final e in node.elements)
            if (!pairs.contains(e)) e,
        ],
        Form.pair,
        shallow: true,
      );
    }
    super.visitSetOrMapLiteral(node);
  }

  /// 並列的成員（record 欄位、引數、list 元素）：有選擇子時，其他成員有文字就命中。
  void _pair(Iterable<AstNode> items) {
    Expression? value(AstNode n) =>
        n is NamedExpression ? n.expression : (n is Expression ? n : null);
    final list = items.toList();
    final sels = <_Sel>[];
    final others = <AstNode>[];
    for (final n in list) {
      final s = _select(value(n));
      if (s != null) {
        sels.add(s);
      } else {
        others.add(n);
      }
    }
    _emit(sels, others, Form.pair, shallow: true);
  }

  @override
  void visitRecordLiteral(RecordLiteral node) {
    _pair(node.fields);
    super.visitRecordLiteral(node);
  }

  @override
  void visitArgumentList(ArgumentList node) {
    _pair(node.arguments);
    super.visitArgumentList(node);
  }

  @override
  void visitListLiteral(ListLiteral node) {
    _pair(node.elements);
    super.visitListLiteral(node);
  }
}

class _PatternVisitor extends RecursiveAstVisitor<void> {
  _PatternVisitor(this.scan, this.out);

  final _ScanVisitor scan;
  final List<_Sel> out;

  @override
  void visitConstantPattern(ConstantPattern node) {
    final s = scan._select(node.expression);
    if (s != null) out.add(s);
  }

  @override
  void visitRelationalPattern(RelationalPattern node) {
    if (node.operator.type != TokenType.EQ_EQ) return;
    final s = scan._select(node.operand);
    if (s != null) out.add(s);
  }
}

bool _isSkippedFile(String name) =>
    !name.endsWith('.dart') ||
    name.endsWith('.g.dart') ||
    name.endsWith('.freezed.dart') ||
    name.endsWith('_test.dart');

List<File> _listSources(Directory dir) {
  final out = <File>[];
  for (final entity in dir.listSync(followLinks: false)) {
    final name = entity.uri.pathSegments.lastWhere((s) => s.isNotEmpty);
    if (entity is Directory) {
      if (!_excludedDirs.contains(name)) out.addAll(_listSources(entity));
    } else if (entity is File && !_isSkippedFile(name)) {
      out.add(entity);
    }
  }
  out.sort((a, b) => a.path.compareTo(b.path));
  return out;
}

String _relative(String base, String path) {
  final prefix = base.endsWith('/') ? base : '$base/';
  return path.startsWith(prefix) ? path.substring(prefix.length) : path;
}

/// 解析跨檔常數（K4）：相對路徑，或本 App 的 `package:<name>/…`（name 取自 root 上一層的 pubspec.yaml，
/// 對應 root 本身，即 lib），而且落在掃描的目錄樹內；只追一層（被匯入檔自己的 import 不再展開）。
ConstResolver makeConstResolver(Directory root) {
  final base = root.absolute.uri.toFilePath();
  final rootPath = base.endsWith('/')
      ? base.substring(0, base.length - 1)
      : base;
  final pubspec = File('$rootPath/../pubspec.yaml');
  final pkg = pubspec.existsSync()
      ? RegExp(
          r'^name:\s*(\S+)',
          multiLine: true,
        ).firstMatch(pubspec.readAsStringSync())?.group(1)
      : null;
  final cache = <String, Set<String>>{};
  return (uri, fromPath) {
    Uri target;
    if (uri.startsWith('package:')) {
      final prefix = 'package:$pkg/';
      if (pkg == null || !uri.startsWith(prefix)) return null;
      target = Uri.file('$rootPath/${uri.substring(prefix.length)}');
    } else if (uri.contains(':')) {
      return null;
    } else {
      target = Uri.file('$rootPath/$fromPath').resolve(uri);
    }
    final file = File.fromUri(target);
    final normalized = file.absolute.uri.normalizePath().toFilePath();
    if (!normalized.startsWith('$rootPath/') ||
        !File(normalized).existsSync()) {
      return null;
    }
    return cache.putIfAbsent(
      normalized,
      () => exportedHanConsts(normalized, File(normalized).readAsStringSync()),
    );
  };
}

/// 掃目錄，路徑相對於該目錄。
List<Hit> scanDir(Directory dir, List<ManifestEnum> manifest) {
  final hits = <Hit>[];
  final consts = makeConstResolver(dir);
  for (final file in _listSources(dir)) {
    hits.addAll(
      scanSource(
        _relative(dir.path, file.path),
        file.readAsStringSync(),
        manifest,
        consts: consts,
      ),
    );
  }
  return hits;
}

/// 只放對應形式（[reportForms]）的命中：跨端檢查問的是「值到文字的對照表」在兩個 client 各寫一份。
Map<String, Object?> buildReport(Iterable<Hit> hits) {
  final enums = <String, List<String>>{};
  for (final h in hits) {
    if (!reportForms.contains(h.form)) continue;
    (enums[h.enumName] ??= []).add('${h.file}:${h.line}');
  }
  for (final list in enums.values) {
    list.sort();
  }
  final keys = enums.keys.toList()..sort();
  return {
    'enums': {for (final k in keys) k: enums[k]},
  };
}

/// 必須擋下的命中：目錄裡的 enum 與伺服器擁有的 enum，任何 form。
List<Hit> forbidden(Iterable<Hit> hits, List<ManifestEnum> manifest) {
  final blocked = {
    for (final e in manifest)
      if (e.inCatalog || e.serverOwned) e.name,
  };
  return [
    for (final h in hits)
      if (blocked.contains(h.enumName)) h,
  ];
}

int run(List<String> argv, {Uri? manifestUri}) {
  final args = [...argv];
  String? reportPath;
  final i = args.indexOf('--report');
  if (i >= 0) {
    if (i + 1 >= args.length) {
      stderr.writeln('用法：check_enum_text [--report <file>] <dir>...');
      return 2;
    }
    reportPath = args[i + 1];
    args.removeRange(i, i + 2);
  }
  if (args.isEmpty) {
    stderr.writeln('用法：check_enum_text [--report <file>] <dir>...');
    return 2;
  }
  final manifest = loadManifest(manifestUri);
  final hits = <Hit>[];
  for (final d in args) {
    final dir = Directory(d);
    if (!dir.existsSync()) {
      stderr.writeln('check_enum_text：找不到目錄 $d');
      return 2;
    }
    hits.addAll(scanDir(dir, manifest));
  }
  if (reportPath != null) {
    File(reportPath).writeAsStringSync(
      '${const JsonEncoder.withIndent('  ').convert(buildReport(hits))}\n',
    );
  }
  final bad = forbidden(hits, manifest);
  for (final h in bad) {
    stderr.writeln(
      '${h.file}:${h.line}: ${h.rule}/${h.form.label} ${h.enumName}.${h.member} 成立時產生使用者面文字；'
      '這個 enum 的文字在共用目錄（hawkr_ui_text）或由伺服器擁有',
    );
  }
  if (bad.isNotEmpty) {
    stderr.writeln(
      '\ncheck_enum_text：${bad.length} 處。文字改用 hawkr_ui_text 的 uiText，或讀伺服器給的 LocalizedMessage；'
      '文字不是這個值的說法時，改成依契約的欄位或伺服器給的可用性分流。',
    );
    return 1;
  }
  return 0;
}

void main(List<String> args) {
  exitCode = run(args);
}
