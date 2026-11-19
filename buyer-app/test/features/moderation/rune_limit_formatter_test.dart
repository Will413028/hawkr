import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/moderation/presentation/rune_limit_formatter.dart';

const _family = '\u{1F468}‍\u{1F469}‍\u{1F467}';

TextEditingValue _value(
  String text, {
  int? caret,
  TextRange composing = TextRange.empty,
}) => TextEditingValue(
  text: text,
  selection: TextSelection.collapsed(offset: caret ?? text.length),
  composing: composing,
);

void main() {
  const formatter = RuneLimitFormatter(10);
  const old = TextEditingValue.empty;

  test('未超過上限不動', () {
    final value = _value('a' * 10);
    expect(formatter.formatEditUpdate(old, value), value);
  });

  test('超過上限依 rune 截斷', () {
    final out = formatter.formatEditUpdate(old, _value('a' * 12));
    expect(out.text, 'a' * 10);
  });

  test('跨過上限的字素叢集（emoji ZWJ 序列）整個丟掉，不拆開', () {
    final out = formatter.formatEditUpdate(old, _value('a' * 8 + _family));
    expect(out.text, 'a' * 8);
  });

  test('剛好放得下的字素叢集保留', () {
    final out = formatter.formatEditUpdate(old, _value('a' * 5 + _family));
    expect(out.text, 'a' * 5 + _family);
  });

  test('輸入法組字中不截斷', () {
    final value = _value(
      'a' * 9 + 'ㄅㄆㄇ',
      composing: const TextRange(start: 9, end: 12),
    );
    expect(formatter.formatEditUpdate(old, value), value);
  });

  test('截斷時游標不被移到尾端，只在超出文字長度時夾回', () {
    final out = formatter.formatEditUpdate(old, _value('a' * 12, caret: 3));
    expect(out.text, 'a' * 10);
    expect(out.selection, const TextSelection.collapsed(offset: 3));
    final tail = formatter.formatEditUpdate(old, _value('a' * 12, caret: 12));
    expect(tail.selection, const TextSelection.collapsed(offset: 10));
  });
}
