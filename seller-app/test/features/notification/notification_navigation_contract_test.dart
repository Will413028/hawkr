import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// 通知（收件匣 ＋ FCM 推播）的導航參數契約。
///
/// 從 `app_router.dart` 自動抽出「缺參數就渲染資料無效」的路由清單
/// （判準＝該 `GoRoute(` 區塊內出現 `_invalidRouteData` 或「訂單資料無效」），
/// 再掃三支通知導航檔的每一個 `push`／`go`，命中清單卻沒帶 `extra:` 就轉紅。
///
/// ⚠️ 已知限制（誠實標註）：`/customer/detail` 的守衛在 `_CustomerDetailRoute`
/// 這個 widget 裡而不在 `GoRoute(` 區塊內，因此不會被認成 guarded。目前沒有
/// 任何通知路徑指向它；哪天有了，要把守衛搬進區塊或在這裡補一筆。
void main() {
  const navigationFiles = [
    'lib/features/notification/data/push/router.dart',
    'lib/features/notification/presentation/notifications_screen.dart',
    'lib/features/notification/data/delegation_navigation.dart',
  ];

  final appRouter = _blankComments(
    File('lib/core/router/app_router.dart').readAsStringSync(),
  );
  final routeConstants = _routeConstants(appRouter);
  final guarded = _guardedRoutes(appRouter, routeConstants);

  test('抽得出必填 extra 的路由清單（清單為空＝掃描器壞了，不是沒有問題）', () {
    // 這一條是掃描器自己的鑑別力：guarded 若因為 app_router 改寫而抽成空集合，
    // 下面那條主契約會變成恆真，靜默失去保護。
    expect(guarded, isNotEmpty);
    expect(guarded.keys, contains('/orders/matched/detail'));
    expect(guarded.keys, contains('/product/preview'));
  });

  test('通知導航不得 push 到必填 extra 的路由卻不帶 extra', () {
    final scanned = <String>[];
    final violations = <String>[];

    for (final path in navigationFiles) {
      final source = _blankComments(File(path).readAsStringSync());
      for (final call in _navigationCalls(source)) {
        final target = _resolveTarget(call.firstArgument, routeConstants);
        if (target == null || !guarded.containsKey(target)) continue;
        scanned.add('$path:${_lineOf(source, call.offset)} → $target');
        if (!call.hasExtra) {
          violations.add(
            '$path:${_lineOf(source, call.offset)} '
            'push 到 $target（${guarded[target]}）卻沒帶 extra —— '
            '該路由缺參數會渲染成「資料無效」空白頁。'
            '帶齊參數，或在取不到參數時不要導航。',
          );
        }
      }
    }

    // 掃到零個命中＝掃描器失效（例如呼叫寫法改了正規表示式對不上），
    // 而不是「全部合規」。
    expect(scanned, isNotEmpty, reason: '一個受保護路由的呼叫都沒掃到，掃描器可能失效');
    expect(violations, isEmpty, reason: violations.join('\n'));
  });
}

// ── 以下是原始碼掃描工具 ─────────────────────────────────────────────

class _NavCall {
  const _NavCall({
    required this.firstArgument,
    required this.hasExtra,
    required this.offset,
  });

  final String firstArgument;
  final bool hasExtra;
  final int offset;
}

/// 把註解換成等長空白（保留 offset），字串字面值原樣留著。
///
/// 必要性：本檔要掃的那幾個檔案的**註解裡就寫著** `push('/post/feed')`
/// （說明修前長什麼樣），不濾掉會把文件當成違規。
String _blankComments(String source) {
  final out = StringBuffer();
  var i = 0;
  while (i < source.length) {
    final c = source[i];
    final next = i + 1 < source.length ? source[i + 1] : '';
    if (c == '/' && next == '/') {
      while (i < source.length && source[i] != '\n') {
        out.write(' ');
        i++;
      }
      continue;
    }
    if (c == '/' && next == '*') {
      while (i < source.length &&
          !(source[i] == '*' &&
              i + 1 < source.length &&
              source[i + 1] == '/')) {
        out.write(source[i] == '\n' ? '\n' : ' ');
        i++;
      }
      out.write('  ');
      i += 2;
      continue;
    }
    if (c == "'" || c == '"') {
      final quote = c;
      out.write(c);
      i++;
      while (i < source.length) {
        if (source[i] == r'\') {
          out.write(source.substring(i, (i + 2).clamp(0, source.length)));
          i += 2;
          continue;
        }
        out.write(source[i]);
        final closed = source[i] == quote;
        i++;
        if (closed) break;
      }
      continue;
    }
    out.write(c);
    i++;
  }
  return out.toString();
}

/// `class XxxRoutes { static const foo = '/foo'; }` → `{'XxxRoutes.foo': '/foo'}`
Map<String, String> _routeConstants(String appRouter) {
  final result = <String, String>{};
  for (final klass in RegExp(
    r'class (\w+Routes) \{([\s\S]*?)\n\}',
  ).allMatches(appRouter)) {
    final name = klass.group(1)!;
    for (final constant in RegExp(
      r"static const (\w+) = '([^']+)';",
    ).allMatches(klass.group(2)!)) {
      result['$name.${constant.group(1)}'] = constant.group(2)!;
    }
  }
  return result;
}

/// 缺參數就渲染「資料無效」的路由 → 該路由的說明標籤。
Map<String, String> _guardedRoutes(
  String appRouter,
  Map<String, String> routeConstants,
) {
  final result = <String, String>{};
  final blocks = appRouter.split('GoRoute(');
  // 第 0 塊是 routes list 之前的東西（含 _CustomerDetailRoute 等 helper），略過。
  for (final block in blocks.skip(1)) {
    final pathMatch = RegExp(r'path: ([\w.]+),').firstMatch(block);
    if (pathMatch == null) continue;
    final literal = routeConstants[pathMatch.group(1)!];
    if (literal == null) continue;
    final guard = RegExp(
      r"_invalidRouteData\('([^']+)'\)|Text\('(訂單資料無效)'\)",
    ).firstMatch(block);
    if (guard == null) continue;
    result[literal] = guard.group(1) ?? guard.group(2)!;
  }
  return result;
}

/// 掃出每一個 `.push(...)` / `.go(...)`，回傳第一個參數與是否帶 `extra:`。
List<_NavCall> _navigationCalls(String source) {
  final calls = <_NavCall>[];
  for (final match in RegExp(r'\.(push|go)(<[^>]*>)?\(').allMatches(source)) {
    final open = match.end - 1;
    final close = _matchingParen(source, open);
    if (close == null) continue;
    final args = source.substring(open + 1, close);
    calls.add(
      _NavCall(
        firstArgument: _firstArgument(args).trim(),
        hasExtra: RegExp(r'\bextra:').hasMatch(args),
        offset: match.start,
      ),
    );
  }
  return calls;
}

int? _matchingParen(String source, int open) {
  var depth = 0;
  var i = open;
  while (i < source.length) {
    final c = source[i];
    if (c == "'" || c == '"') {
      final quote = c;
      i++;
      while (i < source.length && source[i] != quote) {
        i += source[i] == r'\' ? 2 : 1;
      }
    } else if (c == '(') {
      depth++;
    } else if (c == ')') {
      depth--;
      if (depth == 0) return i;
    }
    i++;
  }
  return null;
}

String _firstArgument(String args) {
  var depth = 0;
  var i = 0;
  while (i < args.length) {
    final c = args[i];
    if (c == "'" || c == '"') {
      final quote = c;
      i++;
      while (i < args.length && args[i] != quote) {
        i += args[i] == r'\' ? 2 : 1;
      }
    } else if (c == '(' || c == '{' || c == '[') {
      depth++;
    } else if (c == ')' || c == '}' || c == ']') {
      depth--;
    } else if (c == ',' && depth == 0) {
      return args.substring(0, i);
    }
    i++;
  }
  return args;
}

/// 第一個參數解析成路由字面值；解不出（變數、欄位存取）回 null。
String? _resolveTarget(String firstArgument, Map<String, String> constants) {
  final literal = RegExp(r"^'([^']+)'$").firstMatch(firstArgument);
  if (literal != null) return literal.group(1)!.split('?').first;
  return constants[firstArgument];
}

int _lineOf(String source, int offset) =>
    '\n'.allMatches(source.substring(0, offset)).length + 1;
