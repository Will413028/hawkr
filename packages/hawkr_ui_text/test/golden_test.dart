import 'dart:convert';
import 'dart:io';

import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:test/test.dart';

import 'golden_calls.g.dart';

void main() {
  final cases = [
    for (final c
        in jsonDecode(File('test/golden.g.json').readAsStringSync())
            as List<Object?>)
      c! as Map<String, Object?>,
  ];

  UiLocale localeOf(String tag) =>
      UiLocale.values.singleWhere((l) => l.tag == tag);

  test('每一筆預設受眾都與參考格式器一致', () {
    final failures = <String>[];
    var checked = 0;
    for (final c in cases) {
      if (c['audience'] != 'default') continue;
      checked++;
      final got = goldenText(
        c['fn']! as String,
        (c['value'] as String?) ?? '',
        (c['params'] as Map<String, Object?>?) ?? const {},
        localeOf(c['locale']! as String),
      );
      if (got != c['want']) {
        failures.add(
          '${c['key']} [${c['locale']}] ${c['params']}: got $got, want ${c['want']}',
        );
      }
    }
    expect(failures, isEmpty);
    expect(checked, greaterThan(0));
  });

  test('golden 涵蓋每個函式與兩個語系，且含「沒有文字」的案例', () {
    final withText = <String, int>{};
    final locales = <String>{};
    for (final c in cases.where((c) => c['audience'] == 'default')) {
      locales.add(c['locale']! as String);
      withText.update(
        c['fn']! as String,
        (n) => n + (c['want'] == null ? 0 : 1),
        ifAbsent: () => c['want'] == null ? 0 : 1,
      );
    }
    expect(locales, {'zh-TW', 'en'});
    withText.forEach((fn, n) => expect(n, greaterThan(0), reason: fn));
    expect(cases.any((c) => c['want'] == null), isTrue);
  });

  test('en 缺字時輸出 zh-TW 的文字', () {
    final en = cases.singleWhere(
      (c) =>
          c['key'] == 'seller.MeasureKind.MEASURE_KIND_WARNING' &&
          c['locale'] == 'en',
    );
    expect(en['want'], '警告');
  });

  test('整數參數輸出十進位 ASCII、不分組', () {
    final c = cases.firstWhere(
      (c) =>
          c['fn'] == 'seller.Measure.rejection_cooldown_warning' &&
          (c['params']! as Map<String, Object?>)['days'] == 1234 &&
          c['locale'] == 'zh-TW',
    );
    expect(c['want'], contains(' 1234 '));
  });
}
